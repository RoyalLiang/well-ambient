package server

import (
	"encoding/json"
	"net/http"
	"time"
	"well-ambient/internal/db"
	"well-ambient/internal/telemetry"
)

// handleGetProjectConfigs returns all project configurations
func (s *Server) handleGetProjectConfigs(w http.ResponseWriter, r *http.Request) {
	var configs []db.ProjectConfig
	if err := db.DB.Find(&configs).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(configs)
}

// handleSaveProjectConfig saves or updates a project configuration
func (s *Server) handleSaveProjectConfig(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ProjectKey   string `json:"project_key"`
		ProjectName  string `json:"project_name"`
		GitReposJSON string `json:"git_repos_json"`
		BasePriority string `json:"base_priority"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if input.ProjectKey == "" {
		http.Error(w, "project_key is required", http.StatusBadRequest)
		return
	}

	var config db.ProjectConfig
	err := db.DB.Where("project_key = ?", input.ProjectKey).First(&config).Error
	if err == nil {
		// Update
		config.ProjectName = input.ProjectName
		config.GitReposJSON = input.GitReposJSON
		config.BasePriority = input.BasePriority
		config.UpdatedAt = time.Now()
		if err := db.DB.Save(&config).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		// Create
		config = db.ProjectConfig{
			ProjectKey:   input.ProjectKey,
			ProjectName:  input.ProjectName,
			GitReposJSON: input.GitReposJSON,
			BasePriority: input.BasePriority,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		if err := db.DB.Create(&config).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Trigger recalculate scores
	_, _ = telemetry.CalculateAndSaveScores()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// handleGetProjectScores returns the latest scores for all projects
func (s *Server) handleGetProjectScores(w http.ResponseWriter, r *http.Request) {
	// First ensure scores are calculated
	scores, err := telemetry.CalculateAndSaveScores()
	if err != nil {
		// Fallback to query existing scores if calculator fails
		if errQuery := db.DB.Order("snapshot_date desc").Find(&scores).Error; errQuery != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scores)
}

// handleCalculateProjectScores manually triggers scoring calculation
func (s *Server) handleCalculateProjectScores(w http.ResponseWriter, r *http.Request) {
	scores, err := telemetry.CalculateAndSaveScores()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scores)
}
