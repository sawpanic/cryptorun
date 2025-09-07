// Package menu contains UI components for the premove monitoring board with SSE throttling
package menu

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// PremoveBoardPage handles real-time premove monitoring with SSE throttling ≤1 Hz
type PremoveBoardPage struct {
	mu                  sync.Mutex
	lastSSEUpdate       time.Time
	sseThrottleInterval time.Duration // Default: 1 second (1 Hz)
	subscribers         map[string]*SSESubscriber
	boardData           *PremoveBoardData
}

// PremoveBoardData represents the current state of the premove board
type PremoveBoardData struct {
	Timestamp         time.Time          `json:"timestamp"`
	PortfolioSummary  PortfolioSummary   `json:"portfolio_summary"`
	AlertsSummary     AlertsSummary      `json:"alerts_summary"`
	ExecutionSummary  ExecutionSummary   `json:"execution_summary"`
	ActiveCandidates  []CandidateInfo    `json:"active_candidates"`
	SystemStatus      SystemStatus       `json:"system_status"`
	TransitionUpdates []TransitionUpdate `json:"transition_updates"`
}

// PortfolioSummary provides portfolio management status
type PortfolioSummary struct {
	TotalPositions      int      `json:"total_positions"`
	BetaUtilization     float64  `json:"beta_utilization_pct"`
	ExposureUtilization float64  `json:"exposure_utilization_pct"`
	TightenedVenues     []string `json:"tightened_venues"`
	LastPruneTime       string   `json:"last_prune_time"`
}

// AlertsSummary provides alerts governance status
type AlertsSummary struct {
	HourlyAlerts       int      `json:"hourly_alerts"`
	DailyAlerts        int      `json:"daily_alerts"`
	ActiveSymbols      int      `json:"active_symbols"`
	OverrideRate       float64  `json:"override_rate_pct"`
	RateLimitedSymbols []string `json:"rate_limited_symbols"`
}

// ExecutionSummary provides execution quality metrics
type ExecutionSummary struct {
	GoodExecutionRate float64  `json:"good_execution_rate_pct"`
	AvgSlippageBps    float64  `json:"avg_slippage_bps"`
	TightenedVenues   []string `json:"tightened_venues"`
	VenuesInRecovery  int      `json:"venues_in_recovery"`
	TotalExecutions   int      `json:"total_executions"`
}

// CandidateInfo represents an active premove candidate
type CandidateInfo struct {
	Symbol      string    `json:"symbol"`
	Score       float64   `json:"score"`
	PassedGates int       `json:"passed_gates"`
	Sector      string    `json:"sector"`
	Status      string    `json:"status"` // "portfolio_accepted", "alert_sent", "rate_limited"
	LastUpdate  time.Time `json:"last_update"`
}

// SystemStatus provides overall system health
type SystemStatus struct {
	Status            string    `json:"status"` // "healthy", "degraded", "critical"
	LastHealthCheck   time.Time `json:"last_health_check"`
	ActiveConnections int       `json:"active_connections"`
	DataLatency       float64   `json:"data_latency_ms"`
}

