package db

import "gorm.io/gorm"

// allPageReadIndexes backs the bounded keyset and page-local aggregate readers.
// The list is deliberately kept beside database migration ownership instead of
// being scattered through HTTP handlers. Every index corresponds to a concrete
// ORDER BY / WHERE shape used by a generation-bound read contract.
var allPageReadIndexes = []string{
	`CREATE INDEX IF NOT EXISTS idx_context_facts_status_updated_id
		ON context_facts (status, updated_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_context_facts_updated_id
		ON context_facts (updated_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_context_documents_status_updated_id
		ON context_documents (status, updated_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_context_documents_updated_id
		ON context_documents (updated_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_corpus_candidates_status_created_id
		ON corpus_candidates (status, created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_corpus_candidates_created_id
		ON corpus_candidates (created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_corpus_candidates_document_status
		ON corpus_candidates (context_document_id, status)`,
	`CREATE INDEX IF NOT EXISTS idx_corpus_candidates_document_created_id
		ON corpus_candidates (context_document_id, created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_execution_runs_demand_created_id
		ON execution_runs (demand_id, created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_execution_runs_created_id
		ON execution_runs (created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_execution_actions_run_created_id
		ON execution_actions (execution_run_id, created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_release_versions_page
		ON release_versions (COALESCE(release_date, '9999-12-31T23:59:59Z'), name, id)
		WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_release_versions_project_status_page
		ON release_versions (project_key, status, COALESCE(release_date, '9999-12-31T23:59:59Z'), name, id)
		WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_release_jira_issue_page
		ON task_telemetries (project_key, last_update DESC, task_id)
		WHERE source = 'jira' AND LOWER(TRIM(issue_type)) IN
			('demand','requirement','story','bug','defect','缺陷','故障')`,
	`CREATE INDEX IF NOT EXISTS idx_jira_comment_task_current_created_id
		ON jira_comment_logs (LOWER(task_id), current, created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_git_activity_normalized_task_created_id
		ON git_commit_logs (LOWER(task_id), created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_users_created_id
		ON users (created_at, id)`,
	`CREATE INDEX IF NOT EXISTS idx_task_assignee_directory
		ON task_telemetries (assignee)
		WHERE TRIM(assignee) <> ''`,
	`CREATE INDEX IF NOT EXISTS idx_performance_work_item_timeline
		ON performance_work_item_events (work_item_id, occurred_at, id)`,
}

func MigrateAllPageReadIndexes(conn *gorm.DB) error {
	if conn == nil {
		return gorm.ErrInvalidDB
	}
	for _, statement := range allPageReadIndexes {
		if err := conn.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

// postgresReadIndexes contains only PostgreSQL-specific indexes backed by
// concrete production query shapes. Keep these separate from the portable
// SQLite indexes so local tests do not mask dialect-specific DDL mistakes.
var postgresReadIndexes = []string{
	`CREATE INDEX IF NOT EXISTS idx_pg_task_active_project_created
		ON task_telemetries (project_key, task_created_at, task_id)
		INCLUDE (assignee, due_date, last_update, source_updated_at)
		WHERE status IS NULL OR LOWER(BTRIM(status)) NOT IN
			('done','closed','resolved','completed','archived','已完成','已关闭')`,
	`CREATE INDEX IF NOT EXISTS idx_pg_task_active_assignee_created
		ON task_telemetries (LOWER(BTRIM(assignee)), task_created_at, task_id)
		INCLUDE (project_key, due_date, last_update, source_updated_at)
		WHERE status IS NULL OR LOWER(BTRIM(status)) NOT IN
			('done','closed','resolved','completed','archived','已完成','已关闭')`,
	`CREATE INDEX IF NOT EXISTS idx_pg_solution_catalog_project_page
		ON solution_catalog_entries (project_key, published_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_pg_solution_catalog_page
		ON solution_catalog_entries (published_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_pg_solution_catalog_token_entry
		ON solution_catalog_search_tokens (token, entry_id)`,
	`CREATE INDEX IF NOT EXISTS idx_pg_solution_revision_asset_version
		ON solution_revisions (solution_asset_id, version DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_pg_solution_source_asset_observed
		ON solution_source_refs (solution_asset_id, observed_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_pg_solution_job_asset_id
		ON solution_polish_jobs (solution_asset_id, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_pg_solution_comparison_left_recall
		ON solution_comparisons (left_entry_id, recall_score DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_pg_solution_comparison_right_recall
		ON solution_comparisons (right_entry_id, recall_score DESC, id DESC)`,
}

func MigratePostgresReadIndexes(conn *gorm.DB) error {
	if conn == nil {
		return gorm.ErrInvalidDB
	}
	if conn.Dialector.Name() != "postgres" {
		return nil
	}
	for _, statement := range postgresReadIndexes {
		if err := conn.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}
