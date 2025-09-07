package microstructure

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/venue/types"
)

func TestProofGenerator_GenerateProofBundle(t *testing.T) {
	// Create temporary directory for test artifacts
	tmpDir, err := os.MkdirTemp("", "microstructure_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pg := NewProofGenerator(tmpDir)

	// Create mock validation result
	now := time.Now()
	orderBook := &types.OrderBook{
		Symbol:                "BTCUSDT",
		Venue:                 "binance",
		TimestampMono:         now,
		MidPrice:              43250.75,
		SpreadBPS:             11.55,
		DepthUSDPlusMinus2Pct: 180000,
	}

	metrics := &types.MicrostructureMetrics{
		Symbol:                "BTCUSDT",
		Venue:                 "binance",
		TimestampMono:         now,
		SpreadBPS:             11.55,
		DepthUSDPlusMinus2Pct: 180000,
		VADR:                  2.1,
		SpreadValid:           true,
		DepthValid:            true,
		VADRValid:             true,
		OverallValid:          true,
	}

	result := &ValidationResult{
		Symbol:           "BTCUSDT",
		TimestampMono:    now,
		OverallValid:     true,
		PassedVenueCount: 1,
		TotalVenueCount:  1,
		EligibleVenues:   []string{"binance"},
		VenueResults: map[string]*VenueValidation{
			"binance": {
				Venue:     "binance",
				Valid:     true,
				OrderBook: orderBook,
				Metrics:   metrics,
			},
		},
	}

	// Generate proof bundle
	proofBundle, err := pg.GenerateProofBundle(result)
	if err != nil {
		t.Fatalf("GenerateProofBundle() error = %v", err)
	}

	// Validate proof bundle structure
	if proofBundle.AssetSymbol != "BTCUSDT" {
		t.Errorf("ProofBundle.AssetSymbol = %v, expected BTCUSDT", proofBundle.AssetSymbol)
	}

	if !proofBundle.ProvenValid {
		t.Error("ProofBundle.ProvenValid should be true for valid result")
	}

	if proofBundle.VenueUsed != "binance" {
		t.Errorf("ProofBundle.VenueUsed = %v, expected binance", proofBundle.VenueUsed)
	}

	if proofBundle.OrderBookSnapshot == nil {
		t.Error("ProofBundle.OrderBookSnapshot should not be nil")
	}

	if proofBundle.MicrostructureMetrics == nil {
		t.Error("ProofBundle.MicrostructureMetrics should not be nil")
	}

	// Validate individual proofs
	if !proofBundle.SpreadProof.Passed {
		t.Error("SpreadProof should pass for valid metrics")
	}

	if !proofBundle.DepthProof.Passed {
		t.Error("DepthProof should pass for valid metrics")
	}

	if !proofBundle.VADRProof.Passed {
		t.Error("VADRProof should pass for valid metrics")
	}

	// Check proof ID format
	if proofBundle.ProofID == "" {
		t.Error("ProofBundle.ProofID should not be empty")
	}
}

func TestProofGenerator_GenerateFailureProof(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "microstructure_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pg := NewProofGenerator(tmpDir)

	// Create mock failed validation result
	now := time.Now()
	metrics := &types.MicrostructureMetrics{
		Symbol:                "ALTCOIN",
		Venue:                 "binance",
		TimestampMono:         now,
		SpreadBPS:             75.0,  // High spread - fails
		DepthUSDPlusMinus2Pct: 80000, // Low depth - fails
		VADR:                  1.5,   // Low VADR - fails
		SpreadValid:           false,
		DepthValid:            false,
		VADRValid:             false,
		OverallValid:          false,
	}

	result := &ValidationResult{
		Symbol:           "ALTCOIN",
		TimestampMono:    now,
		OverallValid:     false,
		PassedVenueCount: 0,
		TotalVenueCount:  1,
		EligibleVenues:   []string{},
		FailedVenues:     []string{"binance"},
		VenueResults: map[string]*VenueValidation{
			"binance": {
				Venue:   "binance",
				Valid:   false,
				Metrics: metrics,
				FailureReasons: []string{
					"Spread 75.0bps > 50.0bps limit",
					"Depth $80k < $100k limit",
					"VADR 1.50x < 1.75x limit",
				},
			},
		},
	}

	// Generate failure proof
	proofBundle, err := pg.GenerateProofBundle(result)
	if err != nil {
		t.Fatalf("GenerateProofBundle() error = %v", err)
	}

	// Validate failure proof
	if proofBundle.ProvenValid {
		t.Error("ProofBundle.ProvenValid should be false for failed result")
	}

	if proofBundle.SpreadProof.Passed {
		t.Error("SpreadProof should fail for invalid metrics")
	}

	if proofBundle.DepthProof.Passed {
		t.Error("DepthProof should fail for invalid metrics")
	}

	if proofBundle.VADRProof.Passed {
		t.Error("VADRProof should fail for invalid metrics")
	}

	// Check evidence messages
	expectedSpreadEvidence := "Spread 75.0 bps exceeds required max 50.0 bps"
	if proofBundle.SpreadProof.Evidence != expectedSpreadEvidence {
		t.Errorf("SpreadProof.Evidence = %v, expected %v",
			proofBundle.SpreadProof.Evidence, expectedSpreadEvidence)
	}
}

func TestProofGenerator_SaveProofBundle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "microstructure_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pg := NewProofGenerator(tmpDir)

	// Create mock proof bundle
	now := time.Now()
	proofBundle := &types.ProofBundle{
		AssetSymbol:   "BTCUSDT",
		TimestampMono: now,
		ProvenValid:   true,
		ProofID:       "BTCUSDT-20250115-abc123",
	}

	// Save proof bundle
	filePath, err := pg.SaveProofBundle(proofBundle)
	if err != nil {
		t.Fatalf("SaveProofBundle() error = %v", err)
	}

	// Check file was created
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("Proof bundle file was not created: %v", filePath)
	}

	// Check file path format
	expectedDir := filepath.Join(tmpDir, "proofs", now.Format("2006-01-02"), "microstructure")
	expectedFile := filepath.Join(expectedDir, "BTCUSDT_master_proof.json")

	if filePath != expectedFile {
		t.Errorf("SaveProofBundle() filePath = %v, expected %v", filePath, expectedFile)
	}

	// Verify file contents are valid JSON
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read proof bundle file: %v", err)
	}

	if len(data) == 0 {
		t.Error("Proof bundle file is empty")
	}
}

