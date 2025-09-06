package gates

import (
	"context"
	"fmt"

	"cryptorun/internal/data/venue/binance"
	"cryptorun/internal/data/venue/coinbase"
	"cryptorun/internal/data/venue/okx"
	"cryptorun/internal/domain/microstructure"
)

// MicrostructureGate validates exchange-native microstructure requirements
type MicrostructureGate struct {
	checker        *microstructure.Checker
	proofGenerator *microstructure.ProofGenerator
	venueClients   map[string]microstructure.VenueClient
	enabled        bool
}

// MicrostructureGateConfig configures microstructure validation
type MicrostructureGateConfig struct {
	Enabled          bool     `yaml:"enabled"`
	MaxSpreadBPS     float64  `yaml:"max_spread_bps"`
	MinDepthUSD      float64  `yaml:"min_depth_usd"`
	MinVADR          float64  `yaml:"min_vadr"`
	RequireAllVenues bool     `yaml:"require_all_venues"`
	ArtifactsDir     string   `yaml:"artifacts_dir"`
	EnabledVenues    []string `yaml:"enabled_venues"`
}

// DefaultMicrostructureGateConfig returns PRD-compliant defaults
func DefaultMicrostructureGateConfig() *MicrostructureGateConfig {
	return &MicrostructureGateConfig{
		Enabled:          true,
		MaxSpreadBPS:     50.0,   // < 50 bps spread requirement
		MinDepthUSD:      100000, // >= $100k depth within ±2%
		MinVADR:          1.75,   // >= 1.75× VADR requirement
		RequireAllVenues: false,  // Any venue passing is sufficient
		ArtifactsDir:     "./artifacts",
		EnabledVenues:    []string{"binance", "okx", "coinbase"}, // Kraken preferred but not included yet
	}
}

// NewMicrostructureGate creates a microstructure gate with specified config
func NewMicrostructureGate(config *MicrostructureGateConfig) *MicrostructureGate {
	if config == nil {
		config = DefaultMicrostructureGateConfig()
	}

	// Create checker with microstructure config
	checkerConfig := &microstructure.Config{
		MaxSpreadBPS:     config.MaxSpreadBPS,
		MinDepthUSD:      config.MinDepthUSD,
		MinVADR:          config.MinVADR,
		RequireAllVenues: config.RequireAllVenues,
	}

	checker := microstructure.NewChecker(checkerConfig)
	proofGenerator := microstructure.NewProofGenerator(config.ArtifactsDir)

	// Initialize venue clients based on enabled venues
	venueClients := make(map[string]microstructure.VenueClient)
	for _, venue := range config.EnabledVenues {
		switch venue {
		case "binance":
			venueClients["binance"] = binance.NewOrderBookClient()
		case "okx":
			venueClients["okx"] = okx.NewOrderBookClient()
		case "coinbase":
			venueClients["coinbase"] = coinbase.NewOrderBookClient()
		}
	}

	return &MicrostructureGate{
		checker:        checker,
		proofGenerator: proofGenerator,
		venueClients:   venueClients,
		enabled:        config.Enabled,
	}
}

// Evaluate checks if an asset meets microstructure requirements
func (mg *MicrostructureGate) Evaluate(ctx context.Context, symbol string) (*GateResult, error) {
	result := &GateResult{
		GateName: "microstructure",
		Symbol:   symbol,
		Passed:   false,
		Reason:   "",
		Metadata: make(map[string]interface{}),
	}

	// If disabled, always pass
	if !mg.enabled {
		result.Passed = true
		result.Reason = "microstructure_gate_disabled"
		return result, nil
	}

	// Check asset eligibility across venues
	eligibilityResult, err := mg.checker.CheckAssetEligibility(ctx, symbol, mg.venueClients)
	if err != nil {
		result.Reason = fmt.Sprintf("microstructure_check_failed: %v", err)
		result.Metadata["error"] = err.Error()
		return result, nil
	}

	// Generate proof bundle
	if err := mg.proofGenerator.GenerateProofBundle(ctx, eligibilityResult); err != nil {
		// Log error but don't fail the gate - proofs are for audit only
		result.Metadata["proof_error"] = err.Error()
	}

	// Determine result
	result.Passed = eligibilityResult.OverallEligible
	result.Metadata["eligible_venues"] = eligibilityResult.EligibleVenues
	result.Metadata["venue_count"] = len(eligibilityResult.EligibleVenues)
	result.Metadata["total_venues_checked"] = len(mg.venueClients)

	if result.Passed {
		result.Reason = fmt.Sprintf("microstructure_valid_on_%d_venues", len(eligibilityResult.EligibleVenues))
	} else {
		if len(eligibilityResult.EligibleVenues) == 0 {
			result.Reason = "microstructure_no_eligible_venues"
		} else {
			result.Reason = "microstructure_insufficient_venues"
		}
		result.Metadata["venue_errors"] = eligibilityResult.VenueErrors
	}

	return result, nil
}

// GetConfig returns the current gate configuration
func (mg *MicrostructureGate) GetConfig() *MicrostructureGateConfig {
	// Return basic config - detailed config is internal to checker
	return &MicrostructureGateConfig{
		Enabled:          mg.enabled,
		MaxSpreadBPS:     50.0, // Default values
		MinDepthUSD:      100000,
		MinVADR:          1.75,
		RequireAllVenues: false,
	}
}

// Enable enables or disables the microstructure gate
func (mg *MicrostructureGate) Enable(enabled bool) {
	mg.enabled = enabled
}

// IsEnabled returns whether the gate is currently enabled
func (mg *MicrostructureGate) IsEnabled() bool {
	return mg.enabled
}

// GetVenueStats returns statistics about configured venues
func (mg *MicrostructureGate) GetVenueStats() map[string]interface{} {
	stats := make(map[string]interface{})

	venues := make([]string, 0, len(mg.venueClients))
	for venue := range mg.venueClients {
		venues = append(venues, venue)
	}

	stats["enabled_venues"] = venues
	stats["venue_count"] = len(venues)
	stats["require_all_venues"] = false // Default behavior

	return stats
}

// GateResult represents the result of a gate evaluation
type GateResult struct {
	GateName string                 `json:"gate_name"`
	Symbol   string                 `json:"symbol"`
	Passed   bool                   `json:"passed"`
	Reason   string                 `json:"reason"`
	Metadata map[string]interface{} `json:"metadata"`
}
