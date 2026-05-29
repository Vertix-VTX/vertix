package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/sample"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestMsgRegisterAssetValidateBasic(t *testing.T) {
	issuer := sample.AccAddress()
	good := &types.MsgRegisterAsset{Issuer: issuer, AssetId: "gold-vault-01", Name: "Gold", OraclePair: "XAU:USD", Bond: "10000000000"}
	require.NoError(t, good.ValidateBasic())

	require.Error(t, (&types.MsgRegisterAsset{Issuer: "bad", AssetId: "gold", Name: "x", OraclePair: "XAU:USD", Bond: "1"}).ValidateBasic())
	require.Error(t, (&types.MsgRegisterAsset{Issuer: issuer, AssetId: "AB", Name: "x", OraclePair: "XAU:USD", Bond: "1"}).ValidateBasic())    // too short / uppercase
	require.Error(t, (&types.MsgRegisterAsset{Issuer: issuer, AssetId: "gold", Name: "", OraclePair: "XAU:USD", Bond: "1"}).ValidateBasic())   // empty name
	require.Error(t, (&types.MsgRegisterAsset{Issuer: issuer, AssetId: "gold", Name: "x", OraclePair: "bad pair", Bond: "1"}).ValidateBasic()) // bad pair
	require.Error(t, (&types.MsgRegisterAsset{Issuer: issuer, AssetId: "gold", Name: "x", OraclePair: "XAU:USD", Bond: "0"}).ValidateBasic())  // non-positive bond
}

func TestMsgMintRWAValidateBasic(t *testing.T) {
	issuer := sample.AccAddress()
	require.NoError(t, (&types.MsgMintRWA{Issuer: issuer, AssetId: "gold", Notional: "1000"}).ValidateBasic())
	require.Error(t, (&types.MsgMintRWA{Issuer: issuer, AssetId: "gold", Notional: "0"}).ValidateBasic())
	require.Error(t, (&types.MsgMintRWA{Issuer: issuer, AssetId: "gold", Notional: "-5"}).ValidateBasic())
}

func TestMsgTransferRWAValidateBasic(t *testing.T) {
	require.NoError(t, (&types.MsgTransferRWA{Sender: sample.AccAddress(), Recipient: sample.AccAddress(), AssetId: "gold", Amount: "10"}).ValidateBasic())
	require.Error(t, (&types.MsgTransferRWA{Sender: sample.AccAddress(), Recipient: "bad", AssetId: "gold", Amount: "10"}).ValidateBasic())
}

func TestMsgSlashBondValidateBasic(t *testing.T) {
	require.NoError(t, (&types.MsgSlashBond{Authority: sample.AccAddress(), AssetId: "gold"}).ValidateBasic())
	require.Error(t, (&types.MsgSlashBond{Authority: "bad", AssetId: "gold"}).ValidateBasic())
}

func TestValidateAssetID(t *testing.T) {
	require.NoError(t, types.ValidateAssetID("gold-vault-01"))
	require.Error(t, types.ValidateAssetID("Gold")) // uppercase
	require.Error(t, types.ValidateAssetID("go"))   // too short
	require.Error(t, types.ValidateAssetID("a/b"))  // illegal char
}
