# x/oracle Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the `x/oracle` Cosmos SDK module — validator-integrated price feed aggregation with stake-weighted median, TWAP storage, oracle slashing, and a clean keeper interface consumed by `x/rwa`.

**Architecture:** Active validators submit `MsgSubmitFeed` each block window via an off-chain sidecar. At the end of each vote window (default 10 blocks), on-chain logic aggregates submissions using a stake-weighted median per denom pair and stores the result. TWAP is computed from historical aggregations. Validators who miss feeds or submit outliers are slashed via `x/slashing`. The module exposes `GetPrice` and `GetTWAP` for `x/rwa` consumption.

**Tech Stack:** Cosmos SDK v0.50.x, Go 1.22+, Ignite CLI v28.x (scaffold), `cosmossdk.io/math`, `x/slashing` keeper integration

**Prerequisite:** Plan 01 (Chain Foundation) must be complete. The chain compiles and boots.

---

## File Structure

```
x/oracle/
├── keeper/
│   ├── keeper.go          — keeper struct, constructor, store accessors
│   ├── feed.go            — MsgSubmitFeed handler, per-validator feed storage
│   ├── aggregator.go      — vote window end: weighted median, store aggregate
│   ├── twap.go            — TWAP ring buffer, GetTWAP computation
│   ├── slash.go           — miss counter tracking, slash trigger logic
│   ├── params.go          — param getter/setter
│   └── keeper_test.go     — unit tests for all keeper logic
├── types/
│   ├── types.go           — OracleFeed, AggregatedPrice, OracleParams structs
│   ├── msgs.go            — MsgSubmitFeed, MsgUpdateParams + ValidateBasic
│   ├── keys.go            — store key prefixes
│   ├── errors.go          — sentinel errors
│   ├── events.go          — event type constants
│   └── params.go          — OracleParams defaults and validation
├── client/
│   └── cli/
│       ├── tx.go          — CLI: submit-feed, update-params
│       └── query.go       — CLI: price, twap, params, miss-counters
├── module.go              — AppModule, RegisterServices, BeginBlock/EndBlock
├── genesis.go             — InitGenesis, ExportGenesis
├── abci.go                — EndBlock aggregation and slash hook
└── proto/vertix/oracle/
    ├── v1/
    │   ├── types.proto    — OracleFeed, AggregatedPrice, TWAPEntry, OracleParams
    │   ├── tx.proto       — MsgSubmitFeed, MsgUpdateParams
    │   ├── query.proto    — QueryGetPrice, QueryGetTWAP, QueryGetParams
    │   └── genesis.proto  — GenesisState
```

**Modified files:**
- `app/app.go` — add OracleKeeper, wire x/oracle module
- `app/app_test.go` — add oracle module presence test

---

## Task 1: Scaffold Oracle Module

**Files:** `x/oracle/` (generated skeleton)

- [ ] **Step 1: Scaffold the module**

```bash
ignite scaffold module oracle --dep staking,slashing,params
```

Expected: `x/oracle/` directory created with skeleton `module.go`, `keeper/`, `types/`, `client/cli/`, `proto/`.

- [ ] **Step 2: Verify app still compiles**

```bash
go build ./...
```

Expected: No errors. If scaffold added mint-related references, remove them.

---

## Task 2: Define Protobuf Types

**Files:**
- Create: `proto/vertix/oracle/v1/types.proto`
- Create: `proto/vertix/oracle/v1/tx.proto`
- Create: `proto/vertix/oracle/v1/query.proto`
- Create: `proto/vertix/oracle/v1/genesis.proto`

- [ ] **Step 1: Write types.proto**

```protobuf
syntax = "proto3";
package vertix.oracle.v1;

import "gogoproto/gogo.proto";
import "cosmos_proto/cosmos.proto";
import "google/protobuf/timestamp.proto";

option go_package = "github.com/vertix-network/vertix/x/oracle/types";

// OracleFeed is a single validator price submission for one denom pair.
message OracleFeed {
  string validator = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string pair      = 2;
  string price     = 3 [
    (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec",
    (gogoproto.nullable)   = false
  ];
  int64 block_height = 4;
}

// AggregatedPrice is the result of a vote window aggregation.
message AggregatedPrice {
  string pair  = 1;
  string price = 2 [
    (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec",
    (gogoproto.nullable)   = false
  ];
  int64 block_height = 3;
  google.protobuf.Timestamp timestamp = 4 [
    (gogoproto.stdtime) = true,
    (gogoproto.nullable) = false
  ];
}

// TWAPEntry stores a price snapshot for TWAP computation.
message TWAPEntry {
  string pair  = 1;
  string price = 2 [
    (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec",
    (gogoproto.nullable)   = false
  ];
  google.protobuf.Timestamp timestamp = 3 [
    (gogoproto.stdtime) = true,
    (gogoproto.nullable) = false
  ];
}

// OracleParams defines the governance parameters for x/oracle.
message OracleParams {
  int64  vote_window       = 1; // blocks per aggregation window
  string miss_threshold    = 2 [
    (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec",
    (gogoproto.nullable)   = false
  ];
  string miss_slash_rate   = 3 [
    (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec",
    (gogoproto.nullable)   = false
  ];
  string outlier_slash_rate = 4 [
    (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec",
    (gogoproto.nullable)   = false
  ];
  repeated string accept_list = 5; // whitelisted denom pairs
}
```

