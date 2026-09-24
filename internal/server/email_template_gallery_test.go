package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

func galleryCall(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var text string
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		text = string(encoded)
	}
	req := httptest.NewRequest(method, path, strings.NewReader(text))
	response := httptest.NewRecorder()
	switch method {
	case http.MethodGet:
		s.handleListEmailTemplates(response, req)
	case http.MethodPost:
		s.handleCreateEmailTemplates(response, req)
	case http.MethodDelete:
		req.SetPathValue("id", path[strings.LastIndex(path, "/")+1:])
		s.handleDeleteEmailTemplate(response, req)
	default:
		t.Fatal("unsupported gallery test method")
	}
	return response
}
func galleryDTOs(t *testing.T, response *httptest.ResponseRecorder) []emailTemplateCandidateDTO {
	t.Helper()
	var decoded struct {
		Templates []emailTemplateCandidateDTO `json:"templates"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode gallery %s: %v", response.Body.String(), err)
	}
	return decoded.Templates
}
func galleryBody(entries ...emailTemplateCandidateInput) any {
	return map[string]any{"templates": entries}
}

func TestEmailTemplateGalleryPreviewsProjectGroupLayout(t *testing.T) {
	emailTestDatabase(t)
	response := galleryCall(t, &Server{}, "GET", "/api/daily-jira-email/templates", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("list: %s", response.Body.String())
	}
	entries := galleryDTOs(t, response)
	if len(entries) != 4 {
		t.Fatalf("builtins=%d, want 4", len(entries))
	}
	for _, entry := range entries {
		t.Run(entry.Template.Style, func(t *testing.T) {
			for _, marker := range []string{"email-group-summary", "email-group-metrics", "示例交付组", "负责人：示例负责人甲、示例负责人乙"} {
				if !strings.Contains(entry.PreviewHTML, marker) {
					t.Errorf("gallery preview is missing project-group content %q", marker)
				}
			}
			if strings.Contains(entry.PreviewHTML, "email-category-cell\"") {
				t.Error("gallery preview still renders the category fallback instead of project groups")
			}
			if strings.Contains(entry.PreviewHTML, "<a ") {
				t.Error("synthetic gallery data must not invent external issue links")
			}
		})
	}
}

func TestEmailTemplateGalleryCRUDAndIndependentActiveCopy(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	s := &Server{config: &cfg}
	if err := conn.Create(&db.TaskTelemetry{TaskID: "PRIVATE-1", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Title: "DO_NOT_LEAK_ACTUAL_DATA", SourceUpdatedAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	response := galleryCall(t, s, "GET", "/api/daily-jira-email/templates", nil)
	if response.Code != 200 {
		t.Fatalf("list: %s", response.Body.String())
	}
	builtins := galleryDTOs(t, response)
	if len(builtins) != 4 {
		t.Fatalf("builtins=%d", len(builtins))
	}
	for _, entry := range builtins {
		if !entry.Builtin || entry.ID != "builtin-"+entry.Template.Style || entry.CreatedAt != nil {
			t.Fatalf("bad builtin: %+v", entry)
		}
		if !strings.Contains(entry.PreviewHTML, "DEMO-101") || !strings.Contains(entry.PreviewHTML, "示例数据") || strings.Contains(entry.PreviewHTML, "DO_NOT_LEAK_ACTUAL_DATA") {
			t.Fatal("gallery did not isolate demonstration data")
		}
		if r := galleryCall(t, s, "DELETE", "/api/daily-jira-email/templates/"+entry.ID, nil); r.Code != 403 {
			t.Fatal("builtin deletion allowed")
		}
	}
	malicious := config.EmailTemplate{Style: "focus", Subject: "<script>subject</script>", Introduction: "<img src=x onerror=alert(1)>", Closing: "<a href=javascript:evil()>end</a>"}
	response = galleryCall(t, s, "POST", "/api/daily-jira-email/templates", galleryBody(emailTemplateCandidateInput{Name: "自建候选", Template: malicious}))
	if response.Code != 201 {
		t.Fatalf("create %d %s", response.Code, response.Body.String())
	}
	created := galleryDTOs(t, response)
	if len(created) != 1 || created[0].ID == "" || created[0].ID == "0" || created[0].Builtin || created[0].CreatedAt == nil {
		t.Fatalf("bad created DTO: %+v", created)
	}
	if _, err := strconv.ParseUint(created[0].ID, 10, 64); err != nil {
		t.Fatal("custom id must be a numeric string")
	}
	if strings.Contains(created[0].PreviewHTML, "<script>") || strings.Contains(created[0].PreviewHTML, "<img src=x") || !strings.Contains(created[0].PreviewHTML, "&lt;script&gt;") {
		t.Fatal("unescaped template content")
	}
	cfg.DailyJiraEmail.Template = created[0].Template
	if _, err := s.recordConfigVersion(config.Config{}, cfg, httptest.NewRequest("POST", "/api/config", nil), "manual-save", 0); err != nil {
		t.Fatal(err)
	}
	s.setEmailConfig(cfg)
	// A new handler instance reads persisted candidates, never a process-local list.
	if list := galleryDTOs(t, galleryCall(t, &Server{config: &cfg}, "GET", "/api/daily-jira-email/templates", nil)); len(list) != 5 || list[4].ID != created[0].ID {
		t.Fatalf("candidate not persisted: %+v", list)
	}
	response = galleryCall(t, s, "DELETE", "/api/daily-jira-email/templates/"+created[0].ID, nil)
	if response.Code != 200 {
		t.Fatalf("delete: %s", response.Body.String())
	}
	if s.currentEmailConfig().DailyJiraEmail.Template != created[0].Template {
		t.Fatal("deleting library candidate changed active copy")
	}
	var runtime db.RuntimeConfig
	if err := conn.First(&runtime).Error; err != nil {
		t.Fatal(err)
	}
	var restored config.Config
	if err := json.Unmarshal([]byte(runtime.ConfigJSON), &restored); err != nil {
		t.Fatal(err)
	}
	if restored.DailyJiraEmail.Template != created[0].Template {
		t.Fatal("delete changed persisted active copy")
	}
	if list := galleryDTOs(t, galleryCall(t, s, "GET", "/api/daily-jira-email/templates", nil)); len(list) != 4 {
		t.Fatal("delete retained candidate")
	}
	if r := galleryCall(t, s, "DELETE", "/api/daily-jira-email/templates/"+created[0].ID, nil); r.Code != 404 {
		t.Fatal("missing candidate not 404")
	}
	if r := galleryCall(t, s, "DELETE", "/api/daily-jira-email/templates/1%20OR%201=1", nil); r.Code != 400 {
		t.Fatal("invalid id not rejected")
	}
}

func TestEmailTemplateGalleryAtomicBatchValidationAndCapacity(t *testing.T) {
	conn := emailTestDatabase(t)
	s := &Server{}
	valid := emailTemplateCandidateInput{Name: "有效", Template: config.DefaultEmailTemplate()}
	invalid := valid
	invalid.Template.Style = "<script>"
	for _, body := range []any{galleryBody(), galleryBody(valid, valid, valid, valid), galleryBody(valid, invalid), galleryBody(emailTemplateCandidateInput{Name: "\n", Template: valid.Template})} {
		r := galleryCall(t, s, "POST", "/api/daily-jira-email/templates", body)
		if r.Code != 400 {
			t.Fatalf("invalid batch accepted %d: %s", r.Code, r.Body.String())
		}
	}
	var count int64
	if err := conn.Model(&db.EmailTemplateCandidate{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("invalid batch partially saved count=%d err=%v", count, err)
	}
	for i := 0; i < 9; i++ {
		if r := galleryCall(t, s, "POST", "/api/daily-jira-email/templates", galleryBody(valid, valid, valid)); r.Code != 201 {
			t.Fatal(r.Body.String())
		}
	}
	if r := galleryCall(t, s, "POST", "/api/daily-jira-email/templates", galleryBody(valid)); r.Code != 201 {
		t.Fatal(r.Body.String())
	}
	// With two slots free, a three-record batch must not save a partial prefix.
	if r := galleryCall(t, s, "POST", "/api/daily-jira-email/templates", galleryBody(valid, valid, valid)); r.Code != 409 {
		t.Fatalf("oversized batch status=%d", r.Code)
	}
	if err := conn.Model(&db.EmailTemplateCandidate{}).Count(&count).Error; err != nil || count != 28 {
		t.Fatalf("batch partially committed: count=%d err=%v", count, err)
	}
	if r := galleryCall(t, s, "POST", "/api/daily-jira-email/templates", galleryBody(valid, valid)); r.Code != 201 {
		t.Fatal(r.Body.String())
	}
	if r := galleryCall(t, s, "POST", "/api/daily-jira-email/templates", galleryBody(valid, valid)); r.Code != 409 {
		t.Fatalf("capacity returned %d: %s", r.Code, r.Body.String())
	}
	if err := conn.Model(&db.EmailTemplateCandidate{}).Count(&count).Error; err != nil || count != 30 {
		t.Fatalf("capacity violated count=%d err=%v", count, err)
	}
	if list := galleryDTOs(t, galleryCall(t, s, "GET", "/api/daily-jira-email/templates", nil)); len(list) != 34 {
		t.Fatalf("list cap=%d", len(list))
	}
}

func TestEmailTemplateGalleryConcurrentCapacityClaims(t *testing.T) {
	conn := emailTestDatabase(t)
	encoded, _ := json.Marshal(config.DefaultEmailTemplate())
	seed := make([]db.EmailTemplateCandidate, 29)
	for i := range seed {
		seed[i] = db.EmailTemplateCandidate{Name: "已存", TemplateJSON: string(encoded), CreatedAt: time.Now().UTC()}
	}
	if err := appendEmailTemplates(context.Background(), conn, seed); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- appendEmailTemplates(context.Background(), conn, []db.EmailTemplateCandidate{{Name: "并发", TemplateJSON: string(encoded), CreatedAt: time.Now().UTC()}})
		}()
	}
	wg.Wait()
	close(results)
	successes, full := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, errEmailTemplateGalleryFull) {
			full++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || full != 1 {
		t.Fatalf("success=%d full=%d", successes, full)
	}
	var count int64
	conn.Model(&db.EmailTemplateCandidate{}).Count(&count)
	if count != 30 {
		t.Fatalf("concurrent writes exceeded capacity: %d", count)
	}
}

func TestEmailTemplateGalleryUsesGlobalPermissions(t *testing.T) {
	setupServerTestDB(t)
	cfg := config.Config{}
	s := &Server{config: &cfg, mux: http.NewServeMux()}
	s.routes()
	scoped := seedPolicyTestUser(t, "scoped-gallery@example.test", "admin", "repo", "X")
	global := superAdminToken(t, "global-gallery@example.test", "Gallery Admin", []string{"config:read", "config:write"})
	routes := []struct{ method, path string }{{"GET", "/api/daily-jira-email/templates"}, {"POST", "/api/daily-jira-email/templates"}, {"DELETE", "/api/daily-jira-email/templates/123"}}
	for _, route := range routes {
		for _, query := range []string{"?repo=X", "?project_id=X"} {
			r := httptest.NewRequest(route.method, route.path+query, strings.NewReader(`{"templates":[]}`))
			r.Header.Set("Authorization", "Bearer "+scoped)
			response := httptest.NewRecorder()
			s.mux.ServeHTTP(response, r)
			if response.Code != 403 {
				t.Fatalf("scoped identity accessed %s %s: %d", route.method, route.path+query, response.Code)
			}
		}
	}
	req := httptest.NewRequest("GET", "/api/daily-jira-email/templates", nil)
	req.Header.Set("Authorization", "Bearer "+global)
	response := httptest.NewRecorder()
	s.mux.ServeHTTP(response, req)
	if response.Code != 200 {
		t.Fatalf("global permission rejected: %d %s", response.Code, response.Body.String())
	}
}

func TestEmailTemplateGalleryReadOnlyPermissionCannotWrite(t *testing.T) {
	setupServerTestDB(t)
	username := "gallery-reader@example.test"
	seedPolicyTestUser(t, username, "member", "global", "")
	if err := db.DB.Create(&userdb.AuthorizationPolicy{Effect: "allow", SubjectType: "user", SubjectID: username, Action: "config:read", ResourceType: "config", Scope: "global", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	// Even misleading JWT permission claims cannot replace database authorization.
	token, err := GenerateJWT(username, "Reader", "", "", []string{"member"}, []string{"config:read", "config:write"})
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{config: &config.Config{}, mux: http.NewServeMux()}
	s.routes()
	for _, method := range []string{"GET", "POST", "DELETE"} {
		path := "/api/daily-jira-email/templates"
		if method == "DELETE" {
			path += "/1"
		}
		req := httptest.NewRequest(method, path, strings.NewReader(`{"templates":[]}`))
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		s.mux.ServeHTTP(response, req)
		want := 403
		if method == "GET" {
			want = 200
		}
		if response.Code != want {
			t.Fatalf("read-only %s status=%d want=%d: %s", method, response.Code, want, response.Body.String())
		}
	}
}

func TestGenerateEmailTemplateReturnsOneCompleteCandidate(t *testing.T) {
	emailTestDatabase(t)
	calls := 0
	s := &Server{}
	s.emailLLM = func(context.Context, string, string) (string, error) {
		calls++
		return `{"style":"custom","subject":"AI日报 {{date}}","introduction":"已生成文案","closing":"请核对","html":"<div>{{introduction}}</div><div>{{closing}}</div><span>{{date}}</span><span>{{timezone}}</span><div>{{yesterday}}</div><div>{{unresolved}}</div>"}`, nil
	}
	request := httptest.NewRequest("POST", "/api/daily-jira-email/template", strings.NewReader(`{"requirements":"简洁"}`))
	response := httptest.NewRecorder()
	s.handleGenerateEmailTemplate(response, request)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var decoded struct {
		Template   config.EmailTemplate          `json:"template"`
		Candidates []emailTemplateCandidateInput `json:"candidates"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(decoded.Candidates) != 1 || decoded.Template.Style != "custom" {
		t.Fatalf("bad generation: calls=%d response=%+v", calls, decoded)
	}
	if decoded.Candidates[0].Name != "Agent 自定义整版模板" {
		t.Fatalf("unexpected candidate name: %s", decoded.Candidates[0].Name)
	}
	if decoded.Candidates[0].Template.Style != "custom" || decoded.Candidates[0].Template.Subject != decoded.Template.Subject {
		t.Fatal("candidate content/style drift")
	}
	var count int64
	db.DB.Model(&db.EmailTemplateCandidate{}).Count(&count)
	if count != 0 {
		t.Fatal("generation silently saved candidates")
	}
}

func TestCustomEmailTemplateInjectsReportSections(t *testing.T) {
	html := `<div><h1>自定义整版</h1><p>{{introduction}}</p>{{charts}}{{overview_cards}}{{analysis}}{{yesterday}}{{unresolved}}{{owners}}{{commits}}{{warnings}}<p>{{closing}}</p><span>{{date}}</span><span>{{timezone}}</span></div>`
	report, err := emailTemplateDemo(config.EmailTemplate{
		Style:        "custom",
		Subject:      "自定义模板 {{date}}",
		Introduction: "开场",
		Closing:      "结尾",
		HTML:         html,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"自定义整版",
		"DEMO-101",
		"昨日更新 Jira",
		"近3天创建且未解决",
		"示例交付组",
		"昨日 commit 统计分析",
		"示例数据 · 此预览不会查询实际事项或发送邮件。",
	} {
		if !strings.Contains(report.HTML, want) {
			t.Fatalf("custom template missing %s", want)
		}
	}
	for _, placeholder := range []string{"{{introduction}}", "{{closing}}", "{{date}}", "{{timezone}}", "{{yesterday}}", "{{unresolved}}", "{{owners}}", "{{commits}}", "{{charts}}", "{{overview_cards}}", "{{analysis}}", "{{warnings}}"} {
		if strings.Contains(report.HTML, placeholder) {
			t.Fatalf("custom template left placeholder %s", placeholder)
		}
	}
}
