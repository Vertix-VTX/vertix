# Phase 8 — Public Testnet v1 — Design Spec

**Date:** 2026-05-29
**Status:** Approved (brainstorm output)
**Phase:** 8 of the canonical phase map ([`full-design-spec.md`](../full-design-spec.md) §7.2)
**Depends on:** Phase 7 (security hardening) · **Enables:** Phase 9 (audit + testnet v2 + genesis rehearsal)
**Owns:** the public-testnet launch + operations layer — `vertix-testnet-1` genesis tooling, a public founder/seed/sentry infrastructure profile, a portable external-validator node kit, a hardened public faucet, production monitoring + alerting (Grafana / Tenderduty / PANIC) including validator self-monitoring, the bug-bounty program (docs + platform workflow), a scripted public RWA demo campaign (happy-path + dispute), the feedback/endurance loop (issue templates + triage rubric + v2 backlog + 4-week stability tracker), and a live-ops runbook.

> This is the per-phase design spec produced by the brainstorm of Phase 8. It refines — and must stay consistent with — the program source of truth ([`full-design-spec.md`](../full-design-spec.md) Phase 8). Where this doc and the program spec disagree on phase order or cross-phase contracts, the program spec wins. Module internals defer to [`technical-design.md`](../technical-design.md). The step-by-step execution plan lives in `docs/plans/` (produced after this spec).
>
> **Canonical phase note.** Per [`full-design-spec.md`](../full-design-spec.md) §7.2, "Phase 8" is **Public Testnet v1**. The historical `roadmap.md` numbering calls public testnet "Phase 7" and external audit "Phase 8"; the program spec's canonical ordering supersedes that numbering, and this spec uses it (devnet = Phase 6, security hardening = Phase 7, public testnet = Phase 8, audit + testnet v2 = Phase 9).

---

## 1. Goal & Deliverable Boundary

Stand up `vertix-testnet-1` as a **public, externally-validated network** that battle-tests the three custom modules (`x/oracle`, `x/rwa`, `x/fees`) under real validators, real feeders, and real users — and produce every in-repo artifact required to launch, operate, and endure it for 4+ weeks, plus a documented live-operations runbook covering the real-world rollout.

This phase is **operational endurance, not feature work**. It adds no protocol messages, no module economics, and no proto changes. Its difficulty is wall-clock endurance, external coordination, and abuse-resistance — not new on-chain logic.

**Operating model (decided): founder-launch + post-genesis join.** The Vertix team operates the core (founder validators behind sentries, seed node, public read endpoints, faucet, monitoring); external operators run their own nodes and join the active set via `MsgCreateValidator`. The fully-coordinated `gentx` ceremony is **documented but deferred to Phase 9** (genesis rehearsal). This maps directly onto the spec's onboarding contract: `docs/validator-onboarding.md` is the exact node + feeder setup an external validator follows to join.

**In scope (10 workstreams):**

1. **Testnet identity + genesis tooling** — `vertix-testnet-1` chain ID; a reproducible genesis builder producing a published `genesis.json` + SHA256; seed/peer addrbook.
2. **Founder core infrastructure** — a public Docker Compose profile: founder validators behind ≥2 sentries each, a public seed node, and rate-limited public RPC/LCD/gRPC.
3. **Portable external-validator node kit** — env-driven config + `systemd` + Cosmovisor layout + state-sync/snapshot support + a paired `vertix-feeder` unit, for any external operator on their own host.
4. **Public faucet** — hardened Cosmfaucet (per-address/IP rate limits, daily cap, drip amount, abuse guards), fronted by the explorer.
5. **Monitoring + alerting** — expanded Grafana dashboards + Tenderduty (missed blocks) + PANIC (validator health) + alert rules in the founder stack, **plus** validator-side self-monitoring baked into the node kit.
6. **Bug bounty** — `SECURITY.md` + `docs/bug-bounty.md` (scope = 3 custom modules, severity rubric mapped to the 5 crisis invariants), **plus** a platform listing template + submission-triage workflow in the runbook.
7. **RWA demo campaign** — a scripted, repeatable full lifecycle (`register → bond → attest → mint → settle`) **plus** an adversarial dispute demo (`MsgSlashBond`), with `docs/rwa-quickstart.md`.
8. **Feedback + endurance loop** — GitHub issue/PR templates + labels + a triage rubric + a `v2-backlog` doc + a structured 4-week stability tracker that gates promotion to Phase 9.
9. **Validator onboarding doc** — `docs/validator-onboarding.md` (the onboarding contract).
10. **Live-ops runbook** — `docs/testnet-runbook.md` (launch sequence, validator coordination, incident response, the stability tracker, the Phase 9 promotion gate).

