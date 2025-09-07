package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cryptorun/internal/algo/dip"
	"cryptorun/internal/scan/pipeline"
	"cryptorun/internal/scan/sim"
)

func TestDipPipeline_StrongUptrendScenario_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "out")
	
	// Configure dip scanner with test-friendly settings
	config := pipeline.DipScanConfig{
		Trend: dip.TrendConfig{
			MALen12h:  10,
			MALen24h:  10,
			ADX4hMin:  20.0,
			HurstMin:  0.50,
			LookbackN: 15,
		},
		Fib: dip.FibConfig{
			Min: 0.35,
			Max: 0.65,
		},
		RSI: dip.RSIConfig{
			LowMin:         25,
			LowMax:         40,
			DivConfirmBars: 3,
		},
		Volume: dip.VolumeConfig{
			ADVMultMin: 1.3,
			VADRMin:    1.5,
		},
		Microstructure: dip.MicrostructureConfig{
			SpreadBpsMax:   60.0,
			DepthUSD2PcMin: 80000,
		},
		Scoring: pipeline.ScoringConfig{
			CoreWeight:    0.5,
			VolumeWeight:  0.2,
			QualityWeight: 0.2,
			BrandCap:      10,
			Threshold:     0.55, // Lower threshold for testing
		},
		Decay: dip.TimeDecayConfig{
			BarsToLive: 3,
		},
		Guards: dip.GuardsConfig{
			NewsShock: dip.NewsShockConfig{
				Return24hMin:   -12.0,
				AccelRebound:   2.5,
				ReboundBars:    3,
			},
			StairStep: dip.StairStepConfig{
				MaxAttempts:    3,
				LowerHighWindow: 6,
			},
			TimeDecay: dip.TimeDecayConfig{
				BarsToLive: 3,
			},
		},
	}
	
	// Create uptrend scenario data provider
	dataProvider := sim.CreateUptrendScenario("BTCUSD")
	
	dipPipeline := pipeline.NewDipPipeline(config, dataProvider, outputDir)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	candidates, err := dipPipeline.ScanForDips(ctx, []string{"BTCUSD"})
	if err != nil {
		t.Fatalf("ScanForDips failed: %v", err)
	}
	
	// Should find at least one qualified candidate
	if len(candidates) == 0 {
		t.Fatal("Expected to find dip candidates in strong uptrend scenario")
	}
	
	candidate := candidates[0]
	
	// Validate trend qualification
	if candidate.TrendResult == nil {
		t.Fatal("Trend result should not be nil")
	}
	
	if !candidate.TrendResult.Qualified {
		t.Errorf("Trend should be qualified, reason: %s", candidate.TrendResult.Reason)
	}
	
	// Validate dip detection
	if candidate.DipPoint == nil {
		t.Fatal("Dip point should be detected")
	}
	
	if candidate.DipPoint.RSI < 25 || candidate.DipPoint.RSI > 40 {
		t.Errorf("RSI should be in range [25,40], got: %.1f", candidate.DipPoint.RSI)
	}
	
	// Validate quality metrics
	if candidate.QualityScore == nil {
		t.Fatal("Quality score should not be nil")
	}
	
	if !candidate.QualityScore.Liquidity.Qualified {
		t.Errorf("Liquidity should qualify: %s", candidate.QualityScore.Liquidity.FailReason)
	}
	
	if !candidate.QualityScore.Volume.Qualified {
		t.Errorf("Volume should qualify: %s", candidate.QualityScore.Volume.FailReason)
	}
	
	// Validate composite scoring
	if candidate.CompositeScore < config.Scoring.Threshold {
		t.Errorf("Composite score %.2f should exceed threshold %.2f", 
			candidate.CompositeScore, config.Scoring.Threshold)
	}
	
	// Validate guard results
	if candidate.GuardResult == nil {
		t.Fatal("Guard result should not be nil")
	}
	
	if !candidate.GuardResult.Passed {
		t.Errorf("Guards should pass, but got veto: %s", candidate.GuardResult.VetoReason)
	}
	
	// Validate entry signal generation
	if candidate.Entry == nil {
		t.Error("Entry signal should be generated for qualified candidate")
	} else {
		if candidate.Entry.Confidence <= 0 || candidate.Entry.Confidence > 1 {
			t.Errorf("Entry confidence should be in (0,1], got: %.3f", candidate.Entry.Confidence)
		}
		
		if candidate.Entry.StopLoss >= candidate.Entry.Price {
			t.Error("Stop loss should be below entry price")
		}
		
		if len(candidate.Entry.TakeProfit) == 0 {
			t.Error("Should have take profit levels")
		}
	}
	
	// Validate attribution
	if candidate.Attribution.TrendSource == "" {
		t.Error("Attribution should include trend source")
	}
	
	if candidate.Attribution.ProcessingTime <= 0 {
		t.Error("Processing time should be recorded")
	}
	
	if len(candidate.Attribution.QualityChecks) == 0 {
		t.Error("Quality checks should be recorded in attribution")
	}
	
	// Check explainability output file
	explainPath := filepath.Join(outputDir, "scan", "dip_explain.json")
	if _, err := os.Stat(explainPath); os.IsNotExist(err) {
		t.Error("Explainability JSON should be generated")
	} else {
		// Validate JSON structure
		data, err := os.ReadFile(explainPath)
		if err != nil {
			t.Errorf("Failed to read explainability file: %v", err)
		} else {
			var report pipeline.ExplainabilityReport
			if err := json.Unmarshal(data, &report); err != nil {
				t.Errorf("Failed to parse explainability JSON: %v", err)
			} else {
				if report.TotalCandidates != len(candidates) {
					t.Errorf("Report should show %d candidates, got %d", 
						len(candidates), report.TotalCandidates)
				}
				
				if len(report.Candidates) != len(candidates) {
					t.Error("Report should include all candidates")
				}
				
				if report.Summary.TopSymbol != "BTCUSD" {
					t.Errorf("Top symbol should be BTCUSD, got: %s", report.Summary.TopSymbol)
				}
			}
		}
	}
	
	t.Logf("✅ Strong uptrend scenario passed: score=%.1f, entry=%.2f", 
		candidate.CompositeScore, candidate.Entry.Price)
}

