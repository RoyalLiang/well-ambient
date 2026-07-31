package deliveryplanning

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
	"well-ambient/internal/db"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type queryCounterLogger struct {
	logger.Interface
	count atomic.Int64
}

func (l *queryCounterLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	l.count.Add(1)
	l.Interface.Trace(ctx, begin, fc, err)
}

func openPlanningTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:planning-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := conn.AutoMigrate(
		&db.TaskTelemetry{},
		&db.ReleaseVersion{},
		&db.WorkItemReleaseLink{},
		&db.WorkItemEvent{},
		&db.WorkItemSyncOperation{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return conn
}

func TestNormalizeIssueTypeUsesHardDomainBoundary(t *testing.T) {
	cases := map[string]string{
		"Demand":   WorkItemRequirement,
		"需求":       WorkItemRequirement,
		"Bug":      WorkItemBug,
		"Defect":   WorkItemBug,
		"Sub-task": ExecutionTask,
	}
	for input, want := range cases {
		got, err := NormalizeIssueType(input)
		if err != nil {
			t.Fatalf("normalize %q: %v", input, err)
		}
		if got != want {
			t.Fatalf("normalize %q: got %q want %q", input, got, want)
		}
	}
	if _, err := NormalizeIssueType("epic"); ErrorCode(err) != "unsupported_work_item_kind" {
		t.Fatalf("expected unsupported kind error, got %v", err)
	}
}

func TestQueryPlanUsesBoundedQueriesForWorkItemSnapshots(t *testing.T) {
	conn := openPlanningTestDB(t)
	release := db.ReleaseVersion{ProjectKey: "HIT", Name: "1.0", Status: ReleasePlanned, Source: "local"}
	if err := conn.Create(&release).Error; err != nil {
		t.Fatalf("create release: %v", err)
	}
	for index := 0; index < 40; index++ {
		taskID := fmt.Sprintf("HIT-%03d", index+1)
		kind := "requirement"
		if index%2 == 1 {
			kind = "bug"
		}
		task := db.TaskTelemetry{
			TaskID: taskID, ProjectKey: "HIT", Source: "jira", ExternalKey: taskID,
			IssueType: kind, Title: "bounded query fixture", PlanningState: PlanningReady,
			LastUpdate: time.Date(2026, 7, 31, 9, index, 0, 0, time.UTC),
		}
		if err := conn.Create(&task).Error; err != nil {
			t.Fatalf("create task %s: %v", taskID, err)
		}
		if index%3 == 0 {
			if err := conn.Create(&db.WorkItemReleaseLink{
				WorkItemID: taskID, ReleaseVersionID: release.ID, Relation: ReleaseTargetFix,
				IsPrimary: true, Active: true, Source: "manual",
			}).Error; err != nil {
				t.Fatalf("create release link %s: %v", taskID, err)
			}
		}
		if index%5 == 0 {
			if err := conn.Create(&db.WorkItemSyncOperation{
				IdempotencyKey: taskID + ":sync", WorkItemID: taskID, Operation: "update_issue_versions", Status: "pending",
			}).Error; err != nil {
				t.Fatalf("create sync operation %s: %v", taskID, err)
			}
		}
	}

	counter := &queryCounterLogger{Interface: logger.Default.LogMode(logger.Silent)}
	service := NewService(conn.Session(&gorm.Session{Logger: counter}))
	snapshot, err := service.QueryPlan(context.Background(), PlanQuery{Limit: 100})
	if err != nil {
		t.Fatalf("query plan: %v", err)
	}
	if got := len(snapshot.Items); got != 40 {
		t.Fatalf("items = %d, want 40", got)
	}
	if got := counter.count.Load(); got > 6 {
		t.Fatalf("QueryPlan executed %d SQL statements for 40 rows; want at most 6 bounded batch queries", got)
	}
}

func TestApplyPlanningChangeIsAtomicAndCreatesAuditAndOutbox(t *testing.T) {
	conn := openPlanningTestDB(t)
	task := db.TaskTelemetry{
		TaskID:        "HIT-101",
		IssueType:     "demand",
		Title:         "交付事实收敛",
		ProjectKey:    "HIT",
		Source:        "jira",
		ExternalKey:   "HIT-101",
		PlanningState: PlanningReady,
	}
	if err := conn.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	release := db.ReleaseVersion{
		ProjectKey: "HIT",
		Source:     "jira",
		ExternalID: "13622",
		Name:       "1.1",
		Status:     ReleasePlanned,
	}
	if err := conn.Create(&release).Error; err != nil {
		t.Fatalf("create release: %v", err)
	}
	service := NewService(conn)
	fixedNow := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }
	state := PlanningCommitted
	assignee := "eddie"
	primary := release.ID
	snapshot, err := service.ApplyPlanningChange(context.Background(), PlanningCommand{
		WorkItemID:             task.TaskID,
		ExpectedRevision:       0,
		PrimaryTargetReleaseID: &primary,
		Assignee:               &assignee,
		PlanningState:          &state,
		Reason:                 "纳入 1.1 发布范围",
		Actor:                  "pm",
		Source:                 "manual",
	})
	if err != nil {
		t.Fatalf("apply planning change: %v", err)
	}
	if snapshot.WorkItem.Revision != 1 || snapshot.WorkItem.PlanningState != PlanningCommitted {
		t.Fatalf("unexpected updated work item: %+v", snapshot.WorkItem)
	}
	if got := primaryReleaseID(snapshot.Links); got != release.ID {
		t.Fatalf("primary release = %d, want %d", got, release.ID)
	}
	var eventCount, operationCount int64
	conn.Model(&db.WorkItemEvent{}).Count(&eventCount)
	conn.Model(&db.WorkItemSyncOperation{}).Count(&operationCount)
	if eventCount != 1 || operationCount != 1 {
		t.Fatalf("audit/outbox counts = %d/%d, want 1/1", eventCount, operationCount)
	}
}

