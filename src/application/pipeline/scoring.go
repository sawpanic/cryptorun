package pipeline

import (
	"math"
	"sort"
	"time"

	"github.com/rs/zerolog/log"
)

// CompositeScore represents the final score with component breakdown
type CompositeScore struct {
	Symbol         string                 `json:"symbol"`
	Timestamp      time.Time              `json:"timestamp"`
	Score          float64                `json:"score"`
	Rank           int                    `json:"rank"`
	Components     ScoreComponents        `json:"components"`
	Selected       bool                   `json:"selected"`
	Meta           ScoreMeta              `json:"meta"`
}

type ScoreComponents struct {
	MomentumScore  float64 `json:"momentum_score"`
	VolumeScore    float64 `json:"volume_score"`
	SocialScore    float64 `json:"social_score"`
	VolatilityScore float64 `json:"volatility_score"`
	WeightedSum    float64 `json:"weighted_sum"`
}

type ScoreMeta struct {
	Regime         string    `json:"regime"`
	FactorsUsed    int       `json:"factors_used"`
	ValidationPass bool      `json:"validation_pass"`
	ScoreMethod    string    `json:"score_method"`
	Timestamp      time.Time `json:"timestamp"`
}

// ScoringWeights defines how much each factor contributes to final score
type ScoringWeights struct {
	Momentum   float64 `json:"momentum"`
	Volume     float64 `json:"volume"`  
	Social     float64 `json:"social"`
	Volatility float64 `json:"volatility"`
}

// Default scoring weights (emphasize momentum as primary signal)
var DefaultScoringWeights = ScoringWeights{
	Momentum:   0.60, // 60% - primary momentum signal
	Volume:     0.25, // 25% - volume confirmation
	Social:     0.10, // 10% - social sentiment (capped at +10)
	Volatility: 0.05, // 5%  - volatility adjustment
}

// Scorer handles composite scoring and ranking
type Scorer struct {
	weights ScoringWeights
	regime  string
}

// NewScorer creates a new scorer with default weights
func NewScorer() *Scorer {
	return &Scorer{
		weights: DefaultScoringWeights,
		regime:  "bull",
	}
}

// SetRegime updates scoring regime
func (s *Scorer) SetRegime(regime string) {
	s.regime = regime
	log.Info().Str("regime", regime).Msg("Updated scoring regime")
}

// ComputeScores calculates composite scores for all factor sets
func (s *Scorer) ComputeScores(factorSets []FactorSet) ([]CompositeScore, error) {
	if len(factorSets) == 0 {
		return []CompositeScore{}, nil
	}

	log.Info().Int("symbols", len(factorSets)).Str("regime", s.regime).
		Msg("Computing composite scores")

	scores := make([]CompositeScore, 0, len(factorSets))
	
	for _, fs := range factorSets {
		if !ValidateFactorSet(fs) {
			log.Warn().Str("symbol", fs.Symbol).Msg("Skipping invalid factor set")
			continue
		}
		
		score := s.computeCompositeScore(fs)
		scores = append(scores, score)
	}

	// Rank scores from highest to lowest
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	// Assign ranks
	for i := range scores {
		scores[i].Rank = i + 1
	}

	log.Info().Int("scored_symbols", len(scores)).
		Float64("top_score", func() float64 {
			if len(scores) > 0 { return scores[0].Score }
			return 0.0
		}()).
		Msg("Composite scoring completed")

	return scores, nil
}

