package solutioncatalog

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/db"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Count the actual queries and rows returned through GORM, rather than checking
// generated SQL text. Seeding is excluded by resetting immediately before reads.
type catalogReadMetrics struct {
	logger.Interface
	queries int64
	rows    int64
}

func (metrics *catalogReadMetrics) Trace(_ context.Context, _ time.Time, query func() (string, int64), _ error) {
	_, rows := query()
	metrics.queries++
	if rows > 0 {
		metrics.rows += rows
	}
}

func (metrics *catalogReadMetrics) reset() { metrics.queries, metrics.rows = 0, 0 }

func newCatalogQueryFixture(t testing.TB) (*Module, *gorm.DB, *catalogReadMetrics) {
	t.Helper()
	metrics := &catalogReadMetrics{Interface: logger.Default.LogMode(logger.Silent)}
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: metrics})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := conn.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = pool.Close() })
	if err := conn.AutoMigrate(&db.TaskTelemetry{}, &db.SolutionRevision{}, &db.SolutionCatalogEntry{}, &db.SolutionCatalogSearchToken{}, &db.SolutionComparison{}, &db.SolutionStandardizationProposal{}); err != nil {
		t.Fatal(err)
	}
	return New(conn, Settings{}), conn, metrics
}

func seedCatalogSearch(t testing.TB, conn *gorm.DB, count int) {
	t.Helper()
	entries := make([]db.SolutionCatalogEntry, count)
	tokens := make([]db.SolutionCatalogSearchToken, count)
	for index := range entries {
		id := uint(index + 1)
		entries[index] = db.SolutionCatalogEntry{ID: id, SolutionAssetID: id, PublishedRevisionID: id, ProjectKey: "WA", DemandID: fmt.Sprintf("WA-%d", id)}
		tokens[index] = db.SolutionCatalogSearchToken{EntryID: id, Token: "shared"}
	}
	if err := conn.CreateInBatches(entries, 100).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.CreateInBatches(tokens, 100).Error; err != nil {
		t.Fatal(err)
	}
}

func seedCatalogDetail(t testing.TB, conn *gorm.DB, comparisons int) {
	t.Helper()
	seedCatalogSearch(t, conn, 2)
	payload, err := encodePayload([]byte(strings.Repeat("完整方案正文。", 200)))
	if err != nil {
		t.Fatal(err)
	}
	revision := db.SolutionRevision{ID: 1, SolutionAssetID: 1, Version: 1, Content: payload.stored, ContentEncoding: payload.encoding, ContentHash: payload.hash}
	if err := conn.Create(&revision).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Create(&db.TaskTelemetry{TaskID: "WA-1", Description: "完整需求说明"}).Error; err != nil {
		t.Fatal(err)
	}
	round, err := encodePayload([]byte(`{"verdict":"supported"}`))
	if err != nil {
		t.Fatal(err)
	}
	items := make([]db.SolutionComparison, comparisons)
	proposals := make([]db.SolutionStandardizationProposal, comparisons)
	for index := range items {
		id := uint(index + 1)
		left, right := uint(1), uint(2)
		if index%2 != 0 {
			left, right = right, left
		}
		items[index] = db.SolutionComparison{ID: id, PairKey: fmt.Sprintf("pair-%d", id), LeftEntryID: left, RightEntryID: right, RecallScore: 0.8, RoundOnePayload: round.stored, RoundOneEncoding: round.encoding, RoundOneHash: round.hash, RoundTwoPayload: round.stored, RoundTwoEncoding: round.encoding, RoundTwoHash: round.hash}
		proposals[index] = db.SolutionStandardizationProposal{ComparisonID: id, Status: "pending", Content: payload.stored, ContentEncoding: payload.encoding, ContentHash: payload.hash}
	}
	if err := conn.CreateInBatches(items, 100).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.CreateInBatches(proposals, 100).Error; err != nil {
		t.Fatal(err)
	}
}

