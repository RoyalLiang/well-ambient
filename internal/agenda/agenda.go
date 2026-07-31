package agenda

import (
	"fmt"
	"strings"
	"time"
	"well-ambient/internal/db"
	"well-ambient/internal/telemetry"
)

// TelemetrySnippet holds brief task freshness status for UI presentation
type TelemetrySnippet struct {
	Branch     string    `json:"branch"`
	LastCommit string    `json:"last_commit"`
	LastUpdate time.Time `json:"last_update"`
}

// AgendaItem represents a task/bug diagnosed with risk/delay in meeting
type AgendaItem struct {
	TaskID           string           `json:"task_id"`
	Title            string           `json:"title"`
	Assignee         string           `json:"assignee"`
	Repo             string           `json:"repo"`
	Status           string           `json:"status"`
	IssueType        string           `json:"issue_type"` // bug, task
	RiskLevel        string           `json:"risk_level"` // critical, warning, safe
	RiskType         string           `json:"risk_type"`  // no_commit_48h, overdue, potential_conflict, none
	Desc             string           `json:"desc"`
	TelemetrySnippet TelemetrySnippet `json:"telemetry_snippet"`
	DueDate          *time.Time       `json:"due_date"`
	DecisionLogs     string           `json:"decision_logs"`
}

// AutoDecision represents an AI autonomous operation that ran in the background
type AutoDecision struct {
	Time       string    `json:"time"`
	OccurredAt time.Time `json:"occurred_at"`
	TaskID     string    `json:"task_id"`
	Message    string    `json:"message"`
	Assignee   string    `json:"assignee,omitempty"`
	Repo       string    `json:"repo,omitempty"`
	Branch     string    `json:"branch,omitempty"`
	CommitID   string    `json:"commit_id,omitempty"`
	CommitURL  string    `json:"commit_url,omitempty"`
}

// EvaluateActiveTasks analyzes active tasks and computes their risk levels
func EvaluateActiveTasks(tasks []db.TaskTelemetry) []AgendaItem {
	var items []AgendaItem
	now := time.Now()

	// Helper map to look up other tasks in the same repo to detect conflict risk
	repoTasks := make(map[string][]db.TaskTelemetry)
	for _, t := range tasks {
		if t.Repo != "" {
			repoTasks[t.Repo] = append(repoTasks[t.Repo], t)
		}
	}

	for _, t := range tasks {
		// We only evaluate active tasks
		if strings.ToLower(t.Status) == "done" {
			continue
		}

		item := AgendaItem{
			TaskID:       t.TaskID,
			Title:        t.Title,
			Assignee:     t.Assignee,
			Repo:         t.Repo,
			Status:       t.Status,
			IssueType:    t.IssueType,
			RiskLevel:    "safe",
			RiskType:     "none",
			DueDate:      t.DueDate,
			DecisionLogs: t.DecisionLogs,
			TelemetrySnippet: TelemetrySnippet{
				Branch:     t.Branch,
				LastCommit: t.LastCommit,
				LastUpdate: t.LastUpdate,
			},
		}

		// 1. Check Overdue (DueDate exceeded) -> critical
		if t.DueDate != nil && now.After(*t.DueDate) {
			item.RiskLevel = "critical"
			item.RiskType = "overdue"
			if t.IssueType == "bug" {
				item.Desc = fmt.Sprintf("故障已超出解决期限 (%s)，影响版本发布，请即刻调停负责人。", t.DueDate.Format("2006-01-02"))
			} else {
				item.Desc = fmt.Sprintf("需求已超过设定的截止日期 (%s)，影响交付节点，请评估缩减范围或追加人员。", t.DueDate.Format("2006-01-02"))
			}
		} else if !t.TaskCreatedAt.IsZero() && now.Sub(t.TaskCreatedAt) > 7*24*time.Hour {
			// 2. Check long running task (>7 days since creation) -> warning
			item.RiskLevel = "warning"
			item.RiskType = "overdue"
			if t.IssueType == "bug" {
				item.Desc = fmt.Sprintf("该故障已开启超过 7 天 (%d天前)，仍未合入，可能有隐含架构阻碍。", int(now.Sub(t.TaskCreatedAt).Hours()/24))
			} else {
				item.Desc = fmt.Sprintf("需求开发已超 7 天 (%d天前)，请对齐交付计划，警惕范围蔓延。", int(now.Sub(t.TaskCreatedAt).Hours()/24))
			}
		}

		// 3. Check No Commit > 48h -> critical
		if item.RiskLevel != "critical" && !t.LastUpdate.IsZero() && now.Sub(t.LastUpdate) > 48*time.Hour {
			if t.Status == "progress" || t.Status == "review" {
				item.RiskLevel = "critical"
				item.RiskType = "no_commit_48h"
				if t.IssueType == "bug" {
					item.Desc = fmt.Sprintf("故障处于修复中，但已连续 %d 小时无有效流转，可能卡在本地环境、复现路径或责任边界。", int(now.Sub(t.LastUpdate).Hours()))
				} else {
					item.Desc = fmt.Sprintf("需求推进中，但已连续 %d 小时无有效流转，疑似遭遇联调阻塞、口径分歧或方案重构。", int(now.Sub(t.LastUpdate).Hours()))
				}
			}
		}

		// 4. Check Potential Merge Conflict Risk -> warning (if currently safe)
		if item.RiskLevel == "safe" && t.Status == "progress" && t.Repo != "" {
			siblings := repoTasks[t.Repo]
			hasConflict := false
			var conflictingAssignee string
			for _, sib := range siblings {
				if sib.TaskID != t.TaskID && sib.Status == "progress" && sib.Branch != "" && sib.Branch != t.Branch {
					hasConflict = true
					conflictingAssignee = sib.Assignee
					break
				}
			}
			if hasConflict {
				item.RiskLevel = "warning"
				item.RiskType = "potential_conflict"
				item.Desc = fmt.Sprintf("与 %s 的并行事项存在协作路径重叠，需提前对齐责任边界与验收顺序。", conflictingAssignee)
			}
		}

		// Return all active tasks so the dashboard can display, sort and filter them globally
		items = append(items, item)
	}

	return items
}

