# Phase 2 — `x/fees` (Burn + Distribute) — Design Spec

**Date:** 2026-05-29
**Status:** Approved (brainstorm output)
**Phase:** 2 of the canonical phase map ([`full-design-spec.md`](../full-design-spec.md) §7.2)
**Depends on:** Phase 0 (Chain Foundation); SDK `bank`, `distribution`, `params` · **Enables:** Phase 3 (`x/rwa` fee-routing target)
**Owns:** the `EndBlock` burn of native `uvtx` fees from the standard fee collector, the `FeesParams` (`BurnRatio`/`DistributionRatio`) governance surface, and the burn/distribution split contract that `x/rwa` (Phase 3) routes fees into.

> This is the per-phase design spec produced by the brainstorm of Phase 2. It refines — and must stay consistent with — the program source of truth ([`full-design-spec.md`](../full-design-spec.md) Phase 2) and the engineering reference ([`technical-design.md`](../technical-design.md) §4). Where this doc and the program spec disagree on phase order or cross-phase contracts, the program spec wins. Module internals defer to `technical-design.md`, **except** where this doc explicitly refines it (flagged as doc-sync items in §12 — notably the §4.3 distribution call and the §4.6 invariant, both of which this brainstorm corrects). The step-by-step execution plan lives in `docs/plans/` (produced after this spec).

---

## 1. Goal & Deliverable Boundary

Deliver a working, coordination-only `x/fees` Cosmos SDK module. At each `EndBlock` it burns a governance-set fraction (`BurnRatio`, default 40%) of the native **`uvtx`** balance accumulated in the standard fee collector (`auth.FeeCollectorName`), then **leaves the remainder in place** so the SDK's native `x/distribution` `BeginBlock` distributes it to stakers next block. The module holds **no custom state beyond `FeesParams`**, never influences the staker-distribution math, and only ever *reduces* total supply (monotonic burn) — honoring the 21M hard cap (Invariant 1) and fee-determinism (Invariant 5).

**In scope:**

- Protobuf surface: `FeesParams`, `MsgUpdateParams`, query `Params`, genesis.
- `EndBlock` burn sweep of **`uvtx` only**, with floor + dust + zero guards.
- A module account with **`authtypes.Burner`** permission (required by `BurnCoins`).
- Params + validation (`BurnRatio + DistributionRatio == 1`), genesis, events, gRPC/CLI query.
- App wiring: `fees` appended **last** in the custom `endBlockers` suffix (`oracle → rwa → fees`); `maccPerms` burner entry; gov authority for `MsgUpdateParams`.
- Unit + keeper-integration + app-level tests satisfying the Phase 2 acceptance gate.

**Out of scope (deferred):**

- Fee *generation* — gas fees are built-in; RWA mint/settle fees arrive in Phase 3 and land in the fee collector per the Phase 2 hand-off contract (§11).
- The staker-distribution allocation itself — owned by the unmodified SDK `x/distribution` (Invariant 6). `x/fees` deliberately does **not** reimplement it (D1).
- Community-pool policy and the `community_tax` value — a Phase 0 genesis concern, flagged here as a cross-phase note (§12).
- `x/crisis` invariant *registration* — authored in Phase 7; this spec only specifies the corrected invariant statements (§12).

---

## 2. Locked Decisions (from this brainstorm)

