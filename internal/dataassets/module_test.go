package dataassets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"well-ambient/internal/db"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var testDBSequence atomic.Uint64

func openTestModule(t testing.TB) (*Module, *gorm.DB, *time.Time) {
	t.Helper()
	dsn := fmt.Sprintf(
		"file:data-assets-%s-%d?mode=memory&cache=shared",
		strings.ReplaceAll(t.Name(), "/", "_"),
		testDBSequence.Add(1),
	)
	conn, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.MigrateDataAssets(conn); err != nil {
		t.Fatalf("migrate data assets: %v", err)
	}
	now := time.Date(2026, 8, 1, 9, 30, 0, 0, time.UTC)
	return New(conn, WithClock(func() time.Time { return now })), conn, &now
}

func TestAppendIsIdempotentCompressesAndVerifiesPayload(t *testing.T) {
	module, conn, now := openTestModule(t)
	largeBody := strings.Repeat("可追溯证据-", 1200)
	command := AppendCommand{
		DedupeKey:      "jira:HIT-101:update-7",
		ProjectKey:     "hit",
		SubjectType:    "Work Item",
		SubjectID:      "HIT-101",
		EventType:      "Status Changed",
		SourceSystem:   "Jira",
		SourceRecordID: "HIT-101",
		SourceEventID:  "update-7",
		ActorID:        "alice",
		ActorRole:      "human",
		OccurredAt:     now.Add(-time.Minute),
		ObservedAt:     *now,
		Payload: map[string]any{
			"before": "progress",
			"after":  "review",
			"body":   largeBody,
		},
	}

	first, err := module.Append(context.Background(), command)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if first.Replayed || first.Event.ID == 0 {
		t.Fatalf("unexpected first append result: %+v", first)
	}
	if first.Event.ProjectKey != "HIT" || first.Event.SubjectType != "work_item" || first.Event.EventType != "status_changed" {
		t.Fatalf("event was not normalized: %+v", first.Event)
	}
	if first.Event.PayloadEncoding != "gzip" || first.Event.StoredBytes >= first.Event.PayloadBytes {
		t.Fatalf("large payload was not compressed: %+v", first.Event)
	}

	replayed, err := module.Append(context.Background(), command)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !replayed.Replayed || replayed.Event.ID != first.Event.ID {
		t.Fatalf("replay should return existing event: %+v", replayed)
	}

	changed := command
	changed.Payload = map[string]any{"before": "progress", "after": "done"}
	if _, err := module.Append(context.Background(), changed); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed replay error = %v, want idempotency conflict", err)
	}

	record, err := module.Load(context.Background(), first.Event.ID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(record.Payload, &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if decoded["body"] != largeBody || decoded["after"] != "review" {
		t.Fatalf("loaded payload changed: %#v", decoded)
	}

	var eventCount, payloadCount int64
	conn.Model(&db.DataAssetEvent{}).Count(&eventCount)
	conn.Model(&db.DataAssetEventPayload{}).Count(&payloadCount)
	if eventCount != 1 || payloadCount != 1 {
		t.Fatalf("idempotent append counts = events %d payloads %d", eventCount, payloadCount)
	}
}

func TestAppendRetryWithoutObservedAtRemainsIdempotentAfterClockAdvances(t *testing.T) {
	module, _, now := openTestModule(t)
	command := AppendCommand{
		DedupeKey: "clock-stable-replay", SubjectType: "work_item", SubjectID: "HIT-102",
		EventType: "created", SourceSystem: "local", SourceRecordID: "HIT-102",
		ActorID: "creator", OccurredAt: now.Add(-time.Minute), Payload: map[string]any{"status": "ready"},
	}
	first, err := module.Append(context.Background(), command)
	if err != nil {
		t.Fatalf("first append: %v", err)
	}
	firstObservedAt := first.Event.ObservedAt
	*now = now.Add(30 * time.Minute)
	replayed, err := module.Append(context.Background(), command)
	if err != nil {
		t.Fatalf("clock-delayed replay: %v", err)
	}
	if !replayed.Replayed || replayed.Event.ID != first.Event.ID || !replayed.Event.ObservedAt.Equal(firstObservedAt) {
		t.Fatalf("clock-delayed replay changed identity or observation: first=%+v replay=%+v", first, replayed)
	}
}

func TestTimelineUsesStableHighWatermarkAndRejectsCursorFilterChanges(t *testing.T) {
	module, _, now := openTestModule(t)
	ctx := context.Background()
	initialIDs := make([]uint, 0, 5)
	for index := 0; index < 5; index++ {
		result := appendEvent(t, module, AppendCommand{
			DedupeKey:      fmt.Sprintf("work-item:HIT-202:event-%d", index),
			ProjectKey:     "HIT",
			SubjectType:    "work_item",
			SubjectID:      "HIT-202",
			EventType:      "planning_changed",
			SourceSystem:   "local",
			SourceRecordID: "HIT-202",
			ActorID:        "planner",
			OccurredAt:     now.Add(time.Duration(index/2) * time.Minute),
			ObservedAt:     *now,
			Payload:        map[string]any{"sequence": index},
		})
		initialIDs = append(initialIDs, result.Event.ID)
	}

	first, err := module.Timeline(ctx, TimelineQuery{
		SubjectType: "work_item",
		SubjectID:   "HIT-202",
		Limit:       2,
	})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(first.Items) != 2 || first.NextCursor == "" || first.HighWatermark != initialIDs[4] {
		t.Fatalf("unexpected first page: %+v", first)
	}
	// Events 3 and 4 share a timestamp; the larger ID must sort first.
	if first.Items[0].ID != initialIDs[4] || first.Items[1].ID != initialIDs[3] {
		t.Fatalf("unexpected descending order: %+v", first.Items)
	}

	late := appendEvent(t, module, AppendCommand{
		DedupeKey:      "work-item:HIT-202:late-event",
		ProjectKey:     "HIT",
		SubjectType:    "work_item",
		SubjectID:      "HIT-202",
		EventType:      "comment_added",
		SourceSystem:   "jira",
		SourceRecordID: "HIT-202",
		ActorID:        "reviewer",
		OccurredAt:     now.Add(90 * time.Second),
		ObservedAt:     now.Add(10 * time.Minute),
		Payload:        map[string]any{"late": true},
	})

	seen := []uint{first.Items[0].ID, first.Items[1].ID}
	cursor := first.NextCursor
	for cursor != "" {
		page, err := module.Timeline(ctx, TimelineQuery{
			SubjectType: "work_item",
			SubjectID:   "HIT-202",
			Limit:       2,
			Cursor:      cursor,
		})
		if err != nil {
			t.Fatalf("next page: %v", err)
		}
		for _, item := range page.Items {
			seen = append(seen, item.ID)
		}
		cursor = page.NextCursor
	}
	if len(seen) != len(initialIDs) {
		t.Fatalf("stable timeline returned %d ids, want %d: %v", len(seen), len(initialIDs), seen)
	}
	for _, id := range seen {
		if id == late.Event.ID {
			t.Fatalf("late event %d leaked into fixed high-watermark timeline", id)
		}
	}

	refreshed, err := module.Timeline(ctx, TimelineQuery{SubjectType: "work_item", SubjectID: "HIT-202", Limit: 10})
	if err != nil {
		t.Fatalf("refreshed timeline: %v", err)
	}
	if refreshed.HighWatermark != late.Event.ID || !containsEvent(refreshed.Items, late.Event.ID) {
		t.Fatalf("new timeline did not include late event: %+v", refreshed)
	}

	if _, err := module.Timeline(ctx, TimelineQuery{
		SubjectType: "work_item",
		SubjectID:   "OTHER-1",
		Limit:       2,
		Cursor:      first.NextCursor,
	}); !errors.Is(err, ErrCursorFilterMismatch) {
		t.Fatalf("cursor filter change error = %v", err)
	}
	if _, err := module.Timeline(ctx, TimelineQuery{}); !errors.Is(err, ErrQueryScopeRequired) {
		t.Fatalf("unbounded query error = %v", err)
	}
	if _, err := module.Timeline(ctx, TimelineQuery{SourceRecordID: "HIT-202"}); !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("source record without system error = %v", err)
	}
}

