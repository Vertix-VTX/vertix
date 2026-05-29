# Phase 3 — `x/rwa` Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a working `x/rwa` module: a protocol-agnostic asset registry enforcing the `DRAFT → ATTESTED → ACTIVE → SETTLED` lifecycle with issuer-bond escrow, oracle-gated attestation, `rwa/{asset-id}` factory-denom mint/burn, per-asset transfer restrictions (enforced through a bank send-restriction that also covers IBC), deterministic mint/settle fee routing into the fee collector, and a governance bond-slash path — satisfying the Phase 3 acceptance gate.

**Architecture:** Ignite scaffolds the module skeleton + depinject wiring; we replace the generated protos with the approved Phase 3 surface, then build in vertical slices (types → params → store → restrictions → bond/fee helpers → message handlers → query → genesis → invariants → app wiring → acceptance). `x/rwa` consumes the Phase 1 `OracleKeeper` (`GetPrice`), routes fees to `auth.FeeCollectorName` (Phase 2 sweeps them), and registers a `bankkeeper.AppendSendRestriction` hook in `app.go`. External deps are interface-typed (`BankKeeper`, `OracleKeeper`, `AccountKeeper`, `DistributionKeeper`) with mocks in `testutil/keeper`.

**Tech Stack:** Go 1.25 (repo toolchain), Cosmos SDK v0.50.x, CometBFT v0.38.x, Ignite CLI v28, buf v1.30+, golangci-lint v1.57+.

**Reference spec:** [`docs/specs/2026-05-29-phase-3-rwa-module-design.md`](../specs/2026-05-29-phase-3-rwa-module-design.md). Where this plan and the spec disagree, the spec wins.

**Conventions:** Run commands from repo root `/home/nam-nguyen/my-projects/vertix-projects/vertix`. Module path is `github.com/vertix-network/vertix`. Commit messages follow Conventional Commits with scope (`docs/coding-standards.md` §7.1). The chain base/bond denom is `uvtx`. RWA factory denoms are `rwa/{asset_id}`.

---

## File Map

| File | Responsibility |
|---|---|
| `proto/vertix/rwa/v1/*.proto` | Wire format: types (AssetRecord, AssetStatus, RWAParams), tx (8 msgs), query, genesis |
| `x/rwa/types/keys.go` | `ModuleName`, `StoreKey`, `BondDenom`, `RWADenomPrefix`, store prefixes `0x01`–`0x05`, key builders |
| `x/rwa/types/errors.go` | Registered sentinel errors |
| `x/rwa/types/events.go` | Event type + attribute constants |
| `x/rwa/types/params.go` | `DefaultParams()`, `Validate()`, dec/int accessors |
| `x/rwa/types/msgs.go` | `ValidateBasic` for all 8 messages; slug regex |
| `x/rwa/types/expected_keepers.go` | `BankKeeper`, `OracleKeeper`, `AccountKeeper`, `DistributionKeeper` |
| `x/rwa/types/genesis.go` | `DefaultGenesis()`, `GenesisState.Validate()` |
| `x/rwa/types/codec.go` | `RegisterInterfaces` for the 8 msgs |
| `x/rwa/keeper/keeper.go` | Keeper struct, params, asset CRUD, issuer index, module addr |
| `x/rwa/keeper/denom.go` | `BuildDenom` / `ParseRWADenom` |
| `x/rwa/keeper/restriction.go` | allow/deny membership store ops + `CheckTransferAllowed` |
| `x/rwa/keeper/send_restriction.go` | `SendRestriction` bank hook |
| `x/rwa/keeper/bond.go` | bond lock / release / slash-to-community-pool |
| `x/rwa/keeper/fee.go` | mint/settle fee math + route to fee collector |
| `x/rwa/keeper/msg_server.go` | all 8 message handlers |
| `x/rwa/keeper/grpc_query.go` | `Asset`, `AssetsByIssuer`, `Restrictions`, `Params` |
| `x/rwa/keeper/genesis.go` | `InitGenesis`, `ExportGenesis` |
| `x/rwa/keeper/invariants.go` | bond + denom `x/crisis` invariants |
| `x/rwa/module/module.go` | AppModule, depinject `ProvideModule` (new deps), no-op EndBlock |
| `x/rwa/module/autocli.go` | CLI query/tx wiring |
| `testutil/keeper/rwa.go` | In-memory keeper fixture + mock bank/oracle/account/distribution |
| `app/app_config.go` | ModuleConfig, endBlockers (between oracle & fees), genesisModuleOrder, maccPerms (Minter+Burner) |
| `app/app.go` | RWAKeeper field + inject + `AppendSendRestriction` |
| `app/app_test.go` | Module wired, Minter+Burner maccPerm |
| `docs/technical-design.md` | §3.3 / §3.5 / §3.6 doc-sync (Task 26) |
| `docs/full-design-spec.md` | Appendix C D4/D7 resolved (Task 26) |

---

## Task 1: Scaffold the rwa module

**Files:**
- Create: `x/rwa/**`, `proto/vertix/rwa/v1/**` (Ignite-generated boilerplate)
- Modify: `app/app.go`, `app/app_config.go` (Ignite inserts at `# stargate/app/...` markers)

- [ ] **Step 1: Scaffold with standard dependencies**

