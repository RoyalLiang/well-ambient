package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/deliveryplanning"
)

func TestReleaseArchiveOnlyLifecycleContract(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "release-archive-only@example.com", "Release Archive Only", []string{
		"delivery:read",
		"release:manage",
	})
	srv := NewServer(&config.Config{}, "")

	for _, status := range []string{
		deliveryplanning.ReleasePlanned,
		deliveryplanning.ReleaseReleased,
		deliveryplanning.ReleaseDiscarded,
	} {
		t.Run("archive_"+status, func(t *testing.T) {
			release := db.ReleaseVersion{
				ProjectKey: "FMS",
				Source:     "local",
				ExternalID: "archive-only-" + status,
				Name:       "Archive Only " + status,
				Status:     status,
			}
			if err := db.DB.Create(&release).Error; err != nil {
				t.Fatalf("create release: %v", err)
			}

			rr := authenticatedJSONRequest(t, srv, token, http.MethodPost,
				"/api/releases/"+strconv.Itoa(int(release.ID))+"/archive",
				map[string]string{"reason": "移入归档版本列表"},
			)
			if rr.Code != http.StatusOK {
				t.Fatalf("archive status = %d, body=%s", rr.Code, rr.Body.String())
			}
			var payload struct {
				Release db.ReleaseVersion `json:"release"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode archive response: %v", err)
			}
			if payload.Release.Status != deliveryplanning.ReleaseArchived {
				t.Fatalf("archive status = %q, want archived", payload.Release.Status)
			}
		})
	}
}

func TestReleaseDeleteIsAvailableOnlyAfterArchive(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "release-delete-archived@example.com", "Release Delete Archived", []string{
		"delivery:read",
		"release:manage",
	})
	srv := NewServer(&config.Config{}, "")

	planned := db.ReleaseVersion{ProjectKey: "FMS", Source: "local", ExternalID: "delete-planned", Name: "Delete Planned", Status: deliveryplanning.ReleasePlanned}
	archived := db.ReleaseVersion{ProjectKey: "FMS", Source: "local", ExternalID: "delete-archived", Name: "Delete Archived", Status: deliveryplanning.ReleaseArchived}
	if err := db.DB.Create(&planned).Error; err != nil {
		t.Fatalf("create planned release: %v", err)
	}
	if err := db.DB.Create(&archived).Error; err != nil {
		t.Fatalf("create archived release: %v", err)
	}

	plannedRR := authenticatedJSONRequest(t, srv, token, http.MethodDelete,
		"/api/releases/"+strconv.Itoa(int(planned.ID)),
		map[string]string{"reason": "不应直接删除", "confirm_name": planned.Name},
	)
	if plannedRR.Code != http.StatusConflict {
		t.Fatalf("planned delete status = %d, body=%s", plannedRR.Code, plannedRR.Body.String())
	}

	archivedRR := authenticatedJSONRequest(t, srv, token, http.MethodDelete,
		"/api/releases/"+strconv.Itoa(int(archived.ID)),
		map[string]string{"reason": "从归档列表删除", "confirm_name": archived.Name},
	)
	if archivedRR.Code != http.StatusOK {
		t.Fatalf("archived delete status = %d, body=%s", archivedRR.Code, archivedRR.Body.String())
	}
}

func TestReleaseLifecycleCapabilitiesExposeArchiveThenDelete(t *testing.T) {
	for _, status := range []string{deliveryplanning.ReleasePlanned, deliveryplanning.ReleaseReleased, deliveryplanning.ReleaseDiscarded} {
		capabilities := releaseLifecycleFor(db.ReleaseVersion{Source: "local", Status: status}, 0, 0)
		if capabilities.CanPublish || !capabilities.CanArchive || capabilities.CanDelete || capabilities.CanDiscard {
			t.Fatalf("%s capabilities = %+v, want archive only", status, capabilities)
		}
	}

	archived := releaseLifecycleFor(db.ReleaseVersion{Source: "local", Status: deliveryplanning.ReleaseArchived}, 0, 0)
	if archived.CanPublish || archived.CanArchive || !archived.CanDelete || archived.CanDiscard {
		t.Fatalf("archived capabilities = %+v, want delete only", archived)
	}
}

func TestReleaseListViewsSeparateCurrentAndArchivedVersions(t *testing.T) {
	setupServerTestDB(t)
	srv := NewServer(&config.Config{}, "")
	releases := []db.ReleaseVersion{
		{Source: "local", ExternalID: "view-planned", Name: "View Planned", Status: deliveryplanning.ReleasePlanned},
		{Source: "local", ExternalID: "view-discarded", Name: "View Discarded", Status: deliveryplanning.ReleaseDiscarded},
		{Source: "local", ExternalID: "view-archived", Name: "View Archived", Status: deliveryplanning.ReleaseArchived},
	}
	if err := db.DB.Create(&releases).Error; err != nil {
		t.Fatalf("create release views: %v", err)
	}

	readView := func(view string) []db.ReleaseVersion {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/api/releases?view="+view, nil)
		recorder := httptest.NewRecorder()
		srv.handleListReleases(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("list %s status = %d, body=%s", view, recorder.Code, recorder.Body.String())
		}
		var payload struct {
			Items []struct {
				Release db.ReleaseVersion `json:"release"`
			} `json:"items"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode %s list: %v", view, err)
		}
		result := make([]db.ReleaseVersion, 0, len(payload.Items))
		for _, item := range payload.Items {
			result = append(result, item.Release)
		}
		return result
	}

	current := readView("current")
	if len(current) != 2 {
		t.Fatalf("current releases = %+v, want planned and discarded", current)
	}
	for _, release := range current {
		if release.Status == deliveryplanning.ReleaseArchived {
			t.Fatalf("archived release leaked into current view: %+v", release)
		}
	}

	archived := readView("archived")
	if len(archived) != 1 || archived[0].Status != deliveryplanning.ReleaseArchived {
		t.Fatalf("archived releases = %+v, want archived only", archived)
	}
}
