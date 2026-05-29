package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCoinGeckoFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "bitcoin", r.URL.Query().Get("ids"))
		require.Equal(t, "usd", r.URL.Query().Get("vs_currencies"))
		_, _ = w.Write([]byte(`{"bitcoin":{"usd":65000.12}}`))
	}))
	defer srv.Close()

	p := newCoinGecko(srv.URL)
	require.Equal(t, "coingecko", p.Name())
	price, err := p.Fetch(context.Background(), "bitcoin")
	require.NoError(t, err)
	require.Equal(t, "65000.120000000000000000", price.String())
}

func TestCoinGeckoMissingSymbol(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	_, err := newCoinGecko(srv.URL).Fetch(context.Background(), "bitcoin")
	require.Error(t, err)
}
