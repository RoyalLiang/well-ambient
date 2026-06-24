package db

import (
	"time"
	userdb "well-ambient/internal/db/user"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// DB is the global database instance
var DB *gorm.DB

// WebhookLog stores raw GitLab webhook payloads for auditing
type WebhookLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Event     string    `json:"event"`
	Payload   string    `gorm:"type:text" json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}

// TaskTelemetry tracks the parsed git state for tasks
type TaskTelemetry struct {
	TaskID            string     `gorm:"primaryKey;column:task_id" json:"task_id"`
	Title             string     `json:"title"`
	Description       string     `json:"description"` // 详细描述
	Repo              string     `json:"repo"`
	Assignee          string     `json:"assignee"`
	Creator           string     `json:"creator"`      // 创建人
	CreatorDept       string     `json:"creator_dept"` // 创建人部门
	Branch            string     `json:"branch"`
	LastCommit        string     `json:"last_commit"`
	Status            string     `json:"status"` // backlog, progress, review, done
	IssueType         string     `json:"issue_type"`
	TaskCreatedAt     time.Time  `json:"task_created_at"`
	LastUpdate        time.Time  `json:"last_update"`
	CompletedAt       *time.Time `json:"completed_at"`                              // 完成时间
	DueDate           *time.Time `json:"due_date"`                                  // 任务截止时间
	DecisionLogs      string     `json:"decision_logs"`                             // 会议决策历史，存储为 JSON 字符串
	MrIID             int        `json:"mr_iid"`                                    // Merge Request IID (e.g. 288)
	MrURL             string     `json:"mr_url"`                                    // Merge Request URL
	TaskGroupID       string     `gorm:"column:task_group_id" json:"task_group_id"` // 任务组ID，用于关联一组合解构的任务
	EstimateDays      float64    `json:"estimate_days"`                             // AI 预估开发天数
	EstimateHours     float64    `json:"estimate_hours"`                            // AI 预估开发小时数
	Difficulty        string     `json:"difficulty"`                                // AI 预估难度：High/Medium/Low
	EstimateSource    string     `json:"estimate_source"`                           // 估算来源：ai_deconstruct/manual_adjusted
	EstimateArchiveID uint       `json:"estimate_archive_id"`                       // 对应的解构归档记录
}

// DeconstructArchive stores every AI deconstruction run for later estimate accuracy checks.
type DeconstructArchive struct {
	ID                   uint      `gorm:"primaryKey" json:"id"`
	DemandID             string    `gorm:"index;column:demand_id" json:"demand_id"`
	TaskGroupID          string    `gorm:"index;column:task_group_id" json:"task_group_id"`
	ContextPackID        uint      `gorm:"index;column:context_pack_id" json:"context_pack_id"`
	InputText            string    `gorm:"type:text" json:"input_text"`
	MappedReposJSON      string    `gorm:"type:text" json:"mapped_repos_json"`
	TasksJSON            string    `gorm:"type:text" json:"tasks_json"`
	AnalysisJSON         string    `gorm:"type:text" json:"analysis_json"`
	OverallEstimateDays  float64   `json:"overall_estimate_days"`
	OverallEstimateHours float64   `json:"overall_estimate_hours"`
	OverallDifficulty    string    `json:"overall_difficulty"`
	CompletenessScore    int       `json:"completeness_score"`
	Confidence           float64   `json:"confidence"`
	IsMock               bool      `json:"is_mock"`
	CreatedAt            time.Time `json:"created_at"`
}

// ContextDocument stores source-level knowledge records for AI context.
type ContextDocument struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Title       string    `json:"title"`
	Type        string    `gorm:"index;size:64" json:"type"`
	Scope       string    `gorm:"index;size:64" json:"scope"`
	ScopeID     string    `gorm:"index;size:160" json:"scope_id"`
	Source      string    `gorm:"index;size:64" json:"source"`
	Owner       string    `json:"owner"`
	Status      string    `gorm:"index;size:32" json:"status"`
	Version     int       `json:"version"`
	ContentHash string    `gorm:"index;size:64" json:"content_hash"`
	Summary     string    `gorm:"type:text" json:"summary"`
	Content     string    `gorm:"type:text" json:"content"`
	TokenCount  int       `json:"token_count"`
	Freshness   float64   `json:"freshness"`
	Confidence  float64   `json:"confidence"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ContextFact stores normalized, prompt-ready fact cards.
type ContextFact struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	ContextDocumentID uint      `gorm:"index;column:context_document_id" json:"context_document_id"`
	Type              string    `gorm:"index;size:64" json:"type"`
	Scope             string    `gorm:"index;size:64" json:"scope"`
	ScopeID           string    `gorm:"index;size:160" json:"scope_id"`
	Source            string    `gorm:"index;size:64" json:"source"`
	Owner             string    `json:"owner"`
	Status            string    `gorm:"index;size:32" json:"status"`
	Version           int       `json:"version"`
	ContentHash       string    `gorm:"index;size:64" json:"content_hash"`
	Summary           string    `gorm:"type:text" json:"summary"`
	Content           string    `gorm:"type:text" json:"content"`
	TokenCount        int       `json:"token_count"`
	Freshness         float64   `json:"freshness"`
	Confidence        float64   `json:"confidence"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ContextChunk stores compressed chunks that can later back fact selection.
type ContextChunk struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	ContextDocumentID uint      `gorm:"index;column:context_document_id" json:"context_document_id"`
	ContextFactID     uint      `gorm:"index;column:context_fact_id" json:"context_fact_id"`
	Type              string    `gorm:"index;size:64" json:"type"`
	Scope             string    `gorm:"index;size:64" json:"scope"`
	ScopeID           string    `gorm:"index;size:160" json:"scope_id"`
	Source            string    `gorm:"index;size:64" json:"source"`
	Status            string    `gorm:"index;size:32" json:"status"`
	ContentHash       string    `gorm:"index;size:64" json:"content_hash"`
	Summary           string    `gorm:"type:text" json:"summary"`
	Content           string    `gorm:"type:text" json:"content"`
	TokenCount        int       `json:"token_count"`
	Freshness         float64   `json:"freshness"`
	Confidence        float64   `json:"confidence"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ContextPack records the exact context assembled for a preview or deconstruction run.
type ContextPack struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`
	Purpose               string    `gorm:"index;size:64" json:"purpose"`
	DemandTextHash        string    `gorm:"index;size:64" json:"demand_text_hash"`
	ScopeSignature        string    `gorm:"type:text" json:"scope_signature"`
	Model                 string    `json:"model"`
	PromptTemplateVersion string    `json:"prompt_template_version"`
	WorkHoursPerDay       float64   `json:"work_hours_per_day"`
	TokenBudget           int       `json:"token_budget"`
	TokenCount            int       `json:"token_count"`
	Summary               string    `gorm:"type:text" json:"summary"`
	ContextHash           string    `gorm:"index;size:64" json:"context_hash"`
	ItemCount             int       `json:"item_count"`
	CreatedAt             time.Time `json:"created_at"`
}

// ContextPackItem records each fact/chunk included in a generated pack.
type ContextPackItem struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ContextPackID  uint      `gorm:"index;column:context_pack_id" json:"context_pack_id"`
	ContextFactID  uint      `gorm:"index;column:context_fact_id" json:"context_fact_id"`
	ContextChunkID uint      `gorm:"index;column:context_chunk_id" json:"context_chunk_id"`
	Position       int       `json:"position"`
	Score          float64   `json:"score"`
	TokenCount     int       `json:"token_count"`
	Summary        string    `gorm:"type:text" json:"summary"`
	CreatedAt      time.Time `json:"created_at"`
}

// ConfigVersion stores a redacted, versioned snapshot of integration config
// changes for audit, diff, rollback, and frontend synchronization.
type ConfigVersion struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`
	Version               int       `gorm:"uniqueIndex" json:"version"`
	ActorID               string    `gorm:"index;size:160" json:"actor_id"`
	ActorName             string    `json:"actor_name"`
	Source                string    `gorm:"size:64" json:"source"`
	ConfigJSON            string    `gorm:"type:text" json:"config_json"`
	RedactedConfigJSON    string    `gorm:"type:text" json:"redacted_config_json"`
	ChangedSectionsJSON   string    `gorm:"type:text" json:"changed_sections_json"`
	DiffJSON              string    `gorm:"type:text" json:"diff_json"`
	PreviousVersionID     uint      `json:"previous_version_id"`
	RollbackFromVersionID uint      `json:"rollback_from_version_id"`
	CreatedAt             time.Time `json:"created_at"`
}

