# Phase 2 — `x/fees` Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a working, coordination-only `x/fees` module that, at each `EndBlock`, burns `BurnRatio` (default 40%) of the native `uvtx` balance in the standard fee collector and leaves the remainder for the SDK's native `x/distribution` `BeginBlock` to distribute to stakers — satisfying the Phase 2 acceptance gate.

**Architecture:** Burn-only sweep (spec D1). Ignite scaffolds the module skeleton + depinject wiring; we replace the generated protos with the approved Phase 2 surface, then build in vertical slices (types → params → store → message handler → EndBlock burn → query → genesis → app wiring → acceptance). `x/fees` calls only `bank` (burn) + `auth` (module address) and never reimplements distribution. External deps are interface-typed (`AccountKeeper`, `BankKeeper`) with mocks in `testutil/keeper`.

**Tech Stack:** Go 1.25 (repo toolchain), Cosmos SDK v0.50.x, CometBFT v0.38.x, Ignite CLI v28, buf v1.30+, golangci-lint v1.57+.

**Reference spec:** [`docs/specs/2026-05-29-phase-2-fees-module-design.md`](../specs/2026-05-29-phase-2-fees-module-design.md). Where this plan and the spec disagree, the spec wins.

**Conventions:** Run commands from repo root `/home/nam-nguyen/my-projects/vertix-projects/vertix`. Commit messages follow Conventional Commits with scope (`docs/coding-standards.md` §7.1). The chain base/bond denom is `uvtx`.

---

## File Map

| File | Responsibility |
|---|---|
| `proto/vertix/fees/v1/*.proto` | Wire format: types, tx, query, genesis |
| `x/fees/types/keys.go` | `ModuleName`, `StoreKey`, params store key, `FeeDenom` constant |
| `x/fees/types/params.go` | `DefaultParams()`, `Validate()` (bounds + sum==1), dec accessors |
| `x/fees/types/errors.go` | Registered sentinel errors |
| `x/fees/types/events.go` | Event type + attribute constants |
| `x/fees/types/msgs.go` | `ValidateBasic` for `MsgUpdateParams` |
| `x/fees/types/expected_keepers.go` | `AccountKeeper`, `BankKeeper` interfaces |
| `x/fees/types/genesis.go` | `DefaultGenesis()`, `GenesisState.Validate()` |
| `x/fees/keeper/keeper.go` | Keeper struct, params get/set, keeper handles |
| `x/fees/keeper/abci.go` | `EndBlocker`: compute burn → move → burn → emit |
| `x/fees/keeper/msg_server.go` | `UpdateParams` |
| `x/fees/keeper/grpc_query.go` | `Params` query |
| `x/fees/keeper/genesis.go` | `InitGenesis`, `ExportGenesis` |
| `x/fees/module/module.go` | AppModule (EndBlock → keeper.EndBlocker, genesis, services) |
| `x/fees/module/autocli.go` | CLI query/tx wiring |
| `testutil/keeper/fees.go` | In-memory keeper fixture + mock bank/account keepers |
| `app/app_config.go` | ModuleConfig, endBlockers (last), genesisModuleOrder, maccPerms (Burner) |
| `app/app.go` | FeesKeeper inject + App struct field |
| `app/app_test.go` | Module wired, genesis round-trip for fees |
| `docs/technical-design.md` | §4.3 / §4.6 doc-sync (Task 17) |

---

## Task 1: Scaffold the fees module

**Files:**
- Create: `x/fees/**`, `proto/vertix/fees/v1/**` (Ignite-generated boilerplate)
- Modify: `app/app.go`, `app/app_config.go` (Ignite inserts at `# stargate/app/...` markers)

- [ ] **Step 1: Scaffold with dependencies**

Run:

```bash
cd /home/nam-nguyen/my-projects/vertix-projects/vertix
ignite scaffold module fees --dep bank,distribution -y
```

Expected: creates `x/fees/`, `proto/vertix/fees/v1/`, and updates `app/app.go` + `app/app_config.go`.

- [ ] **Step 2: Verify scaffold compiles**

Run:

```bash
go build ./...
```

Expected: success (scaffold boilerplate builds).

- [ ] **Step 3: Note scaffold files to replace later**

Ignite generates stubs that are overwritten in later tasks. Do not treat these as final:

- `x/fees/types/params.go` (scaffold `Params` / `DefaultParams`) → Task 6
- `x/fees/types/genesis.go` (scaffold `GenesisState` / `Validate`) → Task 14
- `x/fees/types/expected_keepers.go` (stub keepers) → Task 8
- `x/fees/keeper/msg_server.go`, `grpc_query.go` (placeholder handlers) → Tasks 11, 13
- `x/fees/module/autocli.go` → Task 13

- [ ] **Step 4: Commit**

```bash
git add x/fees/ proto/vertix/fees/ app/app.go app/app_config.go go.mod go.sum
git commit -m "$(cat <<'EOF'
feat(fees): scaffold x/fees module via ignite

Depinject wiring for bank, distribution. Refs:
docs/specs/2026-05-29-phase-2-fees-module-design.md
EOF
)"
```

---

## Task 2: Replace protobuf definitions and regenerate

**Files:**
- Modify: `proto/vertix/fees/v1/types.proto`
- Modify: `proto/vertix/fees/v1/tx.proto`
- Modify: `proto/vertix/fees/v1/query.proto`
- Modify: `proto/vertix/fees/v1/genesis.proto`
- Create: generated `x/fees/types/*.pb.go` via `make proto-gen`

- [ ] **Step 1: Write `proto/vertix/fees/v1/types.proto`**

Replace file contents with:

```protobuf
syntax = "proto3";
package vertix.fees.v1;

import "cosmos_proto/cosmos.proto";

option go_package = "github.com/vertix-network/vertix/x/fees/types";

message FeesParams {
  string burn_ratio         = 1 [(cosmos_proto.scalar) = "cosmos.Dec"];
  string distribution_ratio = 2 [(cosmos_proto.scalar) = "cosmos.Dec"];
}
```

