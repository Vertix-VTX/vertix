# Phase 6 — Devnet (Multi-Validator Stack) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up a reproducible 3-validator `docker compose` devnet for `vertix-devnet-1` — with one `vertix-feeder` per validator, a live gaia + Hermes IBC link, a Ping.pub explorer, a faucet, and minimal Prometheus/Grafana — plus a deterministic seed/reset flow and an automated end-to-end smoke gate that proves oracle → rwa → fees → IBC.

**Architecture:** A single Docker image (`vertix:devnet`) carries both `vertixd` and `vertix-feeder`. A one-shot init container runs `scripts/devnet/init-genesis.sh` to build a deterministic genesis (fixed mnemonics + fixed node/consensus keys) into a shared volume; three validator containers boot from it; three feeder containers authorize themselves via `MsgSetFeeder` and submit prices. A gaia node + Hermes relayer open one ICS-20 channel. Explorer, faucet, Prometheus, and Grafana attach as supporting services. `scripts/devnet/smoke.sh` drives the full protocol and asserts on-chain state.

**Tech Stack:** Docker Compose, `vertixd` (Cosmos SDK v0.50.x), `vertix-feeder`, gaia `v19.2.0` (heighliner image), Hermes `v1.8.x`, Ping.pub explorer, cosmjs/cosmfaucet, Prometheus, Grafana, `bash` + `jq` + `curl` for scripts.

**Design spec:** [`docs/specs/2026-05-29-phase-6-devnet-design.md`](../specs/2026-05-29-phase-6-devnet-design.md). Decisions D1–D10 referenced throughout.

---

## Conventions used by every task

- **Repo root** is the working directory unless a step says otherwise.
- **Module path:** `github.com/vertix-network/vertix`. **Bech32:** `vtx` / `vtxvaloper`. **Denom:** `uvtx` (6 decimals). **Chain ID:** `vertix-devnet-1`. **gaia chain ID:** `gaia-devnet-1`.
- **CLI surface (confirmed via autocli):**
  - `vertixd tx oracle set-feeder [feeder] --from <operator-key>` (operator signs; delegates feeder acct)
  - `vertixd tx oracle submit-feed [validator] [pair] [price] --from <feeder-key>`
  - `vertixd query oracle price [pair]` · `query oracle feeder [validator]`
  - `vertixd tx rwa register-asset [asset-id] [name] [oracle-pair] [bond] --from <key>`
  - `vertixd tx rwa attest-asset|mint-rwa|transfer-rwa|settle-rwa ...`
  - `vertixd query rwa asset [asset-id]`
- **Ports (host):** val1 RPC `26657`, gRPC `9090`, REST `1317`, CometBFT Prometheus `26660`; feeders `9201/9202/9203` → container `9200`; gaia RPC `26757`; Hermes telemetry `3001`; explorer `8080`; faucet `8000`; Prometheus `9095`; Grafana `3000`.
- **Pinned image tags** live only in `infra/devnet/.env` so they are swappable.
- **No business-logic changes.** This phase only adds `infra/`, `scripts/devnet/`, docs, and Makefile/`.gitignore` edits. If a step reveals a module bug, STOP and raise it — do not patch `x/*` or `app/` here.
- **Commit cadence:** one commit per task (the final step of each task). Conventional Commits, scope `devnet`.

---

## File Structure

**Created:**
- `infra/devnet/.env` — pinned image tags, chain IDs, ports.
- `infra/devnet/mnemonics.env` — fixed devnet-only mnemonics (NEVER mainnet).
- `infra/devnet/Dockerfile` — multi-stage build → `vertix:devnet` (both binaries).
- `infra/devnet/docker-compose.yml` — full stack.
- `infra/devnet/keys/validator{1,2,3}/{node_key.json,priv_validator_key.json}` — fixed node + consensus keys.
- `infra/devnet/feeder/feeder{1,2,3}.yaml` — per-validator feeder config.
- `infra/devnet/gaia/init-gaia.sh` — gaia single-node genesis + relayer funding.
- `infra/hermes/config.toml` — Hermes config (vertix ↔ gaia).
- `infra/explorer/chains/vertix.json` — Ping.pub chain definition.
- `infra/monitoring/prometheus.yml` — scrape config.
- `infra/monitoring/grafana/provisioning/datasources/prometheus.yml` — datasource.
- `infra/monitoring/grafana/provisioning/dashboards/dashboards.yml` — dashboard provider.
- `infra/monitoring/grafana/dashboards/vertix.json` — starter dashboard.
- `scripts/devnet/lib.sh` — shared bash helpers.
- `scripts/devnet/gen-keys.sh` — one-time mnemonic + node-key generator.
- `scripts/devnet/init-genesis.sh` — deterministic genesis build.
- `scripts/devnet/setup-ibc.sh` — Hermes channel bootstrap.
- `scripts/devnet/smoke.sh` — full e2e assertions.
- `docs/devnet.md` — bring-up + human walkthrough.

**Modified:**
- `Makefile` — Docker devnet target family.
- `.gitignore` — ignore generated node data.
- `docs/relayer.md` — add devnet (vertix↔gaia) Hermes recipe (created if absent).
- `AGENTS.md` §8 — add devnet quick commands (update discipline).

---

## Task 1: Devnet scaffolding, pinned versions, and `.gitignore`

**Files:**
- Create: `infra/devnet/.env`
- Modify: `.gitignore`

- [ ] **Step 1: Create `infra/devnet/.env`**

```dotenv
# infra/devnet/.env — pinned tags + ports for the Vertix devnet stack.
# Swap a tag here to upgrade a single component. NEVER used for mainnet.

VERTIX_IMAGE=vertix:devnet
GAIA_IMAGE=ghcr.io/strangelove-ventures/heighliner/gaia:v19.2.0
HERMES_IMAGE=ghcr.io/informalsystems/hermes:1.8.2
EXPLORER_IMAGE=ghcr.io/ping-pub/explorer:latest
FAUCET_IMAGE=ghcr.io/cosmjs/faucet:0.32.4
PROMETHEUS_IMAGE=prom/prometheus:v2.53.0
GRAFANA_IMAGE=grafana/grafana:11.1.0

CHAIN_ID=vertix-devnet-1
GAIA_CHAIN_ID=gaia-devnet-1
DENOM=uvtx
```

> NOTE: `EXPLORER_IMAGE` and `FAUCET_IMAGE` tags are best-effort; Task 9/10 includes a step to verify each image pulls and to substitute a working tag if not. The durable deliverables are the config files, not the tags.

- [ ] **Step 2: Append generated-data ignores to `.gitignore`**

Add these lines to the end of `.gitignore`:

```gitignore
# Phase 6 devnet generated node data (config templates + keys ARE committed)
infra/devnet/data/
infra/devnet/.gen/
```

- [ ] **Step 3: Verify the .env parses**

Run: `set -a && . infra/devnet/.env && set +a && echo "$VERTIX_IMAGE $GAIA_IMAGE $CHAIN_ID"`
Expected: `vertix:devnet ghcr.io/strangelove-ventures/heighliner/gaia:v19.2.0 vertix-devnet-1`

- [ ] **Step 4: Commit**

```bash
git add infra/devnet/.env .gitignore
git commit -m "chore(devnet): add pinned image/version env and gitignore for devnet data"
```

---

## Task 2: Build the `vertix:devnet` image (vertixd + vertix-feeder)

**Files:**
- Create: `infra/devnet/Dockerfile`
- Modify: `Makefile`

- [ ] **Step 1: Create `infra/devnet/Dockerfile`**

```dockerfile
# Builds a single image carrying both vertixd and vertix-feeder for the devnet.
# Mirrors the root Dockerfile toolchain (CGO on, go1.25.4-alpine) and adds the feeder.
FROM golang:1.25.4-alpine AS builder
RUN apk add --no-cache git make build-base linux-headers
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=1
RUN make build BUILD_DIR=/out VERSION=devnet \
 && make feeder-build BUILD_DIR=/out VERSION=devnet

FROM alpine:3.20
RUN apk add --no-cache ca-certificates libstdc++ bash jq curl
COPY --from=builder /out/vertixd /usr/local/bin/vertixd
COPY --from=builder /out/vertix-feeder /usr/local/bin/vertix-feeder
EXPOSE 26656 26657 1317 9090 26660 9200
# No ENTRYPOINT: compose sets the command per service (vertixd or vertix-feeder).
```

