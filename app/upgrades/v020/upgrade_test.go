package v020

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpgradeName(t *testing.T) {
	require.Equal(t, "v0.2.0-testnet", UpgradeName)
}
