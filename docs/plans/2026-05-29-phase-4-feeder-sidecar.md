# `vertix-feeder` Sidecar (Phase 4) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the off-chain `vertix-feeder` Go binary that fetches external prices, computes a quality-gated cross-source median per pair, and broadcasts one batched `MsgSubmitFeed` transaction per oracle vote window — signed by the delegated feeder key — completing the Phase 1 oracle loop.

**Architecture:** A hybrid two-loop design in the existing chain Go module (`github.com/vertix-network/vertix`). A **price loop** ticks on `feed_interval`, fetches from pluggable providers (CoinGecko, Binance, Static), filters by data-quality guards, and caches the median per pair. An independent **submit loop** polls block height and, once per `VoteWindow`, builds and broadcasts a single batched, feeder-signed tx via the Cosmos SDK client stack (gRPC). Prometheus metrics on `:9200`. Resilient: never crashes on transient errors.

**Tech Stack:** Go 1.25 (pinned `GOTOOLCHAIN=go1.25.4`), Cosmos SDK v0.50.14 (`client`, `tx.Factory`, keyring, gRPC tx/auth/cmt service clients), `cosmossdk.io/math.LegacyDec`, `github.com/prometheus/client_golang`, `gopkg.in/yaml.v3`, `github.com/spf13/cobra`, `github.com/stretchr/testify`.

**Source spec:** [`docs/specs/2026-05-29-phase-4-feeder-sidecar-design.md`](../specs/2026-05-29-phase-4-feeder-sidecar-design.md)

---

## Conventions for every task

- Run all Go commands from the repo root with the pinned toolchain already exported by the Makefile environment. If running `go` directly, prefix with `GOTOOLCHAIN=go1.25.4`.
- Package name for everything under `feeder/feeder/` is `feeder`. Package for `feeder/feeder/provider/` is `provider`. Package for `feeder/cmd/vertix-feeder/` is `main`.
- Import the chain's oracle types as `oracletypes "github.com/vertix-network/vertix/x/oracle/types"`. Never redefine `MsgSubmitFeed`.
- Money/prices are always `cosmossdk.io/math.LegacyDec`. Never `float64` in domain logic (parse provider JSON numbers to string → `math.LegacyNewDecFromStr`).
- TDD: write the failing test, run it red, implement minimally, run it green, commit.
- Commit messages use Conventional Commits with the `feeder` scope, e.g. `feat(feeder): add cross-source median aggregation`.

---

## File Structure

| File | Responsibility |
|---|---|
| `feeder/feeder/config.go` | `Config` struct, YAML load, `Validate()` |
| `feeder/feeder/provider/provider.go` | `PriceProvider` interface + name→constructor registry |
| `feeder/feeder/provider/static.go` | Static/manual provider (VTX:USD bootstrap + tests) |
| `feeder/feeder/provider/coingecko.go` | CoinGecko REST adapter |
| `feeder/feeder/provider/binance.go` | Binance spot REST adapter |
| `feeder/feeder/aggregate.go` | `Median`, `DropDeviating`, `CrossSourceMedian` |
| `feeder/feeder/cache.go` | Concurrency-safe latest-median-per-pair store |
| `feeder/feeder/metrics.go` | Prometheus metrics + `:9200` HTTP server |
| `feeder/feeder/broadcaster.go` | `Broadcaster` interface, `FeedSubmission`, SDK gRPC impl, codec |
| `feeder/feeder/priceloop.go` | Price loop (fetch → filter → median → cache) |
| `feeder/feeder/submitloop.go` | Submit loop (height → window → batch → broadcast) |
| `feeder/feeder/feeder.go` | Orchestrator: wire providers/cache/loops, lifecycle |
| `feeder/cmd/vertix-feeder/main.go` | Cobra root, flags, signal handling |
| `feeder-config.example.yaml` | Documented example config |
| `Makefile` | `feeder-build` target (modify) |

---

## Task 1: Package skeleton + doc stub

**Files:**
- Create: `feeder/feeder/doc.go`

- [ ] **Step 1: Create the package doc file**

```go
// Package feeder implements the vertix-feeder oracle sidecar: it fetches
// external prices, computes a quality-gated cross-source median per pair, and
// broadcasts one batched MsgSubmitFeed transaction per oracle vote window.
package feeder
```

- [ ] **Step 2: Verify it compiles**

Run: `GOTOOLCHAIN=go1.25.4 go build ./feeder/...`
Expected: success, no output (empty package builds).

- [ ] **Step 3: Commit**

```bash
git add feeder/feeder/doc.go
git commit -m "chore(feeder): add feeder package skeleton"
```

---

## Task 2: Config struct, loader, and validation

**Files:**
- Create: `feeder/feeder/config.go`
- Test: `feeder/feeder/config_test.go`

- [ ] **Step 1: Write the failing test**

```go
package feeder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const validYAML = `
chain_id: vertix-devnet-1
node_grpc: localhost:9090
validator: vtxvaloper1xyz
key_name: feeder
keyring_backend: test
keyring_dir: /tmp/.vertix
fees: 2000uvtx
gas: "200000"
gas_adjustment: 1.3
feed_interval: 5s
submit_poll_interval: 1s
vote_window: 0
broadcast_retries: 3
quality:
  min_providers: 2
  max_deviation: "0.10"
  max_quote_age: 30s
providers: ["coingecko", "binance", "static"]
static_prices:
  "VTX:USD": "0.10"
pairs:
  - pair: "VTX:USD"
    symbols: { static: "VTX:USD" }
  - pair: "BTC:USD"
    symbols: { coingecko: "bitcoin", binance: "BTCUSDT" }
prometheus: { enabled: true, port: 9200 }
log_level: info
`

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	return p
}

func TestLoadConfigValid(t *testing.T) {
	cfg, err := LoadConfig(writeTemp(t, validYAML))
	require.NoError(t, err)
	require.Equal(t, "vertix-devnet-1", cfg.ChainID)
	require.Equal(t, 2, cfg.Quality.MinProviders)
	require.Len(t, cfg.Pairs, 2)
	require.Equal(t, "0.10", cfg.StaticPrices["VTX:USD"])
	require.NoError(t, cfg.Validate())
}

func TestValidateRejectsBadPair(t *testing.T) {
	body := validYAML + "\n" // start from valid, then mutate via reload
	cfg, err := LoadConfig(writeTemp(t, body))
	require.NoError(t, err)
	cfg.Pairs[1].Pair = "btc-usd" // not BASE:QUOTE
	err = cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "pair")
}

func TestValidateRejectsMissingStaticPrice(t *testing.T) {
	cfg, err := LoadConfig(writeTemp(t, validYAML))
	require.NoError(t, err)
	delete(cfg.StaticPrices, "VTX:USD")
	err = cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "static")
}

func TestValidateRejectsBadDeviation(t *testing.T) {
	cfg, err := LoadConfig(writeTemp(t, validYAML))
	require.NoError(t, err)
	cfg.Quality.MaxDeviation = "2.0" // > 1
	require.Error(t, cfg.Validate())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/ -run TestLoadConfig -v`
Expected: FAIL — `undefined: LoadConfig`.

- [ ] **Step 3: Write the implementation**

```go
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
		if _, err := math.NewIntFromString(c.Gas); !err {
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
```

> Note: `math.NewIntFromString` returns `(Int, bool)`; the `!err` check above treats the bool as ok. If your linter prefers, rename to `_, ok := math.NewIntFromString(c.Gas); if !ok {`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/ -run 'TestLoadConfig|TestValidate' -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add feeder/feeder/config.go feeder/feeder/config_test.go
git commit -m "feat(feeder): add config struct, loader, and validation"
```

---

## Task 3: PriceProvider interface + registry

**Files:**
- Create: `feeder/feeder/provider/provider.go`

- [ ] **Step 1: Write the implementation (interface + registry; no test yet, exercised by adapters)**

```go
package provider

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
)

// PriceProvider fetches a spot price for a provider-specific symbol.
type PriceProvider interface {
	Name() string
	Fetch(ctx context.Context, symbol string) (math.LegacyDec, error)
}

// Constructor builds a provider from optional config (e.g. static prices).
type Constructor func(opts Options) (PriceProvider, error)

// Options carries provider construction inputs.
type Options struct {
	// StaticPrices maps symbol -> decimal string, used by the static provider.
	StaticPrices map[string]string
}

var registry = map[string]Constructor{}

// Register adds a provider constructor under name. Called from adapter init().
func Register(name string, c Constructor) {
	registry[name] = c
}

