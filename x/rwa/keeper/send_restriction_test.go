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

func TestSendRestriction(t *testing.T) {
	k, ctx := keeper.RWAKeeper(t, keeper.NewMockBank(), keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	require.NoError(t, k.SetAsset(ctx, types.AssetRecord{AssetId: "gold", Issuer: sample.AccAddress(), Status: types.AssetStatus_ASSET_STATUS_ACTIVE, Denom: "rwa/gold", Bond: "1", NotionalMinted: "0", AllowAll: false}))

	a, b := sdkAcc(t), sdkAcc(t)

	// non-rwa coin: no-op pass
	_, err := k.SendRestriction(ctx, a, b, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(5))))
	require.NoError(t, err)

	// restricted rwa coin, neither allowlisted: blocked
	_, err = k.SendRestriction(ctx, a, b, sdk.NewCoins(sdk.NewCoin("rwa/gold", math.NewInt(1))))
	require.ErrorIs(t, err, types.ErrTransferRestricted)

	// after allowlisting both: allowed; returns to unchanged
	require.NoError(t, k.AddAllowlist(ctx, "gold", a))
	require.NoError(t, k.AddAllowlist(ctx, "gold", b))
	newTo, err := k.SendRestriction(ctx, a, b, sdk.NewCoins(sdk.NewCoin("rwa/gold", math.NewInt(1))))
	require.NoError(t, err)
	require.Equal(t, b, newTo)

	// module-origin leg always allowed
	_, err = k.SendRestriction(ctx, k.ModuleAddress(), a, sdk.NewCoins(sdk.NewCoin("rwa/gold", math.NewInt(1))))
	require.NoError(t, err)

	_ = rwakeeper.BuildDenom // keep import
}
