package provider

import (
	"context"
	"net/http"
)

// CoinGeckoProvider fetches prices from CoinGecko for cryptocurrencies.
type CoinGeckoProvider struct {
	httpClient *http.Client
	baseURL    string // overridable for tests
}

// NewCoinGeckoProvider creates a new CoinGecko price provider.
func NewCoinGeckoProvider(httpClient *http.Client) *CoinGeckoProvider {
	return &CoinGeckoProvider{
		httpClient: httpClient,
		baseURL:    "https://api.coingecko.com/api/v3/simple/price",
	}
}

// Name returns the provider's display name.
func (p *CoinGeckoProvider) Name() string { return "CoinGecko" }

// Supports returns true for crypto asset type only.
func (p *CoinGeckoProvider) Supports(assetType string) bool {
	return assetType == "crypto"
}

// FetchPrices fetches current prices from CoinGecko.
func (p *CoinGeckoProvider) FetchPrices(_ context.Context, _ []Security) ([]PriceResult, []FetchError) {
	// TODO: implement in Phase 6
	return nil, nil
}