// computeCompositeScore calculates the composite score for a single factor set
func (s *Scorer) computeCompositeScore(fs FactorSet) CompositeScore {
	timestamp := time.Now().UTC()

	// Normalize factors to scoring range (typically 0-100)
	momentumScore := s.normalizeMomentumScore(fs.MomentumCore)
	volumeScore := s.normalizeVolumeScore(fs.Volume)
	socialScore := s.normalizeSocialScore(fs.Social)
	volatilityScore := s.normalizeVolatilityScore(fs.Volatility)

	// Calculate weighted composite score
	weightedSum := (momentumScore * s.weights.Momentum) +
		          (volumeScore * s.weights.Volume) +
		          (socialScore * s.weights.Social) +
		          (volatilityScore * s.weights.Volatility)

	// Apply regime adjustments
	finalScore := s.applyRegimeAdjustments(weightedSum, fs)

	components := ScoreComponents{
		MomentumScore:   momentumScore,
		VolumeScore:     volumeScore,
		SocialScore:     socialScore,
		VolatilityScore: volatilityScore,
		WeightedSum:     weightedSum,
	}

	meta := ScoreMeta{
		Regime:         s.regime,
		FactorsUsed:    s.countValidFactors(fs),
		ValidationPass: ValidateFactorSet(fs),
		ScoreMethod:    "weighted_composite",
		Timestamp:      timestamp,
	}

	return CompositeScore{
		Symbol:     fs.Symbol,
		Timestamp:  timestamp,
		Score:      finalScore,
		Rank:       0, // Will be set during ranking
		Components: components,
		Selected:   false, // Will be set during Top-N selection
		Meta:       meta,
	}
}

// normalizeMomentumScore converts momentum to 0-100 scoring range
func (s *Scorer) normalizeMomentumScore(momentum float64) float64 {
	if math.IsNaN(momentum) || math.IsInf(momentum, 0) {
		return 0.0
	}

	// Momentum scoring: sigmoid-like function that rewards strong momentum
	// Scale momentum percentage to score (e.g., +20% momentum → high score)
	
	// Clamp momentum to reasonable range (-50% to +50%)
	clampedMomentum := math.Max(-50.0, math.Min(50.0, momentum))
	
	// Transform to 0-100 scale with sigmoid curve
	// Positive momentum gets exponentially higher scores
	if clampedMomentum >= 0 {
		score := 50.0 + (clampedMomentum * 1.5) // Up to 125 for +50% momentum
		return math.Min(100.0, score)
	} else {
		// Negative momentum gets exponentially lower scores
		score := 50.0 + (clampedMomentum * 2.0) // Down to -50 for -50% momentum
		return math.Max(0.0, score)
	}
}

// normalizeVolumeScore converts volume factor to 0-100 scoring range  
func (s *Scorer) normalizeVolumeScore(volume float64) float64 {
	if math.IsNaN(volume) || math.IsInf(volume, 0) {
		return 50.0 // Neutral score for missing volume
	}

	// Volume scoring: higher volume gets higher scores
	// Assume volume factor is already normalized (e.g., vs average volume)
	
	if volume <= 0 {
		return 0.0
	}

	// Log scale for volume (handles wide range)
	logVolume := math.Log10(volume)
	
	// Transform to 0-100 scale
	// 1x volume = 50, 10x volume = 100, 0.1x volume = 0
	score := 50.0 + (logVolume * 25.0)
	
	return math.Max(0.0, math.Min(100.0, score))
}

// normalizeSocialScore converts social factor to 0-100 scoring range
func (s *Scorer) normalizeSocialScore(social float64) float64 {
	if math.IsNaN(social) || math.IsInf(social, 0) {
		return 50.0 // Neutral score for missing social data
	}

	// Social scoring: -10 to +10 range maps to 0-100 score
	// Remember: social factor is already capped at +10
	
	clampedSocial := math.Max(-10.0, math.Min(10.0, social))
	
	// Linear transformation: -10 → 0, 0 → 50, +10 → 100
	score := 50.0 + (clampedSocial * 2.5)
	
	return math.Max(0.0, math.Min(100.0, score))
}

