package composite

import (
	"encoding/json"
	"fmt"
	"time"
)

// Explanation provides comprehensive scoring breakdown and reasoning
type Explanation struct {
	// Core scoring breakdown
	Score         CompositeScore    `json:"score"`
	Weights       RegimeWeights     `json:"weights"`
	Orthogonality OrthogonalityInfo `json:"orthogonality"`
	Gates         GateExplanation   `json:"gates"`

	// Context and metadata
	Symbol      string            `json:"symbol"`
	Timestamp   time.Time         `json:"timestamp"`
	Regime      string            `json:"regime"`
	DataSources map[string]string `json:"data_sources"`

	// Decision reasoning
	Reasons  []string `json:"reasons"`
	Warnings []string `json:"warnings"`

	// Performance metrics
	LatencyMs float64         `json:"latency_ms"`
	CacheHits map[string]bool `json:"cache_hits"`
}

// EnhancedExplanation extends Explanation with new measurement insights
type EnhancedExplanation struct {
	// Original explanation fields
	Explanation

	// Measurement insights
	FundingInsight    string  `json:"funding_insight"`
	OIInsight         string  `json:"oi_insight"`
	ETFInsight        string  `json:"etf_insight"`
	MeasurementsBoost float64 `json:"measurements_boost"`
	DataQuality       string  `json:"data_quality"`

	// Attribution
	FundingAttribution string `json:"funding_attribution"`
	OIAttribution      string `json:"oi_attribution"`
	ETFAttribution     string `json:"etf_attribution"`
}

// OrthogonalityInfo provides orthogonalization diagnostics
type OrthogonalityInfo struct {
	Matrix       [][]float64        `json:"matrix"`        // Dot product matrix
	Magnitudes   map[string]float64 `json:"magnitudes"`    // Factor magnitudes
	IsOrthogonal bool               `json:"is_orthogonal"` // Passes tolerance check
	Tolerance    float64            `json:"tolerance"`     // Tolerance used
}

// GateExplanation breaks down entry gate evaluation
type GateExplanation struct {
	Overall     GateResult             `json:"overall"`
	Thresholds  map[string]interface{} `json:"thresholds"`
	Performance GatePerformance        `json:"performance"`
}

// GatePerformance provides gate evaluation metrics
type GatePerformance struct {
	TotalGates    int      `json:"total_gates"`
	PassedGates   int      `json:"passed_gates"`
	FailedGates   int      `json:"failed_gates"`
	PassRate      float64  `json:"pass_rate"`
	CriticalFails []string `json:"critical_fails"` // Gates that must pass
}

// Explainer generates comprehensive scoring explanations
type Explainer struct {
	gates          *HardEntryGates
	orthogonalizer *Orthogonalizer
}

// NewExplainer creates a new explanation generator
func NewExplainer(gates *HardEntryGates, orthogonalizer *Orthogonalizer) *Explainer {
	return &Explainer{
		gates:          gates,
		orthogonalizer: orthogonalizer,
	}
}

// Explain generates a complete explanation for a composite score
func (e *Explainer) Explain(
	score CompositeScore,
	weights RegimeWeights,
	orthogonalized *OrthogonalizedFactors,
	gateResult GateResult,
	input ScoringInput,
	latencyMs float64,
) *Explanation {

	explanation := &Explanation{
		Score:       score,
		Weights:     weights,
		Symbol:      score.Symbol,
		Timestamp:   score.Timestamp,
		Regime:      score.Regime,
		DataSources: input.DataSources,
		LatencyMs:   latencyMs,
		CacheHits:   make(map[string]bool),
	}

	// Generate orthogonality diagnostics
	explanation.Orthogonality = e.explainOrthogonality(orthogonalized)

	// Generate gate explanation
	explanation.Gates = e.explainGates(gateResult)

	// Generate decision reasoning
	explanation.Reasons = e.buildReasons(&score, &gateResult)
	explanation.Warnings = e.identifyWarnings(&score, &gateResult)

	return explanation
}

// explainOrthogonality provides orthogonalization diagnostics
func (e *Explainer) explainOrthogonality(factors *OrthogonalizedFactors) OrthogonalityInfo {
	// Get orthogonality matrix and magnitudes
	matrix := e.orthogonalizer.GetOrthogonalityMatrix(factors)
	magnitudes := e.orthogonalizer.ComputeResidualMagnitudes(factors)

	// Check orthogonality with standard tolerance
	tolerance := 0.01
	isOrthogonal := e.orthogonalizer.ValidateOrthogonality(factors, tolerance) == nil

	return OrthogonalityInfo{
		Matrix:       matrix,
		Magnitudes:   magnitudes,
		IsOrthogonal: isOrthogonal,
		Tolerance:    tolerance,
	}
}