We depend on `bank` and `distribution` (both standard, recognized by Ignite). The `oracle` dependency is wired manually in Task 24 (Ignite `--dep` is unreliable for custom modules; the depinject interface-binding precedent is the oracle module's own `StakingKeeper`/`SlashingKeeper` inputs).

> **Deviation from spec D0 (intentional):** the spec's illustrative command is `--dep bank,oracle,params`. We use `--dep bank,distribution` instead because (a) `distribution` is required for `MsgSlashBond` → `FundCommunityPool` and is missing from the spec's literal command; (b) `oracle` is a custom module wired by interface binding (Task 24), not a reliable `--dep` target; and (c) `params` is unnecessary — like `x/oracle` and `x/fees`, this module stores params in a raw KV key (`0x05`), not an `x/params` subspace. The spec explicitly treats the scaffold command as a starting point ("then adapt"); the binding contract is the module set, ordering, and cross-phase contracts, all of which this plan satisfies.

Run:

```bash
cd /home/nam-nguyen/my-projects/vertix-projects/vertix
ignite scaffold module rwa --dep bank,distribution -y
```

Expected: creates `x/rwa/`, `proto/vertix/rwa/v1/`, and updates `app/app.go` + `app/app_config.go`.

- [ ] **Step 2: Verify scaffold compiles**

Run:

```bash
go build ./...
```

Expected: success (scaffold boilerplate builds).

- [ ] **Step 3: Note scaffold files to replace later**

These Ignite stubs are overwritten in later tasks — do not treat them as final:

- `x/rwa/types/params.go` → Task 6
- `x/rwa/types/genesis.go` → Task 22
- `x/rwa/types/expected_keepers.go` → Task 8
- `x/rwa/types/codec.go` → Task 2 (Step 6)
- `x/rwa/keeper/keeper.go` → Task 10
- `x/rwa/keeper/msg_server.go`, `grpc_query.go` → Tasks 14–21, 21
- `x/rwa/module/module.go`, `autocli.go` → Tasks 24

- [ ] **Step 4: Commit**

```bash
git add x/rwa/ proto/vertix/rwa/ app/app.go app/app_config.go go.mod go.sum
git commit -m "$(cat <<'EOF'
feat(rwa): scaffold x/rwa module via ignite

Depinject wiring for bank, distribution. Refs:
docs/specs/2026-05-29-phase-3-rwa-module-design.md
EOF
)"
```

---

## Task 2: Replace protobuf definitions and regenerate

**Files:**
- Modify: `proto/vertix/rwa/v1/types.proto`, `tx.proto`, `query.proto`, `genesis.proto`
- Replace: `x/rwa/types/codec.go`
- Create: generated `x/rwa/types/*.pb.go` via `make proto-gen`

- [ ] **Step 1: Write `proto/vertix/rwa/v1/types.proto`**

Replace file contents with:

```protobuf
syntax = "proto3";
package vertix.rwa.v1;

import "cosmos_proto/cosmos.proto";
import "gogoproto/gogo.proto";
import "google/protobuf/timestamp.proto";

option go_package = "github.com/vertix-network/vertix/x/rwa/types";

// AssetStatus is the lifecycle state of an RWA asset.
enum AssetStatus {
  ASSET_STATUS_UNSPECIFIED = 0;
  ASSET_STATUS_DRAFT       = 1;
  ASSET_STATUS_ATTESTED    = 2;
  ASSET_STATUS_ACTIVE      = 3;
  ASSET_STATUS_SETTLED     = 4;
}

// AssetRecord is the registry entry for one RWA asset.
message AssetRecord {
  string      asset_id        = 1;
  string      issuer          = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string      name            = 3;
  string      description     = 4;
  AssetStatus status          = 5;
  string      oracle_pair     = 6;
  string      denom           = 7;
  string      bond            = 8  [(cosmos_proto.scalar) = "cosmos.Int"];
  string      notional_minted = 9  [(cosmos_proto.scalar) = "cosmos.Int"];
  bool        allow_all       = 10;
  string      attested_price  = 11 [(cosmos_proto.scalar) = "cosmos.Dec"];
  google.protobuf.Timestamp attested_at = 12 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
  google.protobuf.Timestamp created_at  = 13 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
  google.protobuf.Timestamp settled_at  = 14 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
}

// RWAParams defines module governance parameters.
message RWAParams {
  string min_issuer_bond = 1 [(cosmos_proto.scalar) = "cosmos.Int"];
  string mint_fee_rate   = 2 [(cosmos_proto.scalar) = "cosmos.Dec"];
  string settle_fee_rate = 3 [(cosmos_proto.scalar) = "cosmos.Dec"];
}
```

- [ ] **Step 2: Write `proto/vertix/rwa/v1/tx.proto`**

```protobuf
syntax = "proto3";
package vertix.rwa.v1;

import "amino/amino.proto";
import "cosmos/msg/v1/msg.proto";
import "cosmos_proto/cosmos.proto";
import "gogoproto/gogo.proto";
import "vertix/rwa/v1/types.proto";

option go_package = "github.com/vertix-network/vertix/x/rwa/types";

service Msg {
  option (cosmos.msg.v1.service) = true;
  rpc RegisterAsset(MsgRegisterAsset)           returns (MsgRegisterAssetResponse);
  rpc AttestAsset(MsgAttestAsset)               returns (MsgAttestAssetResponse);
  rpc MintRWA(MsgMintRWA)                       returns (MsgMintRWAResponse);
  rpc TransferRWA(MsgTransferRWA)               returns (MsgTransferRWAResponse);
  rpc SettleRWA(MsgSettleRWA)                   returns (MsgSettleRWAResponse);
  rpc UpdateRestrictions(MsgUpdateRestrictions) returns (MsgUpdateRestrictionsResponse);
  rpc SlashBond(MsgSlashBond)                   returns (MsgSlashBondResponse);
  rpc UpdateParams(MsgUpdateParams)             returns (MsgUpdateParamsResponse);
}

message MsgRegisterAsset {
  option (cosmos.msg.v1.signer) = "issuer";
  option (amino.name)           = "vertix/x/rwa/MsgRegisterAsset";
  string issuer      = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string asset_id    = 2;
  string name        = 3;
  string description = 4;
  string oracle_pair = 5;
  string bond        = 6 [(cosmos_proto.scalar) = "cosmos.Int"];
}
message MsgRegisterAssetResponse {}

message MsgAttestAsset {
  option (cosmos.msg.v1.signer) = "issuer";
  option (amino.name)           = "vertix/x/rwa/MsgAttestAsset";
  string issuer   = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string asset_id = 2;
}
message MsgAttestAssetResponse {}

message MsgMintRWA {
  option (cosmos.msg.v1.signer) = "issuer";
  option (amino.name)           = "vertix/x/rwa/MsgMintRWA";
  string issuer   = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string asset_id = 2;
  string notional = 3 [(cosmos_proto.scalar) = "cosmos.Int"];
}
message MsgMintRWAResponse {}

message MsgTransferRWA {
  option (cosmos.msg.v1.signer) = "sender";
  option (amino.name)           = "vertix/x/rwa/MsgTransferRWA";
  string sender    = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string recipient = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string asset_id  = 3;
  string amount    = 4 [(cosmos_proto.scalar) = "cosmos.Int"];
}
message MsgTransferRWAResponse {}

message MsgSettleRWA {
  option (cosmos.msg.v1.signer) = "issuer";
  option (amino.name)           = "vertix/x/rwa/MsgSettleRWA";
  string issuer   = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string asset_id = 2;
}
message MsgSettleRWAResponse {}

message MsgUpdateRestrictions {
  option (cosmos.msg.v1.signer) = "issuer";
  option (amino.name)           = "vertix/x/rwa/MsgUpdateRestrictions";
  string issuer             = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string asset_id           = 2;
  bool   allow_all          = 3;
  repeated string add_allow = 4 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  repeated string del_allow = 5 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  repeated string add_deny  = 6 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  repeated string del_deny  = 7 [(cosmos_proto.scalar) = "cosmos.AddressString"];
}
message MsgUpdateRestrictionsResponse {}

message MsgSlashBond {
  option (cosmos.msg.v1.signer) = "authority";
  option (amino.name)           = "vertix/x/rwa/MsgSlashBond";
  string authority = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string asset_id  = 2;
  string reason    = 3;
}
message MsgSlashBondResponse {}

message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";
  option (amino.name)           = "vertix/x/rwa/MsgUpdateParams";
  string    authority = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  RWAParams params    = 2 [(gogoproto.nullable) = false];
}
message MsgUpdateParamsResponse {}
```

- [ ] **Step 3: Write `proto/vertix/rwa/v1/query.proto`**

```protobuf
syntax = "proto3";
package vertix.rwa.v1;

import "cosmos/base/query/v1beta1/pagination.proto";
import "gogoproto/gogo.proto";
import "google/api/annotations.proto";
import "vertix/rwa/v1/types.proto";

option go_package = "github.com/vertix-network/vertix/x/rwa/types";

service Query {
  rpc Asset(QueryAssetRequest) returns (QueryAssetResponse) {
    option (google.api.http).get = "/vertix/rwa/v1/asset/{asset_id}";
  }
  rpc AssetsByIssuer(QueryAssetsByIssuerRequest) returns (QueryAssetsByIssuerResponse) {
    option (google.api.http).get = "/vertix/rwa/v1/assets_by_issuer/{issuer}";
  }
  rpc Restrictions(QueryRestrictionsRequest) returns (QueryRestrictionsResponse) {
    option (google.api.http).get = "/vertix/rwa/v1/restrictions/{asset_id}";
  }
  rpc Params(QueryParamsRequest) returns (QueryParamsResponse) {
    option (google.api.http).get = "/vertix/rwa/v1/params";
  }
}

message QueryAssetRequest { string asset_id = 1; }
message QueryAssetResponse { AssetRecord asset = 1 [(gogoproto.nullable) = false]; }

message QueryAssetsByIssuerRequest {
  string issuer = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  cosmos.base.query.v1beta1.PageRequest pagination = 2;
}
message QueryAssetsByIssuerResponse {
  repeated AssetRecord assets = 1 [(gogoproto.nullable) = false];
  cosmos.base.query.v1beta1.PageResponse pagination = 2;
}

message QueryRestrictionsRequest { string asset_id = 1; }
message QueryRestrictionsResponse {
  bool            allow_all = 1;
  repeated string allowlist = 2;
  repeated string denylist  = 3;
}

message QueryParamsRequest {}
message QueryParamsResponse { RWAParams params = 1 [(gogoproto.nullable) = false]; }
```

Note: `cosmos_proto/cosmos.proto` is imported transitively via the scalar option; add `import "cosmos_proto/cosmos.proto";` to the import block if `buf lint` flags the `cosmos.AddressString` scalar as undefined.

- [ ] **Step 4: Write `proto/vertix/rwa/v1/genesis.proto`**

```protobuf
syntax = "proto3";
package vertix.rwa.v1;

import "cosmos_proto/cosmos.proto";
import "gogoproto/gogo.proto";
import "vertix/rwa/v1/types.proto";

option go_package = "github.com/vertix-network/vertix/x/rwa/types";

// Restriction is one flattened allow/deny membership entry for genesis.
message Restriction {
  string asset_id = 1;
  string address  = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  bool   is_deny  = 3;
}

message GenesisState {
  RWAParams            params       = 1 [(gogoproto.nullable) = false];
  repeated AssetRecord assets       = 2 [(gogoproto.nullable) = false];
  repeated Restriction restrictions = 3 [(gogoproto.nullable) = false];
}
```

- [ ] **Step 5: Regenerate Go from proto**

Run:

```bash
make proto-gen
```

Expected: regenerates `x/rwa/types/*.pb.go`, `x/rwa/types/query.pb.gw.go`.

- [ ] **Step 6: Replace `x/rwa/types/codec.go`**

The manual proto replacement means the scaffold's `RegisterInterfaces` is stale. Write:

```go
package types

import (
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgRegisterAsset{},
		&MsgAttestAsset{},
		&MsgMintRWA{},
		&MsgTransferRWA{},
		&MsgSettleRWA{},
		&MsgUpdateRestrictions{},
		&MsgSlashBond{},
		&MsgUpdateParams{},
	)
	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}
```

Delete any scaffold `# starport scaffolding` marker lines left in the file.

- [ ] **Step 7: Run buf lint + build types**

Run:

```bash
cd proto && buf lint && cd .. && go build ./x/rwa/types/...
```

Expected: no buf errors; `go build` may fail on other rwa packages (keeper) until later tasks — that's fine, just ensure `x/rwa/types` builds.

- [ ] **Step 8: Commit**

```bash
git add proto/vertix/rwa/ x/rwa/types/
git commit -m "$(cat <<'EOF'
feat(rwa): define Phase 3 protobuf surface

AssetRecord/AssetStatus/RWAParams, 8 messages, 4 queries, genesis.
Refs: docs/specs/2026-05-29-phase-3-rwa-module-design.md §5
EOF
)"
```

---

## Task 3: Store keys, denom constants, and key builders

**Files:**
- Replace: `x/rwa/types/keys.go` (overwrite scaffold)
- Create: `x/rwa/types/keys_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/rwa/types/keys_test.go`:

```go
package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestModuleConstants(t *testing.T) {
	require.Equal(t, "rwa", types.ModuleName)
	require.Equal(t, "rwa", types.StoreKey)
	require.Equal(t, "uvtx", types.BondDenom)
	require.Equal(t, "rwa/", types.RWADenomPrefix)
	require.Equal(t, []byte{0x01}, types.KeyPrefixAsset)
	require.Equal(t, []byte{0x05}, types.KeyParams)
}

func TestAssetKeyRoundTrip(t *testing.T) {
	key := types.AssetKey("gold-01")
	require.Equal(t, byte(0x01), key[0])
	require.Equal(t, "gold-01", string(key[1:]))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./x/rwa/types/ -run 'TestModuleConstants|TestAssetKey' -v`

Expected: FAIL — constants / `AssetKey` not defined (scaffold differs).

- [ ] **Step 3: Write `x/rwa/types/keys.go`**

```go
package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/address"
)

const (
	// ModuleName defines the module name.
	ModuleName = "rwa"

	// StoreKey defines the primary module store key.
	StoreKey = ModuleName

	// BondDenom is the denom issuer bonds are escrowed in (chain base denom).
	BondDenom = "uvtx"

	// RWADenomPrefix namespaces every factory denom: rwa/{asset_id}.
	RWADenomPrefix = "rwa/"
)

var (
	KeyPrefixAsset       = []byte{0x01} // {asset_id} -> AssetRecord
	KeyPrefixIssuerIndex = []byte{0x02} // {issuer}/{asset_id} -> 0x01 (presence)
	KeyPrefixAllowlist   = []byte{0x03} // {asset_id}/{addr} -> presence
	KeyPrefixDenylist    = []byte{0x04} // {asset_id}/{addr} -> presence
	KeyParams            = []byte{0x05} // RWAParams
)

// AssetKey returns the 0x01 store key for an asset record.
func AssetKey(assetID string) []byte {
	return append(append([]byte{}, KeyPrefixAsset...), []byte(assetID)...)
}

// IssuerIndexKey returns the 0x02 issuer->asset index key (issuer length-prefixed).
func IssuerIndexKey(issuer sdk.AccAddress, assetID string) []byte {
	key := append(append([]byte{}, KeyPrefixIssuerIndex...), address.MustLengthPrefix(issuer)...)
	return append(key, []byte("/"+assetID)...)
}

// IssuerIndexPrefix returns the 0x02 scan prefix for one issuer.
func IssuerIndexPrefix(issuer sdk.AccAddress) []byte {
	return append(append([]byte{}, KeyPrefixIssuerIndex...), address.MustLengthPrefix(issuer)...)
}

// AllowlistKey / DenylistKey build membership keys; asset_id is length-prefixed
// so the address suffix can be parsed back unambiguously.
func AllowlistKey(assetID string, addr sdk.AccAddress) []byte {
	return membershipKey(KeyPrefixAllowlist, assetID, addr)
}

func DenylistKey(assetID string, addr sdk.AccAddress) []byte {
	return membershipKey(KeyPrefixDenylist, assetID, addr)
}

// AllowlistPrefix / DenylistPrefix scan all members of one asset.
func AllowlistPrefix(assetID string) []byte {
	return membershipPrefix(KeyPrefixAllowlist, assetID)
}

func DenylistPrefix(assetID string) []byte {
	return membershipPrefix(KeyPrefixDenylist, assetID)
}

func membershipKey(prefix []byte, assetID string, addr sdk.AccAddress) []byte {
	return append(membershipPrefix(prefix, assetID), addr.Bytes()...)
}

// membershipPrefix lays out prefix(1) | lenByte(1) | assetID. The asset id is
// single-byte length-prefixed so the trailing address can be parsed back; a
// slug is ≤ 43 chars, which fits in one byte.
func membershipPrefix(prefix []byte, assetID string) []byte {
	idBytes := []byte(assetID)
	key := append(append([]byte{}, prefix...), byte(len(idBytes)))
	return append(key, idBytes...)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./x/rwa/types/ -run 'TestModuleConstants|TestAssetKey' -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/rwa/types/keys.go x/rwa/types/keys_test.go
git commit -m "feat(rwa): add module constants, denom prefix, and store key builders"
```

---

## Task 4: Registered errors

**Files:**
- Create: `x/rwa/types/errors.go`

- [ ] **Step 1: Write `x/rwa/types/errors.go`**

```go
package types

import "cosmossdk.io/errors"

var (
	ErrInvalidSigner      = errors.Register(ModuleName, 2, "invalid signer")
	ErrUnauthorized       = errors.Register(ModuleName, 3, "unauthorized")
	ErrAssetExists        = errors.Register(ModuleName, 4, "asset_id already exists")
	ErrAssetNotFound      = errors.Register(ModuleName, 5, "asset not found")
	ErrInvalidStatus      = errors.Register(ModuleName, 6, "invalid asset status for this transition")
	ErrBondTooLow         = errors.Register(ModuleName, 7, "bond is below min_issuer_bond")
	ErrAttestationFailed  = errors.Register(ModuleName, 8, "oracle attestation failed (no or stale price)")
	ErrInvalidNotional    = errors.Register(ModuleName, 9, "notional must be a positive integer")
	ErrTransferRestricted = errors.Register(ModuleName, 10, "transfer not allowed by asset restrictions")
	ErrInvalidAssetID     = errors.Register(ModuleName, 11, "invalid asset_id (must match slug format)")
	ErrInvalidParams      = errors.Register(ModuleName, 12, "invalid params")
	ErrInvalidAmount      = errors.Register(ModuleName, 13, "invalid amount")
)
```

- [ ] **Step 2: Verify build**

Run: `go build ./x/rwa/types/...`

Expected: success

- [ ] **Step 3: Commit**

```bash
git add x/rwa/types/errors.go
git commit -m "feat(rwa): register module sentinel errors"
```

---

## Task 5: Events

**Files:**
- Create: `x/rwa/types/events.go`

- [ ] **Step 1: Write `x/rwa/types/events.go`**

```go
package types

const (
	EventTypeAssetRegistered    = "rwa_asset_registered"
	EventTypeAssetAttested      = "rwa_asset_attested"
	EventTypeRWAMinted          = "rwa_minted"
	EventTypeRWATransferred     = "rwa_transferred"
	EventTypeRWASettled         = "rwa_settled"
	EventTypeRestrictionUpdated = "rwa_restriction_updated"
	EventTypeBondSlashed        = "rwa_bond_slashed"
	EventTypeParamsUpdated      = "rwa_params_updated"

	AttributeKeyAssetID       = "asset_id"
	AttributeKeyIssuer        = "issuer"
	AttributeKeyBond          = "bond"
	AttributeKeyOraclePair    = "oracle_pair"
	AttributeKeyAttestedPrice = "attested_price"
	AttributeKeyNotional      = "notional"
	AttributeKeyMintFee       = "mint_fee"
	AttributeKeySettleFee     = "settle_fee"
	AttributeKeyBondReturned  = "bond_returned"
	AttributeKeySender        = "sender"
	AttributeKeyRecipient     = "recipient"
	AttributeKeyAmount        = "amount"
	AttributeKeyAllowAll      = "allow_all"
	AttributeKeyAllowCount    = "allow_count"
	AttributeKeyDenyCount     = "deny_count"
	AttributeKeyReason        = "reason"
)
```

- [ ] **Step 2: Commit**

```bash
git add x/rwa/types/events.go
git commit -m "feat(rwa): define typed event constants"
```

---

## Task 6: Params defaults and validation

**Files:**
- Replace: `x/rwa/types/params.go` (delete Ignite scaffold `Params`/`DefaultParams` first)
- Create: `x/rwa/types/params_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/rwa/types/params_test.go`:

```go
package types_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestDefaultParamsValid(t *testing.T) {
	p := types.DefaultParams()
	require.NoError(t, p.Validate())
	require.Equal(t, "10000000000", p.MinIssuerBond)
	require.Equal(t, "0.001000000000000000", p.MintFeeRate)
	require.Equal(t, "0.001000000000000000", p.SettleFeeRate)
}

func TestParamsAccessors(t *testing.T) {
	p := types.DefaultParams()
	bond, ok := p.MinIssuerBondInt()
	require.True(t, ok)
	require.Equal(t, math.NewInt(10000000000), bond)

	rate, err := p.MintFeeRateDec()
	require.NoError(t, err)
	require.True(t, rate.Equal(math.LegacyNewDecWithPrec(1, 3)))
}

func TestValidateRejectsBadFeeRate(t *testing.T) {
	require.Error(t, types.RWAParams{MinIssuerBond: "1", MintFeeRate: "1.5", SettleFeeRate: "0.001"}.Validate())
	require.Error(t, types.RWAParams{MinIssuerBond: "1", MintFeeRate: "abc", SettleFeeRate: "0.001"}.Validate())
}

func TestValidateRejectsNegativeBond(t *testing.T) {
	require.Error(t, types.RWAParams{MinIssuerBond: "-1", MintFeeRate: "0.001", SettleFeeRate: "0.001"}.Validate())
	require.Error(t, types.RWAParams{MinIssuerBond: "notanint", MintFeeRate: "0.001", SettleFeeRate: "0.001"}.Validate())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./x/rwa/types/ -run 'TestDefaultParams|TestParamsAccessors|TestValidate' -v`

Expected: FAIL — `DefaultParams` signature differs / accessors not defined.

- [ ] **Step 3: Write `x/rwa/types/params.go`**

```go
package types

import "cosmossdk.io/math"

// DefaultParams returns launch defaults: 10,000 VTX min bond, 0.10% mint+settle fees.
func DefaultParams() RWAParams {
	return RWAParams{
		MinIssuerBond: math.NewInt(10_000_000_000).String(),   // 10,000 VTX in uvtx
		MintFeeRate:   math.LegacyNewDecWithPrec(1, 3).String(), // 0.001
		SettleFeeRate: math.LegacyNewDecWithPrec(1, 3).String(), // 0.001
	}
}

// MinIssuerBondInt parses min_issuer_bond into an Int.
func (p RWAParams) MinIssuerBondInt() (math.Int, bool) {
	return math.NewIntFromString(p.MinIssuerBond)
}

// MintFeeRateDec parses mint_fee_rate into a LegacyDec.
func (p RWAParams) MintFeeRateDec() (math.LegacyDec, error) {
	return math.LegacyNewDecFromStr(p.MintFeeRate)
}

// SettleFeeRateDec parses settle_fee_rate into a LegacyDec.
func (p RWAParams) SettleFeeRateDec() (math.LegacyDec, error) {
	return math.LegacyNewDecFromStr(p.SettleFeeRate)
}

// Validate enforces: bond is a non-negative Int; both fee rates are in [0,1).
func (p RWAParams) Validate() error {
	bond, ok := math.NewIntFromString(p.MinIssuerBond)
	if !ok {
		return ErrInvalidParams.Wrapf("min_issuer_bond: not an integer: %q", p.MinIssuerBond)
	}
	if bond.IsNegative() {
		return ErrInvalidParams.Wrap("min_issuer_bond must be >= 0")
	}
	if err := validateFeeRate(p.MintFeeRate, "mint_fee_rate"); err != nil {
		return err
	}
	return validateFeeRate(p.SettleFeeRate, "settle_fee_rate")
}

func validateFeeRate(s, name string) error {
	d, err := math.LegacyNewDecFromStr(s)
	if err != nil {
		return ErrInvalidParams.Wrapf("%s: %v", name, err)
	}
	if d.IsNegative() || d.GTE(math.LegacyOneDec()) {
		return ErrInvalidParams.Wrapf("%s must be in [0, 1)", name)
	}
	return nil
}
```

Do **not** add a custom `RWAParams.String()` — gogoproto generates one; a second collides.

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./x/rwa/types/ -run 'TestDefaultParams|TestParamsAccessors|TestValidate' -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/rwa/types/params.go x/rwa/types/params_test.go
git commit -m "feat(rwa): add DefaultParams and Validate"
```

---

## Task 7: Message ValidateBasic + slug regex

**Files:**
- Create: `x/rwa/types/msgs.go`
- Create: `x/rwa/types/msgs_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/rwa/types/msgs_test.go`:

```go
package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/sample"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestMsgRegisterAssetValidateBasic(t *testing.T) {
	issuer := sample.AccAddress()
	good := &types.MsgRegisterAsset{Issuer: issuer, AssetId: "gold-vault-01", Name: "Gold", OraclePair: "XAU:USD", Bond: "10000000000"}
	require.NoError(t, good.ValidateBasic())

	require.Error(t, (&types.MsgRegisterAsset{Issuer: "bad", AssetId: "gold", Name: "x", OraclePair: "XAU:USD", Bond: "1"}).ValidateBasic())
	require.Error(t, (&types.MsgRegisterAsset{Issuer: issuer, AssetId: "AB", Name: "x", OraclePair: "XAU:USD", Bond: "1"}).ValidateBasic())        // too short / uppercase
	require.Error(t, (&types.MsgRegisterAsset{Issuer: issuer, AssetId: "gold", Name: "", OraclePair: "XAU:USD", Bond: "1"}).ValidateBasic())       // empty name
	require.Error(t, (&types.MsgRegisterAsset{Issuer: issuer, AssetId: "gold", Name: "x", OraclePair: "bad pair", Bond: "1"}).ValidateBasic())     // bad pair
	require.Error(t, (&types.MsgRegisterAsset{Issuer: issuer, AssetId: "gold", Name: "x", OraclePair: "XAU:USD", Bond: "0"}).ValidateBasic())      // non-positive bond
}

func TestMsgMintRWAValidateBasic(t *testing.T) {
	issuer := sample.AccAddress()
	require.NoError(t, (&types.MsgMintRWA{Issuer: issuer, AssetId: "gold", Notional: "1000"}).ValidateBasic())
	require.Error(t, (&types.MsgMintRWA{Issuer: issuer, AssetId: "gold", Notional: "0"}).ValidateBasic())
	require.Error(t, (&types.MsgMintRWA{Issuer: issuer, AssetId: "gold", Notional: "-5"}).ValidateBasic())
}

func TestMsgTransferRWAValidateBasic(t *testing.T) {
	require.NoError(t, (&types.MsgTransferRWA{Sender: sample.AccAddress(), Recipient: sample.AccAddress(), AssetId: "gold", Amount: "10"}).ValidateBasic())
	require.Error(t, (&types.MsgTransferRWA{Sender: sample.AccAddress(), Recipient: "bad", AssetId: "gold", Amount: "10"}).ValidateBasic())
}

func TestMsgSlashBondValidateBasic(t *testing.T) {
	require.NoError(t, (&types.MsgSlashBond{Authority: sample.AccAddress(), AssetId: "gold"}).ValidateBasic())
	require.Error(t, (&types.MsgSlashBond{Authority: "bad", AssetId: "gold"}).ValidateBasic())
}

