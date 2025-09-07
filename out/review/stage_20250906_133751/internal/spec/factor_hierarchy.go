package spec

import (
	"fmt"
	"math"

	"cryptorun/internal/application/pipeline"
)

// FactorHierarchySpec tests factor hierarchy compliance
type FactorHierarchySpec struct{}

// Name returns the section name
func (fhs *FactorHierarchySpec) Name() string {
	return "Factor Hierarchy"
}

// Description returns the section description
func (fhs *FactorHierarchySpec) Description() string {
	return "Momentum protected; residuals orthogonal (|ρ|<0.10)"
}

// RunSpecs executes all factor hierarchy specification tests
func (fhs *FactorHierarchySpec) RunSpecs() []SpecResult {
	var results []SpecResult

	// Test data with deliberate correlation structure
	testSymbols := []string{"BTCUSD", "ETHUSD", "SOLUSD", "ADAUSD", "MATICUSD"}
	testData := fhs.createCorrelatedFactorData(testSymbols)

	// Test 1: Momentum Core Protection
	results = append(results, fhs.testMomentumCoreProtection(testData))

	// Test 2: Orthogonal Residuals
	results = append(results, fhs.testOrthogonalResiduals(testData))

	// Test 3: Factor Validation
	results = append(results, fhs.testFactorValidation())

	// Test 4: Correlation Matrix Computation
	results = append(results, fhs.testCorrelationMatrix(testData))

	return results
}

// testMomentumCoreProtection verifies momentum values are preserved during orthogonalization
func (fhs *FactorHierarchySpec) testMomentumCoreProtection(testData []pipeline.FactorSet) SpecResult {
	spec := NewSpecResult("MomentumCoreProtection", "Momentum values unchanged during orthogonalization")

	// Create orthogonalizer
	ortho := pipeline.NewOrthogonalizer()

	// Store original momentum values
	originalMomentum := make(map[string]float64)
	for _, factors := range testData {
		originalMomentum[factors.Symbol] = factors.MomentumCore
	}

	// Apply orthogonalization
	orthogonalSets, err := ortho.OrthogonalizeFactors(testData)
	if err != nil {
		return NewFailedSpecResult(spec.Name, spec.Description,
			fmt.Sprintf("Orthogonalization failed: %v", err))
	}

	// Verify momentum values are unchanged
	for _, orthoSet := range orthogonalSets {
		originalValue := originalMomentum[orthoSet.Symbol]

		if math.Abs(orthoSet.MomentumCore-originalValue) > 1e-10 {
			return NewFailedSpecResult(spec.Name, spec.Description,
				fmt.Sprintf("Momentum core changed from %.6f to %.6f for symbol %s",
					originalValue, orthoSet.MomentumCore, orthoSet.Symbol)).
				WithDetails("Momentum protection violated")
		}
	}

	return spec.WithDetails(fmt.Sprintf("Protected %d momentum values", len(orthogonalSets)))
}

// testOrthogonalResiduals verifies orthogonalized factors have low correlation
func (fhs *FactorHierarchySpec) testOrthogonalResiduals(testData []pipeline.FactorSet) SpecResult {
	spec := NewSpecResult("OrthogonalResiduals", "Non-momentum factors have |ρ|<0.10 after orthogonalization")

	// Create orthogonalizer
	ortho := pipeline.NewOrthogonalizer()

	// Apply orthogonalization
	orthogonalSets, err := ortho.OrthogonalizeFactors(testData)
	if err != nil {
		return NewFailedSpecResult(spec.Name, spec.Description,
			fmt.Sprintf("Orthogonalization failed: %v", err))
	}

	// Compute correlation matrix for orthogonalized factors
	corrMatrix := ortho.ComputeCorrelationMatrix(orthogonalSets)

	// Check all non-momentum correlations are below threshold
	threshold := 0.10
	factorNames := []string{"volume", "social", "volatility"}
	violationCount := 0
	maxCorrelation := 0.0

	for i, factor1 := range factorNames {
		for j, factor2 := range factorNames {
			if i >= j {
				continue // Skip diagonal and lower triangle
			}

			corr := math.Abs(corrMatrix[factor1][factor2])
			if corr > maxCorrelation {
				maxCorrelation = corr
			}

			if corr > threshold {
				violationCount++
			}
		}
	}

	if violationCount > 0 {
		return NewFailedSpecResult(spec.Name, spec.Description,
			fmt.Sprintf("%d factor pairs exceed |ρ|=%.2f threshold (max: %.3f)",
				violationCount, threshold, maxCorrelation))
	}

	return spec.WithDetails(fmt.Sprintf("Max |ρ|=%.3f < %.2f threshold", maxCorrelation, threshold))
}