// GenerateAutonomousDecisions simulates AI automatic state transition logs in background (Ambient Sync)
func GenerateAutonomousDecisions(tasks []db.TaskTelemetry) []AutoDecision {
	var logs []AutoDecision
	now := time.Now()

	// Static base mock logs to simulate continuous ambient actions if no real logs yet,
	// mixed with dynamic telemetry states to show a truly living system
	for _, t := range tasks {
		occurredAt := t.LastUpdate
		if occurredAt.IsZero() {
			occurredAt = now
		}
		if strings.ToLower(t.Status) == "done" {
			// Automatically logs completed tasks transition
			logs = append(logs, withLatestCommitReference(AutoDecision{
				Time:       occurredAt.Format("15:04:05"),
				OccurredAt: occurredAt,
				TaskID:     t.TaskID,
				Message:    "🤖 检测到事项已完成验收，AI 已自动将该任务流转至 DONE 并归档。",
				Assignee:   t.Assignee,
			}))
		} else if t.Status == "review" {
			logs = append(logs, withLatestCommitReference(AutoDecision{
				Time:       occurredAt.Format("15:04:05"),
				OccurredAt: occurredAt,
				TaskID:     t.TaskID,
				Message:    fmt.Sprintf("🤖 检测到开发者 %s 发起评审，AI 已自动流转至 Review 并提醒评审人。", t.Assignee),
				Assignee:   t.Assignee,
			}))
		}
	}

	// Fallback/standard automated actions for empty or active lists to bring visual richness
	if len(logs) < 3 {
		jiraSyncAt := now.Add(-12 * time.Minute)
		logs = append(logs, AutoDecision{
			Time:       jiraSyncAt.Format("15:04:05"),
			OccurredAt: jiraSyncAt,
			TaskID:     "JIRA-AUTO",
			Message:    "🤖 [Jira 同步] 定时检测到 2 个新 Bug 关联，已自动生成影子任务。",
		})
		larkReminderAt := now.Add(-45 * time.Minute)
		logs = append(logs, AutoDecision{
			Time:       larkReminderAt.Format("15:04:05"),
			OccurredAt: larkReminderAt,
			TaskID:     "SYSTEM",
			Message:    "🤖 [飞书机器人] 已对 48h 无有效流转的卡点任务自动向负责人推送交互式卡片以了解阻碍原因。",
		})
		statusSyncAt := now.Add(-2 * time.Hour)
		logs = append(logs, AutoDecision{
			Time:       statusSyncAt.Format("15:04:05"),
			OccurredAt: statusSyncAt,
			TaskID:     "LARK-AUTO",
			Message:    "🤖 [状态对齐] 开发者 Eddie 修改了多维表格状态，AI 自动同步更新关联任务。",
		})
	}

	return logs
}

func withLatestCommitReference(decision AutoDecision) AutoDecision {
	commitID, commitURL, hasWeakSemanticEvidence := latestCommitReference(decision.TaskID)
	decision.CommitID = commitID
	decision.CommitURL = commitURL
	if commitID == "" && hasWeakSemanticEvidence {
		decision.Message = weakSemanticEvidenceMessage(decision.Message)
	}
	return decision
}

func latestCommitReference(taskID string) (string, string, bool) {
	taskID = strings.TrimSpace(taskID)
	if db.DB == nil || taskID == "" {
		return "", "", false
	}

	var gitLogs []db.GitCommitLog
	if err := db.DB.
		Where("task_id = ? AND action = ? AND commit_id <> ?", taskID, "git_push", "").
		Order("created_at desc").
		Limit(20).
		Find(&gitLogs).Error; err != nil || len(gitLogs) == 0 {
		return "", "", false
	}

	hasWeakSemanticEvidence := false
	for _, gitLog := range gitLogs {
		if telemetry.IsWeakSemanticCommit(taskID, gitLog) {
			hasWeakSemanticEvidence = true
			continue
		}

		commitID := strings.TrimSpace(gitLog.CommitID)
		if commitID == "" {
			continue
		}

		var notification db.Notification
		if err := db.DB.
			Where("task_id = ? AND type = ? AND link LIKE ?", taskID, "git_push", "%"+commitID+"%").
			Order("created_at desc").
			First(&notification).Error; err == nil {
			return commitID, strings.TrimSpace(notification.Link), hasWeakSemanticEvidence
		}

		_ = db.DB.
			Where("task_id = ? AND type = ? AND link <> ?", taskID, "git_push", "").
			Order("created_at desc").
			First(&notification).Error

		return commitID, strings.TrimSpace(notification.Link), hasWeakSemanticEvidence
	}

	return "", "", hasWeakSemanticEvidence
}

func weakSemanticEvidenceMessage(message string) string {
	if strings.Contains(message, "DONE") || strings.Contains(message, "归档") {
		return "⚠️ 检测到事项为 DONE，但最近 commit 来自弱语义关联，AI 已暂停自动归档并转入人工复核。"
	}
	return "⚠️ 检测到事项状态变化，但最近 commit 归属置信度不足，AI 已暂停自动流转并转入人工复核。"
}
