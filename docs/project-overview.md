# Vertix — Project Overview

**Chain:** Vertix (`vertix-1`) · **Ticker:** VTX · **Category:** Oracle · Real World Assets (Layer 1)
**Status:** Pre-launch — implementation in progress · **Target Mainnet:** Month 12

---

## 1. What is Vertix?

Vertix is a Cosmos SDK Layer 1 blockchain purpose-built as the settlement layer for **Real World Asset (RWA) tokenization**, powered by native **decentralized oracle infrastructure**.

Both pillars are equal first-class citizens of the protocol:

- **Oracle data feeds** exist to verify and price real-world assets.
- **RWA issuance** exists to settle assets whose trustlessness depends on those feeds.

VTX mirrors Bitcoin's supply discipline — **21 million hard cap, zero inflation** — while generating validator and staker rewards entirely from network fee revenue, supplemented by a Validator Incentives Pool during the bootstrap phase.

---

## 2. Why Vertix?

| Pain Point in RWA Today | Vertix's Answer |
|---|---|
| RWA platforms rely on opaque, centralized off-chain oracles | Validator-integrated, stake-secured price feeds (`x/oracle`) |
| Issuers can mint without skin-in-the-game | Mandatory VTX bond per asset class (`x/rwa`) |
| Token economics rely on dilution | Fee-funded security with 40% burn on a fixed 21M supply (`x/fees`) |
| Asset lifecycle is ad-hoc | On-chain state machine: `DRAFT → ATTESTED → ACTIVE → SETTLED` |
| Cross-chain RWA portability is hard | ICS-20 native at genesis; `rwa/*` denoms portable across IBC |

---

## 3. Core Pillars

### 3.1 Decentralized Oracle (`x/oracle`)
Every active validator runs an off-chain feeder sidecar (`vertix-feeder`) that submits prices each block window. Submissions are aggregated on-chain via **stake-weighted median** and stored as both spot prices and rolling **TWAP** windows (1h, 24h). Validators are slashed for missed feeds (0.5%) or outlier submissions (1.0%).

### 3.2 Real World Assets (`x/rwa`)
A protocol-agnostic asset registry. Issuers define their own asset classes (real estate, bonds, commodities — Vertix has no opinion on category); the protocol enforces lifecycle, bonding (min 10,000 VTX per asset), oracle attestation, factory denom minting (`rwa/{asset-id}`), transfer restrictions, and fee collection.

### 3.3 Fee Engine (`x/fees`)
Every block, all collected fees (gas + RWA mint/settle + oracle subscriptions) are split:
- **40% burned** permanently via `x/bank.BurnCoins`.
- **60% distributed** to VTX stakers via `x/distribution`.

Both ratios are governance-adjustable.

### 3.4 Tokenomics in One Picture

```
┌─────────────────────────────────────────────────────┐
│  Max Supply:  21,000,000 VTX  (hard cap, no mint)   │
│  Decimals:    6   (1 VTX = 1,000,000 uvtx)          │
│  Bech32:      vtx                                    │
├─────────────────────────────────────────────────────┤
│  Team           22%   Foundation       20%           │
│  Ecosystem      18%   Investors        13%           │
│  Validators     12%   Liquidity         8%           │
│  Airdrop         7%                                  │
├─────────────────────────────────────────────────────┤
│  Fee model:   40% burn  +  60% to stakers            │
│  Value:       work + yield + deflationary            │
└─────────────────────────────────────────────────────┘
```

See [`tokenomics.md`](./tokenomics.md) for full vesting schedules and value accrual model.

---

## 4. Tech Stack (Pinned)

| Component | Version |
|---|---|
| Cosmos SDK | v0.50.x |
| CometBFT | v0.38.x |
| ibc-go | v8.x |
| Go | 1.22+ |
| Ignite CLI | v28.x |
| Hermes Relayer | v1.8.x |
| Cosmovisor | v1.5.x |

---

## 5. Repository at a Glance

