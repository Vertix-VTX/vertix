package types_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/fees/types"
)

func TestDefaultParamsValid(t *testing.T) {
	p := types.DefaultParams()
	require.NoError(t, p.Validate())
	require.Equal(t, "0.400000000000000000", p.BurnRatio)
	require.Equal(t, "0.600000000000000000", p.DistributionRatio)
}

func TestValidateRejectsRatiosNotSummingToOne(t *testing.T) {
	p := types.FeesParams{BurnRatio: "0.40", DistributionRatio: "0.50"}
	require.ErrorIs(t, p.Validate(), types.ErrInvalidRatioSum)
}

func TestValidateRejectsRatioAboveOne(t *testing.T) {
	p := types.FeesParams{BurnRatio: "1.50", DistributionRatio: "-0.50"}
	require.Error(t, p.Validate())
}

func TestValidateRejectsUnparseableRatio(t *testing.T) {
	p := types.FeesParams{BurnRatio: "abc", DistributionRatio: "0.60"}
	require.Error(t, p.Validate())
}

func TestBurnRatioDec(t *testing.T) {
	p := types.DefaultParams()
	d, err := p.BurnRatioDec()
	require.NoError(t, err)
	require.True(t, d.Equal(math.LegacyNewDecWithPrec(40, 2)))
}
