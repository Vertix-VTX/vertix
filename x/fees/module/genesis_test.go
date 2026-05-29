package fees_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	keepertest "github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/nullify"
	fees "github.com/vertix-network/vertix/x/fees/module"
	"github.com/vertix-network/vertix/x/fees/types"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),

		// this line is used by starport scaffolding # genesis/test/state
	}

	k, ctx := keepertest.FeesKeeper(t, keepertest.NewMockBank())
	fees.InitGenesis(ctx, k, genesisState)
	got := fees.ExportGenesis(ctx, k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)

	// this line is used by starport scaffolding # genesis/test/assert
}
