package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"well-ambient/internal/db"
	"well-ambient/internal/server"

	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type extraArchiveTableSpec struct {
	name             string
	columns          []string
	targetDDL        string
	resetSequenceSQL string
}

type extraArchiveSourceTable struct {
	spec        extraArchiveTableSpec
	sourceCount int64
}

type extraArchiveTargetTableState struct {
	name     string
	present  bool
	rowCount int64
}

var extraArchiveTableSpecs = []extraArchiveTableSpec{
	{
		name: "ai_context_profiles",
		columns: []string{
			"id", "module_name", "source_filename", "source_sheet", "summary", "prompt_summary",
			"feature_count", "configurable_count", "non_configurable_count", "status_breakdown_json",
			"type_breakdown_json", "feature_snapshot_json", "enabled", "version", "created_at", "updated_at",
		},
		targetDDL: `CREATE TABLE IF NOT EXISTS ai_context_profiles (
			id SERIAL PRIMARY KEY,
			module_name TEXT,
			source_filename TEXT,
			source_sheet TEXT,
			summary TEXT,
			prompt_summary TEXT,
			feature_count INTEGER,
			configurable_count INTEGER,
			non_configurable_count INTEGER,
			status_breakdown_json TEXT,
			type_breakdown_json TEXT,
			feature_snapshot_json TEXT,
			enabled BOOLEAN,
			version INTEGER,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ
		);`,
		resetSequenceSQL: "SELECT setval(pg_get_serial_sequence('ai_context_profiles', 'id'), COALESCE(MAX(id), 1), EXISTS(SELECT 1 FROM ai_context_profiles)) FROM ai_context_profiles;",
	},
	{
		name:    "jira_assignee_archive_runs",
		columns: []string{"run_id", "fetched_at", "config_version", "jql", "projects_json", "users_json", "issue_count", "base_url"},
		targetDDL: `CREATE TABLE IF NOT EXISTS jira_assignee_archive_runs (
			run_id TEXT PRIMARY KEY,
			fetched_at TEXT NOT NULL,
			config_version INTEGER NOT NULL,
			jql TEXT NOT NULL,
			projects_json TEXT NOT NULL,
			users_json TEXT NOT NULL,
			issue_count INTEGER NOT NULL,
			base_url TEXT NOT NULL
		);`,
	},
	{
		name: "jira_assignee_issue_archives",
		columns: []string{
			"run_id", "issue_key", "project_key", "created_at", "updated_at", "categories_json",
			"matched_assignees_json", "payload_json", "payload_sha256",
		},
		targetDDL: `CREATE TABLE IF NOT EXISTS jira_assignee_issue_archives (
			run_id TEXT NOT NULL REFERENCES jira_assignee_archive_runs(run_id),
			issue_key TEXT NOT NULL,
			project_key TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			categories_json TEXT NOT NULL,
			matched_assignees_json TEXT NOT NULL,
			payload_json TEXT NOT NULL,
			payload_sha256 TEXT NOT NULL,
			PRIMARY KEY(run_id, issue_key)
		);`,
	},
	{
		name:    "jira_gpp_archive_runs",
		columns: []string{"run_id", "fetched_at", "config_version", "jql", "projects_json", "users_json", "issue_count", "base_url"},
		targetDDL: `CREATE TABLE IF NOT EXISTS jira_gpp_archive_runs (
			run_id TEXT PRIMARY KEY,
			fetched_at TEXT NOT NULL,
			config_version INTEGER NOT NULL,
			jql TEXT NOT NULL,
			projects_json TEXT NOT NULL,
			users_json TEXT NOT NULL,
			issue_count INTEGER NOT NULL,
			base_url TEXT NOT NULL
		);`,
	},
	{
		name: "jira_gpp_issue_archives",
		columns: []string{
			"run_id", "issue_key", "project_key", "created_at", "updated_at", "categories_json",
			"matched_assignees_json", "payload_json", "payload_sha256",
		},
		targetDDL: `CREATE TABLE IF NOT EXISTS jira_gpp_issue_archives (
			run_id TEXT NOT NULL REFERENCES jira_gpp_archive_runs(run_id),
			issue_key TEXT NOT NULL,
			project_key TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			categories_json TEXT NOT NULL,
			matched_assignees_json TEXT NOT NULL,
			payload_json TEXT NOT NULL,
			payload_sha256 TEXT NOT NULL,
			PRIMARY KEY(run_id, issue_key)
		);`,
	},
}

