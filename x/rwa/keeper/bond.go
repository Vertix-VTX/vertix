package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/rwa/types"
)

// LockBond escrows `amount` uvtx from the issuer into the module account.
func (k Keeper) LockBond(ctx context.Context, issuer sdk.AccAddress, amount math.Int) error {
	coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, amount))
	return k.bankKeeper.SendCoinsFromAccountToModule(ctx, issuer, types.ModuleName, coins)
}

// ReleaseBond returns `amount` uvtx from the module account to the issuer.
func (k Keeper) ReleaseBond(ctx context.Context, issuer sdk.AccAddress, amount math.Int) error {
	coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, amount))
	return k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, issuer, coins)
}

// SlashBondToCommunityPool moves `amount` uvtx from the module account into the
// x/distribution community pool (governance dispute outcome).
func (k Keeper) SlashBondToCommunityPool(ctx context.Context, amount math.Int) error {
	coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, amount))
	return k.distrKeeper.FundCommunityPool(ctx, coins, k.ModuleAddress())
}
