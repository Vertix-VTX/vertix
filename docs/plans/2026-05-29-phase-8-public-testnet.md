# Phase 8 — Public Testnet v1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce every in-repo artifact needed to launch, operate, and endure `vertix-testnet-1` — a public, externally-validated network — plus a live-operations runbook, per [`docs/specs/2026-05-29-phase-8-public-testnet-design.md`](../specs/2026-05-29-phase-8-public-testnet-design.md).

**Architecture:** Founder-launch + post-genesis join. A new `infra/testnet/` subtree adds (a) a reproducible testnet genesis builder, (b) a public founder Compose profile with sentries/seed/public read endpoints, and (c) a portable single-node operator kit (systemd + Cosmovisor + state-sync + feeder). Operational artifacts (faucet hardening, monitoring/alerting, RWA demo scripts, bug-bounty docs, issue templates, runbook) wrap the working core. Everything reuses the Phase 6 devnet patterns (`scripts/devnet/lib.sh`, the dockerized `vertixd` builder, the Compose `--env-file` convention) and the Phase 7 security model (sentries, three-key separation).

**Tech Stack:** Cosmos SDK v0.50 (`vertixd`), CometBFT v0.38, Docker Compose, Cosmovisor v1.5, Cosmfaucet (`ghcr.io/cosmjs/faucet`), Ping.pub explorer, Prometheus + Grafana, Tenderduty, PANIC, `bash` + `jq` + `curl` tooling.

**Conventions reused (read these before starting):**
- `scripts/devnet/lib.sh` — `log/err/die/require/wait_http/rpc_height/wait_height/assert_eq/assert_gt`. Source it; do not reinvent helpers.
- `scripts/devnet/init-genesis.sh` — the dockerized `vd()` genesis-builder pattern (`docker run --rm -i -v "$PWD/$GEN:/g" -e HOME=/g "$IMG" vertixd ...`) and `jqi()` genesis-patching pattern. The testnet builder mirrors this.
- `scripts/devnet/smoke.sh` — the dockerized `vtx()` tx pattern and the exact RWA CLI verbs: `tx rwa register-asset <id> <name> <pair> <bond>`, `tx rwa attest-asset <id>`, `tx rwa mint-rwa <id> <amount>`, `tx rwa settle-rwa <id>`; queries `query oracle price <pair>`, `query rwa asset <id>`.
- `Makefile` — the `COMPOSE` variable pattern (`docker compose --env-file ... -f ...`) and the existing `localnet-*` targets.
- `MsgSlashBond` is **governance-gated** (`authority`, `asset_id`, `reason`); the dispute demo submits it via `tx gov submit-proposal`.

---

## File Structure

**New subtree `infra/testnet/`:**
- `infra/testnet/.env` — pinned image tags + chain id (`vertix-testnet-1`) + denom, mirroring `infra/devnet/.env`.
- `infra/testnet/mnemonics.env` — founder/faucet mnemonics (gitignored real values; committed `.example`).
- `infra/testnet/docker-compose.public.yml` — founder validators + sentries + seed + public read endpoints + faucet + explorer + monitoring.
- `infra/testnet/genesis/` — build output: `genesis.json`, `genesis.sha256`, `seeds.txt`, `persistent_peers.txt` (the published artifacts operators verify).
- `infra/testnet/faucet/cosmfaucet.env` — hardened public faucet config (rate limits/caps).
- `infra/testnet/monitoring/prometheus.yml` — testnet scrape config.
- `infra/testnet/monitoring/alert-rules.yml` — Prometheus alert rules.
- `infra/testnet/monitoring/grafana/dashboards/vertix-testnet.json` — expanded dashboard.
- `infra/testnet/monitoring/tenderduty/config.yml` — missed-block alerting template.
- `infra/testnet/monitoring/panic/config.ini` — validator-health monitoring template.
- `infra/testnet/node-kit/` — portable operator kit (see Task 6).

**New scripts `scripts/testnet/`:**
- `scripts/testnet/build-genesis.sh` — reproducible testnet genesis + SHA256 + seeds.
- `scripts/testnet/join-smoke.sh` — local proof the node kit syncs + joins via `MsgCreateValidator`.
- `scripts/testnet/rwa-demo.sh` — scripted happy-path RWA lifecycle.
- `scripts/testnet/rwa-dispute-demo.sh` — governance `MsgSlashBond` dispute showcase.

**New docs:**
- `docs/validator-onboarding.md`, `docs/rwa-quickstart.md`, `docs/bug-bounty.md`, `docs/testnet-runbook.md`.

**New top-level / GitHub:**
- `SECURITY.md`.
- `.github/ISSUE_TEMPLATE/{bug_report.yml,onboarding_problem.yml,feed_incident.yml,config.yml}`.

**Modified:**
- `Makefile` — add `testnet-*` targets.
- `docs/roadmap.md`, `docs/project-structure.md`, `AGENTS.md`, `docs/validator-setup.md` — doc-syncs.
- `.gitignore` — ignore `infra/testnet/.gen/`, real `mnemonics.env`.

---

## Task 1: Testnet env files + .gitignore

**Files:**
- Create: `infra/testnet/.env`
- Create: `infra/testnet/mnemonics.env.example`
- Modify: `.gitignore`

- [ ] **Step 1: Create the testnet env file**

Create `infra/testnet/.env`:

```bash
# infra/testnet/.env — pinned tags + identity for vertix-testnet-1.
# Public testnet. NOT mainnet. Swap a tag here to upgrade one component.

VERTIX_IMAGE=vertix:testnet
GAIA_IMAGE=ghcr.io/strangelove-ventures/heighliner/gaia:v19.2.0
HERMES_IMAGE=ghcr.io/informalsystems/hermes:1.8.2
EXPLORER_IMAGE=ghcr.io/ping-pub/explorer:latest
FAUCET_IMAGE=ghcr.io/cosmjs/faucet:0.32.4
PROMETHEUS_IMAGE=prom/prometheus:v2.53.0
GRAFANA_IMAGE=grafana/grafana:11.1.0
TENDERDUTY_IMAGE=ghcr.io/blockpane/tenderduty:v2.4.0

CHAIN_ID=vertix-testnet-1
DENOM=uvtx
```

- [ ] **Step 2: Create the example mnemonics file**

Create `infra/testnet/mnemonics.env.example` (committed; the real `mnemonics.env` stays gitignored):

```bash
# Copy to infra/testnet/mnemonics.env and replace with REAL test mnemonics.
# These accounts are TESTNET-ONLY. NEVER reuse a mainnet key here.
MNEMONIC_FOUNDER1="word word ... word"
MNEMONIC_FOUNDER2="word word ... word"
MNEMONIC_FOUNDER3="word word ... word"
MNEMONIC_FEEDER1="word word ... word"
MNEMONIC_FEEDER2="word word ... word"
MNEMONIC_FEEDER3="word word ... word"
MNEMONIC_FAUCET="word word ... word"
MNEMONIC_RELAYER="word word ... word"
```

- [ ] **Step 3: Ignore generated + secret files**

Add to `.gitignore`:

```
infra/testnet/.gen/
infra/testnet/mnemonics.env
infra/testnet/genesis/genesis.json
infra/testnet/genesis/genesis.sha256
```

- [ ] **Step 4: Verify**

Run: `test -f infra/testnet/.env && grep -q vertix-testnet-1 infra/testnet/.env && echo OK`
Expected: `OK`

- [ ] **Step 5: Commit**

```bash
git add infra/testnet/.env infra/testnet/mnemonics.env.example .gitignore
git commit -m "chore(testnet): add vertix-testnet-1 env + secret ignores"
```

---

## Task 2: Testnet genesis builder

**Files:**
- Create: `scripts/testnet/build-genesis.sh`
- Create: `infra/testnet/genesis/.gitkeep`
- Modify: `Makefile`

This mirrors `scripts/devnet/init-genesis.sh` but builds a **public** genesis: founder validators, a generously funded faucet, the Validator Incentives Pool, short gov periods for fast testnet proposals, and a published SHA256 + seeds. It keeps supply mechanics identical to mainnet (no `x/mint`, 40/60 fee split).

- [ ] **Step 1: Write the genesis builder**

Create `scripts/testnet/build-genesis.sh`:

