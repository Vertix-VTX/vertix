# Vertix — Technical Design

This document is the engineering reference for the Vertix custom modules and the `vertix-feeder` sidecar. It defines protobuf surface, keeper APIs, store layouts, state machines, ABCI hooks, slashing math, and security invariants.

For higher-level context see [`architecture.md`](./architecture.md). For the approved program spec see [`full-design-spec.md`](./full-design-spec.md).

---

## 1. Chain Foundation

### 1.1 Identity

| Field | Value |
|---|---|
| Binary | `vertixd` |
| Bech32 prefix | `vtx` |
| Base denom | `uvtx` |
| Display denom | `vtx` |
| Decimals | 6 |
| Mainnet chain ID | `vertix-1` |
| Devnet chain ID | `vertix-devnet-1` |
| Testnet chain IDs | `vertix-testnet-1`, `vertix-testnet-2` |

### 1.2 Genesis Parameters (defaults)

| Module | Parameter | Value |
|---|---|---|
| `x/staking` | `bond_denom` | `uvtx` |
| `x/staking` | `unbonding_time` | `1814400s` (21 days) |
| `x/staking` | `max_validators` | 125 |
| `x/staking` | `min_commission_rate` | `0.05` |
| `x/gov` | `min_deposit` | `10,000,000,000 uvtx` |
| `x/gov` | `voting_period` | `432000s` (5 days) |
| `x/gov` | `expedited_voting_period` | `86400s` (1 day) |
| `x/gov` | `quorum` / `threshold` | `0.334` / `0.5` |
| `x/distribution` | `community_tax` | `0.02` |
| `x/slashing` | `signed_blocks_window` | 100 |
| `x/slashing` | `min_signed_per_window` | `0.5` |
| `x/slashing` | `slash_fraction_double_sign` | `0.05` |
| `x/slashing` | `slash_fraction_downtime` | `0.01` |
| `x/crisis` | `constant_fee` | `1,000,000,000 uvtx` |
| Default `min_gas_prices` | | `0.025uvtx` |

### 1.3 Module Set

**Wired (standard, unmodified):**
`x/auth`, `x/bank`, `x/staking`, `x/gov`, `x/distribution`, `x/slashing`, `x/upgrade`, `x/params`, `x/crisis`, `x/feegrant`, `x/authz`, `x/capability`, `x/consensus`, `x/ibc`, `x/transfer`, `x/genutil`, `x/evidence`, `x/vesting`.

**IBC stack (scaffolded):** includes ICA (ICS-27) and `29-fee`.

**Custom:** `x/oracle`, `x/rwa`, `x/fees`.

**Removed:** `x/mint` — to enforce the 21M VTX hard cap. Removal must be guarded by a unit test (`TestNoMintModule`) in `app/app_test.go`.

---

## 2. `x/oracle` — Module Design

> **Phase 1 design-spec refinement (2026-05-29).** The Phase 1 brainstorm + design review ([`specs/2026-05-29-phase-1-oracle-module-design.md`](./specs/2026-05-29-phase-1-oracle-module-design.md)) refined several items below; this section is synced to match. Key changes: **feeder delegation** (the feeder, not the operator, signs `MsgSubmitFeed`), **quorum-gated** aggregation, **unweighted-median** outlier reference, **tumbling** miss window, **price staleness** (`max_price_age`), `StakingKeeper.GetValidator` (not `GetValidatorByConsAddr`), `SlashingKeeper.Slash` returns `error` only, and per-validator window counters.

### 2.1 Responsibilities
- Accept feeder-submitted, per-validator price submissions per `MsgSubmitFeed` (feeder delegated via `MsgSetFeeder`).
- At end of each `VoteWindow`, aggregate via stake-weighted median per pair **when submissions reach `QuorumFraction` of bonded power**.
- Maintain TWAP histories per pair; serve prices/TWAP with a `MaxPriceAge` staleness guard.
- Track per-validator misses (tumbling window) and outliers; trigger slashes via `x/slashing`.
- Expose a clean keeper interface to `x/rwa`.

### 2.2 Protobuf Surface

```
proto/vertix/oracle/v1/
├── types.proto    OracleFeed, AggregatedPrice, TWAPEntry, OracleParams, FeederDelegation
├── tx.proto       MsgSetFeeder, MsgSubmitFeed, MsgUpdateParams (+ responses)
├── query.proto    Price, Twap, Params, MissCounter, Feeder
└── genesis.proto  GenesisState
```

