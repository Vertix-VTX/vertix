package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
	rwakeeper "github.com/vertix-network/vertix/x/rwa/keeper"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestBondInvariantHolds(t *testing.T) {
	bank := keeper.NewMockBank()
	k, ctx := keeper.RWAKeeper(t, bank, keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())

	require.NoError(t, k.SetAsset(ctx, types.AssetRecord{AssetId: "gold", Issuer: sample.AccAddress(), Status: types.AssetStatus_ASSET_STATUS_ACTIVE, Denom: "rwa/gold", Bond: "5000", NotionalMinted: "0", AllowAll: true}))
	bank.SetBalance(k.ModuleAddress(), sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(5000))))

	_, broken := rwakeeper.BondInvariant(k)(ctx)
	require.False(t, broken)
}

func TestBondInvariantBreaks(t *testing.T) {
	bank := keeper.NewMockBank()
	k, ctx := keeper.RWAKeeper(t, bank, keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())

	require.NoError(t, k.SetAsset(ctx, types.AssetRecord{AssetId: "gold", Issuer: sample.AccAddress(), Status: types.AssetStatus_ASSET_STATUS_ACTIVE, Denom: "rwa/gold", Bond: "5000", NotionalMinted: "0", AllowAll: true}))
	bank.SetBalance(k.ModuleAddress(), sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(1000))))

	_, broken := rwakeeper.BondInvariant(k)(ctx)
	require.True(t, broken)
}