// explainGates provides comprehensive gate analysis
func (e *Explainer) explainGates(result GateResult) GateExplanation {
	thresholds := e.gates.GetThresholds()

	// Calculate performance metrics
	totalGates := len(result.GatesPassed)
	passedGates := 0
	var criticalFails []string

	for gate, passed := range result.GatesPassed {
		if passed {
			passedGates++
		} else {
			// Identify critical gate failures
			if e.isCriticalGate(gate) {
				criticalFails = append(criticalFails, gate)
			}
		}
	}

	passRate := 0.0
	if totalGates > 0 {
		passRate = float64(passedGates) / float64(totalGates)
	}

	performance := GatePerformance{
		TotalGates:    totalGates,
		PassedGates:   passedGates,
		FailedGates:   totalGates - passedGates,
		PassRate:      passRate,
		CriticalFails: criticalFails,
	}

	return GateExplanation{
		Overall:     result,
		Thresholds:  thresholds,
		Performance: performance,
	}
}

// isCriticalGate identifies gates that are critical for entry
func (e *Explainer) isCriticalGate(gate string) bool {
	criticalGates := map[string]bool{
		"composite_score":    true, // NEW: Must have score ≥75
		"vadr":               true, // NEW: Must have VADR ≥1.8×
		"funding_divergence": true, // NEW: Must have funding divergence
		"bar_age":            true, // Data must be fresh
		"spread":             true, // Must have reasonable execution cost
		"depth":              true, // Must have sufficient liquidity
	}

	return criticalGates[gate]
}

// buildReasons constructs decision reasoning chain
func (e *Explainer) buildReasons(score *CompositeScore, gates *GateResult) []string {
	var reasons []string

	// Score composition reasoning
	reasons = append(reasons, e.explainScoreComposition(score)...)

	// Gate reasoning
	reasons = append(reasons, e.explainGateDecisions(gates)...)

	// Social capping reasoning
	if score.SocialResid < 10.0 {
		reasons = append(reasons, fmt.Sprintf("Social factor contributed +%.1f points (uncapped)",
			score.SocialResid))
	} else {
		reasons = append(reasons, "Social factor capped at +10.0 points maximum")
	}

	// Final decision reasoning
	if gates.Allowed {
		reasons = append(reasons, fmt.Sprintf("Entry ALLOWED: Score %.1f ≥ 75, all gates passed",
			score.Internal0to100))
	} else {
		reasons = append(reasons, fmt.Sprintf("Entry BLOCKED: %s", gates.Reason))
	}

	return reasons
}

// explainScoreComposition breaks down score calculation
func (e *Explainer) explainScoreComposition(score *CompositeScore) []string {
	var reasons []string

	reasons = append(reasons, fmt.Sprintf("Regime: %s (affects weight allocation)", score.Regime))
	reasons = append(reasons, fmt.Sprintf("MomentumCore: %.1f points (protected factor)",
		score.MomentumCore))
	reasons = append(reasons, fmt.Sprintf("TechnicalResid: %.1f points (post-momentum residual)",
		score.TechnicalResid))
	reasons = append(reasons, fmt.Sprintf("VolumeResid: %.1f points (volume + ΔOI residual)",
		score.VolumeResid.Combined))
	reasons = append(reasons, fmt.Sprintf("QualityResid: %.1f points (OI + reserves + ETF + venue)",
		score.QualityResid.Combined))
	reasons = append(reasons, fmt.Sprintf("Internal total: %.1f/100 (before social)",
		score.Internal0to100))

	return reasons
}

// explainGateDecisions provides gate-by-gate reasoning
func (e *Explainer) explainGateDecisions(result *GateResult) []string {
	var reasons []string

	// Group gates by category for cleaner explanation
	if !result.CompositePass {
		reasons = append(reasons, "COMPOSITE GATES: Failed entry score requirements")
		for gate := range result.GatesPassed {
			if gate == "composite_score" || gate == "vadr" || gate == "funding_divergence" {
				if reason, exists := result.GateReasons[gate]; exists {
					reasons = append(reasons, fmt.Sprintf("  - %s", reason))
				}
			}
		}
	}

	if !result.FreshnessPass {
		reasons = append(reasons, "FRESHNESS GATES: Data freshness violations")
	}

	if !result.MicroPass {
		reasons = append(reasons, "MICROSTRUCTURE GATES: Liquidity/execution violations")
	}

	if !result.FatiguePass {
		reasons = append(reasons, "FATIGUE GATES: Overextension protection triggered")
	}

	if !result.PolicyPass {
		reasons = append(reasons, "POLICY GATES: Timing/administrative violations")
	}

	return reasons
}

