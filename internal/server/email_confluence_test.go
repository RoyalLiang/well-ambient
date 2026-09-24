package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"image/png"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestEmailConfluenceStorageKeepsFactsAndPNGAttachments(t *testing.T) {
	for _, style := range []string{"brief", "focus", "ledger", "hyperframe"} {
		report, err := emailTemplateDemo(config.EmailTemplate{Style: style})
		if err != nil {
			t.Fatal(err)
		}
		report.ConfluenceURL = "https://confluence.westwell-lab.com/pages/viewpage.action?pageId=123"
		if err := renderEmailReport(&report); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(report.HTML, `class="email-confluence-link"`) || !strings.Contains(report.Text, report.ConfluenceURL) {
			t.Fatalf("%s lost the archive link", style)
		}
		storage, attachments, err := confluenceReportStorage(&report)
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{"<script", "<style", "data:image", "cid:", "__configured__", report.ConfluenceURL} {
			if strings.Contains(storage, bad) {
				t.Fatalf("storage contains forbidden/self-referential value %q", bad)
			}
		}
		for _, fact := range []string{"DEMO-101", "DEMO-102", "DEMO-103", "示例负责人甲", "昨日更新 Jira", "近3天创建且未解决", "昨日 commit 统计分析", "-1d", "-7d"} {
			if !strings.Contains(storage, fact) {
				t.Fatalf("storage lost report fact %q", fact)
			}
		}
		if !strings.Contains(storage, "<th>解决率指标</th><th>状态分布</th><th>近7日更新分布</th>") {
			t.Fatal("top charts must be laid out side-by-side in a 3-column table header")
		}
		if strings.Contains(storage, "<h2>解决率指标</h2>") || strings.Contains(storage, "<h2>近7日更新分布</h2>") {
			t.Fatal("top charts must not be vertically stacked with individual h2 headings")
		}
		if len(attachments) < 3 {
			t.Fatal("report charts must be backed by attachments")
		}
		names := map[string]bool{}
		for _, attachment := range attachments {
			if names[attachment.Filename] || attachment.ContentType != "image/png" {
				t.Fatal("attachments need unique content-addressed names and PNG types")
			}
			names[attachment.Filename] = true
			hash := sha256.Sum256(attachment.Data)
			if attachment.Filename != fmt.Sprintf("daily-jira-chart-%x.png", hash) ||
				!strings.Contains(storage, `ri:filename="`+attachment.Filename+`"`) {
				t.Fatal("chart reference/content mismatch")
			}
			if _, err := png.Decode(bytes.NewReader(attachment.Data)); err != nil {
				t.Fatal(err)
			}
		}
		decoder := xml.NewDecoder(strings.NewReader(`<root xmlns:ac="urn:ac" xmlns:ri="urn:ri">` + storage + "</root>"))
		for {
			if _, err := decoder.Token(); err == io.EOF {
				break
			} else if err != nil {
				t.Fatalf("invalid Confluence XHTML: %v", err)
			}
		}
		report.Commits = &emailCommitSummary{Authors: map[string]int{}}
		report.Introduction = `<script>alert("x")</script>`
		if err := renderEmailReport(&report); err != nil {
			t.Fatal(err)
		}
		storage, _, err = confluenceReportStorage(&report)
		if err != nil || strings.Contains(storage, "<script>") || !strings.Contains(storage, "采集提交：0 次") {
			t.Fatal("zero statistics and escaped user copy must survive archiving")
		}
	}
}

