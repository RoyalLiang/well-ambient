package dataassets

import (
	"context"
	"fmt"
	"testing"
	"time"
	"well-ambient/internal/db"
)

func BenchmarkTimelineKeyset25000(b *testing.B) {
	module, conn, now := openTestModule(b)
	rows := make([]db.DataAssetEvent, 0, 25000)
	for index := 0; index < cap(rows); index++ {
		occurred := now.Add(-time.Duration(index) * time.Second)
		rows = append(rows, db.DataAssetEvent{
			DedupeKey: fmt.Sprintf("benchmark-%d", index), Fingerprint: fmt.Sprintf("%064d", index),
			ProjectKey: "HIT", SubjectType: "work_item", SubjectID: fmt.Sprintf("HIT-%d", index%500),
			EventType: "observed", SourceSystem: "benchmark", SourceRecordID: fmt.Sprintf("record-%d", index),
			ActorID: "benchmark", ActorRole: "system", Classification: ClassificationInternal,
			RetentionClass: RetentionStandard, SchemaVersion: 1, OccurredAt: occurred,
			ObservedAt: occurred, RecordedAt: occurred, PayloadHash: fmt.Sprintf("%064d", index),
			PayloadEncoding: "identity", PayloadBytes: 2, StoredBytes: 2,
		})
	}
	if err := conn.CreateInBatches(rows, 400).Error; err != nil {
		b.Fatalf("seed benchmark: %v", err)
	}
	query := TimelineQuery{ProjectKey: "HIT", Limit: 100}
	_, filterHash, err := module.normalizeTimeline(query)
	if err != nil {
		b.Fatalf("normalize benchmark query: %v", err)
	}
	deepRow := rows[19999]
	query.Cursor, err = encodeTimelineCursor(timelineCursor{
		Version:       1,
		OccurredAt:    deepRow.OccurredAt,
		ID:            deepRow.ID,
		HighWatermark: rows[len(rows)-1].ID,
		FilterHash:    filterHash,
	})
	if err != nil {
		b.Fatalf("encode benchmark cursor: %v", err)
	}
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, err := module.Timeline(context.Background(), query); err != nil {
			b.Fatalf("timeline: %v", err)
		}
	}
}
