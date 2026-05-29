# Phase 1 — `x/oracle` (On-Chain Aggregation + Slashing) — Design Spec

**Date:** 2026-05-29
**Status:** Approved (brainstorm output)
**Phase:** 1 of the canonical phase map ([`full-design-spec.md`](../full-design-spec.md) §7.2)
**Depends on:** Phase 0 (Chain Foundation); SDK `staking`, `slashing`, `params` · **Enables:** Phase 3 (`x/rwa` price consumption), Phase 4 (`vertix-feeder` target)
**Owns:** validator-submitted price feeds, stake-weighted-median aggregation, TWAP histories, miss/outlier slashing, and the `OracleKeeper` interface consumed by `x/rwa`.

> This is the per-phase design spec produced by the brainstorm of Phase 1. It refines — and must stay consistent with — the program source of truth ([`full-design-spec.md`](../full-design-spec.md) Phase 1) and the engineering reference ([`technical-design.md`](../technical-design.md) §2). Where this doc and the program spec disagree on phase order or cross-phase contracts, the program spec wins. Module internals defer to `technical-design.md`, except where this doc explicitly refines it (flagged as doc-sync items in §11). The step-by-step execution plan lives in `docs/plans/` (produced after this spec).

---

## 1. Goal & Deliverable Boundary

Deliver a working `x/oracle` Cosmos SDK module: validators submit per-pair prices via `MsgSubmitFeed`; at each `VoteWindow` boundary the module aggregates submissions into a **stake-weighted median** per pair, records **TWAP** history, and slashes validators for **misses** and **outliers** via `x/slashing`. The module exposes the `OracleKeeper` interface (`GetPrice`, `GetTWAP`) that `x/rwa` (Phase 3) consumes, and honors the `MsgSubmitFeed` wire format the `vertix-feeder` (Phase 4) targets.

**In scope:**

- Protobuf surface: `MsgSubmitFeed`, `MsgUpdateParams`, queries `Price`/`Twap`/`Params`/`MissCounter`, `OracleParams`, genesis.
- `EndBlock` aggregation (stake-weighted median), TWAP append + prune, miss + outlier slashing.
- Params + genesis + events + gRPC/CLI queries.
- The consumer `OracleKeeper` interface and `BASE:QUOTE` pair convention.
- Unit + keeper-integration + app-level tests satisfying the Phase 1 acceptance gate.

**Out of scope (deferred):**

- The off-chain `vertix-feeder` binary (Phase 4).
- RWA price *consumption* / attestation logic (Phase 3).
- Cross-chain oracle queries (post-mainnet, per program spec §1).
- MAD-based outlier detection and sliding-window miss tracking (documented future enhancements, §10).

---

## 2. Locked Decisions (from this brainstorm)

