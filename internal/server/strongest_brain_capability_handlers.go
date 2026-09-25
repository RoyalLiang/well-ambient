package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"well-ambient/internal/agentruntime"
	"well-ambient/internal/db"
	"well-ambient/internal/strongestbrain"
)

// handleGetStrongestBrainCapabilityIntelligence handles GET /api/strongest-brain/capability-intelligence
func (s *Server) handleGetStrongestBrainCapabilityIntelligence(w http.ResponseWriter, r *http.Request) {
	if s.strongestBrain == nil || db.DB == nil {
		http.Error(w, "Strongest brain service not initialized", http.StatusInternalServerError)
		return
	}
	limit := boundedQueryLimit(r, 500, 1000)
	report, err := s.strongestBrain.GetIntelligence(r.Context(), limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to compute capability intelligence: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}

// handleListStrongestBrainCapabilityProposals handles GET /api/strongest-brain/capability-proposals
func (s *Server) handleListStrongestBrainCapabilityProposals(w http.ResponseWriter, r *http.Request) {
	if s.strongestBrain == nil || db.DB == nil {
		http.Error(w, "Strongest brain service not initialized", http.StatusInternalServerError)
		return
	}
	status := r.URL.Query().Get("status")
	list, err := s.strongestBrain.ListProposals(r.Context(), status)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list proposals: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"proposals": list,
		"total":     len(list),
	})
}

