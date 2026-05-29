package types

import "cosmossdk.io/errors"

var (
	ErrUnauthorized    = errors.Register(ModuleName, 2, "unauthorized")
	ErrInvalidParams   = errors.Register(ModuleName, 3, "invalid params")
	ErrInvalidRatioSum = errors.Register(ModuleName, 4, "burn_ratio + distribution_ratio must equal 1")
)
