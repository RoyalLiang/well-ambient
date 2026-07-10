package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/delivery"
)

func TestCorpusCandidatesRequireReviewBeforeContextPromotion(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "curator@example.com", "Corpus Curator", nil)
	srv := NewServer(&config.Config{}, "")
	spec := db.DemandSpecVersion{
		DemandID: "DEMAND-CORPUS", Version: 1, Status: delivery.SpecFrozen,
		Summary: "Reviewed delivery", UserGoal: "Promote only curated facts", ContextPackID: 11,
		AcceptanceCriteriaJSON: `["candidate is reviewed"]`, TestPlanJSON: `["go test ./..."]`, TasksJSON: `[]`, RisksJSON: `[]`,
	}
	if err := db.DB.Create(&spec).Error; err != nil {
		t.Fatalf("seed spec: %v", err)
	}
	run := db.ExecutionRun{
		RunKey: "corpus-run", DemandID: spec.DemandID, DemandSpecVersionID: spec.ID, Repo: "backend-core",
		Status: delivery.RunDelivered, PipelineStatus: "success", AcceptanceState: "accepted", MRURL: "https://gitlab/mr/3",
	}
	if err := db.DB.Create(&run).Error; err != nil {
		t.Fatalf("seed run: %v", err)
	}
	candidates, err := delivery.EnsureCorpusCandidates(db.DB, run)
	if err != nil {
		t.Fatalf("ensure candidates: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidate count=%d, want 2", len(candidates))
	}
	if _, err := delivery.EnsureCorpusCandidates(db.DB, run); err != nil {
		t.Fatalf("idempotent ensure: %v", err)
	}
	var candidateCount int64
	_ = db.DB.Model(&db.CorpusCandidate{}).Count(&candidateCount).Error
	if candidateCount != 2 {
		t.Fatalf("idempotent candidate count=%d", candidateCount)
	}
	var factCount int64
	_ = db.DB.Model(&db.ContextFact{}).Count(&factCount).Error
	if factCount != 0 {
		t.Fatalf("pending candidates must not create context facts, got %d", factCount)
	}

	listReq := authenticatedJSONRequest(t, srv, token, http.MethodGet, "/api/corpus-candidates?status=pending", nil)
	if listReq.Code != http.StatusOK {
		t.Fatalf("list candidates status=%d body=%s", listReq.Code, listReq.Body.String())
	}
	var listed struct {
		Items []db.CorpusCandidate `json:"items"`
	}
	if err := json.NewDecoder(listReq.Body).Decode(&listed); err != nil || len(listed.Items) != 2 {
		t.Fatalf("listed candidates=%+v err=%v", listed.Items, err)
	}

	accepted := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/corpus-candidates/"+itoa(listed.Items[0].ID)+"/review", reviewCorpusCandidateRequest{Decision: "accepted", Note: "verified"})
	if accepted.Code != http.StatusOK {
		t.Fatalf("accept candidate status=%d body=%s", accepted.Code, accepted.Body.String())
	}
	var promoted db.CorpusCandidate
	if err := db.DB.First(&promoted, listed.Items[0].ID).Error; err != nil {
		t.Fatalf("reload candidate: %v", err)
	}
	if promoted.Status != "accepted" || promoted.AcceptedContextFactID == 0 {
		t.Fatalf("candidate not promoted: %+v", promoted)
	}
	var fact db.ContextFact
	if err := db.DB.First(&fact, promoted.AcceptedContextFactID).Error; err != nil {
		t.Fatalf("load promoted fact: %v", err)
	}
	if fact.Status != "active" || fact.Source != "archive" || fact.ScopeID != "backend-core" {
		t.Fatalf("unexpected promoted fact: %+v", fact)
	}

	reused := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/corpus-candidates/"+itoa(promoted.ID)+"/review", reviewCorpusCandidateRequest{Decision: "rejected"})
	if reused.Code != http.StatusOK || !strings.Contains(reused.Body.String(), `"reused":true`) {
		t.Fatalf("review should be idempotent: %d %s", reused.Code, reused.Body.String())
	}
}
