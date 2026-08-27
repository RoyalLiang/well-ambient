package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
	"well-ambient/internal/dailyjira"
	userdb "well-ambient/internal/db/user"
	"well-ambient/internal/readmodel"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the global database instance
var DB *gorm.DB

var sqliteMemorySequence uint64

type Options struct {
	Driver                       string
	DSN                          string
	AutoMigrate                  bool
	MaxOpenConnections           int
	MaxIdleConnections           int
	ConnectionMaxLifetimeMinutes int
	ConnectionMaxIdleTimeMinutes int
	Silent                       bool
}

// WebhookLog stores raw GitLab webhook payloads for auditing
type WebhookLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Event     string    `json:"event"`
	Payload   string    `gorm:"type:text" json:"payload"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// TaskTelemetry tracks the parsed git state for tasks
type TaskTelemetry struct {
	TaskID            string     `gorm:"primaryKey;column:task_id" json:"task_id"`
	ProjectKey        string     `gorm:"index;index:idx_task_jira_project_type,priority:2;size:64;column:project_key" json:"project_key"`
	Source            string     `gorm:"index;index:idx_task_jira_project_type,priority:1;size:32" json:"source"`
	ExternalKey       string     `gorm:"index;size:160;column:external_key" json:"external_key"`
	ParentWorkItemID  string     `gorm:"index;size:160;column:parent_work_item_id" json:"parent_work_item_id"`
	Revision          uint       `gorm:"not null;default:0" json:"revision"`
	PlanningState     string     `gorm:"index;size:32;not null;default:draft;column:planning_state" json:"planning_state"`
	Title             string     `json:"title"`
	Description       string     `json:"description"` // 详细描述
	Repo              string     `json:"repo"`
	Assignee          string     `gorm:"index" json:"assignee"`
	JiraReporter      string     `gorm:"column:jira_reporter" json:"jira_reporter"`
	JiraReporterUser  string     `gorm:"column:jira_reporter_user" json:"-"`
	Creator           string     `json:"creator"`      // 创建人
	CreatorDept       string     `json:"creator_dept"` // 创建人部门
	Branch            string     `gorm:"index" json:"branch"`
	LastCommit        string     `json:"last_commit"`
	Status            string     `gorm:"index" json:"status"` // backlog, progress, review, done
	IssueType         string     `gorm:"index;index:idx_task_jira_project_type,priority:3" json:"issue_type"`
	Priority          string     `gorm:"index;size:32" json:"priority"`
	Severity          string     `gorm:"index;size:32" json:"severity"`
	TaskCreatedAt     time.Time  `json:"task_created_at"`
	LastUpdate        time.Time  `gorm:"index" json:"last_update"`
	SourceUpdatedAt   time.Time  `gorm:"column:source_updated_at" json:"source_updated_at"`
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

// DemandAttachment stores a compressed original uploaded as evidence for demand deconstruction.
type DemandAttachment struct {
	ID                   uint      `gorm:"primaryKey" json:"id"`
	DemandID             string    `gorm:"index;column:demand_id;size:160" json:"demand_id"`
	TaskGroupID          string    `gorm:"index;column:task_group_id;size:160" json:"task_group_id"`
	ContextPackID        uint      `gorm:"index;column:context_pack_id" json:"context_pack_id"`
	DeconstructArchiveID uint      `gorm:"index;column:deconstruct_archive_id" json:"deconstruct_archive_id"`
	UploadedBy           string    `gorm:"index;column:uploaded_by;size:160" json:"uploaded_by"`
	OriginalName         string    `gorm:"size:512" json:"original_name"`
	MimeType             string    `gorm:"size:160" json:"mime_type"`
	Extension            string    `gorm:"size:16" json:"extension"`
	StoragePath          string    `gorm:"uniqueIndex;size:512" json:"storage_path"`
	SHA256               string    `gorm:"index;size:64" json:"sha256"`
	OriginalSize         int64     `json:"original_size"`
	CompressedSize       int64     `json:"compressed_size"`
	Status               string    `gorm:"index;size:32" json:"status"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// ContextDocument stores source-level knowledge records for AI context.