func TestDipPipeline_ChoppyMarketScenario_Rejection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "out")
	
	config := pipeline.DipScanConfig{
		Trend: dip.TrendConfig{
			MALen12h:  10,
			MALen24h:  10,
			ADX4hMin:  25.0,
			HurstMin:  0.55,
			LookbackN: 15,
		},
		Fib: dip.FibConfig{Min: 0.38, Max: 0.62},
		RSI: dip.RSIConfig{LowMin: 25, LowMax: 40, DivConfirmBars: 3},
		Volume: dip.VolumeConfig{ADVMultMin: 1.4, VADRMin: 1.75},
		Microstructure: dip.MicrostructureConfig{
			SpreadBpsMax:   50.0,
			DepthUSD2PcMin: 100000,
		},
		Scoring: pipeline.ScoringConfig{
			CoreWeight: 0.5, VolumeWeight: 0.2, QualityWeight: 0.2,
			BrandCap: 10, Threshold: 0.62,
		},
		Decay: dip.TimeDecayConfig{BarsToLive: 2},
		Guards: dip.GuardsConfig{
			NewsShock: dip.NewsShockConfig{Return24hMin: -15.0, AccelRebound: 3.0, ReboundBars: 2},
			StairStep: dip.StairStepConfig{MaxAttempts: 2, LowerHighWindow: 5},
			TimeDecay: dip.TimeDecayConfig{BarsToLive: 2},
		},
	}
	
	// Create choppy market scenario
	dataProvider := sim.CreateChoppyMarketScenario("ETHUSD")
	
	dipPipeline := pipeline.NewDipPipeline(config, dataProvider, outputDir)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	candidates, err := dipPipeline.ScanForDips(ctx, []string{"ETHUSD"})
	if err != nil {
		t.Fatalf("ScanForDips failed: %v", err)
	}
	
	// Should find no candidates in choppy market
	if len(candidates) != 0 {
		t.Errorf("Expected no candidates in choppy market, got %d", len(candidates))
		
		// If we did find candidates, they should have failed trend qualification
		for i, candidate := range candidates {
			if candidate.TrendResult != nil && candidate.TrendResult.Qualified {
				t.Errorf("Candidate %d should not qualify trend in choppy market", i)
			}
		}
	}
	
	// Explainability output should still be generated
	explainPath := filepath.Join(outputDir, "scan", "dip_explain.json")
	if _, err := os.Stat(explainPath); os.IsNotExist(err) {
		t.Error("Explainability JSON should be generated even with no candidates")
	} else {
		data, err := os.ReadFile(explainPath)
		if err != nil {
			t.Errorf("Failed to read explainability file: %v", err)
		} else {
			var report pipeline.ExplainabilityReport
			if err := json.Unmarshal(data, &report); err != nil {
				t.Errorf("Failed to parse explainability JSON: %v", err)
			} else {
				if report.TotalCandidates != 0 {
					t.Errorf("Report should show 0 candidates, got %d", report.TotalCandidates)
				}
			}
		}
	}
	
	t.Logf("✅ Choppy market correctly rejected: 0 candidates found")
}

