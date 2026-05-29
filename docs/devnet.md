# Vertix Devnet Runbook

Reproducible **3-validator** Docker Compose stack for `vertix-devnet-1`, with one `vertix-feeder` per validator, gaia + Hermes IBC, and supporting explorer/faucet/monitoring services. Design: [`specs/2026-05-29-phase-6-devnet-design.md`](./specs/2026-05-29-phase-6-devnet-design.md).

> **Devnet-only keys:** [`infra/devnet/mnemonics.env`](../infra/devnet/mnemonics.env) and [`infra/devnet/keys/`](../infra/devnet/keys/) hold fixed mnemonics and node keys. **Never fund or reuse these on mainnet or any public network.**

---

## Overview

Three `vertixd` validators peer on a single Docker network; each has a dedicated feeder sidecar. gaia (`gaia-devnet-1`) is the IBC counterparty; Hermes opens an ICS-20 `transfer` channel. Explorer, faucet, Prometheus, and Grafana attach to `vertix-val1` (or scrape targets).

```
            ┌──────────────────────── vertix-devnet-1 ────────────────────────┐
            │  vertix-val1 ◄─persistent_peers─► vertix-val2 ◄──► vertix-val3   │
            │   ▲ host:26657/9090/1317   :26660(prom)                          │
            │  feeder1 (:9201→9200)   feeder2 (:9202)   feeder3 (:9203)        │
            └──────────────────────────────────────────────────────────────────┘
                         │                                   ▲
                       hermes ──── ICS-20 transfer channel ──┘
                         │
                       gaia (single-node cosmoshub, host:26757)

  explorer (Ping.pub) ─► val1 RPC/REST    faucet (Cosmfaucet) ─► val1
  prometheus ─► scrape {val1..3:26660, feeder1..3:9200}    grafana ─► prometheus
```

| Service | Host port | Notes |
|---------|-----------|--------|
| `vertix-val1` RPC | `26657` | Primary CLI / queries |
| `vertix-val1` gRPC | `9090` | Feeder + Hermes |
| `vertix-val1` REST | `1317` | Explorer API |
| `vertix-val1` CometBFT metrics | `26660` | Prometheus scrape |
| `feeder1` / `feeder2` / `feeder3` metrics | `9201` / `9202` / `9203` | Container `:9200` |
| `gaia` RPC | `26757` | Maps container `26657` |
| Hermes telemetry | `3001` | Relayer metrics |
| Ping.pub explorer | `8080` | When `explorer` service is up |
| Faucet | `8000` | When `faucet` service is up |
| Prometheus | `9095` | When `prometheus` service is up |
| Grafana | `3000` | Login `admin` / password `devnet` |

**Make targets (Docker family, coexists with Ignite `make devnet` / `make devnet-reset`):**

| Target | Purpose |
|--------|---------|
| `make devnet-docker-build` | Build `vertix:devnet` image |
| `make localnet-up` | Build image, run genesis init, `docker compose up -d` |
| `make localnet-reset` | `localnet-down` then `localnet-up` (deterministic fresh state) |
| `make localnet-down` | Stop stack and remove volumes |
| `make devnet-smoke` | Full automated gate (`scripts/devnet/smoke.sh --bootstrap`) |

---

## Prerequisites

On the host:

- Docker Engine + Docker Compose v2 (`docker compose`)
- `bash`, `jq`, `curl`, `bc` (used by `scripts/devnet/lib.sh` and smoke)
- Enough RAM/CPU for ~10 containers (validators, feeders, gaia, hermes, plus optional explorer/faucet/monitoring)

From the repo root:

```bash
make build   # optional: local vertixd for host-side queries
```

---

## Quick start

```bash
# 1) Build image and start stack (runs init-genesis.sh, then compose)
make localnet-up

# 2) Wait for block production
source scripts/devnet/lib.sh
wait_height http://localhost:26657 5 150
wait_height http://localhost:26757 5 180

# 3) Authorize feeders (MsgSetFeeder) and open IBC channel
./scripts/devnet/setup-feeders.sh
./scripts/devnet/setup-ibc.sh

# 4) Start Hermes relaying in the background
docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml exec -d hermes hermes start
```

Sanity checks:

```bash
curl -s http://localhost:26657/status | jq -r '.result.sync_info.latest_block_height'
docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml exec -T hermes hermes query channels --chain vertix-devnet-1
```

Automated gate (bootstrap + assert oracle → rwa → fees → IBC):

