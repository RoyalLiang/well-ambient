package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"well-ambient/internal/db"
	"well-ambient/internal/deliveryplanning"

	"gorm.io/gorm"
)

type workItemPlanningRequest struct {
	ExpectedRevision       uint    `json:"expected_revision"`
	ProjectKey             *string `json:"project_key"`
	PrimaryTargetReleaseID *uint   `json:"primary_target_release_id"`
	AffectedReleaseIDs     *[]uint `json:"affected_release_ids"`
	DueDate                *string `json:"due_date"`
	Assignee               *string `json:"assignee"`
	PlanningState          *string `json:"planning_state"`
	Reason                 string  `json:"reason"`
}

type bulkWorkItemPlanningRequest struct {
	Items []struct {
		WorkItemID string `json:"work_item_id"`
		workItemPlanningRequest
	} `json:"items"`
}

func (s *Server) handleListWorkItems(w http.ResponseWriter, r *http.Request) {
	projectKeys := queryFilterValues(r, "project")
	preferenceKeys, err := requestProjectPreferenceKeys(r)
	if err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "project_scope_failed", err.Error())
		return
	}
	projectKeys = intersectProjectKeys(projectKeys, preferenceKeys)
	if len(projectKeys) == 0 && len(preferenceKeys) > 0 && len(queryFilterValues(r, "project")) > 0 {
		writeJSON(w, http.StatusOK, deliveryplanning.PlanSnapshot{Items: []deliveryplanning.WorkItemSnapshot{}, Limit: 100})
		return
	}
	releaseID, _ := strconv.ParseUint(strings.TrimSpace(r.URL.Query().Get("release_id")), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	snapshot, err := deliveryplanning.NewService(db.DB).QueryPlan(r.Context(), deliveryplanning.PlanQuery{
		ProjectKeys:   projectKeys,
		Assignees:     queryFilterValues(r, "assignee"),
		ReleaseID:     uint(releaseID),
		Kinds:         queryFilterValues(r, "kind"),
		Statuses:      queryFilterValues(r, "status"),
		PlanningState: queryFilterValues(r, "planning_state"),
		Search:        r.URL.Query().Get("search"),
		ActiveOnly:    strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("active")), "true"),
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleGetWorkItem(w http.ResponseWriter, r *http.Request) {
	workItemID := strings.TrimSpace(r.PathValue("id"))
	snapshot, err := deliveryplanning.NewService(db.DB).QueryWorkItem(r.Context(), workItemID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeDeliveryError(w, http.StatusNotFound, "work_item_not_found", "work item was not found")
		return
	}
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if !requestCanAccessWorkItem(r, snapshot.WorkItem) {
		writeDeliveryError(w, http.StatusNotFound, "work_item_not_found", "work item was not found")
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handlePatchWorkItemPlanning(w http.ResponseWriter, r *http.Request) {
	var request workItemPlanningRequest
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
	command, err := planningCommandFromRequest(workItemID, request, r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	snapshot, err := deliveryplanning.NewService(db.DB).ApplyPlanningChange(r.Context(), command)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleBulkWorkItemPlanning(w http.ResponseWriter, r *http.Request) {
	var request bulkWorkItemPlanningRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeDeliveryError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	if len(request.Items) == 0 || len(request.Items) > 100 {
		writeDeliveryError(w, http.StatusUnprocessableEntity, "invalid_bulk_size", "bulk planning requires between 1 and 100 items")
		return
	}
	results := make([]deliveryplanning.WorkItemSnapshot, 0, len(request.Items))
	err := db.DB.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		service := deliveryplanning.NewService(tx)
		for _, item := range request.Items {
			workItemID := strings.TrimSpace(item.WorkItemID)
			var task db.TaskTelemetry
			if err := tx.Where("task_id = ?", workItemID).First(&task).Error; err != nil {
				return err
			}
			if !requestCanAccessWorkItem(r, task) {
				return gorm.ErrRecordNotFound
			}
			command, err := planningCommandFromRequest(workItemID, item.workItemPlanningRequest, r)
			if err != nil {
				return err
			}
			snapshot, err := service.ApplyPlanningChange(r.Context(), command)
			if err != nil {
				return err
			}
			results = append(results, snapshot)
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeDeliveryError(w, http.StatusNotFound, "work_item_not_found", "one or more work items were not found")
		return
	}
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": results, "total": len(results), "atomic": true})
}

func (s *Server) handleGetReleaseSnapshot(w http.ResponseWriter, r *http.Request) {
	releaseID, err := parseReleaseID(r.PathValue("id"))
	if err != nil {
		writeDeliveryError(w, http.StatusBadRequest, "invalid_release_id", err.Error())
		return
	}
	snapshot, err := deliveryplanning.NewService(db.DB).QueryRelease(r.Context(), deliveryplanning.ReleaseQuery{ReleaseID: releaseID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeDeliveryError(w, http.StatusNotFound, "release_not_found", "release was not found")
		return
	}
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

type deliveryExceptionDTO struct {
	Type       string `json:"type"`
	WorkItemID string `json:"work_item_id,omitempty"`
	ProjectKey string `json:"project_key,omitempty"`
	Message    string `json:"message"`
	Revision   uint   `json:"revision,omitempty"`
	SyncState  string `json:"sync_state,omitempty"`
	OccurredAt string `json:"occurred_at,omitempty"`
}

func (s *Server) handleGetDeliveryExceptions(w http.ResponseWriter, r *http.Request) {
	var exceptions []deliveryExceptionDTO
	var withoutProject []db.TaskTelemetry
	if err := db.DB.WithContext(r.Context()).
		Where("LOWER(issue_type) IN ? AND TRIM(project_key) = ?", []string{"requirement", "demand", "story", "bug", "defect", "缺陷", "故障"}, "").
		Find(&withoutProject).Error; err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "exception_query_failed", err.Error())
		return
	}
	for _, task := range withoutProject {
		exceptions = append(exceptions, deliveryExceptionDTO{
			Type:       "work_item_without_project",
			WorkItemID: task.TaskID,
			Message:    "交付项尚未确认项目",
			Revision:   task.Revision,
		})
	}
	var orphanTasks []db.TaskTelemetry
	if err := db.DB.WithContext(r.Context()).
		Where("LOWER(issue_type) IN ? AND TRIM(parent_work_item_id) = ?",
			[]string{"execution_task", "execution-task", "task", "sub-task", "subtask"}, "").
		Find(&orphanTasks).Error; err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "exception_query_failed", err.Error())
		return
	}
	for _, task := range orphanTasks {
		exceptions = append(exceptions, deliveryExceptionDTO{
			Type:       "orphan_execution_task",
			WorkItemID: task.TaskID,
			ProjectKey: task.ProjectKey,
			Message:    "执行任务尚未绑定父交付项",
			Revision:   task.Revision,
		})
	}
	var events []db.WorkItemEvent
	if err := db.DB.WithContext(r.Context()).
		Where("event_type IN ? OR sync_state IN ?", []string{"release_scope_conflict_detected", "external_sync_failed"}, []string{"conflict", "failed"}).
		Order("created_at DESC").Limit(500).Find(&events).Error; err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "exception_query_failed", err.Error())
		return
	}
	for _, event := range events {
		exceptions = append(exceptions, deliveryExceptionDTO{
			Type:       event.Reason,
			WorkItemID: event.WorkItemID,
			ProjectKey: event.ProjectKey,
			Message:    deliveryExceptionMessage(event.Reason),
			Revision:   event.Revision,
			SyncState:  event.SyncState,
			OccurredAt: event.CreatedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": exceptions, "total": len(exceptions)})
}

func planningCommandFromRequest(workItemID string, request workItemPlanningRequest, r *http.Request) (deliveryplanning.PlanningCommand, error) {
	var dueDate **time.Time
	if request.DueDate != nil {
		value := strings.TrimSpace(*request.DueDate)
		var parsed *time.Time
		if value != "" {
			date, err := time.Parse("2006-01-02", value)
			if err != nil {
				return deliveryplanning.PlanningCommand{}, &deliveryplanning.DomainError{
					Code: "invalid_due_date", Message: "due_date must use YYYY-MM-DD", StatusCode: 422,
				}
			}
			parsed = &date
		}
		dueDate = &parsed
	}
	actor := strings.TrimSpace(r.Header.Get("x-authenticated-user-id"))
	if actor == "" {
		actor = strings.TrimSpace(r.Header.Get("x-authenticated-user-name"))
	}
	return deliveryplanning.PlanningCommand{
		WorkItemID:             workItemID,
		ExpectedRevision:       request.ExpectedRevision,
		ProjectKey:             request.ProjectKey,
		PrimaryTargetReleaseID: request.PrimaryTargetReleaseID,
		AffectedReleaseIDs:     request.AffectedReleaseIDs,
		DueDate:                dueDate,
		Assignee:               request.Assignee,
		PlanningState:          request.PlanningState,
		Reason:                 request.Reason,
		Actor:                  actor,
		Source:                 "manual",
	}, nil
}

func requestCanAccessWorkItem(r *http.Request, task db.TaskTelemetry) bool {
	keys, err := requestProjectPreferenceKeys(r)
	if err != nil {
		return false
	}
	return db.TaskMatchesExplicitProjectScope(task.ProjectKey, task.TaskID, keys)
}

func intersectProjectKeys(requested, allowed []string) []string {
	requested = db.NormalizeProjectKeys(requested)
	allowed = db.NormalizeProjectKeys(allowed)
	if len(allowed) == 0 {
		return requested
	}
	if len(requested) == 0 {
		return allowed
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	result := make([]string, 0, len(requested))
	for _, key := range requested {
		if _, ok := allowedSet[key]; ok {
			result = append(result, key)
		}
	}
	return result
}

func deliveryExceptionMessage(code string) string {
	switch code {
	case "ambiguous_target_release":
		return "Jira 存在多个目标版本，需要人工确认主版本"
	case "external_primary_release_drift":
		return "Jira 已移除人工确认的主版本，本地决定被保留"
	case "external_project_conflict":
		return "Jira 项目与本地确认项目不一致"
	default:
		return code
	}
}

func writeDomainError(w http.ResponseWriter, err error) {
	var domainErr *deliveryplanning.DomainError
	if errors.As(err, &domainErr) {
		if domainErr.Code == "revision_conflict" {
			planningRevisionConflicts.Add(1)
		}
		status := domainErr.StatusCode
		if status == 0 {
			status = http.StatusUnprocessableEntity
		}
		writeDeliveryError(w, status, domainErr.Code, domainErr.Message)
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeDeliveryError(w, http.StatusNotFound, "not_found", "requested resource was not found")
		return
	}
	writeDeliveryError(w, http.StatusInternalServerError, "delivery_planning_failed", err.Error())
}
