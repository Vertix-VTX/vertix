package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestCheckTransferAllowed_AllowAll(t *testing.T) {
	k, ctx := keeper.RWAKeeper(t, keeper.NewMockBank(), keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	require.NoError(t, k.SetAsset(ctx, types.AssetRecord{AssetId: "gold", Issuer: sample.AccAddress(), Status: types.AssetStatus_ASSET_STATUS_ACTIVE, Denom: "rwa/gold", Bond: "1", NotionalMinted: "0", AllowAll: true}))

	a, b := sdkAcc(t), sdkAcc(t)
	require.NoError(t, k.CheckTransferAllowed(ctx, "gold", a, b))
}

func TestCheckTransferAllowed_Allowlist(t *testing.T) {
	k, ctx := keeper.RWAKeeper(t, keeper.NewMockBank(), keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	require.NoError(t, k.SetAsset(ctx, types.AssetRecord{AssetId: "gold", Issuer: sample.AccAddress(), Status: types.AssetStatus_ASSET_STATUS_ACTIVE, Denom: "rwa/gold", Bond: "1", NotionalMinted: "0", AllowAll: false}))

	a, b := sdkAcc(t), sdkAcc(t)
	// neither allowlisted -> blocked
	require.ErrorIs(t, k.CheckTransferAllowed(ctx, "gold", a, b), types.ErrTransferRestricted)
	// both allowlisted -> allowed
	require.NoError(t, k.AddAllowlist(ctx, "gold", a))
	require.NoError(t, k.AddAllowlist(ctx, "gold", b))
	require.NoError(t, k.CheckTransferAllowed(ctx, "gold", a, b))
	// deny overrides allow
	require.NoError(t, k.AddDenylist(ctx, "gold", b))
	require.ErrorIs(t, k.CheckTransferAllowed(ctx, "gold", a, b), types.ErrTransferRestricted)
}

func TestCheckTransferAllowed_ModuleExempt(t *testing.T) {
	k, ctx := keeper.RWAKeeper(t, keeper.NewMockBank(), keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	require.NoError(t, k.SetAsset(ctx, types.AssetRecord{AssetId: "gold", Issuer: sample.AccAddress(), Status: types.AssetStatus_ASSET_STATUS_ACTIVE, Denom: "rwa/gold", Bond: "1", NotionalMinted: "0", AllowAll: false}))
	// module address as a party is always allowed (mint/burn/escrow)
	require.NoError(t, k.CheckTransferAllowed(ctx, "gold", k.ModuleAddress(), sdkAcc(t)))
}

func sdkAcc(t *testing.T) sdk.AccAddress {
	t.Helper()
	addr, err := sdk.AccAddressFromBech32(sample.AccAddress())
	require.NoError(t, err)
	return addr
}
