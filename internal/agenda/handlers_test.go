package agenda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"well-ambient/internal/db"
	"well-ambient/internal/kanban"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type agendaQueryCounter struct {
	logger.Interface
	count atomic.Int64
}

func (counter *agendaQueryCounter) Trace(ctx context.Context, begin time.Time, sql func() (string, int64), err error) {
	counter.count.Add(1)
	counter.Interface.Trace(ctx, begin, sql, err)
}

func useAgendaTempKanbanFile(t *testing.T) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "well-ambient-agenda-kanban-test")
	if err != nil {
		t.Fatalf("create temp kanban dir: %v", err)
	}

	oldPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = tmpDir + "/task_status.md"
	t.Cleanup(func() {
		kanban.KanbanFilePath = oldPath
		os.RemoveAll(tmpDir)
	})
}

func TestPostAgendaDecisionReassignSyncsKanban(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}
	useAgendaTempKanbanFile(t)

	task := db.TaskTelemetry{
		TaskID:        "DEMAND-AG-1",
		Title:         "Agenda reassignment demand",
		Repo:          "platform-core",
		Assignee:      "Alice",
		Branch:        "-",
		LastCommit:    "-",
		Status:        "backlog",
		IssueType:     "demand",
		TaskCreatedAt: time.Now(),
		LastUpdate:    time.Now(),
	}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if err := kanban.SyncTaskToKanban(&task); err != nil {
		t.Fatalf("seed kanban: %v", err)
	}

	payload := map[string]interface{}{
		"task_id":  "DEMAND-AG-1",
		"action":   "reassign",
		"operator": "PM",
		"payload": map[string]string{
			"assignee": "Bob",
			"note":     "transfer in decision panel",
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/agenda/decision", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	HandlePostAgendaDecision(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("agenda decision status = %d body %s", rr.Code, rr.Body.String())
	}

	var updated db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", "DEMAND-AG-1").First(&updated).Error; err != nil {
		t.Fatalf("query updated task: %v", err)
	}
	if updated.Assignee != "Bob" {
		t.Fatalf("Assignee = %q, want Bob", updated.Assignee)
	}

	content, err := os.ReadFile(kanban.KanbanFilePath)
	if err != nil {
		t.Fatalf("read kanban: %v", err)
	}
	if !strings.Contains(string(content), "| DEMAND-AG-1 | Agenda reassignment demand | platform-core | Bob | - | - |") {
		t.Fatalf("kanban did not reflect reassignment:\n%s", string(content))
	}
}