func TestApplyPlanningChangeRejectsMismatchAffectedAndRevisionConflict(t *testing.T) {
	conn := openPlanningTestDB(t)
	requirement := db.TaskTelemetry{
		TaskID:        "HIT-102",
		IssueType:     "requirement",
		ProjectKey:    "HIT",
		PlanningState: PlanningReady,
	}
	if err := conn.Create(&requirement).Error; err != nil {
		t.Fatalf("create requirement: %v", err)
	}
	otherRelease := db.ReleaseVersion{
		ProjectKey: "FEL2WD",
		Source:     "jira",
		ExternalID: "20",
		Name:       "2.0",
		Status:     ReleasePlanned,
	}
	if err := conn.Create(&otherRelease).Error; err != nil {
		t.Fatalf("create release: %v", err)
	}
	service := NewService(conn)
	primary := otherRelease.ID
	_, err := service.ApplyPlanningChange(context.Background(), PlanningCommand{
		WorkItemID:             requirement.TaskID,
		ExpectedRevision:       0,
		PrimaryTargetReleaseID: &primary,
		Reason:                 "test mismatch",
		Actor:                  "pm",
	})
	if ErrorCode(err) != "release_project_mismatch" {
		t.Fatalf("expected project mismatch, got %v", err)
	}

	affected := []uint{otherRelease.ID}
	_, err = service.ApplyPlanningChange(context.Background(), PlanningCommand{
		WorkItemID:         requirement.TaskID,
		ExpectedRevision:   0,
		AffectedReleaseIDs: &affected,
		Reason:             "test affected",
		Actor:              "pm",
	})
	if ErrorCode(err) != "affected_release_only_for_bug" {
		t.Fatalf("expected affected-only-for-bug, got %v", err)
	}

	if err := conn.Model(&requirement).Update("revision", 3).Error; err != nil {
		t.Fatalf("advance revision: %v", err)
	}
	assignee := "new-owner"
	_, err = service.ApplyPlanningChange(context.Background(), PlanningCommand{
		WorkItemID:       requirement.TaskID,
		ExpectedRevision: 0,
		Assignee:         &assignee,
		Reason:           "test revision",
		Actor:            "pm",
	})
	if ErrorCode(err) != "revision_conflict" {
		t.Fatalf("expected revision conflict, got %v", err)
	}

	var eventCount int64
	conn.Model(&db.WorkItemEvent{}).Count(&eventCount)
	if eventCount != 0 {
		t.Fatalf("rejected changes must not create events, got %d", eventCount)
	}
}