func TestSealSnapshotPreservesEvidenceAndLatestVersion(t *testing.T) {
	module, _, now := openTestModule(t)
	evidenceIDs := make([]uint, 0, 3)
	for index := 0; index < 3; index++ {
		result := appendEvent(t, module, AppendCommand{
			DedupeKey:      fmt.Sprintf("metric-source-%d", index),
			ProjectKey:     "HIT",
			SubjectType:    "work_item",
			SubjectID:      fmt.Sprintf("HIT-%d", 300+index),
			EventType:      "completed",
			SourceSystem:   "local",
			SourceRecordID: fmt.Sprintf("HIT-%d", 300+index),
			ActorID:        "system",
			OccurredAt:     now.Add(time.Duration(index) * time.Minute),
			Payload:        map[string]any{"done": true},
		})
		evidenceIDs = append(evidenceIDs, result.Event.ID)
	}

	command := SealSnapshotCommand{
		DedupeKey:          "weekly:HIT:2026-W31:v1",
		Kind:               "weekly report",
		ScopeType:          "project",
		ScopeID:            "HIT",
		AsOf:               now.Add(time.Hour),
		InputHighWatermark: evidenceIDs[2],
		Producer:           "deterministic KPI",
		ProducerVersion:    "kpi-v1",
		CreatedBy:          "reporter",
		EvidenceEventIDs:   []uint{evidenceIDs[0], evidenceIDs[1], evidenceIDs[1], evidenceIDs[2]},
		Payload:            map[string]any{"completed": 3, "delay_ratio": 0},
	}
	first, err := module.SealSnapshot(context.Background(), command)
	if err != nil {
		t.Fatalf("seal snapshot: %v", err)
	}
	if first.Snapshot.EvidenceCount != 3 || first.Snapshot.Kind != "weekly_report" {
		t.Fatalf("unexpected snapshot metadata: %+v", first.Snapshot)
	}
	replayed, err := module.SealSnapshot(context.Background(), command)
	if err != nil || !replayed.Replayed || replayed.Snapshot.ID != first.Snapshot.ID {
		t.Fatalf("snapshot replay = %+v, err %v", replayed, err)
	}

	record, err := module.LoadSnapshot(context.Background(), first.Snapshot.ID)
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if fmt.Sprint(record.EvidenceEventIDs) != fmt.Sprint(evidenceIDs) {
		t.Fatalf("snapshot evidence = %v, want %v", record.EvidenceEventIDs, evidenceIDs)
	}
	var report map[string]any
	if err := json.Unmarshal(record.Payload, &report); err != nil || report["completed"] != float64(3) {
		t.Fatalf("snapshot payload = %s, err %v", record.Payload, err)
	}

	secondCommand := command
	secondCommand.DedupeKey = "weekly:HIT:2026-W32:v1"
	secondCommand.AsOf = command.AsOf.Add(7 * 24 * time.Hour)
	secondCommand.Payload = map[string]any{"completed": 5, "delay_ratio": 10}
	second, err := module.SealSnapshot(context.Background(), secondCommand)
	if err != nil {
		t.Fatalf("seal second snapshot: %v", err)
	}
	latest, err := module.LatestSnapshot(context.Background(), LatestSnapshotQuery{
		Kind: "weekly_report", ScopeType: "project", ScopeID: "HIT",
	})
	if err != nil || latest.ID != second.Snapshot.ID {
		t.Fatalf("latest snapshot = %+v, err %v", latest, err)
	}
	beforeSecond := secondCommand.AsOf.Add(-time.Second)
	previous, err := module.LatestSnapshot(context.Background(), LatestSnapshotQuery{
		Kind: "weekly_report", ScopeType: "project", ScopeID: "HIT", AsOf: &beforeSecond,
	})
	if err != nil || previous.ID != first.Snapshot.ID {
		t.Fatalf("historical latest snapshot = %+v, err %v", previous, err)
	}

	conflict := command
	conflict.Payload = map[string]any{"completed": 999}
	if _, err := module.SealSnapshot(context.Background(), conflict); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("snapshot conflict error = %v", err)
	}
	invalidWatermark := command
	invalidWatermark.DedupeKey = "weekly:HIT:invalid-watermark"
	invalidWatermark.InputHighWatermark = 9999
	if _, err := module.SealSnapshot(context.Background(), invalidWatermark); !errors.Is(err, ErrInputWatermark) {
		t.Fatalf("invalid watermark error = %v", err)
	}
	futureEvidence := command
	futureEvidence.DedupeKey = "weekly:HIT:future-evidence"
	futureEvidence.AsOf = now.Add(-time.Second)
	if _, err := module.SealSnapshot(context.Background(), futureEvidence); !errors.Is(err, ErrEvidenceAfterAsOf) {
		t.Fatalf("future evidence error = %v", err)
	}
}

