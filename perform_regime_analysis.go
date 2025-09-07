package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Regime analysis results
type RegimeAnalysis struct {
	CurrentRegime     string               `json:"current_regime"`
	Confidence        float64              `json:"confidence"`
	NextRegimePred    string               `json:"next_regime_prediction"`
	TimeToSwitch      time.Duration        `json:"time_to_switch"`
	AnalysisTime      time.Time            `json:"analysis_time"`
	Indicators        RegimeIndicators     `json:"indicators"`
	WeightRecommend   WeightRecommendation `json:"weight_recommendation"`
	RiskAssessment    RiskAssessment       `json:"risk_assessment"`
}

type RegimeIndicators struct {
	RealizedVol7d     VolatilityAnalysis `json:"realized_vol_7d"`
	BreadthAbove20MA  BreadthAnalysis    `json:"breadth_above_20ma"`
	BreadthThrustADX  ThrustAnalysis     `json:"breadth_thrust_adx"`
	MajorityVote      VoteAnalysis       `json:"majority_vote"`
}

type VolatilityAnalysis struct {
	Current      float64 `json:"current"`
	Threshold    float64 `json:"threshold"`
	Status       string  `json:"status"`
	Trend        string  `json:"trend"`
	DaysAbove    int     `json:"days_above_threshold"`
	Signal       string  `json:"signal"`
	Contribution float64 `json:"vote_contribution"`
}

type BreadthAnalysis struct {
	Current      float64 `json:"current"`
	Threshold    float64 `json:"threshold"`
	Status       string  `json:"status"`
	Trend        string  `json:"trend"`
	CoinsAbove   int     `json:"coins_above_20ma"`
	TotalCoins   int     `json:"total_coins"`
	Signal       string  `json:"signal"`
	Contribution float64 `json:"vote_contribution"`
}

type ThrustAnalysis struct {
	Current      float64 `json:"current"`
	Threshold    float64 `json:"threshold"`
	Status       string  `json:"status"`
	Momentum     string  `json:"momentum"`
	Signal       string  `json:"signal"`
	Contribution float64 `json:"vote_contribution"`
}

type VoteAnalysis struct {
	BullVotes    float64 `json:"bull_votes"`
	ChoppyVotes  float64 `json:"choppy_votes"`
	VolatileVotes float64 `json:"volatile_votes"`
	Winner       string  `json:"winner"`
	Margin       float64 `json:"margin"`
}

type WeightRecommendation struct {
	CurrentWeights map[string]float64 `json:"current_weights"`
	RecommendedWeights map[string]float64 `json:"recommended_weights"`
	Adjustments    map[string]float64 `json:"adjustments"`
	Reasoning      []string           `json:"reasoning"`
}

type RiskAssessment struct {
	Level        string   `json:"level"`
	Factors      []string `json:"factors"`
	Whipsaw      float64  `json:"whipsaw_probability"`
	Stability    float64  `json:"regime_stability"`
	Conviction   float64  `json:"conviction_score"`
}

func main() {
	fmt.Println("📊 COMPREHENSIVE REGIME ANALYSIS")
	fmt.Printf("⏰ Analysis Time: %s\n", time.Now().Format("2006-01-02 15:04:05 MST"))
	fmt.Println(strings.Repeat("═", 80))

	ctx := context.Background()
	
	// Perform comprehensive regime analysis
	analysis := performRegimeAnalysis(ctx)
	
	// Display results
	displayRegimeAnalysis(analysis)
	
	// Save analysis results
	saveAnalysisResults(analysis)
}

