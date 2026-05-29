# Phase 1 — `x/oracle` (On-Chain Aggregation + Slashing) — Design Spec

**Date:** 2026-05-29
**Status:** Approved (brainstorm output) · **Revised 2026-05-29** (design-review fixes — see §0 changelog)
**Phase:** 1 of the canonical phase map ([`full-design-spec.md`](../full-design-spec.md) §7.2)
**Depends on:** Phase 0 (Chain Foundation); SDK `staking`, `slashing`, `params` · **Enables:** Phase 3 (`x/rwa` price consumption), Phase 4 (`vertix-feeder` target)
**Owns:** validator-submitted price feeds, feeder delegation, quorum-gated stake-weighted-median aggregation, TWAP histories, miss/outlier slashing, and the `OracleKeeper` interface consumed by `x/rwa`.

> This is the per-phase design spec produced by the brainstorm of Phase 1. It refines — and must stay consistent with — the program source of truth ([`full-design-spec.md`](../full-design-spec.md) Phase 1) and the engineering reference ([`technical-design.md`](../technical-design.md) §2). Where this doc and the program spec disagree on phase order or cross-phase contracts, the program spec wins. Module internals defer to `technical-design.md`, except where this doc explicitly refines it (flagged as doc-sync items in §13). The step-by-step execution plan lives in `docs/plans/` (produced after this spec).

---

## 0. Revision Changelog (2026-05-29 design review)

A careful review of the original brainstorm output surfaced design-level issues in the slashing/aggregation economics and the cross-phase contracts. The fixes below are now incorporated into the body of this spec; the original decisions they supersede are noted inline.

| ID | Severity | Issue | Resolution (this revision) |
|---|---|---|---|
| **C1** | Critical | `MsgSubmitFeed` was signed by the validator **operator key**, forcing that high-value key to be hot on the always-on feeder sidecar. | **Feeder delegation** added: new `MsgSetFeeder`, a `validator → feeder` mapping (`0x07`), and `MsgSubmitFeed` is now signed by the low-value **`feeder`** key. (D10) |
| **C2** | Critical | "Miss = didn't submit **every** pair" meant one dead provider/pair, or a newly-governance-added pair, could slash the **entire validator set** at once. | Miss is assessed **only against pairs that reached quorum** that window; miss/total counters **reset on any `accept_list` change**. (D4, D11) |
| **H1** | High | Outlier band was measured against the **stake-weighted** median, so a >50%-stake validator defines the band and the honest minority gets slashed. | Outlier reference is now the **unweighted (count-based) median** of the window's submissions — decoupled from stake capture. (D2) |
| **H2** | High | Cumulative `min_windows_before_slash=10` + 5% threshold meant a **single** early miss guaranteed a slash. | Replaced with a **tumbling evaluation window** `miss_window_size` (default 500); rate is judged over a fixed, meaningful sample. (D3) |
| **H3** | High | `GetPrice` exposed no freshness, so Phase 3 could attest against a **stale** price. | New `max_price_age` param; `GetPrice`/`GetTWAP` return `ErrStalePrice` when the latest aggregation is older than it. (D12) |
| **H4** | High | `StakingKeeper` expected-interface lacked `GetValidator` (valoper→record) needed by submit + outlier slashing. | Interface corrected; `GetValidatorByConsAddr` dropped in favor of `GetValidator`; EndBlock builds one `valoper→Validator` map per window. (§4) |
| **M1–M4** | Medium | EndBlock panic-safety on stored/imported data, genesis price validation, exact TWAP formula, `vote_window` realignment all under-specified. | Specified in §6/§7/§9. |
| **L1–L5** | Low | distributionHeight choice, zero-power slash no-op, median tie-bias, export-wipes-history all undocumented. | Documented in §7/§9/§13. |

Quorum-gated aggregation (D11) additionally fixes the latent "single validator sets the price" problem: a pair only produces an `AggregatedPrice` when submissions cover ≥ `QuorumFraction` of bonded power.

---

## 1. Goal & Deliverable Boundary

Deliver a working `x/oracle` Cosmos SDK module: a validator delegates a low-value **feeder** key (`MsgSetFeeder`), which submits per-pair prices via `MsgSubmitFeed`; at each `VoteWindow` boundary the module aggregates submissions into a **quorum-gated stake-weighted median** per pair, records **TWAP** history, and slashes validators for **misses** and **outliers** via `x/slashing`. The module exposes the `OracleKeeper` interface (`GetPrice`, `GetTWAP`) — with **staleness** guarantees — that `x/rwa` (Phase 3) consumes, and honors the `MsgSubmitFeed` wire format the `vertix-feeder` (Phase 4) targets.

**In scope:**

- Protobuf surface: `MsgSetFeeder`, `MsgSubmitFeed`, `MsgUpdateParams`, queries `Price`/`Twap`/`Params`/`MissCounter`/`Feeder`, `OracleParams`, genesis.
- **Feeder delegation:** `validator → feeder` mapping so the operator key never has to be hot (C1).
- `EndBlock` aggregation (quorum-gated stake-weighted median), TWAP append + prune, miss + outlier slashing.
- Params + genesis + events + gRPC/CLI queries.
- The consumer `OracleKeeper` interface (with `ErrStalePrice` freshness), and `BASE:QUOTE` pair convention.
- Unit + keeper-integration + app-level tests satisfying the Phase 1 acceptance gate.

**Out of scope (deferred):**

