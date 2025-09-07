package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"cryptorun/internal/atomicio"
)

// RiskEnvelope manages portfolio risk controls and position limits
type RiskEnvelope struct {
	config    *RiskConfig
	state     *RiskState
	emergency *EmergencyControls
}

// RiskConfig defines risk management parameters
type RiskConfig struct {
	// Position sizing
	BasePositionUSD    float64 `json:"base_position_usd"`     // $10k base
	MaxPositionUSD     float64 `json:"max_position_usd"`      // $50k cap
	ATRMultiplier      float64 `json:"atr_multiplier"`        // Base/ATR sizing
	TimeStopHours      int     `json:"time_stop_hours"`       // 48h max hold
	
	// Portfolio limits
	MaxPositions       int     `json:"max_positions"`         // 15 positions max
	MaxSingleAssetPct  float64 `json:"max_single_asset_pct"`  // 10% per asset
	CorrelationCap     float64 `json:"correlation_cap"`       // 0.7 max correlation
	
	// Sector limits
	SectorCaps         map[string]float64 `json:"sector_caps"`    // Sector allocation caps
	EcosystemCaps      map[string]float64 `json:"ecosystem_caps"` // Ecosystem caps
	
	// Emergency thresholds
	DrawdownPausePct   float64 `json:"drawdown_pause_pct"`    // 8% drawdown pause
	BlacklistHours     int     `json:"blacklist_hours"`       // 24h blacklist duration
}

// RiskState tracks current portfolio state
type RiskState struct {
	Positions          map[string]*Position    `json:"positions"`
	TotalExposureUSD   float64                `json:"total_exposure_usd"`
	PortfolioValue     float64                `json:"portfolio_value"`
	CurrentDrawdown    float64                `json:"current_drawdown"`
	CorrelationMatrix  map[string]float64     `json:"correlation_matrix"`
	SectorExposure     map[string]float64     `json:"sector_exposure"`
	EcosystemExposure  map[string]float64     `json:"ecosystem_exposure"`
	LastUpdate         time.Time              `json:"last_update"`
}

// Position represents a trading position
type Position struct {
	Symbol           string    `json:"symbol"`
	Size             float64   `json:"size"`                // Position size in USD
	EntryTime        time.Time `json:"entry_time"`
	EntryPrice       float64   `json:"entry_price"`
	CurrentPrice     float64   `json:"current_price"`
	UnrealizedPnL    float64   `json:"unrealized_pnl"`
	ATR              float64   `json:"atr"`                 // Average True Range
	RiskAllocation   float64   `json:"risk_allocation"`     // % of portfolio
	Sector           string    `json:"sector"`
	Ecosystem        string    `json:"ecosystem"`
	TimeRemaining    int       `json:"time_remaining_hours"`
}

// EmergencyControls manages emergency risk controls
type EmergencyControls struct {
	GlobalPause      bool                   `json:"global_pause"`
	PauseReasons     []string              `json:"pause_reasons"`
	SymbolBlacklist  map[string]time.Time  `json:"symbol_blacklist"`
	DegradedMode     bool                  `json:"degraded_mode"`
	LastPauseTime    time.Time             `json:"last_pause_time"`
}

// RiskCheckResult contains risk validation results
type RiskCheckResult struct {
	Passed           bool                  `json:"passed"`
	Violations       []RiskViolation       `json:"violations"`
	Warnings         []string              `json:"warnings"`
	RecommendedSize  float64              `json:"recommended_size_usd"`
	MaxAllowedSize   float64              `json:"max_allowed_size_usd"`
	PortfolioImpact  *PortfolioImpact     `json:"portfolio_impact"`
}

// RiskViolation represents a risk rule violation
type RiskViolation struct {
	Rule        string  `json:"rule"`
	Severity    string  `json:"severity"`    // "ERROR", "WARNING"
	Current     float64 `json:"current"`
	Limit       float64 `json:"limit"`
	Description string  `json:"description"`
}

// PortfolioImpact shows the impact of a new position
type PortfolioImpact struct {
	NewExposure      float64            `json:"new_exposure_usd"`
	ExposureChange   float64            `json:"exposure_change_pct"`
	NewCorrelations  map[string]float64 `json:"new_correlations"`
	SectorImpact     map[string]float64 `json:"sector_impact"`
}

