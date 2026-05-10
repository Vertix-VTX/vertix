# vertix-feeder Sidecar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `vertix-feeder` — a standalone Go binary that validators run alongside their node. It fetches prices from external APIs, signs `MsgSubmitFeed` transactions, and broadcasts them every oracle vote window. Validators who don't run it will have misses counted by `x/oracle`.

**Architecture:** The feeder is a long-running process with a configurable ticker (default: every 5 seconds, within each 10-block window). It fetches prices from one or more provider APIs (CoinGecko, Binance, CoinMarketCap), computes a median across sources, and broadcasts `MsgSubmitFeed` via the chain's gRPC/LCD endpoint using the validator's operator key. It exposes Prometheus metrics for monitoring.

**Tech Stack:** Go 1.22+, `github.com/cosmos/cosmos-sdk` client libraries, `cosmossdk.io/client/v2`, `github.com/spf13/cobra`, `github.com/prometheus/client_golang`

**Prerequisite:** Plan 02 (`x/oracle`) must be complete. `MsgSubmitFeed` must be registered and the chain must boot.

---

## File Structure

```
feeder/
├── cmd/
│   └── vertix-feeder/
│       └── main.go        — binary entry point, cobra root
├── feeder/
│   ├── config.go          — Config struct, YAML parsing, validation
│   ├── feeder.go          — main loop: tick → fetch → median → broadcast
│   ├── provider/
│   │   ├── provider.go    — PriceProvider interface
│   │   ├── coingecko.go   — CoinGecko REST API provider
│   │   ├── binance.go     — Binance spot price provider
│   │   └── provider_test.go — mock provider unit tests
│   ├── broadcaster.go     — sign and broadcast MsgSubmitFeed via gRPC
│   ├── metrics.go         — Prometheus counters/gauges
│   └── feeder_test.go     — integration: fetch + median + mock broadcast
└── feeder-config.example.yaml  — documented example configuration
```

---

## Task 1: Initialize Feeder Module

**Files:** `feeder/` directory structure

- [ ] **Step 1: Create module directory**

```bash
mkdir -p feeder/cmd/vertix-feeder feeder/feeder/provider
```

- [ ] **Step 2: Add feeder to go.mod workspace (or as a sub-module)**

The feeder can share the main module or be a separate `go.mod`. For simplicity, add it as a subdirectory in the main module:

```bash
# Verify feeder imports are available in the main module
# No separate go.mod needed — feeder/ lives inside the vertix module
echo "feeder/ added to vertix module"
```

- [ ] **Step 3: Add Prometheus and YAML dependencies**

```bash
go get github.com/prometheus/client_golang@latest
go get gopkg.in/yaml.v3@latest
go mod tidy
```

---

## Task 2: Implement Config

**Files:**
- Create: `feeder/feeder/config.go`
- Create: `feeder-config.example.yaml`

- [ ] **Step 1: Write config.go**

```go
package feeder

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all feeder configuration loaded from YAML.
type Config struct {
	ChainID      string        `yaml:"chain_id"`
	NodeGRPC     string        `yaml:"node_grpc"`    // e.g. "localhost:9090"
	NodeLCD      string        `yaml:"node_lcd"`     // e.g. "http://localhost:1317"
	ValidatorKey string        `yaml:"validator_key"` // bech32 operator address
	KeyName      string        `yaml:"key_name"`     // keyring key name
	KeyringDir   string        `yaml:"keyring_dir"`  // path to keyring
	KeyringBackend string      `yaml:"keyring_backend"` // "file", "os", "test"
	FeedInterval time.Duration `yaml:"feed_interval"` // e.g. "5s"
	Pairs        []PairConfig  `yaml:"pairs"`
	Providers    []string      `yaml:"providers"` // e.g. ["coingecko", "binance"]
	Prometheus   PrometheusConfig `yaml:"prometheus"`
}

type PairConfig struct {
	Pair     string            `yaml:"pair"`     // e.g. "VTX:USD"
	Symbols  map[string]string `yaml:"symbols"`  // provider -> symbol mapping
}

type PrometheusConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"` // default 9200
}

