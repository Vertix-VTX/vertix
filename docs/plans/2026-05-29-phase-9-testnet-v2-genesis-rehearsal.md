# Phase 9 — Testnet v2 + Genesis Rehearsal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a mainnet-equivalent operational dry run on `vertix-testnet-2` — gentx ceremony, Cosmovisor store-migration upgrade, module-realistic load test with calibrated SLO, and Chain Registry / wallet prep — per [`docs/specs/2026-05-29-phase-9-testnet-v2-genesis-rehearsal-design.md`](../specs/2026-05-29-phase-9-testnet-v2-genesis-rehearsal-design.md).

**Architecture:** Extend Phase 8 with v2 subpaths under `infra/testnet/v2/` and `scripts/testnet/v2/`. Reuse node kit, monitoring patterns, and Docker image builder from Phase 8. Register a real `x/upgrade` handler (`v0.2.0-testnet`) with a no-op `x/fees` migration (consensus version 1→2). Launch founder-only v2 via split gentx workflow (base builder → per-founder gentx → collect). Audit is **out of scope** (parallel track).

**Tech Stack:** Cosmos SDK v0.50 (`vertixd`), CometBFT v0.38, Cosmovisor v1.5, Docker Compose, `bash` + `jq` + `curl`, Go load client (`vertixd tx` flood).

**Conventions reused (read before starting):**
- `scripts/devnet/lib.sh` — `log/err/die/require/wait_http/rpc_height/wait_height/assert_eq/assert_gt`.
- `scripts/testnet/build-genesis.sh` — dockerized `vd()` + `jqi()` genesis pattern for v1.
- `infra/testnet/docker-compose.public.yml` — sentry/seed/founder topology to mirror for v2.
- `infra/testnet/node-kit/` — Cosmovisor layout; override `CHAIN_ID` / `GENESIS_URL` for v2.
- `scripts/testnet/integration-verify.sh` — E2E verification pattern to fork for v2.

---

## File Structure

**New `infra/testnet/v2/`:**
- `infra/testnet/v2/.env` — chain id `vertix-testnet-2`, image tags.
- `infra/testnet/v2/docker-compose.yml` — founder-only private stack (3 validators + sentries + seed + feeders + monitoring).
- `infra/testnet/v2/genesis/` — published `genesis.json`, `genesis.sha256`, `seeds.txt`, `persistent_peers.txt`.
- `infra/testnet/v2/gentxs/` — collected gentx JSONs (gitignored staging; `.gitkeep` only).

**New `scripts/testnet/v2/`:**
- `build-genesis-base.sh` — base genesis (accounts + params, no gentxs).
- `collect-gentxs.sh` — merge gentxs + validate + publish SHA256.
- `upgrade-test.sh` — gov proposal + Cosmovisor swap verification.
- `load-test.sh` — module-realistic tx flood runner.
- `integration-verify.sh` — v2 founder stack E2E gate.

**New `app/upgrades/v020/`:**
- `upgrade.go` — `SetUpgradeHandler` registration helper.
- `upgrade_test.go` — unit test for handler wiring.

**New `x/fees/module/migrations/v2/`:**
- `migrate.go` — no-op Migrate1to2.

**New `infra/loadtest/`:**
- `mix.toml` — 70/20/10 bank/oracle/rwa mix definition.
- `clients/txflood/main.go` — Go tx flood client.

**New `infra/chain-registry/`:**
- `testnet/chain.json`, `testnet/assetlist.json` — `vertix-testnet-2` draft.
- `mainnet/chain.json`, `mainnet/assetlist.json` — `vertix-1` draft (stretch).
- `testnet/keplr.json`, `testnet/leap.json` — wallet suggestChain configs.

**Stretch `scripts/mainnet/`:**
- `build-genesis.sh` — mainnet allocation from `docs/tokenomics.md`.
- `infra/mainnet/genesis/` — output dir.

