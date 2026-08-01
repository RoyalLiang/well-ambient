package deliveryplanning

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
	"well-ambient/internal/db"
)

func TestBackfillWorkItemAssetsIsCursorBatchedDryRunAndIdempotent(t *testing.T) {
	conn := openPlanningTestDB(t)
	const eventCount = 1201
	events := make([]db.WorkItemEvent, 0, eventCount)
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	for index := 0; index < eventCount; index++ {
		events = append(events, db.WorkItemEvent{
			WorkItemID: fmt.Sprintf("HIT-%d", index%250),
			ProjectKey: "HIT",
			EventType:  "planning_state_changed",
			Actor:      "migration-fixture",
			Reason:     "historical event",
			BeforeJSON: `{"planning_state":"ready"}`,
			AfterJSON:  `{"planning_state":"planned"}`,
			Revision:   uint(index + 1),
			Source:     "manual",
			SyncState:  "not_required",
			CreatedAt:  base.Add(time.Duration(index) * time.Second),
		})
	}
	if err := conn.CreateInBatches(events, 300).Error; err != nil {
		t.Fatalf("seed work item events: %v", err)
	}

	dryRun, err := BackfillWorkItemAssets(context.Background(), conn, WorkItemAssetBackfillOptions{BatchSize: 173})
	if err != nil {
		t.Fatalf("dry-run backfill: %v", err)
	}
	if !dryRun.DryRun || dryRun.Scanned != eventCount || dryRun.WouldAppend != eventCount || dryRun.Appended != 0 || dryRun.LastID != events[len(events)-1].ID {
		t.Fatalf("unexpected dry-run report: %+v", dryRun)
	}
	var assetCount int64
	conn.Model(&db.DataAssetEvent{}).Count(&assetCount)
	if assetCount != 0 {
		t.Fatalf("dry-run wrote %d asset events", assetCount)
	}

	applied, err := BackfillWorkItemAssets(context.Background(), conn, WorkItemAssetBackfillOptions{BatchSize: 173, Apply: true})
	if err != nil {
		t.Fatalf("apply backfill: %v", err)
	}
	if applied.DryRun || applied.Appended != eventCount || applied.Replayed != 0 || applied.Scanned != eventCount {
		t.Fatalf("unexpected applied report: %+v", applied)
	}
	conn.Model(&db.DataAssetEvent{}).Count(&assetCount)
	if assetCount != eventCount {
		t.Fatalf("asset count = %d, want %d", assetCount, eventCount)
	}

	replayed, err := BackfillWorkItemAssets(context.Background(), conn, WorkItemAssetBackfillOptions{BatchSize: 500, Apply: true})
	if err != nil {
		t.Fatalf("replay backfill: %v", err)
	}
	if replayed.Appended != 0 || replayed.Replayed != eventCount {
		t.Fatalf("backfill replay was not idempotent: %+v", replayed)
	}
	if err := conn.Model(&db.WorkItemEvent{}).Where("id = ?", events[0].ID).Update("reason", "historical content changed").Error; err != nil {
		t.Fatalf("change historical source fixture: %v", err)
	}
	conflicted, err := BackfillWorkItemAssets(context.Background(), conn, WorkItemAssetBackfillOptions{BatchSize: 211})
	if err != nil {
		t.Fatalf("conflict-aware dry-run: %v", err)
	}
	if conflicted.Conflicts != 1 || conflicted.Existing != eventCount-1 || len(conflicted.ConflictSamples) != 1 || conflicted.ConflictSamples[0] != workItemAssetDedupeKey(events[0].ID) {
		t.Fatalf("dry-run did not expose historical conflict: %+v", conflicted)
	}

	partial, err := BackfillWorkItemAssets(context.Background(), conn, WorkItemAssetBackfillOptions{
		AfterID: events[600].ID, BatchSize: 100,
	})
	if err != nil {
		t.Fatalf("partial dry-run: %v", err)
	}
	if partial.Scanned != eventCount-601 || partial.Existing != eventCount-601 || partial.WouldAppend != 0 {
		t.Fatalf("partial report ignored cursor/existing assets: %+v", partial)
	}
}

func TestBackfillPreservesInvalidHistoricalJSONAsGovernedEvidence(t *testing.T) {
	conn := openPlanningTestDB(t)
	event := db.WorkItemEvent{
		WorkItemID: "HIT-INVALID-JSON", ProjectKey: "HIT", EventType: "legacy_changed",
		Actor: "legacy", Reason: "preserve malformed evidence", BeforeJSON: `{"broken":`, AfterJSON: `{"ok":true}`,
		Revision: 1, Source: "manual", SyncState: "not_required", CreatedAt: time.Now().UTC(),
	}
	if err := conn.Create(&event).Error; err != nil {
		t.Fatalf("create legacy event: %v", err)
	}
	if _, err := BackfillWorkItemAssets(context.Background(), conn, WorkItemAssetBackfillOptions{Apply: true}); err != nil {
		t.Fatalf("backfill malformed evidence: %v", err)
	}
	var payload db.DataAssetEventPayload
	if err := conn.Where("event_id = ?", 1).First(&payload).Error; err != nil {
		t.Fatalf("load governed payload: %v", err)
	}
	var decoded struct {
		Before          string `json:"before"`
		BeforeJSONValid bool   `json:"before_json_valid"`
		AfterJSONValid  bool   `json:"after_json_valid"`
	}
	if err := json.Unmarshal(payload.Data, &decoded); err != nil {
		t.Fatalf("decode governed payload: %v", err)
	}
	if decoded.Before != event.BeforeJSON || decoded.BeforeJSONValid || !decoded.AfterJSONValid {
		t.Fatalf("malformed evidence was not preserved exactly: %+v", decoded)
	}
}

func TestWorkItemAssetActorRoleUsesDomainTokensInsteadOfNameFragments(t *testing.T) {
	if got := workItemAssetActorRole(db.WorkItemEvent{Actor: "Kai", Source: "manual"}); got != "human" {
		t.Fatalf("human name fragment was misclassified as %q", got)
	}
	if got := workItemAssetActorRole(db.WorkItemEvent{Actor: "ai_planner", Source: "manual"}); got != "model" {
		t.Fatalf("known model actor was classified as %q", got)
	}
	if got := workItemAssetActorRole(db.WorkItemEvent{Actor: "jira_sync", Source: "jira"}); got != "integration" {
		t.Fatalf("integration actor was classified as %q", got)
	}
}