- [ ] **Step 2: Write tx.proto**

```protobuf
syntax = "proto3";
package vertix.oracle.v1;

import "cosmos/msg/v1/msg.proto";
import "cosmos_proto/cosmos.proto";
import "vertix/oracle/v1/types.proto";

option go_package = "github.com/vertix-network/vertix/x/oracle/types";

service Msg {
  option (cosmos.msg.v1.service) = true;

  rpc SubmitFeed(MsgSubmitFeed) returns (MsgSubmitFeedResponse);
  rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);
}

// MsgSubmitFeed is sent by a validator each block to submit price data.
message MsgSubmitFeed {
  option (cosmos.msg.v1.signer) = "validator";
  string validator = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string pair      = 2;
  string price     = 3;
}

message MsgSubmitFeedResponse {}

// MsgUpdateParams is a governance proposal message to update oracle params.
message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";
  string       authority = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  OracleParams params    = 2;
}

message MsgUpdateParamsResponse {}
```

- [ ] **Step 3: Write query.proto**

```protobuf
syntax = "proto3";
package vertix.oracle.v1;

import "google/api/annotations.proto";
import "vertix/oracle/v1/types.proto";

option go_package = "github.com/vertix-network/vertix/x/oracle/types";

service Query {
  rpc GetPrice(QueryGetPriceRequest) returns (QueryGetPriceResponse) {
    option (google.api.http).get = "/vertix/oracle/v1/price/{pair}";
  }
  rpc GetTWAP(QueryGetTWAPRequest) returns (QueryGetTWAPResponse) {
    option (google.api.http).get = "/vertix/oracle/v1/twap/{pair}/{window_seconds}";
  }
  rpc GetParams(QueryGetParamsRequest) returns (QueryGetParamsResponse) {
    option (google.api.http).get = "/vertix/oracle/v1/params";
  }
  rpc GetMissCounter(QueryGetMissCounterRequest) returns (QueryGetMissCounterResponse) {
    option (google.api.http).get = "/vertix/oracle/v1/miss/{validator}";
  }
}

message QueryGetPriceRequest  { string pair = 1; }
message QueryGetPriceResponse { AggregatedPrice price = 1; }

message QueryGetTWAPRequest   { string pair = 1; int64 window_seconds = 2; }
message QueryGetTWAPResponse  { string twap = 1; }

message QueryGetParamsRequest  {}
message QueryGetParamsResponse { OracleParams params = 1; }

message QueryGetMissCounterRequest  { string validator = 1; }
message QueryGetMissCounterResponse { int64 miss_count = 1; int64 total_windows = 2; }
```

- [ ] **Step 4: Write genesis.proto**

```protobuf
syntax = "proto3";
package vertix.oracle.v1;

import "vertix/oracle/v1/types.proto";

option go_package = "github.com/vertix-network/vertix/x/oracle/types";

message GenesisState {
  OracleParams           params  = 1;
  repeated AggregatedPrice prices = 2;
}
```

- [ ] **Step 5: Generate Go code from proto**

```bash
ignite generate proto-go
```

Expected: Go types generated under `x/oracle/types/` from protobuf definitions.

- [ ] **Step 6: Verify compilation**

```bash
go build ./...
```

---

## Task 3: Implement Types Package

**Files:**
- Modify: `x/oracle/types/types.go`
- Modify: `x/oracle/types/keys.go`
- Modify: `x/oracle/types/errors.go`
- Modify: `x/oracle/types/events.go`
- Modify: `x/oracle/types/params.go`

- [ ] **Step 1: Write keys.go**

```go
package types

const (
	ModuleName = "oracle"
	StoreKey   = ModuleName
	RouterKey  = ModuleName
)

var (
	KeyPrefixFeed         = []byte{0x01} // {validator}{pair} -> OracleFeed
	KeyPrefixAggPrice     = []byte{0x02} // {pair} -> AggregatedPrice
	KeyPrefixTWAPHistory  = []byte{0x03} // {pair}{timestamp} -> TWAPEntry
	KeyPrefixMissCounter  = []byte{0x04} // {validator} -> int64 miss count
	KeyPrefixWindowCount  = []byte{0x05} // [] -> int64 total completed windows
	KeyParams             = []byte{0x06}
)

// FeedKey returns the store key for a validator+pair feed.
func FeedKey(validator, pair string) []byte {
	return append(KeyPrefixFeed, []byte(validator+"/"+pair)...)
}

// AggPriceKey returns the store key for an aggregated price.
func AggPriceKey(pair string) []byte {
	return append(KeyPrefixAggPrice, []byte(pair)...)
}

// MissCounterKey returns the store key for a validator's miss counter.
func MissCounterKey(validator string) []byte {
	return append(KeyPrefixMissCounter, []byte(validator)...)
}
```

- [ ] **Step 2: Write errors.go**

```go
package types

import "cosmossdk.io/errors"

var (
	ErrInvalidPair      = errors.Register(ModuleName, 1, "invalid denom pair")
	ErrPairNotAccepted  = errors.Register(ModuleName, 2, "denom pair not in accept list")
	ErrInvalidPrice     = errors.Register(ModuleName, 3, "invalid price: must be positive")
	ErrNotValidator     = errors.Register(ModuleName, 4, "submitter is not an active validator")
	ErrPriceNotFound    = errors.Register(ModuleName, 5, "price not found for pair")
	ErrInsufficientData = errors.Register(ModuleName, 6, "insufficient data for TWAP computation")
)
```

