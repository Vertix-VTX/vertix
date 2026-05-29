#!/usr/bin/env bash
# Reproducible base genesis for vertix-testnet-2 (PUBLIC TESTNET v2 — NOT mainnet distribution).
# Output: infra/testnet/v2/genesis/base/base-genesis.json (+ base-genesis.sha256)
# No gentxs — founders submit gentxs against this base in the ceremony workflow.
set -euo pipefail
cd "$(dirname "$0")/../../.."
. scripts/devnet/lib.sh
require docker
set -a; . infra/testnet/v2/.env; . infra/testnet/mnemonics.env; set +a

GEN="infra/testnet/v2/.gen"
OUT="infra/testnet/v2/genesis/base"
IMG="$VERTIX_IMAGE"
vd() { docker run --rm -i -v "$PWD/$GEN:/g" -e HOME=/g "$IMG" vertixd "$@"; }

log "PUBLIC TESTNET v2 — NOT mainnet distribution"
log "Wiping $GEN"
rm -rf "$GEN"; mkdir -p "$GEN/keyring" "$OUT"
KR="/g/keyring"

recover() { echo "$2" | vd keys add "$1" --recover --keyring-backend test --keyring-dir "$KR" >/dev/null; }
addr()  { vd keys show "$1" -a --keyring-backend test --keyring-dir "$KR"; }

log "Recovering founder/faucet/feeder keys"
recover founder1 "$MNEMONIC_FOUNDER1"; recover founder2 "$MNEMONIC_FOUNDER2"; recover founder3 "$MNEMONIC_FOUNDER3"
recover feeder1 "$MNEMONIC_FEEDER1";   recover feeder2 "$MNEMONIC_FEEDER2";   recover feeder3 "$MNEMONIC_FEEDER3"
recover faucet "$MNEMONIC_FAUCET";     recover relayer "$MNEMONIC_RELAYER"

V1=/g/founder1
vd init founder1 --chain-id "$CHAIN_ID" --default-denom "$DENOM" --home "$V1" >/dev/null 2>&1

# --- TESTNET-ONLY allocation (clearly NOT mainnet distribution). Total << 21,000,000 VTX. ---
# Founders self-stake 1,000,000 VTX each; faucet 8,000,000 VTX (public drips); feeders working balances.
vd genesis add-genesis-account "$(addr founder1)" 1000000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr founder2)" 1000000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr founder3)" 1000000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr faucet)"   8000000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr feeder1)"    10000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr feeder2)"    10000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr feeder3)"    10000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr relayer)"    10000000000uvtx --home "$V1"

jqi() { docker run --rm -i -v "$PWD/$GEN:/g" "$IMG" sh -c "jq '$1' /g/founder1/config/genesis.json > /g/g.tmp && mv /g/g.tmp /g/founder1/config/genesis.json"; }

log "Patching params (chain_id/staking/gov/fees/oracle)"
jqi '.chain_id="vertix-testnet-2"
   | .app_state.staking.params.bond_denom="uvtx"
   | .app_state.crisis.constant_fee.denom="uvtx"
   | .app_state.gov.params.min_deposit[0].denom="uvtx"
   | .app_state.gov.params.voting_period="300s"
   | .app_state.gov.params.expedited_voting_period="120s"
   | .app_state.gov.params.max_deposit_period="600s"'
jqi '.app_state.fees.params.burn_ratio="0.400000000000000000"
   | .app_state.fees.params.distribution_ratio="0.600000000000000000"'
jqi '.app_state.oracle.params.accept_list=["VTX:USD","BTC:USD","ETH:USD","ATOM:USD","USDC:USD"]'

vd genesis validate-genesis --home "$V1"

cp "$GEN/founder1/config/genesis.json" "$OUT/base-genesis.json"
( cd "$OUT" && sha256sum base-genesis.json | tee base-genesis.sha256 )
log "base genesis built: $OUT/base-genesis.json"
( cd "$OUT" && cat base-genesis.sha256 )
