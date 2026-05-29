package feeder

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the Prometheus instruments (design §8).
type Metrics struct {
	FeedsSubmitted  *prometheus.CounterVec
	FeedsFailed     *prometheus.CounterVec
	ProviderLatency *prometheus.HistogramVec
	ProviderErrors  *prometheus.CounterVec
	LastSubmit      *prometheus.GaugeVec
	registry        *prometheus.Registry
}

// NewMetrics registers and returns the feeder metrics on reg.
func NewMetrics(reg *prometheus.Registry) *Metrics {
	m := &Metrics{
		FeedsSubmitted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vertix_feeder_feeds_submitted_total", Help: "Feeds submitted per pair.",
		}, []string{"pair"}),
		FeedsFailed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vertix_feeder_feeds_failed_total", Help: "Failed/skipped feeds per pair and reason.",
		}, []string{"pair", "reason"}),
		ProviderLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "vertix_feeder_provider_latency_ms",
			Help:    "Provider fetch latency in ms.",
			Buckets: []float64{10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
		}, []string{"provider", "pair"}),
		ProviderErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vertix_feeder_provider_errors_total", Help: "Provider fetch errors.",
		}, []string{"provider"}),
		LastSubmit: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "vertix_feeder_last_submit_timestamp", Help: "Unix timestamp of last submit per pair.",
		}, []string{"pair"}),
		registry: reg,
	}
	reg.MustRegister(m.FeedsSubmitted, m.FeedsFailed, m.ProviderLatency, m.ProviderErrors, m.LastSubmit)
	return m
}

// ObserveLatency records a provider fetch latency.
func (m *Metrics) ObserveLatency(provider, pair string, d time.Duration) {
	m.ProviderLatency.WithLabelValues(provider, pair).Observe(float64(d.Milliseconds()))
}

// Serve starts the metrics HTTP server on the given port. Blocks until error.
func (m *Metrics) Serve(port int) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))
	srv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	return srv.ListenAndServe()
}