- [ ] **Step 3: Write events.go**

```go
package types

const (
	EventTypeFeedSubmitted    = "oracle_feed_submitted"
	EventTypePriceAggregated  = "oracle_price_aggregated"
	EventTypeOracleSlash      = "oracle_slash"

	AttributeKeyValidator    = "validator"
	AttributeKeyPair         = "pair"
	AttributeKeyPrice        = "price"
	AttributeKeySlashReason  = "slash_reason"
	AttributeKeySlashAmount  = "slash_amount"
	AttributeKeyWindowNumber = "window_number"
)
```

- [ ] **Step 4: Write params.go (defaults)**

```go
package types

import (
	"cosmossdk.io/math"
)

var (
	DefaultVoteWindow      int64              = 10
	DefaultMissThreshold                      = math.LegacyMustNewDecFromStr("0.05")
	DefaultMissSlashRate                      = math.LegacyMustNewDecFromStr("0.005")
	DefaultOutlierSlashRate                   = math.LegacyMustNewDecFromStr("0.01")
	DefaultAcceptList                         = []string{
		"VTX:USD", "BTC:USD", "ETH:USD", "ATOM:USD", "USDC:USD",
	}
)

func DefaultParams() OracleParams {
	return OracleParams{
		VoteWindow:       DefaultVoteWindow,
		MissThreshold:    DefaultMissThreshold,
		MissSlashRate:    DefaultMissSlashRate,
		OutlierSlashRate: DefaultOutlierSlashRate,
		AcceptList:       DefaultAcceptList,
	}
}

func (p OracleParams) Validate() error {
	if p.VoteWindow <= 0 {
		return ErrInvalidPair.Wrapf("vote_window must be > 0, got %d", p.VoteWindow)
	}
	if p.MissThreshold.IsNegative() || p.MissThreshold.GT(math.LegacyOneDec()) {
		return ErrInvalidPrice.Wrapf("miss_threshold must be in [0,1], got %s", p.MissThreshold)
	}
	if p.MissSlashRate.IsNegative() || p.MissSlashRate.GT(math.LegacyOneDec()) {
		return ErrInvalidPrice.Wrapf("miss_slash_rate must be in [0,1], got %s", p.MissSlashRate)
	}
	if p.OutlierSlashRate.IsNegative() || p.OutlierSlashRate.GT(math.LegacyOneDec()) {
		return ErrInvalidPrice.Wrapf("outlier_slash_rate must be in [0,1], got %s", p.OutlierSlashRate)
	}
	return nil
}
```

- [ ] **Step 5: Write msgs.go ValidateBasic implementations**

```go
package types

import (
	"strings"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"cosmossdk.io/math"
)

var _ sdk.Msg = &MsgSubmitFeed{}

func (m *MsgSubmitFeed) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Validator); err != nil {
		return ErrNotValidator.Wrapf("invalid validator address: %s", err)
	}
	parts := strings.Split(m.Pair, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ErrInvalidPair.Wrapf("pair must be in format BASE:QUOTE, got %q", m.Pair)
	}
	price, err := math.LegacyNewDecFromStr(m.Price)
	if err != nil || price.IsNegative() || price.IsZero() {
		return ErrInvalidPrice.Wrapf("price must be a positive decimal, got %q", m.Price)
	}
	return nil
}

var _ sdk.Msg = &MsgUpdateParams{}

func (m *MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return ErrNotValidator.Wrapf("invalid authority address: %s", err)
	}
	return m.Params.Validate()
}
```

- [ ] **Step 6: Run unit tests for types (write then run)**

Create `x/oracle/types/types_test.go`:

```go
package types_test

import (
	"testing"
	"github.com/stretchr/testify/require"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestMsgSubmitFeedValidateBasic(t *testing.T) {
	cases := []struct {
		name    string
		msg     types.MsgSubmitFeed
		wantErr bool
	}{
		{
			name:    "valid",
			msg:     types.MsgSubmitFeed{Validator: "vtx1qnk2n4nlkpw9xfqntladh74er2xa62wdl82dyn", Pair: "VTX:USD", Price: "1.25"},
			wantErr: false,
		},
		{
			name:    "bad pair format",
			msg:     types.MsgSubmitFeed{Validator: "vtx1qnk2n4nlkpw9xfqntladh74er2xa62wdl82dyn", Pair: "VTXUSD", Price: "1.25"},
			wantErr: true,
		},
		{
			name:    "zero price",
			msg:     types.MsgSubmitFeed{Validator: "vtx1qnk2n4nlkpw9xfqntladh74er2xa62wdl82dyn", Pair: "VTX:USD", Price: "0"},
			wantErr: true,
		},
		{
			name:    "negative price",
			msg:     types.MsgSubmitFeed{Validator: "vtx1qnk2n4nlkpw9xfqntladh74er2xa62wdl82dyn", Pair: "VTX:USD", Price: "-1.0"},
			wantErr: true,
		},
		{
			name:    "invalid validator address",
			msg:     types.MsgSubmitFeed{Validator: "notanaddress", Pair: "VTX:USD", Price: "1.25"},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParamsValidate(t *testing.T) {
	p := types.DefaultParams()
	require.NoError(t, p.Validate())

	bad := p
	bad.VoteWindow = 0
	require.Error(t, bad.Validate())
}
```

