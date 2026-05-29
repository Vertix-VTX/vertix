package types

import (
	"fmt"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultGenesis returns the default genesis state.
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

// Validate performs genesis state validation.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}
	seenPairs := make(map[string]struct{}, len(gs.Prices))
	for i, p := range gs.Prices {
		if !gs.Params.PairAccepted(p.Pair) {
			return fmt.Errorf("genesis price[%d]: pair %q not in accept_list", i, p.Pair)
		}
		if _, ok := seenPairs[p.Pair]; ok {
			return fmt.Errorf("genesis price[%d]: duplicate pair %q", i, p.Pair)
		}
		seenPairs[p.Pair] = struct{}{}
		price, err := math.LegacyNewDecFromStr(p.Price)
		if err != nil {
			return fmt.Errorf("genesis price[%d]: invalid price: %w", i, err)
		}
		if !price.IsPositive() {
			return fmt.Errorf("genesis price[%d]: price must be positive", i)
		}
		if p.BlockTime.IsZero() {
			return fmt.Errorf("genesis price[%d]: block_time must be set", i)
		}
	}
	seenFeeders := make(map[string]struct{}, len(gs.Feeders))
	for i, fd := range gs.Feeders {
		if _, err := sdk.ValAddressFromBech32(fd.Validator); err != nil {
			return fmt.Errorf("genesis feeder[%d]: invalid validator: %w", i, err)
		}
		if _, err := sdk.AccAddressFromBech32(fd.Feeder); err != nil {
			return fmt.Errorf("genesis feeder[%d]: invalid feeder: %w", i, err)
		}
		if _, ok := seenFeeders[fd.Feeder]; ok {
			return fmt.Errorf("genesis feeder[%d]: duplicate feeder %q", i, fd.Feeder)
		}
		seenFeeders[fd.Feeder] = struct{}{}
	}
	return nil
}

// ValidatePriceBlockTime checks that an aggregated price block time is not after chain time.
func ValidatePriceBlockTime(blockTime, chainTime time.Time) error {
	if blockTime.After(chainTime) {
		return fmt.Errorf("aggregated price block_time %v is in the future (chain time %v)", blockTime, chainTime)
	}
	return nil
}
