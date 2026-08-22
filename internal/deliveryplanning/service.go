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

type Service struct {
	repository *Repository
	now        func() time.Time
}

const workItemDonePredicate = `LOWER(TRIM(status)) IN ('done','closed','resolved','completed','archived','已完成','已关闭')`
const workItemActivePredicate = `(status IS NULL OR NOT (` + workItemDonePredicate + `))`
const workItemReviewPredicate = `LOWER(TRIM(status)) IN ('verification','review','testing','in_review','验收','评审','测试')`
const workItemProgressPredicate = `LOWER(TRIM(status)) IN ('in_progress','progress','active','doing','进行中','处理中','排查')`

func NewService(conn *gorm.DB) *Service {
	return &Service{
		repository: NewRepository(conn),
		now:        time.Now,
	}
}

func (s *Service) Repository() *Repository {
	return s.repository
}

func (s *Service) QueryPlan(ctx context.Context, query PlanQuery) (PlanSnapshot, error) {
	if s == nil || s.repository == nil || s.repository.conn == nil {
		return PlanSnapshot{}, fmt.Errorf("database is not initialized")
	}
	conn := s.repository.conn.WithContext(ctx).Model(&db.TaskTelemetry{}).
		Where("LOWER(TRIM(issue_type)) IN ?", []string{
			"requirement", "demand", "story", "需求", "user story", "product requirement",
			"bug", "defect", "缺陷", "故障",
		})

	if projectKeys := normalizeProjectKeys(query.ProjectKeys); len(projectKeys) > 0 {
		clauses := []string{"UPPER(project_key) IN ?"}
		args := []any{projectKeys}
		for _, key := range projectKeys {
			clauses = append(clauses, "(TRIM(project_key) = '' AND UPPER(task_id) LIKE ?)")
			args = append(args, key+"-%")
		}
		conn = conn.Where("("+strings.Join(clauses, " OR ")+")", args...)
	}
	if len(query.Kinds) > 0 {
		kinds := make([]string, 0, len(query.Kinds))
		for _, kind := range query.Kinds {
			normalized, err := NormalizeIssueType(kind)
			if err != nil {
				return PlanSnapshot{}, err
			}
			kinds = append(kinds, normalized)
		}
		kindClauses := make([]string, 0, len(kinds))
		for _, kind := range kinds {
			if kind == WorkItemRequirement {
				kindClauses = append(kindClauses, "LOWER(TRIM(issue_type)) IN ('requirement','demand','story','需求','user story','product requirement')")
			} else if kind == WorkItemBug {
				kindClauses = append(kindClauses, "LOWER(TRIM(issue_type)) IN ('bug','defect','缺陷','故障')")
			}
		}
		if len(kindClauses) > 0 {
			conn = conn.Where("(" + strings.Join(kindClauses, " OR ") + ")")
		}
	}
	if len(query.Statuses) > 0 {
		conn = conn.Where("LOWER(status) IN ?", normalizeStrings(query.Statuses))
	}
	if len(query.PlanningState) > 0 {
		conn = conn.Where("LOWER(planning_state) IN ?", normalizeStrings(query.PlanningState))
	}
	if assignees := normalizeStrings(query.Assignees); len(assignees) > 0 {
		conn = conn.Where("LOWER(TRIM(assignee)) IN ?", assignees)
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		pattern := "%" + strings.ToUpper(search) + "%"
		conn = conn.Where("(UPPER(task_id) LIKE ? OR UPPER(title) LIKE ? OR UPPER(assignee) LIKE ?)", pattern, pattern, pattern)
	}
	if query.ReleaseID > 0 {
		conn = conn.Where(`EXISTS (
			SELECT 1 FROM work_item_release_links links
			WHERE links.work_item_id = task_telemetries.task_id
			  AND links.release_version_id = ?
			  AND links.active = 1
		)`, query.ReleaseID)
	}

	var summary PlanSummary
	summaryProjection := fmt.Sprintf(`
		COUNT(*) AS total,
		COALESCE(SUM(CASE WHEN %[4]s THEN 1 ELSE 0 END), 0) AS active,
		COALESCE(SUM(CASE WHEN %[1]s THEN 1 ELSE 0 END), 0) AS done,
		COALESCE(SUM(CASE WHEN %[4]s AND NOT (%[2]s) AND NOT (%[3]s) THEN 1 ELSE 0 END), 0) AS backlog,
		COALESCE(SUM(CASE WHEN %[3]s THEN 1 ELSE 0 END), 0) AS progress,
		COALESCE(SUM(CASE WHEN %[2]s THEN 1 ELSE 0 END), 0) AS review,
		COALESCE(SUM(CASE WHEN LOWER(TRIM(issue_type)) IN ('bug','defect','缺陷','故障') THEN 0 ELSE 1 END), 0) AS requirements,
		COALESCE(SUM(CASE WHEN LOWER(TRIM(issue_type)) IN ('bug','defect','缺陷','故障') THEN 1 ELSE 0 END), 0) AS bugs,
		COALESCE(SUM(CASE WHEN TRIM(project_key) <> '' THEN 1 ELSE 0 END), 0) AS planned,
		COALESCE(SUM(CASE WHEN TRIM(project_key) = '' THEN 1 ELSE 0 END), 0) AS unplanned
	`, workItemDonePredicate, workItemReviewPredicate, workItemProgressPredicate, workItemActivePredicate)
	if err := conn.Session(&gorm.Session{}).Select(summaryProjection).Scan(&summary).Error; err != nil {
		return PlanSnapshot{}, err
	}
	itemsConn := conn.Session(&gorm.Session{})
	total := summary.Total
	if query.ActiveOnly {
		itemsConn = itemsConn.Where(workItemActivePredicate)
		total = summary.Active
	}
	limit := query.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}
	var tasks []db.TaskTelemetry
	if err := itemsConn.Order("last_update DESC, task_id ASC").Limit(limit).Offset(offset).Find(&tasks).Error; err != nil {
		return PlanSnapshot{}, err
	}
	items, err := loadWorkItemSnapshots(s.repository.conn.WithContext(ctx), tasks)
	if err != nil {
		return PlanSnapshot{}, err
	}
	return PlanSnapshot{Items: items, Total: total, Limit: limit, Offset: offset, Summary: summary}, nil
}

