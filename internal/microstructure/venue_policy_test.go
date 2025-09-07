package microstructure

import (
	"context"
	"testing"
	"time"
)

func TestVenuePolicy(t *testing.T) {
	config := DefaultConfig()
	policy := NewVenuePolicy(config)

	if policy == nil {
		t.Fatal("Failed to create venue policy")
	}

	// Test supported venue check
	if !policy.IsVenueSupported("binance") {
		t.Error("Binance should be supported")
	}

	if policy.IsVenueSupported("dexscreener") {
		t.Error("Aggregators should not be supported")
	}
}

func TestVenueValidation(t *testing.T) {
	config := DefaultConfig()
	policy := NewVenuePolicy(config)

	ctx := context.Background()

	testCases := []struct {
		venue         string
		symbol        string
		shouldApprove bool
		description   string
	}{
		{"binance", "BTCUSD", true, "Supported venue with USD pair"},
		{"okx", "ETHUSD", true, "Supported venue with USD pair"},
		{"coinbase", "BTCUSDT", true, "Supported venue with USDT pair"},
		{"dexscreener", "BTCUSD", false, "Banned aggregator"},
		{"binance", "BTCEUR", false, "Non-USD pair"},
		{"unsupported", "BTCUSD", false, "Unsupported venue"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			result, err := policy.ValidateVenue(ctx, tc.venue, tc.symbol)
			if err != nil {
				t.Fatalf("Validation failed: %v", err)
			}

			if result.Approved != tc.shouldApprove {
				t.Errorf("Expected approved=%t, got approved=%t for %s/%s",
					tc.shouldApprove, result.Approved, tc.venue, tc.symbol)
			}

			if result.Venue != tc.venue {
				t.Errorf("Expected venue %s, got %s", tc.venue, result.Venue)
			}

			// Check that policy violations are recorded for failed cases
			if !tc.shouldApprove && len(result.PolicyViolations) == 0 {
				t.Error("Expected policy violations for failed case")
			}

			// Check that recommendation is set
			if result.Recommendation == "" {
				t.Error("Recommendation should not be empty")
			}
		})
	}
}

func TestUSDPairValidation(t *testing.T) {
	config := DefaultConfig()
	policy := NewVenuePolicy(config)

	testCases := []struct {
		symbol   string
		isUSDPair bool
	}{
		{"BTCUSD", true},
		{"ETHUSD", true},
		{"BTCUSDT", true},
		{"ETHUSDC", true},
		{"btcusd", true},  // Case insensitive
		{"BTCEUR", false},
		{"ETHBTC", false},
		{"ADABNB", false},
		{"", false},
	}

	for _, tc := range testCases {
		result := policy.isUSDPair(tc.symbol)
		if result != tc.isUSDPair {
			t.Errorf("Symbol %s: expected USD pair=%t, got %t",
				tc.symbol, tc.isUSDPair, result)
		}
	}
}

func TestVenueHealthManagement(t *testing.T) {
	config := DefaultConfig()
	policy := NewVenuePolicy(config)

	venue := "binance"

	// Initially no health data
	_, err := policy.GetVenueHealth(venue)
	if err == nil {
		t.Error("Expected error for non-existent venue health")
	}

	// Update health
	health := VenueHealthStatus{
		Healthy:        true,
		RejectRate:     2.5,
		LatencyP99Ms:   1500,
		ErrorRate:      1.2,
		Recommendation: "full_size",
		UptimePercent:  99.8,
	}

	err = policy.UpdateVenueHealth(venue, health)
	if err != nil {
		t.Fatalf("Failed to update venue health: %v", err)
	}

	// Retrieve health
	retrievedHealth, err := policy.GetVenueHealth(venue)
	if err != nil {
		t.Fatalf("Failed to get venue health: %v", err)
	}

	if !retrievedHealth.Healthy {
		t.Error("Health should be healthy")
	}

	if retrievedHealth.RejectRate != 2.5 {
		t.Errorf("Expected reject rate 2.5, got %.1f", retrievedHealth.RejectRate)
	}

	if retrievedHealth.Recommendation != "full_size" {
		t.Errorf("Expected recommendation 'full_size', got %s", retrievedHealth.Recommendation)
	}
}

func TestVenueHealthStaleDetection(t *testing.T) {
	config := DefaultConfig()
	policy := NewVenuePolicy(config)

	venue := "binance"

	// Update health with old timestamp
	health := VenueHealthStatus{
		Healthy:        true,
		RejectRate:     0.0,
		LatencyP99Ms:   1000,
		ErrorRate:      0.0,
		Recommendation: "full_size",
		UptimePercent:  99.9,
		LastUpdate:     time.Now().Add(-10 * time.Minute), // 10 minutes old
	}

	policy.healthMutex.Lock()
	policy.venueHealth[venue] = &health
	policy.healthMutex.Unlock()

	// Health should be marked as unhealthy due to stale data
	retrievedHealth, err := policy.GetVenueHealth(venue)
	if err != nil {
		t.Fatalf("Failed to get venue health: %v", err)
	}

	if retrievedHealth.Healthy {
		t.Error("Stale health data should be marked as unhealthy")
	}

	if retrievedHealth.Recommendation != "avoid" {
		t.Errorf("Stale health should recommend 'avoid', got %s", retrievedHealth.Recommendation)
	}
}

