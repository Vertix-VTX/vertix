package oracle

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	oraclesimulation "github.com/vertix-network/vertix/x/oracle/simulation"
	"github.com/vertix-network/vertix/x/oracle/types"
)

// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	r := simState.Rand
	params := types.DefaultParams()
	pairs := []string{"VTX:USD", "ATOM:USD", "OSMO:USD"}
	n := 1 + r.Intn(len(pairs))
	params.AcceptList = pairs[:n]
	oracleGenesis := types.GenesisState{Params: params}
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&oracleGenesis)
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(sdr simtypes.StoreDecoderRegistry) {
	sdr[types.StoreKey] = oraclesimulation.NewDecodeStore(am.cdc)
}

// WeightedOperations returns the module operations with their weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	var weightSubmit, weightSetFeeder int
	simState.AppParams.GetOrGenerate(oraclesimulation.OpWeightMsgSubmitFeed, &weightSubmit, nil,
		func(_ *rand.Rand) { weightSubmit = oraclesimulation.DefaultWeightMsgSubmitFeed })
	simState.AppParams.GetOrGenerate(oraclesimulation.OpWeightMsgSetFeeder, &weightSetFeeder, nil,
		func(_ *rand.Rand) { weightSetFeeder = oraclesimulation.DefaultWeightMsgSetFeeder })

	return []simtypes.WeightedOperation{
		simulation.NewWeightedOperation(
			weightSubmit,
			oraclesimulation.SimulateMsgSubmitFeed(simState.TxConfig, am.accountKeeper, am.bankKeeper, am.keeper),
		),
		simulation.NewWeightedOperation(
			weightSetFeeder,
			oraclesimulation.SimulateMsgSetFeeder(simState.TxConfig, am.accountKeeper, am.bankKeeper, am.keeper),
		),
	}
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (AppModule) ProposalMsgs(_ module.SimulationState) []simtypes.WeightedProposalMsg {
	return []simtypes.WeightedProposalMsg{}
}