**Modified:**
- `x/fees/module/module.go` — RegisterMigration 1→2, ConsensusVersion 2.
- `app/app.go` — call `v020.RegisterUpgradeHandlers(app)`.
- `Makefile` — `testnet-v2-*` targets.
- `docs/testnet-runbook.md`, `docs/validator-onboarding.md`, `docs/load-test.md`.
- `docs/roadmap.md`, `docs/project-structure.md`, `AGENTS.md`.
- `.gitignore` — `infra/testnet/v2/.gen/`, `infra/testnet/v2/gentxs/*.json`.

---

## Task 1: v2 env + directory scaffold

**Files:**
- Create: `infra/testnet/v2/.env`
- Create: `infra/testnet/v2/gentxs/.gitkeep`
- Create: `infra/testnet/v2/genesis/.gitkeep`
- Modify: `.gitignore`

- [ ] **Step 1: Create v2 env**

Create `infra/testnet/v2/.env`:

```bash
# infra/testnet/v2/.env — vertix-testnet-2 (Phase 9 rehearsal). NOT mainnet.
VERTIX_IMAGE=vertix:testnet
PROMETHEUS_IMAGE=prom/prometheus:v2.53.0
GRAFANA_IMAGE=grafana/grafana:11.1.0
TENDERDUTY_IMAGE=ghcr.io/blockpane/tenderduty:v2.4.0

CHAIN_ID=vertix-testnet-2
DENOM=uvtx
```

- [ ] **Step 2: Scaffold directories**

```bash
mkdir -p infra/testnet/v2/{genesis,gentxs}
touch infra/testnet/v2/gentxs/.gitkeep infra/testnet/v2/genesis/.gitkeep
```

- [ ] **Step 3: Update .gitignore**

Append:

```
infra/testnet/v2/.gen/
infra/testnet/v2/gentxs/*.json
!infra/testnet/v2/gentxs/.gitkeep
```

- [ ] **Step 4: Commit**

```bash
git add infra/testnet/v2/.env infra/testnet/v2/gentxs/.gitkeep infra/testnet/v2/genesis/.gitkeep .gitignore
git commit -m "feat(testnet-v2): scaffold v2 env and directory layout"
```

---

## Task 2: x/fees no-op migration (consensus 1→2)

**Files:**
- Create: `x/fees/module/migrations/v2/migrate.go`
- Modify: `x/fees/module/module.go`

- [ ] **Step 1: Write the failing test**

Create `x/fees/module/migrations/v2/migrate_test.go`:

```go
package v2_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	v2 "github.com/vertix-network/vertix/x/fees/module/migrations/v2"
)

func TestMigrate1to2_NoOp(t *testing.T) {
	err := v2.Migrate1to2(nil)
	require.NoError(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./x/fees/module/migrations/v2/... -v -count=1`
Expected: FAIL (package/file not found)

- [ ] **Step 3: Implement no-op migration**

Create `x/fees/module/migrations/v2/migrate.go`:

```go
package v2

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Migrate1to2 is a no-op migration registered for the v0.2.0-testnet upgrade rehearsal.
// It proves RunMigrations executes without mutating live fee state.
func Migrate1to2(_ sdk.Context) error {
	return nil
}
```

- [ ] **Step 4: Register migration + bump consensus version**

In `x/fees/module/module.go`, update `RegisterServices`:

```go
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), am.keeper)

	if err := cfg.RegisterMigration(types.ModuleName, 1, v2.Migrate1to2); err != nil {
		panic(err)
	}
}
```

Add import: `v2 "github.com/vertix-network/vertix/x/fees/module/migrations/v2"`.

Change `ConsensusVersion()` to return `2`.

- [ ] **Step 5: Run tests**

Run: `go test ./x/fees/... -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add x/fees/module/migrations/v2/ x/fees/module/module.go
git commit -m "feat(fees): add no-op v1→v2 migration for upgrade rehearsal"
```

---

## Task 3: x/upgrade handler registration

**Files:**
- Create: `app/upgrades/v020/upgrade.go`
- Create: `app/upgrades/v020/upgrade_test.go`
- Modify: `app/app.go`

- [ ] **Step 1: Write upgrade registration helper**

Create `app/upgrades/v020/upgrade.go`:

