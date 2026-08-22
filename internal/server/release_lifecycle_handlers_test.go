package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"testing"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/deliveryplanning"

	"gorm.io/gorm"
)

func TestReleaseLifecycleActionsAreAuditedAndGuarded(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "release-lifecycle@example.com", "Release Lifecycle", []string{
		"delivery:read",
		"release:manage",
	})
	srv := NewServer(&config.Config{}, "")

	planned := db.ReleaseVersion{
		ProjectKey: "FMS",
		Source:     "local",
		ExternalID: "local-lifecycle-planned",
		Name:       "FMS 5.5.0",
		Status:     deliveryplanning.ReleasePlanned,
	}
	released := db.ReleaseVersion{
		ProjectKey: "FMS",
		Source:     "local",
		ExternalID: "local-lifecycle-released",
		Name:       "FMS 5.4.0",
		Status:     deliveryplanning.ReleaseReleased,
	}
	if err := db.DB.Create(&planned).Error; err != nil {
		t.Fatalf("create planned release: %v", err)
	}
	if err := db.DB.Create(&released).Error; err != nil {
		t.Fatalf("create released release: %v", err)
	}

	discardRR := authenticatedJSONRequest(t, srv, token, http.MethodPost,
		"/api/releases/"+strconv.Itoa(int(planned.ID))+"/discard",
		map[string]string{"reason": "版本范围已取消"},
	)
	if discardRR.Code != http.StatusOK {
		t.Fatalf("discard status = %d, body=%s", discardRR.Code, discardRR.Body.String())
	}
	var discarded struct {
		Release     db.ReleaseVersion `json:"release"`
		EvidenceRef string            `json:"evidence_ref"`
		Replayed    bool              `json:"replayed"`
	}
	if err := json.Unmarshal(discardRR.Body.Bytes(), &discarded); err != nil {
		t.Fatalf("decode discard response: %v", err)
	}
	if discarded.Release.Status != deliveryplanning.ReleaseDiscarded || discarded.EvidenceRef == "" || discarded.Replayed {
		t.Fatalf("discard result = %+v", discarded)
	}
	discardReplayRR := authenticatedJSONRequest(t, srv, token, http.MethodPost,
		"/api/releases/"+strconv.Itoa(int(planned.ID))+"/discard",
		map[string]string{"reason": "重复确认版本范围已取消"},
	)
	if discardReplayRR.Code != http.StatusOK {
		t.Fatalf("discard replay status = %d, body=%s", discardReplayRR.Code, discardReplayRR.Body.String())
	}
	var discardReplay struct {
		Replayed bool `json:"replayed"`
	}
	if err := json.Unmarshal(discardReplayRR.Body.Bytes(), &discardReplay); err != nil {
		t.Fatalf("decode discard replay response: %v", err)
	}
	if !discardReplay.Replayed {
		t.Fatal("discard replay should return the existing lifecycle fact")
	}

	archiveRR := authenticatedJSONRequest(t, srv, token, http.MethodPost,
		"/api/releases/"+strconv.Itoa(int(released.ID))+"/archive",
		map[string]string{"reason": "版本维护周期结束"},
	)
	if archiveRR.Code != http.StatusOK {
		t.Fatalf("archive status = %d, body=%s", archiveRR.Code, archiveRR.Body.String())
	}
	var archived struct {
		Release     db.ReleaseVersion `json:"release"`
		EvidenceRef string            `json:"evidence_ref"`
	}
	if err := json.Unmarshal(archiveRR.Body.Bytes(), &archived); err != nil {
		t.Fatalf("decode archive response: %v", err)
	}
	if archived.Release.Status != deliveryplanning.ReleaseArchived || archived.EvidenceRef == "" {
		t.Fatalf("archive result = %+v", archived)
	}
	archiveReplayRR := authenticatedJSONRequest(t, srv, token, http.MethodPost,
		"/api/releases/"+strconv.Itoa(int(released.ID))+"/archive",
		map[string]string{"reason": "重复确认维护周期结束"},
	)
	if archiveReplayRR.Code != http.StatusOK {
		t.Fatalf("archive replay status = %d, body=%s", archiveReplayRR.Code, archiveReplayRR.Body.String())
	}
	var archiveReplay struct {
		Replayed bool `json:"replayed"`
	}
	if err := json.Unmarshal(archiveReplayRR.Body.Bytes(), &archiveReplay); err != nil {
		t.Fatalf("decode archive replay response: %v", err)
	}
	if !archiveReplay.Replayed {
		t.Fatal("archive replay should return the existing lifecycle fact")
	}

	deleteRR := authenticatedJSONRequest(t, srv, token, http.MethodDelete,
		"/api/releases/"+strconv.Itoa(int(planned.ID)),
		map[string]string{"reason": "清理已废弃的空版本", "confirm_name": planned.Name},
	)
	if deleteRR.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body=%s", deleteRR.Code, deleteRR.Body.String())
	}
	var active db.ReleaseVersion
	if err := db.DB.First(&active, planned.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("deleted release should be hidden, got err=%v release=%+v", err, active)
	}
	var retained db.ReleaseVersion
	if err := db.DB.Unscoped().First(&retained, planned.ID).Error; err != nil {
		t.Fatalf("soft-deleted release should be retained for audit: %v", err)
	}

	for releaseID, eventType := range map[uint]string{
		planned.ID:  "release_version_deleted",
		released.ID: "release_version_archived",
	} {
		var count int64
		if err := db.DB.Model(&db.DataAssetEvent{}).
			Where("subject_type = ? AND subject_id = ? AND event_type = ?", "release_version", strconv.Itoa(int(releaseID)), eventType).
			Count(&count).Error; err != nil {
			t.Fatalf("count %s evidence: %v", eventType, err)
		}
		if count != 1 {
			t.Fatalf("%s evidence count = %d, want 1", eventType, count)
		}
	}
}