// handleReviewStrongestBrainCapabilityProposal handles POST /api/strongest-brain/capability-proposals/{id}/review
func (s *Server) handleReviewStrongestBrainCapabilityProposal(w http.ResponseWriter, r *http.Request) {
	if s.strongestBrain == nil || db.DB == nil {
		http.Error(w, "Strongest brain service not initialized", http.StatusInternalServerError)
		return
	}
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		http.Error(w, "invalid proposal ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Decision string `json:"decision"` // approved, rejected
		Notes    string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	reviewer := r.Header.Get("x-authenticated-user-id")
	if reviewer == "" {
		reviewer = "admin"
	}

	proposal, err := s.strongestBrain.ReviewProposal(r.Context(), uint(id), req.Decision, reviewer, req.Notes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(proposal)
}

// handleRunAgentRuntimeReplay handles POST /api/agent-runtime/replays
func (s *Server) handleRunAgentRuntimeReplay(w http.ResponseWriter, r *http.Request) {
	if s.strongestBrain == nil || db.DB == nil {
		http.Error(w, "Strongest brain service not initialized", http.StatusInternalServerError)
		return
	}
	var cmd strongestbrain.ReplayCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	eval, err := s.strongestBrain.RunReplay(r.Context(), cmd)
	if err != nil {
		http.Error(w, fmt.Sprintf("Replay evaluation failed: %v", err), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(eval)
}

// handleGetAgentRuntimeReplay handles GET /api/agent-runtime/replays/{id}
func (s *Server) handleGetAgentRuntimeReplay(w http.ResponseWriter, r *http.Request) {
	if s.strongestBrain == nil || db.DB == nil {
		http.Error(w, "Strongest brain service not initialized", http.StatusInternalServerError)
		return
	}
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		http.Error(w, "invalid replay ID", http.StatusBadRequest)
		return
	}
	eval, err := s.strongestBrain.GetReplay(r.Context(), uint(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(eval)
}

// handleListAgentCapabilities handles GET /api/agent-runtime/capabilities
func (s *Server) handleListAgentCapabilities(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	filterStatus := r.URL.Query().Get("status")
	filterKind := r.URL.Query().Get("kind")

	query := db.DB.WithContext(r.Context()).Order("capability_key ASC")
	if filterStatus != "" {
		query = query.Where("status = ?", filterStatus)
	}
	if filterKind != "" {
		query = query.Where("kind = ?", filterKind)
	}
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("capability_key LIKE ? OR owner LIKE ? OR description LIKE ?", like, like, like)
	}

	var caps []db.Capability
	if err := query.Find(&caps).Error; err != nil {
		http.Error(w, "Failed to query capabilities", http.StatusInternalServerError)
		return
	}

	var versions []db.CapabilityVersion
	_ = db.DB.WithContext(r.Context()).Order("version DESC").Find(&versions).Error

	capVersionsMap := make(map[uint][]db.CapabilityVersion)
	var verIDs []uint
	verBytesMap := make(map[uint]int64)
	for _, v := range versions {
		capVersionsMap[v.CapabilityID] = append(capVersionsMap[v.CapabilityID], v)
		verIDs = append(verIDs, v.ID)
		verBytesMap[v.ID] = int64(len(v.ManifestJSON))
	}

	// Compute resource contents disk size per version
	if len(verIDs) > 0 {
		var resources []db.CapabilityResource
		_ = db.DB.WithContext(r.Context()).Select("capability_version_id, content").Where("capability_version_id IN ?", verIDs).Find(&resources).Error
		for _, res := range resources {
			verBytesMap[res.CapabilityVersionID] += int64(len(res.Content))
		}
	}

	// Aggregate run capability bindings count per capability key
	var bindingCounts []struct {
		CapabilityKey string `gorm:"column:capability_key"`
		Count         int64  `gorm:"column:count"`
	}
	_ = db.DB.WithContext(r.Context()).Model(&db.RunCapabilityBinding{}).
		Select("capability_key, count(*) as count").
		Group("capability_key").
		Scan(&bindingCounts).Error
	capBindingsMap := make(map[string]int64, len(bindingCounts))
	for _, bc := range bindingCounts {
		capBindingsMap[bc.CapabilityKey] = bc.Count
	}

	type CapabilityDTO struct {
		ID                 uint                                  `json:"id"`
		CapabilityKey      string                                `json:"capability_key"`
		Kind               string                                `json:"kind"`
		Status             string                                `json:"status"`
		Owner              string                                `json:"owner"`
		Sensitivity        string                                `json:"sensitivity"`
		Description        string                                `json:"description"`
		DiskSizeBytes      int64                                 `json:"disk_size_bytes"`
		DiskSizeFormatted  string                                `json:"disk_size_formatted"`
		BindingsCount      int64                                 `json:"bindings_count"`
		IsArchived         bool                                  `json:"is_archived"`
		IsTopLevelSkill    bool                                  `json:"is_top_level_skill"`
		ParentSkillKey     string                                `json:"parent_skill_key,omitempty"`
		IncludedComponents []agentruntime.IncludedComponentDTO   `json:"included_components,omitempty"`
		LatestVersion      int                                   `json:"latest_version"`
		Versions           []db.CapabilityVersion                `json:"versions"`
		CreatedAt          string                                `json:"created_at"`
		UpdatedAt          string                                `json:"updated_at"`
	}

	var totalDiskSize int64
	result := make([]CapabilityDTO, 0, len(caps))
	for _, c := range caps {
		vers := capVersionsMap[c.ID]
		var capDiskSize int64
		latestVer := 0
		for _, v := range vers {
			capDiskSize += verBytesMap[v.ID]
			if v.Version > latestVer {
				latestVer = v.Version
			}
		}
		totalDiskSize += capDiskSize

		isTopLevel := c.Kind == agentruntime.KindSkill
		parentKey := ""
		if c.CapabilityKey == "gitlab.snapshot" || c.CapabilityKey == "knowledge.search" {
			parentKey = "code_review"
		}

		var incComps []agentruntime.IncludedComponentDTO
		if c.CapabilityKey == "code_review" {
			incComps = []agentruntime.IncludedComponentDTO{
				{
					Key:         "gitlab.snapshot",
					Kind:        "plugin",
					Name:        "GitLab 快照插件",
					Description: "GitLab 代码与 MR 快照提取插件，负责 diff 与 commit 历史切片抽取",
					Tools:       []string{"gitlab.snapshot", "repository.read"},
				},
				{
					Key:         "knowledge.search",
					Kind:        "context_provider",
					Name:        "知识库检索提供方",
					Description: "系统领域知识与工程架构规范检索源，负责注入规范设计语料",
					Tools:       []string{"knowledge.query"},
				},
			}
		}

		result = append(result, CapabilityDTO{
			ID:                 c.ID,
			CapabilityKey:      c.CapabilityKey,
			Kind:               c.Kind,
			Status:             c.Status,
			Owner:              c.Owner,
			Sensitivity:        c.Sensitivity,
			Description:        c.Description,
			DiskSizeBytes:      capDiskSize,
			DiskSizeFormatted:  agentruntime.FormatBytes(capDiskSize),
			BindingsCount:      capBindingsMap[c.CapabilityKey],
			IsArchived:         c.Status == "archived",
			IsTopLevelSkill:    isTopLevel,
			ParentSkillKey:     parentKey,
			IncludedComponents: incComps,
			LatestVersion:      latestVer,
			Versions:           vers,
			CreatedAt:          c.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:          c.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"capabilities":              result,
		"total":                     len(result),
		"total_disk_size_bytes":     totalDiskSize,
		"total_disk_size_formatted": agentruntime.FormatBytes(totalDiskSize),
	})
}

// handleGetAgentCapabilityDetail handles GET /api/agent-runtime/capabilities/{id}
func (s *Server) handleGetAgentCapabilityDetail(w http.ResponseWriter, r *http.Request) {
	if s.capabilityRegistry == nil || db.DB == nil {
		http.Error(w, "Capability registry not initialized", http.StatusInternalServerError)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "capability key or ID is required", http.StatusBadRequest)
		return
	}

	detail, err := s.capabilityRegistry.GetCapabilityDetail(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(detail)
}

// handleUpdateAgentCapabilityStatus handles PATCH /api/agent-runtime/capabilities/{id}/status
func (s *Server) handleUpdateAgentCapabilityStatus(w http.ResponseWriter, r *http.Request) {
	if s.capabilityRegistry == nil || db.DB == nil {
		http.Error(w, "Capability registry not initialized", http.StatusInternalServerError)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "capability key or ID is required", http.StatusBadRequest)
		return
	}

	var req struct {
		Status string `json:"status"` // active, disabled
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Status != "active" && req.Status != "disabled" {
		http.Error(w, "status must be 'active' or 'disabled'", http.StatusBadRequest)
		return
	}

	if err := s.capabilityRegistry.UpdateCapabilityStatus(r.Context(), id, req.Status); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":     id,
		"status": req.Status,
	})
}

// handleUninstallAgentCapability handles DELETE /api/agent-runtime/capabilities/{id}
func (s *Server) handleUninstallAgentCapability(w http.ResponseWriter, r *http.Request) {
	if s.capabilityRegistry == nil || db.DB == nil {
		http.Error(w, "Capability registry not initialized", http.StatusInternalServerError)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "capability key or ID is required", http.StatusBadRequest)
		return
	}

	mode := strings.TrimSpace(r.URL.Query().Get("mode"))
	if mode == "" && r.Body != nil {
		var req struct {
			Mode string `json:"mode"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		mode = strings.TrimSpace(req.Mode)
	}

	if err := s.capabilityRegistry.UninstallCapability(r.Context(), id, mode); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	status := "uninstalled"
	msg := "技能已成功卸载并清理磁盘占用，已保留安装记录供回溯与恢复"
	if mode == "cold_archive" {
		status = "archived"
		msg = "技能已成功冷归档，已保留运行审计凭据与可重放能力"
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  status,
		"message": msg,
		"id":      id,
	})
}

// handleReinstallAgentCapability handles POST /api/agent-runtime/capabilities/{id}/reinstall
func (s *Server) handleReinstallAgentCapability(w http.ResponseWriter, r *http.Request) {
	if s.capabilityRegistry == nil || db.DB == nil {
		http.Error(w, "Capability registry not initialized", http.StatusInternalServerError)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "capability key or ID is required", http.StatusBadRequest)
		return
	}

	if err := s.capabilityRegistry.ReinstallCapability(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "active",
		"message": "技能已成功从历史安装记录重新安装并恢复",
		"id":      id,
	})
}

// handleRemoteInstallAgentCapability handles POST /api/agent-runtime/capabilities/remote-install
func (s *Server) handleRemoteInstallAgentCapability(w http.ResponseWriter, r *http.Request) {
	if s.capabilityRegistry == nil || db.DB == nil {
		http.Error(w, "Capability registry not initialized", http.StatusInternalServerError)
		return
	}

	var req struct {
		URL          string `json:"url"`
		AutoActivate bool   `json:"auto_activate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		http.Error(w, "only http or https protocols are allowed", http.StatusBadRequest)
		return
	}

	// Fetch remote manifest safely with timeout and size cap
	client := &http.Client{Timeout: 15 * time.Second}
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, req.URL, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid URL: %v", err), http.StatusBadRequest)
		return
	}
	httpReq.Header.Set("User-Agent", "WellAmbient-AgentRuntime/1.0")

	resp, err := client.Do(httpReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to fetch remote capability manifest: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		http.Error(w, fmt.Sprintf("remote server returned HTTP %d", resp.StatusCode), http.StatusBadGateway)
		return
	}

	bodyData, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2MB limit
	if err != nil {
		http.Error(w, "failed to read remote content", http.StatusInternalServerError)
		return
	}

	manifest, err := agentruntime.ParseManifest(bodyData)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid remote capability manifest: %v", err), http.StatusBadRequest)
		return
	}

	ver, err := s.capabilityRegistry.Register(r.Context(), manifest)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to register capability: %v", err), http.StatusBadRequest)
		return
	}

	if req.AutoActivate {
		scopeType := manifest.Scope.Type
		if scopeType == "" {
			scopeType = "global"
		}
		_ = s.capabilityRegistry.ActivateVersion(r.Context(), manifest.ID, manifest.Version, scopeType, manifest.Scope.ID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":         "installed",
		"capability_key": manifest.ID,
		"version":        ver.Version,
		"digest":         ver.ContentDigest,
		"manifest":       manifest,
	})
}

