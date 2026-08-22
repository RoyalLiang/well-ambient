package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
	"well-ambient/internal/deliveryplanning"
)

func TestCompleteWorkItemWithExactCommitDoesNotRequireRelease(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "delivery-owner@example.com", "Delivery Owner", []string{"demands:write"})

	task := db.TaskTelemetry{
		TaskID:        "FZ-2247",
		ExternalKey:   "FZ-2247",
		Source:        "jira",
		ProjectKey:    "FZ",
		IssueType:     "requirement",
		Title:         "吊具检测驶离保护",
		Assignee:      "Delivery Owner",
		Status:        "progress",
		PlanningState: deliveryplanning.PlanningDraft,
		LastUpdate:    time.Now().Add(-time.Hour),
	}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("seed work item: %v", err)
	}
	if err := db.DB.Create(&db.GitCommitLog{
		TaskID:    "fz-2247",
		Repo:      "task_executor",
		Branch:    "release",
		CommitID:  "cf2ad20",
		Message:   "feat: FZ-2247 添加吊具检测驶离保护",
		Author:    "Delivery Owner",
		Action:    "git_push",
		CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed exact commit evidence: %v", err)
	}

	server := NewServer(&config.Config{}, "")
	request := httptest.NewRequest(http.MethodPost, "/api/work-items/FZ-2247/complete", strings.NewReader(`{
		"expected_revision": 0,
		"reason": "已核对确切提交证据"
	}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("complete work item status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var completed db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", task.TaskID).First(&completed).Error; err != nil {
		t.Fatalf("load completed work item: %v", err)
	}
	if completed.Status != "done" || completed.CompletedAt == nil {
		t.Fatalf("completion state = status %q, completed_at %v", completed.Status, completed.CompletedAt)
	}
	if completed.PlanningState != deliveryplanning.PlanningDone {
		t.Fatalf("planning state = %q, want done", completed.PlanningState)
	}
	var releaseLinks int64
	if err := db.DB.Model(&db.WorkItemReleaseLink{}).Where("work_item_id = ?", task.TaskID).Count(&releaseLinks).Error; err != nil {
		t.Fatalf("count release links: %v", err)
	}
	if releaseLinks != 0 {
		t.Fatalf("release link count = %d, want 0", releaseLinks)
	}
	var event db.WorkItemEvent
	if err := db.DB.Where("work_item_id = ? AND event_type = ?", task.TaskID, "execution_completed").First(&event).Error; err != nil {
		t.Fatalf("load completion audit event: %v", err)
	}
}

func TestCompleteWorkItemAllowsAssigneeAndRejectsUnrelatedMember(t *testing.T) {
	setupServerTestDB(t)
	users := []userdb.User{
		{Username: "assignee@example.com", Email: "assignee@example.com", Name: "Delivery Owner"},
		{Username: "other@example.com", Email: "other@example.com", Name: "Other Member"},
	}
	if err := db.DB.Create(&users).Error; err != nil {
		t.Fatalf("seed members: %v", err)
	}
	assigneeToken, err := GenerateJWT(users[0].Username, users[0].Name, "mock-token", "", []string{"member"}, []string{"delivery:read"})
	if err != nil {
		t.Fatalf("generate assignee token: %v", err)
	}
	otherToken, err := GenerateJWT(users[1].Username, users[1].Name, "mock-token", "", []string{"member"}, []string{"delivery:read"})
	if err != nil {
		t.Fatalf("generate other token: %v", err)
	}

	seed := func(taskID string) {
		t.Helper()
		if err := db.DB.Create(&db.TaskTelemetry{
			TaskID: taskID, ExternalKey: taskID, ProjectKey: "FZ", Source: "jira",
			IssueType: "requirement", Assignee: users[0].Name, Status: "progress",
		}).Error; err != nil {
			t.Fatalf("seed %s: %v", taskID, err)
		}
		if err := db.DB.Create(&db.GitCommitLog{
			TaskID: taskID, Repo: "task_executor", CommitID: taskID + "-sha", Action: "git_push", CreatedAt: time.Now(),
		}).Error; err != nil {
			t.Fatalf("seed %s evidence: %v", taskID, err)
		}
	}
	seed("FZ-2250")
	seed("FZ-2251")

	server := NewServer(&config.Config{}, "")
	request := func(taskID, token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/work-items/"+taskID+"/complete", strings.NewReader(`{
			"expected_revision": 0,
			"reason": "confirmed exact commit evidence"
		}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		server.mux.ServeHTTP(recorder, req)
		return recorder
	}

	if recorder := request("FZ-2250", assigneeToken); recorder.Code != http.StatusOK {
		t.Fatalf("assignee completion status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder := request("FZ-2251", otherToken); recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "completion_forbidden") {
		t.Fatalf("unrelated member completion status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
}
