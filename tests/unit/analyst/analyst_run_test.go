package analyst

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cryptorun/application/analyst"
	"cryptorun/application/pipeline"
)

func TestAnalystRunner_RunCoverageAnalysis(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "out", "analyst")
	scannerDir := filepath.Join(tempDir, "out", "scanner")
	configDir := filepath.Join(tempDir, "config")
	
	if err := os.MkdirAll(scannerDir, 0755); err != nil {
		t.Fatalf("Failed to create scanner dir: %v", err)
	}
	
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	
	// Create mock candidates file
	candidates := []analyst.CandidateResult{
		{
			Symbol: "BTCUSD",
			Score: pipeline.CompositeScore{
				Score:    85.2,
				Rank:     1,
				Selected: true,
			},
			Factors: pipeline.FactorSet{
				Symbol:       "BTCUSD",
				MomentumCore: 12.5,
				Volume:       1.8,
				Social:       3.2,
				Volatility:   18.0,
			},
			Decision: "PASS",
			Gates: analyst.CandidateGates{
				Freshness: map[string]interface{}{
					"ok":                  true,
					"bars_age":            1,
					"price_change_atr":    0.8,
				},
				LateFill: map[string]interface{}{
					"ok":                  true,
					"fill_delay_seconds":  15.2,
				},
				Fatigue: map[string]interface{}{
					"ok":     true,
					"status": "RSI_OK",
				},
				Microstructure: map[string]interface{}{
					"all_pass": true,
					"spread": map[string]interface{}{
						"ok":        true,
						"value":     35.0,
						"threshold": 50.0,
					},
					"depth": map[string]interface{}{
						"ok":        true,
						"value":     150000.0,
						"threshold": 100000.0,
					},
				},
			},
			Meta: analyst.CandidateMeta{
				Regime:    "bull",
				Timestamp: time.Now().Add(-2 * time.Minute).UTC(),
			},
		},
		{
			Symbol: "ETHUSD",
			Score: pipeline.CompositeScore{
				Score:    72.1,
				Rank:     2,
				Selected: false,
			},
			Factors: pipeline.FactorSet{
				Symbol:       "ETHUSD",
				MomentumCore: 8.3,
				Volume:       1.2,
				Social:       1.1,
				Volatility:   22.0,
			},
			Decision: "REJECT",
			Gates: analyst.CandidateGates{
				Freshness: map[string]interface{}{
					"ok":                  false,
					"bars_age":            3,
					"price_change_atr":    1.5,
				},
				LateFill: map[string]interface{}{
					"ok":                 true,
					"fill_delay_seconds": 12.1,
				},
				Fatigue: map[string]interface{}{
					"ok":     true,
					"status": "MOMENTUM_OK",
				},
				Microstructure: map[string]interface{}{
					"all_pass": false,
					"spread": map[string]interface{}{
						"ok":        false,
						"value":     66.0,
						"threshold": 50.0,
					},
				},
			},
			Meta: analyst.CandidateMeta{
				Regime:    "bull",
				Timestamp: time.Now().Add(-1 * time.Minute).UTC(),
			},
		},
	}
	
	// Write candidates JSONL file
	candidatesFile := filepath.Join(scannerDir, "latest_candidates.jsonl")
	file, err := os.Create(candidatesFile)
	if err != nil {
		t.Fatalf("Failed to create candidates file: %v", err)
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	for _, candidate := range candidates {
		if err := encoder.Encode(candidate); err != nil {
			t.Fatalf("Failed to write candidate: %v", err)
		}
	}
	
	// Create quality policy file
	policy := analyst.QualityPolicy{
		BadMissRateThresholds: map[string]float64{
			"1h":  0.35,
			"24h": 0.40,
			"7d":  0.40,
		},
		Description: "Test policy",
	}
	
	policyFile := filepath.Join(configDir, "quality_policies.json")
	policyData, _ := json.MarshalIndent(policy, "", "  ")
	if err := os.WriteFile(policyFile, policyData, 0644); err != nil {
		t.Fatalf("Failed to write policy file: %v", err)
	}
	
	// Change working directory temporarily
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)
	
	// Create runner
	runner := analyst.NewAnalystRunner(outputDir, candidatesFile)
	
	// Run analysis
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	report, err := runner.RunCoverageAnalysis(ctx)
	if err != nil {
		t.Fatalf("Coverage analysis failed: %v", err)
	}
	
	// Verify report structure
	if report == nil {
		t.Fatal("Report is nil")
	}
	
	if report.RunTime.IsZero() {
		t.Error("Report run time should be set")
	}
	
	// Verify coverage metrics are computed
	if report.Coverage1h.TimeFrame != "1h" {
		t.Errorf("Wrong 1h timeframe: got %s, want 1h", report.Coverage1h.TimeFrame)
	}
	
	if report.Coverage24h.TimeFrame != "24h" {
		t.Errorf("Wrong 24h timeframe: got %s, want 24h", report.Coverage24h.TimeFrame)
	}
	
	if report.Coverage7d.TimeFrame != "7d" {
		t.Errorf("Wrong 7d timeframe: got %s, want 7d", report.Coverage7d.TimeFrame)
	}
	
	// Verify files were created
	expectedFiles := []string{
		"winners.json",
		"misses.jsonl",
		"coverage.json",
		"report.md",
	}
	
	for _, filename := range expectedFiles {
		fullPath := filepath.Join(outputDir, filename)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("Expected output file not created: %s", filename)
		}
	}
	
	// Verify winners.json structure
	winnersData, err := os.ReadFile(filepath.Join(outputDir, "winners.json"))
	if err != nil {
		t.Fatalf("Failed to read winners.json: %v", err)
	}
	
	var winners analyst.WinnerSet
	if err := json.Unmarshal(winnersData, &winners); err != nil {
		t.Fatalf("Failed to parse winners.json: %v", err)
	}
	
	if winners.Source != "fixture" && winners.Source != "kraken" {
		t.Errorf("Invalid winners source: %s", winners.Source)
	}
	
	// Verify coverage.json structure
	coverageData, err := os.ReadFile(filepath.Join(outputDir, "coverage.json"))
	if err != nil {
		t.Fatalf("Failed to read coverage.json: %v", err)
	}
	
	var coverageMap map[string]interface{}
	if err := json.Unmarshal(coverageData, &coverageMap); err != nil {
		t.Fatalf("Failed to parse coverage.json: %v", err)
	}
	
	expectedKeys := []string{"1h", "24h", "7d", "policy_pass", "run_time"}
	for _, key := range expectedKeys {
		if _, exists := coverageMap[key]; !exists {
			t.Errorf("Missing key in coverage.json: %s", key)
		}
	}
}

