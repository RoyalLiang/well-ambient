package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/kanban"
	"well-ambient/internal/telemetry"
)

func TestJiraCompletionCategoryReachesScheduleThroughBothSyncPaths(t *testing.T) {
	for _, path := range []string{"inbound", "history"} {
		t.Run(path, func(t *testing.T) {
			setupServerTestDB(t)
			oldPath := kanban.KanbanFilePath
			kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
			t.Cleanup(func() { kanban.KanbanFilePath = oldPath })
			const issueJSON = `{"key":"SCHEDULE-1","fields":{"summary":"已在 Jira 验收完成","created":"2026-10-01T08:00:00.000+0800","updated":"2026-10-19T10:00:00.000+0800","resolutiondate":"2026-10-19T10:00:00.000+0800","duedate":"2026-10-18","issuetype":{"name":"Story"},"assignee":{"name":"owner","displayName":"Owner"},"status":{"name":"验收完成","statusCategory":{"key":"done","name":"完成"}},"project":{"key":"SCHEDULE","name":"Schedule"}}}`
			jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/rest/api/2/search" {
					fmt.Fprintf(w, `{"total":1,"issues":[%s]}`, issueJSON)
				} else {
					fmt.Fprint(w, `{"startAt":0,"total":0,"comments":[]}`)
				}
			}))
			defer jira.Close()
			cfg := &config.Config{Jira: config.JiraConfig{Enabled: true, BaseURL: jira.URL, SyncProjects: []string{"SCHEDULE"}, SyncUsers: []string{"Owner"}}, PerformanceBrain: config.PerformanceBrainConfig{Enabled: true}}
			srv := NewServer(cfg, "")
			client := telemetry.NewJiraClient(&cfg.Jira)
			if path == "history" {
				if _, err := srv.syncPerformanceJiraHistory(client); err != nil {
					t.Fatal(err)
				}
			} else {
				var issue telemetry.JiraIssue
				if err := json.Unmarshal([]byte(issueJSON), &issue); err != nil {
					t.Fatal(err)
				}
				if _, err := srv.reconcileJiraIssue(client, issue); err != nil {
					t.Fatal(err)
				}
			}
			var cached db.TaskTelemetry
			if err := db.DB.Where("task_id = ?", "SCHEDULE-1").First(&cached).Error; err != nil {
				t.Fatal(err)
			}
			if cached.Status != "done" || cached.CompletedAt == nil {
				t.Fatalf("Jira done category must persist as completed, got status=%q completed_at=%v", cached.Status, cached.CompletedAt)
			}
			response := buildScheduleResponse([]db.TaskTelemetry{cached}, nil, time.Date(2026, 10, 19, 12, 0, 0, 0, time.FixedZone("CST", 8*3600)))
			if len(response.Items) != 1 || response.Items[0].RiskLevel != "done" || response.Summary.Overdue != 0 {
				t.Fatalf("completed Jira must leave attention risk while remaining in completed history: %+v", response)
			}
		})
	}
}

func TestJiraWorkflowCategoryControlsStateWithoutLosingReviewStage(t *testing.T) {
	for _, tc := range []struct{ name, category, want string }{
		{"客户验收结束", "done", "done"},
		{"Done", "new", "backlog"},
		{"自定义开发步骤", "indeterminate", "progress"},
		{"待验收", "indeterminate", "review"},
		{"In Review", "indeterminate", "review"},
		{"已完成", "", "done"},
		{"未完成", "", "backlog"},
		{"Reopened", "", "backlog"},
	} {
		t.Run(tc.name+"/"+tc.category, func(t *testing.T) {
			var issue telemetry.JiraIssue
			issue.Fields.Status.Name = tc.name
			issue.Fields.Status.StatusCategory.Key = tc.category
			if got := mapJiraIssueStatus(issue); got != tc.want {
				t.Fatalf("status %q category %q mapped to %q, want %q", tc.name, tc.category, got, tc.want)
			}
		})
	}
}
