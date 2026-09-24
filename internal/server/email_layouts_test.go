package server

import (
	"context"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/config"
)

func TestEmailLayoutsHaveDistinctReadingOrderAndSameFacts(t *testing.T) {
	rendered := map[string]string{}
	for _, style := range []string{"brief", "focus", "ledger", "hyperframe"} {
		template := config.DefaultEmailTemplate()
		template.Style = style
		report, err := emailTemplateDemo(template)
		if err != nil {
			t.Fatal(err)
		}
		// This contract covers the layout without configured project groups.
		report.YesterdayGroups, report.UnresolvedGroups, report.ProjectGroupStats = nil, nil, nil
		if err := renderEmailReport(&report); err != nil {
			t.Fatal(err)
		}
		if report.Style != style || !strings.Contains(report.HTML, `data-email-style="`+style+`"`) {
			t.Fatalf("wrong style %s", style)
		}
		for _, fact := range []string{"DEMO-101", "DEMO-102", "DEMO-103", "示例负责人甲", "Jira 图表分析", "昨日采集 commit"} {
			if !strings.Contains(report.HTML, fact) {
				t.Errorf("%s missing %s", style, fact)
			}
		}
		if strings.Contains(report.HTML, "数据口径与生成时间") {
			t.Errorf("%s should not contain 数据口径与生成时间", style)
		}
		for _, unsafe := range []string{"<script", "<img src=x", "<iframe", "<link", "<object", "javascript:", "onload=", "onerror="} {
			if strings.Contains(strings.ToLower(report.HTML), unsafe) {
				t.Errorf("%s contains unsafe markup %s", style, unsafe)
			}
		}
		if !strings.Contains(report.HTML, `width="100%"`) || !strings.Contains(report.HTML, "table-layout:fixed") || !strings.Contains(report.HTML, "word-break:break-word") {
			t.Errorf("%s lost narrow-screen email structure", style)
		}
		rendered[style] = report.HTML
	}
	if strings.Index(rendered["brief"], "Jira 图表分析") > strings.Index(rendered["brief"], "昨日更新 Jira") {
		t.Fatal("brief must show analysis first")
	}
	if strings.Index(rendered["focus"], "先看待办") > strings.Index(rendered["focus"], "Coremember 负责人") || strings.Index(rendered["focus"], "Coremember 负责人") > strings.Index(rendered["focus"], "Jira 图表分析") {
		t.Fatal("focus must show unresolved work and owners first")
	}
	if strings.Index(rendered["ledger"], "明细台账") > strings.Index(rendered["ledger"], "Jira 图表分析") || !strings.Contains(rendered["ledger"], `scope="col">范围`) {
		t.Fatal("ledger must show compact range table first")
	}
	if strings.Index(rendered["hyperframe"], "解决率指标") > strings.Index(rendered["hyperframe"], "昨日更新 Jira") {
		t.Fatal("hyperframe must show top charts before issue list")
	}
	if rendered["brief"] == rendered["focus"] || rendered["focus"] == rendered["ledger"] || rendered["ledger"] == rendered["hyperframe"] {
		t.Fatal("layouts are identical")
	}
}

func TestEmailLayoutHeadersExposeDifferentFirstScreenPriorities(t *testing.T) {
	for _, style := range []string{"brief", "focus", "ledger"} {
		tmpl := config.DefaultEmailTemplate()
		tmpl.Style = style
		tmpl.Introduction = "INTRODUCTION_SENTINEL"
		report, err := emailTemplateDemo(tmpl)
		if err != nil {
			t.Fatal(err)
		}
		report.Warnings = []string{"WARNING_SENTINEL"}
		if err := renderEmailReport(&report); err != nil {
			t.Fatal(err)
		}
		if strings.Count(report.HTML, "INTRODUCTION_SENTINEL") != 1 || strings.Count(report.HTML, "WARNING_SENTINEL") != 1 {
			t.Fatalf("%s lost or duplicated introductory content", style)
		}
		title := strings.Index(report.HTML, "<h1")
		brand := strings.Index(report.HTML, "well-ambient")
		intro := strings.Index(report.HTML, "INTRODUCTION_SENTINEL")
		warning := strings.Index(report.HTML, "WARNING_SENTINEL")
		firstIssue := strings.Index(report.HTML, "DEMO-101")
		switch style {
		case "brief":
			if brand > title || intro > firstIssue || !strings.Contains(report.HTML, "font-size:24px") {
				t.Fatal("brief lost standard introduction hierarchy")
			}
		case "focus":
			if intro > strings.Index(report.HTML, "先看待办") || strings.Index(report.HTML, "先看待办") > warning || warning > firstIssue {
				t.Fatal("focus must greet the reader before action summary and priority issues")
			}
			if strings.Contains(report.HTML, "border-left:") {
				t.Fatal("focus reintroduced thick side accent")
			}
		case "ledger":
			if title > brand || !strings.Contains(report.HTML, `<h1 style="font-size:18px;`) || !strings.Contains(report.HTML, "昨日更新 3 项 / 近3天未解决 2 项") {
				t.Fatal("ledger lost compact heading and inline scope summary")
			}
			if strings.Index(report.HTML, `scope="col">范围`) > strings.Index(report.HTML, "Jira 图表分析") {
				t.Fatal("ledger table header moved below analysis")
			}
		}
	}
}