func TestEmailConfluenceTopChartsRenderSideBySideWithoutWrapping(t *testing.T) {
	report, err := emailTemplateDemo(config.EmailTemplate{Style: "brief"})
	if err != nil {
		t.Fatal(err)
	}
	if err := renderEmailReport(&report); err != nil {
		t.Fatal(err)
	}
	storage, attachments, err := confluenceReportStorage(&report)
	if err != nil {
		t.Fatal(err)
	}
	if len(attachments) < 3 {
		t.Fatalf("expected at least 3 chart attachments, got %d", len(attachments))
	}
	// Verify single 3-column table
	const expectedHeader = "<th>解决率指标</th><th>状态分布</th><th>近7日更新分布</th>"
	if !strings.Contains(storage, expectedHeader) {
		t.Fatalf("storage does not contain side-by-side chart table: %s", storage)
	}
	// Verify trend data is rendered cleanly as inline text with dates and counts
	if !strings.Contains(storage, "-7d: 4") || !strings.Contains(storage, "-1d: 3") {
		t.Fatalf("storage does not contain compact non-wrapping trend points: %s", storage)
	}
	// Verify status distribution is cleanly joined
	if !strings.Contains(storage, "done 1") || !strings.Contains(storage, "progress 1") || !strings.Contains(storage, "review 1") {
		t.Fatalf("storage lost status bar summary: %s", storage)
	}
	// Verify XHTML valid
	decoder := xml.NewDecoder(strings.NewReader(`<root xmlns:ac="urn:ac" xmlns:ri="urn:ri">` + storage + "</root>"))
	for {
		if _, err := decoder.Token(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("invalid XHTML storage format: %v", err)
		}
	}
}

func TestEmailConfluenceOptimizedStatsOnlyAndProgressBars(t *testing.T) {
	report, err := emailTemplateDemo(config.EmailTemplate{Style: "brief"})
	if err != nil {
		t.Fatal(err)
	}
	report.Introduction = "各位好，这是今日测试早报问候语，不应出现在归档中。"
	report.Closing = "祝工作顺利，不应出现在归档中。"
	report.Semantics = "这是大段口径说明描述，不应出现在归档中。"
	if err := renderEmailReport(&report); err != nil {
		t.Fatal(err)
	}
	storage, attachments, err := confluenceReportStorage(&report)
	if err != nil {
		t.Fatal(err)
	}

	// 1. 验证描述性客套文字已优化移除，只保留统计与分析相关数据
	for _, verbose := range []string{report.Introduction, report.Closing, report.Semantics} {
		if strings.Contains(storage, verbose) {
			t.Fatalf("confluence storage should not contain verbose copy %q", verbose)
		}
	}
	// 验证核心统计与分析数据完整保留，且大盘态势、项目分组统计、重点跟进建议分别以独立表格展示
	for _, tableHeading := range []string{"早报概览", "大盘态势", "项目分组统计", "重点跟进建议", "统计图表"} {
		if !strings.Contains(storage, tableHeading) {
			t.Fatalf("storage must retain table heading %q", tableHeading)
		}
	}
	if !strings.Contains(storage, "<th>昨日更新</th><th>已解决</th><th>解决率</th><th>近 3 天待跟进</th>") {
		t.Fatal("storage must contain overall posture table")
	}
	if !strings.Contains(storage, "<th>重点跟进对象</th><th>负责人</th><th>待跟进事项</th><th>待跟进占比</th><th>建议行动</th>") {
		t.Fatal("storage must contain action recommendation table")
	}

	// 2. 验证进度条样式已优化为高保真嵌入式 HTML 进度条
	if !strings.Contains(storage, "background-color:#008f96") || !strings.Contains(storage, "background-color:#e2e8f0") {
		t.Fatal("storage must embed high-fidelity progress bars with inline styling")
	}
	if !strings.Contains(storage, "<th>分组 / 状态</th>") || !strings.Contains(storage, ">进度</th>") {
		t.Fatal("Jira distribution charts must be rendered as unified statistical tables with progress columns")
	}

	// 3. 验证曲线图底部横轴移除了冗余的数值行
	if strings.Contains(string(report.CurveChartSVG), `<td style="padding:0;font-weight:600">`) {
		t.Fatal("curve chart bottom axis should no longer contain separate data value rows")
	}

	// 4. 验证附件及 XML 存储格式完全有效
	if len(attachments) < 3 {
		t.Fatalf("expected at least 3 chart attachments, got %d", len(attachments))
	}
	decoder := xml.NewDecoder(strings.NewReader(`<root xmlns:ac="urn:ac" xmlns:ri="urn:ri">` + storage + "</root>"))
	for {
		if _, err := decoder.Token(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("invalid Confluence XHTML storage format: %v", err)
		}
	}
}

