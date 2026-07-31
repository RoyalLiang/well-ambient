package deliveryplanning

import (
	"testing"
	"well-ambient/internal/db"
)

func TestPlanMigrationUsesOnlyUnambiguousProjectAndParentFacts(t *testing.T) {
	projects := []db.ProjectConfig{
		{ProjectKey: "HIT", GitReposJSON: `["platform/api"]`},
		{ProjectKey: "NS2", GitReposJSON: `["shared/repo"]`},
		{ProjectKey: "DG", GitReposJSON: `["shared/repo"]`},
	}
	tasks := []db.TaskTelemetry{
		{TaskID: "HIT-101", IssueType: "demand", TaskGroupID: "g-1"},
		{TaskID: "LOCAL-1", IssueType: "Bug", Repo: "platform/api"},
		{TaskID: "EXEC-1", IssueType: "task", TaskGroupID: "g-1"},
		{TaskID: "EXEC-2", IssueType: "task", Repo: "shared/repo"},
	}
	report := PlanMigration(tasks, projects)
	if report.TotalBefore != 4 || report.TotalAfter != 4 {
		t.Fatalf("migration must conserve tasks: %+v", report)
	}
	changes := make(map[string]MigrationChange)
	for _, change := range report.Changes {
		changes[change.TaskID] = change
	}
	if changes["HIT-101"].ProjectKeyAfter != "HIT" || changes["HIT-101"].IssueTypeAfter != WorkItemRequirement {
		t.Fatalf("unexpected HIT work item migration: %+v", changes["HIT-101"])
	}
	if changes["LOCAL-1"].ProjectKeyAfter != "HIT" || changes["LOCAL-1"].ProjectResolution != "unique_repository" {
		t.Fatalf("unexpected repository migration: %+v", changes["LOCAL-1"])
	}
	if changes["EXEC-1"].ParentWorkItemIDAfter != "HIT-101" ||
		changes["EXEC-1"].ParentResolution != "unique_task_group" ||
		changes["EXEC-1"].IssueTypeAfter != ExecutionTask {
		t.Fatalf("unexpected execution parent migration: %+v", changes["EXEC-1"])
	}
	if changes["EXEC-2"].ProjectKeyAfter != "" || changes["EXEC-2"].ParentWorkItemIDAfter != "" {
		t.Fatalf("ambiguous/orphan task must remain unresolved: %+v", changes["EXEC-2"])
	}
	if report.UnresolvedProjectCount != 1 || report.OrphanExecutionCount != 1 {
		t.Fatalf("unresolved counts = project %d orphan %d, want 1/1", report.UnresolvedProjectCount, report.OrphanExecutionCount)
	}
}

func TestApplyMigrationIsIdempotent(t *testing.T) {
	conn := openPlanningTestDB(t)
	if err := conn.AutoMigrate(&db.ProjectConfig{}); err != nil {
		t.Fatalf("migrate project config: %v", err)
	}
	if err := conn.Create(&db.ProjectConfig{ProjectKey: "HIT", ProjectName: "HIT", GitReposJSON: "[]"}).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := conn.Create(&db.TaskTelemetry{TaskID: "HIT-301", IssueType: "demand"}).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	report, err := LoadMigrationReport(t.Context(), conn)
	if err != nil {
		t.Fatalf("load report: %v", err)
	}
	applied, err := ApplyMigration(t.Context(), conn, report)
	if err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	if applied.TotalAfter != 1 {
		t.Fatalf("applied task total = %d, want 1", applied.TotalAfter)
	}
	second, err := LoadMigrationReport(t.Context(), conn)
	if err != nil {
		t.Fatalf("load second report: %v", err)
	}
	if second.ChangeCount != 0 {
		t.Fatalf("second run still has %d changes: %+v", second.ChangeCount, second.Changes)
	}
}
