package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// v8ChartResponse builds a v8 chart JSON response for a single symbol.
func v8ChartResponse(symbol string, price float64) yahooChartResponse {
	var resp yahooChartResponse
	resp.Chart.Result = []struct {
		Meta struct {
			Symbol             string  `json:"symbol"`
			RegularMarketPrice float64 `json:"regularMarketPrice"`
		} `json:"meta"`
	}{
		{Meta: struct {
			Symbol             string  `json:"symbol"`
			RegularMarketPrice float64 `json:"regularMarketPrice"`
		}{Symbol: symbol, RegularMarketPrice: price}},
	}
	return resp
}

// v8ChartErrorResponse builds a v8 chart error JSON response.
func v8ChartErrorResponse(code, description string) yahooChartResponse {
	var resp yahooChartResponse
	resp.Chart.Error = &struct {
		Code        string `json:"code"`
		Description string `json:"description"`
	}{Code: code, Description: description}
	return resp
}

// newV8MockServer creates a test server that serves v8 chart responses per symbol.
// priceMap maps ticker (from URL path) to price. Tickers not in the map get a chart error.
func newV8MockServer(priceMap map[string]float64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract ticker from path: /{ticker}
		ticker := strings.TrimPrefix(r.URL.Path, "/")
		w.Header().Set("Content-Type", "application/json")

		price, ok := priceMap[ticker]
		if !ok {
			_ = json.NewEncoder(w).Encode(v8ChartErrorResponse("Not Found", "No data found, symbol may be delisted"))
			return
		}
		_ = json.NewEncoder(w).Encode(v8ChartResponse(ticker, price))
	}))
}

func TestYahooProvider_Supports(t *testing.T) {
	p := NewYahooProvider(http.DefaultClient)

	supported := []string{"stock", "etf", "bond", "reit"}
	for _, at := range supported {
		if !p.Supports(at) {
			t.Errorf("expected Supports(%q) = true", at)
		}
	}

	unsupported := []string{"crypto", "commodity", ""}
	for _, at := range unsupported {
		if p.Supports(at) {
			t.Errorf("expected Supports(%q) = false", at)
		}
	}
}

func TestYahooProvider_FetchPrices_Success(t *testing.T) {
	server := newV8MockServer(map[string]float64{
		"AAPL":  178.72,
		"MSFT":  420.55,
		"GOOGL": 175.03,
	})
	defer server.Close()

	p := &YahooProvider{httpClient: server.Client(), baseURL: server.URL}
	securities := []Security{
		{ID: 1, Symbol: "AAPL", AssetType: "stock"},
		{ID: 2, Symbol: "MSFT", AssetType: "stock"},
		{ID: 3, Symbol: "GOOGL", AssetType: "stock"},
	}

	results, fetchErrors := p.FetchPrices(context.Background(), securities)
	if len(fetchErrors) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(fetchErrors), fetchErrors)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	expected := map[uint]int64{
		1: 17872,
		2: 42055,
		3: 17503,
	}
	for _, r := range results {
		want, ok := expected[r.SecurityID]
		if !ok {
			t.Errorf("unexpected security ID %d", r.SecurityID)
			continue
		}
		if r.Price != want {
			t.Errorf("security %d: got price %d, want %d", r.SecurityID, r.Price, want)
		}
	}
}

func TestYahooProvider_FetchPrices_PartialFailure(t *testing.T) {
	// Only AAPL and MSFT have prices; FAKESYM is missing → chart error.
	server := newV8MockServer(map[string]float64{
		"AAPL": 178.72,
		"MSFT": 420.55,
	})
	defer server.Close()

	p := &YahooProvider{httpClient: server.Client(), baseURL: server.URL}
	securities := []Security{
		{ID: 1, Symbol: "AAPL", AssetType: "stock"},
		{ID: 2, Symbol: "MSFT", AssetType: "stock"},
		{ID: 3, Symbol: "FAKESYM", AssetType: "stock"},
	}

	results, fetchErrors := p.FetchPrices(context.Background(), securities)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if len(fetchErrors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(fetchErrors))
	}
	if fetchErrors[0].SecurityID != 3 {
		t.Errorf("expected error for security ID 3, got %d", fetchErrors[0].SecurityID)
	}
}

func TestYahooProvider_FetchPrices_ExchangeSuffix(t *testing.T) {
	var capturedPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPaths = append(capturedPaths, r.URL.Path)
		ticker := strings.TrimPrefix(r.URL.Path, "/")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v8ChartResponse(ticker, 100.00))
	}))
	defer server.Close()

	p := &YahooProvider{httpClient: server.Client(), baseURL: server.URL}
	securities := []Security{
		{ID: 1, Symbol: "SHOP", AssetType: "stock", Exchange: "TSX"},
	}

	results, fetchErrors := p.FetchPrices(context.Background(), securities)
	if len(fetchErrors) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(fetchErrors), fetchErrors)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	found := false
	for _, path := range capturedPaths {
		if strings.Contains(path, "SHOP.TO") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected request path to contain SHOP.TO, got paths=%v", capturedPaths)
	}
}