func TestDipPipeline_NewsShockScenario_GuardVeto(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "out")
	
	config := pipeline.DipScanConfig{
		Trend: dip.TrendConfig{
			MALen12h: 10, MALen24h: 10, 
			ADX4hMin: 20.0, HurstMin: 0.50, LookbackN: 15,
		},
		Fib: dip.FibConfig{Min: 0.30, Max: 0.70}, // Wider range
		RSI: dip.RSIConfig{LowMin: 20, LowMax: 45, DivConfirmBars: 3}, // Wider range
		Volume: dip.VolumeConfig{ADVMultMin: 1.2, VADRMin: 1.4}, // Lower requirements
		Microstructure: dip.MicrostructureConfig{
			SpreadBpsMax: 80.0, DepthUSD2PcMin: 60000, // More lenient
		},
		Scoring: pipeline.ScoringConfig{
			CoreWeight: 0.5, VolumeWeight: 0.2, QualityWeight: 0.2,
			BrandCap: 10, Threshold: 0.45, // Lower threshold to allow technical qualification
		},
		Decay: dip.TimeDecayConfig{BarsToLive: 4},
		Guards: dip.GuardsConfig{
			NewsShock: dip.NewsShockConfig{
				Return24hMin: -12.0, // Stricter news shock threshold
				AccelRebound: 2.5, ReboundBars: 2,
			},
			StairStep: dip.StairStepConfig{MaxAttempts: 3, LowerHighWindow: 6},
			TimeDecay: dip.TimeDecayConfig{BarsToLive: 4},
		},
	}
	
	// Create news shock scenario
	dataProvider := sim.CreateNewsShockScenario("BTCUSD")
	
	dipPipeline := pipeline.NewDipPipeline(config, dataProvider, outputDir)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	candidates, err := dipPipeline.ScanForDips(ctx, []string{"BTCUSD"})
	if err != nil {
		t.Fatalf("ScanForDips failed: %v", err)
	}
	
	// Should find no candidates due to guard veto
	if len(candidates) != 0 {
		t.Errorf("News shock should be vetoed by guards, but got %d candidates", len(candidates))
		
		// If candidates exist, they should have failed guards
		for i, candidate := range candidates {
			if candidate.GuardResult == nil {
				t.Errorf("Candidate %d should have guard results", i)
			} else if candidate.GuardResult.Passed {
				t.Errorf("Candidate %d should fail guards in news shock scenario", i)
			} else {
				// Check that news shock guard specifically failed
				newsCheck, exists := candidate.GuardResult.GuardChecks["news_shock"]
				if !exists {
					t.Errorf("Candidate %d should have news shock check", i)
				} else if newsCheck.Passed {
					t.Errorf("Candidate %d news shock guard should fail", i)
				}
			}
		}
	}
	
	t.Logf("✅ News shock scenario correctly vetoed by guards")
}