- [ ] **Step 2: Add the image-build target to the Makefile**

In `Makefile`, after the `feeder-build` target block (the `.PHONY: build feeder-build clean` line, ~line 144), add:

```makefile
###################
###   Devnet (Docker, Phase 6) ###
###################

DEVNET_DIR ?= infra/devnet
COMPOSE ?= docker compose --env-file $(DEVNET_DIR)/.env -f $(DEVNET_DIR)/docker-compose.yml

devnet-docker-build:
	@echo "--> Building vertix:devnet image"
	@docker build -f $(DEVNET_DIR)/Dockerfile -t vertix:devnet .

.PHONY: devnet-docker-build
```

- [ ] **Step 3: Build the image**

Run: `make devnet-docker-build`
Expected: ends with `naming to docker.io/library/vertix:devnet` (or `Successfully tagged vertix:devnet`); exit 0.

- [ ] **Step 4: Verify both binaries exist in the image**

Run: `docker run --rm vertix:devnet sh -c "vertixd version && vertix-feeder --help | head -1"`
Expected: a version string, then `Vertix oracle price-feed sidecar`.

- [ ] **Step 5: Commit**

```bash
git add infra/devnet/Dockerfile Makefile
git commit -m "feat(devnet): add vertix:devnet image with vertixd + vertix-feeder and build target"
```

---

## Task 3: Shared bash helpers (`lib.sh`)

**Files:**
- Create: `scripts/devnet/lib.sh`

- [ ] **Step 1: Create `scripts/devnet/lib.sh`**

```bash
#!/usr/bin/env bash
# Shared helpers for Vertix devnet scripts. Source, don't execute.
set -euo pipefail

log()  { printf '\033[1;34m[devnet]\033[0m %s\n' "$*" >&2; }
err()  { printf '\033[1;31m[devnet:ERROR]\033[0m %s\n' "$*" >&2; }
die()  { err "$*"; exit 1; }

# require <cmd> ...: fail unless all commands are on PATH.
require() { for c in "$@"; do command -v "$c" >/dev/null 2>&1 || die "missing required tool: $c"; done; }

# wait_http <url> <timeout-sec>: poll until url returns 2xx or timeout.
wait_http() {
  local url=$1 timeout=${2:-60} i=0
  until curl -sf "$url" >/dev/null 2>&1; do
    i=$((i+1)); [ "$i" -ge "$timeout" ] && die "timeout waiting for $url"
    sleep 1
  done
}

# rpc_height <rpc-url>: echo the latest block height, or 0.
rpc_height() {
  curl -s "$1/status" | jq -r '.result.sync_info.latest_block_height // "0"'
}

# wait_height <rpc-url> <min-height> <timeout-sec>
wait_height() {
  local url=$1 min=$2 timeout=${3:-120} i=0 h
  while :; do
    h=$(rpc_height "$url"); h=${h:-0}
    [ "$h" -ge "$min" ] && { log "height $h >= $min at $url"; return 0; }
    i=$((i+1)); [ "$i" -ge "$timeout" ] && die "timeout waiting height>=$min at $url (last=$h)"
    sleep 1
  done
}

# assert_eq <actual> <expected> <message>
assert_eq() { [ "$1" = "$2" ] || die "ASSERT FAILED: $3 (expected '$2', got '$1')"; log "OK: $3"; }
# assert_gt <a> <b> <message>: assert integer a > b
assert_gt() { [ "$(echo "$1 > $2" | bc -l)" = "1" ] || die "ASSERT FAILED: $3 (need $1 > $2)"; log "OK: $3"; }
```

- [ ] **Step 2: Make it executable and lint with bash**

Run: `chmod +x scripts/devnet/lib.sh && bash -n scripts/devnet/lib.sh && echo "lib.sh syntax OK"`
Expected: `lib.sh syntax OK`

- [ ] **Step 3: Commit**

```bash
git add scripts/devnet/lib.sh
git commit -m "feat(devnet): add shared bash helpers for devnet scripts"
```

---

## Task 4: One-time key generation + committed fixed mnemonics/node keys

This produces the deterministic key material (D6). It is run **once** by the implementer; its outputs are committed so every later `reset` reproduces identical addresses and node IDs.

**Files:**
- Create: `scripts/devnet/gen-keys.sh`
- Create (generated, then committed): `infra/devnet/mnemonics.env`, `infra/devnet/keys/validator{1,2,3}/{node_key.json,priv_validator_key.json}`

- [ ] **Step 1: Create `scripts/devnet/gen-keys.sh`**

```bash
#!/usr/bin/env bash
# One-time generator for devnet key material. Run once; commit the outputs.
# Produces infra/devnet/mnemonics.env and fixed node/consensus keys per validator.
set -euo pipefail
cd "$(dirname "$0")/../.."
. scripts/devnet/lib.sh
require docker

OUT_KEYS="infra/devnet/keys"
MNEMO="infra/devnet/mnemonics.env"
NAMES=(val1 val2 val3 feeder1 feeder2 feeder3 faucet relayer testuser)

[ -f "$MNEMO" ] && die "$MNEMO already exists; refusing to overwrite committed keys"

log "Generating mnemonics for: ${NAMES[*]}"
{
  echo "# infra/devnet/mnemonics.env — FIXED DEVNET-ONLY KEY MATERIAL."
  echo "# !!! NEVER fund these on a real network. Generated by scripts/devnet/gen-keys.sh. !!!"
} > "$MNEMO"

for n in "${NAMES[@]}"; do
  m=$(docker run --rm vertix:devnet vertixd keys mnemonic)
  key="MNEMONIC_$(echo "$n" | tr '[:lower:]' '[:upper:]')"
  echo "${key}=\"${m}\"" >> "$MNEMO"
done

log "Generating fixed node_key.json + priv_validator_key.json per validator"
for i in 1 2 3; do
  d="$OUT_KEYS/validator$i"; mkdir -p "$d"
  tmp=$(mktemp -d)
  docker run --rm -v "$tmp:/home/.vertix" vertix:devnet \
    vertixd init "val$i" --chain-id vertix-devnet-1 --default-denom uvtx --home /home/.vertix >/dev/null 2>&1
  cp "$tmp/config/node_key.json" "$d/node_key.json"
  cp "$tmp/config/priv_validator_key.json" "$d/priv_validator_key.json"
  rm -rf "$tmp"
done
log "Done. Review $MNEMO and $OUT_KEYS, then commit."
```

- [ ] **Step 2: Run the generator (requires the Task 2 image)**

Run: `chmod +x scripts/devnet/gen-keys.sh && ./scripts/devnet/gen-keys.sh`
Expected: `Done. Review ...`; `infra/devnet/mnemonics.env` has 9 `MNEMONIC_*` lines; `infra/devnet/keys/validator{1,2,3}/` each contain `node_key.json` + `priv_validator_key.json`.

- [ ] **Step 3: Sanity-check the mnemonics file shape**

Run: `grep -c '^MNEMONIC_' infra/devnet/mnemonics.env`
Expected: `9`

- [ ] **Step 4: Commit (the durable, fixed key material)**

```bash
git add scripts/devnet/gen-keys.sh infra/devnet/mnemonics.env infra/devnet/keys/
git commit -m "feat(devnet): add one-time key generator and committed fixed devnet key material"
```

---

## Task 5: Deterministic genesis builder (`init-genesis.sh`)

Builds the full `vertix-devnet-1` genesis into `infra/devnet/.gen/` (gitignored): recovers all keys into a shared `test` keyring, sets module params (D9 allocation, oracle/fees/rwa params, short gov period), places fixed node/consensus keys, runs `gentx` ×3, collects, and writes per-validator config with computed `persistent_peers`.

**Files:**
- Create: `scripts/devnet/init-genesis.sh`

- [ ] **Step 1: Create `scripts/devnet/init-genesis.sh`**