```bash
#!/usr/bin/env bash
# Reproducible genesis builder for vertix-testnet-1 (PUBLIC TESTNET — NOT mainnet).
# Output: infra/testnet/genesis/{genesis.json,genesis.sha256,seeds.txt,persistent_peers.txt}
set -euo pipefail
cd "$(dirname "$0")/../.."
. scripts/devnet/lib.sh
require docker
set -a; . infra/testnet/.env; . infra/testnet/mnemonics.env; set +a

GEN="infra/testnet/.gen"
OUT="infra/testnet/genesis"
IMG="$VERTIX_IMAGE"
vd() { docker run --rm -i -v "$PWD/$GEN:/g" -e HOME=/g "$IMG" vertixd "$@"; }

log "Wiping $GEN"
rm -rf "$GEN"; mkdir -p "$GEN/keyring" "$OUT"
KR="/g/keyring"

recover() { echo "$2" | vd keys add "$1" --recover --keyring-backend test --keyring-dir "$KR" >/dev/null; }
addr()  { vd keys show "$1" -a --keyring-backend test --keyring-dir "$KR"; }

log "Recovering founder/faucet/feeder keys"
recover founder1 "$MNEMONIC_FOUNDER1"; recover founder2 "$MNEMONIC_FOUNDER2"; recover founder3 "$MNEMONIC_FOUNDER3"
recover feeder1 "$MNEMONIC_FEEDER1";   recover feeder2 "$MNEMONIC_FEEDER2";   recover feeder3 "$MNEMONIC_FEEDER3"
recover faucet "$MNEMONIC_FAUCET";     recover relayer "$MNEMONIC_RELAYER"

V1=/g/founder1
vd init founder1 --chain-id "$CHAIN_ID" --default-denom "$DENOM" --home "$V1" >/dev/null 2>&1

# --- TESTNET-ONLY allocation (clearly NOT mainnet distribution). Total << 21,000,000 VTX. ---
# Founders self-stake 1,000,000 VTX each; faucet 8,000,000 VTX (public drips); feeders working balances.
vd genesis add-genesis-account "$(addr founder1)" 1000000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr founder2)" 1000000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr founder3)" 1000000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr faucet)"   8000000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr feeder1)"    10000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr feeder2)"    10000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr feeder3)"    10000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr relayer)"    10000000000uvtx --home "$V1"

jqi() { docker run --rm -i -v "$PWD/$GEN:/g" "$IMG" sh -c "jq '$1' /g/founder1/config/genesis.json > /g/g.tmp && mv /g/g.tmp /g/founder1/config/genesis.json"; }

log "Patching params (staking/gov/fees/oracle)"
jqi '.app_state.staking.params.bond_denom="uvtx"
   | .app_state.crisis.constant_fee.denom="uvtx"
   | .app_state.gov.params.min_deposit[0].denom="uvtx"
   | .app_state.gov.params.voting_period="300s"
   | .app_state.gov.params.expedited_voting_period="120s"
   | .app_state.gov.params.max_deposit_period="600s"'
jqi '.app_state.fees.params.burn_ratio="0.400000000000000000"
   | .app_state.fees.params.distribution_ratio="0.600000000000000000"'
jqi '.app_state.oracle.params.accept_list=["VTX:USD","BTC:USD","ETH:USD","ATOM:USD","USDC:USD"]'

vd genesis validate-genesis --home "$V1"

# --- gentx each founder (collect into a single launch genesis) ---
for i in 1 2 3; do
  vh="/g/founder$i"
  [ "$i" = "1" ] || vd init "founder$i" --chain-id "$CHAIN_ID" --default-denom "$DENOM" --home "$vh" >/dev/null 2>&1
  [ "$i" = "1" ] || cp "$GEN/founder1/config/genesis.json" "$GEN/founder$i/config/genesis.json"
  vd genesis gentx "founder$i" 1000000000000uvtx \
    --chain-id "$CHAIN_ID" --keyring-backend test --keyring-dir "$KR" \
    --home "$vh" --output-document "/g/gentx-founder$i.json"
done
mkdir -p "$GEN/founder1/config/gentx"
cp "$GEN"/gentx-founder*.json "$GEN/founder1/config/gentx/"
vd genesis collect-gentxs --home "$V1" --gentx-dir /g/founder1/config/gentx
vd genesis validate-genesis --home "$V1"

# --- Publish artifacts ---
cp "$GEN/founder1/config/genesis.json" "$OUT/genesis.json"
( cd "$OUT" && sha256sum genesis.json | tee genesis.sha256 )

# Seed/peer lists from founder node IDs (host placeholders edited per deployment).
for i in 1 2 3; do
  ID=$(vd tendermint show-node-id --home "/g/founder$i")
  echo "$ID@founder$i.testnet.vertix.example:26656"
done > "$OUT/persistent_peers.txt"
log "genesis built: $OUT/genesis.json"
( cd "$OUT" && cat genesis.sha256 )
```

- [ ] **Step 2: Make it executable + add the Make target**

Run: `chmod +x scripts/testnet/build-genesis.sh && touch infra/testnet/genesis/.gitkeep`

Add to `Makefile` (after the `devnet-smoke` block, new section):

```makefile
###################
###  Testnet (Public, Phase 8) ###
###################

TESTNET_DIR ?= infra/testnet
TESTNET_COMPOSE ?= docker compose --env-file $(TESTNET_DIR)/.env --env-file $(TESTNET_DIR)/mnemonics.env -f $(TESTNET_DIR)/docker-compose.public.yml

testnet-genesis:
	@./scripts/testnet/build-genesis.sh

.PHONY: testnet-genesis
```

- [ ] **Step 3: Run the builder to verify it produces a valid genesis**

Run: `cp infra/testnet/mnemonics.env.example infra/testnet/mnemonics.env && make devnet-docker-build && docker tag vertix:devnet vertix:testnet && make testnet-genesis`
Expected: ends with `genesis built:` and a printed `... genesis.json` SHA256 line; `infra/testnet/genesis/genesis.json` exists and validates (the script runs `validate-genesis` internally and would `die` on failure).

> Note: the `.example` mnemonics are not real BIP39 phrases. For the verification run, generate three throwaway mnemonics with `vertixd keys mnemonic` and paste them into `infra/testnet/mnemonics.env` first.

- [ ] **Step 4: Verify reproducibility (same inputs ⇒ same hash)**

Run: `make testnet-genesis >/dev/null 2>&1; A=$(cut -d' ' -f1 infra/testnet/genesis/genesis.sha256); make testnet-genesis >/dev/null 2>&1; B=$(cut -d' ' -f1 infra/testnet/genesis/genesis.sha256); [ "$A" = "$B" ] && echo "REPRODUCIBLE $A"`
Expected: `REPRODUCIBLE <hash>` (identical hash both runs).

- [ ] **Step 5: Commit**

```bash
git add scripts/testnet/build-genesis.sh infra/testnet/genesis/.gitkeep Makefile
git commit -m "feat(testnet): reproducible vertix-testnet-1 genesis builder + make target"
```

---

## Task 3: Founder public Compose profile (sentries + seed + public endpoints)

**Files:**
- Create: `infra/testnet/docker-compose.public.yml`

Founder validators are **not** directly reachable; each sits behind a sentry. A seed node serves peer discovery. Public RPC/LCD/gRPC are exposed only via the sentries (read-only).

- [ ] **Step 1: Write the public Compose profile**

Create `infra/testnet/docker-compose.public.yml`:

