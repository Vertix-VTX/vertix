# Phase 7 — Security Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `x/oracle`, `x/rwa`, `x/fees` audit-ready: complete crisis invariants, add a simulation layer that exercises them, prove the economic attack surface is closed, document gas bounds, wire CodeQL + a sim CI job, establish a load-test baseline, and ship validator security docs.

**Architecture:** Risk-ordered (Approach A): invariants → simulation → adversarial/property/fuzz → gas audit → CI → load test → docs. Invariants are cheap, bounded, and registered with the already-wired `x/crisis`. The only state additions are two internal `x/fees` KV keys (no proto change). The existing `app/sim_test.go` harness is reused unchanged.

**Tech Stack:** Cosmos SDK v0.50.x, CometBFT v0.38.x, Go 1.22+ (toolchain pinned to `go1.25.4`), `x/simulation`, GitHub `codeql-action`, `tm-load-test`, the Phase 6 `docker compose` devnet.

**Reference spec:** [`docs/specs/2026-05-29-phase-7-security-hardening-design.md`](../specs/2026-05-29-phase-7-security-hardening-design.md).

---

## File Structure

**Workstream 1 — Invariants**
- Create: `x/oracle/keeper/invariants.go`, `x/oracle/keeper/invariants_test.go`
- Modify: `x/oracle/module/module.go` (wire `RegisterInvariants`)
- Modify: `x/fees/types/keys.go` (two new KV keys)
- Modify: `x/fees/types/expected_keepers.go` (add `GetSupply` to `BankKeeper`)
- Modify: `x/fees/keeper/keeper.go` (burn-accounting getters/setters)
- Modify: `x/fees/keeper/genesis.go` (snapshot genesis supply)
- Modify: `x/fees/keeper/abci.go` (increment cumulative burned)
- Create: `x/fees/keeper/invariants.go`, `x/fees/keeper/invariants_test.go`
- Modify: `x/fees/module/module.go` (wire `RegisterInvariants`)
- Create: `app/genesis_order_test.go` (assert bank-before-fees)

**Workstream 2 — Simulation layer**
- Modify: `x/oracle/module/simulation.go`, create `x/oracle/simulation/operations.go`, `x/oracle/simulation/decoder.go`
- Modify: `x/rwa/module/simulation.go`, create `x/rwa/simulation/operations.go`, `x/rwa/simulation/decoder.go`
- Modify: `x/fees/module/simulation.go`, create `x/fees/simulation/decoder.go`
- Modify: `Makefile` (sim targets)

**Workstream 3 — Adversarial / property / fuzz**
- Create: `x/oracle/keeper/adversarial_test.go`, `x/oracle/keeper/aggregate_fuzz_test.go`
- Create: `x/rwa/keeper/adversarial_test.go`
- Create: `x/fees/keeper/determinism_test.go`, `x/fees/types/params_fuzz_test.go`

**Workstream 4 — Gas audit**
- Create: `x/rwa/keeper/gas_bounds_test.go`
- Create: `docs/gas-audit.md`

**Workstream 5 — CI**
- Create: `.github/workflows/codeql.yml`
- Modify: `.github/workflows/ci.yml` (sim job)

**Workstream 6 — Load test**
- Create: `infra/loadtest/README.md`, `infra/loadtest/loadtest.toml`
- Modify: `Makefile` (`load-test` target)
- Create: `docs/load-test.md`

**Workstream 7 — Security docs**
- Create: `docs/tmkms.md`, `docs/validator-setup.md`

**Workstream 8 — Doc-syncs**
- Modify: `docs/full-design-spec.md`, `docs/technical-design.md`, `docs/project-structure.md`, `AGENTS.md`

---

## Workstream 1 — Crisis Invariants

### Task 1: `x/oracle` `prices` invariant

**Files:**
- Create: `x/oracle/keeper/invariants.go`
- Test: `x/oracle/keeper/invariants_test.go`
- Modify: `x/oracle/module/module.go:113-114`

- [ ] **Step 1: Write the failing test**

Create `x/oracle/keeper/invariants_test.go`:

```go
package keeper_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	keepertest "github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/oracle/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestPricesInvariant(t *testing.T) {
	ctx, k := keepertest.OracleKeeper(t, keepertest.NewMockStaking(), keepertest.NewMockSlashing())

	params := types.DefaultParams()
	params.AcceptList = []string{"VTX:USD"}
	require.NoError(t, k.SetParams(ctx, params))

	// Healthy: an accepted, positive price -> not broken.
	require.NoError(t, k.SetAggregatedPrice(ctx, types.AggregatedPrice{
		Pair: "VTX:USD", Price: "1.50", BlockHeight: 1, BlockTime: time.Unix(100, 0),
	}))
	_, broken := keeper.PricesInvariant(k)(ctx)
	require.False(t, broken)

	// Orphan pair not in AcceptList -> broken.
	require.NoError(t, k.SetAggregatedPrice(ctx, types.AggregatedPrice{
		Pair: "ZZZ:USD", Price: "2.00", BlockHeight: 1, BlockTime: time.Unix(100, 0),
	}))
	_, broken = keeper.PricesInvariant(k)(ctx)
	require.True(t, broken)
}

func TestPricesInvariantRejectsNonPositive(t *testing.T) {
	ctx, k := keepertest.OracleKeeper(t, keepertest.NewMockStaking(), keepertest.NewMockSlashing())
	params := types.DefaultParams()
	params.AcceptList = []string{"VTX:USD"}
	require.NoError(t, k.SetParams(ctx, params))

	require.NoError(t, k.SetAggregatedPrice(ctx, types.AggregatedPrice{
		Pair: "VTX:USD", Price: "0", BlockHeight: 1, BlockTime: time.Unix(100, 0),
	}))
	_, broken := keeper.PricesInvariant(k)(ctx)
	require.True(t, broken)
}
```

Note: confirm the constructor signature of `keepertest.OracleKeeper` (current signature returns `(sdk.Context, oraclekeeper.Keeper)` and takes `*MockStaking, *MockSlashing`). If `NewMockStaking`/`NewMockSlashing` helpers don't exist, construct the mocks the same way `x/oracle/keeper/*_test.go` already does and adapt the two `OracleKeeper(...)` calls accordingly.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./x/oracle/keeper/ -run TestPricesInvariant -v`
Expected: FAIL — `undefined: keeper.PricesInvariant`.

- [ ] **Step 3: Write the implementation**

Create `x/oracle/keeper/invariants.go`:

```go
package keeper

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/oracle/types"
)

// RegisterInvariants registers the oracle crisis invariants.
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "prices", PricesInvariant(k))
}

// PricesInvariant: every stored AggregatedPrice is for an accept-listed pair
// and parses to a strictly positive decimal. Existence/freshness of prices is
// a monitoring concern, not a halt-worthy invariant (see Phase 7 design D1).
func PricesInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		broken := false
		params, err := k.GetParams(ctx)
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "prices", "failed to load params: "+err.Error()), true
		}
		accept := make(map[string]struct{}, len(params.AcceptList))
		for _, p := range params.AcceptList {
			accept[p] = struct{}{}
		}

		store := k.storeService.OpenKVStore(ctx)
		iter, err := store.Iterator(types.KeyPrefixAggregatedPrice, storePrefixEnd(types.KeyPrefixAggregatedPrice))
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "prices", "failed to iterate prices: "+err.Error()), true
		}
		defer iter.Close()
		for ; iter.Valid(); iter.Next() {
			var ap types.AggregatedPrice
			k.cdc.MustUnmarshal(iter.Value(), &ap)
			if _, ok := accept[ap.Pair]; !ok {
				broken = true
			}
			price, perr := math.LegacyNewDecFromStr(ap.Price)
			if perr != nil || !price.IsPositive() {
				broken = true
			}
		}
		return sdk.FormatInvariant(types.ModuleName, "prices",
			"every stored aggregated price must be accept-listed and strictly positive"), broken
	}
}

// storePrefixEnd returns the exclusive end key for a prefix scan.
func storePrefixEnd(prefix []byte) []byte {
	end := make([]byte, len(prefix))
	copy(end, prefix)
	for i := len(end) - 1; i >= 0; i-- {
		end[i]++
		if end[i] != 0 {
			return end[:i+1]
		}
	}
	return nil
}
```