func TestValidateAssetID(t *testing.T) {
	require.NoError(t, types.ValidateAssetID("gold-vault-01"))
	require.Error(t, types.ValidateAssetID("Gold"))   // uppercase
	require.Error(t, types.ValidateAssetID("go"))      // too short
	require.Error(t, types.ValidateAssetID("a/b"))     // illegal char
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/rwa/types/ -run 'TestMsg|TestValidateAssetID' -v`

Expected: FAIL — `ValidateBasic` / `ValidateAssetID` not defined.

- [ ] **Step 3: Write `x/rwa/types/msgs.go`**

```go
package types

import (
	"regexp"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// slugRegex is a strict subset of the bank denom charset so that
// "rwa/"+asset_id is always a valid factory denom.
var (
	slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,42}$`)
	pairRegex = regexp.MustCompile(`^[A-Z0-9]+:[A-Z0-9]+$`)

	// maxRestrictionBatch bounds add/del entries per UpdateRestrictions tx (gas hygiene).
	maxRestrictionBatch = 100
)

// ValidateAssetID checks the slug format.
func ValidateAssetID(id string) error {
	if !slugRegex.MatchString(id) {
		return ErrInvalidAssetID.Wrapf("%q", id)
	}
	return nil
}

func validatePositiveInt(s, name string) error {
	v, ok := math.NewIntFromString(s)
	if !ok {
		return ErrInvalidAmount.Wrapf("%s: not an integer: %q", name, s)
	}
	if !v.IsPositive() {
		return ErrInvalidAmount.Wrapf("%s must be positive", name)
	}
	return nil
}

func (m *MsgRegisterAsset) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Issuer); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("issuer: %v", err)
	}
	if err := ValidateAssetID(m.AssetId); err != nil {
		return err
	}
	if m.Name == "" {
		return ErrInvalidParams.Wrap("name must not be empty")
	}
	if !pairRegex.MatchString(m.OraclePair) {
		return ErrInvalidParams.Wrapf("oracle_pair %q is not BASE:QUOTE", m.OraclePair)
	}
	return validatePositiveInt(m.Bond, "bond")
}

func (m *MsgAttestAsset) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Issuer); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("issuer: %v", err)
	}
	return ValidateAssetID(m.AssetId)
}

func (m *MsgMintRWA) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Issuer); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("issuer: %v", err)
	}
	if err := ValidateAssetID(m.AssetId); err != nil {
		return err
	}
	if err := validatePositiveInt(m.Notional, "notional"); err != nil {
		return ErrInvalidNotional.Wrap(err.Error())
	}
	return nil
}

func (m *MsgTransferRWA) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Sender); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("sender: %v", err)
	}
	if _, err := sdk.AccAddressFromBech32(m.Recipient); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("recipient: %v", err)
	}
	if err := ValidateAssetID(m.AssetId); err != nil {
		return err
	}
	return validatePositiveInt(m.Amount, "amount")
}

func (m *MsgSettleRWA) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Issuer); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("issuer: %v", err)
	}
	return ValidateAssetID(m.AssetId)
}

func (m *MsgUpdateRestrictions) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Issuer); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("issuer: %v", err)
	}
	if err := ValidateAssetID(m.AssetId); err != nil {
		return err
	}
	for _, group := range [][]string{m.AddAllow, m.DelAllow, m.AddDeny, m.DelDeny} {
		if len(group) > maxRestrictionBatch {
			return ErrInvalidParams.Wrapf("restriction batch exceeds %d entries", maxRestrictionBatch)
		}
		for _, a := range group {
			if _, err := sdk.AccAddressFromBech32(a); err != nil {
				return sdkerrors.ErrInvalidAddress.Wrapf("restriction entry %q: %v", a, err)
			}
		}
	}
	return nil
}

func (m *MsgSlashBond) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("authority: %v", err)
	}
	return ValidateAssetID(m.AssetId)
}

func (m *MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("authority: %v", err)
	}
	return m.Params.Validate()
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./x/rwa/types/ -run 'TestMsg|TestValidateAssetID' -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/rwa/types/msgs.go x/rwa/types/msgs_test.go
git commit -m "feat(rwa): add ValidateBasic for all messages + slug validation"
```

---

## Task 8: Expected keeper interfaces

**Files:**
- Replace: `x/rwa/types/expected_keepers.go`

- [ ] **Step 1: Write `x/rwa/types/expected_keepers.go`**

```go
package types

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BankKeeper is the bank surface x/rwa needs: escrow bonds, mint/burn factory
// denoms, route fees, and restriction-checked transfers.
type BankKeeper interface {
	MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, sender sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipient sdk.AccAddress, amt sdk.Coins) error
	SendCoins(ctx context.Context, from, to sdk.AccAddress, amt sdk.Coins) error
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	GetSupply(ctx context.Context, denom string) sdk.Coin
}

// OracleKeeper is the Phase 1 price consumer interface (attestation gate).
// Both ErrNoPrice and ErrStalePrice are returned as errors.
type OracleKeeper interface {
	GetPrice(ctx context.Context, pair string) (math.LegacyDec, error)
}

// AccountKeeper resolves module account addresses.
type AccountKeeper interface {
	GetModuleAddress(name string) sdk.AccAddress
}

// DistributionKeeper receives slashed bonds into the community pool.
type DistributionKeeper interface {
	FundCommunityPool(ctx context.Context, amount sdk.Coins, sender sdk.AccAddress) error
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./x/rwa/types/...`

Expected: success

- [ ] **Step 3: Commit**

```bash
git add x/rwa/types/expected_keepers.go
git commit -m "feat(rwa): define Bank/Oracle/Account/Distribution interfaces"
```

---

## Task 9: Test fixture with mock keepers

**Files:**
- Create: `testutil/keeper/rwa.go`

- [ ] **Step 1: Write `testutil/keeper/rwa.go`**

```go
package keeper

import (
	"context"
	"errors"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cosmosdb "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/require"

	rwakeeper "github.com/vertix-network/vertix/x/rwa/keeper"
	"github.com/vertix-network/vertix/x/rwa/types"
)

// MockAccount resolves module names to canonical module addresses.
type MockAccount struct{}

func (MockAccount) GetModuleAddress(name string) sdk.AccAddress {
	return authtypes.NewModuleAddress(name)
}

// MockBank tracks balances by address, total supply by denom, and burned coins.
type MockBank struct {
	balances map[string]sdk.Coins
	supply   sdk.Coins
}

func NewMockBank() *MockBank {
	return &MockBank{balances: map[string]sdk.Coins{}, supply: sdk.NewCoins()}
}

func (m *MockBank) SetBalance(addr sdk.AccAddress, coins sdk.Coins) { m.balances[addr.String()] = coins }
func (m *MockBank) Balance(addr sdk.AccAddress) sdk.Coins           { return m.balances[addr.String()] }
func (m *MockBank) ModuleAddr(name string) sdk.AccAddress           { return authtypes.NewModuleAddress(name) }

func (m *MockBank) GetBalance(_ context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	return sdk.NewCoin(denom, m.balances[addr.String()].AmountOf(denom))
}

func (m *MockBank) GetSupply(_ context.Context, denom string) sdk.Coin {
	return sdk.NewCoin(denom, m.supply.AmountOf(denom))
}

func (m *MockBank) MintCoins(_ context.Context, module string, amt sdk.Coins) error {
	addr := authtypes.NewModuleAddress(module).String()
	m.balances[addr] = m.balances[addr].Add(amt...)
	m.supply = m.supply.Add(amt...)
	return nil
}

func (m *MockBank) BurnCoins(_ context.Context, module string, amt sdk.Coins) error {
	addr := authtypes.NewModuleAddress(module).String()
	if !m.balances[addr].IsAllGTE(amt) {
		return errors.New("insufficient module balance to burn")
	}
	m.balances[addr] = m.balances[addr].Sub(amt...)
	m.supply = m.supply.Sub(amt...)
	return nil
}

func (m *MockBank) SendCoinsFromAccountToModule(_ context.Context, sender sdk.AccAddress, module string, amt sdk.Coins) error {
	if !m.balances[sender.String()].IsAllGTE(amt) {
		return errors.New("insufficient funds")
	}
	to := authtypes.NewModuleAddress(module).String()
	m.balances[sender.String()] = m.balances[sender.String()].Sub(amt...)
	m.balances[to] = m.balances[to].Add(amt...)
	return nil
}

func (m *MockBank) SendCoinsFromModuleToAccount(_ context.Context, module string, recipient sdk.AccAddress, amt sdk.Coins) error {
	from := authtypes.NewModuleAddress(module).String()
	if !m.balances[from].IsAllGTE(amt) {
		return errors.New("insufficient module funds")
	}
	m.balances[from] = m.balances[from].Sub(amt...)
	m.balances[recipient.String()] = m.balances[recipient.String()].Add(amt...)
	return nil
}

func (m *MockBank) SendCoins(_ context.Context, from, to sdk.AccAddress, amt sdk.Coins) error {
	if !m.balances[from.String()].IsAllGTE(amt) {
		return errors.New("insufficient funds")
	}
	m.balances[from.String()] = m.balances[from.String()].Sub(amt...)
	m.balances[to.String()] = m.balances[to.String()].Add(amt...)
	return nil
}

// MockOracle returns a canned price or error for any pair.
type MockOracle struct {
	Price math.LegacyDec
	Err   error
}

func (m MockOracle) GetPrice(_ context.Context, _ string) (math.LegacyDec, error) {
	if m.Err != nil {
		return math.LegacyZeroDec(), m.Err
	}
	return m.Price, nil
}

// MockDistribution records community-pool funding.
type MockDistribution struct {
	Funded sdk.Coins
}

func NewMockDistribution() *MockDistribution { return &MockDistribution{Funded: sdk.NewCoins()} }

func (m *MockDistribution) FundCommunityPool(_ context.Context, amount sdk.Coins, _ sdk.AccAddress) error {
	m.Funded = m.Funded.Add(amount...)
	return nil
}

// RWAKeeper builds an in-memory rwa keeper with the supplied mocks.
func RWAKeeper(t testing.TB, bank *MockBank, oracle types.OracleKeeper, distr types.DistributionKeeper) (rwakeeper.Keeper, sdk.Context) {
	t.Helper()
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	db := cosmosdb.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	k := rwakeeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		log.NewNopLogger(),
		authority,
		MockAccount{},
		bank,
		oracle,
		distr,
	)
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 1}, false, log.NewNopLogger())
	require.NoError(t, k.SetParams(ctx, types.DefaultParams()))
	return k, ctx
}
```

- [ ] **Step 2: Verify compile (expected FAIL until Task 10)**

Run: `go build ./testutil/keeper/...`

Expected: FAIL — `rwakeeper.NewKeeper` not yet defined. Proceed to Task 10; this file is committed there.

---

## Task 10: Keeper skeleton, params, asset CRUD, issuer index

**Files:**
- Replace: `x/rwa/keeper/keeper.go`
- Create: `x/rwa/keeper/keeper_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/rwa/keeper/keeper_test.go`:

```go
package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestParamsRoundTrip(t *testing.T) {
	k, ctx := keeper.RWAKeeper(t, keeper.NewMockBank(), keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	got, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, types.DefaultParams(), got)
}

func TestAssetCRUD(t *testing.T) {
	k, ctx := keeper.RWAKeeper(t, keeper.NewMockBank(), keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())

	_, found := k.GetAsset(ctx, "gold")
	require.False(t, found)

	rec := types.AssetRecord{AssetId: "gold", Issuer: "vtx1xyz", Status: types.AssetStatus_ASSET_STATUS_DRAFT, Denom: "rwa/gold", Bond: "10", NotionalMinted: "0"}
	require.NoError(t, k.SetAsset(ctx, rec))

	got, found := k.GetAsset(ctx, "gold")
	require.True(t, found)
	require.Equal(t, "rwa/gold", got.Denom)
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/rwa/keeper/ -run 'TestParamsRoundTrip|TestAssetCRUD' -v`

Expected: FAIL — `NewKeeper` / `GetAsset` not defined.

- [ ] **Step 3: Write `x/rwa/keeper/keeper.go`**

```go
package keeper

import (
	"context"
	"fmt"

	corestore "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/vertix-network/vertix/x/rwa/types"
)

type Keeper struct {
	cdc          codec.BinaryCodec
	storeService corestore.KVStoreService
	logger       log.Logger
	authority    string

	accountKeeper types.AccountKeeper
	bankKeeper    types.BankKeeper
	oracleKeeper  types.OracleKeeper
	distrKeeper   types.DistributionKeeper
}

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService corestore.KVStoreService,
	logger log.Logger,
	authority string,
	ak types.AccountKeeper,
	bk types.BankKeeper,
	ok types.OracleKeeper,
	dk types.DistributionKeeper,
) Keeper {
	return Keeper{
		cdc:           cdc,
		storeService:  storeService,
		logger:        logger,
		authority:     authority,
		accountKeeper: ak,
		bankKeeper:    bk,
		oracleKeeper:  ok,
		distrKeeper:   dk,
	}
}

func (k Keeper) GetAuthority() string { return k.authority }

func (k Keeper) Logger() log.Logger {
	return k.logger.With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// ModuleAddress is the rwa module account (holds bonds; mints/burns factory denoms).
func (k Keeper) ModuleAddress() sdk.AccAddress {
	return authtypes.NewModuleAddress(types.ModuleName)
}

// --- Params ---

func (k Keeper) GetParams(ctx context.Context) (types.RWAParams, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyParams)
	if err != nil {
		return types.RWAParams{}, err
	}
	if bz == nil {
		return types.DefaultParams(), nil
	}
	var p types.RWAParams
	k.cdc.MustUnmarshal(bz, &p)
	return p, nil
}

func (k Keeper) SetParams(ctx context.Context, p types.RWAParams) error {
	if err := p.Validate(); err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := k.cdc.Marshal(&p)
	if err != nil {
		return err
	}
	return store.Set(types.KeyParams, bz)
}

// --- Asset CRUD ---

func (k Keeper) SetAsset(ctx context.Context, rec types.AssetRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := k.cdc.Marshal(&rec)
	if err != nil {
		return err
	}
	if err := store.Set(types.AssetKey(rec.AssetId), bz); err != nil {
		return err
	}
	issuer, err := sdk.AccAddressFromBech32(rec.Issuer)
	if err != nil {
		return err
	}
	return store.Set(types.IssuerIndexKey(issuer, rec.AssetId), []byte{1})
}

func (k Keeper) GetAsset(ctx context.Context, assetID string) (types.AssetRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.AssetKey(assetID))
	if err != nil || bz == nil {
		return types.AssetRecord{}, false
	}
	var rec types.AssetRecord
	k.cdc.MustUnmarshal(bz, &rec)
	return rec, true
}

// IterateAssets walks every asset record in 0x01 order.
func (k Keeper) IterateAssets(ctx context.Context, fn func(types.AssetRecord) bool) error {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.KeyPrefixAsset, storetypes.PrefixEndBytes(types.KeyPrefixAsset))
	if err != nil {
		return err
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var rec types.AssetRecord
		k.cdc.MustUnmarshal(iter.Value(), &rec)
		if !fn(rec) {
			break
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./x/rwa/keeper/ -run 'TestParamsRoundTrip|TestAssetCRUD' -v`

Expected: PASS

- [ ] **Step 5: Commit keeper + testutil**

```bash
git add x/rwa/keeper/keeper.go x/rwa/keeper/keeper_test.go testutil/keeper/rwa.go
git commit -m "feat(rwa): add keeper skeleton, params, asset CRUD, and test fixture"
```

---

## Task 11: Denom build/parse

**Files:**
- Create: `x/rwa/keeper/denom.go`
- Create: `x/rwa/keeper/denom_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/rwa/keeper/denom_test.go`:

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	rwakeeper "github.com/vertix-network/vertix/x/rwa/keeper"
)

func TestBuildAndParseDenom(t *testing.T) {
	require.Equal(t, "rwa/gold-01", rwakeeper.BuildDenom("gold-01"))

	id, ok := rwakeeper.ParseRWADenom("rwa/gold-01")
	require.True(t, ok)
	require.Equal(t, "gold-01", id)

	_, ok = rwakeeper.ParseRWADenom("uvtx")
	require.False(t, ok)

	_, ok = rwakeeper.ParseRWADenom("ibc/ABC")
	require.False(t, ok)
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/rwa/keeper/ -run TestBuildAndParseDenom -v`

Expected: FAIL — undefined functions.

- [ ] **Step 3: Write `x/rwa/keeper/denom.go`**

```go
package keeper

import (
	"strings"

	"github.com/vertix-network/vertix/x/rwa/types"
)

// BuildDenom returns the factory denom for an asset id.
func BuildDenom(assetID string) string {
	return types.RWADenomPrefix + assetID
}

// ParseRWADenom returns the asset id for an rwa/* denom, or ok=false otherwise.
func ParseRWADenom(denom string) (string, bool) {
	if !strings.HasPrefix(denom, types.RWADenomPrefix) {
		return "", false
	}
	id := strings.TrimPrefix(denom, types.RWADenomPrefix)
	if id == "" {
		return "", false
	}
	return id, true
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./x/rwa/keeper/ -run TestBuildAndParseDenom -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/rwa/keeper/denom.go x/rwa/keeper/denom_test.go
git commit -m "feat(rwa): add rwa/{id} denom build and parse helpers"
```

---

## Task 12: Transfer-restriction membership store + predicate

**Files:**
- Create: `x/rwa/keeper/restriction.go`
- Create: `x/rwa/keeper/restriction_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/rwa/keeper/restriction_test.go`:

```go
package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestCheckTransferAllowed_AllowAll(t *testing.T) {
	k, ctx := keeper.RWAKeeper(t, keeper.NewMockBank(), keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	require.NoError(t, k.SetAsset(ctx, types.AssetRecord{AssetId: "gold", Issuer: sample.AccAddress(), Status: types.AssetStatus_ASSET_STATUS_ACTIVE, Denom: "rwa/gold", Bond: "1", NotionalMinted: "0", AllowAll: true}))

	a, b := sdkAcc(t), sdkAcc(t)
	require.NoError(t, k.CheckTransferAllowed(ctx, "gold", a, b))
}

func TestCheckTransferAllowed_Allowlist(t *testing.T) {
	k, ctx := keeper.RWAKeeper(t, keeper.NewMockBank(), keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	require.NoError(t, k.SetAsset(ctx, types.AssetRecord{AssetId: "gold", Issuer: sample.AccAddress(), Status: types.AssetStatus_ASSET_STATUS_ACTIVE, Denom: "rwa/gold", Bond: "1", NotionalMinted: "0", AllowAll: false}))

	a, b := sdkAcc(t), sdkAcc(t)
	// neither allowlisted -> blocked
	require.ErrorIs(t, k.CheckTransferAllowed(ctx, "gold", a, b), types.ErrTransferRestricted)
	// both allowlisted -> allowed
	require.NoError(t, k.AddAllowlist(ctx, "gold", a))
	require.NoError(t, k.AddAllowlist(ctx, "gold", b))
	require.NoError(t, k.CheckTransferAllowed(ctx, "gold", a, b))
	// deny overrides allow
	require.NoError(t, k.AddDenylist(ctx, "gold", b))
	require.ErrorIs(t, k.CheckTransferAllowed(ctx, "gold", a, b), types.ErrTransferRestricted)
}

func TestCheckTransferAllowed_ModuleExempt(t *testing.T) {
	k, ctx := keeper.RWAKeeper(t, keeper.NewMockBank(), keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	require.NoError(t, k.SetAsset(ctx, types.AssetRecord{AssetId: "gold", Issuer: sample.AccAddress(), Status: types.AssetStatus_ASSET_STATUS_ACTIVE, Denom: "rwa/gold", Bond: "1", NotionalMinted: "0", AllowAll: false}))
	// module address as a party is always allowed (mint/burn/escrow)
	require.NoError(t, k.CheckTransferAllowed(ctx, "gold", k.ModuleAddress(), sdkAcc(t)))
}
```

Add a small shared helper at the bottom of the test file (it is reused by later keeper tests in this package):

```go
func sdkAcc(t *testing.T) sdk.AccAddress {
	t.Helper()
	addr, err := sdk.AccAddressFromBech32(sample.AccAddress())
	require.NoError(t, err)
	return addr
}
```

Add the import `sdk "github.com/cosmos/cosmos-sdk/types"` to the test file.

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/rwa/keeper/ -run TestCheckTransferAllowed -v`

Expected: FAIL — `CheckTransferAllowed`/`AddAllowlist`/`AddDenylist` not defined.

- [ ] **Step 3: Write `x/rwa/keeper/restriction.go`**

```go
package keeper

import (
	"context"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/rwa/types"
)

func (k Keeper) AddAllowlist(ctx context.Context, assetID string, addr sdk.AccAddress) error {
	return k.storeService.OpenKVStore(ctx).Set(types.AllowlistKey(assetID, addr), []byte{1})
}

func (k Keeper) DelAllowlist(ctx context.Context, assetID string, addr sdk.AccAddress) error {
	return k.storeService.OpenKVStore(ctx).Delete(types.AllowlistKey(assetID, addr))
}

func (k Keeper) AddDenylist(ctx context.Context, assetID string, addr sdk.AccAddress) error {
	return k.storeService.OpenKVStore(ctx).Set(types.DenylistKey(assetID, addr), []byte{1})
}

func (k Keeper) DelDenylist(ctx context.Context, assetID string, addr sdk.AccAddress) error {
	return k.storeService.OpenKVStore(ctx).Delete(types.DenylistKey(assetID, addr))
}

func (k Keeper) hasKey(ctx context.Context, key []byte) bool {
	ok, err := k.storeService.OpenKVStore(ctx).Has(key)
	return err == nil && ok
}

func (k Keeper) IsAllowlisted(ctx context.Context, assetID string, addr sdk.AccAddress) bool {
	return k.hasKey(ctx, types.AllowlistKey(assetID, addr))
}

func (k Keeper) IsDenylisted(ctx context.Context, assetID string, addr sdk.AccAddress) bool {
	return k.hasKey(ctx, types.DenylistKey(assetID, addr))
}

// CheckTransferAllowed enforces an asset's restriction policy for a transfer.
// Module-account legs (mint payout, burn collection, bond escrow) are exempt.
func (k Keeper) CheckTransferAllowed(ctx context.Context, assetID string, from, to sdk.AccAddress) error {
	moduleAddr := k.ModuleAddress()
	if from.Equals(moduleAddr) || to.Equals(moduleAddr) {
		return nil
	}
	rec, found := k.GetAsset(ctx, assetID)
	if !found {
		return types.ErrAssetNotFound.Wrap(assetID)
	}
	if !rec.AllowAll {
		if !k.IsAllowlisted(ctx, assetID, from) || !k.IsAllowlisted(ctx, assetID, to) {
			return types.ErrTransferRestricted.Wrapf("asset %s requires both parties allowlisted", assetID)
		}
	}
	if k.IsDenylisted(ctx, assetID, from) || k.IsDenylisted(ctx, assetID, to) {
		return types.ErrTransferRestricted.Wrapf("asset %s: party is denylisted", assetID)
	}
	return nil
}

// IterateMembers walks one membership list (allow or deny) for genesis export/query.
func (k Keeper) IterateMembers(ctx context.Context, prefix []byte, fn func(addr sdk.AccAddress) bool) error {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return err
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		// key layout: prefix(1) | lenByte(1) | assetID | addrBytes
		if len(key) < 2 {
			continue
		}
		idLen := int(key[1])
		if len(key) < 2+idLen {
			continue
		}
		addr := sdk.AccAddress(key[2+idLen:])
		if !fn(addr) {
			break
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./x/rwa/keeper/ -run TestCheckTransferAllowed -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/rwa/keeper/restriction.go x/rwa/keeper/restriction_test.go
git commit -m "feat(rwa): add allow/deny membership store and CheckTransferAllowed"
```

---

## Task 13: Bank send-restriction hook

**Files:**
- Create: `x/rwa/keeper/send_restriction.go`
- Create: `x/rwa/keeper/send_restriction_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/rwa/keeper/send_restriction_test.go`:

```go
package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
	rwakeeper "github.com/vertix-network/vertix/x/rwa/keeper"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestSendRestriction(t *testing.T) {
	k, ctx := keeper.RWAKeeper(t, keeper.NewMockBank(), keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	require.NoError(t, k.SetAsset(ctx, types.AssetRecord{AssetId: "gold", Issuer: sample.AccAddress(), Status: types.AssetStatus_ASSET_STATUS_ACTIVE, Denom: "rwa/gold", Bond: "1", NotionalMinted: "0", AllowAll: false}))

	a, b := sdkAcc(t), sdkAcc(t)

	// non-rwa coin: no-op pass
	_, err := k.SendRestriction(ctx, a, b, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(5))))
	require.NoError(t, err)

	// restricted rwa coin, neither allowlisted: blocked
	_, err = k.SendRestriction(ctx, a, b, sdk.NewCoins(sdk.NewCoin("rwa/gold", math.NewInt(1))))
	require.ErrorIs(t, err, types.ErrTransferRestricted)

	// after allowlisting both: allowed; returns to unchanged
	require.NoError(t, k.AddAllowlist(ctx, "gold", a))
	require.NoError(t, k.AddAllowlist(ctx, "gold", b))
	newTo, err := k.SendRestriction(ctx, a, b, sdk.NewCoins(sdk.NewCoin("rwa/gold", math.NewInt(1))))
	require.NoError(t, err)
	require.Equal(t, b, newTo)

	// module-origin leg always allowed
	_, err = k.SendRestriction(ctx, k.ModuleAddress(), a, sdk.NewCoins(sdk.NewCoin("rwa/gold", math.NewInt(1))))
	require.NoError(t, err)

	_ = rwakeeper.BuildDenom // keep import
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/rwa/keeper/ -run TestSendRestriction -v`

Expected: FAIL — `SendRestriction` not defined.

- [ ] **Step 3: Write `x/rwa/keeper/send_restriction.go`**

```go
package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SendRestriction is registered via bankKeeper.AppendSendRestriction in app.go.
// It runs inside BankKeeper.SendCoins — the chokepoint for MsgSend, MsgMultiSend,
// authz-wrapped sends, AND IBC transfer escrow — so rwa/* restriction semantics
// cannot be bypassed. Non-rwa denoms are a cheap no-op.
func (k Keeper) SendRestriction(ctx context.Context, from, to sdk.AccAddress, amt sdk.Coins) (sdk.AccAddress, error) {
	for _, c := range amt {
		assetID, ok := ParseRWADenom(c.Denom)
		if !ok {
			continue
		}
		if err := k.CheckTransferAllowed(ctx, assetID, from, to); err != nil {
			return to, err
		}
	}
	return to, nil
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./x/rwa/keeper/ -run TestSendRestriction -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/rwa/keeper/send_restriction.go x/rwa/keeper/send_restriction_test.go
git commit -m "feat(rwa): add bank SendRestriction hook (covers MsgSend/authz/IBC)"
```

---

## Task 14: Bond and fee helpers

**Files:**
- Create: `x/rwa/keeper/bond.go`
- Create: `x/rwa/keeper/fee.go`
- Create: `x/rwa/keeper/bond_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/rwa/keeper/bond_test.go`:

```go
package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestComputeFee(t *testing.T) {
	k, _ := keeper.RWAKeeper(t, keeper.NewMockBank(), keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	// 1_000_000 * 0.001 = 1000
	fee := k.ComputeFee(math.NewInt(1_000_000), math.LegacyNewDecWithPrec(1, 3))
	require.Equal(t, math.NewInt(1000), fee)
	// floor: 7 * 0.001 = 0.007 -> 0
	require.Equal(t, math.ZeroInt(), k.ComputeFee(math.NewInt(7), math.LegacyNewDecWithPrec(1, 3)))
}

func TestLockAndReleaseBond(t *testing.T) {
	bank := keeper.NewMockBank()
	k, ctx := keeper.RWAKeeper(t, bank, keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())

	issuer := sdkAcc(t)
	bank.SetBalance(issuer, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(10000))))

	require.NoError(t, k.LockBond(ctx, issuer, math.NewInt(6000)))
	require.Equal(t, math.NewInt(6000), bank.GetBalance(ctx, k.ModuleAddress(), "uvtx").Amount)
	require.Equal(t, math.NewInt(4000), bank.GetBalance(ctx, issuer, "uvtx").Amount)

	require.NoError(t, k.ReleaseBond(ctx, issuer, math.NewInt(6000)))
	require.Equal(t, math.NewInt(10000), bank.GetBalance(ctx, issuer, "uvtx").Amount)
	require.True(t, bank.GetBalance(ctx, k.ModuleAddress(), "uvtx").IsZero())
}

func TestSlashBondToCommunityPool(t *testing.T) {
	bank := keeper.NewMockBank()
	distr := keeper.NewMockDistribution()
	k, ctx := keeper.RWAKeeper(t, bank, keeper.MockOracle{Price: math.LegacyNewDec(1)}, distr)

	bank.SetBalance(k.ModuleAddress(), sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(5000))))
	require.NoError(t, k.SlashBondToCommunityPool(ctx, math.NewInt(5000)))
	require.Equal(t, math.NewInt(5000), distr.Funded.AmountOf("uvtx"))

	_ = authtypes.FeeCollectorName // keep import
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/rwa/keeper/ -run 'TestComputeFee|TestLockAndReleaseBond|TestSlashBond' -v`

Expected: FAIL — helpers not defined.

- [ ] **Step 3: Write `x/rwa/keeper/fee.go`**

```go
package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/vertix-network/vertix/x/rwa/types"
)

// ComputeFee returns floor(notional * rate) in uvtx units.
func (k Keeper) ComputeFee(notional math.Int, rate math.LegacyDec) math.Int {
	return math.LegacyNewDecFromInt(notional).Mul(rate).TruncateInt()
}

// CollectFee debits `fee` uvtx from the payer straight into the fee collector
// (single hop — the fee never transits the rwa module account). x/fees sweeps
// it at the next EndBlock. A zero fee is a no-op.
func (k Keeper) CollectFee(ctx context.Context, payer sdk.AccAddress, fee math.Int) error {
	if !fee.IsPositive() {
		return nil
	}
	coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, fee))
	return k.bankKeeper.SendCoinsFromAccountToModule(ctx, payer, authtypes.FeeCollectorName, coins)
}
```

- [ ] **Step 4: Write `x/rwa/keeper/bond.go`**

```go
package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/rwa/types"
)

// LockBond escrows `amount` uvtx from the issuer into the module account.
func (k Keeper) LockBond(ctx context.Context, issuer sdk.AccAddress, amount math.Int) error {
	coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, amount))
	return k.bankKeeper.SendCoinsFromAccountToModule(ctx, issuer, types.ModuleName, coins)
}

// ReleaseBond returns `amount` uvtx from the module account to the issuer.
func (k Keeper) ReleaseBond(ctx context.Context, issuer sdk.AccAddress, amount math.Int) error {
	coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, amount))
	return k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, issuer, coins)
}

// SlashBondToCommunityPool moves `amount` uvtx from the module account into the
// x/distribution community pool (governance dispute outcome).
func (k Keeper) SlashBondToCommunityPool(ctx context.Context, amount math.Int) error {
	coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, amount))
	return k.distrKeeper.FundCommunityPool(ctx, coins, k.ModuleAddress())
}
```

- [ ] **Step 5: Run tests — expect PASS**

Run: `go test ./x/rwa/keeper/ -run 'TestComputeFee|TestLockAndReleaseBond|TestSlashBond' -v`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add x/rwa/keeper/fee.go x/rwa/keeper/bond.go x/rwa/keeper/bond_test.go
git commit -m "feat(rwa): add bond escrow and deterministic fee helpers"
```

---

## Task 15: MsgServer skeleton + RegisterAsset

**Files:**
- Replace: `x/rwa/keeper/msg_server.go` (overwrite scaffold)
- Create: `x/rwa/keeper/msg_server_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/rwa/keeper/msg_server_test.go`:

```go
package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	rwakeeper "github.com/vertix-network/vertix/x/rwa/keeper"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func setupServer(t *testing.T, oracle types.OracleKeeper) (types.MsgServer, rwakeeper.Keeper, *keeper.MockBank, sdk.Context) {
	bank := keeper.NewMockBank()
	k, ctx := keeper.RWAKeeper(t, bank, oracle, keeper.NewMockDistribution())
	return rwakeeper.NewMsgServerImpl(k), k, bank, ctx
}

func TestRegisterAsset(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keeper.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := sdkAcc(t)
	bank.SetBalance(issuer, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(20_000_000_000))))

	_, err := srv.RegisterAsset(ctx, &types.MsgRegisterAsset{
		Issuer: issuer.String(), AssetId: "gold-01", Name: "Gold", OraclePair: "XAU:USD", Bond: "10000000000",
	})
	require.NoError(t, err)

	rec, found := k.GetAsset(ctx, "gold-01")
	require.True(t, found)
	require.Equal(t, types.AssetStatus_ASSET_STATUS_DRAFT, rec.Status)
	require.Equal(t, "rwa/gold-01", rec.Denom)
	require.True(t, rec.AllowAll)
	require.Equal(t, math.NewInt(10_000_000_000), bank.GetBalance(ctx, k.ModuleAddress(), "uvtx").Amount)
}

func TestRegisterAssetRejectsDuplicateAndLowBond(t *testing.T) {
	srv, _, bank, ctx := setupServer(t, keeper.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := sdkAcc(t)
	bank.SetBalance(issuer, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(20_000_000_000))))

	base := &types.MsgRegisterAsset{Issuer: issuer.String(), AssetId: "gold-01", Name: "Gold", OraclePair: "XAU:USD", Bond: "10000000000"}
	_, err := srv.RegisterAsset(ctx, base)
	require.NoError(t, err)

	_, err = srv.RegisterAsset(ctx, base)
	require.ErrorIs(t, err, types.ErrAssetExists)

	_, err = srv.RegisterAsset(ctx, &types.MsgRegisterAsset{Issuer: issuer.String(), AssetId: "silver-01", Name: "Silver", OraclePair: "XAG:USD", Bond: "1"})
	require.ErrorIs(t, err, types.ErrBondTooLow)
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/rwa/keeper/ -run TestRegisterAsset -v`

Expected: FAIL — `NewMsgServerImpl`/`RegisterAsset` differ from scaffold.

- [ ] **Step 3: Write `x/rwa/keeper/msg_server.go`** (skeleton + RegisterAsset; other handlers added in later tasks to the same file)

```go
package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/rwa/types"
)

type msgServer struct {
	Keeper
}

func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

func (ms msgServer) RegisterAsset(ctx context.Context, msg *types.MsgRegisterAsset) (*types.MsgRegisterAssetResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	if _, found := ms.GetAsset(ctx, msg.AssetId); found {
		return nil, types.ErrAssetExists.Wrap(msg.AssetId)
	}
	params, err := ms.GetParams(ctx)
	if err != nil {
		return nil, err
	}
	minBond, _ := params.MinIssuerBondInt()
	bond, ok := math.NewIntFromString(msg.Bond)
	if !ok {
		return nil, types.ErrInvalidAmount.Wrap("bond")
	}
	if bond.LT(minBond) {
		return nil, types.ErrBondTooLow.Wrapf("bond %s < min %s", bond, minBond)
	}
	issuer, err := sdk.AccAddressFromBech32(msg.Issuer)
	if err != nil {
		return nil, err
	}
	if err := ms.LockBond(ctx, issuer, bond); err != nil {
		return nil, err
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	rec := types.AssetRecord{
		AssetId:        msg.AssetId,
		Issuer:         msg.Issuer,
		Name:           msg.Name,
		Description:    msg.Description,
		Status:         types.AssetStatus_ASSET_STATUS_DRAFT,
		OraclePair:     msg.OraclePair,
		Denom:          BuildDenom(msg.AssetId),
		Bond:           bond.String(),
		NotionalMinted: math.ZeroInt().String(),
		AllowAll:       true,
		CreatedAt:      sdkCtx.BlockTime(),
	}
	if err := ms.SetAsset(ctx, rec); err != nil {
		return nil, err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeAssetRegistered,
		sdk.NewAttribute(types.AttributeKeyAssetID, msg.AssetId),
		sdk.NewAttribute(types.AttributeKeyIssuer, msg.Issuer),
		sdk.NewAttribute(types.AttributeKeyBond, bond.String()),
	))
	return &types.MsgRegisterAssetResponse{}, nil
}

// requireIssuer loads an asset and asserts the caller is its issuer.
func (ms msgServer) requireIssuer(ctx context.Context, assetID, caller string) (types.AssetRecord, error) {
	rec, found := ms.GetAsset(ctx, assetID)
	if !found {
		return types.AssetRecord{}, types.ErrAssetNotFound.Wrap(assetID)
	}
	if rec.Issuer != caller {
		return types.AssetRecord{}, errorsmod.Wrapf(types.ErrUnauthorized, "caller %s is not issuer %s", caller, rec.Issuer)
	}
	return rec, nil
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./x/rwa/keeper/ -run TestRegisterAsset -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/rwa/keeper/msg_server.go x/rwa/keeper/msg_server_test.go
git commit -m "feat(rwa): MsgServer skeleton + RegisterAsset (bond locked, DRAFT)"
```

---

## Task 16: AttestAsset (oracle-gated)

**Files:**
- Modify: `x/rwa/keeper/msg_server.go` (append `AttestAsset`)
- Modify: `x/rwa/keeper/msg_server_test.go` (append tests)

- [ ] **Step 1: Append the failing test**

Add to `x/rwa/keeper/msg_server_test.go`:

```go
func registerDraft(t *testing.T, srv types.MsgServer, k rwakeeper.Keeper, bank *keeper.MockBank, ctx sdk.Context, id string) sdk.AccAddress {
	t.Helper()
	issuer := sdkAcc(t)
	bank.SetBalance(issuer, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(20_000_000_000))))
	_, err := srv.RegisterAsset(ctx, &types.MsgRegisterAsset{Issuer: issuer.String(), AssetId: id, Name: "N", OraclePair: "XAU:USD", Bond: "10000000000"})
	require.NoError(t, err)
	return issuer
}

func TestAttestAssetSuccess(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keeper.MockOracle{Price: math.LegacyNewDec(1900)})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")

	_, err := srv.AttestAsset(ctx, &types.MsgAttestAsset{Issuer: issuer.String(), AssetId: "gold-01"})
	require.NoError(t, err)

	rec, _ := k.GetAsset(ctx, "gold-01")
	require.Equal(t, types.AssetStatus_ASSET_STATUS_ATTESTED, rec.Status)
	require.Equal(t, "1900.000000000000000000", rec.AttestedPrice)
}

func TestAttestAssetOracleFailure(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keeper.MockOracle{Err: errFakeStale})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")

	_, err := srv.AttestAsset(ctx, &types.MsgAttestAsset{Issuer: issuer.String(), AssetId: "gold-01"})
	require.ErrorIs(t, err, types.ErrAttestationFailed)
}
```

Add this package-level error to the test file (top, after imports), using a plain stdlib error — the handler only checks `err != nil`. **Do not** register a fake error in the `oracle` codespace: `testutil/keeper` transitively imports `x/oracle/types`, which already registers `oracle/7` (`ErrStalePrice`), so `errors.Register("oracle", 7, ...)` would panic at init with a duplicate-registration error.

```go
var errFakeStale = errors.New("oracle: stale price")
```

Add the standard-library import `"errors"` to the test file.

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/rwa/keeper/ -run TestAttestAsset -v`

