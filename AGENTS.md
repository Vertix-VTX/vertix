# AGENTS.md — Vertix Repository Guide for AI Agents

This file is the **entry point for any AI coding agent** working on the Vertix repository. Read it first. Follow the documents it links to. Do not invent conventions that contradict them.

If you have time for nothing else, read these three documents in order:

1. [`docs/project-overview.md`](./docs/project-overview.md) — what Vertix is.
2. [`docs/architecture.md`](./docs/architecture.md) — how it fits together.
3. [`docs/coding-standards.md`](./docs/coding-standards.md) — how to contribute.

---

## 1. What is Vertix?

Vertix is a Cosmos SDK Layer 1 blockchain (`vertix-1`, ticker `VTX`) for Real World Asset (RWA) tokenization, powered by a validator-integrated oracle. It is pre-launch, currently being implemented against a 12-month roadmap.

**Three custom modules** (the heart of the protocol):
- `x/oracle` — validator-integrated price feeds with stake-weighted median aggregation.
- `x/rwa` — protocol-agnostic RWA registry with bonding, lifecycle, and factory-denom minting.
- `x/fees` — `EndBlock` fee burn (40%) and staker distribution (60%).

**One off-chain binary**:
- `vertix-feeder` — the per-validator price-feed sidecar.

**Hard rule:** total supply is capped at **21,000,000 VTX**. There is no `x/mint`. Do not reintroduce it.

---

## 2. Documentation Index

All design and process docs live in [`docs/`](./docs/). Always check these before writing code.

### Project Knowledge
| Doc | Read when |
|---|---|
| [`docs/project-overview.md`](./docs/project-overview.md) | Onboarding, framing a new task |
| [`docs/architecture.md`](./docs/architecture.md) | Building anything that crosses module boundaries |
| [`docs/technical-design.md`](./docs/technical-design.md) | Implementing or modifying a custom module |
| [`docs/project-structure.md`](./docs/project-structure.md) | Creating new files / unsure where code goes |
| [`docs/coding-standards.md`](./docs/coding-standards.md) | Every code change, every PR |
| [`docs/tokenomics.md`](./docs/tokenomics.md) | Touching genesis, vesting, fees, supply |
| [`docs/roadmap.md`](./docs/roadmap.md) | Scope / sequencing / milestone questions |
| [`docs/relayer.md`](./docs/relayer.md) | Setting up Hermes/`rly` to relay ICS-20 (VTX, `rwa/*`) |
| [`docs/testnet-runbook.md`](./docs/testnet-runbook.md) | Public testnet live-ops, incidents, Phase 9 gate |
| [`docs/validator-onboarding.md`](./docs/validator-onboarding.md) | External validator + feeder join contract |
| [`docs/rwa-quickstart.md`](./docs/rwa-quickstart.md) | RWA lifecycle walkthrough on testnet |
| [`docs/bug-bounty.md`](./docs/bug-bounty.md) | Bug bounty scope + severity rubric |

### Source of Truth
| Path | Purpose |
|---|---|
| [`docs/full-design-spec.md`](./docs/full-design-spec.md) | The **approved** full design spec (program source of truth: invariants, canonical phase order, cross-phase contracts, acceptance gates) |
| [`docs/plans/`](./docs/plans/) | Step-by-step execution plans (one per phase) |

### Execution Plans (run in order; each builds on the previous)
| # | Plan | Outputs |
|---|---|---|
| 01 | [`docs/plans/2026-05-09-01-chain-foundation.md`](./docs/plans/2026-05-09-01-chain-foundation.md) | Bootable chain, no `x/mint`, CI, Makefile |
| 02 | [`docs/plans/2026-05-09-02-oracle-module.md`](./docs/plans/2026-05-09-02-oracle-module.md) | `x/oracle` module |
| 03 | [`docs/plans/2026-05-09-03-fees-module.md`](./docs/plans/2026-05-09-03-fees-module.md) | `x/fees` module |
| 04 | [`docs/plans/2026-05-09-04-rwa-module.md`](./docs/plans/2026-05-09-04-rwa-module.md) | `x/rwa` module |
| 05 | [`docs/plans/2026-05-09-05-oracle-feeder-sidecar.md`](./docs/plans/2026-05-09-05-oracle-feeder-sidecar.md) | `vertix-feeder` binary |
| 06 | [`docs/plans/2026-05-09-06-ibc-devnet.md`](./docs/plans/2026-05-09-06-ibc-devnet.md) | IBC E2E + devnet stack |
| 08 | [`docs/plans/2026-05-29-phase-8-public-testnet.md`](./docs/plans/2026-05-29-phase-8-public-testnet.md) | `vertix-testnet-1` launch artifacts + runbook |
| 09 | [`docs/plans/2026-05-29-phase-9-testnet-v2-genesis-rehearsal.md`](./docs/plans/2026-05-29-phase-9-testnet-v2-genesis-rehearsal.md) | `vertix-testnet-2`, Cosmovisor upgrade, gentx rehearsal, load SLO, registry drafts |