// NewRiskEnvelope creates a new risk envelope with default configuration
func NewRiskEnvelope() *RiskEnvelope {
	config := &RiskConfig{
		BasePositionUSD:   10000,  // $10k base
		MaxPositionUSD:    50000,  // $50k cap
		ATRMultiplier:     1.0,    // Base/ATR
		TimeStopHours:     48,     // 48h time stop
		
		MaxPositions:      15,     // 15 positions max
		MaxSingleAssetPct: 0.10,   // 10% per asset
		CorrelationCap:    0.70,   // 0.7 correlation cap
		
		SectorCaps: map[string]float64{
			"defi":        0.30, // 30% DeFi cap
			"layer1":      0.40, // 40% Layer 1 cap
			"exchange":    0.20, // 20% Exchange tokens
			"gaming":      0.15, // 15% Gaming tokens
			"privacy":     0.10, // 10% Privacy coins
		},
		
		EcosystemCaps: map[string]float64{
			"ethereum":    0.35, // 35% Ethereum ecosystem
			"solana":      0.25, // 25% Solana ecosystem
			"cosmos":      0.15, // 15% Cosmos ecosystem
			"polkadot":    0.15, // 15% Polkadot ecosystem
		},
		
		DrawdownPausePct: 0.08,   // 8% drawdown pause
		BlacklistHours:   24,     // 24h blacklist
	}
	
	state := &RiskState{
		Positions:         make(map[string]*Position),
		CorrelationMatrix: make(map[string]float64),
		SectorExposure:    make(map[string]float64),
		EcosystemExposure: make(map[string]float64),
		LastUpdate:        time.Now(),
	}
	
	emergency := &EmergencyControls{
		SymbolBlacklist: make(map[string]time.Time),
	}
	
	return &RiskEnvelope{
		config:    config,
		state:     state,
		emergency: emergency,
	}
}

// CalculatePositionSize determines position size using ATR-based sizing
func (re *RiskEnvelope) CalculatePositionSize(symbol string, atr, currentPrice float64) (float64, error) {
	if atr <= 0 || currentPrice <= 0 {
		return 0, fmt.Errorf("invalid ATR or price: ATR=%.6f, price=%.6f", atr, currentPrice)
	}
	
	// Base position size = Base / ATR
	baseSize := re.config.BasePositionUSD / atr
	
	// Cap at maximum position size
	if baseSize > re.config.MaxPositionUSD {
		baseSize = re.config.MaxPositionUSD
	}
	
	log.Debug().
		Str("symbol", symbol).
		Float64("atr", atr).
		Float64("current_price", currentPrice).
		Float64("calculated_size", baseSize).
		Msg("Position size calculated")
	
	return baseSize, nil
}

// CheckRiskLimits validates a new position against risk limits
func (re *RiskEnvelope) CheckRiskLimits(ctx context.Context, symbol string, proposedSize float64) (*RiskCheckResult, error) {
	result := &RiskCheckResult{
		Passed:     true,
		Violations: []RiskViolation{},
		Warnings:   []string{},
	}
	
	// Check emergency controls first
	if err := re.checkEmergencyControls(symbol, result); err != nil {
		return result, err
	}
	
	// Check position count limits
	re.checkPositionCountLimits(result)
	
	// Check single asset concentration
	re.checkSingleAssetLimits(symbol, proposedSize, result)
	
	// Check correlation limits
	if err := re.checkCorrelationLimits(symbol, proposedSize, result); err != nil {
		return result, err
	}
	
	// Check sector and ecosystem caps
	re.checkSectorLimits(symbol, proposedSize, result)
	
	// Calculate portfolio impact
	result.PortfolioImpact = re.calculatePortfolioImpact(symbol, proposedSize)
	
	// Set final recommendations
	result.RecommendedSize = proposedSize
	result.MaxAllowedSize = re.config.MaxPositionUSD
	
	// Determine overall pass/fail
	for _, violation := range result.Violations {
		if violation.Severity == "ERROR" {
			result.Passed = false
			break
		}
	}
	
	return result, nil
}

