package premove

import (
	"context"
	"fmt"
	"math"
	"time"
)

// ScoreEngine implements Pre-Movement v3.3 100-point scoring system
type ScoreEngine struct {
	config *ScoreConfig
}

// NewScoreEngine creates a Pre-Movement v3.3 scoring engine
func NewScoreEngine(config *ScoreConfig) *ScoreEngine {
	if config == nil {
		config = DefaultScoreConfig()
	}
	return &ScoreEngine{config: config}
}

// ScoreConfig contains thresholds and weights for Pre-Movement v3.3 scoring
type ScoreConfig struct {
	// Structural Components (0-40 points)
	DerivativesWeight    float64 `yaml:"derivatives_weight"`    // 15 pts: funding, OI, ETF
	SupplyDemandWeight   float64 `yaml:"supply_demand_weight"`  // 15 pts: reserves, whale moves
	MicrostructureWeight float64 `yaml:"microstructure_weight"` // 10 pts: L1/L2 dynamics

	// Behavioral Components (0-35 points)
	SmartMoneyWeight  float64 `yaml:"smart_money_weight"`  // 20 pts: large tx patterns
	CVDResidualWeight float64 `yaml:"cvd_residual_weight"` // 15 pts: cumulative volume delta

	// Catalyst & Compression (0-25 points)
	CatalystWeight    float64 `yaml:"catalyst_weight"`    // 15 pts: news/events
	CompressionWeight float64 `yaml:"compression_weight"` // 10 pts: volatility compression

	// Freshness penalty parameters
	MaxFreshnessHours   float64 `yaml:"max_freshness_hours"`   // 2.0 hours
	FreshnessPenaltyPct float64 `yaml:"freshness_penalty_pct"` // 20% max penalty

	// Score normalization
	MinScore float64 `yaml:"min_score"` // 0.0
	MaxScore float64 `yaml:"max_score"` // 100.0
}

// DefaultScoreConfig returns Pre-Movement v3.3 production configuration
func DefaultScoreConfig() *ScoreConfig {
	return &ScoreConfig{
		// Structural (40 points total)
		DerivativesWeight:    15.0,
		SupplyDemandWeight:   15.0,
		MicrostructureWeight: 10.0,

		// Behavioral (35 points total)
		SmartMoneyWeight:  20.0,
		CVDResidualWeight: 15.0,

		// Catalyst & Compression (25 points total)
		CatalystWeight:    15.0,
		CompressionWeight: 10.0,

		// Freshness parameters
		MaxFreshnessHours:   2.0,  // "worst feed wins" rule
		FreshnessPenaltyPct: 20.0, // max 20% penalty

		// Score bounds
		MinScore: 0.0,
		MaxScore: 100.0,
	}
}

