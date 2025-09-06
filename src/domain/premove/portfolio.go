package premove

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// PortfolioManager handles correlation analysis and position sizing constraints
type PortfolioManager struct {
	maxCorrelation float64
	maxPerSector   int
	betaBudget     float64
	sectors        []string
}

// NewPortfolioManager creates a portfolio manager with specified constraints
func NewPortfolioManager(maxCorrelation float64, maxPerSector int, betaBudget float64, sectors []string) *PortfolioManager {
	return &PortfolioManager{
		maxCorrelation: maxCorrelation,
		maxPerSector:   maxPerSector,
		betaBudget:     betaBudget,
		sectors:        sectors,
	}
}

// CorrelationMatrix represents correlation data between assets
type CorrelationMatrix struct {
	Symbols      []string                      `json:"symbols"`
	Matrix       map[string]map[string]float64 `json:"matrix"`
	Timeframe    string                        `json:"timeframe"` // "1h" or "4h"
	Observations int                           `json:"observations"`
	UpdatedAt    time.Time                     `json:"updated_at"`
}

// Position represents a portfolio position for pruning analysis
type Position struct {
	Symbol      string    `json:"symbol"`
	Score       float64   `json:"score"`  // Pre-movement score
	Sector      string    `json:"sector"` // Asset sector classification
	Beta        float64   `json:"beta"`   // Market beta
	Size        float64   `json:"size"`   // Position size
	EntryTime   time.Time `json:"entry_time"`
	Correlation float64   `json:"correlation"` // Max correlation with existing positions
}

// PortfolioPruningResult contains the result of portfolio pruning
type PortfolioPruningResult struct {
	Candidates       []Position          `json:"candidates"`        // Original candidates
	Accepted         []Position          `json:"accepted"`          // Accepted positions
	Rejected         []Position          `json:"rejected"`          // Rejected positions
	RejectionReasons map[string][]string `json:"rejection_reasons"` // Reasons for rejection by symbol

	// Metrics
	TotalBetaUsed   float64        `json:"total_beta_used"`
	BetaUtilization float64        `json:"beta_utilization"` // % of beta budget used
	SectorCounts    map[string]int `json:"sector_counts"`
	MaxCorrelation  float64        `json:"max_correlation"` // Highest correlation in accepted set
	PrunedCount     int            `json:"pruned_count"`
}

// CalculateCorrelationMatrix computes correlation matrix for given symbols and timeframe
func (pm *PortfolioManager) CalculateCorrelationMatrix(priceData map[string][]float64, timeframe string, minObservations int) (*CorrelationMatrix, error) {
	if len(priceData) == 0 {
		return nil, fmt.Errorf("no price data provided")
	}

	symbols := make([]string, 0, len(priceData))
	for symbol := range priceData {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols) // Ensure deterministic order

	// Verify minimum observations
	observations := 0
	for symbol, prices := range priceData {
		if len(prices) > 0 {
			if observations == 0 || len(prices) < observations {
				observations = len(prices)
			}
		}
	}

	if observations < minObservations {
		return nil, fmt.Errorf("insufficient observations: got %d, need %d", observations, minObservations)
	}

	// Calculate correlation matrix
	matrix := make(map[string]map[string]float64)
	for _, symbol1 := range symbols {
		matrix[symbol1] = make(map[string]float64)

		for _, symbol2 := range symbols {
			if symbol1 == symbol2 {
				matrix[symbol1][symbol2] = 1.0
			} else {
				corr := calculateCorrelation(priceData[symbol1], priceData[symbol2])
				matrix[symbol1][symbol2] = corr
			}
		}
	}

	return &CorrelationMatrix{
		Symbols:      symbols,
		Matrix:       matrix,
		Timeframe:    timeframe,
		Observations: observations,
		UpdatedAt:    time.Now(),
	}, nil
}

