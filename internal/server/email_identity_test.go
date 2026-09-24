package server

import (
	"context"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

func TestEmailDoesNotMergeSameNameCoreMembers(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	cfg.Jira.SyncUsers = []string{"alice.one", "alice.two"}
	users := []userdb.User{{Username: "alice.one", Name: "Alice", Email: "one@example.test"}, {Username: "alice.two", Name: "Alice", Email: "two@example.test"}}
	if err := conn.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	today, yesterday, _, _ := emailDateBounds("2026-03-09", cfg.DailyJiraEmail.Timezone, time.Time{})
	tasks := []db.TaskTelemetry{
		{TaskID: "NAMES-1", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Assignee: "alice.one", Status: "progress", TaskCreatedAt: yesterday, SourceUpdatedAt: yesterday},
		{TaskID: "NAMES-2", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Assignee: "alice.two", Status: "progress", TaskCreatedAt: yesterday, SourceUpdatedAt: yesterday},
		{TaskID: "NAMES-3", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Assignee: "Alice", Status: "progress", TaskCreatedAt: yesterday, SourceUpdatedAt: yesterday},
	}
	if err := conn.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	for _, author := range []string{"alice.one", "alice.two", "Alice"} {
		if err := conn.Create(&db.GitCommitLog{Repo: "identity", CommitID: author, Author: author, Action: "git_push", CreatedAt: yesterday}).Error; err != nil {
			t.Fatal(err)
		}
	}
	report, err := (&Server{config: &cfg}).buildEmailReport(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", today)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.CoreMembers) != 2 || len(report.YesterdayUpdated) != 2 {
		t.Fatalf("wrong scope: %+v %+v", report.CoreMembers, report.YesterdayUpdated)
	}
	if report.Commits == nil || report.Commits.Count != 2 {
		t.Fatalf("ambiguous commit author was not excluded: %+v", report.Commits)
	}
	for _, member := range report.CoreMembers {
		if member.YesterdayCount != 1 || member.CommitCount != 1 || !strings.Contains(member.Name, "alice.") {
			t.Fatalf("merged identity: %+v", member)
		}
	}
	if strings.Contains(strings.Join(report.Warnings, " ")+report.HTML+report.Text, "同名成员") {
		t.Fatal("obsolete ambiguous identity warning rendered")
	}
	if strings.Contains(report.HTML+report.Text, "NAMES-3") || len(report.RecentUnresolved) != 2 {
		t.Fatal("ambiguous identity leaked into issue sections")
	}
}
