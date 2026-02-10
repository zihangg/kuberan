package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

const (
	yahooBaseURL  = "https://query1.finance.yahoo.com/v7/finance/quote"
	yahooBatchMax = 50
	yahooUA       = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)"
)

// exchangeSuffixes maps exchange codes to Yahoo Finance ticker suffixes.
var exchangeSuffixes = map[string]string{
	"TSX":      ".TO",
	"TSXV":     ".V",
	"LSE":      ".L",
	"HKEX":     ".HK",
	"ASX":      ".AX",
	"NSE":      ".NS",
	"BSE":      ".BO",
	"SGX":      ".SI",
	"KRX":      ".KS",
	"KOSDAQ":   ".KQ",
	"BURSA":    ".KL",
	"JPX":      ".T",
	"FRA":      ".F",
	"XETRA":    ".DE",
	"SIX":      ".SW",
	"EURONEXT": ".PA",
}

// yahooQuoteResponse is the top-level Yahoo Finance API response.
type yahooQuoteResponse struct {
	QuoteResponse struct {
		Result []yahooQuoteResult `json:"result"`
		Error  *json.RawMessage   `json:"error"`
	} `json:"quoteResponse"`
}

// yahooQuoteResult is a single quote result from Yahoo Finance.
type yahooQuoteResult struct {
	Symbol             string  `json:"symbol"`
	RegularMarketPrice float64 `json:"regularMarketPrice"`
}

// YahooProvider fetches prices from Yahoo Finance for stocks, ETFs, bonds, and REITs.
type YahooProvider struct {
	httpClient *http.Client
	baseURL    string // overridable for tests
}

// NewYahooProvider creates a new Yahoo Finance price provider.
func NewYahooProvider(httpClient *http.Client) *YahooProvider {
	return &YahooProvider{httpClient: httpClient, baseURL: yahooBaseURL}
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

// buildYahooSymbol converts a security to a Yahoo-compatible ticker.
// If ProviderSymbol is set, it is used directly. Otherwise, the symbol
// is combined with the exchange suffix from the exchangeSuffixes map.
func buildYahooSymbol(sec Security) string {
	if sec.ProviderSymbol != "" {
		return sec.ProviderSymbol
	}
	if suffix, ok := exchangeSuffixes[sec.Exchange]; ok {
		return sec.Symbol + suffix
	}
	return sec.Symbol
}

// FetchPrices fetches current prices from Yahoo Finance.
func (p *YahooProvider) FetchPrices(ctx context.Context, securities []Security) ([]PriceResult, []FetchError) {
	if len(securities) == 0 {
		return nil, nil
	}

	// Build Yahoo tickers and maintain mapping back to security IDs.
	tickerToSec := make(map[string]Security, len(securities))
	tickers := make([]string, 0, len(securities))
	for _, sec := range securities {
		ticker := buildYahooSymbol(sec)
		tickerToSec[ticker] = sec
		tickers = append(tickers, ticker)
	}

	// Split into batches.
	var allResults []PriceResult
	var allErrors []FetchError
	now := time.Now().UTC()

	for i := 0; i < len(tickers); i += yahooBatchMax {
		end := min(i+yahooBatchMax, len(tickers))
		batch := tickers[i:end]

		results, fetchErrors := p.fetchBatch(ctx, batch, tickerToSec, now)
		allResults = append(allResults, results...)
		allErrors = append(allErrors, fetchErrors...)
	}

	return allResults, allErrors
}

// fetchBatch fetches prices for a single batch of tickers.
func (p *YahooProvider) fetchBatch(ctx context.Context, tickers []string, tickerToSec map[string]Security, now time.Time) ([]PriceResult, []FetchError) {
	url := p.baseURL + "?symbols=" + strings.Join(tickers, ",")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, batchErrors(tickers, tickerToSec, fmt.Errorf("building request: %w", err))
	}
	req.Header.Set("User-Agent", yahooUA)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, batchErrors(tickers, tickerToSec, fmt.Errorf("http request: %w", err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, batchErrors(tickers, tickerToSec, fmt.Errorf("unexpected status %d", resp.StatusCode))
	}

	var quoteResp yahooQuoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&quoteResp); err != nil {
		return nil, batchErrors(tickers, tickerToSec, fmt.Errorf("decoding response: %w", err))
	}

	// Index results by symbol for lookup.
	resultMap := make(map[string]float64, len(quoteResp.QuoteResponse.Result))
	for _, r := range quoteResp.QuoteResponse.Result {
		resultMap[r.Symbol] = r.RegularMarketPrice
	}

	var results []PriceResult
	var fetchErrors []FetchError

	for _, ticker := range tickers {
		sec := tickerToSec[ticker]
		price, found := resultMap[ticker]
		if !found {
			fetchErrors = append(fetchErrors, FetchError{
				SecurityID: sec.ID,
				Symbol:     sec.Symbol,
				Err:        fmt.Errorf("symbol %s not found in response", ticker),
			})
			continue
		}
		if price == 0 {
			fetchErrors = append(fetchErrors, FetchError{
				SecurityID: sec.ID,
				Symbol:     sec.Symbol,
				Err:        fmt.Errorf("zero price for %s", ticker),
			})
			continue
		}
		results = append(results, PriceResult{
			SecurityID: sec.ID,
			Price:      int64(math.Round(price * 100)),
			RecordedAt: now,
		})
	}

	return results, fetchErrors
}

// batchErrors creates FetchErrors for all tickers in a failed batch.
func batchErrors(tickers []string, tickerToSec map[string]Security, err error) []FetchError {
	errors := make([]FetchError, len(tickers))
	for i, ticker := range tickers {
		sec := tickerToSec[ticker]
		errors[i] = FetchError{
			SecurityID: sec.ID,
			Symbol:     sec.Symbol,
			Err:        err,
		}
	}
	return errors
}
