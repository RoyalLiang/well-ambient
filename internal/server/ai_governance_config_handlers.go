package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

type solutionPublicURLResponse struct {
	PublicURL string `json:"public_url"`
	Version   int    `json:"version"`
}

type solutionPublicURLUpdateRequest struct {
	PublicURL       string `json:"public_url"`
	ExpectedVersion int    `json:"expected_version"`
}

type aiContextReadinessResponse struct {
	Ready  bool   `json:"ready"`
	Status string `json:"status"`
}

func (s *Server) handleGetSolutionPublicURL(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	s.configMutationMu.Lock()
	defer s.configMutationMu.Unlock()

	var current db.RuntimeConfig
	if err := db.DB.First(&current, runtimeConfigSingletonID).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to read current configuration: %v", err), http.StatusInternalServerError)
		return
	}
	restored, err := restoreVersionedConfig(*s.config, current.ConfigJSON)
	if err != nil {
		http.Error(w, fmt.Sprintf("Current database configuration is invalid: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(solutionPublicURLResponse{
		PublicURL: restored.Server.PublicURL,
		Version:   current.Version,
	})
}

func (s *Server) handleUpdateSolutionPublicURL(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	var request solutionPublicURLUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		return
	}
	publicURL, err := normalizeSolutionPublicURL(request.PublicURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.configMutationMu.Lock()
	defer s.configMutationMu.Unlock()

	var current db.RuntimeConfig
	if err := db.DB.First(&current, runtimeConfigSingletonID).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to read current configuration: %v", err), http.StatusInternalServerError)
		return
	}
	if request.ExpectedVersion != current.Version {
		writeLoginError(w, http.StatusConflict, "config_version_conflict", "配置已被其他操作更新，请刷新后重试。")
		return
	}

	previous, err := restoreVersionedConfig(*s.config, current.ConfigJSON)
	if err != nil {
		http.Error(w, fmt.Sprintf("Current database configuration is invalid: %v", err), http.StatusInternalServerError)
		return
	}
	next := previous
	next.Server.PublicURL = publicURL

	version, err := s.recordConfigVersion(previous, next, r, "solution-public-url", 0)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to persist solution public URL: %v", err), http.StatusInternalServerError)
		return
	}
	if err := s.applyConfig(next); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	userdb.RecordAuditLog(
		db.DB,
		r.Header.Get("x-authenticated-user-id"),
		"solution_public_url_updated",
		"config",
		"solution-public-url",
		fmt.Sprintf("Solution public URL updated at config version %d", version.Version),
		r.RemoteAddr,
	)
	BroadcastConfigUpdated(configVersionDTO(version))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(solutionPublicURLResponse{PublicURL: publicURL, Version: version.Version})
}

func normalizeSolutionPublicURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("public_url must be an absolute HTTP(S) URL")
	}
	return strings.TrimRight(value, "/"), nil
}

func (s *Server) handleGetAIContextReadiness(w http.ResponseWriter, r *http.Request) {
	s.configMutationMu.Lock()
	defer s.configMutationMu.Unlock()

	response := aiContextReadinessResponse{Status: "disabled"}
	if s.config != nil && s.config.AI.Enabled {
		response.Status = "incomplete"
		if strings.TrimSpace(s.config.AI.BaseURL) != "" && strings.TrimSpace(s.config.AI.APIToken) != "" {
			response.Ready = true
			response.Status = "ready"
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}