func confluenceEmailTestConfig() config.Config {
	cfg := emailTestConfig()
	cfg.DailyJiraEmail.Confluence = config.ConfluenceSyncConfig{
		Enabled: true, ParentPageURL: config.DefaultConfluenceParentPageURL, Token: "confluence-test-token",
	}
	return cfg
}

func TestEmailConfluenceSyncBeforeSMTPAndRetry(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := confluenceEmailTestConfig()
	s := &Server{config: &cfg}
	const pageURL = "https://confluence.westwell-lab.com/pages/viewpage.action?pageId=123"
	var syncCalls, sends int
	s.emailConfluenceSync = func(ctx context.Context, settings config.ConfluenceSyncConfig, report *emailReport) (string, error) {
		syncCalls++
		if settings.Token != cfg.DailyJiraEmail.Confluence.Token || report.Date != "2026-03-09" {
			t.Fatal("sync lost credentials or report date")
		}
		if syncCalls == 1 {
			return "", errors.New("upstream error containing confluence-test-token")
		}
		return pageURL, nil
	}
	s.emailSender = func(_ context.Context, _ config.SMTPConfig, _ []string, _, text, html string) error {
		sends++
		if syncCalls != 1 || strings.Contains(text, pageURL) || strings.Contains(html, pageURL) {
			t.Fatal("mail must follow successful sync and include its link in both formats")
		}
		return nil
	}
	now := time.Now()
	if _, err := s.sendDailyEmail(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", "manual", now); err == nil || strings.Contains(err.Error(), cfg.DailyJiraEmail.Confluence.Token) {
		t.Fatal("sync failure must block sending without exposing its raw error")
	}
	var run db.DailyJiraEmailRun
	if err := conn.First(&run).Error; err != nil {
		t.Fatal(err)
	}
	if sends != 1 || run.Status != "sent" || run.ConfluenceStatus != "failed" || run.FinishedAt == nil {
		t.Fatalf("unexpected sync failure ledger: %+v, sends %d", run, sends)
	}
	if _, err := s.sendDailyEmail(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", "manual", now); err != nil {
		t.Fatal(err)
	}
	if err := conn.First(&run).Error; err != nil {
		t.Fatal(err)
	}
	if sends != 1 || run.Status != "sent" || run.ConfluenceURL != pageURL || run.ConfluenceStatus != "synced" {
		t.Fatalf("wrong delivery result: %+v, sends %d", run, sends)
	}
	if _, err := s.sendDailyEmail(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", "manual", now); !errors.Is(err, errEmailAlreadyClaimed) || syncCalls != 2 {
		t.Fatal("already-sent date must not sync or send again")
	}
}