```go
package v020

import (
	"fmt"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
)

const UpgradeName = "v0.2.0-testnet"

type App struct {
	ModuleManager *module.Manager
	Configurator  module.Configurator
}

// RegisterUpgradeHandlers wires the Phase 9 Cosmovisor rehearsal upgrade plan.
func RegisterUpgradeHandlers(app *App, upgradeKeeper *upgradekeeper.Keeper) error {
	upgradeKeeper.SetUpgradeHandler(UpgradeName, func(ctx sdk.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		if plan.Name != UpgradeName {
			return nil, fmt.Errorf("unexpected upgrade plan %q", plan.Name)
		}
		return app.ModuleManager.RunMigrations(ctx, app.Configurator, fromVM)
	})
	return nil
}
```

- [ ] **Step 2: Wire in app.go**

After `app.App = appBuilder.Build(...)` and before `return app, nil`, add:

```go
if err := v020.RegisterUpgradeHandlers(
	&v020.App{ModuleManager: app.ModuleManager, Configurator: app.Configurator()},
	app.UpgradeKeeper,
); err != nil {
	return nil, err
}
```

Add import: `v020 "github.com/vertix-network/vertix/app/upgrades/v020"`.

- [ ] **Step 3: Write unit test**

Create `app/upgrades/v020/upgrade_test.go` verifying `UpgradeName == "v0.2.0-testnet"`.

- [ ] **Step 4: Build + test**

Run: `make build && go test ./app/upgrades/v020/... -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/upgrades/v020/ app/app.go
git commit -m "feat(upgrade): register v0.2.0-testnet store migration handler"
```

---

## Task 4: v2 base genesis builder

**Files:**
- Create: `scripts/testnet/v2/build-genesis-base.sh`
- Modify: `Makefile`

- [ ] **Step 1: Create base genesis script**

Create `scripts/testnet/v2/build-genesis-base.sh` — fork `scripts/testnet/build-genesis.sh` but:

1. Source `infra/testnet/v2/.env` + `infra/testnet/mnemonics.env` (reuse v1 mnemonics for founders).
2. Set `GEN=infra/testnet/v2/.gen`, `OUT=infra/testnet/v2/genesis/base`.
3. Use `CHAIN_ID=vertix-testnet-2`.
4. Add accounts + patch params (same as v1).
5. **Do NOT** run gentx/collect — output `base-genesis.json` only.
6. Print SHA256 of base genesis.

Key header comment at top of output genesis (via jq):

```bash
jqi '.chain_id="vertix-testnet-2" | .app_state.staking.params.bond_denom="uvtx"'
# Add comment in script log: "PUBLIC TESTNET v2 — NOT mainnet distribution"
```

- [ ] **Step 2: Add Makefile target**

```makefile
testnet-v2-genesis-base: devnet-docker-build testnet-tag-image
	@./scripts/testnet/v2/build-genesis-base.sh
```

- [ ] **Step 3: Run and validate**

Run: `make testnet-v2-genesis-base`
Run: `docker run --rm -v "$PWD/infra/testnet/v2/genesis/base:/g" vertix:testnet vertixd genesis validate-genesis /g/base-genesis.json`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add scripts/testnet/v2/build-genesis-base.sh Makefile
git commit -m "feat(testnet-v2): add base genesis builder for gentx ceremony"
```

---

## Task 5: Gentx collect script

**Files:**
- Create: `scripts/testnet/v2/collect-gentxs.sh`
- Modify: `Makefile`

- [ ] **Step 1: Create collect script**

Create `scripts/testnet/v2/collect-gentxs.sh`:

```bash
#!/usr/bin/env bash
# Merge gentx JSONs into final genesis for vertix-testnet-2.
# Usage: ./collect-gentxs.sh [gentx-dir]
set -euo pipefail
GENTX_DIR="${1:-infra/testnet/v2/gentxs}"
BASE="infra/testnet/v2/genesis/base/base-genesis.json"
OUT="infra/testnet/v2/genesis"
# 1. Copy base → working genesis
# 2. Copy all gentx-*.json into gentx/ subdir (sorted)
# 3. vd genesis collect-gentxs
# 4. vd genesis validate-genesis
# 5. Publish genesis.json + sha256sum → genesis.sha256
```

Include helper `make-gentx.sh` snippet in script comments for founders:

```bash
vertixd genesis gentx "$MONIKER" 1000000000000uvtx \
  --chain-id vertix-testnet-2 \
  --home "$HOME/.vertixd" \
  --output-document "gentx-$MONIKER.json"