Note: if a `storePrefixEnd`/`PrefixEndBytes` helper already exists in the oracle keeper package (check `twap.go`/`keeper.go` for an existing prefix-iteration helper), reuse it instead of redefining to avoid a duplicate-symbol error.

- [ ] **Step 4: Wire `RegisterInvariants` into the module**

In `x/oracle/module/module.go`, replace:

```go
func (am AppModule) RegisterInvariants(_ sdk.InvariantRegistry) {}
```

with:

```go
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
	keeper.RegisterInvariants(ir, am.keeper)
}
```

Confirm `keeper` is already imported in `module.go` (it is — `NewAppModule` references `keeper.Keeper`).

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./x/oracle/keeper/ -run TestPricesInvariant -v`
Expected: PASS (both `TestPricesInvariant` and `TestPricesInvariantRejectsNonPositive`).

- [ ] **Step 6: Commit**

```bash
git add x/oracle/keeper/invariants.go x/oracle/keeper/invariants_test.go x/oracle/module/module.go
git commit -m "feat(oracle): register prices structural-integrity invariant"
```

---

### Task 2: `x/fees` burn-accounting state

**Files:**
- Modify: `x/fees/types/keys.go`
- Modify: `x/fees/types/expected_keepers.go`
- Modify: `x/fees/keeper/keeper.go`
- Modify: `x/fees/keeper/genesis.go:11-18`
- Modify: `x/fees/keeper/abci.go:47-50`
- Test: `x/fees/keeper/burn_accounting_test.go` (create)

- [ ] **Step 1: Add the new store keys**

In `x/fees/types/keys.go`, after `var KeyParams = []byte{0x01}`, add:

```go
var (
	// KeyGenesisSupply stores the uvtx total supply snapshotted at InitGenesis.
	KeyGenesisSupply = []byte{0x02}
	// KeyCumulativeBurned stores the lifetime uvtx burned since the current
	// genesis baseline (re-baselined on export/import).
	KeyCumulativeBurned = []byte{0x03}
)
```

- [ ] **Step 2: Add `GetSupply` to the BankKeeper interface**

In `x/fees/types/expected_keepers.go`, add to the `BankKeeper` interface (the real SDK bank keeper and the test `MockBank` both already implement it):

```go
	GetSupply(ctx context.Context, denom string) sdk.Coin
```

Confirm `context` and `sdk` are imported in that file (they are — existing methods use them).

- [ ] **Step 3: Write the failing test**

Create `x/fees/keeper/burn_accounting_test.go`:

```go
package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	keepertest "github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/fees/types"
)

func TestCumulativeBurnedAccounting(t *testing.T) {
	bank := keepertest.NewMockBank()
	k, ctx := keepertest.FeesKeeper(t, bank)

	// Defaults to zero before any burn.
	got, err := k.GetCumulativeBurned(ctx)
	require.NoError(t, err)
	require.True(t, got.IsZero())

	require.NoError(t, k.AddCumulativeBurned(ctx, math.NewInt(100)))
	require.NoError(t, k.AddCumulativeBurned(ctx, math.NewInt(50)))

	got, err = k.GetCumulativeBurned(ctx)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(150), got)
}

func TestGenesisSupplySnapshot(t *testing.T) {
	bank := keepertest.NewMockBank()
	bank.SetModuleBalance("mint-not-used", sdk.NewCoins()) // no-op to keep import
	k, ctx := keepertest.FeesKeeper(t, bank)

	require.NoError(t, k.SetGenesisSupply(ctx, math.NewInt(21_000_000_000_000)))
	got, err := k.GetGenesisSupply(ctx)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(21_000_000_000_000), got)
	_ = types.FeeDenom
}
```

- [ ] **Step 4: Run to verify it fails**

Run: `go test ./x/fees/keeper/ -run 'TestCumulativeBurnedAccounting|TestGenesisSupplySnapshot' -v`
Expected: FAIL — `undefined: k.GetCumulativeBurned` etc.

- [ ] **Step 5: Implement the getters/setters**

Append to `x/fees/keeper/keeper.go`:

```go
// SetGenesisSupply snapshots the uvtx total supply at the current genesis baseline.
func (k Keeper) SetGenesisSupply(ctx context.Context, amt math.Int) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := amt.Marshal()
	if err != nil {
		return err
	}
	return store.Set(types.KeyGenesisSupply, bz)
}

// GetGenesisSupply returns the snapshotted genesis uvtx supply (zero if unset).
func (k Keeper) GetGenesisSupply(ctx context.Context) (math.Int, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyGenesisSupply)
	if err != nil {
		return math.ZeroInt(), err
	}
	if bz == nil {
		return math.ZeroInt(), nil
	}
	var v math.Int
	if err := v.Unmarshal(bz); err != nil {
		return math.ZeroInt(), err
	}
	return v, nil
}

// GetCumulativeBurned returns lifetime uvtx burned since the genesis baseline.
func (k Keeper) GetCumulativeBurned(ctx context.Context) (math.Int, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyCumulativeBurned)
	if err != nil {
		return math.ZeroInt(), err
	}
	if bz == nil {
		return math.ZeroInt(), nil
	}
	var v math.Int
	if err := v.Unmarshal(bz); err != nil {
		return math.ZeroInt(), err
	}
	return v, nil
}

// AddCumulativeBurned increments the lifetime burned counter by amt.
func (k Keeper) AddCumulativeBurned(ctx context.Context, amt math.Int) error {
	cur, err := k.GetCumulativeBurned(ctx)
	if err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := cur.Add(amt).Marshal()
	if err != nil {
		return err
	}
	return store.Set(types.KeyCumulativeBurned, bz)
}
```

Add `"cosmossdk.io/math"` to the import block of `keeper.go` if not present.

- [ ] **Step 6: Snapshot genesis supply at InitGenesis**

In `x/fees/keeper/genesis.go`, change `InitGenesis` to snapshot the supply after setting params:

```go
func (k Keeper) InitGenesis(ctx sdk.Context, genState types.GenesisState) {
	if err := genState.Validate(); err != nil {
		panic(fmt.Errorf("invalid genesis state: %w", err))
	}
	if err := k.SetParams(ctx, genState.Params); err != nil {
		panic(err)
	}
	// Snapshot the uvtx baseline for the reconcile invariant. Requires bank
	// InitGenesis to have run first (asserted by app/genesis_order_test.go).
	supply := k.bankKeeper.GetSupply(ctx, types.FeeDenom).Amount
	if err := k.SetGenesisSupply(ctx, supply); err != nil {
		panic(err)
	}
}
```

Confirm `k.bankKeeper` is a field on the fees keeper (it is — set in `NewKeeper`).

- [ ] **Step 7: Increment cumulative burned in EndBlocker**

In `x/fees/keeper/abci.go`, immediately after the successful `BurnCoins` call (after line 50, before `sdkCtx := ...`), add:

```go
	if err := k.AddCumulativeBurned(ctx, burnAmt); err != nil {
		k.Logger().Error("fees: failed to record cumulative burn", "err", err)
		return nil
	}
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test ./x/fees/keeper/ -run 'TestCumulativeBurnedAccounting|TestGenesisSupplySnapshot' -v`
Expected: PASS.

- [ ] **Step 9: Run the full fees package to catch interface regressions**

Run: `go test ./x/fees/...`
Expected: PASS (the new `GetSupply` interface method is already satisfied by `MockBank`).

- [ ] **Step 10: Commit**

```bash
git add x/fees/types/keys.go x/fees/types/expected_keepers.go x/fees/keeper/keeper.go x/fees/keeper/genesis.go x/fees/keeper/abci.go x/fees/keeper/burn_accounting_test.go
git commit -m "feat(fees): add burn-accounting state (genesis supply snapshot + cumulative burned)"
```

---

### Task 3: `x/fees` `reconcile` and `module-balance` invariants

**Files:**
- Create: `x/fees/keeper/invariants.go`
- Test: `x/fees/keeper/invariants_test.go`
- Modify: `x/fees/module/module.go:113-114`

- [ ] **Step 1: Write the failing test**

Create `x/fees/keeper/invariants_test.go`:

```go
package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	keepertest "github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/fees/keeper"
	"github.com/vertix-network/vertix/x/fees/types"
)

