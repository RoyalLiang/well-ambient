package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"well-ambient/internal/dailyjira"
	"well-ambient/internal/db"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type benchmarkConfig struct {
	Rows      int
	Samples   int
	PageLimit int
	Keep      bool
}

type anchor struct {
	SortOverdue  int
	CreatedUnix  int64
	ActivityUnix int64
	TaskID       string
}

type distribution struct {
	Project  string
	Assignee string
	Bucket   string
}

type latencySummary struct {
	P50 time.Duration
	P95 time.Duration
	P99 time.Duration
	Max time.Duration
}

func main() {
	cfg := benchmarkConfig{}
	flag.IntVar(&cfg.Rows, "rows", 10_000_000, "number of projected Daily Jira rows")
	flag.IntVar(&cfg.Samples, "samples", 500, "warm-query samples per scenario")
	flag.IntVar(&cfg.PageLimit, "limit", 100, "bounded page size")
	flag.BoolVar(&cfg.Keep, "keep", false, "keep the disposable benchmark database")
	flag.Parse()
	if cfg.Rows <= 0 || cfg.Samples <= 0 || cfg.PageLimit <= 0 || cfg.PageLimit > 100 {
		fatalf("rows and samples must be positive; limit must be between 1 and 100")
	}

	tempFile, err := os.CreateTemp("", "well-ambient-daily-jira-bench-*.db")
	if err != nil {
		fatalf("create benchmark database: %v", err)
	}
	dbPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		fatalf("close benchmark database placeholder: %v", err)
	}
	if !cfg.Keep {
		defer os.Remove(dbPath)
	}

	conn, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fatalf("open benchmark database: %v", err)
	}
	sqlDB, err := conn.DB()
	if err != nil {
		fatalf("unwrap benchmark database: %v", err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode=OFF",
		"PRAGMA synchronous=OFF",
		"PRAGMA temp_store=MEMORY",
		"PRAGMA cache_size=-524288",
		"PRAGMA mmap_size=1073741824",
	} {
		if _, err := sqlDB.Exec(pragma); err != nil {
			fatalf("apply %s: %v", pragma, err)
		}
	}
	if err := conn.AutoMigrate(&db.TaskTelemetry{}); err != nil {
		fatalf("migrate benchmark task source: %v", err)
	}
	if err := dailyjira.Migrate(conn); err != nil {
		fatalf("migrate benchmark read model: %v", err)
	}
	prepareForBulkSeed(sqlDB)

	seedStarted := time.Now()
	counts := seedEntries(sqlDB, cfg.Rows)
	seedCounts(sqlDB, counts)
	seedDuration := time.Since(seedStarted)

	indexStarted := time.Now()
	if err := dailyjira.Migrate(conn); err != nil {
		fatalf("build benchmark indexes: %v", err)
	}
	indexDuration := time.Since(indexStarted)
	if _, err := sqlDB.Exec("ANALYZE"); err != nil {
		fatalf("analyze benchmark database: %v", err)
	}

	reader := dailyjira.NewReader(conn)
	query := dailyjira.Query{Bucket: dailyjira.BucketSevenDay, Limit: cfg.PageLimit, Now: time.Now()}
	firstPage, err := reader.ReadPage(context.Background(), query)
	if err != nil {
		fatalf("warm first page: %v", err)
	}
	if firstPage.NextCursor == "" {
		fatalf("benchmark data did not produce a continuation cursor")
	}
	secondQuery := query
	secondQuery.Cursor = firstPage.NextCursor
	searchTaskQuery := query
	searchTaskQuery.Search = fmt.Sprintf("PERF-%08d", cfg.Rows-1)
	searchTitleQuery := query
	searchTitleQuery.Search = fmt.Sprintf("performance issue %08d", cfg.Rows-1)

	for range 20 {
		if _, err := reader.ReadPage(context.Background(), query); err != nil {
			fatalf("warm reader: %v", err)
		}
		if _, err := reader.ReadPage(context.Background(), secondQuery); err != nil {
			fatalf("warm cursor reader: %v", err)
		}
		if _, err := reader.ReadPage(context.Background(), searchTaskQuery); err != nil {
			fatalf("warm task search: %v", err)
		}
		if _, err := reader.ReadPage(context.Background(), searchTitleQuery); err != nil {
			fatalf("warm title search: %v", err)
		}
	}
	firstLatency := measure(cfg.Samples, func() error {
		_, err := reader.ReadPage(context.Background(), query)
		return err
	})
	secondLatency := measure(cfg.Samples, func() error {
		_, err := reader.ReadPage(context.Background(), secondQuery)
		return err
	})
	taskSearchLatency := measure(cfg.Samples, func() error {
		_, err := reader.ReadPage(context.Background(), searchTaskQuery)
		return err
	})
	titleSearchLatency := measure(cfg.Samples, func() error {
		_, err := reader.ReadPage(context.Background(), searchTitleQuery)
		return err
	})

	sevenDayRows := int64(float64(cfg.Rows) * 0.7)
	midAnchor := readAnchor(sqlDB, sevenDayRows/2)
	tailAnchor := readAnchor(sqlDB, int64(math.Max(0, float64(sevenDayRows-1000))))
	for range 20 {
		readRawKeysetPage(sqlDB, midAnchor, cfg.PageLimit)
		readRawKeysetPage(sqlDB, tailAnchor, cfg.PageLimit)
	}
	midLatency := measure(cfg.Samples, func() error {
		return readRawKeysetPage(sqlDB, midAnchor, cfg.PageLimit)
	})
	tailLatency := measure(cfg.Samples, func() error {
		return readRawKeysetPage(sqlDB, tailAnchor, cfg.PageLimit)
	})

	fileInfo, err := os.Stat(dbPath)
	if err != nil {
		fatalf("stat benchmark database: %v", err)
	}
	fmt.Printf("rows=%d database=%s size_bytes=%d seed=%s indexes=%s\n", cfg.Rows, dbPath, fileInfo.Size(), seedDuration.Round(time.Millisecond), indexDuration.Round(time.Millisecond))
	printLatency("reader_first_page", firstLatency)
	printLatency("reader_cursor_page", secondLatency)
	printLatency("reader_selective_task_search", taskSearchLatency)
	printLatency("reader_selective_title_search", titleSearchLatency)
	printLatency("sql_keyset_midpoint", midLatency)
	printLatency("sql_keyset_tail", tailLatency)
	if cfg.Keep {
		fmt.Printf("kept_database=%s\n", dbPath)
	}
}

