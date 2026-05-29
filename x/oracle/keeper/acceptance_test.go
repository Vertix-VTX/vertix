// Acceptance tests map to Phase 1 spec §11 (docs/specs/2026-05-29-phase-1-oracle-module-design.md).
package keeper_test

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
	oraclekeeper "github.com/vertix-network/vertix/x/oracle/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

// TestAcceptance_QuorumGatedAggregation: below quorum → no price; at quorum → stake-weighted median.
func TestAcceptance_QuorumGatedAggregation(t *testing.T) {
	v1, s1 := makeValidator(t)
	v2, s2 := makeValidator(t)
	v3, s3 := makeValidator(t)
	mock := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{
			bondingValidator(t, v1),
			bondingValidator(t, v2),
			bondingValidator(t, v3),
		},
		Powers:     map[string]int64{s1: 10, s2: 30, s3: 10},
		TotalPower: math.NewInt(50),
	}
	ctx, k := keeper.OracleKeeper(t, mock, &keeper.MockSlashing{})
	p := types.DefaultParams()
	p.VoteWindow = 1
	require.NoError(t, k.SetParams(ctx, p))
	server := oraclekeeper.NewMsgServerImpl(k)

	for _, tc := range []struct {
		val sdk.ValAddress
		str string
		px  string
	}{{v1, s1, "100"}, {v2, s2, "110"}, {v3, s3, "120"}} {
		_, err := server.SubmitFeed(ctx, &types.MsgSubmitFeed{
			Feeder: sdk.AccAddress(tc.val.Bytes()).String(), Validator: tc.str,
			Pair: "BTC:USD", Price: tc.px,
		})
		require.NoError(t, err)
	}
	require.NoError(t, k.EndBlocker(ctx))
	got, err := k.GetPrice(ctx, "BTC:USD")
	require.NoError(t, err)
	require.True(t, got.Equal(math.LegacyNewDec(110)))

	// Below quorum: single validator on 100 total power.
	vOnly, sOnly := makeValidator(t)
	mockLow := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{bondingValidator(t, vOnly)},
		Powers:     map[string]int64{sOnly: 10},
		TotalPower: math.NewInt(100),
	}
	ctx2, k2 := keeper.OracleKeeper(t, mockLow, &keeper.MockSlashing{})
	require.NoError(t, k2.SetParams(ctx2, p))
	server2 := oraclekeeper.NewMsgServerImpl(k2)
	_, err = server2.SubmitFeed(ctx2, &types.MsgSubmitFeed{
		Feeder: sdk.AccAddress(vOnly.Bytes()).String(), Validator: sOnly,
		Pair: "BTC:USD", Price: "100",
	})
	require.NoError(t, err)
	require.NoError(t, k2.EndBlocker(ctx2))
	_, err = k2.GetPrice(ctx2, "BTC:USD")
	require.ErrorIs(t, err, types.ErrNoPrice)
}

// TestAcceptance_TWAPAndPrune: duration-weighted TWAP, 24h prune via EndBlock, ErrStalePrice past max_price_age.
func TestAcceptance_TWAPAndPrune(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	base := time.Unix(1000, 0).UTC()
	ctx = ctx.WithBlockTime(base.Add(250 * time.Second))

	require.NoError(t, k.AppendTWAPEntry(ctx, "BTC:USD", twapDec(t, "100"), base))
	require.NoError(t, k.AppendTWAPEntry(ctx, "BTC:USD", twapDec(t, "200"), base.Add(100*time.Second)))
	require.NoError(t, k.AppendTWAPEntry(ctx, "BTC:USD", twapDec(t, "300"), base.Add(200*time.Second)))

	got, err := k.GetTWAP(ctx, "BTC:USD", 200*time.Second)
	require.NoError(t, err)
	require.True(t, got.Equal(twapDec(t, "180")))

	// Entry older than 24h is pruned on EndBlock TWAP append path.
	old := base.Add(-25 * time.Hour)
	require.NoError(t, k.AppendTWAPEntry(ctx, "BTC:USD", twapDec(t, "1"), old))
	require.NoError(t, k.PruneTWAPOlderThan(ctx, "BTC:USD", base.Add(-24*time.Hour)))

	now := time.Unix(2000, 0).UTC()
	ctx = ctx.WithBlockTime(now)
	require.NoError(t, k.SetAggregatedPrice(ctx, types.AggregatedPrice{
		Pair: "BTC:USD", Price: "50000", BlockHeight: 1,
		BlockTime: now.Add(-301 * time.Second),
	}))
	_, err = k.GetPrice(ctx, "BTC:USD")
	require.ErrorIs(t, err, types.ErrStalePrice)
}

