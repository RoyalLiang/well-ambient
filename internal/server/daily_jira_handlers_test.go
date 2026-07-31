package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/kanban"
)

func TestBuildDailyJiraAuditResponseUsesNonOverlappingNaturalDayBuckets(t *testing.T) {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, time.July, 18, 9, 30, 0, 0, location)
	created := func(daysAgo int) time.Time {
		return now.AddDate(0, 0, -daysAgo).Add(-2 * time.Hour)
	}

	tasks := []db.TaskTelemetry{
		{TaskID: "WA-100", Title: "today", Repo: "Well Ambient (WA)", Status: "backlog", Assignee: "Alice", TaskCreatedAt: created(0), LastUpdate: created(0)},
		{TaskID: "WA-102", Title: "watch", Repo: "Well Ambient (WA)", Status: "progress", Assignee: "Alice", TaskCreatedAt: created(2), LastUpdate: created(1)},
		{TaskID: "WA-103", Title: "three", Repo: "Well Ambient (WA)", Status: "progress", Assignee: "Bob", TaskCreatedAt: created(3), LastUpdate: created(2)},
		{TaskID: "WA-106", Title: "six", Repo: "Well Ambient (WA)", Status: "review", Assignee: "Bob", TaskCreatedAt: created(6), LastUpdate: created(4)},
		{TaskID: "WA-107", Title: "seven", Repo: "Well Ambient (WA)", Status: "progress", Assignee: "Carol", TaskCreatedAt: created(7), LastUpdate: created(7)},
		{TaskID: "WA-120", Title: "done", Repo: "Well Ambient (WA)", Status: "done", Assignee: "Alice", TaskCreatedAt: created(12), LastUpdate: created(1)},
		{TaskID: "TASK-999", Title: "local task", Repo: "Well Ambient (WA)", Status: "progress", Assignee: "Alice", TaskCreatedAt: created(9), LastUpdate: created(2)},
		{TaskID: "DEMAND-001", Title: "local demand", Repo: "Well Ambient (WA)", Status: "progress", Assignee: "Alice", Creator: "Alice", TaskCreatedAt: created(9), LastUpdate: created(2)},
		{TaskID: "WA-130", Title: "unknown age", Repo: "Well Ambient (WA)", Status: "progress", Assignee: "Alice"},
	}
	events := []db.DecisionEvent{{TaskID: "WA-107", Action: "daily_jira_escalate", Actor: "PM", CreatedAt: now.Add(-time.Hour)}}
	decisions := []db.DailyJiraDecision{{TaskID: "WA-107", Status: "escalate", Actor: "PM", ReminderAt: now.Add(-time.Minute), CreatedAt: now.Add(-5 * time.Hour)}}

	response := buildDailyJiraAuditResponse(tasks, events, decisions, []string{"Alice", "Bob", "Carol"}, now)
	if response.Summary.Total != 4 || response.Summary.Today != 1 || response.Summary.ThreeDay != 2 || response.Summary.SevenDay != 1 {
		t.Fatalf("unexpected summary: %+v", response.Summary)
	}
	if response.RecentWatchCount != 1 {
		t.Fatalf("recent watch count = %d, want 1", response.RecentWatchCount)
	}
	if response.UnclassifiedCount != 1 {
		t.Fatalf("unclassified count = %d, want 1", response.UnclassifiedCount)
	}
	if len(response.Buckets) != 3 || response.Buckets[1].Items[0].TaskID != "WA-106" || response.Buckets[2].Items[0].DecisionEvents[0].Action != "daily_jira_escalate" {
		t.Fatalf("unexpected bucket content: %+v", response.Buckets)
	}
	latest := response.Buckets[2].Items[0].LatestDecision
	if latest == nil || latest.Status != "escalate" || !latest.ReminderDue {
		t.Fatalf("unexpected latest decision projection: %+v", latest)
	}
}

func TestDailyJiraVisibilityKeepsUnassignedAndExcludesExternalAssignees(t *testing.T) {
	server := NewServer(&config.Config{Jira: config.JiraConfig{SyncUsers: []string{"Alice"}}}, "")
	visibility := coreMemberVisibility{
		filter:    server.buildKPICoreMemberFilter(nil),
		directory: newKPIUserDirectory(nil),
	}
	if !isDailyJiraAssigneeVisible(visibility, "") || !isDailyJiraAssigneeVisible(visibility, "未指派") {
		t.Fatal("unassigned Jira must remain visible for morning assignment")
	}
	if !isDailyJiraAssigneeVisible(visibility, "Alice") {
		t.Fatal("configured core assignee should be visible")
	}
	if isDailyJiraAssigneeVisible(visibility, "External Vendor") {
		t.Fatal("external assignee should remain outside the daily Jira audit scope")
	}
}

