package pipeline

import (
	"math"

	"cryptorun/internal/domain"
	"github.com/rs/zerolog/log"
)

// Note: FactorSet type is now defined in scoring.go as the canonical definition

type OrthogonalMeta struct {
	ProtectedFactors   []string                      `json:"protected_factors"`
	CorrelationMatrix  map[string]map[string]float64 `json:"correlation_matrix,omitempty"`
	OrthogonalityCheck float64                       `json:"orthogonality_check"`
}

// Orthogonalizer handles Gram-Schmidt orthogonalization with protected factors
type Orthogonalizer struct {
	protectedFactors []string
}

// NewOrthogonalizer creates a new orthogonalizer with MomentumCore protection
func NewOrthogonalizer() *Orthogonalizer {
	return &Orthogonalizer{
		protectedFactors: []string{"momentum_core"},
	}
}

// OrthogonalizeFactors applies Gram-Schmidt with protected MomentumCore
func (o *Orthogonalizer) OrthogonalizeFactors(factorSets []FactorSet) ([]FactorSet, error) {
	if len(factorSets) == 0 {
		return factorSets, nil
	}

	log.Info().Int("symbols", len(factorSets)).Msg("Starting factor orthogonalization")

	// Extract factor vectors for matrix operations
	factorMatrix := make([][]float64, len(factorSets))

	for i, fs := range factorSets {
		factorMatrix[i] = []float64{
			fs.MomentumCore,
			fs.Technical,
			fs.Volume,
			fs.Quality,
			fs.Social,
		}
	}

	// Apply Gram-Schmidt orthogonalization using domain package
	orthogonalMatrix := domain.GramSchmidt(factorMatrix)

	// Protect MomentumCore: restore original momentum values to first column
	// This ensures momentum signal is never distorted by other factors
	for i := range orthogonalMatrix {
		orthogonalMatrix[i][0] = factorMatrix[i][0]
	}

	log.Info().Msg("MomentumCore protection applied - momentum signals preserved")

	// Build orthogonalized factor sets
	orthogonalFactorSets := make([]FactorSet, len(factorSets))

	for i, originalSet := range factorSets {
		orthogonalSet := FactorSet{
			Symbol:       originalSet.Symbol,
			MomentumCore: orthogonalMatrix[i][0], // Protected momentum
			Technical:    orthogonalMatrix[i][1], // Orthogonalized technical
			Volume:       orthogonalMatrix[i][2], // Orthogonalized volume
			Quality:      orthogonalMatrix[i][3], // Orthogonalized quality
			Social:       orthogonalMatrix[i][4], // Orthogonalized social
			Timestamp:    originalSet.Timestamp,
			Metadata:     originalSet.Metadata,
		}

		orthogonalFactorSets[i] = orthogonalSet
	}

	log.Info().Msg("Factor orthogonalization completed")

	return orthogonalFactorSets, nil
}

// computeOrthogonalityCheck calculates how orthogonal the factor vector is
func (o *Orthogonalizer) computeOrthogonalityCheck(factors []float64) float64 {
	// Simple check: compute norm of the vector
	sumSquares := 0.0
	for _, f := range factors {
		if !math.IsNaN(f) && !math.IsInf(f, 0) {
			sumSquares += f * f
		}
	}
	return math.Sqrt(sumSquares)
}

// ApplySocialCap enforces the +10 maximum social factor contribution
func (o *Orthogonalizer) ApplySocialCap(factorSets []FactorSet) []FactorSet {
	const maxSocialContribution = 10.0
	cappedCount := 0

	for i := range factorSets {
		// Cap social factor at +10
		if factorSets[i].Social > maxSocialContribution {
			factorSets[i].Social = maxSocialContribution
			cappedCount++
		}
	}

	if cappedCount > 0 {
		log.Info().Int("capped_symbols", cappedCount).
			Float64("max_social", maxSocialContribution).
			Msg("Applied social factor cap")
	}

	return factorSets
}

