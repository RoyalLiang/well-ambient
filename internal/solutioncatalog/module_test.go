package solutioncatalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/db"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newCatalogTestModule(t *testing.T, threshold float64) (*Module, *gorm.DB, time.Time) {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.AutoMigrate(
		&db.TaskTelemetry{}, &db.SolutionAsset{}, &db.SolutionRevision{},
		&db.SolutionCatalogEntry{}, &db.SolutionCatalogSearchToken{}, &db.SolutionCatalogSyncJob{},
		&db.SolutionComparison{}, &db.SolutionStandardizationProposal{}, &db.SolutionStandard{}, &db.SolutionStandardRevision{},
	); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	module := New(conn, Settings{CandidateLimit: 8, RecallThreshold: threshold}, WithClock(func() time.Time { return now }))
	return module, conn, now
}

func createPublishedFixture(t *testing.T, conn *gorm.DB, now time.Time, demandID, projectKey, demandTitle, solutionTitle, summary, markdown string) (db.SolutionAsset, db.SolutionRevision) {
	t.Helper()
	if err := conn.Create(&db.TaskTelemetry{
		TaskID: demandID, ProjectKey: projectKey, Title: demandTitle, Description: demandTitle + " 的详细业务约束",
		IssueType: "requirement", Status: "progress", TaskCreatedAt: now.Add(-24 * time.Hour), LastUpdate: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	asset := db.SolutionAsset{DemandID: demandID, Revision: 2, Sequence: 1, CreatedBy: "owner", CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
	if err := conn.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(markdown))
	revision := db.SolutionRevision{
		SolutionAssetID: asset.ID, Version: 1, Kind: "human_draft", Status: "published",
		Title: solutionTitle, Summary: summary, Content: []byte(markdown), ContentEncoding: "identity",
		ContentHash: hex.EncodeToString(hash[:]), ContentBytes: len([]byte(markdown)), StoredBytes: len([]byte(markdown)),
		AuthoredBy: "owner", CreatedAt: now.Add(-30 * time.Minute),
	}
	if err := conn.Create(&revision).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Model(&asset).Update("published_revision_id", revision.ID).Error; err != nil {
		t.Fatal(err)
	}
	asset.PublishedRevisionID = revision.ID
	return asset, revision
}

func TestPublishedCatalogIsIndexedPermissionScopedAndDoesNotDuplicateMarkdown(t *testing.T) {
	module, conn, now := newCatalogTestModule(t, 0.15)
	waAsset, waRevision := createPublishedFixture(t, conn, now, "WA-101", "WA", "统一登录权限治理", "统一登录方案", "会话、权限与回退", "# 统一登录方案\n\n"+strings.Repeat("统一权限校验与会话回退。\n", 120))
	dgAsset, dgRevision := createPublishedFixture(t, conn, now, "DG-202", "DG", "统一登录权限治理", "统一登录现场方案", "会话、权限与现场差异", "# 现场登录方案\n\n统一权限校验与现场参数。")
	for _, fixture := range []struct {
		asset    db.SolutionAsset
		revision db.SolutionRevision
	}{{waAsset, waRevision}, {dgAsset, dgRevision}} {
		if err := EnqueuePublished(conn, fixture.asset, fixture.revision, now); err != nil {
			t.Fatal(err)
		}
	}
	processed, err := module.ProcessPending(context.Background(), 10)
	if err != nil || processed != 2 {
		t.Fatalf("process catalog jobs = %d, err=%v", processed, err)
	}

	var entries []db.SolutionCatalogEntry
	if err := conn.Order("id ASC").Find(&entries).Error; err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].SearchTokenCount == 0 || entries[0].StoredBytes != waRevision.StoredBytes {
		t.Fatalf("catalog projection = %+v", entries)
	}
	if conn.Migrator().HasColumn(&db.SolutionCatalogEntry{}, "content") {
		t.Fatal("catalog duplicated published Markdown instead of referencing the revision")
	}

	waList, err := module.List(context.Background(), ListFilter{ProjectScope: []string{"WA"}, Search: "统一登录"})
	if err != nil || waList.Total != 1 || waList.Items[0].DemandID != "WA-101" {
		t.Fatalf("WA scoped search = %+v, err=%v", waList, err)
	}
	dgList, err := module.List(context.Background(), ListFilter{ProjectScope: []string{"WA"}, ProjectKey: "DG", Search: "统一登录"})
	if err != nil || dgList.Total != 0 || len(dgList.Items) != 0 {
		t.Fatalf("out-of-scope project leaked through search: %+v err=%v", dgList, err)
	}
	detail, err := module.Get(context.Background(), waList.Items[0].ID, []string{"WA"})
	if err != nil || !strings.Contains(detail.Markdown, "会话回退") {
		t.Fatalf("catalog detail = %+v, err=%v", detail, err)
	}
	if _, err := module.Get(context.Background(), waList.Items[0].ID, []string{"DG"}); err != ErrNotFound {
		t.Fatalf("out-of-scope detail error = %v, want not found", err)
	}

	var plans []struct{ Detail string }
	if err := conn.Raw("EXPLAIN QUERY PLAN SELECT entry_id FROM solution_catalog_search_tokens WHERE token IN ? GROUP BY entry_id", []string{"统一"}).Scan(&plans).Error; err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, plan := range plans {
		joined += plan.Detail
	}
	if !strings.Contains(joined, "idx_solution_catalog_token_entry") {
		t.Fatalf("search token lookup is not index-backed: %s", joined)
	}
}

func TestIncrementalRecallQueuesOneRevisionBoundComparisonAndReconcilesMissingProjection(t *testing.T) {
	module, conn, now := newCatalogTestModule(t, 0.10)
	firstAsset, firstRevision := createPublishedFixture(t, conn, now, "WA-301", "WA", "车辆任务失败自动重试", "任务重试与回退方案", "失败重试、幂等和告警", "# 任务重试\n\n失败后按幂等键重试。")
	secondAsset, secondRevision := createPublishedFixture(t, conn, now, "DG-302", "DG", "车辆任务失败重试策略", "任务重试现场方案", "失败重试、幂等和现场告警", "# 任务重试\n\n失败后按幂等键重试，并保留现场差异。")
	if err := EnqueuePublished(conn, firstAsset, firstRevision, now); err != nil {
		t.Fatal(err)
	}
	if _, err := module.ProcessPending(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if err := EnqueuePublished(conn, secondAsset, secondRevision, now); err != nil {
		t.Fatal(err)
	}
	if _, err := module.ProcessPending(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	var comparisons []db.SolutionComparison
	if err := conn.Find(&comparisons).Error; err != nil {
		t.Fatal(err)
	}
	if len(comparisons) != 1 || comparisons[0].Status != ComparisonQueued || comparisons[0].Stage != ComparisonStageRoundOne {
		t.Fatalf("comparison candidates = %+v", comparisons)
	}
	if err := EnqueuePublished(conn, secondAsset, secondRevision, now); err != nil {
		t.Fatal(err)
	}
	if _, err := module.ProcessPending(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	var comparisonCount int64
	if err := conn.Model(&db.SolutionComparison{}).Count(&comparisonCount).Error; err != nil || comparisonCount != 1 {
		t.Fatalf("replayed sync comparison count=%d err=%v", comparisonCount, err)
	}

	if err := conn.Where("solution_asset_id = ?", firstAsset.ID).Delete(&db.SolutionCatalogEntry{}).Error; err != nil {
		t.Fatal(err)
	}
	queued, err := module.Reconcile(context.Background())
	if err != nil || queued != 1 {
		t.Fatalf("reconcile queued=%d err=%v", queued, err)
	}
	if _, err := module.ProcessPending(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	var repaired db.SolutionCatalogEntry
	if err := conn.Where("solution_asset_id = ?", firstAsset.ID).First(&repaired).Error; err != nil || repaired.PublishedRevisionID != firstRevision.ID {
		t.Fatalf("repaired catalog entry = %+v err=%v", repaired, err)
	}
}

func TestTwoRoundComparisonCreatesCompressedReviewProposalAndAcceptedStandardRevision(t *testing.T) {
	module, conn, now := newCatalogTestModule(t, 0.10)
	firstAsset, firstRevision := createPublishedFixture(t, conn, now, "WA-401", "WA", "统一任务失败重试", "任务失败重试", "幂等重试与告警", "# 重试方案 A\n\n幂等重试。")
	secondAsset, secondRevision := createPublishedFixture(t, conn, now, "DG-402", "DG", "统一任务失败重试", "任务失败重试", "幂等重试与告警", "# 重试方案 B\n\n幂等重试。")
	for _, fixture := range []struct {
		asset    db.SolutionAsset
		revision db.SolutionRevision
	}{{firstAsset, firstRevision}, {secondAsset, secondRevision}} {
		if err := EnqueuePublished(conn, fixture.asset, fixture.revision, now); err != nil {
			t.Fatal(err)
		}
		if _, err := module.ProcessPending(context.Background(), 1); err != nil {
			t.Fatal(err)
		}
	}

	roundOne, err := module.ClaimNextComparison(context.Background())
	if err != nil || roundOne == nil || roundOne.Comparison.Stage != ComparisonStageRoundOne {
		t.Fatalf("claim round one = %+v err=%v", roundOne, err)
	}
	if err := module.CompleteRoundOne(context.Background(), roundOne.Comparison.ID, "prompt:11", RoundOneResult{
		Equivalent: true, Score: 0.91, Reason: "业务目标相同", SharedIntent: []string{"失败后安全重试"},
	}); err != nil {
		t.Fatal(err)
	}
	roundTwo, err := module.ClaimNextComparison(context.Background())
	if err != nil || roundTwo == nil || roundTwo.Comparison.Stage != ComparisonStageRoundTwo {
		t.Fatalf("claim round two = %+v err=%v", roundTwo, err)
	}
	proposalMarkdown := "# 标准任务重试方案\n\n" + strings.Repeat("- 公共核心：幂等重试、告警与回退。\n", 200)
	if err := module.CompleteRoundTwo(context.Background(), roundTwo.Comparison.ID, "prompt:12", RoundTwoResult{
		Compatible: true, Standardizable: true, Score: 0.88, Summary: "公共核心稳定，现场参数作为差异",
		CommonCore: []string{"幂等重试", "失败告警"}, ProposalTitle: "标准任务重试方案", ProposalMarkdown: proposalMarkdown,
	}); err != nil {
		t.Fatal(err)
	}
	var proposal db.SolutionStandardizationProposal
	if err := conn.Where("comparison_id = ?", roundTwo.Comparison.ID).First(&proposal).Error; err != nil {
		t.Fatal(err)
	}
	if proposal.Status != ProposalPending || proposal.ContentEncoding != "gzip" || proposal.StoredBytes >= proposal.ContentBytes {
		t.Fatalf("proposal compression/state = %+v", proposal)
	}
	accepted, standard, err := module.ReviewProposal(context.Background(), ReviewProposalCommand{
		ProposalID: proposal.ID, Action: "accept", ReviewNote: "治理评审通过", Actor: "admin", ProjectScope: []string{"WA", "DG"},
	})
	if err != nil || standard == nil || accepted.Proposal.Status != ProposalAccepted || standard.Revision.Version != 1 || standard.Markdown != strings.TrimSpace(proposalMarkdown) {
		t.Fatalf("accepted proposal=%+v standard=%+v err=%v", accepted, standard, err)
	}
	standards, err := module.ListStandards(context.Background(), []string{"WA", "DG"})
	if err != nil || len(standards) != 1 || standards[0].Version != 1 {
		t.Fatalf("standards = %+v err=%v", standards, err)
	}
	if hidden, err := module.ListStandards(context.Background(), []string{"WA"}); err != nil || len(hidden) != 0 {
		t.Fatalf("standard leaked when one source project is outside scope: %+v err=%v", hidden, err)
	}
}

func TestComparisonJSONParserAcceptsFencedJSONAndRejectsBrokenOutput(t *testing.T) {
	one, err := ParseRoundOne("```json\n{\"equivalent\":true,\"score\":1.4,\"reason\":\"same\"}\n```")
	if err != nil || !one.Equivalent || one.Score != 1 {
		t.Fatalf("round one parse = %+v err=%v", one, err)
	}
	if _, err := ParseRoundTwo("not-json"); err == nil {
		t.Fatal("broken model output was accepted")
	}
}