| # | Decision | Choice | Rationale |
|---|---|---|---|
| D1 | **Distribution mechanism** | **Burn-only sweep.** `x/fees` `EndBlock` burns only the `BurnRatio` portion of `uvtx`; the remaining `DistributionRatio` portion is **left in the fee collector** for the SDK's native `x/distribution` `BeginBlock` to distribute to stakers. | The fee collector is *already* swept to stakers each `BeginBlock` by `x/distribution`. Re-implementing proportional, community-tax-aware, per-validator allocation inside `x/fees` (the "active distribute" alternative) duplicates consensus-critical SDK code and risks double-distribution. Burning only the 40% and letting the native flow handle the rest is minimal, idiomatic, and keeps `x/distribution` unmodified (Invariant 6). It is still fully on-chain and deterministic (Invariant 5). **Trade-off accepted:** the literal "fee collector is zero after `EndBlock`" invariant sketched in `technical-design.md` §4.6 is false by design (60% remains); corrected in §12. |
| D2 | **Burn denom scope** | **`uvtx` only.** The 40% burn applies solely to the native `uvtx` balance; any non-`uvtx` coin in the fee collector is left untouched. | Protects `rwa/*` factory denoms and IBC-bridged assets from ever being burned, and ties the hard-cap burn semantics strictly to the native token. In practice the collector is `uvtx`-only (gas + RWA fees are paid in `uvtx`), but guarding by denom is defensive and cheap. |
| D3 | **Community tax** | **Target `community_tax = 0`** in genesis so the `DistributionRatio` (60%) lands entirely with stakers, matching tokenomics. | With D1, the 60% flows through `x/distribution`, which skims `community_tax` off the top before paying stakers. A zero tax makes "60% to stakers" literal. This is a **Phase 0 genesis** param — flagged as a cross-phase note + doc-sync (§12), not owned by `x/fees`. Governance can later raise it to fund a community pool. |
| D4 | **No custom state / no reimplemented distribution** | `x/fees` stores only `FeesParams`. It calls `bank` to burn and relies on `x/distribution` (unmodified) for staker payout. | Coordination-only module (`technical-design.md` §4.1). Smaller surface, smaller audit target, no consensus-critical allocation logic of our own. |
| D5 | **Module account w/ Burner** | `x/fees` gets a module account with `authtypes.Burner`. Burn flow: `FeeCollectorName → fees account → BurnCoins`. | `BurnCoins` requires the coins to be held by a burner-permissioned module account; the standard fee collector is not one. The `x/fees` account holds coins only transiently within a single `EndBlock` (zero between blocks — basis for the corrected invariant in §12). |
| D6 | **Burn at `EndBlock`** | Burn runs in `x/fees.EndBlock`, ordered **last** in the custom suffix (`oracle → rwa → fees`). | Per program spec Phase 2 and `technical-design.md` §6. RWA forwards fees to the collector during message handling, so by `fees.EndBlock` the block's full fee amount is present; burning 40% then leaves exactly 60% for the next `BeginBlock`'s native distribution. See §9 timeline. |
| D7 | **Keep both ratio params** | Persist `burn_ratio` **and** `distribution_ratio`, validated to sum to 1, even though only `burn_ratio` drives runtime behavior. | Honors the program spec's ratio-invariant cross-phase contract. `distribution_ratio` is the documented complement that the native distribution flow realizes; storing it keeps the governance surface and validation exactly as specified and self-documenting. |
| D8 | Module creation | **`ignite scaffold module fees --dep bank,distribution`, then adapt** | Consistent with Phase 0 (D4) and Phase 1 (D1); inherits depinject wiring, the `buf`/`make proto-gen` pipeline, autocli, and scaffold layout; slots into the existing `app_config.go`/`app.go` markers. `technical-design.md` §7 `NewKeeper(...)` snippets remain illustrative; the binding contract is the module set, the `endBlockers` ordering, and the burn/distribution split. |

These inherit and must not contradict the program-level locked decisions in [`full-design-spec.md`](../full-design-spec.md) §§1–6, nor the architectural invariants §5.

---

## 3. Target Module Layout (Phase 2 outputs)