func TestReconcileInvariant(t *testing.T) {
	bank := keepertest.NewMockBank()
	// Genesis supply = 1000 uvtx in circulation.
	require.NoError(t, bank.MintCoins(ctx0(), "circulating", sdk.NewCoins(sdk.NewCoin(types.FeeDenom, math.NewInt(1000)))))
	k, ctx := keepertest.FeesKeeper(t, bank)
	require.NoError(t, k.SetGenesisSupply(ctx, math.NewInt(1000)))

	// No burns yet, supply unchanged -> healthy.
	_, broken := keeper.ReconcileInvariant(k)(ctx)
	require.False(t, broken)

	// Burn 100: supply drops to 900, cumulative becomes 100 -> still reconciles.
	require.NoError(t, bank.BurnCoins(ctx, "circulating", sdk.NewCoins(sdk.NewCoin(types.FeeDenom, math.NewInt(100)))))
	require.NoError(t, k.AddCumulativeBurned(ctx, math.NewInt(100)))
	_, broken = keeper.ReconcileInvariant(k)(ctx)
	require.False(t, broken)

	// Corrupt: burn supply without recording -> broken (would indicate inflation/leak).
	require.NoError(t, bank.BurnCoins(ctx, "circulating", sdk.NewCoins(sdk.NewCoin(types.FeeDenom, math.NewInt(10)))))
	_, broken = keeper.ReconcileInvariant(k)(ctx)
	require.True(t, broken)
}

func TestModuleBalanceInvariant(t *testing.T) {
	bank := keepertest.NewMockBank()
	k, ctx := keepertest.FeesKeeper(t, bank)

	// Empty module account -> healthy.
	_, broken := keeper.ModuleBalanceInvariant(k)(ctx)
	require.False(t, broken)

	// Stranded coins in the fees module account -> broken.
	bank.SetModuleBalance(types.ModuleName, sdk.NewCoins(sdk.NewCoin(types.FeeDenom, math.NewInt(5))))
	_, broken = keeper.ModuleBalanceInvariant(k)(ctx)
	require.True(t, broken)
}

func ctx0() sdk.Context { return sdk.Context{} }
```

Note: `MockBank.MintCoins`/`BurnCoins` ignore the ctx, so `ctx0()` is a harmless placeholder for the pre-keeper mint; adjust if `MockBank` signatures differ.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./x/fees/keeper/ -run 'TestReconcileInvariant|TestModuleBalanceInvariant' -v`
Expected: FAIL — `undefined: keeper.ReconcileInvariant`.

- [ ] **Step 3: Implement the invariants**

Create `x/fees/keeper/invariants.go`:

```go
package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/fees/types"
)

// RegisterInvariants registers the fees crisis invariants.
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "reconcile", ReconcileInvariant(k))
	ir.RegisterRoute(types.ModuleName, "module-balance", ModuleBalanceInvariant(k))
}

// ReconcileInvariant: genesisSupply - currentSupply(uvtx) == cumulativeBurned.
// Since x/fees burning is the only uvtx sink (no x/mint; rwa/* are distinct
// denoms), this also enforces the 21M hard cap / no-inflation invariant.
func ReconcileInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		genSupply, err := k.GetGenesisSupply(ctx)
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "reconcile", "failed to load genesis supply: "+err.Error()), true
		}
		burned, err := k.GetCumulativeBurned(ctx)
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "reconcile", "failed to load cumulative burned: "+err.Error()), true
		}
		current := k.bankKeeper.GetSupply(ctx, types.FeeDenom).Amount
		broken := !genSupply.Sub(current).Equal(burned)
		return sdk.FormatInvariant(types.ModuleName, "reconcile",
			"genesis uvtx supply minus current supply must equal cumulative burned (21M cap)"), broken
	}
}

// ModuleBalanceInvariant: the x/fees module account holds zero uvtx at the
// block boundary (it holds coins only transiently inside EndBlocker).
func ModuleBalanceInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		addr := k.accountKeeper.GetModuleAddress(types.ModuleName)
		bal := k.bankKeeper.GetBalance(ctx, addr, types.FeeDenom).Amount
		broken := !bal.IsZero()
		return sdk.FormatInvariant(types.ModuleName, "module-balance",
			"x/fees module account uvtx balance must be zero at the block boundary"), broken
	}
}
```

Confirm `k.accountKeeper` is a field on the fees keeper (it is — `NewKeeper` sets `accountKeeper`) and exposes `GetModuleAddress` (it does — `AccountKeeper` interface line 11).

- [ ] **Step 4: Wire into the module**

In `x/fees/module/module.go`, replace the no-op `RegisterInvariants` with:

```go
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
	keeper.RegisterInvariants(ir, am.keeper)
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./x/fees/keeper/ -run 'TestReconcileInvariant|TestModuleBalanceInvariant' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add x/fees/keeper/invariants.go x/fees/keeper/invariants_test.go x/fees/module/module.go
git commit -m "feat(fees): register reconcile + module-balance crisis invariants"
```

---

### Task 4: Assert bank-before-fees genesis ordering

**Files:**
- Create: `app/genesis_order_test.go`

- [ ] **Step 1: Write the test**

Create `app/genesis_order_test.go`:

```go
package app_test

import (
	"testing"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/app"
	feestypes "github.com/vertix-network/vertix/x/fees/types"
)

// The fees reconcile invariant snapshots the uvtx supply at InitGenesis, so
// bank's InitGenesis must run before fees'.
func TestBankInitsBeforeFees(t *testing.T) {
	a := app.Setup(t, false)
	order := a.ModuleManager.OrderInitGenesis

	bankIdx, feesIdx := -1, -1
	for i, name := range order {
		switch name {
		case banktypes.ModuleName:
			bankIdx = i
		case feestypes.ModuleName:
			feesIdx = i
		}
	}
	require.NotEqual(t, -1, bankIdx, "bank not in OrderInitGenesis")
	require.NotEqual(t, -1, feesIdx, "fees not in OrderInitGenesis")
	require.Less(t, bankIdx, feesIdx, "bank must init before fees")
}
```

Note: confirm the test app constructor. Check `app/testing.go` / `app/app_test.go` for the existing helper (e.g. `app.Setup(t, false)` or `app.New(...)`); use whichever the existing tests use and confirm `ModuleManager.OrderInitGenesis` is exported (it is on `*module.Manager`).

- [ ] **Step 2: Run to verify it passes (this is a guard, expected green now)**

Run: `go test ./app/ -run TestBankInitsBeforeFees -v`
Expected: PASS (ordering already holds in `app/app_config.go`). If it FAILS, the genesis order regressed — fix `genesisModuleOrder` so `banktypes` precedes `feesmoduletypes`.

