package rwa

import (
	"math/rand"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	rwasimulation "github.com/vertix-network/vertix/x/rwa/simulation"
	"github.com/vertix-network/vertix/x/rwa/types"
)

// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	r := simState.Rand
	params := types.DefaultParams()

	bond, _ := params.MinIssuerBondInt()
	if bond.IsPositive() {
		switch r.Intn(3) {
		case 0:
			params.MinIssuerBond = bond.MulRaw(int64(1 + r.Intn(5))).String()
		case 1:
			params.MinIssuerBond = bond.QuoRaw(int64(1 + r.Intn(5))).String()
		default:
			// keep default
		}
	}

	if mintRate, err := params.MintFeeRateDec(); err == nil {
		maxMintRate := math.LegacyNewDecWithPrec(1, 2) // 0.01
		if mintRate.GT(maxMintRate) {
			maxMintRate = mintRate
		}
		params.MintFeeRate = simtypes.RandomDecAmount(r, maxMintRate).String()
	}

	rwaGenesis := types.GenesisState{Params: params}
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&rwaGenesis)
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(sdr simtypes.StoreDecoderRegistry) {
	sdr[types.StoreKey] = rwasimulation.NewDecodeStore(am.cdc)
}

// WeightedOperations returns the module operations with their weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	var weightRegister int
	simState.AppParams.GetOrGenerate(rwasimulation.OpWeightMsgRegisterAsset, &weightRegister, nil,
		func(_ *rand.Rand) { weightRegister = rwasimulation.DefaultWeightMsgRegisterAsset })

	return []simtypes.WeightedOperation{
		simulation.NewWeightedOperation(
			weightRegister,
			rwasimulation.SimulateMsgRegisterAsset(simState.TxConfig, am.accountKeeper, am.bankKeeper, am.keeper),
		),
	}
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (AppModule) ProposalMsgs(_ module.SimulationState) []simtypes.WeightedProposalMsg {
	return []simtypes.WeightedProposalMsg{}
}
