package feeder

import (
	"context"
	"sync"

	"cosmossdk.io/log"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/vertix-network/vertix/feeder/feeder/provider"
)

// Feeder is the top-level orchestrator wiring the price and submit loops.
type Feeder struct {
	cfg       *Config
	priceLoop *PriceLoop
	submit    *SubmitLoop
	metrics   *Metrics
	registry  *prometheus.Registry
	logger    log.Logger
	closeFns  []func() error
}

// NewWithDeps builds a Feeder from injected dependencies (used by tests and New).
func NewWithDeps(cfg *Config, providers []provider.PriceProvider, bc Broadcaster, heightFn HeightFunc, voteWindow int64) (*Feeder, error) {
	logger := log.NewNopLogger()
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	cache := NewCache()
	pl := NewPriceLoop(cfg.Pairs, providers, cache, m, cfg.Quality).WithInterval(cfg.FeedInterval).WithLogger(logger)
	sl := NewSubmitLoop(cfg.Pairs, cache, bc, m, voteWindow, cfg.Quality.MaxQuoteAge).
		Configure(cfg.SubmitPoll, cfg.BroadcastRetry, heightFn, logger)

	return &Feeder{cfg: cfg, priceLoop: pl, submit: sl, metrics: m, registry: reg, logger: logger}, nil
}

// New builds a production Feeder: providers from config, gRPC broadcaster,
// and the vote window resolved from chain params (or config override).
func New(cfg *Config, logger log.Logger) (*Feeder, error) {
	provs, err := provider.Build(cfg.Providers, provider.Options{StaticPrices: cfg.StaticPrices})
	if err != nil {
		return nil, err
	}
	bc, err := NewGRPCBroadcaster(cfg, logger)
	if err != nil {
		return nil, err
	}
	voteWindow := cfg.VoteWindow
	if voteWindow <= 0 {
		vw, err := bc.VoteWindow(context.Background())
		if err != nil {
			_ = bc.Close()
			return nil, err
		}
		voteWindow = vw
	}
	f, err := NewWithDeps(cfg, provs, bc, bc.LatestHeight, voteWindow)
	if err != nil {
		_ = bc.Close()
		return nil, err
	}
	f.logger = logger
	f.priceLoop.WithLogger(logger)
	f.submit.logger = logger
	f.closeFns = append(f.closeFns, bc.Close)
	return f, nil
}

// Run starts both loops (and the metrics server if enabled) until ctx ends.
func (f *Feeder) Run(ctx context.Context) {
	var wg sync.WaitGroup
	if f.cfg.Prometheus.Enabled {
		go func() {
			if err := f.metrics.Serve(f.cfg.Prometheus.Port); err != nil {
				f.logger.Error("metrics server stopped", "err", err)
			}
		}()
	}
	wg.Add(2)
	go func() { defer wg.Done(); f.priceLoop.Run(ctx) }()
	go func() { defer wg.Done(); f.submit.Run(ctx) }()
	<-ctx.Done()
	wg.Wait()
	for _, c := range f.closeFns {
		_ = c()
	}
}
