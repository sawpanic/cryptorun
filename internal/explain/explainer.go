package explain

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cryptorun/internal/config/regime"
	"cryptorun/internal/data/derivs"
	"cryptorun/internal/gates"
	"cryptorun/internal/microstructure"
	"cryptorun/internal/score/composite"
)

// Explainer provides human-readable explanations for scoring decisions
type Explainer struct {
	regimeWeights   *regime.WeightsLoader
	fundingProvider *derivs.FundingProvider
	oiProvider      *derivs.OpenInterestProvider
	etfProvider     *derivs.ETFProvider
}

// NewExplainer creates a scoring explainer
func NewExplainer(
	regimeWeights *regime.WeightsLoader,
	fundingProvider *derivs.FundingProvider,
	oiProvider *derivs.OpenInterestProvider,
	etfProvider *derivs.ETFProvider,
) *Explainer {
	return &Explainer{
		regimeWeights:   regimeWeights,
		fundingProvider: fundingProvider,
		oiProvider:      oiProvider,
		etfProvider:     etfProvider,
	}
}

// ScoringExplanation contains detailed breakdown of scoring decision
type ScoringExplanation struct {
	Symbol     string    `json:"symbol"`
	Timestamp  time.Time `json:"timestamp"`
	FinalScore float64   `json:"final_score"`
	Regime     string    `json:"regime"`

	// Score breakdown
	CompositeBreakdown  *CompositeScoreBreakdown       `json:"composite_breakdown"`
	FactorContributions map[string]*FactorContribution `json:"factor_contributions"`
	WeightExplanation   *WeightExplanation             `json:"weight_explanation"`

	// Supporting data explanations
	MicrostructureExplanation *MicrostructureExplanation `json:"microstructure_explanation"`
	DerivativesExplanation    *DerivativesExplanation    `json:"derivatives_explanation"`

	// Entry gate explanations
	EntryGateExplanation *EntryGateExplanation `json:"entry_gate_explanation,omitempty"`

	// Key insights summary
	KeyInsights []string `json:"key_insights"`
	RiskFlags   []string `json:"risk_flags"`

	// Attribution
	DataSources map[string]string `json:"data_sources"`
}

// CompositeScoreBreakdown explains the composite scoring calculation
type CompositeScoreBreakdown struct {
	RawFactors        map[string]float64 `json:"raw_factors"`        // Pre-orthogonalization
	OrthogonalFactors map[string]float64 `json:"orthogonal_factors"` // Post-orthogonalization
	WeightedFactors   map[string]float64 `json:"weighted_factors"`   // After weight application
	InternalTotal     float64            `json:"internal_total"`     // 0-100 score before social
	SocialAddition    float64            `json:"social_addition"`    // +social cap (0 to +10)
	FinalScore        float64            `json:"final_score"`        // Internal + Social
}

// FactorContribution explains a single factor's contribution
type FactorContribution struct {
	Name            string  `json:"name"`
	RawValue        float64 `json:"raw_value"`
	OrthogonalValue float64 `json:"orthogonal_value"`
	Weight          float64 `json:"weight"`
	Contribution    float64 `json:"contribution"`
	Interpretation  string  `json:"interpretation"`
	DataQuality     string  `json:"data_quality"`
}

// WeightExplanation explains the regime-based weight allocation
type WeightExplanation struct {
	CurrentRegime    string             `json:"current_regime"`
	RegimeConfidence float64            `json:"regime_confidence"`
	WeightProfile    map[string]float64 `json:"weight_profile"`
	RegimeReasoning  string             `json:"regime_reasoning"`
}

// MicrostructureExplanation explains microstructure validation
type MicrostructureExplanation struct {
	VADR                float64 `json:"vadr"`
	SpreadBps           float64 `json:"spread_bps"`
	DepthUSD            float64 `json:"depth_usd"`
	LiquidityAssessment string  `json:"liquidity_assessment"`
	RiskAssessment      string  `json:"risk_assessment"`
}

// DerivativesExplanation explains derivatives data insights
type DerivativesExplanation struct {
	FundingDivergence   *FundingExplanation `json:"funding_divergence,omitempty"`
	OpenInterestInsight *OIExplanation      `json:"open_interest_insight,omitempty"`
	ETFFlowInsight      *ETFExplanation     `json:"etf_flow_insight,omitempty"`
}

