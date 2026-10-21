package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/telemetry"
)

func mergeGitLabSecrets(next *config.GitLabConfig, current config.GitLabConfig) {
	// The release anchor is immutable through browser saves.
	next.LegacyDedupeOrigin = current.LegacyDedupeOrigin
	same := strings.TrimRight(next.BaseURL, "/") == strings.TrimRight(current.BaseURL, "/")
	if !same {
		if next.APIToken == configuredSecretPlaceholder {
			next.APIToken = ""
		}
		if next.Secret == configuredSecretPlaceholder {
			next.Secret = ""
		}
	} else {
		next.APIToken = resolveConfiguredSecret(next.APIToken, current.APIToken)
		next.Secret = resolveConfiguredSecret(next.Secret, current.Secret)
	}
	if next.Connections == nil {
		next.Connections = append([]config.GitLabConnection(nil), current.Connections...)
	}
	for i := range next.Connections {
		connection := &next.Connections[i]
		previous := config.GitLabConnection{}
		for _, old := range current.Connections {
			if old.ID == connection.ID && strings.TrimRight(old.BaseURL, "/") == strings.TrimRight(connection.BaseURL, "/") {
				previous = old
				break
			}
		}
		connection.APIToken = resolveConfiguredSecret(connection.APIToken, previous.APIToken)
		connection.Secret = resolveConfiguredSecret(connection.Secret, previous.Secret)
	}
}

func (s *Server) gitLabForRequest(instance string) (config.GitLabConfig, error) {
	selected, err := s.config.GitLab.Connection(instance)
	if err != nil {
		return selected, err
	}
	if !selected.Enabled && instance != "" && instance != "default" {
		return selected, fmt.Errorf("GitLab instance is disabled")
	}
	return selected, nil
}

func (s *Server) handleGitLabInstanceWebhook(w http.ResponseWriter, r *http.Request) {
	instance := r.PathValue("instance")
	if instance == "" || instance == "default" {
		http.Error(w, "Unknown GitLab instance", http.StatusNotFound)
		return
	}
	selected, err := s.gitLabForRequest(instance)
	if err != nil {
		http.Error(w, "Unknown or disabled GitLab instance", http.StatusNotFound)
		return
	}
	cfg := *s.config
	cfg.GitLab = selected
	telemetry.HandleWebhook(&cfg, w, r)
}

func gitLabInstanceWebhookURL(r *http.Request, raw string, instance string) (string, error) {
	if raw == "" && r.URL.Query().Get("webhook_url") == "" {
		raw = strings.TrimRight(requestOrigin(r), "/") + "/api/webhook/gitlab"
		if instance != "" {
			raw += "/" + instance
		}
	}
	value, err := resolveGitLabWebhookURL(r, raw)
	if err != nil {
		return "", err
	}
	if instance != "" && !strings.HasSuffix(value, "/api/webhook/gitlab/"+instance) {
		return "", fmt.Errorf("callback URL must target the selected GitLab instance")
	}
	return value, nil
}

// Configured repositories are the allowlist for non-default connections.
func scopedWebhookRepos(cfg config.GitLabConfig, selected []config.RepoMapping) ([]config.RepoMapping, error) {
	if cfg.ActiveInstanceID == "" {
		for _, repo := range selected {
			if repo.InstanceID != "" || strings.Contains(repo.ProjectID, ":") {
				return nil, fmt.Errorf("repository belongs to another GitLab instance; select that instance first")
			}
		}
		return selected, nil
	}
	result := make([]config.RepoMapping, 0, len(selected))
	for _, requested := range selected {
		ref := requested.ProjectID
		if ref == "" {
			ref = requested.Path
		}
		repo, err := cfg.Repository(ref)
		if err != nil || repo.InstanceID != cfg.ActiveInstanceID {
			return nil, fmt.Errorf("repository is outside the selected GitLab instance")
		}
		result = append(result, repo)
	}
	return result, nil
}

func writeGitLabProjects(w http.ResponseWriter, projects []map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(projects)
}

