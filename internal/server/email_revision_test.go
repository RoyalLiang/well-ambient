package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestEmailRevisionChartsPrecedeOverviewAndDetails(t *testing.T) {
	for _, style := range []string{"brief", "focus", "ledger", "hyperframe"} {
		for _, state := range []string{"plain", "grouped", "empty"} {
			tmpl := config.DefaultEmailTemplate()
			tmpl.Style = style
			tmpl.Introduction = "GREETING_BEFORE_CHARTS"
			report, err := emailTemplateDemo(tmpl)
			if err != nil {
				t.Fatal(err)
			}
			report.YesterdayGroups, report.UnresolvedGroups, report.ProjectGroupStats = nil, nil, nil
			if state == "empty" {
				report.YesterdayUpdated = nil
				report.RecentUnresolved = nil
			}
			if state == "grouped" {
				groups := []config.DailyJiraProjectGroup{{Name: "Alpha team", Projects: []string{"DEMO"}}}
				report.YesterdayGroups, report.YesterdayUngrouped = groupIssues(report.YesterdayUpdated, groups)
				report.UnresolvedGroups, report.UnresolvedUngrouped = groupIssues(report.RecentUnresolved, groups)
			}
			if err := renderEmailReport(&report); err != nil {
				t.Fatal(err)
			}
			endTitle := strings.Index(report.HTML, "</h1>")
			greeting := strings.Index(report.HTML, "GREETING_BEFORE_CHARTS")
			if greeting <= endTitle || strings.Count(report.HTML, "GREETING_BEFORE_CHARTS") != 1 {
				t.Errorf("%s/%s: greeting must appear once after the title", style, state)
			}
			lastChart := -1
			for _, label := range []string{"解决率指标", "状态分布", "近7日更新分布"} {
				pos := strings.Index(report.HTML, ">"+label+"</div>")
				if pos <= greeting || pos <= lastChart || strings.Count(report.HTML, ">"+label+"</div>") != 1 {
					t.Errorf("%s/%s: chart %s missing, duplicated or before greeting", style, state, label)
				}
				lastChart = pos
			}
			for _, marker := range []string{`<h2 class="email-heading-h2"`, `class="email-card email-group-summary"`, `class="email-card email-category-cell"`, "DEMO-101", `class="email-value-blue"`} {
				if pos := strings.Index(report.HTML, marker); pos >= 0 && pos < lastChart {
					t.Errorf("%s/%s: %s precedes charts", style, state, marker)
				}
			}
			if strings.Contains(report.HTML+report.Text, "项目负责人分组统计") {
				t.Errorf("%s/%s: obsolete group statistics remain", style, state)
			}
			if state == "grouped" && (!strings.Contains(report.HTML, `class="email-card email-group-summary"`) || !strings.Contains(report.Text, "项目分组概览") || !strings.Contains(report.Text, "【Alpha team")) {
				t.Errorf("%s: lost group overview or details", style)
			}
		}
	}
}

