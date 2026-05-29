package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AccountKeeper resolves the standard fee collector module address.
type AccountKeeper interface {
	GetModuleAddress(name string) sdk.AccAddress
}

// BankKeeper is the minimal surface x/fees needs: read the fee-collector
// balance, move the burn portion into the fees module account, and burn it.
// x/fees deliberately does NOT distribute — the native x/distribution
// BeginBlock distributes the remainder left in the fee collector (spec D1).
type BankKeeper interface {
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	SendCoinsFromModuleToModule(ctx context.Context, senderModule, recipientModule string, amt sdk.Coins) error
	BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
}
