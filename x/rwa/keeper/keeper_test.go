package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestParamsRoundTrip(t *testing.T) {
	k, ctx := keeper.RWAKeeper(t, keeper.NewMockBank(), keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	got, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, types.DefaultParams(), got)
}

func TestAssetCRUD(t *testing.T) {
	k, ctx := keeper.RWAKeeper(t, keeper.NewMockBank(), keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())

	_, found := k.GetAsset(ctx, "gold")
	require.False(t, found)

	issuer := authtypes.NewModuleAddress("issuer").String()
	rec := types.AssetRecord{AssetId: "gold", Issuer: issuer, Status: types.AssetStatus_ASSET_STATUS_DRAFT, Denom: "rwa/gold", Bond: "10", NotionalMinted: "0"}
	require.NoError(t, k.SetAsset(ctx, rec))

	got, found := k.GetAsset(ctx, "gold")
	require.True(t, found)
	require.Equal(t, "rwa/gold", got.Denom)
}
