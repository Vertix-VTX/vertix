package keeper

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

func dec(s string) math.LegacyDec {
	d, err := math.LegacyNewDecFromStr(s)
	if err != nil {
		panic(err)
	}
	return d
}

func TestWeightedMedianThreeValidators(t *testing.T) {
	entries := []WeightedPrice{
		{Price: dec("100"), Weight: 10, Valoper: "a"},
		{Price: dec("110"), Weight: 30, Valoper: "b"},
		{Price: dec("120"), Weight: 10, Valoper: "c"},
	}
	got := WeightedMedian(entries)
	require.True(t, got.Equal(dec("110")))
}

func TestUnweightedMedianEvenCount(t *testing.T) {
	prices := []math.LegacyDec{dec("100"), dec("200")}
	got := UnweightedMedian(prices)
	require.True(t, got.Equal(dec("100"))) // lower-mid
}

func TestQuorumMet(t *testing.T) {
	require.True(t, QuorumMet(math.NewInt(67), math.NewInt(100), dec("0.667")))
	require.False(t, QuorumMet(math.NewInt(66), math.NewInt(100), dec("0.667")))
}
