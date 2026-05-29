package feeder

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCacheSetAndFresh(t *testing.T) {
	c := NewCache()
	now := time.Unix(1000, 0)
	c.Set("BTC:USD", dec(t, "65000"), now)

	price, ok := c.Fresh("BTC:USD", 30*time.Second, now.Add(10*time.Second))
	require.True(t, ok)
	require.Equal(t, "65000.000000000000000000", price.String())
}

func TestCacheStale(t *testing.T) {
	c := NewCache()
	now := time.Unix(1000, 0)
	c.Set("BTC:USD", dec(t, "65000"), now)
	_, ok := c.Fresh("BTC:USD", 30*time.Second, now.Add(31*time.Second))
	require.False(t, ok)
}

func TestCacheUnhealthyNotFresh(t *testing.T) {
	c := NewCache()
	now := time.Unix(1000, 0)
	c.Set("BTC:USD", dec(t, "65000"), now)
	c.MarkUnhealthy("BTC:USD")
	_, ok := c.Fresh("BTC:USD", 30*time.Second, now)
	require.False(t, ok)
}

func TestCacheMissing(t *testing.T) {
	c := NewCache()
	_, ok := c.Fresh("ETH:USD", 30*time.Second, time.Now())
	require.False(t, ok)
}
