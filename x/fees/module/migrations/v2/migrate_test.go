package v2_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	v2 "github.com/vertix-network/vertix/x/fees/module/migrations/v2"
)

func TestMigrate1to2_NoOp(t *testing.T) {
	err := v2.Migrate1to2(sdk.Context{})
	require.NoError(t, err)
}
