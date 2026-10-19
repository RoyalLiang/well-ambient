package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/kanban"
	"well-ambient/internal/performance"
	"well-ambient/internal/telemetry"
)

func TestCurrentJiraCompletionDoesNotWaitForBlockedHistoricalSearch(t *testing.T) {
	setupServerTestDB(t)
	// Shared-memory SQLite rejects simultaneous writers instead of waiting as
	// production PostgreSQL does. Serialize only the test DB's short statements.
	sqlDB, err := db.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	oldPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldPath })
	if err := db.DB.Create(&db.TaskTelemetry{TaskID: "NS2-344", Source: "jira", ExternalKey: "NS2-344", ProjectKey: "NS2", IssueType: "requirement", Status: "backlog", LastUpdate: time.Now().Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	blocked := make(chan struct{}, 1)
	release := make(chan struct{})
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/rest/api/2/search" {
			if !strings.Contains(r.URL.Query().Get("jql"), "updated >=") {
				select {
				case blocked <- struct{}{}:
				default:
				}
				<-release
				fmt.Fprint(w, `{"total":0,"issues":[]}`)
				return
			}
			fmt.Fprint(w, `{"total":1,"issues":[{"key":"NS2-344","fields":{"summary":"Completed issue","created":"2026-03-17T17:55:00.000+0800","updated":"2026-10-19T13:41:58.000+0800","resolutiondate":"2026-10-19T13:41:58.000+0800","issuetype":{"name":"Task"},"status":{"name":"Done"},"project":{"key":"NS2"}}}]}`)
		} else {
			fmt.Fprint(w, `{"startAt":0,"total":0,"comments":[]}`)
		}
	}))
	defer jira.Close()
	srv := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: true, BaseURL: jira.URL, CustomJQL: `project = NS2 AND status != Done`}}, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		close(release)
		srv.jiraInboundSyncMu.Lock()
		srv.jiraInboundSyncMu.Unlock()
		srv.jiraCurrentSyncMu.Lock()
		srv.jiraCurrentSyncMu.Unlock()
	}()
	updates := make(chan string, 8)
	registerTelemetryClient(updates)
	defer unregisterTelemetryClient(updates)
	srv.startJiraSyncWorkers(ctx)
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("historical search did not block")
	}
	select {
	case key := <-updates:
		if key != "NS2-344" {
			t.Fatalf("unexpected update: %s", key)
		}
		var task db.TaskTelemetry
		if err := db.DB.First(&task, "task_id = ?", key).Error; err != nil || task.Status != "done" {
			t.Fatalf("notified before completion persisted: %+v %v", task, err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Jira completion stayed stale because the production worker is waiting for an unrelated historical search")
	}
}

func TestJiraProjectionCursorAdvancesDespitePerformanceConflict(t *testing.T) {
	setupServerTestDB(t)
	oldPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldPath })
	const updated = "2026-10-19T13:41:58.000+0800"
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/rest/api/2/search" {
			fmt.Fprint(w, `{"total":1,"issues":[{"key":"NS2-344","fields":{"summary":"Completed issue","created":"2026-03-17T17:55:00.000+0800","updated":"`+updated+`","resolutiondate":"`+updated+`","issuetype":{"name":"Task"},"status":{"name":"Done"},"project":{"key":"NS2"}}}]}`)
		} else {
			fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":0,"comments":[]}`)
		}
	}))
	defer jira.Close()
	srv := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: true, BaseURL: jira.URL, CustomJQL: `project = NS2`}, PerformanceBrain: config.PerformanceBrainConfig{Enabled: true}}, "")
	oldWatermark := time.Now().Add(-7 * 24 * time.Hour)
	if err := db.DB.Create(&[]db.JiraInboundSyncState{
		{Scope: jiraInboundSyncScope, SuccessfulThrough: oldWatermark},
		{Scope: jiraCurrentSyncScope, SuccessfulThrough: oldWatermark},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&db.TaskTelemetry{TaskID: "NS2-344", Source: "jira", ExternalKey: "NS2-344", ProjectKey: "NS2", IssueType: "requirement", Status: "backlog", LastUpdate: oldWatermark}).Error; err != nil {
		t.Fatal(err)
	}
	_, err := srv.performance.AppendSourceEvent(context.Background(), performance.SourceEventCommand{
		DedupeKey: "jira:NS2-344:snapshot:" + updated, WorkItemID: "NS2-344", ProjectKey: "NS2", IssueType: "requirement", EventType: performance.SourceEventIssueSnapshot,
		OccurredAt: parseJiraTime(updated), SourceSystem: "jira", SourceEventID: "NS2-344:" + updated, Actor: "jira-sync", Payload: map[string]any{"status": "retained incompatible snapshot"},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv.syncCurrentJiraFacts()
	var task db.TaskTelemetry
	if err := db.DB.First(&task, "task_id = ?", "NS2-344").Error; err != nil || task.Status != "done" {
		t.Fatalf("completion was not persisted: %+v %v", task, err)
	}
	var cursor db.JiraInboundSyncState
	if err := db.DB.First(&cursor, "scope = ?", jiraCurrentSyncScope).Error; err != nil {
		t.Fatal(err)
	}
	if !cursor.SuccessfulThrough.After(oldWatermark) || cursor.LastError != "" {
		t.Fatalf("committed status facts are pinned behind performance conflict; next cycle replays old history: %+v", cursor)
	}
	// The immutable ledger must remain unchanged; enrichment has its own error/retry state.
	var event db.PerformanceWorkItemEvent
	if err := db.DB.First(&event, "dedupe_key = ?", "jira:NS2-344:snapshot:"+updated).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(event.PayloadJSON, "retained incompatible snapshot") {
		t.Fatal("retained source evidence was modified")
	}
	// The full lane still reports and retries the conflict instead of clearing it.
	srv.syncJiraTasks()
	var history db.JiraInboundSyncState
	if err := db.DB.First(&history, "scope = ?", jiraInboundSyncScope).Error; err != nil {
		t.Fatal(err)
	}
	if !history.SuccessfulThrough.Equal(oldWatermark) || !strings.Contains(history.LastError, "source event conflicts") {
		t.Fatalf("historical failure was hidden or its retry cursor advanced: %+v", history)
	}
}

func TestCurrentJiraReadFailureDoesNotAdvanceCursor(t *testing.T) {
	setupServerTestDB(t)
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Jira temporarily unavailable", http.StatusServiceUnavailable)
	}))
	defer jira.Close()
	cfg := &config.Config{Jira: config.JiraConfig{Enabled: true, BaseURL: jira.URL, SyncProjects: []string{"NS2"}}}
	srv := NewServer(cfg, "")
	watermark := time.Now().Add(-time.Minute).Truncate(time.Microsecond)
	if err := db.DB.Create(&db.JiraInboundSyncState{Scope: jiraCurrentSyncScope, SuccessfulThrough: watermark}).Error; err != nil {
		t.Fatal(err)
	}
	srv.syncCurrentJiraFacts()
	var cursor db.JiraInboundSyncState
	if err := db.DB.First(&cursor, "scope = ?", jiraCurrentSyncScope).Error; err != nil {
		t.Fatal(err)
	}
	if !cursor.SuccessfulThrough.Equal(watermark) || cursor.LastError == "" {
		t.Fatalf("failed Jira request must retain its retry cursor and error: %+v", cursor)
	}
}

