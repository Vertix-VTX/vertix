# Vertix Blockchain — Full Roadmap

**Chain:** Vertix (`vertix-1`) | **Ticker:** VTX | **Target Mainnet:** Month 12
**Tech Stack:** Cosmos SDK v0.50.x · CometBFT v0.38.x · ibc-go v8.x · Go 1.22+

---

## Timeline Overview

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
Mainnet                                               [🚀]
```

---

## Phase 0 — Foundation `Month 0–1`

**Goal:** Working skeleton chain, CI/CD, and tooling locked.

### Tasks
- [ ] Scaffold chain: `ignite scaffold chain vertix --no-module`
- [ ] Configure chain identity in `config.yml`:
  - App name: `vertixd`
  - Bech32 prefix: `vtx`
  - Base denom: `uvtx`
  - Chain ID: `vertix-devnet-1`
- [ ] Wire all standard SDK modules in `app/app.go`:
  - `x/auth`, `x/bank`, `x/staking`, `x/gov`, `x/distribution`, `x/slashing`
  - `x/upgrade`, `x/params`, `x/crisis`, `x/feegrant`, `x/authz`
  - `x/capability`, `x/ibc`, `x/transfer`
- [ ] Configure genesis parameters:
  - Bond denom: `uvtx`
  - Unbonding time: 21 days
  - Min validator commission: 5%
  - Min gas price: `0.025uvtx`
- [ ] Repo infrastructure:
  - Makefile targets: `build`, `test`, `lint`, `proto-gen`, `ts-gen`
  - `golangci-lint` config
  - `buf` + `protoc` for protobuf
  - Pre-commit hooks: `gofmt`, lint, proto-breaking checks
- [ ] CI/CD: GitHub Actions — build, lint, unit test on every PR
- [ ] Run chain: `ignite chain serve --reset-once`

**Acceptance Criteria:**
- Chain boots and produces blocks
- REST (`:1317`), gRPC (`:9090`), RPC (`:26657`) endpoints available
- All standard modules pass genesis validation

---

## Phase 1 — `x/oracle` Module `Month 1–4`

**Goal:** Validator-integrated price feed live on devnet with slashing.

### Architecture
- All active validators run an **oracle feeder sidecar** (`vertix-feeder` binary)
- Sidecar fetches prices from external APIs, signs `MsgSubmitFeed`, broadcasts each block
- On-chain aggregation: stake-weighted median per denom pair per vote window (10 blocks default)
- TWAP windows: 1h and 24h stored per pair

### Tasks
- [ ] Scaffold module: `ignite scaffold module oracle --dep staking,slashing,params`
- [ ] Define protobuf types:
  - `MsgSubmitFeed` — validator price submission
  - `MsgUpdateParams` — governance param updates
  - `QueryGetPrice` — spot price query
  - `QueryGetTWAP` — TWAP query by pair + window
  - `OracleFeed`, `AggregatedPrice`, `OracleParams`
- [ ] Generate code: `ignite generate proto-go`
- [ ] Implement keeper logic:
  - Vote round collection per block
  - Stake-weighted median aggregation at end of vote window
  - TWAP calculation (rolling 1h / 24h per pair)
  - Miss counter per validator per window
- [ ] Implement slashing:
  - Miss rate > 5% of windows → slash 0.5% of bonded stake via `x/slashing`
  - Outlier submission (statistical) → slash 1.0%
- [ ] Implement clean keeper interface for `x/rwa` consumption:
  ```go
  GetPrice(ctx, pair string) (sdk.Dec, error)
  GetTWAP(ctx, pair string, window time.Duration) (sdk.Dec, error)
  ```
- [ ] Build `vertix-feeder` sidecar binary (Go):
  - Configurable price source APIs
  - Signs and broadcasts `MsgSubmitFeed` each block
  - Metrics: feeds submitted, misses, latency
- [ ] Emit events: `EventFeedSubmitted`, `EventPriceAggregated`, `EventOracleSlash`
- [ ] Add module params to genesis with validation
- [ ] Unit tests: aggregation logic, TWAP, miss counting, slash conditions
- [ ] Integration tests: full vote round simulation on local node

**Acceptance Criteria:**
- Price feeds aggregate correctly across simulated validators
- TWAP windows populate and expire correctly
- Slash events fire at correct miss/outlier thresholds
- `vertixd q oracle price [pair]` and `q oracle twap [pair] [window]` work via CLI

---

## Phase 2 — `x/rwa` Module `Month 2–5`

**Goal:** Protocol-agnostic asset registry and lifecycle on devnet.

### Asset Lifecycle
```
DRAFT → ATTESTED → ACTIVE → SETTLED
           ↑                    ↓
    (oracle price linked,  (RWA tokens burned,
     issuer bond locked)    bond returned or slashed)