Key messages (feeder signs `MsgSubmitFeed`, operator signs `MsgSetFeeder` — Phase 1 spec D10):

```protobuf
message MsgSetFeeder {
  option (cosmos.msg.v1.signer) = "validator";
  string validator = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"]; // valoper
  string feeder    = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"]; // authorized account
}

message MsgSubmitFeed {
  option (cosmos.msg.v1.signer) = "feeder";
  string feeder    = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"]; // signer (delegated feeder)
  string validator = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"]; // valoper the feed is for
  string pair      = 3;   // "BASE:QUOTE", e.g. "VTX:USD"
  string price     = 4;   // LegacyDec encoded as string
}

message OracleParams {
  int64  vote_window         = 1; // blocks per aggregation window
  string miss_threshold      = 2; // LegacyDec, e.g. "0.05"
  string miss_slash_rate     = 3; // LegacyDec, e.g. "0.005"
  string outlier_slash_rate  = 4; // LegacyDec, e.g. "0.01"
  string outlier_threshold   = 5; // LegacyDec, e.g. "0.05" — band vs UNWEIGHTED median
  int64  miss_window_size    = 6; // tumbling eval window in windows, e.g. 500
  string quorum_fraction     = 7; // LegacyDec, e.g. "0.667" — min bonded power to aggregate
  int64  max_price_age       = 8; // seconds; staleness ceiling for GetPrice/GetTWAP, e.g. 300
  repeated string accept_list = 9; // whitelisted pairs
}
```

### 2.3 Keeper Interfaces

**Public (consumed by other modules):**

```go
type OracleKeeper interface {
    // Returns ErrNoPrice if never aggregated, ErrStalePrice if older than MaxPriceAge.
    GetPrice(ctx context.Context, pair string) (math.LegacyDec, error)
    GetTWAP(ctx context.Context, pair string, window time.Duration) (math.LegacyDec, error)
}
```

`x/rwa` (Phase 3) MUST treat both `ErrNoPrice` and `ErrStalePrice` as attestation failures.

**External dependencies (held by keeper):**

```go
type StakingKeeper interface {
    GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error) // valoper → record
    GetLastValidatorPower(ctx context.Context, addr sdk.ValAddress) (int64, error)
    GetBondedValidatorsByPower(ctx context.Context) ([]stakingtypes.Validator, error)
    GetLastTotalPower(ctx context.Context) (math.Int, error)                               // quorum denominator (consensus power — same units as GetLastValidatorPower)
}

// Slash returns error only in SDK v0.50 (no math.Int). Jail is intentionally NOT used
// for oracle infractions (Phase 1 spec D7) to avoid validator chilling.
type SlashingKeeper interface {
    Slash(ctx context.Context, consAddr sdk.ConsAddress, fraction math.LegacyDec, power, distributionHeight int64) error
}
```

### 2.4 Store Layout

| Prefix | Key | Value |
|---|---|---|
| `0x01` | `{length-prefixed valoper}/{pair}` | `OracleFeed` (current window) |
| `0x02` | `{pair}` | `AggregatedPrice` (latest) |
| `0x03` | `{pair}/{ts_unixnano_be}` | `TWAPEntry` (history) |
| `0x04` | `{validator}` | `int64` miss counter (tumbling) |
| `0x05` | `{validator}` | `int64` total windows (**per-validator**, tumbling) |
| `0x06` | `(empty)` | `OracleParams` |
| `0x07` | `{feeder_acc}` | `{valoper}` (feeder → validator) |
| `0x08` | `{valoper}` | `{feeder_acc}` (reverse index) |

Keys are big-endian encoded for ordered iteration where required (TWAP timestamp scans). `0x05` is **per-validator** (Phase 1 spec D6): a correct miss-rate needs per-validator totals because validators bond at different heights. `0x07`/`0x08` hold feeder delegations (Phase 1 spec D10) and round-trip through genesis.

### 2.5 ABCI: `EndBlock` Algorithm