```
x/fees/
├── keeper/
│   ├── keeper.go         params get/set; bank + auth keeper handles
│   ├── abci.go           EndBlocker: compute burn → move → burn → emit
│   ├── msg_server.go     UpdateParams
│   ├── grpc_query.go     Params
│   ├── genesis.go        InitGenesis / ExportGenesis
│   └── *_test.go
├── types/
│   ├── keys.go           ModuleName, StoreKey, params store key, fee module account name
│   ├── params.go         FeesParams defaults + Validate() (sum==1, bounds)
│   ├── msgs.go           ValidateBasic (MsgUpdateParams)
│   ├── expected_keepers.go  BankKeeper, AccountKeeper (DistributionKeeper not needed under D1)
│   ├── errors.go         ErrInvalidRatioSum, ...
│   ├── events.go, codec.go, genesis.go
│   └── *.pb.go           generated
├── module/
│   ├── module.go         AppModule (EndBlock, genesis, services)
│   ├── depinject.go      ProvideModule (inputs: bank, auth keepers; authority)
│   └── autocli.go        query + tx command wiring
proto/vertix/fees/v1/
├── types.proto    FeesParams { burn_ratio, distribution_ratio }
├── tx.proto       MsgUpdateParams (+ response)
├── query.proto    Params
└── genesis.proto  GenesisState
```

This realizes the `x/fees` rows of [`project-structure.md`](../project-structure.md) and the §4 layout of [`technical-design.md`](../technical-design.md).

---

## 4. App Wiring (depinject)

The scaffold inserts fees at the `# stargate/app/...` markers established in Phase 0:

- **`app_config.go`:**
  - `ModuleConfig` for `fees` appended at `# stargate/app/moduleConfig`.
  - `feestypes.ModuleName` appended to `endBlockers` **last** — the final element of the custom `oracle → rwa → fees` suffix (after all SDK/IBC modules and after the other custom modules), so RWA fees from the same block are present before the burn (D6).
  - `feestypes.ModuleName` appended to `genesisModuleOrder` after the other custom modules (params-only genesis; order is not sensitive but kept in the custom suffix for clarity).
  - **`moduleAccPerms`:** add `feestypes.ModuleName: {authtypes.Burner}` (D5).
  - No `beginBlockers` / `preBlockers` entry.
- **`app.go`:** `FeesKeeper` field added to the `App` struct and to the `depinject.Inject` target. `x/fees` exposes no keeper interface to other modules (it is a pure sink/coordinator).
- **Authority:** `MsgUpdateParams.authority` must equal `authtypes.NewModuleAddress(govtypes.ModuleName).String()`.

**Expected external keepers** (held by the fees keeper, depinject-provided):

```go
type AccountKeeper interface {
    GetModuleAddress(name string) sdk.AccAddress
}

type BankKeeper interface {
    GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
    SendCoinsFromModuleToModule(ctx context.Context, senderModule, recipientModule string, amt sdk.Coins) error
    BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
}
```

> **No `DistributionKeeper` dependency (D1).** Because the 60% is distributed by the native `x/distribution` `BeginBlock` and not by `x/fees`, the fees keeper does **not** hold a distribution keeper handle. The `--dep ...,distribution` scaffold flag is used only to register the wiring relationship; the keeper interface above is the binding contract. This is a doc-sync to `technical-design.md` §4.3 (§12).

---

## 5. Protobuf Surface (`proto/vertix/fees/v1/`)

Additive-only from first generation onward; no field renumbering or removals (coding-standards proto convention).

### 5.1 `types.proto`

```protobuf
message FeesParams {
  string burn_ratio         = 1 [(cosmos_proto.scalar) = "cosmos.Dec"];  // "0.40"
  string distribution_ratio = 2 [(cosmos_proto.scalar) = "cosmos.Dec"];  // "0.60" — complement realized by native x/distribution (D1/D7)
}
```

### 5.2 `tx.proto`

```protobuf
service Msg {
  rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);
}

message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";
  string     authority = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  FeesParams params    = 2 [(gogoproto.nullable) = false];
}
```

### 5.3 `query.proto`

```protobuf
service Query {
  rpc Params(QueryParamsRequest) returns (QueryParamsResponse);
}
// Params() → FeesParams
```

