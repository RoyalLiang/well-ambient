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
	if err := conn.Exec(`CREATE TABLE IF NOT EXISTS read_model_generations (
		dataset TEXT PRIMARY KEY,
		generation INTEGER NOT NULL DEFAULT 1,
		updated_unix INTEGER NOT NULL
	)`).Error; err != nil {
		return fmt.Errorf("create read generation table: %w", err)
	}
	for _, dataset := range datasets {
		if err := validateDataset(dataset); err != nil {
			return err
		}
		tracked := 0
		for _, table := range dataset.Tables {
			exists, err := sqliteTableExists(conn, table)
			if err != nil {
				return err
			}
			if !exists {
				continue
			}
			tracked++
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

func sqliteTableExists(conn *gorm.DB, table string) (bool, error) {
	var count int64
	if err := conn.Raw(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func sqliteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
