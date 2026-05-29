package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/vertix-network/vertix/x/rwa/types"
)

// ComputeFee returns floor(notional * rate) in uvtx units.
func (k Keeper) ComputeFee(notional math.Int, rate math.LegacyDec) math.Int {
	fee := math.LegacyNewDecFromInt(notional).Mul(rate).TruncateInt()
	if fee.IsZero() {
		return math.ZeroInt()
	}
	return fee
}

// CollectFee debits `fee` uvtx from the payer straight into the fee collector
// (single hop — the fee never transits the rwa module account). x/fees sweeps
// it at the next EndBlock. A zero fee is a no-op.
func (k Keeper) CollectFee(ctx context.Context, payer sdk.AccAddress, fee math.Int) error {
	if !fee.IsPositive() {
		return nil
	}
	coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, fee))
	return k.bankKeeper.SendCoinsFromAccountToModule(ctx, payer, authtypes.FeeCollectorName, coins)
}
