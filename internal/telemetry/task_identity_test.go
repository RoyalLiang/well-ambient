package telemetry

import (
	"fmt"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/kanban"
)

func TestProcessWebhookEventCanonicalizesExistingJiraTaskAcrossRepositories(t *testing.T) {
	oldDB := db.DB
	oldKanbanPath := kanban.KanbanFilePath
	t.Cleanup(func() {
		db.DB = oldDB
		kanban.KanbanFilePath = oldKanbanPath
	})

	var err error
	db.DB, err = gorm.Open(sqlite.Open("file:telemetry_task_identity?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	if err := db.DB.AutoMigrate(&db.TaskTelemetry{}, &db.GitCommitLog{}, &db.Notification{}); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")

	existing := db.TaskTelemetry{
		TaskID:        "FZ-2247",
		ExternalKey:   "FZ-2247",
		Source:        "jira",
		IssueType:     "requirement",
		Title:         "吊具检测驶离保护",
		Assignee:      "梁志远",
		Status:        "progress",
		PlanningState: "ready",
	}
	if err := db.DB.Create(&existing).Error; err != nil {
		t.Fatalf("seed Jira work item: %v", err)
	}

	cfg := &config.Config{Jira: config.JiraConfig{Enabled: false}}
	for index, repo := range []string{"task_executor", "crane_manager"} {
		commits := fmt.Sprintf(`[{"id":%q,"message":"feat: FZ-2247 添加吊具检测驶离保护","author":{"name":"梁志远"}}]`, fmt.Sprintf("commit-%d", index+1))
		if repo == "crane_manager" {
			commits = fmt.Sprintf(`[{"id":%q,"message":"feat: FZ-2247 添加吊具检测驶离保护","author":{"name":"梁志远"}},{"id":"follow-up","message":"chore: update generated metadata","author":{"name":"梁志远"}}]`, fmt.Sprintf("commit-%d", index+1))
		}
		payload := fmt.Sprintf(`{
			"object_kind":"push",
			"ref":"refs/heads/dev_fuzhou",
			"project":{"name":%q},
			"commits":%s
		}`, repo, commits)
		if err := ProcessWebhookEvent(cfg, "Push Hook", []byte(payload)); err != nil {
			t.Fatalf("process %s push: %v", repo, err)
		}
	}

	var duplicateCount int64
	if err := db.DB.Model(&db.TaskTelemetry{}).Where("task_id = ?", "fz-2247").Count(&duplicateCount).Error; err != nil {
		t.Fatalf("count lowercase duplicate: %v", err)
	}
	if duplicateCount != 0 {
		t.Fatalf("lowercase duplicate count = %d, want 0", duplicateCount)
	}

	var canonical db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", "FZ-2247").First(&canonical).Error; err != nil {
		t.Fatalf("load canonical Jira work item: %v", err)
	}
	if canonical.Source != "jira" || canonical.Assignee != "梁志远" {
		t.Fatalf("canonical Jira ownership = source %q assignee %q, want jira/梁志远", canonical.Source, canonical.Assignee)
	}

	var logs []db.GitCommitLog
	if err := db.DB.Where("task_id = ?", "FZ-2247").Order("repo asc").Find(&logs).Error; err != nil {
		t.Fatalf("load canonical commit evidence: %v", err)
	}
	if len(logs) != 3 {
		t.Fatalf("canonical commit evidence count = %d, want 3", len(logs))
	}
	if logs[0].Repo != "crane_manager" || logs[1].Repo != "crane_manager" || logs[2].Repo != "task_executor" {
		t.Fatalf("canonical repositories = [%s, %s, %s], want [crane_manager, crane_manager, task_executor]", logs[0].Repo, logs[1].Repo, logs[2].Repo)
	}
}
