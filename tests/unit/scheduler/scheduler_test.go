package scheduler

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sawpanic/cryptorun/internal/application"
	"github.com/sawpanic/cryptorun/internal/scheduler"
)

// TestPremoveGateCombinations tests the 2-of-3 gate enforcement logic
func TestPremoveGateCombinations(t *testing.T) {
	tests := []struct {
		name               string
		fundingScore       float64
		qualityScore       float64
		volumeScore        float64
		momentumCore       float64
		depthUSD           float64
		requireVolumeConfirm bool
		volumeGateOK       bool
		expectedPass       bool
		expectedGates      []string
		description        string
	}{
		{
			name:         "all_three_gates_pass",
			fundingScore: 2.5,    // funding_divergence pass
			qualityScore: 75,     // supply_squeeze pass (quality > 70)
			volumeScore:  80,     // whale_accumulation pass (volume > 75)
			momentumCore: 75,     // whale_accumulation pass (momentum > 70)
			depthUSD:     70000,  // supply_squeeze pass (depth < 80k)
			expectedPass: true,
			expectedGates: []string{"funding_divergence", "supply_squeeze", "whale_accumulation"},
			description:  "All three gates pass - should generate alert",
		},
		{
			name:         "two_gates_pass_funding_supply",
			fundingScore: 2.2,    // funding_divergence pass
			qualityScore: 72,     // supply_squeeze pass
			volumeScore:  60,     // whale_accumulation fail (volume <= 75)
			momentumCore: 65,     // whale_accumulation fail (momentum <= 70)
			depthUSD:     75000,  // supply_squeeze pass
			expectedPass: true,
			expectedGates: []string{"funding_divergence", "supply_squeeze"},
			description:  "2-of-3 gates pass (funding + supply) - should generate alert",
		},
		{
			name:         "two_gates_pass_funding_whale",
			fundingScore: 2.8,    // funding_divergence pass
			qualityScore: 65,     // supply_squeeze fail (quality <= 70)
			volumeScore:  78,     // whale_accumulation pass
			momentumCore: 72,     // whale_accumulation pass
			depthUSD:     85000,  // supply_squeeze fail (depth >= 80k)
			expectedPass: true,
			expectedGates: []string{"funding_divergence", "whale_accumulation"},
			description:  "2-of-3 gates pass (funding + whale) - should generate alert",
		},
		{
			name:         "only_one_gate_passes",
			fundingScore: 2.1,    // funding_divergence pass
			qualityScore: 65,     // supply_squeeze fail
			volumeScore:  70,     // whale_accumulation fail
			momentumCore: 68,     // whale_accumulation fail
			depthUSD:     85000,  // supply_squeeze fail
			expectedPass: false,
			expectedGates: []string{"funding_divergence"},
			description:  "Only 1 gate passes - should not generate alert",
		},
		{
			name:         "volume_confirm_required_and_passes",
			fundingScore: 2.3,
			qualityScore: 72,
			volumeScore:  78,
			momentumCore: 73,
			depthUSD:     75000,
			requireVolumeConfirm: true,
			volumeGateOK: true,  // Volume gate passes
			expectedPass: true,
			expectedGates: []string{"funding_divergence", "supply_squeeze", "whale_accumulation"},
			description:  "Volume confirmation required and passes - should generate alert",
		},
		{
			name:         "volume_confirm_required_but_fails",
			fundingScore: 2.3,
			qualityScore: 72,
			volumeScore:  78,
			momentumCore: 73,
			depthUSD:     75000,
			requireVolumeConfirm: true,
			volumeGateOK: false, // Volume gate fails
			expectedPass: false,
			expectedGates: []string{"funding_divergence", "supply_squeeze", "whale_accumulation"},
			description:  "Volume confirmation required but fails - should not generate alert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock candidate with test data
			candidate := application.CandidateResult{
				Symbol: "BTCUSD",
				Score:  application.Score{Score: 78.5},
				Factors: application.Factors{
					FundingScore:  tt.fundingScore,
					QualityScore:  tt.qualityScore,
					VolumeScore:   tt.volumeScore,
					MomentumCore:  tt.momentumCore,
				},
				Gates: application.CandidateGates{
					Volume: application.GateResult{OK: tt.volumeGateOK},
					Microstructure: application.MicrostructureGates{
						DepthUSD: tt.depthUSD,
						OK:       true,
					},
				},
			}

			// Create scheduler and test gate filtering
			sched := &scheduler.Scheduler{}
			requiredGates := []string{"funding_divergence", "supply_squeeze", "whale_accumulation"}
			minGates := 2

			alerts := sched.FilterCandidatesByPremoveGates(
				[]application.CandidateResult{candidate}, 
				requiredGates, 
				minGates, 
				tt.requireVolumeConfirm,
			)

			if tt.expectedPass {
				require.Len(t, alerts, 1, "Expected 1 alert to be generated")
				
				alert := alerts[0]
				assert.Equal(t, candidate.Symbol, alert.Symbol)
				assert.Equal(t, candidate.Score.Score, alert.TotalScore)
				assert.ElementsMatch(t, tt.expectedGates, alert.GatesPassed, "Gates passed mismatch")
				
				if tt.requireVolumeConfirm {
					assert.Equal(t, tt.volumeGateOK, alert.VolumeConfirmed)
				}
				
				t.Logf("✓ %s: Alert generated with gates %v", tt.description, alert.GatesPassed)
			} else {
				assert.Len(t, alerts, 0, "Expected no alerts to be generated")
				t.Logf("✓ %s: No alert generated (gates: %v)", tt.description, tt.expectedGates)
			}
		})
	}
}

