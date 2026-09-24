package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
	"well-ambient/internal/telemetry"
)

func TestEmailAuthoritativeCategoryAndEveryHistoricalCoreMember(t *testing.T) {
	conn := emailTestDatabase(t)
	if err := conn.AutoMigrate(&db.JiraReportChange{}); err != nil {
		t.Fatal(err)
	}
	cfg := emailTestConfig()
	cfg.Jira.SyncUsers = []string{"alice", "bob"}
	cfg.DailyJiraEmail.ProjectGroups = []config.DailyJiraProjectGroup{{Name: "组", Projects: []string{"P"}, Owners: []string{"outsider"}}}
	for _, user := range []userdb.User{{Username: "alice", Name: "Alice", Email: "alice@example.test"}, {Username: "bob", Name: "Bob", Email: "bob@example.test"}} {
		if err := conn.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}
	today, yesterday, recent, _ := emailDateBounds("2026-03-09", cfg.DailyJiraEmail.Timezone, time.Time{})
	tasks := []db.TaskTelemetry{
		{TaskID: "P-1", Title: "没有 FMS 关键词", Assignee: "outsider", JiraBugCategory: "FMS"},
		{TaskID: "P-2", Title: "FMS 标题不决定分类", Assignee: "Bob", JiraBugCategory: "GPP"},
		{TaskID: "P-3", Title: "FMS GPP 路径死锁", Assignee: "Alice", JiraBugCategory: "硬件"},
		{TaskID: "P-4", Title: "FMS GPP", Assignee: "Alice"},
		{TaskID: "P-5", Assignee: "outsider", JiraBugCategory: "FMS"},
		{TaskID: "P-6", Assignee: "outsider", JiraBugCategory: "FMS"},
	}
	for i := range tasks {
		tasks[i].Source = "jira"
		tasks[i].Status = "progress"
		tasks[i].TaskCreatedAt = recent
		tasks[i].SourceUpdatedAt = yesterday
		tasks[i].JiraHistoryComplete = true
		if i != 3 {
			tasks[i].JiraBugCategoryFieldID = "customfield_42"
		}
	}
	tasks[5].TaskCreatedAt = today.AddDate(0, 0, -20)
	tasks[5].SourceUpdatedAt = today.AddDate(0, 0, -5)
	if err := conn.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	changes := []db.JiraReportChange{
		{TaskID: "P-1", HistoryID: "1", Field: "assignee", FromID: "alice", FromValue: "Alice", ToID: "bob", ToValue: "Bob", OccurredAt: yesterday.Add(-2 * time.Hour)},
		{TaskID: "P-1", HistoryID: "2", Field: "assignee", FromID: "bob", FromValue: "Bob", ToID: "outsider", ToValue: "Outsider", OccurredAt: yesterday.Add(-time.Hour)},
		{TaskID: "P-6", HistoryID: "3", Field: "assignee", FromID: "alice", ToID: "outsider", OccurredAt: today.AddDate(0, 0, -6)},
	}
	if err := conn.Create(&changes).Error; err != nil {
		t.Fatal(err)
	}
	report, err := (&Server{}).buildEmailReport(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", today)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.YesterdayUpdated) != 2 || len(report.RecentUnresolved) != 2 || report.YesterdayGroups[0].Count != 2 {
		t.Fatalf("incorrect scope: %+v", report.YesterdayUpdated)
	}
	if len(report.CoreMembers) != 2 || report.CoreMembers[0].Name != "Alice" || report.CoreMembers[0].YesterdayCount != 1 || report.CoreMembers[1].YesterdayCount != 2 || report.CoreMembers[1].UnresolvedCount != 2 {
		t.Fatalf("historical members: %+v", report.CoreMembers)
	}
	if report.YesterdayUpdated[0].Assignee != "outsider" || report.YesterdayUpdated[1].Category != "GPP" {
		t.Fatal("current assignee or field classification changed")
	}
	total := 0
	for _, n := range report.TrendPoints {
		total += n
	}
	if total != 3 {
		t.Fatalf("trend scope differs: %+v", report.TrendPoints)
	}
	if report.CategoryStats[0].YesterdayCount != 1 || report.CategoryStats[1].YesterdayCount != 1 {
		t.Fatalf("categories: %+v", report.CategoryStats)
	}
}

