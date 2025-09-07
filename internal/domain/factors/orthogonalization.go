package factors

import (
	"fmt"
	"math"

	"github.com/sawpanic/cryptorun/internal/config/regime"
)

// GramSchmidtOrthogonalizer implements the Gram-Schmidt orthogonalization process
type GramSchmidtOrthogonalizer struct {
	config regime.WeightsConfig
}

// NewGramSchmidtOrthogonalizer creates a new orthogonalizer
func NewGramSchmidtOrthogonalizer(config regime.WeightsConfig) *GramSchmidtOrthogonalizer {
	return &GramSchmidtOrthogonalizer{config: config}
}

// OrthogonalizeBatch applies Gram-Schmidt orthogonalization to a batch of factor rows
func (gso *GramSchmidtOrthogonalizer) OrthogonalizeBatch(rawRows []RawFactorRow) ([]OrthogonalizedFactorRow, error) {
	if len(rawRows) == 0 {
		return []OrthogonalizedFactorRow{}, nil
	}

	// Extract factor matrices for batch processing
	factorMatrix := gso.extractFactorMatrix(rawRows)
	
	// Apply Gram-Schmidt orthogonalization
	orthogonalMatrix, orthogonalizationInfo, err := gso.applyGramSchmidt(factorMatrix)
	if err != nil {
		return nil, fmt.Errorf("Gram-Schmidt orthogonalization failed: %w", err)
	}

	// Convert back to structured rows
	orthogonalizedRows := make([]OrthogonalizedFactorRow, len(rawRows))
	for i, rawRow := range rawRows {
		orthogonalizedRows[i] = gso.buildOrthogonalizedRow(rawRow, orthogonalMatrix[i], orthogonalizationInfo)
	}

	return orthogonalizedRows, nil
}

// extractFactorMatrix extracts factor values into a matrix for processing
func (gso *GramSchmidtOrthogonalizer) extractFactorMatrix(rows []RawFactorRow) [][]float64 {
	matrix := make([][]float64, len(rows))
	
	for i, row := range rows {
		// Order according to orthogonalization sequence
		// MomentumCore is always first (protected)
		matrix[i] = []float64{
			row.MomentumCore,    // Index 0: Protected, never orthogonalized
			row.TechnicalFactor, // Index 1: Orthogonalized vs MomentumCore
			row.VolumeFactor,    // Index 2: Orthogonalized vs Momentum + Technical
			row.QualityFactor,   // Index 3: Orthogonalized vs all previous
			row.SocialFactor,    // Index 4: Orthogonalized vs all previous
		}
	}
	
	return matrix
}

// applyGramSchmidt performs the core Gram-Schmidt orthogonalization
func (gso *GramSchmidtOrthogonalizer) applyGramSchmidt(matrix [][]float64) ([][]float64, OrthogonalizationInfo, error) {
	if len(matrix) == 0 || len(matrix[0]) != 5 {
		return nil, OrthogonalizationInfo{}, fmt.Errorf("invalid matrix dimensions")
	}

	numRows := len(matrix)
	numFactors := 5
	
	// Initialize result matrix
	result := make([][]float64, numRows)
	for i := range result {
		result[i] = make([]float64, numFactors)
	}

	// Initialize orthogonalization info
	info := OrthogonalizationInfo{
		CorrelationMatrix:    make(map[string]map[string]float64),
		ProjectionMagnitudes: make(map[string]float64),
		ResidualizationOrder: []string{"TechnicalFactor", "VolumeFactor", "QualityFactor", "SocialFactor"},
	}

	factorNames := []string{"MomentumCore", "TechnicalFactor", "VolumeFactor", "QualityFactor", "SocialFactor"}

	// Step 1: Copy MomentumCore unchanged (protected factor)
	for i := 0; i < numRows; i++ {
		result[i][0] = matrix[i][0] // MomentumCore preserved
	}

	// Step 2: Orthogonalize each subsequent factor against all previous factors
	for factorIdx := 1; factorIdx < numFactors; factorIdx++ {
		currentFactorName := factorNames[factorIdx]
		
		// Extract current factor vector
		currentVector := make([]float64, numRows)
		for i := 0; i < numRows; i++ {
			currentVector[i] = matrix[i][factorIdx]
		}

		// Start with the original vector
		orthogonalVector := make([]float64, numRows)
		copy(orthogonalVector, currentVector)

		// Project out components of all previous factors
		totalProjectionMagnitude := 0.0
		
		for prevFactorIdx := 0; prevFactorIdx < factorIdx; prevFactorIdx++ {
			prevFactorName := factorNames[prevFactorIdx]
			
			// Extract previous factor vector (already orthogonalized)
			prevVector := make([]float64, numRows)
			for i := 0; i < numRows; i++ {
				prevVector[i] = result[i][prevFactorIdx]
			}

			// Calculate projection
			projection := gso.projectVector(currentVector, prevVector)
			projectionMagnitude := gso.vectorNorm(projection)
			totalProjectionMagnitude += projectionMagnitude

			// Subtract projection from current vector
			for i := 0; i < numRows; i++ {
				orthogonalVector[i] -= projection[i]
			}

			// Calculate and store correlation
			correlation := gso.calculateCorrelation(currentVector, prevVector)
			if info.CorrelationMatrix[currentFactorName] == nil {
				info.CorrelationMatrix[currentFactorName] = make(map[string]float64)
			}
			info.CorrelationMatrix[currentFactorName][prevFactorName] = correlation
		}

		// Store projection magnitude for debugging
		info.ProjectionMagnitudes[currentFactorName] = totalProjectionMagnitude

		// Store orthogonalized vector in result
		for i := 0; i < numRows; i++ {
			result[i][factorIdx] = orthogonalVector[i]
		}
	}

	// Step 3: Apply social cap to the social factor (index 4)
	socialCapValue := gso.config.Validation.SocialHardCap
	for i := 0; i < numRows; i++ {
		socialResidual := result[i][4]
		
		// Apply hard cap: clamp to [-socialCapValue, +socialCapValue]
		if socialResidual > socialCapValue {
			result[i][4] = socialCapValue
		} else if socialResidual < -socialCapValue {
			result[i][4] = -socialCapValue
		}
	}

	// Step 4: Calculate quality metrics
	info.QualityMetrics = gso.calculateQualityMetrics(matrix, result, factorNames)

	return result, info, nil
}

