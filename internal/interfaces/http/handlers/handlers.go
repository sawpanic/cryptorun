package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	httpContracts "github.com/sawpanic/cryptorun/internal/http"
)

// Handlers manages all HTTP endpoint handlers
type Handlers struct {
	// Add dependencies here when available (regime detector, candidate manager, etc.)
}

// NewHandlers creates a new handlers instance
func NewHandlers() *Handlers {
	return &Handlers{}
}

// writeJSON writes JSON response with proper error handling
func (h *Handlers) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Fallback error response
		http.Error(w, `{"error":"json_encoding_failed"}`, http.StatusInternalServerError)
	}
}

// writeError writes standardized error response
func (h *Handlers) writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	requestID := r.Context().Value("request_id")
	if requestID == nil {
		requestID = "unknown"
	}

	errorResp := httpContracts.ErrorResponse{
		Error:     http.StatusText(status),
		Message:   message,
		Code:      code,
		RequestID: requestID.(string),
		Timestamp: time.Now().UTC(),
	}

	h.writeJSON(w, status, errorResp)
}

// NotFound handles 404 responses
func (h *Handlers) NotFound(w http.ResponseWriter, r *http.Request) {
	h.writeError(w, r, http.StatusNotFound, "endpoint_not_found",
		"The requested endpoint does not exist")
}
