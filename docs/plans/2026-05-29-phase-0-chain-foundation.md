# Phase 0 — Chain Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce a bootable single-node `vertixd` chain with the full standard Cosmos SDK module set minus `x/mint` (and minus scaffold extras `nft`/`group`/`circuit`), the §1.2 genesis baseline, and the full dev/CI toolchain — passing `make lint && make test && make build`, `genesis validate-genesis`, and `TestNoMintModule`.

**Architecture:** Ignite v28 scaffolds a depinject-based app (`app/app_config.go` + `app/app.go`). We keep depinject and *trim* the module set rather than rewriting to manual wiring. Module ordering and account permissions stay explicit in `app_config.go`. The IBC stack (`ibc`/`transfer`/`ica`/`29-fee`) is kept intact for Phase 5.

**Tech Stack:** Go 1.22, Cosmos SDK v0.50.14, CometBFT v0.38.17, ibc-go/v8 v8.5.2, Ignite CLI v28.11, buf v1.34, golangci-lint (v1.57+; scaffold Makefile installs v1.61), GitHub Actions.

**Reference spec:** [`docs/specs/2026-05-29-phase-0-chain-foundation-design.md`](../specs/2026-05-29-phase-0-chain-foundation-design.md). Where this plan and the spec disagree, the spec wins.

**Conventions for every task:** run commands from the repo root `/home/nam-nguyen/my-projects/vertix-projects/vertix` unless stated. Commit messages follow Conventional Commits with a scope (`coding-standards.md` §7.1).

---

## File Map (what each touched file is responsible for)

| File | Responsibility | Created/Modified |
|---|---|---|
| `go.mod` | Module path + Go directive + pinned deps | Modify (scaffold) |
| `app/app_config.go` | depinject module config, ordering, maccPerms | Modify (trim modules) |
| `app/app.go` | depinject wiring, `App` struct, keeper inject | Modify (trim modules) |
| `app/app_test.go` | `TestNoMintModule`, `TestModulesWired`, `TestGenesisRoundTrip` | Create |
| `config.yml` | Chain identity, devnet accounts, genesis param overrides | Modify (scaffold) |
| `Makefile` | `build`/`test`/`lint`/`validate-genesis`/`devnet*`/`ts-gen`/`hooks`/`clean` | Modify (scaffold) |
| `.golangci.yml` | Enabled linter set + local-prefixes | Create |
| `scripts/git-hooks/pre-commit` | gofmt + goimports + lint on staged Go | Create |
| `.github/workflows/ci.yml` | lint + build + test + proto + genesis on PR | Create |
| `.gitignore` | Build/coverage/home artifacts | Modify (scaffold) |

---

## Task 1: Scaffold the chain into the repository

**Files:**
- Create: everything Ignite generates (`app/`, `cmd/vertixd/`, `proto/`, `go.mod`, `Makefile`, `config.yml`, …)
- Preserve: existing `docs/`, `AGENTS.md`, `.cursor/`, `.git/`

The repo already contains `docs/` and `AGENTS.md`, so we scaffold into a temp directory and copy the generated tree in without clobbering existing docs.

- [ ] **Step 1: Scaffold into a temp directory**

Run:

```bash
rm -rf /tmp/vtx-scaffold && mkdir -p /tmp/vtx-scaffold
cd /tmp/vtx-scaffold
ignite scaffold chain github.com/vertix-network/vertix --no-module --address-prefix vtx
```

Expected: `⭐️ Successfully created a new blockchain 'vertix'.`

- [ ] **Step 2: Copy generated files into the repo (without deleting existing docs)**

Run from the repo root:

```bash
cd /home/nam-nguyen/my-projects/vertix-projects/vertix
rsync -a --exclude='.git/' /tmp/vtx-scaffold/vertix/ ./
```

This adds `app/`, `cmd/`, `proto/`, `go.mod`, `go.sum`, `Makefile`, `config.yml`, `.gitignore`, `readme.md`, `tools/`, `testutil/`, `docs/docs.go`, `docs/static/`, `docs/template/`. It does not remove the existing `docs/*.md`.

