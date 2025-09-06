// Package data provides WebSocket stream implementations and multiplexing
package data

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MockStream implements Stream interface for testing
type MockStream struct {
	exchange     string
	subscribers  map[string]bool
	tradesCh     chan Trade
	booksCh      chan BookSnapshot
	barsCh       chan Bar
	health       StreamHealth
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	closed       bool
}

// NewMockStream creates a new mock stream for testing
func NewMockStream(exchange string) *MockStream {
	ctx, cancel := context.WithCancel(context.Background())
	
	stream := &MockStream{
		exchange:    exchange,
		subscribers: make(map[string]bool),
		tradesCh:    make(chan Trade, 100),
		booksCh:     make(chan BookSnapshot, 100),
		barsCh:      make(chan Bar, 100),
		ctx:         ctx,
		cancel:      cancel,
		health: StreamHealth{
			Connected:    true,
			Exchange:     exchange,
			LastMessage:  time.Now(),
			MessageCount: 0,
			Reconnects:   0,
			LatencyMs:    1.5,
		},
	}
	
	// Start mock data generation
	go stream.generateMockData()
	
	return stream
}

// Subscribe subscribes to symbols and data types
func (m *MockStream) Subscribe(symbols []string, dataTypes []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.closed {
		return fmt.Errorf("stream is closed")
	}
	
	for _, symbol := range symbols {
		m.subscribers[symbol] = true
	}
	
	// Simulate subscription success
	m.health.MessageCount++
	m.health.LastMessage = time.Now()
	
	return nil
}

// Unsubscribe unsubscribes from symbols
func (m *MockStream) Unsubscribe(symbols []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, symbol := range symbols {
		delete(m.subscribers, symbol)
	}
	
	return nil
}

// Trades returns the trades channel
func (m *MockStream) Trades() <-chan Trade {
	return m.tradesCh
}

// Books returns the books channel
func (m *MockStream) Books() <-chan BookSnapshot {
	return m.booksCh
}

// Bars returns the bars channel
func (m *MockStream) Bars() <-chan Bar {
	return m.barsCh
}

// Health returns stream health status
func (m *MockStream) Health() StreamHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.health
}

// Close closes the stream
func (m *MockStream) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.closed {
		return nil
	}
	
	m.closed = true
	m.cancel()
	close(m.tradesCh)
	close(m.booksCh)
	close(m.barsCh)
	
	m.health.Connected = false
	
	return nil
}

// generateMockData generates mock data for testing
func (m *MockStream) generateMockData() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.generateMockTrade()
			m.generateMockBook()
			m.generateMockBar()
		}
	}
}

func (m *MockStream) generateMockTrade() {
	m.mu.RLock()
	symbols := make([]string, 0, len(m.subscribers))
	for symbol := range m.subscribers {
		symbols = append(symbols, symbol)
	}
	m.mu.RUnlock()
	
	if len(symbols) == 0 {
		return
	}
	
	// Generate mock trade for first subscribed symbol
	symbol := symbols[0]
	trade := Trade{
		Symbol:    symbol,
		Timestamp: time.Now(),
		Price:     45000.0 + float64(time.Now().Unix()%1000), // Mock price
		Size:      0.1,
		Side:      "buy",
		Source:    m.exchange,
	}
	
	select {
	case m.tradesCh <- trade:
		m.health.MessageCount++
		m.health.LastMessage = time.Now()
	default:
		// Channel full
	}
}

func (m *MockStream) generateMockBook() {
	m.mu.RLock()
	symbols := make([]string, 0, len(m.subscribers))
	for symbol := range m.subscribers {
		symbols = append(symbols, symbol)
	}
	m.mu.RUnlock()
	
	if len(symbols) == 0 {
		return
	}
	
	symbol := symbols[0]
	book := BookSnapshot{
		Symbol:    symbol,
		Timestamp: time.Now(),
		Bids: []BookLevel{
			{Price: 44999.0, Size: 1.5},
			{Price: 44998.0, Size: 2.0},
		},
		Asks: []BookLevel{
			{Price: 45001.0, Size: 1.2},
			{Price: 45002.0, Size: 1.8},
		},
		Source: m.exchange,
	}
	
	select {
	case m.booksCh <- book:
		m.health.MessageCount++
		m.health.LastMessage = time.Now()
	default:
		// Channel full
	}
}