// identifyWarnings finds potential issues or edge cases
func (e *Explainer) identifyWarnings(score *CompositeScore, gates *GateResult) []string {
	var warnings []string

	// Score-related warnings
	if score.Internal0to100 < 80 && gates.Allowed {
		warnings = append(warnings, "Score below 80 - consider higher confidence threshold")
	}

	if score.SocialResid >= 8.0 {
		warnings = append(warnings, "High social factor influence - verify fundamental strength")
	}

	// Gate-related warnings handled elsewhere

	// Orthogonality warnings handled in orthogonality section

	return warnings
}

// ToJSON serializes explanation to JSON
func (e *Explanation) ToJSON() ([]byte, error) {
	return json.MarshalIndent(e, "", "  ")
}

// ToCompactString provides a compact text summary
func (e *Explanation) ToCompactString() string {
	decision := "BLOCKED"
	if e.Gates.Overall.Allowed {
		decision = "ALLOWED"
	}

	return fmt.Sprintf("%s %s: Score %.1f, Gates %d/%d passed, Regime %s",
		e.Symbol, decision, e.Score.Internal0to100,
		e.Gates.Performance.PassedGates, e.Gates.Performance.TotalGates, e.Regime)
}

// GetKeyMetrics returns essential metrics for monitoring
func (e *Explanation) GetKeyMetrics() map[string]interface{} {
	return map[string]interface{}{
		"symbol":            e.Symbol,
		"score":             e.Score.Internal0to100,
		"final_with_social": e.Score.FinalWithSocial,
		"allowed":           e.Gates.Overall.Allowed,
		"regime":            e.Regime,
		"gate_pass_rate":    0.0,
		"social_capped":     e.Score.SocialResid,
		"latency_ms":        e.LatencyMs,
		"timestamp":         e.Timestamp,
	}
}

// EnhancedExplainer generates explanations with measurement insights
type EnhancedExplainer struct {
	baseExplainer *Explainer
}

// NewEnhancedExplainer creates a new enhanced explanation generator
func NewEnhancedExplainer(gates *HardEntryGates, orthogonalizer *Orthogonalizer) *EnhancedExplainer {
	return &EnhancedExplainer{
		baseExplainer: NewExplainer(gates, orthogonalizer),
	}
}

// ExplainWithMeasurements generates enhanced explanation with measurement insights
func (ee *EnhancedExplainer) ExplainWithMeasurements(
	result *EnhancedCompositeResult,
	weights RegimeWeights,
	orthogonalized *OrthogonalizedFactors,
	gateResult GateResult,
	input ScoringInput,
	latencyMs float64,
) *EnhancedExplanation {

	// Generate base explanation using the composite score equivalent
	baseScore := CompositeScore{
		MomentumCore:    result.MomentumCore,
		TechnicalResid:  result.TechnicalResid,
		VolumeResid:     VolumeComponents{Combined: result.VolumeResid},
		QualityResid:    QualityComponents{Combined: result.QualityResid},
		SocialResid:     result.SocialResidCapped,
		Internal0to100:  result.FinalScore,
		FinalWithSocial: result.FinalScoreWithSocial,
		Regime:          result.Regime,
		Timestamp:       result.Timestamp,
		Symbol:          input.Symbol,
	}

	baseExplanation := ee.baseExplainer.Explain(
		baseScore, weights, orthogonalized, gateResult, input, latencyMs)

	// Create enhanced explanation
	enhanced := &EnhancedExplanation{
		Explanation:       *baseExplanation,
		FundingInsight:    result.FundingInsight,
		OIInsight:         result.OIInsight,
		ETFInsight:        result.ETFInsight,
		MeasurementsBoost: result.MeasurementsBoost,
		DataQuality:       result.DataQuality,
	}

	// Generate attribution strings
	enhanced.FundingAttribution = ee.generateFundingAttribution(result)
	enhanced.OIAttribution = ee.generateOIAttribution(result)
	enhanced.ETFAttribution = ee.generateETFAttribution(result)

	// Enhance reasoning with measurement context
	enhanced.Reasons = ee.enhanceReasoning(enhanced.Reasons, result)
	enhanced.Warnings = ee.enhanceWarnings(enhanced.Warnings, result)

	return enhanced
}

// generateFundingAttribution creates funding data attribution
func (ee *EnhancedExplainer) generateFundingAttribution(result *EnhancedCompositeResult) string {
	if result.FundingInsight == "Funding data unavailable" {
		return "No funding data sources available"
	}

	return fmt.Sprintf("Funding: Cross-venue 7d σ analysis from Binance/OKX/Bybit (cached %.0fs ago)",
		time.Since(result.Timestamp).Seconds())
}