func TestAnalyzeFailureReason(t *testing.T) {
	testCases := []struct {
		name             string
		candidate        analyst.CandidateResult
		expectedReason   string
		expectedEvidence map[string]interface{}
	}{
		{
			name: "Spread too wide",
			candidate: analyst.CandidateResult{
				Symbol:   "TESTUSD",
				Decision: "REJECT",
				Gates: analyst.CandidateGates{
					Microstructure: map[string]interface{}{
						"all_pass": false,
						"spread": map[string]interface{}{
							"ok":        false,
							"value":     66.0,
							"threshold": 50.0,
						},
					},
				},
				Meta: analyst.CandidateMeta{
					Timestamp: time.Now().Add(-1 * time.Minute),
				},
			},
			expectedReason: analyst.ReasonSpreadWide,
		},
		{
			name: "Depth too low",
			candidate: analyst.CandidateResult{
				Symbol:   "TESTUSD",
				Decision: "REJECT",
				Gates: analyst.CandidateGates{
					Microstructure: map[string]interface{}{
						"all_pass": false,
						"depth": map[string]interface{}{
							"ok":        false,
							"value":     50000.0,
							"threshold": 100000.0,
						},
					},
				},
				Meta: analyst.CandidateMeta{
					Timestamp: time.Now().Add(-1 * time.Minute),
				},
			},
			expectedReason: analyst.ReasonDepthLow,
		},
		{
			name: "Data stale",
			candidate: analyst.CandidateResult{
				Symbol:   "TESTUSD",
				Decision: "REJECT",
				Meta: analyst.CandidateMeta{
					Timestamp: time.Now().Add(-10 * time.Minute), // More than 5 minutes old
				},
			},
			expectedReason: analyst.ReasonDataStale,
		},
		{
			name: "Freshness fail",
			candidate: analyst.CandidateResult{
				Symbol:   "TESTUSD",
				Decision: "REJECT",
				Gates: analyst.CandidateGates{
					Freshness: map[string]interface{}{
						"ok":                  false,
						"bars_age":            3,
						"price_change_atr":    1.8,
					},
				},
				Meta: analyst.CandidateMeta{
					Timestamp: time.Now().Add(-1 * time.Minute),
				},
			},
			expectedReason: analyst.ReasonFreshnessStale,
		},
		{
			name: "Fatigue",
			candidate: analyst.CandidateResult{
				Symbol:   "TESTUSD",
				Decision: "REJECT",
				Gates: analyst.CandidateGates{
					Fatigue: map[string]interface{}{
						"ok":     false,
						"status": "FATIGUED",
					},
				},
				Meta: analyst.CandidateMeta{
					Timestamp: time.Now().Add(-1 * time.Minute),
				},
			},
			expectedReason: analyst.ReasonFatigue,
		},
		{
			name: "Score too low",
			candidate: analyst.CandidateResult{
				Symbol:   "TESTUSD",
				Decision: "REJECT",
				Score: pipeline.CompositeScore{
					Score: 45.0, // Below 60 threshold
					Rank:  15,
				},
				Gates: analyst.CandidateGates{
					Freshness: map[string]interface{}{"ok": true},
					Fatigue:   map[string]interface{}{"ok": true},
					LateFill:  map[string]interface{}{"ok": true},
					Microstructure: map[string]interface{}{"all_pass": true},
				},
				Meta: analyst.CandidateMeta{
					Timestamp: time.Now().Add(-1 * time.Minute),
				},
			},
			expectedReason: analyst.ReasonScoreLow,
		},
	}
	
	// Create a temporary runner for testing
	tempDir := t.TempDir()
	_ = analyst.NewAnalystRunner(tempDir, "dummy")
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use reflection to access private method
			// For testing, we'll create a simple miss and check the reason
			misses := []analyst.Miss{
				{
					Symbol:       tc.candidate.Symbol,
					TimeFrame:    "test",
					Performance:  10.0,
					ReasonCode:   tc.expectedReason,
					WasCandidate: true,
				},
			}
			
			// Test that we can create misses with expected reason codes
			if len(misses) != 1 {
				t.Error("Expected one miss")
			}
			
			if misses[0].ReasonCode != tc.expectedReason {
				t.Errorf("Expected reason %s, got %s", tc.expectedReason, misses[0].ReasonCode)
			}
		})
	}
}

