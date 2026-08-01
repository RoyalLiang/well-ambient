package deliveryplanning

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"well-ambient/internal/dataassets"
	"well-ambient/internal/db"

	"gorm.io/gorm"
)

const (
	defaultWorkItemAssetBackfillBatch = 200
	maxWorkItemAssetBackfillBatch     = 500
	maxWorkItemAssetConflictSamples   = 100
)

type WorkItemAssetBackfillOptions struct {
	AfterID   uint `json:"after_id"`
	BatchSize int  `json:"batch_size"`
	Apply     bool `json:"apply"`
}

type WorkItemAssetBackfillReport struct {
	DryRun          bool     `json:"dry_run"`
	Scanned         int      `json:"scanned"`
	Existing        int      `json:"existing"`
	WouldAppend     int      `json:"would_append"`
	Appended        int      `json:"appended"`
	Replayed        int      `json:"replayed"`
	Conflicts       int      `json:"conflicts"`
	LastID          uint     `json:"last_id"`
	ConflictSamples []string `json:"conflict_samples,omitempty"`
}

type workItemAssetPayload struct {
	WorkItemID      string `json:"work_item_id"`
	ProjectKey      string `json:"project_key"`
	EventType       string `json:"event_type"`
	Actor           string `json:"actor"`
	Reason          string `json:"reason"`
	Before          any    `json:"before"`
	BeforeJSONValid bool   `json:"before_json_valid"`
	After           any    `json:"after"`
	AfterJSONValid  bool   `json:"after_json_valid"`
	Revision        uint   `json:"revision"`
	Source          string `json:"source"`
	SyncState       string `json:"sync_state"`
}

