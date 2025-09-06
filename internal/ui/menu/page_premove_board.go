package menu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"cryptorun/src/application/premove"
)

// PreMoveBoardState represents the current state of the premove detection system
type PreMoveBoardState struct {
	LastUpdate       time.Time                 `json:"last_update"`
	ActiveCandidates []premove.Candidate       `json:"active_candidates"`
	RecentAlerts     []premove.AlertRecord     `json:"recent_alerts"`
	PortfolioStatus  map[string]interface{}    `json:"portfolio_status"`
	ExecutionMetrics *premove.ExecutionMetrics `json:"execution_metrics"`
	SystemHealth     map[string]interface{}    `json:"system_health"`
	IsLive           bool                      `json:"is_live"`
	UpdateSequence   int64                     `json:"update_sequence"`
}

// PreMoveBoardUI manages the real-time premove detection dashboard
type PreMoveBoardUI struct {
	mu             sync.RWMutex
	currentState   PreMoveBoardState
	lastUpdate     time.Time
	updateSequence int64

	// SSE throttling (≤1 Hz as required)
	sseClients    map[chan []byte]bool
	sseThrottler  *time.Ticker
	lastSSEUpdate time.Time

	// Background refresh
	refreshTicker *time.Ticker
	ctx           context.Context
	cancel        context.CancelFunc

	// Dependencies
	runner *premove.Runner
}

// NewPreMoveBoardUI creates a new premove board with SSE throttling
func NewPreMoveBoardUI(runner *premove.Runner) *PreMoveBoardUI {
	ctx, cancel := context.WithCancel(context.Background())

	ui := &PreMoveBoardUI{
		sseClients: make(map[chan []byte]bool),
		ctx:        ctx,
		cancel:     cancel,
		runner:     runner,
		currentState: PreMoveBoardState{
			LastUpdate:     time.Now(),
			IsLive:         false,
			UpdateSequence: 0,
		},
	}

	// Initialize SSE throttler at 1 Hz (every 1000ms)
	ui.sseThrottler = time.NewTicker(1000 * time.Millisecond)
	ui.refreshTicker = time.NewTicker(5 * time.Second) // Internal refresh at 5s

	// Start background update routine
	go ui.backgroundUpdater()
	go ui.sseThrottledBroadcast()

	return ui
}

// backgroundUpdater periodically fetches fresh data from the premove system
func (ui *PreMoveBoardUI) backgroundUpdater() {
	defer ui.refreshTicker.Stop()

	for {
		select {
		case <-ui.ctx.Done():
			return
		case <-ui.refreshTicker.C:
			ui.refreshState()
		}
	}
}

// sseThrottledBroadcast sends SSE updates at ≤1 Hz as required
func (ui *PreMoveBoardUI) sseThrottledBroadcast() {
	defer ui.sseThrottler.Stop()

	for {
		select {
		case <-ui.ctx.Done():
			return
		case <-ui.sseThrottler.C:
			// Only broadcast if state has changed since last SSE update
			ui.mu.RLock()
			shouldUpdate := ui.lastUpdate.After(ui.lastSSEUpdate)
			ui.mu.RUnlock()

			if shouldUpdate {
				ui.broadcastSSEUpdate()
			}
		}
	}
}

// refreshState updates the board state from premove components
func (ui *PreMoveBoardUI) refreshState() {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	now := time.Now()
	ui.updateSequence++

	// Get current status from runner components
	var newState PreMoveBoardState

	if ui.runner != nil {
		// Get pipeline status
		pipelineStatus := ui.runner.GetPipelineStatus()

		// Get recent execution metrics
		newState.ExecutionMetrics = ui.runner.GetExecutionSummary()

		// Get portfolio constraints and utilization
		existingPositions := make([]premove.Position, 0) // Would come from persistence
		newState.PortfolioStatus = ui.runner.GetPortfolioConstraints(existingPositions)

		// System health from pipeline status
		newState.SystemHealth = pipelineStatus
		newState.IsLive = true
	} else {
		// Simulation mode
		newState = ui.generateSimulatedState()
		newState.IsLive = false
	}

	// Common fields
	newState.LastUpdate = now
	newState.UpdateSequence = ui.updateSequence

	// Update current state
	ui.currentState = newState
	ui.lastUpdate = now

	log.Debug().
		Int64("sequence", ui.updateSequence).
		Time("timestamp", now).
		Bool("is_live", newState.IsLive).
		Msg("PreMove board state refreshed")
}

