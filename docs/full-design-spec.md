# Vertix Blockchain — Full Design Spec

**Date:** 2026-05-29
**Status:** Approved
**Chain:** Vertix (`vertix-1`) · **Ticker:** VTX · **Category:** Oracle · Real World Assets (Layer 1)
**Role:** Program-level source of truth. This is the **phase-oriented design/contract view** of Vertix. It locks the global decisions and invariants that every phase must honor, defines the canonical phase sequence, and specifies the cross-phase contracts and acceptance gates. Module internals and timelines are owned by other docs (see Appendix A).

> **Canonical ordering note.** The phase order in this spec (§7) is **canonical** and supersedes the ordering implied by `roadmap.md` and the historical `plans/` numbering wherever they disagree. `roadmap.md` remains the timeline/task view; this spec is the design/contract view.

---

# Part I — Program Foundation

These sections capture the decisions that are **locked at the program level**. Per-phase brainstorms inherit them and must not re-litigate them; a change here is a program-level change, not a phase-level one.

## 1. Vision & Scope

Vertix is a Cosmos SDK Layer 1 blockchain purpose-built as the settlement layer for **Real World Asset (RWA) tokenization**, powered by native **decentralized oracle infrastructure**. The two pillars are equal first-class citizens:

- **Oracle data feeds** exist to verify and price real-world assets.
- **RWA issuance** exists to settle assets whose trustlessness depends on those feeds.

VTX mirrors Bitcoin's supply discipline — **21,000,000 hard cap, zero inflation** — generating validator and staker rewards entirely from network fee revenue, supplemented by a Validator Incentives Pool during the bootstrap phase.

**In scope at launch:** the three custom modules (`x/oracle`, `x/rwa`, `x/fees`), the `vertix-feeder` sidecar, ICS-20 IBC for VTX and `rwa/*` denoms, and a standard SDK module set with `x/mint` removed.

**Explicitly out of scope at launch (deferred post-mainnet):** CosmWasm / smart contracts, cross-chain oracle queries (ICS-004), hybrid oracle models, and any mechanism that could introduce inflation.

## 2. Tech Stack (Pinned)

| Component | Version |
|---|---|
| Cosmos SDK | v0.50.x |
| CometBFT | v0.38.x |
| ibc-go | v8.x |
| Go | 1.22+ |
| Ignite CLI | v28.x |
| Hermes Relayer | v1.8.x |
| Cosmovisor | v1.5.x |
| golangci-lint | v1.57+ |
| buf | v1.30+ |

**Do not bump these without an explicit roadmap-aligned task.**

## 3. Architecture at a Glance

`vertixd` is a standard Cosmos SDK app over CometBFT v0.38, composed of three layers (networking/consensus → BaseApp/module manager → modules). Three custom modules carry all Vertix-specific logic; standard SDK modules are wired unmodified; `x/mint` is deliberately removed to make inflation impossible without a hard fork.

```
External APIs ─► vertix-feeder (per validator) ─► MsgSubmitFeed
                                                       │
                                                       ▼
   x/oracle (stake-weighted median + TWAP) ──GetPrice──► x/rwa (lifecycle + bonding)
                                                       │            │ fees on mint/settle
                                                       │            ▼
                                              fee collector ──► x/fees (EndBlock: 40% burn / 60% distribute)
                                                       │
                                                       ▼ IBC (ICS-20): VTX + rwa/* denoms
```

→ Full system architecture, node layering, validator/sentry topology, and IBC diagrams: [`architecture.md`](./architecture.md).
→ Module internals (protobuf surface, keeper APIs, store layouts, slashing math): [`technical-design.md`](./technical-design.md).

## 4. Tokenomics at a Glance

```
Max Supply:  21,000,000 VTX  (hard cap, no x/mint)
Decimals:    6   (1 VTX = 1,000,000 uvtx)
Bech32:      vtx          Base denom: uvtx
Fee model:   40% burn  +  60% to stakers  (both governance-adjustable)
Allocation:  Team 22% · Foundation 20% · Ecosystem 18% · Investors 13%
             Validators 12% · Liquidity 8% · Airdrop 7%
```

A **Validator Incentives Pool** funds staking rewards during bootstrap until fee revenue scales. Burns are permanent and monotonic; total supply only ever decreases after genesis.