type extraArchivePreflightOps struct {
	tableExists  func(context.Context, *sql.DB, extraArchiveTableSpec) (bool, error)
	tableColumns func(context.Context, *sql.DB, extraArchiveTableSpec) ([]string, error)
	tableCount   func(context.Context, *sql.DB, extraArchiveTableSpec) (int64, error)
}

var defaultExtraArchivePreflightOps = extraArchivePreflightOps{
	tableExists:  sqliteExtraArchiveTableExists,
	tableColumns: sqliteExtraArchiveTableColumns,
	tableCount:   sqliteExtraArchiveTableCount,
}

func main() {
	var (
		sourcePath string
		targetDSN  string
		apply      bool
		dryRun     bool
	)

	flag.StringVar(&sourcePath, "source", "well-ambient.db", "Path to source SQLite database")
	flag.StringVar(&targetDSN, "target-dsn", strings.TrimSpace(os.Getenv("WELL_AMBIENT_TARGET_DSN")), "Target PostgreSQL DSN")
	flag.BoolVar(&apply, "apply", false, "Actually apply the synchronization (default is dry-run)")
	flag.BoolVar(&dryRun, "dry-run", false, "Run in dry-run mode (default true if --apply is not passed)")
	flag.Parse()

	if err := runSyncProduction(context.Background(), sourcePath, targetDSN, apply, dryRun); err != nil {
		log.Fatal(err)
	}
}