type ContextDocument struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	ParentDocumentID uint      `gorm:"index" json:"parent_document_id"`
	Title            string    `json:"title"`
	OriginalName     string    `gorm:"size:255" json:"original_name"`
	MimeType         string    `gorm:"size:128" json:"mime_type"`
	Type             string    `gorm:"index;size:64" json:"type"`
	Scope            string    `gorm:"index;size:64" json:"scope"`
	ScopeID          string    `gorm:"index;size:160" json:"scope_id"`
	Source           string    `gorm:"index;size:64" json:"source"`
	Owner            string    `json:"owner"`
	ImportedBy       string    `gorm:"size:160" json:"imported_by"`
	Status           string    `gorm:"index;size:32" json:"status"`
	IngestionStatus  string    `gorm:"index;size:32" json:"ingestion_status"`
	IngestionError   string    `gorm:"type:text" json:"ingestion_error"`
	Version          int       `json:"version"`
	ContentHash      string    `gorm:"index;size:64" json:"content_hash"`
	Summary          string    `gorm:"type:text" json:"summary"`
	Content          string    `gorm:"type:text" json:"content"`
	TokenCount       int       `json:"token_count"`
	Freshness        float64   `json:"freshness"`
	Confidence       float64   `json:"confidence"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
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

// RuntimeConfig stores the single current configuration used by settings
// pages. Bootstrap-only database connection settings are deliberately omitted
// so the application never depends on this row to discover its own database.
// ConfigVersion remains the redacted audit and rollback history.
type RuntimeConfig struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Version    int       `gorm:"index" json:"version"`
	ConfigJSON string    `gorm:"type:text" json:"config_json"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// GitCommitLog tracks detailed git activities for tasks (one task to many commits/repos)
type GitCommitLog struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	TaskID             string    `gorm:"index;column:task_id" json:"task_id"`
	Repo               string    `json:"repo"`
	Branch             string    `json:"branch"`
	CommitID           string    `json:"commit_id"`
	Message            string    `json:"message"`
	Author             string    `json:"author"`
	MrIID              int       `json:"mr_iid"`
	MrURL              string    `json:"mr_url"`
	Action             string    `json:"action"` // git_push, mr_open, mr_merge, mr_close, etc.
	DedupeKey          string    `gorm:"index;size:255;column:dedupe_key" json:"dedupe_key,omitempty"`
	ContentFingerprint string    `gorm:"index;size:64;column:content_fingerprint" json:"content_fingerprint,omitempty"`
	DuplicateOfCommit  string    `gorm:"size:160;column:duplicate_of_commit" json:"duplicate_of_commit,omitempty"`
	TelemetryQuality   string    `gorm:"size:32;column:telemetry_quality" json:"telemetry_quality,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

// JiraCommentLog tracks discussion updates pulled from Jira issues (comments)
type JiraCommentLog struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	TaskID          string     `gorm:"index" json:"task_id"`
	CommentID       string     `gorm:"uniqueIndex" json:"comment_id"` // Jira comment ID for idempotency
	Author          string     `json:"author"`
	Body            string     `json:"body"`
	Current         bool       `gorm:"index;not null;default:true" json:"current"`
	CreatedAt       time.Time  `gorm:"index" json:"created_at"`
	SourceUpdatedAt *time.Time `gorm:"index;column:source_updated_at" json:"source_updated_at"`
}

// JiraInboundSyncState is the durable checkpoint for the Jira pull worker.
// SuccessfulThrough advances only after every issue selected for a cycle has
// reconciled successfully, so restarts and partial failures remain retryable.
type JiraInboundSyncState struct {
	Scope             string    `gorm:"primaryKey;size:64" json:"scope"`
	SuccessfulThrough time.Time `gorm:"index;column:successful_through" json:"successful_through"`
	LastStartedAt     time.Time `gorm:"column:last_started_at" json:"last_started_at"`
	LastSucceededAt   time.Time `gorm:"column:last_succeeded_at" json:"last_succeeded_at"`
	LastError         string    `gorm:"type:text;column:last_error" json:"last_error"`
	LastIssueCount    int       `gorm:"column:last_issue_count" json:"last_issue_count"`
	LastChangedCount  int       `gorm:"column:last_changed_count" json:"last_changed_count"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// JiraIssueSyncState records the latest Jira issue revision whose comments and
