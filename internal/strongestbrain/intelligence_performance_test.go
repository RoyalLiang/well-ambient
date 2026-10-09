package strongestbrain_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"well-ambient/internal/agentruntime"
	"well-ambient/internal/db"
	"well-ambient/internal/strongestbrain"
)

type intelligenceQueryCounts struct {
	total          int
	bindings       int
	maxBindingArgs int
}

func newIntelligencePerformanceDB(tb testing.TB) *gorm.DB {
	tb.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		tb.Fatal(err)
	}
	connection, err := database.DB()
	if err != nil {
		tb.Fatal(err)
	}
	connection.SetMaxOpenConns(1)
	tb.Cleanup(func() { _ = connection.Close() })
	if err := database.AutoMigrate(&db.Capability{}, &db.CapabilityVersion{}, &db.AgentRun{},
		&db.RunCapabilityBinding{}, &db.AgentRunEvent{}, &db.CapabilityProposal{}, &db.CodeReviewRun{}); err != nil {
		tb.Fatal(err)
	}
	return database
}

func countIntelligenceQueries(tb testing.TB, database *gorm.DB) *intelligenceQueryCounts {
	tb.Helper()
	counts := &intelligenceQueryCounts{}
	err := database.Callback().Query().After("gorm:query").Register("test:intelligence_queries", func(tx *gorm.DB) {
		counts.total++
		if tx.Statement.Table == "run_capability_bindings" {
			counts.bindings++
			if len(tx.Statement.Vars) > counts.maxBindingArgs {
				counts.maxBindingArgs = len(tx.Statement.Vars)
			}
		}
	})
	if err != nil {
		tb.Fatal(err)
	}
	return counts
}

func seedIntelligenceRuns(tb testing.TB, database *gorm.DB, count int) {
	tb.Helper()
	if count == 0 {
		return
	}
	runs := make([]db.AgentRun, count)
	bindings := make([]db.RunCapabilityBinding, 0, count*2)
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	for i := range runs {
		runs[i] = db.AgentRun{ID: uint(i + 1), RunKey: fmt.Sprintf("perf-%d", i), AgentKind: "code_review",
			State: "completed", PromptTokens: 100, DurationMs: 200, CreatedAt: now.Add(time.Duration(i) * time.Second)}
		for _, key := range []string{"code_review", "knowledge.search"} {
			bindings = append(bindings, db.RunCapabilityBinding{RunID: runs[i].ID, CapabilityKey: key, Version: 1, Digest: "sha256:" + key})
		}
	}
	if err := database.CreateInBatches(&runs, 25).Error; err != nil {
		tb.Fatal(err)
	}
	if err := database.CreateInBatches(&bindings, 25).Error; err != nil {
		tb.Fatal(err)
	}
}

func TestIntelligenceComputeReportBoundedBindingQueries(t *testing.T) {
	for _, tc := range []struct {
		name     string
		runs     int
		limit    int
		selected int
	}{
		{"empty", 0, 500, 0},
		{"single", 1, 500, 1},
		{"many", 12, 500, 12},
		{"batch", 500, 500, 500},
		{"batch_plus_one", 501, 501, 501},
		{"maximum", 1000, 1000, 1000},
		{"limit_excludes_older_runs", 31, 10, 10},
		{"invalid_limit_uses_default", 501, 1001, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database := newIntelligencePerformanceDB(t)
			seedIntelligenceRuns(t, database, tc.runs)
			counts := countIntelligenceQueries(t, database)
			engine := strongestbrain.NewIntelligenceEngine(database, agentruntime.NewRegistry(database))
			report, err := engine.ComputeReport(context.Background(), tc.limit)
			if err != nil {
				t.Fatal(err)
			}
			if report.Summary.TotalRuns != tc.selected {
				t.Fatalf("selected runs = %d, want %d", report.Summary.TotalRuns, tc.selected)
			}
			for _, metric := range report.Versions {
				if metric.TotalRuns != tc.selected || metric.CompletedRuns != tc.selected {
					t.Fatalf("binding metrics changed: %+v", metric)
				}
			}
			wantBindings := (tc.selected + 499) / 500
			t.Logf("runs=%d selected=%d queries=%d binding_queries=%d max_binding_args=%d", tc.runs, tc.selected, counts.total, counts.bindings, counts.maxBindingArgs)
			if counts.bindings != wantBindings || counts.total != 6+wantBindings {
				t.Fatalf("queries = %d (%d bindings), want %d (%d bindings)", counts.total, counts.bindings, 6+wantBindings, wantBindings)
			}
			if counts.maxBindingArgs > 500 {
				t.Fatalf("binding query has %d parameters, want at most 500", counts.maxBindingArgs)
			}
		})
	}
}

