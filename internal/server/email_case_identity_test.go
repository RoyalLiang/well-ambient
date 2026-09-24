package server

import (
	"context"
	"testing"
	"time"
	"well-ambient/internal/db"
)

func TestEmailChartOwnerCountsMatchCaseInsensitiveCoreScope(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	cfg.Jira.SyncUsers = []string{"ALICE"}
	today, yesterday, _, _ := emailDateBounds("2026-03-09", cfg.DailyJiraEmail.Timezone, time.Time{})
	if err := conn.Create(&db.TaskTelemetry{TaskID: "CASE-1", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Assignee: "alice", Status: "progress", TaskCreatedAt: yesterday, SourceUpdatedAt: yesterday}).Error; err != nil {
		t.Fatal(err)
	}
	report, err := (&Server{config: &cfg}).buildEmailReport(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", today)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.RecentUnresolved) != 1 || report.JiraCharts[1].Total != 1 || report.CoreMembers[0].UnresolvedCount != 1 {
		t.Fatalf("owner chart lost matching record: %+v", report.CoreMembers)
	}
}
