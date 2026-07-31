package server

import (
	"testing"
	"time"

	"well-ambient/internal/db"
)

func TestDemandSchedulingDoesNotRequireImplementationBranch(t *testing.T) {
	dueDate := time.Now().AddDate(0, 0, 7)
	demand := db.TaskTelemetry{
		TaskID:    "DEMAND-PLANNED",
		IssueType: "demand",
		Status:    "backlog",
		Assignee:  "Planner",
		DueDate:   &dueDate,
		Branch:    "",
	}

	if !isDemandScheduled(demand) {
		t.Fatal("a confirmed delivery date is a schedule fact even before an implementation branch exists")
	}
}
