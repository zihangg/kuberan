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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := yahooQuoteResponse{}
		resp.QuoteResponse.Result = []yahooQuoteResult{
			{Symbol: "AAPL", RegularMarketPrice: 178.72},
			{Symbol: "MSFT", RegularMarketPrice: 420.55},
			{Symbol: "GOOGL", RegularMarketPrice: 175.03},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := yahooQuoteResponse{}
		resp.QuoteResponse.Result = []yahooQuoteResult{
			{Symbol: "AAPL", RegularMarketPrice: 178.72},
			{Symbol: "MSFT", RegularMarketPrice: 420.55},
			// MISSING: no result for FAKESYM
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
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
	var capturedSymbols string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSymbols = r.URL.Query().Get("symbols")
		resp := yahooQuoteResponse{}
		resp.QuoteResponse.Result = []yahooQuoteResult{
			{Symbol: "SHOP.TO", RegularMarketPrice: 100.00},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
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
	if !strings.Contains(capturedSymbols, "SHOP.TO") {
		t.Errorf("expected URL to contain SHOP.TO, got symbols=%s", capturedSymbols)
	}
}

func TestYahooProvider_FetchPrices_ProviderSymbol(t *testing.T) {
	var capturedSymbols string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSymbols = r.URL.Query().Get("symbols")
		resp := yahooQuoteResponse{}
		resp.QuoteResponse.Result = []yahooQuoteResult{
			{Symbol: "1023.KL", RegularMarketPrice: 6.50},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
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
	if !strings.Contains(capturedSymbols, "1023.KL") {
		t.Errorf("expected URL to contain 1023.KL, got symbols=%s", capturedSymbols)
	}
	if strings.Contains(capturedSymbols, "CIMB.KL") {
		t.Errorf("should NOT contain CIMB.KL, got symbols=%s", capturedSymbols)
	}
}

func TestYahooProvider_FetchPrices_BatchSplit(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		symbols := strings.Split(r.URL.Query().Get("symbols"), ",")
		resp := yahooQuoteResponse{}
		for _, sym := range symbols {
			resp.QuoteResponse.Result = append(resp.QuoteResponse.Result, yahooQuoteResult{
				Symbol:             sym,
				RegularMarketPrice: 100.00,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := &YahooProvider{httpClient: server.Client(), baseURL: server.URL}

	// Create 60 securities.
	securities := make([]Security, 60)
	for i := range securities {
		securities[i] = Security{
			ID:        uint(i + 1),
			Symbol:    "SYM" + strings.Repeat("X", i),
			AssetType: "stock",
		}
	}

	results, _ := p.FetchPrices(context.Background(), securities)
	if got := requestCount.Load(); got != 2 {
		t.Errorf("expected 2 HTTP requests (50+10), got %d", got)
	}
	if len(results) != 60 {
		t.Errorf("expected 60 results, got %d", len(results))
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := yahooQuoteResponse{}
		resp.QuoteResponse.Result = []yahooQuoteResult{
			{Symbol: "DEAD", RegularMarketPrice: 0},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
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