| Component | Path | Purpose |
|---|---|---|
| Chain binary | `cmd/vertixd/` | Node executable |
| App wiring | `app/app.go` | All SDK + custom modules wired here |
| Custom modules | `x/oracle/`, `x/rwa/`, `x/fees/` | Vertix's three native modules |
| Oracle sidecar | `feeder/` | Standalone `vertix-feeder` binary |
| Protobuf | `proto/vertix/{module}/v1/` | Source of truth for messages and types |
| End-to-end tests | `e2e/` | `interchaintest` IBC scenarios |
| Devnet & ops | `infra/devnet/`, `infra/hermes/`, `infra/monitoring/` | Local 3-validator stack |
| Docs | `docs/` | Spec, plans, architecture, this document |

For detailed paths see [`project-structure.md`](./project-structure.md).

---

## 6. Roadmap Snapshot (12 Months → Mainnet)

```
Month:  1    2    3    4    5    6    7    8    9   10   11   12
        ├────┼────┼────┼────┼────┼────┼────┼────┼────┼────┼────┤
Chain   [Phase 0─────────]
Oracle       [──Phase 1 (x/oracle)──────────]
RWA               [──Phase 2 (x/rwa)──────────]
Fees                        [Phase 3 (x/fees)]
IBC                              [─Phase 4─]
Devnet              [───Phase 5 (devnet)────]
Security                              [Phase 6]
Testnet v1               [────Phase 7────────]
Audit                                 [─Phase 8─]
Testnet v2                                   [Phase 9─]
Mainnet                                               [Launch]
```

Full milestone breakdown: [`roadmap.md`](./roadmap.md).

---

## 7. Documentation Map

| Doc | Audience | Purpose |
|---|---|---|
| [`project-overview.md`](./project-overview.md) | Everyone | This document |
| [`architecture.md`](./architecture.md) | Engineers, integrators | System architecture, components, data flow |
| [`technical-design.md`](./technical-design.md) | Module developers | Module internals, state, messages, events |
| [`project-structure.md`](./project-structure.md) | New contributors | Directory layout & file responsibilities |
| [`coding-standards.md`](./coding-standards.md) | All contributors | Go style, proto, testing, commits |
| [`tokenomics.md`](./tokenomics.md) | Token holders, validators | Supply, vesting, value accrual |
| [`roadmap.md`](./roadmap.md) | Stakeholders | 12-month plan to mainnet |
| [`full-design-spec.md`](./full-design-spec.md) | Architects | Approved full design spec (program source of truth) |
| [`specs/`](./specs/) | Architects | Per-phase design specs (from brainstorms) |
| [`plans/`](./plans/) | Agentic implementers | Step-by-step execution plans |

---

## 8. Status & Current Focus

| Phase | Status | Notes |
|---|---|---|
| Phase 0 — Chain Foundation | In progress | Plan: `plans/2026-05-09-01-chain-foundation.md` |
| Phase 1 — `x/oracle` | Planned | Plan: `plans/2026-05-09-02-oracle-module.md` |
| Phase 2 — `x/rwa` | Planned | Plan: `plans/2026-05-09-04-rwa-module.md` |
| Phase 3 — `x/fees` | Planned | Plan: `plans/2026-05-09-03-fees-module.md` |
| Phase 4 — IBC + Devnet | Planned | Plan: `plans/2026-05-09-06-ibc-devnet.md` |
| Phase 5 — Feeder Sidecar | Planned | Plan: `plans/2026-05-09-05-oracle-feeder-sidecar.md` |

---

## 9. Quick Links

- **Approved spec:** [`full-design-spec.md`](./full-design-spec.md)
- **Cosmos SDK docs:** https://docs.cosmos.network/
- **CometBFT docs:** https://docs.cometbft.com/
- **ibc-go docs:** https://ibc.cosmos.network/
- **Awesome Cosmos:** https://github.com/cosmos/awesome-cosmos
