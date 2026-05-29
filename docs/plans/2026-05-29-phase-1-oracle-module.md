# Phase 1 — `x/oracle` Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a working `x/oracle` module with feeder delegation, quorum-gated stake-weighted median aggregation, TWAP histories, tumbling miss + unweighted-median outlier slashing, and the `OracleKeeper` interface (`GetPrice`, `GetTWAP`) with staleness guarantees — satisfying the Phase 1 acceptance gate.

**Architecture:** Ignite scaffolds the module skeleton and depinject wiring; we replace the generated protos with the approved Phase 1 surface, then build in vertical slices (types → store → messages → aggregation math → EndBlock without slashing → slashing → queries → genesis → app wiring). Slashing is last (spec D9). External deps are interface-typed (`StakingKeeper`, `SlashingKeeper`) with mocks in `testutil/keeper`.

**Tech Stack:** Go 1.25+ (repo toolchain), Cosmos SDK v0.50.14, CometBFT v0.38.x, Ignite CLI v28, buf v1.30+, golangci-lint v1.57+.

**Reference spec:** [`docs/specs/2026-05-29-phase-1-oracle-module-design.md`](../specs/2026-05-29-phase-1-oracle-module-design.md). Where this plan and the spec disagree, the spec wins.

**Revised 2026-05-29** after implementation-plan review: quorum denominator `GetLastTotalPower` (C1b), length-prefixed feed keys, scaffold-overwrite notes, compile-safe snippets, TWAP leading-interval clamp, EndBlock phase order, depinject wiring task.

**Conventions:** Run commands from repo root `/home/nam-nguyen/my-projects/vertix-projects/vertix`. Commit messages follow Conventional Commits with scope (`docs/coding-standards.md` §7.1).

---

## File Map

| File | Responsibility |
|---|---|
| `proto/vertix/oracle/v1/*.proto` | Wire format: types, tx, query, genesis |
| `x/oracle/types/keys.go` | Store prefixes `0x01`–`0x08`, key builders |
| `x/oracle/types/params.go` | `DefaultParams()`, `Validate()` |
| `x/oracle/types/errors.go` | Registered sentinel errors |
| `x/oracle/types/events.go` | Event type + attribute constants |
| `x/oracle/types/msgs.go` | `ValidateBasic` for all Msgs |
| `x/oracle/types/expected_keepers.go` | `StakingKeeper`, `SlashingKeeper` interfaces |
| `x/oracle/types/genesis.go` | `DefaultGenesis()`, genesis validation helpers |
| `x/oracle/keeper/keeper.go` | Keeper struct, params + aggregated price + feed store ops |
| `x/oracle/keeper/feeder.go` | Feeder delegation mapping (`0x07`/`0x08`) |
| `x/oracle/keeper/aggregate.go` | `WeightedMedian`, `UnweightedMedian`, quorum check |
| `x/oracle/keeper/msg_server.go` | `SetFeeder`, `SubmitFeed`, `UpdateParams` |
| `x/oracle/keeper/grpc_query.go` | Price, Twap, Params, MissCounter, Feeder queries |
| `x/oracle/keeper/abci.go` | `EndBlocker` orchestration |
| `x/oracle/keeper/genesis.go` | `InitGenesis`, `ExportGenesis` |
| `x/oracle/module/module.go` | AppModule, EndBlock registration |
| `x/oracle/module/depinject.go` | ProvideModule, keeper construction |
| `x/oracle/module/autocli.go` | CLI query/tx wiring |
| `testutil/keeper/oracle.go` | In-memory keeper fixture + mock staking/slashing |
| `app/app_config.go` | ModuleConfig, endBlockers, genesisModuleOrder |
| `app/app.go` | OracleKeeper inject + App struct field |
| `app/app_test.go` | Module wired, genesis round-trip for oracle |

---

## Task 1: Scaffold the oracle module

**Files:**
- Create: `x/oracle/**`, `proto/vertix/oracle/v1/**` (Ignite-generated boilerplate)
- Modify: `app/app.go`, `app/app_config.go` (Ignite inserts at `# stargate/app/...` markers)

- [ ] **Step 1: Scaffold with dependencies**

Run:

```bash
cd /home/nam-nguyen/my-projects/vertix-projects/vertix
ignite scaffold module oracle --dep bank,staking,slashing -y
```

Expected: creates `x/oracle/`, `proto/vertix/oracle/v1/`, and updates `app/app.go` + `app/app_config.go`.

- [ ] **Step 2: Verify scaffold compiles**

Run:

```bash
go build ./...
```

Expected: success (scaffold boilerplate builds).

- [ ] **Step 3: Note scaffold files to replace later**

Ignite generates stubs that **conflict** with Phase 1 protos after Task 2. Do not treat these as final — overwrite or delete in Task 2:

- `x/oracle/types/params.go` (scaffold `Params` / `DefaultParams`)
- `x/oracle/types/genesis.go` (scaffold `GenesisState` / `Validate`)
- `x/oracle/types/expected_keepers.go` (stub keepers)
- `x/oracle/keeper/msg_server.go`, `grpc_query.go` (placeholder handlers)
- `x/oracle/module/autocli.go` (update in Task 21)

- [ ] **Step 4: Commit**

```bash
git add x/oracle/ proto/vertix/oracle/ app/app.go app/app_config.go go.mod go.sum
git commit -m "$(cat <<'EOF'
feat(oracle): scaffold x/oracle module via ignite

Depinject wiring for bank, staking, slashing. Refs:
docs/specs/2026-05-29-phase-1-oracle-module-design.md
EOF
)"
```

---

## Task 2: Replace protobuf definitions and regenerate

**Files:**
- Modify: `proto/vertix/oracle/v1/types.proto`
- Modify: `proto/vertix/oracle/v1/tx.proto`
- Modify: `proto/vertix/oracle/v1/query.proto`
- Modify: `proto/vertix/oracle/v1/genesis.proto`
- Delete: any Ignite-default messages that conflict (e.g. scaffold `Params` message in `types.proto` if present)
- Replace: scaffold `x/oracle/types/params.go`, `x/oracle/types/genesis.go` (removed in Task 6 / Task 22 — use hand-written versions)
- Create: generated `x/oracle/types/*.pb.go` via `make proto-gen`

- [ ] **Step 1: Write `proto/vertix/oracle/v1/types.proto`**

Replace file contents with:

```protobuf
syntax = "proto3";
package vertix.oracle.v1;

import "gogoproto/gogo.proto";
import "cosmos_proto/cosmos.proto";
import "google/protobuf/timestamp.proto";

option go_package = "github.com/vertix-network/vertix/x/oracle/types";

message OracleFeed {
  string validator    = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string pair         = 2;
  string price        = 3 [(cosmos_proto.scalar) = "cosmos.Dec"];
  int64  block_height = 4;
}

message AggregatedPrice {
  string                    pair         = 1;
  string                    price        = 2 [(cosmos_proto.scalar) = "cosmos.Dec"];
  int64                     block_height = 3;
  google.protobuf.Timestamp block_time   = 4 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
}

message TWAPEntry {
  string                    pair       = 1;
  string                    price      = 2 [(cosmos_proto.scalar) = "cosmos.Dec"];
  google.protobuf.Timestamp block_time = 3 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
}

message OracleParams {
  int64           vote_window        = 1;
  string          miss_threshold     = 2 [(cosmos_proto.scalar) = "cosmos.Dec"];
  string          miss_slash_rate     = 3 [(cosmos_proto.scalar) = "cosmos.Dec"];
  string          outlier_slash_rate  = 4 [(cosmos_proto.scalar) = "cosmos.Dec"];
  string          outlier_threshold   = 5 [(cosmos_proto.scalar) = "cosmos.Dec"];
  int64           miss_window_size    = 6;
  string          quorum_fraction     = 7 [(cosmos_proto.scalar) = "cosmos.Dec"];
  int64           max_price_age       = 8;
  repeated string accept_list         = 9;
}
```

- [ ] **Step 2: Write `proto/vertix/oracle/v1/tx.proto`**

```protobuf
syntax = "proto3";
package vertix.oracle.v1;

import "amino/amino.proto";
import "cosmos/msg/v1/msg.proto";
import "cosmos_proto/cosmos.proto";
import "gogoproto/gogo.proto";
import "vertix/oracle/v1/types.proto";

option go_package = "github.com/vertix-network/vertix/x/oracle/types";

service Msg {
  option (cosmos.msg.v1.service) = true;
  rpc SetFeeder(MsgSetFeeder)       returns (MsgSetFeederResponse);
  rpc SubmitFeed(MsgSubmitFeed)     returns (MsgSubmitFeedResponse);
  rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);
}

message MsgSetFeeder {
  option (cosmos.msg.v1.signer) = "validator";
  option (amino.name)           = "vertix/x/oracle/MsgSetFeeder";

  string validator = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string feeder    = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
}

message MsgSetFeederResponse {}

message MsgSubmitFeed {
  option (cosmos.msg.v1.signer) = "feeder";
  option (amino.name)           = "vertix/x/oracle/MsgSubmitFeed";

  string feeder    = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string validator = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string pair      = 3;
  string price     = 4 [(cosmos_proto.scalar) = "cosmos.Dec"];
}

message MsgSubmitFeedResponse {}

message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";
  option (amino.name)           = "vertix/x/oracle/MsgUpdateParams";

  string       authority = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  OracleParams params    = 2 [(gogoproto.nullable) = false];
}

message MsgUpdateParamsResponse {}
```

