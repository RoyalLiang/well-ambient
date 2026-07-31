package db

import (
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

var taskProjectCompatibilityFallbackCount atomic.Uint64

// UserProjectPreference stores one selected Jira project for one authenticated user.
// No rows for a user means the backward-compatible default: all projects.
type UserProjectPreference struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Username   string    `gorm:"uniqueIndex:idx_user_project_preference;index;size:160;not null" json:"username"`
	ProjectKey string    `gorm:"uniqueIndex:idx_user_project_preference;index;size:64;not null" json:"project_key"`
	CreatedAt  time.Time `json:"created_at"`
}

// NormalizeProjectKeys trims, uppercases, de-duplicates, and sorts project keys.
func NormalizeProjectKeys(keys []string) []string {
	seen := make(map[string]struct{}, len(keys))
	normalized := make([]string, 0, len(keys))
	for _, key := range keys {
		key = strings.ToUpper(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, key)
	}
	sort.Strings(normalized)
	return normalized
}

// LoadUserProjectPreferenceKeys returns the selected keys for a user.
// An empty result means all projects.
func LoadUserProjectPreferenceKeys(conn *gorm.DB, username string) ([]string, error) {
	username = strings.TrimSpace(username)
	if conn == nil || username == "" {
		return nil, nil
	}
	// Some narrow test databases and rolling-upgrade readers may not have run
	// the new migration yet. Preserve the documented default instead of turning
	// a missing preference table into a read-path outage.
	if !conn.Migrator().HasTable(&UserProjectPreference{}) {
		return nil, nil
	}

	var keys []string
	if err := conn.Model(&UserProjectPreference{}).
		Where("username = ?", username).
		Order("project_key asc").
		Pluck("project_key", &keys).Error; err != nil {
		return nil, err
	}
	return NormalizeProjectKeys(keys), nil
}

// ReplaceUserProjectPreferences atomically replaces a user's selection.
// Passing an empty list restores the all-project default.
func ReplaceUserProjectPreferences(conn *gorm.DB, username string, keys []string) error {
	username = strings.TrimSpace(username)
	if conn == nil {
		return fmt.Errorf("database is not initialized")
	}
	if username == "" {
		return fmt.Errorf("username is required")
	}
	keys = NormalizeProjectKeys(keys)

	return conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("username = ?", username).Delete(&UserProjectPreference{}).Error; err != nil {
			return err
		}
		if len(keys) == 0 {
			return nil
		}

		rows := make([]UserProjectPreference, 0, len(keys))
		for _, key := range keys {
			rows = append(rows, UserProjectPreference{Username: username, ProjectKey: key})
		}
		return tx.Create(&rows).Error
	})
}

// ApplyTaskProjectScope intersects a task query with the selected project keys.
// Empty keys intentionally leave the query unchanged (all projects).
func ApplyTaskProjectScope(tx *gorm.DB, keys []string) *gorm.DB {
	keys = NormalizeProjectKeys(keys)
	if tx == nil || len(keys) == 0 {
		return tx
	}
	if tx.Migrator().HasColumn(&TaskTelemetry{}, "project_key") {
		clauses := []string{"UPPER(project_key) IN ?"}
		args := []interface{}{keys}
		for _, key := range keys {
			clauses = append(clauses, "((project_key IS NULL OR TRIM(project_key) = '') AND UPPER(task_id) LIKE ?)")
			args = append(args, key+"-%")
		}
		taskProjectCompatibilityFallbackCount.Add(1)
		return tx.Where("("+strings.Join(clauses, " OR ")+")", args...)
	}
	taskProjectCompatibilityFallbackCount.Add(1)
	return applyProjectColumnScope(tx, "task_id", keys)
}

// ApplyDemandProjectScope applies the same project-prefix rule to demand_id.
func ApplyDemandProjectScope(tx *gorm.DB, keys []string) *gorm.DB {
	return applyProjectColumnScope(tx, "demand_id", keys)
}

func applyProjectColumnScope(tx *gorm.DB, column string, keys []string) *gorm.DB {
	keys = NormalizeProjectKeys(keys)
	if tx == nil || len(keys) == 0 {
		return tx
	}

	clauses := make([]string, 0, len(keys))
	args := make([]interface{}, 0, len(keys))
	for _, key := range keys {
		clauses = append(clauses, "UPPER("+column+") LIKE ?")
		args = append(args, key+"-%")
	}
	return tx.Where("("+strings.Join(clauses, " OR ")+")", args...)
}

// TaskMatchesProjectScope reports whether a task belongs to a selected project.
// Empty keys intentionally match every task.
func TaskMatchesProjectScope(taskID string, keys []string) bool {
	return TaskMatchesExplicitProjectScope("", taskID, keys)
}

// TaskMatchesExplicitProjectScope prefers the persisted project fact and only
// falls back to the historical task-id prefix while migration is incomplete.
func TaskMatchesExplicitProjectScope(projectKey, taskID string, keys []string) bool {
	keys = NormalizeProjectKeys(keys)
	if len(keys) == 0 {
		return true
	}

	projectKey = strings.ToUpper(strings.TrimSpace(projectKey))
	if projectKey != "" {
		for _, key := range keys {
			if projectKey == key {
				return true
			}
		}
		return false
	}
	taskProjectCompatibilityFallbackCount.Add(1)
	taskID = strings.ToUpper(strings.TrimSpace(taskID))
	separator := strings.Index(taskID, "-")
	if separator <= 0 {
		return false
	}
	projectKey = taskID[:separator]
	for _, key := range keys {
		if projectKey == key {
			return true
		}
	}
	return false
}

func TaskProjectCompatibilityFallbackCount() uint64 {
	return taskProjectCompatibilityFallbackCount.Load()
}

func ResetTaskProjectCompatibilityFallbackCount() {
	taskProjectCompatibilityFallbackCount.Store(0)
}

// ResolveTaskProjectKey returns the persisted project fact. The task-id prefix
// remains a measured compatibility fallback until the migration report reaches
// zero unresolved production readers.
func ResolveTaskProjectKey(task TaskTelemetry) string {
	if projectKey := strings.ToUpper(strings.TrimSpace(task.ProjectKey)); projectKey != "" {
		return projectKey
	}
	taskProjectCompatibilityFallbackCount.Add(1)
	taskID := strings.ToUpper(strings.TrimSpace(task.TaskID))
	separator := strings.Index(taskID, "-")
	if separator <= 0 {
		return ""
	}
	return taskID[:separator]
}
