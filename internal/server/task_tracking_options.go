package server

import (
	"encoding/json"
	"net/http"
)

type TaskTrackingAssigneeOptionsResponse struct {
	Assignees []ExecutionAssigneeOptionDTO `json:"assignees"`
}

// handleGetTaskTrackingAssignees preserves the legacy task-table response while
// delegating identity ownership to the shared delivery directory.
func (s *Server) handleGetTaskTrackingAssignees(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	directory, err := s.loadDeliveryDirectory()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(TaskTrackingAssigneeOptionsResponse{Assignees: directory.Assignees})
}