```bash
make devnet-smoke
```

Expected final line: `DEVNET SMOKE: PASS`.

---

## Reset

`make localnet-reset` tears down volumes and re-runs `scripts/devnet/init-genesis.sh` before starting validators again. Because mnemonics and validator node keys are **fixed and committed**, genesis accounts, valoper addresses, node IDs, and `persistent_peers` are identical on every reset.

- Mnemonics: [`infra/devnet/mnemonics.env`](../infra/devnet/mnemonics.env) — header warns **NEVER fund on a real network**.
- Consensus/node keys: [`infra/devnet/keys/validator{1,2,3}/`](../infra/devnet/keys/)
- Generated runtime output (not committed): `infra/devnet/.gen/`, validator data volumes.

To regenerate keys from scratch (rare; breaks determinism vs other clones):

```bash
./scripts/devnet/gen-keys.sh   # overwrites mnemonics.env + keys/
make localnet-reset
```

**Ignite single-node loop** (fast inner dev, different topology): `make devnet-reset` / `make devnet` — does not use this Docker stack.

---

## End-to-end walkthrough

Manual flow matching `scripts/devnet/smoke.sh`. Uses the `vertix:devnet` image and test keyring under `infra/devnet/.gen/keyring` on network `vertix-devnet`.

```bash
set -a && . infra/devnet/.env && set +a
NET=vertix-devnet
KR="$PWD/infra/devnet/.gen/keyring"
RPC=tcp://vertix-val1:26657
vtx() { docker run --rm -i --network "$NET" -v "$KR:/keyring" "$VERTIX_IMAGE" vertixd "$@"; }
txflags="--keyring-backend test --keyring-dir /keyring --chain-id $CHAIN_ID --node $RPC --fees 4000uvtx --gas 400000 -y"
```

### 1. Oracle aggregation

After `setup-feeders.sh` and a short wait for votes:

```bash
vtx query oracle price VTX:USD --node "$RPC" -o json | jq .
```

Expect a non-zero aggregated price (feeders default to `static` provider in [`infra/devnet/feeder/feeder1.yaml`](../infra/devnet/feeder/feeder1.yaml)).

```bash
vtx query oracle feeder "$(vtx keys show val1 --bech val -a --keyring-backend test --keyring-dir /keyring)" --node "$RPC" -o json | jq .
```

### 2. RWA lifecycle

```bash
ASSET=asset-manual-1
vtx tx rwa register-asset "$ASSET" "Manual Asset" "VTX:USD" 10000000000uvtx --from testuser $txflags
sleep 4
vtx tx rwa attest-asset "$ASSET" --from testuser $txflags
sleep 4
vtx query rwa asset "$ASSET" --node "$RPC" -o json | jq -r '.asset.status // .status'
vtx tx rwa mint-rwa "$ASSET" 1000000000 --from testuser $txflags
sleep 4
vtx tx rwa settle-rwa "$ASSET" --from testuser $txflags
sleep 4
vtx query rwa asset "$ASSET" --node "$RPC" -o json | jq -r '.asset.status // .status'
```

Expect status to progress toward **SETTLED** (exact enum string from query JSON).

### 3. Fee burn (supply)

```bash
vtx query bank total --node "$RPC" -o json | jq -r '.supply[] | select(.denom=="uvtx") | .amount'
# Generate fee traffic, wait ~6 blocks, re-query — total uvtx supply should decrease (40% burn).
for n in 1 2 3; do
  vtx tx bank send "$(vtx keys show testuser -a --keyring-backend test --keyring-dir /keyring)" \
    "$(vtx keys show faucet -a --keyring-backend test --keyring-dir /keyring)" 1000uvtx --from testuser $txflags
  sleep 3
done
```

### 4. Distribution pool

```bash
vtx query distribution community-pool --node "$RPC" -o json | jq .
```

Community pool should grow as fees are swept (60% distribute path).

### 5. IBC transfer to gaia

Requires `setup-ibc.sh` and `hermes start` (channel is typically `channel-0` on vertix):

```bash
GADDR=$(docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml \
  exec -T gaia gaiad keys show relayer -a --keyring-backend test --home /root/.gaia)
vtx tx ibc-transfer transfer transfer channel-0 "$GADDR" 500000uvtx --from testuser $txflags
sleep 20
docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml exec -T gaia \
  gaiad query bank balances "$GADDR" --node tcp://localhost:26657 -o json | jq '.balances[] | select(.denom|startswith("ibc/"))'
```

