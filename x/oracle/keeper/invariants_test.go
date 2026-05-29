package keeper_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	keepertest "github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/oracle/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestPricesInvariant(t *testing.T) {
	ctx, k := keepertest.OracleKeeper(t, &keepertest.MockStaking{}, &keepertest.MockSlashing{})

	params := types.DefaultParams()
	params.AcceptList = []string{"VTX:USD"}
	require.NoError(t, k.SetParams(ctx, params))

	require.NoError(t, k.SetAggregatedPrice(ctx, types.AggregatedPrice{
		Pair: "VTX:USD", Price: "1.50", BlockHeight: 1, BlockTime: time.Unix(100, 0),
	}))
	_, broken := keeper.PricesInvariant(k)(ctx)
	require.False(t, broken)

	require.NoError(t, k.SetAggregatedPrice(ctx, types.AggregatedPrice{
		Pair: "ZZZ:USD", Price: "2.00", BlockHeight: 1, BlockTime: time.Unix(100, 0),
	}))
	_, broken = keeper.PricesInvariant(k)(ctx)
	require.True(t, broken)
}

func TestPricesInvariantRejectsNonPositive(t *testing.T) {
	ctx, k := keepertest.OracleKeeper(t, &keepertest.MockStaking{}, &keepertest.MockSlashing{})
	params := types.DefaultParams()
	params.AcceptList = []string{"VTX:USD"}
	require.NoError(t, k.SetParams(ctx, params))

	require.NoError(t, k.SetAggregatedPrice(ctx, types.AggregatedPrice{
		Pair: "VTX:USD", Price: "0", BlockHeight: 1, BlockTime: time.Unix(100, 0),
	}))
	_, broken := keeper.PricesInvariant(k)(ctx)
	require.True(t, broken)
}
