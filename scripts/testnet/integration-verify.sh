#!/usr/bin/env bash
# Phase 8 Task 18 — end-to-end founder stack verification (promtool + up + demos + down).
# Uses sudo for Docker when the current user cannot access /var/run/docker.sock.
set -euo pipefail
cd "$(dirname "$0")/../.."
. scripts/devnet/lib.sh

# sudo make runs git as root; allow this repo without touching global git config.
export GIT_CONFIG_COUNT=1
export GIT_CONFIG_KEY_0=safe.directory
export GIT_CONFIG_VALUE_0="$PWD"

sudo_make() { sudo -E make "$@"; }

log "Prometheus config (promtool)"
# Mirror docker-compose.public.yml volume layout (/etc/prometheus/*).
sudo docker run --rm --entrypoint promtool \
  -v "$PWD/infra/testnet/monitoring/prometheus.yml:/etc/prometheus/prometheus.yml:ro" \
  -v "$PWD/infra/testnet/monitoring/alert-rules.yml:/etc/prometheus/alert-rules.yml:ro" \
  prom/prometheus:v2.53.0 \
  check config /etc/prometheus/prometheus.yml

log "Starting founder stack"
sudo_make testnet-up

log "Waiting for block production (~30s)"
sleep 30
H=$(sudo docker run --rm --network vertix-testnet vertix:testnet \
  sh -c "curl -s http://sentry1:26657/status" | jq -r '.result.sync_info.latest_block_height')
log "latest_block_height=$H"
assert_gt "$H" "0" "chain producing blocks (height=$H)"

log "Join smoke"
sudo_make testnet-join-smoke

log "RWA happy-path demo"
sudo_make testnet-demo

log "RWA dispute demo (~305s voting period)"
sudo ./scripts/testnet/rwa-dispute-demo.sh

log "Tearing down"
sudo_make testnet-down

log "===================="
log "TESTNET INTEGRATION VERIFY: PASS"
log "===================="
