package server

import (
	"fmt"
	"html"
	"html/template"
	"sort"
	"strings"
)

type emailChartBar struct {
	Label   string `json:"label"`
	Count   int    `json:"count"`
	Percent int    `json:"percent"`
}
type emailChart struct {
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Total       int             `json:"total"`
	Bars        []emailChartBar `json:"bars"`
	SVG         template.HTML   `json:"svg,omitempty"`
}

var chartPaletteLight = []string{
	"#008f96", "#2563eb", "#d97706", "#7c3aed", "#059669", "#db2777", "#64748b",
}

func renderEmailProgress(rate, total int) template.HTML {
	rate = max(0, min(100, rate))
	label := fmt.Sprintf("%d%%", rate)
	if total <= 0 {
		rate, label = 0, "—"
	}
	var out strings.Builder
	fmt.Fprintf(&out, `<table class="email-progress-meter" role="img" aria-label="解决率 %s" width="100%%" cellpadding="0" cellspacing="0" style="width:100%%;table-layout:fixed;border-spacing:0;border-radius:6px;overflow:hidden;margin-top:12px"><tr>`, label)
	if rate > 0 {
		text := "&nbsp;"
		if rate >= 50 {
			text = label
		}
		fmt.Fprintf(&out, `<td width="%d%%" class="email-progress-fill" style="width:%d%%;height:24px;padding:0;background-color:#0f766e;background-image:linear-gradient(90deg,#115e59,#0f766e);color:#ffffff;text-align:center;font-size:12px;font-weight:700;line-height:24px">%s</td>`, rate, rate, text)
	}
	if rate < 100 {
		text := "&nbsp;"
		if rate < 50 {
			text = label
		}
		fmt.Fprintf(&out, `<td width="%d%%" class="email-progress-bg email-text-main" style="width:%d%%;height:24px;padding:0;background-color:#e2e8f0;color:#334155;text-align:center;font-size:12px;font-weight:700;line-height:24px">%s</td>`, 100-rate, 100-rate, text)
	}
	out.WriteString(`</tr></table>`)
	return template.HTML(out.String())
}

// Labels stay in HTML: they remain readable on phones and when images are blocked.
func renderEmailCommitChart(chart *emailChart, summary *emailCommitSummary) template.HTML {
	if chart.Total <= 0 || len(chart.Bars) == 0 {
		message := "暂无采集到的提交。"
		if summary.Count > 0 {
			message = "暂无作者分布，已采集提交总数见上方。"
			return template.HTML(`<p class="email-text-muted" style="margin:12px 0;font-size:13px;line-height:1.6;color:#536479">` + message + `</p>`)
		}
		var out strings.Builder
		out.WriteString(`<div class="email-commit-zero-chart">`)
		for _, metric := range []struct {
			label, unit string
			count       int
		}{{"采集提交", "次", summary.Count}, {"涉及仓库", "个", summary.Repositories}, {"贡献者", "人", summary.Contributors}} {
			label := fmt.Sprintf("%s：%d %s", metric.label, metric.count, metric.unit)
			fmt.Fprintf(&out, `<div style="margin:12px 0"><div class="email-text-main" style="font-size:13px;color:#1e293b;margin-bottom:6px">%s</div>%s</div>`,
				label, emailProportionImage(metric.count, max(1, metric.count), label, "email-commit-zero-bar"))
		}
		out.WriteString(`<p class="email-text-muted" style="margin:12px 0;font-size:13px;line-height:1.6;color:#536479">` + message + `</p></div>`)
		return template.HTML(out.String())
	}
	return renderEmailBarRows(chart, "次", "email-commit")
}

func renderBarChartImage(chart *emailChart) template.HTML {
	if chart.Total <= 0 || len(chart.Bars) == 0 {
		return template.HTML(`<p class="email-text-muted" style="font-size:13px;color:#536479">暂无匹配记录。</p>`)
	}
	return renderEmailBarRows(chart, "项", "email-distribution")
}

