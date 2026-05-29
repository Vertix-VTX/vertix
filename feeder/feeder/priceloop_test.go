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
