package jiraissuesync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/openaccess"
)

type Service struct {
	DB       *gorm.DB
	Repos    []config.RepoMapping
	Projects []string
	Now      func() time.Time
}
type Item struct {
	JiraKey         string    `json:"jira_key"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	SourceStatus    string    `json:"source_status"`
	SourceUpdatedAt time.Time `json:"source_updated_at"`
	DedupeKey       string    `json:"dedupe_key"`
	Action          string    `json:"action"`
}
type Preview struct {
	db.JiraIssueSyncPreview
	Mode           string   `json:"mode"`
	Items          []Item   `json:"items"`
	Truncated      bool     `json:"truncated"`
	RemoteVerified bool     `json:"remote_verified"`
	Limitations    []string `json:"limitations"`
}

func (s *Service) Settings(ctx context.Context) (db.JiraIssueSyncConfig, error) {
	initial := db.JiraIssueSyncConfig{ID: 1, Version: 1, TimeOfDay: "09:00"}
	if err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&initial).Error; err != nil {
		return initial, err
	}
	var out db.JiraIssueSyncConfig
	err := s.DB.WithContext(ctx).First(&out, 1).Error
	return out, err
}

func GroupForRepo(repo config.RepoMapping) string {
	path := strings.Trim(strings.TrimSpace(repo.Path), "/")
	i := strings.LastIndex(path, "/")
	if i <= 0 {
		return ""
	}
	return path[:i]
}

func (s *Service) Validate(v db.JiraIssueSyncConfig, requireReady bool) error {
	if _, err := time.Parse("15:04", v.TimeOfDay); err != nil || len(v.TimeOfDay) != 5 {
		return openaccess.NewError("invalid_query", "time_of_day must be HH:mm")
	}
	projectOK := false
	for _, key := range s.Projects {
		if key == v.JiraProject {
			projectOK = true
		}
	}
	if v.JiraProject != "" && !projectOK {
		return openaccess.NewError("invalid_query", "select a configured Jira project")
	}
	if len(v.GroupPath) > 255 || strings.ContainsAny(v.GroupPath, "?#\\") {
		return openaccess.NewError("invalid_query", "invalid group path")
	}
	repoOK := false
	for _, repo := range s.Repos {
		if repo.ProjectID == v.TargetProjectID && v.GroupPath != "" && GroupForRepo(repo) == v.GroupPath {
			repoOK = true
		}
	}
	if v.TargetProjectID != "" && !repoOK {
		return openaccess.NewError("invalid_query", "target repository must belong to the selected configured group")
	}
	if (requireReady || v.Enabled) && (!projectOK || !repoOK) {
		return openaccess.NewError("invalid_query", "select Jira project, group and Issue repository first")
	}
	return nil
}

func (s *Service) Save(ctx context.Context, v db.JiraIssueSyncConfig) (db.JiraIssueSyncConfig, error) {
	v.JiraProject = strings.ToUpper(strings.TrimSpace(v.JiraProject))
	v.GroupPath = strings.Trim(strings.TrimSpace(v.GroupPath), "/")
	if err := s.Validate(v, false); err != nil {
		return v, err
	}
	if _, err := s.Settings(ctx); err != nil {
		return v, err
	}
	result := s.DB.WithContext(ctx).Model(&db.JiraIssueSyncConfig{}).Where("id = 1 AND version = ?", v.Version).Updates(map[string]any{"version": v.Version + 1, "enabled": v.Enabled, "time_of_day": v.TimeOfDay, "jira_project": v.JiraProject, "group_path": v.GroupPath, "target_project_id": v.TargetProjectID, "updated_at": s.Now().UTC()})
	if result.Error != nil {
		return v, result.Error
	}
	if result.RowsAffected != 1 {
		return v, openaccess.NewError("conflict", "configuration changed; reload first")
	}
	return s.Settings(ctx)
}

func (s *Service) Generate(ctx context.Context, v db.JiraIssueSyncConfig, key string) (Preview, error) {
	out := Preview{Mode: "preview_only", Items: []Item{}, Limitations: []string{"local_synchronized_jira_only", "remote_issue_existence_not_checked", "no_remote_create_update_or_close", "first_100_items_only"}}
	if err := s.Validate(v, true); err != nil {
		return out, err
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Configuration is a precondition; never silently preview a superseded target.
		var live db.JiraIssueSyncConfig
		if err := tx.First(&live, 1).Error; err != nil {
			return err
		}
		if live.Version != v.Version {
			return openaccess.NewError("conflict", "configuration changed; reload first")
		}
		q := tx.Model(&db.TaskTelemetry{}).Where("project_key = ? AND (LOWER(source) = ? OR ((source IS NULL OR TRIM(source) = ?) AND task_id LIKE ?))", v.JiraProject, "jira", "", v.JiraProject+"-%")
		if err := q.Count(&out.Total).Error; err != nil {
			return err
		}
		var tasks []db.TaskTelemetry
		if err := q.Order("task_id").Limit(100).Find(&tasks).Error; err != nil {
			return err
		}
		for _, task := range tasks {
			keyBytes := sha256.Sum256([]byte(v.TargetProjectID + ":" + v.JiraProject + ":" + task.TaskID))
			dedupe := hex.EncodeToString(keyBytes[:])
			text := fmt.Sprintf("Jira: %s\nStatus: %s\n\n%s\n\n<!-- well-ambient:jira:%s -->", task.TaskID, task.Status, clip(task.Description, 8000), dedupe)
			out.Items = append(out.Items, Item{JiraKey: task.TaskID, Title: clip("["+task.TaskID+"] "+task.Title, 240), Description: text, SourceStatus: task.Status, SourceUpdatedAt: task.SourceUpdatedAt, DedupeKey: dedupe, Action: "pending_remote_comparison"})
		}
		encoded, err := json.Marshal(out.Items)
		if err != nil {
			return err
		}
		out.JiraIssueSyncPreview.ConfigVersion = v.Version
		out.JiraIssueSyncPreview.JiraProject = v.JiraProject
		out.JiraIssueSyncPreview.TargetProjectID = v.TargetProjectID
		out.JiraIssueSyncPreview.GroupPath = v.GroupPath
		out.ItemsJSON = string(encoded)
		out.CreatedAt = s.Now().UTC()
		out.RunKey = key
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&out.JiraIssueSyncPreview)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			if err := tx.Where("run_key = ?", key).First(&out.JiraIssueSyncPreview).Error; err != nil {
				return err
			}
			if err := json.Unmarshal([]byte(out.ItemsJSON), &out.Items); err != nil {
				return err
			}
		}
		if strings.HasPrefix(key, "daily:") {
			if err := tx.Model(&db.JiraIssueSyncConfig{}).Where("id = 1 AND version = ?", v.Version).Update("last_daily_key", key).Error; err != nil {
				return err
			}
		}
		// Retain a bounded audit history. Scheduler state survives this cleanup.
		// No remote APIs exist in this package.
		var old []uint
		if err := tx.Model(&db.JiraIssueSyncPreview{}).Order("id DESC").Offset(30).Pluck("id", &old).Error; err != nil {
			return err
		}
		if len(old) > 0 {
			return tx.Delete(&db.JiraIssueSyncPreview{}, old).Error
		}
		return nil
	})
	out.Truncated = out.Total > int64(len(out.Items))
	return out, err
}

func (s *Service) Tick(ctx context.Context) error {
	v, err := s.Settings(ctx)
	if err != nil || !v.Enabled {
		return err
	}
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := s.Now().In(location)
	if now.Format("15:04") < v.TimeOfDay {
		return nil
	}
	key := fmt.Sprintf("daily:%s:v%d", now.Format("2006-01-02"), v.Version)
	if v.LastDailyKey == key {
		return nil
	}
	var count int64
	if err := s.DB.WithContext(ctx).Model(&db.JiraIssueSyncPreview{}).Where("run_key = ?", key).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err = s.Generate(ctx, v, key)
	return err
}

func (s *Service) Latest(ctx context.Context) (*Preview, error) {
	var row db.JiraIssueSyncPreview
	if err := s.DB.WithContext(ctx).Order("id DESC").First(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	out := Preview{JiraIssueSyncPreview: row, Mode: "preview_only", Items: []Item{}, Limitations: []string{"local_synchronized_jira_only", "remote_issue_existence_not_checked", "no_remote_create_update_or_close", "first_100_items_only"}}
	if err := json.Unmarshal([]byte(row.ItemsJSON), &out.Items); err != nil {
		return nil, err
	}
	out.Truncated = row.Total > int64(len(out.Items))
	return &out, nil
}

func clip(text string, n int) string {
	r := []rune(text)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return text
}
