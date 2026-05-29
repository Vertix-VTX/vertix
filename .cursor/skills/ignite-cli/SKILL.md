---
name: ignite-cli
description: >-
  Use when scaffolding, building, or debugging Cosmos SDK chains with Ignite CLI,
  including ignite scaffold, ignite chain, config.yml, IBC relayer workflows,
  client generation, testnets, or upgrading Ignite-generated projects.
metadata:
  category: technique
  triggers:
    - ignite
    - ignite-cli
    - scaffold chain
    - config.yml
    - ignite chain
    - ignite scaffold
    - ignite relayer
    - ignite generate
    - ignite testnet
    - ignite app
    - buf.build
risk: unknown
source: community
date_added: "2026-05-03"
---

# Ignite CLI (Cosmos chain scaffolding)

Operate as a senior blockchain engineer focused on [Ignite CLI](https://github.com/ignite/cli): **scaffolding**, **local dev** (`ignite chain serve`), **IBC relayer** helpers, **client generation**, and **project config** around Cosmos SDK application chains.

## When to use this skill

- Creating or extending an **Ignite-scaffolded** chain (`ignite scaffold chain`, modules, messages, queries, packets)
- **Running** or **debugging** dev nodes, **Docker** workflows, or **simapp**-style testing with Ignite
- Editing **`config.yml`**, **accounts**, **faucet**, or **genesis** tuning for local/test networks
- **IBC** tutorials and **`ignite relayer`** usage between chains
- **`ignite generate`** for OpenAPI / **TypeScript** / **Vue** / **Go** clients
- **Version bumps** and **migration** notes when upgrading Ignite or regenerated templates

## Do not use this skill when

- The task is **Cosmos SDK module / BaseApp / ABCI** internals with **no Ignite tooling** surface — use **cosmos-sdk** (`skills/cosmos-sdk/SKILL.md`) instead
- The task is **generic Go** only — use **golang-pro** if present in your skill set
- You lack **Ignite version**, **scaffold layout**, or whether the repo was **hand-wired** vs Ignite-generated — clarify first

## Pair with cosmos-sdk

Ignite **generates** and **wraps** Cosmos SDK apps. Use **this skill** for CLI, `config.yml`, scaffolding, and Ignite-specific packages; use **cosmos-sdk** for keeper design, store migrations, consensus-facing behavior, and SDK module patterns.

## Core mental model

| Piece | Responsibility |
|--------|----------------|
| **`ignite scaffold`** | Creates chains, modules, Msgs, queries, IBC handlers, and related boilerplate |
| **`ignite chain`** | Build, init, **serve** (live reload), and run chain binaries for development |
| **`config.yml`** | Project-level dev config (accounts, faucet, genesis tweaks, host bindings) |
| **`ignite relayer`** | IBC connection/channel lifecycle helpers for local / test setups |
| **`ignite generate`** | Clients and API artifacts from chain code (OpenAPI, TS, Vue, Go) |

Chain **consensus** and **state rules** still live in the **generated SDK app** — Ignite accelerates setup; it does not replace SDK correctness.

## Workflow checklist

1. **Pin context**: Ignite **CLI version**, scaffold **template** era, and **Go** / **Cosmos SDK** versions from `go.mod` and release notes.
2. **Prefer generated layout**: avoid fighting the generator — extend modules under `x/`, wire in `app/`, keep `buf` / proto layout consistent with what `ignite scaffold` produced.
3. **Dev loop**: `ignite chain serve` and logs first; use **debug** and **Docker** guides when reproducing environment-specific failures.
4. **IBC**: confirm **IDs**, **ports**, **ordering**, and **relayer** state; cross-check with the IBC guide and relayer command reference.
5. **Upgrades**: read **migration** docs for your **from → to** Ignite major lines before re-scaffolding or mass-renaming.

## Response approach

1. Identify whether the issue is **Ignite CLI**, **generated app code**, or **pure SDK** — route SDK-only work to **cosmos-sdk**.
2. Trace **command → config.yml → generated binary** for runtime problems; trace **scaffold → proto → codegen** for compile/codegen failures.
3. Prefer the **local doc mirror** under `references/docs/` (see `references/README.md`) or current [Ignite documentation](https://docs.ignite.com).
4. Call out **breaking changes** across Ignite majors (see `06-migration/` in the mirror).
5. Propose **verifiable** steps: exact `ignite` subcommands, `config.yml` snippets, and rebuild/serve checks.

## Quick symptom → where to look

Paths are relative to `references/docs/`.

| Symptom | Doc area (local mirror) |
|---------|-------------------------|
| Install / what is Ignite | `01-welcome/01-index.md`, `01-welcome/02-install.md` |
| First chain / directory layout | `02-guide/02-introduction.md`, `02-guide/03-hello-world.md` |
| Command reference / flags | `03-CLI-Commands/01-cli-commands.md` |
| IBC walkthrough | `02-guide/04-ibc.md`, `02-guide/images/packet_sendpost.png` |
| Debug / Docker / simapp / state | `02-guide/05-debug.md`, `02-guide/06-docker.md`, `02-guide/07-simapp.md`, `02-guide/08-state.md` |
| `config.yml` | `08-configuration/01-config.md`, `08-configuration/02-config_example.md` |
| Go / TS / Vue clients | `04-clients/*.md` |
| Programmatic / library packages | `07-packages/*.md` |
| Ignite Apps | `apps/*.md` |
| Upgrade release notes | `06-migration/*.md` |

Full index: `references/README.md`.

## Common mistakes

- Using **`ignite account`** for **on-chain** keys — those commands target **Ignite** key material; use the **chain binary** (`*d keys`) for validator / account keys tied to `config.yml` (see CLI docs synopsis for `ignite account`).
- Editing **generated** files that **rescaffold** overwrites — know which paths are safe to customize vs regenerate.
- Assuming **`ignite chain serve`** matches **production** — defaults favor **dev** (reload, accounts, faucet).
- **IBC** misconfiguration (wrong chain IDs, ports, or channel filters) masked as “relayer broken”.
- Skipping **migration** notes when jumping **Ignite major** versions.

## Limitations

- This skill does not replace your repo’s **`go.mod`**, **forked templates**, or **custom** post-scaffold scripts — anchor answers in the project’s actual layout and versions.
- The local mirror may **drift** from upstream; prefer pinned **Ignite releases** and [docs.ignite.com](https://docs.ignite.com) when they disagree.
- Stop and escalate when the problem is **economic**, **consensus-critical**, or **key custody** in production without clear requirements.
