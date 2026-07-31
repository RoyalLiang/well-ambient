package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	appconfig "well-ambient/internal/config"
	"well-ambient/internal/db"

	"gorm.io/gorm"
)

type projectPreferenceOption struct {
	ProjectKey  string `json:"project_key"`
	ProjectName string `json:"project_name"`
}

type projectPreferenceResponse struct {
	Mode        string                    `json:"mode"`
	ProjectKeys []string                  `json:"project_keys"`
	Projects    []projectPreferenceOption `json:"projects"`
}

type updateProjectPreferenceRequest struct {
	ProjectKeys []string `json:"project_keys"`
}

func (s *Server) handleGetProjectPreferences(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.Header.Get("x-authenticated-user-id"))
	keys, err := db.LoadUserProjectPreferenceKeys(db.DB, username)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load project preferences: %v", err), http.StatusInternalServerError)
		return
	}
	projects, err := s.availableProjectPreferenceOptions()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load available projects: %v", err), http.StatusInternalServerError)
		return
	}

	writeProjectPreferenceResponse(w, keys, projects)
}

func (s *Server) handleUpdateProjectPreferences(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.Header.Get("x-authenticated-user-id"))
	if username == "" {
		http.Error(w, "Missing authenticated user", http.StatusUnauthorized)
		return
	}

	var req updateProjectPreferenceRequest
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	keys := db.NormalizeProjectKeys(req.ProjectKeys)
	projects, err := s.availableProjectPreferenceOptions()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load available projects: %v", err), http.StatusInternalServerError)
		return
	}
	available := make(map[string]struct{}, len(projects))
	for _, project := range projects {
		available[project.ProjectKey] = struct{}{}
	}
	for _, key := range keys {
		if _, exists := available[key]; !exists {
			http.Error(w, fmt.Sprintf("Unknown project key: %s", key), http.StatusBadRequest)
			return
		}
	}

	if err := db.ReplaceUserProjectPreferences(db.DB, username, keys); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save project preferences: %v", err), http.StatusInternalServerError)
		return
	}
	BroadcastNotifications()
	writeProjectPreferenceResponse(w, keys, projects)
}

func writeProjectPreferenceResponse(w http.ResponseWriter, keys []string, projects []projectPreferenceOption) {
	mode := "selected"
	if len(keys) == 0 {
		mode = "all"
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(projectPreferenceResponse{
		Mode:        mode,
		ProjectKeys: keys,
		Projects:    projects,
	})
}

func (s *Server) availableProjectPreferenceOptions() ([]projectPreferenceOption, error) {
	var configs []db.ProjectConfig
	if err := db.DB.Select("project_key", "project_name").Find(&configs).Error; err != nil {
		return nil, err
	}

	byKey := make(map[string]projectPreferenceOption, len(configs))
	for _, config := range configs {
		key := strings.ToUpper(strings.TrimSpace(config.ProjectKey))
		if key == "" {
			continue
		}
		name := strings.TrimSpace(config.ProjectName)
		if name == "" {
			name = key
		}
		byKey[key] = projectPreferenceOption{ProjectKey: key, ProjectName: name}
	}

	if s.config != nil {
		configuredKeys := appconfig.JiraProjectKeys(&s.config.Jira)
		if len(configuredKeys) > 0 {
			configuredNames := make(map[string]string, len(s.config.Jira.VersionSources))
			for _, source := range s.config.Jira.VersionSources {
				key := strings.ToUpper(strings.TrimSpace(source.ProjectKey))
				if key != "" && strings.TrimSpace(source.ProjectName) != "" {
					configuredNames[key] = strings.TrimSpace(source.ProjectName)
				}
			}
			scoped := make(map[string]projectPreferenceOption, len(configuredKeys))
			for _, rawKey := range configuredKeys {
				key := strings.ToUpper(strings.TrimSpace(rawKey))
				if key == "" {
					continue
				}
				option, exists := byKey[key]
				if !exists {
					option = projectPreferenceOption{ProjectKey: key, ProjectName: firstNonBlank(configuredNames[key], key)}
				} else if name := configuredNames[key]; name != "" {
					option.ProjectName = name
				}
				scoped[key] = option
			}
			byKey = scoped
		}
	}

	projects := make([]projectPreferenceOption, 0, len(byKey))
	for _, project := range byKey {
		projects = append(projects, project)
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].ProjectKey < projects[j].ProjectKey
	})
	return projects, nil
}

func requestProjectPreferenceKeys(r *http.Request) ([]string, error) {
	return db.LoadUserProjectPreferenceKeys(db.DB, strings.TrimSpace(r.Header.Get("x-authenticated-user-id")))
}

func applyRequestProjectScope(tx *gorm.DB, r *http.Request) (*gorm.DB, error) {
	keys, err := requestProjectPreferenceKeys(r)
	if err != nil {
		return nil, err
	}
	return db.ApplyTaskProjectScope(tx, keys), nil
}

func requestCanAccessTask(r *http.Request, taskID string) (bool, error) {
	keys, err := requestProjectPreferenceKeys(r)
	if err != nil {
		return false, err
	}
	return db.TaskMatchesProjectScope(taskID, keys), nil
}
