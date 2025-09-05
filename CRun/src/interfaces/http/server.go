package httpiface

import (
    "context"
    "encoding/json"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
)

// Start launches a simple health/metrics/decile HTTP server.
// Endpoints:
//  - /health  -> { ok: true, time: RFC3339 }
//  - /metrics -> {}
//  - /decile  -> { count: [0..], average: [0..] }
func Start(addr string) *http.Server {
    mux := http.NewServeMux()
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request){
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{
            "ok": true,
            "time": time.Now().UTC().Format(time.RFC3339),
        })
    })
    mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request){
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{})
    })
    mux.HandleFunc("/decile", func(w http.ResponseWriter, r *http.Request){
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{
            "count":   [10]int{},
            "average": [10]float64{},
        })
    })
    srv := &http.Server{Addr: addr, Handler: mux}
    go func(){ _ = srv.ListenAndServe() }()
    return srv
}

// RunUntilSignal starts the server and blocks until SIGINT/SIGTERM, then shuts down.
func RunUntilSignal(addr string) error {
    srv := Start(addr)
    sig := make(chan os.Signal, 1)
    signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
    <-sig
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()
    return srv.Shutdown(ctx)
}

