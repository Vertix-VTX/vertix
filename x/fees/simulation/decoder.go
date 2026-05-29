package simulation

import (
	"bytes"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types/kv"

	"github.com/vertix-network/vertix/x/fees/types"
)

// NewDecodeStore pretty-prints fees KV pairs for simulation diff output.
func NewDecodeStore(cdc codec.BinaryCodec) func(kvA, kvB kv.Pair) string {
	return func(kvA, kvB kv.Pair) string {
		switch {
		case bytes.Equal(kvA.Key, types.KeyParams):
			var a, b types.FeesParams
			cdc.MustUnmarshal(kvA.Value, &a)
			cdc.MustUnmarshal(kvB.Value, &b)
			return fmt.Sprintf("%v\n%v", a, b)
		case bytes.Equal(kvA.Key, types.KeyGenesisSupply) || bytes.Equal(kvA.Key, types.KeyCumulativeBurned):
			return fmt.Sprintf("int %X\n%X", kvA.Value, kvB.Value)
		default:
			return fmt.Sprintf("unrecognized fees key %X\n%X", kvA.Key, kvB.Key)
		}
	}
}
