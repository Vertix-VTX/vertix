package types_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/sample"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func init() {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("vtx", "vtxpub")
	cfg.SetBech32PrefixForValidator("vtxvaloper", "vtxvaloperpub")
	cfg.SetBech32PrefixForConsensusNode("vtxvalcons", "vtxvalconspub")
	cfg.Seal()
}

func valAddress() sdk.ValAddress {
	return sdk.ValAddress(sdk.MustAccAddressFromBech32(sample.AccAddress()).Bytes())
}

func TestMsgSubmitFeedValidateBasic(t *testing.T) {
	val := valAddress()
	feeder := sdk.MustAccAddressFromBech32(sample.AccAddress())
	msg := &types.MsgSubmitFeed{
		Feeder:    feeder.String(),
		Validator: val.String(),
		Pair:      "BTC:USD",
		Price:     "42000.5",
	}
	require.NoError(t, msg.ValidateBasic())

	bad := *msg
	bad.Price = "-1"
	require.Error(t, bad.ValidateBasic())
}

func TestMsgSetFeederValidateBasic(t *testing.T) {
	val := valAddress()
	feeder := sdk.MustAccAddressFromBech32(sample.AccAddress())
	require.NoError(t, (&types.MsgSetFeeder{Validator: val.String(), Feeder: feeder.String()}).ValidateBasic())
}
