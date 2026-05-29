package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/oracle/types"
)

func (k Keeper) SetFeederDelegation(ctx context.Context, val sdk.ValAddress, feeder sdk.AccAddress) error {
	store := k.storeService.OpenKVStore(ctx)
	existing, err := store.Get(types.FeederToValoperKey(feeder))
	if err != nil {
		return err
	}
	if existing != nil && !sdk.ValAddress(existing).Equals(val) {
		return types.ErrFeederAlreadyBound
	}
	if err := store.Set(types.FeederToValoperKey(feeder), val.Bytes()); err != nil {
		return err
	}
	return store.Set(types.ValoperToFeederKey(val), feeder.Bytes())
}

func (k Keeper) GetFeederForValidator(ctx context.Context, val sdk.ValAddress) (sdk.AccAddress, bool, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ValoperToFeederKey(val))
	if err != nil {
		return nil, false, err
	}
	if bz == nil {
		return nil, false, nil
	}
	return sdk.AccAddress(bz), true, nil
}

func (k Keeper) ResolveAuthorizedFeeder(ctx context.Context, val sdk.ValAddress) (sdk.AccAddress, error) {
	if feeder, ok, err := k.GetFeederForValidator(ctx, val); err != nil {
		return nil, err
	} else if ok {
		return feeder, nil
	}
	return sdk.AccAddress(val.Bytes()), nil
}