```

### Tasks
- [ ] Scaffold module: `ignite scaffold module rwa --dep bank,oracle,params`
- [ ] Define protobuf types:
  - `MsgRegisterAsset` — issuer creates asset record
  - `MsgAttestAsset` — link oracle price, transition to ATTESTED
  - `MsgMintRWA` — mint RWA tokens once ATTESTED
  - `MsgTransferRWA` — transfer with module-enforced restrictions
  - `MsgSettleRWA` — burn tokens, release bond, transition to SETTLED
  - `AssetRecord`, `AssetStatus` enum, `RWAParams`
- [ ] Implement state machine transitions with validation at each step
- [ ] Implement issuer bonding:
  - `MsgRegisterAsset` locks `MinIssuerBond` (default 10,000 VTX) in module escrow
  - Bond returned on `SETTLED`; slashed on dispute
- [ ] Implement `rwa/{asset-id}` denom minting via `x/bank` factory denoms
- [ ] Implement transfer restriction hooks (allowlist/denylist per asset, set by issuer)
- [ ] Integrate `OracleKeeper.GetPrice()` for attestation validation
- [ ] Fee collection on mint + settle (0.1% of notional each), forwarded to `x/fees`
- [ ] Emit events: `EventAssetRegistered`, `EventAssetAttested`, `EventRWAMinted`, `EventRWASettled`
- [ ] Genesis params + validation
- [ ] Unit tests: state machine transitions, bond escrow, fee calculation, restriction enforcement
- [ ] Integration test: full lifecycle — register → attest → mint → transfer → settle

**Acceptance Criteria:**
- Full asset lifecycle executable via CLI
- Oracle price required and validated at attestation
- Issuer bond locked and released correctly
- Transfer restrictions enforced by module
- Fees collected and forwarded to `x/fees`

---

## Phase 3 — `x/fees` Module `Month 4–5`

**Goal:** Automated fee burn and staker distribution live.

### Tasks
- [ ] Scaffold module: `ignite scaffold module fees --dep bank,distribution,params`
- [ ] Implement `EndBlock` fee collection hook:
  - Sweep all module fee accounts each block
  - Apply `BurnRatio` (40%): call `x/bank.BurnCoins()`
  - Apply `DistributionRatio` (60%): send to `x/distribution.FeePool`
- [ ] Implement governance-adjustable params: `BurnRatio`, `DistributionRatio`
- [ ] Emit events: `EventFeeBurned` (amount), `EventFeeDistributed` (amount)
- [ ] Integration test: full fee flow from RWA mint → burn confirmed + distribution confirmed

**Acceptance Criteria:**
- Fee burn visible in bank total supply reduction
- Staker rewards reflect distributed fees via `x/distribution` queries
- Events indexed correctly (verifiable via block explorer)
- Governance can update burn/distribution ratios via proposal

---

## Phase 4 — IBC Enablement `Month 5–6`

**Goal:** VTX and RWA denoms transferable cross-chain.

### Tasks
- [ ] Confirm `x/ibc` + `x/transfer` (ICS-20) wired in `app.go`
- [ ] Configure `transfer` module params in genesis
- [ ] Local multi-chain test with `interchaintest`:
  - Two Vertix nodes + Hermes relayer
  - ICS-20 transfer of VTX between chains
  - ICS-20 transfer of `rwa/{id}` denom between chains
- [ ] Provide relayer setup recipes in `docs/relayer.md`:
  - Hermes (`v1.8.x`) config for Vertix channels
  - Go Relayer (`rly`) alternative config
- [ ] Target IBC channel connections for mainnet (docs):
  - Cosmos Hub, Osmosis, Noble (USDC), Neutron

**Acceptance Criteria:**
- ICS-20 VTX transfers succeed in `interchaintest` suite
- RWA denom transfers work cross-chain
- Relayer setup documented and reproducible

---

## Phase 5 — Devnet `Month 3–6` *(runs parallel to module development)*

**Goal:** Stable internal multi-validator network with all modules live.

### Tasks
- [ ] Single-node devnet → multi-validator devnet (3 internal validators)
- [ ] Oracle feeders deployed per internal validator
- [ ] Block explorer: Ping.pub configured for `vertix-devnet-1`
- [ ] Faucet: Cosmfaucet with rate limiting
- [ ] Automated devnet reset + seed scripts (`make devnet-reset`)
- [ ] Internal QA: full RWA issuance + oracle feed + fee burn + IBC cycle demonstrated end-to-end

**Acceptance Criteria:**
- 3-validator devnet stable for 1+ week
- Oracle feeds aggregating correctly across all validators
- Full RWA lifecycle and fee burn visible in explorer
- Faucet operational

---

## Phase 6 — Security Hardening `Month 6–7`

**Goal:** Audit-ready codebase.

### Tasks
- [ ] Register all keeper invariants with `x/crisis`:
  - Oracle: aggregate prices exist for all configured pairs
  - RWA: all ACTIVE assets have bonded issuer stake
  - Fees: module account balances reconcile with burn + distribution totals
- [ ] Simulation tests: random operations, state machine fuzzing for all three custom modules
- [ ] Static analysis: `cosmos-sdk-codeql` in CI
- [ ] Gas metering audit:
  - No unbounded loops in oracle aggregation
  - No unbounded enumeration in RWA registry queries
- [ ] Adversarial testing: oracle feed manipulation scenarios, RWA bond bypass attempts
- [ ] Load testing with `tm-load-test`: establish baseline TPS and latency on devnet
- [ ] `tmkms` setup guide for validators: `docs/tmkms.md`
- [ ] Sentry node architecture documented: `docs/validator-setup.md`

**Acceptance Criteria:**
- No invariant violations under simulation
- All `cosmos-sdk-codeql` findings resolved or documented
- TPS baseline established and documented
- Validator security guide complete

---

## Phase 7 — Public Testnet v1 `Month 7–9`

> **Canonical spec numbering:** this is **Phase 8** (Public Testnet v1) per [`full-design-spec.md`](./full-design-spec.md); live 4-week endurance + 10+ external validators are tracked in [`docs/testnet-runbook.md`](./testnet-runbook.md).

**Goal:** Battle-test with external validators and real users.

### Tasks
- [x] Chain ID: `vertix-testnet-1`
- [x] Genesis tooling + reproducible builder (`scripts/testnet/build-genesis.sh`, `make testnet-genesis`)
- [x] Founder core infra (`infra/testnet/docker-compose.public.yml`)
- [x] External-validator node kit (`infra/testnet/node-kit/`)
- [ ] Onboard 10+ external validators (docs: `docs/validator-onboarding.md`)
- [x] Public faucet live (Cosmfaucet with VTX testnet tokens)
- [x] Bug bounty program launched (`SECURITY.md`, `docs/bug-bounty.md`)
- [ ] External validators run oracle feeder sidecars independently
- [x] RWA demo campaign:
  - Example asset classes registered, attested, minted, and settled publicly (`scripts/testnet/rwa-demo.sh`, `rwa-dispute-demo.sh`)
  - Tutorial: `docs/rwa-quickstart.md`
- [x] Monitoring stack deployed:
  - Prometheus + Grafana dashboards (block time, oracle miss rate, fee burn rate, RWA activity)
  - Tenderduty: missed block alerts for validator operators
  - PANIC: general validator health monitoring
- [x] Issue templates + triage rubric (`.github/ISSUE_TEMPLATE/`)
- [x] Live-ops runbook: `docs/testnet-runbook.md`
- [ ] Collect and triage all public issues → prioritize for testnet v2

**Acceptance Criteria:**
- Testnet stable for 4+ weeks with 10+ external validators
- Oracle feeds operating across independent validator sidecars
- At least one full RWA lifecycle (register → settle) demonstrated publicly
- No critical bugs unaddressed from public reports

---

## Phase 8 — External Security Audit `Month 8–10`

**Goal:** Third-party code review of all custom modules before mainnet.

### Tasks
- [ ] Engage auditor with Cosmos SDK experience (Oak Security, Halborn, or Trail of Bits)
- [ ] Audit scope: `x/oracle`, `x/rwa`, `x/fees`, custom ante handlers, genesis config
- [ ] Fix all **Critical** and **High** severity findings
- [ ] Mitigate or formally accept all **Medium** findings
- [ ] Publish audit report publicly on completion
- [ ] Re-audit any critical components with changes post-fix

**Acceptance Criteria:**
- Zero unresolved Critical or High findings
- Published audit report
- All fixes verified by auditor

---

## Phase 9 — Public Testnet v2 + Genesis Rehearsal `Month 10–11`

**Goal:** Mainnet-equivalent dry run.

### Tasks
- [ ] Integrate all audit fixes
- [ ] Chain ID: `vertix-testnet-2`
- [ ] Cosmovisor end-to-end test: upgrade handler, binary swap, no downtime
- [ ] Genesis ceremony rehearsal:
  - Collect `gentx` from all participating mainnet validators
  - `collect-gentxs`, verify SHA256 of genesis.json
  - Coordinated start at predetermined block time
- [ ] Load test: confirm TPS target under realistic oracle + RWA workload on testnet v2 params
- [ ] Chain Registry preparation:
  - `chain.json` + `assetlist.json` drafted for `cosmos/chain-registry`
  - RPC/LCD/gRPC endpoints confirmed
- [ ] Wallet configs verified: Keplr and Leap `suggestChain` JSON tested
- [ ] Final documentation review: README, module docs, validator guide, RWA quickstart

**Acceptance Criteria:**
- Testnet v2 stable 2+ weeks post-audit fixes
- Genesis ceremony rehearsal completed without issues
- Chain Registry PR drafted and under review
- Cosmovisor upgrade tested successfully

---

## Phase 10 — Mainnet Launch `Month 12`

**Goal:** `vertix-1` live, blocks producing, ecosystem connected.

### Tasks
- [ ] Finalize chain ID: `vertix-1`, version tag, binary release
- [ ] Distribute final `genesis.json` + SHA256 to all validators
- [ ] Publish seed nodes, persistent peers, and addrbook
- [ ] Coordinated genesis ceremony with 20+ validators
- [ ] Confirm all validators verify genesis hash and start at same time
- [ ] Open IBC channels: Cosmos Hub, Osmosis, Noble
- [ ] Chain Registry PR merged
- [ ] Keplr + Leap live with `vertix-1` chain info
- [ ] Ping.pub block explorer: mainnet config live
- [ ] Public RPC/LCD/gRPC endpoints published in chain registry
- [ ] Monitoring: full Prometheus + Grafana + Tenderduty stack on mainnet
- [ ] Announcement: blog post, docs site, community channels
- [ ] Incident response channels established (Discord, on-call rotation)

**Acceptance Criteria:**
- `vertix-1` producing blocks with 20+ validators
- All airdrop and vesting accounts present in genesis
- IBC channels open and active
- Explorer showing live blocks and transactions
- No critical incidents in first 48 hours

---

## Post-Mainnet Roadmap (Month 13–18)

| Milestone | Target Month | Notes |
|---|---|---|
| IBC: Osmosis liquidity pools | 13 | VTX/USDC, VTX/ATOM |
| IBC: Neutron smart contracts | 13 | RWA composability |
| First institutional RWA issuance | 14 | Partner announcement |
| Big Dipper + BDJuno full indexer | 14 | GraphQL API for builders |
| DEX listings coordination | 15 | CEX/DEX VTX pairs |
| Oracle v2 upgrade proposal | 16 | Hybrid stake-weighted model |
| CosmWasm integration (optional) | 18 | Smart contract layer |
| Additional IBC channels | Ongoing | Per community governance |

---

## Quick Command Reference

```bash
# Chain scaffolding
ignite scaffold chain vertix --no-module
ignite scaffold module oracle --dep staking,slashing,params
ignite scaffold module rwa --dep bank,oracle,params
ignite scaffold module fees --dep bank,distribution,params

# Development
ignite chain serve --reset-once
ignite generate proto-go
ignite generate ts-client

# Testing
ignite test --verbose
go test ./x/oracle/... ./x/rwa/... ./x/fees/... -v

# Build
ignite build
make build

# Local devnet
make devnet-reset

# Validation
vertixd validate-genesis
```

---

## References

- Cosmos SDK docs: https://docs.cosmos.network/
- CometBFT docs: https://docs.cometbft.com/
- ibc-go docs: https://ibc.cosmos.network/
- interchaintest: https://github.com/strangelove-ventures/interchaintest
- Hermes relayer: https://hermes.informal.systems/
- cosmos/chain-registry: https://github.com/cosmos/chain-registry
- Awesome Cosmos: https://github.com/cosmos/awesome-cosmos