```
EndBlock(ctx):
    params = GetParams(ctx)
    if BlockHeight % params.VoteWindow != 0:
        return                                           # mid-window: no-op

    vals        = BondedValidatorsByPower()              # sorted, deterministic
    valByOper   = { v.Operator: v for v in vals }        # built once; read-by-key only
    totalPower  = GetLastTotalPower()                      # consensus power (not raw bonded tokens)
    quorumLive  = []                                       # slice in accept_list order

    # 1. Aggregate (quorum-gated) per pair
    for pair in params.AcceptList:
        feeds = GetAllFeedsForPair(pair)                 # full 0x01 scan filtered by pair; parse defensively
        if feeds is empty: continue
        submittedPower = Σ power(f.Validator)
        if submittedPower / totalPower < params.QuorumFraction:
            continue                                     # below quorum: no price, no TWAP, not a miss
        quorumLive.append(pair)
        median    = WeightedMedian(prices, weights)      # STAKE-weighted → published price
        refMedian = UnweightedMedian(prices)             # COUNT-based   → outlier reference
        SetAggregatedPrice(pair, median, height, blockTime)
        AppendTWAPEntry(pair, median, blockTime)
        emit EventPriceAggregated
        for f in feeds:                                  # outlier slash vs UNWEIGHTED median
            if |f.Price - refMedian| / refMedian > params.OutlierThreshold:
                slash(valByOper[f.Validator], params.OutlierSlashRate, "outlier")

    # 2. Miss tracking + slash (tumbling window; only vs quorum-live pairs)
    for v in vals:
        total = inc 0x05[v]
        if not (quorumLive ⊆ pairs v submitted): miss = inc 0x04[v]
        else:                                    miss = get 0x04[v]
        if total >= params.MissWindowSize:               # tumbling boundary
            if miss/total > params.MissThreshold:
                slash(v, params.MissSlashRate, "miss")
            reset 0x04[v]; reset 0x05[v]

    # 3. Clear feeds for next window
    DeleteAllFeeds()

slash(v, rate, reason):                                  # NO Jail (D7); distributionHeight = height
    power = GetLastValidatorPower(v.Operator); if power == 0: return
    slashingKeeper.Slash(v.ConsAddr(), rate, power, height)   # returns error only
    emit EventOracleSlash{validator, slash_reason, slash_fraction=rate}
```

**Stake-weighted median** (`WeightedMedian(prices, weights)`) — the *published* price:
- Sort `(price, weight)` pairs by `(price, valoper)` ascending (valoper tie-break for determinism).
- Walk in order, accumulating weight; the first price whose cumulative weight ≥ `totalWeight / 2` is the result (lower-side on exact half).

**Unweighted median** (`UnweightedMedian(prices)`) — the *outlier reference* only (Phase 1 spec D2/H1): each validator counts once, so a high-stake validator cannot move the band and slash the honest minority.

**Quorum gate** (Phase 1 spec D11/C2): a pair aggregates only when submitting power ≥ `QuorumFraction × totalBonded`. This prevents a single validator from setting the price and is the gate the miss check uses, so an under-covered or freshly-added pair never slashes the whole set. On any `accept_list` change via `MsgUpdateParams`, `0x04`/`0x05` are reset.

### 2.6 Slashing Conditions

| Trigger | Rate | Module that executes |
|---|---|---|
| Miss rate > `MissThreshold` (default 5%) over a tumbling `MissWindowSize` (default 500) windows, counting only quorum-live pairs | `MissSlashRate` (default 0.5%) | `x/slashing.Slash` (called from `x/oracle.EndBlock`) |
| Outlier submission: `|price − unweightedMedian| / unweightedMedian > OutlierThreshold` (default 5%) | `OutlierSlashRate` (default 1.0%) | Same |

Note: oracle slashing is intentionally lighter than double-sign (5%) and **never jails** (Phase 1 spec D7) to avoid validator chilling and accidental jailing. The outlier reference is the **unweighted** window median (D2/H1), not the stake-weighted published price. MAD-based detection and a sliding-window bitmap are documented Phase-7 upgrades.

### 2.7 Events

| Event | Attributes |
|---|---|
| `oracle_feeder_set` | `validator`, `feeder` |
| `oracle_feed_submitted` | `validator`, `feeder`, `pair`, `price` |
| `oracle_price_aggregated` | `pair`, `price` |
| `oracle_slash` | `validator`, `slash_reason`, `slash_fraction` |
| `oracle_params_updated` | `accept_list_changed` |

