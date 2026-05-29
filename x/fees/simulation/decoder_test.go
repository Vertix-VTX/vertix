package simulation_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/kv"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/fees/simulation"
	"github.com/vertix-network/vertix/x/fees/types"
)

func TestDecodeStore(t *testing.T) {
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	dec := simulation.NewDecodeStore(cdc)

	params := types.DefaultParams()
	bz := cdc.MustMarshal(&params)
	pair := kv.Pair{Key: types.KeyParams, Value: bz}
	require.Contains(t, dec(pair, pair), params.BurnRatio)
}
