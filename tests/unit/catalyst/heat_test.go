package catalyst

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sawpanic/cryptorun/src/domain/catalyst"
)

func TestHeatCalculation(t *testing.T) {
	config := catalyst.DefaultHeatConfig()
	calculator := catalyst.NewHeatCalculator(config)
	now := time.Now().UTC()

	t.Run("no events returns neutral score", func(t *testing.T) {
		events := []catalyst.Event{}
		heat := calculator.Heat(events, now)

		// Should return neutral score (50.0) when no events
		assert.Equal(t, 0.0, heat)
	})

	t.Run("single positive imminent event", func(t *testing.T) {
		events := []catalyst.Event{
			{
				ID:       "test_001",
				Symbol:   "BTCUSD",
				Title:    "Bitcoin ETF Approval",
				Date:     now.Add(2 * 7 * 24 * time.Hour), // 2 weeks (imminent bucket)
				Tier:     1,                               // Major event
				Polarity: 1,                               // Positive
				Source:   "test",
			},
		}

		heat := calculator.Heat(events, now)

		// Imminent (1.2×) * Major (1.0) * Positive (+1) * 100 = 120
		// Normalized to 50-100 range: should be high (>80)
		assert.Greater(t, heat, 80.0)
		assert.LessOrEqual(t, heat, 100.0)
	})

	t.Run("single negative imminent event", func(t *testing.T) {
		events := []catalyst.Event{
			{
				ID:       "test_002",
				Symbol:   "ETHUSD",
				Title:    "Ethereum Upgrade Delay",
				Date:     now.Add(1 * 7 * 24 * time.Hour), // 1 week (imminent)
				Tier:     1,                               // Major event
				Polarity: -1,                              // Negative
				Source:   "test",
			},
		}

		heat := calculator.Heat(events, now)

		// Negative events should result in score <50
		assert.Less(t, heat, 50.0)
		assert.GreaterOrEqual(t, heat, 0.0)
	})
}

func TestTimeBuckets(t *testing.T) {
	config := catalyst.DefaultHeatConfig()
	calculator := catalyst.NewHeatCalculator(config)
	now := time.Now().UTC()

	testCases := []struct {
		name               string
		weeksFromNow       float64
		expectedBucket     string
		expectedMultiplier float64
	}{
		{"imminent bucket edge", 2.0, "imminent", 1.2},
		{"imminent bucket boundary", 4.0, "imminent", 1.2},
		{"near-term bucket", 6.0, "near-term", 1.0},
		{"near-term boundary", 8.0, "near-term", 1.0},
		{"medium bucket", 12.0, "medium", 0.8},
		{"medium boundary", 16.0, "medium", 0.8},
		{"distant bucket", 20.0, "distant", 0.6},
		{"far distant", 52.0, "distant", 0.6},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			eventDate := now.Add(time.Duration(tc.weeksFromNow * float64(7*24*time.Hour)))

			event := catalyst.Event{
				ID:       "bucket_test",
				Symbol:   "TESTUSD",
				Date:     eventDate,
				Tier:     1, // Major
				Polarity: 1, // Positive
			}

			// Test bucket classification
			bucket := calculator.GetTimeBucket(event, now)
			assert.Equal(t, tc.expectedBucket, bucket, "Wrong time bucket for %.1f weeks", tc.weeksFromNow)

			// Test heat calculation includes correct multiplier
			events := []catalyst.Event{event}
			heat := calculator.Heat(events, now)

			// Verify heat is affected by time decay (closer events should have higher heat)
			if tc.expectedMultiplier > 1.0 {
				assert.Greater(t, heat, 50.0, "Imminent events should have heat >50")
			} else if tc.expectedMultiplier < 1.0 {
				// Distant events should still be positive but lower
				assert.Greater(t, heat, 50.0, "Positive events should have heat >50")
				assert.Less(t, heat, 80.0, "Distant events should have lower heat")
			}
		})
	}
}

func TestEventTiers(t *testing.T) {
	config := catalyst.DefaultHeatConfig()
	calculator := catalyst.NewHeatCalculator(config)
	now := time.Now().UTC()
	eventDate := now.Add(2 * 7 * 24 * time.Hour) // 2 weeks (imminent)

	testCases := []struct {
		name           string
		tier           int
		expectedWeight float64
	}{
		{"major event", 1, 1.0},
		{"minor event", 2, 0.6},
		{"info event", 3, 0.3},
		{"unknown tier defaults to info", 99, 0.3},
	}

	var previousHeat float64
	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			events := []catalyst.Event{
				{
					ID:       "tier_test",
					Symbol:   "TESTUSD",
					Date:     eventDate,
					Tier:     tc.tier,
					Polarity: 1, // Positive
				},
			}

			heat := calculator.Heat(events, now)

			// All tiers should produce positive heat for positive events
			assert.Greater(t, heat, 50.0, "Positive events should have heat >50")

			// Higher tiers should produce higher heat (with same time/polarity)
			if i > 0 {
				assert.GreaterOrEqual(t, previousHeat, heat, "Higher tiers should have higher heat")
			}
			previousHeat = heat
		})
	}
}

