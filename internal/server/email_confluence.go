package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"net/url"
	"strconv"
	"strings"
	"well-ambient/internal/config"
	"well-ambient/internal/confluence"

	nethtml "golang.org/x/net/html"
)

func syncEmailReportToConfluence(ctx context.Context, cfg config.ConfluenceSyncConfig, report *emailReport) (string, error) {
	storage, attachments, err := confluenceReportStorage(report)
	if err != nil {
		return "", err
	}
	// The archive identity must not change when users edit the email subject.
	page, err := confluence.Sync(ctx, confluence.Config{ParentPageURL: cfg.ParentPageURL, Token: cfg.Token},
		"研发早报 · "+report.Date, storage, attachments)
	if err != nil {
		return "", err
	}
	if page.URL == "" {
		return "", fmt.Errorf("Confluence 未返回早报页面链接")
	}
	return page.URL, nil
}

func confluenceReportStorage(report *emailReport) (string, []confluence.Attachment, error) {
	var out strings.Builder
	var attachments []confluence.Attachment
	seen := map[string]bool{}
	escape := html.EscapeString
	paragraph := func(value string) {
		if value != "" {
			fmt.Fprintf(&out, "<p>%s</p>", strings.ReplaceAll(escape(value), "\n", "<br />"))
		}
	}
	heading := func(value string) { fmt.Fprintf(&out, "<h2>%s</h2>", escape(value)) }
	table := func(headers []string, rows [][]string) {
		out.WriteString("<table><tbody><tr>")
		for _, header := range headers {
			fmt.Fprintf(&out, "<th>%s</th>", escape(header))
		}
		out.WriteString("</tr>")
		for _, row := range rows {
			out.WriteString("<tr>")
			for _, cell := range row {
				fmt.Fprintf(&out, "<td>%s</td>", escape(cell))
			}
			out.WriteString("</tr>")
		}
		out.WriteString("</tbody></table>")
	}
	graphic := func(markup template.HTML) error {
		doc, err := nethtml.Parse(strings.NewReader(string(markup)))
		if err != nil {
			return fmt.Errorf("无法准备 Confluence 图表")
		}
		var walk func(*nethtml.Node) error
		walk = func(node *nethtml.Node) error {
			if node.Type == nethtml.TextNode {
				out.WriteString(escape(node.Data))
				return nil
			}
			if node.Type != nethtml.ElementNode && node.Type != nethtml.DocumentNode {
				return nil
			}
			attrs := map[string]string{}
			for _, attr := range node.Attr {
				attrs[attr.Key] = attr.Val
			}
			tag := node.Data
			switch tag {
			case "head", "script", "style":
				return nil
			case "img":
				const prefix = "data:image/png;base64,"
				if !strings.HasPrefix(attrs["src"], prefix) {
					return fmt.Errorf("Confluence 图表必须使用本地 PNG")
				}
				data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(attrs["src"], prefix))
				if err != nil || len(data) == 0 || len(data) > 2<<20 {
					return fmt.Errorf("Confluence 图表数据无效或过大")
				}
				digest := sha256.Sum256(data)
				name := fmt.Sprintf("daily-jira-chart-%x.png", digest)
				if !seen[name] {
					attachments = append(attachments, confluence.Attachment{Filename: name, Data: data, ContentType: "image/png"})
					seen[name] = true
				}
				width := 640
				if parsed, err := strconv.Atoi(attrs["width"]); err == nil && parsed > 0 && parsed <= 640 {
					width = parsed
				}
				fmt.Fprintf(&out, `<ac:image ac:alt="%s" ac:width="%d"><ri:attachment ri:filename="%s" /></ac:image>`, escape(attrs["alt"]), width, name)
				return nil
			case "div", "p", "span", "strong", "em", "table", "thead", "tbody", "tr", "th", "td", "ul", "ol", "li", "h3":
			case "br":
				out.WriteString("<br />")
				return nil
			default:
				tag = ""
			}
			if tag != "" {
				out.WriteString("<" + tag)
				for _, key := range []string{"colspan", "rowspan", "style", "width", "height", "align", "valign", "class"} {
					if value := attrs[key]; value != "" {
						fmt.Fprintf(&out, ` %s="%s"`, key, escape(value))
					}
				}
				out.WriteString(">")
			}
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				if err := walk(child); err != nil {
					return err
				}
			}
			if tag != "" {
				out.WriteString("</" + tag + ">")
			}
			return nil
		}
		return walk(doc)
	}
	fmt.Fprintf(&out, "<h1>%s</h1>", escape(report.Subject))
	paragraph("报告日期：" + report.Date + " · " + report.Timezone)
	heading("早报概览")

	// 1. 大盘态势表格
	heading("大盘态势")
	rateStr := "—"
	if len(report.YesterdayUpdated) > 0 {
		rateStr = fmt.Sprintf("%d%%", report.ResolutionRate)
	}
	overviewHeaders := []string{"昨日更新", "已解决", "解决率", "近 3 天待跟进"}
	overviewRow := []string{
		fmt.Sprintf("%d 项", len(report.YesterdayUpdated)),
		fmt.Sprintf("%d 项", report.ResolvedCount),
		rateStr,
		fmt.Sprintf("%d 项", len(report.RecentUnresolved)),
	}
	if report.Commits != nil {
		overviewHeaders = append(overviewHeaders, "昨日提交", "涉及仓库")
		overviewRow = append(overviewRow, fmt.Sprintf("%d 次", report.Commits.Count), fmt.Sprintf("%d 个", report.Commits.Repositories))
	}
	table(overviewHeaders, [][]string{overviewRow})

	// 2. 项目分组统计表格
	heading("项目分组统计")
	var groupRows [][]string
	if len(report.ProjectGroupStats) > 0 {
		groupHeaders := []string{"项目分组", "负责人", "负责项目", "昨日更新", "已解决", "解决率", "近 3 天待跟进", "待跟进占比"}
		if report.Commits != nil {
			groupHeaders = append(groupHeaders, "提交")
		}
		for _, group := range report.ProjectGroupStats {
			rate := "—"
			if group.YesterdayCount > 0 {
				rate = fmt.Sprintf("%d%%", group.ResolutionRate)
			}
			owners := group.OwnersLabel
			if owners == "" {
				owners = "—"
			}
			projects := group.ProjectsLabel
			if projects == "" {
				projects = "—"
			}
			pct := 0
			if len(report.RecentUnresolved) > 0 {
				pct = (group.RecentUnresolvedCount * 100) / len(report.RecentUnresolved)
			}
			row := []string{
				group.Name,
				owners,
				projects,
				fmt.Sprintf("%d 项", group.YesterdayCount),
				fmt.Sprintf("%d 项", group.ResolvedCount),
				rate,
				fmt.Sprintf("%d 项", group.RecentUnresolvedCount),
				fmt.Sprintf("%d%%", pct),
			}
			if report.Commits != nil {
				row = append(row, fmt.Sprint(group.CommitCount))
			}
			groupRows = append(groupRows, row)
		}
		table(groupHeaders, groupRows)
	} else {
		groupHeaders := []string{"分类", "说明", "昨日更新", "已解决", "待跟进", "解决率"}
		for _, category := range report.CategoryStats {
			rate := "—"
			if category.YesterdayCount > 0 {
				rate = fmt.Sprintf("%d%%", category.ResolutionRate)
			}
			groupRows = append(groupRows, []string{
				category.Name,
				category.FullName,
				fmt.Sprintf("%d 项", category.YesterdayCount),
				fmt.Sprintf("%d 项", category.ResolvedCount),
				fmt.Sprintf("%d 项", category.UnresolvedCount),
				rate,
			})
		}
		table(groupHeaders, groupRows)
	}

	// 3. 重点跟进建议表格
	heading("重点跟进建议")
	actionHeaders := []string{"重点跟进对象", "负责人", "待跟进事项", "待跟进占比", "建议行动"}
	var actionRows [][]string
	if len(report.ProjectGroupStats) > 0 {
		hasUnresolved := false
		for _, group := range report.ProjectGroupStats {
			if group.RecentUnresolvedCount > 0 {
				hasUnresolved = true
				pct := 0
				if len(report.RecentUnresolved) > 0 {
					pct = (group.RecentUnresolvedCount * 100) / len(report.RecentUnresolved)
				}
				owners := group.OwnersLabel
				if owners == "" {
					owners = "未指定"
				}
				action := "建议优先推进阻断缺陷与验收闭环"
				if group.Name == "其他项目 / 未分组" {
					action = "建议核实项目归属并指派负责人闭环"
				}
				actionRows = append(actionRows, []string{
					group.Name,
					owners,
					fmt.Sprintf("%d 项", group.RecentUnresolvedCount),
					fmt.Sprintf("%d%%", pct),
					action,
				})
			}
		}
		if !hasUnresolved {
			actionRows = append(actionRows, []string{
				"全部分组",
				"全体成员",
				"0 项",
				"0%",
				"当前各项目分组无近 3 天待跟进事项，整体流转顺畅",
			})
		}
	} else if len(report.RecentUnresolved) > 0 {
		actionRows = append(actionRows, []string{
			"全部待跟进事项",
			"相关负责人",
			fmt.Sprintf("%d 项", len(report.RecentUnresolved)),
			"100%",
			"建议优先排查处理高优先级的待解决缺陷",
		})
	} else {
		actionRows = append(actionRows, []string{
			"全部事项",
			"全体成员",
			"0 项",
			"0%",
			"当前无近 3 天未解决事项，流转顺畅",
		})
	}
	table(actionHeaders, actionRows)
	if report.ActivityChart != nil {
		heading(report.ActivityChart.Title)
		paragraph(report.ActivityChart.Description)
		if err := graphic(report.ActivityChart.SVG); err != nil {
			return "", nil, err
		}
	}
	heading("统计图表")
	out.WriteString("<table><tbody><tr><th>解决率指标</th><th>状态分布</th><th>近7日更新分布</th></tr><tr>")

	// 第 1 栏：解决率指标
	out.WriteString("<td>")
	if report.DonutChartSVG != "" {
		if err := graphic(report.DonutChartSVG); err != nil {
			return "", nil, err
		}
	}
	paragraph(report.DonutChartDesc)
	out.WriteString("</td>")

	// 第 2 栏：状态分布
	out.WriteString("<td>")
	if report.PieChartSVG != "" {
		if err := graphic(extractOnlyImg(report.PieChartSVG)); err != nil {
			return "", nil, err
		}
	}
	if len(report.StatusBars) > 0 {
		var parts []string
		for _, sb := range report.StatusBars {
			parts = append(parts, fmt.Sprintf("%s %d", escape(sb.Label), sb.Count))
		}
		out.WriteString(fmt.Sprintf("<p>%s</p>", strings.Join(parts, " · ")))
	} else if report.PieChartLegend != "" {
		if err := graphic(report.PieChartLegend); err != nil {
			return "", nil, err
		}
	} else {
		out.WriteString("<p>暂无状态分布</p>")
	}
	out.WriteString("</td>")

	// 第 3 栏：近7日更新分布
	out.WriteString("<td>")
	if report.CurveChartSVG != "" {
		if err := graphic(extractOnlyImg(report.CurveChartSVG)); err != nil {
			return "", nil, err
		}
	}
	paragraph(report.CurveChartDesc)
	var trendParts []string
	if len(report.TrendBars) == 7 {
		for _, bar := range report.TrendBars {
			trendParts = append(trendParts, fmt.Sprintf("%s: %d", escape(bar.Label), bar.Count))
		}
	} else if len(report.TrendPoints) == 7 {
		labels := []string{"-7d", "-6d", "-5d", "-4d", "-3d", "-2d", "-1d"}
		for i, count := range report.TrendPoints {
			trendParts = append(trendParts, fmt.Sprintf("%s: %d", labels[i], count))
		}
	} else {
		if extracted := extractTrendPointsFromMarkup(report.CurveChartSVG); extracted != "" {
			trendParts = []string{extracted}
		}
	}
	if len(trendParts) > 0 {
		if len(trendParts) == 1 && strings.Contains(trendParts[0], " · ") {
			fmt.Fprintf(&out, "<p>%s</p>", trendParts[0])
		} else {
			fmt.Fprintf(&out, "<p>%s</p>", strings.Join(trendParts, " · "))
		}
	}
	out.WriteString("</td>")

	out.WriteString("</tr></tbody></table>")
	for _, chart := range report.JiraCharts {
		heading(chart.Title)
		if len(chart.Bars) > 0 {
			out.WriteString("<table><tbody><tr><th>分组 / 状态</th><th style=\"text-align:right\">数量</th><th style=\"text-align:right\">占比</th><th style=\"width:50%\">进度</th></tr>")
			for _, bar := range chart.Bars {
				label := strings.TrimSpace(bar.Label)
				if label == "" {
					label = "未标注"
				}
				pct := bar.Percent
				if pct < 0 {
					pct = 0
				} else if pct > 100 {
					pct = 100
				}
				share := float64(bar.Count) / float64(chart.Total)
				percentStr := fmt.Sprintf("%.1f%%", share*100)
				if share > 0 && share < 0.001 {
					percentStr = "<0.1%"
				}
				fmt.Fprintf(&out, `<tr><td><strong>%s</strong></td><td style="text-align:right">%d 项</td><td style="text-align:right">%s</td><td>`, escape(label), bar.Count, percentStr)
				fmt.Fprintf(&out, `<table style="width:100%%;border-collapse:collapse;margin:0;padding:0;height:12px;background-color:#e2e8f0;border-radius:6px;overflow:hidden"><tr><td style="width:%d%%;background-color:#008f96;height:12px;padding:0;border:none"></td><td style="width:%d%%;background-color:#e2e8f0;height:12px;padding:0;border:none"></td></tr></table>`, pct, 100-pct)
				out.WriteString("</td></tr>")
			}
			out.WriteString("</tbody></table>")
		} else if chart.SVG != "" {
			if err := graphic(chart.SVG); err != nil {
				return "", nil, err
			}
		} else {
			out.WriteString("<p>暂无匹配记录。</p>")
		}
	}
	issues := func(items []emailIssue) {
		if len(items) == 0 {
			paragraph("暂无匹配事项")
			return
		}
		out.WriteString("<table><tbody><tr><th>Jira</th><th>类型</th><th>事项</th><th>负责人</th><th>状态</th><th>分类</th><th>更新类型</th></tr>")
		for _, item := range items {
			out.WriteString("<tr><td>")
			u, err := url.Parse(item.URL)
			if err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.User == nil {
				fmt.Fprintf(&out, `<a href="%s">%s</a>`, escape(item.URL), escape(item.TaskID))
			} else {
				out.WriteString(escape(item.TaskID))
			}
			issueType := item.IssueType
			if issueType == "" {
				issueType = "任务"
			}
			fmt.Fprintf(&out, "</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>", escape(issueType), escape(item.Title), escape(item.Assignee), escape(item.Status), escape(item.Category), escape(item.UpdateSummary))
		}
		out.WriteString("</tbody></table>")
	}
	issueSection := func(title string, all []emailIssue, groups []emailIssueGroup, ungrouped []emailIssue) {
		heading(fmt.Sprintf("%s（%d）", title, len(all)))
		if len(groups) == 0 || len(all) == 0 {
			issues(all)
			return
		}
		for _, group := range groups {
			if len(group.Issues) == 0 {
				continue
			}
			fmt.Fprintf(&out, "<h3>%s（%d）</h3>", escape(group.Name), len(group.Issues))
			if group.OwnersLabel != "" {
				paragraph("负责人：" + group.OwnersLabel)
			}
			issues(group.Issues)
		}
		if len(ungrouped) > 0 {
			out.WriteString("<h3>其他项目 / 未分组</h3>")
			issues(ungrouped)
		}
	}
	issueSection("昨日更新 Jira", report.YesterdayUpdated, report.YesterdayGroups, report.YesterdayUngrouped)
	issueSection("近3天创建且未解决", report.RecentUnresolved, report.UnresolvedGroups, report.UnresolvedUngrouped)
	if len(report.ProjectGroupStats) == 0 {
		heading("Coremember 负责人")
		var rows [][]string
		for _, member := range report.CoreMembers {
			row := []string{member.Name, fmt.Sprint(member.YesterdayCount), fmt.Sprint(member.UnresolvedCount)}
			if report.Commits != nil {
				row = append(row, fmt.Sprint(member.CommitCount))
			}
			rows = append(rows, row)
		}
		headers := []string{"负责人", "昨日更新", "近3天未解决"}
		if report.Commits != nil {
			headers = append(headers, "提交")
		}
		table(headers, rows)
	}
	if report.Commits != nil {
		heading("昨日 commit 统计分析")
		table([]string{"采集提交", "涉及仓库", "贡献者"}, [][]string{{fmt.Sprint(report.Commits.Count), fmt.Sprint(report.Commits.Repositories), fmt.Sprint(report.Commits.Contributors)}})
		paragraph(report.Commits.Analysis)
		out.WriteString("<table><tbody><tr><th>仓库</th><th>分支</th><th>次数</th><th>提交记录</th></tr>")
		for _, row := range report.Commits.Rows {
			fmt.Fprintf(&out, "<tr><td>%s</td><td>%s</td><td>%d</td><td>", escape(row.Repository), escape(row.Branch), row.Count)
			for _, c := range row.Commits {
				out.WriteString("<p>")
				if parsed, err := url.Parse(c.URL); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
					fmt.Fprintf(&out, "<a href=\"%s\">%s</a>", escape(c.URL), escape(c.ID))
				} else {
					out.WriteString(escape(c.ID) + "（链接未配置）")
				}
				fmt.Fprintf(&out, " · %s · %s</p>", escape(c.Author), escape(c.Message))
			}
			out.WriteString("</td></tr>")
		}
		out.WriteString("</tbody></table>")
		paragraph("分支按采集记录展示；同一提交出现在多个分支时分别列示，总数仍按仓库与 SHA 去重。")
		if report.CommitChart != nil {
			paragraph(report.CommitChart.Title)
			if err := graphic(report.CommitChart.SVG); err != nil {
				return "", nil, err
			}
		}
	}
	return out.String(), attachments, nil
}

