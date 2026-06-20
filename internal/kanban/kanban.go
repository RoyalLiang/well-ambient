package kanban

import (
	"fmt"
	"os"
	"strings"
	"well-ambient/internal/db"
)

// KanbanFilePath specifies the path to the markdown file.
// Can be overridden in tests.
var KanbanFilePath = "task_status.md"

// TaskRow represents a single task row in the Markdown table
type TaskRow struct {
	TaskID     string
	Title      string
	Repo       string
	Assignee   string
	Branch     string
	LastCommit string
}

// Section represents a kanban column/section in markdown
type Section struct {
	Key             string
	HeaderLine      string
	TableHeaderLine string
	SeparatorLine   string
	Rows            []TaskRow
}

// KanbanBoard represents the parsed task_status.md structure
type KanbanBoard struct {
	Preamble     string
	Sections     map[string]*Section
	SectionOrder []string
}

// ParseKanbanBoard parses markdown content into KanbanBoard structure
func ParseKanbanBoard(content string) (*KanbanBoard, error) {
	lines := strings.Split(content, "\n")
	board := &KanbanBoard{
		Sections:     make(map[string]*Section),
		SectionOrder: []string{"backlog", "progress", "review", "done"},
	}

	// Initialize default sections in case they aren't fully parsed/present
	board.Sections["backlog"] = &Section{
		Key:             "backlog",
		HeaderLine:      "## 待办任务 (Backlog)",
		TableHeaderLine: "| 任务ID | 任务标题 | 代码仓库 | 指派人 | 分支名称 | 最近提交 |",
		SeparatorLine:   "| :--- | :--- | :--- | :--- | :--- | :--- |",
	}
	board.Sections["progress"] = &Section{
		Key:             "progress",
		HeaderLine:      "## 进行中 (In Progress)",
		TableHeaderLine: "| 任务ID | 任务标题 | 代码仓库 | 指派人 | 分支名称 | 最近提交 |",
		SeparatorLine:   "| :--- | :--- | :--- | :--- | :--- | :--- |",
	}
	board.Sections["review"] = &Section{
		Key:             "review",
		HeaderLine:      "## 代码评审 (In Review)",
		TableHeaderLine: "| 任务ID | 任务标题 | 代码仓库 | 指派人 | 分支名称 | 最近提交 |",
		SeparatorLine:   "| :--- | :--- | :--- | :--- | :--- | :--- |",
	}
	board.Sections["done"] = &Section{
		Key:             "done",
		HeaderLine:      "## 已完成 (Done)",
		TableHeaderLine: "| 任务ID | 任务标题 | 代码仓库 | 指派人 | 分支名称 | 最近提交 |",
		SeparatorLine:   "| :--- | :--- | :--- | :--- | :--- | :--- |",
	}

	var preambleLines []string
	var currentSection *Section

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			key := getStatusKey(trimmed)
			if key != "" {
				currentSection = board.Sections[key]
				currentSection.HeaderLine = line
				continue
			} else {
				currentSection = nil
			}
		}

		if currentSection == nil {
			preambleLines = append(preambleLines, line)
			continue
		}

		// Inside a section, parse table rows
		if strings.HasPrefix(trimmed, "|") {
			parts := strings.Split(line, "|")
			if len(parts) >= 8 {
				col1 := strings.TrimSpace(parts[1])
				if col1 == "任务ID" {
					currentSection.TableHeaderLine = line
				} else if strings.HasPrefix(col1, ":-") || strings.HasPrefix(col1, "-") {
					currentSection.SeparatorLine = line
				} else if col1 != "" {
					row := TaskRow{
						TaskID:     col1,
						Title:      strings.TrimSpace(parts[2]),
						Repo:       strings.TrimSpace(parts[3]),
						Assignee:   strings.TrimSpace(parts[4]),
						Branch:     strings.TrimSpace(parts[5]),
						LastCommit: strings.TrimSpace(parts[6]),
					}
					currentSection.Rows = append(currentSection.Rows, row)
				}
			}
		}
	}

	// Join preamble lines, trim trailing newlines
	board.Preamble = strings.TrimRight(strings.Join(preambleLines, "\n"), "\n")
	return board, nil
}

// getStatusKey normalizes markdown section header to status keys
func getStatusKey(header string) string {
	h := strings.ToLower(header)
	if strings.Contains(h, "backlog") {
		return "backlog"
	}
	if strings.Contains(h, "progress") {
		return "progress"
	}
	if strings.Contains(h, "review") {
		return "review"
	}
	if strings.Contains(h, "done") {
		return "done"
	}
	return ""
}

// RemoveTask removes a task row from all sections
func (b *KanbanBoard) RemoveTask(taskID string) {
	for _, sec := range b.Sections {
		var newRows []TaskRow
		for _, row := range sec.Rows {
			if row.TaskID != taskID {
				newRows = append(newRows, row)
			}
		}
		sec.Rows = newRows
	}
}

// AddTask appends a task row to the given section
func (b *KanbanBoard) AddTask(statusKey string, row TaskRow) {
	if sec, ok := b.Sections[statusKey]; ok {
		sec.Rows = append(sec.Rows, row)
	}
}

// String reconstructs the complete markdown content
func (b *KanbanBoard) String() string {
	var sb strings.Builder
	if b.Preamble != "" {
		sb.WriteString(b.Preamble)
		sb.WriteString("\n\n")
	}
	for i, key := range b.SectionOrder {
		sec := b.Sections[key]
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(sec.HeaderLine + "\n")
		sb.WriteString(sec.TableHeaderLine + "\n")
		sb.WriteString(sec.SeparatorLine + "\n")
		for _, row := range sec.Rows {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
				row.TaskID, row.Title, row.Repo, row.Assignee, row.Branch, row.LastCommit))
		}
	}
	return sb.String()
}

// SyncTaskToKanban synchronizes telemetry updates to the markdown kanban board file
func SyncTaskToKanban(telemetry *db.TaskTelemetry) error {
	content, err := os.ReadFile(KanbanFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// If file doesn't exist, we will create it with empty board
			content = []byte("")
		} else {
			return err
		}
	}

	board, err := ParseKanbanBoard(string(content))
	if err != nil {
		return err
	}

	// Remove task row from all sections first
	board.RemoveTask(telemetry.TaskID)

	statusKey := strings.ToLower(telemetry.Status)
	if statusKey == "backlog" || statusKey == "progress" || statusKey == "review" || statusKey == "done" {
		// Clean and default fields
		assignee := telemetry.Assignee
		if assignee == "" {
			if statusKey == "backlog" {
				assignee = "未指派"
			} else {
				assignee = "-"
			}
		}
		branch := telemetry.Branch
		if branch == "" {
			branch = "-"
		}
		lastCommit := telemetry.LastCommit
		if lastCommit == "" {
			lastCommit = "-"
		} else {
			lastCommit = strings.ReplaceAll(lastCommit, "\n", " ")
			lastCommit = strings.ReplaceAll(lastCommit, "\r", "")
		}
		repo := telemetry.Repo
		if repo == "" {
			repo = "-"
		}
		title := telemetry.Title
		if title == "" {
			title = "-"
		} else {
			title = strings.ReplaceAll(title, "\n", " ")
			title = strings.ReplaceAll(title, "\r", "")
		}

		newRow := TaskRow{
			TaskID:     telemetry.TaskID,
			Title:      title,
			Repo:       repo,
			Assignee:   assignee,
			Branch:     branch,
			LastCommit: lastCommit,
		}
		board.AddTask(statusKey, newRow)
	}

	newContent := board.String()
	return os.WriteFile(KanbanFilePath, []byte(newContent), 0644)
}
