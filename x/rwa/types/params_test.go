package types_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestDefaultParamsValid(t *testing.T) {
	p := types.DefaultParams()
	require.NoError(t, p.Validate())
	require.Equal(t, "10000000000", p.MinIssuerBond)
	require.Equal(t, "0.001000000000000000", p.MintFeeRate)
	require.Equal(t, "0.001000000000000000", p.SettleFeeRate)
}

func TestParamsAccessors(t *testing.T) {
	p := types.DefaultParams()
	bond, ok := p.MinIssuerBondInt()
	require.True(t, ok)
	require.Equal(t, math.NewInt(10000000000), bond)

	rate, err := p.MintFeeRateDec()
	require.NoError(t, err)
	require.True(t, rate.Equal(math.LegacyNewDecWithPrec(1, 3)))
}

func TestValidateRejectsBadFeeRate(t *testing.T) {
	require.Error(t, types.RWAParams{MinIssuerBond: "1", MintFeeRate: "1.5", SettleFeeRate: "0.001"}.Validate())
	require.Error(t, types.RWAParams{MinIssuerBond: "1", MintFeeRate: "abc", SettleFeeRate: "0.001"}.Validate())
}

func TestValidateRejectsNegativeBond(t *testing.T) {
	require.Error(t, types.RWAParams{MinIssuerBond: "-1", MintFeeRate: "0.001", SettleFeeRate: "0.001"}.Validate())
	require.Error(t, types.RWAParams{MinIssuerBond: "notanint", MintFeeRate: "0.001", SettleFeeRate: "0.001"}.Validate())
}
