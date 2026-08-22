package server

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
	"well-ambient/internal/solutions"
	"well-ambient/internal/telemetry"
)

func TestSolutionWorkerCreatesCandidateWithoutOverwritingDraftAndPublishesJiraLinkOnce(t *testing.T) {
	setupServerTestDB(t)
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID: "WA-901", ProjectKey: "WA", Title: "异步方案", Description: "保留人工事实",
		IssueType: "requirement", Status: "backlog", LastUpdate: time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}
	srv := NewServer(&config.Config{
		Server: config.ServerConfig{PublicURL: "https://ambient.example.com/base"},
		AI:     config.AIConfig{Model: "solution-test-model"},
	}, "")
	workspace, err := srv.ensureSolutionDraftForTask(context.Background(), db.TaskTelemetry{
		TaskID: "WA-901", Title: "异步方案", Description: "保留人工事实",
	}, "Alice")
	if err != nil {
		t.Fatalf("ensure solution: %v", err)
	}
	beforeID := workspace.Working.ID
	if _, _, err := srv.solutions.RequestPolish(context.Background(), solutions.RequestPolishCommand{
		DemandID: "WA-901", ProjectKey: "WA", RequestedBy: "Alice",
	}); err != nil {
		t.Fatalf("request polish: %v", err)
	}
	var llmInput string
	var llmSystemPrompt string
	srv.solutionLLM = func(_ context.Context, systemPrompt, userPrompt string) (string, error) {
		llmSystemPrompt = systemPrompt
		llmInput = userPrompt
		return "# 异步方案\n\n保留人工事实\n\n## 验收\n\n- 可回滚", nil
	}
	processed, err := srv.processOneSolutionJob(context.Background())
	if err != nil || !processed {
		t.Fatalf("process polish: processed=%v err=%v", processed, err)
	}
	if !strings.Contains(llmInput, "<current_markdown>") || !strings.Contains(llmInput, "保留人工事实") {
		t.Fatalf("LLM input did not bind the current draft: %s", llmInput)
	}
	if !strings.Contains(llmSystemPrompt, "平台不可覆盖安全边界") || !strings.Contains(llmSystemPrompt, "不可信资料") {
		t.Fatalf("platform source safety boundary missing: %s", llmSystemPrompt)
	}
	workspace, err = srv.solutions.GetWorkspace(context.Background(), "WA-901")
	if err != nil || workspace.Working.ID != beforeID || len(workspace.Candidates) != 1 {
		t.Fatalf("candidate overwrote working draft: workspace=%+v err=%v", workspace, err)
	}
	workspace, err = srv.solutions.ApplyCandidate(context.Background(), solutions.ApplyCandidateCommand{
		DemandID: "WA-901", CandidateID: workspace.Candidates[0].ID,
		ExpectedRevision: workspace.Asset.Revision, Actor: "Alice",
	})
	if err != nil {
		t.Fatalf("apply candidate: %v", err)
	}
	link := solutionPublicLink(srv.config.Server.PublicURL, "WA-901")
	workspace, err = srv.solutions.Publish(context.Background(), solutions.PublishCommand{
		DemandID: "WA-901", ExpectedRevision: workspace.Asset.Revision, Actor: "Alice", Link: link,
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	var jiraComments []string
	srv.solutionJiraPost = func(issueKey, comment string) error {
		if issueKey != "WA-901" {
			t.Fatalf("unexpected issue key: %s", issueKey)
		}
		jiraComments = append(jiraComments, comment)
		return nil
	}
	processed, err = srv.processOneSolutionOutbox(context.Background())
	if err != nil || !processed {
		t.Fatalf("process outbox: processed=%v err=%v", processed, err)
	}
	processed, err = srv.processOneSolutionOutbox(context.Background())
	if err != nil || processed {
		t.Fatalf("outbox was not idempotently drained: processed=%v err=%v", processed, err)
	}
	if len(jiraComments) != 1 || !strings.Contains(jiraComments[0], "https://ambient.example.com/base/?demand=WA-901&solution=1&tab=schedule") || !strings.Contains(jiraComments[0], "WELL_AMBIENT_SOLUTION_LINK") {
		t.Fatalf("Jira publication comment = %#v", jiraComments)
	}
}

func TestSolutionPromptRequiresGlobalSuperAdminInAdditionToPermission(t *testing.T) {
	setupServerTestDB(t)
	srv := NewServer(&config.Config{}, "")
	var permission userdb.Permission
	if err := db.DB.Where("code = ?", "solution_prompt:manage").First(&permission).Error; err != nil {
		t.Fatalf("load permission: %v", err)
	}
	group := userdb.UserGroup{Name: "prompt_operator", DisplayName: "提示词操作员"}
	if err := db.DB.Create(&group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	user := userdb.User{Username: "operator@example.com", Email: "operator@example.com", Name: "Operator"}
	if err := db.DB.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.DB.Create(&userdb.UserGroupMembership{UserID: user.ID, UserGroupID: group.ID, Scope: "global"}).Error; err != nil {
		t.Fatalf("bind group: %v", err)
	}
	if err := db.DB.Create(&userdb.GroupPermission{UserGroupID: group.ID, PermissionID: permission.ID}).Error; err != nil {
		t.Fatalf("bind permission: %v", err)
	}
	token, err := GenerateJWT(user.Username, user.Name, "", "", []string{group.Name}, []string{permission.Code})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/solution-prompts", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	srv.mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "global super administrator required") {
		t.Fatalf("permission-only operator status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	superToken := superAdminToken(t, "root@example.com", "Root", []string{"solution_prompt:manage"})
	request = httptest.NewRequest(http.MethodGet, "/api/solution-prompts", nil)
	request.Header.Set("Authorization", "Bearer "+superToken)
	recorder = httptest.NewRecorder()
	srv.mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "资深软件方案编辑") {
		t.Fatalf("super admin status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSolutionPublicLinkUsesConfiguredOriginAndEscapesDemand(t *testing.T) {
	got := solutionPublicLink("https://ambient.example.com/app", "WA 1/2")
	if got != "https://ambient.example.com/app/?demand=WA+1%2F2&solution=1&tab=schedule" {
		t.Fatalf("public link = %q", got)
	}
}

func TestSolutionPublicLinkForRequestPrefersConfiguredPublicURL(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/solutions/publish", nil)
	request.Header.Set("Origin", "https://request.example.com")
	got := solutionPublicLinkForRequest("https://configured.example.com/app", request, "WA-1")
	if got != "https://configured.example.com/app/?demand=WA-1&solution=1&tab=schedule" {
		t.Fatalf("public link = %q", got)
	}
}

func TestBrowserRequestOriginRejectsNonOriginValues(t *testing.T) {
	tests := []string{
		"javascript:alert(1)",
		"https://user@example.com",
		"https://ambient.example.com/app",
		"https://ambient.example.com?next=1",
		"https://ambient.example.com, https://evil.example.com",
	}
	for _, value := range tests {
		request := httptest.NewRequest(http.MethodPost, "/api/solutions/publish", nil)
		request.Header.Set("Origin", value)
		if got := browserRequestOrigin(request); got != "" {
			t.Fatalf("browserRequestOrigin(%q) = %q", value, got)
		}
	}
}

func TestPublishSolutionRejectsRelativeJiraLink(t *testing.T) {
	setupServerTestDB(t)
	if err := db.DB.Create(&db.TaskTelemetry{TaskID: "WA-902", Title: "缺少公网地址", LastUpdate: time.Now()}).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}
	srv := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: true}}, "")
	workspace, err := srv.solutions.EnsureDraft(context.Background(), solutions.EnsureDraftCommand{DemandID: "WA-902", Markdown: "# 方案", Actor: "Root"})
	if err != nil {
		t.Fatalf("ensure draft: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/solutions/publish", strings.NewReader(fmt.Sprintf(`{"demand_id":"WA-902","expected_revision":%d}`, workspace.Asset.Revision)))
	recorder := httptest.NewRecorder()
	srv.handlePublishSolution(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(recorder.Body.String(), "server.public_url") {
		t.Fatalf("publish status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	workspace, err = srv.solutions.GetWorkspace(context.Background(), "WA-902")
	if err != nil || workspace.Working == nil || workspace.Working.Status != solutions.StatusDraft {
		t.Fatalf("rejected publication changed draft: workspace=%+v err=%v", workspace, err)
	}
	var outboxCount int64
	if err := db.DB.Model(&db.SolutionJiraOutbox{}).Count(&outboxCount).Error; err != nil || outboxCount != 0 {
		t.Fatalf("rejected publication outbox count=%d err=%v", outboxCount, err)
	}
}

func TestPublishSolutionUsesBrowserOriginWhenPublicURLIsNotConfigured(t *testing.T) {
	setupServerTestDB(t)
	if err := db.DB.Create(&db.TaskTelemetry{TaskID: "WA-905", Title: "浏览器来源发布", LastUpdate: time.Now()}).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}
	srv := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: true}}, "")
	workspace, err := srv.solutions.EnsureDraft(context.Background(), solutions.EnsureDraftCommand{DemandID: "WA-905", Markdown: "# 方案", Actor: "Root"})
	if err != nil {
		t.Fatalf("ensure draft: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/solutions/publish", strings.NewReader(fmt.Sprintf(`{"demand_id":"WA-905","expected_revision":%d}`, workspace.Asset.Revision)))
	request.Header.Set("Origin", "https://ambient.example.com")
	recorder := httptest.NewRecorder()
	srv.handlePublishSolution(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("publish status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Link string `json:"link"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode publish response: %v", err)
	}
	if response.Link != "https://ambient.example.com/?demand=WA-905&solution=1&tab=schedule" {
		t.Fatalf("publish link=%q", response.Link)
	}
	var outbox db.SolutionJiraOutbox
	if err := db.DB.Where("demand_id = ?", "WA-905").First(&outbox).Error; err != nil {
		t.Fatalf("load Jira outbox: %v", err)
	}
	var payload struct {
		Link string `json:"link"`
	}
	if err := json.Unmarshal([]byte(outbox.PayloadJSON), &payload); err != nil {
		t.Fatalf("decode Jira outbox payload: %v", err)
	}
	if outbox.Status != "pending" || payload.Link != response.Link {
		t.Fatalf("Jira outbox=%+v", outbox)
	}
}

func TestSolutionResponsesUseHTTPGzipWhenAccepted(t *testing.T) {
	largeBody := strings.Repeat("完整方案内容", 1200)
	handler := withSolutionCompression(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{"markdown":%q}`, largeBody)))
	})
	request := httptest.NewRequest(http.MethodGet, "/api/solutions/workspace", nil)
	request.Header.Set("Accept-Encoding", "br, gzip")
	recorder := httptest.NewRecorder()
	handler(recorder, request)
	if recorder.Header().Get("Content-Encoding") != "gzip" || !strings.Contains(recorder.Header().Get("Vary"), "Accept-Encoding") {
		t.Fatalf("compression headers = %#v", recorder.Header())
	}
	reader, err := gzip.NewReader(recorder.Body)
	if err != nil {
		t.Fatalf("open gzip body: %v", err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	if !strings.Contains(string(decoded), largeBody) || recorder.Body.Len() >= len(decoded) {
		t.Fatalf("response was not losslessly compressed: compressed=%d decoded=%d", recorder.Body.Len(), len(decoded))
	}
}

func TestEnsureSolutionJiraCommentSkipsExistingStableMarker(t *testing.T) {
	reads := 0
	srv := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: true}}, "")
	srv.solutionJiraRead = func(issueKey string) ([]telemetry.JiraComment, error) {
		reads++
		if issueKey != "WA-903" {
			t.Fatalf("unexpected issue key: %s", issueKey)
		}
		return []telemetry.JiraComment{{ID: "7", Body: "<!-- WELL_AMBIENT_SOLUTION_LINK:42 -->\n方案 v3 已发布"}}, nil
	}
	err := srv.ensureSolutionJiraComment("WA-903", "<!-- WELL_AMBIENT_SOLUTION_LINK:42 -->", "duplicate")
	if err != nil || reads != 1 {
		t.Fatalf("deduplicate marker: reads=%d err=%v", reads, err)
	}
}

func TestRetryFailedSolutionJobQueuesOneAuditableReplacement(t *testing.T) {
	setupServerTestDB(t)
	srv := NewServer(&config.Config{}, "")
	ctx := context.Background()
	if _, err := srv.solutions.ObserveSource(ctx, solutions.ObserveSourceCommand{
		DemandID: "WA-904", SourceSystem: "jira", ExternalID: "comment-524", Author: "PM",
		Marker: "[方案]", Eligible: true, Body: "[方案]\n重新生成方案",
	}); err != nil {
		t.Fatalf("observe source: %v", err)
	}
	failed, _, err := srv.solutions.RequestInitialDraft(ctx, solutions.RequestInitialDraftCommand{
		DemandID: "WA-904", ProjectKey: "WA", Title: "失败方案", Markdown: "# 失败方案",
		RequestedBy: "jira-sync",
	})
	if err != nil {
		t.Fatalf("request initial draft: %v", err)
	}
	providerError := `LLM provider returned status 524: {"title":"Error 524: A timeout occurred","retryable":true}`
	if err := db.DB.Model(&db.SolutionPolishJob{}).Where("id = ?", failed.ID).Updates(map[string]any{
		"status": solutions.JobFailed, "attempt_count": 3, "last_error": providerError,
	}).Error; err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	token := superAdminToken(t, "solution-retry@example.com", "Solution Retry", []string{"solution:write"})
	path := fmt.Sprintf("/api/solutions/jobs/%d/retry", failed.ID)

	first := authenticatedJSONRequest(t, srv, token, http.MethodPost, path, map[string]any{"demand_id": "WA-904"})
	if first.Code != http.StatusAccepted || !strings.Contains(first.Body.String(), `"status":"queued"`) || !strings.Contains(first.Body.String(), `"workspace"`) {
		t.Fatalf("first retry status/body = %d/%s", first.Code, first.Body.String())
	}
	second := authenticatedJSONRequest(t, srv, token, http.MethodPost, path, map[string]any{"demand_id": "WA-904"})
	if second.Code != http.StatusOK || !strings.Contains(second.Body.String(), `"replayed":true`) {
		t.Fatalf("replayed retry status/body = %d/%s", second.Code, second.Body.String())
	}
	var count int64
	if err := db.DB.Model(&db.SolutionPolishJob{}).Where("solution_asset_id = ?", failed.SolutionAssetID).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("manual retry job count=%d err=%v", count, err)
	}
	var preserved db.SolutionPolishJob
	if err := db.DB.First(&preserved, failed.ID).Error; err != nil || preserved.Status != solutions.JobFailed || preserved.LastError != providerError {
		t.Fatalf("failed job audit changed: job=%+v err=%v", preserved, err)
	}
}
