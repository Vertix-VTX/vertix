package feeder

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

func dec(t *testing.T, s string) math.LegacyDec {
	t.Helper()
	d, err := math.LegacyNewDecFromStr(s)
	require.NoError(t, err)
	return d
}

func TestMedianOdd(t *testing.T) {
	m, err := Median([]math.LegacyDec{dec(t, "3"), dec(t, "1"), dec(t, "2")})
	require.NoError(t, err)
	require.Equal(t, "2.000000000000000000", m.String())
}

func TestMedianEven(t *testing.T) {
	m, err := Median([]math.LegacyDec{dec(t, "1"), dec(t, "3")})
	require.NoError(t, err)
	require.Equal(t, "2.000000000000000000", m.String())
}

func TestMedianEmpty(t *testing.T) {
	_, err := Median(nil)
	require.Error(t, err)
}

func TestDropDeviating(t *testing.T) {
	// median = 100; 10% band keeps [90,110]; 200 is dropped.
	in := []math.LegacyDec{dec(t, "100"), dec(t, "101"), dec(t, "200")}
	out := DropDeviating(in, dec(t, "0.10"))
	require.Len(t, out, 2)
}

func TestCrossSourceMedianTooFew(t *testing.T) {
	_, err := CrossSourceMedian([]math.LegacyDec{dec(t, "100")}, dec(t, "0.10"), 2)
	require.ErrorIs(t, err, ErrInsufficientSources)
}

func TestCrossSourceMedianDropsThenMedians(t *testing.T) {
	in := []math.LegacyDec{dec(t, "100"), dec(t, "102"), dec(t, "300")}
	m, err := CrossSourceMedian(in, dec(t, "0.10"), 2)
	require.NoError(t, err)
	require.Equal(t, "101.000000000000000000", m.String()) // median of [100,102]
}

func TestCrossSourceMedianTooFewAfterDrop(t *testing.T) {
	// two wildly-divergent sources: provisional median between them, both within/around band?
	in := []math.LegacyDec{dec(t, "100"), dec(t, "300")}
	_, err := CrossSourceMedian(in, dec(t, "0.10"), 2)
	require.ErrorIs(t, err, ErrInsufficientSources)
}
