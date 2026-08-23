package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestStatusReportsJiraInboundSyncHealthWithoutLeakingErrorDetails(t *testing.T) {
	setupServerTestDB(t)

	now := time.Now().UTC().Truncate(time.Second)
	state := db.JiraInboundSyncState{
		Scope:             jiraInboundSyncScope,
		SuccessfulThrough: now.Add(-5 * time.Minute),
		LastStartedAt:     now.Add(-2 * time.Minute),
		LastSucceededAt:   now.Add(-5 * time.Minute),
		LastError:         "GET https://jira.example.test?token=must-not-leak: connection reset",
		LastIssueCount:    37,
		LastChangedCount:  2,
	}
	if err := db.DB.Create(&state).Error; err != nil {
		t.Fatalf("seed Jira inbound sync state: %v", err)
	}

	server := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: true}}, "")
	recorder := httptest.NewRecorder()
	server.handleStatus(recorder, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/status status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "must-not-leak") {
		t.Fatal("public status response leaked Jira sync error details")
	}

	var payload struct {
		JiraSync struct {
			Enabled           bool       `json:"enabled"`
			State             string     `json:"state"`
			SuccessfulThrough *time.Time `json:"successful_through"`
			LastStartedAt     *time.Time `json:"last_started_at"`
			LastSucceededAt   *time.Time `json:"last_succeeded_at"`
			LastIssueCount    int        `json:"last_issue_count"`
			LastChangedCount  int        `json:"last_changed_count"`
			HasError          bool       `json:"has_error"`
		} `json:"jira_sync"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if !payload.JiraSync.Enabled || payload.JiraSync.State != "error" || !payload.JiraSync.HasError {
		t.Fatalf("unexpected Jira sync status: %+v", payload.JiraSync)
	}
	if payload.JiraSync.LastIssueCount != 37 || payload.JiraSync.LastChangedCount != 2 {
		t.Fatalf("unexpected Jira sync counts: %+v", payload.JiraSync)
	}
	if payload.JiraSync.SuccessfulThrough == nil || payload.JiraSync.LastStartedAt == nil || payload.JiraSync.LastSucceededAt == nil {
		t.Fatalf("Jira sync timestamps missing: %+v", payload.JiraSync)
	}
}

func TestStatusReportsDisabledAndPendingJiraInboundSync(t *testing.T) {
	setupServerTestDB(t)

	tests := []struct {
		name    string
		enabled bool
		want    string
	}{
		{name: "disabled", enabled: false, want: "disabled"},
		{name: "enabled before first cycle", enabled: true, want: "pending"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			server := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: testCase.enabled}}, "")
			recorder := httptest.NewRecorder()
			server.handleStatus(recorder, httptest.NewRequest(http.MethodGet, "/api/status", nil))

			var payload struct {
				JiraSync struct {
					State string `json:"state"`
				} `json:"jira_sync"`
			}
			if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
				t.Fatalf("decode status response: %v", err)
			}
			if payload.JiraSync.State != testCase.want {
				t.Fatalf("Jira sync state = %q, want %q", payload.JiraSync.State, testCase.want)
			}
		})
	}
}

func TestStatusReportsJiraInboundSyncStaleAfterFourMissedCycles(t *testing.T) {
	setupServerTestDB(t)

	now := time.Now().UTC().Truncate(time.Second)
	lastCycle := now.Add(-jiraInboundStaleAfter - time.Second)
	if err := db.DB.Create(&db.JiraInboundSyncState{
		Scope:             jiraInboundSyncScope,
		SuccessfulThrough: lastCycle,
		LastStartedAt:     lastCycle,
		LastSucceededAt:   lastCycle,
	}).Error; err != nil {
		t.Fatalf("seed stale Jira inbound sync state: %v", err)
	}

	server := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: true}}, "")
	status := server.jiraInboundStatus(now)
	if status.State != "stale" || status.HasError {
		t.Fatalf("Jira sync status = %+v, want stale without source error", status)
	}
}