```bash
go test ./x/oracle/types/... -v
```

Expected: All tests PASS.

---

## Task 4: Implement Keeper

**Files:**
- Create: `x/oracle/keeper/keeper.go`
- Create: `x/oracle/keeper/feed.go`
- Create: `x/oracle/keeper/aggregator.go`
- Create: `x/oracle/keeper/twap.go`
- Create: `x/oracle/keeper/slash.go`
- Create: `x/oracle/keeper/params.go`

- [ ] **Step 1: Write failing keeper tests first**

Create `x/oracle/keeper/keeper_test.go`:

```go
package keeper_test

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	keepertest "github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestSetGetFeed(t *testing.T) {
	ctx, k := keepertest.OracleKeeper(t)
	validator := sdk.AccAddress([]byte("validator1")).String()
	feed := types.OracleFeed{
		Validator:   validator,
		Pair:        "VTX:USD",
		Price:       math.LegacyMustNewDecFromStr("1.25"),
		BlockHeight: 10,
	}
	k.SetFeed(ctx, feed)
	got, found := k.GetFeed(ctx, validator, "VTX:USD")
	require.True(t, found)
	require.Equal(t, feed.Price, got.Price)
}

func TestWeightedMedianAggregation(t *testing.T) {
	ctx, k := keepertest.OracleKeeper(t)

	// Three validators submit feeds; result should be weighted median
	k.SetFeed(ctx, types.OracleFeed{Validator: "val1", Pair: "VTX:USD", Price: math.LegacyMustNewDecFromStr("1.00")})
	k.SetFeed(ctx, types.OracleFeed{Validator: "val2", Pair: "VTX:USD", Price: math.LegacyMustNewDecFromStr("1.20")})
	k.SetFeed(ctx, types.OracleFeed{Validator: "val3", Pair: "VTX:USD", Price: math.LegacyMustNewDecFromStr("1.40")})

	// Simulate aggregation with equal weights
	prices := []math.LegacyDec{
		math.LegacyMustNewDecFromStr("1.00"),
		math.LegacyMustNewDecFromStr("1.20"),
		math.LegacyMustNewDecFromStr("1.40"),
	}
	weights := []math.Int{
		math.NewInt(100),
		math.NewInt(100),
		math.NewInt(100),
	}

	result := k.WeightedMedian(prices, weights)
	require.Equal(t, math.LegacyMustNewDecFromStr("1.20"), result)
}

func TestGetSetAggregatedPrice(t *testing.T) {
	ctx, k := keepertest.OracleKeeper(t)
	price := types.AggregatedPrice{
		Pair:        "VTX:USD",
		Price:       math.LegacyMustNewDecFromStr("1.25"),
		BlockHeight: 10,
		Timestamp:   time.Now().UTC(),
	}
	k.SetAggregatedPrice(ctx, price)
	got, err := k.GetPrice(ctx, "VTX:USD")
	require.NoError(t, err)
	require.Equal(t, price.Price, got)
}

func TestMissCounter(t *testing.T) {
	ctx, k := keepertest.OracleKeeper(t)
	validator := "vtx1validator1"

	require.Equal(t, int64(0), k.GetMissCounter(ctx, validator))
	k.IncrementMissCounter(ctx, validator)
	k.IncrementMissCounter(ctx, validator)
	require.Equal(t, int64(2), k.GetMissCounter(ctx, validator))
	k.ResetMissCounter(ctx, validator)
	require.Equal(t, int64(0), k.GetMissCounter(ctx, validator))
}
```

```bash
go test ./x/oracle/keeper/... -v 2>&1 | head -20
```

Expected: FAIL (keeper and testutil not yet implemented).

- [ ] **Step 2: Create testutil/keeper/oracle.go**

```bash
mkdir -p testutil/keeper
```

Create `testutil/keeper/oracle.go`:

```go
package keeper

import (
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/oracle/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

// OracleKeeper returns a minimal keeper and context for unit tests.
func OracleKeeper(t *testing.T) (sdk.Context, keeper.Keeper) {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	k := keeper.NewKeeper(
		cdc,
		storetypes.NewKVStoreService(storeKey),
		nil, // staking keeper — nil for unit tests
		nil, // slashing keeper — nil for unit tests
		"vtx10d07y265gmmuvt4z0w9aw880jnsr700jxh8nkv", // gov module address
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())
	// Set default params
	require.NoError(t, k.SetParams(ctx, types.DefaultParams()))

	return ctx, k
}
```

- [ ] **Step 3: Write keeper.go**

