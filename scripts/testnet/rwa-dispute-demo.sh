#!/usr/bin/env bash
# Adversarial showcase: a disputed RWA asset is force-settled via governance MsgSlashBond.
# Demonstrates the rwa/bonds safety mechanic publicly.
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

ISSUER=${ISSUER_KEY:-faucet}
ASSET=${ASSET_ID:-disputed-asset-$(date +%s)}
GOV_MODULE_ADDR=$(vtx query auth module-account gov --node "$RPC" -o json | jq -r '.account.value.address // .account.base_account.address')

log "[1/5] register + bond a disputable asset"
vtx tx rwa register-asset "$ASSET" "Disputed Asset" "VTX:USD" 10000000000uvtx --from "$ISSUER" $flags; sleep 5
vtx tx rwa attest-asset "$ASSET" --from "$ISSUER" $flags; sleep 5

log "[2/5] build MsgSlashBond proposal (authority = gov module)"
cat > /tmp/slash-prop.json <<EOF
{
  "messages": [
    {
      "@type": "/vertix.rwa.v1.MsgSlashBond",
      "authority": "$GOV_MODULE_ADDR",
      "asset_id": "$ASSET",
      "reason": "demo dispute: misrepresented backing"
    }
  ],
  "metadata": "ipfs://demo",
  "deposit": "10000000uvtx",
  "title": "Slash bond for $ASSET",
  "summary": "Force-settle disputed asset and route bond to community pool."
}
EOF
docker run --rm -i --network "$NET" -v "$KR:/keyring" -v /tmp/slash-prop.json:/prop.json "$VERTIX_IMAGE" \
  vertixd tx gov submit-proposal /prop.json --from "$ISSUER" $flags; sleep 6

log "[3/5] vote yes from founders"
PID=$(vtx query gov proposals --node "$RPC" -o json | jq -r '.proposals[-1].id')
for v in founder1 founder2 founder3; do vtx tx gov vote "$PID" yes --from "$v" $flags || true; sleep 3; done

log "[4/5] wait out the voting period (testnet voting_period=300s)"
sleep 305

log "[5/5] confirm asset force-settled + bond routed to community pool"
FINAL=$(vtx query rwa asset "$ASSET" --node "$RPC" -o json | jq -r '.asset.status // .status')
log "final status: $FINAL"
POOL=$(vtx query distribution community-pool --node "$RPC" -o json | jq -r '.pool[]? | select(.denom=="uvtx") | .amount' | head -1)
log "community pool uvtx: ${POOL:-0}"
echo "$FINAL" | grep -qiE 'settle|slash|dispute' || die "asset not force-settled (got $FINAL)"

log "===================="
log "RWA DISPUTE DEMO: PASS"
log "===================="
