package websocket

import (
	"fmt"
	"time"

	"cryptorun/internal/domain"
)

// MicrostructureProvider provides real-time microstructure data for scanner gates
type MicrostructureProvider struct {
	hotsetManager *HotSetManager
}

// NewMicrostructureProvider creates a new microstructure provider
func NewMicrostructureProvider(hotsetManager *HotSetManager) *MicrostructureProvider {
	return &MicrostructureProvider{
		hotsetManager: hotsetManager,
	}
}

// GetMicrostructureInputs returns microstructure data for scanner gate evaluation
func (mp *MicrostructureProvider) GetMicrostructureInputs(symbol string) (*MicroGateInputs, error) {
	if mp.hotsetManager == nil {
		return nil, fmt.Errorf("hotset manager not initialized")
	}

	// Get microstructure metrics from hot set
	metrics, err := mp.hotsetManager.GetMicrostructure(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get microstructure for %s: %w", symbol, err)
	}

	// Convert to scanner gate inputs format
	return &MicroGateInputs{
		Symbol:      symbol,
		Bid:         0, // Not available from metrics, would need last tick
		Ask:         0, // Not available from metrics, would need last tick
		Depth2PcUSD: metrics.DepthUSD2Pct,
		VADR:        metrics.VADR,
		ADVUSD:      0, // Would need to be calculated separately
	}, nil
}

// GetLatestTick returns the most recent tick for a symbol
func (mp *MicrostructureProvider) GetLatestTick(symbol string) (*TickUpdate, error) {
	// This would require storing the latest tick per symbol
	// For now, return an error indicating this needs implementation
	return nil, fmt.Errorf("latest tick access not implemented - would require tick storage")
}

// IsSymbolActive checks if a symbol is actively being tracked
func (mp *MicrostructureProvider) IsSymbolActive(symbol string) bool {
	if mp.hotsetManager == nil {
		return false
	}

	_, err := mp.hotsetManager.GetMicrostructure(symbol)
	return err == nil
}

// GetActiveSymbols returns all symbols currently being tracked
func (mp *MicrostructureProvider) GetActiveSymbols() []string {
	if mp.hotsetManager == nil || mp.hotsetManager.microstructure == nil {
		return []string{}
	}

	allMetrics := mp.hotsetManager.microstructure.GetAllMetrics()
	symbols := make([]string, 0, len(allMetrics))

	for symbol := range allMetrics {
		symbols = append(symbols, symbol)
	}

	return symbols
}

// ValidateMicrostructureGates validates microstructure gates for a symbol
func (mp *MicrostructureProvider) ValidateMicrostructureGates(symbol string, thresholds *domain.MicroGateThresholds) (*domain.MicroGateResults, error) {
	inputs, err := mp.GetMicrostructureInputs(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get inputs for %s: %w", symbol, err)
	}

	// Convert inputs to domain format
	domainInputs := domain.MicroGateInputs{
		Symbol:      inputs.Symbol,
		Bid:         inputs.Bid,
		Ask:         inputs.Ask,
		Depth2PcUSD: inputs.Depth2PcUSD,
		VADR:        inputs.VADR,
		ADVUSD:      int64(inputs.ADVUSD),
	}

	// Use default thresholds if none provided
	if thresholds == nil {
		defaultThresholds := domain.DefaultMicroGateThresholds()
		thresholds = &defaultThresholds
	}

	// Evaluate gates using domain logic
	results := domain.EvaluateMicroGates(domainInputs, *thresholds)

	return &results, nil
}

// MicrostructureHealthCheck returns overall health of microstructure system
func (mp *MicrostructureProvider) MicrostructureHealthCheck() MicrostructureHealthStatus {
	if mp.hotsetManager == nil {
		return MicrostructureHealthStatus{
			Status:  "unhealthy",
			Reason:  "hotset manager not initialized",
			Symbols: 0,
		}
	}

	allMetrics := mp.hotsetManager.microstructure.GetAllMetrics()
	activeSymbols := len(allMetrics)

	if activeSymbols == 0 {
		return MicrostructureHealthStatus{
			Status:  "degraded",
			Reason:  "no active symbols",
			Symbols: 0,
		}
	}

	// Check for stale data
	now := time.Now()
	staleCount := 0
	healthyCount := 0

	for _, metrics := range allMetrics {
		age := now.Sub(metrics.LastUpdate)
		if age > 60*time.Second {
			staleCount++
		} else if metrics.VenueHealth == "healthy" {
			healthyCount++
		}
	}

	staleRatio := float64(staleCount) / float64(activeSymbols)
	healthyRatio := float64(healthyCount) / float64(activeSymbols)

	if staleRatio > 0.5 {
		return MicrostructureHealthStatus{
			Status:  "degraded",
			Reason:  fmt.Sprintf("%.1f%% of symbols have stale data", staleRatio*100),
			Symbols: activeSymbols,
		}
	}

	if healthyRatio < 0.7 {
		return MicrostructureHealthStatus{
			Status:  "degraded",
			Reason:  fmt.Sprintf("only %.1f%% of symbols are healthy", healthyRatio*100),
			Symbols: activeSymbols,
		}
	}

	return MicrostructureHealthStatus{
		Status:  "healthy",
		Reason:  fmt.Sprintf("%d symbols active, %.1f%% healthy", activeSymbols, healthyRatio*100),
		Symbols: activeSymbols,
	}
}

// MicroGateInputs represents inputs for microstructure gate evaluation
type MicroGateInputs struct {
	Symbol      string  `json:"symbol"`
	Bid         float64 `json:"bid"`
	Ask         float64 `json:"ask"`
	Depth2PcUSD float64 `json:"depth_2pc_usd"`
	VADR        float64 `json:"vadr"`
	ADVUSD      float64 `json:"adv_usd"`
}

// MicrostructureHealthStatus represents the health of the microstructure system
type MicrostructureHealthStatus struct {
	Status  string `json:"status"` // "healthy", "degraded", "unhealthy"
	Reason  string `json:"reason"`
	Symbols int    `json:"symbols"`
}
