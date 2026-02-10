// Package provider defines the interface for fetching security prices from external data sources.
package provider

import (
	"context"
	"fmt"
	"time"
)

// Security represents a security from the Kuberan API, containing
// the fields needed by price providers to fetch quotes.
type Security struct {
	ID             uint
	Symbol         string
	AssetType      string
	Exchange       string
	ProviderSymbol string
	Network        string
	Currency       string
}

// PriceResult represents a successfully fetched price for a security.
type PriceResult struct {
	SecurityID uint
	Price      int64  // cents in the native currency reported by the data source
	Currency   string // ISO 4217 currency code from the data source (e.g. "USD", "MYR", "GBP")
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
