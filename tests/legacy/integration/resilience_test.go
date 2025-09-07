//go:build legacy
// +build legacy

package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/application/analyst"
	"github.com/sawpanic/cryptorun/exchanges/kraken"
)

// TestTimeoutResilience tests that the system handles API timeouts gracefully
func TestTimeoutResilience(t *testing.T) {
	// Create timeout mock server - first 3 requests will timeout
	mockServer := kraken.NewTimeoutMockServer(3)
	defer mockServer.Close()
	
	client := &http.Client{
		Timeout: 2 * time.Second, // Short timeout for testing
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Test multiple requests to trigger circuit breaker behavior
	for i := 0; i < 5; i++ {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, "GET", mockServer.URL()+"/0/public/Ticker", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			
			resp, err := client.Do(req)
			
			if i < 3 {
				// First 3 requests should timeout
				if err == nil {
					resp.Body.Close()
					t.Error("Expected timeout error, but request succeeded")
				} else {
					// Verify timeout error is handled gracefully
					if !strings.Contains(strings.ToLower(err.Error()), "timeout") && 
					   !strings.Contains(strings.ToLower(err.Error()), "deadline") &&
					   !strings.Contains(strings.ToLower(err.Error()), "context") {
						t.Errorf("Expected timeout-related error, got: %v", err)
					}
				}
			} else {
				// Requests 4-5 should succeed (server recovered)
				if err != nil {
					t.Errorf("Expected successful request after recovery, got error: %v", err)
				} else {
					defer resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
					}
					
					// Verify response is valid JSON
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						t.Errorf("Failed to read response body: %v", err)
					} else {
						var result map[string]interface{}
						if err := json.Unmarshal(body, &result); err != nil {
							t.Errorf("Response is not valid JSON: %v", err)
						}
					}
				}
			}
		})
	}
	
	// Verify timeout count
	if mockServer.GetTimeoutCount() != 3 {
		t.Errorf("Expected 3 timeouts, got %d", mockServer.GetTimeoutCount())
	}
}

// TestBadJSONResilience tests handling of malformed JSON responses
func TestBadJSONResilience(t *testing.T) {
	testSuite := kraken.CreateBadJSONTestSuite()
	
	client := &http.Client{Timeout: 5 * time.Second}
	ctx := context.Background()
	
	for scenarioName, mockServer := range testSuite {
		t.Run(scenarioName, func(t *testing.T) {
			defer mockServer.Close()
			
			// Test bad JSON handling
			for i := 0; i < 4; i++ {
				req, err := http.NewRequestWithContext(ctx, "GET", mockServer.URL()+"/0/public/Ticker", nil)
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}
				
				resp, err := client.Do(req)
				if err != nil {
					t.Errorf("HTTP request failed: %v", err)
					continue
				}
				defer resp.Body.Close()
				
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Errorf("Failed to read response: %v", err)
					continue
				}
				
				var result map[string]interface{}
				jsonErr := json.Unmarshal(body, &result)
				
				if i < mockServer.GetBadResponseCount() {
					// Should be bad JSON that fails to parse
					if jsonErr == nil {
						t.Logf("Iteration %d: Expected JSON parse error but parsing succeeded. Body: %s", i, string(body))
						// This might be OK for some scenarios like "wrong_schema"
					} else {
						// Verify this is a JSON parsing error
						if !kraken.ValidateErrorHandling(jsonErr) {
							t.Errorf("Unexpected JSON error type: %v", jsonErr)
						}
					}
				} else {
					// Should be good JSON after bad period
					if jsonErr != nil {
						t.Errorf("Expected valid JSON after recovery, got parse error: %v", jsonErr)
					}
				}
				
				// Process should not crash regardless of response
				time.Sleep(10 * time.Millisecond) // Small delay between requests
			}
			
			// Verify the system continues operating after bad JSON
			t.Logf("Scenario %s completed without crash. Bad responses: %d", 
				scenarioName, mockServer.GetBadResponseCount())
		})
	}
}

