# Cosmos SDK + Ignite CLI: Full Blockchain Roadmap

This plan breaks down the end-to-end tasks to build a production-ready Cosmos SDK blockchain using Ignite CLI, CometBFT, IBC, and CosmJS. Each phase includes actionable tasks, commands, and acceptance criteria.

## Phase 0 — Prerequisites
- [ ] Install Go (matching Cosmos SDK requirements)
- [ ] Install Ignite CLI
  ```bash
  curl https://get.ignite.com/cli! | bash
  ignite version
  ```
- [ ] Install buf, protoc, and protoc-gen tools (for protobuf)
- [ ] Install Node.js and pnpm/yarn (for CosmJS tooling)
- [ ] Docker (optional, for local relayers and deployment)

Acceptance criteria:
- **Go**, **Ignite**, **buf**, **protoc**, **Node** available in PATH.

---

## Phase 1 — Initialize Chain (Ignite)
- [ ] Scaffold a new chain
  ```bash
  ignite scaffold chain pns --no-module
  ```
- [ ] Set app name, binary, bech32 prefix, and denom in `config.yml`
- [ ] Configure initial accounts, faucet, and balances in `config.yml`
- [ ] Run the chain with live reload
  ```bash
  ignite chain serve --reset-once
  ```

Acceptance criteria:
- Chain boots, produces blocks, default REST/gRPC endpoints are available.

---

## Phase 2 — Core App Wiring
- [ ] Ensure module manager, begin/end blockers, and invariants are wired in `app/app.go`
- [ ] Configure module execution order (BeginBlocker, EndBlocker, InitGenesis)
- [ ] Enable `upgrade`, `params`, `auth`, `bank`, `staking`, `gov`, `distribution`, `slashing`, `feegrant`, `authz`, `consensus`, `crisis`, `capability`, `ibc`, `transfer` modules
- [ ] Configure `encoding` and `txConfig`

Acceptance criteria:
- App compiles; module order validates at start.

---

## Phase 3 — Base Modules Configuration
- [ ] Auth: set `MaxTxGasWanted`, signature modes
- [ ] Bank: set base denom, send enabled, metadata
- [ ] Staking: set bond denom, unbonding time, validators params
- [ ] Gov (new or legacy, depending on SDK version): set voting & deposit params
- [ ] Distribution: set community tax, withdraw address enabled
- [ ] Slashing: set signed blocks window, min signed, downtime/jail params
- [ ] Upgrade: enable handlers and version map
- [ ] Crisis: set constant fee (denom/amount)
- [ ] Feegrant/Authz: ensure keepers are wired
- [ ] Consensus: configure block params
- [ ] IBC core & transfer: enable ports, `transfer` module, capability keeper

Acceptance criteria:
- Genesis reflects configured params; node starts with no invariant violations.

---

## Phase 4 — Custom Module(s) Scaffolding
- [ ] Scaffold a custom module with dependencies
  ```bash
  ignite scaffold module pns --dep bank,auth,params
  ```
- [ ] Add state types using list/map/single
  ```bash
  ignite scaffold list pns domain name:string owner:string expires:int64
  ignite scaffold map  pns resolver name:string address:string
  ignite scaffold single pns params
  ```
- [ ] Add messages and queries
  ```bash
  ignite scaffold message pns register name:string duration:int64 --desc "register domain"
  ignite scaffold message pns set-resolver name:string address:string
  ignite scaffold query   pns domain name:string
  ignite scaffold query   pns resolver name:string
  ```
- [ ] Implement keeper logic, validation (`ValidateBasic()`), authorization (signer checks)
- [ ] Emit events for state changes
- [ ] Add module parameters with validation (in `types/params.go`)
- [ ] Add genesis default and validations (in `genesis.go`)

Acceptance criteria:
- Unit tests pass for keeper, msg server, and query server; txs and queries work via CLI.

---

