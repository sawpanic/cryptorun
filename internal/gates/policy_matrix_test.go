package gates

import (
	"context"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/microstructure"
)

func TestPolicyMatrix(t *testing.T) {
	config := DefaultPolicyMatrixConfig()
	pm := NewPolicyMatrix(config)

	if pm == nil {
		t.Fatal("Failed to create policy matrix")
	}

	// Test default configuration
	if !pm.config.VenueFallbackEnabled {
		t.Error("Venue fallback should be enabled by default")
	}

	if !pm.config.DepegGuardEnabled {
		t.Error("Depeg guard should be enabled by default")
	}

	if !pm.config.RiskOffTogglesEnabled {
		t.Error("Risk-off toggles should be enabled by default")
	}
}

func TestPolicyEvaluation_AllChecksPass(t *testing.T) {
	config := DefaultPolicyMatrixConfig()
	pm := NewPolicyMatrix(config)

	ctx := context.Background()

	// Test with healthy venue
	result, err := pm.EvaluatePolicy(ctx, "BTCUSD", "kraken")
	if err != nil {
		t.Fatalf("Policy evaluation failed: %v", err)
	}

	if !result.PolicyPassed {
		t.Errorf("Policy should pass with healthy venue, got violations: %v", result.PolicyViolations)
	}

	if result.RecommendedAction != "proceed" {
		t.Errorf("Expected recommended action 'proceed', got '%s'", result.RecommendedAction)
	}

	if result.ConfidenceScore < 0.8 {
		t.Errorf("Expected high confidence score (>0.8), got %.3f", result.ConfidenceScore)
	}
}

func TestVenueFallback(t *testing.T) {
	config := DefaultPolicyMatrixConfig()
	config.MinHealthyVenues = 2
	pm := NewPolicyMatrix(config)

	// Mark primary venue as unhealthy
	unhealthyStatus := &microstructure.VenueHealthStatus{
		Healthy:        false,
		RejectRate:     15.0, // Above threshold
		LatencyP99Ms:   3000,  // Above threshold
		ErrorRate:      8.0,   // Above threshold
		Recommendation: "avoid",
		UptimePercent:  85.0, // Below threshold
	}

	err := pm.UpdateVenueHealth("kraken", unhealthyStatus)
	if err != nil {
		t.Fatalf("Failed to update venue health: %v", err)
	}

	ctx := context.Background()
	result, err := pm.EvaluatePolicy(ctx, "BTCUSD", "kraken")
	if err != nil {
		t.Fatalf("Policy evaluation failed: %v", err)
	}

	// Should attempt fallback
	if result.VenueHealthCheck == nil {
		t.Fatal("Venue health check should not be nil")
	}

	if !result.VenueHealthCheck.FallbackRequired {
		t.Error("Fallback should be required for unhealthy venue")
	}

	if result.FallbacksAttempted == 0 {
		t.Error("Should have attempted venue fallback")
	}
}

func TestDepegGuard(t *testing.T) {
	config := DefaultPolicyMatrixConfig()
	config.DepegThresholdBps = 50.0 // 0.5% threshold for testing
	pm := NewPolicyMatrix(config)

	// Add a depeg alert manually for testing
	pm.depegAlerts["USDT"] = DepegAlert{
		Stablecoin:        "USDT",
		CurrentPrice:      0.995, // 0.5% depeg
		DepegBps:          50.0,
		Timestamp:         time.Now(),
		AlertLevel:        "warning",
		RecommendedAction: "monitor",
	}

	ctx := context.Background()
	result, err := pm.EvaluatePolicy(ctx, "BTCUSDT", "kraken")
	if err != nil {
		t.Fatalf("Policy evaluation failed: %v", err)
	}

	if result.DepegCheck == nil {
		t.Fatal("Depeg check should not be nil for stablecoin pair")
	}

	if !result.DepegCheck.DepegDetected {
		t.Error("Depeg should be detected for USDT pair")
	}

	if len(result.DepegCheck.AffectedStablecoins) == 0 {
		t.Error("Should list affected stablecoins")
	}

	if result.DepegCheck.MaxDepegBps != 50.0 {
		t.Errorf("Expected max depeg 50.0 bps, got %.1f", result.DepegCheck.MaxDepegBps)
	}
}

