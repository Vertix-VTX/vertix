# Vertix Blockchain — Full Design Spec

**Date:** 2026-05-09
**Status:** Approved
**Chain:** Vertix (`vertix-1`)
**Ticker:** VTX
**Category:** Oracle · Real World Assets (Layer 1)

---

## 1. Vision

Vertix is a Cosmos SDK Layer 1 blockchain purpose-built as the settlement layer for **Real World Asset (RWA) tokenization**, powered by native **decentralized oracle infrastructure**. Both are equal first-class citizens: oracle data feeds exist to verify and price real-world assets; RWA issuance exists to settle assets whose trustlessness depends on those feeds.

VTX mirrors Bitcoin's supply discipline — 21M hard cap, zero inflation — while generating validator and staker rewards entirely from network fee revenue, supplemented by an emissions pool during the bootstrap phase.

---

## 2. Tech Stack (Pinned Versions)

| Component | Version |
|---|---|
| Cosmos SDK | v0.50.x |
| CometBFT | v0.38.x |
| ibc-go | v8.x |
| Go | 1.22+ |
| Ignite CLI | v28.x (scaffolding + dev loop) |
| Hermes Relayer | v1.8.x |
| Cosmovisor | v1.5.x |

---

## 3. Architecture

### 3.1 Custom Native Modules

#### `x/oracle`
Validator-integrated price feed engine. All active validators run an off-chain oracle feeder sidecar that submits price data per configured denom pair every block window. Submissions are aggregated on-chain using a stake-weighted median. TWAP windows (1h, 24h) are stored per pair for RWA consumption.

**Key parameters (genesis):**

| Param | Default | Governance |
|---|---|---|
| `VoteWindow` | 10 blocks | Yes |
| `MissThreshold` | 5% of windows | Yes |
| `MissSlashRate` | 0.5% of bonded stake | Yes |
| `OutlierSlashRate` | 1.0% of bonded stake | Yes |
| `MinDenomPairs` | governance-set | Yes |

**Keeper interface (consumed by `x/rwa`):**
```go
type OracleKeeper interface {
    GetPrice(ctx sdk.Context, pair string) (sdk.Dec, error)
    GetTWAP(ctx sdk.Context, pair string, window time.Duration) (sdk.Dec, error)
}
```

**Oracle feeder sidecar:** A standalone Go binary (`vertix-feeder`) that validators run alongside their node. It fetches prices from external APIs, signs `MsgSubmitFeed` transactions, and broadcasts them each block. Validators are responsible for sidecar uptime; missed feeds are slashed.

---

#### `x/rwa`
Protocol-agnostic asset registry and lifecycle engine. Issuers define asset classes (real estate, bonds, commodities — the protocol has no opinion); Vertix enforces the lifecycle, bonding, oracle attestation, and fee collection.

**Asset lifecycle:**
```
DRAFT → ATTESTED → ACTIVE → SETTLED
           ↑            ↓
    (oracle price   (RWA tokens
     linked, bond    minted &
     locked)        transferable)
```

- `DRAFT`: Asset record created, metadata submitted, not yet tradable
- `ATTESTED`: Oracle price linked; issuer bond locked (min 10,000 VTX, governance-adjustable); passes validation
- `ACTIVE`: RWA denom minted via `x/bank` factory (`rwa/{asset-id}`); transfer restrictions enforced by module
- `SETTLED`: RWA tokens burned; bond returned on clean settlement; bond slashed on fraud/dispute

**Key parameters (genesis):**

| Param | Default | Governance |
|---|---|---|
| `MinIssuerBond` | 10,000 VTX | Yes |
| `MintFeeRate` | 0.10% of notional | Yes |
| `SettleFeeRate` | 0.10% of notional | Yes |

---

#### `x/fees`
Fee burn and distribution engine. Intercepts all collected module fees at `EndBlock` and splits them per governance-set ratios.

```
All fees (gas + RWA mint/settle + oracle data subscriptions)
    ├── BurnRatio (40%)  → x/bank.BurnCoins()
    └── DistRatio  (60%) → x/distribution FeePool → VTX stakers
```