**Out of scope (deferred):**

- **Third-party audit + fix integration** — Phase 9.
- **Mainnet genesis params, final distribution + SHA256, the coordinated genesis ceremony** — Phase 10 (the `gentx` ceremony is *lightly documented* now for Phase 9 head start, not executed).
- **`vertix-testnet-2`, Cosmovisor upgrade test, Chain Registry / wallet `suggestChain`** — Phase 9.
- **A hard TPS *threshold* / pass-fail load gate** — Phase 9 (Phase 7 established a baseline only).
- **Any change to module economics or the protobuf message surface.**

**Execution order (Approach A — launchable-core first):** (1) genesis tooling → (2) founder core infra → (3) node kit → (9) onboarding doc → (4) faucet → (5) monitoring → (7) RWA demo → (8) feedback loop → (6) bug bounty → (10) runbook. Rationale: an artifact is only useful once the thing it operates exists, so the bootable core (genesis + founder stack + a join path that passes a local smoke test) lands first; the program/process artifacts (bug bounty, runbook, triage) wrap the working system last.

---

## 2. Current State (Baseline)

Phase 8 builds directly on the Phase 6 devnet stack and Phase 7 security docs. The table records what exists at the start of Phase 8 so the plan extends rather than reinvents.

| Area | State at start of Phase 8 |
|---|---|
| Devnet stack | `infra/devnet/docker-compose.yml` — 3 validators + 3 feeders + gaia + hermes + explorer + faucet + prometheus + grafana on one host; single `devnet` network, no sentries, all ports published directly. |
| Genesis tooling | `scripts/devnet/init-genesis.sh` + `gen-keys.sh` + `lib.sh` produce a **devnet** genesis (`vertix-devnet-1`); `make localnet-genesis` / `make validate-genesis`. No public/testnet genesis, no published SHA256, no addrbook. |
| Faucet | Cosmfaucet service in compose with a fixed `FAUCET_CREDIT_AMOUNT_UVTX` + `FAUCET_COOLDOWN_TIME=60`; no per-IP limit / daily cap / abuse guard; no public-facing config. |
| Monitoring | `infra/monitoring/prometheus.yml` + one starter Grafana dashboard (`grafana/dashboards/vertix.json`), anonymous viewer enabled. **No** Tenderduty, **no** PANIC, **no** alert rules. |
| Explorer | `infra/explorer/chains/vertix.json` (Ping.pub), points at devnet. |
| Security docs | `docs/tmkms.md`, `docs/validator-setup.md` exist (Phase 7) — sentry architecture, three-key model, monitoring hooks (`:26660` CometBFT, `:9200` feeder). **No** `docs/validator-onboarding.md`. |
| Node operation | Validators run via `vertixd start` inside containers; no `systemd`/Cosmovisor unit files, no state-sync/snapshot tooling, no portable single-node kit. |
| Bug bounty / safety | **No** `SECURITY.md`, **no** `docs/bug-bounty.md`, **no** `.github/ISSUE_TEMPLATE/`. |
| RWA demo | No scripted lifecycle; no `docs/rwa-quickstart.md`. `scripts/devnet/smoke.sh` exercises an internal e2e gate but is not a user-facing demo. |
| Crisis invariants | Five permanent routes registered (Phase 7): `oracle/prices`, `fees/reconcile`, `fees/module-balance`, `rwa/bonds`, `rwa/denoms`. Must keep passing under public load. |

---

## 3. Testnet Identity & Genesis Tooling

