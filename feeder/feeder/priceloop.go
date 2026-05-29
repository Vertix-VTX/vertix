package feeder

import (
	"context"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/math"

	"github.com/vertix-network/vertix/feeder/feeder/provider"
)

// PriceLoop fetches prices and updates the cache on a fixed interval.
type PriceLoop struct {
	pairs     []PairConfig
	providers []provider.PriceProvider
	cache     *Cache
	metrics   *Metrics
	quality   QualityConfig
	maxDev    math.LegacyDec
	interval  time.Duration
	logger    log.Logger
}

// NewPriceLoop constructs a price loop. maxDeviation is parsed from quality.
func NewPriceLoop(pairs []PairConfig, providers []provider.PriceProvider, cache *Cache, m *Metrics, quality QualityConfig) *PriceLoop {
	dev, err := math.LegacyNewDecFromStr(quality.MaxDeviation)
	if err != nil {
		dev = math.LegacyNewDecWithPrec(10, 2) // 0.10 fallback; config is pre-validated
	}
	return &PriceLoop{
		pairs: pairs, providers: providers, cache: cache, metrics: m,
		quality: quality, maxDev: dev, interval: quality.MaxQuoteAge, logger: log.NewNopLogger(),
	}
}

// WithLogger sets a logger and tick interval; interval defaults to maxQuoteAge otherwise.
func (pl *PriceLoop) WithLogger(l log.Logger) *PriceLoop { pl.logger = l; return pl }

// WithInterval overrides the tick interval (feed_interval).
func (pl *PriceLoop) WithInterval(d time.Duration) *PriceLoop { pl.interval = d; return pl }

// Run ticks until ctx is cancelled.
func (pl *PriceLoop) Run(ctx context.Context) {
	ticker := time.NewTicker(pl.interval)
	defer ticker.Stop()
	pl.tick(ctx, time.Now())
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pl.tick(ctx, time.Now())
		}
	}
}

func (pl *PriceLoop) tick(ctx context.Context, now time.Time) {
	for _, pc := range pl.pairs {
		var quotes []math.LegacyDec
		for _, prov := range pl.providers {
			symbol, ok := pc.Symbols[prov.Name()]
			if !ok {
				continue
			}
			start := time.Now()
			price, err := prov.Fetch(ctx, symbol)
			pl.metrics.ObserveLatency(prov.Name(), pc.Pair, time.Since(start))
			if err != nil {
				pl.metrics.ProviderErrors.WithLabelValues(prov.Name()).Inc()
				pl.logger.Debug("provider fetch failed", "provider", prov.Name(), "pair", pc.Pair, "err", err)
				continue
			}
			quotes = append(quotes, price)
		}
		median, err := CrossSourceMedian(quotes, pl.maxDev, pl.quality.MinProviders)
		if err != nil {
			pl.cache.MarkUnhealthy(pc.Pair)
			pl.metrics.FeedsFailed.WithLabelValues(pc.Pair, "insufficient_sources").Inc()
			continue
		}
		pl.cache.Set(pc.Pair, median, now)
	}
}
