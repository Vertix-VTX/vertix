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
}

func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	params, err := k.GetParams(ctx)
	if err != nil {
		panic(err)
	}
	return &types.GenesisState{Params: params}
}
