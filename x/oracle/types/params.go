package types

import (
	"regexp"

	"cosmossdk.io/math"
)

var pairRegex = regexp.MustCompile(`^[A-Z0-9]+:[A-Z0-9]+$`)

func DefaultParams() OracleParams {
	return OracleParams{
		VoteWindow:       10,
		MissThreshold:    "0.05",
		MissSlashRate:    "0.005",
		OutlierSlashRate: "0.01",
		OutlierThreshold: "0.05",
		MissWindowSize:   500,
		QuorumFraction:   "0.667",
		MaxPriceAge:      300,
		AcceptList:       []string{"VTX:USD", "BTC:USD", "ETH:USD", "ATOM:USD", "USDC:USD"},
	}
}

func (p OracleParams) Validate() error {
	if p.VoteWindow <= 0 {
		return ErrInvalidParams.Wrap("vote_window must be > 0")
	}
	if p.MissWindowSize <= 0 {
		return ErrInvalidParams.Wrap("miss_window_size must be > 0")
	}
	if p.MaxPriceAge <= 0 {
		return ErrInvalidParams.Wrap("max_price_age must be > 0")
	}
	if err := validateDecOpenClosed(p.MissThreshold, "miss_threshold"); err != nil {
		return err
	}
	if err := validateDecOpenClosed(p.OutlierThreshold, "outlier_threshold"); err != nil {
		return err
	}
	if err := validateDecOpenClosed(p.QuorumFraction, "quorum_fraction"); err != nil {
		return err
	}
	if err := validateSlashRate(p.MissSlashRate, "miss_slash_rate"); err != nil {
		return err
	}
	if err := validateSlashRate(p.OutlierSlashRate, "outlier_slash_rate"); err != nil {
		return err
	}
	if len(p.AcceptList) == 0 {
		return ErrInvalidParams.Wrap("accept_list must be non-empty")
	}
	seen := make(map[string]struct{}, len(p.AcceptList))
	for _, pair := range p.AcceptList {
		if !pairRegex.MatchString(pair) {
			return ErrInvalidPairFormat.Wrapf("pair %q", pair)
		}
		if _, ok := seen[pair]; ok {
			return ErrInvalidParams.Wrapf("duplicate pair %q", pair)
		}
		seen[pair] = struct{}{}
	}
	return nil
}

func validateDecOpenClosed(s, name string) error {
	d, err := math.LegacyNewDecFromStr(s)
	if err != nil {
		return ErrInvalidParams.Wrapf("%s: %v", name, err)
	}
	if d.LTE(math.LegacyZeroDec()) || d.GT(math.LegacyOneDec()) {
		return ErrInvalidParams.Wrapf("%s must be in (0, 1]", name)
	}
	return nil
}

func validateSlashRate(s, name string) error {
	d, err := math.LegacyNewDecFromStr(s)
	if err != nil {
		return ErrInvalidParams.Wrapf("%s: %v", name, err)
	}
	ceiling := math.LegacyNewDecWithPrec(5, 2) // 0.05 double-sign ceiling
	if d.IsNegative() || d.GTE(ceiling) {
		return ErrInvalidParams.Wrapf("%s must be in [0, 0.05)", name)
	}
	return nil
}

func (p OracleParams) AcceptListChanged(other OracleParams) bool {
	if len(p.AcceptList) != len(other.AcceptList) {
		return true
	}
	for i := range p.AcceptList {
		if p.AcceptList[i] != other.AcceptList[i] {
			return true
		}
	}
	return false
}

func (p OracleParams) PairAccepted(pair string) bool {
	for _, ap := range p.AcceptList {
		if ap == pair {
			return true
		}
	}
	return false
}
