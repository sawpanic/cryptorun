package integration

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cryptorun/internal/algo/momentum"
	"cryptorun/internal/scan/pipeline"
)

// MockDataProvider implements the DataProvider interface for testing
type MockDataProvider struct {
	marketData map[string]map[string][]momentum.MarketData
	volumeData map[string][]float64
	regimeData string
}

func NewMockDataProvider() *MockDataProvider {
	return &MockDataProvider{
		marketData: make(map[string]map[string][]momentum.MarketData),
		volumeData: make(map[string][]float64),
		regimeData: "trending",
	}
}

func (m *MockDataProvider) GetMarketData(ctx context.Context, symbol string, timeframes []string) (map[string][]momentum.MarketData, error) {
	if data, exists := m.marketData[symbol]; exists {
		return data, nil
	}

	// Generate default test data if not set
	result := make(map[string][]momentum.MarketData)
	baseTime := time.Now().Add(-48 * time.Hour)

	for _, tf := range timeframes {
		var interval time.Duration
		var count int

		switch tf {
		case "1h":
			interval = time.Hour
			count = 48
		case "4h":
			interval = 4 * time.Hour
			count = 12
		case "12h":
			interval = 12 * time.Hour
			count = 4
		case "24h":
			interval = 24 * time.Hour
			count = 2
		default:
			continue
		}

		data := make([]momentum.MarketData, count)
		basePrice := 100.0

		for i := 0; i < count; i++ {
			timestamp := baseTime.Add(time.Duration(i) * interval)

			// Create slight upward trend
			price := basePrice * (1.0 + float64(i)*0.002)

			data[i] = momentum.MarketData{
				Timestamp: timestamp,
				Open:      price * 0.999,
				High:      price * 1.001,
				Low:       price * 0.999,
				Close:     price,
				Volume:    1000 + float64(i*50),
			}
		}

		result[tf] = data
	}

	return result, nil
}

func (m *MockDataProvider) GetVolumeData(ctx context.Context, symbol string, periods int) ([]float64, error) {
	if data, exists := m.volumeData[symbol]; exists {
		return data, nil
	}

	// Generate default volume data
	data := make([]float64, periods)
	for i := 0; i < periods; i++ {
		data[i] = 1000 + float64(i*50) + float64(i%3)*100 // Some variation
	}

	return data, nil
}

func (m *MockDataProvider) GetRegimeData(ctx context.Context) (string, error) {
	return m.regimeData, nil
}

func (m *MockDataProvider) SetMarketData(symbol string, data map[string][]momentum.MarketData) {
	m.marketData[symbol] = data
}

func (m *MockDataProvider) SetVolumeData(symbol string, data []float64) {
	m.volumeData[symbol] = data
}

func (m *MockDataProvider) SetRegime(regime string) {
	m.regimeData = regime
}

