package solutioncatalog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/db"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type reconcileReadProbe struct {
	logger.Interface
	selects          int
	maxRows          int64
	coldRevisionRead bool
}

func (p *reconcileReadProbe) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, rows := fc()
	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "SELECT") {
		return
	}
	p.selects++
	if rows > p.maxRows {
		p.maxRows = rows
	}
	if strings.Contains(sql, "solution_revisions") && (strings.Contains(sql, "SELECT *") || strings.Contains(sql, "`content`")) {
		p.coldRevisionRead = true
	}
}

func reconcileFixture(t testing.TB, projected, missing int) (*Module, *gorm.DB) {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := conn.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := conn.AutoMigrate(&db.SolutionAsset{}, &db.SolutionRevision{}, &db.SolutionCatalogEntry{}, &db.SolutionCatalogSyncJob{}); err != nil {
		t.Fatal(err)
	}
	assets := make([]db.SolutionAsset, 0, projected+missing)
	revisions := make([]db.SolutionRevision, 0, projected+missing)
	entries := make([]db.SolutionCatalogEntry, 0, projected)
	for i := 1; i <= projected+missing; i++ {
		id := uint(i)
		assets = append(assets, db.SolutionAsset{ID: id, DemandID: fmt.Sprintf("WA-%d", i), PublishedRevisionID: id})
		revisions = append(revisions, db.SolutionRevision{ID: id, SolutionAssetID: id, Version: 1, Status: "published", Content: []byte("published evidence")})
		if i <= projected {
			entries = append(entries, db.SolutionCatalogEntry{SolutionAssetID: id, PublishedRevisionID: id, DemandID: fmt.Sprintf("WA-%d", i), ProjectKey: "WA"})
		}
	}
	for _, rows := range []any{assets, revisions} {
		if projected+missing > 0 {
			if err := conn.CreateInBatches(rows, 100).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(entries) > 0 {
		if err := conn.CreateInBatches(entries, 100).Error; err != nil {
			t.Fatal(err)
		}
	}
	return New(conn, Settings{}), conn
}

func TestReconcileSkipsProjectedHistoryAndDoesNotLoadPayloads(t *testing.T) {
	module, conn := reconcileFixture(t, 600, 3)
	probe := &reconcileReadProbe{Interface: logger.Default.LogMode(logger.Silent)}
	module.conn = conn.Session(&gorm.Session{Logger: probe})
	queued, err := module.Reconcile(context.Background())
	if err != nil || queued != 3 {
		t.Fatalf("queued=%d error=%v", queued, err)
	}
	t.Logf("read queries=%d max materialized rows=%d cold revision read=%v", probe.selects, probe.maxRows, probe.coldRevisionRead)
	if probe.selects > 5 || probe.maxRows > 250 || probe.coldRevisionRead {
		t.Fatalf("reconcile reloaded projected history or payloads: queries=%d max_rows=%d cold=%v", probe.selects, probe.maxRows, probe.coldRevisionRead)
	}
	var jobs []db.SolutionCatalogSyncJob
	if err := conn.Order("published_revision_id").Find(&jobs).Error; err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 3 || jobs[0].PublishedRevisionID != 601 || jobs[2].PublishedRevisionID != 603 {
		t.Fatalf("jobs=%+v", jobs)
	}
}

func TestReconcileBatchesAllMissingEntriesAndPreservesProcessingJobs(t *testing.T) {
	module, conn := reconcileFixture(t, 0, 605)
	job := db.SolutionCatalogSyncJob{SolutionAssetID: 251, PublishedRevisionID: 251, Status: SyncProcessing, AttemptCount: 2}
	if err := conn.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	probe := &reconcileReadProbe{Interface: logger.Default.LogMode(logger.Silent)}
	module.conn = conn.Session(&gorm.Session{Logger: probe})
	queued, err := module.Reconcile(context.Background())
	if err != nil || queued != 605 {
		t.Fatalf("queued=%d error=%v", queued, err)
	}
	if probe.selects > 9 || probe.maxRows > 250 || probe.coldRevisionRead {
		t.Fatalf("unbounded reads: %+v", probe)
	}
	var count int64
	if err := conn.Model(&db.SolutionCatalogSyncJob{}).Count(&count).Error; err != nil || count != 605 {
		t.Fatalf("jobs=%d error=%v", count, err)
	}
	if err := conn.First(&job, job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if job.Status != SyncProcessing || job.AttemptCount != 2 {
		t.Fatalf("processing job overwritten: %+v", job)
	}
}

func TestReconcileMissingRevisionReturnsPartialProgress(t *testing.T) {
	module, conn := reconcileFixture(t, 0, 3)
	if err := conn.Delete(&db.SolutionRevision{}, 2).Error; err != nil {
		t.Fatal(err)
	}
	queued, err := module.Reconcile(context.Background())
	if !errors.Is(err, gorm.ErrRecordNotFound) || queued != 1 {
		t.Fatalf("queued=%d error=%v", queued, err)
	}
	var jobs []db.SolutionCatalogSyncJob
	if err := conn.Find(&jobs).Error; err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].PublishedRevisionID != 1 {
		t.Fatalf("jobs=%+v", jobs)
	}
}

func TestReconcileRepairsChangedRevisionAndIgnoresUnpublishedAssets(t *testing.T) {
	module, conn := reconcileFixture(t, 2, 0)
	revision := db.SolutionRevision{ID: 3, SolutionAssetID: 1, Version: 2, Status: "published", Content: []byte("new publication")}
	if err := conn.Create(&revision).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Model(&db.SolutionAsset{}).Where("id = ?", 1).Update("published_revision_id", revision.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Create(&db.SolutionAsset{DemandID: "WA-draft"}).Error; err != nil {
		t.Fatal(err)
	}
	queued, err := module.Reconcile(context.Background())
	if err != nil || queued != 1 {
		t.Fatalf("queued=%d error=%v", queued, err)
	}
	var jobs []db.SolutionCatalogSyncJob
	if err := conn.Find(&jobs).Error; err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].SolutionAssetID != 1 || jobs[0].PublishedRevisionID != 3 {
		t.Fatalf("jobs=%+v", jobs)
	}
}

func TestReconcileEmptyAndCancelled(t *testing.T) {
	module, _ := reconcileFixture(t, 0, 0)
	if queued, err := module.Reconcile(context.Background()); queued != 0 || err != nil {
		t.Fatalf("empty: %d %v", queued, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := module.Reconcile(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error=%v", err)
	}
}

func BenchmarkReconcileProjectedHistory(b *testing.B) {
	for _, rows := range []int{1000, 10000} {
		b.Run(fmt.Sprint(rows), func(b *testing.B) {
			module, _ := reconcileFixture(b, rows, 0)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if queued, err := module.Reconcile(context.Background()); queued != 0 || err != nil {
					b.Fatalf("%d %v", queued, err)
				}
			}
		})
	}
}