`oracle_slash` carries `slash_fraction` (the applied rate), not an absolute amount, because `SlashingKeeper.Slash` returns `error` only in SDK v0.50 (Phase 1 spec §4).

---

## 3. `x/rwa` — Module Design

### 3.1 Responsibilities
- Maintain a registry of RWA assets and their lifecycle state.
- Escrow issuer VTX bonds.
- Mint/burn `rwa/{asset-id}` factory denoms.
- Enforce transfer restrictions (allowlist/denylist).
- Collect mint/settle fees and forward them to the standard fee collector.

### 3.2 State Machine

```
        register     attest        mint        settle
DRAFT ─────────► ATTESTED ────► ACTIVE ────► SETTLED
                    │              │
                    │              │ (any time, with valid params)
                    └─────► allow/deny list updates
```

**Transition guards:**

| From → To | Required preconditions |
|---|---|
| (none) → `DRAFT` | `MsgRegisterAsset`: bond ≥ `MinIssuerBond` locked; valid metadata; unique `asset_id` |
| `DRAFT` → `ATTESTED` | `MsgAttestAsset`: `OracleKeeper.GetPrice(oracle_pair)` returns non-error price |
| `ATTESTED` → `ACTIVE` | `MsgMintRWA`: notional > 0; mint fee paid |
| `ACTIVE` → `SETTLED` | `MsgSettleRWA`: caller is issuer (or governance for dispute); settle fee paid; tokens burned |

Once `SETTLED`, the asset is terminal. Bond is returned to issuer on clean settle, or slashed on dispute resolution.

### 3.3 Protobuf Surface

```
proto/vertix/rwa/v1/
├── types.proto    AssetRecord, AssetStatus, RWAParams
├── tx.proto       MsgRegisterAsset, MsgAttestAsset, MsgMintRWA, MsgTransferRWA,
│                  MsgSettleRWA, MsgUpdateRestrictions, MsgSlashBond, MsgUpdateParams
├── query.proto    GetAsset, AssetsByIssuer, GetRestrictions, GetParams
└── genesis.proto  GenesisState (params, assets, restriction entries)
```

Key types:

```protobuf
enum AssetStatus {
  ASSET_STATUS_UNSPECIFIED = 0;
  ASSET_STATUS_DRAFT       = 1;
  ASSET_STATUS_ATTESTED    = 2;
  ASSET_STATUS_ACTIVE      = 3;
  ASSET_STATUS_SETTLED     = 4;
}

message AssetRecord {
  string      asset_id        = 1;   // slug; denom = "rwa/" + asset_id
  string      issuer          = 2;
  string      name            = 3;
  string      description     = 4;
  AssetStatus status          = 5;
  string      oracle_pair     = 6;   // "BASE:QUOTE"
  string      denom           = 7;   // "rwa/{asset_id}" (derived, stored for convenience)
  string      bond            = 8;   // uvtx locked (>= MinIssuerBond)
  string      notional_minted = 9;   // uvtx; == minted rwa/{id} units (1:1)
  bool        allow_all       = 10;  // transfer-restriction mode
  string      attested_price  = 11;  // oracle snapshot at attest
  google.protobuf.Timestamp attested_at = 12;
  google.protobuf.Timestamp created_at  = 13;
  google.protobuf.Timestamp settled_at  = 14;
}

message RWAParams {
  string min_issuer_bond  = 1; // uvtx amount, e.g. "10000000000"
  string mint_fee_rate    = 2; // LegacyDec, "0.001"
  string settle_fee_rate  = 3; // LegacyDec, "0.001"
}
```

Allow/deny membership is stored in a separate keyed store (`RestrictionEntry` per asset + address), not embedded in `AssetRecord`.

### 3.4 Keeper Boundaries

`x/rwa` depends on:
- `bank.Keeper` — escrow bonds, mint factory denoms, send fees.
- `oracle.Keeper` (`GetPrice`) — attestation validation.
- `auth` fee collector — destination for mint/settle fees.

`x/rwa` exposes minimal public API; most state is queried via gRPC/REST queries.

### 3.5 Fee Math

```
mint_fee   = floor(notional × MintFeeRate)
settle_fee = floor(notional × SettleFeeRate)
```

Fees are paid in `uvtx` and sent to `auth.FeeCollectorName`. They are then split by `x/fees` at the next `EndBlock` (40% burn / 60% distribute).

