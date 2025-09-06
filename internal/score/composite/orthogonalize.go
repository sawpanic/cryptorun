package composite

import (
	"fmt"
	"math"
)

// Orthogonalizer implements Gram-Schmidt orthogonalization with MomentumCore protection
type Orthogonalizer struct{}

// NewOrthogonalizer creates a new orthogonalizer
func NewOrthogonalizer() *Orthogonalizer {
	return &Orthogonalizer{}
}

// Factor represents a factor vector for orthogonalization
type Factor struct {
	Name      string    `json:"name"`
	Values    []float64 `json:"values"`
	Protected bool      `json:"protected"` // If true, cannot be modified during orthogonalization
}

// OrthogonalizedFactors holds the results of Gram-Schmidt orthogonalization
type OrthogonalizedFactors struct {
	MomentumCore   Factor `json:"momentum_core"`
	TechnicalResid Factor `json:"technical_resid"`
	VolumeResid    Factor `json:"volume_resid"`
	QualityResid   Factor `json:"quality_resid"`
}

// Orthogonalize applies Gram-Schmidt orthogonalization with MomentumCore protection
// Order: MomentumCore (protected) → Technical → Volume → Quality
func (o *Orthogonalizer) Orthogonalize(factors []Factor) (*OrthogonalizedFactors, error) {
	if len(factors) != 4 {
		return nil, fmt.Errorf("expected 4 factors, got %d", len(factors))
	}

	// Ensure first factor is momentum_core and protected
	if factors[0].Name != "momentum_core" || !factors[0].Protected {
		return nil, fmt.Errorf("first factor must be protected momentum_core")
	}

	// Step 1: MomentumCore remains unchanged (protected)
	momentumCore := factors[0]

	// Step 2: Technical → Technical - proj(Technical onto MomentumCore)
	technical := factors[1]
	technicalResid := o.subtractProjection(technical, momentumCore)

	// Step 3: Volume → Volume - proj(Volume onto MomentumCore) - proj(Volume onto TechnicalResid)
	volume := factors[2]
	volumeMinusMomentum := o.subtractProjection(volume, momentumCore)
	volumeResid := o.subtractProjection(volumeMinusMomentum, technicalResid)

	// Step 4: Quality → Quality - all previous projections
	quality := factors[3]
	qualityMinusMomentum := o.subtractProjection(quality, momentumCore)
	qualityMinusTechnical := o.subtractProjection(qualityMinusMomentum, technicalResid)
	qualityResid := o.subtractProjection(qualityMinusTechnical, volumeResid)

	return &OrthogonalizedFactors{
		MomentumCore:   momentumCore,
		TechnicalResid: technicalResid,
		VolumeResid:    volumeResid,
		QualityResid:   qualityResid,
	}, nil
}

// subtractProjection computes u - proj_v(u) where proj_v(u) = (u·v / v·v) * v
func (o *Orthogonalizer) subtractProjection(u, v Factor) Factor {
	// Calculate dot products
	uDotV := o.dotProduct(u.Values, v.Values)
	vDotV := o.dotProduct(v.Values, v.Values)

	// Handle zero vector case
	if math.Abs(vDotV) < 1e-10 {
		return u // Return original if v is effectively zero
	}

	// Calculate projection coefficient
	projCoeff := uDotV / vDotV

	// Subtract projection: u - (uDotV / vDotV) * v
	residual := make([]float64, len(u.Values))
	for i := range u.Values {
		if i < len(v.Values) {
			residual[i] = u.Values[i] - projCoeff*v.Values[i]
		} else {
			residual[i] = u.Values[i] // Keep original if v doesn't have this dimension
		}
	}

	return Factor{
		Name:      u.Name + "_residual",
		Values:    residual,
		Protected: false,
	}
}

// dotProduct computes the dot product of two vectors
func (o *Orthogonalizer) dotProduct(a, b []float64) float64 {
	// Handle mismatched lengths by using minimum length
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	if minLen == 0 {
		return 0
	}

	sum := 0.0
	for i := 0; i < minLen; i++ {
		sum += a[i] * b[i]
	}
	return sum
}

// ValidateOrthogonality checks that the resulting factors are orthogonal
func (o *Orthogonalizer) ValidateOrthogonality(factors *OrthogonalizedFactors, tolerance float64) error {
	// Extract all factors for pairwise checks
	allFactors := []Factor{
		factors.MomentumCore,
		factors.TechnicalResid,
		factors.VolumeResid,
		factors.QualityResid,
	}

	// Check pairwise orthogonality
	for i := 0; i < len(allFactors); i++ {
		for j := i + 1; j < len(allFactors); j++ {
			dotProd := math.Abs(o.dotProduct(allFactors[i].Values, allFactors[j].Values))
			if dotProd > tolerance {
				return fmt.Errorf("factors %s and %s not orthogonal: dot product = %.6f > %.6f",
					allFactors[i].Name, allFactors[j].Name, dotProd, tolerance)
			}
		}
	}

	return nil
}

// GetOrthogonalityMatrix returns the dot product matrix for diagnostic purposes
func (o *Orthogonalizer) GetOrthogonalityMatrix(factors *OrthogonalizedFactors) [][]float64 {
	allFactors := []Factor{
		factors.MomentumCore,
		factors.TechnicalResid,
		factors.VolumeResid,
		factors.QualityResid,
	}

	n := len(allFactors)
	matrix := make([][]float64, n)
	for i := range matrix {
		matrix[i] = make([]float64, n)
	}

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			matrix[i][j] = o.dotProduct(allFactors[i].Values, allFactors[j].Values)
		}
	}

	return matrix
}

// ComputeResidualMagnitudes returns the magnitude of each residual component
func (o *Orthogonalizer) ComputeResidualMagnitudes(factors *OrthogonalizedFactors) map[string]float64 {
	magnitudes := make(map[string]float64)

	allFactors := []Factor{
		factors.MomentumCore,
		factors.TechnicalResid,
		factors.VolumeResid,
		factors.QualityResid,
	}

	for _, factor := range allFactors {
		magnitude := math.Sqrt(o.dotProduct(factor.Values, factor.Values))
		magnitudes[factor.Name] = magnitude
	}

	return magnitudes
}
