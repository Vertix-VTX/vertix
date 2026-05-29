# Vertix — Project Structure

This document describes the **target** repository layout once all phases of the [roadmap](./roadmap.md) are implemented. The repo is bootstrapped with `ignite scaffold chain vertix --no-module`; everything below either comes from that scaffold or is added by one of the implementation plans in [`docs/plans/`](./plans/).

> Status legend: ✅ scaffold-generated · 🛠️ created by plans · 📦 generated artifact

---

## 1. Top-Level Layout

```
vertix/
├── app/                      ✅  Cosmos SDK app construction (app.go)
├── cmd/                      ✅  Binary entry points
│   └── vertixd/                  Chain node binary
├── x/                        🛠️  Custom Cosmos SDK modules
│   ├── oracle/
│   ├── rwa/
│   └── fees/
├── feeder/                   🛠️  vertix-feeder sidecar (Go binary)
│   └── cmd/vertix-feeder/        Feeder entry point
├── proto/                    🛠️  Protobuf source of truth
│   └── vertix/
│       ├── oracle/v1/
│       ├── rwa/v1/
│       └── fees/v1/
├── testutil/                 🛠️  Shared test helpers (keeper fixtures, mocks)
├── e2e/                      🛠️  interchaintest IBC end-to-end suite (own go.mod)
├── infra/                    🛠️  Local devnet, relayer, monitoring, explorer config
│   ├── devnet/
│   ├── hermes/
│   ├── explorer/
│   └── monitoring/
├── docs/                     🛠️  Project documentation (this folder)
│   ├── full-design-spec.md   Program source of truth (phases 0–10)
│   ├── specs/                Per-phase design specs (from brainstorms)
│   ├── plans/
│   ├── project-overview.md
│   ├── architecture.md
│   ├── technical-design.md
│   ├── project-structure.md
│   ├── coding-standards.md
│   ├── tokenomics.md
│   └── roadmap.md
├── scripts/                  🛠️  Devnet reset, genesis tooling, helpers
├── .github/                  🛠️  CI/CD
│   └── workflows/
├── build/                    📦  Compiled binaries (gitignored)
├── Dockerfile                🛠️  static vertixd image for interchaintest e2e
├── config.yml                ✅  Ignite chain config (devnet identity)
├── go.mod / go.sum           ✅  Go modules
├── Makefile                  🛠️  Dev targets (build, test, lint, devnet)
├── .golangci.yml             🛠️  Linter config
└── AGENTS.md                 🛠️  Agent entry point — pointers to docs
```

---

## 2. `app/` — Application Construction

```
app/
├── app.go             ✅ → 🛠️  Wires all SDK + custom modules; x/mint REMOVED
├── app_test.go        🛠️         Asserts: no x/mint, required modules wired,
│                                  custom modules present, genesis exports cleanly
├── encoding.go        ✅          Codec setup
├── export.go          ✅          Genesis export helper
└── params/
    └── encoding.go    ✅          Encoding params
```

**Conventions:**
- All keeper construction lives in `NewApp(...)`.
- Module manager order: standard SDK modules first, then `oracle`, `rwa`, `fees`.
- Begin/EndBlock order documented in [`technical-design.md`](./technical-design.md) §7.
- No business logic in `app/`; only wiring.

---

## 3. `cmd/` — Binaries

```
cmd/
└── vertixd/
    ├── main.go        ✅  Entry point
    └── cmd/
        └── root.go    ✅  Cobra root, subcommand wiring
```

`vertixd` is the only chain binary. The feeder lives at `feeder/cmd/vertix-feeder/`.

---

## 4. `x/` — Custom Modules

Every custom module follows the same skeleton. The pattern is illustrated for `x/oracle`; `x/rwa` and `x/fees` mirror it.

