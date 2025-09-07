package policy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyEnforcer_ValidateScanRequest(t *testing.T) {
	pe := NewPolicyEnforcer()
	ctx := context.Background()

	t.Run("valid_scan_request", func(t *testing.T) {
		req := ScanRequest{
			Symbol:     "BTCUSD",
			Venue:      "kraken",
			DataSource: "kraken",
			DataType:   "market_data",
			Price:      50000.0,
		}

		err := pe.ValidateScanRequest(ctx, req)
		assert.NoError(t, err)
	})

	t.Run("invalid_non_usd_pair", func(t *testing.T) {
		req := ScanRequest{
			Symbol:     "BTCEUR",
			Venue:      "kraken",
			DataSource: "kraken",
			DataType:   "market_data",
			Price:      50000.0,
		}

		err := pe.ValidateScanRequest(ctx, req)
		require.Error(t, err)
		
		var validationErr ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, ReasonNonUSDQuote, validationErr.Reason)
	})

	t.Run("invalid_aggregator_microstructure", func(t *testing.T) {
		req := ScanRequest{
			Symbol:     "BTCUSD",
			Venue:      "kraken",
			DataSource: "coingecko",
			DataType:   "depth",
			Price:      50000.0,
		}

		err := pe.ValidateScanRequest(ctx, req)
		require.Error(t, err)
		
		var validationErr ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, ReasonAggregatorBanned, validationErr.Reason)
	})
}

func TestPolicyEnforcer_ValidateGateRequest(t *testing.T) {
	pe := NewPolicyEnforcer()
	ctx := context.Background()

	t.Run("valid_gate_request", func(t *testing.T) {
		req := GateRequest{
			Symbol:     "ETHUSD",
			Venue:      "kraken",
			DataSource: "kraken",
			DataType:   "market_data",
			Price:      3000.0,
			Score:      85.0,
			VADR:       2.5,
		}

		err := pe.ValidateGateRequest(ctx, req)
		assert.NoError(t, err)
	})

	t.Run("invalid_stablecoin_depeg", func(t *testing.T) {
		req := GateRequest{
			Symbol:     "USDTUSD",
			Venue:      "kraken",
			DataSource: "kraken",
			DataType:   "market_data",
			Price:      1.01, // Depegged beyond threshold
			Score:      85.0,
			VADR:       2.5,
		}

		err := pe.ValidateGateRequest(ctx, req)
		require.Error(t, err)
		
		var validationErr ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, ReasonStablecoinDepeg, validationErr.Reason)
	})
}

func TestScannerIntegration_PreScanValidation(t *testing.T) {
	pe := NewPolicyEnforcer()
	si := NewScannerIntegration(pe)
	ctx := context.Background()

	t.Run("valid_symbols", func(t *testing.T) {
		symbols := []string{"BTCUSD", "ETHUSD", "ADAUSD"}
		venue := "kraken"

		err := si.PreScanValidation(ctx, symbols, venue)
		assert.NoError(t, err)
	})

	t.Run("invalid_symbols_mixed", func(t *testing.T) {
		symbols := []string{"BTCUSD", "BTCEUR", "ETHUSD"} // BTCEUR is invalid
		venue := "kraken"

		err := si.PreScanValidation(ctx, symbols, venue)
		require.Error(t, err)
		
		var validationErr ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, ReasonNonUSDQuote, validationErr.Reason)
		assert.Equal(t, "BTCEUR", validationErr.Symbol)
	})

	t.Run("global_pause_active", func(t *testing.T) {
		// Set global pause
		pe.validator.SetGlobalPause(true)
		defer pe.validator.SetGlobalPause(false)

		symbols := []string{"BTCUSD"}
		venue := "kraken"

		err := si.PreScanValidation(ctx, symbols, venue)
		require.Error(t, err)
		
		var validationErr ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, ReasonGlobalPause, validationErr.Reason)
	})
}

func TestScannerIntegration_ValidateSymbolForScanning(t *testing.T) {
	pe := NewPolicyEnforcer()
	si := NewScannerIntegration(pe)
	ctx := context.Background()

	t.Run("valid_symbol", func(t *testing.T) {
		err := si.ValidateSymbolForScanning(ctx, "BTCUSD", "kraken", 50000.0)
		assert.NoError(t, err)
	})

	t.Run("blacklisted_symbol", func(t *testing.T) {
		pe.validator.AddToBlacklist("SCAMUSD")
		defer pe.validator.RemoveFromBlacklist("SCAMUSD")

		err := si.ValidateSymbolForScanning(ctx, "SCAMUSD", "kraken", 1.0)
		require.Error(t, err)
		
		var validationErr ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, ReasonSymbolBlacklisted, validationErr.Reason)
	})
}

func TestGateIntegration_ValidateEntryGate(t *testing.T) {
	pe := NewPolicyEnforcer()
	gi := NewGateIntegration(pe)
	ctx := context.Background()

	t.Run("valid_entry", func(t *testing.T) {
		err := gi.ValidateEntryGate(ctx, "BTCUSD", "kraken", 85.0, 2.5, 50000.0)
		assert.NoError(t, err)
	})

	t.Run("venue_emergency_control", func(t *testing.T) {
		pe.validator.SetEmergencyControl("binance", "RISKYUSD", true)
		defer pe.validator.SetEmergencyControl("binance", "RISKYUSD", false)

		err := gi.ValidateEntryGate(ctx, "RISKYUSD", "binance", 85.0, 2.5, 100.0)
		require.Error(t, err)
		
		var validationErr ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, ReasonEmergencyControl, validationErr.Reason)
	})
}

