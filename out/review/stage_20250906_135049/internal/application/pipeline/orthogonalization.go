package pipeline

import (
	"math"

	"cryptorun/internal/domain"
	"github.com/rs/zerolog/log"
)

// FactorSet represents a complete set of factors for orthogonalization
type FactorSet struct {
	Symbol         string               `json:"symbol"`
	MomentumCore   float64              `json:"momentum_core"`   // Protected base momentum
	Volume         float64              `json:"volume"`          // Volume factor
	Social         float64              `json:"social"`          // Social/brand factor (capped at +10)
	Volatility     float64              `json:"volatility"`      // Volatility factor
	Raw            map[string]float64   `json:"raw_factors"`     // All raw factors before orthogonalization
	Orthogonal     map[string]float64   `json:"orthogonal"`      // Orthogonalized factors
	Meta           OrthogonalMeta       `json:"meta"`
}

type OrthogonalMeta struct {
	ProtectedFactors []string `json:"protected_factors"`
	CorrelationMatrix map[string]map[string]float64 `json:"correlation_matrix,omitempty"`
	OrthogonalityCheck float64 `json:"orthogonality_check"`
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
			fs.Volume,
			fs.Social,
			fs.Volatility,
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
			Volume:       orthogonalMatrix[i][1], // Orthogonalized volume
			Social:       orthogonalMatrix[i][2], // Orthogonalized social
			Volatility:   orthogonalMatrix[i][3], // Orthogonalized volatility
			Raw:          originalSet.Raw,
			Orthogonal:   make(map[string]float64),
			Meta: OrthogonalMeta{
				ProtectedFactors: o.protectedFactors,
			},
		}

		// Store orthogonalized factors in map
		orthogonalSet.Orthogonal["momentum_core"] = orthogonalSet.MomentumCore
		orthogonalSet.Orthogonal["volume"] = orthogonalSet.Volume
		orthogonalSet.Orthogonal["social"] = orthogonalSet.Social
		orthogonalSet.Orthogonal["volatility"] = orthogonalSet.Volatility

		// Compute orthogonality check
		orthogonalSet.Meta.OrthogonalityCheck = o.computeOrthogonalityCheck(orthogonalMatrix[i])

		orthogonalFactorSets[i] = orthogonalSet
	}

	log.Info().Float64("avg_orthogonality", o.averageOrthogonalityCheck(orthogonalFactorSets)).
		Msg("Factor orthogonalization completed")

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

// averageOrthogonalityCheck computes average orthogonality across all factors
func (o *Orthogonalizer) averageOrthogonalityCheck(factorSets []FactorSet) float64 {
	if len(factorSets) == 0 {
		return 0.0
	}

	total := 0.0
	count := 0
	
	for _, fs := range factorSets {
		if !math.IsNaN(fs.Meta.OrthogonalityCheck) {
			total += fs.Meta.OrthogonalityCheck
			count++
		}
	}
	
	if count == 0 {
		return 0.0
	}
	
	return total / float64(count)
}

// ApplySocialCap enforces the +10 maximum social factor contribution
func (o *Orthogonalizer) ApplySocialCap(factorSets []FactorSet) []FactorSet {
	const maxSocialContribution = 10.0
	cappedCount := 0

	for i := range factorSets {
		originalSocial := factorSets[i].Social
		
		// Cap social factor at +10
		if factorSets[i].Social > maxSocialContribution {
			factorSets[i].Social = maxSocialContribution
			// Initialize Orthogonal map if nil
			if factorSets[i].Orthogonal == nil {
				factorSets[i].Orthogonal = make(map[string]float64)
			}
			factorSets[i].Orthogonal["social"] = maxSocialContribution
			cappedCount++
		}
		
		// Store original before capping for analysis
		if factorSets[i].Raw == nil {
			factorSets[i].Raw = make(map[string]float64)
		}
		factorSets[i].Raw["social_before_cap"] = originalSocial
	}

	if cappedCount > 0 {
		log.Info().Int("capped_symbols", cappedCount).
			Float64("max_social", maxSocialContribution).
			Msg("Applied social factor cap")
	}

	return factorSets
}

// BuildFactorSet constructs a FactorSet from momentum and other inputs
func BuildFactorSet(symbol string, momentum *MomentumFactors, volumeFactor, socialFactor, volatilityFactor float64) FactorSet {
	// Calculate momentum core from regime-weighted momentum
	momentumCalc := NewMomentumCalculator(nil) // We only need weights, not data provider
	momentumCore := momentumCalc.ApplyRegimeWeights(momentum)

	factorSet := FactorSet{
		Symbol:       symbol,
		MomentumCore: momentumCore,
		Volume:       volumeFactor,
		Social:       socialFactor,
		Volatility:   volatilityFactor,
		Raw: map[string]float64{
			"momentum_core": momentumCore,
			"volume":        volumeFactor,
			"social":        socialFactor,
			"volatility":    volatilityFactor,
			"momentum_1h":   momentum.Momentum1h,
			"momentum_4h":   momentum.Momentum4h,
			"momentum_12h":  momentum.Momentum12h,
			"momentum_24h":  momentum.Momentum24h,
			"momentum_7d":   momentum.Momentum7d,
			"volume_1h":     momentum.Volume1h,
			"volume_4h":     momentum.Volume4h,
			"volume_24h":    momentum.Volume24h,
			"rsi_4h":        momentum.RSI4h,
			"atr_1h":        momentum.ATR1h,
		},
		Orthogonal: make(map[string]float64),
		Meta: OrthogonalMeta{
			ProtectedFactors: []string{"momentum_core"},
		},
	}

	return factorSet
}

// ValidateFactorSet checks if a factor set has valid (non-NaN) core factors
func ValidateFactorSet(fs FactorSet) bool {
	// Check core factors for validity
	coreFactors := []float64{fs.MomentumCore, fs.Volume, fs.Social, fs.Volatility}
	
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
	factorNames := []string{"momentum_core", "volume", "social", "volatility"}
	
	// Extract factor vectors
	factorVectors := make(map[string][]float64)
	for _, name := range factorNames {
		factorVectors[name] = make([]float64, len(factorSets))
	}
	
	for i, fs := range factorSets {
		factorVectors["momentum_core"][i] = fs.MomentumCore
		factorVectors["volume"][i] = fs.Volume
		factorVectors["social"][i] = fs.Social
		factorVectors["volatility"][i] = fs.Volatility
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