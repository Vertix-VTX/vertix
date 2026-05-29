package app

import (
	"encoding/json"

	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	ibctesting "github.com/cosmos/ibc-go/v8/testing"
	ibctestingtypes "github.com/cosmos/ibc-go/v8/testing/types"
)

func (app *App) GetBaseApp() *baseapp.BaseApp                      { return app.App.BaseApp }
func (app *App) GetStakingKeeper() ibctestingtypes.StakingKeeper   { return app.StakingKeeper }
func (app *App) GetScopedIBCKeeper() capabilitykeeper.ScopedKeeper { return app.ScopedIBCKeeper }
func (app *App) GetTxConfig() client.TxConfig                      { return app.txConfig }

func SetupTestingApp() (ibctesting.TestingApp, map[string]json.RawMessage) {
	a, err := New(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		simtestutil.NewAppOptionsWithFlagHome(""),
	)
	if err != nil {
		panic(err)
	}
	return a, a.DefaultGenesis()
}

var _ ibctesting.TestingApp = (*App)(nil)
