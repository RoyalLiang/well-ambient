package deliveryplanning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"well-ambient/internal/db"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReconcileExternalIssue imports Jira-owned project and version facts without
// overwriting a human-confirmed primary target release.
func (s *Service) ReconcileExternalIssue(ctx context.Context, state ExternalIssueVersionState) (WorkItemSnapshot, error) {
	if s == nil || s.repository == nil || s.repository.conn == nil {
		return WorkItemSnapshot{}, fmt.Errorf("database is not initialized")
	}
	state.IssueKey = strings.TrimSpace(state.IssueKey)
	state.ProjectKey = NormalizeProjectKey(state.ProjectKey)
	kind, err := NormalizeIssueType(state.Kind)
	if err != nil {
		return WorkItemSnapshot{}, err
	}
	if kind == ExecutionTask {
		return WorkItemSnapshot{}, &DomainError{
			Code:       "external_execution_task_not_supported",
			Message:    "Jira version reconciliation only accepts work items",
			StatusCode: 422,
		}
	}
	if state.IssueKey == "" || state.ProjectKey == "" {
		return WorkItemSnapshot{}, fmt.Errorf("external issue requires issue key and project key")
	}

	var result WorkItemSnapshot
	err = s.repository.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task db.TaskTelemetry
		if err := tx.Where("task_id = ?", state.IssueKey).First(&task).Error; err != nil {
			return err
		}
		before, err := loadWorkItemSnapshot(tx, task)
		if err != nil {
			return err
		}
		conflictCode := ""
		if strings.TrimSpace(task.ProjectKey) == "" {
			task.ProjectKey = state.ProjectKey
		} else if NormalizeProjectKey(task.ProjectKey) != state.ProjectKey {
			conflictCode = "external_project_conflict"
		}
		if strings.TrimSpace(task.Source) == "" {
			task.Source = "jira"
		}
		if strings.TrimSpace(task.ExternalKey) == "" {
			task.ExternalKey = state.IssueKey
		}

		allExternal := append([]ExternalRelease{}, state.TargetReleases...)
		allExternal = append(allExternal, state.AffectedReleases...)
		releasesByIdentity, err := upsertExternalReleaseFacts(ctx, tx, state.ProjectKey, allExternal, s.now())
		if err != nil {
			return err
		}
		targetIDs, err := externalReleaseIDs(state.TargetReleases, releasesByIdentity)
		if err != nil {
			return err
		}
		affectedIDs, err := externalReleaseIDs(state.AffectedReleases, releasesByIdentity)
		if err != nil {
			return err
		}
		if kind != WorkItemBug {
			affectedIDs = nil
		}

		manualPrimaryID := confirmedPrimaryTarget(before.Links)
		manualPrimaryStillExternal := containsUint(targetIDs, manualPrimaryID)
		if err := reconcileExternalLinks(tx, task.TaskID, ReleaseTargetFix, targetIDs); err != nil {
			return err
		}
		if err := reconcileExternalLinks(tx, task.TaskID, ReleaseAffected, affectedIDs); err != nil {
			return err
		}

		primaryID := uint(0)
		switch {
		case manualPrimaryID > 0:
			primaryID = manualPrimaryID
			if !manualPrimaryStillExternal {
				conflictCode = "external_primary_release_drift"
			}
		case len(targetIDs) == 1:
			primaryID = targetIDs[0]
		case len(targetIDs) > 1:
			conflictCode = "ambiguous_target_release"
		}
		if err := setReconciledPrimary(tx, task.TaskID, primaryID); err != nil {
			return err
		}

		if task.PlanningState == "" {
			task.PlanningState = PlanningReady
		}
		task.ProjectKey = NormalizeProjectKey(task.ProjectKey)
		task.Source = "jira"
		task.ExternalKey = state.IssueKey
		task.IssueType = kind
		afterTask := task
		if err := tx.Model(&db.TaskTelemetry{}).Where("task_id = ?", task.TaskID).Updates(map[string]any{
			"project_key":    task.ProjectKey,
			"source":         task.Source,
			"external_key":   task.ExternalKey,
			"issue_type":     task.IssueType,
			"planning_state": task.PlanningState,
		}).Error; err != nil {
			return err
		}
		after, err := loadWorkItemSnapshot(tx, afterTask)
		if err != nil {
			return err
		}
		changed := workItemFactsSignature(before) != workItemFactsSignature(after)
		if changed {
			nextRevision := task.Revision + 1
			if err := tx.Model(&db.TaskTelemetry{}).Where("task_id = ?", task.TaskID).Update("revision", nextRevision).Error; err != nil {
				return err
			}
			after.WorkItem.Revision = nextRevision
			task.Revision = nextRevision
			beforeJSON, _ := json.Marshal(before)
			afterJSON, _ := json.Marshal(after)
			eventType := "external_versions_reconciled"
			if conflictCode != "" {
				eventType = "release_scope_conflict_detected"
			}
			event := db.WorkItemEvent{
				WorkItemID: task.TaskID,
				ProjectKey: task.ProjectKey,
				EventType:  eventType,
				Actor:      "jira_sync",
				Reason:     firstNonBlankValue(conflictCode, "Jira version facts reconciled"),
				BeforeJSON: string(beforeJSON),
				AfterJSON:  string(afterJSON),
				Revision:   nextRevision,
				Source:     "jira",
				SyncState:  syncStateForConflict(conflictCode),
				CreatedAt:  s.now(),
			}
			if err := tx.Create(&event).Error; err != nil {
				return err
			}
			if _, err := appendWorkItemAsset(ctx, tx, event); err != nil {
				return err
			}
		} else if conflictCode != "" {
			if err := appendConflictOnce(ctx, tx, task, conflictCode, after, s.now()); err != nil {
				return err
			}
		}
		result = after
		return nil
	})
	return result, err
}

