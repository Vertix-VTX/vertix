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
	"github.com/stretchr/testify/require"

	rwakeeper "github.com/vertix-network/vertix/x/rwa/keeper"
	"github.com/vertix-network/vertix/x/rwa/types"
)

// MockOracle returns a canned price or error for any pair.
type MockOracle struct {
	Price math.LegacyDec
	Err   error
}

func (m MockOracle) GetPrice(_ context.Context, _ string) (math.LegacyDec, error) {
	if m.Err != nil {
		return math.LegacyZeroDec(), m.Err
	}
	return m.Price, nil
}

// MockDistribution records community-pool funding.
type MockDistribution struct {
	Funded sdk.Coins
}

func NewMockDistribution() *MockDistribution { return &MockDistribution{Funded: sdk.NewCoins()} }

func (m *MockDistribution) FundCommunityPool(_ context.Context, amount sdk.Coins, _ sdk.AccAddress) error {
	m.Funded = m.Funded.Add(amount...)
	return nil
}

// RWAKeeper builds an in-memory rwa keeper with the supplied mocks.
func RWAKeeper(t testing.TB, bank *MockBank, oracle types.OracleKeeper, distr types.DistributionKeeper) (rwakeeper.Keeper, sdk.Context) {
	t.Helper()
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	db := cosmosdb.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	k := rwakeeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		log.NewNopLogger(),
		authority,
		MockAccount{},
		bank,
		oracle,
		distr,
	)
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 1}, false, log.NewNopLogger())
	require.NoError(t, k.SetParams(ctx, types.DefaultParams()))
	return k, ctx
}

// RwaKeeper is a convenience wrapper with default mocks.
func RwaKeeper(t testing.TB) (rwakeeper.Keeper, sdk.Context) {
	t.Helper()
	return RWAKeeper(t, NewMockBank(), MockOracle{Price: math.LegacyNewDec(1)}, NewMockDistribution())
}