- [ ] **Step 2: Write `proto/vertix/fees/v1/tx.proto`**

```protobuf
syntax = "proto3";
package vertix.fees.v1;

import "amino/amino.proto";
import "cosmos/msg/v1/msg.proto";
import "cosmos_proto/cosmos.proto";
import "gogoproto/gogo.proto";
import "vertix/fees/v1/types.proto";

option go_package = "github.com/vertix-network/vertix/x/fees/types";

service Msg {
  option (cosmos.msg.v1.service) = true;
  rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);
}

message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";
  option (amino.name)           = "vertix/x/fees/MsgUpdateParams";

  string     authority = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  FeesParams params    = 2 [(gogoproto.nullable) = false];
}

message MsgUpdateParamsResponse {}
```

- [ ] **Step 3: Write `proto/vertix/fees/v1/query.proto`**

```protobuf
syntax = "proto3";
package vertix.fees.v1;

import "gogoproto/gogo.proto";
import "google/api/annotations.proto";
import "vertix/fees/v1/types.proto";

option go_package = "github.com/vertix-network/vertix/x/fees/types";

service Query {
  rpc Params(QueryParamsRequest) returns (QueryParamsResponse) {
    option (google.api.http).get = "/vertix/fees/v1/params";
  }
}

message QueryParamsRequest {}

message QueryParamsResponse {
  FeesParams params = 1 [(gogoproto.nullable) = false];
}
```

- [ ] **Step 4: Write `proto/vertix/fees/v1/genesis.proto`**

```protobuf
syntax = "proto3";
package vertix.fees.v1;

import "gogoproto/gogo.proto";
import "vertix/fees/v1/types.proto";

option go_package = "github.com/vertix-network/vertix/x/fees/types";

message GenesisState {
  FeesParams params = 1 [(gogoproto.nullable) = false];
}
```

- [ ] **Step 5: Regenerate Go from proto**

Run:

```bash
make proto-gen
```

Expected: regenerates `x/fees/types/*.pb.go`, `x/fees/types/query.pb.gw.go`, etc.

- [ ] **Step 6: Run buf lint**

Run:

```bash
cd proto && buf lint
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add proto/vertix/fees/ x/fees/types/
git commit -m "$(cat <<'EOF'
feat(fees): define Phase 2 protobuf surface

FeesParams (burn_ratio, distribution_ratio), MsgUpdateParams, Params query, genesis.
Refs: docs/specs/2026-05-29-phase-2-fees-module-design.md §5
EOF
)"
```

---

## Task 3: Store keys and module constants

**Files:**
- Replace: `x/fees/types/keys.go` (overwrite scaffold)
- Create: `x/fees/types/keys_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/fees/types/keys_test.go`:

```go
package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/fees/types"
)

func TestModuleConstants(t *testing.T) {
	require.Equal(t, "fees", types.ModuleName)
	require.Equal(t, "fees", types.StoreKey)
	require.Equal(t, "uvtx", types.FeeDenom)
	require.Equal(t, []byte{0x01}, types.KeyParams)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./x/fees/types/ -run TestModuleConstants -v`

Expected: FAIL — `FeeDenom` / `KeyParams` not defined (scaffold differs).

- [ ] **Step 3: Write `x/fees/types/keys.go`**

```go
package types

const (
	// ModuleName defines the module name.
	ModuleName = "fees"

	// StoreKey defines the primary module store key.
	StoreKey = ModuleName

	// FeeDenom is the native token whose fee-collector balance x/fees burns.
	// MUST match the chain base/bond denom (config.yml: uvtx). Non-FeeDenom
	// coins in the fee collector are never burned (spec D2).
	FeeDenom = "uvtx"
)

// KeyParams is the store key under which FeesParams is persisted.
var KeyParams = []byte{0x01}
```

Delete any scaffold-generated `Params` key collections (`ParamsKey`, etc.) left in the file that conflict.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./x/fees/types/ -run TestModuleConstants -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/fees/types/keys.go x/fees/types/keys_test.go
git commit -m "feat(fees): add module constants and params store key"
```

---

## Task 4: Registered errors

**Files:**
- Create: `x/fees/types/errors.go`

- [ ] **Step 1: Write `x/fees/types/errors.go`**

```go
package types

import "cosmossdk.io/errors"

var (
	ErrUnauthorized    = errors.Register(ModuleName, 2, "unauthorized")
	ErrInvalidParams   = errors.Register(ModuleName, 3, "invalid params")
	ErrInvalidRatioSum = errors.Register(ModuleName, 4, "burn_ratio + distribution_ratio must equal 1")
)
```

- [ ] **Step 2: Verify build**

Run: `go build ./x/fees/types/...`

Expected: success

- [ ] **Step 3: Commit**

```bash
git add x/fees/types/errors.go
git commit -m "feat(fees): register module sentinel errors"
```

---

## Task 5: Events

**Files:**
- Create: `x/fees/types/events.go`

- [ ] **Step 1: Write `x/fees/types/events.go`**

```go
package types

const (
	EventTypeFeeBurned      = "fee_burned"
	EventTypeFeeDistributed = "fee_distributed"
	EventTypeParamsUpdated  = "fee_params_updated"

	AttributeKeyAmount            = "amount"
	AttributeKeyBurnRatio         = "burn_ratio"
	AttributeKeyDistributionRatio = "distribution_ratio"
)
```

- [ ] **Step 2: Commit**

```bash
git add x/fees/types/events.go
git commit -m "feat(fees): define typed event constants"
```

---

## Task 6: Params defaults and validation

**Files:**
- Replace: `x/fees/types/params.go` (delete Ignite scaffold `Params` / `DefaultParams` first)
- Create: `x/fees/types/params_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/fees/types/params_test.go`:

```go
package types_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/fees/types"
)

