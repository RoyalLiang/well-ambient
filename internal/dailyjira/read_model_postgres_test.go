package dailyjira_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"well-ambient/internal/dailyjira"
	"well-ambient/internal/db"
	"well-ambient/internal/readmodel"
	"well-ambient/internal/testutil/pgtest"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type postgresReadLog struct {
	logger.Interface
	mu        sync.Mutex
	summaries int
	queries   []string
}

func (l *postgresReadLog) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	l.mu.Lock()
	defer l.mu.Unlock()
	if strings.Contains(sql, "GROUP BY bucket") {
		l.summaries++
	}
	l.queries = append(l.queries, sql)
}

func newPostgresReadFixture(t testing.TB, rows int) (*gorm.DB, *postgresReadLog) {
	t.Helper()
	conn := pgtest.Open(t)
	if err := conn.AutoMigrate(&db.TaskTelemetry{}, &db.SolutionCatalogEntry{}, &db.SolutionCatalogSearchToken{},
		&db.SolutionRevision{}, &db.SolutionSourceRef{}, &db.SolutionPolishJob{}, &db.SolutionComparison{}); err != nil {
		t.Fatal(err)
	}
	// Use the repository's real PostgreSQL indexes, not a benchmark-only index.
	if err := db.MigratePostgresReadIndexes(conn); err != nil {
		t.Fatal(err)
	}
	if err := readmodel.EnsureDatasets(conn, []readmodel.Dataset{{Name: "task_facts", Tables: []string{"task_telemetries"}}}); err != nil {
		t.Fatal(err)
	}
	if rows > 0 {
		if err := conn.Exec(`INSERT INTO task_telemetries
			(task_id, project_key, source, repo, title, assignee, status, issue_type, task_created_at, last_update, source_updated_at, due_date, decision_logs)
			SELECT 'P' || (n % 10)::text || '-' || n::text, 'P' || (n % 10)::text, 'jira', 'Synthetic',
			'read fixture ' || n::text, CASE WHEN n % 5 = 0 THEN '' ELSE 'alice' END,
			CASE WHEN n % 11 = 0 THEN 'done' ELSE 'progress' END, 'task',
			CURRENT_DATE - (n % 40)::integer, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP,
			CASE WHEN n % 3 = 0 THEN CURRENT_DATE - 1 ELSE NULL END, repeat('x', 512)
			FROM generate_series(1, ?) AS n`, rows).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := conn.Exec("ANALYZE task_telemetries").Error; err != nil {
		t.Fatal(err)
	}
	log := &postgresReadLog{Interface: logger.Default.LogMode(logger.Silent)}
	return conn.Session(&gorm.Session{Logger: log}), log
}

func TestPostgresReadPageReusesScopedSummary(t *testing.T) {
	conn, log := newPostgresReadFixture(t, 200)
	reader := dailyjira.NewReader(conn)
	query := dailyjira.Query{Bucket: dailyjira.BucketSevenDay, Limit: 3, Scope: dailyjira.Scope{ProjectKeys: []string{"p1"}, Assignees: []string{"ALICE"}, FilterAssignees: true}}
	first, err := reader.ReadPage(context.Background(), query)
	if err != nil || !first.HasMore {
		t.Fatalf("first: %+v, %v", first, err)
	}
	query.Cursor = first.NextCursor
	second, err := reader.ReadPage(context.Background(), query)
	if err != nil || second.Summary != first.Summary {
		t.Fatalf("second: %+v, %v", second, err)
	}
	query.Cursor, query.Direction = second.PreviousCursor, dailyjira.DirectionPrevious
	back, err := reader.ReadPage(context.Background(), query)
	if err != nil || len(back.Items) != len(first.Items) {
		t.Fatalf("previous: %+v, %v", back, err)
	}
	for i := range first.Items {
		if back.Items[i].TaskID != first.Items[i].TaskID {
			t.Fatalf("previous items differ at %d", i)
		}
	}
	if log.summaries != 1 {
		t.Fatalf("three pages recomputed summary %d times, want 1", log.summaries)
	}
}

func TestPostgresSummaryScopeAndMutationIsolation(t *testing.T) {
	conn, log := newPostgresReadFixture(t, 200)
	reader := dailyjira.NewReader(conn)
	ctx := context.Background()
	query := dailyjira.Query{Bucket: dailyjira.BucketSevenDay, Limit: 2, Scope: dailyjira.Scope{ProjectKeys: []string{"P1"}, Assignees: []string{"alice"}, FilterAssignees: true}}
	first, err := reader.ReadPage(ctx, query)
	if err != nil || first.Summary.SevenDay == 0 {
		t.Fatalf("seed page: %+v, %v", first, err)
	}
	query.Scope.ProjectKeys = []string{" p1 ", "P1"}
	query.Scope.Assignees = []string{" ALICE ", "alice"}
	query.Search = "no matching title"
	query.Bucket = dailyjira.BucketToday
	filtered, err := reader.ReadPage(ctx, query)
	if err != nil || len(filtered.Items) != 0 || filtered.Summary != first.Summary || log.summaries != 1 {
		t.Fatalf("equivalent scope/search must reuse full summary: %+v, %v, reads=%d", filtered, err, log.summaries)
	}
	query.Search = ""
	query.Bucket = dailyjira.BucketSevenDay
	query.Scope.Assignees = []string{"bob"}
	hidden, err := reader.ReadPage(ctx, query)
	if err != nil || hidden.Summary.Total != 0 || len(hidden.Items) != 0 {
		t.Fatalf("assignee scope leak: %+v, %v", hidden, err)
	}
	query.Scope.FilterAssignees = false
	unrestricted, err := reader.ReadPage(ctx, query)
	if err != nil || unrestricted.Summary != first.Summary {
		t.Fatalf("filter flag lost: %+v, %v", unrestricted, err)
	}
	query.Scope = dailyjira.Scope{ProjectKeys: []string{"P0"}, Assignees: []string{"nobody"}, FilterAssignees: true}
	unassigned, err := reader.ReadPage(ctx, query)
	if err != nil || unassigned.Summary.Today == 0 || unassigned.Summary.SevenDay == 0 {
		t.Fatalf("unassigned visibility lost: %+v, %v", unassigned, err)
	}
	for _, item := range unassigned.Items {
		if item.ProjectKey != "P0" || item.Assignee != "" {
			t.Fatalf("project scope leak: %+v", item)
		}
	}
	query.Scope = dailyjira.Scope{ProjectKeys: []string{"P1"}, Assignees: []string{"alice"}, FilterAssignees: true}
	if err := conn.Exec(`INSERT INTO task_telemetries(task_id, project_key, source, assignee, status, task_created_at, last_update)
		VALUES ('P1-10001', 'P1', 'jira', 'alice', 'progress', CURRENT_DATE-10, CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatal(err)
	}
	query.Cursor = first.NextCursor
	if _, err := reader.ReadPage(ctx, query); !errors.Is(err, dailyjira.ErrStaleCursor) {
		t.Fatalf("insert must invalidate cursor: %v", err)
	}
	query.Cursor = ""
	inserted, err := reader.ReadPage(ctx, query)
	if err != nil || inserted.Summary.SevenDay != first.Summary.SevenDay+1 {
		t.Fatalf("insert stale: %+v, %v", inserted, err)
	}
	if err := conn.Exec("UPDATE task_telemetries SET assignee='bob' WHERE task_id='P1-10001'").Error; err != nil {
		t.Fatal(err)
	}
	updated, err := reader.ReadPage(ctx, query)
	if err != nil || updated.Summary != first.Summary {
		t.Fatalf("update stale: %+v, %v", updated, err)
	}
	query.Scope.FilterAssignees = false
	if err := conn.Exec("DELETE FROM task_telemetries WHERE task_id='P1-10001'").Error; err != nil {
		t.Fatal(err)
	}
	deleted, err := reader.ReadPage(ctx, query)
	if err != nil || deleted.Summary != first.Summary {
		t.Fatalf("delete stale: %+v, %v", deleted, err)
	}
}

func TestPostgresReadPageDateAndTimezoneBoundCursors(t *testing.T) {
	conn, log := newPostgresReadFixture(t, 40)
	pool, err := conn.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(1)
	if err := conn.Exec("SET TIME ZONE 'Etc/GMT+12'").Error; err != nil {
		t.Fatal(err)
	}
	reader := dailyjira.NewReader(conn)
	query := dailyjira.Query{Bucket: dailyjira.BucketSevenDay, Limit: 2}
	first, err := reader.ReadPage(context.Background(), query)
	if err != nil || first.NextCursor == "" {
		t.Fatalf("first page: %+v, %v", first, err)
	}
	decode := func(raw string) map[string]any {
		t.Helper()
		payload, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err := json.Unmarshal(payload, &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	for _, basis := range []string{"", "1900-01-01|UTC"} {
		cursor := decode(first.NextCursor)
		cursor["d"] = basis
		payload, _ := json.Marshal(cursor)
		query.Cursor = base64.RawURLEncoding.EncodeToString(payload)
		if _, err := reader.ReadPage(context.Background(), query); !errors.Is(err, dailyjira.ErrStaleCursor) {
			t.Fatalf("old date/legacy cursor: %v", err)
		}
	}
	if err := conn.Exec("SET TIME ZONE 'Pacific/Kiritimati'").Error; err != nil {
		t.Fatal(err)
	}
	query.Cursor = first.NextCursor
	if _, err := reader.ReadPage(context.Background(), query); !errors.Is(err, dailyjira.ErrStaleCursor) {
		t.Fatalf("timezone change: %v", err)
	}
	query.Cursor = ""
	changed, err := reader.ReadPage(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if decode(changed.NextCursor)["d"] == decode(first.NextCursor)["d"] || log.summaries != 2 {
		t.Fatal("timezone change reused old time basis or summary")
	}
	if strings.Split(decode(changed.NextCursor)["d"].(string), "|")[0] == strings.Split(decode(first.NextCursor)["d"].(string), "|")[0] {
		t.Fatal("fixture must exercise an actual database date transition")
	}
	// A fresh Reader is an uncached oracle for the database's current date.
	fresh, err := dailyjira.NewReader(conn).ReadPage(context.Background(), query)
	if err != nil || fresh.Summary != changed.Summary {
		t.Fatalf("timezone summary mismatch: %+v vs %+v (%v)", changed.Summary, fresh.Summary, err)
	}
}

func TestPostgresReadPageSnapshotRollbackAndConcurrentReaders(t *testing.T) {
	conn, log := newPostgresReadFixture(t, 200)
	ctx := context.Background()
	reader := dailyjira.NewReader(conn)
	query := dailyjira.Query{Bucket: dailyjira.BucketSevenDay, Limit: 2}
	before, err := reader.ReadPage(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	rollback := errors.New("intentional rollback")
	err = conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.SavePoint("before_change").Error; err != nil {
			return err
		}
		if err := tx.Exec("UPDATE task_telemetries SET status='done'").Error; err != nil {
			return err
		}
		nestedReader := dailyjira.NewReader(tx)
		inside, err := nestedReader.ReadPage(ctx, query)
		if err != nil {
			return err
		}
		if inside.Summary.Total != 0 {
			t.Fatalf("transaction cannot see own changes: %+v", inside)
		}
		outside, err := reader.ReadPage(ctx, query)
		if err != nil {
			return err
		}
		if outside.Generation != before.Generation || outside.Summary != before.Summary {
			t.Fatalf("uncommitted facts escaped: %+v", outside)
		}
		if err := tx.RollbackTo("before_change").Error; err != nil {
			return err
		}
		// This new write reuses the rolled-back generation number with different facts.
		if err := tx.Exec("UPDATE task_telemetries SET title='after rollback'").Error; err != nil {
			return err
		}
		retried, err := nestedReader.ReadPage(ctx, query)
		if err != nil {
			return err
		}
		if retried.Generation != inside.Generation || retried.Summary != before.Summary {
			t.Fatalf("rolled-back generation poisoned nested reader: %+v", retried)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	startSummaries := log.summaries
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			page, err := reader.ReadPage(ctx, query)
			if err != nil || page.Summary != before.Summary || page.Generation != before.Generation {
				t.Errorf("parallel snapshot: %+v, %v", page, err)
			}
		}()
	}
	wg.Wait()
	if log.summaries != startSummaries {
		t.Fatal("warm concurrent reads repeated aggregation")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := reader.ReadPage(cancelled, query); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled request: %v", err)
	}
}

type generationReadHook struct {
	logger.Interface
	once  sync.Once
	after func()
}

func (l *generationReadHook) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	query, _ := fc()
	l.Interface.Trace(ctx, begin, fc, err)
	if strings.HasPrefix(query, "SELECT generation,") {
		l.once.Do(l.after)
	}
}

func TestPostgresReadPageKeepsSummaryAndItemsInOneSnapshot(t *testing.T) {
	conn, log := newPostgresReadFixture(t, 80)
	query := dailyjira.Query{Bucket: dailyjira.BucketSevenDay, Limit: 2}
	before, err := dailyjira.NewReader(conn).ReadPage(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	hook := &generationReadHook{Interface: log, after: func() {
		if err := conn.Exec("UPDATE task_telemetries SET status='done'").Error; err != nil {
			t.Fatal(err)
		}
	}}
	reader := dailyjira.NewReader(conn.Session(&gorm.Session{Logger: hook}))
	page, err := reader.ReadPage(context.Background(), query)
	if err != nil || page.Generation != before.Generation || page.Summary != before.Summary || len(page.Items) != 2 {
		t.Fatalf("mixed snapshots: %+v, %v", page, err)
	}
	after, err := reader.ReadPage(context.Background(), query)
	if err != nil || after.Generation <= before.Generation || after.Summary.Total != 0 || len(after.Items) != 0 {
		t.Fatalf("next snapshot stale: %+v, %v", after, err)
	}
}

func TestPostgresSummaryInvalidatesOnTruncateAndRequiresMigration(t *testing.T) {
	conn, _ := newPostgresReadFixture(t, 80)
	datasets := []readmodel.Dataset{{Name: "task_facts", Tables: []string{"task_telemetries"}}}
	if err := readmodel.VerifyDatasets(conn, datasets); err != nil {
		t.Fatal(err)
	}
	reader := dailyjira.NewReader(conn)
	query := dailyjira.Query{Bucket: dailyjira.BucketSevenDay, Limit: 2}
	before, err := reader.ReadPage(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Exec("TRUNCATE task_telemetries").Error; err != nil {
		t.Fatal(err)
	}
	after, err := reader.ReadPage(context.Background(), query)
	if err != nil || after.Generation <= before.Generation || after.Summary.Total != 0 || len(after.Items) != 0 {
		t.Fatalf("truncate stale: %+v, %v", after, err)
	}
	if err := conn.Exec(`CREATE OR REPLACE TRIGGER read_generation_task_facts_task_telemetries
		AFTER INSERT OR UPDATE OR DELETE ON task_telemetries FOR EACH STATEMENT
		EXECUTE FUNCTION read_generation_bump_task_facts_task_telemetries()`).Error; err != nil {
		t.Fatal(err)
	}
	if err := readmodel.VerifyDatasets(conn, datasets); err == nil {
		t.Fatal("legacy trigger must require migration")
	}
	if err := readmodel.EnsureDatasets(conn, datasets); err != nil {
		t.Fatal(err)
	}
	if err := readmodel.VerifyDatasets(conn, datasets); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresReadPageLocksSnapshotAgainstConcurrentTruncate(t *testing.T) {
	conn, log := newPostgresReadFixture(t, 80)
	hook := &generationReadHook{Interface: log, after: func() {
		err := conn.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("SET LOCAL lock_timeout = '100ms'").Error; err != nil {
				return err
			}
			return tx.Exec("TRUNCATE task_telemetries").Error
		})
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
			t.Fatalf("TRUNCATE must wait for read snapshot, got %v", err)
		}
	}}
	reader := dailyjira.NewReader(conn.Session(&gorm.Session{Logger: hook}))
	query := dailyjira.Query{Bucket: dailyjira.BucketSevenDay, Limit: 2}
	page, err := reader.ReadPage(context.Background(), query)
	if err != nil || page.Summary.Total == 0 || len(page.Items) != 2 {
		t.Fatalf("read lost its snapshot: %+v, %v", page, err)
	}
	if err := conn.Exec("TRUNCATE task_telemetries").Error; err != nil {
		t.Fatal(err)
	}
	page, err = reader.ReadPage(context.Background(), query)
	if err != nil || page.Summary.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("truncate after read: %+v, %v", page, err)
	}
}

func BenchmarkPostgresDailyJiraReadPage(b *testing.B) {
	for _, rows := range []int{10000, 100000} {
		b.Run(fmt.Sprintf("rows_%d", rows), func(b *testing.B) {
			conn, _ := newPostgresReadFixture(b, rows)
			// Logging full SQL is deliberately disabled during timed samples.
			conn = conn.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
			reader := dailyjira.NewReader(conn)
			query := dailyjira.Query{Bucket: dailyjira.BucketSevenDay, Limit: 20, Scope: dailyjira.Scope{ProjectKeys: []string{"P1"}, Assignees: []string{"alice"}, FilterAssignees: true}}
			if _, err := reader.ReadPage(context.Background(), query); err != nil {
				b.Fatal(err)
			}
			samples := make([]time.Duration, b.N)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				page, err := reader.ReadPage(context.Background(), query)
				if err != nil || len(page.Items) != 20 {
					b.Fatalf("page rows=%d: %v", len(page.Items), err)
				}
				samples[i] = time.Since(start)
			}
			b.StopTimer()
			sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
			b.ReportMetric(float64(samples[(len(samples)-1)*95/100].Microseconds())/1000, "p95-ms")
			b.ReportMetric(float64(samples[(len(samples)-1)*99/100].Microseconds())/1000, "p99-ms")
		})
	}
}

// Set WELL_AMBIENT_PG_PLAN_DIR to retain raw plans from synthetic data only.
func TestPostgresDailyJiraExplain(t *testing.T) {
	conn, log := newPostgresReadFixture(t, 100000)
	_, err := dailyjira.NewReader(conn).ReadPage(context.Background(), dailyjira.Query{
		Bucket: dailyjira.BucketSevenDay, Limit: 20,
		Scope: dailyjira.Scope{ProjectKeys: []string{"P1"}, Assignees: []string{"alice"}, FilterAssignees: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	queries := append([]string(nil), log.queries...)
	for _, query := range queries {
		if !strings.HasPrefix(query, "WITH candidates AS") {
			continue
		}
		name := "items"
		if strings.Contains(query, "GROUP BY bucket") {
			name = "summary"
		}
		rows, err := conn.Raw("EXPLAIN (ANALYZE, BUFFERS, SETTINGS, FORMAT JSON) " + query).Rows()
		if err != nil {
			t.Fatal(err)
		}
		var plan json.RawMessage
		if !rows.Next() {
			rows.Close()
			t.Fatal("empty EXPLAIN")
		}
		err = rows.Scan(&plan)
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !json.Valid(plan) {
			t.Fatal("invalid JSON plan")
		}
		if dir := os.Getenv("WELL_AMBIENT_PG_PLAN_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			payload, err := json.MarshalIndent(struct {
				SQL  string
				Plan json.RawMessage
			}{query, plan}, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name+".json"), payload, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
}