// dependent projections completed successfully. TaskTelemetry.SourceUpdatedAt
// remains a source fact; this row is operational retry state.
type JiraIssueSyncState struct {
	TaskID          string    `gorm:"primaryKey;size:160;column:task_id" json:"task_id"`
	SourceUpdatedAt time.Time `gorm:"index;column:source_updated_at" json:"source_updated_at"`
	LastSucceededAt time.Time `gorm:"column:last_succeeded_at" json:"last_succeeded_at"`
	LastError       string    `gorm:"type:text;column:last_error" json:"last_error"`
	UpdatedAt       time.Time `json:"updated_at"`
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
	CreatedAt time.Time `gorm:"index" json:"created_at"`
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
	ID              uint      `gorm:"primaryKey" json:"id"`
	ProjectName     string    `gorm:"uniqueIndex" json:"project_name"`
	ProjectKey      string    `gorm:"uniqueIndex;column:project_key" json:"project_key"` // e.g. "HIT"
	GitReposJSON    string    `gorm:"type:text" json:"git_repos_json"`
	BasePriority    string    `json:"base_priority"`                                          // P0, P1, P2
	ProjectPhase    string    `gorm:"column:project_phase;default:'交付'" json:"project_phase"` // e.g. "POC", "交付", "运营", "售后"
	BaseScore       float64   `gorm:"column:base_score;default:60.0" json:"base_score"`
	BaseScoreWeight float64   `gorm:"column:base_score_weight;default:0.10" json:"base_score_weight"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
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

// DecisionEvent stores override, reassignment, and AI inference events for audit trails
type DecisionEvent struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TaskID     string    `gorm:"index" json:"task_id"`
	Actor      string    `json:"actor"`  // 操作人 (如 "AI_Brain", "Eddie")
	Action     string    `json:"action"` // override_assignee, override_due, ai_infer_stuck等
	OldValue   string    `json:"old_value"`
	NewValue   string    `json:"new_value"`
	Reason     string    `json:"reason"`     // 理由
	Confidence float64   `json:"confidence"` // AI 置信度
	CreatedAt  time.Time `json:"created_at"`
}

// DailyJiraDecision stores the operational follow-up state for a morning Jira decision.
// DecisionEvent remains the immutable audit ledger; this record adds the reminder policy
// needed to revisit an unresolved Jira after the meeting.
type DailyJiraDecision struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TaskID     string    `gorm:"index:idx_daily_jira_task_created" json:"task_id"`
	Status     string    `gorm:"index" json:"status"` // follow_up, escalate, reassign
	Assignee   string    `json:"assignee"`
	Actor      string    `json:"actor"`
	Note       string    `gorm:"type:text" json:"note"`
	ReminderAt time.Time `gorm:"index" json:"reminder_at"`
	CreatedAt  time.Time `gorm:"index:idx_daily_jira_task_created" json:"created_at"`
}

// InitDB keeps the legacy SQLite test/development interface. Production callers
// should use Init with an explicit driver and connection policy.
func InitDB(dbPath string) error {
	return Init(Options{
		Driver:             "sqlite",
		DSN:                dbPath,
		AutoMigrate:        true,
		MaxOpenConnections: 4,
		MaxIdleConnections: 2,
	})
}

// Open creates and validates an independent database connection. It is used by
// first-install probes and migration jobs that must not mutate the process-wide
// runtime handle before their work has succeeded.
func Open(options Options) (*gorm.DB, error) {
	driver := strings.ToLower(strings.TrimSpace(options.Driver))
	var dialector gorm.Dialector
	switch driver {
	case "sqlite", "sqlite3":
		driver = "sqlite"
		dialector = sqlite.Open(normalizeSQLiteDSN(options.DSN))
	case "postgres", "postgresql", "pg":
		driver = "postgres"
		dialector = postgres.Open(options.DSN)
	default:
		return nil, fmt.Errorf("unsupported database driver %q", options.Driver)
	}

	gormConfig := &gorm.Config{}
	if options.Silent {
		gormConfig.Logger = logger.Default.LogMode(logger.Silent)
	}
	conn, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return nil, fmt.Errorf("open %s database: %w", driver, err)
	}
	pool, err := conn.DB()
	if err != nil {
		return nil, fmt.Errorf("open %s connection pool: %w", driver, err)
	}
	configureConnectionPool(pool, options)
	pingContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := pool.PingContext(pingContext); err != nil {
		_ = pool.Close()
		return nil, fmt.Errorf("ping %s database: %w", driver, err)
	}

	if options.AutoMigrate {
		if err := Migrate(conn); err != nil {
			_ = pool.Close()
			return nil, err
		}
	}
	if err := readmodel.InstallGORMObserver(conn); err != nil {
		_ = pool.Close()
		return nil, err
	}
	return conn, nil
}

// Init opens and validates the configured database adapter. Migrations run only
// when explicitly requested by Options.AutoMigrate.
func Init(options Options) error {
	conn, err := Open(options)
	if err != nil {
		return err
	}
	DB = conn
	return nil
}

func normalizeSQLiteDSN(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == ":memory:" {
		sequence := atomic.AddUint64(&sqliteMemorySequence, 1)
		return fmt.Sprintf("file:well_ambient_memory_%d?mode=memory&cache=shared&_foreign_keys=on&_busy_timeout=5000", sequence)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + "_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL"
}

func configureConnectionPool(pool *sql.DB, options Options) {
	if options.MaxOpenConnections > 0 {
		pool.SetMaxOpenConns(options.MaxOpenConnections)
	}
	if options.MaxIdleConnections >= 0 {
		pool.SetMaxIdleConns(options.MaxIdleConnections)
	}
	if options.ConnectionMaxLifetimeMinutes > 0 {
		pool.SetConnMaxLifetime(time.Duration(options.ConnectionMaxLifetimeMinutes) * time.Minute)
	}
	if options.ConnectionMaxIdleTimeMinutes > 0 {
		pool.SetConnMaxIdleTime(time.Duration(options.ConnectionMaxIdleTimeMinutes) * time.Minute)
	}
}

func coreSchemaModels() []any {
	return []any{
		&WebhookLog{},
		&TaskTelemetry{},
		&DeconstructArchive{},
		&DemandAttachment{},
		&ContextDocument{},
		&ContextFact{},
		&ContextChunk{},
		&ContextPack{},
		&ContextPackItem{},
		&ConfigVersion{},
		&RuntimeConfig{},
		&GitCommitLog{},
		&JiraCommentLog{},
		&JiraInboundSyncState{},
		&JiraIssueSyncState{},
		&Notification{},
		&UserNotificationState{},
		&UserProjectPreference{},
		&UserTablePreference{},
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
		&ReleaseVersion{},
		&ReleaseJiraLink{},
		&WorkItemReleaseLink{},
		&WorkItemEvent{},
		&WorkItemSyncOperation{},
		&DecisionEvent{},
		&DailyJiraDecision{},
		&DemandSpecVersion{},
		&ReviewContract{},
		&ExecutionRun{},
		&ExecutionAction{},
		&CorpusCandidate{},
		&SolutionAsset{},
		&SolutionRevision{},
		&SolutionSourceRef{},
		&SolutionPolishJob{},
		&SolutionPromptTemplate{},
		&SolutionJiraOutbox{},
		&SolutionCatalogEntry{},
		&SolutionCatalogSearchToken{},
		&SolutionCatalogSyncJob{},
		&SolutionComparison{},
		&SolutionStandardizationProposal{},
		&SolutionStandard{},
		&SolutionStandardRevision{},
		&PerformanceScoreRun{},
		&PerformanceScoreSnapshot{},
		&PerformanceEvidenceFact{},
		&PerformanceWorkItemEvent{},
		&PerformanceAuditEvent{},
	}
}

// RequiredSchemaModels returns every source-of-truth model that must exist
// before a pre-migrated PostgreSQL database can be accepted by setup mode.
// Callers must treat the returned values as read-only schema descriptors.
func RequiredSchemaModels() []any {
	models := coreSchemaModels()
	return append(models, dataAssetModels...)
}

// Migrate applies the application schema to an already-open database. It is
// intentionally callable as a separate deployment step before the service is
// restarted.
func Migrate(conn *gorm.DB) error {
	if err := MigrateSchema(conn); err != nil {
		return err
	}
	return InitializeReferenceData(conn)
}

// MigrateSchema creates application-owned tables and indexes without inserting
// reference rows. The legacy SQLite importer uses this seam so historical rows
// can be copied before idempotent defaults are added.
func MigrateSchema(conn *gorm.DB) error {
	if conn == nil {
		return fmt.Errorf("database is not initialized")
	}

	// Auto migrate schemas
	err := conn.AutoMigrate(coreSchemaModels()...)
	if err != nil {
		return err
	}
	if err := MigrateDataAssets(conn); err != nil {
		return err
	}
	if err := dailyjira.Migrate(conn); err != nil {
		return err
	}
	// Task-tracking read models filter by normalized issue type before status,
	// project and owner. Expression/composite indexes keep those dashboard reads
	// index-backed without changing legacy mixed-case telemetry rows.
	taskTrackingIndexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_task_tracking_kind_status_project_owner
			ON task_telemetries (LOWER(TRIM(issue_type)), status, project_key, assignee)`,
		`CREATE INDEX IF NOT EXISTS idx_task_tracking_active_last_update
			ON task_telemetries (last_update DESC, task_id)
			WHERE status IS NULL OR LOWER(TRIM(status)) NOT IN ('done','closed','resolved','completed','archived','已完成','已关闭')`,
		`CREATE INDEX IF NOT EXISTS idx_task_tracking_parent_group
			ON task_telemetries (parent_work_item_id, task_group_id)`,
		`CREATE INDEX IF NOT EXISTS idx_git_evidence_task_created
			ON git_commit_logs (task_id, created_at DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_git_commit_dedupe_nonempty
			ON git_commit_logs (dedupe_key) WHERE TRIM(dedupe_key) <> ''`,
		`CREATE INDEX IF NOT EXISTS idx_task_agenda_active
			ON task_telemetries (last_update DESC, task_id)
			WHERE status IS NULL OR LOWER(TRIM(status)) <> 'done'`,
		`CREATE INDEX IF NOT EXISTS idx_task_agenda_history
			ON task_telemetries (last_update DESC, task_id)
			WHERE LOWER(TRIM(status)) IN ('done', 'review')`,
		`CREATE INDEX IF NOT EXISTS idx_task_agenda_repo
			ON task_telemetries (repo)`,
		`CREATE INDEX IF NOT EXISTS idx_git_evidence_task_action_created
			ON git_commit_logs (task_id, action, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_task_type_created
			ON notifications (task_id, type, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_jira_event_task_action_created
			ON decision_events (task_id, action, created_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_jira_decision_task_created
			ON daily_jira_decisions (task_id, created_at DESC, id DESC)`,
	}
	for _, statement := range taskTrackingIndexes {
		if err := conn.Exec(statement).Error; err != nil {
			return err
		}
	}
	if err := MigrateAllPageReadIndexes(conn); err != nil {
		return err
	}
	if err := MigratePostgresReadIndexes(conn); err != nil {
		return err
	}
	return nil
}

