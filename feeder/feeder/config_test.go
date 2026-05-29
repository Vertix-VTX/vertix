package feeder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/vertix-network/vertix/app"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

const validYAML = `
chain_id: vertix-devnet-1
node_grpc: localhost:9090
validator: vtxvaloper1h4aqve7gyktgce2mwm7ae2rmuztsljenwcpe2m
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
