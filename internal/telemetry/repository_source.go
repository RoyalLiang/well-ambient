package telemetry

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"well-ambient/internal/config"
)

func webhookRepositorySource(cfg *config.Config, body []byte) (config.RepositorySource, error) {
	var payload struct {
		Project struct {
			ID     int    `json:"id"`
			Name   string `json:"name"`
			Path   string `json:"path_with_namespace"`
			WebURL string `json:"web_url"`
		} `json:"project"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return config.RepositorySource{}, err
	}
	remote := strconv.Itoa(payload.Project.ID)
	for _, repo := range cfg.GitLab.Repos {
		if repo.InstanceID != cfg.GitLab.ActiveInstanceID {
			continue
		}
		if (payload.Project.ID > 0 && repo.RemoteID() == remote) || (payload.Project.ID == 0 && cfg.GitLab.ActiveInstanceID == "" && repo.Name == payload.Project.Name) {
			source := cfg.GitLab.Source(repo, time.Now())
			if cfg.GitLab.ActiveInstanceID != "" {
				base, err := url.Parse(cfg.GitLab.BaseURL)
				if err != nil {
					return source, err
				}
				web, err := url.Parse(payload.Project.WebURL)
				if err != nil || web.User != nil || web.RawQuery != "" || web.Fragment != "" || web.Scheme != base.Scheme || !strings.EqualFold(web.Host, base.Host) || !strings.HasPrefix(web.Path, strings.TrimRight(base.Path, "/")+"/") {
					return source, fmt.Errorf("webhook repository does not belong to the selected GitLab instance")
				}
				fullPath := strings.TrimPrefix(web.Path, strings.TrimRight(base.Path, "/")+"/")
				for _, connection := range cfg.GitLab.Connections {
					if connection.ID == cfg.GitLab.ActiveInstanceID && connection.RootGroup != "" && !strings.HasPrefix(fullPath, strings.Trim(connection.RootGroup, "/")+"/") {
						return source, fmt.Errorf("webhook repository is outside the configured group")
					}
				}
				source.FullPath, source.WebURL, source.Name = fullPath, payload.Project.WebURL, payload.Project.Name
				if index := strings.LastIndex(fullPath, "/"); index >= 0 {
					if source.NamespacePath != fullPath[:index] {
						source.NamespaceName = ""
					}
					source.NamespacePath = fullPath[:index]
				}
			}
			return source, nil
		}
	}
	if cfg.GitLab.ActiveInstanceID != "" {
		return config.RepositorySource{}, fmt.Errorf("webhook repository is not configured for this instance")
	}
	return config.RepositorySource{}, nil
}

func sourceRepoLabel(source config.RepositorySource, fallback string) string {
	if source.InstanceID != "" && source.InstanceID != "default" {
		return source.InstanceID + " / " + source.FullPath
	}
	return fallback
}