→ Full vesting schedules, pool depletion model, and value-accrual analysis: [`tokenomics.md`](./tokenomics.md).

## 5. Architectural Invariants (bind every phase)

These are non-negotiable. Any phase design or change that contradicts one is rejected by default.

1. **Hard cap.** Total VTX supply ≤ 21,000,000 forever. `x/mint` is not registered (guarded by `TestNoMintModule`). Burns are permanent and monotonic.
2. **No silent inflation.** No module issues VTX outside genesis, vesting unlocks, or `x/staking` rewards funded by fee/pool revenue.
3. **Validator-integrated oracle.** Oracle authority = the active validator set. The sidecar is mandatory; missed feeds and outliers are slashed.
4. **RWA bonding.** No asset reaches `ACTIVE` without a locked issuer bond ≥ `MinIssuerBond`.
5. **Fee determinism.** The `EndBlock` fee split is fully on-chain; off-chain code never influences the burn/distribute ratios.
6. **Standard module integrity.** Standard SDK modules are wired unmodified to preserve auditability and ecosystem compatibility.

## 6. Cross-Cutting Trade-offs & Decisions

| Decision | Choice | Trade-off accepted |
|---|---|---|
| Oracle model | Validator-integrated | Tighter validator ops requirements vs. simpler economic model |
| RWA scope | Protocol-agnostic | Compliance pushed to issuer vs. broader market coverage |
| Inflation | Zero (no `x/mint`) | Bootstrap rewards depend on the Incentives Pool until fees scale |
| Launch shape | Monolithic (oracle + RWA + fees together) | Higher coordination cost vs. complete vision at launch |
| Smart contracts | Not at launch (CosmWasm post-mainnet) | Less programmability vs. smaller audit surface |
| Fees vs. RWA build order | `x/fees` before `x/rwa` | Slightly counter-intuitive vs. having a real fee sink before RWA routes fees into it |

## 7. Phase Model & Workflow

### 7.1 How this spec is used

This spec is the **spine** of the project. Each phase below is taken, in order, through the standard workflow:

```
Full Design Spec (this doc)
   └─ per phase ─► brainstorm  ─►  phase spec (docs/specs/)  ─►  plan (docs/plans/)  ─►  implementation
                                                                                          │
                                                                       gated by prior phase's Acceptance Gate
```

- A phase's **Acceptance Gate** must pass before the **next** phase's brainstorm begins.
- Each phase's **Cross-phase contracts** are binding inputs to the phases that depend on it; downstream phase specs must honor them.
- The **design-level** content here (decisions, contracts, gates) is stable. Actionable task checklists live in [`roadmap.md`](./roadmap.md); module internals live in [`technical-design.md`](./technical-design.md).

### 7.2 Canonical phase map

| Phase | Name | Depends on | Enables |
|---|---|---|---|
| **0** | Chain Foundation | — | all |
| **1** | `x/oracle` (on-chain aggregation + slashing) | 0 | 3, 4 |
| **2** | `x/fees` (burn + distribute) | 0 | 3 |
| **3** | `x/rwa` (lifecycle + bonding) | 1, 2 | 6 |
| **4** | `vertix-feeder` sidecar | 1 | 6 |
| **5** | IBC enablement (ICS-20) | 0 | 6 |
| **6** | Devnet (multi-validator stack) | 3, 4, 5 | 7 |
| **7** | Security hardening | 6 | 8 |
| **8** | Public Testnet v1 | 7 | 9 |
| **9** | Audit + Testnet v2 + genesis rehearsal | 8 | 10 |
| **10** | Mainnet launch (gate) | 9 | — |

### 7.3 Dependency graph

```
        0 ─┬─► 1 ─┬────────────► 3 ─┐
           │      └─► 4 ─────────────┤
           ├─► 2 ──────────────► 3   ├─► 6 ─► 7 ─► 8 ─► 9 ─► 10
           └─► 5 ────────────────────┘
```

Phases **1, 2, 5** can proceed in parallel after Phase 0. Phase **3** waits on 1 and 2; Phase **4** waits on 1. Phase **6** integrates 3, 4, and 5. Phases 7→10 are strictly sequential.

---

