# Phase 6 — Devnet (Multi-Validator Stack) — Design Spec

**Date:** 2026-05-29
**Status:** Approved (brainstorm output)
**Phase:** 6 of the canonical phase map ([`full-design-spec.md`](../full-design-spec.md) §7.2)
**Depends on:** Phase 3 (`x/rwa` lifecycle + bonding), Phase 4 (`vertix-feeder` sidecar), Phase 5 (IBC ICS-20 enablement) · **Enables:** Phase 7 (security hardening)
**Owns:** the reproducible, multi-validator **operational stack** for Vertix — a 3-validator `docker compose` devnet, one `vertix-feeder` per validator, a live IBC counterparty (gaia) + Hermes relayer, a Ping.pub explorer, a Cosmfaucet faucet, a minimal Prometheus + Grafana observability layer, deterministic genesis/seed/reset tooling, and the documented + automated end-to-end run that exercises every prior cross-phase contract together.

> This is the per-phase design spec produced by the brainstorm of Phase 6. It refines — and must stay consistent with — the program source of truth ([`full-design-spec.md`](../full-design-spec.md) Phase 6). Where this doc and the program spec disagree on phase order or cross-phase contracts, the program spec wins. Module internals defer to [`technical-design.md`](../technical-design.md). The step-by-step execution plan lives in `docs/plans/` (produced after this spec).
>
> **Canonical phase note.** Per [`full-design-spec.md`](../full-design-spec.md) §7.2, "Phase 6" is the **multi-validator devnet stack**. The historical `roadmap.md` numbering calls the devnet "Phase 5" and security hardening "Phase 6"; the program spec's canonical ordering supersedes that numbering, and this spec uses it (devnet = Phase 6, security hardening = Phase 7).

---

## 1. Goal & Deliverable Boundary

Stand up a **stable internal multi-validator network running all modules and sidecars together**, reproducibly, with an automated proof that the whole protocol works end to end: a validator feeder submits a price → `x/oracle` aggregates it → `x/rwa` attests/mints/settles against that price → fees route to the collector → `x/fees` burns/distributes in `EndBlock` → VTX and `rwa/{id}` move cross-chain over ICS-20.

This phase is **integration and operations**, not new module code. It consumes the binaries and contracts produced by Phases 0–5 and wires them into one orchestrated stack.

**In scope:**

- **3-validator devnet** (`vertix-val1/2/3`) on a single `docker compose` network, chain ID `vertix-devnet-1`.
- **One `vertix-feeder` per validator** (3 feeders), each authorized via `x/oracle` `MsgSetFeeder` to submit for its own validator with a dedicated feeder key.
- **Live IBC**: a single-node **gaia** (cosmoshub) counterparty + a **Hermes v1.8.x** relayer holding one ICS-20 `transfer` channel.
- **Explorer**: Ping.pub configured for `vertix-devnet-1`.
- **Faucet**: Cosmfaucet, rate-limited, funded from a devnet faucet key.
- **Minimal monitoring**: Prometheus scraping CometBFT (`:26660`) + the 3 feeders (`:9200`), plus one provisioned Grafana dashboard.
- **Deterministic genesis/seed/reset tooling** (`scripts/devnet/`) driven by **fixed, committed devnet-only mnemonics + node keys**.
- **Documented + automated end-to-end run**: `docs/devnet.md` human runbook **and** `scripts/devnet/smoke.sh` that drives and **asserts** the full flow (oracle → rwa → fees → IBC), usable as a QA gate.
- **Makefile targets** for the Docker stack, coexisting with the existing single-node Ignite loop.

**Out of scope (deferred):**

- **External / third-party validators** — Phase 8 (public testnet v1).
- **Security hardening** (invariants registration, simulations, CodeQL, load testing, `tmkms`/sentry docs) — Phase 7.
- **Production-grade monitoring** (full dashboards, Tenderduty, PANIC, alerting) — Phase 8.
- **Mainnet channel operation / Day-1 counterparty set** — Phase 10.
- **New chain or module logic** — Phase 6 changes no `x/*` or `app/` business logic; if an integration gap is found, it is fixed in the owning module's phase scope, not here.

