package simulation

import (
	"bytes"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types/kv"

	"github.com/vertix-network/vertix/x/oracle/types"
)

// NewDecodeStore returns a decoder that pretty-prints oracle KV pairs for the
// simulation diff output.
func NewDecodeStore(cdc codec.BinaryCodec) func(kvA, kvB kv.Pair) string {
	return func(kvA, kvB kv.Pair) string {
		switch {
		case bytes.HasPrefix(kvA.Key, types.KeyPrefixAggregatedPrice):
			var a, b types.AggregatedPrice
			cdc.MustUnmarshal(kvA.Value, &a)
			cdc.MustUnmarshal(kvB.Value, &b)
			return fmt.Sprintf("%v\n%v", a, b)
		case bytes.HasPrefix(kvA.Key, types.KeyPrefixFeed):
			var a, b types.OracleFeed
			cdc.MustUnmarshal(kvA.Value, &a)
			cdc.MustUnmarshal(kvB.Value, &b)
			return fmt.Sprintf("%v\n%v", a, b)
		default:
			return fmt.Sprintf("unrecognized oracle key %X\n%X", kvA.Key, kvB.Key)
		}
	}
}
