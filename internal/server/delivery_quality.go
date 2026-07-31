package server

import (
	"net/http"
	"sync/atomic"
	"well-ambient/internal/db"
)

const (
	legacyTasksReadEndpoint     = "GET /api/tasks"
	legacyScheduleReadEndpoint  = "GET /api/schedule"
	legacyScheduleWriteEndpoint = "POST /api/tasks/schedule"
)

var (
	legacyTasksReadCount      atomic.Uint64
	legacyScheduleReadCount   atomic.Uint64
	legacyScheduleWriteCount  atomic.Uint64
	planningRevisionConflicts atomic.Uint64
)

type deliveryQualityMetrics struct {
	WorkItemsWithoutProject        int64             `json:"work_items_without_project"`
	CommittedWithoutPrimaryRelease int64             `json:"committed_without_primary_release"`
	AmbiguousTargetRelease         int64             `json:"ambiguous_target_release"`
	OrphanExecutionTasks           int64             `json:"orphan_execution_tasks"`
	JiraVersionSyncFailed          int64             `json:"jira_version_sync_failed"`
	PlanningRevisionConflict       uint64            `json:"planning_revision_conflict"`
	ReleasedScopeChanged           int64             `json:"released_scope_changed"`
	ProjectCompatibilityFallback   uint64            `json:"project_compatibility_fallback"`
	LegacyEndpointCalls            map[string]uint64 `json:"legacy_endpoint_calls"`
}

func (s *Server) withLegacyDeliveryAPI(endpoint string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch endpoint {
		case legacyTasksReadEndpoint:
			legacyTasksReadCount.Add(1)
		case legacyScheduleReadEndpoint:
			legacyScheduleReadCount.Add(1)
		case legacyScheduleWriteEndpoint:
			legacyScheduleWriteCount.Add(1)
		}
		w.Header().Set("Deprecation", "true")
		w.Header().Set("X-Well-Ambient-Compatibility-Endpoint", endpoint)
		next(w, r)
	}
}

func (s *Server) handleGetDeliveryQuality(w http.ResponseWriter, r *http.Request) {
	metrics, err := loadDeliveryQualityMetrics()
	if err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "quality_metrics_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, metrics)
}

func loadDeliveryQualityMetrics() (deliveryQualityMetrics, error) {
	metrics := deliveryQualityMetrics{
		PlanningRevisionConflict:     planningRevisionConflicts.Load(),
		ProjectCompatibilityFallback: db.TaskProjectCompatibilityFallbackCount(),
		LegacyEndpointCalls: map[string]uint64{
			legacyTasksReadEndpoint:     legacyTasksReadCount.Load(),
			legacyScheduleReadEndpoint:  legacyScheduleReadCount.Load(),
			legacyScheduleWriteEndpoint: legacyScheduleWriteCount.Load(),
		},
	}
	workItemKinds := []string{"requirement", "demand", "story", "bug", "defect", "缺陷", "故障"}
	executionKinds := []string{"execution_task", "execution-task", "task", "sub-task", "subtask"}

	if err := db.DB.Model(&db.TaskTelemetry{}).
		Where("LOWER(issue_type) IN ? AND TRIM(project_key) = ?", workItemKinds, "").
		Count(&metrics.WorkItemsWithoutProject).Error; err != nil {
		return deliveryQualityMetrics{}, err
	}
	if err := db.DB.Model(&db.TaskTelemetry{}).
		Where("LOWER(issue_type) IN ? AND planning_state = ?", workItemKinds, "committed").
		Where(`NOT EXISTS (
			SELECT 1 FROM work_item_release_links
			WHERE work_item_release_links.work_item_id = task_telemetries.task_id
			  AND work_item_release_links.relation = 'target_fix'
			  AND work_item_release_links.active = 1
			  AND work_item_release_links.is_primary = 1
		)`).
		Count(&metrics.CommittedWithoutPrimaryRelease).Error; err != nil {
		return deliveryQualityMetrics{}, err
	}
	if err := db.DB.Raw(`SELECT COUNT(*) FROM (
		SELECT work_item_id
		FROM work_item_release_links
		WHERE relation = 'target_fix' AND active = 1
		GROUP BY work_item_id
		HAVING COUNT(*) > 1 AND SUM(CASE WHEN is_primary = 1 THEN 1 ELSE 0 END) = 0
	) AS ambiguous`).Scan(&metrics.AmbiguousTargetRelease).Error; err != nil {
		return deliveryQualityMetrics{}, err
	}
	if err := db.DB.Model(&db.TaskTelemetry{}).
		Where("LOWER(issue_type) IN ? AND TRIM(parent_work_item_id) = ?", executionKinds, "").
		Count(&metrics.OrphanExecutionTasks).Error; err != nil {
		return deliveryQualityMetrics{}, err
	}
	if err := db.DB.Model(&db.WorkItemSyncOperation{}).
		Where("status = ?", "failed").
		Count(&metrics.JiraVersionSyncFailed).Error; err != nil {
		return deliveryQualityMetrics{}, err
	}
	if err := db.DB.Model(&db.WorkItemEvent{}).
		Where("event_type = ? OR reason = ?", "released_scope_changed", "released_scope_changed").
		Count(&metrics.ReleasedScopeChanged).Error; err != nil {
		return deliveryQualityMetrics{}, err
	}
	return metrics, nil
}

func resetDeliveryQualityCounters() {
	legacyTasksReadCount.Store(0)
	legacyScheduleReadCount.Store(0)
	legacyScheduleWriteCount.Store(0)
	planningRevisionConflicts.Store(0)
}