# Part II — Per-Phase Design

Each section uses the design-level template: **Goal · Scope · Key design decisions · Deliverables · Dependencies · Cross-phase contracts · Acceptance gate.**

## Phase 0 — Chain Foundation

- **Goal:** A bootable `vertixd` chain with the full standard module set (minus `x/mint`), CI, and tooling locked.
- **Scope.** *In:* chain identity, genesis params, standard module wiring, Makefile/CI/lint/proto tooling, single-node boot. *Out:* any custom module logic, IBC channel operation, multi-validator topology.
- **Key design decisions.**
  - Chain identity: binary `vertixd`, Bech32 `vtx`, base denom `uvtx`, 6 decimals, devnet chain ID `vertix-devnet-1`.
  - `x/mint` removed; guarded by `TestNoMintModule` in `app/app_test.go`.
  - Genesis defaults per [`technical-design.md`](./technical-design.md) §1.2 (unbonding 21d, `max_validators` 125, `min_commission` 5%, `min_gas_prices` `0.025uvtx`, etc.).
- **Deliverables.** `app/app.go` wiring; `cmd/vertixd`; `config.yml`; `Makefile` (`build`/`test`/`lint`/`proto-gen`/`ts-gen`); `golangci-lint` + `buf` config; GitHub Actions CI; pre-commit hooks.
- **Dependencies.** *Depends on:* none. *Enables:* every later phase.
- **Cross-phase contracts.**
  - **App wiring contract:** custom modules are added only in `app/app.go`; standard modules remain unmodified.
  - **Genesis param surface:** the canonical genesis baseline that all later module params extend.
  - **CI/lint/proto gates:** `make lint && make test && make build` must pass — the bar every later phase inherits.
- **Acceptance gate.** Chain boots and produces blocks; REST (`:1317`), gRPC (`:9090`), RPC (`:26657`) up; `genesis validate-genesis` passes; `TestNoMintModule` passes; CI green on a PR.

## Phase 1 — `x/oracle` (On-Chain Aggregation + Slashing)

- **Goal:** Validator-submitted price feeds aggregated on-chain via stake-weighted median, with TWAP histories and miss/outlier slashing.
- **Scope.** *In:* `MsgSubmitFeed`, vote-window aggregation, TWAP (1h, 24h), miss/outlier slashing via `x/slashing`, params + genesis, the consumer keeper interface. *Out:* the off-chain feeder binary (Phase 4), RWA consumption logic (Phase 3).
- **Key design decisions.**
  - Aggregation: **stake-weighted median** per pair per `VoteWindow` (default 10 blocks).
  - Slashing: miss-rate > `MissThreshold` (5%) → `MissSlashRate` (0.5%); statistical outlier → `OutlierSlashRate` (1.0%).
  - Genesis pairs (wire form `BASE:QUOTE`): `VTX:USD`, `BTC:USD`, `ETH:USD`, `ATOM:USD`, `USDC:USD` (governance-adjustable via `accept_list`).
- **Deliverables.** `x/oracle` module; protobuf (`MsgSubmitFeed`, `MsgUpdateParams`, queries `GetPrice`/`GetTWAP`/`GetParams`/`GetMissCounter`); aggregation + TWAP + miss-counter keeper logic; events (`EventFeedSubmitted`, `EventPriceAggregated`, `EventOracleSlash`); unit + integration tests.
- **Dependencies.** *Depends on:* Phase 0; SDK `staking`, `slashing`, `params`. *Enables:* Phase 3 (price consumption), Phase 4 (feeder target).
- **Cross-phase contracts.**
  - **`OracleKeeper` interface** (consumed by `x/rwa` in Phase 3):
    ```go
    type OracleKeeper interface {
        GetPrice(ctx sdk.Context, pair string) (math.LegacyDec, error)
        GetTWAP(ctx sdk.Context, pair string, window time.Duration) (math.LegacyDec, error)
    }
    ```
  - **`MsgSubmitFeed` wire format** (consumed by the Phase 4 feeder): `validator`, `pair` (`"BASE:QUOTE"`), `price` (LegacyDec string).
  - **Pair naming convention:** `BASE:QUOTE` (e.g. `VTX:USD`), shared by feeder and RWA attestation.
