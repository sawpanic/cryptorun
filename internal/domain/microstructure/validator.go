package microstructure

import (
	"fmt"
	"math"
	"time"
)

// MicrostructureData represents L1/L2 order book data from exchanges
type MicrostructureData struct {
	Symbol          string                 `json:"symbol"`
	Exchange        string                 `json:"exchange"`
	Timestamp       time.Time              `json:"timestamp"`
	BestBid         float64                `json:"best_bid"`
	BestAsk         float64                `json:"best_ask"`
	BidSize         float64                `json:"bid_size"`
	AskSize         float64                `json:"ask_size"`
	OrderBook       OrderBook              `json:"order_book"`
	RecentTrades    []Trade                `json:"recent_trades"`
	Volume24h       float64                `json:"volume_24h"`
	MarketCap       float64                `json:"market_cap"`
	CirculatingSupply float64              `json:"circulating_supply"`
	Metadata        MicrostructureMetadata `json:"metadata"`
}

// OrderBook represents order book depth data
type OrderBook struct {
	Bids      []OrderLevel `json:"bids"`       // Sorted highest to lowest
	Asks      []OrderLevel `json:"asks"`       // Sorted lowest to highest
	Timestamp time.Time    `json:"timestamp"`
	Sequence  int64        `json:"sequence"`   // For order book integrity
}

// OrderLevel represents a single price level in the order book
type OrderLevel struct {
	Price      float64 `json:"price"`
	Size       float64 `json:"size"`        // In base currency
	SizeUSD    float64 `json:"size_usd"`    // In USD equivalent
	OrderCount int     `json:"order_count"` // Number of orders at this level
}

// Trade represents a recent trade
type Trade struct {
	Price     float64   `json:"price"`
	Size      float64   `json:"size"`
	Side      string    `json:"side"`      // "buy" or "sell"
	Timestamp time.Time `json:"timestamp"`
	TradeID   string    `json:"trade_id"`
}

// MicrostructureMetadata provides additional context
type MicrostructureMetadata struct {
	DataSource       string    `json:"data_source"`       // "native_api", "websocket", etc.
	LastUpdate       time.Time `json:"last_update"`
	Staleness        float64   `json:"staleness_seconds"` // How old is the data
	IsExchangeNative bool      `json:"is_exchange_native"` // True if from exchange APIs directly
	APIEndpoint      string    `json:"api_endpoint"`
	RateLimit        RateLimit `json:"rate_limit"`
}

// RateLimit tracks API rate limiting
type RateLimit struct {
	RequestsUsed      int `json:"requests_used"`
	RequestsRemaining int `json:"requests_remaining"`
	ResetTimestamp    int64 `json:"reset_timestamp"`
}

// ValidationResult represents microstructure validation outcome
type ValidationResult struct {
	Symbol           string              `json:"symbol"`
	Exchange         string              `json:"exchange"`
	Timestamp        time.Time           `json:"timestamp"`
	Passed           bool                `json:"passed"`
	FailureReasons   []string            `json:"failure_reasons"`
	Warnings         []string            `json:"warnings"`
	Metrics          MicrostructureMetrics `json:"metrics"`
	Recommendation   string              `json:"recommendation"`
	ConfidenceScore  float64             `json:"confidence_score"` // 0-100
}

// MicrostructureMetrics contains calculated microstructure values
type MicrostructureMetrics struct {
	SpreadBps        float64 `json:"spread_bps"`         // Spread in basis points
	SpreadPercent    float64 `json:"spread_percent"`     // Spread as percentage
	DepthBids        float64 `json:"depth_bids_usd"`     // Bid depth within ±2% (USD)
	DepthAsks        float64 `json:"depth_asks_usd"`     // Ask depth within ±2% (USD)
	TotalDepth       float64 `json:"total_depth_usd"`    // Combined depth
	VADR             float64 `json:"vadr"`               // Volume-Adjusted Daily Range
	ADV              float64 `json:"adv"`                // Average Daily Volume (USD)
	VolumeRatio      float64 `json:"volume_ratio"`       // Current vs average volume
	MarketImpact     float64 `json:"market_impact_bps"`  // Est. impact for typical trade
	OrderBookBalance float64 `json:"order_book_balance"` // Bid/ask balance (-1 to +1)
	DataQuality      float64 `json:"data_quality_score"` // 0-100 data quality score
}