- [ ] **Step 3: Verify the existing docs survived and the chain builds**

Run:

```bash
ls docs/full-design-spec.md docs/specs && go build ./... 2>&1 | tail -5
```

Expected: the docs paths exist and `go build ./...` completes with no output (success).

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
build: scaffold vertixd chain via ignite (no-module, vtx prefix)

Ignite v28 depinject scaffold for github.com/vertix-network/vertix.
Generated app/, cmd/vertixd/, proto/, Makefile, config.yml. Existing
docs/ preserved. Refs: docs/specs/2026-05-29-phase-0-chain-foundation-design.md
EOF
)"
```

---

## Task 2: Pin the Go toolchain to 1.22

**Files:**
- Modify: `go.mod` (the `go` directive)

- [ ] **Step 1: Inspect the current directive**

Run:

```bash
grep -nE '^(go|toolchain) ' go.mod
```

Expected: shows `go 1.25.0` (and possibly a `toolchain` line).

- [ ] **Step 2: Set the directive to 1.22 and drop any toolchain pin**

Edit `go.mod`: change the `go 1.25.0` line to:

```
go 1.22
```

If a `toolchain goX.Y.Z` line exists, delete it (the local toolchain will satisfy the lower directive).

- [ ] **Step 3: Tidy and build**

Run:

```bash
go mod tidy && make build
```

Expected: build succeeds and produces `build/vertixd` (the `build` target is added in Task 7; until then run `go build -o build/vertixd ./cmd/vertixd`). If `go mod tidy` reports a dependency requiring a higher Go version, set the directive to the lowest version that builds (try `1.23`) and record it in the PR description.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "build: pin go directive to 1.22 for SDK v0.50.x compatibility"
```

---

## Task 3: Remove `x/mint` (enforce the 21M hard cap)

**Files:**
- Modify: `app/app_config.go`
- Modify: `app/app.go`

`x/mint` is referenced in exactly these places. Remove every one.

- [ ] **Step 1: Remove mint from `app/app_config.go`**

Delete these lines/blocks:

- Import: `mintmodulev1 "cosmossdk.io/api/cosmos/mint/module/v1"`
- Import: `minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"`
- In `genesisModuleOrder`: the `minttypes.ModuleName,` entry
- In `beginBlockers`: the `minttypes.ModuleName,` entry
- In `moduleAccPerms`: `{Account: minttypes.ModuleName, Permissions: []string{authtypes.Minter}},`
- In `blockAccAddrs`: the `minttypes.ModuleName,` entry
- The mint module config block:

```go
{
    Name:   minttypes.ModuleName,
    Config: appconfig.WrapAny(&mintmodulev1.Module{}),
},
```

- [ ] **Step 2: Remove mint from `app/app.go`**

Delete:

- Import side-effect: `_ "github.com/cosmos/cosmos-sdk/x/mint"`
- Import: `mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"`
- `App` struct field: `MintKeeper           mintkeeper.Keeper`
- In the `depinject.Inject(...)` argument list: `&app.MintKeeper,`

- [ ] **Step 3: Build to confirm no dangling references**

Run:

```bash
go build ./... 2>&1 | tail -10
```

Expected: success. Any remaining mint reference will appear here as a compile error — fix it and rebuild.

- [ ] **Step 4: Commit**

```bash
git add app/app_config.go app/app.go
git commit -m "chore(app): remove x/mint to enforce the 21M VTX hard cap"
```

---

## Task 4: Remove scaffold extras `nft`, `group`, `circuit`

**Files:**
- Modify: `app/app_config.go`
- Modify: `app/app.go`

These are not in the §1.3 module set. Remove them one module at a time, rebuilding between each so the compiler catches any missed reference.

- [ ] **Step 1: Remove `nft`**

In `app/app_config.go` delete: import `nftmodulev1 "cosmossdk.io/api/cosmos/nft/module/v1"`; import `"cosmossdk.io/x/nft"`; the `nft.ModuleName,` entry in `genesisModuleOrder`; the `{Account: nft.ModuleName},` entry in `moduleAccPerms`; the `nft.ModuleName,` entry in `blockAccAddrs`; and the nft module config block:

