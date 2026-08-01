package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/dataassets"
	"well-ambient/internal/db"
)

func TestDataAssetReadAPISeparatesTimelineMetadataFromGovernedPayload(t *testing.T) {
	setupServerTestDB(t)
	readerToken := superAdminToken(t, "asset-reader@example.com", "Asset Reader", []string{"data_asset:read"})
	deniedToken, err := GenerateJWT(
		"asset-denied@example.com",
		"Asset Denied",
		"mock_wellos_token",
		"",
		[]string{"member"},
		[]string{"delivery:read"},
	)
	if err != nil {
		t.Fatalf("generate denied token: %v", err)
	}
	now := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	module := dataassets.New(db.DB, dataassets.WithClock(func() time.Time { return now }))
	first, err := module.Append(context.Background(), dataassets.AppendCommand{
		DedupeKey: "api-event-1", ProjectKey: "HIT", SubjectType: "work_item", SubjectID: "HIT-501",
		EventType: "created", SourceSystem: "local", SourceRecordID: "HIT-501",
		ActorID: "owner", OccurredAt: now.Add(-time.Minute),
		Classification: dataassets.ClassificationConfidential,
		Payload:        map[string]any{"secret": "only detail may expose this", "status": "backlog"},
	})
	if err != nil {
		t.Fatalf("append first event: %v", err)
	}
	if _, err := module.Append(context.Background(), dataassets.AppendCommand{
		DedupeKey: "api-event-2", ProjectKey: "HIT", SubjectType: "work_item", SubjectID: "HIT-501",
		EventType: "status_changed", SourceSystem: "local", SourceRecordID: "HIT-501",
		ActorID: "owner", OccurredAt: now, Payload: map[string]any{"status": "progress"},
	}); err != nil {
		t.Fatalf("append second event: %v", err)
	}

	server := NewServer(&config.Config{}, "")
	doGet := func(path, token string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		server.mux.ServeHTTP(recorder, request)
		return recorder
	}

	denied := doGet("/api/data-assets/events?subject_type=work_item&subject_id=HIT-501", deniedToken)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("unauthorized data asset status = %d, want 403 body=%s", denied.Code, denied.Body.String())
	}

	list := doGet("/api/data-assets/events?subject_type=work_item&subject_id=HIT-501&limit=1", readerToken)
	if list.Code != http.StatusOK {
		t.Fatalf("timeline status = %d body=%s", list.Code, list.Body.String())
	}
	if strings.Contains(list.Body.String(), "only detail may expose this") || strings.Contains(list.Body.String(), `"payload"`) {
		t.Fatalf("timeline leaked cold payload: %s", list.Body.String())
	}
	var page dataassets.TimelinePage
	if err := json.Unmarshal(list.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode timeline: %v", err)
	}
	if len(page.Items) != 1 || page.NextCursor == "" || page.HighWatermark == 0 {
		t.Fatalf("unexpected bounded timeline: %+v", page)
	}

	detail := doGet("/api/data-assets/events/"+strconv.FormatUint(uint64(first.Event.ID), 10), readerToken)
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), "only detail may expose this") {
		t.Fatalf("detail status/body = %d %s", detail.Code, detail.Body.String())
	}

	unbounded := doGet("/api/data-assets/events", readerToken)
	if unbounded.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unbounded timeline status = %d, want 422 body=%s", unbounded.Code, unbounded.Body.String())
	}
}

func TestDataAssetSnapshotReadAPIUsesVersionedScope(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "snapshot-reader@example.com", "Snapshot Reader", []string{"data_asset:read"})
	now := time.Date(2026, 8, 1, 11, 0, 0, 0, time.UTC)
	module := dataassets.New(db.DB, dataassets.WithClock(func() time.Time { return now }))
	event, err := module.Append(context.Background(), dataassets.AppendCommand{
		DedupeKey: "snapshot-source", ProjectKey: "HIT", SubjectType: "work_item", SubjectID: "HIT-601",
		EventType: "completed", SourceSystem: "local", SourceRecordID: "HIT-601",
		ActorID: "owner", OccurredAt: now, Payload: map[string]any{"done": true},
	})
	if err != nil {
		t.Fatalf("append evidence: %v", err)
	}
	snapshot, err := module.SealSnapshot(context.Background(), dataassets.SealSnapshotCommand{
		DedupeKey: "api-weekly-HIT", Kind: "weekly_report", ScopeType: "project", ScopeID: "HIT",
		AsOf: now, InputHighWatermark: event.Event.ID, Producer: "kpi", ProducerVersion: "v1",
		CreatedBy: "reporter", EvidenceEventIDs: []uint{event.Event.ID}, Payload: map[string]any{"completed": 1},
	})
	if err != nil {
		t.Fatalf("seal snapshot: %v", err)
	}

	server := NewServer(&config.Config{}, "")
	doGet := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		server.mux.ServeHTTP(recorder, request)
		return recorder
	}

	latest := doGet("/api/data-assets/snapshots/latest?kind=weekly_report&scope_type=project&scope_id=HIT")
	if latest.Code != http.StatusOK || !strings.Contains(latest.Body.String(), `"completed":1`) {
		t.Fatalf("latest snapshot status/body = %d %s", latest.Code, latest.Body.String())
	}
	detail := doGet("/api/data-assets/snapshots/" + strconv.FormatUint(uint64(snapshot.Snapshot.ID), 10))
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"evidence_event_ids":[`+strconv.FormatUint(uint64(event.Event.ID), 10)+`]`) {
		t.Fatalf("snapshot detail status/body = %d %s", detail.Code, detail.Body.String())
	}
}
