package simulation

import (
	"fmt"
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	"github.com/vertix-network/vertix/x/rwa/keeper"
	"github.com/vertix-network/vertix/x/rwa/types"
)

const (
	OpWeightMsgRegisterAsset      = "op_weight_msg_register_asset"
	DefaultWeightMsgRegisterAsset = 50
)

var simOraclePairs = []string{"VTX:USD", "ATOM:USD", "XAU:USD"}

// SimulateMsgRegisterAsset registers a randomized asset with a valid bond.
func SimulateMsgRegisterAsset(
	txCfg client.TxConfig,
	ak simulation.AccountKeeper,
	bk simulation.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		msgType := sdk.MsgTypeURL(&types.MsgRegisterAsset{})

		issuer, _ := simtypes.RandomAcc(r, accs)
		params, err := k.GetParams(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "params"), nil, nil
		}

		bond, ok := params.MinIssuerBondInt()
		if !ok || !bond.IsPositive() {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "min bond"), nil, nil
		}

		assetID := fmt.Sprintf("sim%09d", r.Int63n(1_000_000_000))
		if _, found := k.GetAsset(ctx, assetID); found {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "asset exists"), nil, nil
		}

		msg := &types.MsgRegisterAsset{
			Issuer:     issuer.Address.String(),
			AssetId:    assetID,
			Name:       "sim asset",
			OraclePair: simOraclePairs[r.Intn(len(simOraclePairs))],
			Bond:       bond.String(),
		}

		txCtx := simulation.OperationInput{
			R:               r,
			App:             app,
			TxGen:           txCfg,
			Msg:             msg,
			Context:         ctx,
			SimAccount:      issuer,
			AccountKeeper:   ak,
			Bankkeeper:      bk,
			ModuleName:      types.ModuleName,
			CoinsSpentInMsg: sdk.NewCoins(sdk.NewCoin(types.BondDenom, bond)),
		}

		return simulation.GenAndDeliverTxWithRandFees(txCtx)
	}
}
