//go:build !prod && !integration

package premove

import (
	"time"
)

// Test-only shims for missing premove types and methods
// This file is excluded from production and integration builds

// Position represents a trading position in the premove namespace
type Position struct {
	Symbol        string    `json:"symbol"`
	Size          float64   `json:"size"`
	EntryPrice    float64   `json:"entry_price"`
	CurrentPrice  float64   `json:"current_price"`
	Score         float64   `json:"score"`
	Sector        string    `json:"sector"`
	Beta          float64   `json:"beta"`
	Timestamp     time.Time `json:"timestamp"`
	CorrelationID string    `json:"correlation_id"`
}

// CorrelationMatrix represents position correlations in the premove namespace
type CorrelationMatrix struct {
	Matrix    map[string]map[string]float64 `json:"matrix"`
	Symbols   []string                      `json:"symbols"`
	Timestamp time.Time                     `json:"timestamp"`
}

// NewCorrelationMatrix creates a new correlation matrix (test-only stub)
func NewCorrelationMatrix(symbols []string) *CorrelationMatrix {
	matrix := make(map[string]map[string]float64)
	for _, s1 := range symbols {
		matrix[s1] = make(map[string]float64)
		for _, s2 := range symbols {
			if s1 == s2 {
				matrix[s1][s2] = 1.0
			} else {
				matrix[s1][s2] = 0.1 // Low correlation stub
			}
		}
	}
	
	return &CorrelationMatrix{
		Matrix:    matrix,
		Symbols:   symbols,
		Timestamp: time.Now(),
	}
}

// GetCorrelation returns correlation between two symbols (test-only stub)
func (cm *CorrelationMatrix) GetCorrelation(symbol1, symbol2 string) float64 {
	if symbol1 == symbol2 {
		return 1.0
	}
	
	if cm.Matrix != nil {
		if s1Map, exists := cm.Matrix[symbol1]; exists {
			if corr, exists := s1Map[symbol2]; exists {
				return corr
			}
		}
	}
	
	return 0.1 // Default low correlation
}

// ExecutionMetrics contains execution performance metrics (referenced by UI)
type ExecutionMetrics struct {
	AvgSlippageBps float64       `json:"avg_slippage_bps"`
	AvgFillTime    time.Duration `json:"avg_fill_time"`
	FillRate       float64       `json:"fill_rate"`
}

// GetExecutionSummary returns execution summary (referenced by UI)
func (r *Runner) GetExecutionSummary() map[string]interface{} {
	return map[string]interface{}{
		"total_executions": 0,
		"avg_slippage":     0.0,
		"fill_rate":        0.0,
	}
}

// GetPortfolioConstraints returns portfolio constraints (referenced by UI)
func (r *Runner) GetPortfolioConstraints(positions []Position) map[string]interface{} {
	return map[string]interface{}{
		"total_positions":     len(positions),
		"utilization":         0.0,
		"max_position_size":   100000.0,
		"correlation_limit":   0.7,
		"sector_concentration": 0.3,
	}
}

// PortfolioPruningResult represents legacy portfolio pruning results
type PortfolioPruningResult struct {
	Candidates       []Position           `json:"candidates"`
	Accepted         []Position           `json:"accepted"`
	Rejected         []Position           `json:"rejected"`
	RejectionReasons map[string][]string  `json:"rejection_reasons"`
	PrunedCount      int                  `json:"pruned_count"`
	BetaUtilization  float64              `json:"beta_utilization"`
}

// PrunePortfolio is a legacy method for backward compatibility
func (pm *PortfolioManager) PrunePortfolio(positions []Position, existing []Position, correlations *CorrelationMatrix) (*PortfolioPruningResult, error) {
	// Simple stub for test compatibility
	legacyResult := &PortfolioPruningResult{
		Candidates:       positions,
		Accepted:         positions, // Accept all for simplicity in tests
		Rejected:         make([]Position, 0),
		RejectionReasons: make(map[string][]string),
		PrunedCount:      0,
		BetaUtilization:  0.5,
	}
	
	return legacyResult, nil
}