| # | Decision | Choice | Rationale |
|---|---|---|---|
| D1 | Module creation | **`ignite scaffold module oracle --dep bank,staking,slashing`, then adapt** | Consistent with Phase 0 (D4); inherits depinject wiring, the `buf`/`make proto-gen` pipeline, autocli, and scaffold layout; slots into the existing `app_config.go`/`app.go` markers without re-architecting. The `technical-design.md` §7 `NewKeeper(...)` snippets remain *illustrative* (as Phase 0 D1 established); the binding contract is the module set, ordering, and the `OracleKeeper` interface. |
| D2 | Outlier detection | **Fixed-band deviation vs. the window's stake-weighted median**, governed by a new `OutlierThreshold` param (default `0.05`) | Detection is **cross-sectional** (each submission vs. the same window's median), so the usual "ignores volatility" objection to fixed bands does not apply — all compared submissions are for the same instant. Fully deterministic, gas-cheap (one `LegacyDec` divide + compare per submission), and governance-tunable. MAD is a documented future upgrade (§10). |
| D3 | Miss semantics | **Rolling per-validator counters**: misses + total windows since last reset; slash + reset when `misses/total > MissThreshold` after a grace period | Fits the locked `0x04`/`0x05` store slots; deterministic and bounded (one pass over the bonded set); reset-on-slash prevents repeated punishment for the same accumulated history. A new `MinWindowsBeforeSlash` param (default `10`) provides the grace period. |
| D4 | Miss definition | A bonded validator **misses a window** if it did not submit a valid feed for **every** pair in `accept_list` during that window | Clean binary per `(validator, window)`; matches the feeder operational contract (§5.5 of `technical-design.md`: one feeder submits all pairs each window); avoids fractional per-pair bookkeeping. |
| D5 | TWAP mechanics | **Append one entry per aggregation; `GetTWAP` reverse-iterates `[now-window, now]`; prune entries older than 24h at each window-end** | Fits the time-ordered `0x03` layout; growth is bounded and small (~1,700 entries/pair for 24h at ~50s/window); deterministic and easy to test. Cumulative-accumulator (Uniswap-V2 style) adds `LegacyDec` overflow handling and checkpointing for no benefit at this scale. |
| D6 | `0x05` granularity | **Per-validator** total-windows counter keyed by `{valoper}` (refines `technical-design.md` §2.4's global counter) | A correct miss-*rate* requires per-validator totals because validators bond at different heights. Flagged as a doc-sync (§11). |
| D7 | No jailing | Oracle slashing calls `SlashingKeeper.Slash` only — **never `Jail`** | Per `technical-design.md` §2.6: oracle slashing is intentionally lighter than double-sign to avoid validator chilling / accidental jailing. |
| D8 | No module account | `x/oracle` holds **no funds**; slashing is delegated to `x/slashing` | No `maccPerms`/`blockAccAddrs` entry needed; reduces attack surface. |
| D9 | Build sequencing | **Approach A — one phase spec/plan, vertical slices in dependency order**, slashing last | Only approach that satisfies the Phase 1 acceptance gate (slash events firing) in one pass; risk isolation achieved by ordering + independent slashing tests. |

These inherit and must not contradict the program-level locked decisions in [`full-design-spec.md`](../full-design-spec.md) §§1–6, nor the architectural invariants §5.

---

## 3. Target Module Layout (Phase 1 outputs)

```
x/oracle/
├── keeper/
│   ├── keeper.go            store ops: feeds, aggregated price, TWAP, miss/total counters, params
│   ├── msg_server.go        SubmitFeed, UpdateParams
│   ├── grpc_query.go        Price, Twap, Params, MissCounter
│   ├── abci.go              EndBlocker: aggregate → TWAP → outlier slash → miss slash → clear
│   ├── aggregate.go         WeightedMedian + outlier band check
│   ├── genesis.go           InitGenesis / ExportGenesis
│   └── *_test.go
├── types/
│   ├── keys.go              store prefixes 0x01–0x06
│   ├── params.go            OracleParams defaults + Validate()
│   ├── msgs.go              ValidateBasic (MsgSubmitFeed, MsgUpdateParams)
│   ├── expected_keepers.go  StakingKeeper, SlashingKeeper
│   ├── errors.go            ErrNoPrice, ErrNoTWAPData, ErrPairNotAccepted, ErrNotBondedValidator, ...
│   ├── events.go, codec.go, genesis.go
│   └── *.pb.go              generated
├── module/
│   ├── module.go            AppModule (EndBlock, genesis, services)
│   ├── depinject.go         ProvideModule (inputs: staking, slashing keepers; authority)
│   └── autocli.go           query + tx command wiring
proto/vertix/oracle/v1/
├── types.proto   OracleFeed, AggregatedPrice, TWAPEntry, OracleParams
├── tx.proto      MsgSubmitFeed, MsgUpdateParams (+ responses)
├── query.proto   Price, Twap, Params, MissCounter
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
    GetValidatorByConsAddr(ctx context.Context, addr sdk.ConsAddress) (stakingtypes.Validator, error)
    GetLastValidatorPower(ctx context.Context, addr sdk.ValAddress) (int64, error)
    GetBondedValidatorsByPower(ctx context.Context) ([]stakingtypes.Validator, error)
}

type SlashingKeeper interface {
    Slash(ctx context.Context, consAddr sdk.ConsAddress, fraction math.LegacyDec, power, distributionHeight int64) (math.Int, error)
}
```

(`Jail` is intentionally excluded — D7.)

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
  int64           vote_window              = 1;  // default 10 (blocks per window)
  string          miss_threshold           = 2 [(cosmos_proto.scalar) = "cosmos.Dec"];  // "0.05"
  string          miss_slash_rate          = 3 [(cosmos_proto.scalar) = "cosmos.Dec"];  // "0.005"
  string          outlier_slash_rate       = 4 [(cosmos_proto.scalar) = "cosmos.Dec"];  // "0.01"
  string          outlier_threshold        = 5 [(cosmos_proto.scalar) = "cosmos.Dec"];  // "0.05" (NEW)
  int64           min_windows_before_slash = 6;  // default 10 (NEW — miss grace)
  repeated string accept_list              = 7;  // VTX:USD, BTC:USD, ETH:USD, ATOM:USD, USDC:USD
}
```

### 5.2 `tx.proto`

```protobuf
service Msg {
  rpc SubmitFeed(MsgSubmitFeed)     returns (MsgSubmitFeedResponse);
  rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);
}