// normalizeVolatilityScore converts volatility factor to 0-100 scoring range
func (s *Scorer) normalizeVolatilityScore(volatility float64) float64 {
	if math.IsNaN(volatility) || math.IsInf(volatility, 0) {
		return 50.0 // Neutral score for missing volatility
	}

	// Volatility scoring: moderate volatility is preferred (inverted U-shape)
	// Too low = no movement opportunity
	// Too high = too risky
	
	absVolatility := math.Abs(volatility)
	
	// Optimal volatility around 15-25% (gets highest scores)
	if absVolatility >= 15.0 && absVolatility <= 25.0 {
		return 100.0
	} else if absVolatility < 15.0 {
		// Low volatility: score decreases as volatility approaches 0
		return (absVolatility / 15.0) * 100.0
	} else {
		// High volatility: score decreases as volatility increases beyond 25%
		excessVol := absVolatility - 25.0
		penalty := math.Min(excessVol * 2.0, 80.0) // Max penalty of 80 points
		return math.Max(20.0, 100.0 - penalty)
	}
}

// applyRegimeAdjustments applies regime-specific scoring adjustments
func (s *Scorer) applyRegimeAdjustments(baseScore float64, fs FactorSet) float64 {
	switch s.regime {
	case "bull":
		// Bull market: boost momentum-heavy scores
		if fs.MomentumCore > 10.0 {
			return baseScore * 1.05 // 5% boost for strong momentum
		}
	case "choppy":
		// Choppy market: penalize high volatility
		if math.Abs(fs.Volatility) > 30.0 {
			return baseScore * 0.95 // 5% penalty for high volatility  
		}
	case "high_vol":
		// High volatility: boost stable volume performers
		if fs.Volume > 2.0 && math.Abs(fs.Volatility) < 20.0 {
			return baseScore * 1.03 // 3% boost for stable high-volume
		}
	}
	
	return baseScore
}

// countValidFactors counts how many factors have valid (non-NaN) values
func (s *Scorer) countValidFactors(fs FactorSet) int {
	count := 0
	factors := []float64{fs.MomentumCore, fs.Volume, fs.Social, fs.Volatility}
	
	for _, factor := range factors {
		if !math.IsNaN(factor) && !math.IsInf(factor, 0) {
			count++
		}
	}
	
	return count
}

// SelectTopN selects the top N highest-scoring candidates
func (s *Scorer) SelectTopN(scores []CompositeScore, n int) []CompositeScore {
	if len(scores) == 0 {
		return []CompositeScore{}
	}

	// Ensure scores are ranked (should already be sorted)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	// Select top N
	topN := n
	if len(scores) < topN {
		topN = len(scores)
	}

	selected := make([]CompositeScore, topN)
	copy(selected, scores[:topN])

	// Mark selected candidates
	for i := range selected {
		selected[i].Selected = true
		selected[i].Rank = i + 1
	}

	log.Info().Int("selected", len(selected)).Int("total", len(scores)).
		Float64("cutoff_score", func() float64 {
			if len(selected) > 0 { return selected[len(selected)-1].Score }
			return 0.0
		}()).
		Msg("Top-N selection completed")

	return selected
}

// GetScoreBreakdown returns detailed score breakdown for analysis
func (s *Scorer) GetScoreBreakdown(score CompositeScore) map[string]interface{} {
	return map[string]interface{}{
		"symbol":           score.Symbol,
		"final_score":      score.Score,
		"rank":            score.Rank,
		"selected":        score.Selected,
		"momentum_score":  score.Components.MomentumScore,
		"volume_score":    score.Components.VolumeScore,
		"social_score":    score.Components.SocialScore,
		"volatility_score": score.Components.VolatilityScore,
		"weighted_sum":    score.Components.WeightedSum,
		"regime":          score.Meta.Regime,
		"factors_used":    score.Meta.FactorsUsed,
		"validation_pass": score.Meta.ValidationPass,
		"weights": map[string]float64{
			"momentum":   s.weights.Momentum,
			"volume":     s.weights.Volume,
			"social":     s.weights.Social,
			"volatility": s.weights.Volatility,
		},
	}
}