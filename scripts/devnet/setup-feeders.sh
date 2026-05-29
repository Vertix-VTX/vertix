#!/usr/bin/env bash
# Authorize each feeder key for its validator via MsgSetFeeder. Run after the chain is live.
set -euo pipefail
cd "$(dirname "$0")/../.."
. scripts/devnet/lib.sh
set -a; . infra/devnet/.env; set +a
KR=/keyring
tx() { docker run --rm -i --network vertix-devnet -v "$PWD/infra/devnet/.gen/keyring:/keyring" "$VERTIX_IMAGE" \
        vertixd tx "$@" --keyring-backend test --keyring-dir "$KR" \
        --chain-id "$CHAIN_ID" --node tcp://vertix-val1:26657 --fees 2000uvtx --gas 300000 -y; }
q()  { docker run --rm -i --network vertix-devnet "$VERTIX_IMAGE" \
        vertixd query "$@" --node tcp://vertix-val1:26657 -o json; }
feeder_addr() { docker run --rm -i -v "$PWD/infra/devnet/.gen/keyring:/keyring" "$VERTIX_IMAGE" \
        vertixd keys show "$1" -a --keyring-backend test --keyring-dir /keyring; }

for i in 1 2 3; do
  fa=$(feeder_addr "feeder$i")
  log "set-feeder: val$i -> $fa"
  tx oracle set-feeder "$fa" --from "val$i"
  sleep 3
done
log "feeder authorization submitted"
