package portfolio

import (
	"fmt"
	"math"
	"sort"
)

// Pruner enforces portfolio-level constraints: pairwise correlation ≤0.65, sector caps ≤2, beta ≤2.0, single ≤5%, total ≤20%
type Pruner struct {
	PairwiseCorrMax      float64        `yaml:"pairwise_corr_max"`       // Default: 0.65
	SectorCaps           map[string]int `yaml:"sector_caps"`             // Per-sector position limits
	BetaBudgetToBTC      float64        `yaml:"beta_budget_to_btc"`      // Default: 2.0
	MaxSinglePositionPct float64        `yaml:"max_single_position_pct"` // Default: 5.0%
	MaxTotalExposurePct  float64        `yaml:"max_total_exposure_pct"`  // Default: 20.0%
}

// PruneCandidate represents a candidate for portfolio inclusion
type PruneCandidate struct {
	Symbol      string  `json:"symbol"`
	Score       float64 `json:"score"`
	PassedGates int     `json:"passed_gates"`
	Sector      string  `json:"sector"`
	Beta        float64 `json:"beta"`        // Beta to BTC
	ADV         float64 `json:"adv"`         // Average Daily Volume USD
	Correlation float64 `json:"correlation"` // Max correlation with existing positions
}

// PruneResult contains portfolio pruning results
type PruneResult struct {
	Accepted         []PruneCandidate  `json:"accepted"`
	Rejected         []PruneCandidate  `json:"rejected"`
	RejectionReasons map[string]string `json:"rejection_reasons"`
	Summary          PruneSummary      `json:"summary"`
}

// PruneSummary provides pruning operation statistics
type PruneSummary struct {
	TotalCandidates     int     `json:"total_candidates"`
	AcceptedCount       int     `json:"accepted_count"`
	RejectedCount       int     `json:"rejected_count"`
	BetaUtilization     float64 `json:"beta_utilization_pct"`     // % of beta budget used
	ExposureUtilization float64 `json:"exposure_utilization_pct"` // % of total exposure used
	TopRejectionReason  string  `json:"top_rejection_reason"`
}

// CorrelationProvider provides correlation data between assets
type CorrelationProvider interface {
	GetCorrelation(symbol1, symbol2 string) (float64, bool)
}

// NewPruner creates a portfolio pruner with default constraints
func NewPruner() *Pruner {
	return &Pruner{
		PairwiseCorrMax:      0.65,
		SectorCaps:           map[string]int{"DeFi": 2, "Layer1": 2, "Layer2": 2, "Meme": 1, "AI": 2, "Gaming": 1, "Infrastructure": 2},
		BetaBudgetToBTC:      2.0,
		MaxSinglePositionPct: 5.0,
		MaxTotalExposurePct:  20.0,
	}
}

// NewPrunerWithConstraints creates a portfolio pruner with custom constraints
func NewPrunerWithConstraints(pairwiseCorr float64, sectorCaps map[string]int, betaBudget float64, maxSingle float64, maxTotal float64) *Pruner {
	return &Pruner{
		PairwiseCorrMax:      pairwiseCorr,
		SectorCaps:           sectorCaps,
		BetaBudgetToBTC:      betaBudget,
		MaxSinglePositionPct: maxSingle,
		MaxTotalExposurePct:  maxTotal,
	}
}

// Prune applies portfolio constraints to filter candidate positions
func (p *Pruner) Prune(candidates []PruneCandidate, correlationProvider CorrelationProvider) *PruneResult {
	result := &PruneResult{
		Accepted:         make([]PruneCandidate, 0),
		Rejected:         make([]PruneCandidate, 0),
		RejectionReasons: make(map[string]string),
		Summary: PruneSummary{
			TotalCandidates: len(candidates),
		},
	}

	if len(candidates) == 0 {
		return result
	}

	// Sort candidates by score (highest first) for greedy selection
	sortedCandidates := make([]PruneCandidate, len(candidates))
	copy(sortedCandidates, candidates)
	sort.Slice(sortedCandidates, func(i, j int) bool {
		return sortedCandidates[i].Score > sortedCandidates[j].Score
	})

	// Track running constraints
	sectorCounts := make(map[string]int)
	betaUsed := 0.0
	totalExposure := 0.0
	rejectionReasons := make(map[string]int)

	for _, candidate := range sortedCandidates {
		rejectionReason := p.evaluateCandidate(candidate, result.Accepted, sectorCounts, betaUsed, totalExposure, correlationProvider)

		if rejectionReason == "" {
			// Accept candidate
			result.Accepted = append(result.Accepted, candidate)
			sectorCounts[candidate.Sector]++
			betaUsed += math.Abs(candidate.Beta)

			// Calculate position size based on score/volatility (simplified to 1% for now)
			positionSize := p.calculatePositionSize(candidate)
			totalExposure += positionSize
		} else {
			// Reject candidate
			result.Rejected = append(result.Rejected, candidate)
			result.RejectionReasons[candidate.Symbol] = rejectionReason
			rejectionReasons[rejectionReason]++
		}
	}

	// Calculate summary metrics
	result.Summary.AcceptedCount = len(result.Accepted)
	result.Summary.RejectedCount = len(result.Rejected)
	result.Summary.BetaUtilization = (betaUsed / p.BetaBudgetToBTC) * 100.0
	result.Summary.ExposureUtilization = (totalExposure / p.MaxTotalExposurePct) * 100.0
	result.Summary.TopRejectionReason = p.findTopRejectionReason(rejectionReasons)

	return result
}

