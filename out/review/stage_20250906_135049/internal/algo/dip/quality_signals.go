package dip

import (
	"context"
	"fmt"
	"time"

	"cryptorun/internal/domain"
)

// VolumeConfig contains volume-based quality signal parameters
type VolumeConfig struct {
	ADVMultMin float64 `yaml:"adv_mult_min"`
	VADRMin    float64 `yaml:"vadr_min"`
}

// MicrostructureConfig contains microstructure quality parameters
type MicrostructureConfig struct {
	SpreadBpsMax     float64 `yaml:"spread_bps_max"`
	DepthUSD2PcMin   int64   `yaml:"depth_usd_2pc_min"`
}

// QualityMetrics contains all quality signal measurements
type QualityMetrics struct {
	Liquidity LiquidityMetrics `json:"liquidity"`
	Volume    VolumeMetrics    `json:"volume"`
	Brand     BrandMetrics     `json:"brand"`
	Score     float64          `json:"total_score"`
}

// LiquidityMetrics contains depth and spread measurements
type LiquidityMetrics struct {
	DepthUSD2Pc  int64   `json:"depth_usd_2pc"`
	SpreadBps    float64 `json:"spread_bps"`
	Qualified    bool    `json:"qualified"`
	FailReason   string  `json:"fail_reason,omitempty"`
}

// VolumeMetrics contains volume and VADR measurements
type VolumeMetrics struct {
	Volume1h     float64 `json:"volume_1h"`
	ADV1h        float64 `json:"adv_1h"`
	VolumeRatio  float64 `json:"volume_ratio"`
	VADR6h       float64 `json:"vadr_6h"`
	Qualified    bool    `json:"qualified"`
	FailReason   string  `json:"fail_reason,omitempty"`
}

// BrandMetrics contains brand/social signal measurements
type BrandMetrics struct {
	SocialScore  float64 `json:"social_score"`
	BrandScore   float64 `json:"brand_score"`
	CappedScore  float64 `json:"capped_score"`
	Qualified    bool    `json:"qualified"`
}

// QualityAnalyzer analyzes quality signals for dip candidates
type QualityAnalyzer struct {
	volumeConfig        VolumeConfig
	microstructureConfig MicrostructureConfig
	brandCapPoints      int
}

// NewQualityAnalyzer creates a new quality signal analyzer
func NewQualityAnalyzer(volumeConfig VolumeConfig, microConfig MicrostructureConfig, brandCap int) *QualityAnalyzer {
	return &QualityAnalyzer{
		volumeConfig:         volumeConfig,
		microstructureConfig: microConfig,
		brandCapPoints:       brandCap,
	}
}

// AnalyzeQuality performs comprehensive quality analysis for a dip candidate
func (qa *QualityAnalyzer) AnalyzeQuality(ctx context.Context, symbol string, dipPoint *DipPoint, 
	microInputs *domain.MicroGateInputs, volumeData []MarketData, socialData *SocialData) (*QualityMetrics, error) {
	
	if dipPoint == nil {
		return nil, fmt.Errorf("dip point is required for quality analysis")
	}
	
	// Analyze liquidity using existing microstructure gates
	liquidity, err := qa.analyzeLiquidity(ctx, symbol, microInputs)
	if err != nil {
		return nil, fmt.Errorf("liquidity analysis failed: %w", err)
	}
	
	// Analyze volume patterns
	volume, err := qa.analyzeVolume(ctx, volumeData, dipPoint.Index)
	if err != nil {
		return nil, fmt.Errorf("volume analysis failed: %w", err)
	}
	
	// Analyze brand/social signals
	brand := qa.analyzeBrand(ctx, socialData)
	
	// Calculate composite quality score
	score := qa.calculateQualityScore(liquidity, volume, brand)
	
	return &QualityMetrics{
		Liquidity: *liquidity,
		Volume:    *volume,
		Brand:     *brand,
		Score:     score,
	}, nil
}

