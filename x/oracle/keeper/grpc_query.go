package keeper

import (
	"context"
	"errors"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/vertix-network/vertix/x/oracle/types"
)

var _ types.QueryServer = Keeper{}

func oracleQueryGRPCError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, types.ErrNoPrice), errors.Is(err, types.ErrNoTWAPData):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, types.ErrStalePrice):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return err
	}
}

func (k Keeper) Price(goCtx context.Context, req *types.QueryPriceRequest) (*types.QueryPriceResponse, error) {
	if req == nil || req.Pair == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	ap, err := k.GetAggregatedPrice(ctx, req.Pair)
	if err != nil {
		return nil, oracleQueryGRPCError(err)
	}
	return &types.QueryPriceResponse{Price: ap}, nil
}

func (k Keeper) Twap(goCtx context.Context, req *types.QueryTwapRequest) (*types.QueryTwapResponse, error) {
	if req == nil || req.Pair == "" || req.WindowSeconds == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	price, err := k.GetTWAP(ctx, req.Pair, time.Duration(req.WindowSeconds)*time.Second)
	if err != nil {
		return nil, oracleQueryGRPCError(err)
	}
	return &types.QueryTwapResponse{Price: price}, nil
}

func (k Keeper) Params(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, err
	}
	return &types.QueryParamsResponse{Params: params}, nil
}

func (k Keeper) MissCounter(goCtx context.Context, req *types.QueryMissCounterRequest) (*types.QueryMissCounterResponse, error) {
	if req == nil || req.Validator == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	val, err := sdk.ValAddressFromBech32(req.Validator)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	misses, err := k.GetMissCounter(ctx, val)
	if err != nil {
		return nil, err
	}
	total, err := k.GetTotalWindows(ctx, val)
	if err != nil {
		return nil, err
	}
	return &types.QueryMissCounterResponse{Misses: misses, TotalWindows: total}, nil
}

func (k Keeper) Feeder(goCtx context.Context, req *types.QueryFeederRequest) (*types.QueryFeederResponse, error) {
	if req == nil || req.Validator == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	val, err := sdk.ValAddressFromBech32(req.Validator)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	feeder, err := k.ResolveAuthorizedFeeder(ctx, val)
	if err != nil {
		return nil, err
	}
	return &types.QueryFeederResponse{Feeder: feeder.String()}, nil
}