// RequirementThresholds defines the microstructure validation requirements
type RequirementThresholds struct {
	MaxSpreadBps            float64 `json:"max_spread_bps"`             // 50 bps default
	MinDepthUSD             float64 `json:"min_depth_usd"`              // $100k default
	MinVADR                 float64 `json:"min_vadr"`                   // 1.8x default
	MaxStalenessSeconds     int     `json:"max_staleness_seconds"`      // 60s default
	RequireExchangeNative   bool    `json:"require_exchange_native"`    // true for CryptoRun
	MinDataQualityScore     float64 `json:"min_data_quality_score"`     // 85.0 default
	MaxMarketImpactBps      float64 `json:"max_market_impact_bps"`      // 20 bps default
	BannedAggregators       []string `json:"banned_aggregators"`        // DEXScreener, CoinGecko, etc.
	RequiredExchanges       []string `json:"required_exchanges"`        // Binance, OKX, Coinbase, Kraken
}

// DefaultRequirementThresholds returns CryptoRun's standard microstructure requirements
func DefaultRequirementThresholds() RequirementThresholds {
	return RequirementThresholds{
		MaxSpreadBps:          50.0,   // 0.5% maximum spread
		MinDepthUSD:           100000, // $100k minimum depth within ±2%
		MinVADR:               1.8,    // 1.8x minimum VADR
		MaxStalenessSeconds:   60,     // 1 minute maximum staleness
		RequireExchangeNative: true,   // Must be exchange-native
		MinDataQualityScore:   85.0,   // 85% minimum data quality
		MaxMarketImpactBps:    20.0,   // 20 bps maximum market impact
		BannedAggregators: []string{
			"dexscreener.com",
			"coingecko.com", 
			"coinmarketcap.com",
			"cryptocompare.com",
			"nomics.com",
		},
		RequiredExchanges: []string{
			"binance",
			"okx", 
			"coinbase",
			"kraken",
		},
	}
}

// MicrostructureValidator validates market microstructure data
type MicrostructureValidator struct {
	thresholds RequirementThresholds
}

// NewMicrostructureValidator creates a new microstructure validator
func NewMicrostructureValidator(thresholds RequirementThresholds) *MicrostructureValidator {
	return &MicrostructureValidator{
		thresholds: thresholds,
	}
}