- [ ] **Step 3: Write `proto/vertix/oracle/v1/query.proto`**

```protobuf
syntax = "proto3";
package vertix.oracle.v1;

import "gogoproto/gogo.proto";
import "google/api/annotations.proto";
import "vertix/oracle/v1/types.proto";

option go_package = "github.com/vertix-network/vertix/x/oracle/types";

service Query {
  rpc Price(QueryPriceRequest)             returns (QueryPriceResponse) {
    option (google.api.http).get = "/vertix/oracle/v1/price/{pair}";
  }
  rpc Twap(QueryTwapRequest)               returns (QueryTwapResponse) {
    option (google.api.http).get = "/vertix/oracle/v1/twap/{pair}/{window_seconds}";
  }
  rpc Params(QueryParamsRequest)           returns (QueryParamsResponse) {
    option (google.api.http).get = "/vertix/oracle/v1/params";
  }
  rpc MissCounter(QueryMissCounterRequest) returns (QueryMissCounterResponse) {
    option (google.api.http).get = "/vertix/oracle/v1/miss_counter/{validator}";
  }
  rpc Feeder(QueryFeederRequest)           returns (QueryFeederResponse) {
    option (google.api.http).get = "/vertix/oracle/v1/feeder/{validator}";
  }
}

message QueryPriceRequest {
  string pair = 1;
}

message QueryPriceResponse {
  AggregatedPrice price = 1 [(gogoproto.nullable) = false];
}

message QueryTwapRequest {
  string pair            = 1;
  uint64 window_seconds  = 2;
}

message QueryTwapResponse {
  string price = 1 [(gogoproto.customtype) = "cosmossdk.io/math.LegacyDec", (gogoproto.nullable) = false];
}

message QueryParamsRequest {}

message QueryParamsResponse {
  OracleParams params = 1 [(gogoproto.nullable) = false];
}

message QueryMissCounterRequest {
  string validator = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
}

message QueryMissCounterResponse {
  int64 misses        = 1;
  int64 total_windows = 2;
}

message QueryFeederRequest {
  string validator = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
}

message QueryFeederResponse {
  string feeder = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
}
```

Add `import "cosmos_proto/cosmos.proto";` at the top of `query.proto` (needed for address scalars on MissCounter/Feeder requests).

- [ ] **Step 4: Write `proto/vertix/oracle/v1/genesis.proto`**

```protobuf
syntax = "proto3";
package vertix.oracle.v1;

import "gogoproto/gogo.proto";
import "cosmos_proto/cosmos.proto";
import "vertix/oracle/v1/types.proto";

option go_package = "github.com/vertix-network/vertix/x/oracle/types";

message FeederDelegation {
  string validator = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string feeder    = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
}

message GenesisState {
  OracleParams             params  = 1 [(gogoproto.nullable) = false];
  repeated AggregatedPrice  prices  = 2 [(gogoproto.nullable) = false];
  repeated FeederDelegation feeders = 3 [(gogoproto.nullable) = false];
}
```

- [ ] **Step 5: Regenerate Go from proto**

Run:

```bash
make proto-gen
```

Expected: regenerates `x/oracle/types/*.pb.go`, `x/oracle/types/query.pb.gw.go`, etc.

- [ ] **Step 6: Run buf lint**

Run:

```bash
cd proto && buf lint
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add proto/vertix/oracle/ x/oracle/types/
git commit -m "$(cat <<'EOF'
feat(oracle): define Phase 1 protobuf surface

MsgSetFeeder, MsgSubmitFeed, MsgUpdateParams, queries, genesis.
Refs: docs/specs/2026-05-29-phase-1-oracle-module-design.md §5
EOF
)"
```

---

## Task 3: Store keys and module constants

**Files:**
- Replace: `x/oracle/types/keys.go` (overwrite scaffold if present)
- Create: `x/oracle/types/keys_test.go`
- Modify: `x/oracle/types/codec.go` (register msgs if scaffold left placeholders)

- [ ] **Step 1: Write the failing test**

Create `x/oracle/types/keys_test.go`:

```go
package types_test

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestFeedKeyRoundTrip(t *testing.T) {
	val := sdk.ValAddress([]byte("validator123456789012345678901234"))
	pair := "BTC:USD"
	key := types.FeedKey(val, pair)
	val2, pair2, err := types.ParseFeedKey(key)
	require.NoError(t, err)
	require.Equal(t, val, val2)
	require.Equal(t, pair, pair2)
}

func TestTWAPKeyOrdering(t *testing.T) {
	pair := "ETH:USD"
	t1 := time.Unix(100, 0)
	t2 := time.Unix(200, 0)
	require.True(t, string(types.TWAPKey(pair, t1)) < string(types.TWAPKey(pair, t2)))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./x/oracle/types/ -run 'TestFeedKeyRoundTrip|TestTWAPKeyOrdering' -v`

Expected: FAIL — `FeedKey` not defined.

- [ ] **Step 3: Write `x/oracle/types/keys.go`**

```go
package types

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/cosmos/cosmos-sdk/types/address"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	ModuleName = "oracle"
	StoreKey   = ModuleName
)

var (
	KeyPrefixFeed            = []byte{0x01}
	KeyPrefixAggregatedPrice = []byte{0x02}
	KeyPrefixTWAP            = []byte{0x03}
	KeyPrefixMissCounter     = []byte{0x04}
	KeyPrefixTotalWindows    = []byte{0x05}
	KeyParams                = []byte{0x06}
	KeyPrefixFeederToValoper = []byte{0x07}
	KeyPrefixValoperToFeeder = []byte{0x08}
)

func FeedKey(val sdk.ValAddress, pair string) []byte {
	prefixed, err := address.MustLengthPrefix(val)
	if err != nil {
		panic(err)
	}
	key := append(append([]byte{}, KeyPrefixFeed...), prefixed...)
	return append(key, []byte("/"+pair)...)
}

func ParseFeedKey(key []byte) (sdk.ValAddress, string, error) {
	if len(key) < len(KeyPrefixFeed)+2 || key[0] != KeyPrefixFeed[0] {
		return nil, "", fmt.Errorf("invalid feed key")
	}
	rest := key[len(KeyPrefixFeed):]
	valBytes, err := address.MustBeLengthPrefixed(rest)
	if err != nil {
		return nil, "", fmt.Errorf("invalid feed key: %w", err)
	}
	rest = rest[1+len(valBytes):]
	if len(rest) == 0 || rest[0] != '/' {
		return nil, "", fmt.Errorf("invalid feed key: missing pair")
	}
	return sdk.ValAddress(valBytes), string(rest[1:]), nil
}

func AggregatedPriceKey(pair string) []byte {
	return append(append([]byte{}, KeyPrefixAggregatedPrice...), []byte(pair)...)
}

func TWAPKey(pair string, ts time.Time) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(ts.UnixNano()))
	key := append([]byte{}, KeyPrefixTWAP...)
	key = append(key, []byte(pair)...)
	key = append(key, '/')
	return append(key, buf...)
}

func TWAPPrefix(pair string) []byte {
	key := append([]byte{}, KeyPrefixTWAP...)
	key = append(key, []byte(pair)...)
	return append(key, '/')
}

// FeedKeyPrefix returns the prefix for scanning all feeds (pair filter in callback).
func FeedKeyPrefix() []byte {
	return append([]byte{}, KeyPrefixFeed...)
}

func MissCounterKey(val sdk.ValAddress) []byte {
	return append(append([]byte{}, KeyPrefixMissCounter...), val.Bytes()...)
}

func TotalWindowsKey(val sdk.ValAddress) []byte {
	return append(append([]byte{}, KeyPrefixTotalWindows...), val.Bytes()...)
}

func FeederToValoperKey(feeder sdk.AccAddress) []byte {
	return append(append([]byte{}, KeyPrefixFeederToValoper...), feeder.Bytes()...)
}

func ValoperToFeederKey(val sdk.ValAddress) []byte {
	return append(append([]byte{}, KeyPrefixValoperToFeeder...), val.Bytes()...)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./x/oracle/types/ -run 'TestFeedKeyRoundTrip|TestTWAPKeyOrdering' -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/oracle/types/keys.go x/oracle/types/keys_test.go
git commit -m "feat(oracle): add store key builders for prefixes 0x01-0x08"
```

---

## Task 4: Registered errors

**Files:**
- Create: `x/oracle/types/errors.go`

- [ ] **Step 1: Write `x/oracle/types/errors.go`**