// FundingExplanation explains funding rate divergence
type FundingExplanation struct {
	MaxDivergence   float64            `json:"max_divergence"`
	DivergentVenue  string             `json:"divergent_venue"`
	CrossVenueRates map[string]float64 `json:"cross_venue_rates"`
	Interpretation  string             `json:"interpretation"`
}

// OIExplanation explains open interest dynamics
type OIExplanation struct {
	OIChange24h    float64 `json:"oi_change_24h"`
	OIResidual     float64 `json:"oi_residual"`
	Interpretation string  `json:"interpretation"`
}

// ETFExplanation explains ETF flow dynamics
type ETFExplanation struct {
	FlowTint       float64            `json:"flow_tint"`
	NetFlows       float64            `json:"net_flows"`
	DominantETF    string             `json:"dominant_etf"`
	ETFFlows       map[string]float64 `json:"etf_flows"`
	Interpretation string             `json:"interpretation"`
}

// EntryGateExplanation explains entry gate evaluation
type EntryGateExplanation struct {
	OverallResult  string                      `json:"overall_result"`
	GateResults    map[string]*GateExplanation `json:"gate_results"`
	BlockingGates  []string                    `json:"blocking_gates,omitempty"`
	Recommendation string                      `json:"recommendation"`
}

// GateExplanation explains a single gate result
type GateExplanation struct {
	Passed         bool        `json:"passed"`
	Value          interface{} `json:"value"`
	Threshold      interface{} `json:"threshold"`
	Impact         string      `json:"impact"`         // Impact on entry decision
	Recommendation string      `json:"recommendation"` // What to do about it
}

// ExplainScoring provides comprehensive explanation of scoring decision
func (e *Explainer) ExplainScoring(
	ctx context.Context,
	symbol string,
	compositeScore *composite.CompositeScore,
	rawFactors *composite.RawFactors,
	currentRegime string,
	microResult *microstructure.EvaluationResult,
) (*ScoringExplanation, error) {

	explanation := &ScoringExplanation{
		Symbol:              symbol,
		Timestamp:           time.Now(),
		FinalScore:          compositeScore.FinalScoreWithSocial,
		Regime:              currentRegime,
		FactorContributions: make(map[string]*FactorContribution),
		DataSources:         make(map[string]string),
		KeyInsights:         []string{},
		RiskFlags:           []string{},
	}

	// Explain composite score breakdown
	e.explainCompositeBreakdown(explanation, compositeScore, rawFactors)

	// Explain individual factor contributions
	e.explainFactorContributions(explanation, compositeScore, rawFactors, currentRegime)

	// Explain weight allocation
	e.explainWeightAllocation(explanation, currentRegime)

	// Explain microstructure assessment
	e.explainMicrostructure(explanation, microResult)

	// Explain derivatives data (async to avoid blocking)
	go e.explainDerivativesAsync(ctx, explanation, symbol)

	// Generate key insights
	e.generateKeyInsights(explanation, compositeScore, microResult)

	return explanation, nil
}

// explainCompositeBreakdown explains the step-by-step score calculation
func (e *Explainer) explainCompositeBreakdown(
	explanation *ScoringExplanation,
	compositeScore *composite.CompositeScore,
	rawFactors *composite.RawFactors,
) {
	explanation.CompositeBreakdown = &CompositeScoreBreakdown{
		RawFactors: map[string]float64{
			"momentum_core": rawFactors.MomentumCore,
			"technical":     rawFactors.Technical,
			"volume":        rawFactors.Volume,
			"quality":       rawFactors.Quality,
			"social":        rawFactors.Social,
		},
		OrthogonalFactors: map[string]float64{
			"momentum_core":   compositeScore.MomentumCore, // Protected
			"technical_resid": compositeScore.TechnicalResid,
			"volume_resid":    compositeScore.VolumeResid,
			"quality_resid":   compositeScore.QualityResid,
			"social_resid":    compositeScore.SocialResidCapped,
		},
		WeightedFactors: map[string]float64{
			// These would be calculated from the weighted components
			"momentum_weighted":  compositeScore.MomentumCore * 0.4, // Example weights
			"technical_weighted": compositeScore.TechnicalResid * 0.2,
			"volume_weighted":    compositeScore.VolumeResid * 0.15,
			"quality_weighted":   compositeScore.QualityResid * 0.15,
		},
		InternalTotal:  compositeScore.InternalTotal100,
		SocialAddition: compositeScore.SocialResidCapped,
		FinalScore:     compositeScore.FinalScoreWithSocial,
	}
}

