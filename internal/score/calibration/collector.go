package calibration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sawpanic/cryptorun/internal/score/composite"
)

// CalibrationCollector manages collection of calibration samples from live trading data
type CalibrationCollector struct {
	harness *CalibrationHarness
	
	// Active positions being tracked
	activePositions map[string]*TrackedPosition
	positionMutex   sync.RWMutex
	
	// Configuration
	targetHoldingPeriod time.Duration // Target holding period for outcome measurement
	moveThreshold       float64       // Minimum move to consider "success" (e.g., 0.05 for 5%)
	maxTrackingTime     time.Duration // Maximum time to track a position
	
	// Metrics
	totalPositions   int
	successfulMoves  int
	timeouts         int
	
	// State
	isRunning bool
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

// TrackedPosition represents a position being tracked for calibration
type TrackedPosition struct {
	Symbol          string                      `json:"symbol"`
	Score           float64                     `json:"score"`
	CompositeResult *composite.CompositeScore   `json:"composite_result"`
	EntryTime       time.Time                   `json:"entry_time"`
	EntryPrice      float64                     `json:"entry_price"`
	Regime          string                      `json:"regime"`
	
	// Tracking data
	MaxMove         float64                     `json:"max_move"`
	CurrentMove     float64                     `json:"current_move"`
	LastUpdate      time.Time                   `json:"last_update"`
	
	// Outcome (set when position is closed)
	Outcome         *bool                       `json:"outcome"`          // nil while tracking, true/false when closed
	FinalMove       float64                     `json:"final_move"`       // Move at close
	CloseTime       time.Time                   `json:"close_time"`
	CloseReason     string                      `json:"close_reason"`     // "target_reached", "time_expired", "manual_close"
}

// NewCalibrationCollector creates a new calibration data collector
func NewCalibrationCollector(harness *CalibrationHarness) *CalibrationCollector {
	return &CalibrationCollector{
		harness:             harness,
		activePositions:     make(map[string]*TrackedPosition),
		targetHoldingPeriod: 48 * time.Hour, // 48 hours as per CryptoRun spec
		moveThreshold:       0.05,           // 5% move threshold
		maxTrackingTime:     72 * time.Hour, // Maximum 72 hours tracking
		stopChan:            make(chan struct{}),
	}
}

// StartTracking begins tracking positions for calibration
func (cc *CalibrationCollector) StartTracking(ctx context.Context) error {
	cc.positionMutex.Lock()
	defer cc.positionMutex.Unlock()
	
	if cc.isRunning {
		return fmt.Errorf("calibration collector is already running")
	}
	
	cc.isRunning = true
	
	// Start background goroutine to monitor positions
	cc.wg.Add(1)
	go cc.monitorPositions(ctx)
	
	return nil
}

// StopTracking stops tracking positions
func (cc *CalibrationCollector) StopTracking() {
	cc.positionMutex.Lock()
	defer cc.positionMutex.Unlock()
	
	if !cc.isRunning {
		return
	}
	
	cc.isRunning = false
	close(cc.stopChan)
	cc.wg.Wait()
}

// TrackNewPosition starts tracking a new position for calibration
func (cc *CalibrationCollector) TrackNewPosition(
	symbol string,
	score float64,
	compositeResult *composite.CompositeScore,
	entryPrice float64,
	regime string,
) error {
	cc.positionMutex.Lock()
	defer cc.positionMutex.Unlock()
	
	// Create position ID (symbol + timestamp for uniqueness)
	positionID := fmt.Sprintf("%s_%d", symbol, time.Now().Unix())
	
	position := &TrackedPosition{
		Symbol:          symbol,
		Score:           score,
		CompositeResult: compositeResult,
		EntryTime:       time.Now(),
		EntryPrice:      entryPrice,
		Regime:          regime,
		MaxMove:         0.0,
		CurrentMove:     0.0,
		LastUpdate:      time.Now(),
	}
	
	cc.activePositions[positionID] = position
	cc.totalPositions++
	
	return nil
}

// UpdatePosition updates price movement for a tracked position
func (cc *CalibrationCollector) UpdatePosition(symbol string, currentPrice float64) error {
	cc.positionMutex.Lock()
	defer cc.positionMutex.Unlock()
	
	// Update all positions for this symbol
	for positionID, position := range cc.activePositions {
		if position.Symbol != symbol || position.Outcome != nil {
			continue // Skip different symbols or already closed positions
		}
		
		// Calculate current move
		move := (currentPrice - position.EntryPrice) / position.EntryPrice
		position.CurrentMove = move
		position.LastUpdate = time.Now()
		
		// Update maximum move seen
		if absMove := abs(move); absMove > abs(position.MaxMove) {
			position.MaxMove = move
		}
		
		// Check if target move reached
		if abs(move) >= cc.moveThreshold {
			cc.closePosition(positionID, position, true, "target_reached")
		}
	}
	
	return nil
}

// monitorPositions monitors active positions for timeouts and cleanup
func (cc *CalibrationCollector) monitorPositions(ctx context.Context) {
	defer cc.wg.Done()
	
	ticker := time.NewTicker(1 * time.Minute) // Check every minute
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-cc.stopChan:
			return
		case <-ticker.C:
			cc.checkPositionTimeouts()
		}
	}
}

