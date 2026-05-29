package provider

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
)

type staticProvider struct {
	prices map[string]math.LegacyDec
}

func init() {
	Register("static", func(opts Options) (PriceProvider, error) {
		parsed := make(map[string]math.LegacyDec, len(opts.StaticPrices))
		for sym, s := range opts.StaticPrices {
			d, err := math.LegacyNewDecFromStr(s)
			if err != nil {
				return nil, fmt.Errorf("static price %q: %w", sym, err)
			}
			parsed[sym] = d
		}
		return &staticProvider{prices: parsed}, nil
	})
}

func (p *staticProvider) Name() string { return "static" }

func (p *staticProvider) Fetch(_ context.Context, symbol string) (math.LegacyDec, error) {
	d, ok := p.prices[symbol]
	if !ok {
		return math.LegacyDec{}, fmt.Errorf("static: no price for symbol %q", symbol)
	}
	return d, nil
}
