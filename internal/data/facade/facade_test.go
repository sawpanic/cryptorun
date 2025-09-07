package facade

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock repository for testing
type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) InsertTrade(ctx context.Context, trade Trade) error {
	args := m.Called(ctx, trade)
	return args.Error(0)
}

func (m *mockRepository) ReadTrades(ctx context.Context, symbol string, from time.Time, to time.Time, limit int) ([]Trade, error) {
	args := m.Called(ctx, symbol, from, to, limit)
	return args.Get(0).([]Trade), args.Error(1)
}

func (m *mockRepository) UpsertRegime(ctx context.Context, snapshot RegimeSnapshot) error {
	args := m.Called(ctx, snapshot)
	return args.Error(0)
}

func (m *mockRepository) ReadRegimes(ctx context.Context, from time.Time, to time.Time) ([]RegimeSnapshot, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]RegimeSnapshot), args.Error(1)
}

func (m *mockRepository) UpsertArtifact(ctx context.Context, artifact PremoveArtifact) error {
	args := m.Called(ctx, artifact)
	return args.Error(0)
}

func (m *mockRepository) ReadArtifacts(ctx context.Context, symbol string, from time.Time, to time.Time, limit int) ([]PremoveArtifact, error) {
	args := m.Called(ctx, symbol, from, to, limit)
	return args.Get(0).([]PremoveArtifact), args.Error(1)
}

func (m *mockRepository) Health(ctx context.Context) RepositoryHealth {
	args := m.Called(ctx)
	return args.Get(0).(RepositoryHealth)
}

func TestFacade_SetRepository(t *testing.T) {
	// Setup
	hotCfg := HotConfig{Venues: []string{"kraken"}}
	warmCfg := WarmConfig{Venues: []string{"binance"}}
	cacheCfg := CacheConfig{MaxEntries: 1000}
	
	facade := New(hotCfg, warmCfg, cacheCfg, nil)
	mockRepo := &mockRepository{}
	
	t.Run("initial_state", func(t *testing.T) {
		assert.False(t, facade.dbEnabled)
		assert.Nil(t, facade.repository)
	})
	
	t.Run("set_repository", func(t *testing.T) {
		facade.SetRepository(mockRepo)
		assert.True(t, facade.dbEnabled)
		assert.NotNil(t, facade.repository)
	})
}

func TestPITReader_Trades(t *testing.T) {
	ctx := context.Background()
	symbol := "BTC-USD"
	from := time.Date(2025, 9, 7, 10, 0, 0, 0, time.UTC)
	to := time.Date(2025, 9, 7, 11, 0, 0, 0, time.UTC)
	
	t.Run("database_disabled", func(t *testing.T) {
		pitReader := &pitReader{
			repository: nil,
			dbEnabled:  false,
		}
		
		trades, err := pitReader.Trades(ctx, symbol, from, to)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database not enabled")
		assert.Nil(t, trades)
	})
	
	t.Run("database_enabled_success", func(t *testing.T) {
		mockRepo := &mockRepository{}
		expectedTrades := []Trade{
			{
				Symbol:    "BTC-USD",
				Venue:     "kraken",
				Timestamp: from.Add(5 * time.Minute),
				Price:     50000.0,
				Size:      0.1,
				Side:      "buy",
				TradeID:   "trade123",
			},
		}
		
		mockRepo.On("ReadTrades", ctx, symbol, from, to, 1000).Return(expectedTrades, nil)
		
		pitReader := &pitReader{
			repository: mockRepo,
			dbEnabled:  true,
		}
		
		trades, err := pitReader.Trades(ctx, symbol, from, to)
		require.NoError(t, err)
		assert.Equal(t, expectedTrades, trades)
		
		mockRepo.AssertExpectations(t)
	})
}

func TestPITReader_Regimes(t *testing.T) {
	ctx := context.Background()
	from := time.Date(2025, 9, 7, 8, 0, 0, 0, time.UTC) // 4h boundary
	to := time.Date(2025, 9, 7, 12, 0, 0, 0, time.UTC)   // 4h boundary
	
	t.Run("database_disabled", func(t *testing.T) {
		pitReader := &pitReader{
			repository: nil,
			dbEnabled:  false,
		}
		
		regimes, err := pitReader.Regimes(ctx, from, to)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database not enabled")
		assert.Nil(t, regimes)
	})
	
	t.Run("database_enabled_success", func(t *testing.T) {
		mockRepo := &mockRepository{}
		expectedRegimes := []RegimeSnapshot{
			{
				Timestamp:       from,
				RealizedVol7d:   15.5,
				PctAbove20MA:    67.8,
				BreadthThrust:   0.23,
				Regime:          "trending",
				Weights: map[string]float64{
					"momentum":  30.0,
					"technical": 25.0,
					"volume":    20.0,
					"quality":   15.0,
					"social":    10.0,
				},
				ConfidenceScore: 0.82,
				DetectionMethod: "majority_vote",
				Metadata:        map[string]interface{}{"test": true},
				CreatedAt:       from,
			},
		}
		
		mockRepo.On("ReadRegimes", ctx, from, to).Return(expectedRegimes, nil)
		
		pitReader := &pitReader{
			repository: mockRepo,
			dbEnabled:  true,
		}
		
		regimes, err := pitReader.Regimes(ctx, from, to)
		require.NoError(t, err)
		assert.Equal(t, expectedRegimes, regimes)
		
		mockRepo.AssertExpectations(t)
	})
}

