# Phase 3 — `x/rwa` (Lifecycle + Bonding) — Design Spec

**Date:** 2026-05-29
**Status:** Approved (brainstorm output)
**Phase:** 3 of the canonical phase map ([`full-design-spec.md`](../full-design-spec.md) §7.2)
**Depends on:** Phase 1 (`x/oracle` — `OracleKeeper` interface), Phase 2 (`x/fees` — fee-collector sink); SDK `bank`, `distribution`, `auth`, `params` · **Enables:** Phase 6 (full RWA lifecycle on devnet), Phase 5 (`rwa/*` denom portability over ICS-20)
**Owns:** the RWA asset registry, the `DRAFT → ATTESTED → ACTIVE → SETTLED` lifecycle, issuer-bond escrow, `rwa/{asset-id}` factory-denom mint/burn, per-asset transfer restrictions (incl. the bank send-restriction that closes the bypass), oracle-gated attestation, and mint/settle fee routing into the fee collector.

> This is the per-phase design spec produced by the brainstorm of Phase 3. It refines — and must stay consistent with — the program source of truth ([`full-design-spec.md`](../full-design-spec.md) Phase 3) and the engineering reference ([`technical-design.md`](../technical-design.md) §3). Where this doc and the program spec disagree on phase order or cross-phase contracts, the program spec wins. Module internals defer to `technical-design.md`, except where this doc explicitly refines it (flagged as doc-sync items in §13). The step-by-step execution plan lives in `docs/plans/` (produced after this spec).

---

## 0. What This Brainstorm Resolved

`technical-design.md` §3 already locks the lifecycle, the proto sketch, the fee math, the events, and the `x/crisis` invariants. The Phase 3 brainstorm resolved the items the program spec explicitly deferred to it (Appendix C) plus the under-specified mechanics needed to make the module plan-ready:

| ID | Deferred / open item | Resolution (this spec) |
|---|---|---|
| **D1** | Asset identity & denom shape (under-specified) | Issuer-supplied human-readable slug; charset + uniqueness validated; `denom = rwa/{slug}` |
| **D2** | Mint semantics & notional units (under-specified) | Notional declared directly in `uvtx`; mint `rwa/{id}` **1:1**; fee is a **deterministic** function of the message (no oracle read at mint) |
| **D4** | **Transfer-restriction model** (Appendix C open question) | **Separate keyed store** for allow/deny membership; `AssetRecord` holds only `allow_all`; O(1) checks |
| **D5** | Bypass enforcement mechanism | **Bank `SendRestrictionFn`** (not an ante decorator) — supersedes the §3.6 "ante decorator" wording (doc-sync §13) |
| **D7** | **Dispute resolution** (Appendix C open question) | **Governance-only** `MsgSlashBond`; force-settles the asset and routes the bond to the **community pool** |
| **D6/D8/D9/D10** | Bond shape, settle authority, fee-shortfall behavior, default restriction mode | Issuer bond `≥ MinIssuerBond`; clean settle is issuer-only; fee shortfall fails atomically; new assets default to `allow_all = true` |

---

## 1. Goal & Deliverable Boundary

