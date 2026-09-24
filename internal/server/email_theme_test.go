package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/config"
)

func TestEmailChartsDoNotInventHistoryOrEmptyCompletion(t *testing.T) {
	report := emailReport{}
	if err := renderEmailReport(&report); err != nil {
		t.Fatal(err)
	}
	if report.ResolutionRate != 0 || strings.Contains(string(report.DonutChartSVG), ">100%<") {
		t.Fatal("empty denominator presented as complete")
	}
	if !strings.Contains(string(report.DonutChartSVG), "—") {
		t.Fatal("missing empty completion state")
	}
	for _, points := range [][]int{nil, {2}, {2, 4, 3}} {
		report.TrendPoints = points
		if err := renderEmailReport(&report); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(report.CurveChartSVG), "data:image/png;base64,") || !strings.Contains(report.CurveChartDesc, "暂无完整七日数据") {
			t.Fatal("missing history manufactured a curve")
		}
	}
	points := []int{4, 6, 5, 8, 7, 5, 3}
	svg, _ := renderCurveChartImage(points, nil)
	if !strings.Contains(string(svg), `class="email-trend-image"`) || strings.Count(string(svg), `<td style="padding:0;word-break:normal">`) != 7 {
		t.Fatal("valid history did not retain all seven points")
	}
}

func TestEmailThemeCoversMediaAndOutlookHooks(t *testing.T) {
	report, err := emailTemplateDemo(config.EmailTemplate{Style: "hyperframe", Subject: "主题"})
	if err != nil {
		t.Fatal(err)
	}
	for _, selector := range []string{".email-focus-box", ".email-card-inner", ".email-value-green", ".chart-empty-surface", ".chart-text-highlight", ".chart-series-0"} {
		if !strings.Contains(report.HTML, "[data-ogsc] "+selector) || !strings.Contains(report.HTML, "[data-ogsb] "+selector) {
			t.Fatalf("missing parallel theme rule: %s", selector)
		}
	}
	if !strings.Contains(report.HTML, "prefers-color-scheme:dark") || !strings.Contains(report.HTML, `name="color-scheme" content="light dark"`) {
		t.Fatal("missing media theme metadata")
	}
	if !strings.Contains(report.HTML, `class="email-value-green"`) {
		t.Fatal("un-themed metric text")
	}
	single, _ := renderPieChartImage(map[string]int{"done": 1})
	if !strings.Contains(string(single), `class="email-status-image"`) {
		t.Fatal("single-series chart must remain a real PNG")
	}
	empty, _ := renderPieChartImage(nil)
	if !strings.Contains(string(empty), `alt="昨日更新状态分布，共 0 项`) {
		t.Fatal("empty chart must expose its zero total")
	}
}

func TestEmailZeroCommitsDoesNotReintroduceRemovedWarning(t *testing.T) {
	emailTestDatabase(t)
	cfg := emailTestConfig()
	cfg.DailyJiraEmail.IncludeCommits = true
	report, err := (&Server{config: &cfg}).buildEmailReport(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if report.Commits == nil || report.Commits.Count != 0 {
		t.Fatal("zero commit facts were hidden")
	}
	removed := "本地没有匹配的昨日 commit 采集记录"
	if strings.Contains(report.HTML, removed) || strings.Contains(report.Text, removed) || strings.Contains(strings.Join(report.Warnings, " "), removed) {
		t.Fatal("removed warning returned")
	}
	if len(report.Warnings) == 0 {
		t.Fatal("unrelated Jira disabled warning was hidden")
	}
}

// Opt-in artifacts complement the complete built-in previews with edge states.
func TestEmailThemeExportEdgePreviews(t *testing.T) {
	if os.Getenv("WELL_EMAIL_THEME_EXPORT") != "1" {
		t.Skip("explicit browser artifacts only")
	}
	root := filepath.Join("..", "..", "outputs")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	cases := map[string]emailReport{
		"empty":  {Style: "hyperframe", Subject: "空数据示例", Date: "2026-01-15", Timezone: "Asia/Shanghai"},
		"single": {Style: "brief", Subject: "单状态示例", Date: "2026-01-15", Timezone: "Asia/Shanghai", YesterdayUpdated: []emailIssue{{TaskID: "DEMO-1", Title: "示例事项", Status: "done", Assignee: "示例负责人"}}, CoreMembers: []emailCoreMember{{Name: "示例负责人", YesterdayCount: 1}}, Commits: &emailCommitSummary{Authors: map[string]int{}, Analysis: "昨日采集 0 条提交。"}},
	}
	for name, report := range cases {
		if err := renderEmailReport(&report); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "email-theme-"+name+".html"), []byte(report.HTML), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
