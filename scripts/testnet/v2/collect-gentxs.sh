#!/usr/bin/env bash
# Merge gentx JSONs into final genesis for vertix-testnet-2.
# Usage: ./collect-gentxs.sh [gentx-dir]
# Default gentx-dir: infra/testnet/v2/gentxs
#
# Prerequisites: base genesis from make testnet-v2-genesis-base
#   BASE=infra/testnet/v2/genesis/base/base-genesis.json
#
# Founder gentx helper (submit gentx-<moniker>.json to the coordinator):
#   vertixd genesis gentx "$MONIKER" 1000000000000uvtx \
#     --chain-id vertix-testnet-2 \
#     --home "$HOME/.vertixd" \
#     --output-document "gentx-$MONIKER.json"
set -euo pipefail
cd "$(dirname "$0")/../../.."
. scripts/devnet/lib.sh
require docker
set -a; . infra/testnet/v2/.env; set +a

GENTX_DIR="${1:-infra/testnet/v2/gentxs}"
BASE="infra/testnet/v2/genesis/base/base-genesis.json"
OUT="infra/testnet/v2/genesis"
GEN="infra/testnet/v2/.gen"
WORK="$GEN/collect"
IMG="$VERTIX_IMAGE"
vd() { docker run --rm -i -v "$PWD/$GEN:/g" -e HOME=/g "$IMG" vertixd "$@"; }

[ -f "$BASE" ] || die "missing base genesis: $BASE (run make testnet-v2-genesis-base first)"

shopt -s nullglob
gentxs=( "$GENTX_DIR"/gentx-*.json )
[ "${#gentxs[@]}" -gt 0 ] || die "no gentx-*.json in $GENTX_DIR"

log "Collecting ${#gentxs[@]} gentx(s) from $GENTX_DIR"
rm -rf "$WORK"
mkdir -p "$WORK/config/gentx" "$OUT"

vd init collect --chain-id "$CHAIN_ID" --default-denom "$DENOM" --home /g/collect >/dev/null 2>&1
cp "$BASE" "$WORK/config/genesis.json"

while IFS= read -r f; do
  cp "$f" "$WORK/config/gentx/"
done < <(printf '%s\n' "${gentxs[@]}" | sort)

vd genesis collect-gentxs --home /g/collect --gentx-dir /g/collect/config/gentx
vd genesis validate-genesis --home /g/collect

cp "$WORK/config/genesis.json" "$OUT/genesis.json"
( cd "$OUT" && sha256sum genesis.json | tee genesis.sha256 )
log "genesis published: $OUT/genesis.json"
( cd "$OUT" && cat genesis.sha256 )
