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
