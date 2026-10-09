package server

import (
	"encoding/json"
	"strings"
	"testing"

	"well-ambient/internal/telemetry"
)

func TestOpenJiraStateRetainsCanonicalResponseIdentity(t *testing.T) {
	var issue telemetry.JiraIssue
	if err := json.Unmarshal([]byte(`{"key":"OTHER-1","fields":{"project":{"key":"OTHER"},"assignee":{"name":"Private Person"},"duedate":"2026-10-10","updated":"2026-10-03T12:00:00.000+0000","security":{"id":"10001","name":"Restricted"}}}`), &issue); err != nil {
		t.Fatal(err)
	}
	state, err := jiraStateFromIssue(issue)
	if err != nil || state.IssueKey != "OTHER-1" || state.ProjectRef != "OTHER" || state.SecurityID != "10001" || state.Assignee != "Private Person" || !strings.HasPrefix(state.EvidenceRef, "jira-issue:OTHER-1@") {
		t.Fatalf("response identity was lost: %+v, %v", state, err)
	}
}

func TestOpenJiraStateRejectsMissingOrInconsistentIdentity(t *testing.T) {
	for _, fixture := range []string{
		`{"fields":{"project":{"key":"WA"}}}`,
		`{"key":"WA-1","fields":{}}`,
		`{"key":"OTHER-1","fields":{"project":{"key":"WA"}}}`,
	} {
		var issue telemetry.JiraIssue
		if err := json.Unmarshal([]byte(fixture), &issue); err != nil {
			t.Fatal(err)
		}
		state, err := jiraStateFromIssue(issue)
		if err == nil || state.EvidenceRef != "" || state.IssueKey != "" {
			t.Fatalf("unverified identity produced publication evidence: %+v, %v", state, err)
		}
	}
}