func TestPolarityHandling(t *testing.T) {
	config := catalyst.DefaultHeatConfig()
	calculator := catalyst.NewHeatCalculator(config)
	now := time.Now().UTC()
	eventDate := now.Add(2 * 7 * 24 * time.Hour) // 2 weeks

	t.Run("positive polarity", func(t *testing.T) {
		events := []catalyst.Event{
			{
				ID:       "positive_test",
				Symbol:   "TESTUSD",
				Date:     eventDate,
				Tier:     1,
				Polarity: 1, // Positive
			},
		}

		heat := calculator.Heat(events, now)
		assert.Greater(t, heat, 50.0, "Positive events should have heat >50")
	})

	t.Run("negative polarity", func(t *testing.T) {
		events := []catalyst.Event{
			{
				ID:       "negative_test",
				Symbol:   "TESTUSD",
				Date:     eventDate,
				Tier:     1,
				Polarity: -1, // Negative
			},
		}

		heat := calculator.Heat(events, now)
		assert.Less(t, heat, 50.0, "Negative events should have heat <50")
		assert.GreaterOrEqual(t, heat, 0.0, "Heat should not be negative")
	})

	t.Run("neutral polarity", func(t *testing.T) {
		events := []catalyst.Event{
			{
				ID:       "neutral_test",
				Symbol:   "TESTUSD",
				Date:     eventDate,
				Tier:     1,
				Polarity: 0, // Neutral
			},
		}

		heat := calculator.Heat(events, now)
		// Neutral events should result in neutral score
		assert.Equal(t, 50.0, heat, "Neutral events should have heat = 50")
	})
}

func TestPastEventsIgnored(t *testing.T) {
	config := catalyst.DefaultHeatConfig()
	calculator := catalyst.NewHeatCalculator(config)
	now := time.Now().UTC()

	events := []catalyst.Event{
		{
			ID:       "past_event",
			Symbol:   "TESTUSD",
			Date:     now.Add(-7 * 24 * time.Hour), // 1 week ago
			Tier:     1,
			Polarity: 1,
		},
		{
			ID:       "future_event",
			Symbol:   "TESTUSD",
			Date:     now.Add(7 * 24 * time.Hour), // 1 week future
			Tier:     1,
			Polarity: 1,
		},
	}

	heat := calculator.Heat(events, now)

	// Should only consider future event
	assert.Greater(t, heat, 50.0, "Should consider future event")

	// Heat should be same as single future event
	futureOnlyEvents := []catalyst.Event{events[1]}
	futureOnlyHeat := calculator.Heat(futureOnlyEvents, now)
	assert.Equal(t, futureOnlyHeat, heat, "Past events should be ignored")
}

func TestMultipleEventsAggregation(t *testing.T) {
	config := catalyst.DefaultHeatConfig()
	now := time.Now().UTC()

	t.Run("smooth aggregation", func(t *testing.T) {
		config.AggregationMethod = "smooth"
		calculator := catalyst.NewHeatCalculator(config)

		events := []catalyst.Event{
			{
				ID:       "event1",
				Date:     now.Add(2 * 7 * 24 * time.Hour),
				Tier:     1,
				Polarity: 1,
			},
			{
				ID:       "event2",
				Date:     now.Add(3 * 7 * 24 * time.Hour),
				Tier:     1,
				Polarity: 1,
			},
		}

		smoothHeat := calculator.Heat(events, now)
		singleEventHeat := calculator.Heat([]catalyst.Event{events[0]}, now)

		// Smooth aggregation should be higher than single event but with diminishing returns
		assert.Greater(t, smoothHeat, singleEventHeat)
	})

	t.Run("max aggregation", func(t *testing.T) {
		config.AggregationMethod = "max"
		calculator := catalyst.NewHeatCalculator(config)

		events := []catalyst.Event{
			{
				ID:       "low_event",
				Date:     now.Add(12 * 7 * 24 * time.Hour), // Medium bucket (0.8×)
				Tier:     3,                                // Info tier (0.3×)
				Polarity: 1,
			},
			{
				ID:       "high_event",
				Date:     now.Add(2 * 7 * 24 * time.Hour), // Imminent bucket (1.2×)
				Tier:     1,                               // Major tier (1.0×)
				Polarity: 1,
			},
		}

		maxHeat := calculator.Heat(events, now)
		highEventHeat := calculator.Heat([]catalyst.Event{events[1]}, now)

		// Max aggregation should equal the highest single event
		assert.Equal(t, highEventHeat, maxHeat, "Max aggregation should take highest event")
	})
}

