package db

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestInspectLegacySQLiteValidatesFrozenSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	source := openLegacyMigrationFixture(t, path)
	closeTestConnection(t, source)

	snapshot, err := InspectLegacySQLite(path)
	if err != nil {
		t.Fatalf("InspectLegacySQLite() error = %v", err)
	}
	if snapshot.Path != path || snapshot.SizeBytes <= 0 || snapshot.TableCount < 2 {
		t.Fatalf("unexpected snapshot facts: %#v", snapshot)
	}
	if _, err := InspectLegacySQLite("relative.db"); err == nil {
		t.Fatal("relative legacy path was accepted")
	}
}

func TestMigrateLegacySQLiteCopiesOwnedRowsAndIgnoresUnknownTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	source := openLegacyMigrationFixture(t, path)
	if err := source.Exec(`CREATE TABLE unknown_legacy_cache (id INTEGER PRIMARY KEY, value TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := source.Exec(`INSERT INTO unknown_legacy_cache (value) VALUES ('do not copy')`).Error; err != nil {
		t.Fatal(err)
	}
	closeTestConnection(t, source)

	target, err := gorm.Open(sqlite.Open("file:legacy_migration_target?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	var observed []LegacyMigrationProgress
	report, err := MigrateLegacySQLite(context.Background(), path, target, nil, func(progress LegacyMigrationProgress) {
		observed = append(observed, progress)
	})
	if err != nil {
		t.Fatalf("MigrateLegacySQLite() error = %v", err)
	}
	if report.TableRows["task_telemetries"] != 1 || report.TableRows["webhook_logs"] != 1 || report.RowsCopied != 2 {
		t.Fatalf("unexpected migration report: %#v", report)
	}
	var task TaskTelemetry
	if err := target.First(&task, "task_id = ?", "FZ-2257").Error; err != nil || task.Title != "preserved" {
		t.Fatalf("migrated task = %#v, err = %v", task, err)
	}
	if target.Migrator().HasTable("unknown_legacy_cache") {
		t.Fatal("unknown legacy table was copied")
	}
	if len(observed) == 0 || observed[len(observed)-1].Stage != "completed" {
		t.Fatalf("migration progress did not complete: %#v", observed)
	}
}

func TestMigrateLegacySQLiteRepairsPostgresIncompatibleText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-invalid-utf8.db")
	source, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.AutoMigrate(&Notification{}); err != nil {
		t.Fatal(err)
	}
	invalidMessage := string(append([]byte("truncated message "), 0xe5, 0x8c, 0x00))
	if err := source.Create(&Notification{Type: "semantic_link_review", Message: invalidMessage}).Error; err != nil {
		t.Fatal(err)
	}
	closeTestConnection(t, source)

	target, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "target.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := MigrateLegacySQLite(context.Background(), path, target, nil, nil)
	if err != nil {
		t.Fatalf("MigrateLegacySQLite() error = %v", err)
	}

	var migrated Notification
	if err := target.First(&migrated).Error; err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(migrated.Message) {
		t.Fatalf("migrated message remains invalid UTF-8: %x", []byte(migrated.Message))
	}
	if strings.ContainsRune(migrated.Message, '\x00') {
		t.Fatalf("migrated message contains PostgreSQL-incompatible NUL: %x", []byte(migrated.Message))
	}
	if report.TextValuesRepaired != 1 {
		t.Fatalf("TextValuesRepaired = %d, want 1", report.TextValuesRepaired)
	}
}

func TestMigrateLegacySQLiteRollsBackSchemaAndRowsWhenFinalizationFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	source := openLegacyMigrationFixture(t, path)
	closeTestConnection(t, source)
	target, err := gorm.Open(sqlite.Open("file:legacy_migration_rollback?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = MigrateLegacySQLite(context.Background(), path, target, func(*gorm.DB) error {
		return errors.New("forced read-model failure")
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "forced read-model failure") {
		t.Fatalf("expected finalization failure, got %v", err)
	}
	if target.Migrator().HasTable(&TaskTelemetry{}) {
		t.Fatal("failed migration left schema or copied rows outside the transaction")
	}
}

func TestInspectLegacySQLiteRejectsNonDatabaseFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-database.db")
	if err := os.WriteFile(path, []byte("not sqlite"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectLegacySQLite(path); err == nil {
		t.Fatal("invalid SQLite file was accepted")
	}
}

func TestMigrateLegacySQLiteExternalSnapshot(t *testing.T) {
	path := os.Getenv("WELL_AMBIENT_LEGACY_SQLITE_TEST_PATH")
	if path == "" {
		t.Skip("set WELL_AMBIENT_LEGACY_SQLITE_TEST_PATH to run a full local snapshot simulation")
	}
	targetPath := filepath.Join(t.TempDir(), "simulated-postgres-target.db")
	target, err := gorm.Open(sqlite.Open(targetPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	report, err := MigrateLegacySQLite(context.Background(), path, target, nil, nil)
	if err != nil {
		t.Fatalf("full snapshot migration simulation failed: %v", err)
	}
	if report.TablesCopied == 0 || report.RowsCopied == 0 {
		t.Fatalf("full snapshot simulation copied no application data: %#v", report)
	}
	t.Logf(
		"full snapshot simulation copied %d rows across %d tables and repaired %d text values",
		report.RowsCopied,
		report.TablesCopied,
		report.TextValuesRepaired,
	)
}

func openLegacyMigrationFixture(t *testing.T, path string) *gorm.DB {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.AutoMigrate(&TaskTelemetry{}, &WebhookLog{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	if err := conn.Create(&TaskTelemetry{TaskID: "FZ-2257", ProjectKey: "FZ", Source: "jira", Title: "preserved", LastUpdate: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Create(&WebhookLog{Event: "push", Payload: `{"safe":true}`, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	return conn
}

func closeTestConnection(t *testing.T, conn *gorm.DB) {
	t.Helper()
	if err := CloseConnection(conn); err != nil {
		t.Fatal(err)
	}
}
