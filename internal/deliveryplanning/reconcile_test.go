package deliveryplanning

import (
	"context"
	"testing"
	"time"
	"well-ambient/internal/db"
)

func TestReconcileExternalIssueSingleTargetBugIsIdempotent(t *testing.T) {
	conn := openPlanningTestDB(t)
	task := db.TaskTelemetry{
		TaskID:        "HIT-201",
		IssueType:     "bug",
		PlanningState: PlanningReady,
	}
	if err := conn.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	service := NewService(conn)
	service.now = func() time.Time { return time.Date(2026, 7, 30, 2, 0, 0, 0, time.UTC) }
	state := ExternalIssueVersionState{
		IssueKey:   task.TaskID,
		ProjectKey: "HIT",
		Kind:       "bug",
		TargetReleases: []ExternalRelease{{
			ExternalID: "12",
			Name:       "1.2",
			Status:     ReleasePlanned,
		}},
		AffectedReleases: []ExternalRelease{{
			ExternalID: "10",
			Name:       "1.0",
			Status:     ReleaseReleased,
		}},
	}
	first, err := service.ReconcileExternalIssue(context.Background(), state)
	if err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	if first.WorkItem.ProjectKey != "HIT" || first.WorkItem.Source != "jira" || first.WorkItem.Revision != 1 {
		t.Fatalf("unexpected reconciled task: %+v", first.WorkItem)
	}
	if len(first.TargetReleases) != 1 || len(first.Affected) != 1 || primaryReleaseID(first.Links) == 0 {
		t.Fatalf("unexpected release facts: %+v", first)
	}
	second, err := service.ReconcileExternalIssue(context.Background(), state)
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if second.WorkItem.Revision != 1 {
		t.Fatalf("idempotent reconcile advanced revision to %d", second.WorkItem.Revision)
	}
	var releaseCount, linkCount, eventCount int64
	conn.Model(&db.ReleaseVersion{}).Count(&releaseCount)
	conn.Model(&db.WorkItemReleaseLink{}).Count(&linkCount)
	conn.Model(&db.WorkItemEvent{}).Count(&eventCount)
	if releaseCount != 2 || linkCount != 2 || eventCount != 1 {
		t.Fatalf("idempotent counts release/link/event = %d/%d/%d, want 2/2/1", releaseCount, linkCount, eventCount)
	}
}

func TestReconcileExternalIssueDoesNotGuessMultipleTargetsAndProtectsManualPrimary(t *testing.T) {
	conn := openPlanningTestDB(t)
	task := db.TaskTelemetry{
		TaskID:        "HIT-202",
		IssueType:     "requirement",
		ProjectKey:    "HIT",
		Source:        "jira",
		ExternalKey:   "HIT-202",
		PlanningState: PlanningReady,
	}
	if err := conn.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	service := NewService(conn)
	multiple := ExternalIssueVersionState{
		IssueKey:   task.TaskID,
		ProjectKey: "HIT",
		Kind:       "requirement",
		TargetReleases: []ExternalRelease{
			{ExternalID: "11", Name: "1.1", Status: ReleasePlanned},
			{ExternalID: "12", Name: "1.2", Status: ReleasePlanned},
		},
	}
	snapshot, err := service.ReconcileExternalIssue(context.Background(), multiple)
	if err != nil {
		t.Fatalf("multi release reconcile: %v", err)
	}
	if primaryReleaseID(snapshot.Links) != 0 {
		t.Fatal("multiple Jira targets must not be assigned a guessed primary")
	}
	var conflict db.WorkItemEvent
	if err := conn.Where("work_item_id = ? AND reason = ?", task.TaskID, "ambiguous_target_release").First(&conflict).Error; err != nil {
		t.Fatalf("expected ambiguity event: %v", err)
	}

	confirmedAt := time.Now()
	chosenID := snapshot.TargetReleases[0].ID
	if err := conn.Model(&db.WorkItemReleaseLink{}).
		Where("work_item_id = ? AND release_version_id = ?", task.TaskID, chosenID).
		Updates(map[string]any{
			"is_primary":   true,
			"confirmed_by": "pm",
			"confirmed_at": confirmedAt,
		}).Error; err != nil {
		t.Fatalf("confirm primary: %v", err)
	}
	drifted := multiple
	drifted.TargetReleases = drifted.TargetReleases[1:]
	snapshot, err = service.ReconcileExternalIssue(context.Background(), drifted)
	if err != nil {
		t.Fatalf("drift reconcile: %v", err)
	}
	if primaryReleaseID(snapshot.Links) != chosenID {
		t.Fatalf("manual primary was silently removed: got %d want %d", primaryReleaseID(snapshot.Links), chosenID)
	}
	conflict = db.WorkItemEvent{}
	if err := conn.Where("work_item_id = ? AND reason = ?", task.TaskID, "external_primary_release_drift").
		First(&conflict).Error; err != nil {
		t.Fatalf("expected external drift event: %v", err)
	}
}