Notional is declared in `uvtx`; minting is 1:1 (`rwa/{id}` units == notional). The oracle is read only at attestation, not at mint — the fee is a deterministic function of the message (Phase 3 spec D2).

### 3.6 Transfer Restriction Enforcement

Every `MsgTransferRWA` runs:

```
restriction = asset.TransferRestriction
if !restriction.AllowAll:
    require sender ∈ restriction.Allowlist
    require recipient ∈ restriction.Allowlist
require sender ∉ restriction.Denylist
require recipient ∉ restriction.Denylist
```

Plain `x/bank.MsgSend` (and `MsgMultiSend`, authz-wrapped sends, and IBC transfer
escrow) of `rwa/*` denoms is governed by a bank **SendRestrictionFn** registered
via `bankKeeper.AppendSendRestriction(k.SendRestriction)` in `app.go`. Because the
function runs inside `BankKeeper.SendCoins` — the single chokepoint every transfer
path funnels through — issuers cannot be bypassed. The function defers to the same
`CheckTransferAllowed` predicate `MsgTransferRWA` uses, and exempts the `x/rwa`
module account so mint/burn/escrow legs are never blocked. (Refined from the
original "ante decorator" design — Phase 3 spec D5.)

### 3.7 Events

| Event | Attributes |
|---|---|
| `rwa_asset_registered` | `asset_id`, `issuer`, `bond` |
| `rwa_asset_attested` | `asset_id`, `oracle_pair`, `attested_price` |
| `rwa_minted` | `asset_id`, `notional`, `mint_fee` |
| `rwa_settled` | `asset_id`, `settle_fee`, `bond_returned` |
| `rwa_restriction_updated` | `asset_id`, `allow_all`, counts |

### 3.8 Invariants (registered with `x/crisis`)

- Every `ACTIVE` asset has a non-zero bond locked in the module account.
- Sum of all bonds in the module account ≥ Σ `MinIssuerBond` for active assets.
- For every `rwa/{id}` denom, the issuing `AssetRecord` exists and is `ACTIVE` or `SETTLED`.

---

## 4. `x/fees` — Module Design

### 4.1 Responsibilities
Coordination only — no custom state beyond `FeesParams`. Sweeps fee collector at `EndBlock`, splits, dispatches.

### 4.2 Protobuf Surface

```
proto/vertix/fees/v1/
├── types.proto    FeesParams { burn_ratio, distribution_ratio }
├── tx.proto       MsgUpdateParams
├── query.proto    GetParams
└── genesis.proto  GenesisState
```

### 4.3 EndBlock Algorithm

Burn-only sweep (spec D1). `x/fees` does **not** call `x/distribution`; the `DistributionRatio` portion stays in the fee collector for the SDK's native `BeginBlock` to pay stakers.

```
EndBlock(ctx):
    params = GetParams(ctx)
    if params.BurnRatio == 0: return
    feeCollector = accountKeeper.GetModuleAddress(auth.FeeCollectorName)
    uvtx = bankKeeper.GetBalance(feeCollector, "uvtx")   # native token only
    if uvtx == 0: return
    burn = floor(uvtx × params.BurnRatio)
    if burn == 0: return                                  # dust guard
    bankKeeper.SendCoinsFromModuleToModule(FeeCollector → fees, burn)
    bankKeeper.BurnCoins(fees, burn)
    emit EventFeeBurned(amount=burn)
    emit EventFeeDistributed(amount = uvtx − burn)        # left for native x/distribution BeginBlock
# The DistributionRatio (remainder) is distributed to stakers by the
# unmodified x/distribution BeginBlock next block — x/fees does not call it.
```

### 4.4 Parameter Constraints

- `0 ≤ BurnRatio ≤ 1`
- `0 ≤ DistributionRatio ≤ 1`
- `BurnRatio + DistributionRatio == 1` (validated in `Params.Validate()` and on `MsgUpdateParams`)

### 4.5 Events

| Event | Attributes |
|---|---|
| `fee_burned` | `amount` (sdk.Coins) |
| `fee_distributed` | `amount` (sdk.Coins) |

### 4.6 Invariants

- Total `uvtx` burned (via `EventFeeBurned`) is monotonic and ≤ genesis supply − current supply.
- The `x/fees` module account balance is zero at every block boundary (it holds coins only transiently within `EndBlock` between the move and the burn).

