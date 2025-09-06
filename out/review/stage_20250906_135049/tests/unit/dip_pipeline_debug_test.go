package unit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"cryptorun/internal/algo/dip"
	"cryptorun/internal/scan/pipeline"
	"cryptorun/internal/scan/sim"
)

func TestDipPipeline_GenerateExplainabilityOutput(t *testing.T) {
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "out")
	
	// Configure with more lenient settings
	config := pipeline.DipScanConfig{
		Trend: dip.TrendConfig{
			MALen12h:  10,
			MALen24h:  10,
			ADX4hMin:  15.0,  // Lower ADX requirement
			HurstMin:  0.45,  // Lower Hurst requirement 
			LookbackN: 15,
		},
		Fib: dip.FibConfig{
			Min: 0.30,  // Wider Fib range
			Max: 0.70,
		},
		RSI: dip.RSIConfig{
			LowMin:         20,  // Allow lower RSI
			LowMax:         45,  // Allow higher RSI
			DivConfirmBars: 3,
		},
		Volume: dip.VolumeConfig{
			ADVMultMin: 1.2,  // Lower volume requirement
			VADRMin:    1.4,  // Lower VADR requirement
		},
		Microstructure: dip.MicrostructureConfig{
			SpreadBpsMax:   70.0,  // Allow wider spreads
			DepthUSD2PcMin: 70000,  // Lower depth requirement
		},
		Scoring: pipeline.ScoringConfig{
			CoreWeight:    0.5,
			VolumeWeight:  0.2,
			QualityWeight: 0.2,
			BrandCap:      10,
			Threshold:     0.40,  // Much lower threshold
		},
		Decay: dip.TimeDecayConfig{
			BarsToLive: 5,
		},
		Guards: dip.GuardsConfig{
			NewsShock: dip.NewsShockConfig{
				Return24hMin:   -20.0,  // Allow more severe drops
				AccelRebound:   2.0,    // Lower rebound requirement
				ReboundBars:    3,
			},
			StairStep: dip.StairStepConfig{
				MaxAttempts:    4,    // Allow more attempts
				LowerHighWindow: 6,
			},
			TimeDecay: dip.TimeDecayConfig{
				BarsToLive: 5,
			},
		},
	}
	
	// Create uptrend scenario data provider
	dataProvider := sim.CreateUptrendScenario("BTCUSD")
	
	dipPipeline := pipeline.NewDipPipeline(config, dataProvider, outputDir)
	
	ctx := context.Background()
	candidates, err := dipPipeline.ScanForDips(ctx, []string{"BTCUSD"})
	if err != nil {
		t.Fatalf("ScanForDips failed: %v", err)
	}
	
	t.Logf("Found %d candidates", len(candidates))
	
	// Check explainability output was generated
	explainPath := filepath.Join(outputDir, "scan", "dip_explain.json")
	if _, err := os.Stat(explainPath); os.IsNotExist(err) {
		t.Fatal("Explainability output not generated")
	}
	
	// Read and log explainability content
	data, err := os.ReadFile(explainPath)
	if err != nil {
		t.Fatalf("Failed to read explainability output: %v", err)
	}
	
	var report pipeline.ExplainabilityReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("Failed to parse explainability output: %v", err)
	}
	
	t.Logf("Explainability report generated:")
	t.Logf("  Processing time: %v", report.ProcessingTime)
	t.Logf("  Total candidates: %d", report.TotalCandidates)
	t.Logf("  Qualified trends: %d", report.Summary.QualifiedTrends)
	t.Logf("  Detected dips: %d", report.Summary.DetectedDips)
	t.Logf("  Passed guards: %d", report.Summary.PassedGuards)
	if report.TotalCandidates > 0 {
		t.Logf("  Avg composite score: %.2f", report.Summary.AvgCompositeScore)
		t.Logf("  Top symbol: %s", report.Summary.TopSymbol)
		t.Logf("  Top score: %.2f", report.Summary.TopScore)
		
		candidate := report.Candidates[0]
		t.Logf("  First candidate:")
		t.Logf("    Composite score: %.2f", candidate.CompositeScore)
		if candidate.TrendResult != nil {
			t.Logf("    Trend qualified: %v", candidate.TrendResult.Qualified)
		}
		if candidate.DipPoint != nil {
			t.Logf("    Dip RSI: %.1f", candidate.DipPoint.RSI)
			t.Logf("    Dip Fib: %.2f", candidate.DipPoint.FibLevel)
		}
		if candidate.QualityScore != nil {
			t.Logf("    Liquidity qualified: %v", candidate.QualityScore.Liquidity.Qualified)
			t.Logf("    Volume qualified: %v", candidate.QualityScore.Volume.Qualified)
		}
	}
	
	t.Logf("✅ Explainability output generated at: %s", explainPath)
}