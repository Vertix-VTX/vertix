#!/usr/bin/env bash
set -euo pipefail

HOME_DIR="${1:-/tmp/vtx-smoke}"
BIN="build/vertixd"
CHAIN_ID="vertix-devnet-1"

make build
rm -rf "$HOME_DIR"
"$BIN" init smoke --chain-id "$CHAIN_ID" --default-denom uvtx --home "$HOME_DIR"
"$BIN" genesis validate-genesis --home "$HOME_DIR"

"$BIN" start --home "$HOME_DIR" \
  --rpc.laddr tcp://127.0.0.1:26657 \
  --grpc.address 127.0.0.1:9090 \
  --api.enable --api.address tcp://127.0.0.1:1317 &
PID=$!
sleep 12

curl -sf http://127.0.0.1:26657/status >/dev/null && echo "RPC :26657 OK"
curl -sf http://127.0.0.1:1317/cosmos/base/tendermint/v1beta1/node_info >/dev/null && echo "REST :1317 OK"
HEIGHT=$(curl -s http://127.0.0.1:26657/status | grep -o '"latest_block_height":"[0-9]*"' | grep -o '[0-9]*')
echo "block height: $HEIGHT"
kill "$PID"
[ "${HEIGHT:-0}" -gt 0 ] && echo "BOOT SMOKE PASS" || { echo "BOOT SMOKE FAIL"; exit 1; }