func TestCatalogSearchPreservesRankingScopeAndPageContract(t *testing.T) {
	module, conn, _ := newCatalogQueryFixture(t)
	seedCatalogSearch(t, conn, 5)
	if err := conn.Model(&db.SolutionCatalogEntry{}).Where("id = ?", 5).Update("project_key", "DG").Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Create(&[]db.SolutionCatalogSearchToken{{EntryID: 2, Token: "extra"}, {EntryID: 3, Token: "extra"}, {EntryID: 5, Token: "extra"}}).Error; err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		filter ListFilter
		ids    []uint
		total  int64
	}{
		{"rank_then_descending_id", ListFilter{Search: "shared extra", Limit: 2}, []uint{5, 3}, 5},
		{"second_page", ListFilter{Search: "shared extra", Limit: 2, Offset: 2}, []uint{2, 4}, 5},
		{"scope", ListFilter{Search: "shared extra", ProjectScope: []string{"wa"}, Limit: 2}, []uint{3, 2}, 4},
		{"project", ListFilter{Search: "shared extra", ProjectKey: "dg", Limit: 2}, []uint{5}, 1},
		{"forbidden_project", ListFilter{Search: "shared", ProjectScope: []string{"WA"}, ProjectKey: "DG", Limit: 2}, nil, 0},
		{"past_end", ListFilter{Search: "shared", Limit: 2, Offset: 9}, nil, 5},
		{"no_match", ListFilter{Search: "absent", Limit: 2}, nil, 0},
		{"negative_offset", ListFilter{Search: "shared", Limit: 2, Offset: -4}, []uint{5, 4}, 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			page, err := module.List(context.Background(), test.filter)
			if err != nil {
				t.Fatal(err)
			}
			if page.Total != test.total || page.Limit != 2 || page.Offset != max(test.filter.Offset, 0) || page.Items == nil || len(page.Items) != len(test.ids) {
				t.Fatalf("unexpected page: %+v", page)
			}
			for index, id := range test.ids {
				if page.Items[index].ID != id {
					t.Fatalf("item %d = %d, want %d", index, page.Items[index].ID, id)
				}
			}
		})
	}
}

func TestCatalogSearchMaterializesOnlyRequestedPage(t *testing.T) {
	module, conn, metrics := newCatalogQueryFixture(t)
	seedCatalogSearch(t, conn, 2500)
	metrics.reset()
	page, err := module.List(context.Background(), ListFilter{Search: "shared", Limit: 20, Offset: 100})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2500 || len(page.Items) != 20 || page.Items[0].ID != 2400 {
		t.Fatalf("unexpected page: %+v", page)
	}
	t.Logf("queries=%d materialized_rows=%d", metrics.queries, metrics.rows)
	if metrics.rows > 21 || metrics.queries > 2 {
		t.Fatalf("search materialized the match set: queries=%d rows=%d", metrics.queries, metrics.rows)
	}
}

func TestCatalogDetailBatchesComparisonsWithoutLosingDuplicatesOrPayloads(t *testing.T) {
	module, conn, metrics := newCatalogQueryFixture(t)
	seedCatalogDetail(t, conn, 1200)
	metrics.reset()
	detail, err := module.Get(context.Background(), 1, []string{"WA"})
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Comparisons) != 1200 || detail.DemandDescription != "完整需求说明" || detail.Markdown != strings.Repeat("完整方案正文。", 200) {
		t.Fatalf("incomplete detail: comparisons=%d", len(detail.Comparisons))
	}
	for index, comparison := range detail.Comparisons {
		if comparison.Comparison.ID != uint(1200-index) || comparison.Counterpart.ID != 2 || string(comparison.RoundOne) != `{"verdict":"supported"}` || string(comparison.RoundTwo) != `{"verdict":"supported"}` || comparison.Proposal == nil || comparison.Proposal.Markdown != detail.Markdown {
			t.Fatalf("comparison %d lost ordering or complete payloads: %+v", index, comparison)
		}
	}
	t.Logf("queries=%d materialized_rows=%d", metrics.queries, metrics.rows)
	if metrics.queries > 8 {
		t.Fatalf("detail uses per-comparison queries: %d", metrics.queries)
	}
}

func TestCatalogDetailSkipsInaccessibleCounterpartsBeforeDecoding(t *testing.T) {
	module, conn, _ := newCatalogQueryFixture(t)
	seedCatalogDetail(t, conn, 2)
	if err := conn.Model(&db.SolutionCatalogEntry{}).Where("id = 2").Update("project_key", "DG").Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Model(&db.SolutionComparison{}).Where("id > 0").Update("round_one_hash", "corrupt").Error; err != nil {
		t.Fatal(err)
	}
	detail, err := module.Get(context.Background(), 1, []string{"WA"})
	if err != nil || detail.Comparisons == nil || len(detail.Comparisons) != 0 {
		t.Fatalf("inaccessible comparisons must be omitted before decoding: %+v, %v", detail, err)
	}
	if _, err := module.Get(context.Background(), 1, nil); err != ErrIntegrity {
		t.Fatalf("visible corrupt payload must still fail integrity: %v", err)
	}
	if _, err := module.Get(context.Background(), 1, []string{"DG"}); err != ErrNotFound {
		t.Fatalf("inaccessible detail must be hidden: %v", err)
	}
}