func (c Config) Validate() error {
	if c.ChainID == "" {
		return fmt.Errorf("chain_id must not be empty")
	}
	if c.NodeGRPC == "" {
		return fmt.Errorf("node_grpc must not be empty")
	}
	if c.ValidatorKey == "" {
		return fmt.Errorf("validator_key must not be empty")
	}
	if len(c.Pairs) == 0 {
		return fmt.Errorf("at least one pair must be configured")
	}
	if c.FeedInterval <= 0 {
		return fmt.Errorf("feed_interval must be positive")
	}
	return nil
}

func LoadConfig(path string) (Config, error) {
	bz, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(bz, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.FeedInterval == 0 {
		cfg.FeedInterval = 5 * time.Second
	}
	if cfg.Prometheus.Port == 0 {
		cfg.Prometheus.Port = 9200
	}
	if cfg.KeyringBackend == "" {
		cfg.KeyringBackend = "file"
	}
	return cfg, cfg.Validate()
}
```

- [ ] **Step 2: Write feeder-config.example.yaml**

```yaml
# vertix-feeder configuration example
# Copy to feeder-config.yaml and fill in your values.

chain_id: vertix-1
node_grpc: "localhost:9090"
node_lcd: "http://localhost:1317"

# Your validator operator key (bech32 vtx-prefixed operator address)
validator_key: "vtx1your_operator_address_here"
key_name: "validator"
keyring_dir: "/home/validator/.vertix"
keyring_backend: "file"   # "file", "os", or "test" (dev only)

# How often to submit feeds (must be well within the oracle vote window)
# With VoteWindow=10 blocks and ~6s block time = 60s window, 10s interval is safe
feed_interval: "10s"

# Oracle pairs to feed. "symbols" maps the pair to each provider's ticker symbol.
pairs:
  - pair: "VTX:USD"
    symbols:
      coingecko: "vertix"      # CoinGecko coin ID
      binance: "VTXUSDT"       # Binance trading pair

  - pair: "BTC:USD"
    symbols:
      coingecko: "bitcoin"
      binance: "BTCUSDT"

  - pair: "ETH:USD"
    symbols:
      coingecko: "ethereum"
      binance: "ETHUSDT"

  - pair: "ATOM:USD"
    symbols:
      coingecko: "cosmos"
      binance: "ATOMUSDT"

  - pair: "USDC:USD"
    symbols:
      coingecko: "usd-coin"
      binance: "USDCUSDT"

# Active price providers (median across providers is submitted)
providers:
  - coingecko
  - binance

prometheus:
  enabled: true
  port: 9200
```

- [ ] **Step 3: Write config unit test**

Create `feeder/feeder/config_test.go`:

```go
package feeder_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vertix-network/vertix/feeder/feeder"
)

func TestLoadConfig(t *testing.T) {
	yaml := `
chain_id: vertix-1
node_grpc: "localhost:9090"
node_lcd: "http://localhost:1317"
validator_key: "vtx1qnk2n4nlkpw9xfqntladh74er2xa62wdl82dyn"
key_name: "validator"
keyring_dir: "/tmp"
keyring_backend: "test"
feed_interval: "5s"
pairs:
  - pair: "VTX:USD"
    symbols:
      coingecko: "vertix"
providers:
  - coingecko
prometheus:
  enabled: true
  port: 9200
`
	f, _ := os.CreateTemp("", "feeder-config-*.yaml")
	f.WriteString(yaml)
	f.Close()
	defer os.Remove(f.Name())

	cfg, err := feeder.LoadConfig(f.Name())
	require.NoError(t, err)
	require.Equal(t, "vertix-1", cfg.ChainID)
	require.Equal(t, 5*time.Second, cfg.FeedInterval)
	require.Len(t, cfg.Pairs, 1)
	require.Equal(t, "VTX:USD", cfg.Pairs[0].Pair)
}