## Phase 5 — Protobuf & API
- [ ] Define proto messages/services in `proto/pns/...` with proper packages and versions
- [ ] Add `google.api.http` annotations for gRPC-Gateway REST exposure
- [ ] Follow ADR-044 protobuf update guidelines (no breaking changes)
- [ ] Generate code
  ```bash
  ignite generate proto-go
  ignite generate ts-client
  ```
- [ ] Validate OpenAPI (if generated) and gRPC reflection

Acceptance criteria:
- Generated Go and TS clients compile; REST and gRPC endpoints reflect services.

---

## Phase 6 — CLI and Transactions
- [ ] Ensure auto-generated CLI commands exist for all `rpc` Msg/Query (ADR-058 alignment)
- [ ] Add ergonomic flags for complex types (coins, timestamps, pagination)
- [ ] Provide examples in README for common tx flows
  ```bash
  # examples
  pnsd tx pns register example --duration 31536000 --from alice --fees 2000stake
  pnsd q  pns domain example -o json
  ```

Acceptance criteria:
- All tx/query flows executable through CLI with clear help and examples.

---

## Phase 7 — CosmJS Integration
- [ ] Publish or link generated TS clients from `ts-client`
- [ ] Implement a simple client script for connect, sign, broadcast, and query
  ```ts
  import { SigningStargateClient, DirectSecp256k1HdWallet } from "@cosmjs/stargate";
  // connect, sign, broadcast, query examples against local node
  ```
- [ ] Add high-level helper for your custom module (e.g., `registerDomain(wallet, name, duration)`)
- [ ] Provide E2E script that: creates accounts, funds, sends a tx, queries state, verifies results

Acceptance criteria:
- Node interaction works end-to-end via CosmJS; README documents usage.

---

## Phase 8 — IBC Enablement
- [ ] Enable `ibc` and `transfer` modules in `app/app.go`
- [ ] Configure ICS-20 transfer params, port `transfer`
- [ ] Scaffold IBC packet(s) for custom module if needed
  ```bash
  ignite scaffold packet pns ibctransfer name:string amount:coin --ack success:string
  ```
- [ ] Implement `OnRecvPacket`, `OnAcknowledgementPacket`, `OnTimeoutPacket` logic
- [ ] Spin up a second local chain (another Ignite chain) and create channels
- [ ] Run a relayer (Ignite relayer or Hermes) to relay packets
- [ ] Provide relayer setup recipes for both Hermes and Go Relayer (rly)
- [ ] Local multi-chain dev with `local-interchain` for quick demos

Acceptance criteria:
- ICS-20 transfers succeed between chains; custom IBC packets ack and mutate state as expected.

---

## Phase 9 — Testing Strategy
- [ ] Unit tests: keeper, msg server, query server (table-driven tests)
- [ ] Integration tests: app/module init, tx flow, events
  ```bash
  ignite test --verbose
  ```
- [ ] IBC integration tests with two local chains and relayer
- [ ] Simulation tests (if applicable): state machine invariants, random operations
- [ ] Test coverage threshold (e.g., >70%) and CI setup
- [ ] Load testing with `tm-load-test`; capture baseline TPS/latency
- [ ] End-to-end ABCI swap-out tests using `CometMock`
- [ ] Static analysis with `cosmos-sdk-codeql` in CI

Acceptance criteria:
- CI green on unit/integration; IBC e2e passing; coverage threshold met.

---

## Phase 10 — Genesis & Params
- [ ] Configure `config.yml` accounts, faucet, denominations, and module params
- [ ] Add genesis validation for custom module
- [ ] Provide sample `genesis.json` and quickstart script
- [ ] Document fee settings, min gas prices, and denominations
- [ ] Include `cosmos-genesis-tinkerer` workflows for reproducible genesis edits (audited diffs)

Acceptance criteria:
- Deterministic genesis; `pnsd start` works with configured params.

---

