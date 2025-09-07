package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyValidator_ValidateUSDOnly(t *testing.T) {
	pv := NewPolicyValidator()

	tests := []struct {
		name        string
		symbol      string
		expectError bool
		errorReason ReasonCode
	}{
		{
			name:        "valid_USD_pair",
			symbol:      "BTCUSD",
			expectError: false,
		},
		{
			name:        "valid_USD_lowercase",
			symbol:      "ethusd",
			expectError: false,
		},
		{
			name:        "invalid_EUR_pair",
			symbol:      "BTCEUR",
			expectError: true,
			errorReason: ReasonNonUSDQuote,
		},
		{
			name:        "invalid_USDT_pair",
			symbol:      "BTCUSDT",
			expectError: true,
			errorReason: ReasonNonUSDQuote,
		},
		{
			name:        "invalid_no_quote",
			symbol:      "BTC",
			expectError: true,
			errorReason: ReasonNonUSDQuote,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pv.ValidateUSDOnly(tt.symbol)
			
			if tt.expectError {
				require.Error(t, err)
				var validationErr ValidationError
				require.ErrorAs(t, err, &validationErr)
				assert.Equal(t, tt.errorReason, validationErr.Reason)
				assert.Equal(t, tt.symbol, validationErr.Symbol)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPolicyValidator_ValidateVenuePreference(t *testing.T) {
	pv := NewPolicyValidator()

	tests := []struct {
		name          string
		venue         string
		allowFallback bool
		expectError   bool
		errorReason   ReasonCode
	}{
		{
			name:          "preferred_kraken",
			venue:         "kraken",
			allowFallback: false,
			expectError:   false,
		},
		{
			name:          "fallback_binance_allowed",
			venue:         "binance",
			allowFallback: true,
			expectError:   false,
		},
		{
			name:          "fallback_binance_not_allowed",
			venue:         "binance",
			allowFallback: false,
			expectError:   true,
			errorReason:   ReasonVenueNotPreferred,
		},
		{
			name:          "unknown_venue",
			venue:         "unknown",
			allowFallback: true,
			expectError:   true,
			errorReason:   ReasonVenueNotPreferred,
		},
		{
			name:          "case_insensitive_kraken",
			venue:         "KRAKEN",
			allowFallback: false,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pv.ValidateVenuePreference(tt.venue, tt.allowFallback)
			
			if tt.expectError {
				require.Error(t, err)
				var validationErr ValidationError
				require.ErrorAs(t, err, &validationErr)
				assert.Equal(t, tt.errorReason, validationErr.Reason)
				assert.Equal(t, tt.venue, validationErr.Venue)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPolicyValidator_ValidateAggregatorBan(t *testing.T) {
	pv := NewPolicyValidator()

	tests := []struct {
		name         string
		dataSource   string
		dataType     string
		expectError  bool
		errorReason  ReasonCode
	}{
		{
			name:        "allowed_kraken_depth",
			dataSource:  "kraken",
			dataType:    "depth",
			expectError: false,
		},
		{
			name:        "banned_coingecko_depth",
			dataSource:  "coingecko",
			dataType:    "depth",
			expectError: true,
			errorReason: ReasonAggregatorBanned,
		},
		{
			name:        "banned_dexscreener_orderbook",
			dataSource:  "dexscreener",
			dataType:    "orderbook",
			expectError: true,
			errorReason: ReasonAggregatorBanned,
		},
		{
			name:        "allowed_coingecko_price",
			dataSource:  "coingecko",
			dataType:    "price_data",
			expectError: false,
		},
		{
			name:        "allowed_cmc_market",
			dataSource:  "cmc",
			dataType:    "market_data",
			expectError: false,
		},
		{
			name:        "banned_etherscan_trades",
			dataSource:  "etherscan",
			dataType:    "trades",
			expectError: true,
			errorReason: ReasonAggregatorBanned,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pv.ValidateAggregatorBan(tt.dataSource, tt.dataType)
			
			if tt.expectError {
				require.Error(t, err)
				var validationErr ValidationError
				require.ErrorAs(t, err, &validationErr)
				assert.Equal(t, tt.errorReason, validationErr.Reason)
				assert.Equal(t, tt.dataSource, validationErr.Venue)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPolicyValidator_ValidateStablecoinDepeg(t *testing.T) {
	pv := NewPolicyValidator()

	tests := []struct {
		name        string
		symbol      string
		price       float64
		expectError bool
		errorReason ReasonCode
	}{
		{
			name:        "non_stablecoin",
			symbol:      "BTCUSD",
			price:       50000.0,
			expectError: false,
		},
		{
			name:        "usdt_normal_peg",
			symbol:      "USDTUSD",
			price:       1.001,
			expectError: false,
		},
		{
			name:        "usdc_at_threshold",
			symbol:      "USDCUSD",
			price:       1.005,
			expectError: false,
		},
		{
			name:        "usdt_depegged_high",
			symbol:      "USDTUSD",
			price:       1.008,
			expectError: true,
			errorReason: ReasonStablecoinDepeg,
		},
		{
			name:        "usdc_depegged_low",
			symbol:      "USDCUSD",
			price:       0.992,
			expectError: true,
			errorReason: ReasonStablecoinDepeg,
		},
		{
			name:        "dai_severely_depegged",
			symbol:      "DAIUSD",
			price:       0.95,
			expectError: true,
			errorReason: ReasonStablecoinDepeg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pv.ValidateStablecoinDepeg(tt.symbol, tt.price)
			
			if tt.expectError {
				require.Error(t, err)
				var validationErr ValidationError
				require.ErrorAs(t, err, &validationErr)
				assert.Equal(t, tt.errorReason, validationErr.Reason)
				assert.Equal(t, tt.symbol, validationErr.Symbol)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPolicyValidator_ValidateEmergencyControls(t *testing.T) {
	pv := NewPolicyValidator()

	t.Run("normal_operation", func(t *testing.T) {
		err := pv.ValidateEmergencyControls("BTCUSD", "kraken")
		assert.NoError(t, err)
	})

	t.Run("global_pause", func(t *testing.T) {
		pv.SetGlobalPause(true)
		defer pv.SetGlobalPause(false)
		
		err := pv.ValidateEmergencyControls("BTCUSD", "kraken")
		require.Error(t, err)
		
		var validationErr ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, ReasonGlobalPause, validationErr.Reason)
	})

	t.Run("symbol_blacklist", func(t *testing.T) {
		pv.AddToBlacklist("BTCUSD")
		defer pv.RemoveFromBlacklist("BTCUSD")
		
		err := pv.ValidateEmergencyControls("BTCUSD", "kraken")
		require.Error(t, err)
		
		var validationErr ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, ReasonSymbolBlacklisted, validationErr.Reason)
		assert.Equal(t, "BTCUSD", validationErr.Symbol)
	})

	t.Run("venue_specific_emergency", func(t *testing.T) {
		pv.SetEmergencyControl("kraken", "ETHUSD", true)
		defer pv.SetEmergencyControl("kraken", "ETHUSD", false)
		
		err := pv.ValidateEmergencyControls("ETHUSD", "kraken")
		require.Error(t, err)
		
		var validationErr ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, ReasonEmergencyControl, validationErr.Reason)
		assert.Equal(t, "ETHUSD", validationErr.Symbol)
		assert.Equal(t, "kraken", validationErr.Venue)
	})
}

func TestPolicyValidator_ValidateAll(t *testing.T) {
	pv := NewPolicyValidator()

	t.Run("all_validations_pass", func(t *testing.T) {
		err := pv.ValidateAll("BTCUSD", "kraken", "kraken", "market_data", 50000.0)
		assert.NoError(t, err)
	})

	t.Run("usd_validation_fails", func(t *testing.T) {
		err := pv.ValidateAll("BTCEUR", "kraken", "kraken", "market_data", 50000.0)
		require.Error(t, err)
		
		var validationErr ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, ReasonNonUSDQuote, validationErr.Reason)
	})

	t.Run("aggregator_validation_fails", func(t *testing.T) {
		err := pv.ValidateAll("BTCUSD", "kraken", "coingecko", "depth", 50000.0)
		require.Error(t, err)
		
		var validationErr ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, ReasonAggregatorBanned, validationErr.Reason)
	})

	t.Run("stablecoin_validation_fails", func(t *testing.T) {
		err := pv.ValidateAll("USDTUSD", "kraken", "kraken", "market_data", 1.01)
		require.Error(t, err)
		
		var validationErr ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, ReasonStablecoinDepeg, validationErr.Reason)
	})

	t.Run("emergency_controls_fail", func(t *testing.T) {
		pv.SetGlobalPause(true)
		defer pv.SetGlobalPause(false)
		
		err := pv.ValidateAll("BTCUSD", "kraken", "kraken", "market_data", 50000.0)
		require.Error(t, err)
		
		var validationErr ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, ReasonGlobalPause, validationErr.Reason)
	})
}

func TestPolicyValidator_GetStatus(t *testing.T) {
	pv := NewPolicyValidator()

	// Add some test data
	pv.AddToBlacklist("SCAMUSD")
	pv.SetEmergencyControl("binance", "RISKYUSD", true)
	pv.SetGlobalPause(true)

	status := pv.GetStatus()

	assert.True(t, status["global_pause"].(bool))
	assert.Contains(t, status["blacklisted_symbols"].([]string), "SCAMUSD")
	assert.Contains(t, status["emergency_controls"].([]string), "binance:RISKYUSD")
	assert.Equal(t, []string{"kraken", "binance", "okx", "coinbase"}, status["venue_preference_order"].([]string))
	assert.Equal(t, 0.005, status["stablecoin_threshold"].(float64))
	assert.Contains(t, status["banned_aggregators"].([]string), "coingecko")
	assert.Contains(t, status["banned_aggregators"].([]string), "dexscreener")
}

func TestHelperFunctions(t *testing.T) {
	t.Run("isMicrostructureData", func(t *testing.T) {
		tests := []struct {
			dataType string
			expected bool
		}{
			{"depth", true},
			{"orderbook", true},
			{"l1", true},
			{"l2", true},
			{"trades", true},
			{"ticker", true},
			{"spread", true},
			{"price_data", false},
			{"market_data", false},
			{"social_data", false},
			{"DEPTH", true}, // Case insensitive
		}

		for _, tt := range tests {
			assert.Equal(t, tt.expected, isMicrostructureData(tt.dataType), 
				"dataType: %s", tt.dataType)
		}
	})

	t.Run("isStablecoin", func(t *testing.T) {
		tests := []struct {
			symbol   string
			expected bool
		}{
			{"USDTUSD", true},
			{"USDCUSD", true},
			{"DAIUSD", true},
			{"BUSDEUR", true},
			{"BTCUSD", false},
			{"ETHUSD", false},
			{"usdt", true}, // Case insensitive
		}

		for _, tt := range tests {
			assert.Equal(t, tt.expected, isStablecoin(tt.symbol), 
				"symbol: %s", tt.symbol)
		}
	})
}