package dailyjira

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// postgresDailyJiraCandidates keeps the PostgreSQL adapter behind the same
// Reader interface as the SQLite projection. PostgreSQL derives time buckets
// at transaction start, while the shared task_facts generation protects
// cursors from concurrent writes.
const postgresDailyJiraCandidates = `WITH candidates AS (
	SELECT
		task_id,
		COALESCE(title, '') AS title,
		UPPER(TRIM(COALESCE(NULLIF(project_key, ''), split_part(task_id, '-', 1)))) AS project_key,
		COALESCE(repo, '') AS project,
		COALESCE(assignee, '') AS assignee,
		LOWER(TRIM(COALESCE(assignee, ''))) AS assignee_key,
		COALESCE(jira_reporter, '') AS reporter,
		COALESCE(status, '') AS status,
		COALESCE(issue_type, '') AS issue_type,
		task_created_at,
		CASE WHEN EXTRACT(YEAR FROM task_created_at) > 1 THEN EXTRACT(EPOCH FROM task_created_at)::BIGINT ELSE 0 END AS created_unix,
		CASE WHEN EXTRACT(YEAR FROM source_updated_at) > 1 THEN source_updated_at ELSE last_update END AS last_activity_at,
		CASE
			WHEN EXTRACT(YEAR FROM source_updated_at) > 1 THEN EXTRACT(EPOCH FROM source_updated_at)::BIGINT
			WHEN EXTRACT(YEAR FROM last_update) > 1 THEN EXTRACT(EPOCH FROM last_update)::BIGINT
			ELSE 0
		END AS activity_unix,
		due_date,
		COALESCE(decision_logs, '') AS decision_logs,
		CASE
			WHEN EXTRACT(YEAR FROM task_created_at) <= 1 OR CURRENT_DATE - task_created_at::date < 0 THEN 'unclassified'
			WHEN CURRENT_DATE - task_created_at::date = 0 THEN 'today'
			WHEN CURRENT_DATE - task_created_at::date BETWEEN 1 AND 2 THEN 'recent_watch'
			WHEN CURRENT_DATE - task_created_at::date BETWEEN 3 AND 6 THEN 'three_day'
			ELSE 'seven_day'
		END AS bucket,
		COALESCE(EXTRACT(YEAR FROM due_date) > 1 AND due_date::date < CURRENT_DATE, FALSE) AS overdue,
		CASE WHEN COALESCE(EXTRACT(YEAR FROM due_date) > 1 AND due_date::date < CURRENT_DATE, FALSE) THEN 0 ELSE 1 END AS sort_overdue
	FROM task_telemetries
	WHERE (
		LOWER(TRIM(COALESCE(source, ''))) = 'jira'
		OR (
			TRIM(COALESCE(source, '')) = ''
			AND task_id ~ '^[A-Z][A-Z0-9_]*-[0-9]+$'
			AND task_id NOT LIKE 'TASK-%'
			AND task_id NOT LIKE 'DEMAND-%'
			AND UPPER(COALESCE(repo, '')) LIKE '%(' || split_part(task_id, '-', 1) || ')%'
		)
	)
	AND (status IS NULL OR LOWER(TRIM(status)) NOT IN
		('done','closed','resolved','completed','archived','已完成','已关闭'))
) `

func readPostgresItems(tx *gorm.DB, query Query, cursor *pageCursor) ([]Item, error) {
	var builder strings.Builder
	builder.WriteString(postgresDailyJiraCandidates)
	builder.WriteString(`SELECT task_id, title, project_key, project, assignee, reporter, status, issue_type,
		task_created_at, last_activity_at, due_date, decision_logs, bucket, overdue, created_unix, activity_unix
		FROM candidates WHERE bucket = ?`)
	args := []any{string(query.Bucket)}
	appendScopePredicate(&builder, &args, query.Scope, "")
	if search := strings.TrimSpace(query.Search); search != "" {
		builder.WriteString(` AND (task_id ILIKE ? OR title ILIKE ? OR project_key ILIKE ? OR project ILIKE ? OR assignee ILIKE ? OR status ILIKE ?)`)
		prefix := search + "%"
		args = append(args, prefix, prefix, prefix, prefix, prefix, prefix)
	}
	if cursor != nil {
		operator := ">"
		if query.Direction == DirectionPrevious {
			operator = "<"
		}
		builder.WriteString(" AND (sort_overdue, created_unix, activity_unix, task_id) " + operator + " (?, ?, ?, ?)")
		args = append(args, cursor.SortOverdue, cursor.CreatedUnix, cursor.ActivityUnix, cursor.TaskID)
	}
	order := "ASC"
	if query.Direction == DirectionPrevious {
		order = "DESC"
	}
	builder.WriteString(" ORDER BY sort_overdue " + order + ", created_unix " + order + ", activity_unix " + order + ", task_id " + order + " LIMIT ?")
	args = append(args, query.Limit+1)

	var items []Item
	if err := tx.Raw(builder.String(), args...).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("read PostgreSQL Daily Jira page: %w", err)
	}
	return items, nil
}

func readPostgresSummary(tx *gorm.DB, scope Scope) (Summary, error) {
	var builder strings.Builder
	builder.WriteString(postgresDailyJiraCandidates)
	builder.WriteString("SELECT bucket, COUNT(*) AS item_count FROM candidates WHERE 1 = 1")
	args := make([]any, 0, 8)
	appendScopePredicate(&builder, &args, scope, "")
	builder.WriteString(" GROUP BY bucket")

	var rows []struct {
		Bucket    Bucket `gorm:"column:bucket"`
		ItemCount int64  `gorm:"column:item_count"`
	}
	if err := tx.Raw(builder.String(), args...).Scan(&rows).Error; err != nil {
		return Summary{}, fmt.Errorf("read PostgreSQL Daily Jira summary: %w", err)
	}
	var summary Summary
	for _, row := range rows {
		switch row.Bucket {
		case BucketToday:
			summary.Today = row.ItemCount
		case BucketRecentWatch:
			summary.RecentWatch = row.ItemCount
		case BucketThreeDay:
			summary.ThreeDay = row.ItemCount
		case BucketSevenDay:
			summary.SevenDay = row.ItemCount
		case BucketUnclassified:
			summary.Unclassified = row.ItemCount
		}
	}
	summary.Total = summary.Today + summary.ThreeDay + summary.SevenDay
	return summary, nil
}
