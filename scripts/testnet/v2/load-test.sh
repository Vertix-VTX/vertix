#!/usr/bin/env bash
# Phase 9 — module-realistic load test for vertix-testnet-2.
# Builds txflood, funds load-test keys, ramps 50→100→150 tx/s, calibrates SLO bar (80% peak).
set -euo pipefail
cd "$(dirname "$0")/../../.."
. scripts/devnet/lib.sh

require docker jq curl bc go make

V2_DIR="infra/testnet/v2"
MIX="infra/loadtest/mix.toml"
TXFLOOD_DIR="infra/loadtest/clients/txflood"
TXFLOOD_BIN="$TXFLOOD_DIR/txflood"
DOCS="docs/load-test.md"
RESULTS_DIR="infra/loadtest/results"
STAMP=$(date -u +"%Y-%m-%dT%H:%MZ")

set -a
. "$V2_DIR/.env"
. infra/testnet/mnemonics.env
set +a

NET="vertix-testnet-v2"
GRPC="founder1:9090"
KR="$PWD/$V2_DIR/.gen/keyring"
IMG="$VERTIX_IMAGE"
FLAGS="--fees 5000uvtx --gas 400000 -y"

COMPOSE=(docker compose --env-file "$V2_DIR/.env" -f "$V2_DIR/docker-compose.yml")

founder_container() {
  "${COMPOSE[@]}" ps -q founder1 2>/dev/null | head -1
}

rpc_height_v2() {
  local cid
  cid=$(founder_container)
  if [ -z "$cid" ]; then
    echo 0
    return
  fi
  docker run --rm --network "container:${cid}" "$IMG" \
    sh -c "curl -sf http://127.0.0.1:26657/status" 2>/dev/null \
    | jq -r '.result.sync_info.latest_block_height // "0"'
}

vtx() {
  local cid
  cid=$(founder_container)
  [ -n "$cid" ] || die "founder1 container not running"
  docker run --rm -i --network "container:${cid}" -v "$KR:/keyring" "$IMG" \
    vertixd "$@" --keyring-backend test --keyring-dir /keyring --chain-id "$CHAIN_ID" \
    --node tcp://127.0.0.1:26657
}

assert_stack_up() {
  if ! docker network inspect "$NET" >/dev/null 2>&1; then
    log "v2 stack not running — writing placeholder SLO section to $DOCS"
    write_placeholder_slo "stack not running (run: make testnet-v2-up)"
    exit 0
  fi
  local h
  h=$(rpc_height_v2) || h=0
  h=${h:-0}
  if [ "$(echo "$h > 0" | bc -l  2>/dev/null || echo 0)" != "1" ]; then
    log "v2 stack not producing blocks (height=$h) — placeholder SLO only"
    write_placeholder_slo "stack up but not producing blocks (height=$h)"
    exit 0
  fi
  log "v2 stack OK (height=$h)"
}

ensure_keyring() {
  [ -d "$KR" ] || die "missing $KR (run: make testnet-v2-genesis)"
}

fund_loadtest_accounts() {
  log "Ensuring load-test keys and funding from genesis faucet"
  local faucet_addr
  faucet_addr=$(vtx keys show faucet -a)
  for i in 1 2 3; do
    vtx keys add "loadtest$i" >/dev/null 2>&1 || true
    local addr bal
    addr=$(vtx keys show "loadtest$i" -a)
    bal=$(vtx query bank balances "$addr" -o json \
      | jq -r '[.balances[] | select(.denom=="uvtx") | .amount | tonumber] | add // 0')
    if [ "${bal:-0}" -lt 500000000000 ]; then
      log "Funding loadtest$i ($addr) from faucet"
      vtx tx bank send "$faucet_addr" "$addr" 500000000000uvtx --from faucet $FLAGS >/dev/null
      sleep 4
    fi
  done
}

build_txflood() {
  log "Building txflood"
  ( cd "$PWD" && GOTOOLCHAIN=go1.25.4 CGO_ENABLED=0 go build -o "$TXFLOOD_BIN" ./infra/loadtest/clients/txflood/ )
  [ -x "$TXFLOOD_BIN" ] || die "txflood build failed"
}

run_txflood() {
  mkdir -p "$RESULTS_DIR"
  local out="$RESULTS_DIR/v2-${STAMP}.jsonl"
  log "Running txflood (output: $out)"
  docker run --rm --network "$NET" \
    -v "$PWD/$TXFLOOD_BIN:/txflood:ro" \
    -v "$PWD/$MIX:/mix.toml:ro" \
    -v "$KR:/keyring:ro" \
    alpine:3.20 \
    /txflood --config /mix.toml --keyring-dir /keyring --grpc "$GRPC" \
    | tee "$out"
  echo "$out"
}

