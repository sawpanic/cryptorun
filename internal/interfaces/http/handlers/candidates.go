package handlers

import (
	"net/http"
	"strconv"
	"time"

	httpContracts "cryptorun/internal/http"
)

// Candidates handles GET /candidates endpoint with pagination
func (h *Handlers) Candidates(w http.ResponseWriter, r *http.Request) {
	// Parse pagination parameters
	n := 50   // default
	page := 1 // default

	if nStr := r.URL.Query().Get("n"); nStr != "" {
		if parsed, err := strconv.Atoi(nStr); err == nil && parsed > 0 && parsed <= 100 {
			n = parsed
		}
	}

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if parsed, err := strconv.Atoi(pageStr); err == nil && parsed > 0 {
			page = parsed
		}
	}

	// Mock candidate data - in real implementation, this would query the candidate manager
	mockCandidates := generateMockCandidates(n, page)

	total := 250 // Mock total count
	hasNext := (page * n) < total
	hasPrev := page > 1

	response := httpContracts.CandidatesResponse{
		Candidates: mockCandidates,
		Pagination: httpContracts.PaginationInfo{
			Total:    total,
			Page:     page,
			PageSize: n,
			HasNext:  hasNext,
			HasPrev:  hasPrev,
		},
		Generated: time.Now().UTC(),
	}

	h.writeJSON(w, http.StatusOK, response)
}

// generateMockCandidates creates mock candidate data for testing
func generateMockCandidates(n, page int) []httpContracts.CandidateInfo {
	candidates := make([]httpContracts.CandidateInfo, 0, n)

	// Mock symbols
	symbols := []string{"BTCUSD", "ETHUSD", "SOLUSD", "ADAUSD", "DOTUSD",
		"AVAXUSD", "MATICUSD", "ATOMUSD", "NEARUSD", "FILUSD"}

	startRank := (page-1)*n + 1

	for i := 0; i < n; i++ {
		symbolIdx := (startRank + i - 1) % len(symbols)
		rank := startRank + i

		// Mock decreasing composite scores
		baseScore := 95.0 - float64(rank-1)*0.3
		if baseScore < 0 {
			baseScore = 0
		}

		candidate := httpContracts.CandidateInfo{
			Symbol:         symbols[symbolIdx],
			CompositeScore: baseScore,
			Rank:           rank,
			GateStatus: map[string]bool{
				"score_gate":     baseScore >= 75.0,
				"vadr_gate":      true,
				"funding_gate":   rank <= 30,
				"freshness_gate": true,
				"fatigue_gate":   rank <= 50,
				"late_fill_gate": true,
			},
			Factors: map[string]float64{
				"momentum_core":      baseScore * 0.4,
				"technical_residual": (baseScore - 50) * 0.3,
				"volume_residual":    (baseScore - 50) * 0.2,
				"quality_residual":   (baseScore - 50) * 0.1,
				"social_residual":    min(10.0, baseScore*0.1),
			},
			Timestamp: time.Now().UTC(),
		}

		candidates = append(candidates, candidate)
	}

	return candidates
}

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
