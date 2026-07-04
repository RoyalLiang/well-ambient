package telemetry

import (
	"fmt"
	"log"
	"strings"
	"time"
	"well-ambient/internal/db"
)

const (
	SemanticLinkerType       = "semantic_linker"
	SemanticLinkReviewType   = "semantic_link_review"
	semanticReviewWindow     = 10 * time.Minute
	semanticAcceptMinSignals = 2
)

// HasExplicitTaskReference reports whether the branch or message carries the task ID directly.
func HasExplicitTaskReference(taskID string, values ...string) bool {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return false
	}
	for _, value := range values {
		if strings.EqualFold(ExtractTaskID(value), taskID) {
			return true
		}
	}
	return false
}

// IsWeakSemanticCommit reports evidence that was attached by semantic guessing instead of explicit task references.
func IsWeakSemanticCommit(taskID string, log db.GitCommitLog) bool {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || log.Action != "git_push" {
		return false
	}
	if HasExplicitTaskReference(taskID, log.Branch, log.Message) {
		return false
	}
	return hasNearbySemanticLink(taskID, log.CreatedAt)
}

func hasNearbySemanticLink(taskID string, createdAt time.Time) bool {
	if db.DB == nil || strings.TrimSpace(taskID) == "" {
		return false
	}
	query := db.DB.Model(&db.Notification{}).Where("task_id = ? AND type = ?", taskID, SemanticLinkerType)
	if !createdAt.IsZero() {
		query = query.Where("created_at BETWEEN ? AND ?", createdAt.Add(-semanticReviewWindow), createdAt.Add(semanticReviewWindow))
	}
	var count int64
	return query.Count(&count).Error == nil && count > 0
}

func semanticLinkTrustReason(taskID, branchName, repoName, assigneeName string) (bool, string) {
	if db.DB == nil {
		return false, "数据库未初始化，无法校验证据归属"
	}

	var task db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", taskID).First(&task).Error; err != nil {
		return false, "候选任务不存在，无法自动绑定"
	}

	assigneeMatch := sameEvidenceToken(task.Assignee, assigneeName)
	repoMatch := compatibleEvidenceToken(task.Repo, repoName)
	branchMatch := compatibleEvidenceToken(task.Branch, branchName)

	signals := 0
	if assigneeMatch {
		signals++
	}
	if repoMatch {
		signals++
	}
	if branchMatch {
		signals++
	}

	if signals >= semanticAcceptMinSignals && (assigneeMatch || branchMatch) {
		return true, "负责人/仓库/分支证据满足自动绑定阈值"
	}

	return false, fmt.Sprintf(
		"语义候选缺少强证据: assignee_match=%t repo_match=%t branch_match=%t",
		assigneeMatch,
		repoMatch,
		branchMatch,
	)
}

func createSemanticReviewNotification(taskID, branchName, lastCommit, repoName, assigneeName, reason string) {
	if db.DB == nil || strings.TrimSpace(taskID) == "" {
		return
	}

	notif := db.Notification{
		Type:      SemanticLinkReviewType,
		TaskID:    strings.TrimSpace(taskID),
		Title:     "⚠️ AI 语义关联待人工复核",
		Message:   fmt.Sprintf("AI 候选关联未自动落库: repo=%s, branch=%s, assignee=%s, reason=%s, commit=%s", repoName, branchName, assigneeName, reason, firstCommitLine(lastCommit)),
		Assignee:  assigneeName,
		Link:      "",
		CreatedAt: time.Now(),
	}
	if err := db.DB.Create(&notif).Error; err != nil {
		log.Printf("Semantic Linker: failed to save review notification: %v", err)
		return
	}
	if OnNotificationBroadcast != nil {
		OnNotificationBroadcast()
	}
}

func sameEvidenceToken(left, right string) bool {
	left = normalizedEvidenceToken(left)
	right = normalizedEvidenceToken(right)
	return left != "" && right != "" && left == right
}

func compatibleEvidenceToken(left, right string) bool {
	left = normalizedEvidenceToken(left)
	right = normalizedEvidenceToken(right)
	if left == "" || right == "" || left == "-" || right == "-" {
		return false
	}
	if left == right {
		return true
	}
	return len(left) >= 4 && len(right) >= 4 && (strings.Contains(left, right) || strings.Contains(right, left))
}

func normalizedEvidenceToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func firstCommitLine(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "-"
	}
	line := strings.TrimSpace(strings.Split(message, "\n")[0])
	if len(line) > 80 {
		return line[:80]
	}
	return line
}