```bash
#!/usr/bin/env bash
# Deterministic genesis + per-validator config builder for vertix-devnet-1.
# Output: infra/devnet/.gen/{keyring,validator1,validator2,validator3,genesis.json}
set -euo pipefail
cd "$(dirname "$0")/../.."
. scripts/devnet/lib.sh
require docker
set -a; . infra/devnet/.env; . infra/devnet/mnemonics.env; set +a

GEN="infra/devnet/.gen"
KEYS="infra/devnet/keys"
IMG="$VERTIX_IMAGE"
# Run vertixd in the image against a bind-mounted workdir so output lands on host.
vd() { docker run --rm -i -v "$PWD/$GEN:/g" -e HOME=/g "$IMG" vertixd "$@"; }

log "Wiping $GEN"
rm -rf "$GEN"; mkdir -p "$GEN"
KR="/g/keyring"            # shared test keyring inside container
mkdir -p "$GEN/keyring"

recover() { # recover <name> <mnemonic>
  echo "$2" | vd keys add "$1" --recover --keyring-backend test --keyring-dir "$KR" >/dev/null
}
addr()  { vd keys show "$1" -a --keyring-backend test --keyring-dir "$KR"; }
vaddr() { vd keys show "$1" --bech val -a --keyring-backend test --keyring-dir "$KR"; }

log "Recovering keys into shared keyring"
recover val1 "$MNEMONIC_VAL1";       recover val2 "$MNEMONIC_VAL2";       recover val3 "$MNEMONIC_VAL3"
recover feeder1 "$MNEMONIC_FEEDER1"; recover feeder2 "$MNEMONIC_FEEDER2"; recover feeder3 "$MNEMONIC_FEEDER3"
recover faucet "$MNEMONIC_FAUCET";   recover relayer "$MNEMONIC_RELAYER"; recover testuser "$MNEMONIC_TESTUSER"

# --- Build genesis on validator1's home, then fan out ---
V1=/g/validator1
vd init val1 --chain-id "$CHAIN_ID" --default-denom "$DENOM" --home "$V1" >/dev/null 2>&1

# Genesis accounts (D9 simplified allocation; total well under 21,000,000 VTX = 21e12 uvtx).
# Validators self-stake 1,000,000 VTX each; faucet 5,000,000 VTX; feeders/relayer/testuser working balances.
vd genesis add-genesis-account "$(addr val1)"    1000000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr val2)"    1000000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr val3)"    1000000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr faucet)"  5000000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr feeder1)"   10000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr feeder2)"   10000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr feeder3)"   10000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr relayer)"   10000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr testuser)" 100000000000uvtx --home "$V1"

# --- Patch module params in genesis.json with jq (run jq inside the image) ---
jqi() { docker run --rm -i -v "$PWD/$GEN:/g" "$IMG" sh -c "jq '$1' /g/validator1/config/genesis.json > /g/g.tmp && mv /g/g.tmp /g/validator1/config/genesis.json"; }

log "Patching staking/gov/fees/oracle/rwa params"
jqi '.app_state.staking.params.bond_denom="uvtx"
   | .app_state.crisis.constant_fee.denom="uvtx"
   | .app_state.gov.params.min_deposit[0].denom="uvtx"
   | .app_state.gov.params.voting_period="120s"
   | .app_state.gov.params.expedited_voting_period="60s"
   | .app_state.gov.params.max_deposit_period="120s"'

# fees: 40% burn / 60% distribute (Invariant 5; sum must == 1).
jqi '.app_state.fees.params.burn_ratio="0.400000000000000000"
   | .app_state.fees.params.distribution_ratio="0.600000000000000000"'

# oracle: 5 genesis pairs accept_list.
jqi '.app_state.oracle.params.accept_list=["VTX:USD","BTC:USD","ETH:USD","ATOM:USD","USDC:USD"]'

log "Validating intermediate genesis"
vd genesis validate-genesis --home "$V1"

# --- Place fixed node/consensus keys, then gentx each validator ---
for i in 1 2 3; do
  vh="/g/validator$i"
  [ "$i" = "1" ] || vd init "val$i" --chain-id "$CHAIN_ID" --default-denom "$DENOM" --home "$vh" >/dev/null 2>&1
  cp "$KEYS/validator$i/node_key.json"          "$GEN/validator$i/config/node_key.json"
  cp "$KEYS/validator$i/priv_validator_key.json" "$GEN/validator$i/config/priv_validator_key.json"
  # Validators 2/3 need val1's genesis to gentx against the funded accounts:
  [ "$i" = "1" ] || cp "$GEN/validator1/config/genesis.json" "$GEN/validator$i/config/genesis.json"
  vd genesis gentx "val$i" 1000000000000uvtx \
    --chain-id "$CHAIN_ID" --keyring-backend test --keyring-dir "$KR" \
    --home "$vh" --output-document "/g/gentx-val$i.json"
done

log "Collecting gentxs"
mkdir -p "$GEN/validator1/config/gentx"
cp "$GEN"/gentx-val*.json "$GEN/validator1/config/gentx/"
vd genesis collect-gentxs --home "$V1" --gentx-dir /g/validator1/config/gentx
vd genesis validate-genesis --home "$V1"

# Final genesis fans out to all validators.
cp "$GEN/validator1/config/genesis.json" "$GEN/genesis.json"
for i in 2 3; do cp "$GEN/genesis.json" "$GEN/validator$i/config/genesis.json"; done

# --- persistent_peers from fixed node IDs ---
ID1=$(vd tendermint show-node-id --home /g/validator1)
ID2=$(vd tendermint show-node-id --home /g/validator2)
ID3=$(vd tendermint show-node-id --home /g/validator3)
PEERS="$ID1@vertix-val1:26656,$ID2@vertix-val2:26656,$ID3@vertix-val3:26656"
log "persistent_peers = $PEERS"
echo "$PEERS" > "$GEN/persistent_peers.txt"

# --- per-validator config.toml / app.toml edits via vertixd config + sed-in-image ---
patch_cfg() { # patch_cfg <i>
  local i=$1 ct="/g/validator$i/config/config.toml" at="/g/validator$i/config/app.toml"
  docker run --rm -i -v "$PWD/$GEN:/g" "$IMG" sh -c "
    sed -i 's|^persistent_peers = .*|persistent_peers = \"$PEERS\"|' $ct
    sed -i 's|^laddr = \"tcp://127.0.0.1:26657\"|laddr = \"tcp://0.0.0.0:26657\"|' $ct
    sed -i 's|^prometheus = false|prometheus = true|' $ct
    sed -i 's|^prometheus_listen_addr = \":26660\"|prometheus_listen_addr = \"0.0.0.0:26660\"|' $ct
    sed -i 's|^minimum-gas-prices = \"\"|minimum-gas-prices = \"0.025uvtx\"|' $at
    sed -i '/^\[api\]/,/^\[/ s|^enable = false|enable = true|' $at
    sed -i 's|^address = \"tcp://localhost:1317\"|address = \"tcp://0.0.0.0:1317\"|' $at
    sed -i '/^\[grpc\]/,/^\[/ s|^address = \"localhost:9090\"|address = \"0.0.0.0:9090\"|' $at
  "
}
for i in 1 2 3; do patch_cfg "$i"; done

log "init-genesis complete: $GEN"
```

- [ ] **Step 2: Syntax-check**

Run: `chmod +x scripts/devnet/init-genesis.sh && bash -n scripts/devnet/init-genesis.sh && echo OK`
Expected: `OK`

- [ ] **Step 3: Run the genesis build end-to-end**

Run: `./scripts/devnet/init-genesis.sh`
Expected: ends with `init-genesis complete: infra/devnet/.gen`; no `ASSERT FAILED` / `validate-genesis` errors.

- [ ] **Step 4: Verify the genesis is valid and has the funded accounts + params**

Run:
```bash
docker run --rm -v "$PWD/infra/devnet/.gen:/g" vertix:devnet sh -c \
  'jq ".app_state.fees.params, (.app_state.oracle.params.accept_list), (.app_state.bank.balances|length), (.app_state.genutil.gen_txs|length)" /g/genesis.json'
```
Expected: fees params `0.4/0.6`; accept_list of 5 pairs; balances ≥ 9; `gen_txs` == 3.

- [ ] **Step 5: Verify determinism (re-run yields identical node IDs)**

Run: `cp infra/devnet/.gen/persistent_peers.txt /tmp/peers-a.txt && ./scripts/devnet/init-genesis.sh >/dev/null 2>&1 && diff /tmp/peers-a.txt infra/devnet/.gen/persistent_peers.txt && echo "DETERMINISTIC OK"`
Expected: `DETERMINISTIC OK` (no diff — node IDs are fixed by committed node keys).

- [ ] **Step 6: Commit**

