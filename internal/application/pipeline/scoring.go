package pipeline

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"sort"
	"time"

	"github.com/sawpanic/cryptorun/internal/domain/scoring"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// CompositeScore represents the final score with component breakdown
type CompositeScore struct {
	Symbol     string          `json:"symbol"`
	Timestamp  time.Time       `json:"timestamp"`
	Score      float64         `json:"score"`
	Rank       int             `json:"rank"`
	Components ScoreComponents `json:"components"`
	Selected   bool            `json:"selected"`
	Meta       ScoreMeta       `json:"meta"`
}

// GetScore returns the composite score (legacy compatibility)
func (cs CompositeScore) GetScore() float64 {
	return cs.Score
}

// GetRank returns the composite rank (legacy compatibility)
func (cs CompositeScore) GetRank() int {
	return cs.Rank
}

type ScoreComponents struct {
	MomentumScore   float64 `json:"momentum_score"`
	VolumeScore     float64 `json:"volume_score"`
	SocialScore     float64 `json:"social_score"`
	VolatilityScore float64 `json:"volatility_score"`
	WeightedSum     float64 `json:"weighted_sum"`
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
	Momentum  float64 `yaml:"momentum" json:"momentum"`
	Technical float64 `yaml:"technical" json:"technical"`
	Volume    float64 `yaml:"volume" json:"volume"`
	Quality   float64 `yaml:"quality" json:"quality"`
	Social    float64 `yaml:"social" json:"social"`
}

// WeightsConfig represents the complete weights configuration
type WeightsConfig struct {
	Regimes    map[string]RegimeWeights `yaml:"regimes"`
	Validation struct {
		WeightSumTolerance float64 `yaml:"weight_sum_tolerance"`
		MinMomentumWeight  float64 `yaml:"min_momentum_weight"`
		MaxSocialWeight    float64 `yaml:"max_social_weight"`
		SocialHardCap      float64 `yaml:"social_hard_cap"`
	} `yaml:"validation"`
	DefaultRegime string `yaml:"default_regime"`
}

// RegimeWeights contains weights and metadata for a specific regime
type RegimeWeights struct {
	Momentum    float64 `yaml:"momentum"`
	Technical   float64 `yaml:"technical"`
	Volume      float64 `yaml:"volume"`
	Quality     float64 `yaml:"quality"`
	Social      float64 `yaml:"social"`
	Description string  `yaml:"description"`
}

