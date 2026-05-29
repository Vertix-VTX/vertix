package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"cosmossdk.io/math"
)

const coinGeckoBase = "https://api.coingecko.com"

type coinGecko struct {
	base   string
	client *http.Client
}

func init() {
	Register("coingecko", func(Options) (PriceProvider, error) {
		return newCoinGecko(coinGeckoBase), nil
	})
}

func newCoinGecko(base string) *coinGecko {
	return &coinGecko{base: base, client: &http.Client{Timeout: 8 * time.Second}}
}

func (p *coinGecko) Name() string { return "coingecko" }

func (p *coinGecko) Fetch(ctx context.Context, symbol string) (math.LegacyDec, error) {
	u := fmt.Sprintf("%s/api/v3/simple/price?ids=%s&vs_currencies=usd", p.base, url.QueryEscape(symbol))
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
		return math.LegacyDec{}, fmt.Errorf("coingecko: status %d", resp.StatusCode)
	}
	var body map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return math.LegacyDec{}, fmt.Errorf("coingecko: decode: %w", err)
	}
	entry, ok := body[symbol]
	if !ok {
		return math.LegacyDec{}, fmt.Errorf("coingecko: no entry for %q", symbol)
	}
	usd, ok := entry["usd"]
	if !ok {
		return math.LegacyDec{}, fmt.Errorf("coingecko: no usd price for %q", symbol)
	}
	price, err := math.LegacyNewDecFromStr(strconv.FormatFloat(usd, 'f', -1, 64))
	if err != nil {
		return math.LegacyDec{}, fmt.Errorf("coingecko: parse price for %q: %w", symbol, err)
	}
	if !price.IsPositive() {
		return math.LegacyDec{}, fmt.Errorf("coingecko: non-positive usd price for %q", symbol)
	}
	return price, nil
}
