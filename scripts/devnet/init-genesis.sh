#!/usr/bin/env bash
# Deterministic genesis + per-validator config builder for vertix-devnet-1.
# Output: infra/devnet/.gen/{keyring,validator1,validator2,validator3,genesis.json}
set -euo pipefail
cd "$(dirname "$0")/../.."
. scripts/devnet/lib.sh
require docker
set -a; . infra/devnet/.env; . infra/devnet/mnemonics.env; set +a

GEN="infra/devnet/.gen"
KEYS="infra/devnet/keys"
IMG="$VERTIX_IMAGE"
# Run vertixd in the image against a bind-mounted workdir so output lands on host.
vd() { docker run --rm -i -v "$PWD/$GEN:/g" -e HOME=/g "$IMG" vertixd "$@"; }

log "Wiping $GEN"
rm -rf "$GEN"; mkdir -p "$GEN"
KR="/g/keyring"            # shared test keyring inside container
mkdir -p "$GEN/keyring"

recover() { # recover <name> <mnemonic>
  echo "$2" | vd keys add "$1" --recover --keyring-backend test --keyring-dir "$KR" >/dev/null
}
addr()  { vd keys show "$1" -a --keyring-backend test --keyring-dir "$KR"; }
vaddr() { vd keys show "$1" --bech val -a --keyring-backend test --keyring-dir "$KR"; }

log "Recovering keys into shared keyring"
recover val1 "$MNEMONIC_VAL1";       recover val2 "$MNEMONIC_VAL2";       recover val3 "$MNEMONIC_VAL3"
recover feeder1 "$MNEMONIC_FEEDER1"; recover feeder2 "$MNEMONIC_FEEDER2"; recover feeder3 "$MNEMONIC_FEEDER3"
recover faucet "$MNEMONIC_FAUCET";   recover relayer "$MNEMONIC_RELAYER"; recover testuser "$MNEMONIC_TESTUSER"

# --- Build genesis on validator1's home, then fan out ---
V1=/g/validator1
vd init val1 --chain-id "$CHAIN_ID" --default-denom "$DENOM" --home "$V1" >/dev/null 2>&1

# Genesis accounts (D9 simplified allocation; total well under 21,000,000 VTX = 21e12 uvtx).
# Validators self-stake 1,000,000 VTX each; faucet 5,000,000 VTX; feeders/relayer/testuser working balances.
vd genesis add-genesis-account "$(addr val1)"    1000000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr val2)"    1000000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr val3)"    1000000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr faucet)"  5000000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr feeder1)"   10000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr feeder2)"   10000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr feeder3)"   10000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr relayer)"   10000000000uvtx --home "$V1"
vd genesis add-genesis-account "$(addr testuser)" 100000000000uvtx --home "$V1"

# --- Patch module params in genesis.json with jq (run jq inside the image) ---
jqi() { docker run --rm -i -v "$PWD/$GEN:/g" "$IMG" sh -c "jq '$1' /g/validator1/config/genesis.json > /g/g.tmp && mv /g/g.tmp /g/validator1/config/genesis.json"; }

log "Patching staking/gov/fees/oracle/rwa params"
jqi '.app_state.staking.params.bond_denom="uvtx"
   | .app_state.crisis.constant_fee.denom="uvtx"
   | .app_state.gov.params.min_deposit[0].denom="uvtx"
   | .app_state.gov.params.voting_period="120s"
   | .app_state.gov.params.expedited_voting_period="60s"
   | .app_state.gov.params.max_deposit_period="120s"'

# fees: 40% burn / 60% distribute (Invariant 5; sum must == 1).
jqi '.app_state.fees.params.burn_ratio="0.400000000000000000"
   | .app_state.fees.params.distribution_ratio="0.600000000000000000"'

# oracle: 5 genesis pairs accept_list.
jqi '.app_state.oracle.params.accept_list=["VTX:USD","BTC:USD","ETH:USD","ATOM:USD","USDC:USD"]'

log "Validating intermediate genesis"
vd genesis validate-genesis --home "$V1"

