package provider

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
)

// PriceProvider fetches a spot price for a provider-specific symbol.
type PriceProvider interface {
	Name() string
	Fetch(ctx context.Context, symbol string) (math.LegacyDec, error)
}

// Constructor builds a provider from optional config (e.g. static prices).
type Constructor func(opts Options) (PriceProvider, error)

// Options carries provider construction inputs.
type Options struct {
	// StaticPrices maps symbol -> decimal string, used by the static provider.
	StaticPrices map[string]string
}

var registry = map[string]Constructor{}

// Register adds a provider constructor under name. Called from adapter init().
func Register(name string, c Constructor) {
	registry[name] = c
}

// Build instantiates the named providers. Unknown names are a fatal error.
func Build(names []string, opts Options) ([]PriceProvider, error) {
	out := make([]PriceProvider, 0, len(names))
	for _, n := range names {
		c, ok := registry[n]
		if !ok {
			return nil, fmt.Errorf("unknown provider %q", n)
		}
		p, err := c(opts)
		if err != nil {
			return nil, fmt.Errorf("init provider %q: %w", n, err)
		}
		out = append(out, p)
	}
	return out, nil
}
