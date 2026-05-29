package fees

import (
	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	feessimulation "github.com/vertix-network/vertix/x/fees/simulation"
	"github.com/vertix-network/vertix/x/fees/types"
)

// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	r := simState.Rand
	burn := math.LegacyNewDecWithPrec(int64(r.Intn(101)), 2)
	dist := math.LegacyOneDec().Sub(burn)
	feesGenesis := types.GenesisState{
		Params: types.FeesParams{
			BurnRatio:         burn.String(),
			DistributionRatio: dist.String(),
		},
	}
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&feesGenesis)
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(sdr simtypes.StoreDecoderRegistry) {
	sdr[types.StoreKey] = feessimulation.NewDecodeStore(am.cdc)
}

// WeightedOperations returns the module operations with their weights.
// Fees is EndBlock-only; no message operations are simulated.
func (AppModule) WeightedOperations(_ module.SimulationState) []simtypes.WeightedOperation {
	return []simtypes.WeightedOperation{}
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (AppModule) ProposalMsgs(_ module.SimulationState) []simtypes.WeightedProposalMsg {
	return []simtypes.WeightedProposalMsg{}
}