// checkPositionTimeouts checks for and handles position timeouts
func (cc *CalibrationCollector) checkPositionTimeouts() {
	cc.positionMutex.Lock()
	defer cc.positionMutex.Unlock()
	
	now := time.Now()
	
	for positionID, position := range cc.activePositions {
		if position.Outcome != nil {
			continue // Already closed
		}
		
		// Check for target holding period
		if now.Sub(position.EntryTime) >= cc.targetHoldingPeriod {
			// Close at target holding period
			targetReached := abs(position.CurrentMove) >= cc.moveThreshold
			cc.closePosition(positionID, position, targetReached, "target_period")
			continue
		}
		
		// Check for maximum tracking time
		if now.Sub(position.EntryTime) >= cc.maxTrackingTime {
			// Force close due to timeout
			cc.closePosition(positionID, position, false, "timeout")
			cc.timeouts++
			continue
		}
	}
}

// closePosition closes a tracked position and creates a calibration sample
func (cc *CalibrationCollector) closePosition(positionID string, position *TrackedPosition, success bool, reason string) {
	// Set outcome
	position.Outcome = &success
	position.FinalMove = position.CurrentMove
	position.CloseTime = time.Now()
	position.CloseReason = reason
	
	// Update metrics
	if success {
		cc.successfulMoves++
	}
	
	// Create calibration sample
	sample := CalibrationSample{
		Score:         position.Score,
		Outcome:       success,
		Timestamp:     position.EntryTime,
		Symbol:        position.Symbol,
		Regime:        position.Regime,
		HoldingPeriod: position.CloseTime.Sub(position.EntryTime),
		MaxMove:       position.MaxMove,
		FinalMove:     position.FinalMove,
	}
	
	// Add sample to harness (ignore errors in background collection)
	cc.harness.AddSample(sample)
	
	// Remove from active positions after some time to allow for analysis
	// In a production system, you might want to persist this data
	go func() {
		time.Sleep(5 * time.Minute)
		cc.positionMutex.Lock()
		defer cc.positionMutex.Unlock()
		delete(cc.activePositions, positionID)
	}()
}

// GetCollectionStatus returns current collection status
type CollectionStatus struct {
	IsRunning         bool                        `json:"is_running"`
	ActivePositions   int                         `json:"active_positions"`
	TotalPositions    int                         `json:"total_positions"`
	SuccessfulMoves   int                         `json:"successful_moves"`
	SuccessRate       float64                     `json:"success_rate"`
	Timeouts          int                         `json:"timeouts"`
	Positions         map[string]*TrackedPosition `json:"positions"`
}

// GetStatus returns current collection status
func (cc *CalibrationCollector) GetStatus() CollectionStatus {
	cc.positionMutex.RLock()
	defer cc.positionMutex.RUnlock()
	
	// Count only truly active positions (not closed)
	activeCount := 0
	for _, position := range cc.activePositions {
		if position.Outcome == nil {
			activeCount++
		}
	}
	
	status := CollectionStatus{
		IsRunning:       cc.isRunning,
		ActivePositions: activeCount,
		TotalPositions:  cc.totalPositions,
		SuccessfulMoves: cc.successfulMoves,
		Timeouts:        cc.timeouts,
		Positions:       make(map[string]*TrackedPosition),
	}
	
	// Calculate success rate
	if cc.totalPositions > 0 {
		status.SuccessRate = float64(cc.successfulMoves) / float64(cc.totalPositions)
	}
	
	// Copy positions for thread safety (include all positions, even closed ones)
	for id, position := range cc.activePositions {
		positionCopy := *position
		status.Positions[id] = &positionCopy
	}
	
	return status
}

