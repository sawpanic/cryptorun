package replication

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// HotExecutor handles real-time WebSocket stream replication for hot tier
type HotExecutor struct {
	config      HotExecutorConfig
	connections map[Region]*websocket.Conn
	buffers     map[Region]*ReplayBuffer
	metrics     *ExecutorMetrics
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// HotExecutorConfig holds configuration for hot tier replication
type HotExecutorConfig struct {
	ReplayBufferSize  int           `json:"replay_buffer_size"`
	MaxReconnectDelay time.Duration `json:"max_reconnect_delay"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	SequenceGapLimit  int64         `json:"sequence_gap_limit"`
	EnableCompression bool          `json:"enable_compression"`
	BatchSize         int           `json:"batch_size"`
	FlushInterval     time.Duration `json:"flush_interval"`
}

// ReplayBuffer maintains a circular buffer of recent messages for gap recovery
type ReplayBuffer struct {
	messages []HotMessage
	size     int
	head     int
	tail     int
	mu       sync.RWMutex
}

// HotMessage represents a message in the hot tier replication stream
type HotMessage struct {
	ID        string                 `json:"id"`
	Sequence  int64                  `json:"sequence"`
	Timestamp time.Time              `json:"timestamp"`
	Venue     string                 `json:"venue"`
	Symbol    string                 `json:"symbol"`
	Type      string                 `json:"type"` // trade, depth, funding, etc.
	Data      map[string]interface{} `json:"data"`
	Checksum  string                 `json:"checksum"`
}

// ExecutorMetrics tracks replication executor performance
type ExecutorMetrics struct {
	MessagesReceived    int64
	MessagesReplicated  int64
	MessagesDropped     int64
	SequenceGaps        int64
	ReconnectCount      int64
	LastSequenceNumber  int64
	ReplicationLag      time.Duration
	ErrorRate           float64
	mu                  sync.RWMutex
}

// NewHotExecutor creates a new hot tier executor
func NewHotExecutor(config HotExecutorConfig) *HotExecutor {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &HotExecutor{
		config:      config,
		connections: make(map[Region]*websocket.Conn),
		buffers:     make(map[Region]*ReplayBuffer),
		metrics:     &ExecutorMetrics{},
		ctx:         ctx,
		cancel:      cancel,
	}
}

// NewReplayBuffer creates a new replay buffer
func NewReplayBuffer(size int) *ReplayBuffer {
	return &ReplayBuffer{
		messages: make([]HotMessage, size),
		size:     size,
		head:     0,
		tail:     0,
	}
}

// Add adds a message to the replay buffer
func (rb *ReplayBuffer) Add(msg HotMessage) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	
	rb.messages[rb.head] = msg
	rb.head = (rb.head + 1) % rb.size
	
	// If buffer is full, advance tail
	if rb.head == rb.tail {
		rb.tail = (rb.tail + 1) % rb.size
	}
}

// GetRange returns messages in the specified sequence range
func (rb *ReplayBuffer) GetRange(fromSeq, toSeq int64) []HotMessage {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	
	var result []HotMessage
	
	// Linear search through circular buffer
	current := rb.tail
	for current != rb.head {
		msg := rb.messages[current]
		if msg.Sequence >= fromSeq && msg.Sequence <= toSeq {
			result = append(result, msg)
		}
		current = (current + 1) % rb.size
	}
	
	return result
}

// ExecuteStep executes a hot tier replication step
func (h *HotExecutor) ExecuteStep(ctx context.Context, step Step) error {
	if step.Tier != TierHot {
		return fmt.Errorf("hot executor can only handle hot tier steps")
	}
	
	log.Printf("Executing hot tier replication step %s: %s -> %s", step.ID, step.From, step.To)
	
	// Ensure connections exist for both regions
	if err := h.ensureConnection(step.From); err != nil {
		return fmt.Errorf("failed to establish source connection: %w", err)
	}
	
	if err := h.ensureConnection(step.To); err != nil {
		return fmt.Errorf("failed to establish destination connection: %w", err)
	}
	
	// Start replication pipeline
	return h.replicateStream(ctx, step)
}

// ensureConnection ensures a WebSocket connection exists for a region
func (h *HotExecutor) ensureConnection(region Region) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if conn, exists := h.connections[region]; exists && conn != nil {
		// Check if connection is still alive
		if err := conn.WriteMessage(websocket.PingMessage, nil); err == nil {
			return nil // Connection is healthy
		}
		// Connection is dead, clean up
		conn.Close()
		delete(h.connections, region)
	}
	
	// Create new connection
	wsURL := h.getWebSocketURL(region)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", region, err)
	}
	
	h.connections[region] = conn
	
	// Initialize replay buffer if not exists
	if _, exists := h.buffers[region]; !exists {
		h.buffers[region] = NewReplayBuffer(h.config.ReplayBufferSize)
	}
	
	log.Printf("Established WebSocket connection to region %s", region)
	return nil
}

// getWebSocketURL returns the WebSocket URL for a region
func (h *HotExecutor) getWebSocketURL(region Region) string {
	// In production, this would come from configuration
	urls := map[Region]string{
		RegionUSEast1: "wss://hot-tier-us-east-1.cryptorun.internal/ws",
		RegionUSWest2: "wss://hot-tier-us-west-2.cryptorun.internal/ws",
		RegionEUWest1: "wss://hot-tier-eu-west-1.cryptorun.internal/ws",
	}
	
	if url, exists := urls[region]; exists {
		return url
	}
	
	return fmt.Sprintf("wss://hot-tier-%s.cryptorun.internal/ws", region)
}

// replicateStream handles the actual message replication between regions
func (h *HotExecutor) replicateStream(ctx context.Context, step Step) error {
	sourceConn := h.connections[step.From]
	destConn := h.connections[step.To]
	sourceBuffer := h.buffers[step.From]
	
	// Set up message reading from source
	messageChan := make(chan HotMessage, h.config.BatchSize)
	errorChan := make(chan error, 1)
	
	// Start source reader goroutine
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		defer close(messageChan)
		
		var lastSequence int64
		
		for {
			select {
			case <-ctx.Done():
				return
			default:
				var msg HotMessage
				if err := sourceConn.ReadJSON(&msg); err != nil {
					errorChan <- fmt.Errorf("failed to read from source: %w", err)
					return
				}
				
				// Update metrics
				h.updateMetrics(func(m *ExecutorMetrics) {
					m.MessagesReceived++
					m.LastSequenceNumber = msg.Sequence
				})
				
				// Check for sequence gaps
				if lastSequence > 0 && msg.Sequence > lastSequence+1 {
					gap := msg.Sequence - lastSequence - 1
					h.updateMetrics(func(m *ExecutorMetrics) {
						m.SequenceGaps += gap
					})
					
					// Attempt gap recovery from replay buffer
					if gap <= h.config.SequenceGapLimit {
						h.recoverSequenceGap(lastSequence+1, msg.Sequence-1, sourceBuffer, messageChan)
					} else {
						log.Printf("Sequence gap too large (%d), skipping recovery", gap)
					}
				}
				
				// Add to replay buffer
				sourceBuffer.Add(msg)
				
				// Validate message within time window
				if h.isMessageInWindow(msg, step.Window) {
					select {
					case messageChan <- msg:
					case <-ctx.Done():
						return
					}
				}
				
				lastSequence = msg.Sequence
			}
		}
	}()
	
	// Start destination writer goroutine
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		
		batch := make([]HotMessage, 0, h.config.BatchSize)
		ticker := time.NewTicker(h.config.FlushInterval)
		defer ticker.Stop()
		
		flushBatch := func() {
			if len(batch) == 0 {
				return
			}
			
			if err := h.sendBatch(destConn, batch); err != nil {
				errorChan <- fmt.Errorf("failed to send batch: %w", err)
				return
			}
			
			h.updateMetrics(func(m *ExecutorMetrics) {
				m.MessagesReplicated += int64(len(batch))
			})
			
			batch = batch[:0] // Reset batch
		}
		
		for {
			select {
			case msg, ok := <-messageChan:
				if !ok {
					flushBatch()
					return
				}
				
				// Run validators
				if err := h.validateMessage(msg, step.Validator); err != nil {
					log.Printf("Message validation failed: %v", err)
					h.updateMetrics(func(m *ExecutorMetrics) {
						m.MessagesDropped++
					})
					continue
				}
				
				batch = append(batch, msg)
				
				// Flush batch if full
				if len(batch) >= h.config.BatchSize {
					flushBatch()
				}
				
			case <-ticker.C:
				flushBatch()
				
			case <-ctx.Done():
				flushBatch()
				return
			}
		}
	}()
	
	// Wait for completion or error
	select {
	case err := <-errorChan:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(step.EstimatedDuration * 2): // Timeout after 2x estimated duration
		return fmt.Errorf("step execution timeout")
	}
}

// recoverSequenceGap attempts to recover missing messages from replay buffer
func (h *HotExecutor) recoverSequenceGap(fromSeq, toSeq int64, buffer *ReplayBuffer, msgChan chan<- HotMessage) {
	log.Printf("Attempting to recover sequence gap: %d-%d", fromSeq, toSeq)
	
	recoveredMessages := buffer.GetRange(fromSeq, toSeq)
	
	for _, msg := range recoveredMessages {
		select {
		case msgChan <- msg:
		default:
			// Channel full, drop message
			h.updateMetrics(func(m *ExecutorMetrics) {
				m.MessagesDropped++
			})
		}
	}
	
	log.Printf("Recovered %d messages from replay buffer", len(recoveredMessages))
}

// isMessageInWindow checks if a message falls within the specified time window
func (h *HotExecutor) isMessageInWindow(msg HotMessage, window TimeRange) bool {
	return !msg.Timestamp.Before(window.From) && msg.Timestamp.Before(window.To)
}

// validateMessage runs validation functions on a message
func (h *HotExecutor) validateMessage(msg HotMessage, validators []ValidateFn) error {
	// Convert message to validation format
	data := map[string]interface{}{
		"id":        msg.ID,
		"sequence":  msg.Sequence,
		"timestamp": msg.Timestamp,
		"venue":     msg.Venue,
		"symbol":    msg.Symbol,
		"type":      msg.Type,
		"checksum":  msg.Checksum,
	}
	
	// Add message data
	for k, v := range msg.Data {
		data[k] = v
	}
	
	// Run all validators
	for _, validator := range validators {
		if err := validator(data); err != nil {
			return err
		}
	}
	
	return nil
}

// sendBatch sends a batch of messages to the destination
func (h *HotExecutor) sendBatch(conn *websocket.Conn, batch []HotMessage) error {
	batchData := map[string]interface{}{
		"type":      "replication_batch",
		"timestamp": time.Now(),
		"count":     len(batch),
		"messages":  batch,
	}
	
	return conn.WriteJSON(batchData)
}

// updateMetrics safely updates executor metrics
func (h *HotExecutor) updateMetrics(fn func(*ExecutorMetrics)) {
	h.metrics.mu.Lock()
	defer h.metrics.mu.Unlock()
	fn(h.metrics)
}

// GetMetrics returns a copy of current metrics
func (h *HotExecutor) GetMetrics() ExecutorMetrics {
	h.metrics.mu.RLock()
	defer h.metrics.mu.RUnlock()
	return *h.metrics
}

// StartHealthCheck starts periodic health checks and reconnection
func (h *HotExecutor) StartHealthCheck() {
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		
		ticker := time.NewTicker(h.config.HeartbeatInterval)
		defer ticker.Stop()
		
		for {
			select {
			case <-h.ctx.Done():
				return
			case <-ticker.C:
				h.checkConnections()
			}
		}
	}()
}

// checkConnections verifies all connections are healthy and reconnects if needed
func (h *HotExecutor) checkConnections() {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	for region, conn := range h.connections {
		if conn == nil {
			continue
		}
		
		// Send ping
		if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
			log.Printf("Connection to %s is unhealthy, reconnecting: %v", region, err)
			
			conn.Close()
			delete(h.connections, region)
			
			h.updateMetrics(func(m *ExecutorMetrics) {
				m.ReconnectCount++
			})
			
			// Attempt reconnection
			if err := h.ensureConnection(region); err != nil {
				log.Printf("Failed to reconnect to %s: %v", region, err)
			}
		}
	}
}

// Stop gracefully shuts down the hot executor
func (h *HotExecutor) Stop() error {
	log.Println("Stopping hot tier executor...")
	
	h.cancel()
	
	// Close all connections
	h.mu.Lock()
	for region, conn := range h.connections {
		if conn != nil {
			conn.Close()
			log.Printf("Closed connection to %s", region)
		}
	}
	h.connections = make(map[Region]*websocket.Conn)
	h.mu.Unlock()
	
	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		log.Println("Hot tier executor stopped successfully")
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for executor shutdown")
	}
}