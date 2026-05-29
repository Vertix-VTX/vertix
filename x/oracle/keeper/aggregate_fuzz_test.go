package keeper

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

// FuzzMedianWithinRange asserts UnweightedMedian never panics and stays within input bounds.
func FuzzMedianWithinRange(f *testing.F) {
	f.Add(int64(1), int64(2), int64(3))
	f.Fuzz(func(t *testing.T, a, b, c int64) {
		prices := []math.LegacyDec{
			math.LegacyNewDec(a),
			math.LegacyNewDec(b),
			math.LegacyNewDec(c),
		}
		lo, hi := prices[0], prices[0]
		for _, p := range prices[1:] {
			if p.LT(lo) {
				lo = p
			}
			if p.GT(hi) {
				hi = p
			}
		}
		got := UnweightedMedian(prices)
		require.False(t, got.LT(lo) || got.GT(hi), "median %s outside [%s, %s]", got, lo, hi)
	})
}
