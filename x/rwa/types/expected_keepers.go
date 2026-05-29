package types

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BankKeeper is the bank surface x/rwa needs: escrow bonds, mint/burn factory
// denoms, route fees, and restriction-checked transfers.
type BankKeeper interface {
	MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, sender sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipient sdk.AccAddress, amt sdk.Coins) error
	SendCoins(ctx context.Context, from, to sdk.AccAddress, amt sdk.Coins) error
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	GetSupply(ctx context.Context, denom string) sdk.Coin
}

// OracleKeeper is the Phase 1 price consumer interface (attestation gate).
// Both ErrNoPrice and ErrStalePrice are returned as errors.
type OracleKeeper interface {
	GetPrice(ctx context.Context, pair string) (math.LegacyDec, error)
}

// AccountKeeper resolves module account addresses.
type AccountKeeper interface {
	GetModuleAddress(name string) sdk.AccAddress
}

// DistributionKeeper receives slashed bonds into the community pool.
type DistributionKeeper interface {
	FundCommunityPool(ctx context.Context, amount sdk.Coins, sender sdk.AccAddress) error
}
