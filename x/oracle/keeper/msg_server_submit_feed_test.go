package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
	oraclekeeper "github.com/vertix-network/vertix/x/oracle/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestMsgSubmitFeedAuthorized(t *testing.T) {
	valAcc := sdk.ValAddress(sdk.MustAccAddressFromBech32(sample.AccAddress()).Bytes())
	valStr := valAcc.String()
	// implicit operator: no delegation → feeder is valoper bytes as account address
	feeder := sdk.AccAddress(valAcc.Bytes())
	mockStaking := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{{OperatorAddress: valStr, Status: stakingtypes.Bonded}},
		Powers:     map[string]int64{valStr: 100},
	}
	ctx, k := keeper.OracleKeeper(t, mockStaking, &keeper.MockSlashing{})
	server := oraclekeeper.NewMsgServerImpl(k)

	_, err := server.SubmitFeed(ctx, &types.MsgSubmitFeed{
		Feeder: feeder.String(), Validator: valStr, Pair: "BTC:USD", Price: "42000",
	})
	require.NoError(t, err)

	feed, ok, err := k.GetFeed(ctx, valAcc, "BTC:USD")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "42000", feed.Price)
}

func TestMsgSubmitFeedUnauthorizedFeeder(t *testing.T) {
	valAcc := sdk.ValAddress(sdk.MustAccAddressFromBech32(sample.AccAddress()).Bytes())
	valStr := valAcc.String()
	mockStaking := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{{OperatorAddress: valStr, Status: stakingtypes.Bonded}},
		Powers:     map[string]int64{valStr: 100},
	}
	ctx, k := keeper.OracleKeeper(t, mockStaking, &keeper.MockSlashing{})
	server := oraclekeeper.NewMsgServerImpl(k)

	delegated := sample.AccAddress()
	_, err := server.SetFeeder(ctx, &types.MsgSetFeeder{Validator: valStr, Feeder: delegated})
	require.NoError(t, err)

	wrongFeeder := sample.AccAddress()
	_, err = server.SubmitFeed(ctx, &types.MsgSubmitFeed{
		Feeder: wrongFeeder, Validator: valStr, Pair: "BTC:USD", Price: "42000",
	})
	require.ErrorIs(t, err, types.ErrFeederNotAuthorized)
}