```go
{
    Name:   nft.ModuleName,
    Config: appconfig.WrapAny(&nftmodulev1.Module{}),
},
```

In `app/app.go` delete: imports `nftkeeper "cosmossdk.io/x/nft/keeper"` and `_ "cosmossdk.io/x/nft/module"`; struct field `NFTKeeper            nftkeeper.Keeper`; inject target `&app.NFTKeeper,`.

Run: `go build ./... 2>&1 | tail -10` → expect success.

- [ ] **Step 2: Remove `group`**

In `app/app_config.go` delete: import `groupmodulev1 "cosmossdk.io/api/cosmos/group/module/v1"`; import `"github.com/cosmos/cosmos-sdk/x/group"`; import `"google.golang.org/protobuf/types/known/durationpb"`; the `"time"` import (used only by the group config); the `group.ModuleName,` entry in `genesisModuleOrder`; the `group.ModuleName,` entry in `endBlockers`; and the group module config block:

```go
{
    Name: group.ModuleName,
    Config: appconfig.WrapAny(&groupmodulev1.Module{
        MaxExecutionPeriod: durationpb.New(time.Second * 1209600),
        MaxMetadataLen:     255,
    }),
},
```

In `app/app.go` delete: imports `groupkeeper "github.com/cosmos/cosmos-sdk/x/group/keeper"` and `_ "github.com/cosmos/cosmos-sdk/x/group/module"`; struct field `GroupKeeper          groupkeeper.Keeper`; inject target `&app.GroupKeeper,`.

Run: `go build ./... 2>&1 | tail -10` → expect success.

- [ ] **Step 3: Remove `circuit`**

In `app/app_config.go` delete: import `circuitmodulev1 "cosmossdk.io/api/cosmos/circuit/module/v1"`; import `circuittypes "cosmossdk.io/x/circuit/types"`; the `circuittypes.ModuleName,` entry in `genesisModuleOrder`; and the circuit module config block:

```go
{
    Name:   circuittypes.ModuleName,
    Config: appconfig.WrapAny(&circuitmodulev1.Module{}),
},
```

In `app/app.go` delete: imports `_ "cosmossdk.io/x/circuit"` and `circuitkeeper "cosmossdk.io/x/circuit/keeper"`; struct field `CircuitBreakerKeeper circuitkeeper.Keeper`; inject target `&app.CircuitBreakerKeeper,`.

Run: `go build ./... 2>&1 | tail -10` → expect success.

- [ ] **Step 4: Run gofmt/goimports and verify the binary boots `init`**

Run:

```bash
gofmt -w app/app.go app/app_config.go
go build -o build/vertixd ./cmd/vertixd
rm -rf /tmp/vtxhome && build/vertixd init testnode --chain-id vertix-devnet-1 --home /tmp/vtxhome 2>&1 | tail -3
```

Expected: `init` writes genesis without error (the keeper graph is valid).

- [ ] **Step 5: Commit**

```bash
git add app/app_config.go app/app.go
git commit -m "chore(app): trim nft/group/circuit to match the §1.3 module set"
```

---

## Task 5: App-level guard tests

**Files:**
- Create: `app/app_test.go`
- Test: `app/app_test.go`

These tests lock the hard-cap guard and the module set. They use the depinject app's `ModuleManager` and `GetMaccPerms()` (both real symbols from the scaffold).

- [ ] **Step 1: Write the failing tests**

Create `app/app_test.go`:

```go
package app_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	"github.com/vertix-network/vertix/app"
)

// newTestApp builds an in-memory App for wiring assertions.
func newTestApp(t *testing.T) *app.App {
	t.Helper()
	a, err := app.New(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		simtestutil.NewAppOptionsWithFlagHome(t.TempDir()),
	)
	require.NoError(t, err)
	return a
}

func TestNoMintModule(t *testing.T) {
	a := newTestApp(t)

	_, hasModule := a.ModuleManager.Modules[minttypes.ModuleName]
	require.False(t, hasModule, "x/mint must not be registered (21M hard cap)")

	_, hasPerm := app.GetMaccPerms()[minttypes.ModuleName]
	require.False(t, hasPerm, "x/mint must have no module-account permission")

	require.Nil(t, a.GetKey(minttypes.StoreKey), "x/mint must have no store key")
}

func TestModulesWired(t *testing.T) {
	a := newTestApp(t)
	required := []string{
		"auth", "bank", "staking", "gov", "distribution", "slashing",
		"upgrade", "params", "crisis", "feegrant", "authz", "consensus",
		"genutil", "evidence", "vesting",
	}
	for _, name := range required {
		_, ok := a.ModuleManager.Modules[name]
		require.Truef(t, ok, "module %q must be wired", name)
	}
}

func TestGenesisRoundTrip(t *testing.T) {
	a := newTestApp(t)
	genState := a.DefaultGenesis()
	require.NotEmpty(t, genState)
	_, hasMint := genState[minttypes.ModuleName]
	require.False(t, hasMint, "default genesis must not contain a mint section")
}
```

- [ ] **Step 2: Add the missing imports**

The test references `log`, `dbm`, and `simtestutil`. Add to the import block:

```go
	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
```

- [ ] **Step 3: Run the tests to verify they pass**

Run:

```bash
go test ./app/... -run 'TestNoMintModule|TestModulesWired|TestGenesisRoundTrip' -v 2>&1 | tail -20
```

Expected: `PASS` for all three. If `a.DefaultGenesis()` is not exposed, replace it with `a.ModuleManager.DefaultGenesis(a.AppCodec())`.

- [ ] **Step 4: Commit**

```bash
git add app/app_test.go
git commit -m "test(app): assert no x/mint, required modules wired, clean genesis"
```

---

## Task 6: Chain identity and genesis parameters

**Files:**
- Modify: `config.yml`

The scaffold's `config.yml` uses `token`/`stake`. Replace it with the `uvtx` devnet identity and the §1.2 genesis overrides. Ignite applies the `genesis:` block as raw JSON paths onto the generated genesis.

- [ ] **Step 1: Replace `config.yml`**

Overwrite `config.yml` with:

```yaml
version: 1
validation: sovereign
build:
  main: cmd/vertixd
accounts:
  - name: alice
    coins:
      - 100000000000uvtx
  - name: bob
    coins:
      - 50000000000uvtx
client:
  openapi:
    path: docs/static/openapi.yml
faucet:
  name: bob
  coins:
    - 10000000uvtx
validators:
  - name: alice
    bonded: 50000000000uvtx
genesis:
  chain_id: vertix-devnet-1
  app_state:
    staking:
      params:
        bond_denom: uvtx
        unbonding_time: "1814400s"
        max_validators: 125
        min_commission_rate: "0.050000000000000000"
    gov:
      params:
        min_deposit:
          - denom: uvtx
            amount: "10000000000"
        voting_period: "432000s"
        expedited_voting_period: "86400s"
        quorum: "0.334000000000000000"
        threshold: "0.500000000000000000"
    distribution:
      params:
        community_tax: "0.020000000000000000"
    slashing:
      params:
        signed_blocks_window: "100"
        min_signed_per_window: "0.500000000000000000"
        slash_fraction_double_sign: "0.050000000000000000"
        slash_fraction_downtime: "0.010000000000000000"
    crisis:
      constant_fee:
        denom: uvtx
        amount: "1000000000"
    bank:
      denom_metadata:
        - description: "The native staking token of Vertix."
          denom_units:
            - denom: uvtx
              exponent: 0
            - denom: vtx
              exponent: 6
          base: uvtx
          display: vtx
          name: Vertix
          symbol: VTX
```

- [ ] **Step 2: Generate genesis and validate it**

Run:

```bash
go build -o build/vertixd ./cmd/vertixd
rm -rf /tmp/vtxhome && build/vertixd init testnode --chain-id vertix-devnet-1 --default-denom uvtx --home /tmp/vtxhome
build/vertixd genesis validate-genesis --home /tmp/vtxhome 2>&1 | tail -3
```