---

## 2. Locked Decisions (from this brainstorm)

| # | Decision | Choice | Rationale |
|---|---|---|---|
| D1 | **Orchestration** | **Docker Compose**: build one repo image, run 3 validator containers + 3 feeders + gaia + Hermes + explorer + faucet + Prometheus + Grafana as services on one Docker network. | Matches the target `infra/devnet/docker-compose.yml` in [`project-structure.md`](../project-structure.md) §9. Reproducible, closest to real multi-host validator ops, and the only option that cleanly hosts feeders + relayer + explorer + monitoring as first-class services. Ignite multi-node and raw shell scripts were rejected as less faithful and harder to extend. |
| D2 | **IBC counterparty** | A **live single-node gaia (cosmoshub)** counterparty + **Hermes** relayer in the stack, opening one ICS-20 `transfer` channel. | The spec's Phase 6 e2e run includes IBC and the integration contract requires exercising `rwa→IBC` portability **live**, not just in Phase 5's `interchaintest`. gaia mirrors the spec's Day-1 mainnet channel target (Cosmos Hub), so the devnet proves the realistic partner. (Consistent with Phase 5 D2, which uses Vertix↔Vertix only for *CI determinism*; the runtime devnet favors realism.) |
| D3 | **Relayer** | **Hermes v1.8.x**, configured against both chains; reuses/extends `infra/hermes/config.toml` from Phase 5. `rly` documented as the alternative in `docs/relayer.md`. | Matches the program spec and Phase 5 D3. The `config.toml` is a tested artifact consumed by the running stack, not docs that rot. |
| D4 | **Monitoring scope** | **Minimal**: Prometheus + Grafana with one starter dashboard now (height, missed blocks, feeder `feeds_total`/`errors_total`, fee burn). Full dashboards/alerting deferred to Phase 8. | The feeder already emits Prometheus metrics (`:9200`) and CometBFT exposes `:26660`, so observability is low-cost and makes a "stable for 1+ week" acceptance gate actually observable. Heavy dashboards belong to the public-testnet phase. |
| D5 | **Make targets** | **Coexist, distinct names.** Keep `make devnet` / `devnet-reset` as the fast single-node Ignite loop. Add a Docker family: `devnet-docker-build`, `localnet-up`, `localnet-down`, `localnet-reset`, `devnet-smoke`. | Preserves the existing inner-dev loop (no disruption) while adding the heavyweight multi-validator stack under unambiguous names. |
| D6 | **Determinism / keys** | **Fixed, committed devnet-only mnemonics + node keys**, clearly marked NEVER-FOR-MAINNET. Genesis, addresses, and node IDs (hence `persistent_peers`) are fully reproducible across resets. | Makes `devnet-reset` deterministic, gives feeders/faucet/relayer known funded keys, and lets the runbook + smoke script hardcode copy-pasteable addresses. Standard localnet practice; the security trade-off is acceptable because these keys are devnet-only and never funded on a real network. |
| D7 | **E2E delivery** | **Runbook + automated smoke script.** `docs/devnet.md` (human walkthrough) **and** `scripts/devnet/smoke.sh` that drives the full flow and **asserts** on-chain state, exiting non-zero on failure. | A repeatable QA gate, not just prose — re-runnable every reset and usable in CI later. |
| D8 | **Feeder authorization** | Each feeder uses a **dedicated feeder key** (separate from the operator key); the seed step runs `x/oracle` `MsgSetFeeder` to authorize each feeder key for its valoper. | `x/oracle` already supports feeder delegation (`MsgSetFeeder` / `ResolveAuthorizedFeeder`), honoring the feeder config's "NEVER the operator key" rule. Resolves the integration risk flagged in brainstorm. |
| D9 | **Genesis allocation** | **Simplified devnet allocation** (fund 3 validators + faucet + relayer + a couple of test users), still ≤ 21,000,000 VTX with `TestNoMintModule` intact and `x/mint` absent. Full tokenomics vesting/allocation is a Phase 9 genesis-rehearsal concern. | Devnet needs spendable, well-known balances for testing, not the production vesting schedule. The hard cap (Invariant 1) is preserved. |
| D10 | **Offline fallback** | Provide a **static-only feeder profile** so the devnet runs air-gapped (the `static` provider covers all configured pairs) in addition to the default live providers (CoinGecko + Binance + static-bootstrap `VTX:USD`). | Containers may lack outbound egress in CI/offline dev; the static profile keeps oracle aggregation (and therefore the whole e2e chain) working without external APIs. |