**Chain ID:** `vertix-testnet-1`. Distinct from `vertix-devnet-1` (local) and the future `vertix-1` (mainnet).

**Genesis builder.** A reproducible script (`scripts/testnet/build-genesis.sh`, driven by `make testnet-genesis`) that produces a **byte-deterministic** `genesis.json` and prints its SHA256. Determinism matters: external operators must verify they downloaded the same genesis the founders launched.

**Testnet-only allocation (decided D1).** The testnet genesis uses a clearly-labeled *testnet-only* allocation — a generously funded faucet account, the founder validators' self-bonds, and the Validator Incentives Pool — **while keeping supply mechanics identical to mainnet** (no `x/mint`, fixed total, fee burn/distribute active). The crisis invariants (`fees/reconcile`, etc.) therefore hold on testnet exactly as on mainnet; only the *numbers and recipients* differ from the eventual mainnet genesis. The allocation is explicitly marked "TESTNET — NOT mainnet distribution" in-file and in docs so it is never mistaken for Phase 10 output.

**Published artifacts.** `infra/testnet/genesis/genesis.json` + `genesis.sha256` + a `seeds.txt`/addrbook seed list + the founder `persistent_peers`/sentry topology references. These are what `docs/validator-onboarding.md` tells operators to download and verify.

**Validation.** `vertixd genesis validate-genesis` on the built genesis (extends the existing `make validate-genesis` to the testnet chain ID); the founder stack must boot from it and produce blocks.

**Gentx ceremony (documented, deferred).** A short appendix in the runbook sketches the `gentx → collect-gentxs → genesis-hash` flow so Phase 9's rehearsal starts from a written procedure — but Phase 8 does **not** execute a coordinated gentx launch.

---

## 4. Founder Core Infrastructure

A new public-facing Compose profile, `infra/testnet/docker-compose.public.yml`, that the founders operate. It reuses the Phase 6 services where possible but applies the Phase 7 security model.

- **Founder validators behind sentries.** Each founder validator runs behind ≥2 sentry nodes (`pex`, `persistent_peers`, `private_peer_ids`, `unconditional_peer_ids` per `docs/validator-setup.md`). The validator's p2p is **not** publicly reachable; only sentries expose `:26656`.
- **Public seed node.** A dedicated seed for peer discovery, listed in `seeds.txt`.
- **Public read endpoints.** Rate-limited, read-only RPC/LCD/gRPC for the explorer, faucet, wallets, and users — bound through a fronting layer, never the validators directly.
- **Three-key separation.** Consensus key (optionally `tmkms`-fronted per `docs/tmkms.md`), feeder key, operator key — kept distinct, consistent with Phase 7.
- **Feeders.** Each founder validator runs a `vertix-feeder` (existing image), independent of the external operators' feeders.
- **Explorer + faucet + monitoring** attach to this profile (see §6, §7).

**Boundary:** we operate the *core*; external operators operate *their* nodes (§5). The Compose profile is the founder layer; it is not the path an external validator uses.

---

## 5. Portable External-Validator Node Kit

`infra/testnet/node-kit/` — a host-portable kit any external operator runs to join `vertix-testnet-1`. This is the concrete artifact behind the **onboarding contract**.

- **Env-driven config** — a single `.env`/template (moniker, chain-id, seeds, persistent_peers, min-gas-price, pruning, ports) that renders node config without hand-editing TOML.
- **`systemd` units** — `vertixd` and `vertix-feeder` service units (restart policy, resource hints, log handling).
- **Cosmovisor layout** — directory structure + env so the same node survives a Phase 9 upgrade test without re-provisioning.
- **Fast join** — state-sync config and/or a published snapshot path so a new operator catches up in minutes, not days.
- **Feeder pairing** — feeder config wired to the operator's own feeder key + the public RPC, so external feeds are genuinely independent (the real test of Invariant 3).
- **Self-monitoring** — node-exporter / CometBFT `:26660` / feeder `:9200` scrape endpoints pre-enabled + a Tenderduty config stub (see §6).

