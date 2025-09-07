package data

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sawpanic/cryptorun/internal/data"
)

func TestMultiRegionReplicationConfig(t *testing.T) {
	t.Run("replication_config_validation", func(t *testing.T) {
		config := data.StreamingConfig{
			Enable:  true,
			Backend: "stub",
			Replication: data.ReplicationConfig{
				Enable:              true,
				PrimaryRegion:       "us-east-1",
				SecondaryRegions:    []string{"us-west-2", "eu-west-1"},
				ConflictResolution:  "timestamp_wins",
				RegionPriority:      []string{"us-east-1", "us-west-2", "eu-west-1"},
				Policies: data.ReplicationPolicies{
					ActiveActive: data.ReplicationPolicy{
						Topics:         []string{"cryptorun-cold-tier-events"},
						LagThresholdMs: 500,
						CutoverPolicy:  "automatic",
					},
					ActivePassive: data.ReplicationPolicy{
						Topics:         []string{"cryptorun-cold-dlq"},
						LagThresholdMs: 5000,
						CutoverPolicy:  "manual",
					},
				},
				HealthCheck: data.ReplicationHealthConfig{
					Interval:          "30s",
					Timeout:           "10s",
					FailureThreshold:  3,
					RecoveryThreshold: 2,
				},
				Failover: data.ReplicationFailoverConfig{
					UnhealthyTimeout:   "60s",
					ErrorRateThreshold: 0.05,
					RecoveryTimeout:    "300s",
					OperatorApproval:   true,
				},
			},
		}

		streamer, err := data.NewColdTierStreamer(config)
		require.NoError(t, err)
		require.NotNil(t, streamer)

		assert.True(t, config.Replication.Enable)
		assert.Equal(t, "us-east-1", config.Replication.PrimaryRegion)
		assert.Contains(t, config.Replication.SecondaryRegions, "us-west-2")
		assert.Contains(t, config.Replication.SecondaryRegions, "eu-west-1")
	})

	t.Run("replication_disabled", func(t *testing.T) {
		config := data.StreamingConfig{
			Enable:  true,
			Backend: "stub",
			Replication: data.ReplicationConfig{
				Enable: false, // Disabled
			},
		}

		streamer, err := data.NewColdTierStreamer(config)
		require.NoError(t, err)

		ctx := context.Background()
		status, err := streamer.GetReplicationStatus(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "replication not enabled")
		assert.Empty(t, status.PrimaryRegion)
	})
}

func TestRegionManager(t *testing.T) {
	primary := "us-east-1"
	secondaries := []string{"us-west-2", "eu-west-1"}
	regionManager := data.NewStubRegionManager(primary, secondaries)

	t.Run("basic_region_info", func(t *testing.T) {
		assert.Equal(t, primary, regionManager.GetPrimaryRegion())
		assert.Equal(t, secondaries, regionManager.GetSecondaryRegions())
	})

	t.Run("health_checks", func(t *testing.T) {
		ctx := context.Background()

		healthy, err := regionManager.IsHealthy(ctx, primary)
		assert.NoError(t, err)
		assert.True(t, healthy)

		healthy, err = regionManager.IsHealthy(ctx, "us-west-2")
		assert.NoError(t, err)
		assert.True(t, healthy) // Stub always returns healthy
	})

	t.Run("replication_lag", func(t *testing.T) {
		ctx := context.Background()

		lag, err := regionManager.GetReplicationLag(ctx, "test-topic", "us-west-2")
		assert.NoError(t, err)
		assert.Greater(t, lag, time.Duration(0))
		assert.Less(t, lag, 100*time.Millisecond) // Stub returns minimal lag
	})

	t.Run("failover_trigger", func(t *testing.T) {
		ctx := context.Background()

		err := regionManager.TriggerFailover(ctx, primary, "us-west-2")
		assert.NoError(t, err) // Stub implementation succeeds
	})
}

func TestConflictResolution(t *testing.T) {
	regionManager := data.NewStubRegionManager("us-east-1", []string{"us-west-2"})

	t.Run("timestamp_wins_strategy", func(t *testing.T) {
		now := time.Now()
		message1 := &data.StreamingMessage{
			ID:        "msg1",
			Timestamp: now,
			Payload:   []byte("message1"),
		}
		message2 := &data.StreamingMessage{
			ID:        "msg2", 
			Timestamp: now.Add(1 * time.Second), // Later timestamp
			Payload:   []byte("message2"),
		}

		winner, err := regionManager.ResolveConflict(message1, message2)
		assert.NoError(t, err)
		assert.Equal(t, message2.ID, winner.ID) // Later timestamp wins
		assert.Equal(t, message2.Payload, winner.Payload)

		// Test reverse order
		winner, err = regionManager.ResolveConflict(message2, message1)
		assert.NoError(t, err)
		assert.Equal(t, message2.ID, winner.ID) // Still later timestamp wins
	})
}