// GitCommitLog tracks detailed git activities for tasks (one task to many commits/repos)
type GitCommitLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TaskID    string    `gorm:"index;column:task_id" json:"task_id"`
	Repo      string    `json:"repo"`
	Branch    string    `json:"branch"`
	CommitID  string    `json:"commit_id"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	MrIID     int       `json:"mr_iid"`
	MrURL     string    `json:"mr_url"`
	Action    string    `json:"action"` // git_push, mr_open, mr_merge, mr_close, etc.
	CreatedAt time.Time `json:"created_at"`
}

// Notification represents a notification event in the system
type Notification struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Type      string    `json:"type"`     // delay, git_push, mr_event, ai_review, semantic_linker
	TaskID    string    `json:"task_id"`  // Associated Task ID
	Title     string    `json:"title"`    // Title of the notification
	Message   string    `json:"message"`  // Detailed body text
	Assignee  string    `json:"assignee"` // Owner, author or actor
	Link      string    `json:"link"`     // Actionable link (GitLab commit/MR URL)
	CreatedAt time.Time `json:"created_at"`
}

// UserNotificationState tracks read/dismissed states for notifications on a per-user basis
type UserNotificationState struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	NotificationKey string    `gorm:"index:idx_user_notif,unique;column:notification_key" json:"notification_key"` // e.g. "delay_TASK-101" or "git_push_45"
	UserID          string    `gorm:"index:idx_user_notif,unique;column:user_id" json:"user_id"`
	Status          string    `json:"status"` // unread, dismissed
	UpdatedAt       time.Time `json:"updated_at"`
}

// ProjectConfig stores the config and base priority of a project
type ProjectConfig struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ProjectName  string    `gorm:"uniqueIndex" json:"project_name"`
	ProjectKey   string    `gorm:"uniqueIndex;column:project_key" json:"project_key"` // e.g. "HIT"
	GitReposJSON string    `gorm:"type:text" json:"git_repos_json"`
	BasePriority string    `json:"base_priority"` // P0, P1, P2
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ProjectScore stores the computed health score history
type ProjectScore struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	ProjectKey          string    `gorm:"index" json:"project_key"`
	ProjectName         string    `json:"project_name"`
	ScheduleHealthScore float64   `json:"schedule_health_score"` // SH
	EngineeringQuality  float64   `json:"engineering_quality"`   // EQ
	CollaborationEffic  float64   `json:"collaboration_effic"`   // CE
	StabilityIndex      float64   `json:"stability_index"`       // SI
	CompoundScore       float64   `json:"compound_score"`        // PHDI
	Diagnostic          string    `gorm:"type:text" json:"diagnostic"`
	SnapshotDate        string    `gorm:"index" json:"snapshot_date"`
	CreatedAt           time.Time `json:"created_at"`
}

// InitDB initializes the SQLite connection and runs auto-migrations
func InitDB(dbPath string) error {
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return err
	}

	// Auto migrate schemas
	err = DB.AutoMigrate(
		&WebhookLog{},
		&TaskTelemetry{},
		&DeconstructArchive{},
		&ContextDocument{},
		&ContextFact{},
		&ContextChunk{},
		&ContextPack{},
		&ContextPackItem{},
		&ConfigVersion{},
		&GitCommitLog{},
		&Notification{},
		&UserNotificationState{},
		&userdb.User{},
		&userdb.UserGroup{},
		&userdb.UserGroupMembership{},
		&userdb.Permission{},
		&userdb.GroupPermission{},
		&userdb.AuthorizationPolicy{},
		&userdb.AuthorizationAuditLog{},
		&userdb.AuditLog{},
		&ProjectConfig{},
		&ProjectScore{},
	)
	if err != nil {
		return err
	}

	// Seed data for RBAC
	return userdb.InitializeSeeds(DB)
}