```go
package types

import "cosmossdk.io/errors"

var (
	ErrInvalidSigner        = errors.Register(ModuleName, 2, "invalid signer")
	ErrUnauthorized         = errors.Register(ModuleName, 3, "unauthorized")
	ErrInvalidPrice         = errors.Register(ModuleName, 4, "invalid price: must be positive")
	ErrPairNotAccepted      = errors.Register(ModuleName, 5, "pair not in accept_list")
	ErrNoPrice              = errors.Register(ModuleName, 6, "no aggregated price for pair")
	ErrStalePrice           = errors.Register(ModuleName, 7, "aggregated price is stale")
	ErrNoTWAPData           = errors.Register(ModuleName, 8, "no TWAP data in window")
	ErrNotBondedValidator   = errors.Register(ModuleName, 9, "validator is not bonded")
	ErrFeederNotAuthorized  = errors.Register(ModuleName, 10, "feeder not authorized for validator")
	ErrFeederAlreadyBound   = errors.Register(ModuleName, 11, "feeder already bound to another validator")
	ErrInvalidParams        = errors.Register(ModuleName, 12, "invalid params")
	ErrInvalidPairFormat    = errors.Register(ModuleName, 13, "invalid pair format")
)
```

- [ ] **Step 2: Verify build**

Run: `go build ./x/oracle/types/...`

Expected: success

- [ ] **Step 3: Commit**

```bash
git add x/oracle/types/errors.go
git commit -m "feat(oracle): register module sentinel errors"
```

---

## Task 5: Events

**Files:**
- Create: `x/oracle/types/events.go`

- [ ] **Step 1: Write `x/oracle/types/events.go`**

```go
package types

const (
	EventTypeFeederSet        = "oracle_feeder_set"
	EventTypeFeedSubmitted    = "oracle_feed_submitted"
	EventTypePriceAggregated  = "oracle_price_aggregated"
	EventTypeSlash            = "oracle_slash"
	EventTypeParamsUpdated    = "oracle_params_updated"

	AttributeKeyValidator         = "validator"
	AttributeKeyFeeder            = "feeder"
	AttributeKeyPair              = "pair"
	AttributeKeyPrice             = "price"
	AttributeKeySlashReason       = "slash_reason"
	AttributeKeySlashFraction     = "slash_fraction"
	AttributeKeyAcceptListChanged = "accept_list_changed"
)
```

- [ ] **Step 2: Commit**

```bash
git add x/oracle/types/events.go
git commit -m "feat(oracle): define typed event constants"
```

---

## Task 6: Params defaults and validation

**Files:**
- Replace: `x/oracle/types/params.go` (delete Ignite scaffold `Params` / `DefaultParams` first)
- Create: `x/oracle/types/params_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/oracle/types/params_test.go`:

```go
package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestDefaultParamsValid(t *testing.T) {
	require.NoError(t, types.DefaultParams().Validate())
}

func TestValidateRejectsEmptyAcceptList(t *testing.T) {
	p := types.DefaultParams()
	p.AcceptList = nil
	require.Error(t, p.Validate())
}

func TestValidateRejectsDuplicatePair(t *testing.T) {
	p := types.DefaultParams()
	p.AcceptList = []string{"BTC:USD", "BTC:USD"}
	require.Error(t, p.Validate())
}

func TestValidateRejectsSlashRateAboveDoubleSign(t *testing.T) {
	p := types.DefaultParams()
	p.MissSlashRate = "0.05"
	require.Error(t, p.Validate())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./x/oracle/types/ -run TestDefaultParams -v`

Expected: FAIL — `DefaultParams` not defined.

- [ ] **Step 3: Write `x/oracle/types/params.go`**

```go
package types

import (
	"regexp"

	"cosmossdk.io/math"
)

var pairRegex = regexp.MustCompile(`^[A-Z0-9]+:[A-Z0-9]+$`)

func DefaultParams() OracleParams {
	return OracleParams{
		VoteWindow:        10,
		MissThreshold:     "0.05",
		MissSlashRate:     "0.005",
		OutlierSlashRate:  "0.01",
		OutlierThreshold:  "0.05",
		MissWindowSize:    500,
		QuorumFraction:    "0.667",
		MaxPriceAge:       300,
		AcceptList:        []string{"VTX:USD", "BTC:USD", "ETH:USD", "ATOM:USD", "USDC:USD"},
	}
}

func (p OracleParams) Validate() error {
	if p.VoteWindow <= 0 {
		return ErrInvalidParams.Wrap("vote_window must be > 0")
	}
	if p.MissWindowSize <= 0 {
		return ErrInvalidParams.Wrap("miss_window_size must be > 0")
	}
	if p.MaxPriceAge <= 0 {
		return ErrInvalidParams.Wrap("max_price_age must be > 0")
	}
	if err := validateDecOpenClosed(p.MissThreshold, "miss_threshold"); err != nil {
		return err
	}
	if err := validateDecOpenClosed(p.OutlierThreshold, "outlier_threshold"); err != nil {
		return err
	}
	if err := validateDecOpenClosed(p.QuorumFraction, "quorum_fraction"); err != nil {
		return err
	}
	// quorum_fraction <= 0.5: warn at runtime via keeper Logger if desired; Validate does not reject
	if err := validateSlashRate(p.MissSlashRate, "miss_slash_rate"); err != nil {
		return err
	}
	if err := validateSlashRate(p.OutlierSlashRate, "outlier_slash_rate"); err != nil {
		return err
	}
	if len(p.AcceptList) == 0 {
		return ErrInvalidParams.Wrap("accept_list must be non-empty")
	}
	seen := make(map[string]struct{}, len(p.AcceptList))
	for _, pair := range p.AcceptList {
		if !pairRegex.MatchString(pair) {
			return ErrInvalidPairFormat.Wrapf("pair %q", pair)
		}
		if _, ok := seen[pair]; ok {
			return ErrInvalidParams.Wrapf("duplicate pair %q", pair)
		}
		seen[pair] = struct{}{}
	}
	return nil
}

func validateDecOpenClosed(s, name string) error {
	d, err := math.LegacyNewDecFromStr(s)
	if err != nil {
		return ErrInvalidParams.Wrapf("%s: %v", name, err)
	}
	if d.LTE(math.LegacyZeroDec()) || d.GT(math.LegacyOneDec()) {
		return ErrInvalidParams.Wrapf("%s must be in (0, 1]", name)
	}
	return nil
}

func validateSlashRate(s, name string) error {
	d, err := math.LegacyNewDecFromStr(s)
	if err != nil {
		return ErrInvalidParams.Wrapf("%s: %v", name, err)
	}
	max := math.LegacyNewDecWithPrec(5, 2) // 0.05 double-sign ceiling
	if d.IsNegative() || d.GTE(max) {
		return ErrInvalidParams.Wrapf("%s must be in [0, 0.05)", name)
	}
	return nil
}

func (p OracleParams) AcceptListChanged(other OracleParams) bool {
	if len(p.AcceptList) != len(other.AcceptList) {
		return true
	}
	for i := range p.AcceptList {
		if p.AcceptList[i] != other.AcceptList[i] {
			return true
		}
	}
	return false
}

func (p OracleParams) PairAccepted(pair string) bool {
	for _, ap := range p.AcceptList {
		if ap == pair {
			return true
		}
	}
	return false
}
```

Do **not** add a custom `OracleParams.String()` — gogoproto generates `func (*OracleParams) String() string` on the proto type; a value-receiver `String()` collides and fails to compile.

- [ ] **Step 4: Run tests**

Run: `go test ./x/oracle/types/ -run 'TestDefaultParams|TestValidate' -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add x/oracle/types/params.go x/oracle/types/params_test.go
git commit -m "feat(oracle): add DefaultParams and Validate"
```

---

## Task 7: Message ValidateBasic

**Files:**
- Create: `x/oracle/types/msgs.go`
- Create: `x/oracle/types/msgs_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/oracle/types/msgs_test.go`:

```go
package types_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestMsgSubmitFeedValidateBasic(t *testing.T) {
	val := sdk.ValAddress([]byte("validator123456789012345678901234"))
	feeder := sdk.AccAddress([]byte("feeder1234567890123456789012345"))
	msg := &types.MsgSubmitFeed{
		Feeder:    feeder.String(),
		Validator: val.String(),
		Pair:      "BTC:USD",
		Price:     "42000.5",
	}
	require.NoError(t, msg.ValidateBasic())

	bad := *msg
	bad.Price = "-1"
	require.Error(t, bad.ValidateBasic())
}

func TestMsgSetFeederValidateBasic(t *testing.T) {
	val := sdk.ValAddress([]byte("validator123456789012345678901234"))
	feeder := sdk.AccAddress([]byte("feeder1234567890123456789012345"))
	require.NoError(t, (&types.MsgSetFeeder{Validator: val.String(), Feeder: feeder.String()}).ValidateBasic())
}
```

Configure bech32 prefix in test init — add to `x/oracle/types/msgs_test.go`:

```go
func init() {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("vtx", "vtxpub")
	cfg.SetBech32PrefixForValidator("vtxvaloper", "vtxvaloperpub")
	cfg.SetBech32PrefixForConsensusNode("vtxvalcons", "vtxvalconspub")
	cfg.Seal()
}
```

Use proper-length addresses or use `testutil/sample` if available. Prefer:

```go
val := sdk.ValAddress("validator123456789012345678901234")
feeder := sdk.AccAddress("feeder1234567890123456789012345")
```

