package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestKnowledgeBrowserAndJiraIssuePreviewPermissions(t *testing.T) {
	srv, admin := setupAIGovernanceOpenCapabilityServer(t)
	srv.config.Jira.SyncProjects = []string{"NS2"}
	srv.config.GitLab.Repos = []config.RepoMapping{{ProjectID: "42", Path: "ops/terminal"}}
	member := performanceMemberToken(t, "preview-reader@example.test", []string{"ai_context:read", "config:read"})
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		srv.handler.ServeHTTP(rec, req)
		return rec
	}
	for _, path := range []string{"/api/agent-runtime/knowledge/items", "/api/jira-issue-sync"} {
		if rec := call("GET", path, member, ""); rec.Code != 403 {
			t.Fatalf("member read escaped global boundary %s: %d", path, rec.Code)
		}
	}
	if rec := call("GET", "/api/agent-runtime/knowledge/items", admin, ""); rec.Code != 200 {
		t.Fatalf("browse: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call("GET", "/api/agent-runtime/knowledge/items?snapshot_id=oops", admin, ""); rec.Code != 400 {
		t.Fatal("bad snapshot accepted")
	}
	rec := call("GET", "/api/jira-issue-sync", admin, "")
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var response struct {
		Settings db.JiraIssueSyncConfig `json:"settings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	v := response.Settings
	v.JiraProject = "NS2"
	v.GroupPath = "ops"
	v.TargetProjectID = "42"
	raw, _ := json.Marshal(v)
	rec = call("PUT", "/api/jira-issue-sync", admin, string(raw))
	if rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	if rec = call("POST", "/api/jira-issue-sync/preview", admin, `{"version":1}`); rec.Code != 409 {
		t.Fatal("stale preview accepted")
	}
	if rec = call("POST", "/api/jira-issue-sync/preview", admin, `{"version":2}`); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"remote_verified":false`) {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
}

func TestGitLabProjectCredentialBodyAndHostBinding(t *testing.T) {
	var received bool
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.Header.Get("PRIVATE-TOKEN") == "fixture-token"
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer remote.Close()
	srv, token := setupAIGovernanceOpenCapabilityServer(t)
	srv.config.GitLab.BaseURL = remote.URL
	srv.config.GitLab.APIToken = "fixture-token"
	for _, test := range []struct {
		host   string
		status int
	}{{remote.URL, 200}, {"https://different.example.test", 400}} {
		body, _ := json.Marshal(map[string]string{"base_url": test.host, "api_token": configuredSecretPlaceholder})
		req := httptest.NewRequest("POST", "/api/gitlab/projects", strings.NewReader(string(body)))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		srv.handler.ServeHTTP(rec, req)
		if rec.Code != test.status {
			t.Fatalf("status %d expected %d: %s", rec.Code, test.status, rec.Body.String())
		}
	}
	if !received {
		t.Fatal("token body flow did not reach configured GitLab")
	}
}
