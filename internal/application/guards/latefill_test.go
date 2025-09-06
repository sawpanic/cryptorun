package guards

import (
	"testing"
	"time"

	"cryptorun/internal/telemetry/latency"
)

// MockClock provides controllable time for testing
type MockClock struct {
	currentTime time.Time
}

// NewMockClock creates a mock clock starting at the specified time
func NewMockClock(startTime time.Time) *MockClock {
	return &MockClock{currentTime: startTime}
}

// Now returns the current mock time
func (m *MockClock) Now() time.Time {
	return m.currentTime
}

// Advance moves the mock clock forward by the specified duration
func (m *MockClock) Advance(d time.Duration) {
	m.currentTime = m.currentTime.Add(d)
}

// TestLateFillGuardBasicThreshold tests basic threshold behavior
func TestLateFillGuardBasicThreshold(t *testing.T) {
	guard := NewLateFillGuard(30000, 400, 30000) // 30s base, 400ms p99, 30s grace

	baseTime := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name            string
		delayMs         float64
		expectedAllowed bool
		expectedReason  string
	}{
		{
			name:            "WithinBaseThreshold",
			delayMs:         25000, // 25s
			expectedAllowed: true,
			expectedReason:  "within base threshold: 25000.0ms ≤ 30000.0ms",
		},
		{
			name:            "ExceedsBaseThreshold",
			delayMs:         35000, // 35s
			expectedAllowed: false,
			expectedReason:  "late fill: 35000.0ms > 30000.0ms base threshold (p99 0.0ms ≤ 400.0ms threshold)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset latency history to ensure clean p99
			latency.Record(latency.StageOrder, 0)

			input := LateFillInput{
				Symbol:        "BTCUSD",
				SignalTime:    baseTime,
				ExecutionTime: baseTime.Add(time.Duration(tc.delayMs) * time.Millisecond),
				FreshnessAge:  1, // Within freshness limits
				ATRDistance:   1.0,
				ATRCurrent:    100.0,
			}

			result := guard.Evaluate(input)

			if result.Allowed != tc.expectedAllowed {
				t.Errorf("Expected allowed=%v, got %v", tc.expectedAllowed, result.Allowed)
			}

			if result.Reason != tc.expectedReason {
				t.Errorf("Expected reason=%q, got %q", tc.expectedReason, result.Reason)
			}
		})
	}
}

// TestLateFillGuardFreshnessViolations tests freshness constraint enforcement
func TestLateFillGuardFreshnessViolations(t *testing.T) {
	guard := DefaultLateFillGuard()

	baseTime := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name            string
		freshnessAge    int
		atrDistance     float64
		atrCurrent      float64
		expectedAllowed bool
		expectedReason  string
	}{
		{
			name:            "BarAgeTooHigh",
			freshnessAge:    3,
			atrDistance:     1.0,
			atrCurrent:      100.0,
			expectedAllowed: false,
			expectedReason:  "freshness violation: bar age 3 > 2 bars maximum",
		},
		{
			name:            "ATRDistanceTooHigh",
			freshnessAge:    2,
			atrDistance:     1.5,
			atrCurrent:      100.0,
			expectedAllowed: false,
			expectedReason:  "freshness violation: price distance 1.50×ATR > 1.2×ATR maximum",
		},
		{
			name:            "WithinFreshnessLimits",
			freshnessAge:    2,
			atrDistance:     1.1,
			atrCurrent:      100.0,
			expectedAllowed: true,
			expectedReason:  "within base threshold: 25000.0ms ≤ 30000.0ms",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := LateFillInput{
				Symbol:        "TESTCOIN",
				SignalTime:    baseTime,
				ExecutionTime: baseTime.Add(25 * time.Second), // Within base threshold
				FreshnessAge:  tc.freshnessAge,
				ATRDistance:   tc.atrDistance,
				ATRCurrent:    tc.atrCurrent,
			}

			result := guard.Evaluate(input)

			if result.Allowed != tc.expectedAllowed {
				t.Errorf("Expected allowed=%v, got %v", tc.expectedAllowed, result.Allowed)
			}

			if result.Reason != tc.expectedReason {
				t.Errorf("Expected reason=%q, got %q", tc.expectedReason, result.Reason)
			}
		})
	}
}