// Build instantiates the named providers. Unknown names are a fatal error.
func Build(names []string, opts Options) ([]PriceProvider, error) {
	out := make([]PriceProvider, 0, len(names))
	for _, n := range names {
		c, ok := registry[n]
		if !ok {
			return nil, fmt.Errorf("unknown provider %q", n)
		}
		p, err := c(opts)
		if err != nil {
			return nil, fmt.Errorf("init provider %q: %w", n, err)
		}
		out = append(out, p)
	}
	return out, nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `GOTOOLCHAIN=go1.25.4 go build ./feeder/feeder/provider/`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add feeder/feeder/provider/provider.go
git commit -m "feat(feeder): add PriceProvider interface and registry"
```

---

## Task 4: Static provider

**Files:**
- Create: `feeder/feeder/provider/static.go`
- Test: `feeder/feeder/provider/static_test.go`

- [ ] **Step 1: Write the failing test**

```go
package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStaticProviderFetch(t *testing.T) {
	p, err := Build([]string{"static"}, Options{StaticPrices: map[string]string{"VTX:USD": "0.10"}})
	require.NoError(t, err)
	require.Equal(t, "static", p[0].Name())

	price, err := p[0].Fetch(context.Background(), "VTX:USD")
	require.NoError(t, err)
	require.Equal(t, "0.100000000000000000", price.String())
}

func TestStaticProviderUnknownSymbol(t *testing.T) {
	p, _ := Build([]string{"static"}, Options{StaticPrices: map[string]string{"VTX:USD": "0.10"}})
	_, err := p[0].Fetch(context.Background(), "BTC:USD")
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/provider/ -run TestStatic -v`
Expected: FAIL — `unknown provider "static"`.

- [ ] **Step 3: Write the implementation**

```go
package provider

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
)

type staticProvider struct {
	prices map[string]math.LegacyDec
}

func init() {
	Register("static", func(opts Options) (PriceProvider, error) {
		parsed := make(map[string]math.LegacyDec, len(opts.StaticPrices))
		for sym, s := range opts.StaticPrices {
			d, err := math.LegacyNewDecFromStr(s)
			if err != nil {
				return nil, fmt.Errorf("static price %q: %w", sym, err)
			}
			parsed[sym] = d
		}
		return &staticProvider{prices: parsed}, nil
	})
}

func (p *staticProvider) Name() string { return "static" }

func (p *staticProvider) Fetch(_ context.Context, symbol string) (math.LegacyDec, error) {
	d, ok := p.prices[symbol]
	if !ok {
		return math.LegacyDec{}, fmt.Errorf("static: no price for symbol %q", symbol)
	}
	return d, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/provider/ -run TestStatic -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add feeder/feeder/provider/static.go feeder/feeder/provider/static_test.go
git commit -m "feat(feeder): add static price provider"
```

---

## Task 5: CoinGecko provider

**Files:**
- Create: `feeder/feeder/provider/coingecko.go`
- Test: `feeder/feeder/provider/coingecko_test.go`

CoinGecko simple-price endpoint: `GET {base}/api/v3/simple/price?ids={id}&vs_currencies=usd` → `{"bitcoin":{"usd":65000.12}}`. The base URL is overridable for tests.

- [ ] **Step 1: Write the failing test**

```go
package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCoinGeckoFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "bitcoin", r.URL.Query().Get("ids"))
		require.Equal(t, "usd", r.URL.Query().Get("vs_currencies"))
		_, _ = w.Write([]byte(`{"bitcoin":{"usd":65000.12}}`))
	}))
	defer srv.Close()

	p := newCoinGecko(srv.URL)
	require.Equal(t, "coingecko", p.Name())
	price, err := p.Fetch(context.Background(), "bitcoin")
	require.NoError(t, err)
	require.Equal(t, "65000.120000000000000000", price.String())
}

func TestCoinGeckoMissingSymbol(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	_, err := newCoinGecko(srv.URL).Fetch(context.Background(), "bitcoin")
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/provider/ -run TestCoinGecko -v`
Expected: FAIL — `undefined: newCoinGecko`.

- [ ] **Step 3: Write the implementation**

```go
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"cosmossdk.io/math"
)

const coinGeckoBase = "https://api.coingecko.com"

type coinGecko struct {
	base   string
	client *http.Client
}

func init() {
	Register("coingecko", func(Options) (PriceProvider, error) {
		return newCoinGecko(coinGeckoBase), nil
	})
}

func newCoinGecko(base string) *coinGecko {
	return &coinGecko{base: base, client: &http.Client{Timeout: 8 * time.Second}}
}

func (p *coinGecko) Name() string { return "coingecko" }

func (p *coinGecko) Fetch(ctx context.Context, symbol string) (math.LegacyDec, error) {
	u := fmt.Sprintf("%s/api/v3/simple/price?ids=%s&vs_currencies=usd", p.base, url.QueryEscape(symbol))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return math.LegacyDec{}, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return math.LegacyDec{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return math.LegacyDec{}, fmt.Errorf("coingecko: status %d", resp.StatusCode)
	}
	var body map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return math.LegacyDec{}, fmt.Errorf("coingecko: decode: %w", err)
	}
	entry, ok := body[symbol]
	if !ok {
		return math.LegacyDec{}, fmt.Errorf("coingecko: no entry for %q", symbol)
	}
	usd, ok := entry["usd"]
	if !ok {
		return math.LegacyDec{}, fmt.Errorf("coingecko: no usd price for %q", symbol)
	}
	return math.LegacyNewDecFromStr(strconv.FormatFloat(usd, 'f', -1, 64))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/provider/ -run TestCoinGecko -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add feeder/feeder/provider/coingecko.go feeder/feeder/provider/coingecko_test.go
git commit -m "feat(feeder): add coingecko price provider"
```

---

## Task 6: Binance provider

**Files:**
- Create: `feeder/feeder/provider/binance.go`
- Test: `feeder/feeder/provider/binance_test.go`

Binance ticker endpoint: `GET {base}/api/v3/ticker/price?symbol=BTCUSDT` → `{"symbol":"BTCUSDT","price":"65000.10000000"}`.

- [ ] **Step 1: Write the failing test**

```go
package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBinanceFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
		_, _ = w.Write([]byte(`{"symbol":"BTCUSDT","price":"65000.10000000"}`))
	}))
	defer srv.Close()

	p := newBinance(srv.URL)
	require.Equal(t, "binance", p.Name())
	price, err := p.Fetch(context.Background(), "BTCUSDT")
	require.NoError(t, err)
	require.Equal(t, "65000.100000000000000000", price.String())
}

func TestBinanceBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	_, err := newBinance(srv.URL).Fetch(context.Background(), "BTCUSDT")
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/provider/ -run TestBinance -v`
Expected: FAIL — `undefined: newBinance`.

- [ ] **Step 3: Write the implementation**

```go
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"cosmossdk.io/math"
)

const binanceBase = "https://api.binance.com"

type binance struct {
	base   string
	client *http.Client
}

func init() {
	Register("binance", func(Options) (PriceProvider, error) {
		return newBinance(binanceBase), nil
	})
}

func newBinance(base string) *binance {
	return &binance{base: base, client: &http.Client{Timeout: 8 * time.Second}}
}

func (p *binance) Name() string { return "binance" }

