package deliveryplanning

import (
	"context"
	"testing"
	"time"
	"well-ambient/internal/db"
)

func TestCompleteWorkItemUsesCaseInsensitiveEvidenceWithoutReleaseCommitment(t *testing.T) {
	conn := openPlanningTestDB(t)
	task := db.TaskTelemetry{
		TaskID:        "FZ-2247",
		ExternalKey:   "FZ-2247",
		ProjectKey:    "FZ",
		Source:        "jira",
		IssueType:     "requirement",
		Status:        "progress",
		PlanningState: PlanningDraft,
	}
	if err := conn.Create(&task).Error; err != nil {
		t.Fatalf("seed work item: %v", err)
	}
	logs := []db.GitCommitLog{
		{TaskID: "fz-2247", Repo: "task_executor", CommitID: "task-sha", Action: "git_push", CreatedAt: time.Now().Add(-time.Minute)},
		{TaskID: "FZ-2247", Repo: "crane_manager", CommitID: "crane-sha", Action: "git_push", CreatedAt: time.Now()},
	}
	if err := conn.Create(&logs).Error; err != nil {
		t.Fatalf("seed completion evidence: %v", err)
	}

	service := NewService(conn)
	result, err := service.CompleteWorkItem(context.Background(), CompletionCommand{
		WorkItemID: task.TaskID, ExpectedRevision: 0, Actor: "delivery-owner", Reason: "confirmed exact commit evidence",
	})
	if err != nil {
		t.Fatalf("complete work item: %v", err)
	}
	if result.Snapshot.WorkItem.Status != "done" || result.Snapshot.WorkItem.CompletedAt == nil {
		t.Fatalf("completion snapshot = %+v", result.Snapshot.WorkItem)
	}
	if result.Snapshot.WorkItem.PlanningState != PlanningDone || len(result.Snapshot.Links) != 0 {
		t.Fatalf("completion changed release planning: %+v", result.Snapshot)
	}
	if result.Evidence.CommitCount != 2 || len(result.Evidence.Repositories) != 2 ||
		result.Evidence.Repositories[0] != "crane_manager" || result.Evidence.Repositories[1] != "task_executor" {
		t.Fatalf("completion evidence = %+v", result.Evidence)
	}

	// A repeated confirmation is idempotent even when the caller still holds the
	// pre-completion revision.
	if _, err := service.CompleteWorkItem(context.Background(), CompletionCommand{
		WorkItemID: task.TaskID, ExpectedRevision: 0, Actor: "delivery-owner", Reason: "retry",
	}); err != nil {
		t.Fatalf("repeat completion: %v", err)
	}
	var eventCount, assetCount int64
	conn.Model(&db.WorkItemEvent{}).Where("work_item_id = ? AND event_type = ?", task.TaskID, WorkItemExecutionCompleted).Count(&eventCount)
	conn.Model(&db.DataAssetEvent{}).Where("subject_id = ? AND event_type = ?", task.TaskID, WorkItemExecutionCompleted).Count(&assetCount)
	if eventCount != 1 || assetCount != 1 {
		t.Fatalf("completion audit/asset counts = %d/%d, want 1/1", eventCount, assetCount)
	}
}

func TestCompleteWorkItemRequiresExactGitEvidence(t *testing.T) {
	conn := openPlanningTestDB(t)
	task := db.TaskTelemetry{
		TaskID: "FZ-2248", ExternalKey: "FZ-2248", ProjectKey: "FZ", Source: "jira",
		IssueType: "bug", Status: "progress", PlanningState: PlanningDraft,
	}
	if err := conn.Create(&task).Error; err != nil {
		t.Fatalf("seed work item: %v", err)
	}

	_, err := NewService(conn).CompleteWorkItem(context.Background(), CompletionCommand{
		WorkItemID: task.TaskID, ExpectedRevision: 0, Actor: "delivery-owner", Reason: "complete",
	})
	if ErrorCode(err) != "completion_evidence_required" {
		t.Fatalf("completion error = %v, want completion_evidence_required", err)
	}
	var stored db.TaskTelemetry
	if loadErr := conn.Where("task_id = ?", task.TaskID).First(&stored).Error; loadErr != nil {
		t.Fatalf("load unchanged work item: %v", loadErr)
	}
	if stored.Status != "progress" || stored.CompletedAt != nil || stored.Revision != 0 {
		t.Fatalf("evidence failure mutated work item: %+v", stored)
	}
}

func TestCompleteWorkItemRejectsStaleRevision(t *testing.T) {
	conn := openPlanningTestDB(t)
	task := db.TaskTelemetry{
		TaskID: "FZ-2249", ExternalKey: "FZ-2249", ProjectKey: "FZ", Source: "jira",
		IssueType: "requirement", Status: "progress", PlanningState: PlanningDraft, Revision: 2,
	}
	if err := conn.Create(&task).Error; err != nil {
		t.Fatalf("seed work item: %v", err)
	}
	if err := conn.Create(&db.GitCommitLog{
		TaskID: task.TaskID, Repo: "task_executor", CommitID: "task-sha", Action: "git_push", CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed completion evidence: %v", err)
	}

	_, err := NewService(conn).CompleteWorkItem(context.Background(), CompletionCommand{
		WorkItemID: task.TaskID, ExpectedRevision: 1, Actor: "delivery-owner", Reason: "complete",
	})
	if ErrorCode(err) != "revision_conflict" {
		t.Fatalf("completion error = %v, want revision_conflict", err)
	}
}
