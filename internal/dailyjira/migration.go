package dailyjira

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const readModelSchemaVersion = 3

func Migrate(conn *gorm.DB) error {
	if conn == nil {
		return fmt.Errorf("Daily Jira read model database is not initialized")
	}
	if conn.Dialector.Name() == "postgres" {
		// PostgreSQL derives the projection from the indexed task fact table and
		// uses the shared task_facts generation maintained by readmodel.
		return nil
	}
	if conn.Dialector.Name() != "sqlite" {
		return fmt.Errorf("unsupported Daily Jira database driver %q", conn.Dialector.Name())
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		statements := readModelSchemaStatements()
		for _, statement := range statements[:4] {
			if err := tx.Exec(statement).Error; err != nil {
				return fmt.Errorf("migrate Daily Jira read model: %w", err)
			}
		}
		if err := migrateSortOverdueKey(tx); err != nil {
			return err
		}
		var ftsEnabled int
		if err := tx.Raw("SELECT sqlite_compileoption_used('ENABLE_FTS5')").Scan(&ftsEnabled).Error; err != nil {
			return fmt.Errorf("detect Daily Jira FTS capability: %w", err)
		}
		searchMode := "prefix"
		if ftsEnabled == 1 {
			if err := tx.Exec(readModelFTSSchemaStatement()).Error; err != nil {
				return fmt.Errorf("migrate Daily Jira FTS search: %w", err)
			}
			searchMode = "fts5_trigram"
		}
		if err := tx.Exec("UPDATE daily_jira_audit_state SET search_mode = ? WHERE id = 1", searchMode).Error; err != nil {
			return fmt.Errorf("record Daily Jira search mode: %w", err)
		}

		var backfilled int
		if err := tx.Raw("SELECT backfilled FROM daily_jira_audit_state WHERE id = 1").Scan(&backfilled).Error; err != nil {
			return fmt.Errorf("read Daily Jira backfill state: %w", err)
		}
		if backfilled == 0 {
			if err := tx.Exec(backfillEntriesSQL()).Error; err != nil {
				return fmt.Errorf("backfill Daily Jira read entries: %w", err)
			}
			if err := tx.Exec(`INSERT INTO daily_jira_audit_counts (project_key, assignee_key, bucket, item_count)
				SELECT project_key, assignee_key, bucket, COUNT(*)
				FROM daily_jira_audit_entries
				GROUP BY project_key, assignee_key, bucket`).Error; err != nil {
				return fmt.Errorf("backfill Daily Jira counters: %w", err)
			}
			if ftsEnabled == 1 {
				if err := tx.Exec("INSERT INTO daily_jira_audit_search(daily_jira_audit_search) VALUES('rebuild')").Error; err != nil {
					return fmt.Errorf("backfill Daily Jira search: %w", err)
				}
			}
			if err := tx.Exec(`UPDATE daily_jira_audit_state
				SET schema_version = ?, backfilled = 1, generation = generation + 1, updated_at = CURRENT_TIMESTAMP
				WHERE id = 1`, readModelSchemaVersion).Error; err != nil {
				return fmt.Errorf("finish Daily Jira read backfill: %w", err)
			}
		}
		// Build secondary indexes after the initial bulk materialization. Maintaining
		// every index row-by-row makes a first 10M-row migration substantially slower.
		for _, statement := range statements[4:] {
			if err := tx.Exec(statement).Error; err != nil {
				return fmt.Errorf("index Daily Jira read model: %w", err)
			}
		}

		for _, triggerName := range readModelTriggerNames() {
			if err := tx.Exec("DROP TRIGGER IF EXISTS " + triggerName).Error; err != nil {
				return fmt.Errorf("replace Daily Jira trigger %s: %w", triggerName, err)
			}
		}
		for _, statement := range readModelTriggerStatements(ftsEnabled == 1) {
			if err := tx.Exec(statement).Error; err != nil {
				return fmt.Errorf("install Daily Jira read trigger: %w", err)
			}
		}
		return nil
	})
}