func TestReplicationManager(t *testing.T) {
	primary := "us-east-1"
	secondaries := []string{"us-west-2", "eu-west-1"}
	replicationManager := data.NewStubReplicationManager(primary, secondaries)

	t.Run("replication_status", func(t *testing.T) {
		ctx := context.Background()

		status, err := replicationManager.GetReplicationStatus(ctx)
		assert.NoError(t, err)

		assert.Equal(t, primary, status.PrimaryRegion)
		assert.Equal(t, secondaries, status.SecondaryRegions)
		assert.True(t, status.RegionHealth[primary])
		assert.True(t, status.RegionHealth["us-west-2"])
		assert.True(t, status.RegionHealth["eu-west-1"])
		assert.Equal(t, "active_active", status.ActivePolicy)

		// Check replication lags are reasonable
		for region, lag := range status.ReplicationLags {
			assert.Greater(t, lag, time.Duration(0), "Region %s should have positive lag", region)
			assert.Less(t, lag, 100*time.Millisecond, "Region %s lag should be minimal", region)
		}
	})

	t.Run("replication_lifecycle", func(t *testing.T) {
		ctx := context.Background()

		// Start replication
		err := replicationManager.StartReplication(ctx)
		assert.NoError(t, err)

		// Replicate a message
		message := &data.StreamingMessage{
			ID:        "test-message",
			Topic:     "test-topic",
			Payload:   []byte("test payload"),
			Timestamp: time.Now(),
		}

		err = replicationManager.ReplicateMessage(ctx, message, secondaries)
		assert.NoError(t, err) // Stub implementation always succeeds

		// Stop replication
		err = replicationManager.StopReplication(ctx)
		assert.NoError(t, err)
	})
}

func TestStreamingReplication(t *testing.T) {
	config := data.StreamingConfig{
		Enable:     true,
		Backend:    "stub",
		BatchSize:  3,
		EnableDLQ:  true,
		Topics:     map[string]string{"dlq": "test-dlq"},
		Replication: data.ReplicationConfig{
			Enable:           true,
			PrimaryRegion:    "us-east-1",
			SecondaryRegions: []string{"us-west-2", "eu-west-1"},
			Policies: data.ReplicationPolicies{
				ActiveActive: data.ReplicationPolicy{
					Topics: []string{"cryptorun-cold-tier-events"},
				},
				ActivePassive: data.ReplicationPolicy{
					Topics: []string{"cryptorun-cold-dlq"},
				},
			},
		},
	}

	streamer, err := data.NewColdTierStreamer(config)
	require.NoError(t, err)

	t.Run("replication_enabled_operations", func(t *testing.T) {
		ctx := context.Background()

		// Enable replication
		err := streamer.EnableReplication(ctx)
		assert.NoError(t, err)

		// Check replication status
		status, err := streamer.GetReplicationStatus(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "us-east-1", status.PrimaryRegion)
		assert.Contains(t, status.SecondaryRegions, "us-west-2")

		// Check region health
		healthy, err := streamer.GetRegionHealth(ctx, "us-east-1")
		assert.NoError(t, err)
		assert.True(t, healthy)

		// Check replication lag
		lag, err := streamer.GetReplicationLag(ctx, "test-topic", "us-west-2")
		assert.NoError(t, err)
		assert.Greater(t, lag, time.Duration(0))

		// Trigger failover
		err = streamer.TriggerFailover(ctx, "us-east-1", "us-west-2")
		assert.NoError(t, err)
	})

	t.Run("streaming_with_replication", func(t *testing.T) {
		ctx := context.Background()

		// Track replication metrics
		replicationSuccesses := int64(0)
		replicationErrors := int64(0)

		streamer.SetMetricsCallback(func(metric string, value int64) {
			switch metric {
			case "cold_streaming_replication_success":
				replicationSuccesses += value
			case "cold_streaming_replication_error":
				replicationErrors++
			}
		})

		// Stream messages that should trigger replication
		envelopes := createTestEnvelopes(5)
		err := streamer.StreamEnvelopes(ctx, envelopes, "cryptorun-cold-tier-events")
		assert.NoError(t, err)

		// In stub implementation, replication should succeed
		assert.Greater(t, replicationSuccesses, int64(0))
		assert.Equal(t, int64(0), replicationErrors)

		// Close streamer
		err = streamer.Close(ctx)
		assert.NoError(t, err)
	})
}

