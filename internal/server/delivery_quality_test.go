package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"well-ambient/internal/db"
)

func TestLegacyDeliveryEndpointIsObservableAndDeprecated(t *testing.T) {
	resetDeliveryQualityCounters()
	handler := (&Server{}).withLegacyDeliveryAPI(legacyScheduleReadEndpoint, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodGet, "/api/schedule", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Deprecation"); got != "true" {
		t.Fatalf("Deprecation = %q, want true", got)
	}
	if got := legacyScheduleReadCount.Load(); got != 1 {
		t.Fatalf("legacy schedule read count = %d, want 1", got)
	}
}

func TestDeliveryQualityMetricsExposeConvergenceGates(t *testing.T) {
	setupServerTestDB(t)
	resetDeliveryQualityCounters()
	db.ResetTaskProjectCompatibilityFallbackCount()

	records := []db.TaskTelemetry{
		{TaskID: "REQ-1", IssueType: "requirement", PlanningState: "committed"},
		{TaskID: "TASK-1", IssueType: "execution_task", ProjectKey: "HIT"},
	}
	if err := db.DB.Create(&records).Error; err != nil {
		t.Fatalf("create tasks: %v", err)
	}
	operation := db.WorkItemSyncOperation{
		IdempotencyKey: "quality-failed-1",
		WorkItemID:     "REQ-1",
		Operation:      "sync_planning",
		PayloadJSON:    "{}",
		Status:         "failed",
	}
	if err := db.DB.Create(&operation).Error; err != nil {
		t.Fatalf("create failed sync: %v", err)
	}
	legacyScheduleWriteCount.Add(2)
	planningRevisionConflicts.Add(1)

	metrics, err := loadDeliveryQualityMetrics()
	if err != nil {
		t.Fatalf("load quality metrics: %v", err)
	}
	if metrics.WorkItemsWithoutProject != 1 ||
		metrics.CommittedWithoutPrimaryRelease != 1 ||
		metrics.OrphanExecutionTasks != 1 ||
		metrics.JiraVersionSyncFailed != 1 ||
		metrics.PlanningRevisionConflict != 1 {
		t.Fatalf("unexpected quality metrics: %+v", metrics)
	}
	if metrics.LegacyEndpointCalls[legacyScheduleWriteEndpoint] != 2 {
		t.Fatalf("legacy write calls = %d, want 2", metrics.LegacyEndpointCalls[legacyScheduleWriteEndpoint])
	}
}
