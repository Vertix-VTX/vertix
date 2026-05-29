package app_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	ibctesting "github.com/cosmos/ibc-go/v8/testing"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/app"
)

func init() {
	ibctesting.DefaultTestingAppInit = app.SetupTestingApp
}

// newTransferPath returns an ICS-20 path between two in-process Vertix chains.
func newTransferPath(t *testing.T) (*ibctesting.Coordinator, *ibctesting.Path) {
	t.Helper()
	coord := ibctesting.NewCoordinator(t, 2)
	chainA := coord.GetChain(ibctesting.GetChainID(1))
	chainB := coord.GetChain(ibctesting.GetChainID(2))

	path := ibctesting.NewPath(chainA, chainB)
	path.EndpointA.ChannelConfig.PortID = transfertypes.PortID
	path.EndpointB.ChannelConfig.PortID = transfertypes.PortID
	path.EndpointA.ChannelConfig.Version = transfertypes.Version
	path.EndpointB.ChannelConfig.Version = transfertypes.Version

	coord.Setup(path)
	return coord, path
}

func vertixApp(t *testing.T, chain *ibctesting.TestChain) *app.App {
	t.Helper()
	a, ok := chain.App.(*app.App)
	require.True(t, ok, "chain app must be *app.App")
	return a
}

func TestICS20Transfer_RoundTrip(t *testing.T) {
	coord, path := newTransferPath(t)
	chainA := coord.GetChain(ibctesting.GetChainID(1))
	chainB := coord.GetChain(ibctesting.GetChainID(2))

	appA := vertixApp(t, chainA)
	denom, err := appA.StakingKeeper.BondDenom(chainA.GetContext())
	require.NoError(t, err)

	amount := sdkmath.NewInt(1_000_000)
	coin := sdk.NewCoin(denom, amount)

	sender := chainA.SenderAccount.GetAddress()
	receiver := chainB.SenderAccount.GetAddress()
	timeout := clienttypes.NewHeight(1, 1000)

	msg := transfertypes.NewMsgTransfer(
		path.EndpointA.ChannelConfig.PortID,
		path.EndpointA.ChannelID,
		coin, sender.String(), receiver.String(),
		timeout, 0, "",
	)

	res, err := chainA.SendMsgs(msg)
	require.NoError(t, err)

	packet, err := ibctesting.ParsePacketFromEvents(res.Events)
	require.NoError(t, err)

	// Relay the packet A -> B and the ack back.
	require.NoError(t, path.RelayPacket(packet))

	// Voucher exists on chain B.
	prefixed := transfertypes.GetPrefixedDenom(
		path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID, denom)
	ibcDenom := transfertypes.ParseDenomTrace(prefixed).IBCDenom()

	appB := vertixApp(t, chainB)
	balB := appB.BankKeeper.GetBalance(chainB.GetContext(), receiver, ibcDenom)
	require.Equal(t, amount, balB.Amount, "receiver must hold the ICS-20 voucher")
}