```go
package keeper

import (
	"cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"
)

type StakingKeeper interface {
	GetValidatorByConsAddr(ctx context.Context, addr sdk.ConsAddress) (stakingtypes.Validator, error)
	GetLastValidatorPower(ctx context.Context, addr sdk.ValAddress) (int64, error)
	GetBondedValidatorsByPower(ctx context.Context) ([]stakingtypes.Validator, error)
}

type SlashingKeeper interface {
	Slash(ctx context.Context, consAddr sdk.ConsAddress, fraction math.LegacyDec, power, distributionHeight int64) (math.Int, error)
	Jail(ctx context.Context, consAddr sdk.ConsAddress) error
}

type Keeper struct {
	cdc            codec.BinaryCodec
	storeService   store.KVStoreService
	stakingKeeper  StakingKeeper
	slashingKeeper SlashingKeeper
	authority      string // gov module address
}

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	stakingKeeper StakingKeeper,
	slashingKeeper SlashingKeeper,
	authority string,
) Keeper {
	return Keeper{
		cdc:            cdc,
		storeService:   storeService,
		stakingKeeper:  stakingKeeper,
		slashingKeeper: slashingKeeper,
		authority:      authority,
	}
}

func (k Keeper) GetAuthority() string { return k.authority }
```

- [ ] **Step 4: Write feed.go**

```go
package keeper

import (
	"context"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/vertix-network/vertix/x/oracle/types"
)

// SetFeed stores a validator price feed submission.
func (k Keeper) SetFeed(ctx context.Context, feed types.OracleFeed) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.FeedKey(feed.Validator, feed.Pair)
	bz := k.cdc.MustMarshal(&feed)
	store.Set(key, bz)
}

// GetFeed retrieves a validator's latest feed for a pair. Returns false if not found.
func (k Keeper) GetFeed(ctx context.Context, validator, pair string) (types.OracleFeed, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.FeedKey(validator, pair))
	if err != nil || bz == nil {
		return types.OracleFeed{}, false
	}
	var feed types.OracleFeed
	k.cdc.MustUnmarshal(bz, &feed)
	return feed, true
}

// GetAllFeedsForPair returns all validator feeds for a given pair in the current window.
func (k Keeper) GetAllFeedsForPair(ctx context.Context, pair string) []types.OracleFeed {
	store := k.storeService.OpenKVStore(ctx)
	iter, _ := store.Iterator(types.KeyPrefixFeed, storetypes.PrefixEndBytes(types.KeyPrefixFeed))
	defer iter.Close()

	var feeds []types.OracleFeed
	for ; iter.Valid(); iter.Next() {
		var feed types.OracleFeed
		k.cdc.MustUnmarshal(iter.Value(), &feed)
		if feed.Pair == pair {
			feeds = append(feeds, feed)
		}
	}
	return feeds
}

// DeleteAllFeeds clears all feeds from the store (called after each vote window ends).
func (k Keeper) DeleteAllFeeds(ctx context.Context) {
	store := k.storeService.OpenKVStore(ctx)
	iter, _ := store.Iterator(types.KeyPrefixFeed, storetypes.PrefixEndBytes(types.KeyPrefixFeed))
	defer iter.Close()

	var keys [][]byte
	for ; iter.Valid(); iter.Next() {
		keys = append(keys, iter.Key())
	}
	for _, key := range keys {
		store.Delete(key)
	}
}
```

- [ ] **Step 5: Write aggregator.go**

```go
package keeper

import (
	"context"
	"sort"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/vertix-network/vertix/x/oracle/types"
)

// AggregateAllPairs runs the weighted median aggregation for all accepted pairs.
// Called at the end of each vote window in EndBlock.
func (k Keeper) AggregateAllPairs(ctx context.Context) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params, _ := k.GetParams(ctx)

	for _, pair := range params.AcceptList {
		feeds := k.GetAllFeedsForPair(ctx, pair)
		if len(feeds) == 0 {
			continue
		}

		prices := make([]math.LegacyDec, len(feeds))
		weights := make([]math.Int, len(feeds))
		for i, feed := range feeds {
			prices[i] = feed.Price
			// Use 1 as weight if staking keeper is nil (test mode)
			weights[i] = math.NewInt(1)
			if k.stakingKeeper != nil {
				valAddr, err := sdk.ValAddressFromBech32(feed.Validator)
				if err == nil {
					power, err := k.stakingKeeper.GetLastValidatorPower(ctx, valAddr)
					if err == nil {
						weights[i] = math.NewInt(power)
					}
				}
			}
		}

		median := k.WeightedMedian(prices, weights)
		agg := types.AggregatedPrice{
			Pair:        pair,
			Price:       median,
			BlockHeight: sdkCtx.BlockHeight(),
			Timestamp:   sdkCtx.BlockTime(),
		}
		k.SetAggregatedPrice(ctx, agg)

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventTypePriceAggregated,
			sdk.NewAttribute(types.AttributeKeyPair, pair),
			sdk.NewAttribute(types.AttributeKeyPrice, median.String()),
		))
	}
}

// WeightedMedian computes the stake-weighted median of a price slice.
// prices and weights must be the same length.
func (k Keeper) WeightedMedian(prices []math.LegacyDec, weights []math.Int) math.LegacyDec {
	if len(prices) == 0 {
		return math.LegacyZeroDec()
	}

	type pw struct {
		price  math.LegacyDec
		weight math.Int
	}
	pairs := make([]pw, len(prices))
	totalWeight := math.ZeroInt()
	for i := range prices {
		pairs[i] = pw{prices[i], weights[i]}
		totalWeight = totalWeight.Add(weights[i])
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].price.LT(pairs[j].price)
	})

	halfWeight := totalWeight.QuoRaw(2)
	cumWeight := math.ZeroInt()
	for _, p := range pairs {
		cumWeight = cumWeight.Add(p.weight)
		if cumWeight.GTE(halfWeight) {
			return p.price
		}
	}
	return pairs[len(pairs)-1].price
}

// SetAggregatedPrice stores an aggregated price result.
func (k Keeper) SetAggregatedPrice(ctx context.Context, price types.AggregatedPrice) {
	store := k.storeService.OpenKVStore(ctx)
	bz := k.cdc.MustMarshal(&price)
	store.Set(types.AggPriceKey(price.Pair), bz)

	// Also append to TWAP history
	k.AppendTWAPEntry(ctx, types.TWAPEntry{
		Pair:      price.Pair,
		Price:     price.Price,
		Timestamp: price.Timestamp,
	})
}

// GetPrice returns the latest aggregated spot price for a pair.
func (k Keeper) GetPrice(ctx context.Context, pair string) (math.LegacyDec, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.AggPriceKey(pair))
	if err != nil || bz == nil {
		return math.LegacyZeroDec(), types.ErrPriceNotFound.Wrapf("pair: %s", pair)
	}
	var agg types.AggregatedPrice
	k.cdc.MustUnmarshal(bz, &agg)
	return agg.Price, nil
}
```