func TestJiraReportHistoryPersistsWithoutPerformanceAndIsIdempotent(t *testing.T) {
	conn := emailTestDatabase(t)
	if err := conn.AutoMigrate(&db.JiraReportChange{}); err != nil {
		t.Fatal(err)
	}
	var issue telemetry.JiraIssue
	if err := json.Unmarshal([]byte(`{"key":"P-1","changelog":{"startAt":0,"total":1,"histories":[{"id":"1","created":"2026-09-21T01:00:00.000+0000","items":[{"field":"assignee","from":"alice","fromString":"Alice","to":"other","toString":"Other"},{"field":"summary","fromString":"old","toString":"new"}]}]}}`), &issue); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := persistJiraReportChanges(issue); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	conn.Model(&db.JiraReportChange{}).Count(&count)
	if count != 2 {
		t.Fatalf("history duplicated: %d", count)
	}
	task := db.TaskTelemetry{JiraBugCategory: "FMS", JiraBugCategoryFieldID: "old"}
	if !applyJiraReportFields(&task, issue) || task.JiraBugCategory != "" || !task.JiraHistoryComplete {
		t.Fatalf("stale category was not cleared: %+v", task)
	}
}

func TestEmailHistoricalJiraScopeIncludesTransferredAndResolvedIssues(t *testing.T) {
	cfg := emailTestConfig()
	cfg.Jira.SyncProjects = []string{"P"}
	cfg.Jira.SyncStatuses = []string{"Open"}
	cfg.DailyJiraEmail.Enabled = true
	jql := buildEmailHistoricalJiraScope(cfg)
	for _, want := range []string{`assignee WAS IN ("alice")`, `project IN ("P")`, `updated >= -8d`} {
		if !strings.Contains(jql, want) {
			t.Fatalf("%s missing %s", jql, want)
		}
	}
	if strings.Contains(strings.ToLower(jql), "status") {
		t.Fatal("resolved updates must not be filtered out")
	}
}

func TestEmailGroupOwnersCannotReplaceMissingCoreMemberConfiguration(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	cfg.Jira.SyncUsers = nil
	cfg.DailyJiraEmail.ProjectGroups = []config.DailyJiraProjectGroup{{Name: "P", Projects: []string{"P"}, Owners: []string{"outsider"}}}
	today, yesterday, _, _ := emailDateBounds("2026-03-09", cfg.DailyJiraEmail.Timezone, time.Time{})
	if err := conn.Create(&db.TaskTelemetry{TaskID: "P-1", Source: "jira", Assignee: "outsider", JiraBugCategory: "FMS", SourceUpdatedAt: yesterday, TaskCreatedAt: yesterday}).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Create(&db.GitCommitLog{Repo: "repo", CommitID: "sha", Author: "outsider", Action: "git_push", CreatedAt: yesterday}).Error; err != nil {
		t.Fatal(err)
	}
	report, err := (&Server{}).buildEmailReport(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", today)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.CoreMembers) > 0 || len(report.YesterdayUpdated) > 0 || report.Commits.Count > 0 {
		t.Fatal("empty core scope expanded to group owners or all contributors")
	}
}

func TestJiraReportMissingFieldValueCannotRetainOldCategory(t *testing.T) {
	task := db.TaskTelemetry{JiraBugCategory: "FMS", JiraBugCategoryFieldID: "old"}
	applyJiraReportFields(&task, telemetry.JiraIssue{BugCategoryFieldID: "customfield_42", BugCategoryAvailable: false})
	if task.JiraBugCategory != "" || task.JiraBugCategoryFieldID != "" {
		t.Fatalf("missing field must remain visibly unavailable: %+v", task)
	}
}
