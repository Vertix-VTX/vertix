#!/usr/bin/env bash
# Prepare infra/testnet/v2/.gen for the founder-only Docker Compose stack.
# Requires final genesis at infra/testnet/v2/genesis/genesis.json (after collect-gentxs).
set -euo pipefail
cd "$(dirname "$0")/../../.."
. scripts/devnet/lib.sh
require docker
set -a; . infra/testnet/v2/.env; . infra/testnet/mnemonics.env; set +a

GEN="infra/testnet/v2/.gen"
FINAL="infra/testnet/v2/genesis/genesis.json"
IMG="$VERTIX_IMAGE"
KR="/g/keyring"
vd() { docker run --rm -i -v "$PWD/$GEN:/g" -e HOME=/g "$IMG" vertixd "$@"; }

[ -f "$FINAL" ] || die "missing $FINAL (run make testnet-v2-genesis first)"
[ -d "$GEN/keyring" ] || die "missing $GEN/keyring (run make testnet-v2-genesis-base first)"

log "Preparing v2 Docker stack configs (chain_id=$CHAIN_ID)"

# Ensure founder homes exist and share the collected genesis.
for i in 1 2 3; do
  vh="/g/founder$i"
  if [ "$i" != "1" ]; then
    vd init "founder$i" --chain-id "$CHAIN_ID" --default-denom "$DENOM" --home "$vh" >/dev/null 2>&1 || true
  fi
  docker run --rm -i -v "$PWD/$GEN:/g" -v "$PWD/$FINAL:/final/genesis.json:ro" "$IMG" \
    cp /final/genesis.json "/g/founder$i/config/genesis.json"
done

ID1=$(vd tendermint show-node-id --home /g/founder1)
ID2=$(vd tendermint show-node-id --home /g/founder2)
ID3=$(vd tendermint show-node-id --home /g/founder3)
PEERS="$ID1@founder1:26656,$ID2@founder2:26656,$ID3@founder3:26656"
log "persistent_peers = $PEERS"

patch_cfg() { # patch_cfg <home-dir-name> <public-rpc-0|1> <enable-grpc-for-feeders-0|1>
  local home=$1 public_rpc=$2 grpc_feeder=$3
  local ct="/g/$home/config/config.toml" at="/g/$home/config/app.toml"
  docker run --rm -i -v "$PWD/$GEN:/g" "$IMG" sh -c "
    sed -i 's|^persistent_peers = .*|persistent_peers = \"$PEERS\"|' $ct
    sed -i 's|^pex = .*|pex = true|' $ct
    sed -i 's|^prometheus = false|prometheus = true|' $ct
    sed -i 's|^prometheus_listen_addr = \":26660\"|prometheus_listen_addr = \"0.0.0.0:26660\"|' $ct
    sed -i 's|^minimum-gas-prices = \"\"|minimum-gas-prices = \"0.025uvtx\"|' $at
    sed -i '/^\[api\]/,/^\[/ s|^enable = false|enable = true|' $at
    if [ \"$public_rpc\" = \"1\" ]; then
      sed -i 's|^laddr = \"tcp://127.0.0.1:26657\"|laddr = \"tcp://0.0.0.0:26657\"|' $ct
      sed -i 's|^address = \"tcp://localhost:1317\"|address = \"tcp://0.0.0.0:1317\"|' $at
      sed -i '/^\[grpc\]/,/^\[/ s|^address = \"localhost:9090\"|address = \"0.0.0.0:9090\"|' $at
    else
      sed -i 's|^laddr = \"tcp://0.0.0.0:26657\"|laddr = \"tcp://127.0.0.1:26657\"|' $ct
      sed -i 's|^address = \"tcp://0.0.0.0:1317\"|address = \"tcp://127.0.0.1:1317\"|' $at
      sed -i '/^\[grpc\]/,/^\[/ s|^address = \"0.0.0.0:9090\"|address = \"127.0.0.1:9090\"|' $at
    fi
    if [ \"$grpc_feeder\" = \"1\" ]; then
      sed -i '/^\[grpc\]/,/^\[/ s|^address = \"127.0.0.1:9090\"|address = \"0.0.0.0:9090\"|' $at
    fi
  "
}

vaddr() { vd keys show "$1" --bech val -a --keyring-backend test --keyring-dir "$KR"; }

for i in 1 2 3; do patch_cfg "founder$i" "0" "1"; done

log "Building sentry + seed full-node configs (internal RPC — loopback only)"
for role in sentry1 sentry2 sentry3 seed; do
  vd init "$role" --chain-id "$CHAIN_ID" --default-denom "$DENOM" --home "/g/$role" >/dev/null 2>&1 || true
  docker run --rm -i -v "$PWD/$GEN:/g" -v "$PWD/$FINAL:/final/genesis.json:ro" "$IMG" \
    cp /final/genesis.json "/g/$role/config/genesis.json"
done
for role in sentry1 sentry2 sentry3; do patch_cfg "$role" "0" "0"; done
docker run --rm -i -v "$PWD/$GEN:/g" "$IMG" sh -c "
  sed -i 's|^seed_mode = .*|seed_mode = true|' /g/seed/config/config.toml
  sed -i 's|^moniker = .*|moniker = \"seed\"|' /g/seed/config/config.toml
  sed -i 's|^moniker = .*|moniker = \"sentry1\"|' /g/sentry1/config/config.toml
  sed -i 's|^moniker = .*|moniker = \"sentry2\"|' /g/sentry2/config/config.toml
  sed -i 's|^moniker = .*|moniker = \"sentry3\"|' /g/sentry3/config/config.toml
"
patch_cfg "seed" "0" "0"

log "Rendering feeder configs (node-kit chain_id=$CHAIN_ID)"
mkdir -p "$GEN/feeder"
for i in 1 2 3; do
  vo=$(vaddr "founder$i")
  sed -e "s/vertix-devnet-1/$CHAIN_ID/" \
      -e "s/vertix-val$i/founder$i/" \
      -e "s/VALOPER${i}_PLACEHOLDER/$vo/" \
    "infra/devnet/feeder/feeder$i.yaml" > "$GEN/feeder/feeder$i.yaml"
done
cp -r "$GEN/keyring" "$GEN/feeder/keyring"

SEED_ID=$(vd tendermint show-node-id --home /g/seed)
mkdir -p infra/testnet/v2/genesis
echo "$PEERS" > infra/testnet/v2/genesis/persistent_peers.txt
echo "$SEED_ID@seed:26656" > infra/testnet/v2/genesis/seeds.txt
log "v2 compose .gen ready under $GEN"