// generateOIAttribution creates OI data attribution
func (ee *EnhancedExplainer) generateOIAttribution(result *EnhancedCompositeResult) string {
	if result.OIInsight == "OI data unavailable" {
		return "No open interest data sources available"
	}

	return fmt.Sprintf("OI: 1h Δ with β-regression residual from Binance/OKX (cached %.0fs ago)",
		time.Since(result.Timestamp).Seconds())
}

// generateETFAttribution creates ETF data attribution
func (ee *EnhancedExplainer) generateETFAttribution(result *EnhancedCompositeResult) string {
	if result.ETFInsight == "ETF data unavailable" {
		return "No ETF flow data sources available"
	}

	return fmt.Sprintf("ETF: Daily net flows from issuer dashboards vs 7d ADV (cached %.0fs ago)",
		time.Since(result.Timestamp).Seconds())
}

// enhanceReasoning adds measurement-specific reasoning
func (ee *EnhancedExplainer) enhanceReasoning(baseReasons []string, result *EnhancedCompositeResult) []string {
	enhanced := make([]string, len(baseReasons))
	copy(enhanced, baseReasons)

	// Add measurement insights
	if result.MeasurementsBoost > 0 {
		enhanced = append(enhanced, fmt.Sprintf("Measurement Boost: +%.1f points from data insights",
			result.MeasurementsBoost))

		if result.FundingInsight != "Funding data unavailable" && result.FundingInsight != "Funding rates normal" {
			enhanced = append(enhanced, fmt.Sprintf("  - Funding: %s", result.FundingInsight))
		}

		if result.OIInsight != "OI data unavailable" && result.OIInsight != "OI activity normal" {
			enhanced = append(enhanced, fmt.Sprintf("  - Open Interest: %s", result.OIInsight))
		}

		if result.ETFInsight != "ETF data unavailable" && result.ETFInsight != "ETF flows balanced" {
			enhanced = append(enhanced, fmt.Sprintf("  - ETF Flows: %s", result.ETFInsight))
		}
	}

	// Add data quality assessment
	enhanced = append(enhanced, fmt.Sprintf("Data Coverage: %s", result.DataQuality))

	return enhanced
}

// enhanceWarnings adds measurement-specific warnings
func (ee *EnhancedExplainer) enhanceWarnings(baseWarnings []string, result *EnhancedCompositeResult) []string {
	enhanced := make([]string, len(baseWarnings))
	copy(enhanced, baseWarnings)

	// Data quality warnings
	if result.DataQuality == "Incomplete (0/3 sources)" {
		enhanced = append(enhanced, "No measurement data available - relying on base factors only")
	} else if result.DataQuality == "Limited (1/3 sources)" {
		enhanced = append(enhanced, "Limited measurement data - consider waiting for more sources")
	}

	// High boost warnings
	if result.MeasurementsBoost >= 3.0 {
		enhanced = append(enhanced, "Very high measurement boost - verify data integrity")
	}

	// Specific measurement warnings
	if result.FundingInsight != "Funding data unavailable" && result.MeasurementsBoost >= 1.5 {
		enhanced = append(enhanced, "Strong funding signal - monitor for mean reversion")
	}

	if result.OIInsight != "OI data unavailable" && result.MeasurementsBoost >= 1.0 {
		enhanced = append(enhanced, "Significant OI activity - verify volume confirmation")
	}

	return enhanced
}

// ToEnhancedJSON serializes enhanced explanation to JSON
func (ee *EnhancedExplanation) ToEnhancedJSON() ([]byte, error) {
	return json.MarshalIndent(ee, "", "  ")
}

// ToEnhancedCompactString provides enhanced compact summary
func (ee *EnhancedExplanation) ToEnhancedCompactString() string {
	decision := "BLOCKED"
	if ee.Gates.Overall.Allowed {
		decision = "ALLOWED"
	}

	return fmt.Sprintf("%s %s: Score %.1f+%.1f, %s, Gates %d/%d, %s",
		ee.Symbol, decision, ee.Score.Internal0to100, ee.MeasurementsBoost,
		ee.DataQuality, ee.Gates.Performance.PassedGates,
		ee.Gates.Performance.TotalGates, ee.Regime)
}

// GetEnhancedKeyMetrics returns essential metrics including measurements
func (ee *EnhancedExplanation) GetEnhancedKeyMetrics() map[string]interface{} {
	base := ee.Explanation.GetKeyMetrics()

	// Add measurement metrics
	base["measurements_boost"] = ee.MeasurementsBoost
	base["data_quality"] = ee.DataQuality
	base["funding_insight"] = ee.FundingInsight
	base["oi_insight"] = ee.OIInsight
	base["etf_insight"] = ee.ETFInsight
	base["enhanced_final_score"] = ee.Score.FinalWithSocial + ee.MeasurementsBoost

	return base
}
