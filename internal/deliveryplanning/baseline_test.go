package deliveryplanning

import "testing"

func TestAnalyzeBaselineUsesOnlyDeterministicProjectMatches(t *testing.T) {
	projects := []BaselineProject{
		{ProjectKey: "HIT", Repositories: []string{"hit-repo", "shared-repo"}},
		{ProjectKey: "NS2", Repositories: []string{"ns2-repo", "shared-repo"}},
	}
	tasks := []BaselineTask{
		{TaskID: "LOCAL-1", IssueType: "demand", ProjectKey: "HIT", TaskGroupID: "group-1"},
		{TaskID: "NS2-2", IssueType: "bug"},
		{TaskID: "LOCAL-3", IssueType: "task", Repo: "hit-repo", TaskGroupID: "group-1"},
		{TaskID: "LOCAL-4", IssueType: "task", Repo: "shared-repo"},
		{TaskID: "LOCAL-5", IssueType: "task", Repo: "missing-repo"},
	}

	report := AnalyzeBaseline(tasks, projects, true)

	if report.TotalTasks != 5 || report.ConfiguredProjects != 2 || !report.SchemaHasProjectKey {
		t.Fatalf("unexpected baseline header: %+v", report)
	}
	if report.ProjectResolution.Explicit != 1 ||
		report.ProjectResolution.IDPrefix != 1 ||
		report.ProjectResolution.UniqueRepo != 1 ||
		report.ProjectResolution.AmbiguousRepo != 1 ||
		report.ProjectResolution.Unresolved != 1 {
		t.Fatalf("unexpected project resolution: %+v", report.ProjectResolution)
	}
	if len(report.UnresolvedTasks) != 2 {
		t.Fatalf("unresolved tasks = %d, want ambiguous plus unresolved", len(report.UnresolvedTasks))
	}
	if report.WithTaskGroup != 2 || report.WithoutTaskGroup != 3 {
		t.Fatalf("unexpected task group counts: with=%d without=%d", report.WithTaskGroup, report.WithoutTaskGroup)
	}
}

func TestAnalyzeBaselineRejectsUnknownExplicitProject(t *testing.T) {
	report := AnalyzeBaseline([]BaselineTask{{
		TaskID: "HIT-1", IssueType: "demand", ProjectKey: "UNKNOWN",
	}}, []BaselineProject{{ProjectKey: "HIT"}}, true)

	if report.ProjectResolution.Unresolved != 1 || report.ProjectResolution.IDPrefix != 0 {
		t.Fatalf("unknown explicit project must not silently fall back: %+v", report.ProjectResolution)
	}
	if len(report.UnresolvedTasks) != 1 {
		t.Fatalf("unresolved tasks = %d, want 1", len(report.UnresolvedTasks))
	}
}
