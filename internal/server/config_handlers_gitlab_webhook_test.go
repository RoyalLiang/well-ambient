package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"well-ambient/internal/config"
)

func TestEnsureGitLabWebhooksUpdatesExistingHook(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "gitlab-admin@example.com", "GitLab Admin", []string{"config:write"})

	const secret = "super-secret-token"
	const apiToken = "private-api-token"
	var seenPut bool
	var putPayload map[string]interface{}

	gitlab := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("PRIVATE-TOKEN"); got != apiToken {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/42/hooks":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[{"id":7,"url":"https://ambient.example.com/api/webhook/gitlab","push_events":false,"merge_requests_events":false}]`)
		case r.Method == http.MethodPut && r.URL.Path == "/api/v4/projects/42/hooks/7":
			seenPut = true
			if err := json.NewDecoder(r.Body).Decode(&putPayload); err != nil {
				t.Fatalf("decode PUT payload: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, `{"id":7,"url":"https://ambient.example.com/api/webhook/gitlab","push_events":true,"merge_requests_events":true}`)
		default:
			t.Fatalf("unexpected GitLab API call: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer gitlab.Close()

	srv := NewServer(&config.Config{
		GitLab: config.GitLabConfig{
			BaseURL:  gitlab.URL,
			APIToken: apiToken,
			Secret:   secret,
			Repos: []config.RepoMapping{{
				Name:      "Core",
				Path:      "group/core",
				ProjectID: "42",
			}},
		},
	}, "")

	reqBody := []byte(`{"webhook_url":"https://ambient.example.com/api/webhook/gitlab"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/gitlab/webhooks/ensure", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("ensure status = %d, body: %s", rr.Code, rr.Body.String())
	}
	if !seenPut {
		t.Fatalf("expected existing hook to be updated with PUT")
	}
	if putPayload["token"] != secret {
		t.Fatalf("PUT payload token was not configured secret")
	}
	if putPayload["push_events"] != true || putPayload["merge_requests_events"] != true {
		t.Fatalf("PUT payload events not enabled: %#v", putPayload)
	}

	responseBody := rr.Body.String()
	if strings.Contains(responseBody, secret) || strings.Contains(responseBody, apiToken) {
		t.Fatalf("ensure response leaked a secret: %s", responseBody)
	}

	var res GitLabWebhookResponse
	if err := json.Unmarshal([]byte(responseBody), &res); err != nil {
		t.Fatalf("decode ensure response: %v", err)
	}
	if len(res.Results) != 1 {
		t.Fatalf("result count = %d, want 1", len(res.Results))
	}
	got := res.Results[0]
	if got.ProjectID != "42" || got.Name != "Core" || got.Path != "group/core" {
		t.Fatalf("unexpected project identity in result: %#v", got)
	}
	if got.Status != "updated" || got.HookID != 7 {
		t.Fatalf("result = %#v, want updated hook 7", got)
	}
}

func TestEnsureGitLabWebhooksCreatesMissingHookForSelectedProject(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "gitlab-creator@example.com", "GitLab Creator", []string{"config:write"})

	const secret = "create-secret"
	const apiToken = "create-api-token"
	var seenPost bool

	gitlab := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("PRIVATE-TOKEN"); got != apiToken {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}

		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/api/v4/projects/group%2Fselected/hooks":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[]`)
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/api/v4/projects/group%2Fselected/hooks":
			seenPost = true
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode POST payload: %v", err)
			}
			if payload["url"] != "https://ambient.example.com/api/webhook/gitlab" {
				t.Fatalf("POST url = %#v", payload["url"])
			}
			if payload["token"] != secret {
				t.Fatalf("POST token was not configured secret")
			}
			if payload["push_events"] != true || payload["merge_requests_events"] != true {
				t.Fatalf("POST payload events not enabled: %#v", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"id":11,"url":"https://ambient.example.com/api/webhook/gitlab","push_events":true,"merge_requests_events":true}`)
		default:
			t.Fatalf("unexpected GitLab API call: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer gitlab.Close()

	srv := NewServer(&config.Config{
		GitLab: config.GitLabConfig{
			BaseURL:  gitlab.URL,
			APIToken: apiToken,
			Secret:   secret,
			Repos: []config.RepoMapping{{
				Name:      "Ignored Config Repo",
				Path:      "group/configured",
				ProjectID: "99",
			}},
		},
	}, "")

	reqBody := []byte(`{"projects":[{"name":"Selected","path":"group/selected"}],"webhook_url":"https://ambient.example.com/api/webhook/gitlab"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/gitlab/webhooks/ensure", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("ensure status = %d, body: %s", rr.Code, rr.Body.String())
	}
	if !seenPost {
		t.Fatalf("expected missing hook to be created with POST")
	}

	var res GitLabWebhookResponse
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("decode ensure response: %v", err)
	}
	if len(res.Results) != 1 {
		t.Fatalf("result count = %d, want selected repo only", len(res.Results))
	}
	if res.Results[0].Status != "created" || res.Results[0].HookID != 11 {
		t.Fatalf("result = %#v, want created hook 11", res.Results[0])
	}
	if res.Results[0].Path != "group/selected" {
		t.Fatalf("result path = %q, want selected path", res.Results[0].Path)
	}
}

func TestGetGitLabWebhookStatusReportsConfiguredRepos(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "gitlab-status@example.com", "GitLab Status", []string{"config:write"})

	const apiToken = "status-api-token"
	gitlab := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("PRIVATE-TOKEN"); got != apiToken {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/hooks":
			io.WriteString(w, `[{"id":1,"url":"https://ambient.example.com/api/webhook/gitlab","push_events":true,"merge_requests_events":true}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/2/hooks":
			io.WriteString(w, `[{"id":2,"url":"https://ambient.example.com/api/webhook/gitlab","push_events":true,"merge_requests_events":false}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/3/hooks":
			io.WriteString(w, `[]`)
		default:
			t.Fatalf("unexpected GitLab API call: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer gitlab.Close()

	srv := NewServer(&config.Config{
		GitLab: config.GitLabConfig{
			BaseURL:  gitlab.URL,
			APIToken: apiToken,
			Secret:   "status-secret",
			Repos: []config.RepoMapping{
				{Name: "OK", ProjectID: "1"},
				{Name: "Drift", ProjectID: "2"},
				{Name: "Missing", ProjectID: "3"},
			},
		},
	}, "")

	req := httptest.NewRequest(http.MethodGet, "/api/gitlab/webhooks/status?webhook_url=https://ambient.example.com/api/webhook/gitlab", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d, body: %s", rr.Code, rr.Body.String())
	}
	responseBody := rr.Body.String()
	if strings.Contains(responseBody, "status-secret") || strings.Contains(responseBody, apiToken) {
		t.Fatalf("status response leaked a secret: %s", responseBody)
	}

	var res GitLabWebhookResponse
	if err := json.Unmarshal([]byte(responseBody), &res); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if len(res.Results) != 3 {
		t.Fatalf("result count = %d, want 3", len(res.Results))
	}

	want := map[string]string{
		"OK":      "ok",
		"Drift":   "drift",
		"Missing": "missing",
	}
	for _, result := range res.Results {
		if result.Status != want[result.Name] {
			t.Fatalf("project %s status = %q, want %q (message: %s)", result.Name, result.Status, want[result.Name], result.Message)
		}
	}
}

func TestResolveGitLabWebhookURLDefaultsFromRequestOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/gitlab/webhooks/ensure", nil)
	req.Host = "ambient.internal"
	req.Header.Set("X-Forwarded-Proto", "https")

	got, err := resolveGitLabWebhookURL(req, "")
	if err != nil {
		t.Fatalf("resolve default webhook URL: %v", err)
	}
	if got != "https://ambient.internal/api/webhook/gitlab" {
		t.Fatalf("default webhook URL = %q", got)
	}
}