And in ValidateBasic use `sdk.ValAddressFromBech32` / `sdk.AccAddressFromBech32` — tests must use valid bech32. Use addresses from genesis helpers:

```go
import "github.com/vertix-network/vertix/testutil/sample"

val := sample.AccAddress() // convert to valoper in msg
```

Check `testutil/sample/sample.go` for helpers; if only `AccAddress()` exists, use:

```go
acc := sample.AccAddress()
val := sdk.ValAddress(sdk.MustAccAddressFromBech32(acc).Bytes())
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/oracle/types/ -run TestMsgSubmitFeed -v`

- [ ] **Step 3: Write `x/oracle/types/msgs.go`**

```go
package types

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func (m *MsgSetFeeder) ValidateBasic() error {
	if _, err := sdk.ValAddressFromBech32(m.Validator); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("validator: %v", err)
	}
	if _, err := sdk.AccAddressFromBech32(m.Feeder); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("feeder: %v", err)
	}
	return nil
}

func (m *MsgSubmitFeed) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Feeder); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("feeder: %v", err)
	}
	if _, err := sdk.ValAddressFromBech32(m.Validator); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("validator: %v", err)
	}
	if m.Pair == "" {
		return ErrInvalidPairFormat.Wrap("pair is empty")
	}
	price, err := math.LegacyNewDecFromStr(m.Price)
	if err != nil {
		return ErrInvalidPrice.Wrapf("parse: %v", err)
	}
	if !price.IsPositive() {
		return ErrInvalidPrice.Wrap("price must be positive")
	}
	return nil
}

func (m *MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("authority: %v", err)
	}
	return m.Params.Validate()
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./x/oracle/types/ -run 'TestMsg' -v`

- [ ] **Step 5: Commit**

```bash
git add x/oracle/types/msgs.go x/oracle/types/msgs_test.go
git commit -m "feat(oracle): add ValidateBasic for oracle messages"
```

---

## Task 8: Expected keeper interfaces

**Files:**
- Modify: `x/oracle/types/expected_keepers.go` (replace Ignite scaffold stubs)

- [ ] **Step 1: Write `x/oracle/types/expected_keepers.go`**

```go
package types

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type StakingKeeper interface {
	GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error)
	GetLastValidatorPower(ctx context.Context, addr sdk.ValAddress) (int64, error)
	GetBondedValidatorsByPower(ctx context.Context) ([]stakingtypes.Validator, error)
	GetLastTotalPower(ctx context.Context) (math.Int, error) // quorum denominator — consensus power (spec C1b)
}

type SlashingKeeper interface {
	Slash(ctx context.Context, consAddr sdk.ConsAddress, fraction math.LegacyDec, power, distributionHeight int64) error
}

// OracleKeeper is the public interface consumed by x/rwa (Phase 3).
type OracleKeeper interface {
	GetPrice(ctx context.Context, pair string) (math.LegacyDec, error)
	GetTWAP(ctx context.Context, pair string, window time.Duration) (math.LegacyDec, error)
}
```

Add `"time"` import.

- [ ] **Step 2: Commit**

```bash
git add x/oracle/types/expected_keepers.go
git commit -m "feat(oracle): define StakingKeeper and SlashingKeeper interfaces"
```

---

## Task 9: Test fixture with mock keepers

**Files:**
- Create: `testutil/keeper/oracle.go`

- [ ] **Step 1: Write `testutil/keeper/oracle.go`**

```go
package keeper

import (
	"context"
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
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	oraclekeeper "github.com/vertix-network/vertix/x/oracle/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

type MockStaking struct {
	Validators  []stakingtypes.Validator
	Powers      map[string]int64
	TotalPower  math.Int // total consensus power for quorum (same units as Powers values)
}

func (m *MockStaking) GetValidator(_ context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error) {
	for _, v := range m.Validators {
		if v.OperatorAddress == addr.String() {
			return v, nil
		}
	}
	return stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound
}

func (m *MockStaking) GetLastValidatorPower(_ context.Context, addr sdk.ValAddress) (int64, error) {
	if p, ok := m.Powers[addr.String()]; ok {
		return p, nil
	}
	return 0, nil
}

func (m *MockStaking) GetBondedValidatorsByPower(_ context.Context) ([]stakingtypes.Validator, error) {
	return m.Validators, nil
}

func (m *MockStaking) GetLastTotalPower(_ context.Context) (math.Int, error) {
	if !m.TotalPower.IsZero() {
		return m.TotalPower, nil
	}
	var sum int64
	for _, p := range m.Powers {
		sum += p
	}
	return math.NewInt(sum), nil
}

type MockSlashing struct {
	Calls []SlashCall
}

type SlashCall struct {
	ConsAddr sdk.ConsAddress
	Fraction math.LegacyDec
	Power    int64
	Height   int64
}

func (m *MockSlashing) Slash(_ context.Context, consAddr sdk.ConsAddress, fraction math.LegacyDec, power, height int64) error {
	m.Calls = append(m.Calls, SlashCall{consAddr, fraction, power, height})
	return nil
}

func OracleKeeper(t *testing.T, staking *MockStaking, slashing *MockSlashing) (sdk.Context, oraclekeeper.Keeper) {
	t.Helper()
	memDB := cosmosdb.NewMemDB()
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	stateStore := store.NewCommitMultiStore(memDB, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, memDB)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	k := oraclekeeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		staking,
		slashing,
		authority,
	)
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 1}, false, log.NewNopLogger())
	require.NoError(t, k.SetParams(ctx, types.DefaultParams()))
	return ctx, k
}
```

Register staking interfaces on the codec registry if tests marshal validators.

- [ ] **Step 2: Verify compile**

Run: `go build ./testutil/keeper/...`

Expected: may fail until `keeper.NewKeeper` exists — that's Task 10.

- [ ] **Step 3: Commit after Task 10 completes** (or commit stub now and amend in Task 10)

---

## Task 10: Keeper skeleton and params store

**Files:**
- Modify: `x/oracle/keeper/keeper.go`
- Create: `x/oracle/keeper/params_test.go`

- [ ] **Step 1: Write failing params round-trip test**

Create `x/oracle/keeper/params_test.go`:

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestParamsRoundTrip(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	got, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, types.DefaultParams(), got)
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./x/oracle/keeper/ -run TestParamsRoundTrip -v`

- [ ] **Step 3: Write `x/oracle/keeper/keeper.go`**

```go
package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/oracle/types"
)

type Keeper struct {
	cdc      codec.BinaryCodec
	storeService store.KVStoreService
	stakingKeeper  types.StakingKeeper
	slashingKeeper types.SlashingKeeper
	authority      string
}

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	sk types.StakingKeeper,
	slk types.SlashingKeeper,
	authority string,
) Keeper {
	return Keeper{cdc, storeService, sk, slk, authority}
}

func (k Keeper) GetAuthority() string { return k.authority }

func (k Keeper) Logger(ctx context.Context) log.Logger {
	return sdk.UnwrapSDKContext(ctx).Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

func (k Keeper) GetParams(ctx context.Context) (types.OracleParams, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyParams)
	if err != nil {
		return types.OracleParams{}, err
	}
	if bz == nil {
		return types.DefaultParams(), nil
	}
	var p types.OracleParams
	k.cdc.MustUnmarshal(bz, &p)
	return p, nil
}

func (k Keeper) SetParams(ctx context.Context, p types.OracleParams) error {
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
```

Use `store.KVStoreService` from cosmossdk.io/core/store — match scaffold imports if different.

- [ ] **Step 4: Run test — expect PASS**

Run: `go test ./x/oracle/keeper/ -run TestParamsRoundTrip -v`

- [ ] **Step 5: Commit testutil + keeper**

```bash
git add x/oracle/keeper/ testutil/keeper/
git commit -m "feat(oracle): add keeper skeleton and params store"
```

---

## Task 11: Feeder delegation

**Files:**
- Create: `x/oracle/keeper/feeder.go`
- Create: `x/oracle/keeper/feeder_test.go`

- [ ] **Step 1: Write failing test**

```go
package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
)

func TestResolveAuthorizedFeederImplicitOperator(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	val := sdk.ValAddress([]byte("validator123456789012345678901234"))
	feeder, err := k.ResolveAuthorizedFeeder(ctx, val)
	require.NoError(t, err)
	require.Equal(t, sdk.AccAddress(val.Bytes()).String(), feeder.String())
}
```

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Write `x/oracle/keeper/feeder.go`**

```go
package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/oracle/types"
)

func (k Keeper) SetFeederDelegation(ctx context.Context, val sdk.ValAddress, feeder sdk.AccAddress) error {
	store := k.storeService.OpenKVStore(ctx)
	existing, err := store.Get(types.FeederToValoperKey(feeder))
	if err != nil {
		return err
	}
	if existing != nil && !sdk.ValAddress(existing).Equals(val) {
		return types.ErrFeederAlreadyBound
	}
	if err := store.Set(types.FeederToValoperKey(feeder), val.Bytes()); err != nil {
		return err
	}
	return store.Set(types.ValoperToFeederKey(val), feeder.Bytes())
}

