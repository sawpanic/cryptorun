package application

import (
	"math"
)

type TickerData struct {
	Symbol         string
	Volume24hBase  float64
	Volume24hQuote float64
	LastPrice      float64
	QuoteCurrency  string
}

type ADVResult struct {
	Symbol   string
	ADVUSD   int64
	Valid    bool
	ErrorMsg string
}

func CalculateADV(ticker TickerData) ADVResult {
	result := ADVResult{
		Symbol: ticker.Symbol,
		Valid:  false,
	}

	if ticker.QuoteCurrency == "USD" {
		if ticker.Volume24hQuote > 0 && isFinite(ticker.Volume24hQuote) {
			result.ADVUSD = int64(math.Round(ticker.Volume24hQuote))
			result.Valid = true
		} else if ticker.Volume24hBase > 0 && ticker.LastPrice > 0 &&
			isFinite(ticker.Volume24hBase) && isFinite(ticker.LastPrice) {
			advFloat := ticker.Volume24hBase * ticker.LastPrice
			if isFinite(advFloat) {
				result.ADVUSD = int64(math.Round(advFloat))
				result.Valid = true
			} else {
				result.ErrorMsg = "volume*price calculation resulted in non-finite value"
			}
		} else {
			result.ErrorMsg = "missing or invalid volume/price data"
		}
	} else {
		result.ErrorMsg = "non-USD quote currency: " + ticker.QuoteCurrency
	}

	if !result.Valid && result.ErrorMsg == "" {
		result.ErrorMsg = "unknown ADV calculation error"
	}

	return result
}

func isFinite(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0) && f >= 0
}

func BatchCalculateADV(tickers []TickerData, minADV int64) []ADVResult {
	var results []ADVResult

	for _, ticker := range tickers {
		result := CalculateADV(ticker)
		if result.Valid && result.ADVUSD >= minADV {
			results = append(results, result)
		}
	}

	return results
}