// TestLateFillGuardP99Relaxation tests p99 threshold crossing and grace window
func TestLateFillGuardP99Relaxation(t *testing.T) {
	guard := NewLateFillGuard(30000, 400, 30000) // 30s base, 400ms p99, 30s grace

	baseTime := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	// Simulate high p99 latency by recording multiple high latencies
	for i := 0; i < 100; i++ {
		latency.Record(latency.StageOrder, 500*time.Millisecond) // 500ms > 400ms threshold
	}

	// Verify p99 is above threshold
	currentP99 := latency.GetP99(latency.StageOrder)
	if currentP99 <= 400 {
		t.Fatalf("Expected p99 > 400ms for test setup, got %.1fms", currentP99)
	}

	input := LateFillInput{
		Symbol:        "ETHUSD",
		SignalTime:    baseTime,
		ExecutionTime: baseTime.Add(45 * time.Second), // 45s > 30s base, but ≤ 60s (base + grace)
		FreshnessAge:  1,
		ATRDistance:   1.0,
		ATRCurrent:    100.0,
	}

	result := guard.Evaluate(input)

	if !result.Allowed {
		t.Errorf("Expected relaxation to allow execution, but was blocked: %s", result.Reason)
	}

	if !result.RelaxUsed {
		t.Error("Expected RelaxUsed=true when p99 relaxation applied")
	}

	// Check golden reason format
	expectedRelaxPattern := "latefill_relax[p99_exceeded:"
	if !containsSubstring(result.RelaxReason, expectedRelaxPattern) {
		t.Errorf("Expected relax reason to contain %q, got %q", expectedRelaxPattern, result.RelaxReason)
	}

	// Verify cooldown tracking
	if result.NextRelaxTime.IsZero() {
		t.Error("Expected NextRelaxTime to be set after relax usage")
	}
}

// TestLateFillGuardSingleFire tests single-fire semantics within 30m window
func TestLateFillGuardSingleFire(t *testing.T) {
	guard := NewLateFillGuard(30000, 400, 30000)

	// Setup high p99
	for i := 0; i < 100; i++ {
		latency.Record(latency.StageOrder, 500*time.Millisecond)
	}

	baseTime := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	// First relaxation should succeed
	input1 := LateFillInput{
		Symbol:        "SOLUSD",
		SignalTime:    baseTime,
		ExecutionTime: baseTime.Add(45 * time.Second),
		FreshnessAge:  1,
		ATRDistance:   1.0,
		ATRCurrent:    100.0,
	}

	result1 := guard.Evaluate(input1)

	if !result1.Allowed || !result1.RelaxUsed {
		t.Fatal("First relaxation should succeed")
	}

	// Second relaxation within 30m should fail
	input2 := LateFillInput{
		Symbol:        "SOLUSD",                       // Same symbol
		SignalTime:    baseTime.Add(10 * time.Minute), // 10 minutes later
		ExecutionTime: baseTime.Add(10*time.Minute + 45*time.Second),
		FreshnessAge:  1,
		ATRDistance:   1.0,
		ATRCurrent:    100.0,
	}

	result2 := guard.Evaluate(input2)

	if result2.Allowed {
		t.Error("Second relaxation within cooldown should be blocked")
	}

	if result2.RelaxUsed {
		t.Error("RelaxUsed should be false when on cooldown")
	}

	// Check cooldown reason format
	expectedCooldownPattern := "p99 relax on cooldown until"
	if !containsSubstring(result2.Reason, expectedCooldownPattern) {
		t.Errorf("Expected cooldown reason to contain %q, got %q", expectedCooldownPattern, result2.Reason)
	}
}

// TestLateFillGuardCooldownExpiry tests that relaxation becomes available after 30m
func TestLateFillGuardCooldownExpiry(t *testing.T) {
	guard := NewLateFillGuard(30000, 400, 30000)

	// Setup high p99
	for i := 0; i < 100; i++ {
		latency.Record(latency.StageOrder, 500*time.Millisecond)
	}

	baseTime := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	// Use first relaxation
	guard.markRelaxUsed("ADAUSD")

	// Test after 29 minutes (should still be blocked)
	mockTime := baseTime.Add(29 * time.Minute)
	if guard.canRelax("ADAUSD") {
		t.Error("Relaxation should not be available after 29 minutes")
	}

	// Test after 31 minutes (should be available)
	mockTime = baseTime.Add(31 * time.Minute)
	// Simulate time passage by directly updating the tracker
	guard.relaxMutex.Lock()
	guard.relaxTracker["ADAUSD"] = baseTime.Add(-31 * time.Minute)
	guard.relaxMutex.Unlock()

	if !guard.canRelax("ADAUSD") {
		t.Error("Relaxation should be available after 31 minutes")
	}
}

// TestLateFillGuardExcessiveDelayWithGrace tests that even with grace, excessive delays are blocked
func TestLateFillGuardExcessiveDelayWithGrace(t *testing.T) {
	guard := NewLateFillGuard(30000, 400, 30000)

	// Setup high p99
	for i := 0; i < 100; i++ {
		latency.Record(latency.StageOrder, 500*time.Millisecond)
	}

	baseTime := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	input := LateFillInput{
		Symbol:        "DOGEUSD",
		SignalTime:    baseTime,
		ExecutionTime: baseTime.Add(65 * time.Second), // 65s > 60s (base + grace)
		FreshnessAge:  1,
		ATRDistance:   1.0,
		ATRCurrent:    100.0,
	}

	result := guard.Evaluate(input)

	if result.Allowed {
		t.Error("Excessive delay should be blocked even with grace window")
	}

	if result.RelaxUsed {
		t.Error("RelaxUsed should be false when delay exceeds grace window")
	}

	expectedReasonPattern := "excessive delay even with p99 grace"
	if !containsSubstring(result.Reason, expectedReasonPattern) {
		t.Errorf("Expected reason to contain %q, got %q", expectedReasonPattern, result.Reason)
	}
}

