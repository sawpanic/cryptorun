package regime

import (
	"testing"
)

// MockSocialFactor represents a social scoring component
type MockSocialFactor struct {
	Score      float64
	Source     string
	Confidence float64
}

// MockCompositeScorer simulates the composite scoring system
type MockCompositeScorer struct {
	BaseScore     float64 // Score from weighted factors (0-100)
	SocialFactors []MockSocialFactor
}

func (m *MockCompositeScorer) ApplySocialCap() float64 {
	// Calculate total social contribution
	socialTotal := 0.0
	for _, factor := range m.SocialFactors {
		socialTotal += factor.Score * factor.Confidence
	}

	// Apply hard cap of +10 points
	const SOCIAL_CAP = 10.0
	if socialTotal > SOCIAL_CAP {
		socialTotal = SOCIAL_CAP
	}

	// Social is applied OUTSIDE the base 100% allocation
	return m.BaseScore + socialTotal
}

func TestSocialFactor_HardCap10(t *testing.T) {
	tests := []struct {
		name           string
		baseScore      float64
		socialFactors  []MockSocialFactor
		expectedMax    float64
		expectsCapping bool
	}{
		{
			name:           "no social factors",
			baseScore:      75.0,
			socialFactors:  []MockSocialFactor{},
			expectedMax:    75.0,
			expectsCapping: false,
		},
		{
			name:      "social under cap",
			baseScore: 75.0,
			socialFactors: []MockSocialFactor{
				{Score: 5.0, Confidence: 1.0, Source: "reddit_sentiment"},
				{Score: 3.0, Confidence: 0.8, Source: "twitter_mentions"},
			},
			expectedMax:    82.4, // 75 + 5 + 2.4
			expectsCapping: false,
		},
		{
			name:      "social at cap exactly",
			baseScore: 80.0,
			socialFactors: []MockSocialFactor{
				{Score: 10.0, Confidence: 1.0, Source: "reddit_sentiment"},
			},
			expectedMax:    90.0, // 80 + 10 (capped)
			expectsCapping: false,
		},
		{
			name:      "social exceeds cap - should be capped",
			baseScore: 70.0,
			socialFactors: []MockSocialFactor{
				{Score: 8.0, Confidence: 1.0, Source: "reddit_sentiment"},
				{Score: 6.0, Confidence: 1.0, Source: "twitter_mentions"},
				{Score: 4.0, Confidence: 1.0, Source: "social_volume"},
			},
			expectedMax:    80.0, // 70 + 10 (capped at +10)
			expectsCapping: true,
		},
		{
			name:      "extreme social excess - hard capped",
			baseScore: 85.0,
			socialFactors: []MockSocialFactor{
				{Score: 25.0, Confidence: 1.0, Source: "viral_reddit"},
				{Score: 20.0, Confidence: 1.0, Source: "twitter_trending"},
				{Score: 15.0, Confidence: 1.0, Source: "social_explosion"},
			},
			expectedMax:    95.0, // 85 + 10 (hard capped)
			expectsCapping: true,
		},
		{
			name:      "partial confidence factors",
			baseScore: 78.0,
			socialFactors: []MockSocialFactor{
				{Score: 12.0, Confidence: 0.5, Source: "weak_signal"},    // 6.0 effective
				{Score: 8.0, Confidence: 0.7, Source: "moderate_signal"}, // 5.6 effective
			},
			expectedMax:    89.6, // 78 + 6.0 + 5.6 (under cap)
			expectsCapping: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scorer := &MockCompositeScorer{
				BaseScore:     tt.baseScore,
				SocialFactors: tt.socialFactors,
			}

			finalScore := scorer.ApplySocialCap()

			// Verify final score matches expected
			if finalScore != tt.expectedMax {
				t.Errorf("expected final score %.1f, got %.1f", tt.expectedMax, finalScore)
			}

			// Verify social contribution never exceeds +10
			socialContrib := finalScore - tt.baseScore
			if socialContrib > 10.001 { // Allow small floating point error
				t.Errorf("social contribution %.3f exceeds +10 cap", socialContrib)
			}

			// Verify capping expectation
			rawSocialTotal := 0.0
			for _, factor := range tt.socialFactors {
				rawSocialTotal += factor.Score * factor.Confidence
			}

			actualCapping := rawSocialTotal > 10.0
			if actualCapping != tt.expectsCapping {
				t.Errorf("expected capping=%v, got capping=%v (raw social=%.1f)",
					tt.expectsCapping, actualCapping, rawSocialTotal)
			}
		})
	}
}

