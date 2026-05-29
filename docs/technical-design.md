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

### 2.1 Responsibilities
- Accept per-validator price submissions per `MsgSubmitFeed`.
- At end of each `VoteWindow`, aggregate via stake-weighted median per pair.
- Maintain TWAP histories per pair.
- Track per-validator misses; trigger slashes via `x/slashing`.
- Expose a clean keeper interface to `x/rwa`.

### 2.2 Protobuf Surface

```
proto/vertix/oracle/v1/
├── types.proto    OracleFeed, AggregatedPrice, TWAPEntry, OracleParams
├── tx.proto       MsgSubmitFeed, MsgUpdateParams (+ responses)
├── query.proto    GetPrice, GetTWAP, GetParams, GetMissCounter
└── genesis.proto  GenesisState
```

Key messages:

```protobuf
message MsgSubmitFeed {
  option (cosmos.msg.v1.signer) = "validator";
  string validator = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string pair      = 2;   // "BASE:QUOTE", e.g. "VTX:USD"
  string price     = 3;   // LegacyDec encoded as string
}

message OracleParams {
  int64  vote_window         = 1; // blocks per aggregation window
  string miss_threshold      = 2; // LegacyDec, e.g. "0.05"
  string miss_slash_rate     = 3; // LegacyDec, e.g. "0.005"
  string outlier_slash_rate  = 4; // LegacyDec, e.g. "0.01"
  repeated string accept_list = 5; // whitelisted pairs
}
```

### 2.3 Keeper Interfaces

**Public (consumed by other modules):**

```go
type OracleKeeper interface {
    GetPrice(ctx context.Context, pair string) (math.LegacyDec, error)
    GetTWAP(ctx context.Context, pair string, window time.Duration) (math.LegacyDec, error)
}
```

**External dependencies (held by keeper):**

```go
type StakingKeeper interface {
    GetValidatorByConsAddr(ctx context.Context, addr sdk.ConsAddress) (stakingtypes.Validator, error)
    GetLastValidatorPower(ctx context.Context, addr sdk.ValAddress) (int64, error)
    GetBondedValidatorsByPower(ctx context.Context) ([]stakingtypes.Validator, error)
}

type SlashingKeeper interface {
    Slash(ctx context.Context, consAddr sdk.ConsAddress, fraction math.LegacyDec, power, distributionHeight int64) (math.Int, error)
    Jail(ctx context.Context, consAddr sdk.ConsAddress) error
}
```

### 2.4 Store Layout

| Prefix | Key | Value |
|---|---|---|
| `0x01` | `{validator}/{pair}` | `OracleFeed` (current window) |
| `0x02` | `{pair}` | `AggregatedPrice` (latest) |
| `0x03` | `{pair}/{ts_unixnano_be}` | `TWAPEntry` (history) |
| `0x04` | `{validator}` | `int64` miss counter |
| `0x05` | `(empty)` | `int64` total completed windows |
| `0x06` | `(empty)` | `OracleParams` |

Keys are big-endian encoded for ordered iteration where required (TWAP timestamp scans).

### 2.5 ABCI: `EndBlock` Algorithm

```
EndBlock(ctx):
    params = GetParams(ctx)
    if BlockHeight % params.VoteWindow != 0:
        return                                           # mid-window: no-op

    windowNumber = BlockHeight / params.VoteWindow

    # 1. Aggregate per pair
    for pair in params.AcceptList:
        feeds = GetAllFeedsForPair(pair)
        if feeds is empty: continue
        prices, weights = (f.Price, validatorPower(f.Validator)) for f in feeds
        median = WeightedMedian(prices, weights)
        SetAggregatedPrice(pair, median, height, blockTime)
        AppendTWAPEntry(pair, median, blockTime)
        emit EventPriceAggregated

    # 2. Slash misses
    for v in BondedValidatorsByPower():
        miss = GetMissCounter(v.Operator)
        threshold = params.MissThreshold * windowNumber
        if miss > threshold:
            slashingKeeper.Slash(consAddr, params.MissSlashRate, power, height)
            emit EventOracleSlash
        ResetMissCounter(v.Operator)

    # 3. Clear feeds for next window
    DeleteAllFeeds()
```

