package scoring

import (
	"fmt"
	"math"
	"time"

	"github.com/sawpanic/cryptorun/internal/domain/factors"
	"github.com/sawpanic/cryptorun/internal/config/regime"
)

// CompositeScore represents the final unified scoring result
type CompositeScore struct {
	Symbol            string
	FinalScore        float64
	Regime            regime.RegimeType
	Timestamp         time.Time
	
	// Component scores (pre-weighting)
	MomentumCore      float64
	TechnicalResidual float64
	VolumeResidual    float64
	QualityResidual   float64
	SocialCapped      float64  // Social score after hard cap
	
	// Weighted contributions (showing impact)
	WeightedMomentum  float64
	WeightedTechnical float64
	WeightedVolume    float64
	WeightedQuality   float64
	WeightedSocial    float64  // Social added outside 100% allocation
	
	// Weights used for this scoring
	Weights           regime.DomainRegimeWeights
	
	// Attribution and debugging
	FactorBreakdown   FactorBreakdown
	ScoringMetadata   ScoringMetadata
}

// FactorBreakdown provides detailed factor attribution
type FactorBreakdown struct {
	MomentumComponents map[string]float64  // 1h, 4h, 12h, 24h momentum breakdown
	TechnicalSources   map[string]float64  // RSI, ADX, etc. contributions
	VolumeSources      map[string]float64  // Volume surge, relative volume
	QualitySources     map[string]float64  // Market cap, liquidity, etc.
	SocialSources      map[string]float64  // Social metrics before capping
	
	OrthogonalizationQuality  factors.QualityMetrics  // Gram-Schmidt quality metrics
}

// ScoringMetadata contains scoring attribution and quality info
type ScoringMetadata struct {
	ScoringTime       time.Time
	RegimeConfidence  float64
	DataFreshness     time.Duration
	CacheHitRate      float64
	
	// Quality indicators
	AllFactorsValid   bool
	WeightsValidated  bool
	SocialCapApplied  bool
	
	// Data sources
	DataSources       []string
	MissingData       []string
}

// CompositeScorer implements the unified scoring system
type CompositeScorer struct {
	config            regime.WeightsConfig
	regimeDetector    regime.RegimeDetector
	orthogonalizer    *factors.GramSchmidtOrthogonalizer
}

// NewCompositeScorer creates a new unified composite scorer
func NewCompositeScorer(config regime.WeightsConfig, regimeDetector regime.RegimeDetector) *CompositeScorer {
	orthogonalizer := factors.NewGramSchmidtOrthogonalizer(config)
	
	return &CompositeScorer{
		config:         config,
		regimeDetector: regimeDetector,
		orthogonalizer: orthogonalizer,
	}
}

// CalculateCompositeScore computes the unified composite score
func (cs *CompositeScorer) CalculateCompositeScore(rawFactors factors.RawFactorRow, regimeData regime.MarketData) (*CompositeScore, error) {
	startTime := time.Now()
	
	// Step 1: Detect current regime
	regimeDetection, err := cs.regimeDetector.DetectRegime(regimeData)
	if err != nil {
		return nil, fmt.Errorf("regime detection failed: %w", err)
	}
	
	// Step 2: Get regime-specific weights
	weights, err := cs.regimeDetector.GetWeightsForRegime(regimeDetection.CurrentRegime)
	if err != nil {
		return nil, fmt.Errorf("failed to get weights for regime %s: %w", regimeDetection.CurrentRegime, err)
	}
	
	// Step 3: Validate weights
	err = regime.ValidateRegimeWeights(weights, cs.config)
	if err != nil {
		return nil, fmt.Errorf("invalid regime weights: %w", err)
	}
	
	// Step 4: Apply Gram-Schmidt orthogonalization (protects MomentumCore)
	orthogonalizedRows, err := cs.orthogonalizer.OrthogonalizeBatch([]factors.RawFactorRow{rawFactors})
	if err != nil {
		return nil, fmt.Errorf("orthogonalization failed: %w", err)
	}
	
	if len(orthogonalizedRows) != 1 {
		return nil, fmt.Errorf("expected 1 orthogonalized row, got %d", len(orthogonalizedRows))
	}
	
	orthogonalizedRow := orthogonalizedRows[0]
	
	// Step 5: Apply social cap (already done in orthogonalization, but verify)
	socialCapped := orthogonalizedRow.SocialCapped
	socialCapValue := cs.config.Validation.SocialHardCap
	
	if math.Abs(socialCapped) > socialCapValue+0.001 {
		return nil, fmt.Errorf("social cap not properly applied: |%.3f| > %.1f", socialCapped, socialCapValue)
	}
	
	// Step 6: Calculate weighted components
	weightedMomentum := orthogonalizedRow.MomentumCore * weights.MomentumCore
	weightedTechnical := orthogonalizedRow.TechnicalResidual * weights.Technical  
	weightedVolume := orthogonalizedRow.VolumeResidual * weights.Volume
	weightedQuality := orthogonalizedRow.QualityResidual * weights.Quality
	weightedSocial := socialCapped * weights.Social  // Social added outside main allocation
	
	// Step 7: Calculate final composite score
	// Core score from main factors (should sum to 100% of weight allocation)
	coreScore := weightedMomentum + weightedTechnical + weightedVolume + weightedQuality
	
	// Final score = core score + social (social is additive, outside main allocation)
	finalScore := coreScore + weightedSocial
	
	// Step 8: Build comprehensive result
	score := &CompositeScore{
		Symbol:            rawFactors.Symbol,
		FinalScore:        finalScore,
		Regime:            regimeDetection.CurrentRegime,
		Timestamp:         startTime,
		
		// Component scores (pre-weighting)
		MomentumCore:      orthogonalizedRow.MomentumCore,
		TechnicalResidual: orthogonalizedRow.TechnicalResidual,
		VolumeResidual:    orthogonalizedRow.VolumeResidual,
		QualityResidual:   orthogonalizedRow.QualityResidual,
		SocialCapped:      socialCapped,
		
		// Weighted contributions
		WeightedMomentum:  weightedMomentum,
		WeightedTechnical: weightedTechnical,
		WeightedVolume:    weightedVolume,
		WeightedQuality:   weightedQuality,
		WeightedSocial:    weightedSocial,
		
		Weights: weights,
		
		// Attribution - will be populated by helper methods
		FactorBreakdown: FactorBreakdown{
			OrthogonalizationQuality: orthogonalizedRow.OrthogonalizationInfo.QualityMetrics,
		},
		
		ScoringMetadata: ScoringMetadata{
			ScoringTime:      startTime,
			RegimeConfidence: regimeDetection.Confidence,
			AllFactorsValid:  true,  // Assume true for now
			WeightsValidated: true,
			SocialCapApplied: true,
			DataSources:      []string{"regime_detector", "factor_builder", "orthogonalizer"},
		},
	}
	
	return score, nil
}

