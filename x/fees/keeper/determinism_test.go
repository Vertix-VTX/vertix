package keeper_test

import (
	"context"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	keepertest "github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/fees/keeper"
	"github.com/vertix-network/vertix/x/fees/types"
)

// runDeterministicEndBlock mirrors abci_test setup: fee collector balance, default
// burn_ratio, EndBlocker once. Returns burned coins for cross-run comparison.
func runDeterministicEndBlock(t *testing.T, feeCollectorAmt int64) sdk.Coins {
	t.Helper()
	bank := keepertest.NewMockBank()
	genesisSupply := math.NewInt(feeCollectorAmt)
	require.NoError(t, bank.MintCoins(
		context.Background(),
		"genesis",
		sdk.NewCoins(sdk.NewCoin(types.FeeDenom, genesisSupply)),
	))
	bank.SetModuleBalance(authtypes.FeeCollectorName, coins(types.FeeDenom, feeCollectorAmt))

	k, ctx := keepertest.FeesKeeper(t, bank)
	require.NoError(t, k.SetGenesisSupply(ctx, genesisSupply))

	beforeBurned, err := k.GetCumulativeBurned(ctx)
	require.NoError(t, err)

	require.NoError(t, k.EndBlocker(ctx))

	require.True(t, bank.ModuleBalance(types.ModuleName).IsZero())

	afterBurned, err := k.GetCumulativeBurned(ctx)
	require.NoError(t, err)
	require.Equal(t, bank.Burned.AmountOf(types.FeeDenom), afterBurned.Sub(beforeBurned))

	_, broken := keeper.ReconcileInvariant(k)(ctx)
	require.False(t, broken)

	return bank.Burned
}

func TestEndBlockerDeterminism(t *testing.T) {
	const feeCollectorAmt int64 = 1000

	burn1 := runDeterministicEndBlock(t, feeCollectorAmt)
	burn2 := runDeterministicEndBlock(t, feeCollectorAmt)

	require.Equal(t, burn1, burn2)
	require.Equal(t, coins(types.FeeDenom, 400), burn1)
}
