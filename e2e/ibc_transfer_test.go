package e2e_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibcconntypes "github.com/cosmos/ibc-go/v8/modules/core/03-connection/types"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/vertix-network/vertix/e2e/helpers"
)

func TestVertixToVertix_VTXTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping interchaintest e2e in -short mode")
	}
	ctx := context.Background()

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		helpers.VertixChainSpec("vertix-a-1", 1, 0),
		helpers.VertixChainSpec("vertix-b-1", 1, 0),
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	chainA := chains[0].(*cosmos.CosmosChain)
	chainB := chains[1].(*cosmos.CosmosChain)

	rly := interchaintest.NewBuiltinRelayerFactory(ibc.Hermes, zaptest.NewLogger(t))

	rep := testreporter.NewNopReporter()
	client, network := interchaintest.DockerSetup(t)
	r := rly.Build(t, client, network)

	const ibcPath = "vertix-vertix"
	ic := interchaintest.NewInterchain().
		AddChain(chainA).
		AddChain(chainB).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:  chainA,
			Chain2:  chainB,
			Relayer: r,
			Path:    ibcPath,
		})
	require.NoError(t, ic.Build(ctx, rep.RelayerExecReporter(t), interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
	}))
	t.Cleanup(func() { _ = ic.Close() })

	amount := sdkmath.NewInt(100_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", amount, chainA, chainB)
	userA, userB := users[0], users[1]

	channels, err := relayerChannels(ctx, t, r, rep, chainA.Config().ChainID)
	require.NoError(t, err)
	require.NotEmpty(t, channels)
	channelA, err := transferChannel(channels)
	require.NoError(t, err)

	transferAmt := sdkmath.NewInt(1_000_000)
	dstAddr := userB.FormattedAddress()
	tx, err := chainA.SendIBCTransfer(ctx, channelA.ChannelID, userA.KeyName(), ibc.WalletAmount{
		Address: dstAddr,
		Denom:   "uvtx",
		Amount:  transferAmt,
	}, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate())

	require.NoError(t, testutil.WaitForBlocks(ctx, 10, chainA, chainB))

	dstDenomTrace := "transfer/" + channelA.Counterparty.ChannelID + "/uvtx"
	ibcDenom := types.ParseDenomTrace(types.GetPrefixedDenom(
		channelA.Counterparty.PortID,
		channelA.Counterparty.ChannelID,
		"uvtx",
	)).IBCDenom()

	balB, err := chainB.GetBalance(ctx, dstAddr, ibcDenom)
	require.NoError(t, err)
	require.True(t, balB.Equal(transferAmt), "receiver should hold the uvtx voucher; trace=%s", dstDenomTrace)
}

func relayerChannels(
	ctx context.Context,
	t *testing.T,
	r ibc.Relayer,
	rep *testreporter.Reporter,
	chainID string,
) ([]ibc.ChannelOutput, error) {
	t.Helper()
	return r.GetChannels(ctx, rep.RelayerExecReporter(t), chainID)
}

func transferChannel(channels []ibc.ChannelOutput) (ibc.ChannelOutput, error) {
	for _, channel := range channels {
		state := channel.State
		if !strings.HasPrefix(state, "STATE_") {
			state = "STATE_" + strings.ToUpper(state)
		}
		if channel.PortID == "transfer" && state == ibcconntypes.OPEN.String() {
			return channel, nil
		}
	}
	return ibc.ChannelOutput{}, fmt.Errorf("no open transfer channel found")
}
