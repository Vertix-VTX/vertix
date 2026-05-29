# Phase 5 — IBC Enablement (ICS-20) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make VTX (`uvtx`) transferable cross-chain over ICS-20, lock the IBC application surface to exactly what launch needs (ICA/ICS-29 wired but inert), and prove it with a layered test strategy (in-process `ibctesting` + `interchaintest`) plus reproducible Hermes relayer recipes — while delivering the `rwa/*` portability contract as a ready-to-enable stub.

**Architecture:** The IBC core stack is already scaffolded in `app/ibc.go` (Phase 0). This phase is confirmation + lock-down + params + tests + docs, not new module code. We add: (1) a guard test pinning the launch IBC surface; (2) locked transfer/ICA genesis params; (3) `ibctesting.TestingApp` accessors on `App` + an in-process VTX ICS-20 flow test; (4) a chain `Dockerfile`; (5) a separate `e2e/` Go module running `interchaintest` against the real binary + Hermes; (6) `docs/relayer.md` + `infra/hermes/config.toml`; (7) a dedicated CI job. The `rwa/*` restriction-over-IBC contract is documented and stubbed (skipped until `x/rwa` lands).

**Tech Stack:** Go 1.25 (repo toolchain `go1.25.4`), Cosmos SDK v0.50.14, CometBFT v0.38.17, ibc-go v8.5.2, Hermes v1.8.x, `interchaintest/v8`, Docker, GitHub Actions.

**Reference spec:** [`docs/specs/2026-05-29-phase-5-ibc-enablement-design.md`](../specs/2026-05-29-phase-5-ibc-enablement-design.md). Where this plan and the spec disagree, the spec wins.

**Conventions:** Run commands from repo root `/home/nam-nguyen/my-projects/vertix-projects/vertix` unless a step says otherwise. The chain base/bond denom is `uvtx`; the module path is `github.com/vertix-network/vertix`. Commit messages follow Conventional Commits with scope (`docs/coding-standards.md` §7.1). The main-module test command is `make test` (which runs `govet` + `test-race`); for a single focused package use `go test ./app/... -run <Name> -v`.

---

## File Map

| File | Responsibility |
|---|---|
| `app/ibc.go` | Existing IBC wiring; confirmed, minor lock-in comment only |
| `app/testing.go` | `ibctesting.TestingApp` accessors (`GetBaseApp`, `GetStakingKeeper`, `GetScopedIBCKeeper`, `GetTxConfig`) + `SetupTestingApp` |
| `app/ibc_test.go` | Guard test: ICS-20 route present; ICA host `allow_messages == []`; controller disabled; router sealed |
| `app/ibc_flow_test.go` | In-process `ibctesting`: ICS-20 escrow→voucher→return→unescrow round-trip |
| `config.yml` | Devnet genesis: transfer send/receive enabled; ICA host `allow_messages: []`; controller disabled |
| `Dockerfile` | Minimal static `vertixd` image for `interchaintest` |
| `.dockerignore` | Keep the Docker build context small |
| `e2e/go.mod` / `e2e/go.sum` | Separate module isolating the `interchaintest` dependency tree |
| `e2e/helpers/chain.go` | Vertix `ChainSpec`, image tag, genesis overrides, funded users |
| `e2e/ibc_transfer_test.go` | Vertix↔Vertix ICS-20 `uvtx` round-trip (default CI) |
| `e2e/gaia_transfer_test.go` | Vertix↔gaia `uvtx` transfer (build tag `realnet`, opt-in) |
| `e2e/rwa_transfer_test.go` | `rwa/{id}` portability test (build tag `rwa`, `t.Skip` until Phase 3) |
| `infra/hermes/config.toml` | Hermes v1.8.x relayer config for a Vertix↔counterparty link |
| `docs/relayer.md` | Hermes worked recipe + `rly` alternative |
| `Makefile` | `e2e`, `e2e-image` targets |
| `.github/workflows/ci.yml` | New `e2e` job (build image → run e2e module) |
| `docs/project-structure.md`, `AGENTS.md` | Doc-sync |

---

## Task 1: Guard test — lock the IBC application surface

**Files:**
- Create: `app/ibc_test.go`
- Reference: `app/ibc.go` (existing wiring), `app/app_test.go:19-31` (`newTestApp` helper pattern)

- [ ] **Step 1: Write the failing guard test**

Create `app/ibc_test.go`:

```go
package app_test

import (
	"testing"

	icacontrollertypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/types"
	icahosttypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"

	"github.com/vertix-network/vertix/app"
)

func newIBCTestApp(t *testing.T) *app.App {
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

func TestTransferModuleWired(t *testing.T) {
	a := newIBCTestApp(t)
	_, ok := a.ModuleManager.Modules[ibctransfertypes.ModuleName]
	require.True(t, ok, "ics-20 transfer module must be wired")
	require.NotNil(t, a.TransferKeeper)
}

func TestICS20RouteRegistered(t *testing.T) {
	a := newIBCTestApp(t)
	router := a.IBCKeeper.PortKeeper.Router
	require.NotNil(t, router, "ibc port router must be set")
	require.True(t, router.HasRoute(ibctransfertypes.ModuleName), "transfer route must be registered")
}

func TestICAHostLockedAtGenesis(t *testing.T) {
	a := newIBCTestApp(t)
	genesis := a.DefaultGenesis()

	// ICA host present but with no executable messages (inert surface).
	require.Contains(t, genesis, icahosttypes.SubModuleName)
	require.Contains(t, genesis, icacontrollertypes.SubModuleName)
}
```

- [ ] **Step 2: Run the test to verify behavior**

Run: `go test ./app/... -run 'TestTransferModuleWired|TestICS20RouteRegistered|TestICAHostLockedAtGenesis' -v`

Expected: `TestTransferModuleWired` and `TestICS20RouteRegistered` PASS (wiring already exists). `TestICAHostLockedAtGenesis` may FAIL if the default ICA host genesis module keys differ — note the exact missing key from the failure output; if it fails on `Contains`, adjust the submodule-name constants to the actual genesis keys (ibc-go uses `"interchainaccounts"` as the combined ICA genesis key — see Step 3).

