package db

import "time"

// JiraIssueSyncConfig controls a preview-only integration, independent of Daily Jira.
type JiraIssueSyncConfig struct {
	LastDailyKey    string    `gorm:"size:160" json:"-"`
	ID              uint      `gorm:"primaryKey" json:"id"`
	Version         int       `json:"version"`
	Enabled         bool      `json:"enabled"`
	TimeOfDay       string    `json:"time_of_day"`
	JiraProject     string    `json:"jira_project"`
	GroupPath       string    `json:"group_path"`
	TargetProjectID string    `json:"target_project_id"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type JiraIssueSyncPreview struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	RunKey          string    `gorm:"uniqueIndex;size:160" json:"-"`
	ConfigVersion   int       `json:"config_version"`
	JiraProject     string    `json:"jira_project"`
	TargetProjectID string    `json:"target_project_id"`
	GroupPath       string    `json:"group_path"`
	Total           int64     `json:"total"`
	ItemsJSON       string    `gorm:"type:text" json:"-"`
	CreatedAt       time.Time `json:"created_at"`
}
