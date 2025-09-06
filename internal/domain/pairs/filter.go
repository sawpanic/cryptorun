package pairs

import "strings"

// IsValidUSDPair enforces USD-only pairs, excluding stablecoin bases like USDT/USDC/DAI.
// Accepts symbols in either KRAKEN naming (e.g., "XBT/USD") or common ("BTC/USD").
func IsValidUSDPair(symbol string) bool {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	if !strings.HasSuffix(s, "/USD") {
		return false
	}
	base := strings.TrimSuffix(s, "/USD")
	switch base {
	case "USDT", "USDC", "DAI", "BUSD", "TUSD", "USDP":
		return false
	default:
		return true
	}
}
