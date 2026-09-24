package agentruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"gorm.io/gorm"
	"well-ambient/internal/db"
)

// ResolveQuery parameters for capability planning.
type ResolveQuery struct {
	Intent             string   `json:"intent"`
	Conditions         []string `json:"conditions"`
	ScopeType          string   `json:"scope_type"`
	ScopeID            string   `json:"scope_id"`
	AllowedPermissions []string `json:"allowed_permissions"`
	TotalTokenBudget   int      `json:"total_token_budget"`
}

// CapabilityPlan represents the resolved, topologically ordered capabilities and budgets.
type CapabilityPlan struct {
	Capabilities         []BoundCapability `json:"capabilities"`
	GrantedPermissions   []string          `json:"granted_permissions"`
	Tools                []string          `json:"tools"`
	InstructionBudget    int               `json:"instruction_budget"`
	EvidenceBudget       int               `json:"evidence_budget"`
	RawBudget            int               `json:"raw_budget"`
	TotalEstimatedTokens int               `json:"total_estimated_tokens"`
}

// Registry manages registration, versioning, dependency resolution, and activation.
type Registry struct {
	db *gorm.DB
	mu sync.RWMutex
}

// NewRegistry creates a new capability registry.
func NewRegistry(database *gorm.DB) *Registry {
	return &Registry{db: database}
}

