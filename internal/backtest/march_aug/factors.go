package march_aug

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// FactorCalculatorImpl implements the FactorCalculator interface
type FactorCalculatorImpl struct {
	regimeDetector *RegimeDetector
}

// NewFactorCalculator creates a new factor calculator
func NewFactorCalculator() *FactorCalculatorImpl {
	return &FactorCalculatorImpl{
		regimeDetector: NewRegimeDetector(),
	}
}

// CalculateMomentumFactors computes protected momentum factors with timeframe weights
func (f *FactorCalculatorImpl) CalculateMomentumFactors(data []MarketData) ([]MomentumFactors, error) {
	if len(data) < 25 { // Need at least 25 hours for 24h momentum
		return nil, fmt.Errorf("insufficient data points for momentum calculation: %d", len(data))
	}

	var results []MomentumFactors

	// Sort data by timestamp
	sort.Slice(data, func(i, j int) bool {
		return data[i].Timestamp.Before(data[j].Timestamp)
	})

	// Calculate momentum for each timepoint (starting from index 24 to have lookback)
	for i := 24; i < len(data); i++ {
		current := data[i]

		// Calculate momentum for different timeframes
		momentum1h := f.calculateReturn(data, i, 1)   // 1-hour momentum
		momentum4h := f.calculateReturn(data, i, 4)   // 4-hour momentum
		momentum12h := f.calculateReturn(data, i, 12) // 12-hour momentum
		momentum24h := f.calculateReturn(data, i, 24) // 24-hour momentum

		// Apply momentum weights: 1h=20%, 4h=35%, 12h=30%, 24h=15%
		composite := momentum1h*0.20 + momentum4h*0.35 + momentum12h*0.30 + momentum24h*0.15

		results = append(results, MomentumFactors{
			Symbol:      current.Symbol,
			Timestamp:   current.Timestamp,
			Momentum1h:  momentum1h,
			Momentum4h:  momentum4h,
			Momentum12h: momentum12h,
			Momentum24h: momentum24h,
			Composite:   composite,
			Protected:   true, // Momentum is always protected from orthogonalization
		})
	}

	return results, nil
}

// CalculateSupplyDemandFactors computes supply/demand factors including smart money divergence
func (f *FactorCalculatorImpl) CalculateSupplyDemandFactors(market []MarketData, funding []FundingData,
	oi []OpenInterestData, reserves []ReservesData) ([]SupplyDemandFactors, error) {

	if len(market) == 0 {
		return nil, fmt.Errorf("no market data provided")
	}

	var results []SupplyDemandFactors

	// Create lookup maps for efficient data access
	fundingMap := f.createFundingLookup(funding)
	oiMap := f.createOILookup(oi)
	reservesMap := f.createReservesLookup(reserves)

	for i, marketData := range market {
		if i < 24 { // Need lookback for calculations
			continue
		}

		timestamp := marketData.Timestamp
		symbol := marketData.Symbol

		// Calculate OI/ADV ratio
		oiAdv := f.calculateOIADV(symbol, timestamp, oiMap, market[i-23:i+1])

		// Calculate VADR (Volume-Adjusted Daily Range)
		vadr := f.calculateVADR(market[i-23 : i+1])

		// Calculate reserves flow
		reservesFlow := f.calculateReservesFlow(symbol, timestamp, reservesMap)

		// Calculate funding divergence
		fundingDiv := f.calculateFundingDivergence(symbol, timestamp, fundingMap)

		// Calculate smart money divergence: funding≤0 & price hold & OI residual
		smartMoneyDiv := f.calculateSmartMoneyDivergence(symbol, timestamp, fundingMap, oiMap, market[i-4:i+1])

		// Combine into composite score
		composite := f.combineSupplyDemandFactors(oiAdv, vadr, reservesFlow, fundingDiv, smartMoneyDiv)

		results = append(results, SupplyDemandFactors{
			Symbol:        symbol,
			Timestamp:     timestamp,
			OIADV:         oiAdv,
			VADR:          vadr,
			ReservesFlow:  reservesFlow,
			FundingDiv:    fundingDiv,
			SmartMoneyDiv: smartMoneyDiv,
			Composite:     composite,
		})
	}

	return results, nil
}