func runSyncProduction(ctx context.Context, sourcePath, targetDSN string, apply, dryRun bool) error {
	targetDSN = strings.TrimSpace(targetDSN)
	if targetDSN == "" {
		return errors.New("target PostgreSQL DSN is required; set WELL_AMBIENT_TARGET_DSN or pass --target-dsn")
	}

	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("invalid source database path: %w", err)
	}

	snapshot, err := db.InspectLegacySQLite(absSource)
	if err != nil {
		return fmt.Errorf("inspect source SQLite database: %w", err)
	}
	log.Printf("[Source] Valid SQLite snapshot at %s (Size: %.2f MB, Tables: %d)",
		snapshot.Path, float64(snapshot.SizeBytes)/(1024*1024), snapshot.TableCount)

	extraSource, sourceExtraTables, err := inspectExtraArchiveSource(ctx, absSource)
	if err != nil {
		return fmt.Errorf("inspect source extra archive tables: %w", err)
	}
	defer extraSource.Close()

	var sourceExtraRows int64
	for _, table := range sourceExtraTables {
		sourceExtraRows += table.sourceCount
		log.Printf("[Source Extra Tables] %s: %d rows", table.spec.name, table.sourceCount)
	}

	log.Printf("[Target] Connecting to PostgreSQL...")
	targetDB, err := gorm.Open(postgres.Open(targetDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return fmt.Errorf("connect to target PostgreSQL: %w", err)
	}
	sqlDB, err := targetDB.DB()
	if err != nil {
		return fmt.Errorf("get target DB connection pool: %w", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping target PostgreSQL: %w", err)
	}
	log.Printf("[Target] PostgreSQL connection verified successfully")

	targetSafe, err := migrationTargetIsSafe(ctx, targetDB)
	if err != nil {
		return fmt.Errorf("inspect target database safety: %w", err)
	}
	log.Printf("[Target] Combined target safety check (bootstrap/seed core data and missing/empty known extra tables only): %v", targetSafe)

	if !apply || dryRun {
		log.Printf("================ DRY-RUN SUMMARY ================")
		log.Printf("Source: %s (%.2f MB, %d tables)", absSource, float64(snapshot.SizeBytes)/(1024*1024), snapshot.TableCount)
		log.Printf("Source extra archive tables: %d present, %d rows", len(sourceExtraTables), sourceExtraRows)
		log.Printf("Target safe for full sync: %v", targetSafe)
		log.Printf("Pass --apply to execute full synchronization.")
		log.Printf("=================================================")
		return nil
	}

	if !targetSafe {
		return errors.New("target database safety check failed: target contains non-bootstrap core data or non-empty known extra tables; refusing to overwrite")
	}

	log.Printf("Starting full database synchronization...")
	startTime := time.Now()

	lastStage := ""
	lastLogged := time.Now()
	progress := func(p db.LegacyMigrationProgress) {
		now := time.Now()
		if p.Stage != lastStage || now.Sub(lastLogged) > 3*time.Second || p.TablesCompleted == p.TablesTotal {
			lastStage = p.Stage
			lastLogged = now
			if p.Table != "" {
				log.Printf("[%s] Table %s (%d/%d), copied %d rows so far",
					p.Stage, p.Table, p.TablesCompleted, p.TablesTotal, p.RowsCopied)
			} else {
				log.Printf("[%s] Completed %d/%d tables, %d rows copied",
					p.Stage, p.TablesCompleted, p.TablesTotal, p.RowsCopied)
			}
		}
	}

	targetSafe, err = migrationTargetIsSafe(ctx, targetDB)
	if err != nil {
		return fmt.Errorf("recheck target database safety immediately before migration: %w", err)
	}
	if !targetSafe {
		return errors.New("target database safety recheck failed immediately before migration; refusing to overwrite")
	}
	report, err := db.MigrateLegacySQLite(ctx, absSource, targetDB, server.MigrateReadModels, progress)
	if err != nil {
		return fmt.Errorf("migrate core database: %w", err)
	}

	extraCopied, err := syncExtraTables(ctx, extraSource, targetDB, sourceExtraTables)
	if err != nil {
		return fmt.Errorf("sync extra archive tables: %w", err)
	}
	log.Printf("[Extra Tables] Successfully synced %d rows across %d present archive tables", extraCopied, len(sourceExtraTables))

	elapsed := time.Since(startTime)
	log.Printf("================ SYNC COMPLETE ================")
	log.Printf("Total elapsed time: %v", elapsed)
	log.Printf("Core tables copied: %d", report.TablesCopied)
	log.Printf("Total core rows copied: %d", report.RowsCopied)
	log.Printf("Text values repaired (UTF-8 / null bytes): %d", report.TextValuesRepaired)
	log.Printf("Extra archive rows copied: %d", extraCopied)
	log.Printf("===============================================")
	return nil
}

func inspectExtraArchiveSource(ctx context.Context, sqlitePath string) (*sql.DB, []extraArchiveSourceTable, error) {
	source, err := sql.Open("sqlite3", extraArchiveSQLiteReadOnlyDSN(sqlitePath))
	if err != nil {
		return nil, nil, err
	}
	source.SetMaxOpenConns(1)
	if err := source.PingContext(ctx); err != nil {
		source.Close()
		return nil, nil, fmt.Errorf("open read-only SQLite source: %w", err)
	}
	tables, err := inspectExtraArchiveTables(ctx, source)
	if err != nil {
		source.Close()
		return nil, nil, err
	}
	return source, tables, nil
}

func inspectExtraArchiveTables(ctx context.Context, source *sql.DB) ([]extraArchiveSourceTable, error) {
	return inspectExtraArchiveTablesWithOps(ctx, source, extraArchiveTableSpecs, defaultExtraArchivePreflightOps)
}

func inspectExtraArchiveTablesWithOps(ctx context.Context, source *sql.DB, specs []extraArchiveTableSpec, ops extraArchivePreflightOps) ([]extraArchiveSourceTable, error) {
	if source == nil {
		return nil, errors.New("SQLite source is nil")
	}

	tables := make([]extraArchiveSourceTable, 0, len(specs))
	for _, spec := range specs {
		exists, err := ops.tableExists(ctx, source, spec)
		if err != nil {
			return nil, fmt.Errorf("inspect SQLite extra table %s: %w", spec.name, err)
		}
		if !exists {
			continue
		}
		columns, err := ops.tableColumns(ctx, source, spec)
		if err != nil {
			return nil, fmt.Errorf("inspect SQLite extra table %s columns: %w", spec.name, err)
		}
		if err := validateExtraArchiveColumns(spec, columns); err != nil {
			return nil, err
		}
		count, err := ops.tableCount(ctx, source, spec)
		if err != nil {
			return nil, fmt.Errorf("count SQLite extra table %s: %w", spec.name, err)
		}
		tables = append(tables, extraArchiveSourceTable{spec: spec, sourceCount: count})
	}
	return tables, nil
}

func sqliteExtraArchiveTableExists(ctx context.Context, source *sql.DB, spec extraArchiveTableSpec) (bool, error) {
	var marker int
	err := source.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ? LIMIT 1`, spec.name).Scan(&marker)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func sqliteExtraArchiveTableColumns(ctx context.Context, source *sql.DB, spec extraArchiveTableSpec) ([]string, error) {
	rows, err := source.QueryContext(ctx, `SELECT name FROM pragma_table_info(?) ORDER BY cid`, spec.name)
	if err != nil {
		return nil, err
	}
	columns := make([]string, 0, len(spec.columns))
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return nil, finishExtraArchiveRows(rows, spec.name, fmt.Errorf("scan SQLite table metadata: %w", err))
		}
		columns = append(columns, column)
	}
	if err := finishExtraArchiveRows(rows, spec.name, nil); err != nil {
		return nil, err
	}
	return columns, nil
}

func sqliteExtraArchiveTableCount(ctx context.Context, source *sql.DB, spec extraArchiveTableSpec) (int64, error) {
	var count int64
	if err := source.QueryRowContext(ctx, spec.sourceCountSQL()).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func validateExtraArchiveColumns(spec extraArchiveTableSpec, actual []string) error {
	if len(actual) != len(spec.columns) {
		return fmt.Errorf("SQLite extra table %s columns = %v, want exactly %v", spec.name, actual, spec.columns)
	}
	for index := range spec.columns {
		if actual[index] != spec.columns[index] {
			return fmt.Errorf("SQLite extra table %s columns = %v, want exactly %v", spec.name, actual, spec.columns)
		}
	}
	return nil
}

func inspectExtraArchiveTarget(ctx context.Context, target *gorm.DB) ([]extraArchiveTargetTableState, error) {
	if target == nil {
		return nil, gorm.ErrInvalidDB
	}

	states := make([]extraArchiveTargetTableState, 0, len(extraArchiveTableSpecs))
	if target.Dialector.Name() != "postgres" {
		// The command always opens PostgreSQL; this branch supports fixture-only helper tests.
		for _, spec := range extraArchiveTableSpecs {
			state := extraArchiveTargetTableState{name: spec.name}
			if !target.WithContext(ctx).Migrator().HasTable(spec.name) {
				states = append(states, state)
				continue
			}
			state.present = true
			if err := target.WithContext(ctx).Raw(spec.targetCountSQL()).Scan(&state.rowCount).Error; err != nil {
				return nil, fmt.Errorf("count target extra table %s: %w", spec.name, err)
			}
			states = append(states, state)
		}
		return states, nil
	}

	for _, spec := range extraArchiveTableSpecs {
		state := extraArchiveTargetTableState{name: spec.name}
		var relationKind sql.NullString
		if err := target.WithContext(ctx).Raw(`SELECT c.relkind::text
			FROM pg_catalog.pg_class c
			JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = current_schema() AND c.relname = ?`, spec.name).Scan(&relationKind).Error; err != nil {
			return nil, fmt.Errorf("inspect target relation %s: %w", spec.name, err)
		}
		if !relationKind.Valid {
			states = append(states, state)
			continue
		}
		if relationKind.String != "r" && relationKind.String != "p" {
			return nil, fmt.Errorf("target relation %s exists but is not a table", spec.name)
		}
		state.present = true
		if err := target.WithContext(ctx).Raw(spec.targetCountSQL()).Scan(&state.rowCount).Error; err != nil {
			return nil, fmt.Errorf("count target extra table %s: %w", spec.name, err)
		}
		states = append(states, state)
	}
	return states, nil
}

func migrationTargetIsSafe(ctx context.Context, target *gorm.DB) (bool, error) {
	coreSafe, err := db.LegacyMigrationTargetIsSafe(target)
	if err != nil {
		return false, err
	}
	states, err := inspectExtraArchiveTarget(ctx, target)
	if err != nil {
		return false, err
	}
	extraSafe, _ := extraArchiveTargetIsSafe(states)
	return coreSafe && extraSafe, nil
}

func extraArchiveTargetIsSafe(states []extraArchiveTargetTableState) (bool, []string) {
	unsafe := make([]string, 0)
	for _, state := range states {
		if state.present && state.rowCount > 0 {
			unsafe = append(unsafe, fmt.Sprintf("%s (%d rows)", state.name, state.rowCount))
		}
	}
	return len(unsafe) == 0, unsafe
}

func syncExtraTables(ctx context.Context, source *sql.DB, target *gorm.DB, tables []extraArchiveSourceTable) (int64, error) {
	if source == nil {
		return 0, errors.New("SQLite source is nil")
	}
	if target == nil {
		return 0, gorm.ErrInvalidDB
	}

	var totalCopied int64
	for _, table := range tables {
		copied, err := syncExtraTable(ctx, source, target, table)
		if err != nil {
			return totalCopied, err
		}
		totalCopied += copied
		log.Printf("[Extra Tables] Copied %d rows for %s", copied, table.spec.name)
	}
	return totalCopied, nil
}

func syncExtraTable(ctx context.Context, source *sql.DB, target *gorm.DB, table extraArchiveSourceTable) (copied int64, err error) {
	spec := table.spec
	tx := target.WithContext(ctx).Begin()
	if tx.Error != nil {
		return 0, fmt.Errorf("begin extra table %s transaction: %w", spec.name, tx.Error)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		rollbackErr := tx.Rollback().Error
		if rollbackErr == nil || errors.Is(rollbackErr, sql.ErrTxDone) {
			return
		}
		wrapped := fmt.Errorf("rollback extra table %s: %w", spec.name, rollbackErr)
		if err == nil {
			err = wrapped
			return
		}
		err = errors.Join(err, wrapped)
	}()

	if err := tx.Exec(spec.targetDDL).Error; err != nil {
		return 0, fmt.Errorf("create extra table %s: %w", spec.name, err)
	}
	if err := tx.Exec(spec.targetLockSQL()).Error; err != nil {
		return 0, fmt.Errorf("lock extra table %s: %w", spec.name, err)
	}

	var existingTargetCount int64
	if err := tx.Raw(spec.targetCountSQL()).Scan(&existingTargetCount).Error; err != nil {
		return 0, fmt.Errorf("recheck target extra table %s: %w", spec.name, err)
	}
	if existingTargetCount != 0 {
		return 0, fmt.Errorf("target extra table %s became non-empty after safety check (%d rows); refusing to truncate", spec.name, existingTargetCount)
	}

	if err := tx.Exec(spec.targetTruncateSQL()).Error; err != nil {
		return 0, fmt.Errorf("truncate extra table %s: %w", spec.name, err)
	}

	rows, err := source.QueryContext(ctx, spec.sourceSelectSQL())
	if err != nil {
		return 0, fmt.Errorf("read SQLite extra table %s: %w", spec.name, err)
	}

	rawValues := make([]any, len(spec.columns))
	scanArgs := make([]any, len(spec.columns))
	for index := range rawValues {
		scanArgs[index] = &rawValues[index]
	}

	insertSQL := spec.targetInsertSQL()
	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			return 0, finishExtraArchiveRows(rows, spec.name, fmt.Errorf("scan SQLite extra table %s: %w", spec.name, err))
		}
		execArgs := make([]any, len(rawValues))
		for index, value := range rawValues {
			execArgs[index] = normalizeExtraArchiveValue(value)
		}
		if err := tx.Exec(insertSQL, execArgs...).Error; err != nil {
			return 0, finishExtraArchiveRows(rows, spec.name, fmt.Errorf("insert into extra table %s: %w", spec.name, err))
		}
		copied++
	}
	if err := finishExtraArchiveRows(rows, spec.name, nil); err != nil {
		return 0, err
	}
	if copied != table.sourceCount {
		return 0, fmt.Errorf("verify source extra table %s: inspected %d rows, copied %d", spec.name, table.sourceCount, copied)
	}

	var targetCount int64
	if err := tx.Raw(spec.targetCountSQL()).Scan(&targetCount).Error; err != nil {
		return 0, fmt.Errorf("count target extra table %s: %w", spec.name, err)
	}
	if targetCount != table.sourceCount {
		return 0, fmt.Errorf("verify target extra table %s: source rows %d, target rows %d", spec.name, table.sourceCount, targetCount)
	}

	if spec.resetSequenceSQL != "" {
		if err := tx.Exec(spec.resetSequenceSQL).Error; err != nil {
			return 0, fmt.Errorf("reset sequence for %s: %w", spec.name, err)
		}
	}
	if err := tx.Commit().Error; err != nil {
		return 0, fmt.Errorf("commit extra table %s: %w", spec.name, err)
	}
	committed = true
	return copied, nil
}

func finishExtraArchiveRows(rows *sql.Rows, table string, prior error) error {
	if rowsErr := rows.Err(); rowsErr != nil {
		prior = errors.Join(prior, fmt.Errorf("iterate SQLite extra table %s: %w", table, rowsErr))
	}
	if closeErr := rows.Close(); closeErr != nil {
		prior = errors.Join(prior, fmt.Errorf("close SQLite extra table %s rows: %w", table, closeErr))
	}
	return prior
}

func extraArchiveSQLiteReadOnlyDSN(path string) string {
	fileURL := &url.URL{Scheme: "file", Path: path}
	query := fileURL.Query()
	query.Set("mode", "ro")
	query.Set("immutable", "1")
	query.Set("_foreign_keys", "on")
	query.Set("_busy_timeout", "5000")
	fileURL.RawQuery = query.Encode()
	return fileURL.String()
}

func normalizeExtraArchiveValue(value any) any {
	switch typed := value.(type) {
	case []byte:
		value = string(typed)
	case string:
		value = typed
	default:
		return value
	}
	text := strings.ToValidUTF8(value.(string), "\uFFFD")
	return strings.ReplaceAll(text, "\x00", "\uFFFD")
}

func (spec extraArchiveTableSpec) sourceCountSQL() string {
	return fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteSQLIdentifier(spec.name))
}

func (spec extraArchiveTableSpec) sourceSelectSQL() string {
	return fmt.Sprintf("SELECT %s FROM %s", quoteSQLIdentifierList(spec.columns), quoteSQLIdentifier(spec.name))
}

func (spec extraArchiveTableSpec) targetInsertSQL() string {
	placeholders := make([]string, len(spec.columns))
	for index := range placeholders {
		placeholders[index] = fmt.Sprintf("$%d", index+1)
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quoteSQLIdentifier(spec.name), quoteSQLIdentifierList(spec.columns), strings.Join(placeholders, ", "))
}

func (spec extraArchiveTableSpec) targetLockSQL() string {
	return fmt.Sprintf("LOCK TABLE %s IN ACCESS EXCLUSIVE MODE", quoteSQLIdentifier(spec.name))
}

func (spec extraArchiveTableSpec) targetCountSQL() string {
	return fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteSQLIdentifier(spec.name))
}

func (spec extraArchiveTableSpec) targetTruncateSQL() string {
	return fmt.Sprintf("TRUNCATE TABLE %s CASCADE", quoteSQLIdentifier(spec.name))
}

func quoteSQLIdentifierList(values []string) string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = quoteSQLIdentifier(value)
	}
	return strings.Join(quoted, ", ")
}

func quoteSQLIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
