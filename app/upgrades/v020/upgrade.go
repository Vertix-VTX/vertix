package v020

import (
	"context"
	"fmt"

	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

const UpgradeName = "v0.2.0-testnet"

type App struct {
	ModuleManager *module.Manager
	Configurator  module.Configurator
}

// RegisterUpgradeHandlers wires the Phase 9 Cosmovisor rehearsal upgrade plan.
func RegisterUpgradeHandlers(app *App, upgradeKeeper *upgradekeeper.Keeper) error {
	upgradeKeeper.SetUpgradeHandler(UpgradeName, func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		if plan.Name != UpgradeName {
			return nil, fmt.Errorf("unexpected upgrade plan %q", plan.Name)
		}
		return app.ModuleManager.RunMigrations(ctx, app.Configurator, fromVM)
	})
	return nil
}
