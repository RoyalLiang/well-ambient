package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/deliveryplanning"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestJiraGetCommentsPaginatesWithoutDroppingEdits(t *testing.T) {
	client := NewJiraClient(&config.JiraConfig{BaseURL: "https://jira.example.com"})
	requests := 0
	client.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		body := `{"startAt":0,"maxResults":1,"total":2,"comments":[{"id":"1","body":"[方案] A","created":"2026-08-12T10:00:00Z","updated":"2026-08-12T10:01:00Z"}]}`
		if requests == 2 {
			if request.URL.Query().Get("startAt") != "1" {
				t.Fatalf("second page startAt = %q", request.URL.Query().Get("startAt"))
			}
			body = `{"startAt":1,"maxResults":1,"total":2,"comments":[{"id":"2","body":"[方案] B","created":"2026-08-12T11:00:00Z","updated":"2026-08-12T11:02:00Z"}]}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	comments, err := client.GetComments("WA-1")
	if err != nil || len(comments) != 2 || comments[1].ID != "2" || comments[1].Updated == "" || requests != 2 {
		t.Fatalf("comments=%+v requests=%d err=%v", comments, requests, err)
	}
}

func TestJiraClientNewRequest(t *testing.T) {
	// 1. Basic Auth test
	basicCfg := &config.JiraConfig{
		BaseURL:  "https://jira.example.com",
		Username: "eddie@example.com",
		APIToken: "token123",
	}
	jcBasic := NewJiraClient(basicCfg)
	req, err := jcBasic.newRequest("GET", "/rest/api/2/issue/PROJ-123", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	if req.URL.String() != "https://jira.example.com/rest/api/2/issue/PROJ-123" {
		t.Errorf("Expected URL 'https://jira.example.com/rest/api/2/issue/PROJ-123', got %q", req.URL.String())
	}

	username, password, ok := req.BasicAuth()
	if !ok {
		t.Errorf("Expected Basic Auth to be set")
	}
	if username != "eddie@example.com" || password != "token123" {
		t.Errorf("Expected basic auth credentials 'eddie@example.com' and 'token123', got '%s:%s'", username, password)
	}

	// 2. Bearer Auth test (Personal Access Token)
	bearerCfg := &config.JiraConfig{
		BaseURL:  "https://jira.example.com/",
		Username: "",
		APIToken: "pat456",
	}
	jcBearer := NewJiraClient(bearerCfg)
	reqBearer, err := jcBearer.newRequest("GET", "/rest/api/2/myself", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	if reqBearer.URL.String() != "https://jira.example.com/rest/api/2/myself" {
		t.Errorf("Expected URL 'https://jira.example.com/rest/api/2/myself', got %q", reqBearer.URL.String())
	}

	authHeader := reqBearer.Header.Get("Authorization")
	if authHeader != "Bearer pat456" {
		t.Errorf("Expected Authorization header 'Bearer pat456', got %q", authHeader)
	}
}

func TestJiraClientValidateJQLUsesLightweightSearchAndReturnsJiraError(t *testing.T) {
	client := NewJiraClient(&config.JiraConfig{BaseURL: "https://jira.example.com", APIToken: "token"})
	client.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/rest/api/2/search" {
			t.Fatalf("path = %q, want Jira search", request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("jql") != `project = "FMS-20660"` {
			t.Fatalf("jql = %q", query.Get("jql"))
		}
		if query.Get("maxResults") != "1" || query.Get("fields") != "key" || query.Has("expand") {
			t.Fatalf("validation search was not lightweight: %s", request.URL.RawQuery)
		}
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader(`{"errorMessages":["project has no value FMS-20660"]}`)),
			Header:     make(http.Header),
		}, nil
	})}

	err := client.ValidateJQL(context.Background(), `project = "FMS-20660"`)
	if err == nil || !strings.Contains(err.Error(), "project has no value FMS-20660") {
		t.Fatalf("ValidateJQL error = %v", err)
	}
}

func TestJiraClientUpdateDueDate(t *testing.T) {
	var received map[string]map[string]string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/rest/api/2/issue/PROJ-123" {
			t.Fatalf("path = %s, want issue update path", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Status:     "204 No Content",
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})

	client := NewJiraClient(&config.JiraConfig{BaseURL: "https://jira.example.com", APIToken: "token"})
	client.client = &http.Client{Transport: transport}
	if err := client.UpdateDueDate("PROJ-123", "2026-08-16"); err != nil {
		t.Fatalf("UpdateDueDate returned error: %v", err)
	}
	if received["fields"]["duedate"] != "2026-08-16" {
		t.Fatalf("duedate payload = %#v, want 2026-08-16", received)
	}
}

func TestJiraClientAddComment(t *testing.T) {
	var received map[string]string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/rest/api/2/issue/PROJ-123/comment" {
			t.Fatalf("path = %s, want comment path", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusCreated,
			Status:     "201 Created",
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})

	client := NewJiraClient(&config.JiraConfig{BaseURL: "https://jira.example.com", APIToken: "token"})
	client.client = &http.Client{Transport: transport}
	if err := client.AddComment("PROJ-123", "会议确认本周五交付"); err != nil {
		t.Fatalf("AddComment returned error: %v", err)
	}
	if received["body"] != "会议确认本周五交付" {
		t.Fatalf("comment payload = %#v, want decision description", received)
	}
}

func TestJiraClientListProjectReleases(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/rest/api/2/project/PRJ25024/versions" {
			t.Fatalf("path = %s, want release catalog path", r.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`[
				{"id":"13622","name":"ReeWell-1.1","description":"scope","released":false,"archived":false,"startDate":"2026-07-01","releaseDate":"2026-08-15","self":"https://jira.example.com/rest/api/2/version/13622"},
				{"id":"13623","name":"ReeWell-1.0","released":true,"archived":false}
			]`)),
			Header: make(http.Header),
		}, nil
	})
	client := NewJiraClient(&config.JiraConfig{BaseURL: "https://jira.example.com", APIToken: "token"})
	client.client = &http.Client{Transport: transport}
	releases, err := client.ListProjectReleases(context.Background(), " prj25024 ")
	if err != nil {
		t.Fatalf("list releases: %v", err)
	}
	if len(releases) != 2 {
		t.Fatalf("release count = %d, want 2", len(releases))
	}
	if releases[0].ProjectKey != "PRJ25024" || releases[0].ExternalID != "13622" ||
		releases[0].Status != deliveryplanning.ReleasePlanned || releases[0].ReleaseDate == nil {
		t.Fatalf("unexpected planned release: %+v", releases[0])
	}
	if releases[1].Status != deliveryplanning.ReleaseReleased {
		t.Fatalf("released status = %q, want released", releases[1].Status)
	}
}

func TestJiraClientListProjectReleasesEmptyInvalidAndTimeout(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		client := NewJiraClient(&config.JiraConfig{BaseURL: "https://jira.example.com"})
		client.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`[]`)),
				Header:     make(http.Header),
			}, nil
		})}
		releases, err := client.ListProjectReleases(context.Background(), "HIT")
		if err != nil || len(releases) != 0 {
			t.Fatalf("empty releases = %#v, err=%v", releases, err)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		client := NewJiraClient(&config.JiraConfig{BaseURL: "https://jira.example.com"})
		client.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`not-json`)),
				Header:     make(http.Header),
			}, nil
		})}
		if _, err := client.ListProjectReleases(context.Background(), "HIT"); err == nil {
			t.Fatal("expected invalid JSON error")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		client := NewJiraClient(&config.JiraConfig{BaseURL: "https://jira.example.com"})
		client.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			<-r.Context().Done()
			return nil, r.Context().Err()
		})}
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()
		if _, err := client.ListProjectReleases(ctx, "HIT"); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline exceeded, got %v", err)
		}
	})
}

func TestJiraClientIssueVersionStateAndWritePayload(t *testing.T) {
	var updatePayload map[string]map[string][]map[string]string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.Method {
		case http.MethodGet:
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"key":"HIT-101",
					"fields":{
						"project":{"key":"HIT","name":"HIT"},
						"issuetype":{"name":"Bug"},
						"fixVersions":[{"id":"12","name":"1.2"}],
						"versions":[{"id":"10","name":"1.0","released":true}],
						"updated":"2026-07-30T10:15:20.000+0800"
					}
				}`)),
				Header: make(http.Header),
			}, nil
		case http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
				t.Fatalf("decode update payload: %v", err)
			}
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		default:
			t.Fatalf("unexpected method %s", r.Method)
			return nil, nil
		}
	})
	client := NewJiraClient(&config.JiraConfig{BaseURL: "https://jira.example.com"})
	client.client = &http.Client{Transport: transport}
	state, err := client.LoadIssueVersionState(context.Background(), "HIT-101")
	if err != nil {
		t.Fatalf("load issue versions: %v", err)
	}
	if state.Kind != deliveryplanning.WorkItemBug || len(state.TargetReleases) != 1 || len(state.AffectedReleases) != 1 {
		t.Fatalf("unexpected issue version state: %+v", state)
	}
	if err := client.UpdateIssueVersionState(context.Background(), deliveryplanning.IssueVersionUpdate{
		IssueKey:            "HIT-101",
		TargetExternalIDs:   []string{"12"},
		AffectedExternalIDs: []string{"10"},
	}); err != nil {
		t.Fatalf("update issue versions: %v", err)
	}
	if updatePayload["fields"]["fixVersions"][0]["id"] != "12" ||
		updatePayload["fields"]["versions"][0]["id"] != "10" {
		t.Fatalf("unexpected update payload: %#v", updatePayload)
	}
}
