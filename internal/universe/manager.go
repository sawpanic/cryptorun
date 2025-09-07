package universe

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/interfaces"
)

// Symbol represents a trading symbol with metadata
type Symbol struct {
	Symbol         string            `yaml:"symbol" json:"symbol"`
	ExchangePair   map[string]string `yaml:",inline" json:"exchange_pairs"`
	MarketCapRank  int               `yaml:"market_cap_rank" json:"market_cap_rank"`
	Priority       int               `yaml:"priority" json:"priority"`
	Tier           string            `json:"tier"`
	PreferredVenue string            `json:"preferred_venue"`
}

// UniverseConfig defines the trading universe configuration
type UniverseConfig struct {
	Universe struct {
		Name        string   `yaml:"name"`
		Description string   `yaml:"description"`
		LastUpdated string   `yaml:"last_updated"`
		Filters     struct {
			MinMarketCapUSD     int64    `yaml:"min_market_cap_usd"`
			MinDailyVolumeUSD   int64    `yaml:"min_daily_volume_usd"`
			MaxSpreadBps        float64  `yaml:"max_spread_bps"`
			MinDepthUSD         float64  `yaml:"min_depth_usd"`
			Exchanges           []string `yaml:"exchanges"`
			QuoteCurrencies     []string `yaml:"quote_currencies"`
		} `yaml:"filters"`
		ExchangeRouting struct {
			Primary       string   `yaml:"primary"`
			FallbackChain []string `yaml:"fallback_chain"`
		} `yaml:"exchange_routing"`
		GlobalLimits struct {
			MaxConcurrentRequests int `yaml:"max_concurrent_requests"`
			InterRequestDelayMs   int `yaml:"inter_request_delay_ms"`
			BatchSize             int `yaml:"batch_size"`
		} `yaml:"global_limits"`
	} `yaml:"universe"`
	
	Symbols map[string]map[string][]struct {
		Symbol        string `yaml:"symbol"`
		KrakenPair    string `yaml:"kraken_pair,omitempty"`
		BinancePair   string `yaml:"binance_pair,omitempty"`
		CoinbasePair  string `yaml:"coinbase_pair,omitempty"`
		OKXPair       string `yaml:"okx_pair,omitempty"`
		MarketCapRank int    `yaml:"market_cap_rank"`
		Priority      int    `yaml:"priority"`
	} `yaml:"symbols"`
	
	ScanningStrategies map[string]struct {
		MaxSymbols     int      `yaml:"max_symbols"`
		Tiers          []string `yaml:"tiers"`
		Exchanges      []string `yaml:"exchanges"`
		TimeoutSeconds int      `yaml:"timeout_seconds"`
	} `yaml:"scanning_strategies"`
	
	Performance struct {
		LatencyTargets struct {
			P50Ms int `yaml:"p50_ms"`
			P95Ms int `yaml:"p95_ms"`
			P99Ms int `yaml:"p99_ms"`
		} `yaml:"latency_targets"`
		SuccessRates struct {
			MinSuccessRate       float64 `yaml:"min_success_rate"`
			FallbackTriggerRate  float64 `yaml:"fallback_trigger_rate"`
		} `yaml:"success_rates"`
		ResourceLimits struct {
			MaxMemoryMb    int `yaml:"max_memory_mb"`
			MaxCpuPercent  int `yaml:"max_cpu_percent"`
		} `yaml:"resource_limits"`
	} `yaml:"performance"`
}

// ScanRequest defines parameters for a universe scan
type ScanRequest struct {
	Strategy       string
	MaxSymbols     int
	Tiers          []string
	Exchanges      []string
	TimeoutSeconds int
	Regime         string
	MinScore       float64
	DryRun         bool
}

// ScanResult represents the result of scanning a symbol
type ScanResult struct {
	Symbol         string            `json:"symbol"`
	Venue          string            `json:"venue"`
	Score          float64           `json:"score"`
	Price          float64           `json:"price"`
	SpreadBps      float64           `json:"spread_bps"`
	DepthUSD       float64           `json:"depth_usd"`
	VADR           float64           `json:"vadr"`
	PassesGates    bool              `json:"passes_gates"`
	FailedGates    []string          `json:"failed_gates,omitempty"`
	Priority       int               `json:"priority"`
	Tier           string            `json:"tier"`
	MarketCapRank  int               `json:"market_cap_rank"`
	Timestamp      time.Time         `json:"timestamp"`
	Latency        time.Duration     `json:"latency"`
	Attribution    map[string]string `json:"attribution"`
}

