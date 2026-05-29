package types

import "cosmossdk.io/math"

// DefaultParams returns the launch fee split: 40% burn, 60% to stakers
// (the latter realized by the native x/distribution BeginBlock — spec D1/D7).
func DefaultParams() FeesParams {
	return FeesParams{
		BurnRatio:         math.LegacyNewDecWithPrec(40, 2).String(), // "0.40"
		DistributionRatio: math.LegacyNewDecWithPrec(60, 2).String(), // "0.60"
	}
}

// BurnRatioDec parses burn_ratio into a LegacyDec.
func (p FeesParams) BurnRatioDec() (math.LegacyDec, error) {
	return math.LegacyNewDecFromStr(p.BurnRatio)
}

// DistributionRatioDec parses distribution_ratio into a LegacyDec.
func (p FeesParams) DistributionRatioDec() (math.LegacyDec, error) {
	return math.LegacyNewDecFromStr(p.DistributionRatio)
}

// Validate enforces both ratios are in [0,1] and sum to exactly 1.
func (p FeesParams) Validate() error {
	burn, err := math.LegacyNewDecFromStr(p.BurnRatio)
	if err != nil {
		return ErrInvalidParams.Wrapf("burn_ratio: %v", err)
	}
	distr, err := math.LegacyNewDecFromStr(p.DistributionRatio)
	if err != nil {
		return ErrInvalidParams.Wrapf("distribution_ratio: %v", err)
	}
	if burn.IsNegative() || burn.GT(math.LegacyOneDec()) {
		return ErrInvalidParams.Wrap("burn_ratio must be in [0, 1]")
	}
	if distr.IsNegative() || distr.GT(math.LegacyOneDec()) {
		return ErrInvalidParams.Wrap("distribution_ratio must be in [0, 1]")
	}
	if !burn.Add(distr).Equal(math.LegacyOneDec()) {
		return ErrInvalidRatioSum
	}
	return nil
}
