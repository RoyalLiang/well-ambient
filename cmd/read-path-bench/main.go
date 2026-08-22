package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type config struct {
	Rows    int
	Samples int
	Limit   int
	Keep    bool
}

type latency struct {
	P50 time.Duration `json:"-"`
	P95 time.Duration `json:"-"`
	P99 time.Duration `json:"-"`
	Max time.Duration `json:"-"`
}

type pageRow struct {
	ID       int64  `json:"id"`
	Scope    string `json:"scope"`
	Status   string `json:"status"`
	SortUnix int64  `json:"sort_unix"`
	Payload  string `json:"payload"`
}

type anchor struct {
	SortUnix int64
	ID       int64
}

func main() {
	cfg := config{}
	flag.IntVar(&cfg.Rows, "rows", 10_000_000, "total fact rows in the disposable database")
	flag.IntVar(&cfg.Samples, "samples", 500, "warm samples per scenario")
	flag.IntVar(&cfg.Limit, "limit", 100, "bounded page size")
	flag.BoolVar(&cfg.Keep, "keep", false, "keep the disposable database")
	flag.Parse()
	if cfg.Rows < 1 || cfg.Samples < 1 || cfg.Limit < 1 || cfg.Limit > 100 {
		fatalf("rows and samples must be positive; limit must be between 1 and 100")
	}

	file, err := os.CreateTemp("", "well-ambient-read-path-bench-*.db")
	if err != nil {
		fatalf("create benchmark database: %v", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		fatalf("close benchmark placeholder: %v", err)
	}
	if !cfg.Keep {
		defer func() {
			_ = os.Remove(path)
			_ = os.Remove(path + "-wal")
			_ = os.Remove(path + "-shm")
		}()
	}

	database, err := sql.Open("sqlite3", path)
	if err != nil {
		fatalf("open benchmark database: %v", err)
	}
	defer database.Close()
	database.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode=OFF",
		"PRAGMA synchronous=OFF",
		"PRAGMA temp_store=MEMORY",
		"PRAGMA cache_size=-524288",
		"PRAGMA mmap_size=1073741824",
	} {
		mustExec(database, pragma)
	}
	mustExec(database, `CREATE TABLE read_facts (
		id INTEGER PRIMARY KEY,
		scope TEXT NOT NULL,
		status TEXT NOT NULL,
		sort_unix INTEGER NOT NULL,
		entity_id INTEGER NOT NULL,
		payload TEXT NOT NULL
	)`)
	mustExec(database, `CREATE TABLE read_aggregate_projection (
		scope TEXT NOT NULL,
		status TEXT NOT NULL,
		total INTEGER NOT NULL,
		updated_unix INTEGER NOT NULL,
		PRIMARY KEY (scope, status)
	) WITHOUT ROWID`)

	seedStarted := time.Now()
	seedFacts(database, cfg.Rows)
	seedAggregates(database, cfg.Rows)
	seedDuration := time.Since(seedStarted)
	indexStarted := time.Now()
	mustExec(database, `CREATE INDEX idx_read_entity_page
		ON read_facts (scope, status, sort_unix DESC, id DESC)`)
	mustExec(database, `CREATE INDEX idx_read_timeline
		ON read_facts (entity_id, sort_unix DESC, id DESC)`)
	mustExec(database, "ANALYZE")
	indexDuration := time.Since(indexStarted)

	scope := "S07"
	status := "active"
	tail := tailAnchor(database, scope, status, cfg.Rows/64-1000)
	entityID := int64(42)
	pageSQL := `SELECT id, scope, status, sort_unix, payload
		FROM read_facts WHERE scope = ? AND status = ?
		ORDER BY sort_unix DESC, id DESC LIMIT ?`
	cursorSQL := `SELECT id, scope, status, sort_unix, payload
		FROM read_facts WHERE scope = ? AND status = ? AND (sort_unix, id) < (?, ?)
		ORDER BY sort_unix DESC, id DESC LIMIT ?`
	timelineSQL := `SELECT id, scope, status, sort_unix, payload
		FROM read_facts WHERE entity_id = ? ORDER BY sort_unix DESC, id DESC LIMIT ?`
	aggregateSQL := `SELECT total, updated_unix FROM read_aggregate_projection WHERE scope = ? AND status = ?`

	for range 25 {
		mustReadPage(database, pageSQL, scope, status, cfg.Limit)
		mustReadPage(database, cursorSQL, scope, status, tail.SortUnix, tail.ID, cfg.Limit)
		mustReadPage(database, timelineSQL, entityID, cfg.Limit)
		mustReadAggregate(database, aggregateSQL, scope, status)
	}
	entityFirst := measure(cfg.Samples, func() {
		mustReadPage(database, pageSQL, scope, status, cfg.Limit)
	})
	entityTail := measure(cfg.Samples, func() {
		mustReadPage(database, cursorSQL, scope, status, tail.SortUnix, tail.ID, cfg.Limit)
	})
	timeline := measure(cfg.Samples, func() {
		mustReadPage(database, timelineSQL, entityID, cfg.Limit)
	})
	aggregate := measure(cfg.Samples, func() {
		mustReadAggregate(database, aggregateSQL, scope, status)
	})
	handler := boundedHandler(database, cursorSQL, scope, status, tail, cfg.Limit)
	handlerLatency := measure(cfg.Samples, func() {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/bench", nil))
		if recorder.Code != http.StatusOK {
			fatalf("handler benchmark status=%d", recorder.Code)
		}
	})

	info, err := os.Stat(path)
	if err != nil {
		fatalf("stat benchmark database: %v", err)
	}
	fmt.Printf("rows=%d database=%s size_bytes=%d seed=%s indexes=%s limit=%d samples=%d\n",
		cfg.Rows, path, info.Size(), seedDuration.Round(time.Millisecond), indexDuration.Round(time.Millisecond), cfg.Limit, cfg.Samples)
	printLatency("entity_first_page", entityFirst)
	printLatency("entity_tail_keyset_page", entityTail)
	printLatency("entity_timeline", timeline)
	printLatency("aggregate_projection_lookup", aggregate)
	printLatency("local_handler_cursor_page", handlerLatency)
	printPlan(database, "entity_page_plan", pageSQL, scope, status, cfg.Limit)
	printPlan(database, "entity_tail_plan", cursorSQL, scope, status, tail.SortUnix, tail.ID, cfg.Limit)
	printPlan(database, "timeline_plan", timelineSQL, entityID, cfg.Limit)
	printPlan(database, "aggregate_plan", aggregateSQL, scope, status)
	if cfg.Keep {
		fmt.Printf("kept_database=%s\n", path)
	}
}

