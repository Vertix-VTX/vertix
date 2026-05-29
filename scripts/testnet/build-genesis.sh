#!/usr/bin/env bash
# Reproducible genesis builder for vertix-testnet-1 (PUBLIC TESTNET — NOT mainnet).
# Output: infra/testnet/genesis/{genesis.json,genesis.sha256,seeds.txt,persistent_peers.txt}
set -euo pipefail
cd "$(dirname "$0")/../.."
. scripts/devnet/lib.sh
require docker
set -a; . infra/testnet/.env; . infra/testnet/mnemonics.env; set +a

GEN="infra/testnet/.gen"
OUT="infra/testnet/genesis"
IMG="$VERTIX_IMAGE"
vd() { docker run --rm -i -v "$PWD/$GEN:/g" -e HOME=/g "$IMG" vertixd "$@"; }

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

log "Patching params (staking/gov/fees/oracle)"
jqi '.app_state.staking.params.bond_denom="uvtx"
   | .app_state.crisis.constant_fee.denom="uvtx"
   | .app_state.gov.params.min_deposit[0].denom="uvtx"
   | .app_state.gov.params.voting_period="300s"
   | .app_state.gov.params.expedited_voting_period="120s"
   | .app_state.gov.params.max_deposit_period="600s"'
jqi '.app_state.fees.params.burn_ratio="0.400000000000000000"
   | .app_state.fees.params.distribution_ratio="0.600000000000000000"'
jqi '.app_state.oracle.params.accept_list=["VTX:USD","BTC:USD","ETH:USD","ATOM:USD","USDC:USD"]'

vd genesis validate-genesis --home "$V1"

# --- gentx each founder (collect into a single launch genesis) ---
for i in 1 2 3; do
  vh="/g/founder$i"
  [ "$i" = "1" ] || vd init "founder$i" --chain-id "$CHAIN_ID" --default-denom "$DENOM" --home "$vh" >/dev/null 2>&1
  [ "$i" = "1" ] || cp "$GEN/founder1/config/genesis.json" "$GEN/founder$i/config/genesis.json"
  vd genesis gentx "founder$i" 1000000000000uvtx \
    --chain-id "$CHAIN_ID" --keyring-backend test --keyring-dir "$KR" \
    --home "$vh" --output-document "/g/gentx-founder$i.json"
done
mkdir -p "$GEN/founder1/config/gentx"
cp "$GEN"/gentx-founder*.json "$GEN/founder1/config/gentx/"
vd genesis collect-gentxs --home "$V1" --gentx-dir /g/founder1/config/gentx
vd genesis validate-genesis --home "$V1"

# --- Publish artifacts ---
cp "$GEN/founder1/config/genesis.json" "$OUT/genesis.json"
( cd "$OUT" && sha256sum genesis.json | tee genesis.sha256 )

# Seed/peer lists from founder node IDs (host placeholders edited per deployment).
for i in 1 2 3; do
  ID=$(vd tendermint show-node-id --home "/g/founder$i")
  echo "$ID@founder$i.testnet.vertix.example:26656"
done > "$OUT/persistent_peers.txt"
log "genesis built: $OUT/genesis.json"
( cd "$OUT" && cat genesis.sha256 )
