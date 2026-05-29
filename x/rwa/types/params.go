package types

import "cosmossdk.io/math"

// DefaultParams returns launch defaults: 10,000 VTX min bond, 0.10% mint+settle fees.
func DefaultParams() RWAParams {
	return RWAParams{
		MinIssuerBond: math.NewInt(10_000_000_000).String(),     // 10,000 VTX in uvtx
		MintFeeRate:   math.LegacyNewDecWithPrec(1, 3).String(), // 0.001
		SettleFeeRate: math.LegacyNewDecWithPrec(1, 3).String(), // 0.001
	}
}

// MinIssuerBondInt parses min_issuer_bond into an Int.
func (p RWAParams) MinIssuerBondInt() (math.Int, bool) {
	return math.NewIntFromString(p.MinIssuerBond)
}

// MintFeeRateDec parses mint_fee_rate into a LegacyDec.
func (p RWAParams) MintFeeRateDec() (math.LegacyDec, error) {
	return math.LegacyNewDecFromStr(p.MintFeeRate)
}

// SettleFeeRateDec parses settle_fee_rate into a LegacyDec.
func (p RWAParams) SettleFeeRateDec() (math.LegacyDec, error) {
	return math.LegacyNewDecFromStr(p.SettleFeeRate)
}

// Validate enforces: bond is a non-negative Int; both fee rates are in [0,1).
func (p RWAParams) Validate() error {
	bond, ok := math.NewIntFromString(p.MinIssuerBond)
	if !ok {
		return ErrInvalidParams.Wrapf("min_issuer_bond: not an integer: %q", p.MinIssuerBond)
	}
	if bond.IsNegative() {
		return ErrInvalidParams.Wrap("min_issuer_bond must be >= 0")
	}
	if err := validateFeeRate(p.MintFeeRate, "mint_fee_rate"); err != nil {
		return err
	}
	return validateFeeRate(p.SettleFeeRate, "settle_fee_rate")
}

func validateFeeRate(s, name string) error {
	d, err := math.LegacyNewDecFromStr(s)
	if err != nil {
		return ErrInvalidParams.Wrapf("%s: %v", name, err)
	}
	if d.IsNegative() || d.GTE(math.LegacyOneDec()) {
		return ErrInvalidParams.Wrapf("%s must be in [0, 1)", name)
	}
	return nil
}