// Register adds a capability and its version to the registry.
func (r *Registry) Register(ctx context.Context, m *CapabilityManifest) (*db.CapabilityVersion, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	digest := m.ComputeDigest()
	m.Digest = digest

	manifestBytes, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	var capRecords []db.Capability
	err = r.db.WithContext(ctx).Where("capability_key = ?", m.ID).Limit(1).Find(&capRecords).Error
	if err != nil {
		return nil, err
	}
	var capRecord db.Capability
	if len(capRecords) == 0 {
		capRecord = db.Capability{
			CapabilityKey: m.ID,
			Kind:          m.Kind,
			Status:        "active",
			Owner:         m.Owner,
			Sensitivity:   m.Sensitivity,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		if err := r.db.WithContext(ctx).Create(&capRecord).Error; err != nil {
			return nil, fmt.Errorf("failed to create capability: %w", err)
		}
	} else {
		capRecord = capRecords[0]
	}

	scopeType := m.Scope.Type
	if scopeType == "" {
		scopeType = "global"
	}
	scopeID := m.Scope.ID

	// Check for duplicate version in this scope
	var existingVers []db.CapabilityVersion
	err = r.db.WithContext(ctx).
		Where("capability_id = ? AND version = ? AND scope_type = ? AND scope_id = ?",
			capRecord.ID, m.Version, scopeType, scopeID).
		Limit(1).
		Find(&existingVers).Error
	if err == nil && len(existingVers) > 0 {
		return nil, fmt.Errorf("capability version %s@%d already exists in scope %s:%s",
			m.ID, m.Version, scopeType, scopeID)
	}

	verRecord := db.CapabilityVersion{
		CapabilityID:     capRecord.ID,
		Version:          m.Version,
		ScopeType:        scopeType,
		ScopeID:          scopeID,
		ManifestJSON:     string(manifestBytes),
		ContentDigest:    digest,
		ValidationStatus: "passed", // Default to passed upon successful structural manifest validation
		Status:           "draft",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := r.db.WithContext(ctx).Create(&verRecord).Error; err != nil {
		return nil, fmt.Errorf("failed to create capability version: %w", err)
	}

	// Persist sliced resources
	for _, res := range m.Resources {
		contentHash := res.ContentRef
		if contentHash == "" && res.Content != "" {
			h := sha256.Sum256([]byte(res.Content))
			contentHash = "sha256:" + hex.EncodeToString(h[:])
		}
		resRecord := db.CapabilityResource{
			CapabilityVersionID: verRecord.ID,
			ResourceKey:         res.Key,
			ContentKind:         res.ContentKind,
			ContentHash:         contentHash,
			BlobRef:             res.ContentRef,
			TokenEstimate:       res.TokenEstimate,
			LoadLevel:           res.LoadLevel,
			Content:             res.Content,
			CreatedAt:           time.Now(),
		}
		_ = r.db.WithContext(ctx).Create(&resRecord).Error
	}

	// Persist dependencies
	for _, req := range m.Requires {
		depRecord := db.CapabilityDependency{
			CapabilityVersionID: verRecord.ID,
			DependencyKey:       req.ID,
			VersionConstraint:   req.Version,
			IsOptional:          false,
			CreatedAt:           time.Now(),
		}
		_ = r.db.WithContext(ctx).Create(&depRecord).Error
	}
	for _, opt := range m.Optional {
		depRecord := db.CapabilityDependency{
			CapabilityVersionID: verRecord.ID,
			DependencyKey:       opt.ID,
			VersionConstraint:   opt.Version,
			IsOptional:          true,
			CreatedAt:           time.Now(),
		}
		_ = r.db.WithContext(ctx).Create(&depRecord).Error
	}

	return &verRecord, nil
}

// ActivateVersion atomically activates a specific capability version and retires older ones.
func (r *Registry) ActivateVersion(ctx context.Context, capabilityKey string, version int, scopeType, scopeID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var capRecords []db.Capability
	if err := r.db.WithContext(ctx).Where("capability_key = ?", capabilityKey).Limit(1).Find(&capRecords).Error; err != nil || len(capRecords) == 0 {
		return fmt.Errorf("capability %s not found", capabilityKey)
	}
	capRecord := capRecords[0]

	var targetVers []db.CapabilityVersion
	if err := r.db.WithContext(ctx).
		Where("capability_id = ? AND version = ? AND scope_type = ? AND scope_id = ?",
			capRecord.ID, version, scopeType, scopeID).
		Limit(1).
		Find(&targetVers).Error; err != nil || len(targetVers) == 0 {
		return fmt.Errorf("capability version not found")
	}
	targetVer := targetVers[0]

	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Retire existing active versions in this scope
		if err := tx.Model(&db.CapabilityVersion{}).
			Where("capability_id = ? AND scope_type = ? AND scope_id = ? AND status = ?",
				capRecord.ID, scopeType, scopeID, "active").
			Updates(map[string]any{
				"status":     "retired",
				"retired_at": now,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		// Activate target version
		return tx.Model(&targetVer).Updates(map[string]any{
			"status":       "active",
			"activated_at": now,
			"updated_at":   now,
		}).Error
	})
}

// GetActive retrieves the active capability version for a given key, checking scope inheritance.
func (r *Registry) GetActive(ctx context.Context, capabilityKey string, scopeType, scopeID string) (*CapabilityManifest, *db.CapabilityVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var capRecords []db.Capability
	if err := r.db.WithContext(ctx).Where("capability_key = ?", capabilityKey).Limit(1).Find(&capRecords).Error; err != nil || len(capRecords) == 0 {
		return nil, nil, fmt.Errorf("capability %s not found", capabilityKey)
	}
	capRecord := capRecords[0]

	// Scope cascade: specific scope -> global
	var vers []db.CapabilityVersion
	err := r.db.WithContext(ctx).
		Where("capability_id = ? AND scope_type = ? AND scope_id = ? AND status = ?",
			capRecord.ID, scopeType, scopeID, "active").
		Order("version DESC").
		Limit(1).
		Find(&vers).Error

	if (err != nil || len(vers) == 0) && scopeType != "global" {
		err = r.db.WithContext(ctx).
			Where("capability_id = ? AND scope_type = ? AND status = ?",
				capRecord.ID, "global", "active").
			Order("version DESC").
			Limit(1).
			Find(&vers).Error
	}

	if err != nil || len(vers) == 0 {
		return nil, nil, fmt.Errorf("no active version for capability %s", capabilityKey)
	}
	verRecord := vers[0]

	var m CapabilityManifest
	if err := json.Unmarshal([]byte(verRecord.ManifestJSON), &m); err != nil {
		return nil, nil, fmt.Errorf("failed to parse stored manifest: %w", err)
	}
	return &m, &verRecord, nil
}

// Resolve resolves the complete capability execution plan including DAG dependencies, permissions, and budgets.
func (r *Registry) Resolve(ctx context.Context, query ResolveQuery) (*CapabilityPlan, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. Find all active capabilities that match the query intent
	var activeVersions []db.CapabilityVersion
	err := r.db.WithContext(ctx).
		Where("status = ?", "active").
		Order("version DESC").
		Find(&activeVersions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list active capability versions: %w", err)
	}

	var primaryManifests []*CapabilityManifest
	manifestMap := make(map[string]*CapabilityManifest)

	for _, ver := range activeVersions {
		var m CapabilityManifest
		if err := json.Unmarshal([]byte(ver.ManifestJSON), &m); err != nil {
			continue
		}
		if manifestMap[m.ID] == nil {
			manifestMap[m.ID] = &m
			if m.MatchIntent(query.Intent, query.Conditions) {
				primaryManifests = append(primaryManifests, &m)
			}
		}
	}

	if len(primaryManifests) == 0 {
		return nil, fmt.Errorf("no active capability matched intent %q and conditions %v", query.Intent, query.Conditions)
	}

	// 2. DAG Resolution with Cycle Detection & Optional Dependency Support
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	var resolved []*CapabilityManifest
	reasons := make(map[string]string)

	var resolveDAG func(m *CapabilityManifest, isOptional bool, reason string) error
	resolveDAG = func(m *CapabilityManifest, isOptional bool, reason string) error {
		if visiting[m.ID] {
			return fmt.Errorf("cyclic capability dependency detected involving %s", m.ID)
		}
		if visited[m.ID] {
			return nil
		}
		visiting[m.ID] = true

		for _, req := range m.Requires {
			dep := manifestMap[req.ID]
			if dep == nil {
				return fmt.Errorf("required dependency %q (version %s) not found or inactive for capability %s",
					req.ID, req.Version, m.ID)
			}
			if err := resolveDAG(dep, false, fmt.Sprintf("required by %s", m.ID)); err != nil {
				return err
			}
		}

		for _, opt := range m.Optional {
			dep := manifestMap[opt.ID]
			if dep != nil {
				_ = resolveDAG(dep, true, fmt.Sprintf("optional dependency of %s", m.ID))
			}
		}

		visiting[m.ID] = false
		visited[m.ID] = true
		resolved = append(resolved, m)
		if reasons[m.ID] == "" {
			reasons[m.ID] = reason
		}
		return nil
	}

	for _, m := range primaryManifests {
		reason := fmt.Sprintf("matched intent %s", query.Intent)
		if err := resolveDAG(m, false, reason); err != nil {
			return nil, err
		}
	}

	// 3. Permission & Budget Verification
	permSet := make(map[string]bool)
	toolSet := make(map[string]bool)
	var bound []BoundCapability
	instructionBudget := 0
	evidenceBudget := 0
	totalEstTokens := 0

	for _, m := range resolved {
		// Check permission grant
		if ok, missing := m.CheckPermissionGrant(query.AllowedPermissions); !ok {
			return nil, fmt.Errorf("capability %s requires unauthorized permissions: %v", m.ID, missing)
		}
		for _, p := range m.Permissions {
			permSet[p] = true
		}
		for _, t := range m.Tools {
			toolSet[t] = true
		}

		levels := []string{LoadLevelL0, LoadLevelL1}
		if len(m.Resources) > 2 {
			levels = append(levels, LoadLevelL2)
		}

		bound = append(bound, BoundCapability{
			ID:              m.ID,
			Kind:            m.Kind,
			Version:         m.Version,
			Digest:          m.Digest,
			SelectionReason: reasons[m.ID],
			LoadLevels:      levels,
		})

		instructionBudget += m.Budgets.InstructionTokens
		evidenceBudget += m.Budgets.EvidenceTokens
		for _, r := range m.Resources {
			totalEstTokens += r.TokenEstimate
		}
	}

	if instructionBudget == 0 {
		instructionBudget = 2000
	}
	if evidenceBudget == 0 {
		evidenceBudget = 10000
	}

	var allPerms []string
	for p := range permSet {
		allPerms = append(allPerms, p)
	}
	sort.Strings(allPerms)

	var allTools []string
	for t := range toolSet {
		allTools = append(allTools, t)
	}
	sort.Strings(allTools)

	return &CapabilityPlan{
		Capabilities:         bound,
		GrantedPermissions:   allPerms,
		Tools:                allTools,
		InstructionBudget:    instructionBudget,
		EvidenceBudget:       evidenceBudget,
		RawBudget:            0,
		TotalEstimatedTokens: totalEstTokens,
	}, nil
}