// BuildFactorSet constructs a FactorSet from momentum and other inputs
func BuildFactorSet(symbol string, momentum *MomentumFactors, technicalFactor, volumeFactor, qualityFactor, socialFactor float64) FactorSet {
	// Calculate momentum core from regime-weighted momentum
	momentumCalc := NewMomentumCalculator(nil) // We only need weights, not data provider
	momentumCore := momentumCalc.ApplyRegimeWeights(momentum)

	factorSet := FactorSet{
		Symbol:       symbol,
		MomentumCore: momentumCore,
		Technical:    technicalFactor,
		Volume:       volumeFactor,
		Quality:      qualityFactor,
		Social:       socialFactor,
		Timestamp:    momentum.Timestamp,
		Metadata: map[string]interface{}{
			"momentum_1h":  momentum.Momentum1h,
			"momentum_4h":  momentum.Momentum4h,
			"momentum_12h": momentum.Momentum12h,
			"momentum_24h": momentum.Momentum24h,
			"momentum_7d":  momentum.Momentum7d,
			"volume_1h":    momentum.Volume1h,
			"volume_4h":    momentum.Volume4h,
			"volume_24h":   momentum.Volume24h,
			"rsi_4h":       momentum.RSI4h,
			"atr_1h":       momentum.ATR1h,
		},
	}

	return factorSet
}

// ValidateFactorSet checks if a factor set has valid (non-NaN) core factors
func ValidateFactorSet(fs FactorSet) bool {
	// Check core factors for validity
	coreFactors := []float64{fs.MomentumCore, fs.Technical, fs.Volume, fs.Quality, fs.Social}

	validCount := 0
	for _, factor := range coreFactors {
		if !math.IsNaN(factor) && !math.IsInf(factor, 0) {
			validCount++
		}
	}

	// Require at least momentum core and one other factor to be valid
	return validCount >= 2 && !math.IsNaN(fs.MomentumCore)
}

// ComputeCorrelationMatrix calculates factor correlations for analysis
func (o *Orthogonalizer) ComputeCorrelationMatrix(factorSets []FactorSet) map[string]map[string]float64 {
	factorNames := []string{"momentum_core", "technical", "volume", "quality", "social"}

	// Extract factor vectors
	factorVectors := make(map[string][]float64)
	for _, name := range factorNames {
		factorVectors[name] = make([]float64, len(factorSets))
	}

	for i, fs := range factorSets {
		factorVectors["momentum_core"][i] = fs.MomentumCore
		factorVectors["technical"][i] = fs.Technical
		factorVectors["volume"][i] = fs.Volume
		factorVectors["quality"][i] = fs.Quality
		factorVectors["social"][i] = fs.Social
	}

	// Compute correlation matrix
	correlationMatrix := make(map[string]map[string]float64)

	for _, name1 := range factorNames {
		correlationMatrix[name1] = make(map[string]float64)
		for _, name2 := range factorNames {
			correlation := o.computeCorrelation(factorVectors[name1], factorVectors[name2])
			correlationMatrix[name1][name2] = correlation
		}
	}

	return correlationMatrix
}

// computeCorrelation calculates Pearson correlation coefficient
func (o *Orthogonalizer) computeCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return math.NaN()
	}

	// Filter out NaN values
	var validX, validY []float64
	for i := 0; i < len(x); i++ {
		if !math.IsNaN(x[i]) && !math.IsNaN(y[i]) && !math.IsInf(x[i], 0) && !math.IsInf(y[i], 0) {
			validX = append(validX, x[i])
			validY = append(validY, y[i])
		}
	}

	if len(validX) < 2 {
		return math.NaN()
	}

	// Calculate means
	meanX := 0.0
	meanY := 0.0
	for i := 0; i < len(validX); i++ {
		meanX += validX[i]
		meanY += validY[i]
	}
	meanX /= float64(len(validX))
	meanY /= float64(len(validY))

	// Calculate correlation
	numerator := 0.0
	sumXX := 0.0
	sumYY := 0.0

	for i := 0; i < len(validX); i++ {
		dx := validX[i] - meanX
		dy := validY[i] - meanY
		numerator += dx * dy
		sumXX += dx * dx
		sumYY += dy * dy
	}

	denominator := math.Sqrt(sumXX * sumYY)
	if denominator == 0 {
		return math.NaN()
	}

	return numerator / denominator
}
