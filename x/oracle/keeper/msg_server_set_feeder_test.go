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

func TestMsgSetFeeder(t *testing.T) {
	valAcc := sdk.ValAddress(sdk.MustAccAddressFromBech32(sample.AccAddress()).Bytes())
	valStr := valAcc.String()
	feeder := sample.AccAddress()
	mockStaking := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{
			{OperatorAddress: valStr, Status: stakingtypes.Bonded},
		},
	}
	ctx, k := keeper.OracleKeeper(t, mockStaking, &keeper.MockSlashing{})
	server := oraclekeeper.NewMsgServerImpl(k)

	_, err := server.SetFeeder(ctx, &types.MsgSetFeeder{
		Validator: valStr,
		Feeder:    feeder,
	})
	require.NoError(t, err)

	got, ok, err := k.GetFeederForValidator(ctx, valAcc)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, feeder, got.String())
}