func extractOnlyImg(markup template.HTML) template.HTML {
	doc, err := nethtml.Parse(strings.NewReader(string(markup)))
	if err != nil {
		return markup
	}
	var findImg func(*nethtml.Node) *nethtml.Node
	findImg = func(n *nethtml.Node) *nethtml.Node {
		if n.Type == nethtml.ElementNode && n.Data == "img" {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if res := findImg(c); res != nil {
				return res
			}
		}
		return nil
	}
	imgNode := findImg(doc)
	if imgNode == nil {
		return markup
	}
	var b bytes.Buffer
	if err := nethtml.Render(&b, imgNode); err != nil {
		return markup
	}
	return template.HTML(b.String())
}

func extractTrendPointsFromMarkup(markup template.HTML) string {
	doc, err := nethtml.Parse(strings.NewReader(string(markup)))
	if err != nil {
		return ""
	}
	var texts []string
	var walk func(*nethtml.Node)
	walk = func(n *nethtml.Node) {
		if n.Type == nethtml.TextNode {
			t := strings.TrimSpace(n.Data)
			if t != "" {
				texts = append(texts, t)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	idx := -1
	for i, t := range texts {
		if t == "-7d" {
			idx = i
			break
		}
	}
	if idx >= 0 && idx+13 < len(texts) {
		var parts []string
		for i := 0; i < 7; i++ {
			parts = append(parts, fmt.Sprintf("%s: %s", html.EscapeString(texts[idx+i]), html.EscapeString(texts[idx+7+i])))
		}
		return strings.Join(parts, " · ")
	}
	var parts []string
	for _, t := range texts {
		if strings.HasPrefix(t, "-") && strings.Contains(t, "d:") {
			parts = append(parts, html.EscapeString(t))
		}
	}
	if len(parts) == 7 {
		return strings.Join(parts, " · ")
	}
	return ""
}