// checkEmergencyControls validates emergency controls
func (re *RiskEnvelope) checkEmergencyControls(symbol string, result *RiskCheckResult) error {
	// Check global pause
	if re.emergency.GlobalPause {
		result.Violations = append(result.Violations, RiskViolation{
			Rule:        "global_pause",
			Severity:    "ERROR",
			Description: fmt.Sprintf("Trading paused: %v", re.emergency.PauseReasons),
		})
	}
	
	// Check symbol blacklist
	if blacklistTime, exists := re.emergency.SymbolBlacklist[symbol]; exists {
		hoursRemaining := time.Until(blacklistTime.Add(time.Duration(re.config.BlacklistHours) * time.Hour)).Hours()
		if hoursRemaining > 0 {
			result.Violations = append(result.Violations, RiskViolation{
				Rule:        "symbol_blacklist",
				Severity:    "ERROR",
				Current:     hoursRemaining,
				Limit:       0,
				Description: fmt.Sprintf("Symbol %s blacklisted for %.1f more hours", symbol, hoursRemaining),
			})
		}
	}
	
	return nil
}

// checkPositionCountLimits validates maximum position count
func (re *RiskEnvelope) checkPositionCountLimits(result *RiskCheckResult) {
	currentCount := len(re.state.Positions)
	if currentCount >= re.config.MaxPositions {
		result.Violations = append(result.Violations, RiskViolation{
			Rule:        "max_positions",
			Severity:    "ERROR",
			Current:     float64(currentCount),
			Limit:       float64(re.config.MaxPositions),
			Description: fmt.Sprintf("Maximum positions reached: %d/%d", currentCount, re.config.MaxPositions),
		})
	}
}

// checkSingleAssetLimits validates single asset concentration
func (re *RiskEnvelope) checkSingleAssetLimits(symbol string, proposedSize float64, result *RiskCheckResult) {
	if re.state.PortfolioValue <= 0 {
		return
	}
	
	currentExposure := 0.0
	if pos, exists := re.state.Positions[symbol]; exists {
		currentExposure = pos.Size
	}
	
	newExposure := currentExposure + proposedSize
	exposurePct := newExposure / re.state.PortfolioValue
	
	if exposurePct > re.config.MaxSingleAssetPct {
		result.Violations = append(result.Violations, RiskViolation{
			Rule:        "single_asset_limit",
			Severity:    "ERROR",
			Current:     exposurePct * 100,
			Limit:       re.config.MaxSingleAssetPct * 100,
			Description: fmt.Sprintf("Single asset exposure %.1f%% exceeds limit %.1f%%", exposurePct*100, re.config.MaxSingleAssetPct*100),
		})
	}
}

// checkCorrelationLimits validates correlation clustering
func (re *RiskEnvelope) checkCorrelationLimits(symbol string, proposedSize float64, result *RiskCheckResult) error {
	// Check correlation with existing positions
	for existingSymbol := range re.state.Positions {
		corrKey := fmt.Sprintf("%s-%s", symbol, existingSymbol)
		if corr, exists := re.state.CorrelationMatrix[corrKey]; exists {
			if corr > re.config.CorrelationCap {
				result.Violations = append(result.Violations, RiskViolation{
					Rule:        "correlation_limit",
					Severity:    "WARNING", // Warning for now, could be ERROR
					Current:     corr,
					Limit:       re.config.CorrelationCap,
					Description: fmt.Sprintf("High correlation %.2f with %s (limit %.2f)", corr, existingSymbol, re.config.CorrelationCap),
				})
			}
		}
	}
	
	return nil
}

