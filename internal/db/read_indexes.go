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