func TestConfigValidate(t *testing.T) {
	bad := feeder.Config{} // empty
	require.Error(t, bad.Validate())

	good := feeder.Config{
		ChainID:      "vertix-1",
		NodeGRPC:     "localhost:9090",
		ValidatorKey: "vtx1xxx",
		FeedInterval: 5 * 1e9,
		Pairs:        []feeder.PairConfig{{Pair: "VTX:USD"}},
	}
	require.NoError(t, good.Validate())
}
```

```bash
go test ./feeder/feeder/... -run TestLoadConfig -v
go test ./feeder/feeder/... -run TestConfigValidate -v
```

Expected: Both PASS.

---

## Task 3: Implement Price Providers

**Files:**
- Create: `feeder/feeder/provider/provider.go`
- Create: `feeder/feeder/provider/coingecko.go`
- Create: `feeder/feeder/provider/binance.go`
- Create: `feeder/feeder/provider/provider_test.go`

- [ ] **Step 1: Write provider.go (interface)**

```go
package provider

import (
	"context"
	"cosmossdk.io/math"
)

// PriceProvider fetches the spot price for a given symbol from an external source.
type PriceProvider interface {
	// Name returns the provider identifier (e.g. "coingecko").
	Name() string
	// GetPrice fetches the USD price for the given provider-specific symbol.
	// Returns an error if the price cannot be fetched.
	GetPrice(ctx context.Context, symbol string) (math.LegacyDec, error)
}
```

- [ ] **Step 2: Write coingecko.go**

```go
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cosmossdk.io/math"
)

type CoinGecko struct {
	baseURL    string
	httpClient *http.Client
}

func NewCoinGecko() *CoinGecko {
	return &CoinGecko{
		baseURL:    "https://api.coingecko.com/api/v3",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *CoinGecko) Name() string { return "coingecko" }

func (c *CoinGecko) GetPrice(ctx context.Context, coinID string) (math.LegacyDec, error) {
	url := fmt.Sprintf("%s/simple/price?ids=%s&vs_currencies=usd", c.baseURL, coinID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return math.LegacyZeroDec(), fmt.Errorf("coingecko request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return math.LegacyZeroDec(), fmt.Errorf("coingecko fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return math.LegacyZeroDec(), fmt.Errorf("coingecko status %d for %s", resp.StatusCode, coinID)
	}

	var result map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return math.LegacyZeroDec(), fmt.Errorf("coingecko decode: %w", err)
	}

	usdPrice, ok := result[coinID]["usd"]
	if !ok {
		return math.LegacyZeroDec(), fmt.Errorf("coingecko: no USD price for %s", coinID)
	}

	price, err := math.LegacyNewDecFromStr(fmt.Sprintf("%.8f", usdPrice))
	if err != nil {
		return math.LegacyZeroDec(), fmt.Errorf("coingecko price parse: %w", err)
	}
	return price, nil
}
```

- [ ] **Step 3: Write binance.go**

```go
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cosmossdk.io/math"
)

type Binance struct {
	httpClient *http.Client
}

