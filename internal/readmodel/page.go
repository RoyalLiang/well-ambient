package readmodel

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrStaleCursor = errors.New("read cursor belongs to an older dataset generation")

var identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type Dataset struct {
	Name   string
	Tables []string
}

type PageRequest struct {
	Dataset      string
	Contract     string
	Scope        any
	Cursor       string
	Limit        string
	DefaultLimit int
	MaxLimit     int
}

type PageWindow struct {
	Limit      int
	Generation uint64
	HasCursor  bool
	codec      CursorCodec
}

func EnsureDatasets(conn *gorm.DB, datasets []Dataset) error {
	if conn == nil {
		return fmt.Errorf("read model database is not configured")
	}
	generationType := "INTEGER"
	if conn.Dialector.Name() == "postgres" {
		generationType = "BIGINT"
	}
	if err := conn.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS read_model_generations (
		dataset TEXT PRIMARY KEY,
		generation %s NOT NULL DEFAULT 1,
		updated_unix %s NOT NULL
	)`, generationType, generationType)).Error; err != nil {
		return fmt.Errorf("create read generation table: %w", err)
	}
	for _, dataset := range datasets {
		if err := validateDataset(dataset); err != nil {
			return err
		}
		tracked := 0
		for _, table := range dataset.Tables {
			if !conn.Migrator().HasTable(table) {
				continue
			}
			tracked++
			if conn.Dialector.Name() == "postgres" {
				if err := ensurePostgresGenerationTrigger(conn, dataset.Name, table); err != nil {
					return err
				}
				continue
			}
			for _, operation := range []struct {
				suffix string
				timing string
			}{
				{suffix: "ai", timing: "INSERT"},
				{suffix: "au", timing: "UPDATE"},
				{suffix: "ad", timing: "DELETE"},
			} {
				trigger := "read_generation_" + dataset.Name + "_" + table + "_" + operation.suffix
				if err := conn.Exec("DROP TRIGGER IF EXISTS \"" + trigger + "\"").Error; err != nil {
					return fmt.Errorf("replace read generation trigger %s: %w", trigger, err)
				}
				sql := fmt.Sprintf(`CREATE TRIGGER "%s" AFTER %s ON "%s"
					BEGIN
						INSERT INTO read_model_generations(dataset, generation, updated_unix)
						VALUES (%s, 1, unixepoch())
						ON CONFLICT(dataset) DO UPDATE SET
							generation = generation + 1,
							updated_unix = unixepoch();
					END`, trigger, operation.timing, table, sqliteLiteral(dataset.Name))
				if err := conn.Exec(sql).Error; err != nil {
					return fmt.Errorf("create read generation trigger %s: %w", trigger, err)
				}
			}
		}
		if tracked == 0 {
			continue
		}
		if err := conn.Exec(`INSERT INTO read_model_generations(dataset, generation, updated_unix)
			VALUES (?, 1, ?)
			ON CONFLICT(dataset) DO NOTHING`, dataset.Name, time.Now().UTC().Unix()).Error; err != nil {
			return fmt.Errorf("seed read generation %s: %w", dataset.Name, err)
		}
	}
	return nil
}

// VerifyDatasets is the read-only startup gate used after a separate migration
// job. It proves every declared dataset has a generation row without silently
// creating tables, functions, or triggers in the runtime process.
func VerifyDatasets(conn *gorm.DB, datasets []Dataset) error {
	if conn == nil {
		return fmt.Errorf("read model database is not configured")
	}
	if !conn.Migrator().HasTable("read_model_generations") {
		return fmt.Errorf("read model generations are not migrated")
	}
	for _, dataset := range datasets {
		if err := validateDataset(dataset); err != nil {
			return err
		}
		for _, table := range dataset.Tables {
			if !conn.Migrator().HasTable(table) {
				return fmt.Errorf("read dataset %q is missing table %q", dataset.Name, table)
			}
			if conn.Dialector.Name() == "postgres" {
				triggerName := "read_generation_" + dataset.Name + "_" + table
				var triggerCount int64
				if err := conn.Raw(`SELECT COUNT(*)
					FROM pg_trigger trigger
					JOIN pg_class relation ON relation.oid = trigger.tgrelid
					JOIN pg_namespace namespace ON namespace.oid = relation.relnamespace
					WHERE namespace.nspname = current_schema()
						AND relation.relname = ?
						AND trigger.tgname = ?
						AND NOT trigger.tgisinternal`, table, triggerName).Scan(&triggerCount).Error; err != nil {
					return fmt.Errorf("verify read generation trigger %s: %w", triggerName, err)
				}
				if triggerCount != 1 {
					return fmt.Errorf("read dataset %q is missing generation trigger for %q", dataset.Name, table)
				}
			}
		}
		var count int64
		if err := conn.Raw("SELECT COUNT(*) FROM read_model_generations WHERE dataset = ?", dataset.Name).Scan(&count).Error; err != nil {
			return fmt.Errorf("verify read generation %s: %w", dataset.Name, err)
		}
		if count != 1 {
			return fmt.Errorf("read dataset %q is not migrated", dataset.Name)
		}
	}
	return nil
}

func ensurePostgresGenerationTrigger(conn *gorm.DB, dataset, table string) error {
	functionName := "read_generation_bump_" + dataset + "_" + table
	triggerName := "read_generation_" + dataset + "_" + table
	functionSQL := fmt.Sprintf(`CREATE OR REPLACE FUNCTION "%s"() RETURNS trigger AS $$
		BEGIN
			INSERT INTO read_model_generations(dataset, generation, updated_unix)
			VALUES (%s, 1, EXTRACT(EPOCH FROM NOW())::BIGINT)
			ON CONFLICT(dataset) DO UPDATE SET
				generation = read_model_generations.generation + 1,
				updated_unix = EXTRACT(EPOCH FROM NOW())::BIGINT;
			RETURN NULL;
		END;
		$$ LANGUAGE plpgsql`, functionName, sqlLiteral(dataset))
	if err := conn.Exec(functionSQL).Error; err != nil {
		return fmt.Errorf("create PostgreSQL read generation function %s: %w", functionName, err)
	}
	if err := conn.Exec(fmt.Sprintf(`DROP TRIGGER IF EXISTS "%s" ON "%s"`, triggerName, table)).Error; err != nil {
		return fmt.Errorf("replace PostgreSQL read generation trigger %s: %w", triggerName, err)
	}
	triggerSQL := fmt.Sprintf(`CREATE TRIGGER "%s" AFTER INSERT OR UPDATE OR DELETE ON "%s"
		FOR EACH STATEMENT EXECUTE FUNCTION "%s"()`, triggerName, table, functionName)
	if err := conn.Exec(triggerSQL).Error; err != nil {
		return fmt.Errorf("create PostgreSQL read generation trigger %s: %w", triggerName, err)
	}
	return nil
}

func OpenPage(ctx context.Context, conn *gorm.DB, request PageRequest, position any) (PageWindow, error) {
	if conn == nil {
		return PageWindow{}, fmt.Errorf("read model database is not configured")
	}
	if !identifierPattern.MatchString(request.Dataset) {
		return PageWindow{}, fmt.Errorf("invalid read dataset %q", request.Dataset)
	}
	limit, err := NormalizeLimit(request.Limit, request.DefaultLimit, request.MaxLimit)
	if err != nil {
		return PageWindow{}, err
	}
	codec, err := NewCursorCodec(request.Contract, request.Scope)
	if err != nil {
		return PageWindow{}, err
	}
	generation, err := CurrentGeneration(ctx, conn, request.Dataset)
	if err != nil {
		return PageWindow{}, err
	}
	window := PageWindow{Limit: limit, Generation: generation, codec: codec}
	if strings.TrimSpace(request.Cursor) == "" {
		return window, nil
	}
	watermark, err := codec.Decode(request.Cursor, position)
	if err != nil {
		return PageWindow{}, err
	}
	cursorGeneration, err := strconv.ParseUint(watermark, 10, 64)
	if err != nil {
		return PageWindow{}, ErrInvalidCursor
	}
	if cursorGeneration != generation {
		return PageWindow{}, ErrStaleCursor
	}
	window.HasCursor = true
	return window, nil
}

func (window PageWindow) Next(position any) (string, error) {
	if window.Generation == 0 {
		return "", fmt.Errorf("page window is not initialized")
	}
	return window.codec.Encode(strconv.FormatUint(window.Generation, 10), position)
}

func CurrentGeneration(ctx context.Context, conn *gorm.DB, dataset string) (uint64, error) {
	if conn == nil || !identifierPattern.MatchString(dataset) {
		return 0, fmt.Errorf("invalid read generation request")
	}
	var row struct {
		Generation uint64
	}
	result := conn.WithContext(ctx).Raw(
		"SELECT generation FROM read_model_generations WHERE dataset = ? LIMIT 1",
		dataset,
	).Scan(&row)
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 || row.Generation == 0 {
		return 0, fmt.Errorf("read dataset %q is not initialized", dataset)
	}
	return row.Generation, nil
}

func validateDataset(dataset Dataset) error {
	if !identifierPattern.MatchString(dataset.Name) {
		return fmt.Errorf("invalid read dataset name %q", dataset.Name)
	}
	if len(dataset.Tables) == 0 {
		return fmt.Errorf("read dataset %q has no source tables", dataset.Name)
	}
	for _, table := range dataset.Tables {
		if !identifierPattern.MatchString(table) {
			return fmt.Errorf("invalid source table %q for read dataset %q", table, dataset.Name)
		}
	}
	return nil
}

func sqliteLiteral(value string) string {
	return sqlLiteral(value)
}

func sqlLiteral(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