These inherit and must not contradict the program-level locked decisions in [`full-design-spec.md`](../full-design-spec.md) §§1–6 and the architectural invariants §5. In particular: **no new VTX issuance** (Invariants 1/2 — devnet genesis respects the 21M cap and registers no `x/mint`); the **fee split stays fully on-chain** (Invariant 5 — `x/fees` `EndBlock` is unchanged); and **standard SDK modules remain unmodified** (Invariant 6 — Phase 6 adds only operational config, no module code).

---

## 3. Target Layout (Phase 6 outputs)

Realizes [`project-structure.md`](../project-structure.md) §9 (`infra/`) and the `scripts/` + `docs/` devnet entries. Net-new files are marked `+`; confirmed/edited existing files are marked `~`.

```
infra/
├── devnet/
│   ├── docker-compose.yml          +  3 validators + 3 feeders + gaia + hermes + explorer + faucet + prometheus + grafana
│   ├── Dockerfile                  +  multi-stage build → vertix:devnet (vertixd + vertix-feeder)
│   ├── .env                        +  pinned image tags, chain IDs, ports
│   ├── mnemonics.env               +  FIXED devnet-only mnemonics + node keys (NEVER mainnet)
│   ├── validator1/config/          +  node config templates (app.toml, config.toml, client.toml)
│   ├── validator2/config/          +
│   ├── validator3/config/          +
│   ├── feeder/
│   │   ├── feeder1.yaml             +  points at vertix-val1:9090, feeder1 key
│   │   ├── feeder2.yaml             +
│   │   └── feeder3.yaml             +
│   └── gaia/                        +  gaia genesis/config + relayer account
├── hermes/
│   └── config.toml                 ~  extend Phase 5 config: vertix-devnet-1 ↔ gaia
├── explorer/
│   └── chains/vertix.json          +  Ping.pub chain definition → val1 RPC/REST
└── monitoring/
    ├── prometheus.yml              +  scrape val1-3 (:26660) + feeders (:9200)
    └── grafana/
        ├── provisioning/           +  datasource + dashboard auto-provision
        └── dashboards/vertix.json  +  height, missed blocks, feeds_total/errors_total, fee burn

scripts/devnet/
├── lib.sh                          +  shared helpers (wait-for-height, jq asserts, addr lookups)
├── init-genesis.sh                 +  deterministic genesis build + gentx collection
├── setup-ibc.sh                    +  Hermes client/connection/channel bootstrap
└── smoke.sh                        +  full e2e: oracle → rwa → fees → IBC, with assertions

docs/
├── devnet.md                       +  bring-up + human end-to-end walkthrough
└── relayer.md                      ~  extend with the devnet Hermes (vertix↔gaia) recipe

Makefile                            ~  add Docker devnet family (D5)
.gitignore                          ~  ignore generated node data dirs (keep config templates)
```

---

## 4. Topology

A single `docker compose` stack on one user-defined Docker network. `vertix-val1` is the primary with RPC/gRPC/REST exposed to the host; the other validators and all internal clients use Docker DNS service names.