- **Acceptance gate.** Feeds aggregate correctly across simulated validators; TWAP windows populate and expire; slash events fire at correct thresholds; `vertixd q oracle price [pair]` and `q oracle twap [pair] [window]` work.

## Phase 2 — `x/fees` (Burn + Distribute)

- **Goal:** Automated `EndBlock` fee burn and staker distribution from the standard fee collector.
- **Scope.** *In:* `EndBlock` sweep of the fee collector, burn/distribute split, governance params, events. *Out:* fee *generation* (gas is built-in; RWA fees arrive in Phase 3).
- **Key design decisions.**
  - `x/fees` holds **no custom state**; each `EndBlock` it sweeps `auth.FeeCollectorName` and splits.
  - Split: `BurnRatio` 40% via `x/bank.BurnCoins`; `DistributionRatio` 60% via `x/distribution` FeePool. Both governance-adjustable; must sum to 1.
  - Built before `x/rwa` so RWA has a real, tested fee sink to route into.
- **Deliverables.** `x/fees` module; `EndBlock` hook; params (`BurnRatio`, `DistributionRatio`) + validation; events (`EventFeeBurned`, `EventFeeDistributed`); integration test using gas fees.
- **Dependencies.** *Depends on:* Phase 0; SDK `bank`, `distribution`, `params`. *Enables:* Phase 3 (fee routing target).
- **Cross-phase contracts.**
  - **Fee-collector hand-off:** all fees (gas + RWA mint/settle + future oracle subscriptions) MUST land in `auth.FeeCollectorName`; `x/fees` is the sole sweeper. Phase 3 routes its fees here, not directly to burn/distribute.
  - **Ratio invariant:** `BurnRatio + DistributionRatio == 1` enforced at param validation.
- **Acceptance gate.** Fee burn reduces bank total supply (verifiable); staker rewards reflect distributed fees via `x/distribution` queries; events indexed; governance can update ratios via proposal.

## Phase 3 — `x/rwa` (Lifecycle + Bonding)

- **Goal:** Protocol-agnostic asset registry enforcing the `DRAFT → ATTESTED → ACTIVE → SETTLED` lifecycle with issuer bonding, oracle attestation, factory-denom minting, and fee routing.
- **Scope.** *In:* lifecycle state machine, issuer bond escrow, `rwa/{asset-id}` minting, transfer restrictions, oracle-price attestation, mint/settle fee routing. *Out:* the burn/distribute mechanics (Phase 2 owns those), price aggregation (Phase 1).
- **Key design decisions.**
  - Lifecycle transitions validated at each step; an asset reaches `ACTIVE` only with a locked bond ≥ `MinIssuerBond` (default 10,000 VTX).
  - Attestation requires a valid `OracleKeeper.GetPrice(pair)` result.
  - Mint and settle each charge **0.10% of notional**, routed to the fee collector.
  - Bond returned on clean `SETTLED`; slashed on fraud/dispute (governance).
  - Transfer restrictions (allowlist/denylist per asset) enforced at the message-handler level.
- **Deliverables.** `x/rwa` module; protobuf (`MsgRegisterAsset`, `MsgAttestAsset`, `MsgMintRWA`, `MsgTransferRWA`, `MsgSettleRWA`, `AssetRecord`, `AssetStatus`); state machine + bond escrow + factory-denom minting + restriction hooks; events (`EventAssetRegistered/Attested/RWAMinted/RWASettled`); full-lifecycle integration test.
- **Dependencies.** *Depends on:* Phase 1 (`OracleKeeper`), Phase 2 (fee collector); SDK `bank`, `params`. *Enables:* Phase 6 (full lifecycle on devnet).
- **Cross-phase contracts.**
  - **Consumes** the Phase 1 `OracleKeeper` interface and `BASE:QUOTE` pair convention.
  - **Routes fees** to `auth.FeeCollectorName` per the Phase 2 hand-off contract (never burns/distributes directly).
  - **Denom contract:** RWA tokens use the reserved `rwa/{asset-id}` namespace; IBC (Phase 5) must treat `rwa/*` as transferable.
- **Acceptance gate.** Full lifecycle executable via CLI; oracle price required + validated at attestation; bond locked/released correctly; transfer restrictions enforced; fees collected and swept by `x/fees`.