func (k Keeper) GetFeederForValidator(ctx context.Context, val sdk.ValAddress) (sdk.AccAddress, bool, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ValoperToFeederKey(val))
	if err != nil {
		return nil, false, err
	}
	if bz == nil {
		return nil, false, nil
	}
	return sdk.AccAddress(bz), true, nil
}

func (k Keeper) ResolveAuthorizedFeeder(ctx context.Context, val sdk.ValAddress) (sdk.AccAddress, error) {
	if feeder, ok, err := k.GetFeederForValidator(ctx, val); err != nil {
		return nil, err
	} else if ok {
		return feeder, nil
	}
	return sdk.AccAddress(val.Bytes()), nil
}
```

- [ ] **Step 4: Run — PASS**

- [ ] **Step 5: Commit**

```bash
git add x/oracle/keeper/feeder.go x/oracle/keeper/feeder_test.go
git commit -m "feat(oracle): add feeder delegation store ops"
```

---

## Task 12: MsgSetFeeder handler

**Files:**
- Create: `x/oracle/keeper/msg_server.go`
- Create: `x/oracle/keeper/msg_server_set_feeder_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/oracle/keeper/msg_server_set_feeder_test.go`:

```go
package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
	"github.com/vertix-network/vertix/x/oracle/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestMsgSetFeeder(t *testing.T) {
	valAcc := sdk.ValAddress(sdk.MustAccAddressFromBech32(sample.AccAddress()).Bytes())
	valStr := valAcc.String()
	feeder := sample.AccAddress()
	mockStaking := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{
			{OperatorAddress: valStr, Status: stakingtypes.Bonded},
		},
	}
	ctx, k := keeper.OracleKeeper(t, mockStaking, &keeper.MockSlashing{})
	server := keeper.NewMsgServerImpl(k)

	_, err := server.SetFeeder(ctx, &types.MsgSetFeeder{
		Validator: valStr,
		Feeder:    feeder,
	})
	require.NoError(t, err)

	got, ok, err := k.GetFeederForValidator(ctx, valAcc)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, feeder, got.String())
}
```

- [ ] **Step 2: Implement `SetFeeder` in `msg_server.go`**

```go
func (m msgServer) SetFeeder(ctx context.Context, msg *types.MsgSetFeeder) (*types.MsgSetFeederResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	val, _ := sdk.ValAddressFromBech32(msg.Validator)
	feeder, _ := sdk.AccAddressFromBech32(msg.Feeder)
	if m.k.stakingKeeper != nil {
		if _, err := m.k.stakingKeeper.GetValidator(ctx, val); err != nil {
			return nil, err
		}
	}
	if err := m.k.SetFeederDelegation(ctx, val, feeder); err != nil {
		return nil, err
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeFeederSet,
		sdk.NewAttribute(types.AttributeKeyValidator, msg.Validator),
		sdk.NewAttribute(types.AttributeKeyFeeder, msg.Feeder),
	))
	return &types.MsgSetFeederResponse{}, nil
}
```

Wire `msgServer` struct embedding `Keeper` per scaffold pattern.

- [ ] **Step 3: Run tests — PASS**

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(oracle): implement MsgSetFeeder handler"
```

---

## Task 13: Feed store ops and MsgSubmitFeed

**Files:**
- Modify: `x/oracle/keeper/keeper.go` (feed CRUD)
- Modify: `x/oracle/keeper/msg_server.go` (SubmitFeed)
- Create: `x/oracle/keeper/msg_server_submit_feed_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/oracle/keeper/msg_server_submit_feed_test.go`:

```go
package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
	oraclekeeper "github.com/vertix-network/vertix/x/oracle/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestMsgSubmitFeedAuthorized(t *testing.T) {
	valAcc := sdk.ValAddress(sdk.MustAccAddressFromBech32(sample.AccAddress()).Bytes())
	valStr := valAcc.String()
	feeder := sdk.MustAccAddressFromBech32(sample.AccAddress())
	mockStaking := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{{OperatorAddress: valStr, Status: stakingtypes.Bonded}},
		Powers:     map[string]int64{valStr: 100},
	}
	ctx, k := keeper.OracleKeeper(t, mockStaking, &keeper.MockSlashing{})
	server := oraclekeeper.NewMsgServerImpl(k)

	_, err := server.SubmitFeed(ctx, &types.MsgSubmitFeed{
		Feeder: feeder.String(), Validator: valStr, Pair: "BTC:USD", Price: "42000",
	})
	require.NoError(t, err)

	feed, ok, err := k.GetFeed(ctx, valAcc, "BTC:USD")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "42000", feed.Price)
}

func TestMsgSubmitFeedUnauthorizedFeeder(t *testing.T) {
	valAcc := sdk.ValAddress(sdk.MustAccAddressFromBech32(sample.AccAddress()).Bytes())
	valStr := valAcc.String()
	mockStaking := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{{OperatorAddress: valStr, Status: stakingtypes.Bonded}},
		Powers:     map[string]int64{valStr: 100},
	}
	ctx, k := keeper.OracleKeeper(t, mockStaking, &keeper.MockSlashing{})
	server := oraclekeeper.NewMsgServerImpl(k)

	// delegate a different feeder first
	delegated := sample.AccAddress()
	_, err := server.SetFeeder(ctx, &types.MsgSetFeeder{Validator: valStr, Feeder: delegated})
	require.NoError(t, err)

	wrongFeeder := sample.AccAddress()
	_, err = server.SubmitFeed(ctx, &types.MsgSubmitFeed{
		Feeder: wrongFeeder, Validator: valStr, Pair: "BTC:USD", Price: "42000",
	})
	require.ErrorIs(t, err, types.ErrFeederNotAuthorized)
}
```

- [ ] **Step 2: Add to `keeper.go`**

```go
func (k Keeper) SetFeed(ctx context.Context, feed types.OracleFeed) error {
	val, err := sdk.ValAddressFromBech32(feed.Validator)
	if err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := k.cdc.Marshal(&feed)
	if err != nil {
		return err
	}
	return store.Set(types.FeedKey(val, feed.Pair), bz)
}

func (k Keeper) GetFeed(ctx context.Context, val sdk.ValAddress, pair string) (types.OracleFeed, bool, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.FeedKey(val, pair))
	if err != nil || bz == nil {
		return types.OracleFeed{}, false, err
	}
	var feed types.OracleFeed
	k.cdc.MustUnmarshal(bz, &feed)
	return feed, true, nil
}
```

- [ ] **Step 3: Implement `SubmitFeed`**

```go
func (m msgServer) SubmitFeed(ctx context.Context, msg *types.MsgSubmitFeed) (*types.MsgSubmitFeedResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	val, _ := sdk.ValAddressFromBech32(msg.Validator)
	feeder, _ := sdk.AccAddressFromBech32(msg.Feeder)
	authorized, err := m.k.ResolveAuthorizedFeeder(ctx, val)
	if err != nil {
		return nil, err
	}
	if !authorized.Equals(feeder) {
		return nil, types.ErrFeederNotAuthorized
	}
	params, err := m.k.GetParams(ctx)
	if err != nil {
		return nil, err
	}
	if !params.PairAccepted(msg.Pair) {
		return nil, types.ErrPairNotAccepted
	}
	if m.k.stakingKeeper != nil {
		v, err := m.k.stakingKeeper.GetValidator(ctx, val)
		if err != nil {
			return nil, err
		}
		if v.Status != stakingtypes.Bonded {
			return nil, types.ErrNotBondedValidator
		}
		power, err := m.k.stakingKeeper.GetLastValidatorPower(ctx, val)
		if err != nil || power == 0 {
			return nil, types.ErrNotBondedValidator
		}
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	feed := types.OracleFeed{
		Validator:   msg.Validator,
		Pair:        msg.Pair,
		Price:       msg.Price,
		BlockHeight: sdkCtx.BlockHeight(),
	}
	if err := m.k.SetFeed(ctx, feed); err != nil {
		return nil, err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeFeedSubmitted,
		sdk.NewAttribute(types.AttributeKeyValidator, msg.Validator),
		sdk.NewAttribute(types.AttributeKeyFeeder, msg.Feeder),
		sdk.NewAttribute(types.AttributeKeyPair, msg.Pair),
		sdk.NewAttribute(types.AttributeKeyPrice, msg.Price),
	))
	return &types.MsgSubmitFeedResponse{}, nil
}
```

- [ ] **Step 4: Run tests — PASS**

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(oracle): implement MsgSubmitFeed with feeder authorization"
```

---

## Task 14: MsgUpdateParams with accept_list counter reset

**Files:**
- Modify: `x/oracle/keeper/keeper.go` (ResetMissCounters)
- Modify: `x/oracle/keeper/msg_server.go` (UpdateParams)
- Create: `x/oracle/keeper/msg_server_update_params_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/oracle/keeper/msg_server_update_params_test.go`:

```go
package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
	oraclekeeper "github.com/vertix-network/vertix/x/oracle/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestUpdateParamsResetsCountersOnAcceptListChange(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	val := sdk.ValAddress(sdk.MustAccAddressFromBech32(sample.AccAddress()).Bytes())
	require.NoError(t, k.IncMissCounter(ctx, val))
	require.NoError(t, k.IncTotalWindows(ctx, val))

	server := oraclekeeper.NewMsgServerImpl(k)
	newParams := types.DefaultParams()
	newParams.AcceptList = append(newParams.AcceptList, "SOL:USD")
	_, err := server.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: k.GetAuthority(),
		Params:    newParams,
	})
	require.NoError(t, err)

	misses, err := k.GetMissCounter(ctx, val)
	require.NoError(t, err)
	require.Equal(t, int64(0), misses)
}
```

- [ ] **Step 2: Add `ResetMissCounters`**

```go
func (k Keeper) ResetMissCounters(ctx context.Context) error {
	store := k.storeService.OpenKVStore(ctx)
	if err := deleteAllWithPrefix(store, types.KeyPrefixMissCounter); err != nil {
		return err
	}
	return deleteAllWithPrefix(store, types.KeyPrefixTotalWindows)
}