func TestDeleteReleaseRejectsLinkedOrPublishedFacts(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "release-delete-guard@example.com", "Release Delete Guard", []string{
		"delivery:read",
		"release:manage",
	})
	srv := NewServer(&config.Config{}, "")

	linked := db.ReleaseVersion{
		ProjectKey: "FMS",
		Source:     "local",
		ExternalID: "local-linked",
		Name:       "FMS 5.6.0",
		Status:     deliveryplanning.ReleasePlanned,
	}
	released := db.ReleaseVersion{
		ProjectKey: "FMS",
		Source:     "local",
		ExternalID: "local-published",
		Name:       "FMS 5.3.0",
		Status:     deliveryplanning.ReleaseReleased,
	}
	if err := db.DB.Create(&linked).Error; err != nil {
		t.Fatalf("create linked release: %v", err)
	}
	if err := db.DB.Create(&released).Error; err != nil {
		t.Fatalf("create released release: %v", err)
	}
	if err := db.DB.Create(&db.WorkItemReleaseLink{
		WorkItemID:       "FMS-560",
		ReleaseVersionID: linked.ID,
		Relation:         deliveryplanning.ReleaseTargetFix,
		IsPrimary:        true,
		Active:           true,
		Source:           "local",
	}).Error; err != nil {
		t.Fatalf("create release link: %v", err)
	}

	for _, testCase := range []struct {
		name        string
		release     db.ReleaseVersion
		wantError   string
		confirmName string
	}{
		{name: "linked", release: linked, wantError: "release_delete_blocked", confirmName: linked.Name},
		{name: "published", release: released, wantError: "release_delete_forbidden", confirmName: released.Name},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			rr := authenticatedJSONRequest(t, srv, token, http.MethodDelete,
				"/api/releases/"+strconv.Itoa(int(testCase.release.ID)),
				map[string]string{"reason": "清理版本", "confirm_name": testCase.confirmName},
			)
			if rr.Code != http.StatusConflict {
				t.Fatalf("delete status = %d, body=%s", rr.Code, rr.Body.String())
			}
			var payload struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode delete error: %v", err)
			}
			if payload.Error != testCase.wantError {
				t.Fatalf("delete error = %q, want %q", payload.Error, testCase.wantError)
			}
		})
	}
}