```
x/oracle/
├── keeper/
│   ├── keeper.go         Constructor, struct, deps
│   ├── feed.go           SetFeed / GetFeed / GetAllFeedsForPair
│   ├── aggregator.go     WeightedMedian, AggregateAllPairs, GetPrice
│   ├── twap.go           AppendTWAPEntry, GetTWAP
│   ├── slash.go          MissCounter + ProcessMissesAndSlash
│   ├── params.go         GetParams / SetParams
│   ├── msg_server.go     MsgServer impl (SubmitFeed, UpdateParams)
│   ├── grpc_query.go     QueryServer impl
│   └── *_test.go         Unit tests with testutil/keeper
├── types/
│   ├── types.go          Generated proto Go types (re-exports + helpers)
│   ├── keys.go           StoreKey, prefix bytes, key builders
│   ├── errors.go         Sentinel errors (cosmossdk.io/errors)
│   ├── events.go         Event type and attribute constants
│   ├── params.go         Defaults + Validate()
│   ├── msgs.go           ValidateBasic for each Msg
│   └── *_test.go         Pure type tests
├── client/cli/
│   ├── tx.go             CLI: submit-feed, update-params (gov)
│   └── query.go          CLI: price, twap, params, miss-counter
├── module.go             AppModule, RegisterServices, EndBlock
├── genesis.go            InitGenesis, ExportGenesis, DefaultGenesis
└── abci.go               EndBlocker function (called by AppModule.EndBlock)
```

### 4.1 `x/rwa` Module

Same skeleton, plus:

```
x/rwa/keeper/
├── asset.go              CRUD on AssetRecord, state machine transitions
├── bond.go               Issuer bond escrow lock/release/slash
├── mint.go               rwa/{asset-id} factory denom mint/burn + fee
└── transfer.go           Transfer restriction checks + ante decorator
```

### 4.2 `x/fees` Module

Smallest of the three — no custom state besides `FeesParams`.

```
x/fees/keeper/
├── keeper.go             Constructor with bank/distribution/auth deps
├── split.go              EndBlock fee split (burn + distribute)
└── params.go
```

---

## 5. `feeder/` — Oracle Sidecar Binary

Standalone Go binary; lives in the same Go module as the chain to share types and codec.

```
feeder/
├── cmd/
│   └── vertix-feeder/
│       └── main.go              Cobra root, config flags, lifecycle
├── feeder/
│   ├── config.go                YAML Config struct, validation, loader
│   ├── feeder.go                Main loop (tick → fetch → median → broadcast)
│   ├── broadcaster.go           Tx signing + gRPC broadcast
│   ├── metrics.go               Prometheus counters/gauges/histograms
│   ├── provider/
│   │   ├── provider.go          PriceProvider interface
│   │   ├── coingecko.go         CoinGecko REST adapter
│   │   ├── binance.go           Binance spot adapter
│   │   └── provider_test.go     Mock-based unit tests
│   └── feeder_test.go           Integration: fetch + median + mock broadcast
└── feeder-config.example.yaml   Documented example configuration
```

The feeder is **operationally required** — every active validator must run an instance.

---

## 6. `proto/` — Protobuf Definitions

Single source of truth for messages, queries, and events. Code generation runs via `ignite generate proto-go`.

```
proto/
└── vertix/
    ├── oracle/v1/
    │   ├── types.proto       OracleFeed, AggregatedPrice, TWAPEntry, OracleParams
    │   ├── tx.proto          MsgSubmitFeed, MsgUpdateParams
    │   ├── query.proto       Query gRPC service
    │   └── genesis.proto     GenesisState
    ├── rwa/v1/
    │   ├── types.proto       AssetRecord, AssetStatus, TransferRestriction, RWAParams
    │   ├── tx.proto          Msg{Register,Attest,Mint,Transfer,Settle,...}
    │   ├── query.proto
    │   └── genesis.proto
    └── fees/v1/
        ├── types.proto       FeesParams { burn_ratio, distribution_ratio }
        ├── tx.proto
        ├── query.proto
        └── genesis.proto
```

**Rules:**
- One package per module (`vertix.<module>.v1`).
- All `Msg` services use `cosmos.msg.v1.signer` annotation.
- All decimal fields use `cosmossdk.io/math.LegacyDec` custom type.
- Address fields use `(cosmos_proto.scalar) = "cosmos.AddressString"`.

---

## 7. `testutil/` — Shared Test Helpers

```
testutil/
└── keeper/
    ├── oracle.go     Returns (sdk.Context, oracle.Keeper) backed by in-mem KV store
    ├── rwa.go        Same for x/rwa, with mock OracleKeeper
    └── fees.go       Same for x/fees, with mock bank/distr keepers
```

All keeper fixtures use `cosmossdk.io/store` `NewCommitMultiStore` over `dbm.NewMemDB()` — no chain bring-up needed. See [`coding-standards.md`](./coding-standards.md) §6 for the canonical pattern.

---

## 8. `e2e/` — End-to-End Tests

Independent Go module to keep `interchaintest`'s heavy dependency tree out of the main module.

