package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
	oraclekeeper "github.com/vertix-network/vertix/x/oracle/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestUpdateParamsResetsCountersOnAcceptListChange(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	val := sdk.ValAddress(sdk.MustAccAddressFromBech32(sample.AccAddress()).Bytes())
	_, err := k.IncMissCounter(ctx, val)
	require.NoError(t, err)
	_, err = k.IncTotalWindows(ctx, val)
	require.NoError(t, err)

	server := oraclekeeper.NewMsgServerImpl(k)
	newParams := types.DefaultParams()
	newParams.AcceptList = append(newParams.AcceptList, "SOL:USD")
	_, err = server.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: k.GetAuthority(),
		Params:    newParams,
	})
	require.NoError(t, err)

	misses, err := k.GetMissCounter(ctx, val)
	require.NoError(t, err)
	require.Equal(t, int64(0), misses)
}