message MsgSubmitFeed {
  option (cosmos.msg.v1.signer) = "validator";
  string validator = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string pair      = 2;
  string price     = 3 [(cosmos_proto.scalar) = "cosmos.Dec"];
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
}
// Price(pair) → AggregatedPrice
// Twap(pair, window_seconds) → { price }
// Params() → OracleParams
// MissCounter(validator) → { int64 misses; int64 total_windows }
```

### 5.4 `genesis.proto`

```protobuf
message GenesisState {
  OracleParams              params = 1 [(gogoproto.nullable) = false];
  repeated AggregatedPrice  prices = 2 [(gogoproto.nullable) = false];
}
```

In-flight feeds (`0x01`), TWAP history (`0x03`), and miss/total counters (`0x04`/`0x05`) are **not** exported — they are runtime-rebuilt. Only params + last aggregated prices round-trip, keeping `TestGenesisRoundTrip` deterministic.

---

## 6. Params & Validation (`types/params.go`)

`DefaultParams()`:

| Param | Default |
|---|---|
| `vote_window` | `10` |
| `miss_threshold` | `0.05` |
| `miss_slash_rate` | `0.005` |
| `outlier_slash_rate` | `0.01` |
| `outlier_threshold` | `0.05` |
| `min_windows_before_slash` | `10` |
| `accept_list` | `["VTX:USD","BTC:USD","ETH:USD","ATOM:USD","USDC:USD"]` |

`Validate()` enforces:

- `vote_window > 0`; `min_windows_before_slash >= 0`.
- `miss_threshold`, `outlier_threshold` are valid `LegacyDec` in `(0, 1]`.
- `miss_slash_rate`, `outlier_slash_rate` are valid `LegacyDec` in `[0, 1)` and **`< 0.05`** (the `slash_fraction_double_sign` ceiling — keeps oracle slashing lighter than double-sign per §2.6).
- `accept_list` non-empty; each entry matches `^[A-Z0-9]+:[A-Z0-9]+$`; no duplicates.

`MsgUpdateParams` re-runs `Validate()` in-handler before persisting; `authority` must equal the gov module address.

---

## 7. State & Keeper Store Ops (`types/keys.go`, `keeper/keeper.go`)

| Prefix | Key | Value | Operations |
|---|---|---|---|
| `0x01` | `{valoper}/{pair}` | `OracleFeed` (current window) | set on submit (last write wins); iterate-by-pair in EndBlock; delete-all after window |
| `0x02` | `{pair}` | `AggregatedPrice` (latest) | set in EndBlock; read by `GetPrice` |
| `0x03` | `{pair}/{ts_unixnano_be}` | `TWAPEntry` (history) | append in EndBlock; reverse-iterate in `GetTWAP`; prune > 24h |
| `0x04` | `{valoper}` | `int64` miss counter | inc / reset in EndBlock |
| `0x05` | `{valoper}` | `int64` total windows | inc / reset in EndBlock (**per-validator**, D6) |
| `0x06` | `(empty)` | `OracleParams` | get / set |

Timestamps are big-endian `unixnano` for correct ordered iteration on `0x03`. `GetPrice` returns `ErrNoPrice` and `GetTWAP` returns `ErrNoTWAPData` when empty, so Phase 3 attestation can distinguish "no feed yet" from a zero price. `GetTWAP(pair, window)` computes the time-weighted average over `0x03` entries in `[blockTime-window, blockTime]`; if only one entry exists in range it returns that entry's price.

---

## 8. Message Handlers (`keeper/msg_server.go`)

**`SubmitFeed`:**

1. `ValidateBasic`: valid `validator` valoper bech32; `pair` non-empty; `price` parses as a **positive** `LegacyDec`.
2. Stateful: `pair ∈ accept_list`; signer resolves to a **bonded** validator (operator lookup + non-zero `GetLastValidatorPower`). Reject with `ErrPairNotAccepted` / `ErrNotBondedValidator` otherwise.
3. Write `0x01[{valoper}/{pair}] = OracleFeed{...,block_height}` (overwrite within window).
4. Emit `oracle_feed_submitted{validator, pair, price}`.

**`UpdateParams`:** authority check → `params.Validate()` → persist `0x06`. Emits a param-update event.

---

## 9. ABCI `EndBlock` (`keeper/abci.go`, `keeper/aggregate.go`)

```
EndBlocker(ctx):
  p = GetParams(ctx)
  if (height % p.vote_window) != 0: return                       # mid-window no-op

  # 1. Aggregate + outlier slash, per pair
  for pair in p.accept_list:                                     # stored order (deterministic)
      feeds = iterate 0x01 entries for pair                      # [(valoper, price)]
      if feeds empty: continue
      weighted = [(price, power(valoper)) for each feed]         # power via StakingKeeper
      median   = WeightedMedian(weighted)
      SetAggregatedPrice(pair, median, height, blockTime)        # 0x02
      AppendTWAPEntry(pair, median, blockTime)                   # 0x03
      emit oracle_price_aggregated{pair, price=median}
      for (valoper, price) in feeds:
          if median > 0 and |price - median| / median > p.outlier_threshold:
              slash(valoper, p.outlier_slash_rate, "outlier")

  # 2. Miss tracking + slash, over the bonded set
  submittedAll = set of validators that submitted every accept_list pair this window
  for v in StakingKeeper.GetBondedValidatorsByPower():           # deterministic order
      total = inc 0x05[v]
      if v in submittedAll: miss = get 0x04[v]
      else:                 miss = inc 0x04[v]
      if total >= p.min_windows_before_slash and Dec(miss)/Dec(total) > p.miss_threshold:
          slash(v, p.miss_slash_rate, "miss")
          reset 0x04[v]; reset 0x05[v]

  # 3. Clear feeds for next window
  DeleteAllFeeds()                                               # 0x01

