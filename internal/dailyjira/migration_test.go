package dailyjira

import (
	"fmt"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigrateUpgradesLegacySortAndSearchIndexes(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	statements := []string{
		`CREATE TABLE daily_jira_audit_state (
			id INTEGER PRIMARY KEY, schema_version INTEGER NOT NULL, backfilled INTEGER NOT NULL,
			generation INTEGER NOT NULL, search_mode TEXT NOT NULL, rollover_day INTEGER NOT NULL,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`INSERT INTO daily_jira_audit_state
			(id, schema_version, backfilled, generation, search_mode, rollover_day)
			VALUES (1, 2, 1, 7, 'prefix', CAST(julianday(date('now', 'localtime')) AS INTEGER))`,
		strings.Replace(readModelSchemaStatements()[2], "\n\t\t\tsort_overdue INTEGER NOT NULL DEFAULT 1,", "", 1),
		readModelSchemaStatements()[3],
		`CREATE INDEX idx_daily_jira_page
			ON daily_jira_audit_entries (bucket, overdue DESC, created_unix, activity_unix, task_id)`,
		`CREATE INDEX idx_daily_jira_short_task_search ON daily_jira_audit_entries (task_id)`,
		`CREATE TABLE task_telemetries (
			task_id TEXT PRIMARY KEY, project_key TEXT, source TEXT, title TEXT, repo TEXT,
			assignee TEXT, jira_reporter TEXT, status TEXT, issue_type TEXT,
			task_created_at DATETIME, last_update DATETIME, source_updated_at DATETIME,
			due_date DATETIME, decision_logs TEXT
		)`,
		`INSERT INTO daily_jira_audit_entries
			(task_id, bucket, overdue, created_unix, activity_unix) VALUES ('HIT-1', 'seven_day', 1, 1, 1)`,
	}
	for _, statement := range statements {
		if err := conn.Exec(statement).Error; err != nil {
			t.Fatalf("prepare legacy schema: %v\n%s", err, statement)
		}
	}

	if err := Migrate(conn); err != nil {
		t.Fatalf("upgrade legacy read model: %v", err)
	}
	var state struct {
		SchemaVersion  int    `gorm:"column:schema_version"`
		Generation     int64  `gorm:"column:generation"`
		SortOverdue    int    `gorm:"column:sort_overdue"`
		PageIndexSQL   string `gorm:"column:page_index_sql"`
		TaskIndexSQL   string `gorm:"column:task_index_sql"`
		TaskTriggerSQL string `gorm:"column:task_trigger_sql"`
	}
	if err := conn.Raw(`SELECT
		(SELECT schema_version FROM daily_jira_audit_state WHERE id = 1) AS schema_version,
		(SELECT generation FROM daily_jira_audit_state WHERE id = 1) AS generation,
		(SELECT sort_overdue FROM daily_jira_audit_entries WHERE task_id = 'HIT-1') AS sort_overdue,
		(SELECT sql FROM sqlite_master WHERE type = 'index' AND name = 'idx_daily_jira_page') AS page_index_sql,
		(SELECT sql FROM sqlite_master WHERE type = 'index' AND name = 'idx_daily_jira_short_task_search') AS task_index_sql,
		(SELECT sql FROM sqlite_master WHERE type = 'trigger' AND name = 'daily_jira_tasks_ai') AS task_trigger_sql`).Scan(&state).Error; err != nil {
		t.Fatalf("inspect upgraded read model: %v", err)
	}
	if state.SchemaVersion != readModelSchemaVersion || state.Generation != 8 || state.SortOverdue != 0 {
		t.Fatalf("upgraded state = %+v", state)
	}
	if !strings.Contains(state.PageIndexSQL, "sort_overdue") {
		t.Fatalf("page index was not replaced: %s", state.PageIndexSQL)
	}
	if !strings.Contains(state.TaskIndexSQL, "bucket, task_id") {
		t.Fatalf("task search index was not bucket-scoped: %s", state.TaskIndexSQL)
	}
	if !strings.Contains(state.TaskTriggerSQL, "sort_overdue") {
		t.Fatalf("task projection trigger was not upgraded: %s", state.TaskTriggerSQL)
	}
}

func TestMigrateReplacesExistingTriggerDefinitions(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open trigger replacement database: %v", err)
	}
	if err := conn.Exec(`CREATE TABLE task_telemetries (
		task_id TEXT PRIMARY KEY, project_key TEXT, source TEXT, title TEXT, repo TEXT,
		assignee TEXT, jira_reporter TEXT, status TEXT, issue_type TEXT,
		task_created_at DATETIME, last_update DATETIME, source_updated_at DATETIME,
		due_date DATETIME, decision_logs TEXT
	)`).Error; err != nil {
		t.Fatalf("create task source: %v", err)
	}
	if err := Migrate(conn); err != nil {
		t.Fatalf("install current read model: %v", err)
	}
	if err := conn.Exec("DROP TRIGGER daily_jira_tasks_au").Error; err != nil {
		t.Fatalf("drop current update trigger: %v", err)
	}
	if err := conn.Exec(`CREATE TRIGGER daily_jira_tasks_au AFTER UPDATE ON task_telemetries BEGIN
		UPDATE daily_jira_audit_state SET generation = generation + 100 WHERE id = 1;
	END`).Error; err != nil {
		t.Fatalf("install stale update trigger: %v", err)
	}

	if err := Migrate(conn); err != nil {
		t.Fatalf("rerun read-model migration: %v", err)
	}
	var triggerSQL string
	if err := conn.Raw(`SELECT sql FROM sqlite_master
		WHERE type = 'trigger' AND name = 'daily_jira_tasks_au'`).Scan(&triggerSQL).Error; err != nil {
		t.Fatalf("read replaced trigger: %v", err)
	}
	if !strings.Contains(triggerSQL, "AFTER UPDATE OF") || !strings.Contains(triggerSQL, "sort_overdue") {
		t.Fatalf("stale trigger definition survived rerun: %s", triggerSQL)
	}
}