func TestDefaultParamsValid(t *testing.T) {
	p := types.DefaultParams()
	require.NoError(t, p.Validate())
	require.Equal(t, "0.400000000000000000", p.BurnRatio)
	require.Equal(t, "0.600000000000000000", p.DistributionRatio)
}

func TestValidateRejectsRatiosNotSummingToOne(t *testing.T) {
	p := types.FeesParams{BurnRatio: "0.40", DistributionRatio: "0.50"}
	require.ErrorIs(t, p.Validate(), types.ErrInvalidRatioSum)
}

func TestValidateRejectsRatioAboveOne(t *testing.T) {
	p := types.FeesParams{BurnRatio: "1.50", DistributionRatio: "-0.50"}
	require.Error(t, p.Validate())
}

func TestValidateRejectsUnparseableRatio(t *testing.T) {
	p := types.FeesParams{BurnRatio: "abc", DistributionRatio: "0.60"}
	require.Error(t, p.Validate())
}

func TestBurnRatioDec(t *testing.T) {
	p := types.DefaultParams()
	d, err := p.BurnRatioDec()
	require.NoError(t, err)
	require.True(t, d.Equal(math.LegacyNewDecWithPrec(40, 2)))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./x/fees/types/ -run 'TestDefaultParams|TestValidate|TestBurnRatioDec' -v`

Expected: FAIL — `DefaultParams` signature differs / `BurnRatioDec` not defined.

- [ ] **Step 3: Write `x/fees/types/params.go`**

```go
package types

import "cosmossdk.io/math"

// DefaultParams returns the launch fee split: 40% burn, 60% to stakers
// (the latter realized by the native x/distribution BeginBlock — spec D1/D7).
func DefaultParams() FeesParams {
	return FeesParams{
		BurnRatio:         math.LegacyNewDecWithPrec(40, 2).String(), // "0.40"
		DistributionRatio: math.LegacyNewDecWithPrec(60, 2).String(), // "0.60"
	}
}

// BurnRatioDec parses burn_ratio into a LegacyDec.
func (p FeesParams) BurnRatioDec() (math.LegacyDec, error) {
	return math.LegacyNewDecFromStr(p.BurnRatio)
}

// DistributionRatioDec parses distribution_ratio into a LegacyDec.
func (p FeesParams) DistributionRatioDec() (math.LegacyDec, error) {
	return math.LegacyNewDecFromStr(p.DistributionRatio)
}

// Validate enforces both ratios are in [0,1] and sum to exactly 1.
func (p FeesParams) Validate() error {
	burn, err := math.LegacyNewDecFromStr(p.BurnRatio)
	if err != nil {
		return ErrInvalidParams.Wrapf("burn_ratio: %v", err)
	}
	distr, err := math.LegacyNewDecFromStr(p.DistributionRatio)
	if err != nil {
		return ErrInvalidParams.Wrapf("distribution_ratio: %v", err)
	}
	if burn.IsNegative() || burn.GT(math.LegacyOneDec()) {
		return ErrInvalidParams.Wrap("burn_ratio must be in [0, 1]")
	}
	if distr.IsNegative() || distr.GT(math.LegacyOneDec()) {
		return ErrInvalidParams.Wrap("distribution_ratio must be in [0, 1]")
	}
	if !burn.Add(distr).Equal(math.LegacyOneDec()) {
		return ErrInvalidRatioSum
	}
	return nil
}
```

Do **not** add a custom `FeesParams.String()` — gogoproto generates one on the proto type; a second collides and fails to compile.

- [ ] **Step 4: Run tests**

Run: `go test ./x/fees/types/ -run 'TestDefaultParams|TestValidate|TestBurnRatioDec' -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/fees/types/params.go x/fees/types/params_test.go
git commit -m "feat(fees): add DefaultParams and Validate (sum==1)"
```

---

## Task 7: Message ValidateBasic

**Files:**
- Create: `x/fees/types/msgs.go`
- Create: `x/fees/types/msgs_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/fees/types/msgs_test.go`:

```go
package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/sample"
	"github.com/vertix-network/vertix/x/fees/types"
)

