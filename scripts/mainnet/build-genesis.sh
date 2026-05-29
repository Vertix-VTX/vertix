#!/usr/bin/env bash
# Mainnet genesis builder (Phase 9 stretch) — full 21M VTX allocation per docs/tokenomics.md.
# Output: infra/mainnet/genesis/genesis.json + genesis.sha256
# DRY-RUN: uses mnemonics.env.example unless scripts/mainnet/mnemonics.env exists.
# Phase 10 replaces keys/addresses and TGE before the coordinated launch ceremony.
set -euo pipefail
cd "$(dirname "$0")/../.."
. scripts/devnet/lib.sh
require jq

CHAIN_ID="${CHAIN_ID:-vertix-1}"
DENOM="${DENOM:-uvtx}"
IMG="${VERTIX_IMAGE:-vertix:testnet}"
ALLOC="scripts/mainnet/allocations.json"
GEN="infra/mainnet/.gen"
OUT="infra/mainnet/genesis"

if [ -f scripts/mainnet/mnemonics.env ]; then
  set -a; . scripts/mainnet/mnemonics.env; set +a
else
  log "Using scripts/mainnet/mnemonics.env.example (dry-run keys)"
  set -a; . scripts/mainnet/mnemonics.env.example; set +a
fi

TGE_UNIX="${MAINNET_TGE_UNIX:-$(jq -r '.tge_unix' "$ALLOC")}"
EXPECTED_TOTAL="$(jq -r '.total_uvtx' "$ALLOC")"

USE_DOCKER=0
if [ "${VERTIX_GENESIS_LOCAL:-}" != "1" ] && docker info >/dev/null 2>&1; then
  USE_DOCKER=1
fi

if [ "$USE_DOCKER" = "1" ]; then
  require docker
  NODE_HOME="/g/node"
  KR="/g/keyring"
  GENESIS_FILE="$GEN/node/config/genesis.json"
  vd() { docker run --rm -i -v "$PWD/$GEN:/g" -e HOME=/g "$IMG" vertixd "$@"; }
  jqi() {
    docker run --rm -i -v "$PWD/$GEN:/g" "$IMG" sh -c \
      "jq '$1' /g/node/config/genesis.json > /g/g.tmp && mv /g/g.tmp /g/node/config/genesis.json"
  }
else
  VERTIXD_BIN="${VERTIXD_BIN:-build/vertixd}"
  command -v "$VERTIXD_BIN" >/dev/null 2>&1 || VERTIXD_BIN="$(command -v vertixd)"
  [ -x "$VERTIXD_BIN" ] || die "need docker or vertixd at build/vertixd (set VERTIX_GENESIS_LOCAL=1)"
  log "Using local $VERTIXD_BIN (VERTIX_GENESIS_LOCAL / docker unavailable)"
  NODE_HOME="$PWD/$GEN/node"
  KR="$PWD/$GEN/keyring"
  GENESIS_FILE="$NODE_HOME/config/genesis.json"
  vd() { "$VERTIXD_BIN" "$@"; }
  jqi() { jq "$1" "$GENESIS_FILE" > "${GENESIS_FILE}.tmp" && mv "${GENESIS_FILE}.tmp" "$GENESIS_FILE"; }
fi

# split_uvtx <total> <n_periods> — print space-separated per-period amounts (last gets remainder).
split_uvtx() {
  local total=$1 n=$2
  local base=$(( total / n ))
  local rem=$(( total - base * n ))
  local i
  for (( i = 1; i <= n; i++ )); do
    if [ "$i" -eq "$n" ]; then
      echo $(( base + rem ))
    else
      echo "$base"
    fi
  done
}

