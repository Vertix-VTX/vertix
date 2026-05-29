package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	keepertest "github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/fees/keeper"
	"github.com/vertix-network/vertix/x/fees/types"
)

func TestReconcileInvariant(t *testing.T) {
	bank := keepertest.NewMockBank()
	// Genesis supply = 1000 uvtx in circulation.
	require.NoError(t, bank.MintCoins(ctx0(), "circulating", sdk.NewCoins(sdk.NewCoin(types.FeeDenom, math.NewInt(1000)))))
	k, ctx := keepertest.FeesKeeper(t, bank)
	require.NoError(t, k.SetGenesisSupply(ctx, math.NewInt(1000)))

	// No burns yet, supply unchanged -> healthy.
	_, broken := keeper.ReconcileInvariant(k)(ctx)
	require.False(t, broken)

	// Burn 100: supply drops to 900, cumulative becomes 100 -> still reconciles.
	require.NoError(t, bank.BurnCoins(ctx, "circulating", sdk.NewCoins(sdk.NewCoin(types.FeeDenom, math.NewInt(100)))))
	require.NoError(t, k.AddCumulativeBurned(ctx, math.NewInt(100)))
	_, broken = keeper.ReconcileInvariant(k)(ctx)
	require.False(t, broken)

	// Corrupt: burn supply without recording -> broken (would indicate inflation/leak).
	require.NoError(t, bank.BurnCoins(ctx, "circulating", sdk.NewCoins(sdk.NewCoin(types.FeeDenom, math.NewInt(10)))))
	_, broken = keeper.ReconcileInvariant(k)(ctx)
	require.True(t, broken)
}

func TestModuleBalanceInvariant(t *testing.T) {
	bank := keepertest.NewMockBank()
	k, ctx := keepertest.FeesKeeper(t, bank)

	// Empty module account -> healthy.
	_, broken := keeper.ModuleBalanceInvariant(k)(ctx)
	require.False(t, broken)

	// Stranded coins in the fees module account -> broken.
	bank.SetModuleBalance(types.ModuleName, sdk.NewCoins(sdk.NewCoin(types.FeeDenom, math.NewInt(5))))
	_, broken = keeper.ModuleBalanceInvariant(k)(ctx)
	require.True(t, broken)
}

func ctx0() sdk.Context { return sdk.Context{} }
