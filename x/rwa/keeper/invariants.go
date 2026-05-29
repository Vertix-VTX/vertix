package keeper

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/rwa/types"
)

// RegisterInvariants registers the rwa crisis invariants.
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "bonds", BondInvariant(k))
	ir.RegisterRoute(types.ModuleName, "denoms", DenomInvariant(k))
}

// BondInvariant: the module account uvtx balance equals the sum of bonds over
// all non-settled assets, and every ACTIVE asset has a positive bond.
func BondInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		sum := math.ZeroInt()
		broken := false
		_ = k.IterateAssets(ctx, func(rec types.AssetRecord) bool {
			if rec.Status == types.AssetStatus_ASSET_STATUS_SETTLED {
				return true
			}
			bond, _ := math.NewIntFromString(rec.Bond)
			if rec.Status == types.AssetStatus_ASSET_STATUS_ACTIVE && !bond.IsPositive() {
				broken = true
			}
			sum = sum.Add(bond)
			return true
		})
		moduleBal := k.bankKeeper.GetBalance(ctx, k.ModuleAddress(), types.BondDenom).Amount
		if moduleBal.LT(sum) {
			broken = true
		}
		return sdk.FormatInvariant(types.ModuleName, "bonds",
			"module uvtx balance must be >= sum of non-settled bonds and every ACTIVE asset must be bonded"), broken
	}
}

// DenomInvariant: a pre-mint asset (DRAFT/ATTESTED) must have zero rwa/{id} supply.
func DenomInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		broken := false
		_ = k.IterateAssets(ctx, func(rec types.AssetRecord) bool {
			if rec.Status == types.AssetStatus_ASSET_STATUS_DRAFT || rec.Status == types.AssetStatus_ASSET_STATUS_ATTESTED {
				if !k.bankKeeper.GetSupply(ctx, rec.Denom).Amount.IsZero() {
					broken = true
				}
			}
			return true
		})
		return sdk.FormatInvariant(types.ModuleName, "denoms",
			"pre-mint assets must have zero factory-denom supply"), broken
	}
}
