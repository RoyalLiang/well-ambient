package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"well-ambient/internal/codereview"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/decisioncommands"
	"well-ambient/internal/openaccess"
)

type openHTTPJiraFake struct {
	mu          sync.Mutex
	state       decisioncommands.JiraState
	assignCalls int
}

func (f *openHTTPJiraFake) ReadIssueState(
	context.Context,
	db.JiraExecutionBinding,
	string,
) (decisioncommands.JiraState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state, nil
}

func (f *openHTTPJiraFake) Assign(
	_ context.Context,
	_ db.JiraExecutionBinding,
	_ string,
	assignee string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.assignCalls++
	f.state.Assignee = assignee
	f.state.UpdatedAt = f.state.UpdatedAt.Add(time.Second)
	f.state.EvidenceRef = "jira:confirmed"
	return nil
}

func (f *openHTTPJiraFake) SetDueDate(
	_ context.Context,
	_ db.JiraExecutionBinding,
	_ string,
	dueDate string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state.DueDate = dueDate
	f.state.UpdatedAt = f.state.UpdatedAt.Add(time.Second)
	f.state.EvidenceRef = "jira:confirmed"
	return nil
}

func setupOpenCapabilityServer(t *testing.T) (*Server, openaccess.IssuedCredential, openaccess.Policy) {
	t.Helper()
	t.Setenv("WELL_AMBIENT_OPEN_READ_ENABLED", "1")
	t.Setenv("WELL_AMBIENT_OPEN_PREPARE_ENABLED", "1")
	t.Setenv("WELL_AMBIENT_OPEN_EXECUTE_ENABLED", "1")
	setupServerTestDB(t)
	srv := NewServer(&config.Config{}, "")
	ctx := context.Background()
	if _, err := srv.openAccess.CreateSource(ctx, "trusted-agent", "Trusted Agent", "AI Platform", "standard"); err != nil {
		t.Fatal(err)
	}
	policy, err := srv.openAccess.ActivatePolicy(ctx, openaccess.PolicySpec{
		AllowedProjects:     []string{"WA"},
		AllowedRepositories: []string{"1"},
		Actions: []string{
			"jira.read", "review.read", "decision.read", "decision.prepare", "decision.execute",
			"decision.reassign", "decision.reschedule",
		},
		DataRules: map[string]string{
			"jira_issue_visibility": "verified_cache",
		},
		FieldRules: map[string][]string{
			"jira_describe_schema": {
				"key", "project", "summary", "issue_type", "status", "assignee",
				"priority", "bug_category", "due_date", "created_at", "updated_at", "history_complete",
			},
			"jira_search_issues": {
				"key", "project", "summary", "issue_type", "status", "assignee",
				"priority", "bug_category", "due_date", "created_at", "updated_at", "history_complete",
			},
			"jira_get_issue": {
				"key", "project", "summary", "issue_type", "status", "assignee",
				"priority", "bug_category", "due_date", "created_at", "updated_at", "history_complete", "history",
			},
			"jira_aggregate_issues": {
				"key", "project", "issue_type", "status", "assignee", "priority",
				"bug_category", "due_date", "updated_at", "history_complete",
			},
			"review_search": {
				"run_id", "repository", "kind", "ref", "base_sha", "head_sha", "title",
				"run_status", "reviewed_at", "coverage_gaps", "evidence_complete",
				"findings_count", "code_freshness",
			},
			"review_get": {
				"run_id", "repository", "kind", "ref", "base_sha", "head_sha", "title",
				"run_status", "reviewed_at", "coverage_gaps", "evidence_complete",
				"findings_count", "code_freshness", "overview", "scenario", "validation",
				"questions", "findings", "evidence_refs",
			},
		},
		ApprovalRules: map[string]string{
			decisioncommands.ActionReassign:   "auto",
			decisioncommands.ActionReschedule: "auto",
		},
		QueryLimits: openaccess.QueryLimits{
			MaxPageSize: 50, MaxScanRows: 1000, MaxGroupBy: 2, MaxTimeBuckets: 30,
		},
	}, "test-admin")
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{decisioncommands.ActionReassign, decisioncommands.ActionReschedule} {
		if err := srv.openAccess.UpsertExecutionBinding(ctx, db.JiraExecutionBinding{
			ProjectRef: "WA", ActionClass: action, ConnectorRef: "jira-primary",
			ExecutorRef: "jira-service", Status: openaccess.SourceActive,
		}); err != nil {
			t.Fatal(err)
		}
	}
	credential, err := srv.openAccess.IssueCredential(ctx, "trusted-agent", 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return srv, credential, policy
}

func openRequest(t *testing.T, srv *Server, method, path, secret, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if secret != "" {
		request.Header.Set("Authorization", "Bearer "+secret)
	}
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.handler.ServeHTTP(recorder, request)
	return recorder
}

func TestOpenCapabilityAuthPolicyAndInvocationAudit(t *testing.T) {
	srv, credential, _ := setupOpenCapabilityServer(t)
	now := time.Date(2026, time.September, 27, 10, 0, 0, 0, time.UTC)
	if err := db.DB.Create(&[]db.TaskTelemetry{
		{
			TaskID: "WA-1", ProjectKey: "WA", Source: "jira", Title: "Published",
			IssueType: "bug", Status: "progress", Assignee: "Alice",
			SourceUpdatedAt: now, LastUpdate: now, JiraHistoryComplete: true,
		},
		{
			TaskID: "SECRET-1", ProjectKey: "SECRET", Source: "jira", Title: "Restricted",
			IssueType: "bug", Status: "progress", SourceUpdatedAt: now.Add(time.Hour),
		},
	}).Error; err != nil {
		t.Fatal(err)
	}

	noAuth := openRequest(t, srv, http.MethodGet, "/open/v1/jira/schema?token="+credential.Secret, "", "")
	if noAuth.Code != http.StatusUnauthorized {
		t.Fatalf("URL credential status = %d body=%s", noAuth.Code, noAuth.Body.String())
	}

	body := `{"filters":{"projects":["WA"]},"fields":["key","summary"],"limit":10}`
	request := httptest.NewRequest(http.MethodPost, "/open/v1/jira/issues/search", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+credential.Secret)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-authenticated-user-id", "spoofed-user")
	request.Header.Set("x-integration-source", "spoofed-source")
	recorder := httptest.NewRecorder()
	srv.handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("search status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 1 || response.Items[0]["key"] != "WA-1" {
		t.Fatalf("search response = %+v", response)
	}
	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("request id header is missing")
	}

	var invocation db.CapabilityInvocation
	if err := db.DB.Order("id DESC").First(&invocation).Error; err != nil {
		t.Fatal(err)
	}
	if invocation.SourceID != "trusted-agent" || invocation.ToolName != "jira_search_issues" ||
		invocation.Outcome != "succeeded" {
		t.Fatalf("invocation = %+v", invocation)
	}

	forbidden := openRequest(t, srv, http.MethodPost, "/open/v1/jira/issues/search", credential.Secret,
		`{"filters":{"projects":["SECRET"]},"fields":["key"]}`)
	if forbidden.Code != http.StatusForbidden || !strings.Contains(forbidden.Body.String(), `"code":"forbidden"`) {
		t.Fatalf("forbidden response = %d %s", forbidden.Code, forbidden.Body.String())
	}

	schema := openRequest(t, srv, http.MethodGet, "/open/v1/jira/schema", credential.Secret, "")
	if schema.Code != http.StatusOK || schema.Header().Get("X-Well-Ambient-Read-Contract") != "open-jira-schema" {
		t.Fatalf("schema read contract = %d headers=%v body=%s", schema.Code, schema.Header(), schema.Body.String())
	}

	if err := srv.openAccess.RevokeCredential(context.Background(), credential.KeyID); err != nil {
		t.Fatal(err)
	}
	revoked := openRequest(t, srv, http.MethodGet, "/open/v1/jira/schema", credential.Secret, "")
	if revoked.Code != http.StatusUnauthorized {
		t.Fatalf("revoked credential status = %d body=%s", revoked.Code, revoked.Body.String())
	}
}

