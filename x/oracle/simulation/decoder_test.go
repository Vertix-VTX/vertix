package simulation_test

import (
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/kv"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/oracle/simulation"
	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestDecodeStore(t *testing.T) {
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	dec := simulation.NewDecodeStore(cdc)

	ap := types.AggregatedPrice{Pair: "VTX:USD", Price: "1.50", BlockHeight: 1, BlockTime: time.Unix(1, 0)}
	bz, err := cdc.Marshal(&ap)
	require.NoError(t, err)

	pairs := kv.Pair{Key: types.AggregatedPriceKey("VTX:USD"), Value: bz}
	out := dec(pairs, pairs)
	require.Contains(t, out, "VTX:USD")
}