func TestMsgUpdateParamsValidateBasic(t *testing.T) {
	msg := &types.MsgUpdateParams{
		Authority: sample.AccAddress(),
		Params:    types.DefaultParams(),
	}
	require.NoError(t, msg.ValidateBasic())

	bad := &types.MsgUpdateParams{
		Authority: "not-an-address",
		Params:    types.DefaultParams(),
	}
	require.Error(t, bad.ValidateBasic())

	badParams := &types.MsgUpdateParams{
		Authority: sample.AccAddress(),
		Params:    types.FeesParams{BurnRatio: "0.40", DistributionRatio: "0.50"},
	}
	require.Error(t, badParams.ValidateBasic())
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/fees/types/ -run TestMsgUpdateParams -v`

Expected: FAIL — `ValidateBasic` not defined (or scaffold stub passes everything).

- [ ] **Step 3: Write `x/fees/types/msgs.go`**

```go
package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func (m *MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("authority: %v", err)
	}
	return m.Params.Validate()
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./x/fees/types/ -run TestMsgUpdateParams -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/fees/types/msgs.go x/fees/types/msgs_test.go
git commit -m "feat(fees): add ValidateBasic for MsgUpdateParams"
```

---

## Task 8: Expected keeper interfaces

**Files:**
- Replace: `x/fees/types/expected_keepers.go` (replace Ignite scaffold stubs)

- [ ] **Step 1: Write `x/fees/types/expected_keepers.go`**

```go
package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AccountKeeper resolves the standard fee collector module address.
type AccountKeeper interface {
	GetModuleAddress(name string) sdk.AccAddress
}

// BankKeeper is the minimal surface x/fees needs: read the fee-collector
// balance, move the burn portion into the fees module account, and burn it.
// x/fees deliberately does NOT distribute — the native x/distribution
// BeginBlock distributes the remainder left in the fee collector (spec D1).
type BankKeeper interface {
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	SendCoinsFromModuleToModule(ctx context.Context, senderModule, recipientModule string, amt sdk.Coins) error
	BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./x/fees/types/...`

Expected: success

- [ ] **Step 3: Commit**

```bash
git add x/fees/types/expected_keepers.go
git commit -m "feat(fees): define AccountKeeper and BankKeeper interfaces"
```

---

## Task 9: Test fixture with mock bank/account keepers

**Files:**
- Create: `testutil/keeper/fees.go`

- [ ] **Step 1: Write `testutil/keeper/fees.go`**

```go
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
```

- [ ] **Step 2: Verify compile (expected to fail until Task 10)**

Run: `go build ./testutil/keeper/...`

Expected: FAIL — `feeskeeper.NewKeeper` not yet defined. Proceed to Task 10; this file is committed there.

---

## Task 10: Keeper skeleton and params store

**Files:**
- Replace: `x/fees/keeper/keeper.go` (overwrite scaffold)
- Create: `x/fees/keeper/params_test.go`

- [ ] **Step 1: Write the failing params round-trip test**

Create `x/fees/keeper/params_test.go`:

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/fees/types"
)

func TestParamsRoundTrip(t *testing.T) {
	k, ctx := keeper.FeesKeeper(t, keeper.NewMockBank())
	got, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, types.DefaultParams(), got)
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/fees/keeper/ -run TestParamsRoundTrip -v`

Expected: FAIL — `NewKeeper` signature differs.

- [ ] **Step 3: Write `x/fees/keeper/keeper.go`**

```go
package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/vertix-network/vertix/x/fees/types"
)

type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService
	logger       log.Logger

	authority string

	accountKeeper types.AccountKeeper
	bankKeeper    types.BankKeeper
}

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	logger log.Logger,
	authority string,
	accountKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
) Keeper {
	return Keeper{
		cdc:           cdc,
		storeService:  storeService,
		logger:        logger,
		authority:     authority,
		accountKeeper: accountKeeper,
		bankKeeper:    bankKeeper,
	}
}

func (k Keeper) GetAuthority() string { return k.authority }

func (k Keeper) Logger() log.Logger {
	return k.logger.With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

func (k Keeper) GetParams(ctx context.Context) (types.FeesParams, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyParams)
	if err != nil {
		return types.FeesParams{}, err
	}
	if bz == nil {
		return types.DefaultParams(), nil
	}
	var p types.FeesParams
	k.cdc.MustUnmarshal(bz, &p)
	return p, nil
}

func (k Keeper) SetParams(ctx context.Context, p types.FeesParams) error {
	if err := p.Validate(); err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := k.cdc.Marshal(&p)
	if err != nil {
		return err
	}
	return store.Set(types.KeyParams, bz)
}
```

- [ ] **Step 4: Run test — expect PASS**

Run: `go test ./x/fees/keeper/ -run TestParamsRoundTrip -v`

Expected: PASS

- [ ] **Step 5: Commit keeper + testutil**

```bash
git add x/fees/keeper/keeper.go x/fees/keeper/params_test.go testutil/keeper/fees.go
git commit -m "feat(fees): add keeper skeleton, params store, and test fixture"
```

---

## Task 11: MsgUpdateParams handler

**Files:**
- Replace: `x/fees/keeper/msg_server.go` (overwrite scaffold UpdateParams)
- Create: `x/fees/keeper/msg_server_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/fees/keeper/msg_server_test.go`:

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	feeskeeper "github.com/vertix-network/vertix/x/fees/keeper"
	"github.com/vertix-network/vertix/x/fees/types"
)

func TestUpdateParamsAuthorized(t *testing.T) {
	k, ctx := keeper.FeesKeeper(t, keeper.NewMockBank())
	server := feeskeeper.NewMsgServerImpl(k)

	newParams := types.FeesParams{BurnRatio: "0.50", DistributionRatio: "0.50"}
	_, err := server.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: k.GetAuthority(),
		Params:    newParams,
	})
	require.NoError(t, err)

	got, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, newParams, got)
}

func TestUpdateParamsRejectsWrongAuthority(t *testing.T) {
	k, ctx := keeper.FeesKeeper(t, keeper.NewMockBank())
	server := feeskeeper.NewMsgServerImpl(k)

	_, err := server.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "vtx1wrongauthority000000000000000000000000",
		Params:    types.DefaultParams(),
	})
	require.Error(t, err)
}

func TestUpdateParamsRejectsBadRatioSum(t *testing.T) {
	k, ctx := keeper.FeesKeeper(t, keeper.NewMockBank())
	server := feeskeeper.NewMsgServerImpl(k)

	_, err := server.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: k.GetAuthority(),
		Params:    types.FeesParams{BurnRatio: "0.40", DistributionRatio: "0.50"},
	})
	require.ErrorIs(t, err, types.ErrInvalidRatioSum)
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/fees/keeper/ -run TestUpdateParams -v`

Expected: FAIL — scaffold `UpdateParams` does not enforce the gov authority / sum check.

- [ ] **Step 3: Write `x/fees/keeper/msg_server.go`**

```go
package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/fees/types"
)

type msgServer struct {
	k Keeper
}

func NewMsgServerImpl(k Keeper) types.MsgServer {
	return &msgServer{k: k}
}

var _ types.MsgServer = msgServer{}

