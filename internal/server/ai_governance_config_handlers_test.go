package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

func setupAIGovernanceConfigServer(t *testing.T, cfg config.Config) (*Server, string) {
	t.Helper()
	setupServerTestDB(t)
	t.Setenv("WELL_AMBIENT_MAINTENANCE_MODE", "")
	t.Setenv("WELL_AMBIENT_MAINTENANCE_MODE_SOURCE", "")
	if err := BootstrapVersionedConfig(&cfg); err != nil {
		t.Fatalf("bootstrap versioned config: %v", err)
	}
	token := superAdminToken(
		t,
		"ai-governance-admin@westwell-lab.com",
		"AI Governance Admin",
		[]string{"solution_prompt:manage", "config:read", "config:write", "ai_context:read"},
	)
	return NewServer(&cfg, ""), token
}

func TestSolutionPublicURLUpdatePersistsVersionAndPreservesUnrelatedConfig(t *testing.T) {
	cfg := config.Config{
		Database: config.DatabaseConfig{Driver: "sqlite", DSN: ":memory:"},
		Server: config.ServerConfig{
			Host:            "127.0.0.1",
			Port:            8080,
			PublicURL:       "https://old.example.com",
			MaintenanceMode: true,
		},
		Jira: config.JiraConfig{
			Enabled:  true,
			BaseURL:  "https://jira.example.com",
			APIToken: "jira-secret",
		},
		AI: config.AIConfig{
			Enabled:  true,
			BaseURL:  "https://ai.example.com/v1",
			APIToken: "ai-secret",
			Model:    "governance-model",
		},
		SMTP: config.SMTPConfig{
			Enabled: true,
			Host:    "smtp.example.com",
			Port:    587,
		},
	}
	srv, token := setupAIGovernanceConfigServer(t, cfg)

	getRequest := httptest.NewRequest(http.MethodGet, "/api/solution-prompts/public-url", nil)
	getRequest.Header.Set("Authorization", "Bearer "+token)
	getRecorder := httptest.NewRecorder()
	srv.mux.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getRecorder.Code, getRecorder.Body.String())
	}
	var before solutionPublicURLResponse
	if err := json.NewDecoder(getRecorder.Body).Decode(&before); err != nil {
		t.Fatalf("decode current public URL: %v", err)
	}
	if before.PublicURL != "https://old.example.com" || before.Version != 1 {
		t.Fatalf("unexpected current public URL: %#v", before)
	}

	putRequest := httptest.NewRequest(
		http.MethodPut,
		"/api/solution-prompts/public-url",
		bytes.NewBufferString(`{"public_url":" https://new.example.com/ ","expected_version":1}`),
	)
	putRequest.Header.Set("Authorization", "Bearer "+token)
	putRecorder := httptest.NewRecorder()
	srv.mux.ServeHTTP(putRecorder, putRequest)
	if putRecorder.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", putRecorder.Code, putRecorder.Body.String())
	}
	var updated solutionPublicURLResponse
	if err := json.NewDecoder(putRecorder.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated public URL: %v", err)
	}
	if updated.PublicURL != "https://new.example.com" || updated.Version != 2 {
		t.Fatalf("unexpected updated public URL: %#v", updated)
	}

	if srv.config.Server.PublicURL != updated.PublicURL || !srv.config.Server.MaintenanceMode {
		t.Fatalf("server fields were not applied atomically: %#v", srv.config.Server)
	}
	if srv.config.Jira.BaseURL != cfg.Jira.BaseURL || srv.config.Jira.APIToken != cfg.Jira.APIToken ||
		srv.config.AI.Model != cfg.AI.Model || srv.config.AI.APIToken != cfg.AI.APIToken ||
		srv.config.SMTP.Host != cfg.SMTP.Host {
		t.Fatalf("unrelated config was lost: %#v", srv.config)
	}

	var runtime db.RuntimeConfig
	if err := db.DB.First(&runtime, runtimeConfigSingletonID).Error; err != nil {
		t.Fatalf("read runtime config: %v", err)
	}
	if runtime.Version != 2 {
		t.Fatalf("runtime version = %d, want 2", runtime.Version)
	}
	var persisted config.Config
	if err := json.Unmarshal([]byte(runtime.ConfigJSON), &persisted); err != nil {
		t.Fatalf("decode runtime config: %v", err)
	}
	if persisted.Server.PublicURL != updated.PublicURL || !persisted.Server.MaintenanceMode ||
		persisted.Jira.BaseURL != cfg.Jira.BaseURL || persisted.Jira.APIToken != cfg.Jira.APIToken ||
		persisted.AI.Model != cfg.AI.Model || persisted.AI.APIToken != cfg.AI.APIToken ||
		persisted.SMTP.Host != cfg.SMTP.Host {
		t.Fatalf("runtime config lost unrelated fields: %s", runtime.ConfigJSON)
	}

	var version db.ConfigVersion
	if err := db.DB.Where("version = ?", 2).First(&version).Error; err != nil {
		t.Fatalf("read public URL config version: %v", err)
	}
	if version.Source != "solution-public-url" || !strings.Contains(version.ChangedSectionsJSON, `"server"`) {
		t.Fatalf("unexpected config version: %#v", version)
	}
	var audit userdb.AuditLog
	if err := db.DB.Where("action = ?", "solution_public_url_updated").First(&audit).Error; err != nil {
		t.Fatalf("read public URL audit: %v", err)
	}
	if audit.TargetID != "solution-public-url" {
		t.Fatalf("unexpected public URL audit: %#v", audit)
	}
}