// deleteAllWithPrefix collects keys first, then deletes — safe while iterating.
func deleteAllWithPrefix(store storetypes.KVStore, prefix []byte) error {
	iter, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return err
	}
	var keys [][]byte
	for ; iter.Valid(); iter.Next() {
		keys = append(keys, append([]byte{}, iter.Key()...))
	}
	iter.Close()
	for _, key := range keys {
		if err := store.Delete(key); err != nil {
			return err
		}
	}
	return nil
}
```

Add `storetypes "cosmossdk.io/store/types"` import in `keeper.go`.

- [ ] **Step 3: Implement UpdateParams**

Authority check → Validate → if accept_list changed call ResetMissCounters → SetParams → emit event.

- [ ] **Step 4: Run tests — PASS**

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(oracle): implement MsgUpdateParams with counter reset on accept_list change"
```

---

## Task 15: Aggregation math (pure functions)

**Files:**
- Create: `x/oracle/keeper/aggregate.go`
- Create: `x/oracle/keeper/aggregate_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestWeightedMedianThreeValidators(t *testing.T) {
	entries := []WeightedPrice{
		{Price: dec("100"), Weight: 10, Valoper: "a"},
		{Price: dec("110"), Weight: 30, Valoper: "b"},
		{Price: dec("120"), Weight: 10, Valoper: "c"},
	}
	got := WeightedMedian(entries)
	require.True(t, got.Equal(dec("110")))
}

func TestUnweightedMedianEvenCount(t *testing.T) {
	prices := []math.LegacyDec{dec("100"), dec("200")}
	got := UnweightedMedian(prices)
	require.True(t, got.Equal(dec("100"))) // lower-mid
}

func TestQuorumMet(t *testing.T) {
	require.True(t, QuorumMet(math.NewInt(67), math.NewInt(100), dec("0.667")))
	require.False(t, QuorumMet(math.NewInt(66), math.NewInt(100), dec("0.667")))
}
```

- [ ] **Step 2: Implement `aggregate.go`**

```go
type WeightedPrice struct {
	Price   math.LegacyDec
	Weight  int64
	Valoper string
}

func WeightedMedian(entries []WeightedPrice) math.LegacyDec {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Price.Equal(entries[j].Price) {
			return entries[i].Valoper < entries[j].Valoper
		}
		return entries[i].Price.LT(entries[j].Price)
	})
	var total int64
	for _, e := range entries {
		total += e.Weight
	}
	half := total / 2
	var cum int64
	for _, e := range entries {
		cum += e.Weight
		if cum >= half {
			return e.Price
		}
	}
	return math.LegacyZeroDec()
}

func UnweightedMedian(prices []math.LegacyDec) math.LegacyDec {
	sort.Slice(prices, func(i, j int) bool { return prices[i].LT(prices[j]) })
	n := len(prices)
	if n == 0 {
		return math.LegacyZeroDec()
	}
	return prices[n/2]
}

func QuorumMet(submittedPower, totalPower math.Int, quorumFraction math.LegacyDec) bool {
	if totalPower.IsZero() {
		return false
	}
	ratio := math.LegacyNewDec(submittedPower.Int64()).Quo(math.LegacyNewDec(totalPower.Int64()))
	return !ratio.LT(quorumFraction)
}
```

- [ ] **Step 3: Run tests — PASS**

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(oracle): add WeightedMedian, UnweightedMedian, QuorumMet"
```

---

## Task 16: GetPrice with staleness

**Files:**
- Modify: `x/oracle/keeper/keeper.go`
- Create: `x/oracle/keeper/price_test.go`

- [ ] **Step 1: Write failing tests**

Cases: no price → ErrNoPrice; stale → ErrStalePrice; fresh → price returned; pair not in list → ErrPairNotAccepted.

- [ ] **Step 2: Implement SetAggregatedPrice + GetPrice**

```go
func (k Keeper) SetAggregatedPrice(ctx context.Context, ap types.AggregatedPrice) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := k.cdc.Marshal(&ap)
	if err != nil {
		return err
	}
	return store.Set(types.AggregatedPriceKey(ap.Pair), bz)
}

func (k Keeper) GetPrice(ctx context.Context, pair string) (math.LegacyDec, error) {
	params, err := k.GetParams(ctx)
	if err != nil {
		return math.LegacyZeroDec(), err
	}
	if !params.PairAccepted(pair) {
		return math.LegacyZeroDec(), types.ErrPairNotAccepted
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.AggregatedPriceKey(pair))
	if err != nil {
		return math.LegacyZeroDec(), err
	}
	if bz == nil {
		return math.LegacyZeroDec(), types.ErrNoPrice
	}
	var ap types.AggregatedPrice
	k.cdc.MustUnmarshal(bz, &ap)
	price, err := math.LegacyNewDecFromStr(ap.Price)
	if err != nil {
		return math.LegacyZeroDec(), err
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	age := sdkCtx.BlockTime().Sub(ap.BlockTime)
	if age > time.Duration(params.MaxPriceAge)*time.Second {
		return math.LegacyZeroDec(), types.ErrStalePrice
	}
	return price, nil
}
```

- [ ] **Step 3: Run tests — PASS**

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(oracle): implement GetPrice with MaxPriceAge staleness guard"
```

---

## Task 17: TWAP store, GetTWAP, prune

**Files:**
- Create: `x/oracle/keeper/twap.go`
- Create: `x/oracle/keeper/twap_test.go`

- [ ] **Step 1: Write the failing test**

Create `x/oracle/keeper/twap_test.go`:

```go
package keeper_test

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
)

func dec(s string) math.LegacyDec {
	d, err := math.LegacyNewDecFromStr(s)
	require.NoError(t, err)
	return d
}

func TestGetTWAPDurationWeighted(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	base := time.Unix(1000, 0).UTC()
	ctx = ctx.WithBlockTime(base.Add(250 * time.Second))

	require.NoError(t, k.AppendTWAPEntry(ctx, "BTC:USD", dec("100"), base))
	require.NoError(t, k.AppendTWAPEntry(ctx, "BTC:USD", dec("200"), base.Add(100*time.Second)))
	require.NoError(t, k.AppendTWAPEntry(ctx, "BTC:USD", dec("300"), base.Add(200*time.Second)))

	got, err := k.GetTWAP(sdk.WrapSDKContext(ctx), "BTC:USD", 200*time.Second)
	require.NoError(t, err)
	// window [1050, 1250]: entry@1000 contributes from 1050; 100×100s + 200×100s + 300×50s = 45000 / 250s
	require.True(t, got.Equal(dec("180")))
}
```

- [ ] **Step 2: Implement `twap.go`**

```go
func (k Keeper) AppendTWAPEntry(ctx context.Context, pair string, price math.LegacyDec, blockTime time.Time) error {
	store := k.storeService.OpenKVStore(ctx)
	entry := types.TWAPEntry{Pair: pair, Price: price.String(), BlockTime: blockTime}
	bz, err := k.cdc.Marshal(&entry)
	if err != nil {
		return err
	}
	return store.Set(types.TWAPKey(pair, blockTime), bz)
}

func (k Keeper) PruneTWAPOlderThan(ctx context.Context, pair string, cutoff time.Time) error {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.TWAPPrefix(pair)
	iter, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return err
	}
	var toDelete [][]byte
	for ; iter.Valid(); iter.Next() {
		var e types.TWAPEntry
		k.cdc.MustUnmarshal(iter.Value(), &e)
		if e.BlockTime.Before(cutoff) {
			toDelete = append(toDelete, append([]byte{}, iter.Key()...))
		} else {
			break
		}
	}
	iter.Close()
	for _, key := range toDelete {
		if err := store.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

func (k Keeper) GetTWAP(ctx context.Context, pair string, window time.Duration) (math.LegacyDec, error) {
	params, err := k.GetParams(ctx)
	if err != nil {
		return math.LegacyZeroDec(), err
	}
	if !params.PairAccepted(pair) {
		return math.LegacyZeroDec(), types.ErrPairNotAccepted
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime()
	windowStart := now.Add(-window)

	store := k.storeService.OpenKVStore(ctx)
	prefix := types.TWAPPrefix(pair)
	iter, err := store.ReverseIterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return math.LegacyZeroDec(), err
	}
	defer iter.Close()

	var entries []types.TWAPEntry
	for ; iter.Valid(); iter.Next() {
		var e types.TWAPEntry
		k.cdc.MustUnmarshal(iter.Value(), &e)
		if e.BlockTime.After(now) {
			continue
		}
		entries = append(entries, e)
		if !e.BlockTime.After(windowStart) {
			break // include last entry at or before windowStart (leading clamp)
		}
	}
	if len(entries) == 0 {
		return math.LegacyZeroDec(), types.ErrNoTWAPData
	}
	// reverse to ascending
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	if now.Sub(entries[len(entries)-1].BlockTime) > time.Duration(params.MaxPriceAge)*time.Second {
		return math.LegacyZeroDec(), types.ErrStalePrice
	}
	if len(entries) == 1 {
		return math.LegacyNewDecFromStr(entries[0].Price)
	}
	var weightedSum, totalWeight math.LegacyDec
	for i := 0; i < len(entries); i++ {
		price, _ := math.LegacyNewDecFromStr(entries[i].Price)
		start := entries[i].BlockTime
		if start.Before(windowStart) {
			start = windowStart
		}
		var end time.Time
		if i+1 < len(entries) {
			end = entries[i+1].BlockTime
		} else {
			end = now
		}
		w := math.LegacyNewDec(end.Sub(start).Nanoseconds())
		weightedSum = weightedSum.Add(price.Mul(w))
		totalWeight = totalWeight.Add(w)
	}
	if totalWeight.IsZero() {
		return math.LegacyZeroDec(), types.ErrNoTWAPData
	}
	return weightedSum.Quo(totalWeight), nil
}
```

- [ ] **Step 3: Run tests — PASS**

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(oracle): implement TWAP append, prune, and duration-weighted GetTWAP"
```

---

## Task 18: EndBlock aggregation + TWAP (no slashing)

**Files:**
- Create: `x/oracle/keeper/abci.go`
- Create: `x/oracle/keeper/endblock_aggregate_test.go`

- [ ] **Step 1: Write the failing integration test**

Create `x/oracle/keeper/endblock_aggregate_test.go`:

```go
package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/testutil/sample"
	oraclekeeper "github.com/vertix-network/vertix/x/oracle/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func bondingValidator(t *testing.T, val sdk.ValAddress) stakingtypes.Validator {
	t.Helper()
	pub := ed25519.GenPrivKey().PubKey()
	pkAny, err := codectypes.NewAnyWithValue(pub)
	require.NoError(t, err)
	return stakingtypes.Validator{
		OperatorAddress: val.String(),
		ConsensusPubkey: pkAny,
		Status:          stakingtypes.Bonded,
	}
}