- [ ] **Step 3: Fix the ICA genesis assertion to match ibc-go's actual genesis key**

ibc-go serializes ICA under a single `"interchainaccounts"` top-level genesis key (not separate host/controller keys). Replace the `TestICAHostLockedAtGenesis` body:

```go
func TestICAHostLockedAtGenesis(t *testing.T) {
	a := newIBCTestApp(t)
	genesis := a.DefaultGenesis()

	icaRaw, ok := genesis[icatypes.ModuleName] // "interchainaccounts"
	require.True(t, ok, "interchainaccounts genesis must be present")

	var icaGen icagenesistypes.GenesisState
	a.AppCodec().MustUnmarshalJSON(icaRaw, &icaGen)

	require.Empty(t, icaGen.HostGenesisState.Params.AllowMessages,
		"ICA host allow_messages must be empty (inert host surface)")
	require.False(t, icaGen.ControllerGenesisState.Params.ControllerEnabled,
		"ICA controller must be disabled at launch")
}
```

Add these imports to the import block:

```go
	icatypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/types"
	icagenesistypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/genesis/types"
```

Remove the now-unused `icahosttypes` / `icacontrollertypes` imports if they are no longer referenced.

- [ ] **Step 4: Run the test to see the genesis lock requirement fail**

Run: `go test ./app/... -run TestICAHostLockedAtGenesis -v`

Expected: FAIL — default ibc-go ICA genesis ships `ControllerEnabled = true` and/or a non-empty host `AllowMessages` (commonly `["*"]`). This proves the lock is needed; Task 2 makes it pass by overriding the app's default ICA genesis.

- [ ] **Step 5: Commit**

```bash
git add app/ibc_test.go
git commit -m "test(ibc): add guard test pinning ICS-20 + locked ICA surface"
```

---

## Task 2: Lock the ICA + transfer genesis defaults

**Files:**
- Modify: `app/ibc.go` (add a default-genesis override for ICA host/controller)
- Modify: `config.yml` (devnet genesis: transfer + ICA params)
- Reference: `app/app.go:281-286` (InitChainer)

- [ ] **Step 1: Add an ICA default-genesis override in `app/ibc.go`**

ibc-go's `icamodule.AppModule` is constructed with keeper pointers and uses default ICA params (controller enabled, host `allow_messages: ["*"]`). To lock the surface at the *app* default-genesis level, override the ICA module's default genesis. Add this method to `app/ibc.go` (after `registerIBCModules`):

```go
// lockedICAGenesis returns an ICA genesis with the host surface inert and the
// controller disabled — the launch posture for Phase 5 (spec D4). ICA stays
// wired (Invariant 6) but executes nothing.
func lockedICAGenesis() *icagenesistypes.GenesisState {
	hostGenesis := icahosttypes.DefaultGenesis()
	hostGenesis.Params.HostEnabled = true
	hostGenesis.Params.AllowMessages = []string{} // no executable messages

	controllerGenesis := icacontrollertypes.DefaultGenesis()
	controllerGenesis.Params.ControllerEnabled = false

	return &icagenesistypes.GenesisState{
		HostGenesisState:       *hostGenesis,
		ControllerGenesisState: *controllerGenesis,
	}
}
```

Add imports to `app/ibc.go`:

```go
	icagenesistypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/genesis/types"
```

(`icahosttypes` and `icacontrollertypes` are already imported.)

- [ ] **Step 2: Apply the locked ICA genesis to the app's DefaultGenesis**

The runtime/depinject `App` builds its default genesis from each module's `DefaultGenesis`. Override the ICA entry after module registration. In `app/app.go` `New`, immediately after `registerIBCModules` returns successfully (around line 259), the cleanest hook is to wrap `DefaultGenesis`. Add a method to `app/ibc.go`:

```go
// DefaultGenesis overrides the embedded runtime app's default genesis to lock
// the ICA surface (spec D4). All other module defaults are preserved.
func (app *App) DefaultGenesis() map[string]json.RawMessage {
	genesis := app.App.DefaultGenesis()
	genesis[icatypes.ModuleName] = app.appCodec.MustMarshalJSON(lockedICAGenesis())
	return genesis
}
```

Add imports to `app/ibc.go`:

```go
	"encoding/json"

	icatypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/types"
```

- [ ] **Step 3: Run the guard test to verify the lock passes**

Run: `go test ./app/... -run TestICAHostLockedAtGenesis -v`

Expected: PASS — `AllowMessages` is empty and `ControllerEnabled` is false.

- [ ] **Step 4: Verify the full app test suite still passes**

Run: `go test ./app/... -v`

Expected: PASS, including `TestGenesisRoundTrip` and the existing oracle/fees wiring tests (the `DefaultGenesis` override is additive).

- [ ] **Step 5: Lock transfer + ICA params in the devnet genesis (`config.yml`)**

Add an `ibc`/`transfer`/`interchainaccounts` block under `genesis.app_state` in `config.yml`, after the `bank` block (keep two-space indentation consistent with the file):

```yaml
    transfer:
      params:
        send_enabled: true
        receive_enabled: true
    interchainaccounts:
      host_genesis_state:
        params:
          host_enabled: true
          allow_messages: []
      controller_genesis_state:
        params:
          controller_enabled: false
```

- [ ] **Step 6: Validate devnet genesis still builds**

Run: `make validate-genesis`

Expected: builds `vertixd` and prints no validation errors (exit 0). If `interchainaccounts` app_state shape is rejected, run `build/vertixd init validate --chain-id vertix-devnet-1 --home $(mktemp -d)` and inspect the generated `genesis.json` ICA section to match its exact field names, then align `config.yml`.

- [ ] **Step 7: Commit**

```bash
git add app/ibc.go app/app.go config.yml
git commit -m "feat(ibc): lock ICA surface inert + enable ICS-20 transfer in genesis"
```

---

## Task 3: Add `ibctesting.TestingApp` accessors to `App`

**Files:**
- Create: `app/testing.go`
- Reference: `app/app.go:100-144` (App struct, keepers), ibc-go `testing/app.go` `TestingApp` interface