// TransitionUpdate represents a state transition for SSE streaming
type TransitionUpdate struct {
	Type      string      `json:"type"` // "candidate_added", "portfolio_pruned", "alert_sent", "execution_recorded"
	Symbol    string      `json:"symbol,omitempty"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// SSESubscriber represents a connected client for server-sent events
type SSESubscriber struct {
	ID            string
	Channel       chan []byte
	LastPing      time.Time
	Connected     bool
	FilterSymbols []string // Optional symbol filtering
}

// NewPremoveBoardPage creates a new premove board page with default SSE throttling
func NewPremoveBoardPage() *PremoveBoardPage {
	return &PremoveBoardPage{
		sseThrottleInterval: time.Second, // 1 Hz throttling
		subscribers:         make(map[string]*SSESubscriber),
		boardData: &PremoveBoardData{
			Timestamp:         time.Now(),
			ActiveCandidates:  make([]CandidateInfo, 0),
			TransitionUpdates: make([]TransitionUpdate, 0),
		},
	}
}

// NewPremoveBoardPageWithThrottle creates a board page with custom SSE throttling
func NewPremoveBoardPageWithThrottle(throttleInterval time.Duration) *PremoveBoardPage {
	return &PremoveBoardPage{
		sseThrottleInterval: throttleInterval,
		subscribers:         make(map[string]*SSESubscriber),
		boardData: &PremoveBoardData{
			Timestamp:         time.Now(),
			ActiveCandidates:  make([]CandidateInfo, 0),
			TransitionUpdates: make([]TransitionUpdate, 0),
		},
	}
}

// UpdateBoardData updates the board data and triggers SSE if throttle allows
func (pb *PremoveBoardPage) UpdateBoardData(data *PremoveBoardData) error {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.boardData = data
	pb.boardData.Timestamp = time.Now()

	// Check SSE throttle
	if pb.shouldSendSSEUpdate() {
		pb.broadcastSSEUpdate()
		pb.lastSSEUpdate = time.Now()
	}

	return nil
}

// AddTransition adds a transition update and triggers SSE if throttle allows
func (pb *PremoveBoardPage) AddTransition(transitionType string, symbol string, data interface{}) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	transition := TransitionUpdate{
		Type:      transitionType,
		Symbol:    symbol,
		Data:      data,
		Timestamp: time.Now(),
	}

	// Add transition to board data
	pb.boardData.TransitionUpdates = append(pb.boardData.TransitionUpdates, transition)

	// Keep only last 50 transitions
	if len(pb.boardData.TransitionUpdates) > 50 {
		pb.boardData.TransitionUpdates = pb.boardData.TransitionUpdates[len(pb.boardData.TransitionUpdates)-50:]
	}

	// Check SSE throttle for transition updates
	if pb.shouldSendSSEUpdate() {
		pb.broadcastSSETransition(transition)
		pb.lastSSEUpdate = time.Now()
	}
}

// SubscribeSSE adds a new SSE subscriber
func (pb *PremoveBoardPage) SubscribeSSE(subscriberID string, filterSymbols []string) *SSESubscriber {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	subscriber := &SSESubscriber{
		ID:            subscriberID,
		Channel:       make(chan []byte, 100), // Buffer for 100 messages
		LastPing:      time.Now(),
		Connected:     true,
		FilterSymbols: filterSymbols,
	}

	pb.subscribers[subscriberID] = subscriber

	// Send initial board data
	if initialData, err := json.Marshal(pb.boardData); err == nil {
		select {
		case subscriber.Channel <- initialData:
		default:
			// Channel full, subscriber may be slow
		}
	}

	return subscriber
}

// UnsubscribeSSE removes an SSE subscriber
func (pb *PremoveBoardPage) UnsubscribeSSE(subscriberID string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if subscriber, exists := pb.subscribers[subscriberID]; exists {
		subscriber.Connected = false
		close(subscriber.Channel)
		delete(pb.subscribers, subscriberID)
	}
}

// shouldSendSSEUpdate checks if enough time has passed for next SSE update
func (pb *PremoveBoardPage) shouldSendSSEUpdate() bool {
	return time.Since(pb.lastSSEUpdate) >= pb.sseThrottleInterval
}

// broadcastSSEUpdate sends full board data to all subscribers
func (pb *PremoveBoardPage) broadcastSSEUpdate() {
	data, err := json.Marshal(pb.boardData)
	if err != nil {
		return
	}

	for _, subscriber := range pb.subscribers {
		if !subscriber.Connected {
			continue
		}

		select {
		case subscriber.Channel <- data:
		default:
			// Channel full, subscriber may be disconnected
			subscriber.Connected = false
		}
	}
}

// broadcastSSETransition sends transition update to relevant subscribers
func (pb *PremoveBoardPage) broadcastSSETransition(transition TransitionUpdate) {
	transitionData := map[string]interface{}{
		"type":       "transition",
		"transition": transition,
		"timestamp":  time.Now(),
	}

	data, err := json.Marshal(transitionData)
	if err != nil {
		return
	}

	for _, subscriber := range pb.subscribers {
		if !subscriber.Connected {
			continue
		}

		// Apply symbol filtering if specified
		if len(subscriber.FilterSymbols) > 0 && transition.Symbol != "" {
			symbolMatch := false
			for _, filterSymbol := range subscriber.FilterSymbols {
				if filterSymbol == transition.Symbol {
					symbolMatch = true
					break
				}
			}
			if !symbolMatch {
				continue
			}
		}

		select {
		case subscriber.Channel <- data:
		default:
			subscriber.Connected = false
		}
	}
}

// GetBoardData returns the current board data (for HTTP endpoints)
func (pb *PremoveBoardPage) GetBoardData() *PremoveBoardData {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	// Return a copy to avoid concurrent access issues
	data := *pb.boardData
	return &data
}

// GetSSEStats returns statistics about SSE subscribers and throttling
func (pb *PremoveBoardPage) GetSSEStats() map[string]interface{} {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	connectedCount := 0
	for _, subscriber := range pb.subscribers {
		if subscriber.Connected {
			connectedCount++
		}
	}

	return map[string]interface{}{
		"total_subscribers":      len(pb.subscribers),
		"connected_subscribers":  connectedCount,
		"throttle_interval_ms":   pb.sseThrottleInterval.Milliseconds(),
		"last_sse_update":        pb.lastSSEUpdate.Format(time.RFC3339),
		"time_since_last_update": time.Since(pb.lastSSEUpdate).Milliseconds(),
	}
}

// CleanupStaleSubscribers removes disconnected or stale subscribers
func (pb *PremoveBoardPage) CleanupStaleSubscribers() {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	staleThreshold := time.Minute * 5 // 5 minutes without ping
	now := time.Now()

	for id, subscriber := range pb.subscribers {
		if !subscriber.Connected || now.Sub(subscriber.LastPing) > staleThreshold {
			close(subscriber.Channel)
			delete(pb.subscribers, id)
		}
	}
}

// PingSubscriber updates the last ping time for a subscriber (for keepalive)
func (pb *PremoveBoardPage) PingSubscriber(subscriberID string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if subscriber, exists := pb.subscribers[subscriberID]; exists {
		subscriber.LastPing = time.Now()
	}
}

// FormatBoardSummary returns a human-readable summary of the board state
func (pb *PremoveBoardPage) FormatBoardSummary() string {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	return fmt.Sprintf(`
=== PREMOVE BOARD SUMMARY ===
Timestamp: %s
Active Candidates: %d
Portfolio Beta Utilization: %.1f%%
Hourly Alerts: %d/day: %d
Execution Quality: %.1f%% good (%.1f bps avg slippage)
SSE Subscribers: %d connected
Tightened Venues: %v
Recent Transitions: %d
System Status: %s
`,
		pb.boardData.Timestamp.Format("15:04:05"),
		len(pb.boardData.ActiveCandidates),
		pb.boardData.PortfolioSummary.BetaUtilization,
		pb.boardData.AlertsSummary.HourlyAlerts,
		pb.boardData.AlertsSummary.DailyAlerts,
		pb.boardData.ExecutionSummary.GoodExecutionRate,
		pb.boardData.ExecutionSummary.AvgSlippageBps,
		len(pb.subscribers),
		pb.boardData.ExecutionSummary.TightenedVenues,
		len(pb.boardData.TransitionUpdates),
		pb.boardData.SystemStatus.Status,
	)
}