`x/fees` uses a burn-only sweep (spec D1): the `DistributionRatio` portion stays in the fee collector and is distributed to stakers by the native `x/distribution` `BeginBlock`. Set `community_tax = 0` in genesis so the full `DistributionRatio` reaches stakers (Phase 0 owns that genesis value).

---

## 5. `vertix-feeder` Sidecar — Design

### 5.1 Architecture

```
   ┌────────────────────────────────────────────────────┐
   │                    main loop                       │
   │                                                    │
   │  ticker (FeedInterval, default 5s)                 │
   │       │                                            │
   │       ▼                                            │
   │  ┌──────────────────┐                              │
   │  │ for pair in cfg: │                              │
   │  │   prices = []    │                              │
   │  │   for prov in    │                              │
   │  │     providers:   │                              │
   │  │     prices ←─    │                              │
   │  │     prov.Fetch() │                              │
   │  │   median = med() │                              │
   │  │   broadcaster    │                              │
   │  │     .Submit(     │                              │
   │  │      pair,median)│                              │
   │  └──────────────────┘                              │
   │                                                    │
   │  metrics: feeds_total, errors_total,              │
   │           provider_latency_ms                      │
   └────────────────────────────────────────────────────┘
```

### 5.2 Configuration (YAML)

```yaml
chain_id: vertix-devnet-1
node_grpc: localhost:9090
node_lcd: http://localhost:1317
validator_key: vtxvaloper1...      # operator address
key_name: validator
keyring_backend: file              # file | os | test
keyring_dir: ~/.vertix
feed_interval: 5s
providers: ["coingecko", "binance"]
pairs:
  - pair: VTX:USD
    symbols:
      coingecko: vertix
      binance:   VTXUSDT
  - pair: BTC:USD
    symbols:
      coingecko: bitcoin
      binance:   BTCUSDT
prometheus:
  enabled: true
  port: 9200
```

### 5.3 Provider Interface

```go
type PriceProvider interface {
    Name() string
    Fetch(ctx context.Context, symbol string) (math.LegacyDec, error)
}
```

### 5.4 Metrics Surface (Prometheus)

| Metric | Type | Labels |
|---|---|---|
| `vertix_feeder_feeds_submitted_total` | Counter | `pair` |
| `vertix_feeder_feeds_failed_total` | Counter | `pair`, `reason` |
| `vertix_feeder_provider_latency_ms` | Histogram | `provider`, `pair` |
| `vertix_feeder_provider_errors_total` | Counter | `provider` |
| `vertix_feeder_last_submit_timestamp` | Gauge | `pair` |

### 5.5 Operational Contract with `x/oracle`

- The validator delegates a **feeder** account via `MsgSetFeeder`; the sidecar signs `MsgSubmitFeed` with that **delegated feeder key only** — never the operator key (Phase 1 spec D10/C1). The feeder key is low-value and rotatable: a compromise lets an attacker submit feeds (bounded by outlier slashing) but touches no funds, and the operator can re-delegate to rotate it.
- The feeder must stay hot enough to submit each window, and must submit **all `accept_list` pairs** it can; a validator "misses" a window if it fails to submit any pair **that reached quorum** that window (under-covered/new pairs do not count).
- A submission is "missed" if no `MsgSubmitFeed` for this validator on a quorum-live accepted pair lands during the window.
- Misses exceeding `MissThreshold` over a tumbling `MissWindowSize` window trigger a `MissSlashRate` slash; outlier submissions (vs the unweighted window median) trigger `OutlierSlashRate`.

---

## 6. Security Considerations

### 6.1 Slash Math Summary

| Event | Default Rate | Notes |
|---|---|---|
| Double-sign (consensus) | 5.00% | Standard SDK |
| Downtime jail | 0.01% | Standard SDK |
| Oracle miss | 0.50% | Vertix custom (`MissSlashRate`) |
| Oracle outlier | 1.00% | Vertix custom (`OutlierSlashRate`) |
| RWA fraud / dispute | issuer bond (governance vote) | Vertix custom |

### 6.2 Threat Model Highlights