```
            ┌──────────────────────── vertix-devnet-1 ────────────────────────┐
            │  vertix-val1 ◄─persistent_peers─► vertix-val2 ◄──► vertix-val3   │
            │   ▲ host:26657/9090/1317   :26660(prom)                          │
            │  feeder1 (:9201→9200)   feeder2 (:9202)   feeder3 (:9203)        │
            └──────────────────────────────────────────────────────────────────┘
                         │                                   ▲
                       hermes ──── ICS-20 transfer channel ──┘
                         │
                       gaia (single-node cosmoshub, host:26757/9190/1417)

  explorer (Ping.pub) ─► val1 RPC/REST    faucet (Cosmfaucet) ─► val1
  prometheus ─► scrape {val1..3:26660, feeder1..3:9200}    grafana ─► prometheus
```

- **Validators.** Same `vertix:devnet` image, 3 replicas with distinct configs. All bonded with equal-ish stake (so stake-weighted median has ≥3 contributors). Each enables CometBFT Prometheus (`:26660`) and (at least on val1) API + gRPC.
- **Feeders.** Same image (feeder binary). Each feeder connects to **its own** validator's in-network gRPC (`vertix-valN:9090`), signs with feeder key `feederN`, submits for `vertix-valN`'s valoper. Prometheus on `:9200` (host-mapped to `:920N`).
- **gaia + hermes.** gaia is a single-node cosmoshub instance; Hermes holds relayer keys on both chains and one ICS-20 channel. `setup-ibc.sh` performs the handshake once both chains produce blocks.
- **explorer / faucet / prometheus / grafana.** Supporting services, all pointed at val1 (or the scrape targets).

---

## 5. Images & Build

- **One repo image `vertix:devnet`** from a multi-stage `infra/devnet/Dockerfile` containing **both** `vertixd` and `vertix-feeder` (same Go module → single build context, no duplication). Build pins `GOTOOLCHAIN=go1.25.4` per the Makefile note.
- **Pinned external images** in `infra/devnet/.env`: gaia (`ghcr.io/cosmos/gaia` pinned tag), `informalsystems/hermes:1.8.x`, a Ping.pub explorer image, a Cosmos faucet image (Cosmfaucet), `prom/prometheus`, `grafana/grafana`.
- `make devnet-docker-build` wraps the image build; compose references the built tag.

---

## 6. Genesis & Determinism

`scripts/devnet/init-genesis.sh` (run by an init step / one-shot container before validators start, output to a shared volume):

1. `vertixd init` into a working home.
2. Import **fixed devnet mnemonics** (test keyring): `val1/2/3` operator keys, `feeder1/2/3`, `faucet`, `relayer`, and 1–2 `test-user` accounts. Node keys (`node_key.json` / validator consensus keys) are also fixed so node IDs and `persistent_peers` are stable.
3. `add-genesis-account` for each with a **simplified devnet allocation** (D9): validators get bondable stake; faucet gets a large spendable pool; relayer + test users get working balances. Total ≤ 21,000,000 VTX; no `x/mint`.
4. Set **custom-module genesis params**:
   - `x/oracle`: `accept_list` = `VTX:USD, BTC:USD, ETH:USD, ATOM:USD, USDC:USD`; default `VoteWindow`, miss/outlier params.
   - `x/fees`: `BurnRatio = 0.40`, `DistributionRatio = 0.60` (sum == 1, Invariant 5).
   - `x/rwa`: `MinIssuerBond` (default 10,000 VTX), mint/settle fee = 0.10% of notional.
   - Bank denom metadata for `uvtx`/`vtx`; `min_gas_prices = 0.025uvtx`.
   - **Short gov voting period** (e.g. minutes) so param-change demos finish inside a devnet session.
   - IBC/transfer genesis per Phase 5 (`send_enabled`/`receive_enabled = true`; ICA host enabled with empty `allow_messages`; controller disabled).
5. `gentx` for each validator → `collect-gentxs` → final `genesis.json`.
6. Write per-validator `config.toml`/`app.toml` (persistent_peers from fixed node IDs, Prometheus on, pruning, API/gRPC on val1).

`localnet-reset` wipes generated node data and re-runs the init → identical fresh state every time.

---

