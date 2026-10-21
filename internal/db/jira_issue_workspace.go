package db

import "time"

// JiraIssueWorkspace is independent of the legacy preview and global integrations.
// Payloads contain public configuration; encrypted credentials are never serialized.
type JiraIssueWorkspace struct {
	ID          uint      `gorm:"primaryKey" json:"-"`
	Version     int       `json:"-"`
	ConfigJSON  string    `gorm:"type:text" json:"-"`
	Credentials string    `gorm:"type:text" json:"-"`
	CatalogJSON string    `gorm:"type:text" json:"-"`
	Lease       string    `gorm:"size:100" json:"-"`
	LeaseUntil  time.Time `json:"-"`
	RunID       uint      `json:"-"`
	Cancelled   bool      `json:"-"`
	DailyKey    string    `gorm:"size:100" json:"-"`
	UpdatedAt   time.Time `json:"-"`
}

type JiraIssueRun struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Kind          string    `gorm:"size:24" json:"kind"`
	ConfigVersion int       `json:"config_version"`
	Status        string    `gorm:"size:24" json:"status"`
	Message       string    `gorm:"size:500" json:"message"`
	ItemCount     int       `json:"-"`
	ResultJSON    string    `gorm:"type:text" json:"-"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// JiraIssueLink preserves remote identity even when users rename an Issue.
type JiraIssueLink struct {
	ID             uint   `gorm:"primaryKey"`
	ScopeKey       string `gorm:"uniqueIndex;size:64"`
	IssueID        int64
	IssueIID       int64
	JiraKey        string `gorm:"index;size:64"`
	JiraOrigin     string `gorm:"size:500"`
	GitLabOrigin   string `gorm:"size:500"`
	ProjectKey     string `gorm:"size:40"`
	ClosedCodeHash string `gorm:"size:64"`
	Closed         bool

	CreatedAt time.Time
}