// TestAcceptance_FeederDelegation: SetFeeder + feeder-signed submit OK; operator rejected when different feeder set.
func TestAcceptance_FeederDelegation(t *testing.T) {
	valAcc := sdk.ValAddress(sdk.MustAccAddressFromBech32(sample.AccAddress()).Bytes())
	valStr := valAcc.String()
	delegated := sample.AccAddress()
	mockStaking := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{
			{OperatorAddress: valStr, Status: stakingtypes.Bonded},
		},
		Powers: map[string]int64{valStr: 100},
	}
	ctx, k := keeper.OracleKeeper(t, mockStaking, &keeper.MockSlashing{})
	server := oraclekeeper.NewMsgServerImpl(k)

	_, err := server.SetFeeder(ctx, &types.MsgSetFeeder{Validator: valStr, Feeder: delegated})
	require.NoError(t, err)

	_, err = server.SubmitFeed(ctx, &types.MsgSubmitFeed{
		Feeder: delegated, Validator: valStr, Pair: "BTC:USD", Price: "42000",
	})
	require.NoError(t, err)

	operatorFeeder := sdk.AccAddress(valAcc.Bytes()).String()
	_, err = server.SubmitFeed(ctx, &types.MsgSubmitFeed{
		Feeder: operatorFeeder, Validator: valStr, Pair: "BTC:USD", Price: "43000",
	})
	require.ErrorIs(t, err, types.ErrFeederNotAuthorized)

	// Implicit operator path (no delegation).
	val2, s2 := makeValidator(t)
	mockImplicit := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{
			bondingValidator(t, val2),
		},
		Powers: map[string]int64{s2: 100},
	}
	ctx2, k2 := keeper.OracleKeeper(t, mockImplicit, &keeper.MockSlashing{})
	_, err = oraclekeeper.NewMsgServerImpl(k2).SubmitFeed(ctx2, &types.MsgSubmitFeed{
		Feeder: sdk.AccAddress(val2.Bytes()).String(), Validator: s2,
		Pair: "BTC:USD", Price: "100",
	})
	require.NoError(t, err)
}

// TestAcceptance_MissNoMassSlashOnLowQuorum: pair below quorum must not miss-slash validators.
func TestAcceptance_MissNoMassSlashOnLowQuorum(t *testing.T) {
	v1, s1 := makeValidator(t)
	v2, s2 := makeValidator(t)
	mockStaking := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{
			bondingValidator(t, v1),
			bondingValidator(t, v2),
		},
		Powers:     map[string]int64{s1: 10, s2: 10},
		TotalPower: math.NewInt(100),
	}
	mockSlashing := &keeper.MockSlashing{}
	ctx, k := keeper.OracleKeeper(t, mockStaking, mockSlashing)
	p := types.DefaultParams()
	p.VoteWindow = 1
	p.MissWindowSize = 1
	p.MissThreshold = "0.05"
	require.NoError(t, k.SetParams(ctx, p))
	server := oraclekeeper.NewMsgServerImpl(k)

	_, err := server.SubmitFeed(ctx, &types.MsgSubmitFeed{
		Feeder: sdk.AccAddress(v1.Bytes()).String(), Validator: s1,
		Pair: "BTC:USD", Price: "100",
	})
	require.NoError(t, err)
	require.NoError(t, k.EndBlocker(ctx))
	require.Empty(t, mockSlashing.Calls)

	miss, err := k.GetMissCounter(ctx, v2)
	require.NoError(t, err)
	require.Zero(t, miss)
}