func makeValidator(t *testing.T) (sdk.ValAddress, string) {
	t.Helper()
	val := sdk.ValAddress(sdk.MustAccAddressFromBech32(sample.AccAddress()).Bytes())
	return val, val.String()
}

func TestEndBlockAggregatesWeightedMedian(t *testing.T) {
	v1, s1 := makeValidator(t)
	v2, s2 := makeValidator(t)
	v3, s3 := makeValidator(t)
	mock := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{
			bondingValidator(t, v1),
			bondingValidator(t, v2),
			bondingValidator(t, v3),
		},
		Powers:     map[string]int64{s1: 10, s2: 30, s3: 10},
		TotalPower: math.NewInt(50), // consensus power (same units as Powers)
	}
	ctx, k := keeper.OracleKeeper(t, mock, &keeper.MockSlashing{})
	p := types.DefaultParams()
	p.VoteWindow = 1
	require.NoError(t, k.SetParams(ctx, p))
	server := oraclekeeper.NewMsgServerImpl(k)
	for _, tc := range []struct {
		val sdk.ValAddress
		str string
		px  string
	}{{v1, s1, "100"}, {v2, s2, "110"}, {v3, s3, "120"}} {
		_, err := server.SubmitFeed(ctx, &types.MsgSubmitFeed{
			Feeder: sdk.AccAddress(tc.val.Bytes()).String(), Validator: tc.str,
			Pair: "BTC:USD", Price: tc.px,
		})
		require.NoError(t, err)
	}
	require.NoError(t, k.EndBlocker(sdk.WrapSDKContext(ctx)))
	got, err := k.GetPrice(sdk.WrapSDKContext(ctx), "BTC:USD")
	require.NoError(t, err)
	require.True(t, got.Equal(math.LegacyNewDec(110)))
}

func TestEndBlockSkipsPairBelowQuorum(t *testing.T) {
	v1, s1 := makeValidator(t)
	mock := &keeper.MockStaking{
		Validators: []stakingtypes.Validator{bondingValidator(t, v1)},
		Powers:     map[string]int64{s1: 10},
		TotalPower: math.NewInt(100), // 10/100 < 0.667 quorum
	}
	ctx, k := keeper.OracleKeeper(t, mock, &keeper.MockSlashing{})
	p := types.DefaultParams()
	p.VoteWindow = 1
	require.NoError(t, k.SetParams(ctx, p))
	server := oraclekeeper.NewMsgServerImpl(k)
	_, err := server.SubmitFeed(ctx, &types.MsgSubmitFeed{
		Feeder: sdk.AccAddress(v1.Bytes()).String(), Validator: s1,
		Pair: "BTC:USD", Price: "100",
	})
	require.NoError(t, err)
	require.NoError(t, k.EndBlocker(sdk.WrapSDKContext(ctx)))
	_, err = k.GetPrice(sdk.WrapSDKContext(ctx), "BTC:USD")
	require.ErrorIs(t, err, types.ErrNoPrice)
}
```

- [ ] **Step 2: Implement feed iteration + EndBlock phase 1**

Add to `keeper.go`:

```go
// IterateAllFeeds scans 0x01; fn receives (valoper, pair, priceStr). Filter by pair in the callback.
func (k Keeper) IterateAllFeeds(ctx context.Context, fn func(val sdk.ValAddress, pair, priceStr string) bool) error
func (k Keeper) DeleteAllFeeds(ctx context.Context) error // collect keys, delete after iter.Close()
func (k Keeper) GetValidatorSubmittedPairs(ctx context.Context, val sdk.ValAddress) (map[string]struct{}, error)
```

Implement `EndBlocker` in `abci.go` with:
- height % vote_window check
- `totalPower := stakingKeeper.GetLastTotalPower(ctx)` (consensus power — spec C1b)
- build valByOper map
- `quorumLive := []string{}` (slice in `accept_list` order — never range a map)
- per-pair aggregation: full `0x01` scan, filter `pair`; quorum via `QuorumMet(submittedPower, totalPower, ...)`
- **skip slashing sections** initially (comment placeholders)
- **do not** call `DeleteAllFeeds` yet (miss logic in Task 20 needs feeds)

Register in `module/module.go` `EndBlock`.

Add imports to `endblock_aggregate_test.go`:

```go
"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
codectypes "github.com/cosmos/cosmos-sdk/codec/types"
```

- [ ] **Step 3: Run tests — PASS**

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(oracle): EndBlock quorum-gated aggregation and TWAP append"
```

---

## Task 19: Outlier slashing in EndBlock

**Files:**
- Modify: `x/oracle/keeper/abci.go`
- Create: `x/oracle/keeper/endblock_outlier_test.go`

- [ ] **Step 1: Write failing test**

4 validators submit 100/100/100/200; outlier_threshold=0.05 → validator with 200 slashed at outlier_slash_rate; MockSlashing.Calls len=1, reason outlier.

- [ ] **Step 2: Add `slashValidator` helper and outlier loop**

```go
func (k Keeper) slashValidator(ctx context.Context, v stakingtypes.Validator, rate math.LegacyDec, reason string) {
	consAddr, err := v.GetConsAddr()
	if err != nil || consAddr == nil {
		return // skip — never panic (spec M1)
	}
	power, err := k.stakingKeeper.GetLastValidatorPower(ctx, sdk.ValAddress(v.OperatorAddress))
	if err != nil || power == 0 {
		return
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	_ = k.slashingKeeper.Slash(sdkCtx, consAddr, rate, power, sdkCtx.BlockHeight())
	// emit oracle_slash event with reason + fraction
}
```

Compare each feed price to `UnweightedMedian(rawPrices)`; if `|price-ref|/ref > threshold`, call `slashValidator` (no Jail). Tests must use `bondingValidator` (ConsensusPubkey set).

- [ ] **Step 3: Run tests — PASS**

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(oracle): slash outlier feeds vs unweighted median in EndBlock"
```

---

## Task 20: Miss tracking and tumbling slash

**Files:**
- Modify: `x/oracle/keeper/abci.go`
- Modify: `x/oracle/keeper/keeper.go` (miss/total counter ops)
- Create: `x/oracle/keeper/endblock_miss_test.go`

- [ ] **Step 1: Write failing tests**

**Test A:** miss_window_size=3, miss_threshold=0.05; validator misses 1 of 3 quorum-live windows → no slash at boundary; misses 2 of 3 → slash.

**Test B:** pair below quorum → not counted in miss check (no mass slash).

- [ ] **Step 2: Implement miss counters**

```go
func (k Keeper) IncMissCounter(ctx context.Context, val sdk.ValAddress) (int64, error)
func (k Keeper) IncTotalWindows(ctx context.Context, val sdk.ValAddress) (int64, error)
func (k Keeper) GetMissCounter(ctx context.Context, val sdk.ValAddress) (int64, error)
func (k Keeper) ResetValidatorWindows(ctx context.Context, val sdk.ValAddress) error
```

EndBlock miss loop per spec §9: for each bonded validator, increment total; if missed any quorum-live pair increment miss; at tumbling boundary evaluate rate, slash if above threshold, reset counters.

`submittedAllLive`: for each pair in **`quorumLive` slice** (iterate slice, not map), check validator submitted via `GetValidatorSubmittedPairs` (KV iteration over `0x01`).

**EndBlock phase order (final):** aggregation + outlier slash → miss tracking + slash → **`DeleteAllFeeds()` last**.

- [ ] **Step 3: Run tests — PASS**

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(oracle): tumbling miss window tracking and slashing in EndBlock"
```

