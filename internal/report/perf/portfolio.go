package perf

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/sawpanic/cryptorun/internal/persistence"
)

// PortfolioCalculator computes portfolio-level metrics and analysis
type PortfolioCalculator struct {
	config     PerfCalculatorConfig
	tradesRepo persistence.TradesRepo
}

// NewPortfolioCalculator creates a new portfolio calculator
func NewPortfolioCalculator(config PerfCalculatorConfig, tradesRepo persistence.TradesRepo) *PortfolioCalculator {
	return &PortfolioCalculator{
		config:     config,
		tradesRepo: tradesRepo,
	}
}

// CalculatePortfolio computes comprehensive portfolio metrics
func (pc *PortfolioCalculator) CalculatePortfolio(ctx context.Context, asOfTime time.Time) (*PortfolioMetrics, error) {
	// Fetch current positions from trades repository
	positions, err := pc.getCurrentPositions(ctx, asOfTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get current positions: %w", err)
	}

	portfolio := &PortfolioMetrics{
		Positions:         positions,
		SectorAllocation:  make(map[string]float64),
		CorrelationMatrix: make(map[string]map[string]float64),
		AsOfTime:          asOfTime,
	}

	// Calculate portfolio-level metrics
	pc.calculatePortfolioValues(portfolio)
	pc.calculateRiskMetrics(portfolio)
	pc.calculateExposureMetrics(portfolio)
	pc.calculateSectorAllocation(portfolio)
	pc.calculatePerformanceAttribution(portfolio)
	pc.calculateCorrelationMatrix(ctx, portfolio)

	return portfolio, nil
}

// getCurrentPositions reconstructs current positions from trade history
func (pc *PortfolioCalculator) getCurrentPositions(ctx context.Context, asOfTime time.Time) ([]PositionSummary, error) {
	// Get all trades up to the as-of time
	timeRange := persistence.TimeRange{
		From: time.Now().AddDate(-1, 0, 0), // Look back 1 year
		To:   asOfTime,
	}

	// Get trades for all symbols (would need to enhance repository interface)
	// For now, using a mock approach with common crypto symbols
	commonSymbols := []string{"BTC-USD", "ETH-USD", "USDT", "USDC", "ADA-USD", "DOT-USD", "LINK-USD", "UNI-USD"}
	
	allPositions := make([]PositionSummary, 0)
	
	for _, symbol := range commonSymbols {
		trades, err := pc.tradesRepo.ListBySymbol(ctx, symbol, timeRange, 10000) // Large limit
		if err != nil {
			continue // Skip symbols with errors
		}

		if len(trades) == 0 {
			continue
		}

		// Calculate net position
		position := pc.calculateNetPosition(symbol, trades)
		if position.Quantity != 0 { // Only include non-zero positions
			allPositions = append(allPositions, position)
		}
	}

	return allPositions, nil
}

// calculateNetPosition computes net position from trade history
func (pc *PortfolioCalculator) calculateNetPosition(symbol string, trades []persistence.Trade) PositionSummary {
	position := PositionSummary{
		Symbol:     symbol,
		Sector:     pc.getSectorForSymbol(symbol),
		LastUpdate: time.Now(),
	}

	if len(trades) == 0 {
		return position
	}

	// Calculate net quantity and weighted average cost
	totalQuantity := 0.0
	totalCost := 0.0
	var firstTradeTime time.Time

	for i, trade := range trades {
		if i == 0 {
			firstTradeTime = trade.Timestamp
		}

		quantity := trade.Qty
		if trade.Side == "sell" {
			quantity = -quantity
		}

		totalQuantity += quantity
		totalCost += math.Abs(quantity) * trade.Price
		
		if trade.Timestamp.After(position.LastUpdate) {
			position.LastUpdate = trade.Timestamp
		}
	}

	position.Quantity = totalQuantity
	
	if totalQuantity != 0 {
		position.CostBasis = totalCost / math.Abs(totalQuantity)
		
		// Estimate current market value (would use real market data in production)
		currentPrice := pc.estimateCurrentPrice(symbol, trades)
		position.MarketValue = totalQuantity * currentPrice
		position.UnrealizedPnL = position.MarketValue - (position.CostBasis * totalQuantity)
		
		// Calculate days held
		position.DaysHeld = int(time.Since(firstTradeTime).Hours() / 24)
	}

	return position
}

// estimateCurrentPrice estimates current price from recent trades
func (pc *PortfolioCalculator) estimateCurrentPrice(symbol string, trades []persistence.Trade) float64 {
	if len(trades) == 0 {
		return 0.0
	}

	// Sort trades by timestamp (most recent first)
	sort.Slice(trades, func(i, j int) bool {
		return trades[i].Timestamp.After(trades[j].Timestamp)
	})

	// Use most recent trade price
	return trades[0].Price
}