**Join flow (documented in §9):** download + verify genesis SHA256 → configure via `.env` → start + state-sync → fund operator key from faucet → `MsgCreateValidator` → start feeder → confirm submissions land and self-monitoring is green.

**Local validation (no external operators required to prove the kit):** a smoke test brings up the founder profile, then starts a node-kit node that syncs and joins via `MsgCreateValidator`, confirming the kit + onboarding path work end-to-end. Real 10+ operator onboarding is live-ops (runbook, §10).

---

## 6. Monitoring & Alerting

Extends `infra/monitoring/` and ships operator-side monitoring in the node kit.

**Founder-stack dashboards (Grafana).** Expand beyond the Phase 6 starter dashboard to cover the acceptance-gate signals:
- block time / block height progression,
- **oracle miss rate** + feed coverage per pair (from feeder `:9200` + chain state),
- **fee burn rate** + cumulative burned (ties to the `fees/reconcile` invariant),
- **RWA activity** (assets by lifecycle state, bonded total, mints),
- validator set: missed blocks, voting power, jailed status.

**Alerting.**
- **Tenderduty** — missed-block / not-signing alerts for founder validators; config template shipped for operators too.
- **PANIC** — general validator health (node down, peer count, sync status, disk).
- **Prometheus alert rules** — rising `feeds_failed_total`, flat `feeds_submitted_total` across vote windows, rising missed blocks (the precursors called out in `docs/validator-setup.md`), and any crisis-invariant-adjacent anomaly (e.g. `x/fees` module-balance nonzero at block boundary as a dashboard signal).

**Validator self-monitoring (node kit).** Each external operator gets pre-enabled scrape endpoints + a Tenderduty stub + a one-page "is my validator healthy?" checklist (signing? feeding? synced? peered? disk OK?). This makes individual operators able to detect a slashable feeder outage before it costs them.

---

## 7. Public Faucet

Harden the existing Cosmfaucet for public exposure (`infra/testnet/faucet/`).

- **Abuse guards (decided):** per-address cooldown + **per-IP rate limit** + **daily cap** + bounded drip amount. The dominant risk is draining/abuse, not branding.
- **Funding:** drips from the testnet-only faucet account (§3); generous but finite, monitored on the dashboard.
- **Surface:** the existing HTTP endpoint, linked from the explorer; a branded web page is an **optional stretch** documented in the runbook, not core scope.
- **Operations:** refill procedure + abuse-response (tighten limits / blocklist) documented in the runbook.

---

## 8. RWA Demo Campaign

Two scripts under `scripts/testnet/` + example asset definitions + a user-facing quickstart.

- **`rwa-demo.sh` (happy path, repeatable):** drives the full lifecycle against the testnet — register asset → lock issuer bond (≥ `MinIssuerBond`) → attest against a live aggregated oracle price (`DRAFT → ATTESTED`) → activate → mint `rwa/{asset-id}` factory denom → settle → release bond. Asserts the relevant crisis invariants still hold at the end. This script satisfies the acceptance gate's "≥1 full public RWA lifecycle."
- **`rwa-dispute-demo.sh` (adversarial showcase):** exercises the governance `MsgSlashBond` force-settle path (bond routed to community pool) so the protocol's safety mechanic is demonstrated publicly and the `rwa/bonds` invariant is exercised under a dispute.
- **`docs/rwa-quickstart.md`:** a by-hand, copy-paste CLI walkthrough of the happy path for real users (get tokens from faucet → register → … → mint), with a section explaining the dispute path.
- **Example asset definitions:** a small set of representative RWA classes (clearly testnet/demo) used by the scripts and quickstart.

---

## 9. Validator Onboarding Doc (Onboarding Contract)

`docs/validator-onboarding.md` — the exact, reproducible node + feeder setup an external validator follows to join `vertix-testnet-1`. This is the spec's **onboarding contract** and is carried into Phase 9/10.

