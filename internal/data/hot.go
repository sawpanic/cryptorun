package data

import (
	"context"
	"fmt"
	"sync"
)

// HotData implements real-time WebSocket data tier
type HotData struct {
	clients map[string]WSClient  // venue -> client
	cache   map[string]*Envelope // key -> latest data
	mutex   sync.RWMutex

	// Configuration
	tickBufferSize   int
	staleThresholdMS int64
}

// WSClient interface for venue-specific WebSocket implementations
type WSClient interface {
	Connect() error
	Disconnect() error
	Subscribe(symbol string) error
	Unsubscribe(symbol string) error
	IsConnected() bool
	GetLastTick(symbol string) (*Envelope, error)
}

// NewHotData creates a new hot data tier
func NewHotData() *HotData {
	return &HotData{
		clients:          make(map[string]WSClient),
		cache:            make(map[string]*Envelope),
		tickBufferSize:   100,
		staleThresholdMS: 5000, // 5 seconds
	}
}

// RegisterClient adds a WebSocket client for a venue
func (h *HotData) RegisterClient(venue string, client WSClient) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.clients[venue] = client
}

// GetOrderBook retrieves real-time order book data
func (h *HotData) GetOrderBook(ctx context.Context, venue, symbol string) (*Envelope, error) {
	h.mutex.RLock()
	client, exists := h.clients[venue]
	h.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no WebSocket client for venue: %s", venue)
	}

	if !client.IsConnected() {
		return nil, fmt.Errorf("WebSocket client not connected for venue: %s", venue)
	}

	envelope, err := client.GetLastTick(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get tick for %s %s: %w", venue, symbol, err)
	}

	// Update freshness and cache
	envelope.CalculateFreshness()
	envelope.SourceTier = TierHot

	h.cacheEnvelope(venue, symbol, envelope)

	return envelope, nil
}

// GetPriceData retrieves real-time price data (same as order book for hot tier)
func (h *HotData) GetPriceData(ctx context.Context, venue, symbol string) (*Envelope, error) {
	return h.GetOrderBook(ctx, venue, symbol)
}

// IsAvailable checks if hot data is available for venue
func (h *HotData) IsAvailable(ctx context.Context, venue string) bool {
	h.mutex.RLock()
	client, exists := h.clients[venue]
	h.mutex.RUnlock()

	return exists && client.IsConnected()
}

// Subscribe to real-time data for a symbol
func (h *HotData) Subscribe(venue, symbol string) error {
	h.mutex.RLock()
	client, exists := h.clients[venue]
	h.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("no WebSocket client for venue: %s", venue)
	}

	return client.Subscribe(symbol)
}

// Unsubscribe from real-time data for a symbol
func (h *HotData) Unsubscribe(venue, symbol string) error {
	h.mutex.RLock()
	client, exists := h.clients[venue]
	h.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("no WebSocket client for venue: %s", venue)
	}

	return client.Unsubscribe(symbol)
}

// GetLatestTick retrieves the most recent tick without freshness validation
func (h *HotData) GetLatestTick(venue, symbol string) (*Envelope, error) {
	key := fmt.Sprintf("%s:%s", venue, symbol)

	h.mutex.RLock()
	envelope, exists := h.cache[key]
	h.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no cached data for %s %s", venue, symbol)
	}

	return envelope, nil
}

// cacheEnvelope stores envelope in local cache
func (h *HotData) cacheEnvelope(venue, symbol string, envelope *Envelope) {
	key := fmt.Sprintf("%s:%s", venue, symbol)

	h.mutex.Lock()
	h.cache[key] = envelope
	h.mutex.Unlock()
}

// CleanupStaleData removes stale cached data
func (h *HotData) CleanupStaleData() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	for key, envelope := range h.cache {
		if envelope.FreshnessMS > h.staleThresholdMS {
			delete(h.cache, key)
		}
	}
}

// GetStats returns hot tier statistics
func (h *HotData) GetStats() map[string]interface{} {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	connectedVenues := 0
	for _, client := range h.clients {
		if client.IsConnected() {
			connectedVenues++
		}
	}

	return map[string]interface{}{
		"total_venues":       len(h.clients),
		"connected_venues":   connectedVenues,
		"cached_symbols":     len(h.cache),
		"stale_threshold_ms": h.staleThresholdMS,
	}
}
