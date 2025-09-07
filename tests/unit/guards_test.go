package unit

import (
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/application"
	"github.com/sawpanic/cryptorun/internal/domain/guards"
	"github.com/sawpanic/cryptorun/internal/domain/indicators"
)

func TestFatigueGuard(t *testing.T) {
	config := application.GuardsConfig{
		ActiveProfile: "test",
		Profiles: map[string]application.GuardProfile{
			"test": {
				Regimes: map[string]application.RegimeGuardSettings{
					"trending": {
						Fatigue: application.FatigueGuardConfig{
							Threshold24h: 12.0,
							RSI4h:        70,
						},
					},
				},
			},
		},
	}

	sg := guards.NewSafetyGuards(config)

	tests := []struct {
		name           string
		momentum24h    float64
		rsiValue       float64
		expectPassed   bool
		expectWarning  bool
		expectBlocking bool
	}{
		{
			name:           "normal momentum and RSI",
			momentum24h:    8.0,
			rsiValue:       60.0,
			expectPassed:   true,
			expectWarning:  false,
			expectBlocking: false,
		},
		{
			name:           "high momentum only",
			momentum24h:    15.0,
			rsiValue:       60.0,
			expectPassed:   true,
			expectWarning:  true,
			expectBlocking: false,
		},
		{
			name:           "high RSI only",
			momentum24h:    8.0,
			rsiValue:       75.0,
			expectPassed:   true,
			expectWarning:  true,
			expectBlocking: false,
		},
		{
			name:           "both high - should block",
			momentum24h:    15.0,
			rsiValue:       75.0,
			expectPassed:   false,
			expectWarning:  false,
			expectBlocking: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := guards.CandidateData{
				Symbol:      "BTC-USD",
				Momentum24h: tt.momentum24h,
				Indicators: indicators.TechnicalIndicators{
					RSI: indicators.RSIResult{
						Value:   tt.rsiValue,
						IsValid: true,
					},
				},
			}

			results, err := sg.EvaluateAllGuards("trending", candidate)
			if err != nil {
				t.Fatalf("EvaluateAllGuards failed: %v", err)
			}

			// Find fatigue guard result
			var fatigueResult *guards.GuardResult
			for _, result := range results {
				if result.GuardName == "FatigueGuard" {
					fatigueResult = &result
					break
				}
			}

			if fatigueResult == nil {
				t.Fatal("FatigueGuard result not found")
			}

			if fatigueResult.Passed != tt.expectPassed {
				t.Errorf("Expected passed=%v, got %v. Reason: %s", 
					tt.expectPassed, fatigueResult.Passed, fatigueResult.Reason)
			}

			if fatigueResult.IsWarning != tt.expectWarning {
				t.Errorf("Expected warning=%v, got %v", tt.expectWarning, fatigueResult.IsWarning)
			}

			if tt.expectBlocking && fatigueResult.Passed {
				t.Error("Expected blocking failure but guard passed")
			}
		})
	}
}

func TestFreshnessGuard(t *testing.T) {
	config := application.GuardsConfig{
		ActiveProfile: "test",
		Profiles: map[string]application.GuardProfile{
			"test": {
				Regimes: map[string]application.RegimeGuardSettings{
					"trending": {
						Freshness: application.FreshnessGuardConfig{
							MaxBarsAge: 2,
							ATRFactor:  1.2,
						},
					},
				},
			},
		},
	}

	sg := guards.NewSafetyGuards(config)

	tests := []struct {
		name         string
		barsAge      int
		atrProximity float64
		expectPassed bool
	}{
		{
			name:         "fresh signal",
			barsAge:      1,
			atrProximity: 0.8,
			expectPassed: true,
		},
		{
			name:         "stale signal by age",
			barsAge:      5,
			atrProximity: 0.8,
			expectPassed: false,
		},
		{
			name:         "stale signal by price movement",
			barsAge:      1,
			atrProximity: 2.0,
			expectPassed: false,
		},
		{
			name:         "both stale",
			barsAge:      5,
			atrProximity: 2.0,
			expectPassed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := guards.CandidateData{
				Symbol:       "BTC-USD",
				BarsAge:      tt.barsAge,
				ATRProximity: tt.atrProximity,
				CurrentPrice: 50000.0,
				LastATR:      1000.0,
			}

			results, err := sg.EvaluateAllGuards("trending", candidate)
			if err != nil {
				t.Fatalf("EvaluateAllGuards failed: %v", err)
			}

			// Find freshness guard result
			var freshnessResult *guards.GuardResult
			for _, result := range results {
				if result.GuardName == "FreshnessGuard" {
					freshnessResult = &result
					break
				}
			}

			if freshnessResult == nil {
				t.Fatal("FreshnessGuard result not found")
			}

			if freshnessResult.Passed != tt.expectPassed {
				t.Errorf("Expected passed=%v, got %v. Reason: %s", 
					tt.expectPassed, freshnessResult.Passed, freshnessResult.Reason)
			}
		})
	}
}

