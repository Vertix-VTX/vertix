package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/address"
)

const (
	// ModuleName defines the module name.
	ModuleName = "rwa"

	// StoreKey defines the primary module store key.
	StoreKey = ModuleName

	// BondDenom is the denom issuer bonds are escrowed in (chain base denom).
	BondDenom = "uvtx"

	// RWADenomPrefix namespaces every factory denom: rwa/{asset_id}.
	RWADenomPrefix = "rwa/"
)

var (
	KeyPrefixAsset       = []byte{0x01} // {asset_id} -> AssetRecord
	KeyPrefixIssuerIndex = []byte{0x02} // {issuer}/{asset_id} -> 0x01 (presence)
	KeyPrefixAllowlist   = []byte{0x03} // {asset_id}/{addr} -> presence
	KeyPrefixDenylist    = []byte{0x04} // {asset_id}/{addr} -> presence
	KeyParams            = []byte{0x05} // RWAParams
)

// AssetKey returns the 0x01 store key for an asset record.
func AssetKey(assetID string) []byte {
	return append(append([]byte{}, KeyPrefixAsset...), []byte(assetID)...)
}

// IssuerIndexKey returns the 0x02 issuer->asset index key (issuer length-prefixed).
func IssuerIndexKey(issuer sdk.AccAddress, assetID string) []byte {
	key := append(append([]byte{}, KeyPrefixIssuerIndex...), address.MustLengthPrefix(issuer)...)
	return append(key, []byte("/"+assetID)...)
}

// IssuerIndexPrefix returns the 0x02 scan prefix for one issuer.
func IssuerIndexPrefix(issuer sdk.AccAddress) []byte {
	return append(append([]byte{}, KeyPrefixIssuerIndex...), address.MustLengthPrefix(issuer)...)
}

// AllowlistKey / DenylistKey build membership keys; asset_id is length-prefixed
// so the address suffix can be parsed back unambiguously.
func AllowlistKey(assetID string, addr sdk.AccAddress) []byte {
	return membershipKey(KeyPrefixAllowlist, assetID, addr)
}

func DenylistKey(assetID string, addr sdk.AccAddress) []byte {
	return membershipKey(KeyPrefixDenylist, assetID, addr)
}

// AllowlistPrefix / DenylistPrefix scan all members of one asset.
func AllowlistPrefix(assetID string) []byte {
	return membershipPrefix(KeyPrefixAllowlist, assetID)
}

func DenylistPrefix(assetID string) []byte {
	return membershipPrefix(KeyPrefixDenylist, assetID)
}

func membershipKey(prefix []byte, assetID string, addr sdk.AccAddress) []byte {
	return append(membershipPrefix(prefix, assetID), addr.Bytes()...)
}

// membershipPrefix lays out prefix(1) | lenByte(1) | assetID. The asset id is
// single-byte length-prefixed so the trailing address can be parsed back; a
// slug is ≤ 43 chars, which fits in one byte.
func membershipPrefix(prefix []byte, assetID string) []byte {
	idBytes := []byte(assetID)
	key := append(append([]byte{}, prefix...), byte(len(idBytes)))
	return append(key, idBytes...)
}