// getSectorForSymbol maps symbol to sector (simplified categorization)
func (pc *PortfolioCalculator) getSectorForSymbol(symbol string) string {
	sectorMap := map[string]string{
		"BTC-USD":  "Large Cap Crypto",
		"ETH-USD":  "Large Cap Crypto", 
		"USDT":     "Stablecoins",
		"USDC":     "Stablecoins",
		"BUSD":     "Stablecoins",
		"DAI":      "Stablecoins",
		"ADA-USD":  "Smart Contract Platforms",
		"DOT-USD":  "Interoperability",
		"LINK-USD": "Oracle Networks",
		"UNI-USD":  "DeFi",
		"AAVE-USD": "DeFi",
		"COMP-USD": "DeFi",
		"MKR-USD":  "DeFi",
		"CRV-USD":  "DeFi",
	}

	if sector, ok := sectorMap[symbol]; ok {
		return sector
	}
	
	return "Other"
}

// calculatePortfolioValues computes total portfolio value and position weights
func (pc *PortfolioCalculator) calculatePortfolioValues(portfolio *PortfolioMetrics) {
	totalValue := 0.0
	
	for _, position := range portfolio.Positions {
		totalValue += position.MarketValue
	}
	
	portfolio.TotalValue = totalValue
	
	// Calculate position weights
	for i := range portfolio.Positions {
		if totalValue > 0 {
			portfolio.Positions[i].Weight = portfolio.Positions[i].MarketValue / totalValue
		}
	}
}

// calculateRiskMetrics computes portfolio risk measures
func (pc *PortfolioCalculator) calculateRiskMetrics(portfolio *PortfolioMetrics) {
	if len(portfolio.Positions) == 0 {
		return
	}

	// Calculate concentration risk (sum of squared weights)
	concentrationRisk := 0.0
	for _, position := range portfolio.Positions {
		concentrationRisk += position.Weight * position.Weight
	}
	portfolio.ConcentrationRisk = concentrationRisk

	// Calculate simplified VaR (would use historical returns in production)
	// Using a simplified approach based on position volatilities
	totalVaR := 0.0
	for _, position := range portfolio.Positions {
		// Estimate position volatility (simplified)
		positionVol := pc.estimatePositionVolatility(position.Symbol)
		positionVaR := position.MarketValue * positionVol * 1.96 // 95% confidence
		totalVaR += positionVaR * positionVaR // Assuming independence (simplified)
	}
	
	portfolio.PortfolioVaR = math.Sqrt(totalVaR)
}

// estimatePositionVolatility estimates position volatility (simplified)
func (pc *PortfolioCalculator) estimatePositionVolatility(symbol string) float64 {
	// Simplified volatility estimates for common crypto assets
	volatilityMap := map[string]float64{
		"BTC-USD":  0.40, // 40% annual volatility
		"ETH-USD":  0.50, // 50% annual volatility
		"USDT":     0.01, // 1% annual volatility (stable)
		"USDC":     0.01, // 1% annual volatility (stable)
		"ADA-USD":  0.60, // 60% annual volatility
		"DOT-USD":  0.65, // 65% annual volatility
		"LINK-USD": 0.55, // 55% annual volatility
		"UNI-USD":  0.70, // 70% annual volatility
	}

	if vol, ok := volatilityMap[symbol]; ok {
		return vol
	}
	
	return 0.80 // Default high volatility for unknown assets
}

// calculateExposureMetrics computes long/short exposure metrics
func (pc *PortfolioCalculator) calculateExposureMetrics(portfolio *PortfolioMetrics) {
	longExposure := 0.0
	shortExposure := 0.0

	for _, position := range portfolio.Positions {
		if position.Quantity > 0 {
			longExposure += position.MarketValue
		} else if position.Quantity < 0 {
			shortExposure += math.Abs(position.MarketValue)
		}
	}

	portfolio.LongExposure = longExposure
	portfolio.ShortExposure = shortExposure
	portfolio.NetExposure = longExposure - shortExposure
	portfolio.GrossExposure = longExposure + shortExposure
}

// calculateSectorAllocation computes allocation by sector
func (pc *PortfolioCalculator) calculateSectorAllocation(portfolio *PortfolioMetrics) {
	sectorValues := make(map[string]float64)

	for _, position := range portfolio.Positions {
		sectorValues[position.Sector] += position.MarketValue
	}

	// Convert to percentages
	if portfolio.TotalValue > 0 {
		for sector, value := range sectorValues {
			portfolio.SectorAllocation[sector] = value / portfolio.TotalValue
		}
	}
}