// UniverseScanSummary provides comprehensive scan results
type UniverseScanSummary struct {
	Strategy        string                 `json:"strategy"`
	TotalSymbols    int                    `json:"total_symbols"`
	SuccessfulScans int                    `json:"successful_scans"`
	CandidatesFound int                    `json:"candidates_found"`
	ScanDuration    time.Duration          `json:"scan_duration"`
	Timestamp       time.Time              `json:"timestamp"`
	VenueStats      map[string]VenueStats  `json:"venue_stats"`
	TierStats       map[string]int         `json:"tier_stats"`
	PerformanceStats PerformanceStats      `json:"performance_stats"`
	Errors          []string               `json:"errors,omitempty"`
}

type VenueStats struct {
	Requested int     `json:"requested"`
	Successful int    `json:"successful"`
	Failed     int     `json:"failed"`
	AvgLatency float64 `json:"avg_latency_ms"`
	SuccessRate float64 `json:"success_rate"`
}

type PerformanceStats struct {
	P50LatencyMs int `json:"p50_latency_ms"`
	P95LatencyMs int `json:"p95_latency_ms"`
	P99LatencyMs int `json:"p99_latency_ms"`
	TotalRequests int `json:"total_requests"`
	FailedRequests int `json:"failed_requests"`
	OverallSuccessRate float64 `json:"overall_success_rate"`
}

// Manager coordinates universe scanning across multiple exchanges
type Manager struct {
	config    *UniverseConfig
	exchanges map[string]interfaces.Exchange
	symbols   []Symbol
	mu        sync.RWMutex
}

// NewManager creates a universe manager with configuration
func NewManager(config *UniverseConfig, exchanges map[string]interfaces.Exchange) *Manager {
	manager := &Manager{
		config:    config,
		exchanges: exchanges,
		symbols:   make([]Symbol, 0, 200),
	}
	
	manager.buildUniverseFromConfig()
	return manager
}

// buildUniverseFromConfig constructs the symbol universe from YAML config
func (m *Manager) buildUniverseFromConfig() {
	for exchangeName, tiers := range m.config.Symbols {
		for tierName, symbols := range tiers {
			for _, symbolConfig := range symbols {
				symbol := Symbol{
					Symbol:        symbolConfig.Symbol,
					MarketCapRank: symbolConfig.MarketCapRank,
					Priority:      symbolConfig.Priority,
					Tier:          tierName,
					ExchangePair:  make(map[string]string),
				}
				
				// Set default preferred venue to the exchange this symbol came from
				symbol.PreferredVenue = exchangeName
				
				// Map exchange-specific pairs
				if symbolConfig.KrakenPair != "" {
					symbol.ExchangePair["kraken"] = symbolConfig.KrakenPair
				}
				if symbolConfig.BinancePair != "" {
					symbol.ExchangePair["binance"] = symbolConfig.BinancePair
				}
				if symbolConfig.CoinbasePair != "" {
					symbol.ExchangePair["coinbase"] = symbolConfig.CoinbasePair
				}
				if symbolConfig.OKXPair != "" {
					symbol.ExchangePair["okx"] = symbolConfig.OKXPair
				}
				
				// Determine preferred venue based on routing config
				if _, hasKraken := symbol.ExchangePair[m.config.Universe.ExchangeRouting.Primary]; hasKraken {
					symbol.PreferredVenue = m.config.Universe.ExchangeRouting.Primary
				} else {
					// Use first available fallback
					for _, fallback := range m.config.Universe.ExchangeRouting.FallbackChain {
						if _, hasFallback := symbol.ExchangePair[fallback]; hasFallback {
							symbol.PreferredVenue = fallback
							break
						}
					}
				}
				
				m.symbols = append(m.symbols, symbol)
			}
		}
	}
	
	// Sort by priority then market cap rank
	sort.Slice(m.symbols, func(i, j int) bool {
		if m.symbols[i].Priority != m.symbols[j].Priority {
			return m.symbols[i].Priority < m.symbols[j].Priority
		}
		return m.symbols[i].MarketCapRank < m.symbols[j].MarketCapRank
	})
}