func NewBinance() *Binance {
	return &Binance{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (b *Binance) Name() string { return "binance" }

func (b *Binance) GetPrice(ctx context.Context, symbol string) (math.LegacyDec, error) {
	url := fmt.Sprintf("https://api.binance.com/api/v3/ticker/price?symbol=%s", symbol)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return math.LegacyZeroDec(), fmt.Errorf("binance request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return math.LegacyZeroDec(), fmt.Errorf("binance fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return math.LegacyZeroDec(), fmt.Errorf("binance status %d for %s", resp.StatusCode, symbol)
	}

	var result struct {
		Price string `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return math.LegacyZeroDec(), fmt.Errorf("binance decode: %w", err)
	}

	price, err := math.LegacyNewDecFromStr(result.Price)
	if err != nil {
		return math.LegacyZeroDec(), fmt.Errorf("binance price parse: %w", err)
	}
	return price, nil
}
```

- [ ] **Step 4: Write provider_test.go with mock provider**

```go
package provider_test

import (
	"context"
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/feeder/feeder/provider"
)

// mockProvider returns a fixed price for testing.
type mockProvider struct {
	name  string
	price math.LegacyDec
	err   error
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) GetPrice(_ context.Context, _ string) (math.LegacyDec, error) {
	return m.price, m.err
}

func TestMedianAcrossProviders(t *testing.T) {
	providers := []provider.PriceProvider{
		&mockProvider{name: "p1", price: math.LegacyMustNewDecFromStr("100.0")},
		&mockProvider{name: "p2", price: math.LegacyMustNewDecFromStr("102.0")},
		&mockProvider{name: "p3", price: math.LegacyMustNewDecFromStr("98.0")},
	}

	prices := make([]math.LegacyDec, 0, len(providers))
	for _, p := range providers {
		price, err := p.GetPrice(context.Background(), "BTC")
		require.NoError(t, err)
		prices = append(prices, price)
	}

	median := provider.Median(prices)
	require.Equal(t, math.LegacyMustNewDecFromStr("100.0"), median)
}

func TestMedianSingleProvider(t *testing.T) {
	prices := []math.LegacyDec{math.LegacyMustNewDecFromStr("55.5")}
	require.Equal(t, math.LegacyMustNewDecFromStr("55.5"), provider.Median(prices))
}
```

- [ ] **Step 5: Add Median function to provider.go**

Append to `feeder/feeder/provider/provider.go`:

```go
import "sort"

// Median returns the median of a slice of prices (simple, unweighted).
// Used to aggregate prices across multiple providers before submission.
func Median(prices []math.LegacyDec) math.LegacyDec {
	if len(prices) == 0 {
		return math.LegacyZeroDec()
	}
	sorted := make([]math.LegacyDec, len(prices))
	copy(sorted, prices)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].LT(sorted[j]) })
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return sorted[mid-1].Add(sorted[mid]).QuoInt64(2)
	}
	return sorted[mid]
}
```

```bash
go test ./feeder/feeder/provider/... -v
```

Expected: Both tests PASS.

---

## Task 4: Implement Broadcaster

**Files:**
- Create: `feeder/feeder/broadcaster.go`

- [ ] **Step 1: Write broadcaster.go**

```go
package feeder

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	oracletypes "github.com/vertix-network/vertix/x/oracle/types"
)

// Broadcaster signs and broadcasts MsgSubmitFeed to the chain.
type Broadcaster struct {
	clientCtx client.Context
	keyName   string
	chainID   string
}

// NewBroadcaster initializes a broadcaster with the validator's key.
func NewBroadcaster(cfg Config) (*Broadcaster, error) {
	encodingCfg := makeEncodingConfig()

	kr, err := keyring.New(
		"vertix",
		cfg.KeyringBackend,
		cfg.KeyringDir,
		nil,
		encodingCfg.Codec,
	)
	if err != nil {
		return nil, fmt.Errorf("init keyring: %w", err)
	}

	clientCtx := client.Context{}.
		WithChainID(cfg.ChainID).
		WithCodec(encodingCfg.Codec).
		WithInterfaceRegistry(encodingCfg.InterfaceRegistry).
		WithKeyring(kr).
		WithNodeURI(cfg.NodeLCD).
		WithBroadcastMode("sync")

	return &Broadcaster{
		clientCtx: clientCtx,
		keyName:   cfg.KeyName,
		chainID:   cfg.ChainID,
	}, nil
}

// BroadcastFeed signs and broadcasts a single MsgSubmitFeed.
func (b *Broadcaster) BroadcastFeed(ctx context.Context, pair string, price math.LegacyDec) error {
	info, err := b.clientCtx.Keyring.Key(b.keyName)
	if err != nil {
		return fmt.Errorf("get key %q: %w", b.keyName, err)
	}
	addr, err := info.GetAddress()
	if err != nil {
		return fmt.Errorf("get address: %w", err)
	}

	msg := &oracletypes.MsgSubmitFeed{
		Validator: addr.String(),
		Pair:      pair,
		Price:     price.String(),
	}
	if err := msg.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid feed msg: %w", err)
	}

	txf := tx.Factory{}.
		WithChainID(b.chainID).
		WithKeybase(b.clientCtx.Keyring).
		WithTxConfig(b.clientCtx.TxConfig).
		WithSignMode(signing.SignMode_SIGN_MODE_DIRECT).
		WithGasAdjustment(1.5).
		WithGas(200_000)

	if err := tx.BroadcastTx(b.clientCtx, txf, msg); err != nil {
		return fmt.Errorf("broadcast MsgSubmitFeed pair=%s price=%s: %w", pair, price, err)
	}
	return nil
}
```

- [ ] **Step 2: Add encoding config helper**

Create `feeder/feeder/encoding.go`:

```go
package feeder

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
)

type EncodingConfig struct {
	Codec             codec.Codec
	InterfaceRegistry codectypes.InterfaceRegistry
	TxConfig          client.TxConfig
}

func makeEncodingConfig() EncodingConfig {
	ir := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(ir)
	cdc := codec.NewProtoCodec(ir)
	txCfg := authtx.NewTxConfig(cdc, authtx.DefaultSignModes)
	return EncodingConfig{Codec: cdc, InterfaceRegistry: ir, TxConfig: txCfg}
}
```

- [ ] **Step 3: Compile check**

```bash
go build ./feeder/...
```

Expected: No errors.

---

## Task 5: Implement Prometheus Metrics

**Files:**
- Create: `feeder/feeder/metrics.go`

- [ ] **Step 1: Write metrics.go**

```go
package feeder

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	feedsSubmitted = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "vertix_feeder_feeds_submitted_total",
		Help: "Total number of oracle feeds successfully submitted.",
	}, []string{"pair"})

	feedErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "vertix_feeder_feed_errors_total",
		Help: "Total number of feed submission errors.",
	}, []string{"pair", "source"})

	lastPrice = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vertix_feeder_last_price",
		Help: "Last submitted price per pair.",
	}, []string{"pair"})

	fetchDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vertix_feeder_fetch_duration_seconds",
		Help:    "Price fetch duration per provider.",
		Buckets: prometheus.DefBuckets,
	}, []string{"provider"})
)

func startMetricsServer(port int) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}
```

- [ ] **Step 2: Compile check**

```bash
go build ./feeder/...
```

---

## Task 6: Implement Main Feeder Loop

**Files:**
- Create: `feeder/feeder/feeder.go`

- [ ] **Step 1: Write feeder.go**

```go
package feeder

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cosmossdk.io/math"
	"github.com/vertix-network/vertix/feeder/feeder/provider"
)

// Feeder orchestrates price fetching, aggregation, and broadcasting.
type Feeder struct {
	cfg         Config
	providers   map[string]provider.PriceProvider
	broadcaster *Broadcaster
	logger      *slog.Logger
}

// New creates a Feeder from a Config.
func New(cfg Config, logger *slog.Logger) (*Feeder, error) {
	providerMap := map[string]provider.PriceProvider{
		"coingecko": provider.NewCoinGecko(),
		"binance":   provider.NewBinance(),
	}
	active := make(map[string]provider.PriceProvider)
	for _, name := range cfg.Providers {
		p, ok := providerMap[name]
		if !ok {
			return nil, fmt.Errorf("unknown provider: %s (supported: coingecko, binance)", name)
		}
		active[name] = p
	}

	bc, err := NewBroadcaster(cfg)
	if err != nil {
		return nil, fmt.Errorf("init broadcaster: %w", err)
	}

	return &Feeder{
		cfg:         cfg,
		providers:   active,
		broadcaster: bc,
		logger:      logger,
	}, nil
}

// Run starts the main feed loop. Blocks until ctx is cancelled.
func (f *Feeder) Run(ctx context.Context) error {
	if f.cfg.Prometheus.Enabled {
		startMetricsServer(f.cfg.Prometheus.Port)
		f.logger.Info("prometheus metrics server started", "port", f.cfg.Prometheus.Port)
	}

	ticker := time.NewTicker(f.cfg.FeedInterval)
	defer ticker.Stop()

	f.logger.Info("feeder started",
		"chain_id", f.cfg.ChainID,
		"pairs", len(f.cfg.Pairs),
		"providers", len(f.providers),
		"interval", f.cfg.FeedInterval,
	)

	for {
		select {
		case <-ctx.Done():
			f.logger.Info("feeder shutting down")
			return ctx.Err()
		case <-ticker.C:
			f.feedOnce(ctx)
		}
	}
}

// feedOnce runs one feed cycle: fetch all pairs, aggregate, broadcast.
func (f *Feeder) feedOnce(ctx context.Context) {
	for _, pairCfg := range f.cfg.Pairs {
		prices, err := f.fetchPrices(ctx, pairCfg)
		if err != nil {
			f.logger.Error("fetch failed", "pair", pairCfg.Pair, "err", err)
			feedErrors.WithLabelValues(pairCfg.Pair, "fetch").Inc()
			continue
		}
		if len(prices) == 0 {
			f.logger.Warn("no prices fetched", "pair", pairCfg.Pair)
			continue
		}

		median := provider.Median(prices)
		f.logger.Debug("submitting feed", "pair", pairCfg.Pair, "price", median, "sources", len(prices))

		if err := f.broadcaster.BroadcastFeed(ctx, pairCfg.Pair, median); err != nil {
			f.logger.Error("broadcast failed", "pair", pairCfg.Pair, "err", err)
			feedErrors.WithLabelValues(pairCfg.Pair, "broadcast").Inc()
			continue
		}

		feedsSubmitted.WithLabelValues(pairCfg.Pair).Inc()
		f64, _ := median.Float64()
		lastPrice.WithLabelValues(pairCfg.Pair).Set(f64)
		f.logger.Info("feed submitted", "pair", pairCfg.Pair, "price", median)
	}
}

// fetchPrices collects prices from all configured providers for one pair.
func (f *Feeder) fetchPrices(ctx context.Context, pairCfg PairConfig) ([]math.LegacyDec, error) {
	var prices []math.LegacyDec
	for providerName, symbol := range pairCfg.Symbols {
		p, ok := f.providers[providerName]
		if !ok {
			continue
		}
		start := time.Now()
		price, err := p.GetPrice(ctx, symbol)
		fetchDuration.WithLabelValues(providerName).Observe(time.Since(start).Seconds())
		if err != nil {
			f.logger.Warn("provider error", "provider", providerName, "pair", pairCfg.Pair, "err", err)
			feedErrors.WithLabelValues(pairCfg.Pair, providerName).Inc()
			continue // use other providers
		}
		prices = append(prices, price)
	}
	return prices, nil
}
```

- [ ] **Step 2: Write feeder_test.go**

Create `feeder/feeder/feeder_test.go`:

```go
package feeder_test

import (
	"context"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"
	"github.com/vertix-network/vertix/feeder/feeder/provider"
)

func TestMedianAggregation(t *testing.T) {
	prices := []math.LegacyDec{
		math.LegacyMustNewDecFromStr("100.0"),
		math.LegacyMustNewDecFromStr("102.0"),
		math.LegacyMustNewDecFromStr("98.0"),
	}
	median := provider.Median(prices)
	require.Equal(t, math.LegacyMustNewDecFromStr("100.0"), median)
}

func TestMedianEvenCount(t *testing.T) {
	prices := []math.LegacyDec{
		math.LegacyMustNewDecFromStr("100.0"),
		math.LegacyMustNewDecFromStr("102.0"),
	}
	median := provider.Median(prices)
	require.Equal(t, math.LegacyMustNewDecFromStr("101.0"), median)
}
```

```bash
go test ./feeder/... -v
```

Expected: PASS.

---

## Task 7: Implement CLI Entry Point

**Files:**
- Create: `feeder/cmd/vertix-feeder/main.go`

- [ ] **Step 1: Write main.go**

```go
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/vertix-network/vertix/feeder/feeder"
)

func main() {
	root := &cobra.Command{
		Use:   "vertix-feeder",
		Short: "Vertix oracle price feeder for validators",
		Long: `vertix-feeder fetches prices from external APIs and submits
MsgSubmitFeed transactions to the Vertix chain each oracle vote window.
Validators must run this alongside their node to avoid oracle miss slashing.`,
	}

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the oracle feeder",
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			logLevel, _ := cmd.Flags().GetString("log-level")

			logger := newLogger(logLevel)

			cfg, err := feeder.LoadConfig(configPath)
			if err != nil {
				return err
			}

			f, err := feeder.New(cfg, logger)
			if err != nil {
				return err
			}

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			logger.Info("starting vertix-feeder", "config", configPath, "chain_id", cfg.ChainID)
			return f.Run(ctx)
		},
	}
	startCmd.Flags().String("config", "feeder-config.yaml", "path to feeder config file")
	startCmd.Flags().String("log-level", "info", "log level: debug, info, warn, error")

	root.AddCommand(startCmd)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
```

- [ ] **Step 2: Add feeder to Makefile**

Add to `Makefile`:

```makefile
## Build the vertix-feeder binary
build-feeder:
	@echo "Building vertix-feeder..."
	@go build -o $(BUILD_DIR)/vertix-feeder ./feeder/cmd/vertix-feeder
	@echo "Binary at $(BUILD_DIR)/vertix-feeder"
```

Update the `all` target:

```makefile
all: lint test build build-feeder
```

- [ ] **Step 3: Build and test**

```bash
make build-feeder
build/vertix-feeder --help
build/vertix-feeder start --help
```

Expected: Help text shows, binary runs.

```bash
go test ./feeder/... -v -count=1
```

Expected: All tests PASS.

---

## Task 8: Add Feeder Docs

**Files:**
- Create: `docs/feeder.md`

- [ ] **Step 1: Write docs/feeder.md**

```markdown
# Vertix Oracle Feeder

The `vertix-feeder` binary is required for all active Vertix validators. Validators who fail to submit oracle feeds within the vote window will have their miss counter incremented. Exceeding the miss threshold results in oracle slashing (0.5% of bonded stake).

## Installation

```bash
make build-feeder
sudo cp build/vertix-feeder /usr/local/bin/
```

## Configuration

Copy the example config and fill in your values:

```bash
cp feeder-config.example.yaml feeder-config.yaml
# Edit feeder-config.yaml with your chain_id, node endpoints, and key info
```

## Running as a systemd service

Create `/etc/systemd/system/vertix-feeder.service`:

```ini
[Unit]
Description=Vertix Oracle Feeder
After=network.target vertixd.service

[Service]
User=validator
ExecStart=/usr/local/bin/vertix-feeder start --config /home/validator/feeder-config.yaml
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable vertix-feeder
sudo systemctl start vertix-feeder
sudo journalctl -fu vertix-feeder
```

## Prometheus Metrics

The feeder exposes metrics at `http://localhost:9200/metrics`:

| Metric | Description |
|---|---|
| `vertix_feeder_feeds_submitted_total` | Total successful feeds per pair |
| `vertix_feeder_feed_errors_total` | Total errors per pair and source |
| `vertix_feeder_last_price` | Last submitted price per pair |
| `vertix_feeder_fetch_duration_seconds` | Price fetch latency per provider |
```

---

## Task 9: Final Verification

- [ ] **Step 1: Run all feeder tests**

```bash
go test ./feeder/... -v -count=1 -timeout 2m
```

Expected: All PASS.

- [ ] **Step 2: Binary smoke test**

```bash
make build-feeder
build/vertix-feeder start --config /nonexistent.yaml 2>&1 | grep -i "read config"
```

Expected: Error message about missing config file (not a panic).

- [ ] **Step 3: Final commit**

```bash
git add .
git commit -m "chore: plan 05 complete — vertix-feeder oracle sidecar ready"
```

---

*Next plan: `docs/plans/2026-05-09-06-ibc-devnet.md` — IBC enablement and devnet setup*
