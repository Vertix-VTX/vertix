package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	keepertest "github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestGetParams(t *testing.T) {
	k, ctx := keepertest.RwaKeeper(t)
	params := types.DefaultParams()

	require.NoError(t, k.SetParams(ctx, params))
	got, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.EqualValues(t, params, got)
}