func TestDipPipeline_MultipleSymbolsScenario(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "out")
	
	config := pipeline.DipScanConfig{
		Trend: dip.TrendConfig{
			MALen12h: 8, MALen24h: 8,
			ADX4hMin: 22.0, HurstMin: 0.52, LookbackN: 12,
		},
		Fib: dip.FibConfig{Min: 0.35, Max: 0.65},
		RSI: dip.RSIConfig{LowMin: 25, LowMax: 40, DivConfirmBars: 3},
		Volume: dip.VolumeConfig{ADVMultMin: 1.3, VADRMin: 1.6},
		Microstructure: dip.MicrostructureConfig{
			SpreadBpsMax: 55.0, DepthUSD2PcMin: 90000,
		},
		Scoring: pipeline.ScoringConfig{
			CoreWeight: 0.5, VolumeWeight: 0.2, QualityWeight: 0.2,
			BrandCap: 10, Threshold: 0.58,
		},
		Decay: dip.TimeDecayConfig{BarsToLive: 2},
		Guards: dip.GuardsConfig{
			NewsShock: dip.NewsShockConfig{Return24hMin: -15.0, AccelRebound: 3.0, ReboundBars: 2},
			StairStep: dip.StairStepConfig{MaxAttempts: 2, LowerHighWindow: 5},
			TimeDecay: dip.TimeDecayConfig{BarsToLive: 2},
		},
	}
	
	// Create mixed data provider with multiple scenarios
	dataProvider := sim.NewFixtureDataProvider()
	
	// BTCUSD: Strong uptrend (should qualify)
	uptrendProvider := sim.CreateUptrendScenario("BTCUSD")
	for timeframe, data := range uptrendProvider.GetAllMarketData("BTCUSD") {
		dataProvider.SetMarketData("BTCUSD", timeframe, data)
	}
	dataProvider.SetMicrostructureData("BTCUSD", uptrendProvider.GetStoredMicrostructureData("BTCUSD"))
	dataProvider.SetSocialData("BTCUSD", uptrendProvider.GetStoredSocialData("BTCUSD"))
	
	// ETHUSD: Choppy market (should be rejected)
	choppyProvider := sim.CreateChoppyMarketScenario("ETHUSD")
	for timeframe, data := range choppyProvider.GetAllMarketData("ETHUSD") {
		dataProvider.SetMarketData("ETHUSD", timeframe, data)
	}
	dataProvider.SetMicrostructureData("ETHUSD", choppyProvider.GetStoredMicrostructureData("ETHUSD"))
	
	dipPipeline := pipeline.NewDipPipeline(config, dataProvider, outputDir)
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	candidates, err := dipPipeline.ScanForDips(ctx, []string{"BTCUSD", "ETHUSD"})
	if err != nil {
		t.Fatalf("ScanForDips failed: %v", err)
	}
	
	// Should find candidates only for BTCUSD (uptrend scenario)
	btcCandidates := 0
	ethCandidates := 0
	
	for _, candidate := range candidates {
		if candidate.Symbol == "BTCUSD" {
			btcCandidates++
		} else if candidate.Symbol == "ETHUSD" {
			ethCandidates++
		}
	}
	
	if btcCandidates == 0 {
		t.Error("Should find dip candidates for BTCUSD (uptrend scenario)")
	}
	
	if ethCandidates != 0 {
		t.Error("Should not find candidates for ETHUSD (choppy scenario)")
	}
	
	// Validate explainability report covers both symbols
	explainPath := filepath.Join(outputDir, "scan", "dip_explain.json")
	if _, err := os.Stat(explainPath); os.IsNotExist(err) {
		t.Error("Explainability JSON should be generated")
	} else {
		data, err := os.ReadFile(explainPath)
		if err == nil {
			var report pipeline.ExplainabilityReport
			if err := json.Unmarshal(data, &report); err == nil {
				// Report should have summary of all analysis
				if report.TotalCandidates != len(candidates) {
					t.Errorf("Report total should match candidates: expected %d, got %d",
						len(candidates), report.TotalCandidates)
				}
				
				if report.Summary.TopSymbol == "" && len(candidates) > 0 {
					t.Error("Summary should identify top symbol when candidates exist")
				}
			}
		}
	}
	
	t.Logf("✅ Multiple symbols processed: BTC=%d candidates, ETH=%d candidates", 
		btcCandidates, ethCandidates)
}

func TestDipPipeline_DataProviderErrors_GracefulHandling(t *testing.T) {
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "out")
	
	config := pipeline.DipScanConfig{
		Trend: dip.TrendConfig{MALen12h: 5, MALen24h: 5, ADX4hMin: 20.0, HurstMin: 0.50, LookbackN: 5},
		Fib: dip.FibConfig{Min: 0.38, Max: 0.62},
		RSI: dip.RSIConfig{LowMin: 25, LowMax: 40, DivConfirmBars: 3},
		Volume: dip.VolumeConfig{ADVMultMin: 1.4, VADRMin: 1.75},
		Microstructure: dip.MicrostructureConfig{SpreadBpsMax: 50.0, DepthUSD2PcMin: 100000},
		Scoring: pipeline.ScoringConfig{CoreWeight: 0.5, VolumeWeight: 0.2, QualityWeight: 0.2, BrandCap: 10, Threshold: 0.62},
		Decay: dip.TimeDecayConfig{BarsToLive: 2},
		Guards: dip.GuardsConfig{
			NewsShock: dip.NewsShockConfig{Return24hMin: -15.0, AccelRebound: 3.0, ReboundBars: 2},
			StairStep: dip.StairStepConfig{MaxAttempts: 2, LowerHighWindow: 5},
			TimeDecay: dip.TimeDecayConfig{BarsToLive: 2},
		},
	}
	
	// Create data provider that simulates failures
	dataProvider := sim.NewFixtureDataProvider()
	dataProvider.SetFailureSimulator(&sim.FailureSimulator{
		MarketDataError: true,
		ErrorMessage:    "simulated market data unavailable",
	})
	
	dipPipeline := pipeline.NewDipPipeline(config, dataProvider, outputDir)
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	candidates, err := dipPipeline.ScanForDips(ctx, []string{"FAILUSD"})
	
	// Should handle errors gracefully and continue
	if err != nil {
		t.Errorf("Pipeline should handle data provider errors gracefully: %v", err)
	}
	
	// Should have no candidates due to data errors
	if len(candidates) != 0 {
		t.Errorf("Should have no candidates when data provider fails, got %d", len(candidates))
	}
	
	// Should still generate explainability output
	explainPath := filepath.Join(outputDir, "scan", "dip_explain.json")
	if _, err := os.Stat(explainPath); os.IsNotExist(err) {
		t.Error("Should generate explainability output even on data errors")
	}
	
	t.Logf("✅ Data provider errors handled gracefully")
}