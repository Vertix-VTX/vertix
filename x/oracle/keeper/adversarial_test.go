package keeper_test

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	oraclekeeper "github.com/vertix-network/vertix/x/oracle/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

// TestAdversarial_HighStakeExtremeDoesNotInvertOutlierSlash asserts outlier detection
// uses the count-median reference band, so a whale stake cannot frame honest validators.
func TestAdversarial_HighStakeExtremeDoesNotInvertOutlierSlash(t *testing.T) {
	tests := []struct {
		name       string
		powers     []int64
		prices     []string
		whaleIndex int
	}{
		{
			name:       "70pct_stake_extreme_price",
			powers:     []int64{70, 10, 10, 10},
			prices:     []string{"1000000", "100", "100", "100"},
			whaleIndex: 0,
		},
		{
			name:       "90pct_stake_extreme_price",
			powers:     []int64{90, 5, 5},
			prices:     []string{"500000", "100", "100"},
			whaleIndex: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n := len(tc.powers)
			vals := make([]sdk.ValAddress, n)
			strs := make([]string, n)
			validators := make([]stakingtypes.Validator, n)
			powers := make(map[string]int64)
			var total int64
			for i := range n {
				vals[i], strs[i] = makeValidator(t)
				validators[i] = bondingValidator(t, vals[i])
				powers[strs[i]] = tc.powers[i]
				total += tc.powers[i]
			}
			mockStaking := &keeper.MockStaking{
				Validators: validators,
				Powers:     powers,
				TotalPower: math.NewInt(total),
			}
			mockSlashing := &keeper.MockSlashing{}
			ctx, k := keeper.OracleKeeper(t, mockStaking, mockSlashing)
			p := types.DefaultParams()
			p.VoteWindow = 1
			require.NoError(t, k.SetParams(ctx, p))
			server := oraclekeeper.NewMsgServerImpl(k)

			for i := range n {
				_, err := server.SubmitFeed(ctx, &types.MsgSubmitFeed{
					Feeder:    sdk.AccAddress(vals[i].Bytes()).String(),
					Validator: strs[i],
					Pair:      "BTC:USD",
					Price:     tc.prices[i],
				})
				require.NoError(t, err)
			}

			require.NoError(t, k.EndBlocker(ctx))
			require.Len(t, mockSlashing.Calls, 1, "only the extreme submitter is outlier-slashed")
			require.Equal(t, powers[strs[tc.whaleIndex]], mockSlashing.Calls[0].Power)
		})
	}
}