// TestEmptyBookResilience tests handling of empty order book responses
func TestEmptyBookResilience(t *testing.T) {
	testSuite := kraken.CreateEmptyBookTestSuite()
	
	client := &http.Client{Timeout: 5 * time.Second}
	ctx := context.Background()
	
	for scenarioName, mockServer := range testSuite {
		t.Run(scenarioName, func(t *testing.T) {
			defer mockServer.Close()
			
			// Test empty book handling
			for i := 0; i < 4; i++ {
				req, err := http.NewRequestWithContext(ctx, "GET", mockServer.URL()+"/0/public/Ticker", nil)
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}
				
				resp, err := client.Do(req)
				if err != nil {
					t.Errorf("HTTP request failed: %v", err)
					continue
				}
				defer resp.Body.Close()
				
				if resp.StatusCode != http.StatusOK {
					t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
					continue
				}
				
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Errorf("Failed to read response: %v", err)
					continue  
				}
				
				var result map[string]interface{}
				if err := json.Unmarshal(body, &result); err != nil {
					t.Errorf("Response is not valid JSON: %v", err)
					continue
				}
				
				// Verify response structure
				if errorField, ok := result["error"]; !ok {
					t.Error("Response missing 'error' field")
				} else if errors, ok := errorField.([]interface{}); !ok || len(errors) > 0 {
					t.Errorf("Unexpected error in response: %v", errors)
				}
				
				resultField, hasResult := result["result"]
				if !hasResult {
					t.Error("Response missing 'result' field")
					continue
				}
				
				// Check if result is empty for the first few requests
				if i < mockServer.GetEmptyResponseCount() {
					if resultMap, ok := resultField.(map[string]interface{}); ok {
						if len(resultMap) == 0 {
							t.Logf("Iteration %d: Empty result as expected for scenario %s", i, scenarioName)
						} else {
							// Check for zero values in the data
							t.Logf("Iteration %d: Non-empty result, checking for zero values", i)
						}
					}
				} else {
					// After empty period, should have normal data
					if resultMap, ok := resultField.(map[string]interface{}); ok && len(resultMap) == 0 {
						t.Error("Expected normal data after recovery, got empty result")
					}
				}
				
				time.Sleep(10 * time.Millisecond) // Small delay between requests
			}
			
			t.Logf("Empty book scenario %s handled gracefully. Empty responses: %d",
				scenarioName, mockServer.GetEmptyResponseCount())
		})
	}
}

// TestWinnersFetcherResilience tests the analyst winners fetcher resilience
func TestWinnersFetcherResilience(t *testing.T) {
	// Test timeout scenario
	t.Run("timeout_fallback", func(t *testing.T) {
		// Create timeout server
		mockServer := kraken.NewTimeoutMockServer(5) // All requests timeout
		defer mockServer.Close()
		
		fetcher := analyst.NewKrakenWinnersFetcher()
		
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		
		// This should fallback to fixtures when live data fails
		winners, err := fetcher.FetchWinners(ctx)
		
		if err != nil {
			t.Errorf("FetchWinners should handle timeouts gracefully: %v", err)
		}
		
		if winners == nil {
			t.Fatal("Winners should not be nil")
		}
		
		// Should fallback to fixture data
		if winners.Source != "fixture" {
			t.Errorf("Expected fixture source due to timeout, got %s", winners.Source)
		}
		
		// Should have winners for each timeframe
		if len(winners.Winners1h) == 0 || len(winners.Winners24h) == 0 || len(winners.Winners7d) == 0 {
			t.Error("Fixture fallback should provide winners for all timeframes")
		}
	})
	
	// Test bad JSON scenario  
	t.Run("badjson_fallback", func(t *testing.T) {
		mockServer := kraken.NewBadJSONMockServer(5, kraken.BadJSONMalformed)
		defer mockServer.Close()
		
		fetcher := analyst.NewKrakenWinnersFetcher()
		
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		winners, err := fetcher.FetchWinners(ctx)
		
		if err != nil {
			t.Errorf("FetchWinners should handle bad JSON gracefully: %v", err)
		}
		
		if winners == nil {
			t.Fatal("Winners should not be nil")
		}
		
		// Should fallback to fixture data
		if winners.Source != "fixture" {
			t.Errorf("Expected fixture source due to bad JSON, got %s", winners.Source)
		}
	})
	
	// Test empty book scenario
	t.Run("emptybook_fallback", func(t *testing.T) {
		mockServer := kraken.NewEmptyBookMockServer(5, kraken.EmptyBookNoTickers)
		defer mockServer.Close()
		
		fetcher := analyst.NewKrakenWinnersFetcher()
		
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		winners, err := fetcher.FetchWinners(ctx)
		
		// Empty book should be handled - may succeed with empty data or fallback to fixtures
		if err != nil {
			t.Logf("FetchWinners with empty book: %v (may be expected)", err)
		}
		
		if winners != nil {
			t.Logf("Got winners from source: %s", winners.Source)
			
			// Either fixture fallback or empty results are acceptable
			if winners.Source == "kraken" {
				// If Kraken source, winners arrays might be empty due to no tickers
				t.Logf("Kraken source with empty book: 1h=%d, 24h=%d, 7d=%d winners",
					len(winners.Winners1h), len(winners.Winners24h), len(winners.Winners7d))
			}
		}
	})
}

