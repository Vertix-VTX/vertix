---
description: >-
  Index of Ignite CLI topics mapped to the local doc mirror under
  docs for deep dives while using the ignite-cli skill.
metadata:
  tags: [ignite-cli, documentation, navigation]
  source: internal
---

# Ignite CLI doc map (local mirror)

**Base path:** `docs/`

Tables list paths **under that directory** (Docusaurus-style layout from upstream docs).

## Welcome

| Topic | Path |
|--------|------|
| Introduction, ecosystem | `01-welcome/01-index.md` |
| Installation | `01-welcome/02-install.md` |

## Guides

| Topic | Path |
|--------|------|
| Introduction / `ignite scaffold chain` | `02-guide/02-introduction.md` |
| Hello world | `02-guide/03-hello-world.md` |
| IBC | `02-guide/04-ibc.md` |
| Debugging | `02-guide/05-debug.md` |
| Docker | `02-guide/06-docker.md` |
| Simapp | `02-guide/07-simapp.md` |
| State | `02-guide/08-state.md` |

## CLI reference

| Topic | Path |
|--------|------|
| All commands (generated help text) | `03-CLI-Commands/01-cli-commands.md` |

## Clients

| Topic | Path |
|--------|------|
| Go client | `04-clients/01-go-client.md` |
| TypeScript | `04-clients/02-typescript.md` |
| Vue | `04-clients/03-vue.md` |

## Apps & contributing

| Topic | Path |
|--------|------|
| Using / developing Ignite Apps | `apps/01-using-apps.md`, `apps/02-developing-apps.md` |
| Contributing to docs | `05-contributing/01-docs.md` |

## Migrations

| Topic | Path |
|--------|------|
| Index / readme | `06-migration/readme.md` |
| Per-version notes | `06-migration/v*.md` |

## Packages (Go APIs)

| Topic | Path |
|--------|------|
| `chaincmd`, `chaincmdrunner`, `chainregistry` | `07-packages/chaincmd.md`, `chaincmdrunner.md`, `chainregistry.md` |
| Cosmos analysis / buf / gen / ver | `07-packages/cosmosanalysis.md`, `cosmosbuf.md`, `cosmosgen.md`, `cosmosver.md` |
| Account, client, faucet, tx collector | `07-packages/cosmosaccount.md`, `cosmosclient.md`, `cosmosfaucet.md`, `cosmostxcollector.md` |

## Configuration

| Topic | Path |
|--------|------|
| `config.yml` reference | `08-configuration/01-config.md` |
| Example config | `08-configuration/02-config_example.md` |

## Official source

When the mirror is missing or outdated, use [Ignite documentation](https://docs.ignite.com) and the Ignite / SDK versions pinned in the target repo’s `go.mod` and Ignite release notes.