// analyzeLiquidity uses existing microstructure validation interfaces
func (qa *QualityAnalyzer) analyzeLiquidity(ctx context.Context, symbol string, inputs *domain.MicroGateInputs) (*LiquidityMetrics, error) {
	if inputs == nil {
		return &LiquidityMetrics{
			Qualified:  false,
			FailReason: "microstructure inputs not available",
		}, nil
	}
	
	// Calculate spread in basis points
	spreadBps := 0.0
	if inputs.Ask > inputs.Bid && inputs.Bid > 0 {
		midpoint := (inputs.Bid + inputs.Ask) / 2
		spreadBps = ((inputs.Ask - inputs.Bid) / midpoint) * 10000
	}
	
	// Check depth requirement
	depthUSD2Pc := int64(inputs.Depth2PcUSD)
	
	// Apply quality thresholds
	spreadOK := spreadBps <= qa.microstructureConfig.SpreadBpsMax
	depthOK := depthUSD2Pc >= qa.microstructureConfig.DepthUSD2PcMin
	
	qualified := spreadOK && depthOK
	failReason := ""
	
	if !spreadOK {
		failReason = fmt.Sprintf("spread %.1f bps exceeds max %.1f bps", spreadBps, qa.microstructureConfig.SpreadBpsMax)
	} else if !depthOK {
		failReason = fmt.Sprintf("depth $%d below min $%d", depthUSD2Pc, qa.microstructureConfig.DepthUSD2PcMin)
	}
	
	return &LiquidityMetrics{
		DepthUSD2Pc: depthUSD2Pc,
		SpreadBps:   spreadBps,
		Qualified:   qualified,
		FailReason:  failReason,
	}, nil
}

// analyzeVolume checks volume surge and VADR requirements
func (qa *QualityAnalyzer) analyzeVolume(ctx context.Context, data []MarketData, dipIndex int) (*VolumeMetrics, error) {
	if len(data) < 24 || dipIndex < 0 || dipIndex >= len(data) {
		return &VolumeMetrics{
			Qualified:  false,
			FailReason: "insufficient volume data",
		}, nil
	}
	
	// Calculate 1h volume at dip point
	volume1h := data[dipIndex].Volume
	
	// Calculate ADV over last 24 periods (representing 24h if 1h data)
	adv1h := qa.calculateADV(data, dipIndex, 24)
	
	// Volume ratio requirement
	volumeRatio := 0.0
	if adv1h > 0 {
		volumeRatio = volume1h / adv1h
	}
	
	// Calculate VADR over 6h window
	vadr6h := qa.calculateVADR(data, dipIndex, 6)
	
	// Apply volume quality thresholds
	volumeOK := volumeRatio >= qa.volumeConfig.ADVMultMin
	vadrOK := vadr6h >= qa.volumeConfig.VADRMin
	
	qualified := volumeOK && vadrOK
	failReason := ""
	
	if !volumeOK {
		failReason = fmt.Sprintf("volume ratio %.2fx below min %.2fx", volumeRatio, qa.volumeConfig.ADVMultMin)
	} else if !vadrOK {
		failReason = fmt.Sprintf("VADR %.2fx below min %.2fx", vadr6h, qa.volumeConfig.VADRMin)
	}
	
	return &VolumeMetrics{
		Volume1h:    volume1h,
		ADV1h:       adv1h,
		VolumeRatio: volumeRatio,
		VADR6h:      vadr6h,
		Qualified:   qualified,
		FailReason:  failReason,
	}, nil
}

// analyzeBrand applies social/brand scoring with cap
func (qa *QualityAnalyzer) analyzeBrand(ctx context.Context, socialData *SocialData) *BrandMetrics {
	if socialData == nil {
		return &BrandMetrics{
			SocialScore: 0,
			BrandScore:  0,
			CappedScore: 0,
			Qualified:   true, // Brand is optional
		}
	}
	
	// Calculate raw social score (implementation-dependent)
	socialScore := socialData.SentimentScore * socialData.VolumeMultiplier
	brandScore := socialData.BrandRecognition * socialData.TrustScore
	
	// Total before capping
	rawTotal := socialScore + brandScore
	
	// Apply cap
	cappedScore := rawTotal
	if cappedScore > float64(qa.brandCapPoints) {
		cappedScore = float64(qa.brandCapPoints)
	}
	
	return &BrandMetrics{
		SocialScore: socialScore,
		BrandScore:  brandScore,
		CappedScore: cappedScore,
		Qualified:   true, // Brand signals are always accepted, just capped
	}
}

