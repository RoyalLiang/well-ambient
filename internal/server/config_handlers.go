package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	"well-ambient/internal/config"
)

// handleGetConfig returns the current server configuration
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s.config); err != nil {
		log.Printf("Error encoding config: %v", err)
	}
}

// handleSaveConfig updates the server configuration and saves it to disk
func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var newCfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		return
	}

	// Save to disk
	if s.configPath != "" {
		if err := config.SaveConfig(s.configPath, &newCfg); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("Configuration saved to %s", s.configPath)
	} else {
		log.Printf("Warning: configPath is empty, configuration not saved to disk")
	}

	// Hot reload configuration in memory
	*s.config = newCfg

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true,"message":"Configuration saved and applied successfully"}`))
}

type ConnectionTestRequest struct {
	Type   string               `json:"type"`
	GitLab *config.GitLabConfig `json:"gitlab,omitempty"`
	Feishu *config.FeishuConfig `json:"feishu,omitempty"`
	Jira   *config.JiraConfig   `json:"jira,omitempty"`
	AI     *config.AIConfig     `json:"ai,omitempty"`
}

type ConnectionTestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// handleTestConnection dry-runs the connection for a specific integration
func (s *Server) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ConnectionTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		return
	}

	var res ConnectionTestResponse

	switch req.Type {
	case "gitlab":
		if req.GitLab == nil {
			res.Success = false
			res.Message = "GitLab config is missing in test request"
		} else {
			res.Success, res.Message, res.Details = testGitLabConnection(req.GitLab)
		}
	case "feishu":
		if req.Feishu == nil {
			res.Success = false
			res.Message = "Feishu config is missing in test request"
		} else {
			res.Success, res.Message, res.Details = testFeishuConnection(req.Feishu)
		}
	case "jira":
		if req.Jira == nil {
			res.Success = false
			res.Message = "Jira config is missing in test request"
		} else {
			res.Success, res.Message, res.Details = testJiraConnection(req.Jira)
		}
	case "ai":
		if req.AI == nil {
			res.Success = false
			res.Message = "AI config is missing in test request"
		} else {
			res.Success, res.Message, res.Details = testAIConnection(req.AI)
		}
	default:
		res.Success = false
		res.Message = fmt.Sprintf("Unknown integration type: %s", req.Type)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Printf("Error encoding test connection response: %v", err)
	}
}

func testGitLabConnection(cfg *config.GitLabConfig) (bool, string, string) {
	if cfg.BaseURL == "" {
		return false, "GitLab URL is required", ""
	}

	// Normalize URL
	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")

	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL)
	if err != nil {
		return false, "Failed to connect to GitLab URL", err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
		return false, fmt.Sprintf("GitLab host responded with status code %d", resp.StatusCode), ""
	}

	return true, "GitLab URL is reachable.", fmt.Sprintf("HTTP Status: %s", resp.Status)
}

type FeishuTokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
}

func testFeishuConnection(cfg *config.FeishuConfig) (bool, string, string) {
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return false, "Feishu App ID and App Secret are required", ""
	}

	reqBody, err := json.Marshal(map[string]string{
		"app_id":     cfg.AppID,
		"app_secret": cfg.AppSecret,
	})
	if err != nil {
		return false, "Failed to serialize app credentials", err.Error()
	}

	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post("https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return false, "Failed to contact Feishu authentication endpoint", err.Error()
	}
	defer resp.Body.Close()

	var tokenResp FeishuTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return false, "Failed to parse Feishu authentication response", err.Error()
	}

	if tokenResp.Code != 0 {
		return false, fmt.Sprintf("Feishu API Error: %s (code: %d)", tokenResp.Msg, tokenResp.Code), ""
	}

	details := "Feishu App Credentials verified. Tenant access token obtained successfully."

	// Test Bitable connectivity if enabled
	if cfg.Bitable.Enabled {
		if cfg.Bitable.AppToken == "" {
			return false, "Bitable App Token is required when Bitable is enabled", details
		}

		req, err := http.NewRequest("GET", fmt.Sprintf("https://open.feishu.cn/open-apis/bitable/v1/apps/%s", cfg.Bitable.AppToken), nil)
		if err != nil {
			return false, "Failed to create Bitable verification request", details
		}
		req.Header.Set("Authorization", "Bearer "+tokenResp.TenantAccessToken)

		respTable, err := client.Do(req)
		if err != nil {
			return false, "Failed to reach Feishu Bitable API", fmt.Sprintf("%s\nError: %v", details, err)
		}
		defer respTable.Body.Close()

		type BitableResponse struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		var bitableResp BitableResponse
		if err := json.NewDecoder(respTable.Body).Decode(&bitableResp); err != nil {
			return false, "Failed to decode Bitable verification response", details
		}

		if bitableResp.Code != 0 {
			return false, fmt.Sprintf("Bitable access failed: %s (code: %d)", bitableResp.Msg, bitableResp.Code), details
		}

		details += "\nBitable App Token verified successfully."
	}

	return true, "Feishu connection successful.", details
}

func testJiraConnection(cfg *config.JiraConfig) (bool, string, string) {
	if cfg.BaseURL == "" || cfg.APIToken == "" {
		return false, "Jira Base URL and API Token are required", ""
	}

	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
	url := fmt.Sprintf("%s/rest/api/2/myself", baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, "Failed to construct Jira verification request", err.Error()
	}

	if cfg.Username == "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIToken)
	} else {
		req.SetBasicAuth(cfg.Username, cfg.APIToken)
	}

	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, "Failed to contact Jira server", err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, "Jira 返回 404 未找到。如果这是自建 Jira 服务，请检查 URL 是否包含上下文路径（例如 '/jira'），或 REST API 是否受限。", fmt.Sprintf("HTTP Status: %s", resp.Status)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return false, "Jira 凭证校验失败 (401 Unauthorized)。请检查用户名或 API 令牌/PAT 是否正确。", ""
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("Jira returned unexpected response status %s", resp.Status), ""
	}

	type JiraMyself struct {
		DisplayName  string `json:"displayName"`
		EmailAddress string `json:"emailAddress"`
	}
	var user JiraMyself
	if err := json.NewDecoder(resp.Body).Decode(&user); err == nil && user.DisplayName != "" {
		return true, "Jira connection successful.", fmt.Sprintf("Authenticated as: %s (%s)", user.DisplayName, user.EmailAddress)
	}

	return true, "Jira connection successful.", "Authentication checks passed."
}

// handleGetGitLabProjects fetches project list from GitLab API for selection
func (s *Server) handleGetGitLabProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	baseURL := r.URL.Query().Get("base_url")
	token := r.URL.Query().Get("api_token")

	if baseURL == "" || token == "" {
		http.Error(w, "base_url and api_token are required", http.StatusBadRequest)
		return
	}

	baseURL = strings.TrimSuffix(baseURL, "/")
	apiURL := fmt.Sprintf("%s/api/v4/projects?membership=true&simple=true&per_page=100", baseURL)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Create request failed: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("PRIVATE-TOKEN", token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to contact GitLab server: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("GitLab API returned status code %d", resp.StatusCode), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

type GitLabWebhookEnsureRequest struct {
	WebhookURL   string               `json:"webhook_url,omitempty"`
	Repos        []config.RepoMapping `json:"repos,omitempty"`
	Projects     []config.RepoMapping `json:"projects,omitempty"`
	ProjectIDs   []string             `json:"project_ids,omitempty"`
	ProjectPaths []string             `json:"project_paths,omitempty"`
}

type GitLabWebhookProjectResult struct {
	ProjectID string `json:"project_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Path      string `json:"path,omitempty"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	HookID    int    `json:"hook_id,omitempty"`
}

