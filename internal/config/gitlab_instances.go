package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// The legacy fields remain the default connection. Other instances never inherit
// its credentials, even when remote project IDs or repository names are equal.
type GitLabConnection struct {
	ID        string `yaml:"id" json:"id"`
	Name      string `yaml:"name" json:"name"`
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	BaseURL   string `yaml:"base_url" json:"base_url"`
	APIToken  string `yaml:"api_token" json:"api_token"`
	Secret    string `yaml:"secret_token" json:"secret_token"`
	RootGroup string `yaml:"root_group,omitempty" json:"root_group,omitempty"`
}

// RepositorySource is a frozen attribution record, not a live configuration join.
type RepositorySource struct {
	LegacyDedupe    bool      `json:"legacy_dedupe,omitempty"`
	SchemaVersion   int       `json:"schema_version"`
	InstanceName    string    `json:"instance_name"`
	Provider        string    `json:"provider"`
	InstanceID      string    `json:"instance_id"`
	InstanceURL     string    `json:"instance_url"`
	RepositoryKey   string    `json:"repository_key"`
	ProjectID       string    `json:"project_id"`
	RemoteProjectID string    `json:"remote_project_id"`
	Name            string    `json:"name"`
	FullPath        string    `json:"full_path"`
	NamespaceName   string    `json:"namespace_name"`
	NamespacePath   string    `json:"namespace_path"`
	WebURL          string    `json:"web_url"`
	CapturedAt      time.Time `json:"captured_at"`
}

func (r RepoMapping) RemoteID() string {
	if r.RemoteProjectID != "" {
		return r.RemoteProjectID
	}
	if r.InstanceID == "" {
		return r.ProjectID
	}
	return ""
}

func (r RepoMapping) DisplayName() string {
	if r.InstanceID != "" {
		return r.InstanceID + " / " + r.Path
	}
	return r.Name
}

func (c GitLabConfig) Connection(id string) (GitLabConfig, error) {
	id = strings.TrimSpace(id)
	if id == "default" {
		id = ""
	}
	if id == c.ActiveInstanceID {
		out := c
		out.Repos = nil
		for _, repo := range c.Repos {
			if repo.InstanceID == id {
				out.Repos = append(out.Repos, repo)
			}
		}
		return out, nil
	}
	for _, connection := range c.Connections {
		if connection.ID == id {
			out := c
			out.Enabled, out.BaseURL, out.APIToken, out.Secret = connection.Enabled, connection.BaseURL, connection.APIToken, connection.Secret
			out.ActiveInstanceID = id
			out.Repos = nil
			for _, repo := range c.Repos {
				if repo.InstanceID == id {
					out.Repos = append(out.Repos, repo)
				}
			}
			return out, nil
		}
	}
	return GitLabConfig{}, fmt.Errorf("GitLab instance %q is not configured", id)
}

func (c GitLabConfig) Repository(project string) (RepoMapping, error) {
	project = strings.TrimSpace(project)
	if project == "" {
		return RepoMapping{}, fmt.Errorf("repository is required")
	}
	for _, repo := range c.Repos {
		if repo.ProjectID == project {
			return repo, nil
		}
	}
	var matches []RepoMapping
	for _, repo := range c.Repos {
		if repo.Path == project || repo.Name == project {
			matches = append(matches, repo)
		}
	}
	if len(matches) != 1 {
		return RepoMapping{}, fmt.Errorf("repository is unconfigured or ambiguous; use its instance-qualified project ID")
	}
	return matches[0], nil
}

func (c GitLabConfig) ForRepository(project string) (GitLabConfig, RepoMapping, error) {
	repo, err := c.Repository(project)
	if err != nil {
		return GitLabConfig{}, repo, err
	}
	connection, err := c.Connection(repo.InstanceID)
	return connection, repo, err
}