func TestGetAgendaSummaryUsesBoundedBatchQueries(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	tasks := make([]db.TaskTelemetry, 0, 253)
	for index := 0; index < 250; index++ {
		tasks = append(tasks, db.TaskTelemetry{
			TaskID:     fmt.Sprintf("DONE-%03d", index),
			Title:      "historical work item",
			Repo:       "History Project (HIS)",
			Assignee:   "Alice",
			Status:     "done",
			IssueType:  "requirement",
			LastUpdate: now.Add(-time.Duration(index) * time.Minute),
		})
	}
	for index := 0; index < 3; index++ {
		tasks = append(tasks, db.TaskTelemetry{
			TaskID:        fmt.Sprintf("ACTIVE-%03d", index),
			Title:         "active work item",
			Repo:          "Active Project (ACT)",
			Assignee:      "Bob",
			Status:        "progress",
			IssueType:     "bug",
			TaskCreatedAt: now.Add(-time.Hour),
			LastUpdate:    now,
		})
	}
	if err := db.DB.CreateInBatches(&tasks, 50).Error; err != nil {
		t.Fatalf("seed tasks: %v", err)
	}
	if err := db.DB.Create(&db.GitCommitLog{
		TaskID: "DONE-000", CommitID: "abc123", Action: "git_push",
		Branch: "feature/DONE-000", Message: "DONE-000 implementation", CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed commit: %v", err)
	}
	if err := db.DB.Create(&db.Notification{
		Type: "git_push", TaskID: "DONE-000", Link: "https://git.example/commit/abc123", CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed notification: %v", err)
	}

	counter := &agendaQueryCounter{Interface: logger.Default.LogMode(logger.Silent)}
	db.DB = db.DB.Session(&gorm.Session{Logger: counter})

	req := httptest.NewRequest(http.MethodGet, "/api/agenda/summary", nil)
	recorder := httptest.NewRecorder()
	HandleGetAgendaSummary(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("agenda summary status = %d body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		TotalActiveTasks int            `json:"total_active_tasks"`
		AgendaItems      []AgendaItem   `json:"agenda_items"`
		AutoDecisions    []AutoDecision `json:"auto_decisions"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode agenda summary: %v", err)
	}
	if response.TotalActiveTasks != 3 || len(response.AgendaItems) != 3 {
		t.Fatalf("active summary = total %d items %d, want 3/3", response.TotalActiveTasks, len(response.AgendaItems))
	}
	queryCount := counter.count.Load()
	if len(response.AutoDecisions) != 200 || queryCount > 6 {
		t.Fatalf(
			"agenda summary returned %d auto decisions and executed %d SQL statements; want 200 decisions and at most 6 SQL statements",
			len(response.AutoDecisions),
			queryCount,
		)
	}
	var latest *AutoDecision
	for index := range response.AutoDecisions {
		if response.AutoDecisions[index].TaskID == "DONE-000" {
			latest = &response.AutoDecisions[index]
			break
		}
	}
	if latest == nil || latest.CommitID != "abc123" || latest.CommitURL != "https://git.example/commit/abc123" {
		t.Fatalf("latest preloaded commit evidence = %+v", latest)
	}
	t.Logf("bounded agenda summary: auto_decisions=%d sql=%d", len(response.AutoDecisions), queryCount)
}

func TestGetAgendaSummaryAppliesProjectScopeToEveryProjection(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	tasks := []db.TaskTelemetry{
		{TaskID: "HIT-1", ProjectKey: "HIT", Repo: "HIT Platform (HIT)", Status: "progress", IssueType: "requirement", LastUpdate: now},
		{TaskID: "HIT-2", ProjectKey: "HIT", Repo: "HIT Platform (HIT)", Status: "done", IssueType: "requirement", LastUpdate: now},
		{TaskID: "NS2-1", ProjectKey: "NS2", Repo: "Nansha Phase 2 (NS2)", Status: "progress", IssueType: "bug", LastUpdate: now},
		{TaskID: "NS2-2", ProjectKey: "NS2", Repo: "Nansha Phase 2 (NS2)", Status: "done", IssueType: "bug", LastUpdate: now},
	}
	if err := db.DB.Create(&tasks).Error; err != nil {
		t.Fatalf("seed tasks: %v", err)
	}
	if err := db.ReplaceUserProjectPreferences(db.DB, "alice@example.com", []string{"HIT"}); err != nil {
		t.Fatalf("seed project preference: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/agenda/summary", nil)
	request.Header.Set("x-authenticated-user-id", "alice@example.com")
	recorder := httptest.NewRecorder()
	HandleGetAgendaSummary(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("agenda summary status = %d body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		AgendaItems   []AgendaItem      `json:"agenda_items"`
		AutoDecisions []AutoDecision    `json:"auto_decisions"`
		ProjectMap    map[string]string `json:"project_map"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode agenda summary: %v", err)
	}
	if len(response.AgendaItems) != 1 || response.AgendaItems[0].TaskID != "HIT-1" {
		t.Fatalf("scoped agenda items = %+v", response.AgendaItems)
	}
	foundHITDecision := false
	for _, decision := range response.AutoDecisions {
		if decision.TaskID == "NS2-2" {
			t.Fatalf("project scope leaked NS2 decision: %+v", response.AutoDecisions)
		}
		if decision.TaskID == "HIT-2" {
			foundHITDecision = true
		}
	}
	if !foundHITDecision {
		t.Fatalf("scoped automatic decisions omitted HIT-2: %+v", response.AutoDecisions)
	}
	if len(response.ProjectMap) != 1 || response.ProjectMap["HIT"] != "HIT Platform" {
		t.Fatalf("scoped project map = %+v", response.ProjectMap)
	}
}