Deliver a working `x/rwa` Cosmos SDK module: a protocol-agnostic asset registry that enforces the `DRAFT → ATTESTED → ACTIVE → SETTLED` lifecycle with issuer **bond escrow**, **oracle-gated attestation**, `rwa/{asset-id}` **factory-denom** mint/burn, per-asset **transfer restrictions** (enforced even against plain `x/bank` sends and IBC escrow), and **mint/settle fee routing** into `auth.FeeCollectorName` (where Phase 2's `x/fees` sweeps it). Governance can slash an issuer's bond on a dispute.

**In scope:**

- Protobuf surface: `MsgRegisterAsset`, `MsgAttestAsset`, `MsgMintRWA`, `MsgTransferRWA`, `MsgSettleRWA`, `MsgUpdateRestrictions`, `MsgSlashBond` (gov), `MsgUpdateParams` (gov); queries `Asset`, `AssetsByIssuer`, `Restrictions`, `Params`; `AssetRecord`, `AssetStatus`, `RWAParams`, genesis.
- Lifecycle state machine with per-transition guards; issuer-bond escrow in the `x/rwa` module account.
- Oracle-gated attestation via `OracleKeeper.GetPrice` (treating `ErrNoPrice`/`ErrStalePrice` as failures); `attested_price` snapshot.
- `rwa/{asset-id}` factory-denom mint (on `MintRWA`) and burn (on `SettleRWA`) via `x/bank`.
- Per-asset transfer restrictions (allow/deny) with O(1) membership checks, plus a **bank `SendRestrictionFn`** that closes the `MsgSend`/`MultiSend`/`authz`/IBC bypass.
- Deterministic mint/settle fee routing to the fee collector.
- Governance bond slash (`MsgSlashBond`) routing the bond to the community pool.
- Params + genesis + events + gRPC/CLI queries; the three `x/crisis` invariants (§3.8).
- Unit + keeper-integration + app-level tests satisfying the Phase 3 acceptance gate.

**Out of scope (deferred):**

- Fee **burn/distribute** mechanics — owned by Phase 2 `x/fees`; `x/rwa` only *routes* fees to the collector, never burns/distributes directly.
- Oracle **price aggregation** — owned by Phase 1; `x/rwa` is a pure consumer of `GetPrice`.
- ICS-20 transfer of `rwa/*` — wired in Phase 5; this spec only guarantees the send-restriction semantics that Phase 5 must preserve.
- A full on-chain **dispute state machine** (records/voting/timeouts) — governance provides that machinery; a richer module is post-mainnet if ever needed.
- **Notional-scaled bonds** and **physical-unit** token semantics — documented future options, not launch scope (D6/D2 rationale).

---

## 2. Locked Decisions (from this brainstorm)

| # | Decision | Choice | Rationale |
|---|---|---|---|
| D0 | Module creation | **`ignite scaffold module rwa --dep bank,oracle,params`, then adapt** | Consistent with Phase 1/2; inherits depinject wiring, the `buf`/`make proto-gen` pipeline, autocli, and scaffold layout; slots into the existing `app_config.go`/`app.go` markers. The `technical-design.md` §7 `NewKeeper(...)` snippet remains *illustrative*; the binding contract is the module set, EndBlock/genesis ordering, and the cross-phase contracts consumed. |
| D1 | Asset identity & denom | **Issuer-supplied human-readable slug**; validated `^[a-z0-9][a-z0-9-]{2,42}$`; globally unique; `denom = "rwa/" + slug` | Human-readable denoms aid explorers/wallets and the Phase 5 assetlist. The slug regex is a strict subset of the bank denom regex (`[a-zA-Z][a-zA-Z0-9/:._-]{2,127}`) so `rwa/{slug}` is always a valid denom and never collides with the `rwa/` prefix or another asset. Chain-generated IDs (sequential/hash) were rejected as less operable; issuer+suffix adds composition complexity for no real uniqueness benefit once the registry enforces global uniqueness. |
| D2 | Mint semantics & fee | Issuer declares **notional in `uvtx`**; mint exactly `notional` units of `rwa/{id}` (**1:1**); `mint_fee = floor(notional × MintFeeRate)`; the oracle is **not** read at mint | **Fee determinism (Invariant 5).** A fee that depended on a live `GetPrice` at mint time would vary block-to-block and could *fail* on staleness, coupling a value-transfer message to oracle liveness. Making the fee a pure function of the message keeps the §3.8/§4.6-style invariants trivial and mint deterministic. The oracle's role is correctly placed at **attestation** (D3). The token amount is denominated in VTX-notional rather than physical units; acceptable for a protocol-agnostic launch (physical-unit scaling is a post-launch option). |
| D3 | Oracle role + snapshot | `GetPrice(oracle_pair)` is the **attestation gate**; on success snapshot `attested_price` + `attested_at` onto the record; `ErrNoPrice` **and** `ErrStalePrice` are attestation failures | Honors the Phase 1 cross-phase contract (Phase 3 must treat stale/absent prices as failures, protecting Invariant 3). The snapshot backs the `rwa_asset_attested` event's `attested_price` and gives a permanent audit trail for later dispute review. |
| D4 | **Transfer-restriction storage** | **Separate keyed store**: `AssetRecord` holds only `allow_all`; allow/deny membership are individual keys `0x03|{asset_id}|{addr}` / `0x04|{asset_id}|{addr}`; checks are O(1) `Has` lookups | **Resolves Appendix C.** Inline arrays force deserializing the whole list on *every* transfer and rewrite the record on every edit — Phase 7 forbids unbounded enumeration in hot paths. Keyed membership gives bounded gas regardless of list size, cheap single-entry edits, and clean prefix-iteration for genesis export. Mirrors the `x/oracle` feeder-index pattern (`0x07`/`0x08`). |
| D5 | **Bypass enforcement** | A **bank `SendRestrictionFn`** registered via `bankKeeper.AppendSendRestriction(k.SendRestriction)` — **not** an ante decorator | **Refines `technical-design.md` §3.6 (doc-sync §13).** A `SendRestrictionFn` runs inside `BankKeeper.SendCoins`, the single chokepoint that `MsgSend`, `MsgMultiSend`, `authz.MsgExec`, and **IBC transfer escrow** all funnel through. An ante decorator only sees the outer tx message list (bypassable via `authz`/`group` wrappers, blind to IBC); `SendEnabled=false` is not checked on the `SendCoins` path IBC uses (so it would leave the Phase 5 portability contract violated). The send-restriction is the only mechanism that closes all bypasses and shares logic with `MsgTransferRWA`. |
| D6 | Issuer bond | Issuer locks `bond ≥ MinIssuerBond` (`uvtx`), stored per-asset on the record; full return on clean settle | One-line stricter than a fixed bond but lets a serious issuer post a larger credible bond. The `bond` field is needed regardless (invariants sum it). Notional-scaled bonds (top-up at mint) were rejected as out-of-spec complexity; governance can raise `MinIssuerBond`. |
| D7 | **Dispute resolution** | **Governance-only** `MsgSlashBond { authority, asset_id }` (`authority == gov module addr`); force-settles `ACTIVE → SETTLED` and routes the bond to the **community pool** (`x/distribution` fund) | **Resolves Appendix C.** Matches §6.1 ("issuer bond — governance vote") and keeps the locked 4-state machine intact (no `FROZEN` state). A full on-chain dispute subsystem is YAGNI for launch; `x/gov` already provides voting/timeouts. The community pool keeps the slashed value under governance control (a follow-up spend proposal can compensate harmed parties) rather than silently burning it. |
| D8 | Clean settle authority | `MsgSettleRWA` is **issuer-only** | Keeps the "return bond" path (issuer) cleanly separate from the "slash bond" path (governance, D7). Folding governance into `SettleRWA` would blur the two bond destinations into one message. |
| D9 | Fee shortfall | If the issuer lacks `uvtx` for the mint/settle fee, the message **fails atomically** (no partial mint/settle); the bond is never touched | Standard Cosmos atomicity. Deducting from the bond could drop it below `MinIssuerBond`, breaking the §3.8 "every `ACTIVE` asset bonded ≥ minimum" invariant. |
| D10 | Default restriction mode | New assets register with `allow_all = true`; issuers opt into restrictions via `MsgUpdateRestrictions` | Protocol-agnostic, least surprise: a freshly registered asset is transferable. Deny-by-default would leave every new asset untransferable until configured (a footgun); restriction-by-default issuers send one `MsgUpdateRestrictions` after register. |
| D11 | No EndBlock logic | `x/rwa` registers **no** `EndBlock` work of its own; it slots in the `oracle → rwa → fees` EndBlock order purely for module-manager positioning | All RWA state changes are message-driven; fees it routes are swept by `x/fees` in the *same* block's later EndBlock. No begin-block work either. |

These inherit and must not contradict the program-level locked decisions in [`full-design-spec.md`](../full-design-spec.md) §§1–6, nor the architectural invariants §5.

---

## 3. Target Module Layout (Phase 3 outputs)

```
x/rwa/
├── keeper/
│   ├── keeper.go            store ops: asset CRUD, issuer index, params; keeper struct + deps
│   ├── msg_server.go        RegisterAsset, AttestAsset, MintRWA, TransferRWA, SettleRWA,
│   │                        UpdateRestrictions, SlashBond, UpdateParams
│   ├── grpc_query.go        Asset, AssetsByIssuer, Restrictions, Params
│   ├── asset.go             lifecycle transitions + transition-guard helpers
│   ├── bond.go              bond escrow: lock (register), release (settle), slash→community pool
│   ├── restriction.go       allow/deny membership store ops + checkTransferAllowed
│   ├── send_restriction.go  SendRestriction(ctx, from, to, amt) bank hook (D5)
│   ├── fee.go               mint/settle fee math + send to FeeCollectorName
│   ├── denom.go             rwa/{slug} build + parse (parseRWADenom)
│   ├── genesis.go           InitGenesis / ExportGenesis
│   └── *_test.go
├── types/
│   ├── keys.go              store prefixes 0x01–0x05; key builders; RWADenomPrefix
│   ├── params.go            RWAParams defaults + Validate()
│   ├── msgs.go              ValidateBasic for all messages; slug regex
│   ├── expected_keepers.go  BankKeeper, OracleKeeper, AccountKeeper, DistributionKeeper
│   ├── errors.go            ErrAssetExists, ErrAssetNotFound, ErrInvalidStatus,
│   │                        ErrBondTooLow, ErrAttestationFailed, ErrTransferRestricted,
│   │                        ErrUnauthorized, ErrInvalidNotional, ...
│   ├── events.go, codec.go, genesis.go
│   └── *.pb.go              generated
├── module/
│   ├── module.go            AppModule (genesis, services); ProvideModule (depinject)
│   ├── autocli.go           query + tx command wiring (UpdateParams/SlashBond gov-only, skipped)
│   └── simulation.go        sim hooks (stub-level, mirrors oracle/fees)
└── simulation/helpers.go
proto/vertix/rwa/v1/
├── types.proto   AssetRecord, AssetStatus, RWAParams
├── tx.proto      MsgRegisterAsset, MsgAttestAsset, MsgMintRWA, MsgTransferRWA,
│                 MsgSettleRWA, MsgUpdateRestrictions, MsgSlashBond, MsgUpdateParams (+ responses)
├── query.proto   Asset, AssetsByIssuer, Restrictions, Params
└── genesis.proto GenesisState
proto/vertix/rwa/module/module.proto   depinject module config
testutil/keeper/rwa.go                 test keeper + mock Bank/Oracle/Account/Distribution
```

This realizes the `x/rwa` rows of [`project-structure.md`](../project-structure.md) and the §3.3 protobuf layout of [`technical-design.md`](../technical-design.md).

---

## 4. App Wiring (depinject)

The scaffold inserts rwa at the `# stargate/app/...` markers established in Phase 0:

- **`app_config.go`:**
  - `ModuleConfig` for `rwa` appended at `# stargate/app/moduleConfig`.
  - `rwatypes.ModuleName` inserted into `endBlockers` **between `oracle` and `fees`** to realize the canonical `oracle → rwa → fees` order (§7 of `technical-design.md`). (RWA has no EndBlock work — D11 — but the position is fixed by spec.)
  - `rwatypes.ModuleName` inserted into `genesisModuleOrder` **after `oracle` and before `fees`** (`staking → oracle → rwa → fees`), so the oracle is initialized before RWA genesis and the fee collector exists before fees genesis.
  - `moduleAccPerms` gains `{Account: rwatypes.ModuleName, Permissions: [Minter, Burner]}` — RWA mints/burns `rwa/{id}` factory denoms and escrows bonds.
  - No `beginBlockers` / `preBlockers` entry.
- **`app.go`:** `RWAKeeper` field added to the `App` struct and to the `depinject.Inject` target.
- **Send restriction:** after keepers are constructed, register `app.BankKeeper.AppendSendRestriction(app.RWAKeeper.SendRestriction)` (D5). This is additive to any existing restriction (e.g. blocked addresses) and must be wired in `app.go`, not inside depinject `ProvideModule`, because it mutates the bank keeper post-construction.
- **Authority:** `MsgUpdateParams.authority` and `MsgSlashBond.authority` must equal `authtypes.NewModuleAddress(govtypes.ModuleName).String()`.

**Expected external keepers** (held by the rwa keeper, depinject-provided):

```go
type BankKeeper interface {
    MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
    BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
    SendCoinsFromAccountToModule(ctx context.Context, sender sdk.AccAddress, recipientModule string, amt sdk.Coins) error
    SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipient sdk.AccAddress, amt sdk.Coins) error
    SendCoinsFromModuleToModule(ctx context.Context, senderModule, recipientModule string, amt sdk.Coins) error
    SendCoins(ctx context.Context, from, to sdk.AccAddress, amt sdk.Coins) error   // restriction-checked transfer path
    GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
}

// Note: AppendSendRestriction(k.SendRestriction) is called on the concrete
// app.BankKeeper in app.go (§4), not through this interface — the rwa keeper
// never calls it, it only provides the SendRestrictionFn.

// GetPrice is the only method needed (attestation gate). Verbatim from the
// Phase 1 OracleKeeper contract; ErrNoPrice AND ErrStalePrice are failures.
type OracleKeeper interface {
    GetPrice(ctx context.Context, pair string) (math.LegacyDec, error)
}

type AccountKeeper interface {
    GetModuleAddress(name string) sdk.AccAddress
}

// For the dispute bond slash (D7).
type DistributionKeeper interface {
    FundCommunityPool(ctx context.Context, amount sdk.Coins, sender sdk.AccAddress) error
}
```

> The rwa keeper also needs the fee-collector destination; it sends fees with `SendCoinsFromModuleToModule(rwa, authtypes.FeeCollectorName, fee)` after the payer funds the rwa module account, or directly `SendCoinsFromAccountToModule(payer, FeeCollectorName, fee)`. The plan picks the single-hop form (`SendCoinsFromAccountToModule(payer → FeeCollectorName)`) so the fee never transits the rwa module account (keeps the §3.8-style "rwa balance = Σ bonds only" reasoning clean).

---

## 5. Protobuf Surface (`proto/vertix/rwa/v1/`)

Additive-only from first generation onward; no field renumbering or removals (coding-standards proto convention).

### 5.1 `types.proto`

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
  string      issuer          = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string      name            = 3;
  string      description     = 4;
  AssetStatus status          = 5;
  string      oracle_pair     = 6;   // "BASE:QUOTE", validated against oracle accept_list at attest
  string      denom           = 7;   // "rwa/{asset_id}" (derived, stored for convenience)
  string      bond            = 8 [(cosmos_proto.scalar) = "cosmos.Int"];   // uvtx locked (>= MinIssuerBond)
  string      notional_minted = 9 [(cosmos_proto.scalar) = "cosmos.Int"];   // uvtx; == minted rwa/{id} units (D2)
  bool        allow_all       = 10;  // transfer-restriction mode (D4/D10)
  string      attested_price  = 11 [(cosmos_proto.scalar) = "cosmos.Dec"];  // snapshot at attest (D3)
  google.protobuf.Timestamp attested_at = 12 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
  google.protobuf.Timestamp created_at  = 13 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
  google.protobuf.Timestamp settled_at  = 14 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
}