func upsertExternalReleaseFacts(ctx context.Context, conn *gorm.DB, projectKey string, releases []ExternalRelease, syncedAt time.Time) (map[string]db.ReleaseVersion, error) {
	byIdentity := make(map[string]db.ReleaseVersion)
	for _, external := range releases {
		external.ProjectKey = projectKey
		externalID := strings.TrimSpace(external.ExternalID)
		if externalID == "" {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(external.Status))
		if status == "" {
			status = ReleasePlanned
		}
		var before db.ReleaseVersion
		beforeErr := conn.Where("project_key = ? AND source = ? AND external_id = ?", projectKey, "jira", externalID).
			First(&before).Error
		if beforeErr != nil && !errors.Is(beforeErr, gorm.ErrRecordNotFound) {
			return nil, beforeErr
		}
		row := db.ReleaseVersion{
			ProjectKey:  projectKey,
			Source:      "jira",
			ExternalID:  externalID,
			Name:        firstNonBlankValue(strings.TrimSpace(external.Name), externalID),
			Description: external.Description,
			Status:      status,
			StartDate:   external.StartDate,
			ReleaseDate: external.ReleaseDate,
			SourceURL:   external.SourceURL,
			SyncedAt:    &syncedAt,
		}
		if err := conn.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "project_key"}, {Name: "source"}, {Name: "external_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"name", "description", "status", "start_date", "release_date", "source_url", "synced_at", "updated_at",
			}),
		}).Create(&row).Error; err != nil {
			return nil, err
		}
		if err := conn.Where("project_key = ? AND source = ? AND external_id = ?", projectKey, "jira", externalID).
			First(&row).Error; err != nil {
			return nil, err
		}
		var beforeRow *db.ReleaseVersion
		if beforeErr == nil {
			beforeRow = &before
		}
		if _, err := appendReleaseVersionAsset(ctx, conn, beforeRow, row, syncedAt); err != nil {
			return nil, err
		}
		byIdentity[externalID] = row
	}
	return byIdentity, nil
}

func externalReleaseIDs(external []ExternalRelease, releases map[string]db.ReleaseVersion) ([]uint, error) {
	ids := make([]uint, 0, len(external))
	for _, release := range external {
		externalID := strings.TrimSpace(release.ExternalID)
		row, ok := releases[externalID]
		if !ok {
			return nil, fmt.Errorf("release %q was not persisted", externalID)
		}
		ids = append(ids, row.ID)
	}
	return uniqueUint(ids), nil
}