func (m msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	if msg.Authority != m.k.GetAuthority() {
		return nil, types.ErrUnauthorized.Wrapf("expected %s, got %s", m.k.GetAuthority(), msg.Authority)
	}
	if err := m.k.SetParams(ctx, msg.Params); err != nil {
		return nil, err
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeParamsUpdated,
		sdk.NewAttribute(types.AttributeKeyBurnRatio, msg.Params.BurnRatio),
		sdk.NewAttribute(types.AttributeKeyDistributionRatio, msg.Params.DistributionRatio),
	))
	return &types.MsgUpdateParamsResponse{}, nil
}
```

If the scaffold defined `msgServer` / `NewMsgServerImpl` elsewhere (e.g. `msg_server.go` already has the struct), keep a single definition — delete the scaffold duplicate.

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./x/fees/keeper/ -run TestUpdateParams -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/fees/keeper/msg_server.go x/fees/keeper/msg_server_test.go
git commit -m "feat(fees): implement MsgUpdateParams with gov authority + sum check"
```

---

## Task 12: EndBlock burn logic

**Files:**
- Create: `x/fees/keeper/abci.go`
- Create: `x/fees/keeper/abci_test.go`

- [ ] **Step 1: Write the failing tests**

Create `x/fees/keeper/abci_test.go`:

```go
package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/fees/types"
)

func coins(denom string, amt int64) sdk.Coins {
	return sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(amt)))
}

func TestEndBlockerBurns40Percent(t *testing.T) {
	bank := keeper.NewMockBank()
	bank.SetModuleBalance(authtypes.FeeCollectorName, coins("uvtx", 1000))
	k, ctx := keeper.FeesKeeper(t, bank)

	require.NoError(t, k.EndBlocker(ctx))

	// 1000 × 0.40 = 400 burned; 600 left in the collector for native distribution.
	require.Equal(t, coins("uvtx", 400), bank.Burned)
	require.Equal(t, math.NewInt(600), bank.ModuleBalance(authtypes.FeeCollectorName).AmountOf("uvtx"))
	require.True(t, bank.ModuleBalance(types.ModuleName).IsZero()) // fees account empty between blocks
}

func TestEndBlockerFloorsBurn(t *testing.T) {
	bank := keeper.NewMockBank()
	bank.SetModuleBalance(authtypes.FeeCollectorName, coins("uvtx", 7)) // 7 × 0.40 = 2.8 → floor 2
	k, ctx := keeper.FeesKeeper(t, bank)

	require.NoError(t, k.EndBlocker(ctx))

	require.Equal(t, coins("uvtx", 2), bank.Burned)
	require.Equal(t, math.NewInt(5), bank.ModuleBalance(authtypes.FeeCollectorName).AmountOf("uvtx"))
}

func TestEndBlockerLeavesNonUvtxUntouched(t *testing.T) {
	bank := keeper.NewMockBank()
	bank.SetModuleBalance(authtypes.FeeCollectorName,
		coins("uvtx", 1000).Add(sdk.NewCoin("rwa/gold", math.NewInt(500))))
	k, ctx := keeper.FeesKeeper(t, bank)

	require.NoError(t, k.EndBlocker(ctx))

	require.Equal(t, coins("uvtx", 400), bank.Burned)
	require.Equal(t, math.NewInt(500),
		bank.ModuleBalance(authtypes.FeeCollectorName).AmountOf("rwa/gold")) // untouched
}

func TestEndBlockerZeroBalanceNoOp(t *testing.T) {
	bank := keeper.NewMockBank()
	k, ctx := keeper.FeesKeeper(t, bank)
	require.NoError(t, k.EndBlocker(ctx))
	require.True(t, bank.Burned.IsZero())
}

func TestEndBlockerZeroBurnRatioNoOp(t *testing.T) {
	bank := keeper.NewMockBank()
	bank.SetModuleBalance(authtypes.FeeCollectorName, coins("uvtx", 1000))
	k, ctx := keeper.FeesKeeper(t, bank)
	require.NoError(t, k.SetParams(ctx, types.FeesParams{BurnRatio: "0", DistributionRatio: "1"}))

	require.NoError(t, k.EndBlocker(ctx))
	require.True(t, bank.Burned.IsZero())
	require.Equal(t, math.NewInt(1000), bank.ModuleBalance(authtypes.FeeCollectorName).AmountOf("uvtx"))
}

func TestEndBlockerDustNoOp(t *testing.T) {
	bank := keeper.NewMockBank()
	bank.SetModuleBalance(authtypes.FeeCollectorName, coins("uvtx", 1)) // 1 × 0.40 = 0.4 → floor 0
	k, ctx := keeper.FeesKeeper(t, bank)
	require.NoError(t, k.EndBlocker(ctx))
	require.True(t, bank.Burned.IsZero())
}
```

- [ ] **Step 2: Run tests — expect FAIL**

Run: `go test ./x/fees/keeper/ -run TestEndBlocker -v`

Expected: FAIL — `EndBlocker` not defined.

- [ ] **Step 3: Write `x/fees/keeper/abci.go`**

```go
package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/vertix-network/vertix/x/fees/types"
)

// EndBlocker burns BurnRatio of the fee collector's uvtx and leaves the
// remainder for the native x/distribution BeginBlock to pay stakers (spec D1).
// Never panics: any bank error is logged and the block proceeds.
func (k Keeper) EndBlocker(ctx context.Context) error {
	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}
	burnRatio, err := params.BurnRatioDec()
	if err != nil {
		return err
	}
	if burnRatio.IsZero() {
		return nil // native flow distributes 100%; nothing to burn
	}

	feeCollector := k.accountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
	balance := k.bankKeeper.GetBalance(ctx, feeCollector, types.FeeDenom)
	if balance.Amount.IsZero() {
		return nil
	}

	burnAmt := math.LegacyNewDecFromInt(balance.Amount).Mul(burnRatio).TruncateInt()
	if burnAmt.IsZero() {
		return nil // dust: nothing to burn this block
	}
	burnCoins := sdk.NewCoins(sdk.NewCoin(types.FeeDenom, burnAmt))

	if err := k.bankKeeper.SendCoinsFromModuleToModule(
		ctx, authtypes.FeeCollectorName, types.ModuleName, burnCoins,
	); err != nil {
		k.Logger().Error("fees: failed to move burn coins from fee collector", "err", err)
		return nil
	}
	if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, burnCoins); err != nil {
		k.Logger().Error("fees: failed to burn coins", "err", err)
		return nil
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	distributed := balance.Amount.Sub(burnAmt)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeFeeBurned,
		sdk.NewAttribute(types.AttributeKeyAmount, burnCoins.String()),
	))
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeFeeDistributed,
		sdk.NewAttribute(types.AttributeKeyAmount, sdk.NewCoin(types.FeeDenom, distributed).String()),
	))
	return nil
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./x/fees/keeper/ -run TestEndBlocker -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/fees/keeper/abci.go x/fees/keeper/abci_test.go
git commit -m "feat(fees): EndBlock burn-only sweep of uvtx fee collector"
```

