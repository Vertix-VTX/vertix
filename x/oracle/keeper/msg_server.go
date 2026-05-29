package keeper

import (
	"context"
	"strconv"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/vertix-network/vertix/x/oracle/types"
)

type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

func (ms msgServer) SetFeeder(ctx context.Context, msg *types.MsgSetFeeder) (*types.MsgSetFeederResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	val, _ := sdk.ValAddressFromBech32(msg.Validator)
	feeder, _ := sdk.AccAddressFromBech32(msg.Feeder)
	if ms.stakingKeeper != nil {
		v, err := ms.stakingKeeper.GetValidator(ctx, val)
		if err != nil {
			return nil, err
		}
		if v.Status != stakingtypes.Bonded {
			return nil, types.ErrNotBondedValidator
		}
	}
	if err := ms.SetFeederDelegation(ctx, val, feeder); err != nil {
		return nil, err
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeFeederSet,
		sdk.NewAttribute(types.AttributeKeyValidator, msg.Validator),
		sdk.NewAttribute(types.AttributeKeyFeeder, msg.Feeder),
	))
	return &types.MsgSetFeederResponse{}, nil
}

func (ms msgServer) SubmitFeed(ctx context.Context, msg *types.MsgSubmitFeed) (*types.MsgSubmitFeedResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	val, _ := sdk.ValAddressFromBech32(msg.Validator)
	feeder, _ := sdk.AccAddressFromBech32(msg.Feeder)
	authorized, err := ms.ResolveAuthorizedFeeder(ctx, val)
	if err != nil {
		return nil, err
	}
	if !authorized.Equals(feeder) {
		return nil, types.ErrFeederNotAuthorized
	}
	params, err := ms.GetParams(ctx)
	if err != nil {
		return nil, err
	}
	if !params.PairAccepted(msg.Pair) {
		return nil, types.ErrPairNotAccepted
	}
	if ms.stakingKeeper != nil {
		v, err := ms.stakingKeeper.GetValidator(ctx, val)
		if err != nil {
			return nil, err
		}
		if v.Status != stakingtypes.Bonded {
			return nil, types.ErrNotBondedValidator
		}
		power, err := ms.stakingKeeper.GetLastValidatorPower(ctx, val)
		if err != nil || power == 0 {
			return nil, types.ErrNotBondedValidator
		}
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	feed := types.OracleFeed{
		Validator:   msg.Validator,
		Pair:        msg.Pair,
		Price:       msg.Price,
		BlockHeight: sdkCtx.BlockHeight(),
	}
	if err := ms.SetFeed(ctx, feed); err != nil {
		return nil, err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeFeedSubmitted,
		sdk.NewAttribute(types.AttributeKeyValidator, msg.Validator),
		sdk.NewAttribute(types.AttributeKeyFeeder, msg.Feeder),
		sdk.NewAttribute(types.AttributeKeyPair, msg.Pair),
		sdk.NewAttribute(types.AttributeKeyPrice, msg.Price),
	))
	return &types.MsgSubmitFeedResponse{}, nil
}

func (ms msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if ms.GetAuthority() != msg.Authority {
		return nil, errorsmod.Wrapf(types.ErrInvalidSigner, "invalid authority; expected %s, got %s", ms.GetAuthority(), msg.Authority)
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	oldParams, err := ms.GetParams(ctx)
	if err != nil {
		return nil, err
	}
	acceptListChanged := oldParams.AcceptListChanged(msg.Params)
	if acceptListChanged {
		if err := ms.ResetMissCounters(ctx); err != nil {
			return nil, err
		}
	}
	if err := ms.SetParams(ctx, msg.Params); err != nil {
		return nil, err
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeParamsUpdated,
		sdk.NewAttribute(types.AttributeKeyAcceptListChanged, strconv.FormatBool(acceptListChanged)),
	))
	return &types.MsgUpdateParamsResponse{}, nil
}
