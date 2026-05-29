package keeper

import (
	"context"
	"errors"
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
	supply   sdk.Coins
	Burned   sdk.Coins
}

func NewMockBank() *MockBank {
	return &MockBank{balances: map[string]sdk.Coins{}, supply: sdk.NewCoins(), Burned: sdk.NewCoins()}
}

func (m *MockBank) SetBalance(addr sdk.AccAddress, coins sdk.Coins) {
	m.balances[addr.String()] = coins
}
func (m *MockBank) Balance(addr sdk.AccAddress) sdk.Coins { return m.balances[addr.String()] }
func (m *MockBank) ModuleAddr(name string) sdk.AccAddress { return authtypes.NewModuleAddress(name) }

func (m *MockBank) GetSupply(_ context.Context, denom string) sdk.Coin {
	return sdk.NewCoin(denom, m.supply.AmountOf(denom))
}

func (m *MockBank) MintCoins(_ context.Context, module string, amt sdk.Coins) error {
	addr := authtypes.NewModuleAddress(module).String()
	m.balances[addr] = m.balances[addr].Add(amt...)
	m.supply = m.supply.Add(amt...)
	return nil
}

func (m *MockBank) SendCoinsFromAccountToModule(_ context.Context, sender sdk.AccAddress, module string, amt sdk.Coins) error {
	if !m.balances[sender.String()].IsAllGTE(amt) {
		return errors.New("insufficient funds")
	}
	to := authtypes.NewModuleAddress(module).String()
	m.balances[sender.String()] = m.balances[sender.String()].Sub(amt...)
	m.balances[to] = m.balances[to].Add(amt...)
	return nil
}

func (m *MockBank) SendCoinsFromModuleToAccount(_ context.Context, module string, recipient sdk.AccAddress, amt sdk.Coins) error {
	from := authtypes.NewModuleAddress(module).String()
	if !m.balances[from].IsAllGTE(amt) {
		return errors.New("insufficient module funds")
	}
	m.balances[from] = m.balances[from].Sub(amt...)
	m.balances[recipient.String()] = m.balances[recipient.String()].Add(amt...)
	return nil
}

func (m *MockBank) SendCoins(_ context.Context, from, to sdk.AccAddress, amt sdk.Coins) error {
	if !m.balances[from.String()].IsAllGTE(amt) {
		return errors.New("insufficient funds")
	}
	m.balances[from.String()] = m.balances[from.String()].Sub(amt...)
	m.balances[to.String()] = m.balances[to.String()].Add(amt...)
	return nil
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
	if !m.balances[addr].IsAllGTE(amt) {
		return errors.New("insufficient module balance to burn")
	}
	m.balances[addr] = m.balances[addr].Sub(amt...)
	if m.supply.IsAllGTE(amt) {
		m.supply = m.supply.Sub(amt...)
	}
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