func TestOpenCapabilitiesRemainDisabledUntilDeploymentFlagsAreEnabled(t *testing.T) {
	t.Setenv("WELL_AMBIENT_OPEN_READ_ENABLED", "")
	t.Setenv("WELL_AMBIENT_OPEN_PREPARE_ENABLED", "")
	t.Setenv("WELL_AMBIENT_OPEN_EXECUTE_ENABLED", "")
	setupServerTestDB(t)
	srv := NewServer(&config.Config{}, "")
	if _, err := srv.openAccess.CreateSource(context.Background(), "disabled-source", "Disabled Source", "", "standard"); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.openAccess.ActivatePolicy(context.Background(), openaccess.PolicySpec{
		AllowedProjects: []string{"WA"},
		Actions:         []string{"jira.read"},
		DataRules: map[string]string{
			"jira_issue_visibility": "verified_cache",
		},
		FieldRules: map[string][]string{
			"jira_describe_schema": {"key"},
		},
	}, "test"); err != nil {
		t.Fatal(err)
	}
	credential, err := srv.openAccess.IssueCredential(context.Background(), "disabled-source", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	response := openRequest(t, srv, http.MethodGet, "/open/v1/jira/schema", credential.Secret, "")
	if response.Code != http.StatusForbidden ||
		!strings.Contains(response.Body.String(), "open read capabilities are disabled") {
		t.Fatalf("disabled feature response = %d %s", response.Code, response.Body.String())
	}
}

func TestOpenExecuteDisablesLegacyJiraWriteOwners(t *testing.T) {
	setupServerTestDB(t)
	srv := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: true}}, "")
	srv.openFeatures.Execute = true
	var legacyCalls int
	srv.jiraAssigneeSync = func(string, string) { legacyCalls++ }
	srv.jiraDailyReviewSync = func(string, *string, string) error {
		legacyCalls++
		return nil
	}

	intervention := httptest.NewRequest(
		http.MethodPost,
		"/api/strongest-brain/intervention",
		strings.NewReader(`{"task_id":"WA-900","action":"reassign","value":"Bob"}`),
	)
	interventionRecorder := httptest.NewRecorder()
	srv.handleStrongestBrainIntervention(interventionRecorder, intervention)
	if interventionRecorder.Code != http.StatusConflict ||
		!strings.Contains(interventionRecorder.Body.String(), "legacy_sender_disabled") {
		t.Fatalf("legacy intervention response = %d %s", interventionRecorder.Code, interventionRecorder.Body.String())
	}

	dailyReview := httptest.NewRequest(
		http.MethodPost,
		"/api/decision/daily-jira/review",
		strings.NewReader(`{"task_id":"WA-900","decision":"reassign","assignee":"Bob","assignee_mode":"specified","note":"move"}`),
	)
	dailyRecorder := httptest.NewRecorder()
	srv.handlePostDailyJiraReview(dailyRecorder, dailyReview)
	if dailyRecorder.Code != http.StatusConflict ||
		!strings.Contains(dailyRecorder.Body.String(), "legacy_sender_disabled") {
		t.Fatalf("legacy daily review response = %d %s", dailyRecorder.Code, dailyRecorder.Body.String())
	}
	if legacyCalls != 0 {
		t.Fatalf("legacy Jira sender calls = %d, want 0", legacyCalls)
	}
}