// evaluateCandidate checks if candidate violates any portfolio constraints
func (p *Pruner) evaluateCandidate(candidate PruneCandidate, accepted []PruneCandidate, sectorCounts map[string]int, betaUsed float64, totalExposure float64, correlationProvider CorrelationProvider) string {
	// Check pairwise correlation constraint
	if p.violatesCorrelationConstraint(candidate, accepted, correlationProvider) {
		return fmt.Sprintf("correlation > %.2f", p.PairwiseCorrMax)
	}

	// Check sector cap constraint
	sectorCap, exists := p.SectorCaps[candidate.Sector]
	if !exists {
		sectorCap = 1 // Default sector cap for unknown sectors
	}
	if sectorCounts[candidate.Sector] >= sectorCap {
		return fmt.Sprintf("sector %s at cap (%d)", candidate.Sector, sectorCap)
	}

	// Check beta budget constraint
	candidateBeta := math.Abs(candidate.Beta)
	if betaUsed+candidateBeta > p.BetaBudgetToBTC {
		return fmt.Sprintf("beta budget exceeded (%.2f+%.2f > %.2f)", betaUsed, candidateBeta, p.BetaBudgetToBTC)
	}

	// Check single position size constraint
	positionSize := p.calculatePositionSize(candidate)
	if positionSize > p.MaxSinglePositionPct {
		return fmt.Sprintf("position %.1f%% > %.1f%% limit", positionSize, p.MaxSinglePositionPct)
	}

	// Check total exposure constraint
	if totalExposure+positionSize > p.MaxTotalExposurePct {
		return fmt.Sprintf("total exposure %.1f%%+%.1f%% > %.1f%% limit", totalExposure, positionSize, p.MaxTotalExposurePct)
	}

	return "" // No violation
}

// violatesCorrelationConstraint checks if candidate has excessive correlation with accepted positions
func (p *Pruner) violatesCorrelationConstraint(candidate PruneCandidate, accepted []PruneCandidate, correlationProvider CorrelationProvider) bool {
	if correlationProvider == nil {
		return false // Cannot check without correlation data
	}

	for _, acceptedCandidate := range accepted {
		if corr, exists := correlationProvider.GetCorrelation(candidate.Symbol, acceptedCandidate.Symbol); exists {
			if math.Abs(corr) > p.PairwiseCorrMax {
				return true
			}
		}
	}
	return false
}

// calculatePositionSize determines position size based on candidate characteristics
func (p *Pruner) calculatePositionSize(candidate PruneCandidate) float64 {
	// Simplified position sizing: inversely related to beta, scaled by score
	// In practice, this would consider volatility, liquidity, and risk metrics
	baseSizing := 1.0 // 1% base position

	// Scale by score (higher scores get slightly larger positions)
	scoreMultiplier := math.Min(candidate.Score/75.0, 1.5) // Cap at 1.5x for scores ≥75

	// Scale inversely by beta (higher beta assets get smaller positions)
	betaAdjustment := 1.0 / (1.0 + math.Abs(candidate.Beta)*0.1)

	return baseSizing * scoreMultiplier * betaAdjustment
}

// findTopRejectionReason returns the most common rejection reason
func (p *Pruner) findTopRejectionReason(rejectionReasons map[string]int) string {
	if len(rejectionReasons) == 0 {
		return "none"
	}

	topReason := ""
	maxCount := 0

	for reason, count := range rejectionReasons {
		if count > maxCount {
			maxCount = count
			topReason = reason
		}
	}

	return topReason
}

// GetConstraintSummary returns current constraint configuration
func (p *Pruner) GetConstraintSummary() map[string]interface{} {
	return map[string]interface{}{
		"pairwise_corr_max":       p.PairwiseCorrMax,
		"sector_caps":             p.SectorCaps,
		"beta_budget_to_btc":      p.BetaBudgetToBTC,
		"max_single_position_pct": p.MaxSinglePositionPct,
		"max_total_exposure_pct":  p.MaxTotalExposurePct,
	}
}
