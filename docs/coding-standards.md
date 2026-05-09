# Vertix — Coding Standards

This document is the **single source of truth** for code style, conventions, and quality bars in the Vertix repository. All contributors and AI agents must follow these rules. CI enforces a subset; reviewers enforce the rest.

---

## 1. Languages & Versions

| Item | Value | Enforcement |
|---|---|---|
| Go | 1.22+ | `go.mod` + CI `setup-go` |
| Cosmos SDK | v0.50.x | Pinned in `go.mod` |
| CometBFT | v0.38.x | Pinned in `go.mod` |
| ibc-go | v8.x | Pinned in `go.mod` |
| Protobuf | proto3 + buf | `buf` toolchain |
| YAML (config) | 1.2 | Loader-validated |

Pinned versions are non-negotiable without a roadmap change. Bumps must accompany a migration note in the PR.

---

## 2. Go Style

### 2.1 Formatting & Imports

- Code MUST be `gofmt`-clean. CI fails otherwise.
- Imports MUST be `goimports`-grouped with `local-prefixes: github.com/vertix-network/vertix`:

```go
import (
    "context"
    "fmt"
    "time"

    "cosmossdk.io/math"
    sdk "github.com/cosmos/cosmos-sdk/types"

    "github.com/vertix-network/vertix/x/oracle/types"
)
```

Three groups: stdlib, third-party, local. Blank line between groups.

### 2.2 Linters (CI-enforced)

`.golangci.yml` enables: `govet`, `errcheck`, `staticcheck`, `unused`, `gosimple`, `ineffassign`, `typecheck`, `gofmt`, `goimports`, `revive`, `misspell`, `unconvert`, `unparam`, `prealloc`.

A PR with new lint findings does not merge.

### 2.3 Naming

| Construct | Rule | Example |
|---|---|---|
| Packages | lowercase, no underscores | `keeper`, `types` |
| Module name | match directory: `oracle`, `rwa`, `fees` | `x/oracle/types.ModuleName = "oracle"` |
| Errors | `Err<Subject><Predicate>` | `ErrInvalidPrice`, `ErrPairNotAccepted` |
| Events | `event_<module>_<verb_past_tense>` | `oracle_feed_submitted`, `rwa_minted` |
| Event attribute keys | `AttributeKey<Field>` constants | `AttributeKeyValidator` |
| Store key prefix vars | `KeyPrefix<Subject>` (byte slice) | `KeyPrefixFeed = []byte{0x01}` |
| Single-key constants | `Key<Subject>` (no prefix concept) | `KeyParams` |
| Test functions | `Test<Subject><Behaviour>` | `TestWeightedMedianAggregation` |

### 2.4 Errors

- Use `cosmossdk.io/errors` for module-level sentinel errors. Register them in `types/errors.go`:

```go
var (
    ErrInvalidPair  = errors.Register(ModuleName, 1, "invalid denom pair")
    ErrInvalidPrice = errors.Register(ModuleName, 3, "invalid price: must be positive")
)
```

- Wrap with context using `.Wrapf(...)`:

```go
return nil, types.ErrInvalidPrice.Wrapf("got %q", msg.Price)
```

- Never use `panic` in keeper code outside `InitGenesis`. Return errors.
- Never use `fmt.Errorf` for module errors that callers may inspect — use registered sentinels.

### 2.5 Comments & Godoc

- Exported functions, types, and constants MUST have a Godoc comment that begins with the identifier name.
- Comments explain **why**, not **what**, when the code is not self-evident.
- No "narrating" comments (`// Increment counter`, `// Return result`).

### 2.6 Concurrency

- Keeper code is single-threaded per ABCI call; do not introduce goroutines.
- The feeder is concurrent: use `context.Context` for cancellation, `sync.WaitGroup` for coordinated shutdown, `time.Ticker` for periodic work.
- Never share a `sdk.Context` across goroutines.

---

## 3. Protobuf Conventions

### 3.1 Layout

```
proto/vertix/<module>/v1/
├── types.proto      Domain types (records, params)
├── tx.proto         Msg service + Msg* requests/responses
├── query.proto      Query service + queries
└── genesis.proto    GenesisState
```

### 3.2 Rules

- One `package` per module: `vertix.<module>.v1`.
- `option go_package = "github.com/vertix-network/vertix/x/<module>/types";`
- All `Msg`s in the `Msg` service set `option (cosmos.msg.v1.signer) = "<field>";`.
- Decimal fields use `math.LegacyDec`:

```protobuf
string price = 3 [
  (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec",
  (gogoproto.nullable)   = false
];
```

- Address fields:

```protobuf
string validator = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
```

- Coin amounts use `cosmos.base.v1beta1.Coin` from `cosmos/base/v1beta1/coin.proto`.
- Timestamps use `google.protobuf.Timestamp` with `(gogoproto.stdtime) = true` and `(gogoproto.nullable) = false`.
- Enums use `<NAME>_UNSPECIFIED = 0` first.

### 3.3 Backward Compatibility