Expected: FAIL — `AttestAsset` not defined.

- [ ] **Step 3: Append `AttestAsset` to `x/rwa/keeper/msg_server.go`**

```go
func (ms msgServer) AttestAsset(ctx context.Context, msg *types.MsgAttestAsset) (*types.MsgAttestAssetResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	rec, err := ms.requireIssuer(ctx, msg.AssetId, msg.Issuer)
	if err != nil {
		return nil, err
	}
	if rec.Status != types.AssetStatus_ASSET_STATUS_DRAFT {
		return nil, types.ErrInvalidStatus.Wrapf("attest requires DRAFT, got %s", rec.Status)
	}
	price, err := ms.oracleKeeper.GetPrice(ctx, rec.OraclePair)
	if err != nil {
		return nil, types.ErrAttestationFailed.Wrapf("pair %s: %v", rec.OraclePair, err)
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	rec.Status = types.AssetStatus_ASSET_STATUS_ATTESTED
	rec.AttestedPrice = price.String()
	rec.AttestedAt = sdkCtx.BlockTime()
	if err := ms.SetAsset(ctx, rec); err != nil {
		return nil, err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeAssetAttested,
		sdk.NewAttribute(types.AttributeKeyAssetID, rec.AssetId),
		sdk.NewAttribute(types.AttributeKeyOraclePair, rec.OraclePair),
		sdk.NewAttribute(types.AttributeKeyAttestedPrice, rec.AttestedPrice),
	))
	return &types.MsgAttestAssetResponse{}, nil
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./x/rwa/keeper/ -run TestAttestAsset -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/rwa/keeper/msg_server.go x/rwa/keeper/msg_server_test.go
git commit -m "feat(rwa): AttestAsset gated on oracle GetPrice with snapshot"
```