**Events emitted:** `EventFeeBurned`, `EventFeeDistributed` (indexed for explorers and indexers).

**Key parameters (genesis):**

| Param | Default | Governance |
|---|---|---|
| `BurnRatio` | 40% | Yes |
| `DistributionRatio` | 60% | Yes |

---

### 3.2 Standard SDK Modules (wired, unmodified)

`x/auth`, `x/bank`, `x/staking`, `x/gov`, `x/distribution`, `x/slashing`, `x/upgrade`, `x/params`, `x/crisis`, `x/feegrant`, `x/authz`, `x/capability`, `x/ibc`, `x/transfer`

---

### 3.3 Inter-Module Data Flow

```
External APIs
    ↓
Oracle Feeder Sidecar (per validator)
    ↓  MsgSubmitFeed (each block)
x/oracle  ←──── slash via x/slashing
    ↓  OracleKeeper.GetPrice() / GetTWAP()
  x/rwa  ←──── issuer bonds VTX (x/bank escrow)
    ↓  mint rwa/{id} denom, collect fees
  x/fees (EndBlock)
    ├── 40% burned (x/bank.BurnCoins)
    └── 60% → x/distribution → VTX stakers
```

---

### 3.4 IBC Strategy

- ICS-20 token transfers: enabled at genesis (VTX and `rwa/*` denoms portable)
- Target mainnet IBC connections: Cosmos Hub, Osmosis, Noble, Neutron
- Future (v2): oracle price queries cross-chain via IBC queries (ICS-004)

---

## 4. Tokenomics

### 4.1 Denomination

| Field | Value |
|---|---|
| Base denom | `uvtx` |
| Display denom | `VTX` |
| Decimals | 6 |
| Conversion | 1 VTX = 1,000,000 uvtx |
| Total supply | 21,000,000 VTX = 21,000,000,000,000 uvtx |
| Inflation | None — hard cap, zero mint after genesis |

---

### 4.2 Token Allocation

| Category | % | VTX | Rationale |
|---|---|---|---|
| Team & Core Contributors | 22% | 4,620,000 | Core execution team |
| Foundation / Treasury | 20% | 4,200,000 | Protocol sustainability & ops |
| Strategic Investors | 13% | 2,730,000 | Seed + strategic round |
| Ecosystem & Grants | 18% | 3,780,000 | Developer adoption |
| Validator Incentives Pool | 12% | 2,520,000 | Bootstrap fee emissions |
| Public Liquidity | 8% | 1,680,000 | DEX + market making |
| Community Airdrop | 7% | 1,470,000 | Genesis distribution |
| **Total** | **100%** | **21,000,000** | |

Team + Foundation = **42%** (execution-focused)

---

### 4.3 Vesting Schedules

| Category | Cliff | Linear Vesting | SDK Type | Notes |
|---|---|---|---|---|
| Team | 12 months | 36 months | PeriodicVesting | 4 years total |
| Foundation | 12 months | 48 months | PeriodicVesting | Governance multisig |
| Investors | 6 months | 18 months | PeriodicVesting | Monthly unlocks |
| Ecosystem & Grants | None | 48 months quarterly | PeriodicVesting | Grants committee |
| Validator Incentives | None | 60 months linear | ContinuousVesting | Oracle + block rewards |
| Public Liquidity | 50% at TGE | 6 months linear | ContinuousVesting | DEX bootstrap |
| Airdrop | None | None | BaseAccount | Claimable at genesis |

---

### 4.4 Value Accrual Model

VTX is a **work token + yield token + deflationary asset**:

1. **Work token:** Validators must bond VTX and run oracle feeders. RWA issuers must bond VTX per asset class. Participation requires skin in the game.
2. **Yield token:** 60% of all network fees flow to VTX stakers proportional to bonded stake.
3. **Deflationary:** 40% of all fees are permanently burned. On a 21M hard cap, burn creates real scarcity pressure as network activity grows.

