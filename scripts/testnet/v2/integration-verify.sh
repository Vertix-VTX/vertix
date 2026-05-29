#!/usr/bin/env bash
# Phase 9 Task 8 — v2 founder stack gate: up → invariants → upgrade → invariants → down.
# Uses sudo for Docker when the current user cannot access /var/run/docker.sock.
set -euo pipefail
cd "$(dirname "$0")/../../.."
. scripts/devnet/lib.sh
. scripts/devnet/check-crisis-invariants.sh

require docker jq curl bc make

export GIT_CONFIG_COUNT=1
export GIT_CONFIG_KEY_0=safe.directory
export GIT_CONFIG_VALUE_0="$PWD"

sudo_make() { sudo -E make "$@"; }

V2_DIR="infra/testnet/v2"
set -a
. "$V2_DIR/.env"
. infra/testnet/mnemonics.env
set +a

NET="vertix-testnet-v2"
RPC="tcp://sentry1:26657"
IMG="$VERTIX_IMAGE"

export VTX_NET="$NET"
export VTX_RPC="$RPC"
export VTX_IMG="$IMG"
export VTX_GENESIS="$PWD/$V2_DIR/genesis/genesis.json"
export VTX_CHAIN_ID="$CHAIN_ID"

log "Starting v2 founder stack"
sudo_make testnet-v2-up

log "Waiting for block production on sentry1 (up to ~120s)"
H=0
for _ in $(seq 1 60); do
  H=$(sudo docker run --rm --network "$NET" "$IMG" \
    sh -c "curl -sf http://sentry1:26657/status" 2>/dev/null \
    | jq -r '.result.sync_info.latest_block_height // "0"') || H=0
  H=${H:-0}
  if [ "$(echo "$H > 0" | bc -l)" = "1" ]; then
    log "latest_block_height=$H"
    break
  fi
  sleep 2
done
assert_gt "$H" "0" "chain producing blocks (height=$H)"

log "Waiting for oracle feeders (~30s)"
sleep 30

log "Pre-upgrade crisis invariant checks"
check_crisis_invariants

log "Cosmovisor upgrade E2E"
sudo_make testnet-v2-upgrade

log "Post-upgrade crisis invariant checks"
check_crisis_invariants

log "Tearing down"
sudo_make testnet-v2-down

log "===================="
log "TESTNET V2 INTEGRATION VERIFY: PASS"
log "===================="
