package keeper

import (
	"context"
	"time"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/oracle/types"
)

func (k Keeper) AppendTWAPEntry(ctx context.Context, pair string, price math.LegacyDec, blockTime time.Time) error {
	store := k.storeService.OpenKVStore(ctx)
	entry := types.TWAPEntry{Pair: pair, Price: price.String(), BlockTime: blockTime}
	bz, err := k.cdc.Marshal(&entry)
	if err != nil {
		return err
	}
	return store.Set(types.TWAPKey(pair, blockTime), bz)
}

func (k Keeper) PruneTWAPOlderThan(ctx context.Context, pair string, cutoff time.Time) error {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.TWAPPrefix(pair)
	iter, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return err
	}
	var toDelete [][]byte
	for ; iter.Valid(); iter.Next() {
		var e types.TWAPEntry
		k.cdc.MustUnmarshal(iter.Value(), &e)
		if e.BlockTime.Before(cutoff) {
			toDelete = append(toDelete, append([]byte{}, iter.Key()...))
		} else {
			break
		}
	}
	iter.Close()
	for _, key := range toDelete {
		if err := store.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

func (k Keeper) GetTWAP(ctx context.Context, pair string, window time.Duration) (math.LegacyDec, error) {
	params, err := k.GetParams(ctx)
	if err != nil {
		return math.LegacyZeroDec(), err
	}
	if !params.PairAccepted(pair) {
		return math.LegacyZeroDec(), types.ErrPairNotAccepted
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime()
	windowStart := now.Add(-window)

	store := k.storeService.OpenKVStore(ctx)
	prefix := types.TWAPPrefix(pair)
	iter, err := store.ReverseIterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return math.LegacyZeroDec(), err
	}
	defer iter.Close()

	var entries []types.TWAPEntry
	for ; iter.Valid(); iter.Next() {
		var e types.TWAPEntry
		k.cdc.MustUnmarshal(iter.Value(), &e)
		if e.BlockTime.After(now) {
			continue
		}
		entries = append(entries, e)
		if !e.BlockTime.After(windowStart) {
			break // include last entry at or before windowStart (leading clamp)
		}
	}
	if len(entries) == 0 {
		return math.LegacyZeroDec(), types.ErrNoTWAPData
	}
	// reverse to ascending
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	if now.Sub(entries[len(entries)-1].BlockTime) > time.Duration(params.MaxPriceAge)*time.Second {
		return math.LegacyZeroDec(), types.ErrStalePrice
	}
	if len(entries) == 1 {
		return math.LegacyNewDecFromStr(entries[0].Price)
	}
	weightedSum := math.LegacyZeroDec()
	totalWeight := math.LegacyZeroDec()
	for i := 0; i < len(entries); i++ {
		price, err := math.LegacyNewDecFromStr(entries[i].Price)
		if err != nil {
			return math.LegacyZeroDec(), err
		}
		start := entries[i].BlockTime
		var end time.Time
		if i+1 < len(entries) {
			end = entries[i+1].BlockTime
		} else {
			end = now
		}
		w := math.LegacyNewDec(end.Sub(start).Nanoseconds())
		weightedSum = weightedSum.Add(price.Mul(w))
		totalWeight = totalWeight.Add(w)
	}
	if totalWeight.IsZero() {
		return math.LegacyZeroDec(), types.ErrNoTWAPData
	}
	return weightedSum.Quo(totalWeight), nil
}