- [ ] **Step 3: Commit**

```bash
git add app/genesis_order_test.go
git commit -m "test(app): assert bank InitGenesis precedes fees (reconcile invariant dep)"
```

---

## Workstream 2 — Simulation Layer

### Task 5: `x/oracle` simulation (genesis + ops + decoder)

**Files:**
- Modify: `x/oracle/module/simulation.go`
- Create: `x/oracle/simulation/operations.go`
- Create: `x/oracle/simulation/decoder.go`
- Test: `x/oracle/simulation/decoder_test.go`

- [ ] **Step 1: Write the failing decoder test**

Create `x/oracle/simulation/decoder_test.go`:

```go
package simulation_test

import (
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/types/kv"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/app"
	"github.com/vertix-network/vertix/x/oracle/simulation"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestDecodeStore(t *testing.T) {
	cdc := app.MakeEncodingConfig().Codec
	dec := simulation.NewDecodeStore(cdc)

	ap := types.AggregatedPrice{Pair: "VTX:USD", Price: "1.50", BlockHeight: 1, BlockTime: time.Unix(1, 0)}
	bz := cdc.MustMarshal(&ap)

	pairs := kv.Pair{Key: types.AggregatedPriceKey("VTX:USD"), Value: bz}
	out := dec(pairs, pairs)
	require.Contains(t, out, "VTX:USD")
}
```

Note: confirm how this repo exposes an app codec for tests (search for `MakeEncodingConfig` or an equivalent in `app/`). If it differs, build a `codec.ProtoCodec` from a fresh `codectypes.NewInterfaceRegistry()` the same way `testutil/keeper/oracle.go` does.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./x/oracle/simulation/ -run TestDecodeStore -v`
Expected: FAIL — `undefined: simulation.NewDecodeStore`.

- [ ] **Step 3: Implement the store decoder**

Create `x/oracle/simulation/decoder.go`:

```go
package simulation

import (
	"bytes"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types/kv"

	"github.com/vertix-network/vertix/x/oracle/types"
)

// NewDecodeStore returns a decoder that pretty-prints oracle KV pairs for the
// simulation diff output.
func NewDecodeStore(cdc codec.Codec) func(kvA, kvB kv.Pair) string {
	return func(kvA, kvB kv.Pair) string {
		switch {
		case bytes.HasPrefix(kvA.Key, types.KeyPrefixAggregatedPrice):
			var a, b types.AggregatedPrice
			cdc.MustUnmarshal(kvA.Value, &a)
			cdc.MustUnmarshal(kvB.Value, &b)
			return fmt.Sprintf("%v\n%v", a, b)
		case bytes.HasPrefix(kvA.Key, types.KeyPrefixFeed):
			var a, b types.OracleFeed
			cdc.MustUnmarshal(kvA.Value, &a)
			cdc.MustUnmarshal(kvB.Value, &b)
			return fmt.Sprintf("%v\n%v", a, b)
		default:
			return fmt.Sprintf("unrecognized oracle key %X\n%X", kvA.Key, kvB.Key)
		}
	}
}
```

Confirm `types.OracleFeed` is the stored feed type (grep `type OracleFeed` in `x/oracle/types`); if the stored value under `KeyPrefixFeed` is a different type, decode that type instead.

- [ ] **Step 4: Implement weighted operations**

Create `x/oracle/simulation/operations.go`:

```go
package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	"github.com/vertix-network/vertix/x/oracle/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

const (
	OpWeightMsgSetFeeder  = "op_weight_msg_set_feeder"
	OpWeightMsgSubmitFeed = "op_weight_msg_submit_feed"

	defaultWeightMsgSetFeeder  = 30
	defaultWeightMsgSubmitFeed = 80
)

// WeightedOperations returns oracle simulation operations driving feeder
// delegation and price submission against accept-listed pairs.
func WeightedOperations(
	appParams simtypes.AppParams,
	txGen interface{ /* client.TxConfig */ },
	k keeper.Keeper,
) simulation.WeightedOperations {
	// Placeholder gate: real wiring is done in the module's WeightedOperations
	// (Step 5), which has the concrete TxConfig and account keepers.
	return nil
}

// SimulateMsgSubmitFeed builds a feed submission for a random accept-listed pair.
func SimulateMsgSubmitFeed(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		params, err := k.GetParams(ctx)
		if err != nil || len(params.AcceptList) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, "MsgSubmitFeed", "no accept-listed pairs"), nil, nil
		}
		pair := params.AcceptList[r.Intn(len(params.AcceptList))]
		price := simtypes.RandomDecAmount(r, sdk.NewDec(1000)).Add(sdk.OneDec()) // strictly positive
		_ = pair
		_ = price
		// Concrete account/validator selection + delivery is finalized in the
		// module WeightedOperations (Step 5) which owns the TxConfig.
		return simtypes.NoOpMsg(types.ModuleName, "MsgSubmitFeed", "delegated to module wiring"), nil, nil
	}
}
```

Note: the SDK's recommended pattern wires ops in the module's `WeightedOperations` because that method receives the `module.SimulationState` (and thus the codec/tx config). Keep the heavy lifting in Step 5 and use this file only for shared helpers/constants. Simplify this file to just the constants + `SimulateMsgSubmitFeed`/`SimulateMsgSetFeeder` signatures if the no-op stubs cause unused-import lint errors.

- [ ] **Step 5: Implement module genesis + WeightedOperations**

Replace the body of `x/oracle/module/simulation.go`'s `GenerateGenesisState` and `WeightedOperations`:

```go
// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	r := simState.Rand
	params := types.DefaultParams()
	// Small random accept-list so price invariants are non-vacuous.
	pairs := []string{"VTX:USD", "ATOM:USD", "OSMO:USD"}
	n := 1 + r.Intn(len(pairs))
	params.AcceptList = pairs[:n]
	oracleGenesis := types.GenesisState{Params: params}
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&oracleGenesis)
}

