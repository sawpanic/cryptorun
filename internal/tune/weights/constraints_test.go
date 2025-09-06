package weights

import (
	"fmt"
	"math"
	"testing"
)

func TestConstraintSystem_ValidateWeights(t *testing.T) {
	cs := NewConstraintSystem()

	tests := []struct {
		name    string
		regime  string
		weights RegimeWeights
		wantErr bool
		errMsg  string
	}{
		{
			name:   "impossible normal regime bounds",
			regime: "normal",
			weights: RegimeWeights{
				MomentumCore:      0.40, // Within [0.40, 0.45] - minimum
				TechnicalResidual: 0.18, // Within [0.18, 0.22] - minimum
				VolumeResidual:    0.30, // S/D total = 0.42, exceeds max (0.22)
				QualityResidual:   0.12, // Above minimum 0.08
			},
			wantErr: true,
			errMsg:  "supply_demand total",
		},
		{
			name:   "momentum below bounds",
			regime: "normal",
			weights: RegimeWeights{
				MomentumCore:      0.35, // Below 0.40
				TechnicalResidual: 0.20,
				VolumeResidual:    0.20,
				QualityResidual:   0.25,
			},
			wantErr: true,
			errMsg:  "momentum_core",
		},
		{
			name:   "momentum above bounds",
			regime: "normal",
			weights: RegimeWeights{
				MomentumCore:      0.50, // Above 0.45 for normal
				TechnicalResidual: 0.20,
				VolumeResidual:    0.15,
				QualityResidual:   0.15,
			},
			wantErr: true,
			errMsg:  "momentum_core",
		},
		{
			name:   "technical below bounds",
			regime: "normal",
			weights: RegimeWeights{
				MomentumCore:      0.42,
				TechnicalResidual: 0.15, // Below 0.18
				VolumeResidual:    0.21,
				QualityResidual:   0.22,
			},
			wantErr: true,
			errMsg:  "technical_residual",
		},
		{
			name:   "supply demand total below bounds",
			regime: "normal",
			weights: RegimeWeights{
				MomentumCore:      0.42,
				TechnicalResidual: 0.20,
				VolumeResidual:    0.05, // Total S/D = 0.15, below 0.20
				QualityResidual:   0.10,
			},
			wantErr: true,
			errMsg:  "supply_demand total",
		},
		{
			name:   "supply demand total above bounds",
			regime: "normal",
			weights: RegimeWeights{
				MomentumCore:      0.42,
				TechnicalResidual: 0.18,
				VolumeResidual:    0.15, // Total S/D = 0.25, above 0.22
				QualityResidual:   0.25,
			},
			wantErr: true,
			errMsg:  "supply_demand total",
		},
		{
			name:   "quality below minimum",
			regime: "normal",
			weights: RegimeWeights{
				MomentumCore:      0.42,
				TechnicalResidual: 0.20,
				VolumeResidual:    0.17,
				QualityResidual:   0.05, // Below minimum 0.08
			},
			wantErr: true,
			errMsg:  "quality_residual",
		},
		{
			name:   "weights don't sum to 1",
			regime: "normal",
			weights: RegimeWeights{
				MomentumCore:      0.42,
				TechnicalResidual: 0.20,
				VolumeResidual:    0.15,
				QualityResidual:   0.20, // Sum = 0.97, not 1.0
			},
			wantErr: true,
			errMsg:  "sum to",
		},
		{
			name:   "unknown regime",
			regime: "unknown",
			weights: RegimeWeights{
				MomentumCore:      0.42,
				TechnicalResidual: 0.20,
				VolumeResidual:    0.15,
				QualityResidual:   0.23,
			},
			wantErr: true,
			errMsg:  "no constraints defined",
		},
		{
			name:   "valid volatile regime weights",
			regime: "volatile",
			weights: RegimeWeights{
				MomentumCore:      0.45, // Within [0.42, 0.48]
				TechnicalResidual: 0.18, // Within [0.15, 0.22]
				VolumeResidual:    0.15, // Combined S/D = 0.25 within [0.22, 0.28]
				QualityResidual:   0.22, // Above minimum 0.06, total = 1.0
			},
			wantErr: false,
		},
		{
			name:   "valid calm regime weights",
			regime: "calm",
			weights: RegimeWeights{
				MomentumCore:      0.47, // Within [0.40, 0.50]
				TechnicalResidual: 0.22, // Within [0.18, 0.25]
				VolumeResidual:    0.12, // Combined S/D = 0.31 within [0.25, 0.35]
				QualityResidual:   0.19, // Above minimum 0.08
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cs.ValidateWeights(tt.regime, tt.weights)

			if tt.wantErr && err == nil {
				t.Errorf("expected error containing '%s', got none", tt.errMsg)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !containsSubstring(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestConstraintSystem_ClampWeights(t *testing.T) {
	cs := NewConstraintSystem()

	tests := []struct {
		name     string
		regime   string
		input    RegimeWeights
		expectOK bool
	}{
		{
			name:   "clamp momentum above bounds",
			regime: "normal",
			input: RegimeWeights{
				MomentumCore:      0.60, // Above 0.45, should be clamped
				TechnicalResidual: 0.20,
				VolumeResidual:    0.10,
				QualityResidual:   0.10,
			},
			expectOK: true,
		},
		{
			name:   "clamp momentum below bounds",
			regime: "normal",
			input: RegimeWeights{
				MomentumCore:      0.30, // Below 0.40, should be clamped
				TechnicalResidual: 0.20,
				VolumeResidual:    0.25,
				QualityResidual:   0.25,
			},
			expectOK: true,
		},
		{
			name:   "clamp technical bounds",
			regime: "normal",
			input: RegimeWeights{
				MomentumCore:      0.42,
				TechnicalResidual: 0.30, // Above 0.22, should be clamped
				VolumeResidual:    0.14,
				QualityResidual:   0.14,
			},
			expectOK: true,
		},
		{
			name:   "clamp and renormalize",
			regime: "normal",
			input: RegimeWeights{
				MomentumCore:      0.80, // Way too high
				TechnicalResidual: 0.10,
				VolumeResidual:    0.05,
				QualityResidual:   0.05,
			},
			expectOK: true,
		},
		{
			name:   "preserve quality minimum",
			regime: "normal",
			input: RegimeWeights{
				MomentumCore:      0.42,
				TechnicalResidual: 0.20,
				VolumeResidual:    0.35,
				QualityResidual:   0.03, // Below minimum, should be raised
			},
			expectOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clamped, err := cs.ClampWeights(tt.regime, tt.input)

			if tt.expectOK && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if !tt.expectOK && err == nil {
				t.Errorf("expected error, got none")
				return
			}

			if tt.expectOK {
				// Verify clamped weights are valid
				if err := cs.ValidateWeights(tt.regime, clamped); err != nil {
					t.Errorf("clamped weights invalid: %v", err)
				}

				// Verify sum is 1.0 (within tolerance)
				total := clamped.MomentumCore + clamped.TechnicalResidual + clamped.VolumeResidual + clamped.QualityResidual
				if math.Abs(total-1.0) > 0.001 {
					t.Errorf("clamped weights don't sum to 1.0: got %f", total)
				}

				// Verify constraints are satisfied
				constraints, _ := cs.GetConstraints(tt.regime)

				if clamped.MomentumCore < constraints.MomentumBounds[0] || clamped.MomentumCore > constraints.MomentumBounds[1] {
					t.Errorf("momentum not within bounds [%f, %f]: got %f", constraints.MomentumBounds[0], constraints.MomentumBounds[1], clamped.MomentumCore)
				}

				if clamped.QualityResidual < constraints.QualityMinimum {
					t.Errorf("quality below minimum %f: got %f", constraints.QualityMinimum, clamped.QualityResidual)
				}
			}
		})
	}
}

func TestConstraintSystem_CalculateSlack(t *testing.T) {
	cs := NewConstraintSystem()

	weights := RegimeWeights{
		MomentumCore:      0.425, // Mid-range for normal (0.40-0.45)
		TechnicalResidual: 0.20,  // Mid-range for normal (0.18-0.22)
		VolumeResidual:    0.10,
		QualityResidual:   0.275, // Total S/D = 0.375, but only 0.21 is allowed max
	}

	// First clamp to valid weights
	clamped, err := cs.ClampWeights("normal", weights)
	if err != nil {
		t.Fatalf("failed to clamp weights: %v", err)
	}

	slack, err := cs.CalculateSlack("normal", clamped)
	if err != nil {
		t.Fatalf("failed to calculate slack: %v", err)
	}

	// Verify slack values are reasonable
	if slack["momentum_core"] < 0 {
		t.Errorf("momentum slack should be non-negative, got %f", slack["momentum_core"])
	}
	if slack["technical_residual"] < 0 {
		t.Errorf("technical slack should be non-negative, got %f", slack["technical_residual"])
	}
	if slack["supply_demand_total"] < 0 {
		t.Errorf("supply_demand slack should be non-negative, got %f", slack["supply_demand_total"])
	}
	if slack["quality_minimum"] < 0 {
		t.Errorf("quality slack should be non-negative, got %f", slack["quality_minimum"])
	}

	t.Logf("Slack values: momentum=%.3f, technical=%.3f, supply_demand=%.3f, quality=%.3f",
		slack["momentum_core"], slack["technical_residual"], slack["supply_demand_total"], slack["quality_minimum"])
}

func TestConstraintSystem_GenerateRandomValidWeights(t *testing.T) {
	cs := NewConstraintSystem()

	tests := []struct {
		name   string
		regime string
		seed   uint64
	}{
		{"normal regime", "normal", 12345},
		{"calm regime", "calm", 54321},
		{"volatile regime", "volatile", 98765},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rng := NewRandGen(tt.seed)
			weights, err := cs.GenerateRandomValidWeights(tt.regime, rng)

			if err != nil {
				t.Fatalf("failed to generate random weights: %v", err)
			}

			// Verify generated weights are valid
			if err := cs.ValidateWeights(tt.regime, weights); err != nil {
				t.Errorf("generated weights invalid: %v", err)
			}

			// Verify deterministic behavior
			rng2 := NewRandGen(tt.seed)
			weights2, err := cs.GenerateRandomValidWeights(tt.regime, rng2)
			if err != nil {
				t.Fatalf("failed to generate second set of weights: %v", err)
			}

			if !weightsEqual(weights, weights2, 1e-10) {
				t.Errorf("random generation not deterministic")
			}

			t.Logf("Generated %s weights: momentum=%.3f, technical=%.3f, volume=%.3f, quality=%.3f",
				tt.regime, weights.MomentumCore, weights.TechnicalResidual, weights.VolumeResidual, weights.QualityResidual)
		})
	}
}

func TestConstraintSystem_GetConstraints(t *testing.T) {
	cs := NewConstraintSystem()

	// Test valid regimes
	regimes := []string{"normal", "calm", "volatile"}
	for _, regime := range regimes {
		constraints, err := cs.GetConstraints(regime)
		if err != nil {
			t.Errorf("failed to get constraints for %s: %v", regime, err)
			continue
		}

		if constraints.Regime != regime {
			t.Errorf("expected regime %s, got %s", regime, constraints.Regime)
		}

		// Verify bounds are sensible
		if constraints.MomentumBounds[0] >= constraints.MomentumBounds[1] {
			t.Errorf("invalid momentum bounds for %s: [%f, %f]", regime, constraints.MomentumBounds[0], constraints.MomentumBounds[1])
		}
		if constraints.TechnicalBounds[0] >= constraints.TechnicalBounds[1] {
			t.Errorf("invalid technical bounds for %s: [%f, %f]", regime, constraints.TechnicalBounds[0], constraints.TechnicalBounds[1])
		}
		if constraints.SupplyDemandBounds[0] >= constraints.SupplyDemandBounds[1] {
			t.Errorf("invalid supply-demand bounds for %s: [%f, %f]", regime, constraints.SupplyDemandBounds[0], constraints.SupplyDemandBounds[1])
		}
		if constraints.QualityMinimum < 0 {
			t.Errorf("invalid quality minimum for %s: %f", regime, constraints.QualityMinimum)
		}
	}

	// Test invalid regime
	_, err := cs.GetConstraints("invalid")
	if err == nil {
		t.Errorf("expected error for invalid regime")
	}
}

func TestConstraintSystem_GetAllRegimes(t *testing.T) {
	cs := NewConstraintSystem()
	regimes := cs.GetAllRegimes()

	expected := []string{"calm", "normal", "volatile"} // Should be sorted
	if len(regimes) != len(expected) {
		t.Errorf("expected %d regimes, got %d", len(expected), len(regimes))
	}

	// Check all expected regimes are present
	regimeMap := make(map[string]bool)
	for _, regime := range regimes {
		regimeMap[regime] = true
	}

	for _, expected := range expected {
		if !regimeMap[expected] {
			t.Errorf("missing expected regime: %s", expected)
		}
	}
}

func TestConstraintSystem_AddConstraints(t *testing.T) {
	cs := NewConstraintSystem()

	// Add custom constraints
	customConstraints := RegimeConstraints{
		Regime:             "custom",
		MomentumBounds:     [2]float64{0.30, 0.35},
		TechnicalBounds:    [2]float64{0.20, 0.25},
		SupplyDemandBounds: [2]float64{0.40, 0.45},
		QualityMinimum:     0.15,
	}

	cs.AddConstraints("custom", customConstraints)

	// Verify constraints were added
	retrieved, err := cs.GetConstraints("custom")
	if err != nil {
		t.Fatalf("failed to retrieve custom constraints: %v", err)
	}

	if retrieved.Regime != "custom" {
		t.Errorf("expected regime 'custom', got '%s'", retrieved.Regime)
	}
	if retrieved.MomentumBounds != customConstraints.MomentumBounds {
		t.Errorf("momentum bounds mismatch")
	}
	if retrieved.QualityMinimum != customConstraints.QualityMinimum {
		t.Errorf("quality minimum mismatch")
	}

	// Test validation with custom constraints
	validWeights := RegimeWeights{
		MomentumCore:      0.32,
		TechnicalResidual: 0.22,
		VolumeResidual:    0.26,
		QualityResidual:   0.20, // S/D total = 0.46, outside bounds [0.40, 0.45]
	}

	// Clamp to make valid
	clampedWeights, err := cs.ClampWeights("custom", validWeights)
	if err != nil {
		t.Fatalf("failed to clamp custom weights: %v", err)
	}

	if err := cs.ValidateWeights("custom", clampedWeights); err != nil {
		t.Errorf("clamped weights rejected with custom constraints: %v", err)
	}
}

func TestRandGen_Deterministic(t *testing.T) {
	seed := uint64(42)

	rng1 := NewRandGen(seed)
	rng2 := NewRandGen(seed)

	// Generate sequences and compare
	for i := 0; i < 100; i++ {
		val1 := rng1.Float64()
		val2 := rng2.Float64()

		if val1 != val2 {
			t.Errorf("RandGen not deterministic at step %d: %f != %f", i, val1, val2)
		}

		if val1 < 0.0 || val1 >= 1.0 {
			t.Errorf("RandGen value out of range [0, 1): %f", val1)
		}
	}
}

func TestRandGen_Distribution(t *testing.T) {
	rng := NewRandGen(123456)
	n := 10000
	sum := 0.0

	for i := 0; i < n; i++ {
		val := rng.Float64()
		sum += val
	}

	mean := sum / float64(n)
	expectedMean := 0.5

	// Mean should be approximately 0.5 for uniform distribution
	if math.Abs(mean-expectedMean) > 0.05 {
		t.Errorf("mean too far from expected: got %f, expected ~%f", mean, expectedMean)
	}

	t.Logf("RandGen mean over %d samples: %f", n, mean)
}

// Test protected MomentumCore behavior
func TestConstraintSystem_MomentumProtection(t *testing.T) {
	cs := NewConstraintSystem()

	// Test that momentum core has strict bounds for each regime
	regimes := []string{"normal", "calm", "volatile"}

	for _, regime := range regimes {
		constraints, _ := cs.GetConstraints(regime)

		// Momentum bounds should be restrictive (not allow full range 0-1)
		if constraints.MomentumBounds[1]-constraints.MomentumBounds[0] > 0.10 {
			t.Errorf("momentum bounds too wide for %s: range %f", regime, constraints.MomentumBounds[1]-constraints.MomentumBounds[0])
		}

		// Momentum should get significant allocation (>40%)
		if constraints.MomentumBounds[0] < 0.40 {
			t.Errorf("momentum minimum too low for %s: %f", regime, constraints.MomentumBounds[0])
		}
	}
}

// Test sum-to-100 enforcement
func TestConstraintSystem_SumTo100Enforcement(t *testing.T) {
	cs := NewConstraintSystem()

	// Test various weight combinations that don't sum to 1.0 but are within bounds
	testCases := []RegimeWeights{
		{0.42, 0.20, 0.15, 0.20},    // Sum = 0.97, all within normal bounds
		{0.43, 0.19, 0.16, 0.23},    // Sum = 1.01, all within normal bounds
		{0.425, 0.195, 0.155, 0.22}, // Sum = 0.995, close to 1.0
		{0.41, 0.21, 0.14, 0.24},    // Sum = 1.00, exactly 1.0
	}

	for i, weights := range testCases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			originalSum := weights.MomentumCore + weights.TechnicalResidual + weights.VolumeResidual + weights.QualityResidual

			// Validation should fail for non-unity sums
			err := cs.ValidateWeights("normal", weights)
			if err == nil && math.Abs(originalSum-1.0) > 0.001 {
				t.Errorf("validation should reject weights that don't sum to 1.0 (sum = %f)", originalSum)
			}

			// Clamping should fix the sum
			clamped, err := cs.ClampWeights("normal", weights)
			if err != nil {
				t.Fatalf("clamping failed: %v", err)
			}

			clampedSum := clamped.MomentumCore + clamped.TechnicalResidual + clamped.VolumeResidual + clamped.QualityResidual
			if math.Abs(clampedSum-1.0) > 0.001 {
				t.Errorf("clamped weights don't sum to 1.0: %f", clampedSum)
			}
		})
	}
}

// Helper functions
func containsSubstring(str, substr string) bool {
	return len(substr) == 0 || len(str) >= len(substr) && str[:len(str)-(len(substr)-1)] != str ||
		func() bool {
			for i := 0; i <= len(str)-len(substr); i++ {
				if str[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}()
}

func weightsEqual(w1, w2 RegimeWeights, tolerance float64) bool {
	return math.Abs(w1.MomentumCore-w2.MomentumCore) <= tolerance &&
		math.Abs(w1.TechnicalResidual-w2.TechnicalResidual) <= tolerance &&
		math.Abs(w1.VolumeResidual-w2.VolumeResidual) <= tolerance &&
		math.Abs(w1.QualityResidual-w2.QualityResidual) <= tolerance
}