- [ ] **Step 6: Write twap.go**

```go
package keeper

import (
	"context"
	"encoding/binary"
	"time"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/vertix-network/vertix/x/oracle/types"
)

// AppendTWAPEntry stores a price snapshot keyed by pair + timestamp.
func (k Keeper) AppendTWAPEntry(ctx context.Context, entry types.TWAPEntry) {
	store := k.storeService.OpenKVStore(ctx)
	ts := uint64(entry.Timestamp.UnixNano())
	tsBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(tsBytes, ts)
	key := append(append(types.KeyPrefixTWAPHistory, []byte(entry.Pair+"/")...), tsBytes...)
	bz := k.cdc.MustMarshal(&entry)
	store.Set(key, bz)
}

// GetTWAP computes the time-weighted average price over [now-window, now].
func (k Keeper) GetTWAP(ctx context.Context, pair string, window time.Duration) (math.LegacyDec, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime()
	cutoff := now.Add(-window)

	store := k.storeService.OpenKVStore(ctx)
	prefix := append(types.KeyPrefixTWAPHistory, []byte(pair+"/")...)
	iter, _ := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer iter.Close()

	var entries []types.TWAPEntry
	for ; iter.Valid(); iter.Next() {
		var e types.TWAPEntry
		k.cdc.MustUnmarshal(iter.Value(), &e)
		if e.Timestamp.After(cutoff) {
			entries = append(entries, e)
		}
	}

	if len(entries) == 0 {
		return math.LegacyZeroDec(), types.ErrInsufficientData.Wrapf(
			"no price data for %s in the last %s", pair, window,
		)
	}

	// Simple average over window entries (approximates TWAP)
	sum := math.LegacyZeroDec()
	for _, e := range entries {
		sum = sum.Add(e.Price)
	}
	return sum.QuoInt64(int64(len(entries))), nil
}
```

- [ ] **Step 7: Write slash.go**

```go
package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/vertix-network/vertix/x/oracle/types"
)

// GetMissCounter returns the miss count for a validator.
func (k Keeper) GetMissCounter(ctx context.Context, validator string) int64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.MissCounterKey(validator))
	if err != nil || bz == nil {
		return 0
	}
	return int64(binary.BigEndian.Uint64(bz))
}

// IncrementMissCounter adds 1 to a validator's miss counter.
func (k Keeper) IncrementMissCounter(ctx context.Context, validator string) {
	count := k.GetMissCounter(ctx, validator) + 1
	k.setMissCounter(ctx, validator, count)
}

// ResetMissCounter resets a validator's miss counter to 0.
func (k Keeper) ResetMissCounter(ctx context.Context, validator string) {
	k.setMissCounter(ctx, validator, 0)
}

func (k Keeper) setMissCounter(ctx context.Context, validator string, count int64) {
	store := k.storeService.OpenKVStore(ctx)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, uint64(count))
	store.Set(types.MissCounterKey(validator), bz)
}

// ProcessMissesAndSlash checks all active validators after a window and slashes those
// who exceeded the miss threshold. Called from EndBlock after aggregation.
func (k Keeper) ProcessMissesAndSlash(ctx context.Context, windowNumber int64) {
	if k.stakingKeeper == nil || k.slashingKeeper == nil {
		return // skip in unit test mode
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params, _ := k.GetParams(ctx)

	validators, err := k.stakingKeeper.GetBondedValidatorsByPower(ctx)
	if err != nil {
		return
	}

	for _, val := range validators {
		valAddr := val.GetOperator()
		missCount := k.GetMissCounter(ctx, valAddr)
		threshold := int64(params.MissThreshold.MulInt64(windowNumber).TruncateInt64())

		if missCount > threshold {
			consAddr, err := val.GetConsAddr()
			if err != nil {
				continue
			}
			power, _ := k.stakingKeeper.GetLastValidatorPower(ctx, sdk.ValAddress(valAddr))
			_, err = k.slashingKeeper.Slash(ctx, consAddr, params.MissSlashRate, power, sdkCtx.BlockHeight())
			if err == nil {
				sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
					types.EventTypeOracleSlash,
					sdk.NewAttribute(types.AttributeKeyValidator, valAddr),
					sdk.NewAttribute(types.AttributeKeySlashReason, "miss_threshold_exceeded"),
					sdk.NewAttribute(types.AttributeKeySlashAmount, params.MissSlashRate.String()),
				))
			}
		}
		k.ResetMissCounter(ctx, valAddr)
	}
}
```

