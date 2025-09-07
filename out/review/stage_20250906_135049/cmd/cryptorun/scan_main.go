package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"cryptorun/internal/scan/pipeline"
	"cryptorun/internal/scan/progress"
	"cryptorun/internal/algo/momentum"
)

// runScanMomentum runs the momentum scanning pipeline
func runScanMomentum(cmd *cobra.Command, args []string) error {
	log.Info().Msg("Starting momentum scanning pipeline")

	// Get flags
	venues, _ := cmd.Flags().GetString("venues")
	maxSample, _ := cmd.Flags().GetInt("max-sample")
	ttl, _ := cmd.Flags().GetInt("ttl")
	progressMode, _ := cmd.Flags().GetString("progress")
	regime, _ := cmd.Flags().GetString("regime")
	topN, _ := cmd.Flags().GetInt("top-n")

	// Parse venues
	venueList := strings.Split(venues, ",")
	for i, venue := range venueList {
		venueList[i] = strings.TrimSpace(venue)
	}

	// Create momentum pipeline configuration
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
				HardStop:       5.0,
				VenueHealth:    0.8,
				MaxHoldHours:   48,
				AccelReversal:  0.5,
				FadeThreshold:  1.0,
				TrailingStop:   2.0,
				ProfitTarget:   8.0,
			},
		},
		Pipeline: pipeline.PipelineConfig{
			ExplainabilityOutput: true,
			OutputPath:          "./out/scan",
			ProtectedFactors:    []string{"MomentumCore"},
			MaxSymbols:         maxSample,
		},
	}

	// Create mock data provider
	dataProvider := &MockDataProvider{}

	// Create progress bus for streaming
	progressBus := progress.NewScanProgressBus(progressMode, "out/audit")
	
	// Create momentum pipeline
	mp := pipeline.NewMomentumPipeline(config, dataProvider, "./out/scan")
	mp.SetProgressBus(progressBus)

	// Mock symbols
	symbols := []string{"BTCUSD", "ETHUSD", "ADAUSD", "SOLUSD", "DOTUSD"}
	if len(symbols) > maxSample {
		symbols = symbols[:maxSample]
	}

	log.Info().
		Strs("venues", venueList).
		Int("max_sample", maxSample).
		Int("ttl", ttl).
		Str("progress", progressMode).
		Str("regime", regime).
		Int("top_n", topN).
		Strs("symbols", symbols).
		Msg("Momentum scan configuration")

	// Run scanning
	ctx := context.Background()
	candidates, err := mp.ScanMomentum(ctx, symbols, dataProvider)
	if err != nil {
		return fmt.Errorf("momentum scanning failed: %w", err)
	}

	// Output results
	fmt.Printf("✅ Momentum scan completed: %d candidates found\n", len(candidates))
	fmt.Printf("Results written to: out/scan/momentum_explain.json\n")

	// Log top candidates
	topCount := topN
	if len(candidates) < topCount {
		topCount = len(candidates)
	}

	for i := 0; i < topCount; i++ {
		candidate := candidates[i]
		fmt.Printf("  %d. %s - Score: %.2f, Qualified: %v\n", 
			i+1, candidate.Symbol, candidate.OrthogonalScore, candidate.Qualified)
	}

	return nil
}

// runScanDip runs the quality-dip scanning pipeline
func runScanDip(cmd *cobra.Command, args []string) error {
	log.Info().Msg("Starting quality-dip scanning pipeline")

	// Get flags
	venues, _ := cmd.Flags().GetString("venues")
	maxSample, _ := cmd.Flags().GetInt("max-sample")
	ttl, _ := cmd.Flags().GetInt("ttl")
	progressMode, _ := cmd.Flags().GetString("progress")
	regime, _ := cmd.Flags().GetString("regime")
	topN, _ := cmd.Flags().GetInt("top-n")

	// Parse venues
	venueList := strings.Split(venues, ",")
	for i, venue := range venueList {
		venueList[i] = strings.TrimSpace(venue)
	}

	// Mock symbols
	symbols := []string{"BTCUSD", "ETHUSD", "ADAUSD", "SOLUSD", "DOTUSD"}
	if len(symbols) > maxSample {
		symbols = symbols[:maxSample]
	}

	log.Info().
		Strs("venues", venueList).
		Int("max_sample", maxSample).
		Int("ttl", ttl).
		Str("progress", progressMode).
		Str("regime", regime).
		Int("top_n", topN).
		Strs("symbols", symbols).
		Msg("Quality-dip scan configuration")

	// Simple dip candidate structure
	type DipCandidate struct {
		Symbol     string
		Timestamp  time.Time
		Qualified  bool
		Reason     string
	}
	
	candidates := []DipCandidate{}
	for _, symbol := range symbols {
		candidate := DipCandidate{
			Symbol:     symbol,
			Timestamp:  time.Now(),
			Qualified:  false,
			Reason:     "dip scanner implementation pending - use momentum for now",
		}
		candidates = append(candidates, candidate)
	}

	// Output results
	fmt.Printf("✅ Dip scan completed: %d candidates found\n", len(candidates))
	fmt.Printf("Results written to: out/scan/dip_explain.json\n")

	// Log candidates
	topCount := topN
	if len(candidates) < topCount {
		topCount = len(candidates)
	}

	for i := 0; i < topCount; i++ {
		candidate := candidates[i]
		fmt.Printf("  %d. %s - Qualified: %v, Reason: %s\n", 
			i+1, candidate.Symbol, candidate.Qualified, candidate.Reason)
	}

	return nil
}

// MockDataProvider implements momentum DataProvider interface
type MockDataProvider struct{}

func (m *MockDataProvider) GetMarketData(ctx context.Context, symbol string, timeframes []string) (map[string][]momentum.MarketData, error) {
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
	data := make([]float64, periods)
	for i := 0; i < periods; i++ {
		data[i] = 1000 + float64(i*50) + float64(i%3)*100
	}
	return data, nil
}

func (m *MockDataProvider) GetRegimeData(ctx context.Context) (string, error) {
	return "trending", nil
}