// TestAdversarial_MissAndQuorumBoundary covers quorumFraction and missThreshold edges.
func TestAdversarial_MissAndQuorumBoundary(t *testing.T) {
	tests := []struct {
		name             string
		submitterPower   int64
		totalPower       int64
		quorumFraction   string
		missWindowSize   int64
		missThreshold    string
		v1OnlyWindows    int // windows where only v1 submits (v2 misses quorum-live pair)
		bothWindows      int // windows where v1 and v2 both submit
		checkMissAfterV1 int // assert v2 miss counter after this many v1-only windows (0 = skip)
		wantMissV2       int64
		wantPrice        bool
		wantSlash        bool
	}{
		{
			name:             "submitted_power_just_below_quorum",
			submitterPower:   66,
			totalPower:       100,
			quorumFraction:   "0.667",
			missWindowSize:   2,
			missThreshold:    "0.5",
			v1OnlyWindows:    1,
			wantPrice:        false,
			checkMissAfterV1: 1,
			wantMissV2:       0, // quorum not met → no miss accounting
			wantSlash:        false,
		},
		{
			name:             "submitted_power_at_quorum",
			submitterPower:   67,
			totalPower:       100,
			quorumFraction:   "0.667",
			missWindowSize:   2,
			missThreshold:    "0.5",
			v1OnlyWindows:    1,
			wantPrice:        true,
			checkMissAfterV1: 1,
			wantMissV2:       1,
			wantSlash:        false,
		},
		{
			name:             "miss_rate_at_threshold_no_slash",
			submitterPower:   70,
			totalPower:       100,
			quorumFraction:   "0.667",
			missWindowSize:   3,
			missThreshold:    "0.667",
			v1OnlyWindows:    2,
			bothWindows:      1,
			wantPrice:        true,
			checkMissAfterV1: 2,
			wantMissV2:       2,
			wantSlash:        false,
		},
		{
			name:             "miss_rate_above_threshold_slash",
			submitterPower:   70,
			totalPower:       100,
			quorumFraction:   "0.667",
			missWindowSize:   3,
			missThreshold:    "0.5",
			v1OnlyWindows:    3,
			wantPrice:        true,
			checkMissAfterV1: 2,
			wantMissV2:       2,
			wantSlash:        true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v1, s1 := makeValidator(t)
			v2, s2 := makeValidator(t)
			mockStaking := &keeper.MockStaking{
				Validators: []stakingtypes.Validator{
					bondingValidator(t, v1),
					bondingValidator(t, v2),
				},
				Powers: map[string]int64{
					s1: tc.submitterPower,
					s2: tc.totalPower - tc.submitterPower,
				},
				TotalPower: math.NewInt(tc.totalPower),
			}
			mockSlashing := &keeper.MockSlashing{}
			ctx, k := keeper.OracleKeeper(t, mockStaking, mockSlashing)
			p := types.DefaultParams()
			p.VoteWindow = 1
			p.QuorumFraction = tc.quorumFraction
			p.MissWindowSize = tc.missWindowSize
			p.MissThreshold = tc.missThreshold
			require.NoError(t, k.SetParams(ctx, p))
			server := oraclekeeper.NewMsgServerImpl(k)

			submitV1 := func() {
				_, err := server.SubmitFeed(ctx, &types.MsgSubmitFeed{
					Feeder: sdk.AccAddress(v1.Bytes()).String(), Validator: s1,
					Pair: "BTC:USD", Price: "100",
				})
				require.NoError(t, err)
			}
			submitBoth := func() {
				submitV1()
				_, err := server.SubmitFeed(ctx, &types.MsgSubmitFeed{
					Feeder: sdk.AccAddress(v2.Bytes()).String(), Validator: s2,
					Pair: "BTC:USD", Price: "100",
				})
				require.NoError(t, err)
			}

			runV1Only := func(n int) {
				for range n {
					submitV1()
					require.NoError(t, k.EndBlocker(ctx))
				}
			}
			if tc.checkMissAfterV1 > 0 {
				runV1Only(tc.checkMissAfterV1)
				miss, err := k.GetMissCounter(ctx, v2)
				require.NoError(t, err)
				require.Equal(t, tc.wantMissV2, miss)
			}
			if tc.v1OnlyWindows > tc.checkMissAfterV1 {
				runV1Only(tc.v1OnlyWindows - tc.checkMissAfterV1)
			}
			for range tc.bothWindows {
				submitBoth()
				require.NoError(t, k.EndBlocker(ctx))
			}

			if tc.wantPrice {
				_, err := k.GetPrice(ctx, "BTC:USD")
				require.NoError(t, err)
			} else {
				_, err := k.GetPrice(ctx, "BTC:USD")
				require.ErrorIs(t, err, types.ErrNoPrice)
			}

			if tc.wantSlash {
				require.NotEmpty(t, mockSlashing.Calls)
			} else {
				require.Empty(t, mockSlashing.Calls)
			}
		})
	}
}

// TestAdversarial_StaleAggregatedPrice verifies GetAggregatedPrice rejects aged prices.
func TestAdversarial_StaleAggregatedPrice(t *testing.T) {
	tests := []struct {
		name        string
		maxPriceAge int64
		ageSeconds  int64
		wantStale   bool
	}{
		{
			name:        "one_second_past_max_age",
			maxPriceAge: 300,
			ageSeconds:  301,
			wantStale:   true,
		},
		{
			name:        "exactly_at_max_age",
			maxPriceAge: 300,
			ageSeconds:  300,
			wantStale:   false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
			p := types.DefaultParams()
			p.MaxPriceAge = tc.maxPriceAge
			require.NoError(t, k.SetParams(ctx, p))

			now := time.Unix(2000, 0).UTC()
			ctx = ctx.WithBlockTime(now)
			blockTime := now.Add(-time.Duration(tc.ageSeconds) * time.Second)
			require.NoError(t, k.SetAggregatedPrice(ctx, types.AggregatedPrice{
				Pair:        "BTC:USD",
				Price:       "50000",
				BlockHeight: 1,
				BlockTime:   blockTime,
			}))

			_, err := k.GetAggregatedPrice(ctx, "BTC:USD")
			if tc.wantStale {
				require.ErrorIs(t, err, types.ErrStalePrice)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