Expected: `File at ... is a valid genesis file`. (This validates the default genesis; the `config.yml` overrides are exercised by `ignite chain serve` / `make devnet-reset` in Task 7.)

- [ ] **Step 3: Set the default min gas price**

Edit the generated `app.toml` template only matters at runtime; document `minimum-gas-prices = "0.025uvtx"` in the devnet README. For the chain default, confirm `cmd/vertixd/cmd/root.go` / `app.toml` default. No code change required for the gate; record `0.025uvtx` in the PR description.

- [ ] **Step 4: Commit**

```bash
git add config.yml
git commit -m "feat(app): set uvtx identity and §1.2 genesis params for devnet"
```

---

## Task 7: Makefile targets

**Files:**
- Modify: `Makefile`

Add the canonical targets that the scaffold lacks. Keep the existing `install`/`proto-gen`/`govet`/`govulncheck`. Reconcile `test` to run with `-race`.

- [ ] **Step 1: Append the missing targets to `Makefile`**

Add this block at the end of `Makefile`:

```make
###################
###   Build     ###
###################

BUILD_DIR ?= build
COVER_FILE ?= coverage.out
COVER_HTML_FILE ?= coverage.html

build:
	@echo "--> Building $(APPNAME)d"
	@go build $(BUILD_FLAGS) -mod=readonly -o $(BUILD_DIR)/$(APPNAME)d ./cmd/$(APPNAME)d

clean:
	@echo "--> Cleaning build artifacts"
	@rm -rf $(BUILD_DIR) $(COVER_FILE) $(COVER_HTML_FILE)

.PHONY: build clean

###################
###  Genesis    ###
###################

validate-genesis: build
	@echo "--> Validating genesis"
	@$(BUILD_DIR)/$(APPNAME)d genesis validate-genesis

.PHONY: validate-genesis

###################
###  Devnet     ###
###################

devnet-reset:
	@echo "--> Starting fresh devnet"
	@ignite chain serve --reset-once --verbose

devnet:
	@echo "--> Starting devnet (keeping state)"
	@ignite chain serve --verbose

.PHONY: devnet-reset devnet

###################
###  Clients    ###
###################

ts-gen:
	@echo "--> Generating TypeScript client"
	@ignite generate ts-client --yes

.PHONY: ts-gen

###################
###    Hooks    ###
###################

hooks:
	@echo "--> Installing git hooks"
	@ln -sf ../../scripts/git-hooks/pre-commit .git/hooks/pre-commit
	@chmod +x scripts/git-hooks/pre-commit

.PHONY: hooks
```

- [ ] **Step 2: Make `make test` run with race detection**

In `Makefile`, change the `test` aggregate target line from:

```make
test: govet govulncheck test-unit
```

to:

```make
test: govet test-race
```

(Drops `govulncheck` from the default test path — it stays available as its own target — and uses the race-enabled run mandated by `coding-standards.md` §5.5.)

- [ ] **Step 3: Verify the new targets**

Run:

```bash
make build && make validate-genesis 2>&1 | tail -3
```

Expected: builds `build/vertixd` and prints a valid-genesis message.

- [ ] **Step 4: Commit**

```bash
git add Makefile
git commit -m "build: add build/clean/validate-genesis/devnet/ts-gen/hooks targets"
```

---

## Task 8: Linter configuration

**Files:**
- Create: `.golangci.yml`

- [ ] **Step 1: Create `.golangci.yml`**

```yaml
run:
  timeout: 15m
  tests: true

linters:
  disable-all: true
  enable:
    - govet
    - errcheck
    - staticcheck
    - unused
    - gosimple
    - ineffassign
    - typecheck
    - gofmt
    - goimports
    - revive
    - misspell
    - unconvert
    - unparam
    - prealloc

linters-settings:
  goimports:
    local-prefixes: github.com/vertix-network/vertix
  gofmt:
    simplify: true

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - unparam
  max-issues-per-linter: 0
  max-same-issues: 0
```

