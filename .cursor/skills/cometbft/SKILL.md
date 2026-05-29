---
name: cometbft
description: >-
  Use when operating, configuring, or debugging CometBFT consensus and
  networking, ABCI/ABCI++ interaction with the application, mempool,
  validators, RPC, state sync, light clients, block sync, or node
  deployment—not application-specific Cosmos SDK module logic.
metadata:
  category: technique
  triggers:
    - cometbft
    - CometBFT
    - ABCI
    - ABCI++
    - mempool
    - consensus
    - FinalizeBlock
    - PrepareProposal
    - ProcessProposal
    - CheckTx
    - state sync
    - light client
    - priv_validator
    - config.toml
    - genesis.json
risk: unknown
source: community
date_added: "2026-05-03"
---

# CometBFT (consensus engine)

Operate as a senior blockchain engineer focused on [CometBFT](https://github.com/cometbft/cometbft): **BFT state machine replication**, **P2P**, **mempool**, **consensus rounds**, and the **ABCI/ABCI++** boundary to the application. CometBFT orders transactions and drives the app; it does not implement chain business rules (that lives in the app, e.g. Cosmos SDK).

## When to use this skill

- Explaining or fixing **consensus**, **round/step timeouts**, **proposer selection**, or **evidence** behavior at the engine layer
- **ABCI/ABCI++** call ordering, **PrepareProposal** / **ProcessProposal**, **ExtendVote** / **VerifyVoteExtension**, or mismatch between engine expectations and app responses
- **Mempool** behavior, **CheckTx** propagation, **tx indexing**, or **subscription** / event streaming from the node
- **Node operations**: **config.toml**, **genesis.json**, **priv_validator** key/state, **RPC**, **metrics**, logs, **running in production**
- **Sync paths**: **block sync**, **state sync**, **light client** verification, or **catch-up** after downtime
- **Networks**: **sentry** / **validator** topology, **Docker**-based deployments, or **benchmarking** / QA-style performance notes in the doc mirror

## Do not use this skill when

- The task is **Cosmos SDK** modules, **keepers**, **BaseApp** routing, **AnteHandler**, or **store** design — use **cosmos-sdk** skill instead
- The problem is **Ignite CLI** scaffolding or **`config.yml`** without touching consensus or CometBFT config — use **ignite-cli** skill if present
- You lack **CometBFT major version**, **whether ABCI++ is enabled**, or **which binary** (`cometbft`, embedded in app, etc.) — clarify first

## Pair with cosmos-sdk

Cosmos SDK chains **embed** CometBFT and implement **ABCI** in **BaseApp**. Use **this skill** for engine-side semantics (blocks, votes, mempool, RPC, sync). Use **cosmos-sdk** for **Msg** routing, **keepers**, **multistore**, and **app** upgrades. Trace failures **across** the boundary: engine logs and ABCI method sequence vs app return codes and events.

## Core mental model

| Layer | Responsibility |
|--------|----------------|
| **Consensus + reactors** | Agree on block order; handle block/store/mempool/pex/evidence reactors |
| **Mempool** | Admission and ordering hints for txs; **CheckTx** validation path |
| **ABCI/ABCI++** | Calls into the app: proposals, votes, deliver/commit, queries |
| **Application** (e.g. SDK) | Validates and executes txs; returns validator updates, app hash, events |

Blocks and commits are **consensus truth**; the app’s job is to be **deterministic** for the same delivered tx sequence.

## Engine work checklist

1. **Version**: match docs and config knobs to the **CometBFT** (or embedded) version actually running.
2. **Boundary**: reproduce the issue at **ABCI** level (which method, what `Response*`, what codes).
3. **Config**: correlate `config.toml` / `genesis.json` with behavior (timeouts, mempool type, evidence params).
4. **Ops**: disk, peers, RPC exposure, **priv_validator** state, and **halt** / **restart** semantics.
5. **Sync**: choose **block** vs **state** sync vs **light** verification deliberately; each has different trust assumptions.
6. **Evidence**: treat double-sign and light-client attacks as **consensus-critical**; follow production guidance.

## Response approach

1. Identify **CometBFT version**, **sync mode**, and whether the topic is **mempool**, **consensus**, or **app ABCI**.
2. Prefer the **local doc mirror** under `references/docs` (see `references/README.md`) or current [CometBFT documentation](https://docs.cometbft.com).
3. Separate **engine** vs **application** causes; cite ABCI request/response fields when relevant.
4. Call out **security** (RPC exposure, **priv_validator**, peer trust) and **availability** (sentries, backups, retention).
5. Propose **verifiable** steps: RPC queries, log lines, config diffs, or minimal repro with `abci-cli` / known kvstore — not hand-wavy advice.

## Quick symptom → where to look

Paths are relative to `references/docs/` (see full index in `references/README.md`).

| Symptom | Doc area (local mirror) |
|---------|-------------------------|
| “Tx stays pending / mempool weird” | `core/mempool.md`, `core/using-cometbft.md`, `app-dev/getting-started.md` |
| “ABCI errors / app hash mismatch” | `app-dev/app-architecture.md`, `introduction/README.md` (ABCI overview), spec links in root `README.md` |
| “Node won’t catch up” | `core/block-sync.md`, `core/state-sync.md` |
| “RPC / subscription issues” | `core/rpc.md`, `core/subscription.md` |
| “Validator / signing problems” | `core/validators.md`, `references/config/priv_validator_key.json.md`, `references/config/priv_validator_state.json.md` |
| “Production hardening” | `core/running-in-production.md`, `core/configuration.md`, `networks/` |

## Common mistakes

- Debugging **SDK keepers** while ignoring **CheckTx** vs **deliver** divergence — compare mempool admission to committed execution.
- Changing **`genesis.json`** or **consensus params** without a coordinated **chain-id** / **upgrade** plan across validators.
- Treating **light client** trust as full-node trust — bootstraps and witness sets matter.
- Exposing **unsafe** RPC or **priv_validator** keys on shared networks.
- Assuming **Tendermint**-era blog posts match **CometBFT** **ABCI++** without checking versioned behavior.

## Limitations

- This skill does not replace your **node’s** actual `config.toml`, **genesis**, or **deployment** repo — anchor answers in what is running.
- The mirror under `references/docs` may **drift** from [docs.cometbft.com](https://docs.cometbft.com); prefer the **release tag** in your binary when they disagree.
- Stop and escalate when the issue is **cryptographic protocol design**, **coordinated incident response**, or **legal/compliance** around evidence or slashing.
