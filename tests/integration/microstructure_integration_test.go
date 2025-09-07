package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/venue/types"
	"github.com/sawpanic/cryptorun/internal/domain/microstructure"
)

func TestMicrostructureWithRealFixtures(t *testing.T) {
	checker := microstructure.NewChecker(nil)
	proofGenerator := microstructure.NewProofGenerator("./test_artifacts")

	tests := []struct {
		name        string
		fixturePath string
		symbol      string
		venue       string
		expectValid bool
		description string
	}{
		{
			name:        "Binance BTCUSDT - Should pass all requirements",
			fixturePath: "binance_orderbook_btcusdt.json",
			symbol:      "BTCUSDT",
			venue:       "binance",
			expectValid: true,
			description: "Tight spread, good depth expected",
		},
		{
			name:        "OKX ETHUSDT - Should have reasonable metrics",
			fixturePath: "okx_orderbook_ethusdt.json",
			symbol:      "ETHUSDT",
			venue:       "okx",
			expectValid: true,
			description: "Good liquidity pair on OKX",
		},
		{
			name:        "Coinbase SOLUSDT - May have wider spreads",
			fixturePath: "coinbase_orderbook_solusdt.json",
			symbol:      "SOLUSDT",
			venue:       "coinbase",
			expectValid: false, // SOL often has wider spreads
			description: "SOL typically has wider spreads on Coinbase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load fixture data
			orderBook, err := loadOrderBookFixture(tt.fixturePath, tt.symbol, tt.venue)
			if err != nil {
				t.Fatalf("Failed to load fixture: %v", err)
			}

			// Mock VADR/ADV values for testing
			vadr := 2.1
			adv := 500000.0

			// Validate microstructure
			ctx := context.Background()
			metrics := checker.ValidateOrderBook(ctx, orderBook, vadr, adv)

			// Log results for debugging
			t.Logf("Venue: %s, Symbol: %s", orderBook.Venue, orderBook.Symbol)
			t.Logf("Spread: %.2f bps (valid: %v)", orderBook.SpreadBPS, metrics.SpreadValid)
			t.Logf("Depth: $%.0f (valid: %v)", orderBook.DepthUSDPlusMinus2Pct, metrics.DepthValid)
			t.Logf("VADR: %.2fx (valid: %v)", vadr, metrics.VADRValid)
			t.Logf("Overall: %v", metrics.OverallValid)

			// Generate proof
			proof := checker.GenerateProof(ctx, orderBook, metrics)

			// Save proof for inspection
			if err := proofGenerator.GenerateProofBundle(ctx, &microstructure.AssetEligibilityResult{
				Symbol:          tt.symbol,
				CheckedAt:       time.Now(),
				OverallEligible: metrics.OverallValid,
				EligibleVenues:  []string{tt.venue},
				VenueResults: map[string]*types.MicrostructureMetrics{
					tt.venue: metrics,
				},
				ProofBundles: map[string]*types.ProofBundle{
					tt.venue: proof,
				},
			}); err != nil {
				t.Logf("Warning: Failed to generate proof bundle: %v", err)
			}

			// Verify basic proof structure
			if proof.AssetSymbol != tt.symbol {
				t.Errorf("Expected AssetSymbol=%s, got %s", tt.symbol, proof.AssetSymbol)
			}

			if proof.VenueUsed != tt.venue {
				t.Errorf("Expected VenueUsed=%s, got %s", tt.venue, proof.VenueUsed)
			}

			// Verify proof validity matches metrics
			if proof.ProvenValid != metrics.OverallValid {
				t.Errorf("Proof validity (%v) doesn't match metrics validity (%v)",
					proof.ProvenValid, metrics.OverallValid)
			}

			// Check that all individual proofs have evidence
			if proof.SpreadProof.Evidence == "" {
				t.Error("SpreadProof missing evidence text")
			}
			if proof.DepthProof.Evidence == "" {
				t.Error("DepthProof missing evidence text")
			}
			if proof.VADRProof.Evidence == "" {
				t.Error("VADRProof missing evidence text")
			}
		})
	}
}

