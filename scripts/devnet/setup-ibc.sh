#!/usr/bin/env bash
# Import relayer keys into Hermes and open an ICS-20 transfer channel: vertix <-> gaia.
set -euo pipefail
cd "$(dirname "$0")/../.."
. scripts/devnet/lib.sh
set -a; . infra/devnet/.env; . infra/devnet/mnemonics.env; set +a

H() { docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml exec -T hermes "$@"; }

log "Writing relayer mnemonics into hermes container"
H sh -c "printf '%s' '$MNEMONIC_RELAYER' > /tmp/vtx.mn && printf '%s' '$MNEMONIC_RELAYER' > /tmp/gaia.mn"
H hermes keys add --chain vertix-devnet-1 --mnemonic-file /tmp/vtx.mn --key-name relayer --overwrite
H hermes keys add --chain gaia-devnet-1   --mnemonic-file /tmp/gaia.mn --key-name relayer --overwrite

log "Creating ICS-20 channel (transfer/transfer, unordered)"
H hermes create channel --a-chain vertix-devnet-1 --b-chain gaia-devnet-1 \
  --a-port transfer --b-port transfer --new-client-connection --yes

log "IBC channel setup complete"
H hermes query channels --chain vertix-devnet-1
