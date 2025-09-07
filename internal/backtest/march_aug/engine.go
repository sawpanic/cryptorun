package march_aug

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// BacktestEngineImpl implements the BacktestEngine interface
type BacktestEngineImpl struct {
	dataSource     DataSource
	factorCalc     FactorCalculator
	gateEvaluator  GateEvaluator
	regimeDetector *RegimeDetector
}

// NewBacktestEngine creates a new backtest engine with all components
func NewBacktestEngine() *BacktestEngineImpl {
	universe := []string{
		"BTC-USD", "ETH-USD", "SOL-USD", "ADA-USD", "DOT-USD", "AVAX-USD",
		"LINK-USD", "UNI-USD", "AAVE-USD", "MATIC-USD",
	}

	return &BacktestEngineImpl{
		dataSource:     NewMockDataSource(universe),
		factorCalc:     NewFactorCalculator(),
		gateEvaluator:  NewGateEvaluator(),
		regimeDetector: NewRegimeDetector(),
	}
}

// RunBacktest executes the March-August 2025 backtest
func (e *BacktestEngineImpl) RunBacktest(period BacktestPeriod, universe []string) (*BacktestSummary, error) {
	fmt.Printf("Starting backtest: %s (%v to %v)\n", period.Name,
		period.StartDate.Format("2006-01-02"), period.EndDate.Format("2006-01-02"))

	var allResults []BacktestResult
	regimeData := make(map[string][]BacktestResult) // Results by regime

	// Process each symbol in the universe
	for i, symbol := range universe {
		fmt.Printf("Processing %s (%d/%d)...\n", symbol, i+1, len(universe))

		// Get all data for the symbol
		marketData, err := e.dataSource.GetMarketData(symbol, period.StartDate, period.EndDate)
		if err != nil {
			return nil, fmt.Errorf("failed to get market data for %s: %w", symbol, err)
		}

		fundingData, err := e.dataSource.GetFundingData(symbol, period.StartDate, period.EndDate)
		if err != nil {
			return nil, fmt.Errorf("failed to get funding data for %s: %w", symbol, err)
		}

		oiData, err := e.dataSource.GetOpenInterestData(symbol, period.StartDate, period.EndDate)
		if err != nil {
			return nil, fmt.Errorf("failed to get OI data for %s: %w", symbol, err)
		}

		reservesData, err := e.dataSource.GetReservesData(symbol, period.StartDate, period.EndDate)
		if err != nil {
			return nil, fmt.Errorf("failed to get reserves data for %s: %w", symbol, err)
		}

		catalystData, err := e.dataSource.GetCatalystData(symbol, period.StartDate, period.EndDate)
		if err != nil {
			return nil, fmt.Errorf("failed to get catalyst data for %s: %w", symbol, err)
		}

		socialData, err := e.dataSource.GetSocialData(symbol, period.StartDate, period.EndDate)
		if err != nil {
			return nil, fmt.Errorf("failed to get social data for %s: %w", symbol, err)
		}

		// Process the symbol and get results
		symbolResults, err := e.processSymbol(symbol, marketData, fundingData, oiData,
			reservesData, catalystData, socialData)
		if err != nil {
			fmt.Printf("Warning: failed to process %s: %v\n", symbol, err)
			continue
		}

		// Collect results by regime
		for _, result := range symbolResults {
			regime := result.Signal.Factors.Regime
			regimeData[regime] = append(regimeData[regime], result)
		}

		allResults = append(allResults, symbolResults...)
	}

	if len(allResults) == 0 {
		return nil, fmt.Errorf("no backtest results generated")
	}

	fmt.Printf("Generated %d total signals\n", len(allResults))

	// Calculate overall statistics
	passedGates := 0
	winCount := 0
	totalReturn := 0.0
	var returns []float64

	for _, result := range allResults {
		if result.Signal.Gates.OverallPass {
			passedGates++
		}
		if result.Return48h > 0 {
			winCount++
		}
		totalReturn += result.Return48h
		returns = append(returns, result.Return48h)
	}

	// Generate decile and attribution analysis
	decileStats, err := e.GenerateDecileAnalysis(allResults)
	if err != nil {
		return nil, fmt.Errorf("failed to generate decile analysis: %w", err)
	}

	attribution, err := e.GenerateAttributionAnalysis(allResults)
	if err != nil {
		return nil, fmt.Errorf("failed to generate attribution analysis: %w", err)
	}

	// Calculate summary metrics
	avgReturn := totalReturn / float64(len(allResults))
	medianReturn := e.calculateMedian(returns)
	sharpe := e.calculateSharpe(returns)
	maxDrawdown := e.calculateMaxDrawdown(allResults)

	// Count false positives (high score, negative return)
	falsePositives := 0
	for _, result := range allResults {
		if result.Signal.Score >= 75.0 && result.Return48h < 0 {
			falsePositives++
		}
	}

	// Generate regime breakdown
	regimeBreakdown := make(map[string]BacktestSummary)
	for regime, results := range regimeData {
		if len(results) > 0 {
			breakdown := e.calculateRegimeBreakdown(regime, results)
			regimeBreakdown[regime] = breakdown
		}
	}

	summary := &BacktestSummary{
		Period:          period,
		TotalSignals:    len(allResults),
		PassedGates:     passedGates,
		GatePassRate:    float64(passedGates) / float64(len(allResults)),
		WinRate:         float64(winCount) / float64(len(allResults)),
		AvgReturn48h:    avgReturn * 100, // Convert to percentage
		MedianReturn:    medianReturn * 100,
		Sharpe:          sharpe,
		MaxDrawdown:     maxDrawdown,
		FalsePositives:  falsePositives,
		DecileStats:     decileStats,
		Attribution:     attribution,
		RegimeBreakdown: regimeBreakdown,
	}

	fmt.Printf("Backtest complete: %.1f%% win rate, %.2f%% avg return, %.1f%% gate pass rate\n",
		summary.WinRate*100, summary.AvgReturn48h, summary.GatePassRate*100)

	return summary, nil
}

