package premove

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/sawpanic/cryptorun/internal/microstructure"
)

// PreMovementEngine orchestrates the complete Pre-Movement v3.3 system
type PreMovementEngine struct {
	scoreEngine    *ScoreEngine
	gateEvaluator  *GateEvaluator
	cvdAnalyzer    *CVDResidualAnalyzer
	microEvaluator microstructure.Evaluator
	config         *EngineConfig
}

// NewPreMovementEngine creates a complete Pre-Movement v3.3 engine
func NewPreMovementEngine(
	microEvaluator microstructure.Evaluator,
	config *EngineConfig,
) *PreMovementEngine {
	if config == nil {
		config = DefaultEngineConfig()
	}

	return &PreMovementEngine{
		scoreEngine:    NewScoreEngine(config.ScoreConfig),
		gateEvaluator:  NewGateEvaluator(microEvaluator, config.GateConfig),
		cvdAnalyzer:    NewCVDResidualAnalyzer(config.CVDConfig),
		microEvaluator: microEvaluator,
		config:         config,
	}
}

// EngineConfig contains configuration for the complete Pre-Movement system
type EngineConfig struct {
	ScoreConfig *ScoreConfig `yaml:"score_config"`
	GateConfig  *GateConfig  `yaml:"gate_config"`
	CVDConfig   *CVDConfig   `yaml:"cvd_config"`

	// API limits
	MaxCandidates    int   `yaml:"max_candidates"`      // 50 max candidates returned
	MaxProcessTimeMs int64 `yaml:"max_process_time_ms"` // 2000ms max processing time
	RequireScore     bool  `yaml:"require_score"`       // true - require valid score
	RequireGates     bool  `yaml:"require_gates"`       // true - require gate pass

	// Data freshness requirements
	MaxDataStaleness int64 `yaml:"max_data_staleness"` // 1800 seconds (30 min)
	StaleDataWarning int64 `yaml:"stale_data_warning"` // 600 seconds (10 min)
}

// DefaultEngineConfig returns production Pre-Movement engine configuration
func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		ScoreConfig: DefaultScoreConfig(),
		GateConfig:  DefaultGateConfig(),
		CVDConfig:   DefaultCVDConfig(),

		// API limits
		MaxCandidates:    50,
		MaxProcessTimeMs: 2000, // 2 seconds max
		RequireScore:     true,
		RequireGates:     true,

		// Data freshness
		MaxDataStaleness: 1800, // 30 minutes
		StaleDataWarning: 600,  // 10 minutes
	}
}

