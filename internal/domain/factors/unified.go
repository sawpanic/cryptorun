package factors

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// FactorRow represents a single symbol's factor values for orthogonalization
type FactorRow struct {
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`

	// Raw factors (before orthogonalization)
	MomentumCore    float64 `json:"momentum_core"`    // Protected momentum signal
	TechnicalFactor float64 `json:"technical_factor"` // RSI, MACD, etc.
	VolumeFactor    float64 `json:"volume_factor"`    // Volume surge, ADV ratio
	QualityFactor   float64 `json:"quality_factor"`   // Fundamental quality metrics
	SocialFactor    float64 `json:"social_factor"`    // Social sentiment (pre-cap)

	// Orthogonalized residuals (post Gram-Schmidt)
	TechnicalResidual float64 `json:"technical_residual"`
	VolumeResidual    float64 `json:"volume_residual"`
	QualityResidual   float64 `json:"quality_residual"`
	SocialResidual    float64 `json:"social_residual"` // Applied with +10 cap

	// Final composite score
	CompositeScore float64 `json:"composite_score"`
	Rank           int     `json:"rank"`
	Selected       bool    `json:"selected"`
}

// OrthogonalizationOrder defines the protected hierarchy for Gram-Schmidt
type OrthogonalizationOrder struct {
	Protected []string `json:"protected"` // MomentumCore - never residualized
	Sequence  []string `json:"sequence"`  // Order for residualization
}

// Default orthogonalization order per requirements
var DefaultOrthogonalizationOrder = OrthogonalizationOrder{
	Protected: []string{"MomentumCore"},
	Sequence:  []string{"TechnicalFactor", "VolumeFactor", "QualityFactor", "SocialFactor"},
}

// RegimeWeights represents normalized weights that sum to 1.0
type RegimeWeights struct {
	MomentumCore      float64 `yaml:"momentum_core" json:"momentum_core"`
	TechnicalResidual float64 `yaml:"technical_residual" json:"technical_residual"`
	VolumeResidual    float64 `yaml:"volume_residual" json:"volume_residual"`
	QualityResidual   float64 `yaml:"quality_residual" json:"quality_residual"`
	SocialResidual    float64 `yaml:"social_residual" json:"social_residual"`
}

// Sum returns the sum of all weights (should be 1.0)
func (rw RegimeWeights) Sum() float64 {
	return rw.MomentumCore + rw.TechnicalResidual + rw.VolumeResidual +
		rw.QualityResidual + rw.SocialResidual
}

// Validate ensures weights sum to 1.0 within tolerance
func (rw RegimeWeights) Validate(tolerance float64) error {
	sum := rw.Sum()
	if math.Abs(sum-1.0) > tolerance {
		return fmt.Errorf("weights sum to %.6f, expected 1.0 ± %.6f", sum, tolerance)
	}

	// Check non-negative weights
	if rw.MomentumCore < 0 || rw.TechnicalResidual < 0 || rw.VolumeResidual < 0 ||
		rw.QualityResidual < 0 || rw.SocialResidual < 0 {
		return fmt.Errorf("all weights must be non-negative")
	}

	return nil
}

// UnifiedFactorEngine handles the single path for factor processing
type UnifiedFactorEngine struct {
	order   OrthogonalizationOrder
	weights RegimeWeights
	regime  string

	// Configuration constants
	socialHardCap   float64
	weightTolerance float64
}

// NewUnifiedFactorEngine creates the single factor processing engine
func NewUnifiedFactorEngine(regime string, weights RegimeWeights) (*UnifiedFactorEngine, error) {
	// Validate weights sum to 1.0
	tolerance := 0.001
	if err := weights.Validate(tolerance); err != nil {
		return nil, fmt.Errorf("invalid regime weights: %w", err)
	}

	return &UnifiedFactorEngine{
		order:           DefaultOrthogonalizationOrder,
		weights:         weights,
		regime:          regime,
		socialHardCap:   10.0, // +10 hard cap for social
		weightTolerance: tolerance,
	}, nil
}

// ProcessFactors performs orthogonalization and scoring in single unified path
func (ufe *UnifiedFactorEngine) ProcessFactors(factorRows []FactorRow) ([]FactorRow, error) {
	if len(factorRows) == 0 {
		return []FactorRow{}, nil
	}

	// Step 1: Apply orthogonalization with MomentumCore protection
	orthogonalized, err := ufe.applyOrthogonalization(factorRows)
	if err != nil {
		return nil, fmt.Errorf("orthogonalization failed: %w", err)
	}

	// Step 2: Apply social factor cap AFTER orthogonalization
	capped := ufe.applySocialCap(orthogonalized)

	// Step 3: Calculate composite scores with regime weights
	scored := ufe.calculateCompositeScores(capped)

	// Step 4: Rank from highest to lowest score
	ranked := ufe.rankByScore(scored)

	return ranked, nil
}

// applyOrthogonalization performs Gram-Schmidt with MomentumCore protection
func (ufe *UnifiedFactorEngine) applyOrthogonalization(rows []FactorRow) ([]FactorRow, error) {
	n := len(rows)
	if n == 0 {
		return rows, nil
	}

	// Extract factor matrices for orthogonalization
	momentumCore := make([]float64, n)
	technical := make([]float64, n)
	volume := make([]float64, n)
	quality := make([]float64, n)
	social := make([]float64, n)

	for i, row := range rows {
		momentumCore[i] = row.MomentumCore
		technical[i] = row.TechnicalFactor
		volume[i] = row.VolumeFactor
		quality[i] = row.QualityFactor
		social[i] = row.SocialFactor
	}

	// MomentumCore is protected - never residualized
	// Apply Gram-Schmidt to create orthogonal residuals

	// Technical residual = Technical - proj(Technical onto MomentumCore)
	technicalResidual := subtractProjection(technical, momentumCore)

	// Volume residual = Volume - proj(Volume onto MomentumCore) - proj(Volume onto TechnicalResidual)
	volumeResidual := subtractProjection(volume, momentumCore)
	volumeResidual = subtractProjection(volumeResidual, technicalResidual)

	// Quality residual = Quality - projections onto all previous orthogonal factors
	qualityResidual := subtractProjection(quality, momentumCore)
	qualityResidual = subtractProjection(qualityResidual, technicalResidual)
	qualityResidual = subtractProjection(qualityResidual, volumeResidual)

	// Social residual = Social - projections onto all previous orthogonal factors
	socialResidual := subtractProjection(social, momentumCore)
	socialResidual = subtractProjection(socialResidual, technicalResidual)
	socialResidual = subtractProjection(socialResidual, volumeResidual)
	socialResidual = subtractProjection(socialResidual, qualityResidual)

	// Update rows with orthogonalized residuals
	result := make([]FactorRow, n)
	for i, row := range rows {
		result[i] = row
		result[i].TechnicalResidual = technicalResidual[i]
		result[i].VolumeResidual = volumeResidual[i]
		result[i].QualityResidual = qualityResidual[i]
		result[i].SocialResidual = socialResidual[i]
	}

	return result, nil
}

// subtractProjection removes projection of vector a onto vector b: a - proj(a onto b)
func subtractProjection(a, b []float64) []float64 {
	if len(a) != len(b) {
		return a // Return original if dimensions don't match
	}

	// Calculate projection coefficient: (a·b) / (b·b)
	dotProduct := 0.0
	normSquared := 0.0
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normSquared += b[i] * b[i]
	}

	if normSquared == 0 {
		return a // Return original if b is zero vector
	}

	coeff := dotProduct / normSquared

	// Subtract projection: a - coeff * b
	result := make([]float64, len(a))
	for i := 0; i < len(a); i++ {
		result[i] = a[i] - coeff*b[i]
	}

	return result
}

// applySocialCap applies +10 hard cap to social residual (post-orthogonalization)
func (ufe *UnifiedFactorEngine) applySocialCap(rows []FactorRow) []FactorRow {
	result := make([]FactorRow, len(rows))
	for i, row := range rows {
		result[i] = row
		// Apply hard cap: social contribution cannot exceed +10
		if result[i].SocialResidual > ufe.socialHardCap {
			result[i].SocialResidual = ufe.socialHardCap
		}
		if result[i].SocialResidual < -ufe.socialHardCap {
			result[i].SocialResidual = -ufe.socialHardCap
		}
	}
	return result
}

// calculateCompositeScores computes final scores using regime weights
func (ufe *UnifiedFactorEngine) calculateCompositeScores(rows []FactorRow) []FactorRow {
	result := make([]FactorRow, len(rows))
	for i, row := range rows {
		result[i] = row

		// Weighted sum with normalized regime weights (sum = 1.0)
		score := (row.MomentumCore * ufe.weights.MomentumCore) +
			(row.TechnicalResidual * ufe.weights.TechnicalResidual) +
			(row.VolumeResidual * ufe.weights.VolumeResidual) +
			(row.QualityResidual * ufe.weights.QualityResidual) +
			(row.SocialResidual * ufe.weights.SocialResidual)

		result[i].CompositeScore = score
	}
	return result
}

// rankByScore sorts by composite score (highest first) and assigns ranks
func (ufe *UnifiedFactorEngine) rankByScore(rows []FactorRow) []FactorRow {
	// Sort by composite score (descending)
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].CompositeScore > rows[j].CompositeScore
	})

	// Assign ranks
	for i := range rows {
		rows[i].Rank = i + 1
		rows[i].Selected = false // Will be set during Top-N selection
	}

	return rows
}

// GetCorrelationMatrix returns correlation matrix for debugging orthogonality
func (ufe *UnifiedFactorEngine) GetCorrelationMatrix(rows []FactorRow) map[string]map[string]float64 {
	if len(rows) == 0 {
		return make(map[string]map[string]float64)
	}

	factors := map[string][]float64{
		"MomentumCore":      make([]float64, len(rows)),
		"TechnicalResidual": make([]float64, len(rows)),
		"VolumeResidual":    make([]float64, len(rows)),
		"QualityResidual":   make([]float64, len(rows)),
		"SocialResidual":    make([]float64, len(rows)),
	}

	// Extract factor vectors
	for i, row := range rows {
		factors["MomentumCore"][i] = row.MomentumCore
		factors["TechnicalResidual"][i] = row.TechnicalResidual
		factors["VolumeResidual"][i] = row.VolumeResidual
		factors["QualityResidual"][i] = row.QualityResidual
		factors["SocialResidual"][i] = row.SocialResidual
	}

	// Calculate correlation matrix
	correlationMatrix := make(map[string]map[string]float64)
	factorNames := []string{"MomentumCore", "TechnicalResidual", "VolumeResidual", "QualityResidual", "SocialResidual"}

	for _, f1 := range factorNames {
		correlationMatrix[f1] = make(map[string]float64)
		for _, f2 := range factorNames {
			correlationMatrix[f1][f2] = calculateCorrelation(factors[f1], factors[f2])
		}
	}

	return correlationMatrix
}

// calculateCorrelation computes Pearson correlation coefficient
func calculateCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0.0
	}

	n := len(x)

	// Calculate means
	meanX, meanY := 0.0, 0.0
	for i := 0; i < n; i++ {
		meanX += x[i]
		meanY += y[i]
	}
	meanX /= float64(n)
	meanY /= float64(n)

	// Calculate correlation components
	numerator := 0.0
	denomX := 0.0
	denomY := 0.0

	for i := 0; i < n; i++ {
		dx := x[i] - meanX
		dy := y[i] - meanY
		numerator += dx * dy
		denomX += dx * dx
		denomY += dy * dy
	}

	denom := math.Sqrt(denomX * denomY)
	if denom == 0 {
		return 0.0
	}

	return numerator / denom
}

// SetRegime updates regime and validates new weights
func (ufe *UnifiedFactorEngine) SetRegime(regime string, weights RegimeWeights) error {
	if err := weights.Validate(ufe.weightTolerance); err != nil {
		return fmt.Errorf("invalid regime weights for %s: %w", regime, err)
	}

	ufe.regime = regime
	ufe.weights = weights
	return nil
}

// GetCurrentRegime returns current regime
func (ufe *UnifiedFactorEngine) GetCurrentRegime() string {
	return ufe.regime
}

// GetCurrentWeights returns current weights
func (ufe *UnifiedFactorEngine) GetCurrentWeights() RegimeWeights {
	return ufe.weights
}
