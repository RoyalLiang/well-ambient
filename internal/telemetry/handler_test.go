package telemetry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/kanban"
)

const sampleMarkdown = `# well-ambient 任务看板

以下任务看板由 well-ambient 服务根据 GitLab 提交流程自动流转。

## 待办任务 (Backlog)
| 任务ID | 任务标题 | 代码仓库 | 指派人 | 分支名称 | 最近提交 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| task-104 | 优化多仓大日志拉取性能与内存开销 | backend-core | 未指派 | - | - |

## 进行中 (In Progress)
| 任务ID | 任务标题 | 代码仓库 | 指派人 | 分支名称 | 最近提交 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| task-101 | 飞书多维表格（Bitable）双向同步对接 API | backend-core | Eddie | dev/task-101-feishu-sync | feat(#task-101): setup base client & auth request |

## 代码评审 (In Review)
| 任务ID | 任务标题 | 代码仓库 | 指派人 | 分支名称 | 最近提交 |
| :--- | :--- | :--- | :--- | :--- | :--- |

## 已完成 (Done)
| 任务ID | 任务标题 | 代码仓库 | 指派人 | 分支名称 | 最近提交 |
| :--- | :--- | :--- | :--- | :--- | :--- |
`

func TestProcessWebhookEvent(t *testing.T) {
	// 1. Setup in-memory DB
	var err error
	db.DB, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}
	err = db.DB.AutoMigrate(&db.WebhookLog{}, &db.TaskTelemetry{}, &db.GitCommitLog{}, &db.Notification{})
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	mockConfig := &config.Config{
		Jira: config.JiraConfig{
			Enabled: false,
		},
	}

	// 2. Setup temporary task_status.md
	tmpDir, err := os.MkdirTemp("", "telemetry_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFilePath := filepath.Join(tmpDir, "task_status.md")
	err = os.WriteFile(tmpFilePath, []byte(sampleMarkdown), 0644)
	if err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Backup and restore KanbanFilePath
	oldPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = tmpFilePath
	defer func() { kanban.KanbanFilePath = oldPath }()

	// 3. Test Push Hook (move task-104 to In Progress)
	pushPayload := `{
		"object_kind": "push",
		"ref": "refs/heads/dev/task-104-log-opt",
		"user_name": "Antigravity",
		"project": {
			"name": "backend-core",
			"web_url": "https://gitlab.example.com/group/backend-core"
		},
		"commits": [
			{
				"id": "1a2b3c4d5e",
				"message": "feat(#task-104): optimize log memory usage\n\nMore details...",
				"author": {
					"name": "Antigravity"
				}
			}
		]
	}`

	err = ProcessWebhookEvent(mockConfig, "Push Hook", []byte(pushPayload))
	if err != nil {
		t.Fatalf("ProcessWebhookEvent (Push) failed: %v", err)
	}

	// Verify DB state
	var tele104 db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", "task-104").First(&tele104).Error; err != nil {
		t.Fatalf("Failed to find task-104 telemetry in DB: %v", err)
	}
	if tele104.Status != "progress" {
		t.Errorf("Expected status 'progress', got %q", tele104.Status)
	}
	if tele104.Assignee != "Antigravity" {
		t.Errorf("Expected assignee 'Antigravity', got %q", tele104.Assignee)
	}
	if !strings.Contains(tele104.LastCommit, "feat(#task-104): optimize log memory usage") {
		t.Errorf("Expected last commit to match push message, got %q", tele104.LastCommit)
	}
	var pushNotif db.Notification
	if err := db.DB.Where("task_id = ? AND type = ?", "task-104", "git_push").First(&pushNotif).Error; err != nil {
		t.Fatalf("Failed to find git push notification: %v", err)
	}
	expectedCommitLink := "https://gitlab.example.com/group/backend-core/commit/1a2b3c4d5e"
	if pushNotif.Link != expectedCommitLink {
		t.Fatalf("Notification link = %q, want %q", pushNotif.Link, expectedCommitLink)
	}

	// Verify Markdown file
	contentBytes, err := os.ReadFile(tmpFilePath)
	if err != nil {
		t.Fatalf("Failed to read task_status.md: %v", err)
	}
	content := string(contentBytes)
	if !strings.Contains(content, "task-104") {
		t.Errorf("task-104 not found in task_status.md")
	}

	board, err := kanban.ParseKanbanBoard(content)
	if err != nil {
		t.Fatalf("Failed to parse board: %v", err)
	}
	foundInProgress := false
	for _, row := range board.Sections["progress"].Rows {
		if row.TaskID == "task-104" {
			foundInProgress = true
			if row.Title != "优化多仓大日志拉取性能与内存开销" {
				t.Errorf("Expected title '优化多仓大日志拉取性能与内存开销', got %q", row.Title)
			}
		}
	}
	if !foundInProgress {
		t.Errorf("task-104 not found in In Progress section of markdown")
	}

	// 4. Test Merge Request Hook (move task-104 to Review, then Done)
	mrReviewPayload := `{
		"object_kind": "merge_request",
		"project": {
			"name": "backend-core"
		},
		"object_attributes": {
			"action": "open",
			"state": "opened",
			"title": "MR for task-104: log optimization",
			"source_branch": "dev/task-104-log-opt",
			"last_commit": {
				"message": "feat(#task-104): final changes"
			}
		},
		"assignees": [
			{
				"name": "Eddie"
			}
		]
	}`

	err = ProcessWebhookEvent(mockConfig, "Merge Request Hook", []byte(mrReviewPayload))
	if err != nil {
		t.Fatalf("ProcessWebhookEvent (MR open) failed: %v", err)
	}

	if err := db.DB.Where("task_id = ?", "task-104").First(&tele104).Error; err != nil {
		t.Fatalf("Failed to find task-104 in DB: %v", err)
	}
	if tele104.Status != "review" {
		t.Errorf("Expected status 'review', got %q", tele104.Status)
	}
	if tele104.Assignee != "Eddie" {
		t.Errorf("Expected assignee 'Eddie' from MR assignees, got %q", tele104.Assignee)
	}

	// Test MR merge (Done)
	mrMergedPayload := `{
		"object_kind": "merge_request",
		"project": {
			"name": "backend-core"
		},
		"object_attributes": {
			"action": "merge",
			"state": "merged",
			"title": "MR for task-104: log optimization",
			"source_branch": "dev/task-104-log-opt",
			"last_commit": {
				"message": "feat(#task-104): final changes"
			}
		},
		"assignees": [
			{
				"name": "Eddie"
			}
		]
	}`

	err = ProcessWebhookEvent(mockConfig, "Merge Request Hook", []byte(mrMergedPayload))
	if err != nil {
		t.Fatalf("ProcessWebhookEvent (MR merge) failed: %v", err)
	}

	if err := db.DB.Where("task_id = ?", "task-104").First(&tele104).Error; err != nil {
		t.Fatalf("Failed to find task-104 in DB: %v", err)
	}
	if tele104.Status != "done" {
		t.Errorf("Expected status 'done', got %q", tele104.Status)
	}
}

