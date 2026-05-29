package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"cosmossdk.io/math"
)

const binanceBase = "https://api.binance.com"

type binance struct {
	base   string
	client *http.Client
}

func init() {
	Register("binance", func(Options) (PriceProvider, error) {
		return newBinance(binanceBase), nil
	})
}

func newBinance(base string) *binance {
	return &binance{base: base, client: &http.Client{Timeout: 8 * time.Second}}
}

func (p *binance) Name() string { return "binance" }

func (p *binance) Fetch(ctx context.Context, symbol string) (math.LegacyDec, error) {
	u := fmt.Sprintf("%s/api/v3/ticker/price?symbol=%s", p.base, url.QueryEscape(symbol))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return math.LegacyDec{}, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return math.LegacyDec{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return math.LegacyDec{}, fmt.Errorf("binance: status %d", resp.StatusCode)
	}
	var body struct {
		Price string `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return math.LegacyDec{}, fmt.Errorf("binance: decode: %w", err)
	}
	if body.Price == "" {
		return math.LegacyDec{}, fmt.Errorf("binance: empty price for %q", symbol)
	}
	return math.LegacyNewDecFromStr(body.Price)
}
