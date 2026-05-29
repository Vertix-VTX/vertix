# Phase 9 — Testnet v2 + Genesis Rehearsal — Design Spec

**Date:** 2026-05-29
**Status:** Approved (brainstorm output)
**Phase:** 9 of the canonical phase map ([`full-design-spec.md`](../full-design-spec.md) §7.2)
**Depends on:** Phase 8 (public testnet v1) · **Enables:** Phase 10 (mainnet launch)
**Owns:** `vertix-testnet-2` genesis tooling + gentx ceremony rehearsal, founder-only → public-open lifecycle, `x/upgrade` store-migration handler + Cosmovisor E2E, module-realistic load harness with calibrated SLO, Chain Registry + wallet prep, mainnet genesis builder (stretch).

> This is the per-phase design spec produced by the Phase 9 brainstorm. It refines — and must stay consistent with — the program source of truth ([`full-design-spec.md`](../full-design-spec.md) Phase 9). Where this doc and the program spec disagree on phase order or cross-phase contracts, the program spec wins. Module internals defer to [`technical-design.md`](../technical-design.md). The step-by-step execution plan lives in `docs/plans/` (produced after this spec).
>
> **Audit deferral (brainstorm decision).** The canonical program spec bundles third-party audit + fix integration into Phase 9. This brainstorm **defers audit to a parallel track** tracked separately; Phase 9 deliverables below cover testnet v2, Cosmovisor upgrade, genesis rehearsal, load test, and ecosystem prep only. Audit integration remains a **Phase 10 gate** (must complete before mainnet launch).

---

## 1. Goal & Deliverable Boundary

**Goal:** Deliver a **mainnet-equivalent operational dry run** on `vertix-testnet-2` — coordinated gentx genesis ceremony, Cosmovisor store migration upgrade, module-realistic load test with a calibrated pass/fail bar, and Chain Registry / wallet prep — while `vertix-testnet-1` stays live.

This phase is **operational rehearsal + upgrade infrastructure**, not new protocol features. Difficulty is multi-party coordination, irreversibility of genesis templates, and Cosmovisor correctness — not new on-chain logic (except a trivial no-op migration to prove the upgrade path).

### 1.1 Brainstorm decisions (locked)

| Decision | Choice |
|---|---|
| Audit | **Deferred** — parallel track; not in Phase 9 implementation scope |
| Genesis rehearsal | **Two-step:** Step 1 (required) = procedure on testnet allocation; Step 2 (stretch) = mainnet allocation dry run |
| Cosmovisor upgrade | **Store migration** — `SetUpgradeHandler` + `RunMigrations` + trivial module version bump; no param changes |
| Load test | **Harness-first, calibrate bar** — build module-realistic client; set SLO from measured v2 results |
| v1 → v2 transition | **Founder-only v2 first** — internal gate; v1 stays public; open v2 to external validators after gate |
| Implementation approach | **Approach C** — extend Phase 8 with v2 subpaths; reuse node kit, monitoring, Cosmovisor layout |

### 1.2 In scope (9 workstreams)

1. **v2 genesis tooling** — split base builder + per-validator gentx + coordinator collect script; published SHA256.
2. **Founder-only v2 stack** — private Compose profile reusing Phase 8 sentry/seed/monitoring model.
3. **Gentx coordinator playbook** — runbook section + deadline/hash workflow.
4. **`x/upgrade` handler** — named plan `v0.2.0-testnet` with real `RunMigrations`.
5. **Cosmovisor E2E script** — gov proposal → binary swap → resume → invariants pass.
6. **Module-realistic load harness** — bank/oracle/RWA mix; calibrated SLO in `docs/load-test.md`.
7. **Chain Registry drafts** — `chain.json` + `assetlist.json` for testnet-2; mainnet draft (stretch).
8. **Runbook + onboarding updates** — v2 launch, upgrade procedure, internal → public gate.
9. **Mainnet genesis builder (stretch)** — `scripts/mainnet/build-genesis.sh` encoding [`tokenomics.md`](../tokenomics.md).

### 1.3 Out of scope

- Third-party audit engagement, fix integration, published audit report (parallel track; Phase 10 gate).
- Mainnet launch (`vertix-1`) — Phase 10.
- Protocol/proto/economics changes unrelated to the upgrade-test migration.
- Sunset of `vertix-testnet-1` (stays live through Phase 9).
- Hard merge of Chain Registry PR (draft only; mainnet merge is Phase 10).
- CI-enforced load-test gate (operator-run, same policy as Phase 7 baseline).