```bash
git add scripts/devnet/init-genesis.sh
git commit -m "feat(devnet): add deterministic genesis + per-validator config builder"
```

---

## Task 6: Validators-only compose stack + liveness

Bring up just the 3 validators (no feeders/gaia yet) to validate consensus, peering, and exposed endpoints.

**Files:**
- Create: `infra/devnet/docker-compose.yml`
- Modify: `Makefile`

- [ ] **Step 1: Create `infra/devnet/docker-compose.yml` (validators only — extended in later tasks)**

```yaml
name: vertix-devnet

x-validator: &validator
  image: ${VERTIX_IMAGE}
  restart: unless-stopped
  networks: [devnet]

services:
  init:
    image: ${VERTIX_IMAGE}
    networks: [devnet]
    volumes:
      - ./.gen:/gen:ro
      - val1data:/v1
      - val2data:/v2
      - val3data:/v3
    entrypoint: ["sh","-c"]
    command:
      - |
        set -e
        for i in 1 2 3; do
          rm -rf /v$$i/* && cp -r /gen/validator$$i/* /v$$i/ 2>/dev/null || true
          mkdir -p /v$$i/config /v$$i/data
          cp -r /gen/validator$$i/config/* /v$$i/config/
          # fresh priv_validator_state for a clean reset
          echo '{"height":"0","round":0,"step":0}' > /v$$i/data/priv_validator_state.json
        done
        echo "init: seeded validator homes"

  vertix-val1:
    <<: *validator
    depends_on: { init: { condition: service_completed_successfully } }
    command: ["vertixd","start","--home","/root/.vertixd"]
    volumes: ["val1data:/root/.vertixd"]
    ports:
      - "26657:26657"   # RPC
      - "9090:9090"     # gRPC
      - "1317:1317"     # REST
      - "26660:26660"   # CometBFT Prometheus

  vertix-val2:
    <<: *validator
    depends_on: { init: { condition: service_completed_successfully } }
    command: ["vertixd","start","--home","/root/.vertixd"]
    volumes: ["val2data:/root/.vertixd"]

  vertix-val3:
    <<: *validator
    depends_on: { init: { condition: service_completed_successfully } }
    command: ["vertixd","start","--home","/root/.vertixd"]
    volumes: ["val3data:/root/.vertixd"]

networks:
  devnet: { name: vertix-devnet }

volumes:
  val1data: {}
  val2data: {}
  val3data: {}
```

> NOTE: the `init` service copies the prebuilt `.gen/validatorN` homes into named volumes. `--home /root/.vertixd` matches where the copy lands (volume mounted at `/root/.vertixd`). The copy in `init` writes to `/vN`; adjust the mount so the validator reads the same volume — the validator mounts `valNdata:/root/.vertixd` and `init` mounts the same volume at `/vN`, so files written to `/vN` appear at `/root/.vertixd`.

- [ ] **Step 2: Add up/down/reset targets to the Makefile**

In the Devnet (Docker) section added in Task 2, append:

```makefile
localnet-genesis:
	@./scripts/devnet/init-genesis.sh

localnet-up: devnet-docker-build localnet-genesis
	@echo "--> Starting devnet stack"
	@$(COMPOSE) up -d

localnet-down:
	@echo "--> Stopping devnet stack"
	@$(COMPOSE) down -v

localnet-reset: localnet-down localnet-up

.PHONY: localnet-genesis localnet-up localnet-down localnet-reset
```

- [ ] **Step 3: Validate the compose file**

Run: `docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml config -q && echo "COMPOSE OK"`
Expected: `COMPOSE OK` (no schema errors).

- [ ] **Step 4: Bring the stack up and confirm all three produce blocks**

Run:
```bash
make localnet-up
source scripts/devnet/lib.sh
wait_height http://localhost:26657 5 120
```
Expected: `height N >= 5 at http://localhost:26657` (consensus advancing → peering works).

- [ ] **Step 5: Confirm 3 bonded validators**

Run: `curl -s http://localhost:26657/validators | jq '.result.validators | length'`
Expected: `3`

- [ ] **Step 6: Confirm REST + CometBFT metrics are reachable**

Run: `curl -sf http://localhost:1317/cosmos/base/tendermint/v1beta1/node_info >/dev/null && curl -sf http://localhost:26660/metrics | head -1 && echo "ENDPOINTS OK"`
Expected: a Prometheus `# HELP ...` line, then `ENDPOINTS OK`.

- [ ] **Step 7: Tear down and commit**

```bash
make localnet-down
git add infra/devnet/docker-compose.yml Makefile
git commit -m "feat(devnet): add 3-validator compose stack with deterministic seeded homes"
```

---

## Task 7: Per-validator feeders + oracle authorization

**Files:**
- Create: `infra/devnet/feeder/feeder1.yaml`, `feeder2.yaml`, `feeder3.yaml`
- Modify: `infra/devnet/docker-compose.yml`, `scripts/devnet/init-genesis.sh`

- [ ] **Step 1: Create `infra/devnet/feeder/feeder1.yaml`**

```yaml
chain_id: vertix-devnet-1
node_grpc: vertix-val1:9090
validator: VALOPER1_PLACEHOLDER     # replaced at seed time by init-genesis.sh
key_name: feeder1
keyring_backend: test
keyring_dir: /keyring
fees: 2000uvtx
gas: "300000"
gas_adjustment: 1.3
feed_interval: 5s
submit_poll_interval: 1s
vote_window: 0
broadcast_retries: 3
quality:
  min_providers: 1
  max_deviation: "0.10"
  max_quote_age: 30s
providers: ["static"]               # offline-safe default (D10); add coingecko/binance for live
static_prices:
  "VTX:USD": "0.10"
  "BTC:USD": "65000.00"
  "ETH:USD": "3500.00"
  "ATOM:USD": "8.50"
  "USDC:USD": "1.00"
pairs:
  - { pair: "VTX:USD",  symbols: { static: "VTX:USD" } }
  - { pair: "BTC:USD",  symbols: { static: "BTC:USD" } }
  - { pair: "ETH:USD",  symbols: { static: "ETH:USD" } }
  - { pair: "ATOM:USD", symbols: { static: "ATOM:USD" } }
  - { pair: "USDC:USD", symbols: { static: "USDC:USD" } }
prometheus: { enabled: true, port: 9200 }
log_level: info
```

- [ ] **Step 2: Create `feeder2.yaml` and `feeder3.yaml`**

Copy `feeder1.yaml` to `feeder2.yaml` and `feeder3.yaml`, changing only:
- `node_grpc:` → `vertix-val2:9090` / `vertix-val3:9090`
- `validator:` → `VALOPER2_PLACEHOLDER` / `VALOPER3_PLACEHOLDER`
- `key_name:` → `feeder2` / `feeder3`

- [ ] **Step 3: Have `init-genesis.sh` render feeder configs + authorize feeders post-start**

Append to the end of `scripts/devnet/init-genesis.sh` (before the final `log` line), a block that writes resolved feeder configs into `.gen/feeder/`:

```bash
# --- Render feeder configs with real valoper addresses ---
mkdir -p "$GEN/feeder"
for i in 1 2 3; do
  vo=$(vaddr "val$i")
  sed "s|VALOPER${i}_PLACEHOLDER|$vo|" "infra/devnet/feeder/feeder$i.yaml" > "$GEN/feeder/feeder$i.yaml"
done
# Export the shared keyring so feeders can sign (the same /keyring dir is mounted into feeder containers).
cp -r "$GEN/keyring" "$GEN/feeder/keyring"
log "rendered feeder configs into $GEN/feeder"
```

- [ ] **Step 4: Create `scripts/devnet/setup-feeders.sh` to run `set-feeder` for each validator**