## Phase 4 — `vertix-feeder` Sidecar

- **Goal:** The off-chain Go binary every active validator runs to fetch prices and broadcast `MsgSubmitFeed` each block window — completing the oracle data loop.
- **Scope.** *In:* provider integrations, cross-source median, tx signing/broadcast, retry, Prometheus metrics, config file. *Out:* on-chain aggregation/slashing (Phase 1).
- **Key design decisions.**
  - Pluggable providers (CoinGecko, Binance, …); take the **cross-source median** before submitting.
  - Configurable tick (default 5s) within the 10-block vote window.
  - Validators own their sidecar uptime; misses are slashed on-chain.
  - Exposes Prometheus metrics (`feeds_total`, `errors_total`, latency) on `:9200`.
- **Deliverables.** `feeder/` module + `vertix-feeder` binary; provider abstraction; broadcaster; `feeder-config.example.yaml`; metrics; unit tests for providers/median/retry.
- **Dependencies.** *Depends on:* Phase 1 (`MsgSubmitFeed` wire format + pair convention). *Enables:* Phase 6 (live feeds per validator).
- **Cross-phase contracts.**
  - **Honors** the Phase 1 `MsgSubmitFeed` format and `BASE:QUOTE` convention exactly.
  - **Operational contract:** one feeder per validator; submission cadence fits within `VoteWindow`.
- **Acceptance gate.** Feeder fetches from ≥2 providers, computes median, signs and broadcasts each window against a local node; metrics exposed; missing/lagging providers handled gracefully.

## Phase 5 — IBC Enablement (ICS-20)

- **Goal:** VTX and `rwa/*` denoms transferable cross-chain via ICS-20.
- **Scope.** *In:* confirm `x/ibc` + `x/transfer` wiring, genesis transfer params, `interchaintest` ICS-20 suite, relayer recipes. *Out:* cross-chain oracle queries (post-mainnet), mainnet channel operation (Phase 10).
- **Key design decisions.**
  - ICS-20 enabled at genesis for both VTX and `rwa/*`.
  - Hermes v1.8.x is the primary relayer; `rly` documented as an alternative.
  - Mainnet Day-1 channel target: Cosmos Hub (Osmosis, Noble, Neutron post-mainnet).
- **Deliverables.** Confirmed `app.go` IBC wiring; transfer genesis params; `e2e/` ICS-20 tests (VTX + `rwa/{id}`); `docs/relayer.md` (Hermes + rly configs).
- **Dependencies.** *Depends on:* Phase 0 (IBC modules wired). *Enables:* Phase 6 (cross-chain in devnet stack).
- **Cross-phase contracts.**
  - **Denom portability contract:** the `rwa/*` namespace (Phase 3) must transfer correctly over ICS-20 without losing restriction semantics at the source chain.
- **Acceptance gate.** ICS-20 VTX transfers pass in `interchaintest`; `rwa/{id}` transfers work cross-chain; relayer setup documented and reproducible.

## Phase 6 — Devnet (Multi-Validator Stack)

- **Goal:** A stable internal multi-validator network running all modules, feeders, explorer, and faucet together.
- **Scope.** *In:* 3-validator devnet, per-validator feeders, Ping.pub explorer, Cosmfaucet, reset/seed scripts, end-to-end QA. *Out:* external validators (Phase 8), hardening (Phase 7).
- **Key design decisions.**
  - Single-node → 3 internal validators; each runs a `vertix-feeder`.
  - `make devnet-reset` provides deterministic fresh state + seed.
- **Deliverables.** `infra/devnet/docker-compose.yml`; feeder deployment per validator; explorer + faucet config; reset/seed scripts; documented end-to-end run (oracle feed → RWA lifecycle → fee burn → IBC).
- **Dependencies.** *Depends on:* Phases 3, 4, 5. *Enables:* Phase 7.
- **Cross-phase contracts.**
  - **Integration contract:** exercises every prior cross-phase contract together (oracle→rwa price, rwa→fees routing, feeder→oracle submission, rwa→IBC portability).
- **Acceptance gate.** 3-validator devnet stable for 1+ week; feeds aggregate across all validators; full RWA lifecycle + fee burn visible in explorer; faucet operational.