# --- Place fixed node/consensus keys, then gentx each validator ---
for i in 1 2 3; do
  vh="/g/validator$i"
  [ "$i" = "1" ] || vd init "val$i" --chain-id "$CHAIN_ID" --default-denom "$DENOM" --home "$vh" >/dev/null 2>&1
  cp "$KEYS/validator$i/node_key.json"          "$GEN/validator$i/config/node_key.json"
  cp "$KEYS/validator$i/priv_validator_key.json" "$GEN/validator$i/config/priv_validator_key.json"
  # Validators 2/3 need val1's genesis to gentx against the funded accounts:
  [ "$i" = "1" ] || cp "$GEN/validator1/config/genesis.json" "$GEN/validator$i/config/genesis.json"
  vd genesis gentx "val$i" 1000000000000uvtx \
    --chain-id "$CHAIN_ID" --keyring-backend test --keyring-dir "$KR" \
    --home "$vh" --output-document "/g/gentx-val$i.json"
done

log "Collecting gentxs"
mkdir -p "$GEN/validator1/config/gentx"
cp "$GEN"/gentx-val*.json "$GEN/validator1/config/gentx/"
vd genesis collect-gentxs --home "$V1" --gentx-dir /g/validator1/config/gentx
vd genesis validate-genesis --home "$V1"

# Final genesis fans out to all validators.
cp "$GEN/validator1/config/genesis.json" "$GEN/genesis.json"
for i in 2 3; do cp "$GEN/genesis.json" "$GEN/validator$i/config/genesis.json"; done

# --- persistent_peers from fixed node IDs ---
ID1=$(vd tendermint show-node-id --home /g/validator1)
ID2=$(vd tendermint show-node-id --home /g/validator2)
ID3=$(vd tendermint show-node-id --home /g/validator3)
PEERS="$ID1@vertix-val1:26656,$ID2@vertix-val2:26656,$ID3@vertix-val3:26656"
log "persistent_peers = $PEERS"
echo "$PEERS" > "$GEN/persistent_peers.txt"

# --- per-validator config.toml / app.toml edits via vertixd config + sed-in-image ---
patch_cfg() { # patch_cfg <i>
  local i=$1 ct="/g/validator$i/config/config.toml" at="/g/validator$i/config/app.toml"
  docker run --rm -i -v "$PWD/$GEN:/g" "$IMG" sh -c "
    sed -i 's|^persistent_peers = .*|persistent_peers = \"$PEERS\"|' $ct
    sed -i 's|^laddr = \"tcp://127.0.0.1:26657\"|laddr = \"tcp://0.0.0.0:26657\"|' $ct
    sed -i 's|^prometheus = false|prometheus = true|' $ct
    sed -i 's|^prometheus_listen_addr = \":26660\"|prometheus_listen_addr = \"0.0.0.0:26660\"|' $ct
    sed -i 's|^minimum-gas-prices = \"\"|minimum-gas-prices = \"0.025uvtx\"|' $at
    sed -i '/^\[api\]/,/^\[/ s|^enable = false|enable = true|' $at
    sed -i 's|^address = \"tcp://localhost:1317\"|address = \"tcp://0.0.0.0:1317\"|' $at
    sed -i '/^\[grpc\]/,/^\[/ s|^address = \"localhost:9090\"|address = \"0.0.0.0:9090\"|' $at
  "
}
for i in 1 2 3; do patch_cfg "$i"; done

# --- Render feeder configs with real valoper addresses ---
mkdir -p "$GEN/feeder"
for i in 1 2 3; do
  vo=$(vaddr "val$i")
  sed "s|VALOPER${i}_PLACEHOLDER|$vo|" "infra/devnet/feeder/feeder$i.yaml" > "$GEN/feeder/feeder$i.yaml"
done
# Export the shared keyring so feeders can sign (the same /keyring dir is mounted into feeder containers).
cp -r "$GEN/keyring" "$GEN/feeder/keyring"
log "rendered feeder configs into $GEN/feeder"

log "init-genesis complete: $GEN"
