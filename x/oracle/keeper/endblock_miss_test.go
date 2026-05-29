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

func TestEndBlockMissTumblingWindow(t *testing.T) {
	v1, s1 := makeValidator(t)
	v2, s2 := makeValidator(t)
	mockStaking := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{
			bondingValidator(t, v1),
			bondingValidator(t, v2),
		},
		Powers:     map[string]int64{s1: 70, s2: 30},
		TotalPower: math.NewInt(100),
	}
	mockSlashing := &keeper.MockSlashing{}
	ctx, k := keeper.OracleKeeper(t, mockStaking, mockSlashing)
	p := types.DefaultParams()
	p.VoteWindow = 1
	p.MissWindowSize = 3
	p.MissThreshold = "0.5"
	require.NoError(t, k.SetParams(ctx, p))
	server := oraclekeeper.NewMsgServerImpl(k)

	submit := func(val sdk.ValAddress, valStr string) {
		_, err := server.SubmitFeed(ctx, &types.MsgSubmitFeed{
			Feeder: sdk.AccAddress(val.Bytes()).String(), Validator: valStr,
			Pair: "BTC:USD", Price: "100",
		})
		require.NoError(t, err)
	}

	// Window 1: both submit (no miss)
	submit(v1, s1)
	submit(v2, s2)
	require.NoError(t, k.EndBlocker(ctx))
	require.Empty(t, mockSlashing.Calls)

	// Window 2: v2 misses quorum-live BTC (v1 alone meets quorum)
	submit(v1, s1)
	require.NoError(t, k.EndBlocker(ctx))
	require.Empty(t, mockSlashing.Calls)

	// Window 3: v2 misses again → 2/3 > 0.5 → slash at boundary
	submit(v1, s1)
	require.NoError(t, k.EndBlocker(ctx))
	require.Len(t, mockSlashing.Calls, 1)
}

func TestEndBlockNoMassSlashBelowQuorum(t *testing.T) {
	v1, s1 := makeValidator(t)
	v2, s2 := makeValidator(t)
	mockStaking := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{
			bondingValidator(t, v1),
			bondingValidator(t, v2),
		},
		Powers:     map[string]int64{s1: 10, s2: 10},
		TotalPower: math.NewInt(100),
	}
	mockSlashing := &keeper.MockSlashing{}
	ctx, k := keeper.OracleKeeper(t, mockStaking, mockSlashing)
	p := types.DefaultParams()
	p.VoteWindow = 1
	p.MissWindowSize = 1
	p.MissThreshold = "0.05"
	require.NoError(t, k.SetParams(ctx, p))
	server := oraclekeeper.NewMsgServerImpl(k)

	// Only v1 submits BTC; 20/100 power < quorum → quorumLive empty → no miss slash
	_, err := server.SubmitFeed(ctx, &types.MsgSubmitFeed{
		Feeder: sdk.AccAddress(v1.Bytes()).String(), Validator: s1,
		Pair: "BTC:USD", Price: "100",
	})
	require.NoError(t, err)

	require.NoError(t, k.EndBlocker(ctx))
	require.Empty(t, mockSlashing.Calls)

	miss, err := k.GetMissCounter(ctx, v2)
	require.NoError(t, err)
	require.Zero(t, miss)
}
