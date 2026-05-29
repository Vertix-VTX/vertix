package keeper

import (
	"context"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/require"

	feeskeeper "github.com/vertix-network/vertix/x/fees/keeper"
	"github.com/vertix-network/vertix/x/fees/types"
)

// MockAccount resolves module names to their canonical module addresses,
// matching the real x/auth account keeper.
type MockAccount struct{}

func (MockAccount) GetModuleAddress(name string) sdk.AccAddress {
	return authtypes.NewModuleAddress(name)
}

// MockBank tracks per-address balances keyed by module-derived address and
// records what was burned. Module transfers resolve names via NewModuleAddress
// so they agree with MockAccount.
type MockBank struct {
	balances map[string]sdk.Coins
	Burned   sdk.Coins
}

func NewMockBank() *MockBank {
	return &MockBank{balances: map[string]sdk.Coins{}, Burned: sdk.NewCoins()}
}

func (m *MockBank) SetModuleBalance(moduleName string, coins sdk.Coins) {
	m.balances[authtypes.NewModuleAddress(moduleName).String()] = coins
}

func (m *MockBank) ModuleBalance(moduleName string) sdk.Coins {
	return m.balances[authtypes.NewModuleAddress(moduleName).String()]
}

func (m *MockBank) GetBalance(_ context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	return sdk.NewCoin(denom, m.balances[addr.String()].AmountOf(denom))
}

func (m *MockBank) SendCoinsFromModuleToModule(_ context.Context, from, to string, amt sdk.Coins) error {
	fromAddr := authtypes.NewModuleAddress(from).String()
	toAddr := authtypes.NewModuleAddress(to).String()
	m.balances[fromAddr] = m.balances[fromAddr].Sub(amt...)
	m.balances[toAddr] = m.balances[toAddr].Add(amt...)
	return nil
}

func (m *MockBank) BurnCoins(_ context.Context, module string, amt sdk.Coins) error {
	addr := authtypes.NewModuleAddress(module).String()
	m.balances[addr] = m.balances[addr].Sub(amt...)
	m.Burned = m.Burned.Add(amt...)
	return nil
}

// FeesKeeper builds an in-memory fees keeper with the given mock bank.
func FeesKeeper(t testing.TB, bank *MockBank) (feeskeeper.Keeper, sdk.Context) {
	t.Helper()
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	k := feeskeeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		log.NewNopLogger(),
		authority,
		MockAccount{},
		bank,
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 1}, false, log.NewNopLogger())
	require.NoError(t, k.SetParams(ctx, types.DefaultParams()))
	return k, ctx
}