// RouteAPIPath binds qualified project requests to the corresponding endpoint.
// Unknown qualified IDs are rejected; they must never fall back to the default PAT.
func (c GitLabConfig) RouteAPIPath(path string) (GitLabConfig, string, error) {
	const prefix = "/projects/"
	if !strings.HasPrefix(path, prefix) {
		return c, path, nil
	}
	remaining := strings.TrimPrefix(path, prefix)
	end := strings.IndexAny(remaining, "/?")
	if end < 0 {
		end = len(remaining)
	}
	project, err := url.PathUnescape(remaining[:end])
	if err != nil {
		return GitLabConfig{}, "", err
	}
	connection, repo, err := c.ForRepository(project)
	if err != nil {
		if strings.Contains(project, ":") {
			return GitLabConfig{}, "", err
		}
		return c, path, nil // legacy direct API calls retain their existing connection
	}
	remote := repo.RemoteID()
	if connection.ActiveInstanceID != "" && (!connection.Enabled || strings.TrimSpace(connection.APIToken) == "") {
		return GitLabConfig{}, "", fmt.Errorf("GitLab instance is disabled or has no API token")
	}
	if remote == "" {
		return GitLabConfig{}, "", fmt.Errorf("remote repository ID is required")
	}
	return connection, prefix + url.PathEscape(remote) + remaining[end:], nil
}

func (c GitLabConfig) Source(repo RepoMapping, at time.Time) RepositorySource {
	connection, err := c.Connection(repo.InstanceID)
	if err != nil {
		return RepositorySource{}
	}
	base := strings.TrimRight(connection.BaseURL, "/")
	remote := repo.RemoteID()
	namespace := repo.NamespacePath
	if namespace == "" {
		if index := strings.LastIndex(repo.Path, "/"); index >= 0 {
			namespace = repo.Path[:index]
		}
	}
	webURL := repo.WebURL
	if webURL == "" && repo.Path != "" {
		webURL = base + "/" + strings.TrimPrefix(repo.Path, "/")
	}
	id := repo.InstanceID
	if id == "" {
		id = "default"
	}
	instanceName := id
	for _, connection := range c.Connections {
		if connection.ID == repo.InstanceID {
			instanceName = connection.Name
		}
	}
	return RepositorySource{LegacyDedupe: c.LegacyDedupeOrigin != "" && repo.InstanceID == "" && strings.TrimRight(c.LegacyDedupeOrigin, "/") == base, SchemaVersion: 1, InstanceName: instanceName, Provider: "gitlab", InstanceID: id, InstanceURL: base, RepositoryKey: "gitlab:" + base + ":" + remote, ProjectID: repo.ProjectID, RemoteProjectID: remote, Name: repo.Name, FullPath: repo.Path, NamespaceName: repo.NamespaceName, NamespacePath: namespace, WebURL: webURL, CapturedAt: at.UTC()}
}

func (s RepositorySource) JSON() string { data, _ := json.Marshal(s); return string(data) }

var gitLabInstanceID = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

func ValidateGitLabURL(value string) error {
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) {
		return fmt.Errorf("GitLab URL must be a trusted HTTPS endpoint")
	}
	return nil
}

func (c GitLabConfig) ValidateInstances() error {
	seen := map[string]bool{"default": true}
	for _, connection := range c.Connections {
		if !gitLabInstanceID.MatchString(connection.ID) || seen[connection.ID] {
			return fmt.Errorf("GitLab instance IDs must be unique lowercase identifiers")
		}
		seen[connection.ID] = true
		if err := ValidateGitLabURL(connection.BaseURL); err != nil {
			return err
		}
	}
	ids := map[string]bool{}
	for _, repo := range c.Repos {
		if repo.ProjectID == "" || ids[repo.ProjectID] {
			return fmt.Errorf("repository project IDs must be unique")
		}
		ids[repo.ProjectID] = true
		if repo.InstanceID != "" && (!seen[repo.InstanceID] || repo.RemoteID() == "" || repo.ProjectID != repo.InstanceID+":"+repo.RemoteID()) {
			return fmt.Errorf("repository must belong to its configured GitLab instance")
		}
	}
	return nil
}