type GitLabWebhookResponse struct {
	WebhookURL string                       `json:"webhook_url"`
	Results    []GitLabWebhookProjectResult `json:"results"`
}

type gitLabProjectHook struct {
	ID                    int    `json:"id"`
	URL                   string `json:"url"`
	PushEvents            bool   `json:"push_events"`
	MergeRequestsEvents   bool   `json:"merge_requests_events"`
	EnableSSLVerification bool   `json:"enable_ssl_verification"`
}

type gitLabWebhookClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func newGitLabWebhookClient(cfg *config.GitLabConfig) (*gitLabWebhookClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("GitLab config is missing")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("gitlab.base_url is required")
	}
	if strings.TrimSpace(cfg.APIToken) == "" {
		return nil, fmt.Errorf("gitlab.api_token is required")
	}

	return &gitLabWebhookClient{
		baseURL: baseURL,
		token:   strings.TrimSpace(cfg.APIToken),
		client:  &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (c *gitLabWebhookClient) newRequest(method, path string, payload interface{}) (*http.Request, error) {
	var body io.Reader
	if payload != nil {
		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewBuffer(bodyBytes)
	}

	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *gitLabWebhookClient) listProjectHooks(projectRef string) ([]gitLabProjectHook, error) {
	path := fmt.Sprintf("/api/v4/projects/%s/hooks?per_page=100", url.PathEscape(projectRef))
	req, err := c.newRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to contact GitLab API")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("GitLab API returned status %d while listing hooks", resp.StatusCode)
	}

	var hooks []gitLabProjectHook
	if err := json.NewDecoder(resp.Body).Decode(&hooks); err != nil {
		return nil, fmt.Errorf("failed to decode GitLab hooks response")
	}
	return hooks, nil
}