- [ ] **Step 8: Write params.go in keeper**

```go
package keeper

import (
	"context"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func (k Keeper) GetParams(ctx context.Context) (types.OracleParams, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyParams)
	if err != nil || bz == nil {
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
	bz := k.cdc.MustMarshal(&p)
	return store.Set(types.KeyParams, bz)
}
```

- [ ] **Step 9: Run keeper tests**

```bash
go test ./x/oracle/keeper/... -v
```

Expected: All tests PASS. Fix any compilation errors before continuing.

---

## Task 5: Implement EndBlock (ABCI Hook)

**Files:**
- Create: `x/oracle/abci.go`

- [ ] **Step 1: Write abci.go**

```go
package oracle

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/vertix-network/vertix/x/oracle/keeper"
)

// EndBlocker is called at the end of every block.
// If the current block height is a multiple of VoteWindow, it:
//  1. Aggregates all submitted feeds per accepted pair
//  2. Checks for validator misses and slashes accordingly
//  3. Clears all feed submissions for the next window
func EndBlocker(ctx context.Context, k keeper.Keeper) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params, err := k.GetParams(ctx)
	if err != nil {
		return
	}

	if sdkCtx.BlockHeight()%params.VoteWindow != 0 {
		return // not end of vote window
	}

	windowNumber := sdkCtx.BlockHeight() / params.VoteWindow

	// 1. Aggregate prices for all accepted pairs
	k.AggregateAllPairs(ctx)

	// 2. Process misses and slash validators
	k.ProcessMissesAndSlash(ctx, windowNumber)

	// 3. Clear feed submissions for next window
	k.DeleteAllFeeds(ctx)
}
```

- [ ] **Step 2: Write abci_test.go**

Create `x/oracle/abci_test.go`:

```go
package oracle_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"cosmossdk.io/math"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	oracle "github.com/vertix-network/vertix/x/oracle"
	keepertest "github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestEndBlockerAggregatesAtWindowEnd(t *testing.T) {
	ctx, k := keepertest.OracleKeeper(t)

	// Submit feeds from 3 validators
	k.SetFeed(ctx, types.OracleFeed{Validator: "val1", Pair: "VTX:USD", Price: math.LegacyMustNewDecFromStr("1.00")})
	k.SetFeed(ctx, types.OracleFeed{Validator: "val2", Pair: "VTX:USD", Price: math.LegacyMustNewDecFromStr("1.20")})
	k.SetFeed(ctx, types.OracleFeed{Validator: "val3", Pair: "VTX:USD", Price: math.LegacyMustNewDecFromStr("1.40")})

	// Advance to block 10 (first vote window end with VoteWindow=10)
	ctx = ctx.WithBlockHeight(10)
	oracle.EndBlocker(ctx, k)

	price, err := k.GetPrice(ctx, "VTX:USD")
	require.NoError(t, err)
	require.Equal(t, math.LegacyMustNewDecFromStr("1.20"), price, "should be weighted median")

	// Feeds should be cleared after window
	feeds := k.GetAllFeedsForPair(ctx, "VTX:USD")
	require.Empty(t, feeds, "feeds should be cleared after aggregation")
}

func TestEndBlockerNoOpMidWindow(t *testing.T) {
	ctx, k := keepertest.OracleKeeper(t)
	k.SetFeed(ctx, types.OracleFeed{Validator: "val1", Pair: "VTX:USD", Price: math.LegacyMustNewDecFromStr("1.00")})

	// Block 5 — mid window, no aggregation
	ctx = ctx.WithBlockHeight(5)
	oracle.EndBlocker(ctx, k)

	_, err := k.GetPrice(ctx, "VTX:USD")
	require.Error(t, err, "price should not be set mid-window")
}
```

```bash
go test ./x/oracle/... -run TestEndBlocker -v
```

Expected: Both PASS.

---

## Task 6: Implement Msg Server and Wire Module

**Files:**
- Create: `x/oracle/keeper/msg_server.go`
- Modify: `x/oracle/module.go`
- Modify: `x/oracle/genesis.go`

- [ ] **Step 1: Write msg_server.go**

```go
package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/vertix-network/vertix/x/oracle/types"
)

type msgServer struct{ Keeper }

func NewMsgServer(k Keeper) types.MsgServer { return msgServer{k} }

func (m msgServer) SubmitFeed(ctx context.Context, msg *types.MsgSubmitFeed) (*types.MsgSubmitFeedResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	params, _ := m.GetParams(ctx)
	accepted := false
	for _, p := range params.AcceptList {
		if p == msg.Pair {
			accepted = true
			break
		}
	}
	if !accepted {
		return nil, types.ErrPairNotAccepted.Wrapf("pair %q not in accept list", msg.Pair)
	}

	price, err := math.LegacyNewDecFromStr(msg.Price)
	if err != nil {
		return nil, types.ErrInvalidPrice.Wrap(err.Error())
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	feed := types.OracleFeed{
		Validator:   msg.Validator,
		Pair:        msg.Pair,
		Price:       price,
		BlockHeight: sdkCtx.BlockHeight(),
	}
	m.SetFeed(ctx, feed)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeFeedSubmitted,
		sdk.NewAttribute(types.AttributeKeyValidator, msg.Validator),
		sdk.NewAttribute(types.AttributeKeyPair, msg.Pair),
		sdk.NewAttribute(types.AttributeKeyPrice, msg.Price),
	))

	return &types.MsgSubmitFeedResponse{}, nil
}

func (m msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if msg.Authority != m.GetAuthority() {
		return nil, types.ErrNotValidator.Wrapf("expected authority %s, got %s", m.GetAuthority(), msg.Authority)
	}
	if err := m.SetParams(ctx, msg.Params); err != nil {
		return nil, err
	}
	return &types.MsgUpdateParamsResponse{}, nil
}
```

