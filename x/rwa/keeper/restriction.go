package keeper

import (
	"context"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/rwa/types"
)

func (k Keeper) AddAllowlist(ctx context.Context, assetID string, addr sdk.AccAddress) error {
	return k.storeService.OpenKVStore(ctx).Set(types.AllowlistKey(assetID, addr), []byte{1})
}

func (k Keeper) DelAllowlist(ctx context.Context, assetID string, addr sdk.AccAddress) error {
	return k.storeService.OpenKVStore(ctx).Delete(types.AllowlistKey(assetID, addr))
}

func (k Keeper) AddDenylist(ctx context.Context, assetID string, addr sdk.AccAddress) error {
	return k.storeService.OpenKVStore(ctx).Set(types.DenylistKey(assetID, addr), []byte{1})
}

func (k Keeper) DelDenylist(ctx context.Context, assetID string, addr sdk.AccAddress) error {
	return k.storeService.OpenKVStore(ctx).Delete(types.DenylistKey(assetID, addr))
}

func (k Keeper) hasKey(ctx context.Context, key []byte) bool {
	ok, err := k.storeService.OpenKVStore(ctx).Has(key)
	return err == nil && ok
}

func (k Keeper) IsAllowlisted(ctx context.Context, assetID string, addr sdk.AccAddress) bool {
	return k.hasKey(ctx, types.AllowlistKey(assetID, addr))
}

func (k Keeper) IsDenylisted(ctx context.Context, assetID string, addr sdk.AccAddress) bool {
	return k.hasKey(ctx, types.DenylistKey(assetID, addr))
}

// CheckTransferAllowed enforces an asset's restriction policy for a transfer.
// Module-account legs (mint payout, burn collection, bond escrow) are exempt.
func (k Keeper) CheckTransferAllowed(ctx context.Context, assetID string, from, to sdk.AccAddress) error {
	moduleAddr := k.ModuleAddress()
	if from.Equals(moduleAddr) || to.Equals(moduleAddr) {
		return nil
	}
	rec, found := k.GetAsset(ctx, assetID)
	if !found {
		return types.ErrAssetNotFound.Wrap(assetID)
	}
	if !rec.AllowAll {
		if !k.IsAllowlisted(ctx, assetID, from) || !k.IsAllowlisted(ctx, assetID, to) {
			return types.ErrTransferRestricted.Wrapf("asset %s requires both parties allowlisted", assetID)
		}
	}
	if k.IsDenylisted(ctx, assetID, from) || k.IsDenylisted(ctx, assetID, to) {
		return types.ErrTransferRestricted.Wrapf("asset %s: party is denylisted", assetID)
	}
	return nil
}

// IterateMembers walks one membership list (allow or deny) for genesis export/query.
func (k Keeper) IterateMembers(ctx context.Context, prefix []byte, fn func(addr sdk.AccAddress) bool) error {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return err
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		// key layout: prefix(1) | lenByte(1) | assetID | addrBytes
		if len(key) < 2 {
			continue
		}
		idLen := int(key[1])
		if len(key) < 2+idLen {
			continue
		}
		addr := sdk.AccAddress(key[2+idLen:])
		if !fn(addr) {
			break
		}
	}
	return nil
}
