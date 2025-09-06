package premove

import (
	"testing"
	"time"

	"cryptorun/src/application/premove"
)

func TestAlertsGovernor_RateLimiting(t *testing.T) {
	t.Run("standard_rate_limits", func(t *testing.T) {
		// This test expects an AlertsGovernor that doesn't exist yet
		governor := premove.NewAlertsGovernor(premove.AlertsConfig{
			PerHour:         3,
			PerDay:          10,
			HighVolPerHour:  6,
			BurstAllowance:  2,
		})
		
		// Simulate multiple alerts within hour limit
		for i := 0; i < 5; i++ {
			alert := premove.Alert{
				Symbol:    "BTCUSD",
				Score:     85.0,
				Timestamp: time.Now(),
				Priority:  premove.PriorityNormal,
			}
			
			allowed, reason := governor.ShouldAllow(alert)
			if i < 3 {
				if !allowed {
					t.Errorf("Alert %d should be allowed, but was blocked: %s", i+1, reason)
				}
			} else {
				if allowed {
					t.Errorf("Alert %d should be rate limited", i+1)
				}
			}
		}
	})

	t.Run("high_volatility_allowance", func(t *testing.T) {
		governor := premove.NewAlertsGovernor(premove.AlertsConfig{
			PerHour:        3,
			HighVolPerHour: 6,
		})
		
		// Set high volatility regime
		err := governor.SetVolatilityRegime(premove.VolatilityHigh)
		if err != nil {
			t.Errorf("Failed to set volatility regime: %v", err)
		}
		
		// Should allow up to 6 alerts in high vol
		for i := 0; i < 7; i++ {
			alert := premove.Alert{
				Symbol:    "ETHUSD",
				Score:     90.0,
				Timestamp: time.Now(),
				Priority:  premove.PriorityHigh,
			}
			
			allowed, _ := governor.ShouldAllow(alert)
			if i < 6 {
				if !allowed {
					t.Errorf("High vol alert %d should be allowed", i+1)
				}
			} else {
				if allowed {
					t.Errorf("High vol alert %d should be rate limited", i+1)
				}
			}
		}
	})

	t.Run("manual_override_conditions", func(t *testing.T) {
		governor := premove.NewAlertsGovernor(premove.AlertsConfig{
			PerHour: 3,
			ManualOverride: premove.ManualOverrideConfig{
				Condition: "score>90 && gates<2",
				Mode:      "alert_only",
				Duration:  30 * time.Minute,
			},
		})
		
		// Trigger manual override
		err := governor.TriggerManualOverride("Emergency market condition")
		if err != nil {
			t.Errorf("Failed to trigger manual override: %v", err)
		}
		
		// High score alert should pass even with rate limiting
		alert := premove.Alert{
			Symbol:      "BTCUSD",
			Score:       95.0,
			PassedGates: 1, // < 2 gates
			Timestamp:   time.Now(),
			Priority:    premove.PriorityCritical,
		}
		
		allowed, _ := governor.ShouldAllow(alert)
		if !allowed {
			t.Error("Manual override alert should be allowed")
		}
		
		// Check if in alert-only mode
		if !governor.IsInAlertOnlyMode() {
			t.Error("Should be in alert-only mode during manual override")
		}
	})

	t.Run("priority_based_queuing", func(t *testing.T) {
		governor := premove.NewAlertsGovernor(premove.AlertsConfig{
			PerHour:     3,
			QueueSize:   5,
			UsePriority: true,
		})
		
		// Fill up rate limit
		for i := 0; i < 3; i++ {
			alert := premove.Alert{
				Symbol:    "SOLUSD",
				Score:     80.0,
				Priority:  premove.PriorityNormal,
				Timestamp: time.Now(),
			}
			governor.ShouldAllow(alert)
		}
		
		// Add low priority alert - should be queued
		lowPriorityAlert := premove.Alert{
			Symbol:    "ADAUSD",
			Score:     70.0,
			Priority:  premove.PriorityLow,
			Timestamp: time.Now(),
		}
		
		allowed, _ := governor.ShouldAllow(lowPriorityAlert)
		if allowed {
			t.Error("Low priority alert should be queued, not immediately allowed")
		}
		
		// Add critical priority alert - should override queue
		criticalAlert := premove.Alert{
			Symbol:    "ETHUSD",
			Score:     95.0,
			Priority:  premove.PriorityCritical,
			Timestamp: time.Now(),
		}
		
		allowed, _ = governor.ShouldAllow(criticalAlert)
		if !allowed {
			t.Error("Critical priority alert should override rate limits")
		}
	})
}