func TestLateFillGuard(t *testing.T) {
	config := application.GuardsConfig{
		ActiveProfile: "test",
		Profiles: map[string]application.GuardProfile{
			"test": {
				Regimes: map[string]application.RegimeGuardSettings{
					"trending": {
						LateFill: application.LateFillGuardConfig{
							MaxDelaySeconds: 30,
							P99LatencyReq:   400,
							ATRProximity:    1.2,
						},
					},
				},
			},
		},
	}

	sg := guards.NewSafetyGuards(config)

	baseTime := time.Now()

	tests := []struct {
		name          string
		signalTime    time.Time
		executionTime time.Time
		p99LatencyMs  int
		atrProximity  float64
		expectPassed  bool
	}{
		{
			name:          "fast execution",
			signalTime:    baseTime,
			executionTime: baseTime.Add(10 * time.Second),
			p99LatencyMs:  200,
			atrProximity:  0.5,
			expectPassed:  true,
		},
		{
			name:          "slow execution",
			signalTime:    baseTime,
			executionTime: baseTime.Add(60 * time.Second),
			p99LatencyMs:  200,
			atrProximity:  0.5,
			expectPassed:  false,
		},
		{
			name:          "high latency",
			signalTime:    baseTime,
			executionTime: baseTime.Add(10 * time.Second),
			p99LatencyMs:  600,
			atrProximity:  0.5,
			expectPassed:  false,
		},
		{
			name:          "price moved too much",
			signalTime:    baseTime,
			executionTime: baseTime.Add(10 * time.Second),
			p99LatencyMs:  200,
			atrProximity:  2.0,
			expectPassed:  false,
		},
		{
			name:         "pre-execution check",
			signalTime:   baseTime,
			p99LatencyMs: 200,
			atrProximity: 0.5,
			expectPassed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := guards.CandidateData{
				Symbol:        "BTC-USD",
				SignalTime:    tt.signalTime,
				ExecutionTime: tt.executionTime,
				P99LatencyMs:  tt.p99LatencyMs,
				ATRProximity:  tt.atrProximity,
			}

			results, err := sg.EvaluateAllGuards("trending", candidate)
			if err != nil {
				t.Fatalf("EvaluateAllGuards failed: %v", err)
			}

			// Find late fill guard result
			var lateFillResult *guards.GuardResult
			for _, result := range results {
				if result.GuardName == "LateFillGuard" {
					lateFillResult = &result
					break
				}
			}

			if lateFillResult == nil {
				t.Fatal("LateFillGuard result not found")
			}

			if lateFillResult.Passed != tt.expectPassed {
				t.Errorf("Expected passed=%v, got %v. Reason: %s", 
					tt.expectPassed, lateFillResult.Passed, lateFillResult.Reason)
			}
		})
	}
}

func TestGetGuardSummary(t *testing.T) {
	tests := []struct {
		name                 string
		results              []guards.GuardResult
		expectAllPassed      bool
		expectBlockingIssues int
		expectWarnings       int
		expectRecommendation string
	}{
		{
			name: "all passed",
			results: []guards.GuardResult{
				{Passed: true, IsWarning: false, Confidence: 1.0},
				{Passed: true, IsWarning: false, Confidence: 0.9},
			},
			expectAllPassed:      true,
			expectBlockingIssues: 0,
			expectWarnings:       0,
			expectRecommendation: "APPROVE",
		},
		{
			name: "with warnings",
			results: []guards.GuardResult{
				{Passed: true, IsWarning: false, Confidence: 1.0},
				{Passed: true, IsWarning: true, Confidence: 0.8},
			},
			expectAllPassed:      true,
			expectBlockingIssues: 0,
			expectWarnings:       1,
			expectRecommendation: "CAUTION",
		},
		{
			name: "with blocking issues",
			results: []guards.GuardResult{
				{Passed: true, IsWarning: false, Confidence: 1.0},
				{Passed: false, IsWarning: false, Confidence: 0.9},
			},
			expectAllPassed:      false,
			expectBlockingIssues: 1,
			expectWarnings:       0,
			expectRecommendation: "REJECT",
		},
		{
			name: "mixed results",
			results: []guards.GuardResult{
				{Passed: true, IsWarning: false, Confidence: 1.0},
				{Passed: true, IsWarning: true, Confidence: 0.8},
				{Passed: false, IsWarning: false, Confidence: 0.9},
			},
			expectAllPassed:      false,
			expectBlockingIssues: 1,
			expectWarnings:       1,
			expectRecommendation: "REJECT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := guards.GetGuardSummary(tt.results)

			if summary.AllPassed != tt.expectAllPassed {
				t.Errorf("Expected AllPassed=%v, got %v", tt.expectAllPassed, summary.AllPassed)
			}

			if summary.BlockingIssues != tt.expectBlockingIssues {
				t.Errorf("Expected BlockingIssues=%d, got %d", tt.expectBlockingIssues, summary.BlockingIssues)
			}

			if summary.Warnings != tt.expectWarnings {
				t.Errorf("Expected Warnings=%d, got %d", tt.expectWarnings, summary.Warnings)
			}

			if !containsSubstring(summary.Recommendation, tt.expectRecommendation) {
				t.Errorf("Expected recommendation containing '%s', got '%s'", 
					tt.expectRecommendation, summary.Recommendation)
			}

			if summary.TotalGuards != len(tt.results) {
				t.Errorf("Expected TotalGuards=%d, got %d", len(tt.results), summary.TotalGuards)
			}

			if summary.OverallScore < 0 || summary.OverallScore > 100 {
				t.Errorf("OverallScore should be 0-100, got %.1f", summary.OverallScore)
			}
		})
	}
}

