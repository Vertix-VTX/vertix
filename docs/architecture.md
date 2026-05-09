# Vertix — Architecture

This document describes the high-level architecture of the Vertix Layer 1 blockchain: components, boundaries, data flow, and operational topology. For module internals (state stores, messages, events, slashing math) see [`technical-design.md`](./technical-design.md).

---

## 1. System Context

```
                ┌─────────────────────────────────────────────────────┐
                │                External World                       │
                │                                                     │
                │   Price APIs       RWA Issuers      End Users        │
                │  (CoinGecko,       (Real estate,   (Stakers,         │
                │   Binance, ...)     bonds, etc.)    traders)         │
                └────────┬─────────────────┬───────────────┬───────────┘
                         │                 │               │
                         │ HTTP            │ Tx            │ Tx / Query
                         ▼                 ▼               ▼
                ┌─────────────────────────────────────────────────────┐
                │                 Vertix Network                      │
                │                                                     │
                │   ┌──────────────────────────────────────────┐      │
                │   │   Validators × N (active set)            │      │
                │   │   ┌──────────────┐  ┌─────────────────┐  │      │
                │   │   │ vertixd node │◄─┤ vertix-feeder   │  │      │
                │   │   │ (CometBFT +  │  │ (sidecar        │  │      │
                │   │   │  SDK app)    │  │  per validator) │  │      │
                │   │   └──────┬───────┘  └─────────────────┘  │      │
                │   │          │                               │      │
                │   │   gossip │ p2p (sentries)                │      │
                │   └──────────┼───────────────────────────────┘      │
                │              ▼                                      │
                │   Consensus: CometBFT v0.38                         │
                │              │                                      │
                │              ▼                                      │
                │   Application: Vertix SDK app (custom + std)        │
                │                                                     │
                └─────────┬─────────────────────────────────┬─────────┘
                          │ IBC (ICS-20)                    │
                          ▼                                 ▼
                  Cosmos Hub, Osmosis,          Block explorers, indexers,
                  Noble, Neutron, ...           wallets (Keplr, Leap)
```

---

## 2. Layered Architecture of the Node

The `vertixd` binary is a standard Cosmos SDK app composed of three layers:

```
┌────────────────────────────────────────────────────────────────┐
│  Networking & Consensus            (CometBFT v0.38.x)          │
│   - p2p gossip  - mempool  - block production  - state sync    │
└─────────────────────┬──────────────────────────────────────────┘
                      │ ABCI++
┌─────────────────────▼──────────────────────────────────────────┐
│  BaseApp & Module Manager          (Cosmos SDK v0.50.x)        │
│   - tx routing  - ante handlers  - begin/end blockers  - gov   │
└─────────────────────┬──────────────────────────────────────────┘
                      │
┌─────────────────────▼──────────────────────────────────────────┐
│  Modules                                                       │
│                                                                │
│   Custom (Vertix):     x/oracle    x/rwa    x/fees             │
│                                                                │
│   Standard SDK:        x/auth   x/bank   x/staking   x/gov     │
│                        x/distribution  x/slashing  x/upgrade   │
│                        x/params  x/crisis  x/feegrant  x/authz │
│                        x/capability  x/ibc  x/transfer         │
│                        x/genutil  x/evidence  x/vesting        │
│                                                                │
│   Removed:             x/mint   ← enforces 21M hard cap        │
└────────────────────────────────────────────────────────────────┘
```

Removing `x/mint` is a deliberate architectural choice. It guarantees no future inflation can be introduced via parameter change — only a hard fork could re-enable minting. Validator security is funded by `x/distribution` from fee revenue (and the Validator Incentives Pool during bootstrap).

---

## 3. Custom Modules and Their Boundaries

### 3.1 `x/oracle` — Validator-Integrated Price Feeds

```
                  ┌─────────────────────────┐
   external API ─►│   vertix-feeder (Go)    │
                  │   per-validator sidecar │
                  └────────────┬────────────┘
                               │ MsgSubmitFeed (each block)
                               ▼
       ┌──────────────────────────────────────────────────┐
       │ x/oracle keeper                                  │
       │                                                  │
       │   FeedStore (per validator × pair)               │
       │        │                                         │
       │        │ EndBlock (every VoteWindow blocks)      │
       │        ▼                                         │
       │   Aggregator: stake-weighted median              │
       │        │                                         │
       │        ▼                                         │
       │   AggregatedPrice store ──► TWAP ring buffer     │
       │                                                  │
       │   MissCounter ──► slash via x/slashing           │
       └──────────────────────────────────────────────────┘
                          │
                          │ OracleKeeper.GetPrice / GetTWAP
                          ▼
                       x/rwa
```

**Public keeper interface (consumed by `x/rwa`):**

```go
type OracleKeeper interface {
    GetPrice(ctx sdk.Context, pair string) (math.LegacyDec, error)
    GetTWAP(ctx sdk.Context, pair string, window time.Duration) (math.LegacyDec, error)
}
```

