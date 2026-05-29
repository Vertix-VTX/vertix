package keeper

import (
	"sort"

	"cosmossdk.io/math"
)

// WeightedPrice is a validator price with stake weight for median aggregation.
type WeightedPrice struct {
	Price   math.LegacyDec
	Weight  int64
	Valoper string
}

// WeightedMedian returns the stake-weighted median price (published oracle price).
func WeightedMedian(entries []WeightedPrice) math.LegacyDec {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Price.Equal(entries[j].Price) {
			return entries[i].Valoper < entries[j].Valoper
		}
		return entries[i].Price.LT(entries[j].Price)
	})
	var total int64
	for _, e := range entries {
		total += e.Weight
	}
	half := total / 2
	var cum int64
	for _, e := range entries {
		cum += e.Weight
		if cum >= half {
			return e.Price
		}
	}
	return math.LegacyZeroDec()
}

// UnweightedMedian returns the lower-middle price (outlier reference band).
func UnweightedMedian(prices []math.LegacyDec) math.LegacyDec {
	sort.Slice(prices, func(i, j int) bool { return prices[i].LT(prices[j]) })
	n := len(prices)
	if n == 0 {
		return math.LegacyZeroDec()
	}
	return prices[(n-1)/2]
}

// QuorumMet reports whether submittedPower / totalPower meets quorumFraction.
func QuorumMet(submittedPower, totalPower math.Int, quorumFraction math.LegacyDec) bool {
	if totalPower.IsZero() {
		return false
	}
	ratio := math.LegacyNewDec(submittedPower.Int64()).Quo(math.LegacyNewDec(totalPower.Int64()))
	return !ratio.LT(quorumFraction)
}
