package feeder

import (
	"sync"
	"time"

	"cosmossdk.io/math"
)

type cacheEntry struct {
	price   math.LegacyDec
	at      time.Time
	healthy bool
}

// Cache is a concurrency-safe latest-median-per-pair store shared by the
// price loop (writer) and submit loop (reader).
type Cache struct {
	mu sync.RWMutex
	m  map[string]cacheEntry
}

// NewCache returns an empty cache.
func NewCache() *Cache {
	return &Cache{m: make(map[string]cacheEntry)}
}

// Set stores a fresh, healthy median for pair at time t.
func (c *Cache) Set(pair string, price math.LegacyDec, t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[pair] = cacheEntry{price: price, at: t, healthy: true}
}

// MarkUnhealthy flags pair so it will not be submitted, keeping any prior value.
func (c *Cache) MarkUnhealthy(pair string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.m[pair]
	e.healthy = false
	c.m[pair] = e
}

// Fresh returns the cached price if it is healthy and newer than maxAge at now.
func (c *Cache) Fresh(pair string, maxAge time.Duration, now time.Time) (math.LegacyDec, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.m[pair]
	if !ok || !e.healthy {
		return math.LegacyDec{}, false
	}
	if now.Sub(e.at) > maxAge {
		return math.LegacyDec{}, false
	}
	return e.price, true
}