func prepareForBulkSeed(db *sql.DB) {
	triggerRows, err := db.Query("SELECT name FROM sqlite_master WHERE type = 'trigger' AND name LIKE 'daily_jira_%'")
	if err != nil {
		fatalf("list read-model triggers: %v", err)
	}
	var triggers []string
	for triggerRows.Next() {
		var name string
		if err := triggerRows.Scan(&name); err != nil {
			fatalf("scan trigger name: %v", err)
		}
		triggers = append(triggers, name)
	}
	triggerRows.Close()
	for _, name := range triggers {
		if _, err := db.Exec("DROP TRIGGER " + name); err != nil {
			fatalf("drop benchmark trigger %s: %v", name, err)
		}
	}
	for _, name := range []string{
		"idx_daily_jira_page",
		"idx_daily_jira_project_page",
		"idx_daily_jira_assignee_page",
		"idx_daily_jira_project_assignee_page",
		"idx_daily_jira_bucket_transition",
		"idx_daily_jira_due_transition",
		"idx_daily_jira_short_task_search",
		"idx_daily_jira_short_title_search",
		"idx_daily_jira_short_project_search",
		"idx_daily_jira_short_assignee_search",
		"idx_daily_jira_short_status_search",
		"idx_daily_jira_count_bucket",
		"idx_daily_jira_count_assignee",
	} {
		if _, err := db.Exec("DROP INDEX IF EXISTS " + name); err != nil {
			fatalf("drop benchmark index %s: %v", name, err)
		}
	}
	if _, err := db.Exec("DELETE FROM daily_jira_audit_entries"); err != nil {
		fatalf("clear benchmark entries: %v", err)
	}
	if _, err := db.Exec("DELETE FROM daily_jira_audit_counts"); err != nil {
		fatalf("clear benchmark counts: %v", err)
	}
	if _, err := db.Exec("UPDATE daily_jira_audit_state SET backfilled = 1, generation = 1"); err != nil {
		fatalf("reset benchmark projection state: %v", err)
	}
}