// handleUpgradeAgentCapability handles POST /api/agent-runtime/capabilities/{id}/upgrade
func (s *Server) handleUpgradeAgentCapability(w http.ResponseWriter, r *http.Request) {
	if s.capabilityRegistry == nil || db.DB == nil {
		http.Error(w, "Capability registry not initialized", http.StatusInternalServerError)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "capability key or ID is required", http.StatusBadRequest)
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Could be either JSON payload with "url" or direct manifest YAML/JSON
	var urlPayload struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(data, &urlPayload); err == nil && urlPayload.URL != "" {
		client := &http.Client{Timeout: 15 * time.Second}
		httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, urlPayload.URL, nil)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid upgrade URL: %v", err), http.StatusBadRequest)
			return
		}
		resp, err := client.Do(httpReq)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to fetch remote upgrade manifest: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		data, err = io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		if err != nil {
			http.Error(w, "failed to read remote content", http.StatusInternalServerError)
			return
		}
	}

	manifest, err := agentruntime.ParseManifest(data)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid upgrade manifest: %v", err), http.StatusBadRequest)
		return
	}

	ver, err := s.capabilityRegistry.UpgradeCapability(r.Context(), id, manifest)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to upgrade capability: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "upgraded",
		"id":      id,
		"version": ver.Version,
		"digest":  ver.ContentDigest,
	})
}

