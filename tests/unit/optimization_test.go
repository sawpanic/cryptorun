package unit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/application/optimization"
)

// TestMomentumParameterValidation tests momentum parameter validation
func TestMomentumParameterValidation(t *testing.T) {
	config := optimization.OptimizerConfig{
		Target:        optimization.TargetMomentum,
		MaxIterations: 10,
	}

	provider := &MockDataProvider{}
	evaluator := optimization.NewStandardEvaluator()

	optimizer := optimization.NewMomentumOptimizer(config, provider, evaluator)

	// Test valid parameters
	validParams := optimization.ParameterSet{
		Target: optimization.TargetMomentum,
		Parameters: map[string]optimization.Parameter{
			"bull_weight_1h": {
				Name:  "bull_weight_1h",
				Value: 0.20,
				Min:   0.15,
				Max:   0.25,
				Type:  "float",
			},
			"bull_weight_4h": {
				Name:  "bull_weight_4h",
				Value: 0.35,
				Min:   0.30,
				Max:   0.40,
				Type:  "float",
			},
			"bull_weight_12h": {
				Name:  "bull_weight_12h",
				Value: 0.30,
				Min:   0.25,
				Max:   0.35,
				Type:  "float",
			},
			"bull_weight_24h": {
				Name:  "bull_weight_24h",
				Value: 0.15,
				Min:   0.10,
				Max:   0.15,
				Type:  "float",
			},
		},
	}

	validation := optimizer.ValidateParameters(validParams)
	if !validation.Valid {
		t.Errorf("Expected valid parameters, got errors: %v", validation.Errors)
	}

	// Test invalid parameters (weights don't sum to 1.0)
	invalidParams := validParams
	invalidParams.Parameters["bull_weight_1h"] = optimization.Parameter{
		Name:  "bull_weight_1h",
		Value: 0.50, // Too high
		Min:   0.15,
		Max:   0.25,
		Type:  "float",
	}

	validation = optimizer.ValidateParameters(invalidParams)
	if validation.Valid {
		t.Error("Expected invalid parameters due to weight sum, but validation passed")
	}
}

// TestDipParameterValidation tests dip parameter validation
func TestDipParameterValidation(t *testing.T) {
	config := optimization.OptimizerConfig{
		Target:        optimization.TargetDip,
		MaxIterations: 10,
	}

	provider := &MockDataProvider{}
	evaluator := optimization.NewStandardEvaluator()

	optimizer := optimization.NewDipOptimizer(config, provider, evaluator)

	// Test valid parameters
	validParams := optimization.ParameterSet{
		Target: optimization.TargetDip,
		Parameters: map[string]optimization.Parameter{
			"rsi_trigger_1h": {
				Name:  "rsi_trigger_1h",
				Value: 25.0,
				Min:   18.0,
				Max:   32.0,
				Type:  "float",
			},
			"dip_depth_min": {
				Name:  "dip_depth_min",
				Value: -12.0,
				Min:   -20.0,
				Max:   -6.0,
				Type:  "float",
			},
			"volume_flush_min": {
				Name:  "volume_flush_min",
				Value: 1.8,
				Min:   1.25,
				Max:   2.5,
				Type:  "float",
			},
		},
	}

	validation := optimizer.ValidateParameters(validParams)
	if !validation.Valid {
		t.Errorf("Expected valid dip parameters, got errors: %v", validation.Errors)
	}

	// Test invalid RSI trigger
	invalidParams := validParams
	invalidParams.Parameters["rsi_trigger_1h"] = optimization.Parameter{
		Name:  "rsi_trigger_1h",
		Value: 35.0, // Above max bound
		Min:   18.0,
		Max:   32.0,
		Type:  "float",
	}

	validation = optimizer.ValidateParameters(invalidParams)
	if validation.Valid {
		t.Error("Expected invalid parameters due to RSI bounds, but validation passed")
	}
}

// TestPrecisionCalculation tests precision@N calculation
func TestPrecisionCalculation(t *testing.T) {
	evaluator := optimization.NewStandardEvaluator()

	// Create test predictions
	predictions := []optimization.Prediction{
		{CompositeScore: 90.0, Success24h: true, Success48h: true, Gates: optimization.GateStatus{AllPass: true}},
		{CompositeScore: 85.0, Success24h: true, Success48h: false, Gates: optimization.GateStatus{AllPass: true}},
		{CompositeScore: 80.0, Success24h: false, Success48h: true, Gates: optimization.GateStatus{AllPass: true}},
		{CompositeScore: 75.0, Success24h: false, Success48h: false, Gates: optimization.GateStatus{AllPass: true}},
		{CompositeScore: 70.0, Success24h: true, Success48h: true, Gates: optimization.GateStatus{AllPass: true}},
	}

	metrics := evaluator.CalculatePrecisionMetrics(predictions)

	// Check precision@3 (top 3 predictions)
	// Expected: 2 successes out of 3 for 24h = 66.67%
	expectedP20_24h := 2.0 / 3.0 // 2 successes in top 3
	if abs(metrics.Precision20_24h-expectedP20_24h) > 0.01 {
		t.Errorf("Expected precision@20 (24h) %.2f, got %.2f", expectedP20_24h, metrics.Precision20_24h)
	}

	// Check total predictions
	if metrics.TotalPredictions != 5 {
		t.Errorf("Expected 5 total predictions, got %d", metrics.TotalPredictions)
	}

	// Check win rate
	expectedWinRate24h := 3.0 / 5.0 // 3 successes out of 5
	if abs(metrics.WinRate24h-expectedWinRate24h) > 0.01 {
		t.Errorf("Expected win rate 24h %.2f, got %.2f", expectedWinRate24h, metrics.WinRate24h)
	}
}