// FactorSet contains all factor values for a given symbol
type FactorSet struct {
	Symbol       string                 `json:"symbol"`
	MomentumCore float64                `json:"momentum_core"` // Protected base momentum
	Technical    float64                `json:"technical"`     // Technical indicators factor
	Volume       float64                `json:"volume"`        // Volume factor
	Quality      float64                `json:"quality"`       // Quality metrics factor
	Social       float64                `json:"social"`        // Social/brand factor (capped at +10)
	Timestamp    time.Time              `json:"timestamp"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ToScoringWeights converts RegimeWeights to ScoringWeights
func (rw RegimeWeights) ToScoringWeights() ScoringWeights {
	return ScoringWeights{
		Momentum:  rw.Momentum,
		Technical: rw.Technical,
		Volume:    rw.Volume,
		Quality:   rw.Quality,
		Social:    rw.Social,
	}
}

// Default scoring weights (fallback if config fails to load)
var DefaultScoringWeights = ScoringWeights{
	Momentum:  0.40, // 40% - primary momentum signal
	Technical: 0.25, // 25% - technical indicators
	Volume:    0.20, // 20% - volume confirmation
	Quality:   0.10, // 10% - quality metrics
	Social:    0.05, // 5%  - social sentiment (capped at +10)
}

// Scorer handles composite scoring and ranking
type Scorer struct {
	weights          ScoringWeights
	regime           string
	config           *WeightsConfig
	configPath       string
	regimeService    *RegimeDetectorService
	autoDetectRegime bool
}

// NewScorer creates a new scorer with weights from config
func NewScorer() *Scorer {
	configPath := "config/weights.yaml"
	config, err := loadWeightsConfig(configPath)
	if err != nil {
		log.Error().Err(err).Str("path", configPath).Msg("Failed to load weights config, using defaults")
		return &Scorer{
			weights:          DefaultScoringWeights,
			regime:           "trending", // default regime
			config:           nil,
			configPath:       configPath,
			regimeService:    NewRegimeDetectorService(),
			autoDetectRegime: false,
		}
	}

	// Initialize regime service
	regimeService := NewRegimeDetectorService()

	// Use default regime from config
	regime := config.DefaultRegime
	weights := config.Regimes[regime].ToScoringWeights()

	// Validate weights
	if err := validateWeights(weights, config); err != nil {
		log.Error().Err(err).Str("regime", regime).Msg("Invalid weights in config, using defaults")
		return &Scorer{
			weights:          DefaultScoringWeights,
			regime:           "trending",
			config:           config,
			configPath:       configPath,
			regimeService:    regimeService,
			autoDetectRegime: false,
		}
	}

	log.Info().Str("regime", regime).Str("config", configPath).
		Float64("momentum", weights.Momentum).
		Float64("technical", weights.Technical).
		Float64("volume", weights.Volume).
		Float64("quality", weights.Quality).
		Float64("social", weights.Social).
		Msg("Loaded scoring weights from config")

	return &Scorer{
		weights:          weights,
		regime:           regime,
		config:           config,
		configPath:       configPath,
		regimeService:    regimeService,
		autoDetectRegime: false,
	}
}

// SetRegime updates scoring regime and loads corresponding weights
func (s *Scorer) SetRegime(regime string) {
	if s.config == nil {
		log.Warn().Str("regime", regime).Msg("No config loaded, cannot update regime weights")
		s.regime = regime
		return
	}

	regimeWeights, exists := s.config.Regimes[regime]
	if !exists {
		log.Error().Str("regime", regime).Msg("Unknown regime, keeping current weights")
		return
	}

	newWeights := regimeWeights.ToScoringWeights()
	if err := validateWeights(newWeights, s.config); err != nil {
		log.Error().Err(err).Str("regime", regime).Msg("Invalid regime weights, keeping current weights")
		return
	}

	s.regime = regime
	s.weights = newWeights

	log.Info().Str("regime", regime).
		Float64("momentum", newWeights.Momentum).
		Float64("volume", newWeights.Volume).
		Float64("social", newWeights.Social).
		Float64("quality", newWeights.Quality).
		Msg("Updated scoring regime and weights")
}

// EnableAutoRegimeDetection enables automatic regime detection and weight updates
func (s *Scorer) EnableAutoRegimeDetection() {
	s.autoDetectRegime = true
	log.Info().Msg("Enabled automatic regime detection for scoring")
}

// DisableAutoRegimeDetection disables automatic regime detection
func (s *Scorer) DisableAutoRegimeDetection() {
	s.autoDetectRegime = false
	log.Info().Msg("Disabled automatic regime detection for scoring")
}

// UpdateRegimeIfNeeded checks for regime changes and updates weights accordingly
func (s *Scorer) UpdateRegimeIfNeeded(ctx context.Context) error {
	if !s.autoDetectRegime || s.regimeService == nil {
		return nil // Auto-detection disabled
	}

	// Check if regime update is due
	shouldUpdate, err := s.regimeService.ShouldUpdate(ctx)
	if err != nil {
		return fmt.Errorf("failed to check regime update status: %w", err)
	}

	if !shouldUpdate {
		return nil // No update needed
	}

	// Detect and update regime
	result, err := s.regimeService.DetectAndUpdateRegime(ctx)
	if err != nil {
		return fmt.Errorf("regime detection failed: %w", err)
	}

	// Update scoring weights if regime changed
	newRegimeStr := result.Regime.String()
	if newRegimeStr != s.regime {
		s.SetRegime(newRegimeStr)
		log.Info().
			Str("old_regime", s.regime).
			Str("new_regime", newRegimeStr).
			Float64("confidence", result.Confidence).
			Msg("Auto-updated scoring regime")
	}

	return nil
}

// GetRegimeStatus returns current regime detection status
func (s *Scorer) GetRegimeStatus() map[string]interface{} {
	status := map[string]interface{}{
		"current_regime":      s.regime,
		"auto_detect_enabled": s.autoDetectRegime,
		"weights": map[string]float64{
			"momentum":  s.weights.Momentum,
			"technical": s.weights.Technical,
			"volume":    s.weights.Volume,
			"quality":   s.weights.Quality,
			"social":    s.weights.Social,
		},
	}

	if s.regimeService != nil {
		regimeStatus := s.regimeService.GetRegimeStatus()
		status["regime_service"] = regimeStatus
	}

	return status
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
			if len(scores) > 0 {
				return scores[0].Score
			}
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
	qualityScore := s.normalizeQualityScore(fs.Quality)

	// Calculate weighted composite score
	weightedSum := (momentumScore * s.weights.Momentum) +
		(volumeScore * s.weights.Volume) +
		(socialScore * s.weights.Social) +
		(qualityScore * s.weights.Quality)

	// Apply regime adjustments
	finalScore := s.applyRegimeAdjustments(weightedSum, fs)

	components := ScoreComponents{
		MomentumScore:   momentumScore,
		VolumeScore:     volumeScore,
		SocialScore:     socialScore,
		VolatilityScore: qualityScore,
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
	volumeMetrics := scoring.NormalizeVolumeScore(volume)
	return volumeMetrics.Score
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

// normalizeQualityScore converts quality factor to 0-100 scoring range
func (s *Scorer) normalizeQualityScore(quality float64) float64 {
	// Simple normalization for quality factor
	// Assuming quality factor ranges from -100 to +100, normalize to 0-100
	score := 50.0 + (quality * 0.5) // Maps [-100,100] to [0,100]
	return math.Max(0.0, math.Min(100.0, score))
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
		if math.Abs(fs.Quality) > 30.0 {
			return baseScore * 0.95 // 5% penalty for high volatility
		}
	case "high_vol":
		// High volatility: boost stable volume performers
		if fs.Volume > 2.0 && math.Abs(fs.Quality) < 20.0 {
			return baseScore * 1.03 // 3% boost for stable high-volume
		}
	}

	return baseScore
}

// countValidFactors counts how many factors have valid (non-NaN) values
func (s *Scorer) countValidFactors(fs FactorSet) int {
	count := 0
	factors := []float64{fs.MomentumCore, fs.Technical, fs.Volume, fs.Quality, fs.Social}

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
			if len(selected) > 0 {
				return selected[len(selected)-1].Score
			}
			return 0.0
		}()).
		Msg("Top-N selection completed")

	return selected
}

// GetScoreBreakdown returns detailed score breakdown for analysis
func (s *Scorer) GetScoreBreakdown(score CompositeScore) map[string]interface{} {
	return map[string]interface{}{
		"symbol":          score.Symbol,
		"final_score":     score.Score,
		"rank":            score.Rank,
		"selected":        score.Selected,
		"momentum_score":  score.Components.MomentumScore,
		"volume_score":    score.Components.VolumeScore,
		"social_score":    score.Components.SocialScore,
		"quality_score":   score.Components.VolatilityScore,
		"weighted_sum":    score.Components.WeightedSum,
		"regime":          score.Meta.Regime,
		"factors_used":    score.Meta.FactorsUsed,
		"validation_pass": score.Meta.ValidationPass,
		"weights": map[string]float64{
			"momentum": s.weights.Momentum,
			"volume":   s.weights.Volume,
			"social":   s.weights.Social,
			"quality":  s.weights.Quality,
		},
	}
}

// LoadWeightsConfig loads weights configuration from YAML file (exported for testing)
func LoadWeightsConfig(configPath string) (*WeightsConfig, error) {
	return loadWeightsConfig(configPath)
}

// loadWeightsConfig loads weights configuration from YAML file
func loadWeightsConfig(configPath string) (*WeightsConfig, error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config WeightsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	return &config, nil
}

// ValidateWeights ensures weights meet validation requirements (exported for testing)
func ValidateWeights(weights ScoringWeights, config *WeightsConfig) error {
	return validateWeights(weights, config)
}

// validateWeights ensures weights meet validation requirements
func validateWeights(weights ScoringWeights, config *WeightsConfig) error {
	if config == nil {
		// Basic validation without config
		sum := weights.Momentum + weights.Technical + weights.Volume + weights.Quality + weights.Social
		if math.Abs(sum-1.0) > 0.01 {
			return fmt.Errorf("weights sum to %.3f, expected 1.0", sum)
		}
		return nil
	}

	validation := config.Validation

	// Check weight sum
	sum := weights.Momentum + weights.Technical + weights.Volume + weights.Quality + weights.Social
	if math.Abs(sum-1.0) > validation.WeightSumTolerance {
		return fmt.Errorf("weights sum to %.6f, expected 1.0 ± %.6f", sum, validation.WeightSumTolerance)
	}

	// Check minimum momentum weight
	if weights.Momentum < validation.MinMomentumWeight {
		return fmt.Errorf("momentum weight %.3f below minimum %.3f", weights.Momentum, validation.MinMomentumWeight)
	}

	// Check maximum social weight
	if weights.Social > validation.MaxSocialWeight {
		return fmt.Errorf("social weight %.3f exceeds maximum %.3f", weights.Social, validation.MaxSocialWeight)
	}

	// Validate individual weights are non-negative
	if weights.Momentum < 0 || weights.Technical < 0 || weights.Volume < 0 || weights.Quality < 0 || weights.Social < 0 {
		return fmt.Errorf("all weights must be non-negative")
	}

	return nil
}

// GetAvailableRegimes returns list of available regimes from config
func (s *Scorer) GetAvailableRegimes() []string {
	if s.config == nil {
		return []string{"trending", "choppy", "high_vol"} // fallback regimes
	}

	regimes := make([]string, 0, len(s.config.Regimes))
	for regime := range s.config.Regimes {
		regimes = append(regimes, regime)
	}
	return regimes
}

// GetCurrentWeights returns current scoring weights
func (s *Scorer) GetCurrentWeights() ScoringWeights {
	return s.weights
}

// GetWeightSum returns sum of all weights (should be 1.0)
func (s *Scorer) GetWeightSum() float64 {
	return s.weights.Momentum + s.weights.Technical + s.weights.Volume + s.weights.Quality + s.weights.Social
}