func (s *Service) QueryWorkItem(ctx context.Context, workItemID string) (WorkItemSnapshot, error) {
	if s == nil || s.repository == nil || s.repository.conn == nil {
		return WorkItemSnapshot{}, fmt.Errorf("database is not initialized")
	}
	var task db.TaskTelemetry
	if err := s.repository.conn.WithContext(ctx).Where("task_id = ?", strings.TrimSpace(workItemID)).First(&task).Error; err != nil {
		return WorkItemSnapshot{}, err
	}
	kind, err := NormalizeIssueType(task.IssueType)
	if err != nil || kind == ExecutionTask {
		if err != nil {
			return WorkItemSnapshot{}, err
		}
		return WorkItemSnapshot{}, gorm.ErrRecordNotFound
	}
	return loadWorkItemSnapshot(s.repository.conn.WithContext(ctx), task)
}

func (s *Service) QueryRelease(ctx context.Context, query ReleaseQuery) (ReleaseSnapshot, error) {
	return s.repository.ReleaseSnapshot(ctx, query.ReleaseID)
}

func (s *Service) ApplyPlanningChange(ctx context.Context, command PlanningCommand) (WorkItemSnapshot, error) {
	if s == nil || s.repository == nil || s.repository.conn == nil {
		return WorkItemSnapshot{}, fmt.Errorf("database is not initialized")
	}
	command.WorkItemID = strings.TrimSpace(command.WorkItemID)
	command.Actor = strings.TrimSpace(command.Actor)
	command.Reason = strings.TrimSpace(command.Reason)
	command.Source = strings.ToLower(strings.TrimSpace(command.Source))
	if command.Source == "" {
		command.Source = "manual"
	}
	if command.WorkItemID == "" {
		return WorkItemSnapshot{}, &DomainError{Code: "work_item_required", Message: "work item id is required", StatusCode: 422}
	}
	if command.Actor == "" {
		return WorkItemSnapshot{}, &DomainError{Code: "actor_required", Message: "actor is required", StatusCode: 422}
	}
	if command.Reason == "" {
		return WorkItemSnapshot{}, &DomainError{Code: "reason_required", Message: "a reason is required for planning changes", StatusCode: 422}
	}

	var result WorkItemSnapshot
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
				Code:       "execution_task_not_plannable",
				Message:    "execution tasks inherit planning facts from their parent work item",
				StatusCode: 422,
			}
		}
		if current.Revision != command.ExpectedRevision {
			return &DomainError{
				Code:       "revision_conflict",
				Message:    fmt.Sprintf("planning revision changed from %d to %d", command.ExpectedRevision, current.Revision),
				StatusCode: 409,
			}
		}

		before, err := loadWorkItemSnapshot(tx, current)
		if err != nil {
			return err
		}
		next := current
		if command.ProjectKey != nil {
			next.ProjectKey = NormalizeProjectKey(*command.ProjectKey)
		}
		if command.Assignee != nil {
			next.Assignee = strings.TrimSpace(*command.Assignee)
		}
		if command.DueDate != nil {
			next.DueDate = *command.DueDate
		}
		if command.PlanningState != nil {
			next.PlanningState = strings.ToLower(strings.TrimSpace(*command.PlanningState))
		}
		if strings.TrimSpace(next.PlanningState) == "" {
			next.PlanningState = PlanningDraft
		}

		primaryReleaseID := primaryReleaseID(before.Links)
		if command.PrimaryTargetReleaseID != nil {
			primaryReleaseID = *command.PrimaryTargetReleaseID
		}
		if err := ValidatePlanningGate(kind, next.ProjectKey, next.PlanningState, primaryReleaseID); err != nil {
			return err
		}
		if primaryReleaseID > 0 {
			var release db.ReleaseVersion
			if err := tx.First(&release, primaryReleaseID).Error; err != nil {
				return err
			}
			if err := ValidateReleaseForWorkItem(release, next.ProjectKey, ReleaseTargetFix); err != nil {
				return err
			}
		}

		affectedIDs := activeAffectedReleaseIDs(before.Links)
		if command.AffectedReleaseIDs != nil {
			affectedIDs = uniqueUint(*command.AffectedReleaseIDs)
		}
		if len(affectedIDs) > 0 && kind != WorkItemBug {
			return &DomainError{
				Code:       "affected_release_only_for_bug",
				Message:    "affected releases are only valid for bugs",
				StatusCode: 422,
			}
		}
		for _, releaseID := range affectedIDs {
			var release db.ReleaseVersion
			if err := tx.First(&release, releaseID).Error; err != nil {
				return err
			}
			if err := ValidateReleaseForWorkItem(release, next.ProjectKey, ReleaseAffected); err != nil {
				return err
			}
		}

		nextRevision := current.Revision + 1
		updates := map[string]any{
			"project_key":    next.ProjectKey,
			"assignee":       next.Assignee,
			"due_date":       next.DueDate,
			"planning_state": next.PlanningState,
			"revision":       nextRevision,
			"last_update":    s.now(),
		}
		update := tx.Model(&db.TaskTelemetry{}).
			Where("task_id = ? AND revision = ?", current.TaskID, command.ExpectedRevision).
			Updates(updates)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return &DomainError{
				Code:       "revision_conflict",
				Message:    "planning revision changed while the update was being applied",
				StatusCode: 409,
			}
		}

		releaseChanged := false
		if command.PrimaryTargetReleaseID != nil {
			releaseChanged = primaryReleaseID != primaryReleaseIDFromSnapshot(before)
			if err := applyPrimaryTarget(tx, current.TaskID, primaryReleaseID, command.Actor, s.now()); err != nil {
				return err
			}
		}
		if command.AffectedReleaseIDs != nil {
			releaseChanged = releaseChanged || !sameUintSet(affectedIDs, activeAffectedReleaseIDs(before.Links))
			if err := applyAffectedReleases(tx, current.TaskID, affectedIDs, command.Actor, s.now()); err != nil {
				return err
			}
		}

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
			EventType:  planningEventType(current, next, releaseChanged),
			Actor:      command.Actor,
			Reason:     command.Reason,
			BeforeJSON: string(beforeJSON),
			AfterJSON:  string(afterJSON),
			Revision:   nextRevision,
			Source:     command.Source,
			SyncState:  "not_required",
			CreatedAt:  s.now(),
		}
		if releaseChanged &&
			jiraReleaseScopeChanged(before, after) &&
			strings.EqualFold(current.Source, "jira") &&
			strings.TrimSpace(current.ExternalKey) != "" {
			payload, err := releaseSyncPayload(after)
			if err != nil {
				return err
			}
			operation := db.WorkItemSyncOperation{
				IdempotencyKey: fmt.Sprintf("%s:%d:update_versions", current.TaskID, nextRevision),
				WorkItemID:     current.TaskID,
				Operation:      "update_issue_versions",
				PayloadJSON:    payload,
				Status:         "pending",
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&operation).Error; err != nil {
				return err
			}
			event.SyncState = "pending"
		}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		if _, err := appendWorkItemAsset(ctx, tx, event); err != nil {
			return err
		}
		result = after
		return nil
	})
	if err != nil {
		return WorkItemSnapshot{}, err
	}
	return result, nil
}

