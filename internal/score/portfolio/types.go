package portfolio

import "time"

// Position represents a trading position
type Position struct {
	Symbol      string    `json:"symbol"`
	Size        float64   `json:"size"`        // Position size as % of portfolio
	Sector      string    `json:"sector"`      
	Beta        float64   `json:"beta"`        
	Correlation float64   `json:"correlation"` // Correlation with new position
	EntryTime   time.Time `json:"entry_time"`
}

// PortfolioState represents current portfolio state for scoring
type PortfolioState struct {
	Positions     []Position         `json:"positions"`
	TotalValue    float64            `json:"total_value"`
	SectorWeights map[string]float64 `json:"sector_weights"`
	BetaExposure  float64            `json:"beta_exposure"`
	Drawdown      float64            `json:"drawdown"`
	Volatility    float64            `json:"volatility"`
}

// ConstraintConfig defines portfolio-aware scoring constraints  
type ConstraintConfig struct {
	MaxPositionSize    float64 `yaml:"max_position_size"`
	MaxSectorConc      float64 `yaml:"max_sector_conc"`
	MaxCorrelation     float64 `yaml:"max_correlation"`
	MinLiquidity       float64 `yaml:"min_liquidity"`
	DrawdownThreshold  float64 `yaml:"drawdown_threshold"`
	BetaBudgetLimit    float64 `yaml:"beta_budget_limit"`
}

// PortfolioGateConfig defines gate-level portfolio constraints
type PortfolioGateConfig struct {
	MinScore          float64 `yaml:"min_score"`
	MinAdjustedScore  float64 `yaml:"min_adjusted_score"`
	MaxPositions      int     `yaml:"max_positions"`
	MaxSectorWeight   float64 `yaml:"max_sector_weight"`
	MaxCorrelation    float64 `yaml:"max_correlation"`
	MinLiquidity      float64 `yaml:"min_liquidity"`
	MaxDrawdown       float64 `yaml:"max_drawdown"`
	RequireStandard   bool    `yaml:"require_standard"`
}

// DrawdownLevel defines drawdown-based adjustments
type DrawdownLevel struct {
	Threshold  float64 `json:"threshold"`
	Multiplier float64 `json:"multiplier"`
}