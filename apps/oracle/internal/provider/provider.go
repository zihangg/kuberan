// Package provider defines the interface for fetching security prices from external data sources.
package provider

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Security represents a security from the Kuberan API, containing
// the fields needed by price providers to fetch quotes.
type Security struct {
	ID        uint
	Symbol    string
	AssetType string
	Exchange  string
	Network   string
	Currency  string
}

// PriceResult represents a successfully fetched price for a security.
type PriceResult struct {
	SecurityID uint
	Price      int64 // cents
	RecordedAt time.Time
}

// FetchError represents a failed price fetch for a specific security.
type FetchError struct {
	SecurityID uint
	Symbol     string
	Err        error
}

// Error implements the error interface.
func (e *FetchError) Error() string {
	return fmt.Sprintf("failed to fetch price for %s (ID %d): %v", e.Symbol, e.SecurityID, e.Err)
}

// Provider fetches current market prices for a set of securities.
type Provider interface {
	// Name returns the provider's display name (e.g., "Yahoo Finance", "CoinGecko").
	Name() string

	// Supports returns true if this provider can fetch prices for the given asset type.
	Supports(assetType string) bool

	// FetchPrices fetches current prices for the given securities.
	// Returns successful results and any per-security errors.
	// A provider should return as many prices as possible, even if some fail.
	FetchPrices(ctx context.Context, securities []Security) ([]PriceResult, []FetchError)
}

// YahooProvider fetches prices from Yahoo Finance for stocks, ETFs, bonds, and REITs.
type YahooProvider struct {
	httpClient *http.Client
}

// NewYahooProvider creates a new Yahoo Finance price provider.
func NewYahooProvider(httpClient *http.Client) *YahooProvider {
	return &YahooProvider{httpClient: httpClient}
}

// Name returns the provider's display name.
func (p *YahooProvider) Name() string { return "Yahoo Finance" }

// Supports returns true for stock, etf, bond, and reit asset types.
func (p *YahooProvider) Supports(assetType string) bool {
	switch assetType {
	case "stock", "etf", "bond", "reit":
		return true
	default:
		return false
	}
}

// FetchPrices fetches current prices from Yahoo Finance.
func (p *YahooProvider) FetchPrices(_ context.Context, _ []Security) ([]PriceResult, []FetchError) {
	// TODO: implement in Phase 5
	return nil, nil
}

// CoinGeckoProvider fetches prices from CoinGecko for cryptocurrencies.
type CoinGeckoProvider struct {
	httpClient *http.Client
}

// NewCoinGeckoProvider creates a new CoinGecko price provider.
func NewCoinGeckoProvider(httpClient *http.Client) *CoinGeckoProvider {
	return &CoinGeckoProvider{httpClient: httpClient}
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