// generateSimulatedState creates realistic test data for development/demo
func (ui *PreMoveBoardUI) generateSimulatedState() PreMoveBoardState {
	now := time.Now()

	// Simulated candidates
	candidates := []premove.Candidate{
		{
			Symbol:      "BTCUSD",
			Score:       78.5,
			Sector:      "crypto_major",
			Beta:        1.2,
			Size:        1000.0,
			PassedGates: 3,
			GateResults: map[string]bool{
				"freshness": true,
				"vadr":      true,
				"funding":   true,
			},
			Reasons:   []string{"Strong 4h momentum", "Volume surge +180%", "Funding divergence"},
			Timestamp: now.Add(-2 * time.Minute),
		},
		{
			Symbol:      "ETHUSD",
			Score:       82.1,
			Sector:      "crypto_major",
			Beta:        1.4,
			Size:        750.0,
			PassedGates: 2,
			GateResults: map[string]bool{
				"freshness": true,
				"vadr":      false,
				"funding":   true,
			},
			Reasons:   []string{"Multi-timeframe alignment", "ETF flow correlation"},
			Timestamp: now.Add(-90 * time.Second),
		},
	}

	// Simulated recent alerts
	alerts := []premove.AlertRecord{
		{
			ID:        "alert_" + fmt.Sprintf("%d", now.Unix()) + "_BTCUSD",
			Symbol:    "BTCUSD",
			AlertType: "pre_movement",
			Severity:  "high",
			Score:     78.5,
			Message:   "Pre-movement detected for BTCUSD (score: 78.5) - Strong 4h momentum",
			Timestamp: now.Add(-2 * time.Minute),
			Source:    "detector",
			Status:    "sent",
		},
	}

	// Simulated portfolio status
	portfolioStatus := map[string]interface{}{
		"total_positions": 2,
		"sector_counts": map[string]int{
			"crypto_major": 2,
			"defi":         0,
		},
		"utilization": map[string]interface{}{
			"beta_used":        3.8,
			"beta_utilization": 25.3,
			"max_correlation":  0.42,
		},
		"capacity": map[string]interface{}{
			"beta_available":   11.2,
			"sectors_at_limit": []string{},
		},
	}

	// Simulated execution metrics
	executionMetrics := &premove.ExecutionMetrics{
		TotalExecutions:        12,
		SuccessfulExecutions:   10,
		AvgSlippageBps:         15.8,
		AvgFillTimeMs:          2800.0,
		AvgQualityScore:        78.5,
		AcceptableSlippageRate: 83.3,
		InRecoveryMode:         false,
		ConsecutiveFails:       0,
		LastUpdated:            now,
	}

	// System health
	systemHealth := map[string]interface{}{
		"pipeline": "premove_detection",
		"components": map[string]interface{}{
			"portfolio_manager": true,
			"alert_manager":     true,
			"execution_monitor": true,
			"backtest_engine":   true,
		},
		"last_updated": now,
	}

	return PreMoveBoardState{
		ActiveCandidates: candidates,
		RecentAlerts:     alerts,
		PortfolioStatus:  portfolioStatus,
		ExecutionMetrics: executionMetrics,
		SystemHealth:     systemHealth,
	}
}

// broadcastSSEUpdate sends state changes to connected SSE clients
func (ui *PreMoveBoardUI) broadcastSSEUpdate() {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	// Serialize current state
	stateJSON, err := json.Marshal(ui.currentState)
	if err != nil {
		log.Error().Err(err).Msg("Failed to serialize premove board state for SSE")
		return
	}

	// Format as SSE event
	sseData := fmt.Sprintf("data: %s\n\n", stateJSON)
	sseBytes := []byte(sseData)

	// Broadcast to all connected clients
	for clientChan := range ui.sseClients {
		select {
		case clientChan <- sseBytes:
			// Successfully sent
		default:
			// Client channel blocked, remove it
			close(clientChan)
			delete(ui.sseClients, clientChan)
		}
	}

	ui.lastSSEUpdate = time.Now()

	log.Debug().
		Int("clients", len(ui.sseClients)).
		Int("state_size", len(stateJSON)).
		Msg("SSE update broadcasted to premove board clients")
}

// ServeSSE handles Server-Sent Events connections with 1Hz throttling
func (ui *PreMoveBoardUI) ServeSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create client channel
	clientChan := make(chan []byte, 10) // Buffer to prevent blocking

	// Register client
	ui.mu.Lock()
	ui.sseClients[clientChan] = true
	clientCount := len(ui.sseClients)
	ui.mu.Unlock()

	log.Info().
		Int("total_clients", clientCount).
		Str("client_ip", r.RemoteAddr).
		Msg("New SSE client connected to premove board")

	// Send initial state immediately
	ui.mu.RLock()
	initialState, _ := json.Marshal(ui.currentState)
	ui.mu.RUnlock()

	initialSSE := fmt.Sprintf("data: %s\n\n", initialState)
	fmt.Fprint(w, initialSSE)
	w.(http.Flusher).Flush()

	// Stream updates until client disconnects
	for {
		select {
		case <-r.Context().Done():
			// Client disconnected
			ui.mu.Lock()
			delete(ui.sseClients, clientChan)
			remainingClients := len(ui.sseClients)
			ui.mu.Unlock()

			close(clientChan)

			log.Info().
				Int("remaining_clients", remainingClients).
				Str("client_ip", r.RemoteAddr).
				Msg("SSE client disconnected from premove board")
			return

		case sseData := <-clientChan:
			// Send throttled update (≤1 Hz guaranteed by sseThrottledBroadcast)
			_, err := w.Write(sseData)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to write SSE data to client")
				return
			}
			w.(http.Flusher).Flush()
		}
	}
}