```bash
#!/usr/bin/env bash
# Authorize each feeder key for its validator via MsgSetFeeder. Run after the chain is live.
set -euo pipefail
cd "$(dirname "$0")/../.."
. scripts/devnet/lib.sh
set -a; . infra/devnet/.env; set +a
KR=/keyring
tx() { docker run --rm -i --network vertix-devnet -v "$PWD/infra/devnet/.gen/keyring:/keyring" "$VERTIX_IMAGE" \
        vertixd tx "$@" --keyring-backend test --keyring-dir "$KR" \
        --chain-id "$CHAIN_ID" --node tcp://vertix-val1:26657 --fees 2000uvtx --gas 300000 -y; }
q()  { docker run --rm -i --network vertix-devnet "$VERTIX_IMAGE" \
        vertixd query "$@" --node tcp://vertix-val1:26657 -o json; }
feeder_addr() { docker run --rm -i -v "$PWD/infra/devnet/.gen/keyring:/keyring" "$VERTIX_IMAGE" \
        vertixd keys show "$1" -a --keyring-backend test --keyring-dir /keyring; }

for i in 1 2 3; do
  fa=$(feeder_addr "feeder$i")
  log "set-feeder: val$i -> $fa"
  tx oracle set-feeder "$fa" --from "val$i"
  sleep 3
done
log "feeder authorization submitted"
```

- [ ] **Step 5: Add the feeder services to `docker-compose.yml`**

Add under `services:` (after `vertix-val3`):

```yaml
  feeder1:
    image: ${VERTIX_IMAGE}
    restart: unless-stopped
    networks: [devnet]
    depends_on: { vertix-val1: { condition: service_started } }
    command: ["vertix-feeder","--config","/cfg/feeder1.yaml"]
    volumes:
      - ./.gen/feeder:/cfg:ro
      - ./.gen/feeder/keyring:/keyring:ro
    ports: ["9201:9200"]

  feeder2:
    image: ${VERTIX_IMAGE}
    restart: unless-stopped
    networks: [devnet]
    depends_on: { vertix-val2: { condition: service_started } }
    command: ["vertix-feeder","--config","/cfg/feeder2.yaml"]
    volumes: ["./.gen/feeder:/cfg:ro","./.gen/feeder/keyring:/keyring:ro"]
    ports: ["9202:9200"]

  feeder3:
    image: ${VERTIX_IMAGE}
    restart: unless-stopped
    networks: [devnet]
    depends_on: { vertix-val3: { condition: service_started } }
    command: ["vertix-feeder","--config","/cfg/feeder3.yaml"]
    volumes: ["./.gen/feeder:/cfg:ro","./.gen/feeder/keyring:/keyring:ro"]
    ports: ["9203:9200"]
```

- [ ] **Step 6: Rebuild genesis (renders feeder configs), bring up, authorize, verify price aggregation**

Run:
```bash
chmod +x scripts/devnet/setup-feeders.sh
make localnet-reset
source scripts/devnet/lib.sh
wait_height http://localhost:26657 3 120
./scripts/devnet/setup-feeders.sh
sleep 25   # allow feeders to submit across a vote window
docker run --rm --network vertix-devnet vertix:devnet \
  vertixd query oracle price VTX:USD --node tcp://vertix-val1:26657 -o json | jq
```
Expected: a JSON price object with a non-empty `price` near `0.10` for `VTX:USD` (proves feeders authorized + submitting + on-chain aggregation across validators).

- [ ] **Step 7: Verify each feeder exposes metrics**

Run: `for p in 9201 9202 9203; do curl -sf "http://localhost:$p/metrics" | grep -m1 feeds_total && echo "feeder $p OK"; done`
Expected: a `feeds_total` line + `feeder <port> OK` for each.

- [ ] **Step 8: Tear down and commit**

```bash
make localnet-down
git add infra/devnet/feeder/ infra/devnet/docker-compose.yml scripts/devnet/init-genesis.sh scripts/devnet/setup-feeders.sh
git commit -m "feat(devnet): add per-validator feeders with MsgSetFeeder authorization and static provider"
```

---

## Task 8: gaia counterparty + Hermes relayer + ICS-20 channel

**Files:**
- Create: `infra/devnet/gaia/init-gaia.sh`, `infra/hermes/config.toml`, `scripts/devnet/setup-ibc.sh`
- Modify: `infra/devnet/docker-compose.yml`

- [ ] **Step 1: Create `infra/devnet/gaia/init-gaia.sh` (gaia single-node entrypoint)**

```bash
#!/usr/bin/env sh
# Single-node gaia genesis bootstrap. Funds the relayer account from a fixed mnemonic.
set -e
CHAIN_ID="${GAIA_CHAIN_ID:-gaia-devnet-1}"
HOME_DIR=/root/.gaia
RELAYER_MNEMONIC="$1"   # passed by compose from mnemonics.env (MNEMONIC_RELAYER)

if [ ! -f "$HOME_DIR/config/genesis.json" ]; then
  gaiad init gaia --chain-id "$CHAIN_ID" --home "$HOME_DIR"
  echo "$RELAYER_MNEMONIC" | gaiad keys add relayer --recover --keyring-backend test --home "$HOME_DIR"
  gaiad keys add val --keyring-backend test --home "$HOME_DIR"
  gaiad genesis add-genesis-account val 1000000000stake,1000000000uatom --keyring-backend test --home "$HOME_DIR"
  gaiad genesis add-genesis-account relayer 1000000000uatom --keyring-backend test --home "$HOME_DIR"
  gaiad genesis gentx val 700000000stake --chain-id "$CHAIN_ID" --keyring-backend test --home "$HOME_DIR"
  gaiad genesis collect-gentxs --home "$HOME_DIR"
  sed -i 's|^laddr = "tcp://127.0.0.1:26657"|laddr = "tcp://0.0.0.0:26657"|' "$HOME_DIR/config/config.toml"
  sed -i 's|^minimum-gas-prices = ""|minimum-gas-prices = "0.0uatom"|' "$HOME_DIR/config/app.toml"
fi
exec gaiad start --home "$HOME_DIR"
```

- [ ] **Step 2: Create `infra/hermes/config.toml`**

```toml
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
host = "0.0.0.0"
port = 3001

[[chains]]
id = "vertix-devnet-1"
rpc_addr = "http://vertix-val1:26657"
grpc_addr = "http://vertix-val1:9090"
event_source = { mode = "push", url = "ws://vertix-val1:26657/websocket", batch_delay = "500ms" }
account_prefix = "vtx"
key_name = "relayer"
store_prefix = "ibc"
gas_price = { price = 0.025, denom = "uvtx" }
gas_multiplier = 1.3
trusting_period = "336h"
clock_drift = "10s"
max_block_time = "30s"

[[chains]]
id = "gaia-devnet-1"
rpc_addr = "http://gaia:26657"
grpc_addr = "http://gaia:9090"
event_source = { mode = "push", url = "ws://gaia:26657/websocket", batch_delay = "500ms" }
account_prefix = "cosmos"
key_name = "relayer"
store_prefix = "ibc"
gas_price = { price = 0.0, denom = "uatom" }
gas_multiplier = 1.3
trusting_period = "336h"
clock_drift = "10s"
max_block_time = "30s"
```

- [ ] **Step 3: Create `scripts/devnet/setup-ibc.sh`**

```bash
#!/usr/bin/env bash
# Import relayer keys into Hermes and open an ICS-20 transfer channel: vertix <-> gaia.
set -euo pipefail
cd "$(dirname "$0")/../.."
. scripts/devnet/lib.sh
set -a; . infra/devnet/.env; . infra/devnet/mnemonics.env; set +a

H() { docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml exec -T hermes "$@"; }

log "Writing relayer mnemonics into hermes container"
H sh -c "printf '%s' '$MNEMONIC_RELAYER' > /tmp/vtx.mn && printf '%s' '$MNEMONIC_RELAYER' > /tmp/gaia.mn"
H hermes keys add --chain vertix-devnet-1 --mnemonic-file /tmp/vtx.mn --key-name relayer --overwrite
H hermes keys add --chain gaia-devnet-1   --mnemonic-file /tmp/gaia.mn --key-name relayer --overwrite

log "Creating ICS-20 channel (transfer/transfer, unordered)"
H hermes create channel --a-chain vertix-devnet-1 --b-chain gaia-devnet-1 \
  --a-port transfer --b-port transfer --new-client-connection --yes

log "IBC channel setup complete"
H hermes query channels --chain vertix-devnet-1
```

- [ ] **Step 4: Add gaia + hermes services to `docker-compose.yml`**