// TestProviderHealthFallback tests provider degradation and fallback logic
func TestProviderHealthFallback(t *testing.T) {
	tests := []struct {
		name             string
		provider         string
		healthy          bool
		usagePercent     float64
		circuitState     string
		expectedFallback string
		expectedTTL      int
		description      string
	}{
		{
			name:         "healthy_provider_no_fallback",
			provider:     "binance",
			healthy:      true,
			usagePercent: 50.0,
			circuitState: "CLOSED",
			expectedFallback: "",
			expectedTTL:  300, // Normal TTL
			description:  "Healthy provider should not trigger fallback",
		},
		{
			name:         "unhealthy_provider_triggers_fallback",
			provider:     "okx",
			healthy:      false,
			usagePercent: 100.0,
			circuitState: "OPEN",
			expectedFallback: "coinbase", // OKX falls back to coinbase
			expectedTTL:  600, // Doubled TTL due to degradation
			description:  "Unhealthy provider should trigger fallback",
		},
		{
			name:         "high_usage_doubles_ttl",
			provider:     "kraken",
			healthy:      true,
			usagePercent: 85.0, // > 80% usage
			circuitState: "CLOSED",
			expectedFallback: "",
			expectedTTL:  600, // Doubled TTL due to high usage
			description:  "High usage should double cache TTL",
		},
		{
			name:         "circuit_breaker_open_triggers_fallback",
			provider:     "binance",
			healthy:      true,
			usagePercent: 60.0,
			circuitState: "OPEN",
			expectedFallback: "okx", // Binance falls back to okx
			expectedTTL:  300, // Usage not high, so normal TTL
			description:  "Open circuit breaker should trigger fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock provider health result
			result := scheduler.ProviderHealthResult{
				Provider:     tt.provider,
				Healthy:      tt.healthy,
				LastCheck:    time.Now(),
				ResponseTime: 150,
				RateLimit: scheduler.ProviderRateLimit{
					Used:  800,
					Limit: 1000,
					Usage: tt.usagePercent,
				},
				CircuitState: tt.circuitState,
				ErrorRate:    0.05,
				CacheTTL:     300, // Initial TTL
			}

			results := []scheduler.ProviderHealthResult{result}
			
			// Create scheduler and apply fallback logic
			sched := &scheduler.Scheduler{}
			sched.ApplyProviderFallbacks(results)
			sched.AdjustCacheTTLs(results)

			// Check fallback assignment
			if tt.expectedFallback != "" {
				assert.Equal(t, tt.expectedFallback, results[0].Fallback, 
					"Fallback provider mismatch")
				t.Logf("✓ %s: Fallback to %s applied", tt.description, results[0].Fallback)
			} else {
				assert.Empty(t, results[0].Fallback, "No fallback should be applied")
				t.Logf("✓ %s: No fallback applied", tt.description)
			}

			// Check TTL adjustment
			assert.Equal(t, tt.expectedTTL, results[0].CacheTTL, "Cache TTL mismatch")
			
			if tt.expectedTTL > 300 {
				t.Logf("✓ %s: Cache TTL doubled to %d seconds", tt.description, results[0].CacheTTL)
			} else {
				t.Logf("✓ %s: Cache TTL remains at %d seconds", tt.description, results[0].CacheTTL)
			}
		})
	}
}