---

## Task 13: gRPC query and autocli

**Files:**
- Replace: `x/fees/keeper/grpc_query.go` (overwrite scaffold)
- Modify: `x/fees/module/autocli.go`
- Create: `x/fees/keeper/grpc_query_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/fees/keeper/grpc_query_test.go`:

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/fees/types"
)

func TestParamsQuery(t *testing.T) {
	k, ctx := keeper.FeesKeeper(t, keeper.NewMockBank())
	resp, err := k.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, types.DefaultParams(), resp.Params)
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/fees/keeper/ -run TestParamsQuery -v`

Expected: FAIL — `Params` query signature differs from scaffold.

- [ ] **Step 3: Write `x/fees/keeper/grpc_query.go`**

```go
package keeper

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/vertix-network/vertix/x/fees/types"
)

var _ types.QueryServer = Keeper{}

func (k Keeper) Params(ctx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, err
	}
	return &types.QueryParamsResponse{Params: params}, nil
}
```

- [ ] **Step 4: Update `x/fees/module/autocli.go`**

Ensure the query service exposes `params` and the tx service exposes `update-params`:

```go
package fees

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	"github.com/vertix-network/vertix/x/fees/types"
)

func (am AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: types.Query_serviceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{RpcMethod: "Params", Use: "params", Short: "Query the current fees params"},
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service:              types.Msg_serviceDesc.ServiceName,
			EnhanceCustomCommand: true,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{RpcMethod: "UpdateParams", Skip: true}, // gov-gated; submitted via proposal
			},
		},
	}
}
```

Match the exact generated `Query_serviceDesc` / `Msg_serviceDesc` symbol names produced by `make proto-gen`; adjust if the scaffold used `_Query_serviceDesc`.

- [ ] **Step 5: Run tests — expect PASS**

Run: `go test ./x/fees/keeper/ -run TestParamsQuery -v`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add x/fees/keeper/grpc_query.go x/fees/keeper/grpc_query_test.go x/fees/module/autocli.go
git commit -m "feat(fees): add Params gRPC query and autocli wiring"
```

---

## Task 14: Genesis InitGenesis / ExportGenesis

**Files:**
- Replace: `x/fees/types/genesis.go` (delete Ignite scaffold genesis first)
- Replace: `x/fees/keeper/genesis.go` (overwrite scaffold)
- Create: `x/fees/keeper/genesis_test.go`

- [ ] **Step 1: Write the failing round-trip test**

Create `x/fees/keeper/genesis_test.go`:

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	feeskeeper "github.com/vertix-network/vertix/x/fees/keeper"
	"github.com/vertix-network/vertix/x/fees/types"
)

func TestGenesisRoundTrip(t *testing.T) {
	k, ctx := keeper.FeesKeeper(t, keeper.NewMockBank())
	gs := types.GenesisState{Params: types.FeesParams{BurnRatio: "0.30", DistributionRatio: "0.70"}}

	feeskeeper.InitGenesis(ctx, k, gs)
	exported := feeskeeper.ExportGenesis(ctx, k)
	require.Equal(t, gs.Params, exported.Params)
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/fees/keeper/ -run TestGenesisRoundTrip -v`

Expected: FAIL — `InitGenesis` / `ExportGenesis` signatures differ.

- [ ] **Step 3: Write `x/fees/types/genesis.go`**

```go
package types

func DefaultGenesis() *GenesisState {
	return &GenesisState{Params: DefaultParams()}
}

func (gs GenesisState) Validate() error {
	return gs.Params.Validate()
}
```

- [ ] **Step 4: Write `x/fees/keeper/genesis.go`**

```go
package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/fees/types"
)

func InitGenesis(ctx sdk.Context, k Keeper, gs types.GenesisState) {
	if err := gs.Validate(); err != nil {
		panic(err)
	}
	if err := k.SetParams(ctx, gs.Params); err != nil {
		panic(err)
	}
}

func ExportGenesis(ctx sdk.Context, k Keeper) *types.GenesisState {
	params, err := k.GetParams(ctx)
	if err != nil {
		panic(err)
	}
	return &types.GenesisState{Params: params}
}
```

If the scaffold's `module.go` calls `InitGenesis(ctx, am.keeper, genState)` with a value (not pointer), match that signature; the snippets above take a value `gs` and return a pointer, matching the oracle module's pattern.

- [ ] **Step 5: Run tests — expect PASS**

Run: `go test ./x/fees/keeper/ -run TestGenesisRoundTrip -v`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add x/fees/types/genesis.go x/fees/keeper/genesis.go x/fees/keeper/genesis_test.go
git commit -m "feat(fees): genesis import/export with validation"
```

---

## Task 15: Wire EndBlock into the AppModule + depinject

**Files:**
- Modify: `x/fees/module/module.go` (EndBlock → keeper.EndBlocker; ProvideModule passes account+bank keepers)

- [ ] **Step 1: Point `AppModule.EndBlock` at the keeper**

In `x/fees/module/module.go`, replace the scaffold no-op `EndBlock` with:

```go
// EndBlock burns the fee-collector uvtx fraction each block (spec §9).
func (am AppModule) EndBlock(ctx context.Context) error {
	return am.keeper.EndBlocker(ctx)
}
```