---

## Task 17: MintRWA

**Files:**
- Modify: `x/rwa/keeper/msg_server.go` (append `MintRWA`)
- Modify: `x/rwa/keeper/msg_server_test.go` (append tests)

- [ ] **Step 1: Append the failing test**

Add to `x/rwa/keeper/msg_server_test.go`:

```go
func attestAsset(t *testing.T, srv types.MsgServer, issuer sdk.AccAddress, id string, ctx sdk.Context) {
	t.Helper()
	_, err := srv.AttestAsset(ctx, &types.MsgAttestAsset{Issuer: issuer.String(), AssetId: id})
	require.NoError(t, err)
}

func TestMintRWA(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keeper.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")
	attestAsset(t, srv, issuer, "gold-01", ctx)

	_, err := srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000000"})
	require.NoError(t, err)

	rec, _ := k.GetAsset(ctx, "gold-01")
	require.Equal(t, types.AssetStatus_ASSET_STATUS_ACTIVE, rec.Status)
	require.Equal(t, math.NewInt(1000000).String(), rec.NotionalMinted)
	// issuer holds 1_000_000 rwa/gold-01
	require.Equal(t, math.NewInt(1000000), bank.GetBalance(ctx, issuer, "rwa/gold-01").Amount)
	// 0.001 fee = 1000 uvtx routed to fee collector
	require.Equal(t, math.NewInt(1000), bank.GetBalance(ctx, k.ModuleAddr("fee_collector"), "uvtx").Amount)
}

func TestMintRWARequiresAttested(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keeper.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")
	_ = k
	_, err := srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000"})
	require.ErrorIs(t, err, types.ErrInvalidStatus)
}

// Spec §8 / residual-risk: the chosen model is "accumulate" — a re-mint on an
// ACTIVE asset adds supply and accrues notional_minted.
func TestMintRWAReMintAccumulates(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keeper.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")
	attestAsset(t, srv, issuer, "gold-01", ctx)

	_, err := srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000000"})
	require.NoError(t, err)
	_, err = srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "500000"})
	require.NoError(t, err)

	rec, _ := k.GetAsset(ctx, "gold-01")
	require.Equal(t, math.NewInt(1500000).String(), rec.NotionalMinted)
	require.Equal(t, math.NewInt(1500000), bank.GetBalance(ctx, issuer, "rwa/gold-01").Amount)
}
```

Note: `fee_collector` is the module name for `authtypes.FeeCollectorName`; `k.ModuleAddr` resolves it via `authtypes.NewModuleAddress`.

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/rwa/keeper/ -run TestMintRWA -v`

Expected: FAIL — `MintRWA` not defined.

- [ ] **Step 3: Append `MintRWA` to `x/rwa/keeper/msg_server.go`**

```go
func (ms msgServer) MintRWA(ctx context.Context, msg *types.MsgMintRWA) (*types.MsgMintRWAResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	rec, err := ms.requireIssuer(ctx, msg.AssetId, msg.Issuer)
	if err != nil {
		return nil, err
	}
	// First mint requires ATTESTED; subsequent mints accumulate on ACTIVE.
	if rec.Status != types.AssetStatus_ASSET_STATUS_ATTESTED && rec.Status != types.AssetStatus_ASSET_STATUS_ACTIVE {
		return nil, types.ErrInvalidStatus.Wrapf("mint requires ATTESTED or ACTIVE, got %s", rec.Status)
	}
	notional, ok := math.NewIntFromString(msg.Notional)
	if !ok || !notional.IsPositive() {
		return nil, types.ErrInvalidNotional.Wrap(msg.Notional)
	}
	params, err := ms.GetParams(ctx)
	if err != nil {
		return nil, err
	}
	rate, err := params.MintFeeRateDec()
	if err != nil {
		return nil, err
	}
	issuer, err := sdk.AccAddressFromBech32(msg.Issuer)
	if err != nil {
		return nil, err
	}
	fee := ms.ComputeFee(notional, rate)
	if err := ms.CollectFee(ctx, issuer, fee); err != nil {
		return nil, err
	}
	mintCoins := sdk.NewCoins(sdk.NewCoin(rec.Denom, notional))
	if err := ms.bankKeeper.MintCoins(ctx, types.ModuleName, mintCoins); err != nil {
		return nil, err
	}
	if err := ms.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, issuer, mintCoins); err != nil {
		return nil, err
	}
	prev, _ := math.NewIntFromString(rec.NotionalMinted)
	rec.NotionalMinted = prev.Add(notional).String()
	rec.Status = types.AssetStatus_ASSET_STATUS_ACTIVE
	if err := ms.SetAsset(ctx, rec); err != nil {
		return nil, err
	}
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeRWAMinted,
		sdk.NewAttribute(types.AttributeKeyAssetID, rec.AssetId),
		sdk.NewAttribute(types.AttributeKeyNotional, notional.String()),
		sdk.NewAttribute(types.AttributeKeyMintFee, fee.String()),
	))
	return &types.MsgMintRWAResponse{}, nil
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./x/rwa/keeper/ -run TestMintRWA -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/rwa/keeper/msg_server.go x/rwa/keeper/msg_server_test.go
git commit -m "feat(rwa): MintRWA mints rwa/{id} 1:1 with deterministic fee"
```

---

## Task 18: TransferRWA

**Files:**
- Modify: `x/rwa/keeper/msg_server.go` (append `TransferRWA`)
- Modify: `x/rwa/keeper/msg_server_test.go` (append tests)

- [ ] **Step 1: Append the failing test**

Add to `x/rwa/keeper/msg_server_test.go`:

```go
func TestTransferRWA(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keeper.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")
	attestAsset(t, srv, issuer, "gold-01", ctx)
	_, err := srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000"})
	require.NoError(t, err)

	bob := sdkAcc(t)
	_, err = srv.TransferRWA(ctx, &types.MsgTransferRWA{Sender: issuer.String(), Recipient: bob.String(), AssetId: "gold-01", Amount: "300"})
	require.NoError(t, err)
	require.Equal(t, math.NewInt(300), bank.GetBalance(ctx, bob, "rwa/gold-01").Amount)
	require.Equal(t, math.NewInt(700), bank.GetBalance(ctx, issuer, "rwa/gold-01").Amount)
}

func TestTransferRWARestricted(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keeper.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")
	attestAsset(t, srv, issuer, "gold-01", ctx)
	_, err := srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000"})
	require.NoError(t, err)

	// turn on allowlist mode with nobody listed
	_, err = srv.UpdateRestrictions(ctx, &types.MsgUpdateRestrictions{Issuer: issuer.String(), AssetId: "gold-01", AllowAll: false})
	require.NoError(t, err)

	bob := sdkAcc(t)
	_, err = srv.TransferRWA(ctx, &types.MsgTransferRWA{Sender: issuer.String(), Recipient: bob.String(), AssetId: "gold-01", Amount: "100"})
	require.ErrorIs(t, err, types.ErrTransferRestricted)
}
```

(`TestTransferRWARestricted` also exercises `UpdateRestrictions` from Task 19; if running this task in isolation before Task 19, expect a compile error for `UpdateRestrictions` — implement Task 19 first or run the two tasks together. The recommended order keeps Task 19 immediately after.)

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/rwa/keeper/ -run TestTransferRWA -v`

Expected: FAIL — `TransferRWA` not defined.

- [ ] **Step 3: Append `TransferRWA` to `x/rwa/keeper/msg_server.go`**

```go
func (ms msgServer) TransferRWA(ctx context.Context, msg *types.MsgTransferRWA) (*types.MsgTransferRWAResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	rec, found := ms.GetAsset(ctx, msg.AssetId)
	if !found {
		return nil, types.ErrAssetNotFound.Wrap(msg.AssetId)
	}
	if rec.Status != types.AssetStatus_ASSET_STATUS_ACTIVE {
		return nil, types.ErrInvalidStatus.Wrapf("transfer requires ACTIVE, got %s", rec.Status)
	}
	sender, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return nil, err
	}
	recipient, err := sdk.AccAddressFromBech32(msg.Recipient)
	if err != nil {
		return nil, err
	}
	if err := ms.CheckTransferAllowed(ctx, msg.AssetId, sender, recipient); err != nil {
		return nil, err
	}
	amount, ok := math.NewIntFromString(msg.Amount)
	if !ok || !amount.IsPositive() {
		return nil, types.ErrInvalidAmount.Wrap(msg.Amount)
	}
	coins := sdk.NewCoins(sdk.NewCoin(rec.Denom, amount))
	if err := ms.bankKeeper.SendCoins(ctx, sender, recipient, coins); err != nil {
		return nil, err
	}
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeRWATransferred,
		sdk.NewAttribute(types.AttributeKeyAssetID, rec.AssetId),
		sdk.NewAttribute(types.AttributeKeySender, msg.Sender),
		sdk.NewAttribute(types.AttributeKeyRecipient, msg.Recipient),
		sdk.NewAttribute(types.AttributeKeyAmount, amount.String()),
	))
	return &types.MsgTransferRWAResponse{}, nil
}
```

- [ ] **Step 4: Run tests — expect PASS** (after Task 19 is in place, or run 18+19 together)

Run: `go test ./x/rwa/keeper/ -run TestTransferRWA -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/rwa/keeper/msg_server.go x/rwa/keeper/msg_server_test.go
git commit -m "feat(rwa): TransferRWA with restriction check"
```

---

## Task 19: UpdateRestrictions

**Files:**
- Modify: `x/rwa/keeper/msg_server.go` (append `UpdateRestrictions`)
- Modify: `x/rwa/keeper/msg_server_test.go` (append test)

- [ ] **Step 1: Append the failing test**

Add to `x/rwa/keeper/msg_server_test.go`:

```go
func TestUpdateRestrictions(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keeper.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")

	alice := sdkAcc(t)
	_, err := srv.UpdateRestrictions(ctx, &types.MsgUpdateRestrictions{
		Issuer: issuer.String(), AssetId: "gold-01", AllowAll: false, AddAllow: []string{alice.String()},
	})
	require.NoError(t, err)

	rec, _ := k.GetAsset(ctx, "gold-01")
	require.False(t, rec.AllowAll)
	require.True(t, k.IsAllowlisted(ctx, "gold-01", alice))

	// non-issuer cannot update
	_, err = srv.UpdateRestrictions(ctx, &types.MsgUpdateRestrictions{Issuer: sdkAcc(t).String(), AssetId: "gold-01", AllowAll: true})
	require.ErrorIs(t, err, types.ErrUnauthorized)
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/rwa/keeper/ -run TestUpdateRestrictions -v`

Expected: FAIL — `UpdateRestrictions` not defined.

- [ ] **Step 3: Append `UpdateRestrictions` to `x/rwa/keeper/msg_server.go`**

```go
func (ms msgServer) UpdateRestrictions(ctx context.Context, msg *types.MsgUpdateRestrictions) (*types.MsgUpdateRestrictionsResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	rec, err := ms.requireIssuer(ctx, msg.AssetId, msg.Issuer)
	if err != nil {
		return nil, err
	}
	if rec.Status == types.AssetStatus_ASSET_STATUS_SETTLED {
		return nil, types.ErrInvalidStatus.Wrap("cannot update restrictions on a settled asset")
	}
	apply := func(list []string, add bool, fnAdd, fnDel func(context.Context, string, sdk.AccAddress) error) error {
		for _, a := range list {
			addr, err := sdk.AccAddressFromBech32(a)
			if err != nil {
				return err
			}
			if add {
				if err := fnAdd(ctx, msg.AssetId, addr); err != nil {
					return err
				}
			} else if err := fnDel(ctx, msg.AssetId, addr); err != nil {
				return err
			}
		}
		return nil
	}
	if err := apply(msg.AddAllow, true, ms.AddAllowlist, ms.DelAllowlist); err != nil {
		return nil, err
	}
	if err := apply(msg.DelAllow, false, ms.AddAllowlist, ms.DelAllowlist); err != nil {
		return nil, err
	}
	if err := apply(msg.AddDeny, true, ms.AddDenylist, ms.DelDenylist); err != nil {
		return nil, err
	}
	if err := apply(msg.DelDeny, false, ms.AddDenylist, ms.DelDenylist); err != nil {
		return nil, err
	}
	rec.AllowAll = msg.AllowAll
	if err := ms.SetAsset(ctx, rec); err != nil {
		return nil, err
	}
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeRestrictionUpdated,
		sdk.NewAttribute(types.AttributeKeyAssetID, rec.AssetId),
		sdk.NewAttribute(types.AttributeKeyAllowAll, strconv.FormatBool(rec.AllowAll)),
	))
	return &types.MsgUpdateRestrictionsResponse{}, nil
}
```

Add `"strconv"` to the `msg_server.go` import block.

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./x/rwa/keeper/ -run 'TestUpdateRestrictions|TestTransferRWA' -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/rwa/keeper/msg_server.go x/rwa/keeper/msg_server_test.go
git commit -m "feat(rwa): UpdateRestrictions manages allow/deny membership"
```

---

## Task 20: SettleRWA

**Files:**
- Modify: `x/rwa/keeper/msg_server.go` (append `SettleRWA`)
- Modify: `x/rwa/keeper/msg_server_test.go` (append tests)

- [ ] **Step 1: Append the failing test**

Add to `x/rwa/keeper/msg_server_test.go`:

```go
func TestSettleRWA(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keeper.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")
	attestAsset(t, srv, issuer, "gold-01", ctx)
	_, err := srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000000"})
	require.NoError(t, err)

	issuerUvtxBefore := bank.GetBalance(ctx, issuer, "uvtx").Amount

	_, err = srv.SettleRWA(ctx, &types.MsgSettleRWA{Issuer: issuer.String(), AssetId: "gold-01"})
	require.NoError(t, err)

	rec, _ := k.GetAsset(ctx, "gold-01")
	require.Equal(t, types.AssetStatus_ASSET_STATUS_SETTLED, rec.Status)
	// rwa tokens burned from issuer
	require.True(t, bank.GetBalance(ctx, issuer, "rwa/gold-01").IsZero())
	// bond (10_000_000_000) returned minus 1000 settle fee paid from uvtx
	expected := issuerUvtxBefore.Add(math.NewInt(10_000_000_000)).Sub(math.NewInt(1000))
	require.Equal(t, expected, bank.GetBalance(ctx, issuer, "uvtx").Amount)
	require.True(t, bank.GetBalance(ctx, k.ModuleAddress(), "uvtx").IsZero()) // bond released
}

func TestSettleRWARequiresIssuerHoldsAllUnits(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keeper.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")
	attestAsset(t, srv, issuer, "gold-01", ctx)
	_, err := srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000"})
	require.NoError(t, err)
	// move some units away
	bob := sdkAcc(t)
	_, err = srv.TransferRWA(ctx, &types.MsgTransferRWA{Sender: issuer.String(), Recipient: bob.String(), AssetId: "gold-01", Amount: "400"})
	require.NoError(t, err)
	// settle should fail: issuer no longer holds all 1000 units
	_, err = srv.SettleRWA(ctx, &types.MsgSettleRWA{Issuer: issuer.String(), AssetId: "gold-01"})
	require.Error(t, err)
	_ = k
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/rwa/keeper/ -run TestSettleRWA -v`

Expected: FAIL — `SettleRWA` not defined.

- [ ] **Step 3: Append `SettleRWA` to `x/rwa/keeper/msg_server.go`**

```go
func (ms msgServer) SettleRWA(ctx context.Context, msg *types.MsgSettleRWA) (*types.MsgSettleRWAResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	rec, err := ms.requireIssuer(ctx, msg.AssetId, msg.Issuer)
	if err != nil {
		return nil, err
	}
	if rec.Status != types.AssetStatus_ASSET_STATUS_ACTIVE {
		return nil, types.ErrInvalidStatus.Wrapf("settle requires ACTIVE, got %s", rec.Status)
	}
	issuer, err := sdk.AccAddressFromBech32(msg.Issuer)
	if err != nil {
		return nil, err
	}
	notionalMinted, _ := math.NewIntFromString(rec.NotionalMinted)

	// settle fee on the minted notional
	params, err := ms.GetParams(ctx)
	if err != nil {
		return nil, err
	}
	rate, err := params.SettleFeeRateDec()
	if err != nil {
		return nil, err
	}
	fee := ms.ComputeFee(notionalMinted, rate)
	if err := ms.CollectFee(ctx, issuer, fee); err != nil {
		return nil, err
	}

	// clean settle: issuer must hold the full minted supply; collect + burn it
	burnCoins := sdk.NewCoins(sdk.NewCoin(rec.Denom, notionalMinted))
	if notionalMinted.IsPositive() {
		if err := ms.bankKeeper.SendCoinsFromAccountToModule(ctx, issuer, types.ModuleName, burnCoins); err != nil {
			return nil, err
		}
		if err := ms.bankKeeper.BurnCoins(ctx, types.ModuleName, burnCoins); err != nil {
			return nil, err
		}
	}

	// release bond
	bond, _ := math.NewIntFromString(rec.Bond)
	if bond.IsPositive() {
		if err := ms.ReleaseBond(ctx, issuer, bond); err != nil {
			return nil, err
		}
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	rec.Status = types.AssetStatus_ASSET_STATUS_SETTLED
	rec.SettledAt = sdkCtx.BlockTime()
	if err := ms.SetAsset(ctx, rec); err != nil {
		return nil, err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeRWASettled,
		sdk.NewAttribute(types.AttributeKeyAssetID, rec.AssetId),
		sdk.NewAttribute(types.AttributeKeySettleFee, fee.String()),
		sdk.NewAttribute(types.AttributeKeyBondReturned, bond.String()),
	))
	return &types.MsgSettleRWAResponse{}, nil
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./x/rwa/keeper/ -run TestSettleRWA -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/rwa/keeper/msg_server.go x/rwa/keeper/msg_server_test.go
git commit -m "feat(rwa): SettleRWA burns supply, returns bond (issuer-only)"
```

---

## Task 21: SlashBond + UpdateParams + Params query

**Files:**
- Modify: `x/rwa/keeper/msg_server.go` (append `SlashBond`, `UpdateParams`)
- Replace: `x/rwa/keeper/grpc_query.go` (overwrite scaffold)
- Modify: `x/rwa/keeper/msg_server_test.go` (append tests)

- [ ] **Step 1: Append the failing tests**

Add to `x/rwa/keeper/msg_server_test.go`:

```go
func TestSlashBond(t *testing.T) {
	bank := keeper.NewMockBank()
	distr := keeper.NewMockDistribution()
	k, ctx := keeper.RWAKeeper(t, bank, keeper.MockOracle{Price: math.LegacyNewDec(1)}, distr)
	srv := rwakeeper.NewMsgServerImpl(k)

	issuer := sdkAcc(t)
	bank.SetBalance(issuer, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(20_000_000_000))))
	_, err := srv.RegisterAsset(ctx, &types.MsgRegisterAsset{Issuer: issuer.String(), AssetId: "gold-01", Name: "N", OraclePair: "XAU:USD", Bond: "10000000000"})
	require.NoError(t, err)
	_, err = srv.AttestAsset(ctx, &types.MsgAttestAsset{Issuer: issuer.String(), AssetId: "gold-01"})
	require.NoError(t, err)
	_, err = srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000"})
	require.NoError(t, err)

	// wrong authority rejected
	_, err = srv.SlashBond(ctx, &types.MsgSlashBond{Authority: sdkAcc(t).String(), AssetId: "gold-01"})
	require.ErrorIs(t, err, types.ErrUnauthorized)

	// gov authority slashes bond into community pool, force-settles
	_, err = srv.SlashBond(ctx, &types.MsgSlashBond{Authority: k.GetAuthority(), AssetId: "gold-01", Reason: "fraud"})
	require.NoError(t, err)
	rec, _ := k.GetAsset(ctx, "gold-01")
	require.Equal(t, types.AssetStatus_ASSET_STATUS_SETTLED, rec.Status)
	require.Equal(t, math.NewInt(10_000_000_000), distr.Funded.AmountOf("uvtx"))
}

