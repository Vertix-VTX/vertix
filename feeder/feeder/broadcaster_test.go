package feeder

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	_ "github.com/vertix-network/vertix/app"
	oracletypes "github.com/vertix-network/vertix/x/oracle/types"
)

func testAddrs(t *testing.T) (feeder, validator string) {
	t.Helper()
	priv := secp256k1.GenPrivKey()
	acc := sdk.AccAddress(priv.PubKey().Address())
	val := sdk.ValAddress(priv.PubKey().Address())
	return acc.String(), val.String()
}

func TestBuildMsgsValidates(t *testing.T) {
	feeder, validator := testAddrs(t)

	feeds := []FeedSubmission{
		{Pair: "BTC:USD", Price: dec(t, "65000.5")},
		{Pair: "ETH:USD", Price: dec(t, "3200")},
	}
	msgs, err := buildMsgs(feeder, validator, feeds)
	require.NoError(t, err)
	require.Len(t, msgs, 2)

	m0, ok := msgs[0].(*oracletypes.MsgSubmitFeed)
	require.True(t, ok)
	require.Equal(t, "BTC:USD", m0.Pair)
	require.Equal(t, feeder, m0.Feeder)
	require.Equal(t, validator, m0.Validator)
	require.Equal(t, "65000.500000000000000000", m0.Price)
	require.NoError(t, m0.ValidateBasic())
}

func TestNewCodecRegistersOracle(t *testing.T) {
	c := newCodec()
	require.NotNil(t, c.Marshaler)
	require.NotNil(t, c.TxConfig)
	_, err := c.InterfaceRegistry.Resolve("/vertix.oracle.v1.MsgSubmitFeed")
	require.NoError(t, err)
}
