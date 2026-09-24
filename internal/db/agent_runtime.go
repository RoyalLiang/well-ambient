package db

import (
	"time"
)

// Capability represents a stable identity of an agent capability (skill, mcp, plugin, policy, etc.).
type Capability struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	CapabilityKey string    `gorm:"uniqueIndex;size:128;not null;column:capability_key" json:"capability_key"` // e.g. "code_review", "gitlab.snapshot"
	Kind          string    `gorm:"index;size:64;not null" json:"kind"`                                         // skill, mcp, plugin, policy, model_profile, context_provider
	Status        string    `gorm:"index;size:32;not null;default:'active'" json:"status"`                      // active, disabled, retired
	Owner         string    `gorm:"size:128" json:"owner"`                                                      // responsible team or author
	Sensitivity   string    `gorm:"size:32;default:'internal'" json:"sensitivity"`                              // public, internal, restricted
	Description   string    `gorm:"type:text" json:"description"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CapabilityVersion represents an append-only, immutable version of a capability.
type CapabilityVersion struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	CapabilityID     uint       `gorm:"index:idx_cap_ver_scope,unique;not null;column:capability_id" json:"capability_id"`
	Version          int        `gorm:"index:idx_cap_ver_scope,unique;not null" json:"version"`
	ScopeType        string     `gorm:"index:idx_cap_ver_scope,unique;size:32;not null;default:'global';column:scope_type" json:"scope_type"` // global, tenant, project, repository
	ScopeID          string     `gorm:"index:idx_cap_ver_scope,unique;size:160;not null;default:'';column:scope_id" json:"scope_id"`
	ManifestJSON     string     `gorm:"type:text;not null" json:"manifest_json"`
	ContentDigest    string     `gorm:"size:128;not null" json:"content_digest"` // sha256 checksum of manifest & resources
	Signature        string     `gorm:"size:256" json:"signature"`               // publisher signature
	ValidationStatus string     `gorm:"index;size:32;default:'untested'" json:"validation_status"` // untested, passed, failed
	Status           string     `gorm:"index;size:32;not null;default:'draft'" json:"status"`      // draft, active, retired
	ActivatedAt      *time.Time `json:"activated_at"`
	RetiredAt        *time.Time `json:"retired_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// CapabilityResource represents a granular sliced resource of a capability version (e.g. prompt section, evidence schema).
type CapabilityResource struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	CapabilityVersionID uint      `gorm:"index;not null;column:capability_version_id" json:"capability_version_id"`
	ResourceKey         string    `gorm:"index;size:128;not null;column:resource_key" json:"resource_key"` // e.g. "review-method", "evidence-contract"
	ContentKind         string    `gorm:"size:64;not null" json:"content_kind"`                            // text, json, schema, wasm, binary, ref
	ContentHash         string    `gorm:"size:128;not null" json:"content_hash"`
	BlobRef             string    `gorm:"size:256" json:"blob_ref"`
	TokenEstimate       int       `json:"token_estimate"`
	LoadLevel           string    `gorm:"size:16;default:'L1'" json:"load_level"` // L0, L1, L2, L3
	Content             string    `gorm:"type:text" json:"content"`
	CreatedAt           time.Time `json:"created_at"`
}

// CapabilityDependency represents a declared dependency between capabilities.
type CapabilityDependency struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	CapabilityVersionID uint      `gorm:"index;not null;column:capability_version_id" json:"capability_version_id"`
	DependencyKey       string    `gorm:"size:128;not null;column:dependency_key" json:"dependency_key"`
	VersionConstraint   string    `gorm:"size:64;not null;default:'*'" json:"version_constraint"` // e.g. ">=2.0", "^1.0"
	IsOptional          bool      `gorm:"default:false" json:"is_optional"`
	CreatedAt           time.Time `json:"created_at"`
}

// AgentRun represents the universal execution root of an autonomous or assisted agent run.
type AgentRun struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	RunKey            string     `gorm:"uniqueIndex;size:160;not null;column:run_key" json:"run_key"`
	AgentKind         string     `gorm:"index;size:64;not null" json:"agent_kind"` // code_review, deconstructor, triage, etc.
	KernelVersion     string     `gorm:"size:64;not null;default:'v1'" json:"kernel_version"`
	ModelProfileRef   string     `gorm:"size:128" json:"model_profile_ref"`
	State             string     `gorm:"index;size:32;not null;default:'queued'" json:"state"` // queued, resolving, loading, running, verifying, completed, partial, failed, cancelled
	ContextPackID     uint       `gorm:"index;column:context_pack_id" json:"context_pack_id"`
	PermissionGrantID string     `gorm:"size:128" json:"permission_grant_id"`
	BudgetJSON        string     `gorm:"type:text" json:"budget_json"`
	LockfileHash      string     `gorm:"size:128" json:"lockfile_hash"`
	PromptTokens      int        `json:"prompt_tokens"`
	CompletionTokens  int        `json:"completion_tokens"`
	TotalCost         float64    `json:"total_cost"`
	DurationMs        int64      `json:"duration_ms"`
	Error             string     `gorm:"type:text" json:"error"`
	MetadataJSON      string     `gorm:"type:text" json:"metadata_json"`
	CreatedAt         time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	CompletedAt       *time.Time `json:"completed_at"`
}

