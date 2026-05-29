package keeper_test

import (
	"encoding/binary"
	"testing"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
	"github.com/vertix-network/vertix/x/rwa/types"
)

const (
	gasAllowlistSizeSmall = 1
	gasAllowlistSizeLarge = 1000
	// Per-check gas must not scale with total allow-list size; IAVL depth adds
	// only O(log N) store cost — allow a small constant slack vs N=1.
	gasAllowlistTolerance = 25_000
)

// TestGasBounds asserts transfer-restriction membership checks do not enumerate
// the allow-list: gas for SendRestriction is independent of how many other
// members exist (Phase 3 D4 keyed Has lookups).
func TestGasBounds(t *testing.T) {
	t.Helper()

	gasSmall := measureRestrictionCheckGas(t, gasAllowlistSizeSmall)
	gasLarge := measureRestrictionCheckGas(t, gasAllowlistSizeLarge)

	var diff uint64
	if gasLarge >= gasSmall {
		diff = gasLarge - gasSmall
	} else {
		diff = gasSmall - gasLarge
	}
	require.LessOrEqual(t, diff, uint64(gasAllowlistTolerance),
		"allow-list size must not materially affect restriction check gas (small=%d large=%d)",
		gasSmall, gasLarge)
}

func measureRestrictionCheckGas(t *testing.T, allowlistMembers int) uint64 {
	t.Helper()

	k, ctx := keeper.RWAKeeper(t, keeper.NewMockBank(), keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	const assetID = "gold"
	require.NoError(t, k.SetAsset(ctx, types.AssetRecord{
		AssetId: assetID, Issuer: sample.AccAddress(), Status: types.AssetStatus_ASSET_STATUS_ACTIVE,
		Denom: "rwa/gold", Bond: "1", NotionalMinted: "0", AllowAll: false,
	}))

	from, to := uniqueAccAddress(t), uniqueAccAddress(t)
	for i := 0; i < allowlistMembers; i++ {
		require.NoError(t, k.AddAllowlist(ctx, assetID, fillerAllowlistAddr(i)))
	}
	require.NoError(t, k.AddAllowlist(ctx, assetID, from))
	require.NoError(t, k.AddAllowlist(ctx, assetID, to))

	ctx = ctx.WithGasMeter(storetypes.NewGasMeter(1_000_000_000)).
		WithKVGasConfig(storetypes.KVGasConfig())

	_, err := k.SendRestriction(ctx, from, to, sdk.NewCoins(sdk.NewCoin("rwa/gold", math.NewInt(1))))
	require.NoError(t, err)
	return ctx.GasMeter().GasConsumed()
}

func uniqueAccAddress(t *testing.T) sdk.AccAddress {
	t.Helper()
	addr, err := sdk.AccAddressFromBech32(sample.AccAddress())
	require.NoError(t, err)
	return addr
}

func fillerAllowlistAddr(i int) sdk.AccAddress {
	b := make([]byte, 20)
	binary.BigEndian.PutUint32(b[16:], uint32(i+1))
	return sdk.AccAddress(b)
}