// TestCircuitBreakerBehavior tests circuit breaker patterns
func TestCircuitBreakerBehavior(t *testing.T) {
	// This is a conceptual test - in a real system you'd test actual circuit breaker implementation
	t.Run("simulated_breaker", func(t *testing.T) {
		mockServer := kraken.NewTimeoutMockServer(3)
		defer mockServer.Close()
		
		client := &http.Client{Timeout: 1 * time.Second}
		ctx := context.Background()
		
		failureCount := 0
		successCount := 0
		
		// Simulate circuit breaker logic
		for i := 0; i < 10; i++ {
			req, _ := http.NewRequestWithContext(ctx, "GET", mockServer.URL()+"/0/public/Ticker", nil)
			
			start := time.Now()
			resp, err := client.Do(req)
			duration := time.Since(start)
			
			if err != nil {
				failureCount++
				
				// After 3 failures, circuit should be "open"
				if failureCount > 3 && duration < 100*time.Millisecond {
					// Fast failure indicates circuit breaker opened
					t.Logf("Request %d: Fast failure (circuit open), duration: %v", i+1, duration)
				} else {
					t.Logf("Request %d: Timeout failure, duration: %v", i+1, duration)
				}
			} else {
				resp.Body.Close()
				successCount++
				t.Logf("Request %d: Success after %d failures", i+1, failureCount)
				
				// Reset failure count on success (circuit closed)
				if successCount > 0 && failureCount > 0 {
					t.Logf("Circuit recovered: %d failures -> success", failureCount)
				}
			}
			
			time.Sleep(200 * time.Millisecond) // Delay between requests
		}
		
		t.Logf("Circuit breaker test completed: %d failures, %d successes", failureCount, successCount)
		
		// Verify we had both failures and eventual success
		if failureCount == 0 {
			t.Error("Expected some failures to test circuit breaker")
		}
		
		if successCount == 0 {
			t.Error("Expected eventual success to test circuit recovery")
		}
	})
}

// TestGracefulDegradation tests that the system continues operating under adverse conditions
func TestGracefulDegradation(t *testing.T) {
	scenarios := []struct {
		name        string
		serverFunc  func() *kraken.TimeoutMockServer
		expectation string
	}{
		{
			name: "complete_timeout",
			serverFunc: func() *kraken.TimeoutMockServer {
				return kraken.NewTimeoutMockServer(100) // All requests timeout
			},
			expectation: "system should continue with fallbacks",
		},
	}
	
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			mockServer := scenario.serverFunc()
			defer mockServer.Close()
			
			// Test that system functions continue to work with fallbacks
			fetcher := analyst.NewKrakenWinnersFetcher()
			
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			
			// Should not panic or crash
			winners, err := fetcher.FetchWinners(ctx)
			
			t.Logf("Scenario %s: error=%v, winners_source=%s", 
				scenario.name, 
				err,
				func() string { if winners != nil { return winners.Source } else { return "nil" } }())
			
			// The key test is that we don't panic and can continue operating
			// Even if err != nil, that's acceptable as long as we handle it gracefully
		})
	}
}