## 7. Feeders, IBC, Explorer, Faucet, Monitoring

- **Feeders (D8, D10).** During seed, run `vertixd tx oracle set-feeder <valoperN> <feederN-addr>` for each validator so the feeder key is authorized. Default config uses live providers (CoinGecko + Binance + `static` bootstrap for `VTX:USD`); a `static-only` profile (all pairs via `static_prices`) keeps the devnet working air-gapped.
- **IBC.** `setup-ibc.sh` waits for both chains to reach a height, then `hermes create channel` (ICS-20 `transfer` port, unordered). Relayer accounts are pre-funded in both genesis files. Channel IDs surfaced to `docs/devnet.md` and `smoke.sh`.
- **Explorer.** `infra/explorer/chains/vertix.json` (Ping.pub) → `vertix-val1` RPC (`:26657`) + REST (`:1317`). Shows blocks, txs, validators, and the RWA/fee activity from the smoke run.
- **Faucet.** Cosmfaucet container funded from the `faucet` key, rate-limited; exposes an HTTP endpoint for `uvtx` drips.
- **Monitoring (D4).** `prometheus.yml` scrapes `vertix-val{1,2,3}:26660` and `feeder{1,2,3}:9200`. Grafana auto-provisions the Prometheus datasource + one `vertix.json` dashboard: block height per validator, missed-block counters, feeder `feeds_total`/`errors_total`/latency, and bank total-supply (fee burn) trend.

---

## 8. Make Targets & the E2E Smoke Gate (D5, D7)

**New Docker family (coexists with single-node `make devnet`):**

| Target | Purpose |
|---|---|
| `make devnet-docker-build` | Build `vertix:devnet` image |
| `make localnet-up` | Bring up the full compose stack |
| `make localnet-down` | Stop + remove the stack |
| `make localnet-reset` | Wipe node data + re-seed (deterministic fresh state) |
| `make devnet-smoke` | Run `scripts/devnet/smoke.sh` against the running stack |

**`scripts/devnet/smoke.sh` asserts the full cross-phase chain:**

1. **Liveness** — all 3 validators past height N; all 3 feeders authorized.
2. **Oracle** — feeds aggregate; `q oracle price VTX:USD` (and others) returns a non-zero stake-weighted price across all 3 validators; TWAP populates.
3. **RWA lifecycle** — `register → attest (consumes oracle price) → mint → transfer → settle`; assert status transitions, bond lock on attest/active and release on clean settle, and `rwa/{id}` balances.
4. **Fees** — capture bank total supply + distribution FeePool before/after fee-generating txs; assert supply **decreased** (burn) and FeePool **increased** (distribute).
5. **IBC** — transfer **VTX** to gaia and assert the `ibc/<hash>` voucher balance on gaia; transfer an **unrestricted `rwa/{id}`** and assert receipt (and that a restricted asset is correctly rejected at the source boundary, per Phase 5 D7).

Exits non-zero on any failed assertion → reusable as a QA gate (and CI candidate later).

---

## 9. Cross-Phase Contracts (consumed & honored)

Phase 6 is the **integration contract** ([`full-design-spec.md`](../full-design-spec.md) Phase 6): it exercises every prior cross-phase contract together, and must honor each:

- **Phase 1 `OracleKeeper` + `BASE:QUOTE` convention** — feeders submit `MsgSubmitFeed` in the locked wire format; RWA attestation reads `GetPrice`.
- **Phase 2 fee-collector hand-off** — all fees land in `auth.FeeCollectorName`; `x/fees` is the sole sweeper. Smoke test verifies via supply/FeePool deltas, never by inspecting burn internals.
- **Phase 3 lifecycle + `rwa/{id}` denom + bonding** — full lifecycle driven via CLI; bond locked/released.
- **Phase 4 feeder operational contract** — one feeder per validator, cadence within `VoteWindow`; authorized via `MsgSetFeeder`.
- **Phase 5 denom portability + restriction semantics** — `rwa/*` transfers over ICS-20; restricted assets are not IBC-exportable (source-boundary `SendRestrictionFn`).