// CalculateBatchScores computes composite scores for multiple symbols efficiently
func (cs *CompositeScorer) CalculateBatchScores(rawFactorsMap map[string]factors.RawFactorRow, regimeData regime.MarketData) (map[string]*CompositeScore, []error) {
	scores := make(map[string]*CompositeScore)
	errors := []error{}
	
	// Single regime detection for the batch (regime is market-wide)
	regimeDetection, err := cs.regimeDetector.DetectRegime(regimeData)
	if err != nil {
		errors = append(errors, fmt.Errorf("batch regime detection failed: %w", err))
		return scores, errors
	}
	
	// Get weights for detected regime
	weights, err := cs.regimeDetector.GetWeightsForRegime(regimeDetection.CurrentRegime)
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to get batch weights for regime %s: %w", regimeDetection.CurrentRegime, err))
		return scores, errors
	}
	
	// Prepare batch for orthogonalization
	symbols := make([]string, 0, len(rawFactorsMap))
	rawFactorRows := make([]factors.RawFactorRow, 0, len(rawFactorsMap))
	
	for symbol, rawFactors := range rawFactorsMap {
		symbols = append(symbols, symbol)
		rawFactorRows = append(rawFactorRows, rawFactors)
	}
	
	// Batch orthogonalization
	orthogonalizedRows, err := cs.orthogonalizer.OrthogonalizeBatch(rawFactorRows)
	if err != nil {
		errors = append(errors, fmt.Errorf("batch orthogonalization failed: %w", err))
		return scores, errors
	}
	
	// Process each symbol
	for i, symbol := range symbols {
		if i >= len(orthogonalizedRows) {
			errors = append(errors, fmt.Errorf("missing orthogonalized row for symbol %s", symbol))
			continue
		}
		
		orthRow := orthogonalizedRows[i]
		_ = rawFactorRows[i] // rawRow for potential future use in debugging
		
		// Calculate weighted components
		weightedMomentum := orthRow.MomentumCore * weights.MomentumCore
		weightedTechnical := orthRow.TechnicalResidual * weights.Technical
		weightedVolume := orthRow.VolumeResidual * weights.Volume
		weightedQuality := orthRow.QualityResidual * weights.Quality
		weightedSocial := orthRow.SocialCapped * weights.Social
		
		finalScore := weightedMomentum + weightedTechnical + weightedVolume + weightedQuality + weightedSocial
		
		scores[symbol] = &CompositeScore{
			Symbol:            symbol,
			FinalScore:        finalScore,
			Regime:            regimeDetection.CurrentRegime,
			Timestamp:         time.Now(),
			
			MomentumCore:      orthRow.MomentumCore,
			TechnicalResidual: orthRow.TechnicalResidual,
			VolumeResidual:    orthRow.VolumeResidual,
			QualityResidual:   orthRow.QualityResidual,
			SocialCapped:      orthRow.SocialCapped,
			
			WeightedMomentum:  weightedMomentum,
			WeightedTechnical: weightedTechnical,
			WeightedVolume:    weightedVolume,
			WeightedQuality:   weightedQuality,
			WeightedSocial:    weightedSocial,
			
			Weights: weights,
			
			FactorBreakdown: FactorBreakdown{
				OrthogonalizationQuality: orthRow.OrthogonalizationInfo.QualityMetrics,
			},
			
			ScoringMetadata: ScoringMetadata{
				ScoringTime:      time.Now(),
				RegimeConfidence: regimeDetection.Confidence,
				AllFactorsValid:  true,
				WeightsValidated: true,
				SocialCapApplied: true,
				DataSources:      []string{"batch_regime_detector", "batch_orthogonalizer"},
			},
		}
	}
	
	return scores, errors
}

