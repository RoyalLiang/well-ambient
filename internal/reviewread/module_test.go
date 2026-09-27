package reviewread

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"well-ambient/internal/codereview"
	"well-ambient/internal/db"
	"well-ambient/internal/openaccess"
)

func testReviewModule(t *testing.T) (*Module, *gorm.DB, openaccess.Policy) {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.AutoMigrate(&db.CodeReviewRun{}); err != nil {
		t.Fatal(err)
	}
	module := New(conn)
	module.now = func() time.Time {
		return time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	}
	policy := openaccess.Policy{
		Version: 3,
		PolicySpec: openaccess.PolicySpec{
			AllowedRepositories: []string{"1"},
			Actions:             []string{"review.read"},
			FieldRules: map[string][]string{
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
			QueryLimits: openaccess.QueryLimits{
				MaxPageSize: 2, MaxScanRows: 100, MaxGroupBy: 2, MaxTimeBuckets: 10,
			},
		},
	}
	return module, conn, policy
}

func seedReviews(t *testing.T, conn *gorm.DB) []db.CodeReviewRun {
	t.Helper()
	now := time.Date(2026, time.September, 27, 10, 0, 0, 0, time.UTC)
	report := codereview.Report{
		EvidenceComplete: false,
		Summary:          "One validated finding; token=top-secret must not be exposed.",
		Scenario:         "general",
		Validation:       "evidence checked",
		Questions:        []string{"missing integration fixture"},
		Findings: []codereview.Finding{
			{
				Dimension: "security", Severity: "high", Title: "Token leak",
				File: "auth.go", Line: 42, Evidence: "Authorization: Bearer abc.def", Impact: "credential exposure",
				Suggestion: "redact token", Verification: "run log redaction test", KnowledgeIDs: []uint{7},
			},
			{
				Dimension: "testing", Severity: "low", Title: "Missing test",
				File: "auth_test.go", Line: 9, Evidence: "TODO", Impact: "regression",
				Suggestion: "add test", Verification: "run test suite",
			},
		},
	}
	reportJSON, _ := json.Marshal(report)
	snapshotJSON, _ := json.Marshal(map[string]any{
		"complete": false,
		"gaps":     []string{"diff pagination budget exceeded"},
	})
	rows := []db.CodeReviewRun{
		{
			Key:       "review-newest",
			ProjectID: "1", Repo: "platform/api", Kind: "mr", Ref: "42",
			HeadSHA: "head-3", BaseSHA: "base-3", Title: "Newest", Status: "partial",
			Error: "sensitive internal stack", ReportJSON: string(reportJSON), SnapshotJSON: string(snapshotJSON),
			PolicyJSON: `{"secret":"must-not-leak"}`, CreatedAt: now.Add(3 * time.Hour), UpdatedAt: now.Add(4 * time.Hour),
		},
		{
			Key:       "review-older",
			ProjectID: "1", Repo: "platform/api", Kind: "commit", Ref: "head-2",
			HeadSHA: "head-2", BaseSHA: "base-2", Title: "Older", Status: "completed",
			ReportJSON: string(reportJSON), CreatedAt: now.Add(2 * time.Hour), UpdatedAt: now.Add(2 * time.Hour),
		},
		{
			Key:       "review-restricted",
			ProjectID: "2", Repo: "platform/api", Kind: "mr", Ref: "7",
			HeadSHA: "secret-head", BaseSHA: "secret-base", Title: "Restricted", Status: "failed",
			Error: "credential=secret", CreatedAt: now.Add(10 * time.Hour), UpdatedAt: now.Add(10 * time.Hour),
		},
	}
	if err := conn.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestSearchEnforcesRepositoryPolicyAndStableCursor(t *testing.T) {
	module, conn, policy := testReviewModule(t)
	seedReviews(t, conn)
	first, err := module.Search(context.Background(), policy, "req-review", SearchRequest{
		Repositories: []string{"1"}, Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 1 || first.Items[0].Repository != "platform/api" ||
		first.Items[0].Title != "Newest" || !first.HasMore || first.NextCursor == "" {
		t.Fatalf("unexpected first page: %+v", first)
	}
	if first.Items[0].RunStatus != "partial" || first.Items[0].CodeFreshness != "not_checked" {
		t.Fatalf("unexpected summary semantics: %+v", first.Items[0])
	}
	if first.Items[0].FindingsCount != 2 || first.Items[0].EvidenceComplete ||
		len(first.Items[0].CoverageGaps) == 0 {
		t.Fatalf("review search omitted report summary: %+v", first.Items[0])
	}
	second, err := module.Search(context.Background(), policy, "req-review-2", SearchRequest{
		Repositories: []string{"1"}, Limit: 1, Cursor: first.NextCursor,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.Items[0].Title != "Older" || second.HasMore {
		t.Fatalf("unexpected second page: %+v", second)
	}
	if _, err := module.Search(context.Background(), policy, "restricted", SearchRequest{
		Repositories: []string{"2"},
	}); openaccess.ErrorCode(err) != "forbidden" {
		t.Fatalf("restricted repository error = %v", err)
	}
}

func TestGetPublishesFilteredEvidenceWithoutInternalMetadata(t *testing.T) {
	module, conn, policy := testReviewModule(t)
	rows := seedReviews(t, conn)
	detail, err := module.Get(context.Background(), policy, "req-detail", GetRequest{
		RunID: rows[0].ID, Severities: []string{"high"}, CurrentHeadSHA: "new-head",
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if detail.Summary.CodeFreshness != "outdated" || detail.Summary.FreshnessChecked == nil {
		t.Fatalf("freshness = %+v", detail.Summary)
	}
	if detail.Summary.RunStatus != "partial" || detail.Summary.EvidenceComplete {
		t.Fatalf("completion semantics = %+v", detail.Summary)
	}
	if len(detail.Summary.CoverageGaps) < 2 || len(detail.Findings) != 1 ||
		detail.Findings[0].Severity != "high" || len(detail.Findings[0].EvidenceRefs) != 2 {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	encoded, _ := json.Marshal(detail)
	text := string(encoded)
	for _, forbidden := range []string{
		"must-not-leak", "sensitive internal stack", "policy_json", "snapshot_json",
		"report_json", "top-secret", "abc.def",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("public detail leaked %q: %s", forbidden, text)
		}
	}
}

func TestFailedReviewDoesNotExposeFailureDetailsOrImplyPass(t *testing.T) {
	module, conn, policy := testReviewModule(t)
	now := time.Date(2026, time.September, 27, 10, 0, 0, 0, time.UTC)
	run := db.CodeReviewRun{
		Key:       "review-failed",
		ProjectID: "1", Repo: "platform/api", Kind: "mr", Ref: "9",
		HeadSHA: "head", Status: "failed", Error: "token=top-secret",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := conn.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	detail, err := module.Get(context.Background(), policy, "req-failed", GetRequest{RunID: run.ID})
	if err != nil {
		t.Fatal(err)
	}
	if detail.Summary.RunStatus != "failed" || detail.Summary.FindingsCount != 0 {
		t.Fatalf("failed review summary = %+v", detail.Summary)
	}
	encoded, _ := json.Marshal(detail)
	if strings.Contains(string(encoded), "top-secret") || len(detail.Meta.MissingReasons) == 0 {
		t.Fatalf("failed review disclosure = %s", encoded)
	}
}