func TestUpdateParams(t *testing.T) {
	k, ctx := keeper.RWAKeeper(t, keeper.NewMockBank(), keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	srv := rwakeeper.NewMsgServerImpl(k)

	newParams := types.RWAParams{MinIssuerBond: "5000000000", MintFeeRate: "0.002", SettleFeeRate: "0.002"}
	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{Authority: k.GetAuthority(), Params: newParams})
	require.NoError(t, err)
	got, _ := k.GetParams(ctx)
	require.Equal(t, newParams, got)

	_, err = srv.UpdateParams(ctx, &types.MsgUpdateParams{Authority: sdkAcc(t).String(), Params: types.DefaultParams()})
	require.ErrorIs(t, err, types.ErrInvalidSigner)
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/rwa/keeper/ -run 'TestSlashBond|TestUpdateParams' -v`

Expected: FAIL — handlers not defined.

- [ ] **Step 3: Append `SlashBond` + `UpdateParams` to `x/rwa/keeper/msg_server.go`**

```go
func (ms msgServer) SlashBond(ctx context.Context, msg *types.MsgSlashBond) (*types.MsgSlashBondResponse, error) {
	if ms.GetAuthority() != msg.Authority {
		return nil, errorsmod.Wrapf(types.ErrUnauthorized, "expected gov authority %s, got %s", ms.GetAuthority(), msg.Authority)
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	rec, found := ms.GetAsset(ctx, msg.AssetId)
	if !found {
		return nil, types.ErrAssetNotFound.Wrap(msg.AssetId)
	}
	if rec.Status != types.AssetStatus_ASSET_STATUS_ACTIVE {
		return nil, types.ErrInvalidStatus.Wrapf("slash requires ACTIVE, got %s", rec.Status)
	}
	bond, _ := math.NewIntFromString(rec.Bond)
	if bond.IsPositive() {
		if err := ms.SlashBondToCommunityPool(ctx, bond); err != nil {
			return nil, err
		}
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	rec.Status = types.AssetStatus_ASSET_STATUS_SETTLED
	rec.SettledAt = sdkCtx.BlockTime()
	rec.Bond = math.ZeroInt().String()
	if err := ms.SetAsset(ctx, rec); err != nil {
		return nil, err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeBondSlashed,
		sdk.NewAttribute(types.AttributeKeyAssetID, rec.AssetId),
		sdk.NewAttribute(types.AttributeKeyAmount, bond.String()),
		sdk.NewAttribute(types.AttributeKeyReason, msg.Reason),
	))
	return &types.MsgSlashBondResponse{}, nil
}

func (ms msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if ms.GetAuthority() != msg.Authority {
		return nil, errorsmod.Wrapf(types.ErrInvalidSigner, "expected %s, got %s", ms.GetAuthority(), msg.Authority)
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	if err := ms.SetParams(ctx, msg.Params); err != nil {
		return nil, err
	}
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent(types.EventTypeParamsUpdated))
	return &types.MsgUpdateParamsResponse{}, nil
}
```

- [ ] **Step 4: Write `x/rwa/keeper/grpc_query.go`** (with a failing query test)

First add the query test to a new file `x/rwa/keeper/grpc_query_test.go`:

```go
package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestQueries(t *testing.T) {
	bank := keeper.NewMockBank()
	k, ctx := keeper.RWAKeeper(t, bank, keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	issuer := sdkAcc(t)
	require.NoError(t, k.SetAsset(ctx, types.AssetRecord{AssetId: "gold", Issuer: issuer.String(), Status: types.AssetStatus_ASSET_STATUS_DRAFT, Denom: "rwa/gold", Bond: "1", NotionalMinted: "0", AllowAll: false}))
	alice := sdkAcc(t)
	require.NoError(t, k.AddAllowlist(ctx, "gold", alice))

	// sdk.Context satisfies context.Context, so the query methods accept it directly.
	asset, err := k.Asset(ctx, &types.QueryAssetRequest{AssetId: "gold"})
	require.NoError(t, err)
	require.Equal(t, "rwa/gold", asset.Asset.Denom)

	_, err = k.Asset(ctx, &types.QueryAssetRequest{AssetId: "missing"})
	require.Error(t, err)

	byIssuer, err := k.AssetsByIssuer(ctx, &types.QueryAssetsByIssuerRequest{Issuer: issuer.String()})
	require.NoError(t, err)
	require.Len(t, byIssuer.Assets, 1)

	restr, err := k.Restrictions(ctx, &types.QueryRestrictionsRequest{AssetId: "gold"})
	require.NoError(t, err)
	require.False(t, restr.AllowAll)
	require.Contains(t, restr.Allowlist, alice.String())

	params, err := k.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, types.DefaultParams(), params.Params)
}
```

Then write `x/rwa/keeper/grpc_query.go`:

```go
package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/vertix-network/vertix/x/rwa/types"
)

var _ types.QueryServer = Keeper{}

func (k Keeper) Asset(goCtx context.Context, req *types.QueryAssetRequest) (*types.QueryAssetResponse, error) {
	if req == nil || req.AssetId == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	rec, found := k.GetAsset(goCtx, req.AssetId)
	if !found {
		return nil, status.Error(codes.NotFound, types.ErrAssetNotFound.Error())
	}
	return &types.QueryAssetResponse{Asset: rec}, nil
}

func (k Keeper) AssetsByIssuer(goCtx context.Context, req *types.QueryAssetsByIssuerRequest) (*types.QueryAssetsByIssuerResponse, error) {
	if req == nil || req.Issuer == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	issuer, err := sdk.AccAddressFromBech32(req.Issuer)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	adapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(goCtx))
	idxStore := prefix.NewStore(adapter, types.IssuerIndexPrefix(issuer))

	var assets []types.AssetRecord
	pageRes, err := query.Paginate(idxStore, req.Pagination, func(key, _ []byte) error {
		// key is "/{asset_id}" within the issuer prefix
		assetID := string(key)
		if len(assetID) > 0 && assetID[0] == '/' {
			assetID = assetID[1:]
		}
		if rec, found := k.GetAsset(goCtx, assetID); found {
			assets = append(assets, rec)
		}
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &types.QueryAssetsByIssuerResponse{Assets: assets, Pagination: pageRes}, nil
}

func (k Keeper) Restrictions(goCtx context.Context, req *types.QueryRestrictionsRequest) (*types.QueryRestrictionsResponse, error) {
	if req == nil || req.AssetId == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	rec, found := k.GetAsset(goCtx, req.AssetId)
	if !found {
		return nil, status.Error(codes.NotFound, types.ErrAssetNotFound.Error())
	}
	resp := &types.QueryRestrictionsResponse{AllowAll: rec.AllowAll}
	_ = k.IterateMembers(goCtx, types.AllowlistPrefix(req.AssetId), func(addr sdk.AccAddress) bool {
		resp.Allowlist = append(resp.Allowlist, addr.String())
		return true
	})
	_ = k.IterateMembers(goCtx, types.DenylistPrefix(req.AssetId), func(addr sdk.AccAddress) bool {
		resp.Denylist = append(resp.Denylist, addr.String())
		return true
	})
	return resp, nil
}

func (k Keeper) Params(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	params, err := k.GetParams(goCtx)
	if err != nil {
		return nil, err
	}
	return &types.QueryParamsResponse{Params: params}, nil
}
```

- [ ] **Step 5: Run tests — expect PASS**

Run: `go test ./x/rwa/keeper/ -run 'TestSlashBond|TestUpdateParams|TestQueries' -v`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add x/rwa/keeper/msg_server.go x/rwa/keeper/grpc_query.go x/rwa/keeper/grpc_query_test.go x/rwa/keeper/msg_server_test.go
git commit -m "feat(rwa): SlashBond + UpdateParams handlers and gRPC queries"
```

---

## Task 22: Genesis import/export

**Files:**
- Replace: `x/rwa/types/genesis.go`
- Replace: `x/rwa/keeper/genesis.go`
- Create: `x/rwa/keeper/genesis_test.go`

- [ ] **Step 1: Write the failing round-trip test**

Create `x/rwa/keeper/genesis_test.go`:

```go
package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestGenesisRoundTrip(t *testing.T) {
	k, ctx := keeper.RWAKeeper(t, keeper.NewMockBank(), keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())

	alice := sample.AccAddress()
	gs := types.GenesisState{
		Params: types.DefaultParams(),
		Assets: []types.AssetRecord{
			{AssetId: "gold", Issuer: sample.AccAddress(), Status: types.AssetStatus_ASSET_STATUS_DRAFT, OraclePair: "XAU:USD", Denom: "rwa/gold", Bond: "10000000000", NotionalMinted: "0", AllowAll: false},
		},
		Restrictions: []types.Restriction{
			{AssetId: "gold", Address: alice, IsDeny: false},
		},
	}

	k.InitGenesis(ctx, gs)
	exported := k.ExportGenesis(ctx)
	require.Equal(t, gs.Params, exported.Params)
	require.Len(t, exported.Assets, 1)
	require.Len(t, exported.Restrictions, 1)
	require.Equal(t, "gold", exported.Restrictions[0].AssetId)
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/rwa/keeper/ -run TestGenesisRoundTrip -v`

Expected: FAIL — `InitGenesis`/`ExportGenesis` differ.

- [ ] **Step 3: Write `x/rwa/types/genesis.go`**

```go
package types

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func DefaultGenesis() *GenesisState {
	return &GenesisState{Params: DefaultParams()}
}

func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}
	minBond, _ := gs.Params.MinIssuerBondInt()
	seen := make(map[string]struct{}, len(gs.Assets))
	for i, a := range gs.Assets {
		if err := ValidateAssetID(a.AssetId); err != nil {
			return fmt.Errorf("genesis asset[%d]: %w", i, err)
		}
		if _, ok := seen[a.AssetId]; ok {
			return fmt.Errorf("genesis asset[%d]: duplicate asset_id %q", i, a.AssetId)
		}
		seen[a.AssetId] = struct{}{}
		if _, err := sdk.AccAddressFromBech32(a.Issuer); err != nil {
			return fmt.Errorf("genesis asset[%d]: invalid issuer: %w", i, err)
		}
		if a.Status < AssetStatus_ASSET_STATUS_DRAFT || a.Status > AssetStatus_ASSET_STATUS_SETTLED {
			return fmt.Errorf("genesis asset[%d]: invalid status %s", i, a.Status)
		}
		if !pairRegex.MatchString(a.OraclePair) {
			return fmt.Errorf("genesis asset[%d]: oracle_pair %q is not BASE:QUOTE", i, a.OraclePair)
		}
		bond, ok := math.NewIntFromString(a.Bond)
		if !ok {
			return fmt.Errorf("genesis asset[%d]: invalid bond %q", i, a.Bond)
		}
		if a.Status == AssetStatus_ASSET_STATUS_ATTESTED || a.Status == AssetStatus_ASSET_STATUS_ACTIVE {
			if bond.LT(minBond) {
				return fmt.Errorf("genesis asset[%d]: bond %s < min_issuer_bond %s", i, bond, minBond)
			}
		}
		if _, ok := math.NewIntFromString(a.NotionalMinted); !ok {
			return fmt.Errorf("genesis asset[%d]: invalid notional_minted %q", i, a.NotionalMinted)
		}
	}
	for i, r := range gs.Restrictions {
		if _, ok := seen[r.AssetId]; !ok {
			return fmt.Errorf("genesis restriction[%d]: unknown asset_id %q", i, r.AssetId)
		}
		if _, err := sdk.AccAddressFromBech32(r.Address); err != nil {
			return fmt.Errorf("genesis restriction[%d]: invalid address: %w", i, err)
		}
	}
	return nil
}
```

`pairRegex` is the unexported regex defined in `x/rwa/types/msgs.go` (Task 7); it is reused here since both files are in package `types`.

- [ ] **Step 4: Write `x/rwa/keeper/genesis.go`**

```go
package keeper

import (
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/rwa/types"
)

func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) {
	if err := gs.Validate(); err != nil {
		panic(fmt.Errorf("invalid rwa genesis: %w", err))
	}
	if err := k.SetParams(ctx, gs.Params); err != nil {
		panic(err)
	}
	for _, a := range gs.Assets {
		if err := k.SetAsset(ctx, a); err != nil {
			panic(err)
		}
	}
	for _, r := range gs.Restrictions {
		addr, err := sdk.AccAddressFromBech32(r.Address)
		if err != nil {
			panic(err)
		}
		if r.IsDeny {
			if err := k.AddDenylist(ctx, r.AssetId, addr); err != nil {
				panic(err)
			}
		} else if err := k.AddAllowlist(ctx, r.AssetId, addr); err != nil {
			panic(err)
		}
	}
}

func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	params, err := k.GetParams(ctx)
	if err != nil {
		panic(err)
	}
	gs := &types.GenesisState{Params: params}

	_ = k.IterateAssets(ctx, func(rec types.AssetRecord) bool {
		gs.Assets = append(gs.Assets, rec)
		return true
	})
	sort.Slice(gs.Assets, func(i, j int) bool { return gs.Assets[i].AssetId < gs.Assets[j].AssetId })

	for _, a := range gs.Assets {
		assetID := a.AssetId
		_ = k.IterateMembers(ctx, types.AllowlistPrefix(assetID), func(addr sdk.AccAddress) bool {
			gs.Restrictions = append(gs.Restrictions, types.Restriction{AssetId: assetID, Address: addr.String(), IsDeny: false})
			return true
		})
		_ = k.IterateMembers(ctx, types.DenylistPrefix(assetID), func(addr sdk.AccAddress) bool {
			gs.Restrictions = append(gs.Restrictions, types.Restriction{AssetId: assetID, Address: addr.String(), IsDeny: true})
			return true
		})
	}
	return gs
}
```

- [ ] **Step 5: Run tests — expect PASS**

Run: `go test ./x/rwa/keeper/ -run TestGenesisRoundTrip -v`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add x/rwa/types/genesis.go x/rwa/keeper/genesis.go x/rwa/keeper/genesis_test.go
git commit -m "feat(rwa): genesis import/export for params, assets, restrictions"
```

---

## Task 23: Crisis invariants

**Files:**
- Create: `x/rwa/keeper/invariants.go`
- Create: `x/rwa/keeper/invariants_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/rwa/keeper/invariants_test.go`:

```go
package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
	rwakeeper "github.com/vertix-network/vertix/x/rwa/keeper"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestBondInvariantHolds(t *testing.T) {
	bank := keeper.NewMockBank()
	k, ctx := keeper.RWAKeeper(t, bank, keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())

	require.NoError(t, k.SetAsset(ctx, types.AssetRecord{AssetId: "gold", Issuer: sample.AccAddress(), Status: types.AssetStatus_ASSET_STATUS_ACTIVE, Denom: "rwa/gold", Bond: "5000", NotionalMinted: "0", AllowAll: true}))
	bank.SetBalance(k.ModuleAddress(), sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(5000))))

	_, broken := rwakeeper.BondInvariant(k)(ctx)
	require.False(t, broken)
}

func TestBondInvariantBreaks(t *testing.T) {
	bank := keeper.NewMockBank()
	k, ctx := keeper.RWAKeeper(t, bank, keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())

	require.NoError(t, k.SetAsset(ctx, types.AssetRecord{AssetId: "gold", Issuer: sample.AccAddress(), Status: types.AssetStatus_ASSET_STATUS_ACTIVE, Denom: "rwa/gold", Bond: "5000", NotionalMinted: "0", AllowAll: true}))
	// module account underfunded
	bank.SetBalance(k.ModuleAddress(), sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(1000))))

	_, broken := rwakeeper.BondInvariant(k)(ctx)
	require.True(t, broken)
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/rwa/keeper/ -run TestBondInvariant -v`

Expected: FAIL — `BondInvariant` not defined.

- [ ] **Step 3: Write `x/rwa/keeper/invariants.go`**

```go
package keeper

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/rwa/types"
)

// RegisterInvariants registers the rwa crisis invariants.
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "bonds", BondInvariant(k))
	ir.RegisterRoute(types.ModuleName, "denoms", DenomInvariant(k))
}

// BondInvariant: the module account uvtx balance equals the sum of bonds over
// all non-settled assets, and every ACTIVE asset has a positive bond.
func BondInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		sum := math.ZeroInt()
		broken := false
		_ = k.IterateAssets(ctx, func(rec types.AssetRecord) bool {
			if rec.Status == types.AssetStatus_ASSET_STATUS_SETTLED {
				return true
			}
			bond, _ := math.NewIntFromString(rec.Bond)
			if rec.Status == types.AssetStatus_ASSET_STATUS_ACTIVE && !bond.IsPositive() {
				broken = true
			}
			sum = sum.Add(bond)
			return true
		})
		// Spec invariant 2: the module account must hold at LEAST the sum of live
		// bonds (≥, not ==, since uvtx could in principle be sent to the module
		// account directly; fees never transit it).
		moduleBal := k.bankKeeper.GetBalance(ctx, k.ModuleAddress(), types.BondDenom).Amount
		if moduleBal.LT(sum) {
			broken = true
		}
		return sdk.FormatInvariant(types.ModuleName, "bonds",
			"module uvtx balance must be >= sum of non-settled bonds and every ACTIVE asset must be bonded"), broken
	}
}

// DenomInvariant: a pre-mint asset (DRAFT/ATTESTED) must have zero rwa/{id} supply.
func DenomInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		broken := false
		_ = k.IterateAssets(ctx, func(rec types.AssetRecord) bool {
			if rec.Status == types.AssetStatus_ASSET_STATUS_DRAFT || rec.Status == types.AssetStatus_ASSET_STATUS_ATTESTED {
				if !k.bankKeeper.GetSupply(ctx, rec.Denom).Amount.IsZero() {
					broken = true
				}
			}
			return true
		})
		return sdk.FormatInvariant(types.ModuleName, "denoms",
			"pre-mint assets must have zero factory-denom supply"), broken
	}
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./x/rwa/keeper/ -run TestBondInvariant -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/rwa/keeper/invariants.go x/rwa/keeper/invariants_test.go
git commit -m "feat(rwa): register bond and denom crisis invariants"
```

---

## Task 24: Module wiring (depinject deps, autocli, invariants, no-op EndBlock)

**Files:**
- Modify: `x/rwa/module/module.go`
- Replace: `x/rwa/module/autocli.go`

- [ ] **Step 1: Wire `ProvideModule` with the four dependencies + manual OracleKeeper input**

In `x/rwa/module/module.go`, update `ModuleInputs`, `ProvideModule`, `RegisterInvariants`, `InitGenesis`/`ExportGenesis` calls, and the no-op `EndBlock`. The scaffold gives `AccountKeeper`, `BankKeeper`; add `OracleKeeper` and `DistributionKeeper`:

```go
type ModuleInputs struct {
	depinject.In

	StoreService store.KVStoreService
	Cdc          codec.Codec
	Config       *modulev1.Module
	Logger       log.Logger

	AccountKeeper types.AccountKeeper
	BankKeeper    types.BankKeeper
	OracleKeeper  types.OracleKeeper
	DistrKeeper   types.DistributionKeeper
}

type ModuleOutputs struct {
	depinject.Out

	RWAKeeper keeper.Keeper
	Module    appmodule.AppModule
}

func ProvideModule(in ModuleInputs) ModuleOutputs {
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)
	if in.Config != nil && in.Config.Authority != "" {
		authority = authtypes.NewModuleAddressOrBech32Address(in.Config.Authority)
	}
	k := keeper.NewKeeper(
		in.Cdc,
		in.StoreService,
		in.Logger,
		authority.String(),
		in.AccountKeeper,
		in.BankKeeper,
		in.OracleKeeper,
		in.DistrKeeper,
	)
	m := NewAppModule(in.Cdc, k)
	return ModuleOutputs{RWAKeeper: k, Module: m}
}
```

> `OracleKeeper` is supplied by interface binding: the oracle module outputs the concrete `oraclekeeper.Keeper`, which satisfies `rwatypes.OracleKeeper` (`GetPrice`). This is the same mechanism the oracle module uses to receive `StakingKeeper`/`SlashingKeeper`. `DistrKeeper` is supplied by the `distrkeeper.Keeper` (it implements `FundCommunityPool`). The `--dep bank,distribution` scaffold already provides these in the container.

- [ ] **Step 2: Wire invariants + genesis + no-op EndBlock in `module.go`**

```go
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
	keeper.RegisterInvariants(ir, am.keeper)
}

func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, gs json.RawMessage) {
	var genState types.GenesisState
	cdc.MustUnmarshalJSON(gs, &genState)
	am.keeper.InitGenesis(ctx, genState)
}

func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(am.keeper.ExportGenesis(ctx))
}

// EndBlock is intentionally a no-op: x/rwa has no per-block work (spec D11).
// It exists only to hold the canonical oracle → rwa → fees position in the
// module-manager EndBlock order.
func (am AppModule) EndBlock(_ context.Context) error { return nil }
```

Keep the `_ appmodule.HasEndBlocker = (*AppModule)(nil)` assertion the scaffold added (the no-op satisfies it). Ensure `RegisterServices` registers both servers (scaffold already does: `types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(am.keeper))` and `types.RegisterQueryServer(cfg.QueryServer(), am.keeper)`).

- [ ] **Step 3: Replace `x/rwa/module/autocli.go`**

```go
package rwa

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	"github.com/vertix-network/vertix/x/rwa/types"
)

func (am AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: types.Query_serviceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{RpcMethod: "Asset", Use: "asset [asset-id]", Short: "Query an asset record", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "asset_id"}}},
				{RpcMethod: "AssetsByIssuer", Use: "assets-by-issuer [issuer]", Short: "Query assets by issuer", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "issuer"}}},
				{RpcMethod: "Restrictions", Use: "restrictions [asset-id]", Short: "Query transfer restrictions", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "asset_id"}}},
				{RpcMethod: "Params", Use: "params", Short: "Query module params"},
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service:              types.Msg_serviceDesc.ServiceName,
			EnhanceCustomCommand: true,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{RpcMethod: "RegisterAsset", Use: "register-asset [asset-id] [name] [oracle-pair] [bond]", Short: "Register a new RWA asset", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "asset_id"}, {ProtoField: "name"}, {ProtoField: "oracle_pair"}, {ProtoField: "bond"}}},
				{RpcMethod: "AttestAsset", Use: "attest-asset [asset-id]", Short: "Attest an asset against its oracle price", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "asset_id"}}},
				{RpcMethod: "MintRWA", Use: "mint-rwa [asset-id] [notional]", Short: "Mint rwa/{id} tokens", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "asset_id"}, {ProtoField: "notional"}}},
				{RpcMethod: "TransferRWA", Use: "transfer-rwa [recipient] [asset-id] [amount]", Short: "Transfer rwa/{id} tokens", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "recipient"}, {ProtoField: "asset_id"}, {ProtoField: "amount"}}},
				{RpcMethod: "SettleRWA", Use: "settle-rwa [asset-id]", Short: "Settle an asset and reclaim bond", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "asset_id"}}},
				{RpcMethod: "UpdateRestrictions", Skip: true}, // multi-field; use generated command
				{RpcMethod: "SlashBond", Skip: true},          // gov-gated
				{RpcMethod: "UpdateParams", Skip: true},       // gov-gated
			},
		},
	}
}
```

`UpdateRestrictions` is skipped from the positional CLI (its repeated address fields don't map to positional args cleanly); it remains reachable via the generated `--from`-flagged command since `EnhanceCustomCommand` is true.

- [ ] **Step 4: Verify build**

Run: `go build ./...`

Expected: success (rwa packages and app build).

- [ ] **Step 5: Run the full rwa test suite**

Run: `go test ./x/rwa/... -count=1`

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add x/rwa/module/
git commit -m "feat(rwa): wire depinject deps, invariants, autocli, no-op EndBlock"
```