```

- [ ] **Step 2: Internal test with founder gentxs**

Extend `build-genesis-base.sh` or add `scripts/testnet/v2/build-genesis-internal.sh` that auto-generates 3 founder gentxs (like v1) for **local verification only** — used by `make testnet-v2-genesis` but not the public ceremony workflow.

- [ ] **Step 3: Makefile target**

```makefile
testnet-v2-genesis: testnet-v2-genesis-base
	@./scripts/testnet/v2/build-genesis-internal.sh
	@./scripts/testnet/v2/collect-gentxs.sh
```

- [ ] **Step 4: Verify reproducible SHA256**

Run `make testnet-v2-genesis` twice; compare `infra/testnet/v2/genesis/genesis.sha256`.
Expected: identical hashes.

- [ ] **Step 5: Commit**

```bash
git add scripts/testnet/v2/collect-gentxs.sh scripts/testnet/v2/build-genesis-internal.sh Makefile
git commit -m "feat(testnet-v2): gentx collect workflow and internal genesis builder"
```

---

## Task 6: Founder-only v2 Docker Compose stack

**Files:**
- Create: `infra/testnet/v2/docker-compose.yml`
- Modify: `Makefile`

- [ ] **Step 1: Create v2 compose**

Fork `infra/testnet/docker-compose.public.yml`:

- Network name: `vertix-testnet-v2`
- Chain id env: `vertix-testnet-2`
- Mount `infra/testnet/v2/genesis/genesis.json`
- **No public RPC exposure** in internal profile — bind RPC to `127.0.0.1` only.
- Include: 3 founders, 3 sentries, 1 seed, 3 feeders, prometheus, grafana.
- Omit faucet/explorer for internal gate (add in Task 12 public-open profile).

- [ ] **Step 2: Makefile targets**

```makefile
testnet-v2-up: testnet-v2-genesis
	@docker compose --env-file infra/testnet/v2/.env -f infra/testnet/v2/docker-compose.yml up -d

testnet-v2-down:
	@docker compose --env-file infra/testnet/v2/.env -f infra/testnet/v2/docker-compose.yml down -v
```

- [ ] **Step 3: Smoke test**

Run: `make testnet-v2-up && sleep 30`
Query height via docker network.
Expected: height > 0

Run: `make testnet-v2-down`

- [ ] **Step 4: Commit**

```bash
git add infra/testnet/v2/docker-compose.yml Makefile
git commit -m "feat(testnet-v2): founder-only docker compose stack"
```

---

## Task 7: Cosmovisor upgrade E2E script

**Files:**
- Create: `scripts/testnet/v2/upgrade-test.sh`
- Modify: `Makefile`

- [ ] **Step 1: Create upgrade test script**

`scripts/testnet/v2/upgrade-test.sh` workflow:

1. Assert v2 stack running at height H.
2. Submit gov `SoftwareUpgrade` proposal for plan `v0.2.0-testnet` at height H+50 (testnet gov voting period is 300s — use expedited if available, or pre-vote in script).
3. Document Cosmovisor binary placement:

```bash
# On each validator host:
mkdir -p "$DAEMON_HOME/cosmovisor/upgrades/v0.2.0-testnet/bin"
cp build/vertixd "$DAEMON_HOME/cosmovisor/upgrades/v0.2.0-testnet/bin/vertixd"
```

4. Wait for upgrade height + 30s.
5. Assert height > H+50 and chain producing.
6. Query fees module still operational (`vertixd query fees params`).

For Docker-internal test: mount pre-built binary into cosmovisor upgrade dir via volume before upgrade height.

- [ ] **Step 2: Makefile target**

```makefile
testnet-v2-upgrade:
	@./scripts/testnet/v2/upgrade-test.sh
