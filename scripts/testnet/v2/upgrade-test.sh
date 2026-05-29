#!/usr/bin/env bash
# Phase 9 — Cosmovisor upgrade E2E on vertix-testnet-2 (founder-only v2 stack).
# Prerequisites: make testnet-v2-up (stack running); build/vertixd present.
#
# Cosmovisor binary placement (each validator / full node host):
#   mkdir -p "$DAEMON_HOME/cosmovisor/upgrades/v0.2.0-testnet/bin"
#   cp build/vertixd "$DAEMON_HOME/cosmovisor/upgrades/v0.2.0-testnet/bin/vertixd"
#
# Docker-internal rehearsal: infra/testnet/v2/.upgrade-bin/vertixd is bind-mounted into
#   $DAEMON_HOME/cosmovisor/upgrades/v0.2.0-testnet/bin/vertixd before the upgrade height.
set -euo pipefail
cd "$(dirname "$0")/../../.."
. scripts/devnet/lib.sh

require docker jq curl bc make

V2_DIR="infra/testnet/v2"
UPGRADE_NAME="v0.2.0-testnet"
# Upgrade plan height = H + UPGRADE_LEAD_BLOCKS (default 50 per Phase 9 spec).
UPGRADE_LEAD_BLOCKS="${UPGRADE_LEAD_BLOCKS:-50}"
EXPEDITED_VOTING_SEC="${EXPEDITED_VOTING_SEC:-125}"

set -a
. "$V2_DIR/.env"
. infra/testnet/mnemonics.env
set +a

NET="vertix-testnet-v2"
RPC="tcp://sentry1:26657"
KR="$PWD/$V2_DIR/.gen/keyring"
IMG="$VERTIX_IMAGE"

vtx() {
  docker run --rm -i --network "$NET" -v "$KR:/keyring" "$IMG" vertixd "$@"
}

rpc_height_v2() {
  docker run --rm --network "$NET" "$IMG" \
    sh -c "curl -sf http://sentry1:26657/status" | jq -r '.result.sync_info.latest_block_height // "0"'
}

wait_height_v2() {
  local min=$1 timeout=${2:-600} i=0 h
  while :; do
    h=$(rpc_height_v2); h=${h:-0}
    [ "$h" -ge "$min" ] && { log "height $h >= $min (sentry1)"; return 0; }
    i=$((i+1)); [ "$i" -ge "$timeout" ] && die "timeout waiting height>=$min at sentry1 (last=$h)"
    sleep 1
  done
}

flags="--keyring-backend test --keyring-dir /keyring --chain-id $CHAIN_ID --node $RPC --fees 5000000uvtx --gas 800000 -y"

stage_upgrade_binary() {
  log "Staging upgrade binary for Cosmovisor (bind-mount + docker compose cp fallback)"
  mkdir -p "$V2_DIR/.upgrade-bin"
  if [ ! -x build/vertixd ]; then
    log "build/vertixd missing — running make build"
    make build
  fi
  cp -f build/vertixd "$V2_DIR/.upgrade-bin/vertixd"
  chmod +x "$V2_DIR/.upgrade-bin/vertixd"
  [ -f "$V2_DIR/.upgrade-bin/vertixd" ] || die "failed to stage $V2_DIR/.upgrade-bin/vertixd"
  if docker network inspect "$NET" >/dev/null 2>&1; then
    local svc dest
    dest="/root/.vertixd/cosmovisor/upgrades/v0.2.0-testnet/bin/vertixd"
    for svc in founder1 founder2 founder3 sentry1 sentry2 sentry3 seed; do
      docker compose --env-file "$V2_DIR/.env" -f "$V2_DIR/docker-compose.yml" \
        cp "$V2_DIR/.upgrade-bin/vertixd" "$svc:$dest" 2>/dev/null \
        || log "note: could not docker compose cp to $svc (may pick up bind-mount on restart)"
    done
  fi
}

assert_stack_running() {
  if ! docker network inspect "$NET" >/dev/null 2>&1; then
    die "docker network $NET not found (run: make testnet-v2-up)"
  fi
  local h
  h=$(docker run --rm --network "$NET" "$IMG" \
    sh -c "curl -sf http://sentry1:26657/status" | jq -r '.result.sync_info.latest_block_height // "0"')
  h=${h:-0}
  assert_gt "$h" "0" "v2 stack producing blocks (height=$h)"
  echo "$h"
}

log "[1/6] Assert v2 stack running"
H=$(assert_stack_running)
log "current height H=$H"

stage_upgrade_binary

log "[2/6] Submit expedited SoftwareUpgrade gov proposal (plan $UPGRADE_NAME)"
UPGRADE_HEIGHT=$((H + UPGRADE_LEAD_BLOCKS))
log "upgrade height = H+$UPGRADE_LEAD_BLOCKS = $UPGRADE_HEIGHT"

GOV_MODULE_ADDR=$(vtx query auth module-account gov --node "$RPC" -o json \
  | jq -r '.account.value.address // .account.base_account.address')

PROP_FILE=$(mktemp)
trap 'rm -f "$PROP_FILE"' EXIT
cat >"$PROP_FILE" <<EOF
{
  "messages": [
    {
      "@type": "/cosmossdk.io.x.upgrade.v1.MsgSoftwareUpgrade",
      "authority": "$GOV_MODULE_ADDR",
      "plan": {
        "name": "$UPGRADE_NAME",
        "height": "$UPGRADE_HEIGHT",
        "info": ""
      }
    }
  ],
  "metadata": "phase9-cosmovisor-rehearsal",
  "deposit": "50000000uvtx",
  "title": "v0.2.0-testnet Cosmovisor rehearsal",
  "summary": "Expedited software upgrade for Phase 9 store-migration drill",
  "expedited": true
}
EOF

docker run --rm -i --network "$NET" -v "$KR:/keyring" -v "$PROP_FILE:/prop.json:ro" "$IMG" \
  vertixd tx gov submit-proposal /prop.json --from founder1 $flags
sleep 6

log "[3/6] Vote yes from all founders (expedited voting_period=120s on testnet)"
PID=$(vtx query gov proposals --node "$RPC" -o json | jq -r '.proposals[-1].id')
[ -n "$PID" ] && [ "$PID" != "null" ] || die "could not read latest gov proposal id"
log "proposal id=$PID"
for v in founder1 founder2 founder3; do
  vtx tx gov vote "$PID" yes --from "$v" $flags || true
  sleep 3
done

log "[4/6] Wait expedited voting period (${EXPEDITED_VOTING_SEC}s)"
sleep "$EXPEDITED_VOTING_SEC"

log "[5/6] Wait for upgrade height $UPGRADE_HEIGHT + 30s"
wait_height_v2 "$UPGRADE_HEIGHT" 600
sleep 30

log "[6/6] Assert chain resumed past upgrade height"
H_AFTER=$(docker run --rm --network "$NET" "$IMG" \
  sh -c "curl -sf http://sentry1:26657/status" | jq -r '.result.sync_info.latest_block_height // "0"')
H_AFTER=${H_AFTER:-0}
assert_gt "$H_AFTER" "$UPGRADE_HEIGHT" "height past upgrade (got $H_AFTER, need > $UPGRADE_HEIGHT)"

log "query fees params (post-upgrade)"
vtx query fees params --node "$RPC" -o json | jq -e '.params.burn_ratio' >/dev/null \
  || die "fees module query failed"

log "===================="
log "TESTNET V2 UPGRADE: PASS (height $H -> $H_AFTER, plan $UPGRADE_NAME @ $UPGRADE_HEIGHT)"
log "===================="
