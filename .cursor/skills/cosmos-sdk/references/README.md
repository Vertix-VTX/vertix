---
description: >-
  Index of Cosmos SDK topics mapped to the local doc mirror under
  docs for deep dives while using
  the cosmos-sdk skill.
metadata:
  tags: [cosmos-sdk, documentation, navigation]
  source: internal
---

# Cosmos SDK doc map (local mirror)

**Base path:** `docs/`

Tables below list paths **under that directory** (same layout as upstream docs).

## Learn (concepts)

| Topic | Path |
|--------|------|
| Overview / why app-specific | `learn/intro/00-overview.md`, `learn/intro/01-why-app-specific.md` |
| App architecture | `learn/intro/02-sdk-app-architecture.md`, `learn/beginner/00-app-anatomy.md` |
| Tx / query lifecycle | `learn/beginner/01-tx-lifecycle.md`, `learn/beginner/02-query-lifecycle.md` |
| Accounts, gas, fees | `learn/beginner/03-accounts.md`, `learn/beginner/04-gas-fees.md` |
| BaseApp | `learn/advanced/00-baseapp.md` |
| Transactions, context, node, store | `learn/advanced/01-transactions.md`, `learn/advanced/02-context.md`, `learn/advanced/03-node.md`, `learn/advanced/04-store.md` |
| Encoding, gRPC/REST, CLI, events | `learn/advanced/05-encoding.md`, `learn/advanced/06-grpc_rest.md`, `learn/advanced/07-cli.md`, `learn/advanced/08-events.md` |
| Telemetry, ocap, middleware, simulation, upgrades, config | `learn/advanced/09-telemetry.md` through `learn/advanced/16-config.md` |

## Build (implementing)

| Topic | Path |
|--------|------|
| ABCI++ (prepare/process proposal, CheckTx, vote extensions) | `build/abci/*.md` |
| App runtime, mempool, upgrades, testnet, vote extensions | `build/building-apps/*.md` |
| Module intro, manager, msgs/queries, services, keeper, hooks, genesis, errors, testing, depinject | `build/building-modules/*.md` |
| Protobuf tooling | `build/tooling/00-protobuf.md` |
| Migrations | `build/migrations/01-intro.md` |

## User / operations

| Topic | Path |
|--------|------|
| Keyring, run node, interact, txs, testnet, production | `user/run-node/*.md` |

## Official source

When the mirror is missing or outdated, use [Cosmos SDK documentation](https://docs.cosmos.network) and the SDK version pinned in the target chain’s `go.mod`.
