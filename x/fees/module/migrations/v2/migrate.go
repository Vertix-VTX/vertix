package v2

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Migrate1to2 is a no-op migration registered for the v0.2.0-testnet upgrade rehearsal.
// It proves RunMigrations executes without mutating live fee state.
func Migrate1to2(_ sdk.Context) error {
	return nil
}
