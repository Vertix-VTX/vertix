package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
	oraclekeeper "github.com/vertix-network/vertix/x/oracle/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func bondingValidator(t *testing.T, val sdk.ValAddress) stakingtypes.Validator {
	t.Helper()
	pub := ed25519.GenPrivKey().PubKey()
	pkAny, err := codectypes.NewAnyWithValue(pub)
	require.NoError(t, err)
	return stakingtypes.Validator{
		OperatorAddress: val.String(),
		ConsensusPubkey: pkAny,
		Status:          stakingtypes.Bonded,
	}
}

func makeValidator(t *testing.T) (sdk.ValAddress, string) {
	t.Helper()
	val := sdk.ValAddress(sdk.MustAccAddressFromBech32(sample.AccAddress()).Bytes())
	return val, val.String()
}

func TestEndBlockAggregatesWeightedMedian(t *testing.T) {
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
}

func TestEndBlockSkipsPairBelowQuorum(t *testing.T) {
	v1, s1 := makeValidator(t)
	mock := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{bondingValidator(t, v1)},
		Powers:     map[string]int64{s1: 10},
		TotalPower: math.NewInt(100),
	}
	ctx, k := keeper.OracleKeeper(t, mock, &keeper.MockSlashing{})
	p := types.DefaultParams()
	p.VoteWindow = 1
	require.NoError(t, k.SetParams(ctx, p))
	server := oraclekeeper.NewMsgServerImpl(k)
	_, err := server.SubmitFeed(ctx, &types.MsgSubmitFeed{
		Feeder: sdk.AccAddress(v1.Bytes()).String(), Validator: s1,
		Pair: "BTC:USD", Price: "100",
	})
	require.NoError(t, err)
	require.NoError(t, k.EndBlocker(ctx))
	_, err = k.GetPrice(ctx, "BTC:USD")
	require.ErrorIs(t, err, types.ErrNoPrice)
}
