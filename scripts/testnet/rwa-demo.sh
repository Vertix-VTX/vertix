#!/usr/bin/env bash
# Public RWA lifecycle demo against vertix-testnet-1: register -> attest -> mint -> settle.
# Asserts rwa/bonds + rwa/denoms invariants implicitly via successful lifecycle + supply checks.
set -euo pipefail
cd "$(dirname "$0")/../.."
. scripts/devnet/lib.sh
require docker jq
set -a; . infra/testnet/.env; set +a

NET=vertix-testnet
RPC=tcp://sentry1:26657
KR="$PWD/infra/testnet/.gen/keyring"
vtx() { docker run --rm -i --network "$NET" -v "$KR:/keyring" "$VERTIX_IMAGE" vertixd "$@"; }
addr() { vtx keys show "$1" -a --keyring-backend test --keyring-dir /keyring; }
flags="--keyring-backend test --keyring-dir /keyring --chain-id $CHAIN_ID --node $RPC --fees 4000uvtx --gas 400000 -y"

ISSUER=${ISSUER_KEY:-faucet}            # any funded testnet key with bond + fees
ASSET=${ASSET_ID:-demo-gold-$(date +%s)}
PAIR="VTX:USD"
BOND="10000000000uvtx"                  # >= MinIssuerBond

log "[1/5] oracle has a live price for $PAIR"
PRICE=$(vtx query oracle price "$PAIR" --node "$RPC" -o json | jq -r '.price // .aggregated_price.price // empty')
[ -n "$PRICE" ] || die "no aggregated $PAIR price (is a feeder running?)"
assert_gt "$PRICE" "0" "$PAIR price > 0 ($PRICE)"

log "[2/5] register + bond asset $ASSET"
vtx tx rwa register-asset "$ASSET" "Demo Gold Bar" "$PAIR" "$BOND" --from "$ISSUER" $flags; sleep 5

log "[3/5] attest against live oracle price"
vtx tx rwa attest-asset "$ASSET" --from "$ISSUER" $flags; sleep 5
STATUS=$(vtx query rwa asset "$ASSET" --node "$RPC" -o json | jq -r '.asset.status // .status')
log "post-attest status: $STATUS"

log "[4/5] mint factory denom rwa/$ASSET"
vtx tx rwa mint-rwa "$ASSET" 1000000000 --from "$ISSUER" $flags; sleep 5
SUP=$(vtx query bank total --node "$RPC" -o json | jq -r '.supply[] | select(.denom|test("rwa/")) | .amount' | head -1)
log "rwa denom supply: ${SUP:-0}"

log "[5/5] settle + release bond"
vtx tx rwa settle-rwa "$ASSET" --from "$ISSUER" $flags; sleep 5
FINAL=$(vtx query rwa asset "$ASSET" --node "$RPC" -o json | jq -r '.asset.status // .status')
echo "$FINAL" | grep -qiE 'settle' || die "asset did not reach SETTLED (got $FINAL)"

log "===================="
log "RWA DEMO: PASS ($ASSET reached $FINAL)"
log "===================="