// CalculateCompositeScores combines all factors with regime-aware weights and orthogonalization
func (f *FactorCalculatorImpl) CalculateCompositeScores(momentum []MomentumFactors, supply []SupplyDemandFactors,
	catalyst []CatalystData, social []SocialData, regime []RegimeData) ([]CompositeScores, error) {

	if len(momentum) == 0 {
		return nil, fmt.Errorf("no momentum data provided")
	}

	var results []CompositeScores

	// Create lookup maps
	supplyMap := f.createSupplyLookup(supply)
	catalystMap := f.createCatalystLookup(catalyst)
	socialMap := f.createSocialLookup(social)
	regimeMap := f.createRegimeLookup(regime)

	for _, mom := range momentum {
		timestamp := mom.Timestamp
		symbol := mom.Symbol

		// Get current regime
		currentRegime := f.getCurrentRegime(timestamp, regimeMap)

		// Get other factor values
		supplyScore := f.getSupplyScore(symbol, timestamp, supplyMap)
		catalystScore := f.getCatalystScore(symbol, timestamp, catalystMap)
		socialScore := f.getSocialScore(symbol, timestamp, socialMap)

		// Apply Gram-Schmidt orthogonalization (momentum is protected)
		orthogonalized := f.orthogonalizeFactors(mom.Composite, supplyScore, catalystScore, socialScore)

		// Apply regime weights
		weighted := f.applyRegimeWeights(orthogonalized, currentRegime)

		// Calculate final score with social capping
		finalScore := weighted["momentum"] + weighted["supply"] + weighted["catalyst"] + math.Min(weighted["social"], 10.0)

		// Create attribution map
		attribution := map[string]float64{
			"momentum":      weighted["momentum"],
			"supply_demand": weighted["supply"],
			"catalyst_heat": weighted["catalyst"],
			"social_signal": math.Min(weighted["social"], 10.0),
		}

		results = append(results, CompositeScores{
			Symbol:        symbol,
			Timestamp:     timestamp,
			MomentumScore: weighted["momentum"],
			SupplyDemand:  weighted["supply"],
			CatalystHeat:  weighted["catalyst"],
			SocialSignal:  math.Min(weighted["social"], 10.0),
			FinalScore:    finalScore,
			Regime:        currentRegime.Regime,
			Attribution:   attribution,
		})
	}

	return results, nil
}

// Helper methods for factor calculations

func (f *FactorCalculatorImpl) calculateReturn(data []MarketData, currentIndex, hoursBack int) float64 {
	if currentIndex < hoursBack {
		return 0.0
	}

	current := data[currentIndex].Close
	past := data[currentIndex-hoursBack].Close

	if past == 0 {
		return 0.0
	}

	return (current - past) / past
}

func (f *FactorCalculatorImpl) calculateOIADV(symbol string, timestamp time.Time,
	oiMap map[string]map[time.Time]OpenInterestData, marketData []MarketData) float64 {

	if symbolData, exists := oiMap[symbol]; exists {
		// Find closest OI data
		var closestOI OpenInterestData
		minDiff := time.Hour * 24 // Max 24h difference

		for t, oi := range symbolData {
			diff := timestamp.Sub(t)
			if diff >= 0 && diff < minDiff {
				minDiff = diff
				closestOI = oi
			}
		}

		if minDiff < time.Hour*24 {
			// Calculate average daily volume
			totalVolume := 0.0
			for _, md := range marketData {
				totalVolume += md.Volume
			}
			avgDailyVolume := totalVolume / float64(len(marketData)) * 24 // Scale to daily

			if avgDailyVolume > 0 {
				return closestOI.OpenInterest / avgDailyVolume
			}
		}
	}

	return 1.0 // Default neutral value
}

