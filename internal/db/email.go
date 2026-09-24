package db

import "time"

// EmailTemplateCandidate is an independent copy; deleting it never changes the
// active runtime configuration or a configuration version.
type EmailTemplateCandidate struct {
	ID           uint      `gorm:"primaryKey"`
	Name         string    `gorm:"size:320;not null"`
	TemplateJSON string    `gorm:"type:text;not null"`
	CreatedAt    time.Time `gorm:"not null"`
}

// DailyJiraEmailRun retains every claimed attempt, even with an unknown SMTP outcome.
type DailyJiraEmailRun struct {
	Date             string     `gorm:"primaryKey;size:10" json:"date"`
	Timezone         string     `gorm:"size:80" json:"timezone"`
	Trigger          string     `gorm:"size:16" json:"trigger"`
	Status           string     `gorm:"size:32" json:"status"`
	StartedAt        time.Time  `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	RecipientsJSON   string     `gorm:"type:text" json:"recipients_json"`
	ConfluenceStatus string     `gorm:"size:32" json:"confluence_status"`
	ConfluenceError  string     `gorm:"type:text" json:"confluence_error"`
	ConfluenceURL    string     `gorm:"type:text" json:"confluence_url,omitempty"`
}
