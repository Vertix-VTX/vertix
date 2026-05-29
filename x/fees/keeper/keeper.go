package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/vertix-network/vertix/x/fees/types"
)

type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService
	logger       log.Logger

	authority string

	accountKeeper types.AccountKeeper
	bankKeeper    types.BankKeeper
}

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	logger log.Logger,
	authority string,
	accountKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
) Keeper {
	return Keeper{
		cdc:           cdc,
		storeService:  storeService,
		logger:        logger,
		authority:     authority,
		accountKeeper: accountKeeper,
		bankKeeper:    bankKeeper,
	}
}

func (k Keeper) GetAuthority() string { return k.authority }

func (k Keeper) Logger() log.Logger {
	return k.logger.With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

func (k Keeper) GetParams(ctx context.Context) (types.FeesParams, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyParams)
	if err != nil {
		return types.FeesParams{}, err
	}
	if bz == nil {
		return types.DefaultParams(), nil
	}
	var p types.FeesParams
	k.cdc.MustUnmarshal(bz, &p)
	return p, nil
}

func (k Keeper) SetParams(ctx context.Context, p types.FeesParams) error {
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
