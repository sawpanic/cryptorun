package domain

type Pair struct{ Base, Quote string }

type PairFilterConfig struct {
	MinDailyVolumeUSD  float64
	MinHistoryDays     int
	Quote              string
	ExcludeStablecoins bool
}

func AllowKrakenUSD(pair Pair, volUSD float64, historyDays int, cfg PairFilterConfig) bool {
	if pair.Quote != "USD" {
		return false
	}
	if cfg.ExcludeStablecoins && (pair.Base == "USDT" || pair.Base == "USDC") {
		return false
	}
	if volUSD < cfg.MinDailyVolumeUSD {
		return false
	}
	if historyDays < cfg.MinHistoryDays {
		return false
	}
	return true
}
