package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/deliveryplanning"
)

type JiraVersion struct {
	Self        string `json:"self"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ProjectID   int64  `json:"projectId"`
	Archived    bool   `json:"archived"`
	Released    bool   `json:"released"`
	StartDate   string `json:"startDate"`
	ReleaseDate string `json:"releaseDate"`
}

type JiraIssue struct {
	Key       string `json:"key"`
	Changelog struct {
		Histories []JiraHistory `json:"histories"`
	} `json:"changelog"`
	Fields struct {
		Summary              string `json:"summary"`
		Description          string `json:"description"`
		Created              string `json:"created"`
		ResolutionDate       string `json:"resolutiondate"`
		DueDate              string `json:"duedate"`
		TimeOriginalEstimate int64  `json:"timeoriginalestimate"`
		TimeTracking         struct {
			OriginalEstimateSeconds int64 `json:"originalEstimateSeconds"`
		} `json:"timetracking"`
		IssueType struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Parent     *JiraIssueReference `json:"parent"`
		IssueLinks []JiraIssueLink     `json:"issuelinks"`
		Priority   struct {
			Name string `json:"name"`
		} `json:"priority"`
		Assignee *struct {
			Name         string `json:"name"`
			EmailAddress string `json:"emailAddress"`
			DisplayName  string `json:"displayName"`
		} `json:"assignee"`
		Reporter *struct {
			Name         string `json:"name"`
			EmailAddress string `json:"emailAddress"`
			DisplayName  string `json:"displayName"`
		} `json:"reporter"`
		Status struct {
			Name string `json:"name"`
		} `json:"status"`
		Project struct {
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"project"`
		FixVersions []JiraVersion `json:"fixVersions"`
		Versions    []JiraVersion `json:"versions"`
		Updated     string        `json:"updated"`
	} `json:"fields"`
}

type JiraIssueReference struct {
	Key    string `json:"key"`
	Fields struct {
		IssueType struct {
			Name string `json:"name"`
		} `json:"issuetype"`
	} `json:"fields"`
}

type JiraIssueLink struct {
	Type struct {
		Name    string `json:"name"`
		Inward  string `json:"inward"`
		Outward string `json:"outward"`
	} `json:"type"`
	InwardIssue  *JiraIssueReference `json:"inwardIssue"`
	OutwardIssue *JiraIssueReference `json:"outwardIssue"`
}

type JiraHistory struct {
	ID      string `json:"id"`
	Created string `json:"created"`
	Author  struct {
		Name         string `json:"name"`
		EmailAddress string `json:"emailAddress"`
		DisplayName  string `json:"displayName"`
	} `json:"author"`
	Items []struct {
		Field      string `json:"field"`
		FieldID    string `json:"fieldId"`
		FromString string `json:"fromString"`
		ToString   string `json:"toString"`
	} `json:"items"`
}

type JiraSearchResponse struct {
	Total  int         `json:"total"`
	Issues []JiraIssue `json:"issues"`
}

type JiraTransition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   struct {
		Name string `json:"name"`
	} `json:"to"`
}

type JiraTransitionsResponse struct {
	Transitions []JiraTransition `json:"transitions"`
}

type JiraClient struct {
	Config *config.JiraConfig
	client *http.Client
}

