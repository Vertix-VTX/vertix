# Phase 0 — Chain Foundation — Design Spec

**Date:** 2026-05-29
**Status:** Approved (brainstorm output)
**Phase:** 0 of the canonical phase map ([`full-design-spec.md`](../full-design-spec.md) §7.2)
**Depends on:** none · **Enables:** every later phase
**Owns:** the bootable `vertixd` chain, manual app wiring, genesis baseline, and the dev/CI toolchain gate.

> This is the per-phase design spec produced by the brainstorm of Phase 0. It refines — and must stay consistent with — the program source of truth ([`full-design-spec.md`](../full-design-spec.md)) and the engineering reference ([`technical-design.md`](../technical-design.md) §1, §7). Where this doc and the program spec disagree on phase order or cross-phase contracts, the program spec wins. Module internals defer to `technical-design.md`. The step-by-step execution plan lives in `docs/plans/` (produced after this spec).

---

## 1. Goal & Deliverable Boundary

Produce a **bootable single-node `vertixd` chain** with the full standard Cosmos SDK module set **minus `x/mint`**, manual `app.go` wiring, the §1.2 genesis defaults, and the complete developer/CI toolchain.

**In scope:**

- Chain identity (binary, Bech32 prefixes, denom, decimals, devnet chain ID).
- Standard SDK module wiring (unmodified), `x/mint` removed.
- Genesis parameter baseline ([`technical-design.md`](../technical-design.md) §1.2) + a handful of devnet dev accounts.
- Tooling: `Makefile`, `.golangci.yml`, `buf` config, GitHub Actions CI, pre-commit hooks.
- Single-node boot producing blocks.

**Out of scope (deferred):**

- Any custom module logic — `x/oracle` (Phase 1), `x/fees` (Phase 2), `x/rwa` (Phase 3).
- IBC channel **operation** (modules are wired in Phase 0; operation is Phase 5).
- Multi-validator topology (Phase 6).
- Token allocation + vesting schedules (Phase 9 genesis rehearsal).

---

## 2. Locked Decisions (from this brainstorm)

| # | Decision | Choice | Rationale |
|---|---|---|---|
| D1 | App construction | **Ignite scaffold → manual `app.go` wiring** (simapp-style) | Matches `technical-design.md` §7 literally; explicit Begin/EndBlock + InitGenesis ordering; auditability (Invariant 6); makes the `x/mint` absence trivially visible. |
| D2 | Genesis scope | **Module param defaults (§1.2) + a few devnet dev accounts only** | Matches the spec's "bootable chain" scope; full allocation/vesting belongs to Phase 9. |
| D3 | Canonical Go version | **Pin `go.mod` + CI to Go 1.22** | Aligns with SDK v0.50.x (built/tested on Go 1.21–1.22) and golangci-lint v1.57 (built with Go 1.22). Local Go 1.26 still builds via the `go` directive. |
| D4 | Execution approach | **Approach A: Ignite scaffold + adapt** | Fastest bootable chain; inherits proto pipeline, `cmd/vertixd`, buf config, `.gitignore`; honors `project-structure.md`. |
| D5 | Pre-commit hooks | **Plain shell git hook installed via `make hooks`** | Avoids adding a Python/Node dependency (`pre-commit`/lefthook) to a Go repo. |

These inherit and must not contradict the program-level locked decisions in [`full-design-spec.md`](../full-design-spec.md) §§1–6.

---

## 3. Target Repository Layout (Phase 0 outputs)

```
vertix/
├── app/
│   ├── app.go            manual wiring; module set; x/mint REMOVED; Begin/EndBlock + InitGenesis order
│   ├── app_config.go     module-account permissions registry (if needed by manual wiring)
│   ├── app_test.go       TestNoMintModule, TestModulesWired, TestGenesisRoundTrip
│   ├── encoding.go       codec / interface registry
│   └── export.go         genesis export helper
├── cmd/vertixd/
│   ├── main.go
│   └── cmd/root.go       cobra root + subcommands (init, start, keys, genesis, …)
├── proto/vertix/         buf workspace (no custom protos in P0; pipeline ready for P1)
├── testutil/             placeholder for keeper fixtures (filled from P1 onward)
├── scripts/
│   └── git-hooks/pre-commit   gofmt + goimports + lint on staged Go files
├── config.yml            Ignite identity + devnet accounts + genesis overrides
├── Makefile              build/install/test/test-cover/lint/proto-gen/ts-gen/validate-genesis/devnet/clean/hooks
├── .golangci.yml         lint config (Go 1.22, the coding-standards §2.2 linter set)
├── buf.yaml / buf.gen.yaml / proto/buf.*   proto lint + codegen
├── .github/workflows/ci.yml   lint + build + test + proto + validate-genesis on PR
├── go.mod / go.sum       module github.com/vertix-network/vertix; go 1.22; pinned deps
├── .gitignore            build/, coverage*, ~/.vertix, IDE state
└── (AGENTS.md, docs/ already present)
```

