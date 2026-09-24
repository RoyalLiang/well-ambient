package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/codereview"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestStrongestBrainReviewIntelligenceAggregatesSkillQualityWithoutEvidencePayloads(t *testing.T) {
	setupServerTestDB(t)
	cfg := &config.Config{}
	cfg.GitLab.Repos = []config.RepoMapping{{ProjectID: "10", Name: "dispatch", Path: "fms/dispatch"}}
	srv := NewServer(cfg, "")
	var skill db.SolutionPromptTemplate
	if err := db.DB.Where("purpose = ? AND status = ?", "code_review", "active").First(&skill).Error; err != nil {
		t.Fatal(err)
	}
	report := codereview.Report{
		EvidenceComplete: false,
		Summary:          "fixture summary",
		Findings: []codereview.Finding{{
			Dimension: "robustness", Severity: "high", Title: "fixture finding",
		}},
		Questions: []string{"missing contract"},
	}
	reportJSON, _ := json.Marshal(report)
	now := time.Now()
	runs := []db.CodeReviewRun{
		{
			Key: "brain-review-1", ProjectID: "10", Repo: "dispatch", Kind: "mr", Ref: "7",
			Status: "completed", ReportJSON: string(reportJSON), PromptVersion: "review-skill:1/v1",
			SkillVersionID: skill.ID, SkillVersion: skill.Version, SkillName: skill.Name,
			CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
		},
		{
			Key: "brain-review-2", ProjectID: "10", Repo: "dispatch", Kind: "commit", Ref: strings.Repeat("a", 40),
			Status: "partial", PromptVersion: "review-skill:1/v1", RetryOfID: 1,
			SkillVersionID: skill.ID, SkillVersion: skill.Version, SkillName: skill.Name,
			CreatedAt: now, UpdatedAt: now,
		},
	}
	if err := db.DB.Create(&runs).Error; err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/api/strongest-brain/review-intelligence", nil)
	recorder := httptest.NewRecorder()
	srv.handleGetStrongestBrainReviewIntelligence(recorder, request)
	if recorder.Code != 200 {
		t.Fatalf("status/body=%d/%s", recorder.Code, recorder.Body.String())
	}
	var response StrongestBrainReviewIntelligenceResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Summary.Total != 2 || response.Summary.Partial != 1 ||
		response.Summary.HighRiskFindings != 1 || response.Summary.Questions != 1 ||
		response.Summary.Retried != 1 || len(response.Versions) != 1 ||
		len(response.Recommendations) == 0 || len(response.ActiveSkills) == 0 {
		t.Fatalf("unexpected review intelligence: %+v", response)
	}
	for _, forbidden := range []string{"report_json", "snapshot_json", "fixture summary", "missing contract"} {
		if strings.Contains(recorder.Body.String(), forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, recorder.Body.String())
		}
	}
}

func TestStrongestBrainReviewIntelligenceRejectsUnknownProject(t *testing.T) {
	setupServerTestDB(t)
	cfg := &config.Config{}
	cfg.GitLab.Repos = []config.RepoMapping{{ProjectID: "10", Name: "dispatch"}}
	srv := NewServer(cfg, "")
	request := httptest.NewRequest("GET", "/api/strongest-brain/review-intelligence?project_id=999", nil)
	recorder := httptest.NewRecorder()
	srv.handleGetStrongestBrainReviewIntelligence(recorder, request)
	if recorder.Code != 400 {
		t.Fatalf("status/body=%d/%s", recorder.Code, recorder.Body.String())
	}
}
