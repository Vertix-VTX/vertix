package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestModuleConstants(t *testing.T) {
	require.Equal(t, "rwa", types.ModuleName)
	require.Equal(t, "rwa", types.StoreKey)
	require.Equal(t, "uvtx", types.BondDenom)
	require.Equal(t, "rwa/", types.RWADenomPrefix)
	require.Equal(t, []byte{0x01}, types.KeyPrefixAsset)
	require.Equal(t, []byte{0x05}, types.KeyParams)
}

func TestAssetKeyRoundTrip(t *testing.T) {
	key := types.AssetKey("gold-01")
	require.Equal(t, byte(0x01), key[0])
	require.Equal(t, "gold-01", string(key[1:]))
}