This realizes the `✅ scaffold-generated` and the Phase-0 `🛠️ created by plans` rows of [`project-structure.md`](../project-structure.md).

---

## 4. App Wiring (`app/app.go`)

### 4.1 Module set

**Wired (standard, unmodified):** `auth`, `bank`, `staking`, `gov`, `distribution`, `slashing`, `upgrade`, `params`, `crisis`, `feegrant`, `authz`, `capability`, `ibc`, `transfer`, `genutil`, `evidence`, `vesting`, `consensus`.

> `x/consensus` is required by Cosmos SDK v0.50 (replaces the consensus params previously held in CometBFT genesis). It is added beyond the [`technical-design.md`](../technical-design.md) §1.3 list, which predates this note; flag for a doc sync.

**Custom:** none in Phase 0. Wiring is structured with explicit, commented insertion points so `oracle → rwa → fees` slot in without re-architecting.

**Removed:** `x/mint` — not constructed, no store key, no module-manager entry, no genesis section. This is the headline divergence from the Ignite scaffold default.

### 4.2 Ordering (locked now, even with no custom modules)

- **`SetOrderEndBlockers`:** standard chain order today; custom suffix `oracle → rwa → fees` reserved (`technical-design.md` §7).
- **`SetOrderInitGenesis`:** SDK default order today; custom prefix `staking → oracle → rwa → fees` reserved.
- **`SetOrderBeginBlockers`:** SDK default (no custom Begin-block work in Vertix modules).

### 4.3 Reward funding without `x/mint`

With `x/mint` absent, `x/distribution` has no inflationary token source. In Phase 0 the devnet simply produces blocks; staking rewards are not yet funded (the Validator Incentives Pool plumbing is a later phase). Phase 0's only obligation is to **not** reintroduce inflation. (Invariants 1–2.)

---

## 5. Chain Identity & Genesis Parameters

### 5.1 Identity

| Field | Value |
|---|---|
| Binary | `vertixd` |
| Bech32 prefixes | `vtx`, `vtxvaloper`, `vtxvalcons` (+ pub variants) |
| Base denom | `uvtx` |
| Display denom | `vtx` |
| Decimals | 6 |
| Devnet chain ID | `vertix-devnet-1` |
| Default `min_gas_prices` | `0.025uvtx` |

### 5.2 Genesis parameter defaults (from [`technical-design.md`](../technical-design.md) §1.2)

| Module | Parameter | Value |
|---|---|---|
| `x/staking` | `bond_denom` | `uvtx` |
| `x/staking` | `unbonding_time` | `1814400s` (21 days) |
| `x/staking` | `max_validators` | `125` |
| `x/staking` | `min_commission_rate` | `0.05` |
| `x/gov` | `min_deposit` | `10000000000uvtx` |
| `x/gov` | `voting_period` | `432000s` (5 days) |
| `x/gov` | `expedited_voting_period` | `86400s` (1 day) |
| `x/gov` | `quorum` / `threshold` | `0.334` / `0.5` |
| `x/distribution` | `community_tax` | `0.02` |
| `x/slashing` | `signed_blocks_window` | `100` |
| `x/slashing` | `min_signed_per_window` | `0.5` |
| `x/slashing` | `slash_fraction_double_sign` | `0.05` |
| `x/slashing` | `slash_fraction_downtime` | `0.01` |
| `x/crisis` | `constant_fee` | `1000000000uvtx` |

Applied via `config.yml` genesis overrides for devnet and verified by `genesis validate-genesis`.

---

## 6. Toolchain & Developer Workflow

### 6.1 `go.mod`

Module `github.com/vertix-network/vertix`; `go 1.22`; pinned: cosmos-sdk v0.50.x, cometbft v0.38.x, ibc-go/v8. Confirm Ignite's scaffold actually pins v0.50.x; force-pin if it pulls a newer line.

### 6.2 Makefile targets ([`project-structure.md`](../project-structure.md) §11)

| Target | Action |
|---|---|
| `build` / `install` | compile `vertixd` → `build/` / `$GOPATH/bin` |
| `test` / `test-cover` | `go test ./... -race -count=1` / + HTML coverage |
| `lint` | `golangci-lint run` |
| `proto-gen` / `ts-gen` | `ignite generate proto-go` / `ignite generate ts-client` |
| `validate-genesis` | `vertixd genesis validate-genesis` |
| `devnet-reset` / `devnet` | `ignite chain serve` (`--reset-once` / keep state) |
| `hooks` | install `scripts/git-hooks/pre-commit` |
| `clean` | remove build artifacts |

### 6.3 Lint (`.golangci.yml`)