// GetSymbols returns filtered symbol list based on scan request
func (m *Manager) GetSymbols(req ScanRequest) []Symbol {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var filtered []Symbol
	
	// Get strategy configuration if specified
	if req.Strategy != "" {
		if strategy, exists := m.config.ScanningStrategies[req.Strategy]; exists {
			req.MaxSymbols = strategy.MaxSymbols
			req.Tiers = strategy.Tiers
			req.Exchanges = strategy.Exchanges
			req.TimeoutSeconds = strategy.TimeoutSeconds
		}
	}
	
	tierSet := make(map[string]bool)
	for _, tier := range req.Tiers {
		tierSet[tier] = true
	}
	
	exchangeSet := make(map[string]bool)
	for _, exchange := range req.Exchanges {
		exchangeSet[exchange] = true
	}
	
	for _, symbol := range m.symbols {
		// Filter by tier
		if len(tierSet) > 0 && !tierSet[symbol.Tier] {
			continue
		}
		
		// Filter by exchange availability
		if len(exchangeSet) > 0 {
			hasExchange := false
			for exchange := range exchangeSet {
				if _, exists := symbol.ExchangePair[exchange]; exists {
					hasExchange = true
					break
				}
			}
			if !hasExchange {
				continue
			}
		}
		
		filtered = append(filtered, symbol)
		
		// Respect max symbols limit
		if req.MaxSymbols > 0 && len(filtered) >= req.MaxSymbols {
			break
		}
	}
	
	return filtered
}

// ScanUniverse executes a comprehensive scan across the symbol universe
func (m *Manager) ScanUniverse(ctx context.Context, req ScanRequest) ([]ScanResult, *UniverseScanSummary, error) {
	startTime := time.Now()
	
	symbols := m.GetSymbols(req)
	if len(symbols) == 0 {
		return nil, nil, fmt.Errorf("no symbols match scan criteria")
	}
	
	// Create timeout context
	scanCtx := ctx
	if req.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		scanCtx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutSeconds)*time.Second)
		defer cancel()
	}
	
	// Scan with concurrency control
	results, errors := m.scanSymbolsConcurrent(scanCtx, symbols, req)
	
	// Calculate summary statistics
	summary := m.calculateSummary(req.Strategy, symbols, results, errors, time.Since(startTime))
	
	// Filter for candidates that pass gates
	candidates := make([]ScanResult, 0)
	for _, result := range results {
		if result.PassesGates && result.Score >= req.MinScore {
			candidates = append(candidates, result)
		}
	}
	
	// Sort candidates by score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	
	return candidates, summary, nil
}

// scanSymbolsConcurrent performs concurrent scanning with rate limiting
func (m *Manager) scanSymbolsConcurrent(ctx context.Context, symbols []Symbol, req ScanRequest) ([]ScanResult, []string) {
	maxConcurrent := m.config.Universe.GlobalLimits.MaxConcurrentRequests
	if maxConcurrent <= 0 {
		maxConcurrent = 5 // Default
	}
	
	semaphore := make(chan struct{}, maxConcurrent)
	resultsChan := make(chan ScanResult, len(symbols))
	errorsChan := make(chan string, len(symbols))
	
	var wg sync.WaitGroup
	
	for _, symbol := range symbols {
		wg.Add(1)
		go func(sym Symbol) {
			defer wg.Done()
			
			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}
			
			// Scan the symbol
			result, err := m.scanSymbol(ctx, sym, req)
			if err != nil {
				select {
				case errorsChan <- fmt.Sprintf("%s: %v", sym.Symbol, err):
				case <-ctx.Done():
				}
				return
			}
			
			select {
			case resultsChan <- result:
			case <-ctx.Done():
			}
			
			// Rate limiting delay
			if delay := m.config.Universe.GlobalLimits.InterRequestDelayMs; delay > 0 {
				time.Sleep(time.Duration(delay) * time.Millisecond)
			}
		}(symbol)
	}
	
	// Close channels when all goroutines complete
	go func() {
		wg.Wait()
		close(resultsChan)
		close(errorsChan)
	}()
	
	// Collect results
	var results []ScanResult
	var errors []string
	
	for {
		select {
		case result, ok := <-resultsChan:
			if !ok {
				resultsChan = nil
			} else {
				results = append(results, result)
			}
		case err, ok := <-errorsChan:
			if !ok {
				errorsChan = nil
			} else {
				errors = append(errors, err)
			}
		case <-ctx.Done():
			return results, append(errors, "scan timeout")
		}
		
		if resultsChan == nil && errorsChan == nil {
			break
		}
	}
	
	return results, errors
}