// projectVector calculates the projection of vector a onto vector b
func (gso *GramSchmidtOrthogonalizer) projectVector(a, b []float64) []float64 {
	if len(a) != len(b) {
		return nil
	}

	dotProduct := gso.dotProduct(a, b)
	bNormSquared := gso.dotProduct(b, b)

	if bNormSquared == 0 {
		return make([]float64, len(a)) // Zero vector if b is zero
	}

	scalar := dotProduct / bNormSquared
	projection := make([]float64, len(a))
	
	for i := range projection {
		projection[i] = scalar * b[i]
	}

	return projection
}

// dotProduct calculates the dot product of two vectors
func (gso *GramSchmidtOrthogonalizer) dotProduct(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	sum := 0.0
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

// vectorNorm calculates the Euclidean norm of a vector
func (gso *GramSchmidtOrthogonalizer) vectorNorm(v []float64) float64 {
	sumSquares := 0.0
	for _, val := range v {
		sumSquares += val * val
	}
	return math.Sqrt(sumSquares)
}

// calculateCorrelation calculates Pearson correlation coefficient
func (gso *GramSchmidtOrthogonalizer) calculateCorrelation(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}

	// Calculate means
	meanA := 0.0
	meanB := 0.0
	n := float64(len(a))
	
	for i := range a {
		meanA += a[i]
		meanB += b[i]
	}
	meanA /= n
	meanB /= n

	// Calculate covariance and standard deviations
	covariance := 0.0
	varA := 0.0
	varB := 0.0
	
	for i := range a {
		diffA := a[i] - meanA
		diffB := b[i] - meanB
		
		covariance += diffA * diffB
		varA += diffA * diffA
		varB += diffB * diffB
	}

	// Calculate correlation coefficient
	if varA == 0 || varB == 0 {
		return 0.0
	}

	correlation := covariance / math.Sqrt(varA*varB)
	
	// Clamp to [-1, 1] to handle numerical precision issues
	if correlation > 1.0 {
		correlation = 1.0
	} else if correlation < -1.0 {
		correlation = -1.0
	}

	return correlation
}

// calculateQualityMetrics assesses the quality of orthogonalization
func (gso *GramSchmidtOrthogonalizer) calculateQualityMetrics(original, orthogonal [][]float64, factorNames []string) QualityMetrics {
	metrics := QualityMetrics{}

	if len(original) == 0 || len(orthogonal) == 0 {
		return metrics
	}

	numRows := len(original)
	numFactors := len(factorNames)

	// Calculate max correlation between orthogonalized factors
	maxCorr := 0.0
	
	for i := 0; i < numFactors; i++ {
		for j := i + 1; j < numFactors; j++ {
			vectorI := make([]float64, numRows)
			vectorJ := make([]float64, numRows)
			
			for k := 0; k < numRows; k++ {
				vectorI[k] = orthogonal[k][i]
				vectorJ[k] = orthogonal[k][j]
			}
			
			corr := math.Abs(gso.calculateCorrelation(vectorI, vectorJ))
			if corr > maxCorr {
				maxCorr = corr
			}
		}
	}
	metrics.MaxCorrelation = maxCorr

	// Calculate momentum preservation (MomentumCore should be unchanged)
	momentumOriginal := make([]float64, numRows)
	momentumOrthogonal := make([]float64, numRows)
	
	for i := 0; i < numRows; i++ {
		momentumOriginal[i] = original[i][0]
		momentumOrthogonal[i] = orthogonal[i][0]
	}
	
	momentumCorr := gso.calculateCorrelation(momentumOriginal, momentumOrthogonal)
	metrics.MomentumPreserved = momentumCorr * 100.0 // Should be 100%

	// Calculate total variance kept (simplified)
	originalVariance := gso.calculateTotalVariance(original)
	orthogonalVariance := gso.calculateTotalVariance(orthogonal)
	
	if originalVariance > 0 {
		metrics.TotalVarianceKept = (orthogonalVariance / originalVariance) * 100.0
	}

	// Calculate overall orthogonality score (100 = perfect orthogonality)
	metrics.OrthogonalityScore = (1.0 - maxCorr) * 100.0

	return metrics
}