Confirm `var _ appmodule.HasEndBlocker = (*AppModule)(nil)` is present (Ignite adds it). Remove the `HasBeginBlocker` assertion + `BeginBlock` method if the scaffold added them and they are unused (the linter flags unused — keep a no-op `BeginBlock` only if the assertion remains; simplest is to delete both).

- [ ] **Step 2: Confirm `ProvideModule` passes the account + bank keepers**

The scaffold's `ModuleInputs` should already include `AccountKeeper types.AccountKeeper` and `BankKeeper types.BankKeeper`. Ensure `ProvideModule` constructs the keeper as:

```go
k := keeper.NewKeeper(
	in.Cdc,
	in.StoreService,
	in.Logger,
	authority.String(),
	in.AccountKeeper,
	in.BankKeeper,
)
m := NewAppModule(in.Cdc, k, in.AccountKeeper, in.BankKeeper)
return ModuleOutputs{FeesKeeper: k, Module: m}
```

Rename the scaffold's output field to `FeesKeeper` if it differs. The `--dep ...,distribution` flag may have added a `DistKeeper` input; it is unused under D1 — leave the input field if present (depinject tolerates it) but do not pass it to the keeper.

- [ ] **Step 3: Verify build**

Run: `go build ./...`

Expected: success.

- [ ] **Step 4: Commit**

```bash
git add x/fees/module/
git commit -m "feat(fees): wire EndBlock to keeper.EndBlocker and depinject keepers"
```

---

## Task 16: App wiring (endBlockers last, Burner maccPerm) + app tests

**Files:**
- Modify: `app/app_config.go` (verify/insert fees ordering + maccPerms)
- Modify: `app/app.go` (FeesKeeper field + depinject)
- Modify: `app/app_test.go`

- [ ] **Step 1: Verify/adjust `app/app_config.go`**

Confirm the Ignite scaffold inserted `feestypes` and adjust so that:

- `endBlockers` lists `feesmoduletypes.ModuleName` **after** `oraclemoduletypes.ModuleName` (last in the custom suffix — spec §4):

```go
	endBlockers = []string{
		// ...
		// chain modules
		oraclemoduletypes.ModuleName,
		feesmoduletypes.ModuleName,
		// this line is used by starport scaffolding # stargate/app/endBlockers
	}
```

- `genesisModuleOrder` lists `feesmoduletypes.ModuleName` after `oraclemoduletypes.ModuleName`.
- `moduleAccPerms` includes the fees Burner account (Ignite may add it without permissions — set Burner):

```go
		{Account: feesmoduletypes.ModuleName, Permissions: []string{authtypes.Burner}},
```

- A `ModuleConfig` for fees exists at `# stargate/app/moduleConfig`.
- No `beginBlockers` / `preBlockers` entry for fees.

Use the import alias the scaffold generated (likely `feesmoduletypes "github.com/vertix-network/vertix/x/fees/types"`); match it consistently.

- [ ] **Step 2: Confirm `app/app.go` exposes the keeper**

Ensure `FeesKeeper feeskeeper.Keeper` is a field on `App` and is in the `depinject.Inject` output target list.

- [ ] **Step 3: Add app-level tests**

Append to `app/app_test.go`:

```go
func TestFeesModuleWired(t *testing.T) {
	a := newTestApp(t)
	_, ok := a.ModuleManager.Modules["fees"]
	require.True(t, ok, "x/fees must be wired")
}

func TestFeesModuleAccountHasBurner(t *testing.T) {
	a := newTestApp(t)
	acc := a.AccountKeeper.GetModuleAccount(a.NewContext(false), feestypes.ModuleName)
	require.NotNil(t, acc)
	macc, ok := acc.(interface{ GetPermissions() []string })
	require.True(t, ok)
	require.Contains(t, macc.GetPermissions(), authtypes.Burner)
}
```

Match the existing `app_test.go` helpers (`newTestApp`, import aliases). If `newTestApp`/`NewContext` differ, mirror the pattern used by `TestNoMintModule` / the oracle wiring test already in the file.

- [ ] **Step 4: Run app tests**

Run: `go test ./app/ -run 'TestFees|TestNoMint|TestOracle' -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/
git commit -m "feat(app): wire x/fees endBlock (last) with Burner module account"
```

---

## Task 17: Doc-syncs to technical-design.md

**Files:**
- Modify: `docs/technical-design.md` §4.3, §4.6

- [ ] **Step 1: Correct §4.3 EndBlock algorithm**

Replace the `distributionKeeper.AllocateTokensToFeePool(distr coins)` block (no such SDK method; would double-distribute against the native `BeginBlock`) with the burn-only sweep. New §4.3 body:

```
EndBlock(ctx):
    params = GetParams(ctx)
    if params.BurnRatio == 0: return
    feeCollector = accountKeeper.GetModuleAddress(auth.FeeCollectorName)
    uvtx = bankKeeper.GetBalance(feeCollector, "uvtx")   # native token only
    if uvtx == 0: return
    burn = floor(uvtx × params.BurnRatio)
    if burn == 0: return                                  # dust guard
    bankKeeper.SendCoinsFromModuleToModule(FeeCollector → fees, burn)
    bankKeeper.BurnCoins(fees, burn)
    emit EventFeeBurned(amount=burn)
    emit EventFeeDistributed(amount = uvtx − burn)        # left for native x/distribution BeginBlock
# The DistributionRatio (remainder) is distributed to stakers by the
# unmodified x/distribution BeginBlock next block — x/fees does not call it.
```

- [ ] **Step 2: Correct §4.6 invariants**

Replace "After each `EndBlock`, `FeeCollector` balance for `uvtx` is zero (everything was swept)" with:

```
- Total uvtx burned (via EventFeeBurned) is monotonic and ≤ genesis supply − current supply.
- The x/fees module account balance is zero at every block boundary (it holds
  coins only transiently within EndBlock between the move and the burn).
```