func TestHistoricalJiraSnapshotCannotRevertNewerCompletion(t *testing.T) {
	setupServerTestDB(t)
	completed := parseOptionalJiraTime("2026-10-19T13:41:58.000+0800")
	if err := db.DB.Create(&db.TaskTelemetry{TaskID: "NS2-344", Source: "jira", ExternalKey: "NS2-344", ProjectKey: "NS2", Status: "done", SourceUpdatedAt: completed}).Error; err != nil {
		t.Fatal(err)
	}
	srv := NewServer(&config.Config{}, "")
	var issue telemetry.JiraIssue
	issue.Key = "NS2-344"
	issue.Fields.Status.Name = "Open"
	issue.Fields.Project.Key = "NS2"
	issue.Fields.Updated = "2026-10-19T13:00:00.000+0800"
	if changed, err := srv.reconcileJiraIssueFacts(issue); err != nil || changed {
		t.Fatalf("stale inbound snapshot changed newer facts: changed=%v err=%v", changed, err)
	}
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"total":1,"issues":[{"key":"NS2-344","fields":{"summary":"Old snapshot","created":"2026-03-17T17:55:00.000+0800","updated":"2026-10-19T13:00:00.000+0800","duedate":"2026-10-12","issuetype":{"name":"Task"},"status":{"name":"Open"},"project":{"key":"NS2"}}}]}`)
	}))
	defer jira.Close()
	srv.config = &config.Config{Jira: config.JiraConfig{Enabled: true, BaseURL: jira.URL, SyncProjects: []string{"NS2"}, SyncUsers: []string{"owner"}}, PerformanceBrain: config.PerformanceBrainConfig{Enabled: true}}
	if _, err := srv.syncPerformanceJiraHistory(telemetry.NewJiraClient(&srv.config.Jira)); err != nil {
		t.Fatal(err)
	}
	var task db.TaskTelemetry
	if err := db.DB.First(&task, "task_id = ?", "NS2-344").Error; err != nil {
		t.Fatal(err)
	}
	if task.Status != "done" || !task.SourceUpdatedAt.Equal(completed) {
		t.Fatalf("historical snapshot reverted newer completion: %+v", task)
	}
}

func TestCurrentJiraStartupFailureRetainsInitialWindow(t *testing.T) {
	setupServerTestDB(t)
	anchor := time.Now().Add(-time.Hour).Truncate(time.Microsecond)
	var query string
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query().Get("jql")
		http.Error(w, "Jira temporarily unavailable", http.StatusServiceUnavailable)
	}))
	defer jira.Close()
	srv := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: true, BaseURL: jira.URL, SyncProjects: []string{"NS2"}}}, "")
	if err := db.DB.Create(&db.JiraInboundSyncState{Scope: jiraCurrentSyncScope, LastStartedAt: anchor}).Error; err != nil {
		t.Fatal(err)
	}
	srv.syncCurrentJiraFacts()
	var cursor db.JiraInboundSyncState
	if err := db.DB.First(&cursor, "scope = ?", jiraCurrentSyncScope).Error; err != nil {
		t.Fatal(err)
	}
	if !cursor.SuccessfulThrough.IsZero() || !cursor.LastStartedAt.Equal(anchor) || cursor.LastError == "" {
		t.Fatalf("failed first cycle lost its original retry anchor: %+v", cursor)
	}
	if !strings.Contains(query, anchor.Add(-jiraInboundSyncOverlap).In(time.Local).Format("2006-01-02 15:04")) {
		t.Fatalf("retry query skipped original startup window: %s", query)
	}
}

func TestCurrentJiraFactsReconcileKnownIssuesAfterTheyLeaveActiveFilter(t *testing.T) {
	setupServerTestDB(t)
	oldPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldPath })
	if err := db.DB.Create(&[]db.TaskTelemetry{
		{TaskID: "NS2-2569", Source: "jira", ProjectKey: "NS2", Status: "backlog", Assignee: "old-owner"},
		{TaskID: "NS2-900", Source: "agent", ProjectKey: "NS2", Status: "progress"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	var query string
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query().Get("jql")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"total":3,"issues":[{"key":"NS2-2569","fields":{"updated":"2026-10-19T13:41:58.000+0800","status":{"name":"Done"},"assignee":{"name":"new-owner","displayName":"New Owner"},"project":{"key":"NS2"}}},{"key":"NS2-900","fields":{"status":{"name":"Done"},"project":{"key":"NS2"}}},{"key":"NS2-999","fields":{"status":{"name":"Done"},"project":{"key":"NS2"}}}]}`)
	}))
	defer jira.Close()
	srv := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: true, BaseURL: jira.URL, CustomJQL: `project = NS2 AND assignee = "old-owner" AND status != Done`}}, "")
	updates := make(chan string, 8)
	registerTelemetryClient(updates)
	defer unregisterTelemetryClient(updates)
	srv.syncCurrentJiraFacts()
	if strings.Contains(strings.ToLower(query), "assignee") || strings.Contains(strings.ToLower(query), "status") || !strings.Contains(query, "updated >=") {
		t.Fatalf("current facts reused the exclusionary active-work filter: %s", query)
	}
	var task db.TaskTelemetry
	if err := db.DB.First(&task, "task_id = ?", "NS2-2569").Error; err != nil || task.Status != "done" || task.Assignee != "New Owner" {
		t.Fatalf("reassigned completed issue stayed stale: %+v err=%v", task, err)
	}
	select {
	case event := <-updates:
		if event != "NS2-2569" {
			t.Fatalf("wrong issue notification: %+v", event)
		}
	default:
		t.Fatal("committed current facts did not notify the frontend")
	}
	var manual db.TaskTelemetry
	if err := db.DB.First(&manual, "task_id = ?", "NS2-900").Error; err != nil || manual.Status != "progress" {
		t.Fatalf("current Jira facts overwrote a non-Jira task: %+v err=%v", manual, err)
	}
	var count int64
	if err := db.DB.Model(&db.TaskTelemetry{}).Where("task_id = ?", "NS2-999").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("current lane expanded issue discovery scope: count=%d err=%v", count, err)
	}
}