// scanSymbol scans a single symbol with fallback routing
func (m *Manager) scanSymbol(ctx context.Context, symbol Symbol, req ScanRequest) (ScanResult, error) {
	startTime := time.Now()
	
	// Try preferred venue first
	if exchange := m.exchanges[symbol.PreferredVenue]; exchange != nil {
		if result, err := m.scanSymbolOnExchange(ctx, symbol, exchange, symbol.PreferredVenue); err == nil {
			result.Latency = time.Since(startTime)
			return result, nil
		}
	}
	
	// Try fallback chain
	for _, venueName := range m.config.Universe.ExchangeRouting.FallbackChain {
		if venueName == symbol.PreferredVenue {
			continue // Already tried
		}
		
		if _, hasSymbol := symbol.ExchangePair[venueName]; !hasSymbol {
			continue // Symbol not available on this venue
		}
		
		if exchange := m.exchanges[venueName]; exchange != nil {
			if result, err := m.scanSymbolOnExchange(ctx, symbol, exchange, venueName); err == nil {
				result.Latency = time.Since(startTime)
				return result, nil
			}
		}
	}
	
	return ScanResult{}, fmt.Errorf("all venues failed for %s", symbol.Symbol)
}

// scanSymbolOnExchange performs the actual scanning logic for one symbol on one exchange
func (m *Manager) scanSymbolOnExchange(ctx context.Context, symbol Symbol, exchange interfaces.Exchange, venueName string) (ScanResult, error) {
	result := ScanResult{
		Symbol:        symbol.Symbol,
		Venue:         venueName,
		Priority:      symbol.Priority,
		Tier:          symbol.Tier,
		MarketCapRank: symbol.MarketCapRank,
		Timestamp:     time.Now(),
		Attribution:   make(map[string]string),
	}
	
	// Get order book for microstructure analysis
	book, err := exchange.GetBookL2(ctx, symbol.Symbol)
	if err != nil {
		return result, fmt.Errorf("failed to get order book: %w", err)
	}
	
	if len(book.Bids) == 0 || len(book.Asks) == 0 {
		return result, fmt.Errorf("empty order book")
	}
	
	// Calculate microstructure metrics
	bestBid := book.Bids[0].Price
	bestAsk := book.Asks[0].Price
	midPrice := (bestBid + bestAsk) / 2.0
	
	result.Price = midPrice
	
	// Spread calculation
	spread := (bestAsk - bestBid) / midPrice
	result.SpreadBps = spread * 10000
	
	// Depth calculation (±2%)
	bidDepth, askDepth := calculateDepthWithinRange(book, midPrice, 2.0)
	result.DepthUSD = (bidDepth + askDepth) / 2.0
	
	// VADR estimation
	result.VADR = estimateVADRFromBook(book, midPrice)
	
	// Mock composite score (would be real scorer in production)
	result.Score = generateMockScore(symbol.Symbol, symbol.MarketCapRank)
	
	// Entry gates evaluation
	gates := map[string]bool{
		"score_threshold": result.Score >= 75.0,
		"spread_limit":    result.SpreadBps < m.config.Universe.Filters.MaxSpreadBps,
		"depth_minimum":   result.DepthUSD >= m.config.Universe.Filters.MinDepthUSD,
		"vadr_threshold":  result.VADR >= 1.75,
	}
	
	result.PassesGates = true
	result.FailedGates = make([]string, 0)
	
	for gate, passed := range gates {
		if !passed {
			result.PassesGates = false
			result.FailedGates = append(result.FailedGates, gate)
		}
	}
	
	result.Attribution["source"] = fmt.Sprintf("%s_l2_%s", venueName, book.Timestamp.Format("15:04:05"))
	result.Attribution["data_age"] = time.Since(book.Timestamp).String()
	
	return result, nil
}

