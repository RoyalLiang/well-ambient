package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
	"well-ambient/internal/server/authz"
)

type authorizationPolicyRequest struct {
	ID            uint   `json:"id"`
	Effect        string `json:"effect"`
	SubjectType   string `json:"subject_type"`
	SubjectID     string `json:"subject_id"`
	Action        string `json:"action"`
	ResourceType  string `json:"resource_type"`
	ResourceID    string `json:"resource_id"`
	Scope         string `json:"scope"`
	ScopeID       string `json:"scope_id"`
	ConditionJSON string `json:"condition_json"`
	Priority      int    `json:"priority"`
	Enabled       *bool  `json:"enabled"`
	Reason        string `json:"reason"`
}

type authorizationExplainRequest struct {
	Username     string `json:"username"`
	Action       string `json:"action"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	Scope        string `json:"scope"`
	ScopeID      string `json:"scope_id"`
}

func (s *Server) handleListAuthorizationPolicies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	var policies []userdb.AuthorizationPolicy
	if err := db.DB.Order("enabled desc, priority desc, id desc").Find(&policies).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to query authorization policies: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"policies": policies,
	})
}

func (s *Server) handleSaveAuthorizationPolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	var req authorizationPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		return
	}

	policy, err := normalizeAuthorizationPolicyRequest(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.ID > 0 {
		var existing userdb.AuthorizationPolicy
		if err := db.DB.First(&existing, req.ID).Error; err != nil {
			http.Error(w, "Authorization policy not found", http.StatusNotFound)
			return
		}
		policy.ID = existing.ID
		policy.CreatedAt = existing.CreatedAt
		if err := db.DB.Save(&policy).Error; err != nil {
			http.Error(w, fmt.Sprintf("Failed to update authorization policy: %v", err), http.StatusInternalServerError)
			return
		}
	} else if err := db.DB.Create(&policy).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to create authorization policy: %v", err), http.StatusInternalServerError)
		return
	}

	actor := r.Header.Get("x-authenticated-user-id")
	_ = userdb.RecordAuditLog(db.DB, actor, "authorization_policy_save", "authorization_policy", strconv.FormatUint(uint64(policy.ID), 10), policy.Reason, r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"policy":  policy,
	})
}

func (s *Server) handleExplainAuthorization(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	var req authorizationExplainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		username = r.Header.Get("x-authenticated-user-id")
	}
	if strings.TrimSpace(req.Action) == "" {
		http.Error(w, "action is required", http.StatusBadRequest)
		return
	}

	decision := authz.NewEvaluator(db.DB).Authorize(
		r.Context(),
		authz.Subject{Username: username},
		strings.TrimSpace(req.Action),
		authz.Resource{
			Type:        req.ResourceType,
			ID:          strings.TrimSpace(req.ResourceID),
			Scope:       strings.TrimSpace(req.Scope),
			ScopeID:     strings.TrimSpace(req.ScopeID),
			RequestPath: r.URL.Path,
			IPAddress:   r.RemoteAddr,
		},
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"allowed":            decision.Allowed,
		"reason":             decision.Reason,
		"missing_permission": decision.MissingPermission,
		"scope":              decision.Scope,
		"risk_level":         decision.RiskLevel,
		"matched_policy":     decision.MatchedPolicy,
	})
}

func (s *Server) handleListAuthorizationAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	limit := 80
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 && parsed <= 300 {
			limit = parsed
		}
	}

	var logs []userdb.AuthorizationAuditLog
	if err := db.DB.Order("created_at desc, id desc").Limit(limit).Find(&logs).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to query authorization audit logs: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs": logs,
	})
}

func normalizeAuthorizationPolicyRequest(req authorizationPolicyRequest) (userdb.AuthorizationPolicy, error) {
	effect := strings.ToLower(strings.TrimSpace(req.Effect))
	if effect == "" {
		effect = authz.EffectDeny
	}
	if effect != authz.EffectAllow && effect != authz.EffectDeny {
		return userdb.AuthorizationPolicy{}, fmt.Errorf("effect must be allow or deny")
	}

	action := strings.TrimSpace(req.Action)
	if action == "" {
		return userdb.AuthorizationPolicy{}, fmt.Errorf("action is required")
	}

	subjectType := strings.ToLower(strings.TrimSpace(req.SubjectType))
	if subjectType == "" {
		subjectType = authz.SubjectAny
	}
	if subjectType != authz.SubjectAny && subjectType != authz.SubjectUser && subjectType != authz.SubjectGroup {
		return userdb.AuthorizationPolicy{}, fmt.Errorf("subject_type must be any, user, or group")
	}

	scope := strings.ToLower(strings.TrimSpace(req.Scope))
	if scope == "" {
		scope = "global"
	}
	conditionJSON := strings.TrimSpace(req.ConditionJSON)
	if conditionJSON != "" {
		var condition map[string]interface{}
		if err := json.Unmarshal([]byte(conditionJSON), &condition); err != nil {
			return userdb.AuthorizationPolicy{}, fmt.Errorf("condition_json must be valid JSON")
		}
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	return userdb.AuthorizationPolicy{
		Effect:        effect,
		SubjectType:   subjectType,
		SubjectID:     strings.TrimSpace(req.SubjectID),
		Action:        action,
		ResourceType:  normalizePolicyResourceType(req.ResourceType, action),
		ResourceID:    strings.TrimSpace(req.ResourceID),
		Scope:         scope,
		ScopeID:       strings.TrimSpace(req.ScopeID),
		ConditionJSON: conditionJSON,
		Priority:      req.Priority,
		Enabled:       enabled,
		Reason:        strings.TrimSpace(req.Reason),
	}, nil
}

func normalizePolicyResourceType(resourceType, action string) string {
	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	if resourceType != "" {
		return resourceType
	}
	return resourceTypeForPermission(action)
}
