package telemetry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Keep the authoritative custom-field values until search's names expansion
// identifies the configured Jira field. Titles and assignees are not classifiers.
func (issue *JiraIssue) UnmarshalJSON(data []byte) error {
	type plain JiraIssue
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw struct {
		Fields    map[string]json.RawMessage `json:"fields"`
		Changelog json.RawMessage            `json:"changelog"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*issue = JiraIssue(decoded)
	issue.rawFields = raw.Fields
	var metadata struct {
		Total *int `json:"total"`
	}
	_ = json.Unmarshal(raw.Changelog, &metadata)
	issue.HistoryAvailable = metadata.Total != nil
	return nil
}

func (jc *JiraClient) completeReportHistory(issue *JiraIssue) {
	if !issue.HistoryAvailable || (issue.Changelog.StartAt == 0 && len(issue.Changelog.Histories) >= issue.Changelog.Total) {
		return
	}
	// Data Center normally provides its full history on the individual issue.
	var expanded JiraIssue
	if jc.readReportJSON("/rest/api/2/issue/"+url.PathEscape(issue.Key)+"?fields=key&expand=changelog", &expanded) == nil && expanded.HistoryAvailable && expanded.Changelog.StartAt == 0 && len(expanded.Changelog.Histories) >= expanded.Changelog.Total {
		issue.Changelog = expanded.Changelog
		return
	}
	// Servers supporting the paginated changelog API may cap search expansions.
	// Replace the original only after every page has been verified.
	histories := []JiraHistory{}
	seen := map[string]bool{}
	for pageCount := 0; pageCount < 500; pageCount++ {
		var page struct {
			StartAt int           `json:"startAt"`
			Total   int           `json:"total"`
			Values  []JiraHistory `json:"values"`
		}
		path := fmt.Sprintf("/rest/api/2/issue/%s/changelog?startAt=%d&maxResults=100", url.PathEscape(issue.Key), len(histories))
		if jc.readReportJSON(path, &page) != nil || page.StartAt != len(histories) || len(page.Values) == 0 || page.Total < issue.Changelog.Total {
			return
		}
		for _, history := range page.Values {
			if history.ID == "" || seen[history.ID] {
				return
			}
			seen[history.ID] = true
			histories = append(histories, history)
		}
		if len(histories) == page.Total {
			issue.Changelog.Histories = histories
			issue.Changelog.StartAt = 0
			issue.Changelog.Total = page.Total
			return
		}
		if len(histories) > page.Total {
			return
		}
	}
}

func (jc *JiraClient) readReportJSON(path string, target any) error {
	req, err := jc.newRequest(http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	resp, err := jc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Jira report facts returned HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (issue *JiraIssue) applyReportFieldNames(names map[string]string) {
	issue.BugCategory, issue.BugCategoryFieldID, issue.BugCategoryAvailable = "", "", false
	var ids []string
	for id, name := range names {
		if strings.TrimSpace(name) == "bug归类" {
			ids = append(ids, id)
		}
	}
	if len(ids) != 1 {
		return
	}
	issue.BugCategoryFieldID = ids[0]
	raw, ok := issue.rawFields[ids[0]]
	issue.BugCategoryAvailable = ok
	if !ok {
		return
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		issue.BugCategoryAvailable = false
		return
	}
	categories := map[string]bool{}
	var visit func(any)
	visit = func(value any) {
		switch v := value.(type) {
		case string:
			normalized := strings.ToUpper(strings.TrimSpace(v))
			if normalized == "FMS" || normalized == "GPP" {
				categories[normalized] = true
			}
		case []any:
			for _, child := range v {
				visit(child)
			}
		case map[string]any:
			visit(v["value"])
			visit(v["child"])
		}
	}
	visit(value)
	// Conflicting selections do not silently take precedence over one another.
	if len(categories) == 1 {
		for category := range categories {
			issue.BugCategory = category
		}
	}
}