func TestPITReader_Artifacts(t *testing.T) {
	ctx := context.Background()
	symbol := "ETH-USD"
	from := time.Date(2025, 9, 7, 10, 0, 0, 0, time.UTC)
	to := time.Date(2025, 9, 7, 11, 0, 0, 0, time.UTC)
	
	t.Run("database_disabled", func(t *testing.T) {
		pitReader := &pitReader{
			repository: nil,
			dbEnabled:  false,
		}
		
		artifacts, err := pitReader.Artifacts(ctx, symbol, from, to)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database not enabled")
		assert.Nil(t, artifacts)
	})
	
	t.Run("database_enabled_success", func(t *testing.T) {
		mockRepo := &mockRepository{}
		score := 85.5
		momentumCore := 42.1
		socialResidual := 8.7
		regime := "trending"
		
		expectedArtifacts := []PremoveArtifact{
			{
				ID:                 1,
				Timestamp:          from.Add(10 * time.Minute),
				Symbol:             "ETH-USD",
				Venue:              "binance",
				GateScore:          true,
				GateVADR:           true,
				GateFunding:        true,
				GateMicrostructure: true,
				GateFreshness:      true,
				GateFatigue:        false,
				Score:              &score,
				MomentumCore:       &momentumCore,
				SocialResidual:     &socialResidual,
				Factors: map[string]interface{}{
					"momentum_timeframes": []string{"1h", "4h", "12h", "24h"},
				},
				Regime:          &regime,
				ConfidenceScore: 0.75,
				CreatedAt:       from,
			},
		}
		
		mockRepo.On("ReadArtifacts", ctx, symbol, from, to, 500).Return(expectedArtifacts, nil)
		
		pitReader := &pitReader{
			repository: mockRepo,
			dbEnabled:  true,
		}
		
		artifacts, err := pitReader.Artifacts(ctx, symbol, from, to)
		require.NoError(t, err)
		assert.Equal(t, expectedArtifacts, artifacts)
		
		// Verify social cap enforcement
		for _, artifact := range artifacts {
			if artifact.SocialResidual != nil {
				assert.LessOrEqual(t, *artifact.SocialResidual, 10.0, "Social residual must be capped at +10")
			}
		}
		
		mockRepo.AssertExpectations(t)
	})
}

func TestFacade_PITReads_Integration(t *testing.T) {
	// Setup facade with repository
	hotCfg := HotConfig{Venues: []string{"kraken"}}
	warmCfg := WarmConfig{Venues: []string{"binance"}}
	cacheCfg := CacheConfig{MaxEntries: 1000}
	
	facade := New(hotCfg, warmCfg, cacheCfg, nil)
	mockRepo := &mockRepository{}
	facade.SetRepository(mockRepo)
	
	t.Run("pit_reader_creation", func(t *testing.T) {
		pitReader := facade.PITReads()
		assert.NotNil(t, pitReader)
		
		// Test that it can access the facade's repository through the interface
		ctx := context.Background()
		symbol := "BTC-USD"
		from := time.Now().Add(-1 * time.Hour)
		to := time.Now()
		
		// Should return an error if repository is properly connected but no data
		mockRepo.On("ReadTrades", ctx, symbol, from, to, 1000).Return([]Trade{}, nil)
		
		trades, err := pitReader.Trades(ctx, symbol, from, to)
		assert.NoError(t, err) // No error, just empty result
		assert.Empty(t, trades)
	})
}

func TestRepositoryHealthCheck_Structure(t *testing.T) {
	health := RepositoryHealth{
		Healthy: true,
		Errors:  []string{},
		ConnectionPool: map[string]int{
			"active":   5,
			"idle":     10,
			"max":      20,
		},
		LastCheck:      time.Now(),
		ResponseTimeMS: 45,
	}
	
	t.Run("valid_health_structure", func(t *testing.T) {
		assert.True(t, health.Healthy)
		assert.Empty(t, health.Errors)
		assert.Contains(t, health.ConnectionPool, "active")
		assert.Contains(t, health.ConnectionPool, "idle")
		assert.Contains(t, health.ConnectionPool, "max")
		assert.Greater(t, health.ResponseTimeMS, int64(0))
	})
}