---
description: >-
  Compact map of Cosmos ecosystem resources aligned with Awesome Cosmos;
  use for tool discovery and cross-links while operating cosmos-sdk-expert.
metadata:
  tags: [cosmos, ecosystem, ibc, tooling, awesome-list]
  source: community
---

# Ecosystem map (Awesome Cosmos alignment)

**Upstream index:** [Awesome Cosmos](https://github.com/cosmos/awesome-cosmos/blob/main/README.md) (community curated; not an official endorsement list).

Use this file as a **fast router**. Prefer official docs ([Cosmos SDK](https://docs.cosmos.network/), [IBC](https://ibc.cosmos.com/), [CometBFT](https://docs.cometbft.com/)) for protocol and API details.

## Core repos

| Area | Starting point |
|------|----------------|
| Hub reference app | [Gaia](https://github.com/cosmos/gaia) |
| Application framework | [Cosmos SDK](https://github.com/cosmos/cosmos-sdk) |
| IBC (Go) | [ibc-go](https://github.com/cosmos/ibc-go) |
| Consensus | [CometBFT](https://github.com/cometbft/cometbft) |
| Smart contracts (Wasm) | [CosmWasm](https://github.com/CosmWasm/cosmwasm) |
| Protobuf / Buf | [buf.build/cosmos](https://buf.build/cosmos) |
| Merkle proofs / trees | [IAVL](https://github.com/cosmos/iavl), [ICS23](https://github.com/cosmos/ics23) |

## Learning portals

| Audience | Link |
|----------|------|
| Tutorials | [Cosmos Developer Portal](https://tutorials.cosmos.network) |
| Structured courses | [Interchain Developer Academy](https://ida.interchain.io/) |
| Hub-specific | [Cosmos Hub docs](https://hub.cosmos.network/) |

## Clients & frontends

| Language / stack | Examples from Awesome Cosmos |
|------------------|------------------------------|
| JS / TS | [CosmJS](https://github.com/cosmos/cosmjs), [Telescope](https://github.com/osmosis-labs/telescope), [cosmos-kit](https://github.com/cosmology-tech/cosmos-kit), [graz](https://github.com/strangelove-ventures/graz) |
| Go | [Ignite CLI](https://github.com/ignite/cli) (scaffold + dev), [lens](https://github.com/strangelove-ventures/lens) |
| Python | [cosmpy](https://github.com/fetchai/cosmpy), [mospy](https://github.com/ctrl-Felix/mospy) |
| Rust | [cosmrs](https://github.com/cosmos/cosmos-rust/tree/main/cosmrs), [ocular](https://github.com/peggyjv/ocular) |

## IBC & relayers

| Need | Tool / repo |
|------|-------------|
| Protocol & ICS | [IBC protocol site](https://ibcprotocol.dev/), [ICS repo](https://github.com/cosmos/ibc/) |
| Go stack | [ibc-go](https://github.com/cosmos/ibc-go) |
| Rust stack | [ibc-rs](https://github.com/cosmos/ibc-rs) |
| Relayers | [Go relayer](https://github.com/cosmos/relayer), [Hermes](https://github.com/informalsystems/hermes), [ts-relayer](https://github.com/confio/ts-relayer) |
| E2E tests | [interchaintest](https://github.com/strangelove-ventures/interchaintest) |
| Local dev mesh | [local-interchain](https://github.com/Reecepbcups/local-interchain) |

## Registry & discovery

| Need | Link |
|------|------|
| Chain metadata (endpoints, assets) | [cosmos/chain-registry](https://github.com/cosmos/chain-registry/) |
| Discovery UI | [Cosmos Directory](https://cosmos.directory) |

## Testing & quality

| Need | Link |
|------|------|
| IBC E2E | [interchaintest](https://github.com/strangelove-ventures/interchaintest) |
| SDK-focused E2E | [Atomkraft](https://github.com/informalsystems/atomkraft-cosmos) |
| Load | [tm-load-test](https://github.com/informalsystems/tm-load-test), [cosmosloadtester](https://github.com/orijtech/cosmosloadtester) |
| CometBFT test double | [CometMock](https://github.com/informalsystems/CometMock) |
| Formal methods | [Quint](https://github.com/informalsystems/quint), [Apalache](https://github.com/informalsystems/apalache) |
| Static analysis | [cosmos-sdk-codeql](https://github.com/crypto-com/cosmos-sdk-codeql) |

## Templates & scaffolds

| Style | Link |
|-------|------|
| Ignite (CLI + UI path) | [Ignite CLI](https://github.com/ignite/cli) |
| Minimal module example | [cosmosregistry/example](https://github.com/cosmosregistry/example) |
| Minimal chain | [cosmosregistry/chain-minimal](https://github.com/cosmosregistry/chain-minimal) |
| Spawn | [spawn](https://github.com/rollchains/spawn) |

## Operations & validators

| Concern | Examples |
|---------|----------|
| Binary upgrades | [Cosmovisor](https://github.com/cosmos/cosmos-sdk/tree/main/cosmovisor#readme) |
| Signing / KMS | [tmkms](https://github.com/iqlusioninc/tmkms) |
| Multisig CLI | [multisig](https://github.com/informalsystems/multisig) |
| Genesis editing | [cosmos-genesis-tinkerer](https://github.com/hyphacoop/cosmos-genesis-tinkerer) |
| K8s operator | [cosmos-operator](https://github.com/strangelove-ventures/cosmos-operator) |
| Monitoring (sample) | [PANIC](https://github.com/SimplyVC/panic), [Tenderduty](https://github.com/blockpane/tenderduty), [cosmos-node-exporter](https://github.com/QuokkaStake/cosmos-node-exporter) |

## Indexers & data

| Style | Examples |
|-------|----------|
| Go indexer engine | [Cosmscan](https://github.com/cosmscan/cosmscan-go) |
| GraphQL / Postgres patterns | [BDJuno](https://github.com/forbole/bdjuno) |
| Generalized Go indexer | [Cosmos Indexer](https://github.com/DefiantLabs/cosmos-indexer) |

## Virtual machines (on SDK)

| VM | Entry |
|----|--------|
| Wasm | [CosmWasm](https://github.com/CosmWasm/cosmwasm) |
| EVM | [Ethermint](https://github.com/evmos/ethermint) (ecosystem; verify fork used by your chain) |
| JS contracts | [Agoric SDK](https://github.com/Agoric/agoric-sdk) |

## Security reading (selected)

| Topic | Link |
|-------|------|
| Cosmos app vuln patterns | [(Not So) Smart Cosmos](https://github.com/crytic/building-secure-contracts/tree/master/not-so-smart-contracts/cosmos) |
| Hardening | [Orijtech Cosmos hardening](https://cyber.orijtech.com/scsec/cosmos-hardening) |
| Go on Cosmos | [Orijtech Go coding guide](https://cyber.orijtech.com/scsec/cosmos-go-coding-guide) |

For the full set of explorers, bots, wallets, and niche tools, browse the upstream **[Awesome Cosmos README](https://github.com/cosmos/awesome-cosmos/blob/main/README.md)**.