func NewJiraClient(cfg *config.JiraConfig) *JiraClient {
	return &JiraClient{
		Config: cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (jc *JiraClient) newRequest(method, path string, body io.Reader) (*http.Request, error) {
	baseURL := strings.TrimSuffix(jc.Config.BaseURL, "/")
	fullURL := fmt.Sprintf("%s%s", baseURL, path)

	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if jc.Config.Username == "" {
		req.Header.Set("Authorization", "Bearer "+jc.Config.APIToken)
	} else {
		req.SetBasicAuth(jc.Config.Username, jc.Config.APIToken)
	}

	return req, nil
}

func (jc *JiraClient) SearchIssues(jql string) ([]JiraIssue, error) {
	var allIssues []JiraIssue
	startAt := 0
	maxResults := 50
	fields := strings.Join([]string{
		"summary", "description", "created", "issuetype", "assignee", "reporter", "status", "project",
		"fixVersions", "versions", "updated", "resolutiondate", "duedate",
		"timeoriginalestimate", "timetracking", "priority",
		"parent", "issuelinks",
	}, ",")

	for {
		path := fmt.Sprintf("/rest/api/2/search?jql=%s&startAt=%d&maxResults=%d&fields=%s&expand=changelog", url.QueryEscape(jql), startAt, maxResults, url.QueryEscape(fields))
		req, err := jc.newRequest("GET", path, nil)
		if err != nil {
			return nil, err
		}

		resp, err := jc.client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("Jira search failed with status %d: %s", resp.StatusCode, string(bodyBytes))
		}

		var res JiraSearchResponse
		err = json.NewDecoder(resp.Body).Decode(&res)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		allIssues = append(allIssues, res.Issues...)
		if len(allIssues) >= res.Total || len(res.Issues) == 0 {
			break
		}
		startAt += len(res.Issues)
	}

	return allIssues, nil
}

func (jc *JiraClient) ListProjectReleases(ctx context.Context, projectKey string) ([]deliveryplanning.ExternalRelease, error) {
	projectKey = deliveryplanning.NormalizeProjectKey(projectKey)
	if projectKey == "" {
		return nil, fmt.Errorf("project key is required")
	}
	req, err := jc.newRequest(
		http.MethodGet,
		fmt.Sprintf("/rest/api/2/project/%s/versions", url.PathEscape(projectKey)),
		nil,
	)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := jc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Jira release list failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}
	var versions []JiraVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, fmt.Errorf("decode Jira release list: %w", err)
	}
	releases := make([]deliveryplanning.ExternalRelease, 0, len(versions))
	for _, version := range versions {
		startDate, err := parseJiraDate(version.StartDate)
		if err != nil {
			return nil, fmt.Errorf("parse start date for Jira release %s: %w", version.ID, err)
		}
		releaseDate, err := parseJiraDate(version.ReleaseDate)
		if err != nil {
			return nil, fmt.Errorf("parse release date for Jira release %s: %w", version.ID, err)
		}
		status := deliveryplanning.ReleasePlanned
		if version.Archived {
			status = deliveryplanning.ReleaseArchived
		} else if version.Released {
			status = deliveryplanning.ReleaseReleased
		}
		releases = append(releases, deliveryplanning.ExternalRelease{
			ProjectKey:  projectKey,
			ExternalID:  strings.TrimSpace(version.ID),
			Name:        strings.TrimSpace(version.Name),
			Description: version.Description,
			Status:      status,
			StartDate:   startDate,
			ReleaseDate: releaseDate,
			SourceURL:   version.Self,
		})
	}
	return releases, nil
}