### 5.4 `genesis.proto`

```protobuf
message GenesisState {
  FeesParams params = 1 [(gogoproto.nullable) = false];
}
```

No custom state beyond params (D4), so genesis round-trips params only.

---

## 6. Params & Validation (`types/params.go`)

`DefaultParams()`:

| Param | Default | Notes |
|---|---|---|
| `burn_ratio` | `0.40` | fraction of `uvtx` fees burned each `EndBlock` |
| `distribution_ratio` | `0.60` | documented complement; realized by native `x/distribution` (D1/D7) |

`Validate()` enforces:

- `burn_ratio` is a valid `LegacyDec` in `[0, 1]`.
- `distribution_ratio` is a valid `LegacyDec` in `[0, 1]`.
- `burn_ratio + distribution_ratio == 1` (the ratio invariant; `ErrInvalidRatioSum` otherwise).

`MsgUpdateParams` re-runs `Validate()` in-handler before persisting; `authority` must equal the gov module address.

---

## 7. State & Keeper Store Ops (`types/keys.go`, `keeper/keeper.go`)

| Prefix | Key | Value | Operations |
|---|---|---|---|
| `0x01` | `(empty)` | `FeesParams` | get / set |

The `x/fees` module account (name `feestypes.ModuleName`, `Burner` perm, D5) holds `uvtx` only transiently inside `EndBlock` between the `SendCoinsFromModuleToModule` and `BurnCoins` calls; it is empty at every block boundary.

---

## 8. Message Handlers (`keeper/msg_server.go`)

**`UpdateParams`** (signed by the gov authority):

1. Authority check: `msg.authority == gov module address`, else `unauthorized`.
2. `msg.params.Validate()` (bounds + sum==1).
3. Persist `0x01`.
4. Emit `fee_params_updated{burn_ratio, distribution_ratio}`.

---

## 9. ABCI `EndBlock` (`keeper/abci.go`)

```
EndBlocker(ctx):
  p = GetParams(ctx)
  if p.burn_ratio.IsZero(): return                       # native flow distributes 100%; nothing to burn
  feeCollector = accountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
  uvtx = bankKeeper.GetBalance(feeCollector, "uvtx")     # D2: native token only
  if uvtx.Amount.IsZero(): return
  burnAmt = uvtx.Amount.ToLegacyDec().Mul(p.burn_ratio).TruncateInt()   # floor (rounds in favor of stakers)
  if burnAmt.IsZero(): return                            # dust guard: tiny balances burn nothing this block
  burnCoins = sdk.NewCoins(sdk.NewCoin("uvtx", burnAmt))
  bankKeeper.SendCoinsFromModuleToModule(FeeCollectorName, fees, burnCoins)   # err → return (never panic)
  bankKeeper.BurnCoins(fees, burnCoins)                                       # err → return (never panic)
  emit fee_burned{amount=burnCoins}
  emit fee_distributed{amount = uvtx.Amount - burnAmt}   # informational: left in collector for next BeginBlock
```

**Cross-boundary timeline (why the split is exact).** With genesis collector = 0:

```
Block N:
  BeginBlock  x/distribution distributes collector balance from block N-1 (= 0.6 × F_{N-1}) to stakers; collector → 0
  Txs         gas + RWA fees F_N accrue → collector = F_N
  EndBlock    x/fees burns 0.4 × F_N (uvtx); collector = 0.6 × F_N
Block N+1:
  BeginBlock  x/distribution distributes 0.6 × F_N to stakers; collector → 0
  ...
```

So each block's `uvtx` fees are 40% burned (same block) and 60% distributed to stakers (next block). The first block's `BeginBlock` distributes the genesis collector (0) — a no-op, no special-casing needed.

**Panic-safety.** All keeper errors (`SendCoinsFromModuleToModule`, `BurnCoins`) cause an early `return` (logged), never a panic — `EndBlock` must not halt the chain on a transient bank error. Math uses `TruncateInt` (floor); the truncation remainder simply stays in the collector and is distributed, so no `uvtx` is lost or created.

