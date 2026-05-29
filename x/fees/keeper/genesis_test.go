package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/fees/types"
)

func TestGenesisRoundTrip(t *testing.T) {
	k, ctx := keeper.FeesKeeper(t, keeper.NewMockBank())
	gs := types.GenesisState{Params: types.FeesParams{BurnRatio: "0.30", DistributionRatio: "0.70"}}

	k.InitGenesis(ctx, gs)
	exported := k.ExportGenesis(ctx)
	require.Equal(t, gs.Params, exported.Params)

	k2, ctx2 := keeper.FeesKeeper(t, keeper.NewMockBank())
	k2.InitGenesis(ctx2, *exported)
	require.Equal(t, exported, k2.ExportGenesis(ctx2))
}
