package keeper

import (
	"context"
	"fmt"

	corestore "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/vertix-network/vertix/x/rwa/types"
)

type Keeper struct {
	cdc          codec.BinaryCodec
	storeService corestore.KVStoreService
	logger       log.Logger
	authority    string

	accountKeeper types.AccountKeeper
	bankKeeper    types.BankKeeper
	oracleKeeper  types.OracleKeeper
	distrKeeper   types.DistributionKeeper
}

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService corestore.KVStoreService,
	logger log.Logger,
	authority string,
	ak types.AccountKeeper,
	bk types.BankKeeper,
	ok types.OracleKeeper,
	dk types.DistributionKeeper,
) Keeper {
	return Keeper{
		cdc:           cdc,
		storeService:  storeService,
		logger:        logger,
		authority:     authority,
		accountKeeper: ak,
		bankKeeper:    bk,
		oracleKeeper:  ok,
		distrKeeper:   dk,
	}
}

func (k Keeper) GetAuthority() string { return k.authority }

func (k Keeper) Logger() log.Logger {
	return k.logger.With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// ModuleAddress is the rwa module account (holds bonds; mints/burns factory denoms).
func (k Keeper) ModuleAddress() sdk.AccAddress {
	return authtypes.NewModuleAddress(types.ModuleName)
}

// --- Params ---

func (k Keeper) GetParams(ctx context.Context) (types.RWAParams, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyParams)
	if err != nil {
		return types.RWAParams{}, err
	}
	if bz == nil {
		return types.DefaultParams(), nil
	}
	var p types.RWAParams
	k.cdc.MustUnmarshal(bz, &p)
	return p, nil
}

func (k Keeper) SetParams(ctx context.Context, p types.RWAParams) error {
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

// --- Asset CRUD ---

func (k Keeper) SetAsset(ctx context.Context, rec types.AssetRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := k.cdc.Marshal(&rec)
	if err != nil {
		return err
	}
	if err := store.Set(types.AssetKey(rec.AssetId), bz); err != nil {
		return err
	}
	issuer, err := sdk.AccAddressFromBech32(rec.Issuer)
	if err != nil {
		return err
	}
	return store.Set(types.IssuerIndexKey(issuer, rec.AssetId), []byte{1})
}

func (k Keeper) GetAsset(ctx context.Context, assetID string) (types.AssetRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.AssetKey(assetID))
	if err != nil || bz == nil {
		return types.AssetRecord{}, false
	}
	var rec types.AssetRecord
	k.cdc.MustUnmarshal(bz, &rec)
	return rec, true
}

// IterateAssets walks every asset record in 0x01 order.
func (k Keeper) IterateAssets(ctx context.Context, fn func(types.AssetRecord) bool) error {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.KeyPrefixAsset, storetypes.PrefixEndBytes(types.KeyPrefixAsset))
	if err != nil {
		return err
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var rec types.AssetRecord
		k.cdc.MustUnmarshal(iter.Value(), &rec)
		if !fn(rec) {
			break
		}
	}
	return nil
}
