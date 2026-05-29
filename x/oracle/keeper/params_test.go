package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestParamsRoundTrip(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	got, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, types.DefaultParams(), got)
}
