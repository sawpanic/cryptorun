package premove

import (
	"context"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/src/application/premove"
	"github.com/sawpanic/cryptorun/src/domain/premove/ports"
)

// Mock implementations for testing
type mockPercentileEngine struct{}

func (m *mockPercentileEngine) Percentile(values []float64, p float64, w ports.PercentileWindow) (float64, bool) {
	if len(values) < 10 {
		return 0.0, false
	}
	// Simple mock: return p-th value
	return p, true
}

type mockCVDResiduals struct{}

func (m *mockCVDResiduals) Residualize(cvdNorm, volNorm []float64) (residuals []float64, r2 float64, ok bool) {
	if len(cvdNorm) < 200 {
		return cvdNorm, 0.0, false
	}
	// Mock residualization: subtract mean
	residuals = make([]float64, len(cvdNorm))
	var mean float64
	for _, v := range cvdNorm {
		mean += v
	}
	mean /= float64(len(cvdNorm))

	for i, v := range cvdNorm {
		residuals[i] = v - mean
	}
	return residuals, 0.8, true
}

type mockSupplyProxy struct{}

func (m *mockSupplyProxy) Evaluate(pi ports.ProxyInputs) (gatesPassed int, requireVolumeConfirm bool) {
	gates := 0
	if pi.FundingZBelowNeg15 && pi.SpotAboveVWAP24h {
		gates++
	}
	if pi.ExchangeReserves7dDown || (pi.WhaleAccum2of3 && pi.SpotAboveVWAP24h) {
		gates++
	}
	if pi.WhaleAccum2of3 {
		gates++
	}

	requireVol := (pi.Regime == "risk_off" || pi.Regime == "btc_driven")
	if pi.VolumeFirstBarP80 {
		gates++
	}

	return gates, requireVol
}

func TestRunner_V33Dependencies(t *testing.T) {
	// Create runner with mocked v3.3 dependencies
	runner := premove.NewRunner(nil, nil, nil, nil)

	if runner.Deps == nil {
		t.Fatal("Runner should have v3.3 dependencies")
	}

	status := runner.GetEngineStatus()

	if !status["engines_initialized"].(bool) {
		t.Error("Engines should be initialized")
	}

	if !status["percentile_engine"].(bool) {
		t.Error("PercentileEngine should be initialized")
	}

	if !status["cvd_residuals"].(bool) {
		t.Error("CVDResiduals should be initialized")
	}

	if !status["supply_proxy"].(bool) {
		t.Error("SupplyProxy should be initialized")
	}
}

func TestRunner_ProcessWithEngines(t *testing.T) {
	runner := premove.NewRunner(nil, nil, nil, nil)

	// Test data processing
	rawData := map[string][]float64{
		"cvd_norm": {1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0, 11.0, 12.0, 13.0, 14.0, 15.0, 16.0, 17.0, 18.0, 19.0, 20.0, 21.0, 22.0, 23.0, 24.0, 25.0},
		"vol_norm": {2.0, 4.0, 6.0, 8.0, 10.0, 12.0, 14.0, 16.0, 18.0, 20.0, 22.0, 24.0, 26.0, 28.0, 30.0, 32.0, 34.0, 36.0, 38.0, 40.0, 42.0, 44.0, 46.0, 48.0, 50.0},
	}

	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	timestamps := make([]time.Time, 25)
	for i := range timestamps {
		timestamps[i] = baseTime.Add(time.Duration(i) * time.Hour)
	}

	ctx := context.Background()
	result, err := runner.ProcessWithEngines(ctx, rawData, timestamps)

	if err != nil {
		t.Fatalf("ProcessWithEngines failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected processing errors: %v", result.Errors)
	}
}

func TestRunner_EvaluateSupplyProxy(t *testing.T) {
	runner := premove.NewRunner(nil, nil, nil, nil)

	inputs := ports.ProxyInputs{
		FundingZBelowNeg15:     true,
		SpotAboveVWAP24h:       true,
		ExchangeReserves7dDown: false,
		WhaleAccum2of3:         true,
		VolumeFirstBarP80:      true,
		Regime:                 "risk_off",
	}

	ctx := context.Background()
	result, err := runner.EvaluateSupplyProxy(ctx, inputs)

	if err != nil {
		t.Fatalf("EvaluateSupplyProxy failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if result.GatesPassed < 1 {
		t.Error("Should pass at least one gate")
	}

	if !result.RequireVolumeConfirm {
		t.Error("risk_off regime should require volume confirmation")
	}
}