Phase 6 produces **no new** cross-phase contract for downstream phases beyond a reproducible stack; Phase 7 (hardening) builds directly on this devnet.

---

## 10. Acceptance Gate

From [`full-design-spec.md`](../full-design-spec.md) Phase 6, plus this brainstorm's additions:

- 3-validator devnet **stable for 1+ week** (no halts; consistent block production).
- Feeds **aggregate across all 3 validators** (stake-weighted median with ≥3 contributors); TWAP windows populate.
- **Full RWA lifecycle + fee burn visible in the explorer.**
- **Faucet operational** (rate-limited drips succeed).
- **Live IBC**: VTX and unrestricted `rwa/{id}` transfer to gaia and back; restricted asset rejected at source.
- **`make localnet-reset` is deterministic** (identical genesis/addresses/node IDs each run) and **`make devnet-smoke` is green**.
- Monitoring shows live height, feeder, and fee-burn signals.

---

## 11. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| **Feeder egress** to CoinGecko/Binance unavailable (CI/offline) → oracle has no data → whole e2e chain stalls. | `static-only` feeder profile (D10) covers all pairs locally; default profile keeps live providers for realistic runs. |
| **gaia image / version drift** breaks the ICS-20 handshake. | Pin the gaia tag in `.env`; `setup-ibc.sh` is idempotent and waits for both chains; `docs/relayer.md` documents the exact versions. |
| **Non-determinism** (random keys/node IDs) breaks copy-paste docs + reset. | Fixed mnemonics + fixed node keys (D6); reset re-derives identical state. |
| **Committed devnet keys mistaken for real keys.** | Loud NEVER-FOR-MAINNET headers in `mnemonics.env` + `docs/devnet.md`; keys are devnet-only and never funded on a real network; Phase 9 genesis rehearsal uses a fresh ceremony. |
| **Heavy stack flakiness** on the inner dev loop. | Docker devnet is under separate `localnet-*` targets; the fast `make devnet` (single-node Ignite) and per-PR unit tests are untouched. |
| **`MsgSetFeeder` semantics** differ from assumption (e.g. default-authorized account). | Confirmed `x/oracle` exposes `MsgSetFeeder`/`ResolveAuthorizedFeeder`; the plan re-verifies the exact authorization default before wiring seed. |

---

## 12. Open Questions (resolved in this brainstorm)

- **Orchestration tool** — RESOLVED: Docker Compose (D1).
- **IBC in devnet** — RESOLVED: live gaia counterparty + Hermes (D2).
- **Monitoring scope** — RESOLVED: minimal Prometheus + Grafana now (D4).
- **Make-target layout** — RESOLVED: coexist, distinct names (D5).
- **Key/genesis provisioning** — RESOLVED: fixed committed devnet mnemonics + node keys (D6).
- **E2E delivery** — RESOLVED: runbook + automated smoke script (D7).
- **Feeder signing authorization** — RESOLVED: dedicated feeder key via `MsgSetFeeder` (D8).

No open questions remain for the execution plan.

---

## Appendix A — Document Map (this phase)

| Doc | Role for Phase 6 |
|---|---|
| [`full-design-spec.md`](../full-design-spec.md) Phase 6 | Program source of truth (goal, scope, integration contract, acceptance gate) |
| This spec | Per-phase design / locked decisions / target layout |
| `docs/plans/2026-05-29-phase-6-devnet.md` | Step-by-step execution plan (produced after this spec) |
| [`project-structure.md`](../project-structure.md) §9 | Target `infra/` layout this phase realizes |
| [`2026-05-29-phase-5-ibc-enablement-design.md`](./2026-05-29-phase-5-ibc-enablement-design.md) | IBC wiring, Hermes config, `rwa/*` restriction rule (D7) consumed here |
| [`2026-05-29-phase-4-feeder-sidecar-design.md`](./2026-05-29-phase-4-feeder-sidecar-design.md) | Feeder config surface + provider model deployed here |
