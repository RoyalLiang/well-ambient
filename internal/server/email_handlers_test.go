package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestSMTPSecretPersistenceRedactionAndArchive(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	cfg.SMTP.TLSMode = "starttls"
	cfg.SMTP.Username = "smtp-user"
	cfg.SMTP.Password = "smtp-password-never-return"
	s := &Server{config: &cfg}
	version, err := s.recordConfigVersion(config.Config{}, cfg, httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader("{}")), "manual-save", 0)
	if err != nil {
		t.Fatal(err)
	}
	var persisted db.RuntimeConfig
	if err := conn.First(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(persisted.ConfigJSON, cfg.SMTP.Password) {
		t.Fatal("runtime password not persisted")
	}
	if strings.Contains(version.ConfigJSON, cfg.SMTP.Password) || strings.Contains(version.RedactedConfigJSON, cfg.SMTP.Password) || strings.Contains(version.DiffJSON, cfg.SMTP.Password) {
		t.Fatal("password leaked into archive")
	}
	response := httptest.NewRecorder()
	s.handleGetConfig(response, httptest.NewRequest("GET", "/api/config", nil))
	if strings.Contains(response.Body.String(), cfg.SMTP.Password) {
		t.Fatal("password leaked via GET")
	}
	var redacted config.Config
	if err := json.Unmarshal(response.Body.Bytes(), &redacted); err != nil {
		t.Fatal(err)
	}
	if redacted.SMTP.Password != configuredSecretPlaceholder {
		t.Fatal("missing marker")
	}
	mergeConfiguredSecrets(&redacted, cfg)
	if redacted.SMTP.Password != cfg.SMTP.Password {
		t.Fatal("marker failed")
	}
	redacted.SMTP.Password = ""
	mergeConfiguredSecrets(&redacted, cfg)
	if redacted.SMTP.Password != "" {
		t.Fatal("explicit clear failed")
	}
	restored := config.Config{}
	if err := BootstrapVersionedConfig(&restored); err != nil {
		t.Fatal(err)
	}
	if restored.SMTP.Password != cfg.SMTP.Password {
		t.Fatal("restart lost password")
	}
}
func TestGenerateEmailTemplateLLMValidation(t *testing.T) {
	cfg := emailTestConfig()
	s := &Server{config: &cfg}
	called := false
	s.emailLLM = func(ctx context.Context, system, user string) (string, error) {
		called = true
		if !strings.Contains(user, "Current template:") || !strings.Contains(user, "Requirements:\n简洁中文") || !strings.Contains(system, "replace the complete") || !strings.Contains(system, "style") || !strings.Contains(system, "JSON") {
			t.Fatal("wrong prompt")
		}
		return `{"style":"custom","subject":"日报 {{date}}","introduction":"开始","closing":"结束","html":"<div>{{introduction}}</div><div>{{closing}}</div><span>{{date}}</span><span>{{timezone}}</span><div>{{yesterday}}</div><div>{{unresolved}}</div>"}`, nil
	}
	response := httptest.NewRecorder()
	s.handleGenerateEmailTemplate(response, httptest.NewRequest("POST", "/api/daily-jira-email/template", strings.NewReader(`{"requirements":"简洁中文"}`)))
	if response.Code != 200 || !called || !strings.Contains(response.Body.String(), "日报") {
		t.Fatalf("template %d %s", response.Code, response.Body.String())
	}
	for _, output := range []string{`{"subject":"ok","html":"<script>evil</script>"}`, `{"subject":"{{secret}}"}`, `{"subject":"ok"} {"subject":"second"}`, `{"style":"custom","subject":"ok"}`, `{"subject":"ok"}`} {
		s.emailLLM = func(context.Context, string, string) (string, error) { return output, nil }
		response = httptest.NewRecorder()
		s.handleGenerateEmailTemplate(response, httptest.NewRequest("POST", "/api/daily-jira-email/template", strings.NewReader(`{"requirements":"test"}`)))
		if response.Code != 502 {
			t.Fatalf("accepted malformed %s: %d", output, response.Code)
		}
	}
}
func TestEmailEndpointsRequireGlobalPermission(t *testing.T) {
	setupServerTestDB(t)
	cfg := emailTestConfig()
	s := &Server{config: &cfg, mux: http.NewServeMux()}
	s.routes()
	scoped := seedPolicyTestUser(t, "scoped@example.test", "admin", "repo", "X")
	global := superAdminToken(t, "global@example.test", "Global", []string{"config:read", "config:write"})
	for _, path := range []string{"/api/daily-jira-email/template", "/api/daily-jira-email/preview", "/api/daily-jira-email/send", "/api/config/test"} {
		for _, suffix := range []string{"", "?repo=X", "?project_id=X"} {
			for _, test := range []struct {
				token string
				want  int
			}{{"", 401}, {scoped, 403}} {
				request := httptest.NewRequest("POST", path+suffix, strings.NewReader(`{}`))
				if test.token != "" {
					request.Header.Set("Authorization", "Bearer "+test.token)
				}
				response := httptest.NewRecorder()
				s.mux.ServeHTTP(response, request)
				if response.Code != test.want {
					t.Fatalf("%s auth expected %d got %d", path+suffix, test.want, response.Code)
				}
			}
		}
	}
	request := httptest.NewRequest("POST", "/api/daily-jira-email/preview?repo=X", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+global)
	response := httptest.NewRecorder()
	s.mux.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("global preview denied: %d %s", response.Code, response.Body.String())
	}
}
func TestEmailPreviewNeverSendsAndSendRejectsDraftOverrides(t *testing.T) {
	emailTestDatabase(t)
	cfg := emailTestConfig()
	s := &Server{config: &cfg}
	calls := 0
	s.emailSender = func(ctx context.Context, smtp config.SMTPConfig, to []string, subject, text, html string) error {
		calls++
		if len(to) != 1 || to[0] != "to@example.test" {
			t.Fatalf("recipients changed: %v", to)
		}
		return nil
	}
	body := `{"date":"2026-03-09","settings":{"recipients":["attacker@example.test"],"timezone":"UTC","template":{"subject":"Draft"}}}`
	response := httptest.NewRecorder()
	s.handlePreviewEmail(response, httptest.NewRequest("POST", "/api/daily-jira-email/preview", strings.NewReader(body)))
	if response.Code != 200 || calls != 0 {
		t.Fatalf("preview %d calls=%d", response.Code, calls)
	}
	response = httptest.NewRecorder()
	s.handleSendEmail(response, httptest.NewRequest("POST", "/api/daily-jira-email/send", strings.NewReader(body)))
	if response.Code != 400 || calls != 0 {
		t.Fatal("send allowed draft override")
	}
	for _, want := range []int{200, 409} {
		response = httptest.NewRecorder()
		s.handleSendEmail(response, httptest.NewRequest("POST", "/api/daily-jira-email/send", strings.NewReader(`{"date":"2026-03-09"}`)))
		if response.Code != want {
			t.Fatalf("send want %d got %d: %s", want, response.Code, response.Body.String())
		}
	}
	if calls != 1 {
		t.Fatalf("duplicate SMTP calls %d", calls)
	}
}

func TestEmailCandidateOwnersEndpoint(t *testing.T) {
	setupServerTestDB(t)
	cfg := emailTestConfig()
	cfg.Jira.SyncUsers = []string{"Alice", "Bob"}
	s := &Server{config: &cfg, mux: http.NewServeMux()}

	// Insert task telemetry with distinct assignees into db
	if db.DB != nil {
		_ = db.DB.Create(&db.TaskTelemetry{
			TaskID:   "TEST-99",
			Title:    "Something",
			Assignee: "Charlie",
			Status:   "open",
		})
	}

	req := httptest.NewRequest("GET", "/api/daily-jira-email/candidate-owners", nil)
	rec := httptest.NewRecorder()
	s.handleGetEmailCandidateOwners(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res struct {
		Owners []string `json:"owners"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	hasAlice, hasCharlie := false, false
	for _, o := range res.Owners {
		if o == "Alice" {
			hasAlice = true
		}
		if o == "Charlie" {
			hasCharlie = true
		}
	}
	if !hasAlice || !hasCharlie {
		t.Fatalf("expected Alice and Charlie in candidate owners, got %v", res.Owners)
	}
}