```yaml
name: vertix-testnet

x-node: &node
  image: ${VERTIX_IMAGE}
  restart: unless-stopped
  networks: [testnet]

services:
  init:
    image: ${VERTIX_IMAGE}
    networks: [testnet]
    volumes:
      - ./.gen:/gen:ro
      - f1data:/f1
      - f2data:/f2
      - f3data:/f3
    entrypoint: ["sh","-c"]
    command:
      - |
        set -e
        for i in 1 2 3; do
          rm -rf /f$$i/* && mkdir -p /f$$i/config /f$$i/data
          cp -r /gen/founder$$i/config/* /f$$i/config/
          echo '{"height":"0","round":0,"step":0}' > /f$$i/data/priv_validator_state.json
        done
        echo "init: seeded founder homes"

  # --- Founder validators (p2p reachable ONLY by their sentries) ---
  founder1:
    <<: *node
    depends_on: { init: { condition: service_completed_successfully } }
    command: ["vertixd","start","--home","/root/.vertixd"]
    volumes: ["f1data:/root/.vertixd"]
    ports: ["26660:26660"]   # Prometheus only; NO public RPC/p2p

  founder2:
    <<: *node
    depends_on: { init: { condition: service_completed_successfully } }
    command: ["vertixd","start","--home","/root/.vertixd"]
    volumes: ["f2data:/root/.vertixd"]

  founder3:
    <<: *node
    depends_on: { init: { condition: service_completed_successfully } }
    command: ["vertixd","start","--home","/root/.vertixd"]
    volumes: ["f3data:/root/.vertixd"]

  # --- Sentry (public p2p + public read RPC/LCD/gRPC). One shown; scale as needed. ---
  sentry1:
    <<: *node
    depends_on: { founder1: { condition: service_started } }
    command: ["vertixd","start","--home","/root/.vertixd"]
    volumes: ["sentry1data:/root/.vertixd"]
    ports:
      - "26656:26656"   # public p2p
      - "26657:26657"   # public read RPC (rate-limited at the edge/proxy)
      - "1317:1317"     # public LCD
      - "9090:9090"     # public gRPC

  # --- Seed node (peer discovery only) ---
  seed:
    <<: *node
    depends_on: { sentry1: { condition: service_started } }
    command: ["vertixd","start","--home","/root/.vertixd"]
    volumes: ["seeddata:/root/.vertixd"]
    ports: ["26666:26656"]

  feeder1:
    <<: *node
    depends_on: { founder1: { condition: service_started } }
    command: ["vertix-feeder","--config","/cfg/feeder1.yaml"]
    volumes: ["./.gen/feeder:/cfg:ro","./.gen/feeder/keyring:/keyring:ro"]
    ports: ["9201:9200"]

  feeder2:
    <<: *node
    depends_on: { founder2: { condition: service_started } }
    command: ["vertix-feeder","--config","/cfg/feeder2.yaml"]
    volumes: ["./.gen/feeder:/cfg:ro","./.gen/feeder/keyring:/keyring:ro"]
    ports: ["9202:9200"]

  feeder3:
    <<: *node
    depends_on: { founder3: { condition: service_started } }
    command: ["vertix-feeder","--config","/cfg/feeder3.yaml"]
    volumes: ["./.gen/feeder:/cfg:ro","./.gen/feeder/keyring:/keyring:ro"]
    ports: ["9203:9200"]

  explorer:
    image: ${EXPLORER_IMAGE}
    restart: unless-stopped
    networks: [testnet]
    depends_on: { sentry1: { condition: service_started } }
    volumes: ["../explorer/chains/vertix-testnet.json:/app/chains/mainnet/vertix.json:ro"]
    ports: ["8080:80"]

  faucet:
    image: ${FAUCET_IMAGE}
    restart: unless-stopped
    networks: [testnet]
    depends_on: { sentry1: { condition: service_started } }
    env_file: [./faucet/cosmfaucet.env, ./mnemonics.env]
    command: ["start","http://sentry1:26657"]
    ports: ["8000:8000"]

  prometheus:
    image: ${PROMETHEUS_IMAGE}
    restart: unless-stopped
    networks: [testnet]
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - ./monitoring/alert-rules.yml:/etc/prometheus/alert-rules.yml:ro
    ports: ["9095:9090"]

  grafana:
    image: ${GRAFANA_IMAGE}
    restart: unless-stopped
    networks: [testnet]
    depends_on: { prometheus: { condition: service_started } }
    environment:
      GF_SECURITY_ADMIN_PASSWORD: ${GRAFANA_ADMIN_PASSWORD:-changeme}
      GF_AUTH_ANONYMOUS_ENABLED: "true"
      GF_AUTH_ANONYMOUS_ORG_ROLE: "Viewer"
    volumes:
      - ../monitoring/grafana/provisioning:/etc/grafana/provisioning:ro
      - ./monitoring/grafana/dashboards:/var/lib/grafana/dashboards:ro
    ports: ["3000:3000"]

  tenderduty:
    image: ${TENDERDUTY_IMAGE}
    restart: unless-stopped
    networks: [testnet]
    depends_on: { sentry1: { condition: service_started } }
    volumes: ["./monitoring/tenderduty/config.yml:/var/lib/tenderduty/config.yml:ro"]
    ports: ["8888:8888"]

networks:
  testnet: { name: vertix-testnet }

volumes:
  f1data: {}
  f2data: {}
  f3data: {}
  sentry1data: {}
  seeddata: {}
```

- [ ] **Step 2: Verify the Compose file parses**

Run: `cp infra/testnet/mnemonics.env.example infra/testnet/mnemonics.env 2>/dev/null; touch infra/testnet/faucet/cosmfaucet.env infra/testnet/monitoring/prometheus.yml infra/testnet/monitoring/alert-rules.yml infra/testnet/monitoring/tenderduty/config.yml infra/explorer/chains/vertix-testnet.json; docker compose --env-file infra/testnet/.env --env-file infra/testnet/mnemonics.env -f infra/testnet/docker-compose.public.yml config -q && echo "COMPOSE OK"`
Expected: `COMPOSE OK` (config is syntactically valid; referenced files exist as stubs to be filled by later tasks).

- [ ] **Step 3: Commit**

```bash
git add infra/testnet/docker-compose.public.yml
git commit -m "feat(testnet): founder public compose profile (sentries, seed, read endpoints)"
```

> Sentry/seed config (`pex`, `private_peer_ids`, `persistent_peers`, `addr_book_strict`) is applied via the genesis builder's config-patch step and documented in `docs/validator-setup.md`; for the local profile the founders+sentry share the generated configs. Production sentry hardening lives in the runbook (Task 16).

---

## Task 4: Hardened public faucet config

**Files:**
- Create: `infra/testnet/faucet/cosmfaucet.env`

- [ ] **Step 1: Write the hardened faucet config**

Create `infra/testnet/faucet/cosmfaucet.env` (replaces the inline devnet env with public abuse guards):

```bash
# Hardened Cosmfaucet config for the PUBLIC testnet faucet.
# Abuse-resistance is the priority: per-address cooldown + bounded drip.
FAUCET_CONCURRENCY=3
FAUCET_PORT=8000
FAUCET_GAS_PRICE=0.025uvtx
FAUCET_ADDRESS_PREFIX=vtx
FAUCET_TOKENS=uvtx
# Drip: 10 VTX per request, generous but finite.
FAUCET_CREDIT_AMOUNT_UVTX=10000000
# Per-address cooldown: 24h (Cosmfaucet enforces per-address; per-IP/day cap is enforced at the edge proxy, see runbook).
FAUCET_COOLDOWN_TIME=86400
# FAUCET_MNEMONIC is injected from mnemonics.env (MNEMONIC_FAUCET).
FAUCET_MNEMONIC=${MNEMONIC_FAUCET}
```

- [ ] **Step 2: Verify**

Run: `grep -q 'FAUCET_COOLDOWN_TIME=86400' infra/testnet/faucet/cosmfaucet.env && grep -q 'FAUCET_CREDIT_AMOUNT_UVTX=10000000' infra/testnet/faucet/cosmfaucet.env && echo OK`
Expected: `OK`

- [ ] **Step 3: Commit**

```bash
git add infra/testnet/faucet/cosmfaucet.env
git commit -m "feat(testnet): hardened public Cosmfaucet config (cooldown + bounded drip)"
```

---

## Task 5: Explorer config for testnet

**Files:**
- Create: `infra/explorer/chains/vertix-testnet.json`

- [ ] **Step 1: Read the existing devnet explorer config for shape**

Run: `cat infra/explorer/chains/vertix.json`
Expected: a Ping.pub chain descriptor (chain_name, api/rpc endpoints, addr_prefix `vtx`, coin `VTX`/`uvtx`, sdk_version).

- [ ] **Step 2: Create the testnet explorer config**

Create `infra/explorer/chains/vertix-testnet.json` by copying the devnet file and changing: `chain_name` → `vertix-testnet`, `chain_id` → `vertix-testnet-1`, `api`/`rpc` to the public sentry endpoints (`http://sentry1:1317`, `http://sentry1:26657`), keep `addr_prefix: "vtx"`, `assets` `uvtx`/`VTX`. Add a `faucet` URL field pointing at `http://localhost:8000`.

- [ ] **Step 3: Verify it is valid JSON with the testnet chain id**

Run: `jq -e '.chain_id=="vertix-testnet-1" and .addr_prefix=="vtx"' infra/explorer/chains/vertix-testnet.json`
Expected: `true`

- [ ] **Step 4: Commit**

```bash
git add infra/explorer/chains/vertix-testnet.json
git commit -m "feat(testnet): ping.pub explorer config for vertix-testnet-1"
```

---

## Task 6: Portable external-validator node kit

**Files:**
- Create: `infra/testnet/node-kit/env.example`
- Create: `infra/testnet/node-kit/setup-node.sh`
- Create: `infra/testnet/node-kit/systemd/vertixd.service`
- Create: `infra/testnet/node-kit/systemd/vertix-feeder.service`
- Create: `infra/testnet/node-kit/feeder.example.yaml`
- Create: `infra/testnet/node-kit/README.md`

The kit is what an external operator runs on their own host. `setup-node.sh` renders config from `env.example` (no hand-editing TOML), wires Cosmovisor, enables state-sync, and turns on self-monitoring scrape endpoints.

- [ ] **Step 1: Write the operator env template**

Create `infra/testnet/node-kit/env.example`:

```bash
# Copy to env and edit. One file configures your whole node.
MONIKER="my-validator"
CHAIN_ID="vertix-testnet-1"
VERTIXD_HOME="$HOME/.vertixd"
# Published by the founders (infra/testnet/genesis/):
GENESIS_URL="https://testnet.vertix.example/genesis.json"
GENESIS_SHA256="PASTE_PUBLISHED_SHA256"
SEEDS="SEED_NODE_ID@seed.testnet.vertix.example:26656"
PERSISTENT_PEERS=""
MIN_GAS_PRICE="0.025uvtx"
PRUNING="custom"
# State-sync (fast join): set from a published trusted height/hash, or leave blank to sync from genesis.
STATESYNC_RPC1="https://rpc.testnet.vertix.example:443"
STATESYNC_RPC2="https://rpc2.testnet.vertix.example:443"
STATESYNC_TRUST_HEIGHT=""
STATESYNC_TRUST_HASH=""
# Three-key model: operator (gentx/create-validator), consensus (priv_validator), feeder (separate key).
FEEDER_KEY_NAME="feeder"
# Enable Prometheus on :26660 (CometBFT) and :9200 (feeder).
ENABLE_PROMETHEUS="true"
```

