package keeper

import (
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/rwa/types"
)

func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) {
	if err := gs.Validate(); err != nil {
		panic(fmt.Errorf("invalid rwa genesis: %w", err))
	}
	if err := k.SetParams(ctx, gs.Params); err != nil {
		panic(err)
	}
	for _, a := range gs.Assets {
		if err := k.SetAsset(ctx, a); err != nil {
			panic(err)
		}
	}
	for _, r := range gs.Restrictions {
		addr, err := sdk.AccAddressFromBech32(r.Address)
		if err != nil {
			panic(err)
		}
		if r.IsDeny {
			if err := k.AddDenylist(ctx, r.AssetId, addr); err != nil {
				panic(err)
			}
		} else if err := k.AddAllowlist(ctx, r.AssetId, addr); err != nil {
			panic(err)
		}
	}
}

func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	params, err := k.GetParams(ctx)
	if err != nil {
		panic(err)
	}
	gs := &types.GenesisState{Params: params}

	_ = k.IterateAssets(ctx, func(rec types.AssetRecord) bool {
		gs.Assets = append(gs.Assets, rec)
		return true
	})
	sort.Slice(gs.Assets, func(i, j int) bool { return gs.Assets[i].AssetId < gs.Assets[j].AssetId })

	for _, a := range gs.Assets {
		assetID := a.AssetId
		_ = k.IterateMembers(ctx, types.AllowlistPrefix(assetID), func(addr sdk.AccAddress) bool {
			gs.Restrictions = append(gs.Restrictions, types.Restriction{AssetId: assetID, Address: addr.String(), IsDeny: false})
			return true
		})
		_ = k.IterateMembers(ctx, types.DenylistPrefix(assetID), func(addr sdk.AccAddress) bool {
			gs.Restrictions = append(gs.Restrictions, types.Restriction{AssetId: assetID, Address: addr.String(), IsDeny: true})
			return true
		})
	}
	return gs
}