// calculateSummary generates comprehensive scan statistics
func (m *Manager) calculateSummary(strategy string, symbols []Symbol, results []ScanResult, errors []string, duration time.Duration) *UniverseScanSummary {
	summary := &UniverseScanSummary{
		Strategy:        strategy,
		TotalSymbols:    len(symbols),
		SuccessfulScans: len(results),
		ScanDuration:    duration,
		Timestamp:       time.Now(),
		VenueStats:      make(map[string]VenueStats),
		TierStats:       make(map[string]int),
		Errors:          errors,
	}
	
	// Count candidates
	for _, result := range results {
		if result.PassesGates {
			summary.CandidatesFound++
		}
	}
	
	// Calculate venue statistics
	venueRequested := make(map[string]int)
	venueSuccessful := make(map[string]int)
	venueLatencies := make(map[string][]float64)
	
	for _, symbol := range symbols {
		venueRequested[symbol.PreferredVenue]++
	}
	
	for _, result := range results {
		venueSuccessful[result.Venue]++
		venueLatencies[result.Venue] = append(venueLatencies[result.Venue], float64(result.Latency.Nanoseconds())/1e6)
	}
	
	for venue, requested := range venueRequested {
		successful := venueSuccessful[venue]
		failed := requested - successful
		
		var avgLatency float64
		if latencies := venueLatencies[venue]; len(latencies) > 0 {
			sum := 0.0
			for _, lat := range latencies {
				sum += lat
			}
			avgLatency = sum / float64(len(latencies))
		}
		
		summary.VenueStats[venue] = VenueStats{
			Requested:   requested,
			Successful:  successful,
			Failed:      failed,
			AvgLatency:  avgLatency,
			SuccessRate: float64(successful) / float64(requested),
		}
	}
	
	// Calculate tier statistics
	for _, result := range results {
		summary.TierStats[result.Tier]++
	}
	
	return summary
}

// Helper functions

func calculateDepthWithinRange(book *interfaces.BookL2, midPrice, percentRange float64) (bidDepth, askDepth float64) {
	lowerBound := midPrice * (1 - percentRange/100)
	upperBound := midPrice * (1 + percentRange/100)
	
	for _, bid := range book.Bids {
		if bid.Price >= lowerBound {
			bidDepth += bid.Price * bid.Size
		}
	}
	
	for _, ask := range book.Asks {
		if ask.Price <= upperBound {
			askDepth += ask.Price * ask.Size
		}
	}
	
	return bidDepth, askDepth
}

func estimateVADRFromBook(book *interfaces.BookL2, midPrice float64) float64 {
	bookDepth := len(book.Bids) + len(book.Asks)
	if bookDepth < 10 {
		return 0.8
	}
	
	bidDepth, askDepth := calculateDepthWithinRange(book, midPrice, 1.0)
	totalDepth := bidDepth + askDepth
	
	switch {
	case totalDepth > 2000000:
		return 3.5
	case totalDepth > 1000000:
		return 3.0
	case totalDepth > 500000:
		return 2.5
	case totalDepth > 200000:
		return 2.0
	case totalDepth > 100000:
		return 1.8
	default:
		return 1.2
	}
}

func generateMockScore(symbol string, marketCapRank int) float64 {
	// Generate score based on symbol and market cap
	baseScore := 50.0
	
	// Higher score for top market cap
	if marketCapRank <= 10 {
		baseScore += 25.0
	} else if marketCapRank <= 25 {
		baseScore += 15.0
	} else if marketCapRank <= 50 {
		baseScore += 10.0
	}
	
	// Add some symbol-specific variation
	seed := int64(0)
	for _, char := range symbol {
		seed += int64(char)
	}
	
	variation := float64(seed%20) - 10.0 // ±10 points
	
	return baseScore + variation
}