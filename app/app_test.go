package app_test

import (
	"testing"

	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/stretchr/testify/require"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/vertix-network/vertix/app"
	feestypes "github.com/vertix-network/vertix/x/fees/types"
	oracletypes "github.com/vertix-network/vertix/x/oracle/types"
	rwatypes "github.com/vertix-network/vertix/x/rwa/types"
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

func TestOracleModuleWired(t *testing.T) {
	a := newTestApp(t)
	_, ok := a.ModuleManager.Modules[oracletypes.ModuleName]
	require.True(t, ok, "x/oracle must be wired")
	require.NotNil(t, a.OracleKeeper)
}

func TestOracleGenesisRoundTrip(t *testing.T) {
	a := newTestApp(t)
	genState := a.DefaultGenesis()
	require.Contains(t, genState, oracletypes.ModuleName)
}

func TestFeesModuleWired(t *testing.T) {
	a := newTestApp(t)
	_, ok := a.ModuleManager.Modules[feestypes.ModuleName]
	require.True(t, ok, "x/fees must be wired")
	require.NotNil(t, a.FeesKeeper)
}

func TestFeesModuleAccountHasBurner(t *testing.T) {
	newTestApp(t)
	perms, ok := app.GetMaccPerms()[feestypes.ModuleName]
	require.True(t, ok, "x/fees module account must be configured")
	require.Contains(t, perms, authtypes.Burner)
	require.NotContains(t, perms, authtypes.Minter)
	require.NotContains(t, perms, authtypes.Staking)
}

func TestRWAModuleWired(t *testing.T) {
	a := newTestApp(t)
	_, ok := a.ModuleManager.Modules[rwatypes.ModuleName]
	require.True(t, ok, "x/rwa must be wired")
	require.NotNil(t, a.RwaKeeper)
}

func TestRWAModuleAccountHasMinterBurner(t *testing.T) {
	newTestApp(t)
	perms, ok := app.GetMaccPerms()[rwatypes.ModuleName]
	require.True(t, ok, "x/rwa module account must be configured")
	require.Contains(t, perms, authtypes.Minter)
	require.Contains(t, perms, authtypes.Burner)
}

func TestRWAGenesisRoundTrip(t *testing.T) {
	a := newTestApp(t)
	genState := a.DefaultGenesis()
	require.Contains(t, genState, rwatypes.ModuleName)
}
