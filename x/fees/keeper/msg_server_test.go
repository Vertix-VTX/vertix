package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	feeskeeper "github.com/vertix-network/vertix/x/fees/keeper"
	"github.com/vertix-network/vertix/x/fees/types"
)

func TestUpdateParamsAuthorized(t *testing.T) {
	k, ctx := keeper.FeesKeeper(t, keeper.NewMockBank())
	server := feeskeeper.NewMsgServerImpl(k)

	newParams := types.FeesParams{BurnRatio: "0.50", DistributionRatio: "0.50"}
	_, err := server.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: k.GetAuthority(),
		Params:    newParams,
	})
	require.NoError(t, err)

	got, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, newParams, got)
}

func TestUpdateParamsRejectsWrongAuthority(t *testing.T) {
	k, ctx := keeper.FeesKeeper(t, keeper.NewMockBank())
	server := feeskeeper.NewMsgServerImpl(k)

	_, err := server.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "vtx1wrongauthority000000000000000000000000",
		Params:    types.DefaultParams(),
	})
	require.Error(t, err)
}

func TestUpdateParamsRejectsBadRatioSum(t *testing.T) {
	k, ctx := keeper.FeesKeeper(t, keeper.NewMockBank())
	server := feeskeeper.NewMsgServerImpl(k)

	_, err := server.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: k.GetAuthority(),
		Params:    types.FeesParams{BurnRatio: "0.40", DistributionRatio: "0.50"},
	})
	require.ErrorIs(t, err, types.ErrInvalidRatioSum)
}