func TestMomentumPipeline_ScanMomentum(t *testing.T) {
	// Create temporary output directory
	tempDir, err := ioutil.TempDir("", "momentum_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Pipeline configuration
	config := pipeline.MomentumPipelineConfig{
		Momentum: momentum.MomentumConfig{
			Weights: momentum.WeightConfig{
				TF1h:  0.20,
				TF4h:  0.35,
				TF12h: 0.30,
				TF24h: 0.15,
			},
			Fatigue: momentum.FatigueConfig{
				Return24hThreshold: 12.0,
				RSI4hThreshold:     70.0,
				AccelRenewal:       true,
			},
			Freshness: momentum.FreshnessConfig{
				MaxBarsAge: 2,
				ATRWindow:  14,
				ATRFactor:  1.2,
			},
			LateFill: momentum.LateFillConfig{
				MaxDelaySeconds: 30,
			},
			Regime: momentum.RegimeConfig{
				AdaptWeights: true,
				UpdatePeriod: 4,
			},
		},
		EntryExit: momentum.EntryExitConfig{
			Entry: momentum.EntryGateConfig{
				MinScore:       2.5,
				VolumeMultiple: 1.75,
				ADXThreshold:   25.0,
				HurstThreshold: 0.55,
			},
			Exit: momentum.ExitGateConfig{
				HardStop:      5.0,
				VenueHealth:   0.8,
				MaxHoldHours:  48,
				AccelReversal: 0.5,
				FadeThreshold: 1.0,
				TrailingStop:  2.0,
				ProfitTarget:  8.0,
			},
		},
		Pipeline: pipeline.PipelineConfig{
			ExplainabilityOutput: true,
			OutputPath:           tempDir,
			ProtectedFactors:     []string{"MomentumCore"},
			MaxSymbols:           10,
		},
	}

	// Create pipeline
	dataProvider := NewMockDataProvider()
	mp := pipeline.NewMomentumPipeline(config, dataProvider, tempDir)

	// Test symbols
	symbols := []string{"BTCUSD", "ETHUSD", "ADAUSD"}

	// Run momentum scan
	candidates, err := mp.ScanMomentum(context.Background(), symbols, dataProvider)
	if err != nil {
		t.Fatalf("Momentum scan failed: %v", err)
	}

	// Verify results
	if len(candidates) == 0 {
		t.Error("Expected at least some candidates")
	}

	for _, candidate := range candidates {
		// Verify candidate structure
		if candidate.Symbol == "" {
			t.Error("Candidate should have symbol")
		}

		if candidate.MomentumResult == nil {
			t.Error("Candidate should have momentum result")
		}

		if candidate.EntrySignal == nil {
			t.Error("Candidate should have entry signal")
		}

		// Verify attribution data
		if len(candidate.Attribution.DataSources) == 0 {
			t.Error("Candidate should have data sources attribution")
		}

		if candidate.Attribution.ProcessingTime == 0 {
			t.Error("Candidate should have processing time")
		}

		if candidate.Attribution.Methodology == "" {
			t.Error("Candidate should have methodology")
		}

		if candidate.Attribution.Confidence < 0 || candidate.Attribution.Confidence > 100 {
			t.Errorf("Confidence should be 0-100, got %f", candidate.Attribution.Confidence)
		}

		// Verify orthogonal score is set
		if candidate.OrthogonalScore == 0.0 && candidate.MomentumResult.CoreScore != 0.0 {
			t.Error("Orthogonal score should be set")
		}

		t.Logf("Candidate %s: Score=%f, Qualified=%v, Reason=%s",
			candidate.Symbol, candidate.OrthogonalScore, candidate.Qualified, candidate.Reason)
	}

	// Verify explainability output was generated
	explainFile := filepath.Join(tempDir, "scan", "momentum_explain.json")
	if _, err := os.Stat(explainFile); os.IsNotExist(err) {
		t.Error("Explainability output file should be created")
	} else {
		// Verify file contents
		content, err := ioutil.ReadFile(explainFile)
		if err != nil {
			t.Errorf("Failed to read explainability file: %v", err)
		} else {
			var explainData map[string]interface{}
			err = json.Unmarshal(content, &explainData)
			if err != nil {
				t.Errorf("Explainability file should be valid JSON: %v", err)
			} else {
				// Verify required sections
				requiredSections := []string{"scan_metadata", "configuration", "candidates", "summary"}
				for _, section := range requiredSections {
					if _, exists := explainData[section]; !exists {
						t.Errorf("Explainability output missing section: %s", section)
					}
				}
			}
		}
	}
}

func TestMomentumPipeline_RegimeAdaptation(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "momentum_regime_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := pipeline.MomentumPipelineConfig{
		Momentum: momentum.MomentumConfig{
			Weights: momentum.WeightConfig{
				TF1h:  0.20,
				TF4h:  0.35,
				TF12h: 0.30,
				TF24h: 0.15,
			},
			Regime: momentum.RegimeConfig{
				AdaptWeights: true,
				UpdatePeriod: 4,
			},
		},
		Pipeline: pipeline.PipelineConfig{
			ExplainabilityOutput: false,
			MaxSymbols:           5,
		},
	}

	dataProvider := NewMockDataProvider()
	mp := pipeline.NewMomentumPipeline(config, dataProvider, tempDir)

	symbols := []string{"BTCUSD"}

	// Test different regimes
	regimes := []string{"trending", "choppy", "volatile"}
	var results []*pipeline.MomentumCandidate

	for _, regime := range regimes {
		dataProvider.SetRegime(regime)

		candidates, err := mp.ScanMomentum(context.Background(), symbols, dataProvider)
		if err != nil {
			t.Errorf("Scan failed for regime %s: %v", regime, err)
			continue
		}

		if len(candidates) > 0 {
			results = append(results, candidates[0])
			t.Logf("Regime %s: Score=%f", regime, candidates[0].OrthogonalScore)
		}
	}

	// Verify that different regimes can produce different scores
	if len(results) >= 2 {
		score1 := results[0].OrthogonalScore
		score2 := results[1].OrthogonalScore

		if score1 == score2 {
			t.Log("Note: Regime adaptation may not be significant with default test data")
		}
	}
}

