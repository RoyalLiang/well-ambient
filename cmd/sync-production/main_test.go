package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestExtraArchiveTableSpecsGenerateStaticSQL(t *testing.T) {
	expectedColumns := map[string][]string{
		"ai_context_profiles": {
			"id", "module_name", "source_filename", "source_sheet", "summary", "prompt_summary",
			"feature_count", "configurable_count", "non_configurable_count", "status_breakdown_json",
			"type_breakdown_json", "feature_snapshot_json", "enabled", "version", "created_at", "updated_at",
		},
		"jira_assignee_archive_runs": {
			"run_id", "fetched_at", "config_version", "jql", "projects_json", "users_json", "issue_count", "base_url",
		},
		"jira_assignee_issue_archives": {
			"run_id", "issue_key", "project_key", "created_at", "updated_at", "categories_json",
			"matched_assignees_json", "payload_json", "payload_sha256",
		},
		"jira_gpp_archive_runs": {
			"run_id", "fetched_at", "config_version", "jql", "projects_json", "users_json", "issue_count", "base_url",
		},
		"jira_gpp_issue_archives": {
			"run_id", "issue_key", "project_key", "created_at", "updated_at", "categories_json",
			"matched_assignees_json", "payload_json", "payload_sha256",
		},
	}
	if len(extraArchiveTableSpecs) != len(expectedColumns) {
		t.Fatalf("extraArchiveTableSpecs has %d entries, want %d", len(extraArchiveTableSpecs), len(expectedColumns))
	}

	seen := make(map[string]bool, len(extraArchiveTableSpecs))
	for _, spec := range extraArchiveTableSpecs {
		wantColumns, ok := expectedColumns[spec.name]
		if !ok {
			t.Fatalf("unexpected extra archive table spec %q", spec.name)
		}
		if seen[spec.name] {
			t.Fatalf("duplicate extra archive table spec %q", spec.name)
		}
		seen[spec.name] = true
		if !reflect.DeepEqual(spec.columns, wantColumns) {
			t.Fatalf("%s columns = %#v, want %#v", spec.name, spec.columns, wantColumns)
		}
		if !strings.Contains(spec.targetDDL, "CREATE TABLE IF NOT EXISTS "+spec.name) {
			t.Fatalf("%s DDL does not create the static table: %s", spec.name, spec.targetDDL)
		}

		quotedColumns := `"` + strings.Join(wantColumns, `", "`) + `"`
		wantSelect := fmt.Sprintf(`SELECT %s FROM "%s"`, quotedColumns, spec.name)
		if got := spec.sourceSelectSQL(); got != wantSelect {
			t.Fatalf("%s source SELECT = %q, want %q", spec.name, got, wantSelect)
		}
		if strings.Contains(spec.sourceSelectSQL(), "*") {
			t.Fatalf("%s source SELECT uses a wildcard: %s", spec.name, spec.sourceSelectSQL())
		}

		placeholders := make([]string, len(wantColumns))
		for index := range placeholders {
			placeholders[index] = fmt.Sprintf("$%d", index+1)
		}
		wantInsert := fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s)`, spec.name, quotedColumns, strings.Join(placeholders, ", "))
		if got := spec.targetInsertSQL(); got != wantInsert {
			t.Fatalf("%s target INSERT = %q, want %q", spec.name, got, wantInsert)
		}
		if got := spec.targetLockSQL(); got != fmt.Sprintf(`LOCK TABLE "%s" IN ACCESS EXCLUSIVE MODE`, spec.name) {
			t.Fatalf("%s target LOCK = %q", spec.name, got)
		}
		if got := spec.targetTruncateSQL(); got != fmt.Sprintf(`TRUNCATE TABLE "%s" CASCADE`, spec.name) {
			t.Fatalf("%s target TRUNCATE = %q", spec.name, got)
		}
	}

	if spec := extraArchiveSpecByName(t, "ai_context_profiles"); spec.resetSequenceSQL == "" {
		t.Fatal("ai_context_profiles is missing static sequence reset SQL")
	}
}

func TestInspectExtraArchiveTablesSkipsMissingAndIncludesPresentCounts(t *testing.T) {
	source := openExtraArchiveTestSQLite(t)
	emptySpec := extraArchiveSpecByName(t, "ai_context_profiles")
	populatedSpec := extraArchiveSpecByName(t, "jira_assignee_archive_runs")
	if _, err := source.Exec(emptySpec.targetDDL); err != nil {
		t.Fatalf("create empty fixture table: %v", err)
	}
	if _, err := source.Exec(populatedSpec.targetDDL); err != nil {
		t.Fatalf("create populated fixture table: %v", err)
	}
	if _, err := source.Exec(
		populatedSpec.targetInsertSQL(),
		"run-1", "2026-09-26T10:00:00Z", 1, "project = TEST", `[]`, `[]`, 3, "https://jira.invalid",
	); err != nil {
		t.Fatalf("insert populated fixture row: %v", err)
	}

	tables, err := inspectExtraArchiveTables(context.Background(), source)
	if err != nil {
		t.Fatalf("inspectExtraArchiveTables() error = %v", err)
	}
	if len(tables) != 2 {
		t.Fatalf("inspectExtraArchiveTables() returned %d present tables, want 2: %#v", len(tables), tables)
	}
	counts := make(map[string]int64, len(tables))
	for _, table := range tables {
		counts[table.spec.name] = table.sourceCount
	}
	if count, ok := counts[emptySpec.name]; !ok || count != 0 {
		t.Fatalf("empty present table count = %d, present = %v; want 0, true", count, ok)
	}
	if count, ok := counts[populatedSpec.name]; !ok || count != 1 {
		t.Fatalf("populated table count = %d, present = %v; want 1, true", count, ok)
	}
	if _, ok := counts["jira_gpp_archive_runs"]; ok {
		t.Fatal("missing table was included in the preflight plan")
	}
}

func TestInspectExtraArchiveSourceHandlesURLSensitivePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source #1?.db")
	writerURL := &url.URL{Scheme: "file", Path: path}
	writer, err := sql.Open("sqlite3", writerURL.String())
	if err != nil {
		t.Fatal(err)
	}
	spec := extraArchiveSpecByName(t, "jira_assignee_archive_runs")
	if _, err := writer.Exec(spec.targetDDL); err != nil {
		writer.Close()
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	source, tables, err := inspectExtraArchiveSource(context.Background(), path)
	if err != nil {
		t.Fatalf("inspectExtraArchiveSource() error = %v", err)
	}
	defer source.Close()
	if len(tables) != 1 || tables[0].spec.name != spec.name || tables[0].sourceCount != 0 {
		t.Fatalf("inspectExtraArchiveSource() tables = %#v, want one empty %s table", tables, spec.name)
	}
}

func TestInspectExtraArchiveTablesRejectsPresentTableWithWrongSchema(t *testing.T) {
	source := openExtraArchiveTestSQLite(t)
	if _, err := source.Exec(`CREATE TABLE ai_context_profiles (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}

	_, err := inspectExtraArchiveTables(context.Background(), source)
	if err == nil || !strings.Contains(err.Error(), "want exactly") {
		t.Fatalf("inspectExtraArchiveTables() error = %v, want exact-column validation failure", err)
	}
}

