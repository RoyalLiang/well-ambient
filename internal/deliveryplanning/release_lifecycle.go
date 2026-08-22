package deliveryplanning

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"well-ambient/internal/dataassets"
	"well-ambient/internal/db"

	"gorm.io/gorm"
)

type ReleaseLifecycleCommand struct {
	ReleaseID uint
	Actor     string
	Reason    string
}

type ReleaseLifecycleResult struct {
	Release     db.ReleaseVersion `json:"release"`
	EvidenceRef string            `json:"evidence_ref"`
	Replayed    bool              `json:"replayed"`
}

type DeleteReleaseCommand struct {
	ReleaseID   uint
	Actor       string
	Reason      string
	ConfirmName string
}

type DeleteReleaseResult struct {
	ReleaseID   uint   `json:"release_id"`
	ReleaseName string `json:"release_name"`
	EvidenceRef string `json:"evidence_ref"`
	Replayed    bool   `json:"replayed"`
}

type releaseLifecycleAssetPayload struct {
	Before releaseVersionAssetFacts  `json:"before"`
	After  *releaseVersionAssetFacts `json:"after,omitempty"`
	Reason string                    `json:"reason"`
}

func (s *Service) ArchiveRelease(ctx context.Context, command ReleaseLifecycleCommand) (ReleaseLifecycleResult, error) {
	return s.transitionRelease(ctx, command, ReleaseReleased, ReleaseArchived, "archived")
}

func (s *Service) DiscardRelease(ctx context.Context, command ReleaseLifecycleCommand) (ReleaseLifecycleResult, error) {
	return s.transitionRelease(ctx, command, ReleasePlanned, ReleaseDiscarded, "discarded")
}

func (s *Service) transitionRelease(
	ctx context.Context,
	command ReleaseLifecycleCommand,
	requiredStatus string,
	targetStatus string,
	action string,
) (ReleaseLifecycleResult, error) {
	if s == nil || s.repository == nil || s.repository.conn == nil {
		return ReleaseLifecycleResult{}, fmt.Errorf("database is not initialized")
	}
	command.Actor = strings.TrimSpace(command.Actor)
	command.Reason = strings.TrimSpace(command.Reason)
	if command.ReleaseID == 0 {
		return ReleaseLifecycleResult{}, &DomainError{Code: "release_required", Message: "release id is required", StatusCode: 422}
	}
	if command.Actor == "" {
		return ReleaseLifecycleResult{}, &DomainError{Code: "actor_required", Message: "actor is required", StatusCode: 422}
	}
	if command.Reason == "" {
		return ReleaseLifecycleResult{}, &DomainError{
			Code:       "reason_required",
			Message:    fmt.Sprintf("a reason is required to mark a release as %s", action),
			StatusCode: 422,
		}
	}

	result := ReleaseLifecycleResult{}
	err := s.repository.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current db.ReleaseVersion
		if err := tx.First(&current, command.ReleaseID).Error; err != nil {
			return err
		}
		if !strings.EqualFold(strings.TrimSpace(current.Source), "local") {
			return &DomainError{
				Code:       "external_release_read_only",
				Message:    "external release status must be changed in its source system",
				StatusCode: 409,
			}
		}
		if current.Status == targetStatus {
			event, err := findReleaseLifecycleEvent(tx, current.ID, "release_version_"+action)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return &DomainError{
						Code:       "release_already_" + action,
						Message:    fmt.Sprintf("release is already %s", action),
						StatusCode: 409,
					}
				}
				return err
			}
			result = ReleaseLifecycleResult{
				Release:     current,
				EvidenceRef: fmt.Sprintf("data_asset_event:%d", event.ID),
				Replayed:    true,
			}
			return nil
		}
		if current.Status != requiredStatus {
			return &DomainError{
				Code: "invalid_release_transition",
				Message: fmt.Sprintf(
					"only a %s release can be marked as %s",
					requiredStatus,
					action,
				),
				StatusCode: 409,
			}
		}

		before := current
		now := s.now().UTC()
		update := tx.Model(&db.ReleaseVersion{}).
			Where("id = ? AND status = ?", current.ID, requiredStatus).
			Updates(map[string]any{"status": targetStatus, "updated_at": now})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return &DomainError{
				Code:       "release_state_conflict",
				Message:    "release status changed while the lifecycle action was being applied",
				StatusCode: 409,
			}
		}
		if err := tx.First(&current, command.ReleaseID).Error; err != nil {
			return err
		}
		after := releaseVersionFacts(current)
		asset, err := dataassets.New(tx).Append(ctx, dataassets.AppendCommand{
			DedupeKey:      fmt.Sprintf("release_version:%d:%s", current.ID, action),
			ProjectKey:     current.ProjectKey,
			SubjectType:    "release_version",
			SubjectID:      strconv.FormatUint(uint64(current.ID), 10),
			EventType:      "release_version_" + action,
			SourceSystem:   "local",
			SourceRecordID: current.ExternalID,
			SourceEventID:  current.UpdatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
			ActorID:        command.Actor,
			ActorRole:      "release_manager",
			CorrelationID:  fmt.Sprintf("release_version:%d", current.ID),
			Classification: dataassets.ClassificationInternal,
			RetentionClass: dataassets.RetentionLongTerm,
			SchemaVersion:  1,
			OccurredAt:     now,
			ObservedAt:     now,
			Payload: releaseLifecycleAssetPayload{
				Before: releaseVersionFacts(before),
				After:  &after,
				Reason: command.Reason,
			},
		})
		if err != nil {
			return err
		}
		result = ReleaseLifecycleResult{
			Release:     current,
			EvidenceRef: fmt.Sprintf("data_asset_event:%d", asset.Event.ID),
			Replayed:    asset.Replayed,
		}
		return nil
	})
	return result, err
}