**Determinism.** Single denom (`uvtx`), single balance read, deterministic `LegacyDec` multiply + truncate. No iteration, no maps, no external input.

---

## 10. Queries, CLI, Events, Genesis

- **gRPC + autocli query:** `vertixd q fees params`.
- **Tx CLI:** `vertixd tx fees update-params` (gov-gated; normally submitted via a governance proposal).
- **Events (typed):**
  - `fee_burned{amount}` — the `uvtx` burned this block.
  - `fee_distributed{amount}` — the `uvtx` left in the fee collector for the next `BeginBlock`'s native staker distribution (informational; the actual transfer is performed by `x/distribution`).
  - `fee_params_updated{burn_ratio, distribution_ratio}`.
- **Genesis:** `DefaultGenesis` = `DefaultParams`; `InitGenesis` validates then sets params; `ExportGenesis` dumps params. `TestGenesisRoundTrip` covers it.

---

## 11. Acceptance Gate Mapping

| Spec gate ([`full-design-spec.md`](../full-design-spec.md) Phase 2) | How satisfied |
|---|---|
| Fee burn reduces bank total supply (verifiable) | Keeper-integration test: seed collector with `uvtx` → `EndBlock` → assert `bankKeeper.GetSupply("uvtx")` dropped by exactly `floor(balance × burn_ratio)` |
| Staker rewards reflect distributed fees via `x/distribution` queries | App-level test: real gas fees → `EndBlock` burn → next block → `x/distribution` outstanding/withdrawable rewards reflect the 60% remainder (with `community_tax = 0`, D3) |
| Events indexed | Assert `fee_burned` / `fee_distributed` emitted with correct amounts; `fee_params_updated` on governance update |
| Governance can update ratios via proposal | `MsgUpdateParams` authority check + `Validate()` (sum==1) test; rejected when authority ≠ gov or ratios don't sum to 1 |
| Non-`uvtx` untouched (D2) | Seed collector with a non-`uvtx` denom → assert it is not burned and remains for native distribution |
| Inherited Phase 0 gate (`make lint && make test && make build`, `buf lint/breaking`) | CI stays green with the new module + protos |

---

## 12. Cross-Phase Contracts Established / Honored Here + Doc-Syncs

**Cross-phase contracts:**

- **Fee-collector hand-off (consumed by Phase 3):** all fees (gas + RWA mint/settle + future oracle subscriptions) MUST land in `auth.FeeCollectorName`; `x/fees` is the sole burner and never receives fees routed directly to it. Phase 3 routes its `uvtx` mint/settle fees to the collector, never to burn/distribute directly.
- **Ratio invariant (honored, D7):** `burn_ratio + distribution_ratio == 1`, enforced at param validation and on `MsgUpdateParams`.
- **Inherited Phase 0 gates:** depinject wiring, `buf`/proto pipeline, CI/lint/test bar.

**Doc-syncs this phase forces (land alongside this spec / its plan):**

1. **`technical-design.md` §4.3** — replace the `distributionKeeper.AllocateTokensToFeePool(distr coins)` step (no such SDK method; it would double-distribute against the native `BeginBlock` flow) with the **burn-only sweep** of §9. `x/fees` does not call the distribution keeper at all (D1/D4).
2. **`technical-design.md` §4.6 invariant** — the "After each `EndBlock`, `FeeCollector` balance for `uvtx` is zero (everything was swept)" statement is **incorrect** under burn-only (the `DistributionRatio` portion remains by design). Replace with:
   - *(a)* Total `uvtx` burned (via `fee_burned`) is monotonic and ≤ genesis supply − current supply.
   - *(b)* The `x/fees` **module account** balance is zero at every block boundary (it holds coins only transiently within `EndBlock`).
   These are the statements Phase 7 registers with `x/crisis`.
