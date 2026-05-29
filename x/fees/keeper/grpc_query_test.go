package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/fees/types"
)

func TestParamsQuery(t *testing.T) {
	k, ctx := keeper.FeesKeeper(t, keeper.NewMockBank())
	params := types.DefaultParams()
	require.NoError(t, k.SetParams(ctx, params))

	resp, err := k.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, params, resp.Params)
}
