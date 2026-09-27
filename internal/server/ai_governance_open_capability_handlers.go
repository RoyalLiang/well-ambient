package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/openaccess"
	"well-ambient/internal/openmcp"
)

type openCapabilitiesOverviewResponse struct {
	Config         config.OpenCapabilitiesConfig `json:"config"`
	Version        int                           `json:"version"`
	ActiveFeatures openCapabilityFeatures        `json:"active_features"`
	BaseURL        string                        `json:"base_url"`
	MCP            openCapabilitiesMCPInfo       `json:"mcp"`
	Policy         *openCapabilitiesPolicyInfo   `json:"policy,omitempty"`
	Sources        []db.IntegrationSource        `json:"sources"`
	Credentials    []db.IntegrationCredential    `json:"credentials"`
	Bindings       []db.JiraExecutionBinding     `json:"bindings"`
	Skills         []openSkillSummary            `json:"skills"`
	Intelligence   any                           `json:"intelligence,omitempty"`
}

type openCapabilitiesMCPInfo struct {
	EndpointURL   string   `json:"endpoint_url"`
	ServerName    string   `json:"server_name"`
	ServerVersion string   `json:"server_version"`
	Tools         []string `json:"tools"`
}

type openCapabilitiesPolicyInfo struct {
	Version             int                 `json:"version"`
	Digest              string              `json:"digest"`
	AllowedProjects     []string            `json:"allowed_projects"`
	AllowedRepositories []string            `json:"allowed_repositories"`
	Actions             []string            `json:"actions"`
	DataRules           map[string]string   `json:"data_rules"`
	FieldRules          map[string][]string `json:"field_rules"`
}

type updateOpenCapabilitiesConfigRequest struct {
	Enabled         bool `json:"enabled"`
	ReadEnabled     bool `json:"read_enabled"`
	PrepareEnabled  bool `json:"prepare_enabled"`
	ExecuteEnabled  bool `json:"execute_enabled"`
	ExpectedVersion int  `json:"expected_version"`
}

type issueCredentialRequest struct {
	SourceID string `json:"source_id"`
	TTLHours int    `json:"ttl_hours"`
}

type createSourceRequest struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Owner        string `json:"owner"`
	QuotaProfile string `json:"quota_profile"`
}

func (s *Server) handleGetAIGovernanceOpenCapabilitiesOverview(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}
	s.ensureOpenCapabilities()

	s.configMutationMu.Lock()
	version := 0
	var current db.RuntimeConfig
	if err := db.DB.First(&current, runtimeConfigSingletonID).Error; err == nil {
		version = current.Version
	}
	s.configMutationMu.Unlock()

	baseURL := strings.TrimRight(strings.TrimSpace(s.config.Server.PublicURL), "/")
	if baseURL == "" {
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		host := r.Host
		if host == "" {
			host = fmt.Sprintf("localhost:%d", s.config.Server.Port)
		}
		baseURL = fmt.Sprintf("%s://%s", scheme, host)
	}

	response := openCapabilitiesOverviewResponse{
		Config:         s.config.OpenCapabilities,
		Version:        version,
		ActiveFeatures: s.openFeatures,
		BaseURL:        baseURL,
		MCP: openCapabilitiesMCPInfo{
			EndpointURL:   fmt.Sprintf("%s/mcp", baseURL),
			ServerName:    openmcp.ServerName,
			ServerVersion: openmcp.ServerVersion,
			Tools:         openmcp.ToolNames,
		},
		Skills: getOfficialSkillsCatalog(baseURL),
	}

	if s.openAccess != nil {
		if policy, err := s.openAccess.ActivePolicy(r.Context()); err == nil && policy.Version > 0 {
			response.Policy = &openCapabilitiesPolicyInfo{
				Version:             policy.Version,
				Digest:              policy.Digest,
				AllowedProjects:     policy.AllowedProjects,
				AllowedRepositories: policy.AllowedRepositories,
				Actions:             policy.Actions,
				DataRules:           policy.DataRules,
				FieldRules:          policy.FieldRules,
			}
		}

		if sources, err := s.openAccess.Sources(r.Context()); err == nil {
			response.Sources = sources
		} else {
			response.Sources = []db.IntegrationSource{}
		}

		if creds, err := s.openAccess.Credentials(r.Context(), ""); err == nil {
			response.Credentials = creds
		} else {
			response.Credentials = []db.IntegrationCredential{}
		}

		if bindings, err := s.openAccess.ExecutionBindings(r.Context()); err == nil {
			response.Bindings = bindings
		} else {
			response.Bindings = []db.JiraExecutionBinding{}
		}

		if report, err := s.openAccess.Intelligence(r.Context(), 30*24*time.Hour); err == nil {
			response.Intelligence = report
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (s *Server) handleUpdateAIGovernanceOpenCapabilitiesConfig(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	var req updateOpenCapabilitiesConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		return
	}

	s.configMutationMu.Lock()
	defer s.configMutationMu.Unlock()

	var current db.RuntimeConfig
	if err := db.DB.First(&current, runtimeConfigSingletonID).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to read current configuration: %v", err), http.StatusInternalServerError)
		return
	}
	if req.ExpectedVersion != current.Version {
		writeLoginError(w, http.StatusConflict, "config_version_conflict", "配置已被其他操作更新，请刷新后重试。")
		return
	}

	previous, err := restoreVersionedConfig(*s.config, current.ConfigJSON)
	if err != nil {
		http.Error(w, fmt.Sprintf("Current database configuration is invalid: %v", err), http.StatusInternalServerError)
		return
	}

	next := previous
	next.OpenCapabilities = config.OpenCapabilitiesConfig{
		Enabled:        req.Enabled,
		ReadEnabled:    req.ReadEnabled,
		PrepareEnabled: req.PrepareEnabled,
		ExecuteEnabled: req.ExecuteEnabled,
	}

	newVersion, err := s.recordConfigVersion(previous, next, r, "open-capabilities-config", 0)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to persist configuration version: %v", err), http.StatusInternalServerError)
		return
	}
	if err := s.applyConfig(next); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"version":         newVersion,
		"config":          next.OpenCapabilities,
		"active_features": s.openFeatures,
	})
}