func (c *gitLabWebhookClient) createProjectHook(projectRef, webhookURL, secret string) (*gitLabProjectHook, error) {
	payload := gitLabHookPayload(webhookURL, secret)
	path := fmt.Sprintf("/api/v4/projects/%s/hooks", url.PathEscape(projectRef))
	req, err := c.newRequest(http.MethodPost, path, payload)
	if err != nil {
		return nil, err
	}
	return c.doHookMutation(req, http.StatusCreated, "creating hook")
}

func (c *gitLabWebhookClient) updateProjectHook(projectRef string, hookID int, webhookURL, secret string) (*gitLabProjectHook, error) {
	payload := gitLabHookPayload(webhookURL, secret)
	path := fmt.Sprintf("/api/v4/projects/%s/hooks/%d", url.PathEscape(projectRef), hookID)
	req, err := c.newRequest(http.MethodPut, path, payload)
	if err != nil {
		return nil, err
	}
	return c.doHookMutation(req, http.StatusOK, "updating hook")
}

func (c *gitLabWebhookClient) doHookMutation(req *http.Request, expectedStatus int, operation string) (*gitLabProjectHook, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to contact GitLab API")
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatus {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("GitLab API returned status %d while %s", resp.StatusCode, operation)
	}

	var hook gitLabProjectHook
	if err := json.NewDecoder(resp.Body).Decode(&hook); err != nil {
		return nil, fmt.Errorf("failed to decode GitLab hook response")
	}
	return &hook, nil
}

func gitLabHookPayload(webhookURL, secret string) map[string]interface{} {
	return map[string]interface{}{
		"url":                     webhookURL,
		"token":                   secret,
		"push_events":             true,
		"merge_requests_events":   true,
		"enable_ssl_verification": true,
	}
}