## Phase 11 — Observability & Ops
- [ ] Enable structured logging and log levels
- [ ] Enable Prometheus metrics and scrape configs
- [ ] Expose app/consensus metrics (CometBFT)
- [ ] Configure pprof (optional)
- [ ] Integrate Tenderduty for missed-blocks alerting (validators)
- [ ] Integrate PANIC monitoring and alerting
- [ ] Add auxiliary Prometheus exporters: node-exporter, wallets-exporter, validators-exporter
- [ ] Provide a default Grafana dashboard (CometBFT + SDK + OS metrics)

Acceptance criteria:
- Metrics visible; dashboards show node health and performance; alerts fire on missed blocks and low balances

---

## Phase 12 — Upgrades & Migrations
- [ ] Wire the `upgrade` module with handlers per version name
- [ ] Implement `Migrations()` for store/key changes
- [ ] Write upgrade handler tests
- [ ] Document on-chain governance-based upgrade procedure

Acceptance criteria:
- Simulated upgrade executes; state migrated correctly; node continues producing blocks.

---

## Phase 13 — Security & Invariants
- [ ] Validate all inputs; strict `ValidateBasic()`
- [ ] Signer/authorization checks in all messages
- [ ] Emit events for state changes
- [ ] Add invariants (where applicable) and integrate with crisis module
- [ ] Gas metering for loops/iterations; avoid unbounded operations
- [ ] Proper key prefixes and capability scoping for IBC

Acceptance criteria:
- No invariant violations; fuzz and adversarial tests pass.

---

## Phase 14 — Developer Experience
- [ ] Makefile targets for build, test, lint, proto-gen, ts-gen
- [ ] `ignite chain serve --reset-once` for fast iteration
- [ ] Pre-commit hooks for formatting (gofmt, golangci-lint) and proto-breaking checks (buf)
- [ ] Local dev scripts: seed accounts, send sample txs, query helpers

Acceptance criteria:
- One-command dev loop; contributors can run, test, and develop easily.

---

## Phase 15 — Packaging & Deployment
- [ ] Dockerfile for node binary
- [ ] Configuration for seeds/peers, pruning, snapshots
- [ ] Persistent peers and address book setup
- [ ] Release pipeline to publish binaries and Docker images

Acceptance criteria:
- Reproducible builds; node deployable on remote servers with documented steps.

---

## Phase 16 — Documentation
- [ ] Update README: overview, prerequisites, quickstart, commands
- [ ] Module docs: state, messages, queries, events, params
- [ ] API docs: gRPC/REST endpoints and CosmJS examples
- [ ] IBC guide: channels, relayer setup, demo flows
- [ ] Operations guide: upgrades, backups, monitoring

Acceptance criteria:
- Developers and validators can follow docs to run, build, and integrate with the chain.

---

## Phase 17 — Tokenomics & Unlock Schedule
- [ ] Define total supply (base units) and base/display denoms (`upns`/`pns`)
- [ ] Allocate supply across categories (team, investors, foundation, community, ecosystem, liquidity, airdrop)
- [ ] Choose vesting types per category/beneficiary:
  - ContinuousVesting (linear between start/end)
  - DelayedVesting (cliff, all at end)
  - PeriodicVesting (custom periods and amounts)
- [ ] Produce beneficiary list with addresses and amounts
- [ ] Specify unlock schedule (cliff, cadence, periods) per category
- [ ] Implement tokenomics in genesis via one of:
  - `config.yml` genesis overrides with vesting accounts and bank balances
  - or CLI: `pnsd genesis add-genesis-account` with `--vesting-amount`, `--vesting-start-time`, `--vesting-end-time`
  - or manual patch of `genesis.json` adding `cosmos.vesting.v1beta1.*VestingAccount`
- [ ] Document tokenomics and schedules in `docs/tokenomics.md`

Acceptance criteria:
- Token allocation table equals total supply
- Vesting accounts present in genesis; bank balances match allocations
- Schedules verified via queries (staking params unaffected; balances and vesting fields correct)