Expect an `ibc/…` voucher with positive amount.

---

## Explorer, faucet, and dashboards

| UI | URL | What to check |
|----|-----|----------------|
| Ping.pub explorer | http://localhost:8080 | Blocks advancing; RWA/fee txs from walkthrough |
| Faucet | http://localhost:8000 | Health / credit API |
| Grafana | http://localhost:3000 | Login `admin` / `devnet`; **Vertix** dashboard |
| Prometheus | http://localhost:9095 | Targets `cometbft`, `feeders`, `hermes` healthy |

Chain definition for the explorer: [`infra/explorer/chains/vertix.json`](../infra/explorer/chains/vertix.json) (RPC `http://localhost:26657`, REST `http://localhost:1317`).

Example faucet drip (cosmjs faucet API):

```bash
curl -s -X POST http://localhost:8000/credit -H 'Content-Type: application/json' \
  -d '{"address":"vtx1..."}' | jq .
```

Monitoring config: [`infra/monitoring/prometheus.yml`](../infra/monitoring/prometheus.yml), Grafana provisioning under [`infra/monitoring/grafana/`](../infra/monitoring/grafana/).

```bash
curl -s http://localhost:9095/api/v1/targets | jq -r '.data.activeTargets[] | "\(.labels.job) \(.health)"' | sort -u
curl -s 'http://localhost:3000/api/search?query=Vertix' | jq -r '.[].title'
```

Feeder metrics (host): http://localhost:9201/metrics (and `:9202`, `:9203`).

---

## Troubleshooting

### Image pull / tag failures

Pinned tags live in [`infra/devnet/.env`](../infra/devnet/.env). If `docker compose pull` fails for explorer or faucet, substitute a working tag in `.env` and rebuild:

```bash
docker pull ghcr.io/ping-pub/explorer:latest   # or alternate tag from Task 9 notes
make devnet-docker-build
make localnet-reset
```

### Validators not peering / no blocks

```bash
docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml ps
docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml logs vertix-val1 --tail 50
cat infra/devnet/.gen/persistent_peers.txt
```

Re-run genesis if `.gen` is stale: `make localnet-reset`.

### Feeders not submitting prices

1. Run `./scripts/devnet/setup-feeders.sh` and confirm txs committed.
2. Check authorization: `vtx query oracle feeder <valoper>`.
3. Feeder logs: `docker compose … logs feeder1`.
4. Default config uses `providers: ["static"]` (offline-safe). For live prices, add `coingecko` / `binance` in feeder YAML and ensure egress from the container.
5. Metrics: `curl -s http://localhost:9201/metrics | grep feeds_total`.

### IBC channel not opening

- gaia must reach height: `wait_height http://localhost:26757 5 180`
- Re-run `./scripts/devnet/setup-ibc.sh`
- Hermes logs: `docker compose … logs hermes`
- Confirm config mount: [`infra/hermes/config.toml`](../infra/hermes/config.toml)

### Smoke / `jq` assertion failures

Inspect raw query JSON and align selectors in `scripts/devnet/smoke.sh`:

```bash
vtx query oracle price VTX:USD -o json | jq .
vtx query rwa asset asset-smoke-1 -o json | jq .
```

Do **not** change `x/*` module code for devnet script drift.

---

## Acceptance gate

Evidence map for Phase 6 ([`full-design-spec.md`](./full-design-spec.md) Phase 6, design spec §10):

| Criterion | How to evidence |
|-----------|-----------------|
| 3 validators stable, producing blocks | `wait_height http://localhost:26657 …`; three valoper accounts bonded |
| Oracle aggregates across validators | `query oracle price VTX:USD` > 0; three feeders authorized |
| Full RWA lifecycle + fees visible | Manual walkthrough or explorer txs; `query rwa asset` → SETTLED |
| Fee burn | `query bank total` uvtx decreases after fee traffic |
| Faucet operational | `POST /credit` on `:8000`; balance increases |
| Live IBC VTX → gaia | `ibc/` voucher on gaia relayer account |
| Deterministic reset | `diff infra/devnet/.gen/persistent_peers.txt` across two `localnet-reset` runs → identical |
| Automated gate green | `make devnet-smoke` → `DEVNET SMOKE: PASS` |
| Monitoring | Prometheus targets up; Grafana Vertix dashboard shows height / feeder / supply |

Teardown:

```bash
make localnet-down
```