// WeightedOperations returns the module operations with their weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	var weightSubmit, weightSetFeeder int
	simState.AppParams.GetOrGenerate(oraclesimulation.OpWeightMsgSubmitFeed, &weightSubmit, nil,
		func(_ *rand.Rand) { weightSubmit = oraclesimulation.DefaultWeightMsgSubmitFeed })
	simState.AppParams.GetOrGenerate(oraclesimulation.OpWeightMsgSetFeeder, &weightSetFeeder, nil,
		func(_ *rand.Rand) { weightSetFeeder = oraclesimulation.DefaultWeightMsgSetFeeder })

	return []simtypes.WeightedOperation{
		simulation.NewWeightedOperation(weightSubmit, oraclesimulation.SimulateMsgSubmitFeed(am.keeper)),
		simulation.NewWeightedOperation(weightSetFeeder, oraclesimulation.SimulateMsgSetFeeder(am.keeper)),
	}
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(sdr simtypes.StoreDecoderRegistry) {
	sdr[types.StoreKey] = oraclesimulation.NewDecodeStore(am.cdc)
}
```

Then in `x/oracle/simulation/operations.go` export the weight constants as `DefaultWeightMsgSubmitFeed`/`DefaultWeightMsgSetFeeder` (capitalize), and implement `SimulateMsgSetFeeder(k)` and `SimulateMsgSubmitFeed(k)` returning real `simtypes.Operation`s: pick a random validator from `stakingKeeper` (via the keeper if accessible) or fall back to `simtypes.RandomAcc`, build `types.MsgSetFeeder{Validator, Feeder}` / `types.MsgSubmitFeed{Feeder, Validator, Pair, Price}`, and deliver via the standard `simulation.GenAndDeliverTxWithRandFees` helper using `am.cdc`/tx config. Where a precondition isn't met (no validators, no accept-list), return `simtypes.NoOpMsg(...)`.

Confirm `am.cdc` exists on the oracle `AppModule` (it does — `NewAppModuleBasic(cdc)`); add `oraclesimulation` and `rand`/`simulation` imports. Remove the `var _ = ...` unused-import block now that the symbols are used.

- [ ] **Step 6: Run the decoder test + build**

Run: `go test ./x/oracle/simulation/ -run TestDecodeStore -v && go build ./...`
Expected: decoder test PASS; build succeeds.

- [ ] **Step 7: Commit**

```bash
git add x/oracle/module/simulation.go x/oracle/simulation/operations.go x/oracle/simulation/decoder.go x/oracle/simulation/decoder_test.go
git commit -m "feat(oracle): randomized sim genesis, weighted ops, store decoder"
```

---

### Task 6: `x/rwa` simulation (genesis + ops + decoder)

**Files:**
- Modify: `x/rwa/module/simulation.go`
- Create: `x/rwa/simulation/operations.go`
- Create: `x/rwa/simulation/decoder.go`
- Test: `x/rwa/simulation/decoder_test.go`

- [ ] **Step 1: Write the failing decoder test**

Create `x/rwa/simulation/decoder_test.go`:

```go
package simulation_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/types/kv"
	"github.com/stretchr/testify/require"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/vertix-network/vertix/x/rwa/simulation"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestDecodeStore(t *testing.T) {
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	dec := simulation.NewDecodeStore(cdc)

	rec := types.AssetRecord{AssetId: "asset-1", Denom: "rwa/asset-1"}
	bz := cdc.MustMarshal(&rec)
	pair := kv.Pair{Key: types.AssetKey("asset-1"), Value: bz}
	require.Contains(t, dec(pair, pair), "asset-1")
}
```

Note: confirm the asset record type name (`types.AssetRecord`) and its key constructor (grep `func AssetKey` / the asset key prefix in `x/rwa/types/keys.go`); adapt the key builder and any required fields (`Status`, `Bond`, etc.) to satisfy marshaling.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./x/rwa/simulation/ -run TestDecodeStore -v`
Expected: FAIL — `undefined: simulation.NewDecodeStore`.

- [ ] **Step 3: Implement the decoder**

Create `x/rwa/simulation/decoder.go`:

```go
package simulation

import (
	"bytes"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types/kv"

	"github.com/vertix-network/vertix/x/rwa/types"
)

// NewDecodeStore pretty-prints rwa KV pairs for simulation diff output.
func NewDecodeStore(cdc codec.Codec) func(kvA, kvB kv.Pair) string {
	return func(kvA, kvB kv.Pair) string {
		if bytes.HasPrefix(kvA.Key, types.KeyPrefixAsset) {
			var a, b types.AssetRecord
			cdc.MustUnmarshal(kvA.Value, &a)
			cdc.MustUnmarshal(kvB.Value, &b)
			return fmt.Sprintf("%v\n%v", a, b)
		}
		return fmt.Sprintf("unrecognized rwa key %X\n%X", kvA.Key, kvB.Key)
	}
}
```

Confirm the asset prefix constant name in `x/rwa/types/keys.go` (e.g. `KeyPrefixAsset` / `AssetKeyPrefix`); use the real name.

- [ ] **Step 4: Implement genesis + weighted ops in the module**

In `x/rwa/module/simulation.go`, implement a randomized `GenerateGenesisState` (random `MinIssuerBond`, `MintFeeRate` within validated bounds via `types.DefaultParams()` then perturb) and a `WeightedOperations` returning a single lifecycle operation `SimulateMsgRegisterAsset(am.keeper, am.bankKeeper)` plus optional follow-ups. Create `x/rwa/simulation/operations.go` with:

```go
package simulation

import (
	"fmt"
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/vertix-network/vertix/x/rwa/keeper"
	"github.com/vertix-network/vertix/x/rwa/types"
)

const (
	OpWeightMsgRegisterAsset       = "op_weight_msg_register_asset"
	DefaultWeightMsgRegisterAsset  = 50
)

// SimulateMsgRegisterAsset registers a randomized asset with a valid bond.
func SimulateMsgRegisterAsset(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		issuer, _ := simtypes.RandomAcc(r, accs)
		params, err := k.GetParams(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, "MsgRegisterAsset", "params"), nil, nil
		}
		_ = params
		assetID := fmt.Sprintf("sim-%d", r.Int63())
		msg := &types.MsgRegisterAsset{
			Issuer:     issuer.Address.String(),
			AssetId:    assetID,
			Name:       "sim asset",
			OraclePair: "VTX:USD",
			Bond:       params.MinIssuerBond, // valid by construction
		}
		_ = msg
		// Deliver via the standard sim tx helper (GenAndDeliverTxWithRandFees)
		// using the module tx config; on failed precondition return NoOpMsg.
		return simtypes.NoOpMsg(types.ModuleName, "MsgRegisterAsset", "delegated to module delivery"), nil, nil
	}
}
```

Then finalize delivery in the module's `WeightedOperations` using `simulation.GenAndDeliverTxWithRandFees` (mirror an SDK example such as `x/bank` simulation), funding the issuer's bond from `simtypes` spendable coins. Confirm `MinIssuerBond` is a string field on `RwaParams` (grep `type RwaParams`/`Params` in `x/rwa/types`); if it's a different type, format accordingly. Implement `RegisterStoreDecoder` to register `NewDecodeStore(am.cdc)` under `types.StoreKey`.

- [ ] **Step 5: Run the decoder test + build**

Run: `go test ./x/rwa/simulation/ -run TestDecodeStore -v && go build ./...`
Expected: decoder test PASS; build succeeds.

- [ ] **Step 6: Commit**

```bash
git add x/rwa/module/simulation.go x/rwa/simulation/operations.go x/rwa/simulation/decoder.go x/rwa/simulation/decoder_test.go
git commit -m "feat(rwa): randomized sim genesis, lifecycle weighted op, store decoder"
```

---

### Task 7: `x/fees` simulation (genesis + decoder, no ops)

**Files:**
- Modify: `x/fees/module/simulation.go`
- Create: `x/fees/simulation/decoder.go`
- Test: `x/fees/simulation/decoder_test.go`

- [ ] **Step 1: Write the failing decoder test**

Create `x/fees/simulation/decoder_test.go`:

```go
package simulation_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/kv"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/fees/simulation"
	"github.com/vertix-network/vertix/x/fees/types"
)

func TestDecodeStore(t *testing.T) {
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	dec := simulation.NewDecodeStore(cdc)

	params := types.DefaultParams()
	bz := cdc.MustMarshal(&params)
	pair := kv.Pair{Key: types.KeyParams, Value: bz}
	require.Contains(t, dec(pair, pair), "burn")
}
```

Note: `require.Contains(..., "burn")` assumes the params proto stringifies with a burn-ratio field; if not, assert on a field name that does appear, or just `require.NotEmpty(t, dec(pair, pair))`.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./x/fees/simulation/ -run TestDecodeStore -v`
Expected: FAIL — `undefined: simulation.NewDecodeStore`.

- [ ] **Step 3: Implement the decoder**

Create `x/fees/simulation/decoder.go`:

```go
package simulation

import (
	"bytes"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types/kv"

	"github.com/vertix-network/vertix/x/fees/types"
)

