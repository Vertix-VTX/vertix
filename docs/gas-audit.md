# Vertix Gas / Unbounded-Loop Audit (Phase 7)

Each iteration site in the three custom modules, its bound, worst-case input,
and why it is acceptable for launch.

| Module | Site | File | Iterates over | Bound | Notes |
|--------|------|------|---------------|-------|-------|
| oracle | EndBlocker aggregation | `x/oracle/keeper/abci.go` | `params.AcceptList` × full feed prefix via `IterateAllFeeds` | O(P × F) | P = gov `AcceptList` (small); F = feeds submitted this window (≤ active validators × P). Per-pair scan filters `feedPair != pair` inside the callback. |
| oracle | EndBlocker outlier slash | `x/oracle/keeper/abci.go` | `validFeeds` for one pair | O(V) | V = validators that submitted a valid feed for the pair at quorum; ≤ bonded set. |
| oracle | EndBlocker miss accounting | `x/oracle/keeper/abci.go` | bonded validators × `quorumLive` pairs | O(V × P′) | P′ = pairs that met quorum this block; P′ ≤ \|AcceptList\|. |
| oracle | `DeleteAllFeeds` | `x/oracle/keeper/keeper.go` | all keys under feed prefix `0x01` | O(F) | End of window; F bounded as above; not per-tx. |
| oracle | `GetValidatorSubmittedPairs` | `x/oracle/keeper/keeper.go` | full feed prefix | O(F) | Called once per validator in miss loop; same F bound. |
| oracle | TWAP prune | `x/oracle/keeper/twap.go` (`PruneTWAPOlderThan`) | TWAP entries older than 24h cutoff | O(T) | T = stale entries per pair; keys time-ordered; stops at first in-window entry. |
| oracle | TWAP query | `x/oracle/keeper/twap.go` (`GetTWAP`) | TWAP prefix reverse scan | O(Tw) | Tw = entries within requested window + one leading clamp; not full chain history. |
| oracle | `prices` invariant | `x/oracle/keeper/invariants.go` | aggregated-price prefix `0x02` | O(P) | Crisis path only; stored prices ≤ \|AcceptList\|. |
| oracle | Msg handlers | `x/oracle/keeper/msg_server.go` | — | O(1) | Single-key reads/writes per message; no registry scan. |
| rwa | `SendRestriction` / `CheckTransferAllowed` | `x/rwa/keeper/send_restriction.go`, `restriction.go` | coins in transfer (≤ few denoms) | O(C) | C = coin count in transfer (tiny). Per rwa/* coin: `GetAsset` + fixed `Has` allow/deny lookups — **not** list enumeration. Guarded by `gas_bounds_test.go`. |
| rwa | `MsgUpdateRestrictions` batch | `x/rwa/keeper/msg_server.go` | add/del address lists in tx | O(B) | B ≤ `maxRestrictionBatch` (100) per `x/rwa/types/msgs.go`; issuer scales large lists across txs. |
| rwa | `QueryRestrictions` | `x/rwa/keeper/grpc_query.go` | allow/deny prefix per asset | O(M) | M = members of one asset; query/gRPC only, not bank hot path. |
| rwa | Genesis import/export | `x/rwa/keeper/genesis.go` | assets + membership prefixes | O(A + M) | One-time / export; not per-tx. |
| rwa | `bonds` / `denoms` invariants | `x/rwa/keeper/invariants.go` | all asset records via `IterateAssets` | O(A) | A = registered assets; crisis check off hot path. |
| fees | EndBlocker burn | `x/fees/keeper/abci.go` | — | O(1) | Single fee-collector balance read, module transfer, burn, cumulative counter update. |
| fees | `reconcile` invariant | `x/fees/keeper/invariants.go` | — | O(1) | Three scalar reads: genesis supply, current supply, cumulative burned. |
| fees | `module-balance` invariant | `x/fees/keeper/invariants.go` | — | O(1) | Single module-account balance read. |
| fees | Msg handlers | `x/fees/keeper/msg_server.go` | — | O(1) | Params update only. |

**Conclusion:** No unbounded enumeration on any message or bank send hot path. Oracle
EndBlock work scales with the bonded validator set and governance-sized accept list.
RWA transfer policy uses keyed membership (`0x03` / `0x04` prefixes in
`x/rwa/types/keys.go`), not inline lists. Fees EndBlock and invariants are
constant-time. Invariants and genesis/export iterators run off the per-tx hot path.

**Guard test:** `x/rwa/keeper/gas_bounds_test.go` (`TestGasBounds`) — compares
`SendRestriction` gas with 1 vs 1000 allow-list members (`allow_all=false`).