// processSymbol processes a single symbol through the entire pipeline
func (e *BacktestEngineImpl) processSymbol(symbol string, marketData []MarketData,
	fundingData []FundingData, oiData []OpenInterestData, reservesData []ReservesData,
	catalystData []CatalystData, socialData []SocialData) ([]BacktestResult, error) {

	// Calculate momentum factors (protected)
	momentumFactors, err := e.factorCalc.CalculateMomentumFactors(marketData)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate momentum factors: %w", err)
	}

	// Calculate supply/demand factors
	supplyDemandFactors, err := e.factorCalc.CalculateSupplyDemandFactors(marketData,
		fundingData, oiData, reservesData)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate supply/demand factors: %w", err)
	}

	// Generate regime data
	var regimeData []RegimeData
	regimeMap := make(map[time.Time]RegimeData)

	for _, md := range marketData[24:] { // Skip first 24 hours for lookback
		recentData := e.getRecentMarketData(marketData, md.Timestamp, 24)
		regime := e.regimeDetector.DetectRegime(md.Timestamp, recentData)
		regimeData = append(regimeData, regime)
		regimeMap[md.Timestamp] = regime
	}

	// Calculate composite scores with regime awareness
	compositeScores, err := e.factorCalc.CalculateCompositeScores(momentumFactors,
		supplyDemandFactors, catalystData, socialData, regimeData)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate composite scores: %w", err)
	}

	// Generate signals by evaluating gates
	var results []BacktestResult
	for i, scores := range compositeScores {
		// Find corresponding market data
		var marketPoint MarketData
		for _, md := range marketData {
			if md.Timestamp.Equal(scores.Timestamp) {
				marketPoint = md
				break
			}
		}

		if marketPoint.Symbol == "" {
			continue // Skip if no matching market data
		}

		// Evaluate gates
		gates, err := e.gateEvaluator.EvaluateGates(scores, marketPoint)
		if err != nil {
			continue // Skip on evaluation error
		}

		// Create signal
		signal := BacktestSignal{
			Symbol:      scores.Symbol,
			Timestamp:   scores.Timestamp,
			Score:       scores.FinalScore,
			Gates:       gates,
			Factors:     scores,
			MarketData:  marketPoint,
			SignalType:  "entry",
			Confidence:  0.75 + (scores.FinalScore-50)/200, // Scale confidence with score
			Attribution: scores.Attribution,
		}

		// Calculate outcome (48h forward return)
		outcome := e.calculateSignalOutcome(signal, marketData, i+48) // 48h forward

		results = append(results, outcome)
	}

	return results, nil
}

// calculateSignalOutcome determines the 48h forward return for a signal
func (e *BacktestEngineImpl) calculateSignalOutcome(signal BacktestSignal,
	marketData []MarketData, forwardIndex int) BacktestResult {

	entryPrice := signal.MarketData.Close
	var exitPrice float64
	var return48h float64
	outcome := "timeout"

	// Find price 48 hours later
	if forwardIndex < len(marketData) {
		exitPrice = marketData[forwardIndex].Close
		return48h = (exitPrice - entryPrice) / entryPrice

		if return48h > 0 {
			outcome = "win"
		} else {
			outcome = "loss"
		}
	} else {
		// Use last available price if 48h data not available
		if len(marketData) > 0 {
			exitPrice = marketData[len(marketData)-1].Close
			return48h = (exitPrice - entryPrice) / entryPrice
			outcome = "partial"
		}
	}

	// Calculate additional metrics
	maxDrawdown := math.Max(0, -return48h) // Simplified drawdown calculation
	hitTarget := return48h > 0.05          // 5% target
	stoppedOut := return48h < -0.03        // 3% stop loss

	return BacktestResult{
		Signal:      signal,
		EntryPrice:  entryPrice,
		ExitPrice:   exitPrice,
		Return48h:   return48h,
		HoldingTime: 48 * time.Hour,
		Outcome:     outcome,
		PnLPct:      return48h * 100, // Convert to percentage
		MaxDrawdown: maxDrawdown,
		HitTarget:   hitTarget,
		StoppedOut:  stoppedOut,
	}
}