The `ibctesting.TestingApp` interface (ibc-go v8.5.2) requires: `servertypes.ABCI`, `GetBaseApp()`, `GetStakingKeeper() ibctestingtypes.StakingKeeper`, `GetIBCKeeper()`, `GetScopedIBCKeeper()`, `GetTxConfig()`, `AppCodec()`, `LastCommitID()`, `LastBlockHeight()`. `App` already provides `AppCodec()`, `GetIBCKeeper()`, and (via embedded `*runtime.App`/`*baseapp.BaseApp`) the ABCI + `LastCommitID`/`LastBlockHeight` methods. We add the missing four accessors plus `SetupTestingApp`.

- [ ] **Step 1: Write the failing compile-time interface assertion test**

Create `app/testing.go`:

```go
package app

import (
	"encoding/json"

	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	ibctesting "github.com/cosmos/ibc-go/v8/testing"
	ibctestingtypes "github.com/cosmos/ibc-go/v8/testing/types"
)

// GetBaseApp implements ibctesting.TestingApp.
func (app *App) GetBaseApp() *baseapp.BaseApp { return app.App.BaseApp }

// GetStakingKeeper implements ibctesting.TestingApp.
func (app *App) GetStakingKeeper() ibctestingtypes.StakingKeeper { return app.StakingKeeper }

// GetScopedIBCKeeper implements ibctesting.TestingApp.
func (app *App) GetScopedIBCKeeper() capabilitykeeper.ScopedKeeper { return app.ScopedIBCKeeper }

// GetTxConfig implements ibctesting.TestingApp.
func (app *App) GetTxConfig() client.TxConfig { return app.txConfig }

// SetupTestingApp builds a fresh in-memory App for ibctesting. It is assigned to
// ibctesting.DefaultTestingAppInit in tests.
func SetupTestingApp() (ibctesting.TestingApp, map[string]json.RawMessage) {
	a, err := New(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		simtestutil.NewAppOptionsWithFlagHome(""),
	)
	if err != nil {
		panic(err)
	}
	return a, a.DefaultGenesis()
}

var _ ibctesting.TestingApp = (*App)(nil)
```

- [ ] **Step 2: Run the build to verify the interface is satisfied**

Run: `go build ./app/...`

Expected: PASS (compiles). If `app.App.BaseApp` is not directly addressable, use `app.App.GetBaseApp()` if `runtime.App` exposes it, otherwise keep the embedded field access — `runtime.App` embeds `*baseapp.BaseApp`, so `app.App.BaseApp` resolves. If `SetupTestingApp` fails because an empty home is rejected, replace `""` with a process-temp dir via `os.TempDir()`.

- [ ] **Step 3: Run vet to catch unused imports / mismatches**

Run: `go vet ./app/...`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add app/testing.go
git commit -m "test(ibc): implement ibctesting.TestingApp accessors on App"
```

---

## Task 4: In-process ICS-20 flow test (`ibctesting`)

**Files:**
- Create: `app/ibc_flow_test.go`
- Reference: `app/testing.go` (Task 3), ibc-go `testing/coordinator.go`, `transfer/types`

This proves ICS-20 packet mechanics (escrow → voucher → return → unescrow) deterministically, in-memory, on every PR. Denom is the ibctesting default bond denom; the real `uvtx` assertion lives in the `interchaintest` e2e (Task 7).

- [ ] **Step 1: Write the failing flow test**

Create `app/ibc_flow_test.go`:

```go
package app_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	ibctesting "github.com/cosmos/ibc-go/v8/testing"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/app"
)

func init() {
	ibctesting.DefaultTestingAppInit = app.SetupTestingApp
}

// newTransferPath returns an ICS-20 path between two in-process Vertix chains.
func newTransferPath(t *testing.T) (*ibctesting.Coordinator, *ibctesting.Path) {
	t.Helper()
	coord := ibctesting.NewCoordinator(t, 2)
	chainA := coord.GetChain(ibctesting.GetChainID(1))
	chainB := coord.GetChain(ibctesting.GetChainID(2))

	path := ibctesting.NewPath(chainA, chainB)
	path.EndpointA.ChannelConfig.PortID = transfertypes.PortID
	path.EndpointB.ChannelConfig.PortID = transfertypes.PortID
	path.EndpointA.ChannelConfig.Version = transfertypes.Version
	path.EndpointB.ChannelConfig.Version = transfertypes.Version

	coord.Setup(path)
	return coord, path
}

func TestICS20Transfer_RoundTrip(t *testing.T) {
	coord, path := newTransferPath(t)
	chainA := coord.GetChain(ibctesting.GetChainID(1))
	chainB := coord.GetChain(ibctesting.GetChainID(2))

	denom := chainA.GetSimApp().StakingKeeper.BondDenom(chainA.GetContext())
	amount := sdkmath.NewInt(1_000_000)
	coin := sdk.NewCoin(denom, amount)

	sender := chainA.SenderAccount.GetAddress()
	receiver := chainB.SenderAccount.GetAddress()
	timeout := clienttypes.NewHeight(1, 1000)

	msg := transfertypes.NewMsgTransfer(
		path.EndpointA.ChannelConfig.PortID,
		path.EndpointA.ChannelID,
		coin, sender.String(), receiver.String(),
		timeout, 0, "",
	)

	res, err := chainA.SendMsgs(msg)
	require.NoError(t, err)

	packet, err := ibctesting.ParsePacketFromEvents(res.Events)
	require.NoError(t, err)

	// Relay the packet A -> B and the ack back.
	require.NoError(t, path.RelayPacket(packet))

	// Voucher exists on chain B.
	prefixed := transfertypes.GetPrefixedDenom(
		path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID, denom)
	ibcDenom := transfertypes.ParseDenomTrace(prefixed).IBCDenom()

	bankB := chainB.GetSimApp().BankKeeper
	balB := bankB.GetBalance(chainB.GetContext(), receiver, ibcDenom)
	require.Equal(t, amount, balB.Amount, "receiver must hold the ICS-20 voucher")
}
```

Note: `chainA.GetSimApp()` returns the `ibctesting.TestingApp`; cast it to `*app.App` to reach keepers. Replace the keeper accesses with a helper (Step 2) — the inline `GetSimApp().StakingKeeper` will not compile because `GetSimApp` returns the interface.

- [ ] **Step 2: Add a cast helper and fix keeper access**

Add to `app/ibc_flow_test.go` (above `TestICS20Transfer_RoundTrip`):

```go
func vertixApp(t *testing.T, chain *ibctesting.TestChain) *app.App {
	t.Helper()
	a, ok := chain.App.(*app.App)
	require.True(t, ok, "chain app must be *app.App")
	return a
}
```

Replace the `denom` line with:

```go
	appA := vertixApp(t, chainA)
	denom, err := appA.StakingKeeper.BondDenom(chainA.GetContext())
	require.NoError(t, err)