**Supply reduction levers:**
- 40% fee burn (permanent)
- RWA issuer bonds (locked per active asset)
- Staker unbonding period (21 days, VTX illiquid)
- Validator bonds (active set, continually locked)

---

### 4.5 Bootstrap → Mature Transition

```
Phase         Validator Rewards Source
──────────────────────────────────────────────────────
Months 0–18   Validator Incentives Pool (2.52M VTX) + growing fees
Months 18–36  Declining pool emissions + majority from fees
Month 36+     Pure fee-driven rewards (pool depleted)
```

This mirrors Bitcoin's long-run model but with RWA + oracle fees replacing miner fees.

---

## 5. Key Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Oracle model | Validator-integrated (A) | Proven in Cosmos (Umee, Kujira); simple to audit; migratable via upgrade |
| RWA scope | Protocol-agnostic (C) | Maximizes addressable market; compliance is issuer's concern |
| Value accrual | Bond + fee share + burn (D) | Strongest economic loop on a fixed 21M supply |
| Distribution | Execution-focused (C) | Team + treasury = 42%; fund aggressive development |
| Timeline | 12-month aggressive (A) | Parallel workstreams; all modules ship together |
| Approach | Monolithic full stack (1) | Full vision at launch; no perception of deferred features |
| Inflation | Zero | Hard cap integrity; fee revenue funds security from day one |

---

## 6. Security Considerations

- **Oracle slashing:** Separate slash conditions for missed feeds (0.5%) vs. outlier submissions (1%). Oracle slash is lighter than double-sign (5%) to avoid validator chilling.
- **RWA bond slashing:** Dispute resolution module can slash issuer bond on fraud findings. Governance vote required for bond confiscation above threshold.
- **Key custody:** `tmkms` (Tendermint KMS) required for all mainnet validators. Setup guide in `docs/tmkms.md`.
- **Upgrade safety:** All upgrades via `x/upgrade` governance. Migration handlers required for any store key changes. Dry-run in testnet v2 before mainnet.
- **IBC client expiry:** Relayer uptime monitoring required. Expired clients on IBC channels are a liveness risk. Hermes `client_refresh` strategy configured per channel.
- **Audit scope:** `x/oracle`, `x/rwa`, `x/fees`, custom ante handlers. Target auditors with Cosmos SDK experience (Oak Security, Halborn, Trail of Bits).

---

## 7. Infrastructure & Ops

### Validator Architecture
```
Internet
    ↓
Sentry Node (public P2P) ←→ Sentry Node (public P2P)
    ↓                              ↓
Validator Node (private, tmkms) + Oracle Feeder Sidecar
```

### Observability Stack
- Prometheus metrics: CometBFT + SDK + OS (node-exporter)
- Grafana dashboards: block time, missed oracle feeds, fee burn rate, active RWA count
- Tenderduty: missed block alerts
- PANIC: validator health monitoring

### Snapshot & State Sync
- Snapshot cadence: every 1000 blocks
- State sync enabled in `config.toml` for fast validator onboarding
- Public snapshot service published in chain registry

---

## 8. Ecosystem Integrations (Mainnet + Post-Mainnet)

| Integration | Target |
|---|---|
| Keplr wallet | Mainnet launch |
| Leap wallet | Mainnet launch |
| Ping.pub explorer | Mainnet launch |
| IBC: Cosmos Hub | Mainnet launch |
| IBC: Osmosis | Month 13 |
| IBC: Noble (USDC) | Month 13 |
| IBC: Neutron | Month 13 |
| Big Dipper + BDJuno | Month 14 |
| cosmos/chain-registry PR | Mainnet launch |

---

## 9. Open Questions (Post-Spec)

- RWA dispute resolution: governance vote vs. dedicated arbitration module?
- Oracle data subscriptions: should external protocols pay VTX to query Vertix oracle data cross-chain?
- CosmWasm integration: optional smart contract layer in post-mainnet roadmap?
- Oracle v2 upgrade: timeline and trigger conditions for migrating to hybrid stake-weighted model?
