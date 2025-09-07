// Package premove contains the application layer logic for pre-movement detection
// and filtering systems. This includes portfolio management, alerting, execution
// quality tracking, and backtesting capabilities.
package premove

import (
	"fmt"
	"time"

	"github.com/sawpanic/cryptorun/src/domain/premove/portfolio"
)

// PortfolioManager handles portfolio-level risk management and candidate filtering
type PortfolioManager struct {
	pruner              *portfolio.Pruner
	correlationProvider portfolio.CorrelationProvider
	lastPruneResult     *portfolio.PruneResult
	lastPruneTime       time.Time
}

// PortfolioConfig holds portfolio management configuration
type PortfolioConfig struct {
	PairwiseCorrMax      float64        `yaml:"pairwise_corr_max"`
	SectorCaps           map[string]int `yaml:"sector_caps"`
	BetaBudgetToBTC      float64        `yaml:"beta_budget_to_btc"`
	MaxSinglePositionPct float64        `yaml:"max_single_position_pct"`
	MaxTotalExposurePct  float64        `yaml:"max_total_exposure_pct"`
}

// SimpleCorrelationProvider provides basic correlation lookups
type SimpleCorrelationProvider struct {
	correlations map[string]map[string]float64
}

// NewSimpleCorrelationProvider creates a correlation provider with static data
func NewSimpleCorrelationProvider() *SimpleCorrelationProvider {
	// Initialize with some basic correlations for testing
	correlations := map[string]map[string]float64{
		"BTC-USD": {"ETH-USD": 0.7, "ADA-USD": 0.6, "SOL-USD": 0.65},
		"ETH-USD": {"BTC-USD": 0.7, "ADA-USD": 0.8, "SOL-USD": 0.75},
		"ADA-USD": {"BTC-USD": 0.6, "ETH-USD": 0.8, "SOL-USD": 0.7},
		"SOL-USD": {"BTC-USD": 0.65, "ETH-USD": 0.75, "ADA-USD": 0.7},
	}

	return &SimpleCorrelationProvider{correlations: correlations}
}

// GetCorrelation returns correlation between two symbols
func (scp *SimpleCorrelationProvider) GetCorrelation(symbol1, symbol2 string) (float64, bool) {
	if symbol1 == symbol2 {
		return 1.0, true
	}

	if correlations, exists := scp.correlations[symbol1]; exists {
		if corr, exists := correlations[symbol2]; exists {
			return corr, true
		}
	}

	// Try reverse lookup
	if correlations, exists := scp.correlations[symbol2]; exists {
		if corr, exists := correlations[symbol1]; exists {
			return corr, true
		}
	}

	return 0.0, false
}

// NewPortfolioManager creates a portfolio manager with default configuration
func NewPortfolioManager() *PortfolioManager {
	return &PortfolioManager{
		pruner:              portfolio.NewPruner(),
		correlationProvider: NewSimpleCorrelationProvider(),
	}
}

// NewPortfolioManagerWithConfig creates a portfolio manager with custom configuration
func NewPortfolioManagerWithConfig(config PortfolioConfig) *PortfolioManager {
	pruner := portfolio.NewPrunerWithConstraints(
		config.PairwiseCorrMax,
		config.SectorCaps,
		config.BetaBudgetToBTC,
		config.MaxSinglePositionPct,
		config.MaxTotalExposurePct,
	)

	return &PortfolioManager{
		pruner:              pruner,
		correlationProvider: NewSimpleCorrelationProvider(),
	}
}

// PrunePostGates filters candidates after gates processing to enforce portfolio constraints
// This is called post-gates, pre-alerts in the runner pipeline
func (pm *PortfolioManager) PrunePostGates(candidates []portfolio.PruneCandidate) (*portfolio.PruneResult, error) {
	if len(candidates) == 0 {
		return &portfolio.PruneResult{
			Accepted:         make([]portfolio.PruneCandidate, 0),
			Rejected:         make([]portfolio.PruneCandidate, 0),
			RejectionReasons: make(map[string]string),
		}, nil
	}

	result := pm.pruner.Prune(candidates, pm.correlationProvider)

	// Cache the result for monitoring/debugging
	pm.lastPruneResult = result
	pm.lastPruneTime = time.Now()

	return result, nil
}

// ValidatePortfolio performs portfolio-wide validation
func (pm *PortfolioManager) ValidatePortfolio(positions []portfolio.PruneCandidate) error {
	// Check total beta exposure
	totalBeta := 0.0
	sectorCounts := make(map[string]int)
	totalExposure := 0.0

	for _, pos := range positions {
		totalBeta += pos.Beta
		sectorCounts[pos.Sector]++
		// Simplified position sizing for validation
		totalExposure += 1.0
	}

	// Validate beta budget
	if totalBeta > pm.pruner.BetaBudgetToBTC {
		return fmt.Errorf("beta budget exceeded: %.2f > %.2f", totalBeta, pm.pruner.BetaBudgetToBTC)
	}

	// Validate sector caps
	for sector, count := range sectorCounts {
		if sectorCap, exists := pm.pruner.SectorCaps[sector]; exists {
			if count > sectorCap {
				return fmt.Errorf("sector %s exceeds cap: %d > %d", sector, count, sectorCap)
			}
		}
	}

	// Validate total exposure
	if totalExposure > pm.pruner.MaxTotalExposurePct {
		return fmt.Errorf("total exposure exceeds limit: %.1f%% > %.1f%%", totalExposure, pm.pruner.MaxTotalExposurePct)
	}

	return nil
}

// GetPortfolioStatus returns current portfolio status and metrics
func (pm *PortfolioManager) GetPortfolioStatus() map[string]interface{} {
	status := map[string]interface{}{
		"constraints":     pm.pruner.GetConstraintSummary(),
		"last_prune_time": pm.lastPruneTime.Format(time.RFC3339),
	}

	if pm.lastPruneResult != nil {
		status["last_prune_result"] = pm.lastPruneResult.Summary
	}

	return status
}

// SetCorrelationProvider allows injecting a custom correlation provider
func (pm *PortfolioManager) SetCorrelationProvider(provider portfolio.CorrelationProvider) {
	pm.correlationProvider = provider
}
