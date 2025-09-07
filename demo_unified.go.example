// demo_unified.go - Demonstrates the unified composite scoring model
package main

import (
	"fmt"
	"time"

	"github.com/sawpanic/cryptorun/internal/score/composite"
)

func main() {
	fmt.Println("Unified Composite Scoring Demo")
	fmt.Println("==============================")

	// Create sample scoring input
	input := composite.ScoringInput{
		Symbol:    "BTCUSD",
		Timestamp: time.Now(),

		// Momentum factors (strong momentum case)
		Momentum1h:  0.03, // 3% 1h return
		Momentum4h:  0.08, // 8% 4h return
		Momentum12h: 0.12, // 12% 12h return
		Momentum24h: 0.18, // 18% 24h return
		Momentum7d:  0.05, // 5% 7d return

		// Technical factors
		RSI4h:    65.0, // Bullish but not overbought
		ADX1h:    45.0, // Strong trend
		HurstExp: 0.65, // Trending market

		// Volume factors
		VolumeSurge: 2.8,  // 2.8× volume surge
		DeltaOI:     0.15, // 15% OI increase

		// Quality factors
		OIAbsolute:   250000, // Strong OI
		ReserveRatio: 0.92,   // Good reserves
		ETFFlows:     75000,  // ETF inflows
		VenueHealth:  0.88,   // Good venue health

		// Social factors
		SocialScore: 6.5, // Moderate social interest
		BrandScore:  4.0, // Moderate brand score

		Regime: "normal", // Normal market regime
	}

	fmt.Printf("Input: %s in %s regime\n", input.Symbol, input.Regime)
	fmt.Printf("Momentum: 1h=%.1f%% 4h=%.1f%% 12h=%.1f%% 24h=%.1f%%\n",
		input.Momentum1h*100, input.Momentum4h*100, input.Momentum12h*100, input.Momentum24h*100)

	// Create unified scorer
	scorer := composite.NewUnifiedScorer()

	// Score the input
	result := scorer.Score(input)

	fmt.Println("\n📊 Unified Composite Score Results:")
	fmt.Printf("  MomentumCore:    %.2f (protected factor)\n", result.MomentumCore)
	fmt.Printf("  TechnicalResid:  %.2f (post-momentum residual)\n", result.TechnicalResid)
	fmt.Printf("  VolumeResid:     %.2f (volume %.2f + OI %.2f)\n",
		result.VolumeResid.Combined, result.VolumeResid.Volume, result.VolumeResid.DeltaOI)
	fmt.Printf("  QualityResid:    %.2f (OI+reserves+ETF+venue)\n", result.QualityResid.Combined)
	fmt.Printf("  SocialResid:     %.2f (capped at +10)\n", result.SocialResid)

	fmt.Printf("\n🎯 Final Scores:\n")
	fmt.Printf("  Internal (0-100): %.2f\n", result.Internal0to100)
	fmt.Printf("  With Social:      %.2f (max 110)\n", result.FinalWithSocial)

	// Validate the score
	err := result.Validate()
	if err != nil {
		fmt.Printf("❌ Validation failed: %v\n", err)
	} else {
		fmt.Printf("✅ Score validation passed\n")
	}

	// Test entry gates
	fmt.Println("\n🛡️ Hard Entry Gates:")
	gates := composite.NewHardEntryGates()

	gateInput := composite.GateInput{
		Symbol:         input.Symbol,
		Timestamp:      time.Now(),
		CompositeScore: result.Internal0to100, // Use internal score (before social)
		VADR:           2.2,                   // Strong VADR
		FundingZScore:  -0.3,                  // Negative funding (divergence present)
		BarAge:         1,                     // Fresh data
		ATRDistance:    1.0,                   // Within ATR limits
		ATRCurrent:     150.0,
		SignalTime:     time.Now().Add(-15 * time.Second),
		ExecutionTime:  time.Now(),
		SpreadBps:      30.0,                    // Good spread
		DepthUSD:       250000,                  // Good depth
		MicroVADR:      1.85,                    // Good micro VADR
		Momentum24h:    input.Momentum24h * 100, // 18% momentum
		RSI4h:          input.RSI4h,             // 65 RSI
	}

	gateResult := gates.EvaluateAll(gateInput)

	if gateResult.Allowed {
		fmt.Printf("  ✅ Entry ALLOWED: All gates passed\n")
	} else {
		fmt.Printf("  ❌ Entry BLOCKED: %s\n", gateResult.Reason)
	}

	fmt.Printf("  Gates Summary: %d/%d passed\n",
		len(gateResult.GatesPassed)-countFalse(gateResult.GatesPassed),
		len(gateResult.GatesPassed))

	// Show key gate results
	fmt.Printf("  - Composite Score ≥75: %v (%.1f)\n",
		gateResult.GatesPassed["composite_score"], result.Internal0to100)
	fmt.Printf("  - VADR ≥1.8×: %v (%.2f)\n",
		gateResult.GatesPassed["vadr"], gateInput.VADR)
	fmt.Printf("  - Funding Divergence: %v (z=%.2f)\n",
		gateResult.GatesPassed["funding_divergence"], gateInput.FundingZScore)

	fmt.Println("\n🎉 MODEL.UNIFY.COMPOSITE.V1 Demo Complete")
	fmt.Printf("Single unified scoring path with MomentumCore protection and hard entry gates.\n")
}

func countFalse(m map[string]bool) int {
	count := 0
	for _, v := range m {
		if !v {
			count++
		}
	}
	return count
}
