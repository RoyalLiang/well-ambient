package db

import "time"

// DemandSpecVersion is an immutable, versioned snapshot of a demand once frozen.
// JSON fields intentionally preserve the AI/human reviewed structure without
// coupling persistence to one prompt schema.
type DemandSpecVersion struct {
	ID                     uint       `gorm:"primaryKey" json:"id"`
	DemandID               string     `gorm:"uniqueIndex:idx_demand_spec_version;index;size:160" json:"demand_id"`
	Version                int        `gorm:"uniqueIndex:idx_demand_spec_version" json:"version"`
	Status                 string     `gorm:"index;size:32" json:"status"`
	SourceArchiveID        uint       `gorm:"index" json:"source_archive_id"`
	ContextPackID          uint       `gorm:"index" json:"context_pack_id"`
	OriginalText           string     `gorm:"type:text" json:"original_text"`
	Intent                 string     `gorm:"size:64" json:"intent"`
	IntentConfidence       float64    `json:"intent_confidence"`
	Summary                string     `gorm:"type:text" json:"summary"`
	UserGoal               string     `gorm:"type:text" json:"user_goal"`
	FactsJSON              string     `gorm:"type:text" json:"facts_json"`
	InferencesJSON         string     `gorm:"type:text" json:"inferences_json"`
	MissingContextJSON     string     `gorm:"type:text" json:"missing_context_json"`
	BusinessRulesJSON      string     `gorm:"type:text" json:"business_rules_json"`
	MainFlowsJSON          string     `gorm:"type:text" json:"main_flows_json"`
	ExceptionFlowsJSON     string     `gorm:"type:text" json:"exception_flows_json"`
	PermissionRulesJSON    string     `gorm:"type:text" json:"permission_rules_json"`
	DataImpactJSON         string     `gorm:"type:text" json:"data_impact_json"`
	APIImpactJSON          string     `gorm:"type:text" json:"api_impact_json"`
	UIImpactJSON           string     `gorm:"type:text" json:"ui_impact_json"`
	DependenciesJSON       string     `gorm:"type:text" json:"dependencies_json"`
	RisksJSON              string     `gorm:"type:text" json:"risks_json"`
	AcceptanceCriteriaJSON string     `gorm:"type:text" json:"acceptance_criteria_json"`
	TestPlanJSON           string     `gorm:"type:text" json:"test_plan_json"`
	MappedReposJSON        string     `gorm:"type:text" json:"mapped_repos_json"`
	TasksJSON              string     `gorm:"type:text" json:"tasks_json"`
	ReadinessScore         int        `json:"readiness_score"`
	ModelVersion           string     `json:"model_version"`
	RuleVersion            string     `json:"rule_version"`
	AuthoredBy             string     `gorm:"size:160" json:"authored_by"`
	ReviewedBy             string     `gorm:"size:160" json:"reviewed_by"`
	FrozenBy               string     `gorm:"size:160" json:"frozen_by"`
	FrozenAt               *time.Time `json:"frozen_at"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// ReviewContract captures the review roles and approval policy frozen with a spec.
type ReviewContract struct {
	ID                     uint       `gorm:"primaryKey" json:"id"`
	DemandSpecVersionID    uint       `gorm:"uniqueIndex;index" json:"demand_spec_version_id"`
	DemandID               string     `gorm:"index;size:160" json:"demand_id"`
	Status                 string     `gorm:"index;size:32" json:"status"`
	RequiredRolesJSON      string     `gorm:"type:text" json:"required_roles_json"`
	ReviewerCandidatesJSON string     `gorm:"type:text" json:"reviewer_candidates_json"`
	ResolvedReviewersJSON  string     `gorm:"type:text" json:"resolved_reviewers_json"`
	AcceptanceOwner        string     `gorm:"size:160" json:"acceptance_owner"`
	MinimumApprovals       int        `json:"minimum_approvals"`
	ProtectedPathRulesJSON string     `gorm:"type:text" json:"protected_path_rules_json"`
	SegregationRulesJSON   string     `gorm:"type:text" json:"segregation_rules_json"`
	ReviewSLAHours         int        `json:"review_sla_hours"`
	EscalationOwner        string     `gorm:"size:160" json:"escalation_owner"`
	ResolutionStatus       string     `gorm:"size:32" json:"resolution_status"`
	ResolutionReason       string     `gorm:"type:text" json:"resolution_reason"`
	ApprovedBy             string     `gorm:"size:160" json:"approved_by"`
	ApprovedAt             *time.Time `json:"approved_at"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// ExecutionRun is the idempotent orchestration root for one spec/repository run.
type ExecutionRun struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	RunKey                string     `gorm:"uniqueIndex;size:96" json:"run_key"`
	DemandID              string     `gorm:"index;size:160" json:"demand_id"`
	DemandSpecVersionID   uint       `gorm:"index" json:"demand_spec_version_id"`
	ReviewContractID      uint       `gorm:"index" json:"review_contract_id"`
	TaskGroupID           string     `gorm:"index;size:160" json:"task_group_id"`
	Repo                  string     `gorm:"index;size:160" json:"repo"`
	ProjectRef            string     `gorm:"size:256" json:"project_ref"`
	Provider              string     `gorm:"size:32" json:"provider"`
	BaseBranch            string     `gorm:"size:256" json:"base_branch"`
	TopicBranch           string     `gorm:"index;size:256" json:"topic_branch"`
	Status                string     `gorm:"index;size:32" json:"status"`
	InitiatedBy           string     `gorm:"size:160" json:"initiated_by"`
	ServiceIdentity       string     `gorm:"size:160" json:"service_identity"`
	BlockReason           string     `gorm:"type:text" json:"block_reason"`
	ChangeSetJSON         string     `gorm:"type:text" json:"change_set_json"`
	ChangedPathsJSON      string     `gorm:"type:text" json:"changed_paths_json"`
	TestCommandsJSON      string     `gorm:"type:text" json:"test_commands_json"`
	TestResultJSON        string     `gorm:"type:text" json:"test_result_json"`
	ResolvedReviewersJSON string     `gorm:"type:text" json:"resolved_reviewers_json"`
	CommitSHA             string     `gorm:"size:128" json:"commit_sha"`
	MRIID                 int        `json:"mr_iid"`
	MRURL                 string     `gorm:"type:text" json:"mr_url"`
	MRState               string     `gorm:"size:32" json:"mr_state"`
	PipelineStatus        string     `gorm:"size:32" json:"pipeline_status"`
	AcceptanceState       string     `gorm:"size:32" json:"acceptance_state"`
	StartedAt             *time.Time `json:"started_at"`
	CompletedAt           *time.Time `json:"completed_at"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// ExecutionAction is an append-only audit event for an execution run.
type ExecutionAction struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ExecutionRunID uint      `gorm:"index" json:"execution_run_id"`
	ActionKey      string    `gorm:"uniqueIndex;size:128" json:"action_key"`
	Action         string    `gorm:"index;size:64" json:"action"`
	Status         string    `gorm:"index;size:32" json:"status"`
	Actor          string    `gorm:"size:160" json:"actor"`
	RequestHash    string    `gorm:"size:64" json:"request_hash"`
	RequestJSON    string    `gorm:"type:text" json:"request_json"`
	ResultJSON     string    `gorm:"type:text" json:"result_json"`
	ExternalRef    string    `gorm:"type:text" json:"external_ref"`
	ErrorMessage   string    `gorm:"type:text" json:"error_message"`
	CreatedAt      time.Time `json:"created_at"`
}

// CorpusCandidate is a governed proposal; pending/rejected records are never
// selected by the production context-pack builder.
type CorpusCandidate struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	ExecutionRunID        uint       `gorm:"index" json:"execution_run_id"`
	DemandSpecVersionID   uint       `gorm:"index" json:"demand_spec_version_id"`
	ContextPackID         uint       `gorm:"index" json:"context_pack_id"`
	CandidateType         string     `gorm:"index;size:64" json:"candidate_type"`
	Scope                 string     `gorm:"index;size:64" json:"scope"`
	ScopeID               string     `gorm:"index;size:160" json:"scope_id"`
	Title                 string     `json:"title"`
	Summary               string     `gorm:"type:text" json:"summary"`
	Content               string     `gorm:"type:text" json:"content"`
	ProvenanceJSON        string     `gorm:"type:text" json:"provenance_json"`
	Confidence            float64    `json:"confidence"`
	Sensitivity           string     `gorm:"size:32" json:"sensitivity"`
	Status                string     `gorm:"index;size:32" json:"status"`
	ReviewedBy            string     `gorm:"size:160" json:"reviewed_by"`
	ReviewNote            string     `gorm:"type:text" json:"review_note"`
	AcceptedContextFactID uint       `gorm:"index" json:"accepted_context_fact_id"`
	ExpiresAt             *time.Time `json:"expires_at"`
	ReviewedAt            *time.Time `json:"reviewed_at"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}