func TestSocialFactor_AppliedOutsideBase100(t *testing.T) {
	// Verify that social factors are applied OUTSIDE the base 100% weight allocation

	baseWeightedScore := 85.0 // This represents the 100% weighted base factors
	socialContribution := 7.5 // Social adds on top

	scorer := &MockCompositeScorer{
		BaseScore: baseWeightedScore,
		SocialFactors: []MockSocialFactor{
			{Score: 7.5, Confidence: 1.0, Source: "reddit_sentiment"},
		},
	}

	finalScore := scorer.ApplySocialCap()
	expected := baseWeightedScore + socialContribution

	if finalScore != expected {
		t.Errorf("expected final score %.1f (base %.1f + social %.1f), got %.1f",
			expected, baseWeightedScore, socialContribution, finalScore)
	}

	// Verify social is truly additive, not part of the weighted average
	if finalScore <= 100.0 {
		t.Log("Good: final score can exceed 100 when social factors are applied")
	} else {
		// This is expected behavior - social can push scores above 100
		t.Logf("Final score %.1f exceeds 100 due to social factors (expected behavior)", finalScore)
	}
}

func TestSocialFactor_ZeroConfidenceIgnored(t *testing.T) {
	scorer := &MockCompositeScorer{
		BaseScore: 80.0,
		SocialFactors: []MockSocialFactor{
			{Score: 15.0, Confidence: 0.0, Source: "unreliable_source"}, // Should be ignored
			{Score: 5.0, Confidence: 1.0, Source: "reliable_source"},    // Should contribute
		},
	}

	finalScore := scorer.ApplySocialCap()
	expected := 85.0 // 80 + 5 (only reliable source)

	if finalScore != expected {
		t.Errorf("expected final score %.1f, got %.1f", expected, finalScore)
	}
}

func TestSocialFactor_NegativeScoresAllowed(t *testing.T) {
	// Negative social sentiment should be allowed to reduce scores
	scorer := &MockCompositeScorer{
		BaseScore: 85.0,
		SocialFactors: []MockSocialFactor{
			{Score: -3.0, Confidence: 1.0, Source: "negative_sentiment"},
			{Score: 2.0, Confidence: 1.0, Source: "positive_mention"},
		},
	}

	finalScore := scorer.ApplySocialCap()
	expected := 84.0 // 85 - 3 + 2 = 84

	if finalScore != expected {
		t.Errorf("expected final score %.1f, got %.1f", expected, finalScore)
	}
}

func TestSocialFactor_CapEnforcementAfterConfidence(t *testing.T) {
	// Test that cap is applied AFTER confidence weighting
	scorer := &MockCompositeScorer{
		BaseScore: 75.0,
		SocialFactors: []MockSocialFactor{
			// Raw total would be 20, but confidence-weighted total is 12
			// Still exceeds cap of 10, so should be capped
			{Score: 15.0, Confidence: 0.6, Source: "source1"}, // 9.0 effective
			{Score: 10.0, Confidence: 0.3, Source: "source2"}, // 3.0 effective
		},
	}

	finalScore := scorer.ApplySocialCap()
	expected := 85.0 // 75 + 10 (capped)

	if finalScore != expected {
		t.Errorf("expected final score %.1f (75 + 10 capped), got %.1f", expected, finalScore)
	}

	// Verify the confidence-weighted total exceeds cap
	confWeightedTotal := 15.0*0.6 + 10.0*0.3 // 9.0 + 3.0 = 12.0
	if confWeightedTotal <= 10.0 {
		t.Errorf("test setup error: confidence-weighted total %.1f should exceed cap", confWeightedTotal)
	}
}