func TestEmailLayoutsEscapeAllUntrustedContentAndRejectUnknownStyles(t *testing.T) {
	for _, style := range []string{"brief", "focus", "ledger", "hyperframe"} {
		report := emailReport{Style: style, Subject: `<script>alert(1)</script>`, Introduction: `<img src=x onerror=alert(2)>`, Closing: `<a href="javascript:evil()">click</a>`, YesterdayUpdated: []emailIssue{{TaskID: "DEMO-1", Title: `<iframe src="https://evil.test">`, Assignee: `<svg onload=evil()>`, Status: `<script>status</script>`}}, RecentUnresolved: []emailIssue{{Title: `<img src=x>`, Assignee: `<script>owner</script>`}}, CoreMembers: []emailCoreMember{{Name: `<script>member</script>`, UnresolvedCount: 1}}, Commits: &emailCommitSummary{Authors: map[string]int{`<img src=x onerror=evil()>`: 1}}}
		if err := renderEmailReport(&report); err != nil {
			t.Fatal(err)
		}
		for _, unsafe := range []string{"<script", "<img src=x", "<iframe", "<a href=", "<svg onload"} {
			if strings.Contains(report.HTML, unsafe) {
				t.Fatalf("%s contains unescaped %s", style, unsafe)
			}
		}
		if !strings.Contains(report.HTML, "&lt;script&gt;") {
			t.Fatalf("%s removed rather than escaped content", style)
		}
	}
	legacy := emailReport{}
	if err := renderEmailReport(&legacy); err != nil || legacy.Style != "brief" {
		t.Fatalf("legacy renderer default: %s %v", legacy.Style, err)
	}
	invalid := emailReport{Style: `brief" onload="evil()`}
	if err := renderEmailReport(&invalid); err == nil {
		t.Fatal("uncontrolled style accepted")
	}
}

func TestEmailPreviewAndSendShareRendererForEveryStyle(t *testing.T) {
	emailTestDatabase(t)
	for index, style := range []string{"brief", "focus", "ledger", "hyperframe"} {
		cfg := emailTestConfig()
		cfg.DailyJiraEmail.Template = config.DefaultEmailTemplate()
		cfg.DailyJiraEmail.Template.Style = style
		s := &Server{config: &cfg}
		now := time.Date(2026, 1, 15+index, 9, 0, 0, 0, time.UTC)
		date := now.Format("2006-01-02")
		preview, err := s.buildEmailReport(context.Background(), cfg, cfg.DailyJiraEmail, date, now)
		if err != nil {
			t.Fatal(err)
		}
		calls := 0
		s.emailSender = func(_ context.Context, _ config.SMTPConfig, _ []string, subject, text, html string) error {
			calls++
			if subject != preview.Subject || text != preview.Text || html != preview.HTML {
				t.Errorf("%s preview and actual send body diverged", style)
			}
			return nil
		}
		if _, err := s.sendDailyEmail(context.Background(), cfg, cfg.DailyJiraEmail, date, "manual", now); err != nil {
			t.Fatal(err)
		}
		if calls != 1 {
			t.Fatalf("%s sender calls=%d", style, calls)
		}
	}
}
