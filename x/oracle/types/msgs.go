package types

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func (m *MsgSetFeeder) ValidateBasic() error {
	if _, err := sdk.ValAddressFromBech32(m.Validator); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("validator: %v", err)
	}
	if _, err := sdk.AccAddressFromBech32(m.Feeder); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("feeder: %v", err)
	}
	return nil
}

func (m *MsgSubmitFeed) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Feeder); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("feeder: %v", err)
	}
	if _, err := sdk.ValAddressFromBech32(m.Validator); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("validator: %v", err)
	}
	if m.Pair == "" {
		return ErrInvalidPairFormat.Wrap("pair is empty")
	}
	price, err := math.LegacyNewDecFromStr(m.Price)
	if err != nil {
		return ErrInvalidPrice.Wrapf("parse: %v", err)
	}
	if !price.IsPositive() {
		return ErrInvalidPrice.Wrap("price must be positive")
	}
	return nil
}

func (m *MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("authority: %v", err)
	}
	return m.Params.Validate()
}
