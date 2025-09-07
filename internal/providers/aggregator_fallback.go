//go:build with_agg
// +build with_agg

// Package providers - Aggregator fallback support (compile-time optional)
// This file is only included when building with -tags with_agg
// CRITICAL: Aggregators NEVER provide microstructure data (depth/spread/orderbook)
package providers

import (
	"context"
	"fmt"
	"time"
)

// AggregatorFallbackProvider provides fallback data for non-microstructure use cases
// IMPORTANT: This provider is explicitly BANNED from providing microstructure data
type AggregatorFallbackProvider struct {
	name        string
	baseURL     string
	rateLimiter RateLimiter
	guard       *ExchangeNativeGuard
}

// NewAggregatorFallbackProvider creates a fallback provider with strict guards
func NewAggregatorFallbackProvider(name, baseURL string) (*AggregatorFallbackProvider, error) {
	// Validate against banned aggregator names
	guard := NewExchangeNativeGuard()
	if err := guard.ValidateDataSource(name, DataTypeMicrostructure); err != nil {
		return nil, fmt.Errorf("aggregator validation failed: %w", err)
	}
	
	return &AggregatorFallbackProvider{
		name:    name,
		baseURL: baseURL,
		guard:   guard,
	}, nil
}

// GetPriceData provides price/volume data ONLY - microstructure BANNED
func (a *AggregatorFallbackProvider) GetPriceData(ctx context.Context, symbol string) (*PriceData, error) {
	// Validate this is not being used for microstructure
	if err := a.guard.ValidateDataSource(a.name, DataTypePricing); err != nil {
		return nil, fmt.Errorf("aggregator guard violation: %w", err)
	}
	
	// Implementation would fetch price data from aggregator API
	// This is a stub implementation
	return &PriceData{
		Symbol:    symbol,
		Price:     0.0,
		Volume24h: 0.0,
		Source:    fmt.Sprintf("aggregator_fallback_%s", a.name),
		Timestamp: time.Now(),
		Warning:   "AGGREGATOR FALLBACK - exchange-native preferred",
	}, fmt.Errorf("aggregator fallback not implemented - use exchange-native sources")
}

// GetOrderBook is explicitly BANNED - will always return error
func (a *AggregatorFallbackProvider) GetOrderBook(ctx context.Context, symbol string) (*OrderBookData, error) {
	return nil, &AggregatorViolationError{
		Source:   a.name,
		DataType: DataTypeMicrostructure,
		Reason:   "Aggregators BANNED from providing order book data - use exchange-native only",
	}
}

// GetDepthData is explicitly BANNED - will always return error  
func (a *AggregatorFallbackProvider) GetDepthData(ctx context.Context, symbol string) (*DepthData, error) {
	return nil, &AggregatorViolationError{
		Source:   a.name,
		DataType: DataTypeMicrostructure,
		Reason:   "Aggregators BANNED from providing depth data - use exchange-native only",
	}
}

// GetSpreadData is explicitly BANNED - will always return error
func (a *AggregatorFallbackProvider) GetSpreadData(ctx context.Context, symbol string) (*SpreadData, error) {
	return nil, &AggregatorViolationError{
		Source:   a.name,
		DataType: DataTypeMicrostructure,
		Reason:   "Aggregators BANNED from providing spread data - use exchange-native only",
	}
}

// Validate ensures aggregator is not being used for banned operations
func (a *AggregatorFallbackProvider) Validate() error {
	return a.guard.ValidateProvider(a)
}

// Data structures

type PriceData struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Volume24h float64   `json:"volume_24h"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
	Warning   string    `json:"warning,omitempty"`
}

type OrderBookData struct {
	Symbol    string      `json:"symbol"`
	Bids      [][]float64 `json:"bids"` // [price, volume] pairs
	Asks      [][]float64 `json:"asks"` // [price, volume] pairs
	Source    string      `json:"source"`
	Timestamp time.Time   `json:"timestamp"`
}

type DepthData struct {
	Symbol    string    `json:"symbol"`
	BidDepth  float64   `json:"bid_depth"`
	AskDepth  float64   `json:"ask_depth"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
}

type SpreadData struct {
	Symbol    string    `json:"symbol"`
	SpreadBps float64   `json:"spread_bps"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
}

// RateLimiter interface for aggregator providers
type RateLimiter interface {
	Wait(ctx context.Context) error
}