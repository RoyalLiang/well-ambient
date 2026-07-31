package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

type jiraSyncCall struct {
	issueKey string
	value    string
}

func receiveJiraSyncCall(t *testing.T, calls <-chan jiraSyncCall) jiraSyncCall {
	t.Helper()
	select {
	case call := <-calls:
		return call
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Jira sync")
		return jiraSyncCall{}
	}
}

func TestScheduleTaskSyncsDueDateAfterLocalSave(t *testing.T) {
	setupServerTestDB(t)
	useTempKanbanFile(t)
	_ = superAdminToken(t, "pm-sync@example.com", "PM Sync", []string{"demands:write"})
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID:     "JIRA-501",
		Title:      "需要同步到期日",
		IssueType:  "demand",
		Status:     "backlog",
		Assignee:   "Owner",
		LastUpdate: time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	dueDateCalls := make(chan jiraSyncCall, 1)
	server := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: true}}, "")
	server.jiraDueDateSync = func(issueKey, dueDate string) {
		dueDateCalls <- jiraSyncCall{issueKey: issueKey, value: dueDate}
	}

	request := httptest.NewRequest(http.MethodPost, "/api/tasks/schedule", bytes.NewBufferString(`{
		"task_id": "JIRA-501",
		"due_date": "2026-08-18"
	}`))
	request.Header.Set("x-authenticated-user-id", "pm-sync@example.com")
	request.Header.Set("x-authenticated-user-name", "PM Sync")
	recorder := httptest.NewRecorder()
	server.handleScheduleTask(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("schedule status = %d, body=%s", recorder.Code, recorder.Body.String())
	}

	call := receiveJiraSyncCall(t, dueDateCalls)
	if call.issueKey != "JIRA-501" || call.value != "2026-08-18" {
		t.Fatalf("due date sync = %+v", call)
	}
	var saved db.TaskTelemetry
	if err := db.DB.First(&saved, "task_id = ?", "JIRA-501").Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if saved.DueDate == nil || saved.DueDate.Format("2006-01-02") != "2026-08-18" {
		t.Fatalf("saved due date = %v", saved.DueDate)
	}
}

func TestDecisionRescheduleSyncsDueDateAndExactMeetingNoteComment(t *testing.T) {
	setupServerTestDB(t)
	oldDueDate := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.Local)
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID:     "JIRA-502",
		Title:      "决策调整排期",
		IssueType:  "bug",
		Status:     "progress",
		Assignee:   "Owner",
		DueDate:    &oldDueDate,
		LastUpdate: time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	dueDateCalls := make(chan jiraSyncCall, 1)
	commentCalls := make(chan jiraSyncCall, 1)
	server := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: true}}, "")
	server.jiraDueDateSync = func(issueKey, dueDate string) {
		dueDateCalls <- jiraSyncCall{issueKey: issueKey, value: dueDate}
	}
	server.jiraCommentSync = func(issueKey, comment string) {
		commentCalls <- jiraSyncCall{issueKey: issueKey, value: comment}
	}

	request := httptest.NewRequest(http.MethodPost, "/api/strongest-brain/intervention", bytes.NewBufferString(`{
		"task_id": "JIRA-502",
		"action": "reschedule",
		"value": "2026-08-22",
		"reason": "旧客户端默认原因不应成为 Jira 评论",
		"meeting_note": "决策会确认等待接口联调后交付"
	}`))
	request.Header.Set("x-authenticated-user-id", "pm-sync@example.com")
	request.Header.Set("x-authenticated-user-name", "PM Sync")
	recorder := httptest.NewRecorder()
	server.handleStrongestBrainIntervention(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("intervention status = %d, body=%s", recorder.Code, recorder.Body.String())
	}

	dueDateCall := receiveJiraSyncCall(t, dueDateCalls)
	if dueDateCall.issueKey != "JIRA-502" || dueDateCall.value != "2026-08-22" {
		t.Fatalf("due date sync = %+v", dueDateCall)
	}
	commentCall := receiveJiraSyncCall(t, commentCalls)
	if commentCall.issueKey != "JIRA-502" || commentCall.value != "决策会确认等待接口联调后交付" {
		t.Fatalf("comment sync = %+v", commentCall)
	}
}

func TestDecisionInterventionWithoutMeetingNoteDoesNotCreateJiraComment(t *testing.T) {
	setupServerTestDB(t)
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID:     "JIRA-503",
		Title:      "无会议备注调停",
		IssueType:  "bug",
		Status:     "progress",
		Assignee:   "Owner",
		LastUpdate: time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	commentCalls := make(chan jiraSyncCall, 1)
	server := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: true}}, "")
	server.jiraAssigneeSync = func(issueKey, assignee string) {}
	server.jiraCommentSync = func(issueKey, comment string) {
		commentCalls <- jiraSyncCall{issueKey: issueKey, value: comment}
	}

	request := httptest.NewRequest(http.MethodPost, "/api/strongest-brain/intervention", bytes.NewBufferString(`{
		"task_id": "JIRA-503",
		"action": "reassign",
		"value": "New Owner",
		"reason": "人工调停干预",
		"meeting_note": ""
	}`))
	recorder := httptest.NewRecorder()
	server.handleStrongestBrainIntervention(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("intervention status = %d, body=%s", recorder.Code, recorder.Body.String())
	}

	select {
	case call := <-commentCalls:
		t.Fatalf("unexpected Jira comment sync = %+v", call)
	case <-time.After(100 * time.Millisecond):
	}
}