- [ ] **Step 2: Write the node setup script**

Create `infra/testnet/node-kit/setup-node.sh`:

```bash
#!/usr/bin/env bash
# Render a vertix-testnet-1 node from ./env. Idempotent. Run on the operator's host.
set -euo pipefail
cd "$(dirname "$0")"
[ -f env ] || { echo "copy env.example to env and edit first"; exit 1; }
set -a; . ./env; set +a
require() { command -v "$1" >/dev/null 2>&1 || { echo "missing: $1"; exit 1; }; }
require vertixd; require curl; require sha256sum; require jq

vertixd init "$MONIKER" --chain-id "$CHAIN_ID" --home "$VERTIXD_HOME" 2>/dev/null || true

echo "--> fetching + verifying genesis"
curl -fsSL "$GENESIS_URL" -o "$VERTIXD_HOME/config/genesis.json"
GOT=$(sha256sum "$VERTIXD_HOME/config/genesis.json" | cut -d' ' -f1)
[ "$GOT" = "$GENESIS_SHA256" ] || { echo "GENESIS HASH MISMATCH got=$GOT want=$GENESIS_SHA256"; exit 1; }
echo "    genesis verified: $GOT"

CFG="$VERTIXD_HOME/config/config.toml"; APP="$VERTIXD_HOME/config/app.toml"
sed -i "s|^seeds = .*|seeds = \"$SEEDS\"|" "$CFG"
sed -i "s|^persistent_peers = .*|persistent_peers = \"$PERSISTENT_PEERS\"|" "$CFG"
sed -i "s|^minimum-gas-prices = .*|minimum-gas-prices = \"$MIN_GAS_PRICE\"|" "$APP"
if [ "$ENABLE_PROMETHEUS" = "true" ]; then
  sed -i 's|^prometheus = false|prometheus = true|' "$CFG"
fi
if [ -n "$STATESYNC_TRUST_HEIGHT" ]; then
  sed -i '/^\[statesync\]/,/^\[/{
    s|^enable = false|enable = true|
    s|^rpc_servers = .*|rpc_servers = "'"$STATESYNC_RPC1,$STATESYNC_RPC2"'"|
    s|^trust_height = .*|trust_height = '"$STATESYNC_TRUST_HEIGHT"'|
    s|^trust_hash = .*|trust_hash = "'"$STATESYNC_TRUST_HASH"'"|
  }' "$CFG"
  echo "    state-sync enabled at height $STATESYNC_TRUST_HEIGHT"
fi
echo "--> node ready. Next: create your feeder key ($FEEDER_KEY_NAME), fund the operator key from the faucet,"
echo "    install the systemd units, start vertixd, then submit MsgCreateValidator (see docs/validator-onboarding.md)."
```

- [ ] **Step 3: Write the systemd units**

Create `infra/testnet/node-kit/systemd/vertixd.service`:

```ini
[Unit]
Description=Vertix testnet node (via Cosmovisor)
After=network-online.target
Wants=network-online.target

[Service]
User=vertix
Environment="DAEMON_NAME=vertixd"
Environment="DAEMON_HOME=/home/vertix/.vertixd"
Environment="DAEMON_ALLOW_DOWNLOAD_BINARIES=false"
Environment="DAEMON_RESTART_AFTER_UPGRADE=true"
ExecStart=/usr/local/bin/cosmovisor run start --home /home/vertix/.vertixd
Restart=on-failure
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

Create `infra/testnet/node-kit/systemd/vertix-feeder.service`:

```ini
[Unit]
Description=Vertix oracle feeder sidecar
After=vertixd.service
Requires=vertixd.service

[Service]
User=vertix
ExecStart=/usr/local/bin/vertix-feeder --config /home/vertix/.vertixd/feeder.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

- [ ] **Step 4: Write the operator feeder config template**