// GenerateDecileAnalysis creates score vs return decile breakdown
func (e *BacktestEngineImpl) GenerateDecileAnalysis(results []BacktestResult) ([]DecileAnalysis, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("no results provided for decile analysis")
	}

	// Sort results by score
	sortedResults := make([]BacktestResult, len(results))
	copy(sortedResults, results)
	sort.Slice(sortedResults, func(i, j int) bool {
		return sortedResults[i].Signal.Score < sortedResults[j].Signal.Score
	})

	decileSize := len(sortedResults) / 10
	var deciles []DecileAnalysis

	for decile := 1; decile <= 10; decile++ {
		start := (decile - 1) * decileSize
		end := decile * decileSize
		if decile == 10 {
			end = len(sortedResults) // Include remainder in last decile
		}

		if start >= len(sortedResults) {
			break
		}

		decileResults := sortedResults[start:end]
		analysis := e.calculateDecileStats(decile, decileResults)
		deciles = append(deciles, analysis)
	}

	// Calculate lift vs first decile
	if len(deciles) > 0 {
		firstDecileReturn := deciles[0].AvgReturn48h
		for i := range deciles {
			if firstDecileReturn != 0 {
				deciles[i].LiftVsDecile1 = (deciles[i].AvgReturn48h - firstDecileReturn) / math.Abs(firstDecileReturn)
			}
		}
	}

	return deciles, nil
}

// calculateDecileStats computes statistics for a single decile
func (e *BacktestEngineImpl) calculateDecileStats(decile int, results []BacktestResult) DecileAnalysis {
	if len(results) == 0 {
		return DecileAnalysis{Decile: decile}
	}

	// Calculate basic stats
	totalReturn := 0.0
	totalScore := 0.0
	winCount := 0
	var returns []float64
	var scores []float64
	maxDrawdown := 0.0

	for _, result := range results {
		totalReturn += result.Return48h
		totalScore += result.Signal.Score
		returns = append(returns, result.Return48h)
		scores = append(scores, result.Signal.Score)

		if result.Return48h > 0 {
			winCount++
		}

		if result.MaxDrawdown > maxDrawdown {
			maxDrawdown = result.MaxDrawdown
		}
	}

	avgReturn := totalReturn / float64(len(results))
	avgScore := totalScore / float64(len(results))
	winRate := float64(winCount) / float64(len(results))

	// Score range
	minScore := scores[0]
	maxScore := scores[len(scores)-1]
	scoreRange := fmt.Sprintf("%.1f-%.1f", minScore, maxScore)

	// Calculate median and std dev
	medianReturn := e.calculateMedian(returns)
	stdDev := e.calculateStdDev(returns)

	// Calculate Sharpe ratio
	sharpe := 0.0
	if stdDev > 0 {
		sharpe = avgReturn / stdDev
	}

	return DecileAnalysis{
		Decile:        decile,
		ScoreRange:    scoreRange,
		Count:         len(results),
		AvgScore:      avgScore,
		AvgReturn48h:  avgReturn * 100, // Convert to percentage
		WinRate:       winRate,
		MedianReturn:  medianReturn * 100,
		StdDev:        stdDev * 100,
		Sharpe:        sharpe,
		MaxDrawdown:   maxDrawdown * 100,
		LiftVsDecile1: 0.0, // Calculated later
	}
}

// GenerateAttributionAnalysis analyzes factor contributions to returns
func (e *BacktestEngineImpl) GenerateAttributionAnalysis(results []BacktestResult) ([]AttributionAnalysis, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("no results provided for attribution analysis")
	}

	factors := []string{"momentum", "supply_demand", "catalyst_heat", "social_signal"}
	var attribution []AttributionAnalysis

	for _, factor := range factors {
		analysis := e.calculateFactorAttribution(factor, results)
		attribution = append(attribution, analysis)
	}

	return attribution, nil
}

