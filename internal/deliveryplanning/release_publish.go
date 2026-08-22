package deliveryplanning

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"well-ambient/internal/dataassets"
	"well-ambient/internal/db"

	"gorm.io/gorm"
)

type PublishReleaseCommand struct {
	ReleaseID   uint
	ReleaseDate time.Time
	Actor       string
	Reason      string
}

type PublishReleaseResult struct {
	Release     db.ReleaseVersion `json:"release"`
	EvidenceRef string            `json:"evidence_ref"`
	Replayed    bool              `json:"replayed"`
}

type releasePublishedAssetPayload struct {
	Before releaseVersionAssetFacts `json:"before"`
	After  releaseVersionAssetFacts `json:"after"`
	Reason string                   `json:"reason"`
}

func (s *Service) PublishRelease(ctx context.Context, command PublishReleaseCommand) (PublishReleaseResult, error) {
	if s == nil || s.repository == nil || s.repository.conn == nil {
		return PublishReleaseResult{}, fmt.Errorf("database is not initialized")
	}
	command.Actor = strings.TrimSpace(command.Actor)
	command.Reason = strings.TrimSpace(command.Reason)
	if command.ReleaseID == 0 {
		return PublishReleaseResult{}, &DomainError{Code: "release_required", Message: "release id is required", StatusCode: 422}
	}
	if command.ReleaseDate.IsZero() {
		return PublishReleaseResult{}, &DomainError{Code: "release_date_required", Message: "release date is required", StatusCode: 422}
	}
	if command.Actor == "" {
		return PublishReleaseResult{}, &DomainError{Code: "actor_required", Message: "actor is required", StatusCode: 422}
	}
	if command.Reason == "" {
		return PublishReleaseResult{}, &DomainError{Code: "reason_required", Message: "a reason is required to publish a release", StatusCode: 422}
	}

	result := PublishReleaseResult{}
	err := s.repository.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current db.ReleaseVersion
		if err := tx.First(&current, command.ReleaseID).Error; err != nil {
			return err
		}
		if NormalizeProjectKey(current.ProjectKey) == "" {
			return &DomainError{
				Code:       "release_project_required",
				Message:    "a project is required before publishing a release",
				StatusCode: 422,
			}
		}
		if !strings.EqualFold(strings.TrimSpace(current.Source), "local") {
			return &DomainError{
				Code:       "external_release_read_only",
				Message:    "external release status must be changed in its source system",
				StatusCode: 409,
			}
		}
		if current.Status == ReleaseArchived {
			return &DomainError{
				Code:       "invalid_release_transition",
				Message:    "an archived release cannot be published",
				StatusCode: 409,
			}
		}
		if current.Status == ReleaseReleased {
			var event db.DataAssetEvent
			err := tx.Where(
				"subject_type = ? AND subject_id = ? AND event_type = ?",
				"release_version",
				strconv.FormatUint(uint64(current.ID), 10),
				"release_version_published",
			).Order("id DESC").First(&event).Error
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return &DomainError{
						Code:       "release_already_published",
						Message:    "release is already published",
						StatusCode: 409,
					}
				}
				return err
			}
			result = PublishReleaseResult{
				Release:     current,
				EvidenceRef: fmt.Sprintf("data_asset_event:%d", event.ID),
				Replayed:    true,
			}
			return nil
		}
		if current.Status != ReleasePlanned {
			return &DomainError{
				Code:       "invalid_release_transition",
				Message:    "only a planned release can be published",
				StatusCode: 409,
			}
		}

		before := current
		now := s.now().UTC()
		update := tx.Model(&db.ReleaseVersion{}).
			Where("id = ? AND status = ?", current.ID, ReleasePlanned).
			Updates(map[string]any{
				"status":       ReleaseReleased,
				"release_date": command.ReleaseDate,
				"updated_at":   now,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return &DomainError{
				Code:       "release_state_conflict",
				Message:    "release status changed while it was being published",
				StatusCode: 409,
			}
		}
		if err := tx.First(&current, command.ReleaseID).Error; err != nil {
			return err
		}
		asset, err := dataassets.New(tx).Append(ctx, dataassets.AppendCommand{
			DedupeKey:      fmt.Sprintf("release_version:%d:published", current.ID),
			ProjectKey:     current.ProjectKey,
			SubjectType:    "release_version",
			SubjectID:      strconv.FormatUint(uint64(current.ID), 10),
			EventType:      "release_version_published",
			SourceSystem:   "local",
			SourceRecordID: current.ExternalID,
			SourceEventID:  current.UpdatedAt.UTC().Format(time.RFC3339Nano),
			ActorID:        command.Actor,
			ActorRole:      "release_manager",
			CorrelationID:  fmt.Sprintf("release_version:%d", current.ID),
			Classification: dataassets.ClassificationInternal,
			RetentionClass: dataassets.RetentionLongTerm,
			SchemaVersion:  1,
			OccurredAt:     now,
			ObservedAt:     now,
			Payload: releasePublishedAssetPayload{
				Before: releaseVersionFacts(before),
				After:  releaseVersionFacts(current),
				Reason: command.Reason,
			},
		})
		if err != nil {
			return err
		}
		result = PublishReleaseResult{
			Release:     current,
			EvidenceRef: fmt.Sprintf("data_asset_event:%d", asset.Event.ID),
			Replayed:    asset.Replayed,
		}
		return nil
	})
	return result, err
}
