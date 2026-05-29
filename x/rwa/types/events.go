package types

const (
	EventTypeAssetRegistered    = "rwa_asset_registered"
	EventTypeAssetAttested      = "rwa_asset_attested"
	EventTypeRWAMinted          = "rwa_minted"
	EventTypeRWATransferred     = "rwa_transferred"
	EventTypeRWASettled         = "rwa_settled"
	EventTypeRestrictionUpdated = "rwa_restriction_updated"
	EventTypeBondSlashed        = "rwa_bond_slashed"
	EventTypeParamsUpdated      = "rwa_params_updated"

	AttributeKeyAssetID       = "asset_id"
	AttributeKeyIssuer        = "issuer"
	AttributeKeyBond          = "bond"
	AttributeKeyOraclePair    = "oracle_pair"
	AttributeKeyAttestedPrice = "attested_price"
	AttributeKeyNotional      = "notional"
	AttributeKeyMintFee       = "mint_fee"
	AttributeKeySettleFee     = "settle_fee"
	AttributeKeyBondReturned  = "bond_returned"
	AttributeKeySender        = "sender"
	AttributeKeyRecipient     = "recipient"
	AttributeKeyAmount        = "amount"
	AttributeKeyAllowAll      = "allow_all"
	AttributeKeyAllowCount    = "allow_count"
	AttributeKeyDenyCount     = "deny_count"
	AttributeKeyReason        = "reason"
)