---

## 3. Tech Stack (Pinned)

| Component | Version |
|---|---|
| Cosmos SDK | v0.50.x |
| CometBFT | v0.38.x |
| ibc-go | v8.x |
| Go | 1.22+ |
| Ignite CLI | v28.x |
| Hermes Relayer | v1.8.x |
| Cosmovisor | v1.5.x |
| golangci-lint | v1.57+ |
| buf | v1.30+ |

Do **not** bump these without an explicit roadmap-aligned task.

---

## 4. Repository Layout (One Glance)

```
vertix/
├── app/             SDK app construction (no x/mint)
├── cmd/vertixd/     Chain binary
├── x/{oracle,rwa,fees}/   Custom modules
├── feeder/          vertix-feeder sidecar (Go binary)
├── proto/vertix/    Protobuf source of truth
├── testutil/        Shared test helpers
├── e2e/             interchaintest IBC E2E (own go.mod)
├── infra/           Devnet, hermes, monitoring, explorer
│   └── testnet/     Phase 8 — infra/testnet/ public stack + node kit
├── docs/            All design + plans + this guide
├── .github/workflows/ci.yml
├── Makefile
├── config.yml       Ignite chain identity + devnet genesis
└── AGENTS.md        ← you are here
```

For full details: [`docs/project-structure.md`](./docs/project-structure.md).

---

## 5. Required Behavior for AI Agents

### 5.1 Before You Write Code

1. **Identify the relevant doc(s)** from §2 and read them.
2. If a [`docs/plans/`](./docs/plans/) plan covers the task, follow that plan task-by-task. Plans are designed to be agent-executable.
3. If your change crosses module boundaries, re-read [`docs/architecture.md`](./docs/architecture.md) §3 and [`docs/technical-design.md`](./docs/technical-design.md) §7.
4. Verify the change is consistent with the approved spec ([`docs/full-design-spec.md`](./docs/full-design-spec.md)).

### 5.2 While You Code

- Follow [`docs/coding-standards.md`](./docs/coding-standards.md) — every section.
- Test-first: add a failing test, implement, make it pass.
- Keep proto changes additive (no field renumbering, no removals).
- Never modify standard SDK module sources; only wire them in `app/app.go`.
- Emit events on every state change.

### 5.3 Before You Commit

Run all of:

```bash
make lint
make test
make build
```

All three must succeed locally before pushing. CI runs the same checks.

### 5.4 Commit / PR

- Use Conventional Commits with module scope (see [`docs/coding-standards.md`](./docs/coding-standards.md) §7.1).
- One logical change per PR.
- Reference the spec or plan you're executing in the PR description.

---

## 6. Architectural Invariants (Never Break These)

These are repeated from [`docs/architecture.md`](./docs/architecture.md) §9 because they are critical:

1. **21M hard cap.** No `x/mint`. No silent inflation. Burn is permanent.
2. **Validator-integrated oracle.** Oracle authority = active validator set. Sidecar is mandatory; misses are slashed.
3. **RWA bonding.** No asset reaches `ACTIVE` without a locked issuer bond ≥ `MinIssuerBond`.
4. **Fee determinism.** `EndBlock` fee split is fully on-chain; off-chain code never influences burn/distribute.
5. **Standard SDK modules unmodified.** Only custom modules (`x/oracle`, `x/rwa`, `x/fees`) carry Vertix-specific logic.