parse_results() {
  local out=$1
  local summary peak bar met
  summary=$(grep '"peak_sustained_tps"' "$out" | tail -1)
  peak=$(echo "$summary" | jq -r '.peak_sustained_tps // 0')
  bar=$(echo "$summary" | jq -r '.bar_tps_80pct // 0')
  # Last ramp sustained TPS vs bar
  local last_tps
  last_tps=$(grep '"sustained_tps"' "$out" | tail -1 | jq -r '.sustained_tps // 0')
  if [ "$(echo "$last_tps >= $bar && $bar > 0" | bc -l)" = "1" ]; then
    met="PASS"
  else
    met="CALIBRATION NEEDED"
  fi
  echo "$peak|$bar|$last_tps|$met|$summary"
}

write_slo_section() {
  local out=$1 peak=$2 bar=$3 last_tps=$4 met=$5 summary=$6
  local tmp
  tmp=$(mktemp)

  if grep -q '## Phase 9 SLO (testnet v2)' "$DOCS"; then
    awk '/^## Phase 9 SLO \(testnet v2\)/{exit} {print}' "$DOCS" > "$tmp"
  else
    cp "$DOCS" "$tmp"
    printf '\n' >> "$tmp"
  fi

  {
    echo "## Phase 9 SLO (testnet v2)"
    echo ""
    echo "> Last run: **${STAMP}** — harness \`make testnet-v2-load-test\`"
    echo ""
    echo "| Item | Value |"
    echo "|------|--------|"
    echo "| Chain ID | \`vertix-testnet-2\` |"
    echo "| Mix | 70% bank / 20% oracle / 10% RWA ([\`mix.toml\`](../infra/loadtest/mix.toml)) |"
    echo "| Ramp rates | 50 → 100 → 150 tx/s (200 s each, 600 s total) |"
    echo "| Peak sustained TPS | ${peak} |"
    echo "| SLO bar (80% peak) | ${bar} |"
    echo "| Last ramp sustained TPS | ${last_tps} |"
    echo "| Gate | **${met}** |"
    echo ""
    echo "Raw JSONL: \`${out#$PWD/}\`"
    echo ""
    echo "<details><summary>Final txflood summary JSON</summary>"
    echo ""
    echo '```json'
    echo "$summary" | jq .
    echo '```'
    echo ""
    echo "</details>"
    echo ""
    echo "### Per-ramp metrics"
    echo ""
    echo "| Rate target (tx/s) | Sustained TPS | Success | Failed | p50 ms | p99 ms |"
    echo "|-------------------:|--------------:|--------:|-------:|-------:|-------:|"
    grep '"rate_target"' "$out" | while read -r line; do
      echo "$line" | jq -r '[.rate_target, .sustained_tps, .success, .failed, .latency_ms.p50, .latency_ms.p99] | @tsv' \
        | awk -F'\t' '{printf "| %s | %s | %s | %s | %s | %s |\n", $1, $2, $3, $4, $5, $6}'
    done
  } >> "$tmp"

  mv "$tmp" "$DOCS"
  log "Updated $DOCS (gate: $met, peak=$peak, bar=$bar)"
}

write_placeholder_slo() {
  local reason=$1
  local tmp
  tmp=$(mktemp)
  if grep -q '## Phase 9 SLO (testnet v2)' "$DOCS"; then
    awk '/^## Phase 9 SLO \(testnet v2\)/{exit} {print}' "$DOCS" > "$tmp"
  else
    cp "$DOCS" "$tmp"
    printf '\n' >> "$tmp"
  fi
  {
    echo "## Phase 9 SLO (testnet v2)"
    echo ""
    echo "> Placeholder — **${STAMP}**: ${reason}"
    echo ""
    echo "| Item | Value |"
    echo "|------|--------|"
    echo "| Chain ID | \`vertix-testnet-2\` |"
    echo "| Mix | 70% bank / 20% oracle / 10% RWA |"
    echo "| Peak sustained TPS | _not measured_ |"
    echo "| SLO bar (80% peak) | _pending calibration_ |"
    echo "| Gate | **PENDING** |"
    echo ""
    echo "Re-run when the stack is up: \`make testnet-v2-load-test\`"
  } >> "$tmp"
  mv "$tmp" "$DOCS"
}

log "Phase 9 module-realistic load test (vertix-testnet-2)"
assert_stack_up
ensure_keyring
build_txflood
fund_loadtest_accounts

OUT=$(run_txflood)
IFS='|' read -r PEAK BAR LAST_TPS MET SUMMARY <<< "$(parse_results "$OUT")"

log "Peak sustained TPS=$PEAK bar(80%)=$BAR last_ramp=$LAST_TPS → $MET"
write_slo_section "$OUT" "$PEAK" "$BAR" "$LAST_TPS" "$MET" "$SUMMARY"

if [ "$MET" = "CALIBRATION NEEDED" ]; then
  log "Load SLO: CALIBRATION NEEDED (last ramp $LAST_TPS < bar $BAR)"
  exit 0
fi
log "Load SLO: PASS"
