#!/usr/bin/env bash
# Phase 8 Task 18 — end-to-end founder stack verification (promtool + up + demos + down).
# Uses sudo for Docker when the current user cannot access /var/run/docker.sock.
set -euo pipefail
cd "$(dirname "$0")/../.."
. scripts/devnet/lib.sh

log "Prometheus config (promtool)"
sudo docker run --rm -v "$PWD/infra/testnet/monitoring:/m" prom/prometheus:v2.53.0 \
  promtool check config /m/prometheus.yml

log "Starting founder stack"
sudo make testnet-up

log "Waiting for block production (~30s)"
sleep 30
H=$(sudo docker run --rm --network vertix-testnet vertix:testnet \
  sh -c "curl -s http://sentry1:26657/status" | jq -r '.result.sync_info.latest_block_height')
log "latest_block_height=$H"
assert_gt "$H" "0" "chain producing blocks (height=$H)"

log "Join smoke"
sudo make testnet-join-smoke

log "RWA happy-path demo"
sudo make testnet-demo

log "RWA dispute demo (~305s voting period)"
sudo ./scripts/testnet/rwa-dispute-demo.sh

log "Tearing down"
sudo make testnet-down

log "===================="
log "TESTNET INTEGRATION VERIFY: PASS"
log "===================="