// calculateFactorAttribution analyzes a specific factor's contribution
func (e *BacktestEngineImpl) calculateFactorAttribution(factor string, results []BacktestResult) AttributionAnalysis {
	var contributions []float64
	var returns []float64
	positiveCount := 0
	signalCount := 0
	topDecileContribs := make([]float64, 0)

	// Sort results by score to identify top decile
	sortedResults := make([]BacktestResult, len(results))
	copy(sortedResults, results)
	sort.Slice(sortedResults, func(i, j int) bool {
		return sortedResults[i].Signal.Score > sortedResults[j].Signal.Score
	})

	topDecileSize := len(sortedResults) / 10
	topDecileThreshold := sortedResults[topDecileSize].Signal.Score

	for _, result := range results {
		if contrib, exists := result.Signal.Attribution[factor]; exists {
			contributions = append(contributions, contrib)
			returns = append(returns, result.Return48h)
			signalCount++

			if contrib > 0 {
				positiveCount++
			}

			// Check if in top decile
			if result.Signal.Score >= topDecileThreshold {
				topDecileContribs = append(topDecileContribs, contrib)
			}
		}
	}

	if len(contributions) == 0 {
		return AttributionAnalysis{Factor: factor}
	}

	// Calculate statistics
	avgContrib := e.calculateMean(contributions)
	contribStdDev := e.calculateStdDev(contributions)
	returnCorr := e.calculateCorrelation(contributions, returns)
	positiveRate := float64(positiveCount) / float64(signalCount)
	topDecileAvg := e.calculateMean(topDecileContribs)

	return AttributionAnalysis{
		Factor:        factor,
		AvgContrib:    avgContrib,
		ContribStdDev: contribStdDev,
		ReturnCorr:    returnCorr,
		SignalCount:   signalCount,
		PositiveRate:  positiveRate,
		TopDecileAvg:  topDecileAvg,
	}
}

// Helper calculation methods

func (e *BacktestEngineImpl) calculateRegimeBreakdown(regime string, results []BacktestResult) BacktestSummary {
	if len(results) == 0 {
		return BacktestSummary{}
	}

	winCount := 0
	totalReturn := 0.0
	var returns []float64

	for _, result := range results {
		if result.Return48h > 0 {
			winCount++
		}
		totalReturn += result.Return48h
		returns = append(returns, result.Return48h)
	}

	avgReturn := totalReturn / float64(len(results))
	winRate := float64(winCount) / float64(len(results))
	medianReturn := e.calculateMedian(returns)
	sharpe := e.calculateSharpe(returns)

	return BacktestSummary{
		TotalSignals: len(results),
		WinRate:      winRate,
		AvgReturn48h: avgReturn * 100,
		MedianReturn: medianReturn * 100,
		Sharpe:       sharpe,
	}
}

func (e *BacktestEngineImpl) calculateMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

func (e *BacktestEngineImpl) calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func (e *BacktestEngineImpl) calculateStdDev(values []float64) float64 {
	if len(values) <= 1 {
		return 0.0
	}

	mean := e.calculateMean(values)
	sumSq := 0.0

	for _, v := range values {
		sumSq += math.Pow(v-mean, 2)
	}

	variance := sumSq / float64(len(values)-1)
	return math.Sqrt(variance)
}

func (e *BacktestEngineImpl) calculateSharpe(returns []float64) float64 {
	if len(returns) == 0 {
		return 0.0
	}

	avgReturn := e.calculateMean(returns)
	stdDev := e.calculateStdDev(returns)

	if stdDev == 0 {
		return 0.0
	}

	// Assuming risk-free rate of 0 for simplicity
	return avgReturn / stdDev
}

func (e *BacktestEngineImpl) calculateMaxDrawdown(results []BacktestResult) float64 {
	if len(results) == 0 {
		return 0.0
	}

	maxDD := 0.0
	runningReturn := 0.0
	peak := 0.0

	for _, result := range results {
		runningReturn += result.Return48h
		if runningReturn > peak {
			peak = runningReturn
		}

		drawdown := peak - runningReturn
		if drawdown > maxDD {
			maxDD = drawdown
		}
	}

	return maxDD * 100 // Convert to percentage
}

func (e *BacktestEngineImpl) calculateCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0.0
	}

	meanX := e.calculateMean(x)
	meanY := e.calculateMean(y)

	numerator := 0.0
	sumXSq := 0.0
	sumYSq := 0.0

	for i := 0; i < len(x); i++ {
		xDiff := x[i] - meanX
		yDiff := y[i] - meanY

		numerator += xDiff * yDiff
		sumXSq += xDiff * xDiff
		sumYSq += yDiff * yDiff
	}

	denominator := math.Sqrt(sumXSq * sumYSq)
	if denominator == 0 {
		return 0.0
	}

	return numerator / denominator
}

func (e *BacktestEngineImpl) getRecentMarketData(allData []MarketData, timestamp time.Time, hours int) []MarketData {
	var recent []MarketData
	cutoff := timestamp.Add(-time.Duration(hours) * time.Hour)

	for _, data := range allData {
		if data.Timestamp.After(cutoff) && data.Timestamp.Before(timestamp.Add(time.Hour)) {
			recent = append(recent, data)
		}
	}

	return recent
}
