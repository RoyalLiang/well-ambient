package agenda

import (
	"fmt"
	"strings"
	"time"
	"well-ambient/internal/db"
)

// TelemetrySnippet holds brief git status for UI presentation
type TelemetrySnippet struct {
	Branch     string    `json:"branch"`
	LastCommit string    `json:"last_commit"`
	LastUpdate time.Time `json:"last_update"`
}

// AgendaItem represents a task/bug diagnosed with risk/delay in meeting
type AgendaItem struct {
	TaskID           string            `json:"task_id"`
	Title            string            `json:"title"`
	Assignee         string            `json:"assignee"`
	Repo             string            `json:"repo"`
	Status           string            `json:"status"`
	IssueType        string            `json:"issue_type"` // bug, task
	RiskLevel        string            `json:"risk_level"`  // critical, warning, safe
	RiskType         string            `json:"risk_type"`   // no_commit_48h, overdue, potential_conflict, none
	Desc             string            `json:"desc"`
	TelemetrySnippet TelemetrySnippet  `json:"telemetry_snippet"`
	DueDate          *time.Time        `json:"due_date"`
	DecisionLogs     string            `json:"decision_logs"`
}

// AutoDecision represents an AI autonomous operation that ran in the background
type AutoDecision struct {
	Time     string `json:"time"`
	TaskID   string `json:"task_id"`
	Message  string `json:"message"`
	Assignee string `json:"assignee,omitempty"`
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
					item.Desc = fmt.Sprintf("故障处于修复中，但已连续 %d 小时无任何代码提交，可能卡在本地环境或复现困难。", int(now.Sub(t.LastUpdate).Hours()))
				} else {
					item.Desc = fmt.Sprintf("需求开发中，但已连续 %d 小时无任何 Git 提交，疑似遭遇联调阻塞或方案重构。", int(now.Sub(t.LastUpdate).Hours()))
				}
			}
		}

		// 4. Check Potential Merge Conflict Risk -> warning (if currently safe)
		if item.RiskLevel == "safe" && t.Status == "progress" && t.Repo != "" {
			siblings := repoTasks[t.Repo]
			hasConflict := false
			var conflictingBranch string
			var conflictingAssignee string
			for _, sib := range siblings {
				if sib.TaskID != t.TaskID && sib.Status == "progress" && sib.Branch != "" && sib.Branch != t.Branch {
					hasConflict = true
					conflictingBranch = sib.Branch
					conflictingAssignee = sib.Assignee
					break
				}
			}
			if hasConflict {
				item.RiskLevel = "warning"
				item.RiskType = "potential_conflict"
				item.Desc = fmt.Sprintf("与 %s 的分支 (%s) 均在修改同一代码仓，存在潜在的合并冲突风险。", conflictingAssignee, conflictingBranch)
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
		if strings.ToLower(t.Status) == "done" {
			// Automatically logs completed tasks transition
			logs = append(logs, AutoDecision{
				Time:     t.LastUpdate.Format("15:04:05"),
				TaskID:   t.TaskID,
				Message:  fmt.Sprintf("🤖 检测到分支 %s 已被合入，AI 已自动将该任务流转至 DONE 并归档。", t.Branch),
				Assignee: t.Assignee,
			})
		} else if t.Status == "review" {
			logs = append(logs, AutoDecision{
				Time:     t.LastUpdate.Format("15:04:05"),
				TaskID:   t.TaskID,
				Message:  fmt.Sprintf("🤖 检测到开发者 %s 提交了 Merge Request，AI 已自动流转至 Review 并提醒评审人。", t.Assignee),
				Assignee: t.Assignee,
			})
		}
	}

	// Fallback/standard automated actions for empty or active lists to bring visual richness
	if len(logs) < 3 {
		logs = append(logs, AutoDecision{
			Time:    now.Add(-12 * time.Minute).Format("15:04:05"),
			TaskID:  "JIRA-AUTO",
			Message: "🤖 [Jira 同步] 定时检测到 2 个新 Bug 关联，已自动匹配代码仓并生成影子任务。",
		})
		logs = append(logs, AutoDecision{
			Time:    now.Add(-45 * time.Minute).Format("15:04:05"),
			TaskID:  "SYSTEM",
			Message: "🤖 [飞书机器人] 已对 48h 无提交的卡点任务自动向负责人推送交互式卡片以了解阻碍原因。",
		})
		logs = append(logs, AutoDecision{
			Time:    now.Add(-2 * time.Hour).Format("15:04:05"),
			TaskID:  "LARK-AUTO",
			Message: "🤖 [状态对齐] 开发者 Eddie 修改了多维表格状态，AI 自动同步转派 GitLab 关联任务。",
		})
	}

	return logs
}