### 1.4 Execution order

WS4 (upgrade handler) → WS1 (genesis tooling) → WS2 (founder stack) → WS3 (gentx playbook) → WS5 (Cosmovisor E2E) → WS6 (load harness) → WS7 (registry) → WS8 (runbook) → WS9 (mainnet builder, stretch).

---

## 2. Current State (Baseline)

Phase 9 builds on Phase 8 public testnet infrastructure.

| Area | State at start of Phase 9 |
|---|---|
| Testnet v1 | `vertix-testnet-1` — founder-launch + post-genesis `MsgCreateValidator`; `scripts/testnet/build-genesis.sh`, `infra/testnet/genesis/`, `make testnet-*` targets |
| Node kit | `infra/testnet/node-kit/` — Cosmovisor layout, systemd units, state-sync, feeder pairing |
| Gentx procedure | Sketched in `docs/testnet-runbook.md` appendix; not executed on v1 |
| `x/upgrade` | Wired in `app/app.go`; `SetModuleVersionMap` in InitChainer; **no** `SetUpgradeHandler` registered |
| Load test | Phase 7 baseline harness (`infra/loadtest/`, `make load-test`); kvstore client only; no measured numbers; no pass/fail bar |
| Chain Registry | No `infra/chain-registry/` artifacts |
| Mainnet genesis | [`tokenomics.md`](../tokenomics.md) defines allocation/vesting; no builder script |
| Crisis invariants | Five permanent routes (Phase 7); must keep passing on v2 |

---

## 3. Architecture & Phase Flow

### 3.1 Founder-only → public open

```
Phase 9 Internal (founders)          Phase 9 Public open
─────────────────────────────        ─────────────────────
build-genesis + gentx ceremony  →    invite v1 validators
launch founder-only v2          →    public v2 reopen
Cosmovisor upgrade test         →    registry PR drafted
load harness + calibrate SLO    →    wallets verified
2-week stability gate           →

Parallel: vertix-testnet-1 stays live (unchanged)
```

v1 and v2 use **separate chain IDs** and **separate genesis artifacts**. The node kit is shared — operators override `CHAIN_ID`, `GENESIS_URL`, `GENESIS_SHA256` in `env`.

### 3.2 Repository layout (Approach C)

```
infra/testnet/
├── genesis/              # v1 (unchanged)
├── v2/
│   ├── genesis/          # published v2 artifacts
│   ├── docker-compose.yml
│   └── .env.example
scripts/testnet/
├── build-genesis.sh      # v1 (unchanged)
├── v2/
│   ├── build-genesis.sh  # base genesis (accounts, params)
│   ├── collect-gentxs.sh # coordinator merge + SHA256
│   ├── upgrade-test.sh   # Cosmovisor E2E
│   └── load-test.sh      # module-realistic runner
app/upgrades/v020/        # UpgradeHandler + no-op migration
infra/loadtest/
├── clients/              # vertixd tx flood
└── mix.toml              # 70/20/10 bank/oracle/rwa
infra/chain-registry/
├── testnet/              # vertix-testnet-2 draft
└── mainnet/              # vertix-1 draft (stretch)
scripts/mainnet/
└── build-genesis.sh      # stretch — tokenomics vesting
```

### 3.3 Makefile targets

| Target | Purpose |
|---|---|
| `make testnet-v2-genesis` | Build base genesis + collect gentxs |
| `make testnet-v2-up` | Start founder-only v2 stack |
| `make testnet-v2-upgrade` | Cosmovisor E2E (gov → swap → resume) |
| `make testnet-v2-load-test` | Module-realistic harness |
| `make testnet-v2-verify` | Block production + crisis invariants |

---

## 4. Workstream Details

### 4.1 WS1 + WS3 — Gentx ceremony (Step 1)

Step 1 rehearses the coordinated pre-launch flow that v1 skipped.

**Coordinator workflow:**

1. `build-genesis.sh` → base `genesis.json` (accounts, module params, no gentxs).
2. Distribute base genesis + submission deadline to each founder.
3. Each founder: `init` → `genesis gentx` → submit JSON to coordinator.
4. `collect-gentxs.sh` → merge gentxs → final `genesis.json`.
5. `validate-genesis` → SHA256 → publish to `infra/testnet/v2/genesis/`.
6. All founders: verify hash → start at agreed UTC time.