func TestDatabaseGuardsRejectRawMutationAndRequiredIndexesExist(t *testing.T) {
	module, conn, now := openTestModule(t)
	result := appendEvent(t, module, AppendCommand{
		DedupeKey:      "immutable-event",
		SubjectType:    "work_item",
		SubjectID:      "HIT-404",
		EventType:      "created",
		SourceSystem:   "local",
		SourceRecordID: "HIT-404",
		ActorID:        "creator",
		OccurredAt:     *now,
		Payload:        map[string]any{"title": "immutable"},
	})
	if err := conn.Exec("UPDATE data_asset_events SET actor_id = ? WHERE id = ?", "tampered", result.Event.ID).Error; err == nil {
		t.Fatal("raw event update unexpectedly succeeded")
	}
	if err := conn.Exec("DELETE FROM data_asset_event_payloads WHERE event_id = ?", result.Event.ID).Error; err == nil {
		t.Fatal("raw payload delete unexpectedly succeeded")
	}
	if err := conn.Exec(`INSERT OR REPLACE INTO data_asset_event_payloads
		(event_id, content_type, encoding, data, original_size, stored_size, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, result.Event.ID, "application/json", "identity", []byte(`{"tampered":true}`), 17, 17, *now).Error; err == nil {
		t.Fatal("raw payload replace unexpectedly succeeded")
	}

	for _, indexName := range []string{
		"idx_data_asset_occurred_time",
		"idx_data_asset_subject_time",
		"idx_data_asset_project_time",
		"idx_data_asset_source_time",
		"idx_data_asset_source_record_time",
		"idx_data_asset_event_type_time",
		"idx_data_asset_correlation_time",
		"idx_data_asset_recorded_watermark",
		"idx_data_asset_retention_expiry",
		"idx_data_asset_snapshot_scope_time",
		"idx_data_asset_snapshot_evidence_event",
	} {
		var count int64
		if err := conn.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?", indexName).Scan(&count).Error; err != nil {
			t.Fatalf("query index %s: %v", indexName, err)
		}
		if count != 1 {
			t.Fatalf("missing performance index %s", indexName)
		}
	}

	var plan []struct{ Detail string }
	if err := conn.Raw(`EXPLAIN QUERY PLAN
		SELECT id, subject_type, subject_id, occurred_at
		FROM data_asset_events
		WHERE subject_type = ? AND subject_id = ? AND id <= ?
		ORDER BY occurred_at DESC, id DESC
		LIMIT ?`, "work_item", "HIT-404", result.Event.ID, 100).Scan(&plan).Error; err != nil {
		t.Fatalf("explain timeline: %v", err)
	}
	details := fmt.Sprint(plan)
	if !strings.Contains(details, "idx_data_asset_subject_time") {
		t.Fatalf("timeline plan did not use subject-time index: %s", details)
	}
	if strings.Contains(details, "payload") {
		t.Fatalf("timeline query touched cold payload storage: %s", details)
	}
	plan = nil
	if err := conn.Raw(`EXPLAIN QUERY PLAN
		SELECT id, occurred_at FROM data_asset_events
		WHERE occurred_at >= ? ORDER BY occurred_at DESC, id DESC LIMIT ?`, now.Add(-time.Hour), 100).Scan(&plan).Error; err != nil {
		t.Fatalf("explain time timeline: %v", err)
	}
	if details = fmt.Sprint(plan); !strings.Contains(details, "idx_data_asset_occurred_time") || strings.Contains(details, "TEMP B-TREE") {
		t.Fatalf("time timeline plan is not index ordered: %s", details)
	}
	plan = nil
	if err := conn.Raw(`EXPLAIN QUERY PLAN
		SELECT id, occurred_at FROM data_asset_events
		WHERE source_system = ? ORDER BY occurred_at DESC, id DESC LIMIT ?`, "local", 100).Scan(&plan).Error; err != nil {
		t.Fatalf("explain source timeline: %v", err)
	}
	if details = fmt.Sprint(plan); !strings.Contains(details, "idx_data_asset_source_time") || strings.Contains(details, "TEMP B-TREE") {
		t.Fatalf("source timeline plan is not index ordered: %s", details)
	}
}

func TestTimelineReturnsBoundedPageAcrossLargeMetadataSet(t *testing.T) {
	module, conn, now := openTestModule(t)
	const rowCount = 12000
	rows := make([]db.DataAssetEvent, 0, rowCount)
	for index := 0; index < rowCount; index++ {
		occurred := now.Add(-time.Duration(index) * time.Second)
		rows = append(rows, db.DataAssetEvent{
			DedupeKey:       fmt.Sprintf("bulk-%d", index),
			Fingerprint:     fmt.Sprintf("%064d", index),
			ProjectKey:      "HIT",
			SubjectType:     "work_item",
			SubjectID:       fmt.Sprintf("HIT-%d", index%200),
			EventType:       "bulk_observed",
			SourceSystem:    "fixture",
			SourceRecordID:  fmt.Sprintf("record-%d", index),
			ActorID:         "fixture",
			ActorRole:       "system",
			Classification:  ClassificationInternal,
			RetentionClass:  RetentionStandard,
			SchemaVersion:   1,
			OccurredAt:      occurred,
			ObservedAt:      occurred,
			RecordedAt:      occurred,
			PayloadHash:     fmt.Sprintf("%064d", index),
			PayloadEncoding: "identity",
			PayloadBytes:    2,
			StoredBytes:     2,
		})
	}
	if err := conn.CreateInBatches(rows, 400).Error; err != nil {
		t.Fatalf("seed large metadata set: %v", err)
	}

	page, err := module.Timeline(context.Background(), TimelineQuery{ProjectKey: "HIT", Limit: 1000})
	if err != nil {
		t.Fatalf("large timeline: %v", err)
	}
	if len(page.Items) != defaultMaxTimelineLimit || page.NextCursor == "" {
		t.Fatalf("large timeline was not bounded: items=%d cursor=%q", len(page.Items), page.NextCursor)
	}
	var payloadCount int64
	conn.Model(&db.DataAssetEventPayload{}).Count(&payloadCount)
	if payloadCount != 0 {
		t.Fatalf("large metadata timeline unexpectedly required payload rows: %d", payloadCount)
	}
}

func appendEvent(t *testing.T, module *Module, command AppendCommand) AppendResult {
	t.Helper()
	result, err := module.Append(context.Background(), command)
	if err != nil {
		t.Fatalf("append event %q: %v", command.DedupeKey, err)
	}
	return result
}

func containsEvent(items []Event, eventID uint) bool {
	for _, item := range items {
		if item.ID == eventID {
			return true
		}
	}
	return false
}