func loadWorkItemSnapshot(conn *gorm.DB, task db.TaskTelemetry) (WorkItemSnapshot, error) {
	var links []db.WorkItemReleaseLink
	if err := conn.Where("work_item_id = ? AND active = ?", task.TaskID, true).
		Order("relation ASC, is_primary DESC, release_version_id ASC").
		Find(&links).Error; err != nil {
		return WorkItemSnapshot{}, err
	}
	releaseIDs := make([]uint, 0, len(links))
	for _, link := range links {
		releaseIDs = append(releaseIDs, link.ReleaseVersionID)
	}
	releaseByID := map[uint]db.ReleaseVersion{}
	if len(releaseIDs) > 0 {
		var releases []db.ReleaseVersion
		if err := conn.Where("id IN ?", uniqueUint(releaseIDs)).Find(&releases).Error; err != nil {
			return WorkItemSnapshot{}, err
		}
		for _, release := range releases {
			releaseByID[release.ID] = release
		}
	}
	snapshot := WorkItemSnapshot{
		WorkItem:  task,
		Links:     links,
		SyncState: "not_required",
	}
	for _, link := range links {
		release, ok := releaseByID[link.ReleaseVersionID]
		if !ok {
			continue
		}
		if link.Relation == ReleaseAffected {
			snapshot.Affected = append(snapshot.Affected, release)
		} else if link.Relation == ReleaseTargetFix {
			snapshot.TargetReleases = append(snapshot.TargetReleases, release)
		}
	}
	var latestOperation db.WorkItemSyncOperation
	err := conn.Where("work_item_id = ?", task.TaskID).Order("id DESC").First(&latestOperation).Error
	if err == nil {
		snapshot.SyncState = latestOperation.Status
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return WorkItemSnapshot{}, err
	}
	return snapshot, nil
}