func TestPostDailyJiraReviewPersistsDecisionAndReassignment(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}

	tmpDir := t.TempDir()
	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(tmpDir, "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })

	originalUpdate := time.Date(2026, time.July, 17, 8, 0, 0, 0, time.Local)
	task := db.TaskTelemetry{
		TaskID:        "WA-207",
		Title:         "Seven day unresolved Jira",
		Repo:          "Well Ambient (WA)",
		Assignee:      "Alice",
		Status:        "progress",
		IssueType:     "bug",
		TaskCreatedAt: time.Date(2026, time.July, 11, 8, 0, 0, 0, time.Local),
		LastUpdate:    originalUpdate,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if err := kanban.SyncTaskToKanban(&task); err != nil {
		t.Fatalf("seed kanban: %v", err)
	}

	server := NewServer(&config.Config{}, "")
	body := bytes.NewBufferString(`{"task_id":"WA-207","decision":"reassign","assignee":"Bob","note":"早会确认由 Bob 接手"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/decision/daily-jira/review", body)
	request.Header.Set("x-authenticated-user-id", "pm@westwell-lab.com")
	request.Header.Set("x-authenticated-user-name", "PM")
	response := httptest.NewRecorder()
	server.handlePostDailyJiraReview(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("review status = %d body %s", response.Code, response.Body.String())
	}

	var updated db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", task.TaskID).First(&updated).Error; err != nil {
		t.Fatalf("query updated task: %v", err)
	}
	if updated.Assignee != "Bob" || !strings.Contains(updated.DecisionLogs, "每日 Jira 审计：转派给 Bob") {
		t.Fatalf("unexpected updated task: %+v", updated)
	}

	var event db.DecisionEvent
	if err := db.DB.Where("task_id = ?", task.TaskID).First(&event).Error; err != nil {
		t.Fatalf("query decision event: %v", err)
	}
	if event.Action != "daily_jira_reassign" || event.Actor != "PM" || event.OldValue != "Alice" || event.NewValue != "Bob" || event.Reason != "早会确认由 Bob 接手" {
		t.Fatalf("unexpected event: %+v", event)
	}
	var decision db.DailyJiraDecision
	if err := db.DB.Where("task_id = ?", task.TaskID).First(&decision).Error; err != nil {
		t.Fatalf("query decision record: %v", err)
	}
	if decision.Status != "reassign" || decision.Assignee != "Bob" || decision.Actor != "PM" || decision.Note != "早会确认由 Bob 接手" {
		t.Fatalf("unexpected decision record: %+v", decision)
	}
	if reminderDelay := decision.ReminderAt.Sub(decision.CreatedAt); reminderDelay != 24*time.Hour {
		t.Fatalf("reassign reminder delay = %s, want 24h", reminderDelay)
	}

	kanbanContent, err := os.ReadFile(kanban.KanbanFilePath)
	if err != nil {
		t.Fatalf("read kanban: %v", err)
	}
	if !strings.Contains(string(kanbanContent), "| WA-207 | Seven day unresolved Jira | Well Ambient (WA) | Bob |") {
		t.Fatalf("kanban did not contain reassignment:\n%s", string(kanbanContent))
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload["status"] != "success" {
		t.Fatalf("unexpected response payload: %v err=%v", payload, err)
	}
}

func TestPostDailyJiraReviewRecordsFollowUpWithoutResettingActivity(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}

	tmpDir := t.TempDir()
	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(tmpDir, "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })

	originalUpdate := time.Date(2026, time.July, 14, 8, 0, 0, 0, time.Local)
	task := db.TaskTelemetry{
		TaskID:        "WA-303",
		Title:         "Three day follow up",
		Repo:          "Well Ambient (WA)",
		Assignee:      "Alice",
		Status:        "progress",
		IssueType:     "demand",
		TaskCreatedAt: originalUpdate,
		LastUpdate:    originalUpdate,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	server := NewServer(&config.Config{}, "")
	request := httptest.NewRequest(http.MethodPost, "/api/decision/daily-jira/review", bytes.NewBufferString(`{"task_id":"WA-303","decision":"follow_up","note":"下午同步联调结果"}`))
	request.Header.Set("x-authenticated-user-name", "PM")
	response := httptest.NewRecorder()
	server.handlePostDailyJiraReview(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("review status = %d body %s", response.Code, response.Body.String())
	}

	var updated db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", task.TaskID).First(&updated).Error; err != nil {
		t.Fatalf("query updated task: %v", err)
	}
	if !updated.LastUpdate.Equal(originalUpdate) {
		t.Fatalf("follow-up reset Jira activity: got %s want %s", updated.LastUpdate, originalUpdate)
	}
	if !strings.Contains(updated.DecisionLogs, "结论 [继续跟进]") && !strings.Contains(updated.DecisionLogs, "每日 Jira 审计：继续跟进") {
		t.Fatalf("follow-up decision log missing: %s", updated.DecisionLogs)
	}
	var decision db.DailyJiraDecision
	if err := db.DB.Where("task_id = ?", task.TaskID).First(&decision).Error; err != nil {
		t.Fatalf("query follow-up decision: %v", err)
	}
	if decision.Status != "follow_up" || decision.ReminderAt.Sub(decision.CreatedAt) != 24*time.Hour {
		t.Fatalf("unexpected follow-up decision reminder: %+v", decision)
	}
}

func TestDailyJiraEscalationUsesShorterReminderWindow(t *testing.T) {
	if got := dailyJiraReminderDelay("escalate"); got != 4*time.Hour {
		t.Fatalf("escalation reminder = %s, want 4h", got)
	}
	if got := dailyJiraReminderDelay("follow_up"); got != 24*time.Hour {
		t.Fatalf("follow-up reminder = %s, want 24h", got)
	}
	if got := dailyJiraReminderDelay("reassign"); got != 24*time.Hour {
		t.Fatalf("reassign reminder = %s, want 24h", got)
	}
}

func TestDailyJiraReminderAlertsUseLatestUnresolvedDecision(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}
	now := time.Now()
	tasks := []db.TaskTelemetry{
		{TaskID: "WA-401", Title: "new decision wins", Assignee: "Alice", Status: "progress"},
		{TaskID: "WA-402", Title: "resolved Jira", Assignee: "Bob", Status: "done"},
		{TaskID: "WA-403", Title: "escalation due", Assignee: "Carol", Status: "review"},
	}
	for _, task := range tasks {
		if err := db.DB.Create(&task).Error; err != nil {
			t.Fatalf("seed task %s: %v", task.TaskID, err)
		}
	}
	decisions := []db.DailyJiraDecision{
		{TaskID: "WA-401", Status: "follow_up", ReminderAt: now.Add(-time.Hour), CreatedAt: now.Add(-25 * time.Hour)},
		{TaskID: "WA-401", Status: "reassign", ReminderAt: now.Add(time.Hour), CreatedAt: now.Add(-time.Hour)},
		{TaskID: "WA-402", Status: "follow_up", ReminderAt: now.Add(-time.Hour), CreatedAt: now.Add(-25 * time.Hour)},
		{TaskID: "WA-403", Status: "escalate", Note: "等待平台团队响应", ReminderAt: now.Add(-time.Minute), CreatedAt: now.Add(-4*time.Hour - time.Minute)},
	}
	if err := db.DB.Create(&decisions).Error; err != nil {
		t.Fatalf("seed decisions: %v", err)
	}

	server := NewServer(&config.Config{}, "")
	alerts := server.computeDailyJiraDecisionAlerts()
	if len(alerts) != 1 {
		t.Fatalf("alerts = %+v, want one due latest unresolved decision", alerts)
	}
	if alerts[0].TaskID != "WA-403" || alerts[0].Type != "daily_jira_reminder" || alerts[0].Severity != "critical" || alerts[0].Status != "escalate" {
		t.Fatalf("unexpected reminder alert: %+v", alerts[0])
	}
	if !strings.Contains(alerts[0].Message, "等待平台团队响应") {
		t.Fatalf("reminder does not contain decision context: %s", alerts[0].Message)
	}
}

func TestDailyJiraReminderCanBeDismissedPerUser(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}
	task := db.TaskTelemetry{TaskID: "WA-404", Title: "dismiss reminder", Assignee: "Alice", Status: "progress"}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	decision := db.DailyJiraDecision{
		TaskID:     task.TaskID,
		Status:     "follow_up",
		Assignee:   task.Assignee,
		ReminderAt: time.Now().Add(-time.Minute),
		CreatedAt:  time.Now().Add(-24*time.Hour - time.Minute),
	}
	if err := db.DB.Create(&decision).Error; err != nil {
		t.Fatalf("seed decision: %v", err)
	}

	server := NewServer(&config.Config{}, "")
	if alerts := server.getMergedNotifications("pm@westwell-lab.com"); len(alerts) != 1 || alerts[0].Type != "daily_jira_reminder" {
		t.Fatalf("expected visible Jira reminder before dismissal, got %+v", alerts)
	}
	state := db.UserNotificationState{
		NotificationKey: fmt.Sprintf("daily_jira_reminder_%d", decision.ID),
		UserID:          "pm@westwell-lab.com",
		Status:          "dismissed",
		UpdatedAt:       time.Now(),
	}
	if err := db.DB.Create(&state).Error; err != nil {
		t.Fatalf("seed dismissed state: %v", err)
	}
	if alerts := server.getMergedNotifications("pm@westwell-lab.com"); len(alerts) != 0 {
		t.Fatalf("dismissed Jira reminder remained visible: %+v", alerts)
	}
}
