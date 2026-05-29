package app_test

import (
	"testing"

	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	icagenesistypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/genesis/types"
	icatypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/app"
)

func newIBCTestApp(t *testing.T) *app.App {
	t.Helper()
	a, err := app.New(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		simtestutil.NewAppOptionsWithFlagHome(t.TempDir()),
	)
	require.NoError(t, err)
	return a
}

func TestTransferModuleWired(t *testing.T) {
	a := newIBCTestApp(t)
	_, ok := a.ModuleManager.Modules[ibctransfertypes.ModuleName]
	require.True(t, ok, "ics-20 transfer module must be wired")
	require.NotNil(t, a.TransferKeeper)
}

func TestICS20RouteRegistered(t *testing.T) {
	a := newIBCTestApp(t)
	router := a.IBCKeeper.PortKeeper.Router
	require.NotNil(t, router, "ibc port router must be set")
	require.True(t, router.HasRoute(ibctransfertypes.ModuleName), "transfer route must be registered")
}

func TestICAHostLockedAtGenesis(t *testing.T) {
	a := newIBCTestApp(t)
	genesis := a.DefaultGenesis()

	icaRaw, ok := genesis[icatypes.ModuleName]
	require.True(t, ok, "interchainaccounts genesis must be present")

	var icaGen icagenesistypes.GenesisState
	a.AppCodec().MustUnmarshalJSON(icaRaw, &icaGen)

	require.Empty(t, icaGen.HostGenesisState.Params.AllowMessages,
		"ICA host allow_messages must be empty (inert host surface)")
	require.False(t, icaGen.ControllerGenesisState.Params.ControllerEnabled,
		"ICA controller must be disabled at launch")
}
