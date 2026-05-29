package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/fees/types"
)

func TestModuleConstants(t *testing.T) {
	require.Equal(t, "fees", types.ModuleName)
	require.Equal(t, "fees", types.StoreKey)
	require.Equal(t, "uvtx", types.FeeDenom)
	require.Equal(t, []byte{0x01}, types.KeyParams)
}
