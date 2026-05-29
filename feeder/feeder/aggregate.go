package feeder

import (
	"errors"
	"sort"

	"cosmossdk.io/math"
)

// ErrInsufficientSources is returned when fewer than min healthy sources remain.
var ErrInsufficientSources = errors.New("insufficient healthy price sources")

// Median returns the median of prices (average of the two middle values for
// an even count). Returns an error for an empty slice.
func Median(prices []math.LegacyDec) (math.LegacyDec, error) {
	n := len(prices)
	if n == 0 {
		return math.LegacyDec{}, errors.New("median of empty set")
	}
	sorted := make([]math.LegacyDec, n)
	copy(sorted, prices)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].LT(sorted[j]) })
	if n%2 == 1 {
		return sorted[n/2], nil
	}
	lo, hi := sorted[n/2-1], sorted[n/2]
	return lo.Add(hi).QuoInt64(2), nil
}

// DropDeviating removes prices that deviate more than maxDeviation (fractional,
// e.g. 0.10 = 10%) from the provisional median of the input.
func DropDeviating(prices []math.LegacyDec, maxDeviation math.LegacyDec) []math.LegacyDec {
	if len(prices) == 0 {
		return prices
	}
	mid, err := Median(prices)
	if err != nil || !mid.IsPositive() {
		return prices
	}
	band := mid.Mul(maxDeviation)
	out := make([]math.LegacyDec, 0, len(prices))
	for _, p := range prices {
		diff := p.Sub(mid).Abs()
		if diff.LTE(band) {
			out = append(out, p)
		}
	}
	return out
}

// CrossSourceMedian applies the data-quality policy (design D3): require
// minProviders sources, drop deviating sources, then require minProviders
// survivors before returning their median.
func CrossSourceMedian(prices []math.LegacyDec, maxDeviation math.LegacyDec, minProviders int) (math.LegacyDec, error) {
	if len(prices) < minProviders {
		return math.LegacyDec{}, ErrInsufficientSources
	}
	filtered := DropDeviating(prices, maxDeviation)
	if len(filtered) < minProviders {
		return math.LegacyDec{}, ErrInsufficientSources
	}
	return Median(filtered)
}
