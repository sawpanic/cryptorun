package unit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sawpanic/cryptorun/internal/explain/delta"
)

// TestExplainDeltaRunner tests the core delta analysis functionality
func TestExplainDeltaRunner(t *testing.T) {
	tempDir := t.TempDir()

	config := &delta.Config{
		Universe:     "BTCUSD,ETHUSD",
		BaselinePath: "synthetic",
		OutputDir:    tempDir,
		Progress:     false,
	}

	runner := delta.NewRunner(config)
	require.NotNil(t, runner)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := runner.Run(ctx)
	require.NoError(t, err)
	require.NotNil(t, results)

	// Validate basic structure
	assert.Equal(t, "BTCUSD,ETHUSD", results.Universe)
	assert.NotEmpty(t, results.Regime)
	assert.Greater(t, results.TotalAssets, 0)
	assert.True(t, results.CurrentTimestamp.After(results.BaselineTimestamp))

	// Should have processed both assets
	assert.Len(t, results.Assets, 2)

	// Validate asset deltas
	for _, asset := range results.Assets {
		assert.Contains(t, []string{"BTCUSD", "ETHUSD"}, asset.Symbol)
		assert.Contains(t, []string{"OK", "WARN", "FAIL"}, asset.Status)
		assert.NotEmpty(t, asset.BaselineFactors)
		assert.NotEmpty(t, asset.CurrentFactors)
		assert.NotEmpty(t, asset.Deltas)
		assert.NotEmpty(t, asset.ToleranceCheck)
	}

	// Status counts should add up
	assert.Equal(t, results.TotalAssets, results.FailCount+results.WarnCount+results.OKCount)
}

// TestComparator tests the delta comparison logic
func TestComparator(t *testing.T) {
	comparator := delta.NewComparator()
	require.NotNil(t, comparator)

	// Create synthetic baseline
	baseline := &delta.BaselineSnapshot{
		Timestamp:  time.Now().Add(-24 * time.Hour),
		Universe:   "test",
		Regime:     "bull",
		AssetCount: 2,
		Factors: map[string]*delta.AssetFactors{
			"BTCUSD": {
				Symbol:         "BTCUSD",
				Regime:         "bull",
				MomentumCore:   75.0,
				TechnicalResid: 12.0,
				VolumeResid:    8.0,
				QualityResid:   4.0,
				SocialResid:    2.0,
				CompositeScore: 78.0,
				Gates:          map[string]bool{"freshness": true},
			},
			"ETHUSD": {
				Symbol:         "ETHUSD",
				Regime:         "bull",
				MomentumCore:   68.0,
				TechnicalResid: 15.0,
				VolumeResid:    11.0,
				QualityResid:   3.0,
				SocialResid:    4.0,
				CompositeScore: 72.0,
				Gates:          map[string]bool{"freshness": false},
			},
		},
	}

	// Create current factors with deliberate variations
	current := map[string]*delta.AssetFactors{
		"BTCUSD": {
			Symbol:         "BTCUSD",
			Regime:         "bull",
			MomentumCore:   85.0, // +10 delta (should WARN)
			TechnicalResid: 14.0, // +2 delta (OK)
			VolumeResid:    10.0, // +2 delta (OK)
			QualityResid:   6.0,  // +2 delta (OK)
			SocialResid:    3.0,  // +1 delta (OK)
			CompositeScore: 88.0, // +10 delta (should WARN)
			Gates:          map[string]bool{"freshness": true},
		},
		"ETHUSD": {
			Symbol:         "ETHUSD",
			Regime:         "bull",
			MomentumCore:   85.0, // +17 delta (should FAIL)
			TechnicalResid: 16.0, // +1 delta (OK)
			VolumeResid:    13.0, // +2 delta (OK)
			QualityResid:   4.0,  // +1 delta (OK)
			SocialResid:    5.0,  // +1 delta (OK)
			CompositeScore: 95.0, // +23 delta (should FAIL)
			Gates:          map[string]bool{"freshness": true},
		},
	}

	// Default tolerance config (from runner)
	tolerance := &delta.ToleranceConfig{
		Regimes: map[string]*delta.RegimeTolerance{
			"bull": {
				Name: "bull",
				FactorTolerances: map[string]*delta.FactorTolerance{
					"momentum_core":   {Factor: "momentum_core", WarnAt: 8.0, FailAt: 15.0, Direction: "both"},
					"composite_score": {Factor: "composite_score", WarnAt: 10.0, FailAt: 20.0, Direction: "both"},
				},
			},
		},
	}

	results, err := comparator.Compare(baseline, current, "bull", tolerance)
	require.NoError(t, err)
	require.NotNil(t, results)

	// Validate results
	assert.Equal(t, 2, results.TotalAssets)
	assert.Equal(t, 1, results.FailCount) // ETHUSD should fail
	assert.Equal(t, 1, results.WarnCount) // BTCUSD should warn
	assert.Equal(t, 0, results.OKCount)

	// Check specific asset statuses
	var btcAsset, ethAsset *delta.AssetDelta
	for _, asset := range results.Assets {
		if asset.Symbol == "BTCUSD" {
			btcAsset = asset
		} else if asset.Symbol == "ETHUSD" {
			ethAsset = asset
		}
	}

	require.NotNil(t, btcAsset)
	require.NotNil(t, ethAsset)

	assert.Equal(t, "WARN", btcAsset.Status)
	assert.Equal(t, "FAIL", ethAsset.Status)

	// Check deltas
	assert.Equal(t, 10.0, btcAsset.Deltas["momentum_core"])
	assert.Equal(t, 17.0, ethAsset.Deltas["momentum_core"])

	// Check worst offenders
	assert.Greater(t, len(results.WorstOffenders), 0)
	worstOffender := results.WorstOffenders[0]
	assert.Equal(t, "ETHUSD", worstOffender.Symbol)
	assert.Equal(t, "FAIL", worstOffender.Severity)
}

