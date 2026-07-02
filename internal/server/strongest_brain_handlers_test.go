package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

func seedStrongestBrainUser(t *testing.T) string {
	t.Helper()
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}
	u := userdb.User{
		Username:   "brain-user",
		Email:      "brain-user@westwell-lab.com",
		Name:       "Brain User",
		Department: "AI Platform",
	}
	if err := db.DB.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	var group userdb.UserGroup
	if err := db.DB.Where("name = ?", "member").First(&group).Error; err != nil {
		t.Fatalf("query member group: %v", err)
	}
	if err := db.DB.Create(&userdb.UserGroupMembership{UserID: u.ID, UserGroupID: group.ID, Scope: "global"}).Error; err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	token, err := GenerateJWT(u.Username, u.Name, "mock-token", "", []string{"member"}, []string{"dashboard:read", "demands:read", "decision:read"}, u.Department)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return token
}

func TestStrongestBrainDecisionQueueBuildsScheduleAndEvidenceItems(t *testing.T) {
	token := seedStrongestBrainUser(t)
	now := time.Now()
	due := now.AddDate(0, 0, -2)
	demand := db.TaskTelemetry{
		TaskID:        "DEMAND-1",
		Title:         "逾期需求",
		IssueType:     "demand",
		Status:        "progress",
		Assignee:      "Brain User",
		Branch:        "feature/late",
		DueDate:       &due,
		TaskGroupID:   "brain-demand-1",
		TaskCreatedAt: now.AddDate(0, 0, -5),
		LastUpdate:    now.AddDate(0, 0, -4),
	}
	task := db.TaskTelemetry{
		TaskID:        "TASK-1",
		Title:         "已合并待回写",
		IssueType:     "task",
		Status:        "review",
		Assignee:      "Brain User",
		Repo:          "well-ambient",
		Branch:        "feature/late",
		TaskGroupID:   "brain-demand-1",
		TaskCreatedAt: now.AddDate(0, 0, -3),
		LastUpdate:    now.AddDate(0, 0, -2),
	}
	if err := db.DB.Create(&demand).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if err := db.DB.Create(&db.GitCommitLog{
		TaskID:    "TASK-1",
		Repo:      "well-ambient",
		Branch:    "feature/late",
		Action:    "mr_merge",
		MrURL:     "https://gitlab.example/mr/1",
		CreatedAt: now.Add(-2 * time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed git log: %v", err)
	}

	srv := NewServer(&config.Config{Server: config.ServerConfig{Host: "127.0.0.1", Port: 8080}}, "")
	req := httptest.NewRequest(http.MethodGet, "/api/strongest-brain/decision-queue", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var response StrongestBrainDecisionQueueResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Summary.Total < 2 {
		t.Fatalf("expected schedule and evidence decisions, got %+v", response.Summary)
	}
	if response.Summary.Critical == 0 {
		t.Fatalf("expected critical decision in %+v", response.Summary)
	}
}

func TestStrongestBrainEvidenceChainReturnsRelatedTasksAndLogs(t *testing.T) {
	token := seedStrongestBrainUser(t)
	now := time.Now()
	root := db.TaskTelemetry{
		TaskID:      "DEMAND-2",
		Title:       "需求证据链",
		IssueType:   "demand",
		Status:      "progress",
		TaskGroupID: "brain-demand-2",
		LastUpdate:  now,
	}
	child := db.TaskTelemetry{
		TaskID:      "TASK-2",
		Title:       "子任务",
		IssueType:   "task",
		Status:      "progress",
		TaskGroupID: "brain-demand-2",
		LastUpdate:  now,
	}
	if err := db.DB.Create(&root).Error; err != nil {
		t.Fatalf("seed root: %v", err)
	}
	if err := db.DB.Create(&child).Error; err != nil {
		t.Fatalf("seed child: %v", err)
	}
	if err := db.DB.Create(&db.GitCommitLog{TaskID: "TASK-2", Action: "git_push", CommitID: "abc123", CreatedAt: now}).Error; err != nil {
		t.Fatalf("seed log: %v", err)
	}

	srv := NewServer(&config.Config{Server: config.ServerConfig{Host: "127.0.0.1", Port: 8080}}, "")
	req := httptest.NewRequest(http.MethodGet, "/api/strongest-brain/evidence-chain?task_id=DEMAND-2", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var response StrongestBrainEvidenceChainResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Summary.RelatedTasks != 2 || response.Summary.Commits != 1 {
		t.Fatalf("unexpected evidence chain summary: %+v", response.Summary)
	}
}

func TestAIIntentSummaryDeterministicFallback(t *testing.T) {
	token := seedStrongestBrainUser(t)
	srv := NewServer(&config.Config{Server: config.ServerConfig{Host: "127.0.0.1", Port: 8080}}, "")
	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"请总结本周逾期风险，并给出下次周会要问的问题"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/intent-summary", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var response AIIntentResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Intent != "summary" {
		t.Fatalf("intent = %q, want summary", response.Intent)
	}
	if response.Summary == "" || len(response.NextQuestions) == 0 || response.Source != "deterministic" {
		t.Fatalf("unexpected intent response: %+v", response)
	}
}
