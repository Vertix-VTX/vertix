package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/sample"
	"github.com/vertix-network/vertix/x/fees/types"
)

func TestMsgUpdateParamsValidateBasic(t *testing.T) {
	msg := &types.MsgUpdateParams{
		Authority: sample.AccAddress(),
		Params:    types.DefaultParams(),
	}
	require.NoError(t, msg.ValidateBasic())

	bad := &types.MsgUpdateParams{
		Authority: "not-an-address",
		Params:    types.DefaultParams(),
	}
	require.Error(t, bad.ValidateBasic())

	badParams := &types.MsgUpdateParams{
		Authority: sample.AccAddress(),
		Params:    types.FeesParams{BurnRatio: "0.40", DistributionRatio: "0.50"},
	}
	require.Error(t, badParams.ValidateBasic())
}
