#!/usr/bin/env bash
# LOCAL VERIFICATION ONLY — auto-generates 3 founder gentxs for make testnet-v2-genesis.
# Uses v2 .gen state from build-genesis-base.sh; outputs to infra/testnet/v2/gentxs/.
# NOT used in the public gentx ceremony workflow.
set -euo pipefail
cd "$(dirname "$0")/../../.."
. scripts/devnet/lib.sh
require docker
set -a; . infra/testnet/v2/.env; . infra/testnet/mnemonics.env; set +a

GEN="infra/testnet/v2/.gen"
GENTX_OUT="infra/testnet/v2/gentxs"
IMG="$VERTIX_IMAGE"
KR="/g/keyring"
vd() { docker run --rm -i -v "$PWD/$GEN:/g" -e HOME=/g "$IMG" vertixd "$@"; }

[ -d "$GEN/founder1/config" ] || die "missing $GEN (run make testnet-v2-genesis-base first)"

log "INTERNAL ONLY — generating 3 founder gentxs for local verification"
mkdir -p "$GENTX_OUT"
rm -f "$GENTX_OUT"/gentx-*.json

for i in 1 2 3; do
  vh="/g/founder$i"
  [ "$i" = "1" ] || vd init "founder$i" --chain-id "$CHAIN_ID" --default-denom "$DENOM" --home "$vh" >/dev/null 2>&1
  [ "$i" = "1" ] || cp "$GEN/founder1/config/genesis.json" "$GEN/founder$i/config/genesis.json"
  vd genesis gentx "founder$i" 1000000000000uvtx \
    --chain-id "$CHAIN_ID" --keyring-backend test --keyring-dir "$KR" \
    --home "$vh" --output-document "/g/gentx-founder$i.json"
  cp "$GEN/gentx-founder$i.json" "$GENTX_OUT/gentx-founder$i.json"
done

log "gentxs written to $GENTX_OUT"