// CandidateInput contains all data required for Pre-Movement analysis
type CandidateInput struct {
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`

	// Pre-Movement scoring data
	PreMovementData *PreMovementData `json:"premove_data"`

	// Gate confirmation data
	ConfirmationData *ConfirmationData `json:"confirmation_data"`

	// CVD residual data
	CVDDataPoints []*CVDDataPoint `json:"cvd_data_points"`
}

// PreMovementCandidate represents a complete analyzed candidate
type PreMovementCandidate struct {
	Symbol         string                           `json:"symbol"`
	Timestamp      time.Time                        `json:"timestamp"`
	TotalScore     float64                          `json:"total_score"`     // 0-100 Pre-Movement score
	ScoreBreakdown *ScoreResult                     `json:"score_breakdown"` // Detailed scoring
	GatesStatus    string                           `json:"gates_status"`    // "CONFIRMED", "BLOCKED", "WARNING"
	GatesResult    *ConfirmationResult              `json:"gates_result"`    // Gate evaluation details
	CVDResult      *CVDResidualResult               `json:"cvd_result"`      // CVD residual analysis
	MicroReport    *microstructure.EvaluationResult `json:"micro_report"`    // Microstructure consultation
	OverallStatus  string                           `json:"overall_status"`  // "STRONG", "MODERATE", "WEAK", "BLOCKED"
	Reasons        []string                         `json:"reasons"`         // Key reasons for recommendation
	Warnings       []string                         `json:"warnings"`        // Data quality or analysis warnings
	ProcessTimeMs  int64                            `json:"process_time_ms"` // Individual processing time
	Rank           int                              `json:"rank"`            // Rank among candidates (1=best)
}

// AnalysisResult contains complete Pre-Movement analysis results
type AnalysisResult struct {
	Timestamp        time.Time               `json:"timestamp"`
	TotalCandidates  int                     `json:"total_candidates"`  // Total candidates analyzed
	ValidCandidates  int                     `json:"valid_candidates"`  // Candidates passing filters
	StrongCandidates int                     `json:"strong_candidates"` // High-confidence candidates
	Candidates       []*PreMovementCandidate `json:"candidates"`        // Ranked candidate list
	ProcessTimeMs    int64                   `json:"process_time_ms"`   // Total processing time
	DataFreshness    *DataFreshnessReport    `json:"data_freshness"`    // Data quality summary
	SystemWarnings   []string                `json:"system_warnings"`   // System-level warnings
}

// DataFreshnessReport summarizes data quality across all candidates
type DataFreshnessReport struct {
	AverageAgeSeconds    int64   `json:"average_age_seconds"`    // Average data age
	StaleCandidatesCount int     `json:"stale_candidates_count"` // Candidates with stale data
	StaleCandidatesPct   float64 `json:"stale_candidates_pct"`   // % stale candidates
	OldestDataSeconds    int64   `json:"oldest_data_seconds"`    // Oldest data point
	FreshnessGrade       string  `json:"freshness_grade"`        // "A", "B", "C", "D", "F"
}

// ListCandidates performs complete Pre-Movement analysis and returns ranked candidates
func (pme *PreMovementEngine) ListCandidates(ctx context.Context, inputs []*CandidateInput, limit int) (*AnalysisResult, error) {
	startTime := time.Now()

	if limit <= 0 || limit > pme.config.MaxCandidates {
		limit = pme.config.MaxCandidates
	}

	result := &AnalysisResult{
		Timestamp:       time.Now(),
		TotalCandidates: len(inputs),
		Candidates:      make([]*PreMovementCandidate, 0, len(inputs)),
		SystemWarnings:  []string{},
	}

	// Process each candidate
	for _, input := range inputs {
		candidate, err := pme.analyzeCandidate(ctx, input)
		if err != nil {
			result.SystemWarnings = append(result.SystemWarnings,
				fmt.Sprintf("%s: analysis failed - %v", input.Symbol, err))
			continue
		}

		// Apply filters
		if pme.shouldIncludeCandidate(candidate) {
			result.Candidates = append(result.Candidates, candidate)
		}
	}

	result.ValidCandidates = len(result.Candidates)

	// Rank candidates by overall strength
	pme.rankCandidates(result.Candidates)

	// Limit results
	if limit < len(result.Candidates) {
		result.Candidates = result.Candidates[:limit]
	}

	// Count strong candidates (top tier)
	for _, candidate := range result.Candidates {
		if candidate.OverallStatus == "STRONG" {
			result.StrongCandidates++
		}
	}

	// Assess data freshness
	result.DataFreshness = pme.assessDataFreshness(result.Candidates)

	result.ProcessTimeMs = time.Since(startTime).Milliseconds()

	// Performance warning
	if result.ProcessTimeMs > pme.config.MaxProcessTimeMs {
		result.SystemWarnings = append(result.SystemWarnings,
			fmt.Sprintf("Analysis took %dms (>%dms threshold)", result.ProcessTimeMs, pme.config.MaxProcessTimeMs))
	}

	return result, nil
}

// analyzeCandidate performs complete Pre-Movement analysis for a single candidate
func (pme *PreMovementEngine) analyzeCandidate(ctx context.Context, input *CandidateInput) (*PreMovementCandidate, error) {
	startTime := time.Now()

	candidate := &PreMovementCandidate{
		Symbol:    input.Symbol,
		Timestamp: time.Now(),
		Reasons:   []string{},
		Warnings:  []string{},
	}

	// 1. Score calculation
	if input.PreMovementData != nil {
		scoreResult, err := pme.scoreEngine.CalculateScore(ctx, input.PreMovementData)
		if err != nil {
			return nil, fmt.Errorf("scoring failed: %w", err)
		}
		candidate.ScoreBreakdown = scoreResult
		candidate.TotalScore = scoreResult.TotalScore

		// Add scoring reasons
		if scoreResult.TotalScore >= 75 {
			candidate.Reasons = append(candidate.Reasons, fmt.Sprintf("High Pre-Movement score (%.1f)", scoreResult.TotalScore))
		}
	}

	// 2. Gate evaluation
	if input.ConfirmationData != nil {
		gateResult, err := pme.gateEvaluator.EvaluateConfirmation(ctx, input.ConfirmationData)
		if err != nil {
			return nil, fmt.Errorf("gate evaluation failed: %w", err)
		}
		candidate.GatesResult = gateResult

		if gateResult.Passed {
			candidate.GatesStatus = "CONFIRMED"
			candidate.Reasons = append(candidate.Reasons,
				fmt.Sprintf("%d-of-%d confirmations passed", gateResult.ConfirmationCount, gateResult.RequiredCount))
		} else {
			candidate.GatesStatus = "BLOCKED"
		}

		// Collect gate warnings
		candidate.Warnings = append(candidate.Warnings, gateResult.Warnings...)
	}

	// 3. CVD residual analysis
	if input.CVDDataPoints != nil && len(input.CVDDataPoints) > 0 {
		cvdResult, err := pme.cvdAnalyzer.AnalyzeCVDResidual(ctx, input.Symbol, input.CVDDataPoints)
		if err != nil {
			candidate.Warnings = append(candidate.Warnings, fmt.Sprintf("CVD analysis failed: %v", err))
		} else {
			candidate.CVDResult = cvdResult

			if cvdResult.IsSignificant {
				candidate.Reasons = append(candidate.Reasons,
					fmt.Sprintf("Significant CVD residual (%.1f%%)", cvdResult.PercentileRank))
			}

			// Collect CVD warnings
			candidate.Warnings = append(candidate.Warnings, cvdResult.Warnings...)
		}
	}

	// 4. Microstructure consultation (from gates result)
	if candidate.GatesResult != nil && candidate.GatesResult.MicroReport != nil {
		candidate.MicroReport = candidate.GatesResult.MicroReport
	}

	// 5. Determine overall status
	candidate.OverallStatus = pme.determineOverallStatus(candidate)

	candidate.ProcessTimeMs = time.Since(startTime).Milliseconds()

	return candidate, nil
}

// shouldIncludeCandidate applies filters to determine if candidate should be included
func (pme *PreMovementEngine) shouldIncludeCandidate(candidate *PreMovementCandidate) bool {
	// Require valid score if configured
	if pme.config.RequireScore {
		if candidate.ScoreBreakdown == nil || !candidate.ScoreBreakdown.IsValid {
			return false
		}
	}

	// Require gate confirmation if configured
	if pme.config.RequireGates {
		if candidate.GatesResult == nil || !candidate.GatesResult.Passed {
			return false
		}
	}

	// Check data freshness
	if candidate.ScoreBreakdown != nil && candidate.ScoreBreakdown.DataFreshness != nil {
		ageSeconds := int64(candidate.ScoreBreakdown.DataFreshness.OldestFeedHours * 3600)
		if ageSeconds > pme.config.MaxDataStaleness {
			return false // Reject stale data
		}
	}

	return true
}

// determineOverallStatus assigns overall status based on scoring and gates
func (pme *PreMovementEngine) determineOverallStatus(candidate *PreMovementCandidate) string {
	// BLOCKED: Failed gates or critical issues
	if candidate.GatesStatus == "BLOCKED" {
		return "BLOCKED"
	}

	score := candidate.TotalScore
	hasConfirmation := candidate.GatesStatus == "CONFIRMED"
	hasSignificantCVD := candidate.CVDResult != nil && candidate.CVDResult.IsSignificant

	// STRONG: High score + confirmation + strong signals
	if score >= 85 && hasConfirmation && hasSignificantCVD {
		return "STRONG"
	}

	// MODERATE: Good score + confirmation OR high score alone
	if (score >= 75 && hasConfirmation) || score >= 90 {
		return "MODERATE"
	}

	// WEAK: Below thresholds but not blocked
	return "WEAK"
}

// rankCandidates sorts candidates by overall strength and assigns ranks
func (pme *PreMovementEngine) rankCandidates(candidates []*PreMovementCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		candi, candj := candidates[i], candidates[j]

		// First sort by overall status priority
		statusPriority := map[string]int{
			"STRONG":   4,
			"MODERATE": 3,
			"WEAK":     2,
			"BLOCKED":  1,
		}

		priI, priJ := statusPriority[candi.OverallStatus], statusPriority[candj.OverallStatus]
		if priI != priJ {
			return priI > priJ
		}

		// Then by Pre-Movement score
		if candi.TotalScore != candj.TotalScore {
			return candi.TotalScore > candj.TotalScore
		}

		// Then by gate precedence score
		var precedenceI, precedenceJ float64
		if candi.GatesResult != nil {
			precedenceI = candi.GatesResult.PrecedenceScore
		}
		if candj.GatesResult != nil {
			precedenceJ = candj.GatesResult.PrecedenceScore
		}

		if precedenceI != precedenceJ {
			return precedenceI > precedenceJ
		}

		// Finally by CVD significance
		var cvdI, cvdJ float64
		if candi.CVDResult != nil {
			cvdI = candi.CVDResult.SignificanceScore
		}
		if candj.CVDResult != nil {
			cvdJ = candj.CVDResult.SignificanceScore
		}

		return cvdI > cvdJ
	})

	// Assign ranks
	for i, candidate := range candidates {
		candidate.Rank = i + 1
	}
}

// assessDataFreshness evaluates overall data quality
func (pme *PreMovementEngine) assessDataFreshness(candidates []*PreMovementCandidate) *DataFreshnessReport {
	report := &DataFreshnessReport{}

	if len(candidates) == 0 {
		report.FreshnessGrade = "N/A"
		return report
	}

	var totalAge, oldestAge int64
	staleCount := 0

	for _, candidate := range candidates {
		if candidate.ScoreBreakdown != nil && candidate.ScoreBreakdown.DataFreshness != nil {
			ageSeconds := int64(candidate.ScoreBreakdown.DataFreshness.OldestFeedHours * 3600)
			totalAge += ageSeconds

			if ageSeconds > oldestAge {
				oldestAge = ageSeconds
			}

			if ageSeconds > pme.config.StaleDataWarning {
				staleCount++
			}
		}
	}

	report.AverageAgeSeconds = totalAge / int64(len(candidates))
	report.OldestDataSeconds = oldestAge
	report.StaleCandidatesCount = staleCount
	report.StaleCandidatesPct = float64(staleCount) / float64(len(candidates)) * 100.0

	// Assign freshness grade
	if report.AverageAgeSeconds < 300 { // < 5 min
		report.FreshnessGrade = "A"
	} else if report.AverageAgeSeconds < 600 { // < 10 min
		report.FreshnessGrade = "B"
	} else if report.AverageAgeSeconds < 1200 { // < 20 min
		report.FreshnessGrade = "C"
	} else if report.AverageAgeSeconds < 1800 { // < 30 min
		report.FreshnessGrade = "D"
	} else {
		report.FreshnessGrade = "F"
	}

	return report
}

// GetAnalysisSummary returns a concise summary of Pre-Movement analysis
func (ar *AnalysisResult) GetAnalysisSummary() string {
	return fmt.Sprintf("Pre-Movement v3.3 Analysis: %d candidates, %d valid, %d strong (freshness: %s, %dms)",
		ar.TotalCandidates, ar.ValidCandidates, ar.StrongCandidates,
		ar.DataFreshness.FreshnessGrade, ar.ProcessTimeMs)
}

// GetTopCandidatesSummary returns a summary of top N candidates
func (ar *AnalysisResult) GetTopCandidatesSummary(n int) string {
	if n > len(ar.Candidates) {
		n = len(ar.Candidates)
	}

	summary := fmt.Sprintf("Top %d Pre-Movement Candidates:\n", n)

	for i := 0; i < n; i++ {
		candidate := ar.Candidates[i]
		status := map[string]string{
			"STRONG":   "🔥",
			"MODERATE": "📈",
			"WEAK":     "📊",
			"BLOCKED":  "❌",
		}[candidate.OverallStatus]

		gates := "❌"
		if candidate.GatesStatus == "CONFIRMED" {
			gates = "✅"
		}

		summary += fmt.Sprintf("  %d. %s %s %s | Score: %.1f | Gates: %s\n",
			candidate.Rank, status, candidate.Symbol, candidate.OverallStatus,
			candidate.TotalScore, gates)
	}

	return summary
}

// GetCandidateDetails returns detailed analysis for a specific candidate
func (ar *AnalysisResult) GetCandidateDetails(symbol string) string {
	for _, candidate := range ar.Candidates {
		if candidate.Symbol == symbol {
			report := fmt.Sprintf("Pre-Movement v3.3 Analysis: %s (Rank #%d)\n", symbol, candidate.Rank)
			report += fmt.Sprintf("Status: %s | Score: %.1f | Gates: %s | Time: %dms\n\n",
				candidate.OverallStatus, candidate.TotalScore, candidate.GatesStatus, candidate.ProcessTimeMs)

			// Key reasons
			if len(candidate.Reasons) > 0 {
				report += "Key Reasons:\n"
				for i, reason := range candidate.Reasons {
					report += fmt.Sprintf("  %d. %s\n", i+1, reason)
				}
				report += "\n"
			}

			// Score breakdown
			if candidate.ScoreBreakdown != nil {
				report += candidate.ScoreBreakdown.GetDetailedBreakdown() + "\n"
			}

			// Gate details
			if candidate.GatesResult != nil {
				report += candidate.GatesResult.GetDetailedReport() + "\n"
			}

			// CVD analysis
			if candidate.CVDResult != nil {
				report += candidate.CVDResult.GetDetailedAnalysis() + "\n"
			}

			// Warnings
			if len(candidate.Warnings) > 0 {
				report += "Warnings:\n"
				for i, warning := range candidate.Warnings {
					report += fmt.Sprintf("  %d. %s\n", i+1, warning)
				}
			}

			return report
		}
	}

	return fmt.Sprintf("Candidate %s not found in analysis results", symbol)
}