WeightedMedian(weighted):
  sort weighted ascending by price
  half = totalWeight / 2
  walk accumulating weight; return first price with cumWeight >= half

slash(valoper, rate, reason):
  consAddr = consensus address of valoper
  power    = GetLastValidatorPower(valoper)
  SlashingKeeper.Slash(consAddr, rate, power, height)            # NO Jail (D7)
  emit oracle_slash{validator, slash_reason=reason, slash_amount}
```

**Determinism guarantees:** `accept_list` iterated in stored order; bonded set via `GetBondedValidatorsByPower` (sorted); `0x01` iteration is over ordered KV keys; no Go-map iteration in the consensus hot path. Outlier and miss slashes are independent (a validator may incur both in one window with distinct events).

---

## 10. Queries, CLI, Events, Genesis

- **gRPC + autocli queries:** `vertixd q oracle price [pair]`, `q oracle twap [pair] [window]`, `q oracle params`, `q oracle miss-counter [valoper]`.
- **Tx CLI:** `vertixd tx oracle submit-feed [pair] [price]` (test/manual; the feeder broadcasts in Phase 4).
- **Events (typed):** `oracle_feed_submitted{validator,pair,price}`, `oracle_price_aggregated{pair,price}`, `oracle_slash{validator,slash_reason,slash_amount}`.
- **Genesis:** `DefaultGenesis` = `DefaultParams` + no prices; `InitGenesis` sets params (+ any seeded prices); `ExportGenesis` dumps params + current `0x02` prices.

**Documented future enhancements (not in Phase 1):** MAD-based outlier detection (if legitimate feeds trip the fixed band); sliding-window miss tracking (bitmap of last N windows) for recency sensitivity.

---

## 11. Acceptance Gate Mapping

| Spec gate ([`full-design-spec.md`](../full-design-spec.md) Phase 1) | How satisfied |
|---|---|
| Feeds aggregate correctly across simulated validators | Keeper integration test: multi-validator window → stake-weighted median |
| TWAP windows populate and expire | TWAP test: append over time, `GetTWAP` returns weighted avg, prune drops > 24h |
| Slash events fire at correct thresholds | Miss test (trips after grace at `MissThreshold`); outlier test (band breach) → `oracle_slash` |
| `vertixd q oracle price [pair]` works | autocli `Price` query + CLI test |
| `vertixd q oracle twap [pair] [window]` works | autocli `Twap` query + CLI test |
| Inherited Phase 0 gate (`make lint && make test && make build`, `buf lint/breaking`) | CI stays green with the new module + protos |

---

## 12. Cross-Phase Contracts Established / Honored Here

- **`OracleKeeper` interface** (`GetPrice`, `GetTWAP`) — exported for Phase 3 `x/rwa` attestation. Stable signature per §4.
- **`MsgSubmitFeed` wire format** (`validator`, `pair` as `BASE:QUOTE`, `price` as `LegacyDec` string) — the exact target for the Phase 4 feeder.
- **`BASE:QUOTE` pair convention** — shared by feeder submission and RWA attestation; validated by the `accept_list` regex.
- **Inherited Phase 0 gates** — depinject wiring, `buf`/proto pipeline, CI/lint/test bar.

---

## 13. Risks & Open Items

| Item | Risk | Mitigation |
|---|---|---|
| `0x05` per-validator refinement vs `technical-design.md` §2.4 (global) | Doc divergence | D6; doc-sync `technical-design.md` §2.4 to per-validator |
| New params (`outlier_threshold`, `min_windows_before_slash`) vs §2.2 `OracleParams` | Doc divergence | Doc-sync `technical-design.md` §2.2 to add both fields |
| valoper → consAddr resolution for slashing | Wrong address fails or mis-slashes | Resolve via validator record consensus pubkey; unit-test the mapping; integration test asserts the correct validator is slashed |
| Stake-weighted median with one feed / equal weights / ties | Edge-case correctness | Dedicated unit tests for single feed, equal weights, exact-half boundary |
| Outlier band tripping legitimate feeds in volatile/illiquid pairs | False-positive slashing | Cross-sectional comparison (same window) + governance-tunable `outlier_threshold`; MAD documented as fallback (§10) |
| EndBlock cost over large bonded sets (125 validators × pairs) | Block-time pressure | Bounded passes, no nested unbounded loops; benchmark in tests; pruning keeps `0x03` flat |
| `LegacyDec` division by zero when median = 0 | Panic | Guard `median > 0` before outlier ratio; reject non-positive prices at submit |
| Feeds map for "submitted all pairs" check | Non-deterministic if map-iterated | Build the `submittedAll` set by deterministic KV iteration, not map ranging in consensus path |

---

## 14. Definition of Done

- `x/oracle` builds and is depinject-wired in the `endBlockers` (`oracle` first of the custom suffix) and `genesisModuleOrder` (after `staking`).
- `MsgSubmitFeed` accepts bonded-validator submissions for `accept_list` pairs; non-accepted pairs / non-bonded signers are rejected.
- EndBlock aggregates the stake-weighted median per pair, appends + prunes TWAP, and slashes misses + outliers via `x/slashing` (no jail), emitting the three events.
- The four queries return correctly via gRPC/CLI.
- Unit + keeper-integration + app-level tests pass; `make lint && make test && make build`, `buf lint`, and `buf breaking` stay green in CI.
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