- Do **not** delete or renumber existing fields. Use `reserved` for retired field numbers.
- Do **not** change wire types of existing fields.
- New optional fields are appended at the end with the next available number.
- All breaking changes must be staged through an `x/upgrade` migration.

---

## 4. Cosmos SDK Module Patterns

### 4.1 Keeper Construction

```go
type Keeper struct {
    cdc          codec.BinaryCodec
    storeService store.KVStoreService
    // typed external deps (interfaces, not concrete keepers)
    bankKeeper   BankKeeper
    oracleKeeper OracleKeeper
    authority    string // gov module address
}

func NewKeeper(
    cdc codec.BinaryCodec,
    storeService store.KVStoreService,
    bk BankKeeper,
    ok OracleKeeper,
    authority string,
) Keeper { /* ... */ }
```

- Keeper holds **interfaces**, not concrete keepers, so unit tests can pass mocks.
- `authority` (gov module address) is captured at construction; no global lookup.

### 4.2 Msg Server

- Always validate inputs:

```go
func (m msgServer) SubmitFeed(ctx context.Context, msg *types.MsgSubmitFeed) (*types.MsgSubmitFeedResponse, error) {
    if err := msg.ValidateBasic(); err != nil {
        return nil, err
    }
    // ... business logic
    return &types.MsgSubmitFeedResponse{}, nil
}
```

- For governance-gated handlers, check authority first:

```go
if msg.Authority != m.GetAuthority() {
    return nil, types.ErrUnauthorized.Wrapf("expected %s, got %s", m.GetAuthority(), msg.Authority)
}
```

### 4.3 Events

Emit events on every meaningful state change:

```go
sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
    types.EventTypeFeedSubmitted,
    sdk.NewAttribute(types.AttributeKeyValidator, msg.Validator),
    sdk.NewAttribute(types.AttributeKeyPair, msg.Pair),
    sdk.NewAttribute(types.AttributeKeyPrice, msg.Price),
))
```

Indexers, explorers, and tests rely on event consistency. Do not skip them.

### 4.4 Store Access

- Use `storeService.OpenKVStore(ctx)`; never `ctx.KVStore(storeKey)` directly.
- Always check the iterator error and `defer iter.Close()`.
- Big-endian encode integer keys for ordered iteration.

### 4.5 Genesis

- `InitGenesis` may panic on unrecoverable invariants — the chain genuinely cannot start.
- `ExportGenesis` MUST be a pure read; no state mutation.
- A round-trip test (`InitGenesis(ExportGenesis(state)) == state`) is required for every custom module.

### 4.6 Begin/EndBlock

- No unbounded loops. If iterating validators or assets, document an upper bound and cover with a test.
- Gas-meter awareness: `EndBlock` runs without per-tx gas meter — discipline is on the developer.

---

## 5. Testing Standards

### 5.1 Test Pyramid

| Layer | Where | Speed | Coverage Target |
|---|---|---|---|
| Type tests | `x/<m>/types/*_test.go` | <1s | 100% on `Validate`, `ValidateBasic`, key builders |
| Keeper unit tests | `x/<m>/keeper/*_test.go` with `testutil/keeper` | <5s | All branches in keeper logic |
| ABCI tests | `x/<m>/abci_test.go` | <5s | Begin/EndBlock paths |
| App tests | `app/app_test.go` | <30s | Module wiring, genesis export |
| Simulation | `x/<m>/simulation/...` | minutes | Phase 6 onward |
| E2E IBC | `e2e/` (separate go.mod) | minutes | ICS-20 transfers |

### 5.2 Test Helpers

Use `testutil/keeper/<module>.go` for keeper fixtures. The canonical pattern:

```go
func OracleKeeper(t *testing.T) (sdk.Context, keeper.Keeper) {
    t.Helper()
    storeKey := storetypes.NewKVStoreKey(types.StoreKey)
    db := dbm.NewMemDB()
    stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
    stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
    require.NoError(t, stateStore.LoadLatestVersion())

    cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
    k := keeper.NewKeeper(cdc, runtime.NewKVStoreService(storeKey), nil, nil, /* authority */ "vtx1...")
    ctx := sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())
    require.NoError(t, k.SetParams(ctx, types.DefaultParams()))
    return ctx, k
}
```

External keeper deps (`nil` above) are wired only when the test exercises that integration; otherwise pass `nil` and guard with `if k.<dep> == nil { return }` in the keeper.

### 5.3 Test-First Discipline

Plans in `docs/plans/` follow a write-failing-test → implement → green pattern. New code added outside of a plan must follow the same:

1. Add a failing unit test in the relevant `_test.go`.
2. Run it; confirm it fails for the right reason.
3. Implement the minimum code to pass.
4. Add edge cases.

### 5.4 Required Negative Tests

Every `ValidateBasic`, every state-machine transition, and every keeper write must have at least one negative test case (invalid input → expected error). Table-driven tests are preferred:

```go
cases := []struct{
    name    string
    msg     types.MsgSubmitFeed
    wantErr bool
}{ /* ... */ }
for _, tc := range cases {
    t.Run(tc.name, func(t *testing.T) {
        err := tc.msg.ValidateBasic()
        if tc.wantErr { require.Error(t, err) } else { require.NoError(t, err) }
    })
}
```