---

## Task 21: gRPC queries and autocli

**Files:**
- Create: `x/oracle/keeper/grpc_query.go`
- Modify: `x/oracle/module/autocli.go`
- Create: `x/oracle/keeper/grpc_query_test.go`

- [ ] **Step 1: Write failing query tests**

Price, Twap, Params, MissCounter, Feeder queries return expected data / errors.

- [ ] **Step 2: Implement grpc_query.go**

Wire each RPC to keeper methods; map `ErrNoPrice`, `ErrStalePrice`, and `ErrNoTWAPData` to appropriate gRPC status codes (`NotFound` / `FailedPrecondition`).

- [ ] **Step 3: Update autocli.go**

Ensure commands: `q oracle price [pair]`, `q oracle twap [pair] [window]`, `q oracle params`, `q oracle miss-counter [valoper]`, `q oracle feeder [valoper]`, `tx oracle set-feeder [feeder]`, `tx oracle submit-feed [validator] [pair] [price]`.

- [ ] **Step 4: Run tests — PASS**

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(oracle): add gRPC queries and autocli wiring"
```

---

## Task 22: Genesis InitGenesis / ExportGenesis

**Files:**
- Replace: `x/oracle/types/genesis.go` (delete Ignite scaffold genesis first)
- Replace: `x/oracle/keeper/genesis.go` (overwrite scaffold)
- Create: `x/oracle/keeper/genesis_test.go`

- [ ] **Step 1: Write failing round-trip test**

```go
func TestGenesisRoundTrip(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	// seed price + feeder delegation
 exported := k.ExportGenesis(ctx)
 ctx2, k2 := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
 k2.InitGenesis(ctx2, exported)
 require.Equal(t, exported, k2.ExportGenesis(ctx2))
}
```

- [ ] **Step 2: Implement types/genesis.go**

```go
func DefaultGenesis() *GenesisState {
 return &GenesisState{Params: DefaultParams()}
}
func (gs GenesisState) Validate() error { /* params + prices + feeders */ }
```

Validate imported prices: pair ∈ accept_list, price > 0, block_time not in future. Validate feeders: valid bech32, valoper exists (if staking wired).

- [ ] **Step 3: Implement keeper/genesis.go**

InitGenesis: validate → SetParams → set aggregated prices → set feeder delegations.
ExportGenesis: params + all `0x02` prices + all `0x08` feeder delegations (not feeds/TWAP/counters).

- [ ] **Step 4: Run tests — PASS**

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(oracle): genesis import/export with validation"
```

---

## Task 23: App wiring and depinject

**Files:**
- Modify: `app/app_config.go` (verify Ignite inserted oracle in endBlockers after IBC, in genesisModuleOrder after staking)
- Modify: `app/app.go` (OracleKeeper field + depinject)
- Replace: `x/oracle/module/depinject.go` (wire real staking/slashing keepers + gov authority)
- Modify: `app/app_test.go`

- [ ] **Step 1: Update `x/oracle/module/depinject.go`**

Ensure `ProvideModule` constructs the keeper with:

```go
k := keeper.NewKeeper(
    in.Cdc,
    in.StoreService,
    in.StakingKeeper,   // must satisfy types.StakingKeeper (includes GetLastTotalPower)
    in.SlashingKeeper,  // must satisfy types.SlashingKeeper
    authtypes.NewModuleAddress(govtypes.ModuleName).String(),
)
```

SDK `x/staking` keeper implements `GetLastTotalPower`; verify compile. Export `OracleKeeper` for Phase 3 injection in `app.go`.

- [ ] **Step 2: Verify app_config.go ordering**

Confirm:
- `oracletypes.ModuleName` in `endBlockers` at `# stargate/app/endBlockers` (first custom module)
- `oracletypes.ModuleName` in `genesisModuleOrder` after `stakingtypes.ModuleName`
- No maccPerms entry for oracle

If Ignite missed markers, add manually per spec §4.

- [ ] **Step 3: Update app_test.go**

```go
func TestOracleModuleWired(t *testing.T) {
	a := newTestApp(t)
	_, ok := a.ModuleManager.Modules["oracle"]
	require.True(t, ok, "x/oracle must be wired")
}

func TestOracleGenesisRoundTrip(t *testing.T) {
	a := newTestApp(t)
	genState := a.DefaultGenesis()
	require.Contains(t, genState, "oracle")
}
```

- [ ] **Step 4: Run app tests**

Run: `go test ./app/ -run 'TestOracle|TestNoMint|TestModulesWired|TestGenesis' -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/
git commit -m "test(app): assert oracle module wiring and genesis"
```

---

## Task 24: Acceptance gate integration tests

**Files:**
- Create: `x/oracle/keeper/acceptance_test.go`

- [ ] **Step 1: Write acceptance tests mapping spec §11**

| Test | Asserts |
|---|---|
| `TestAcceptance_QuorumGatedAggregation` | Below quorum → no price; at quorum → stake-weighted median |
| `TestAcceptance_TWAPAndPrune` | TWAP formula, 24h prune, ErrStalePrice |
| `TestAcceptance_FeederDelegation` | SetFeeder + feeder-signed submit OK; operator rejected when different feeder set |
| `TestAcceptance_MissNoMassSlashOnLowQuorum` | Pair misses quorum → validators not miss-slashed |
| `TestAcceptance_TumblingMissSlash` | Slash only after miss_window_size with rate > threshold |
| `TestAcceptance_OutlierUnweightedMedian` | Outlier band vs count-median, not stake-weighted price |

- [ ] **Step 2: Run full oracle test suite**

Run: `go test ./x/oracle/... -race -count=1 -v`

Expected: all PASS

- [ ] **Step 3: Commit**

```bash
git commit -m "test(oracle): Phase 1 acceptance gate integration tests"
```

---

## Task 25: Full CI verification

**Files:** none (verification only)

- [ ] **Step 1: Lint**

Run: `make lint`

Expected: no findings

- [ ] **Step 2: Test**

Run: `make test`

Expected: all packages pass with `-race -count=1`

- [ ] **Step 3: Build**

Run: `make build`

Expected: `build/vertixd` produced

- [ ] **Step 4: Proto checks**

Run:

```bash
cd proto && buf lint && buf breaking --against '.git#branch=main'
```

Expected: pass (first oracle proto — breaking against main should pass if main has no oracle proto)

- [ ] **Step 5: Genesis validate**

Run:

```bash
make build && ./build/vertixd genesis validate-genesis
```

Expected: valid genesis

- [ ] **Step 6: Manual CLI smoke (optional)**

```bash
./build/vertixd q oracle params
./build/vertixd q oracle price BTC:USD
```

Expected: params returned; price returns error (no aggregation yet) — not a crash

- [ ] **Step 7: Final commit if any fixes needed**

```bash
git commit -m "chore(oracle): Phase 1 CI green — lint test build"
```

---

## Spec Coverage Checklist (self-review)

| Spec requirement | Task |
|---|---|
| D1 Ignite scaffold + adapt | Task 1 |
| D10 Feeder delegation (0x07/0x08) | Tasks 11–13 |
| D11 Quorum-gated aggregation (`GetLastTotalPower`) | Tasks 8–9, 15, 18 |
| C1b Quorum power units | Tasks 8–9, 15, 18 |
| D2 Unweighted outlier reference | Tasks 15, 19 |
| D3 Tumbling miss window | Task 20 |
| D4 Miss only vs quorum-live pairs | Task 20 |
| D5 TWAP append + 24h prune | Tasks 17–18 |
| D12 MaxPriceAge staleness | Tasks 16–17 |
| D7 No jail, D8 no module account | Tasks 19–20, 23 |
| MsgUpdateParams accept_list reset | Task 14 |
| Genesis validation + round-trip | Task 22 |
| OracleKeeper interface | Tasks 8, 16–17 |
| Five queries + CLI | Task 21 |
| depinject keeper wiring | Task 23 |
| Acceptance gate §11 | Tasks 24–25 |
| Events §10 | Tasks 12–14, 19–20 |
| Feed keys length-prefixed | Task 3 |
| EndBlock phase order | Tasks 18, 20 |

**Doc-sync:** Update `technical-design.md` §2.3 `StakingKeeper` to use `GetLastTotalPower` (not `TotalBondedTokens`) when implementing — spec §13 / C1b.