**Stake-weighted median** (`WeightedMedian(prices, weights)`):
- Sort `(price, weight)` pairs by price ascending.
- Walk in order, accumulating weight; the first price whose cumulative weight ≥ `totalWeight / 2` is the result.

### 2.6 Slashing Conditions

| Trigger | Rate | Module that executes |
|---|---|---|
| Miss rate > `MissThreshold` (default 5%) over completed windows | `MissSlashRate` (default 0.5%) | `x/slashing.Slash` (called from `x/oracle.EndBlock`) |
| Outlier submission (z-score / interquartile threshold) | `OutlierSlashRate` (default 1.0%) | Same |

Note: oracle slashing is intentionally lighter than double-sign (5%) to avoid validator chilling and accidental jailing.

### 2.7 Events

| Event | Attributes |
|---|---|
| `oracle_feed_submitted` | `validator`, `pair`, `price` |
| `oracle_price_aggregated` | `pair`, `price` |
| `oracle_slash` | `validator`, `slash_reason`, `slash_amount` |

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
├── types.proto    AssetRecord, AssetStatus, TransferRestriction, RWAParams
├── tx.proto       MsgRegisterAsset, MsgAttestAsset, MsgMintRWA, MsgTransferRWA,
│                  MsgSettleRWA, MsgUpdateRestrictions, MsgUpdateParams
├── query.proto    GetAsset, AssetsByIssuer, GetParams
└── genesis.proto  GenesisState
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
  string      asset_id    = 1;
  string      issuer      = 2;
  string      name        = 3;
  string      description = 4;
  AssetStatus status      = 5;
  string      oracle_pair = 6;
  // ... bond, denom, restriction, timestamps
}

message RWAParams {
  string min_issuer_bond  = 1; // uvtx amount, e.g. "10000000000"
  string mint_fee_rate    = 2; // LegacyDec, "0.001"
  string settle_fee_rate  = 3; // LegacyDec, "0.001"
}
```

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

Plain `x/bank.MsgSend` of `rwa/*` denoms is blocked by an ante decorator that defers to the same logic — issuers cannot be bypassed via the standard bank transfer path.

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

```
EndBlock(ctx):
    params = GetParams(ctx)
    feeCollector = authKeeper.GetModuleAddress(auth.FeeCollectorName)
    balances = bankKeeper.GetAllBalances(feeCollector)
    if balances.IsZero(): return

    for coin in balances:
        burn   = coin.Amount × params.BurnRatio          # rounded down
        distr  = coin.Amount − burn

        if burn > 0:
            bankKeeper.SendCoinsFromModuleToModule(
                feeCollector → fees module, burn)
            bankKeeper.BurnCoins(fees module, burn coins)
            emit EventFeeBurned(amount=burn)

        if distr > 0:
            distributionKeeper.AllocateTokensToFeePool(distr coins)
            emit EventFeeDistributed(amount=distr)
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

- After each `EndBlock`, `FeeCollector` balance for `uvtx` is zero (everything was swept).
- Total `uvtx` burned (via `EventFeeBurned`) ≤ original genesis supply minus current supply.

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

- The validator **must** keep the feeder's broadcast keys hot enough to sign every block of every window.
- A submission is "missed" if no `MsgSubmitFeed` from this validator for an accepted pair lands during the window.
- Misses across `MissThreshold` of windows trigger a `MissSlashRate` slash.

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

- **Oracle manipulation:** mitigated by stake-weighted median + outlier slashing. Feed pump from a single high-stake validator still costs them on outlier detection.
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
GetPrice(ctx, pair) (math.LegacyDec, error)
GetTWAP(ctx, pair, window) (math.LegacyDec, error)
SubmitFeed(MsgSubmitFeed) → MsgSubmitFeedResponse
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