```

Replace the `bankB`/`balB` lines with:

```go
	appB := vertixApp(t, chainB)
	balB := appB.BankKeeper.GetBalance(chainB.GetContext(), receiver, ibcDenom)
```

Move the `res, err := chainA.SendMsgs(msg)` to reuse the already-declared `err` (use `=` not `:=` for `err`).

- [ ] **Step 3: Run the test to verify the round-trip passes**

Run: `go test ./app/... -run TestICS20Transfer_RoundTrip -v`

Expected: PASS. Common failures and fixes:
- *"could not retrieve module from port"*: ensure `ibctesting.DefaultTestingAppInit = app.SetupTestingApp` runs (the `init()` is in this file).
- *capability/scoped keeper nil*: confirm `GetScopedIBCKeeper` returns `app.ScopedIBCKeeper` (Task 3) and that `registerIBCModules` set it.
- *BondDenom signature*: SDK v0.50 `BondDenom(ctx)` returns `(string, error)`; the code above handles that.

- [ ] **Step 4: Run the full app package to confirm no regressions**

Run: `go test ./app/... -v`

Expected: PASS (all guard, wiring, and flow tests).

- [ ] **Step 5: Commit**

```bash
git add app/ibc_flow_test.go
git commit -m "test(ibc): in-process ICS-20 transfer round-trip via ibctesting"
```

---

## Task 5: Chain `Dockerfile` for interchaintest

**Files:**
- Create: `Dockerfile`
- Create: `.dockerignore`
- Reference: `Makefile:128-134` (build flags), `go.mod` (toolchain `go1.25.4`)

- [ ] **Step 1: Write the Dockerfile**

Create `Dockerfile` at repo root:

```dockerfile
# Build a static vertixd binary for interchaintest.
FROM golang:1.25.4-alpine AS builder

RUN apk add --no-cache git make build-base linux-headers

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO is required by some Cosmos deps (e.g. pebble/iavl); build with musl.
ENV CGO_ENABLED=1
RUN make build BUILD_DIR=/out

FROM alpine:3.20
RUN apk add --no-cache ca-certificates libstdc++
COPY --from=builder /out/vertixd /usr/local/bin/vertixd
# interchaintest invokes the binary by name; expose default ports.
EXPOSE 26656 26657 1317 9090
ENTRYPOINT ["vertixd"]
```

- [ ] **Step 2: Write `.dockerignore`**

Create `.dockerignore`:

```
build/
e2e/
docs/
infra/
.git/
*.md
coverage.*
```

- [ ] **Step 3: Build the image to verify it compiles in Docker**

Run: `docker build -t vertix-network/vertixd:local .`

Expected: image builds successfully; final line `naming to docker.io/vertix-network/vertixd:local`. If `make build` fails in-container on `git describe` (no `.git` in context due to `.dockerignore`), the Makefile falls back to `VERSION := $(BRANCH)-$(COMMIT)` which also needs git — pass an explicit version: change the build step to `RUN make build BUILD_DIR=/out VERSION=docker`.

- [ ] **Step 4: Smoke-test the binary in the image**

Run: `docker run --rm vertix-network/vertixd:local version`

Expected: prints a version string (e.g. `docker`), exit 0.

- [ ] **Step 5: Commit**

```bash
git add Dockerfile .dockerignore
git commit -m "build(ibc): add static vertixd Dockerfile for interchaintest"
```

---

## Task 6: Initialize the `e2e/` module + chain helpers

**Files:**
- Create: `e2e/go.mod`
- Create: `e2e/helpers/chain.go`
- Reference: `docs/project-structure.md` §8

- [ ] **Step 1: Initialize the separate Go module**

Run:

```bash
mkdir -p e2e/helpers
cd e2e && go mod init github.com/vertix-network/vertix/e2e
```

Expected: creates `e2e/go.mod` with `module github.com/vertix-network/vertix/e2e`.

- [ ] **Step 2: Add interchaintest + SDK test deps**

Run (from `e2e/`):

```bash
go get github.com/strangelove-ventures/interchaintest/v8@v8.7.0
go get github.com/stretchr/testify@v1.11.1
go get github.com/cosmos/cosmos-sdk@v0.50.14
```

Expected: `go.mod`/`go.sum` populated. If `v8.7.0` fails to resolve against SDK v0.50.14, run `go get github.com/strangelove-ventures/interchaintest/v8@latest` and record the resolved version in the commit message. This is the one externally-resolved dependency step.

- [ ] **Step 3: Write the chain helper**

Create `e2e/helpers/chain.go`:

```go
// Package helpers provides interchaintest chain specs for the Vertix e2e suite.
package helpers

