package types

import "cosmossdk.io/errors"

var (
	ErrInvalidSigner       = errors.Register(ModuleName, 2, "invalid signer")
	ErrUnauthorized        = errors.Register(ModuleName, 3, "unauthorized")
	ErrInvalidPrice        = errors.Register(ModuleName, 4, "invalid price: must be positive")
	ErrPairNotAccepted     = errors.Register(ModuleName, 5, "pair not in accept_list")
	ErrNoPrice             = errors.Register(ModuleName, 6, "no aggregated price for pair")
	ErrStalePrice          = errors.Register(ModuleName, 7, "aggregated price is stale")
	ErrNoTWAPData          = errors.Register(ModuleName, 8, "no TWAP data in window")
	ErrNotBondedValidator  = errors.Register(ModuleName, 9, "validator is not bonded")
	ErrFeederNotAuthorized = errors.Register(ModuleName, 10, "feeder not authorized for validator")
	ErrFeederAlreadyBound  = errors.Register(ModuleName, 11, "feeder already bound to another validator")
	ErrInvalidParams       = errors.Register(ModuleName, 12, "invalid params")
	ErrInvalidPairFormat   = errors.Register(ModuleName, 13, "invalid pair format")
)
