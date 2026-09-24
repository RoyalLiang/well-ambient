package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func emailTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "email.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.AutoMigrate(&db.TaskTelemetry{}, &db.GitCommitLog{}, &userdb.User{}, &db.DailyJiraEmailRun{}, &db.EmailTemplateCandidate{}, &db.ConfigVersion{}, &db.RuntimeConfig{}); err != nil {
		t.Fatal(err)
	}
	previous := db.DB
	db.DB = conn
	sqlDB, err := conn.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { db.DB = previous; _ = sqlDB.Close() })
	return conn
}
func emailTestConfig() config.Config {
	return config.Config{SMTP: config.SMTPConfig{Enabled: true, Host: "127.0.0.1", Port: 2525, From: "sender@example.test", TLSMode: "none"}, Jira: config.JiraConfig{SyncUsers: []string{"alice"}}, DailyJiraEmail: config.DailyJiraEmailConfig{Recipients: []string{"to@example.test"}, Timezone: "America/New_York", SendTime: "09:00", IncludeCommits: true, Template: config.EmailTemplate{Subject: "报告 {{date}}", Introduction: "<img src=x onerror=alert(1)>", Closing: "</body><script>alert(1)</script>"}}}
}
func TestEmailCalendarBoundariesHandleDST(t *testing.T) {
	for _, tc := range []struct {
		date  string
		hours time.Duration
	}{{"2026-03-09", 23}, {"2026-11-02", 25}} {
		today, yesterday, recent, err := emailDateBounds(tc.date, "America/New_York", time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		if today.Sub(yesterday) != tc.hours*time.Hour {
			t.Fatalf("%s has %v hours", tc.date, today.Sub(yesterday).Hours())
		}
		if today.Hour() != 0 || yesterday.Hour() != 0 || recent.Hour() != 0 {
			t.Fatal("calendar boundary drift")
		}
	}
	if _, _, _, err := emailDateBounds("2026-02-30", "UTC", time.Now()); err == nil {
		t.Fatal("invalid date accepted")
	}
}
func TestEmailReportSourceDatesScopeAndHTML(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	s := &Server{config: &cfg}
	if err := conn.Create(&userdb.User{Username: "alice", Email: "alice@example.test", Name: "Alice"}).Error; err != nil {
		t.Fatal(err)
	}
	today, yesterday, recent, err := emailDateBounds("2026-03-09", cfg.DailyJiraEmail.Timezone, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	mixed := yesterday.In(time.FixedZone("+0800", 8*3600))
	tasks := []db.TaskTelemetry{
		{TaskID: "ABC-1", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Title: "<script>alert(1)</script>", Assignee: "Alice", Status: "progress", TaskCreatedAt: recent.UTC(), SourceUpdatedAt: mixed},
		{TaskID: "ABC-2", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Assignee: "Alice", Status: "progress", TaskCreatedAt: today.UTC(), SourceUpdatedAt: today.UTC()},
		{TaskID: "ABC-3", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Assignee: "Alice", Status: "done", TaskCreatedAt: yesterday.UTC(), SourceUpdatedAt: yesterday.Add(time.Hour).UTC()},
		{TaskID: "ABC-4", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Assignee: "outsider", Status: "progress", TaskCreatedAt: yesterday.UTC(), SourceUpdatedAt: yesterday.UTC()},
		{TaskID: "ABC-5", Source: "local", Assignee: "Alice", Status: "progress", TaskCreatedAt: yesterday.UTC(), SourceUpdatedAt: yesterday.UTC()},
		{TaskID: "ABC-6", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Assignee: "Alice", Status: "progress", TaskCreatedAt: recent.Add(-time.Second).UTC(), SourceUpdatedAt: recent.UTC(), LastUpdate: yesterday.UTC()},
		{TaskID: "ABC-7", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Assignee: "Alice", Status: "progress", TaskCreatedAt: recent.Add(-time.Second), SourceUpdatedAt: yesterday.Add(-time.Nanosecond)},
	}
	if err := conn.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	commits := []db.GitCommitLog{
		{Repo: "repo", CommitID: "sha", Author: "Alice", Action: "git_push", CreatedAt: mixed},
		{Repo: "repo", CommitID: "sha", Author: "Alice", Action: "git_push", CreatedAt: yesterday.UTC()},
		{Repo: "repo", CommitID: "other", Author: "outsider", Action: "git_push", CreatedAt: yesterday.UTC()},
		{Repo: "repo", CommitID: "mr", Author: "Alice", Action: "mr_merge", CreatedAt: yesterday.UTC()},
		{Repo: "repo", CommitID: "today", Author: "Alice", Action: "git_push", CreatedAt: today.UTC()},
	}
	if err := conn.Create(&commits).Error; err != nil {
		t.Fatal(err)
	}
	report, err := s.buildEmailReport(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", today)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.YesterdayUpdated) != 2 || report.YesterdayUpdated[0].TaskID != "ABC-1" || report.YesterdayUpdated[1].TaskID != "ABC-3" {
		t.Fatalf("wrong updated selection: %+v", report.YesterdayUpdated)
	}
	if len(report.RecentUnresolved) != 1 || report.RecentUnresolved[0].TaskID != "ABC-1" {
		t.Fatalf("wrong created selection: %+v", report.RecentUnresolved)
	}
	if report.Commits == nil || report.Commits.Count != 1 {
		t.Fatalf("wrong commits: %+v", report.Commits)
	}
	if len(report.CoreMembers) != 1 || report.CoreMembers[0].YesterdayCount != 2 || report.CoreMembers[0].CommitCount != 1 {
		t.Fatalf("wrong core: %+v", report.CoreMembers)
	}
	if strings.Contains(report.HTML, "<script>") || strings.Contains(report.HTML, "<img src=x") || !strings.Contains(report.HTML, "&lt;script&gt;") {
		t.Fatal("unsafe rendered HTML")
	}
	if !strings.Contains(report.Semantics, "source_updated_at") || !strings.Contains(report.Semantics, "采集时间") {
		t.Fatal("missing semantics")
	}
	cfg.Jira.SyncUsers = nil
	cfg.Jira.CustomJQL = ""
	report, err = s.buildEmailReport(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", today)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.YesterdayUpdated) != 0 || len(report.Warnings) == 0 {
		t.Fatal("empty scope exposes all facts")
	}
	cfg.DailyJiraEmail.IncludeCommits = false
	report, err = s.buildEmailReport(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", today)
	if err != nil {
		t.Fatal(err)
	}
	if report.Commits != nil || strings.Contains(report.HTML, "commit 统计分析") {
		t.Fatal("disabled commit block rendered")
	}
}
func TestEmailClaimsSurviveFailureAndSuppressDuplicateSend(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	s := &Server{config: &cfg}
	calls := 0
	s.emailSender = func(context.Context, config.SMTPConfig, []string, string, string, string) error {
		calls++
		return errors.New("transport interrupted")
	}
	now := time.Date(2026, 3, 9, 14, 0, 0, 0, time.UTC)
	if _, err := s.sendDailyEmail(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", "manual", now); err == nil {
		t.Fatal("failed transport succeeded")
	}
	restarted := &Server{config: &cfg, emailSender: s.emailSender}
	if _, err := restarted.sendDailyEmail(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", "scheduled", now); !errors.Is(err, errEmailAlreadyClaimed) {
		t.Fatalf("duplicate error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("transport calls=%d", calls)
	}
	var claim db.DailyJiraEmailRun
	if err := conn.First(&claim, "date = ?", "2026-03-09").Error; err != nil {
		t.Fatal(err)
	}
	if claim.Status != "failed_or_unknown" || claim.FinishedAt == nil {
		t.Fatalf("bad ledger: %+v", claim)
	}
}
func TestEmailConcurrentWorkersClaimOneAttempt(t *testing.T) {
	emailTestDatabase(t)
	cfg := emailTestConfig()
	s := &Server{config: &cfg}
	var calls atomic.Int32
	s.emailSender = func(context.Context, config.SMTPConfig, []string, string, string, string) error {
		calls.Add(1)
		return nil
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.sendDailyEmail(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", "scheduled", time.Now())
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	successes, duplicates := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, errEmailAlreadyClaimed) {
			duplicates++
		} else {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 || successes != 1 || duplicates != 1 {
		t.Fatalf("calls=%d success=%d duplicate=%d", calls.Load(), successes, duplicates)
	}
}
func TestEmailSchedulerDisabledDueAndCancellation(t *testing.T) {
	cfg := emailTestConfig()
	now := time.Date(2026, 3, 9, 14, 0, 0, 0, time.UTC)
	if _, due := emailScheduleDue(now, cfg); due {
		t.Fatal("disabled mail due")
	}
	cfg.DailyJiraEmail.Enabled = true
	if date, due := emailScheduleDue(now, cfg); !due || date != "2026-03-09" {
		t.Fatalf("expected due %s %v", date, due)
	}
	if _, due := emailScheduleDue(now.Add(-2*time.Hour), cfg); due {
		t.Fatal("early due")
	}
	cfg.SMTP.Enabled = false
	if _, due := emailScheduleDue(now, cfg); due {
		t.Fatal("disabled SMTP due")
	}
	s := &Server{config: &cfg}
	s.setEmailConfig(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	select {
	case <-s.startEmailWorker(ctx):
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestExportEmailPreviews(t *testing.T) {
	for _, style := range []string{"brief", "focus", "ledger", "hyperframe"} {
		tmpl := config.DefaultEmailTemplate()
		tmpl.Style = style
		demo, err := emailTemplateDemo(tmpl)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join("..", "..", "outputs", fmt.Sprintf("email-template-%s.html", style))
		if err := os.WriteFile(path, []byte(demo.HTML), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEmailSchedulerNeverReclaimsAlreadySentDate(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	cfg.SMTP.Enabled = true
	cfg.DailyJiraEmail.Enabled = true
	cfg.DailyJiraEmail.SendTime = "17:31"
	cfg.DailyJiraEmail.Timezone = "Asia/Shanghai"
	cfg.DailyJiraEmail.Recipients = []string{"qa@example.com"}

	var sendCount atomic.Int32
	s := &Server{config: &cfg}
	s.setEmailConfig(cfg)
	s.emailSender = func(ctx context.Context, smtpCfg config.SMTPConfig, recipients []string, subject, text, html string) error {
		sendCount.Add(1)
		return nil
	}

	loc, _ := time.LoadLocation("Asia/Shanghai")
	// 1. Simulate an earlier run from 12:45 today (e.g. startup catchup or older send time).
	earlierToday := time.Date(2026, 9, 16, 12, 45, 0, 0, loc)
	staleRun := db.DailyJiraEmailRun{
		Date:      "2026-09-16",
		Timezone:  "Asia/Shanghai",
		Trigger:   "scheduled",
		Status:    "sent",
		StartedAt: earlierToday.UTC(),
	}
	if err := conn.Create(&staleRun).Error; err != nil {
		t.Fatal(err)
	}

	// 2. Now at 17:30 (before send_time), it should NOT be due yet.
	at1730 := time.Date(2026, 9, 16, 17, 30, 0, 0, loc)
	s.runEmailSchedule(context.Background(), at1730)
	if sendCount.Load() != 0 {
		t.Fatalf("expected 0 sends before send_time, got %d", sendCount.Load())
	}

	// 3. At 17:31 (send_time arrived), runEmailSchedule should recognize that the earlier 12:45 run was before 17:31, clear it, and send!
	at1731 := time.Date(2026, 9, 16, 17, 31, 0, 0, loc)
	s.runEmailSchedule(context.Background(), at1731)
	if sendCount.Load() != 0 {
		t.Fatalf("expected no resend at scheduled time, got %d", sendCount.Load())
	}

	var updatedRun db.DailyJiraEmailRun
	if err := conn.First(&updatedRun, "date = ?", "2026-09-16").Error; err != nil {
		t.Fatal(err)
	}
	if updatedRun.Status != "sent" || updatedRun.Trigger != "scheduled" {
		t.Fatalf("unexpected updated run: %+v", updatedRun)
	}
	if updatedRun.StartedAt.In(loc).Format("15:04") != "12:45" {
		t.Fatalf("expected run time 17:31, got %s", updatedRun.StartedAt.In(loc).Format("15:04"))
	}

	// 4. Calling at 17:32 must NOT duplicate!
	at1732 := time.Date(2026, 9, 16, 17, 32, 0, 0, loc)
	s.runEmailSchedule(context.Background(), at1732)
	if sendCount.Load() != 0 {
		t.Fatalf("expected no duplicate send, got %d", sendCount.Load())
	}

	// A manual delivery also consumes the date claim.
	tomorrowLoc := time.Date(2026, 9, 17, 10, 0, 0, 0, loc)
	manualRun := db.DailyJiraEmailRun{
		Date:      "2026-09-17",
		Timezone:  "Asia/Shanghai",
		Trigger:   "manual",
		Status:    "sent",
		StartedAt: tomorrowLoc.UTC(),
	}
	if err := conn.Create(&manualRun).Error; err != nil {
		t.Fatal(err)
	}

	atTomorrowScheduled := time.Date(2026, 9, 17, 17, 31, 0, 0, loc)
	s.runEmailSchedule(context.Background(), atTomorrowScheduled)
	if sendCount.Load() != 0 {
		t.Fatalf("manual delivery must prevent scheduled duplicate, got %d", sendCount.Load())
	}
}

func TestEmailWorkerWakeupOnConfigChange(t *testing.T) {
	_ = emailTestDatabase(t)
	cfg := emailTestConfig()
	cfg.SMTP.Enabled = true
	cfg.DailyJiraEmail.Enabled = true
	cfg.DailyJiraEmail.SendTime = "17:31"
	cfg.DailyJiraEmail.Timezone = "Asia/Shanghai"
	cfg.DailyJiraEmail.Recipients = []string{"qa@example.com"}

	s := &Server{config: &cfg}
	s.setEmailConfig(cfg)
	s.emailSender = func(ctx context.Context, smtpCfg config.SMTPConfig, recipients []string, subject, text, html string) error {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := s.startEmailWorker(ctx)

	s.triggerEmailWorker()
	select {
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit cleanly")
	}
}

func TestEmailProjectGrouping(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	s := &Server{config: &cfg}

	_ = conn.Create(&userdb.User{Username: "alice", Name: "Alice", Email: "alice@example.com"}).Error
	_ = conn.Create(&userdb.User{Username: "bob", Name: "Bob", Email: "bob@example.com"}).Error
	_ = conn.Create(&userdb.User{Username: "charlie", Name: "Charlie", Email: "charlie@example.com"}).Error
	_ = conn.Create(&userdb.User{Username: "eve", Name: "Eve", Email: "eve@example.com"}).Error

	cfg.Jira.SyncUsers = []string{"alice", "bob", "charlie", "eve"}
	cfg.DailyJiraEmail.ProjectGroups = []config.DailyJiraProjectGroup{
		{
			Name:     "FMS核心组",
			Owners:   []string{"Alice", "Bob"},
			Projects: []string{"FMS", "CORE"},
		},
		{
			Name:     "GPP调度组",
			Owners:   []string{"Charlie"},
			Projects: []string{"GPP"},
		},
	}

	today, yesterday, recent, err := emailDateBounds("2026-03-09", cfg.DailyJiraEmail.Timezone, time.Time{})
	if err != nil {
		t.Fatal(err)
	}

	tasks := []db.TaskTelemetry{
		{TaskID: "FMS-101", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Title: "FMS调度核心更新", Assignee: "Alice", Status: "done", TaskCreatedAt: yesterday.UTC(), SourceUpdatedAt: yesterday.Add(time.Hour).UTC()},
		{TaskID: "GPP-202", Source: "jira", JiraBugCategory: "GPP", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Title: "GPP避让优化", Assignee: "Charlie", Status: "progress", TaskCreatedAt: yesterday.UTC(), SourceUpdatedAt: yesterday.Add(2 * time.Hour).UTC()},
		{TaskID: "MISC-303", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Title: "杂项运维", Assignee: "Bob", Status: "progress", TaskCreatedAt: yesterday.UTC(), SourceUpdatedAt: yesterday.Add(3 * time.Hour).UTC()}, // Ungrouped (Eve is not in any group's owners, OTH prefix not in any group's projects)
		{TaskID: "OTH-404", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Title: "独立小需求", Assignee: "Eve", Status: "progress", TaskCreatedAt: yesterday.UTC(), SourceUpdatedAt: yesterday.Add(4 * time.Hour).UTC()},
		// Recent unresolved for FMS核心组
		{TaskID: "FMS-105", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Title: "FMS锁站缺陷", Assignee: "Alice", Status: "progress", TaskCreatedAt: recent.Add(time.Hour).UTC(), SourceUpdatedAt: recent.Add(time.Hour).UTC()},
	}
	if err := conn.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}

	report, err := s.buildEmailReport(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", today)
	if err != nil {
		t.Fatal(err)
	}

	if len(report.YesterdayGroups) != 2 {
		t.Fatalf("expected 2 yesterday groups, got %d", len(report.YesterdayGroups))
	}
	if report.YesterdayGroups[0].Name != "FMS核心组" || report.YesterdayGroups[0].Count != 1 {
		t.Fatalf("FMS核心组 expected 1 item, got %+v", report.YesterdayGroups[0])
	}
	if report.YesterdayGroups[1].Name != "GPP调度组" || report.YesterdayGroups[1].Count != 1 {
		t.Fatalf("GPP调度组 expected 1 item, got %+v", report.YesterdayGroups[1])
	}
	if len(report.YesterdayUngrouped) != 2 || report.YesterdayUngrouped[0].TaskID != "MISC-303" || report.YesterdayUngrouped[1].TaskID != "OTH-404" {
		t.Fatalf("YesterdayUngrouped expected 1 item (OTH-404), got %+v", report.YesterdayUngrouped)
	}
	if len(report.UnresolvedGroups) != 2 || report.UnresolvedGroups[0].Count != 1 {
		t.Fatalf("UnresolvedGroups expected 2 items in group 0, got %+v", report.UnresolvedGroups)
	}

	if len(report.ProjectGroupStats) != 3 {
		t.Fatalf("expected 3 ProjectGroupStats entries, got %d: %+v", len(report.ProjectGroupStats), report.ProjectGroupStats)
	}
	fmsStat := report.ProjectGroupStats[0]
	if fmsStat.Name != "FMS核心组" || fmsStat.YesterdayCount != 1 || fmsStat.ResolvedCount != 1 || fmsStat.ResolutionRate != 100 || fmsStat.RecentUnresolvedCount != 1 {
		t.Fatalf("unexpected FMS核心组 stats: %+v", fmsStat)
	}
	if fmsStat.ProjectCount != 2 {
		t.Fatalf("unexpected FMS核心组 ProjectCount: %d, expected 2", fmsStat.ProjectCount)
	}
	if report.ProjectGroupStats[1].ProjectCount != 1 {
		t.Fatalf("unexpected GPP调度组 ProjectCount: %d, expected 1", report.ProjectGroupStats[1].ProjectCount)
	}
	if report.ProjectGroupStats[2].ProjectCount != 2 {
		t.Fatalf("unexpected ungrouped ProjectCount: %d, expected 1", report.ProjectGroupStats[2].ProjectCount)
	}

	for _, c := range report.JiraCharts {
		if strings.Contains(c.Title, "近 3 天未解决 · 负责人分布") {
			t.Fatalf("JiraCharts should not contain individual owner chart when project groups exist, got: %s", c.Title)
		}
	}

	for _, needle := range []string{"FMS核心组", "GPP调度组", "其他项目 / 未分组", "Alice、Bob", "项目数"} {
		if !strings.Contains(report.HTML, needle) {
			t.Fatalf("HTML missing %q", needle)
		}
		if !strings.Contains(report.Text, needle) {
			t.Fatalf("Text missing %q", needle)
		}
	}

	for _, style := range []string{"brief", "focus", "ledger", "hyperframe"} {
		reportCopy := report
		reportCopy.Style = style
		if err := renderEmailReport(&reportCopy); err != nil {
			t.Fatalf("style %s failed to render: %v", style, err)
		}
		if !strings.Contains(reportCopy.HTML, "FMS核心组") {
			t.Fatalf("style %s HTML missing FMS核心组", style)
		}
		if strings.Contains(reportCopy.HTML+reportCopy.Text, "项目负责人分组统计") {
			t.Fatalf("style %s retained redundant group statistics", style)
		}
		if !strings.Contains(reportCopy.HTML, "email-group-summary") || !strings.Contains(reportCopy.Text, "项目分组概览") {
			t.Fatalf("style %s lost the top group overview", style)
		}
	}
}

func TestEmailReportProjectGroupIsolationAndNoTitleSubstringHijack(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	cfg.Jira.SyncUsers = []string{"朱祥", "现场交付", "梁志远", "吴回", "朱家聪", "陈义东"}
	s := &Server{}

	groups := []config.DailyJiraProjectGroup{
		{
			Name:     "梁志远 朱家聪 吴回",
			Owners:   []string{"梁志远", "朱家聪", "吴回"},
			Projects: []string{"PRJ23096", "CR", "NS2", "TH", "shg", "HACTL2", "HKAA", "ML", "PACTL", "PD", "EZ", "WLY"},
		},
		{
			Name:     "陈义东 潘祺鑫",
			Owners:   []string{"陈义东", "潘祺鑫"},
			Projects: []string{},
		},
		{
			Name:     "邓强 穆陆振",
			Owners:   []string{"邓强", "穆陆振"},
			Projects: []string{"FEL", "FEL2WD", "AB", "ICA", "AQCT"},
		},
		{
			Name:     "吕博兴 朱祥",
			Owners:   []string{"吕博兴", "朱祥"},
			Projects: []string{"HR", "MDL", "FZ", "ZK", "tpy", "QZ", "quz", "YH", "HIT", "DL", "LZ", "ZPU", "LUZ", "CK", "LCB", "WUH", "YBET"},
		},
	}
	cfg.DailyJiraEmail.ProjectGroups = groups

	today, yesterday, recent, err := emailDateBounds("2026-03-09", cfg.DailyJiraEmail.Timezone, time.Time{})
	if err != nil {
		t.Fatal(err)
	}

	tasks := []db.TaskTelemetry{
		// 1. FEL2WD 任务，标题包含 Berth 7（含 th），经办人朱祥。必须归入邓强组，绝不能因为标题含 th 进梁志远组，也不能因为经办人进朱祥组
		{TaskID: "FEL2WD-2847", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Title: "【生产】AT56 AT35 AT12在Berth 7 下岸桥时发生死锁 —— 人工拉车", Assignee: "朱祥", Status: "progress", TaskCreatedAt: recent.Add(time.Hour).UTC(), SourceUpdatedAt: yesterday.UTC()},
		// 2. ICA 任务，标题包含 the / create（含 th / cr），经办人现场团队。必须归入邓强组，绝不能误入梁志远组
		{TaskID: "ICA-11180", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Title: "FMS is not able to create a route to any Lane 4 task when there is a QC close to the TLS6", Assignee: "现场交付", Status: "progress", TaskCreatedAt: recent.Add(2 * time.Hour).UTC(), SourceUpdatedAt: yesterday.UTC()},
		// 3. FEL2WD 任务，经办人是梁志远。由于 FEL2WD 属于邓强组项目，必须归入邓强组，梁志远组内严禁出现 FEL2WD
		{TaskID: "FEL2WD-2760", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Title: "【对位2.0上线】FMS适配精停", Assignee: "梁志远", Status: "progress", TaskCreatedAt: recent.Add(3 * time.Hour).UTC(), SourceUpdatedAt: yesterday.UTC()},
		// 4. 正宗属于梁志远组的项目
		{TaskID: "TH-101", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Title: "TH现场堆场路径优化", Assignee: "吴回", Status: "progress", TaskCreatedAt: recent.Add(4 * time.Hour).UTC(), SourceUpdatedAt: yesterday.UTC()},
		{TaskID: "CR-202", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Title: "CR装箱流程修复", Assignee: "朱家聪", Status: "progress", TaskCreatedAt: recent.Add(5 * time.Hour).UTC(), SourceUpdatedAt: yesterday.UTC()},
		// 5. 无 Projects 的纯人员组（陈义东 潘祺鑫），经办人是陈义东，项目为外围公共项目 ZJK
		{TaskID: "ZJK-303", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Title: "张家口现场适配", Assignee: "陈义东", Status: "progress", TaskCreatedAt: recent.Add(6 * time.Hour).UTC(), SourceUpdatedAt: yesterday.UTC()},
	}
	if err := conn.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}

	report, err := s.buildEmailReport(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", today)
	if err != nil {
		t.Fatal(err)
	}

	var lzyGroup *emailIssueGroup
	var dqGroup *emailIssueGroup
	var cydGroup *emailIssueGroup
	for i := range report.UnresolvedGroups {
		g := &report.UnresolvedGroups[i]
		switch g.Name {
		case "梁志远 朱家聪 吴回":
			lzyGroup = g
		case "邓强 穆陆振":
			dqGroup = g
		case "陈义东 潘祺鑫":
			cydGroup = g
		}
	}

	if lzyGroup == nil || dqGroup == nil || cydGroup == nil {
		t.Fatalf("missing expected groups in UnresolvedGroups: %+v", report.UnresolvedGroups)
	}

	// 彻查验证：梁志远组内绝对不能包含 FEL2WD 或 ICA！
	for _, issue := range lzyGroup.Issues {
		if strings.HasPrefix(issue.TaskID, "FEL2WD") || strings.HasPrefix(issue.TaskID, "ICA") {
			t.Fatalf("梁志远组内严禁出现非本组项目：%s (Title: %s)", issue.TaskID, issue.Title)
		}
	}
	if lzyGroup.Count != 2 {
		t.Fatalf("梁志远组应仅包含 TH-101 与 CR-202，实际包含 %d 项: %+v", lzyGroup.Count, lzyGroup.Issues)
	}

	// 验证邓强组正确接管 FEL2WD 与 ICA
	if dqGroup.Count != 3 {
		t.Fatalf("邓强组应包含 FEL2WD-2847, ICA-11180, FEL2WD-2760 共 3 项，实际包含 %d 项: %+v", dqGroup.Count, dqGroup.Issues)
	}

	// 验证纯人员组正常归类
	if cydGroup.Count != 1 || cydGroup.Issues[0].TaskID != "ZJK-303" {
		t.Fatalf("陈义东组应归入 ZJK-303，实际: %+v", cydGroup.Issues)
	}
}

func TestEmailAndConfluenceIssueTypeDisplay(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	s := &Server{}

	today, yesterday, recent, err := emailDateBounds("2026-03-09", cfg.DailyJiraEmail.Timezone, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	cfg.Jira.SyncUsers = []string{"张三", "李四", "王五"}

	tasks := []db.TaskTelemetry{
		{TaskID: "BUG-101", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Title: "调度死锁修复", Assignee: "张三", Status: "progress", IssueType: "bug", TaskCreatedAt: recent.Add(time.Hour).UTC(), SourceUpdatedAt: yesterday.Add(time.Hour).UTC()},
		{TaskID: "TSK-202", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Title: "路网拓扑发布", Assignee: "李四", Status: "progress", IssueType: "task", TaskCreatedAt: recent.Add(2 * time.Hour).UTC(), SourceUpdatedAt: yesterday.Add(2 * time.Hour).UTC()},
		{TaskID: "REQ-303", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Title: "换电流程新特性", Assignee: "王五", Status: "done", IssueType: "requirement", TaskCreatedAt: recent.Add(3 * time.Hour).UTC(), SourceUpdatedAt: yesterday.Add(3 * time.Hour).UTC()},
	}
	if err := conn.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}

	report, err := s.buildEmailReport(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", today)
	if err != nil {
		t.Fatal(err)
	}

	// 1. 验证报告内部字段正确映射
	typeMap := map[string]string{}
	for _, it := range report.YesterdayUpdated {
		typeMap[it.TaskID] = it.IssueType
	}
	if typeMap["BUG-101"] != "缺陷" {
		t.Fatalf("BUG-101 expected '缺陷', got %q", typeMap["BUG-101"])
	}
	if typeMap["TSK-202"] != "任务" {
		t.Fatalf("TSK-202 expected '任务', got %q", typeMap["TSK-202"])
	}
	if typeMap["REQ-303"] != "需求" {
		t.Fatalf("REQ-303 expected '需求', got %q", typeMap["REQ-303"])
	}

	// 2. 验证邮件 HTML 包含类型列以及彩色徽章
	if !strings.Contains(report.HTML, "类型") {
		t.Fatalf("email HTML table must contain '类型' header, got: %s", report.HTML)
	}
	if !strings.Contains(report.HTML, ">缺陷</span>") {
		t.Fatalf("email HTML table must render '缺陷' badge")
	}
	if !strings.Contains(report.HTML, ">任务</span>") {
		t.Fatalf("email HTML table must render '任务' badge")
	}

	// 3. 验证 Confluence 存储格式中包含类型列及数据
	storage, _, err := confluenceReportStorage(&report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(storage, "<th>Jira</th><th>类型</th><th>事项</th>") {
		t.Fatalf("Confluence table must contain '<th>Jira</th><th>类型</th><th>事项</th>', got: %s", storage)
	}
	if !strings.Contains(storage, "<td>缺陷</td>") || !strings.Contains(storage, "<td>任务</td>") {
		t.Fatalf("Confluence table must contain cells '<td>缺陷</td>' and '<td>任务</td>', got: %s", storage)
	}
}
