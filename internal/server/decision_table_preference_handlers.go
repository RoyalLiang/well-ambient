package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"well-ambient/internal/db"
)

const decisionAgendaTableKey = "decision_agenda"

var decisionAgendaDefaultColumns = []string{"task_id", "title", "owner", "risk", "due", "status"}

type decisionTableColumnPreferenceResponse struct {
	VisibleColumns []string `json:"visible_columns"`
}

type updateDecisionTableColumnPreferenceRequest struct {
	VisibleColumns []string `json:"visible_columns"`
}

func canonicalDecisionAgendaColumns(columns []string) ([]string, error) {
	requested := make(map[string]struct{}, len(columns))
	allowed := make(map[string]struct{}, len(decisionAgendaDefaultColumns))
	for _, column := range decisionAgendaDefaultColumns {
		allowed[column] = struct{}{}
	}
	for _, column := range columns {
		column = strings.TrimSpace(column)
		if _, exists := allowed[column]; !exists {
			return nil, fmt.Errorf("unknown column: %s", column)
		}
		requested[column] = struct{}{}
	}
	if _, exists := requested["task_id"]; !exists {
		return nil, fmt.Errorf("task_id column is required")
	}
	result := make([]string, 0, len(requested))
	for _, column := range decisionAgendaDefaultColumns {
		if _, exists := requested[column]; exists {
			result = append(result, column)
		}
	}
	return result, nil
}

func (s *Server) handleGetDecisionTableColumns(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.Header.Get("x-authenticated-user-id"))
	columns, found, err := db.LoadUserTablePreference(db.DB, username, decisionAgendaTableKey)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load table preference: %v", err), http.StatusInternalServerError)
		return
	}
	if !found {
		columns = append([]string(nil), decisionAgendaDefaultColumns...)
	}
	canonical, err := canonicalDecisionAgendaColumns(columns)
	if err != nil {
		canonical = append([]string(nil), decisionAgendaDefaultColumns...)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(decisionTableColumnPreferenceResponse{VisibleColumns: canonical})
}

func (s *Server) handleUpdateDecisionTableColumns(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.Header.Get("x-authenticated-user-id"))
	if username == "" {
		http.Error(w, "Missing authenticated user", http.StatusUnauthorized)
		return
	}
	var req updateDecisionTableColumnPreferenceRequest
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	columns, err := canonicalDecisionAgendaColumns(req.VisibleColumns)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := db.SaveUserTablePreference(db.DB, username, decisionAgendaTableKey, columns); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save table preference: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(decisionTableColumnPreferenceResponse{VisibleColumns: columns})
}
