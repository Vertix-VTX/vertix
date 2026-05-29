package types

const (
	// ModuleName defines the module name.
	ModuleName = "fees"

	// StoreKey defines the primary module store key.
	StoreKey = ModuleName

	// FeeDenom is the native token whose fee-collector balance x/fees burns.
	// MUST match the chain base/bond denom (config.yml: uvtx). Non-FeeDenom
	// coins in the fee collector are never burned (spec D2).
	FeeDenom = "uvtx"
)

// KeyParams is the store key under which FeesParams is persisted.
var KeyParams = []byte{0x01}