message RWAParams {
  string min_issuer_bond = 1 [(cosmos_proto.scalar) = "cosmos.Int"];  // "10000000000" (10,000 VTX)
  string mint_fee_rate   = 2 [(cosmos_proto.scalar) = "cosmos.Dec"];  // "0.001"
  string settle_fee_rate = 3 [(cosmos_proto.scalar) = "cosmos.Dec"];  // "0.001"
}
```

### 5.2 `tx.proto`

```protobuf
service Msg {
  rpc RegisterAsset(MsgRegisterAsset)             returns (MsgRegisterAssetResponse);
  rpc AttestAsset(MsgAttestAsset)                 returns (MsgAttestAssetResponse);
  rpc MintRWA(MsgMintRWA)                         returns (MsgMintRWAResponse);
  rpc TransferRWA(MsgTransferRWA)                 returns (MsgTransferRWAResponse);
  rpc SettleRWA(MsgSettleRWA)                     returns (MsgSettleRWAResponse);
  rpc UpdateRestrictions(MsgUpdateRestrictions)   returns (MsgUpdateRestrictionsResponse);
  rpc SlashBond(MsgSlashBond)                     returns (MsgSlashBondResponse);   // gov
  rpc UpdateParams(MsgUpdateParams)               returns (MsgUpdateParamsResponse); // gov
}