References:
- Cosmos SDK Vesting Accounts: `https://docs.cosmos.network/v0.53/user/run-node/run-node` (genesis accounts) and module docs
- Ignite Config Genesis Overrides: `https://docs.ignite.com/guide/config`

## Quick Command Reference (Ignite)
```bash
ignite scaffold chain pns --no-module
ignite scaffold module pns --dep bank,auth,params
ignite scaffold list pns domain name:string owner:string expires:int64
ignite scaffold map  pns resolver name:string address:string
ignite scaffold single pns params
ignite scaffold message pns register name:string duration:int64
ignite scaffold query   pns domain name:string
ignite scaffold packet  pns ibctransfer name:string amount:coin --ack success:string
ignite generate proto-go
ignite generate ts-client
ignite test --verbose
ignite build
ignite chain serve --reset-once
```

Notes:
- Refer to Ignite CLI references for more scaffolding options (list, map, single, type, packet, message, query, react, vue).
- Follow Cosmos SDK ADR-044 for protobuf updates and ADR-058 for auto-generated CLI expectations.

---

## Phase 18 — Block Explorer
- [ ] Expose node endpoints for explorers (public or localhost):
  - RPC (26657), REST (1317), gRPC (9090), gRPC-Web (9091, optional)
  - Enable CORS for REST if required by explorer
- [ ] Choose explorer stack:
  - Ping.pub (UI-only, simple config; uses LCD/RPC)
  - Big Dipper + BDJuno + Hasura (full indexer + UI)
  - Terminal explorers (gex, cshtop, pvtop) for dev/ops
- [ ] Configure chain metadata (bech32 prefix, denom, decimals, logo) for the explorer
- [ ] Stand up the explorer locally via Docker or source build
- [ ] Verify blocks, transactions, accounts, validators, and (optional) IBC pages

Implementation options:
- Ping.pub: Prepare a chain config JSON and run the Docker image with mounted config
- Big Dipper: Deploy BDJuno indexer (PostgreSQL), Hasura GraphQL, and Big Dipper UI; set env to point at chain RPC/LCD
- Optional lightweight indexers: Cosmscan or interchain-indexer (Python)

Acceptance criteria:
- Explorer shows live blocks and transactions for the local node
- Account and transaction detail pages render correctly
- Validator list (if staking enabled) displays with correct voting power
- Optional: ICS-20 transfers visible if `transfer` module enabled
- Optional: At least one indexer pipeline (BDJuno or lightweight) runs locally and serves the explorer

References:
- See `docs/explorer.md` for detailed steps
- Awesome list explorers and indexers in `docs/cosmos-sdk-awesome.md` (Block Explorers, Indexers)

---

## Phase 19 — Wallet Integrations
- [ ] Wallet support: Keplr, Leap, Cosmostation
- [ ] Chain info for wallets (suggestChain JSON or chain-registry entries)
- [ ] Verify offline signer via CosmJS; ADR-027/ADR-036 compatibility if applicable
- [ ] UI testing with basic web dApp (connect wallet, show balances, send tx)
- [ ] Ensure denom metadata and decimals display correctly in wallets

Acceptance criteria:
- Wallets can connect to the node, derive addresses, and sign/broadcast txs
- Denom `PNS` shown with correct decimals (6); balances reflected accurately

References:
- CosmJS: `https://github.com/cosmos/cosmjs`
- Chain Registry format: `https://github.com/cosmos/chain-registry`

---

## Phase 20 — Testnets
- [ ] Spin up devnet: single node with faucet and explorer
- [ ] Public testnet: 2+ validators, seeds, persistent peers, explorer, faucet
- [ ] Publish endpoints, faucet instructions, and quickstart docs
- [ ] Capture known issues and upgrade a testnet as a rehearsal
- [ ] Deploy a public faucet service (Cosmfaucet) with rate limiting
- [ ] Choose explorer stack (Ping.pub vs Big Dipper + BDJuno); document trade-offs