**Allocation:** testnet-style (faucet, founder bonds, VIP pool) — same economics pattern as v1. Header: `PUBLIC TESTNET v2 — NOT mainnet distribution`.

**Determinism:** base genesis bytes deterministic from mnemonics + params; final genesis sorted by validator address for reproducibility.

**Difference from v1 builder:** v1 auto-generates all gentxs in one script. v2 splits into base builder + per-validator gentx + collect step to simulate mainnet coordination.

### 4.2 WS2 — Founder-only v2 stack

Private Compose profile — not publicly announced until internal gate passes.

| Component | Notes |
|---|---|
| 3 founder validators | Behind sentries (Phase 8 topology) |
| 3 feeders | Independent sidecars |
| Seed node | Private DNS/IP (founders only during rehearsal) |
| RPC/LCD/gRPC | Internal during rehearsal; opened at public gate |
| Monitoring | Reuse v1 Grafana/Tenderduty/PANIC with v2 chain-id labels |
| Faucet | Optional for internal gate; required at public open |

Chain ID: `vertix-testnet-2`.

### 4.3 WS4 + WS5 — Cosmovisor store migration upgrade

**Upgrade plan name:** `v0.2.0-testnet`.

**Handler:** `SetUpgradeHandler("v0.2.0-testnet", ...)` calls `app.ModuleManager.RunMigrations(ctx, app.Configurator(), fromVM)`.

**No-op migration:** register trivial `Migration` on one custom module (e.g. `x/fees` consensus version 1→2) returning `nil` — proves hook fires without touching live economics.

**Cosmovisor flow:**

1. Chain running v0.1.x at height H.
2. Gov proposal: `SoftwareUpgrade { name: "v0.2.0-testnet", height: H+100 }`.
3. Validators place v0.2.x binary in `$DAEMON_HOME/cosmovisor/upgrades/v0.2.0-testnet/bin/`.
4. At H+100: halt → Cosmovisor swaps → `RunMigrations` → resume.
5. Verify: block production, five crisis invariants, oracle feeds resume.

**Verification:** `scripts/testnet/v2/upgrade-test.sh` submits gov proposal, waits for upgrade height, asserts height advances post-halt, runs `integration-verify.sh` against v2 endpoints.

### 4.4 WS6 — Load harness (calibrate-then-gate)

**Client:** `vertixd tx` flood (Go or bash), not kvstore `tm-load-test`.

| Message type | Share | Rationale |
|---|---|---|
| `MsgSend` (bank) | 70% | Baseline throughput |
| `MsgSubmitPrice` (oracle) | 20% | EndBlock aggregation pressure |
| RWA step (register or attest) | 10% | Custom module gas + state writes |

**Topology:** 3 founder validators + 3 feeders on v2 Compose profile.

**Calibration protocol:**

1. Run 10-minute flood at ramping rates (50 → 100 → 150 tx/s).
2. Record sustained TPS, p50/p99 commit latency, block time, reject rate.
3. Set bar = **80% of peak sustained TPS**, **p99 ≤ 2× target block time**, **zero crisis trips**.
4. Document in `docs/load-test.md` § "Phase 9 SLO (testnet v2)".

Resolves Appendix C "TPS target" open item for Phase 9.

### 4.5 WS7 — Chain Registry + wallets

| File | Chain | Purpose |
|---|---|---|
| `infra/chain-registry/testnet/chain.json` | `vertix-testnet-2` | Registry PR draft |
| `infra/chain-registry/testnet/assetlist.json` | `uvtx` / VTX | Wallet denom metadata |
| `infra/chain-registry/mainnet/chain.json` | `vertix-1` | Stretch — `"stage": "pre-launch"` |
| `infra/chain-registry/mainnet/assetlist.json` | `uvtx` / VTX | Stretch |

Keplr + Leap `suggestChain` JSON derived from `chain.json` (bech32 prefix `vertix`, coin type 118). Tested against v2 public RPC after internal gate.

Registry PR drafted after public v2 open; not merged in Phase 9.

### 4.6 WS9 — Step 2 stretch (mainnet genesis builder)

Runs after internal gate passes and 5–10 validators commit.

`scripts/mainnet/build-genesis.sh` encodes [`tokenomics.md`](../tokenomics.md):

- All 7 allocation categories at full 21M supply.
- Vesting types (`PeriodicVestingAccount`, `ContinuousVestingAccount`) per category.
- Mainnet module params (gov periods, oracle accept-list, fees ratios).
- Gentx collection from committed validators.
- Output: `infra/mainnet/genesis/` + SHA256 — **frozen as Phase 10 template**.