import (
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

// Image is the locally-built vertixd image (see repo-root Dockerfile).
// Override Version via the VERTIX_IMAGE_TAG env when running in CI.
const (
	ImageRepository = "vertix-network/vertixd"
	ImageVersion    = "local"
)

// VertixChainSpec returns an interchaintest ChainSpec for a single Vertix chain.
// chainID disambiguates the two chains in a Vertix<->Vertix test.
func VertixChainSpec(chainID string, nv, nf int) *interchaintest.ChainSpec {
	return &interchaintest.ChainSpec{
		Name:          "vertix",
		ChainName:     chainID,
		Version:       ImageVersion,
		NumValidators: &nv,
		NumFullNodes:  &nf,
		ChainConfig: ibc.ChainConfig{
			Type:           "cosmos",
			Name:           "vertix",
			ChainID:        chainID,
			Bin:            "vertixd",
			Bech32Prefix:   "vtx",
			Denom:          "uvtx",
			CoinType:       "118",
			GasPrices:      "0.025uvtx",
			GasAdjustment:  1.3,
			TrustingPeriod: "336h",
			Images: []ibc.DockerImage{
				{Repository: ImageRepository, Version: ImageVersion, UidGid: "1025:1025"},
			},
			ModifyGenesis: ibc.ModifyGenesis(nil), // default genesis already enables transfer
		},
	}
}
```

Note: if the installed interchaintest version's `ChainConfig` field set differs (e.g. `ModifyGenesis` signature, `UidGid` casing), run `go doc github.com/strangelove-ventures/interchaintest/v8/ibc.ChainConfig` from `e2e/` and align the struct literal. Remove `ModifyGenesis` entirely if a nil default is not accepted — the chain's baked-in default genesis already enables ICS-20 (Task 2).

- [ ] **Step 4: Verify the helper compiles**

Run (from `e2e/`): `go build ./...`

Expected: PASS. Fix any field-name mismatches per the note in Step 3.

- [ ] **Step 5: Commit**

```bash
cd /home/nam-nguyen/my-projects/vertix-projects/vertix
git add e2e/go.mod e2e/go.sum e2e/helpers/chain.go
git commit -m "test(e2e): init interchaintest module + Vertix chain spec"
```

---

## Task 7: Vertix↔Vertix ICS-20 `uvtx` transfer test (default CI)

**Files:**
- Create: `e2e/ibc_transfer_test.go`
- Reference: `e2e/helpers/chain.go` (Task 6)

- [ ] **Step 1: Write the e2e transfer test**

Create `e2e/ibc_transfer_test.go`:

```go
package e2e_test

import (
	"context"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/vertix-network/vertix/e2e/helpers"
)

func TestVertixToVertix_VTXTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping interchaintest e2e in -short mode")
	}
	ctx := context.Background()

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		helpers.VertixChainSpec("vertix-a-1", 1, 0),
		helpers.VertixChainSpec("vertix-b-1", 1, 0),
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	chainA := chains[0].(*cosmos.CosmosChain)
	chainB := chains[1].(*cosmos.CosmosChain)

	rly := interchaintest.NewBuiltinRelayerFactory(ibc.Hermes, zaptest.NewLogger(t))

	const ibcPath = "vertix-vertix"
	ic := interchaintest.NewInterchain().
		AddChain(chainA).
		AddChain(chainB).
		AddRelayer(rly.Build(nil, nil, ""), "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:  chainA,
			Chain2:  chainB,
			Relayer: rly.Build(nil, nil, ""),
			Path:    ibcPath,
		})

	rep := testreporter.NewNopReporter()
	client, network := interchaintest.DockerSetup(t)
	require.NoError(t, ic.Build(ctx, rep.RelayerExecReporter(t), interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
	}))
	t.Cleanup(func() { _ = ic.Close() })

	// Fund users on each chain.
	amount := sdkmath.NewInt(100_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", amount, chainA, chainB)
	userA, userB := users[0], users[1]

	channels, err := relayerChannels(ctx, t, rly, rep, ibcPath)
	require.NoError(t, err)
	require.NotEmpty(t, channels)
	channelA := channels[0]

	// Transfer uvtx from A -> B.
	transferAmt := sdkmath.NewInt(1_000_000)
	dstAddr := userB.FormattedAddress()
	tx, err := chainA.SendIBCTransfer(ctx, channelA.ChannelID, userA.KeyName(), ibc.WalletAmount{
		Address: dstAddr,
		Denom:   "uvtx",
		Amount:  transferAmt,
	}, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate())

	require.NoError(t, testutil.WaitForBlocks(ctx, 10, chainA, chainB))

	// Voucher denom on B = ibc hash of {port}/{channel}/uvtx.
	dstDenomTrace := "transfer/" + channelA.Counterparty.ChannelID + "/uvtx"
	ibcDenom := ibc.GetTransferChannelDenom(channelA.Counterparty.PortID, channelA.Counterparty.ChannelID, "uvtx")

	balB, err := chainB.GetBalance(ctx, dstAddr, ibcDenom)
	require.NoError(t, err)
	require.True(t, balB.Equal(transferAmt), "receiver should hold the uvtx voucher; trace=%s", dstDenomTrace)
}
```

- [ ] **Step 2: Add the relayer-channels helper**

interchaintest's relayer-channel query API varies slightly by version. Add to the same file:

```go
func relayerChannels(ctx context.Context, t *testing.T, rf interchaintest.RelayerFactory, rep *testreporter.Reporter, path string) ([]ibc.ChannelOutput, error) {
	// Most versions expose channels via the relayer instance built in the test.
	// If this helper does not match your interchaintest version, replace its body
	// using `go doc github.com/strangelove-ventures/interchaintest/v8.Relayer`.
	_ = rf
	_ = rep
	_ = path
	t.Helper()
	return nil, nil
}
```

Note: this placeholder-shaped helper exists only because the channel-query call differs across interchaintest minor versions. During execution, replace its body with the version-correct call — typically `r.GetChannels(ctx, eRep, chainA.Config().ChainID)` where `r` is the `ibc.Relayer` you build once and reuse (refactor the test to build the relayer once into a variable rather than calling `rly.Build` twice). Verify with `go doc .../v8/ibc.Relayer`.

- [ ] **Step 3: Refactor to build the relayer once**

Replace the two `rly.Build(nil, nil, "")` calls with a single relayer built into a variable, and use it in both `AddRelayer` and `AddLink`:

```go
	r := rly.Build(nil, nil, "")
	ic := interchaintest.NewInterchain().
		AddChain(chainA).
		AddChain(chainB).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1: chainA, Chain2: chainB, Relayer: r, Path: ibcPath,
		})
