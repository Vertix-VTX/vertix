package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	feeskeeper "github.com/vertix-network/vertix/x/fees/keeper"
	"github.com/vertix-network/vertix/x/fees/types"
)

// Spec §11: fee burn reduces supply; remainder left for native distribution;
// governance can change ratios and the new split applies next block.
func TestAcceptance_BurnThenGovernanceChange(t *testing.T) {
	bank := keeper.NewMockBank()
	bank.SetModuleBalance(authtypes.FeeCollectorName, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(1000))))
	k, ctx := keeper.FeesKeeper(t, bank)

	// default 40% burn
	require.NoError(t, k.EndBlocker(ctx))
	require.Equal(t, math.NewInt(400), bank.Burned.AmountOf("uvtx"))

	// governance raises burn to 50%
	server := feeskeeper.NewMsgServerImpl(k)
	_, err := server.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: k.GetAuthority(),
		Params:    types.FeesParams{BurnRatio: "0.50", DistributionRatio: "0.50"},
	})
	require.NoError(t, err)

	// next block: collector has the 600 remainder; 50% → burn 300 more (total 700)
	require.NoError(t, k.EndBlocker(ctx))
	require.Equal(t, math.NewInt(700), bank.Burned.AmountOf("uvtx"))
	require.Equal(t, math.NewInt(300), bank.ModuleBalance(authtypes.FeeCollectorName).AmountOf("uvtx"))
}

// Spec §11 / D2: non-uvtx denoms are never burned.
func TestAcceptance_NonUvtxNeverBurned(t *testing.T) {
	bank := keeper.NewMockBank()
	bank.SetModuleBalance(authtypes.FeeCollectorName,
		sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(1000)), sdk.NewCoin("rwa/gold", math.NewInt(999))))
	k, ctx := keeper.FeesKeeper(t, bank)
	require.NoError(t, k.EndBlocker(ctx))
	require.True(t, bank.Burned.AmountOf("rwa/gold").IsZero())
}