```

- [ ] **Step 3: Run E2E**

Run: `make testnet-v2-up && make testnet-v2-upgrade`
Expected: PASS — blocks resume post-upgrade.

- [ ] **Step 4: Commit**

```bash
git add scripts/testnet/v2/upgrade-test.sh Makefile
git commit -m "feat(testnet-v2): Cosmovisor upgrade E2E script"
```

---

## Task 8: v2 integration verify

**Files:**
- Create: `scripts/testnet/v2/integration-verify.sh`
- Modify: `Makefile`

- [ ] **Step 1: Fork integration-verify for v2**

Create `scripts/testnet/v2/integration-verify.sh`:

1. `make testnet-v2-up`
2. Wait for blocks
3. Run crisis invariant checks (reuse queries from Phase 7 — oracle prices positive, fees reconcile)
4. `make testnet-v2-upgrade`
5. Re-check invariants post-upgrade
6. `make testnet-v2-down`

- [ ] **Step 2: Makefile target**

```makefile
testnet-v2-verify:
	@./scripts/testnet/v2/integration-verify.sh
```

- [ ] **Step 3: Run full verify**

Run: `make testnet-v2-verify`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add scripts/testnet/v2/integration-verify.sh Makefile
git commit -m "feat(testnet-v2): integration verify gate (pre/post upgrade)"
```

---

## Task 9: Module-realistic load harness

**Files:**
- Create: `infra/loadtest/mix.toml`
- Create: `infra/loadtest/clients/txflood/main.go`
- Create: `scripts/testnet/v2/load-test.sh`
- Modify: `docs/load-test.md`
- Modify: `Makefile`

- [ ] **Step 1: Define mix config**

Create `infra/loadtest/mix.toml`:

```toml
[run]
duration_sec = 600
ramp_rates = [50, 100, 150]  # tx/s targets to try
rpc = "http://localhost:26657"
chain_id = "vertix-testnet-2"

[mix]
bank_send = 0.70
oracle_submit = 0.20
rwa_register = 0.10

[accounts]
# Funded load-test keys from testnet faucet / genesis
```

- [ ] **Step 2: Implement tx flood client**

Create `infra/loadtest/clients/txflood/main.go` using Cosmos SDK client:

- Sign and broadcast `MsgSend`, `MsgSubmitPrice`, `MsgRegisterAsset` (minimal fields).
- Track success/failure, record p50/p99 latency from broadcast to tx inclusion.
- Output JSON summary to stdout.

- [ ] **Step 3: Load test runner script**

`scripts/testnet/v2/load-test.sh`:

1. Assert v2 stack up.
2. Fund load-test accounts from faucet (or genesis).
3. Run txflood at ramp rates.
4. Compute peak sustained TPS.
5. Set bar = 80% peak; assert run meets bar OR print "CALIBRATION NEEDED" with numbers.
6. Append results to `docs/load-test.md` § "Phase 9 SLO (testnet v2)".

- [ ] **Step 4: Makefile target**

```makefile
testnet-v2-load-test:
	@./scripts/testnet/v2/load-test.sh
```

- [ ] **Step 5: Commit**

```bash
git add infra/loadtest/mix.toml infra/loadtest/clients/txflood/ scripts/testnet/v2/load-test.sh docs/load-test.md Makefile
git commit -m "feat(loadtest): module-realistic tx flood harness for testnet v2"
```

---

## Task 10: Chain Registry + wallet configs

**Files:**
- Create: `infra/chain-registry/testnet/chain.json`
- Create: `infra/chain-registry/testnet/assetlist.json`
- Create: `infra/chain-registry/testnet/keplr.json`
- Create: `infra/chain-registry/testnet/leap.json`
- Create: `infra/chain-registry/mainnet/chain.json` (stretch)
- Create: `infra/chain-registry/mainnet/assetlist.json` (stretch)

- [ ] **Step 1: Draft testnet chain.json**