Contents: hardware/OS prerequisites → install `vertixd` + `vertix-feeder` (pinned versions) → fetch & **verify genesis SHA256** → configure via the node kit `.env` → seeds/persistent_peers → state-sync/snapshot fast join → key generation (three-key model) → fund from faucet → `MsgCreateValidator` → start + verify feeder submissions → enable self-monitoring → sentry guidance (cross-link `docs/validator-setup.md`, `docs/tmkms.md`). Includes a troubleshooting section feeding the onboarding-problem issue template (§10).

---

## 10. Feedback, Endurance Loop & Live-Ops Runbook

**Issue/PR templates + labels (`.github/ISSUE_TEMPLATE/`).** Bug report, validator-onboarding problem, oracle-feed incident; labels for severity, module (`oracle`/`rwa`/`fees`), and `v2-backlog`.

**Triage rubric.** A doc mapping severity to the five crisis invariants (e.g. anything that could trip `fees/reconcile` or `rwa/bonds` is Critical), defining response SLAs, and the path from "public report" → triage → `v2-backlog` or hotfix.

**v2 backlog doc.** A tracked list the runbook feeds; the source of the spec's "triaged issue backlog for v2" deliverable.

**Bug bounty.** `SECURITY.md` (disclosure entry point) + `docs/bug-bounty.md` (scope = `x/oracle`/`x/rwa`/`x/fees` + ante handlers + genesis; severity rubric mapped to crisis invariants; reward tiers; safe-harbor; out-of-scope; coordinated-disclosure flow) + a platform listing template (Immunefi/HackerOne) and submission-triage workflow written into the runbook.

**Live-ops runbook (`docs/testnet-runbook.md`).** Launch sequence (build genesis → publish + SHA256 → bring up founder core → open faucet/explorer/monitoring → announce + onboard) · validator coordination/comms · **incident response** (chain halt, bad/ stalled feed, faucet drain, invariant trip, validator mass-jailing) · faucet refill/abuse response · the **4-week stability tracker** · the **Phase 9 promotion gate**.

**4-week stability tracker.** A structured weekly checklist that makes "stable 4+ weeks" observable: uptime / block-time, missed-block rate, feed coverage across **independent** sidecars, fee-burn reconciliation (matches the `fees/reconcile` invariant), open Criticals/Highs. Promotion to Phase 9 requires 4 consecutive clean weeks with 10+ external validators and no unaddressed critical.

---

## 11. Deliverables, Acceptance Gate, Cross-Phase Contract, Doc-Syncs

### 11.1 Deliverables (by workstream)

| # | Deliverable | Path(s) |
|---|---|---|
| 1 | Testnet genesis builder + published genesis + SHA256 + seeds | `scripts/testnet/build-genesis.sh`, `infra/testnet/genesis/`, `make testnet-genesis` |
| 2 | Founder public infra profile (sentries, seed, public endpoints) | `infra/testnet/docker-compose.public.yml` |
| 3 | Portable external-validator node kit | `infra/testnet/node-kit/` |
| 4 | Hardened public faucet | `infra/testnet/faucet/` |
| 5 | Expanded dashboards + Tenderduty + PANIC + alert rules + self-monitoring | `infra/testnet/monitoring/` (+ node-kit stubs) |
| 6 | Bug bounty program + platform workflow | `SECURITY.md`, `docs/bug-bounty.md`, runbook section |
| 7 | RWA demo (happy + dispute) + quickstart | `scripts/testnet/rwa-demo.sh`, `rwa-dispute-demo.sh`, `docs/rwa-quickstart.md` |
| 8 | Issue templates + triage rubric + v2 backlog + stability tracker | `.github/ISSUE_TEMPLATE/`, `docs/testnet-runbook.md` |
| 9 | Validator onboarding doc (onboarding contract) | `docs/validator-onboarding.md` |
| 10 | Live-ops runbook | `docs/testnet-runbook.md` |
| — | Make targets | `testnet-genesis`, `testnet-up`, `testnet-demo`, `testnet-join-smoke` |

### 11.2 Acceptance Gate