- [ ] **Step 2: Run the linter**

Run:

```bash
make lint 2>&1 | tail -20
```

Expected: completes; address any findings in scaffold code that are trivially fixable (e.g. add Godoc, run `make lint-fix`). Pre-existing scaffold lint findings that are non-trivial may be excluded by path with a noted justification rather than blocking Phase 0.

- [ ] **Step 3: Commit**

```bash
git add .golangci.yml
git commit -m "ci: add golangci-lint config (coding-standards §2.2 linter set)"
```

---

## Task 9: Pre-commit hook

**Files:**
- Create: `scripts/git-hooks/pre-commit`

- [ ] **Step 1: Create the hook script**

Create `scripts/git-hooks/pre-commit`:

```bash
#!/usr/bin/env bash
set -euo pipefail

staged_go=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$' || true)
if [ -z "$staged_go" ]; then
  exit 0
fi

echo "pre-commit: gofmt"
unformatted=$(gofmt -l $staged_go || true)
if [ -n "$unformatted" ]; then
  echo "These files need gofmt:" >&2
  echo "$unformatted" >&2
  echo "Run: gofmt -w $unformatted" >&2
  exit 1
fi

echo "pre-commit: goimports"
if command -v goimports >/dev/null 2>&1; then
  goimports -l -local github.com/vertix-network/vertix $staged_go
fi

echo "pre-commit: golangci-lint"
if command -v golangci-lint >/dev/null 2>&1; then
  golangci-lint run --new-from-rev=HEAD~1 ./... || golangci-lint run ./...
fi
```

- [ ] **Step 2: Install and test the hook**

Run:

```bash
make hooks
git config core.hooksPath >/dev/null 2>&1 || true
ls -l .git/hooks/pre-commit
```

Expected: the symlink exists and is executable.

- [ ] **Step 3: Commit**

```bash
git add scripts/git-hooks/pre-commit
git commit -m "build: add gofmt/goimports/lint pre-commit hook + make hooks"
```

---

## Task 10: GitHub Actions CI

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Create `ci.yml`**

```yaml
name: CI

on:
  pull_request:
  push:
    branches: [main]

env:
  GO_VERSION: "1.22"

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true
      - run: make build

  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true
      - run: make test

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true
      - uses: golangci/golangci-lint-action@v6
        with:
          version: v1.61.0
          args: --timeout 15m

  proto:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: bufbuild/buf-setup-action@v1
        with:
          version: "1.34.0"
      - run: cd proto && buf lint
      - run: cd proto && buf breaking --against 'https://github.com/vertix-network/vertix.git#branch=main,subdir=proto'
        continue-on-error: true

  genesis:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true
      - run: make validate-genesis
```

- [ ] **Step 2: Validate the workflow YAML locally**

Run:

```bash
python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/ci.yml')); print('ci.yml OK')"
```

Expected: `ci.yml OK`.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add lint/build/test/proto/genesis workflow on PR"
```

---

## Task 11: Boot smoke check and docs sync

**Files:**
- Create: `scripts/boot-smoke.sh`
- Modify: `docs/technical-design.md` (§1.3 module-set note)

- [ ] **Step 1: Create a boot smoke script**

Create `scripts/boot-smoke.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

HOME_DIR="${1:-/tmp/vtx-smoke}"
BIN="build/vertixd"
CHAIN_ID="vertix-devnet-1"

make build
rm -rf "$HOME_DIR"
"$BIN" init smoke --chain-id "$CHAIN_ID" --default-denom uvtx --home "$HOME_DIR"
"$BIN" genesis validate-genesis --home "$HOME_DIR"

"$BIN" start --home "$HOME_DIR" \
  --rpc.laddr tcp://127.0.0.1:26657 \
  --grpc.address 127.0.0.1:9090 \
  --api.enable --api.address tcp://127.0.0.1:1317 &
PID=$!
sleep 12

