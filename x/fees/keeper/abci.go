package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/vertix-network/vertix/x/fees/types"
)

// EndBlocker burns BurnRatio of the fee collector's uvtx and leaves the
// remainder for the native x/distribution BeginBlock to pay stakers (spec D1).
// Never panics: any bank error is logged and the block proceeds.
func (k Keeper) EndBlocker(ctx context.Context) error {
	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}
	burnRatio, err := params.BurnRatioDec()
	if err != nil {
		return err
	}
	if burnRatio.IsZero() {
		return nil // native flow distributes 100%; nothing to burn
	}

	feeCollector := k.accountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
	balance := k.bankKeeper.GetBalance(ctx, feeCollector, types.FeeDenom)
	if balance.Amount.IsZero() {
		return nil
	}

	burnAmt := math.LegacyNewDecFromInt(balance.Amount).Mul(burnRatio).TruncateInt()
	if burnAmt.IsZero() {
		return nil // dust: nothing to burn this block
	}
	burnCoins := sdk.NewCoins(sdk.NewCoin(types.FeeDenom, burnAmt))

	if err := k.bankKeeper.SendCoinsFromModuleToModule(
		ctx, authtypes.FeeCollectorName, types.ModuleName, burnCoins,
	); err != nil {
		k.Logger().Error("fees: failed to move burn coins from fee collector", "err", err)
		return nil
	}
	if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, burnCoins); err != nil {
		k.Logger().Error("fees: failed to burn coins", "err", err)
		return nil
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	distributed := balance.Amount.Sub(burnAmt)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeFeeBurned,
		sdk.NewAttribute(types.AttributeKeyAmount, burnCoins.String()),
	))
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeFeeDistributed,
		sdk.NewAttribute(types.AttributeKeyAmount, sdk.NewCoin(types.FeeDenom, distributed).String()),
	))
	return nil
}