Step 1 proves the process; Step 2 proves the economics.

---

## 5. Acceptance Gates

### 5.1 Internal gate (founder-only → public open)

All must pass before inviting external validators:

| # | Gate | Verification |
|---|---|---|
| 1 | Gentx ceremony complete | All founders verify SHA256; blocks producing |
| 2 | Cosmovisor upgrade success | Zero downtime; height advances post-halt |
| 3 | Crisis invariants post-upgrade | All five routes pass (`integration-verify.sh`) |
| 4 | Load SLO met | Calibrated bar documented and satisfied |
| 5 | 2-week stability | Uptime ≥99%, reconciliation passing, feeds green |
| 6 | Oracle feeds resume post-upgrade | Independent sidecars submitting within 2 vote windows |

### 5.2 Phase 9 completion gate (public v2)

| # | Gate | Verification |
|---|---|---|
| 1 | Public v2 open | External validators joined; ≥3 in active set |
| 2 | Registry PR drafted | `chain.json` + `assetlist.json` PR open against `cosmos/chain-registry` |
| 3 | Wallets verified | Keplr + Leap `suggestChain` tested on live RPC |
| 4 | Runbook complete | v2 launch, upgrade, gentx coordinator, internal→public gate documented |
| 5 | Load SLO published | `docs/load-test.md` Phase 9 section filled with measured values |
| 6 | Upgrade contract proven | Documented Cosmovisor path reusable for future upgrades |

### 5.3 Stretch gate (Step 2 — mainnet genesis)

| # | Gate | Verification |
|---|---|---|
| 1 | Mainnet genesis builder | `scripts/mainnet/build-genesis.sh` produces valid genesis |
| 2 | Reduced validator dry run | 5–10 validators submit gentxs; SHA256 coordinated |
| 3 | Genesis contract frozen | Output marked as Phase 10 template in docs |

---

## 6. Testing Strategy

| Layer | What | How |
|---|---|---|
| Unit | Upgrade handler + no-op migration | Go test in `app/upgrades/v020/` |
| Integration | Genesis builder determinism | `make testnet-v2-genesis` twice → same SHA256 |
| Integration | Gentx collect + validate | `collect-gentxs.sh` + `validate-genesis` |
| E2E | Cosmovisor upgrade | `make testnet-v2-upgrade` on running v2 stack |
| E2E | Crisis invariants | `make testnet-v2-verify` pre- and post-upgrade |
| Load | Module-realistic mix | `make testnet-v2-load-test` on v2 stack |
| Manual | Wallet suggestChain | Keplr/Leap against public RPC |
| Manual | Gentx coordinator comms | Runbook walkthrough with founders |

No new CI gates for load test (operator-run, hardware-dependent).

---

## 7. Error Handling & Rollback

| Failure | Response |
|---|---|
| Gentx hash mismatch at start | **Do not start.** Coordinator re-runs collect; validators re-verify. |
| Validator misses coordinated start | Chain may not produce blocks; restart after hash agreement. v2 is disposable — wipe and re-run ceremony. |
| Cosmovisor swap fails | Check binary path, `upgrade-info.json`, `DAEMON_NAME`. Roll back to pre-upgrade snapshot; re-schedule at H+N. |
| Migration panics at upgrade height | **Chain halt.** Fix handler offline; restore from snapshot; never retry broken migration on live v2. |
| Crisis invariant trip post-upgrade | Halt investigation per runbook §7; treat as upgrade regression. |
| Load SLO not met | Document bottleneck (block size, gas limits, EndBlock cost). Tune params via gov on v2; re-run harness. Not a Phase 9 blocker if documented with remediation plan. |
| External validator gentx invalid | Coordinator rejects before collect; validator fixes and resubmits. |

v2 is intentionally **disposable** during Step 1 — failed ceremonies wipe and retry without affecting v1.

---

## 8. Cross-Phase Contracts

| Contract | Phase 9 output | Consumer |
|---|---|---|
| **Upgrade contract** | Working `SetUpgradeHandler` + Cosmovisor swap path, documented in runbook | All future upgrades; Phase 10 Day-1 |
| **Genesis contract (stretch)** | Mainnet genesis template from Step 2 dry run | Phase 10 final genesis |
| **Onboarding contract (inherited)** | Node kit + `validator-onboarding.md` updated for v2 chain ID + gentx join path | Phase 10 validator set |
| **Invariant contract (inherited)** | Five crisis routes pass on v2 pre/post-upgrade | All phases |
| **Load contract** | Phase 9 SLO documented in `load-test.md` | Phase 10 performance baseline |
| **Registry contract** | Draft PR artifacts | Phase 10 merge |