// ForceClosePosition manually closes a tracked position
func (cc *CalibrationCollector) ForceClosePosition(positionID string, currentPrice float64) error {
	cc.positionMutex.Lock()
	defer cc.positionMutex.Unlock()
	
	position, exists := cc.activePositions[positionID]
	if !exists {
		return fmt.Errorf("position %s not found", positionID)
	}
	
	if position.Outcome != nil {
		return fmt.Errorf("position %s already closed", positionID)
	}
	
	// Update current move
	move := (currentPrice - position.EntryPrice) / position.EntryPrice
	position.CurrentMove = move
	
	// Close position
	success := abs(move) >= cc.moveThreshold
	cc.closePosition(positionID, position, success, "manual_close")
	
	return nil
}

// GetPositionDetails returns details of a specific position
func (cc *CalibrationCollector) GetPositionDetails(positionID string) (*TrackedPosition, error) {
	cc.positionMutex.RLock()
	defer cc.positionMutex.RUnlock()
	
	position, exists := cc.activePositions[positionID]
	if !exists {
		return nil, fmt.Errorf("position %s not found", positionID)
	}
	
	// Return a copy for thread safety
	positionCopy := *position
	return &positionCopy, nil
}

// CleanupClosedPositions removes closed positions from memory
func (cc *CalibrationCollector) CleanupClosedPositions() int {
	cc.positionMutex.Lock()
	defer cc.positionMutex.Unlock()
	
	originalCount := len(cc.activePositions)
	
	for positionID, position := range cc.activePositions {
		if position.Outcome != nil {
			// Position is closed, remove it
			delete(cc.activePositions, positionID)
		}
	}
	
	return originalCount - len(cc.activePositions)
}

// SetMoveThreshold updates the move threshold for determining success
func (cc *CalibrationCollector) SetMoveThreshold(threshold float64) {
	if threshold <= 0 || threshold > 1.0 {
		return // Invalid threshold
	}
	cc.moveThreshold = threshold
}

// SetTargetHoldingPeriod updates the target holding period
func (cc *CalibrationCollector) SetTargetHoldingPeriod(period time.Duration) {
	if period <= 0 {
		return // Invalid period
	}
	cc.targetHoldingPeriod = period
}

// ExportPositionHistory exports historical position data for analysis
type PositionHistoryExport struct {
	TotalPositions   int                        `json:"total_positions"`
	SuccessRate      float64                    `json:"success_rate"`
	AvgHoldingPeriod time.Duration              `json:"avg_holding_period"`
	ClosedPositions  []*TrackedPosition         `json:"closed_positions"`
	ExportedAt       time.Time                  `json:"exported_at"`
}

// ExportPositionHistory exports closed positions for analysis
func (cc *CalibrationCollector) ExportPositionHistory() PositionHistoryExport {
	cc.positionMutex.RLock()
	defer cc.positionMutex.RUnlock()
	
	export := PositionHistoryExport{
		TotalPositions:  cc.totalPositions,
		ClosedPositions: make([]*TrackedPosition, 0),
		ExportedAt:      time.Now(),
	}
	
	// Collect closed positions
	totalHoldingTime := time.Duration(0)
	closedCount := 0
	
	for _, position := range cc.activePositions {
		if position.Outcome != nil {
			// Position is closed
			positionCopy := *position
			export.ClosedPositions = append(export.ClosedPositions, &positionCopy)
			totalHoldingTime += position.CloseTime.Sub(position.EntryTime)
			closedCount++
		}
	}
	
	// Calculate averages
	if closedCount > 0 {
		export.SuccessRate = float64(cc.successfulMoves) / float64(closedCount)
		export.AvgHoldingPeriod = totalHoldingTime / time.Duration(closedCount)
	}
	
	return export
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}