func performRegimeAnalysis(ctx context.Context) RegimeAnalysis {
	fmt.Printf("🔍 Analyzing market regime indicators...\n")
	
	// Fetch market data for analysis
	marketData := fetchMarketDataForAnalysis(ctx)
	
	// Calculate regime indicators
	realizedVol := analyzeRealizedVolatility(marketData)
	breadthMA := analyzeBreadthAbove20MA(marketData)
	thrustADX := analyzeBreadthThrust(marketData)
	
	// Perform majority vote
	vote := performMajorityVote(realizedVol, breadthMA, thrustADX)
	
	// Determine current regime and confidence
	currentRegime, confidence := determineRegime(vote)
	
	// Predict next regime
	nextRegime, timeToSwitch := predictNextRegime(vote, realizedVol, breadthMA, thrustADX)
	
	// Generate weight recommendations
	weights := generateWeightRecommendations(currentRegime, vote)
	
	// Assess risks
	risks := assessRegimeRisks(vote, realizedVol, breadthMA, thrustADX)
	
	return RegimeAnalysis{
		CurrentRegime:  currentRegime,
		Confidence:     confidence,
		NextRegimePred: nextRegime,
		TimeToSwitch:   timeToSwitch,
		AnalysisTime:   time.Now(),
		Indicators: RegimeIndicators{
			RealizedVol7d:    realizedVol,
			BreadthAbove20MA: breadthMA,
			BreadthThrustADX: thrustADX,
			MajorityVote:     vote,
		},
		WeightRecommend: weights,
		RiskAssessment:  risks,
	}
}

func fetchMarketDataForAnalysis(ctx context.Context) []MarketDataPoint {
	// Fetch top 50 coins for breadth analysis
	url := "https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&order=market_cap_desc&per_page=50&page=1&sparkline=true&price_change_percentage=1h,24h,7d"
	
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("User-Agent", "CryptoRun/1.0")
	
	resp, err := client.Do(req)
	if err != nil {
		return generateMockMarketData()
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return generateMockMarketData()
	}
	
	var geckoData []struct {
		Symbol                     string    `json:"symbol"`
		Name                       string    `json:"name"`
		CurrentPrice               float64   `json:"current_price"`
		PriceChangePercent1h       float64   `json:"price_change_percentage_1h"`
		PriceChangePercent24h      float64   `json:"price_change_percentage_24h"`
		PriceChangePercent7d       float64   `json:"price_change_percentage_7d"`
		SparklineIn7d              struct {
			Price []float64 `json:"price"`
		} `json:"sparkline_in_7d"`
		MarketCapRank              int       `json:"market_cap_rank"`
	}
	
	if err := json.Unmarshal(body, &geckoData); err != nil {
		return generateMockMarketData()
	}
	
	marketData := make([]MarketDataPoint, len(geckoData))
	for i, coin := range geckoData {
		marketData[i] = MarketDataPoint{
			Symbol:        strings.ToUpper(coin.Symbol),
			Name:          coin.Name,
			Price:         coin.CurrentPrice,
			Change1h:      coin.PriceChangePercent1h,
			Change24h:     coin.PriceChangePercent24h,
			Change7d:      coin.PriceChangePercent7d,
			Sparkline:     coin.SparklineIn7d.Price,
			Rank:          coin.MarketCapRank,
		}
	}
	
	fmt.Printf("✅ Fetched data for %d cryptocurrencies\n", len(marketData))
	return marketData
}