**Audit contract (deferred):** third-party audit + fix integration remains a **Phase 10 launch gate**, not satisfied by this spec's scope.

---

## 9. Deliverables

| # | Deliverable | Path(s) |
|---|---|---|
| 1 | v2 genesis builder + collect + published SHA256 | `scripts/testnet/v2/build-genesis.sh`, `collect-gentxs.sh`, `infra/testnet/v2/genesis/` |
| 2 | Founder-only v2 Compose stack | `infra/testnet/v2/docker-compose.yml` |
| 3 | Upgrade handler + no-op migration | `app/upgrades/v020/` |
| 4 | Cosmovisor E2E script | `scripts/testnet/v2/upgrade-test.sh` |
| 5 | Module-realistic load harness + SLO | `infra/loadtest/clients/`, `mix.toml`, `scripts/testnet/v2/load-test.sh` |
| 6 | Chain Registry drafts | `infra/chain-registry/testnet/`, `infra/chain-registry/mainnet/` (stretch) |
| 7 | Wallet suggestChain JSON | `infra/chain-registry/testnet/` or `docs/wallet-config.md` |
| 8 | Runbook + onboarding updates | `docs/testnet-runbook.md`, `docs/validator-onboarding.md` |
| 9 | Mainnet genesis builder (stretch) | `scripts/mainnet/build-genesis.sh`, `infra/mainnet/genesis/` |
| — | Make targets | `testnet-v2-genesis`, `testnet-v2-up`, `testnet-v2-upgrade`, `testnet-v2-load-test`, `testnet-v2-verify` |

---

## 10. Doc-Syncs

- Add Phase 9 section to `docs/roadmap.md` (note audit deferral vs canonical spec).
- Update `docs/load-test.md` — Phase 9 SLO section; resolve Appendix C TPS item.
- Update `docs/testnet-runbook.md` — v2 launch, gentx coordinator, upgrade test, internal→public gate.
- Update `docs/validator-onboarding.md` — v2 chain ID, gentx path for ceremony participants.
- Add `infra/testnet/v2/`, `infra/chain-registry/`, `app/upgrades/` to `docs/project-structure.md`.
- Add Phase 9 plan reference to `AGENTS.md` §2 when plan is written.
- Cross-link `docs/architecture.md` §8 (Upgrades row) to upgrade runbook section.

---

## 11. Risks & Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Audit deferred — latent bugs reach v2 | Medium/High findings discovered late | Track audit parallel; v2 is rehearsal not mainnet; bug bounty on v1 continues |
| Gentx coordination failure | Chain won't start | v2 disposable; documented retry; Step 1 uses only founders |
| Cosmovisor misconfiguration | Chain halt at upgrade height | Worked example in runbook; E2E script; snapshot before upgrade |
| Migration panic | Irrecoverable halt on v2 | No-op migration only; unit test handler; snapshot restore procedure |
| Load SLO unrealistic | False pass or false fail | Calibrate from measurement; document hardware; not CI-gated |
| v1/v2 operator confusion | Wrong chain ID / genesis | Separate artifact paths; clear naming; node kit `env` overrides |
| Mainnet genesis builder complexity | Vesting encoding errors | Step 2 stretch only; validate with `validate-genesis`; compare total supply to 21M |
| External validators unavailable for Step 2 | Genesis contract not frozen | Step 2 is stretch; Step 1 still satisfies core Phase 9 gate; Step 2 can slip to early Phase 10 |

---

## 12. Out-of-Scope Confirmations

- **No** third-party audit engagement or fix integration in this phase (parallel track).
- **No** mainnet launch or final `vertix-1` genesis execution (Phase 10).
- **No** sunset of `vertix-testnet-1`.
- **No** Chain Registry PR merge (draft only).
- **No** proto/economics changes except trivial no-op migration for upgrade test.
- **No** modification of standard SDK module sources.

---

## 13. Update Log

| Date | Change |
|---|---|
| 2026-05-29 | Initial spec from Phase 9 brainstorm (audit deferred; Approach C; two-step genesis; store migration upgrade; harness-first load test; founder-only v2 first) |