func TestEmailConfluenceRetryClaimIsExclusive(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := confluenceEmailTestConfig()
	if err := conn.Create(&db.DailyJiraEmailRun{Date: "2026-03-09", Status: "confluence_failed"}).Error; err != nil {
		t.Fatal(err)
	}
	s := &Server{config: &cfg}
	var syncCalls, sends atomic.Int32
	s.emailConfluenceSync = func(context.Context, config.ConfluenceSyncConfig, *emailReport) (string, error) {
		syncCalls.Add(1)
		return "https://confluence.westwell-lab.com/pages/viewpage.action?pageId=123", nil
	}
	s.emailSender = func(context.Context, config.SMTPConfig, []string, string, string, string) error {
		sends.Add(1)
		return nil
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.sendDailyEmail(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", "manual", time.Now())
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, errEmailAlreadyClaimed):
			conflicts++
		default:
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 1 || syncCalls.Load() != 1 || sends.Load() != 1 {
		t.Fatal("concurrent retries must have one sync and one send")
	}
}

func TestEmailConfluenceDisabledAndPreviewNeverSync(t *testing.T) {
	emailTestDatabase(t)
	cfg := confluenceEmailTestConfig()
	s := &Server{config: &cfg}
	s.emailConfluenceSync = func(context.Context, config.ConfluenceSyncConfig, *emailReport) (string, error) {
		t.Fatal("preview/disabled send must not write Confluence")
		return "", nil
	}
	draft := cfg.DailyJiraEmail
	draft.Confluence.Token = configuredSecretPlaceholder
	body, _ := json.Marshal(emailReportRequest{Date: "2026-03-09", Settings: &draft})
	response := httptest.NewRecorder()
	s.handlePreviewEmail(response, httptest.NewRequest("POST", "/api/daily-jira-email/preview", strings.NewReader(string(body))))
	if response.Code != 200 || strings.Contains(response.Body.String(), cfg.DailyJiraEmail.Confluence.Token) {
		t.Fatalf("preview status %d", response.Code)
	}
	cfg.DailyJiraEmail.Confluence.Enabled = false
	s.emailSender = func(context.Context, config.SMTPConfig, []string, string, string, string) error { return nil }
	if _, err := s.sendDailyEmail(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", "manual", time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestEmailConfluenceSecretRedactionPersistenceAndDestination(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := confluenceEmailTestConfig()
	s := &Server{config: &cfg}
	version, err := s.recordConfigVersion(config.Config{}, cfg, httptest.NewRequest("POST", "/api/config", nil), "manual-save", 0)
	if err != nil {
		t.Fatal(err)
	}
	token := cfg.DailyJiraEmail.Confluence.Token
	for _, raw := range []string{version.ConfigJSON, version.RedactedConfigJSON, version.DiffJSON} {
		if strings.Contains(raw, token) {
			t.Fatal("Confluence token leaked into config history")
		}
	}
	var runtime db.RuntimeConfig
	if err := conn.First(&runtime).Error; err != nil || !strings.Contains(runtime.ConfigJSON, token) {
		t.Fatal("runtime configuration must retain the token")
	}
	response := httptest.NewRecorder()
	s.handleGetConfig(response, httptest.NewRequest("GET", "/api/config", nil))
	if strings.Contains(response.Body.String(), token) {
		t.Fatal("configuration response exposed token")
	}
	var redacted config.Config
	if err := json.Unmarshal(response.Body.Bytes(), &redacted); err != nil {
		t.Fatal(err)
	}
	if redacted.DailyJiraEmail.Confluence.Token != configuredSecretPlaceholder {
		t.Fatal("missing configured-token marker")
	}
	mergeConfiguredSecrets(&redacted, cfg)
	if redacted.DailyJiraEmail.Confluence.Token != token {
		t.Fatal("unchanged destination must preserve configured token")
	}
	redacted.DailyJiraEmail.Confluence.Token = configuredSecretPlaceholder
	redacted.DailyJiraEmail.Confluence.ParentPageURL = "https://other.example.test/pages/viewpage.action?pageId=2"
	mergeConfiguredSecrets(&redacted, cfg)
	if redacted.DailyJiraEmail.Confluence.Token != "" {
		t.Fatal("stored token must not be silently sent to a changed destination")
	}
	var restored config.Config
	if err := BootstrapVersionedConfig(&restored); err != nil || restored.DailyJiraEmail.Confluence.Token != token {
		t.Fatal("restart lost the Confluence credential")
	}
}

func TestEmailConfluenceSchedulerKeepsSyncClaim(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := confluenceEmailTestConfig()
	cfg.DailyJiraEmail.Enabled = true
	cfg.DailyJiraEmail.Timezone = "UTC"
	cfg.DailyJiraEmail.SendTime = "09:00"
	now := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	for _, state := range []string{"syncing_confluence", "confluence_failed", "sending", "failed_or_unknown"} {
		conn.Where("date = ?", "2026-09-17").Delete(&db.DailyJiraEmailRun{})
		if err := conn.Create(&db.DailyJiraEmailRun{Date: "2026-09-17", Status: state, Trigger: "manual", StartedAt: now.Add(-time.Hour)}).Error; err != nil {
			t.Fatal(err)
		}
		s := &Server{config: &cfg}
		s.emailConfluenceSync = func(context.Context, config.ConfluenceSyncConfig, *emailReport) (string, error) {
			t.Fatal("scheduler replaced an in-flight or explicitly retryable claim")
			return "", nil
		}
		s.runEmailSchedule(context.Background(), now)
		var run db.DailyJiraEmailRun
		if err := conn.First(&run).Error; err != nil || run.Status != state {
			t.Fatal("scheduler changed protected run state")
		}
	}
}
