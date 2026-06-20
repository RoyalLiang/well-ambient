package kanban

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"well-ambient/internal/db"
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
| task-102 | Svelte 嵌入式大屏看板组件与样式设计 | frontend-dashboard | Antigravity | dev/task-102-svelte-ui | feat(#task-102): add status and kanban dashboard |

## 代码评审 (In Review)
| 任务ID | 任务标题 | 代码仓库 | 指派人 | 分支名称 | 最近提交 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| task-100 | GitLab 多仓 Webhook 事件解析中枢 | backend-core | Eddie | dev/task-100-webhook-router | feat(#task-100): parse merge request hook body |

## 已完成 (Done)
| 任务ID | 任务标题 | 代码仓库 | 指派人 | 分支名称 | 最近提交 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| task-099 | 项目初始化及基础配置文件生成 | well-ambient | Antigravity | dev/task-099-init | feat(#task-099): initialize folder structure & config |
`

func TestParseAndStringify(t *testing.T) {
	board, err := ParseKanbanBoard(sampleMarkdown)
	if err != nil {
		t.Fatalf("Failed to parse sample markdown: %v", err)
	}

	// Verify preamble
	expectedPreamble := "# well-ambient 任务看板\n\n以下任务看板由 well-ambient 服务根据 GitLab 提交流程自动流转。"
	if board.Preamble != expectedPreamble {
		t.Errorf("Expected preamble:\n%q\nGot:\n%q", expectedPreamble, board.Preamble)
	}

	// Verify sections
	if len(board.Sections["backlog"].Rows) != 1 {
		t.Errorf("Expected 1 row in backlog, got %d", len(board.Sections["backlog"].Rows))
	}
	if len(board.Sections["progress"].Rows) != 2 {
		t.Errorf("Expected 2 rows in progress, got %d", len(board.Sections["progress"].Rows))
	}

	// Check fields of one row
	row := board.Sections["progress"].Rows[0]
	if row.TaskID != "task-101" || row.Title != "飞书多维表格（Bitable）双向同步对接 API" || row.Assignee != "Eddie" {
		t.Errorf("Unexpected task-101 details: %+v", row)
	}

	// Stringify and match with original
	output := board.String()
	// Normalize line endings and trim spaces for comparison if needed, or check exact equality
	if strings.TrimSpace(output) != strings.TrimSpace(sampleMarkdown) {
		t.Errorf("Stringify output does not match sample markdown.\nGot:\n%s\nExpected:\n%s", output, sampleMarkdown)
	}
}

func TestSyncTaskToKanban(t *testing.T) {
	// Setup temporary file
	tmpDir, err := os.MkdirTemp("", "kanban_test")
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
	oldPath := KanbanFilePath
	KanbanFilePath = tmpFilePath
	defer func() { KanbanFilePath = oldPath }()

	// Update existing task-101: move it to "review"
	telemetry := &db.TaskTelemetry{
		TaskID:     "task-101",
		Title:      "飞书多维表格（Bitable）双向同步对接 API (Updated)",
		Repo:       "backend-core",
		Assignee:   "Eddie",
		Branch:     "dev/task-101-feishu-sync",
		LastCommit: "feat(#task-101): update API call",
		Status:     "review",
	}

	err = SyncTaskToKanban(telemetry)
	if err != nil {
		t.Fatalf("SyncTaskToKanban failed: %v", err)
	}

	// Read updated content and verify
	updatedContent, err := os.ReadFile(tmpFilePath)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	board, err := ParseKanbanBoard(string(updatedContent))
	if err != nil {
		t.Fatalf("Failed to parse updated board: %v", err)
	}

	// task-101 should not be in progress anymore
	for _, row := range board.Sections["progress"].Rows {
		if row.TaskID == "task-101" {
			t.Errorf("task-101 still found in progress section")
		}
	}

	// task-101 should be in review
	foundInReview := false
	var reviewRow TaskRow
	for _, row := range board.Sections["review"].Rows {
		if row.TaskID == "task-101" {
			foundInReview = true
			reviewRow = row
			break
		}
	}
	if !foundInReview {
		t.Errorf("task-101 not found in review section")
	} else {
		if reviewRow.Title != "飞书多维表格（Bitable）双向同步对接 API (Updated)" {
			t.Errorf("Expected title updated, got %q", reviewRow.Title)
		}
		if reviewRow.LastCommit != "feat(#task-101): update API call" {
			t.Errorf("Expected last commit updated, got %q", reviewRow.LastCommit)
		}
	}

	// Now try adding a brand new task to progress
	newTask := &db.TaskTelemetry{
		TaskID:     "task-105",
		Title:      "New Feature Task",
		Repo:       "frontend-dashboard",
		Assignee:   "Antigravity",
		Branch:     "dev/task-105-new-feature",
		LastCommit: "init task-105",
		Status:     "progress",
	}

	err = SyncTaskToKanban(newTask)
	if err != nil {
		t.Fatalf("SyncTaskToKanban for new task failed: %v", err)
	}

	// Verify new task
	updatedContent, _ = os.ReadFile(tmpFilePath)
	board, _ = ParseKanbanBoard(string(updatedContent))

	foundNewTask := false
	for _, row := range board.Sections["progress"].Rows {
		if row.TaskID == "task-105" {
			foundNewTask = true
			if row.Title != "New Feature Task" || row.Assignee != "Antigravity" {
				t.Errorf("New task-105 has incorrect details: %+v", row)
			}
		}
	}
	if !foundNewTask {
		t.Errorf("newTask task-105 not found in progress section")
	}
}
