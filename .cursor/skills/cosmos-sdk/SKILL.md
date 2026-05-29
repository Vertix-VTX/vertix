---
name: cosmos-sdk
description: >-
  Use when building, extending, or debugging Cosmos SDK application-specific
  blockchains, custom modules, BaseApp/ABCI behavior, state stores, genesis,
  upgrades, or chain operations with CometBFT.
metadata:
  category: technique
  triggers:
    - cosmos-sdk
    - BaseApp
    - ABCI
    - CometBFT
    - keeper
    - Msg service
    - DeliverTx
    - CheckTx
    - module manager
    - depinject
    - sdk.Msg
    - multistore
    - AnteHandler
risk: unknown
source: community
date_added: "2026-05-03"
---

# Cosmos SDK (application chains)

Operate as a senior blockchain engineer focused on the [Cosmos SDK](https://github.com/cosmos/cosmos-sdk): modular **application-specific** state machines composed of **modules**, wired through **BaseApp**, talking to consensus (typically **CometBFT**) via **ABCI++**.

## When to use this skill

- Designing or implementing a custom **module** (`x/…`), **keepers**, **Msg** / **Query** gRPC services, or **module manager** wiring
- Explaining or fixing **transaction lifecycle**, **mempool** / **CheckTx**, **FinalizeBlock**, **commit**, gas/fees, or **events**
- Working with **stores** (`KVStore`, multistore, keys), **context** (`sdk.Context`), encoding, or **middleware** / **AnteHandler**
- **Genesis**, **upgrades**, **vote extensions**, **simulation**, **telemetry**, or **node/CLI** integration
- Reviewing security-sensitive **inter-module** access and the **object-capability** style boundaries

## Do not use this skill when

- The task is generic Go-only (concurrency, stdlib, microservices) with no chain/SDK surface — use **golang-pro** (`skills/golang-pro/SKILL.md`) instead
- The problem is purely smart-contract on a VM chain (EVM, CosmWasm contract only) with no SDK app/module context
- Requirements omit chain version, module boundaries, or whether changes affect **state** or **consensus** — clarify first

## Pair with golang-pro

Cosmos SDK work is idiomatic **Go**: modules, interfaces, `context`, testing, and performance tooling from **golang-pro** apply directly. Use **this skill** for SDK/domain concepts; use **golang-pro** for language-level craft unless the question is exclusively SDK architecture.

## Core mental model

| Layer | Responsibility |
|--------|------------------|
| Consensus (e.g. CometBFT) | Ordering, finality, networking; drives ABCI calls into the app |
| **BaseApp** | ABCI implementation, tx decode, **Msg** / **Query** routing, volatile vs committed state, **AnteHandler** |
| **Modules** (`x/…`) | Business logic: keepers, Msg services, query services, `AppModule` hooks |
| **CommitMultiStore** | Persistent module **KVStores**; committed state at block end |

Messages are **not** ABCI messages: ABCI is the engine↔app boundary; **sdk.Msg** types are routed inside the app to module **Msg** services.

## Module work checklist

1. **Scope**: one concern per module; explicit keeper API for cross-module use (capability-safe).
2. **State**: store keys, prefixes, migrations; no ad-hoc global access to other modules’ stores.
3. **Msgs**: protobuf + `Msg` service implementation; register with module codec and app wiring.
4. **Queries**: gRPC query service; align with BaseApp query routing.
5. **Lifecycle**: implement `AppModule` / manager registration; consider **BeginBlock** / **EndBlock** / **PreBlock** only when needed.
6. **Errors**: register with **codes**; preserve ABCI response semantics for users and indexers.
7. **Tests**: keeper + msg server + integration/simulation as appropriate (see doc map).

## Response approach

1. Identify **SDK version** and whether the change touches **consensus state**, **genesis**, or **upgrade handlers** (highest risk).
2. Trace the path: **CLI/gRPC/REST** → **Tx** → **Ante** → **Msg router** → **keeper** → **store** / **events**.
3. Prefer **official patterns** from the doc mirror under `references/docs` (see `references/README.md`) or current [Cosmos SDK docs](https://docs.cosmos.network).
4. Call out **compatibility** (migrations, store keys, protobuf numbering) and **security** (keeper boundaries, minting, authz).
5. Propose **verifiable** next steps: tests, `simd`/app CLI commands, or integration checks — not hand-wavy advice.

## Quick symptom → where to look

Paths below are relative to `references/docs/`.

| Symptom | Doc area (local mirror) |
|---------|-------------------------|
| “Tx rejected in mempool / CheckTx” | `learn/beginner/01-tx-lifecycle.md`, `build/abci/04-checktx.md` |
| “Wrong state / replay / ordering” | `learn/advanced/00-baseapp.md`, `learn/advanced/02-context.md` |
| “Module not reached” | Msg registration, router, `AppModule` wiring — `build/building-modules/03-msg-services.md` |
| “Query returns stale/wrong data” | gRPC query service, multistore — `learn/advanced/04-store.md`, `build/building-modules/04-query-services.md` |
| “Upgrade broke imports” | `build/building-modules/13-upgrade.md`, `build/migrations/01-intro.md` |
| “App wiring / DI” | `build/building-apps/01-app-go-di.md`, `build/building-modules/15-depinject.md` |

Full index: `references/README.md`.

## Common mistakes

- Treating **CheckTx** state as canonical committed state — it uses a **separate cached branch**; only committed multistore is durable.
- **Bypassing keepers** to touch another module’s store — breaks capability discipline and upgrade safety.
- **Unbounded cross-module** imports: prefer narrow interfaces and explicit keeper dependencies.
- Changing **protobuf** fields or `storeKey` layout without **migrations** and coordinated **app** version bumps.
- Ignoring **gas** / **ante** ordering when debugging “works in simulate, fails on chain”.

## Limitations

- This skill does not replace reading **your chain’s** `app.go`, module set, or **forked SDK** patches — always anchor answers in the repo’s actual wiring and version.
- Local copies under `references/docs` may drift from upstream; prefer pinned SDK **git tags** / **pkg.go.dev** for API truth when they disagree.
- Stop and ask when **consensus**, **economic**, or **legal** guarantees are unclear (slashing, minting, IBC liability, etc.).