// calculatePerformanceAttribution computes performance attribution by position
func (pc *PortfolioCalculator) calculatePerformanceAttribution(portfolio *PortfolioMetrics) {
	attribution := make([]AttributionEntry, 0, len(portfolio.Positions))

	// Portfolio return calculation available but currently unused
	// Could be used for portfolio-level attribution in the future

	for _, position := range portfolio.Positions {
		// Calculate position contribution to portfolio return
		positionReturn := 0.0
		if position.CostBasis > 0 && position.Quantity != 0 {
			positionReturn = position.UnrealizedPnL / (position.CostBasis * math.Abs(position.Quantity))
		}

		contribution := position.Weight * positionReturn
		
		// Simplified attribution (would be more complex in practice)
		entry := AttributionEntry{
			Symbol:            position.Symbol,
			Contribution:      contribution,
			AllocationEffect:  contribution * 0.3,  // 30% allocation effect
			SelectionEffect:   contribution * 0.7,  // 70% selection effect
			InteractionEffect: 0.0,                 // Simplified
		}

		attribution = append(attribution, entry)
	}

	portfolio.Attribution = attribution
}

// calculateCorrelationMatrix computes pairwise correlations (simplified)
func (pc *PortfolioCalculator) calculateCorrelationMatrix(ctx context.Context, portfolio *PortfolioMetrics) {
	// Initialize correlation matrix
	for _, pos1 := range portfolio.Positions {
		portfolio.CorrelationMatrix[pos1.Symbol] = make(map[string]float64)
		
		for _, pos2 := range portfolio.Positions {
			if pos1.Symbol == pos2.Symbol {
				portfolio.CorrelationMatrix[pos1.Symbol][pos2.Symbol] = 1.0
			} else {
				// Simplified correlation estimation
				correlation := pc.estimateCorrelation(pos1.Symbol, pos2.Symbol)
				portfolio.CorrelationMatrix[pos1.Symbol][pos2.Symbol] = correlation
			}
		}
	}
}

// estimateCorrelation provides simplified correlation estimates
func (pc *PortfolioCalculator) estimateCorrelation(symbol1, symbol2 string) float64 {
	// Simplified correlation matrix for common crypto pairs
	correlations := map[string]map[string]float64{
		"BTC-USD": {
			"ETH-USD":  0.75,
			"ADA-USD":  0.65,
			"DOT-USD":  0.60,
			"LINK-USD": 0.55,
			"UNI-USD":  0.50,
			"USDT":     0.05,
			"USDC":     0.05,
		},
		"ETH-USD": {
			"BTC-USD":  0.75,
			"ADA-USD":  0.70,
			"DOT-USD":  0.68,
			"LINK-USD": 0.60,
			"UNI-USD":  0.65,
			"USDT":     0.05,
			"USDC":     0.05,
		},
	}

	// Check both directions
	if corrs1, ok := correlations[symbol1]; ok {
		if corr, ok := corrs1[symbol2]; ok {
			return corr
		}
	}
	
	if corrs2, ok := correlations[symbol2]; ok {
		if corr, ok := corrs2[symbol1]; ok {
			return corr
		}
	}

	// Default correlations based on asset types
	sector1 := pc.getSectorForSymbol(symbol1)
	sector2 := pc.getSectorForSymbol(symbol2)

	if sector1 == sector2 {
		if sector1 == "Stablecoins" {
			return 0.95 // High correlation between stablecoins
		}
		return 0.70 // High correlation within sectors
	}

	if (sector1 == "Stablecoins" && sector2 != "Stablecoins") || 
		(sector1 != "Stablecoins" && sector2 == "Stablecoins") {
		return 0.05 // Low correlation between stablecoins and crypto
	}

	return 0.45 // Moderate correlation between different crypto sectors
}

// CheckPortfolioAlerts checks portfolio metrics against configured thresholds
func (pc *PortfolioCalculator) CheckPortfolioAlerts(portfolio *PortfolioMetrics) []string {
	alerts := make([]string, 0)

	// Check correlation limits
	for symbol1, correlations := range portfolio.CorrelationMatrix {
		for symbol2, correlation := range correlations {
			if symbol1 != symbol2 && correlation > pc.config.MaxCorrelation {
				alert := fmt.Sprintf("High correlation detected: %s vs %s (%.3f > %.3f)", 
					symbol1, symbol2, correlation, pc.config.MaxCorrelation)
				alerts = append(alerts, alert)
			}
		}
	}

	// Check concentration risk
	if portfolio.ConcentrationRisk > 0.5 { // Arbitrary threshold
		alert := fmt.Sprintf("High concentration risk: %.3f (diversify positions)", portfolio.ConcentrationRisk)
		alerts = append(alerts, alert)
	}

	// Check sector allocation limits
	for sector, allocation := range portfolio.SectorAllocation {
		if allocation > 0.40 { // 40% sector limit
			alert := fmt.Sprintf("High sector concentration: %s %.1f%% (limit: 40%%)", sector, allocation*100)
			alerts = append(alerts, alert)
		}
	}

	return alerts
}