```yaml
  gaia:
    image: ${GAIA_IMAGE}
    restart: unless-stopped
    networks: [devnet]
    environment:
      GAIA_CHAIN_ID: ${GAIA_CHAIN_ID}
    entrypoint: ["sh","-c","/init-gaia.sh \"$$RELAYER_MNEMONIC\""]
    env_file: [./mnemonics.env]
    volumes:
      - ./gaia/init-gaia.sh:/init-gaia.sh:ro
      - gaiadata:/root/.gaia
    ports: ["26757:26657"]

  hermes:
    image: ${HERMES_IMAGE}
    restart: unless-stopped
    networks: [devnet]
    depends_on: { vertix-val1: { condition: service_started }, gaia: { condition: service_started } }
    entrypoint: ["sh","-c","sleep infinity"]   # channel opened by setup-ibc.sh; then relays
    volumes: ["../hermes/config.toml:/root/.hermes/config.toml:ro"]
    ports: ["3001:3001"]
```

Add `gaiadata: {}` under the `volumes:` block.

> NOTE: `mnemonics.env` is read by gaia via `env_file`; ensure `MNEMONIC_RELAYER` exists (it does, from Task 4). The hermes container idles until `setup-ibc.sh` creates the channel; afterward run `docker compose exec -d hermes hermes start` (added to smoke/up flow in Step 6).

- [ ] **Step 5: Make scripts executable + validate compose**

Run: `chmod +x infra/devnet/gaia/init-gaia.sh scripts/devnet/setup-ibc.sh && docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml config -q && echo OK`
Expected: `OK`

- [ ] **Step 6: Bring up, open the channel, start relaying, verify a VTX transfer**

Run:
```bash
make localnet-reset
source scripts/devnet/lib.sh
wait_height http://localhost:26657 3 120
wait_height http://localhost:26757 3 180     # gaia
./scripts/devnet/setup-ibc.sh
docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml exec -d hermes hermes start
sleep 10
# send 1 VTX from testuser to a gaia address
GAIA_ADDR=$(docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml exec -T gaia gaiad keys show relayer -a --keyring-backend test --home /root/.gaia)
docker run --rm --network vertix-devnet -v "$PWD/infra/devnet/.gen/keyring:/keyring" vertix:devnet \
  vertixd tx ibc-transfer transfer transfer channel-0 "$GAIA_ADDR" 1000000uvtx \
  --from testuser --keyring-backend test --keyring-dir /keyring \
  --chain-id vertix-devnet-1 --node tcp://vertix-val1:26657 --fees 2000uvtx --gas 300000 -y
sleep 20
docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml exec -T gaia \
  gaiad query bank balances "$GAIA_ADDR" --node tcp://localhost:26657 -o json | jq
```
Expected: gaia balances include an `ibc/<HASH>` denom with amount `1000000` (the VTX voucher arrived → channel + relayer working).

- [ ] **Step 7: Extend `infra/hermes/config.toml`/`docs` reference + tear down + commit**

```bash
make localnet-down
git add infra/devnet/gaia/ infra/hermes/config.toml scripts/devnet/setup-ibc.sh infra/devnet/docker-compose.yml
git commit -m "feat(devnet): add gaia counterparty, hermes relayer, and ICS-20 channel bootstrap"
```

---

## Task 9: Explorer (Ping.pub) + faucet

**Files:**
- Create: `infra/explorer/chains/vertix.json`
- Modify: `infra/devnet/docker-compose.yml`

- [ ] **Step 1: Verify the pinned explorer + faucet images pull (substitute if needed)**