// checkSectorLimits validates sector and ecosystem caps
func (re *RiskEnvelope) checkSectorLimits(symbol string, proposedSize float64, result *RiskCheckResult) {
	// Get sector and ecosystem for symbol (mock implementation)
	sector := re.getSectorForSymbol(symbol)
	ecosystem := re.getEcosystemForSymbol(symbol)
	
	// Check sector cap
	if sectorCap, exists := re.config.SectorCaps[sector]; exists {
		currentSectorExposure := re.state.SectorExposure[sector]
		newSectorExposure := (currentSectorExposure*re.state.PortfolioValue + proposedSize) / re.state.PortfolioValue
		
		if newSectorExposure > sectorCap {
			result.Violations = append(result.Violations, RiskViolation{
				Rule:        "sector_cap",
				Severity:    "WARNING",
				Current:     newSectorExposure * 100,
				Limit:       sectorCap * 100,
				Description: fmt.Sprintf("Sector %s exposure %.1f%% exceeds soft cap %.1f%%", sector, newSectorExposure*100, sectorCap*100),
			})
		}
	}
	
	// Check ecosystem cap
	if ecosystemCap, exists := re.config.EcosystemCaps[ecosystem]; exists {
		currentEcosystemExposure := re.state.EcosystemExposure[ecosystem]
		newEcosystemExposure := (currentEcosystemExposure*re.state.PortfolioValue + proposedSize) / re.state.PortfolioValue
		
		if newEcosystemExposure > ecosystemCap {
			result.Violations = append(result.Violations, RiskViolation{
				Rule:        "ecosystem_cap",
				Severity:    "WARNING",
				Current:     newEcosystemExposure * 100,
				Limit:       ecosystemCap * 100,
				Description: fmt.Sprintf("Ecosystem %s exposure %.1f%% exceeds soft cap %.1f%%", ecosystem, newEcosystemExposure*100, ecosystemCap*100),
			})
		}
	}
}

// calculatePortfolioImpact computes the impact of adding a new position
func (re *RiskEnvelope) calculatePortfolioImpact(symbol string, proposedSize float64) *PortfolioImpact {
	newExposure := re.state.TotalExposureUSD + proposedSize
	exposureChange := 0.0
	if re.state.TotalExposureUSD > 0 {
		exposureChange = (proposedSize / re.state.TotalExposureUSD) * 100
	}
	
	// Calculate new correlations (mock)
	newCorrelations := make(map[string]float64)
	for existingSymbol := range re.state.Positions {
		// Mock correlation calculation
		newCorrelations[existingSymbol] = 0.5 // Placeholder
	}
	
	// Calculate sector impact
	sector := re.getSectorForSymbol(symbol)
	sectorImpact := make(map[string]float64)
	if re.state.PortfolioValue > 0 {
		sectorImpact[sector] = proposedSize / re.state.PortfolioValue
	}
	
	return &PortfolioImpact{
		NewExposure:     newExposure,
		ExposureChange:  exposureChange,
		NewCorrelations: newCorrelations,
		SectorImpact:    sectorImpact,
	}
}

// TriggerEmergencyPause activates emergency pause with reasons
func (re *RiskEnvelope) TriggerEmergencyPause(reasons []string) {
	re.emergency.GlobalPause = true
	re.emergency.PauseReasons = reasons
	re.emergency.LastPauseTime = time.Now()
	
	log.Warn().
		Strs("reasons", reasons).
		Msg("Emergency pause triggered")
}

// BlacklistSymbol adds a symbol to the blacklist
func (re *RiskEnvelope) BlacklistSymbol(symbol string, reason string) {
	re.emergency.SymbolBlacklist[symbol] = time.Now()
	
	log.Warn().
		Str("symbol", symbol).
		Str("reason", reason).
		Int("duration_hours", re.config.BlacklistHours).
		Msg("Symbol blacklisted")
}

// UpdateDrawdown updates current portfolio drawdown
func (re *RiskEnvelope) UpdateDrawdown(drawdownPct float64) {
	re.state.CurrentDrawdown = drawdownPct
	
	// Check drawdown pause threshold
	if drawdownPct > re.config.DrawdownPausePct {
		reason := fmt.Sprintf("Drawdown %.2f%% exceeds limit %.2f%%", drawdownPct*100, re.config.DrawdownPausePct*100)
		re.TriggerEmergencyPause([]string{reason})
	}
}