func (f *FactorCalculatorImpl) calculateVADR(marketData []MarketData) float64 {
	if len(marketData) < 24 {
		return 1.0
	}

	// Calculate 24-hour range
	var high, low float64
	totalVolume := 0.0

	for i, data := range marketData {
		if i == 0 {
			high = data.High
			low = data.Low
		} else {
			high = math.Max(high, data.High)
			low = math.Min(low, data.Low)
		}
		totalVolume += data.Volume
	}

	dailyRange := (high - low) / low // Percentage range
	avgVolume := totalVolume / float64(len(marketData))

	// VADR = range adjusted by volume (higher volume = more significant range)
	if avgVolume > 0 {
		volumeAdjustment := math.Log(avgVolume/1000 + 1) // Log scale adjustment
		return dailyRange * volumeAdjustment * 10        // Scale for usability
	}

	return dailyRange * 10
}

func (f *FactorCalculatorImpl) calculateReservesFlow(symbol string, timestamp time.Time,
	reservesMap map[string]map[time.Time]ReservesData) float64 {

	if symbolData, exists := reservesMap[symbol]; exists {
		// Find closest reserves data
		var closestReserves ReservesData
		minDiff := time.Hour * 48 // Max 48h difference for daily data

		for t, res := range symbolData {
			diff := timestamp.Sub(t)
			if diff >= 0 && diff < minDiff {
				minDiff = diff
				closestReserves = res
			}
		}

		if minDiff < time.Hour*48 && closestReserves.Available {
			return closestReserves.ReservesPct * 100 // Convert to percentage points
		}
	}

	return 0.0 // Neutral if no data
}

func (f *FactorCalculatorImpl) calculateFundingDivergence(symbol string, timestamp time.Time,
	fundingMap map[string]map[time.Time]FundingData) float64 {

	if symbolData, exists := fundingMap[symbol]; exists {
		// Find closest funding data (within 8 hours)
		var closestFunding FundingData
		minDiff := time.Hour * 8

		for t, fund := range symbolData {
			diff := timestamp.Sub(t)
			if diff >= 0 && diff < minDiff {
				minDiff = diff
				closestFunding = fund
			}
		}

		if minDiff < time.Hour*8 {
			return closestFunding.Divergence // Already in standard deviations
		}
	}

	return 0.0 // Neutral if no data
}

func (f *FactorCalculatorImpl) calculateSmartMoneyDivergence(symbol string, timestamp time.Time,
	fundingMap map[string]map[time.Time]FundingData, oiMap map[string]map[time.Time]OpenInterestData,
	recentMarket []MarketData) float64 {

	// Get funding rate
	var fundingRate float64
	if symbolData, exists := fundingMap[symbol]; exists {
		for t, fund := range symbolData {
			if timestamp.Sub(t) < time.Hour*8 {
				fundingRate = fund.MedianFR
				break
			}
		}
	}

	// Get OI residual
	var oiResidual float64
	if symbolData, exists := oiMap[symbol]; exists {
		for t, oi := range symbolData {
			if timestamp.Sub(t) < time.Hour*4 {
				oiResidual = oi.OIResidual
				break
			}
		}
	}

	// Check if price is holding (low volatility in recent periods)
	priceStable := f.isPriceStable(recentMarket)

	// Smart money divergence: negative funding + stable price + positive OI residual
	if fundingRate <= 0 && priceStable && oiResidual > 0 {
		return math.Abs(fundingRate)*100 + oiResidual*50 // Scale appropriately
	}

	return 0.0
}

func (f *FactorCalculatorImpl) isPriceStable(marketData []MarketData) bool {
	if len(marketData) < 3 {
		return false
	}

	// Calculate recent volatility
	var returns []float64
	for i := 1; i < len(marketData); i++ {
		if marketData[i-1].Close > 0 {
			ret := (marketData[i].Close - marketData[i-1].Close) / marketData[i-1].Close
			returns = append(returns, ret)
		}
	}

	if len(returns) == 0 {
		return false
	}

	// Calculate standard deviation of returns
	mean := 0.0
	for _, ret := range returns {
		mean += ret
	}
	mean /= float64(len(returns))

	variance := 0.0
	for _, ret := range returns {
		variance += math.Pow(ret-mean, 2)
	}
	variance /= float64(len(returns) - 1)

	stdDev := math.Sqrt(variance)

	// Consider stable if volatility is below threshold
	return stdDev < 0.02 // 2% hourly volatility threshold
}

