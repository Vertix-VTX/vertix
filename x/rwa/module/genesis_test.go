package rwa_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	keepertest "github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/nullify"
	rwa "github.com/vertix-network/vertix/x/rwa/module"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),

		// this line is used by starport scaffolding # genesis/test/state
	}

	k, ctx := keepertest.RwaKeeper(t)
	rwa.InitGenesis(ctx, k, genesisState)
	got := rwa.ExportGenesis(ctx, k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)

	// this line is used by starport scaffolding # genesis/test/assert
}
