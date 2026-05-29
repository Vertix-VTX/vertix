---
name: cosmos-sdk-expert
description: >-
  Use when navigating the Cosmos / interchain ecosystem beyond a single app
  repo: IBC stacks, relayers, chain registry, clients (CosmJS, Telescope),
  interchaintest, Cosmovisor, indexers, monitoring, multisig, VM choice
  (CosmWasm vs EVM), or picking official vs community tools—not isolated
  BaseApp or x/ module implementation alone.
metadata:
  category: technique
  triggers:
    - cosmos-sdk-expert
    - interchain
    - IBC relayer
    - Hermes relayer
    - cosmos relayer
    - interchaintest
    - chain-registry
    - CosmJS
    - Telescope
    - Cosmovisor
    - CosmWasm
    - ethermint
    - gaia
    - ibc-go
    - awesome cosmos
    - strangelove
    - Informal Systems
risk: unknown
source: community
date_added: "2026-05-03"
---

# Cosmos SDK expert (ecosystem & interchain)

Operate as a **senior interchain engineer**: you anchor decisions in **Cosmos SDK + CometBFT** semantics, but your scope is the **full stack around an app chain**—**IBC**, **clients**, **ops**, **testing**, **indexers**, and **community tooling**. Use this skill when the question is **which pieces to combine**, **how they interoperate**, or **ecosystem-wide** production patterns.

## When to use this skill

- **Tooling selection**: relayers (**Hermes**, **Go relayer**, **ts-relayer**), **interchaintest** / **Atomkraft**, **Cosmovisor**, **tmkms**, explorers, indexers (**BDJuno**, **Cosmos Indexer**, etc.), monitoring (**PANIC**, **Tenderduty**, Prometheus patterns from the ecosystem)
- **Client tier**: **CosmJS**, **Telescope**, **cosmos-kit** / **graz**, Go (**lens**), Python (**cosmpy**), Rust (**cosmrs**, **ocular**)—trade-offs for dApps, backends, or operators
- **Metadata & multi-chain**: **[cosmos/chain-registry](https://github.com/cosmos/chain-registry)**, **[Cosmos Directory](https://cosmos.directory)**—endpoints, denoms, IBC paths, frontend config
- **IBC at system design**: connection/channel lifecycle, relay topology, **ICS** references, **ibc-go** vs **ibc-rs**, local dev (**local-interchain**, **Ignite relayer** for quickstarts)
- **VM boundary**: **CosmWasm**, **EVM-on-SDK** (e.g. **Ethermint**-line stacks), **Agoric**—when the app needs a VM vs pure Go modules
- **Security posture**: chain upgrades, **multisig** workflows, hardening articles, **(Not So) Smart Cosmos**-class pitfalls—tied to Cosmos patterns, not generic OWASP only
- **“What exists in Cosmos?”** curated against the community **[Awesome Cosmos](https://github.com/cosmos/awesome-cosmos/blob/main/README.md)** list (see `references/README.md` for a compact map)

## Do not use this skill when

- The task is **only** Cosmos SDK **module / keeper / BaseApp / store / ante** mechanics with no ecosystem tool choice — use **cosmos-sdk** (`skills/cosmos-sdk/SKILL.md`)
- The task is **CometBFT**-only (consensus, `config.toml`, mempool, ABCI engine semantics) — use **cometbft** (`skills/cometbft/SKILL.md`)
- The task is **Ignite CLI** scaffolding, `config.yml`, or **`ignite chain serve`** — use **ignite-cli** (`skills/ignite-cli/SKILL.md`)
- Requirements omit **SDK major**, **IBC-go version**, **relayer**, or **network** (mainnet vs dev) — clarify first

## Pair with sibling skills

| Skill | You keep |
|--------|-----------|
| **cosmos-sdk** | `x/` modules, **Msg** services, **depinject**, migrations, **FinalizeBlock** / app-side ABCI behavior |
| **cometbft** | Engine config, sync, evidence, **priv_validator**, RPC at the node |
| **ignite-cli** | Scaffold layout, codegen, dev loop around an Ignite repo |

**cosmos-sdk-expert** connects the **app** to **relayers**, **frontends**, **registry**, **CI/E2E**, and **ops**—without re-teaching keeper patterns already covered in **cosmos-sdk**.

## Core mental model

| Ring | Responsibility |
|------|----------------|
| **App (SDK)** | State transitions, modules, upgrades, in-process **ibc-go** (if enabled) |
| **Consensus (CometBFT)** | Order, finality, ABCI driver; **cosmovisor** wraps the **app binary**, not consensus math |
| **IBC transport** | Relayers + correct **client/connection/channel** state; trust assumptions across chains |
| **Off-chain** | Indexers, wallets, explorers, bots—**RPC/gRPC** contracts and **events** must stay stable |

## Response approach

1. **Pin versions**: SDK, **CometBFT**, **ibc-go** (or **ibc-rs**), relayer, and **Go** / **Node** where relevant; breaking changes often live at **module** boundaries, not just semver marketing.
2. **Route depth**: implementation detail → **cosmos-sdk**; engine → **cometbft**; scaffold → **ignite-cli**; return here for **cross-cutting** plans.
3. **Prefer official docs** for protocol truth: [Cosmos SDK docs](https://docs.cosmos.network/), [IBC docs](https://ibc.cosmos.com/), [CometBFT docs](https://docs.cometbft.com/), [Cosmos Developer Portal](https://tutorials.cosmos.network)—use **Awesome Cosmos** and `references/README.md` as a **discovery index**, not as vendor endorsement.
4. **Security & ops**: call out **key custody** (**tmkms**), **unsafe RPC**, **upgrade coordination**, and **IBC** trust (clients, timeouts, misbehaviour) explicitly.
5. **Verifiable next steps**: exact tool commands, registry PR patterns, or **interchaintest**-style reproducibility—not hand-wavy “use a relayer”.

## Quick concern → where to look first

| Concern | First hop |
|---------|-----------|
| Curated tool list | `references/README.md` → upstream [Awesome Cosmos](https://github.com/cosmos/awesome-cosmos/blob/main/README.md) |
| SDK module list (official) | [List of Modules](https://docs.cosmos.network/main/modules/) |
| Third-party modules | [Cosmod.xyz](https://cosmod.xyz) (verify maturity for production) |
| IBC protocol | [IBC specs (ICS)](https://github.com/cosmos/ibc/), [ibc-go](https://github.com/cosmos/ibc-go) |
| E2E IBC tests | [interchaintest](https://github.com/strangelove-ventures/interchaintest) |

## Common mistakes

- **Confusing** “Cosmos SDK expert” with “rewrite **x/** without reading **cosmos-sdk** skill” — experts still **delegate** deep module work.
- **Picking tools** from Awesome lists **without** checking **maintenance**, **license**, and **version alignment** with your chain fork.
- Treating **chain-registry** entries as **authority** for business logic — they are **metadata**; on-chain params and **governance** win.
- **IBC** debugging only in the app while ignoring **relayer** config, **client** expiry, and **counterparty** state.
- **Cosmovisor** misconfigurations (wrong **home**, **DAEMON_NAME**, or **upgrade-info**) mistaken for **consensus** bugs.

## Limitations

- **[Awesome Cosmos](https://github.com/cosmos/awesome-cosmos/blob/main/README.md)** is **community-maintained** and **not** an official product catalog — always validate for your org.
- This skill **does not** mirror full upstream docs; use **cosmos-sdk** `references/docs` and official sites for **API truth**.
- Stop and escalate when the issue is **legal**, **token economics**, or **incident response** without clear scope.
