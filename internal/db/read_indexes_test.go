package db

import (
	"strings"
	"testing"
	"time"

	userdb "well-ambient/internal/db/user"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAllPageReadIndexesBackBoundedKeysetQueries(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open("file:all_page_read_indexes?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.AutoMigrate(
		&ContextFact{}, &ContextDocument{}, &CorpusCandidate{},
		&ExecutionRun{}, &ExecutionAction{}, &ReleaseVersion{}, &TaskTelemetry{},
		&GitCommitLog{}, &JiraCommentLog{}, &userdb.User{}, &PerformanceWorkItemEvent{},
	); err != nil {
		t.Fatal(err)
	}
	if err := MigrateAllPageReadIndexes(conn); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 21, 16, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		sql       string
		args      []any
		wantIndex string
	}{
		{
			name: "context facts", wantIndex: "idx_context_facts_status_updated_id",
			sql: `SELECT id FROM context_facts
				WHERE status = ? AND (updated_at < ? OR (updated_at = ? AND id < ?))
				ORDER BY updated_at DESC, id DESC LIMIT 51`,
			args: []any{"active", now, now, 100},
		},
		{
			name: "context documents", wantIndex: "idx_context_documents_updated_id",
			sql: `SELECT id FROM context_documents
				WHERE updated_at < ? OR (updated_at = ? AND id < ?)
				ORDER BY updated_at DESC, id DESC LIMIT 51`,
			args: []any{now, now, 100},
		},
		{
			name: "corpus candidates", wantIndex: "idx_corpus_candidates_status_created_id",
			sql: `SELECT id FROM corpus_candidates
				WHERE status = ? AND (created_at < ? OR (created_at = ? AND id < ?))
				ORDER BY created_at DESC, id DESC LIMIT 51`,
			args: []any{"pending", now, now, 100},
		},
		{
			name: "execution runs", wantIndex: "idx_execution_runs_demand_created_id",
			sql: `SELECT id FROM execution_runs
				WHERE demand_id = ? AND (created_at < ? OR (created_at = ? AND id < ?))
				ORDER BY created_at DESC, id DESC LIMIT 51`,
			args: []any{"FZ-2257", now, now, 100},
		},
		{
			name: "execution actions", wantIndex: "idx_execution_actions_run_created_id",
			sql: `SELECT id FROM execution_actions
				WHERE execution_run_id = ? ORDER BY created_at DESC, id DESC LIMIT 20`,
			args: []any{1},
		},
		{
			name: "release page", wantIndex: "idx_release_versions_project_status_page",
			sql: `SELECT id FROM release_versions
				WHERE deleted_at IS NULL AND project_key = ? AND status = ?
				ORDER BY COALESCE(release_date, '9999-12-31T23:59:59Z'), name, id LIMIT 51`,
			args: []any{"FZ", "planned"},
		},
		{
			name: "release Jira issue page", wantIndex: "idx_release_jira_issue_page",
			sql: `SELECT task_id FROM task_telemetries
				WHERE source = 'jira' AND project_key = ?
				AND LOWER(TRIM(issue_type)) IN ('demand','requirement','story','bug','defect','缺陷','故障')
				ORDER BY last_update DESC, task_id ASC LIMIT 51`,
			args: []any{"FZ"},
		},
		{
			name: "Jira activity timeline", wantIndex: "idx_jira_comment_task_current_created_id",
			sql: `SELECT id FROM jira_comment_logs
				WHERE LOWER(task_id) = LOWER(?) AND current = ?
				ORDER BY created_at DESC, id DESC LIMIT 100`,
			args: []any{"FZ-2257", true},
		},
		{
			name: "Git activity timeline", wantIndex: "idx_git_activity_normalized_task_created_id",
			sql: `SELECT id FROM git_commit_logs
				WHERE LOWER(task_id) = LOWER(?)
				ORDER BY created_at DESC, id DESC LIMIT 100`,
			args: []any{"FZ-2257"},
		},
		{
			name: "user directory", wantIndex: "idx_users_created_id",
			sql: `SELECT id FROM users ORDER BY created_at, id LIMIT 5000`,
		},
		{
			name: "task assignee directory", wantIndex: "idx_task_assignee_directory",
			sql: `SELECT DISTINCT assignee FROM task_telemetries
				WHERE TRIM(assignee) <> '' ORDER BY assignee LIMIT 5000`,
		},
		{
			name: "performance work item timeline", wantIndex: "idx_performance_work_item_timeline",
			sql: `SELECT id FROM performance_work_item_events
				WHERE work_item_id IN (?, ?) AND occurred_at <= ?
				ORDER BY work_item_id, occurred_at, id`,
			args: []any{"FZ-2257", "NS2-1986", now},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var plan []struct{ Detail string }
			if err := conn.Raw("EXPLAIN QUERY PLAN "+testCase.sql, testCase.args...).Scan(&plan).Error; err != nil {
				t.Fatal(err)
			}
			joined := ""
			for _, row := range plan {
				joined += row.Detail + "\n"
			}
			if !strings.Contains(joined, testCase.wantIndex) {
				t.Fatalf("query plan does not use %s:\n%s", testCase.wantIndex, joined)
			}
			if strings.Contains(strings.ToUpper(joined), "USE TEMP B-TREE FOR ORDER BY") {
				t.Fatalf("query plan sorts outside the index:\n%s", joined)
			}
		})
	}
}

func TestPostgresReadIndexesMatchAuditedQueryLeadingColumns(t *testing.T) {
	joined := strings.Join(postgresReadIndexes, "\n")
	for _, fragment := range []string{
		"(project_key, task_created_at, task_id)",
		"(LOWER(BTRIM(assignee)), task_created_at, task_id)",
		"INCLUDE (assignee, due_date, last_update, source_updated_at)",
		"WHERE status IS NULL OR LOWER(BTRIM(status)) NOT IN",
		"(project_key, published_at DESC, id DESC)",
		"(token, entry_id)",
		"(solution_asset_id, version DESC)",
		"(left_entry_id, recall_score DESC, id DESC)",
		"(right_entry_id, recall_score DESC, id DESC)",
	} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("PostgreSQL index contract missing %q:\n%s", fragment, joined)
		}
	}
}

func TestPostgresReadIndexesAreSkippedForSQLite(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open("file:postgres_index_skip?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigratePostgresReadIndexes(conn); err != nil {
		t.Fatalf("PostgreSQL-only index migration should be a no-op on SQLite: %v", err)
	}
}