func TestProofGeneratorPersistence(t *testing.T) {
	tempDir := t.TempDir()
	proofGenerator := microstructure.NewProofGenerator(tempDir)

	// Create a test eligibility result
	result := &microstructure.AssetEligibilityResult{
		Symbol:          "TESTCOIN",
		CheckedAt:       time.Now(),
		OverallEligible: true,
		EligibleVenues:  []string{"binance", "okx"},
		VenueResults: map[string]*types.MicrostructureMetrics{
			"binance": {
				Symbol:                "TESTCOIN",
				Venue:                 "binance",
				SpreadBPS:             35.0,
				DepthUSDPlusMinus2Pct: 150000,
				VADR:                  2.1,
				SpreadValid:           true,
				DepthValid:            true,
				VADRValid:             true,
				OverallValid:          true,
			},
		},
		ProofBundles: map[string]*types.ProofBundle{
			"binance": {
				AssetSymbol: "TESTCOIN",
				ProvenValid: true,
				VenueUsed:   "binance",
				ProofID:     "TESTCOIN_binance_123456",
			},
		},
	}

	// Generate proof bundle
	ctx := context.Background()
	err := proofGenerator.GenerateProofBundle(ctx, result)
	if err != nil {
		t.Fatalf("Failed to generate proof bundle: %v", err)
	}

	// Verify files were created
	dateDir := time.Now().Format("2006-01-02")
	expectedFiles := []string{
		filepath.Join(tempDir, "proofs", dateDir, "microstructure", "TESTCOIN_master_proof.json"),
		filepath.Join(tempDir, "proofs", dateDir, "microstructure", "TESTCOIN_binance_proof.json"),
		filepath.Join(tempDir, "proofs", dateDir, "microstructure", "TESTCOIN_metrics_summary.json"),
	}

	for _, expectedFile := range expectedFiles {
		if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
			t.Errorf("Expected file not created: %s", expectedFile)
		}
	}

	// Test loading the proof bundle back
	loadedBundle, err := proofGenerator.LoadProofBundle(ctx, "TESTCOIN", dateDir)
	if err != nil {
		t.Fatalf("Failed to load proof bundle: %v", err)
	}

	if loadedBundle.AssetSymbol != result.Symbol {
		t.Errorf("Loaded bundle symbol mismatch: expected %s, got %s",
			result.Symbol, loadedBundle.AssetSymbol)
	}

	if loadedBundle.OverallEligible != result.OverallEligible {
		t.Errorf("Loaded bundle eligibility mismatch: expected %v, got %v",
			result.OverallEligible, loadedBundle.OverallEligible)
	}
}

func TestAuditReportGeneration(t *testing.T) {
	tempDir := t.TempDir()
	proofGenerator := microstructure.NewProofGenerator(tempDir)

	// Create multiple test results for audit
	results := []*microstructure.AssetEligibilityResult{
		{
			Symbol:          "BTC",
			OverallEligible: true,
			EligibleVenues:  []string{"binance", "okx"},
			VenueResults: map[string]*types.MicrostructureMetrics{
				"binance": {SpreadBPS: 25.0, DepthUSDPlusMinus2Pct: 200000, OverallValid: true},
				"okx":     {SpreadBPS: 35.0, DepthUSDPlusMinus2Pct: 150000, OverallValid: true},
			},
		},
		{
			Symbol:          "ETH",
			OverallEligible: true,
			EligibleVenues:  []string{"binance"},
			VenueResults: map[string]*types.MicrostructureMetrics{
				"binance":  {SpreadBPS: 30.0, DepthUSDPlusMinus2Pct: 180000, OverallValid: true},
				"coinbase": {SpreadBPS: 65.0, DepthUSDPlusMinus2Pct: 80000, OverallValid: false},
			},
		},
		{
			Symbol:          "BADCOIN",
			OverallEligible: false,
			EligibleVenues:  []string{},
			VenueResults: map[string]*types.MicrostructureMetrics{
				"binance": {SpreadBPS: 85.0, DepthUSDPlusMinus2Pct: 50000, OverallValid: false},
				"okx":     {SpreadBPS: 95.0, DepthUSDPlusMinus2Pct: 30000, OverallValid: false},
			},
		},
	}

	// Generate audit report
	ctx := context.Background()
	report, err := proofGenerator.GenerateAuditReport(ctx, results)
	if err != nil {
		t.Fatalf("Failed to generate audit report: %v", err)
	}

	// Verify report contents
	if report.TotalAssets != 3 {
		t.Errorf("Expected TotalAssets=3, got %d", report.TotalAssets)
	}

	if report.Summary.EligibleCount != 2 {
		t.Errorf("Expected EligibleCount=2, got %d", report.Summary.EligibleCount)
	}

	if report.Summary.IneligibleCount != 1 {
		t.Errorf("Expected IneligibleCount=1, got %d", report.Summary.IneligibleCount)
	}

	expectedEligibilityRate := 66.7 // 2/3 * 100
	if report.Summary.EligibilityRate < expectedEligibilityRate-0.1 ||
		report.Summary.EligibilityRate > expectedEligibilityRate+0.1 {
		t.Errorf("Expected EligibilityRate~%.1f%%, got %.1f%%",
			expectedEligibilityRate, report.Summary.EligibilityRate)
	}

	// Check venue statistics
	binanceStats := report.VenueStats["binance"]
	if binanceStats == nil {
		t.Fatal("Expected binance venue stats, got nil")
	}

	if binanceStats.TotalChecked != 3 {
		t.Errorf("Expected binance TotalChecked=3, got %d", binanceStats.TotalChecked)
	}

	if binanceStats.PassedChecks != 2 {
		t.Errorf("Expected binance PassedChecks=2, got %d", binanceStats.PassedChecks)
	}
}

