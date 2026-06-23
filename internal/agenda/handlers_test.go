package agenda

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/db"
	"well-ambient/internal/kanban"
)

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
