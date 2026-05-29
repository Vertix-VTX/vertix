package app_test

import (
	"testing"

	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/app"
)

// newTestApp builds an in-memory App for wiring assertions.
func newTestApp(t *testing.T) *app.App {
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

func TestNoMintModule(t *testing.T) {
	a := newTestApp(t)

	_, hasModule := a.ModuleManager.Modules[minttypes.ModuleName]
	require.False(t, hasModule, "x/mint must not be registered (21M hard cap)")

	_, hasPerm := app.GetMaccPerms()[minttypes.ModuleName]
	require.False(t, hasPerm, "x/mint must have no module-account permission")

	require.Nil(t, a.GetKey(minttypes.StoreKey), "x/mint must have no store key")
}

func TestModulesWired(t *testing.T) {
	a := newTestApp(t)
	required := []string{
		"auth", "bank", "staking", "gov", "distribution", "slashing",
		"upgrade", "params", "crisis", "feegrant", "authz", "consensus",
		"genutil", "evidence", "vesting",
	}
	for _, name := range required {
		_, ok := a.ModuleManager.Modules[name]
		require.Truef(t, ok, "module %q must be wired", name)
	}
}

func TestGenesisRoundTrip(t *testing.T) {
	a := newTestApp(t)
	genState := a.DefaultGenesis()
	require.NotEmpty(t, genState)
	_, hasMint := genState[minttypes.ModuleName]
	require.False(t, hasMint, "default genesis must not contain a mint section")
}
