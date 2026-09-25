package codereview

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

const head = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const base = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

type fixture struct {
	s                           *Service
	cfg                         *config.Config
	posts, calls, mrCalls       int
	comments                    []map[string]any
	unknown, changed, truncated bool
	failSecondMR                bool
	postStatus                  int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := conn.DB()
	sql.SetMaxOpenConns(1)
	t.Cleanup(func() { sql.Close() })
	if err = conn.AutoMigrate(&db.CodeReviewPolicy{}, &db.CodeReviewRun{}, &db.CodeReviewPublication{}, &db.ContextFact{}, &db.SolutionPromptTemplate{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err = conn.Create(&db.SolutionPromptTemplate{
		Purpose: "code_review", ScopeType: "global", ScopeID: "", Version: 1,
		Status: "active", Name: "fixture review skill", SystemPrompt: db.DefaultCodeReviewSkillPrompt,
		ContentHash: digest(db.DefaultCodeReviewSkillPrompt), ValidationStatus: "passed",
		CreatedBy: "system", ActivatedBy: "system", ActivatedAt: &now, CreatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	f := &fixture{cfg: &config.Config{}}
	f.cfg.GitLab = config.GitLabConfig{Enabled: true, BaseURL: "https://gitlab.example.test", APIToken: "fixture", Repos: []config.RepoMapping{{ProjectID: "10", Name: "dispatch", Path: "fms/dispatch"}}}
	f.cfg.AI.Enabled = true
	f.s = &Service{DB: conn, Config: func() *config.Config { return f.cfg }, Generate: func(ctx context.Context, system, user string) (string, error) {
		f.calls++
		if !strings.Contains(system, "robustness") || !strings.Contains(user, "return nil") {
			t.Error("missing source or engineering dimensions")
		}
		return encode(exampleReport()), nil
	}}
	f.s.HTTP = &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("PRIVATE-TOKEN") != "fixture" {
			t.Fatal("missing GitLab auth")
		}
		var body any
		headers := make(http.Header)
		switch {
		case r.Method == "POST":
			f.posts++
			if f.postStatus != 0 {
				return &http.Response{
					StatusCode: f.postStatus,
					Header:     headers,
					Body:       io.NopCloser(strings.NewReader(`{"message":"rejected"}`)),
				}, nil
			}
			var p map[string]string
			_ = json.NewDecoder(r.Body).Decode(&p)
			note := p["body"]
			if note == "" {
				note = p["note"]
			}
			f.comments = append(f.comments, map[string]any{"id": 123, "body": note, "note": note})
			if f.unknown {
				return nil, errors.New("response lost")
			}
			body = map[string]any{"id": 123}
		case strings.HasSuffix(r.URL.Path, "/notes") || strings.HasSuffix(r.URL.Path, "/comments"):
			body = f.comments
			if body == nil {
				body = []any{}
			}
		case strings.Contains(r.URL.Path, "/repository/files/"):
			body = map[string]string{"encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte("package dispatch\nfunc Run() error {\n return nil\n}\n"))}
		case strings.HasSuffix(r.URL.Path, "/diffs") || strings.HasSuffix(r.URL.Path, "/diff"):
			body = []File{{OldPath: "dispatch.go", NewPath: "dispatch.go", Diff: "@@ -1,4 +1,4 @@\n package dispatch\n func Run() error {\n- return err\n+ return nil\n }", TooLarge: f.truncated}}
		case strings.Contains(r.URL.Path, "/merge_requests/"):
			f.mrCalls++
			if f.failSecondMR && f.mrCalls >= 2 {
				return nil, errors.New("second MR read failed")
			}
			h := head
			if f.changed {
				h = base
			}
			body = map[string]any{"sha": h, "state": "opened", "title": "Adjust dispatch failure handling", "description": "Keep task state consistent", "web_url": "https://gitlab.example.test/fms/dispatch/-/merge_requests/7", "diff_refs": map[string]string{"head_sha": h, "base_sha": base}}
		case strings.Contains(r.URL.Path, "/repository/commits/"):
			body = map[string]any{"id": head, "parent_ids": []string{base}, "title": "Adjust dispatch", "message": "Handle error"}
		default:
			t.Fatalf("unexpected request %s", r.URL)
		}
		b, _ := json.Marshal(body)
		return &http.Response{StatusCode: 200, Header: headers, Body: io.NopCloser(strings.NewReader(string(b)))}, nil
	})}
	return f
}
func exampleReport() Report {
	r := Report{Summary: "发现错误处理风险，建议补充失败路径测试。", Scenario: "调度", Findings: []Finding{{Dimension: "robustness", Severity: "medium", Title: "错误被忽略", File: "dispatch.go", Line: 3, Evidence: "return nil", Impact: "上层无法识别执行失败", Suggestion: "返回原始错误", Verification: "增加失败分支测试"}}}
	for _, d := range Dimensions {
		r.Assessments = append(r.Assessments, Assessment{Dimension: d, Analysis: "已检查当前变更范围；外部调用需要进一步验证。"})
	}
	return r
}
func (f *fixture) policy(t *testing.T, p db.CodeReviewPolicy) {
	t.Helper()
	p.ProjectID = "10"
	if p.Domain == "" {
		p.Domain = "general"
	}
	if p.Scenario == "" {
		p.Scenario = "general"
	}
	if err := f.s.DB.Save(&p).Error; err != nil {
		t.Fatal(err)
	}
}
func (f *fixture) review(t *testing.T, kind string) db.CodeReviewRun {
	t.Helper()
	ref := "7"
	if kind == "commit" {
		ref = head
	}
	run, err := f.s.Enqueue(context.Background(), "10", kind, ref, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	run, err = f.s.Get(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	return run
}
func TestTwoPassReviewAndKnowledgeScope(t *testing.T) {
	f := newFixture(t)
	f.policy(t, db.CodeReviewPolicy{Domain: "fms", Scenario: "dispatch", KnowledgeScope: "FMS"})
	for _, fact := range []db.ContextFact{{Scope: "project", ScopeID: "FMS", Status: "active", Summary: "完成证据", Content: "任务完成必须有明确反馈", Version: 2}, {Scope: "repo", ScopeID: "10", Status: "active", Content: "错误不等于成功"}, {Scope: "project", ScopeID: "SECRET", Status: "active", Content: "unrelated"}, {Scope: "global", Status: "archived", Content: "obsolete"}} {
		f.s.DB.Create(&fact)
	}
	run := f.review(t, "mr")
	if run.Status != "completed" || f.calls != 2 || f.posts != 0 {
		t.Fatalf("unexpected run %#v calls=%d posts=%d", run, f.calls, f.posts)
	}
	var s Snapshot
	_ = json.Unmarshal([]byte(run.SnapshotJSON), &s)
	if len(s.Knowledge) != 2 || strings.Contains(run.SnapshotJSON, "unrelated") || s.Head != head {
		t.Fatal("knowledge scope or version not frozen")
	}
}
func TestReviewRunFreezesActiveOnlineSkillVersion(t *testing.T) {
	f := newFixture(t)
	var first db.SolutionPromptTemplate
	if err := f.s.DB.Where("purpose = ? AND status = ?", "code_review", "active").First(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.s.DB.Model(&first).Update("system_prompt", "ONLINE_SKILL_V1 evidence-first").Error; err != nil {
		t.Fatal(err)
	}
	run, err := f.s.Enqueue(context.Background(), "10", "mr", "7", "skill-freeze")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err = f.s.DB.Model(&first).Update("status", "retired").Error; err != nil {
		t.Fatal(err)
	}
	second := db.SolutionPromptTemplate{
		Purpose: "code_review", ScopeType: "global", ScopeID: "", Version: 2,
		Status: "active", Name: "review skill v2", SystemPrompt: "ONLINE_SKILL_V2",
		CreatedBy: "root", ActivatedBy: "root", ActivatedAt: &now, CreatedAt: now,
	}
	if err = f.s.DB.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	f.s.Generate = func(_ context.Context, system, _ string) (string, error) {
		if !strings.Contains(system, "ONLINE_SKILL_V1") || strings.Contains(system, "ONLINE_SKILL_V2") {
			t.Fatalf("worker did not use frozen skill: %s", system)
		}
		return encode(exampleReport()), nil
	}
	if _, err = f.s.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	run, err = f.s.Get(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run.SkillVersionID != first.ID || run.SkillVersion != 1 || run.SkillName != first.Name ||
		!strings.Contains(run.PromptVersion, fmt.Sprintf("review-skill:%d/v1", first.ID)) {
		t.Fatalf("frozen skill binding = %+v", run)
	}
}
func TestEnqueueFailsWithoutActiveValidatedReviewSkill(t *testing.T) {
	f := newFixture(t)
	if err := f.s.DB.Model(&db.SolutionPromptTemplate{}).
		Where("purpose = ?", "code_review").
		Updates(map[string]any{"status": "draft", "validation_status": "untested"}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.Enqueue(context.Background(), "10", "mr", "7", "missing-skill"); err == nil ||
		!strings.Contains(err.Error(), "未配置或未启用") {
		t.Fatalf("enqueue err=%v", err)
	}
}
func TestFMSEvidenceGapBlocksAutoComments(t *testing.T) {
	f := newFixture(t)
	f.policy(t, db.CodeReviewPolicy{Domain: "fms", Scenario: "yard", SyncMRs: true})
	run := f.review(t, "mr")
	if run.Status != "partial" || run.PublishStatus != "blocked" {
		t.Fatalf("unexpected state %s/%s", run.Status, run.PublishStatus)
	}
	if err := f.s.Publish(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	if f.posts != 0 {
		t.Fatal("published incomplete FMS review")
	}
}
func TestSwitchesAndStaleVersion(t *testing.T) {
	for _, kind := range []string{"commit", "mr"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			f.policy(t, db.CodeReviewPolicy{SyncCommits: true, SyncMRs: true})
			run := f.review(t, kind)
			f.policy(t, db.CodeReviewPolicy{})
			if err := f.s.Publish(context.Background(), run); err != nil {
				t.Fatal(err)
			}
			if f.posts != 0 {
				t.Fatal("switch off ignored")
			}
			f.policy(t, db.CodeReviewPolicy{SyncCommits: true, SyncMRs: true})
			if kind == "mr" {
				f.changed = true
			}
			if err := f.s.Publish(context.Background(), run); err != nil {
				t.Fatal(err)
			}
			want := 1
			if kind == "mr" {
				want = 0
			}
			if f.posts != want {
				t.Fatal("stale version or commit publish behavior wrong")
			}
		})
	}
}
func TestPublishOnceAcrossRerunsAndUnknownOutcome(t *testing.T) {
	f := newFixture(t)
	f.policy(t, db.CodeReviewPolicy{SyncMRs: true})
	run := f.review(t, "mr")
	f.unknown = true
	if err := f.s.Publish(context.Background(), run); err == nil {
		t.Fatal("expected unknown outcome")
	}
	if f.posts != 1 {
		t.Fatal("missing request")
	}
	if err := f.s.Publish(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	run, _ = f.s.Get(run.ID)
	if run.PublishStatus != "published" || f.posts != 1 {
		t.Fatal("did not reconcile or duplicated")
	}
	next := f.review(t, "mr")
	if err := f.s.Publish(context.Background(), next); err != nil {
		t.Fatal(err)
	}
	if f.posts != 1 {
		t.Fatal("rerun duplicated comment")
	}
}
func TestUnknownWithoutVisibleCommentDoesNotRetry(t *testing.T) {
	f := newFixture(t)
	f.policy(t, db.CodeReviewPolicy{SyncMRs: true})
	run := f.review(t, "mr")
	f.unknown = true
	_ = f.s.Publish(context.Background(), run)
	f.comments = nil
	_ = f.s.Publish(context.Background(), run)
	if f.posts != 1 {
		t.Fatal("unsafe POST retry")
	}
}
func TestPrePostFailureReleasesPublicationClaim(t *testing.T) {
	f := newFixture(t)
	f.policy(t, db.CodeReviewPolicy{SyncMRs: true})
	run := f.review(t, "mr")
	f.mrCalls = 0
	f.failSecondMR = true
	if err := f.s.Publish(context.Background(), run); err == nil {
		t.Fatal("expected second MR read failure")
	}
	if f.posts != 0 {
		t.Fatal("pre-POST failure sent a comment")
	}
	var claimCount int64
	if err := f.s.DB.Model(&db.CodeReviewPublication{}).Count(&claimCount).Error; err != nil {
		t.Fatal(err)
	}
	run, _ = f.s.Get(run.ID)
	if claimCount != 0 || run.PublishStatus != "sync_failed" {
		t.Fatalf("claim/status after pre-POST failure = %d/%s", claimCount, run.PublishStatus)
	}
	f.failSecondMR = false
	if err := f.s.Publish(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	run, _ = f.s.Get(run.ID)
	if f.posts != 1 || run.PublishStatus != "published" {
		t.Fatalf("recovery posts/status = %d/%s", f.posts, run.PublishStatus)
	}
}
func TestKnownHTTPRejectionRemainsRetryable(t *testing.T) {
	f := newFixture(t)
	f.policy(t, db.CodeReviewPolicy{SyncMRs: true})
	run := f.review(t, "mr")
	f.postStatus = http.StatusForbidden
	if err := f.s.Publish(context.Background(), run); err == nil {
		t.Fatal("expected GitLab rejection")
	}
	var claimCount int64
	if err := f.s.DB.Model(&db.CodeReviewPublication{}).Count(&claimCount).Error; err != nil {
		t.Fatal(err)
	}
	run, _ = f.s.Get(run.ID)
	if claimCount != 0 || run.PublishStatus != "sync_failed" {
		t.Fatalf("known rejection claim/status = %d/%s", claimCount, run.PublishStatus)
	}
	f.postStatus = 0
	if err := f.s.Publish(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	run, _ = f.s.Get(run.ID)
	if f.posts != 2 || run.PublishStatus != "published" {
		t.Fatalf("known rejection recovery = posts %d status %s", f.posts, run.PublishStatus)
	}
}
func TestPublishedClaimCannotRegressRunToUnknown(t *testing.T) {
	f := newFixture(t)
	f.policy(t, db.CodeReviewPolicy{SyncMRs: true})
	run := f.review(t, "mr")
	key := publicationKey(run)
	if err := f.s.DB.Create(&db.CodeReviewPublication{
		Key:       key,
		RunID:     run.ID,
		Status:    "published",
		CommentID: 321,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.s.Publish(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	run, _ = f.s.Get(run.ID)
	if f.posts != 0 || run.PublishStatus != "published" || run.CommentID != 321 {
		t.Fatalf("published claim reconciliation = posts %d run %+v", f.posts, run)
	}
}
func TestStalePublicationClaimIsReclaimedAndQueueProgresses(t *testing.T) {
	f := newFixture(t)
	f.policy(t, db.CodeReviewPolicy{SyncMRs: true, SyncCommits: true})
	first := f.review(t, "mr")
	second := f.review(t, "commit")
	stale := db.CodeReviewPublication{
		Key:       publicationKey(first),
		RunID:     first.ID,
		Status:    "publishing",
		UpdatedAt: time.Now().Add(-5 * time.Minute),
	}
	if err := f.s.DB.Create(&stale).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.s.DB.Model(&stale).Update("updated_at", time.Now().Add(-5*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.s.PublishNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := f.s.PublishNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	first, _ = f.s.Get(first.ID)
	second, _ = f.s.Get(second.ID)
	if f.posts != 2 || first.PublishStatus != "published" || second.PublishStatus != "published" {
		t.Fatalf("queue recovery = posts %d first %s second %s", f.posts, first.PublishStatus, second.PublishStatus)
	}
}
func TestSendingPublicationIsNotReclaimedOrDuplicated(t *testing.T) {
	f := newFixture(t)
	f.policy(t, db.CodeReviewPolicy{SyncMRs: true})
	run := f.review(t, "mr")
	sending := db.CodeReviewPublication{
		Key:        publicationKey(run),
		RunID:      run.ID,
		Status:     "sending",
		LeaseToken: "owner-a",
		UpdatedAt:  time.Now().Add(-10 * time.Minute),
	}
	if err := f.s.DB.Create(&sending).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.s.Publish(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	var ledger db.CodeReviewPublication
	if err := f.s.DB.First(&ledger, "key = ?", sending.Key).Error; err != nil {
		t.Fatal(err)
	}
	run, _ = f.s.Get(run.ID)
	if f.posts != 0 || ledger.Status != "unknown" || run.PublishStatus != "unknown" {
		t.Fatalf("sending claim was reclaimed: posts=%d ledger=%+v run=%+v", f.posts, ledger, run)
	}
}
func TestValidationRejectsFabricatedEvidence(t *testing.T) {
	r := exampleReport()
	r.Findings[0].Line = 99
	s := Snapshot{Files: []File{{NewPath: "dispatch.go", Source: "package dispatch\nfunc Run() error {\n return nil\n}"}}}
	result, err := parseReport(encode(r), s)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 0 || len(result.Questions) == 0 {
		t.Fatal("fabricated finding accepted")
	}
	r.Assessments = r.Assessments[:2]
	if _, err = parseReport(encode(r), s); err == nil {
		t.Fatal("incomplete dimensions accepted")
	}
}
func TestHookAutomaticReviewAndDeduplication(t *testing.T) {
	f := newFixture(t)
	body := []byte(`{"project":{"id":10},"commits":[{"id":"` + head + `"}]}`)
	// No policy row is needed; comments remain off.
	if err := f.s.Hook(context.Background(), "Push Hook", body); err != nil {
		t.Fatal(err)
	}
	var runs []db.CodeReviewRun
	f.s.DB.Find(&runs)
	if len(runs) != 1 || runs[0].PublishStatus != "off" {
		t.Fatalf("automatic review without publication: %+v", runs)
	}
	// Persisted legacy opt-out must not block new events.
	f.policy(t, db.CodeReviewPolicy{AutoReview: false})
	mr := []byte(`{"project":{"id":10},"object_attributes":{"iid":7,"state":"opened","updated_at":"2026-09-23","last_commit":{"id":"` + head + `"}}}`)
	for i := 0; i < 2; i++ {
		if err := f.s.Hook(context.Background(), "Push Hook", body); err != nil {
			t.Fatal(err)
		}
		if err := f.s.Hook(context.Background(), "Merge Request Hook", mr); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	f.s.DB.Model(&db.CodeReviewRun{}).Count(&count)
	if count != 2 {
		t.Fatalf("expected one commit and one MR, got %d", count)
	}
	if err := f.s.Hook(context.Background(), "Push Hook", []byte(`{"project":{"id":20},"commits":[{"id":"`+head+`"}]}`)); err != nil {
		t.Fatal(err)
	}
	f.s.DB.Model(&db.CodeReviewRun{}).Count(&count)
	if count != 2 {
		t.Fatal("unconfigured repository event queued")
	}
}

func TestTruncatedSnapshotNotPublished(t *testing.T) {
	f := newFixture(t)
	f.truncated = true
	f.policy(t, db.CodeReviewPolicy{SyncMRs: true})
	run := f.review(t, "mr")
	if run.Status != "partial" {
		t.Fatal("truncated diff not marked partial")
	}
	_ = f.s.Publish(context.Background(), run)
	if f.posts != 0 {
		t.Fatal("partial review published")
	}
}
func TestCancelledAndInterruptedRuns(t *testing.T) {
	f := newFixture(t)
	run, err := f.s.Enqueue(context.Background(), "10", "mr", "7", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.Cancel(run.ID); err != nil {
		t.Fatal(err)
	}
	processed, _ := f.s.ProcessOne(context.Background())
	if processed {
		t.Fatal("cancelled run executed")
	}
	f.s.DB.Model(&run).Updates(map[string]any{"status": "running", "updated_at": time.Now().Add(-time.Hour)})
	_, _ = f.s.ProcessOne(context.Background())
	run, _ = f.s.Get(run.ID)
	if run.Status != "failed" {
		t.Fatal("interrupted run not recovered")
	}
}

func TestFailedRunManualRetryIsIdempotent(t *testing.T) {
	f := newFixture(t)
	f.policy(t, db.CodeReviewPolicy{SyncMRs: true})
	failed, err := f.s.Enqueue(context.Background(), "10", "mr", "7", "failed-source")
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.DB.Model(&failed).Updates(map[string]any{
		"status": "failed",
		"phase":  "评审失败",
		"error":  "fixture failure",
	}).Error; err != nil {
		t.Fatal(err)
	}
	retry, err := f.s.Retry(context.Background(), failed.ID)
	if err != nil {
		t.Fatal(err)
	}
	// 就地重试：保留原 ID，不增加新记录
	if retry.ID != failed.ID || retry.Status != "queued" || retry.PublishStatus != "waiting_review" || retry.Error != "" {
		t.Fatalf("unexpected in-place retry: %+v", retry)
	}
	var count int64
	if err = f.s.DB.Model(&db.CodeReviewRun{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("run count=%d, want 1 (in-place retry should not add new records)", count)
	}
	// 处于 queued 状态不可重复重试
	if _, err = f.s.Retry(context.Background(), retry.ID); err == nil {
		t.Fatal("queued retry accepted")
	}

	// 模拟执行后再次失败，再次重试依然保持只有 1 条记录
	if err = f.s.DB.Model(&failed).Updates(map[string]any{
		"status": "failed",
		"phase":  "评审失败",
		"error":  "second failure",
	}).Error; err != nil {
		t.Fatal(err)
	}
	secondRetry, err := f.s.Retry(context.Background(), failed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if secondRetry.ID != failed.ID || secondRetry.Status != "queued" {
		t.Fatalf("unexpected second retry: %+v", secondRetry)
	}
	if err = f.s.DB.Model(&db.CodeReviewRun{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("run count=%d, multiple retries should strictly never increase records", count)
	}
}

func TestPushHookDoesNotSplitOpenedMRCommits(t *testing.T) {
	f := newFixture(t)
	// 模拟 transport 返回该分支对应的 opened MR
	f.s.HTTP = &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		headers := make(http.Header)
		if strings.Contains(r.URL.Path, "/merge_requests") && strings.Contains(r.URL.RawQuery, "state=opened") {
			body := `[{"iid":42,"state":"opened","updated_at":"2026-09-24T20:00:00Z","diff_refs":{"base_sha":"` + base + `","head_sha":"` + head + `"}}]`
			return &http.Response{
				StatusCode: 200,
				Header:     headers,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}, nil
	})}

	// 推送包含 3 个 commit 的 Push Hook
	body := []byte(`{
		"project": {"id": 10},
		"ref": "refs/heads/feature-vehicle-dispatch",
		"after": "` + head + `",
		"commits": [
			{"id": "1111111111111111111111111111111111111111"},
			{"id": "2222222222222222222222222222222222222222"},
			{"id": "3333333333333333333333333333333333333333"}
		]
	}`)

	if err := f.s.Hook(context.Background(), "Push Hook", body); err != nil {
		t.Fatal(err)
	}

	var runs []db.CodeReviewRun
	f.s.DB.Find(&runs)

	// 验证：不会被拆分成 3 个 commit 评审，而是仅有 1 个针对 MR 42 的评审
	if len(runs) != 1 {
		t.Fatalf("expected exactly 1 MR run, got %d runs: %+v", len(runs), runs)
	}
	if runs[0].Kind != "mr" || runs[0].Ref != "42" {
		t.Fatalf("expected MR run for iid 42, got kind=%s ref=%s", runs[0].Kind, runs[0].Ref)
	}
}

func TestInvalidEvidenceBlocksPublication(t *testing.T) {
	f := newFixture(t)
	f.policy(t, db.CodeReviewPolicy{SyncMRs: true})
	f.s.Generate = func(context.Context, string, string) (string, error) {
		r := exampleReport()
		r.Findings[0].Line = 400
		return encode(r), nil
	}
	run := f.review(t, "mr")
	if run.Status != "partial" || run.PublishStatus != "blocked" {
		t.Fatalf("invalid evidence not blocked: %s/%s", run.Status, run.PublishStatus)
	}
}
func TestRunningCancellationDoesNotProduceReport(t *testing.T) {
	f := newFixture(t)
	run, err := f.s.Enqueue(context.Background(), "10", "mr", "7", "")
	if err != nil {
		t.Fatal(err)
	}
	f.s.Generate = func(ctx context.Context, _, _ string) (string, error) {
		if err := f.s.Cancel(run.ID); err != nil {
			t.Fatal(err)
		}
		<-ctx.Done()
		return "", ctx.Err()
	}
	if _, err = f.s.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	run, _ = f.s.Get(run.ID)
	if run.Status != "cancelled" || run.ReportJSON != "" || f.posts != 0 {
		t.Fatal("cancelled review produced effects")
	}
}
func TestCommentReadFailureDoesNotStarveQueue(t *testing.T) {
	f := newFixture(t)
	f.policy(t, db.CodeReviewPolicy{SyncMRs: true})
	run := f.review(t, "mr")
	f.s.HTTP = &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })}
	if err := f.s.PublishNext(context.Background()); err == nil {
		t.Fatal("expected read error")
	}
	run, _ = f.s.Get(run.ID)
	if run.PublishStatus != "sync_failed" || run.PublishError == "" {
		t.Fatal("missing visible synchronization error")
	}
}
func TestDiffAnchorRejectsUnrelatedFileLine(t *testing.T) {
	r := exampleReport()
	s := Snapshot{Files: []File{{NewPath: "dispatch.go", Source: "package dispatch\nfunc Run() error {\n return nil\n}", Diff: "@@ -20 +20 @@\n-old\n+new"}}}
	out, err := parseReport(encode(r), s)
	if err != nil {
		t.Fatal(err)
	}
	if out.EvidenceComplete || len(out.Findings) != 0 {
		t.Fatal("unrelated line accepted")
	}
}

func TestRepositoryRulesSnapshotAndPublication(t *testing.T) {
	f := newFixture(t)
	rules := "任务完成必须有匹配反馈；重试不得重复释放资源。"
	f.policy(t, db.CodeReviewPolicy{Rules: rules, SyncMRs: true})
	run, err := f.s.Enqueue(context.Background(), "10", "mr", "7", "rules-snapshot")
	if err != nil {
		t.Fatal(err)
	}
	var frozen db.CodeReviewPolicy
	if err := json.Unmarshal([]byte(run.PolicyJSON), &frozen); err != nil {
		t.Fatal(err)
	}
	if frozen.Rules != rules {
		t.Fatal("rules missing from immutable snapshot")
	}
	// Editing policy after enqueue must not alter either model pass.
	f.policy(t, db.CodeReviewPolicy{Rules: "新规则：必须记录状态转换审计日志。", SyncMRs: true})
	model := f.s.Generate
	calls := 0
	f.s.Generate = func(ctx context.Context, system, user string) (string, error) {
		calls++
		if !strings.Contains(system, rules) || strings.Contains(system, "新规则：") {
			t.Fatal("model did not receive frozen rules")
		}
		for _, rule := range DefaultReviewRules {
			if !strings.Contains(system, rule) {
				t.Fatal("default rule missing")
			}
		}
		return model(ctx, system, user)
	}
	if _, err := f.s.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("expected two review passes, got %d", calls)
	}
	if allowed, err := f.s.policyAllows(run); allowed || err == nil {
		t.Fatal("changed rules allowed old report publication")
	}
	if err := ValidatePolicy(db.CodeReviewPolicy{Domain: "general", Scenario: "general", Rules: strings.Repeat("规", 8001)}); err == nil {
		t.Fatal("oversized rules accepted")
	}
}