// NewDecodeStore pretty-prints fees KV pairs for simulation diff output.
func NewDecodeStore(cdc codec.Codec) func(kvA, kvB kv.Pair) string {
	return func(kvA, kvB kv.Pair) string {
		switch {
		case bytes.Equal(kvA.Key, types.KeyParams):
			var a, b types.FeesParams
			cdc.MustUnmarshal(kvA.Value, &a)
			cdc.MustUnmarshal(kvB.Value, &b)
			return fmt.Sprintf("%v\n%v", a, b)
		case bytes.Equal(kvA.Key, types.KeyGenesisSupply) || bytes.Equal(kvA.Key, types.KeyCumulativeBurned):
			return fmt.Sprintf("int %X\n%X", kvA.Value, kvB.Value)
		default:
			return fmt.Sprintf("unrecognized fees key %X\n%X", kvA.Key, kvB.Key)
		}
	}
}
```

- [ ] **Step 4: Randomize genesis + register decoder in the module**

In `x/fees/module/simulation.go`:
- `GenerateGenesisState`: build a randomized `burn_ratio` in `[0,1]` and set `distribution_ratio = 1 - burn_ratio` (use the field names/types from `types.DefaultParams()`; keep the ratio invariant `burn + distribution == 1`). Marshal into the fees `GenesisState`.
- `RegisterStoreDecoder`: `sdr[types.StoreKey] = feessimulation.NewDecodeStore(am.cdc)`.
- `WeightedOperations`: leave returning an empty slice (fees is EndBlock-only). Keep the function present.

Confirm `am.cdc` exists on the fees `AppModule`; add the `feessimulation` import.

- [ ] **Step 5: Run the decoder test + build**

Run: `go test ./x/fees/simulation/ -run TestDecodeStore -v && go build ./...`
Expected: PASS; build succeeds.

- [ ] **Step 6: Commit**

```bash
git add x/fees/module/simulation.go x/fees/simulation/decoder.go x/fees/simulation/decoder_test.go
git commit -m "feat(fees): randomized sim genesis (burn ratio) + store decoder"
```

---

### Task 8: Makefile sim targets + full-app simulation run

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add sim targets**

In `Makefile`, after the `bench:` target (line 45) and before `.PHONY: test ...` (line 49), add:

```make
###################
###  Simulation ###
###################

SIM_NUM_BLOCKS ?= 50
SIM_BLOCK_SIZE ?= 50
SIM_SEED ?= 42

test-sim-nondeterminism:
	@echo "Running non-determinism simulation..."
	@go test -mod=readonly ./app -run TestAppStateDeterminism -Enabled=true \
		-NumBlocks=$(SIM_NUM_BLOCKS) -BlockSize=$(SIM_BLOCK_SIZE) -Commit=true -Period=1 -v -timeout 30m

test-sim-fullapp:
	@echo "Running full-app simulation..."
	@go test -mod=readonly ./app -run TestFullAppSimulation -Enabled=true \
		-NumBlocks=$(SIM_NUM_BLOCKS) -BlockSize=$(SIM_BLOCK_SIZE) -Commit=true -Seed=$(SIM_SEED) -Period=1 -v -timeout 30m

test-sim-import-export:
	@echo "Running import/export simulation..."
	@go test -mod=readonly ./app -run TestAppImportExport -Enabled=true \
		-NumBlocks=$(SIM_NUM_BLOCKS) -BlockSize=$(SIM_BLOCK_SIZE) -Commit=true -Seed=$(SIM_SEED) -Period=1 -v -timeout 30m

.PHONY: test-sim-nondeterminism test-sim-fullapp test-sim-import-export
```

Confirm the flag names by inspecting `simcli.GetSimulatorFlags()` usage in `app/sim_test.go` (the SDK flags are `-Enabled`, `-NumBlocks`, `-BlockSize`, `-Commit`, `-Seed`, `-Period`). Confirm the test names: `TestFullAppSimulation`, `TestAppImportExport`, `TestAppStateDeterminism` (verified present in `app/sim_test.go`).

- [ ] **Step 2: Run the full-app simulation (the headline acceptance gate)**

Run: `make test-sim-fullapp SIM_NUM_BLOCKS=30 SIM_BLOCK_SIZE=25`
Expected: PASS — simulation completes with **zero invariant violations**. If an invariant breaks, the failure log (via the store decoders) identifies the offending module/key; fix the invariant or the sim op that produced invalid state.

- [ ] **Step 3: Run import/export + non-determinism**

Run: `make test-sim-import-export SIM_NUM_BLOCKS=30 && make test-sim-nondeterminism SIM_NUM_BLOCKS=30`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add Makefile
git commit -m "build: add simulation make targets (fullapp, import-export, nondeterminism)"
```

---

## Workstream 3 — Adversarial, Property & Fuzz Tests

### Task 9: Oracle manipulation adversarial + fuzz tests

**Files:**
- Create: `x/oracle/keeper/adversarial_test.go`
- Create: `x/oracle/keeper/aggregate_fuzz_test.go`

- [ ] **Step 1: Write the adversarial tests**

Create `x/oracle/keeper/adversarial_test.go` with table-driven tests that reuse the existing oracle keeper test harness (mirror `x/oracle/keeper/endblock_outlier_test.go` / `endblock_aggregate_test.go` setup). Cover:
- A single high-stake validator submitting an extreme price does not move the **unweighted-median** outlier reference enough to invert the outlier decision (assert the outlier slash fires on the extreme submitter, not the honest majority).
- A feeder submitting right at the `MissThreshold`/`QuorumFraction` boundary is accounted correctly (assert miss counter / slash behavior at the edge).
- `GetAggregatedPrice` for a price older than `MaxPriceAge` returns `types.ErrStalePrice` (verified path in `keeper.go:68`).

Use the exact aggregation/EndBlocker entrypoints already under test in the package; assert on `GetMissCounter`, slash calls (via the mock slashing keeper), and returned errors.

- [ ] **Step 2: Run the adversarial tests**

Run: `go test ./x/oracle/keeper/ -run Adversarial -v`
Expected: PASS.

- [ ] **Step 3: Write the fuzz test**

Create `x/oracle/keeper/aggregate_fuzz_test.go`:

```go
package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
)

// FuzzMedianWithinRange asserts the aggregation median always lands within
// [min, max] of the inputs and never panics.
func FuzzMedianWithinRange(f *testing.F) {
	f.Add(int64(1), int64(2), int64(3))
	f.Fuzz(func(t *testing.T, a, b, c int64) {
		prices := []math.LegacyDec{
			math.LegacyNewDec(a), math.LegacyNewDec(b), math.LegacyNewDec(c),
		}
		// Call the package's exported median/aggregation helper over `prices`.
		// Replace `median(prices)` with the real exported function name from
		// x/oracle/keeper/aggregate.go (grep for the median/aggregate helper).
		_ = prices
	})
}
```

Note: wire the fuzz body to the real aggregation helper in `aggregate.go` (grep for the median function; it may be unexported — if so, add a small exported test shim or move the fuzz into `package keeper`). Assert `min <= result <= max`.

- [ ] **Step 4: Run the fuzz test (short)**

Run: `go test ./x/oracle/keeper/ -run FuzzMedianWithinRange -v` then `go test ./x/oracle/keeper/ -fuzz FuzzMedianWithinRange -fuzztime 20s`
Expected: PASS; no new corpus failures.

- [ ] **Step 5: Commit**

```bash
git add x/oracle/keeper/adversarial_test.go x/oracle/keeper/aggregate_fuzz_test.go
git commit -m "test(oracle): adversarial manipulation + median fuzz coverage"
```

---

### Task 10: RWA bond-bypass adversarial tests

**Files:**
- Create: `x/rwa/keeper/adversarial_test.go`

- [ ] **Step 1: Write the tests**

