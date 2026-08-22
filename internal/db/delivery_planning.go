package db

import (
	"time"

	"gorm.io/gorm"
)

// ReleaseVersion is an independently-owned local release catalog entry.
// It may remain unbound until commitment; project binding and Jira association
// do not replace its stable Source and ExternalID identity.
type ReleaseVersion struct {
	ID          uint           `gorm:"primaryKey;index:idx_release_catalog,priority:4" json:"id"`
	ProjectKey  string         `gorm:"uniqueIndex:idx_release_identity,priority:1;index;index:idx_release_catalog,priority:1;size:64;not null;column:project_key" json:"project_key"`
	Source      string         `gorm:"uniqueIndex:idx_release_identity,priority:2;index;size:32;not null" json:"source"`
	ExternalID  string         `gorm:"uniqueIndex:idx_release_identity,priority:3;size:160;not null;column:external_id" json:"external_id"`
	Name        string         `gorm:"size:255;not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	Status      string         `gorm:"index;index:idx_release_catalog,priority:2;size:32;not null;default:planned" json:"status"`
	StartDate   *time.Time     `json:"start_date"`
	ReleaseDate *time.Time     `gorm:"index:idx_release_catalog,priority:3" json:"release_date"`
	SourceURL   string         `gorm:"size:1024;column:source_url" json:"source_url"`
	SyncedAt    *time.Time     `gorm:"index;column:synced_at" json:"synced_at"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index:idx_release_deleted_at;column:deleted_at" json:"-"`
}

// ReleaseJiraLink attaches one local release fact to one Jira release version.
// The local release remains the owned identity; Jira fields are an external
// reference snapshot and never overwrite the local name, status, or dates.
type ReleaseJiraLink struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	ReleaseVersionID uint      `gorm:"uniqueIndex;not null;column:release_version_id" json:"release_version_id"`
	JiraProjectKey   string    `gorm:"uniqueIndex:idx_release_jira_identity;index;size:64;not null;column:jira_project_key" json:"jira_project_key"`
	JiraVersionID    string    `gorm:"uniqueIndex:idx_release_jira_identity;size:160;not null;column:jira_version_id" json:"jira_version_id"`
	JiraVersionName  string    `gorm:"size:255;not null;column:jira_version_name" json:"jira_version_name"`
	JiraVersionURL   string    `gorm:"size:1024;column:jira_version_url" json:"jira_version_url"`
	JiraStatus       string    `gorm:"index;size:32;column:jira_status" json:"jira_status"`
	LinkedBy         string    `gorm:"size:160;column:linked_by" json:"linked_by"`
	LinkedAt         time.Time `gorm:"index;column:linked_at" json:"linked_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// WorkItemReleaseLink classifies a work item against target and affected
// releases. Active=false retains externally removed relationships for audit.
type WorkItemReleaseLink struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	WorkItemID       string     `gorm:"uniqueIndex:idx_work_item_release_relation,priority:1;index;index:idx_work_item_active_relation_primary,priority:1;size:160;not null;column:work_item_id" json:"work_item_id"`
	ReleaseVersionID uint       `gorm:"uniqueIndex:idx_work_item_release_relation,priority:2;index;index:idx_release_active_relation_primary,priority:1;not null;column:release_version_id" json:"release_version_id"`
	Relation         string     `gorm:"uniqueIndex:idx_work_item_release_relation,priority:3;index;index:idx_work_item_active_relation_primary,priority:3;index:idx_release_active_relation_primary,priority:3;size:32;not null" json:"relation"`
	IsPrimary        bool       `gorm:"index;index:idx_work_item_active_relation_primary,priority:4;index:idx_release_active_relation_primary,priority:4;not null;default:false;column:is_primary" json:"is_primary"`
	Active           bool       `gorm:"index;index:idx_work_item_active_relation_primary,priority:2;index:idx_release_active_relation_primary,priority:2;not null;default:true" json:"active"`
	Source           string     `gorm:"index;size:32;not null" json:"source"`
	ConfirmedBy      string     `gorm:"size:160;column:confirmed_by" json:"confirmed_by"`
	ConfirmedAt      *time.Time `gorm:"column:confirmed_at" json:"confirmed_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// WorkItemEvent is the immutable structured planning audit stream.
type WorkItemEvent struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	WorkItemID string    `gorm:"index;size:160;not null;column:work_item_id" json:"work_item_id"`
	ProjectKey string    `gorm:"index;size:64;column:project_key" json:"project_key"`
	EventType  string    `gorm:"index;size:64;not null;column:event_type" json:"event_type"`
	Actor      string    `gorm:"index;size:160;not null" json:"actor"`
	Reason     string    `gorm:"type:text;not null" json:"reason"`
	BeforeJSON string    `gorm:"type:text;column:before_json" json:"before_json"`
	AfterJSON  string    `gorm:"type:text;column:after_json" json:"after_json"`
	Revision   uint      `gorm:"index;not null" json:"revision"`
	Source     string    `gorm:"index;size:32;not null" json:"source"`
	SyncState  string    `gorm:"index;size:32;not null;default:not_required;column:sync_state" json:"sync_state"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

// WorkItemSyncOperation is an outbox record. Network writes are deliberately
// decoupled from the local planning transaction.
type WorkItemSyncOperation struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	IdempotencyKey string     `gorm:"uniqueIndex;size:255;not null;column:idempotency_key" json:"idempotency_key"`
	WorkItemID     string     `gorm:"index;size:160;not null;column:work_item_id" json:"work_item_id"`
	Operation      string     `gorm:"index;size:64;not null" json:"operation"`
	PayloadJSON    string     `gorm:"type:text;not null;column:payload_json" json:"payload_json"`
	Status         string     `gorm:"index;size:32;not null;default:pending" json:"status"`
	AttemptCount   int        `gorm:"not null;default:0;column:attempt_count" json:"attempt_count"`
	LastError      string     `gorm:"type:text;column:last_error" json:"last_error"`
	NextAttemptAt  *time.Time `gorm:"index;column:next_attempt_at" json:"next_attempt_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
