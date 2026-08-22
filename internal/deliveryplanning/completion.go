package deliveryplanning

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"well-ambient/internal/db"

	"gorm.io/gorm"
)

const WorkItemExecutionCompleted = "execution_completed"

type CompletionCommand struct {
	WorkItemID       string
	ExpectedRevision uint
	Actor            string
	Reason           string
	Source           string
}

type CompletionEvidence struct {
	CommitCount   int        `json:"commit_count"`
	MergedMRCount int        `json:"merged_mr_count"`
	Repositories  []string   `json:"repositories"`
	LatestAt      *time.Time `json:"latest_at,omitempty"`
}

type CompletionResult struct {
	Snapshot WorkItemSnapshot   `json:"snapshot"`
	Evidence CompletionEvidence `json:"evidence"`
}

// CompleteWorkItem confirms execution completion from already captured Git
// evidence. Release commitment is intentionally not part of this transition.
func (s *Service) CompleteWorkItem(ctx context.Context, command CompletionCommand) (CompletionResult, error) {
	if s == nil || s.repository == nil || s.repository.conn == nil {
		return CompletionResult{}, fmt.Errorf("database is not initialized")
	}
	command.WorkItemID = strings.TrimSpace(command.WorkItemID)
	command.Actor = strings.TrimSpace(command.Actor)
	command.Reason = strings.TrimSpace(command.Reason)
	command.Source = strings.ToLower(strings.TrimSpace(command.Source))
	if command.Source == "" {
		command.Source = "manual_evidence"
	}
	if command.WorkItemID == "" {
		return CompletionResult{}, &DomainError{Code: "work_item_required", Message: "work item id is required", StatusCode: 422}
	}
	if command.Actor == "" {
		return CompletionResult{}, &DomainError{Code: "actor_required", Message: "actor is required", StatusCode: 422}
	}
	if command.Reason == "" {
		return CompletionResult{}, &DomainError{Code: "reason_required", Message: "a reason is required to confirm completion", StatusCode: 422}
	}

	var result CompletionResult
	err := s.repository.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current db.TaskTelemetry
		if err := tx.Where("task_id = ?", command.WorkItemID).First(&current).Error; err != nil {
			return err
		}
		kind, err := NormalizeIssueType(current.IssueType)
		if err != nil {
			return err
		}
		if kind == ExecutionTask {
			return &DomainError{
				Code:       "execution_task_completion_inherited",
				Message:    "execution tasks report evidence to their parent work item",
				StatusCode: 422,
			}
		}

		evidence, err := loadCompletionEvidence(tx, current)
		if err != nil {
			return err
		}
		if strings.EqualFold(strings.TrimSpace(current.Status), "done") && current.CompletedAt != nil {
			snapshot, err := loadWorkItemSnapshot(tx, current)
			if err != nil {
				return err
			}
			result = CompletionResult{Snapshot: snapshot, Evidence: evidence}
			return nil
		}
		if current.Revision != command.ExpectedRevision {
			return &DomainError{
				Code:       "revision_conflict",
				Message:    fmt.Sprintf("work item revision changed from %d to %d", command.ExpectedRevision, current.Revision),
				StatusCode: 409,
			}
		}
		if evidence.CommitCount == 0 && evidence.MergedMRCount == 0 {
			return &DomainError{
				Code:       "completion_evidence_required",
				Message:    "an exact commit or merged merge request is required before manual completion",
				StatusCode: 409,
			}
		}
		if err := ValidatePlanningGate(kind, current.ProjectKey, PlanningDone, 0); err != nil {
			return err
		}

		before, err := loadWorkItemSnapshot(tx, current)
		if err != nil {
			return err
		}
		completedAt := s.now()
		nextRevision := current.Revision + 1
		update := tx.Model(&db.TaskTelemetry{}).
			Where("task_id = ? AND revision = ?", current.TaskID, command.ExpectedRevision).
			Updates(map[string]any{
				"status":         "done",
				"planning_state": PlanningDone,
				"completed_at":   &completedAt,
				"last_update":    completedAt,
				"revision":       nextRevision,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return &DomainError{
				Code:       "revision_conflict",
				Message:    "work item revision changed while completion was being applied",
				StatusCode: 409,
			}
		}

		var next db.TaskTelemetry
		if err := tx.Where("task_id = ?", current.TaskID).First(&next).Error; err != nil {
			return err
		}
		after, err := loadWorkItemSnapshot(tx, next)
		if err != nil {
			return err
		}
		beforeJSON, err := json.Marshal(before)
		if err != nil {
			return err
		}
		afterJSON, err := json.Marshal(after)
		if err != nil {
			return err
		}
		event := db.WorkItemEvent{
			WorkItemID: current.TaskID,
			ProjectKey: next.ProjectKey,
			EventType:  WorkItemExecutionCompleted,
			Actor:      command.Actor,
			Reason:     command.Reason,
			BeforeJSON: string(beforeJSON),
			AfterJSON:  string(afterJSON),
			Revision:   nextRevision,
			Source:     command.Source,
			SyncState:  "not_required",
			CreatedAt:  completedAt,
		}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		if _, err := appendWorkItemAsset(ctx, tx, event); err != nil {
			return err
		}
		result = CompletionResult{Snapshot: after, Evidence: evidence}
		return nil
	})
	if err != nil {
		return CompletionResult{}, err
	}
	return result, nil
}

func loadCompletionEvidence(conn *gorm.DB, task db.TaskTelemetry) (CompletionEvidence, error) {
	variants := workItemIDVariants(task.TaskID, task.ExternalKey)
	var logs []db.GitCommitLog
	if err := conn.
		Where("task_id IN ?", variants).
		Where("(LOWER(action) = ? AND TRIM(commit_id) <> '') OR LOWER(action) = ?", "git_push", "mr_merge").
		Order("created_at DESC, id DESC").
		Find(&logs).Error; err != nil {
		return CompletionEvidence{}, err
	}

	repositories := make(map[string]struct{})
	evidence := CompletionEvidence{}
	for index, entry := range logs {
		switch strings.ToLower(strings.TrimSpace(entry.Action)) {
		case "git_push":
			evidence.CommitCount++
		case "mr_merge":
			evidence.MergedMRCount++
		}
		if repo := strings.TrimSpace(entry.Repo); repo != "" {
			repositories[repo] = struct{}{}
		}
		if index == 0 {
			latest := entry.CreatedAt
			evidence.LatestAt = &latest
		}
	}
	for repo := range repositories {
		evidence.Repositories = append(evidence.Repositories, repo)
	}
	sort.Strings(evidence.Repositories)
	return evidence, nil
}

func workItemIDVariants(values ...string) []string {
	seen := make(map[string]struct{}, len(values)*3)
	variants := make([]string, 0, len(values)*3)
	for _, value := range values {
		value = strings.TrimSpace(value)
		for _, variant := range []string{value, strings.ToUpper(value), strings.ToLower(value)} {
			if variant == "" {
				continue
			}
			if _, exists := seen[variant]; exists {
				continue
			}
			seen[variant] = struct{}{}
			variants = append(variants, variant)
		}
	}
	return variants
}
