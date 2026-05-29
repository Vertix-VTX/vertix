#!/usr/bin/env bash
# Full devnet end-to-end smoke gate. Assumes `make localnet-up` + setup-feeders + setup-ibc done,
# or pass --bootstrap to run them. Exits non-zero on the first failed assertion.
set -euo pipefail
cd "$(dirname "$0")/../.."
. scripts/devnet/lib.sh
require docker jq curl bc
set -a; . infra/devnet/.env; set +a

NET=vertix-devnet
KR="$PWD/infra/devnet/.gen/keyring"
RPC=tcp://vertix-val1:26657
vtx()  { docker run --rm -i --network "$NET" -v "$KR:/keyring" "$VERTIX_IMAGE" vertixd "$@"; }
txflags="--keyring-backend test --keyring-dir /keyring --chain-id $CHAIN_ID --node $RPC --fees 4000uvtx --gas 400000 -y"
addr() { vtx keys show "$1" -a --keyring-backend test --keyring-dir /keyring; }

if [ "${1:-}" = "--bootstrap" ]; then
  log "bootstrapping feeders + ibc"
  ./scripts/devnet/setup-feeders.sh
  ./scripts/devnet/setup-ibc.sh
  docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml exec -d hermes hermes start || true
fi

# 1) ORACLE: a configured pair has a non-empty aggregated price.
log "[1/5] oracle price aggregation"
sleep 20
PRICE=$(vtx query oracle price VTX:USD --node "$RPC" -o json | jq -r '.price // .aggregated_price.price // empty')
[ -n "$PRICE" ] || die "no aggregated VTX:USD price"
assert_gt "$PRICE" "0" "VTX:USD price > 0 ($PRICE)"

# 2) RWA lifecycle: register -> attest -> mint -> settle
log "[2/5] rwa lifecycle"
ASSET="asset-smoke-1"
ISSUER=$(addr testuser)
vtx tx rwa register-asset "$ASSET" "Smoke Asset" "VTX:USD" 10000000000uvtx --from testuser $txflags; sleep 4
vtx tx rwa attest-asset "$ASSET" --from testuser $txflags; sleep 4
STATUS=$(vtx query rwa asset "$ASSET" --node "$RPC" -o json | jq -r '.asset.status // .status')
log "post-attest status: $STATUS"
vtx tx rwa mint-rwa "$ASSET" 1000000000 --from testuser $txflags; sleep 4
vtx tx rwa settle-rwa "$ASSET" --from testuser $txflags; sleep 4
FINAL=$(vtx query rwa asset "$ASSET" --node "$RPC" -o json | jq -r '.asset.status // .status')
log "final status: $FINAL"
echo "$FINAL" | grep -qiE 'settle' || die "asset did not reach SETTLED (got $FINAL)"
log "OK: rwa lifecycle reached settled"

# 3) FEES: total supply decreases over a few blocks (EndBlock burn of collected fees).
log "[3/5] fee burn reduces total supply"
SUP1=$(vtx query bank total --node "$RPC" -o json | jq -r '.supply[] | select(.denom=="uvtx") | .amount')
# generate fee traffic
for n in 1 2 3; do vtx tx bank send "$(addr testuser)" "$(addr faucet)" 1000uvtx --from testuser $txflags >/dev/null; sleep 3; done
sleep 6
SUP2=$(vtx query bank total --node "$RPC" -o json | jq -r '.supply[] | select(.denom=="uvtx") | .amount')
log "supply before=$SUP1 after=$SUP2"
assert_gt "$SUP1" "$SUP2" "uvtx total supply decreased (burn)"

# 4) DISTRIBUTION: community pool / fee pool grew (60% distribute path).
log "[4/5] distribution received fees"
POOL=$(vtx query distribution community-pool --node "$RPC" -o json | jq -r '.pool[0].amount // "0"')
log "community pool: $POOL"
assert_gt "$POOL" "0" "distribution community pool > 0"

# 5) IBC: VTX voucher present on gaia after a transfer.
log "[5/5] IBC VTX transfer to gaia"
GADDR=$(docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml exec -T gaia gaiad keys show relayer -a --keyring-backend test --home /root/.gaia)
vtx tx ibc-transfer transfer transfer channel-0 "$GADDR" 500000uvtx --from testuser $txflags; sleep 20
VOUCHER=$(docker compose --env-file infra/devnet/.env -f infra/devnet/docker-compose.yml exec -T gaia \
  gaiad query bank balances "$GADDR" --node tcp://localhost:26657 -o json | jq -r '.balances[] | select(.denom|startswith("ibc/")) | .amount' | head -1)
[ -n "$VOUCHER" ] || die "no ibc voucher on gaia"
assert_gt "$VOUCHER" "0" "gaia holds VTX ibc voucher ($VOUCHER)"

log "===================="
log "DEVNET SMOKE: PASS"
log "===================="