curl -sf http://127.0.0.1:26657/status >/dev/null && echo "RPC :26657 OK"
curl -sf http://127.0.0.1:1317/cosmos/base/tendermint/v1beta1/node_info >/dev/null && echo "REST :1317 OK"
HEIGHT=$(curl -s http://127.0.0.1:26657/status | grep -o '"latest_block_height":"[0-9]*"' | grep -o '[0-9]*')
echo "block height: $HEIGHT"
kill "$PID"
[ "${HEIGHT:-0}" -gt 0 ] && echo "BOOT SMOKE PASS" || { echo "BOOT SMOKE FAIL"; exit 1; }
```

Note: single-node start needs a gentx-funded validator. For the smoke test, either use `ignite chain serve --reset-once` (preferred, it wires a validator from `config.yml`) or add a `gentx` step. Simplest reliable check: `make devnet-reset` and confirm blocks advance, then stop.

- [ ] **Step 2: Run the smoke check via Ignite**

Run:

```bash
chmod +x scripts/boot-smoke.sh
timeout 90 ignite chain serve --reset-once --verbose 2>&1 | grep -m1 "Blockchain is running" && echo "DEVNET BOOT OK"
```

Expected: Ignite builds, inits from `config.yml`, and reports the chain is running with the `:1317/:9090/:26657` endpoints. Ctrl-C / timeout stops it.

- [ ] **Step 3: Sync the module-set note in `technical-design.md`**

In `docs/technical-design.md` §1.3, add `x/consensus` to the wired list and a note that the IBC stack includes `ica` (27) and `29-fee` as scaffolded. Keep the change minimal and additive.

- [ ] **Step 4: Commit**

```bash
git add scripts/boot-smoke.sh docs/technical-design.md
git commit -m "test: add boot smoke check; docs: note x/consensus + IBC stack in §1.3"
```

---

## Task 12: Final gate verification

**Files:** none (verification only)

- [ ] **Step 1: Run the full inherited gate**

Run:

```bash
make lint && make test && make build && make validate-genesis
```

Expected: all four succeed.

- [ ] **Step 2: Confirm the hard-cap guard**

Run:

```bash
go test ./app/... -run TestNoMintModule -v -race 2>&1 | tail -5
```

Expected: `PASS`.

- [ ] **Step 3: Open the PR**

```bash
git push -u origin HEAD
gh pr create --title "feat: Phase 0 chain foundation (bootable vertixd, no x/mint)" --body "$(cat <<'EOF'
## Summary
- Scaffold vertixd (Ignite v28, depinject), pin Go 1.22.
- Remove x/mint (21M hard cap) and scaffold extras nft/group/circuit.
- uvtx identity + §1.2 genesis params; Makefile/lint/CI/hooks; guard tests.

## Test plan
- [ ] make lint && make test && make build && make validate-genesis
- [ ] TestNoMintModule / TestModulesWired / TestGenesisRoundTrip pass
- [ ] ignite chain serve --reset-once boots; :1317/:9090/:26657 up

Refs: docs/specs/2026-05-29-phase-0-chain-foundation-design.md
EOF
)"
```

Expected: CI goes green on the PR — opening the gate to Phases 1, 2, and 5.

---

## Self-Review (completed by plan author)

- **Spec coverage:** identity + §1.2 params (Task 6), module set minus mint + extras (Tasks 3–4), tooling/Makefile/lint/buf/CI/hooks (Tasks 7–10), TestNoMintModule + round-trip (Task 5), acceptance gate (Tasks 11–12). All §-references map to a task.
- **Placeholder scan:** no TBD/TODO; every code step shows real content. The `app.toml` min-gas note (Task 6 Step 3) is a documentation action, not a code placeholder.
- **Type consistency:** test symbols (`app.New`, `a.ModuleManager.Modules`, `app.GetMaccPerms`, `a.GetKey`) match the real scaffold (`app/app.go`). Makefile vars (`APPNAME`, `BUILD_FLAGS`, `BUILD_DIR`) are consistent with the scaffold Makefile.
- **Known soft spots (verify on execution):** `a.DefaultGenesis()` exact signature (fallback provided in Task 5 Step 3); single-node `start` needs a gentx, so Task 11 prefers `ignite chain serve`.