// testFactorValidation verifies factor validation logic
func (fhs *FactorHierarchySpec) testFactorValidation() SpecResult {
	spec := NewSpecResult("FactorValidation", "ValidateFactorSet rejects invalid factor combinations")

	// Create valid factor set
	validSet := pipeline.FactorSet{
		Symbol:       "BTCUSD",
		MomentumCore: 12.5,
		Volume:       8.3,
		Social:       3.2,
		Volatility:   18.0,
	}

	// Should pass validation
	if !pipeline.ValidateFactorSet(validSet) {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Valid factor set rejected by validation")
	}

	// Create invalid factor set (NaN momentum)
	invalidSet := pipeline.FactorSet{
		Symbol:       "ETHUSD",
		MomentumCore: math.NaN(),
		Volume:       8.3,
		Social:       3.2,
		Volatility:   18.0,
	}

	// Should fail validation
	if pipeline.ValidateFactorSet(invalidSet) {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Invalid factor set (NaN momentum) accepted by validation")
	}

	// Create factor set with insufficient valid factors
	insufficientSet := pipeline.FactorSet{
		Symbol:       "SOLUSD",
		MomentumCore: 5.2,
		Volume:       math.NaN(),
		Social:       math.Inf(1),
		Volatility:   math.NaN(),
	}

	// Should fail validation (only momentum is valid)
	if pipeline.ValidateFactorSet(insufficientSet) {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Factor set with insufficient valid factors accepted")
	}

	return spec.WithDetails("Valid/invalid factor sets correctly identified")
}

// testCorrelationMatrix verifies correlation matrix computation
func (fhs *FactorHierarchySpec) testCorrelationMatrix(testData []pipeline.FactorSet) SpecResult {
	spec := NewSpecResult("CorrelationMatrix", "Correlation matrix computed correctly for factor analysis")

	// Create orthogonalizer
	ortho := pipeline.NewOrthogonalizer()

	// Compute correlation matrix
	corrMatrix := ortho.ComputeCorrelationMatrix(testData)

	// Verify matrix properties
	expectedFactors := []string{"momentum_core", "volume", "social", "volatility"}

	// Check all expected factors present
	for _, factor := range expectedFactors {
		if _, exists := corrMatrix[factor]; !exists {
			return NewFailedSpecResult(spec.Name, spec.Description,
				fmt.Sprintf("Missing factor '%s' in correlation matrix", factor))
		}
	}

	// Check diagonal elements are 1.0
	for _, factor := range expectedFactors {
		if math.Abs(corrMatrix[factor][factor]-1.0) > 1e-10 {
			return NewFailedSpecResult(spec.Name, spec.Description,
				fmt.Sprintf("Diagonal element for '%s' is %.6f, expected 1.0",
					factor, corrMatrix[factor][factor]))
		}
	}

	// Check matrix is symmetric
	for _, factor1 := range expectedFactors {
		for _, factor2 := range expectedFactors {
			if math.Abs(corrMatrix[factor1][factor2]-corrMatrix[factor2][factor1]) > 1e-10 {
				return NewFailedSpecResult(spec.Name, spec.Description,
					fmt.Sprintf("Matrix not symmetric: [%s][%s]=%.6f != [%s][%s]=%.6f",
						factor1, factor2, corrMatrix[factor1][factor2],
						factor2, factor1, corrMatrix[factor2][factor1]))
			}
		}
	}

	return spec.WithDetails(fmt.Sprintf("4x4 correlation matrix validated"))
}

// createCorrelatedFactorData creates test data with deliberate correlation structure
func (fhs *FactorHierarchySpec) createCorrelatedFactorData(symbols []string) []pipeline.FactorSet {
	// Create factors with known correlation patterns to test orthogonalization
	data := make([]pipeline.FactorSet, 0, len(symbols))

	for i, symbol := range symbols {
		// Create factors with some correlation
		base := float64(i + 1)

		factorSet := pipeline.FactorSet{
			Symbol:       symbol,
			MomentumCore: 10.0 + base*2.0, // Momentum varies independently
			Volume:       5.0 + base*1.5,  // Somewhat correlated with momentum
			Social:       3.0 + base*0.8,  // Partially correlated with volume
			Volatility:   20.0 - base*1.2, // Negatively correlated with momentum
		}

		data = append(data, factorSet)
	}

	return data
}