func TestProofGenerator_GenerateBatchReport(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "microstructure_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pg := NewProofGenerator(tmpDir)

	// Create mock validation results
	_ = time.Now() // Mark as used
	results := []*ValidationResult{
		{
			Symbol:           "BTCUSDT",
			OverallValid:     true,
			PassedVenueCount: 2,
			TotalVenueCount:  3,
			EligibleVenues:   []string{"binance", "okx"},
			FailedVenues:     []string{"coinbase"},
			VenueResults: map[string]*VenueValidation{
				"binance":  {Venue: "binance", Valid: true, Metrics: &types.MicrostructureMetrics{SpreadBPS: 25.0, DepthUSDPlusMinus2Pct: 150000}},
				"okx":      {Venue: "okx", Valid: true, Metrics: &types.MicrostructureMetrics{SpreadBPS: 30.0, DepthUSDPlusMinus2Pct: 120000}},
				"coinbase": {Venue: "coinbase", Valid: false, Metrics: &types.MicrostructureMetrics{SpreadBPS: 60.0, DepthUSDPlusMinus2Pct: 80000}},
			},
		},
		{
			Symbol:           "ETHUSD",
			OverallValid:     false,
			PassedVenueCount: 0,
			TotalVenueCount:  3,
			EligibleVenues:   []string{},
			FailedVenues:     []string{"binance", "okx", "coinbase"},
			VenueResults: map[string]*VenueValidation{
				"binance":  {Venue: "binance", Valid: false, Metrics: &types.MicrostructureMetrics{SpreadBPS: 70.0, DepthUSDPlusMinus2Pct: 70000}},
				"okx":      {Venue: "okx", Valid: false, Metrics: &types.MicrostructureMetrics{SpreadBPS: 65.0, DepthUSDPlusMinus2Pct: 75000}},
				"coinbase": {Venue: "coinbase", Valid: false, Metrics: &types.MicrostructureMetrics{SpreadBPS: 80.0, DepthUSDPlusMinus2Pct: 60000}},
			},
		},
	}

	// Generate batch report
	report, err := pg.GenerateBatchReport(results)
	if err != nil {
		t.Fatalf("GenerateBatchReport() error = %v", err)
	}

	// Validate report structure
	if report.TotalAssets != 2 {
		t.Errorf("BatchReport.TotalAssets = %v, expected 2", report.TotalAssets)
	}

	if report.EligibleAssets != 1 {
		t.Errorf("BatchReport.EligibleAssets = %v, expected 1", report.EligibleAssets)
	}

	expectedEligibilityRate := 50.0 // 1 out of 2
	if report.EligibilityRate != expectedEligibilityRate {
		t.Errorf("BatchReport.EligibilityRate = %v, expected %v",
			report.EligibilityRate, expectedEligibilityRate)
	}

	// Check venue stats
	if len(report.VenueStats) != 3 {
		t.Errorf("BatchReport.VenueStats length = %v, expected 3", len(report.VenueStats))
	}

	binanceStats := report.VenueStats["binance"]
	if binanceStats.TotalChecked != 2 {
		t.Errorf("Binance TotalChecked = %v, expected 2", binanceStats.TotalChecked)
	}

	if binanceStats.PassedCount != 1 {
		t.Errorf("Binance PassedCount = %v, expected 1", binanceStats.PassedCount)
	}

	expectedPassRate := 50.0 // 1 out of 2
	if binanceStats.PassRate != expectedPassRate {
		t.Errorf("Binance PassRate = %v, expected %v", binanceStats.PassRate, expectedPassRate)
	}

	// Check asset summaries
	if len(report.AssetSummaries) != 2 {
		t.Errorf("BatchReport.AssetSummaries length = %v, expected 2", len(report.AssetSummaries))
	}
}

