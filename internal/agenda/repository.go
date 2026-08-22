package agenda

import (
	"strings"
	"time"

	"well-ambient/internal/db"
	"well-ambient/internal/telemetry"

	"gorm.io/gorm"
)

const automaticDecisionHistoryLimit = 200

const activeAgendaPredicate = "(status IS NULL OR LOWER(TRIM(status)) <> 'done')"

// automaticDecisionTaskRecord is a read-only ORM projection. The associations
// describe the stable task_id relationship for batched preloading without
// migrating destructive database constraints onto append-only evidence rows.
type automaticDecisionTaskRecord struct {
	TaskID        string `gorm:"primaryKey;column:task_id"`
	Assignee      string
	Status        string
	LastUpdate    time.Time
	GitCommitLogs []db.GitCommitLog `gorm:"foreignKey:TaskID;references:TaskID"`
	Notifications []db.Notification `gorm:"foreignKey:TaskID;references:TaskID"`
}

func (automaticDecisionTaskRecord) TableName() string {
	return "task_telemetries"
}

type agendaSummaryRecords struct {
	ActiveTasks            []db.TaskTelemetry
	AutomaticDecisionTasks []automaticDecisionTaskRecord
	ProjectRepos           []string
}

func loadAgendaSummaryRecords(conn *gorm.DB, projectKeys []string) (agendaSummaryRecords, error) {
	var records agendaSummaryRecords
	err := conn.Transaction(func(tx *gorm.DB) error {
		activeQuery := db.ApplyTaskProjectScope(tx.Model(&db.TaskTelemetry{}), projectKeys)
		if err := activeQuery.
			Select(
				"task_id", "title", "repo", "assignee", "branch", "last_commit",
				"status", "issue_type", "task_created_at", "last_update", "due_date", "decision_logs",
			).
			Where(activeAgendaPredicate).
			Find(&records.ActiveTasks).Error; err != nil {
			return err
		}

		historyQuery := db.ApplyTaskProjectScope(tx.Model(&automaticDecisionTaskRecord{}), projectKeys)
		if err := historyQuery.
			Select("task_id", "assignee", "status", "last_update").
			Where("LOWER(TRIM(status)) IN ?", []string{"done", "review"}).
			Order("last_update DESC").
			Order("task_id ASC").
			Limit(automaticDecisionHistoryLimit).
			Preload("GitCommitLogs", func(preload *gorm.DB) *gorm.DB {
				return preload.
					Select("id", "task_id", "branch", "commit_id", "message", "action", "created_at").
					Where("action = ? AND commit_id <> ?", "git_push", "").
					Order("task_id ASC").
					Order("created_at DESC").
					Order("id DESC")
			}).
			Preload("Notifications", func(preload *gorm.DB) *gorm.DB {
				return preload.
					Select("id", "task_id", "type", "link", "created_at").
					Where("type IN ?", []string{"git_push", telemetry.SemanticLinkerType}).
					Order("task_id ASC").
					Order("created_at DESC").
					Order("id DESC")
			}).
			Find(&records.AutomaticDecisionTasks).Error; err != nil {
			return err
		}

		projectQuery := db.ApplyTaskProjectScope(tx.Model(&db.TaskTelemetry{}), projectKeys)
		return projectQuery.
			Distinct("repo").
			Where("TRIM(repo) <> '' AND repo <> '-'").
			Order("repo ASC").
			Pluck("repo", &records.ProjectRepos).Error
	})
	return records, err
}

func preloadAutomaticDecisionEvidence(conn *gorm.DB, records []automaticDecisionTaskRecord) error {
	if conn == nil || len(records) == 0 {
		return nil
	}
	taskIDs := make([]string, 0, len(records))
	byTaskID := make(map[string]*automaticDecisionTaskRecord, len(records))
	for index := range records {
		taskID := strings.TrimSpace(records[index].TaskID)
		if taskID == "" {
			continue
		}
		taskIDs = append(taskIDs, taskID)
		byTaskID[taskID] = &records[index]
	}
	if len(taskIDs) == 0 {
		return nil
	}

	var gitLogs []db.GitCommitLog
	if err := conn.
		Select("id", "task_id", "branch", "commit_id", "message", "action", "created_at").
		Where("task_id IN ? AND action = ? AND commit_id <> ?", taskIDs, "git_push", "").
		Order("task_id ASC").
		Order("created_at DESC").
		Order("id DESC").
		Find(&gitLogs).Error; err != nil {
		return err
	}
	for _, gitLog := range gitLogs {
		if record := byTaskID[gitLog.TaskID]; record != nil {
			record.GitCommitLogs = append(record.GitCommitLogs, gitLog)
		}
	}

	var notifications []db.Notification
	if err := conn.
		Select("id", "task_id", "type", "link", "created_at").
		Where("task_id IN ? AND type IN ?", taskIDs, []string{"git_push", telemetry.SemanticLinkerType}).
		Order("task_id ASC").
		Order("created_at DESC").
		Order("id DESC").
		Find(&notifications).Error; err != nil {
		return err
	}
	for _, notification := range notifications {
		if record := byTaskID[notification.TaskID]; record != nil {
			record.Notifications = append(record.Notifications, notification)
		}
	}
	return nil
}
