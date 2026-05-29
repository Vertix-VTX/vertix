package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	oraclekeeper "github.com/vertix-network/vertix/x/oracle/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestEndBlockSlashesOutlier(t *testing.T) {
	vals := make([]sdk.ValAddress, 4)
	strs := make([]string, 4)
	validators := make([]stakingtypes.Validator, 4)
	powers := make(map[string]int64)
	for i := range 4 {
		vals[i], strs[i] = makeValidator(t)
		validators[i] = bondingValidator(t, vals[i])
		powers[strs[i]] = 25
	}
	mockStaking := &keeper.MockStaking{
		Validators: validators,
		Powers:     powers,
		TotalPower: math.NewInt(100),
	}
	mockSlashing := &keeper.MockSlashing{}
	ctx, k := keeper.OracleKeeper(t, mockStaking, mockSlashing)
	p := types.DefaultParams()
	p.VoteWindow = 1
	require.NoError(t, k.SetParams(ctx, p))
	server := oraclekeeper.NewMsgServerImpl(k)

	prices := []string{"100", "100", "100", "200"}
	for i := range 4 {
		_, err := server.SubmitFeed(ctx, &types.MsgSubmitFeed{
			Feeder:    sdk.AccAddress(vals[i].Bytes()).String(),
			Validator: strs[i],
			Pair:      "BTC:USD",
			Price:     prices[i],
		})
		require.NoError(t, err)
	}

	require.NoError(t, k.EndBlocker(ctx))
	require.Len(t, mockSlashing.Calls, 1)
	require.Equal(t, powers[strs[3]], mockSlashing.Calls[0].Power)

	got, err := k.GetPrice(ctx, "BTC:USD")
	require.NoError(t, err)
	require.True(t, got.Equal(math.LegacyNewDec(100)))
}
