package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestEmailGroupsNeverFallbackOutsideConfiguredProjects(t *testing.T) {
	groups := []config.DailyJiraProjectGroup{{Name: "FMS", Projects: []string{"FMS"}, Owners: []string{"alice"}}}
	issues := []emailIssue{{TaskID: "OTHER-1", Assignee: "alice"}, {TaskID: "FMS-2", Assignee: "alice"}}
	grouped, other := groupIssues(issues, groups)
	if grouped[0].Count != 1 || len(other) != 1 || other[0].TaskID != "OTHER-1" {
		t.Fatalf("grouped=%#v other=%#v", grouped, other)
	}
	ownerOnly := config.DailyJiraProjectGroup{Name: "owners", Owners: []string{"alice"}}
	if issueMatchesGroup("OTHER-1", "", "alice-long", ownerOnly) != 0 {
		t.Fatal("partial names must not establish responsibility")
	}
	if issueMatchesGroup("OTHER-1", "", "alice", ownerOnly) != 10 {
		t.Fatal("exact owner-only group no longer matches")
	}
}

func TestEmailIssueCategorySharesTypeLine(t *testing.T) {
	report := emailReport{Style: "brief", YesterdayUpdated: []emailIssue{{TaskID: "FMS-1", IssueType: "缺陷", Category: "FMS", UpdateSummary: "添加评论"}}}
	if err := renderEmailReport(&report); err != nil {
		t.Fatal(err)
	}
	start := strings.Index(report.HTML, `class="email-issue-kind"`)
	if start < 0 {
		t.Fatal("missing shared type/category wrapper")
	}
	end := strings.Index(report.HTML[start:], "</td>")
	cell := report.HTML[start : start+end]
	if strings.Contains(cell, "<br>") || !strings.Contains(cell, "缺陷") || !strings.Contains(cell, "FMS") {
		t.Fatal(cell)
	}
	if !strings.Contains(report.HTML, "更新：添加评论") {
		t.Fatal("missing operation type")
	}
}

func TestEmailCommitRepositoryBranchesAndLinks(t *testing.T) {
	cfg := emailTestConfig()
	cfg.GitLab = config.GitLabConfig{BaseURL: "https://git.example.test/gitlab", Repos: []config.RepoMapping{{Name: "backend", Path: "team/backend"}}}
	commits := []db.GitCommitLog{
		{Repo: "backend", Branch: "main", CommitID: "abcd123", Author: "alice", Message: "<fix>"},
		{Repo: "backend", Branch: "main", CommitID: "abcd123"},
		{Repo: "backend", Branch: "release", CommitID: "abcd123", Author: "alice"},
		{Repo: "unknown", CommitID: "cdef456"},
	}
	rows := buildEmailCommitRows(cfg, commits)
	if len(rows) != 3 || rows[0].Count != 1 || rows[1].Branch != "release" || rows[2].Branch != "未记录分支" || rows[2].Commits[0].URL != "" {
		t.Fatalf("rows: %#v", rows)
	}
	link := "https://git.example.test/gitlab/team/backend/-/commit/abcd123"
	if rows[0].Commits[0].URL != link {
		t.Fatal(rows[0])
	}
	for _, style := range []string{"brief", "focus", "ledger", "hyperframe"} {
		report := emailReport{Style: style, Date: "2026-09-22", YesterdayUpdated: []emailIssue{{TaskID: "FMS-1", Title: "FMS issue", Category: "FMS", UpdateSummary: "添加评论"}}, Commits: &emailCommitSummary{Count: 2, Rows: rows, Authors: map[string]int{"alice": 2}}}
		if err := renderEmailReport(&report); err != nil {
			t.Fatal(err)
		}
		storage, _, err := confluenceReportStorage(&report)
		if err != nil {
			t.Fatal(err)
		}
		for _, output := range []string{report.HTML, storage} {
			for _, want := range []string{"添加评论", "仓库", "分支", "release", `href="` + link + `"`, "&lt;fix&gt;"} {
				if !strings.Contains(output, want) {
					t.Errorf("%s missing %s", style, want)
				}
			}
		}
	}
	cfg.GitLab.Repos = append(cfg.GitLab.Repos, config.RepoMapping{Name: "backend", Path: "other/backend"})
	if emailCommitURL(cfg, "backend", "abcd123") != "" {
		t.Fatal("ambiguous repository must not link")
	}
}

func TestEmailJiraUpdatesUseSourceFactsAndHistoricalOwners(t *testing.T) {
	conn := emailTestDatabase(t)
	if err := conn.AutoMigrate(&db.PerformanceWorkItemEvent{}, &db.JiraCommentLog{}); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1)
	events := []db.PerformanceWorkItemEvent{
		{DedupeKey: "owner", WorkItemID: "FMS-1", FieldName: "assignee", FromValue: "alice", ToValue: "bob", SourceSystem: "jira", OccurredAt: start.Add(-time.Hour)},
		{DedupeKey: "status", WorkItemID: "FMS-1", FieldName: "status", FromValue: "Open", ToValue: "Done", SourceSystem: "jira", OccurredAt: start},
		{DedupeKey: "future", WorkItemID: "FMS-1", FieldName: "priority", FromValue: "Low", ToValue: "High", SourceSystem: "jira", OccurredAt: end},
	}
	if err := conn.Create(&events).Error; err != nil {
		t.Fatal(err)
	}
	edited := start.Add(time.Hour)
	comments := []db.JiraCommentLog{{TaskID: "FMS-1", CommentID: "new", CreatedAt: start}, {TaskID: "FMS-1", CommentID: "old", CreatedAt: start.Add(-time.Hour), SourceUpdatedAt: &edited}}
	if err := conn.Create(&comments).Error; err != nil {
		t.Fatal(err)
	}
	facts, err := loadEmailJiraFacts(context.Background(), []db.TaskTelemetry{{TaskID: "FMS-1", Assignee: "bob"}}, start, end)
	if err != nil {
		t.Fatal(err)
	}
	updates := strings.Join(facts.Updates["FMS-1"], ";")
	for _, want := range []string{"添加评论", "编辑评论", "修改状态：Open → Done"} {
		if !strings.Contains(updates, want) {
			t.Fatal(updates)
		}
	}
	if strings.Contains(updates, "优先级") || strings.Join(facts.Owners["FMS-1"], ",") != "bob,alice" {
		t.Fatalf("facts: %#v", facts)
	}
}
