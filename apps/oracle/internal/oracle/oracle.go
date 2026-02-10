// Package oracle orchestrates fetching prices from providers and pushing them to the Kuberan API.
package oracle

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/kuberan/oracle/internal/client"
	"github.com/kuberan/oracle/internal/config"
	"github.com/kuberan/oracle/internal/provider"
)

// SecurityClient defines the Kuberan API operations needed by the oracle.
type SecurityClient interface {
	GetSecurities(ctx context.Context) ([]client.Security, error)
	RecordPrices(ctx context.Context, prices []client.RecordPriceEntry) (int, error)
	ComputeSnapshots(ctx context.Context) (int, error)
}

// RunResult contains the outcome of an oracle run.
type RunResult struct {
	SecuritiesFetched int
	PricesRecorded    int
	SnapshotsRecorded int
	Errors            []provider.FetchError
	Duration          time.Duration
}

// Oracle fetches security prices from external providers and records them via the Kuberan API.
type Oracle struct {
	client    SecurityClient
	providers []provider.Provider
	config    *config.Config
	logger    *slog.Logger
}

// NewOracle creates a new Oracle instance.
func NewOracle(client SecurityClient, providers []provider.Provider, cfg *config.Config, logger *slog.Logger) *Oracle {
	return &Oracle{
		client:    client,
		providers: providers,
		config:    cfg,
		logger:    logger,
	}
}

// Run executes a single oracle cycle: fetch securities, get prices, record results.
func (o *Oracle) Run(ctx context.Context) (*RunResult, error) {
	start := time.Now()
	result := &RunResult{}

	// 1. Fetch securities from Kuberan API.
	securities, err := o.client.GetSecurities(ctx)
	if err != nil {
		return nil, err
	}
	result.SecuritiesFetched = len(securities)

	if len(securities) == 0 {
		o.logger.Info("no securities found, nothing to do")
		result.Duration = time.Since(start)
		return result, nil
	}

	// 2. Convert to provider types.
	providerSecurities := make([]provider.Security, len(securities))
	for i, s := range securities {
		providerSecurities[i] = provider.Security{
			ID:        s.ID,
			Symbol:    s.Symbol,
			AssetType: s.AssetType,
			Exchange:  s.Exchange,
			Network:   s.Network,
			Currency:  s.Currency,
		}
	}

	// 3. Group by provider.
	groups := make(map[int][]provider.Security) // provider index -> securities
	for _, sec := range providerSecurities {
		matched := false
		for i, p := range o.providers {
			if p.Supports(sec.AssetType) {
				groups[i] = append(groups[i], sec)
				matched = true
				break
			}
		}
		if !matched {
			o.logger.Warn("no provider supports asset type", "symbol", sec.Symbol, "asset_type", sec.AssetType)
		}
	}

	// 4. Fetch prices from each provider concurrently.
	var mu sync.Mutex
	var allResults []provider.PriceResult
	var allErrors []provider.FetchError

	var wg sync.WaitGroup
	for i, secs := range groups {
		wg.Add(1)
		go func(p provider.Provider, securities []provider.Security) {
			defer wg.Done()
			o.logger.Info("fetching prices", "provider", p.Name(), "count", len(securities))
			prices, fetchErrors := p.FetchPrices(ctx, securities)
			mu.Lock()
			allResults = append(allResults, prices...)
			allErrors = append(allErrors, fetchErrors...)
			mu.Unlock()
		}(o.providers[i], secs)
	}
	wg.Wait()

	result.Errors = allErrors

	// 5. If no prices fetched, return early.
	if len(allResults) == 0 {
		o.logger.Info("no prices fetched")
		result.Duration = time.Since(start)
		return result, nil
	}

	// 6. Convert to client price entries and record.
	entries := make([]client.RecordPriceEntry, len(allResults))
	for i, r := range allResults {
		entries[i] = client.RecordPriceEntry{
			SecurityID: r.SecurityID,
			Price:      r.Price,
			RecordedAt: r.RecordedAt.Format(time.RFC3339),
		}
	}

	recorded, err := o.client.RecordPrices(ctx, entries)
	if err != nil {
		return nil, err
	}
	result.PricesRecorded = recorded

	// 7. Trigger snapshots if configured.
	if o.config.ComputeSnapshots {
		snapshots, err := o.client.ComputeSnapshots(ctx)
		if err != nil {
			o.logger.Warn("failed to compute snapshots", "error", err)
		} else {
			result.SnapshotsRecorded = snapshots
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}