# patch_periodic <address> <amount_uvtx> <tge> <cliff_sec> <period_sec> <n_periods>
# Cliff is modeled by start_time = tge + cliff_sec (SDK rejects zero-amount vesting periods).
patch_periodic() {
  local addr=$1 total=$2 tge=$3 cliff_sec=$4 period_sec=$5 n=$6
  local vest_start=$(( tge + cliff_sec ))
  local periods_json='[]'
  local -a amounts
  mapfile -t amounts < <(split_uvtx "$total" "$n")
  local i amt
  for i in "${!amounts[@]}"; do
    amt="${amounts[$i]}"
    periods_json=$(jq -n \
      --argjson prev "$periods_json" \
      --arg len "$period_sec" \
      --arg amt "$amt" \
      '$prev + [{length: $len, amount: [{denom: "uvtx", amount: $amt}]}]')
  done
  local end_time=$(( vest_start + period_sec * n ))
  jq --arg addr "$addr" \
     --arg total "$total" \
     --arg start "$vest_start" \
     --arg end "$end_time" \
     --argjson periods "$periods_json" '
    .app_state.auth.accounts |= map(
      if .address == $addr then
        . as $base |
        {
          "@type": "/cosmos.vesting.v1beta1.PeriodicVestingAccount",
          base_vesting_account: {
            base_account: {
              address: $addr,
              pub_key: null,
              account_number: $base.account_number,
              sequence: $base.sequence
            },
            original_vesting: [{denom: "uvtx", amount: $total}],
            delegated_free: [],
            delegated_vesting: [],
            end_time: $end
          },
          start_time: $start,
          vesting_periods: $periods
        }
      else . end
    )
  ' "$GENESIS_FILE" > "${GENESIS_FILE}.tmp" && mv "${GENESIS_FILE}.tmp" "$GENESIS_FILE"
}

add_account() {
  local home=$1 addr=$2 amount=$3
  shift 3
  vd genesis add-genesis-account "$addr" "${amount}uvtx" --home "$home" "$@"
}

log "MAINNET genesis builder (stretch dry-run) — chain_id=$CHAIN_ID TGE=$TGE_UNIX"
log "Wiping $GEN"
rm -rf "$GEN"; mkdir -p "$GEN/keyring" "$OUT" "$GEN/node"

recover() {
  local name=$1 mnemonic=$2
  echo "$mnemonic" | vd keys add "$name" --recover --keyring-backend test --keyring-dir "$KR" >/dev/null
}
addr() { vd keys show "$1" -a --keyring-backend test --keyring-dir "$KR"; }

log "Recovering allocation keys (dry-run mnemonics)"
recover mainnet-team              "$MNEMONIC_MAINNET_TEAM"
recover mainnet-foundation        "$MNEMONIC_MAINNET_FOUNDATION"
recover mainnet-investors         "$MNEMONIC_MAINNET_INVESTORS"
recover mainnet-ecosystem         "$MNEMONIC_MAINNET_ECOSYSTEM"
recover mainnet-validator-pool    "$MNEMONIC_MAINNET_VALIDATOR_POOL"
recover mainnet-liquidity-tge     "$MNEMONIC_MAINNET_LIQUIDITY_TGE"
recover mainnet-liquidity-reserve "$MNEMONIC_MAINNET_LIQUIDITY_RESERVE"
recover mainnet-airdrop           "$MNEMONIC_MAINNET_AIRDROP"

vd init mainnet-genesis --chain-id "$CHAIN_ID" --default-denom "$DENOM" --home "$NODE_HOME" >/dev/null 2>&1

# Add all accounts as BaseAccount first (CLI cannot read PeriodicVesting mid-build).
log "Adding genesis balances (base accounts)"
AIRDROP_AMT="$(jq -r '.allocations[] | select(.id=="community_airdrop") | .amount_uvtx' "$ALLOC")"
LIQ_TGE="$(jq -r '.allocations[] | select(.id=="public_liquidity") | .accounts[0].amount_uvtx' "$ALLOC")"
LIQ_RES="$(jq -r '.allocations[] | select(.id=="public_liquidity") | .accounts[1].amount_uvtx' "$ALLOC")"
VAL_AMT="$(jq -r '.allocations[] | select(.id=="validator_pool") | .amount_uvtx' "$ALLOC")"
for key in mainnet-airdrop mainnet-liquidity-tge mainnet-team mainnet-foundation \
  mainnet-investors mainnet-ecosystem; do
  case "$key" in
    mainnet-airdrop) amt="$AIRDROP_AMT" ;;
    mainnet-liquidity-tge) amt="$LIQ_TGE" ;;
    mainnet-team) amt="$(jq -r '.allocations[]|select(.id=="team")|.amount_uvtx' "$ALLOC")" ;;
    mainnet-foundation) amt="$(jq -r '.allocations[]|select(.id=="foundation")|.amount_uvtx' "$ALLOC")" ;;
    mainnet-investors) amt="$(jq -r '.allocations[]|select(.id=="strategic_investors")|.amount_uvtx' "$ALLOC")" ;;
    mainnet-ecosystem) amt="$(jq -r '.allocations[]|select(.id=="ecosystem")|.amount_uvtx' "$ALLOC")" ;;
  esac
  add_account "$NODE_HOME" "$(addr "$key")" "$amt"
