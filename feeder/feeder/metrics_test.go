package feeder

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestMetricsRegister(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	require.NotNil(t, m)

	m.FeedsSubmitted.WithLabelValues("BTC:USD").Inc()
	m.FeedsFailed.WithLabelValues("VTX:USD", "insufficient_sources").Inc()
	m.ProviderErrors.WithLabelValues("binance").Inc()

	mfs, err := reg.Gather()
	require.NoError(t, err)
	names := map[string]bool{}
	for _, mf := range mfs {
		names[mf.GetName()] = true
	}
	require.True(t, names["vertix_feeder_feeds_submitted_total"])
	require.True(t, names["vertix_feeder_feeds_failed_total"])
	require.True(t, names["vertix_feeder_provider_errors_total"])
}
