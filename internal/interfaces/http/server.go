package http

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"cryptorun/internal/interfaces/http/handlers"
)

// Server represents the read-only HTTP server
type Server struct {
	router   *mux.Router
	server   *http.Server
	handlers *handlers.Handlers
	config   ServerConfig
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// DefaultServerConfig returns default server configuration
func DefaultServerConfig() ServerConfig {
	port := 8080
	if portStr := os.Getenv("HTTP_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	return ServerConfig{
		Host:         "127.0.0.1", // Local-only by default
		Port:         port,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// NewServer creates a new HTTP server instance
func NewServer(config ServerConfig) (*Server, error) {
	// Check if port is available
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("port %d is busy or unavailable: %w", config.Port, err)
	}
	listener.Close()

	router := mux.NewRouter()

	// Initialize handlers
	handlerManager := handlers.NewHandlers()

	server := &Server{
		router:   router,
		handlers: handlerManager,
		config:   config,
	}

	// Setup routes
	server.setupRoutes()

	// Create HTTP server
	server.server = &http.Server{
		Addr:         addr,
		Handler:      server.router,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	return server, nil
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Middleware for all routes
	s.router.Use(s.requestLoggingMiddleware)
	s.router.Use(s.requestIDMiddleware)
	s.router.Use(s.timeoutMiddleware)
	s.router.Use(s.corsMiddleware)

	// API routes (JSON only)
	api := s.router.PathPrefix("/").Subrouter()
	api.Use(s.jsonContentTypeMiddleware)

	// Health endpoint
	api.HandleFunc("/health", s.handlers.Health).Methods("GET")

	// Candidates endpoint with pagination
	api.HandleFunc("/candidates", s.handlers.Candidates).Methods("GET")

	// Explain endpoint for symbol analysis
	api.HandleFunc("/explain/{symbol}", s.handlers.Explain).Methods("GET")

	// Regime endpoint for market regime data
	api.HandleFunc("/regime", s.handlers.Regime).Methods("GET")

	// 404 handler
	s.router.NotFoundHandler = http.HandlerFunc(s.handlers.NotFound)
}

// requestIDMiddleware adds unique request ID to each request
func (s *Server) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()[:8]
		ctx := context.WithValue(r.Context(), "request_id", requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requestLoggingMiddleware logs all requests with structured format
func (s *Server) requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := r.Context().Value("request_id")

		// Capture response status
		wrapper := &responseWrapper{ResponseWriter: w, statusCode: 200}
		next.ServeHTTP(wrapper, r)

		duration := time.Since(start)

		log.Printf("REQ %s %s %s %d %v %s",
			requestID,
			r.Method,
			r.URL.Path,
			wrapper.statusCode,
			duration,
			r.RemoteAddr,
		)
	})
}

// timeoutMiddleware enforces request timeouts
func (s *Server) timeoutMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// corsMiddleware adds CORS headers for local development
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only allow localhost origins
		origin := r.Header.Get("Origin")
		if strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// jsonContentTypeMiddleware sets JSON content type for API responses
func (s *Server) jsonContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Printf("Starting HTTP server on %s:%d (local-only, read-only)",
		s.config.Host, s.config.Port)

	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Printf("Shutting down HTTP server...")
	return s.server.Shutdown(ctx)
}

// GetAddress returns the server address
func (s *Server) GetAddress() string {
	return fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
}

// responseWrapper captures HTTP status codes for logging
type responseWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
