---
description: >-
  Index of CometBFT topics mapped to the local documentation mirror under
  docs for deep dives while using the cometbft skill.
metadata:
  tags: [cometbft, documentation, navigation]
  source: internal
---

# CometBFT doc map (local mirror)

**Base path:** `docs/` (under this directory: `skills/cometbft/references/docs/`)

Tables list paths **under `docs/`** (upstream-style layout from [CometBFT docs](https://docs.cometbft.com)).

## Introduction and onboarding

| Topic | Path |
|--------|------|
| What is CometBFT / BFT / ABCI | `introduction/README.md` |
| Root overview and links | `README.md` |
| Quick start | `guides/quick-start.md` |
| Install, Go guides | `guides/install.md`, `guides/go.md`, `guides/go-built-in.md` |

## Application development (ABCI side)

| Topic | Path |
|--------|------|
| Getting started, `abci-cli` | `app-dev/getting-started.md`, `app-dev/abci-cli.md` |
| App architecture | `app-dev/app-architecture.md` |
| Transaction indexing | `app-dev/indexing-transactions.md` |

## Core node (operations and internals)

| Topic | Path |
|--------|------|
| Section index | `core/README.md` |
| Using CometBFT, configuration | `core/using-cometbft.md`, `core/configuration.md` |
| Production, metrics, logs | `core/running-in-production.md`, `core/metrics.md`, `core/how-to-read-logs.md` |
| Validators, events, blocks | `core/validators.md`, `core/subscription.md`, `core/block-structure.md` |
| RPC | `core/rpc.md` |
| Block sync, state sync, light client, mempool | `core/block-sync.md`, `core/state-sync.md`, `core/light-client.md`, `core/mempool.md` |

## Networks and tools

| Topic | Path |
|--------|------|
| Networks (e.g. Docker, compose) | `networks/README.md`, `networks/docker-compose.md` |
| Tools / benchmarking index | `tools/README.md` |

## Tutorials

| Topic | Path |
|--------|------|
| Example forum app (partial) | `tutorials/forum-application/3.send-message.md` |

## Deep reference (ADRs, RFCs, config specs, QA)

| Topic | Path |
|--------|------|
| Architecture ADRs | `references/architecture/README.md`, `references/architecture/tendermint-core/` |
| RFCs | `references/rfc/` |
| Config file semantics | `references/config/README.md` (`config.toml.md`, `genesis.json.md`, key files) |
| Storage / QA reports | `references/storage/`, `references/qa/` |

## Pairing with other skills

| Need | Skill |
|------|--------|
| Cosmos SDK modules, BaseApp, stores | `skills/cosmos-sdk/SKILL.md` |
| Ignite scaffold / `ignite chain` | `skills/ignite-cli/SKILL.md` |

## Official source

When the mirror is missing or outdated, use [CometBFT documentation](https://docs.cometbft.com) and the CometBFT version pinned in your project’s `go.mod` or release binary.
