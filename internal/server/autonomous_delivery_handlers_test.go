package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/delivery"
)

func TestDemandSpecReviewContractAndFreezeLifecycle(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "author@example.com", "Author", nil)
	srv := NewServer(&config.Config{
		Jira: config.JiraConfig{SyncUsers: []string{"Reviewer", "Author"}},
	}, "")

	demand := db.TaskTelemetry{
		TaskID:        "DEMAND-AUTO-1",
		Title:         "Controlled delivery",
		Description:   "Create a reviewed autonomous delivery flow",
		Assignee:      "Product Owner",
		Creator:       "Author",
		IssueType:     "demand",
		Status:        "backlog",
		TaskCreatedAt: time.Now(),
		LastUpdate:    time.Now(),
	}
	if err := db.DB.Create(&demand).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}

	createBody := demandSpecPayload{
		DemandID:           demand.TaskID,
		OriginalText:       demand.Description,
		Summary:            "Deliver only after human review",
		UserGoal:           "Create auditable Draft MRs",
		AcceptanceCriteria: []string{"A Draft MR is created from a topic branch"},
		TestPlan:           []string{"Run focused backend tests"},
		MappedRepos:        []string{"backend-core"},
		ReadinessScore:     88,
		Tasks: []TaskDetail{{
			ID: "task-900", Repo: "backend-core", Title: "Implement controlled run", Assignee: "Reviewer",
		}},
	}
	createRR := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/demand-specs", createBody)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("create spec status = %d, body = %s", createRR.Code, createRR.Body.String())
	}
	var created struct {
		Spec           demandSpecDTO     `json:"spec"`
		ReviewContract reviewContractDTO `json:"review_contract"`
	}
	if err := json.NewDecoder(createRR.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Spec.Version != 1 || created.Spec.Status != delivery.SpecDraft {
		t.Fatalf("unexpected spec: %+v", created.Spec)
	}
	if created.ReviewContract.Status != delivery.ReviewDraft || created.ReviewContract.AcceptanceOwner != "Product Owner" {
		t.Fatalf("unexpected default review contract: %+v", created.ReviewContract)
	}

	contractBody := reviewContractPayload{
		ID:                 created.ReviewContract.ID,
		ReviewerCandidates: []string{"Reviewer"},
		RequiredRoles:      []string{"code_owner", "qa"},
		AcceptanceOwner:    "Product Owner",
		MinimumApprovals:   1,
		ReviewSLAHours:     12,
		EscalationOwner:    "Engineering Lead",
		SegregationRules:   []string{"author_cannot_self_approve"},
		ProtectedPathRules: []protectedPathRule{{Pattern: "internal/db/**", RequiredRole: "dba", ReviewerCandidates: []string{"Reviewer"}}},
	}
	contractRR := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/review-contracts", contractBody)
	if contractRR.Code != http.StatusOK {
		t.Fatalf("update review contract status = %d, body = %s", contractRR.Code, contractRR.Body.String())
	}

	approveRR := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/review-contracts/"+itoa(created.ReviewContract.ID)+"/approve", map[string]interface{}{})
	if approveRR.Code != http.StatusOK {
		t.Fatalf("approve review contract status = %d, body = %s", approveRR.Code, approveRR.Body.String())
	}

	freezeRR := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/demand-specs/"+itoa(created.Spec.ID)+"/freeze", map[string]interface{}{})
	if freezeRR.Code != http.StatusOK {
		t.Fatalf("freeze spec status = %d, body = %s", freezeRR.Code, freezeRR.Body.String())
	}
	var frozen struct {
		Spec demandSpecDTO `json:"spec"`
	}
	if err := json.NewDecoder(freezeRR.Body).Decode(&frozen); err != nil {
		t.Fatalf("decode frozen spec: %v", err)
	}
	if frozen.Spec.Status != delivery.SpecFrozen || frozen.Spec.FrozenBy != "Author" {
		t.Fatalf("unexpected frozen spec: %+v", frozen.Spec)
	}

	createBody.ID = created.Spec.ID
	createBody.Summary = "Attempt to mutate frozen spec"
	mutateRR := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/demand-specs", createBody)
	if mutateRR.Code != http.StatusConflict {
		t.Fatalf("mutating frozen spec status = %d, want %d; body = %s", mutateRR.Code, http.StatusConflict, mutateRR.Body.String())
	}

	createBody.ID = 0
	createBody.Summary = "Version two"
	versionTwoRR := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/demand-specs", createBody)
	if versionTwoRR.Code != http.StatusCreated {
		t.Fatalf("create v2 status = %d, body = %s", versionTwoRR.Code, versionTwoRR.Body.String())
	}
	var versionTwo struct {
		Spec demandSpecDTO `json:"spec"`
	}
	if err := json.NewDecoder(versionTwoRR.Body).Decode(&versionTwo); err != nil {
		t.Fatalf("decode v2: %v", err)
	}
	if versionTwo.Spec.Version != 2 || versionTwo.Spec.Status != delivery.SpecDraft {
		t.Fatalf("unexpected v2: %+v", versionTwo.Spec)
	}
}