func TestAlertsGovernor_OperatorFatigue(t *testing.T) {
	t.Run("fatigue_detection", func(t *testing.T) {
		// This test expects fatigue detector
		detector := premove.NewOperatorFatigueDetector(premove.FatigueConfig{
			MaxAlertsPerHour:     10,
			FatigueThreshold:     0.7,
			RecoveryPeriod:       2 * time.Hour,
			AdaptiveLimits:       true,
		})
		
		// Simulate rapid alerts
		for i := 0; i < 15; i++ {
			alert := premove.Alert{
				Symbol:    "BTCUSD",
				Timestamp: time.Now().Add(-time.Duration(i) * time.Minute),
				Score:     80.0 + float64(i),
			}
			
			detector.RecordAlert(alert)
		}
		
		fatigueLevel := detector.GetFatigueLevel()
		if fatigueLevel < 0.7 {
			t.Errorf("Expected high fatigue level, got %.2f", fatigueLevel)
		}
		
		// Should recommend reduced alert frequency
		recommended := detector.GetRecommendedFrequency()
		if recommended.AlertsPerHour > 5 {
			t.Errorf("Expected reduced frequency recommendation, got %d/hour", recommended.AlertsPerHour)
		}
	})

	t.Run("adaptive_throttling", func(t *testing.T) {
		throttler := premove.NewAdaptiveThrottler(premove.ThrottlingConfig{
			BaseInterval:     30 * time.Second,
			MaxInterval:      10 * time.Minute,
			BackoffFactor:    1.5,
			SuccessDecayRate: 0.9,
		})
		
		// Simulate ignored alerts (increase throttling)
		for i := 0; i < 5; i++ {
			alert := premove.Alert{
				Symbol: "ETHUSD",
				Score:  75.0,
			}
			
			throttler.RecordIgnoredAlert(alert)
		}
		
		interval := throttler.GetCurrentInterval()
		if interval <= 30*time.Second {
			t.Error("Interval should increase after ignored alerts")
		}
		
		// Simulate acted-upon alert (decrease throttling)
		throttler.RecordActionTaken(premove.Alert{Symbol: "BTCUSD", Score: 90.0})
		
		newInterval := throttler.GetCurrentInterval()
		if newInterval >= interval {
			t.Error("Interval should decrease after successful action")
		}
	})

	t.Run("context_awareness", func(t *testing.T) {
		contextManager := premove.NewAlertContextManager()
		
		// Set market context
		context := premove.MarketContext{
			VolatilityRegime: premove.VolatilityHigh,
			TradingSession:   premove.SessionAsia,
			MarketSentiment:  premove.SentimentFearful,
			MajorEvents: []premove.Event{
				{Type: "fed_announcement", Impact: premove.ImpactHigh},
			},
		}
		
		contextManager.UpdateContext(context)
		
		// High volatility + fearful sentiment should affect alert thresholds
		adjustment := contextManager.GetThresholdAdjustment()
		if adjustment.ScoreThreshold <= 0 {
			t.Error("Expected score threshold adjustment in high vol/fearful context")
		}
		
		if adjustment.RateLimitMultiplier <= 1.0 {
			t.Error("Expected rate limit relaxation in volatile context")
		}
	})
}

func TestAlertsGovernor_DeliveryChannels(t *testing.T) {
	t.Run("multi_channel_routing", func(t *testing.T) {
		router := premove.NewAlertRouter(premove.RouterConfig{
			Channels: []premove.DeliveryChannel{
				{Name: "console", Priority: 1, MaxThroughput: 10},
				{Name: "webhook", Priority: 2, MaxThroughput: 5},
				{Name: "email", Priority: 3, MaxThroughput: 2},
			},
		})
		
		alert := premove.Alert{
			Symbol:   "BTCUSD",
			Score:    90.0,
			Priority: premove.PriorityHigh,
		}
		
		routes, err := router.RouteAlert(alert)
		if err != nil {
			t.Errorf("Alert routing failed: %v", err)
		}
		
		if len(routes) == 0 {
			t.Error("Expected at least one delivery route")
		}
		
		// High priority should use multiple channels
		channelNames := make([]string, len(routes))
		for i, route := range routes {
			channelNames[i] = route.ChannelName
		}
		
		expectedChannels := []string{"console", "webhook"}
		for _, expected := range expectedChannels {
			found := false
			for _, actual := range channelNames {
				if actual == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected channel %s not found in routes", expected)
			}
		}
	})

	t.Run("delivery_confirmation", func(t *testing.T) {
		tracker := premove.NewDeliveryTracker()
		
		alert := premove.Alert{
			ID:       "test-001",
			Symbol:   "ETHUSD",
			Score:    85.0,
			Priority: premove.PriorityNormal,
		}
		
		// Track delivery attempt
		deliveryID := tracker.TrackDelivery(alert, "webhook")
		
		// Simulate delivery confirmation
		err := tracker.ConfirmDelivery(deliveryID, premove.DeliveryResult{
			Success:      true,
			ResponseTime: 150 * time.Millisecond,
			StatusCode:   200,
		})
		
		if err != nil {
			t.Errorf("Delivery confirmation failed: %v", err)
		}
		
		// Check delivery status
		status := tracker.GetDeliveryStatus(deliveryID)
		if status.Status != premove.DeliveryConfirmed {
			t.Errorf("Expected confirmed delivery, got %v", status.Status)
		}
		
		// Check metrics
		metrics := tracker.GetDeliveryMetrics("webhook")
		if metrics.SuccessRate <= 0 {
			t.Errorf("Expected positive success rate, got %.2f", metrics.SuccessRate)
		}
	})

	t.Run("circuit_breaker_integration", func(t *testing.T) {
		breaker := premove.NewAlertCircuitBreaker(premove.CircuitBreakerConfig{
			FailureThreshold:  5,
			RecoveryTimeout:   60 * time.Second,
			HalfOpenRequests: 3,
		})
		
		// Simulate delivery failures
		for i := 0; i < 6; i++ {
			alert := premove.Alert{Symbol: "SOLUSD", Score: 80.0}
			result := premove.DeliveryResult{
				Success: false,
				Error:   "webhook timeout",
			}
			
			breaker.RecordResult("webhook", result)
		}
		
		// Circuit should be open
		if !breaker.IsOpen("webhook") {
			t.Error("Circuit breaker should be open after failures")
		}
		
		// Alert delivery should be blocked
		alert := premove.Alert{Symbol: "ADAUSD", Score: 85.0}
		allowed := breaker.AllowDelivery("webhook", alert)
		if allowed {
			t.Error("Delivery should be blocked when circuit is open")
		}
		
		// Wait for recovery timeout (simulate)
		breaker.ForceHalfOpen("webhook") // For testing
		
		if !breaker.IsHalfOpen("webhook") {
			t.Error("Circuit breaker should be half-open after timeout")
		}
	})
}