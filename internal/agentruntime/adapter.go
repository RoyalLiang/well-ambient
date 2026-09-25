package agentruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
	"well-ambient/internal/db"
)

// LegacySkillAdapter provides bi-directional compatibility between SolutionPromptTemplate
// (Phase 0 prompt-centric model) and CapabilityVersion (universal runtime model).
type LegacySkillAdapter struct {
	db       *gorm.DB
	registry *Registry
}

// NewLegacySkillAdapter initializes the compatibility adapter.
func NewLegacySkillAdapter(database *gorm.DB, registry *Registry) *LegacySkillAdapter {
	return &LegacySkillAdapter{
		db:       database,
		registry: registry,
	}
}

// EnsureDefaultCapabilities boots the baseline capabilities if they do not yet exist:
// 1. merge-review (Skill)
// 2. gitlab.snapshot (Plugin)
// 3. knowledge.search (ContextProvider)
func (a *LegacySkillAdapter) EnsureDefaultCapabilities(ctx context.Context) error {
	// 1. gitlab.snapshot Plugin
	snapshotManifest := &CapabilityManifest{
		Schema:      "capability-manifest/v1",
		ID:          "gitlab.snapshot",
		Kind:        KindPlugin,
		Version:     1,
		Owner:       "platform-team",
		Description: "GitLab 代码仓库与 MR 快照提取插件，由 code_review 技能调用，负责 diff 与 commit 历史切片抽取",
		Provides: []string{
			"gitlab.diff",
			"gitlab.commit_log",
			"gitlab.mr_meta",
		},
		Permissions: []string{"source.read"},
		Tools:       []string{"gitlab.snapshot", "repository.read"},
		Budgets: BudgetDef{
			InstructionTokens: 500,
			EvidenceTokens:    8000,
			ToolCalls:         10,
			WallTimeSeconds:   120,
		},
	}
	snapshotManifest.Scope.Type = "global"
	snapshotManifest.Triggers.Intents = []string{"code-review", "merge-review"}

	var capSnaps []db.Capability
	if err := a.db.WithContext(ctx).Where("capability_key = ?", "gitlab.snapshot").Limit(1).Find(&capSnaps).Error; err == nil {
		if len(capSnaps) == 0 {
			if _, err := a.registry.Register(ctx, snapshotManifest); err != nil {
				return fmt.Errorf("failed to register gitlab.snapshot: %w", err)
			}
			if err := a.registry.ActivateVersion(ctx, "gitlab.snapshot", 1, "global", ""); err != nil {
				return fmt.Errorf("failed to activate gitlab.snapshot: %w", err)
			}
		} else if capSnaps[0].Description == "" {
			_ = a.db.WithContext(ctx).Model(&capSnaps[0]).Update("description", snapshotManifest.Description).Error
		}
	}

	// 2. knowledge.search ContextProvider
	knowledgeManifest := &CapabilityManifest{
		Schema:      "capability-manifest/v1",
		ID:          "knowledge.search",
		Kind:        KindContextProvider,
		Version:     1,
		Owner:       "architecture-team",
		Description: "系统领域知识与工程架构规范检索源，由 code_review 技能调用，负责注入规范设计语料",
		Provides: []string{
			"system.knowledge",
			"domain.contracts",
		},
		Permissions: []string{"knowledge.read"},
		Tools:       []string{"knowledge.query"},
		Budgets: BudgetDef{
			InstructionTokens: 500,
			EvidenceTokens:    4000,
			ToolCalls:         5,
			WallTimeSeconds:   60,
		},
	}
	knowledgeManifest.Scope.Type = "global"
	knowledgeManifest.Triggers.Intents = []string{"code-review", "merge-review"}

	var capKnowledges []db.Capability
	if err := a.db.WithContext(ctx).Where("capability_key = ?", "knowledge.search").Limit(1).Find(&capKnowledges).Error; err == nil {
		if len(capKnowledges) == 0 {
			if _, err := a.registry.Register(ctx, knowledgeManifest); err != nil {
				return fmt.Errorf("failed to register knowledge.search: %w", err)
			}
			if err := a.registry.ActivateVersion(ctx, "knowledge.search", 1, "global", ""); err != nil {
				return fmt.Errorf("failed to activate knowledge.search: %w", err)
			}
		} else if capKnowledges[0].Description == "" {
			_ = a.db.WithContext(ctx).Model(&capKnowledges[0]).Update("description", knowledgeManifest.Description).Error
		}
	}

	// 3. code_review (or merge-review) Skill
	// Check if active SolutionPromptTemplate exists
	var activeLegacies []db.SolutionPromptTemplate
	err := a.db.WithContext(ctx).
		Where("purpose = ? AND status = ?", "code_review", "active").
		Order("version DESC").
		Limit(1).
		Find(&activeLegacies).Error

	instructions := "Standard code review rules: verify logic, safety, contracts, performance and evidence completeness."
	version := 1
	if err == nil && len(activeLegacies) > 0 {
		instructions = activeLegacies[0].SystemPrompt
		version = activeLegacies[0].Version
	}

	var capReviews []db.Capability
	if err := a.db.WithContext(ctx).Where("capability_key = ?", "code_review").Limit(1).Find(&capReviews).Error; err == nil {
		reviewManifest := &CapabilityManifest{
			Schema:      "capability-manifest/v1",
			ID:          "code_review",
			Kind:        KindSkill,
			Version:     version,
			Owner:       "engineering-governance",
			Description: "代码评审顶级业务技能，统一调度 gitlab.snapshot 快照插件与 knowledge.search 领域知识库，输出结构化报告",
			Sensitivity: "internal",
			Provides: []string{
				"diff-review",
				"evidence-validation",
				"structured-review-report",
			},
			Requires: []DependencyRef{
				{ID: "gitlab.snapshot", Kind: KindPlugin, Version: ">=1"},
				{ID: "knowledge.search", Kind: KindContextProvider, Version: ">=1"},
			},
			Permissions: []string{"source.read", "knowledge.read"},
			Resources: []ResourceDef{
				{
					Key:           "review-instructions",
					LoadLevel:     LoadLevelL1,
					ContentKind:   "text",
					Content:       instructions,
					TokenEstimate: EstimateTokens(instructions),
				},
				{
					Key:           "output-contract",
					LoadLevel:     LoadLevelL1,
					ContentKind:   "json",
					Content:       `{"type":"object","required":["summary","verdict","findings"]}`,
					TokenEstimate: 100,
				},
			},
			Budgets: BudgetDef{
				InstructionTokens: 2000,
				EvidenceTokens:    12000,
				ToolCalls:         20,
				WallTimeSeconds:   300,
			},
		}
		reviewManifest.Scope.Type = "global"
		reviewManifest.Triggers.Intents = []string{"code-review", "merge-review"}

		if len(capReviews) == 0 {
			if _, err := a.registry.Register(ctx, reviewManifest); err != nil {
				return fmt.Errorf("failed to register code_review skill: %w", err)
			}
			if err := a.registry.ActivateVersion(ctx, "code_review", version, "global", ""); err != nil {
				return fmt.Errorf("failed to activate code_review skill: %w", err)
			}
		} else if capReviews[0].Description == "" {
			_ = a.db.WithContext(ctx).Model(&capReviews[0]).Update("description", reviewManifest.Description).Error
		}
	}

	return nil
}