```
e2e/
├── go.mod
├── go.sum
├── ibc_transfer_test.go     ICS-20 VTX transfer between two Vertix chains (default CI)
├── gaia_transfer_test.go    ICS-20 VTX transfer Vertix<->gaia (build tag `realnet`, opt-in)
├── rwa_transfer_test.go     rwa/* portability test (build tag `rwa`, enabled after Phase 3)
└── helpers/
    └── chain.go             Vertix ChainSpec, image tag, genesis overrides
```

Run with `cd e2e && go test ./... -timeout 30m -v`.

---

## 9. `infra/` — Operational Configuration

```
infra/
├── devnet/
│   ├── docker-compose.yml          3-validator devnet stack
│   ├── validator1/config/
│   ├── validator2/config/
│   └── validator3/config/
├── hermes/
│   └── config.toml                 Hermes relayer config
├── explorer/
│   └── chains/
│       └── vertix.json             Ping.pub chain definition
└── monitoring/
    ├── prometheus.yml              Scrape config for vertixd + feeder
    └── grafana/
        └── dashboards/
            └── vertix.json         CometBFT + oracle + fees + RWA panels
```

Devnet workflow:

```bash
make devnet-reset       # wipe and start a fresh 3-validator devnet
make devnet             # bring up devnet keeping state
```

---

## 10. `docs/` — Documentation

```
docs/
├── full-design-spec.md          Approved program spec (phases 0–10)
├── specs/                       Per-phase design specs (from brainstorms)
├── plans/
│   ├── 2026-05-09-01-chain-foundation.md
│   ├── 2026-05-09-02-oracle-module.md
│   ├── 2026-05-09-03-fees-module.md
│   ├── 2026-05-09-04-rwa-module.md
│   ├── 2026-05-09-05-oracle-feeder-sidecar.md
│   └── 2026-05-09-06-ibc-devnet.md
├── project-overview.md
├── architecture.md
├── technical-design.md
├── project-structure.md         (this file)
├── coding-standards.md
├── roadmap.md
├── tokenomics.md
├── devnet.md                    (created by Plan 06)
├── relayer.md                   (created by Plan 06)
├── tmkms.md                     (created by Phase 6 — security hardening)
├── validator-setup.md           (created by Phase 6)
├── validator-onboarding.md      (created by Phase 7)
└── rwa-quickstart.md            (created by Phase 7)
```

---

## 11. CI/CD & Tooling

```
.github/
└── workflows/
    └── ci.yml             Lint + build + test + validate-genesis on every PR

.golangci.yml              Lint configuration (Go 1.22)
Makefile                   Canonical dev targets (see below)
config.yml                 Ignite chain identity + devnet genesis overrides
```

**Makefile targets:**

| Target | Purpose |
|---|---|
| `make build` | Compile `vertixd` binary into `build/` |
| `make install` | Install `vertixd` into `$GOPATH/bin` |
| `make test` | Run all unit + integration tests with `-race` |
| `make test-cover` | Run tests with coverage HTML output |
| `make lint` | Run `golangci-lint` |
| `make proto-gen` | Generate Go from protobuf |
| `make ts-gen` | Generate TypeScript client |
| `make devnet-reset` | Start fresh devnet (Ignite serve --reset-once) |
| `make devnet` | Start devnet keeping state |
| `make validate-genesis` | Run `vertixd genesis validate-genesis` |
| `make clean` | Remove build artifacts |

---

## 12. Generated & Ignored Paths

The following are produced by tooling and are **not committed**:

```
build/             — compiled binaries (output of `make build`)
coverage.out       — Go test coverage data
coverage.html      — HTML coverage report
~/.vertix/         — local node home (devnet/testnet state)
node_modules/      — only if TS client generation is used locally
.idea/, .vscode/   — IDE state (per-developer)
```

`.gitignore` is configured by the Ignite scaffold and extended in Plan 01.

---

## 13. Why This Layout

- **Custom modules co-located in `x/`** — matches Cosmos SDK convention; auditors and SDK-fluent contributors find code where they expect.
- **Single Go module for chain + feeder** — feeder reuses message types and codec without import cycles or duplicate proto generation.
- **Separate `e2e/` go.mod** — keeps `interchaintest`'s Docker / interchain dependency tree out of the chain build.
- **`infra/` separate from `docs/`** — operational config is versioned and reproducible; documentation references it but does not embed it.
- **`docs/specs` + `docs/plans`** — separates design intent (spec) from execution units (plans). Each plan is independently runnable by an agent.