func TestReplicationPolicies(t *testing.T) {
	config := data.StreamingConfig{
		Enable:  true,
		Backend: "stub",
		Replication: data.ReplicationConfig{
			Enable:           true,
			PrimaryRegion:    "us-east-1",
			SecondaryRegions: []string{"us-west-2"},
			Policies: data.ReplicationPolicies{
				ActiveActive: data.ReplicationPolicy{
					Topics: []string{"active-active-topic"},
				},
				ActivePassive: data.ReplicationPolicy{
					Topics: []string{"active-passive-topic"},
				},
			},
		},
	}

	streamer, err := data.NewColdTierStreamer(config)
	require.NoError(t, err)

	t.Run("active_active_replication", func(t *testing.T) {
		ctx := context.Background()
		
		// Active-active topics should replicate to all secondary regions
		envelopes := createTestEnvelopes(2)
		err := streamer.StreamEnvelopes(ctx, envelopes, "active-active-topic")
		assert.NoError(t, err)
	})

	t.Run("active_passive_replication", func(t *testing.T) {
		ctx := context.Background()
		
		// Active-passive topics should only replicate to primary
		envelopes := createTestEnvelopes(2)
		err := streamer.StreamEnvelopes(ctx, envelopes, "active-passive-topic")
		assert.NoError(t, err)
	})

	t.Run("non_replicated_topics", func(t *testing.T) {
		ctx := context.Background()
		
		// Topics not in replication policies should not replicate
		envelopes := createTestEnvelopes(2)
		err := streamer.StreamEnvelopes(ctx, envelopes, "non-replicated-topic")
		assert.NoError(t, err)
	})
}

func TestReplicationMetrics(t *testing.T) {
	config := data.StreamingConfig{
		Enable:  true,
		Backend: "stub",
		Replication: data.ReplicationConfig{
			Enable:           true,
			PrimaryRegion:    "us-east-1",
			SecondaryRegions: []string{"us-west-2", "eu-west-1"},
			Policies: data.ReplicationPolicies{
				ActiveActive: data.ReplicationPolicy{
					Topics: []string{"replicated-topic"},
				},
			},
		},
	}

	streamer, err := data.NewColdTierStreamer(config)
	require.NoError(t, err)

	t.Run("replication_success_metrics", func(t *testing.T) {
		ctx := context.Background()

		var replicationMetrics = make(map[string]int64)
		streamer.SetMetricsCallback(func(metric string, value int64) {
			replicationMetrics[metric] += value
		})

		// Stream messages that will trigger replication
		envelopes := createTestEnvelopes(3)
		err := streamer.StreamEnvelopes(ctx, envelopes, "replicated-topic")
		assert.NoError(t, err)

		// Should have successful batch and replication metrics
		assert.Greater(t, replicationMetrics["cold_streaming_batch_success"], int64(0))
		assert.Greater(t, replicationMetrics["cold_streaming_replication_success"], int64(0))
		assert.Equal(t, int64(0), replicationMetrics["cold_streaming_replication_error"])
	})
}

func TestReplicationFailureScenarios(t *testing.T) {
	t.Run("replication_disabled_operations", func(t *testing.T) {
		config := data.StreamingConfig{
			Enable:  true,
			Backend: "stub",
			Replication: data.ReplicationConfig{
				Enable: false, // Explicitly disabled
			},
		}

		streamer, err := data.NewColdTierStreamer(config)
		require.NoError(t, err)

		ctx := context.Background()

		// Operations should fail when replication is disabled
		err = streamer.EnableReplication(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "replication is disabled")

		_, err = streamer.GetRegionHealth(ctx, "us-east-1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "region manager not initialized")

		err = streamer.TriggerFailover(ctx, "us-east-1", "us-west-2")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "region manager not initialized")
	})

	t.Run("streaming_without_replication_config", func(t *testing.T) {
		config := data.StreamingConfig{
			Enable:  true,
			Backend: "stub",
			// No replication config - should default to disabled
		}

		streamer, err := data.NewColdTierStreamer(config)
		require.NoError(t, err)

		ctx := context.Background()

		// Should handle gracefully without replication
		envelopes := createTestEnvelopes(2)
		err = streamer.StreamEnvelopes(ctx, envelopes, "test-topic")
		assert.NoError(t, err)

		err = streamer.Close(ctx)
		assert.NoError(t, err)
	})
}

// Note: createTestEnvelopes is defined in cold_streaming_test.go to avoid duplication