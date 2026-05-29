package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
)

func TestResolveAuthorizedFeederImplicitOperator(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	val := sdk.ValAddress(sdk.MustAccAddressFromBech32(sample.AccAddress()).Bytes())
	feeder, err := k.ResolveAuthorizedFeeder(ctx, val)
	require.NoError(t, err)
	require.Equal(t, sdk.AccAddress(val.Bytes()).String(), feeder.String())
}
