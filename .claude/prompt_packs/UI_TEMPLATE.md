# UI Layer Prompt Pack Template

```
OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS
DOCS MANDATE — UPDATE MD ON EVERY PROMPT
SPEED/BATCH — NO SIMPLIFICATIONS — PROMPT_ID=[YOUR_PROMPT_ID]
WRITE-SCOPE — ALLOW ONLY:
  - internal/ui/**
  - internal/interfaces/http/**
  - tests/integration/ui/**
  - docs/UI_[COMPONENT].md
  - CHANGELOG.md
PATCH-ONLY — Emit unified diffs or complete file blocks. No prose.
TEST-FIRST — Write integration test before UI implementation.
POSTFLIGHT — Test UI manually and check SSE throttling ≤1 Hz.

GOAL
[Specific UI/interface goal, e.g., "Add real-time scoring dashboard with SSE updates"]

SCOPE (Atomic)
[List specific UI components and endpoints to create/modify]

Example:
- Create internal/ui/dashboard/scoring_board.go:
  - NewScoringBoard(updateFreq time.Duration) *ScoringBoard
  - ServeSSE(w http.ResponseWriter, r *http.Request) 
  - UpdateScores(scores map[string]float64)
  - Shutdown() error
- Add internal/interfaces/http/scoring_handler.go:
  - GET /api/scoring/live (SSE endpoint)
  - GET /api/scoring/snapshot (JSON endpoint)
- Add tests/integration/ui/scoring_board_test.go:
  - TestScoringBoard_SSEThrottling
  - TestScoringBoard_ClientDisconnection
- Update docs/UI_DASHBOARD.md with SSE integration

GUARDS
- SSE updates must be throttled to ≤1 Hz
- Graceful client disconnection handling
- No blocking on slow clients
- Proper CORS headers for cross-origin access
- Error boundaries for UI state corruption

ACCEPTANCE
- SSE throttling verified at 1000ms intervals
- Multiple client connections handled without blocking
- Client disconnection cleanup works correctly
- Integration tests pass with real HTTP server
- Manual UI testing shows smooth updates

GIT COMMIT CHECKLIST
1) git add internal/ui/** internal/interfaces/http/** tests/integration/ui/** docs/UI_*.md CHANGELOG.md
2) go test ./tests/integration/ui/... -count=1 -v
3) go build ./internal/ui/... ./internal/interfaces/http/...
4) Manual test: Connect 3 SSE clients, verify ≤1 Hz updates
5) Update PROGRESS.yaml if UI milestone achieved
6) git commit -m "feat(ui): [description] with SSE throttling and integration tests"
7) git push -u origin HEAD
```

## Usage Notes

### SSE Implementation Pattern
```go
// Standard SSE handler with throttling
type SSEHandler struct {
    clients     map[chan []byte]bool
    throttler   *time.Ticker
    mu          sync.RWMutex
}

func (h *SSEHandler) ServeSSE(w http.ResponseWriter, r *http.Request) {
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    
    // Create client channel with buffer
    clientChan := make(chan []byte, 10)
    
    // Register client
    h.mu.Lock()
    h.clients[clientChan] = true
    h.mu.Unlock()
    
    // Stream until disconnect
    defer h.cleanupClient(clientChan)
    
    for {
        select {
        case data := <-clientChan:
            fmt.Fprintf(w, "data: %s\n\n", data)
            w.(http.Flusher).Flush()
        case <-r.Context().Done():
            return
        }
    }
}

// Throttled broadcaster (≤1 Hz)
func (h *SSEHandler) broadcastThrottled() {
    for range h.throttler.C {
        h.mu.RLock()
        clients := make([]chan []byte, 0, len(h.clients))
        for client := range h.clients {
            clients = append(clients, client)
        }
        h.mu.RUnlock()
        
        data := h.getCurrentData()
        for _, client := range clients {
            select {
            case client <- data:
                // Successfully sent
            default:
                // Client blocked, remove it
                h.cleanupClient(client)
            }
        }
    }
}
```

### UI Component Structure Template
```
internal/ui/[COMPONENT]/
├── board.go          # Main UI component
├── sse.go           # Server-Sent Events handler  
├── state.go         # UI state management
└── templates.go     # HTML/template rendering

internal/interfaces/http/
├── [component]_handler.go  # HTTP endpoints
└── middleware/
    ├── cors.go            # CORS handling
    └── rate_limit.go      # Rate limiting

tests/integration/ui/
├── [component]_test.go    # Integration tests
└── fixtures/
    ├── test_data.json
    └── mock_responses.json
```

### Common WRITE-SCOPE Patterns
```
# Single UI component
WRITE-SCOPE — ALLOW ONLY:
  - internal/ui/dashboard/**
  - internal/interfaces/http/dashboard_handler.go
  - tests/integration/ui/dashboard_test.go

# Multi-component UI update
WRITE-SCOPE — ALLOW ONLY:  
  - internal/ui/menu/**
  - internal/ui/board/**
  - tests/integration/ui/**

# Backend + UI integration
WRITE-SCOPE — ALLOW ONLY:
  - internal/ui/scoring/**
  - src/application/scoring/pipeline.go
  - tests/integration/scoring_ui_test.go
```

### SSE Testing Pattern
```go
func TestSSEThrottling(t *testing.T) {
    handler := NewSSEHandler(1000 * time.Millisecond) // 1 Hz
    server := httptest.NewServer(http.HandlerFunc(handler.ServeSSE))
    defer server.Close()
    
    // Connect SSE client
    resp, err := http.Get(server.URL)
    require.NoError(t, err)
    defer resp.Body.Close()
    
    // Count events over 3 seconds
    scanner := bufio.NewScanner(resp.Body)
    events := 0
    timeout := time.After(3 * time.Second)
    
    for {
        select {
        case <-timeout:
            // Should receive ~3 events (1 Hz for 3 seconds)
            assert.InDelta(t, 3, events, 1)
            return
        default:
            if scanner.Scan() && strings.HasPrefix(scanner.Text(), "data:") {
                events++
            }
        }
    }
}
```

### Manual Testing Checklist
```
□ Open browser to SSE endpoint
□ Verify events arrive at ≤1 Hz rate
□ Open multiple tabs, check all receive updates  
□ Close tab, verify server cleans up connection
□ Check browser dev tools for proper SSE headers
□ Verify CORS works for cross-origin requests
□ Test with slow network connection
□ Confirm UI doesn't freeze during updates
```