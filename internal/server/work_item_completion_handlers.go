package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"well-ambient/internal/db"
	"well-ambient/internal/deliveryplanning"
	"well-ambient/internal/telemetry"

	"gorm.io/gorm"
)

type workItemCompletionRequest struct {
	ExpectedRevision uint   `json:"expected_revision"`
	Reason           string `json:"reason"`
}

func (s *Server) handleCompleteWorkItem(w http.ResponseWriter, r *http.Request) {
	var request workItemCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeDeliveryError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	workItemID := strings.TrimSpace(r.PathValue("id"))
	var task db.TaskTelemetry
	if err := db.DB.WithContext(r.Context()).Where("task_id = ?", workItemID).First(&task).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		writeDeliveryError(w, http.StatusNotFound, "work_item_not_found", "work item was not found")
		return
	} else if err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "work_item_query_failed", err.Error())
		return
	}
	if !requestCanAccessWorkItem(r, task) {
		writeDeliveryError(w, http.StatusNotFound, "work_item_not_found", "work item was not found")
		return
	}

	actorID := strings.TrimSpace(r.Header.Get("x-authenticated-user-id"))
	actorName := strings.TrimSpace(r.Header.Get("x-authenticated-user-name"))
	if actorName == "" {
		actorName = actorID
	}
	isAssignee := strings.EqualFold(strings.TrimSpace(task.Assignee), actorID) ||
		strings.EqualFold(strings.TrimSpace(task.Assignee), actorName)
	if !s.checkIsAdmin(actorID) && !isAssignee {
		writeDeliveryError(w, http.StatusForbidden, "completion_forbidden", "only the assignee or a manager can confirm completion")
		return
	}

	result, err := deliveryplanning.NewService(db.DB).CompleteWorkItem(r.Context(), deliveryplanning.CompletionCommand{
		WorkItemID:       workItemID,
		ExpectedRevision: request.ExpectedRevision,
		Actor:            actorID,
		Reason:           request.Reason,
		Source:           "manual_evidence",
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	BroadcastNotifications()
	BroadcastTelemetryUpdated(workItemID)
	telemetry.SyncStatusToJira(s.config, workItemID, "done")
	writeJSON(w, http.StatusOK, result)
}