func TestSolutionPublicURLUpdateRejectsStaleVersionWithoutMutation(t *testing.T) {
	cfg := config.Config{
		Database: config.DatabaseConfig{Driver: "sqlite", DSN: ":memory:"},
		Server: config.ServerConfig{
			Host:      "127.0.0.1",
			Port:      8080,
			PublicURL: "https://current.example.com",
		},
	}
	srv, token := setupAIGovernanceConfigServer(t, cfg)

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/solution-prompts/public-url",
		bytes.NewBufferString(`{"public_url":"https://stale.example.com","expected_version":0}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	srv.mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "config_version_conflict") {
		t.Fatalf("stale update status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if srv.config.Server.PublicURL != cfg.Server.PublicURL {
		t.Fatalf("stale update changed in-memory config: %q", srv.config.Server.PublicURL)
	}
	var runtime db.RuntimeConfig
	if err := db.DB.First(&runtime, runtimeConfigSingletonID).Error; err != nil {
		t.Fatalf("read runtime config: %v", err)
	}
	if runtime.Version != 1 || !strings.Contains(runtime.ConfigJSON, cfg.Server.PublicURL) {
		t.Fatalf("stale update changed runtime config: %#v", runtime)
	}
	var versionCount int64
	if err := db.DB.Model(&db.ConfigVersion{}).Count(&versionCount).Error; err != nil {
		t.Fatalf("count config versions: %v", err)
	}
	if versionCount != 1 {
		t.Fatalf("stale update created config versions: %d", versionCount)
	}
}

func TestSolutionPublicURLUpdateRequiresGlobalSuperAdmin(t *testing.T) {
	cfg := config.Config{
		Database: config.DatabaseConfig{Driver: "sqlite", DSN: ":memory:"},
		Server:   config.ServerConfig{Host: "127.0.0.1", Port: 8080},
	}
	srv, _ := setupAIGovernanceConfigServer(t, cfg)
	user := seedLocalUserWithGroup(t, "ai-governance-operator@westwell-lab.com", "AI Governance Operator", "admin", "secret")
	token, err := GenerateJWT(
		user.Username,
		user.Name,
		"mock_wellos_token",
		"",
		[]string{"admin"},
		[]string{"solution_prompt:manage", "config:read", "config:write"},
	)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/solution-prompts/public-url",
		bytes.NewBufferString(`{"public_url":"https://forbidden.example.com","expected_version":1}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	srv.mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("non-super-admin status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestAIContextReadinessReturnsOnlyNonSecretStatus(t *testing.T) {
	tests := []struct {
		name       string
		ai         config.AIConfig
		wantReady  bool
		wantStatus string
	}{
		{name: "disabled", ai: config.AIConfig{Enabled: false, BaseURL: "https://ai.example.com", APIToken: "secret"}, wantStatus: "disabled"},
		{name: "incomplete", ai: config.AIConfig{Enabled: true, BaseURL: "https://ai.example.com"}, wantStatus: "incomplete"},
		{name: "ready", ai: config.AIConfig{Enabled: true, BaseURL: "https://ai.example.com", APIToken: "secret"}, wantReady: true, wantStatus: "ready"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.Config{
				Database: config.DatabaseConfig{Driver: "sqlite", DSN: ":memory:"},
				Server:   config.ServerConfig{Host: "127.0.0.1", Port: 8080},
				AI:       testCase.ai,
			}
			srv, token := setupAIGovernanceConfigServer(t, cfg)
			request := httptest.NewRequest(http.MethodGet, "/api/ai/context-readiness", nil)
			request.Header.Set("Authorization", "Bearer "+token)
			recorder := httptest.NewRecorder()
			srv.mux.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("readiness status = %d, body = %s", recorder.Code, recorder.Body.String())
			}

			var payload map[string]any
			if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
				t.Fatalf("decode readiness: %v", err)
			}
			if len(payload) != 2 || payload["ready"] != testCase.wantReady || payload["status"] != testCase.wantStatus {
				t.Fatalf("unexpected readiness payload: %#v", payload)
			}
			encoded, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("encode readiness payload: %v", err)
			}
			body := string(encoded)
			if token := strings.TrimSpace(testCase.ai.APIToken); token != "" && strings.Contains(body, token) {
				t.Fatalf("readiness leaked AI token: %s", body)
			}
			if baseURL := strings.TrimSpace(testCase.ai.BaseURL); baseURL != "" && strings.Contains(body, baseURL) {
				t.Fatalf("readiness leaked AI base URL: %s", body)
			}
		})
	}
}