func TestProcessWebhookEventPreservesTaskGroupLink(t *testing.T) {
	oldDB := db.DB
	t.Cleanup(func() { db.DB = oldDB })

	var err error
	db.DB, err = gorm.Open(sqlite.Open("file:telemetry_preserve?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}
	if err := db.DB.AutoMigrate(&db.WebhookLog{}, &db.TaskTelemetry{}, &db.GitCommitLog{}, &db.Notification{}); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "telemetry_preserve_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(tmpDir, "task_status.md")
	defer func() { kanban.KanbanFilePath = oldPath }()

	now := time.Now()
	dueDate := now.AddDate(0, 0, 7)
	existing := db.TaskTelemetry{
		TaskID:        "task-777",
		Title:         "Linked shadow task",
		Repo:          "backend-core",
		Assignee:      "Eddie",
		Status:        "backlog",
		IssueType:     "task",
		TaskCreatedAt: now.Add(-1 * time.Hour),
		LastUpdate:    now.Add(-1 * time.Hour),
		DueDate:       &dueDate,
		DecisionLogs:  "existing decision",
		TaskGroupID:   "group-linked-001",
	}
	if err := db.DB.Create(&existing).Error; err != nil {
		t.Fatalf("Failed to seed existing task: %v", err)
	}

	pushPayload := `{
		"object_kind": "push",
		"ref": "refs/heads/dev/task-777-linked-work",
		"user_name": "Eddie",
		"project": {"name": "backend-core"},
		"commits": [
			{
				"id": "abc777",
				"message": "feat(#task-777): continue linked shadow work",
				"author": {"name": "Eddie"}
			}
		]
	}`

	mockConfig := &config.Config{
		Jira: config.JiraConfig{Enabled: false},
	}
	if err := ProcessWebhookEvent(mockConfig, "Push Hook", []byte(pushPayload)); err != nil {
		t.Fatalf("ProcessWebhookEvent failed: %v", err)
	}

	var updated db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", "task-777").First(&updated).Error; err != nil {
		t.Fatalf("Failed to load updated task: %v", err)
	}
	if updated.TaskGroupID != "group-linked-001" {
		t.Fatalf("TaskGroupID = %q, want %q", updated.TaskGroupID, "group-linked-001")
	}
	if updated.DecisionLogs != "existing decision" {
		t.Fatalf("DecisionLogs = %q, want existing decision", updated.DecisionLogs)
	}
	if updated.DueDate == nil || !updated.DueDate.Equal(dueDate) {
		t.Fatalf("DueDate was not preserved: %+v", updated.DueDate)
	}
}
