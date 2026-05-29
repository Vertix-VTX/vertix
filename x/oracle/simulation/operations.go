package simulation

import (
	"math/rand"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	"github.com/vertix-network/vertix/x/oracle/keeper"
	"github.com/vertix-network/vertix/x/oracle/types"
)

const (
	OpWeightMsgSetFeeder  = "op_weight_msg_set_feeder"
	OpWeightMsgSubmitFeed = "op_weight_msg_submit_feed"

	DefaultWeightMsgSetFeeder  = 30
	DefaultWeightMsgSubmitFeed = 80
)

// SimulateMsgSubmitFeed builds and delivers a feed submission for a random accept-listed pair.
func SimulateMsgSubmitFeed(
	txCfg client.TxConfig,
	ak simulation.AccountKeeper,
	bk simulation.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		msgType := sdk.MsgTypeURL(&types.MsgSubmitFeed{})

		params, err := k.GetParams(ctx)
		if err != nil || len(params.AcceptList) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "no accept-listed pairs"), nil, nil
		}

		val, ok := k.BondedValidator(ctx, r)
		if !ok {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "no bonded validators"), nil, nil
		}

		valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "invalid validator address"), nil, nil
		}

		feederAddr, err := k.ResolveAuthorizedFeeder(ctx, valAddr)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "resolve feeder"), nil, nil
		}

		simAccount, found := simtypes.FindAccount(accs, feederAddr)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "feeder sim account not found"), nil, nil
		}

		pair := params.AcceptList[r.Intn(len(params.AcceptList))]
		price := simtypes.RandomDecAmount(r, math.LegacyNewDec(1000)).Add(math.LegacyOneDec())

		msg := &types.MsgSubmitFeed{
			Feeder:    feederAddr.String(),
			Validator: val.GetOperator(),
			Pair:      pair,
			Price:     price.String(),
		}

		txCtx := simulation.OperationInput{
			R:               r,
			App:             app,
			TxGen:           txCfg,
			Msg:             msg,
			Context:         ctx,
			SimAccount:      simAccount,
			AccountKeeper:   ak,
			Bankkeeper:      bk,
			ModuleName:      types.ModuleName,
			CoinsSpentInMsg: sdk.Coins{},
		}

		return simulation.GenAndDeliverTxWithRandFees(txCtx)
	}
}

// SimulateMsgSetFeeder builds and delivers a feeder delegation for a bonded validator.
func SimulateMsgSetFeeder(
	txCfg client.TxConfig,
	ak simulation.AccountKeeper,
	bk simulation.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		msgType := sdk.MsgTypeURL(&types.MsgSetFeeder{})

		val, ok := k.BondedValidator(ctx, r)
		if !ok {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "no bonded validators"), nil, nil
		}

		valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "invalid validator address"), nil, nil
		}

		valSimAccount, found := simtypes.FindAccount(accs, sdk.AccAddress(valAddr.Bytes()))
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "validator sim account not found"), nil, nil
		}

		feederSimAccount, _ := simtypes.RandomAcc(r, accs)

		msg := &types.MsgSetFeeder{
			Validator: val.GetOperator(),
			Feeder:    feederSimAccount.Address.String(),
		}

		txCtx := simulation.OperationInput{
			R:               r,
			App:             app,
			TxGen:           txCfg,
			Msg:             msg,
			Context:         ctx,
			SimAccount:      valSimAccount,
			AccountKeeper:   ak,
			Bankkeeper:      bk,
			ModuleName:      types.ModuleName,
			CoinsSpentInMsg: sdk.Coins{},
		}

		return simulation.GenAndDeliverTxWithRandFees(txCtx)
	}
}