done

# Continuous vesting via CLI (liquidity reserve + validator pool).
LIQ_VEST_SEC="$(jq -r '.allocations[] | select(.id=="public_liquidity") | .accounts[1].vesting.duration_seconds' "$ALLOC")"
LIQ_END=$(( TGE_UNIX + LIQ_VEST_SEC ))
add_account "$NODE_HOME" "$(addr mainnet-liquidity-reserve)" "$LIQ_RES" \
  --vesting-amount "${LIQ_RES}uvtx" \
  --vesting-start-time "$TGE_UNIX" \
  --vesting-end-time "$LIQ_END"
VAL_DUR="$(jq -r '.allocations[] | select(.id=="validator_pool") | .vesting.duration_seconds' "$ALLOC")"
VAL_END=$(( TGE_UNIX + VAL_DUR ))
add_account "$NODE_HOME" "$(addr mainnet-validator-pool)" "$VAL_AMT" \
  --vesting-amount "${VAL_AMT}uvtx" \
  --vesting-start-time "$TGE_UNIX" \
  --vesting-end-time "$VAL_END"

# PeriodicVestingAccount — jq patch after all CLI accounts are added.
add_periodic() {
  local id=$1 key=$2
  local amt cliff_sec period_sec n_periods
  amt="$(jq -r --arg id "$id" '.allocations[] | select(.id==$id) | .amount_uvtx' "$ALLOC")"
  cliff_sec="$(jq -r --arg id "$id" '.allocations[] | select(.id==$id) | .vesting.cliff_seconds' "$ALLOC")"
  period_sec="$(jq -r --arg id "$id" '.allocations[] | select(.id==$id) | .vesting.linear_period_seconds' "$ALLOC")"
  n_periods="$(jq -r --arg id "$id" '.allocations[] | select(.id==$id) | .vesting.linear_periods' "$ALLOC")"
  patch_periodic "$(addr "$key")" "$amt" "$TGE_UNIX" \
    "$cliff_sec" "$period_sec" "$n_periods"
}

log "Patching PeriodicVestingAccount schedules"
add_periodic team mainnet-team
add_periodic foundation mainnet-foundation
add_periodic strategic_investors mainnet-investors
add_periodic ecosystem mainnet-ecosystem

log "Patching mainnet module params (gov/oracle/fees/bank metadata)"
jqi '.chain_id="'"$CHAIN_ID"'"
   | .app_state.staking.params.bond_denom="uvtx"
   | .app_state.crisis.constant_fee.denom="uvtx"
   | .app_state.gov.params.min_deposit[0].denom="uvtx"
   | .app_state.gov.params.voting_period="432000s"
   | .app_state.gov.params.expedited_voting_period="86400s"
   | .app_state.gov.params.max_deposit_period="1209600s"'
jqi '.app_state.fees.params.burn_ratio="0.400000000000000000"
   | .app_state.fees.params.distribution_ratio="0.600000000000000000"'
jqi '.app_state.oracle.params.accept_list=["VTX:USD","BTC:USD","ETH:USD","ATOM:USD","USDC:USD"]'
jqi '.app_state.bank.denom_metadata = [{
      "description": "The native token of the Vertix blockchain",
      "denom_units": [
        {"denom": "uvtx", "exponent": 0, "aliases": ["microvtx"]},
        {"denom": "mvtx", "exponent": 3, "aliases": ["millivtx"]},
        {"denom": "vtx", "exponent": 6, "aliases": []}
      ],
      "base": "uvtx",
      "display": "vtx",
      "name": "Vertix",
      "symbol": "VTX"
    }]'

log "Asserting total supply"
ACTUAL=$(jq '[.app_state.bank.balances[].coins[] | select(.denom=="uvtx") | .amount | tonumber] | add' \
  "$GENESIS_FILE")
if [ "$ACTUAL" != "$EXPECTED_TOTAL" ]; then
  die "supply mismatch: expected $EXPECTED_TOTAL uvtx, got $ACTUAL"
fi
log "Total supply OK: $ACTUAL uvtx (21,000,000 VTX)"

vd genesis validate-genesis --home "$NODE_HOME"

cp "$GENESIS_FILE" "$OUT/genesis.json"
( cd "$OUT" && sha256sum genesis.json | tee genesis.sha256 )
log "mainnet genesis built: $OUT/genesis.json"
( cd "$OUT" && cat genesis.sha256 )