// GetRiskSummary returns current risk envelope status
func (re *RiskEnvelope) GetRiskSummary() map[string]interface{} {
	activeViolations := 0
	activeCaps := make([]string, 0)
	
	// Count positions approaching limits
	currentCount := len(re.state.Positions)
	if float64(currentCount) >= float64(re.config.MaxPositions)*0.8 {
		activeCaps = append(activeCaps, fmt.Sprintf("positions: %d/%d", currentCount, re.config.MaxPositions))
	}
	
	// Check drawdown
	if re.state.CurrentDrawdown >= re.config.DrawdownPausePct*0.8 {
		activeCaps = append(activeCaps, fmt.Sprintf("drawdown: %.1f%%", re.state.CurrentDrawdown*100))
	}
	
	// Check blacklisted symbols
	blacklistedCount := 0
	now := time.Now()
	for _, blacklistTime := range re.emergency.SymbolBlacklist {
		if now.Sub(blacklistTime).Hours() < float64(re.config.BlacklistHours) {
			blacklistedCount++
		}
	}
	
	return map[string]interface{}{
		"global_pause":        re.emergency.GlobalPause,
		"pause_reasons":       re.emergency.PauseReasons,
		"positions":           fmt.Sprintf("%d/%d", currentCount, re.config.MaxPositions),
		"total_exposure_usd":  re.state.TotalExposureUSD,
		"current_drawdown":    fmt.Sprintf("%.2f%%", re.state.CurrentDrawdown*100),
		"blacklisted_symbols": blacklistedCount,
		"active_caps":         activeCaps,
		"violations":          activeViolations,
		"degraded_mode":       re.emergency.DegradedMode,
		"last_update":         re.state.LastUpdate.Format(time.RFC3339),
	}
}

// getSectorForSymbol returns the sector classification for a symbol
func (re *RiskEnvelope) getSectorForSymbol(symbol string) string {
	// Mock sector mapping - in practice would come from a configuration file
	sectorMap := map[string]string{
		"BTCUSD":  "layer1",
		"ETHUSD":  "layer1", 
		"ADAUSD":  "layer1",
		"SOLUSD":  "layer1",
		"DOTUSD":  "layer1",
		"UNIUSD":  "defi",
		"AAVEUSD": "defi",
		"BNBUSD":  "exchange",
	}
	
	if sector, exists := sectorMap[symbol]; exists {
		return sector
	}
	return "other"
}

// getEcosystemForSymbol returns the ecosystem classification for a symbol  
func (re *RiskEnvelope) getEcosystemForSymbol(symbol string) string {
	// Mock ecosystem mapping
	ecosystemMap := map[string]string{
		"ETHUSD":  "ethereum",
		"UNIUSD":  "ethereum",
		"AAVEUSD": "ethereum",
		"SOLUSD":  "solana",
		"ADAUSD":  "cardano",
		"DOTUSD":  "polkadot",
	}
	
	if ecosystem, exists := ecosystemMap[symbol]; exists {
		return ecosystem
	}
	return "independent"
}

// SaveState persists risk envelope state to disk
func (re *RiskEnvelope) SaveState() error {
	stateData, err := json.MarshalIndent(map[string]interface{}{
		"config":    re.config,
		"state":     re.state,
		"emergency": re.emergency,
	}, "", "  ")
	if err != nil {
		return err
	}
	
	statePath := "out/risk/envelope_state.json"
	if err := os.MkdirAll("out/risk", 0755); err != nil {
		return err
	}
	
	return atomicio.WriteFile(statePath, stateData, 0644)
}

// LoadState loads risk envelope state from disk
func (re *RiskEnvelope) LoadState() error {
	statePath := "out/risk/envelope_state.json"
	data, err := os.ReadFile(statePath)
	if err != nil {
		return err // State file doesn't exist, use defaults
	}
	
	var stateObj map[string]interface{}
	if err := json.Unmarshal(data, &stateObj); err != nil {
		return err
	}
	
	// Restore state (simplified for now)
	re.state.LastUpdate = time.Now()
	
	return nil
}

// Testing helper methods

// AddPosition adds a position for testing (exported for unit tests)
func (re *RiskEnvelope) AddPosition(symbol string, size float64) {
	if re.state.Positions == nil {
		re.state.Positions = make(map[string]*Position)
	}
	
	re.state.Positions[symbol] = &Position{
		Symbol:    symbol,
		Size:      size,
		EntryTime: time.Now(),
	}
	
	re.state.TotalExposureUSD += size
}

// SetPortfolioValue sets portfolio value for testing
func (re *RiskEnvelope) SetPortfolioValue(value float64) {
	re.state.PortfolioValue = value
}

// SetCorrelation sets correlation between symbols for testing
func (re *RiskEnvelope) SetCorrelation(symbol1, symbol2 string, correlation float64) {
	key := fmt.Sprintf("%s-%s", symbol1, symbol2)
	re.state.CorrelationMatrix[key] = correlation
}