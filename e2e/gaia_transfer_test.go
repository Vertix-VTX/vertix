//go:build realnet

package e2e_test

import (
	"context"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/vertix-network/vertix/e2e/helpers"
)

// Vertix <-> Cosmos Hub (gaia) realism test. Opt-in: `go test -tags realnet`.
func TestVertixToGaia_VTXTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping realnet e2e in -short mode")
	}
	ctx := context.Background()

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		helpers.VertixChainSpec("vertix-a-1", 1, 0),
		{Name: "gaia", ChainName: "gaia", Version: "v19.2.0", NumValidators: ptr(1), NumFullNodes: ptr(0)},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	vertix := chains[0].(*cosmos.CosmosChain)
	gaia := chains[1].(*cosmos.CosmosChain)

	rly := interchaintest.NewBuiltinRelayerFactory(ibc.Hermes, zaptest.NewLogger(t))
	r := rly.Build(nil, nil, "")

	const ibcPath = "vertix-gaia"
	ic := interchaintest.NewInterchain().
		AddChain(vertix).AddChain(gaia).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{Chain1: vertix, Chain2: gaia, Relayer: r, Path: ibcPath})

	rep := testreporter.NewNopReporter()
	client, network := interchaintest.DockerSetup(t)
	require.NoError(t, ic.Build(ctx, rep.RelayerExecReporter(t), interchaintest.InterchainBuildOptions{
		TestName: t.Name(), Client: client, NetworkID: network,
	}))
	t.Cleanup(func() { _ = ic.Close() })

	amount := sdkmath.NewInt(100_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", amount, vertix, gaia)
	userVertix, userGaia := users[0], users[1]

	channels, err := r.GetChannels(ctx, rep.RelayerExecReporter(t), vertix.Config().ChainID)
	require.NoError(t, err)
	require.NotEmpty(t, channels)
	ch := channels[0]

	transferAmt := sdkmath.NewInt(1_000_000)
	tx, err := vertix.SendIBCTransfer(ctx, ch.ChannelID, userVertix.KeyName(), ibc.WalletAmount{
		Address: userGaia.FormattedAddress(), Denom: "uvtx", Amount: transferAmt,
	}, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate())
	require.NoError(t, testutil.WaitForBlocks(ctx, 10, vertix, gaia))

	ibcDenom := ibc.GetTransferChannelDenom(ch.Counterparty.PortID, ch.Counterparty.ChannelID, "uvtx")
	balGaia, err := gaia.GetBalance(ctx, userGaia.FormattedAddress(), ibcDenom)
	require.NoError(t, err)
	require.True(t, balGaia.Equal(transferAmt), "gaia receiver should hold the uvtx voucher")
}

func ptr[T any](v T) *T { return &v }