### 3.2 `x/rwa` — Asset Lifecycle & Issuance

```
   Issuer
     │  MsgRegisterAsset (locks bond ≥ 10,000 VTX)
     ▼
   ┌─────────┐    MsgAttestAsset      ┌──────────┐    MsgMintRWA     ┌────────┐
   │  DRAFT  │ ──────────────────────►│ ATTESTED │ ─────────────────►│ ACTIVE │
   └─────────┘  (oracle price linked) └──────────┘  (rwa/{id} mint)  └────┬───┘
                                                                          │
                                                            MsgSettleRWA  │
                                                          (burn + return  │
                                                           or slash bond) │
                                                                          ▼
                                                                     ┌────────┐
                                                                     │SETTLED │
                                                                     └────────┘
```

- **Bond escrow:** managed via `x/bank` module account.
- **Token mint:** uses `x/bank` factory denom `rwa/{asset-id}`.
- **Transfer restrictions:** allowlist/denylist enforced at the message handler level.
- **Fees on mint and settle:** 0.1% of notional each, sent to the fee collector (consumed by `x/fees`).

### 3.3 `x/fees` — Burn + Distribute Engine

`x/fees` has no custom state of its own. Each `EndBlock` it sweeps the standard fee collector account (`auth.FeeCollectorName`) and routes funds:

```
                   Fee Collector Account
                   (gas + RWA mint/settle + future oracle subs)
                              │
                              │ EndBlock sweep
                              ▼
                      ┌───────────────────┐
                      │   x/fees split    │
                      └─┬─────────────────┘
                        │
            BurnRatio   │   DistributionRatio
            (40%)       │   (60%)
                        │
            ┌───────────┘                  ┌──────────────┐
            ▼                              ▼              │
   x/bank.BurnCoins              x/distribution.FeePool   │
   (permanent supply             ──► VTX stakers          │
    reduction on 21M)                proportional         │
                                     to bonded stake      │
                                                          │
   Events: EventFeeBurned          Events: EventFeeDistributed
```

Both ratios are governance-adjustable via `MsgUpdateParams` (authority = gov module address).

---

## 4. Inter-Module Data Flow (One Asset Lifecycle)

```
[Issuer]
   │
   │ MsgRegisterAsset (asset_id, oracle_pair, bond=10000VTX)
   ▼
x/rwa ──────► x/bank (escrow bond)
   │
   │ MsgAttestAsset
   ▼
x/rwa ──────► x/oracle.GetPrice(pair) ─► returns aggregated median
   │                                  ► attest → ATTESTED
   │
   │ MsgMintRWA (notional)
   ▼
x/rwa ──────► x/bank.MintCoins(rwa/{id})
   │      ──► x/bank.SendCoins(holder)
   │      ──► fee collector (0.1% of notional)
   │
   │ ... transfers (with restriction checks) ...
   │
   │ MsgSettleRWA
   ▼
x/rwa ──────► x/bank.BurnCoins(rwa/{id})
   │      ──► fee collector (0.1% of notional)
   │      ──► x/bank (release bond OR slash on dispute)
   │
   ▼
[End of block]
   │
   ▼
x/fees EndBlock
   │
   ├─► 40% burn (x/bank.BurnCoins of uvtx fees)
   └─► 60% to x/distribution.FeePool ─► stakers next withdraw cycle
```

---

## 5. Off-Chain Components

### 5.1 `vertix-feeder` Sidecar

A standalone Go binary that **every active validator must run**. Architecture:

```
                ┌──────────────────────────────────────┐
                │           vertix-feeder              │
                │                                      │
   ┌──────┐     │   ┌────────────┐                    │
   │ APIs │────►│──►│ Providers  │                    │
   └──────┘     │   │ (CoinGecko,│                    │
                │   │  Binance)  │                    │
                │   └─────┬──────┘                    │
                │         │ median across providers   │
                │         ▼                           │
                │   ┌────────────┐                    │
                │   │ Broadcaster│                    │
                │   │ - sign tx  │                    │
                │   │ - retry    │                    │
                │   └─────┬──────┘                    │
                │         │ MsgSubmitFeed             │
                │         ▼                           │
                │   ┌────────────┐    Prometheus :9200│
                │   │ vertixd    │◄───┐  feeds_total, │
                │   │  gRPC      │    │  errors_total │
                │   └────────────┘    │               │
                └─────────────────────┴───────────────┘
```

- Configurable tick interval (default 5s, within 10-block windows).
- Fetches from N providers, takes the cross-source median, broadcasts `MsgSubmitFeed`.
- Exposes Prometheus metrics for SLO monitoring.
- Validators are **responsible for their own sidecar uptime**; missed feeds are slashed by `x/oracle`.

### 5.2 Validator Architecture (Production)