func renderEmailBarRows(chart *emailChart, unit, class string) template.HTML {
	var out strings.Builder
	fmt.Fprintf(&out, `<div class="%s-chart">`, class)
	for _, bar := range chart.Bars {
		share := float64(bar.Count) / float64(chart.Total)
		percent := fmt.Sprintf("%.1f%%", share*100)
		if share > 0 && share < 0.001 {
			percent = "<0.1%"
		}
		label := strings.TrimSpace(bar.Label)
		if label == "" {
			label = "未标注作者"
		}
		accessible := fmt.Sprintf("%s：%d %s，占 %s", label, bar.Count, unit, percent)
		graphic := emailProportionImage(bar.Count, chart.Total, accessible, class+"-bar")
		fmt.Fprintf(&out, `<table class="%s-row" role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="width:100%%;table-layout:fixed;margin:14px 0"><tr><td class="email-text-main" style="font-size:13px;font-weight:600;line-height:1.6;color:#1e293b;word-break:break-word;overflow-wrap:anywhere;padding-right:12px">%s</td><td class="email-text-muted" width="112" style="width:112px;text-align:right;font-size:12px;line-height:1.6;color:#536479;font-variant-numeric:tabular-nums">%d %s · %s</td></tr><tr><td colspan="2" style="padding-top:6px">%s</td></tr></table>`, class, html.EscapeString(label), bar.Count, unit, html.EscapeString(percent), graphic)
	}
	out.WriteString(`</div>`)
	return template.HTML(out.String())
}

// Charts remain data, never AI-generated HTML or executable chart scripts.
func makeEmailChart(title, description string, counts map[string]int) emailChart {
	chart := makeEmailChartData(title, description, counts)
	chart.SVG = renderBarChartImage(&chart)
	return chart
}

func makeEmailChartData(title, description string, counts map[string]int) emailChart {
	chart := emailChart{Title: title, Description: description, Bars: []emailChartBar{}}
	for label, count := range counts {
		if count > 0 {
			chart.Total += count
			chart.Bars = append(chart.Bars, emailChartBar{Label: label, Count: count})
		}
	}
	sort.Slice(chart.Bars, func(i, j int) bool {
		if chart.Bars[i].Count == chart.Bars[j].Count {
			return chart.Bars[i].Label < chart.Bars[j].Label
		}
		return chart.Bars[i].Count > chart.Bars[j].Count
	})
	if len(chart.Bars) > 8 {
		other := 0
		for _, bar := range chart.Bars[8:] {
			other += bar.Count
		}
		chart.Bars = append(chart.Bars[:8], emailChartBar{Label: "其他（合计）", Count: other})
	}
	for i := range chart.Bars {
		chart.Bars[i].Percent = (chart.Bars[i].Count*100 + chart.Total/2) / chart.Total
	}
	return chart
}