func TestCatalogDetailBatchesDistinctCounterpartsAcrossParameterLimit(t *testing.T) {
	module, conn, metrics := newCatalogQueryFixture(t)
	seedCatalogDetail(t, conn, 1200)
	entries := make([]db.SolutionCatalogEntry, 1199)
	for index := range entries {
		id := uint(index + 3)
		entries[index] = db.SolutionCatalogEntry{ID: id, SolutionAssetID: id, PublishedRevisionID: id, ProjectKey: "WA", DemandID: fmt.Sprintf("WA-%d", id)}
	}
	if err := conn.CreateInBatches(entries, 100).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Model(&db.SolutionComparison{}).Where("id > 0").Updates(map[string]any{"left_entry_id": 1, "right_entry_id": gorm.Expr("id + 1")}).Error; err != nil {
		t.Fatal(err)
	}
	metrics.reset()
	detail, err := module.Get(context.Background(), 1, []string{"WA"})
	if err != nil || len(detail.Comparisons) != 1200 {
		t.Fatalf("all comparison batches must be returned: count=%d, %v", len(detail.Comparisons), err)
	}
	for _, comparison := range detail.Comparisons {
		if comparison.Counterpart.ID != comparison.Comparison.ID+1 || comparison.Proposal == nil {
			t.Fatalf("counterpart or proposal missing across batch boundary: %+v", comparison)
		}
	}
	if metrics.queries > 10 {
		t.Fatalf("distinct counterpart lookups were not batched: %d", metrics.queries)
	}
}

func TestCatalogDetailPreservesMissingProposalAndCounterpartSemantics(t *testing.T) {
	module, conn, _ := newCatalogQueryFixture(t)
	seedCatalogDetail(t, conn, 2)
	if err := conn.Where("comparison_id = 1").Delete(&db.SolutionStandardizationProposal{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Model(&db.SolutionComparison{}).Where("id = 2").Update("left_entry_id", 9999).Error; err != nil {
		t.Fatal(err)
	}
	detail, err := module.Get(context.Background(), 1, []string{"WA"})
	if err != nil || len(detail.Comparisons) != 1 || detail.Comparisons[0].Comparison.ID != 1 || detail.Comparisons[0].Proposal != nil {
		t.Fatalf("missing counterpart must be omitted and absent proposal remain nil: %+v, %v", detail.Comparisons, err)
	}
	if err := conn.Where("id > 0").Delete(&db.SolutionComparison{}).Error; err != nil {
		t.Fatal(err)
	}
	detail, err = module.Get(context.Background(), 1, []string{"WA"})
	if err != nil || detail.Comparisons == nil || len(detail.Comparisons) != 0 {
		t.Fatalf("empty comparison collection changed: %+v, %v", detail.Comparisons, err)
	}
}

func BenchmarkCatalogSearchPage(b *testing.B) {
	module, conn, metrics := newCatalogQueryFixture(b)
	seedCatalogSearch(b, conn, 10000)
	metrics.reset()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		page, err := module.List(context.Background(), ListFilter{Search: "shared", Limit: 20, Offset: 5000})
		if err != nil || len(page.Items) != 20 || page.Total != 10000 {
			b.Fatalf("bad page: %+v, %v", page, err)
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(metrics.queries)/float64(b.N), "queries/op")
	b.ReportMetric(float64(metrics.rows)/float64(b.N), "rows/op")
}

func BenchmarkCatalogDetailComparisons(b *testing.B) {
	module, conn, metrics := newCatalogQueryFixture(b)
	seedCatalogDetail(b, conn, 128)
	metrics.reset()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		detail, err := module.Get(context.Background(), 1, []string{"WA"})
		if err != nil || len(detail.Comparisons) != 128 {
			b.Fatalf("bad detail: comparisons=%d, %v", len(detail.Comparisons), err)
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(metrics.queries)/float64(b.N), "queries/op")
	b.ReportMetric(float64(metrics.rows)/float64(b.N), "rows/op")
}