func TestFreezeDemandSpecReturnsReadinessBlockers(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "gate@example.com", "Gate", nil)
	srv := NewServer(&config.Config{Jira: config.JiraConfig{SyncUsers: []string{"Reviewer"}}}, "")
	demand := db.TaskTelemetry{TaskID: "DEMAND-BLOCKED", Title: "Blocked", Assignee: "Owner", IssueType: "demand", Status: "backlog", LastUpdate: time.Now()}
	if err := db.DB.Create(&demand).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}

	createRR := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/demand-specs", demandSpecPayload{
		DemandID: demand.TaskID, Summary: "Incomplete", ReadinessScore: 20,
	})
	if createRR.Code != http.StatusCreated {
		t.Fatalf("create incomplete spec: %d %s", createRR.Code, createRR.Body.String())
	}
	var created struct {
		Spec           demandSpecDTO     `json:"spec"`
		ReviewContract reviewContractDTO `json:"review_contract"`
	}
	_ = json.NewDecoder(createRR.Body).Decode(&created)

	approveRR := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/review-contracts/"+itoa(created.ReviewContract.ID)+"/approve", map[string]interface{}{})
	if approveRR.Code != http.StatusOK {
		t.Fatalf("approve default contract: %d %s", approveRR.Code, approveRR.Body.String())
	}
	freezeRR := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/demand-specs/"+itoa(created.Spec.ID)+"/freeze", map[string]interface{}{})
	if freezeRR.Code != http.StatusUnprocessableEntity {
		t.Fatalf("freeze incomplete spec status = %d, body = %s", freezeRR.Code, freezeRR.Body.String())
	}
}

func TestExecutionPreflightResolvesReviewersAndRejectsUnsafeChanges(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "executor@example.com", "Executor", nil)
	srv := NewServer(&config.Config{
		GitLab: config.GitLabConfig{Repos: []config.RepoMapping{{Name: "backend-core", Path: "group/backend-core", ProjectID: "42"}}},
	}, "")

	now := time.Now()
	spec := db.DemandSpecVersion{
		DemandID: "DEMAND-PREFLIGHT", Version: 1, Status: delivery.SpecFrozen,
		Summary: "Safe delivery", ReadinessScore: 90, MappedReposJSON: `["backend-core"]`,
		AcceptanceCriteriaJSON: `["Draft MR exists"]`, TestPlanJSON: `["go test ./..."]`,
		AuthoredBy: "Author", FrozenBy: "Owner", FrozenAt: &now,
	}
	if err := db.DB.Create(&spec).Error; err != nil {
		t.Fatalf("seed spec: %v", err)
	}
	contract := db.ReviewContract{
		DemandSpecVersionID: spec.ID, DemandID: spec.DemandID, Status: delivery.ReviewApproved,
		RequiredRolesJSON: `["code_owner"]`, ReviewerCandidatesJSON: `["Author","Reviewer"]`,
		AcceptanceOwner: "Product Owner", MinimumApprovals: 1,
		ProtectedPathRulesJSON: `[]`, SegregationRulesJSON: `["author_cannot_self_approve"]`,
	}
	if err := db.DB.Create(&contract).Error; err != nil {
		t.Fatalf("seed contract: %v", err)
	}

	good := executionPreflightRequest{
		DemandSpecVersionID: spec.ID, Repo: "backend-core", Author: "Author",
		ChangeSet:    []delivery.FileAction{{Action: "update", Path: "internal/server/handler.go", Content: "package server"}},
		TestCommands: []string{"go test ./internal/server"},
	}
	rr := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/execution/preflight", good)
	if rr.Code != http.StatusOK {
		t.Fatalf("preflight status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var result executionPreflightResponse
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("decode preflight: %v", err)
	}
	if !result.Ready || len(result.ReviewerResolution.Reviewers) != 1 || result.ReviewerResolution.Reviewers[0] != "Reviewer" {
		t.Fatalf("unexpected preflight result: %+v", result)
	}

	bad := good
	bad.ChangeSet = []delivery.FileAction{{Action: "update", Path: "../outside", Content: "bad"}}
	badRR := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/execution/preflight", bad)
	if badRR.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unsafe preflight status = %d, body = %s", badRR.Code, badRR.Body.String())
	}
}

func authenticatedJSONRequest(t *testing.T, srv *Server, token, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	return rr
}

func itoa(value uint) string {
	return fmt.Sprintf("%d", value)
}
