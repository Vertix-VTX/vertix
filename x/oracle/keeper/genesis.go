package keeper

import (
	"fmt"
	"sort"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/oracle/types"
)

func (k Keeper) InitGenesis(ctx sdk.Context, genState types.GenesisState) {
	if err := genState.Validate(); err != nil {
		panic(fmt.Errorf("invalid genesis state: %w", err))
	}
	for i, p := range genState.Prices {
		if err := types.ValidatePriceBlockTime(p.BlockTime, ctx.BlockTime()); err != nil {
			panic(fmt.Errorf("genesis price[%d]: %w", i, err))
		}
	}
	if err := k.SetParams(ctx, genState.Params); err != nil {
		panic(err)
	}
	for _, p := range genState.Prices {
		if err := k.SetAggregatedPrice(ctx, p); err != nil {
			panic(err)
		}
	}
	for _, fd := range genState.Feeders {
		val, err := sdk.ValAddressFromBech32(fd.Validator)
		if err != nil {
			panic(err)
		}
		feeder, err := sdk.AccAddressFromBech32(fd.Feeder)
		if err != nil {
			panic(err)
		}
		if k.stakingKeeper != nil {
			if _, err := k.stakingKeeper.GetValidator(ctx, val); err != nil {
				panic(fmt.Errorf("genesis feeder for %s: validator not found: %w", fd.Validator, err))
			}
		}
		if err := k.SetFeederDelegation(ctx, val, feeder); err != nil {
			panic(err)
		}
	}
}

func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	params, err := k.GetParams(ctx)
	if err != nil {
		panic(err)
	}
	genesis := &types.GenesisState{Params: params}

	store := k.storeService.OpenKVStore(ctx)
	pricePrefix := types.KeyPrefixAggregatedPrice
	iter, err := store.Iterator(pricePrefix, storetypes.PrefixEndBytes(pricePrefix))
	if err != nil {
		panic(err)
	}
	for ; iter.Valid(); iter.Next() {
		var ap types.AggregatedPrice
		k.cdc.MustUnmarshal(iter.Value(), &ap)
		genesis.Prices = append(genesis.Prices, ap)
	}
	iter.Close()
	sort.Slice(genesis.Prices, func(i, j int) bool {
		return genesis.Prices[i].Pair < genesis.Prices[j].Pair
	})

	feederPrefix := types.KeyPrefixValoperToFeeder
	iter, err = store.Iterator(feederPrefix, storetypes.PrefixEndBytes(feederPrefix))
	if err != nil {
		panic(err)
	}
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) <= len(feederPrefix) {
			continue
		}
		val := sdk.ValAddress(key[len(feederPrefix):])
		feeder := sdk.AccAddress(iter.Value())
		genesis.Feeders = append(genesis.Feeders, types.FeederDelegation{
			Validator: val.String(),
			Feeder:    feeder.String(),
		})
	}
	iter.Close()
	sort.Slice(genesis.Feeders, func(i, j int) bool {
		return genesis.Feeders[i].Validator < genesis.Feeders[j].Validator
	})

	return genesis
}