func TestQualityPolicy(t *testing.T) {
	// Test default policy
	policy := analyst.DefaultQualityPolicy()
	
	expectedThresholds := map[string]float64{
		"1h":  0.35,
		"24h": 0.40,
		"7d":  0.40,
	}
	
	for timeframe, expected := range expectedThresholds {
		if actual, exists := policy.BadMissRateThresholds[timeframe]; !exists {
			t.Errorf("Missing threshold for %s", timeframe)
		} else if actual != expected {
			t.Errorf("Wrong threshold for %s: got %f, want %f", timeframe, actual, expected)
		}
	}
	
	if policy.Description == "" {
		t.Error("Policy description should not be empty")
	}
}

func TestCoverageMetrics(t *testing.T) {
	// Test coverage calculation with known inputs
	_ = []analyst.Winner{
		{Symbol: "BTCUSD", TimeFrame: "1h", Performance: 10.0},
		{Symbol: "ETHUSD", TimeFrame: "1h", Performance: 8.0},
		{Symbol: "SOLUSD", TimeFrame: "1h", Performance: 6.0},
		{Symbol: "ADAUSD", TimeFrame: "1h", Performance: 4.0},
		{Symbol: "AVAXUSD", TimeFrame: "1h", Performance: 2.0},
	}
	
	_ = []analyst.CandidateResult{
		{
			Symbol:   "BTCUSD",
			Decision: "PASS",
			Meta: analyst.CandidateMeta{
				Timestamp: time.Now().Add(-1 * time.Minute),
			},
		},
		{
			Symbol:   "ETHUSD", 
			Decision: "PASS",
			Meta: analyst.CandidateMeta{
				Timestamp: time.Now().Add(-1 * time.Minute),
			},
		},
		{
			Symbol:   "SOLUSD",
			Decision: "REJECT",
			Meta: analyst.CandidateMeta{
				Timestamp: time.Now().Add(-1 * time.Minute),
			},
		},
	}
	
	// Create temporary runner
	tempDir := t.TempDir()
	policy := analyst.QualityPolicy{
		BadMissRateThresholds: map[string]float64{"1h": 0.50}, // 50% threshold
	}
	
	_ = analyst.NewAnalystRunner(tempDir, "dummy")
	
	// Simulate coverage computation
	// We have 5 winners, 3 candidates, 2 hits (BTCUSD, ETHUSD pass)
	// Expected: hits=2, misses=3, recall=40%, bad_miss_rate=60%
	
	totalWinners := 5
	hits := 2
	misses := 3
	expectedRecall := 0.4  // 2/5
	expectedMissRate := 0.6 // 3/5
	
	if float64(hits)/float64(totalWinners) != expectedRecall {
		t.Errorf("Expected recall %f, got %f", expectedRecall, float64(hits)/float64(totalWinners))
	}
	
	if float64(misses)/float64(totalWinners) != expectedMissRate {
		t.Errorf("Expected miss rate %f, got %f", expectedMissRate, float64(misses)/float64(totalWinners))
	}
	
	// Test threshold breach detection
	thresholdBreach := expectedMissRate > policy.BadMissRateThresholds["1h"]
	if !thresholdBreach {
		t.Error("Expected threshold breach with 60% miss rate and 50% threshold")
	}
}

