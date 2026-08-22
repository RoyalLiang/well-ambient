package performance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"well-ambient/internal/db"

	"gorm.io/gorm"
)

var ErrSourceEventConflict = errors.New("performance source event conflicts with retained payload")

// AppendSourceEvent stores one idempotent immutable Jira/Git source fact. It
// does not trigger a score run; the silent scheduler consumes it at its next
// fixed watermark.
func (m *Module) AppendSourceEvent(ctx context.Context, command SourceEventCommand) (SourceEventResult, error) {
	if m == nil || m.db == nil {
		return SourceEventResult{}, errors.New("performance module database is not configured")
	}
	command.DedupeKey = strings.TrimSpace(command.DedupeKey)
	command.WorkItemID = strings.TrimSpace(command.WorkItemID)
	command.ProjectKey = strings.TrimSpace(command.ProjectKey)
	command.IssueType = strings.ToLower(strings.TrimSpace(command.IssueType))
	command.EventType = strings.ToLower(strings.TrimSpace(command.EventType))
	command.FieldName = strings.ToLower(strings.TrimSpace(command.FieldName))
	command.SourceSystem = strings.ToLower(strings.TrimSpace(command.SourceSystem))
	command.SourceEventID = strings.TrimSpace(command.SourceEventID)
	if command.DedupeKey == "" || command.WorkItemID == "" || command.EventType == "" || command.SourceSystem == "" || command.SourceEventID == "" || command.OccurredAt.IsZero() {
		return SourceEventResult{}, errors.New("source event requires dedupe_key, work_item_id, event_type, occurred_at, source_system, and source_event_id")
	}
	if command.Payload == nil {
		command.Payload = map[string]any{}
	}
	payload, err := json.Marshal(command.Payload)
	if err != nil {
		return SourceEventResult{}, fmt.Errorf("encode performance source event: %w", err)
	}
	digestInput, err := json.Marshal(struct {
		WorkItemID, ProjectKey, IssueType, EventType, FieldName, FromValue, ToValue, Actor string
		OccurredAt                                                                         time.Time
		SourceSystem, SourceEventID                                                        string
		Payload                                                                            json.RawMessage
	}{
		command.WorkItemID, command.ProjectKey, command.IssueType, command.EventType,
		command.FieldName, command.FromValue, command.ToValue, command.Actor,
		command.OccurredAt.UTC(), command.SourceSystem, command.SourceEventID, payload,
	})
	if err != nil {
		return SourceEventResult{}, err
	}
	digest := sha256.Sum256(digestInput)
	payloadHash := hex.EncodeToString(digest[:])
	var result SourceEventResult
	err = m.withBusyRetry(ctx, func() error {
		return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var existing db.PerformanceWorkItemEvent
			lookup := tx.Where("dedupe_key = ?", command.DedupeKey).Limit(1).Find(&existing)
			if lookup.Error != nil {
				return lookup.Error
			}
			if lookup.RowsAffected > 0 {
				if existing.PayloadHash != payloadHash {
					return fmt.Errorf("%w: %q", ErrSourceEventConflict, command.DedupeKey)
				}
				result = SourceEventResult{ID: existing.ID, Replayed: true}
				return nil
			}
			now := m.now().UTC()
			row := db.PerformanceWorkItemEvent{
				DedupeKey: command.DedupeKey, WorkItemID: command.WorkItemID,
				ProjectKey: command.ProjectKey, IssueType: command.IssueType,
				EventType: command.EventType, FieldName: command.FieldName,
				FromValue: command.FromValue, ToValue: command.ToValue, Actor: command.Actor,
				OccurredAt: command.OccurredAt.UTC(), ObservedAt: now,
				SourceSystem: command.SourceSystem, SourceEventID: command.SourceEventID,
				PayloadJSON: string(payload), PayloadHash: payloadHash, CreatedAt: now,
			}
			if createErr := tx.Create(&row).Error; createErr != nil {
				// A concurrent ingestion may win after the initial lookup. Resolve
				// that race as an idempotent replay without weakening the immutable
				// payload contract.
				var concurrent db.PerformanceWorkItemEvent
				concurrentLookup := tx.Where("dedupe_key = ?", command.DedupeKey).Limit(1).Find(&concurrent)
				if concurrentLookup.Error == nil && concurrentLookup.RowsAffected > 0 {
					if concurrent.PayloadHash != payloadHash {
						return fmt.Errorf("%w: %q", ErrSourceEventConflict, command.DedupeKey)
					}
					result = SourceEventResult{ID: concurrent.ID, Replayed: true}
					return nil
				}
				return createErr
			}
			if err := createAudit(tx, db.PerformanceAuditEvent{
				RunID: "source:" + payloadHash[:16], EventType: "source_event_recorded",
				RecordType: "performance_work_item_event", RecordID: row.ID,
				FormulaVersion: m.currentSettings().FormulaVersion, CreatedAt: now,
			}, map[string]any{
				"dedupe_key": command.DedupeKey, "work_item_id": command.WorkItemID,
				"event_type": command.EventType, "source_event_id": command.SourceEventID,
				"payload_hash": payloadHash,
			}); err != nil {
				return err
			}
			result = SourceEventResult{ID: row.ID}
			return nil
		})
	})
	return result, err
}

// AppendSourceEventAllowingActorDrift preserves the retained actor label when
// a source system rewrites historical display names. All other immutable event
// fields still pass through the normal payload-hash conflict check.
func (m *Module) AppendSourceEventAllowingActorDrift(ctx context.Context, command SourceEventCommand) (SourceEventResult, error) {
	result, err := m.AppendSourceEvent(ctx, command)
	if !errors.Is(err, ErrSourceEventConflict) || m == nil || m.db == nil {
		return result, err
	}

	var existing db.PerformanceWorkItemEvent
	found := false
	lookupErr := m.withBusyRetry(ctx, func() error {
		lookup := m.db.WithContext(ctx).
			Where("dedupe_key = ?", strings.TrimSpace(command.DedupeKey)).
			Limit(1).
			Find(&existing)
		found = lookup.RowsAffected > 0
		return lookup.Error
	})
	if lookupErr != nil {
		return SourceEventResult{}, lookupErr
	}
	if !found {
		return result, err
	}

	command.Actor = existing.Actor
	return m.AppendSourceEvent(ctx, command)
}