// GetCurrentState returns the current board state (for HTTP API)
func (ui *PreMoveBoardUI) GetCurrentState() PreMoveBoardState {
	ui.mu.RLock()
	defer ui.mu.RUnlock()
	return ui.currentState
}

// ForceRefresh triggers an immediate state refresh
func (ui *PreMoveBoardUI) ForceRefresh() {
	ui.refreshState()
}

// Shutdown gracefully stops the board UI
func (ui *PreMoveBoardUI) Shutdown() {
	log.Info().Msg("Shutting down premove board UI")

	// Cancel background routines
	ui.cancel()

	// Close all SSE clients
	ui.mu.Lock()
	for clientChan := range ui.sseClients {
		close(clientChan)
	}
	ui.sseClients = make(map[chan []byte]bool)
	ui.mu.Unlock()

	log.Info().Msg("Premove board UI shutdown complete")
}

// DisplayConsoleBoard renders the board state to console (for menu integration)
func (ui *PreMoveBoardUI) DisplayConsoleBoard() {
	ui.mu.RLock()
	state := ui.currentState
	ui.mu.RUnlock()

	fmt.Printf(`
╔════════════════ PREMOVE DETECTION BOARD ════════════════╗

📊 System Status: %s | Last Update: %s
🔄 Update Sequence: #%d | SSE Clients: %d

┌─── Active Candidates ───────────────────────────────────┐
`,
		func() string {
			if state.IsLive {
				return "🟢 LIVE"
			} else {
				return "🟡 DEMO"
			}
		}(),
		state.LastUpdate.Format("15:04:05"),
		state.UpdateSequence,
		len(ui.sseClients))

	for i, candidate := range state.ActiveCandidates {
		fmt.Printf("│ %d. %s | Score: %.1f | Gates: %d/3 | β: %.1f\n",
			i+1, candidate.Symbol, candidate.Score, candidate.PassedGates, candidate.Beta)
		fmt.Printf("│    Reasons: %v\n", candidate.Reasons[:1]) // Show first reason
		if candidate.PassedGates >= 2 {
			fmt.Printf("│    ✅ Passed minimum gates requirement\n")
		} else {
			fmt.Printf("│    ❌ Failed gates requirement (%d/3)\n", candidate.PassedGates)
		}
		fmt.Println("│")
	}

	if len(state.ActiveCandidates) == 0 {
		fmt.Println("│ No active candidates detected")
	}

	fmt.Printf(`└─────────────────────────────────────────────────────────┘

┌─── Portfolio & Execution Status ───────────────────────┐
`)

	if state.ExecutionMetrics != nil {
		fmt.Printf("│ Executions: %d total | Success Rate: %.1f%%\n",
			state.ExecutionMetrics.TotalExecutions,
			float64(state.ExecutionMetrics.SuccessfulExecutions)/float64(state.ExecutionMetrics.TotalExecutions)*100.0)
		fmt.Printf("│ Avg Slippage: %.1f bps | Avg Fill Time: %.0f ms\n",
			state.ExecutionMetrics.AvgSlippageBps,
			state.ExecutionMetrics.AvgFillTimeMs)

		if state.ExecutionMetrics.InRecoveryMode {
			fmt.Printf("│ ⚠️  RECOVERY MODE: %d consecutive fails\n",
				state.ExecutionMetrics.ConsecutiveFails)
		} else {
			fmt.Printf("│ ✅ Normal Operation | Quality Score: %.1f\n",
				state.ExecutionMetrics.AvgQualityScore)
		}
	}

	fmt.Printf(`└─────────────────────────────────────────────────────────┘

┌─── Recent Alerts ───────────────────────────────────────┐
`)

	for i, alert := range state.RecentAlerts {
		if i >= 3 {
			break
		} // Show max 3 recent alerts

		status := "✅"
		if alert.Status == "rate_limited" {
			status = "⏸️"
		} else if alert.Status == "failed" {
			status = "❌"
		}

		fmt.Printf("│ %s %s | %s | Score: %.1f\n",
			status, alert.Symbol, alert.Severity, alert.Score)
		fmt.Printf("│    %s (%s)\n",
			alert.Message,
			alert.Timestamp.Format("15:04:05"))
	}

	if len(state.RecentAlerts) == 0 {
		fmt.Println("│ No recent alerts")
	}

	fmt.Printf(`└─────────────────────────────────────────────────────────┘

╚══════════════════════════════════════════════════════════╝

💡 SSE Updates: ≤1 Hz throttled | Web Dashboard: /premove/board
🔧 Manual Refresh: Press 'r' | Exit: Press 'q'

`)
}
