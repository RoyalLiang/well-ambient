package solutions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/db"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testModule(t *testing.T, threshold int) (*Module, *gorm.DB) {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := conn.AutoMigrate(
		&db.SolutionAsset{}, &db.SolutionRevision{}, &db.SolutionSourceRef{},
		&db.SolutionPolishJob{}, &db.SolutionPromptTemplate{}, &db.SolutionJiraOutbox{},
		&db.SolutionCatalogSyncJob{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	module := New(conn, WithClock(func() time.Time { return now }), WithCompressionThreshold(threshold))
	prompt := db.SolutionPromptTemplate{
		Purpose: "solution_polish", ScopeType: "global", ScopeID: "", Version: 1,
		Status: "active", Name: "default", SystemPrompt: db.DefaultSolutionPolishPrompt,
		CreatedBy: "system", CreatedAt: now,
	}
	if err := conn.Create(&prompt).Error; err != nil {
		t.Fatalf("seed prompt: %v", err)
	}
	return module, conn
}

func TestDraftCASCompressionAndPublishedFork(t *testing.T) {
	module, conn := testModule(t, 128)
	ctx := context.Background()
	large := "# 初稿\n\n" + strings.Repeat("相同的可压缩方案内容。\n", 300)

	created, err := module.EnsureDraft(ctx, EnsureDraftCommand{DemandID: "WA-101", Title: "方案", Markdown: large, Actor: "Alice"})
	if err != nil {
		t.Fatalf("ensure draft: %v", err)
	}
	if created.Asset.Revision != 1 || created.Working == nil || created.Working.Markdown != normalizeMarkdown(large) {
		t.Fatalf("unexpected created workspace: %+v", created)
	}
	if created.Working.ContentEncoding != "gzip" || created.Working.StoredBytes >= created.Working.ContentBytes {
		t.Fatalf("large markdown was not compressed: %+v", created.Working)
	}

	saved, err := module.SaveDraft(ctx, SaveDraftCommand{
		DemandID: "WA-101", ExpectedRevision: created.Asset.Revision, BaseRevisionID: created.Working.ID,
		Title: "方案", Markdown: large + "\n## 验收\n\n- 可验证", Actor: "Bob",
	})
	if err != nil {
		t.Fatalf("save draft: %v", err)
	}
	if saved.Asset.Revision != 2 || saved.Working == nil || saved.Working.ParentRevisionID != created.Working.ID {
		t.Fatalf("save did not create an immutable child: %+v", saved)
	}

	_, err = module.SaveDraft(ctx, SaveDraftCommand{
		DemandID: "WA-101", ExpectedRevision: created.Asset.Revision, BaseRevisionID: created.Working.ID,
		Markdown: "# stale overwrite", Actor: "Carol",
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("stale save error = %v, want conflict", err)
	}
	afterConflict, _ := module.GetWorkspace(ctx, "WA-101")
	if afterConflict.Working.ID != saved.Working.ID || strings.Contains(afterConflict.Working.Markdown, "stale overwrite") {
		t.Fatalf("stale save changed working content: %+v", afterConflict.Working)
	}

	published, err := module.Publish(ctx, PublishCommand{
		DemandID: "WA-101", ExpectedRevision: saved.Asset.Revision, Actor: "Owner",
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.Published == nil || published.Working == nil || published.Published.ID != published.Working.ID || published.Working.Status != StatusPublished {
		t.Fatalf("published pointers are inconsistent: %+v", published)
	}
	var outbox []db.SolutionJiraOutbox
	if err := conn.Find(&outbox).Error; err != nil || len(outbox) != 0 {
		t.Fatalf("publish outbox = %+v, err=%v", outbox, err)
	}
	var catalogJobs []db.SolutionCatalogSyncJob
	if err := conn.Find(&catalogJobs).Error; err != nil || len(catalogJobs) != 1 || catalogJobs[0].PublishedRevisionID != published.Published.ID {
		t.Fatalf("publish catalog hand-off = %+v, err=%v", catalogJobs, err)
	}

	forked, err := module.ForkDraft(ctx, ForkDraftCommand{DemandID: "WA-101", ExpectedRevision: published.Asset.Revision, Actor: "Alice"})
	if err != nil {
		t.Fatalf("fork published: %v", err)
	}
	if forked.Working == nil || forked.Working.Status != StatusDraft || forked.Working.ParentRevisionID != published.Published.ID {
		t.Fatalf("published edit did not fork a draft: %+v", forked.Working)
	}
	if forked.Published == nil || forked.Published.ID != published.Published.ID {
		t.Fatalf("fork changed published pointer: %+v", forked.Published)
	}
}

func TestSourceSnapshotsPolishCandidateAndHumanApply(t *testing.T) {
	module, _ := testModule(t, 64)
	ctx := context.Background()
	workspace, err := module.EnsureDraft(ctx, EnsureDraftCommand{
		DemandID: "WA-202", Markdown: "# 当前人工方案\n\n保留人工内容", Actor: "Alice",
	})
	if err != nil {
		t.Fatalf("ensure draft: %v", err)
	}
	createdAt := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	first, err := module.ObserveSource(ctx, ObserveSourceCommand{
		DemandID: "WA-202", SourceSystem: "jira", ExternalID: "comment-7", Author: "PM",
		Marker: "[方案]", Eligible: true, Body: "[方案]\n补充异常回退", SourceCreatedAt: &createdAt,
	})
	if err != nil || first.Replayed {
		t.Fatalf("observe first = %+v, err=%v", first, err)
	}
	replay, err := module.ObserveSource(ctx, ObserveSourceCommand{
		DemandID: "WA-202", SourceSystem: "jira", ExternalID: "comment-7", Author: "PM",
		Marker: "[方案]", Eligible: true, Body: "[方案]\n补充异常回退", SourceCreatedAt: &createdAt,
	})
	if err != nil || !replay.Replayed || replay.Source.ID != first.Source.ID {
		t.Fatalf("source replay = %+v, err=%v", replay, err)
	}
	edited, err := module.ObserveSource(ctx, ObserveSourceCommand{
		DemandID: "WA-202", SourceSystem: "jira", ExternalID: "comment-7", Author: "PM",
		Marker: "[方案]", Eligible: true, Body: "[方案]\n补充异常回退与权限", SourceCreatedAt: &createdAt,
	})
	if err != nil || edited.Replayed || edited.Source.ID == first.Source.ID {
		t.Fatalf("edited source should create a snapshot: %+v, err=%v", edited, err)
	}
	reverted, err := module.ObserveSource(ctx, ObserveSourceCommand{
		DemandID: "WA-202", SourceSystem: "jira", ExternalID: "comment-7", Author: "PM",
		Marker: "[方案]", Eligible: true, Body: "[方案]\n补充异常回退", SourceCreatedAt: &createdAt,
	})
	if err != nil || reverted.Replayed || reverted.Source.ID != first.Source.ID || !reverted.Source.Current {
		t.Fatalf("A-B-A source edit should reactivate the immutable A snapshot: %+v, err=%v", reverted, err)
	}
	if err := module.ReconcileSourceSet(ctx, "WA-202", "jira", nil); err != nil {
		t.Fatalf("reconcile removed source: %v", err)
	}
	var currentSourceCount int64
	if err := module.conn.Model(&db.SolutionSourceRef{}).Where("solution_asset_id = ? AND current = ?", workspace.Asset.ID, true).Count(&currentSourceCount).Error; err != nil || currentSourceCount != 0 {
		t.Fatalf("removed Jira source remains current: count=%d err=%v", currentSourceCount, err)
	}
	if _, err := module.ObserveSource(ctx, ObserveSourceCommand{
		DemandID: "WA-202", SourceSystem: "jira", ExternalID: "comment-7", Author: "PM",
		Marker: "[方案]", Eligible: true, Body: "[方案]\n补充异常回退与权限", SourceCreatedAt: &createdAt,
	}); err != nil {
		t.Fatalf("restore edited source: %v", err)
	}

	job, replayed, err := module.RequestPolish(ctx, RequestPolishCommand{DemandID: "WA-202", RequestedBy: "jira-sync"})
	if err != nil || replayed || job.InputRevisionID != workspace.Working.ID {
		t.Fatalf("request polish = %+v replay=%v err=%v", job, replayed, err)
	}
	replayedJob, replayed, err := module.RequestPolish(ctx, RequestPolishCommand{DemandID: "WA-202", RequestedBy: "jira-sync"})
	if err != nil || !replayed || replayedJob.ID != job.ID {
		t.Fatalf("polish replay = %+v replay=%v err=%v", replayedJob, replayed, err)
	}

	claimed, err := module.ClaimNextJob(ctx)
	if err != nil || claimed == nil || claimed.Job.ID != job.ID || len(claimed.Sources) != 1 || !strings.Contains(claimed.Sources[0].Markdown, "权限") {
		t.Fatalf("claim = %+v, err=%v", claimed, err)
	}
	candidate, err := module.CompletePolish(ctx, CompletePolishCommand{
		JobID: job.ID, Markdown: "# Agent 候选\n\n保留人工内容\n\n## 异常与回退\n\n- 回滚", ModelVersion: "test-model",
	})
	if err != nil || candidate.Status != StatusCandidate || candidate.DerivedFromRevisionID != workspace.Working.ID {
		t.Fatalf("candidate = %+v, err=%v", candidate, err)
	}
	afterCandidate, _ := module.GetWorkspace(ctx, "WA-202")
	if afterCandidate.Working.ID != workspace.Working.ID || afterCandidate.Working.Markdown != workspace.Working.Markdown {
		t.Fatalf("agent completion overwrote editor draft: before=%+v after=%+v", workspace.Working, afterCandidate.Working)
	}
	if len(afterCandidate.Candidates) != 1 || afterCandidate.Candidates[0].ID != candidate.ID {
		t.Fatalf("candidate not visible: %+v", afterCandidate.Candidates)
	}

	applied, err := module.ApplyCandidate(ctx, ApplyCandidateCommand{
		DemandID: "WA-202", CandidateID: candidate.ID, ExpectedRevision: afterCandidate.Asset.Revision, Actor: "Alice",
	})
	if err != nil {
		t.Fatalf("apply candidate: %v", err)
	}
	if applied.Working == nil || applied.Working.DerivedFromRevisionID != candidate.ID || !strings.Contains(applied.Working.Markdown, "Agent 候选") {
		t.Fatalf("candidate did not create human draft: %+v", applied.Working)
	}
	if len(applied.Candidates) != 0 {
		t.Fatalf("applied candidate remains actionable: %+v", applied.Candidates)
	}
}

func TestInitialAgentDraftUsesHiddenSeedAndBecomesFirstWorkingVersion(t *testing.T) {
	module, conn := testModule(t, 64)
	ctx := context.Background()
	if _, err := module.ObserveSource(ctx, ObserveSourceCommand{
		DemandID: "WA-203", SourceSystem: "jira", ExternalID: "comment-8", Author: "PM",
		Marker: "[方案]", Eligible: true, Body: "[方案]\n直接生成第一版完整方案",
	}); err != nil {
		t.Fatalf("observe source: %v", err)
	}

	job, replayed, err := module.RequestInitialDraft(ctx, RequestInitialDraftCommand{
		DemandID: "WA-203", ProjectKey: "WA", Title: "首次 Agent 方案",
		Markdown: "# 首次 Agent 方案\n\n需求背景", RequestedBy: "jira-sync",
	})
	if err != nil || replayed || job.InputRevisionID == 0 {
		t.Fatalf("request initial draft = %+v replay=%v err=%v", job, replayed, err)
	}
	before, err := module.GetWorkspace(ctx, "WA-203")
	if err != nil || before.Working != nil || len(before.History) != 0 || len(before.Candidates) != 0 {
		t.Fatalf("hidden generation seed leaked as a solution version: workspace=%+v err=%v", before, err)
	}
	var seed db.SolutionRevision
	if err := conn.First(&seed, job.InputRevisionID).Error; err != nil {
		t.Fatalf("load generation seed: %v", err)
	}
	if seed.Kind != KindSystemSeed || seed.Version != 0 {
		t.Fatalf("generation seed = %+v, want hidden v0 system seed", seed)
	}

	claimed, err := module.ClaimNextJob(ctx)
	if err != nil || claimed == nil || !strings.Contains(claimed.Input.Markdown, "需求背景") {
		t.Fatalf("claim initial draft = %+v err=%v", claimed, err)
	}
	agentDraft, err := module.CompletePolish(ctx, CompletePolishCommand{
		JobID: job.ID, Markdown: "# Agent 第一版\n\n## 验收\n\n- 可直接编辑", ModelVersion: "test-model",
	})
	if err != nil {
		t.Fatalf("complete initial draft: %v", err)
	}
	if agentDraft.Version != 1 || agentDraft.Kind != KindAgentDraft || agentDraft.Status != StatusDraft {
		t.Fatalf("initial agent output = %+v, want editable v1", agentDraft)
	}
	after, err := module.GetWorkspace(ctx, "WA-203")
	if err != nil || after.Working == nil || after.Working.ID != agentDraft.ID || len(after.Candidates) != 0 || len(after.History) != 1 {
		t.Fatalf("initial agent output did not become the only visible working version: workspace=%+v err=%v", after, err)
	}
}

func TestInitialAgentDraftDoesNotQueueWhenHumanDraftExists(t *testing.T) {
	module, conn := testModule(t, 64)
	ctx := context.Background()
	if _, err := module.EnsureDraft(ctx, EnsureDraftCommand{
		DemandID: "WA-204", Title: "人工方案", Markdown: "# 人工方案\n\n已确认事实", Actor: "Alice",
	}); err != nil {
		t.Fatalf("ensure human draft: %v", err)
	}
	if _, err := module.ObserveSource(ctx, ObserveSourceCommand{
		DemandID: "WA-204", SourceSystem: "jira", ExternalID: "comment-9", Author: "PM",
		Marker: "[方案]", Eligible: true, Body: "[方案]\n补充评论",
	}); err != nil {
		t.Fatalf("observe source: %v", err)
	}

	_, _, err := module.RequestInitialDraft(ctx, RequestInitialDraftCommand{
		DemandID: "WA-204", ProjectKey: "WA", Title: "人工方案",
		Markdown: "# 人工方案\n\n需求背景", RequestedBy: "jira-sync",
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("request with human draft error = %v, want conflict", err)
	}
	var jobCount int64
	if err := conn.Model(&db.SolutionPolishJob{}).Count(&jobCount).Error; err != nil || jobCount != 0 {
		t.Fatalf("human draft queued background agent work: count=%d err=%v", jobCount, err)
	}
}

func TestInitialAgentDraftDuplicateJobsConvergeOnOneWorkingVersion(t *testing.T) {
	module, _ := testModule(t, 64)
	ctx := context.Background()
	if _, err := module.ObserveSource(ctx, ObserveSourceCommand{
		DemandID: "WA-205", SourceSystem: "jira", ExternalID: "comment-10", Author: "PM",
		Marker: "[方案]", Eligible: true, Body: "[方案]\n幂等首稿",
	}); err != nil {
		t.Fatalf("observe source: %v", err)
	}
	first, _, err := module.RequestInitialDraft(ctx, RequestInitialDraftCommand{
		DemandID: "WA-205", ProjectKey: "WA", Title: "幂等首稿", Markdown: "# 幂等首稿",
		RequestedBy: "jira-sync", IdempotencyKey: "initial-one",
	})
	if err != nil {
		t.Fatalf("request first job: %v", err)
	}
	second, _, err := module.RequestInitialDraft(ctx, RequestInitialDraftCommand{
		DemandID: "WA-205", ProjectKey: "WA", Title: "幂等首稿", Markdown: "# 幂等首稿",
		RequestedBy: "jira-sync", IdempotencyKey: "initial-two",
	})
	if err != nil {
		t.Fatalf("request duplicate job: %v", err)
	}
	for index, job := range []db.SolutionPolishJob{first, second} {
		claimed, err := module.ClaimNextJob(ctx)
		if err != nil || claimed == nil || claimed.Job.ID != job.ID {
			t.Fatalf("claim job %d = %+v err=%v", index+1, claimed, err)
		}
		if _, err := module.CompletePolish(ctx, CompletePolishCommand{
			JobID: job.ID, Markdown: fmt.Sprintf("# Agent 输出 %d", index+1), ModelVersion: "test-model",
		}); err != nil {
			t.Fatalf("complete job %d: %v", index+1, err)
		}
	}
	workspace, err := module.GetWorkspace(ctx, "WA-205")
	if err != nil || workspace.Working == nil || strings.TrimSpace(workspace.Working.Markdown) != "# Agent 输出 1" || len(workspace.History) != 1 || len(workspace.Candidates) != 0 {
		t.Fatalf("duplicate initial jobs created branching versions: workspace=%+v err=%v", workspace, err)
	}
}

func TestRetryFailedInitialDraftCreatesAuditableIdempotentJob(t *testing.T) {
	module, conn := testModule(t, 64)
	ctx := context.Background()
	if _, err := module.ObserveSource(ctx, ObserveSourceCommand{
		DemandID: "WA-207", SourceSystem: "jira", ExternalID: "comment-12", Author: "PM",
		Marker: "[方案]", Eligible: true, Body: "[方案]\n需要重新生成首版方案",
	}); err != nil {
		t.Fatalf("observe source: %v", err)
	}
	failed, _, err := module.RequestInitialDraft(ctx, RequestInitialDraftCommand{
		DemandID: "WA-207", ProjectKey: "WA", Title: "失败首稿", Markdown: "# 失败首稿",
		RequestedBy: "jira-sync",
	})
	if err != nil {
		t.Fatalf("request initial draft: %v", err)
	}
	providerError := `LLM provider returned status 524: {"title":"Error 524: A timeout occurred","retryable":true}`
	if err := conn.Model(&db.SolutionPolishJob{}).Where("id = ?", failed.ID).Updates(map[string]any{
		"status": JobFailed, "attempt_count": 3, "last_error": providerError,
	}).Error; err != nil {
		t.Fatalf("mark terminal failure: %v", err)
	}

	retried, replayed, err := module.RetryFailedPolish(ctx, RetryFailedPolishCommand{
		DemandID: "WA-207", JobID: failed.ID, RequestedBy: "Alice",
	})
	if err != nil || replayed {
		t.Fatalf("retry failed job = %+v replayed=%v err=%v", retried, replayed, err)
	}
	if retried.ID == failed.ID || retried.Status != JobQueued || retried.AttemptCount != 0 || retried.LastError != "" {
		t.Fatalf("manual retry did not create a fresh queued job: %+v", retried)
	}
	if retried.InputRevisionID != failed.InputRevisionID || retried.PromptTemplateVersionID != failed.PromptTemplateVersionID || retried.SourceRefsJSON != failed.SourceRefsJSON {
		t.Fatalf("manual retry changed bound generation inputs: failed=%+v retried=%+v", failed, retried)
	}
	var preserved db.SolutionPolishJob
	if err := conn.First(&preserved, failed.ID).Error; err != nil {
		t.Fatalf("load preserved failure: %v", err)
	}
	if preserved.Status != JobFailed || preserved.AttemptCount != 3 || preserved.LastError != providerError {
		t.Fatalf("manual retry overwrote failure audit: %+v", preserved)
	}
	replay, replayed, err := module.RetryFailedPolish(ctx, RetryFailedPolishCommand{
		DemandID: "WA-207", JobID: failed.ID, RequestedBy: "Alice",
	})
	if err != nil || !replayed || replay.ID != retried.ID {
		t.Fatalf("duplicate manual retry = %+v replayed=%v err=%v", replay, replayed, err)
	}
	var count int64
	if err := conn.Model(&db.SolutionPolishJob{}).Where("solution_asset_id = ?", failed.SolutionAssetID).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("duplicate retry created extra jobs: count=%d err=%v", count, err)
	}
}

func TestRetryFailedPolishRejectsStaleFailureAndExistingWorkingSolution(t *testing.T) {
	t.Run("newer job", func(t *testing.T) {
		module, conn := testModule(t, 64)
		ctx := context.Background()
		failed, _, err := module.RequestInitialDraft(ctx, RequestInitialDraftCommand{
			DemandID: "WA-208", ProjectKey: "WA", Title: "失败首稿", Markdown: "# 失败首稿", RequestedBy: "jira-sync",
		})
		if err != nil {
			t.Fatalf("request initial draft: %v", err)
		}
		if err := conn.Model(&db.SolutionPolishJob{}).Where("id = ?", failed.ID).Updates(map[string]any{
			"status": JobFailed, "attempt_count": 3, "last_error": "terminal failure",
		}).Error; err != nil {
			t.Fatalf("mark failed: %v", err)
		}
		newer := failed
		newer.ID = 0
		newer.IdempotencyKey = "newer-generation-job"
		newer.Status = JobQueued
		newer.AttemptCount = 0
		newer.LastError = ""
		if err := conn.Create(&newer).Error; err != nil {
			t.Fatalf("create newer job: %v", err)
		}
		if _, _, err := module.RetryFailedPolish(ctx, RetryFailedPolishCommand{
			DemandID: "WA-208", JobID: failed.ID, RequestedBy: "Alice",
		}); !errors.Is(err, ErrConflict) {
			t.Fatalf("retry stale failure error=%v, want conflict", err)
		}
	})

	t.Run("working solution", func(t *testing.T) {
		module, conn := testModule(t, 64)
		ctx := context.Background()
		workspace, err := module.EnsureDraft(ctx, EnsureDraftCommand{
			DemandID: "WA-209", Title: "人工方案", Markdown: "# 人工方案", Actor: "Alice",
		})
		if err != nil {
			t.Fatalf("ensure draft: %v", err)
		}
		var prompt db.SolutionPromptTemplate
		if err := conn.Where("purpose = ? AND status = ?", "solution_polish", "active").First(&prompt).Error; err != nil {
			t.Fatalf("load prompt: %v", err)
		}
		failed := db.SolutionPolishJob{
			IdempotencyKey: "working-failed-job", SolutionAssetID: workspace.Asset.ID,
			InputRevisionID: workspace.Working.ID, PromptTemplateVersionID: prompt.ID,
			SourceRefsJSON: "[]", Status: JobFailed, RequestedBy: "jira-sync",
			AttemptCount: 3, LastError: "terminal failure", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := conn.Create(&failed).Error; err != nil {
			t.Fatalf("create failed job: %v", err)
		}
		if _, _, err := module.RetryFailedPolish(ctx, RetryFailedPolishCommand{
			DemandID: "WA-209", JobID: failed.ID, RequestedBy: "Alice",
		}); !errors.Is(err, ErrConflict) {
			t.Fatalf("retry with working solution error=%v, want conflict", err)
		}
	})
}

func TestMigrateLegacyJiraPlaceholderAndCandidateWithoutDeletingHistory(t *testing.T) {
	module, conn := testModule(t, 64)
	ctx := context.Background()
	legacy, err := module.EnsureDraft(ctx, EnsureDraftCommand{
		DemandID: "WA-206", Title: "旧方案", Markdown: "# 旧占位", Actor: "jira-sync",
	})
	if err != nil {
		t.Fatalf("seed legacy placeholder: %v", err)
	}
	job, _, err := module.RequestPolish(ctx, RequestPolishCommand{DemandID: "WA-206", ProjectKey: "WA", RequestedBy: "jira-sync"})
	if err != nil {
		t.Fatalf("request legacy polish: %v", err)
	}
	if _, err := module.ClaimNextJob(ctx); err != nil {
		t.Fatalf("claim legacy polish: %v", err)
	}
	candidate, err := module.CompletePolish(ctx, CompletePolishCommand{JobID: job.ID, Markdown: "# 旧 Agent 正文", ModelVersion: "legacy-model"})
	if err != nil {
		t.Fatalf("complete legacy polish: %v", err)
	}
	if err := module.MigrateLegacyInitialDrafts(ctx); err != nil {
		t.Fatalf("migrate legacy lifecycle: %v", err)
	}
	workspace, err := module.GetWorkspace(ctx, "WA-206")
	if err != nil || workspace.Working == nil || workspace.Working.ID != candidate.ID || workspace.Working.Kind != KindAgentDraft || workspace.Working.Status != StatusDraft || len(workspace.Candidates) != 0 || len(workspace.History) != 1 {
		t.Fatalf("legacy candidate was not promoted: workspace=%+v err=%v", workspace, err)
	}
	var seed db.SolutionRevision
	if err := conn.First(&seed, legacy.Working.ID).Error; err != nil {
		t.Fatalf("load migrated seed: %v", err)
	}
	if seed.Kind != KindSystemSeed || seed.Status != StatusSeed {
		t.Fatalf("legacy placeholder was not hidden: %+v", seed)
	}
	var revisionCount int64
	if err := conn.Model(&db.SolutionRevision{}).Where("solution_asset_id = ?", workspace.Asset.ID).Count(&revisionCount).Error; err != nil || revisionCount != 2 {
		t.Fatalf("migration deleted history: count=%d err=%v", revisionCount, err)
	}
}

func TestObserveSourceReplayRefreshesDerivedClassification(t *testing.T) {
	module, conn := testModule(t, 64)
	ctx := context.Background()
	command := ObserveSourceCommand{
		DemandID: "DG-394", SourceSystem: "jira", ExternalID: "457134",
		Author: "jira公用-解决方案", Body: "FMS 更新充电状态。",
	}
	first, err := module.ObserveSource(ctx, command)
	if err != nil || first.Replayed || first.Source.Eligible {
		t.Fatalf("initial observation = %+v, err=%v", first, err)
	}

	command.Marker = "author:jira公用-解决方案"
	command.Eligible = true
	replayed, err := module.ObserveSource(ctx, command)
	if err != nil || !replayed.Replayed || replayed.Source.ID != first.Source.ID {
		t.Fatalf("reclassified replay = %+v, err=%v", replayed, err)
	}
	if !replayed.Source.Eligible || replayed.Source.Marker != command.Marker {
		t.Fatalf("replayed source did not refresh derived classification: %+v", replayed.Source)
	}

	var persisted db.SolutionSourceRef
	if err := conn.First(&persisted, first.Source.ID).Error; err != nil {
		t.Fatalf("query persisted source: %v", err)
	}
	if !persisted.Eligible || persisted.Marker != command.Marker {
		t.Fatalf("persisted classification = %+v", persisted)
	}
}

func TestProjectPromptOverridesGlobalAndVersionsAreAppendOnly(t *testing.T) {
	module, conn := testModule(t, 0)
	ctx := context.Background()
	projectPrompt, err := module.SavePrompt(ctx, SavePromptCommand{
		ScopeType: "project", ScopeID: "WA", Name: "WA 方案模板", SystemPrompt: "只输出 WA Markdown", Actor: "Root", Activate: true,
	})
	if err != nil {
		t.Fatalf("save project prompt: %v", err)
	}
	if projectPrompt.Version != 1 || projectPrompt.Status != "active" {
		t.Fatalf("unexpected project prompt: %+v", projectPrompt)
	}
	_, err = module.SavePrompt(ctx, SavePromptCommand{
		ScopeType: "project", ScopeID: "WA", Name: "WA 方案模板 2", SystemPrompt: "只输出新版 WA Markdown", Actor: "Root", Activate: true,
	})
	if err != nil {
		t.Fatalf("save second project prompt: %v", err)
	}
	var prompts []db.SolutionPromptTemplate
	if err := conn.Where("scope_type = ? AND scope_id = ?", "project", "WA").Order("version asc").Find(&prompts).Error; err != nil {
		t.Fatalf("list project prompts: %v", err)
	}
	if len(prompts) != 2 || prompts[0].Status != "retired" || prompts[1].Status != "active" || prompts[0].SystemPrompt == prompts[1].SystemPrompt {
		t.Fatalf("prompt versions were overwritten: %+v", prompts)
	}

	workspace, err := module.EnsureDraft(ctx, EnsureDraftCommand{DemandID: "WA-303", Markdown: "# 方案", Actor: "A"})
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	job, _, err := module.RequestPolish(ctx, RequestPolishCommand{DemandID: "WA-303", ProjectKey: "WA", RequestedBy: "A"})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if job.InputRevisionID != workspace.Working.ID || job.PromptTemplateVersionID != prompts[1].ID {
		t.Fatalf("project active prompt not bound to job: %+v", job)
	}
}

func TestComparisonPromptsAreVersionedByPurpose(t *testing.T) {
	module, _ := testModule(t, 0)
	ctx := context.Background()
	created, err := module.SavePrompt(ctx, SavePromptCommand{
		Purpose: "solution_compare_requirement", ScopeType: "global", Name: "需求等价性 v1",
		SystemPrompt: "只输出需求等价性 JSON", Actor: "Root", Activate: true,
	})
	if err != nil {
		t.Fatalf("save comparison prompt: %v", err)
	}
	if created.Purpose != "solution_compare_requirement" || created.Version != 1 || created.Status != "active" {
		t.Fatalf("comparison prompt = %+v", created)
	}
	active, err := module.ActivePrompt(ctx, "solution_compare_requirement", "")
	if err != nil || active.ID != created.ID {
		t.Fatalf("active comparison prompt = %+v err=%v", active, err)
	}
	polish, err := module.ActivePrompt(ctx, "solution_polish", "")
	if err != nil || polish.ID == created.ID || polish.Purpose != "solution_polish" {
		t.Fatalf("polish prompt boundary = %+v err=%v", polish, err)
	}
	all, err := module.ListPrompts(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("prompt list = %+v err=%v", all, err)
	}
}

func TestCodeReviewSkillPurposeIsVersionedAndActivatable(t *testing.T) {
	module, _ := testModule(t, 0)
	ctx := context.Background()
	created, err := module.SavePrompt(ctx, SavePromptCommand{
		Purpose: "code_review", ScopeType: "global", Name: "代码评审技能 v1",
		SystemPrompt: "只报告有证据的问题", Actor: "Root", Activate: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Purpose != "code_review" || created.Status != "draft" || created.Version != 1 {
		t.Fatalf("created skill = %+v", created)
	}
	if _, err = module.ActivatePrompt(ctx, created.ID, "Root"); !errors.Is(err, ErrConflict) {
		t.Fatalf("untested skill activation err=%v", err)
	}
	if _, err = module.RecordPromptValidation(ctx, created.ID, "Root", "fixture passed", true); err != nil {
		t.Fatal(err)
	}
	if _, err = module.ActivatePrompt(ctx, created.ID, "Root"); err != nil {
		t.Fatal(err)
	}
	draft, err := module.SavePrompt(ctx, SavePromptCommand{
		Purpose: "code_review", ScopeType: "global", Name: "代码评审技能 v2",
		SystemPrompt: "增加反证复核", Actor: "Root", Activate: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = module.RecordPromptValidation(ctx, draft.ID, "Root", "fixture passed", true); err != nil {
		t.Fatal(err)
	}
	if _, err = module.ActivatePrompt(ctx, draft.ID, "Root"); err != nil {
		t.Fatal(err)
	}
	active, err := module.ActivePrompt(ctx, "code_review", "")
	if err != nil || active.ID != draft.ID || active.Version != 2 {
		t.Fatalf("active code review skill = %+v err=%v", active, err)
	}
	all, err := module.ListPrompts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, prompt := range all {
		if prompt.Purpose == "code_review" {
			found++
		}
	}
	if found != 2 {
		t.Fatalf("code review skill versions=%d, prompts=%+v", found, all)
	}
}

func TestProjectPromptMissFallsBackWithoutRecordNotFoundLog(t *testing.T) {
	var output bytes.Buffer
	conn, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.New(log.New(&output, "", 0), logger.Config{
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: false,
		}),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := conn.AutoMigrate(
		&db.SolutionAsset{}, &db.SolutionRevision{}, &db.SolutionSourceRef{},
		&db.SolutionPolishJob{}, &db.SolutionPromptTemplate{}, &db.SolutionJiraOutbox{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	globalPrompt := db.SolutionPromptTemplate{
		Purpose: "solution_polish", ScopeType: "global", ScopeID: "", Version: 1,
		Status: "active", Name: "default", SystemPrompt: db.DefaultSolutionPolishPrompt,
		CreatedBy: "system", CreatedAt: now,
	}
	if err := conn.Create(&globalPrompt).Error; err != nil {
		t.Fatalf("seed global prompt: %v", err)
	}
	module := New(conn, WithClock(func() time.Time { return now }))
	workspace, err := module.EnsureDraft(context.Background(), EnsureDraftCommand{
		DemandID: "HIT-1", Markdown: "# 方案", Actor: "jira-sync",
	})
	if err != nil {
		t.Fatalf("ensure draft: %v", err)
	}
	output.Reset()

	job, replayed, err := module.RequestPolish(context.Background(), RequestPolishCommand{
		DemandID: "HIT-1", ProjectKey: "HIT", RequestedBy: "jira-sync",
	})
	if err != nil || replayed || job.InputRevisionID != workspace.Working.ID || job.PromptTemplateVersionID != globalPrompt.ID {
		t.Fatalf("global prompt fallback job = %+v replayed=%v err=%v", job, replayed, err)
	}
	if logText := output.String(); strings.Contains(logText, "record not found") || strings.Contains(logText, "scope_id = \"HIT\"") {
		t.Fatalf("normal project prompt fallback emitted an error log:\n%s", logText)
	}
}

func TestWorkerLeasesRecoverStaleRunningRecords(t *testing.T) {
	module, conn := testModule(t, 0)
	ctx := context.Background()
	workspace, err := module.EnsureDraft(ctx, EnsureDraftCommand{DemandID: "WA-404", Markdown: "# 方案", Actor: "A"})
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	job, _, err := module.RequestPolish(ctx, RequestPolishCommand{DemandID: "WA-404", RequestedBy: "A"})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	stale := module.now().Add(-11 * time.Minute)
	if err := conn.Model(&db.SolutionPolishJob{}).Where("id = ?", job.ID).Updates(map[string]any{"status": JobRunning, "updated_at": stale}).Error; err != nil {
		t.Fatalf("stale job: %v", err)
	}
	claimed, err := module.ClaimNextJob(ctx)
	if err != nil || claimed == nil || claimed.Job.ID != job.ID || claimed.Job.AttemptCount != 1 {
		t.Fatalf("recovered job = %+v err=%v", claimed, err)
	}
	outbox := db.SolutionJiraOutbox{
		IdempotencyKey: "lease-outbox", SolutionAssetID: workspace.Asset.ID,
		SolutionRevisionID: workspace.Working.ID, DemandID: "WA-404", Operation: "publish_solution_link",
		PayloadJSON: `{}`, Status: "processing", AttemptCount: 1, CreatedAt: stale, UpdatedAt: stale,
	}
	if err := conn.Create(&outbox).Error; err != nil {
		t.Fatalf("seed outbox: %v", err)
	}
	claimedOutbox, err := module.ClaimNextOutbox(ctx)
	if err != nil || claimedOutbox == nil || claimedOutbox.ID != outbox.ID || claimedOutbox.AttemptCount != 2 {
		t.Fatalf("recovered outbox = %+v err=%v", claimedOutbox, err)
	}
}

func TestWorkerClaimsReturnNilWhenQueuesAreEmpty(t *testing.T) {
	module, _ := testModule(t, 0)
	ctx := context.Background()

	job, err := module.ClaimNextJob(ctx)
	if err != nil || job != nil {
		t.Fatalf("empty job queue = %+v err=%v", job, err)
	}
	outbox, err := module.ClaimNextOutbox(ctx)
	if err != nil || outbox != nil {
		t.Fatalf("empty outbox queue = %+v err=%v", outbox, err)
	}
}

func TestReconcileSourceSetMissingAssetIsSilent(t *testing.T) {
	var output bytes.Buffer
	conn, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.New(log.New(&output, "", 0), logger.Config{
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: false,
		}),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := conn.AutoMigrate(&db.SolutionAsset{}, &db.SolutionSourceRef{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	output.Reset()

	if err := New(conn).ReconcileSourceSet(context.Background(), "DL-4309", "jira", nil); err != nil {
		t.Fatalf("reconcile missing asset: %v", err)
	}
	if logText := output.String(); strings.Contains(logText, "record not found") || strings.Contains(logText, "solution_assets") {
		t.Fatalf("normal missing solution asset emitted an error log:\n%s", logText)
	}
}