func TestSQLiteMasterDistinguishesMissingTablesFromErrors(t *testing.T) {
	source := openExtraArchiveTestSQLite(t)
	spec := extraArchiveSpecByName(t, "jira_gpp_archive_runs")

	exists, err := sqliteExtraArchiveTableExists(context.Background(), source, spec)
	if err != nil || exists {
		t.Fatalf("missing table exists = %v, err = %v; want false, nil", exists, err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	exists, err = sqliteExtraArchiveTableExists(context.Background(), source, spec)
	if err == nil || exists {
		t.Fatalf("closed database exists = %v, err = %v; want false, real error", exists, err)
	}
}

func TestInspectExtraArchiveTablesPropagatesSchemaAndCountSeamErrors(t *testing.T) {
	source := openExtraArchiveTestSQLite(t)
	spec := extraArchiveSpecByName(t, "jira_assignee_archive_runs")

	t.Run("schema", func(t *testing.T) {
		wantErr := errors.New("schema read failed")
		ops := extraArchivePreflightOps{
			tableExists: func(context.Context, *sql.DB, extraArchiveTableSpec) (bool, error) { return true, nil },
			tableColumns: func(context.Context, *sql.DB, extraArchiveTableSpec) ([]string, error) {
				return nil, wantErr
			},
			tableCount: func(context.Context, *sql.DB, extraArchiveTableSpec) (int64, error) { return 0, nil },
		}
		_, err := inspectExtraArchiveTablesWithOps(context.Background(), source, []extraArchiveTableSpec{spec}, ops)
		if !errors.Is(err, wantErr) {
			t.Fatalf("schema error = %v, want %v", err, wantErr)
		}
	})

	t.Run("count", func(t *testing.T) {
		wantErr := errors.New("count failed")
		ops := extraArchivePreflightOps{
			tableExists: func(context.Context, *sql.DB, extraArchiveTableSpec) (bool, error) { return true, nil },
			tableColumns: func(context.Context, *sql.DB, extraArchiveTableSpec) ([]string, error) {
				return append([]string(nil), spec.columns...), nil
			},
			tableCount: func(context.Context, *sql.DB, extraArchiveTableSpec) (int64, error) { return 0, wantErr },
		}
		_, err := inspectExtraArchiveTablesWithOps(context.Background(), source, []extraArchiveTableSpec{spec}, ops)
		if !errors.Is(err, wantErr) {
			t.Fatalf("count error = %v, want %v", err, wantErr)
		}
	})
}

func TestMigrationTargetIsSafeAcceptsEmptyKnownExtraTables(t *testing.T) {
	target := openExtraArchiveGORMTarget(t)
	for _, spec := range extraArchiveTableSpecs {
		if err := target.Exec(fmt.Sprintf(`CREATE TABLE %s (marker INTEGER)`, quoteSQLIdentifier(spec.name))).Error; err != nil {
			t.Fatalf("create empty target table %s: %v", spec.name, err)
		}
	}

	safe, err := migrationTargetIsSafe(context.Background(), target)
	if err != nil || !safe {
		t.Fatalf("migrationTargetIsSafe() = %v, %v; want true, nil", safe, err)
	}
}

func TestMigrationTargetIsSafeRejectsEveryNonemptyKnownExtraTable(t *testing.T) {
	for _, spec := range extraArchiveTableSpecs {
		spec := spec
		t.Run(spec.name, func(t *testing.T) {
			target := openExtraArchiveGORMTarget(t)
			if err := target.Exec(fmt.Sprintf(`CREATE TABLE %s (marker INTEGER)`, quoteSQLIdentifier(spec.name))).Error; err != nil {
				t.Fatal(err)
			}
			if err := target.Exec(fmt.Sprintf(`INSERT INTO %s (marker) VALUES (1)`, quoteSQLIdentifier(spec.name))).Error; err != nil {
				t.Fatal(err)
			}

			safe, err := migrationTargetIsSafe(context.Background(), target)
			if err != nil {
				t.Fatalf("migrationTargetIsSafe() error = %v", err)
			}
			if safe {
				t.Fatalf("nonempty known extra table %s was accepted", spec.name)
			}
		})
	}
}

func TestSyncProductionFailClosedOrderingAndCheckedOperations(t *testing.T) {
	contents, err := osReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(contents)
	if strings.Contains(source, ".Columns()") {
		t.Fatal("sync-production must not derive identifiers from rows.Columns()")
	}
	if strings.Contains(source, "Warning: failed to sync extra archive tables") {
		t.Fatal("extra archive failures must not be reduced to warnings")
	}

	runBody := goFunctionSource(t, source, "func runSyncProduction(")
	preflight := strings.Index(runBody, "inspectExtraArchiveSource(")
	targetConnect := strings.Index(runBody, "gorm.Open(postgres.Open(")
	coreMigration := strings.Index(runBody, "db.MigrateLegacySQLite(")
	lastSafety := strings.LastIndex(runBody[:coreMigration], "migrationTargetIsSafe(ctx, targetDB)")
	if preflight < 0 || targetConnect < 0 || preflight > targetConnect {
		t.Fatalf("source extra preflight must occur before target connection: preflight=%d target=%d", preflight, targetConnect)
	}
	if strings.Count(runBody, "migrationTargetIsSafe(ctx, targetDB)") < 2 || lastSafety < 0 || lastSafety > coreMigration {
		t.Fatalf("combined target safety must be checked initially and immediately before core migration")
	}

	syncCall := strings.Index(runBody, "extraCopied, err := syncExtraTables(")
	completion := strings.Index(runBody, "SYNC COMPLETE")
	if syncCall < 0 || completion < 0 || syncCall > completion {
		t.Fatalf("extra sync must precede completion logging: sync=%d complete=%d", syncCall, completion)
	}

	syncBody := goFunctionSource(t, source, "func syncExtraTable(")
	begin := strings.Index(syncBody, "target.WithContext(ctx).Begin()")
	create := strings.Index(syncBody, "tx.Exec(spec.targetDDL)")
	lock := strings.Index(syncBody, "tx.Exec(spec.targetLockSQL())")
	truncate := strings.Index(syncBody, "tx.Exec(spec.targetTruncateSQL())")
	if begin < 0 || create < 0 || lock < 0 || truncate < 0 || !(begin < create && create < lock && lock < truncate) {
		t.Fatalf("CREATE, LOCK, and TRUNCATE must occur after transaction begin: begin=%d create=%d lock=%d truncate=%d", begin, create, lock, truncate)
	}
	for _, required := range []string{
		"tx.Error",
		"finishExtraArchiveRows(rows",
		"copied != table.sourceCount",
		"targetCount != table.sourceCount",
		"tx.Rollback().Error",
		"tx.Commit().Error",
	} {
		if !strings.Contains(syncBody, required) {
			t.Fatalf("syncExtraTable is missing required checked operation %q", required)
		}
	}
	finishRowsBody := goFunctionSource(t, source, "func finishExtraArchiveRows(")
	for _, required := range []string{"rows.Err()", "rows.Close()"} {
		if !strings.Contains(finishRowsBody, required) {
			t.Fatalf("finishExtraArchiveRows is missing required checked operation %q", required)
		}
	}
}

func openExtraArchiveTestSQLite(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "source.db")
	source, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	source.SetMaxOpenConns(1)
	if err := source.Ping(); err != nil {
		source.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = source.Close() })
	return source
}

func openExtraArchiveGORMTarget(t *testing.T) *gorm.DB {
	t.Helper()
	target, err := gorm.Open(gormsqlite.Open(filepath.Join(t.TempDir(), "target.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := target.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return target
}

func extraArchiveSpecByName(t *testing.T, name string) extraArchiveTableSpec {
	t.Helper()
	for _, spec := range extraArchiveTableSpecs {
		if spec.name == name {
			return spec
		}
	}
	t.Fatalf("extra archive table spec %q not found", name)
	return extraArchiveTableSpec{}
}

func goFunctionSource(t *testing.T, source, signature string) string {
	t.Helper()
	start := strings.Index(source, signature)
	if start < 0 {
		t.Fatalf("function signature %q not found", signature)
	}
	rest := source[start+len(signature):]
	if next := strings.Index(rest, "\nfunc "); next >= 0 {
		return source[start : start+len(signature)+next]
	}
	return source[start:]
}

var osReadFile = func(path string) ([]byte, error) {
	return os.ReadFile(path)
}