func TestRiskOffMode(t *testing.T) {
	config := DefaultPolicyMatrixConfig()
	pm := NewPolicyMatrix(config)

	// Manually set risk-off state for testing
	pm.riskOffDetector.currentState = RiskOffState{
		Active:         true,
		TriggerReasons: []string{"VIX spike: 35.0 > 30.0", "BTC drop: -18.0% < -15.0%"},
		Confidence:     0.7,
		Severity:       "high",
		Timestamp:      time.Now(),
	}

	ctx := context.Background()
	result, err := pm.EvaluatePolicy(ctx, "BTCUSD", "kraken")
	if err != nil {
		t.Fatalf("Policy evaluation failed: %v", err)
	}

	if result.RiskOffCheck == nil {
		t.Fatal("Risk-off check should not be nil")
	}

	if !result.RiskOffCheck.RiskOffActive {
		t.Error("Risk-off mode should be active")
	}

	if result.RiskOffCheck.Severity != "high" {
		t.Errorf("Expected high severity, got '%s'", result.RiskOffCheck.Severity)
	}

	if result.RiskOffCheck.RecommendedAction != "halt" {
		t.Errorf("Expected recommended action 'halt' for high severity, got '%s'", result.RiskOffCheck.RecommendedAction)
	}

	if result.PolicyPassed {
		t.Error("Policy should not pass during active risk-off mode")
	}
}

func TestVenueHealthUpdates(t *testing.T) {
	config := DefaultPolicyMatrixConfig()
	pm := NewPolicyMatrix(config)

	venue := "binance"

	// Test healthy status update
	healthyStatus := &microstructure.VenueHealthStatus{
		Healthy:        true,
		RejectRate:     2.0,
		LatencyP99Ms:   1500,
		ErrorRate:      1.0,
		Recommendation: "full_size",
		UptimePercent:  99.5,
	}

	err := pm.UpdateVenueHealth(venue, healthyStatus)
	if err != nil {
		t.Fatalf("Failed to update venue health: %v", err)
	}

	// Verify status was updated
	pm.matrixMutex.RLock()
	status, exists := pm.activeVenues[venue]
	pm.matrixMutex.RUnlock()

	if !exists {
		t.Fatal("Venue status should exist after update")
	}

	if !status.Healthy {
		t.Error("Venue should be healthy")
	}

	if status.ConsecutiveFailures != 0 {
		t.Errorf("Expected 0 consecutive failures, got %d", status.ConsecutiveFailures)
	}

	// Test unhealthy status update
	unhealthyStatus := &microstructure.VenueHealthStatus{
		Healthy:        false,
		RejectRate:     12.0,
		LatencyP99Ms:   5000,
		ErrorRate:      7.0,
		Recommendation: "avoid",
		UptimePercent:  80.0,
	}

	err = pm.UpdateVenueHealth(venue, unhealthyStatus)
	if err != nil {
		t.Fatalf("Failed to update unhealthy venue status: %v", err)
	}

	// Verify unhealthy status
	pm.matrixMutex.RLock()
	status = pm.activeVenues[venue]
	pm.matrixMutex.RUnlock()

	if status.Healthy {
		t.Error("Venue should be unhealthy")
	}

	if status.ConsecutiveFailures != 1 {
		t.Errorf("Expected 1 consecutive failure, got %d", status.ConsecutiveFailures)
	}
}

func TestVenueRanking(t *testing.T) {
	config := DefaultPolicyMatrixConfig()
	pm := NewPolicyMatrix(config)

	// Test default venue ranking
	if len(pm.venueRanking) == 0 {
		t.Error("Should have default venue ranking")
	}

	// Verify Kraken is highest priority (lowest priority number)
	krakenFound := false
	lowestPriority := 999
	for _, rank := range pm.venueRanking {
		if rank.Venue == "kraken" {
			krakenFound = true
			if rank.Priority > lowestPriority {
				t.Error("Kraken should have highest priority (lowest number)")
			}
		}
		if rank.Priority < lowestPriority {
			lowestPriority = rank.Priority
		}
	}

	if !krakenFound {
		t.Error("Kraken should be in venue ranking")
	}

	// Test venue fallback order
	ctx := context.Background()

	// Mark kraken as unhealthy
	unhealthyStatus := &microstructure.VenueHealthStatus{
		Healthy:        false,
		Recommendation: "avoid",
	}
	pm.UpdateVenueHealth("kraken", unhealthyStatus)

	fallbackVenue, attempts, err := pm.attemptVenueFallback(ctx, "kraken")
	if err != nil {
		t.Fatalf("Venue fallback failed: %v", err)
	}

	if fallbackVenue == "" {
		t.Error("Should find a fallback venue")
	}

	if fallbackVenue == "kraken" {
		t.Error("Fallback should not be the same as failed venue")
	}

	if attempts == 0 {
		t.Error("Should have attempted at least one fallback")
	}

	// Verify fallback venue is in primary venues list
	isValidFallback := false
	for _, primaryVenue := range config.PrimaryVenues {
		if primaryVenue == fallbackVenue {
			isValidFallback = true
			break
		}
	}
	if !isValidFallback {
		t.Errorf("Fallback venue '%s' should be in primary venues list", fallbackVenue)
	}
}