// handleEnsureGitLabWebhooks creates or updates project webhooks for selected or configured repositories.
func (s *Server) handleEnsureGitLabWebhooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqBody GitLabWebhookEnsureRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil && err != io.EOF {
			http.Error(w, "Bad Request: invalid JSON body", http.StatusBadRequest)
			return
		}
	}

	webhookURL, err := resolveGitLabWebhookURL(r, reqBody.WebhookURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(s.config.GitLab.Secret) == "" {
		http.Error(w, "gitlab.secret_token is required to install webhooks", http.StatusBadRequest)
		return
	}

	client, err := newGitLabWebhookClient(&s.config.GitLab)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	repos := selectedGitLabWebhookRepos(&s.config.GitLab, reqBody, r.URL.Query())
	if len(repos) == 0 {
		http.Error(w, "No GitLab repositories configured or selected", http.StatusBadRequest)
		return
	}

	results := make([]GitLabWebhookProjectResult, 0, len(repos))
	for _, repo := range repos {
		result := gitLabWebhookResultForRepo(repo)
		projectRef := gitLabProjectRef(repo)
		if projectRef == "" {
			result.Status = "skipped"
			result.Message = "project_id or path is required"
			results = append(results, result)
			continue
		}

		hooks, err := client.listProjectHooks(projectRef)
		if err != nil {
			result.Status = "error"
			result.Message = err.Error()
			results = append(results, result)
			continue
		}

		existing := findGitLabHookByURL(hooks, webhookURL)
		if existing != nil {
			hook, err := client.updateProjectHook(projectRef, existing.ID, webhookURL, s.config.GitLab.Secret)
			if err != nil {
				result.Status = "error"
				result.Message = err.Error()
			} else {
				result.Status = "updated"
				result.Message = "matching webhook updated with required events"
				result.HookID = hook.ID
			}
			results = append(results, result)
			continue
		}

		hook, err := client.createProjectHook(projectRef, webhookURL, s.config.GitLab.Secret)
		if err != nil {
			result.Status = "error"
			result.Message = err.Error()
		} else {
			result.Status = "created"
			result.Message = "webhook created with required events"
			result.HookID = hook.ID
		}
		results = append(results, result)
	}

	writeGitLabWebhookResponse(w, GitLabWebhookResponse{
		WebhookURL: webhookURL,
		Results:    results,
	})
}

// handleGetGitLabWebhookStatus reports whether configured repositories have the expected webhook.
func (s *Server) handleGetGitLabWebhookStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	webhookURL, err := resolveGitLabWebhookURL(r, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	client, err := newGitLabWebhookClient(&s.config.GitLab)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	repos := selectedGitLabWebhookRepos(&s.config.GitLab, GitLabWebhookEnsureRequest{}, nil)
	if len(repos) == 0 {
		http.Error(w, "No GitLab repositories configured", http.StatusBadRequest)
		return
	}

	results := make([]GitLabWebhookProjectResult, 0, len(repos))
	for _, repo := range repos {
		result := gitLabWebhookResultForRepo(repo)
		projectRef := gitLabProjectRef(repo)
		if projectRef == "" {
			result.Status = "skipped"
			result.Message = "project_id or path is required"
			results = append(results, result)
			continue
		}

		hooks, err := client.listProjectHooks(projectRef)
		if err != nil {
			result.Status = "error"
			result.Message = err.Error()
			results = append(results, result)
			continue
		}

		hook := findGitLabHookByURL(hooks, webhookURL)
		result.Status, result.Message = gitLabHookStatus(hook)
		if hook != nil {
			result.HookID = hook.ID
		}
		results = append(results, result)
	}

	writeGitLabWebhookResponse(w, GitLabWebhookResponse{
		WebhookURL: webhookURL,
		Results:    results,
	})
}

func selectedGitLabWebhookRepos(cfg *config.GitLabConfig, req GitLabWebhookEnsureRequest, query url.Values) []config.RepoMapping {
	var repos []config.RepoMapping
	repos = append(repos, req.Repos...)
	repos = append(repos, req.Projects...)

	for _, projectID := range req.ProjectIDs {
		if projectID = strings.TrimSpace(projectID); projectID != "" {
			repos = append(repos, config.RepoMapping{ProjectID: projectID})
		}
	}
	for _, projectPath := range req.ProjectPaths {
		if projectPath = strings.TrimSpace(projectPath); projectPath != "" {
			repos = append(repos, config.RepoMapping{Path: projectPath})
		}
	}

	if query != nil {
		for _, projectID := range splitGitLabQueryValues(query["project_id"]) {
			repos = append(repos, config.RepoMapping{ProjectID: projectID})
		}
		for _, projectPath := range splitGitLabQueryValues(query["project_path"]) {
			repos = append(repos, config.RepoMapping{Path: projectPath})
		}
	}

	if len(repos) == 0 && cfg != nil {
		repos = append(repos, cfg.Repos...)
	}

	return dedupeGitLabRepos(repos)
}

