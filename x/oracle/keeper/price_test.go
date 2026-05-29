package keeper_test

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func priceDec(t *testing.T, s string) math.LegacyDec {
	t.Helper()
	d, err := math.LegacyNewDecFromStr(s)
	require.NoError(t, err)
	return d
}

func TestGetPriceNoPrice(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	_, err := k.GetPrice(ctx, "BTC:USD")
	require.ErrorIs(t, err, types.ErrNoPrice)
}

func TestGetPriceStale(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	now := time.Unix(2000, 0).UTC()
	ctx = ctx.WithBlockTime(now)
	staleTime := now.Add(-301 * time.Second)
	require.NoError(t, k.SetAggregatedPrice(ctx, types.AggregatedPrice{
		Pair:        "BTC:USD",
		Price:       "50000",
		BlockHeight: 1,
		BlockTime:   staleTime,
	}))
	_, err := k.GetPrice(ctx, "BTC:USD")
	require.ErrorIs(t, err, types.ErrStalePrice)
}

func TestGetPriceFresh(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	now := time.Unix(2000, 0).UTC()
	ctx = ctx.WithBlockTime(now)
	require.NoError(t, k.SetAggregatedPrice(ctx, types.AggregatedPrice{
		Pair:        "BTC:USD",
		Price:       "50000",
		BlockHeight: 1,
		BlockTime:   now,
	}))
	got, err := k.GetPrice(ctx, "BTC:USD")
	require.NoError(t, err)
	require.True(t, got.Equal(priceDec(t, "50000")))
}

func TestGetPricePairNotAccepted(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	_, err := k.GetPrice(ctx, "DOGE:USD")
	require.ErrorIs(t, err, types.ErrPairNotAccepted)
}
