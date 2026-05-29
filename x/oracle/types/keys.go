package types

import (
	"encoding/binary"
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/address"
)

const (
	ModuleName = "oracle"
	StoreKey   = ModuleName
)

var (
	KeyPrefixFeed            = []byte{0x01}
	KeyPrefixAggregatedPrice = []byte{0x02}
	KeyPrefixTWAP            = []byte{0x03}
	KeyPrefixMissCounter     = []byte{0x04}
	KeyPrefixTotalWindows    = []byte{0x05}
	KeyParams                = []byte{0x06}
	KeyPrefixFeederToValoper = []byte{0x07}
	KeyPrefixValoperToFeeder = []byte{0x08}
)

func FeedKey(val sdk.ValAddress, pair string) []byte {
	prefixed := address.MustLengthPrefix(val)
	key := append(append([]byte{}, KeyPrefixFeed...), prefixed...)
	return append(key, []byte("/"+pair)...)
}

func ParseFeedKey(key []byte) (sdk.ValAddress, string, error) {
	if len(key) < len(KeyPrefixFeed)+2 || key[0] != KeyPrefixFeed[0] {
		return nil, "", fmt.Errorf("invalid feed key")
	}
	rest := key[len(KeyPrefixFeed):]
	if len(rest) < 1 {
		return nil, "", fmt.Errorf("invalid feed key: missing validator")
	}
	addrLen := int(rest[0])
	if len(rest) < 1+addrLen {
		return nil, "", fmt.Errorf("invalid feed key: truncated validator")
	}
	valBytes := rest[1 : 1+addrLen]
	rest = rest[1+addrLen:]
	if len(rest) == 0 || rest[0] != '/' {
		return nil, "", fmt.Errorf("invalid feed key: missing pair")
	}
	return sdk.ValAddress(valBytes), string(rest[1:]), nil
}

func AggregatedPriceKey(pair string) []byte {
	return append(append([]byte{}, KeyPrefixAggregatedPrice...), []byte(pair)...)
}

func TWAPKey(pair string, ts time.Time) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(ts.UnixNano()))
	key := append([]byte{}, KeyPrefixTWAP...)
	key = append(key, []byte(pair)...)
	key = append(key, '/')
	return append(key, buf...)
}

func TWAPPrefix(pair string) []byte {
	key := append([]byte{}, KeyPrefixTWAP...)
	key = append(key, []byte(pair)...)
	return append(key, '/')
}

// FeedKeyPrefix returns the prefix for scanning all feeds (pair filter in callback).
func FeedKeyPrefix() []byte {
	return append([]byte{}, KeyPrefixFeed...)
}

func MissCounterKey(val sdk.ValAddress) []byte {
	return append(append([]byte{}, KeyPrefixMissCounter...), val.Bytes()...)
}

func TotalWindowsKey(val sdk.ValAddress) []byte {
	return append(append([]byte{}, KeyPrefixTotalWindows...), val.Bytes()...)
}

func FeederToValoperKey(feeder sdk.AccAddress) []byte {
	return append(append([]byte{}, KeyPrefixFeederToValoper...), feeder.Bytes()...)
}

func ValoperToFeederKey(val sdk.ValAddress) []byte {
	return append(append([]byte{}, KeyPrefixValoperToFeeder...), val.Bytes()...)
}
