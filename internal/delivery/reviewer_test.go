package delivery

import "testing"

func TestResolveReviewersExcludesAuthorAndHonorsProtectedPaths(t *testing.T) {
	result := ResolveReviewers(ReviewerResolutionInput{
		RequiredRoles:      []string{"code_owner"},
		ReviewerCandidates: []string{"Author", "General Reviewer"},
		MinimumApprovals:   2,
		AcceptanceOwner:    "Product Owner",
		Author:             "Author",
		ChangedPaths:       []string{"internal/db/schema.go", "web/src/App.svelte"},
		ProtectedPathRules: []ReviewPathRule{{
			Pattern: "internal/db/**", RequiredRole: "dba", ReviewerCandidates: []string{"DB Reviewer"},
		}},
	})
	if !result.Resolved {
		t.Fatalf("expected resolution, got %+v", result)
	}
	if len(result.Reviewers) != 2 || result.Reviewers[0] != "DB Reviewer" || result.Reviewers[1] != "General Reviewer" {
		t.Fatalf("unexpected reviewers: %+v", result.Reviewers)
	}
	for _, reviewer := range result.Reviewers {
		if reviewer == "Author" {
			t.Fatal("author must never resolve as reviewer")
		}
	}
}

func TestResolveReviewersBlocksUnavailableProtectedOwner(t *testing.T) {
	result := ResolveReviewers(ReviewerResolutionInput{
		ReviewerCandidates: []string{"General Reviewer"}, MinimumApprovals: 1, AcceptanceOwner: "Owner",
		Unavailable: []string{"DB Reviewer"}, ChangedPaths: []string{"internal/db/schema.go"},
		ProtectedPathRules: []ReviewPathRule{{Pattern: "internal/db/**", RequiredRole: "dba", ReviewerCandidates: []string{"DB Reviewer"}}},
	})
	if result.Resolved || len(result.Blockers) == 0 {
		t.Fatalf("expected protected path blocker, got %+v", result)
	}
}

func TestValidateFileActionsRejectsUnsafeAndDuplicatePaths(t *testing.T) {
	paths, blockers := ValidateFileActions([]FileAction{
		{Action: "update", Path: "internal/server/a.go", Content: "ok"},
		{Action: "update", Path: "internal/server/a.go", Content: "duplicate"},
		{Action: "create", Path: "../secret", Content: "bad"},
		{Action: "update", Path: ".git/config", Content: "bad"},
	})
	if len(paths) != 1 || len(blockers) != 3 {
		t.Fatalf("paths=%v blockers=%v", paths, blockers)
	}
}