func prepareEmailCharts(report *emailReport) {
	if report.ActivityChart != nil {
		report.ActivityChart.SVG = renderEmailActivityBars(*report.ActivityChart)
	}
	statuses := map[string]int{}
	resolved := 0
	for _, issue := range report.YesterdayUpdated {
		status := issue.Status
		if status == "" {
			status = "未标注状态"
		}
		statuses[status]++
		if isResolvedDailyJiraStatus(issue.Status) {
			resolved++
		}
	}
	report.ResolvedCount = resolved
	if len(report.YesterdayUpdated) > 0 {
		report.ResolutionRate = (resolved * 100) / len(report.YesterdayUpdated)
	} else {
		report.ResolutionRate = 0
	}

	// Top charts use email-safe PNGs and HTML labels.
	donutSVG, donutDesc := renderDonutChartImage(resolved, len(report.YesterdayUpdated))
	report.DonutChartSVG = donutSVG
	report.DonutChartDesc = donutDesc

	// All status counts remain visible beside the image.
	pieSVG, pieLegend := renderPieChartImage(statuses)
	report.PieChartSVG = pieSVG
	report.PieChartLegend = pieLegend

	// Trend dates and values remain HTML rather than image-only text.
	trend := report.TrendPoints
	curveLabels := []string{"-7d", "-6d", "-5d", "-4d", "-3d", "-2d", "-1d"}
	curveSVG, curveDesc := renderCurveChartImage(trend, curveLabels)
	report.CurveChartSVG = curveSVG
	report.CurveChartDesc = curveDesc
	if report.TrendUnavailable != "" {
		report.CurveChartDesc = report.TrendUnavailable
	}

	// Prepare status bars for HTML table fallback
	report.StatusBars = nil
	if len(report.YesterdayUpdated) > 0 {
		var statusList []emailStatusBar
		totalStatus := len(report.YesterdayUpdated)
		statusColorMap := map[string]string{
			"done":        "#059669",
			"progress":    "#0284c7",
			"review":      "#d97706",
			"todo":        "#64748b",
			"in progress": "#0284c7",
			"closed":      "#059669",
			"resolved":    "#059669",
		}
		type stEntry struct {
			name  string
			count int
		}
		var sortedSt []stEntry
		for st, c := range statuses {
			sortedSt = append(sortedSt, stEntry{st, c})
		}
		sort.Slice(sortedSt, func(i, j int) bool {
			if sortedSt[i].count == sortedSt[j].count {
				return sortedSt[i].name < sortedSt[j].name
			}
			return sortedSt[i].count > sortedSt[j].count
		})

		for i, st := range sortedSt {
			color, ok := statusColorMap[strings.ToLower(st.name)]
			if !ok {
				color = chartPaletteLight[i%len(chartPaletteLight)]
			}
			pct := (st.count * 100) / totalStatus
			statusList = append(statusList, emailStatusBar{
				Label:   st.name,
				Count:   st.count,
				Percent: pct,
				Color:   color,
			})
		}
		report.StatusBars = statusList
	}

	// Prepare trend bars for HTML table fallback
	report.TrendBars = nil
	if len(report.TrendPoints) == 7 {
		var trendBars []emailTrendBar
		maxTrend := 1
		for _, v := range report.TrendPoints {
			if v > maxTrend {
				maxTrend = v
			}
		}
		for i, count := range report.TrendPoints {
			barH := 4
			if maxTrend > 0 && count > 0 {
				barH = 4 + (count*22)/maxTrend
			}
			lbl := curveLabels[i]
			trendBars = append(trendBars, emailTrendBar{
				Label:     lbl,
				Count:     count,
				BarHeight: barH,
				IsLast:    i == 6,
			})
		}
		report.TrendBars = trendBars
	}

	// Owners distribution for classic bar charts
	owners := map[string]int{}
	for _, member := range report.CoreMembers {
		owners[member.Name] += member.UnresolvedCount
	}
	report.JiraCharts = []emailChart{}

	// If project groups are defined, add project group charts first!
	if len(report.YesterdayGroups) > 0 || len(report.UnresolvedGroups) > 0 {
		unresolvedGroupCounts := map[string]int{}
		for _, g := range report.UnresolvedGroups {
			if g.Count > 0 {
				unresolvedGroupCounts[g.Name] = g.Count
			}
		}
		if len(report.UnresolvedUngrouped) > 0 {
			unresolvedGroupCounts["其他项目 / 未分组"] = len(report.UnresolvedUngrouped)
		}

		yesterdayGroupCounts := map[string]int{}
		for _, g := range report.YesterdayGroups {
			if g.Count > 0 {
				yesterdayGroupCounts[g.Name] = g.Count
			}
		}
		if len(report.YesterdayUngrouped) > 0 {
			yesterdayGroupCounts["其他项目 / 未分组"] = len(report.YesterdayUngrouped)
		}

		if len(unresolvedGroupCounts) > 0 {
			report.JiraCharts = append(report.JiraCharts, makeEmailChart("项目分组 · 近 3 天未解决分布", "按负责人负责项目分组统计待跟进事项", unresolvedGroupCounts))
		}
		if len(yesterdayGroupCounts) > 0 {
			report.JiraCharts = append(report.JiraCharts, makeEmailChart("项目分组 · 昨日更新分布", "按负责人负责项目分组统计昨日事项流转", yesterdayGroupCounts))
		}
		report.JiraCharts = append(report.JiraCharts,
			makeEmailChart("昨日 Jira 状态分布", "昨日更新事项当前状态分组明细", statuses),
		)
	} else {
		report.JiraCharts = append(report.JiraCharts,
			makeEmailChart("昨日 Jira 状态分布", "昨日更新事项当前状态分组明细", statuses),
			makeEmailChart("近 3 天未解决 · 负责人分布", "前 3 天创建且未解决事项分布", owners),
		)
	}

	// De-AI-ified, concise facts and structured group analysis
	rateStr := "—"
	if len(report.YesterdayUpdated) > 0 {
		rateStr = fmt.Sprintf("%d%%", report.ResolutionRate)
	}
	report.Analysis = []string{
		fmt.Sprintf("大盘态势：昨日更新 %d 项（已解决 %d 项，解决率 %s），近 3 天待跟进事项共 %d 项。", len(report.YesterdayUpdated), resolved, rateStr, len(report.RecentUnresolved)),
	}

	if len(report.ProjectGroupStats) > 0 {
		var groupSummaries []string
		for _, g := range report.ProjectGroupStats {
			ownersText := ""
			if g.OwnersLabel != "" && g.OwnersLabel != "未指定分组" {
				ownersText = fmt.Sprintf("负责人：%s，", g.OwnersLabel)
			}
			pct := 0
			if len(report.RecentUnresolved) > 0 {
				pct = (g.RecentUnresolvedCount * 100) / len(report.RecentUnresolved)
			}
			rate := "—"
			if g.YesterdayCount > 0 {
				rate = fmt.Sprintf("%d%%", g.ResolutionRate)
			}
			if g.ResolvedCount > 0 {
				groupSummaries = append(groupSummaries, fmt.Sprintf("【%s】（%s待解决 %d 项/占 %d%%，昨日更新 %d 项/解决率 %s）", g.Name, ownersText, g.RecentUnresolvedCount, pct, g.YesterdayCount, rate))
			} else {
				groupSummaries = append(groupSummaries, fmt.Sprintf("【%s】（%s待解决 %d 项/占 %d%%，昨日更新 %d 项）", g.Name, ownersText, g.RecentUnresolvedCount, pct, g.YesterdayCount))
			}
		}
		if len(groupSummaries) > 0 {
			report.Analysis = append(report.Analysis, "项目分组统计："+strings.Join(groupSummaries, "；")+"。")
		}
		maxGroup := ""
		maxOwners := ""
		maxCount := 0
		for _, g := range report.ProjectGroupStats {
			if g.RecentUnresolvedCount > maxCount && g.Name != "其他项目 / 未分组" {
				maxCount = g.RecentUnresolvedCount
				maxGroup = g.Name
				maxOwners = g.OwnersLabel
			}
		}
		if maxCount == 0 {
			for _, g := range report.ProjectGroupStats {
				if g.RecentUnresolvedCount > maxCount {
					maxCount = g.RecentUnresolvedCount
					maxGroup = g.Name
					maxOwners = g.OwnersLabel
				}
			}
		}
		if maxCount > 0 {
			pct := 0
			if len(report.RecentUnresolved) > 0 {
				pct = (maxCount * 100) / len(report.RecentUnresolved)
			}
			ownerHint := ""
			if maxOwners != "" && maxOwners != "未指定分组" {
				ownerHint = fmt.Sprintf("（负责人：%s）", maxOwners)
			}
			report.Analysis = append(report.Analysis, fmt.Sprintf("重点跟进建议：「%s」%s当前待解决事项最多（%d 项，占 %d%%），建议优先推进阻断缺陷与验收闭环。", maxGroup, ownerHint, maxCount, pct))
		} else {
			report.Analysis = append(report.Analysis, "当前各项目分组无近 3 天待跟进事项，整体流转顺畅。")
		}
	} else if len(report.YesterdayGroups) > 0 || len(report.UnresolvedGroups) > 0 {
		var groupSummaries []string
		for _, g := range report.UnresolvedGroups {
			yCount := 0
			for _, yg := range report.YesterdayGroups {
				if yg.Name == g.Name {
					yCount = yg.Count
					break
				}
			}
			ownersText := ""
			if len(g.Owners) > 0 {
				ownersText = fmt.Sprintf("负责人：%s，", strings.Join(g.Owners, "、"))
			}
			pct := 0
			if len(report.RecentUnresolved) > 0 {
				pct = (g.Count * 100) / len(report.RecentUnresolved)
			}
			groupSummaries = append(groupSummaries, fmt.Sprintf("【%s】（%s待解决 %d 项/占 %d%%，昨日更新 %d 项）", g.Name, ownersText, g.Count, pct, yCount))
		}
		if len(report.UnresolvedUngrouped) > 0 || len(report.YesterdayUngrouped) > 0 {
			pct := 0
			if len(report.RecentUnresolved) > 0 {
				pct = (len(report.UnresolvedUngrouped) * 100) / len(report.RecentUnresolved)
			}
			groupSummaries = append(groupSummaries, fmt.Sprintf("【未分组】（待解决 %d 项/占 %d%%，昨日更新 %d 项）", len(report.UnresolvedUngrouped), pct, len(report.YesterdayUngrouped)))
		}
		if len(groupSummaries) > 0 {
			report.Analysis = append(report.Analysis, "项目分组统计："+strings.Join(groupSummaries, "；")+"。")
		}
		maxGroup := ""
		maxCount := 0
		for _, g := range report.UnresolvedGroups {
			if g.Count > maxCount {
				maxCount = g.Count
				maxGroup = g.Name
			}
		}
		if maxCount > 0 {
			pct := 0
			if len(report.RecentUnresolved) > 0 {
				pct = (maxCount * 100) / len(report.RecentUnresolved)
			}
			report.Analysis = append(report.Analysis, fmt.Sprintf("重点跟进建议：「%s」当前待解决事项最多（%d 项，占 %d%%），建议优先跟进协调。", maxGroup, maxCount, pct))
		}
	} else if len(owners) > 0 {
		var topName string
		var topCount int
		for n, c := range owners {
			if c > topCount {
				topCount = c
				topName = n
			}
		}
		if topCount > 0 && len(report.RecentUnresolved) > 0 {
			pct := (topCount * 100) / len(report.RecentUnresolved)
			report.Analysis = append(report.Analysis, fmt.Sprintf("重点跟进建议：%s 待解决事项较多（%d 项，占 %d%%），建议优先跟进闭环。", topName, topCount, pct))
		}
	} else {
		report.Analysis = append(report.Analysis, "当前无近 3 天未解决事项，流转顺畅。")
	}

	report.CommitChart = nil
	if report.Commits != nil {
		report.Commits.Contributors = 0
		for _, count := range report.Commits.Authors {
			if count > 0 {
				report.Commits.Contributors++
			}
		}
		chart := makeEmailChartData("昨日采集 commit · 作者分布", "按去重提交数统计作者占比。", report.Commits.Authors)
		if report.Commits.Contributors > 8 {
			chart.Description += "展示前 8 位作者，其余合并为“其他”。"
		}
		if chart.Total != report.Commits.Count {
			chart.Description += fmt.Sprintf("作者分布覆盖 %d / %d 次已采集提交。", chart.Total, report.Commits.Count)
		}
		if report.Commits.Count == 0 && chart.Total == 0 {
			chart.Title = "昨日采集 commit · 零值统计"
			chart.Description = "各统计维度保留零值轨道；昨日没有采集到提交。"
		}
		chart.SVG = renderEmailCommitChart(&chart, report.Commits)
		report.CommitChart = &chart
	}
}

func renderEmailActivityBars(chart emailChart) template.HTML {
	var out strings.Builder
	peak := 1
	for _, bar := range chart.Bars {
		peak = max(peak, bar.Count)
	}
	out.WriteString(`<table role="presentation" class="email-activity-bars" style="width:100%;table-layout:fixed;border-collapse:collapse"><tr>`)
	for i, bar := range chart.Bars {
		fmt.Fprintf(&out, `<td style="text-align:center;padding:12px 16px 0;vertical-align:bottom">%s<p class="email-text-main" style="font-size:13px;margin:10px 0">%s · %d</p></td>`, emailActivityColumn(bar.Count, peak, i, fmt.Sprintf("昨日 %s：%d", bar.Label, bar.Count)), html.EscapeString(bar.Label), bar.Count)
	}
	out.WriteString(`</tr></table>`)
	return template.HTML(out.String())
}