func (jc *JiraClient) LoadIssueVersionState(ctx context.Context, issueKey string) (deliveryplanning.ExternalIssueVersionState, error) {
	issueKey = strings.TrimSpace(issueKey)
	if issueKey == "" {
		return deliveryplanning.ExternalIssueVersionState{}, fmt.Errorf("issue key is required")
	}
	path := fmt.Sprintf(
		"/rest/api/2/issue/%s?fields=project,issuetype,fixVersions,versions,updated",
		url.PathEscape(issueKey),
	)
	req, err := jc.newRequest(http.MethodGet, path, nil)
	if err != nil {
		return deliveryplanning.ExternalIssueVersionState{}, err
	}
	req = req.WithContext(ctx)
	resp, err := jc.client.Do(req)
	if err != nil {
		return deliveryplanning.ExternalIssueVersionState{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return deliveryplanning.ExternalIssueVersionState{}, fmt.Errorf("Jira issue version load failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}
	var issue JiraIssue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return deliveryplanning.ExternalIssueVersionState{}, fmt.Errorf("decode Jira issue version state: %w", err)
	}
	return JiraIssueVersionState(issue)
}

func JiraIssueVersionState(issue JiraIssue) (deliveryplanning.ExternalIssueVersionState, error) {
	kind, err := deliveryplanning.NormalizeIssueType(issue.Fields.IssueType.Name)
	if err != nil {
		return deliveryplanning.ExternalIssueVersionState{}, err
	}
	updatedAt, err := parseJiraTimestamp(issue.Fields.Updated)
	if err != nil {
		return deliveryplanning.ExternalIssueVersionState{}, err
	}
	projectKey := deliveryplanning.NormalizeProjectKey(issue.Fields.Project.Key)
	return deliveryplanning.ExternalIssueVersionState{
		IssueKey:         issue.Key,
		ProjectKey:       projectKey,
		Kind:             kind,
		UpdatedAt:        updatedAt,
		TargetReleases:   jiraVersionsToExternal(projectKey, issue.Fields.FixVersions),
		AffectedReleases: jiraVersionsToExternal(projectKey, issue.Fields.Versions),
	}, nil
}

func (jc *JiraClient) UpdateIssueVersionState(ctx context.Context, command deliveryplanning.IssueVersionUpdate) error {
	issueKey := strings.TrimSpace(command.IssueKey)
	if issueKey == "" {
		return fmt.Errorf("issue key is required")
	}
	versionRefs := func(ids []string) []map[string]string {
		refs := make([]map[string]string, 0, len(ids))
		for _, id := range ids {
			if id = strings.TrimSpace(id); id != "" {
				refs = append(refs, map[string]string{"id": id})
			}
		}
		return refs
	}
	payload := map[string]any{"fields": map[string]any{
		"fixVersions": versionRefs(command.TargetExternalIDs),
		"versions":    versionRefs(command.AffectedExternalIDs),
	}}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := jc.newRequest(http.MethodPut, "/rest/api/2/issue/"+url.PathEscape(issueKey), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	resp, err := jc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Jira issue version update failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

func parseJiraDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseJiraTimestamp(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.000-0700"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("parse Jira timestamp %q", value)
}

func jiraVersionsToExternal(projectKey string, versions []JiraVersion) []deliveryplanning.ExternalRelease {
	releases := make([]deliveryplanning.ExternalRelease, 0, len(versions))
	for _, version := range versions {
		status := deliveryplanning.ReleasePlanned
		if version.Archived {
			status = deliveryplanning.ReleaseArchived
		} else if version.Released {
			status = deliveryplanning.ReleaseReleased
		}
		startDate, _ := parseJiraDate(version.StartDate)
		releaseDate, _ := parseJiraDate(version.ReleaseDate)
		releases = append(releases, deliveryplanning.ExternalRelease{
			ProjectKey:  projectKey,
			ExternalID:  version.ID,
			Name:        version.Name,
			Description: version.Description,
			Status:      status,
			StartDate:   startDate,
			ReleaseDate: releaseDate,
			SourceURL:   version.Self,
		})
	}
	return releases
}

func (jc *JiraClient) GetTransitions(issueKey string) ([]JiraTransition, error) {
	path := fmt.Sprintf("/rest/api/2/issue/%s/transitions", issueKey)
	req, err := jc.newRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := jc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Jira get transitions failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var res JiraTransitionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return res.Transitions, nil
}

func (jc *JiraClient) TransitionIssue(issueKey, transitionID string) error {
	path := fmt.Sprintf("/rest/api/2/issue/%s/transitions", issueKey)
	payload := map[string]interface{}{
		"transition": map[string]string{
			"id": transitionID,
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := jc.newRequest("POST", path, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}

	resp, err := jc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Jira transition failed with status %d: %s", resp.StatusCode, string(respBytes))
	}

	return nil
}

func (jc *JiraClient) UpdateAssignee(issueKey string, assigneeName string) error {
	var payload map[string]interface{}
	if assigneeName == "" {
		payload = map[string]interface{}{
			"name": nil,
		}
	} else {
		payload = map[string]interface{}{
			"name": assigneeName,
		}
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := jc.newRequest("PUT", fmt.Sprintf("/rest/api/2/issue/%s/assignee", issueKey), bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := jc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Jira API returned status %s: %s", resp.Status, string(respBytes))
	}
	return nil
}

// UpdateIssueWithComment applies a Daily Jira decision in one Jira issue update.
// A nil assignee keeps the current owner; a non-nil value updates or clears it.
func (jc *JiraClient) UpdateIssueWithComment(issueKey string, assigneeName *string, comment string) error {
	issueKey = strings.TrimSpace(issueKey)
	comment = strings.TrimSpace(comment)
	if issueKey == "" {
		return fmt.Errorf("issue key is required")
	}
	if comment == "" {
		return fmt.Errorf("decision comment is required")
	}

	payload := map[string]interface{}{
		"update": map[string]interface{}{
			"comment": []map[string]interface{}{{"add": map[string]string{"body": comment}}},
		},
	}
	if assigneeName != nil {
		var name interface{} = strings.TrimSpace(*assigneeName)
		if name == "" {
			name = nil
		}
		payload["fields"] = map[string]interface{}{
			"assignee": map[string]interface{}{"name": name},
		}
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := jc.newRequest(http.MethodPut, fmt.Sprintf("/rest/api/2/issue/%s", url.PathEscape(issueKey)), bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	resp, err := jc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Jira decision update failed with status %s: %s", resp.Status, string(respBytes))
	}
	return nil
}

func (jc *JiraClient) UpdateDueDate(issueKey, dueDate string) error {
	payload := map[string]interface{}{
		"fields": map[string]string{
			"duedate": dueDate,
		},
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := jc.newRequest(http.MethodPut, fmt.Sprintf("/rest/api/2/issue/%s", issueKey), bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	resp, err := jc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Jira due date update failed with status %s: %s", resp.Status, string(respBytes))
	}
	return nil
}

func (jc *JiraClient) AddComment(issueKey, body string) error {
	bodyBytes, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return err
	}
	req, err := jc.newRequest(http.MethodPost, fmt.Sprintf("/rest/api/2/issue/%s/comment", issueKey), bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	resp, err := jc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Jira comment creation failed with status %s: %s", resp.Status, string(respBytes))
	}
	return nil
}

type JiraComment struct {
	ID     string `json:"id"`
	Author struct {
		DisplayName string `json:"displayName"`
	} `json:"author"`
	Body    string `json:"body"`
	Created string `json:"created"`
	Updated string `json:"updated"`
}

type JiraCommentsResponse struct {
	StartAt    int           `json:"startAt"`
	MaxResults int           `json:"maxResults"`
	Total      int           `json:"total"`
	Comments   []JiraComment `json:"comments"`
}

func (jc *JiraClient) GetComments(issueKey string) ([]JiraComment, error) {
	basePath := fmt.Sprintf("/rest/api/2/issue/%s/comment", issueKey)
	comments := make([]JiraComment, 0)
	startAt := 0
	for {
		path := basePath
		if startAt > 0 {
			path = fmt.Sprintf("%s?startAt=%d&maxResults=100", basePath, startAt)
		}
		req, err := jc.newRequest("GET", path, nil)
		if err != nil {
			return nil, err
		}
		resp, err := jc.client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("Jira get comments failed with status %d: %s", resp.StatusCode, string(bodyBytes))
		}
		var page JiraCommentsResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&page)
		resp.Body.Close()
		if decodeErr != nil {
			return nil, decodeErr
		}
		comments = append(comments, page.Comments...)
		if page.Total <= len(comments) || len(page.Comments) == 0 || page.Total == 0 {
			return comments, nil
		}
		startAt += len(page.Comments)
	}
}