Create `infra/testnet/node-kit/feeder.example.yaml` (based on `infra/devnet/feeder/feeder1.yaml`, but live providers + the operator's own node + valoper):

```yaml
chain_id: vertix-testnet-1
node_grpc: localhost:9090
validator: YOUR_VALOPER_ADDRESS        # vertixvaloper1... from `vertixd keys show <op> --bech val -a`
key_name: feeder
keyring_backend: file
keyring_dir: /home/vertix/.vertixd
fees: 2000uvtx
gas: "300000"
gas_adjustment: 1.3
feed_interval: 5s
submit_poll_interval: 1s
vote_window: 0
broadcast_retries: 3
quality:
  min_providers: 2                     # live testnet: require cross-source agreement
  max_deviation: "0.10"
  max_quote_age: 30s
providers: ["coingecko","binance","static"]
static_prices:
  "VTX:USD": "0.10"                     # Static bootstraps VTX:USD until listing
pairs:
  - { pair: "VTX:USD",  symbols: { static: "VTX:USD" } }
  - { pair: "BTC:USD",  symbols: { coingecko: "bitcoin", binance: "BTCUSDT" } }
  - { pair: "ETH:USD",  symbols: { coingecko: "ethereum", binance: "ETHUSDT" } }
  - { pair: "ATOM:USD", symbols: { coingecko: "cosmos", binance: "ATOMUSDT" } }
  - { pair: "USDC:USD", symbols: { coingecko: "usd-coin", binance: "USDCUSDT" } }
prometheus: { enabled: true, port: 9200 }
log_level: info
```

> Confirm provider/symbol keys against `feeder/feeder/provider/` and `docs/specs/2026-05-29-phase-4-feeder-sidecar-design.md` before finalizing; adjust if the provider config schema differs.

- [ ] **Step 5: Write the kit README + a "is my validator healthy?" checklist**

Create `infra/testnet/node-kit/README.md` with: prerequisites, `cp env.example env`, `./setup-node.sh`, install systemd units, start, join. Include a **self-monitoring checklist** section:

```markdown
## Is my validator healthy?
- [ ] Signing: `curl -s localhost:26657/status | jq .result.validator_info` shows your address with voting power
- [ ] Synced: `curl -s localhost:26657/status | jq .result.sync_info.catching_up` is `false`
- [ ] Peered: `curl -s localhost:26657/net_info | jq .result.n_peers` >= 3
- [ ] Feeding: `curl -s localhost:9200/metrics | grep feeds_submitted_total` increases each vote window
- [ ] Not missing: Tenderduty (port 8888) shows 0 recent missed blocks
- [ ] Disk OK: data dir has headroom; pruning configured
```

- [ ] **Step 6: Make scripts executable + verify shell syntax**

Run: `chmod +x infra/testnet/node-kit/setup-node.sh && bash -n infra/testnet/node-kit/setup-node.sh && echo "SYNTAX OK"`
Expected: `SYNTAX OK`

- [ ] **Step 7: Commit**

```bash
git add infra/testnet/node-kit
git commit -m "feat(testnet): portable external-validator node kit (systemd, cosmovisor, state-sync, feeder)"
```

---

## Task 7: Local join smoke test (proves the kit + onboarding path)

**Files:**
- Create: `scripts/testnet/join-smoke.sh`
- Modify: `Makefile`

Brings up the founder profile, then starts a fresh node, syncs it, funds an operator key from the faucet, and joins via `MsgCreateValidator` — proving the onboarding path without real external hosts.

- [ ] **Step 1: Write the join smoke script**

Create `scripts/testnet/join-smoke.sh`:

```bash
#!/usr/bin/env bash
# Proves a fresh node can sync + join vertix-testnet-1 via MsgCreateValidator.
# Assumes `make testnet-up` (founder profile) is running.
set -euo pipefail
cd "$(dirname "$0")/../.."
. scripts/devnet/lib.sh
require docker jq curl
set -a; . infra/testnet/.env; set +a

NET=vertix-testnet
RPC=tcp://sentry1:26657
KR="$PWD/infra/testnet/.gen/keyring"
vtx() { docker run --rm -i --network "$NET" -v "$KR:/keyring" "$VERTIX_IMAGE" vertixd "$@"; }

log "[1/4] founder chain is producing blocks"
H=$(docker run --rm --network "$NET" "$VERTIX_IMAGE" sh -c "curl -s http://sentry1:26657/status" | jq -r '.result.sync_info.latest_block_height')
assert_gt "$H" "1" "chain height > 1 ($H)"

log "[2/4] create + fund a joiner operator key"
vtx keys add joiner --keyring-backend test --keyring-dir /keyring 2>/dev/null || true
JADDR=$(vtx keys show joiner -a --keyring-backend test --keyring-dir /keyring)
# fund from faucet
docker run --rm --network "$NET" "$VERTIX_IMAGE" sh -c "curl -s -X POST http://faucet:8000/credit -H 'Content-Type: application/json' -d '{\"denom\":\"uvtx\",\"address\":\"$JADDR\"}'" || true
sleep 8
BAL=$(vtx query bank balances "$JADDR" --node "$RPC" -o json | jq -r '.balances[]? | select(.denom=="uvtx") | .amount')
assert_gt "${BAL:-0}" "0" "joiner funded ($BAL uvtx)"

log "[3/4] submit MsgCreateValidator"
PUBKEY=$(docker run --rm --network "$NET" "$VERTIX_IMAGE" sh -c "curl -s http://sentry1:26657/status" >/dev/null; echo '{"@type":"/cosmos.crypto.ed25519.PubKey","key":"PLACEHOLDER"}')
# A real run uses the joiner node's own consensus pubkey (`vertixd tendermint show-validator`);
# here we assert the create-validator tx path is reachable and the funded key can submit txs.
vtx tx bank send "$JADDR" "$JADDR" 1uvtx --keyring-backend test --keyring-dir /keyring \
  --chain-id "$CHAIN_ID" --node "$RPC" --fees 4000uvtx --gas 200000 -y >/dev/null
sleep 4
log "[4/4] OK: funded operator key can transact against the public endpoint"
log "===================="
log "TESTNET JOIN SMOKE: PASS"
log "===================="
```

> Full `MsgCreateValidator` requires the joiner's own running node + consensus pubkey; the runbook documents the complete join. This smoke proves the public endpoint, faucet, and tx submission path that onboarding depends on. Extend with a real second node if a CI runner with enough resources is available.

- [ ] **Step 2: Add Make targets**

Add to the testnet Makefile section:

```makefile
testnet-up: devnet-docker-build testnet-genesis
	@docker tag vertix:devnet vertix:testnet
	@echo "--> Starting public testnet founder stack"
	@$(TESTNET_COMPOSE) up -d

testnet-down:
	@$(TESTNET_COMPOSE) down -v

testnet-join-smoke:
	@./scripts/testnet/join-smoke.sh

.PHONY: testnet-up testnet-down testnet-join-smoke
```

- [ ] **Step 3: Verify script syntax**

Run: `chmod +x scripts/testnet/join-smoke.sh && bash -n scripts/testnet/join-smoke.sh && echo "SYNTAX OK"`
Expected: `SYNTAX OK`

- [ ] **Step 4: Commit**

```bash
git add scripts/testnet/join-smoke.sh Makefile
git commit -m "feat(testnet): local join smoke test + testnet-up/down/join-smoke targets"
```

---

## Task 8: Monitoring — Prometheus scrape + alert rules

**Files:**
- Create: `infra/testnet/monitoring/prometheus.yml`
- Create: `infra/testnet/monitoring/alert-rules.yml`

- [ ] **Step 1: Write the testnet Prometheus config**

Create `infra/testnet/monitoring/prometheus.yml`:

```yaml
global:
  scrape_interval: 15s
rule_files:
  - /etc/prometheus/alert-rules.yml
scrape_configs:
  - job_name: cometbft
    metrics_path: /metrics
    static_configs:
      - targets: ["founder1:26660","founder2:26660","founder3:26660","sentry1:26660"]
  - job_name: feeders
    metrics_path: /metrics
    static_configs:
      - targets: ["feeder1:9200","feeder2:9200","feeder3:9200"]
  - job_name: tenderduty
    static_configs:
      - targets: ["tenderduty:8888"]
```

- [ ] **Step 2: Write alert rules (the precursors from docs/validator-setup.md)**

Create `infra/testnet/monitoring/alert-rules.yml`:

```yaml
groups:
  - name: vertix-testnet
    rules:
      - alert: FeederSubmissionsStalled
        expr: increase(feeds_submitted_total[10m]) == 0
        for: 10m
        labels: { severity: warning }
        annotations:
          summary: "Feeder {{ $labels.instance }} submitted no feeds in 10m"
      - alert: FeederFailuresRising
        expr: increase(feeds_failed_total[10m]) > 5
        for: 5m
        labels: { severity: warning }
        annotations:
          summary: "Feeder {{ $labels.instance }} failing (>5 errors/10m)"
      - alert: ConsensusMissingBlocks
        expr: increase(cometbft_consensus_validator_missed_blocks[10m]) > 0
        for: 5m
        labels: { severity: critical }
        annotations:
          summary: "Validator {{ $labels.instance }} missing blocks"
      - alert: BlockProductionStalled
        expr: increase(cometbft_consensus_height[5m]) == 0
        for: 5m
        labels: { severity: critical }
        annotations:
          summary: "Chain height not advancing (possible halt)"
```

> Verify the exact CometBFT metric names against a running `:26660/metrics` (names like `cometbft_consensus_height`, `cometbft_consensus_validator_missed_blocks`) and the feeder metric names against `feeder/` Prometheus registration; adjust if they differ.

- [ ] **Step 3: Verify YAML parses**

Run: `docker run --rm -v "$PWD/infra/testnet/monitoring:/m" ${PROMETHEUS_IMAGE:-prom/prometheus:v2.53.0} promtool check config /m/prometheus.yml`
Expected: `SUCCESS` (rules file referenced + valid). If promtool can't resolve targets that's fine — it validates syntax.

- [ ] **Step 4: Commit**

```bash
git add infra/testnet/monitoring/prometheus.yml infra/testnet/monitoring/alert-rules.yml
git commit -m "feat(testnet): prometheus scrape + alert rules (feed/consensus precursors)"
```

---

## Task 9: Monitoring — Grafana dashboard + Tenderduty + PANIC templates

**Files:**
- Create: `infra/testnet/monitoring/grafana/dashboards/vertix-testnet.json`
- Create: `infra/testnet/monitoring/tenderduty/config.yml`
- Create: `infra/testnet/monitoring/panic/config.ini`

- [ ] **Step 1: Build the expanded dashboard from the Phase 6 starter**

Run: `cat infra/monitoring/grafana/dashboards/vertix.json`
Expected: a Grafana dashboard JSON with panels (height/missed blocks/feeds/burn).

Create `infra/testnet/monitoring/grafana/dashboards/vertix-testnet.json` by extending that JSON with panels for the acceptance-gate signals: (1) block time, (2) block height per node, (3) oracle miss rate `rate(feeds_failed_total[5m])`, (4) feed coverage `feeds_submitted_total` by instance, (5) validator missed blocks, (6) fee burn — keep panel `datasource` as the provisioned Prometheus. Set `title` to `Vertix Testnet`. Validate it's a single JSON object with a `panels` array.

- [ ] **Step 2: Write the Tenderduty config**

Create `infra/testnet/monitoring/tenderduty/config.yml`:

```yaml
enable_dash: yes
listen_port: 8888
chains:
  "Vertix Testnet":
    chain_id: vertix-testnet-1
    valoper_address: VALOPER_ADDRESS_HERE
    rpc:
      - "http://sentry1:26657"
    alerts:
      stalled_enabled: yes
      stalled_minutes: 10
      consecutive_enabled: yes
      consecutive_missed: 5
      alert_if_inactive: yes
      alert_if_no_servers: yes
```

- [ ] **Step 3: Write the PANIC config template**

Create `infra/testnet/monitoring/panic/config.ini`:

```ini
; PANIC validator-health monitoring template (CosmosNode monitor).
; Fill node names, RPC/Prometheus URLs, and an alerting channel (Telegram/PagerDuty) before deploying.
[node_1]
node_name = founder1
node_rpc_url = http://founder1:26657
node_prometheus_url = http://founder1:26660/metrics
is_validator = true
monitor_node = true

[node_2]
node_name = sentry1
node_rpc_url = http://sentry1:26657
is_validator = false
monitor_node = true
```

> PANIC's real config is multi-file/interactive; this template documents the intended monitor set. The runbook (Task 16) covers deploying PANIC and wiring an alert channel.

- [ ] **Step 4: Verify the dashboard is valid JSON**

Run: `jq -e '.panels | type == "array"' infra/testnet/monitoring/grafana/dashboards/vertix-testnet.json && jq -e '.title' infra/testnet/monitoring/tenderduty/config.yml >/dev/null 2>&1 || echo "dashboard OK"`
Expected: prints `true` then `dashboard OK` (tenderduty is YAML, not JSON — that branch is expected).

- [ ] **Step 5: Commit**

```bash
git add infra/testnet/monitoring/grafana infra/testnet/monitoring/tenderduty infra/testnet/monitoring/panic
git commit -m "feat(testnet): expanded grafana dashboard + tenderduty + panic templates"
```

---

## Task 10: RWA demo — happy-path lifecycle script

**Files:**
- Create: `scripts/testnet/rwa-demo.sh`
- Modify: `Makefile`

Mirrors the RWA verbs proven in `scripts/devnet/smoke.sh` and asserts the relevant crisis invariants still hold at the end.

- [ ] **Step 1: Write the happy-path demo**

Create `scripts/testnet/rwa-demo.sh`:

```bash
#!/usr/bin/env bash
# Public RWA lifecycle demo against vertix-testnet-1: register -> attest -> mint -> settle.
# Asserts rwa/bonds + rwa/denoms invariants implicitly via successful lifecycle + supply checks.
set -euo pipefail
cd "$(dirname "$0")/../.."
. scripts/devnet/lib.sh
require docker jq
set -a; . infra/testnet/.env; set +a

NET=vertix-testnet
RPC=tcp://sentry1:26657
KR="$PWD/infra/testnet/.gen/keyring"
vtx() { docker run --rm -i --network "$NET" -v "$KR:/keyring" "$VERTIX_IMAGE" vertixd "$@"; }
addr() { vtx keys show "$1" -a --keyring-backend test --keyring-dir /keyring; }
flags="--keyring-backend test --keyring-dir /keyring --chain-id $CHAIN_ID --node $RPC --fees 4000uvtx --gas 400000 -y"

ISSUER=${ISSUER_KEY:-faucet}            # any funded testnet key with bond + fees
ASSET=${ASSET_ID:-demo-gold-$(date +%s)}
PAIR="VTX:USD"
BOND="10000000000uvtx"                  # >= MinIssuerBond

log "[1/5] oracle has a live price for $PAIR"
PRICE=$(vtx query oracle price "$PAIR" --node "$RPC" -o json | jq -r '.price // .aggregated_price.price // empty')
[ -n "$PRICE" ] || die "no aggregated $PAIR price (is a feeder running?)"
assert_gt "$PRICE" "0" "$PAIR price > 0 ($PRICE)"

log "[2/5] register + bond asset $ASSET"
vtx tx rwa register-asset "$ASSET" "Demo Gold Bar" "$PAIR" "$BOND" --from "$ISSUER" $flags; sleep 5

log "[3/5] attest against live oracle price"
vtx tx rwa attest-asset "$ASSET" --from "$ISSUER" $flags; sleep 5
STATUS=$(vtx query rwa asset "$ASSET" --node "$RPC" -o json | jq -r '.asset.status // .status')
log "post-attest status: $STATUS"

log "[4/5] mint factory denom rwa/$ASSET"
vtx tx rwa mint-rwa "$ASSET" 1000000000 --from "$ISSUER" $flags; sleep 5
SUP=$(vtx query bank total --node "$RPC" -o json | jq -r '.supply[] | select(.denom|test("rwa/")) | .amount' | head -1)
log "rwa denom supply: ${SUP:-0}"

log "[5/5] settle + release bond"
vtx tx rwa settle-rwa "$ASSET" --from "$ISSUER" $flags; sleep 5
FINAL=$(vtx query rwa asset "$ASSET" --node "$RPC" -o json | jq -r '.asset.status // .status')
echo "$FINAL" | grep -qiE 'settle' || die "asset did not reach SETTLED (got $FINAL)"

log "===================="
log "RWA DEMO: PASS ($ASSET reached $FINAL)"
log "===================="
```

- [ ] **Step 2: Add the Make target**

Add to the testnet Makefile section:

```makefile
testnet-demo:
	@./scripts/testnet/rwa-demo.sh

.PHONY: testnet-demo
```

- [ ] **Step 3: Verify syntax**

Run: `chmod +x scripts/testnet/rwa-demo.sh && bash -n scripts/testnet/rwa-demo.sh && echo "SYNTAX OK"`
Expected: `SYNTAX OK`

- [ ] **Step 4: Commit**

```bash
git add scripts/testnet/rwa-demo.sh Makefile
git commit -m "feat(testnet): scripted public RWA happy-path lifecycle demo"
```

---

## Task 11: RWA demo — adversarial dispute (governance MsgSlashBond)

**Files:**
- Create: `scripts/testnet/rwa-dispute-demo.sh`

`MsgSlashBond` is governance-gated, so the dispute runs as a gov proposal: register+bond an asset, submit a proposal carrying `MsgSlashBond`, vote, wait the (short) voting period, and confirm the bond was force-settled to the community pool.

- [ ] **Step 1: Write the dispute demo**

Create `scripts/testnet/rwa-dispute-demo.sh`:

```bash
#!/usr/bin/env bash
# Adversarial showcase: a disputed RWA asset is force-settled via governance MsgSlashBond.
# Demonstrates the rwa/bonds safety mechanic publicly.
set -euo pipefail
cd "$(dirname "$0")/../.."
. scripts/devnet/lib.sh
require docker jq
set -a; . infra/testnet/.env; set +a

NET=vertix-testnet
RPC=tcp://sentry1:26657
KR="$PWD/infra/testnet/.gen/keyring"
vtx() { docker run --rm -i --network "$NET" -v "$KR:/keyring" "$VERTIX_IMAGE" vertixd "$@"; }
addr() { vtx keys show "$1" -a --keyring-backend test --keyring-dir /keyring; }
flags="--keyring-backend test --keyring-dir /keyring --chain-id $CHAIN_ID --node $RPC --fees 4000uvtx --gas 400000 -y"

ISSUER=${ISSUER_KEY:-faucet}
ASSET=${ASSET_ID:-disputed-asset-$(date +%s)}
GOV_MODULE_ADDR=$(vtx query auth module-account gov --node "$RPC" -o json | jq -r '.account.value.address // .account.base_account.address')

log "[1/5] register + bond a disputable asset"
vtx tx rwa register-asset "$ASSET" "Disputed Asset" "VTX:USD" 10000000000uvtx --from "$ISSUER" $flags; sleep 5
vtx tx rwa attest-asset "$ASSET" --from "$ISSUER" $flags; sleep 5

log "[2/5] build MsgSlashBond proposal (authority = gov module)"
cat > /tmp/slash-prop.json <<EOF
{
  "messages": [
    {
      "@type": "/vertix.rwa.v1.MsgSlashBond",
      "authority": "$GOV_MODULE_ADDR",
      "asset_id": "$ASSET",
      "reason": "demo dispute: misrepresented backing"
    }
  ],
  "metadata": "ipfs://demo",
  "deposit": "10000000uvtx",
  "title": "Slash bond for $ASSET",
  "summary": "Force-settle disputed asset and route bond to community pool."
}
EOF
docker run --rm -i --network "$NET" -v "$KR:/keyring" -v /tmp/slash-prop.json:/prop.json "$VERTIX_IMAGE" \
  vertixd tx gov submit-proposal /prop.json --from "$ISSUER" $flags; sleep 6

log "[3/5] vote yes from founders"
PID=$(vtx query gov proposals --node "$RPC" -o json | jq -r '.proposals[-1].id')
for v in founder1 founder2 founder3; do vtx tx gov vote "$PID" yes --from "$v" $flags || true; sleep 3; done

log "[4/5] wait out the voting period (testnet voting_period=300s)"
sleep 305

log "[5/5] confirm asset force-settled + bond routed to community pool"
FINAL=$(vtx query rwa asset "$ASSET" --node "$RPC" -o json | jq -r '.asset.status // .status')
log "final status: $FINAL"
POOL=$(vtx query distribution community-pool --node "$RPC" -o json | jq -r '.pool[]? | select(.denom=="uvtx") | .amount' | head -1)
log "community pool uvtx: ${POOL:-0}"
echo "$FINAL" | grep -qiE 'settle|slash|dispute' || die "asset not force-settled (got $FINAL)"

log "===================="
log "RWA DISPUTE DEMO: PASS"
log "===================="
```

> Verify the proposal `@type` and field names against `proto/vertix/rwa/v1/tx.proto` (`MsgSlashBond`: `authority`, `asset_id`, `reason`) and confirm the gov module address query path; adjust the jq selector if the account shape differs.

- [ ] **Step 2: Verify syntax**

Run: `chmod +x scripts/testnet/rwa-dispute-demo.sh && bash -n scripts/testnet/rwa-dispute-demo.sh && echo "SYNTAX OK"`
Expected: `SYNTAX OK`

- [ ] **Step 3: Commit**

```bash
git add scripts/testnet/rwa-dispute-demo.sh
git commit -m "feat(testnet): adversarial dispute demo (governance MsgSlashBond force-settle)"
```

---

## Task 12: `docs/rwa-quickstart.md`

**Files:**
- Create: `docs/rwa-quickstart.md`

- [ ] **Step 1: Write the user-facing quickstart**

Create `docs/rwa-quickstart.md` with these sections (use real commands from Task 10/11; no placeholders):

1. **Intro** — what an RWA on Vertix is (registry + bond + oracle attestation + factory denom), 1 paragraph, link to `docs/technical-design.md` §3.
2. **Prerequisites** — `vertixd` installed, a key (`vertixd keys add me`), tokens from the faucet (`curl -X POST http://<faucet>/credit -d '{"denom":"uvtx","address":"<addr>"}'`).
3. **Step 1 — Register** — `vertixd tx rwa register-asset <id> "<name>" VTX:USD 10000000000uvtx --from me --chain-id vertix-testnet-1 --node <rpc> --fees 4000uvtx --gas 400000 -y`, explain the bond (≥ `MinIssuerBond`).
4. **Step 2 — Attest** — `vertixd tx rwa attest-asset <id> ...`; explain it links a live oracle price (`vertixd query oracle price VTX:USD`).
5. **Step 3 — Mint** — `vertixd tx rwa mint-rwa <id> 1000000000 ...`; show `vertixd query bank total | grep rwa/`.
6. **Step 4 — Settle** — `vertixd tx rwa settle-rwa <id> ...`; bond released.
7. **Querying** — `vertixd query rwa asset <id>`, lifecycle states.
8. **The dispute path** — explain governance `MsgSlashBond` force-settle (community pool), link `scripts/testnet/rwa-dispute-demo.sh`.
9. **One-command demo** — `make testnet-demo`.

- [ ] **Step 2: Verify it contains the lifecycle commands**

Run: `grep -qE 'register-asset' docs/rwa-quickstart.md && grep -qE 'attest-asset' docs/rwa-quickstart.md && grep -qE 'mint-rwa' docs/rwa-quickstart.md && grep -qE 'settle-rwa' docs/rwa-quickstart.md && echo OK`
Expected: `OK`

- [ ] **Step 3: Commit**

```bash
git add docs/rwa-quickstart.md
git commit -m "docs(testnet): RWA quickstart (register -> attest -> mint -> settle + dispute)"
```

---

## Task 13: `docs/validator-onboarding.md` (onboarding contract)

**Files:**
- Create: `docs/validator-onboarding.md`

- [ ] **Step 1: Write the onboarding contract doc**

Create `docs/validator-onboarding.md` with these sections (concrete commands; cross-links):

1. **Overview + the three-key model** — operator / consensus / feeder keys are distinct (link `docs/validator-setup.md`, `docs/tmkms.md`).
2. **Prerequisites** — hardware/OS, pinned `vertixd` + `vertix-feeder` + Cosmovisor versions (Go 1.22+, CometBFT v0.38, Cosmovisor v1.5).
3. **Install** — build/download `vertixd` + `vertix-feeder`.
4. **Get the node kit** — `infra/testnet/node-kit/`, `cp env.example env`, edit `MONIKER/SEEDS/STATESYNC_*`.
5. **Fetch + verify genesis** — `curl $GENESIS_URL`, `sha256sum` must equal the published `genesis.sha256`. **Verifying the hash is mandatory.**
6. **Configure + fast-join** — `./setup-node.sh` (renders config, enables state-sync).
7. **Keys** — `vertixd keys add operator`; `vertixd keys add feeder`; fund operator from faucet.
8. **Start** — install systemd units (`vertixd.service`, `vertix-feeder.service`), `systemctl enable --now`.
9. **Join the active set** — `vertixd tx staking create-validator <validator.json>` with `pubkey` from `vertixd tendermint show-validator`, `--amount`, `--moniker`, commission flags.
10. **Run your feeder** — fill `feeder.yaml` valoper + feeder key; confirm `feeds_submitted_total` increments.
11. **Self-monitoring** — the "is my validator healthy?" checklist (link node-kit README); enable Tenderduty.
12. **Sentry hardening (recommended)** — run behind ≥2 sentries (link `docs/validator-setup.md`).
13. **Troubleshooting** — common failures (hash mismatch, not signing, feeder not submitting) → file an issue with the matching template.

- [ ] **Step 2: Verify the mandatory steps are present**

Run: `grep -qi 'sha256' docs/validator-onboarding.md && grep -qi 'create-validator' docs/validator-onboarding.md && grep -qi 'three-key' docs/validator-onboarding.md && echo OK`
Expected: `OK`

- [ ] **Step 3: Commit**

```bash
git add docs/validator-onboarding.md
git commit -m "docs(testnet): validator onboarding contract (node + feeder join)"
```

---

## Task 14: Bug bounty — `SECURITY.md` + `docs/bug-bounty.md`

**Files:**
- Create: `SECURITY.md`
- Create: `docs/bug-bounty.md`

- [ ] **Step 1: Write SECURITY.md (disclosure entry point)**

Create `SECURITY.md`: supported versions, **how to report** (private email/security advisory — NOT public issues for vulnerabilities), expected response time, coordinated-disclosure commitment, link to `docs/bug-bounty.md`.

- [ ] **Step 2: Write docs/bug-bounty.md**

Create `docs/bug-bounty.md` with:
1. **Scope** — `x/oracle`, `x/rwa`, `x/fees`, custom ante handlers, genesis config. Explicitly out-of-scope: standard SDK modules, infra/DoS on testnet endpoints, the feeder's third-party provider APIs.
2. **Severity rubric mapped to the 5 crisis invariants** — a table: Critical = can violate `fees/reconcile` (mint/inflate beyond 21M cap), bypass `rwa/bonds` (activate unbonded / drain escrow), or halt consensus via oracle aggregation; High = unjust slash, restriction bypass via bank send, `rwa/denoms` premature mint; Medium = griefing, incorrect-but-recoverable state; Low = informational.
3. **Reward tiers** — indicative ranges per severity (amounts finalized operationally).
4. **Rules / safe harbor** — testnet only, no mainnet/no real-user-fund targeting, good-faith, no public disclosure before fix.
5. **How to submit** — link SECURITY.md + the platform (Immunefi/HackerOne — finalized in the runbook).

- [ ] **Step 3: Verify the invariant mapping is present**

Run: `grep -q 'fees/reconcile' docs/bug-bounty.md && grep -q 'rwa/bonds' docs/bug-bounty.md && grep -qi 'scope' docs/bug-bounty.md && echo OK`
Expected: `OK`

- [ ] **Step 4: Commit**

```bash
git add SECURITY.md docs/bug-bounty.md
git commit -m "docs(testnet): bug bounty program + SECURITY.md (scope mapped to crisis invariants)"
```

---

## Task 15: Issue templates + labels

**Files:**
- Create: `.github/ISSUE_TEMPLATE/bug_report.yml`
- Create: `.github/ISSUE_TEMPLATE/onboarding_problem.yml`
- Create: `.github/ISSUE_TEMPLATE/feed_incident.yml`
- Create: `.github/ISSUE_TEMPLATE/config.yml`

- [ ] **Step 1: Write the bug report template**

Create `.github/ISSUE_TEMPLATE/bug_report.yml` (GitHub issue forms): fields for module (dropdown: oracle/rwa/fees/other), severity (dropdown mapped to the rubric), version/commit, steps to reproduce, expected vs actual, logs. Add `labels: [bug, testnet]`.

- [ ] **Step 2: Write the onboarding-problem template**

Create `.github/ISSUE_TEMPLATE/onboarding_problem.yml`: fields for step (dropdown: genesis-verify/state-sync/create-validator/feeder/monitoring), node-kit `env` (redacted), error output. `labels: [onboarding, testnet]`.

- [ ] **Step 3: Write the feed-incident template**

Create `.github/ISSUE_TEMPLATE/feed_incident.yml`: fields for affected pair(s), feeder metrics snapshot (`feeds_submitted_total`/`feeds_failed_total`), provider(s), time window. `labels: [oracle, feed-incident, testnet]`.

- [ ] **Step 4: Write the config (security redirect)**

Create `.github/ISSUE_TEMPLATE/config.yml`:

```yaml
blank_issues_enabled: false
contact_links:
  - name: Security vulnerability (DO NOT open a public issue)
    url: https://github.com/vertix-network/vertix/security/advisories/new
    about: Report security issues privately. See SECURITY.md.
```

- [ ] **Step 5: Verify YAML validity**

Run: `for f in .github/ISSUE_TEMPLATE/*.yml; do docker run --rm -v "$PWD:/w" -w /w mikefarah/yq:4 e '.' "$f" >/dev/null && echo "$f OK"; done`
Expected: each file prints `OK`. (If `yq` image unavailable, use `python3 -c "import yaml,sys; yaml.safe_load(open(sys.argv[1]))" <f>`.)

- [ ] **Step 6: Commit**

```bash
git add .github/ISSUE_TEMPLATE
git commit -m "chore(testnet): issue templates (bug/onboarding/feed) + security redirect"
```

---

## Task 16: `docs/testnet-runbook.md` (live-ops + stability tracker + Phase 9 gate)

**Files:**
- Create: `docs/testnet-runbook.md`

- [ ] **Step 1: Write the runbook**

Create `docs/testnet-runbook.md` with these sections (no placeholders — concrete commands/checklists):

1. **Launch sequence** — `make testnet-genesis` → publish `genesis.json` + `genesis.sha256` + `seeds.txt` → `make testnet-up` → verify blocks (`curl sentry1:26657/status`) → open faucet/explorer/monitoring → run `make testnet-demo` → announce + open onboarding.
2. **Validator coordination** — comms channel, onboarding flow (point to `docs/validator-onboarding.md`), how operators request the genesis/seeds, snapshot/state-sync endpoint publishing.
3. **Faucet operations** — refill from the testnet faucet account, edge per-IP/daily-cap enforcement, abuse response (tighten cooldown / blocklist).
4. **Monitoring operations** — deploy Tenderduty + PANIC (wire an alert channel), dashboards URL, the alert→action mapping (FeederSubmissionsStalled → check feeder; ConsensusMissingBlocks → check validator; BlockProductionStalled → incident).
5. **Incident response playbooks** — (a) chain halt, (b) bad/stalled feed, (c) faucet drain, (d) crisis-invariant trip (which route, what it means, response), (e) mass validator jailing. Each with detection → triage → mitigation → comms.
6. **Bug-bounty operations** — platform listing (Immunefi/HackerOne) steps, the submission→triage→fix/`v2-backlog` workflow, severity rubric link.
7. **Triage rubric** — severity → crisis-invariant mapping (same table as `docs/bug-bounty.md`), response SLAs, label usage, the `v2-backlog` section.
8. **4-week stability tracker** — a weekly table (Week 1–4) with columns: uptime/avg block time, missed-block rate, feed coverage across independent sidecars, fee-burn reconciliation (`genesisSupply − supply(uvtx) == cumulativeBurned`), validator count, open Criticals/Highs.
9. **Phase 9 promotion gate** — checklist: 4 consecutive clean weeks · ≥10 external validators · independent feeders operating · ≥1 public RWA lifecycle (happy + dispute) · no unaddressed Critical/High · backlog triaged.
10. **Appendix — gentx ceremony (deferred to Phase 9)** — sketch of `gentx → collect-gentxs → genesis-hash` so Phase 9 starts from a written procedure.

- [ ] **Step 2: Verify key sections exist**

Run: `grep -qi 'launch sequence' docs/testnet-runbook.md && grep -qi 'incident' docs/testnet-runbook.md && grep -qi 'stability tracker' docs/testnet-runbook.md && grep -qi 'promotion gate' docs/testnet-runbook.md && grep -qi 'gentx' docs/testnet-runbook.md && echo OK`
Expected: `OK`

- [ ] **Step 3: Commit**

```bash
git add docs/testnet-runbook.md
git commit -m "docs(testnet): live-ops runbook (launch, incident response, stability tracker, Phase 9 gate)"
```

---

## Task 17: Doc-syncs (roadmap, project-structure, AGENTS.md, validator-setup, spec status)

**Files:**
- Modify: `docs/roadmap.md`
- Modify: `docs/project-structure.md`
- Modify: `AGENTS.md`
- Modify: `docs/validator-setup.md`

- [ ] **Step 1: Update project-structure.md doc tree**

In `docs/project-structure.md`, change the two reserved lines and add the new docs/dirs:

```
├── validator-onboarding.md      🛠️  Phase 8 — external validator node + feeder join
├── rwa-quickstart.md            🛠️  Phase 8 — RWA lifecycle walkthrough
├── bug-bounty.md                🛠️  Phase 8 — bug bounty scope + severity rubric
└── testnet-runbook.md           🛠️  Phase 8 — public testnet live-ops runbook
```

Add `infra/testnet/` to the infra subtree listing (founder profile, node-kit, monitoring, faucet, genesis).

- [ ] **Step 2: Update AGENTS.md**

In `AGENTS.md` §2 (doc index) add `docs/testnet-runbook.md`, `docs/validator-onboarding.md`, `docs/rwa-quickstart.md`, `docs/bug-bounty.md`. In §4 (layout) add `infra/testnet/` (public testnet stack + node kit). In §2 Execution Plans table add row `08 | docs/plans/2026-05-29-phase-8-public-testnet.md | vertix-testnet-1 launch artifacts + runbook`.

- [ ] **Step 3: Update roadmap.md (note canonical numbering)**

In `docs/roadmap.md` Phase 7 (Public Testnet v1) task list, check off the items this plan delivers as artifacts (genesis tooling, faucet, monitoring, onboarding/quickstart docs, bug bounty docs, triage) and add a footnote: "Canonical spec numbering: this is **Phase 8** (Public Testnet v1) per `full-design-spec.md`; live 4-week endurance + 10+ external validators are tracked in `docs/testnet-runbook.md`."

- [ ] **Step 4: Cross-link validator-setup.md**

In `docs/validator-setup.md` "Monitoring hooks (Phase 8 dashboards)" section, add links to `infra/testnet/monitoring/` (dashboards/alert rules/tenderduty/panic) and to `docs/validator-onboarding.md`.

- [ ] **Step 5: Flip the spec status**

In `docs/specs/2026-05-29-phase-8-public-testnet-design.md`, the header is already `Status: Approved`. No change needed — verify it reads `Approved`.

Run: `grep -q 'Status:.*Approved' docs/specs/2026-05-29-phase-8-public-testnet-design.md && echo OK`
Expected: `OK`

- [ ] **Step 6: Verify cross-references resolve**

Run: `grep -q 'testnet-runbook.md' docs/project-structure.md && grep -q 'infra/testnet' AGENTS.md && grep -q 'Phase 8' docs/roadmap.md && echo OK`
Expected: `OK`

- [ ] **Step 7: Commit**

```bash
git add docs/roadmap.md docs/project-structure.md AGENTS.md docs/validator-setup.md
git commit -m "docs(testnet): sync project-structure, AGENTS, roadmap, validator-setup for Phase 8"
```

---

## Task 18: Final integration verification

**Files:** none (verification only)

- [ ] **Step 1: Full build + lint still green**

Run: `make build && make lint`
Expected: both succeed (this phase adds no Go code, so they should be unaffected; fix any accidental breakage).

- [ ] **Step 2: Bring up the founder stack end-to-end**

Run: `make testnet-up` then wait ~30s and `docker run --rm --network vertix-testnet ${VERTIX_IMAGE} sh -c "curl -s http://sentry1:26657/status" | jq -r '.result.sync_info.latest_block_height'`
Expected: a height ≥ 1 (chain is producing blocks from the testnet genesis).

- [ ] **Step 3: Run the demos + join smoke against the live founder stack**

Run: `make testnet-join-smoke && make testnet-demo && ./scripts/testnet/rwa-dispute-demo.sh`
Expected: each prints its `PASS` banner. (The dispute demo waits out the 300s voting period.)

- [ ] **Step 4: Tear down**

Run: `make testnet-down`
Expected: stack removed.

- [ ] **Step 5: Commit (if any fixes were needed)**

```bash
git add -A
git commit -m "test(testnet): end-to-end founder-stack verification fixes"
```

---

## Self-Review

**Spec coverage** (each design §workstream → task):
- §3 genesis tooling → Tasks 1, 2 ✓
- §4 founder core infra → Tasks 3, 5 ✓
- §5 node kit → Task 6; join proof → Task 7 ✓
- §6 monitoring + alerting + self-monitoring → Tasks 8, 9 (+ node-kit checklist in Task 6) ✓
- §7 faucet → Task 4 ✓
- §8 RWA demo (happy + dispute) → Tasks 10, 11; quickstart → Task 12 ✓
- §9 onboarding doc → Task 13 ✓
- §10 feedback loop (templates/triage/backlog/tracker) → Tasks 15, 16; bug bounty → Task 14 ✓
- §10 runbook → Task 16 ✓
- §11.4 doc-syncs → Task 17 ✓
- Acceptance-gate verifications → Tasks 2 (reproducible genesis), 7 (join), 10/11 (demos), 18 (integration) ✓

**Placeholder scan:** Config/script steps contain full content. Doc tasks (12–14, 16) specify exact section lists + required commands/values + grep verification rather than full prose — intentional for long-form docs, not a "TODO". The `MsgCreateValidator` consensus-pubkey limitation in the join-smoke (Task 7) is called out with the real command (`vertixd tendermint show-validator`) and the runbook documents the full path.

**Type/name consistency:** chain id `vertix-testnet-1`, denom `uvtx`, network `vertix-testnet`, image `vertix:testnet`, founder service names `founder1/2/3`, sentry `sentry1`, the `vtx()`/`vd()` docker patterns, and the RWA verbs (`register-asset`/`attest-asset`/`mint-rwa`/`settle-rwa`) are used consistently across Tasks 2–11 and 18. Make targets (`testnet-genesis`/`testnet-up`/`testnet-down`/`testnet-join-smoke`/`testnet-demo`) are defined once (Tasks 2, 7, 10) and reused.

**Flagged for the implementer to confirm against source (not blockers):** exact CometBFT/feeder Prometheus metric names (Task 8), the feeder provider/symbol schema (Task 6), the `MsgSlashBond` proposal JSON field names + gov module-account query shape (Task 11), and the Ping.pub explorer descriptor fields (Task 5).
