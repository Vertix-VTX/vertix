package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestDefaultParamsValid(t *testing.T) {
	require.NoError(t, types.DefaultParams().Validate())
}

func TestValidateRejectsEmptyAcceptList(t *testing.T) {
	p := types.DefaultParams()
	p.AcceptList = nil
	require.Error(t, p.Validate())
}

func TestValidateRejectsDuplicatePair(t *testing.T) {
	p := types.DefaultParams()
	p.AcceptList = []string{"BTC:USD", "BTC:USD"}
	require.Error(t, p.Validate())
}

func TestValidateRejectsSlashRateAboveDoubleSign(t *testing.T) {
	p := types.DefaultParams()
	p.MissSlashRate = "0.05"
	require.Error(t, p.Validate())
}
