package feeder

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"gopkg.in/yaml.v3"
)

var pairRegex = regexp.MustCompile(`^[A-Z0-9]+:[A-Z0-9]+$`)

// PairConfig maps a canonical BASE:QUOTE pair to per-provider symbols.
type PairConfig struct {
	Pair    string            `yaml:"pair"`
	Symbols map[string]string `yaml:"symbols"`
}

// QualityConfig holds the data-quality guards (design D3).
type QualityConfig struct {
	MinProviders int           `yaml:"min_providers"`
	MaxDeviation string        `yaml:"max_deviation"`
	MaxQuoteAge  time.Duration `yaml:"max_quote_age"`
}

// PrometheusConfig configures the metrics server.
type PrometheusConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// Config is the full vertix-feeder configuration.
type Config struct {
	ChainID        string            `yaml:"chain_id"`
	NodeGRPC       string            `yaml:"node_grpc"`
	Validator      string            `yaml:"validator"`
	KeyName        string            `yaml:"key_name"`
	KeyringBackend string            `yaml:"keyring_backend"`
	KeyringDir     string            `yaml:"keyring_dir"`
	Fees           string            `yaml:"fees"`
	Gas            string            `yaml:"gas"`
	GasAdjustment  float64           `yaml:"gas_adjustment"`
	FeedInterval   time.Duration     `yaml:"feed_interval"`
	SubmitPoll     time.Duration     `yaml:"submit_poll_interval"`
	VoteWindow     int64             `yaml:"vote_window"`
	BroadcastRetry int               `yaml:"broadcast_retries"`
	Quality        QualityConfig     `yaml:"quality"`
	Providers      []string          `yaml:"providers"`
	StaticPrices   map[string]string `yaml:"static_prices"`
	Pairs          []PairConfig      `yaml:"pairs"`
	Prometheus     PrometheusConfig  `yaml:"prometheus"`
	LogLevel       string            `yaml:"log_level"`
}

// LoadConfig reads and unmarshals a YAML config file.
func LoadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

func (c *Config) providerEnabled(name string) bool {
	for _, p := range c.Providers {
		if p == name {
			return true
		}
	}
	return false
}

// Validate enforces the startup-fatal invariants (design §5).
func (c *Config) Validate() error {
	if c.ChainID == "" || c.NodeGRPC == "" || c.KeyName == "" {
		return fmt.Errorf("chain_id, node_grpc, and key_name are required")
	}
	if _, err := sdk.ValAddressFromBech32(c.Validator); err != nil {
		return fmt.Errorf("validator must be a vtxvaloper address: %w", err)
	}
	if c.FeedInterval <= 0 || c.SubmitPoll <= 0 || c.Quality.MaxQuoteAge <= 0 {
		return fmt.Errorf("feed_interval, submit_poll_interval, and quality.max_quote_age must be positive")
	}
	if c.Quality.MinProviders < 1 {
		return fmt.Errorf("quality.min_providers must be >= 1")
	}
	dev, err := math.LegacyNewDecFromStr(c.Quality.MaxDeviation)
	if err != nil || !dev.IsPositive() || dev.GT(math.LegacyOneDec()) {
		return fmt.Errorf("quality.max_deviation must be a decimal in (0, 1]")
	}
	if _, err := sdk.ParseCoinsNormalized(c.Fees); err != nil {
		return fmt.Errorf("fees must be valid coins: %w", err)
	}
	if c.Gas != "auto" {
		gas, ok := math.NewIntFromString(c.Gas)
		if !ok || !gas.IsPositive() {
			return fmt.Errorf("gas must be \"auto\" or a positive integer")
		}
	} else if c.GasAdjustment <= 1 {
		return fmt.Errorf("gas_adjustment must be > 1 when gas is auto")
	}
	if len(c.Pairs) == 0 {
		return fmt.Errorf("at least one pair is required")
	}
	for _, pc := range c.Pairs {
		if !pairRegex.MatchString(pc.Pair) {
			return fmt.Errorf("pair %q must be BASE:QUOTE uppercase", pc.Pair)
		}
		hasEnabled := false
		for prov := range pc.Symbols {
			if c.providerEnabled(prov) {
				hasEnabled = true
			}
		}
		if !hasEnabled {
			return fmt.Errorf("pair %q has no symbol for an enabled provider", pc.Pair)
		}
		if _, ok := pc.Symbols["static"]; ok && c.providerEnabled("static") {
			ps, found := c.StaticPrices[pc.Symbols["static"]]
			if !found {
				return fmt.Errorf("static provider enabled for %q but no static_prices entry", pc.Pair)
			}
			if p, err := math.LegacyNewDecFromStr(ps); err != nil || !p.IsPositive() {
				return fmt.Errorf("static_prices[%q] must be a positive decimal", pc.Symbols["static"])
			}
		}
	}
	return nil
}
