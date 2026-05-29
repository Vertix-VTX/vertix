package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/fees/types"
)

func coins(denom string, amt int64) sdk.Coins {
	return sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(amt)))
}

func TestEndBlockerBurns40Percent(t *testing.T) {
	bank := keeper.NewMockBank()
	bank.SetModuleBalance(authtypes.FeeCollectorName, coins("uvtx", 1000))
	k, ctx := keeper.FeesKeeper(t, bank)

	require.NoError(t, k.EndBlocker(ctx))

	// 1000 × 0.40 = 400 burned; 600 left in the collector for native distribution.
	require.Equal(t, coins("uvtx", 400), bank.Burned)
	require.Equal(t, math.NewInt(600), bank.ModuleBalance(authtypes.FeeCollectorName).AmountOf("uvtx"))
	require.True(t, bank.ModuleBalance(types.ModuleName).IsZero()) // fees account empty between blocks
}

func TestEndBlockerFloorsBurn(t *testing.T) {
	bank := keeper.NewMockBank()
	bank.SetModuleBalance(authtypes.FeeCollectorName, coins("uvtx", 7)) // 7 × 0.40 = 2.8 → floor 2
	k, ctx := keeper.FeesKeeper(t, bank)

	require.NoError(t, k.EndBlocker(ctx))

	require.Equal(t, coins("uvtx", 2), bank.Burned)
	require.Equal(t, math.NewInt(5), bank.ModuleBalance(authtypes.FeeCollectorName).AmountOf("uvtx"))
}

func TestEndBlockerLeavesNonUvtxUntouched(t *testing.T) {
	bank := keeper.NewMockBank()
	bank.SetModuleBalance(authtypes.FeeCollectorName,
		coins("uvtx", 1000).Add(sdk.NewCoin("rwa/gold", math.NewInt(500))))
	k, ctx := keeper.FeesKeeper(t, bank)

	require.NoError(t, k.EndBlocker(ctx))

	require.Equal(t, coins("uvtx", 400), bank.Burned)
	require.Equal(t, math.NewInt(500),
		bank.ModuleBalance(authtypes.FeeCollectorName).AmountOf("rwa/gold")) // untouched
}

func TestEndBlockerZeroBalanceNoOp(t *testing.T) {
	bank := keeper.NewMockBank()
	k, ctx := keeper.FeesKeeper(t, bank)
	require.NoError(t, k.EndBlocker(ctx))
	require.True(t, bank.Burned.IsZero())
}

func TestEndBlockerZeroBurnRatioNoOp(t *testing.T) {
	bank := keeper.NewMockBank()
	bank.SetModuleBalance(authtypes.FeeCollectorName, coins("uvtx", 1000))
	k, ctx := keeper.FeesKeeper(t, bank)
	require.NoError(t, k.SetParams(ctx, types.FeesParams{BurnRatio: "0", DistributionRatio: "1"}))

	require.NoError(t, k.EndBlocker(ctx))
	require.True(t, bank.Burned.IsZero())
	require.Equal(t, math.NewInt(1000), bank.ModuleBalance(authtypes.FeeCollectorName).AmountOf("uvtx"))
}

func TestEndBlockerDustNoOp(t *testing.T) {
	bank := keeper.NewMockBank()
	bank.SetModuleBalance(authtypes.FeeCollectorName, coins("uvtx", 1)) // 1 × 0.40 = 0.4 → floor 0
	k, ctx := keeper.FeesKeeper(t, bank)
	require.NoError(t, k.EndBlocker(ctx))
	require.True(t, bank.Burned.IsZero())
}
