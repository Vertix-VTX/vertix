# Phase 7 — Security Hardening — Design Spec

**Date:** 2026-05-29
**Status:** Approved (brainstorm output)
**Phase:** 7 of the canonical phase map ([`full-design-spec.md`](../full-design-spec.md) §7.2)
**Depends on:** Phase 6 (multi-validator devnet stack) · **Enables:** Phase 8 (public testnet v1)
**Owns:** the audit-readiness layer for the three custom modules — permanent `x/crisis` invariants, a simulation layer that actually exercises them, adversarial/property/fuzz coverage of the economic attack surface, a gas / unbounded-loop audit, CI static analysis (CodeQL), a documented `tm-load-test` throughput baseline, and validator security operations docs (`tmkms`, sentry/key-separation).

> This is the per-phase design spec produced by the brainstorm of Phase 7. It refines — and must stay consistent with — the program source of truth ([`full-design-spec.md`](../full-design-spec.md) Phase 7). Where this doc and the program spec disagree on phase order or cross-phase contracts, the program spec wins. Module internals defer to [`technical-design.md`](../technical-design.md). The step-by-step execution plan lives in `docs/plans/` (produced after this spec).
>
> **Canonical phase note.** Per [`full-design-spec.md`](../full-design-spec.md) §7.2, "Phase 7" is **Security Hardening**. The historical `roadmap.md` numbering calls security hardening "Phase 6" and public testnet "Phase 7"; the program spec's canonical ordering supersedes that numbering, and this spec uses it (devnet = Phase 6, security hardening = Phase 7, public testnet = Phase 8).

---

## 1. Goal & Deliverable Boundary

Make the three custom modules (`x/oracle`, `x/rwa`, `x/fees`) **audit-ready**: register permanent on-chain invariants, drive them with a simulation layer, prove the economic attack surface is closed with adversarial/property tests, document gas bounds, wire static analysis into CI, establish a load-test throughput baseline, and ship validator security guidance.

This phase is **hardening, not feature work**. It adds no protocol messages and no module economics. The **only** state additions are two internal `x/fees` KV keys required to make the fee-reconciliation invariant checkable (§3.2) — additive, no proto changes.

**In scope (7 workstreams):**