func seedFacts(database *sql.DB, rows int) {
	const batchSize = 100_000
	for start := 1; start <= rows; start += batchSize {
		end := min(rows, start+batchSize-1)
		_, err := database.Exec(`WITH RECURSIVE sequence(n) AS (
			VALUES (?) UNION ALL SELECT n + 1 FROM sequence WHERE n < ?
		)
		INSERT INTO read_facts(id, scope, status, sort_unix, entity_id, payload)
		SELECT n,
			printf('S%02d', n % 32),
			CASE WHEN (n / 32) % 2 = 0 THEN 'active' ELSE 'done' END,
			n,
			n % 100000,
			printf('bounded-payload-%010d', n)
		FROM sequence`, start, end)
		if err != nil {
			fatalf("seed facts %d-%d: %v", start, end, err)
		}
	}
}

func seedAggregates(database *sql.DB, rows int) {
	transaction, err := database.Begin()
	if err != nil {
		fatalf("begin aggregate seed: %v", err)
	}
	statement, err := transaction.Prepare(`INSERT INTO read_aggregate_projection(scope, status, total, updated_unix) VALUES (?, ?, ?, ?)`)
	if err != nil {
		fatalf("prepare aggregate seed: %v", err)
	}
	defer statement.Close()
	for scopeIndex := range 32 {
		for _, status := range []string{"active", "done"} {
			if _, err := statement.Exec(fmt.Sprintf("S%02d", scopeIndex), status, rows/64, time.Now().Unix()); err != nil {
				fatalf("seed aggregate: %v", err)
			}
		}
	}
	if err := transaction.Commit(); err != nil {
		fatalf("commit aggregate seed: %v", err)
	}
}

func tailAnchor(database *sql.DB, scope, status string, offset int) anchor {
	if offset < 0 {
		offset = 0
	}
	var value anchor
	err := database.QueryRow(`SELECT sort_unix, id FROM read_facts
		WHERE scope = ? AND status = ? ORDER BY sort_unix DESC, id DESC LIMIT 1 OFFSET ?`, scope, status, offset).
		Scan(&value.SortUnix, &value.ID)
	if err != nil {
		fatalf("read tail anchor: %v", err)
	}
	return value
}

func mustReadPage(database *sql.DB, query string, args ...any) []pageRow {
	rows, err := database.Query(query, args...)
	if err != nil {
		fatalf("read bounded page: %v", err)
	}
	defer rows.Close()
	items := make([]pageRow, 0, 100)
	for rows.Next() {
		var item pageRow
		if err := rows.Scan(&item.ID, &item.Scope, &item.Status, &item.SortUnix, &item.Payload); err != nil {
			fatalf("scan bounded page: %v", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		fatalf("iterate bounded page: %v", err)
	}
	return items
}

func mustReadAggregate(database *sql.DB, query string, args ...any) {
	var total, updated int64
	if err := database.QueryRow(query, args...).Scan(&total, &updated); err != nil {
		fatalf("read aggregate projection: %v", err)
	}
}

func boundedHandler(database *sql.DB, query, scope, status string, cursor anchor, limit int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		items := mustReadPage(database, query, scope, status, cursor.SortUnix, cursor.ID, limit)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": items,
			"page":  map[string]any{"limit": limit, "has_more": len(items) == limit},
		})
	})
}

func measure(samples int, run func()) latency {
	values := make([]time.Duration, 0, samples)
	for range samples {
		started := time.Now()
		run()
		values = append(values, time.Since(started))
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return latency{
		P50: percentile(values, 0.50), P95: percentile(values, 0.95),
		P99: percentile(values, 0.99), Max: values[len(values)-1],
	}
}

func percentile(values []time.Duration, quantile float64) time.Duration {
	index := int(float64(len(values)-1) * quantile)
	return values[index]
}

func printLatency(name string, value latency) {
	fmt.Printf("scenario=%s p50_ms=%.3f p95_ms=%.3f p99_ms=%.3f max_ms=%.3f\n",
		name, milliseconds(value.P50), milliseconds(value.P95), milliseconds(value.P99), milliseconds(value.Max))
}

func milliseconds(value time.Duration) float64 {
	return float64(value.Microseconds()) / 1000
}

func printPlan(database *sql.DB, name, query string, args ...any) {
	rows, err := database.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		fatalf("explain %s: %v", name, err)
	}
	defer rows.Close()
	parts := make([]string, 0, 4)
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			fatalf("scan plan %s: %v", name, err)
		}
		parts = append(parts, detail)
	}
	fmt.Printf("plan=%s detail=%q\n", name, strings.Join(parts, " | "))
}

func mustExec(database *sql.DB, statement string) {
	if _, err := database.Exec(statement); err != nil {
		fatalf("execute %q: %v", statement, err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
