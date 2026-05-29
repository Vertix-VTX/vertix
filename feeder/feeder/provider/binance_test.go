package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBinanceFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
		_, _ = w.Write([]byte(`{"symbol":"BTCUSDT","price":"65000.10000000"}`))
	}))
	defer srv.Close()

	p := newBinance(srv.URL)
	require.Equal(t, "binance", p.Name())
	price, err := p.Fetch(context.Background(), "BTCUSDT")
	require.NoError(t, err)
	require.Equal(t, "65000.100000000000000000", price.String())
}

func TestBinanceBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	_, err := newBinance(srv.URL).Fetch(context.Background(), "BTCUSDT")
	require.Error(t, err)
}