3. **Phase 0 genesis (cross-phase note, D3):** set `community_tax = 0` so the 60% lands entirely with stakers per tokenomics. If a non-zero tax is later desired for a community pool, it is a governance decision and "60% to stakers" becomes "60% × (1 − community_tax) to stakers."

---

## 13. Risks & Open Items

| Item | Risk | Mitigation |
|---|---|---|
| `community_tax ≠ 0` silently skims the 60% | Stakers receive less than tokenomics states | D3 sets it to 0 in Phase 0 genesis; flagged as a doc-sync + Phase 0 verification (§12) |
| Truncation (`floor`) dust accumulates in the collector | Negligible `uvtx` perpetually rolls into the distributed portion | Acceptable — dust is distributed to stakers, never lost or minted; documented (§9) |
| `EndBlock` bank error mid-burn | Chain halt if it panicked | All keeper errors early-`return` (logged), never panic (§9) |
| Misreading "fee collector zero after EndBlock" from stale `technical-design.md` §4.6 | Implementer adds an erroneous full-sweep / active distribute | Doc-sync §12 corrects it before/with the plan; this spec is authoritative on internals here |
| Burning a non-native denom | Destroying user/IBC assets | D2 restricts the burn to `uvtx`; test asserts non-`uvtx` untouched (§11) |
| `BurnRatio` set to 1 via governance | 100% burn, zero staker rewards from fees | Bounds allow it; it is a deliberate governance lever, not a bug; bootstrap rewards still come from the Validator Incentives Pool (tokenomics) |

---

## 14. Definition of Done

- `x/fees` builds and is depinject-wired in `endBlockers` (**last** in the custom suffix) with a `Burner` module account (`maccPerms`).
- `EndBlock` burns exactly `floor(uvtx_balance × burn_ratio)` of **`uvtx` only**, leaves the remainder for native distribution, emits `fee_burned` + `fee_distributed`, and never panics on a bank error.
- `MsgUpdateParams` enforces gov authority + `Validate()` (bounds, `burn_ratio + distribution_ratio == 1`); the `Params` query returns current params via gRPC/CLI.
- Genesis round-trips params (`DefaultParams` default).
- Unit + keeper-integration + app-level tests pass (burn-reduces-supply, native-distribution-reflects-remainder, non-`uvtx`-untouched, dust/zero no-ops, governance update, genesis round-trip); `make lint && make test && make build`, `buf lint`, and `buf breaking` stay green in CI.
- The `technical-design.md` doc-syncs (§12 items 1–2) are landed; the Phase 0 `community_tax = 0` note (item 3) is recorded.
- Phase 2 acceptance gate (§11) satisfied — providing the tested fee sink that Phase 3 (`x/rwa`) routes mint/settle fees into.

---

## Appendix A — Document Map

| Doc | Relationship to this spec |
|---|---|
| [`full-design-spec.md`](../full-design-spec.md) | Program source of truth; Phase 2 section is the parent of this spec |
| [`technical-design.md`](../technical-design.md) | Owns §4 fees internals referenced here; §4.3 and §4.6 receive doc-sync (§12) |
| [`2026-05-29-phase-0-chain-foundation-design.md`](./2026-05-29-phase-0-chain-foundation-design.md) | The depinject app + CI/proto gates this phase builds on; `community_tax` genesis (D3) verified there |
| [`2026-05-29-phase-1-oracle-module-design.md`](./2026-05-29-phase-1-oracle-module-design.md) | Sibling custom module; shares the scaffold-then-adapt approach (D8) and the `endBlockers` custom suffix ordering |
| [`project-structure.md`](../project-structure.md) | Target layout for `x/fees` |
| [`coding-standards.md`](../coding-standards.md) | Lint set, test patterns, proto additivity, genesis round-trip |
| `docs/plans/` | Step-by-step execution plan derived from this spec (next step) |
