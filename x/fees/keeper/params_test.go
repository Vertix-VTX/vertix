package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/fees/types"
)

func TestParamsRoundTrip(t *testing.T) {
	k, ctx := keeper.FeesKeeper(t, keeper.NewMockBank())
	got, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, types.DefaultParams(), got)
}