```
                     Internet
                        │
              ┌─────────┴─────────┐
              ▼                   ▼
        ┌──────────┐         ┌──────────┐
        │ Sentry 1 │ ◄─────► │ Sentry 2 │   (public p2p)
        └────┬─────┘         └────┬─────┘
             └────────┬───────────┘
                      ▼
              ┌────────────────┐
              │  Validator     │  ◄── tmkms (remote signer)
              │  (private)     │
              │  + vertix-     │
              │    feeder      │
              └────────────────┘
```

- **`tmkms`** required on mainnet for consensus key custody.
- **Sentry nodes** isolate validators from public p2p.

---

## 6. Cross-Chain (IBC) Architecture

```
   ┌────────────┐                      ┌──────────────┐
   │   Vertix   │ ◄──── ICS-20 ──────► │ Cosmos Hub   │
   │ (vertix-1) │                      └──────────────┘
   │            │ ◄──── ICS-20 ──────► ┌──────────────┐
   │  VTX +     │                      │  Osmosis     │
   │  rwa/*     │                      └──────────────┘
   │  denoms    │ ◄──── ICS-20 ──────► ┌──────────────┐
   │            │                      │  Noble (USDC)│
   └────────────┘                      └──────────────┘
         ▲
         │ Hermes v1.8.x relayer
         │ (or rly Go relayer)
```

- ICS-20 enabled at genesis for both VTX and `rwa/*` denoms.
- Mainnet Day-1 channels: Cosmos Hub. Month 13: Osmosis, Noble, Neutron.
- v2 future: cross-chain oracle queries via ICS-004.

---

## 7. Operational Stack (Devnet → Mainnet)

```
┌─────────────────────────────────────────────────────────────┐
│  Operational Layer                                          │
│                                                             │
│  Observability:    Prometheus + Grafana                     │
│                    (CometBFT, SDK, oracle, fees, RWA)       │
│  Alerting:         Tenderduty (missed blocks)               │
│                    PANIC (validator health)                 │
│  Explorer:         Ping.pub (devnet, testnet, mainnet)      │
│                    Big Dipper + BDJuno (post-mainnet)       │
│  Faucet:           Cosmfaucet (devnet + testnet)            │
│  Snapshots:        every 1000 blocks; chain-registry hosted │
│  Upgrades:         Cosmovisor v1.5.x for binary swap        │
└─────────────────────────────────────────────────────────────┘
```

The 3-validator local devnet (`infra/devnet/docker-compose.yml`) mirrors this stack at smaller scale for development.

---

## 8. Security Architecture

| Layer | Mitigation |
|---|---|
| Consensus key | `tmkms` remote signer; HSM backing for mainnet |
| Network | Sentry nodes; private validator p2p; firewall rules |
| Oracle data | Stake-weighted median; outlier slashing 1.0%; miss slashing 0.5% |
| RWA fraud | Issuer bond (≥10,000 VTX) slashable via governance |
| Upgrades | `x/upgrade` + Cosmovisor; mandatory testnet v2 dry run |
| Module code | External audit (Oak / Halborn / Trail of Bits) before mainnet |
| State invariants | `x/crisis` invariants for oracle, RWA, fees |
| Static analysis | `cosmos-sdk-codeql` in CI |
| IBC liveness | Relayer monitoring; Hermes `client_refresh` configured |

See [`technical-design.md`](./technical-design.md) §6 for slash math and invariant definitions.

---

## 9. Architectural Invariants

These are non-negotiable properties of the system that any change must preserve:

1. **Hard cap.** Total VTX supply ≤ 21,000,000 forever. `x/mint` is not registered. Burns are permanent and monotonic.
2. **No silent inflation.** No module may issue VTX outside genesis, vesting unlocks, or `x/staking` rewards funded by fee/pool revenue.
3. **Validator-integrated oracle.** Oracle authority = active validator set. Sidecar is mandatory; misses are slashed.
4. **RWA bonding.** No asset reaches `ACTIVE` without a locked issuer bond ≥ `MinIssuerBond`.
5. **Fee determinism.** `EndBlock` fee split is fully on-chain; off-chain code may not influence the burn/distribute ratios.
6. **Standard module integrity.** Standard SDK modules are wired unmodified to preserve auditability and ecosystem compatibility.

---

## 10. Trade-offs & Decisions

| Decision | Choice | Trade-off accepted |
|---|---|---|
| Oracle model | Validator-integrated | Tighter validator ops requirements vs. simpler economic model |
| RWA scope | Protocol-agnostic | Compliance pushed to issuer vs. broader market coverage |
| Inflation | Zero | Bootstrap rewards depend on Pool until fees scale |
| Approach | Monolithic launch (oracle + RWA + fees together) | Higher coordination cost vs. complete vision at launch |
| Smart contracts | Not at launch (CosmWasm post-mainnet) | Less programmability vs. smaller audit surface |

Full rationale is in the approved spec: [`specs/2026-05-09-vertix-blockchain-design.md`](./specs/2026-05-09-vertix-blockchain-design.md) §5.
