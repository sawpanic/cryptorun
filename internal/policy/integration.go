package policy

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

// PolicyEnforcer integrates policy validation into scanners and gates
type PolicyEnforcer struct {
	validator *PolicyValidator
}

// NewPolicyEnforcer creates a policy enforcer with default validator
func NewPolicyEnforcer() *PolicyEnforcer {
	return &PolicyEnforcer{
		validator: NewPolicyValidator(),
	}
}

// NewPolicyEnforcerWithValidator creates a policy enforcer with custom validator
func NewPolicyEnforcerWithValidator(validator *PolicyValidator) *PolicyEnforcer {
	return &PolicyEnforcer{
		validator: validator,
	}
}

// ScanRequest represents a scan request that needs policy validation
type ScanRequest struct {
	Symbol     string
	Venue      string
	DataSource string
	DataType   string
	Price      float64
}

// GateRequest represents a gate entry request that needs policy validation
type GateRequest struct {
	Symbol     string
	Venue      string
	DataSource string
	DataType   string
	Price      float64
	Score      float64
	VADR       float64
}

// ValidateScanRequest validates a scan request against all policies
func (pe *PolicyEnforcer) ValidateScanRequest(ctx context.Context, req ScanRequest) error {
	log.Debug().
		Str("symbol", req.Symbol).
		Str("venue", req.Venue).
		Str("data_source", req.DataSource).
		Str("data_type", req.DataType).
		Float64("price", req.Price).
		Msg("Validating scan request")

	return pe.validator.ValidateAll(req.Symbol, req.Venue, req.DataSource, req.DataType, req.Price)
}

// ValidateGateRequest validates a gate entry request against all policies
func (pe *PolicyEnforcer) ValidateGateRequest(ctx context.Context, req GateRequest) error {
	log.Debug().
		Str("symbol", req.Symbol).
		Str("venue", req.Venue).
		Str("data_source", req.DataSource).
		Str("data_type", req.DataType).
		Float64("price", req.Price).
		Float64("score", req.Score).
		Float64("vadr", req.VADR).
		Msg("Validating gate request")

	// Run all policy validations
	if err := pe.validator.ValidateAll(req.Symbol, req.Venue, req.DataSource, req.DataType, req.Price); err != nil {
		return fmt.Errorf("policy validation failed: %w", err)
	}

	log.Info().
		Str("symbol", req.Symbol).
		Str("venue", req.Venue).
		Float64("score", req.Score).
		Float64("vadr", req.VADR).
		Msg("Gate request passed policy validation")
	return nil
}

// ScannerIntegration provides policy hooks for scanners
type ScannerIntegration struct {
	enforcer *PolicyEnforcer
}

// NewScannerIntegration creates scanner policy integration
func NewScannerIntegration(enforcer *PolicyEnforcer) *ScannerIntegration {
	return &ScannerIntegration{
		enforcer: enforcer,
	}
}

// PreScanValidation validates before starting a scan
func (si *ScannerIntegration) PreScanValidation(ctx context.Context, symbols []string, venue string) error {
	log.Info().
		Strs("symbols", symbols).
		Str("venue", venue).
		Msg("Running pre-scan policy validation")

	for _, symbol := range symbols {
		req := ScanRequest{
			Symbol:     symbol,
			Venue:      venue,
			DataSource: venue,
			DataType:   "scan",
			Price:      1.0, // Default for validation
		}

		if err := si.enforcer.ValidateScanRequest(ctx, req); err != nil {
			log.Error().
				Str("symbol", symbol).
				Str("venue", venue).
				Err(err).
				Msg("Pre-scan validation failed")
			return err
		}
	}

	log.Info().
		Int("symbols_count", len(symbols)).
		Str("venue", venue).
		Msg("Pre-scan policy validation passed")
	return nil
}

// ValidateSymbolForScanning validates individual symbol during scanning
func (si *ScannerIntegration) ValidateSymbolForScanning(ctx context.Context, symbol, venue string, price float64) error {
	req := ScanRequest{
		Symbol:     symbol,
		Venue:      venue,
		DataSource: venue,
		DataType:   "price_data",
		Price:      price,
	}

	return si.enforcer.ValidateScanRequest(ctx, req)
}

// GateIntegration provides policy hooks for entry gates
type GateIntegration struct {
	enforcer *PolicyEnforcer
}

// NewGateIntegration creates gate policy integration
func NewGateIntegration(enforcer *PolicyEnforcer) *GateIntegration {
	return &GateIntegration{
		enforcer: enforcer,
	}
}

// ValidateEntryGate validates entry gate conditions with policy checks
func (gi *GateIntegration) ValidateEntryGate(ctx context.Context, symbol, venue string, score, vadr, price float64) error {
	req := GateRequest{
		Symbol:     symbol,
		Venue:      venue,
		DataSource: venue,
		DataType:   "market_data",
		Price:      price,
		Score:      score,
		VADR:       vadr,
	}

	return gi.enforcer.ValidateGateRequest(ctx, req)
}

// ValidateMicrostructureData validates microstructure data source compliance
func (gi *GateIntegration) ValidateMicrostructureData(ctx context.Context, symbol, venue, dataSource string) error {
	// Focus on aggregator ban validation for microstructure
	return gi.enforcer.validator.ValidateAggregatorBan(dataSource, "depth")
}

// GlobalPolicyManager provides global policy control interface
type GlobalPolicyManager struct {
	enforcer *PolicyEnforcer
}

// NewGlobalPolicyManager creates global policy manager
func NewGlobalPolicyManager() *GlobalPolicyManager {
	return &GlobalPolicyManager{
		enforcer: NewPolicyEnforcer(),
	}
}

// SetGlobalPause activates/deactivates global trading pause
func (gpm *GlobalPolicyManager) SetGlobalPause(paused bool) {
	gpm.enforcer.validator.SetGlobalPause(paused)
}

// AddToBlacklist adds symbol to global blacklist
func (gpm *GlobalPolicyManager) AddToBlacklist(symbol string) {
	gpm.enforcer.validator.AddToBlacklist(symbol)
}

// RemoveFromBlacklist removes symbol from global blacklist
func (gpm *GlobalPolicyManager) RemoveFromBlacklist(symbol string) {
	gpm.enforcer.validator.RemoveFromBlacklist(symbol)
}

// SetEmergencyControl sets venue-specific emergency control
func (gpm *GlobalPolicyManager) SetEmergencyControl(venue, symbol string, active bool) {
	gpm.enforcer.validator.SetEmergencyControl(venue, symbol, active)
}

// GetPolicyStatus returns current policy status
func (gpm *GlobalPolicyManager) GetPolicyStatus() map[string]interface{} {
	return gpm.enforcer.validator.GetStatus()
}

// GetEnforcer returns the underlying policy enforcer
func (gpm *GlobalPolicyManager) GetEnforcer() *PolicyEnforcer {
	return gpm.enforcer
}

// GetScannerIntegration returns scanner integration instance
func (gpm *GlobalPolicyManager) GetScannerIntegration() *ScannerIntegration {
	return NewScannerIntegration(gpm.enforcer)
}

// GetGateIntegration returns gate integration instance
func (gpm *GlobalPolicyManager) GetGateIntegration() *GateIntegration {
	return NewGateIntegration(gpm.enforcer)
}