func loadWorkItemSnapshots(conn *gorm.DB, tasks []db.TaskTelemetry) ([]WorkItemSnapshot, error) {
	if len(tasks) == 0 {
		return []WorkItemSnapshot{}, nil
	}

	workItemIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		workItemIDs = append(workItemIDs, task.TaskID)
	}

	var links []db.WorkItemReleaseLink
	if err := conn.Where("work_item_id IN ? AND active = ?", workItemIDs, true).
		Order("work_item_id ASC, relation ASC, is_primary DESC, release_version_id ASC").
		Find(&links).Error; err != nil {
		return nil, err
	}
	linksByWorkItem := make(map[string][]db.WorkItemReleaseLink, len(tasks))
	releaseIDs := make([]uint, 0, len(links))
	for _, link := range links {
		linksByWorkItem[link.WorkItemID] = append(linksByWorkItem[link.WorkItemID], link)
		releaseIDs = append(releaseIDs, link.ReleaseVersionID)
	}

	releaseByID := make(map[uint]db.ReleaseVersion)
	if len(releaseIDs) > 0 {
		var releases []db.ReleaseVersion
		if err := conn.Where("id IN ?", uniqueUint(releaseIDs)).Find(&releases).Error; err != nil {
			return nil, err
		}
		for _, release := range releases {
			releaseByID[release.ID] = release
		}
	}

	type latestSyncState struct {
		WorkItemID string
		Status     string
	}
	var latestStates []latestSyncState
	if err := conn.Table("work_item_sync_operations AS operations").
		Select("operations.work_item_id, operations.status").
		Joins(`JOIN (
			SELECT work_item_id, MAX(id) AS latest_id
			FROM work_item_sync_operations
			WHERE work_item_id IN ?
			GROUP BY work_item_id
		) AS latest ON latest.latest_id = operations.id`, workItemIDs).
		Scan(&latestStates).Error; err != nil {
		return nil, err
	}
	syncStateByWorkItem := make(map[string]string, len(latestStates))
	for _, state := range latestStates {
		syncStateByWorkItem[state.WorkItemID] = state.Status
	}

	items := make([]WorkItemSnapshot, 0, len(tasks))
	for _, task := range tasks {
		snapshot := WorkItemSnapshot{
			WorkItem:  task,
			Links:     linksByWorkItem[task.TaskID],
			SyncState: "not_required",
		}
		if state := syncStateByWorkItem[task.TaskID]; state != "" {
			snapshot.SyncState = state
		}
		for _, link := range snapshot.Links {
			release, ok := releaseByID[link.ReleaseVersionID]
			if !ok {
				continue
			}
			switch link.Relation {
			case ReleaseAffected:
				snapshot.Affected = append(snapshot.Affected, release)
			case ReleaseTargetFix:
				snapshot.TargetReleases = append(snapshot.TargetReleases, release)
			}
		}
		items = append(items, snapshot)
	}
	return items, nil
}

