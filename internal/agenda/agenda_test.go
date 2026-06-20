package agenda

import (
	"testing"
	"time"
	"well-ambient/internal/db"
)

func TestEvaluateActiveTasks(t *testing.T) {
	now := time.Now()

	// 1. Safe Task: Updated recently, not overdue
	safeTask := db.TaskTelemetry{
		TaskID:        "SAFE-1",
		Title:         "Safe Task",
		Status:        "progress",
		TaskCreatedAt: now.Add(-24 * time.Hour),
		LastUpdate:    now.Add(-1 * time.Hour),
	}

	// 2. Critical No Commit Task: In progress but no commits for 72 hours
	noCommitTask := db.TaskTelemetry{
		TaskID:        "NO-COMMIT-1",
		Title:         "No Commit Task",
		Status:        "progress",
		TaskCreatedAt: now.Add(-5 * 24 * time.Hour),
		LastUpdate:    now.Add(-72 * time.Hour),
	}

	// 3. Overdue Task: DueDate exceeded
	pastDue := now.Add(-12 * time.Hour)
	overdueTask := db.TaskTelemetry{
		TaskID:        "OVERDUE-1",
		Title:         "Overdue Task",
		Status:        "progress",
		TaskCreatedAt: now.Add(-2 * 24 * time.Hour),
		LastUpdate:    now.Add(-2 * time.Hour),
		DueDate:       &pastDue,
	}

	// 4. Warning Long-running Task: Created 10 days ago
	longRunningTask := db.TaskTelemetry{
		TaskID:        "LONG-1",
		Title:         "Long running Task",
		Status:        "progress",
		TaskCreatedAt: now.Add(-10 * 24 * time.Hour),
		LastUpdate:    now.Add(-2 * time.Hour),
	}

	// 5. Conflict Tasks: Two progress tasks on the same Repo
	conflictTask1 := db.TaskTelemetry{
		TaskID:        "CONFLICT-1",
		Title:         "Conflict Task 1",
		Status:        "progress",
		Repo:          "test-repo",
		Branch:        "feat-1",
		TaskCreatedAt: now.Add(-12 * time.Hour),
		LastUpdate:    now.Add(-2 * time.Hour),
	}
	conflictTask2 := db.TaskTelemetry{
		TaskID:        "CONFLICT-2",
		Title:         "Conflict Task 2",
		Status:        "progress",
		Repo:          "test-repo",
		Branch:        "feat-2",
		TaskCreatedAt: now.Add(-12 * time.Hour),
		LastUpdate:    now.Add(-2 * time.Hour),
	}

	tasks := []db.TaskTelemetry{safeTask, noCommitTask, overdueTask, longRunningTask, conflictTask1, conflictTask2}
	items := EvaluateActiveTasks(tasks)

	// We expect 6 items back (since safeTask is also returned now for global dashboard filtering)
	if len(items) != 6 {
		t.Fatalf("Expected 6 items, got %d", len(items))
	}

	// Map items for easy check
	itemMap := make(map[string]AgendaItem)
	for _, it := range items {
		itemMap[it.TaskID] = it
	}

	// Verify safeTask is indeed safe
	if safeIt, safeOk := itemMap["SAFE-1"]; !safeOk || safeIt.RiskLevel != "safe" {
		t.Errorf("SAFE-1 risk level should be safe, got: %+v", safeIt)
	}

	// Verify noCommitTask
	it, ok := itemMap["NO-COMMIT-1"]
	if !ok || it.RiskLevel != "critical" || it.RiskType != "no_commit_48h" {
		t.Errorf("NO-COMMIT-1 risk diagnosis incorrect: %+v", it)
	}

	// Verify overdueTask
	it, ok = itemMap["OVERDUE-1"]
	if !ok || it.RiskLevel != "critical" || it.RiskType != "overdue" {
		t.Errorf("OVERDUE-1 risk diagnosis incorrect: %+v", it)
	}

	// Verify longRunningTask
	it, ok = itemMap["LONG-1"]
	if !ok || it.RiskLevel != "warning" || it.RiskType != "overdue" {
		t.Errorf("LONG-1 risk diagnosis incorrect: %+v", it)
	}

	// Verify Conflict Tasks (at least one should raise conflict risk since they modify same repo on different branches)
	c1, ok1 := itemMap["CONFLICT-1"]
	c2, ok2 := itemMap["CONFLICT-2"]
	if !ok1 && !ok2 {
		t.Error("Expected conflict risk to be generated for CONFLICT-1 or CONFLICT-2")
	} else {
		if ok1 && (c1.RiskLevel != "warning" || c1.RiskType != "potential_conflict") {
			t.Errorf("CONFLICT-1 risk diagnosis incorrect: %+v", c1)
		}
		if ok2 && (c2.RiskLevel != "warning" || c2.RiskType != "potential_conflict") {
			t.Errorf("CONFLICT-2 risk diagnosis incorrect: %+v", c2)
		}
	}
}
