package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStaticProviderFetch(t *testing.T) {
	p, err := Build([]string{"static"}, Options{StaticPrices: map[string]string{"VTX:USD": "0.10"}})
	require.NoError(t, err)
	require.Equal(t, "static", p[0].Name())

	price, err := p[0].Fetch(context.Background(), "VTX:USD")
	require.NoError(t, err)
	require.Equal(t, "0.100000000000000000", price.String())
}

func TestStaticProviderUnknownSymbol(t *testing.T) {
	p, _ := Build([]string{"static"}, Options{StaticPrices: map[string]string{"VTX:USD": "0.10"}})
	_, err := p[0].Fetch(context.Background(), "BTC:USD")
	require.Error(t, err)
}
