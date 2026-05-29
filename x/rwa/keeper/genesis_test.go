package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestGenesisRoundTrip(t *testing.T) {
	k, ctx := keeper.RWAKeeper(t, keeper.NewMockBank(), keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())

	alice := sample.AccAddress()
	gs := types.GenesisState{
		Params: types.DefaultParams(),
		Assets: []types.AssetRecord{
			{AssetId: "gold", Issuer: sample.AccAddress(), Status: types.AssetStatus_ASSET_STATUS_DRAFT, OraclePair: "XAU:USD", Denom: "rwa/gold", Bond: "10000000000", NotionalMinted: "0", AllowAll: false},
		},
		Restrictions: []types.Restriction{
			{AssetId: "gold", Address: alice, IsDeny: false},
		},
	}

	k.InitGenesis(ctx, gs)
	exported := k.ExportGenesis(ctx)
	require.Equal(t, gs.Params, exported.Params)
	require.Len(t, exported.Assets, 1)
	require.Len(t, exported.Restrictions, 1)
	require.Equal(t, "gold", exported.Restrictions[0].AssetId)
}