In-repo / locally verifiable (what the plan can actually prove):
- `make testnet-genesis` is reproducible (matching SHA256) and `validate-genesis` passes for `vertix-testnet-1`.
- The founder public profile boots and produces blocks.
- `make testnet-join-smoke` brings up the founder core, then a node-kit node state-syncs and joins via `MsgCreateValidator`.
- `rwa-demo.sh` completes a full lifecycle; `rwa-dispute-demo.sh` exercises slash/force-settle; both assert the five crisis invariants still hold.
- Faucet enforces its rate limits/caps; dashboards render the gate signals.

Live (runbook-driven, satisfied during operation — matches the program spec):
- Testnet stable 4+ weeks with 10+ **external** validators.
- Feeds operate across **independent** sidecars.
- ≥1 full public RWA lifecycle demonstrated.
- No critical bugs left unaddressed (triaged via the rubric).

### 11.3 Cross-Phase Contract

- **Onboarding contract (this phase owns it):** `docs/validator-onboarding.md` + the node kit define the exact node + feeder setup external validators follow. This is carried forward into Phase 9/10 genesis and operations.
- **Invariant contract (inherited, must keep passing):** the five Phase 7 crisis routes must hold under public load; the dashboards/alerts and demo assertions watch them.
- **Supply contract (inherited):** 21M-style fixed supply mechanics, no `x/mint`, fee burn/distribute active — preserved on testnet (only the testnet-only allocation numbers differ; §3 D1).

### 11.4 Doc-Syncs

- Update `docs/roadmap.md` Phase tasks (note the canonical-numbering reconciliation already used by Phase 7).
- Add `docs/validator-onboarding.md`, `docs/rwa-quickstart.md`, `docs/bug-bounty.md`, `docs/testnet-runbook.md` to `docs/project-structure.md` (the file already reserves the first two as "created by Phase 8").
- `AGENTS.md` §2/§4: add the public-testnet runbook + node kit to the doc index and repo layout; add `infra/testnet/` as a top-level infra subtree.
- Cross-link `docs/validator-setup.md` (Phase 8 dashboards section) ↔ the new monitoring + onboarding docs.

---

## 12. Risks & Open Items

| Risk / Item | Impact | Resolution |
|---|---|---|
| **D1 — Testnet vs mainnet allocation** | A testnet genesis could be mistaken for mainnet distribution | Testnet-only allocation explicitly labeled in-file + docs; supply *mechanics* identical so invariants hold; mainnet distribution remains Phase 10. |
| **"Real external validators" not provable in-repo** | The headline gate is live, not unit-testable | Prove the *kit + onboarding path* via `testnet-join-smoke`; treat 10+ operator onboarding + 4-week endurance as runbook-tracked live ops. |
| Public RPC/LCD/gRPC abuse / DoS | Endpoint overload, node instability | Read-only, rate-limited public endpoints; validators behind sentries (Phase 7); never expose validator p2p/RPC directly. |
| Faucet draining | Demo + onboarding blocked | Per-IP + daily cap + cooldown + bounded drip; monitored; documented refill/abuse response. |
| Independent feeders disagree in the wild | Skipped pairs / oracle misses / slashes | Feeder defaults (min_providers, max_deviation, max_quote_age) from Phase 4; self-monitoring + alerting surfaces precursors before slashing. |
| Crisis invariant trips under public load | Chain halt | Dashboards/alerts watch the five routes; runbook incident-response path; demo scripts assert invariants. |
| Gentx ceremony deferral | Phase 9 starts cold on genesis ceremony | Document the gentx flow now (runbook appendix) for a Phase 9 head start. |

---

## 13. Out-of-Scope Confirmations

- **No** third-party audit or audit-fix integration (Phase 9).
- **No** mainnet genesis params, final distribution, SHA256-of-mainnet, or coordinated launch ceremony execution (Phase 10); the gentx ceremony is documented, not executed.
- **No** `vertix-testnet-2`, Cosmovisor upgrade execution, or Chain Registry / wallet `suggestChain` PRs (Phase 9) — though the node kit is Cosmovisor-ready so the Phase 9 upgrade test reuses it.
- **No** changes to module economics, the protobuf message surface, or standard SDK module sources.
- **No** hard TPS pass/fail threshold (Phase 9); Phase 7's documented baseline stands.