// TestRegimeMajorityVote tests the regime detection majority vote logic
func TestRegimeMajorityVote(t *testing.T) {
	tests := []struct {
		name           string
		realizedVol7d  float64
		pctAbove20MA   float64
		breadthThrust  float64
		expectedRegime string
		description    string
	}{
		{
			name:           "calm_regime_all_indicators",
			realizedVol7d:  0.20,  // calm vote (< 0.25)
			pctAbove20MA:   75.0,  // calm vote (> 70)
			breadthThrust:  0.65,  // calm vote (> 0.6)
			expectedRegime: "calm",
			description:    "All indicators vote calm - should result in calm regime",
		},
		{
			name:           "volatile_regime_all_indicators",
			realizedVol7d:  0.50,  // volatile vote (> 0.45)
			pctAbove20MA:   40.0,  // volatile vote (< 45)
			breadthThrust:  0.25,  // volatile vote (< 0.3)
			expectedRegime: "volatile",
			description:    "All indicators vote volatile - should result in volatile regime",
		},
		{
			name:           "normal_regime_majority",
			realizedVol7d:  0.35,  // normal vote (0.25-0.45)
			pctAbove20MA:   60.0,  // normal vote (45-70)
			breadthThrust:  0.25,  // volatile vote (< 0.3)
			expectedRegime: "normal",
			description:    "2-of-3 indicators vote normal - should result in normal regime",
		},
		{
			name:           "mixed_votes_calm_wins",
			realizedVol7d:  0.22,  // calm vote
			pctAbove20MA:   72.0,  // calm vote
			breadthThrust:  0.25,  // volatile vote
			expectedRegime: "calm",
			description:    "2-of-3 indicators vote calm - should result in calm regime",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create regime data
			regimeData := scheduler.RegimeData{
				RealizedVol7d: tt.realizedVol7d,
				PctAbove20MA:  tt.pctAbove20MA,
				BreadthThrust: tt.breadthThrust,
				Confidence:    0.85,
				Timestamp:     time.Now(),
			}

			// Create scheduler and perform majority vote
			sched := &scheduler.Scheduler{}
			regime := sched.PerformRegimeMajorityVote(regimeData)

			assert.Equal(t, tt.expectedRegime, regime, "Regime detection mismatch")
			
			t.Logf("✓ %s: Detected regime '%s'", tt.description, regime)
			t.Logf("  - Realized Vol 7d: %.2f", tt.realizedVol7d)
			t.Logf("  - %% Above 20MA: %.1f", tt.pctAbove20MA)
			t.Logf("  - Breadth Thrust: %.2f", tt.breadthThrust)
		})
	}
}

// TestJobScheduleConfiguration tests job configuration parsing and validation
func TestJobScheduleConfiguration(t *testing.T) {
	// Create temporary config file for testing
	configPath := filepath.Join(t.TempDir(), "test_scheduler.yaml")
	
	configContent := `
global:
  artifacts_dir: "test_artifacts"
  log_level: "info"
  timezone: "UTC"

jobs:
  - name: "scan.hot"
    schedule: "*/15 * * * *"
    type: "scan.hot"
    description: "Test hot scan"
    enabled: true
    config:
      universe: "top30"
      venues: ["kraken", "okx"]
      max_sample: 30
      ttl: 300
      top_n: 10
      premove: true
      regime_aware: true
      
  - name: "premove.hourly"
    schedule: "0 * * * *"
    type: "premove.hourly"
    description: "Test premove sweep"
    enabled: true
    config:
      universe: "top50"
      venues: ["kraken"]
      require_gates: ["funding_divergence", "supply_squeeze"]
      min_gates_passed: 2
      volume_confirm: true
`

	// Write test config file
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	// Test scheduler initialization
	sched, err := scheduler.NewScheduler(configPath)
	require.NoError(t, err, "Failed to initialize scheduler")

	// Test job listing
	jobs, err := sched.ListJobs()
	require.NoError(t, err, "Failed to list jobs")
	require.Len(t, jobs, 2, "Expected 2 jobs to be loaded")

	// Verify hot scan job configuration
	hotScanJob := jobs[0]
	assert.Equal(t, "scan.hot", hotScanJob.Name)
	assert.Equal(t, "*/15 * * * *", hotScanJob.Schedule)
	assert.True(t, hotScanJob.Enabled)
	assert.Equal(t, "top30", hotScanJob.Config.Universe)
	assert.True(t, hotScanJob.Config.Premove)
	assert.True(t, hotScanJob.Config.RegimeAware)

	// Verify premove job configuration
	premoveJob := jobs[1]
	assert.Equal(t, "premove.hourly", premoveJob.Name)
	assert.Equal(t, "0 * * * *", premoveJob.Schedule)
	assert.Equal(t, 2, premoveJob.Config.MinGatesPassed)
	assert.True(t, premoveJob.Config.VolumeConfirm)
	assert.ElementsMatch(t, []string{"funding_divergence", "supply_squeeze"}, premoveJob.Config.RequireGates)

	t.Logf("✓ Successfully loaded and validated scheduler configuration with %d jobs", len(jobs))
}