package db

import "time"

// IntegrationSource identifies an external system or Agent host. It is used
// for provenance and quotas, not as a source-specific business role.
type IntegrationSource struct {
	ID           string    `gorm:"primaryKey;size:64" json:"id"`
	Name         string    `gorm:"uniqueIndex;size:160;not null" json:"name"`
	Status       string    `gorm:"index;size:32;not null" json:"status"`
	Owner        string    `gorm:"size:160" json:"owner"`
	QuotaProfile string    `gorm:"size:64;not null" json:"quota_profile"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// IntegrationCredential stores only a one-way verifier. Plaintext credentials
// are returned once by the management command and never persisted.
type IntegrationCredential struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	KeyID      string     `gorm:"uniqueIndex;size:32;not null" json:"key_id"`
	SourceID   string     `gorm:"index;size:64;not null" json:"source_id"`
	Prefix     string     `gorm:"size:24;not null" json:"prefix"`
	Verifier   string     `gorm:"size:64;not null" json:"-"`
	Status     string     `gorm:"index;size:32;not null" json:"status"`
	ExpiresAt  *time.Time `gorm:"index" json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// IntegrationQuotaWindow is the shared fixed-window request counter for one
// trusted source. Composite identity prevents rotated keys from multiplying
// the source request budget.
type IntegrationQuotaWindow struct {
	SourceID     string    `gorm:"primaryKey;size:64" json:"source_id"`
	WindowStart  time.Time `gorm:"primaryKey" json:"window_start"`
	RequestCount int       `gorm:"not null;default:0" json:"request_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// IntegrationQuotaLease is a crash-recoverable shared concurrency slot.
type IntegrationQuotaLease struct {
	ID        string    `gorm:"primaryKey;size:64" json:"id"`
	SourceID  string    `gorm:"index;size:64;not null" json:"source_id"`
	ExpiresAt time.Time `gorm:"index;not null" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// IntegrationPolicyVersion is append-only. Exactly one version should be
// active at a time; activation is serialized by the management service.
type IntegrationPolicyVersion struct {
	ID                      uint       `gorm:"primaryKey" json:"id"`
	Version                 int        `gorm:"uniqueIndex;not null" json:"version"`
	Status                  string     `gorm:"index;size:32;not null" json:"status"`
	AllowedProjectsJSON     string     `gorm:"type:text;not null" json:"allowed_projects_json"`
	AllowedRepositoriesJSON string     `gorm:"type:text;not null" json:"allowed_repositories_json"`
	ActionsJSON             string     `gorm:"type:text;not null" json:"actions_json"`
	FieldRulesJSON          string     `gorm:"type:text;not null" json:"field_rules_json"`
	DataRulesJSON           string     `gorm:"type:text;not null;default:'{}'" json:"data_rules_json"`
	ApprovalRulesJSON       string     `gorm:"type:text;not null" json:"approval_rules_json"`
	QueryLimitsJSON         string     `gorm:"type:text;not null" json:"query_limits_json"`
	Digest                  string     `gorm:"uniqueIndex;size:64;not null" json:"digest"`
	CreatedBy               string     `gorm:"size:160" json:"created_by"`
	CreatedAt               time.Time  `json:"created_at"`
	ActivatedAt             *time.Time `json:"activated_at,omitempty"`
}

// JiraExecutionBinding maps a project and action class to a system-owned Jira
// connector and executor reference. Secret material remains outside this row.
type JiraExecutionBinding struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ProjectRef   string    `gorm:"uniqueIndex:idx_jira_execution_binding,priority:1;size:64;not null" json:"project_ref"`
	ActionClass  string    `gorm:"uniqueIndex:idx_jira_execution_binding,priority:2;size:64;not null" json:"action_class"`
	ConnectorRef string    `gorm:"size:160;not null" json:"connector_ref"`
	ExecutorRef  string    `gorm:"size:160;not null" json:"executor_ref"`
	Version      int       `gorm:"not null" json:"version"`
	Status       string    `gorm:"index;size:32;not null" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// DecisionPlan freezes a validated target, desired changes and preconditions.
type DecisionPlan struct {
	ID                string    `gorm:"primaryKey;size:64" json:"id"`
	SourceID          string    `gorm:"index;size:64;not null" json:"source_id"`
	TargetRef         string    `gorm:"index;size:160;not null" json:"target_ref"`
	ProjectRef        string    `gorm:"index;size:64;not null" json:"project_ref"`
	ChangesJSON       string    `gorm:"type:text;not null" json:"changes_json"`
	PreconditionsJSON string    `gorm:"type:text;not null" json:"preconditions_json"`
	PolicyVersion     int       `gorm:"index;not null" json:"policy_version"`
	ExpiresAt         time.Time `gorm:"index;not null" json:"expires_at"`
	Digest            string    `gorm:"size:64;not null" json:"digest"`
	ApprovalRef       string    `gorm:"size:160" json:"approval_ref,omitempty"`
	State             string    `gorm:"index;size:32;not null" json:"state"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// OpenOperation is the persistent execution root for a DecisionPlan.
type OpenOperation struct {
	ID                 string    `gorm:"primaryKey;size:64" json:"id"`
	PlanID             string    `gorm:"uniqueIndex;size:64;not null" json:"plan_id"`
	SourceID           string    `gorm:"uniqueIndex:idx_open_operation_idempotency,priority:1;index;size:64;not null" json:"source_id"`
	KeyID              string    `gorm:"index;size:32;not null" json:"key_id"`
	Scope              string    `gorm:"uniqueIndex:idx_open_operation_idempotency,priority:2;size:64;not null" json:"scope"`
	IdempotencyKey     string    `gorm:"uniqueIndex:idx_open_operation_idempotency,priority:3;size:160;not null" json:"idempotency_key"`
	RequestDigest      string    `gorm:"size:64;not null" json:"request_digest"`
	State              string    `gorm:"index;size:32;not null" json:"state"`
	RemoteEvidenceJSON string    `gorm:"type:text" json:"remote_evidence_json"`
	LastErrorCode      string    `gorm:"size:64" json:"last_error_code,omitempty"`
	LastError          string    `gorm:"type:text" json:"last_error,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// OpenOperationAction records desired and confirmed values independently.
type OpenOperationAction struct {
	ID                   uint       `gorm:"primaryKey" json:"id"`
	OperationID          string     `gorm:"index;uniqueIndex:idx_open_action_position,priority:1;size:64;not null" json:"operation_id"`
	Position             int        `gorm:"uniqueIndex:idx_open_action_position,priority:2;not null;default:0" json:"position"`
	ActionType           string     `gorm:"size:64;not null" json:"action_type"`
	DesiredValue         string     `gorm:"type:text;not null" json:"desired_value"`
	ConfirmedRemoteValue string     `gorm:"type:text" json:"confirmed_remote_value,omitempty"`
	ConnectorRef         string     `gorm:"size:160" json:"connector_ref,omitempty"`
	ExecutorRef          string     `gorm:"size:160" json:"executor_ref,omitempty"`
	BindingVersion       int        `json:"binding_version,omitempty"`
	State                string     `gorm:"index;size:32;not null" json:"state"`
	RemoteEvidenceJSON   string     `gorm:"type:text" json:"remote_evidence_json"`
	LastErrorCode        string     `gorm:"size:64" json:"last_error_code,omitempty"`
	LastError            string     `gorm:"type:text" json:"last_error,omitempty"`
	DispatchedAt         *time.Time `json:"dispatched_at,omitempty"`
	ConfirmedAt          *time.Time `json:"confirmed_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// OpenOutbox gives every operation action durable queue ownership and lease
// state. Unknown outcomes are retained for verification rather than retried.
type OpenOutbox struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	OperationActionID uint       `gorm:"uniqueIndex;not null" json:"operation_action_id"`
	State             string     `gorm:"index;size:32;not null" json:"state"`
	LeaseToken        string     `gorm:"size:64" json:"lease_token,omitempty"`
	LeasedUntil       *time.Time `gorm:"index" json:"leased_until,omitempty"`
	Attempt           int        `gorm:"not null;default:0" json:"attempt"`
	NextAttemptAt     time.Time  `gorm:"index;not null" json:"next_attempt_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// CapabilityInvocation is the externally verifiable trace shared by HTTP, MCP
// and internal controlled invocations.
type CapabilityInvocation struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	RequestID      string    `gorm:"uniqueIndex;size:64;not null" json:"request_id"`
	SourceID       string    `gorm:"index;size:64;not null" json:"source_id"`
	KeyID          string    `gorm:"index;size:32" json:"key_id,omitempty"`
	Transport      string    `gorm:"index;size:32;not null" json:"transport"`
	ToolName       string    `gorm:"index;size:128;not null" json:"tool_name"`
	ToolVersion    string    `gorm:"size:32;not null" json:"tool_version"`
	PolicyVersion  int       `gorm:"index;not null" json:"policy_version"`
	TargetRefsJSON string    `gorm:"type:text" json:"target_refs_json"`
	DurationMS     int64     `json:"duration_ms"`
	Outcome        string    `gorm:"index;size:32;not null" json:"outcome"`
	ErrorCode      string    `gorm:"index;size:64" json:"error_code,omitempty"`
	CreatedAt      time.Time `gorm:"index" json:"created_at"`
}

// OpenQuerySnapshot freezes a public query result for stable pagination and
// drill-down semantics while mutable source projections continue to sync.
type OpenQuerySnapshot struct {
	ID            string    `gorm:"primaryKey;size:64" json:"id"`
	Contract      string    `gorm:"index;size:64;not null" json:"contract"`
	PolicyVersion int       `gorm:"index;not null" json:"policy_version"`
	ScopeDigest   string    `gorm:"index;size:64;not null" json:"scope_digest"`
	DataAsOf      time.Time `gorm:"index;not null" json:"data_as_of"`
	PayloadJSON   string    `gorm:"type:text;not null" json:"payload_json"`
	Completeness  float64   `json:"completeness"`
	MissingJSON   string    `gorm:"type:text;not null" json:"missing_json"`
	ExpiresAt     time.Time `gorm:"index;not null" json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
}
