package types

import (
	"regexp"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// slugRegex is a strict subset of the bank denom charset so that
// "rwa/"+asset_id is always a valid factory denom.
var (
	slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,42}$`)
	pairRegex = regexp.MustCompile(`^[A-Z0-9]+:[A-Z0-9]+$`)

	// maxRestrictionBatch bounds add/del entries per UpdateRestrictions tx (gas hygiene).
	maxRestrictionBatch = 100
)

// ValidateAssetID checks the slug format.
func ValidateAssetID(id string) error {
	if !slugRegex.MatchString(id) {
		return ErrInvalidAssetID.Wrapf("%q", id)
	}
	return nil
}

func validatePositiveInt(s, name string) error {
	v, ok := math.NewIntFromString(s)
	if !ok {
		return ErrInvalidAmount.Wrapf("%s: not an integer: %q", name, s)
	}
	if !v.IsPositive() {
		return ErrInvalidAmount.Wrapf("%s must be positive", name)
	}
	return nil
}

func (m *MsgRegisterAsset) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Issuer); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("issuer: %v", err)
	}
	if err := ValidateAssetID(m.AssetId); err != nil {
		return err
	}
	if m.Name == "" {
		return ErrInvalidParams.Wrap("name must not be empty")
	}
	if !pairRegex.MatchString(m.OraclePair) {
		return ErrInvalidParams.Wrapf("oracle_pair %q is not BASE:QUOTE", m.OraclePair)
	}
	return validatePositiveInt(m.Bond, "bond")
}

func (m *MsgAttestAsset) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Issuer); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("issuer: %v", err)
	}
	return ValidateAssetID(m.AssetId)
}

func (m *MsgMintRWA) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Issuer); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("issuer: %v", err)
	}
	if err := ValidateAssetID(m.AssetId); err != nil {
		return err
	}
	if err := validatePositiveInt(m.Notional, "notional"); err != nil {
		return ErrInvalidNotional.Wrap(err.Error())
	}
	return nil
}

func (m *MsgTransferRWA) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Sender); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("sender: %v", err)
	}
	if _, err := sdk.AccAddressFromBech32(m.Recipient); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("recipient: %v", err)
	}
	if err := ValidateAssetID(m.AssetId); err != nil {
		return err
	}
	return validatePositiveInt(m.Amount, "amount")
}

func (m *MsgSettleRWA) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Issuer); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("issuer: %v", err)
	}
	return ValidateAssetID(m.AssetId)
}

func (m *MsgUpdateRestrictions) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Issuer); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("issuer: %v", err)
	}
	if err := ValidateAssetID(m.AssetId); err != nil {
		return err
	}
	for _, group := range [][]string{m.AddAllow, m.DelAllow, m.AddDeny, m.DelDeny} {
		if len(group) > maxRestrictionBatch {
			return ErrInvalidParams.Wrapf("restriction batch exceeds %d entries", maxRestrictionBatch)
		}
		for _, a := range group {
			if _, err := sdk.AccAddressFromBech32(a); err != nil {
				return sdkerrors.ErrInvalidAddress.Wrapf("restriction entry %q: %v", a, err)
			}
		}
	}
	return nil
}

func (m *MsgSlashBond) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("authority: %v", err)
	}
	return ValidateAssetID(m.AssetId)
}

func (m *MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("authority: %v", err)
	}
	return m.Params.Validate()
}
