package oracle

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/kuberan/oracle/internal/client"
	"github.com/kuberan/oracle/internal/config"
	"github.com/kuberan/oracle/internal/provider"
)

// mockClient implements SecurityClient for testing.
type mockClient struct {
	getSecuritiesFn    func(ctx context.Context) ([]client.Security, error)
	recordPricesFn     func(ctx context.Context, prices []client.RecordPriceEntry) (int, error)
	computeSnapshotsFn func(ctx context.Context) (int, error)
}

func (m *mockClient) GetSecurities(ctx context.Context) ([]client.Security, error) {
	return m.getSecuritiesFn(ctx)
}

func (m *mockClient) RecordPrices(ctx context.Context, prices []client.RecordPriceEntry) (int, error) {
	return m.recordPricesFn(ctx, prices)
}

func (m *mockClient) ComputeSnapshots(ctx context.Context) (int, error) {
	return m.computeSnapshotsFn(ctx)
}

// mockProvider implements provider.Provider for testing.
type mockProvider struct {
	name       string
	supports   func(assetType string) bool
	fetchPrices func(ctx context.Context, securities []provider.Security) ([]provider.PriceResult, []provider.FetchError)
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) Supports(assetType string) bool { return m.supports(assetType) }

func (m *mockProvider) FetchPrices(ctx context.Context, securities []provider.Security) ([]provider.PriceResult, []provider.FetchError) {
	return m.fetchPrices(ctx, securities)
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func defaultConfig(snapshots bool) *config.Config {
	return &config.Config{
		KuberanAPIURL:    "http://localhost:8080",
		PipelineAPIKey:   "test-key",
		RequestTimeout:   30 * time.Second,
		ComputeSnapshots: snapshots,
	}
}

func TestOracle_Run_FullFlow(t *testing.T) {
	now := time.Now().UTC()

	var recordedPrices []client.RecordPriceEntry
	snapshotsCalled := false

	mc := &mockClient{
		getSecuritiesFn: func(_ context.Context) ([]client.Security, error) {
			return []client.Security{
				{ID: 1, Symbol: "AAPL", AssetType: "stock", Currency: "USD", Exchange: "NASDAQ"},
				{ID: 2, Symbol: "MSFT", AssetType: "stock", Currency: "USD", Exchange: "NASDAQ"},
				{ID: 3, Symbol: "SHOP", AssetType: "stock", Currency: "CAD", Exchange: "TSX"},
				{ID: 4, Symbol: "BTC", AssetType: "crypto", Currency: "USD"},
				{ID: 5, Symbol: "ETH", AssetType: "crypto", Currency: "USD"},
			}, nil
		},
		recordPricesFn: func(_ context.Context, prices []client.RecordPriceEntry) (int, error) {
			recordedPrices = prices
			return len(prices), nil
		},
		computeSnapshotsFn: func(_ context.Context) (int, error) {
			snapshotsCalled = true
			return 3, nil
		},
	}

	yahooProvider := &mockProvider{
		name:     "Yahoo Finance",
		supports: func(at string) bool { return at == "stock" || at == "etf" || at == "reit" || at == "bond" },
		fetchPrices: func(_ context.Context, secs []provider.Security) ([]provider.PriceResult, []provider.FetchError) {
			results := make([]provider.PriceResult, len(secs))
			for i, s := range secs {
				results[i] = provider.PriceResult{SecurityID: s.ID, Price: 10000 + int64(s.ID)*100, RecordedAt: now}
			}
			return results, nil
		},
	}

	geckoProvider := &mockProvider{
		name:     "CoinGecko",
		supports: func(at string) bool { return at == "crypto" },
		fetchPrices: func(_ context.Context, secs []provider.Security) ([]provider.PriceResult, []provider.FetchError) {
			results := make([]provider.PriceResult, len(secs))
			for i, s := range secs {
				results[i] = provider.PriceResult{SecurityID: s.ID, Price: 500000 + int64(s.ID)*1000, RecordedAt: now}
			}
			return results, nil
		},
	}

	orc := NewOracle(mc, []provider.Provider{yahooProvider, geckoProvider}, defaultConfig(true), newTestLogger())
	result, err := orc.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.SecuritiesFetched != 5 {
		t.Errorf("SecuritiesFetched = %d, want 5", result.SecuritiesFetched)
	}
	if result.PricesRecorded != 5 {
		t.Errorf("PricesRecorded = %d, want 5", result.PricesRecorded)
	}
	if result.SnapshotsRecorded != 3 {
		t.Errorf("SnapshotsRecorded = %d, want 3", result.SnapshotsRecorded)
	}
	if len(result.Errors) != 0 {
		t.Errorf("Errors = %d, want 0", len(result.Errors))
	}
	if len(recordedPrices) != 5 {
		t.Errorf("recorded %d prices, want 5", len(recordedPrices))
	}
	if !snapshotsCalled {
		t.Error("ComputeSnapshots was not called")
	}
	if result.Duration <= 0 {
		t.Error("Duration should be positive")
	}
}

func TestOracle_Run_PartialProviderFailure(t *testing.T) {
	now := time.Now().UTC()

	mc := &mockClient{
		getSecuritiesFn: func(_ context.Context) ([]client.Security, error) {
			return []client.Security{
				{ID: 1, Symbol: "AAPL", AssetType: "stock", Currency: "USD"},
				{ID: 2, Symbol: "MSFT", AssetType: "stock", Currency: "USD"},
				{ID: 3, Symbol: "FAIL", AssetType: "stock", Currency: "USD"},
				{ID: 4, Symbol: "BTC", AssetType: "crypto", Currency: "USD"},
				{ID: 5, Symbol: "ETH", AssetType: "crypto", Currency: "USD"},
			}, nil
		},
		recordPricesFn: func(_ context.Context, prices []client.RecordPriceEntry) (int, error) {
			return len(prices), nil
		},
		computeSnapshotsFn: func(_ context.Context) (int, error) {
			return 2, nil
		},
	}

	yahooProvider := &mockProvider{
		name:     "Yahoo Finance",
		supports: func(at string) bool { return at == "stock" },
		fetchPrices: func(_ context.Context, secs []provider.Security) ([]provider.PriceResult, []provider.FetchError) {
			var results []provider.PriceResult
			var errs []provider.FetchError
			for _, s := range secs {
				if s.Symbol == "FAIL" {
					errs = append(errs, provider.FetchError{SecurityID: s.ID, Symbol: s.Symbol, Err: errors.New("not found")})
				} else {
					results = append(results, provider.PriceResult{SecurityID: s.ID, Price: 10000, RecordedAt: now})
				}
			}
			return results, errs
		},
	}

	geckoProvider := &mockProvider{
		name:     "CoinGecko",
		supports: func(at string) bool { return at == "crypto" },
		fetchPrices: func(_ context.Context, secs []provider.Security) ([]provider.PriceResult, []provider.FetchError) {
			results := make([]provider.PriceResult, len(secs))
			for i, s := range secs {
				results[i] = provider.PriceResult{SecurityID: s.ID, Price: 500000, RecordedAt: now}
			}
			return results, nil
		},
	}

	orc := NewOracle(mc, []provider.Provider{yahooProvider, geckoProvider}, defaultConfig(true), newTestLogger())
	result, err := orc.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.PricesRecorded != 4 {
		t.Errorf("PricesRecorded = %d, want 4", result.PricesRecorded)
	}
	if len(result.Errors) != 1 {
		t.Errorf("Errors = %d, want 1", len(result.Errors))
	}
	if len(result.Errors) > 0 && result.Errors[0].Symbol != "FAIL" {
		t.Errorf("error symbol = %q, want %q", result.Errors[0].Symbol, "FAIL")
	}
}

func TestOracle_Run_NoSecurities(t *testing.T) {
	providerCalled := false

	mc := &mockClient{
		getSecuritiesFn: func(_ context.Context) ([]client.Security, error) {
			return []client.Security{}, nil
		},
		recordPricesFn: func(_ context.Context, _ []client.RecordPriceEntry) (int, error) {
			t.Error("RecordPrices should not be called")
			return 0, nil
		},
		computeSnapshotsFn: func(_ context.Context) (int, error) {
			t.Error("ComputeSnapshots should not be called")
			return 0, nil
		},
	}

	mp := &mockProvider{
		name:     "Test",
		supports: func(_ string) bool { return true },
		fetchPrices: func(_ context.Context, _ []provider.Security) ([]provider.PriceResult, []provider.FetchError) {
			providerCalled = true
			return nil, nil
		},
	}

	orc := NewOracle(mc, []provider.Provider{mp}, defaultConfig(true), newTestLogger())
	result, err := orc.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.SecuritiesFetched != 0 {
		t.Errorf("SecuritiesFetched = %d, want 0", result.SecuritiesFetched)
	}
	if result.PricesRecorded != 0 {
		t.Errorf("PricesRecorded = %d, want 0", result.PricesRecorded)
	}
	if providerCalled {
		t.Error("provider should not be called when no securities")
	}
}

func TestOracle_Run_UnsupportedAssetType(t *testing.T) {
	mc := &mockClient{
		getSecuritiesFn: func(_ context.Context) ([]client.Security, error) {
			return []client.Security{
				{ID: 1, Symbol: "BOND1", AssetType: "bond", Currency: "USD"},
			}, nil
		},
		recordPricesFn: func(_ context.Context, _ []client.RecordPriceEntry) (int, error) {
			t.Error("RecordPrices should not be called when no prices fetched")
			return 0, nil
		},
		computeSnapshotsFn: func(_ context.Context) (int, error) {
			t.Error("ComputeSnapshots should not be called when no prices fetched")
			return 0, nil
		},
	}

	// Provider that only supports crypto — bond won't match.
	cryptoOnly := &mockProvider{
		name:     "CryptoOnly",
		supports: func(at string) bool { return at == "crypto" },
		fetchPrices: func(_ context.Context, _ []provider.Security) ([]provider.PriceResult, []provider.FetchError) {
			t.Error("FetchPrices should not be called for unsupported asset type")
			return nil, nil
		},
	}

	orc := NewOracle(mc, []provider.Provider{cryptoOnly}, defaultConfig(true), newTestLogger())
	result, err := orc.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.SecuritiesFetched != 1 {
		t.Errorf("SecuritiesFetched = %d, want 1", result.SecuritiesFetched)
	}
	if result.PricesRecorded != 0 {
		t.Errorf("PricesRecorded = %d, want 0", result.PricesRecorded)
	}
}

func TestOracle_Run_GetSecuritiesFails(t *testing.T) {
	mc := &mockClient{
		getSecuritiesFn: func(_ context.Context) ([]client.Security, error) {
			return nil, errors.New("connection refused")
		},
		recordPricesFn: func(_ context.Context, _ []client.RecordPriceEntry) (int, error) {
			return 0, nil
		},
		computeSnapshotsFn: func(_ context.Context) (int, error) {
			return 0, nil
		},
	}

	orc := NewOracle(mc, nil, defaultConfig(true), newTestLogger())
	result, err := orc.Run(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if err.Error() != "connection refused" {
		t.Errorf("error = %q, want %q", err.Error(), "connection refused")
	}
}

func TestOracle_Run_RecordPricesFails(t *testing.T) {
	now := time.Now().UTC()

	mc := &mockClient{
		getSecuritiesFn: func(_ context.Context) ([]client.Security, error) {
			return []client.Security{
				{ID: 1, Symbol: "AAPL", AssetType: "stock", Currency: "USD"},
			}, nil
		},
		recordPricesFn: func(_ context.Context, _ []client.RecordPriceEntry) (int, error) {
			return 0, errors.New("server error")
		},
		computeSnapshotsFn: func(_ context.Context) (int, error) {
			t.Error("ComputeSnapshots should not be called when RecordPrices fails")
			return 0, nil
		},
	}

	mp := &mockProvider{
		name:     "Yahoo Finance",
		supports: func(at string) bool { return at == "stock" },
		fetchPrices: func(_ context.Context, secs []provider.Security) ([]provider.PriceResult, []provider.FetchError) {
			return []provider.PriceResult{
				{SecurityID: secs[0].ID, Price: 17800, RecordedAt: now},
			}, nil
		},
	}

	orc := NewOracle(mc, []provider.Provider{mp}, defaultConfig(true), newTestLogger())
	result, err := orc.Run(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if err.Error() != "server error" {
		t.Errorf("error = %q, want %q", err.Error(), "server error")
	}
}

func TestOracle_Run_SnapshotFailureNonFatal(t *testing.T) {
	now := time.Now().UTC()

	mc := &mockClient{
		getSecuritiesFn: func(_ context.Context) ([]client.Security, error) {
			return []client.Security{
				{ID: 1, Symbol: "AAPL", AssetType: "stock", Currency: "USD"},
			}, nil
		},
		recordPricesFn: func(_ context.Context, prices []client.RecordPriceEntry) (int, error) {
			return len(prices), nil
		},
		computeSnapshotsFn: func(_ context.Context) (int, error) {
			return 0, errors.New("snapshot service unavailable")
		},
	}

	mp := &mockProvider{
		name:     "Yahoo Finance",
		supports: func(at string) bool { return at == "stock" },
		fetchPrices: func(_ context.Context, secs []provider.Security) ([]provider.PriceResult, []provider.FetchError) {
			return []provider.PriceResult{
				{SecurityID: secs[0].ID, Price: 17800, RecordedAt: now},
			}, nil
		},
	}

	orc := NewOracle(mc, []provider.Provider{mp}, defaultConfig(true), newTestLogger())
	result, err := orc.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.PricesRecorded != 1 {
		t.Errorf("PricesRecorded = %d, want 1", result.PricesRecorded)
	}
	if result.SnapshotsRecorded != 0 {
		t.Errorf("SnapshotsRecorded = %d, want 0", result.SnapshotsRecorded)
	}
}

func TestOracle_Run_SnapshotsDisabled(t *testing.T) {
	now := time.Now().UTC()
	snapshotsCalled := false

	mc := &mockClient{
		getSecuritiesFn: func(_ context.Context) ([]client.Security, error) {
			return []client.Security{
				{ID: 1, Symbol: "BTC", AssetType: "crypto", Currency: "USD"},
			}, nil
		},
		recordPricesFn: func(_ context.Context, prices []client.RecordPriceEntry) (int, error) {
			return len(prices), nil
		},
		computeSnapshotsFn: func(_ context.Context) (int, error) {
			snapshotsCalled = true
			return 1, nil
		},
	}

	mp := &mockProvider{
		name:     "CoinGecko",
		supports: func(at string) bool { return at == "crypto" },
		fetchPrices: func(_ context.Context, secs []provider.Security) ([]provider.PriceResult, []provider.FetchError) {
			return []provider.PriceResult{
				{SecurityID: secs[0].ID, Price: 6700000, RecordedAt: now},
			}, nil
		},
	}

	orc := NewOracle(mc, []provider.Provider{mp}, defaultConfig(false), newTestLogger())
	result, err := orc.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if snapshotsCalled {
		t.Error("ComputeSnapshots should not be called when disabled")
	}
	if result.PricesRecorded != 1 {
		t.Errorf("PricesRecorded = %d, want 1", result.PricesRecorded)
	}
	if result.SnapshotsRecorded != 0 {
		t.Errorf("SnapshotsRecorded = %d, want 0", result.SnapshotsRecorded)
	}
}