Run: `set -a; . infra/devnet/.env; set +a; docker pull "$EXPLORER_IMAGE"; docker pull "$FAUCET_IMAGE"`
Expected: both pull. If a tag 404s, find a working tag (`docker search` / the project's registry) and update `infra/devnet/.env`, then re-run. Record the working tags in the commit message.

- [ ] **Step 2: Create `infra/explorer/chains/vertix.json` (Ping.pub chain definition)**

```json
{
  "chain_name": "vertix-devnet",
  "api": ["http://localhost:1317"],
  "rpc": ["http://localhost:26657"],
  "snapshot_provider": "",
  "sdk_version": "0.50.0",
  "coin_type": "118",
  "min_tx_fee": "2000",
  "addr_prefix": "vtx",
  "logo": "",
  "assets": [
    {
      "base": "uvtx",
      "symbol": "VTX",
      "exponent": "6",
      "coingecko_id": "",
      "logo": ""
    }
  ]
}
```

- [ ] **Step 3: Add explorer + faucet services to `docker-compose.yml`**

```yaml
  explorer:
    image: ${EXPLORER_IMAGE}
    restart: unless-stopped
    networks: [devnet]
    depends_on: { vertix-val1: { condition: service_started } }
    volumes: ["../explorer/chains/vertix.json:/app/chains/mainnet/vertix.json:ro"]
    ports: ["8080:80"]

  faucet:
    image: ${FAUCET_IMAGE}
    restart: unless-stopped
    networks: [devnet]
    depends_on: { vertix-val1: { condition: service_started } }
    env_file: [./mnemonics.env]
    environment:
      FAUCET_CONCURRENCY: "2"
      FAUCET_PORT: "8000"
      FAUCET_GAS_PRICE: "0.025uvtx"
      FAUCET_MNEMONIC: ${MNEMONIC_FAUCET}
      FAUCET_ADDRESS_PREFIX: "vtx"
      FAUCET_TOKENS: "uvtx"
      FAUCET_CREDIT_AMOUNT_UVTX: "10000000"
      FAUCET_COOLDOWN_TIME: "60"
    command: ["start","http://vertix-val1:26657"]
    ports: ["8000:8000"]
```

> NOTE: faucet env var names follow the cosmjs faucet. If the substituted faucet image (Step 1) uses different env names, adapt this block to that image's documented variables. The deliverable is "a rate-limited faucet funded from `MNEMONIC_FAUCET` reachable on :8000".

- [ ] **Step 4: Bring up and verify explorer + faucet respond**

Run:
```bash
make localnet-up
source scripts/devnet/lib.sh
wait_http http://localhost:8080 90      # explorer UI
wait_http http://localhost:8000/status 90 || curl -s http://localhost:8000 | head   # faucet health
echo "EXPLORER+FAUCET REACHABLE"
```
Expected: `EXPLORER+FAUCET REACHABLE` (both ports serve HTTP).

- [ ] **Step 5: Verify a faucet drip credits an address**

Run:
```bash
NEW=$(docker run --rm vertix:devnet vertixd keys add tmp --keyring-backend test --output json 2>/dev/null | jq -r .address || true)
# cosmjs faucet credit endpoint:
curl -s -X POST http://localhost:8000/credit -H 'Content-Type: application/json' \
  -d "{\"denom\":\"uvtx\",\"address\":\"$NEW\"}" ; echo
sleep 6
docker run --rm --network vertix-devnet vertix:devnet \
  vertixd query bank balances "$NEW" --node tcp://vertix-val1:26657 -o json | jq
```
Expected: the new address shows a positive `uvtx` balance (faucet operational). If the faucet image's credit API differs, use its documented endpoint.

- [ ] **Step 6: Tear down and commit**

```bash
make localnet-down
git add infra/explorer/chains/vertix.json infra/devnet/docker-compose.yml infra/devnet/.env
git commit -m "feat(devnet): add Ping.pub explorer and faucet services"
```

---

## Task 10: Monitoring (Prometheus + Grafana)

**Files:**
- Create: `infra/monitoring/prometheus.yml`, `infra/monitoring/grafana/provisioning/datasources/prometheus.yml`, `infra/monitoring/grafana/provisioning/dashboards/dashboards.yml`, `infra/monitoring/grafana/dashboards/vertix.json`
- Modify: `infra/devnet/docker-compose.yml`

- [ ] **Step 1: Create `infra/monitoring/prometheus.yml`**

```yaml
global:
  scrape_interval: 15s
scrape_configs:
  - job_name: cometbft
    metrics_path: /metrics
    static_configs:
      - targets: ["vertix-val1:26660","vertix-val2:26660","vertix-val3:26660"]
  - job_name: feeders
    metrics_path: /metrics
    static_configs:
      - targets: ["feeder1:9200","feeder2:9200","feeder3:9200"]
  - job_name: hermes
    static_configs:
      - targets: ["hermes:3001"]
```

- [ ] **Step 2: Create Grafana provisioning — datasource**

`infra/monitoring/grafana/provisioning/datasources/prometheus.yml`:

```yaml
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
```

- [ ] **Step 3: Create Grafana provisioning — dashboard provider**

`infra/monitoring/grafana/provisioning/dashboards/dashboards.yml`:

```yaml
apiVersion: 1
providers:
  - name: vertix
    type: file
    options:
      path: /var/lib/grafana/dashboards
```

- [ ] **Step 4: Create `infra/monitoring/grafana/dashboards/vertix.json` (starter dashboard)**

```json
{
  "title": "Vertix Devnet",
  "uid": "vertix-devnet",
  "schemaVersion": 39,
  "timezone": "browser",
  "panels": [
    {
      "type": "timeseries", "title": "Block height (per validator)",
      "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0},
      "targets": [{"expr": "cometbft_consensus_height", "legendFormat": "{{instance}}"}]
    },
    {
      "type": "timeseries", "title": "Validator missed blocks",
      "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0},
      "targets": [{"expr": "cometbft_consensus_validator_missed_blocks", "legendFormat": "{{instance}}"}]
    },
    {
      "type": "timeseries", "title": "Feeder feeds_total",
      "gridPos": {"h": 8, "w": 12, "x": 0, "y": 8},
      "targets": [{"expr": "feeds_total", "legendFormat": "{{instance}}"}]
    },
    {
      "type": "timeseries", "title": "Feeder errors_total",
      "gridPos": {"h": 8, "w": 12, "x": 12, "y": 8},
      "targets": [{"expr": "errors_total", "legendFormat": "{{instance}}"}]
    }
  ]
}
```

> NOTE: the feeder metric names (`feeds_total`, `errors_total`) come from `feeder/feeder/metrics.go`; if they are namespaced (e.g. `vertix_feeder_feeds_total`), update the `expr` fields to match. Verify with the `curl :9201/metrics` output from Task 7.

- [ ] **Step 5: Add prometheus + grafana services to `docker-compose.yml`**

```yaml
  prometheus:
    image: ${PROMETHEUS_IMAGE}
    restart: unless-stopped
    networks: [devnet]
    volumes: ["../monitoring/prometheus.yml:/etc/prometheus/prometheus.yml:ro"]
    ports: ["9095:9090"]

  grafana:
    image: ${GRAFANA_IMAGE}
    restart: unless-stopped
    networks: [devnet]
    depends_on: { prometheus: { condition: service_started } }
    environment:
      GF_SECURITY_ADMIN_PASSWORD: "devnet"
      GF_AUTH_ANONYMOUS_ENABLED: "true"
      GF_AUTH_ANONYMOUS_ORG_ROLE: "Viewer"
    volumes:
      - ../monitoring/grafana/provisioning:/etc/grafana/provisioning:ro
      - ../monitoring/grafana/dashboards:/var/lib/grafana/dashboards:ro
    ports: ["3000:3000"]
```

- [ ] **Step 6: Bring up and verify Prometheus scrapes all targets + Grafana loads the dashboard**

Run:
```bash
make localnet-up
source scripts/devnet/lib.sh
wait_http http://localhost:9095/-/ready 90
# all targets should appear; allow a scrape cycle
sleep 20
curl -s 'http://localhost:9095/api/v1/targets' | jq -r '.data.activeTargets[] | "\(.labels.job) \(.health)"' | sort -u
wait_http http://localhost:3000/api/health 90
curl -s http://localhost:3000/api/search?query=Vertix | jq -r '.[].title'
```
Expected: targets list shows `cometbft up`, `feeders up` (and `hermes up` once relaying); Grafana search returns `Vertix Devnet`.

- [ ] **Step 7: Tear down and commit**

```bash
make localnet-down
git add infra/monitoring/ infra/devnet/docker-compose.yml
git commit -m "feat(devnet): add minimal Prometheus + Grafana observability stack"
```

---

## Task 11: End-to-end smoke gate (`smoke.sh`) + `make devnet-smoke`

Asserts the full cross-phase chain (D7, spec §8): oracle → rwa lifecycle → fee burn → IBC.

**Files:**
- Create: `scripts/devnet/smoke.sh`
- Modify: `Makefile`

- [ ] **Step 1: Create `scripts/devnet/smoke.sh`**

```bash
#!/usr/bin/env bash
# Full devnet end-to-end smoke gate. Assumes `make localnet-up` + setup-feeders + setup-ibc done,
# or pass --bootstrap to run them. Exits non-zero on the first failed assertion.
set -euo pipefail
cd "$(dirname "$0")/../.."
. scripts/devnet/lib.sh
require docker jq curl bc
set -a; . infra/devnet/.env; set +a

NET=vertix-devnet
KR="$PWD/infra/devnet/.gen/keyring"
RPC=tcp://vertix-val1:26657
vtx()  { docker run --rm -i --network "$NET" -v "$KR:/keyring" "$VERTIX_IMAGE" vertixd "$@"; }
txflags="--keyring-backend test --keyring-dir /keyring --chain-id $CHAIN_ID --node $RPC --fees 4000uvtx --gas 400000 -y"
addr() { vtx keys show "$1" -a --keyring-backend test --keyring-dir /keyring; }

if [ "${1:-}" = "--bootstrap" ]; then
  log "bootstrapping feeders + ibc"
  ./scripts/devnet/setup-feeders.sh
  ./scripts/devnet/setup-ibc.sh
  docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml exec -d hermes hermes start || true
fi

# 1) ORACLE: a configured pair has a non-empty aggregated price.
log "[1/5] oracle price aggregation"
sleep 20
PRICE=$(vtx query oracle price VTX:USD --node "$RPC" -o json | jq -r '.price // .aggregated_price.price // empty')
[ -n "$PRICE" ] || die "no aggregated VTX:USD price"
assert_gt "$PRICE" "0" "VTX:USD price > 0 ($PRICE)"

# 2) RWA lifecycle: register -> attest -> mint -> settle
log "[2/5] rwa lifecycle"
ASSET="asset-smoke-1"
ISSUER=$(addr testuser)
vtx tx rwa register-asset "$ASSET" "Smoke Asset" "VTX:USD" 10000000000uvtx --from testuser $txflags; sleep 4
vtx tx rwa attest-asset "$ASSET" --from testuser $txflags; sleep 4
STATUS=$(vtx query rwa asset "$ASSET" --node "$RPC" -o json | jq -r '.asset.status // .status')
log "post-attest status: $STATUS"
vtx tx rwa mint-rwa "$ASSET" 1000000000 --from testuser $txflags; sleep 4
vtx tx rwa settle-rwa "$ASSET" --from testuser $txflags; sleep 4
FINAL=$(vtx query rwa asset "$ASSET" --node "$RPC" -o json | jq -r '.asset.status // .status')
log "final status: $FINAL"
echo "$FINAL" | grep -qiE 'settle' || die "asset did not reach SETTLED (got $FINAL)"
log "OK: rwa lifecycle reached settled"

# 3) FEES: total supply decreases over a few blocks (EndBlock burn of collected fees).
log "[3/5] fee burn reduces total supply"
SUP1=$(vtx query bank total --node "$RPC" -o json | jq -r '.supply[] | select(.denom=="uvtx") | .amount')
# generate fee traffic
for n in 1 2 3; do vtx tx bank send "$(addr testuser)" "$(addr faucet)" 1000uvtx --from testuser $txflags >/dev/null; sleep 3; done
sleep 6
SUP2=$(vtx query bank total --node "$RPC" -o json | jq -r '.supply[] | select(.denom=="uvtx") | .amount')
log "supply before=$SUP1 after=$SUP2"
assert_gt "$SUP1" "$SUP2" "uvtx total supply decreased (burn)"

# 4) DISTRIBUTION: community pool / fee pool grew (60% distribute path).
log "[4/5] distribution received fees"
POOL=$(vtx query distribution community-pool --node "$RPC" -o json | jq -r '.pool[0].amount // "0"')
log "community pool: $POOL"
assert_gt "$POOL" "0" "distribution community pool > 0"

# 5) IBC: VTX voucher present on gaia after a transfer.
log "[5/5] IBC VTX transfer to gaia"
GADDR=$(docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml exec -T gaia gaiad keys show relayer -a --keyring-backend test --home /root/.gaia)
vtx tx ibc-transfer transfer transfer channel-0 "$GADDR" 500000uvtx --from testuser $txflags; sleep 20
VOUCHER=$(docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml exec -T gaia \
  gaiad query bank balances "$GADDR" --node tcp://localhost:26657 -o json | jq -r '.balances[] | select(.denom|startswith("ibc/")) | .amount' | head -1)
[ -n "$VOUCHER" ] || die "no ibc voucher on gaia"
assert_gt "$VOUCHER" "0" "gaia holds VTX ibc voucher ($VOUCHER)"

log "===================="
log "DEVNET SMOKE: PASS"
log "===================="
```

- [ ] **Step 2: Add `devnet-smoke` target to the Makefile**

In the Devnet (Docker) section:

```makefile
devnet-smoke:
	@./scripts/devnet/smoke.sh --bootstrap

.PHONY: devnet-smoke
```

- [ ] **Step 3: Syntax-check**

Run: `chmod +x scripts/devnet/smoke.sh && bash -n scripts/devnet/smoke.sh && echo OK`
Expected: `OK`

- [ ] **Step 4: Run the full gate against a fresh stack**

Run:
```bash
make localnet-reset
source scripts/devnet/lib.sh
wait_height http://localhost:26657 3 120
wait_height http://localhost:26757 3 180
make devnet-smoke
```
Expected: ends with `DEVNET SMOKE: PASS`; exit code 0.

> If any assertion fails, the script prints the exact failing check. Common adjustments: the JSON paths in `jq` (`.price`, `.asset.status`) must match the actual query responses — verify with `vtx query oracle price VTX:USD -o json` / `vtx query rwa asset <id> -o json` and fix the selector. Do NOT change module code.

- [ ] **Step 5: Tear down and commit**

```bash
make localnet-down
git add scripts/devnet/smoke.sh Makefile
git commit -m "feat(devnet): add end-to-end smoke gate (oracle->rwa->fees->ibc) and make target"
```

---

## Task 12: Documentation (`docs/devnet.md`, `docs/relayer.md`) + AGENTS.md

**Files:**
- Create: `docs/devnet.md`
- Create or modify: `docs/relayer.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: Create `docs/devnet.md`**

Write a runbook with these sections (use real commands from this plan, copy-pasteable):
1. **Overview** — topology diagram (copy from spec §4), services + host ports table.
2. **Prerequisites** — Docker + Docker Compose, `jq`, `curl`, `bc`.
3. **Quick start** — `make localnet-up`, then `./scripts/devnet/setup-feeders.sh`, `./scripts/devnet/setup-ibc.sh`, `docker compose ... exec -d hermes hermes start`.
4. **Reset** — `make localnet-reset` (deterministic; explain fixed mnemonics/node keys, link `mnemonics.env` with the NEVER-FOR-MAINNET warning).
5. **End-to-end walkthrough** — the manual version of `smoke.sh`: submit/observe an oracle price, run the RWA lifecycle, watch supply burn, do an IBC transfer; show the exact `vertixd` commands and expected outputs.
6. **Explorer / faucet / dashboards** — URLs (`:8080`, `:8000`, Grafana `:3000` admin/`devnet`), what to look for.
7. **Troubleshooting** — image pull/tag substitution, feeders not submitting (check `set-feeder`, provider egress, switch to `static`), channel not opening (gaia height), JSON-path drift in smoke.
8. **Acceptance gate** — restate spec §10 and how to evidence each item.

- [ ] **Step 2: Create/extend `docs/relayer.md` with the devnet Hermes recipe**

If `docs/relayer.md` does not exist, create it; otherwise append a `## Devnet (vertix ↔ gaia)` section documenting: `infra/hermes/config.toml`, `scripts/devnet/setup-ibc.sh`, key import, `hermes create channel`, `hermes start`, and a short `rly` alternative (config + `rly tx link`).

- [ ] **Step 3: Add devnet commands to `AGENTS.md` §8 (Quick Command Reference)**

In `AGENTS.md`, under "§8 Quick Command Reference", add a devnet block:

```bash
# Multi-validator devnet (Phase 6, Docker)
make devnet-docker-build       # build vertix:devnet image
make localnet-up               # start 3 validators + feeders + gaia + hermes + explorer + faucet + monitoring
make localnet-reset            # deterministic fresh devnet
make devnet-smoke              # full e2e gate (oracle -> rwa -> fees -> ibc)
make localnet-down             # stop + remove
```

- [ ] **Step 4: Verify docs render (no broken local links) and markdownlint is clean if configured**

Run: `ls docs/devnet.md docs/relayer.md && grep -c '##' docs/devnet.md`
Expected: both files listed; section count ≥ 8.

- [ ] **Step 5: Commit**

```bash
git add docs/devnet.md docs/relayer.md AGENTS.md
git commit -m "docs(devnet): add devnet runbook, relayer recipe, and AGENTS quick commands"
```

---

## Task 13: Update project-structure status + final verification

**Files:**
- Modify: `docs/project-structure.md`

- [ ] **Step 1: Mark Phase 6 infra as realized in `docs/project-structure.md`**

In §9 (`infra/`) and the docs list (§10), update the status legend markers for the now-created files (`infra/devnet/*`, `infra/hermes/config.toml`, `infra/explorer/*`, `infra/monitoring/*`, `docs/devnet.md`, `docs/relayer.md`) from "🛠️ planned" to created, and correct the `make devnet-reset`/`make devnet` note to reflect the coexisting `localnet-*` family (D5). Keep edits minimal and factual.

- [ ] **Step 2: Full clean verification run (fresh machine simulation)**

Run:
```bash
make devnet-docker-build
make localnet-reset
source scripts/devnet/lib.sh
wait_height http://localhost:26657 5 150
wait_height http://localhost:26757 5 180
make devnet-smoke
make localnet-down
echo "PHASE 6 VERIFY OK"
```
Expected: `DEVNET SMOKE: PASS` then `PHASE 6 VERIFY OK`.

- [ ] **Step 3: Confirm the repo lint/build gates still pass (no module code touched)**

Run: `make build && make lint`
Expected: both exit 0 (Phase 6 added only infra/scripts/docs + Makefile/.gitignore; no Go changes).

- [ ] **Step 4: Commit**

```bash
git add docs/project-structure.md
git commit -m "docs(devnet): mark Phase 6 infra realized in project-structure"
```

---

## Acceptance Gate (must all hold)

- `make localnet-reset` is deterministic (Task 5 Step 5; identical `persistent_peers.txt`).
- 3 validators produce blocks and peer (Task 6).
- Feeds aggregate across all 3 validators; `query oracle price VTX:USD` > 0 (Task 7, Task 11 step 1).
- Full RWA lifecycle reaches `SETTLED` with bond release (Task 11 step 2).
- Fee burn reduces `uvtx` total supply and distribution pool grows (Task 11 steps 3–4).
- Live IBC: VTX voucher lands on gaia (Task 8, Task 11 step 5).
- Explorer (`:8080`), faucet (`:8000`), Grafana (`:3000`) reachable; Prometheus scrapes cometbft + feeders (Tasks 9–10).
- `make devnet-smoke` exits 0 (Task 11, Task 13).
- `make build && make lint` still green (Task 13 step 3).

---

## Self-Review Notes (author)

- **Spec coverage:** D1 (Task 6 compose), D2/D3 (Task 8 gaia+hermes), D4 (Task 10), D5 (Makefile `localnet-*` coexist, Tasks 2/6/11), D6 (Task 4 fixed keys + Task 5 determinism check), D7 (Task 11 asserting smoke + Task 12 runbook), D8 (Task 7 `set-feeder`), D9 (Task 5 allocation ≤ 21M, no x/mint touched), D10 (Task 7 `static` provider default + note). Acceptance gate (spec §10) mapped above.
- **Known verification points flagged inline (not placeholders):** explorer/faucet image tags (Task 9 step 1 substitutes), faucet env var names (Task 9 note), feeder metric namespacing (Task 10 note), and smoke `jq` selectors (Task 11 step 4 note). Each has a concrete verification command and a bounded fix that does NOT touch module code.
- **Type/command consistency:** all `vertixd` subcommands match the autocli surface (`set-feeder [feeder]`, `submit-feed [validator] [pair] [price]`, `register-asset [asset-id] [name] [oracle-pair] [bond]`, `attest-asset`, `mint-rwa [asset-id] [notional]`, `settle-rwa`, `query oracle price`, `query rwa asset`). Service/volume names are consistent across compose tasks (`vertix-val{1,2,3}`, `feeder{1,2,3}`, `val{1,2,3}data`, `gaiadata`, network `vertix-devnet`).
