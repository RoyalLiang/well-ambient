package server

import (
	"fmt"
	"strings"
	"testing"
)

func TestEmailChartsTotalsTopOtherAndZero(t *testing.T) {
	empty := makeEmailChart("empty", "", map[string]int{"zero": 0})
	if empty.Total != 0 || len(empty.Bars) != 0 {
		t.Fatalf("zero chart: %+v", empty)
	}
	counts := map[string]int{}
	for i := 0; i < 12; i++ {
		counts[fmt.Sprintf("owner-%02d", i)] = i + 1
	}
	chart := makeEmailChart("owners", "", counts)
	sum := 0
	for _, bar := range chart.Bars {
		sum += bar.Count
		if bar.Percent < 0 || bar.Percent > 100 {
			t.Fatal("invalid percentage")
		}
	}
	if chart.Total != 78 || sum != 78 || len(chart.Bars) != 9 || chart.Bars[8].Label != "其他（合计）" {
		t.Fatalf("chart dropped facts: %+v", chart)
	}
	single := makeEmailChart("single", "", map[string]int{"Owner": 7})
	if len(single.Bars) != 1 || single.Bars[0].Percent != 100 {
		t.Fatalf("single chart: %+v", single)
	}
}
func TestEmailChartsSafeHTMLAndTextFallback(t *testing.T) {
	report := emailReport{Subject: "早报", YesterdayUpdated: []emailIssue{{TaskID: "X-1", Category: "FMS", Status: "done"}, {TaskID: "X-2", Category: "FMS", Status: "progress"}}, RecentUnresolved: []emailIssue{{TaskID: "X-2", Category: "FMS", Status: "progress"}}, CoreMembers: []emailCoreMember{{Name: "<img src=x onerror=alert(1)>", UnresolvedCount: 1}}, Commits: &emailCommitSummary{Count: 1, Authors: map[string]int{"Alice": 1}}}
	if err := renderEmailReport(&report); err != nil {
		t.Fatal(err)
	}
	if len(report.JiraCharts) != 2 || report.JiraCharts[0].Total != 2 || report.JiraCharts[1].Total != 1 || report.CommitChart == nil || report.CommitChart.Total != 1 {
		t.Fatal("chart/source mismatch")
	}
	for _, bad := range []string{"<script", "<img src=x", "ZgotmplZ", "<canvas", "NaN", "Inf%"} {
		if strings.Contains(report.HTML, bad) {
			t.Fatalf("unsafe or invalid chart token %s", bad)
		}
	}
	if !strings.Contains(report.HTML, "width:50%") || !strings.Contains(report.HTML, "width:100%") || !strings.Contains(report.HTML, "&lt;img") {
		t.Fatal("missing safe bars")
	}
	if !strings.Contains(report.Text, "Jira 图表分析") || !strings.Contains(report.Text, "done：1（50%）") {
		t.Fatal("text fallback lacks chart facts")
	}
	report.Commits = nil
	report.YesterdayUpdated = nil
	report.RecentUnresolved = nil
	report.CoreMembers = nil
	if err := renderEmailReport(&report); err != nil {
		t.Fatal(err)
	}
	if report.CommitChart != nil || strings.Contains(report.HTML, "昨日采集 commit · 作者分布") || strings.Contains(report.HTML, "width:0%") {
		t.Fatal("empty/disabled chart rendered a misleading bar")
	}
	if !strings.Contains(report.HTML, "暂无匹配记录") {
		t.Fatal("empty chart needs explanation")
	}
}

func TestEmailChartsPNGDimensionsAndHTMLBars(t *testing.T) {
	donut, _ := renderDonutChartImage(3, 4)
	pie, _ := renderPieChartImage(map[string]int{"done": 5})
	curve, _ := renderCurveChartImage([]int{1, 2, 3, 4, 5, 6, 7}, []string{"-7d", "-6d", "-5d", "-4d", "-3d", "-2d", "-1d"})
	for _, image := range []struct{ markup, width, height string }{
		{string(donut), `width="96"`, `height="96"`},
		{string(pie), `width="96"`, `height="96"`},
		{string(curve), `width="160"`, `height="90"`},
	} {
		if !strings.Contains(image.markup, "data:image/png;base64,") || !strings.Contains(image.markup, image.width) || !strings.Contains(image.markup, image.height) || strings.Contains(image.markup, "<svg") {
			t.Fatal("chart must use dimensioned PNG rather than SVG")
		}
	}

	// 4. HTML fallback bars
	report := emailReport{
		Date:             "2026-03-09",
		Subject:          "早报",
		YesterdayUpdated: []emailIssue{{TaskID: "X-1", Category: "FMS", Status: "done"}, {TaskID: "X-2", Category: "FMS", Status: "progress"}, {TaskID: "X-3", Status: "progress"}},
		RecentUnresolved: []emailIssue{{TaskID: "X-2", Category: "FMS", Status: "progress"}},
		TrendPoints:      []int{2, 3, 1, 5, 4, 2, 3},
	}
	prepareEmailCharts(&report)
	if len(report.StatusBars) != 2 {
		t.Fatalf("expected 2 status bars, got %d", len(report.StatusBars))
	}
	if len(report.TrendBars) != 7 {
		t.Fatalf("expected 7 trend bars, got %d", len(report.TrendBars))
	}
	if !report.TrendBars[6].IsLast {
		t.Fatalf("last trend bar should have IsLast=true")
	}

	if err := renderEmailReport(&report); err != nil {
		t.Fatal(err)
	}
	// HTML should include the fallback progress bar and trend bars
	if !strings.Contains(report.HTML, "email-progress-bg") {
		t.Fatal("HTML should contain status progress bar")
	}
	if !strings.Contains(report.HTML, "昨日") {
		t.Fatal("HTML should contain trend bar label")
	}
}

