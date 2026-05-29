package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/fees/types"
)

// RegisterInvariants registers the fees crisis invariants.
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "reconcile", ReconcileInvariant(k))
	ir.RegisterRoute(types.ModuleName, "module-balance", ModuleBalanceInvariant(k))
}

// ReconcileInvariant: genesisSupply - currentSupply(uvtx) == cumulativeBurned.
// Since x/fees burning is the only uvtx sink (no x/mint; rwa/* are distinct
// denoms), this also enforces the 21M hard cap / no-inflation invariant.
func ReconcileInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		genSupply, err := k.GetGenesisSupply(ctx)
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "reconcile", "failed to load genesis supply: "+err.Error()), true
		}
		burned, err := k.GetCumulativeBurned(ctx)
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "reconcile", "failed to load cumulative burned: "+err.Error()), true
		}
		current := k.bankKeeper.GetSupply(ctx, types.FeeDenom).Amount
		broken := !genSupply.Sub(current).Equal(burned)
		return sdk.FormatInvariant(types.ModuleName, "reconcile",
			"genesis uvtx supply minus current supply must equal cumulative burned (21M cap)"), broken
	}
}

// ModuleBalanceInvariant: the x/fees module account holds zero uvtx at the
// block boundary (it holds coins only transiently inside EndBlocker).
func ModuleBalanceInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		addr := k.accountKeeper.GetModuleAddress(types.ModuleName)
		bal := k.bankKeeper.GetBalance(ctx, addr, types.FeeDenom).Amount
		broken := !bal.IsZero()
		return sdk.FormatInvariant(types.ModuleName, "module-balance",
			"x/fees module account uvtx balance must be zero at the block boundary"), broken
	}
}