// calculateTotalVariance calculates the total variance of a factor matrix
func (gso *GramSchmidtOrthogonalizer) calculateTotalVariance(matrix [][]float64) float64 {
	if len(matrix) == 0 || len(matrix[0]) == 0 {
		return 0.0
	}

	totalVariance := 0.0
	numFactors := len(matrix[0])

	for factorIdx := 0; factorIdx < numFactors; factorIdx++ {
		// Extract factor vector
		factor := make([]float64, len(matrix))
		for i := 0; i < len(matrix); i++ {
			factor[i] = matrix[i][factorIdx]
		}

		// Calculate variance
		mean := 0.0
		for _, val := range factor {
			mean += val
		}
		mean /= float64(len(factor))

		variance := 0.0
		for _, val := range factor {
			diff := val - mean
			variance += diff * diff
		}
		variance /= float64(len(factor) - 1)

		totalVariance += variance
	}

	return totalVariance
}

// buildOrthogonalizedRow constructs an orthogonalized factor row from the matrix results
func (gso *GramSchmidtOrthogonalizer) buildOrthogonalizedRow(rawRow RawFactorRow, orthogonalFactors []float64, info OrthogonalizationInfo) OrthogonalizedFactorRow {
	// Apply social capping
	socialResidual := orthogonalFactors[4]
	socialCapped := socialResidual
	
	// The social cap was already applied in applyGramSchmidt, but we track it here
	socialCapValue := gso.config.Validation.SocialHardCap
	if math.Abs(socialResidual) > socialCapValue {
		if socialResidual > 0 {
			socialCapped = socialCapValue
		} else {
			socialCapped = -socialCapValue
		}
	}

	return OrthogonalizedFactorRow{
		Symbol:              rawRow.Symbol,
		MomentumCore:        orthogonalFactors[0], // Should be unchanged
		TechnicalResidual:   orthogonalFactors[1],
		VolumeResidual:      orthogonalFactors[2],
		QualityResidual:     orthogonalFactors[3],
		SocialResidual:      socialResidual,
		SocialCapped:        socialCapped,
		Timestamp:           rawRow.Timestamp,
		OrthogonalizationInfo: info,
	}
}

// ValidateOrthogonalization checks if orthogonalization meets quality requirements
func ValidateOrthogonalization(row OrthogonalizedFactorRow, config regime.WeightsConfig) error {
	// Check that MomentumCore is preserved (should be same as original)
	if math.IsNaN(row.MomentumCore) || math.IsInf(row.MomentumCore, 0) {
		return fmt.Errorf("MomentumCore corrupted during orthogonalization")
	}

	// Check correlation threshold
	maxCorr := row.OrthogonalizationInfo.QualityMetrics.MaxCorrelation
	if maxCorr > config.QARequirements.CorrelationThreshold {
		return fmt.Errorf("orthogonalization quality insufficient: max correlation %.3f > threshold %.3f", 
			maxCorr, config.QARequirements.CorrelationThreshold)
	}

	// Check momentum preservation
	momentumPreserved := row.OrthogonalizationInfo.QualityMetrics.MomentumPreserved
	if momentumPreserved < 95.0 { // Should be very close to 100%
		return fmt.Errorf("momentum core not properly preserved: %.1f%% < 95%%", momentumPreserved)
	}

	// Check social cap enforcement
	socialCapValue := config.Validation.SocialHardCap
	if math.Abs(row.SocialCapped) > socialCapValue+0.001 { // Small tolerance for floating point
		return fmt.Errorf("social cap not enforced: |%.3f| > %.1f", row.SocialCapped, socialCapValue)
	}

	// Check for NaN/Inf in residuals
	residuals := []float64{row.TechnicalResidual, row.VolumeResidual, row.QualityResidual, row.SocialResidual}
	residualNames := []string{"TechnicalResidual", "VolumeResidual", "QualityResidual", "SocialResidual"}

	for i, residual := range residuals {
		if math.IsNaN(residual) {
			return fmt.Errorf("%s is NaN after orthogonalization", residualNames[i])
		}
		if math.IsInf(residual, 0) {
			return fmt.Errorf("%s is infinite after orthogonalization", residualNames[i])
		}
	}

	return nil
}