func TestEmailProjectGroupChartsAndAnalysis(t *testing.T) {
	report := emailReport{
		Date:    "2026-03-09",
		Subject: "早报",
		Style:   "brief",
		YesterdayGroups: []emailIssueGroup{
			{Name: "前端组", Count: 3, Owners: []string{"Alice"}},
			{Name: "后端组", Count: 5, Owners: []string{"Bob"}},
		},
		UnresolvedGroups: []emailIssueGroup{
			{Name: "前端组", Count: 2, Owners: []string{"Alice"}},
			{Name: "后端组", Count: 4, Owners: []string{"Bob"}},
		},
		YesterdayUpdated: []emailIssue{
			{TaskID: "FE-1", Title: "UI fix", Assignee: "Alice", GroupName: "前端组"},
			{TaskID: "BE-1", Title: "API fix", Assignee: "Bob", GroupName: "后端组"},
		},
		RecentUnresolved: []emailIssue{
			{TaskID: "FE-1", Title: "UI fix", Assignee: "Alice", GroupName: "前端组"},
			{TaskID: "BE-1", Title: "API fix", Assignee: "Bob", GroupName: "后端组"},
		},
	}

	prepareEmailCharts(&report)

	// Should have group charts prepended
	if len(report.JiraCharts) < 2 {
		t.Fatalf("expected at least 2 JiraCharts with group charts, got %d", len(report.JiraCharts))
	}
	hasUnresolvedGroupChart := false
	hasYesterdayGroupChart := false
	for _, c := range report.JiraCharts {
		if strings.Contains(c.Title, "项目分组 · 近 3 天未解决分布") {
			hasUnresolvedGroupChart = true
			if !strings.Contains(string(c.SVG), "data:image/png;base64,") || !strings.Contains(string(c.SVG), "前端组") {
				t.Fatalf("unresolved group chart image should contain embedded PNG and 前端组, got %s", string(c.SVG))
			}
			if !strings.Contains(string(c.SVG), `width="100%"`) || strings.Contains(string(c.SVG), "max-width:540px") {
				t.Fatalf("group chart image must use full-width image layout without 540 max-width: %s", string(c.SVG))
			}
			if !strings.Contains(string(c.SVG), `font-size:13px`) {
				t.Fatalf("group chart image must use crisp 13px label font: %s", string(c.SVG))
			}
		}
		if strings.Contains(c.Title, "项目分组 · 昨日更新分布") {
			hasYesterdayGroupChart = true
			if !strings.Contains(string(c.SVG), "data:image/png;base64,") || !strings.Contains(string(c.SVG), "后端组") {
				t.Fatalf("yesterday group chart image should contain embedded PNG and 后端组, got %s", string(c.SVG))
			}
			if !strings.Contains(string(c.SVG), `width="100%"`) || strings.Contains(string(c.SVG), "max-width:540px") {
				t.Fatalf("yesterday group chart image must use full-width image layout without 540 max-width: %s", string(c.SVG))
			}
		}
	}
	if !hasUnresolvedGroupChart || !hasYesterdayGroupChart {
		t.Fatalf("expected both unresolved and yesterday group charts, got hasUnres=%v hasYest=%v", hasUnresolvedGroupChart, hasYesterdayGroupChart)
	}

	// Analysis text should include group statistics in Go struct
	hasGroupAnalysis := false
	for _, a := range report.Analysis {
		if strings.Contains(a, "项目分组统计") && strings.Contains(a, "后端组") {
			hasGroupAnalysis = true
		}
	}
	if !hasGroupAnalysis {
		t.Fatalf("expected group analysis in report.Analysis, got %v", report.Analysis)
	}

	if err := renderEmailReport(&report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.HTML, "项目分组 · 近 3 天未解决分布") {
		t.Fatal("rendered HTML should contain group chart title")
	}
	// HTML template should not contain the large paragraph analysis box above charts
	if strings.Contains(report.HTML, "大盘态势：") || strings.Contains(report.HTML, "重点跟进建议：") {
		t.Fatal("rendered HTML should not contain large paragraph analysis card above charts")
	}
}