// ValidateMicrostructure performs comprehensive microstructure validation
func (mv *MicrostructureValidator) ValidateMicrostructure(data MicrostructureData) ValidationResult {
	result := ValidationResult{
		Symbol:          data.Symbol,
		Exchange:        data.Exchange,
		Timestamp:       time.Now(),
		Passed:          true,
		FailureReasons:  []string{},
		Warnings:        []string{},
		ConfidenceScore: 100.0,
	}

	// Calculate microstructure metrics
	metrics := mv.calculateMicrostructureMetrics(data)
	result.Metrics = metrics

	// 1. Validate data source (exchange-native requirement)
	if mv.thresholds.RequireExchangeNative {
		if !data.Metadata.IsExchangeNative {
			result.Passed = false
			result.FailureReasons = append(result.FailureReasons, "Data source is not exchange-native")
			result.ConfidenceScore -= 30
		}

		// Check for banned aggregators
		for _, bannedSource := range mv.thresholds.BannedAggregators {
			if containsString(data.Metadata.APIEndpoint, bannedSource) || 
			   containsString(data.Metadata.DataSource, bannedSource) {
				result.Passed = false
				result.FailureReasons = append(result.FailureReasons, 
					fmt.Sprintf("Banned aggregator detected: %s", bannedSource))
				result.ConfidenceScore -= 50
			}
		}

		// Validate exchange is in approved list
		if len(mv.thresholds.RequiredExchanges) > 0 {
			exchangeApproved := false
			for _, approvedExchange := range mv.thresholds.RequiredExchanges {
				if data.Exchange == approvedExchange {
					exchangeApproved = true
					break
				}
			}
			if !exchangeApproved {
				result.Passed = false
				result.FailureReasons = append(result.FailureReasons, 
					fmt.Sprintf("Exchange %s not in approved list", data.Exchange))
				result.ConfidenceScore -= 25
			}
		}
	}

	// 2. Validate data freshness
	staleness := data.Metadata.Staleness
	if staleness > float64(mv.thresholds.MaxStalenessSeconds) {
		result.Passed = false
		result.FailureReasons = append(result.FailureReasons, 
			fmt.Sprintf("Data too stale: %.1fs > %ds", staleness, mv.thresholds.MaxStalenessSeconds))
		result.ConfidenceScore -= 20
	} else if staleness > float64(mv.thresholds.MaxStalenessSeconds)/2 {
		result.Warnings = append(result.Warnings, 
			fmt.Sprintf("Data moderately stale: %.1fs", staleness))
		result.ConfidenceScore -= 5
	}

	// 3. Validate spread requirements
	if metrics.SpreadBps > mv.thresholds.MaxSpreadBps {
		result.Passed = false
		result.FailureReasons = append(result.FailureReasons, 
			fmt.Sprintf("Spread too wide: %.1f bps > %.1f bps", 
				metrics.SpreadBps, mv.thresholds.MaxSpreadBps))
		result.ConfidenceScore -= 15
	} else if metrics.SpreadBps > mv.thresholds.MaxSpreadBps*0.8 {
		result.Warnings = append(result.Warnings, 
			fmt.Sprintf("Spread approaching limit: %.1f bps", metrics.SpreadBps))
		result.ConfidenceScore -= 5
	}

	// 4. Validate depth requirements
	if metrics.TotalDepth < mv.thresholds.MinDepthUSD {
		result.Passed = false
		result.FailureReasons = append(result.FailureReasons, 
			fmt.Sprintf("Insufficient depth: $%.0f < $%.0f", 
				metrics.TotalDepth, mv.thresholds.MinDepthUSD))
		result.ConfidenceScore -= 20
	} else if metrics.TotalDepth < mv.thresholds.MinDepthUSD*1.5 {
		result.Warnings = append(result.Warnings, 
			fmt.Sprintf("Low depth: $%.0f", metrics.TotalDepth))
		result.ConfidenceScore -= 5
	}

	// 5. Validate VADR requirements
	if metrics.VADR < mv.thresholds.MinVADR {
		result.Passed = false
		result.FailureReasons = append(result.FailureReasons, 
			fmt.Sprintf("VADR too low: %.2fx < %.2fx", 
				metrics.VADR, mv.thresholds.MinVADR))
		result.ConfidenceScore -= 15
	}

	// 6. Validate market impact
	if metrics.MarketImpact > mv.thresholds.MaxMarketImpactBps {
		result.Passed = false
		result.FailureReasons = append(result.FailureReasons, 
			fmt.Sprintf("Market impact too high: %.1f bps > %.1f bps", 
				metrics.MarketImpact, mv.thresholds.MaxMarketImpactBps))
		result.ConfidenceScore -= 10
	}

	// 7. Validate data quality
	if metrics.DataQuality < mv.thresholds.MinDataQualityScore {
		result.Passed = false
		result.FailureReasons = append(result.FailureReasons, 
			fmt.Sprintf("Data quality too low: %.1f < %.1f", 
				metrics.DataQuality, mv.thresholds.MinDataQualityScore))
		result.ConfidenceScore -= 15
	}

	// Generate recommendation
	if result.Passed {
		if len(result.Warnings) > 0 {
			result.Recommendation = "APPROVED_WITH_CAUTION: Microstructure acceptable but monitor warnings"
		} else {
			result.Recommendation = "APPROVED: All microstructure requirements met"
		}
	} else {
		result.Recommendation = "REJECTED: Microstructure requirements not met"
	}

	// Ensure confidence score doesn't go below 0
	if result.ConfidenceScore < 0 {
		result.ConfidenceScore = 0
	}

	return result
}

