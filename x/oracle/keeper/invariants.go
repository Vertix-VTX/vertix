package keeper

import (
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/oracle/types"
)

// RegisterInvariants registers the oracle crisis invariants.
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "prices", PricesInvariant(k))
}

// PricesInvariant checks every stored AggregatedPrice is accept-listed and strictly positive.
// Use storetypes.PrefixEndBytes for iteration end key.
func PricesInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		broken := false
		params, err := k.GetParams(ctx)
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "prices", "failed to load params: "+err.Error()), true
		}
		accept := make(map[string]struct{}, len(params.AcceptList))
		for _, p := range params.AcceptList {
			accept[p] = struct{}{}
		}

		store := k.storeService.OpenKVStore(ctx)
		iter, err := store.Iterator(types.KeyPrefixAggregatedPrice, storetypes.PrefixEndBytes(types.KeyPrefixAggregatedPrice))
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "prices", "failed to iterate prices: "+err.Error()), true
		}
		defer iter.Close()
		for ; iter.Valid(); iter.Next() {
			var ap types.AggregatedPrice
			k.cdc.MustUnmarshal(iter.Value(), &ap)
			if _, ok := accept[ap.Pair]; !ok {
				broken = true
			}
			price, perr := math.LegacyNewDecFromStr(ap.Price)
			if perr != nil || !price.IsPositive() {
				broken = true
			}
		}
		return sdk.FormatInvariant(types.ModuleName, "prices",
			"every stored aggregated price must be accept-listed and strictly positive"), broken
	}
}
