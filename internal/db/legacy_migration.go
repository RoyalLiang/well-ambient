package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const legacyMigrationBatchSize = 500

type LegacySQLiteSnapshot struct {
	Path       string `json:"-"`
	SizeBytes  int64  `json:"size_bytes"`
	TableCount int    `json:"table_count"`
}

type LegacyMigrationProgress struct {
	Stage           string `json:"stage"`
	Table           string `json:"table,omitempty"`
	TablesCompleted int    `json:"tables_completed"`
	TablesTotal     int    `json:"tables_total"`
	RowsCopied      int64  `json:"rows_copied"`
}

type LegacyMigrationReport struct {
	TablesCopied       int              `json:"tables_copied"`
	RowsCopied         int64            `json:"rows_copied"`
	TextValuesRepaired int64            `json:"text_values_repaired"`
	TableRows          map[string]int64 `json:"table_rows"`
}

type legacyTableCopy struct {
	model any
	name  string
	rows  int64
}

// InspectLegacySQLite validates a server-controlled, frozen SQLite snapshot.
// The setup API exposes only size and table count; the filesystem path never
// leaves the server-side configuration.
func InspectLegacySQLite(path string) (LegacySQLiteSnapshot, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return LegacySQLiteSnapshot{}, os.ErrNotExist
	}
	if !strings.HasPrefix(cleanPath, "/") {
		return LegacySQLiteSnapshot{}, errors.New("legacy SQLite path must be absolute")
	}
	info, err := os.Stat(cleanPath)
	if err != nil {
		return LegacySQLiteSnapshot{}, err
	}
	if !info.Mode().IsRegular() {
		return LegacySQLiteSnapshot{}, errors.New("legacy SQLite snapshot is not a regular file")
	}

	conn, err := sql.Open("sqlite3", legacySQLiteReadOnlyDSN(cleanPath))
	if err != nil {
		return LegacySQLiteSnapshot{}, err
	}
	defer conn.Close()
	if err := conn.Ping(); err != nil {
		return LegacySQLiteSnapshot{}, fmt.Errorf("open legacy SQLite snapshot: %w", err)
	}
	rows, err := conn.Query("PRAGMA quick_check")
	if err != nil {
		return LegacySQLiteSnapshot{}, fmt.Errorf("check legacy SQLite snapshot: %w", err)
	}
	quickCheckOK := true
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			rows.Close()
			return LegacySQLiteSnapshot{}, err
		}
		if result != "ok" {
			quickCheckOK = false
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return LegacySQLiteSnapshot{}, err
	}
	if err := rows.Close(); err != nil {
		return LegacySQLiteSnapshot{}, err
	}
	if !quickCheckOK {
		return LegacySQLiteSnapshot{}, errors.New("legacy SQLite quick_check failed")
	}

	foreignKeys, err := conn.Query("PRAGMA foreign_key_check")
	if err != nil {
		return LegacySQLiteSnapshot{}, fmt.Errorf("check legacy SQLite foreign keys: %w", err)
	}
	foreignKeyViolation := foreignKeys.Next()
	if err := foreignKeys.Close(); err != nil {
		return LegacySQLiteSnapshot{}, err
	}
	if foreignKeyViolation {
		return LegacySQLiteSnapshot{}, errors.New("legacy SQLite foreign_key_check failed")
	}

	var tableCount int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`).Scan(&tableCount); err != nil {
		return LegacySQLiteSnapshot{}, fmt.Errorf("count legacy SQLite tables: %w", err)
	}
	return LegacySQLiteSnapshot{Path: cleanPath, SizeBytes: info.Size(), TableCount: tableCount}, nil
}

func legacySQLiteReadOnlyDSN(path string) string {
	fileURL := &url.URL{Scheme: "file", Path: path}
	query := fileURL.Query()
	query.Set("mode", "ro")
	query.Set("immutable", "1")
	query.Set("_foreign_keys", "on")
	query.Set("_busy_timeout", "5000")
	fileURL.RawQuery = query.Encode()
	return fileURL.String()
}

// MigrateLegacySQLite copies only application-owned source tables into a
// transactionally initialized target. Row counts are verified before the
// transaction commits; any mismatch leaves the target schema unchanged.
func MigrateLegacySQLite(
	ctx context.Context,
	path string,
	target *gorm.DB,
	finalize func(*gorm.DB) error,
	progress func(LegacyMigrationProgress),
) (LegacyMigrationReport, error) {
	if target == nil {
		return LegacyMigrationReport{}, gorm.ErrInvalidDB
	}
	if _, err := InspectLegacySQLite(path); err != nil {
		return LegacyMigrationReport{}, err
	}
	source, err := gorm.Open(sqlite.Open(legacySQLiteReadOnlyDSN(path)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return LegacyMigrationReport{}, fmt.Errorf("open legacy SQLite source: %w", err)
	}
	defer CloseConnection(source)

	tables, err := legacyCopyPlan(source)
	if err != nil {
		return LegacyMigrationReport{}, err
	}
	report := LegacyMigrationReport{TableRows: make(map[string]int64, len(tables))}
	update := func(item LegacyMigrationProgress) {
		if progress != nil {
			progress(item)
		}
	}
	update(LegacyMigrationProgress{Stage: "preparing_schema", TablesTotal: len(tables)})

	err = target.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := MigrateSchema(tx); err != nil {
			return fmt.Errorf("create PostgreSQL schema: %w", err)
		}
		var copiedRows int64
		for tableIndex, table := range tables {
			update(LegacyMigrationProgress{
				Stage: "copying_data", Table: table.name, TablesCompleted: tableIndex,
				TablesTotal: len(tables), RowsCopied: copiedRows,
			})
			rows, repaired, err := copyLegacyTable(ctx, source, tx, table)
			if err != nil {
				return err
			}
			copiedRows += rows
			report.TextValuesRepaired += repaired
			report.TableRows[table.name] = rows
			report.TablesCopied++
			report.RowsCopied = copiedRows
			update(LegacyMigrationProgress{
				Stage: "copying_data", Table: table.name, TablesCompleted: tableIndex + 1,
				TablesTotal: len(tables), RowsCopied: copiedRows,
			})
		}

		update(LegacyMigrationProgress{Stage: "verifying_counts", TablesCompleted: len(tables), TablesTotal: len(tables), RowsCopied: copiedRows})
		for _, table := range tables {
			var targetCount int64
			if err := tx.Table(table.name).Count(&targetCount).Error; err != nil {
				return fmt.Errorf("count target table %s: %w", table.name, err)
			}
			if targetCount != table.rows {
				return fmt.Errorf("verify table %s: source rows %d, target rows %d", table.name, table.rows, targetCount)
			}
		}
		if err := resetPostgresSequences(tx, tables); err != nil {
			return err
		}
		if err := InitializeReferenceData(tx); err != nil {
			return fmt.Errorf("initialize reference data: %w", err)
		}
		if finalize != nil {
			if err := finalize(tx); err != nil {
				return fmt.Errorf("finalize PostgreSQL read models: %w", err)
			}
		}
		if tx.Dialector.Name() == "postgres" {
			update(LegacyMigrationProgress{Stage: "analyzing", TablesCompleted: len(tables), TablesTotal: len(tables), RowsCopied: report.RowsCopied})
			if err := tx.Exec("ANALYZE").Error; err != nil {
				return fmt.Errorf("analyze migrated PostgreSQL database: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return LegacyMigrationReport{}, fmt.Errorf("migrate legacy SQLite snapshot: %w", err)
	}
	update(LegacyMigrationProgress{Stage: "completed", TablesCompleted: len(tables), TablesTotal: len(tables), RowsCopied: report.RowsCopied})
	return report, nil
}

func legacyCopyPlan(source *gorm.DB) ([]legacyTableCopy, error) {
	models := RequiredSchemaModels()
	plan := make([]legacyTableCopy, 0, len(models))
	for _, model := range models {
		statement := &gorm.Statement{DB: source}
		if err := statement.Parse(model); err != nil {
			return nil, fmt.Errorf("parse legacy model: %w", err)
		}
		table := statement.Schema.Table
		if !source.Migrator().HasTable(table) {
			continue
		}
		var count int64
		if err := source.Table(table).Count(&count).Error; err != nil {
			return nil, fmt.Errorf("count legacy table %s: %w", table, err)
		}
		plan = append(plan, legacyTableCopy{model: model, name: table, rows: count})
	}
	return plan, nil
}

func copyLegacyTable(ctx context.Context, source, target *gorm.DB, table legacyTableCopy) (int64, int64, error) {
	modelType := reflect.TypeOf(table.model)
	if modelType.Kind() != reflect.Pointer || modelType.Elem().Kind() != reflect.Struct {
		return 0, 0, fmt.Errorf("legacy model for %s is not a struct pointer", table.name)
	}
	slice := reflect.New(reflect.SliceOf(modelType.Elem()))
	var copied int64
	var repaired int64
	err := source.WithContext(ctx).Table(table.name).FindInBatches(slice.Interface(), legacyMigrationBatchSize, func(_ *gorm.DB, _ int) error {
		batchSize := slice.Elem().Len()
		if batchSize == 0 {
			return nil
		}
		repaired += int64(repairLegacyTextValues(slice.Interface()))
		if err := target.WithContext(ctx).Table(table.name).Create(slice.Interface()).Error; err != nil {
			return fmt.Errorf("copy legacy table %s: %w", table.name, err)
		}
		copied += int64(batchSize)
		return nil
	}).Error
	if err != nil {
		return 0, 0, err
	}
	if copied != table.rows {
		return 0, 0, fmt.Errorf("read legacy table %s: expected %d rows, copied %d", table.name, table.rows, copied)
	}
	return copied, repaired, nil
}

func repairLegacyTextValues(batch any) int {
	value := reflect.ValueOf(batch)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return 0
	}
	value = value.Elem()
	if value.Kind() != reflect.Slice {
		return 0
	}

	repaired := 0
	for index := 0; index < value.Len(); index++ {
		repaired += repairLegacyTextStruct(value.Index(index))
	}
	return repaired
}

func repairLegacyTextStruct(value reflect.Value) int {
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return 0
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return 0
	}

	repaired := 0
	valueType := value.Type()
	for index := 0; index < value.NumField(); index++ {
		field := value.Field(index)
		fieldType := valueType.Field(index)
		if !field.CanSet() {
			continue
		}
		if field.Kind() == reflect.String {
			original := field.String()
			normalized := strings.ToValidUTF8(original, "\uFFFD")
			normalized = strings.ReplaceAll(normalized, "\x00", "\uFFFD")
			if normalized != original {
				field.SetString(normalized)
				repaired++
			}
			continue
		}
		if fieldType.Anonymous {
			repaired += repairLegacyTextStruct(field)
		}
	}
	return repaired
}

func resetPostgresSequences(target *gorm.DB, tables []legacyTableCopy) error {
	if target.Dialector.Name() != "postgres" {
		return nil
	}
	for _, table := range tables {
		statement := &gorm.Statement{DB: target}
		if err := statement.Parse(table.model); err != nil {
			return err
		}
		if len(statement.Schema.PrimaryFields) != 1 || !statement.Schema.PrimaryFields[0].AutoIncrement {
			continue
		}
		column := statement.Schema.PrimaryFields[0].DBName
		var sequence sql.NullString
		if err := target.Raw("SELECT pg_get_serial_sequence(?, ?)", table.name, column).Scan(&sequence).Error; err != nil {
			return fmt.Errorf("find sequence for %s.%s: %w", table.name, column, err)
		}
		if !sequence.Valid || sequence.String == "" {
			continue
		}
		quotedTable := quotePostgresIdentifier(table.name)
		quotedColumn := quotePostgresIdentifier(column)
		query := fmt.Sprintf(
			`SELECT setval(?::regclass, COALESCE((SELECT MAX(%s) FROM %s), 1), EXISTS(SELECT 1 FROM %s))`,
			quotedColumn, quotedTable, quotedTable,
		)
		if err := target.Exec(query, sequence.String).Error; err != nil {
			return fmt.Errorf("reset sequence for %s.%s: %w", table.name, column, err)
		}
	}
	return nil
}

func quotePostgresIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
