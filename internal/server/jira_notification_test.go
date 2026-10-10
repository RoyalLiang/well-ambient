package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/kanban"
	"well-ambient/internal/telemetry"
)

func TestJiraReassignmentNotifiesFrontendAfterCommitBeforeSameIssueComments(t *testing.T) {
	setupServerTestDB(t)
	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID: "NS2-2569", ProjectKey: "NS2", Source: "jira", ExternalKey: "NS2-2569",
		Title: "Reassigned issue", Assignee: "梁志远", Status: "backlog", IssueType: "bug",
		TaskCreatedAt: parseJiraTime("2026-10-01T08:00:00.000+0800"), LastUpdate: time.Now().Add(-time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
	commentsStarted := make(chan struct{}, 1)
	releaseComments := make(chan struct{})
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/search":
			fmt.Fprint(w, `{"total":1,"issues":[{"key":"NS2-2569","fields":{"summary":"Reassigned issue","created":"2026-10-01T08:00:00.000+0800","updated":"2026-10-09T23:00:00.000+0800","issuetype":{"name":"Bug"},"assignee":{"name":"external.owner","displayName":"外部负责人"},"status":{"name":"Open"},"project":{"key":"NS2","name":"NS2"},"fixVersions":[],"versions":[]}}]}`)
		case strings.HasSuffix(r.URL.Path, "/comment"):
			commentsStarted <- struct{}{}
			<-releaseComments
			fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":0,"comments":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer jira.Close()
	srv := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: true, BaseURL: jira.URL, SyncProjects: []string{"NS2"}}}, "")
	web := httptest.NewServer(srv.handler)
	defer web.Close()
	token := superAdminToken(t, "notification-fixture@example.test", "Notification Fixture", []string{"decision:read", "dashboard:read"})
	response, err := http.Get(web.URL + "/api/notifications/sse?token=" + token)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("authenticated SSE status = %d", response.StatusCode)
	}
	updates := make(chan string, 8)
	go func() {
		scanner := bufio.NewScanner(response.Body)
		telemetryEvent := false
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "event:") {
				telemetryEvent = line == "event: telemetry-updated"
			}
			if telemetryEvent && strings.HasPrefix(line, "data:") {
				var payload struct {
					TaskID string `json:"task_id"`
				}
				if json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &payload) == nil {
					updates <- payload.TaskID
				}
			}
		}
	}()
	done := make(chan struct{})
	go func() { defer close(done); srv.syncJiraTasks() }()
	defer func() { close(releaseComments); <-done }()
	select {
	case <-commentsStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("Jira comment sync did not start")
	}
	select {
	case key := <-updates:
		if key != "NS2-2569" {
			t.Fatalf("notification key = %q", key)
		}
		var persisted db.TaskTelemetry
		if err := db.DB.Where("task_id = ?", key).First(&persisted).Error; err != nil {
			t.Fatal(err)
		}
		if persisted.Assignee != "外部负责人" {
			t.Fatalf("notification arrived before reassignment commit: %q", persisted.Assignee)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("reassignment was committed but the frontend notification is blocked by the same issue's comments")
	}
}

func TestJiraHistoryNotifiesForCreatedAndReassignedFactsOnly(t *testing.T) {
	setupServerTestDB(t)
	var owner atomic.Value
	owner.Store("梁志远")
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		updatedAt := "2026-10-09T23:00:00.000+0800"
		if owner.Load() == "李四" {
			updatedAt = "2026-10-09T23:01:00.000+0800"
		}
		fmt.Fprintf(w, `{"total":1,"issues":[{"key":"NS2-2569","fields":{"summary":"History reassignment","created":"2026-10-01T08:00:00.000+0800","updated":%q,"duedate":"2026-10-08","issuetype":{"name":"Bug"},"assignee":{"name":%q,"displayName":%q},"status":{"name":"Open"},"project":{"key":"NS2","name":"NS2"}}}]}`, updatedAt, owner.Load(), owner.Load())
	}))
	defer jira.Close()
	cfg := &config.Config{Jira: config.JiraConfig{Enabled: true, BaseURL: jira.URL, SyncProjects: []string{"NS2"}, SyncUsers: []string{"梁志远"}}, PerformanceBrain: config.PerformanceBrainConfig{Enabled: true}}
	srv := NewServer(cfg, "")
	updates := make(chan string, 8)
	registerTelemetryClient(updates)
	defer unregisterTelemetryClient(updates)
	for _, assignee := range []string{"梁志远", "李四"} {
		owner.Store(assignee)
		changed, err := srv.syncPerformanceJiraHistory(telemetry.NewJiraClient(&cfg.Jira))
		if err != nil || !changed {
			t.Fatalf("history sync changed=%v, error=%v", changed, err)
		}
		select {
		case key := <-updates:
			var task db.TaskTelemetry
			if key != "NS2-2569" || db.DB.Where("task_id = ?", key).First(&task).Error != nil || task.Assignee != assignee {
				t.Fatalf("history notification must reference committed owner %q: %q %+v", assignee, key, task)
			}
		default:
			t.Fatalf("history committed owner %q without notifying the frontend", assignee)
		}
	}
	if changed, err := srv.syncPerformanceJiraHistory(telemetry.NewJiraClient(&cfg.Jira)); err != nil || changed {
		t.Fatalf("unchanged history: changed=%v error=%v", changed, err)
	}
	select {
	case key := <-updates:
		t.Fatalf("unchanged history emitted duplicate notification: %q", key)
	default:
	}
}