// explainFactorContributions explains each factor's role and contribution
func (e *Explainer) explainFactorContributions(
	explanation *ScoringExplanation,
	compositeScore *composite.CompositeScore,
	rawFactors *composite.RawFactors,
	currentRegime string,
) {
	// Get regime weights
	weights, err := e.regimeWeights.GetWeightsForRegime(currentRegime)
	if err != nil {
		weights = e.regimeWeights.GetDefaultWeights()
	}

	// MomentumCore (protected)
	explanation.FactorContributions["momentum_core"] = &FactorContribution{
		Name:            "Momentum Core",
		RawValue:        rawFactors.MomentumCore,
		OrthogonalValue: compositeScore.MomentumCore,
		Weight:          weights.MomentumCore,
		Contribution:    compositeScore.MomentumCore * weights.MomentumCore,
		Interpretation:  e.interpretMomentumCore(rawFactors.MomentumCore),
		DataQuality:     "high", // Assumed high quality for core momentum
	}

	// Technical Residual
	explanation.FactorContributions["technical"] = &FactorContribution{
		Name:            "Technical (Residualized)",
		RawValue:        rawFactors.Technical,
		OrthogonalValue: compositeScore.TechnicalResid,
		Weight:          weights.TechnicalResid,
		Contribution:    compositeScore.TechnicalResid * weights.TechnicalResid,
		Interpretation:  e.interpretTechnical(compositeScore.TechnicalResid),
		DataQuality:     "medium",
	}

	// Volume Residual
	supplyDemandWeight := weights.SupplyDemandBlock
	volumeWeight := 0.55 * supplyDemandWeight
	explanation.FactorContributions["volume"] = &FactorContribution{
		Name:            "Volume (Residualized)",
		RawValue:        rawFactors.Volume,
		OrthogonalValue: compositeScore.VolumeResid,
		Weight:          volumeWeight,
		Contribution:    compositeScore.VolumeResid * volumeWeight,
		Interpretation:  e.interpretVolume(compositeScore.VolumeResid),
		DataQuality:     "high",
	}

	// Quality Residual
	qualityWeight := 0.45 * supplyDemandWeight
	explanation.FactorContributions["quality"] = &FactorContribution{
		Name:            "Quality (Residualized)",
		RawValue:        rawFactors.Quality,
		OrthogonalValue: compositeScore.QualityResid,
		Weight:          qualityWeight,
		Contribution:    compositeScore.QualityResid * qualityWeight,
		Interpretation:  e.interpretQuality(compositeScore.QualityResid),
		DataQuality:     "medium",
	}

	// Social (capped)
	explanation.FactorContributions["social"] = &FactorContribution{
		Name:            "Social (Capped at +10)",
		RawValue:        rawFactors.Social,
		OrthogonalValue: compositeScore.SocialResidCapped,
		Weight:          1.0, // Applied outside 100% allocation
		Contribution:    compositeScore.SocialResidCapped,
		Interpretation:  e.interpretSocial(compositeScore.SocialResidCapped, rawFactors.Social),
		DataQuality:     "low", // Social data is typically lower quality
	}
}

// explainWeightAllocation explains the regime-based weight selection
func (e *Explainer) explainWeightAllocation(explanation *ScoringExplanation, currentRegime string) {
	weights, err := e.regimeWeights.GetWeightsForRegime(currentRegime)
	if err != nil {
		weights = e.regimeWeights.GetDefaultWeights()
	}

	explanation.WeightExplanation = &WeightExplanation{
		CurrentRegime:    currentRegime,
		RegimeConfidence: 0.85, // Mock confidence - would come from regime detector
		WeightProfile: map[string]float64{
			"momentum_core":       weights.MomentumCore,
			"technical_resid":     weights.TechnicalResid,
			"supply_demand_block": weights.SupplyDemandBlock,
			"catalyst_block":      weights.CatalystBlock,
		},
		RegimeReasoning: e.explainRegimeChoice(currentRegime),
	}
}

