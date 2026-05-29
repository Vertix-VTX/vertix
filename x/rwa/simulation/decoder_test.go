package simulation_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/kv"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/rwa/simulation"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestDecodeStore(t *testing.T) {
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	dec := simulation.NewDecodeStore(cdc)

	rec := types.AssetRecord{AssetId: "asset-1", Denom: "rwa/asset-1"}
	bz, err := cdc.Marshal(&rec)
	require.NoError(t, err)

	pair := kv.Pair{Key: types.AssetKey("asset-1"), Value: bz}
	out := dec(pair, pair)
	require.Contains(t, out, "asset-1")
}