func TestEmailRevisionJiraLinksUseConfiguredBaseURL(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	cfg.Jira.BaseURL = "https://jira.example.test/team/"
	today, yesterday, _, _ := emailDateBounds("2026-03-09", cfg.DailyJiraEmail.Timezone, time.Time{})
	for _, key := range []string{"ABC-1", "OTHER-2"} {
		if err := conn.Create(&db.TaskTelemetry{TaskID: key, Title: "Linked issue", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Assignee: "alice", Status: "progress", TaskCreatedAt: yesterday, SourceUpdatedAt: yesterday}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, style := range []string{"brief", "focus", "ledger", "hyperframe"} {
		for _, grouped := range []bool{false, true} {
			settings := cfg.DailyJiraEmail
			settings.Template.Style = style
			if grouped {
				settings.ProjectGroups = []config.DailyJiraProjectGroup{{Name: "Alpha team", Projects: []string{"ABC"}}}
			}
			report, err := (&Server{}).buildEmailReport(context.Background(), cfg, settings, "2026-03-09", today)
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"ABC-1", "OTHER-2"} {
				url := "https://jira.example.test/team/browse/" + key
				if strings.Count(report.Text, url) != 2 {
					t.Errorf("%s grouped=%v: plain text missing URL for %s in yesterday and unresolved sections", style, grouped, key)
				}
				want := `href="` + url + `"`
				if strings.Count(report.HTML, want) != 2 {
					t.Errorf("%s grouped=%v: expected linked %s in yesterday and unresolved sections", style, grouped, key)
				}
			}
		}
	}
}

func TestEmailRevisionJiraURLSafetyAndEscaping(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	today, yesterday, _, _ := emailDateBounds("2026-03-09", cfg.DailyJiraEmail.Timezone, time.Time{})
	if err := conn.Create(&db.TaskTelemetry{TaskID: "ABC-1", Title: `<script>alert("title")</script>`, Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Assignee: "alice", Status: "progress", TaskCreatedAt: yesterday, SourceUpdatedAt: yesterday}).Error; err != nil {
		t.Fatal(err)
	}
	for _, style := range []string{"brief", "focus", "ledger", "hyperframe"} {
		for _, base := range []string{"", "javascript:alert(1)", "data:text/html,evil", "//jira.example.test", "/jira", "https://", "https://bad host/", "https://user:password@jira.example.test", "https://jira.example.test?redirect=evil", "https://jira.example.test#evil", "https://jira.example.test?", "https://jira.example.test#"} {
			cfg.Jira.BaseURL = base
			cfg.DailyJiraEmail.Template.Style = style
			report, err := (&Server{}).buildEmailReport(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", today)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(report.Text, "/browse/") || strings.Contains(report.Text, "jira.example.test") || strings.Count(report.Text, "ABC-1 | [FMS] <script>alert(\"title\")</script> | alice | progress\n") != 2 {
				t.Errorf("%s: plain text fallback changed or leaked invalid base %q", style, base)
			}
			if strings.Contains(report.HTML, `class="email-jira-link"`) || !strings.Contains(report.HTML, "ABC-1") || strings.Contains(report.HTML, "<script>") || strings.Contains(report.HTML, "ZgotmplZ") {
				t.Errorf("%s: unsafe or broken fallback for %q", style, base)
			}
		}
		cfg.Jira.BaseURL = "http://jira.example.test:8080/jira/"
		report, err := (&Server{}).buildEmailReport(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", today)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(report.HTML, `href="http://jira.example.test:8080/jira/browse/ABC-1" target="_blank" rel="noopener noreferrer"`) {
			t.Errorf("%s: HTTP context-path link missing safe target", style)
		}
		issue := emailIssue{TaskID: `ABC-1/<img src=x>`, URL: (&Server{config: &cfg}).emailIssuePageURL(`ABC-1/<img src=x>`), Title: `<script>title</script>`}
		escaped := emailReport{Style: style, YesterdayUpdated: []emailIssue{issue}, RecentUnresolved: []emailIssue{issue}}
		if err := renderEmailReport(&escaped); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(escaped.HTML, "<img src=x") || strings.Contains(escaped.HTML, "<script") || !strings.Contains(escaped.HTML, "ABC-1%2F%3Cimg%20src=x%3E") || !strings.Contains(escaped.HTML, "ABC-1/&lt;img src=x&gt;") {
			t.Errorf("%s: issue key or title was not escaped safely", style)
		}
	}
}

// Opt-in export for the parent agent's browser validation; ordinary tests do not write artifacts.
func TestEmailRevisionExportBrowserHTML(t *testing.T) {
	dir := os.Getenv("WELL_EMAIL_REVISION_OUTPUT")
	if dir == "" {
		t.Skip("set WELL_EMAIL_REVISION_OUTPUT to export browser fixtures")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, style := range []string{"brief", "focus", "ledger", "hyperframe"} {
		for _, empty := range []bool{false, true} {
			tmpl := config.DefaultEmailTemplate()
			tmpl.Style = style
			report, err := emailTemplateDemo(tmpl)
			if err != nil {
				t.Fatal(err)
			}
			for i := range report.YesterdayUpdated {
				report.YesterdayUpdated[i].URL = "https://jira.example.test/jira/browse/" + report.YesterdayUpdated[i].TaskID
			}
			for i := range report.RecentUnresolved {
				report.RecentUnresolved[i].URL = "https://jira.example.test/jira/browse/" + report.RecentUnresolved[i].TaskID
			}
			suffix := ""
			if empty {
				report.YesterdayUpdated = nil
				report.RecentUnresolved = nil
				report.TrendPoints = make([]int, 7)
				suffix = "-empty"
			}
			groups := []config.DailyJiraProjectGroup{{Name: "邮件验证组", Projects: []string{"DEMO"}, Owners: []string{"示例负责人甲", "示例负责人乙"}}}
			report.YesterdayGroups, report.YesterdayUngrouped = groupIssues(report.YesterdayUpdated, groups)
			report.UnresolvedGroups, report.UnresolvedUngrouped = groupIssues(report.RecentUnresolved, groups)
			report.ProjectGroupStats = nil
			if err := renderEmailReport(&report); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "email-revision-"+style+suffix+".html"), []byte(report.HTML), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
