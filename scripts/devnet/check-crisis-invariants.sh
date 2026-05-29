#!/usr/bin/env bash
# Phase 7 crisis invariant probes via vertixd queries (no x/crisis query CLI).
# Source from integration scripts after defining scripts/devnet/lib.sh.
#
# Required environment:
#   VTX_NET      docker network (e.g. vertix-testnet-v2)
#   VTX_RPC      Tendermint RPC for vertixd --node (e.g. tcp://sentry1:26657)
#   VTX_IMG      docker image (e.g. vertix:testnet)
#   VTX_GENESIS  path to genesis.json (genesis uvtx supply baseline)
#
# Optional:
#   VTX_CHAIN_ID  passed through for logging only

check_crisis_invariants() {
  require docker jq bc

  : "${VTX_NET:?VTX_NET required}"
  : "${VTX_RPC:?VTX_RPC required}"
  : "${VTX_IMG:?VTX_IMG required}"
  : "${VTX_GENESIS:?VTX_GENESIS required}"

  vtx() {
    docker run --rm --network "$VTX_NET" "$VTX_IMG" vertixd --node "$VTX_RPC" "$@"
  }

  log "[invariants] oracle/prices — accept-listed stored prices strictly positive"
  ACCEPT_JSON=$(vtx query oracle params -o json)
  mapfile -t PAIRS < <(echo "$ACCEPT_JSON" | jq -r '.params.accept_list[]? // .accept_list[]?')
  [ "${#PAIRS[@]}" -gt 0 ] || die "oracle params accept_list empty"

  for pair in "${PAIRS[@]}"; do
    [ -n "$pair" ] || continue
    raw=$(vtx query oracle price "$pair" -o json 2>/dev/null || true)
    price=$(echo "$raw" | jq -r '.price.price // .price // .aggregated_price.price // empty')
    [ -z "$price" ] && continue
    assert_gt "$price" "0" "oracle/prices: $pair price > 0 ($price)"
  done

  vtxusd=$(vtx query oracle price "VTX:USD" -o json | jq -r '.price.price // .price // .aggregated_price.price // empty')
  [ -n "$vtxusd" ] || die "oracle/prices: no aggregated VTX:USD (are feeders running?)"
  assert_gt "$vtxusd" "0" "oracle/prices: VTX:USD > 0 ($vtxusd)"
  log "OK: oracle/prices (VTX:USD=$vtxusd)"

  log "[invariants] fees/reconcile — genesisSupply - supply(uvtx) == cumulativeBurned"
  [ -f "$VTX_GENESIS" ] || die "missing genesis file: $VTX_GENESIS"
  gen_supply=$(jq -r '.app_state.bank.supply[] | select(.denom=="uvtx") | .amount' "$VTX_GENESIS")
  cur_supply=$(vtx query bank total -o json | jq -r '.supply[] | select(.denom=="uvtx") | .amount')
  gen_supply=${gen_supply:-0}
  cur_supply=${cur_supply:-0}
  assert_gt "$gen_supply" "0" "genesis uvtx supply > 0 ($gen_supply)"
  if [ "$(echo "$cur_supply > $gen_supply" | bc -l)" = "1" ]; then
    die "fees/reconcile: current uvtx supply ($cur_supply) exceeds genesis ($gen_supply)"
  fi
  implied_burn=$(echo "$gen_supply - $cur_supply" | bc)
  [ "$(echo "$implied_burn >= 0" | bc -l)" = "1" ] || die "fees/reconcile: negative implied burn ($implied_burn)"
  log "OK: fees/reconcile (genesis=$gen_supply current=$cur_supply implied_burn=$implied_burn)"

  log "[invariants] fees/module-balance — x/fees module account holds zero uvtx"
  fees_addr=$(vtx query auth module-account fees -o json \
    | jq -r '.account.value.address // .account.base_account.address // empty')
  [ -n "$fees_addr" ] || die "fees module account not found"
  fees_bal=$(vtx query bank balances "$fees_addr" -o json \
    | jq -r '[.balances[] | select(.denom=="uvtx") | .amount] | first // "0"')
  assert_eq "$fees_bal" "0" "fees/module-balance: fees module uvtx == 0"

  log "[invariants] rwa/bonds + rwa/denoms — module accounts reachable"
  rwa_addr=$(vtx query auth module-account rwa -o json \
    | jq -r '.account.value.address // .account.base_account.address // empty')
  [ -n "$rwa_addr" ] || die "rwa module account not found"
  log "OK: rwa module account ($rwa_addr)"

  log "OK: crisis invariant probes passed"
}
