package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

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
	var caps []db.Capability
	if err := db.DB.WithContext(r.Context()).Order("capability_key ASC").Find(&caps).Error; err != nil {
		http.Error(w, "Failed to query capabilities", http.StatusInternalServerError)
		return
	}

	var versions []db.CapabilityVersion
	_ = db.DB.WithContext(r.Context()).Order("version DESC").Find(&versions).Error

	capVersionsMap := make(map[uint][]db.CapabilityVersion)
	for _, v := range versions {
		capVersionsMap[v.CapabilityID] = append(capVersionsMap[v.CapabilityID], v)
	}

	type CapabilityDTO struct {
		ID            uint                   `json:"id"`
		CapabilityKey string                 `json:"capability_key"`
		Kind          string                 `json:"kind"`
		Status        string                 `json:"status"`
		Owner         string                 `json:"owner"`
		Sensitivity   string                 `json:"sensitivity"`
		Versions      []db.CapabilityVersion `json:"versions"`
	}

	result := make([]CapabilityDTO, 0, len(caps))
	for _, c := range caps {
		result = append(result, CapabilityDTO{
			ID:            c.ID,
			CapabilityKey: c.CapabilityKey,
			Kind:          c.Kind,
			Status:        c.Status,
			Owner:         c.Owner,
			Sensitivity:   c.Sensitivity,
			Versions:      capVersionsMap[c.ID],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"capabilities": result,
		"total":        len(result),
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