func seedEntries(db *sql.DB, rows int) map[distribution]int64 {
	counts := make(map[distribution]int64, 32*512*3)
	const batchSize = 100_000
	const createdAt = "2026-01-01T00:00:00+08:00"
	const activityAt = "2026-08-21T00:00:00+08:00"
	for batchStart := 0; batchStart < rows; batchStart += batchSize {
		batchEnd := min(rows, batchStart+batchSize)
		tx, err := db.Begin()
		if err != nil {
			fatalf("begin seed batch: %v", err)
		}
		statement, err := tx.Prepare(`INSERT INTO daily_jira_audit_entries (
			task_id, project_key, title, title_key, project, assignee, assignee_key,
			status, status_key, issue_type, task_created_at, created_unix, created_day,
			last_activity_at, activity_unix, bucket, overdue, sort_overdue, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'progress', 'progress', 'bug', ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			fatalf("prepare seed statement: %v", err)
		}
		for index := batchStart; index < batchEnd; index++ {
			project := fmt.Sprintf("P%02d", index%32)
			assignee := fmt.Sprintf("member-%03d", index%512)
			bucket := "seven_day"
			switch index % 10 {
			case 0:
				bucket = "today"
			case 1, 2:
				bucket = "three_day"
			}
			overdue := 0
			if index%13 == 0 {
				overdue = 1
			}
			sortOverdue := 1 - overdue
			createdUnix := int64(1_700_000_000 + index)
			activityUnix := int64(1_800_000_000 + index%100_000)
			taskID := fmt.Sprintf("PERF-%08d", index)
			title := fmt.Sprintf("performance issue %08d", index)
			if _, err := statement.Exec(
				taskID, project, title, title, project, assignee, assignee,
				createdAt, createdUnix, 2_460_000, activityAt, activityUnix,
				bucket, overdue, sortOverdue, activityAt,
			); err != nil {
				fatalf("seed row %d: %v", index, err)
			}
			counts[distribution{Project: project, Assignee: assignee, Bucket: bucket}]++
		}
		statement.Close()
		if err := tx.Commit(); err != nil {
			fatalf("commit seed batch: %v", err)
		}
		if batchEnd == rows || batchEnd%(1_000_000) == 0 {
			fmt.Printf("seeded=%d/%d\n", batchEnd, rows)
		}
	}
	return counts
}

func seedCounts(db *sql.DB, counts map[distribution]int64) {
	tx, err := db.Begin()
	if err != nil {
		fatalf("begin count seed: %v", err)
	}
	statement, err := tx.Prepare(`INSERT INTO daily_jira_audit_counts
		(project_key, assignee_key, bucket, item_count) VALUES (?, ?, ?, ?)`)
	if err != nil {
		fatalf("prepare count seed: %v", err)
	}
	for key, count := range counts {
		if _, err := statement.Exec(key.Project, key.Assignee, key.Bucket, count); err != nil {
			fatalf("seed count: %v", err)
		}
	}
	statement.Close()
	if err := tx.Commit(); err != nil {
		fatalf("commit count seed: %v", err)
	}
}

func readAnchor(db *sql.DB, offset int64) anchor {
	var result anchor
	err := db.QueryRow(`SELECT sort_overdue, created_unix, activity_unix, task_id
		FROM daily_jira_audit_entries WHERE bucket = 'seven_day'
		ORDER BY sort_overdue, created_unix, activity_unix, task_id
		LIMIT 1 OFFSET ?`, offset).Scan(&result.SortOverdue, &result.CreatedUnix, &result.ActivityUnix, &result.TaskID)
	if err != nil {
		fatalf("read keyset anchor at %d: %v", offset, err)
	}
	return result
}

func readRawKeysetPage(db *sql.DB, cursor anchor, limit int) error {
	rows, err := db.Query(`SELECT task_id, title, project_key, assignee, status
		FROM daily_jira_audit_entries
		WHERE bucket = 'seven_day'
			AND (sort_overdue, created_unix, activity_unix, task_id) > (?, ?, ?, ?)
		ORDER BY sort_overdue, created_unix, activity_unix, task_id
		LIMIT ?`,
		cursor.SortOverdue, cursor.CreatedUnix, cursor.ActivityUnix, cursor.TaskID,
		limit,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var values [5]string
		if err := rows.Scan(&values[0], &values[1], &values[2], &values[3], &values[4]); err != nil {
			return err
		}
	}
	return rows.Err()
}

func measure(samples int, operation func() error) latencySummary {
	values := make([]time.Duration, 0, samples)
	for range samples {
		started := time.Now()
		if err := operation(); err != nil {
			fatalf("benchmark operation: %v", err)
		}
		values = append(values, time.Since(started))
	}
	sort.Slice(values, func(left, right int) bool { return values[left] < values[right] })
	return latencySummary{
		P50: percentile(values, 0.50),
		P95: percentile(values, 0.95),
		P99: percentile(values, 0.99),
		Max: values[len(values)-1],
	}
}

func percentile(values []time.Duration, quantile float64) time.Duration {
	index := int(math.Ceil(float64(len(values))*quantile)) - 1
	return values[max(0, min(len(values)-1, index))]
}

func printLatency(name string, value latencySummary) {
	fmt.Printf("%s p50=%s p95=%s p99=%s max=%s\n", name, value.P50, value.P95, value.P99, value.Max)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