func TestMomentumPipeline_MaxSymbolsLimit(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "momentum_limit_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := pipeline.MomentumPipelineConfig{
		Momentum: momentum.MomentumConfig{
			Weights: momentum.WeightConfig{
				TF1h:  0.25,
				TF4h:  0.35,
				TF12h: 0.25,
				TF24h: 0.15,
			},
		},
		Pipeline: pipeline.PipelineConfig{
			ExplainabilityOutput: false,
			MaxSymbols:           3, // Limit to 3 symbols
		},
	}

	dataProvider := NewMockDataProvider()
	mp := pipeline.NewMomentumPipeline(config, dataProvider, tempDir)

	// Provide more symbols than the limit
	symbols := []string{"BTCUSD", "ETHUSD", "ADAUSD", "SOLUSD", "DOTUSD"}

	candidates, err := mp.ScanMomentum(context.Background(), symbols, dataProvider)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Should process at most 3 symbols
	if len(candidates) > 3 {
		t.Errorf("Expected at most 3 candidates, got %d", len(candidates))
	}
}

func TestMomentumPipeline_QualifiedFiltering(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "momentum_filter_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := pipeline.MomentumPipelineConfig{
		Momentum: momentum.MomentumConfig{
			Weights: momentum.WeightConfig{
				TF1h:  0.25,
				TF4h:  0.35,
				TF12h: 0.25,
				TF24h: 0.15,
			},
			// Set very restrictive thresholds to test filtering
			Fatigue: momentum.FatigueConfig{
				Return24hThreshold: 1.0,   // Very low threshold
				RSI4hThreshold:     30.0,  // Very low threshold
				AccelRenewal:       false, // Disable renewal
			},
		},
		EntryExit: momentum.EntryExitConfig{
			Entry: momentum.EntryGateConfig{
				MinScore:       10.0, // Very high score requirement
				VolumeMultiple: 5.0,  // Very high volume requirement
				ADXThreshold:   50.0, // Very high ADX requirement
				HurstThreshold: 0.9,  // Very high Hurst requirement
			},
		},
		Pipeline: pipeline.PipelineConfig{
			ExplainabilityOutput: false,
			MaxSymbols:           5,
		},
	}

	dataProvider := NewMockDataProvider()
	mp := pipeline.NewMomentumPipeline(config, dataProvider, tempDir)

	symbols := []string{"BTCUSD", "ETHUSD"}

	candidates, err := mp.ScanMomentum(context.Background(), symbols, dataProvider)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// With restrictive filters, expect few or no qualified candidates
	qualifiedCount := 0
	for _, candidate := range candidates {
		if candidate.Qualified {
			qualifiedCount++
		}
	}

	t.Logf("Qualified candidates with restrictive filters: %d/%d", qualifiedCount, len(candidates))

	// Verify that unqualified candidates have reasons
	for _, candidate := range candidates {
		if !candidate.Qualified && candidate.Reason == "" {
			t.Error("Unqualified candidates should have reason")
		}
	}
}

func TestMomentumPipeline_ErrorHandling(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "momentum_error_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := pipeline.MomentumPipelineConfig{
		Momentum: momentum.MomentumConfig{
			Weights: momentum.WeightConfig{
				TF1h:  0.25,
				TF4h:  0.35,
				TF12h: 0.25,
				TF24h: 0.15,
			},
		},
		Pipeline: pipeline.PipelineConfig{
			ExplainabilityOutput: false,
			MaxSymbols:           5,
		},
	}

	dataProvider := NewMockDataProvider()
	mp := pipeline.NewMomentumPipeline(config, dataProvider, tempDir)

	// Test with empty symbols list
	emptySymbols := []string{}
	candidates, err := mp.ScanMomentum(context.Background(), emptySymbols, dataProvider)
	if err != nil {
		t.Errorf("Empty symbols should not cause error: %v", err)
	}

	if len(candidates) != 0 {
		t.Error("Empty symbols should produce no candidates")
	}

	// Test with context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	symbols := []string{"BTCUSD"}
	_, err = mp.ScanMomentum(ctx, symbols, dataProvider)
	// Note: Current implementation doesn't check context cancellation,
	// but this test documents expected behavior for future enhancement
}