1. **Crisis invariants** — complete `x/oracle` + `x/fees` (RWA's `bonds`/`denoms` already ship from Phase 3).
2. **Module simulations** — real randomized `GenerateGenesisState` (all 3 modules) + `WeightedOperations` (`x/oracle`, `x/rwa`) + store decoders; reuse the existing `app/sim_test.go` harness.
3. **Adversarial + property + fuzz tests** — oracle manipulation, RWA bond bypass, fee determinism.
4. **Gas / unbounded-loop audit** — bounded-iteration review + guard tests + `docs/gas-audit.md`.
5. **CI** — CodeQL (Go) + a non-blocking simulation job + Makefile sim targets.
6. **Load test** — `tm-load-test` against the Phase 6 devnet + documented baseline (`docs/load-test.md`).
7. **Validator security docs** — `docs/tmkms.md`, `docs/validator-setup.md`.

**Out of scope (deferred):**

- **Third-party audit + fix integration** — Phase 9.
- **External validators, public faucet, bug bounty, production monitoring dashboards** — Phase 8.
- **`docs/validator-onboarding.md` and `docs/rwa-quickstart.md`** — Phase 8.
- **A hard TPS *threshold* / pass-fail load gate** — Phase 9 (this phase only establishes a documented baseline).
- **Any change to module economics or the protobuf message surface** — except the additive `x/fees` burn-accounting KV state in §3.2 (no proto change).

**Execution order (Approach A — risk-ordered, correctness-first):** (1) invariants → (2) simulation layer → (3) adversarial/property/fuzz → (4) gas audit → (5) CI (CodeQL + sim job) → (6) load test → (7) security docs. Each step builds on the prior; the headline gate ("no invariant violations under simulation") is reached early and then continuously protected.

---

## 2. Current State (Baseline)

| Area | State at start of Phase 7 |
|---|---|
| `x/crisis` | Wired in `app/app.go`; `ModuleManager.RegisterInvariants(app.CrisisKeeper)` called. |
| `x/rwa` invariants | **Done** (Phase 3): `bonds`, `denoms` in `x/rwa/keeper/invariants.go`. |
| `x/oracle` invariants | **Empty stub** — `AppModule.RegisterInvariants` is a no-op. |
| `x/fees` invariants | **Empty stub** — `AppModule.RegisterInvariants` is a no-op; keeper stores **only params** (no burn accounting). |
| Module `simulation.go` | Ignite stubs: `GenerateGenesisState` sets only `DefaultParams`; `WeightedOperations` empty; `simulation/helpers.go` are 15-line stubs. |
| App sim harness | `app/sim_test.go` (~430 lines) + `app/sim_bench_test.go` present (standard simapp harness). |
| CI | `build`, `test`, `lint`, `proto`, `genesis`, `e2e`. **No** CodeQL, sim, or load-test jobs. |
| Makefile | Has `bench`. **No** `sim` or `load-test` targets. |
| Security docs | `docs/tmkms.md`, `docs/validator-setup.md` **do not exist**. |

---

## 3. Crisis Invariants

Invariants are permanent safety properties registered with `x/crisis`. They must be **cheap** (no unbounded scans), **correct** (no false halts), and **non-vacuous** (driven by the simulation layer in §4).

### 3.1 `x/oracle` — `prices` invariant

**Decision (D1): register the structural-integrity form, not the existence form.**

The program spec's loose phrasing — *"aggregate prices exist for all configured pairs"* — is **not** a valid crisis invariant:

- A freshly-started chain has **no** aggregated prices until the first `VoteWindow` closes, so an existence invariant would halt the chain immediately after genesis.
- Price existence is **liveness-dependent** (it requires feeders to be up and a quorum to submit), not a safety property. Safety invariants must hold unconditionally given valid state.

Instead, register `oracle/prices` asserting **structural integrity** of whatever prices *do* exist:

- Every stored `AggregatedPrice` has `pair ∈ Params.AcceptList` — no orphaned or unconfigured prices linger (e.g. after a pair is removed from the accept-list).
- Every stored price parses to a **strictly positive** `math.LegacyDec`.

Bound: O(stored prices) ≤ O(`AcceptList`). Existence/freshness of feeds is handled as **monitoring** (Phase 8 dashboards), not a halt condition. This refines the spec wording (doc-sync §10).

Implementation: `x/oracle/keeper/invariants.go` + wire `RegisterInvariants(ir, k)` in `x/oracle/module/module.go`. Iterate the `0x02` aggregated-price prefix; read `Params.AcceptList` once into a set.

### 3.2 `x/fees` — `reconcile` and `module-balance` invariants

Per the Phase 2 design spec §12, Phase 7 owns these two statements. Today `x/fees` stores only params, so we add the minimal state needed to check reconciliation.

**Decision (D2): add two internal KV keys — no proto change.**

- `KeyGenesisSupply` → `uvtx` total supply snapshotted at `x/fees` `InitGenesis` via `bankKeeper.GetSupply("uvtx")`.
- `KeyCumulativeBurned` → `math.Int`, started at zero in `InitGenesis`, incremented in `EndBlocker` by each `burnAmt`.

These are **runtime KV values, not genesis fields** — on chain export/import the new baseline re-snapshots `KeyGenesisSupply` and resets `KeyCumulativeBurned` to zero, so reconciliation stays correct from the new baseline without any genesis/proto field. (If a future upgrade must preserve the lifetime burn total across export, that becomes an explicit migration — out of scope here.)

**Ordering dependency:** `bank` `InitGenesis` must run before `x/fees` `InitGenesis` so the supply snapshot is populated. This already holds in `app.go`'s `OrderInitGenesis`; the plan asserts it with a test.

Register two routes:

- `fees/reconcile`: `GenesisSupply − bankKeeper.GetSupply("uvtx") == CumulativeBurned`. Because the only sink for `uvtx` is the `x/fees` burn (there is no `x/mint`; `rwa/{id}` factory denoms are *different* denoms), this equality simultaneously enforces the **21M hard-cap / no-inflation** invariant (Architectural Invariant 1). Monotonicity of burn is structural — the counter only ever increments.
- `fees/module-balance`: the `x/fees` module account holds **zero** `uvtx` at the block boundary (it holds coins only transiently inside `EndBlocker`: collector → fees module → burn, all within one block). This is statement (b).

Bound: both O(1). Implementation: `x/fees/keeper/invariants.go`, keeper getters/setters for the two keys, an `EndBlocker` increment, and `RegisterInvariants` wired in `x/fees/module/module.go`.

### 3.3 `x/rwa` — already registered (no change)

`rwa/bonds` (module `uvtx` balance ≥ Σ non-settled bonds; every `ACTIVE` asset bonded) and `rwa/denoms` (pre-mint assets have zero factory-denom supply) ship from Phase 3 and are O(active assets). Phase 7 adds **simulation coverage** (§4) and an **adversarial test** (§5) for them but changes no code.

### 3.4 Invariant route summary (the permanent cross-phase contract)

| Module | Route | Statement | Bound |
|---|---|---|---|
| oracle | `prices` | stored prices are accept-listed + strictly positive | O(AcceptList) |
| fees | `reconcile` | `genesisSupply − supply(uvtx) == cumulativeBurned` (⇒ 21M cap) | O(1) |
| fees | `module-balance` | fees module account `uvtx` == 0 at block boundary | O(1) |
| rwa | `bonds` | module balance ≥ Σ non-settled bonds; ACTIVE ⇒ bonded | O(active assets) |
| rwa | `denoms` | pre-mint assets have zero factory-denom supply | O(active assets) |

---

## 4. Simulation Layer

Replace the Ignite stubs so the SDK simulator drives the custom modules into non-trivial, invariant-relevant state. Scope = **pragmatic** (per brainstorm): real randomized genesis everywhere; weighted ops only for state-creating messages.

### 4.1 Randomized genesis (`GenerateGenesisState`, all 3 modules)

Emit **valid** randomized params so import/export and param-edge paths are exercised:

- `x/oracle`: random `VoteWindow`, `MissThreshold`, slash rates, `OutlierThreshold`, `MissWindowSize`, `QuorumFraction`, `MaxPriceAge` within their validated bounds; a small random `AcceptList` (1–3 pairs). Optionally seed a few feeder delegations.
- `x/fees`: random `burn_ratio` in `[0,1]` with `burn_ratio + distribution_ratio == 1` (honor the ratio invariant).
- `x/rwa`: random `MinIssuerBond` and `MintFeeRate` within validated bounds.

This is what makes the invariants of §3 **non-vacuous** under simulation.

### 4.2 Weighted operations

| Module | Operations | Notes |
|---|---|---|
| `x/oracle` | `SimulateMsgSetFeeder`, `SimulateMsgSubmitFeed` | A bonded validator delegates a feeder, which submits a plausible randomized price for an `AcceptList` pair. |
| `x/rwa` | `SimulateMsgRegisterAsset` → `AttestAsset` → `MintRWA` → `TransferRWA` → `SettleRWA`; occasional `UpdateRestrictions` | Lifecycle-aware: ops pick assets in a valid prior state; bond funding via `simtypes` spendable-coins selection. `AttestAsset` requires a live oracle price (skip/no-op if none aggregated yet). |
| `x/fees` | **none** | `EndBlock`-only; exercised every simulated block automatically. `UpdateParams` is governance-gated and contributes little invariant coverage — excluded. |

`SlashBond`/`UpdateParams` for `x/rwa` are governance/authority messages; excluded from the weighted op set (low invariant value, high fragility), consistent with the pragmatic decision. They remain covered by unit tests.

### 4.3 Store decoders

Implement `RegisterStoreDecoder` for each module so a failed simulation pretty-prints the offending key/value diff. Cover the major prefixes: oracle prices/feeds/feeders/miss-counters; rwa asset records + allow/deny membership; fees params + the two new burn-accounting keys.

### 4.4 Harness reuse

The existing `app/sim_test.go` entrypoints (`TestFullAppSimulation`, `TestAppImportExport`, `TestAppSimulationAfterImport`, non-determinism) are reused **unchanged** — they automatically pick up the new genesis generators, ops, and decoders via the module manager. `app/sim_bench_test.go` likewise.

---

## 5. Adversarial, Property & Fuzz Tests

Hand-written tests (not the random simulator) targeting the specific economics. These encode "think like an attacker against your own invariants."

**Oracle manipulation:**

- A single large-stake validator submits an extreme price; assert the **unweighted-median** outlier reference (Phase 1 design) prevents stake-capture inversion and that the outlier slash fires.
- Miss-window gaming: a feeder submits exactly at the `QuorumFraction`/`MissThreshold` boundary; assert miss accounting and slashing behave at the edges.
- Stale-price attestation: a price older than `MaxPriceAge` is rejected at the `x/rwa` attestation gate (`ErrStalePrice`).

**RWA bond bypass:**

- `MintRWA` / transition to `ACTIVE` without `bond ≥ MinIssuerBond` is rejected.
- `SettleRWA` cannot reclaim the bond while obligations/active supply remain.
- A restricted-asset transfer cannot escape the `SendRestrictionFn` via `authz.MsgExec`, `MsgMultiSend`, or IBC transfer escrow — re-asserts the Phase 3 D5 single-chokepoint claim.

**Fee determinism:**

- `MintRWA` fee is a pure function of notional, independent of oracle price or staleness (re-asserts Phase 3 D2).
- `EndBlock` burn/distribute split is deterministic across replays; module account nets to zero.

**Fuzz (Go `testing/fuzz`):** oracle median/aggregation over random submission sets (no panic, output within input range); `burn_ratio` decimal math over random ratios (split sums exactly, no negative/overflow).

---

## 6. Gas / Unbounded-Loop Audit

- **Review and document** every hot-path iteration in the three modules: oracle `EndBlocker` aggregation (bounded by active validator set × `AcceptList`), miss accounting, RWA registry access, and the invariants themselves.
- **Guard tests** asserting the bounds hold: transfer-restriction checks are O(1) `Has` lookups (Phase 3 D4 keyed membership, not list enumeration); no message handler iterates the entire asset registry; invariants iterate only bounded prefixes.
- **Output:** `docs/gas-audit.md` — a concise table: each iteration site, its bound, worst-case input, and justification that it is acceptable for launch. This is the artifact an external auditor reads first.

---

## 7. CI Integration

**CodeQL (`.github/workflows/codeql.yml`):** GitHub `codeql-action` for Go, `security-extended` query suite (plus the cosmos community query pack if resolvable). Runs on PR + a weekly schedule. **Advisory** — uploads alerts to the Security tab; not a hard merge gate initially. Matches the acceptance gate "findings resolved or documented."

**Simulation job (extend `.github/workflows/ci.yml`):** a `sim` job running `make test-sim-nondeterminism` and a bounded `make test-sim-fullapp` (small block/seed counts for CI time budget). **Non-blocking on PRs**; a fuller run on the weekly schedule. Rationale: full simulations are time-variable; gating every PR on them is brittle.

**Makefile targets:** `test-sim-fullapp`, `test-sim-nondeterminism`, `test-sim-import-export` — thin wrappers over the existing `app/sim_test.go` entrypoints with sensible default flags.

---

## 8. Load Test (Baseline Only)

Per the brainstorm, Phase 7 establishes and **documents** a baseline; it sets **no** pass/fail threshold (resolves Appendix C "TPS target" for Phase 7 as *baseline-only*; the hard target is a Phase 9 concern).

- Driver: `tm-load-test` against the **Phase 6 3-validator devnet** (`make localnet-up`).
- New `infra/loadtest/` holding the runner config/script; `make load-test` target to execute it.
- **Output:** `docs/load-test.md` — method, hardware/topology, transaction mix, and the measured **baseline**: sustained TPS, p50/p99 broadcast-to-commit latency, and observed block time. Includes a short "how to re-run" section so Phase 9 can compare against mainnet-equivalent params.

---

## 9. Validator Security Docs

- **`docs/tmkms.md`** — `tmkms` setup for consensus-key isolation: supported backends (softsign/HSM/YubiHSM), `tmkms.toml` example for `vertix-1`, connection to `vertixd` via the privval socket, and failover guidance. **Vertix-specific note:** the **feeder key is separate** from the consensus key and from the operator key — `tmkms` protects only the consensus key; the feeder uses its own delegated key (Phase 1 feeder-delegation), and the operator key never lives on the feeder host.
- **`docs/validator-setup.md`** — sentry-node architecture (validator behind sentries, `pex`/`persistent_peers`/`private_peer_ids` config), firewall/port guidance, the three-key separation model (consensus vs feeder vs operator), and the monitoring hooks Phase 8 will build on. (External-validator **onboarding** and the RWA **quickstart** remain Phase 8.)

---

## 10. Deliverables, Acceptance Gate, Cross-Phase Contract, Doc-Syncs

### 10.1 Deliverables

- `x/oracle/keeper/invariants.go` (`prices`) + wired in `x/oracle/module/module.go`.
- `x/fees/keeper/invariants.go` (`reconcile`, `module-balance`) + two burn-accounting KV keys + `EndBlocker` increment + `InitGenesis` snapshot + wired in `x/fees/module/module.go`.
- Real `GenerateGenesisState`, `WeightedOperations`, `RegisterStoreDecoder` for `x/oracle`, `x/rwa`, `x/fees` (fees: genesis + decoder only).
- Adversarial/property/fuzz test files under each module's `keeper`/`simulation` packages.
- `docs/gas-audit.md`, `docs/load-test.md`, `docs/tmkms.md`, `docs/validator-setup.md`.
- `.github/workflows/codeql.yml`; `sim` job in `ci.yml`; `infra/loadtest/`; Makefile `test-sim-*` and `load-test` targets.

### 10.2 Acceptance gate

- Full-app simulation across N seeds completes with **zero invariant violations**; import/export and non-determinism sims pass.
- Adversarial + property + fuzz suites green.
- CodeQL findings resolved or explicitly documented.
- TPS baseline measured and recorded in `docs/load-test.md`.
- `docs/tmkms.md` + `docs/validator-setup.md` complete.
- `make lint`, `make test`, `make build` green.

### 10.3 Cross-phase contract (produced)

The five invariant routes — `oracle/prices`, `fees/reconcile`, `fees/module-balance`, `rwa/bonds`, `rwa/denoms` — become **permanent crisis checks**. Every later phase (8/9/10) must keep them passing; any change that would break one is rejected by default.

### 10.4 Doc-syncs forced by this phase

1. **`full-design-spec.md` Phase 7** — replace "aggregate prices exist for all configured pairs" with the structural-integrity invariant (D1); note the additive `x/fees` burn-accounting KV state (D2).
2. **`full-design-spec.md` Appendix C** — mark "TPS target (Phases 7/9)" as **resolved for Phase 7 = baseline-only**; the hard threshold remains a Phase 9 open item.
3. **`technical-design.md`** — record the oracle `prices` invariant form, the `x/fees` `reconcile`/`module-balance` invariants, and the two new fees KV keys + `InitGenesis`/`EndBlocker` touchpoints.
4. **`project-structure.md`** — add `docs/gas-audit.md`, `docs/load-test.md`, `infra/loadtest/`; correct the `tmkms.md`/`validator-setup.md` "created by" note to Phase 7 (canonical).
5. **`AGENTS.md` §6** — add the five invariant routes to the invariants reference.

---

## 11. Risks & Open Items

| Item | Risk | Mitigation |
|---|---|---|
| Weighted sim ops are fragile (account/funding/state selection) | Flaky or vacuous simulations | Lifecycle-aware op selection; skip-on-invalid-precondition (return `noOp`); start with oracle + rwa-mint, expand incrementally. |
| `fees/reconcile` ordering dependency (bank before fees) | Wrong genesis snapshot ⇒ false invariant break | Assert `OrderInitGenesis` places `bank` before `fees` with a test; snapshot defensively. |
| Existence-of-prices not enforced on-chain (D1) | A stalled feed set goes unnoticed by invariants | Explicitly a **monitoring** responsibility (Phase 8 dashboards / Tenderduty); documented as such. |
| CodeQL false positives | Noise / wasted triage | Advisory (non-blocking); triage + document accepted findings. |
| Load-test numbers hardware-dependent | Misleading baseline | Record exact topology/hardware in `docs/load-test.md`; baseline is descriptive, not a gate. |
| Sim runtime in CI | Slow PRs | Bounded fullapp on PR; fuller run on weekly schedule. |

---

## 12. Out-of-Scope Confirmations

- No new protobuf messages or fields (the two `x/fees` keys are raw KV, not proto).
- No changes to standard SDK module sources (Architectural Invariant 5).
- No economics changes (slash rates, fee ratios, bond minimums) — only their *validation* and *invariant coverage*.
- Third-party audit, external validators, public faucet, onboarding/quickstart docs, and hard TPS gating are later phases.