func TestIntelligenceComputeReportPreservesMixedRunMetrics(t *testing.T) {
	database := newIntelligencePerformanceDB(t)
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	runs := []db.AgentRun{
		{ID: 1, RunKey: "completed", AgentKind: "code_review", State: "completed", PromptTokens: 900, DurationMs: 90, CreatedAt: now},
		{ID: 2, RunKey: "partial", AgentKind: "code_review", State: "partial", PromptTokens: 600, DurationMs: 60, CreatedAt: now.Add(time.Second)},
		{ID: 3, RunKey: "unbound", AgentKind: "code_review", State: "failed", PromptTokens: 300, DurationMs: 30, CreatedAt: now.Add(2 * time.Second)},
	}
	bindings := []db.RunCapabilityBinding{
		{ID: 1, RunID: 1, CapabilityKey: "code_review", Version: 1, Digest: "review-v1"},
		{ID: 2, RunID: 1, CapabilityKey: "knowledge.search", Version: 2, Digest: "knowledge-v2"},
		{ID: 3, RunID: 2, CapabilityKey: "code_review", Version: 1, Digest: "review-v1"},
		{ID: 4, RunID: 2, CapabilityKey: "knowledge.search", Version: 2, Digest: "knowledge-v2"},
		{ID: 5, RunID: 999, CapabilityKey: "unselected", Version: 1, Digest: "excluded"},
	}
	legacy := []db.CodeReviewRun{
		{Key: "legacy-completed", Status: "completed", SkillName: "code_review", SkillVersion: 1, SkillHash: "review-v1",
			ReportJSON: `{"evidence_complete":false,"findings":[{"severity":"high"}],"questions":["q1","q2"]}`},
		{Key: "legacy-failed", Status: "failed", SkillVersion: 1, SkillHash: "review-v1", RetryOfID: 1, ReportJSON: "invalid"},
		{Key: "legacy-running", Status: "running", SkillName: "other", SkillVersion: 3},
	}
	for _, rows := range []any{&runs, &bindings, &legacy, &db.AgentRunEvent{RunID: 999, Sequence: 1, EventType: agentruntime.EventValidationFailed}} {
		if err := database.Create(rows).Error; err != nil {
			t.Fatal(err)
		}
	}
	engine := strongestbrain.NewIntelligenceEngine(database, agentruntime.NewRegistry(database))
	report, err := engine.ComputeReport(context.Background(), 500)
	if err != nil {
		t.Fatal(err)
	}
	// Preserve current denominators and all-history security counts. Correcting
	// these governance definitions is intentionally separate from query batching.
	wantSummary := strongestbrain.DimensionSummary{TotalRuns: 6, CompletedRuns: 2, PartialRuns: 1, FailedRuns: 2,
		RetriedRuns: 1, ActiveRuns: 1, EvidenceGapRuns: 2, HighRiskFindings: 1, QuestionsCount: 2,
		AvgPromptTokens: 300, AvgDurationMs: 30, SecurityRejects: 1}
	if !reflect.DeepEqual(report.Summary, wantSummary) {
		t.Fatalf("summary = %+v, want %+v", report.Summary, wantSummary)
	}
	wantMetrics := map[string]strongestbrain.CapabilityVersionMetric{
		"code_review": {CapabilityKey: "code_review", Version: 1, Digest: "review-v1", TotalRuns: 4, CompletedRuns: 2,
			PartialRuns: 1, FailedRuns: 1, RetriedRuns: 1, EvidenceGapRuns: 2, HighRiskFindings: 1, QuestionsCount: 2,
			QualityScore: 50, EvidenceScore: 50, StabilityScore: 50},
		"knowledge.search": {CapabilityKey: "knowledge.search", Version: 2, Digest: "knowledge-v2", TotalRuns: 2,
			CompletedRuns: 1, PartialRuns: 1, QualityScore: 50, EvidenceScore: 100, StabilityScore: 100},
		"other": {CapabilityKey: "other", Version: 3, TotalRuns: 1, EvidenceScore: 100, StabilityScore: 100},
	}
	if len(report.Versions) != len(wantMetrics) {
		t.Fatalf("metrics = %+v", report.Versions)
	}
	for _, metric := range report.Versions {
		if !reflect.DeepEqual(metric, wantMetrics[metric.CapabilityKey]) {
			t.Fatalf("metric = %+v, want %+v", metric, wantMetrics[metric.CapabilityKey])
		}
	}
	if len(report.RecentExceptions) != 3 || report.RecentExceptions[0].RunKey != "unbound" ||
		report.RecentExceptions[0].CapabilityKey != "unknown" || report.RecentExceptions[1].RunKey != "partial" ||
		report.RecentExceptions[1].CapabilityKey != "code_review" || report.RecentExceptions[1].Version != 1 {
		t.Fatalf("exception order or first binding changed: %+v", report.RecentExceptions)
	}
}

func TestIntelligenceComputeReportPreservesBestEffortBindingFailure(t *testing.T) {
	database := newIntelligencePerformanceDB(t)
	seedIntelligenceRuns(t, database, 2)
	if err := database.Model(&db.AgentRun{}).Where("id = ?", 1).Update("state", "failed").Error; err != nil {
		t.Fatal(err)
	}
	err := database.Callback().Query().Before("gorm:query").Register("test:fail_bindings", func(tx *gorm.DB) {
		if tx.Statement.Table == "run_capability_bindings" {
			tx.AddError(errors.New("binding storage unavailable"))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	engine := strongestbrain.NewIntelligenceEngine(database, agentruntime.NewRegistry(database))
	report, err := engine.ComputeReport(context.Background(), 500)
	if err != nil || report == nil {
		t.Fatalf("best-effort error behavior changed: report=%v error=%v", report, err)
	}
	if report.Summary.TotalRuns != 2 || len(report.Versions) != 0 || len(report.RecentExceptions) != 1 ||
		report.RecentExceptions[0].CapabilityKey != "unknown" {
		t.Fatalf("best-effort report changed: %+v", report)
	}
}

func BenchmarkIntelligenceComputeReport(b *testing.B) {
	for _, count := range []int{10, 500, 1000} {
		b.Run(fmt.Sprintf("runs_%d", count), func(b *testing.B) {
			database := newIntelligencePerformanceDB(b)
			seedIntelligenceRuns(b, database, count)
			counts := countIntelligenceQueries(b, database)
			engine := strongestbrain.NewIntelligenceEngine(database, agentruntime.NewRegistry(database))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				report, err := engine.ComputeReport(context.Background(), count)
				if err != nil || report.Summary.TotalRuns != count {
					b.Fatalf("report failed: report=%v error=%v", report, err)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(counts.total)/float64(b.N), "queries/op")
			b.ReportMetric(float64(counts.bindings)/float64(b.N), "binding_queries/op")
		})
	}
}
