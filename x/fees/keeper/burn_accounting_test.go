package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	keepertest "github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/fees/types"
)

func TestCumulativeBurnedAccounting(t *testing.T) {
	bank := keepertest.NewMockBank()
	k, ctx := keepertest.FeesKeeper(t, bank)

	// Defaults to zero before any burn.
	got, err := k.GetCumulativeBurned(ctx)
	require.NoError(t, err)
	require.True(t, got.IsZero())

	require.NoError(t, k.AddCumulativeBurned(ctx, math.NewInt(100)))
	require.NoError(t, k.AddCumulativeBurned(ctx, math.NewInt(50)))

	got, err = k.GetCumulativeBurned(ctx)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(150), got)
}

func TestGenesisSupplySnapshot(t *testing.T) {
	bank := keepertest.NewMockBank()
	bank.SetModuleBalance("mint-not-used", sdk.NewCoins()) // no-op to keep import
	k, ctx := keepertest.FeesKeeper(t, bank)

	require.NoError(t, k.SetGenesisSupply(ctx, math.NewInt(21_000_000_000_000)))
	got, err := k.GetGenesisSupply(ctx)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(21_000_000_000_000), got)
	_ = types.FeeDenom
}