// TestLateFillGuardGoldenReasons tests that reason strings match expected golden patterns
func TestLateFillGuardGoldenReasons(t *testing.T) {
	guard := NewLateFillGuard(30000, 400, 30000)

	baseTime := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	goldenTests := []struct {
		name           string
		setupP99       bool
		delaySeconds   int
		freshnessAge   int
		atrDistance    float64
		expectedReason string
		expectedRelax  string
	}{
		{
			name:           "WithinThreshold",
			setupP99:       false,
			delaySeconds:   25,
			freshnessAge:   1,
			atrDistance:    1.0,
			expectedReason: "within base threshold: 25000.0ms ≤ 30000.0ms",
			expectedRelax:  "",
		},
		{
			name:           "P99RelaxApplied",
			setupP99:       true,
			delaySeconds:   45,
			freshnessAge:   1,
			atrDistance:    1.0,
			expectedReason: "p99 relaxation applied: 45000.0ms ≤ 60000.0ms (base + grace)",
			expectedRelax:  "latefill_relax[p99_exceeded:",
		},
		{
			name:           "FreshnessViolation",
			setupP99:       false,
			delaySeconds:   25,
			freshnessAge:   3,
			atrDistance:    1.0,
			expectedReason: "freshness violation: bar age 3 > 2 bars maximum",
			expectedRelax:  "",
		},
		{
			name:           "ATRViolation",
			setupP99:       false,
			delaySeconds:   25,
			freshnessAge:   1,
			atrDistance:    1.5,
			expectedReason: "freshness violation: price distance 1.50×ATR > 1.2×ATR maximum",
			expectedRelax:  "",
		},
	}

	for _, gt := range goldenTests {
		t.Run(gt.name, func(t *testing.T) {
			// Reset guard state
			guard.Reset()

			// Setup p99 if needed
			if gt.setupP99 {
				for i := 0; i < 100; i++ {
					latency.Record(latency.StageOrder, 500*time.Millisecond)
				}
			} else {
				for i := 0; i < 100; i++ {
					latency.Record(latency.StageOrder, 100*time.Millisecond)
				}
			}

			input := LateFillInput{
				Symbol:        "TESTCOIN",
				SignalTime:    baseTime,
				ExecutionTime: baseTime.Add(time.Duration(gt.delaySeconds) * time.Second),
				FreshnessAge:  gt.freshnessAge,
				ATRDistance:   gt.atrDistance,
				ATRCurrent:    100.0,
			}

			result := guard.Evaluate(input)

			if result.Reason != gt.expectedReason {
				t.Errorf("Golden reason mismatch:\nExpected: %q\nActual:   %q", gt.expectedReason, result.Reason)
			}

			if gt.expectedRelax != "" {
				if !containsSubstring(result.RelaxReason, gt.expectedRelax) {
					t.Errorf("Expected relax reason to contain %q, got %q", gt.expectedRelax, result.RelaxReason)
				}
			} else if result.RelaxReason != "" {
				t.Errorf("Expected no relax reason, got %q", result.RelaxReason)
			}
		})
	}
}

// TestLateFillGuardMetrics tests metrics collection and reporting
func TestLateFillGuardMetrics(t *testing.T) {
	guard := NewLateFillGuard(30000, 400, 30000)

	metrics := guard.GetMetrics()

	if metrics.CurrentP99Ms < 0 {
		t.Error("Expected non-negative p99 metric")
	}

	if len(metrics.RelaxAvailability) != 0 {
		t.Error("Expected empty relax availability for new guard")
	}

	// Use a relaxation and check metrics update
	guard.markRelaxUsed("TESTCOIN")

	metrics = guard.GetMetrics()

	if len(metrics.ActiveRelaxSymbols) != 1 {
		t.Errorf("Expected 1 active relax symbol, got %d", len(metrics.ActiveRelaxSymbols))
	}

	if metrics.ActiveRelaxSymbols[0] != "TESTCOIN" {
		t.Errorf("Expected TESTCOIN in active relaxes, got %s", metrics.ActiveRelaxSymbols[0])
	}
}

// Helper function for substring checking
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s[:len(substr)] == substr ||
		(len(s) > len(substr) && containsSubstring(s[1:], substr)))
}