// TestToleranceCheck tests individual tolerance validation
func TestToleranceCheck(t *testing.T) {
	comparator := delta.NewComparator()

	testCases := []struct {
		name        string
		delta       float64
		tolerance   *delta.FactorTolerance
		expectedOK  bool
		expectedSev string
	}{
		{
			name:  "within tolerance",
			delta: 5.0,
			tolerance: &delta.FactorTolerance{
				Factor:    "test",
				WarnAt:    8.0,
				FailAt:    15.0,
				Direction: "both",
			},
			expectedOK:  true,
			expectedSev: "OK",
		},
		{
			name:  "warning threshold",
			delta: 10.0,
			tolerance: &delta.FactorTolerance{
				Factor:    "test",
				WarnAt:    8.0,
				FailAt:    15.0,
				Direction: "both",
			},
			expectedOK:  false,
			expectedSev: "WARN",
		},
		{
			name:  "failure threshold",
			delta: 20.0,
			tolerance: &delta.FactorTolerance{
				Factor:    "test",
				WarnAt:    8.0,
				FailAt:    15.0,
				Direction: "both",
			},
			expectedOK:  false,
			expectedSev: "FAIL",
		},
		{
			name:  "positive only - negative delta ignored",
			delta: -20.0,
			tolerance: &delta.FactorTolerance{
				Factor:    "test",
				WarnAt:    8.0,
				FailAt:    15.0,
				Direction: "positive",
			},
			expectedOK:  true,
			expectedSev: "OK",
		},
		{
			name:  "negative only - positive delta ignored",
			delta: 20.0,
			tolerance: &delta.FactorTolerance{
				Factor:    "test",
				WarnAt:    8.0,
				FailAt:    15.0,
				Direction: "negative",
			},
			expectedOK:  true,
			expectedSev: "OK",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use reflection to call private method or create a public wrapper
			// For now, we'll test through the public Compare method
			baseline := &delta.BaselineSnapshot{
				Timestamp:  time.Now().Add(-time.Hour),
				Universe:   "test",
				Regime:     "bull",
				AssetCount: 1,
				Factors: map[string]*delta.AssetFactors{
					"TEST": {
						Symbol:         "TEST",
						Regime:         "bull",
						MomentumCore:   50.0,
						TechnicalResid: 0.0,
						VolumeResid:    0.0,
						QualityResid:   0.0,
						SocialResid:    0.0,
						CompositeScore: 50.0,
						Gates:          make(map[string]bool),
					},
				},
			}

			current := map[string]*delta.AssetFactors{
				"TEST": {
					Symbol:         "TEST",
					Regime:         "bull",
					MomentumCore:   50.0 + tc.delta,
					TechnicalResid: 0.0,
					VolumeResid:    0.0,
					QualityResid:   0.0,
					SocialResid:    0.0,
					CompositeScore: 50.0,
					Gates:          make(map[string]bool),
				},
			}

			tolerance := &delta.ToleranceConfig{
				Regimes: map[string]*delta.RegimeTolerance{
					"bull": {
						Name: "bull",
						FactorTolerances: map[string]*delta.FactorTolerance{
							"momentum_core": tc.tolerance,
						},
					},
				},
			}

			results, err := comparator.Compare(baseline, current, "bull", tolerance)
			require.NoError(t, err)
			require.Len(t, results.Assets, 1)

			asset := results.Assets[0]
			check := asset.ToleranceCheck["momentum_core"]
			require.NotNil(t, check)

			assert.Equal(t, tc.expectedSev, check.Severity)
			assert.Equal(t, !tc.expectedOK, check.Exceeded)
		})
	}
}