func (s *Service) DeleteRelease(ctx context.Context, command DeleteReleaseCommand) (DeleteReleaseResult, error) {
	if s == nil || s.repository == nil || s.repository.conn == nil {
		return DeleteReleaseResult{}, fmt.Errorf("database is not initialized")
	}
	command.Actor = strings.TrimSpace(command.Actor)
	command.Reason = strings.TrimSpace(command.Reason)
	command.ConfirmName = strings.TrimSpace(command.ConfirmName)
	if command.ReleaseID == 0 {
		return DeleteReleaseResult{}, &DomainError{Code: "release_required", Message: "release id is required", StatusCode: 422}
	}
	if command.Actor == "" {
		return DeleteReleaseResult{}, &DomainError{Code: "actor_required", Message: "actor is required", StatusCode: 422}
	}
	if command.Reason == "" {
		return DeleteReleaseResult{}, &DomainError{Code: "reason_required", Message: "a reason is required to delete a release", StatusCode: 422}
	}

	result := DeleteReleaseResult{}
	err := s.repository.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current db.ReleaseVersion
		if err := tx.First(&current, command.ReleaseID).Error; err != nil {
			return err
		}
		if !strings.EqualFold(strings.TrimSpace(current.Source), "local") {
			return &DomainError{Code: "external_release_read_only", Message: "external releases cannot be deleted locally", StatusCode: 409}
		}
		if current.Status != ReleasePlanned && current.Status != ReleaseDiscarded {
			return &DomainError{
				Code:       "release_delete_forbidden",
				Message:    "published or archived release facts cannot be deleted",
				StatusCode: 409,
			}
		}
		if command.ConfirmName != current.Name {
			return &DomainError{
				Code:       "release_delete_confirmation_mismatch",
				Message:    "release name confirmation does not match",
				StatusCode: 422,
			}
		}

		var activeWorkItemCount int64
		if err := tx.Model(&db.WorkItemReleaseLink{}).
			Where("release_version_id = ? AND active = ?", current.ID, true).
			Count(&activeWorkItemCount).Error; err != nil {
			return err
		}
		var jiraLinkCount int64
		if err := tx.Model(&db.ReleaseJiraLink{}).
			Where("release_version_id = ?", current.ID).
			Count(&jiraLinkCount).Error; err != nil {
			return err
		}
		if activeWorkItemCount > 0 || jiraLinkCount > 0 {
			return &DomainError{
				Code:       "release_delete_blocked",
				Message:    "remove active Jira work items and Jira release links before deleting this release",
				StatusCode: 409,
			}
		}

		now := s.now().UTC()
		asset, err := dataassets.New(tx).Append(ctx, dataassets.AppendCommand{
			DedupeKey:      fmt.Sprintf("release_version:%d:deleted", current.ID),
			ProjectKey:     current.ProjectKey,
			SubjectType:    "release_version",
			SubjectID:      strconv.FormatUint(uint64(current.ID), 10),
			EventType:      "release_version_deleted",
			SourceSystem:   "local",
			SourceRecordID: current.ExternalID,
			SourceEventID:  now.Format("2006-01-02T15:04:05.999999999Z07:00"),
			ActorID:        command.Actor,
			ActorRole:      "release_manager",
			CorrelationID:  fmt.Sprintf("release_version:%d", current.ID),
			Classification: dataassets.ClassificationInternal,
			RetentionClass: dataassets.RetentionLongTerm,
			SchemaVersion:  1,
			OccurredAt:     now,
			ObservedAt:     now,
			Payload: releaseLifecycleAssetPayload{
				Before: releaseVersionFacts(current),
				Reason: command.Reason,
			},
		})
		if err != nil {
			return err
		}
		deleted := tx.Delete(&current)
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return &DomainError{
				Code:       "release_state_conflict",
				Message:    "release changed while it was being deleted",
				StatusCode: 409,
			}
		}
		result = DeleteReleaseResult{
			ReleaseID:   current.ID,
			ReleaseName: current.Name,
			EvidenceRef: fmt.Sprintf("data_asset_event:%d", asset.Event.ID),
			Replayed:    asset.Replayed,
		}
		return nil
	})
	return result, err
}

func findReleaseLifecycleEvent(tx *gorm.DB, releaseID uint, eventType string) (db.DataAssetEvent, error) {
	var event db.DataAssetEvent
	err := tx.Where(
		"subject_type = ? AND subject_id = ? AND event_type = ?",
		"release_version",
		strconv.FormatUint(uint64(releaseID), 10),
		eventType,
	).Order("id DESC").First(&event).Error
	return event, err
}
