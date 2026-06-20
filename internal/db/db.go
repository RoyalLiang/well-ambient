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
	TaskID        string     `gorm:"primaryKey;column:task_id" json:"task_id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`   // 详细描述
	Repo          string     `json:"repo"`
	Assignee      string     `json:"assignee"`
	Creator       string     `json:"creator"`       // 创建人
	CreatorDept   string     `json:"creator_dept"`  // 创建人部门
	Branch        string     `json:"branch"`
	LastCommit    string     `json:"last_commit"`
	Status        string     `json:"status"` // backlog, progress, review, done
	IssueType     string     `json:"issue_type"`
	TaskCreatedAt time.Time  `json:"task_created_at"`
	LastUpdate    time.Time  `json:"last_update"`
	CompletedAt   *time.Time `json:"completed_at"`  // 完成时间
	DueDate       *time.Time `json:"due_date"`      // 任务截止时间
	DecisionLogs  string     `json:"decision_logs"` // 会议决策历史，存储为 JSON 字符串
	MrIID         int        `json:"mr_iid"`        // Merge Request IID (e.g. 288)
	MrURL         string     `json:"mr_url"`        // Merge Request URL
	TaskGroupID   string     `gorm:"column:task_group_id" json:"task_group_id"` // 任务组ID，用于关联一组合解构的任务
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
	Type      string    `json:"type"`       // delay, git_push, mr_event, ai_review, semantic_linker
	TaskID    string    `json:"task_id"`    // Associated Task ID
	Title     string    `json:"title"`      // Title of the notification
	Message   string    `json:"message"`    // Detailed body text
	Assignee  string    `json:"assignee"`   // Owner, author or actor
	Link      string    `json:"link"`       // Actionable link (GitLab commit/MR URL)
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