message MsgRegisterAsset {
  option (cosmos.msg.v1.signer) = "issuer";
  string issuer      = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string asset_id    = 2;   // slug
  string name        = 3;
  string description  = 4;
  string oracle_pair = 5;   // "BASE:QUOTE"
  string bond        = 6 [(cosmos_proto.scalar) = "cosmos.Int"];   // uvtx; >= MinIssuerBond
}

message MsgAttestAsset {
  option (cosmos.msg.v1.signer) = "issuer";
  string issuer   = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string asset_id = 2;
}

message MsgMintRWA {
  option (cosmos.msg.v1.signer) = "issuer";
  string issuer   = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string asset_id = 2;
  string notional = 3 [(cosmos_proto.scalar) = "cosmos.Int"];   // uvtx; mints `notional` units of rwa/{id} (D2)
}

message MsgTransferRWA {
  option (cosmos.msg.v1.signer) = "sender";
  string sender    = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string recipient = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string asset_id  = 3;
  string amount    = 4 [(cosmos_proto.scalar) = "cosmos.Int"];   // rwa/{id} units
}

message MsgSettleRWA {
  option (cosmos.msg.v1.signer) = "issuer";   // clean settle is issuer-only (D8)
  string issuer   = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string asset_id = 2;
}

message MsgUpdateRestrictions {
  option (cosmos.msg.v1.signer) = "issuer";
  string issuer            = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string asset_id          = 2;
  bool   allow_all         = 3;
  repeated string add_allow = 4 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  repeated string del_allow = 5 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  repeated string add_deny  = 6 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  repeated string del_deny  = 7 [(cosmos_proto.scalar) = "cosmos.AddressString"];
}