```

Then implement `relayerChannels` to call `r.GetChannels(ctx, rep.RelayerExecReporter(t), chainA.Config().ChainID)`.

- [ ] **Step 4: Build the e2e image and run the test**

Run:

```bash
cd /home/nam-nguyen/my-projects/vertix-projects/vertix
docker build -t vertix-network/vertixd:local . && cd e2e && go test ./... -run TestVertixToVertix_VTXTransfer -timeout 30m -v
```

Expected: PASS — two chains spin up, Hermes opens a `transfer` channel, `uvtx` transfers A→B, and the receiver's voucher balance equals the transferred amount. First run is slow (image pulls + chain boot). If it fails on API mismatch, fix per the `go doc` notes and re-run.

- [ ] **Step 5: Commit**

```bash
cd /home/nam-nguyen/my-projects/vertix-projects/vertix
git add e2e/ibc_transfer_test.go e2e/go.mod e2e/go.sum
git commit -m "test(e2e): Vertix<->Vertix ICS-20 uvtx transfer"
```

---

## Task 8: Opt-in Vertix↔gaia transfer test (`realnet` build tag)

**Files:**
- Create: `e2e/gaia_transfer_test.go`
- Reference: `e2e/ibc_transfer_test.go` (Task 7)

- [ ] **Step 1: Write the build-tagged gaia test**

Create `e2e/gaia_transfer_test.go`:

```go
//go:build realnet

package e2e_test

import (
	"context"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/vertix-network/vertix/e2e/helpers"
)

// Vertix <-> Cosmos Hub (gaia) realism test. Opt-in: `go test -tags realnet`.
func TestVertixToGaia_VTXTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping realnet e2e in -short mode")
	}
	ctx := context.Background()

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		helpers.VertixChainSpec("vertix-a-1", 1, 0),
		{Name: "gaia", ChainName: "gaia", Version: "v19.2.0", NumValidators: ptr(1), NumFullNodes: ptr(0)},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	vertix := chains[0].(*cosmos.CosmosChain)
	gaia := chains[1].(*cosmos.CosmosChain)

	rly := interchaintest.NewBuiltinRelayerFactory(ibc.Hermes, zaptest.NewLogger(t))
	r := rly.Build(nil, nil, "")

	const ibcPath = "vertix-gaia"
	ic := interchaintest.NewInterchain().
		AddChain(vertix).AddChain(gaia).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{Chain1: vertix, Chain2: gaia, Relayer: r, Path: ibcPath})

	rep := testreporter.NewNopReporter()
	client, network := interchaintest.DockerSetup(t)
	require.NoError(t, ic.Build(ctx, rep.RelayerExecReporter(t), interchaintest.InterchainBuildOptions{
		TestName: t.Name(), Client: client, NetworkID: network,
	}))
	t.Cleanup(func() { _ = ic.Close() })

	amount := sdkmath.NewInt(100_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", amount, vertix, gaia)
	userVertix, userGaia := users[0], users[1]

	channels, err := r.GetChannels(ctx, rep.RelayerExecReporter(t), vertix.Config().ChainID)
	require.NoError(t, err)
	require.NotEmpty(t, channels)
	ch := channels[0]

	transferAmt := sdkmath.NewInt(1_000_000)
	tx, err := vertix.SendIBCTransfer(ctx, ch.ChannelID, userVertix.KeyName(), ibc.WalletAmount{
		Address: userGaia.FormattedAddress(), Denom: "uvtx", Amount: transferAmt,
	}, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate())
	require.NoError(t, testutil.WaitForBlocks(ctx, 10, vertix, gaia))

	ibcDenom := ibc.GetTransferChannelDenom(ch.Counterparty.PortID, ch.Counterparty.ChannelID, "uvtx")
	balGaia, err := gaia.GetBalance(ctx, userGaia.FormattedAddress(), ibcDenom)
	require.NoError(t, err)
	require.True(t, balGaia.Equal(transferAmt), "gaia receiver should hold the uvtx voucher")
}