// calculateMicrostructureMetrics calculates all microstructure metrics
func (mv *MicrostructureValidator) calculateMicrostructureMetrics(data MicrostructureData) MicrostructureMetrics {
	metrics := MicrostructureMetrics{}

	// 1. Calculate spread
	if data.BestBid > 0 && data.BestAsk > 0 && data.BestAsk > data.BestBid {
		midPrice := (data.BestBid + data.BestAsk) / 2.0
		spread := data.BestAsk - data.BestBid
		
		metrics.SpreadPercent = (spread / midPrice) * 100.0
		metrics.SpreadBps = metrics.SpreadPercent * 100.0 // Convert to basis points
	}

	// 2. Calculate depth within ±2%
	if len(data.OrderBook.Bids) > 0 && len(data.OrderBook.Asks) > 0 {
		midPrice := (data.BestBid + data.BestAsk) / 2.0
		
		// Calculate bid depth within 2% of mid price
		minBidPrice := midPrice * 0.98 // 2% below mid
		for _, bid := range data.OrderBook.Bids {
			if bid.Price >= minBidPrice {
				metrics.DepthBids += bid.SizeUSD
			}
		}
		
		// Calculate ask depth within 2% of mid price  
		maxAskPrice := midPrice * 1.02 // 2% above mid
		for _, ask := range data.OrderBook.Asks {
			if ask.Price <= maxAskPrice {
				metrics.DepthAsks += ask.SizeUSD
			}
		}
		
		metrics.TotalDepth = metrics.DepthBids + metrics.DepthAsks
	}

	// 3. Calculate VADR (Volume-Adjusted Daily Range)
	if data.Volume24h > 0 && data.MarketCap > 0 {
		// Calculate daily range from recent trades (simplified)
		if len(data.RecentTrades) > 0 {
			high := data.RecentTrades[0].Price
			low := data.RecentTrades[0].Price
			
			for _, trade := range data.RecentTrades {
				if trade.Price > high {
					high = trade.Price
				}
				if trade.Price < low {
					low = trade.Price
				}
			}
			
			dailyRange := (high - low) / low
			avgPrice := (high + low) / 2.0
			
			// Volume as percentage of market cap
			volumeRatio := data.Volume24h / (data.MarketCap * avgPrice / data.CirculatingSupply)
			
			if volumeRatio > 0 {
				metrics.VADR = dailyRange / volumeRatio
			}
		}
	}

	// 4. Calculate ADV (simplified)
	metrics.ADV = data.Volume24h // This would be a rolling average in production

	// 5. Calculate volume ratio (current vs average)
	if metrics.ADV > 0 {
		metrics.VolumeRatio = data.Volume24h / metrics.ADV
	} else {
		metrics.VolumeRatio = 1.0
	}

	// 6. Estimate market impact (simplified)
	if metrics.TotalDepth > 0 {
		typicalTradeSize := 10000.0 // $10k typical trade
		impactRatio := typicalTradeSize / metrics.TotalDepth
		metrics.MarketImpact = impactRatio * metrics.SpreadBps
	}

	// 7. Calculate order book balance
	if metrics.DepthBids > 0 && metrics.DepthAsks > 0 {
		total := metrics.DepthBids + metrics.DepthAsks
		bidRatio := metrics.DepthBids / total
		askRatio := metrics.DepthAsks / total
		metrics.OrderBookBalance = bidRatio - askRatio // -1 to +1 scale
	}

	// 8. Calculate data quality score
	metrics.DataQuality = mv.calculateDataQuality(data)

	return metrics
}