The [`coding-standards.md`](../coding-standards.md) §2.2 set: `govet, errcheck, staticcheck, unused, gosimple, ineffassign, typecheck, gofmt, goimports, revive, misspell, unconvert, unparam, prealloc`, with `goimports.local-prefixes: github.com/vertix-network/vertix`.

### 6.4 buf

`buf.yaml` + `buf.gen.yaml` wired even with no custom protos yet, so Phase 1 inherits a working `make proto-gen` + `buf lint` + `buf breaking` pipeline.

### 6.5 Pre-commit hooks

A lightweight repo-local `scripts/git-hooks/pre-commit`, installed via `make hooks`, running `gofmt -l` + `goimports` + `golangci-lint run` over staged Go files. Plain shell — no Python/Node dependency.

---

## 7. CI (`.github/workflows/ci.yml`)

Single PR-triggered workflow on Go 1.22 with Go module/build caching. Jobs:

- **lint** — `golangci-lint run`.
- **build** — `make build`.
- **test** — `make test` (`-race -count=1`).
- **proto** — `buf lint` + `buf breaking` against `main`.
- **genesis** — `make build` then `genesis validate-genesis`.

All green is the bar every later phase inherits (cross-phase CI/lint/proto gate).

---

## 8. Testing Strategy (`app/app_test.go`)

1. **`TestNoMintModule`** — module manager, store keys, and default genesis contain no `mint` module/key/section. (Invariant 1 code-level guard.)
2. **`TestModulesWired`** — every module in the §4.1 "wired" list is present and constructed; custom-module insertion points exist but are empty.
3. **`TestGenesisRoundTrip`** — `InitGenesis(ExportGenesis()) == ExportGenesis()` for default genesis ([`coding-standards.md`](../coding-standards.md) §4.5).
4. **Boot smoke check** — scripted `vertixd init` → `start` confirming block production and that `:1317` / `:9090` / `:26657` bind. Runnable in CI as an optional step or documented as a manual gate.

---

## 9. Acceptance Gate Mapping

| Spec gate ([`full-design-spec.md`](../full-design-spec.md) Phase 0) | How satisfied |
|---|---|
| Chain boots & produces blocks | `make devnet-reset` / boot smoke check |
| REST `:1317`, gRPC `:9090`, RPC `:26657` up | endpoints enabled in `config.yml` / `app.toml` defaults |
| `genesis validate-genesis` passes | `make validate-genesis` + CI `genesis` job |
| `TestNoMintModule` passes | `app/app_test.go` |
| CI green on a PR | `ci.yml` (lint + build + test + proto + genesis) |

---

## 10. Cross-Phase Contracts Established Here

- **App-wiring contract:** custom modules are added only in `app.go`; standard modules remain unmodified.
- **Genesis baseline:** the §5.2 param surface is the canonical baseline all later module params extend.
- **CI/lint/proto gate:** `make lint && make test && make build` (plus `buf` + `validate-genesis`) is the inherited bar for every later phase.

---

## 11. Risks & Open Items

| Item | Risk | Mitigation |
|---|---|---|
| Ignite (depinject) → manual `app.go` conversion | Highest-effort step; IBC + capability wiring is fiddly | Use SDK v0.50 `simapp` as the reference shape; convert incrementally and keep the chain booting at each step |
| SDK version drift in scaffold | Ignite may pull a line newer than v0.50.x | Verify in `go.mod` after scaffold; force-pin v0.50.x |
| golangci-lint v1.57 vs local Go 1.26 | Local lint noise for devs on newer Go | CI runs Go 1.22 (clean); document local guidance in README |
| `ts-gen` | No client consumers yet | Wire the target; do not exercise in CI |
| `x/consensus` vs `technical-design.md` §1.3 list | Doc omits it | Add `x/consensus` to the §1.3 wired list in a docs sync |

---

## 12. Definition of Done

- `vertixd` builds and a single node produces blocks with the §5 identity and params.
- `x/mint` is absent and `TestNoMintModule` (+ `TestModulesWired`, `TestGenesisRoundTrip`) pass.
- `make lint && make test && make build` and `make validate-genesis` succeed locally and in CI.
- A PR with all the above goes green — opening the gate to Phases 1, 2, and 5.

---

## Appendix A — Document Map

| Doc | Relationship to this spec |
|---|---|
| [`full-design-spec.md`](../full-design-spec.md) | Program source of truth; Phase 0 section is the parent of this spec |
| [`technical-design.md`](../technical-design.md) | Owns §1 identity/params and §7 wiring referenced here |
| [`project-structure.md`](../project-structure.md) | Target layout this phase begins to realize |
| [`coding-standards.md`](../coding-standards.md) | Lint set, test patterns, genesis round-trip requirement |
| `docs/plans/` | Step-by-step execution plan derived from this spec (next step) |