func (m *MockStream) generateMockBar() {
	m.mu.RLock()
	symbols := make([]string, 0, len(m.subscribers))
	for symbol := range m.subscribers {
		symbols = append(symbols, symbol)
	}
	m.mu.RUnlock()
	
	if len(symbols) == 0 {
		return
	}
	
	symbol := symbols[0]
	basePrice := 45000.0
	bar := Bar{
		Symbol:    symbol,
		Timestamp: time.Now().Truncate(time.Minute),
		Open:      basePrice,
		High:      basePrice + 100,
		Low:       basePrice - 50,
		Close:     basePrice + 25,
		Volume:    1000.0,
		Source:    m.exchange,
	}
	
	select {
	case m.barsCh <- bar:
		m.health.MessageCount++
		m.health.LastMessage = time.Now()
	default:
		// Channel full
	}
}

// MultiplexedStream combines multiple exchange streams
type MultiplexedStream struct {
	streams   map[string]Stream
	symbols   []string
	tradesCh  chan Trade
	booksCh   chan BookSnapshot
	barsCh    chan Bar
	ctx       context.Context
	cancel    context.CancelFunc
	closed    bool
	mu        sync.RWMutex
}

// NewMultiplexedStream creates a multiplexed stream
func NewMultiplexedStream(streams map[string]Stream, symbols []string) *MultiplexedStream {
	ctx, cancel := context.WithCancel(context.Background())
	
	ms := &MultiplexedStream{
		streams:  streams,
		symbols:  symbols,
		tradesCh: make(chan Trade, 1000),
		booksCh:  make(chan BookSnapshot, 1000),
		barsCh:   make(chan Bar, 1000),
		ctx:      ctx,
		cancel:   cancel,
	}
	
	// Start multiplexing
	go ms.multiplex()
	
	return ms
}

// multiplex forwards data from all streams to output channels
func (ms *MultiplexedStream) multiplex() {
	var wg sync.WaitGroup
	
	for exchange, stream := range ms.streams {
		wg.Add(1)
		go func(ex string, s Stream) {
			defer wg.Done()
			ms.multiplexStream(ex, s)
		}(exchange, stream)
	}
	
	wg.Wait()
}

func (ms *MultiplexedStream) multiplexStream(exchange string, stream Stream) {
	for {
		select {
		case <-ms.ctx.Done():
			return
		case trade, ok := <-stream.Trades():
			if !ok {
				return
			}
			select {
			case ms.tradesCh <- trade:
			case <-ms.ctx.Done():
				return
			default:
				// Channel full, drop message
			}
		case book, ok := <-stream.Books():
			if !ok {
				return
			}
			select {
			case ms.booksCh <- book:
			case <-ms.ctx.Done():
				return
			default:
				// Channel full, drop message
			}
		case bar, ok := <-stream.Bars():
			if !ok {
				return
			}
			select {
			case ms.barsCh <- bar:
			case <-ms.ctx.Done():
				return
			default:
				// Channel full, drop message
			}
		}
	}
}

// Subscribe is a no-op for multiplexed stream (subscription handled by individual streams)
func (ms *MultiplexedStream) Subscribe(symbols []string, dataTypes []string) error {
	return nil
}

// Unsubscribe is a no-op for multiplexed stream
func (ms *MultiplexedStream) Unsubscribe(symbols []string) error {
	return nil
}

// Trades returns the multiplexed trades channel
func (ms *MultiplexedStream) Trades() <-chan Trade {
	return ms.tradesCh
}

// Books returns the multiplexed books channel
func (ms *MultiplexedStream) Books() <-chan BookSnapshot {
	return ms.booksCh
}

// Bars returns the multiplexed bars channel
func (ms *MultiplexedStream) Bars() <-chan Bar {
	return ms.barsCh
}

// Health returns aggregated health from all streams
func (ms *MultiplexedStream) Health() StreamHealth {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	health := StreamHealth{
		Exchange:     "multiplexed",
		Connected:    true,
		LastMessage:  time.Now(),
		MessageCount: 0,
		Reconnects:   0,
	}
	
	// Aggregate health from all streams
	for _, stream := range ms.streams {
		streamHealth := stream.Health()
		if !streamHealth.Connected {
			health.Connected = false
		}
		health.MessageCount += streamHealth.MessageCount
		health.Reconnects += streamHealth.Reconnects
		health.ErrorCount += streamHealth.ErrorCount
		
		if streamHealth.LastMessage.After(health.LastMessage) {
			health.LastMessage = streamHealth.LastMessage
		}
	}
	
	// Calculate average latency
	if len(ms.streams) > 0 {
		totalLatency := 0.0
		for _, stream := range ms.streams {
			totalLatency += stream.Health().LatencyMs
		}
		health.LatencyMs = totalLatency / float64(len(ms.streams))
	}
	
	return health
}

// Close closes the multiplexed stream
func (ms *MultiplexedStream) Close() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	if ms.closed {
		return nil
	}
	
	ms.closed = true
	ms.cancel()
	
	close(ms.tradesCh)
	close(ms.booksCh)
	close(ms.barsCh)
	
	return nil
}