package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/fees/types"
)

func (k Keeper) InitGenesis(ctx sdk.Context, genState types.GenesisState) {
	if err := genState.Validate(); err != nil {
		panic(fmt.Errorf("invalid genesis state: %w", err))
	}
	if err := k.SetParams(ctx, genState.Params); err != nil {
		panic(err)
	}
	// Snapshot the uvtx baseline for the reconcile invariant. Requires bank
	// InitGenesis to have run first (asserted by app/genesis_order_test.go).
	supply := k.bankKeeper.GetSupply(ctx, types.FeeDenom).Amount
	if err := k.SetGenesisSupply(ctx, supply); err != nil {
		panic(err)
	}
}

func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	params, err := k.GetParams(ctx)
	if err != nil {
		panic(err)
	}
	return &types.GenesisState{Params: params}
}