// handleRegisterAgentCapability handles POST /api/agent-runtime/capabilities
func (s *Server) handleRegisterAgentCapability(w http.ResponseWriter, r *http.Request) {
	if s.capabilityRegistry == nil || db.DB == nil {
		http.Error(w, "Capability registry not initialized", http.StatusInternalServerError)
		return
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	manifest, err := agentruntime.ParseManifest(data)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid manifest: %v", err), http.StatusBadRequest)
		return
	}

	ver, err := s.capabilityRegistry.Register(r.Context(), manifest)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to register capability: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(ver)
}

// handleActivateAgentCapabilityVersion handles POST /api/agent-runtime/capabilities/{id}/activate
func (s *Server) handleActivateAgentCapabilityVersion(w http.ResponseWriter, r *http.Request) {
	if s.capabilityRegistry == nil || db.DB == nil {
		http.Error(w, "Capability registry not initialized", http.StatusInternalServerError)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "capability key or ID is required", http.StatusBadRequest)
		return
	}

	var req struct {
		Version   int    `json:"version"`
		ScopeType string `json:"scope_type"`
		ScopeID   string `json:"scope_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.ScopeType == "" {
		req.ScopeType = "global"
	}

	if err := s.capabilityRegistry.ActivateVersion(r.Context(), id, req.Version, req.ScopeType, req.ScopeID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "activated",
		"id":      id,
		"version": req.Version,
		"scope":   req.ScopeType,
	})
}

// handlePreviewCapabilityResolve handles POST /api/agent-runtime/resolve/preview
func (s *Server) handlePreviewCapabilityResolve(w http.ResponseWriter, r *http.Request) {
	if s.capabilityRegistry == nil || db.DB == nil {
		http.Error(w, "Capability registry not initialized", http.StatusInternalServerError)
		return
	}
	var q agentruntime.ResolveQuery
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		http.Error(w, "invalid query", http.StatusBadRequest)
		return
	}
	if len(q.AllowedPermissions) == 0 {
		q.AllowedPermissions = []string{"*"}
	}

	plan, err := s.capabilityRegistry.Resolve(r.Context(), q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(plan)
}

// handleListAgentRuns handles GET /api/agent-runtime/runs
func (s *Server) handleListAgentRuns(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}
	limit := boundedQueryLimit(r, 100, 500)
	state := r.URL.Query().Get("state")
	kind := r.URL.Query().Get("kind")

	query := db.DB.WithContext(r.Context()).Order("created_at DESC, id DESC").Limit(limit)
	if state != "" {
		query = query.Where("state = ?", state)
	}
	if kind != "" {
		query = query.Where("agent_kind = ?", kind)
	}

	var runs []db.AgentRun
	if err := query.Find(&runs).Error; err != nil {
		http.Error(w, "failed to query runs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"runs":  runs,
		"total": len(runs),
	})
}

// handleGetAgentRunLockfile handles GET /api/agent-runtime/runs/{id}/lockfile
func (s *Server) handleGetAgentRunLockfile(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}

	var run db.AgentRun
	if err := db.DB.WithContext(r.Context()).First(&run, id).Error; err != nil {
		http.Error(w, "agent run not found", http.StatusNotFound)
		return
	}

	var bindings []db.RunCapabilityBinding
	_ = db.DB.WithContext(r.Context()).Where("run_id = ?", run.ID).Find(&bindings).Error

	caps := make([]agentruntime.BoundCapability, 0, len(bindings))
	for _, b := range bindings {
		caps = append(caps, agentruntime.BoundCapability{
			ID:              b.CapabilityKey,
			Version:         b.Version,
			Digest:          b.Digest,
			SelectionReason: b.SelectionReason,
			LoadLevels:      strings.Split(b.LoadLevel, ","),
		})
	}

	lockfile := agentruntime.RunLockfile{
		Schema:    "agent-run-lockfile/v1",
		RunID:     run.RunKey,
		AgentKind: run.AgentKind,
		Kernel: agentruntime.RefInfo{
			Version: run.KernelVersion,
			Digest:  "sha256:kernel-" + run.KernelVersion,
		},
		ModelProfile: agentruntime.RefInfo{
			ID:     run.ModelProfileRef,
			Digest: "sha256:model-" + run.ModelProfileRef,
		},
		Capabilities: caps,
		ContextPack: agentruntime.RefInfo{
			ID:     strconv.FormatUint(uint64(run.ContextPackID), 10),
			Digest: "sha256:context-pack",
		},
		PermissionGrant: agentruntime.PermissionGrantInfo{
			ID:      run.PermissionGrantID,
			Digest:  "sha256:grant-" + run.PermissionGrantID,
			Actions: []string{"source.read", "knowledge.read"},
		},
		Budgets: agentruntime.LockfileBudgets{
			InputTokens:     18000,
			OutputTokens:    10000,
			ToolCalls:       30,
			WallTimeSeconds: 480,
		},
		Resolver: agentruntime.RefInfo{
			Version: "capability-resolver-v1",
			Digest:  "sha256:resolver-v1",
		},
	}

	hash, _ := lockfile.ComputeHash()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-lockfile-hash", hash)
	_ = json.NewEncoder(w).Encode(lockfile)
}

// handleGetAgentRunTrace handles GET /api/agent-runtime/runs/{id}/trace
func (s *Server) handleGetAgentRunTrace(w http.ResponseWriter, r *http.Request) {
	if s.agentTrace == nil || db.DB == nil {
		http.Error(w, "Trace collector not initialized", http.StatusInternalServerError)
		return
	}
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}

	events, err := s.agentTrace.GetRunEvents(r.Context(), uint(id))
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get trace events: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"run_id": id,
		"events": events,
		"total":  len(events),
	})
}