func (s *Server) handleGitLabProjectsForInstance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var input struct {
		InstanceID string `json:"instance_id"`
		BaseURL    string `json:"base_url"`
		APIToken   string `json:"api_token"`
		RootGroup  string `json:"root_group"`
	}
	if r.Method == http.MethodPost {
		if err := decodeOpenJSON(r, &input); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
	} else {
		input.InstanceID = r.URL.Query().Get("instance_id")
		input.BaseURL = r.URL.Query().Get("base_url")
	}
	selected, err := s.config.GitLab.Connection(input.InstanceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if input.BaseURL == "" {
		input.BaseURL = selected.BaseURL
	}
	if input.APIToken == "" || input.APIToken == configuredSecretPlaceholder {
		if strings.TrimRight(input.BaseURL, "/") != strings.TrimRight(selected.BaseURL, "/") {
			http.Error(w, "Re-enter API token for a changed GitLab server", http.StatusBadRequest)
			return
		}
		input.APIToken = selected.APIToken
	}
	selected.BaseURL, selected.APIToken = input.BaseURL, input.APIToken
	client, err := newGitLabWebhookClient(&selected)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	group := strings.Trim(input.RootGroup, "/")
	for _, connection := range s.config.GitLab.Connections {
		if connection.ID == input.InstanceID && connection.RootGroup != "" {
			group = strings.Trim(connection.RootGroup, "/")
		}
	}
	path := "/api/v4/projects?membership=true&simple=true&order_by=id&sort=asc"
	if group != "" {
		path = "/api/v4/groups/" + url.PathEscape(group) + "/projects?include_subgroups=true&with_shared=false&order_by=id&sort=asc"
	}
	projects := []map[string]any{}
	for page := 1; page <= 100; page++ {
		req, err := client.newRequest(http.MethodGet, path+"&per_page=100&page="+strconv.Itoa(page), nil)
		if err != nil {
			http.Error(w, "Invalid GitLab request", http.StatusBadRequest)
			return
		}
		resp, err := client.client.Do(req.WithContext(r.Context()))
		if err != nil {
			http.Error(w, "GitLab project inventory unavailable", http.StatusBadGateway)
			return
		}
		var items []map[string]any
		err = json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&items)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK || err != nil {
			http.Error(w, "GitLab project inventory is incomplete", http.StatusBadGateway)
			return
		}
		for _, item := range items {
			clean := map[string]any{}
			for _, key := range []string{"id", "name", "path", "path_with_namespace", "namespace", "web_url", "archived", "default_branch"} {
				clean[key] = item[key]
			}
			projects = append(projects, clean)
		}
		next := resp.Header.Get("X-Next-Page")
		if next == "" && len(items) < 100 {
			writeGitLabProjects(w, projects)
			return
		}
		if _, present := resp.Header["X-Next-Page"]; present && next == "" {
			writeGitLabProjects(w, projects)
			return
		}
		if next != "" && next != strconv.Itoa(page+1) {
			http.Error(w, "Invalid GitLab pagination", http.StatusBadGateway)
			return
		}
	}
	http.Error(w, "GitLab project inventory exceeds the supported page budget", http.StatusBadGateway)
}

func (s *Server) gitLabClientForProject(project string) (*gitLabDeliveryClient, error) {
	connection, _, err := s.config.GitLab.ForRepository(project)
	if err != nil {
		return nil, err
	}
	if connection.ActiveInstanceID != "" && !connection.Enabled {
		return nil, fmt.Errorf("GitLab instance is disabled")
	}
	return newGitLabDeliveryClient(&connection)
}

func (s *Server) gitLabClientForExecution(run db.ExecutionRun) (*gitLabDeliveryClient, error) {
	connection, repo, err := s.config.GitLab.ForRepository(run.ProjectRef)
	if err != nil {
		return nil, err
	}
	if run.RepositorySourceJSON != "" {
		var frozen config.RepositorySource
		if err := json.Unmarshal([]byte(run.RepositorySourceJSON), &frozen); err != nil {
			return nil, fmt.Errorf("execution repository snapshot is invalid")
		}
		if frozen.RepositoryKey != s.config.GitLab.Source(repo, time.Now()).RepositoryKey {
			return nil, fmt.Errorf("execution repository endpoint changed; rerun preflight")
		}
	} else if connection.ActiveInstanceID != "" {
		return nil, fmt.Errorf("execution has no frozen instance identity")
	}
	if connection.ActiveInstanceID != "" && !connection.Enabled {
		return nil, fmt.Errorf("GitLab instance is disabled")
	}
	return newGitLabDeliveryClient(&connection)
}
