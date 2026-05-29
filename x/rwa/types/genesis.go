package types

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func DefaultGenesis() *GenesisState {
	return &GenesisState{Params: DefaultParams()}
}

func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}
	minBond, _ := gs.Params.MinIssuerBondInt()
	seen := make(map[string]struct{}, len(gs.Assets))
	for i, a := range gs.Assets {
		if err := ValidateAssetID(a.AssetId); err != nil {
			return fmt.Errorf("genesis asset[%d]: %w", i, err)
		}
		if _, ok := seen[a.AssetId]; ok {
			return fmt.Errorf("genesis asset[%d]: duplicate asset_id %q", i, a.AssetId)
		}
		seen[a.AssetId] = struct{}{}
		if _, err := sdk.AccAddressFromBech32(a.Issuer); err != nil {
			return fmt.Errorf("genesis asset[%d]: invalid issuer: %w", i, err)
		}
		if a.Status < AssetStatus_ASSET_STATUS_DRAFT || a.Status > AssetStatus_ASSET_STATUS_SETTLED {
			return fmt.Errorf("genesis asset[%d]: invalid status %s", i, a.Status)
		}
		if !pairRegex.MatchString(a.OraclePair) {
			return fmt.Errorf("genesis asset[%d]: oracle_pair %q is not BASE:QUOTE", i, a.OraclePair)
		}
		bond, ok := math.NewIntFromString(a.Bond)
		if !ok {
			return fmt.Errorf("genesis asset[%d]: invalid bond %q", i, a.Bond)
		}
		if a.Status == AssetStatus_ASSET_STATUS_ATTESTED || a.Status == AssetStatus_ASSET_STATUS_ACTIVE {
			if bond.LT(minBond) {
				return fmt.Errorf("genesis asset[%d]: bond %s < min_issuer_bond %s", i, bond, minBond)
			}
		}
		if _, ok := math.NewIntFromString(a.NotionalMinted); !ok {
			return fmt.Errorf("genesis asset[%d]: invalid notional_minted %q", i, a.NotionalMinted)
		}
	}
	for i, r := range gs.Restrictions {
		if _, ok := seen[r.AssetId]; !ok {
			return fmt.Errorf("genesis restriction[%d]: unknown asset_id %q", i, r.AssetId)
		}
		if _, err := sdk.AccAddressFromBech32(r.Address); err != nil {
			return fmt.Errorf("genesis restriction[%d]: invalid address: %w", i, err)
		}
	}
	return nil
}