// explainMicrostructure explains the microstructure assessment
func (e *Explainer) explainMicrostructure(
	explanation *ScoringExplanation,
	microResult *microstructure.EvaluationResult,
) {
	explanation.MicrostructureExplanation = &MicrostructureExplanation{
		VADR:                microResult.VADR,
		SpreadBps:           microResult.SpreadBps,
		DepthUSD:            microResult.DepthUSD,
		LiquidityAssessment: e.assessLiquidity(microResult),
		RiskAssessment:      e.assessMicroRisk(microResult),
	}

	// Add microstructure insights to key insights
	if microResult.VADR < 1.5 {
		explanation.RiskFlags = append(explanation.RiskFlags,
			fmt.Sprintf("Low VADR (%.2f×) suggests thin liquidity", microResult.VADR))
	}

	if microResult.SpreadBps > 30 {
		explanation.RiskFlags = append(explanation.RiskFlags,
			fmt.Sprintf("Wide spread (%.1f bps) increases execution cost", microResult.SpreadBps))
	}
}

// explainDerivativesAsync explains derivatives data (runs asynchronously)
func (e *Explainer) explainDerivativesAsync(ctx context.Context, explanation *ScoringExplanation, symbol string) {
	derivExplanation := &DerivativesExplanation{}

	// Funding divergence
	if fundingSnapshot, err := e.fundingProvider.GetFundingSnapshot(ctx, symbol); err == nil {
		venue, zScore := fundingSnapshot.GetDivergentVenue()
		derivExplanation.FundingDivergence = &FundingExplanation{
			MaxDivergence:   fundingSnapshot.MaxDivergence,
			DivergentVenue:  venue,
			CrossVenueRates: fundingSnapshot.VenueRates,
			Interpretation:  e.interpretFundingDivergence(fundingSnapshot.MaxDivergence, venue),
		}
	}

	// Open interest
	if oiSnapshot, err := e.oiProvider.GetOpenInterestSnapshot(ctx, symbol, 0.05); err == nil {
		derivExplanation.OpenInterestInsight = &OIExplanation{
			OIChange24h:    oiSnapshot.OIChange24h,
			OIResidual:     oiSnapshot.OIResidual,
			Interpretation: e.interpretOI(oiSnapshot.OIResidual, oiSnapshot.OIChange24h),
		}
	}

	// ETF flows
	if etfSnapshot, err := e.etfProvider.GetETFFlowSnapshot(ctx, symbol); err == nil && len(etfSnapshot.ETFList) > 0 {
		dominantETF, _ := etfSnapshot.GetDominantETF()
		derivExplanation.ETFFlowInsight = &ETFExplanation{
			FlowTint:       etfSnapshot.FlowTint,
			NetFlows:       etfSnapshot.TotalFlow,
			DominantETF:    dominantETF,
			ETFFlows:       etfSnapshot.ETFFlows,
			Interpretation: e.interpretETFFlows(etfSnapshot.FlowTint, etfSnapshot.TotalFlow),
		}
	}

	explanation.DerivativesExplanation = derivExplanation
}

// ExplainEntryGates provides detailed explanation of entry gate evaluation
func (e *Explainer) ExplainEntryGates(entryResult *gates.EntryGateResult) *EntryGateExplanation {
	explanation := &EntryGateExplanation{
		GateResults:   make(map[string]*GateExplanation),
		BlockingGates: []string{},
	}

	if entryResult.Passed {
		explanation.OverallResult = "All entry gates passed - ENTRY CLEARED ✅"
		explanation.Recommendation = "Asset cleared for entry consideration subject to portfolio constraints"
	} else {
		explanation.OverallResult = "Entry gates failed - ENTRY BLOCKED ❌"
		explanation.Recommendation = "Wait for improved conditions or consider alternative assets"
	}

	// Explain each gate result
	for gateName, gateCheck := range entryResult.GateResults {
		explanation.GateResults[gateName] = &GateExplanation{
			Passed:         gateCheck.Passed,
			Value:          gateCheck.Value,
			Threshold:      gateCheck.Threshold,
			Impact:         e.explainGateImpact(gateName, gateCheck.Passed),
			Recommendation: e.explainGateRecommendation(gateName, gateCheck.Passed, gateCheck.Value),
		}

		if !gateCheck.Passed {
			explanation.BlockingGates = append(explanation.BlockingGates, gateName)
		}
	}

	return explanation
}

