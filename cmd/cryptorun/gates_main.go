package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/sawpanic/cryptorun/internal/domain/gates"
)

// runGatesExplain handles the gates explain CLI command
func runGatesExplain(cmd *cobra.Command, args []string) error {
	// Get required symbol
	symbol, _ := cmd.Flags().GetString("symbol")
	if symbol == "" {
		return fmt.Errorf("--symbol flag is required")
	}

	// Parse timestamp or use current time
	atStr, _ := cmd.Flags().GetString("at")
	var timestamp time.Time
	if atStr != "" {
		var err error
		timestamp, err = time.Parse(time.RFC3339, atStr)
		if err != nil {
			return fmt.Errorf("invalid --at timestamp format (use RFC3339): %w", err)
		}
	} else {
		timestamp = time.Now()
	}

	// Parse signal and execution times
	signalTimeStr, _ := cmd.Flags().GetString("signal-time")
	executionTimeStr, _ := cmd.Flags().GetString("execution-time")
	
	var signalTime, executionTime time.Time
	if signalTimeStr != "" {
		var err error
		signalTime, err = time.Parse(time.RFC3339, signalTimeStr)
		if err != nil {
			return fmt.Errorf("invalid --signal-time format (use RFC3339): %w", err)
		}
	} else {
		signalTime = timestamp.Add(-30 * time.Second) // Default to 30s ago
	}
	
	if executionTimeStr != "" {
		var err error
		executionTime, err = time.Parse(time.RFC3339, executionTimeStr)
		if err != nil {
			return fmt.Errorf("invalid --execution-time format (use RFC3339): %w", err)
		}
	} else {
		executionTime = timestamp // Default to evaluation time
	}

	// Get all the gate input parameters
	barsAge, _ := cmd.Flags().GetInt("bars-age")
	priceChange, _ := cmd.Flags().GetFloat64("price-change")
	atr1h, _ := cmd.Flags().GetFloat64("atr-1h")
	momentum24h, _ := cmd.Flags().GetFloat64("momentum-24h")
	rsi4h, _ := cmd.Flags().GetFloat64("rsi-4h")
	acceleration, _ := cmd.Flags().GetFloat64("acceleration")
	spreadBps, _ := cmd.Flags().GetFloat64("spread-bps")
	depthUSD, _ := cmd.Flags().GetFloat64("depth-usd")
	vadr, _ := cmd.Flags().GetFloat64("vadr")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	// Build inputs for gate evaluation
	inputs := gates.EvaluateAllGatesInputs{
		Symbol:        symbol,
		Timestamp:     timestamp,
		BarsAge:       barsAge,
		PriceChange:   priceChange,
		ATR1h:         atr1h,
		Momentum24h:   momentum24h,
		RSI4h:         rsi4h,
		Acceleration:  acceleration,
		SignalTime:    signalTime,
		ExecutionTime: executionTime,
	}

	// Add optional microstructure data if provided
	if spreadBps >= 0 {
		inputs.Spread = &spreadBps
	}
	if depthUSD >= 0 {
		inputs.Depth = &depthUSD
	}
	if vadr >= 0 {
		inputs.VADR = &vadr
	}

	// Evaluate all gates
	ctx := context.Background()
	result, err := gates.EvaluateAllGates(ctx, inputs)
	if err != nil {
		return fmt.Errorf("gate evaluation failed: %w", err)
	}

	// Output results
	if jsonOutput {
		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("JSON serialization failed: %w", err)
		}
		fmt.Println(string(jsonData))
	} else {
		explanation := gates.FormatGateExplanation(result)
		fmt.Print(explanation)
		
		// Add example usage if helpful
		if !result.Passed {
			fmt.Print(generateUsageExamples(result))
		}
	}

	return nil
}

// generateUsageExamples provides helpful examples for common gate failures
func generateUsageExamples(result *gates.EvaluateAllGatesResult) string {
	var examples []string
	
	for _, reason := range result.Reasons {
		if !reason.Passed {
			switch reason.Name {
			case "freshness":
				if reason.Message == "stale_bars" {
					examples = append(examples, 
						fmt.Sprintf("  Try with fresher signal: --bars-age 1"))
				}
				if reason.Message == "excessive_move" {
					if atrRatio, ok := reason.Metrics["atr_ratio"]; ok && atrRatio > 1.2 {
						examples = append(examples, 
							fmt.Sprintf("  Price moved %.2fx ATR, reduce --price-change or increase --atr-1h", atrRatio))
					}
				}
			case "fatigue":
				if strings.Contains(reason.Message, "fatigue_block") {
					if momentum, ok := reason.Metrics["momentum_24h"]; ok && momentum > 12 {
						examples = append(examples, 
							fmt.Sprintf("  Override fatigue with stronger acceleration: --acceleration 2.5"))
					}
				}
			case "late_fill":
				if reason.Message == "late_fill" {
					examples = append(examples, 
						fmt.Sprintf("  Use faster execution: --execution-time closer to --signal-time"))
				}
			case "microstructure":
				if strings.Contains(reason.Message, "spread_too_wide") {
					examples = append(examples, 
						fmt.Sprintf("  Reduce spread: --spread-bps 45"))
				}
				if strings.Contains(reason.Message, "insufficient_depth") {
					examples = append(examples, 
						fmt.Sprintf("  Increase depth: --depth-usd 120000"))
				}
				if strings.Contains(reason.Message, "low_vadr") {
					examples = append(examples, 
						fmt.Sprintf("  Increase VADR: --vadr 2.0"))
				}
			}
		}
	}
	
	if len(examples) > 0 {
		return fmt.Sprintf("\n💡 To simulate passing gates, try:\n%s\n", strings.Join(examples, "\n"))
	}
	
	return ""
}

// parseFloatPtr parses a string to float64 pointer, returns nil for empty/invalid
func parseFloatPtr(s string) *float64 {
	if s == "" || s == "-1" {
		return nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return &f
	}
	return nil
}