func (f *FactorCalculatorImpl) combineSupplyDemandFactors(oiAdv, vadr, reservesFlow, fundingDiv, smartMoneyDiv float64) float64 {
	// Weighted combination of supply/demand factors
	weights := map[string]float64{
		"oi_adv":          0.25,
		"vadr":            0.30,
		"reserves_flow":   0.15,
		"funding_div":     0.20,
		"smart_money_div": 0.10,
	}

	composite := oiAdv*weights["oi_adv"] +
		vadr*weights["vadr"] +
		reservesFlow*weights["reserves_flow"] +
		fundingDiv*weights["funding_div"] +
		smartMoneyDiv*weights["smart_money_div"]

	return composite
}

func (f *FactorCalculatorImpl) orthogonalizeFactors(momentum, supply, catalyst, social float64) map[string]float64 {
	// Momentum is protected - never orthogonalized
	orthogonalized := map[string]float64{
		"momentum": momentum,
		"supply":   supply,
		"catalyst": catalyst,
		"social":   social,
	}

	// Apply Gram-Schmidt orthogonalization to non-momentum factors
	// Order: momentum (protected), supply, catalyst, social

	// Supply orthogonalized against momentum
	correlation := 0.3 // Mock correlation between momentum and supply
	orthogonalized["supply"] = supply - correlation*momentum

	// Catalyst orthogonalized against momentum and supply
	corrMomCat := 0.2
	corrSupCat := 0.25
	orthogonalized["catalyst"] = catalyst - corrMomCat*momentum - corrSupCat*orthogonalized["supply"]

	// Social orthogonalized against all previous factors
	corrMomSoc := 0.15
	corrSupSoc := 0.20
	corrCatSoc := 0.10
	orthogonalized["social"] = social - corrMomSoc*momentum -
		corrSupSoc*orthogonalized["supply"] - corrCatSoc*orthogonalized["catalyst"]

	return orthogonalized
}

func (f *FactorCalculatorImpl) applyRegimeWeights(orthFactors map[string]float64, regime RegimeData) map[string]float64 {
	weights := f.getRegimeWeights(regime.Regime)

	weighted := map[string]float64{
		"momentum": orthFactors["momentum"] * weights["momentum"],
		"supply":   orthFactors["supply"] * weights["supply"],
		"catalyst": orthFactors["catalyst"] * weights["catalyst"],
		"social":   orthFactors["social"] * weights["social"],
	}

	return weighted
}

func (f *FactorCalculatorImpl) getRegimeWeights(regime string) map[string]float64 {
	switch regime {
	case "trending_bull":
		return map[string]float64{
			"momentum": 0.50, "supply": 0.25, "catalyst": 0.15, "social": 0.10,
		}
	case "choppy":
		return map[string]float64{
			"momentum": 0.35, "supply": 0.35, "catalyst": 0.20, "social": 0.10,
		}
	case "high_vol":
		return map[string]float64{
			"momentum": 0.30, "supply": 0.40, "catalyst": 0.20, "social": 0.10,
		}
	default:
		return map[string]float64{
			"momentum": 0.40, "supply": 0.30, "catalyst": 0.20, "social": 0.10,
		}
	}
}

// Lookup creation methods

func (f *FactorCalculatorImpl) createFundingLookup(funding []FundingData) map[string]map[time.Time]FundingData {
	lookup := make(map[string]map[time.Time]FundingData)
	for _, fund := range funding {
		if lookup[fund.Symbol] == nil {
			lookup[fund.Symbol] = make(map[time.Time]FundingData)
		}
		lookup[fund.Symbol][fund.Timestamp] = fund
	}
	return lookup
}

func (f *FactorCalculatorImpl) createOILookup(oi []OpenInterestData) map[string]map[time.Time]OpenInterestData {
	lookup := make(map[string]map[time.Time]OpenInterestData)
	for _, oiData := range oi {
		if lookup[oiData.Symbol] == nil {
			lookup[oiData.Symbol] = make(map[time.Time]OpenInterestData)
		}
		lookup[oiData.Symbol][oiData.Timestamp] = oiData
	}
	return lookup
}

