package telemetry

import (
	"testing"
	"well-ambient/internal/config"
)

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