func splitGitLabQueryValues(values []string) []string {
	var out []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	return out
}

func dedupeGitLabRepos(repos []config.RepoMapping) []config.RepoMapping {
	seen := make(map[string]bool)
	deduped := make([]config.RepoMapping, 0, len(repos))
	for _, repo := range repos {
		repo.ProjectID = strings.TrimSpace(repo.ProjectID)
		repo.Path = strings.TrimSpace(repo.Path)
		repo.Name = strings.TrimSpace(repo.Name)
		key := gitLabProjectRef(repo)
		if key == "" {
			key = repo.Name
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, repo)
	}
	return deduped
}

func gitLabWebhookResultForRepo(repo config.RepoMapping) GitLabWebhookProjectResult {
	return GitLabWebhookProjectResult{
		ProjectID: strings.TrimSpace(repo.ProjectID),
		Name:      strings.TrimSpace(repo.Name),
		Path:      strings.TrimSpace(repo.Path),
	}
}

func gitLabProjectRef(repo config.RepoMapping) string {
	if projectID := strings.TrimSpace(repo.ProjectID); projectID != "" {
		return projectID
	}
	return strings.TrimSpace(repo.Path)
}

func findGitLabHookByURL(hooks []gitLabProjectHook, webhookURL string) *gitLabProjectHook {
	target := normalizeGitLabWebhookURL(webhookURL)
	for i := range hooks {
		if normalizeGitLabWebhookURL(hooks[i].URL) == target {
			return &hooks[i]
		}
	}
	return nil
}

func gitLabHookStatus(hook *gitLabProjectHook) (string, string) {
	if hook == nil {
		return "missing", "matching webhook is not installed"
	}

	var missing []string
	if !hook.PushEvents {
		missing = append(missing, "push_events")
	}
	if !hook.MergeRequestsEvents {
		missing = append(missing, "merge_requests_events")
	}
	if len(missing) > 0 {
		return "drift", "matching webhook exists but required events are disabled: " + strings.Join(missing, ", ")
	}
	return "ok", "matching webhook is installed with required events"
}

func normalizeGitLabWebhookURL(webhookURL string) string {
	return strings.TrimRight(strings.TrimSpace(webhookURL), "/")
}

func resolveGitLabWebhookURL(r *http.Request, bodyWebhookURL string) (string, error) {
	webhookURL := strings.TrimSpace(r.URL.Query().Get("webhook_url"))
	if webhookURL == "" {
		webhookURL = strings.TrimSpace(bodyWebhookURL)
	}
	if webhookURL == "" {
		origin := requestOrigin(r)
		if origin == "" {
			return "", fmt.Errorf("webhook_url is required when request origin cannot be inferred")
		}
		webhookURL = strings.TrimRight(origin, "/") + "/api/webhook/gitlab"
	}

	parsed, err := url.Parse(webhookURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("webhook_url must be an absolute http or https URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("webhook_url must use http or https")
	}
	return webhookURL, nil
}

func requestOrigin(r *http.Request) string {
	if origin := normalizedOrigin(r.Header.Get("Origin")); origin != "" {
		return origin
	}

	host := firstForwardedValue(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if host == "" {
		return ""
	}

	proto := firstForwardedValue(r.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		if r.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	if proto != "http" && proto != "https" {
		return ""
	}
	return proto + "://" + host
}

func normalizedOrigin(origin string) string {
	origin = strings.TrimSpace(origin)
	if origin == "" || origin == "null" {
		return ""
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func firstForwardedValue(value string) string {
	if value == "" {
		return ""
	}
	value = strings.Split(value, ",")[0]
	return strings.TrimSpace(value)
}

func writeGitLabWebhookResponse(w http.ResponseWriter, response GitLabWebhookResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode GitLab webhook response: %v", err)
	}
}