Add a one-line note: "x/fees uses a burn-only sweep (spec D1): the DistributionRatio portion stays in the fee collector and is distributed to stakers by the native x/distribution BeginBlock. Set community_tax = 0 in genesis so the full DistributionRatio reaches stakers."

- [ ] **Step 3: Commit**

```bash
git add docs/technical-design.md
git commit -m "docs(fees): sync technical-design §4.3/§4.6 to burn-only sweep"
```

---

## Task 18: Acceptance gate integration tests

**Files:**
- Create: `x/fees/keeper/acceptance_test.go`

- [ ] **Step 1: Write acceptance tests mapping spec §11**

Create `x/fees/keeper/acceptance_test.go`:

```go
package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	feeskeeper "github.com/vertix-network/vertix/x/fees/keeper"
	"github.com/vertix-network/vertix/x/fees/types"
)

// Spec §11: fee burn reduces supply; remainder left for native distribution;
// governance can change ratios and the new split applies next block.
func TestAcceptance_BurnThenGovernanceChange(t *testing.T) {
	bank := keeper.NewMockBank()
	bank.SetModuleBalance(authtypes.FeeCollectorName, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(1000))))
	k, ctx := keeper.FeesKeeper(t, bank)

	// default 40% burn
	require.NoError(t, k.EndBlocker(ctx))
	require.Equal(t, math.NewInt(400), bank.Burned.AmountOf("uvtx"))

	// governance raises burn to 50%
	server := feeskeeper.NewMsgServerImpl(k)
	_, err := server.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: k.GetAuthority(),
		Params:    types.FeesParams{BurnRatio: "0.50", DistributionRatio: "0.50"},
	})
	require.NoError(t, err)

	// next block: collector has the 600 remainder; 50% → burn 300 more (total 700)
	require.NoError(t, k.EndBlocker(ctx))
	require.Equal(t, math.NewInt(700), bank.Burned.AmountOf("uvtx"))
	require.Equal(t, math.NewInt(300), bank.ModuleBalance(authtypes.FeeCollectorName).AmountOf("uvtx"))
}

// Spec §11 / D2: non-uvtx denoms are never burned.
func TestAcceptance_NonUvtxNeverBurned(t *testing.T) {
	bank := keeper.NewMockBank()
	bank.SetModuleBalance(authtypes.FeeCollectorName,
		sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(1000)), sdk.NewCoin("rwa/gold", math.NewInt(999))))
	k, ctx := keeper.FeesKeeper(t, bank)
	require.NoError(t, k.EndBlocker(ctx))
	require.True(t, bank.Burned.AmountOf("rwa/gold").IsZero())
}
```

- [ ] **Step 2: Run full fees test suite**

Run: `go test ./x/fees/... -race -count=1 -v`

Expected: all PASS

- [ ] **Step 3: Commit**

```bash
git add x/fees/keeper/acceptance_test.go
git commit -m "test(fees): Phase 2 acceptance gate integration tests"
```

---

## Task 19: Full CI verification

**Files:** none (verification only)

- [ ] **Step 1: Lint**

Run: `make lint`

Expected: no findings (fix any unused imports, e.g. the `sdk` guard in `keeper.go`).

- [ ] **Step 2: Test**

Run: `make test`

Expected: all packages pass with `-race -count=1`.

- [ ] **Step 3: Build**

Run: `make build`

Expected: `build/vertixd` produced.

- [ ] **Step 4: Proto checks**

Run:

```bash
cd proto && buf lint && buf breaking --against '.git#branch=main'
```

Expected: pass (first fees proto — breaking against main passes since main has no fees proto).

- [ ] **Step 5: Genesis validate**

Run:

```bash
./build/vertixd genesis validate-genesis
```

Expected: valid genesis (fees params present with 0.40/0.60).

- [ ] **Step 6: Manual CLI smoke (optional)**

```bash
./build/vertixd q fees params
```

Expected: returns `burn_ratio: "0.400000000000000000"`, `distribution_ratio: "0.600000000000000000"` — not a crash.

- [ ] **Step 7: Final commit if any fixes needed**

```bash
git commit -am "chore(fees): Phase 2 CI green — lint test build"
```

---

## Spec Coverage Checklist (self-review)

| Spec requirement | Task |
|---|---|
| D8 Ignite scaffold + adapt | Task 1 |
| D1 Burn-only sweep (no distribution call) | Tasks 8, 12 |
| D2 uvtx-only burn | Tasks 3, 12, 18 |
| D4 No custom state beyond params | Tasks 3, 10 |
| D5 Burner module account; FeeCollector→fees→burn | Tasks 12, 16 |
| D6 Burn at EndBlock, last in custom suffix | Tasks 15, 16 |
| D7 Keep both ratio params; sum==1 | Tasks 2, 6 |
| Params + Validate (bounds, sum==1) | Task 6 |
| MsgUpdateParams gov authority | Tasks 7, 11 |
| EndBlock floor/dust/zero/zero-ratio guards | Task 12 |
| Events fee_burned / fee_distributed / fee_params_updated | Tasks 5, 11, 12 |
| Params query + autocli | Task 13 |
| Genesis default + round-trip | Task 14 |
| App wiring (endBlockers last, Burner maccPerm) | Task 16 |
| Doc-sync §4.3 (no AllocateTokensToFeePool) | Task 17 |
| Doc-sync §4.6 (collector not zeroed; module acct zero) | Task 17 |
| Community_tax = 0 cross-phase note (D3) | Task 17 (recorded; Phase 0 owns the genesis value) |
| Acceptance gate §11 | Tasks 18–19 |

**Cross-phase note (D3):** the `community_tax = 0` genesis value is owned by Phase 0, not implemented in `x/fees`. Task 17 records it in `technical-design.md`; verifying/setting the actual genesis value is a Phase 0 follow-up flagged there.
