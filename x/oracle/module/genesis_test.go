package oracle_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	keepertest "github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/nullify"
	oracle "github.com/vertix-network/vertix/x/oracle/module"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),

		// this line is used by starport scaffolding # genesis/test/state
	}

	ctx, k := keepertest.OracleKeeper(t, &keepertest.MockStaking{}, &keepertest.MockSlashing{})
	oracle.InitGenesis(ctx, k, genesisState)
	got := oracle.ExportGenesis(ctx, k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)

	// this line is used by starport scaffolding # genesis/test/assert
}
