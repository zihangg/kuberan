package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCoinGeckoProvider_Supports(t *testing.T) {
	p := NewCoinGeckoProvider(http.DefaultClient)

	if !p.Supports("crypto") {
		t.Error("expected Supports(crypto) = true")
	}

	unsupported := []string{"stock", "etf", "bond", "reit", ""}
	for _, at := range unsupported {
		if p.Supports(at) {
			t.Errorf("expected Supports(%q) = false", at)
		}
	}
}

func TestCoinGeckoProvider_FetchPrices_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]map[string]float64{
			"bitcoin":  {"usd": 67234.56},
			"ethereum": {"usd": 3456.78},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := &CoinGeckoProvider{httpClient: server.Client(), baseURL: server.URL}
	securities := []Security{
		{ID: 1, Symbol: "BTC", AssetType: "crypto", Currency: "USD"},
		{ID: 2, Symbol: "ETH", AssetType: "crypto", Currency: "USD"},
	}

	results, fetchErrors := p.FetchPrices(context.Background(), securities)
	if len(fetchErrors) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(fetchErrors), fetchErrors)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	expected := map[uint]int64{
		1: 6723456,
		2: 345678,
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

func TestCoinGeckoProvider_FetchPrices_UnknownSymbol(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not make HTTP request for unknown symbol only")
	}))
	defer server.Close()

	p := &CoinGeckoProvider{httpClient: server.Client(), baseURL: server.URL}
	securities := []Security{
		{ID: 1, Symbol: "OBSCURECOIN", AssetType: "crypto", Currency: "USD"},
	}

	results, fetchErrors := p.FetchPrices(context.Background(), securities)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
	if len(fetchErrors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(fetchErrors))
	}
	if fetchErrors[0].SecurityID != 1 {
		t.Errorf("expected error for security ID 1, got %d", fetchErrors[0].SecurityID)
	}
	if !strings.Contains(fetchErrors[0].Err.Error(), "no CoinGecko mapping") {
		t.Errorf("expected mapping error, got: %v", fetchErrors[0].Err)
	}
}

func TestCoinGeckoProvider_FetchPrices_PartialResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Return price for bitcoin but not ethereum.
		resp := map[string]map[string]float64{
			"bitcoin": {"usd": 67234.56},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := &CoinGeckoProvider{httpClient: server.Client(), baseURL: server.URL}
	securities := []Security{
		{ID: 1, Symbol: "BTC", AssetType: "crypto", Currency: "USD"},
		{ID: 2, Symbol: "ETH", AssetType: "crypto", Currency: "USD"},
	}

	results, fetchErrors := p.FetchPrices(context.Background(), securities)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if len(fetchErrors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(fetchErrors))
	}
	if results[0].SecurityID != 1 {
		t.Errorf("expected result for security ID 1, got %d", results[0].SecurityID)
	}
	if fetchErrors[0].SecurityID != 2 {
		t.Errorf("expected error for security ID 2, got %d", fetchErrors[0].SecurityID)
	}
}

func TestCoinGeckoProvider_FetchPrices_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	p := &CoinGeckoProvider{httpClient: server.Client(), baseURL: server.URL}
	securities := []Security{
		{ID: 1, Symbol: "BTC", AssetType: "crypto", Currency: "USD"},
		{ID: 2, Symbol: "ETH", AssetType: "crypto", Currency: "USD"},
	}

	results, fetchErrors := p.FetchPrices(context.Background(), securities)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
	if len(fetchErrors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(fetchErrors))
	}
	for _, fe := range fetchErrors {
		if !strings.Contains(fe.Err.Error(), "429") {
			t.Errorf("expected error to mention 429, got: %v", fe.Err)
		}
	}
}

func TestLookupCoinGeckoID_Found(t *testing.T) {
	tests := []struct {
		symbol string
		want   string
	}{
		{"BTC", "bitcoin"},
		{"ETH", "ethereum"},
		{"SOL", "solana"},
	}
	for _, tt := range tests {
		id, ok := LookupCoinGeckoID(tt.symbol)
		if !ok {
			t.Errorf("LookupCoinGeckoID(%q): expected found", tt.symbol)
			continue
		}
		if id != tt.want {
			t.Errorf("LookupCoinGeckoID(%q) = %q, want %q", tt.symbol, id, tt.want)
		}
	}
}

func TestLookupCoinGeckoID_CaseInsensitive(t *testing.T) {
	id, ok := LookupCoinGeckoID("btc")
	if !ok {
		t.Fatal("expected LookupCoinGeckoID(btc) to find match")
	}
	if id != "bitcoin" {
		t.Errorf("got %q, want bitcoin", id)
	}
}

func TestLookupCoinGeckoID_NotFound(t *testing.T) {
	id, ok := LookupCoinGeckoID("DOESNOTEXIST")
	if ok {
		t.Error("expected LookupCoinGeckoID(DOESNOTEXIST) to return false")
	}
	if id != "" {
		t.Errorf("expected empty string, got %q", id)
	}
}