func (s *Server) handleIssueAIGovernanceCredential(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil || s.openAccess == nil {
		http.Error(w, "Open access service unavailable", http.StatusInternalServerError)
		return
	}

	var req issueCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		return
	}
	sourceID := strings.TrimSpace(req.SourceID)
	if sourceID == "" {
		sourceID = "target-agent"
	}
	ttlHours := req.TTLHours
	if ttlHours <= 0 {
		ttlHours = 720 // default 30 days
	}

	// Ensure source exists
	if _, err := s.openAccess.Source(r.Context(), sourceID); err != nil {
		_, _ = s.openAccess.CreateSource(r.Context(), sourceID, strings.ToUpper(sourceID), "AI Governance", openaccess.DefaultQuotaProfile)
	}

	issued, err := s.openAccess.IssueCredential(r.Context(), sourceID, time.Duration(ttlHours)*time.Hour)
	if err != nil {
		writeOpenError(w, "", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"key_id":     issued.KeyID,
		"key":        issued.Secret,
		"key_prefix": issued.Prefix,
		"source_id":  sourceID,
		"expires_at": issued.ExpiresAt,
	})
}

func (s *Server) handleRevokeAIGovernanceCredential(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil || s.openAccess == nil {
		http.Error(w, "Open access service unavailable", http.StatusInternalServerError)
		return
	}

	keyID := strings.TrimSpace(r.PathValue("id"))
	if keyID == "" {
		http.Error(w, "Key ID is required", http.StatusBadRequest)
		return
	}

	if err := s.openAccess.RevokeCredential(r.Context(), keyID); err != nil {
		writeOpenError(w, "", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"key_id": keyID,
		"status": openaccess.CredentialRevoked,
	})
}

func (s *Server) handleCreateAIGovernanceSource(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil || s.openAccess == nil {
		http.Error(w, "Open access service unavailable", http.StatusInternalServerError)
		return
	}

	var req createSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		return
	}
	id := strings.TrimSpace(req.ID)
	name := strings.TrimSpace(req.Name)
	if id == "" || name == "" {
		http.Error(w, "Source ID and Name are required", http.StatusBadRequest)
		return
	}
	quota := strings.TrimSpace(req.QuotaProfile)
	if quota == "" {
		quota = openaccess.DefaultQuotaProfile
	}

	source, err := s.openAccess.CreateSource(r.Context(), id, name, strings.TrimSpace(req.Owner), quota)
	if err != nil {
		writeOpenError(w, "", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"source": source,
	})
}

func (s *Server) handleUpdateAIGovernancePolicy(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil || s.openAccess == nil {
		http.Error(w, "Open access service unavailable", http.StatusInternalServerError)
		return
	}

	var spec openaccess.PolicySpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		return
	}

	actor := "ai-governance-admin"
	policy, err := s.openAccess.ActivatePolicy(r.Context(), spec, actor)
	if err != nil {
		writeOpenError(w, "", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"policy": policy,
	})
}

func (s *Server) handleUpsertAIGovernanceBinding(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil || s.openAccess == nil {
		http.Error(w, "Open access service unavailable", http.StatusInternalServerError)
		return
	}

	var binding db.JiraExecutionBinding
	if err := json.NewDecoder(r.Body).Decode(&binding); err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		return
	}

	if err := s.openAccess.UpsertExecutionBinding(r.Context(), binding); err != nil {
		writeOpenError(w, "", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "success",
	})
}
