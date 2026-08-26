package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"well-ambient/internal/config"
)

func TestHandleGetConfigRedactsSecretsAndOmitsBootstrapDatabase(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{Driver: "postgres", DSN: "postgres://ambient:database-secret@postgres/ambient"},
		GitLab:   config.GitLabConfig{Secret: "webhook-secret", APIToken: "gitlab-secret"},
		Feishu: config.FeishuConfig{
			AppSecret: "feishu-secret",
			Bitable:   config.BitableConfig{AppToken: "bitable-secret"},
		},
		Jira: config.JiraConfig{APIToken: "jira-secret"},
		AI:   config.AIConfig{APIToken: "ai-secret"},
	}
	server := &Server{config: cfg}
	recorder := httptest.NewRecorder()
	server.handleGetConfig(recorder, httptest.NewRequest(http.MethodGet, "/api/config", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, forbidden := range []string{"database-secret", "webhook-secret", "gitlab-secret", "feishu-secret", "bitable-secret", "jira-secret", "ai-secret", "postgres://"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("GET /api/config exposed %q in %s", forbidden, body)
		}
	}
	var response config.Config
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode config response: %v", err)
	}
	for name, value := range map[string]string{
		"gitlab webhook": response.GitLab.Secret,
		"gitlab api":     response.GitLab.APIToken,
		"feishu":         response.Feishu.AppSecret,
		"bitable":        response.Feishu.Bitable.AppToken,
		"jira":           response.Jira.APIToken,
		"ai":             response.AI.APIToken,
	} {
		if value != configuredSecretPlaceholder {
			t.Fatalf("%s secret response = %q, want configured marker", name, value)
		}
	}
	if response.Database.Driver != "" || response.Database.DSN != "" {
		t.Fatalf("bootstrap database appeared in JSON response: %#v", response.Database)
	}
}

func TestMergeConfiguredSecretsPreservesOnlyConfiguredMarker(t *testing.T) {
	current := config.Config{
		GitLab: config.GitLabConfig{Secret: "old-webhook", APIToken: "old-gitlab"},
		Jira:   config.JiraConfig{APIToken: "old-jira"},
		AI:     config.AIConfig{APIToken: "old-ai"},
	}
	next := config.Config{
		GitLab: config.GitLabConfig{Secret: configuredSecretPlaceholder, APIToken: "new-gitlab"},
		Jira:   config.JiraConfig{APIToken: configuredSecretPlaceholder},
		AI:     config.AIConfig{APIToken: ""},
	}
	mergeConfiguredSecrets(&next, current)
	if next.GitLab.Secret != "old-webhook" || next.GitLab.APIToken != "new-gitlab" || next.Jira.APIToken != "old-jira" {
		t.Fatalf("configured marker merge failed: %#v", next)
	}
	if next.AI.APIToken != "" {
		t.Fatalf("explicitly cleared secret was unexpectedly restored: %q", next.AI.APIToken)
	}
}

func TestMergeConnectionTestSecretsResolvesConfiguredMarker(t *testing.T) {
	req := ConnectionTestRequest{
		AI:   &config.AIConfig{APIToken: configuredSecretPlaceholder},
		Jira: &config.JiraConfig{APIToken: configuredSecretPlaceholder},
	}
	mergeConnectionTestSecrets(&req, config.Config{
		AI:   config.AIConfig{APIToken: "real-ai"},
		Jira: config.JiraConfig{APIToken: "real-jira"},
	})
	if req.AI.APIToken != "real-ai" || req.Jira.APIToken != "real-jira" {
		t.Fatalf("connection test secrets were not resolved: %#v", req)
	}
}