// calculateDataQuality assesses the quality of the microstructure data
func (mv *MicrostructureValidator) calculateDataQuality(data MicrostructureData) float64 {
	score := 100.0
	
	// Check basic data completeness
	if data.BestBid <= 0 || data.BestAsk <= 0 {
		score -= 30
	}
	
	if len(data.OrderBook.Bids) == 0 || len(data.OrderBook.Asks) == 0 {
		score -= 25
	}
	
	if len(data.RecentTrades) == 0 {
		score -= 15
	}
	
	// Check data consistency
	if data.BestAsk <= data.BestBid {
		score -= 20 // Crossed quotes
	}
	
	if len(data.OrderBook.Bids) > 0 && data.OrderBook.Bids[0].Price != data.BestBid {
		score -= 10 // Inconsistent best bid
	}
	
	if len(data.OrderBook.Asks) > 0 && data.OrderBook.Asks[0].Price != data.BestAsk {
		score -= 10 // Inconsistent best ask
	}
	
	// Check staleness
	staleness := data.Metadata.Staleness
	if staleness > 30 {
		score -= 15
	} else if staleness > 10 {
		score -= 5
	}
	
	// Check order book depth
	if len(data.OrderBook.Bids) < 10 || len(data.OrderBook.Asks) < 10 {
		score -= 5 // Shallow order book
	}
	
	return math.Max(0, score)
}

// Helper functions
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		   (len(s) > len(substr) && 
		    (containsString(s[1:], substr) || containsString(s[:len(s)-1], substr))))
}

// BatchValidate validates multiple symbols efficiently
func (mv *MicrostructureValidator) BatchValidate(dataList []MicrostructureData) []ValidationResult {
	results := make([]ValidationResult, len(dataList))
	
	for i, data := range dataList {
		results[i] = mv.ValidateMicrostructure(data)
	}
	
	return results
}

// GetValidationSummary provides a summary of batch validation results
type ValidationSummary struct {
	TotalSymbols    int     `json:"total_symbols"`
	PassedCount     int     `json:"passed_count"`
	FailedCount     int     `json:"failed_count"`
	WarningCount    int     `json:"warning_count"`
	PassRate        float64 `json:"pass_rate"`
	AverageConfidence float64 `json:"average_confidence"`
	CommonFailures  map[string]int `json:"common_failures"`
	Recommendations []string `json:"recommendations"`
}

// GetValidationSummary analyzes batch validation results
func GetValidationSummary(results []ValidationResult) ValidationSummary {
	summary := ValidationSummary{
		TotalSymbols:   len(results),
		CommonFailures: make(map[string]int),
	}
	
	if len(results) == 0 {
		return summary
	}
	
	totalConfidence := 0.0
	
	for _, result := range results {
		totalConfidence += result.ConfidenceScore
		
		if result.Passed {
			summary.PassedCount++
		} else {
			summary.FailedCount++
			
			// Count common failure reasons
			for _, reason := range result.FailureReasons {
				summary.CommonFailures[reason]++
			}
		}
		
		if len(result.Warnings) > 0 {
			summary.WarningCount++
		}
	}
	
	summary.PassRate = float64(summary.PassedCount) / float64(summary.TotalSymbols) * 100.0
	summary.AverageConfidence = totalConfidence / float64(summary.TotalSymbols)
	
	// Generate recommendations based on common failures
	if summary.FailedCount > 0 {
		for reason, count := range summary.CommonFailures {
			if float64(count)/float64(summary.TotalSymbols) > 0.5 { // More than 50% failure rate
				summary.Recommendations = append(summary.Recommendations, 
					fmt.Sprintf("Address common issue: %s (affects %d symbols)", reason, count))
			}
		}
	}
	
	if summary.PassRate < 50 {
		summary.Recommendations = append(summary.Recommendations, 
			"Consider relaxing microstructure requirements - low pass rate")
	}
	
	if summary.AverageConfidence < 70 {
		summary.Recommendations = append(summary.Recommendations, 
			"Review data quality - low average confidence")
	}
	
	return summary
}