func TestVenueRiskProfile(t *testing.T) {
	config := DefaultConfig()
	policy := NewVenuePolicy(config)

	testCases := []struct {
		venue           string
		expectSuccess   bool
		expectedRisk    float64
		maxPositionSize float64
	}{
		{"binance", true, 0.2, 10000000.0},
		{"coinbase", true, 0.1, 15000000.0},
		{"okx", true, 0.3, 5000000.0},
		{"kraken", true, 0.15, 8000000.0},
		{"unsupported", false, 0.0, 0.0},
	}

	for _, tc := range testCases {
		t.Run(tc.venue, func(t *testing.T) {
			profile, err := policy.GetVenueRiskProfile(tc.venue)

			if tc.expectSuccess {
				if err != nil {
					t.Fatalf("Expected success but got error: %v", err)
				}

				if profile.RiskScore != tc.expectedRisk {
					t.Errorf("Expected risk score %.1f, got %.1f",
						tc.expectedRisk, profile.RiskScore)
				}

				if profile.MaxPositionSize != tc.maxPositionSize {
					t.Errorf("Expected max position size %.0f, got %.0f",
						tc.maxPositionSize, profile.MaxPositionSize)
				}
			} else {
				if err == nil {
					t.Error("Expected error for unsupported venue")
				}
			}
		})
	}
}

func TestPolicyStatus(t *testing.T) {
	config := DefaultConfig()
	policy := NewVenuePolicy(config)

	// Add some venue health data
	policy.UpdateVenueHealth("binance", VenueHealthStatus{
		Healthy:        true,
		Recommendation: "full_size",
	})

	policy.UpdateVenueHealth("okx", VenueHealthStatus{
		Healthy:        false,
		Recommendation: "avoid",
	})

	status := policy.GetPolicyStatus()
	if status == nil {
		t.Fatal("Policy status should not be nil")
	}

	expectedFields := []string{
		"supported_venues",
		"venue_health",
		"aggregator_violations",
		"defi_hooks_enabled",
		"last_health_check",
	}

	for _, field := range expectedFields {
		if _, exists := status[field]; !exists {
			t.Errorf("Policy status missing field: %s", field)
		}
	}

	// Check venue health summary
	venueHealth, ok := status["venue_health"].(map[string]string)
	if !ok {
		t.Error("Venue health should be a map[string]string")
	} else {
		if venueHealth["binance"] != "healthy" {
			t.Error("Binance should be healthy")
		}
		if venueHealth["okx"] != "degraded" {
			t.Error("OKX should be degraded")
		}
	}
}

func TestDeFiHooksToggle(t *testing.T) {
	config := DefaultConfig()
	policy := NewVenuePolicy(config)

	// Initially disabled
	if policy.enableDeFiHooks {
		t.Error("DeFi hooks should be disabled by default")
	}

	// Enable hooks
	policy.EnableDeFiHooks()
	if !policy.enableDeFiHooks {
		t.Error("DeFi hooks should be enabled")
	}

	// Disable hooks
	policy.DisableDeFiHooks()
	if policy.enableDeFiHooks {
		t.Error("DeFi hooks should be disabled")
	}
}

func TestDeFiHooks(t *testing.T) {
	hooks := NewDeFiHooks()
	if hooks == nil {
		t.Fatal("Failed to create DeFi hooks")
	}

	// Initially disabled
	if hooks.enabled {
		t.Error("DeFi hooks should be disabled by default")
	}

	// Test getting metrics while disabled
	ctx := context.Background()
	_, err := hooks.GetLiquidityMetrics(ctx, "BTCUSD")
	if err == nil {
		t.Error("Expected error when DeFi hooks disabled")
	}

	// Enable and test
	hooks.enabled = true
	metrics, err := hooks.GetLiquidityMetrics(ctx, "BTCUSD")
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}

	if metrics == nil {
		t.Fatal("Metrics should not be nil")
	}

	if metrics.OnChainLiquidity <= 0 {
		t.Error("On-chain liquidity should be positive")
	}

	if len(metrics.DEXVenues) == 0 {
		t.Error("Should have DEX venues listed")
	}
}

func TestDeFiLiquidityPools(t *testing.T) {
	hooks := NewDeFiHooks()
	hooks.enabled = true

	ctx := context.Background()

	// Test refresh
	err := hooks.RefreshLiquidityPools(ctx)
	if err != nil {
		t.Fatalf("Failed to refresh liquidity pools: %v", err)
	}

	// Test total liquidity
	totalLiquidity := hooks.GetTotalOnChainLiquidity()
	if totalLiquidity <= 0 {
		t.Error("Total liquidity should be positive")
	}

	// Test stale data detection
	hooks.lastUpdate = time.Now().Add(-10 * time.Minute)
	if !hooks.IsStaleData() {
		t.Error("Data should be detected as stale")
	}

	hooks.lastUpdate = time.Now()
	if hooks.IsStaleData() {
		t.Error("Fresh data should not be stale")
	}
}

func TestRecommendationLogic(t *testing.T) {
	config := DefaultConfig()
	policy := NewVenuePolicy(config)

	testCases := []struct {
		description        string
		approved          bool
		venueRecommendation string
		expectedResult    string
	}{
		{"Not approved", false, "full_size", "reject"},
		{"Approved with full size", true, "full_size", "full_size"},
		{"Approved with halve size", true, "halve_size", "halve_size"},
		{"Approved with avoid", true, "avoid", "reject"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			result := &VenuePolicyResult{
				Approved: tc.approved,
				Health: &VenueHealthStatus{
					Recommendation: tc.venueRecommendation,
				},
			}

			recommendation := policy.determineRecommendation(result)
			if recommendation != tc.expectedResult {
				t.Errorf("Expected recommendation '%s', got '%s'",
					tc.expectedResult, recommendation)
			}
		})
	}
}