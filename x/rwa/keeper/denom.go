package keeper

import (
	"strings"

	"github.com/vertix-network/vertix/x/rwa/types"
)

// BuildDenom returns the factory denom for an asset id.
func BuildDenom(assetID string) string {
	return types.RWADenomPrefix + assetID
}

// ParseRWADenom returns the asset id for an rwa/* denom, or ok=false otherwise.
func ParseRWADenom(denom string) (string, bool) {
	if !strings.HasPrefix(denom, types.RWADenomPrefix) {
		return "", false
	}
	id := strings.TrimPrefix(denom, types.RWADenomPrefix)
	if id == "" {
		return "", false
	}
	return id, true
}
