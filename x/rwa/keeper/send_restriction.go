package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SendRestriction is registered via bankKeeper.AppendSendRestriction in app.go.
// It runs inside BankKeeper.SendCoins — the chokepoint for MsgSend, MsgMultiSend,
// authz-wrapped sends, AND IBC transfer escrow — so rwa/* restriction semantics
// cannot be bypassed. Non-rwa denoms are a cheap no-op.
func (k Keeper) SendRestriction(ctx context.Context, from, to sdk.AccAddress, amt sdk.Coins) (sdk.AccAddress, error) {
	for _, c := range amt {
		assetID, ok := ParseRWADenom(c.Denom)
		if !ok {
			continue
		}
		if err := k.CheckTransferAllowed(ctx, assetID, from, to); err != nil {
			return to, err
		}
	}
	return to, nil
}
