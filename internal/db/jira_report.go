package db

import "time"

// JiraReportChange is a source changelog item. It remains available to daily
// reports independently of the optional performance-scoring module.
type JiraReportChange struct {
	ID         uint   `gorm:"primaryKey"`
	TaskID     string `gorm:"uniqueIndex:idx_jira_report_change,priority:1;index;size:160"`
	HistoryID  string `gorm:"uniqueIndex:idx_jira_report_change,priority:2;size:160"`
	ItemIndex  int    `gorm:"uniqueIndex:idx_jira_report_change,priority:3"`
	Field      string
	FromValue  string
	ToValue    string
	FromID     string
	ToID       string
	OccurredAt time.Time `gorm:"index"`
}
