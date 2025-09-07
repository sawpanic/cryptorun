package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/exchanges/kraken"
	"github.com/sawpanic/cryptorun/internal/data/interfaces"
)

// D3GatesResult represents entry gates evaluation result
type D3GatesResult struct {
	Symbol         string            `json:"symbol"`
	Venue          string            `json:"venue"`
	Price          float64           `json:"price"`
	SpreadBps      float64           `json:"spread_bps"`
	DepthUSD       float64           `json:"depth_usd"`
	VADR           float64           `json:"vadr"`
	MockScore      float64           `json:"mock_score"`
	PassesGates    bool              `json:"passes_gates"`
	GateResults    map[string]bool   `json:"gate_results"`
	FailedGates    []string          `json:"failed_gates"`
	Timestamp      time.Time         `json:"timestamp"`
	DataAge        string            `json:"data_age"`
	Attribution    string            `json:"attribution"`
}

// Entry gates thresholds (from PRD)
const (
	MinScore     = 75.0
	MaxSpreadBps = 50.0
	MinDepthUSD  = 100000.0
	MinVADR      = 1.75
)

func main() {
	ctx := context.Background()
	
	fmt.Println("🎯 D3 Entry Gates Integration Test")
	fmt.Println("Connecting Kraken provider to entry gate system...")
	fmt.Println()

	// Initialize Kraken adapter
	krakenAdapter := kraken.NewAdapter("kraken-d3")
	
	// Check venue health
	health := krakenAdapter.Health()
	fmt.Printf("🏥 Kraken Health: %s (%s)\n", health.Status, health.Recommendation)
	
	if health.Status == "unhealthy" {
		log.Fatal("❌ Kraken unhealthy - aborting")
	}
	
	// Test symbols (subset for demo)
	testSymbols := []string{"BTCUSD", "ETHUSD", "SOLUSD"}
	
	results := make([]D3GatesResult, 0, len(testSymbols))
	
	for i, symbol := range testSymbols {
		fmt.Printf("[%d/%d] Testing %s...\n", i+1, len(testSymbols), symbol)
		
		result, err := evaluateEntryGates(ctx, krakenAdapter, symbol)
		if err != nil {
			fmt.Printf("  ❌ Error: %v\n", err)
			continue
		}
		
		results = append(results, result)
		
		// Display gate status
		gateIcon := "❌"
		if result.PassesGates {
			gateIcon = "✅"
		}
		
		fmt.Printf("  %s Score=%.1f, Spread=%.1fbps, Depth=$%.0fk, VADR=%.2f\n", 
			gateIcon, result.MockScore, result.SpreadBps, result.DepthUSD/1000, result.VADR)
			
		if len(result.FailedGates) > 0 {
			fmt.Printf("    Failed: %v\n", result.FailedGates)
		}
		
		fmt.Println()
		
		// Be respectful to Kraken API
		time.Sleep(1 * time.Second)
	}
	
	// Output summary
	passed := 0
	for _, result := range results {
		if result.PassesGates {
			passed++
		}
	}
	
	fmt.Printf("🏁 D3 Integration Test Complete\n")
	fmt.Printf("📊 Results: %d/%d symbols passed all entry gates\n", passed, len(results))
	
	// Save results
	outputFile := "out/d3_gates_test.json"
	os.MkdirAll("out", 0755)
	
	data, _ := json.MarshalIndent(results, "", "  ")
	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		fmt.Printf("⚠️  Failed to save results: %v\n", err)
	} else {
		fmt.Printf("📁 Results saved to: %s\n", outputFile)
	}
	
	if passed > 0 {
		fmt.Printf("\n🎯 Symbols passing all gates:\n")
		for _, result := range results {
			if result.PassesGates {
				fmt.Printf("  %s: Score=%.1f, Data=%s\n", 
					result.Symbol, result.MockScore, result.Attribution)
			}
		}
	}
}

