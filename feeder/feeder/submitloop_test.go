package feeder

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

type mockBroadcaster struct {
	mu    sync.Mutex
	calls [][]FeedSubmission
}

func (m *mockBroadcaster) SubmitFeeds(_ context.Context, feeds []FeedSubmission) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, feeds)
	return "HASH", nil
}

func TestSubmitLoopOncePerWindow(t *testing.T) {
	cache := NewCache()
	now := time.Unix(1000, 0)
	cache.Set("BTC:USD", dec(t, "65000"), now)
	cache.Set("ETH:USD", dec(t, "3200"), now)

	bc := &mockBroadcaster{}
	m := NewMetrics(prometheus.NewRegistry())
	sl := NewSubmitLoop(
		[]PairConfig{{Pair: "BTC:USD"}, {Pair: "ETH:USD"}},
		cache, bc, m, 10, 30*time.Second,
	)
	sl.nowFn = func() time.Time { return now }

	sl.maybeSubmit(context.Background(), 5)
	sl.maybeSubmit(context.Background(), 8)
	sl.maybeSubmit(context.Background(), 12)

	require.Len(t, bc.calls, 2)
	require.Len(t, bc.calls[0], 2)
}

func TestSubmitLoopSkipsStalePairs(t *testing.T) {
	cache := NewCache()
	now := time.Unix(1000, 0)
	cache.Set("BTC:USD", dec(t, "65000"), now.Add(-time.Hour))
	cache.Set("ETH:USD", dec(t, "3200"), now)

	bc := &mockBroadcaster{}
	m := NewMetrics(prometheus.NewRegistry())
	sl := NewSubmitLoop([]PairConfig{{Pair: "BTC:USD"}, {Pair: "ETH:USD"}}, cache, bc, m, 10, 30*time.Second)
	sl.nowFn = func() time.Time { return now }

	sl.maybeSubmit(context.Background(), 5)
	require.Len(t, bc.calls, 1)
	require.Len(t, bc.calls[0], 1)
	require.Equal(t, "ETH:USD", bc.calls[0][0].Pair)
}

func TestSubmitLoopNoFreshPairsNoCall(t *testing.T) {
	cache := NewCache()
	bc := &mockBroadcaster{}
	m := NewMetrics(prometheus.NewRegistry())
	sl := NewSubmitLoop([]PairConfig{{Pair: "BTC:USD"}}, cache, bc, m, 10, 30*time.Second)
	sl.nowFn = func() time.Time { return time.Unix(1000, 0) }
	sl.maybeSubmit(context.Background(), 5)
	require.Empty(t, bc.calls)
}