// TestDataProvider tests the file data provider
func TestDataProvider(t *testing.T) {
	// Create temporary ledger file
	tempDir := t.TempDir()
	ledgerPath := filepath.Join(tempDir, "test_ledger.jsonl")

	// Write test data
	testData := `{"ts_scan":"2025-09-05T10:30:00Z","symbol":"BTCUSD","composite":75.5,"gates_all_pass":true,"horizons":{"24h":"2025-09-06T10:30:00Z","48h":"2025-09-07T10:30:00Z"},"realized":{"24h":2.8,"48h":3.2},"pass":{"24h":true,"48h":true}}
{"ts_scan":"2025-09-05T11:30:00Z","symbol":"ETHUSD","composite":68.2,"gates_all_pass":true,"horizons":{"24h":"2025-09-06T11:30:00Z","48h":"2025-09-07T11:30:00Z"},"realized":{"24h":-1.2,"48h":0.8},"pass":{"24h":false,"48h":true}}
`

	err := os.WriteFile(ledgerPath, []byte(testData), 0644)
	if err != nil {
		t.Fatalf("Failed to write test ledger: %v", err)
	}

	// Test data provider
	provider := optimization.NewFileDataProvider(ledgerPath, "", "")

	// Validate availability
	err = provider.ValidateDataAvailability()
	if err != nil {
		t.Errorf("Data validation failed: %v", err)
	}

	// Get all data
	ctx := context.Background()
	data, err := provider.GetLedgerData(ctx, time.Time{}, time.Time{})
	if err != nil {
		t.Errorf("Failed to get ledger data: %v", err)
	}

	if len(data) != 2 {
		t.Errorf("Expected 2 ledger entries, got %d", len(data))
	}

	// Check first entry
	if data[0].Symbol != "BTCUSD" {
		t.Errorf("Expected first symbol BTCUSD, got %s", data[0].Symbol)
	}

	if data[0].Composite != 75.5 {
		t.Errorf("Expected composite 75.5, got %.1f", data[0].Composite)
	}

	// Get data summary
	summary, err := provider.GetDataSummary(ctx)
	if err != nil {
		t.Errorf("Failed to get data summary: %v", err)
	}

	if summary.TotalEntries != 2 {
		t.Errorf("Expected 2 total entries, got %d", summary.TotalEntries)
	}

	if summary.UniqueSymbols != 2 {
		t.Errorf("Expected 2 unique symbols, got %d", summary.UniqueSymbols)
	}
}

// TestOptimizationObjective tests objective function calculation
func TestOptimizationObjective(t *testing.T) {
	metrics := optimization.EvaluationMetrics{
		Precision20_24h:    0.75,
		Precision20_48h:    0.65,
		FalsePositiveRate:  0.20,
		MaxDrawdownPenalty: 0.10,
	}

	objective := optimization.CalculateObjective(metrics)

	// J = 1.0·precision@20(24h) + 0.5·precision@20(48h) – 0.2·false_positive_rate – 0.2·max_drawdown_penalty
	expected := 1.0*0.75 + 0.5*0.65 - 0.2*0.20 - 0.2*0.10
	expected = 0.75 + 0.325 - 0.04 - 0.02
	expected = 1.015

	if abs(objective-expected) > 0.001 {
		t.Errorf("Expected objective %.4f, got %.4f", expected, objective)
	}
}

// MockDataProvider implements DataProvider for testing
type MockDataProvider struct{}

func (mdp *MockDataProvider) GetLedgerData(ctx context.Context, start, end time.Time) ([]optimization.LedgerEntry, error) {
	// Return minimal test data
	return []optimization.LedgerEntry{
		{
			TsScan:    time.Now().Add(-24 * time.Hour),
			Symbol:    "BTCUSD",
			Composite: 75.0,
			GatesPass: true,
			Realized: struct {
				H24 float64 `json:"24h"`
				H48 float64 `json:"48h"`
			}{H24: 2.5, H48: 3.8},
			Pass: struct {
				H24 bool `json:"24h"`
				H48 bool `json:"48h"`
			}{H24: true, H48: true},
		},
		{
			TsScan:    time.Now().Add(-23 * time.Hour),
			Symbol:    "ETHUSD",
			Composite: 68.0,
			GatesPass: true,
			Realized: struct {
				H24 float64 `json:"24h"`
				H48 float64 `json:"48h"`
			}{H24: -1.2, H48: 0.5},
			Pass: struct {
				H24 bool `json:"24h"`
				H48 bool `json:"48h"`
			}{H24: false, H48: true},
		},
	}, nil
}

func (mdp *MockDataProvider) GetMarketData(ctx context.Context, symbol string, start, end time.Time) ([]optimization.MarketDataPoint, error) {
	return []optimization.MarketDataPoint{}, nil
}

func (mdp *MockDataProvider) ValidateDataAvailability() error {
	return nil
}

func (mdp *MockDataProvider) GetDataSummary(ctx context.Context) (*optimization.DataSummary, error) {
	return &optimization.DataSummary{
		TotalEntries:  2,
		UniqueSymbols: 2,
		GatePassRate:  1.0,
	}, nil
}

func (mdp *MockDataProvider) ClearCache() {}

func (mdp *MockDataProvider) GetCacheStats() map[string]int {
	return map[string]int{"test": 0}
}

// Helper function for absolute value
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