func evaluateEntryGates(ctx context.Context, exchange *kraken.Adapter, symbol string) (D3GatesResult, error) {
	result := D3GatesResult{
		Symbol:      symbol,
		Venue:       exchange.Name(),
		Timestamp:   time.Now(),
		GateResults: make(map[string]bool),
		FailedGates: make([]string, 0),
	}
	
	// Fetch L2 order book data
	book, err := exchange.GetBookL2(ctx, symbol)
	if err != nil {
		return result, fmt.Errorf("failed to get order book: %w", err)
	}
	
	result.DataAge = time.Since(book.Timestamp).String()
	result.Attribution = fmt.Sprintf("kraken_l2_%s", book.Timestamp.Format("15:04:05"))
	
	// Validate order book has data
	if len(book.Bids) == 0 || len(book.Asks) == 0 {
		return result, fmt.Errorf("empty order book for %s", symbol)
	}
	
	// Calculate microstructure metrics
	bestBid := book.Bids[0].Price
	bestAsk := book.Asks[0].Price
	midPrice := (bestBid + bestAsk) / 2.0
	
	result.Price = midPrice
	
	// 1. Spread check (must be <50bps)
	spread := (bestAsk - bestBid) / midPrice
	result.SpreadBps = spread * 10000
	result.GateResults["spread"] = result.SpreadBps < MaxSpreadBps
	
	// 2. Depth check (must be >$100k within ±2%)
	bidDepth, askDepth := calculateDepthWithinRange(book, midPrice, 2.0)
	result.DepthUSD = (bidDepth + askDepth) / 2.0
	result.GateResults["depth"] = result.DepthUSD >= MinDepthUSD
	
	// 3. VADR estimation (simplified for demo)
	result.VADR = estimateVADRFromBook(book, midPrice)
	result.GateResults["vadr"] = result.VADR >= MinVADR
	
	// 4. Mock score check (would be real composite score in production)
	result.MockScore = generateDeterministicScore(symbol)
	result.GateResults["score"] = result.MockScore >= MinScore
	
	// Determine overall gate status
	result.PassesGates = true
	for gateName, passed := range result.GateResults {
		if !passed {
			result.PassesGates = false
			result.FailedGates = append(result.FailedGates, gateName)
		}
	}
	
	return result, nil
}

func calculateDepthWithinRange(book *interfaces.BookL2, midPrice, percentRange float64) (bidDepth, askDepth float64) {
	lowerBound := midPrice * (1 - percentRange/100)
	upperBound := midPrice * (1 + percentRange/100)
	
	// Sum bid depth within range above lowerBound
	for _, bid := range book.Bids {
		if bid.Price >= lowerBound {
			bidDepth += bid.Price * bid.Size
		}
	}
	
	// Sum ask depth within range below upperBound  
	for _, ask := range book.Asks {
		if ask.Price <= upperBound {
			askDepth += ask.Price * ask.Size
		}
	}
	
	return bidDepth, askDepth
}

func estimateVADRFromBook(book *interfaces.BookL2, midPrice float64) float64 {
	// VADR = Volume Adequacy for Daily Range
	// Simplified estimation based on order book liquidity
	
	bookDepth := len(book.Bids) + len(book.Asks)
	if bookDepth < 10 {
		return 0.5 // Very thin book
	}
	
	// Use total depth as proxy for volume adequacy
	bidDepth, askDepth := calculateDepthWithinRange(book, midPrice, 1.0)
	totalDepth := bidDepth + askDepth
	
	switch {
	case totalDepth > 1000000: // >$1M
		return 3.0
	case totalDepth > 500000:  // >$500k
		return 2.5
	case totalDepth > 200000:  // >$200k
		return 2.0
	case totalDepth > 100000:  // >$100k
		return 1.8
	default:
		return 1.2
	}
}

func generateDeterministicScore(symbol string) float64 {
	// Generate consistent mock score for testing
	seed := int64(0)
	for _, char := range symbol {
		seed += int64(char)
	}
	
	baseScore := 60.0 + float64(seed%30)
	
	// Boost major pairs
	if symbol == "BTCUSD" || symbol == "ETHUSD" {
		baseScore += 20.0
	}
	
	return baseScore
}