## Phase 7 — Security Hardening

- **Goal:** An audit-ready codebase with invariants, simulations, gas/load analysis, and validator security guidance.
- **Scope.** *In:* `x/crisis` invariants, simulation/fuzz tests, static analysis, gas-metering audit, adversarial testing, load testing, `tmkms`/sentry docs. *Out:* third-party audit (Phase 9), external exposure (Phase 8).
- **Key design decisions.**
  - Register invariants: oracle (prices exist for configured pairs), RWA (every `ACTIVE` asset has a bonded issuer), fees (module balances reconcile with burn+distribute totals).
  - No unbounded loops in oracle aggregation or RWA registry enumeration.
- **Deliverables.** Registered invariants; module simulations; `cosmos-sdk-codeql` in CI; gas audit notes; adversarial test cases (oracle manipulation, bond bypass); `tm-load-test` TPS baseline; `docs/tmkms.md`, `docs/validator-setup.md`.
- **Dependencies.** *Depends on:* Phase 6. *Enables:* Phase 8.
- **Cross-phase contracts.**
  - **Invariant contract:** the three module invariants become permanent crisis-module checks all later phases must keep passing.
- **Acceptance gate.** No invariant violations under simulation; CodeQL findings resolved/documented; TPS baseline documented; validator security guide complete.

## Phase 8 — Public Testnet v1

- **Goal:** Battle-test with external validators and real users.
- **Scope.** *In:* `vertix-testnet-1`, 10+ external validators, public faucet, bug bounty, independent feeders, RWA demo campaign, monitoring stack. *Out:* audit (Phase 9), mainnet params (Phase 10).
- **Key design decisions.**
  - External validators run feeders independently — first real test of Invariant 3 in the wild.
  - Monitoring: Prometheus + Grafana, Tenderduty (missed blocks), PANIC (validator health).
- **Deliverables.** Testnet genesis + onboarding docs; public faucet; bug bounty (scope = three custom modules); monitoring dashboards; `docs/rwa-quickstart.md`; triaged issue backlog for v2.
- **Dependencies.** *Depends on:* Phase 7. *Enables:* Phase 9.
- **Cross-phase contracts.**
  - **Onboarding contract:** `docs/validator-onboarding.md` defines the exact node + feeder setup external validators must follow (carried into Phase 9/10 genesis).
- **Acceptance gate.** Testnet stable 4+ weeks with 10+ external validators; feeds operate across independent sidecars; ≥1 full public RWA lifecycle; no critical bugs left unaddressed.

## Phase 9 — Audit + Testnet v2 + Genesis Rehearsal

- **Goal:** A mainnet-equivalent dry run with all third-party audit fixes integrated.
- **Scope.** *In:* external audit of all custom modules, fix integration, `vertix-testnet-2`, Cosmovisor upgrade test, genesis ceremony rehearsal, load test, Chain Registry + wallet prep. *Out:* mainnet launch itself (Phase 10).
- **Key design decisions.**
  - Combine audit + testnet v2 because v2 exists to validate audit fixes under mainnet-equivalent params.
  - Fix all Critical/High findings; mitigate or formally accept Mediums; publish the report.
- **Deliverables.** Published audit report; integrated fixes; `vertix-testnet-2`; tested Cosmovisor upgrade; rehearsed `gentx`/`collect-gentxs`/genesis-hash ceremony; load-test results; drafted `chain.json`/`assetlist.json`; tested Keplr/Leap `suggestChain`.
- **Dependencies.** *Depends on:* Phase 8. *Enables:* Phase 10.
- **Cross-phase contracts.**
  - **Genesis contract:** the rehearsed genesis structure (accounts, vesting, module params) is frozen as the mainnet genesis template.
  - **Upgrade contract:** a working `x/upgrade` handler + Cosmovisor swap path, reused for all future upgrades.
- **Acceptance gate.** Testnet v2 stable 2+ weeks post-fixes; zero unresolved Critical/High findings; genesis rehearsal clean; Cosmovisor upgrade verified; Chain Registry PR drafted.

## Phase 10 — Mainnet Launch (Gate)