Create `x/rwa/keeper/adversarial_test.go` reusing the existing rwa keeper test harness (`testutil/keeper.RWAKeeper`/`RwaKeeper`). Cover:
- Mint / transition to `ACTIVE` with a bond below `MinIssuerBond` is rejected with the bond error.
- `SettleRWA` cannot reclaim the bond while active supply / obligations remain (assert error and that bond stays escrowed).
- A restricted-asset transfer is blocked by the `SendRestrictionFn` even when wrapped — directly invoke `k.SendRestriction(ctx, from, to, coins)` for a denied address and assert it errors; add a comment referencing that `authz.MsgExec`, `MsgMultiSend`, and IBC escrow all funnel through `BankKeeper.SendCoins` (Phase 3 D5), so the unit-level `SendRestriction` assertion covers all wrappers.

Mirror the assertion style and constructors used in the existing `x/rwa/keeper/*_test.go` files; confirm the `SendRestriction` method name (grep `func (k Keeper) SendRestriction`).

- [ ] **Step 2: Run the tests**

Run: `go test ./x/rwa/keeper/ -run Adversarial -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add x/rwa/keeper/adversarial_test.go
git commit -m "test(rwa): bond-bypass + send-restriction adversarial coverage"
```

---

### Task 11: Fees determinism + ratio fuzz tests

**Files:**
- Create: `x/fees/keeper/determinism_test.go`
- Create: `x/fees/types/params_fuzz_test.go`

- [ ] **Step 1: Write the determinism test**

Create `x/fees/keeper/determinism_test.go` reusing `testutil/keeper.FeesKeeper`. Cover:
- With the same fee-collector balance and `burn_ratio`, two `EndBlocker` runs burn the identical amount and leave the fees module account at zero (assert `bank.Burned` equal across runs and `ModuleBalance(types.ModuleName)` is empty).
- After `EndBlocker`, `GetCumulativeBurned` increased by exactly the burned amount and `ReconcileInvariant` is not broken.

Mirror `x/fees/keeper/abci_test.go` setup.

- [ ] **Step 2: Run it**

Run: `go test ./x/fees/keeper/ -run Determinism -v`
Expected: PASS.

- [ ] **Step 3: Write the params fuzz test**

Create `x/fees/types/params_fuzz_test.go`:

```go
package types_test

import (
	"testing"

	"github.com/vertix-network/vertix/x/fees/types"
)

// FuzzBurnRatioSplit asserts that for any valid burn ratio, burn + distribution
// reconstructs the whole and Validate accepts it.
func FuzzBurnRatioSplit(f *testing.F) {
	f.Add(uint64(40))
	f.Fuzz(func(t *testing.T, pct uint64) {
		p := pct % 101 // 0..100
		params := types.DefaultParams()
		// Set burn_ratio = p/100 and distribution_ratio = (100-p)/100 using the
		// real param field setters/format (grep DefaultParams in x/fees/types).
		_ = params
		_ = p
		// require.NoError(t, params.Validate())
	})
}
```

Note: complete the body against the real `FeesParams` field names/types (e.g. `BurnRatio`/`DistributionRatio` as `LegacyDec` strings); assert `params.Validate()` passes and `burn + distribution == 1`.

- [ ] **Step 4: Run the fuzz test (short)**

Run: `go test ./x/fees/types/ -run FuzzBurnRatioSplit -v && go test ./x/fees/types/ -fuzz FuzzBurnRatioSplit -fuzztime 15s`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add x/fees/keeper/determinism_test.go x/fees/types/params_fuzz_test.go
git commit -m "test(fees): EndBlock determinism + burn-ratio fuzz coverage"
```

---

## Workstream 4 — Gas / Unbounded-Loop Audit

### Task 12: Gas-bound guard test + audit notes

**Files:**
- Create: `x/rwa/keeper/gas_bounds_test.go`
- Create: `docs/gas-audit.md`

- [ ] **Step 1: Write the gas-bound guard test**

Create `x/rwa/keeper/gas_bounds_test.go` that asserts transfer-restriction membership checks are O(1) and independent of list size:
- Build an asset with `allow_all=false` and add N allow-list members (e.g. 1 and 1000).
- Measure `ctx.GasMeter().GasConsumed()` deltas around a single `SendRestriction`/membership `Has` check at N=1 vs N=1000.
- Assert the per-check gas delta does not grow with N (allow a small constant tolerance).

Use the existing rwa keeper harness; confirm the allow/deny add method names (grep the `UpdateRestrictions` keeper path / `0x03`/`0x04` membership writers).

- [ ] **Step 2: Run it**

Run: `go test ./x/rwa/keeper/ -run GasBounds -v`
Expected: PASS.

- [ ] **Step 3: Write the audit notes**

Create `docs/gas-audit.md` with a table of every hot-path iteration site and its bound:

```markdown
# Vertix Gas / Unbounded-Loop Audit (Phase 7)

Each iteration site in the three custom modules, its bound, worst-case input,
and why it is acceptable for launch.

| Module | Site | Iterates over | Bound | Notes |
|---|---|---|---|---|
| oracle | EndBlocker aggregation (`abci.go`) | feeds for active validators × accept-list | O(V × P) | V = active set (≤ ~150), P = accept-list (small, gov-controlled) |
| oracle | miss accounting (`abci.go`) | active validators | O(V) | per-window reset bounded |
| oracle | `prices` invariant | stored aggregated prices | O(P) | crisis check; bounded by accept-list |
| rwa | `bonds`/`denoms` invariants | asset records | O(A) | A = registered assets; crisis check (off hot path) |
| rwa | transfer restriction check | single `Has` lookup | O(1) | keyed membership (Phase 3 D4), not list enumeration; guarded by gas_bounds_test.go |
| fees | EndBlocker burn | none | O(1) | single balance read + burn |
| fees | reconcile / module-balance invariants | none | O(1) | two reads |

**Conclusion:** No unbounded enumeration on any message hot path. Validator-set
and accept-list bounds are governance-controlled and small. Invariants iterate
only bounded prefixes and run off the per-tx hot path.
```

Fill the table against the actual code (adjust site names/files after grepping each module's `abci.go` and keeper iteration helpers).

- [ ] **Step 4: Commit**

```bash
git add x/rwa/keeper/gas_bounds_test.go docs/gas-audit.md
git commit -m "test(rwa): O(1) restriction-check gas guard; docs: gas audit notes"
```

---

## Workstream 5 — CI Integration

### Task 13: CodeQL workflow + sim CI job

**Files:**
- Create: `.github/workflows/codeql.yml`
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Create the CodeQL workflow**

Create `.github/workflows/codeql.yml`:

```yaml
name: CodeQL

on:
  pull_request:
  push:
    branches: [main]
  schedule:
    - cron: "0 6 * * 1"

jobs:
  analyze:
    runs-on: ubuntu-latest
    permissions:
      actions: read
      contents: read
      security-events: write
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25.4"
          cache: true
      - name: Initialize CodeQL
        uses: github/codeql-action/init@v3
        with:
          languages: go
          queries: security-extended
      - name: Build
        run: make build
      - name: Perform CodeQL Analysis
        uses: github/codeql-action/analyze@v3
        with:
          category: "/language:go"
```

This is advisory — it uploads alerts to the Security tab and does not block merges.

- [ ] **Step 2: Add the sim job to ci.yml**

In `.github/workflows/ci.yml`, append a new job under `jobs:` (after `e2e:`):

```yaml
  sim:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true
      - name: Non-determinism simulation
        run: make test-sim-nondeterminism SIM_NUM_BLOCKS=30 SIM_BLOCK_SIZE=25
      - name: Full-app simulation (bounded)
        run: make test-sim-fullapp SIM_NUM_BLOCKS=30 SIM_BLOCK_SIZE=25