- [ ] **Step 2: Update module.go to wire EndBlock and register services**

In `x/oracle/module.go`, update the `EndBlock` and `RegisterServices` methods:

```go
// In AppModule.EndBlock:
func (am AppModule) EndBlock(ctx context.Context) error {
	EndBlocker(ctx, am.keeper)
	return nil
}

// In AppModule.RegisterServices:
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServer(am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.NewQueryServer(am.keeper))
}
```

- [ ] **Step 3: Update genesis.go**

```go
package oracle

import (
	"context"
	"github.com/vertix-network/vertix/x/oracle/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func InitGenesis(ctx context.Context, k keeper.Keeper, gs types.GenesisState) {
	if err := k.SetParams(ctx, gs.Params); err != nil {
		panic(err)
	}
	for _, price := range gs.Prices {
		k.SetAggregatedPrice(ctx, price)
	}
}

func ExportGenesis(ctx context.Context, k keeper.Keeper) *types.GenesisState {
	params, _ := k.GetParams(ctx)
	return &types.GenesisState{Params: params}
}

func DefaultGenesis() *types.GenesisState {
	return &types.GenesisState{Params: types.DefaultParams()}
}
```

- [ ] **Step 4: Run full oracle test suite**

```bash
go test ./x/oracle/... -v -timeout 2m
```

Expected: All tests PASS.

---

## Task 7: Wire x/oracle into app/app.go

**Files:**
- Modify: `app/app.go`
- Modify: `app/app_test.go`

- [ ] **Step 1: Write failing test**

Add to `app/app_test.go`:

```go
func TestOracleModuleRegistered(t *testing.T) {
	db := dbm.NewMemDB()
	vertixApp := app.New(
		log.NewNopLogger(), db, nil, true,
		map[int64]bool{}, app.DefaultNodeHome, 0,
		app.MakeEncodingConfig(), app.EmptyAppOptions{},
	)
	_, hasOracle := vertixApp.ModuleManager.Modules["oracle"]
	require.True(t, hasOracle, "x/oracle must be registered")
}
```

```bash
go test ./app/... -run TestOracleModuleRegistered -v
```

Expected: FAIL (oracle not wired yet).

- [ ] **Step 2: Add OracleKeeper to App struct in app/app.go**

```go
// Add to App struct:
OracleKeeper oraclekeeper.Keeper
```

- [ ] **Step 3: Initialize OracleKeeper in NewApp()**

```go
// Add after staking and slashing keepers are initialized:
app.OracleKeeper = oraclekeeper.NewKeeper(
    appCodec,
    runtime.NewKVStoreService(keys[oracletypes.StoreKey]),
    app.StakingKeeper,
    app.SlashingKeeper,
    authtypes.NewModuleAddress(govtypes.ModuleName).String(),
)
```

- [ ] **Step 4: Add oracle to module manager, begin/end blockers, genesis order**

```go
// In module.NewManager(...) add:
oracle.NewAppModule(appCodec, app.OracleKeeper),

// In SetOrderBeginBlockers (after upgradetypes.ModuleName):
// oracle has no BeginBlock hook

// In SetOrderEndBlockers (add oracle):
oracletypes.ModuleName,

// In SetOrderInitGenesis (add oracle after staking):
oracletypes.ModuleName,
```

- [ ] **Step 5: Add oracle store key**

```go
// In NewKVStoreKeys(...) add:
oracletypes.StoreKey,
```

- [ ] **Step 6: Run test**

```bash
go test ./app/... -run TestOracleModuleRegistered -v
```

Expected: PASS.

- [ ] **Step 7: Run full build and test suite**

```bash
make build && make test
```

Expected: Zero errors.

---

## Task 8: Final Verification

- [ ] **Step 1: Run all oracle tests**

```bash
go test ./x/oracle/... ./app/... -v -count=1 -timeout 5m
```

Expected: All tests PASS.

- [ ] **Step 2: Start devnet and verify oracle CLI**

```bash
ignite chain serve --reset-once &
sleep 15

# Query oracle params
vertixd q oracle params --output json

# Query price (expect error — no validators have submitted feeds yet)
vertixd q oracle price VTX:USD --output json

# Stop devnet
pkill -f "vertixd start"
```

Expected: Params query returns default params. Price query returns `ErrPriceNotFound`.

- [ ] **Step 3: Final commit**

```bash
git add .
git commit -m "chore: plan 02 complete — x/oracle module ready"
```

---

*Next plan: `docs/plans/2026-05-09-03-fees-module.md` — `x/fees` EndBlock burn + staker distribution*