// calculateQualityScore computes weighted composite quality score
func (qa *QualityAnalyzer) calculateQualityScore(liquidity *LiquidityMetrics, volume *VolumeMetrics, brand *BrandMetrics) float64 {
	score := 0.0
	
	// Liquidity contributes if qualified
	if liquidity.Qualified {
		// Higher score for tighter spreads and deeper books
		spreadScore := (qa.microstructureConfig.SpreadBpsMax - liquidity.SpreadBps) / qa.microstructureConfig.SpreadBpsMax
		if spreadScore < 0 {
			spreadScore = 0
		}
		
		depthScore := float64(liquidity.DepthUSD2Pc) / float64(qa.microstructureConfig.DepthUSD2PcMin)
		if depthScore > 2.0 {
			depthScore = 2.0 // Cap at 2x requirement
		}
		
		score += (spreadScore + depthScore) * 25 // Up to 50 points for liquidity
	}
	
	// Volume contributes if qualified
	if volume.Qualified {
		volumeScore := volume.VolumeRatio / qa.volumeConfig.ADVMultMin
		if volumeScore > 2.0 {
			volumeScore = 2.0 // Cap at 2x requirement
		}
		
		vadrScore := volume.VADR6h / qa.volumeConfig.VADRMin
		if vadrScore > 2.0 {
			vadrScore = 2.0 // Cap at 2x requirement
		}
		
		score += (volumeScore + vadrScore) * 12.5 // Up to 25 points for volume
	}
	
	// Brand always contributes (already capped)
	score += brand.CappedScore
	
	return score
}

// calculateADV computes average daily volume over specified periods
func (qa *QualityAnalyzer) calculateADV(data []MarketData, fromIndex, periods int) float64 {
	if fromIndex < periods {
		periods = fromIndex + 1
	}
	
	sum := 0.0
	count := 0
	
	start := fromIndex - periods + 1
	if start < 0 {
		start = 0
	}
	
	for i := start; i <= fromIndex; i++ {
		sum += data[i].Volume
		count++
	}
	
	if count == 0 {
		return 0
	}
	
	return sum / float64(count)
}

// calculateVADR computes Volume-Adjusted Daily Range over window
func (qa *QualityAnalyzer) calculateVADR(data []MarketData, fromIndex, windowHours int) float64 {
	if fromIndex < windowHours {
		windowHours = fromIndex + 1
	}
	
	start := fromIndex - windowHours + 1
	if start < 0 {
		start = 0
	}
	
	totalVolumeRange := 0.0
	totalVolume := 0.0
	
	for i := start; i <= fromIndex; i++ {
		bar := data[i]
		if bar.High > bar.Low && bar.Volume > 0 {
			priceRange := (bar.High - bar.Low) / bar.Low // Normalized range
			volumeWeight := bar.Volume
			
			totalVolumeRange += priceRange * volumeWeight
			totalVolume += volumeWeight
		}
	}
	
	if totalVolume == 0 {
		return 0
	}
	
	// Return volume-weighted average range multiplier
	avgRange := totalVolumeRange / totalVolume
	
	// Calculate reference range for comparison
	refRange := qa.calculateReferenceRange(data, fromIndex, windowHours*4) // 4x window for reference
	
	if refRange == 0 {
		return 1.0
	}
	
	return avgRange / refRange
}

// calculateReferenceRange computes reference range for VADR calculation
func (qa *QualityAnalyzer) calculateReferenceRange(data []MarketData, fromIndex, refWindow int) float64 {
	if fromIndex < refWindow {
		refWindow = fromIndex + 1
	}
	
	start := fromIndex - refWindow + 1
	if start < 0 {
		start = 0
	}
	
	sum := 0.0
	count := 0
	
	for i := start; i <= fromIndex; i++ {
		bar := data[i]
		if bar.High > bar.Low {
			sum += (bar.High - bar.Low) / bar.Low
			count++
		}
	}
	
	if count == 0 {
		return 0
	}
	
	return sum / float64(count)
}

// SocialData represents social/brand signal inputs
type SocialData struct {
	SentimentScore    float64   `json:"sentiment_score"`
	VolumeMultiplier  float64   `json:"volume_multiplier"`
	BrandRecognition  float64   `json:"brand_recognition"`
	TrustScore        float64   `json:"trust_score"`
	LastUpdated       time.Time `json:"last_updated"`
}

// ValidateSocialData ensures social data is current and valid
func ValidateSocialData(data *SocialData, maxAgeMinutes int) bool {
	if data == nil {
		return false
	}
	
	// Check data age
	age := time.Since(data.LastUpdated)
	if age > time.Duration(maxAgeMinutes)*time.Minute {
		return false
	}
	
	// Validate ranges
	return data.SentimentScore >= -1.0 && data.SentimentScore <= 1.0 &&
		   data.VolumeMultiplier >= 0 && data.VolumeMultiplier <= 10.0 &&
		   data.BrandRecognition >= 0 && data.BrandRecognition <= 1.0 &&
		   data.TrustScore >= 0 && data.TrustScore <= 1.0
}