func TestOpenReviewDTOAndDecisionOperationLifecycle(t *testing.T) {
	srv, credential, _ := setupOpenCapabilityServer(t)
	now := time.Date(2026, time.September, 27, 10, 0, 0, 0, time.UTC)
	report, _ := json.Marshal(codereview.Report{
		EvidenceComplete: true, Summary: "validated", Questions: []string{},
		Findings: []codereview.Finding{{
			Dimension: "security", Severity: "high", Title: "Unsafe log",
			File: "auth.go", Line: 12, Evidence: "log(secret)", Impact: "leak",
			Suggestion: "redact", Verification: "run test",
		}},
	})
	run := db.CodeReviewRun{
		Key: "open-review", ProjectID: "1", Repo: "platform/api", Kind: "mr", Ref: "42",
		HeadSHA: "head-1", BaseSHA: "base-1", Status: "completed",
		Error: "internal stack", PolicyJSON: `{"secret":"hidden"}`, ReportJSON: string(report),
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.DB.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	review := openRequest(t, srv, http.MethodGet,
		fmt.Sprintf("/open/v1/reviews/%d?current_head_sha=head-1", run.ID),
		credential.Secret, "",
	)
	if review.Code != http.StatusOK {
		t.Fatalf("review status = %d body=%s", review.Code, review.Body.String())
	}
	for _, forbidden := range []string{"internal stack", `"secret":"hidden"`, "policy_json", "report_json"} {
		if strings.Contains(review.Body.String(), forbidden) {
			t.Fatalf("review leaked %q: %s", forbidden, review.Body.String())
		}
	}
	if !strings.Contains(review.Body.String(), `"code_freshness":"matches_current_head"`) {
		t.Fatalf("review freshness missing: %s", review.Body.String())
	}

	fake := &openHTTPJiraFake{state: decisioncommands.JiraState{
		Assignee: "Alice", DueDate: "2026-09-30", UpdatedAt: now, EvidenceRef: "jira:before",
	}}
	srv.decisionCommands = decisioncommands.New(db.DB, srv.openAccess, fake)
	assigneeBody := `{"issue_key":"WA-9","assignee":"Bob"}`
	prepared := openRequest(t, srv, http.MethodPost, "/open/v1/decisions/plans", credential.Secret, assigneeBody)
	if prepared.Code != http.StatusCreated {
		t.Fatalf("prepare status = %d body=%s", prepared.Code, prepared.Body.String())
	}
	var plan decisioncommands.Plan
	if err := json.Unmarshal(prepared.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	executeBody := fmt.Sprintf(`{"plan_id":%q,"idempotency_key":"http-decision-1"}`, plan.ID)
	accepted := openRequest(t, srv, http.MethodPost, "/open/v1/decisions/executions", credential.Secret, executeBody)
	if accepted.Code != http.StatusAccepted {
		t.Fatalf("execute status = %d body=%s", accepted.Code, accepted.Body.String())
	}
	var operation decisioncommands.Operation
	if err := json.Unmarshal(accepted.Body.Bytes(), &operation); err != nil {
		t.Fatal(err)
	}
	if processed, err := srv.decisionCommands.ProcessNext(context.Background()); err != nil || !processed {
		t.Fatalf("process operation = %v, %v", processed, err)
	}
	completed := openRequest(t, srv, http.MethodGet, "/open/v1/operations/"+operation.ID, credential.Secret, "")
	if completed.Code != http.StatusOK ||
		!strings.Contains(completed.Body.String(), `"state":"succeeded"`) ||
		!strings.Contains(completed.Body.String(), `"confirmed_remote_value":"Bob"`) {
		t.Fatalf("completed operation = %d %s", completed.Code, completed.Body.String())
	}
	if fake.assignCalls != 1 {
		t.Fatalf("Jira assign calls = %d, want 1", fake.assignCalls)
	}

}

func TestOpenMCPListsTenToolsAndCallsSharedJiraModule(t *testing.T) {
	srv, credential, _ := setupOpenCapabilityServer(t)
	now := time.Date(2026, time.September, 27, 10, 0, 0, 0, time.UTC)
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID: "WA-88", ProjectKey: "WA", Source: "jira", Title: "MCP issue",
		IssueType: "bug", Status: "progress", Assignee: "Alice",
		SourceUpdatedAt: now, LastUpdate: now, JiraHistoryComplete: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	mcpRequest := func(body, secret string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		request.Header.Set("MCP-Protocol-Version", "2026-07-28")
		var envelope struct {
			Method string `json:"method"`
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		if json.Unmarshal([]byte(body), &envelope) == nil && envelope.Method != "" {
			request.Header.Set("Mcp-Method", envelope.Method)
			if envelope.Params.Name != "" {
				request.Header.Set("Mcp-Name", envelope.Params.Name)
			}
		}
		if secret != "" {
			request.Header.Set("Authorization", "Bearer "+secret)
		}
		recorder := httptest.NewRecorder()
		srv.handler.ServeHTTP(recorder, request)
		return recorder
	}
	unauthenticated := mcpRequest(
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"well-ambient-test","version":"1.0.0"},"io.modelcontextprotocol/clientCapabilities":{}}}}`,
		"",
	)
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated MCP status = %d body=%s", unauthenticated.Code, unauthenticated.Body.String())
	}
	listed := mcpRequest(
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"well-ambient-test","version":"1.0.0"},"io.modelcontextprotocol/clientCapabilities":{}}}}`,
		credential.Secret,
	)
	if listed.Code != http.StatusOK {
		t.Fatalf("tools/list status = %d body=%s", listed.Code, listed.Body.String())
	}
	var listResponse struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &listResponse); err != nil {
		t.Fatal(err)
	}
	if len(listResponse.Result.Tools) != 10 {
		t.Fatalf("tools/list count = %d body=%s", len(listResponse.Result.Tools), listed.Body.String())
	}

	called := mcpRequest(
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"well-ambient-test","version":"1.0.0"},"io.modelcontextprotocol/clientCapabilities":{}},"name":"jira_search_issues","arguments":{"filters":{"projects":["WA"]},"fields":["key","summary"],"limit":10}}}`,
		credential.Secret,
	)
	if called.Code != http.StatusOK ||
		!strings.Contains(called.Body.String(), `"WA-88"`) ||
		!strings.Contains(called.Body.String(), `"structuredContent"`) {
		t.Fatalf("tools/call response = %d %s", called.Code, called.Body.String())
	}
	var invocations int64
	if err := db.DB.Model(&db.CapabilityInvocation{}).Where("tool_name = ?", "jira_search_issues").Count(&invocations).Error; err != nil {
		t.Fatal(err)
	}
	if invocations != 1 {
		t.Fatalf("MCP business invocation count = %d, want 1", invocations)
	}
}
