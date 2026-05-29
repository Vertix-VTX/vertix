package types_test

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/oracle/types"
)

func TestFeedKeyRoundTrip(t *testing.T) {
	val := sdk.ValAddress([]byte("validator123456789012345678901234"))
	pair := "BTC:USD"
	key := types.FeedKey(val, pair)
	val2, pair2, err := types.ParseFeedKey(key)
	require.NoError(t, err)
	require.Equal(t, val, val2)
	require.Equal(t, pair, pair2)
}

func TestTWAPKeyOrdering(t *testing.T) {
	pair := "ETH:USD"
	t1 := time.Unix(100, 0)
	t2 := time.Unix(200, 0)
	require.True(t, string(types.TWAPKey(pair, t1)) < string(types.TWAPKey(pair, t2)))
}
