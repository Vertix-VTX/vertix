package keeper

import (
	"context"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cosmosdb "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	oraclekeeper "github.com/vertix-network/vertix/x/oracle/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

type MockStaking struct {
	Validators []stakingtypes.Validator
	Powers     map[string]int64
	TotalPower math.Int // total consensus power for quorum (same units as Powers values)
}

func (m *MockStaking) GetValidator(_ context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error) {
	for _, v := range m.Validators {
		if v.OperatorAddress == addr.String() {
			return v, nil
		}
	}
	return stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound
}

func (m *MockStaking) GetLastValidatorPower(_ context.Context, addr sdk.ValAddress) (int64, error) {
	if p, ok := m.Powers[addr.String()]; ok {
		return p, nil
	}
	return 0, nil
}

func (m *MockStaking) GetBondedValidatorsByPower(_ context.Context) ([]stakingtypes.Validator, error) {
	return m.Validators, nil
}

func (m *MockStaking) GetLastTotalPower(_ context.Context) (math.Int, error) {
	if !m.TotalPower.IsZero() {
		return m.TotalPower, nil
	}
	var sum int64
	for _, p := range m.Powers {
		sum += p
	}
	return math.NewInt(sum), nil
}

type MockSlashing struct {
	Calls []SlashCall
}

type SlashCall struct {
	ConsAddr sdk.ConsAddress
	Fraction math.LegacyDec
	Power    int64
	Height   int64
}

func (m *MockSlashing) Slash(_ context.Context, consAddr sdk.ConsAddress, fraction math.LegacyDec, power, height int64) error {
	m.Calls = append(m.Calls, SlashCall{consAddr, fraction, power, height})
	return nil
}

func OracleKeeper(t *testing.T, staking *MockStaking, slashing *MockSlashing) (sdk.Context, oraclekeeper.Keeper) {
	t.Helper()
	memDB := cosmosdb.NewMemDB()
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	stateStore := store.NewCommitMultiStore(memDB, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, memDB)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	k := oraclekeeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		staking,
		slashing,
		authority,
	)
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 1}, false, log.NewNopLogger())
	require.NoError(t, k.SetParams(ctx, types.DefaultParams()))
	return ctx, k
}
