package telemetry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"well-ambient/internal/config"
)

func TestJiraReportCategoryUsesOnlyNamedCustomField(t *testing.T) {
	for _, tc := range []struct {
		name, raw, want string
		names           map[string]string
		available       bool
	}{
		{"select", `{"value":"FMS"}`, "FMS", map[string]string{"customfield_42": "bug归类"}, true},
		{"cascade", `{"value":"软件","child":{"value":"GPP"}}`, "GPP", map[string]string{"customfield_42": "bug归类"}, true},
		{"other", `{"value":"硬件"}`, "", map[string]string{"customfield_42": "bug归类"}, true},
		{"empty", `null`, "", map[string]string{"customfield_42": "bug归类"}, true},
		{"conflicting", `[{"value":"FMS"},{"value":"GPP"}]`, "", map[string]string{"customfield_42": "bug归类"}, true},
		{"wrong-field", `"FMS"`, "", map[string]string{"customfield_42": "其他字段"}, false},
		{"ambiguous-name", `"FMS"`, "", map[string]string{"customfield_42": "bug归类", "customfield_43": "bug归类"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var issue JiraIssue
			raw := fmt.Sprintf(`{"key":"FMS-1","fields":{"summary":"GPP 路径死锁 FMS","customfield_42":%s}}`, tc.raw)
			if err := json.Unmarshal([]byte(raw), &issue); err != nil {
				t.Fatal(err)
			}
			issue.applyReportFieldNames(tc.names)
			if issue.BugCategory != tc.want || issue.BugCategoryAvailable != tc.available {
				t.Fatalf("category=%q available=%v", issue.BugCategory, issue.BugCategoryAvailable)
			}
		})
	}
}

func TestJiraSearchCollectsCategoryAndCompleteHistoricalAssignees(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/api/2/search":
			if !strings.Contains(r.URL.Query().Get("expand"), "names") || !strings.Contains(r.URL.Query().Get("fields"), "*all") {
				t.Error("custom field names/values were not requested")
			}
			fmt.Fprint(w, `{"total":1,"names":{"customfield_42":"bug归类"},"issues":[{"key":"PROJ-1","fields":{"customfield_42":{"value":"GPP"}},"changelog":{"startAt":1,"total":2,"histories":[{"id":"2"}]}}]}`)
		case "/rest/api/2/issue/PROJ-1":
			fmt.Fprint(w, `{"key":"PROJ-1","changelog":{"startAt":1,"total":2,"histories":[{"id":"2"}]}}`)
		case "/rest/api/2/issue/PROJ-1/changelog":
			if r.URL.Query().Get("startAt") == "0" {
				fmt.Fprint(w, `{"startAt":0,"total":2,"values":[{"id":"1","created":"2026-09-20T10:00:00.000+0000","items":[{"field":"assignee","from":"alice","fromString":"Alice","to":"outsider","toString":"Outsider"}]}]}`)
			} else {
				fmt.Fprint(w, `{"startAt":1,"total":2,"values":[{"id":"2"}]}`)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()
	issues, err := NewJiraClient(&config.JiraConfig{BaseURL: api.URL}).SearchIssues("assignee WAS IN (alice)")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].BugCategory != "GPP" || issues[0].Changelog.StartAt != 0 || len(issues[0].Changelog.Histories) != 2 || issues[0].Changelog.Histories[0].Items[0].From != "alice" {
		t.Fatalf("issues: %+v", issues)
	}
}

func TestJiraTruncatedHistoryNeverClaimsCompletenessWhenAPIUnavailable(t *testing.T) {
	api := httptest.NewServer(http.NotFoundHandler())
	defer api.Close()
	var issue JiraIssue
	_ = json.Unmarshal([]byte(`{"key":"P-1","changelog":{"startAt":1,"total":2,"histories":[{"id":"2"}]}}`), &issue)
	NewJiraClient(&config.JiraConfig{BaseURL: api.URL}).completeReportHistory(&issue)
	if issue.Changelog.StartAt == 0 || len(issue.Changelog.Histories) >= issue.Changelog.Total {
		t.Fatal("partial history was marked complete")
	}
}
