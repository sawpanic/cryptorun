package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"cryptorun/internal/application"
)

// TestUniverseBuilder_HashIntegrity tests universe hash generation and integrity
func TestUniverseBuilder_HashIntegrity(t *testing.T) {
	criteria := application.UniverseCriteria{
		Quote:     "USD",
		MinADVUSD: 100000,
		Venue:     "kraken",
	}
	
	builder := application.NewUniverseBuilder(criteria)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Build universe
	result, err := builder.BuildUniverse(ctx)
	if err != nil {
		t.Fatalf("Universe build failed: %v", err)
	}
	
	// Verify hash is 64-character hex
	if len(result.NewHash) != 64 {
		t.Errorf("Hash length should be 64, got %d", len(result.NewHash))
	}
	
	// Verify hash contains only hex characters
	for _, char := range result.NewHash {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			t.Errorf("Hash contains non-hex character: %c", char)
		}
	}
	
	// Verify metadata contains criteria
	if result.Snapshot.Metadata.Criteria.MinADVUSD != criteria.MinADVUSD {
		t.Errorf("ADV criteria mismatch: expected %d, got %d", 
			criteria.MinADVUSD, result.Snapshot.Metadata.Criteria.MinADVUSD)
	}
	
	// Build again with same criteria - hash should be identical
	result2, err := builder.BuildUniverse(ctx)
	if err != nil {
		t.Fatalf("Second universe build failed: %v", err)
	}
	
	if result.NewHash != result2.NewHash {
		t.Errorf("Hash not deterministic: %s != %s", result.NewHash, result2.NewHash)
	}
}