func applyPrimaryTarget(conn *gorm.DB, workItemID string, releaseID uint, actor string, now time.Time) error {
	if err := conn.Model(&db.WorkItemReleaseLink{}).
		Where("work_item_id = ? AND relation = ? AND is_primary = ?", workItemID, ReleaseTargetFix, true).
		Update("is_primary", false).Error; err != nil {
		return err
	}
	if releaseID == 0 {
		return nil
	}
	var link db.WorkItemReleaseLink
	err := conn.Where(
		"work_item_id = ? AND release_version_id = ? AND relation = ?",
		workItemID, releaseID, ReleaseTargetFix,
	).First(&link).Error
	switch {
	case err == nil:
		return conn.Model(&link).Updates(map[string]any{
			"is_primary":   true,
			"active":       true,
			"confirmed_by": actor,
			"confirmed_at": now,
		}).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		return conn.Create(&db.WorkItemReleaseLink{
			WorkItemID:       workItemID,
			ReleaseVersionID: releaseID,
			Relation:         ReleaseTargetFix,
			IsPrimary:        true,
			Active:           true,
			Source:           "manual",
			ConfirmedBy:      actor,
			ConfirmedAt:      &now,
		}).Error
	default:
		return err
	}
}

func applyAffectedReleases(conn *gorm.DB, workItemID string, releaseIDs []uint, actor string, now time.Time) error {
	if err := conn.Model(&db.WorkItemReleaseLink{}).
		Where("work_item_id = ? AND relation = ? AND source IN ?", workItemID, ReleaseAffected, []string{"manual", "local"}).
		Update("active", false).Error; err != nil {
		return err
	}
	for _, releaseID := range uniqueUint(releaseIDs) {
		var link db.WorkItemReleaseLink
		err := conn.Where(
			"work_item_id = ? AND release_version_id = ? AND relation = ?",
			workItemID, releaseID, ReleaseAffected,
		).First(&link).Error
		switch {
		case err == nil:
			if err := conn.Model(&link).Updates(map[string]any{
				"active":       true,
				"confirmed_by": actor,
				"confirmed_at": now,
			}).Error; err != nil {
				return err
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := conn.Create(&db.WorkItemReleaseLink{
				WorkItemID:       workItemID,
				ReleaseVersionID: releaseID,
				Relation:         ReleaseAffected,
				Active:           true,
				Source:           "manual",
				ConfirmedBy:      actor,
				ConfirmedAt:      &now,
			}).Error; err != nil {
				return err
			}
		default:
			return err
		}
	}
	return nil
}

func releaseSyncPayload(snapshot WorkItemSnapshot) (string, error) {
	target := make([]string, 0, len(snapshot.TargetReleases))
	affected := make([]string, 0, len(snapshot.Affected))
	for _, release := range snapshot.TargetReleases {
		if release.Source == "jira" && release.ExternalID != "" {
			target = append(target, release.ExternalID)
		}
	}
	for _, release := range snapshot.Affected {
		if release.Source == "jira" && release.ExternalID != "" {
			affected = append(affected, release.ExternalID)
		}
	}
	sort.Strings(target)
	sort.Strings(affected)
	payload, err := json.Marshal(map[string]any{
		"issue_key":             snapshot.WorkItem.ExternalKey,
		"target_external_ids":   target,
		"affected_external_ids": affected,
	})
	return string(payload), err
}

func jiraReleaseScopeChanged(before, after WorkItemSnapshot) bool {
	beforeTarget, beforeAffected := externalJiraReleaseIDs(before)
	afterTarget, afterAffected := externalJiraReleaseIDs(after)
	return strings.Join(beforeTarget, "\x00") != strings.Join(afterTarget, "\x00") ||
		strings.Join(beforeAffected, "\x00") != strings.Join(afterAffected, "\x00")
}

func externalJiraReleaseIDs(snapshot WorkItemSnapshot) ([]string, []string) {
	target := make([]string, 0, len(snapshot.TargetReleases))
	affected := make([]string, 0, len(snapshot.Affected))
	for _, release := range snapshot.TargetReleases {
		if strings.EqualFold(strings.TrimSpace(release.Source), "jira") && strings.TrimSpace(release.ExternalID) != "" {
			target = append(target, strings.TrimSpace(release.ExternalID))
		}
	}
	for _, release := range snapshot.Affected {
		if strings.EqualFold(strings.TrimSpace(release.Source), "jira") && strings.TrimSpace(release.ExternalID) != "" {
			affected = append(affected, strings.TrimSpace(release.ExternalID))
		}
	}
	sort.Strings(target)
	sort.Strings(affected)
	return uniqueStrings(target), uniqueStrings(affected)
}

func primaryReleaseID(links []db.WorkItemReleaseLink) uint {
	for _, link := range links {
		if link.Active && link.Relation == ReleaseTargetFix && link.IsPrimary {
			return link.ReleaseVersionID
		}
	}
	return 0
}

func primaryReleaseIDFromSnapshot(snapshot WorkItemSnapshot) uint {
	return primaryReleaseID(snapshot.Links)
}

func activeAffectedReleaseIDs(links []db.WorkItemReleaseLink) []uint {
	ids := make([]uint, 0)
	for _, link := range links {
		if link.Active && link.Relation == ReleaseAffected {
			ids = append(ids, link.ReleaseVersionID)
		}
	}
	return uniqueUint(ids)
}

func planningEventType(before, after db.TaskTelemetry, releaseChanged bool) string {
	if NormalizeProjectKey(before.ProjectKey) != NormalizeProjectKey(after.ProjectKey) {
		if strings.TrimSpace(before.ProjectKey) == "" {
			return "project_assigned"
		}
		return "project_changed"
	}
	if releaseChanged {
		return "target_release_changed"
	}
	if before.PlanningState != after.PlanningState {
		return "planning_state_changed"
	}
	return "planning_changed"
}

func normalizeProjectKeys(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = NormalizeProjectKey(value)
		if value != "" {
			normalized = append(normalized, value)
		}
	}
	return uniqueStrings(normalized)
}

func normalizeStrings(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			normalized = append(normalized, value)
		}
	}
	return uniqueStrings(normalized)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func uniqueUint(values []uint) []uint {
	seen := make(map[uint]struct{}, len(values))
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func sameUintSet(left, right []uint) bool {
	left = uniqueUint(left)
	right = uniqueUint(right)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
