package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"cryptorun/internal/domain"
	"cryptorun/internal/metrics"
)

// HotSetManager manages real-time WebSocket connections for top USD pairs
type HotSetManager struct {
	config         *HotSetConfig
	connections    map[string]*VenueConnection
	metrics        *HotSetMetrics
	microstructure *MicrostructureProcessor
	latencyMonitor *LatencyMonitor
	subscribers    []chan *TickUpdate
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

// HotSetConfig configures the hot set manager
type HotSetConfig struct {
	TopN             int                        `yaml:"top_n"`
	Venues           []VenueConfig             `yaml:"venues"`
	UpdateInterval   time.Duration             `yaml:"update_interval"`
	ReconnectDelay   time.Duration             `yaml:"reconnect_delay"`
	MaxReconnects    int                       `yaml:"max_reconnects"`
	MetricsInterval  time.Duration             `yaml:"metrics_interval"`
	VADRMinBars      int                       `yaml:"vadr_min_bars"`
	StaleThreshold   time.Duration             `yaml:"stale_threshold"`
}

// VenueConfig configures a specific exchange venue
type VenueConfig struct {
	Name           string            `yaml:"name"`
	WSEndpoint     string            `yaml:"ws_endpoint"`
	SubscribeMsg   map[string]interface{} `yaml:"subscribe_msg"`
	PingInterval   time.Duration     `yaml:"ping_interval"`
	PongTimeout    time.Duration     `yaml:"pong_timeout"`
	RateLimit      RateLimitConfig   `yaml:"rate_limit"`
	Enabled        bool              `yaml:"enabled"`
}

// RateLimitConfig defines rate limiting parameters for a venue
type RateLimitConfig struct {
	RequestsPerSecond float64 `yaml:"requests_per_second"`
	Burst            int     `yaml:"burst"`
}

// VenueConnection represents a WebSocket connection to an exchange
type VenueConnection struct {
	venue      string
	conn       *websocket.Conn
	config     *VenueConfig
	metrics    *VenueMetrics
	subscribed map[string]bool
	lastPong   time.Time
	mu         sync.RWMutex
}

// TickUpdate represents a normalized tick from any venue
type TickUpdate struct {
	Venue     string    `json:"venue"`
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`
	Bid       float64   `json:"bid"`
	Ask       float64   `json:"ask"`
	BidSize   float64   `json:"bid_size"`
	AskSize   float64   `json:"ask_size"`
	LastPrice float64   `json:"last_price"`
	Volume24h float64   `json:"volume_24h"`
	ProcessingLatency time.Duration `json:"processing_latency"`
}

// HotSetMetrics tracks performance metrics for the hot set system
type HotSetMetrics struct {
	IngestCount       *metrics.Counter   `json:"ingest_count"`
	NormalizeLatency  *metrics.Histogram `json:"normalize_latency"`
	ProcessLatency    *metrics.Histogram `json:"process_latency"`
	ServeLatency      *metrics.Histogram `json:"serve_latency"`
	StaleTickCount    *metrics.Counter   `json:"stale_tick_count"`
	ErrorCount        *metrics.Counter   `json:"error_count"`
	ConnectedVenues   *metrics.Gauge     `json:"connected_venues"`
	ActiveSymbols     *metrics.Gauge     `json:"active_symbols"`
	mu                sync.RWMutex
}

// VenueMetrics tracks per-venue metrics
type VenueMetrics struct {
	ConnectionUptime  time.Time         `json:"connection_uptime"`
	MessagesReceived  *metrics.Counter  `json:"messages_received"`
	MessagesProcessed *metrics.Counter  `json:"messages_processed"`
	ReconnectCount    *metrics.Counter  `json:"reconnect_count"`
	LastTickTime      time.Time         `json:"last_tick_time"`
	TicksPerSecond    *metrics.Gauge    `json:"ticks_per_second"`
}

// NewHotSetManager creates a new hot set manager
func NewHotSetManager(config *HotSetConfig) *HotSetManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	manager := &HotSetManager{
		config:         config,
		connections:    make(map[string]*VenueConnection),
		subscribers:    make([]chan *TickUpdate, 0),
		microstructure: NewMicrostructureProcessor(config.VADRMinBars),
		latencyMonitor: NewLatencyMonitor(),
		ctx:            ctx,
		cancel:         cancel,
	}
	
	// Initialize metrics
	manager.metrics = &HotSetMetrics{
		IngestCount:       metrics.NewCounter("hotset_ingest_total"),
		NormalizeLatency:  metrics.NewHistogram("hotset_normalize_latency_ms"),
		ProcessLatency:    metrics.NewHistogram("hotset_process_latency_ms"),
		ServeLatency:      metrics.NewHistogram("hotset_serve_latency_ms"),
		StaleTickCount:    metrics.NewCounter("hotset_stale_ticks_total"),
		ErrorCount:        metrics.NewCounter("hotset_errors_total"),
		ConnectedVenues:   metrics.NewGauge("hotset_connected_venues"),
		ActiveSymbols:     metrics.NewGauge("hotset_active_symbols"),
	}
	
	return manager
}

// Start begins WebSocket connections and processing
func (hsm *HotSetManager) Start(symbols []string) error {
	log.Info().Int("symbols", len(symbols)).Msg("Starting hot set manager")
	
	// Initialize microstructure processor with symbols
	if err := hsm.microstructure.Initialize(symbols); err != nil {
		return fmt.Errorf("failed to initialize microstructure processor: %w", err)
	}
	
	// Start venue connections
	for _, venueConfig := range hsm.config.Venues {
		if !venueConfig.Enabled {
			continue
		}
		
		if err := hsm.connectVenue(&venueConfig, symbols); err != nil {
			log.Error().Err(err).Str("venue", venueConfig.Name).Msg("Failed to connect to venue")
			continue
		}
	}
	
	// Start metrics collection
	hsm.wg.Add(1)
	go hsm.metricsCollector()
	
	// Start microstructure processing
	hsm.wg.Add(1)
	go hsm.microstructureProcessor()
	
	log.Info().Int("venues", len(hsm.connections)).Msg("Hot set manager started")
	return nil
}

// Stop gracefully shuts down all connections
func (hsm *HotSetManager) Stop() error {
	log.Info().Msg("Stopping hot set manager")
	
	hsm.cancel()
	
	// Close all venue connections
	hsm.mu.Lock()
	for venue, conn := range hsm.connections {
		if err := conn.Close(); err != nil {
			log.Warn().Err(err).Str("venue", venue).Msg("Error closing venue connection")
		}
	}
	hsm.connections = make(map[string]*VenueConnection)
	hsm.mu.Unlock()
	
	// Wait for goroutines to finish
	hsm.wg.Wait()
	
	log.Info().Msg("Hot set manager stopped")
	return nil
}

// Subscribe adds a subscriber for tick updates
func (hsm *HotSetManager) Subscribe() <-chan *TickUpdate {
	ch := make(chan *TickUpdate, 1000) // Buffered for high throughput
	
	hsm.mu.Lock()
	hsm.subscribers = append(hsm.subscribers, ch)
	hsm.mu.Unlock()
	
	return ch
}

// GetMicrostructure returns current microstructure data for a symbol
func (hsm *HotSetManager) GetMicrostructure(symbol string) (*domain.MicrostructureMetrics, error) {
	return hsm.microstructure.GetMetrics(symbol)
}

// GetLatencyHistograms returns latency performance metrics
func (hsm *HotSetManager) GetLatencyHistograms() map[string]*metrics.Histogram {
	hsm.metrics.mu.RLock()
	defer hsm.metrics.mu.RUnlock()
	
	return map[string]*metrics.Histogram{
		"normalize": hsm.metrics.NormalizeLatency,
		"process":   hsm.metrics.ProcessLatency,
		"serve":     hsm.metrics.ServeLatency,
	}
}

// GetLatencyMonitor returns the latency monitor for detailed metrics
func (hsm *HotSetManager) GetLatencyMonitor() *LatencyMonitor {
	return hsm.latencyMonitor
}

// connectVenue establishes a WebSocket connection to a venue
func (hsm *HotSetManager) connectVenue(config *VenueConfig, symbols []string) error {
	log.Info().Str("venue", config.Name).Str("endpoint", config.WSEndpoint).Msg("Connecting to venue")
	
	conn, _, err := websocket.DefaultDialer.Dial(config.WSEndpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", config.Name, err)
	}
	
	venueConn := &VenueConnection{
		venue:      config.Name,
		conn:       conn,
		config:     config,
		subscribed: make(map[string]bool),
		lastPong:   time.Now(),
	}
	
	// Initialize venue metrics
	venueConn.metrics = &VenueMetrics{
		ConnectionUptime:  time.Now(),
		MessagesReceived:  metrics.NewCounter(fmt.Sprintf("hotset_%s_messages_received", config.Name)),
		MessagesProcessed: metrics.NewCounter(fmt.Sprintf("hotset_%s_messages_processed", config.Name)),
		ReconnectCount:    metrics.NewCounter(fmt.Sprintf("hotset_%s_reconnects", config.Name)),
		TicksPerSecond:    metrics.NewGauge(fmt.Sprintf("hotset_%s_tps", config.Name)),
	}
	
	hsm.mu.Lock()
	hsm.connections[config.Name] = venueConn
	hsm.mu.Unlock()
	
	// Subscribe to symbols
	if err := hsm.subscribeSymbols(venueConn, symbols); err != nil {
		venueConn.Close()
		return fmt.Errorf("failed to subscribe to symbols on %s: %w", config.Name, err)
	}
	
	// Start message processing for this venue
	hsm.wg.Add(2)
	go hsm.venueMessageHandler(venueConn)
	go hsm.venuePingHandler(venueConn)
	
	hsm.metrics.ConnectedVenues.Inc()
	
	return nil
}

// subscribeSymbols sends subscription messages for the given symbols
func (hsm *HotSetManager) subscribeSymbols(venueConn *VenueConnection, symbols []string) error {
	// Create venue-specific subscription message
	subMsg := make(map[string]interface{})
	for k, v := range venueConn.config.SubscribeMsg {
		subMsg[k] = v
	}
	
	// Add symbols to subscription
	subMsg["symbols"] = symbols
	
	msgBytes, err := json.Marshal(subMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal subscription message: %w", err)
	}
	
	if err := venueConn.conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		return fmt.Errorf("failed to send subscription message: %w", err)
	}
	
	// Mark symbols as subscribed
	venueConn.mu.Lock()
	for _, symbol := range symbols {
		venueConn.subscribed[symbol] = true
	}
	venueConn.mu.Unlock()
	
	log.Info().Str("venue", venueConn.venue).Int("symbols", len(symbols)).Msg("Subscribed to symbols")
	
	return nil
}

// venueMessageHandler processes incoming WebSocket messages from a venue
func (hsm *HotSetManager) venueMessageHandler(venueConn *VenueConnection) {
	defer hsm.wg.Done()
	defer venueConn.Close()
	
	for {
		select {
		case <-hsm.ctx.Done():
			return
		default:
		}
		
		messageType, message, err := venueConn.conn.ReadMessage()
		if err != nil {
			log.Error().Err(err).Str("venue", venueConn.venue).Msg("WebSocket read error")
			hsm.metrics.ErrorCount.Inc()
			return
		}
		
		venueConn.metrics.MessagesReceived.Inc()
		
		if messageType == websocket.PongMessage {
			venueConn.mu.Lock()
			venueConn.lastPong = time.Now()
			venueConn.mu.Unlock()
			continue
		}
		
		if messageType != websocket.TextMessage {
			continue
		}
		
		// Start latency probe
		probe := hsm.latencyMonitor.StartProbe("unknown")
		probe.RecordIngest()
		
		// Normalize the message to our standard format
		tick, err := hsm.normalizeTick(venueConn.venue, message)
		if err != nil {
			log.Debug().Err(err).Str("venue", venueConn.venue).Msg("Failed to normalize tick")
			hsm.metrics.ErrorCount.Inc()
			continue
		}
		
		if tick == nil {
			continue // Not a tick message
		}
		
		probe.Symbol = tick.Symbol
		probe.RecordNormalize()
		
		normalizeLatency := probe.NormalizeTime.Sub(probe.IngestTime)
		hsm.metrics.NormalizeLatency.Observe(float64(normalizeLatency.Nanoseconds()) / 1e6)
		
		// Check for stale data
		if time.Since(tick.Timestamp) > hsm.config.StaleThreshold {
			hsm.metrics.StaleTickCount.Inc()
			continue
		}
		
		tick.ProcessingLatency = normalizeLatency
		venueConn.metrics.MessagesProcessed.Inc()
		venueConn.metrics.LastTickTime = time.Now()
		
		// Process microstructure data
		hsm.microstructure.ProcessTick(tick)
		probe.RecordProcess()
		processLatency := probe.ProcessTime.Sub(probe.NormalizeTime)
		hsm.metrics.ProcessLatency.Observe(float64(processLatency.Nanoseconds()) / 1e6)
		
		// Distribute to subscribers
		hsm.distributeTick(tick)
		probe.RecordServe()
		serveLatency := probe.ServeTime.Sub(probe.ProcessTime)
		hsm.metrics.ServeLatency.Observe(float64(serveLatency.Nanoseconds()) / 1e6)
		
		// Finish latency probe
		hsm.latencyMonitor.Finish(probe)
		
		hsm.metrics.IngestCount.Inc()
	}
}

// distributeTick sends tick updates to all subscribers
func (hsm *HotSetManager) distributeTick(tick *TickUpdate) {
	hsm.mu.RLock()
	subscribers := make([]chan *TickUpdate, len(hsm.subscribers))
	copy(subscribers, hsm.subscribers)
	hsm.mu.RUnlock()
	
	for _, ch := range subscribers {
		select {
		case ch <- tick:
		default:
			// Channel full, skip this subscriber to avoid blocking
			log.Warn().Str("symbol", tick.Symbol).Msg("Subscriber channel full, dropping tick")
		}
	}
}

// Close closes the venue connection
func (vc *VenueConnection) Close() error {
	if vc.conn != nil {
		return vc.conn.Close()
	}
	return nil
}

// metricsCollector periodically updates venue metrics
func (hsm *HotSetManager) metricsCollector() {
	defer hsm.wg.Done()
	
	ticker := time.NewTicker(hsm.config.MetricsInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-hsm.ctx.Done():
			return
		case <-ticker.C:
			hsm.collectVenueMetrics()
		}
	}
}

// collectVenueMetrics updates metrics for all venues
func (hsm *HotSetManager) collectVenueMetrics() {
	hsm.mu.RLock()
	venues := make([]*VenueConnection, 0, len(hsm.connections))
	for _, conn := range hsm.connections {
		venues = append(venues, conn)
	}
	hsm.mu.RUnlock()
	
	connectedVenues := 0
	activeSymbols := make(map[string]bool)
	
	for _, venue := range venues {
		venue.mu.RLock()
		if venue.conn != nil {
			connectedVenues++
			
			// Calculate ticks per second
			timeSince := time.Since(venue.metrics.LastTickTime)
			if timeSince > 0 && timeSince < time.Minute {
				// Estimate TPS based on recent activity
				venue.metrics.TicksPerSecond.Set(1.0 / timeSince.Seconds())
			}
			
			// Track active symbols
			for symbol := range venue.subscribed {
				activeSymbols[symbol] = true
			}
		}
		venue.mu.RUnlock()
	}
	
	// Update global metrics
	hsm.metrics.ConnectedVenues.Set(float64(connectedVenues))
	hsm.metrics.ActiveSymbols.Set(float64(len(activeSymbols)))
}

// microstructureProcessor handles background microstructure processing
func (hsm *HotSetManager) microstructureProcessor() {
	defer hsm.wg.Done()
	
	ticker := time.NewTicker(5 * time.Second) // Process every 5 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-hsm.ctx.Done():
			return
		case <-ticker.C:
			hsm.processMicrostructureUpdates()
		}
	}
}

// processMicrostructureUpdates performs background microstructure maintenance
func (hsm *HotSetManager) processMicrostructureUpdates() {
	// Get all current metrics
	allMetrics := hsm.microstructure.GetAllMetrics()
	
	now := time.Now()
	staleThreshold := hsm.config.StaleThreshold
	
	for symbol, metrics := range allMetrics {
		// Check for stale data
		if now.Sub(metrics.LastUpdate) > staleThreshold {
			log.Debug().Str("symbol", symbol).
				Dur("age", now.Sub(metrics.LastUpdate)).
				Msg("Microstructure data is stale")
			hsm.metrics.StaleTickCount.Inc()
		}
		
		// Log health warnings
		if !metrics.MicrostructureOK {
			reasons := []string{}
			if !metrics.SpreadOK {
				reasons = append(reasons, "spread")
			}
			if !metrics.DepthOK {
				reasons = append(reasons, "depth")
			}
			if !metrics.VADROK {
				reasons = append(reasons, "vadr")
			}
			if !metrics.VenueHealthOK {
				reasons = append(reasons, "venue")
			}
			
			log.Debug().Str("symbol", symbol).
				Strs("failed_gates", reasons).
				Msg("Microstructure gates not passing")
		}
	}
}

// venuePingHandler manages WebSocket keepalive for a venue
func (hsm *HotSetManager) venuePingHandler(venueConn *VenueConnection) {
	defer hsm.wg.Done()
	
	if venueConn.config.PingInterval <= 0 {
		// No ping required for this venue
		return
	}
	
	ticker := time.NewTicker(venueConn.config.PingInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-hsm.ctx.Done():
			return
		case <-ticker.C:
			if err := hsm.sendPing(venueConn); err != nil {
				log.Error().Err(err).Str("venue", venueConn.venue).
					Msg("Failed to send ping")
				hsm.metrics.ErrorCount.Inc()
				
				// Check if pong timeout exceeded
				venueConn.mu.RLock()
				pongAge := time.Since(venueConn.lastPong)
				venueConn.mu.RUnlock()
				
				if venueConn.config.PongTimeout > 0 && pongAge > venueConn.config.PongTimeout {
					log.Warn().Str("venue", venueConn.venue).
						Dur("pong_age", pongAge).
						Msg("Pong timeout exceeded, connection may be dead")
					return // Exit ping handler, connection will be cleaned up
				}
			}
		}
	}
}

// sendPing sends a ping message to the venue
func (hsm *HotSetManager) sendPing(venueConn *VenueConnection) error {
	if venueConn.conn == nil {
		return fmt.Errorf("connection is nil")
	}
	
	// Send WebSocket ping
	venueConn.mu.Lock()
	err := venueConn.conn.WriteMessage(websocket.PingMessage, []byte{})
	venueConn.mu.Unlock()
	
	return err
}