- **Goal:** `vertix-1` live, producing blocks with 20+ validators, ecosystem connected.
- **Scope.** *In:* final genesis distribution + hash verification, coordinated genesis ceremony, Day-1 IBC channels, registry/wallet/explorer/monitoring go-live, incident response. *Out:* post-mainnet roadmap (Osmosis pools, Neutron, CosmWasm, etc.).
- **Key design decisions.** Finalize `vertix-1` + version tag; open Cosmos Hub channel Day-1; full Prometheus/Grafana/Tenderduty on mainnet.
- **Deliverables.** Final `genesis.json` + SHA256; seed nodes/peers/addrbook; merged Chain Registry PR; live Keplr/Leap + Ping.pub; public RPC/LCD/gRPC; incident-response channels.
- **Dependencies.** *Depends on:* Phase 9. *Enables:* post-mainnet roadmap.
- **Acceptance gate (launch criteria).** `vertix-1` producing blocks with 20+ validators; all airdrop/vesting accounts present in genesis; IBC channels open; explorer live; no critical incidents in first 48 hours.

---

# Part III — Appendices

## Appendix A — Document Map

| Doc | View | Owns |
|---|---|---|
| **This spec** | Design / contract | Program invariants, canonical phase order, cross-phase contracts, acceptance gates |
| [`roadmap.md`](./roadmap.md) | Timeline / tasks | Month-by-month schedule and per-phase task checklists |
| [`architecture.md`](./architecture.md) | System shape | Components, boundaries, data flow, topology, security architecture |
| [`technical-design.md`](./technical-design.md) | Module internals | Protobuf surface, keeper APIs, store layouts, state machines, slashing math |
| [`tokenomics.md`](./tokenomics.md) | Supply | Allocation, vesting, pool depletion, value accrual |
| [`project-overview.md`](./project-overview.md) | Orientation | One-page introduction |

When this spec disagrees with another doc on **phase order or cross-phase contracts**, this spec wins. When it disagrees on **module internals**, `technical-design.md` wins. When it disagrees on **supply**, `tokenomics.md` wins.

## Appendix B — Glossary

- **Vote window:** the block span (`VoteWindow`, default 10) over which oracle submissions are aggregated.
- **Stake-weighted median:** aggregation weighting each validator's submission by bonded stake.
- **TWAP:** time-weighted average price, stored per pair over rolling 1h/24h windows.
- **Issuer bond:** VTX locked by an RWA issuer (≥ `MinIssuerBond`), released on clean settlement or slashed on dispute.
- **Attestation:** linking a live oracle price to an asset, transitioning it `DRAFT → ATTESTED`.
- **Fee collector:** the standard `auth.FeeCollectorName` account; the sole input to `x/fees`.
- **Validator Incentives Pool:** genesis-funded pool that bootstraps staking rewards until fee revenue scales.
- **Factory denom:** the `rwa/{asset-id}` token namespace minted via `x/bank`.

## Appendix C — Open Questions (deferred to per-phase brainstorms)

- **Outlier detection method** (Phase 1): exact statistical test (e.g. MAD vs. fixed-band deviation) for the 1.0% outlier slash.
- **Transfer-restriction model** (Phase 3): RESOLVED — separate keyed store for
  allow/deny membership; enforced via a bank SendRestrictionFn. See
  `specs/2026-05-29-phase-3-rwa-module-design.md` D4/D5.
- **Dispute resolution** (Phase 3): RESOLVED — governance-only `MsgSlashBond`
  force-settles the asset and routes the bond to the community pool. See
  `specs/2026-05-29-phase-3-rwa-module-design.md` D7.
- **Provider set + weighting** (Phase 4): **Resolved** — CoinGecko + Binance + a Static provider (Static bootstraps VTX:USD until listing); disagreement handled by a configurable cross-source median with strict defaults (min_providers=2, max_deviation=0.10, max_quote_age), skipping under-covered pairs. See [specs/2026-05-29-phase-4-feeder-sidecar-design.md](./specs/2026-05-29-phase-4-feeder-sidecar-design.md) §2 (D2/D3).
- **TPS target** (Phases 7/9): the concrete throughput/latency bar load tests must clear.
- **Day-1 IBC channel set** (Phase 10): final confirmation beyond Cosmos Hub.

Each is resolved when its phase is brainstormed; resolutions update the relevant phase section and `technical-design.md`.