Follow [cosmos/chain-registry schema](https://github.com/cosmos/chain-registry/blob/master/chain.schema.json):

```json
{
  "chain_name": "vertixtestnet",
  "chain_id": "vertix-testnet-2",
  "pretty_name": "Vertix Testnet",
  "bech32_prefix": "vertix",
  "slip44": 118,
  "apis": {
    "rpc": [{ "address": "https://rpc.testnet.vertix.example", "provider": "vertix" }],
    "rest": [{ "address": "https://lcd.testnet.vertix.example", "provider": "vertix" }]
  }
}
```

Use placeholder endpoints; document swap procedure in runbook for public open.

- [ ] **Step 2: Draft assetlist.json**

```json
{
  "chain_name": "vertixtestnet",
  "assets": [{
    "description": "Vertix native token (testnet)",
    "denom_units": [
      { "denom": "uvtx", "exponent": 0 },
      { "denom": "vtx", "exponent": 6 }
    ],
    "base": "uvtx",
    "name": "Vertix",
    "display": "vtx",
    "symbol": "VTX"
  }]
}
```

- [ ] **Step 3: Keplr + Leap suggestChain JSON**

Derive from chain.json; store as `keplr.json` and `leap.json`.

- [ ] **Step 4: Stretch mainnet drafts**

Same structure with `"chain_id": "vertix-1"`, `"stage": "pre-launch"` note in README.

- [ ] **Step 5: Commit**

```bash
git add infra/chain-registry/
git commit -m "feat(chain-registry): draft testnet v2 and mainnet chain metadata"
```

---

## Task 11: Runbook — v2 launch + gentx coordinator + upgrade

**Files:**
- Modify: `docs/testnet-runbook.md`

- [ ] **Step 1: Add §10 Testnet v2 (Phase 9)**

Sections:

1. **Internal gate overview** — founder-only first; v1 stays live.
2. **Gentx coordinator playbook** — deadline, base genesis distribution, collect, SHA256 publish, coordinated UTC start.
3. **Cosmovisor upgrade procedure** — gov proposal, binary placement, verification checklist.
4. **Internal → public open gate** — checklist from design spec §5.1.
5. **Public v2 open** — endpoint publication, validator invitation, registry PR.

- [ ] **Step 2: Update gentx appendix**

Replace "deferred to Phase 9" header with "Step 1 procedure rehearsal" and link to coordinator script.

- [ ] **Step 3: Commit**

```bash
git add docs/testnet-runbook.md
git commit -m "docs(testnet): v2 launch, gentx coordinator, and upgrade runbook"
```

---

## Task 12: Validator onboarding — v2 paths

**Files:**
- Modify: `docs/validator-onboarding.md`

- [ ] **Step 1: Document dual join paths**

| Phase | Join method |
|---|---|
| testnet v1 | Post-genesis `MsgCreateValidator` (existing) |
| testnet v2 ceremony | Pre-launch `genesis gentx` submission to coordinator |
| testnet v2 post-open | Post-genesis `MsgCreateValidator` (same as v1) |

- [ ] **Step 2: Node kit env overrides for v2**

```bash
CHAIN_ID=vertix-testnet-2
GENESIS_URL=https://.../genesis.json
GENESIS_SHA256=<from infra/testnet/v2/genesis/genesis.sha256>
```

- [ ] **Step 3: Cosmovisor upgrade example**

Document exact `$DAEMON_HOME/cosmovisor/` layout with `v0.2.0-testnet` worked example.

- [ ] **Step 4: Commit**

```bash
git add docs/validator-onboarding.md
git commit -m "docs(onboarding): v2 chain id, gentx ceremony, and cosmovisor upgrade paths"
```

---

## Task 13: Doc-syncs

**Files:**
- Modify: `docs/roadmap.md`
- Modify: `docs/project-structure.md`
- Modify: `AGENTS.md`
- Modify: `docs/full-design-spec.md` (Appendix C TPS resolution note)

- [ ] **Step 1: Update roadmap Phase 9 tasks** — mark audit as parallel track; add v2 deliverables.

- [ ] **Step 2: Update project-structure.md** — add `infra/testnet/v2/`, `infra/chain-registry/`, `app/upgrades/`, `scripts/testnet/v2/`.

- [ ] **Step 3: Update AGENTS.md §2** — add Phase 9 plan link when complete; add chain-registry to doc index.

- [ ] **Step 4: Resolve Appendix C TPS item** — note Phase 9 calibrates bar in `docs/load-test.md`.

- [ ] **Step 5: Commit**

```bash
git add docs/roadmap.md docs/project-structure.md AGENTS.md docs/full-design-spec.md
git commit -m "docs: sync Phase 9 artifacts into roadmap and project structure"
```

---

## Task 14 (Stretch): Mainnet genesis builder

**Files:**
- Create: `scripts/mainnet/build-genesis.sh`
- Create: `scripts/mainnet/allocations.json` — uvtx amounts per tokenomics.md category
- Create: `infra/mainnet/genesis/.gitkeep`

- [ ] **Step 1: Define allocations JSON**

Map all 7 categories from `docs/tokenomics.md` to addresses + vesting types:

| Category | uvtx | Vesting type |
|---|---|---|
| Team | 4620000000000 | PeriodicVestingAccount (12mo cliff, 36mo linear) |
| Foundation | 4200000000000 | PeriodicVestingAccount (12mo cliff, 48mo linear) |
| ... | ... | ... |

Total must equal `21000000000000uvtx`.

- [ ] **Step 2: Build genesis script**

Use `vertixd genesis add-genesis-account` with `--vesting-amount` and `--vesting-start-time` / `--vesting-end-time` flags per category.

Patch mainnet module params (gov voting periods per tokenomics, oracle accept-list, fees ratios).

Output: `infra/mainnet/genesis/genesis.json` + SHA256.

- [ ] **Step 3: Validate**

Run: `vertixd genesis validate-genesis infra/mainnet/genesis/genesis.json`
Assert total supply == 21M VTX.

- [ ] **Step 4: Commit**

```bash
git add scripts/mainnet/ infra/mainnet/
git commit -m "feat(mainnet): genesis builder with tokenomics vesting (stretch)"
```

---

## Task 15: Final gate — lint, test, build

- [ ] **Step 1: Run full CI locally**

```bash
make lint
make test
make build
```

Expected: all PASS

- [ ] **Step 2: Run v2 integration verify**

```bash
make testnet-v2-verify
```

Expected: PASS

- [ ] **Step 3: Final commit if any fixups needed**

---

## Acceptance Checklist (maps to design spec §5)

**Internal gate (in-repo verifiable):**
- [ ] `make testnet-v2-genesis` reproducible SHA256
- [ ] `make testnet-v2-up` produces blocks
- [ ] `make testnet-v2-upgrade` succeeds with zero downtime
- [ ] `make testnet-v2-verify` passes pre/post upgrade
- [ ] `make testnet-v2-load-test` documents calibrated SLO in `docs/load-test.md`
- [ ] Chain registry drafts exist under `infra/chain-registry/testnet/`
- [ ] Runbook + onboarding updated

**Live ops (runbook-tracked):**
- [ ] 2-week founder stability on v2
- [ ] Public v2 open with external validators
- [ ] Registry PR drafted on GitHub
- [ ] Keplr/Leap tested against live RPC

**Stretch:**
- [ ] `scripts/mainnet/build-genesis.sh` produces valid 21M genesis

---

## Plan Self-Review

| Spec requirement | Task |
|---|---|
| WS1 v2 genesis tooling | Tasks 4, 5 |
| WS2 founder-only stack | Task 6 |
| WS3 gentx playbook | Tasks 5, 11 |
| WS4 upgrade handler | Tasks 2, 3 |
| WS5 Cosmovisor E2E | Task 7 |
| WS6 load harness | Task 9 |
| WS7 chain registry | Task 10 |
| WS8 runbook + onboarding | Tasks 11, 12 |
| WS9 mainnet builder | Task 14 (stretch) |
| Audit deferred | Not in plan (parallel track) |

No TBD/TODO placeholders in task steps.
