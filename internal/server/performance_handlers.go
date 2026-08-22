package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"well-ambient/internal/performance"

	"gorm.io/gorm"
)

func (s *Server) handleGetPerformanceExplanation(w http.ResponseWriter, r *http.Request) {
	limit := 30
	if raw := strings.TrimSpace(r.URL.Query().Get("snapshot_limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 100 {
			http.Error(w, "snapshot_limit must be between 1 and 100", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	explanation, err := s.performance.Explain(r.Context(), limit)
	if err != nil {
		http.Error(w, "failed to load performance calculation explanation", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, explanation)
}

func (s *Server) handleGetPerformanceSnapshot(w http.ResponseWriter, r *http.Request) {
	rawID := strings.TrimSpace(r.PathValue("id"))
	parsedID, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || parsedID == 0 {
		http.Error(w, "invalid performance snapshot id", http.StatusBadRequest)
		return
	}
	detail, err := s.performance.ExplainSnapshot(r.Context(), uint(parsedID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		http.Error(w, "performance snapshot not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to load performance snapshot", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAppendPerformanceEvidence(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var command performance.EvidenceCommand
	if err := decoder.Decode(&command); err != nil {
		http.Error(w, "invalid performance evidence request", http.StatusBadRequest)
		return
	}
	if err := ensureJSONEOF(decoder); err != nil {
		http.Error(w, "invalid performance evidence request", http.StatusBadRequest)
		return
	}
	command.CreatedBy = authenticatedActor(r)
	result, err := s.performance.AppendEvidence(r.Context(), command)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("multiple JSON values are not allowed")
	}
	return err
}
