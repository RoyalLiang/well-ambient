package deliveryplanning

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"well-ambient/internal/db"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	conn *gorm.DB
}

func NewRepository(conn *gorm.DB) *Repository {
	return &Repository{conn: conn}
}

func (r *Repository) DB() *gorm.DB {
	return r.conn
}

func (r *Repository) ListReleases(ctx context.Context, projectKey string) ([]db.ReleaseVersion, error) {
	if r == nil || r.conn == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	projectKey = NormalizeProjectKey(projectKey)
	var releases []db.ReleaseVersion
	query := r.conn.WithContext(ctx).Order("release_date IS NULL, release_date ASC, name ASC")
	if projectKey != "" {
		query = query.Where("UPPER(project_key) = ?", projectKey)
	}
	if err := query.Find(&releases).Error; err != nil {
		return nil, err
	}
	return releases, nil
}

func (r *Repository) GetRelease(ctx context.Context, id uint) (db.ReleaseVersion, error) {
	if r == nil || r.conn == nil {
		return db.ReleaseVersion{}, fmt.Errorf("database is not initialized")
	}
	var release db.ReleaseVersion
	if err := r.conn.WithContext(ctx).First(&release, id).Error; err != nil {
		return db.ReleaseVersion{}, err
	}
	return release, nil
}

func (r *Repository) UpsertExternalReleases(ctx context.Context, releases []ExternalRelease, syncedAt time.Time) ([]db.ReleaseVersion, error) {
	if r == nil || r.conn == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	if len(releases) == 0 {
		return []db.ReleaseVersion{}, nil
	}
	result := make([]db.ReleaseVersion, 0, len(releases))
	err := r.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, external := range releases {
			projectKey := NormalizeProjectKey(external.ProjectKey)
			externalID := strings.TrimSpace(external.ExternalID)
			if projectKey == "" || externalID == "" || strings.TrimSpace(external.Name) == "" {
				return fmt.Errorf("external release requires project, external id, and name")
			}
			status := strings.ToLower(strings.TrimSpace(external.Status))
			if status == "" {
				status = ReleasePlanned
			}
			row := db.ReleaseVersion{
				ProjectKey:  projectKey,
				Source:      "jira",
				ExternalID:  externalID,
				Name:        strings.TrimSpace(external.Name),
				Description: external.Description,
				Status:      status,
				StartDate:   external.StartDate,
				ReleaseDate: external.ReleaseDate,
				SourceURL:   external.SourceURL,
				SyncedAt:    &syncedAt,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "project_key"},
					{Name: "source"},
					{Name: "external_id"},
				},
				DoUpdates: clause.AssignmentColumns([]string{
					"name", "description", "status", "start_date", "release_date",
					"source_url", "synced_at", "updated_at",
				}),
			}).Create(&row).Error; err != nil {
				return err
			}
			if err := tx.Where(
				"project_key = ? AND source = ? AND external_id = ?",
				projectKey, "jira", externalID,
			).First(&row).Error; err != nil {
				return err
			}
			result = append(result, row)
		}
		return nil
	})
	return result, err
}

func (r *Repository) ReleaseSnapshot(ctx context.Context, releaseID uint) (ReleaseSnapshot, error) {
	release, err := r.GetRelease(ctx, releaseID)
	if err != nil {
		return ReleaseSnapshot{}, err
	}
	type aggregate struct {
		Total         int64
		Completed     int64
		EstimatedDays float64
	}
	var stats aggregate
	err = r.conn.WithContext(ctx).Table("work_item_release_links AS links").
		Select(`COUNT(DISTINCT links.work_item_id) AS total,
			COUNT(DISTINCT CASE WHEN tasks.status IN ('done', 'closed', 'resolved') THEN links.work_item_id END) AS completed,
			COALESCE(SUM(CASE WHEN links.is_primary = 1 THEN tasks.estimate_days ELSE 0 END), 0) AS estimated_days`).
		Joins("JOIN task_telemetries AS tasks ON tasks.task_id = links.work_item_id").
		Where("links.release_version_id = ? AND links.active = ? AND links.relation = ?", releaseID, true, ReleaseTargetFix).
		Scan(&stats).Error
	if err != nil {
		return ReleaseSnapshot{}, err
	}
	snapshot := ReleaseSnapshot{
		Release:        release,
		WorkItemCount:  stats.Total,
		CompletedCount: stats.Completed,
		OpenCount:      stats.Total - stats.Completed,
		EstimatedDays:  stats.EstimatedDays,
	}
	if stats.Total > 0 {
		snapshot.CompletedPercent = float64(stats.Completed) / float64(stats.Total) * 100
	}
	return snapshot, nil
}

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
