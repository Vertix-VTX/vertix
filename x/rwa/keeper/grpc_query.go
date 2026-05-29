package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/vertix-network/vertix/x/rwa/types"
)

var _ types.QueryServer = Keeper{}

func (k Keeper) Asset(goCtx context.Context, req *types.QueryAssetRequest) (*types.QueryAssetResponse, error) {
	if req == nil || req.AssetId == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	rec, found := k.GetAsset(goCtx, req.AssetId)
	if !found {
		return nil, status.Error(codes.NotFound, types.ErrAssetNotFound.Error())
	}
	return &types.QueryAssetResponse{Asset: rec}, nil
}

func (k Keeper) AssetsByIssuer(goCtx context.Context, req *types.QueryAssetsByIssuerRequest) (*types.QueryAssetsByIssuerResponse, error) {
	if req == nil || req.Issuer == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	issuer, err := sdk.AccAddressFromBech32(req.Issuer)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	adapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(goCtx))
	idxStore := prefix.NewStore(adapter, types.IssuerIndexPrefix(issuer))

	var assets []types.AssetRecord
	pageRes, err := query.Paginate(idxStore, req.Pagination, func(key, _ []byte) error {
		assetID := string(key)
		if len(assetID) > 0 && assetID[0] == '/' {
			assetID = assetID[1:]
		}
		if rec, found := k.GetAsset(goCtx, assetID); found {
			assets = append(assets, rec)
		}
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &types.QueryAssetsByIssuerResponse{Assets: assets, Pagination: pageRes}, nil
}

func (k Keeper) Restrictions(goCtx context.Context, req *types.QueryRestrictionsRequest) (*types.QueryRestrictionsResponse, error) {
	if req == nil || req.AssetId == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	rec, found := k.GetAsset(goCtx, req.AssetId)
	if !found {
		return nil, status.Error(codes.NotFound, types.ErrAssetNotFound.Error())
	}
	resp := &types.QueryRestrictionsResponse{AllowAll: rec.AllowAll}
	_ = k.IterateMembers(goCtx, types.AllowlistPrefix(req.AssetId), func(addr sdk.AccAddress) bool {
		resp.Allowlist = append(resp.Allowlist, addr.String())
		return true
	})
	_ = k.IterateMembers(goCtx, types.DenylistPrefix(req.AssetId), func(addr sdk.AccAddress) bool {
		resp.Denylist = append(resp.Denylist, addr.String())
		return true
	})
	return resp, nil
}

func (k Keeper) Params(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	params, err := k.GetParams(goCtx)
	if err != nil {
		return nil, err
	}
	return &types.QueryParamsResponse{Params: params}, nil
}