- **Oracle manipulation:** mitigated by quorum-gated stake-weighted median (a pair needs ≥ `QuorumFraction` of bonded power before it produces a price, so a small minority cannot set it) plus outlier slashing measured against the **unweighted** window median. The unweighted reference is deliberate: comparing against the stake-weighted price would let a >50%-stake validator define the band and slash the honest minority (Phase 1 spec H1). A high-stake validator pushing an off-market price is still flagged as an outlier (it deviates from the count-based median) unless it also controls a majority of *distinct validators*, which the bonding/`min_commission` requirements make expensive.
- **RWA bond bypass:** mint and ante checks both validate `AssetStatus == ACTIVE` and a bonded record. Module account balance invariants caught by `x/crisis`.
- **Inflation attack:** `x/mint` is not registered; presence is asserted in `app/app_test.go`. Adding it back requires a hard-fork upgrade plus governance.
- **Fee bypass:** `MsgMintRWA` and `MsgSettleRWA` compute fees inside the keeper; fee debit is part of the same atomic message handler.
- **IBC liveness:** Hermes monitored; client refresh strategy configured per channel.

### 6.3 Required Tests

- Unit: every keeper method, every `ValidateBasic`, every state-machine transition (positive and negative).
- Simulation: random operations on the three custom modules.
- Integration: full RWA lifecycle via `interchaintest`.
- IBC E2E: ICS-20 of VTX and `rwa/*` between two Vertix instances.

---

## 7. Cross-Module Wiring (in `app/app.go`)

```go
// Construction order (after staking, slashing, bank, distribution, auth):

app.OracleKeeper = oraclekeeper.NewKeeper(
    appCodec,
    runtime.NewKVStoreService(keys[oracletypes.StoreKey]),
    app.StakingKeeper,
    app.SlashingKeeper,
    authtypes.NewModuleAddress(govtypes.ModuleName).String(),
)

app.FeesKeeper = feeskeeper.NewKeeper(
    appCodec,
    runtime.NewKVStoreService(keys[feestypes.StoreKey]),
    app.AccountKeeper,
    app.BankKeeper,
    app.DistrKeeper,
    authtypes.NewModuleAddress(govtypes.ModuleName).String(),
)

app.RWAKeeper = rwakeeper.NewKeeper(
    appCodec,
    runtime.NewKVStoreService(keys[rwatypes.StoreKey]),
    app.BankKeeper,
    app.OracleKeeper,             // OracleKeeper interface
    app.AccountKeeper,
    authtypes.NewModuleAddress(govtypes.ModuleName).String(),
)
```

**EndBlock order:** `oracle → rwa → fees`.
- Oracle aggregates first so RWA queries see the latest price within the same block boundary.
- RWA forwards fees to the fee collector.
- Fees sweeps the fee collector last.

**InitGenesis order (custom prefix):** `staking → oracle → rwa → fees`.

---

## 8. Governance & Upgrade Path

| Channel | Use |
|---|---|
| `MsgUpdateParams` per custom module | Tune `BurnRatio`, `MintFeeRate`, `MissThreshold`, etc. |
| `x/upgrade` software upgrade proposal | Binary swap via Cosmovisor |
| `x/gov` text proposal | Off-chain coordination, RWA dispute outcomes |

Every `MsgUpdateParams` requires `authority == govtypes.ModuleName` address. Param validation runs inside the message handler before persistence.

For binary upgrades, every store-key change must ship a migration handler in the upgrade plan.

---

## 9. Reference: Module API Cheat Sheet

```go
// x/oracle
GetPrice(ctx, pair) (math.LegacyDec, error)   // ErrNoPrice | ErrStalePrice
GetTWAP(ctx, pair, window) (math.LegacyDec, error)
SetFeeder(MsgSetFeeder)        // operator delegates a feeder key
SubmitFeed(MsgSubmitFeed) → MsgSubmitFeedResponse   // signed by the delegated feeder
UpdateParams(MsgUpdateParams)

// x/rwa
RegisterAsset → AssetRecord (DRAFT)
AttestAsset   → AssetRecord (ATTESTED)  [reads OracleKeeper.GetPrice]
MintRWA       → bank mint rwa/{id} + fee
TransferRWA   → restriction-checked transfer
SettleRWA     → bank burn rwa/{id} + fee + bond release/slash
UpdateRestrictions
UpdateParams

// x/fees
EndBlock     → sweep fee collector → burn + distribute
UpdateParams (governance only)
```