// calculateCorrelation computes Pearson correlation between two price series
func calculateCorrelation(x, y []float64) float64 {
	n := len(x)
	if n != len(y) || n == 0 {
		return 0.0
	}

	// Calculate means
	meanX, meanY := 0.0, 0.0
	for i := 0; i < n; i++ {
		meanX += x[i]
		meanY += y[i]
	}
	meanX /= float64(n)
	meanY /= float64(n)

	// Calculate correlation components
	numerator := 0.0
	sumSqX := 0.0
	sumSqY := 0.0

	for i := 0; i < n; i++ {
		dx := x[i] - meanX
		dy := y[i] - meanY
		numerator += dx * dy
		sumSqX += dx * dx
		sumSqY += dy * dy
	}

	denominator := math.Sqrt(sumSqX * sumSqY)
	if denominator == 0 {
		return 0.0
	}

	return numerator / denominator
}

// PrunePortfolio applies correlation, sector, and beta constraints to candidate positions
func (pm *PortfolioManager) PrunePortfolio(candidates []Position, existing []Position, correlationMatrix *CorrelationMatrix) (*PortfolioPruningResult, error) {
	result := &PortfolioPruningResult{
		Candidates:       candidates,
		Accepted:         make([]Position, 0),
		Rejected:         make([]Position, 0),
		RejectionReasons: make(map[string][]string),
		SectorCounts:     make(map[string]int),
	}

	// Count existing positions by sector and calculate used beta
	existingSectorCounts := make(map[string]int)
	existingBetaUsed := 0.0

	for _, pos := range existing {
		existingSectorCounts[pos.Sector]++
		existingBetaUsed += math.Abs(pos.Beta * pos.Size)
	}

	// Copy sector counts for tracking
	for sector, count := range existingSectorCounts {
		result.SectorCounts[sector] = count
	}

	// Sort candidates by score (highest first)
	sortedCandidates := make([]Position, len(candidates))
	copy(sortedCandidates, candidates)
	sort.Slice(sortedCandidates, func(i, j int) bool {
		return sortedCandidates[i].Score > sortedCandidates[j].Score
	})

	// Process each candidate
	for _, candidate := range sortedCandidates {
		reasons := make([]string, 0)
		canAccept := true

		// Check sector constraints
		currentSectorCount := result.SectorCounts[candidate.Sector]
		if currentSectorCount >= pm.maxPerSector {
			reasons = append(reasons, fmt.Sprintf("sector limit exceeded: %s already has %d positions (max %d)",
				candidate.Sector, currentSectorCount, pm.maxPerSector))
			canAccept = false
		}

		// Check beta budget
		candidateBetaUsage := math.Abs(candidate.Beta * candidate.Size)
		if result.TotalBetaUsed+candidateBetaUsage > pm.betaBudget {
			reasons = append(reasons, fmt.Sprintf("beta budget exceeded: would use %.2f + %.2f = %.2f > %.2f limit",
				result.TotalBetaUsed, candidateBetaUsage, result.TotalBetaUsed+candidateBetaUsage, pm.betaBudget))
			canAccept = false
		}

		// Check correlation constraints with existing + accepted positions
		maxCorr := pm.calculateMaxCorrelation(candidate.Symbol, existing, result.Accepted, correlationMatrix)
		if maxCorr > pm.maxCorrelation {
			reasons = append(reasons, fmt.Sprintf("correlation too high: max correlation %.3f > %.3f limit",
				maxCorr, pm.maxCorrelation))
			canAccept = false
		}

		// Update candidate with correlation info
		candidate.Correlation = maxCorr

		if canAccept {
			result.Accepted = append(result.Accepted, candidate)
			result.SectorCounts[candidate.Sector]++
			result.TotalBetaUsed += candidateBetaUsage

			// Update max correlation in accepted set
			if maxCorr > result.MaxCorrelation {
				result.MaxCorrelation = maxCorr
			}
		} else {
			result.Rejected = append(result.Rejected, candidate)
			result.RejectionReasons[candidate.Symbol] = reasons
		}
	}

	// Calculate final metrics
	result.BetaUtilization = (result.TotalBetaUsed / pm.betaBudget) * 100.0
	result.PrunedCount = len(result.Rejected)

	return result, nil
}