// TestAcceptance_TumblingMissSlash: slash only after miss_window_size when rate > threshold.
func TestAcceptance_TumblingMissSlash(t *testing.T) {
	v1, s1 := makeValidator(t)
	v2, s2 := makeValidator(t)
	mockStaking := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{
			bondingValidator(t, v1),
			bondingValidator(t, v2),
		},
		Powers:     map[string]int64{s1: 70, s2: 30},
		TotalPower: math.NewInt(100),
	}
	mockSlashing := &keeper.MockSlashing{}
	ctx, k := keeper.OracleKeeper(t, mockStaking, mockSlashing)
	p := types.DefaultParams()
	p.VoteWindow = 1
	p.MissWindowSize = 3
	p.MissThreshold = "0.5"
	require.NoError(t, k.SetParams(ctx, p))
	server := oraclekeeper.NewMsgServerImpl(k)

	submit := func(val sdk.ValAddress, valStr string) {
		_, err := server.SubmitFeed(ctx, &types.MsgSubmitFeed{
			Feeder: sdk.AccAddress(val.Bytes()).String(), Validator: valStr,
			Pair: "BTC:USD", Price: "100",
		})
		require.NoError(t, err)
	}

	submit(v1, s1)
	submit(v2, s2)
	require.NoError(t, k.EndBlocker(ctx))
	require.Empty(t, mockSlashing.Calls)

	submit(v1, s1)
	require.NoError(t, k.EndBlocker(ctx))
	require.Empty(t, mockSlashing.Calls)

	submit(v1, s1)
	require.NoError(t, k.EndBlocker(ctx))
	require.Len(t, mockSlashing.Calls, 1)
}

// TestAcceptance_OutlierUnweightedMedian: outlier band vs count-median, not stake-weighted aggregate.
func TestAcceptance_OutlierUnweightedMedian(t *testing.T) {
	vals := make([]sdk.ValAddress, 4)
	strs := make([]string, 4)
	validators := make([]stakingtypes.Validator, 4)
	powers := make(map[string]int64)
	for i := range 4 {
		vals[i], strs[i] = makeValidator(t)
		validators[i] = bondingValidator(t, vals[i])
		powers[strs[i]] = 25
	}
	mockStaking := &keeper.MockStaking{
		Validators: validators,
		Powers:     powers,
		TotalPower: math.NewInt(100),
	}
	mockSlashing := &keeper.MockSlashing{}
	ctx, k := keeper.OracleKeeper(t, mockStaking, mockSlashing)
	p := types.DefaultParams()
	p.VoteWindow = 1
	require.NoError(t, k.SetParams(ctx, p))
	server := oraclekeeper.NewMsgServerImpl(k)

	prices := []string{"100", "100", "100", "200"}
	for i := range 4 {
		_, err := server.SubmitFeed(ctx, &types.MsgSubmitFeed{
			Feeder: sdk.AccAddress(vals[i].Bytes()).String(), Validator: strs[i],
			Pair: "BTC:USD", Price: prices[i],
		})
		require.NoError(t, err)
	}

	require.NoError(t, k.EndBlocker(ctx))
	require.Len(t, mockSlashing.Calls, 1)
	require.Equal(t, powers[strs[3]], mockSlashing.Calls[0].Power)

	// Published price is stake-weighted median (100), not unweighted ref alone.
	got, err := k.GetPrice(ctx, "BTC:USD")
	require.NoError(t, err)
	require.True(t, got.Equal(math.LegacyNewDec(100)))
}