func TestIsTradeAllowed(t *testing.T) {
	tests := []struct {
		name     string
		results  []guards.GuardResult
		expected bool
	}{
		{
			name: "all passed",
			results: []guards.GuardResult{
				{Passed: true, IsWarning: false},
				{Passed: true, IsWarning: false},
			},
			expected: true,
		},
		{
			name: "warnings allowed",
			results: []guards.GuardResult{
				{Passed: true, IsWarning: false},
				{Passed: true, IsWarning: true},
			},
			expected: true,
		},
		{
			name: "blocking failure",
			results: []guards.GuardResult{
				{Passed: true, IsWarning: false},
				{Passed: false, IsWarning: false},
			},
			expected: false,
		},
		{
			name:     "empty results",
			results:  []guards.GuardResult{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := guards.IsTradeAllowed(tt.results)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestValidateGuardConfig(t *testing.T) {
	validConfig := application.GuardsConfig{
		ActiveProfile: "test",
		Profiles: map[string]application.GuardProfile{
			"test": {
				Regimes: map[string]application.RegimeGuardSettings{
					"trending": {
						Fatigue: application.FatigueGuardConfig{
							Threshold24h: 12.0,
							RSI4h:        70,
						},
						Freshness: application.FreshnessGuardConfig{
							MaxBarsAge: 2,
							ATRFactor:  1.2,
						},
						LateFill: application.LateFillGuardConfig{
							MaxDelaySeconds: 30,
							P99LatencyReq:   400,
							ATRProximity:    1.2,
						},
					},
				},
			},
		},
	}

	// Valid config should pass
	err := guards.ValidateGuardConfig(validConfig)
	if err != nil {
		t.Errorf("Valid config should pass validation, got: %v", err)
	}

	// Test invalid fatigue threshold
	invalidConfig := validConfig
	invalidConfig.Profiles["test"].Regimes["trending"].Fatigue.Threshold24h = -5.0
	err = guards.ValidateGuardConfig(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid fatigue threshold")
	}

	// Test invalid RSI threshold
	invalidConfig = validConfig
	invalidConfig.Profiles["test"].Regimes["trending"].Fatigue.RSI4h = 150
	err = guards.ValidateGuardConfig(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid RSI threshold")
	}

	// Test invalid max bars age
	invalidConfig = validConfig
	invalidConfig.Profiles["test"].Regimes["trending"].Freshness.MaxBarsAge = -1
	err = guards.ValidateGuardConfig(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid max bars age")
	}

	// Test invalid ATR factor
	invalidConfig = validConfig
	invalidConfig.Profiles["test"].Regimes["trending"].Freshness.ATRFactor = 0.0
	err = guards.ValidateGuardConfig(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid ATR factor")
	}

	// Test invalid max delay
	invalidConfig = validConfig
	invalidConfig.Profiles["test"].Regimes["trending"].LateFill.MaxDelaySeconds = 0
	err = guards.ValidateGuardConfig(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid max delay")
	}

	// Test invalid P99 latency
	invalidConfig = validConfig
	invalidConfig.Profiles["test"].Regimes["trending"].LateFill.P99LatencyReq = 0
	err = guards.ValidateGuardConfig(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid P99 latency requirement")
	}
}