func TestGateIntegration_ValidateMicrostructureData(t *testing.T) {
	pe := NewPolicyEnforcer()
	gi := NewGateIntegration(pe)
	ctx := context.Background()

	t.Run("valid_exchange_native", func(t *testing.T) {
		err := gi.ValidateMicrostructureData(ctx, "BTCUSD", "kraken", "kraken")
		assert.NoError(t, err)
	})

	t.Run("invalid_aggregator", func(t *testing.T) {
		err := gi.ValidateMicrostructureData(ctx, "BTCUSD", "kraken", "coingecko")
		require.Error(t, err)
		
		var validationErr ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, ReasonAggregatorBanned, validationErr.Reason)
	})

	t.Run("valid_binance_fallback", func(t *testing.T) {
		err := gi.ValidateMicrostructureData(ctx, "BTCUSD", "binance", "binance")
		assert.NoError(t, err)
	})
}

func TestGlobalPolicyManager(t *testing.T) {
	gpm := NewGlobalPolicyManager()

	t.Run("global_pause_control", func(t *testing.T) {
		// Initially not paused
		status := gpm.GetPolicyStatus()
		assert.False(t, status["global_pause"].(bool))

		// Set pause
		gpm.SetGlobalPause(true)
		status = gpm.GetPolicyStatus()
		assert.True(t, status["global_pause"].(bool))

		// Clear pause
		gpm.SetGlobalPause(false)
		status = gpm.GetPolicyStatus()
		assert.False(t, status["global_pause"].(bool))
	})

	t.Run("blacklist_management", func(t *testing.T) {
		// Add to blacklist
		gpm.AddToBlacklist("SCAMUSD")
		status := gpm.GetPolicyStatus()
		assert.Contains(t, status["blacklisted_symbols"].([]string), "SCAMUSD")

		// Remove from blacklist
		gpm.RemoveFromBlacklist("SCAMUSD")
		status = gpm.GetPolicyStatus()
		assert.NotContains(t, status["blacklisted_symbols"].([]string), "SCAMUSD")
	})

	t.Run("emergency_control_management", func(t *testing.T) {
		// Set emergency control
		gpm.SetEmergencyControl("binance", "RISKYUSD", true)
		status := gpm.GetPolicyStatus()
		assert.Contains(t, status["emergency_controls"].([]string), "binance:RISKYUSD")

		// Clear emergency control
		gpm.SetEmergencyControl("binance", "RISKYUSD", false)
		status = gpm.GetPolicyStatus()
		assert.NotContains(t, status["emergency_controls"].([]string), "binance:RISKYUSD")
	})

	t.Run("get_integrations", func(t *testing.T) {
		scannerIntegration := gpm.GetScannerIntegration()
		assert.NotNil(t, scannerIntegration)

		gateIntegration := gpm.GetGateIntegration()
		assert.NotNil(t, gateIntegration)

		enforcer := gpm.GetEnforcer()
		assert.NotNil(t, enforcer)
	})
}

func TestIntegrationWorkflows(t *testing.T) {
	gpm := NewGlobalPolicyManager()
	scannerIntegration := gpm.GetScannerIntegration()
	gateIntegration := gpm.GetGateIntegration()
	ctx := context.Background()

	t.Run("full_scan_to_gate_workflow", func(t *testing.T) {
		// Step 1: Pre-scan validation
		symbols := []string{"BTCUSD", "ETHUSD"}
		venue := "kraken"
		
		err := scannerIntegration.PreScanValidation(ctx, symbols, venue)
		require.NoError(t, err)

		// Step 2: Individual symbol validation during scanning
		for _, symbol := range symbols {
			price := 50000.0
			if symbol == "ETHUSD" {
				price = 3000.0
			}
			
			err := scannerIntegration.ValidateSymbolForScanning(ctx, symbol, venue, price)
			require.NoError(t, err)
		}

		// Step 3: Microstructure validation
		err = gateIntegration.ValidateMicrostructureData(ctx, "BTCUSD", venue, venue)
		require.NoError(t, err)

		// Step 4: Entry gate validation
		err = gateIntegration.ValidateEntryGate(ctx, "BTCUSD", venue, 85.0, 2.5, 50000.0)
		require.NoError(t, err)
	})

	t.Run("workflow_with_emergency_control", func(t *testing.T) {
		// Set emergency control for specific symbol
		gpm.SetEmergencyControl("kraken", "BTCUSD", true)
		defer gpm.SetEmergencyControl("kraken", "BTCUSD", false)

		// Pre-scan should fail
		symbols := []string{"BTCUSD"}
		venue := "kraken"
		
		err := scannerIntegration.PreScanValidation(ctx, symbols, venue)
		require.Error(t, err)
		
		var validationErr ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, ReasonEmergencyControl, validationErr.Reason)
	})

	t.Run("workflow_with_global_pause", func(t *testing.T) {
		// Set global pause
		gpm.SetGlobalPause(true)
		defer gpm.SetGlobalPause(false)

		// All operations should fail
		symbols := []string{"ETHUSD"}
		venue := "kraken"
		
		err := scannerIntegration.PreScanValidation(ctx, symbols, venue)
		require.Error(t, err)
		
		var validationErr ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, ReasonGlobalPause, validationErr.Reason)

		// Gate validation should also fail
		err = gateIntegration.ValidateEntryGate(ctx, "ETHUSD", venue, 85.0, 2.5, 3000.0)
		require.Error(t, err)
		
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, ReasonGlobalPause, validationErr.Reason)
	})
}