// PreMovementData contains all inputs for Pre-Movement v3.3 scoring
type PreMovementData struct {
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`

	// Structural factors
	FundingZScore   float64 `json:"funding_z_score"`   // Cross-venue funding divergence
	OIResidual      float64 `json:"oi_residual"`       // Open interest anomaly
	ETFFlowTint     float64 `json:"etf_flow_tint"`     // ETF net flow direction
	ReserveChange7d float64 `json:"reserve_change_7d"` // Exchange reserves change
	WhaleComposite  float64 `json:"whale_composite"`   // Large transaction patterns
	MicroDynamics   float64 `json:"micro_dynamics"`    // L1/L2 order book stress

	// Behavioral factors
	SmartMoneyFlow float64 `json:"smart_money_flow"` // Institutional flow patterns
	CVDResidual    float64 `json:"cvd_residual"`     // Volume-price residual

	// Catalyst & compression
	CatalystHeat       float64 `json:"catalyst_heat"`        // News/event significance
	VolCompressionRank float64 `json:"vol_compression_rank"` // Volatility compression percentile

	// Data freshness tracking (worst feed wins)
	OldestFeedHours float64 `json:"oldest_feed_hours"` // Hours since oldest data point
}

// ScoreResult contains the complete Pre-Movement v3.3 score breakdown
type ScoreResult struct {
	Symbol           string                 `json:"symbol"`
	Timestamp        time.Time              `json:"timestamp"`
	TotalScore       float64                `json:"total_score"`      // 0-100 final score
	ComponentScores  map[string]float64     `json:"component_scores"` // Individual component contributions
	Attribution      map[string]interface{} `json:"attribution"`      // Detailed score attribution
	DataFreshness    *FreshnessInfo         `json:"data_freshness"`   // Freshness penalty details
	EvaluationTimeMs int64                  `json:"evaluation_time_ms"`
	IsValid          bool                   `json:"is_valid"` // Whether score is actionable
	Warnings         []string               `json:"warnings"` // Data quality warnings
}

// FreshnessInfo tracks data staleness and penalties applied
type FreshnessInfo struct {
	OldestFeedHours    float64  `json:"oldest_feed_hours"`
	FreshnessPenalty   float64  `json:"freshness_penalty"`   // 0-20% penalty applied
	AffectedComponents []string `json:"affected_components"` // Components penalized
	WorstFeed          string   `json:"worst_feed"`          // Name of stalest feed
}

// CalculateScore computes the complete Pre-Movement v3.3 score
func (se *ScoreEngine) CalculateScore(ctx context.Context, data *PreMovementData) (*ScoreResult, error) {
	startTime := time.Now()

	result := &ScoreResult{
		Symbol:          data.Symbol,
		Timestamp:       time.Now(),
		ComponentScores: make(map[string]float64),
		Attribution:     make(map[string]interface{}),
		Warnings:        []string{},
	}

	// Calculate individual component scores
	structuralScore := se.calculateStructuralScore(data, result)
	behavioralScore := se.calculateBehavioralScore(data, result)
	catalystScore := se.calculateCatalystScore(data, result)

	// Base score before freshness penalty
	baseScore := structuralScore + behavioralScore + catalystScore

	// Apply freshness penalty ("worst feed wins" rule)
	freshnessInfo, finalScore := se.applyFreshnessPenalty(baseScore, data.OldestFeedHours)
	result.DataFreshness = freshnessInfo

	// Normalize and bound the score
	result.TotalScore = se.normalizeScore(finalScore)
	result.IsValid = result.TotalScore >= 0.0 && len(result.Warnings) == 0
	result.EvaluationTimeMs = time.Since(startTime).Milliseconds()

	return result, nil
}

// calculateStructuralScore computes structural factors (40 points max)
func (se *ScoreEngine) calculateStructuralScore(data *PreMovementData, result *ScoreResult) float64 {
	var score float64

	// Derivatives component (15 points max)
	derivScore := se.scoreDerivatives(data.FundingZScore, data.OIResidual, data.ETFFlowTint)
	result.ComponentScores["derivatives"] = derivScore
	result.Attribution["derivatives"] = map[string]interface{}{
		"funding_z_score": data.FundingZScore,
		"oi_residual":     data.OIResidual,
		"etf_flow_tint":   data.ETFFlowTint,
		"contribution":    derivScore,
	}
	score += derivScore

	// Supply/demand component (15 points max)
	supplyScore := se.scoreSupplyDemand(data.ReserveChange7d, data.WhaleComposite)
	result.ComponentScores["supply_demand"] = supplyScore
	result.Attribution["supply_demand"] = map[string]interface{}{
		"reserve_change_7d": data.ReserveChange7d,
		"whale_composite":   data.WhaleComposite,
		"contribution":      supplyScore,
	}
	score += supplyScore

	// Microstructure component (10 points max)
	microScore := se.scoreMicrostructure(data.MicroDynamics)
	result.ComponentScores["microstructure"] = microScore
	result.Attribution["microstructure"] = map[string]interface{}{
		"micro_dynamics": data.MicroDynamics,
		"contribution":   microScore,
	}
	score += microScore

	return score
}

// calculateBehavioralScore computes behavioral factors (35 points max)
func (se *ScoreEngine) calculateBehavioralScore(data *PreMovementData, result *ScoreResult) float64 {
	var score float64

	// Smart money component (20 points max)
	smartScore := se.scoreSmartMoney(data.SmartMoneyFlow)
	result.ComponentScores["smart_money"] = smartScore
	result.Attribution["smart_money"] = map[string]interface{}{
		"smart_money_flow": data.SmartMoneyFlow,
		"contribution":     smartScore,
	}
	score += smartScore

	// CVD residual component (15 points max)
	cvdScore := se.scoreCVDResidual(data.CVDResidual)
	result.ComponentScores["cvd_residual"] = cvdScore
	result.Attribution["cvd_residual"] = map[string]interface{}{
		"cvd_residual": data.CVDResidual,
		"contribution": cvdScore,
	}
	score += cvdScore

	return score
}

// calculateCatalystScore computes catalyst and compression factors (25 points max)
func (se *ScoreEngine) calculateCatalystScore(data *PreMovementData, result *ScoreResult) float64 {
	var score float64

	// Catalyst component (15 points max)
	catalystScore := se.scoreCatalyst(data.CatalystHeat)
	result.ComponentScores["catalyst"] = catalystScore
	result.Attribution["catalyst"] = map[string]interface{}{
		"catalyst_heat": data.CatalystHeat,
		"contribution":  catalystScore,
	}
	score += catalystScore

	// Compression component (10 points max)
	compressionScore := se.scoreCompression(data.VolCompressionRank)
	result.ComponentScores["compression"] = compressionScore
	result.Attribution["compression"] = map[string]interface{}{
		"vol_compression_rank": data.VolCompressionRank,
		"contribution":         compressionScore,
	}
	score += compressionScore

	return score
}

// Individual scoring functions implementing Pre-Movement v3.3 logic

func (se *ScoreEngine) scoreDerivatives(fundingZ, oiResidual, etfTint float64) float64 {
	// Funding z-score contribution (0-7 points)
	fundingScore := math.Min(7.0, math.Max(0.0, fundingZ*1.5)) // Scale z-score to 0-7 range

	// OI residual contribution (0-4 points)
	oiScore := math.Min(4.0, math.Max(0.0, oiResidual/250000.0)) // Scale $1M OI residual = 4 points

	// ETF flow tint contribution (0-4 points)
	etfScore := math.Min(4.0, math.Max(0.0, etfTint*4.0)) // Scale 0-1 tint to 0-4 points

	return fundingScore + oiScore + etfScore
}

func (se *ScoreEngine) scoreSupplyDemand(reserveChange, whaleComposite float64) float64 {
	// Reserve depletion contribution (0-8 points)
	reserveScore := math.Min(8.0, math.Max(0.0, -reserveChange*0.4)) // -20% reserves = 8 points

	// Whale composite contribution (0-7 points)
	whaleScore := math.Min(7.0, math.Max(0.0, whaleComposite*7.0)) // Scale 0-1 composite to 0-7 points

	return reserveScore + whaleScore
}

func (se *ScoreEngine) scoreMicrostructure(microDynamics float64) float64 {
	// L1/L2 stress contribution (0-10 points)
	return math.Min(10.0, math.Max(0.0, microDynamics*10.0)) // Scale 0-1 dynamics to 0-10 points
}

func (se *ScoreEngine) scoreSmartMoney(smartFlow float64) float64 {
	// Institutional flow patterns (0-20 points)
	return math.Min(20.0, math.Max(0.0, smartFlow*20.0)) // Scale 0-1 flow to 0-20 points
}

func (se *ScoreEngine) scoreCVDResidual(cvdResidual float64) float64 {
	// CVD residual strength (0-15 points)
	return math.Min(15.0, math.Max(0.0, math.Abs(cvdResidual)*15.0)) // Scale 0-1 residual to 0-15 points
}

func (se *ScoreEngine) scoreCatalyst(catalystHeat float64) float64 {
	// News/event significance (0-15 points)
	return math.Min(15.0, math.Max(0.0, catalystHeat*15.0)) // Scale 0-1 heat to 0-15 points
}

func (se *ScoreEngine) scoreCompression(compressionRank float64) float64 {
	// Volatility compression percentile (0-10 points)
	return math.Min(10.0, math.Max(0.0, compressionRank*10.0)) // Scale 0-1 percentile to 0-10 points
}

// applyFreshnessPenalty implements "worst feed wins" freshness penalty
func (se *ScoreEngine) applyFreshnessPenalty(baseScore, oldestFeedHours float64) (*FreshnessInfo, float64) {
	freshnessInfo := &FreshnessInfo{
		OldestFeedHours:    oldestFeedHours,
		FreshnessPenalty:   0.0,
		AffectedComponents: []string{},
		WorstFeed:          "unknown",
	}

	// No penalty if data is fresh (< max freshness hours)
	if oldestFeedHours <= se.config.MaxFreshnessHours {
		return freshnessInfo, baseScore
	}

	// Calculate penalty: linear scale from 0% at max_hours to 20% at 2*max_hours
	excessHours := oldestFeedHours - se.config.MaxFreshnessHours
	maxExcessHours := se.config.MaxFreshnessHours // 100% penalty at 2x max_hours

	penaltyRatio := math.Min(1.0, excessHours/maxExcessHours)
	freshnessInfo.FreshnessPenalty = penaltyRatio * se.config.FreshnessPenaltyPct / 100.0

	// Apply penalty to final score
	penalty := baseScore * freshnessInfo.FreshnessPenalty
	finalScore := baseScore - penalty

	// Track affected components (all components affected by freshness)
	freshnessInfo.AffectedComponents = []string{"all"}

	return freshnessInfo, finalScore
}

// normalizeScore ensures score stays within configured bounds
func (se *ScoreEngine) normalizeScore(score float64) float64 {
	return math.Min(se.config.MaxScore, math.Max(se.config.MinScore, score))
}

// GetScoreSummary returns a concise summary of the Pre-Movement score
func (sr *ScoreResult) GetScoreSummary() string {
	freshnessNote := ""
	if sr.DataFreshness.FreshnessPenalty > 0.0 {
		freshnessNote = fmt.Sprintf(" (-%0.1f%% stale)", sr.DataFreshness.FreshnessPenalty*100)
	}

	validity := "✅ VALID"
	if !sr.IsValid {
		validity = "⚠️  CHECK"
	}

	return fmt.Sprintf("%s — %s score: %.1f%s (%dms)",
		validity, sr.Symbol, sr.TotalScore, freshnessNote, sr.EvaluationTimeMs)
}

// GetDetailedBreakdown returns comprehensive score attribution
func (sr *ScoreResult) GetDetailedBreakdown() string {
	report := fmt.Sprintf("Pre-Movement v3.3 Score: %s (%.1f/100)\n", sr.Symbol, sr.TotalScore)
	report += fmt.Sprintf("Valid: %t | Evaluation: %dms\n\n", sr.IsValid, sr.EvaluationTimeMs)

	// Component breakdown
	report += "Component Scores:\n"
	componentOrder := []string{"derivatives", "supply_demand", "microstructure", "smart_money", "cvd_residual", "catalyst", "compression"}

	for _, component := range componentOrder {
		if score, exists := sr.ComponentScores[component]; exists {
			report += fmt.Sprintf("  %s: %.1f pts\n", component, score)
		}
	}

	// Freshness penalty
	if sr.DataFreshness.FreshnessPenalty > 0.0 {
		report += fmt.Sprintf("\nFreshness Penalty: -%.1f%% (%.1fh old data)\n",
			sr.DataFreshness.FreshnessPenalty*100, sr.DataFreshness.OldestFeedHours)
	}

	// Warnings
	if len(sr.Warnings) > 0 {
		report += fmt.Sprintf("\nWarnings:\n")
		for i, warning := range sr.Warnings {
			report += fmt.Sprintf("  %d. %s\n", i+1, warning)
		}
	}

	return report
}