func TestDepegMonitor(t *testing.T) {
	config := DefaultPolicyMatrixConfig()
	monitor := NewDepegMonitor(config)

	if monitor == nil {
		t.Fatal("Failed to create depeg monitor")
	}

	ctx := context.Background()

	// Test price updates
	err := monitor.UpdatePrices(ctx)
	if err != nil {
		t.Fatalf("Failed to update prices: %v", err)
	}

	// Check that prices were updated
	if len(monitor.monitoredCoins) == 0 {
		t.Error("Should have monitored coin prices after update")
	}

	// Verify expected stablecoins are monitored
	expectedCoins := []string{"USDT", "USDC", "DAI"}
	for _, coin := range expectedCoins {
		if _, exists := monitor.monitoredCoins[coin]; !exists {
			t.Errorf("Should monitor %s price", coin)
		}
	}

	// Check for reasonable prices (should be close to $1.00)
	for coin, price := range monitor.monitoredCoins {
		if price < 0.99 || price > 1.01 {
			t.Errorf("%s price %.4f is outside reasonable range [0.99, 1.01]", coin, price)
		}
	}
}

func TestRiskOffDetector(t *testing.T) {
	thresholds := &RiskOffThresholds{
		VIXSpike:              30.0,
		BTCDrop24h:            -15.0,
		StablecoinVolumeSpike: 3.0,
		FundingRateExtreme:    0.1,
	}

	detector := NewRiskOffDetector(thresholds)
	if detector == nil {
		t.Fatal("Failed to create risk-off detector")
	}

	ctx := context.Background()

	// Test market data update
	err := detector.UpdateMarketData(ctx)
	if err != nil {
		t.Fatalf("Failed to update market data: %v", err)
	}

	// Get current state
	state := detector.GetCurrentState()

	// Verify state structure
	if state.Timestamp.IsZero() {
		t.Error("State timestamp should be set")
	}

	if state.Confidence < 0.0 || state.Confidence > 1.0 {
		t.Errorf("Confidence should be between 0.0 and 1.0, got %.3f", state.Confidence)
	}

	validSeverities := []string{"low", "medium", "high"}
	validSeverity := false
	for _, severity := range validSeverities {
		if state.Severity == severity {
			validSeverity = true
			break
		}
	}
	if !validSeverity {
		t.Errorf("Invalid severity '%s', should be one of %v", state.Severity, validSeverities)
	}
}

func TestPolicyMatrixStatus(t *testing.T) {
	config := DefaultPolicyMatrixConfig()
	pm := NewPolicyMatrix(config)

	status := pm.GetPolicyStatus()
	if status == nil {
		t.Fatal("Policy status should not be nil")
	}

	expectedFields := []string{
		"venue_fallback_enabled",
		"depeg_guard_enabled",
		"risk_off_toggles_enabled",
		"healthy_venues",
		"total_venues",
		"risk_off_mode",
		"active_depeg_alerts",
		"last_update",
		"primary_venues",
	}

	for _, field := range expectedFields {
		if _, exists := status[field]; !exists {
			t.Errorf("Policy status missing field: %s", field)
		}
	}

	// Verify boolean fields
	if status["venue_fallback_enabled"] != config.VenueFallbackEnabled {
		t.Error("Venue fallback enabled status mismatch")
	}

	if status["depeg_guard_enabled"] != config.DepegGuardEnabled {
		t.Error("Depeg guard enabled status mismatch")
	}

	if status["risk_off_toggles_enabled"] != config.RiskOffTogglesEnabled {
		t.Error("Risk-off toggles enabled status mismatch")
	}
}