### 5.5 Race & Determinism

- All test runs use `-race -count=1`.
- No `time.Sleep` for synchronization — use channels or `eventually` patterns.

---

## 6. Security & Invariants

### 6.1 Mandatory Checks

- Every keeper write that depends on bonded stake MUST tolerate `nil` external keepers (test mode) by short-circuiting cleanly.
- Every monetary path (`Mint`, `Burn`, `Send`) MUST go through `bank.Keeper`. No raw store mutation of supply.
- `MsgUpdateParams` handlers MUST validate the incoming params before persisting.

### 6.2 Crisis Invariants

Every custom module registers at least one invariant with `x/crisis`:

| Module | Invariant |
|---|---|
| `x/oracle` | An aggregated price exists for every accepted pair after the first window |
| `x/rwa` | All ACTIVE assets have a bonded amount in the module account ≥ `MinIssuerBond` |
| `x/fees` | After EndBlock, fee collector has zero balance for `uvtx` |

### 6.3 The Hard Cap

Three independent guards:

1. `x/mint` is not in `app.go`.
2. `app/app_test.go::TestNoMintModule` asserts module manager has no `mint`.
3. Code review checklist: any PR touching `app.go` requires hard-cap review.

Any change that can produce new VTX outside the genesis allocation, vesting unlocks, or `x/staking` rewards funded by fee/pool revenue is **rejected by default**.

---

## 7. Commit & Branch Conventions

### 7.1 Commit Message Format

[Conventional Commits](https://www.conventionalcommits.org/) with module scope:

```
<type>(<scope>): <short summary, imperative mood, no trailing period>

<optional body — what / why; wrap at 72 chars>

<optional footer — Refs: #123, BREAKING CHANGE: ...>
```

Allowed `<type>`: `feat`, `fix`, `refactor`, `test`, `docs`, `build`, `ci`, `chore`, `perf`.
Allowed `<scope>`: `oracle`, `rwa`, `fees`, `feeder`, `app`, `proto`, `infra`, `docs`, `ci`, or omit for cross-cutting.

Examples:

```
feat(oracle): implement weighted-median aggregator
fix(rwa): block transfer when restriction allowlist empty
docs: add architecture and technical design
chore: remove x/mint to enforce 21M hard cap
```

### 7.2 Branches

- `main` — protected, always green, deployable to devnet.
- `develop` — integration branch (optional; PRs may target `main` directly).
- Feature branches: `feat/<scope>-<slug>` (e.g. `feat/oracle-weighted-median`).
- Plan-driven branches: `plan/<NN>-<slug>` for the Nth plan in `docs/plans/`.

### 7.3 PRs

- One logical change per PR.
- Title follows commit format.
- Description must include: motivation, what changed, how it was tested, links to spec/plan/issues.
- Every PR must keep `make test` and `make lint` green.

---

## 8. Documentation Requirements

| Change | Required doc update |
|---|---|
| New module | Add module section to `architecture.md` and `technical-design.md` |
| New `Msg` or `Query` | Update module's `client/cli/` and reflect in `technical-design.md` |
| New genesis param | Update `tokenomics.md` (governance table) and the spec |
| New event | Update `technical-design.md` event tables |
| Breaking proto change | Migration note + changelog + spec update |
| New script / Make target | Update `project-structure.md` Makefile table |

The `docs/specs/` directory holds **approved** specs (immutable once dated). Iterations live in `docs/plans/`.

---

## 9. Tooling Cheat Sheet

```bash
# Format
gofmt -w .
goimports -w -local github.com/vertix-network/vertix .

# Lint
make lint                          # full repo
golangci-lint run ./x/oracle/...   # one module

# Test
make test                          # all
go test ./x/oracle/... -v          # one module
go test ./x/oracle/... -run TestWeightedMedian -v -race

# Coverage
make test-cover && open coverage.html

# Proto
make proto-gen
buf lint
buf breaking --against '.git#branch=main'

# Devnet
make devnet-reset
make build && build/vertixd genesis validate-genesis

# Feeder
go build -o build/vertix-feeder ./feeder/cmd/vertix-feeder
build/vertix-feeder --config feeder-config.example.yaml
```

---

## 10. Code Review Checklist

Use this list for any PR touching production code:

- [ ] All new exported symbols have Godoc.
- [ ] No new `golangci-lint` findings.
- [ ] Tests added for new behavior (positive + negative).
- [ ] Tests run with `-race`; no flakes.
- [ ] Events emitted for every state change.
- [ ] No raw `KVStore` access bypassing `storeService`.
- [ ] No `panic` outside `InitGenesis`.
- [ ] No new `x/mint` reference; supply guards intact.
- [ ] Proto changes are additive (or behind an `x/upgrade` migration).
- [ ] Docs updated where §8 requires.
- [ ] Commit messages follow §7.1.
- [ ] Spec/plan/issue referenced in PR description.
