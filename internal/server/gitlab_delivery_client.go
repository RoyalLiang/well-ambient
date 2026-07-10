package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/delivery"
)

type gitLabProject struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	PathWithNS    string `json:"path_with_namespace"`
	DefaultBranch string `json:"default_branch"`
}

type gitLabCommit struct {
	ID      string `json:"id"`
	ShortID string `json:"short_id"`
	WebURL  string `json:"web_url"`
}

type gitLabMergeRequest struct {
	IID    int    `json:"iid"`
	WebURL string `json:"web_url"`
	State  string `json:"state"`
}

type gitLabPipeline struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
	SHA    string `json:"sha"`
	WebURL string `json:"web_url"`
}

type gitLabDelivery interface {
	GetProject(context.Context, string) (gitLabProject, error)
	GetFile(context.Context, string, string, string) (string, error)
	GetLatestPipeline(context.Context, string, string) (gitLabPipeline, error)
	ResolveReviewerIDs(context.Context, []string) ([]int, error)
	CreateBranch(context.Context, string, string, string) error
	Commit(context.Context, string, string, string, []delivery.FileAction) (gitLabCommit, error)
	CreateDraftMR(context.Context, string, string, string, string, string, []int) (gitLabMergeRequest, error)
}

func (c *gitLabDeliveryClient) GetFile(ctx context.Context, projectRef, filePath, ref string) (string, error) {
	var response struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	path := "/api/v4/projects/" + url.PathEscape(projectRef) + "/repository/files/" + url.PathEscape(strings.TrimSpace(filePath)) + "?ref=" + url.QueryEscape(ref)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, http.StatusOK, &response); err != nil {
		return "", err
	}
	if response.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(response.Content, "\n", ""))
		if err != nil {
			return "", fmt.Errorf("failed to decode repository file %s", filePath)
		}
		return string(decoded), nil
	}
	return response.Content, nil
}

func (c *gitLabDeliveryClient) GetLatestPipeline(ctx context.Context, projectRef, ref string) (gitLabPipeline, error) {
	var pipelines []gitLabPipeline
	path := "/api/v4/projects/" + url.PathEscape(projectRef) + "/pipelines?per_page=1&ref=" + url.QueryEscape(ref)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, http.StatusOK, &pipelines); err != nil {
		return gitLabPipeline{}, err
	}
	if len(pipelines) == 0 {
		return gitLabPipeline{Status: "not_found"}, nil
	}
	return pipelines[0], nil
}

type gitLabDeliveryClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func newGitLabDeliveryClient(cfg *config.GitLabConfig) (*gitLabDeliveryClient, error) {
	if cfg == nil || strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, fmt.Errorf("gitlab.base_url is required")
	}
	if strings.TrimSpace(cfg.APIToken) == "" {
		return nil, fmt.Errorf("gitlab.api_token is required")
	}
	return &gitLabDeliveryClient{
		baseURL: strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		token:   strings.TrimSpace(cfg.APIToken),
		client:  &http.Client{Timeout: 20 * time.Second},
	}, nil
}

func (c *gitLabDeliveryClient) GetProject(ctx context.Context, projectRef string) (gitLabProject, error) {
	var project gitLabProject
	err := c.doJSON(ctx, http.MethodGet, "/api/v4/projects/"+url.PathEscape(projectRef), nil, http.StatusOK, &project)
	if err != nil {
		return gitLabProject{}, err
	}
	if strings.TrimSpace(project.DefaultBranch) == "" {
		return gitLabProject{}, fmt.Errorf("GitLab project has no default branch")
	}
	return project, nil
}

func (c *gitLabDeliveryClient) ResolveReviewerIDs(ctx context.Context, names []string) ([]int, error) {
	type gitLabUser struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
		State    string `json:"state"`
	}
	ids := make([]int, 0, len(names))
	for _, name := range normalizeDeliveryStrings(names) {
		var users []gitLabUser
		path := "/api/v4/users?active=true&search=" + url.QueryEscape(name)
		if err := c.doJSON(ctx, http.MethodGet, path, nil, http.StatusOK, &users); err != nil {
			return nil, err
		}
		matchedID := 0
		for _, user := range users {
			if strings.EqualFold(user.Username, name) || strings.EqualFold(user.Name, name) {
				matchedID = user.ID
				break
			}
		}
		if matchedID == 0 {
			return nil, fmt.Errorf("GitLab reviewer %q could not be resolved", name)
		}
		ids = append(ids, matchedID)
	}
	return ids, nil
}

func (c *gitLabDeliveryClient) CreateBranch(ctx context.Context, projectRef, branch, ref string) error {
	payload := map[string]string{"branch": branch, "ref": ref}
	return c.doJSON(ctx, http.MethodPost, "/api/v4/projects/"+url.PathEscape(projectRef)+"/repository/branches", payload, http.StatusCreated, nil)
}

func (c *gitLabDeliveryClient) Commit(ctx context.Context, projectRef, branch, message string, changes []delivery.FileAction) (gitLabCommit, error) {
	actions := make([]map[string]interface{}, 0, len(changes))
	for _, change := range changes {
		action := map[string]interface{}{
			"action":    delivery.NormalizeState(change.Action),
			"file_path": strings.TrimSpace(strings.ReplaceAll(change.Path, "\\", "/")),
		}
		if delivery.NormalizeState(change.Action) != "delete" {
			action["content"] = change.Content
			if strings.TrimSpace(change.Encoding) == "base64" {
				action["encoding"] = "base64"
			}
		}
		actions = append(actions, action)
	}
	payload := map[string]interface{}{
		"branch":         branch,
		"commit_message": message,
		"actions":        actions,
	}
	var commit gitLabCommit
	err := c.doJSON(ctx, http.MethodPost, "/api/v4/projects/"+url.PathEscape(projectRef)+"/repository/commits", payload, http.StatusCreated, &commit)
	return commit, err
}

func (c *gitLabDeliveryClient) CreateDraftMR(ctx context.Context, projectRef, sourceBranch, targetBranch, title, description string, reviewerIDs []int) (gitLabMergeRequest, error) {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(title)), "draft:") {
		title = "Draft: " + strings.TrimSpace(title)
	}
	payload := map[string]interface{}{
		"source_branch":        sourceBranch,
		"target_branch":        targetBranch,
		"title":                title,
		"description":          description,
		"reviewer_ids":         reviewerIDs,
		"remove_source_branch": false,
		"labels":               "ai-generated,needs-human-review",
	}
	var mr gitLabMergeRequest
	err := c.doJSON(ctx, http.MethodPost, "/api/v4/projects/"+url.PathEscape(projectRef)+"/merge_requests", payload, http.StatusCreated, &mr)
	return mr, err
}

func (c *gitLabDeliveryClient) doJSON(ctx context.Context, method, path string, payload interface{}, expected int, target interface{}) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to contact GitLab API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != expected {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(message)))
	}
	if target == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("failed to decode GitLab response: %w", err)
	}
	return nil
}