// generateKeyInsights creates high-level insights from the analysis
func (e *Explainer) generateKeyInsights(
	explanation *ScoringExplanation,
	compositeScore *composite.CompositeScore,
	microResult *microstructure.EvaluationResult,
) {
	insights := []string{}

	// Score level insights
	if compositeScore.FinalScoreWithSocial >= 85 {
		insights = append(insights, "🔥 Strong momentum signal across multiple timeframes")
	} else if compositeScore.FinalScoreWithSocial >= 75 {
		insights = append(insights, "✅ Solid momentum signal with good risk/reward")
	} else if compositeScore.FinalScoreWithSocial >= 50 {
		insights = append(insights, "⚠️ Mixed signals - proceed with caution")
	} else {
		insights = append(insights, "❌ Weak signal - consider avoiding")
	}

	// Liquidity insights
	if microResult.VADR >= 2.0 && microResult.SpreadBps <= 25 {
		insights = append(insights, "💧 Excellent liquidity conditions for execution")
	} else if microResult.VADR >= 1.8 {
		insights = append(insights, "💧 Adequate liquidity for moderate position sizes")
	}

	// Factor dominance insights
	if compositeScore.MomentumCore > 70 {
		insights = append(insights, "⚡ Momentum-driven opportunity with strong price action")
	}

	if compositeScore.SocialResidCapped > 5 {
		insights = append(insights, "📱 Positive social sentiment providing additional lift")
	}

	explanation.KeyInsights = insights
}

// Interpretation helpers

func (e *Explainer) interpretMomentumCore(value float64) string {
	if value >= 80 {
		return "Very strong multi-timeframe momentum"
	} else if value >= 60 {
		return "Good momentum with consistent price action"
	} else if value >= 40 {
		return "Moderate momentum - mixed signals"
	} else {
		return "Weak momentum - little price conviction"
	}
}

func (e *Explainer) interpretTechnical(value float64) string {
	if value > 10 {
		return "Technical indicators strongly bullish after removing momentum correlation"
	} else if value > 0 {
		return "Technical indicators mildly positive independent of momentum"
	} else {
		return "Technical indicators neutral to negative after momentum adjustment"
	}
}

func (e *Explainer) interpretVolume(value float64) string {
	if value > 15 {
		return "Exceptional volume activity beyond price-explained levels"
	} else if value > 5 {
		return "Above-average volume supporting price action"
	} else {
		return "Volume in line with or below price-implied levels"
	}
}

func (e *Explainer) interpretQuality(value float64) string {
	if value > 10 {
		return "High-quality asset with strong fundamentals and market structure"
	} else if value > 0 {
		return "Decent quality metrics after adjusting for other factors"
	} else {
		return "Quality concerns or crowded positioning"
	}
}

func (e *Explainer) interpretSocial(residValue, rawValue float64) string {
	if residValue >= 8 {
		return fmt.Sprintf("Strong social buzz (%.1f/10 cap) - raw social: %.1f", residValue, rawValue)
	} else if residValue >= 4 {
		return fmt.Sprintf("Moderate social interest (%.1f/10) - raw: %.1f", residValue, rawValue)
	} else {
		return fmt.Sprintf("Limited social attention (%.1f/10) - raw: %.1f", residValue, rawValue)
	}
}

func (e *Explainer) explainRegimeChoice(regime string) string {
	switch regime {
	case "calm":
		return "Low volatility environment - higher momentum allocation, lower catalyst weighting"
	case "volatile":
		return "High volatility environment - emphasis on technical signals and risk management"
	case "normal":
		return "Balanced market conditions - standard weight allocation across factors"
	default:
		return "Standard regime weighting applied"
	}
}