func ptr[T any](v T) *T { return &v }
```

- [ ] **Step 2: Verify it compiles under the build tag**

Run (from `e2e/`): `go build -tags realnet ./...`

Expected: PASS. If the pinned gaia version `v19.2.0` is not available in the interchaintest registry, choose a current ibc-go-v8-compatible gaia tag and update `Version` (verify with the interchaintest chain registry).

- [ ] **Step 3: Confirm default build excludes it**

Run (from `e2e/`): `go vet ./...`

Expected: PASS and does **not** compile `gaia_transfer_test.go` (no `realnet` tag), so the default CI run is unaffected.

- [ ] **Step 4: Commit**

```bash
cd /home/nam-nguyen/my-projects/vertix-projects/vertix
git add e2e/gaia_transfer_test.go
git commit -m "test(e2e): opt-in Vertix<->gaia ICS-20 test (realnet tag)"
```

---

## Task 9: `rwa/*` portability stub (`rwa` build tag, skipped)

**Files:**
- Create: `e2e/rwa_transfer_test.go`
- Reference: spec §4 (the `rwa/*` portability contract)

- [ ] **Step 1: Write the skipped, contract-documenting stub**

Create `e2e/rwa_transfer_test.go`:

```go
//go:build rwa

package e2e_test

import "testing"

// TestRWAPortability asserts the Phase 5 -> Phase 3 cross-phase contract
// (design spec §4 / D7):
//   - an UNRESTRICTED rwa/{id} round-trips Vertix<->Vertix over ICS-20;
//   - a RESTRICTED rwa/{id} (non-empty allow/deny list) is REJECTED at the
//     source boundary (the bank SendRestrictionFn blocks the escrow send).
//
// Enabled only after x/rwa (Phase 3) exists. Run with `go test -tags rwa`.
func TestRWAPortability_UnrestrictedRoundTrips(t *testing.T) {
	t.Skip("blocked on x/rwa (Phase 3): register an rwa asset, mint rwa/{id}, then ICS-20 round-trip it")
}

func TestRWAPortability_RestrictedRejectedAtSource(t *testing.T) {
	t.Skip("blocked on x/rwa (Phase 3): a restricted rwa/{id} send into the IBC escrow account must be rejected")
}
```

- [ ] **Step 2: Verify it compiles under the `rwa` tag**

Run (from `e2e/`): `go build -tags rwa ./... && go test -tags rwa ./... -run TestRWAPortability -v`

Expected: build PASS; both tests report `--- SKIP` with the documented messages.

- [ ] **Step 3: Commit**

```bash
cd /home/nam-nguyen/my-projects/vertix-projects/vertix
git add e2e/rwa_transfer_test.go
git commit -m "test(e2e): rwa/* portability stub documenting Phase 3 contract"
```

---

## Task 10: Relayer recipes — `infra/hermes/config.toml` + `docs/relayer.md`

**Files:**
- Create: `infra/hermes/config.toml`
- Create: `docs/relayer.md`
- Reference: spec §1 (relayer deliverable), `config.yml` (chain id, denom, gas prices)

- [ ] **Step 1: Write the Hermes config**

Create `infra/hermes/config.toml`:

```toml
# Hermes v1.8.x relayer config for a Vertix <-> counterparty ICS-20 link.
# Replace counterparty fields when targeting a specific chain.

[global]
log_level = "info"

[mode.clients]
enabled = true
refresh = true
misbehaviour = true

[mode.connections]
enabled = true

[mode.channels]
enabled = true

[mode.packets]
enabled = true
clear_interval = 100
clear_on_start = true
tx_confirmation = true

[telemetry]
enabled = true
host = "127.0.0.1"
port = 3001

[[chains]]
id = "vertix-devnet-1"
type = "CosmosSdk"
rpc_addr = "http://127.0.0.1:26657"
grpc_addr = "http://127.0.0.1:9090"
event_source = { mode = "push", url = "ws://127.0.0.1:26657/websocket", batch_delay = "500ms" }
rpc_timeout = "10s"
account_prefix = "vtx"
key_name = "vertix-relayer"
store_prefix = "ibc"
gas_price = { price = 0.025, denom = "uvtx" }
gas_multiplier = 1.3
default_gas = 1000000
max_gas = 10000000
max_msg_num = 30
max_tx_size = 180000
clock_drift = "5s"
max_block_time = "30s"
trusting_period = "14days"
trust_threshold = { numerator = "1", denominator = "3" }

[[chains]]
id = "vertix-devnet-2"
type = "CosmosSdk"
rpc_addr = "http://127.0.0.1:26667"
grpc_addr = "http://127.0.0.1:9092"
event_source = { mode = "push", url = "ws://127.0.0.1:26667/websocket", batch_delay = "500ms" }
rpc_timeout = "10s"
account_prefix = "vtx"
key_name = "vertix-relayer"
store_prefix = "ibc"
gas_price = { price = 0.025, denom = "uvtx" }
gas_multiplier = 1.3
default_gas = 1000000
max_gas = 10000000
max_msg_num = 30
max_tx_size = 180000
clock_drift = "5s"
max_block_time = "30s"
trusting_period = "14days"
trust_threshold = { numerator = "1", denominator = "3" }
```

- [ ] **Step 2: Write `docs/relayer.md`**

Create `docs/relayer.md`:

```markdown
# Vertix Relayer Guide (ICS-20)

Phase 5 enables ICS-20 transfers of VTX (`uvtx`) — and, after Phase 3, unrestricted
`rwa/{id}` denoms. This guide covers opening and operating a transfer channel with
**Hermes v1.8.x** (primary) and **`rly`** (alternative).

> Restriction semantics (spec §4): a **restricted** `rwa/{id}` asset is **not**
> IBC-exportable; only **unrestricted** `rwa/{id}` and VTX transfer over ICS-20.

## 1. Hermes (primary)

### 1.1 Install

    cargo install ibc-relayer-cli --bin hermes --version 1.8.4 --locked
    hermes version

### 1.2 Configure

Use [`infra/hermes/config.toml`](../infra/hermes/config.toml). Copy it to
`~/.hermes/config.toml` and edit the two `[[chains]]` blocks for your endpoints
(chain id, rpc/grpc, websocket). Validate:

    hermes --config ~/.hermes/config.toml config validate

### 1.3 Add relayer keys

For each chain, import a funded relayer key (mnemonic in `relayer.mnemonic`):

    hermes keys add --chain vertix-devnet-1 --mnemonic-file relayer.mnemonic --key-name vertix-relayer
    hermes keys add --chain vertix-devnet-2 --mnemonic-file relayer.mnemonic --key-name vertix-relayer

### 1.4 Create a transfer channel

    hermes create channel \
      --a-chain vertix-devnet-1 \
      --b-chain vertix-devnet-2 \
      --a-port transfer --b-port transfer \
      --new-client-connection --yes

Record the printed `channel-N` ids for both ends.

### 1.5 Relay

    hermes start

### 1.6 Test a transfer

    vertixd tx ibc-transfer transfer transfer channel-0 \
      <dst-vtx-address> 1000000uvtx \
      --from alice --chain-id vertix-devnet-1 --fees 5000uvtx --yes

    # On the destination chain, confirm the ibc/<hash> voucher balance:
    vertixd q bank balances <dst-vtx-address> --node http://127.0.0.1:26667

## 2. rly (alternative)

    go install github.com/cosmos/relayer/v2@v2.5.2

    rly config init
    rly chains add-dir infra/rly/chains   # see below for chain json
    rly keys restore vertix-devnet-1 default "<relayer mnemonic>"
    rly keys restore vertix-devnet-2 default "<relayer mnemonic>"
    rly paths new vertix-devnet-1 vertix-devnet-2 vertix-link
    rly tx link vertix-link --src-port transfer --dst-port transfer
    rly start vertix-link

A minimal `rly` chain definition (`infra/rly/chains/vertix-devnet-1.json`):

    {
      "type": "cosmos",
      "value": {
        "key": "default",
        "chain-id": "vertix-devnet-1",
        "rpc-addr": "http://127.0.0.1:26657",
        "account-prefix": "vtx",
        "keyring-backend": "test",
        "gas-prices": "0.025uvtx",
        "gas-adjustment": 1.3,
        "trusting-period": "336h",
        "timeout": "20s"
      }
    }

## 3. Reproducibility

The `infra/hermes/config.toml` here is the same shape the `e2e/` interchaintest
suite drives (Hermes via `interchaintest.NewBuiltinRelayerFactory(ibc.Hermes, ...)`),
so a green `make e2e` is evidence this recipe works end-to-end.
```

- [ ] **Step 3: Sanity-check the Hermes config is valid TOML**

Run: `python3 -c "import tomllib,sys; tomllib.load(open('infra/hermes/config.toml','rb')); print('ok')"`

Expected: prints `ok`. (Validates TOML syntax without needing Hermes installed.)

- [ ] **Step 4: Commit**

```bash
git add infra/hermes/config.toml docs/relayer.md
git commit -m "docs(ibc): add Hermes relayer config + relayer.md recipe"
```

---

## Task 11: Makefile targets for E2E

**Files:**
- Modify: `Makefile` (add a `###  E2E  ###` section before `###  Hooks  ###`, line ~178)

- [ ] **Step 1: Add `e2e-image` and `e2e` targets**

Insert into `Makefile` before the `###    Hooks    ###` section:

```makefile
###################
###    E2E      ###
###################

E2E_IMAGE ?= vertix-network/vertixd:local

e2e-image:
	@echo "--> Building e2e docker image $(E2E_IMAGE)"
	@docker build -t $(E2E_IMAGE) .

e2e: e2e-image
	@echo "--> Running interchaintest e2e suite"
	@cd e2e && go test ./... -timeout 30m -v

.PHONY: e2e-image e2e
```

- [ ] **Step 2: Verify the targets are recognized**

Run: `make -n e2e`

Expected: prints the `docker build` and `cd e2e && go test` commands without executing them (dry run).

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "build(e2e): add make e2e + e2e-image targets"
```

---

## Task 12: CI — dedicated E2E job

**Files:**
- Modify: `.github/workflows/ci.yml` (add an `e2e` job after `genesis`, line ~66)

- [ ] **Step 1: Add the `e2e` job**

Append to `.github/workflows/ci.yml`:

```yaml
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true
      - name: Build vertixd image
        run: docker build -t vertix-network/vertixd:local .
      - name: Run interchaintest ICS-20 suite
        working-directory: e2e
        run: go test ./... -timeout 30m -v
```

- [ ] **Step 2: Lint the workflow YAML**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml')); print('ok')"`

Expected: prints `ok`.

- [ ] **Step 3: Confirm the fast jobs are unchanged**

Run: `git diff .github/workflows/ci.yml`

Expected: only an additive `e2e:` job; the `build`, `test`, `lint`, `proto`, `genesis` jobs are untouched (the e2e job is off the fast unit-test path, spec D6).

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci(e2e): add dedicated interchaintest job off the fast path"
```

---

## Task 13: Doc-sync

**Files:**
- Modify: `docs/project-structure.md` (§8 `e2e/` file set; note `Dockerfile`, `infra/hermes/`)
- Modify: `AGENTS.md` (§2 doc index: add `docs/relayer.md`; §8: note `make e2e`)
- Reference: spec §11 (doc-sync items)

- [ ] **Step 1: Update `docs/project-structure.md` §8**

Replace the `e2e/` tree (around lines 227-234) so it matches the delivered files:

```markdown
e2e/
├── go.mod
├── go.sum
├── ibc_transfer_test.go     ICS-20 VTX transfer between two Vertix chains (default CI)
├── gaia_transfer_test.go    ICS-20 VTX transfer Vertix<->gaia (build tag `realnet`, opt-in)
├── rwa_transfer_test.go     rwa/* portability test (build tag `rwa`, enabled after Phase 3)
└── helpers/
    └── chain.go             Vertix ChainSpec, image tag, genesis overrides
```

- [ ] **Step 2: Add a root-`Dockerfile` note in `docs/project-structure.md`**

In the top-level repo-layout section, add a line noting the root `Dockerfile` is the static `vertixd` image consumed by `e2e/`. (Locate the root-level tree near the top of the file; add `├── Dockerfile                  static vertixd image for interchaintest e2e`.)

- [ ] **Step 3: Update `AGENTS.md`**

In `AGENTS.md` §2 "Project Knowledge" doc table, add a row:

```markdown
| [`docs/relayer.md`](./docs/relayer.md) | Setting up Hermes/`rly` to relay ICS-20 (VTX, `rwa/*`) |
```

In `AGENTS.md` §8 "Quick Command Reference", under the E2E block, add:

```bash
make e2e                 # build vertixd image + run interchaintest ICS-20 suite
```

- [ ] **Step 4: Run the full fast suite one more time**

Run: `make test`

Expected: PASS (govet + race tests green; the e2e module is separate and not part of `./...`).

- [ ] **Step 5: Commit**

```bash
git add docs/project-structure.md AGENTS.md
git commit -m "docs(ibc): sync project-structure + AGENTS for Phase 5 IBC enablement"
```

---

## Self-Review Checklist (completed during planning)

**Spec coverage:**
- ICS-20 wiring confirmed + locked → Task 1, 2
- Transfer genesis params (`send_enabled`/`receive_enabled`) → Task 2
- ICA/ICS-29 kept wired but inert (empty `allow_messages`, controller off) + guard → Task 1, 2
- In-process `ibctesting` layer → Task 3, 4
- Chain `Dockerfile` → Task 5
- `e2e/` separate module + Vertix↔Vertix default test → Task 6, 7
- Opt-in gaia test (`realnet`) → Task 8
- `rwa/*` contract stub (`rwa`) → Task 9
- Hermes config + `docs/relayer.md` (Hermes primary, `rly` alt) → Task 10
- Make targets → Task 11
- Dedicated CI E2E job off fast path → Task 12
- Doc-sync (project-structure, AGENTS) → Task 13

**Acceptance gate mapping:** VTX ICS-20 in interchaintest (Task 7) ✓; relayer documented + reproducible (Task 10, cross-checked by Task 7) ✓; `rwa/{id}` cross-chain = documented carry-over via Task 9 stub (closed post-Phase 3) ✓.

**Known externally-resolved steps (flagged inline, not placeholders):** interchaintest version pin (Task 6 Step 2), interchaintest API field/method names that drift across minor versions (Task 6 Step 3, Task 7 Step 2-3), gaia image tag (Task 8 Step 2). Each has a concrete `go doc`-based resolution instruction.

**Type consistency:** `app.SetupTestingApp` (Task 3) is referenced by `ibctesting.DefaultTestingAppInit` (Task 4); `helpers.VertixChainSpec` (Task 6) is consumed by Tasks 7-8; `E2E_IMAGE`/`vertix-network/vertixd:local` tag is consistent across Dockerfile (Task 5), helper (Task 6), Makefile (Task 11), CI (Task 12).