func (f *FactorCalculatorImpl) createReservesLookup(reserves []ReservesData) map[string]map[time.Time]ReservesData {
	lookup := make(map[string]map[time.Time]ReservesData)
	for _, res := range reserves {
		if lookup[res.Symbol] == nil {
			lookup[res.Symbol] = make(map[time.Time]ReservesData)
		}
		lookup[res.Symbol][res.Timestamp] = res
	}
	return lookup
}

func (f *FactorCalculatorImpl) createSupplyLookup(supply []SupplyDemandFactors) map[string]map[time.Time]SupplyDemandFactors {
	lookup := make(map[string]map[time.Time]SupplyDemandFactors)
	for _, sup := range supply {
		if lookup[sup.Symbol] == nil {
			lookup[sup.Symbol] = make(map[time.Time]SupplyDemandFactors)
		}
		lookup[sup.Symbol][sup.Timestamp] = sup
	}
	return lookup
}

func (f *FactorCalculatorImpl) createCatalystLookup(catalyst []CatalystData) map[string]map[time.Time]CatalystData {
	lookup := make(map[string]map[time.Time]CatalystData)
	for _, cat := range catalyst {
		if lookup[cat.Symbol] == nil {
			lookup[cat.Symbol] = make(map[time.Time]CatalystData)
		}
		lookup[cat.Symbol][cat.Timestamp] = cat
	}
	return lookup
}

func (f *FactorCalculatorImpl) createSocialLookup(social []SocialData) map[string]map[time.Time]SocialData {
	lookup := make(map[string]map[time.Time]SocialData)
	for _, soc := range social {
		if lookup[soc.Symbol] == nil {
			lookup[soc.Symbol] = make(map[time.Time]SocialData)
		}
		lookup[soc.Symbol][soc.Timestamp] = soc
	}
	return lookup
}

func (f *FactorCalculatorImpl) createRegimeLookup(regime []RegimeData) map[time.Time]RegimeData {
	lookup := make(map[time.Time]RegimeData)
	for _, reg := range regime {
		lookup[reg.Timestamp] = reg
	}
	return lookup
}

// Data access methods

func (f *FactorCalculatorImpl) getCurrentRegime(timestamp time.Time, regimeMap map[time.Time]RegimeData) RegimeData {
	// Find the most recent regime data
	var latest RegimeData
	var latestTime time.Time

	for t, regime := range regimeMap {
		if t.Before(timestamp) || t.Equal(timestamp) {
			if latestTime.IsZero() || t.After(latestTime) {
				latestTime = t
				latest = regime
			}
		}
	}

	if latestTime.IsZero() {
		// Default regime if none found
		return RegimeData{
			Timestamp:  timestamp,
			Regime:     "choppy",
			Confidence: 0.5,
		}
	}

	return latest
}

func (f *FactorCalculatorImpl) getSupplyScore(symbol string, timestamp time.Time,
	supplyMap map[string]map[time.Time]SupplyDemandFactors) float64 {

	if symbolData, exists := supplyMap[symbol]; exists {
		// Find closest data within 4 hours
		for t, supply := range symbolData {
			if timestamp.Sub(t) >= 0 && timestamp.Sub(t) < time.Hour*4 {
				return supply.Composite
			}
		}
	}
	return 0.0
}

func (f *FactorCalculatorImpl) getCatalystScore(symbol string, timestamp time.Time,
	catalystMap map[string]map[time.Time]CatalystData) float64 {

	totalHeat := 0.0
	if symbolData, exists := catalystMap[symbol]; exists {
		// Sum all catalyst heat within 4 weeks
		for t, cat := range symbolData {
			diff := timestamp.Sub(t)
			if diff >= 0 && diff < time.Hour*24*7*4 { // 4 weeks
				totalHeat += cat.HeatScore
			}
		}
	}
	return totalHeat
}

func (f *FactorCalculatorImpl) getSocialScore(symbol string, timestamp time.Time,
	socialMap map[string]map[time.Time]SocialData) float64 {

	if symbolData, exists := socialMap[symbol]; exists {
		// Find closest social data within 8 hours
		for t, social := range symbolData {
			if timestamp.Sub(t) >= 0 && timestamp.Sub(t) < time.Hour*8 {
				return social.SocialScore
			}
		}
	}
	return 0.0
}