// RunCapabilityBinding freezes the exact capability versions and loader decisions bound to an AgentRun.
type RunCapabilityBinding struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	RunID               uint      `gorm:"index;not null;column:run_id" json:"run_id"`
	CapabilityVersionID uint      `gorm:"index;not null;column:capability_version_id" json:"capability_version_id"`
	CapabilityKey       string    `gorm:"size:128;not null" json:"capability_key"`
	Version             int       `json:"version"`
	Digest              string    `gorm:"size:128;not null" json:"digest"`
	SelectionReason     string    `gorm:"size:256" json:"selection_reason"`
	LoadLevel           string    `gorm:"size:16;not null;default:'L0'" json:"load_level"` // L0, L1, L2, L3
	PermissionGrant     string    `gorm:"size:256" json:"permission_grant"`
	LoadedAt            time.Time `json:"loaded_at"`
	InvocationCount     int       `gorm:"default:0" json:"invocation_count"`
	TokenCost           int       `gorm:"default:0" json:"token_cost"`
}

// AgentRunEvent represents an append-only timeline audit trace of an AgentRun.
type AgentRunEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	RunID     uint      `gorm:"index:idx_run_seq;not null;column:run_id" json:"run_id"`
	Sequence  int       `gorm:"index:idx_run_seq;not null" json:"sequence"`
	EventType string    `gorm:"index;size:64;not null;column:event_type" json:"event_type"` // intent_compiled, capability_selected, resource_loaded, tool_called, context_expanded, validation_failed, completed, etc.
	PayloadJSON string  `gorm:"type:text" json:"payload_json"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// CapabilityEvaluation stores the output of a deterministic offline/sandbox replay.
type CapabilityEvaluation struct {
	ID                   uint      `gorm:"primaryKey" json:"id"`
	CandidateVersionID   uint      `gorm:"index;column:candidate_version_id" json:"candidate_version_id"`
	BaselineVersionID    uint      `gorm:"index;column:baseline_version_id" json:"baseline_version_id"`
	DatasetRef           string    `gorm:"size:128;not null" json:"dataset_ref"`
	QualityScore         float64   `json:"quality_score"`
	EvidenceScore        float64   `json:"evidence_score"`
	SafetyScore          float64   `json:"safety_score"`
	StabilityScore       float64   `json:"stability_score"`
	TokenEfficiencyScore float64   `json:"token_efficiency_score"`
	LatencyScore         float64   `json:"latency_score"`
	OverallScore         float64   `json:"overall_score"`
	Verdict              string    `gorm:"index;size:32;not null" json:"verdict"` // pass, reject, manual_review
	RegressionsJSON      string    `gorm:"type:text" json:"regressions_json"`
	MetricsJSON          string    `gorm:"type:text" json:"metrics_json"`
	CreatedAt            time.Time `json:"created_at"`
}

// CapabilityProposal represents an optimization recommendation proposed by Strongest Brain.
// Proposals NEVER automatically activate; they require human signoff.
type CapabilityProposal struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	Title                 string     `gorm:"size:256;not null" json:"title"`
	ProposalType          string     `gorm:"index;size:64;not null" json:"proposal_type"` // resolver_tune, budget_realloc, skill_resource_slice, tool_reorder, model_switch
	Status                string     `gorm:"index;size:32;not null;default:'pending_review'" json:"status"` // pending_review, approved, rejected, canary_active, applied
	TargetCapabilityKey   string     `gorm:"index;size:128;not null;column:target_capability_key" json:"target_capability_key"`
	ChangesJSON           string     `gorm:"type:text;not null" json:"changes_json"`
	BaselineMetricsJSON   string     `gorm:"type:text" json:"baseline_metrics_json"`
	CandidateMetricsJSON  string     `gorm:"type:text" json:"candidate_metrics_json"`
	ReplayEvaluationID    uint       `gorm:"index;column:replay_evaluation_id" json:"replay_evaluation_id"`
	CanaryScopeJSON       string     `gorm:"type:text" json:"canary_scope_json"`
	RollbackTriggersJSON  string     `gorm:"type:text" json:"rollback_triggers_json"`
	ReviewedBy            string     `gorm:"size:128" json:"reviewed_by"`
	ReviewedAt            *time.Time `json:"reviewed_at"`
	ReviewDecision        string     `gorm:"size:32" json:"review_decision"` // approved, rejected
	ReviewNotes           string     `gorm:"type:text" json:"review_notes"`
	CreatedAt             time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}
