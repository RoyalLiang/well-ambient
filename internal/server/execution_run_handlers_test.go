package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/delivery"
)

func TestExecutionRunCreatesGitLabBranchCommitAndDraftMRIdempotently(t *testing.T) {
	setupServerTestDB(t)
	var calls atomic.Int32
	gitlab := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("PRIVATE-TOKEN") != "delivery-token" {
			t.Fatalf("missing GitLab private token")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/42":
			_, _ = io.WriteString(w, `{"id":42,"name":"Backend","path_with_namespace":"group/backend","default_branch":"main"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/users":
			if r.URL.Query().Get("search") != "Reviewer" {
				t.Fatalf("unexpected reviewer search: %s", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `[{"id":7,"username":"reviewer","name":"Reviewer","state":"active"}]`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/42/repository/branches":
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["ref"] != "main" || !strings.HasPrefix(body["branch"].(string), "ai/demand-run/v1-") {
				t.Fatalf("unexpected branch request: %+v", body)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/42/repository/commits":
			var body struct {
				Branch  string                   `json:"branch"`
				Actions []map[string]interface{} `json:"actions"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if len(body.Actions) != 1 || body.Actions[0]["file_path"] != "internal/new.go" || body.Actions[0]["action"] != "create" {
				t.Fatalf("unexpected commit actions: %+v", body)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"id":"abc123","short_id":"abc123","web_url":"https://gitlab.example/commit/abc123"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/42/merge_requests":
			var body struct {
				Title       string `json:"title"`
				ReviewerIDs []int  `json:"reviewer_ids"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if !strings.HasPrefix(body.Title, "Draft:") || len(body.ReviewerIDs) != 1 || body.ReviewerIDs[0] != 7 {
				t.Fatalf("unexpected MR request: %+v", body)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"iid":15,"web_url":"https://gitlab.example/mr/15","state":"opened"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/42/pipelines":
			if r.URL.Query().Get("ref") == "" {
				t.Fatal("pipeline ref is required")
			}
			_, _ = io.WriteString(w, `[{"id":21,"status":"success","sha":"abc123","web_url":"https://gitlab.example/pipelines/21"}]`)
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer gitlab.Close()

	token := superAdminToken(t, "executor@example.com", "Executor", nil)
	srv := NewServer(&config.Config{GitLab: config.GitLabConfig{
		Enabled: true, BaseURL: gitlab.URL, APIToken: "delivery-token",
		Repos: []config.RepoMapping{{Name: "backend-core", Path: "group/backend", ProjectID: "42"}},
	}}, "")

	demand := db.TaskTelemetry{TaskID: "DEMAND-RUN", Title: "Run delivery", Description: "Create a Draft MR", Assignee: "Owner", IssueType: "demand", Status: "backlog", LastUpdate: time.Now()}
	if err := db.DB.Create(&demand).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}
	now := time.Now()
	spec := db.DemandSpecVersion{
		DemandID: demand.TaskID, Version: 1, Status: delivery.SpecFrozen, Summary: "Create reviewed change",
		MappedReposJSON: `["backend-core"]`, AcceptanceCriteriaJSON: `["MR is Draft"]`, TestPlanJSON: `["go test ./..."]`,
		ReadinessScore: 95, FrozenAt: &now, AuthoredBy: "Author",
	}
	if err := db.DB.Create(&spec).Error; err != nil {
		t.Fatalf("seed spec: %v", err)
	}
	contract := db.ReviewContract{
		DemandSpecVersionID: spec.ID, DemandID: demand.TaskID, Status: delivery.ReviewApproved,
		RequiredRolesJSON: `["code_owner"]`, ReviewerCandidatesJSON: `["Reviewer"]`,
		AcceptanceOwner: "Owner", MinimumApprovals: 1, ProtectedPathRulesJSON: `[]`,
	}
	if err := db.DB.Create(&contract).Error; err != nil {
		t.Fatalf("seed contract: %v", err)
	}

	request := createExecutionRunRequest{ExecutionPreflight: executionPreflightRequest{
		DemandSpecVersionID: spec.ID, Repo: "backend-core", Author: "Author",
		ChangeSet:    []delivery.FileAction{{Action: "create", Path: "internal/new.go", Content: "package internal"}},
		TestCommands: []string{"go test ./..."},
	}}
	createRR := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/execution/runs", request)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("create run status = %d, body = %s", createRR.Code, createRR.Body.String())
	}
	var created struct {
		Run executionRunDTO `json:"run"`
	}
	if err := json.NewDecoder(createRR.Body).Decode(&created); err != nil {
		t.Fatalf("decode run: %v", err)
	}

	startRR := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/execution/runs/"+itoa(created.Run.ID)+"/start", map[string]interface{}{})
	if startRR.Code != http.StatusOK {
		t.Fatalf("start run status = %d, body = %s", startRR.Code, startRR.Body.String())
	}
	var run db.ExecutionRun
	if err := db.DB.First(&run, created.Run.ID).Error; err != nil {
		t.Fatalf("load run: %v", err)
	}
	if run.Status != delivery.RunReviewPending || run.CommitSHA != "abc123" || run.MRIID != 15 || run.MRURL == "" {
		t.Fatalf("unexpected completed run: %+v", run)
	}
	if got := calls.Load(); got != 5 {
		t.Fatalf("GitLab calls = %d, want 5", got)
	}
	refreshRR := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/execution/runs/"+itoa(created.Run.ID)+"/refresh", map[string]interface{}{})
	if refreshRR.Code != http.StatusOK || !strings.Contains(refreshRR.Body.String(), `"pipeline_status":"success"`) {
		t.Fatalf("refresh pipeline status = %d, body = %s", refreshRR.Code, refreshRR.Body.String())
	}
	if got := calls.Load(); got != 6 {
		t.Fatalf("GitLab calls after refresh = %d, want 6", got)
	}

	reusedCreate := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/execution/runs", request)
	if reusedCreate.Code != http.StatusOK || !strings.Contains(reusedCreate.Body.String(), `"reused":true`) {
		t.Fatalf("expected reused create, got %d %s", reusedCreate.Code, reusedCreate.Body.String())
	}
	reusedStart := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/execution/runs/"+itoa(created.Run.ID)+"/start", map[string]interface{}{})
	if reusedStart.Code != http.StatusOK || calls.Load() != 6 {
		t.Fatalf("reused start should not call GitLab again: status=%d calls=%d body=%s", reusedStart.Code, calls.Load(), reusedStart.Body.String())
	}

	var evidence int64
	if err := db.DB.Model(&db.GitCommitLog{}).Where("task_id = ?", demand.TaskID).Count(&evidence).Error; err != nil || evidence != 3 {
		t.Fatalf("execution evidence count=%d err=%v", evidence, err)
	}
	var updatedDemand db.TaskTelemetry
	_ = db.DB.First(&updatedDemand, "task_id = ?", demand.TaskID).Error
	if updatedDemand.Status != "review" || updatedDemand.MrIID != 15 {
		t.Fatalf("demand evidence projection not updated: %+v", updatedDemand)
	}
}

func TestExecutionRunFailureIsAuditedAndCannotRepeatSideEffects(t *testing.T) {
	setupServerTestDB(t)
	var calls atomic.Int32
	gitlab := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
	}))
	defer gitlab.Close()

	client, err := newGitLabDeliveryClient(&config.GitLabConfig{BaseURL: gitlab.URL, APIToken: "token"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	if _, err := client.GetProject(t.Context(), "42"); err == nil {
		t.Fatal("expected GitLab failure")
	}
	if calls.Load() != 1 {
		t.Fatalf("calls=%d, want 1", calls.Load())
	}
}

func TestExecutionRunHumanAcceptanceRequiresNamedOwnerAndSuccessfulPipeline(t *testing.T) {
	setupServerTestDB(t)
	ownerToken := superAdminToken(t, "owner@example.com", "Product Owner", nil)
	srv := NewServer(&config.Config{}, "")
	demand := db.TaskTelemetry{TaskID: "DEMAND-ACCEPT", Title: "Acceptance", IssueType: "demand", Status: "review", LastUpdate: time.Now()}
	if err := db.DB.Create(&demand).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}
	spec := db.DemandSpecVersion{DemandID: demand.TaskID, Version: 1, Status: delivery.SpecFrozen}
	if err := db.DB.Create(&spec).Error; err != nil {
		t.Fatalf("seed spec: %v", err)
	}
	contract := db.ReviewContract{DemandSpecVersionID: spec.ID, DemandID: demand.TaskID, Status: delivery.ReviewApproved, AcceptanceOwner: "Product Owner"}
	if err := db.DB.Create(&contract).Error; err != nil {
		t.Fatalf("seed contract: %v", err)
	}
	run := db.ExecutionRun{
		RunKey: "accept-run", DemandID: demand.TaskID, DemandSpecVersionID: spec.ID, ReviewContractID: contract.ID,
		Status: delivery.RunReviewPending, PipelineStatus: "failed", MRState: "opened",
	}
	if err := db.DB.Create(&run).Error; err != nil {
		t.Fatalf("seed run: %v", err)
	}

	failedPipeline := authenticatedJSONRequest(t, srv, ownerToken, http.MethodPost, "/api/execution/runs/"+itoa(run.ID)+"/verify", verifyExecutionRunRequest{Decision: "accepted"})
	if failedPipeline.Code != http.StatusConflict {
		t.Fatalf("accept failed pipeline status=%d body=%s", failedPipeline.Code, failedPipeline.Body.String())
	}
	if err := db.DB.Model(&run).Update("pipeline_status", "success").Error; err != nil {
		t.Fatalf("set pipeline: %v", err)
	}
	accepted := authenticatedJSONRequest(t, srv, ownerToken, http.MethodPost, "/api/execution/runs/"+itoa(run.ID)+"/verify", verifyExecutionRunRequest{Decision: "accepted", Note: "verified"})
	if accepted.Code != http.StatusOK {
		t.Fatalf("accept run status=%d body=%s", accepted.Code, accepted.Body.String())
	}
	if err := db.DB.First(&run, run.ID).Error; err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if run.Status != delivery.RunAcceptancePending || run.AcceptanceState != "accepted" {
		t.Fatalf("unexpected accepted run: %+v", run)
	}

	run.MRState = "merged"
	if err := db.DB.Save(&run).Error; err != nil {
		t.Fatalf("mark merged: %v", err)
	}
	delivered := authenticatedJSONRequest(t, srv, ownerToken, http.MethodPost, "/api/execution/runs/"+itoa(run.ID)+"/verify", verifyExecutionRunRequest{Decision: "accepted"})
	if delivered.Code != http.StatusOK {
		t.Fatalf("deliver accepted merged run status=%d body=%s", delivered.Code, delivered.Body.String())
	}
	if err := db.DB.First(&run, run.ID).Error; err != nil || run.Status != delivery.RunDelivered {
		t.Fatalf("run should be delivered: %+v err=%v", run, err)
	}
}
