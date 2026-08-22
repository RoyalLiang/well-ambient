package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/deliveryplanning"
)

func TestPublishReleasePersistsEvidenceAndMakesItVisibleToStrongestBrain(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "release-manager@example.com", "Release Manager", []string{
		"release:manage",
		"delivery:read",
		"decision:read",
	})
	if err := db.DB.Create(&db.ProjectConfig{ProjectKey: "FMS", ProjectName: "FMS"}).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	release := db.ReleaseVersion{
		ProjectKey: "FMS",
		Source:     "local",
		ExternalID: "local-publish-contract",
		Name:       "FMS 5.4.0",
		Status:     deliveryplanning.ReleasePlanned,
	}
	if err := db.DB.Create(&release).Error; err != nil {
		t.Fatalf("create release: %v", err)
	}
	workItem := db.TaskTelemetry{
		TaskID:        "FMS-540",
		ExternalKey:   "FMS-540",
		Source:        "jira",
		ProjectKey:    "FMS",
		IssueType:     "requirement",
		Title:         "Publish FMS 5.4.0",
		Status:        "done",
		PlanningState: deliveryplanning.PlanningDone,
		LastUpdate:    time.Now(),
	}
	if err := db.DB.Create(&workItem).Error; err != nil {
		t.Fatalf("create work item: %v", err)
	}
	if err := db.DB.Create(&db.WorkItemReleaseLink{
		WorkItemID:       workItem.TaskID,
		ReleaseVersionID: release.ID,
		Relation:         deliveryplanning.ReleaseTargetFix,
		IsPrimary:        true,
		Active:           true,
		Source:           "manual",
	}).Error; err != nil {
		t.Fatalf("create release link: %v", err)
	}

	server := NewServer(&config.Config{}, "")
	bypassRequest := httptest.NewRequest(
		http.MethodPatch,
		"/api/releases/"+jsonNumber(release.ID),
		strings.NewReader(`{"status":"released","release_date":"2026-08-12"}`),
	)
	bypassRequest.Header.Set("Authorization", "Bearer "+token)
	bypassRequest.Header.Set("Content-Type", "application/json")
	bypassRecorder := httptest.NewRecorder()
	server.mux.ServeHTTP(bypassRecorder, bypassRequest)
	if bypassRecorder.Code != http.StatusConflict || !strings.Contains(bypassRecorder.Body.String(), "use_release_publish_endpoint") {
		t.Fatalf("generic release bypass status/body = %d/%s", bypassRecorder.Code, bypassRecorder.Body.String())
	}

	publish := func() *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(
			http.MethodPost,
			"/api/releases/"+jsonNumber(release.ID)+"/publish",
			strings.NewReader(`{"release_date":"2026-08-12","reason":"验收通过，发布范围已核对"}`),
		)
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		server.mux.ServeHTTP(recorder, request)
		return recorder
	}

	first := publish()
	if first.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body=%s", first.Code, first.Body.String())
	}
	var publishedPayload struct {
		Release     db.ReleaseVersion `json:"release"`
		EvidenceRef string            `json:"evidence_ref"`
		Replayed    bool              `json:"replayed"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &publishedPayload); err != nil {
		t.Fatalf("decode publish response: %v", err)
	}
	if publishedPayload.Release.Status != deliveryplanning.ReleaseReleased || publishedPayload.Release.ReleaseDate == nil {
		t.Fatalf("published release = %+v", publishedPayload.Release)
	}
	if publishedPayload.Release.ReleaseDate.Format("2006-01-02") != "2026-08-12" {
		t.Fatalf("release date = %s, want 2026-08-12", publishedPayload.Release.ReleaseDate.Format("2006-01-02"))
	}
	if !strings.HasPrefix(publishedPayload.EvidenceRef, "data_asset_event:") || publishedPayload.Replayed {
		t.Fatalf("publish evidence = %q replayed=%v", publishedPayload.EvidenceRef, publishedPayload.Replayed)
	}

	var assetEvents []db.DataAssetEvent
	if err := db.DB.Where(
		"subject_type = ? AND subject_id = ? AND event_type = ?",
		"release_version",
		jsonNumber(release.ID),
		"release_version_published",
	).Find(&assetEvents).Error; err != nil {
		t.Fatalf("query release evidence: %v", err)
	}
	if len(assetEvents) != 1 || assetEvents[0].ActorID != "release-manager@example.com" {
		t.Fatalf("release evidence = %+v", assetEvents)
	}

	lockedRequests := []struct {
		method string
		path   string
		body   string
	}{
		{
			method: http.MethodPatch,
			path:   "/api/releases/" + jsonNumber(release.ID),
			body:   `{"name":"Changed after publish"}`,
		},
		{
			method: http.MethodPost,
			path:   "/api/releases/" + jsonNumber(release.ID) + "/jira-issues/bulk",
			body:   `{"work_item_ids":["FMS-540"],"reason":"change published scope"}`,
		},
		{
			method: http.MethodDelete,
			path:   "/api/releases/" + jsonNumber(release.ID) + "/jira-issues/FMS-540",
		},
	}
	for _, lockedRequest := range lockedRequests {
		request := httptest.NewRequest(lockedRequest.method, lockedRequest.path, strings.NewReader(lockedRequest.body))
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		server.mux.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "release_scope_locked") {
			t.Fatalf("locked release mutation %s %s status/body = %d/%s", lockedRequest.method, lockedRequest.path, recorder.Code, recorder.Body.String())
		}
	}

	retry := publish()
	if retry.Code != http.StatusOK {
		t.Fatalf("publish retry status = %d, body=%s", retry.Code, retry.Body.String())
	}
	var retryPayload struct {
		EvidenceRef string `json:"evidence_ref"`
		Replayed    bool   `json:"replayed"`
	}
	if err := json.Unmarshal(retry.Body.Bytes(), &retryPayload); err != nil {
		t.Fatalf("decode publish retry: %v", err)
	}
	if !retryPayload.Replayed || retryPayload.EvidenceRef != publishedPayload.EvidenceRef {
		t.Fatalf("publish retry = %+v, first evidence=%q", retryPayload, publishedPayload.EvidenceRef)
	}
	var assetCount int64
	if err := db.DB.Model(&db.DataAssetEvent{}).
		Where("subject_type = ? AND subject_id = ? AND event_type = ?", "release_version", jsonNumber(release.ID), "release_version_published").
		Count(&assetCount).Error; err != nil {
		t.Fatalf("count release evidence: %v", err)
	}
	if assetCount != 1 {
		t.Fatalf("release evidence count = %d, want 1", assetCount)
	}

	cockpitRequest := httptest.NewRequest(http.MethodGet, "/api/strongest-brain/delivery-cockpit", nil)
	cockpitRequest.Header.Set("Authorization", "Bearer "+token)
	cockpitRecorder := httptest.NewRecorder()
	server.mux.ServeHTTP(cockpitRecorder, cockpitRequest)
	if cockpitRecorder.Code != http.StatusOK {
		t.Fatalf("strongest-brain status = %d, body=%s", cockpitRecorder.Code, cockpitRecorder.Body.String())
	}
	var cockpit struct {
		Releases struct {
			Total    int `json:"total"`
			Released int `json:"released"`
			Recent   []struct {
				ID            uint     `json:"id"`
				Name          string   `json:"name"`
				Status        string   `json:"status"`
				WorkItemCount int64    `json:"work_item_count"`
				EvidenceRefs  []string `json:"evidence_refs"`
			} `json:"recent"`
		} `json:"releases"`
	}
	if err := json.Unmarshal(cockpitRecorder.Body.Bytes(), &cockpit); err != nil {
		t.Fatalf("decode strongest-brain response: %v", err)
	}
	if cockpit.Releases.Total != 1 || cockpit.Releases.Released != 1 || len(cockpit.Releases.Recent) != 1 {
		t.Fatalf("strongest-brain releases = %+v", cockpit.Releases)
	}
	recent := cockpit.Releases.Recent[0]
	if recent.ID != release.ID || recent.Name != release.Name || recent.Status != deliveryplanning.ReleaseReleased || recent.WorkItemCount != 1 {
		t.Fatalf("strongest-brain recent release = %+v", recent)
	}
	if len(recent.EvidenceRefs) != 1 || recent.EvidenceRefs[0] != publishedPayload.EvidenceRef {
		t.Fatalf("strongest-brain evidence refs = %#v, want %q", recent.EvidenceRefs, publishedPayload.EvidenceRef)
	}
}

func TestPublishReleaseRequiresBoundLocalPlannedRelease(t *testing.T) {
	setupServerTestDB(t)
	service := deliveryplanning.NewService(db.DB)
	date := time.Date(2026, time.August, 12, 0, 0, 0, 0, time.UTC)
	command := func(releaseID uint) deliveryplanning.PublishReleaseCommand {
		return deliveryplanning.PublishReleaseCommand{
			ReleaseID:   releaseID,
			ReleaseDate: date,
			Actor:       "release-manager@example.com",
			Reason:      "publish contract",
		}
	}

	cases := []struct {
		name       string
		release    db.ReleaseVersion
		wantCode   string
		wantStatus int
	}{
		{
			name:       "project is required",
			release:    db.ReleaseVersion{Source: "local", ExternalID: "local-unbound", Name: "Unbound", Status: deliveryplanning.ReleasePlanned},
			wantCode:   "release_project_required",
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "external releases are read only",
			release:    db.ReleaseVersion{ProjectKey: "FMS", Source: "jira", ExternalID: "jira-7", Name: "Jira 7", Status: deliveryplanning.ReleasePlanned},
			wantCode:   "external_release_read_only",
			wantStatus: http.StatusConflict,
		},
		{
			name:       "archived releases stay closed",
			release:    db.ReleaseVersion{ProjectKey: "FMS", Source: "local", ExternalID: "local-archived", Name: "Archived", Status: deliveryplanning.ReleaseArchived},
			wantCode:   "invalid_release_transition",
			wantStatus: http.StatusConflict,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := db.DB.Create(&tc.release).Error; err != nil {
				t.Fatalf("create release: %v", err)
			}
			_, err := service.PublishRelease(t.Context(), command(tc.release.ID))
			if deliveryplanning.ErrorCode(err) != tc.wantCode {
				t.Fatalf("publish error = %v (%q), want %q", err, deliveryplanning.ErrorCode(err), tc.wantCode)
			}
			var domainErr *deliveryplanning.DomainError
			if !errors.As(err, &domainErr) || domainErr.StatusCode != tc.wantStatus {
				t.Fatalf("publish domain error = %#v, want status %d", domainErr, tc.wantStatus)
			}
			var stored db.ReleaseVersion
			if err := db.DB.First(&stored, tc.release.ID).Error; err != nil {
				t.Fatalf("reload release: %v", err)
			}
			if stored.Status != tc.release.Status {
				t.Fatalf("release status = %q, want unchanged %q", stored.Status, tc.release.Status)
			}
		})
	}
	var eventCount int64
	if err := db.DB.Model(&db.DataAssetEvent{}).Where("event_type = ?", "release_version_published").Count(&eventCount).Error; err != nil {
		t.Fatalf("count publish events: %v", err)
	}
	if eventCount != 0 {
		t.Fatalf("publish event count = %d after rejected transitions, want 0", eventCount)
	}
}