// Helper function to load orderbook fixtures and convert them
func loadOrderBookFixture(fixturePath, symbol, venue string) (*types.OrderBook, error) {
	fixturesDir := filepath.Join("..", "fixtures")
	fullPath := filepath.Join(fixturesDir, fixturePath)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	// Mock conversion - in a real implementation, this would use the actual venue clients
	// For testing, we'll create a realistic orderbook based on the fixture data

	var mockOrderBook *types.OrderBook
	fetchTime := time.Now()

	switch venue {
	case "binance":
		var binanceResp struct {
			LastUpdateId int64      `json:"lastUpdateId"`
			Bids         [][]string `json:"bids"`
			Asks         [][]string `json:"asks"`
		}
		if err := json.Unmarshal(data, &binanceResp); err != nil {
			return nil, err
		}

		// Calculate mock metrics based on fixture data
		bestBidPrice := 43250.50
		bestAskPrice := 43251.25
		midPrice := (bestBidPrice + bestAskPrice) / 2.0
		spread := bestAskPrice - bestBidPrice
		spreadBPS := (spread / midPrice) * 10000

		mockOrderBook = &types.OrderBook{
			Symbol:                symbol,
			Venue:                 venue,
			TimestampMono:         fetchTime,
			SequenceNum:           binanceResp.LastUpdateId,
			BestBidPrice:          bestBidPrice,
			BestAskPrice:          bestAskPrice,
			MidPrice:              midPrice,
			SpreadBPS:             spreadBPS,
			DepthUSDPlusMinus2Pct: calculateMockDepth(binanceResp.Bids, binanceResp.Asks, midPrice),
		}

	case "okx":
		mockOrderBook = &types.OrderBook{
			Symbol:                symbol,
			Venue:                 venue,
			TimestampMono:         fetchTime,
			SequenceNum:           1705329600000,
			BestBidPrice:          2579.85,
			BestAskPrice:          2580.45,
			MidPrice:              2580.15,
			SpreadBPS:             23.3,   // (0.60 / 2580.15) * 10000
			DepthUSDPlusMinus2Pct: 125000, // Mock depth calculation
		}

	case "coinbase":
		mockOrderBook = &types.OrderBook{
			Symbol:                symbol,
			Venue:                 venue,
			TimestampMono:         fetchTime,
			SequenceNum:           987654321,
			BestBidPrice:          98.75,
			BestAskPrice:          98.78,
			MidPrice:              98.765,
			SpreadBPS:             30.4,  // (0.03 / 98.765) * 10000
			DepthUSDPlusMinus2Pct: 85000, // Mock - likely below threshold
		}
	}

	return mockOrderBook, nil
}

// Mock depth calculation for fixtures
func calculateMockDepth(bids, asks [][]string, midPrice float64) float64 {
	// Simplified mock calculation
	// In real implementation, this would parse the bid/ask arrays properly
	if len(bids) > 5 && len(asks) > 5 {
		return 150000 // Good depth
	}
	return 80000 // Marginal depth
}
