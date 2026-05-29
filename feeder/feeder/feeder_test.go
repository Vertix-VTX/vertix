package feeder

import (
	"context"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	_ "github.com/vertix-network/vertix/app"
	"github.com/vertix-network/vertix/feeder/feeder/provider"
)

func mustValAddr(t *testing.T) string {
	t.Helper()
	return sdk.ValAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
}

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