- The off-chain `vertix-feeder` binary (Phase 4) — this spec only fixes the on-chain message/signing contract it must honor.
- RWA price *consumption* / attestation logic (Phase 3).
- Cross-chain oracle queries (post-mainnet, per program spec §1).
- MAD-based outlier detection and per-window **bitmap** miss tracking (documented future hardening for Phase 7, §10). This revision uses an unweighted-median outlier reference and a tumbling miss window, which are sufficient for launch.

---

## 2. Locked Decisions (from this brainstorm)

| # | Decision | Choice | Rationale |
|---|---|---|---|
| D1 | Module creation | **`ignite scaffold module oracle --dep bank,staking,slashing`, then adapt** | Consistent with Phase 0 (D4); inherits depinject wiring, the `buf`/`make proto-gen` pipeline, autocli, and scaffold layout; slots into the existing `app_config.go`/`app.go` markers without re-architecting. The `technical-design.md` §7 `NewKeeper(...)` snippets remain *illustrative* (as Phase 0 D1 established); the binding contract is the module set, ordering, and the `OracleKeeper` interface. |
| D2 | Outlier detection | **Fixed-band deviation vs. the window's UNWEIGHTED (count-based) median**, governed by `OutlierThreshold` (default `0.05`) | **Revised (H1).** The earlier choice compared each submission to the *stake-weighted* median, which a >50%-stake validator can set to its own value — slashing the honest minority as "outliers." Using the **unweighted** median of the window's submissions as the outlier reference decouples the band from stake capture (the documented threat in `technical-design.md` §6.2). Still cross-sectional, deterministic, and gas-cheap (one `LegacyDec` divide + compare per submission). The *aggregated price* remains the **stake-weighted** median (D11); only the **outlier reference** is unweighted. MAD is a documented Phase-7 upgrade (§10). |
| D3 | Miss semantics | **Tumbling evaluation window** per validator: count misses + total over a fixed `MissWindowSize` (default `500`) windows; when `total == MissWindowSize`, slash iff `misses/total > MissThreshold`, then reset both | **Revised (H2).** The earlier cumulative model with `MinWindowsBeforeSlash=10` made a 5% threshold meaningless at N=10 (a *single* early miss forced a slash). A fixed tumbling window of 500 makes the rate meaningful (>25 misses tolerated per 500 windows ≈ 7h at ~50s/window) and still fits `0x04`/`0x05` (misses + total, reset when total hits the window or on `accept_list` change). Finer-grained sliding-window bitmaps are Phase-7 hardening (§10). |
| D4 | Miss definition | A bonded validator **misses a window** if, for **any pair that reached quorum that window** (D11), its feeder did not submit a valid feed | **Revised (C2).** The earlier "must submit *every* `accept_list` pair" rule meant one dead provider, illiquid pair, or a freshly governance-added pair (before feeders ship config) would slash the **whole** validator set. Assessing misses only against **quorum-reached** pairs removes that systemic risk: a pair with no/low coverage simply doesn't count toward anyone's misses. Still a clean binary per `(validator, window)`. |
| D5 | TWAP mechanics | **Append one entry per aggregation; `GetTWAP` reverse-iterates `[now-window, now]`; prune entries older than 24h at each window-end** | Fits the time-ordered `0x03` layout; growth is bounded and small (~1,700 entries/pair for 24h at ~50s/window); deterministic and easy to test. Cumulative-accumulator (Uniswap-V2 style) adds `LegacyDec` overflow handling and checkpointing for no benefit at this scale. |
| D6 | `0x05` granularity | **Per-validator** total-windows counter keyed by `{valoper}` (refines `technical-design.md` §2.4's global counter) | A correct miss-*rate* requires per-validator totals because validators bond at different heights. Flagged as a doc-sync (§13). |
| D7 | No jailing | Oracle slashing calls `SlashingKeeper.Slash` only — **never `Jail`** | Per `technical-design.md` §2.6: oracle slashing is intentionally lighter than double-sign to avoid validator chilling / accidental jailing. |
| D8 | No module account | `x/oracle` holds **no funds**; slashing is delegated to `x/slashing` | No `maccPerms`/`blockAccAddrs` entry needed; reduces attack surface. |
| D9 | Build sequencing | **Approach A — one phase spec/plan, vertical slices in dependency order**, slashing last | Only approach that satisfies the Phase 1 acceptance gate (slash events firing) in one pass; risk isolation achieved by ordering + independent slashing tests. |
| D10 | **Feeder delegation** | A validator delegates a **`feeder`** account via `MsgSetFeeder`; `MsgSubmitFeed` is signed by the `feeder`, not the operator. Unset ⇒ the operator's own account address is the implicit feeder. | **New (C1).** Keeps the high-value operator key cold; only a low-value, rotatable feeder key is hot on the sidecar. Mirrors the proven Umee/Ojo `MsgDelegateFeedConsent` pattern. The mapping lives in `0x07` (`feeder→valoper`) with reverse `0x08` (`valoper→feeder`). This is a **Phase 1** decision because it defines the `MsgSubmitFeed` signer — a contract the Phase 4 feeder and Phase 3 are bound by and which cannot be changed additively later. |
| D11 | **Quorum-gated aggregation** | A pair produces an `AggregatedPrice` only if the submitting validators' summed power ≥ `QuorumFraction` (default `0.667`) of **total bonded power**; otherwise the window is skipped for that pair (no new price, no TWAP entry, not counted for misses). | **New (C2/H1 support).** Prevents a single (or tiny minority) validator from unilaterally setting the on-chain price, and is the gate D4 uses to decide which pairs count toward misses. Deterministic: sum power over the `0x01` feeds for the pair, compare to the bonded total. |
| D12 | **Price staleness** | `GetPrice`/`GetTWAP` return `ErrStalePrice` when the latest aggregation for the pair is older than `MaxPriceAge` (default `300s`). | **New (H3).** `0x02` otherwise retains the last price forever; without a freshness guard Phase 3 could attest/activate an RWA against an hours-old price, undermining Invariant 3. Surfacing staleness as an `error` keeps the `OracleKeeper` signature unchanged. |

These inherit and must not contradict the program-level locked decisions in [`full-design-spec.md`](../full-design-spec.md) §§1–6, nor the architectural invariants §5.

---

## 3. Target Module Layout (Phase 1 outputs)

```
x/oracle/
├── keeper/
│   ├── keeper.go            store ops: feeds, aggregated price, TWAP, miss/total counters, feeder map, params
│   ├── msg_server.go        SetFeeder, SubmitFeed, UpdateParams
│   ├── grpc_query.go        Price, Twap, Params, MissCounter, Feeder
│   ├── abci.go              EndBlocker: aggregate(quorum) → TWAP → outlier slash → miss slash → clear
│   ├── aggregate.go         WeightedMedian + unweighted-median outlier reference + quorum check
│   ├── feeder.go            SetFeeder / ResolveValidatorForFeeder mapping ops
│   ├── genesis.go           InitGenesis / ExportGenesis
│   └── *_test.go
├── types/
│   ├── keys.go              store prefixes 0x01–0x08
│   ├── params.go            OracleParams defaults + Validate()
│   ├── msgs.go              ValidateBasic (MsgSetFeeder, MsgSubmitFeed, MsgUpdateParams)
│   ├── expected_keepers.go  StakingKeeper, SlashingKeeper
│   ├── errors.go            ErrNoPrice, ErrStalePrice, ErrNoTWAPData, ErrPairNotAccepted,
│   │                        ErrNotBondedValidator, ErrFeederNotAuthorized, ...
│   ├── events.go, codec.go, genesis.go
│   └── *.pb.go              generated
├── module/
│   ├── module.go            AppModule (EndBlock, genesis, services)
│   ├── depinject.go         ProvideModule (inputs: staking, slashing keepers; authority)
│   └── autocli.go           query + tx command wiring
proto/vertix/oracle/v1/
├── types.proto   OracleFeed, AggregatedPrice, TWAPEntry, OracleParams
├── tx.proto      MsgSetFeeder, MsgSubmitFeed, MsgUpdateParams (+ responses)
├── query.proto   Price, Twap, Params, MissCounter, Feeder
└── genesis.proto GenesisState
```

This realizes the `x/oracle` rows of [`project-structure.md`](../project-structure.md) and the §2.2 protobuf layout of [`technical-design.md`](../technical-design.md).

---

## 4. App Wiring (depinject)

The scaffold inserts oracle at the `# stargate/app/...` markers established in Phase 0:

- **`app_config.go`:**
  - `ModuleConfig` for `oracle` appended at `# stargate/app/moduleConfig`.
  - `oracletypes.ModuleName` appended to `endBlockers` — the first element of the future `oracle → rwa → fees` custom suffix (after all SDK/IBC modules).
  - `oracletypes.ModuleName` appended to `genesisModuleOrder` **after** `staking` (so the bonded set / powers are initialized before oracle genesis).
  - No `beginBlockers` / `preBlockers` entry (no begin-block work).
  - No `moduleAccPerms` / `blockAccAddrs` entry (D8 — no module account).
- **`app.go`:** `OracleKeeper` field added to the `App` struct and to the `depinject.Inject` target; module-output keeper exposed for Phase 3's `RWAKeeper` to consume via the `OracleKeeper` interface.
- **Authority:** `MsgUpdateParams.authority` must equal `authtypes.NewModuleAddress(govtypes.ModuleName).String()`.

**`OracleKeeper` interface** (exported for Phase 3, verbatim from the cross-phase contract):

```go
type OracleKeeper interface {
    GetPrice(ctx context.Context, pair string) (math.LegacyDec, error)
    GetTWAP(ctx context.Context, pair string, window time.Duration) (math.LegacyDec, error)
}
```

**Expected external keepers** (held by the oracle keeper, depinject-provided):

```go
type StakingKeeper interface {
    GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error)        // valoper → record (submit verify + outlier consAddr)
    GetLastValidatorPower(ctx context.Context, addr sdk.ValAddress) (int64, error)                // weight + slash power
    GetBondedValidatorsByPower(ctx context.Context) ([]stakingtypes.Validator, error)             // deterministic miss loop + quorum total
    TotalBondedTokens(ctx context.Context) (math.Int, error)                                      // quorum denominator (D11)
}

type SlashingKeeper interface {
    Slash(ctx context.Context, consAddr sdk.ConsAddress, fraction math.LegacyDec, power, distributionHeight int64) error
}
```

(`Jail` is intentionally excluded — D7.)

> **Interface correction (H4).** The original draft listed `GetValidatorByConsAddr` (cons→record), but every oracle code path starts from the **operator** address (`MsgSubmitFeed.validator`, the `0x01` feed key): `SubmitFeed` must verify the operator is bonded, and the **outlier** slash path must derive `consAddr` from a valoper. Both need `GetValidator(valAddr)`, which is now the listed method. In `EndBlock` the keeper builds **one `valoper → Validator` map** from `GetBondedValidatorsByPower()` and reuses it for power weights, the quorum total, miss iteration, and `consAddr` resolution — avoiding repeated keeper calls. `TotalBondedTokens` provides the quorum denominator (D11). Doc-sync to `technical-design.md` §2.3 (§13).

> **Signature note (verified against SDK v0.50.14):** `x/slashing` keeper's `Slash` returns only `error`, not `(math.Int, error)` as `technical-design.md` §2.3 shows. The burned amount is therefore not available to the event emitter — the `oracle_slash` event carries the slash **fraction** (rate) instead of an absolute amount. Flagged as a doc-sync (§13).

**Feeder signer model (C1 / D10).** `MsgSubmitFeed` is signed by the **`feeder`** account, never the operator key. The keeper resolves `feeder → validator` via the `0x07` mapping (set by `MsgSetFeeder`, which *is* signed by the operator). If a validator has set no feeder, its own account address (same bytes as the valoper) is the implicit authorized feeder, preserving a zero-config path for test/devnet. `MsgSetFeeder.authority` is the validator operator; `MsgUpdateParams.authority` remains the gov module address.

---

## 5. Protobuf Surface (`proto/vertix/oracle/v1/`)

Additive-only; no field renumbering or removals (coding-standards proto convention).

### 5.1 `types.proto`

```protobuf
message OracleFeed {
  string validator    = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string pair         = 2;
  string price        = 3 [(cosmos_proto.scalar) = "cosmos.Dec"];
  int64  block_height = 4;
}

message AggregatedPrice {
  string                    pair         = 1;
  string                    price        = 2 [(cosmos_proto.scalar) = "cosmos.Dec"];
  int64                     block_height = 3;
  google.protobuf.Timestamp block_time   = 4 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
}

message TWAPEntry {
  string                    pair       = 1;
  string                    price      = 2 [(cosmos_proto.scalar) = "cosmos.Dec"];
  google.protobuf.Timestamp block_time = 3 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
}

message OracleParams {
  int64           vote_window         = 1;  // default 10 (blocks per window)
  string          miss_threshold      = 2 [(cosmos_proto.scalar) = "cosmos.Dec"];  // "0.05"
  string          miss_slash_rate     = 3 [(cosmos_proto.scalar) = "cosmos.Dec"];  // "0.005"
  string          outlier_slash_rate  = 4 [(cosmos_proto.scalar) = "cosmos.Dec"];  // "0.01"
  string          outlier_threshold   = 5 [(cosmos_proto.scalar) = "cosmos.Dec"];  // "0.05" — band vs UNWEIGHTED median (D2)
  int64           miss_window_size    = 6;  // default 500 — tumbling eval window (D3, was min_windows_before_slash)
  string          quorum_fraction     = 7 [(cosmos_proto.scalar) = "cosmos.Dec"];  // "0.667" — min bonded power to aggregate (D11)
  int64           max_price_age       = 8;  // default 300 (seconds) — staleness ceiling (D12)
  repeated string accept_list         = 9;  // VTX:USD, BTC:USD, ETH:USD, ATOM:USD, USDC:USD
}
```

> Field numbers were re-laid out here because this is a **net-new** module (no `.pb.go` shipped yet); the additive-only proto rule binds from first generation onward, not within this brainstorm. `miss_window_size` (6) replaces the original `min_windows_before_slash`; `quorum_fraction` (7) and `max_price_age` (8) are new (D11/D12).

### 5.2 `tx.proto`

```protobuf
service Msg {
  rpc SetFeeder(MsgSetFeeder)       returns (MsgSetFeederResponse);
  rpc SubmitFeed(MsgSubmitFeed)     returns (MsgSubmitFeedResponse);
  rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);
}

// Operator delegates a low-value feeder key (C1/D10). Signed by the operator.
message MsgSetFeeder {
  option (cosmos.msg.v1.signer) = "validator";
  string validator = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];  // valoper
  string feeder    = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];  // account addr authorized to submit
}

// Signed by the FEEDER (not the operator). The keeper resolves feeder → validator.
message MsgSubmitFeed {
  option (cosmos.msg.v1.signer) = "feeder";
  string feeder    = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];  // signer; must be the validator's authorized feeder
  string validator = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];  // valoper the feed is for
  string pair      = 3;
  string price     = 4 [(cosmos_proto.scalar) = "cosmos.Dec"];
}

message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";
  string       authority = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  OracleParams params     = 2 [(gogoproto.nullable) = false];
}
```

### 5.3 `query.proto`

```protobuf
service Query {
  rpc Price(QueryPriceRequest)               returns (QueryPriceResponse);
  rpc Twap(QueryTwapRequest)                 returns (QueryTwapResponse);
  rpc Params(QueryParamsRequest)             returns (QueryParamsResponse);
  rpc MissCounter(QueryMissCounterRequest)   returns (QueryMissCounterResponse);
  rpc Feeder(QueryFeederRequest)             returns (QueryFeederResponse);
}
// Price(pair) → AggregatedPrice  (gRPC error = ErrNoPrice | ErrStalePrice, D12)
// Twap(pair, window_seconds) → { price }  (same staleness contract)
// Params() → OracleParams
// MissCounter(validator) → { int64 misses; int64 total_windows }
// Feeder(validator) → { string feeder }   (the authorized feeder, or the implicit operator addr)
```

### 5.4 `genesis.proto`

```protobuf
message FeederDelegation {
  string validator = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string feeder    = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
}

message GenesisState {
  OracleParams              params   = 1 [(gogoproto.nullable) = false];
  repeated AggregatedPrice  prices   = 2 [(gogoproto.nullable) = false];
  repeated FeederDelegation  feeders = 3 [(gogoproto.nullable) = false];
}
```

In-flight feeds (`0x01`), TWAP history (`0x03`), and miss/total counters (`0x04`/`0x05`) are **not** exported — they are runtime-rebuilt. Params, last aggregated prices, and **feeder delegations** (`0x07`/`0x08`) round-trip — delegations must survive a genesis export/import so validators don't silently revert to operator-key signing after an upgrade. `TestGenesisRoundTrip` covers all three. **`InitGenesis` validates** every imported `AggregatedPrice` (pair ∈ `accept_list`, `price > 0`, parseable, `block_time` not in the future) and every `FeederDelegation` (valid bech32, valoper exists) before writing — malformed imports are a genesis error, never an `EndBlock` panic (M1/M2).

---

## 6. Params & Validation (`types/params.go`)

`DefaultParams()`:

| Param | Default | Notes |
|---|---|---|
| `vote_window` | `10` | blocks per aggregation window |
| `miss_threshold` | `0.05` | max tolerated miss rate over a tumbling window |
| `miss_slash_rate` | `0.005` | |
| `outlier_slash_rate` | `0.01` | |
| `outlier_threshold` | `0.05` | band vs **unweighted** window median (D2) |
| `miss_window_size` | `500` | tumbling eval window in windows (D3); ≈7h at ~50s/window |
| `quorum_fraction` | `0.667` | min bonded power that must submit to aggregate a pair (D11) |
| `max_price_age` | `300` | seconds; staleness ceiling for `GetPrice`/`GetTWAP` (D12) |
| `accept_list` | `["VTX:USD","BTC:USD","ETH:USD","ATOM:USD","USDC:USD"]` | |

`Validate()` enforces:

- `vote_window > 0`; `miss_window_size > 0`; `max_price_age > 0`.
- `miss_threshold`, `outlier_threshold` are valid `LegacyDec` in `(0, 1]`.
- `quorum_fraction` is valid `LegacyDec` in `(0, 1]` (**recommended `> 0.5`** so a minority cannot set the price; warned, not rejected, below 0.5).
- `miss_slash_rate`, `outlier_slash_rate` are valid `LegacyDec` in `[0, 1)` and **`< 0.05`** (the `slash_fraction_double_sign` ceiling — keeps oracle slashing lighter than double-sign per §2.6).
- `accept_list` non-empty; each entry matches `^[A-Z0-9]+:[A-Z0-9]+$`; no duplicates.

`MsgUpdateParams` re-runs `Validate()` in-handler before persisting; `authority` must equal the gov module address. **If the new `accept_list` differs from the stored one, the handler resets all miss/total counters (`0x04`/`0x05`)** so newly-added pairs (which feeders cannot yet serve) and removed pairs do not retroactively slash anyone (C2). A `param_update` event records whether a counter reset occurred.

---

## 7. State & Keeper Store Ops (`types/keys.go`, `keeper/keeper.go`)

| Prefix | Key | Value | Operations |
|---|---|---|---|
| `0x01` | `{valoper}/{pair}` | `OracleFeed` (current window) | set on submit (last write wins); iterate-by-pair in EndBlock; delete-all after window |
| `0x02` | `{pair}` | `AggregatedPrice` (latest) | set in EndBlock (only if quorum met, D11); read by `GetPrice` |
| `0x03` | `{pair}/{ts_unixnano_be}` | `TWAPEntry` (history) | append in EndBlock; reverse-iterate in `GetTWAP`; prune > 24h |
| `0x04` | `{valoper}` | `int64` miss counter | inc / reset in EndBlock (tumbling, D3) |
| `0x05` | `{valoper}` | `int64` total windows | inc / reset in EndBlock (**per-validator**, D6; tumbling, D3) |
| `0x06` | `(empty)` | `OracleParams` | get / set |
| `0x07` | `{feeder_acc}` | `{valoper}` | set/overwrite on `MsgSetFeeder`; read on `SubmitFeed` (feeder → validator, D10) |
| `0x08` | `{valoper}` | `{feeder_acc}` | reverse index for `Feeder` query + genesis export (D10) |

Timestamps are big-endian `unixnano` for correct ordered iteration on `0x03`.

**Staleness & emptiness (D12).** `GetPrice(pair)`:
- returns `ErrPairNotAccepted` if `pair ∉ accept_list`,
- returns `ErrNoPrice` if `0x02[pair]` is empty (never aggregated),
- returns `ErrStalePrice` if `blockTime − stored.block_time > MaxPriceAge`,
- otherwise returns the price.

So Phase 3 attestation can distinguish "no feed yet" from "feed too old" from a live price, and can never attest against a stale value.

**TWAP formula (M3).** `GetTWAP(pair, window)` is a true **time-weighted** average over `0x03` entries with `block_time ∈ [now − window, now]`, sorted ascending: each entry `eᵢ` is weighted by the duration it was the prevailing price, `wᵢ = (eᵢ₊₁.time − eᵢ.time)` for interior entries and `(now − eₙ.time)` for the last; the partial leading interval is clamped at `now − window`. Result `= Σ(eᵢ.price · wᵢ) / Σ wᵢ`. If exactly one entry is in range it returns that entry's price; if none, `ErrNoTWAPData`. `GetTWAP` applies the same `MaxPriceAge` staleness check against the most recent entry. This is a frozen cross-phase contract (Phase 3).

---

## 8. Message Handlers (`keeper/msg_server.go`)

**`SetFeeder`** (signed by the operator — D10/C1):

1. `ValidateBasic`: valid `validator` valoper bech32; valid `feeder` account bech32.
2. Stateful: `validator` resolves to an existing validator via `GetValidator`. (Bonded status is *not* required to set a feeder.)
3. Write `0x07[{feeder}] = {valoper}` and `0x08[{valoper}] = {feeder}` (overwrite any prior delegation; a feeder address may serve at most one validator — reject if `0x07[{feeder}]` already maps to a *different* valoper).
4. Emit `oracle_feeder_set{validator, feeder}`.

**`SubmitFeed`** (signed by the feeder — D10/C1):

1. `ValidateBasic`: valid `feeder` and `validator` bech32; `pair` non-empty; `price` parses as a **positive** `LegacyDec`.
2. **Feeder authorization:** resolve the authorized feeder for `validator` — `0x07`/`0x08` if set, else the operator's own account address (implicit). The message `feeder` (signer) must equal it, else `ErrFeederNotAuthorized`.
3. Stateful: `pair ∈ accept_list` (`ErrPairNotAccepted`); `validator` resolves via `GetValidator` to a **bonded** validator with non-zero `GetLastValidatorPower` (`ErrNotBondedValidator`).
4. Write `0x01[{valoper}/{pair}] = OracleFeed{validator, pair, price, block_height}` (overwrite within window; keyed by valoper so the feeder identity never enters aggregation).
5. Emit `oracle_feed_submitted{validator, feeder, pair, price}`.

**`UpdateParams`:** authority check → `params.Validate()` → if `accept_list` changed, reset `0x04`/`0x05` (C2) → persist `0x06`. Emits `oracle_params_updated{accept_list_changed}`.

---

## 9. ABCI `EndBlock` (`keeper/abci.go`, `keeper/aggregate.go`)

```
EndBlocker(ctx):
  p = GetParams(ctx)
  if (height % p.vote_window) != 0: return                       # mid-window no-op

  # Build ONE deterministic valoper→Validator map for the whole pass (H4).
  vals        = StakingKeeper.GetBondedValidatorsByPower()       # sorted, deterministic
  valByOper   = { v.OperatorAddress: v for v in vals }
  totalBonded = StakingKeeper.TotalBondedTokens()                # quorum denominator (D11)
  quorumLive  = {}                                               # pairs that reached quorum this window

  # 1. Aggregate (quorum-gated) + outlier slash, per pair
  for pair in p.accept_list:                                     # stored order (deterministic)
      feeds = iterate 0x01 entries for pair                      # [(valoper, priceStr)]
      if feeds empty: continue
      submittedPower = 0; weighted = []; rawPrices = []
      for (valoper, priceStr) in feeds:
          price = parseDec(priceStr); if price == nil or price <= 0: continue   # M1: skip, never panic
          v = valByOper[valoper]; if v == nil: continue                          # unbonded mid-window → ignore (L2)
          pw = GetLastValidatorPower(valoper)
          weighted.append((price, pw)); rawPrices.append(price); submittedPower += pw

      if Dec(submittedPower) / Dec(totalBonded) < p.quorum_fraction:            # D11: below quorum → skip pair
          continue                                                              #   (no price, no TWAP, not a miss)
      quorumLive.add(pair)

      median    = WeightedMedian(weighted)                       # STAKE-weighted → the published price
      refMedian = UnweightedMedian(rawPrices)                    # COUNT-based   → outlier reference (D2/H1)
      SetAggregatedPrice(pair, median, height, blockTime)        # 0x02
      AppendTWAPEntry(pair, median, blockTime)                   # 0x03
      emit oracle_price_aggregated{pair, price=median}

      if refMedian > 0:                                          # L?: guard divide-by-zero (rawPrices are all > 0)
          for (valoper, price) in feeds-with-valid-price:
              if |price - refMedian| / refMedian > p.outlier_threshold:
                  slash(valByOper[valoper], p.outlier_slash_rate, "outlier")

  # 2. Miss tracking + slash, tumbling window (D3), only vs quorum-live pairs (D4/C2)
  for v in vals:                                                 # deterministic order
      submittedAllLive = (quorumLive ⊆ pairs v submitted this window)   # built by KV iteration, not maps
      total = inc 0x05[v]
      if not submittedAllLive: miss = inc 0x04[v]
      else:                    miss = get 0x04[v]
      if total >= p.miss_window_size:                            # tumbling boundary
          if Dec(miss)/Dec(total) > p.miss_threshold:
              slash(v, p.miss_slash_rate, "miss")
          reset 0x04[v]; reset 0x05[v]                           # reset every boundary, slashed or not

  # 3. Clear feeds for next window
  DeleteAllFeeds()                                               # 0x01

WeightedMedian(weighted):                                        # published price
  sort weighted ascending by (price, valoper)                    # ties broken by valoper for determinism (L3)
  half = totalWeight / 2
  walk accumulating weight; return first price with cumWeight >= half   # lower-side on exact half (documented, L3)

UnweightedMedian(prices):                                        # outlier reference (each validator = 1 vote)
  sort prices ascending; return prices[n/2] (lower-mid on even n)

slash(v, rate, reason):
  consAddr = v.GetConsAddr()                                     # from the Validator record in the map (H4)
  power    = GetLastValidatorPower(v.OperatorAddress)
  if power == 0: return                                          # nothing to slash (L2)
  SlashingKeeper.Slash(consAddr, rate, power, height)            # error only; NO Jail (D7); distributionHeight = height (L1)
  emit oracle_slash{validator, slash_reason=reason, slash_fraction=rate}
```

**Determinism guarantees:** `accept_list` iterated in stored order; the bonded set via `GetBondedValidatorsByPower` (sorted, with valoper tie-break); `0x01` iteration is over ordered KV keys; the `valByOper` map is built once but only *read* by key (never range-iterated) in the hot path; the `submittedAllLive` check is computed by deterministic KV iteration, not Go-map ranging. Outlier and miss slashes are independent (a validator may incur both in one window with distinct events).

**Panic-safety (M1):** every value read from the store in `EndBlock` (feed prices, params) is parsed defensively — an unparseable or non-positive feed is **skipped**, never fatal. Genesis-imported prices are validated at `InitGenesis` (§5.4). The only division guards (`totalBonded > 0`, `refMedian > 0`) are covered because a quorum-met pair has ≥1 positive-price feed and a non-zero bonded set.

**`distributionHeight` (L1):** slashing uses the current `height`; oracle infractions are attributed to "this window," and there is no historical-infraction lookback as with double-sign evidence. Documented so it isn't "corrected" to a past height later.

---

## 10. Queries, CLI, Events, Genesis

- **gRPC + autocli queries:** `vertixd q oracle price [pair]`, `q oracle twap [pair] [window]`, `q oracle params`, `q oracle miss-counter [valoper]`, `q oracle feeder [valoper]`.
- **Tx CLI:** `vertixd tx oracle set-feeder [feeder-addr]` (operator-signed; D10), `vertixd tx oracle submit-feed [validator] [pair] [price]` (feeder-signed; test/manual — the sidecar broadcasts in Phase 4).
- **Events (typed):**
  - `oracle_feeder_set{validator, feeder}`
  - `oracle_feed_submitted{validator, feeder, pair, price}`
  - `oracle_price_aggregated{pair, price}`
  - `oracle_slash{validator, slash_reason, slash_fraction}` (`slash_fraction` is the applied rate; the absolute burned amount is not returned by `x/slashing` — see §4 signature note)
  - `oracle_params_updated{accept_list_changed}`
- **Genesis:** `DefaultGenesis` = `DefaultParams` + no prices + no feeders; `InitGenesis` validates then sets params, any seeded prices, and feeder delegations; `ExportGenesis` dumps params + current `0x02` prices + `0x08` feeder delegations (counters/TWAP are runtime-rebuilt — see §5.4 and L4).

**Documented future hardening (Phase 7, not Phase 1):** MAD-based outlier detection (if legitimate feeds trip the fixed band even against the unweighted median); per-window **bitmap** miss tracking (last-N-windows sliding window, like `x/slashing`'s missed-block bitmap) for finer recency sensitivity than the tumbling `miss_window_size` used here.

---

## 11. Acceptance Gate Mapping

| Spec gate ([`full-design-spec.md`](../full-design-spec.md) Phase 1) | How satisfied |
|---|---|
| Feeds aggregate correctly across simulated validators | Keeper integration test: multi-validator window → stake-weighted median; **quorum test** asserts a pair below `quorum_fraction` produces no price (D11) |
| TWAP windows populate and expire | TWAP test: append over time, `GetTWAP` returns the time-weighted avg (M3), prune drops > 24h, `ErrStalePrice` past `max_price_age` (D12) |
| Slash events fire at correct thresholds | Miss test (no slash before `miss_window_size`; trips at boundary when rate > `MissThreshold`; **no mass-slash when a pair misses quorum**, C2); outlier test (band breach vs **unweighted** median, H1) → `oracle_slash` |
| `vertixd q oracle price [pair]` works | autocli `Price` query + CLI test; asserts `ErrNoPrice`/`ErrStalePrice` paths |
| `vertixd q oracle twap [pair] [window]` works | autocli `Twap` query + CLI test |
| **Feeder delegation works** (added, C1) | `MsgSetFeeder` then feeder-signed `MsgSubmitFeed` accepted; operator-signed submit rejected once a different feeder is set; implicit-operator path works with no delegation |
| Inherited Phase 0 gate (`make lint && make test && make build`, `buf lint/breaking`) | CI stays green with the new module + protos |

---

## 12. Cross-Phase Contracts Established / Honored Here

- **`OracleKeeper` interface** (`GetPrice`, `GetTWAP`) — exported for Phase 3 `x/rwa` attestation. Stable signature per §4. **Phase 3 MUST treat `ErrStalePrice` (D12) and `ErrNoPrice` as attestation failures** — a stale or absent price cannot move an asset `DRAFT → ATTESTED`.
- **`MsgSubmitFeed` wire format** (`feeder` signer, `validator`, `pair` as `BASE:QUOTE`, `price` as `LegacyDec` string) — the exact target for the Phase 4 feeder. **The feeder signs, not the operator** (C1/D10): the Phase 4 sidecar holds only the delegated feeder key, and operators must run `MsgSetFeeder` (or accept the implicit operator-as-feeder default) before feeds are accepted.
- **`BASE:QUOTE` pair convention** — shared by feeder submission and RWA attestation; validated by the `accept_list` regex.
- **Operational contract for Phase 4/6:** adding a pair to `accept_list` requires a coordinated feeder rollout; until submissions reach `quorum_fraction`, the pair produces no price and is excluded from miss accounting (C2) — so a governance pair-add is safe but does not yield a usable price until feeders ship it.
- **Inherited Phase 0 gates** — depinject wiring, `buf`/proto pipeline, CI/lint/test bar.

---

## 13. Risks & Open Items

**Resolved in this revision** (see §0 changelog for the mapping):

| Item | Was | Now |
|---|---|---|
| C1 Operator key hot on feeder | Operator-signed `MsgSubmitFeed` | Feeder delegation (`MsgSetFeeder`, `0x07`/`0x08`), feeder-signed submit (D10, §4/§5/§8) |
| C2 Whole-set slashing on one bad pair | Miss = "every pair" | Miss only vs **quorum-live** pairs; counter reset on `accept_list` change (D4/D11, §6/§9) |
| H1 Outlier slashes honest minority under stake capture | Band vs stake-weighted median | Band vs **unweighted** median; published price still stake-weighted (D2, §9) |
| H2 Single early miss → guaranteed slash | Cumulative + `MinWindowsBeforeSlash=10` | Tumbling `miss_window_size=500` (D3, §6/§9) |
| H3 Stale price attestable by Phase 3 | `GetPrice` had no freshness | `max_price_age` + `ErrStalePrice` (D12, §6/§7) |
| H4 `StakingKeeper` interface mismatch | `GetValidatorByConsAddr` listed | `GetValidator` + `TotalBondedTokens`; one `valoper→Validator` map per window (§4/§9) |
| M1 EndBlock panic on bad stored data | Unspecified | Defensive parse/skip; never fatal (§9) |
| M2 Genesis price validation | Unspecified | `InitGenesis` validates prices + feeders (§5.4) |
| M3 TWAP "time-weighted" undefined | Ambiguous | Exact duration-weighted formula (§7) |
| L1–L4 distributionHeight / zero-power / tie-bias / export-wipe | Undocumented | Documented in §9 and below |

**Residual risks (carried into the plan / Phase 7):**

| Item | Risk | Mitigation |
|---|---|---|
| Doc-sync to `technical-design.md` §2.2/§2.3/§2.4/§2.7/§5.5 (params, `Slash` signature, per-validator counter, event attrs, feeder model) | Source-of-truth drift | Land the edits this revision describes (done alongside this spec) |
| Quorum unreachable in early devnet (few validators, sparse feeds) | No price produced; RWA attestation blocked | `quorum_fraction` is governance-tunable; devnet may lower it; documented in Phase 6 |
| Tumbling `miss_window_size` detection latency (~7h) | Slow to punish a persistently-absent feeder | Acceptable for an economic (non-safety) penalty; bitmap sliding-window is the Phase-7 upgrade (§10) |
| Outlier band still trips legitimate feeds in extreme volatility (even vs unweighted median) | False-positive slashing | Governance-tunable `outlier_threshold`; MAD fallback (§10); `outlier_slash_rate` kept low (1%) |
| Stake-weighted median edge cases (single feed at quorum, equal weights, exact-half, valoper tie-break) | Edge-case correctness | Dedicated unit tests; lower-side bias on exact half is documented (L3) |
| EndBlock cost over large bonded sets (125 validators × pairs) | Block-time pressure | Bounded passes, single `valoper→Validator` map, no nested unbounded loops; benchmark in tests; pruning keeps `0x03` flat |
| Feeder key compromise | Attacker submits feeds for a validator | Blast radius limited to feeds (no fund access); operator can re-`SetFeeder` to rotate; outlier slash bounds manipulation |
| `submittedAllLive` set built non-deterministically | Consensus divergence | Built by ordered KV iteration over `0x01`, never Go-map ranging |

---

## 14. Definition of Done

- `x/oracle` builds and is depinject-wired in the `endBlockers` (`oracle` first of the custom suffix) and `genesisModuleOrder` (after `staking`).
- `MsgSetFeeder` delegates a feeder; `MsgSubmitFeed` is accepted only from the authorized **feeder** (or implicit operator) for a **bonded** validator on an `accept_list` pair; non-accepted pairs, non-bonded validators, and unauthorized feeders are rejected.
- EndBlock aggregates the **quorum-gated stake-weighted** median per pair (skipping pairs below `quorum_fraction`), appends + prunes TWAP, and slashes misses (tumbling `miss_window_size`) + outliers (vs the unweighted median) via `x/slashing` (no jail), emitting all five events — and never panics on malformed stored/imported data.
- `GetPrice`/`GetTWAP` enforce `ErrStalePrice`/`ErrNoPrice`/`ErrNoTWAPData` per D12.
- The five queries (`price`, `twap`, `params`, `miss-counter`, `feeder`) return correctly via gRPC/CLI.
- Genesis round-trips params, prices, and feeder delegations; counters/TWAP are runtime-rebuilt.
- Unit + keeper-integration + app-level tests pass (including quorum, staleness, feeder-auth, tumbling-miss, and unweighted-outlier cases); `make lint && make test && make build`, `buf lint`, and `buf breaking` stay green in CI.
- The `technical-design.md` doc-syncs (§13) are landed.
- Phase 1 acceptance gate (§11) satisfied — opening Phase 3 (price consumption) and Phase 4 (feeder target).

---

## Appendix A — Document Map

| Doc | Relationship to this spec |
|---|---|
| [`full-design-spec.md`](../full-design-spec.md) | Program source of truth; Phase 1 section is the parent of this spec |
| [`technical-design.md`](../technical-design.md) | Owns §2 oracle internals referenced here; §2.2/§2.4 receive doc-sync (§11) |
| [`2026-05-29-phase-0-chain-foundation-design.md`](./2026-05-29-phase-0-chain-foundation-design.md) | The depinject app + CI/proto gates this phase builds on |
| [`project-structure.md`](../project-structure.md) | Target layout for `x/oracle` |
| [`coding-standards.md`](../coding-standards.md) | Lint set, test patterns, proto additivity, genesis round-trip |
| `docs/plans/` | Step-by-step execution plan derived from this spec (next step) |