```

Keep blocks bounded for PR time budget; the weekly schedule in `codeql.yml` plus a longer manual run cover deeper sims. (Optional: add `if: github.event_name == 'schedule'` gating for a heavier variant later.)

- [ ] **Step 3: Validate YAML locally**

Run: `python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/codeql.yml')); yaml.safe_load(open('.github/workflows/ci.yml')); print('ok')"`
Expected: prints `ok`.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/codeql.yml .github/workflows/ci.yml
git commit -m "ci: add CodeQL analysis + bounded simulation job"
```

---

## Workstream 6 — Load Test (Baseline)

### Task 14: tm-load-test harness + baseline doc

**Files:**
- Create: `infra/loadtest/README.md`
- Create: `infra/loadtest/loadtest.toml`
- Modify: `Makefile`
- Create: `docs/load-test.md`

- [ ] **Step 1: Create the load-test config + README**

Create `infra/loadtest/loadtest.toml` documenting the run parameters (endpoints, connections, rate, duration, transaction size) for `tm-load-test` against the Phase 6 devnet RPC (`ws://localhost:26657/websocket`). Create `infra/loadtest/README.md` explaining prerequisites (`make localnet-up` from `docs/devnet.md`), how to install `tm-load-test` (`go install github.com/informalsystems/tm-load-test/cmd/tm-load-test@latest`), and how to run `make load-test`.

- [ ] **Step 2: Add the Makefile target**

In `Makefile`, add:

```make
LOADTEST_ENDPOINT ?= ws://localhost:26657/websocket
LOADTEST_DURATION ?= 60
LOADTEST_RATE ?= 200
LOADTEST_CONNS ?= 4

load-test:
	@echo "Running tm-load-test against $(LOADTEST_ENDPOINT) (devnet must be up)..."
	@tm-load-test -c $(LOADTEST_CONNS) -T $(LOADTEST_DURATION) -r $(LOADTEST_RATE) \
		--broadcast-tx-method sync --endpoints $(LOADTEST_ENDPOINT)

.PHONY: load-test
```

- [ ] **Step 3: Run against the devnet and capture the baseline**

Run (requires devnet up): `make localnet-up` then `make load-test`.
Then record the observed numbers. If `tm-load-test` cannot drive Vertix tx format out of the box, document the limitation and use `vertixd`'s built-in `tx` flooding via a small script in `infra/loadtest/` instead; the deliverable is a **documented baseline**, not a specific tool.

- [ ] **Step 4: Write the baseline doc**

Create `docs/load-test.md` with: method, exact topology/hardware, transaction mix, and the measured baseline (sustained TPS, p50/p99 broadcast-to-commit latency, observed block time), plus a "how to re-run" section. Explicitly note **no pass/fail threshold is set in Phase 7** (deferred to Phase 9, per the design spec §8 and Appendix C).

- [ ] **Step 5: Commit**

```bash
git add infra/loadtest/ docs/load-test.md Makefile
git commit -m "perf: tm-load-test harness + documented TPS baseline (Phase 7)"
```

---

## Workstream 7 — Validator Security Docs

### Task 15: `docs/tmkms.md` + `docs/validator-setup.md`

**Files:**
- Create: `docs/tmkms.md`
- Create: `docs/validator-setup.md`

- [ ] **Step 1: Write `docs/tmkms.md`**

Create `docs/tmkms.md` covering: what `tmkms` protects (consensus key only), supported backends (softsign / YubiHSM / Ledger), a `tmkms.toml` example pinned to `chain_id = "vertix-1"`, connecting `tmkms` to `vertixd` over the privval socket (`priv_validator_laddr`), double-sign protection, and failover. Add a **Vertix-specific** section: the **feeder key is separate** from the consensus key and the operator key — `tmkms` guards only consensus; the feeder uses its own delegated key (`x/oracle` feeder delegation, Phase 1) and the operator key never lives on the feeder host.

- [ ] **Step 2: Write `docs/validator-setup.md`**

Create `docs/validator-setup.md` covering: sentry-node architecture (validator behind ≥2 sentries, `pex`, `persistent_peers`, `private_peer_ids`, `unconditional_peer_ids`), firewall/port guidance (p2p `:26656` open on sentries only, RPC bound to localhost), the three-key separation model (consensus vs feeder vs operator), and the monitoring hooks (CometBFT `:26660` Prometheus, feeder `:9200`) that Phase 8 builds dashboards on. Cross-link `docs/tmkms.md` and the Phase 6 `docs/devnet.md`.

- [ ] **Step 3: Commit**

```bash
git add docs/tmkms.md docs/validator-setup.md
git commit -m "docs: add tmkms + validator sentry/key-separation security guides"
```

---

## Workstream 8 — Doc-Syncs

### Task 16: Update spec/design/structure/AGENTS docs

**Files:**
- Modify: `docs/full-design-spec.md`
- Modify: `docs/technical-design.md`
- Modify: `docs/project-structure.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: `full-design-spec.md`**

In the Phase 7 section, replace the invariant bullet "oracle (prices exist for configured pairs)" wording with the structural-integrity form (stored prices are accept-listed + strictly positive; existence/freshness is monitoring). In Appendix C, mark "TPS target (Phases 7/9)" as **resolved for Phase 7 = baseline-only** (link `docs/load-test.md`); the hard threshold remains a Phase 9 item.

- [ ] **Step 2: `technical-design.md`**

Record: the oracle `prices` invariant form; the `x/fees` `reconcile` + `module-balance` invariants; and the two new fees KV keys (`KeyGenesisSupply` `0x02`, `KeyCumulativeBurned` `0x03`) with their `InitGenesis` snapshot + `EndBlocker` increment touchpoints. Correct §4.6 if it still references the pre-burn-counter invariant statement.

- [ ] **Step 3: `project-structure.md`**

Add `docs/gas-audit.md`, `docs/load-test.md`, and `infra/loadtest/` to the layout. Correct the "created by Phase 6" note on `tmkms.md`/`validator-setup.md` to **Phase 7** (canonical numbering).

- [ ] **Step 4: `AGENTS.md`**

In §6 (Architectural Invariants), add a reference to the five permanent crisis-invariant routes: `oracle/prices`, `fees/reconcile`, `fees/module-balance`, `rwa/bonds`, `rwa/denoms`.

- [ ] **Step 5: Commit**

```bash
git add docs/full-design-spec.md docs/technical-design.md docs/project-structure.md AGENTS.md
git commit -m "docs: sync spec/design/structure/AGENTS for Phase 7 invariants + load baseline"
```

---

## Final Gate

- [ ] **Run the full local gate**

Run: `make lint && make test && make build`
Expected: all three succeed.

- [ ] **Run the simulation gate**

Run: `make test-sim-fullapp && make test-sim-import-export && make test-sim-nondeterminism`
Expected: zero invariant violations across all three.

- [ ] **Verify acceptance criteria**

Confirm against the design spec §10.2: invariants registered + green under simulation; adversarial/property/fuzz suites green; CodeQL workflow present (findings triaged on first run); TPS baseline documented in `docs/load-test.md`; `docs/tmkms.md` + `docs/validator-setup.md` complete.

---

## Notes for the Implementer

- **TDD discipline:** every behavioral task writes the failing test first, watches it fail, implements, watches it pass, commits.
- **Where signatures are uncertain:** several steps say "confirm X by grepping Y". Do that confirmation before writing the code — the surrounding existing tests (`x/*/keeper/*_test.go`) are the source of truth for harness construction and message field names.
- **Simulation ops are the fragile part:** start with the smallest working op set (oracle `SubmitFeed`, rwa `RegisterAsset`), get `make test-sim-fullapp` green at small block counts, then expand. Always return `simtypes.NoOpMsg(...)` when a precondition isn't met rather than erroring.
- **No proto changes:** the only new state is the two raw-KV fees keys. Do not add genesis/proto fields.
- **Toolchain:** the Makefile pins `GOTOOLCHAIN=go1.25.4`; run Go commands through `make` targets where possible so the toolchain is consistent.