func migrateSortOverdueKey(tx *gorm.DB) error {
	var columns []struct {
		Name string `gorm:"column:name"`
	}
	if err := tx.Raw("SELECT name FROM pragma_table_info('daily_jira_audit_entries')").Scan(&columns).Error; err != nil {
		return fmt.Errorf("inspect Daily Jira sort key: %w", err)
	}
	hasSortKey := false
	for _, column := range columns {
		if column.Name == "sort_overdue" {
			hasSortKey = true
			break
		}
	}
	if !hasSortKey {
		if err := tx.Exec("ALTER TABLE daily_jira_audit_entries ADD COLUMN sort_overdue INTEGER NOT NULL DEFAULT 1").Error; err != nil {
			return fmt.Errorf("add Daily Jira monotonic sort key: %w", err)
		}
	}

	var schemaVersion int
	if err := tx.Raw("SELECT schema_version FROM daily_jira_audit_state WHERE id = 1").Scan(&schemaVersion).Error; err != nil {
		return fmt.Errorf("read Daily Jira read-model schema version: %w", err)
	}
	if schemaVersion >= readModelSchemaVersion {
		return nil
	}
	if err := tx.Exec("UPDATE daily_jira_audit_entries SET sort_overdue = CASE WHEN overdue = 1 THEN 0 ELSE 1 END").Error; err != nil {
		return fmt.Errorf("backfill Daily Jira monotonic sort key: %w", err)
	}
	for _, indexName := range []string{
		"idx_daily_jira_page",
		"idx_daily_jira_project_page",
		"idx_daily_jira_assignee_page",
		"idx_daily_jira_project_assignee_page",
		"idx_daily_jira_short_task_search",
		"idx_daily_jira_short_title_search",
		"idx_daily_jira_short_project_search",
		"idx_daily_jira_short_assignee_search",
		"idx_daily_jira_short_status_search",
	} {
		if err := tx.Exec("DROP INDEX IF EXISTS " + indexName).Error; err != nil {
			return fmt.Errorf("replace Daily Jira page index %s: %w", indexName, err)
		}
	}
	if err := tx.Exec(`UPDATE daily_jira_audit_state
		SET schema_version = ?, generation = generation + 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = 1`, readModelSchemaVersion).Error; err != nil {
		return fmt.Errorf("record Daily Jira read-model schema version: %w", err)
	}
	return nil
}

func readModelTriggerNames() []string {
	return []string{
		"daily_jira_entries_ai",
		"daily_jira_entries_ad",
		"daily_jira_entries_au_count",
		"daily_jira_tasks_ai",
		"daily_jira_tasks_au",
		"daily_jira_tasks_ad",
		"daily_jira_entries_fts_ai",
		"daily_jira_entries_fts_ad",
		"daily_jira_entries_fts_au",
	}
}

func readModelSchemaStatements() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS daily_jira_audit_state (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			schema_version INTEGER NOT NULL DEFAULT 3,
			backfilled INTEGER NOT NULL DEFAULT 0,
			generation INTEGER NOT NULL DEFAULT 0,
			search_mode TEXT NOT NULL DEFAULT 'prefix',
			rollover_day INTEGER NOT NULL,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`INSERT OR IGNORE INTO daily_jira_audit_state (id, schema_version, backfilled, generation, rollover_day)
			VALUES (1, 3, 0, 0, CAST(julianday(date('now', 'localtime')) AS INTEGER))`,
		`CREATE TABLE IF NOT EXISTS daily_jira_audit_entries (
			task_id TEXT PRIMARY KEY,
			project_key TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			title_key TEXT NOT NULL DEFAULT '',
			project TEXT NOT NULL DEFAULT '',
			assignee TEXT NOT NULL DEFAULT '',
			assignee_key TEXT NOT NULL DEFAULT '',
			reporter TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT '',
			status_key TEXT NOT NULL DEFAULT '',
			issue_type TEXT NOT NULL DEFAULT '',
			task_created_at DATETIME,
			created_unix INTEGER NOT NULL DEFAULT 0,
			created_day INTEGER,
			last_activity_at DATETIME,
			activity_unix INTEGER NOT NULL DEFAULT 0,
			due_date DATETIME,
			due_day INTEGER,
			decision_logs TEXT NOT NULL DEFAULT '',
			bucket TEXT NOT NULL,
			overdue INTEGER NOT NULL DEFAULT 0,
			sort_overdue INTEGER NOT NULL DEFAULT 1,
			next_bucket_day INTEGER,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS daily_jira_audit_counts (
			project_key TEXT NOT NULL,
			assignee_key TEXT NOT NULL,
			bucket TEXT NOT NULL,
			item_count INTEGER NOT NULL,
			PRIMARY KEY (project_key, assignee_key, bucket)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_jira_page
			ON daily_jira_audit_entries (bucket, sort_overdue, created_unix, activity_unix, task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_jira_project_page
			ON daily_jira_audit_entries (project_key, bucket, sort_overdue, created_unix, activity_unix, task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_jira_assignee_page
			ON daily_jira_audit_entries (assignee_key, bucket, sort_overdue, created_unix, activity_unix, task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_jira_project_assignee_page
			ON daily_jira_audit_entries (project_key, assignee_key, bucket, sort_overdue, created_unix, activity_unix, task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_jira_bucket_transition
			ON daily_jira_audit_entries (next_bucket_day) WHERE next_bucket_day IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_daily_jira_due_transition
			ON daily_jira_audit_entries (overdue, due_day) WHERE due_day IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_daily_jira_short_task_search ON daily_jira_audit_entries (bucket, task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_jira_short_title_search ON daily_jira_audit_entries (bucket, title_key)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_jira_short_project_search ON daily_jira_audit_entries (bucket, project_key)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_jira_short_assignee_search ON daily_jira_audit_entries (bucket, assignee_key)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_jira_short_status_search ON daily_jira_audit_entries (bucket, status_key)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_jira_count_bucket
			ON daily_jira_audit_counts (bucket, project_key, assignee_key)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_jira_count_assignee
			ON daily_jira_audit_counts (assignee_key, bucket, project_key)`,
	}
}

