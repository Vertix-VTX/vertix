package types

import "cosmossdk.io/errors"

var (
	ErrInvalidSigner      = errors.Register(ModuleName, 2, "invalid signer")
	ErrUnauthorized       = errors.Register(ModuleName, 3, "unauthorized")
	ErrAssetExists        = errors.Register(ModuleName, 4, "asset_id already exists")
	ErrAssetNotFound      = errors.Register(ModuleName, 5, "asset not found")
	ErrInvalidStatus      = errors.Register(ModuleName, 6, "invalid asset status for this transition")
	ErrBondTooLow         = errors.Register(ModuleName, 7, "bond is below min_issuer_bond")
	ErrAttestationFailed  = errors.Register(ModuleName, 8, "oracle attestation failed (no or stale price)")
	ErrInvalidNotional    = errors.Register(ModuleName, 9, "notional must be a positive integer")
	ErrTransferRestricted = errors.Register(ModuleName, 10, "transfer not allowed by asset restrictions")
	ErrInvalidAssetID     = errors.Register(ModuleName, 11, "invalid asset_id (must match slug format)")
	ErrInvalidParams      = errors.Register(ModuleName, 12, "invalid params")
	ErrInvalidAmount      = errors.Register(ModuleName, 13, "invalid amount")
)
