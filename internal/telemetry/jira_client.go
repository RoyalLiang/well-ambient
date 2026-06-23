package telemetry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"well-ambient/internal/config"
)

type JiraIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string `json:"summary"`
		Description string `json:"description"`
		Created     string `json:"created"`
		IssueType   struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Assignee *struct {
			Name         string `json:"name"`
			EmailAddress string `json:"emailAddress"`
			DisplayName  string `json:"displayName"`
		} `json:"assignee"`
		Status struct {
			Name string `json:"name"`
		} `json:"status"`
		Project struct {
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"project"`
	} `json:"fields"`
}

type JiraSearchResponse struct {
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
	path := fmt.Sprintf("/rest/api/2/search?jql=%s", url.QueryEscape(jql))
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
		return nil, fmt.Errorf("Jira search failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var res JiraSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return res.Issues, nil
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