func TestStablecoinDetection(t *testing.T) {
	config := DefaultPolicyMatrixConfig()
	pm := NewPolicyMatrix(config)

	testCases := []struct {
		symbol      string
		isStablecoin bool
	}{
		{"BTCUSDT", true},
		{"ETHUSDC", true},
		{"ADAUSDT", true},
		{"BTCDAI", true},
		{"BTCUSD", false},   // Not a stablecoin pair (USD vs stablecoin)
		{"BTCEUR", false},   // Not a stablecoin pair
		{"ETHBTC", false},   // Not a stablecoin pair
	}

	for _, tc := range testCases {
		result := pm.isStablecoinPair(tc.symbol)
		if result != tc.isStablecoin {
			t.Errorf("Symbol %s: expected stablecoin pair = %t, got %t",
				tc.symbol, tc.isStablecoin, result)
		}
	}
}

func TestVenueApproval(t *testing.T) {
	config := DefaultPolicyMatrixConfig()
	pm := NewPolicyMatrix(config)

	testCases := []struct {
		venue    string
		approved bool
	}{
		{"kraken", true},
		{"coinbase", true},
		{"binance", true},
		{"okx", true},
		{"dexscreener", false},
		{"unknown", false},
		{"", false},
	}

	for _, tc := range testCases {
		result := pm.isVenueApproved(tc.venue)
		if result != tc.approved {
			t.Errorf("Venue %s: expected approved = %t, got %t",
				tc.venue, tc.approved, result)
		}
	}
}

func TestConfidenceScoreCalculation(t *testing.T) {
	config := DefaultPolicyMatrixConfig()
	pm := NewPolicyMatrix(config)

	// Test high confidence (no issues)
	cleanResult := &PolicyEvaluationResult{
		PolicyPassed:       true,
		PolicyViolations:   []string{},
		FallbackVenue:      "",
		FallbacksAttempted: 0,
		RiskOffCheck: &RiskOffCheckResult{
			RiskOffActive: false,
		},
	}

	score := pm.calculateConfidenceScore(cleanResult)
	if score < 0.9 {
		t.Errorf("Clean result should have high confidence (>0.9), got %.3f", score)
	}

	// Test reduced confidence (policy violations)
	violatedResult := &PolicyEvaluationResult{
		PolicyPassed:       false,
		PolicyViolations:   []string{"depeg detected", "risk-off active"},
		FallbackVenue:      "",
		FallbacksAttempted: 0,
	}

	score = pm.calculateConfidenceScore(violatedResult)
	if score >= 0.7 {
		t.Errorf("Violated result should have lower confidence (<0.7), got %.3f", score)
	}

	// Test fallback impact on confidence
	fallbackResult := &PolicyEvaluationResult{
		PolicyPassed:       true,
		PolicyViolations:   []string{},
		FallbackVenue:      "coinbase",
		FallbacksAttempted: 2,
	}

	score = pm.calculateConfidenceScore(fallbackResult)
	if score >= 0.9 {
		t.Errorf("Fallback result should have reduced confidence (<0.9), got %.3f", score)
	}
}

func TestRecommendedActionDetermination(t *testing.T) {
	config := DefaultPolicyMatrixConfig()
	pm := NewPolicyMatrix(config)

	testCases := []struct {
		description      string
		result           *PolicyEvaluationResult
		expectedAction   string
	}{
		{
			description: "Clean pass",
			result: &PolicyEvaluationResult{
				PolicyPassed:     true,
				PolicyViolations: []string{},
				FallbackVenue:    "",
			},
			expectedAction: "proceed",
		},
		{
			description: "Successful fallback",
			result: &PolicyEvaluationResult{
				PolicyPassed:     true,
				PolicyViolations: []string{},
				FallbackVenue:    "coinbase",
			},
			expectedAction: "proceed_with_fallback",
		},
		{
			description: "High severity risk-off",
			result: &PolicyEvaluationResult{
				PolicyPassed:     false,
				PolicyViolations: []string{"risk-off active"},
				RiskOffCheck: &RiskOffCheckResult{
					RiskOffActive: true,
					Severity:      "high",
				},
			},
			expectedAction: "halt",
		},
		{
			description: "Critical depeg",
			result: &PolicyEvaluationResult{
				PolicyPassed:     false,
				PolicyViolations: []string{"depeg detected"},
				DepegCheck: &DepegCheckResult{
					DepegDetected: true,
					MaxDepegBps:   250.0, // >2% depeg
				},
			},
			expectedAction: "halt",
		},
		{
			description: "General policy failure",
			result: &PolicyEvaluationResult{
				PolicyPassed:     false,
				PolicyViolations: []string{"some violation"},
			},
			expectedAction: "defer",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			action := pm.determineRecommendedAction(tc.result)
			if action != tc.expectedAction {
				t.Errorf("Expected action '%s', got '%s'", tc.expectedAction, action)
			}
		})
	}
}