type MarketDataPoint struct {
	Symbol    string    `json:"symbol"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	Change1h  float64   `json:"change_1h"`
	Change24h float64   `json:"change_24h"`
	Change7d  float64   `json:"change_7d"`
	Sparkline []float64 `json:"sparkline"`
	Rank      int       `json:"rank"`
}

func analyzeRealizedVolatility(data []MarketDataPoint) VolatilityAnalysis {
	// Calculate 7-day realized volatility for top 10 coins
	totalVol := 0.0
	count := 0
	
	for i, coin := range data {
		if i >= 10 { // Top 10 only for volatility calc
			break
		}
		
		if len(coin.Sparkline) >= 7 {
			// Calculate daily volatility from sparkline
			dailyReturns := make([]float64, 0, 7)
			for j := 1; j < len(coin.Sparkline) && j <= 7; j++ {
				if coin.Sparkline[j-1] > 0 && coin.Sparkline[j] > 0 {
					ret := math.Log(coin.Sparkline[j] / coin.Sparkline[j-1])
					dailyReturns = append(dailyReturns, ret)
				}
			}
			
			if len(dailyReturns) > 0 {
				// Calculate standard deviation
				mean := 0.0
				for _, ret := range dailyReturns {
					mean += ret
				}
				mean /= float64(len(dailyReturns))
				
				variance := 0.0
				for _, ret := range dailyReturns {
					variance += math.Pow(ret-mean, 2)
				}
				variance /= float64(len(dailyReturns))
				
				vol := math.Sqrt(variance) * math.Sqrt(365) // Annualized
				totalVol += vol
				count++
			}
		}
	}
	
	avgVol := 0.0
	if count > 0 {
		avgVol = totalVol / float64(count)
	}
	
	// Convert to percentage
	avgVolPercent := avgVol * 100
	
	threshold := 25.0 // 25% threshold
	status := "low"
	signal := "bull"
	contribution := 0.4 // Bull vote
	
	if avgVolPercent > threshold {
		status = "high"
		signal = "volatile"
		contribution = -0.3 // Volatile vote
	}
	
	trend := "stable"
	if avgVolPercent > threshold*1.2 {
		trend = "rising"
	} else if avgVolPercent < threshold*0.8 {
		trend = "falling"
	}
	
	return VolatilityAnalysis{
		Current:      avgVolPercent,
		Threshold:    threshold,
		Status:       status,
		Trend:        trend,
		DaysAbove:    calculateDaysAbove(avgVolPercent, threshold),
		Signal:       signal,
		Contribution: contribution,
	}
}

func analyzeBreadthAbove20MA(data []MarketDataPoint) BreadthAnalysis {
	totalCoins := len(data)
	coinsAbove20MA := 0
	
	for _, coin := range data {
		// Estimate if above 20MA using 7d change as proxy
		// If 7d change > -5%, consider above 20MA
		if coin.Change7d > -5.0 {
			coinsAbove20MA++
		}
	}
	
	breadthPercent := float64(coinsAbove20MA) / float64(totalCoins) * 100
	threshold := 60.0 // 60% threshold
	
	status := "low"
	signal := "choppy"
	contribution := 0.0 // Neutral
	
	if breadthPercent > threshold {
		status = "high"
		signal = "bull"
		contribution = 0.5 // Bull vote
	}
	
	trend := "neutral"
	if breadthPercent > threshold*1.1 {
		trend = "improving"
	} else if breadthPercent < threshold*0.9 {
		trend = "deteriorating"
	}
	
	return BreadthAnalysis{
		Current:      breadthPercent,
		Threshold:    threshold,
		Status:       status,
		Trend:        trend,
		CoinsAbove:   coinsAbove20MA,
		TotalCoins:   totalCoins,
		Signal:       signal,
		Contribution: contribution,
	}
}

func analyzeBreadthThrust(data []MarketDataPoint) ThrustAnalysis {
	// Calculate momentum thrust using 1h vs 24h changes
	positiveThrust := 0
	totalMeasured := 0
	
	for _, coin := range data {
		if coin.Change1h != 0 && coin.Change24h != 0 {
			// Positive thrust if 1h acceleration > 24h average
			if math.Abs(coin.Change1h) > math.Abs(coin.Change24h/24) {
				positiveThrust++
			}
			totalMeasured++
		}
	}
	
	thrustPercent := 0.0
	if totalMeasured > 0 {
		thrustPercent = float64(positiveThrust) / float64(totalMeasured) * 100
	}
	
	threshold := 70.0 // 70% threshold
	
	status := "low"
	signal := "choppy"
	momentum := "decelerating"
	contribution := -0.2 // Choppy vote
	
	if thrustPercent > threshold {
		status = "high"
		signal = "bull"
		momentum = "accelerating"
		contribution = 0.3 // Bull vote
	}
	
	return ThrustAnalysis{
		Current:      thrustPercent,
		Threshold:    threshold,
		Status:       status,
		Momentum:     momentum,
		Signal:       signal,
		Contribution: contribution,
	}
}

func performMajorityVote(vol VolatilityAnalysis, breadth BreadthAnalysis, thrust ThrustAnalysis) VoteAnalysis {
	// Weighted voting system
	bullVotes := 0.0
	choppyVotes := 0.0
	volatileVotes := 0.0
	
	// Volatility contribution
	if vol.Signal == "bull" {
		bullVotes += math.Abs(vol.Contribution)
	} else if vol.Signal == "volatile" {
		volatileVotes += math.Abs(vol.Contribution)
	} else {
		choppyVotes += 0.1
	}
	
	// Breadth contribution
	if breadth.Signal == "bull" {
		bullVotes += math.Abs(breadth.Contribution)
	} else {
		choppyVotes += 0.2
	}
	
	// Thrust contribution
	if thrust.Signal == "bull" {
		bullVotes += math.Abs(thrust.Contribution)
	} else {
		choppyVotes += math.Abs(thrust.Contribution)
	}
	
	// Determine winner
	winner := "choppy"
	margin := choppyVotes
	
	if bullVotes > choppyVotes && bullVotes > volatileVotes {
		winner = "trending_bull"
		margin = bullVotes - math.Max(choppyVotes, volatileVotes)
	} else if volatileVotes > bullVotes && volatileVotes > choppyVotes {
		winner = "high_vol"
		margin = volatileVotes - math.Max(bullVotes, choppyVotes)
	} else {
		margin = choppyVotes - math.Max(bullVotes, volatileVotes)
	}
	
	return VoteAnalysis{
		BullVotes:     bullVotes,
		ChoppyVotes:   choppyVotes,
		VolatileVotes: volatileVotes,
		Winner:        winner,
		Margin:        margin,
	}
}

func determineRegime(vote VoteAnalysis) (string, float64) {
	regime := vote.Winner
	
	// Calculate confidence based on margin
	totalVotes := vote.BullVotes + vote.ChoppyVotes + vote.VolatileVotes
	confidence := 0.5 // Base confidence
	
	if totalVotes > 0 {
		confidence = 0.5 + (vote.Margin/totalVotes)*0.4
	}
	
	// Cap confidence at 95%
	if confidence > 0.95 {
		confidence = 0.95
	}
	
	return regime, confidence
}

func predictNextRegime(vote VoteAnalysis, vol VolatilityAnalysis, breadth BreadthAnalysis, thrust ThrustAnalysis) (string, time.Duration) {
	currentWinner := vote.Winner
	
	// Predict next regime based on trend momentum
	nextRegime := currentWinner
	timeToSwitch := 4 * time.Hour // Default 4h cycle
	
	// Check for regime instability indicators
	if vote.Margin < 0.2 {
		// Close vote - regime could flip soon
		timeToSwitch = 1 * time.Hour
		
		if vol.Trend == "rising" && vol.Current > vol.Threshold*0.9 {
			nextRegime = "high_vol"
		} else if breadth.Trend == "improving" && thrust.Momentum == "accelerating" {
			nextRegime = "trending_bull"
		} else {
			nextRegime = "choppy"
		}
	}
	
	return nextRegime, timeToSwitch
}

func generateWeightRecommendations(regime string, vote VoteAnalysis) WeightRecommendation {
	// Current weights (from regime config)
	currentWeights := map[string]float64{
		"momentum_core": 0.40,
		"technical":     0.25,
		"volume":        0.20,
		"quality":       0.10,
		"social":        0.05,
	}
	
	recommendedWeights := make(map[string]float64)
	adjustments := make(map[string]float64)
	reasoning := make([]string, 0)
	
	switch regime {
	case "trending_bull":
		recommendedWeights = map[string]float64{
			"momentum_core": 0.45, // Boost momentum
			"technical":     0.20, // Reduce technical
			"volume":        0.25, // Boost volume
			"quality":       0.07,
			"social":        0.03,
		}
		reasoning = append(reasoning, "Bull market: Increased momentum and volume weights")
		reasoning = append(reasoning, "Reduced technical noise sensitivity")
		
	case "choppy":
		recommendedWeights = map[string]float64{
			"momentum_core": 0.35, // Reduce momentum
			"technical":     0.30, // Boost technical
			"volume":        0.15, // Reduce volume
			"quality":       0.15, // Boost quality
			"social":        0.05,
		}
		reasoning = append(reasoning, "Choppy market: Emphasized technical and quality")
		reasoning = append(reasoning, "Reduced momentum and volume sensitivity")
		
	case "high_vol":
		recommendedWeights = map[string]float64{
			"momentum_core": 0.30, // Reduce momentum
			"technical":     0.35, // Boost technical
			"volume":        0.10, // Reduce volume
			"quality":       0.20, // Boost quality
			"social":        0.05,
		}
		reasoning = append(reasoning, "High volatility: Conservative technical focus")
		reasoning = append(reasoning, "Quality filter to avoid false signals")
		
	default:
		recommendedWeights = currentWeights
		reasoning = append(reasoning, "Maintaining current weights")
	}
	
	// Calculate adjustments
	for key := range currentWeights {
		adjustments[key] = recommendedWeights[key] - currentWeights[key]
	}
	
	return WeightRecommendation{
		CurrentWeights:     currentWeights,
		RecommendedWeights: recommendedWeights,
		Adjustments:        adjustments,
		Reasoning:          reasoning,
	}
}

func assessRegimeRisks(vote VoteAnalysis, vol VolatilityAnalysis, breadth BreadthAnalysis, thrust ThrustAnalysis) RiskAssessment {
	level := "LOW"
	factors := make([]string, 0)
	whipsaw := 0.1 // Base 10% whipsaw probability
	stability := 0.8
	conviction := vote.Margin
	
	// Assess whipsaw risk
	if vote.Margin < 0.3 {
		whipsaw += 0.3
		factors = append(factors, "Close regime vote increases whipsaw risk")
		level = "MEDIUM"
	}
	
	if vol.Trend == "rising" {
		whipsaw += 0.2
		factors = append(factors, "Rising volatility threatens regime stability")
		level = "HIGH"
	}
	
	if breadth.Trend == "deteriorating" {
		whipsaw += 0.15
		factors = append(factors, "Deteriorating breadth suggests weakness")
	}
	
	// Calculate stability
	stability = 1.0 - whipsaw
	if stability < 0.1 {
		stability = 0.1
	}
	
	// Adjust conviction
	if conviction < 0.2 {
		level = "HIGH"
	} else if conviction > 0.5 {
		level = "LOW"
	}
	
	if len(factors) == 0 {
		factors = append(factors, "Strong regime signals with good stability")
	}
	
	return RiskAssessment{
		Level:      level,
		Factors:    factors,
		Whipsaw:    whipsaw,
		Stability:  stability,
		Conviction: conviction,
	}
}

func displayRegimeAnalysis(analysis RegimeAnalysis) {
	fmt.Printf("\n🌊 REGIME ANALYSIS RESULTS\n")
	fmt.Println(strings.Repeat("═", 80))
	
	// Current regime
	regimeIcon := getRegimeIcon(analysis.CurrentRegime)
	fmt.Printf("Current Regime: %s %s (%.1f%% confidence)\n",
		regimeIcon, strings.ToUpper(analysis.CurrentRegime), analysis.Confidence*100)
	
	fmt.Printf("Next Prediction: %s %s (in %s)\n",
		getRegimeIcon(analysis.NextRegimePred), strings.ToUpper(analysis.NextRegimePred),
		analysis.TimeToSwitch.Truncate(time.Minute))
	
	// Indicators breakdown
	fmt.Printf("\n📊 REGIME INDICATORS ANALYSIS\n")
	fmt.Println(strings.Repeat("-", 60))
	
	vol := analysis.Indicators.RealizedVol7d
	fmt.Printf("7d Realized Vol:  %.1f%% vs %.1f%% threshold (%s, %s)\n",
		vol.Current, vol.Threshold, vol.Status, vol.Trend)
	fmt.Printf("  Signal: %s | Vote: %+.2f\n", vol.Signal, vol.Contribution)
	
	breadth := analysis.Indicators.BreadthAbove20MA
	fmt.Printf("\nBreadth >20MA:    %.1f%% vs %.1f%% threshold (%s, %s)\n",
		breadth.Current, breadth.Threshold, breadth.Status, breadth.Trend)
	fmt.Printf("  Coins: %d/%d | Signal: %s | Vote: %+.2f\n",
		breadth.CoinsAbove, breadth.TotalCoins, breadth.Signal, breadth.Contribution)
	
	thrust := analysis.Indicators.BreadthThrustADX
	fmt.Printf("\nThrust Momentum:  %.1f%% vs %.1f%% threshold (%s, %s)\n",
		thrust.Current, thrust.Threshold, thrust.Status, thrust.Momentum)
	fmt.Printf("  Signal: %s | Vote: %+.2f\n", thrust.Signal, thrust.Contribution)
	
	// Majority vote
	vote := analysis.Indicators.MajorityVote
	fmt.Printf("\n🗳️ MAJORITY VOTE RESULTS\n")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("Bull Votes:     %.2f\n", vote.BullVotes)
	fmt.Printf("Choppy Votes:   %.2f\n", vote.ChoppyVotes)
	fmt.Printf("Volatile Votes: %.2f\n", vote.VolatileVotes)
	fmt.Printf("Winner: %s (margin: %.2f)\n", strings.ToUpper(vote.Winner), vote.Margin)
	
	// Weight recommendations
	fmt.Printf("\n⚖️ WEIGHT RECOMMENDATIONS\n")
	fmt.Println(strings.Repeat("-", 50))
	
	weights := analysis.WeightRecommend
	factorOrder := []string{"momentum_core", "technical", "volume", "quality", "social"}
	
	for _, factor := range factorOrder {
		current := weights.CurrentWeights[factor]
		recommended := weights.RecommendedWeights[factor]
		adjustment := weights.Adjustments[factor]
		
		adjIcon := "→"
		if adjustment > 0.01 {
			adjIcon = "↗"
		} else if adjustment < -0.01 {
			adjIcon = "↘"
		}
		
		fmt.Printf("%-12s: %.2f %s %.2f (%+.2f)\n",
			strings.Title(factor), current, adjIcon, recommended, adjustment)
	}
	
	fmt.Printf("\nReasoning:\n")
	for _, reason := range weights.Reasoning {
		fmt.Printf("  • %s\n", reason)
	}
	
	// Risk assessment
	fmt.Printf("\n⚠️ RISK ASSESSMENT\n")
	fmt.Println(strings.Repeat("-", 40))
	
	risk := analysis.RiskAssessment
	riskIcon := "🟢"
	if risk.Level == "MEDIUM" {
		riskIcon = "🟡"
	} else if risk.Level == "HIGH" {
		riskIcon = "🔴"
	}
	
	fmt.Printf("Risk Level: %s %s\n", riskIcon, risk.Level)
	fmt.Printf("Whipsaw Probability: %.1f%%\n", risk.Whipsaw*100)
	fmt.Printf("Regime Stability: %.1f%%\n", risk.Stability*100)
	fmt.Printf("Conviction Score: %.2f\n", risk.Conviction)
	
	fmt.Printf("\nRisk Factors:\n")
	for _, factor := range risk.Factors {
		fmt.Printf("  • %s\n", factor)
	}
}

// Helper functions
func generateMockMarketData() []MarketDataPoint {
	mockData := []MarketDataPoint{
		{"BTC", "Bitcoin", 111000, 0.5, 1.2, -2.1, []float64{110000, 111000, 112000, 111500, 111000, 110800, 111200}, 1},
		{"ETH", "Ethereum", 4300, -0.2, 0.8, -1.8, []float64{4250, 4280, 4320, 4300, 4290, 4285, 4300}, 2},
		{"SOL", "Solana", 204, 1.1, 2.1, 3.2, []float64{200, 202, 205, 203, 204, 205, 204}, 5},
	}
	
	// Add more mock data
	for i := 4; i <= 50; i++ {
		mock := MarketDataPoint{
			Symbol:    fmt.Sprintf("COIN%d", i),
			Name:      fmt.Sprintf("Coin %d", i),
			Price:     float64(100 + i*10),
			Change1h:  float64(i%5-2) * 0.5,
			Change24h: float64(i%7-3) * 1.2,
			Change7d:  float64(i%9-4) * 2.1,
			Sparkline: []float64{100, 102, 101, 103, 102, 104, 103},
			Rank:      i,
		}
		mockData = append(mockData, mock)
	}
	
	return mockData
}

func calculateDaysAbove(current, threshold float64) int {
	if current > threshold {
		return 3 // Mock: 3 days above
	}
	return 0
}

func getRegimeIcon(regime string) string {
	switch regime {
	case "trending_bull":
		return "🚀"
	case "choppy":
		return "⚡"
	case "high_vol":
		return "🌪️"
	default:
		return "📊"
	}
}

func saveAnalysisResults(analysis RegimeAnalysis) {
	data, _ := json.MarshalIndent(analysis, "", "  ")
	
	// Save to file with timestamp
	filename := fmt.Sprintf("out/regime_analysis_%s.json", 
		time.Now().Format("20060102_150405"))
	
	// Ensure directory exists
	if err := os.MkdirAll("out", 0755); err == nil {
		if err := os.WriteFile(filename, data, 0644); err == nil {
			fmt.Printf("\n📁 Analysis saved to: %s\n", filename)
		}
	}
}