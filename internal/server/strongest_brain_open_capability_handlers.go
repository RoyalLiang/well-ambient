package server

import (
	"net/http"
	"time"

	"well-ambient/internal/openaccess"
)

func (s *Server) handleGetStrongestBrainOpenCapabilityIntelligence(w http.ResponseWriter, r *http.Request) {
	if s.openAccess == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "open capability intelligence is unavailable"})
		return
	}
	report, err := s.openAccess.Intelligence(r.Context(), 30*24*time.Hour)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": openaccess.ErrorCode(err),
		})
		return
	}
	writeJSON(w, http.StatusOK, report)
}
