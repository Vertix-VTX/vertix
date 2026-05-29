package feeder

import (
	"context"
	"time"

	"cosmossdk.io/log"
)

// HeightFunc returns the node's latest block height.
type HeightFunc func(ctx context.Context) (int64, error)

// SubmitLoop submits the cached medians exactly once per vote window.
type SubmitLoop struct {
	pairs       []PairConfig
	cache       *Cache
	bc          Broadcaster
	metrics     *Metrics
	voteWindow  int64
	maxQuoteAge time.Duration
	poll        time.Duration
	retries     int
	heightFn    HeightFunc
	nowFn       func() time.Time
	logger      log.Logger
	lastWindow  int64
}

// NewSubmitLoop constructs a submit loop. voteWindow must be > 0.
func NewSubmitLoop(pairs []PairConfig, cache *Cache, bc Broadcaster, m *Metrics, voteWindow int64, maxQuoteAge time.Duration) *SubmitLoop {
	return &SubmitLoop{
		pairs: pairs, cache: cache, bc: bc, metrics: m,
		voteWindow: voteWindow, maxQuoteAge: maxQuoteAge,
		poll: time.Second, retries: 3, nowFn: time.Now, lastWindow: -1, logger: log.NewNopLogger(),
	}
}

// Configure sets runtime options from config.
func (sl *SubmitLoop) Configure(poll time.Duration, retries int, heightFn HeightFunc, logger log.Logger) *SubmitLoop {
	sl.poll = poll
	sl.retries = retries
	sl.heightFn = heightFn
	sl.logger = logger
	return sl
}

// Run polls height until ctx is cancelled.
func (sl *SubmitLoop) Run(ctx context.Context) {
	ticker := time.NewTicker(sl.poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h, err := sl.heightFn(ctx)
			if err != nil {
				sl.logger.Debug("height query failed", "err", err)
				continue
			}
			sl.maybeSubmit(ctx, h)
		}
	}
}

func (sl *SubmitLoop) maybeSubmit(ctx context.Context, height int64) {
	if sl.voteWindow <= 0 {
		return
	}
	window := height / sl.voteWindow
	if window == sl.lastWindow {
		return
	}
	now := sl.nowFn()
	feeds := make([]FeedSubmission, 0, len(sl.pairs))
	for _, pc := range sl.pairs {
		price, ok := sl.cache.Fresh(pc.Pair, sl.maxQuoteAge, now)
		if !ok {
			continue
		}
		feeds = append(feeds, FeedSubmission{Pair: pc.Pair, Price: price})
	}
	if len(feeds) == 0 {
		return
	}
	attempts := sl.retries
	if attempts < 1 {
		attempts = 1
	}
	var hash string
	var err error
	for i := 0; i < attempts; i++ {
		hash, err = sl.bc.SubmitFeeds(ctx, feeds)
		if err == nil {
			break
		}
		if i < attempts-1 {
			sl.logger.Debug("broadcast retry", "attempt", i+1, "err", err)
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
		}
	}
	if err != nil {
		sl.logger.Error("broadcast failed", "err", err)
		for _, f := range feeds {
			sl.metrics.FeedsFailed.WithLabelValues(f.Pair, "broadcast_error").Inc()
		}
		return
	}
	sl.lastWindow = window
	tsec := float64(now.Unix())
	for _, f := range feeds {
		sl.metrics.FeedsSubmitted.WithLabelValues(f.Pair).Inc()
		sl.metrics.LastSubmit.WithLabelValues(f.Pair).Set(tsec)
	}
	sl.logger.Info("submitted feeds", "window", window, "pairs", len(feeds), "tx", hash)
}
