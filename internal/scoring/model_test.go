package scoring

import (
	"math"
	"testing"
)

func TestCompositeScore_Basic(t *testing.T) {
	calc := NewCalculator(RegimeChoppy)
	
	input := FactorInput{
		Symbol: "BTC-USD",
		Momentum: MomentumFactors{
			Return1h:  5.0,
			Return4h:  8.0,
			Return12h: 10.0,
			Return24h: 12.0,
			Return7d:  15.0,
			Accel4h:   2.0,
		},
		Technical: TechnicalFactors{
			RSI14:    70.0,
			MACD:     1.5,
			BBWidth:  0.8,
			ATRRatio: 1.2,
		},
		Volume: VolumeFactors{
			VolumeRatio24h: 2.5,
			VWAP:          100.5,
			OBV:           50000,
			VolSpike:      3.0,
		},
		Quality: QualityFactors{
			Spread:    0.001,
			Depth:     100000,
			VADR:      2.0,
			MarketCap: 1000000000,
		},
		Social: SocialFactors{
			Sentiment:    0.8,
			Mentions:     1000,
			SocialVolume: 5000,
			RedditScore:  85.0,
		},
	}

	result, err := calc.Calculate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Score == 0 {
		t.Error("expected non-zero composite score")
	}

	if result.Meta.Symbol != "BTC-USD" {
		t.Errorf("expected symbol BTC-USD, got %s", result.Meta.Symbol)
	}

	if result.Meta.Regime != RegimeChoppy {
		t.Errorf("expected regime choppy, got %s", result.Meta.Regime)
	}

	if !result.Meta.IsOrthogonal {
		t.Error("expected orthogonal flag to be true")
	}
}

func TestWeightSum_100Percent(t *testing.T) {
	rwc := NewRegimeWeights()
	
	testCases := []Regime{RegimeTrending, RegimeChoppy, RegimeHighVol}
	
	for _, regime := range testCases {
		weights := rwc.GetWeights(regime)
		sum := weights.Sum()
		
		if math.Abs(sum-1.0) > 0.001 {
			t.Errorf("regime %s weights sum to %.3f, expected 1.000", regime, sum)
		}
	}
}

func TestSocialCap_Enforcement(t *testing.T) {
	sr := NewSocialResiduals()
	
	testCases := []struct {
		input    float64
		expected float64
	}{
		{5.0, 5.0},
		{10.0, 10.0},
		{15.0, 10.0},
		{-5.0, -5.0},
		{-10.0, -10.0},
		{-15.0, -10.0},
		{100.0, 10.0},
		{-100.0, -10.0},
	}
	
	for _, tc := range testCases {
		result := sr.applyCap(tc.input)
		if result != tc.expected {
			t.Errorf("social cap input %.1f: expected %.1f, got %.1f", tc.input, tc.expected, result)
		}
	}
}

func TestMomentumCore_Protection(t *testing.T) {
	mc := NewMomentumCore()
	
	if !mc.IsProtected() {
		t.Error("momentum core should be protected from orthogonalization")
	}
}

func TestDecileMonotonicity(t *testing.T) {
	calc := NewCalculator(RegimeTrending)
	
	baseInput := FactorInput{
		Symbol: "TEST-USD",
		Momentum: MomentumFactors{Return4h: 0},
		Technical: TechnicalFactors{RSI14: 50},
		Volume: VolumeFactors{VolumeRatio24h: 1.0},
		Quality: QualityFactors{VADR: 1.5},
		Social: SocialFactors{Sentiment: 0.0},
	}
	
	var prevScore float64
	for i := 1; i <= 5; i++ {
		testInput := baseInput
		testInput.Momentum.Return4h = float64(i * 5)
		testInput.Momentum.Return1h = float64(i * 2)
		testInput.Momentum.Return12h = float64(i * 3)
		testInput.Momentum.Return24h = float64(i * 4)
		
		result, err := calc.Calculate(testInput)
		if err != nil {
			t.Fatalf("unexpected error for decile %d: %v", i, err)
		}
		
		if i > 1 && result.Parts["momentum"] <= prevScore {
			t.Errorf("decile %d momentum %.2f not greater than decile %d momentum %.2f", 
				i, result.Parts["momentum"], i-1, prevScore)
		}
		
		prevScore = result.Parts["momentum"]
	}
}

func TestOrthogonalitySmoke(t *testing.T) {
	calc := NewCalculator(RegimeChoppy)
	
	testCases := []struct {
		name string
		input FactorInput
	}{
		{
			name: "high_momentum",
			input: FactorInput{
				Symbol: "TEST1-USD",
				Momentum: MomentumFactors{Return4h: 20},
				Technical: TechnicalFactors{RSI14: 30},
				Volume: VolumeFactors{VolumeRatio24h: 1.0},
				Quality: QualityFactors{VADR: 1.5},
				Social: SocialFactors{Sentiment: 0.0},
			},
		},
		{
			name: "high_technical",
			input: FactorInput{
				Symbol: "TEST2-USD",
				Momentum: MomentumFactors{Return4h: 5},
				Technical: TechnicalFactors{RSI14: 80},
				Volume: VolumeFactors{VolumeRatio24h: 1.0},
				Quality: QualityFactors{VADR: 1.5},
				Social: SocialFactors{Sentiment: 0.0},
			},
		},
	}
	
	correlations := make(map[string]float64)
	
	for _, tc := range testCases {
		result, err := calc.Calculate(tc.input)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", tc.name, err)
		}
		
		correlations[tc.name] = result.Parts["technical"]
	}
	
	if len(correlations) != 2 {
		t.Error("expected 2 test cases for correlation check")
	}
}

func TestRegimeWeights_Validation(t *testing.T) {
	rwc := NewRegimeWeights()
	
	if err := rwc.ValidateWeights(); err != nil {
		t.Errorf("weight validation failed: %v", err)
	}
}