message MsgSlashBond {            // governance-only dispute resolution (D7)
  option (cosmos.msg.v1.signer) = "authority";
  string authority = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];   // gov module addr
  string asset_id  = 2;
  string reason    = 3;
}

message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";
  string    authority = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  RWAParams params    = 2 [(gogoproto.nullable) = false];
}
```

> `MsgUpdateRestrictions` add/del batches are bounded per message by `ValidateBasic` (e.g. ≤ 100 entries each) so a single tx cannot do unbounded writes; an issuer manages a large allowlist across multiple txs. This bound is per-tx gas hygiene, not a cap on total list size (which is unbounded by D4's keyed store).

### 5.3 `query.proto`

```protobuf
service Query {
  rpc Asset(QueryAssetRequest)                   returns (QueryAssetResponse);
  rpc AssetsByIssuer(QueryAssetsByIssuerRequest) returns (QueryAssetsByIssuerResponse);   // paginated
  rpc Restrictions(QueryRestrictionsRequest)     returns (QueryRestrictionsResponse);     // paginated
  rpc Params(QueryParamsRequest)                 returns (QueryParamsResponse);
}
// Asset(asset_id) → AssetRecord            (gRPC NotFound = ErrAssetNotFound)
// AssetsByIssuer(issuer, pagination) → repeated AssetRecord
// Restrictions(asset_id, pagination) → { bool allow_all; repeated string allowlist; repeated string denylist }
// Params() → RWAParams
```

### 5.4 `genesis.proto`

```protobuf
message GenesisState {
  RWAParams              params       = 1 [(gogoproto.nullable) = false];
  repeated AssetRecord   assets       = 2 [(gogoproto.nullable) = false];
  repeated Restriction   restrictions = 3 [(gogoproto.nullable) = false];   // flattened allow/deny entries
}