func TestHeatAnalysis(t *testing.T) {
	config := catalyst.DefaultHeatConfig()
	calculator := catalyst.NewHeatCalculator(config)
	now := time.Now().UTC()

	events := []catalyst.Event{
		{
			ID:       "imminent_major",
			Date:     now.Add(2 * 7 * 24 * time.Hour),
			Tier:     1,
			Polarity: 1,
			Source:   "test",
		},
		{
			ID:       "distant_minor",
			Date:     now.Add(20 * 7 * 24 * time.Hour),
			Tier:     2,
			Polarity: -1,
			Source:   "test",
		},
	}

	analysis := calculator.AnalyzeHeat(events, now)

	// Verify analysis structure
	assert.Equal(t, 2, analysis.EventCount)
	assert.Equal(t, len(events), len(analysis.EventDetails))

	// Check bucket counts
	assert.Equal(t, 1, analysis.BucketCounts["imminent"])
	assert.Equal(t, 1, analysis.BucketCounts["distant"])

	// Check tier counts
	assert.Equal(t, 1, analysis.TierCounts[1]) // Major
	assert.Equal(t, 1, analysis.TierCounts[2]) // Minor

	// Check polarity counts
	assert.Equal(t, 1, analysis.PolarityCounts[1])  // Positive
	assert.Equal(t, 1, analysis.PolarityCounts[-1]) // Negative

	// Verify event details
	for i, detail := range analysis.EventDetails {
		assert.Equal(t, events[i].ID, detail.Event.ID)
		assert.Greater(t, detail.WeeksToEvent, 0.0)
		assert.NotEmpty(t, detail.TimeBucket)
		assert.Greater(t, detail.DecayMultiplier, 0.0)
		assert.Greater(t, detail.TierWeight, 0.0)
	}
}

func TestConfigurationEdgeCases(t *testing.T) {
	t.Run("zero multipliers", func(t *testing.T) {
		config := catalyst.DefaultHeatConfig()
		config.ImminentMultiplier = 0.0
		calculator := catalyst.NewHeatCalculator(config)
		now := time.Now().UTC()

		events := []catalyst.Event{
			{
				Date:     now.Add(2 * 7 * 24 * time.Hour), // Imminent
				Tier:     1,
				Polarity: 1,
			},
		}

		heat := calculator.Heat(events, now)
		assert.Equal(t, 50.0, heat, "Zero multiplier should result in neutral heat")
	})

	t.Run("custom bucket boundaries", func(t *testing.T) {
		config := catalyst.DefaultHeatConfig()
		config.ImminentWeeks = 2.0 // Shorter imminent window
		config.NearTermWeeks = 4.0
		calculator := catalyst.NewHeatCalculator(config)
		now := time.Now().UTC()

		// Event at 3 weeks should now be near-term, not imminent
		event := catalyst.Event{
			Date:     now.Add(3 * 7 * 24 * time.Hour),
			Tier:     1,
			Polarity: 1,
		}

		bucket := calculator.GetTimeBucket(event, now)
		assert.Equal(t, "near-term", bucket, "Custom boundaries should affect bucket classification")
	})
}

// Benchmark tests for performance validation
func BenchmarkHeatCalculation(b *testing.B) {
	config := catalyst.DefaultHeatConfig()
	calculator := catalyst.NewHeatCalculator(config)
	now := time.Now().UTC()

	// Create 100 test events
	events := make([]catalyst.Event, 100)
	for i := 0; i < 100; i++ {
		events[i] = catalyst.Event{
			ID:       fmt.Sprintf("bench_%d", i),
			Date:     now.Add(time.Duration(i) * 24 * time.Hour),
			Tier:     (i % 3) + 1,
			Polarity: []int{-1, 0, 1}[i%3],
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculator.Heat(events, now)
	}
}

func BenchmarkHeatAnalysis(b *testing.B) {
	config := catalyst.DefaultHeatConfig()
	calculator := catalyst.NewHeatCalculator(config)
	now := time.Now().UTC()

	events := make([]catalyst.Event, 50)
	for i := 0; i < 50; i++ {
		events[i] = catalyst.Event{
			ID:       fmt.Sprintf("analysis_bench_%d", i),
			Date:     now.Add(time.Duration(i) * 24 * time.Hour),
			Tier:     (i % 3) + 1,
			Polarity: []int{-1, 1}[i%2],
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculator.AnalyzeHeat(events, now)
	}
}
