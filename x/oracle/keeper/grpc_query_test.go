package keeper_test

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestPriceQueryFresh(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	now := time.Unix(2000, 0).UTC()
	ctx = ctx.WithBlockTime(now)
	ap := types.AggregatedPrice{
		Pair:        "BTC:USD",
		Price:       "50000",
		BlockHeight: 1,
		BlockTime:   now,
	}
	require.NoError(t, k.SetAggregatedPrice(ctx, ap))

	resp, err := k.Price(ctx, &types.QueryPriceRequest{Pair: "BTC:USD"})
	require.NoError(t, err)
	require.Equal(t, ap, resp.Price)
}

func TestPriceQueryNoPrice(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})

	_, err := k.Price(ctx, &types.QueryPriceRequest{Pair: "BTC:USD"})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.NotFound, st.Code())
}

func TestPriceQueryStale(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	now := time.Unix(2000, 0).UTC()
	ctx = ctx.WithBlockTime(now)
	require.NoError(t, k.SetAggregatedPrice(ctx, types.AggregatedPrice{
		Pair:        "BTC:USD",
		Price:       "50000",
		BlockHeight: 1,
		BlockTime:   now.Add(-301 * time.Second),
	}))

	_, err := k.Price(ctx, &types.QueryPriceRequest{Pair: "BTC:USD"})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestTwapQuery(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	base := time.Unix(1000, 0).UTC()
	ctx = ctx.WithBlockTime(base.Add(250 * time.Second))
	require.NoError(t, k.AppendTWAPEntry(ctx, "BTC:USD", twapDec(t, "100"), base))
	require.NoError(t, k.AppendTWAPEntry(ctx, "BTC:USD", twapDec(t, "200"), base.Add(100*time.Second)))
	require.NoError(t, k.AppendTWAPEntry(ctx, "BTC:USD", twapDec(t, "300"), base.Add(200*time.Second)))

	resp, err := k.Twap(ctx, &types.QueryTwapRequest{Pair: "BTC:USD", WindowSeconds: 200})
	require.NoError(t, err)
	require.True(t, resp.Price.Equal(twapDec(t, "180")))
}

func TestTwapQueryNoData(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})

	_, err := k.Twap(ctx, &types.QueryTwapRequest{Pair: "BTC:USD", WindowSeconds: 60})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.NotFound, st.Code())
}

func TestParamsQuery(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	params := types.DefaultParams()
	require.NoError(t, k.SetParams(ctx, params))

	resp, err := k.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, params, resp.Params)
}

func TestMissCounterQuery(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	val := sdk.ValAddress([]byte("validator123456789012345678901234"))
	_, err := k.IncMissCounter(ctx, val)
	require.NoError(t, err)
	_, err = k.IncMissCounter(ctx, val)
	require.NoError(t, err)
	_, err = k.IncTotalWindows(ctx, val)
	require.NoError(t, err)
	_, err = k.IncTotalWindows(ctx, val)
	require.NoError(t, err)
	_, err = k.IncTotalWindows(ctx, val)
	require.NoError(t, err)

	resp, err := k.MissCounter(ctx, &types.QueryMissCounterRequest{Validator: val.String()})
	require.NoError(t, err)
	require.Equal(t, int64(2), resp.Misses)
	require.Equal(t, int64(3), resp.TotalWindows)
}

func TestFeederQueryExplicit(t *testing.T) {
	valAcc := sdk.ValAddress(sdk.MustAccAddressFromBech32(sample.AccAddress()).Bytes())
	valStr := valAcc.String()
	feeder := sample.AccAddress()
	mockStaking := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{
			{OperatorAddress: valStr, Status: stakingtypes.Bonded},
		},
	}
	ctx, k := keeper.OracleKeeper(t, mockStaking, &keeper.MockSlashing{})
	require.NoError(t, k.SetFeederDelegation(ctx, valAcc, sdk.MustAccAddressFromBech32(feeder)))

	resp, err := k.Feeder(ctx, &types.QueryFeederRequest{Validator: valStr})
	require.NoError(t, err)
	require.Equal(t, feeder, resp.Feeder)
}

func TestFeederQueryImplicit(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	val := sdk.ValAddress([]byte("validator123456789012345678901234"))

	resp, err := k.Feeder(ctx, &types.QueryFeederRequest{Validator: val.String()})
	require.NoError(t, err)
	require.Equal(t, sdk.AccAddress(val.Bytes()).String(), resp.Feeder)
}