// InitializeReferenceData inserts only idempotent application defaults. It is
// deliberately separate from schema creation for cross-database data imports.
func InitializeReferenceData(conn *gorm.DB) error {
	if conn == nil {
		return fmt.Errorf("database is not initialized")
	}
	if err := ensureDefaultSolutionPrompts(conn); err != nil {
		return err
	}
	// Seed data for RBAC before attaching the request-scoped query observer so
	// startup migrations and seeds are not reported as HTTP read work.
	return userdb.InitializeSeeds(conn)
}

func Ping(ctx context.Context) error {
	if DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	pool, err := DB.DB()
	if err != nil {
		return err
	}
	return pool.PingContext(ctx)
}

func Close() error {
	if DB == nil {
		return nil
	}
	pool, err := DB.DB()
	if err != nil {
		return err
	}
	DB = nil
	return pool.Close()
}

// CloseConnection closes an independent connection returned by Open without
// changing the global runtime handle.
func CloseConnection(conn *gorm.DB) error {
	if conn == nil {
		return nil
	}
	pool, err := conn.DB()
	if err != nil {
		return err
	}
	return pool.Close()
}

func ensureDefaultSolutionPrompts(conn *gorm.DB) error {
	defaults := []struct {
		purpose, name, prompt string
	}{
		{"solution_polish", "默认方案润色", DefaultSolutionPolishPrompt},
		{"solution_compare_requirement", "默认需求等价性对比", DefaultSolutionRequirementComparisonPrompt},
		{"solution_compare_compatibility", "默认方案兼容性对比", DefaultSolutionCompatibilityComparisonPrompt},
	}
	for _, item := range defaults {
		var count int64
		if err := conn.Model(&SolutionPromptTemplate{}).
			Where("purpose = ? AND scope_type = ? AND scope_id = ?", item.purpose, "global", "").
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		now := time.Now()
		if err := conn.Create(&SolutionPromptTemplate{
			Purpose: item.purpose, ScopeType: "global", ScopeID: "", Version: 1,
			Status: "active", Name: item.name, SystemPrompt: item.prompt,
			CreatedBy: "system", ActivatedBy: "system", ActivatedAt: &now, CreatedAt: now,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}
