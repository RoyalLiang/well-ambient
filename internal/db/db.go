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

// AIContextProfile stores generated module context profiles that can be
// selectively injected into AI deconstruction prompts.
type AIContextProfile struct {
	ID                   uint      `gorm:"primaryKey" json:"id"`
	ModuleName           string    `gorm:"index;size:160" json:"module_name"`
	SourceFilename       string    `json:"source_filename"`
	SourceSheet          string    `json:"source_sheet"`
	Summary              string    `gorm:"type:text" json:"summary"`
	PromptSummary        string    `gorm:"type:text" json:"prompt_summary"`
	FeatureCount         int       `json:"feature_count"`
	ConfigurableCount    int       `json:"configurable_count"`
	NonConfigurableCount int       `json:"non_configurable_count"`
	StatusBreakdownJSON  string    `gorm:"type:text" json:"status_breakdown_json"`
	TypeBreakdownJSON    string    `gorm:"type:text" json:"type_breakdown_json"`
	FeatureSnapshotJSON  string    `gorm:"type:text" json:"feature_snapshot_json"`
	Enabled              bool      `gorm:"index" json:"enabled"`
	Version              int       `gorm:"index" json:"version"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
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
		&AIContextProfile{},
		&ConfigVersion{},
		&GitCommitLog{},
		&Notification{},
		&UserNotificationState{},
		&userdb.User{},
		&userdb.UserGroup{},
		&userdb.UserGroupMembership{},
		&userdb.Permission{},
		&userdb.GroupPermission{},
		&userdb.AuditLog{},
	)
	if err != nil {
		return err
	}

	// Seed data for RBAC
	return userdb.InitializeSeeds(DB)
}