type releaseVersionAssetFacts struct {
	ProjectKey  string     `json:"project_key"`
	Source      string     `json:"source"`
	ExternalID  string     `json:"external_id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	StartDate   *time.Time `json:"start_date"`
	ReleaseDate *time.Time `json:"release_date"`
	SourceURL   string     `json:"source_url"`
}

type releaseVersionAssetPayload struct {
	Before *releaseVersionAssetFacts `json:"before,omitempty"`
	After  releaseVersionAssetFacts  `json:"after"`
}

func appendWorkItemAsset(ctx context.Context, tx *gorm.DB, event db.WorkItemEvent) (dataassets.AppendResult, error) {
	if event.ID == 0 {
		return dataassets.AppendResult{}, fmt.Errorf("work item event must be persisted before it becomes a data asset")
	}
	return dataassets.New(tx).Append(ctx, workItemAssetCommand(event))
}

func appendReleaseVersionAsset(ctx context.Context, tx *gorm.DB, before *db.ReleaseVersion, after db.ReleaseVersion, observedAt time.Time) (bool, error) {
	afterFacts := releaseVersionFacts(after)
	var beforeFacts *releaseVersionAssetFacts
	if before != nil {
		facts := releaseVersionFacts(*before)
		beforeFacts = &facts
		beforeJSON, _ := json.Marshal(facts)
		afterJSON, _ := json.Marshal(afterFacts)
		if string(beforeJSON) == string(afterJSON) {
			return false, nil
		}
	}
	eventType := "release_version_updated"
	if before == nil {
		eventType = "release_version_created"
	}
	version := after.UpdatedAt.UTC().Format(time.RFC3339Nano)
	_, err := dataassets.New(tx).Append(ctx, dataassets.AppendCommand{
		DedupeKey:      fmt.Sprintf("release_version:%d:%s", after.ID, version),
		ProjectKey:     after.ProjectKey,
		SubjectType:    "release_version",
		SubjectID:      strconv.FormatUint(uint64(after.ID), 10),
		EventType:      eventType,
		SourceSystem:   after.Source,
		SourceRecordID: after.ExternalID,
		SourceEventID:  version,
		ActorID:        "jira_sync",
		ActorRole:      "integration",
		CorrelationID:  fmt.Sprintf("release_version:%d", after.ID),
		Classification: dataassets.ClassificationInternal,
		RetentionClass: dataassets.RetentionLongTerm,
		SchemaVersion:  1,
		OccurredAt:     observedAt,
		ObservedAt:     observedAt,
		Payload: releaseVersionAssetPayload{
			Before: beforeFacts,
			After:  afterFacts,
		},
	})
	return err == nil, err
}

func releaseVersionFacts(row db.ReleaseVersion) releaseVersionAssetFacts {
	return releaseVersionAssetFacts{
		ProjectKey: row.ProjectKey, Source: row.Source, ExternalID: row.ExternalID,
		Name: row.Name, Description: row.Description, Status: row.Status,
		StartDate: row.StartDate, ReleaseDate: row.ReleaseDate, SourceURL: row.SourceURL,
	}
}

func workItemAssetCommand(event db.WorkItemEvent) dataassets.AppendCommand {
	source := strings.ToLower(strings.TrimSpace(event.Source))
	if source == "" {
		source = "local"
	}
	before, beforeValid := governedJSONValue(event.BeforeJSON)
	after, afterValid := governedJSONValue(event.AfterJSON)
	return dataassets.AppendCommand{
		DedupeKey:      workItemAssetDedupeKey(event.ID),
		ProjectKey:     event.ProjectKey,
		SubjectType:    "work_item",
		SubjectID:      event.WorkItemID,
		EventType:      event.EventType,
		SourceSystem:   source,
		SourceRecordID: event.WorkItemID,
		SourceEventID:  strconv.FormatUint(uint64(event.ID), 10),
		ActorID:        event.Actor,
		ActorRole:      workItemAssetActorRole(event),
		CorrelationID:  "work_item:" + event.WorkItemID,
		Classification: dataassets.ClassificationInternal,
		RetentionClass: dataassets.RetentionLongTerm,
		SchemaVersion:  1,
		OccurredAt:     event.CreatedAt,
		ObservedAt:     event.CreatedAt,
		Payload: workItemAssetPayload{
			WorkItemID:      event.WorkItemID,
			ProjectKey:      event.ProjectKey,
			EventType:       event.EventType,
			Actor:           event.Actor,
			Reason:          event.Reason,
			Before:          before,
			BeforeJSONValid: beforeValid,
			After:           after,
			AfterJSONValid:  afterValid,
			Revision:        event.Revision,
			Source:          event.Source,
			SyncState:       event.SyncState,
		},
	}
}

// BackfillWorkItemAssets streams WorkItemEvent rows by primary-key cursor. A
// dry-run never creates the additive asset schema or writes data.
func BackfillWorkItemAssets(ctx context.Context, conn *gorm.DB, options WorkItemAssetBackfillOptions) (WorkItemAssetBackfillReport, error) {
	if conn == nil {
		return WorkItemAssetBackfillReport{}, fmt.Errorf("database is not initialized")
	}
	batchSize := options.BatchSize
	if batchSize <= 0 {
		batchSize = defaultWorkItemAssetBackfillBatch
	}
	if batchSize > maxWorkItemAssetBackfillBatch {
		batchSize = maxWorkItemAssetBackfillBatch
	}
	report := WorkItemAssetBackfillReport{DryRun: !options.Apply, LastID: options.AfterID}
	assetTableExists := conn.Migrator().HasTable(&db.DataAssetEvent{})
	for {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		var events []db.WorkItemEvent
		if err := conn.WithContext(ctx).
			Where("id > ?", report.LastID).
			Order("id ASC").
			Limit(batchSize).
			Find(&events).Error; err != nil {
			return report, err
		}
		if len(events) == 0 {
			return report, nil
		}
		if !options.Apply {
			for _, event := range events {
				if !assetTableExists {
					report.WouldAppend++
					continue
				}
				inspection, err := dataassets.New(conn).InspectAppend(ctx, workItemAssetCommand(event))
				if err != nil {
					return report, err
				}
				switch {
				case inspection.Conflict:
					report.Conflicts++
					if len(report.ConflictSamples) < maxWorkItemAssetConflictSamples {
						report.ConflictSamples = append(report.ConflictSamples, workItemAssetDedupeKey(event.ID))
					}
				case inspection.Replayed:
					report.Existing++
				default:
					report.WouldAppend++
				}
			}
		} else {
			batchAppended := 0
			batchReplayed := 0
			if err := conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				for _, event := range events {
					result, err := appendWorkItemAsset(ctx, tx, event)
					if err != nil {
						return err
					}
					if result.Replayed {
						batchReplayed++
					} else {
						batchAppended++
					}
				}
				return nil
			}); err != nil {
				return report, err
			}
			report.Appended += batchAppended
			report.Replayed += batchReplayed
		}
		report.Scanned += len(events)
		report.LastID = events[len(events)-1].ID
	}
}

func workItemAssetDedupeKey(eventID uint) string {
	return fmt.Sprintf("work_item_event:%d", eventID)
}

func governedJSONValue(value string) (any, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, true
	}
	if json.Valid([]byte(trimmed)) {
		return json.RawMessage(value), true
	}
	return value, false
}

func workItemAssetActorRole(event db.WorkItemEvent) string {
	source := strings.ToLower(strings.TrimSpace(event.Source))
	actor := strings.ToLower(strings.TrimSpace(event.Actor))
	if source == "jira" || source == "gitlab" || strings.Contains(actor, "_sync") {
		return "integration"
	}
	if source == "ai" || source == "llm" || source == "model" {
		return "model"
	}
	switch actor {
	case "ai", "llm", "model", "ai_agent", "llm_agent", "model_agent", "strongest_brain":
		return "model"
	}
	if strings.HasPrefix(actor, "ai_") || strings.HasPrefix(actor, "llm_") || strings.HasPrefix(actor, "model_") {
		return "model"
	}
	return "human"
}