func (p *binance) Fetch(ctx context.Context, symbol string) (math.LegacyDec, error) {
	u := fmt.Sprintf("%s/api/v3/ticker/price?symbol=%s", p.base, url.QueryEscape(symbol))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return math.LegacyDec{}, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return math.LegacyDec{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return math.LegacyDec{}, fmt.Errorf("binance: status %d", resp.StatusCode)
	}
	var body struct {
		Price string `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return math.LegacyDec{}, fmt.Errorf("binance: decode: %w", err)
	}
	if body.Price == "" {
		return math.LegacyDec{}, fmt.Errorf("binance: empty price for %q", symbol)
	}
	return math.LegacyNewDecFromStr(body.Price)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/provider/ -run TestBinance -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add feeder/feeder/provider/binance.go feeder/feeder/provider/binance_test.go
git commit -m "feat(feeder): add binance price provider"
```

---

## Task 7: Aggregation (median, deviation drop, cross-source median)

**Files:**
- Create: `feeder/feeder/aggregate.go`
- Test: `feeder/feeder/aggregate_test.go`

- [ ] **Step 1: Write the failing test**

```go
package feeder

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

func dec(t *testing.T, s string) math.LegacyDec {
	t.Helper()
	d, err := math.LegacyNewDecFromStr(s)
	require.NoError(t, err)
	return d
}

func TestMedianOdd(t *testing.T) {
	m, err := Median([]math.LegacyDec{dec(t, "3"), dec(t, "1"), dec(t, "2")})
	require.NoError(t, err)
	require.Equal(t, "2.000000000000000000", m.String())
}

func TestMedianEven(t *testing.T) {
	m, err := Median([]math.LegacyDec{dec(t, "1"), dec(t, "3")})
	require.NoError(t, err)
	require.Equal(t, "2.000000000000000000", m.String())
}

func TestMedianEmpty(t *testing.T) {
	_, err := Median(nil)
	require.Error(t, err)
}

func TestDropDeviating(t *testing.T) {
	// median = 100; 10% band keeps [90,110]; 200 is dropped.
	in := []math.LegacyDec{dec(t, "100"), dec(t, "101"), dec(t, "200")}
	out := DropDeviating(in, dec(t, "0.10"))
	require.Len(t, out, 2)
}

func TestCrossSourceMedianTooFew(t *testing.T) {
	_, err := CrossSourceMedian([]math.LegacyDec{dec(t, "100")}, dec(t, "0.10"), 2)
	require.ErrorIs(t, err, ErrInsufficientSources)
}

func TestCrossSourceMedianDropsThenMedians(t *testing.T) {
	in := []math.LegacyDec{dec(t, "100"), dec(t, "102"), dec(t, "300")}
	m, err := CrossSourceMedian(in, dec(t, "0.10"), 2)
	require.NoError(t, err)
	require.Equal(t, "101.000000000000000000", m.String()) // median of [100,102]
}

func TestCrossSourceMedianTooFewAfterDrop(t *testing.T) {
	// two wildly-divergent sources: provisional median between them, both within/around band?
	in := []math.LegacyDec{dec(t, "100"), dec(t, "300")}
	_, err := CrossSourceMedian(in, dec(t, "0.10"), 2)
	require.ErrorIs(t, err, ErrInsufficientSources)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/ -run 'TestMedian|TestDrop|TestCrossSource' -v`
Expected: FAIL — `undefined: Median`.

- [ ] **Step 3: Write the implementation**

```go
package feeder

import (
	"errors"
	"sort"

	"cosmossdk.io/math"
)

// ErrInsufficientSources is returned when fewer than min healthy sources remain.
var ErrInsufficientSources = errors.New("insufficient healthy price sources")

// Median returns the median of prices (average of the two middle values for
// an even count). Returns an error for an empty slice.
func Median(prices []math.LegacyDec) (math.LegacyDec, error) {
	n := len(prices)
	if n == 0 {
		return math.LegacyDec{}, errors.New("median of empty set")
	}
	sorted := make([]math.LegacyDec, n)
	copy(sorted, prices)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].LT(sorted[j]) })
	if n%2 == 1 {
		return sorted[n/2], nil
	}
	lo, hi := sorted[n/2-1], sorted[n/2]
	return lo.Add(hi).QuoInt64(2), nil
}

// DropDeviating removes prices that deviate more than maxDeviation (fractional,
// e.g. 0.10 = 10%) from the provisional median of the input.
func DropDeviating(prices []math.LegacyDec, maxDeviation math.LegacyDec) []math.LegacyDec {
	if len(prices) == 0 {
		return prices
	}
	mid, err := Median(prices)
	if err != nil || !mid.IsPositive() {
		return prices
	}
	band := mid.Mul(maxDeviation)
	out := make([]math.LegacyDec, 0, len(prices))
	for _, p := range prices {
		diff := p.Sub(mid).Abs()
		if diff.LTE(band) {
			out = append(out, p)
		}
	}
	return out
}

// CrossSourceMedian applies the data-quality policy (design D3): require
// minProviders sources, drop deviating sources, then require minProviders
// survivors before returning their median.
func CrossSourceMedian(prices []math.LegacyDec, maxDeviation math.LegacyDec, minProviders int) (math.LegacyDec, error) {
	if len(prices) < minProviders {
		return math.LegacyDec{}, ErrInsufficientSources
	}
	filtered := DropDeviating(prices, maxDeviation)
	if len(filtered) < minProviders {
		return math.LegacyDec{}, ErrInsufficientSources
	}
	return Median(filtered)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/ -run 'TestMedian|TestDrop|TestCrossSource' -v`
Expected: PASS (7 tests).

- [ ] **Step 5: Commit**

```bash
git add feeder/feeder/aggregate.go feeder/feeder/aggregate_test.go
git commit -m "feat(feeder): add cross-source median aggregation with deviation guard"
```

---

## Task 8: Price cache

**Files:**
- Create: `feeder/feeder/cache.go`
- Test: `feeder/feeder/cache_test.go`

- [ ] **Step 1: Write the failing test**

```go
package feeder

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCacheSetAndFresh(t *testing.T) {
	c := NewCache()
	now := time.Unix(1000, 0)
	c.Set("BTC:USD", dec(t, "65000"), now)

	price, ok := c.Fresh("BTC:USD", 30*time.Second, now.Add(10*time.Second))
	require.True(t, ok)
	require.Equal(t, "65000.000000000000000000", price.String())
}

func TestCacheStale(t *testing.T) {
	c := NewCache()
	now := time.Unix(1000, 0)
	c.Set("BTC:USD", dec(t, "65000"), now)
	_, ok := c.Fresh("BTC:USD", 30*time.Second, now.Add(31*time.Second))
	require.False(t, ok)
}

func TestCacheUnhealthyNotFresh(t *testing.T) {
	c := NewCache()
	now := time.Unix(1000, 0)
	c.Set("BTC:USD", dec(t, "65000"), now)
	c.MarkUnhealthy("BTC:USD")
	_, ok := c.Fresh("BTC:USD", 30*time.Second, now)
	require.False(t, ok)
}

func TestCacheMissing(t *testing.T) {
	c := NewCache()
	_, ok := c.Fresh("ETH:USD", 30*time.Second, time.Now())
	require.False(t, ok)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/ -run TestCache -v`
Expected: FAIL — `undefined: NewCache`.

- [ ] **Step 3: Write the implementation**

```go
package feeder

import (
	"sync"
	"time"

	"cosmossdk.io/math"
)

type cacheEntry struct {
	price   math.LegacyDec
	at      time.Time
	healthy bool
}

// Cache is a concurrency-safe latest-median-per-pair store shared by the
// price loop (writer) and submit loop (reader).
type Cache struct {
	mu sync.RWMutex
	m  map[string]cacheEntry
}

// NewCache returns an empty cache.
func NewCache() *Cache {
	return &Cache{m: make(map[string]cacheEntry)}
}

// Set stores a fresh, healthy median for pair at time t.
func (c *Cache) Set(pair string, price math.LegacyDec, t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[pair] = cacheEntry{price: price, at: t, healthy: true}
}

// MarkUnhealthy flags pair so it will not be submitted, keeping any prior value.
func (c *Cache) MarkUnhealthy(pair string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.m[pair]
	e.healthy = false
	c.m[pair] = e
}

// Fresh returns the cached price if it is healthy and newer than maxAge at now.
func (c *Cache) Fresh(pair string, maxAge time.Duration, now time.Time) (math.LegacyDec, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.m[pair]
	if !ok || !e.healthy {
		return math.LegacyDec{}, false
	}
	if now.Sub(e.at) > maxAge {
		return math.LegacyDec{}, false
	}
	return e.price, true
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/ -run TestCache -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add feeder/feeder/cache.go feeder/feeder/cache_test.go
git commit -m "feat(feeder): add concurrency-safe price cache"
```

---

## Task 9: Prometheus metrics

**Files:**
- Create: `feeder/feeder/metrics.go`
- Test: `feeder/feeder/metrics_test.go`

- [ ] **Step 1: Write the failing test**

```go
package feeder

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestMetricsRegister(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	require.NotNil(t, m)

	m.FeedsSubmitted.WithLabelValues("BTC:USD").Inc()
	m.FeedsFailed.WithLabelValues("VTX:USD", "insufficient_sources").Inc()
	m.ProviderErrors.WithLabelValues("binance").Inc()

	mfs, err := reg.Gather()
	require.NoError(t, err)
	names := map[string]bool{}
	for _, mf := range mfs {
		names[mf.GetName()] = true
	}
	require.True(t, names["vertix_feeder_feeds_submitted_total"])
	require.True(t, names["vertix_feeder_feeds_failed_total"])
	require.True(t, names["vertix_feeder_provider_errors_total"])
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/ -run TestMetrics -v`
Expected: FAIL — `undefined: NewMetrics`.

- [ ] **Step 3: Write the implementation**

```go
package feeder

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the Prometheus instruments (design §8).
type Metrics struct {
	FeedsSubmitted  *prometheus.CounterVec
	FeedsFailed     *prometheus.CounterVec
	ProviderLatency *prometheus.HistogramVec
	ProviderErrors  *prometheus.CounterVec
	LastSubmit      *prometheus.GaugeVec
	registry        *prometheus.Registry
}

// NewMetrics registers and returns the feeder metrics on reg.
func NewMetrics(reg *prometheus.Registry) *Metrics {
	m := &Metrics{
		FeedsSubmitted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vertix_feeder_feeds_submitted_total", Help: "Feeds submitted per pair.",
		}, []string{"pair"}),
		FeedsFailed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vertix_feeder_feeds_failed_total", Help: "Failed/skipped feeds per pair and reason.",
		}, []string{"pair", "reason"}),
		ProviderLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "vertix_feeder_provider_latency_ms", Help: "Provider fetch latency in ms.",
			Buckets: []float64{10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
		}, []string{"provider", "pair"}),
		ProviderErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vertix_feeder_provider_errors_total", Help: "Provider fetch errors.",
		}, []string{"provider"}),
		LastSubmit: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "vertix_feeder_last_submit_timestamp", Help: "Unix timestamp of last submit per pair.",
		}, []string{"pair"}),
		registry: reg,
	}
	reg.MustRegister(m.FeedsSubmitted, m.FeedsFailed, m.ProviderLatency, m.ProviderErrors, m.LastSubmit)
	return m
}

// ObserveLatency records a provider fetch latency.
func (m *Metrics) ObserveLatency(provider, pair string, d time.Duration) {
	m.ProviderLatency.WithLabelValues(provider, pair).Observe(float64(d.Milliseconds()))
}

// Serve starts the metrics HTTP server on the given port. Blocks until error.
func (m *Metrics) Serve(port int) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))
	srv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	return srv.ListenAndServe()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/ -run TestMetrics -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add feeder/feeder/metrics.go feeder/feeder/metrics_test.go
git commit -m "feat(feeder): add prometheus metrics surface"
```

---

## Task 10: Price loop

**Files:**
- Create: `feeder/feeder/priceloop.go`
- Test: `feeder/feeder/priceloop_test.go`

The price loop iterates pairs, fetches each pair from every enabled provider that has a symbol for it, computes the cross-source median, and updates the cache (or marks the pair unhealthy).

- [ ] **Step 1: Write the failing test (uses a fake provider, no network)**

```go
package feeder

import (
	"context"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/feeder/feeder/provider"
)

type fakeProvider struct {
	name   string
	prices map[string]string // symbol -> dec string
	err    error
}

func (f fakeProvider) Name() string { return f.name }
func (f fakeProvider) Fetch(_ context.Context, symbol string) (math.LegacyDec, error) {
	if f.err != nil {
		return math.LegacyDec{}, f.err
	}
	return math.LegacyNewDecFromStr(f.prices[symbol])
}

func testPriceLoop(t *testing.T, providers []provider.PriceProvider, pairs []PairConfig) (*PriceLoop, *Cache) {
	t.Helper()
	cache := NewCache()
	m := NewMetrics(prometheus.NewRegistry())
	pl := NewPriceLoop(pairs, providers, cache, m, QualityConfig{MinProviders: 2, MaxDeviation: "0.10", MaxQuoteAge: 30 * time.Second})
	return pl, cache
}

func TestPriceLoopTickHealthy(t *testing.T) {
	pairs := []PairConfig{{Pair: "BTC:USD", Symbols: map[string]string{"a": "BTCUSDT", "b": "BTCUSDT"}}}
	provs := []provider.PriceProvider{
		fakeProvider{name: "a", prices: map[string]string{"BTCUSDT": "100"}},
		fakeProvider{name: "b", prices: map[string]string{"BTCUSDT": "102"}},
	}
	pl, cache := testPriceLoop(t, provs, pairs)
	pl.tick(context.Background(), time.Unix(1000, 0))

	price, ok := cache.Fresh("BTC:USD", 30*time.Second, time.Unix(1000, 0))
	require.True(t, ok)
	require.Equal(t, "101.000000000000000000", price.String())
}

func TestPriceLoopTickInsufficientSources(t *testing.T) {
	pairs := []PairConfig{{Pair: "BTC:USD", Symbols: map[string]string{"a": "BTCUSDT", "b": "BTCUSDT"}}}
	provs := []provider.PriceProvider{
		fakeProvider{name: "a", prices: map[string]string{"BTCUSDT": "100"}},
		fakeProvider{name: "b", err: context.DeadlineExceeded},
	}
	pl, cache := testPriceLoop(t, provs, pairs)
	pl.tick(context.Background(), time.Unix(1000, 0))

	_, ok := cache.Fresh("BTC:USD", 30*time.Second, time.Unix(1000, 0))
	require.False(t, ok) // only 1 healthy source, min is 2 -> unhealthy
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/ -run TestPriceLoop -v`
Expected: FAIL — `undefined: NewPriceLoop`.

- [ ] **Step 3: Write the implementation**

```go
package feeder

import (
	"context"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/math"

	"github.com/vertix-network/vertix/feeder/feeder/provider"
)

// PriceLoop fetches prices and updates the cache on a fixed interval.
type PriceLoop struct {
	pairs     []PairConfig
	providers []provider.PriceProvider
	cache     *Cache
	metrics   *Metrics
	quality   QualityConfig
	maxDev    math.LegacyDec
	interval  time.Duration
	logger    log.Logger
}

// NewPriceLoop constructs a price loop. maxDeviation is parsed from quality.
func NewPriceLoop(pairs []PairConfig, providers []provider.PriceProvider, cache *Cache, m *Metrics, quality QualityConfig) *PriceLoop {
	dev, err := math.LegacyNewDecFromStr(quality.MaxDeviation)
	if err != nil {
		dev = math.LegacyNewDecWithPrec(10, 2) // 0.10 fallback; config is pre-validated
	}
	return &PriceLoop{
		pairs: pairs, providers: providers, cache: cache, metrics: m,
		quality: quality, maxDev: dev, interval: quality.MaxQuoteAge, logger: log.NewNopLogger(),
	}
}

// WithLogger sets a logger and tick interval; interval defaults to maxQuoteAge otherwise.
func (pl *PriceLoop) WithLogger(l log.Logger) *PriceLoop { pl.logger = l; return pl }

// WithInterval overrides the tick interval (feed_interval).
func (pl *PriceLoop) WithInterval(d time.Duration) *PriceLoop { pl.interval = d; return pl }

// Run ticks until ctx is cancelled.
func (pl *PriceLoop) Run(ctx context.Context) {
	ticker := time.NewTicker(pl.interval)
	defer ticker.Stop()
	pl.tick(ctx, time.Now())
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pl.tick(ctx, time.Now())
		}
	}
}

func (pl *PriceLoop) tick(ctx context.Context, now time.Time) {
	for _, pc := range pl.pairs {
		var quotes []math.LegacyDec
		for _, prov := range pl.providers {
			symbol, ok := pc.Symbols[prov.Name()]
			if !ok {
				continue
			}
			start := time.Now()
			price, err := prov.Fetch(ctx, symbol)
			pl.metrics.ObserveLatency(prov.Name(), pc.Pair, time.Since(start))
			if err != nil {
				pl.metrics.ProviderErrors.WithLabelValues(prov.Name()).Inc()
				pl.logger.Debug("provider fetch failed", "provider", prov.Name(), "pair", pc.Pair, "err", err)
				continue
			}
			quotes = append(quotes, price)
		}
		median, err := CrossSourceMedian(quotes, pl.maxDev, pl.quality.MinProviders)
		if err != nil {
			pl.cache.MarkUnhealthy(pc.Pair)
			pl.metrics.FeedsFailed.WithLabelValues(pc.Pair, "insufficient_sources").Inc()
			continue
		}
		pl.cache.Set(pc.Pair, median, now)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/ -run TestPriceLoop -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add feeder/feeder/priceloop.go feeder/feeder/priceloop_test.go
git commit -m "feat(feeder): add price loop with quality-gated caching"
```

---

## Task 11: Broadcaster interface, FeedSubmission, message building

**Files:**
- Create: `feeder/feeder/broadcaster.go`
- Test: `feeder/feeder/broadcaster_test.go`

This task adds the `Broadcaster` interface, the `FeedSubmission` type, a `newCodec()` helper (interface registry + tx config), and a **pure, unit-tested `buildMsgs` function** that turns submissions into validated `MsgSubmitFeed`. The live gRPC implementation is added but verified against a node (not CI).

- [ ] **Step 1: Write the failing test (covers buildMsgs + codec registration)**

```go
package feeder

import (
	"testing"

	"github.com/stretchr/testify/require"

	oracletypes "github.com/vertix-network/vertix/x/oracle/types"
)

func TestBuildMsgsValidates(t *testing.T) {
	feeder := "vtx1feederaddrxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	validator := "vtxvaloper1valaddrxxxxxxxxxxxxxxxxxxxxxxxx"
	// Use deterministic, well-formed bech32 addresses generated in-test.
	feeder, validator = testAddrs(t)

	feeds := []FeedSubmission{
		{Pair: "BTC:USD", Price: dec(t, "65000.5")},
		{Pair: "ETH:USD", Price: dec(t, "3200")},
	}
	msgs, err := buildMsgs(feeder, validator, feeds)
	require.NoError(t, err)
	require.Len(t, msgs, 2)

	m0, ok := msgs[0].(*oracletypes.MsgSubmitFeed)
	require.True(t, ok)
	require.Equal(t, "BTC:USD", m0.Pair)
	require.Equal(t, feeder, m0.Feeder)
	require.Equal(t, validator, m0.Validator)
	require.NoError(t, m0.ValidateBasic())
}

func TestNewCodecRegistersOracle(t *testing.T) {
	c := newCodec()
	require.NotNil(t, c.Marshaler)
	require.NotNil(t, c.TxConfig)
	// Resolving the MsgSubmitFeed type URL must succeed via the registry.
	_, err := c.InterfaceRegistry.Resolve("/vertix.oracle.v1.MsgSubmitFeed")
	require.NoError(t, err)
}
```

Add the address helper in the same test file:

```go
func testAddrs(t *testing.T) (feeder, validator string) {
	t.Helper()
	priv := secp256k1.GenPrivKey()
	acc := sdk.AccAddress(priv.PubKey().Address())
	val := sdk.ValAddress(priv.PubKey().Address())
	return acc.String(), val.String()
}
```

with imports:

```go
import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
)
```

> The test relies on the chain's Bech32 config (`vtx`/`vtxvaloper`). Since `app/config.go` seals the SDK config in its `init()`, import the app package for its side effect by adding a blank import in the test file: `_ "github.com/vertix-network/vertix/app"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/ -run 'TestBuildMsgs|TestNewCodec' -v`
Expected: FAIL — `undefined: buildMsgs` / `undefined: newCodec`.

- [ ] **Step 3: Write the implementation**

```go
package feeder

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	oracletypes "github.com/vertix-network/vertix/x/oracle/types"
)

// FeedSubmission is a single pair/price to submit in the per-window batch.
type FeedSubmission struct {
	Pair  string
	Price math.LegacyDec
}

// Broadcaster builds, signs, and broadcasts one tx containing a MsgSubmitFeed
// per submission. The submit loop depends on this interface (testable seam).
type Broadcaster interface {
	SubmitFeeds(ctx context.Context, feeds []FeedSubmission) (txHash string, err error)
}

// codecSet bundles the codec + tx config used to encode/sign feeder txs.
type codecSet struct {
	Marshaler         codec.Codec
	InterfaceRegistry codectypes.InterfaceRegistry
	TxConfig          interface{ ResolvesTx() } // placeholder; replaced below
}

// newCodec returns an interface registry + proto codec + tx config with the
// std, auth, crypto, and oracle interfaces registered (design §4).
func newCodec() codecCfg {
	ir := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(ir)
	authtypes.RegisterInterfaces(ir)
	oracletypes.RegisterInterfaces(ir)
	cdc := codec.NewProtoCodec(ir)
	txCfg := authtx.NewTxConfig(cdc, authtx.DefaultSignModes)
	return codecCfg{Marshaler: cdc, InterfaceRegistry: ir, TxConfig: txCfg}
}

// codecCfg is the concrete codec bundle (replaces the placeholder above).
type codecCfg struct {
	Marshaler         codec.Codec
	InterfaceRegistry codectypes.InterfaceRegistry
	TxConfig          interface {
		// minimal surface used by the broadcaster
	}
}

// buildMsgs converts submissions to validated MsgSubmitFeed messages.
func buildMsgs(feeder, validator string, feeds []FeedSubmission) ([]sdk.Msg, error) {
	msgs := make([]sdk.Msg, 0, len(feeds))
	for _, f := range feeds {
		m := &oracletypes.MsgSubmitFeed{
			Feeder:    feeder,
			Validator: validator,
			Pair:      f.Pair,
			Price:     f.Price.String(),
		}
		if err := m.ValidateBasic(); err != nil {
			return nil, fmt.Errorf("invalid MsgSubmitFeed for %q: %w", f.Pair, err)
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

var _ = testutil.TestEncodingConfig{} // keep import if needed; remove if unused
```

> **Implementer note:** the `codecSet`/placeholder scaffolding above is illustrative. Simplify to a single clean bundle: a struct holding `Marshaler codec.Codec`, `InterfaceRegistry codectypes.InterfaceRegistry`, and `TxConfig client.TxConfig` (import `"github.com/cosmos/cosmos-sdk/client"`). `newCodec()` returns it. Delete the `testutil` line if unused. The test only asserts `Marshaler`, `TxConfig` non-nil and that the registry resolves `/vertix.oracle.v1.MsgSubmitFeed`.

Replace the scaffolding with this clean version:

```go
package feeder

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	oracletypes "github.com/vertix-network/vertix/x/oracle/types"
)

type FeedSubmission struct {
	Pair  string
	Price math.LegacyDec
}

type Broadcaster interface {
	SubmitFeeds(ctx context.Context, feeds []FeedSubmission) (txHash string, err error)
}

type codecCfg struct {
	Marshaler         codec.Codec
	InterfaceRegistry codectypes.InterfaceRegistry
	TxConfig          client.TxConfig
}

func newCodec() codecCfg {
	ir := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(ir)
	authtypes.RegisterInterfaces(ir)
	oracletypes.RegisterInterfaces(ir)
	cdc := codec.NewProtoCodec(ir)
	return codecCfg{Marshaler: cdc, InterfaceRegistry: ir, TxConfig: authtx.NewTxConfig(cdc, authtx.DefaultSignModes)}
}

func buildMsgs(feeder, validator string, feeds []FeedSubmission) ([]sdk.Msg, error) {
	msgs := make([]sdk.Msg, 0, len(feeds))
	for _, f := range feeds {
		m := &oracletypes.MsgSubmitFeed{Feeder: feeder, Validator: validator, Pair: f.Pair, Price: f.Price.String()}
		if err := m.ValidateBasic(); err != nil {
			return nil, fmt.Errorf("invalid MsgSubmitFeed for %q: %w", f.Pair, err)
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}
```

(Use this clean version as the file content; the earlier block was scaffolding to delete.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/ -run 'TestBuildMsgs|TestNewCodec' -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add feeder/feeder/broadcaster.go feeder/feeder/broadcaster_test.go
git commit -m "feat(feeder): add broadcaster interface, codec, and message building"
```

---

## Task 12: Live gRPC broadcaster (node-verified)

**Files:**
- Modify: `feeder/feeder/broadcaster.go`

This adds the concrete `grpcBroadcaster` implementing `Broadcaster` and a chain-query helper for height + vote window. Signing/broadcast correctness is verified against a local node (acceptance gate), not in CI.

- [ ] **Step 1: Add the gRPC broadcaster implementation**

Append to `feeder/feeder/broadcaster.go`:

```go
import (
	// add to the existing import block:
	"sync"
	"time"

	"cosmossdk.io/log"
	cmtservice "github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	clienttx "github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// grpcBroadcaster signs MsgSubmitFeed batches with the feeder key and
// broadcasts them over gRPC (design §4, Approach A).
type grpcBroadcaster struct {
	cfg        *Config
	codec      codecCfg
	kr         keyring.Keyring
	conn       *grpc.ClientConn
	feederAddr string
	accNum     uint64
	mu         sync.Mutex
	seq        uint64
	logger     log.Logger
}

// NewGRPCBroadcaster dials the node, loads the feeder key, and primes the
// account number/sequence. Fatal errors here are startup-fatal (design §9).
func NewGRPCBroadcaster(cfg *Config, logger log.Logger) (*grpcBroadcaster, error) {
	cdc := newCodec()
	kr, err := keyring.New("vertix-feeder", cfg.KeyringBackend, cfg.KeyringDir, nil, cdc.Marshaler)
	if err != nil {
		return nil, fmt.Errorf("open keyring: %w", err)
	}
	rec, err := kr.Key(cfg.KeyName)
	if err != nil {
		return nil, fmt.Errorf("key %q not found: %w", cfg.KeyName, err)
	}
	addr, err := rec.GetAddress()
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(cfg.NodeGRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial node: %w", err)
	}
	b := &grpcBroadcaster{cfg: cfg, codec: cdc, kr: kr, conn: conn, feederAddr: addr.String(), logger: logger}
	if err := b.refreshAccount(context.Background()); err != nil {
		return nil, fmt.Errorf("prime account: %w", err)
	}
	return b, nil
}

func (b *grpcBroadcaster) refreshAccount(ctx context.Context) error {
	q := authtypes.NewQueryClient(b.conn)
	res, err := q.Account(ctx, &authtypes.QueryAccountRequest{Address: b.feederAddr})
	if err != nil {
		return err
	}
	var acc authtypes.AccountI
	if err := b.codec.InterfaceRegistry.UnpackAny(res.Account, &acc); err != nil {
		return err
	}
	b.mu.Lock()
	b.accNum = acc.GetAccountNumber()
	b.seq = acc.GetSequence()
	b.mu.Unlock()
	return nil
}

// LatestHeight returns the node's latest block height (for the submit loop).
func (b *grpcBroadcaster) LatestHeight(ctx context.Context) (int64, error) {
	c := cmtservice.NewServiceClient(b.conn)
	res, err := c.GetLatestBlock(ctx, &cmtservice.GetLatestBlockRequest{})
	if err != nil {
		return 0, err
	}
	if res.SdkBlock != nil {
		return res.SdkBlock.Header.Height, nil
	}
	return res.Block.Header.Height, nil
}

// SubmitFeeds builds, signs, and SYNC-broadcasts one tx with the batch.
func (b *grpcBroadcaster) SubmitFeeds(ctx context.Context, feeds []FeedSubmission) (string, error) {
	msgs, err := buildMsgs(b.feederAddr, b.cfg.Validator, feeds)
	if err != nil {
		return "", err
	}
	b.mu.Lock()
	accNum, seq := b.accNum, b.seq
	b.mu.Unlock()

	fees, _ := sdk.ParseCoinsNormalized(b.cfg.Fees)
	gas, _ := math.NewIntFromString(b.cfg.Gas) // "auto" handled below
	txf := clienttx.Factory{}.
		WithChainID(b.cfg.ChainID).
		WithKeybase(b.kr).
		WithTxConfig(b.codec.TxConfig).
		WithAccountNumber(accNum).
		WithSequence(seq).
		WithFees(fees.String()).
		WithSignMode(signingtypes.SignMode_SIGN_MODE_DIRECT)
	if b.cfg.Gas == "auto" {
		txf = txf.WithSimulateAndExecute(true).WithGasAdjustment(b.cfg.GasAdjustment)
	} else {
		txf = txf.WithGas(gas.Uint64())
	}

	txb, err := txf.BuildUnsignedTx(msgs...)
	if err != nil {
		return "", err
	}
	if err := clienttx.Sign(ctx, txf, b.cfg.KeyName, txb, true); err != nil {
		return "", err
	}
	bz, err := b.codec.TxConfig.TxEncoder()(txb.GetTx())
	if err != nil {
		return "", err
	}
	svc := txtypes.NewServiceClient(b.conn)
	res, err := svc.BroadcastTx(ctx, &txtypes.BroadcastTxRequest{TxBytes: bz, Mode: txtypes.BroadcastMode_BROADCAST_MODE_SYNC})
	if err != nil {
		return "", err
	}
	if res.TxResponse.Code != 0 {
		// Sequence mismatch: re-sync and let the caller retry next poll.
		if err := b.refreshAccount(ctx); err == nil {
			b.logger.Debug("resynced account after non-zero code", "code", res.TxResponse.Code)
		}
		return res.TxResponse.TxHash, fmt.Errorf("tx rejected, code %d: %s", res.TxResponse.Code, res.TxResponse.RawLog)
	}
	b.mu.Lock()
	b.seq++
	b.mu.Unlock()
	return res.TxResponse.TxHash, nil
}

// VoteWindow queries the oracle module params for the vote window.
func (b *grpcBroadcaster) VoteWindow(ctx context.Context) (int64, error) {
	q := oracletypes.NewQueryClient(b.conn)
	res, err := q.Params(ctx, &oracletypes.QueryParamsRequest{})
	if err != nil {
		return 0, err
	}
	return int64(res.Params.VoteWindow), nil
}

// Close releases the gRPC connection.
func (b *grpcBroadcaster) Close() error { return b.conn.Close() }

var _ Broadcaster = (*grpcBroadcaster)(nil)
var _ = time.Second // keep time import if otherwise unused
```

> **Implementer notes (verify against SDK v0.50.14 at build time, adjust import paths/names if the compiler disagrees — these are the documented v0.50 symbols):**
> - `oracletypes.QueryClient` / `QueryParamsRequest` / `Params.VoteWindow` come from the generated `x/oracle/types/query.pb.go` (confirm field is `VoteWindow uint32`).
> - `clienttx.Factory` builder methods (`WithChainID`, `WithKeybase`, `WithTxConfig`, `WithAccountNumber`, `WithSequence`, `WithGas`, `WithFees`, `WithSignMode`, `WithSimulateAndExecute`, `WithGasAdjustment`) and `clienttx.Sign` are in `github.com/cosmos/cosmos-sdk/client/tx`.
> - `cmtservice.GetLatestBlockRequest`/`ServiceClient` are in `github.com/cosmos/cosmos-sdk/client/grpc/cmtservice`.
> - Remove the trailing `var _ = time.Second` line if `time` is used elsewhere.

- [ ] **Step 2: Verify it compiles**

Run: `GOTOOLCHAIN=go1.25.4 go build ./feeder/...`
Expected: success. Fix any import/symbol mismatches per the notes until it builds.

- [ ] **Step 3: Run existing feeder tests (ensure no regression)**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/ -run 'TestBuildMsgs|TestNewCodec' -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add feeder/feeder/broadcaster.go
git commit -m "feat(feeder): add gRPC broadcaster with signing and account management"
```

---

## Task 13: Submit loop (window detection + batch)

**Files:**
- Create: `feeder/feeder/submitloop.go`
- Test: `feeder/feeder/submitloop_test.go`

The submit loop is decoupled from the gRPC client via a `HeightFunc` and the `Broadcaster` interface, making it unit-testable with mocks.

- [ ] **Step 1: Write the failing test**

```go
package feeder

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

type mockBroadcaster struct {
	mu    sync.Mutex
	calls [][]FeedSubmission
}

func (m *mockBroadcaster) SubmitFeeds(_ context.Context, feeds []FeedSubmission) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, feeds)
	return "HASH", nil
}

func TestSubmitLoopOncePerWindow(t *testing.T) {
	cache := NewCache()
	now := time.Unix(1000, 0)
	cache.Set("BTC:USD", dec(t, "65000"), now)
	cache.Set("ETH:USD", dec(t, "3200"), now)

	bc := &mockBroadcaster{}
	m := NewMetrics(prometheus.NewRegistry())
	sl := NewSubmitLoop(
		[]PairConfig{{Pair: "BTC:USD"}, {Pair: "ETH:USD"}},
		cache, bc, m, 10 /*voteWindow*/, 30*time.Second,
	)
	sl.nowFn = func() time.Time { return now }

	// height 5 -> window 0
	sl.maybeSubmit(context.Background(), 5)
	// height 8 -> still window 0, must NOT submit again
	sl.maybeSubmit(context.Background(), 8)
	// height 12 -> window 1, must submit
	sl.maybeSubmit(context.Background(), 12)

	require.Len(t, bc.calls, 2)
	require.Len(t, bc.calls[0], 2) // both pairs fresh+healthy
}

func TestSubmitLoopSkipsStalePairs(t *testing.T) {
	cache := NewCache()
	now := time.Unix(1000, 0)
	cache.Set("BTC:USD", dec(t, "65000"), now.Add(-time.Hour)) // stale
	cache.Set("ETH:USD", dec(t, "3200"), now)                  // fresh

	bc := &mockBroadcaster{}
	m := NewMetrics(prometheus.NewRegistry())
	sl := NewSubmitLoop([]PairConfig{{Pair: "BTC:USD"}, {Pair: "ETH:USD"}}, cache, bc, m, 10, 30*time.Second)
	sl.nowFn = func() time.Time { return now }

	sl.maybeSubmit(context.Background(), 5)
	require.Len(t, bc.calls, 1)
	require.Len(t, bc.calls[0], 1) // only ETH:USD
	require.Equal(t, "ETH:USD", bc.calls[0][0].Pair)
}

func TestSubmitLoopNoFreshPairsNoCall(t *testing.T) {
	cache := NewCache()
	bc := &mockBroadcaster{}
	m := NewMetrics(prometheus.NewRegistry())
	sl := NewSubmitLoop([]PairConfig{{Pair: "BTC:USD"}}, cache, bc, m, 10, 30*time.Second)
	sl.nowFn = func() time.Time { return time.Unix(1000, 0) }
	sl.maybeSubmit(context.Background(), 5)
	require.Empty(t, bc.calls)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/ -run TestSubmitLoop -v`
Expected: FAIL — `undefined: NewSubmitLoop`.

- [ ] **Step 3: Write the implementation**

```go
package feeder

import (
	"context"
	"time"

	"cosmossdk.io/log"
)

// HeightFunc returns the node's latest block height.
type HeightFunc func(ctx context.Context) (int64, error)

// SubmitLoop submits the cached medians exactly once per vote window.
type SubmitLoop struct {
	pairs       []PairConfig
	cache       *Cache
	bc          Broadcaster
	metrics     *Metrics
	voteWindow  int64
	maxQuoteAge time.Duration
	poll        time.Duration
	retries     int
	heightFn    HeightFunc
	nowFn       func() time.Time
	logger      log.Logger
	lastWindow  int64
}

// NewSubmitLoop constructs a submit loop. voteWindow must be > 0.
func NewSubmitLoop(pairs []PairConfig, cache *Cache, bc Broadcaster, m *Metrics, voteWindow int64, maxQuoteAge time.Duration) *SubmitLoop {
	return &SubmitLoop{
		pairs: pairs, cache: cache, bc: bc, metrics: m,
		voteWindow: voteWindow, maxQuoteAge: maxQuoteAge,
		poll: time.Second, retries: 3, nowFn: time.Now, lastWindow: -1, logger: log.NewNopLogger(),
	}
}

// Configure sets runtime options from config.
func (sl *SubmitLoop) Configure(poll time.Duration, retries int, heightFn HeightFunc, logger log.Logger) *SubmitLoop {
	sl.poll = poll
	sl.retries = retries
	sl.heightFn = heightFn
	sl.logger = logger
	return sl
}

// Run polls height until ctx is cancelled.
func (sl *SubmitLoop) Run(ctx context.Context) {
	ticker := time.NewTicker(sl.poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h, err := sl.heightFn(ctx)
			if err != nil {
				sl.logger.Debug("height query failed", "err", err)
				continue
			}
			sl.maybeSubmit(ctx, h)
		}
	}
}

func (sl *SubmitLoop) maybeSubmit(ctx context.Context, height int64) {
	if sl.voteWindow <= 0 {
		return
	}
	window := height / sl.voteWindow
	if window == sl.lastWindow {
		return
	}
	now := sl.nowFn()
	var feeds []FeedSubmission
	for _, pc := range sl.pairs {
		price, ok := sl.cache.Fresh(pc.Pair, sl.maxQuoteAge, now)
		if !ok {
			continue
		}
		feeds = append(feeds, FeedSubmission{Pair: pc.Pair, Price: price})
	}
	if len(feeds) == 0 {
		return
	}
	hash, err := sl.bc.SubmitFeeds(ctx, feeds)
	if err != nil {
		sl.logger.Error("broadcast failed", "err", err)
		for _, f := range feeds {
			sl.metrics.FeedsFailed.WithLabelValues(f.Pair, "broadcast_error").Inc()
		}
		return // do not advance lastWindow; retried next poll within the window
	}
	sl.lastWindow = window
	tsec := float64(now.Unix())
	for _, f := range feeds {
		sl.metrics.FeedsSubmitted.WithLabelValues(f.Pair).Inc()
		sl.metrics.LastSubmit.WithLabelValues(f.Pair).Set(tsec)
	}
	sl.logger.Info("submitted feeds", "window", window, "pairs", len(feeds), "tx", hash)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/ -run TestSubmitLoop -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add feeder/feeder/submitloop.go feeder/feeder/submitloop_test.go
git commit -m "feat(feeder): add window-gated submit loop"
```

---

## Task 14: Feeder orchestrator + integration test

**Files:**
- Create: `feeder/feeder/feeder.go`
- Test: `feeder/feeder/feeder_test.go`

The orchestrator wires providers → price loop and cache → submit loop, runs the metrics server, and supports dependency injection of a `Broadcaster` + `HeightFunc` for testing.

- [ ] **Step 1: Write the failing integration test**

```go
package feeder

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/feeder/feeder/provider"
)

func TestFeederEndToEndWithMocks(t *testing.T) {
	cfg := &Config{
		ChainID:        "vertix-devnet-1",
		Validator:      mustValAddr(t),
		FeedInterval:   10 * time.Millisecond,
		SubmitPoll:     10 * time.Millisecond,
		BroadcastRetry: 3,
		Quality:        QualityConfig{MinProviders: 2, MaxDeviation: "0.10", MaxQuoteAge: time.Minute},
		Pairs: []PairConfig{
			{Pair: "BTC:USD", Symbols: map[string]string{"a": "BTCUSDT", "b": "BTCUSDT"}},
		},
		Prometheus: PrometheusConfig{Enabled: false},
	}
	provs := []provider.PriceProvider{
		fakeProvider{name: "a", prices: map[string]string{"BTCUSDT": "100"}},
		fakeProvider{name: "b", prices: map[string]string{"BTCUSDT": "102"}},
	}
	bc := &mockBroadcaster{}

	f, err := NewWithDeps(cfg, provs, bc, func(context.Context) (int64, error) { return 100, nil }, 10)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	f.Run(ctx)

	require.GreaterOrEqual(t, len(bc.calls), 1)
	require.Equal(t, "BTC:USD", bc.calls[0][0].Pair)
	require.Equal(t, "101.000000000000000000", bc.calls[0][0].Price.String())
}
```

Add a helper to the test file:

```go
import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	_ "github.com/vertix-network/vertix/app"
)

func mustValAddr(t *testing.T) string {
	t.Helper()
	return sdk.ValAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/ -run TestFeederEndToEnd -v`
Expected: FAIL — `undefined: NewWithDeps`.

- [ ] **Step 3: Write the implementation**

```go
package feeder

import (
	"context"
	"sync"
	"time"

	"cosmossdk.io/log"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/vertix-network/vertix/feeder/feeder/provider"
)

// Feeder is the top-level orchestrator wiring the price and submit loops.
type Feeder struct {
	cfg       *Config
	priceLoop *PriceLoop
	submit    *SubmitLoop
	metrics   *Metrics
	registry  *prometheus.Registry
	logger    log.Logger
	closeFns  []func() error
}

// NewWithDeps builds a Feeder from injected dependencies (used by tests and New).
func NewWithDeps(cfg *Config, providers []provider.PriceProvider, bc Broadcaster, heightFn HeightFunc, voteWindow int64) (*Feeder, error) {
	logger := log.NewNopLogger()
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	cache := NewCache()
	pl := NewPriceLoop(cfg.Pairs, providers, cache, m, cfg.Quality).WithInterval(cfg.FeedInterval).WithLogger(logger)
	sl := NewSubmitLoop(cfg.Pairs, cache, bc, m, voteWindow, cfg.Quality.MaxQuoteAge).
		Configure(cfg.SubmitPoll, cfg.BroadcastRetry, heightFn, logger)

	return &Feeder{cfg: cfg, priceLoop: pl, submit: sl, metrics: m, registry: reg, logger: logger}, nil
}

// New builds a production Feeder: providers from config, gRPC broadcaster,
// and the vote window resolved from chain params (or config override).
func New(cfg *Config, logger log.Logger) (*Feeder, error) {
	provs, err := provider.Build(cfg.Providers, provider.Options{StaticPrices: cfg.StaticPrices})
	if err != nil {
		return nil, err
	}
	bc, err := NewGRPCBroadcaster(cfg, logger)
	if err != nil {
		return nil, err
	}
	voteWindow := cfg.VoteWindow
	if voteWindow <= 0 {
		vw, err := bc.VoteWindow(context.Background())
		if err != nil {
			_ = bc.Close()
			return nil, err
		}
		voteWindow = vw
	}
	f, err := NewWithDeps(cfg, provs, bc, bc.LatestHeight, voteWindow)
	if err != nil {
		_ = bc.Close()
		return nil, err
	}
	f.logger = logger
	f.priceLoop.WithLogger(logger)
	f.submit.logger = logger
	f.closeFns = append(f.closeFns, bc.Close)
	return f, nil
}

// Run starts both loops (and the metrics server if enabled) until ctx ends.
func (f *Feeder) Run(ctx context.Context) {
	var wg sync.WaitGroup
	if f.cfg.Prometheus.Enabled {
		go func() {
			if err := f.metrics.Serve(f.cfg.Prometheus.Port); err != nil {
				f.logger.Error("metrics server stopped", "err", err)
			}
		}()
	}
	wg.Add(2)
	go func() { defer wg.Done(); f.priceLoop.Run(ctx) }()
	go func() { defer wg.Done(); f.submit.Run(ctx) }()
	<-ctx.Done()
	wg.Wait()
	for _, c := range f.closeFns {
		_ = c()
	}
}

var _ = time.Second
```

> Remove the trailing `var _ = time.Second` if `time` ends up used elsewhere (it is via durations in signatures — drop the line and the `time` import if the compiler flags it as unused).

> **Test wiring note:** `NewWithDeps` sets `f.submit.heightFn` via `Configure`. In `TestFeederEndToEnd`, the submit loop's `nowFn` defaults to `time.Now`, and the cache is written by the price loop with `time.Now()`, so the freshness window (`MaxQuoteAge: time.Minute`) keeps entries fresh. The submit loop uses `heightFn` returning a constant 100 with `voteWindow 10` → window 10, which differs from the initial `lastWindow -1`, so it submits once.

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOTOOLCHAIN=go1.25.4 go test ./feeder/feeder/ -run TestFeederEndToEnd -v`
Expected: PASS.

- [ ] **Step 5: Run the full feeder package test suite + race detector**

Run: `GOTOOLCHAIN=go1.25.4 go test -race ./feeder/...`
Expected: PASS (all tests).

- [ ] **Step 6: Commit**

```bash
git add feeder/feeder/feeder.go feeder/feeder/feeder_test.go
git commit -m "feat(feeder): add orchestrator wiring price and submit loops"
```

---

## Task 15: CLI entrypoint (main.go)

**Files:**
- Create: `feeder/cmd/vertix-feeder/main.go`

- [ ] **Step 1: Write the implementation**

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"cosmossdk.io/log"
	"github.com/spf13/cobra"

	"github.com/vertix-network/vertix/feeder/feeder"

	// Seal the SDK Bech32 config (vtx/vtxvaloper) on import.
	_ "github.com/vertix-network/vertix/app"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	var configPath string
	var logLevel string
	cmd := &cobra.Command{
		Use:   "vertix-feeder",
		Short: "Vertix oracle price-feed sidecar",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := feeder.LoadConfig(configPath)
			if err != nil {
				return err
			}
			if logLevel == "" {
				logLevel = cfg.LogLevel
			}
			lvl, err := log.ParseLogLevel(logLevel)
			if err != nil {
				lvl = log.LevelInfo
			}
			logger := log.NewLogger(os.Stderr, log.LevelOption(lvl))
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}
			f, err := feeder.New(cfg, logger)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			logger.Info("vertix-feeder starting", "chain_id", cfg.ChainID, "node", cfg.NodeGRPC, "pairs", len(cfg.Pairs))
			f.Run(ctx)
			logger.Info("vertix-feeder stopped")
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "feeder-config.yaml", "path to the feeder config YAML")
	cmd.Flags().StringVar(&logLevel, "log-level", "", "log level (overrides config)")
	return cmd
}
```

> **Note:** `log.ParseLogLevel` and `log.LevelOption` are from `cosmossdk.io/log`. If the exact helper names differ in v1.5.0, fall back to `log.NewLogger(os.Stderr)` (default level) — the binary must still build and run.

- [ ] **Step 2: Verify it builds**

Run: `GOTOOLCHAIN=go1.25.4 go build -o build/vertix-feeder ./feeder/cmd/vertix-feeder`
Expected: success; `build/vertix-feeder` exists.

- [ ] **Step 3: Smoke-test the binary fails cleanly without config**

Run: `./build/vertix-feeder --config /nonexistent.yaml`
Expected: prints `error: read config: ...` and exits non-zero.

- [ ] **Step 4: Commit**

```bash
git add feeder/cmd/vertix-feeder/main.go
git commit -m "feat(feeder): add vertix-feeder CLI entrypoint"
```

---

## Task 16: Example config + Makefile target

**Files:**
- Create: `feeder-config.example.yaml`
- Modify: `Makefile` (add `feeder-build` target after the `build` target, around line 134)

- [ ] **Step 1: Write the example config**

Create `feeder-config.example.yaml` with the exact contents from the design spec §5:

```yaml
# vertix-feeder example configuration.
# Copy to feeder-config.yaml and edit for your validator.

chain_id: vertix-devnet-1
node_grpc: localhost:9090          # gRPC: account/sequence queries + tx broadcast
validator: vtxvaloper1...          # valoper this feeder submits for (-> MsgSubmitFeed.validator)
key_name: feeder                   # feeder key in the keyring (signs the tx; NEVER the operator key)
keyring_backend: test              # test | file | os
keyring_dir: ~/.vertix

fees: 2000uvtx                     # flat fee per window tx (chain min_gas_prices = 0.025uvtx)
gas: "200000"                      # fixed gas, or "auto" to simulate
gas_adjustment: 1.3                # used only when gas: auto

feed_interval: 5s                  # price-loop tick
submit_poll_interval: 1s           # submit-loop height poll
vote_window: 0                     # 0 = query from chain oracle params at startup; >0 overrides
broadcast_retries: 3               # bounded retry within the window

quality:
  min_providers: 2
  max_deviation: "0.10"            # drop a source > 10% from the cross-source median
  max_quote_age: 30s               # drop quotes older than this

providers: ["coingecko", "binance", "static"]

static_prices:
  # BOOTSTRAP PLACEHOLDER — VTX is unlisted pre-mainnet. Replace this with a real
  # market source (e.g. an Osmosis VTX pool / DEX TWAP provider) once VTX lists.
  "VTX:USD": "0.10"

pairs:
  - pair: "VTX:USD"
    symbols: { static: "VTX:USD" }
  - pair: "BTC:USD"
    symbols: { coingecko: "bitcoin",  binance: "BTCUSDT" }
  - pair: "ETH:USD"
    symbols: { coingecko: "ethereum", binance: "ETHUSDT" }
  - pair: "ATOM:USD"
    symbols: { coingecko: "cosmos",   binance: "ATOMUSDT" }
  - pair: "USDC:USD"
    symbols: { coingecko: "usd-coin", binance: "USDCUSDT" }

prometheus:
  enabled: true
  port: 9200

log_level: info
```

- [ ] **Step 2: Add the Makefile target**

In `Makefile`, modify the build section (the `build:` target ends near line 134). Add immediately after the `build:` recipe block, before `clean:`:

```makefile
feeder-build:
	@echo "--> Building vertix-feeder"
	@go build $(BUILD_FLAGS) -mod=readonly -o $(BUILD_DIR)/vertix-feeder ./feeder/cmd/vertix-feeder
```

And update the `.PHONY` line in that section from `.PHONY: build clean` to:

```makefile
.PHONY: build feeder-build clean
```

- [ ] **Step 3: Verify the target works**

Run: `make feeder-build`
Expected: builds `build/vertix-feeder` successfully.

- [ ] **Step 4: Commit**

```bash
git add feeder-config.example.yaml Makefile
git commit -m "feat(feeder): add example config and make feeder-build target"
```

---

## Task 17: Doc-syncs (technical-design.md + full-design-spec.md)

**Files:**
- Modify: `docs/technical-design.md` (§5.1 loop model ~line 414–439; §5.2 config ~line 441–465)
- Modify: `docs/full-design-spec.md` (Appendix C provider bullet ~line 347)

- [ ] **Step 1: Update `technical-design.md` §5.1 to the hybrid loop**

Replace the single-loop ASCII diagram block under `### 5.1 Architecture` (the `main loop` box) with a description of the hybrid two-loop model. Use StrReplace on the fenced block to read:

```
   Hybrid two-loop design (Phase 4 spec D1):

   price loop (every feed_interval, default 5s)
     for pair: fetch from each provider -> drop stale/deviating quotes ->
     if >= min_providers survive: cache[pair] = cross-source median (timestamped)
     else: mark pair unhealthy (metric), keep last value

   submit loop (every submit_poll_interval)
     poll latest height -> window = height / VoteWindow
     if new window and fresh+healthy medians exist:
       build ONE batched, feeder-signed MsgSubmitFeed tx -> gRPC SYNC broadcast
       (bounded retry within the window; sequence re-sync on mismatch)

   metrics: feeds_submitted_total, feeds_failed_total, provider_latency_ms,
            provider_errors_total, last_submit_timestamp
```

Add a sentence after the diagram: `> Refined by the Phase 4 design spec ([specs/2026-05-29-phase-4-feeder-sidecar-design.md](./specs/2026-05-29-phase-4-feeder-sidecar-design.md)): the loop is split into an interval-driven price loop and a window-gated submit loop so provider latency never affects submission timing and each vote window gets exactly one batched submission.`

- [ ] **Step 2: Update `technical-design.md` §5.2 config**

In the YAML under `### 5.2 Configuration (YAML)`, add the new keys to match the Phase 4 spec: add `submit_poll_interval: 1s`, `vote_window: 0`, `broadcast_retries: 3`, `fees: 2000uvtx`, `gas: "200000"`, `gas_adjustment: 1.3`, and a `quality:` block (`min_providers: 2`, `max_deviation: "0.10"`, `max_quote_age: 30s`), plus a `static_prices:` block with `"VTX:USD": "0.10"`. Rename `validator_key: vtxvaloper1... # operator address` to `validator: vtxvaloper1... # valoper for MsgSubmitFeed.validator` and keep `key_name` with the comment `# feeder signing key (never the operator key)`.

- [ ] **Step 3: Mark the Appendix C provider question resolved in `full-design-spec.md`**

Replace the Appendix C bullet:
`- **Provider set + weighting** (Phase 4): default provider list and handling of provider disagreement/outages.`
with:
`- **Provider set + weighting** (Phase 4): **Resolved** — CoinGecko + Binance + a Static provider (Static bootstraps VTX:USD until listing); disagreement handled by a configurable cross-source median with strict defaults (min_providers=2, max_deviation=0.10, max_quote_age), skipping under-covered pairs. See [specs/2026-05-29-phase-4-feeder-sidecar-design.md](./specs/2026-05-29-phase-4-feeder-sidecar-design.md) §2 (D2/D3).`

- [ ] **Step 4: Verify markdown still renders (no broken fences) and commit**

Run: `GOTOOLCHAIN=go1.25.4 go build ./...` (sanity that nothing code-side changed)
Expected: success.

```bash
git add docs/technical-design.md docs/full-design-spec.md
git commit -m "docs: sync feeder loop model, config, and resolve Appendix C provider question"
```

---

## Task 18: Full verification (lint, test, build)

**Files:** none (verification only)

- [ ] **Step 1: Run the linter**

Run: `make lint`
Expected: no findings in `feeder/...`. Fix any reported issues (unused imports, error-check lints) and re-run.

- [ ] **Step 2: Run the full test suite with race detector**

Run: `make test`
Expected: PASS, including all `feeder/...` tests. (This runs `govet` + `test-race`.)

- [ ] **Step 3: Build both binaries**

Run: `make build && make feeder-build`
Expected: `build/vertixd` and `build/vertix-feeder` both produced.

- [ ] **Step 4: Commit any lint fixes**

```bash
git add -A
git commit -m "chore(feeder): satisfy lint and finalize phase 4 feeder"
```

> If steps 1–3 are already clean with nothing to commit, skip the commit.

---

## Manual / node-verified acceptance (not CI)

These exercise the live gRPC broadcaster against a running devnet node and satisfy the spec acceptance gate items that cannot run in CI. Record results in the PR description.

- [ ] Start a single-node devnet (`make devnet-reset`), create a feeder key (`vertixd keys add feeder --keyring-backend test`), fund it, and delegate it: `vertixd tx oracle set-feeder $(vertixd keys show <validator> -a --bech val) $(vertixd keys show feeder -a) --from <operator>`.
- [ ] Point `feeder-config.yaml` at the node (`node_grpc: localhost:9090`), run `./build/vertix-feeder --config feeder-config.yaml`, and confirm: feeds broadcast once per window (`vertixd q oracle price BTC:USD` returns an aggregated price), `:9200/metrics` exposes the five series, and killing a provider (block its URL) degrades gracefully without a crash.

---

## Self-Review (completed by plan author)

**Spec coverage:** §1 goal → Tasks 1–16; §2 D1 hybrid → Tasks 10/13/14; D2 static → Task 4 + 16; D3 quality → Tasks 2/7/10; D4 batched tx → Tasks 11/12; D5 keyring/sequence → Task 12; D6 resilience → Tasks 10/12/13 (skip/retry/no-crash); D7 Approach A → Tasks 11/12; D8 delegated key → Task 12 (`feederAddr` from keyring, `validator` in body); D9 in-module binary → Task 1 + layout. §3 layout → file table + tasks. §5 config → Tasks 2/16. §6 data flow → Tasks 10/13. §7 providers → Tasks 3–6. §8 metrics → Task 9. §9 error handling → Tasks 10/12/13. §11 acceptance gate → CI tasks + manual section. §12 doc-syncs → Task 17.

**Placeholder scan:** No "TBD/TODO". The one literal `VTX:USD: "0.10"` is an intentional, documented bootstrap value. The Task 11 scaffolding-then-clean note explicitly instructs deleting the illustrative block and using the clean version.

**Type consistency:** `FeedSubmission{Pair, Price}`, `Broadcaster.SubmitFeeds`, `HeightFunc`, `CrossSourceMedian(prices, maxDev, minProviders)`, `Cache.Fresh/Set/MarkUnhealthy`, `Config`/`QualityConfig`/`PairConfig` field names are used identically across Tasks 2–16. `NewPriceLoop`/`NewSubmitLoop`/`NewWithDeps`/`New` signatures match their call sites.