func reconcileExternalLinks(conn *gorm.DB, workItemID, relation string, releaseIDs []uint) error {
	var current []db.WorkItemReleaseLink
	if err := conn.Where("work_item_id = ? AND relation = ? AND source = ?", workItemID, relation, "jira").
		Find(&current).Error; err != nil {
		return err
	}
	incoming := make(map[uint]struct{}, len(releaseIDs))
	for _, releaseID := range releaseIDs {
		incoming[releaseID] = struct{}{}
	}
	for _, link := range current {
		_, keep := incoming[link.ReleaseVersionID]
		if !keep && relation == ReleaseTargetFix && link.IsPrimary && strings.TrimSpace(link.ConfirmedBy) != "" {
			keep = true
		}
		if link.Active != keep {
			if err := conn.Model(&link).Update("active", keep).Error; err != nil {
				return err
			}
		}
	}
	for _, releaseID := range releaseIDs {
		var link db.WorkItemReleaseLink
		err := conn.Where(
			"work_item_id = ? AND release_version_id = ? AND relation = ?",
			workItemID, releaseID, relation,
		).First(&link).Error
		switch {
		case err == nil:
			if err := conn.Model(&link).Update("active", true).Error; err != nil {
				return err
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := conn.Create(&db.WorkItemReleaseLink{
				WorkItemID:       workItemID,
				ReleaseVersionID: releaseID,
				Relation:         relation,
				Active:           true,
				Source:           "jira",
			}).Error; err != nil {
				return err
			}
		default:
			return err
		}
	}
	return nil
}

func setReconciledPrimary(conn *gorm.DB, workItemID string, primaryID uint) error {
	var targetLinks []db.WorkItemReleaseLink
	if err := conn.Where("work_item_id = ? AND relation = ?", workItemID, ReleaseTargetFix).Find(&targetLinks).Error; err != nil {
		return err
	}
	for _, link := range targetLinks {
		desired := primaryID > 0 && link.ReleaseVersionID == primaryID
		if link.IsPrimary != desired {
			if err := conn.Model(&link).Update("is_primary", desired).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func confirmedPrimaryTarget(links []db.WorkItemReleaseLink) uint {
	for _, link := range links {
		if link.Active && link.Relation == ReleaseTargetFix && link.IsPrimary && strings.TrimSpace(link.ConfirmedBy) != "" {
			return link.ReleaseVersionID
		}
	}
	return 0
}

func workItemFactsSignature(snapshot WorkItemSnapshot) string {
	type linkFact struct {
		ReleaseID uint
		Relation  string
		Primary   bool
		Active    bool
		Confirmed bool
	}
	links := make([]linkFact, 0, len(snapshot.Links))
	for _, link := range snapshot.Links {
		links = append(links, linkFact{
			ReleaseID: link.ReleaseVersionID,
			Relation:  link.Relation,
			Primary:   link.IsPrimary,
			Active:    link.Active,
			Confirmed: strings.TrimSpace(link.ConfirmedBy) != "",
		})
	}
	sort.Slice(links, func(i, j int) bool {
		if links[i].Relation != links[j].Relation {
			return links[i].Relation < links[j].Relation
		}
		return links[i].ReleaseID < links[j].ReleaseID
	})
	payload, _ := json.Marshal(map[string]any{
		"project_key":    snapshot.WorkItem.ProjectKey,
		"source":         snapshot.WorkItem.Source,
		"external_key":   snapshot.WorkItem.ExternalKey,
		"kind":           snapshot.WorkItem.IssueType,
		"planning_state": snapshot.WorkItem.PlanningState,
		"links":          links,
	})
	return string(payload)
}

func appendConflictOnce(ctx context.Context, conn *gorm.DB, task db.TaskTelemetry, code string, snapshot WorkItemSnapshot, occurredAt time.Time) error {
	var count int64
	if err := conn.Model(&db.WorkItemEvent{}).
		Where("work_item_id = ? AND event_type = ? AND reason = ? AND revision = ?",
			task.TaskID, "release_scope_conflict_detected", code, task.Revision).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	afterJSON, _ := json.Marshal(snapshot)
	event := db.WorkItemEvent{
		WorkItemID: task.TaskID,
		ProjectKey: task.ProjectKey,
		EventType:  "release_scope_conflict_detected",
		Actor:      "jira_sync",
		Reason:     code,
		AfterJSON:  string(afterJSON),
		Revision:   task.Revision,
		Source:     "jira",
		SyncState:  "conflict",
		CreatedAt:  occurredAt,
	}
	if err := conn.Create(&event).Error; err != nil {
		return err
	}
	_, err := appendWorkItemAsset(ctx, conn, event)
	return err
}

func containsUint(values []uint, wanted uint) bool {
	if wanted == 0 {
		return false
	}
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func syncStateForConflict(code string) string {
	if code == "" {
		return "synchronized"
	}
	return "conflict"
}

func firstNonBlankValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