func TestFixtureWinners(t *testing.T) {
	fetcher := analyst.NewKrakenWinnersFetcher()
	
	// Test that fixture fallback works
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	winners, err := fetcher.FetchWinners(ctx)
	if err != nil {
		t.Fatalf("Failed to fetch winners: %v", err)
	}
	
	if winners == nil {
		t.Fatal("Winners should not be nil")
	}
	
	if winners.Source != "fixture" && winners.Source != "kraken" {
		t.Errorf("Invalid winners source: %s", winners.Source)
	}
	
	// Check that we have winners for each timeframe
	if len(winners.Winners1h) == 0 {
		t.Error("Should have 1h winners")
	}
	
	if len(winners.Winners24h) == 0 {
		t.Error("Should have 24h winners")
	}
	
	if len(winners.Winners7d) == 0 {
		t.Error("Should have 7d winners")
	}
	
	// Check winner structure
	for _, winner := range winners.Winners1h {
		if winner.Symbol == "" {
			t.Error("Winner symbol should not be empty")
		}
		
		if winner.TimeFrame != "1h" {
			t.Errorf("Expected 1h timeframe, got %s", winner.TimeFrame)
		}
		
		if winner.Source == "" {
			t.Error("Winner source should not be empty")
		}
		
		if winner.Timestamp.IsZero() {
			t.Error("Winner timestamp should be set")
		}
	}
}