// MapCodeReviewRunToAgentRun adapts a legacy CodeReviewRun to an AgentRun and bindings.
func (a *LegacySkillAdapter) MapCodeReviewRunToAgentRun(run *db.CodeReviewRun) (*db.AgentRun, []db.RunCapabilityBinding) {
	runKey := run.Key
	if runKey == "" {
		runKey = fmt.Sprintf("review-%d", run.ID)
	}

	kernelVer := "v1"
	profileRef := run.Model
	if profileRef == "" {
		profileRef = "default-model-profile"
	}

	state := run.Status
	if state == "" {
		state = "completed"
	}

	metadata := map[string]any{
		"legacy_review_run_id": run.ID,
		"project_id":           run.ProjectID,
		"author":               run.Author,
		"repo":                 run.Repo,
		"kind":                 run.Kind,
		"ref":                  run.Ref,
		"head_sha":             run.HeadSHA,
		"base_sha":             run.BaseSHA,
		"title":                run.Title,
	}
	metaBytes, _ := json.Marshal(metadata)

	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", runKey, run.ID, run.SkillHash)))
	lockfileHash := "sha256:" + hex.EncodeToString(h[:])

	agentRun := &db.AgentRun{
		RunKey:          runKey,
		AgentKind:       "code_review",
		KernelVersion:   kernelVer,
		ModelProfileRef: profileRef,
		State:           state,
		LockfileHash:    lockfileHash,
		MetadataJSON:    string(metaBytes),
		CreatedAt:       run.CreatedAt,
		UpdatedAt:       run.UpdatedAt,
		CompletedAt:     &run.UpdatedAt,
	}

	bindings := []db.RunCapabilityBinding{
		{
			CapabilityKey:   "code_review",
			Version:         run.SkillVersion,
			Digest:          run.SkillHash,
			SelectionReason: "legacy review run binding",
			LoadLevel:       LoadLevelL1,
			PermissionGrant: "source.read,knowledge.read",
			LoadedAt:        run.CreatedAt,
		},
		{
			CapabilityKey:   "gitlab.snapshot",
			Version:         1,
			Digest:          "sha256:legacy-gitlab-snapshot",
			SelectionReason: "required by code_review",
			LoadLevel:       LoadLevelL2,
			PermissionGrant: "source.read",
			LoadedAt:        run.CreatedAt,
		},
	}

	return agentRun, bindings
}
