package app_test

import (
	"testing"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/require"

	feestypes "github.com/vertix-network/vertix/x/fees/types"
)

// The fees reconcile invariant snapshots the uvtx supply at InitGenesis, so
// bank's InitGenesis must run before fees'.
func TestBankInitsBeforeFees(t *testing.T) {
	a := newTestApp(t)
	order := a.ModuleManager.OrderInitGenesis

	bankIdx, feesIdx := -1, -1
	for i, name := range order {
		switch name {
		case banktypes.ModuleName:
			bankIdx = i
		case feestypes.ModuleName:
			feesIdx = i
		}
	}
	require.NotEqual(t, -1, bankIdx, "bank not in OrderInitGenesis")
	require.NotEqual(t, -1, feesIdx, "fees not in OrderInitGenesis")
	require.Less(t, bankIdx, feesIdx, "bank must init before fees")
}