func TestYahooProvider_FetchPrices_ProviderSymbol(t *testing.T) {
	var capturedPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPaths = append(capturedPaths, r.URL.Path)
		ticker := strings.TrimPrefix(r.URL.Path, "/")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v8ChartResponse(ticker, 6.50))
	}))
	defer server.Close()

	p := &YahooProvider{httpClient: server.Client(), baseURL: server.URL}
	securities := []Security{
		{ID: 1, Symbol: "CIMB", AssetType: "stock", Exchange: "BURSA", ProviderSymbol: "1023.KL"},
	}

	results, fetchErrors := p.FetchPrices(context.Background(), securities)
	if len(fetchErrors) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(fetchErrors), fetchErrors)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Price != 650 {
		t.Errorf("expected price 650 cents, got %d", results[0].Price)
	}

	// Verify the request used provider_symbol (1023.KL), not CIMB.KL.
	foundProvider := false
	foundWrong := false
	for _, path := range capturedPaths {
		if strings.Contains(path, "1023.KL") {
			foundProvider = true
		}
		if strings.Contains(path, "CIMB.KL") {
			foundWrong = true
		}
	}
	if !foundProvider {
		t.Errorf("expected request path to contain 1023.KL, got paths=%v", capturedPaths)
	}
	if foundWrong {
		t.Errorf("should NOT contain CIMB.KL, got paths=%v", capturedPaths)
	}
}

func TestYahooProvider_FetchPrices_Concurrent(t *testing.T) {
	var maxInFlight atomic.Int32
	var curInFlight atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := curInFlight.Add(1)
		// Track peak concurrency.
		for {
			old := maxInFlight.Load()
			if cur <= old || maxInFlight.CompareAndSwap(old, cur) {
				break
			}
		}
		defer curInFlight.Add(-1)

		ticker := strings.TrimPrefix(r.URL.Path, "/")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v8ChartResponse(ticker, 100.00))
	}))
	defer server.Close()

	p := &YahooProvider{httpClient: server.Client(), baseURL: server.URL}

	// Create 15 securities.
	securities := make([]Security, 15)
	for i := range securities {
		securities[i] = Security{
			ID:        uint(i + 1),
			Symbol:    "SYM" + string(rune('A'+i)),
			AssetType: "stock",
		}
	}

	results, fetchErrors := p.FetchPrices(context.Background(), securities)
	if len(fetchErrors) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(fetchErrors), fetchErrors)
	}
	if len(results) != 15 {
		t.Errorf("expected 15 results, got %d", len(results))
	}
	if peak := maxInFlight.Load(); peak > int32(yahooMaxConcurrent) {
		t.Errorf("peak concurrency %d exceeded limit %d", peak, yahooMaxConcurrent)
	}
}

func TestYahooProvider_FetchPrices_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	p := &YahooProvider{httpClient: server.Client(), baseURL: server.URL}
	securities := []Security{
		{ID: 1, Symbol: "AAPL", AssetType: "stock"},
		{ID: 2, Symbol: "MSFT", AssetType: "stock"},
	}

	results, fetchErrors := p.FetchPrices(context.Background(), securities)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
	if len(fetchErrors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(fetchErrors))
	}
	for _, fe := range fetchErrors {
		if !strings.Contains(fe.Err.Error(), "500") {
			t.Errorf("expected error to mention 500, got: %v", fe.Err)
		}
	}
}

func TestYahooProvider_FetchPrices_ZeroPrice(t *testing.T) {
	server := newV8MockServer(map[string]float64{
		"DEAD": 0,
	})
	defer server.Close()

	p := &YahooProvider{httpClient: server.Client(), baseURL: server.URL}
	securities := []Security{
		{ID: 1, Symbol: "DEAD", AssetType: "stock"},
	}

	results, fetchErrors := p.FetchPrices(context.Background(), securities)
	if len(results) != 0 {
		t.Errorf("expected 0 results for zero price, got %d", len(results))
	}
	if len(fetchErrors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(fetchErrors))
	}
	if !strings.Contains(fetchErrors[0].Err.Error(), "zero price") {
		t.Errorf("expected error about zero price, got: %v", fetchErrors[0].Err)
	}
}

func TestYahooProvider_FetchPrices_ChartError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v8ChartErrorResponse("Not Found", "No data found, symbol may be delisted"))
	}))
	defer server.Close()

	p := &YahooProvider{httpClient: server.Client(), baseURL: server.URL}
	securities := []Security{
		{ID: 1, Symbol: "DELISTED", AssetType: "stock"},
	}

	results, fetchErrors := p.FetchPrices(context.Background(), securities)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
	if len(fetchErrors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(fetchErrors))
	}
	if !strings.Contains(fetchErrors[0].Err.Error(), "Not Found") {
		t.Errorf("expected error to mention 'Not Found', got: %v", fetchErrors[0].Err)
	}
}
