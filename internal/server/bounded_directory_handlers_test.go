package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

func TestTaskActivityUsesBoundedRecentWindow(t *testing.T) {
	setupServerTestDB(t)
	server := NewServer(&config.Config{}, "")
	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID: "FZ-BOUNDED", ExternalKey: "FZ-BOUNDED", Source: "jira",
		IssueType: "bug", Status: "active", Title: "bounded timeline",
	}).Error; err != nil {
		t.Fatal(err)
	}
	gitRows := make([]db.GitCommitLog, 0, 125)
	commentRows := make([]db.JiraCommentLog, 0, 125)
	for index := 0; index < 125; index++ {
		gitRows = append(gitRows, db.GitCommitLog{
			TaskID: "FZ-BOUNDED", CommitID: fmt.Sprintf("sha-%03d", index),
			Action: "git_push", CreatedAt: now.Add(-time.Duration(index*2+1) * time.Minute),
		})
		commentRows = append(commentRows, db.JiraCommentLog{
			TaskID: "FZ-BOUNDED", CommentID: fmt.Sprintf("comment-%03d", index),
			Current: true, Body: fmt.Sprintf("comment %03d", index),
			CreatedAt: now.Add(-time.Duration(index*2) * time.Minute),
		})
	}
	if err := db.DB.CreateInBatches(&gitRows, 50).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.CreateInBatches(&commentRows, 50).Error; err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	server.handleGetTaskCommits(recorder, httptest.NewRequest(http.MethodGet, "/api/tasks/commits?task_id=FZ-BOUNDED", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var activities []TelemetryActivityDTO
	if err := json.NewDecoder(recorder.Body).Decode(&activities); err != nil {
		t.Fatal(err)
	}
	if len(activities) != maxTaskActivityRows {
		t.Fatalf("activity rows = %d, want %d", len(activities), maxTaskActivityRows)
	}
	if activities[0].Action != "jira_comment" || !activities[0].CreatedAt.Equal(now) {
		t.Fatalf("newest activity was not retained: %+v", activities[0])
	}
}

func TestDemandSpecHistoryIsHardCapped(t *testing.T) {
	setupServerTestDB(t)
	server := NewServer(&config.Config{}, "")
	specs := make([]db.DemandSpecVersion, 0, 125)
	for version := 1; version <= 125; version++ {
		specs = append(specs, db.DemandSpecVersion{DemandID: "FZ-SPEC", Version: version, Status: "draft"})
	}
	if err := db.DB.CreateInBatches(&specs, 50).Error; err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	server.handleListDemandSpecs(recorder, httptest.NewRequest(http.MethodGet, "/api/demand-specs?demand_id=FZ-SPEC", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Items []demandSpecDTO `json:"items"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Items) != 100 || payload.Items[0].Version != 125 || payload.Items[99].Version != 26 {
		t.Fatalf("unexpected bounded spec window: len=%d first=%d last=%d", len(payload.Items), payload.Items[0].Version, payload.Items[len(payload.Items)-1].Version)
	}
}

func TestUserDirectoryBatchesMemberships(t *testing.T) {
	setupServerTestDB(t)
	server := NewServer(&config.Config{}, "")
	users := []userdb.User{
		{Username: "bounded-a", Email: "bounded-a@example.com", Name: "Bounded A"},
		{Username: "bounded-b", Email: "bounded-b@example.com", Name: "Bounded B"},
		{Username: "bounded-c", Email: "bounded-c@example.com", Name: "Bounded C"},
	}
	if err := db.DB.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	var queries atomic.Int64
	queryCallbackName := "test:user-directory-query-count"
	rowCallbackName := "test:user-directory-row-count"
	rawCallbackName := "test:user-directory-raw-count"
	countQuery := func(*gorm.DB) {
		queries.Add(1)
	}
	if err := db.DB.Callback().Query().Before("gorm:query").Register(queryCallbackName, countQuery); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Callback().Row().Before("gorm:row").Register(rowCallbackName, countQuery); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Callback().Raw().Before("gorm:raw").Register(rawCallbackName, countQuery); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.DB.Callback().Query().Remove(queryCallbackName)
		_ = db.DB.Callback().Row().Remove(rowCallbackName)
		_ = db.DB.Callback().Raw().Remove(rawCallbackName)
	})

	recorder := httptest.NewRecorder()
	server.handleGetUsers(recorder, httptest.NewRequest(http.MethodGet, "/api/users", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if got := queries.Load(); got != 2 {
		t.Fatalf("user directory queries = %d, want 2 regardless of user count", got)
	}
}
