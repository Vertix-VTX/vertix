package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
)

func TestComputeFee(t *testing.T) {
	k, _ := keeper.RWAKeeper(t, keeper.NewMockBank(), keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	// 1_000_000 * 0.001 = 1000
	fee := k.ComputeFee(math.NewInt(1_000_000), math.LegacyNewDecWithPrec(1, 3))
	require.Equal(t, math.NewInt(1000), fee)
	// floor: 7 * 0.001 = 0.007 -> 0
	require.Equal(t, math.ZeroInt(), k.ComputeFee(math.NewInt(7), math.LegacyNewDecWithPrec(1, 3)))
}

func TestLockAndReleaseBond(t *testing.T) {
	bank := keeper.NewMockBank()
	k, ctx := keeper.RWAKeeper(t, bank, keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())

	issuer := sdkAcc(t)
	bank.SetBalance(issuer, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(10000))))

	require.NoError(t, k.LockBond(ctx, issuer, math.NewInt(6000)))
	require.Equal(t, math.NewInt(6000), bank.GetBalance(ctx, k.ModuleAddress(), "uvtx").Amount)
	require.Equal(t, math.NewInt(4000), bank.GetBalance(ctx, issuer, "uvtx").Amount)

	require.NoError(t, k.ReleaseBond(ctx, issuer, math.NewInt(6000)))
	require.Equal(t, math.NewInt(10000), bank.GetBalance(ctx, issuer, "uvtx").Amount)
	require.True(t, bank.GetBalance(ctx, k.ModuleAddress(), "uvtx").IsZero())
}

func TestSlashBondToCommunityPool(t *testing.T) {
	bank := keeper.NewMockBank()
	distr := keeper.NewMockDistribution()
	k, ctx := keeper.RWAKeeper(t, bank, keeper.MockOracle{Price: math.LegacyNewDec(1)}, distr)

	bank.SetBalance(k.ModuleAddress(), sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(5000))))
	require.NoError(t, k.SlashBondToCommunityPool(ctx, math.NewInt(5000)))
	require.Equal(t, math.NewInt(5000), distr.Funded.AmountOf("uvtx"))

	_ = authtypes.FeeCollectorName // keep import
}