message Restriction {
  string asset_id = 1;
  string address  = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  bool   is_deny  = 3;   // false = allowlist entry, true = denylist entry
}
```

`InitGenesis` validates every imported `AssetRecord` (unique `asset_id`, valid slug + derived denom, `status` valid, `bond ≥ MinIssuerBond` for `ACTIVE`/`ATTESTED` records that hold a bond, parseable `notional_minted`/`attested_price`, `oracle_pair` well-formed) and every `Restriction` (the referenced `asset_id` exists, valid bech32) before writing — malformed imports are a genesis error, never a runtime panic. `ExportGenesis` dumps params, all `0x01` asset records, and all `0x03`/`0x04` membership entries (flattened). `TestGenesisRoundTrip` covers params + assets + restrictions. Genesis does **not** re-mint factory denoms or re-escrow bonds: balances and the module account are part of the `x/bank` genesis, and `InitGenesis` asserts consistency (the bond invariant) rather than moving coins.

---

## 6. Params & Validation (`types/params.go`)

`DefaultParams()`:

| Param | Default | Notes |
|---|---|---|
| `min_issuer_bond` | `10000000000` (10,000 VTX in `uvtx`) | minimum bond to register; gate to `ACTIVE` |
| `mint_fee_rate` | `0.001` (0.10%) | of notional, charged at mint |
| `settle_fee_rate` | `0.001` (0.10%) | of notional, charged at settle |

`Validate()` enforces:

- `min_issuer_bond` is a valid non-negative `Int` (`> 0` recommended; `0` allowed only for test/devnet, warned).
- `mint_fee_rate`, `settle_fee_rate` are valid `LegacyDec` in `[0, 1)`.

`MsgUpdateParams` re-runs `Validate()` in-handler before persisting; `authority` must equal the gov module address. Lowering `min_issuer_bond` does not retroactively under-bond existing assets (their stored `bond` is unchanged); raising it applies only to new registrations. Emits `rwa_params_updated`.

---

## 7. State & Keeper Store Ops (`types/keys.go`, `keeper/keeper.go`)

| Prefix | Key | Value | Operations |
|---|---|---|---|
| `0x01` | `{asset_id}` | `AssetRecord` | set on register/attest/mint/settle/slash; get on every message + `Asset` query |
| `0x02` | `{issuer}/{asset_id}` | `∅` (presence) | set on register; prefix-scan `{issuer}/` for `AssetsByIssuer` (paginated) |
| `0x03` | `{asset_id}/{addr}` | `∅` (presence) | allowlist membership; `Has`/set/delete; prefix-scan for `Restrictions` query + export |
| `0x04` | `{asset_id}/{addr}` | `∅` (presence) | denylist membership; same ops |
| `0x05` | `(empty)` | `RWAParams` | get / set |

Keys use length-prefixed address segments where an address precedes another segment (addresses may contain separator bytes), mirroring the `x/oracle` `0x01` convention. `RWADenomPrefix = "rwa/"`.

**Denom build/parse (`keeper/denom.go`):** `BuildDenom(assetID) = "rwa/" + assetID`; `parseRWADenom(denom) → (assetID, ok)` strips the `rwa/` prefix and returns `ok=false` for any non-`rwa/*` denom (so the send-restriction is a cheap no-op for `uvtx` and IBC vouchers).

---

## 8. Message Handlers (`keeper/msg_server.go`, `keeper/asset.go`, `keeper/bond.go`, `keeper/fee.go`)

All handlers are atomic (D9): any failed step aborts the whole message with no state change.

**`RegisterAsset`** → `DRAFT`:
1. `ValidateBasic`: valid `issuer` bech32; `asset_id` matches the slug regex; `name` non-empty and ≤ a sane max; `oracle_pair` matches `^[A-Z0-9]+:[A-Z0-9]+$`; `bond` parses as positive `Int`.
2. Stateful: `asset_id` must not already exist (`ErrAssetExists`); `bond ≥ MinIssuerBond` (`ErrBondTooLow`).
3. Lock bond: `SendCoinsFromAccountToModule(issuer → rwa, bond uvtx)`.
4. Write `AssetRecord{status=DRAFT, denom="rwa/"+asset_id, bond, allow_all=true (D10), created_at=blockTime}` to `0x01`; write `0x02[{issuer}/{asset_id}]`.
5. Emit `rwa_asset_registered{asset_id, issuer, bond}`.

**`AttestAsset`** (`DRAFT → ATTESTED`):
1. `ValidateBasic`: valid `issuer`; `asset_id` non-empty.
2. Load record; require `status == DRAFT` (`ErrInvalidStatus`) and `msg.issuer == record.issuer` (`ErrUnauthorized`).
3. `price, err := oracleKeeper.GetPrice(ctx, record.oracle_pair)`; **any** error (incl. `ErrNoPrice`/`ErrStalePrice`) → `ErrAttestationFailed` (D3).
4. Set `status=ATTESTED`, `attested_price=price`, `attested_at=blockTime`; persist.
5. Emit `rwa_asset_attested{asset_id, oracle_pair, attested_price}`.

**`MintRWA`** (`ATTESTED → ACTIVE`):
1. `ValidateBasic`: valid `issuer`; `notional` parses as positive `Int` (`ErrInvalidNotional`).
2. Load record; require `status == ATTESTED` and `msg.issuer == record.issuer`.
3. `fee = floor(notional × MintFeeRate)` (`keeper/fee.go`). Pay fee: `SendCoinsFromAccountToModule(issuer → FeeCollectorName, fee uvtx)` — fails atomically if the issuer can't pay (D9).
4. Mint `notional` units of `rwa/{id}`: `MintCoins(rwa, denom×notional)` then `SendCoinsFromModuleToAccount(rwa → issuer)`. (The send-restriction permits module-origin mints — §9.)
5. Set `status=ACTIVE`, `notional_minted += notional`; persist.
6. Emit `rwa_minted{asset_id, notional, mint_fee}`.

> A re-mint on an already-`ACTIVE` asset is permitted (status stays `ACTIVE`, `notional_minted` accumulates) — minting more units of a live asset is a normal issuer action; the `ATTESTED → ACTIVE` guard applies only to the first mint. (If the plan prefers a single-mint model, it tightens this to require `status == ATTESTED`; default is accumulate.)

**`TransferRWA`** (no status change; requires `ACTIVE`):
1. `ValidateBasic`: valid `sender`/`recipient`; `amount` positive `Int`.
2. Load record; require `status == ACTIVE` (`ErrInvalidStatus`).
3. `checkTransferAllowed(asset_id, sender, recipient)` (§9 logic) → `ErrTransferRestricted` on violation.
4. `bankKeeper.SendCoins(sender → recipient, denom×amount)` — a direct account-to-account send (which re-invokes the registered send-restriction harmlessly with the same predicate).
5. Emit `rwa_transferred{asset_id, sender, recipient, amount}`.

**`SettleRWA`** (`ACTIVE → SETTLED`, issuer-only — D8):
1. `ValidateBasic`: valid `issuer`.
2. Load record; require `status == ACTIVE` and `msg.issuer == record.issuer`.
3. `settle_fee = floor(notional_minted × SettleFeeRate)`; pay `issuer → FeeCollectorName` (atomic, D9).
4. Burn outstanding supply: the issuer must hold the full `notional_minted` of `rwa/{id}` (clean settle assumes redemption); `SendCoinsFromAccountToModule(issuer → rwa)` then `BurnCoins(rwa)`. If the issuer doesn't hold the full minted supply, settle fails (clean settle requires all units returned).
5. Release bond: `SendCoinsFromModuleToAccount(rwa → issuer, bond)`.
6. Set `status=SETTLED`, `settled_at=blockTime`; persist.
7. Emit `rwa_settled{asset_id, settle_fee, bond_returned}`.

**`UpdateRestrictions`** (issuer-only; asset must not be `SETTLED`):
1. `ValidateBasic`: valid `issuer`; add/del lists valid bech32 and within the per-tx bound.
2. Load record; require `msg.issuer == record.issuer` and `status != SETTLED`.
3. Set `record.allow_all = msg.allow_all`; apply `add_allow`/`del_allow` to `0x03`, `add_deny`/`del_deny` to `0x04` (idempotent set/delete).
4. Emit `rwa_restriction_updated{asset_id, allow_all, allow_count, deny_count}`.

**`SlashBond`** (governance-only — D7):
1. `authority == gov module addr` (`ErrUnauthorized`).
2. Load record; require `status == ACTIVE` (only a live, bonded asset can be disputed).
3. Route bond to community pool: `FundCommunityPool(record.bond uvtx, rwaModuleAddr)`.
4. Force-settle: set `status=SETTLED`, `settled_at=blockTime`, zero the recorded bond; persist. (Outstanding `rwa/{id}` supply is **not** auto-burned — holders keep their tokens; governance handles redemption off-chain. This keeps the slash deterministic and avoids seizing holder balances.)
5. Emit `rwa_bond_slashed{asset_id, amount, reason}`.

**`UpdateParams`** (governance-only): authority check → `params.Validate()` → persist `0x05`; emit `rwa_params_updated`.

---

## 9. Transfer Restrictions & the Bank Send-Restriction (`keeper/restriction.go`, `keeper/send_restriction.go`)

**Shared predicate** `checkTransferAllowed(ctx, assetID, from, to) error`:

```
record = GetAsset(assetID); if missing: error
if from or to is the rwa module account: return nil          # module-origin mint/burn/escrow always allowed
if !record.allow_all:
    require Has(0x03, assetID, from) AND Has(0x03, assetID, to)   # both ends allowlisted
require NOT Has(0x04, assetID, from)                          # deny overrides
require NOT Has(0x04, assetID, to)
return nil
```

**Bank `SendRestrictionFn`** (D5), registered via `app.BankKeeper.AppendSendRestriction(k.SendRestriction)`:

```go
func (k Keeper) SendRestriction(ctx context.Context, from, to sdk.AccAddress, amt sdk.Coins) (sdk.AccAddress, error) {
    for _, c := range amt {
        assetID, ok := parseRWADenom(c.Denom)
        if !ok { continue }                                  // non-rwa denom: no-op (cheap)
        if err := k.checkTransferAllowed(ctx, assetID, from, to); err != nil {
            return to, err
        }
    }
    return to, nil
}
```

Because this runs inside `BankKeeper.SendCoins`, it covers `MsgSend`, `MsgMultiSend`, `authz`-wrapped sends, **and IBC transfer escrow** — the single chokepoint that closes every bypass and satisfies the Phase 5 portability contract (restriction semantics preserved at the source chain). `MsgTransferRWA` calls the same predicate first (for a precise error + status check) and then performs a bank send that re-invokes the restriction harmlessly. Module-origin transfers (mint payout, burn collection, bond escrow) are exempt via the module-account check, so the module's own lifecycle operations are never blocked.

> A `SETTLED` (or gov-slashed) asset's `rwa/{id}` tokens remain transferable under whatever restriction state the record holds at settle time — settlement does not freeze transfers (no `FROZEN` state, D7). Issuers wanting post-settle lockup set a denylist before settling.

---

## 10. Queries, CLI, Events, Genesis

- **gRPC + autocli queries:** `vertixd q rwa asset [asset-id]`, `q rwa assets-by-issuer [issuer]`, `q rwa restrictions [asset-id]`, `q rwa params`.
- **Tx CLI:** `tx rwa register-asset`, `tx rwa attest-asset`, `tx rwa mint-rwa`, `tx rwa transfer-rwa`, `tx rwa settle-rwa`, `tx rwa update-restrictions`. `slash-bond` and `update-params` are governance-only (submitted as gov proposals; skipped from the public tx CLI, mirroring oracle/fees `UpdateParams`).
- **Events (typed):** `rwa_asset_registered{asset_id, issuer, bond}`, `rwa_asset_attested{asset_id, oracle_pair, attested_price}`, `rwa_minted{asset_id, notional, mint_fee}`, `rwa_transferred{asset_id, sender, recipient, amount}`, `rwa_settled{asset_id, settle_fee, bond_returned}`, `rwa_restriction_updated{asset_id, allow_all, allow_count, deny_count}`, `rwa_bond_slashed{asset_id, amount, reason}`, `rwa_params_updated`.
- **Genesis:** `DefaultGenesis` = `DefaultParams` + no assets + no restrictions; `InitGenesis` validates then sets (§5.4); `ExportGenesis` dumps params + assets + flattened restrictions.

---

## 11. Invariants (`x/crisis`) & Acceptance Gate Mapping

**Registered invariants (§3.8):**
1. Every `ACTIVE` asset has a non-zero `bond` recorded.
2. The `x/rwa` module account's `uvtx` balance ≥ Σ `bond` over all non-`SETTLED` assets (the module account holds exactly the live bonds; fees never transit it — §4).
3. For every `rwa/{id}` denom with non-zero bank supply, an `AssetRecord` exists and is `ACTIVE` or `SETTLED`.

| Spec gate ([`full-design-spec.md`](../full-design-spec.md) Phase 3) | How satisfied |
|---|---|
| Full lifecycle executable via CLI | Integration test + CLI: register → attest → mint → transfer → settle end to end |
| Oracle price required + validated at attestation | `AttestAsset` test with a mock `OracleKeeper`: success path attests + snapshots; `ErrNoPrice` and `ErrStalePrice` both yield `ErrAttestationFailed` (D3) |
| Bond locked/released correctly | Bond test: register locks `≥ MinIssuerBond`; clean settle returns it; gov `SlashBond` routes it to the community pool; invariants hold throughout |
| Transfer restrictions enforced | Restriction test: allow_all vs allowlist/denylist; `MsgTransferRWA` rejects violations; **bank `MsgSend` of `rwa/*` is blocked by the send-restriction** (the bypass test, D5); module-origin mint/burn exempt |
| Fees collected and swept by `x/fees` | Fee test: mint/settle send `floor(notional × rate)` uvtx to `FeeCollectorName`; an app-level test asserts the next-EndBlock `x/fees` sweep burns/distributes them |
| Inherited Phase 0 gate (`make lint && make test && make build`, `buf lint/breaking`) | CI stays green with the new module + protos |

---

## 12. Cross-Phase Contracts Established / Honored Here

- **Consumes the Phase 1 `OracleKeeper` interface** (`GetPrice`) and the `BASE:QUOTE` pair convention; treats `ErrNoPrice` and `ErrStalePrice` as attestation failures (honoring the Phase 1 contract that protects Invariant 3).
- **Routes fees to `auth.FeeCollectorName`** per the Phase 2 hand-off contract — never burns/distributes directly; `x/fees` is the sole sweeper.
- **`rwa/{asset-id}` denom contract** — the reserved `rwa/` namespace, one denom per asset, human-readable slug. **Phase 5 must treat `rwa/*` as transferable over ICS-20 while preserving the source-chain restriction semantics** — which the bank `SendRestrictionFn` (D5) enforces on the escrow path, making the contract satisfiable.
- **Governance dispute contract** — `MsgSlashBond` is the single governance entry point that moves an issuer bond off the issuer (to the community pool) and force-settles the asset; future dispute tooling builds on this message, not a new state.
- **Inherited Phase 0 gates** — depinject wiring, `buf`/proto pipeline, CI/lint/test bar.

---

## 13. Risks, Open Items & Doc-Sync

**Doc-sync to `technical-design.md` (land alongside this spec):**

| Section | Change |
|---|---|
| §3.6 | "ante decorator" → **bank `SendRestrictionFn`** (D5); update the bypass-prevention paragraph to describe the `SendCoins` chokepoint and IBC coverage |
| §3.3 | Expand `AssetRecord` and `RWAParams` to the concrete fields in §5.1 (denom, bond, notional_minted, allow_all, attested_price, timestamps) |
| §3.3 | Add `MsgSlashBond` to the `tx.proto` listing; note `UpdateRestrictions` field shape |
| §3.5 | Clarify notional is declared in `uvtx` and minting is 1:1 (D2); the oracle is not read at mint |
| Appendix C ([`full-design-spec.md`](../full-design-spec.md)) | Mark "transfer-restriction model" (D4) and "dispute resolution" (D7) **resolved**; reference this spec |

**Residual risks (carried into the plan / Phase 7):**

| Item | Risk | Mitigation |
|---|---|---|
| Send-restriction recursion / module-origin paths | A mis-scoped exemption could either block legitimate mint/burn or open a hole | Exempt only the `rwa` module account; unit-test mint payout, burn collection, bond escrow, and an IBC-escrow simulation |
| Gov `SlashBond` leaves outstanding `rwa/{id}` with holders | Holders hold tokens of a disputed/settled asset | Documented (D7): governance handles redemption off-chain; tokens are not seized. Re-evaluate a holder-protection mechanism post-mainnet if needed |
| Re-mint accumulation vs single-mint | Ambiguity in whether `MintRWA` can add supply to an `ACTIVE` asset | §8 sets the default (accumulate); the plan must pick and test one model explicitly |
| Slug namespace squatting | First-come `asset_id` slugs are globally unique and permanent | Acceptable at launch; governance/registry curation is a Phase 8+ concern |
| Notional-in-uvtx vs physical units | RWA token amount is VTX-denominated, not e.g. grams | Documented trade-off (D2); physical-unit scaling is a post-launch option, not a launch blocker |
| EndBlock-free module ordering | RWA's `oracle → rwa → fees` slot is positional only (D11) | App-test asserts the ordering; no EndBlock work means no per-block cost |
| Bond invariant under fee atomicity | A fee paid from issuer funds must never come from the bond | Fee uses issuer balance only; atomic failure on shortfall (D9); invariant test |

---

## 14. Definition of Done

- `x/rwa` builds and is depinject-wired: `endBlockers` has `rwa` between `oracle` and `fees`; `genesisModuleOrder` is `staking → oracle → rwa → fees`; `moduleAccPerms` grants the `rwa` account `Minter`+`Burner`; `app.go` registers `AppendSendRestriction(k.SendRestriction)`.
- The full lifecycle works: register (bond `≥ MinIssuerBond` locked) → attest (oracle-gated, snapshotted) → mint (`rwa/{id}` 1:1, fee → collector) → transfer (restriction-checked) → settle (issuer-only, fee → collector, tokens burned, bond returned).
- `MsgSlashBond` (gov) force-settles and routes the bond to the community pool; `MsgUpdateRestrictions` manages allow/deny membership; `MsgUpdateParams` (gov) tunes params.
- The bank `SendRestrictionFn` blocks restricted `rwa/*` `MsgSend`/`MultiSend`/`authz`/IBC-escrow transfers while exempting module-origin operations.
- All eight events emit; the three `x/crisis` invariants are registered and hold under test.
- The four queries (`asset`, `assets-by-issuer`, `restrictions`, `params`) return correctly via gRPC/CLI.
- Genesis round-trips params, assets, and restrictions; factory denoms/bonds are reconciled (not re-minted) at `InitGenesis`.
- Unit + keeper-integration + app-level tests pass (incl. the attestation-failure, restriction-bypass, fee-routing, and bond-slash cases); `make lint && make test && make build`, `buf lint`, and `buf breaking` stay green.
- The `technical-design.md` doc-syncs (§13) are landed; Appendix C items D4/D7 marked resolved.
- Phase 3 acceptance gate (§11) satisfied — opening Phase 6 (full lifecycle on devnet).

---

## Appendix A — Document Map

| Doc | Relationship to this spec |
|---|---|
| [`full-design-spec.md`](../full-design-spec.md) | Program source of truth; Phase 3 section is the parent of this spec; Appendix C D4/D7 resolved here |
| [`technical-design.md`](../technical-design.md) | Owns §3 rwa internals referenced here; §3.3/§3.5/§3.6 receive doc-sync (§13) |
| [`2026-05-29-phase-1-oracle-module-design.md`](./2026-05-29-phase-1-oracle-module-design.md) | Establishes the `OracleKeeper` interface this phase consumes |
| [`2026-05-29-phase-2-fees-module-design.md`](./2026-05-29-phase-2-fees-module-design.md) | Establishes the fee-collector sweep this phase routes into |
| [`project-structure.md`](../project-structure.md) | Target layout for `x/rwa` |
| [`coding-standards.md`](../coding-standards.md) | Lint set, test patterns, proto additivity, genesis round-trip |
| `docs/plans/` | Step-by-step execution plan derived from this spec (next step) |
