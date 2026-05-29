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