// calculateMaxCorrelation finds the maximum correlation of a candidate with existing and accepted positions
func (pm *PortfolioManager) calculateMaxCorrelation(candidateSymbol string, existing, accepted []Position, correlationMatrix *CorrelationMatrix) float64 {
	maxCorr := 0.0

	if correlationMatrix == nil {
		return 0.0 // Cannot calculate without correlation matrix
	}

	// Check correlation with existing positions
	for _, pos := range existing {
		if corr, exists := correlationMatrix.Matrix[candidateSymbol][pos.Symbol]; exists {
			absCorr := math.Abs(corr)
			if absCorr > maxCorr {
				maxCorr = absCorr
			}
		}
	}

	// Check correlation with already accepted positions
	for _, pos := range accepted {
		if corr, exists := correlationMatrix.Matrix[candidateSymbol][pos.Symbol]; exists {
			absCorr := math.Abs(corr)
			if absCorr > maxCorr {
				maxCorr = absCorr
			}
		}
	}

	return maxCorr
}

// ValidateCorrelationMatrix performs basic validation on correlation matrix
func ValidateCorrelationMatrix(matrix *CorrelationMatrix) error {
	if matrix == nil {
		return fmt.Errorf("correlation matrix is nil")
	}

	if len(matrix.Symbols) == 0 {
		return fmt.Errorf("no symbols in correlation matrix")
	}

	if matrix.Observations < 10 {
		return fmt.Errorf("too few observations: %d (minimum 10)", matrix.Observations)
	}

	// Check matrix symmetry and diagonal values
	for _, symbol1 := range matrix.Symbols {
		row, exists := matrix.Matrix[symbol1]
		if !exists {
			return fmt.Errorf("missing row for symbol %s", symbol1)
		}

		for _, symbol2 := range matrix.Symbols {
			corr, exists := row[symbol2]
			if !exists {
				return fmt.Errorf("missing correlation for %s-%s", symbol1, symbol2)
			}

			// Check diagonal is 1.0
			if symbol1 == symbol2 && math.Abs(corr-1.0) > 0.001 {
				return fmt.Errorf("diagonal correlation for %s should be 1.0, got %.6f", symbol1, corr)
			}

			// Check correlation bounds
			if math.Abs(corr) > 1.0 {
				return fmt.Errorf("invalid correlation %s-%s: %.6f (must be [-1,1])", symbol1, symbol2, corr)
			}

			// Check symmetry
			if reverseCorr, exists := matrix.Matrix[symbol2][symbol1]; exists {
				if math.Abs(corr-reverseCorr) > 0.001 {
					return fmt.Errorf("correlation matrix not symmetric: %s-%s=%.6f, %s-%s=%.6f",
						symbol1, symbol2, corr, symbol2, symbol1, reverseCorr)
				}
			}
		}
	}

	return nil
}

// GetPortfolioStatus returns current portfolio status and constraints
func (pm *PortfolioManager) GetPortfolioStatus(existing []Position) map[string]interface{} {
	sectorCounts := make(map[string]int)
	betaUsed := 0.0
	maxCorr := 0.0

	for _, pos := range existing {
		sectorCounts[pos.Sector]++
		betaUsed += math.Abs(pos.Beta * pos.Size)
		if pos.Correlation > maxCorr {
			maxCorr = pos.Correlation
		}
	}

	return map[string]interface{}{
		"total_positions": len(existing),
		"sector_counts":   sectorCounts,
		"constraints": map[string]interface{}{
			"max_correlation": pm.maxCorrelation,
			"max_per_sector":  pm.maxPerSector,
			"beta_budget":     pm.betaBudget,
		},
		"utilization": map[string]interface{}{
			"beta_used":        betaUsed,
			"beta_utilization": (betaUsed / pm.betaBudget) * 100.0,
			"max_correlation":  maxCorr,
		},
		"capacity": map[string]interface{}{
			"beta_available":   pm.betaBudget - betaUsed,
			"sectors_at_limit": pm.getSectorsAtLimit(sectorCounts),
		},
	}
}

// getSectorsAtLimit returns sectors that are at their position limit
func (pm *PortfolioManager) getSectorsAtLimit(sectorCounts map[string]int) []string {
	atLimit := make([]string, 0)
	for sector, count := range sectorCounts {
		if count >= pm.maxPerSector {
			atLimit = append(atLimit, sector)
		}
	}
	return atLimit
}
