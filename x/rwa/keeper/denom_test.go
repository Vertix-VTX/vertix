package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	rwakeeper "github.com/vertix-network/vertix/x/rwa/keeper"
)

func TestBuildAndParseDenom(t *testing.T) {
	require.Equal(t, "rwa/gold-01", rwakeeper.BuildDenom("gold-01"))

	id, ok := rwakeeper.ParseRWADenom("rwa/gold-01")
	require.True(t, ok)
	require.Equal(t, "gold-01", id)

	_, ok = rwakeeper.ParseRWADenom("uvtx")
	require.False(t, ok)

	_, ok = rwakeeper.ParseRWADenom("ibc/ABC")
	require.False(t, ok)
}