Acceptance criteria:
- Public testnet stable for ≥1 week; users can request funds and transact
- Explorer shows blocks/txs; faucet operational; documentation clear

---

## Phase 21 — Genesis Ceremony & Validator Coordination
- [ ] Decide min commission, params, and governance settings
- [ ] Distribute instructions: `pnsd init`, `gentx`, `collect-gentxs`, peer lists
- [ ] Aggregate gentxs with consistent chain-id and genesis time
- [ ] Publish genesis.json and SHA256; finalize minimum gas prices
- [ ] Dry-run with a coordinated pre-launch validation
- [ ] Distribute `persistent_peers`/`seeds` lists and addrbook; peer discovery checklist

Acceptance criteria:
- All validators can verify genesis hash, start at same time, and produce blocks
- Peer connectivity functional; no double-sign risk from instructions
- Peer mesh forms quickly at launch; no network partitions

---

## Phase 22 — Chain Registry & Metadata
- [ ] Prepare chain.json and assetlist for `cosmos/chain-registry`
- [ ] Include RPC/LCD endpoints, explorers, peers/seeds, logo, fees
- [ ] Submit PR and update as endpoints change
- [ ] Prepare wallet suggestChain configs (Keplr, Leap, Cosmostation) where applicable

Acceptance criteria:
- PR merged; wallets/explorers ingest registry data successfully
- Wallets connect via registry data; denom metadata/decimals display correctly

---

## Phase 23 — Launch Infra & SRE
- [ ] Sentry architecture: validators behind sentry nodes; seeds and peers
- [ ] State sync and snapshots; publish snapshot service cadence
- [ ] Cosmovisor setup and binary distribution strategy
- [ ] P2P hardening: addrbook, peer-exchange, seeds, persistent peers
- [ ] Backup and restore playbooks
- [ ] Document Cosmovisor service layout and upgrade procedure
- [ ] Publish snapshot cadence and validate restoration regularly

Acceptance criteria:
- Validators join via documented peer discovery; snapshots reduce sync time
- Cosmovisor seamlessly manages upgrades in staging

---

## Phase 24 — Mainnet Launch Checklist
- [ ] Finalize chain-id and version tag; release binaries and Docker images
- [ ] Distribute final genesis, SHA256, peers, and start time
- [ ] Perform prelaunch verification: `validate-genesis`, dry sync, load test
- [ ] Prepare incident response and emergency comms channels

Acceptance criteria:
- All validators confirm readiness; network starts producing blocks as scheduled
- No critical incidents in first 24–48 hours

---

## Phase 25 — Post-Launch Operations
- [ ] Monitoring: Prometheus, Grafana dashboards, alerting thresholds
- [ ] On-call rotation and incident runbooks
- [ ] Patch/hotfix process; security advisories
- [ ] Snapshot cadence; pruning strategy; backups

Acceptance criteria:
- Alerts actionable; MTTR within targets; regular snapshots validated

---

## Phase 26 — Ecosystem Integrations
- [ ] IBC channels to major hubs (e.g., Cosmos Hub, Osmosis)
- [ ] Production relayers (Hermes/Go relayer) with monitoring
- [ ] DEX listings coordination; price feeds/oracles as needed
- [ ] Indexers: BDJuno, Hasura, TheGraph-like adapters if needed

Acceptance criteria:
- Stable IBC packet flow and ICS-20 transfers; relayer uptime acceptable
- Token listed on target platforms (where applicable)

---

## Phase 27 — Compliance & Legal (Optional)
- [ ] Publish disclaimers and terms for token distribution
- [ ] Airdrop criteria and regional restrictions (if any)
- [ ] Document tax considerations and reporting guidance (jurisdiction-dependent)
- [ ] Privacy policy and data handling for services (explorer, faucet)

Acceptance criteria:
- Public docs cover distribution terms and regulatory notices as required
