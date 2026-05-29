package simulation

import (
	"bytes"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types/kv"

	"github.com/vertix-network/vertix/x/rwa/types"
)

// NewDecodeStore returns a decoder that pretty-prints rwa KV pairs for the
// simulation diff output.
func NewDecodeStore(cdc codec.BinaryCodec) func(kvA, kvB kv.Pair) string {
	return func(kvA, kvB kv.Pair) string {
		if bytes.HasPrefix(kvA.Key, types.KeyPrefixAsset) {
			var a, b types.AssetRecord
			cdc.MustUnmarshal(kvA.Value, &a)
			cdc.MustUnmarshal(kvB.Value, &b)
			return fmt.Sprintf("%v\n%v", a, b)
		}
		return fmt.Sprintf("unrecognized rwa key %X\n%X", kvA.Key, kvB.Key)
	}
}
