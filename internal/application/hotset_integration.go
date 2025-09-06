package application

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"cryptorun/internal/domain"
	"cryptorun/internal/infrastructure/websocket"
)

// HotSetIntegration manages the integration between hot set WebSocket data and scanner
type HotSetIntegration struct {
	hotsetManager          *websocket.HotSetManager
	microstructureProvider *websocket.MicrostructureProvider
	config                 *websocket.HotSetConfig
	isRunning              bool
}

// NewHotSetIntegration creates a new hot set integration
func NewHotSetIntegration(config *websocket.HotSetConfig) *HotSetIntegration {
	hotsetManager := websocket.NewHotSetManager(config)
	microstructureProvider := websocket.NewMicrostructureProvider(hotsetManager)

	return &HotSetIntegration{
		hotsetManager:          hotsetManager,
		microstructureProvider: microstructureProvider,
		config:                 config,
	}
}

// StartHotSet starts the WebSocket hot set system for the given symbols
func (hsi *HotSetIntegration) StartHotSet(symbols []string) error {
	if hsi.isRunning {
		return fmt.Errorf("hot set is already running")
	}

	log.Info().Int("symbols", len(symbols)).Msg("Starting hot set integration")

	err := hsi.hotsetManager.Start(symbols)
	if err != nil {
		return fmt.Errorf("failed to start hot set manager: %w", err)
	}

	hsi.isRunning = true

	log.Info().Msg("Hot set integration started successfully")
	return nil
}

// StopHotSet stops the WebSocket hot set system
func (hsi *HotSetIntegration) StopHotSet() error {
	if !hsi.isRunning {
		return nil
	}

	log.Info().Msg("Stopping hot set integration")

	err := hsi.hotsetManager.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop hot set manager: %w", err)
	}

	hsi.isRunning = false

	log.Info().Msg("Hot set integration stopped")
	return nil
}

// EvaluateSymbolMicrostructure evaluates microstructure gates for a symbol using real-time data
func (hsi *HotSetIntegration) EvaluateSymbolMicrostructure(symbol string) (*domain.MicroGateResults, error) {
	if !hsi.isRunning {
		return nil, fmt.Errorf("hot set is not running")
	}

	// Use default thresholds
	return hsi.microstructureProvider.ValidateMicrostructureGates(symbol, nil)
}

// EvaluateSymbolMicrostructureWithThresholds evaluates microstructure gates with custom thresholds
func (hsi *HotSetIntegration) EvaluateSymbolMicrostructureWithThresholds(symbol string, thresholds *domain.MicroGateThresholds) (*domain.MicroGateResults, error) {
	if !hsi.isRunning {
		return nil, fmt.Errorf("hot set is not running")
	}

	return hsi.microstructureProvider.ValidateMicrostructureGates(symbol, thresholds)
}

// GetActiveSymbols returns all symbols currently being tracked by the hot set
func (hsi *HotSetIntegration) GetActiveSymbols() []string {
	if !hsi.isRunning {
		return []string{}
	}

	return hsi.microstructureProvider.GetActiveSymbols()
}

// GetMicrostructureHealth returns the overall health of the microstructure system
func (hsi *HotSetIntegration) GetMicrostructureHealth() websocket.MicrostructureHealthStatus {
	return hsi.microstructureProvider.MicrostructureHealthCheck()
}

// GetLatencyMetrics returns latency performance metrics
func (hsi *HotSetIntegration) GetLatencyMetrics() *websocket.LatencyMetricsSummary {
	if !hsi.isRunning || hsi.hotsetManager == nil {
		return nil
	}

	latencyMonitor := hsi.hotsetManager.GetLatencyMonitor()
	if latencyMonitor == nil {
		return nil
	}

	summary := latencyMonitor.GetMetricsSummary()
	return &summary
}

// GetMicrostructureMetrics returns detailed microstructure metrics for a symbol
func (hsi *HotSetIntegration) GetMicrostructureMetrics(symbol string) (*domain.MicrostructureMetrics, error) {
	if !hsi.isRunning {
		return nil, fmt.Errorf("hot set is not running")
	}

	return hsi.hotsetManager.GetMicrostructure(symbol)
}

// FilterSymbolsByMicrostructure filters symbols based on microstructure gate validation
func (hsi *HotSetIntegration) FilterSymbolsByMicrostructure(symbols []string, thresholds *domain.MicroGateThresholds) ([]string, error) {
	if !hsi.isRunning {
		return nil, fmt.Errorf("hot set is not running")
	}

	var validSymbols []string

	for _, symbol := range symbols {
		results, err := hsi.microstructureProvider.ValidateMicrostructureGates(symbol, thresholds)
		if err != nil {
			log.Debug().Err(err).Str("symbol", symbol).Msg("Failed to validate microstructure gates")
			continue
		}

		if results.AllPass {
			validSymbols = append(validSymbols, symbol)
		} else {
			log.Debug().Str("symbol", symbol).Str("reason", results.Reason).Msg("Symbol failed microstructure gates")
		}
	}

	log.Info().Int("input", len(symbols)).Int("valid", len(validSymbols)).Msg("Filtered symbols by microstructure")

	return validSymbols, nil
}

// SubscribeToTicks returns a channel for receiving real-time tick updates
func (hsi *HotSetIntegration) SubscribeToTicks() <-chan *websocket.TickUpdate {
	if !hsi.isRunning {
		// Return a closed channel
		ch := make(chan *websocket.TickUpdate)
		close(ch)
		return ch
	}

	return hsi.hotsetManager.Subscribe()
}

// IsRunning returns whether the hot set system is currently running
func (hsi *HotSetIntegration) IsRunning() bool {
	return hsi.isRunning
}

// GetConfig returns the hot set configuration
func (hsi *HotSetIntegration) GetConfig() *websocket.HotSetConfig {
	return hsi.config
}
