package keeper_test

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
)

func twapDec(t *testing.T, s string) math.LegacyDec {
	t.Helper()
	d, err := math.LegacyNewDecFromStr(s)
	require.NoError(t, err)
	return d
}

func TestGetTWAPDurationWeighted(t *testing.T) {
	ctx, k := keeper.OracleKeeper(t, &keeper.MockStaking{}, &keeper.MockSlashing{})
	base := time.Unix(1000, 0).UTC()
	ctx = ctx.WithBlockTime(base.Add(250 * time.Second))

	require.NoError(t, k.AppendTWAPEntry(ctx, "BTC:USD", twapDec(t, "100"), base))
	require.NoError(t, k.AppendTWAPEntry(ctx, "BTC:USD", twapDec(t, "200"), base.Add(100*time.Second)))
	require.NoError(t, k.AppendTWAPEntry(ctx, "BTC:USD", twapDec(t, "300"), base.Add(200*time.Second)))

	got, err := k.GetTWAP(ctx, "BTC:USD", 200*time.Second)
	require.NoError(t, err)
	// window [1050, 1250]: entry@1000 contributes from 1050; 100×100s + 200×100s + 300×50s = 45000 / 250s
	require.True(t, got.Equal(twapDec(t, "180")))
}