func readModelFTSSchemaStatement() string {
	return `CREATE VIRTUAL TABLE IF NOT EXISTS daily_jira_audit_search USING fts5(
		task_id, title, project, assignee, status,
		content='daily_jira_audit_entries', content_rowid='rowid', tokenize='trigram'
	)`
}

func readModelTriggerStatements(ftsEnabled bool) []string {
	insertEntry := projectionInsertSQL("NEW.")
	projectionColumns := `task_id, project_key, source, title, repo, assignee, jira_reporter,
		status, issue_type, task_created_at, last_update, source_updated_at, due_date, decision_logs`
	statements := []string{
		`CREATE TRIGGER IF NOT EXISTS daily_jira_entries_ai AFTER INSERT ON daily_jira_audit_entries BEGIN
			INSERT INTO daily_jira_audit_counts (project_key, assignee_key, bucket, item_count)
			VALUES (NEW.project_key, NEW.assignee_key, NEW.bucket, 1)
			ON CONFLICT(project_key, assignee_key, bucket) DO UPDATE SET item_count = item_count + 1;
		END`,
		`CREATE TRIGGER IF NOT EXISTS daily_jira_entries_ad AFTER DELETE ON daily_jira_audit_entries BEGIN
			UPDATE daily_jira_audit_counts SET item_count = item_count - 1
			WHERE project_key = OLD.project_key AND assignee_key = OLD.assignee_key AND bucket = OLD.bucket;
			DELETE FROM daily_jira_audit_counts WHERE item_count <= 0;
		END`,
		`CREATE TRIGGER IF NOT EXISTS daily_jira_entries_au_count
		AFTER UPDATE OF project_key, assignee_key, bucket ON daily_jira_audit_entries
		WHEN OLD.project_key <> NEW.project_key OR OLD.assignee_key <> NEW.assignee_key OR OLD.bucket <> NEW.bucket
		BEGIN
			UPDATE daily_jira_audit_counts SET item_count = item_count - 1
			WHERE project_key = OLD.project_key AND assignee_key = OLD.assignee_key AND bucket = OLD.bucket;
			DELETE FROM daily_jira_audit_counts WHERE item_count <= 0;
			INSERT INTO daily_jira_audit_counts (project_key, assignee_key, bucket, item_count)
			VALUES (NEW.project_key, NEW.assignee_key, NEW.bucket, 1)
			ON CONFLICT(project_key, assignee_key, bucket) DO UPDATE SET item_count = item_count + 1;
		END`,
		fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS daily_jira_tasks_ai AFTER INSERT ON task_telemetries
		WHEN %s BEGIN
			UPDATE daily_jira_audit_state SET generation = generation + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
			%s;
		END`, eligibleTaskSQL("NEW."), insertEntry),
		fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS daily_jira_tasks_au AFTER UPDATE OF %s ON task_telemetries
		WHEN (%s OR %s) AND (%s) BEGIN
			UPDATE daily_jira_audit_state SET generation = generation + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
			DELETE FROM daily_jira_audit_entries WHERE task_id = OLD.task_id;
			%s;
		END`, projectionColumns, eligibleTaskSQL("OLD."), eligibleTaskSQL("NEW."), projectionChangedSQL(), insertEntry),
		fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS daily_jira_tasks_ad AFTER DELETE ON task_telemetries
		WHEN %s BEGIN
			UPDATE daily_jira_audit_state SET generation = generation + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
			DELETE FROM daily_jira_audit_entries WHERE task_id = OLD.task_id;
		END`, eligibleTaskSQL("OLD.")),
	}
	if ftsEnabled {
		statements = append(statements,
			`CREATE TRIGGER IF NOT EXISTS daily_jira_entries_fts_ai AFTER INSERT ON daily_jira_audit_entries BEGIN
				INSERT INTO daily_jira_audit_search(rowid, task_id, title, project, assignee, status)
				VALUES (NEW.rowid, NEW.task_id, NEW.title, NEW.project, NEW.assignee, NEW.status);
			END`,
			`CREATE TRIGGER IF NOT EXISTS daily_jira_entries_fts_ad AFTER DELETE ON daily_jira_audit_entries BEGIN
				INSERT INTO daily_jira_audit_search(daily_jira_audit_search, rowid, task_id, title, project, assignee, status)
				VALUES ('delete', OLD.rowid, OLD.task_id, OLD.title, OLD.project, OLD.assignee, OLD.status);
			END`,
			`CREATE TRIGGER IF NOT EXISTS daily_jira_entries_fts_au
			AFTER UPDATE OF task_id, title, project, assignee, status ON daily_jira_audit_entries BEGIN
				INSERT INTO daily_jira_audit_search(daily_jira_audit_search, rowid, task_id, title, project, assignee, status)
				VALUES ('delete', OLD.rowid, OLD.task_id, OLD.title, OLD.project, OLD.assignee, OLD.status);
				INSERT INTO daily_jira_audit_search(rowid, task_id, title, project, assignee, status)
				VALUES (NEW.rowid, NEW.task_id, NEW.title, NEW.project, NEW.assignee, NEW.status);
			END`,
		)
	}
	return statements
}

func backfillEntriesSQL() string {
	return projectionInsertSQL("") + " FROM task_telemetries WHERE " + eligibleTaskSQL("")
}

func projectionInsertSQL(prefix string) string {
	fromClause := ""
	whereClause := " WHERE " + eligibleTaskSQL(prefix)
	if prefix == "" {
		fromClause = ""
		whereClause = ""
	}
	activity := activityTimeSQL(prefix)
	return fmt.Sprintf(`INSERT INTO daily_jira_audit_entries (
		task_id, project_key, title, title_key, project, assignee, assignee_key, reporter, status, status_key, issue_type,
		task_created_at, created_unix, created_day, last_activity_at, activity_unix,
		due_date, due_day, decision_logs, bucket, overdue, sort_overdue, next_bucket_day, updated_at
	) SELECT
		%[1]stask_id,
		%[2]s,
		COALESCE(%[1]stitle, ''),
		LOWER(TRIM(COALESCE(%[1]stitle, ''))),
		COALESCE(%[1]srepo, ''),
		COALESCE(%[1]sassignee, ''),
		LOWER(TRIM(COALESCE(%[1]sassignee, ''))),
		COALESCE(%[1]sjira_reporter, ''),
		COALESCE(%[1]sstatus, ''),
		LOWER(TRIM(COALESCE(%[1]sstatus, ''))),
		COALESCE(%[1]sissue_type, ''),
		%[1]stask_created_at,
		COALESCE(unixepoch(%[1]stask_created_at), 0),
		%[3]s,
		%[4]s,
		COALESCE(unixepoch(%[4]s), 0),
		%[1]sdue_date,
		%[5]s,
		COALESCE(%[1]sdecision_logs, ''),
		%[6]s,
		%[7]s,
		1 - (%[7]s),
		%[8]s,
		CURRENT_TIMESTAMP%[9]s%[10]s`,
		prefix,
		projectKeySQL(prefix),
		daySQL(prefix+"task_created_at"),
		activity,
		daySQL(prefix+"due_date"),
		bucketSQL(prefix),
		overdueSQL(prefix),
		nextBucketDaySQL(prefix),
		fromClause,
		whereClause,
	)
}

func eligibleTaskSQL(prefix string) string {
	return "(" + candidateTaskSQL(prefix) + ") AND (" + unresolvedTaskSQL(prefix) + ")"
}

func projectionChangedSQL() string {
	columns := []string{
		"task_id", "project_key", "source", "title", "repo", "assignee", "jira_reporter",
		"status", "issue_type", "task_created_at", "last_update", "source_updated_at", "due_date", "decision_logs",
	}
	changes := make([]string, 0, len(columns))
	for _, column := range columns {
		changes = append(changes, "OLD."+column+" IS NOT NEW."+column)
	}
	return strings.Join(changes, " OR ")
}

func candidateTaskSQL(prefix string) string {
	taskID := prefix + "task_id"
	source := prefix + "source"
	repo := prefix + "repo"
	prefixExpr := "UPPER(substr(" + taskID + ", 1, instr(" + taskID + ", '-') - 1))"
	suffixExpr := "substr(" + taskID + ", instr(" + taskID + ", '-') + 1)"
	return fmt.Sprintf(`LOWER(TRIM(COALESCE(%s, ''))) = 'jira' OR (
		TRIM(COALESCE(%s, '')) = ''
		AND instr(%s, '-') > 1
		AND %s GLOB '[A-Z]*'
		AND %s NOT GLOB '*[^A-Z0-9_]*'
		AND %s <> '' AND %s NOT GLOB '*[^0-9]*'
		AND %s NOT LIKE 'TASK-%%' AND %s NOT LIKE 'DEMAND-%%'
		AND UPPER(COALESCE(%s, '')) LIKE '%%(' || %s || ')%%'
	)`, source, source, taskID, prefixExpr, prefixExpr, suffixExpr, suffixExpr, taskID, taskID, repo, prefixExpr)
}

func unresolvedTaskSQL(prefix string) string {
	status := prefix + "status"
	return fmt.Sprintf(`%s IS NULL OR LOWER(TRIM(%s)) NOT IN
		('done','closed','resolved','completed','archived','已完成','已关闭')`, status, status)
}

func projectKeySQL(prefix string) string {
	taskID := prefix + "task_id"
	return fmt.Sprintf(`UPPER(TRIM(COALESCE(NULLIF(%sproject_key, ''),
		CASE WHEN instr(%s, '-') > 1 THEN substr(%s, 1, instr(%s, '-') - 1) ELSE '' END)))`, prefix, taskID, taskID, taskID)
}

func validDateSQL(column string) string {
	return fmt.Sprintf(`%s IS NOT NULL AND TRIM(CAST(%s AS TEXT)) <> ''
		AND COALESCE(CAST(strftime('%%Y', %s) AS INTEGER), 0) > 1`, column, column, column)
}

func daySQL(column string) string {
	return fmt.Sprintf(`CASE WHEN %s THEN CAST(julianday(date(%s, 'localtime')) AS INTEGER) ELSE NULL END`, validDateSQL(column), column)
}

func ageSQL(prefix string) string {
	return fmt.Sprintf(`CAST(julianday(date('now', 'localtime')) AS INTEGER) - (%s)`, daySQL(prefix+"task_created_at"))
}

func bucketSQL(prefix string) string {
	age := ageSQL(prefix)
	return fmt.Sprintf(`CASE
		WHEN NOT (%s) OR %s < 0 THEN 'unclassified'
		WHEN %s = 0 THEN 'today'
		WHEN %s BETWEEN 1 AND 2 THEN 'recent_watch'
		WHEN %s BETWEEN 3 AND 6 THEN 'three_day'
		ELSE 'seven_day'
	END`, validDateSQL(prefix+"task_created_at"), age, age, age, age)
}

func nextBucketDaySQL(prefix string) string {
	age := ageSQL(prefix)
	createdDay := daySQL(prefix + "task_created_at")
	return fmt.Sprintf(`CASE
		WHEN NOT (%s) OR %s < 0 THEN NULL
		WHEN %s = 0 THEN (%s) + 1
		WHEN %s BETWEEN 1 AND 2 THEN (%s) + 3
		WHEN %s BETWEEN 3 AND 6 THEN (%s) + 7
		ELSE NULL
	END`, validDateSQL(prefix+"task_created_at"), age, age, createdDay, age, createdDay, age, createdDay)
}

func overdueSQL(prefix string) string {
	dueDay := daySQL(prefix + "due_date")
	return fmt.Sprintf(`CASE WHEN %s AND (%s) < CAST(julianday(date('now', 'localtime')) AS INTEGER)
		THEN 1 ELSE 0 END`, validDateSQL(prefix+"due_date"), dueDay)
}

func activityTimeSQL(prefix string) string {
	return fmt.Sprintf(`CASE WHEN %s THEN %ssource_updated_at ELSE %slast_update END`, validDateSQL(prefix+"source_updated_at"), prefix, prefix)
}
