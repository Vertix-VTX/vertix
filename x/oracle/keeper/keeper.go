package keeper

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	corestore "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/oracle/types"
)

type Keeper struct {
	cdc            codec.BinaryCodec
	storeService   corestore.KVStoreService
	stakingKeeper  types.StakingKeeper
	slashingKeeper types.SlashingKeeper
	authority      string
}

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService corestore.KVStoreService,
	sk types.StakingKeeper,
	slk types.SlashingKeeper,
	authority string,
) Keeper {
	return Keeper{cdc, storeService, sk, slk, authority}
}

func (k Keeper) GetAuthority() string { return k.authority }

func (k Keeper) SetAggregatedPrice(ctx context.Context, ap types.AggregatedPrice) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := k.cdc.Marshal(&ap)
	if err != nil {
		return err
	}
	return store.Set(types.AggregatedPriceKey(ap.Pair), bz)
}

func (k Keeper) GetAggregatedPrice(ctx context.Context, pair string) (types.AggregatedPrice, error) {
	params, err := k.GetParams(ctx)
	if err != nil {
		return types.AggregatedPrice{}, err
	}
	if !params.PairAccepted(pair) {
		return types.AggregatedPrice{}, types.ErrPairNotAccepted
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.AggregatedPriceKey(pair))
	if err != nil {
		return types.AggregatedPrice{}, err
	}
	if bz == nil {
		return types.AggregatedPrice{}, types.ErrNoPrice
	}
	var ap types.AggregatedPrice
	k.cdc.MustUnmarshal(bz, &ap)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	age := sdkCtx.BlockTime().Sub(ap.BlockTime)
	if age > time.Duration(params.MaxPriceAge)*time.Second {
		return types.AggregatedPrice{}, types.ErrStalePrice
	}
	return ap, nil
}

func (k Keeper) GetPrice(ctx context.Context, pair string) (math.LegacyDec, error) {
	ap, err := k.GetAggregatedPrice(ctx, pair)
	if err != nil {
		return math.LegacyZeroDec(), err
	}
	return math.LegacyNewDecFromStr(ap.Price)
}

func (k Keeper) Logger(ctx context.Context) log.Logger {
	return sdk.UnwrapSDKContext(ctx).Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

func (k Keeper) GetParams(ctx context.Context) (types.OracleParams, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyParams)
	if err != nil {
		return types.OracleParams{}, err
	}
	if bz == nil {
		return types.DefaultParams(), nil
	}
	var p types.OracleParams
	k.cdc.MustUnmarshal(bz, &p)
	return p, nil
}

func (k Keeper) SetParams(ctx context.Context, p types.OracleParams) error {
	if err := p.Validate(); err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := k.cdc.Marshal(&p)
	if err != nil {
		return err
	}
	return store.Set(types.KeyParams, bz)
}

func (k Keeper) SetFeed(ctx context.Context, feed types.OracleFeed) error {
	val, err := sdk.ValAddressFromBech32(feed.Validator)
	if err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := k.cdc.Marshal(&feed)
	if err != nil {
		return err
	}
	return store.Set(types.FeedKey(val, feed.Pair), bz)
}

func (k Keeper) GetFeed(ctx context.Context, val sdk.ValAddress, pair string) (types.OracleFeed, bool, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.FeedKey(val, pair))
	if err != nil {
		return types.OracleFeed{}, false, err
	}
	if bz == nil {
		return types.OracleFeed{}, false, nil
	}
	var feed types.OracleFeed
	k.cdc.MustUnmarshal(bz, &feed)
	return feed, true, nil
}

func (k Keeper) IncMissCounter(ctx context.Context, val sdk.ValAddress) (int64, error) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.MissCounterKey(val)
	bz, err := store.Get(key)
	if err != nil {
		return 0, err
	}
	var count int64
	if bz != nil {
		count = int64(binary.BigEndian.Uint64(bz))
	}
	count++
	newBz := make([]byte, 8)
	binary.BigEndian.PutUint64(newBz, uint64(count))
	if err := store.Set(key, newBz); err != nil {
		return 0, err
	}
	return count, nil
}

func (k Keeper) IncTotalWindows(ctx context.Context, val sdk.ValAddress) (int64, error) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.TotalWindowsKey(val)
	bz, err := store.Get(key)
	if err != nil {
		return 0, err
	}
	var count int64
	if bz != nil {
		count = int64(binary.BigEndian.Uint64(bz))
	}
	count++
	newBz := make([]byte, 8)
	binary.BigEndian.PutUint64(newBz, uint64(count))
	if err := store.Set(key, newBz); err != nil {
		return 0, err
	}
	return count, nil
}

func (k Keeper) GetMissCounter(ctx context.Context, val sdk.ValAddress) (int64, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.MissCounterKey(val))
	if err != nil {
		return 0, err
	}
	if bz == nil {
		return 0, nil
	}
	return int64(binary.BigEndian.Uint64(bz)), nil
}

func (k Keeper) GetTotalWindows(ctx context.Context, val sdk.ValAddress) (int64, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.TotalWindowsKey(val))
	if err != nil {
		return 0, err
	}
	if bz == nil {
		return 0, nil
	}
	return int64(binary.BigEndian.Uint64(bz)), nil
}

func (k Keeper) ResetMissCounters(ctx context.Context) error {
	store := k.storeService.OpenKVStore(ctx)
	if err := deleteAllWithPrefix(store, types.KeyPrefixMissCounter); err != nil {
		return err
	}
	return deleteAllWithPrefix(store, types.KeyPrefixTotalWindows)
}

func (k Keeper) ResetValidatorWindows(ctx context.Context, val sdk.ValAddress) error {
	store := k.storeService.OpenKVStore(ctx)
	if err := store.Delete(types.MissCounterKey(val)); err != nil {
		return err
	}
	return store.Delete(types.TotalWindowsKey(val))
}

// IterateAllFeeds scans prefix 0x01; fn receives (valoper, pair, priceStr).
func (k Keeper) IterateAllFeeds(ctx context.Context, fn func(val sdk.ValAddress, pair, priceStr string) bool) error {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.FeedKeyPrefix()
	iter, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return err
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		val, pair, err := types.ParseFeedKey(iter.Key())
		if err != nil {
			continue
		}
		var feed types.OracleFeed
		k.cdc.MustUnmarshal(iter.Value(), &feed)
		if !fn(val, pair, feed.Price) {
			break
		}
	}
	return nil
}

func (k Keeper) DeleteAllFeeds(ctx context.Context) error {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.FeedKeyPrefix()
	iter, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return err
	}
	var keys [][]byte
	for ; iter.Valid(); iter.Next() {
		keys = append(keys, append([]byte{}, iter.Key()...))
	}
	iter.Close()
	for _, key := range keys {
		if err := store.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

// GetValidatorSubmittedPairs returns pairs the validator submitted in the current window.
func (k Keeper) GetValidatorSubmittedPairs(ctx context.Context, val sdk.ValAddress) (map[string]struct{}, error) {
	pairs := make(map[string]struct{})
	err := k.IterateAllFeeds(ctx, func(v sdk.ValAddress, pair, _ string) bool {
		if v.Equals(val) {
			pairs[pair] = struct{}{}
		}
		return true
	})
	return pairs, err
}

func deleteAllWithPrefix(store corestore.KVStore, prefix []byte) error {
	iter, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return err
	}
	var keys [][]byte
	for ; iter.Valid(); iter.Next() {
		keys = append(keys, append([]byte{}, iter.Key()...))
	}
	iter.Close()
	for _, key := range keys {
		if err := store.Delete(key); err != nil {
			return err
		}
	}
	return nil
}