// TestWriter tests artifact generation
func TestWriter(t *testing.T) {
	tempDir := t.TempDir()
	writer := delta.NewWriter(tempDir)

	// Create sample results
	results := &delta.Results{
		Universe:          "BTCUSD,ETHUSD",
		Regime:            "bull",
		BaselineTimestamp: time.Now().Add(-24 * time.Hour),
		CurrentTimestamp:  time.Now(),
		TotalAssets:       2,
		FailCount:         1,
		WarnCount:         1,
		OKCount:           0,
		Assets: []*delta.AssetDelta{
			{
				Symbol:          "BTCUSD",
				Regime:          "bull",
				Status:          "WARN",
				BaselineFactors: map[string]float64{"momentum_core": 75.0},
				CurrentFactors:  map[string]float64{"momentum_core": 85.0},
				Deltas:          map[string]float64{"momentum_core": 10.0},
				ToleranceCheck: map[string]*delta.ToleranceCheck{
					"momentum_core": {
						Factor:    "momentum_core",
						Delta:     10.0,
						Tolerance: 8.0,
						Exceeded:  true,
						Severity:  "WARN",
					},
				},
			},
			{
				Symbol:          "ETHUSD",
				Regime:          "bull",
				Status:          "FAIL",
				BaselineFactors: map[string]float64{"momentum_core": 68.0},
				CurrentFactors:  map[string]float64{"momentum_core": 85.0},
				Deltas:          map[string]float64{"momentum_core": 17.0},
				ToleranceCheck: map[string]*delta.ToleranceCheck{
					"momentum_core": {
						Factor:    "momentum_core",
						Delta:     17.0,
						Tolerance: 15.0,
						Exceeded:  true,
						Severity:  "FAIL",
					},
				},
				WorstViolation: &delta.WorstOffender{
					Symbol:    "ETHUSD",
					Factor:    "momentum_core",
					Delta:     17.0,
					Tolerance: 15.0,
					Severity:  "FAIL",
					Hint:      "momentum strength increased significantly",
				},
			},
		},
		WorstOffenders: []*delta.WorstOffender{
			{
				Symbol:    "ETHUSD",
				Factor:    "momentum_core",
				Delta:     17.0,
				Tolerance: 15.0,
				Severity:  "FAIL",
				Hint:      "momentum strength increased significantly",
			},
		},
	}

	// Test JSONL writing
	err := writer.WriteJSONL(results)
	require.NoError(t, err)

	jsonlPath := filepath.Join(tempDir, "results.jsonl")
	assert.FileExists(t, jsonlPath)

	jsonlContent, err := os.ReadFile(jsonlPath)
	require.NoError(t, err)
	assert.Contains(t, string(jsonlContent), "explain_delta_header")
	assert.Contains(t, string(jsonlContent), "asset_delta")
	assert.Contains(t, string(jsonlContent), "worst_offenders")

	// Test markdown writing
	err = writer.WriteMarkdown(results)
	require.NoError(t, err)

	mdPath := filepath.Join(tempDir, "summary.md")
	assert.FileExists(t, mdPath)

	mdContent, err := os.ReadFile(mdPath)
	require.NoError(t, err)
	assert.Contains(t, string(mdContent), "# Explain Delta Analysis Report")
	assert.Contains(t, string(mdContent), "## UX MUST — Live Progress & Explainability")
	assert.Contains(t, string(mdContent), "FAIL(1) WARN(1) OK(0)")
	assert.Contains(t, string(mdContent), "BTCUSD")
	assert.Contains(t, string(mdContent), "ETHUSD")

	// Test artifact paths
	artifacts := writer.GetArtifactPaths()
	assert.Equal(t, jsonlPath, artifacts.ResultsJSONL)
	assert.Equal(t, mdPath, artifacts.SummaryMD)
	assert.Equal(t, tempDir, artifacts.OutputDir)
}

// TestSyntheticBaseline tests synthetic baseline generation
func TestSyntheticBaseline(t *testing.T) {
	config := &delta.Config{
		Universe:     "synthetic",
		BaselinePath: "synthetic",
		OutputDir:    t.TempDir(),
		Progress:     false,
	}

	runner := delta.NewRunner(config)

	// This would normally be a private method, but we can test it through Run()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, err := runner.Run(ctx)
	require.NoError(t, err)

	// Should have synthetic data
	assert.Greater(t, results.TotalAssets, 0)
	assert.NotEmpty(t, results.Assets)

	// Check that synthetic assets have expected symbols
	symbols := make(map[string]bool)
	for _, asset := range results.Assets {
		symbols[asset.Symbol] = true
	}

	// Should contain at least some expected symbols from synthetic baseline
	expectedSymbols := []string{"BTCUSD", "ETHUSD", "SOLUSD"}
	for _, expectedSymbol := range expectedSymbols {
		if symbols[expectedSymbol] {
			// Found at least one expected symbol
			break
		}
		if expectedSymbol == expectedSymbols[len(expectedSymbols)-1] {
			// Didn't find any expected symbols
			t.Fatal("No expected symbols found in synthetic baseline")
		}
	}
}
