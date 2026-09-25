package agentruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
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
			Description:   m.Description,
			Sensitivity:   m.Sensitivity,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		if err := r.db.WithContext(ctx).Create(&capRecord).Error; err != nil {
			return nil, fmt.Errorf("failed to create capability: %w", err)
		}
	} else {
		capRecord = capRecords[0]
		if capRecord.Description == "" && m.Description != "" {
			capRecord.Description = m.Description
			_ = r.db.WithContext(ctx).Model(&capRecord).Update("description", m.Description).Error
		}
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

// IncludedComponentDTO describes a sub-plugin, tool, or context provider included inside a top-level skill.
type IncludedComponentDTO struct {
	Key         string   `json:"key"`
	Kind        string   `json:"kind"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tools       []string `json:"tools,omitempty"`
}

// CapabilityDetail contains detailed metadata and resource breakdown of a capability.
type CapabilityDetail struct {
	Capability         db.Capability             `json:"capability"`
	Versions           []CapabilityVersionDetail `json:"versions"`
	DiskSizeBytes      int64                     `json:"disk_size_bytes"`
	DiskSizeFormatted  string                    `json:"disk_size_formatted"`
	BindingsCount      int64                     `json:"bindings_count"`
	IsArchived         bool                      `json:"is_archived"`
	IsTopLevelSkill    bool                      `json:"is_top_level_skill"`
	ParentSkillKey     string                    `json:"parent_skill_key,omitempty"`
	IncludedComponents []IncludedComponentDTO    `json:"included_components,omitempty"`
	SkillMarkdown      string                    `json:"skill_markdown"`
}

// CapabilityVersionDetail includes resources and dependencies for a version.
type CapabilityVersionDetail struct {
	db.CapabilityVersion
	Resources         []db.CapabilityResource   `json:"resources"`
	Dependencies      []db.CapabilityDependency `json:"dependencies"`
	DiskSizeBytes     int64                     `json:"disk_size_bytes"`
	DiskSizeFormatted string                    `json:"disk_size_formatted"`
}

// FormatBytes formats byte counts into human-readable units.
func FormatBytes(b int64) string {
	if b < 0 {
		return "0 B"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := "KMGTPE"
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), units[exp])
}

// findCapability retrieves a capability by numeric ID or capability_key.
func (r *Registry) findCapability(ctx context.Context, idOrKey string) (*db.Capability, error) {
	var caps []db.Capability
	q := r.db.WithContext(ctx)
	if id, err := fmt.Sscanf(idOrKey, "%d", new(uint)); err == nil && id == 1 {
		q = q.Where("id = ? OR capability_key = ?", idOrKey, idOrKey)
	} else {
		q = q.Where("capability_key = ?", idOrKey)
	}
	if err := q.Limit(1).Find(&caps).Error; err != nil {
		return nil, err
	}
	if len(caps) == 0 {
		return nil, fmt.Errorf("capability %q not found", idOrKey)
	}
	return &caps[0], nil
}

// GetCapabilityDiskSize computes physical disk bytes occupied by a capability's manifests and resource contents.
func (r *Registry) GetCapabilityDiskSize(ctx context.Context, capabilityID uint) (int64, error) {
	var versions []db.CapabilityVersion
	if err := r.db.WithContext(ctx).Where("capability_id = ?", capabilityID).Find(&versions).Error; err != nil {
		return 0, err
	}
	var totalBytes int64
	var versionIDs []uint
	for _, v := range versions {
		totalBytes += int64(len(v.ManifestJSON))
		versionIDs = append(versionIDs, v.ID)
	}
	if len(versionIDs) > 0 {
		var resources []db.CapabilityResource
		if err := r.db.WithContext(ctx).Where("capability_version_id IN ?", versionIDs).Find(&resources).Error; err == nil {
			for _, res := range resources {
				totalBytes += int64(len(res.Content))
			}
		}
	}
	return totalBytes, nil
}

// GetCapabilityRunBindingsCount returns the number of AgentRuns that bound this capability.
func (r *Registry) GetCapabilityRunBindingsCount(ctx context.Context, capabilityKey string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&db.RunCapabilityBinding{}).
		Where("capability_key = ?", capabilityKey).
		Count(&count).Error
	return count, err
}

// GetCapabilityDetail returns full inspection data for a capability, including resources, disk size, and run bindings count.
func (r *Registry) GetCapabilityDetail(ctx context.Context, idOrKey string) (*CapabilityDetail, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	capRecord, err := r.findCapability(ctx, idOrKey)
	if err != nil {
		return nil, err
	}

	var versions []db.CapabilityVersion
	if err := r.db.WithContext(ctx).Where("capability_id = ?", capRecord.ID).Order("version DESC").Find(&versions).Error; err != nil {
		return nil, err
	}

	var verDetails []CapabilityVersionDetail
	var totalDiskSize int64

	for _, v := range versions {
		var resources []db.CapabilityResource
		_ = r.db.WithContext(ctx).Where("capability_version_id = ?", v.ID).Find(&resources).Error

		var deps []db.CapabilityDependency
		_ = r.db.WithContext(ctx).Where("capability_version_id = ?", v.ID).Find(&deps).Error

		var verBytes int64 = int64(len(v.ManifestJSON))
		for _, res := range resources {
			verBytes += int64(len(res.Content))
		}
		totalDiskSize += verBytes

		verDetails = append(verDetails, CapabilityVersionDetail{
			CapabilityVersion: v,
			Resources:         resources,
			Dependencies:      deps,
			DiskSizeBytes:     verBytes,
			DiskSizeFormatted: FormatBytes(verBytes),
		})
	}

	bindingsCount, _ := r.GetCapabilityRunBindingsCount(ctx, capRecord.CapabilityKey)
	isArchived := capRecord.Status == "archived"
	isTopLevel := capRecord.Kind == KindSkill
	parentKey := ""
	if capRecord.CapabilityKey == "gitlab.snapshot" || capRecord.CapabilityKey == "knowledge.search" {
		parentKey = "code_review"
	}

	// Build included components and skill markdown
	var includedComponents []IncludedComponentDTO
	skillMarkdown := ""

	if len(verDetails) > 0 {
		latestVer := verDetails[0]
		var m CapabilityManifest
		if err := json.Unmarshal([]byte(latestVer.ManifestJSON), &m); err == nil {
			// Convert resources for generator
			var resDefs []ResourceDef
			for _, r := range latestVer.Resources {
				resDefs = append(resDefs, ResourceDef{
					Key:           r.ResourceKey,
					LoadLevel:     r.LoadLevel,
					ContentKind:   r.ContentKind,
					Content:       r.Content,
					TokenEstimate: r.TokenEstimate,
				})
			}

			// Try reading local SKILL.md if present
			if capRecord.CapabilityKey == "code_review" || capRecord.CapabilityKey == "merge-review" {
				if localData, err := os.ReadFile(".agents/skills/merge-review/SKILL.md"); err == nil && len(localData) > 0 {
					skillMarkdown = string(localData)
				}
			}
			if skillMarkdown == "" {
				skillMarkdown = m.GenerateSkillMarkdown(resDefs)
			}

			// Build included components for top level skill
			if len(m.Requires) > 0 {
				for _, req := range m.Requires {
					var childCap db.Capability
					desc := ""
					if err := r.db.WithContext(ctx).Where("capability_key = ?", req.ID).Limit(1).Find(&childCap).Error; err == nil && childCap.ID > 0 {
						desc = childCap.Description
					}
					if desc == "" {
						if req.ID == "gitlab.snapshot" {
							desc = "GitLab 代码与 MR 快照提取插件，负责 diff 与 commit 历史切片抽取"
						} else if req.ID == "knowledge.search" {
							desc = "系统领域知识与工程架构规范检索源，负责注入规范设计语料"
						}
					}
					includedComponents = append(includedComponents, IncludedComponentDTO{
						Key:         req.ID,
						Kind:        req.Kind,
						Name:        req.ID,
						Description: desc,
					})
				}
			}
		}
	}

	// Fallback for code_review if manifest didn't parse requires
	if capRecord.CapabilityKey == "code_review" && len(includedComponents) == 0 {
		includedComponents = []IncludedComponentDTO{
			{
				Key:         "gitlab.snapshot",
				Kind:        "plugin",
				Name:        "GitLab 快照插件",
				Description: "GitLab 代码仓库与 MR 快照提取插件，由 code_review 技能调用，负责 diff 与 commit 历史切片抽取",
				Tools:       []string{"gitlab.snapshot", "repository.read"},
			},
			{
				Key:         "knowledge.search",
				Kind:        "context_provider",
				Name:        "知识库检索提供方",
				Description: "系统领域知识与工程架构规范检索源，由 code_review 技能调用，负责注入规范设计语料",
				Tools:       []string{"knowledge.query"},
			},
		}
	}

	return &CapabilityDetail{
		Capability:         *capRecord,
		Versions:           verDetails,
		DiskSizeBytes:      totalDiskSize,
		DiskSizeFormatted:  FormatBytes(totalDiskSize),
		BindingsCount:      bindingsCount,
		IsArchived:         isArchived,
		IsTopLevelSkill:    isTopLevel,
		ParentSkillKey:     parentKey,
		IncludedComponents: includedComponents,
		SkillMarkdown:      skillMarkdown,
	}, nil
}

// UpdateCapabilityStatus toggles or updates capability status (e.g. active, disabled).
func (r *Registry) UpdateCapabilityStatus(ctx context.Context, idOrKey string, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	capRecord, err := r.findCapability(ctx, idOrKey)
	if err != nil {
		return err
	}

	now := time.Now()
	return r.db.WithContext(ctx).Model(capRecord).Updates(map[string]any{
		"status":     status,
		"updated_at": now,
	}).Error
}

// UninstallCapability uninstalls or cold-archives a capability, automatically manages resource disk contents,
// and preserves the installation record for auditing and reinstallation.
// mode can be "cold_archive" (retains resources for historical run replay and audit),
// "purge" (cleans up physical disk contents), or "" (auto: cold_archive if bindings exist, else purge).
func (r *Registry) UninstallCapability(ctx context.Context, idOrKey string, mode string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	capRecord, err := r.findCapability(ctx, idOrKey)
	if err != nil {
		return err
	}

	// Determine effective mode
	if mode == "" {
		bindingsCount, _ := r.GetCapabilityRunBindingsCount(ctx, capRecord.CapabilityKey)
		if bindingsCount > 0 {
			mode = "cold_archive"
		} else {
			mode = "purge"
		}
	}

	var versions []db.CapabilityVersion
	if err := r.db.WithContext(ctx).Where("capability_id = ?", capRecord.ID).Find(&versions).Error; err != nil {
		return err
	}
	var versionIDs []uint
	for _, v := range versions {
		versionIDs = append(versionIDs, v.ID)
	}

	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if mode == "cold_archive" {
			// Mark capability as archived
			if err := tx.Model(capRecord).Updates(map[string]any{
				"status":     "archived",
				"updated_at": now,
			}).Error; err != nil {
				return err
			}

			// Retire/archive versions to withdraw from active resolver, but PRESERVE contents for replay
			if len(versionIDs) > 0 {
				if err := tx.Model(&db.CapabilityVersion{}).
					Where("capability_id = ?", capRecord.ID).
					Updates(map[string]any{
						"status":     "archived",
						"retired_at": now,
						"updated_at": now,
					}).Error; err != nil {
					return err
				}
			}
			return nil
		}

		// Purge mode: mark uninstalled and wipe resource content to free 100% space
		if err := tx.Model(capRecord).Updates(map[string]any{
			"status":     "uninstalled",
			"updated_at": now,
		}).Error; err != nil {
			return err
		}

		if len(versionIDs) > 0 {
			if err := tx.Model(&db.CapabilityVersion{}).
				Where("capability_id = ?", capRecord.ID).
				Updates(map[string]any{
					"status":     "retired",
					"retired_at": now,
					"updated_at": now,
				}).Error; err != nil {
					return err
			}

			// Clean up physical disk resource contents to free space
			if err := tx.Model(&db.CapabilityResource{}).
				Where("capability_version_id IN ?", versionIDs).
				Update("content", "").Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ReinstallCapability restores an uninstalled capability back to active status.
func (r *Registry) ReinstallCapability(ctx context.Context, idOrKey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	capRecord, err := r.findCapability(ctx, idOrKey)
	if err != nil {
		return err
	}

	// Find the latest version
	var latestVers []db.CapabilityVersion
	if err := r.db.WithContext(ctx).
		Where("capability_id = ?", capRecord.ID).
		Order("version DESC").
		Limit(1).
		Find(&latestVers).Error; err != nil || len(latestVers) == 0 {
		return fmt.Errorf("no recorded version found to reinstall")
	}
	latestVer := latestVers[0]

	// Restore resources from manifest JSON if content was cleared
	var m CapabilityManifest
	if err := json.Unmarshal([]byte(latestVer.ManifestJSON), &m); err == nil && len(m.Resources) > 0 {
		for _, res := range m.Resources {
			_ = r.db.WithContext(ctx).Model(&db.CapabilityResource{}).
				Where("capability_version_id = ? AND resource_key = ?", latestVer.ID, res.Key).
				Update("content", res.Content).Error
		}
	}

	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(capRecord).Updates(map[string]any{
			"status":     "active",
			"updated_at": now,
		}).Error; err != nil {
			return err
		}

		return tx.Model(&latestVer).Updates(map[string]any{
			"status":       "active",
			"activated_at": now,
			"updated_at":   now,
		}).Error
	})
}

// UpgradeCapability upgrades an existing capability with a new manifest version and activates it.
func (r *Registry) UpgradeCapability(ctx context.Context, idOrKey string, newManifest *CapabilityManifest) (*db.CapabilityVersion, error) {
	capRecord, err := r.findCapability(ctx, idOrKey)
	if err != nil {
		return nil, err
	}

	// Ensure manifest capability ID matches
	if newManifest.ID == "" {
		newManifest.ID = capRecord.CapabilityKey
	}
	if newManifest.ID != capRecord.CapabilityKey {
		return nil, fmt.Errorf("manifest ID %q does not match target capability %q", newManifest.ID, capRecord.CapabilityKey)
	}

	// Verify new version is higher than existing versions
	var maxVer int
	row := r.db.WithContext(ctx).Model(&db.CapabilityVersion{}).
		Where("capability_id = ?", capRecord.ID).
		Select("COALESCE(MAX(version), 0)").Row()
	_ = row.Scan(&maxVer)

	if newManifest.Version <= maxVer {
		newManifest.Version = maxVer + 1
	}

	ver, err := r.Register(ctx, newManifest)
	if err != nil {
		return nil, fmt.Errorf("upgrade registration failed: %w", err)
	}

	// Activate new version
	scopeType := newManifest.Scope.Type
	if scopeType == "" {
		scopeType = "global"
	}
	if err := r.ActivateVersion(ctx, capRecord.CapabilityKey, newManifest.Version, scopeType, newManifest.Scope.ID); err != nil {
		return nil, fmt.Errorf("failed to activate upgraded version: %w", err)
	}

	// Ensure capability itself is active
	_ = r.UpdateCapabilityStatus(ctx, capRecord.CapabilityKey, "active")

	return ver, nil
}

