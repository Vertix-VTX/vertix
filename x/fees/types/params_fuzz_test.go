package types_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/fees/types"
)

// FuzzBurnRatioSplit asserts that for any valid burn ratio, burn + distribution
// reconstructs the whole and Validate accepts it.
func FuzzBurnRatioSplit(f *testing.F) {
	f.Add(uint64(40))
	f.Fuzz(func(t *testing.T, pct uint64) {
		p := pct % 101 // 0..100
		params := types.FeesParams{
			BurnRatio:         math.LegacyNewDecWithPrec(int64(p), 2).String(),
			DistributionRatio: math.LegacyNewDecWithPrec(int64(100-p), 2).String(),
		}
		require.NoError(t, params.Validate())
		burn, err := params.BurnRatioDec()
		require.NoError(t, err)
		distr, err := params.DistributionRatioDec()
		require.NoError(t, err)
		require.True(t, burn.Add(distr).Equal(math.LegacyOneDec()))
	})
}
