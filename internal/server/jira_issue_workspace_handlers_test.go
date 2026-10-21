package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"well-ambient/internal/config"
)

func TestJiraIssueWorkspaceDoesNotBorrowGeneralCredentials(t *testing.T) {
	s, admin := setupAIGovernanceOpenCapabilityServer(t)
	s.config.GitLab = config.GitLabConfig{BaseURL: "https://global.example.test", APIToken: "global-secret"}
	request := httptest.NewRequest("GET", "/api/jira-issue-sync/workspace", nil)
	request.Header.Set("Authorization", "Bearer "+admin)
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, request)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		GitLab bool `json:"gitlab_configured"`
		Jira   bool `json:"jira_configured"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.GitLab || body.Jira || strings.Contains(rec.Body.String(), "global") {
		t.Fatal("independent configuration borrowed global settings")
	}
	member := performanceMemberToken(t, "sync-reader@example.test", []string{"config:read", "config:write"})
	for _, action := range []string{"discover", "analyze", "sync", "cancel"} {
		req := httptest.NewRequest("POST", "/api/jira-issue-sync/workspace/"+action, strings.NewReader(`{"version":1}`))
		req.Header.Set("Authorization", "Bearer "+member)
		result := httptest.NewRecorder()
		s.handler.ServeHTTP(result, req)
		if result.Code != 403 {
			t.Fatalf("member could execute %s: %d", action, result.Code)
		}
	}
}