---

## Task 25: App wiring (order, maccPerms, send-restriction) + app tests

**Files:**
- Modify: `app/app_config.go`
- Modify: `app/app.go`
- Modify: `app/app_test.go`

- [ ] **Step 1: Adjust `app/app_config.go` ordering + permissions**

Confirm Ignite inserted `rwamoduletypes` import and a `ModuleConfig`. Then:

- `genesisModuleOrder`: place `rwamoduletypes.ModuleName` **after `oraclemoduletypes.ModuleName`, before `feesmoduletypes.ModuleName`**. Since `feesmoduletypes` is the last entry before the scaffolding marker, the result is `... oracle, slashing, ... consensus, rwa, fees`. To honor `staking → oracle → rwa → fees`, move `rwamoduletypes.ModuleName` to sit directly after `oraclemoduletypes.ModuleName`:

```go
		stakingtypes.ModuleName,
		oraclemoduletypes.ModuleName,
		rwamoduletypes.ModuleName,
		slashingtypes.ModuleName,
		// ... unchanged ...
		feesmoduletypes.ModuleName,
		// this line is used by starport scaffolding # stargate/app/initGenesis
```

- `endBlockers`: place `rwamoduletypes.ModuleName` **between** oracle and fees:

```go
		// chain modules
		oraclemoduletypes.ModuleName,
		rwamoduletypes.ModuleName,
		feesmoduletypes.ModuleName,
		// this line is used by starport scaffolding # stargate/app/endBlockers
```

- `moduleAccPerms`: ensure the rwa account has **Minter + Burner**:

```go
		{Account: rwamoduletypes.ModuleName, Permissions: []string{authtypes.Minter, authtypes.Burner}},
```

- A `ModuleConfig` for rwa exists at `# stargate/app/moduleConfig` (Ignite adds it; verify it references `rwamodulev1.Module{}`).
- No `beginBlockers` / `preBlockers` entry.

Use the exact import alias Ignite generated (likely `rwamoduletypes "github.com/vertix-network/vertix/x/rwa/types"` and `rwamodulev1 "github.com/vertix-network/vertix/api/vertix/rwa/module"`).

- [ ] **Step 2: Register the send-restriction in `app/app.go`**

After the `depinject.Inject(...)` block completes (so `app.BankKeeper` and `app.RWAKeeper` are populated) and before `app.App = appBuilder.Build(...)` — or immediately after Build, but before `app.Load` — add:

```go
	// Enforce rwa/* transfer restrictions on every bank SendCoins path
	// (MsgSend, MsgMultiSend, authz, IBC escrow). Must run on the concrete
	// bank keeper after depinject populates both keepers (spec D5).
	app.BankKeeper.AppendSendRestriction(app.RWAKeeper.SendRestriction)
```

Place this right after the `if err := depinject.Inject(...)` block (after the `panic(err)` close brace, before `baseAppOptions = append(...)`). Confirm `app.RWAKeeper` field and `&app.RWAKeeper` inject target were added by Ignite at the `# stargate/app/keeperDeclaration` and `# stargate/app/keeperDefinition` markers; if not, add:

```go
	// in the App struct, near OracleKeeper/FeesKeeper:
	RWAKeeper rwamodulekeeper.Keeper
```
```go
	// in the depinject.Inject target list, after &app.FeesKeeper:
		&app.RWAKeeper,
```

Add the import `rwamodulekeeper "github.com/vertix-network/vertix/x/rwa/keeper"` if Ignite didn't.

- [ ] **Step 3: Add app-level tests**

Append to `app/app_test.go` (add `rwatypes "github.com/vertix-network/vertix/x/rwa/types"` to imports):

```go
func TestRWAModuleWired(t *testing.T) {
	a := newTestApp(t)
	_, ok := a.ModuleManager.Modules[rwatypes.ModuleName]
	require.True(t, ok, "x/rwa must be wired")
	require.NotNil(t, a.RWAKeeper)
}

func TestRWAModuleAccountHasMinterBurner(t *testing.T) {
	newTestApp(t)
	perms, ok := app.GetMaccPerms()[rwatypes.ModuleName]
	require.True(t, ok, "x/rwa module account must be configured")
	require.Contains(t, perms, authtypes.Minter)
	require.Contains(t, perms, authtypes.Burner)
}

func TestRWAGenesisRoundTrip(t *testing.T) {
	a := newTestApp(t)
	genState := a.DefaultGenesis()
	require.Contains(t, genState, rwatypes.ModuleName)
}
```

- [ ] **Step 4: Run app tests**

Run: `go test ./app/ -run 'TestRWA|TestNoMint|TestOracle|TestFees|TestModulesWired' -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/
git commit -m "feat(app): wire x/rwa (order, Minter+Burner, send-restriction)"
```

---

## Task 26: Doc-syncs

**Files:**
- Modify: `docs/technical-design.md` (§3.3, §3.5, §3.6)
- Modify: `docs/full-design-spec.md` (Appendix C)

- [ ] **Step 1: Update `docs/technical-design.md` §3.6**

Replace the §3.6 transfer-restriction-enforcement paragraph (the "ante decorator" wording) with:

```
Plain `x/bank.MsgSend` (and `MsgMultiSend`, authz-wrapped sends, and IBC transfer
escrow) of `rwa/*` denoms is governed by a bank **SendRestrictionFn** registered
via `bankKeeper.AppendSendRestriction(k.SendRestriction)` in `app.go`. Because the
function runs inside `BankKeeper.SendCoins` — the single chokepoint every transfer
path funnels through — issuers cannot be bypassed. The function defers to the same
`CheckTransferAllowed` predicate `MsgTransferRWA` uses, and exempts the `x/rwa`
module account so mint/burn/escrow legs are never blocked. (Refined from the
original "ante decorator" design — Phase 3 spec D5.)
```

- [ ] **Step 2: Update `docs/technical-design.md` §3.3**

Expand the `AssetRecord` proto sketch to include `denom`, `bond`, `notional_minted`, `allow_all`, `attested_price`, `attested_at`, `created_at`, `settled_at` (mirror §5.1 of the Phase 3 spec), and add `MsgSlashBond` to the `tx.proto` message list. Add a note under §3.5: "Notional is declared in `uvtx`; minting is 1:1 (`rwa/{id}` units == notional). The oracle is read only at attestation, not at mint — the fee is a deterministic function of the message (Phase 3 spec D2)."

- [ ] **Step 3: Update `docs/full-design-spec.md` Appendix C**

Mark the two Phase 3 open questions resolved:

```
- **Transfer-restriction model** (Phase 3): RESOLVED — separate keyed store for
  allow/deny membership; enforced via a bank SendRestrictionFn. See
  `specs/2026-05-29-phase-3-rwa-module-design.md` D4/D5.
- **Dispute resolution** (Phase 3): RESOLVED — governance-only `MsgSlashBond`
  force-settles the asset and routes the bond to the community pool. See
  `specs/2026-05-29-phase-3-rwa-module-design.md` D7.
```

- [ ] **Step 4: Commit**

```bash
git add docs/technical-design.md docs/full-design-spec.md
git commit -m "docs(rwa): sync §3.3/§3.5/§3.6 + resolve Appendix C D4/D7"
```

---

## Task 27: Acceptance gate integration test

**Files:**
- Create: `x/rwa/keeper/acceptance_test.go`

- [ ] **Step 1: Write the full-lifecycle acceptance test mapping spec §11**

Create `x/rwa/keeper/acceptance_test.go`:

```go
package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	rwakeeper "github.com/vertix-network/vertix/x/rwa/keeper"
	"github.com/vertix-network/vertix/x/rwa/types"
)

// Spec §11: full lifecycle register → attest → mint → transfer → settle, with
// oracle gating, bond escrow/return, restriction enforcement, and fee routing.
func TestAcceptance_FullLifecycle(t *testing.T) {
	bank := keeper.NewMockBank()
	distr := keeper.NewMockDistribution()
	k, ctx := keeper.RWAKeeper(t, bank, keeper.MockOracle{Price: math.LegacyNewDec(1900)}, distr)
	srv := rwakeeper.NewMsgServerImpl(k)

	issuer := sdkAcc(t)
	bank.SetBalance(issuer, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(20_000_000_000))))

	// register (bond locked)
	_, err := srv.RegisterAsset(ctx, &types.MsgRegisterAsset{Issuer: issuer.String(), AssetId: "gold-01", Name: "Gold", OraclePair: "XAU:USD", Bond: "10000000000"})
	require.NoError(t, err)
	require.Equal(t, math.NewInt(10_000_000_000), bank.GetBalance(ctx, k.ModuleAddress(), "uvtx").Amount)

	// attest (oracle gate + snapshot)
	_, err = srv.AttestAsset(ctx, &types.MsgAttestAsset{Issuer: issuer.String(), AssetId: "gold-01"})
	require.NoError(t, err)

	// mint (fee to collector)
	_, err = srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000000"})
	require.NoError(t, err)
	require.Equal(t, math.NewInt(1000), bank.GetBalance(ctx, authtypes.NewModuleAddress(authtypes.FeeCollectorName), "uvtx").Amount)

	// transfer to bob then back (so issuer holds all units before settle)
	bob := sdkAcc(t)
	_, err = srv.TransferRWA(ctx, &types.MsgTransferRWA{Sender: issuer.String(), Recipient: bob.String(), AssetId: "gold-01", Amount: "400000"})
	require.NoError(t, err)
	_, err = srv.TransferRWA(ctx, &types.MsgTransferRWA{Sender: bob.String(), Recipient: issuer.String(), AssetId: "gold-01", Amount: "400000"})
	require.NoError(t, err)

	// settle (burn, bond returned, settle fee charged)
	_, err = srv.SettleRWA(ctx, &types.MsgSettleRWA{Issuer: issuer.String(), AssetId: "gold-01"})
	require.NoError(t, err)
	rec, _ := k.GetAsset(ctx, "gold-01")
	require.Equal(t, types.AssetStatus_ASSET_STATUS_SETTLED, rec.Status)
	require.True(t, bank.GetBalance(ctx, k.ModuleAddress(), "uvtx").IsZero()) // bond released

	// invariants hold at the end
	_, broken := rwakeeper.BondInvariant(k)(ctx)
	require.False(t, broken)
	_, broken = rwakeeper.DenomInvariant(k)(ctx)
	require.False(t, broken)
}

// Spec §11 / D5: bank send-restriction blocks restricted rwa/* sends (bypass test).
func TestAcceptance_SendRestrictionBlocksBypass(t *testing.T) {
	bank := keeper.NewMockBank()
	k, ctx := keeper.RWAKeeper(t, bank, keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	srv := rwakeeper.NewMsgServerImpl(k)

	issuer := sdkAcc(t)
	bank.SetBalance(issuer, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(20_000_000_000))))
	_, err := srv.RegisterAsset(ctx, &types.MsgRegisterAsset{Issuer: issuer.String(), AssetId: "gold-01", Name: "G", OraclePair: "XAU:USD", Bond: "10000000000"})
	require.NoError(t, err)
	_, err = srv.AttestAsset(ctx, &types.MsgAttestAsset{Issuer: issuer.String(), AssetId: "gold-01"})
	require.NoError(t, err)
	_, err = srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000"})
	require.NoError(t, err)
	_, err = srv.UpdateRestrictions(ctx, &types.MsgUpdateRestrictions{Issuer: issuer.String(), AssetId: "gold-01", AllowAll: false})
	require.NoError(t, err)

	// a raw bank send (simulated via the registered restriction) is blocked
	bob := sdkAcc(t)
	_, err = k.SendRestriction(ctx, issuer, bob, sdk.NewCoins(sdk.NewCoin("rwa/gold-01", math.NewInt(1))))
	require.ErrorIs(t, err, types.ErrTransferRestricted)
}
```

- [ ] **Step 2: Run the full rwa suite**

Run: `go test ./x/rwa/... -race -count=1 -v`

Expected: all PASS

- [ ] **Step 3: Commit**

```bash
git add x/rwa/keeper/acceptance_test.go
git commit -m "test(rwa): Phase 3 acceptance gate full-lifecycle + bypass tests"
```

---

## Task 28: Full CI verification

**Files:** none (verification only)

- [ ] **Step 1: Lint**

Run: `make lint`

Expected: no findings. Common fixes: unused imports, and the `_ = ...` keep-import guards used in a few tests (e.g. `_ = rwakeeper.BuildDenom`, `_ = authtypes.FeeCollectorName`) — remove a guard if its package is already used elsewhere in the file so `golangci-lint` does not flag it as redundant.

- [ ] **Step 2: Test**

Run: `make test`

Expected: all packages pass with `-race -count=1`.

- [ ] **Step 3: Build**

Run: `make build`

Expected: `build/vertixd` produced.

- [ ] **Step 4: Proto checks**

Run:

```bash
cd proto && buf lint && buf breaking --against '.git#branch=main'
```

Expected: pass (first rwa proto; breaking against main passes since main has no rwa proto).

- [ ] **Step 5: Genesis validate**

Run:

```bash
./build/vertixd genesis validate-genesis
```

Expected: valid genesis (rwa params present: `min_issuer_bond: "10000000000"`, fee rates `0.001`).

- [ ] **Step 6: Manual CLI smoke (optional)**

```bash
./build/vertixd q rwa params
```

Expected: returns the default params — not a crash.

- [ ] **Step 7: Final commit if any fixes needed**

```bash
git commit -am "chore(rwa): Phase 3 CI green — lint test build"
```

---

## Spec Coverage Checklist (self-review)

| Spec requirement | Task |
|---|---|
| D0 Ignite scaffold + adapt | Task 1 |
| D1 Issuer slug asset_id; denom = rwa/{slug} | Tasks 2, 3, 7, 11, 15 |
| D2 Notional in uvtx; mint 1:1; deterministic fee; no oracle at mint | Tasks 14, 17 |
| D3 Oracle gate at attest + price/timestamp snapshot | Task 16 |
| D4 Separate keyed restriction store; O(1) checks | Tasks 3, 12 |
| D5 Bank SendRestrictionFn (MsgSend/authz/IBC) | Tasks 13, 25 |
| D6 Issuer bond ≥ MinIssuerBond, per-asset | Tasks 6, 14, 15 |
| D7 Governance-only SlashBond → community pool, force-settle | Tasks 14, 21 |
| D8 Clean settle issuer-only | Task 20 |
| D9 Fee shortfall fails atomically; bond untouched | Tasks 14, 17, 20 |
| D10 Default allow_all = true | Task 15 |
| D11 No EndBlock work; positional order only | Tasks 24, 25 |
| Proto surface (8 msgs, 4 queries, AssetRecord/AssetStatus/RWAParams, genesis) | Task 2 |
| Params + Validate | Task 6 |
| All ValidateBasic + slug regex | Task 7 |
| Expected keepers (Bank/Oracle/Account/Distribution) | Task 8 |
| Lifecycle handlers (register/attest/mint/transfer/settle) | Tasks 15–20 |
| UpdateRestrictions | Task 19 |
| SlashBond + UpdateParams | Task 21 |
| Queries (asset/assets-by-issuer/restrictions/params) + autocli | Tasks 21, 24 |
| Genesis import/export (params/assets/restrictions) | Task 22 |
| Crisis invariants (bond + denom) | Task 23 |
| App wiring (order, Minter+Burner, AppendSendRestriction) + app tests | Task 25 |
| Doc-syncs §3.3/§3.5/§3.6 + Appendix C D4/D7 | Task 26 |
| Acceptance gate §11 (full lifecycle + bypass) | Tasks 27–28 |
| Eight events emitted | Tasks 5, 15–21 |

**Cross-phase notes:** `x/rwa` consumes the Phase 1 `OracleKeeper` (`GetPrice`) by depinject interface binding (Task 24) and routes fees to `auth.FeeCollectorName` for Phase 2's `x/fees` to sweep (Tasks 14, 17, 20). The send-restriction (Task 13/25) is the mechanism Phase 5 relies on to preserve `rwa/*` restriction semantics over ICS-20.

**Scoping note on the app-level fee-sweep gate (spec §11):** the spec's gate row "Fees collected and swept by `x/fees`" has two halves. The "collected" half (mint/settle route `floor(notional × rate)` uvtx into `FeeCollectorName`) is verified at keeper level in Tasks 17, 20, and 27. The "swept" half is Phase 2's already-tested `EndBlocker` behavior (`x/fees` Task 12/18) — re-driving it through a full app-level `MintRWA` + `EndBlock` integration test is deliberately descoped here because it would require standing up bonded validators, feeders, and a real oracle aggregation just to reach `ATTESTED`. If a true cross-module app-level test is desired later, add it as a Phase 6 (devnet) end-to-end scenario, where the full validator/oracle stack already exists.