func (e *Explainer) assessLiquidity(microResult *microstructure.EvaluationResult) string {
	if microResult.VADR >= 2.5 && microResult.DepthUSD >= 500000 && microResult.SpreadBps <= 20 {
		return "Excellent - deep, tight markets suitable for large positions"
	} else if microResult.VADR >= 1.8 && microResult.DepthUSD >= 100000 && microResult.SpreadBps <= 50 {
		return "Good - adequate liquidity for moderate position sizes"
	} else {
		return "Limited - requires careful position sizing and execution timing"
	}
}

func (e *Explainer) assessMicroRisk(microResult *microstructure.EvaluationResult) string {
	riskFactors := []string{}

	if microResult.VADR < 1.5 {
		riskFactors = append(riskFactors, "low VADR")
	}
	if microResult.SpreadBps > 75 {
		riskFactors = append(riskFactors, "wide spreads")
	}
	if microResult.DepthUSD < 50000 {
		riskFactors = append(riskFactors, "thin depth")
	}

	if len(riskFactors) == 0 {
		return "Low microstructure risk"
	} else {
		return fmt.Sprintf("Elevated risk: %s", strings.Join(riskFactors, ", "))
	}
}

func (e *Explainer) interpretFundingDivergence(maxDivergence float64, venue string) string {
	if maxDivergence >= 3.0 {
		return fmt.Sprintf("Extreme funding divergence (%.2f σ) at %s - strong directional signal", maxDivergence, venue)
	} else if maxDivergence >= 2.0 {
		return fmt.Sprintf("Significant funding divergence (%.2f σ) at %s - moderate directional signal", maxDivergence, venue)
	} else {
		return fmt.Sprintf("Limited funding divergence (%.2f σ) - weak signal", maxDivergence)
	}
}

func (e *Explainer) interpretOI(residual, change float64) string {
	if residual > 5000000 && change > 0 {
		return "Strong OI growth beyond price-explained levels - bullish structure building"
	} else if residual > 0 {
		return "Moderate OI expansion independent of price action"
	} else {
		return "OI contraction or in line with price moves"
	}
}

func (e *Explainer) interpretETFFlows(tint, netFlows float64) string {
	if tint >= 0.5 {
		return fmt.Sprintf("Strong ETF inflows (%.1f%% tint) - institutional demand", tint*100)
	} else if tint >= 0.2 {
		return fmt.Sprintf("Modest ETF inflows (%.1f%% tint)", tint*100)
	} else if tint <= -0.2 {
		return fmt.Sprintf("ETF outflows (%.1f%% tint) - institutional selling", tint*100)
	} else {
		return "Neutral ETF flows"
	}
}

func (e *Explainer) explainGateImpact(gateName string, passed bool) string {
	impacts := map[string]map[bool]string{
		"composite_score": {
			true:  "Score meets minimum threshold for consideration",
			false: "Score too low for entry - insufficient signal strength",
		},
		"vadr": {
			true:  "Adequate liquidity for execution",
			false: "Insufficient liquidity - high slippage risk",
		},
		"spread": {
			true:  "Tight spreads enable efficient execution",
			false: "Wide spreads increase transaction costs",
		},
		"depth": {
			true:  "Sufficient market depth for position sizing",
			false: "Thin orderbook limits position size",
		},
		"funding_divergence": {
			true:  "Funding divergence confirms directional bias",
			false: "No funding divergence - weaker conviction signal",
		},
	}

	if gateImpacts, exists := impacts[gateName]; exists {
		if impact, exists := gateImpacts[passed]; exists {
			return impact
		}
	}

	return "Standard gate evaluation"
}

func (e *Explainer) explainGateRecommendation(gateName string, passed bool, value interface{}) string {
	if passed {
		return "Gate passed - no action required"
	}

	recommendations := map[string]string{
		"composite_score":    "Wait for stronger momentum signal or consider alternative assets",
		"vadr":               "Monitor for improved liquidity conditions before entry",
		"spread":             "Consider limit orders or smaller position sizes to minimize impact",
		"depth":              "Reduce position size or use TWAP execution strategy",
		"funding_divergence": "Wait for funding divergence to emerge or reduce conviction",
	}

	if rec, exists := recommendations[gateName]; exists {
		return rec
	}

	return "Address gate failure before considering entry"
}