// TestUniverseBuilder_USDOnlyCompliance tests USD-only symbol filtering
func TestUniverseBuilder_USDOnlyCompliance(t *testing.T) {
	criteria := application.UniverseCriteria{
		Quote:     "USD",
		MinADVUSD: 100000,
		Venue:     "kraken",
	}
	
	builder := application.NewUniverseBuilder(criteria)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	result, err := builder.BuildUniverse(ctx)
	if err != nil {
		t.Fatalf("Universe build failed: %v", err)
	}
	
	// Check every symbol ends with USD
	for _, symbol := range result.Snapshot.Universe {
		if len(symbol) < 4 || symbol[len(symbol)-3:] != "USD" {
			t.Errorf("Non-USD symbol found: %s", symbol)
		}
		
		// Check symbol format: ^[A-Z0-9]+USD$
		base := symbol[:len(symbol)-3]
		if len(base) == 0 {
			t.Errorf("Empty base symbol: %s", symbol)
		}
		
		for _, char := range base {
			if !((char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
				t.Errorf("Invalid character in symbol %s: %c", symbol, char)
			}
		}
	}
}

// TestUniverseBuilder_XBTNormalization tests XBT→BTC symbol normalization
func TestUniverseBuilder_XBTNormalization(t *testing.T) {
	builder := application.NewUniverseBuilder(application.UniverseCriteria{
		Quote:     "USD",
		MinADVUSD: 100000,
		Venue:     "kraken",
	})
	
	// Test normalization function with mock data
	testSymbols := []string{"XBTUSD", "ETHUSD", "XBTEUR", "ADAUSD"}
	normalized := builder.NormalizeSymbols(testSymbols)
	
	// Should contain BTCUSD (not XBTUSD), ETHUSD, ADAUSD
	// Should NOT contain XBTEUR (not USD)
	expectedUSD := map[string]bool{
		"BTCUSD": true,
		"ETHUSD": true,
		"ADAUSD": true,
	}
	
	for _, symbol := range normalized {
		if !expectedUSD[symbol] {
			t.Errorf("Unexpected symbol after normalization: %s", symbol)
		}
		delete(expectedUSD, symbol)
	}
	
	if len(expectedUSD) > 0 {
		t.Errorf("Missing expected symbols after normalization: %v", expectedUSD)
	}
}

// TestUniverseBuilder_ADVFilter tests ADV filtering (mock)
func TestUniverseBuilder_ADVFilter(t *testing.T) {
	criteria := application.UniverseCriteria{
		Quote:     "USD",
		MinADVUSD: 100000,
		Venue:     "kraken",
	}
	
	builder := application.NewUniverseBuilder(criteria)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	result, err := builder.BuildUniverse(ctx)
	if err != nil {
		t.Fatalf("Universe build failed: %v", err)
	}
	
	// Verify criteria is recorded correctly
	if result.Snapshot.Metadata.Criteria.MinADVUSD != 100000 {
		t.Errorf("ADV criteria not recorded: expected 100000, got %d", 
			result.Snapshot.Metadata.Criteria.MinADVUSD)
	}
	
	// In a real implementation, this would validate actual ADV data
	// For now, ensure the criteria structure is correct
	if result.Snapshot.Metadata.Criteria.Quote != "USD" {
		t.Errorf("Quote criteria not recorded: expected USD, got %s", 
			result.Snapshot.Metadata.Criteria.Quote)
	}
}

// TestRiskEnvelope_PositionSizing tests ATR-based position sizing
func TestRiskEnvelope_PositionSizing(t *testing.T) {
	envelope := application.NewRiskEnvelope()
	
	testCases := []struct {
		symbol       string
		atr          float64
		currentPrice float64
		expectedSize float64
		shouldError  bool
	}{
		{"BTCUSD", 1000, 50000, 10000, false},  // Base: $10k / ATR $1k = $10k
		{"ETHUSD", 100, 3000, 10000, false},    // Base: $10k / ATR $100 = $10k  
		{"SOLUSD", 5, 100, 10000, false},       // Base: $10k / ATR $5 = $10k
		{"BTCUSD", 100, 50000, 50000, false},   // Base: $10k / ATR $100 = $100k → cap at $50k
		{"BTCUSD", 0, 50000, 0, true},          // Invalid ATR should error
		{"BTCUSD", 1000, 0, 0, true},           // Invalid price should error
	}
	
	for _, tc := range testCases {
		size, err := envelope.CalculatePositionSize(tc.symbol, tc.atr, tc.currentPrice)
		
		if tc.shouldError {
			if err == nil {
				t.Errorf("Expected error for %s with ATR=%f, price=%f", tc.symbol, tc.atr, tc.currentPrice)
			}
			continue
		}
		
		if err != nil {
			t.Errorf("Unexpected error for %s: %v", tc.symbol, err)
			continue
		}
		
		if size != tc.expectedSize {
			t.Errorf("Position size mismatch for %s: expected %f, got %f", tc.symbol, tc.expectedSize, size)
		}
	}
}

// TestRiskEnvelope_PositionCountLimits tests maximum position limits
func TestRiskEnvelope_PositionCountLimits(t *testing.T) {
	envelope := application.NewRiskEnvelope()
	
	ctx := context.Background()
	
	// Fill up to the position limit (15 positions)
	for i := 0; i < 15; i++ {
		symbol := fmt.Sprintf("TEST%dUSD", i)
		result, err := envelope.CheckRiskLimits(ctx, symbol, 10000)
		if err != nil {
			t.Errorf("Risk check failed for position %d: %v", i, err)
		}
		
		if !result.Passed {
			t.Errorf("Position %d should pass risk limits", i)
		}
		
		// Simulate adding position
		envelope.AddPosition(symbol, 10000)
	}
	
	// 16th position should fail
	result, err := envelope.CheckRiskLimits(ctx, "TEST16USD", 10000)
	if err != nil {
		t.Errorf("Risk check error: %v", err)
	}
	
	if result.Passed {
		t.Error("16th position should fail due to position count limit")
	}
	
	// Check for position count violation
	found := false
	for _, violation := range result.Violations {
		if violation.Rule == "max_positions" {
			found = true
			if violation.Severity != "ERROR" {
				t.Errorf("Position count violation should be ERROR, got %s", violation.Severity)
			}
			break
		}
	}
	
	if !found {
		t.Error("Expected max_positions violation not found")
	}
}

// TestRiskEnvelope_SingleAssetLimits tests single asset concentration limits
func TestRiskEnvelope_SingleAssetLimits(t *testing.T) {
	envelope := application.NewRiskEnvelope()
	envelope.SetPortfolioValue(100000) // $100k portfolio
	
	ctx := context.Background()
	
	// Test 5% position (should pass)
	result, err := envelope.CheckRiskLimits(ctx, "BTCUSD", 5000)
	if err != nil {
		t.Errorf("Risk check failed: %v", err)
	}
	if !result.Passed {
		t.Error("5% position should pass")
	}
	
	// Test 15% position (should fail - exceeds 10% limit)
	result, err = envelope.CheckRiskLimits(ctx, "BTCUSD", 15000)
	if err != nil {
		t.Errorf("Risk check failed: %v", err)
	}
	if result.Passed {
		t.Error("15% position should fail due to single asset limit")
	}
	
	// Check for single asset violation
	found := false
	for _, violation := range result.Violations {
		if violation.Rule == "single_asset_limit" {
			found = true
			if violation.Severity != "ERROR" {
				t.Errorf("Single asset violation should be ERROR, got %s", violation.Severity)
			}
			if violation.Current != 15.0 || violation.Limit != 10.0 {
				t.Errorf("Single asset violation values wrong: current=%f, limit=%f", 
					violation.Current, violation.Limit)
			}
			break
		}
	}
	
	if !found {
		t.Error("Expected single_asset_limit violation not found")
	}
}

// TestRiskEnvelope_CorrelationLimits tests correlation clustering detection
func TestRiskEnvelope_CorrelationLimits(t *testing.T) {
	envelope := application.NewRiskEnvelope()
	
	// Set up high correlation between BTCUSD and ETHUSD
	envelope.SetCorrelation("BTCUSD", "ETHUSD", 0.85)
	envelope.AddPosition("BTCUSD", 10000)
	
	ctx := context.Background()
	
	// Check ETHUSD position with high correlation to existing BTCUSD
	result, err := envelope.CheckRiskLimits(ctx, "ETHUSD", 10000)
	if err != nil {
		t.Errorf("Risk check failed: %v", err)
	}
	
	// Should have correlation warning (not error for now)
	found := false
	for _, violation := range result.Violations {
		if violation.Rule == "correlation_limit" {
			found = true
			if violation.Severity != "WARNING" {
				t.Errorf("Correlation violation should be WARNING, got %s", violation.Severity)
			}
			if violation.Current != 0.85 || violation.Limit != 0.70 {
				t.Errorf("Correlation violation values wrong: current=%f, limit=%f", 
					violation.Current, violation.Limit)
			}
			break
		}
	}
	
	if !found {
		t.Error("Expected correlation_limit violation not found")
	}
}

// TestRiskEnvelope_EmergencyControls tests emergency pause and blacklist
func TestRiskEnvelope_EmergencyControls(t *testing.T) {
	envelope := application.NewRiskEnvelope()
	ctx := context.Background()
	
	// Test normal operation
	result, err := envelope.CheckRiskLimits(ctx, "BTCUSD", 10000)
	if err != nil {
		t.Errorf("Risk check failed: %v", err)
	}
	if !result.Passed {
		t.Error("Normal operation should pass")
	}
	
	// Trigger emergency pause
	envelope.TriggerEmergencyPause([]string{"Drawdown exceeds limit"})
	
	// Check should now fail due to global pause
	result, err = envelope.CheckRiskLimits(ctx, "BTCUSD", 10000)
	if err != nil {
		t.Errorf("Risk check failed: %v", err)
	}
	if result.Passed {
		t.Error("Should fail due to global pause")
	}
	
	// Check for global pause violation
	found := false
	for _, violation := range result.Violations {
		if violation.Rule == "global_pause" {
			found = true
			if violation.Severity != "ERROR" {
				t.Errorf("Global pause violation should be ERROR, got %s", violation.Severity)
			}
			break
		}
	}
	
	if !found {
		t.Error("Expected global_pause violation not found")
	}
}

// TestRiskEnvelope_DrawdownPause tests automatic drawdown pause
func TestRiskEnvelope_DrawdownPause(t *testing.T) {
	envelope := application.NewRiskEnvelope()
	
	// Update drawdown to 5% (below 8% threshold)
	envelope.UpdateDrawdown(0.05)
	summary := envelope.GetRiskSummary()
	if summary["global_pause"].(bool) {
		t.Error("Should not pause at 5% drawdown")
	}
	
	// Update drawdown to 10% (above 8% threshold)
	envelope.UpdateDrawdown(0.10)
	summary = envelope.GetRiskSummary()
	if !summary["global_pause"].(bool) {
		t.Error("Should pause at 10% drawdown")
	}
	
	// Check pause reasons contain drawdown
	reasons := summary["pause_reasons"].([]string)
	if len(reasons) == 0 {
		t.Error("Should have pause reasons")
	}
	
	foundDrawdownReason := false
	for _, reason := range reasons {
		if strings.Contains(strings.ToLower(reason), "drawdown") {
			foundDrawdownReason = true
			break
		}
	}
	
	if !foundDrawdownReason {
		t.Error("Pause reasons should include drawdown")
	}
}

// TestRiskEnvelope_SymbolBlacklist tests symbol blacklisting
func TestRiskEnvelope_SymbolBlacklist(t *testing.T) {
	envelope := application.NewRiskEnvelope()
	ctx := context.Background()
	
	// Blacklist BTCUSD
	envelope.BlacklistSymbol("BTCUSD", "Failed risk gate")
	
	// Check BTCUSD should fail
	result, err := envelope.CheckRiskLimits(ctx, "BTCUSD", 10000)
	if err != nil {
		t.Errorf("Risk check failed: %v", err)
	}
	if result.Passed {
		t.Error("Blacklisted symbol should fail")
	}
	
	// Check ETHUSD should still pass
	result, err = envelope.CheckRiskLimits(ctx, "ETHUSD", 10000)
	if err != nil {
		t.Errorf("Risk check failed: %v", err)
	}
	if !result.Passed {
		t.Error("Non-blacklisted symbol should pass")
	}
	
	// Check for blacklist violation
	result, _ = envelope.CheckRiskLimits(ctx, "BTCUSD", 10000)
	found := false
	for _, violation := range result.Violations {
		if violation.Rule == "symbol_blacklist" {
			found = true
			if violation.Severity != "ERROR" {
				t.Errorf("Blacklist violation should be ERROR, got %s", violation.Severity)
			}
			break
		}
	}
	
	if !found {
		t.Error("Expected symbol_blacklist violation not found")
	}
}

// TestRiskEnvelope_SectorLimits tests sector concentration caps
func TestRiskEnvelope_SectorLimits(t *testing.T) {
	envelope := application.NewRiskEnvelope()
	envelope.SetPortfolioValue(100000)
	
	ctx := context.Background()
	
	// Add positions in DeFi sector (UNI, AAVE)
	envelope.AddPosition("UNIUSD", 25000)  // 25% in UNI (DeFi)
	
	// Try to add more DeFi (should trigger warning at 30% cap)
	result, err := envelope.CheckRiskLimits(ctx, "AAVEUSD", 10000) // Would be 35% DeFi total
	if err != nil {
		t.Errorf("Risk check failed: %v", err)
	}
	
	// Should have sector cap warning
	found := false
	for _, violation := range result.Violations {
		if violation.Rule == "sector_cap" {
			found = true
			if violation.Severity != "WARNING" {
				t.Errorf("Sector cap violation should be WARNING, got %s", violation.Severity)
			}
			break
		}
	}
	
	if !found {
		t.Error("Expected sector_cap violation not found")
	}
}

// TestRiskEnvelope_RiskSummary tests risk summary generation
func TestRiskEnvelope_RiskSummary(t *testing.T) {
	envelope := application.NewRiskEnvelope()
	
	// Add some positions
	envelope.AddPosition("BTCUSD", 10000)
	envelope.AddPosition("ETHUSD", 8000)
	envelope.BlacklistSymbol("DOTUSD", "Test blacklist")
	
	summary := envelope.GetRiskSummary()
	
	// Check required fields
	requiredFields := []string{
		"global_pause", "pause_reasons", "positions", 
		"total_exposure_usd", "current_drawdown", 
		"blacklisted_symbols", "active_caps", "violations",
		"degraded_mode", "last_update",
	}
	
	for _, field := range requiredFields {
		if _, exists := summary[field]; !exists {
			t.Errorf("Risk summary missing required field: %s", field)
		}
	}
	
	// Check blacklisted count
	if summary["blacklisted_symbols"].(int) != 1 {
		t.Errorf("Expected 1 blacklisted symbol, got %d", summary["blacklisted_symbols"].(int))
	}
}

// Helper methods are now exported from the main application package for testing

// BenchmarkUniverseHashGeneration benchmarks hash generation performance
func BenchmarkUniverseHashGeneration(b *testing.B) {
	criteria := application.UniverseCriteria{
		Quote:     "USD",
		MinADVUSD: 100000,
		Venue:     "kraken",
	}
	
	builder := application.NewUniverseBuilder(criteria)
	
	// Create mock snapshot
	snapshot := &application.UniverseSnapshot{
		Metadata: application.UniverseMetadata{
			Generated: time.Now(),
			Source:    "kraken",
			Criteria:  criteria,
		},
		Universe: []string{"BTCUSD", "ETHUSD", "ADAUSD", "SOLUSD", "DOTUSD"},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = builder.GenerateHash(snapshot)
	}
}

// BenchmarkRiskCheckLimits benchmarks risk limit checking performance
func BenchmarkRiskCheckLimits(b *testing.B) {
	envelope := application.NewRiskEnvelope()
	envelope.SetPortfolioValue(100000)
	
	// Add some existing positions
	for i := 0; i < 10; i++ {
		envelope.AddPosition(fmt.Sprintf("TEST%dUSD", i), 5000)
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		symbol := fmt.Sprintf("BENCH%dUSD", i%100)
		_, _ = envelope.CheckRiskLimits(ctx, symbol, 10000)
	}
}