**Crisis invariant routes (Phase 7+).** Five permanent `x/crisis` checks — keep them passing under simulation and in production; any change that breaks one is rejected by default:

| Route | Statement (summary) |
|---|---|
| `oracle/prices` | Stored aggregated prices are accept-listed and strictly positive |
| `fees/reconcile` | `genesisSupply − supply(uvtx) == cumulativeBurned` (⇒ 21M cap) |
| `fees/module-balance` | `x/fees` module account holds zero `uvtx` at block boundary |
| `rwa/bonds` | Module bond escrow ≥ required bonds; every `ACTIVE` asset is bonded |
| `rwa/denoms` | Pre-mint assets have zero factory-denom supply |

Details: [`docs/technical-design.md`](./docs/technical-design.md) §§2.8, 3.8, 4.6; [`docs/full-design-spec.md`](./docs/full-design-spec.md) Phase 7.

A change that contradicts any of these is rejected by default.

---

## 7. Common Tasks → Where to Look

| Task | Start here |
|---|---|
| Add a new oracle pair | [`docs/technical-design.md`](./docs/technical-design.md) §2 + governance `MsgUpdateParams` |
| Add a new RWA message | [`docs/technical-design.md`](./docs/technical-design.md) §3 + plan `04-rwa-module.md` |
| Tune fee burn ratio | Governance `MsgUpdateParams` against `x/fees`; see [`docs/tokenomics.md`](./docs/tokenomics.md) §6 |
| Add a price provider to feeder | `feeder/feeder/provider/` + plan `05-oracle-feeder-sidecar.md` |
| Add an IBC channel | [`docs/architecture.md`](./docs/architecture.md) §6 + plan `06-ibc-devnet.md` |
| Modify devnet topology | `infra/devnet/docker-compose.yml` + plan `06-ibc-devnet.md` |
| Wire a new SDK module | [`docs/coding-standards.md`](./docs/coding-standards.md) §4 + `app/app.go` |
| Change a genesis param | [`docs/technical-design.md`](./docs/technical-design.md) §1.2 + `config.yml` |

---

## 8. Quick Command Reference

```bash
# Build & run
make build                    # produce build/vertixd
make devnet-reset             # fresh devnet via Ignite
build/vertixd genesis validate-genesis

# Tests
make test                     # all (race + count=1)
make test-cover               # HTML coverage
go test ./x/oracle/... -v     # focused

# Lint & format
make lint
gofmt -w .

# Proto
make proto-gen                # generate Go from proto
make ts-gen                   # generate TypeScript client

# Feeder
go build -o build/vertix-feeder ./feeder/cmd/vertix-feeder
build/vertix-feeder --config feeder-config.example.yaml

# E2E (separate Go module)
make e2e                 # build vertixd image + run interchaintest ICS-20 suite
cd e2e && go test ./... -timeout 30m -v

# Multi-validator devnet (Phase 6, Docker)
make devnet-docker-build       # build vertix:devnet image
make localnet-up               # start 3 validators + feeders + gaia + hermes + explorer + faucet + monitoring
make localnet-reset            # deterministic fresh devnet
make devnet-smoke              # full e2e gate (oracle -> rwa -> fees -> ibc)
make localnet-down             # stop + remove
```

See [`docs/devnet.md`](./docs/devnet.md) for the full runbook.

---

## 9. When in Doubt

- **The spec wins.** If a doc disagrees with [`docs/full-design-spec.md`](./docs/full-design-spec.md), the spec is correct — flag the disagreement in your PR.
- **Standards beat preferences.** If your style differs from [`docs/coding-standards.md`](./docs/coding-standards.md), match the standards.
- **Plans beat ad-hoc work.** If a plan exists for what you're doing, run it; don't reinvent the order.
- **Ask the user before** changing pinned versions, removing tests, modifying standard SDK modules, or touching anything that affects supply.

---

## 10. Update Discipline

When you change the codebase in a way that affects what this file describes:

- New module → add it to §1, §4, and the relevant doc cross-references.
- New plan → add it to §2.
- New invariant → add it to §6 and to [`docs/architecture.md`](./docs/architecture.md) §9.
- New top-level directory → add it to §4 and to [`docs/project-structure.md`](./docs/project-structure.md).

This file and [`docs/`](./docs/) are part of the product. Treat them like code.
