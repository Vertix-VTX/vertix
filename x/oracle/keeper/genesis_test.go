package keeper_test

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestGenesisRoundTrip(t *testing.T) {
	valAcc := sdk.ValAddress(sdk.MustAccAddressFromBech32(sample.AccAddress()).Bytes())
	valStr := valAcc.String()
	feeder := sample.AccAddress()
	mockStaking := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{
			{OperatorAddress: valStr, Status: stakingtypes.Bonded},
		},
	}

	ctx, k := keeper.OracleKeeper(t, mockStaking, &keeper.MockSlashing{})
	now := time.Unix(2000, 0).UTC()
	ctx = ctx.WithBlockTime(now)
	require.NoError(t, k.SetAggregatedPrice(ctx, types.AggregatedPrice{
		Pair:        "BTC:USD",
		Price:       "50000",
		BlockHeight: 1,
		BlockTime:   now,
	}))
	require.NoError(t, k.SetFeederDelegation(ctx, valAcc, sdk.MustAccAddressFromBech32(feeder)))

	exported := k.ExportGenesis(ctx)

	ctx2, k2 := keeper.OracleKeeper(t, mockStaking, &keeper.MockSlashing{})
	ctx2 = ctx2.WithBlockTime(now)
	k2.InitGenesis(ctx2, *exported)
	require.Equal(t, exported, k2.ExportGenesis(ctx2))
}
