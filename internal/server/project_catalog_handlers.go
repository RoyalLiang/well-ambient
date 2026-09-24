package server

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

// handleGetProjectCatalog lists settings candidates without changing business
// project scopes or materializing repository mappings for discovered projects.
func (s *Server) handleGetProjectCatalog(w http.ResponseWriter, r *http.Request) {
	var mappings []db.ProjectConfig
	if err := db.DB.WithContext(r.Context()).
		Select("project_key", "project_name").Order("id").Find(&mappings).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var jiraProjects []db.TaskTelemetry
	if err := db.DB.WithContext(r.Context()).
		Where("source = ? AND TRIM(project_key) <> ''", "jira").
		Distinct("project_key").Find(&jiraProjects).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var legacyTasks []db.TaskTelemetry
	if err := db.DB.WithContext(r.Context()).
		Select("external_key", "task_id", "repo").
		Where("source = ? AND (project_key IS NULL OR TRIM(project_key) = '')", "jira").
		Order("task_id").Find(&legacyTasks).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var jiraNames []db.TaskTelemetry
	if err := db.DB.WithContext(r.Context()).
		Where("source = ? AND TRIM(project_key) <> '' AND TRIM(repo) LIKE ?", "jira", "% (%)").
		Distinct("project_key", "repo").Order("project_key, repo").Find(&jiraNames).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	byKey := make(map[string]projectPreferenceOption)
	add := func(rawKey, rawName string) {
		key := strings.ToUpper(strings.TrimSpace(rawKey))
		if key == "" {
			return
		}
		name := strings.TrimSpace(rawName)
		option, exists := byKey[key]
		if !exists {
			option = projectPreferenceOption{ProjectKey: key, ProjectName: key}
		}
		if name != "" && !strings.EqualFold(name, key) {
			option.ProjectName = name
		}
		byKey[key] = option
	}

	for _, project := range jiraProjects {
		add(db.ResolveTaskProjectKey(project), "")
	}
	addJiraName := func(key, repo string) {
		add(key, "")
		key = strings.ToUpper(strings.TrimSpace(key))
		option := byKey[key]
		repo = strings.TrimSpace(repo)
		suffix := " (" + key + ")"
		if key != "" && option.ProjectName == key &&
			len(repo) > len(suffix) && strings.EqualFold(repo[len(repo)-len(suffix):], suffix) {
			add(key, strings.TrimSpace(repo[:len(repo)-len(suffix)]))
		}
	}
	for _, task := range legacyTasks {
		// Only Jira rows can use the legacy issue-key fallback. Prefer the
		// external identity when the local task ID is not the Jira key.
		externalKey := strings.ToUpper(strings.TrimSpace(task.ExternalKey))
		if looksLikeJiraIssueKey(externalKey) {
			task.TaskID = externalKey
		} else if !looksLikeJiraIssueKey(strings.ToUpper(strings.TrimSpace(task.TaskID))) {
			continue
		}
		addJiraName(db.ResolveTaskProjectKey(task), task.Repo)
	}
	for _, project := range jiraNames {
		addJiraName(db.ResolveTaskProjectKey(project), project.Repo)
	}
	if s.config != nil {
		for _, key := range config.JiraProjectKeys(&s.config.Jira) {
			add(key, "")
		}
		for _, source := range s.config.Jira.VersionSources {
			key := source.ProjectKey
			if strings.TrimSpace(key) == "" {
				if ref, err := config.ParseJiraVersionURL(s.config.Jira.BaseURL, source.VersionURL); err == nil {
					key = ref.ProjectKey
				}
			}
			add(key, source.ProjectName)
		}
	}
	for _, mapping := range mappings {
		// Explicit mapping names take precedence over version-source labels.
		add(mapping.ProjectKey, mapping.ProjectName)
	}

	projects := make([]projectPreferenceOption, 0, len(byKey))
	for _, project := range byKey {
		projects = append(projects, project)
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].ProjectKey < projects[j].ProjectKey
	})
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(struct {
		Projects []projectPreferenceOption `json:"projects"`
	}{Projects: projects})
}
