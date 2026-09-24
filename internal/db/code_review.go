package db

import "time"

// CodeReviewPolicy is an explicit repository-level contract, independent of MR closure.
type CodeReviewPolicy struct {
	ProjectID      string    `gorm:"primaryKey;size:160" json:"project_id"`
	AutoReview     bool      `json:"auto_review"`
	SyncCommits    bool      `json:"sync_commits"`
	SyncMRs        bool      `json:"sync_mrs"`
	Domain         string    `gorm:"size:32" json:"domain"`
	Scenario       string    `gorm:"size:64" json:"scenario"`
	KnowledgeScope string    `gorm:"size:160" json:"knowledge_scope"`
	Rules          string    `gorm:"type:text" json:"rules"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// A run owns an immutable snapshot. Automatic enqueue keys deduplicate webhook deliveries.
type CodeReviewRun struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Key            string     `gorm:"uniqueIndex;size:160" json:"-"`
	ProjectID      string     `gorm:"index;size:160" json:"project_id"`
	Author         string     `json:"author"`
	Repo           string     `json:"repo"`
	Kind           string     `gorm:"size:16" json:"kind"`
	Ref            string     `gorm:"size:128" json:"ref"`
	HeadSHA        string     `gorm:"size:128" json:"head_sha"`
	BaseSHA        string     `gorm:"size:128" json:"base_sha"`
	Title          string     `json:"title"`
	URL            string     `json:"url"`
	Status         string     `gorm:"index;size:32" json:"status"`
	Phase          string     `json:"phase"`
	Error          string     `json:"error"`
	PolicyJSON     string     `gorm:"type:text" json:"policy_json"`
	SnapshotJSON   string     `gorm:"type:text" json:"snapshot_json"`
	ReportJSON     string     `gorm:"type:text" json:"report_json"`
	Model          string     `json:"model"`
	PromptVersion  string     `json:"prompt_version"`
	SkillVersionID uint       `gorm:"index" json:"skill_version_id"`
	SkillVersion   int        `json:"skill_version"`
	SkillScopeType string     `gorm:"size:32" json:"skill_scope_type"`
	SkillScopeID   string     `gorm:"size:160" json:"skill_scope_id"`
	SkillName      string     `gorm:"size:160" json:"skill_name"`
	SkillHash      string     `gorm:"size:64" json:"skill_hash"`
	RetryOfID      uint       `gorm:"index" json:"retry_of_id"`
	PublishStatus  string     `gorm:"index;size:32" json:"publish_status"`
	PublishError   string     `json:"publish_error"`
	CommentID      int        `json:"comment_id"`
	PublishedAt    *time.Time `json:"published_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Shared across reruns to prevent duplicate comments for the same code snapshot.
type CodeReviewPublication struct {
	Key        string `gorm:"primaryKey;size:160"`
	RunID      uint
	Status     string `gorm:"size:32"`
	LeaseToken string `gorm:"size:64"`
	CommentID  int
	UpdatedAt  time.Time
}