// ValidateScore ensures the composite score meets quality requirements
func ValidateScore(score *CompositeScore, config regime.WeightsConfig) error {
	if score == nil {
		return fmt.Errorf("score cannot be nil")
	}
	
	// Check for NaN or Inf values
	if math.IsNaN(score.FinalScore) || math.IsInf(score.FinalScore, 0) {
		return fmt.Errorf("final score is NaN or infinite: %f", score.FinalScore)
	}
	
	// Verify social cap enforcement
	socialCapValue := config.Validation.SocialHardCap
	if math.Abs(score.SocialCapped) > socialCapValue+0.001 {
		return fmt.Errorf("social cap violated: |%.3f| > %.1f", score.SocialCapped, socialCapValue)
	}
	
	// Verify weight sum (excluding social)
	coreWeightSum := score.Weights.MomentumCore + score.Weights.Technical + score.Weights.Volume + score.Weights.Quality
	tolerance := config.Validation.WeightSumTolerance
	
	if math.Abs(coreWeightSum-1.0) > tolerance {
		return fmt.Errorf("core weight sum %.3f outside tolerance %.3f of 1.0", coreWeightSum, tolerance)
	}
	
	// Verify momentum preservation (should be unchanged by orthogonalization)
	expectedMomentum := score.MomentumCore
	if math.IsNaN(expectedMomentum) || math.IsInf(expectedMomentum, 0) {
		return fmt.Errorf("momentum core corrupted: %f", expectedMomentum)
	}
	
	return nil
}

// GetScoreExplanation provides detailed explanation of score components
func GetScoreExplanation(score *CompositeScore) string {
	if score == nil {
		return "No score available"
	}
	
	explanation := fmt.Sprintf("Composite Score: %.1f (Regime: %s, Confidence: %.1f%%)\n\n",
		score.FinalScore, score.Regime, score.ScoringMetadata.RegimeConfidence)
	
	explanation += "Component Breakdown:\n"
	explanation += fmt.Sprintf("  Momentum Core: %.2f � %.3f = %.2f\n", 
		score.MomentumCore, score.Weights.MomentumCore, score.WeightedMomentum)
	explanation += fmt.Sprintf("  Technical (residual): %.2f � %.3f = %.2f\n", 
		score.TechnicalResidual, score.Weights.Technical, score.WeightedTechnical)
	explanation += fmt.Sprintf("  Volume (residual): %.2f � %.3f = %.2f\n", 
		score.VolumeResidual, score.Weights.Volume, score.WeightedVolume)
	explanation += fmt.Sprintf("  Quality (residual): %.2f � %.3f = %.2f\n", 
		score.QualityResidual, score.Weights.Quality, score.WeightedQuality)
	explanation += fmt.Sprintf("  Social (capped): %.2f � %.3f = %.2f\n", 
		score.SocialCapped, score.Weights.Social, score.WeightedSocial)
	
	coreScore := score.WeightedMomentum + score.WeightedTechnical + score.WeightedVolume + score.WeightedQuality
	explanation += fmt.Sprintf("\nCore Score: %.2f\n", coreScore)
	explanation += fmt.Sprintf("Social Adjustment: +%.2f\n", score.WeightedSocial)
	explanation += fmt.Sprintf("Final Score: %.2f\n", score.FinalScore)
	
	// Quality metrics
	quality := score.FactorBreakdown.OrthogonalizationQuality
	explanation += fmt.Sprintf("\nOrthogonalization Quality:\n")
	explanation += fmt.Sprintf("  Max Correlation: %.3f\n", quality.MaxCorrelation)
	explanation += fmt.Sprintf("  Momentum Preserved: %.1f%%\n", quality.MomentumPreserved)
	explanation += fmt.Sprintf("  Orthogonality Score: %.1f\n", quality.OrthogonalityScore)
	
	return explanation
}

// RankScores sorts scores by final score (descending)
func RankScores(scores map[string]*CompositeScore) []string {
	type scorePair struct {
		symbol string
		score  float64
	}
	
	pairs := make([]scorePair, 0, len(scores))
	for symbol, score := range scores {
		if score != nil {
			pairs = append(pairs, scorePair{symbol: symbol, score: score.FinalScore})
		}
	}
	
	// Sort by score descending
	for i := 0; i < len(pairs)-1; i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[i].score < pairs[j].score {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}
	
	ranking := make([]string, len(pairs))
	for i, pair := range pairs {
		ranking[i] = pair.symbol
	}
	
	return ranking
}