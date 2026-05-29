package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/fees/types"
)

type msgServer struct {
	k Keeper
}

func NewMsgServerImpl(k Keeper) types.MsgServer {
	return &msgServer{k: k}
}

var _ types.MsgServer = msgServer{}

func (m msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	if msg.Authority != m.k.GetAuthority() {
		return nil, types.ErrUnauthorized.Wrapf("expected %s, got %s", m.k.GetAuthority(), msg.Authority)
	}
	if err := m.k.SetParams(ctx, msg.Params); err != nil {
		return nil, err
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeParamsUpdated,
		sdk.NewAttribute(types.AttributeKeyBurnRatio, msg.Params.BurnRatio),
		sdk.NewAttribute(types.AttributeKeyDistributionRatio, msg.Params.DistributionRatio),
	))
	return &types.MsgUpdateParamsResponse{}, nil
}
