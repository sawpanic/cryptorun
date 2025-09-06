package premove

import (
	"context"
	"testing"

	"cryptorun/src/domain/premove/ports"
	"cryptorun/src/domain/premove/proxy"
)

func TestSupplyProxy_GateCounting(t *testing.T) {
	supplyProxy := proxy.NewSupplyProxy()

	tests := []struct {
		name          string
		inputs        ports.ProxyInputs
		expectedGates int
		requireVolume bool
	}{
		{
			name: "all_gates_risk_on",
			inputs: ports.ProxyInputs{
				FundingZBelowNeg15:     true,
				SpotAboveVWAP24h:       true,
				ExchangeReserves7dDown: true,
				WhaleAccum2of3:         true,
				VolumeFirstBarP80:      true,
				Regime:                 "risk_on",
			},
			expectedGates: 3, // Gate A + Gate B + Gate C
			requireVolume: false,
		},
		{
			name: "risk_off_with_volume",
			inputs: ports.ProxyInputs{
				FundingZBelowNeg15:     true,
				SpotAboveVWAP24h:       true,
				ExchangeReserves7dDown: false,
				WhaleAccum2of3:         true,
				VolumeFirstBarP80:      true,
				Regime:                 "risk_off",
			},
			expectedGates: 3, // Gate A + Gate B + Gate C
			requireVolume: true,
		},
		{
			name: "risk_off_no_volume",
			inputs: ports.ProxyInputs{
				FundingZBelowNeg15:     true,
				SpotAboveVWAP24h:       true,
				ExchangeReserves7dDown: false,
				WhaleAccum2of3:         true,
				VolumeFirstBarP80:      false,
				Regime:                 "risk_off",
			},
			expectedGates: 2, // Gate A + Gate B (no volume gate)
			requireVolume: true,
		},
		{
			name: "btc_driven_volume_required",
			inputs: ports.ProxyInputs{
				FundingZBelowNeg15:     false,
				SpotAboveVWAP24h:       true,
				ExchangeReserves7dDown: true,
				WhaleAccum2of3:         true,
				VolumeFirstBarP80:      true,
				Regime:                 "btc_driven",
			},
			expectedGates: 2, // Gate B + Gate C
			requireVolume: true,
		},
		{
			name: "selective_no_requirements",
			inputs: ports.ProxyInputs{
				FundingZBelowNeg15:     false,
				SpotAboveVWAP24h:       false,
				ExchangeReserves7dDown: false,
				WhaleAccum2of3:         false,
				VolumeFirstBarP80:      true,
				Regime:                 "selective",
			},
			expectedGates: 1, // Only Gate C (volume)
			requireVolume: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gates, requireVol := supplyProxy.Evaluate(tt.inputs)

			if gates != tt.expectedGates {
				t.Errorf("Expected %d gates, got %d", tt.expectedGates, gates)
			}

			if requireVol != tt.requireVolume {
				t.Errorf("Expected requireVolumeConfirm=%v, got %v", tt.requireVolume, requireVol)
			}
		})
	}
}

func TestSupplyProxy_GateLogic(t *testing.T) {
	supplyProxy := proxy.NewSupplyProxy()

	// Test Gate A: funding divergence
	inputs := ports.ProxyInputs{
		FundingZBelowNeg15:     true,
		SpotAboveVWAP24h:       true,
		ExchangeReserves7dDown: false,
		WhaleAccum2of3:         false,
		VolumeFirstBarP80:      false,
		Regime:                 "risk_on",
	}
	gates, _ := supplyProxy.Evaluate(inputs)
	if gates != 1 {
		t.Errorf("Gate A should pass when both funding and spot conditions met")
	}

	// Test Gate A fails when conditions not met
	inputs.SpotAboveVWAP24h = false
	gates, _ = supplyProxy.Evaluate(inputs)
	if gates != 0 {
		t.Errorf("Gate A should fail when spot condition not met")
	}

	// Test Gate B: supply squeeze
	inputs = ports.ProxyInputs{
		FundingZBelowNeg15:     false,
		SpotAboveVWAP24h:       false,
		ExchangeReserves7dDown: true,
		WhaleAccum2of3:         false,
		VolumeFirstBarP80:      false,
		Regime:                 "risk_on",
	}
	gates, _ = supplyProxy.Evaluate(inputs)
	if gates != 1 {
		t.Errorf("Gate B should pass with reserves down")
	}

	// Test Gate B with whale accumulation
	inputs.ExchangeReserves7dDown = false
	inputs.WhaleAccum2of3 = true
	gates, _ = supplyProxy.Evaluate(inputs)
	if gates != 1 {
		t.Errorf("Gate B should pass with whale accumulation")
	}

	// Test Gate C: volume confirmation
	inputs = ports.ProxyInputs{
		FundingZBelowNeg15:     false,
		SpotAboveVWAP24h:       false,
		ExchangeReserves7dDown: false,
		WhaleAccum2of3:         false,
		VolumeFirstBarP80:      true,
		Regime:                 "risk_on",
	}
	gates, _ = supplyProxy.Evaluate(inputs)
	if gates != 1 {
		t.Errorf("Gate C should pass with volume confirmation")
	}
}

func TestSupplyProxy_EvaluateDetailed(t *testing.T) {
	evaluator := proxy.NewSupplyProxy()

	inputs := ports.ProxyInputs{
		FundingZBelowNeg15:     true,
		SpotAboveVWAP24h:       true,
		ExchangeReserves7dDown: false,
		WhaleAccum2of3:         true,
		VolumeFirstBarP80:      true,
		Regime:                 "risk_off",
	}

	ctx := context.Background()
	result, err := evaluator.EvaluateDetailed(ctx, inputs)

	if err != nil {
		t.Fatalf("EvaluateDetailed failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if result.GatesPassed != 3 {
		t.Errorf("Expected 3 gates passed, got %d", result.GatesPassed)
	}

	if !result.RequireVolumeConfirm {
		t.Error("Should require volume confirmation for risk_off regime")
	}

	expectedScore := 40.0 + 35.0 + 25.0 // All gates pass
	if result.Score != expectedScore {
		t.Errorf("Expected score %f, got %f", expectedScore, result.Score)
	}

	// Check gate details
	if !result.GateDetails["gate_a_funding_spot"] {
		t.Error("Gate A should pass")
	}
	if !result.GateDetails["gate_b_reserves_whales"] {
		t.Error("Gate B should pass")
	}
	if !result.GateDetails["gate_c_volume"] {
		t.Error("Gate C should pass")
	}
}

func TestSupplyProxy_VolumeConfirmRequirement(t *testing.T) {
	supplyProxy := proxy.NewSupplyProxy()

	regimeTests := []struct {
		regime        string
		requireVolume bool
	}{
		{"risk_on", false},
		{"risk_off", true},
		{"btc_driven", true},
		{"selective", false},
	}

	for _, tt := range regimeTests {
		t.Run(tt.regime, func(t *testing.T) {
			inputs := ports.ProxyInputs{
				FundingZBelowNeg15:     false,
				SpotAboveVWAP24h:       false,
				ExchangeReserves7dDown: false,
				WhaleAccum2of3:         false,
				VolumeFirstBarP80:      false,
				Regime:                 tt.regime,
			}

			_, requireVol := supplyProxy.Evaluate(inputs)

			if requireVol != tt.requireVolume {
				t.Errorf("Regime %s: expected requireVolumeConfirm=%v, got %v",
					tt.regime, tt.requireVolume, requireVol)
			}
		})
	}
}
