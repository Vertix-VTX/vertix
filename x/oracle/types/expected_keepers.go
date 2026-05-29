package types

import (
	"context"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type StakingKeeper interface {
	GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error)
	GetLastValidatorPower(ctx context.Context, addr sdk.ValAddress) (int64, error)
	GetBondedValidatorsByPower(ctx context.Context) ([]stakingtypes.Validator, error)
	GetLastTotalPower(ctx context.Context) (math.Int, error) // quorum denominator — consensus power (spec C1b)
}

type SlashingKeeper interface {
	Slash(ctx context.Context, consAddr sdk.ConsAddress, fraction math.LegacyDec, power, distributionHeight int64) error
}

// OracleKeeper is the public interface consumed by x/rwa (Phase 3).
type OracleKeeper interface {
	GetPrice(ctx context.Context, pair string) (math.LegacyDec, error)
	GetTWAP(ctx context.Context, pair string, window time.Duration) (math.LegacyDec, error)
}