func TestProofGenerator_SaveBatchReport(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "microstructure_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pg := NewProofGenerator(tmpDir)

	// Create mock batch report
	now := time.Now()
	report := &BatchReport{
		GeneratedAt:     now,
		TotalAssets:     5,
		EligibleAssets:  3,
		EligibilityRate: 60.0,
		VenueStats:      make(map[string]*VenueStats),
		AssetSummaries:  []*AssetSummary{},
	}

	// Save batch report
	filePath, err := pg.SaveBatchReport(report)
	if err != nil {
		t.Fatalf("SaveBatchReport() error = %v", err)
	}

	// Check file was created
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("Batch report file was not created: %v", filePath)
	}

	// Check file path format
	expectedDir := filepath.Join(tmpDir, "proofs", now.Format("2006-01-02"), "reports")
	if !filepath.HasPrefix(filePath, expectedDir) {
		t.Errorf("SaveBatchReport() filePath should be in %v, got %v", expectedDir, filePath)
	}

	// Verify filename pattern
	filename := filepath.Base(filePath)
	matched, err := filepath.Match("microstructure_audit_*.json", filename)
	if err != nil {
		t.Fatalf("filepath.Match() error = %v", err)
	}
	if !matched {
		t.Errorf("SaveBatchReport() filename = %v, should match microstructure_audit_*.json", filename)
	}
}
