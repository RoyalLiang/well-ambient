package server

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"html/template"
	"net/url"
	"sort"
	"strings"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

type emailIssue struct {
	UpdateTypes   []string  `json:"update_types,omitempty"`
	UpdateSummary string    `json:"update_summary,omitempty"`
	TaskID        string    `json:"task_id"`
	URL           string    `json:"url,omitempty"`
	Title         string    `json:"title"`
	Assignee      string    `json:"assignee"`
	Status        string    `json:"status"`
	IssueType     string    `json:"issue_type,omitempty"`
	Category      string    `json:"category,omitempty"`
	GroupName     string    `json:"group_name,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
type emailCoreMember struct {
	Name            string `json:"name"`
	YesterdayCount  int    `json:"yesterday_count"`
	UnresolvedCount int    `json:"unresolved_count"`
	CommitCount     int    `json:"commit_count"`
}
type emailCommitSummary struct {
	Rows         []emailCommitRow `json:"rows"`
	Count        int              `json:"count"`
	Repositories int              `json:"repositories"`
	Contributors int              `json:"contributors"`
	Authors      map[string]int   `json:"authors"`
	Analysis     string           `json:"analysis"`
}
type emailCategoryStat struct {
	Name            string `json:"name"`
	FullName        string `json:"full_name"`
	Description     string `json:"description"`
	YesterdayCount  int    `json:"yesterday_count"`
	ResolvedCount   int    `json:"resolved_count"`
	UnresolvedCount int    `json:"unresolved_count"`
	ResolutionRate  int    `json:"resolution_rate"`
}
type emailIssueGroup struct {
	Name        string       `json:"name"`
	Owners      []string     `json:"owners"`
	OwnersLabel string       `json:"owners_label"`
	Projects    []string     `json:"projects"`
	Issues      []emailIssue `json:"issues"`
	Count       int          `json:"count"`
}
type emailProjectGroupStat struct {
	Name                  string   `json:"name"`
	Owners                []string `json:"owners"`
	OwnersLabel           string   `json:"owners_label"`
	Projects              []string `json:"projects"`
	ProjectsLabel         string   `json:"projects_label"`
	ProjectCount          int      `json:"project_count"`
	YesterdayCount        int      `json:"yesterday_count"`
	ResolvedCount         int      `json:"resolved_count"`
	UnresolvedCount       int      `json:"unresolved_count"`
	ResolutionRate        int      `json:"resolution_rate"`
	RecentUnresolvedCount int      `json:"recent_unresolved_count"`
	CommitCount           int      `json:"commit_count"`
}
type emailStatusBar struct {
	Label   string `json:"label"`
	Count   int    `json:"count"`
	Percent int    `json:"percent"`
	Color   string `json:"color"`
}
type emailTrendBar struct {
	Label     string `json:"label"`
	Count     int    `json:"count"`
	BarHeight int    `json:"bar_height"`
	IsLast    bool   `json:"is_last"`
}
type emailReport struct {
	Style               string                  `json:"style"`
	Date                string                  `json:"date"`
	Timezone            string                  `json:"timezone"`
	Subject             string                  `json:"subject"`
	HTML                string                  `json:"html"`
	Text                string                  `json:"text"`
	ConfluenceURL       string                  `json:"confluence_url,omitempty"`
	Semantics           string                  `json:"semantics"`
	YesterdayUpdated    []emailIssue            `json:"yesterday_updated"`
	RecentUnresolved    []emailIssue            `json:"recent_unresolved"`
	YesterdayGroups     []emailIssueGroup       `json:"yesterday_groups,omitempty"`
	YesterdayUngrouped  []emailIssue            `json:"yesterday_ungrouped,omitempty"`
	UnresolvedGroups    []emailIssueGroup       `json:"unresolved_groups,omitempty"`
	UnresolvedUngrouped []emailIssue            `json:"unresolved_ungrouped,omitempty"`
	ProjectGroupStats   []emailProjectGroupStat `json:"project_group_stats,omitempty"`
	CoreMembers         []emailCoreMember       `json:"core_members"`
	Commits             *emailCommitSummary     `json:"commits,omitempty"`
	CategoryStats       []emailCategoryStat     `json:"category_stats,omitempty"`
	Warnings            []string                `json:"warnings"`
	Introduction        string                  `json:"-"`
	Closing             string                  `json:"-"`
	JiraCharts          []emailChart            `json:"jira_charts"`
	CommitChart         *emailChart             `json:"commit_chart,omitempty"`
	ActivityChart       *emailChart             `json:"activity_chart,omitempty"`
	Analysis            []string                `json:"analysis"`
	CustomHTML          template.HTML           `json:"-"`
	ResolvedCount       int                     `json:"resolved_count"`
	ResolutionRate      int                     `json:"resolution_rate"`
	DonutChartSVG       template.HTML           `json:"donut_chart_svg"`
	DonutChartDesc      string                  `json:"donut_chart_desc"`
	PieChartSVG         template.HTML           `json:"pie_chart_svg"`
	PieChartLegend      template.HTML           `json:"pie_chart_legend"`
	CurveChartSVG       template.HTML           `json:"curve_chart_svg"`
	CurveChartDesc      string                  `json:"curve_chart_desc"`
	TrendPoints         []int                   `json:"trend_points,omitempty"`
	TrendUnavailable    string                  `json:"trend_unavailable,omitempty"`
	StatusBars          []emailStatusBar        `json:"status_bars,omitempty"`
	TrendBars           []emailTrendBar         `json:"trend_bars,omitempty"`
}

// date is the report morning. Calendar arithmetic handles 23/25 hour days.
func emailDateBounds(date, timezone string, now time.Time) (time.Time, time.Time, time.Time, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("invalid email timezone")
	}
	if date == "" {
		date = now.In(location).Format("2006-01-02")
	}
	today, err := time.ParseInLocation("2006-01-02", date, location)
	if err != nil || today.Format("2006-01-02") != date {
		return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("date must be YYYY-MM-DD")
	}
	return today, today.AddDate(0, 0, -1), today.AddDate(0, 0, -3), nil
}
func (s *Server) buildEmailReport(ctx context.Context, cfg config.Config, settings config.DailyJiraEmailConfig, date string, now time.Time) (emailReport, error) {
	settings = settings.Normalized()
	if err := config.ValidateDailyJiraEmail(settings); err != nil {
		return emailReport{}, err
	}
	today, yesterday, recent, err := emailDateBounds(date, settings.Timezone, now)
	if err != nil {
		return emailReport{}, err
	}
	report := emailReport{Style: settings.Template.Style, Date: today.Format("2006-01-02"), Timezone: settings.Timezone, YesterdayUpdated: []emailIssue{}, RecentUnresolved: []emailIssue{}, CoreMembers: []emailCoreMember{}, Warnings: []string{}}
	report.Semantics = fmt.Sprintf("报告日期为发送日；昨日 Jira 按 source_updated_at 位于 [%s, %s)，包含已解决事项；近3天未解决按创建时间位于 [%s, %s)，不含今日新建。时区 %s。数据来自本地 Jira 同步事实，不触发外部同步；生成于 %s。", yesterday.Format("2006-01-02 15:04"), today.Format("2006-01-02 15:04"), recent.Format("2006-01-02 15:04"), today.Format("2006-01-02 15:04"), settings.Timezone, now.In(today.Location()).Format(time.RFC3339))
	if db.DB == nil {
		return report, fmt.Errorf("database is unavailable")
	}
	scopedServer := &Server{config: &cfg}
	members := scopedServer.configuredKPICoreMembers()
	members = normalizeConfiguredKPICoreMembers(members)
	var users []userdb.User
	if err := db.DB.WithContext(ctx).Order("id").Limit(5000).Find(&users).Error; err != nil {
		return report, fmt.Errorf("cannot load core member identities")
	}
	directory := newKPIUserDirectory(users)
	filter := kpiCoreMemberFilter{
		enabled: len(members) > 0,
		keys:    make(map[string]struct{}, len(members)*3),
	}
	for _, member := range members {
		addKPICoreMemberKeys(filter.keys, member)
		identity := directory.resolve(member)
		addKPICoreMemberKeys(filter.keys, identity.Name)
		addKPICoreMemberKeys(filter.keys, identity.Username)
	}
	coreIndices := map[string]int{}
	nameCounts := map[string]int{}
	exactUsers := map[string]bool{}
	for _, user := range users {
		if user.Name != "" {
			nameCounts[strings.ToLower(normalizeAssignee(user.Name))]++
		}
		exactUsers[strings.ToLower(user.Username)] = true
		if user.Email != "" {
			exactUsers[strings.ToLower(user.Email)] = true
		}
	}
	identityLabel := func(value string) string {
		identity := directory.resolve(value)
		name := identity.Name
		if name == "" {
			name = value
		}
		if identity.Username != "" && nameCounts[strings.ToLower(name)] > 1 {
			return name + " (" + identity.Username + ")"
		}
		return name
	}
	ambiguous := func(value string) bool {
		return !exactUsers[strings.ToLower(value)] && nameCounts[strings.ToLower(normalizeAssignee(value))] >= 2
	}
	for _, member := range members {
		if ambiguous(member) {
			continue
		}
		name := identityLabel(member)
		if _, exists := coreIndices[name]; exists {
			continue
		}
		coreIndices[name] = len(report.CoreMembers)
		report.CoreMembers = append(report.CoreMembers, emailCoreMember{Name: name})
	}
	coreName := func(value string) string {
		label := identityLabel(value)
		if _, ok := coreIndices[label]; ok {
			return label
		}
		for _, member := range members {
			if strings.EqualFold(identityLabel(member), label) || strings.EqualFold(normalizeAssignee(member), normalizeAssignee(value)) {
				return identityLabel(member)
			}
		}
		return label
	}

	responsibleMembers := func(owners []string) []string {
		names := []string{}
		for _, owner := range owners {
			if ambiguous(owner) || !filter.enabled || !filter.includesAssignee(owner, directory) {
				continue
			}
			name := coreName(owner)
			if _, ok := coreIndices[name]; ok {
				names = appendEmailFact(names, name)
			}
		}
		return names
	}
	report.Semantics += " 仅统计 bug归类 字段为 FMS/GPP 且曾由 coremember 负责的 Jira；当前转派不移除历史责任。每位历史核心负责人各计一次，同一事项在汇总中只计一次。"
	if len(members) == 0 {
		report.Warnings = append(report.Warnings, "未配置 Jira coremember（sync_users 或 JQL assignee），早报不展示全量人员数据。")
	}
	if !cfg.Jira.Enabled {
		report.Warnings = append(report.Warnings, "Jira 同步当前未启用，数据可能不是最新状态。")
	}
	// This is a distribution of the latest timestamps in the local snapshot,
	// not a reconstruction of every historical Jira change.
	report.Semantics += " 七日图按本地记录最后更新时间分布，不代表每天全部变更次数。更新明细仅展示本地已记录的变更与评论；历史记录不全时不能还原全部操作。"
	trendStart := today.AddDate(0, 0, -7)
	trendDays := map[string]int{}
	var tasks []db.TaskTelemetry
	if len(members) > 0 {
		report.TrendPoints = make([]int, 7)
		for i := range report.TrendPoints {
			trendDays[trendStart.AddDate(0, 0, i).Format("2006-01-02")] = i
		}
		condition := "(source_updated_at >= ? AND source_updated_at < ?) OR (task_created_at >= ? AND task_created_at < ?)"
		updatedStart, end, createdStart := yesterday.UTC(), today.UTC(), recent.UTC()
		if db.DB.Dialector.Name() == "sqlite" {
			condition = "(julianday(source_updated_at) >= julianday(?) AND julianday(source_updated_at) < julianday(?)) OR (julianday(task_created_at) >= julianday(?) AND julianday(task_created_at) < julianday(?))"
			updatedStart = updatedStart.Add(-time.Second)
			createdStart = createdStart.Add(-time.Second)
			end = end.Add(time.Second)
		}
		err = db.DB.WithContext(ctx).Select("task_id", "title", "assignee", "status", "issue_type", "source", "repo", "task_created_at", "source_updated_at", "jira_bug_category", "jira_bug_category_field_id", "jira_history_complete").Where(condition, updatedStart, end, createdStart, end).Order("task_id").Limit(10001).Find(&tasks).Error
		if err != nil {
			return report, fmt.Errorf("cannot load Jira report facts")
		}
		if len(tasks) > 10000 {
			return report, fmt.Errorf("report exceeds 10000 candidate issues; narrow the Jira sync scope")
		}
	}
	if len(members) > 0 {
		// Optional chart work cannot enlarge the main report's candidate quota.
		trendContext, cancelTrend := context.WithTimeout(ctx, 3*time.Second)
		defer cancelTrend()
		condition := "source_updated_at >= ? AND source_updated_at < ?"
		start, end := trendStart.UTC(), today.UTC()
		if db.DB.Dialector.Name() == "sqlite" {
			condition = "julianday(source_updated_at) >= julianday(?) AND julianday(source_updated_at) < julianday(?)"
			start, end = start.Add(-time.Second), end.Add(time.Second)
		}
		var trendTasks []db.TaskTelemetry
		trendErr := db.DB.WithContext(trendContext).Select("task_id", "source", "repo", "assignee", "source_updated_at", "jira_bug_category", "jira_bug_category_field_id", "jira_history_complete").Where(condition, start, end).Order("task_id").Limit(10001).Find(&trendTasks).Error
		switch {
		case trendErr != nil:
			report.TrendPoints = nil
			report.TrendUnavailable = "近7日数据暂不可用"
		case len(trendTasks) > 10000:
			report.TrendPoints = nil
			report.TrendUnavailable = "近7日数据过多，未展示"
		default:
			trendFacts, factsErr := loadEmailJiraFacts(trendContext, trendTasks, trendStart, today)
			if factsErr != nil {
				report.TrendPoints = nil
				report.TrendUnavailable = "近7日负责人历史暂不可用"
				break
			}
			for _, task := range trendTasks {
				if !isDailyJiraCandidate(task) || (task.JiraBugCategory != "FMS" && task.JiraBugCategory != "GPP") || len(responsibleMembers(trendFacts.Owners[task.TaskID])) == 0 {
					continue
				}
				if index, ok := trendDays[task.SourceUpdatedAt.In(today.Location()).Format("2006-01-02")]; ok {
					report.TrendPoints[index]++
				}
			}
		}
	}
	facts, err := loadEmailJiraFacts(ctx, tasks, yesterday, today)
	if err != nil {
		return report, err
	}
	missingCategory, missingHistory := 0, 0
	for _, task := range tasks {
		if !isDailyJiraCandidate(task) {
			continue
		}
		if task.JiraBugCategoryFieldID == "" {
			missingCategory++
		}
		if !task.JiraHistoryComplete {
			missingHistory++
		}
		if task.JiraBugCategory != "FMS" && task.JiraBugCategory != "GPP" {
			continue
		}
		responsible := responsibleMembers(facts.Owners[task.TaskID])
		if len(responsible) == 0 {
			continue
		}
		category := task.JiraBugCategory
		item := emailIssue{
			TaskID:    task.TaskID,
			URL:       scopedServer.emailIssuePageURL(task.TaskID),
			Title:     task.Title,
			Assignee:  task.Assignee,
			Status:    task.Status,
			IssueType: formatDailyJiraIssueType(task.IssueType),
			Category:  category,
			CreatedAt: task.TaskCreatedAt,
			UpdatedAt: task.SourceUpdatedAt,
		}
		item.UpdateTypes = facts.Updates[task.TaskID]
		if !task.TaskCreatedAt.Before(yesterday) && task.TaskCreatedAt.Before(today) {
			item.UpdateTypes = appendEmailFact(item.UpdateTypes, "创建事项")
		}
		if len(item.UpdateTypes) == 0 {
			item.UpdateTypes = []string{"更新类型未记录"}
		}
		item.UpdateSummary = strings.Join(item.UpdateTypes, "；")
		if !task.SourceUpdatedAt.Before(yesterday) && task.SourceUpdatedAt.Before(today) {
			report.YesterdayUpdated = append(report.YesterdayUpdated, item)
			for _, name := range responsible {
				report.CoreMembers[coreIndices[name]].YesterdayCount++
			}
		}
		if !task.TaskCreatedAt.Before(recent) && task.TaskCreatedAt.Before(today) && !isResolvedDailyJiraStatus(task.Status) {
			report.RecentUnresolved = append(report.RecentUnresolved, item)
			for _, name := range responsible {
				report.CoreMembers[coreIndices[name]].UnresolvedCount++
			}
		}
	}
	if missingCategory > 0 {
		report.Warnings = append(report.Warnings, fmt.Sprintf("%d 条候选 Jira 尚未同步 bug归类 字段，未据标题或负责人推测分类。", missingCategory))
	}
	if missingHistory > 0 {
		report.Warnings = append(report.Warnings, fmt.Sprintf("%d 条候选 Jira 的负责人历史尚不完整，可能遗漏历史核心负责人；需完成 Jira 历史同步。", missingHistory))
	}
	if settings.IncludeCommits {
		summary := &emailCommitSummary{Authors: map[string]int{}}
		var commits []db.GitCommitLog
		if len(members) > 0 {
			condition := "created_at >= ? AND created_at < ? AND action = ?"
			start, end := yesterday.UTC(), today.UTC()
			if db.DB.Dialector.Name() == "sqlite" {
				condition = "julianday(created_at) >= julianday(?) AND julianday(created_at) < julianday(?) AND action = ?"
				start = start.Add(-time.Second)
				end = end.Add(time.Second)
			}
			if err := db.DB.WithContext(ctx).Select("id", "repo", "commit_id", "branch", "message", "author", "action", "duplicate_of_commit", "created_at").Where(condition, start, end, "git_push").Order("id").Limit(20001).Find(&commits).Error; err != nil {
				return report, fmt.Errorf("cannot load commit facts")
			}
			if len(commits) > 20000 {
				return report, fmt.Errorf("report exceeds 20000 candidate commits")
			}
		}
		rowCommits := []db.GitCommitLog{}
		seen := map[string]bool{}
		repos := map[string]bool{}
		for _, commit := range commits {
			if commit.CreatedAt.Before(yesterday) || !commit.CreatedAt.Before(today) {
				continue
			}
			if ambiguous(commit.Author) || (filter.enabled && !filter.includesAssignee(commit.Author, directory)) || commit.CommitID == "" || commit.DuplicateOfCommit != "" {
				continue
			}
			rowCommits = append(rowCommits, commit)
			key := commit.Repo + "\x00" + commit.CommitID
			if seen[key] {
				continue
			}
			seen[key] = true
			name := coreName(commit.Author)
			if name == "" {
				name = commit.Author
			}
			summary.Count++
			summary.Authors[name]++
			repos[commit.Repo] = true
			if index, ok := coreIndices[name]; ok {
				report.CoreMembers[index].CommitCount++
			}
		}
		summary.Rows = buildEmailCommitRows(cfg, rowCommits)
		summary.Repositories = len(repos)
		summary.Analysis = fmt.Sprintf("昨日采集到 %d 条去重提交，涉及 %d 个仓库、%d 位负责人；按仓库与 commit SHA 去重，仅统计 git_push。采集时间不等同于原始提交时间；提交数量不代表生产力或质量。", summary.Count, summary.Repositories, len(summary.Authors))
		var mrLogs []db.GitCommitLog
		if len(members) > 0 {
			condition := "created_at >= ? AND created_at < ? AND action IN ?"
			if db.DB.Dialector.Name() == "sqlite" {
				condition = "julianday(created_at) >= julianday(?) AND julianday(created_at) < julianday(?) AND action IN ?"
			}
			if err := db.DB.WithContext(ctx).Where(condition, yesterday.UTC(), today.UTC(), []string{"mr_open", "mr_update", "mr_merge"}).Order("id").Limit(20001).Find(&mrLogs).Error; err != nil {
				return report, fmt.Errorf("cannot load yesterday MR activity")
			}
			if len(mrLogs) > 20000 {
				return report, fmt.Errorf("yesterday MR activity exceeds report limit")
			}
		}
		seenMR := map[string]bool{}
		for _, entry := range mrLogs {
			if entry.MrIID <= 0 || ambiguous(entry.Author) || (filter.enabled && !filter.includesAssignee(entry.Author, directory)) {
				continue
			}
			seenMR[fmt.Sprintf("%s:%d", entry.Repo, entry.MrIID)] = true
		}
		report.ActivityChart = &emailChart{Title: "昨日 Commit / MR", Description: "按报告时区昨日采集时间统计；Commit 按仓库与 SHA 去重，MR 按仓库与 IID 去重，涵盖新建、更新和合并事件。", Total: summary.Count + len(seenMR), Bars: []emailChartBar{{Label: "Commit", Count: summary.Count}, {Label: "MR", Count: len(seenMR)}}}

		report.Commits = summary
		report.Semantics += " commit 使用本地 GitCommitLog.created_at 采集时间，非源代码提交时间。"
	}
	sort.Slice(report.CoreMembers, func(i, j int) bool { return report.CoreMembers[i].Name < report.CoreMembers[j].Name })
	expand := func(value string) string {
		return strings.NewReplacer("{{date}}", report.Date, "{{timezone}}", report.Timezone).Replace(value)
	}
	report.Subject = expand(settings.Template.Subject)
	report.Introduction = expand(settings.Template.Introduction)
	report.Closing = expand(settings.Template.Closing)
	if settings.Template.Style == "custom" {
		sanitized, err := sanitizeEmailTemplateHTML(settings.Template.HTML)
		if err != nil {
			return report, err
		}
		report.CustomHTML = template.HTML(sanitized)
	}
	if len(settings.ProjectGroups) > 0 {
		report.YesterdayGroups, report.YesterdayUngrouped = groupIssues(report.YesterdayUpdated, settings.ProjectGroups)
		report.UnresolvedGroups, report.UnresolvedUngrouped = groupIssues(report.RecentUnresolved, settings.ProjectGroups)
		prepareEmailProjectGroupStats(&report, settings.ProjectGroups)
	}
	if err := renderEmailReport(&report); err != nil {
		return report, err
	}
	return report, nil
}

func expandEmailTemplateHTML(source string, report *emailReport) string {
	source = strings.NewReplacer("{{date}}", report.Date, "{{timezone}}", report.Timezone).Replace(source)
	source = strings.ReplaceAll(source, "{{introduction}}", emailTemplateTextToHTML(report.Introduction))
	source = strings.ReplaceAll(source, "{{closing}}", emailTemplateTextToHTML(report.Closing))
	sections := map[string]string{
		"{{yesterday}}":      "yesterday",
		"{{unresolved}}":     "unresolved",
		"{{owners}}":         "owners",
		"{{commits}}":        "commits",
		"{{charts}}":         "top_charts",
		"{{overview_cards}}": "overview_cards",
		"{{analysis}}":       "analysis",
		"{{warnings}}":       "warnings",
	}
	for placeholder, name := range sections {
		if !strings.Contains(source, placeholder) {
			continue
		}
		var section bytes.Buffer
		if err := dailyEmailHTML.ExecuteTemplate(&section, name, report); err != nil {
			continue
		}
		source = strings.ReplaceAll(source, placeholder, section.String())
	}
	return source
}

func emailTemplateTextToHTML(text string) string {
	return strings.ReplaceAll(html.EscapeString(text), "\n", "<br>")
}

// Email links must be absolute web URLs; a missing or invalid Jira setting
// leaves the issue key readable without creating a broken or unsafe link.
func (s *Server) emailIssuePageURL(issueKey string) string {
	if s.config == nil {
		return ""
	}
	base, err := url.Parse(strings.TrimSpace(s.config.Jira.BaseURL))
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.ForceQuery || strings.Contains(s.config.Jira.BaseURL, "#") {
		return ""
	}
	return s.jiraIssuePageURL(issueKey)
}

func renderEmailReport(report *emailReport) error {
	if report.Style == "" {
		report.Style = "brief"
	}
	if err := config.ValidateEmailTemplateStyle(report.Style); err != nil {
		return err
	}
	if report.Style == "custom" && report.CustomHTML == "" {
		return fmt.Errorf("custom email template requires html")
	}
	if len(report.ProjectGroupStats) == 0 && (len(report.YesterdayGroups) > 0 || len(report.UnresolvedGroups) > 0) {
		prepareEmailProjectGroupStats(report, nil)
	}
	prepareEmailCharts(report)
	prepareEmailCategoryStats(report)
	if report.Style == "custom" {
		report.CustomHTML = template.HTML(expandEmailTemplateHTML(string(report.CustomHTML), report))
	}
	var html bytes.Buffer
	if err := dailyEmailHTML.ExecuteTemplate(&html, report.Style, report); err != nil {
		return fmt.Errorf("cannot render email")
	}
	report.HTML = html.String()
	var text strings.Builder
	fmt.Fprintf(&text, "%s\n%s\n%s\n", report.Subject, report.Introduction, report.Semantics)
	if report.ConfluenceURL != "" {
		fmt.Fprintf(&text, "Confluence 早报归档：%s\n", report.ConfluenceURL)
	}
	for _, warning := range report.Warnings {
		fmt.Fprintln(&text, warning)
	}
	if len(report.ProjectGroupStats) > 0 {
		fmt.Fprintln(&text, "\n项目分组概览")
		for _, g := range report.ProjectGroupStats {
			rate := "—"
			if g.YesterdayCount > 0 {
				rate = fmt.Sprintf("%d%%", g.ResolutionRate)
			}
			commitInfo := ""
			if report.Commits != nil {
				commitInfo = fmt.Sprintf("；提交 %d", g.CommitCount)
			}
			fmt.Fprintf(&text, "%s（负责人：%s）：项目数 %d；昨日更新 %d；解决率 %s；近3天待跟进 %d%s\n", g.Name, g.OwnersLabel, g.ProjectCount, g.YesterdayCount, rate, g.RecentUnresolvedCount, commitInfo)
		}
	} else if len(report.CategoryStats) > 0 {
		fmt.Fprintln(&text, "\nFMS / GPP 分类统计")
		for _, cat := range report.CategoryStats {
			rate := "—"
			if cat.YesterdayCount > 0 {
				rate = fmt.Sprintf("%d%%", cat.ResolutionRate)
			}
			fmt.Fprintf(&text, "%s：%s（昨日更新 %d，已解决 %d，待跟进 %d，解决率 %s）\n", cat.Name, cat.FullName, cat.YesterdayCount, cat.ResolvedCount, cat.UnresolvedCount, rate)
		}
	}
	fmt.Fprintln(&text, "\nJira 图表分析")
	for _, analysis := range report.Analysis {
		fmt.Fprintln(&text, analysis)
	}
	fmt.Fprintln(&text, report.DonutChartDesc)
	fmt.Fprintf(&text, "\n近7日更新分布：%s\n", report.CurveChartDesc)
	for _, point := range report.TrendBars {
		fmt.Fprintf(&text, "%s：%d 项\n", point.Label, point.Count)
	}
	for _, chart := range report.JiraCharts {
		fmt.Fprintf(&text, "\n%s（共%d条）\n", chart.Title, chart.Total)
		for _, bar := range chart.Bars {
			fmt.Fprintf(&text, "%s：%d（%d%%）\n", bar.Label, bar.Count, bar.Percent)
		}
	}
	formatIssue := func(item emailIssue) {
		catTag := ""
		if item.Category != "" {
			catTag = "[" + item.Category + "] "
		}
		fmt.Fprintf(&text, "%s | %s%s | %s | %s", item.TaskID, catTag, item.Title, item.Assignee, item.Status)
		if item.URL != "" {
			fmt.Fprintf(&text, " | %s", item.URL)
		}
		if item.UpdateSummary != "" {
			fmt.Fprintf(&text, "\n  更新：%s", item.UpdateSummary)
		}
		fmt.Fprintln(&text)
	}
	formatIssuesSection := func(title string, totalCount int, groups []emailIssueGroup, ungrouped []emailIssue, allIssues []emailIssue) {
		fmt.Fprintf(&text, "\n%s（%d）\n", title, totalCount)
		if len(groups) > 0 && totalCount > 0 {
			for _, g := range groups {
				if len(g.Issues) == 0 {
					continue
				}
				ownerInfo := ""
				if g.OwnersLabel != "" {
					ownerInfo = fmt.Sprintf("（负责人：%s）", g.OwnersLabel)
				}
				fmt.Fprintf(&text, "\n【%s%s】（%d）\n", g.Name, ownerInfo, len(g.Issues))
				for _, item := range g.Issues {
					formatIssue(item)
				}
			}
			if len(ungrouped) > 0 {
				fmt.Fprintf(&text, "\n【其他项目 / 未分组】（%d）\n", len(ungrouped))
				for _, item := range ungrouped {
					formatIssue(item)
				}
			}
		} else {
			for _, item := range allIssues {
				formatIssue(item)
			}
		}
	}
	formatIssuesSection("昨日更新 Jira", len(report.YesterdayUpdated), report.YesterdayGroups, report.YesterdayUngrouped, report.YesterdayUpdated)
	formatIssuesSection("近3天创建且未解决", len(report.RecentUnresolved), report.UnresolvedGroups, report.UnresolvedUngrouped, report.RecentUnresolved)
	if len(report.ProjectGroupStats) == 0 {
		fmt.Fprintln(&text, "\nCoremember 负责人")
		for _, member := range report.CoreMembers {
			fmt.Fprintf(&text, "%s：昨日更新 %d；近3天未解决 %d", member.Name, member.YesterdayCount, member.UnresolvedCount)
			if report.Commits != nil {
				fmt.Fprintf(&text, "；提交 %d", member.CommitCount)
			}
			fmt.Fprintln(&text)
		}
	}
	if report.Commits != nil {
		fmt.Fprintln(&text, report.Commits.Analysis)
		for _, row := range report.Commits.Rows {
			fmt.Fprintf(&text, "仓库：%s | 分支：%s | 提交：%d\n", row.Repository, row.Branch, row.Count)
			for _, c := range row.Commits {
				fmt.Fprintf(&text, "%s | %s | %s | %s\n", c.ID, c.Author, c.Message, c.URL)
			}
		}
		if report.CommitChart != nil {
			fmt.Fprintln(&text, report.CommitChart.Title)
			for _, bar := range report.CommitChart.Bars {
				fmt.Fprintf(&text, "%s：%d（%d%%）\n", bar.Label, bar.Count, bar.Percent)
			}
		}
	}
	fmt.Fprintln(&text, report.Closing)
	report.Text = text.String()
	return nil
}

func prepareEmailCategoryStats(report *emailReport) {
	if len(report.YesterdayUpdated) == 0 && len(report.RecentUnresolved) == 0 {
		report.CategoryStats = nil
		return
	}
	fmsYesterday, fmsResolved, fmsUnresolved := 0, 0, 0
	gppYesterday, gppResolved, gppUnresolved := 0, 0, 0

	for i := range report.YesterdayUpdated {
		issue := &report.YesterdayUpdated[i]
		if issue.Category == "GPP" {
			gppYesterday++
			if isResolvedDailyJiraStatus(issue.Status) {
				gppResolved++
			}
		} else if issue.Category == "FMS" {
			fmsYesterday++
			if isResolvedDailyJiraStatus(issue.Status) {
				fmsResolved++
			}
		}
	}

	for i := range report.RecentUnresolved {
		issue := &report.RecentUnresolved[i]
		if issue.Category == "GPP" {
			gppUnresolved++
		} else if issue.Category == "FMS" {
			fmsUnresolved++
		}
	}

	fmsRate := 0
	if fmsYesterday > 0 {
		fmsRate = (fmsResolved*100 + fmsYesterday/2) / fmsYesterday
		if fmsRate > 100 {
			fmsRate = 100
		}
	}
	gppRate := 0
	if gppYesterday > 0 {
		gppRate = (gppResolved*100 + gppYesterday/2) / gppYesterday
		if gppRate > 100 {
			gppRate = 100
		}
	}

	report.CategoryStats = []emailCategoryStat{
		{
			Name:            "FMS",
			FullName:        "FMS 业务调度",
			Description:     "车队任务调度 · 状态机同步 · 现场作业协同",
			YesterdayCount:  fmsYesterday,
			ResolvedCount:   fmsResolved,
			UnresolvedCount: fmsUnresolved,
			ResolutionRate:  fmsRate,
		},
		{
			Name:            "GPP",
			FullName:        "GPP 路径规划",
			Description:     "长短路径规划 · 道路约束权重 · 冲突规避与路权",
			YesterdayCount:  gppYesterday,
			ResolvedCount:   gppResolved,
			UnresolvedCount: gppUnresolved,
			ResolutionRate:  gppRate,
		},
	}
}

func formatDailyJiraIssueType(raw string) string {
	t := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(t, "bug") || strings.Contains(t, "缺陷") || strings.Contains(t, "故障"):
		return "缺陷"
	case strings.Contains(t, "task") || strings.Contains(t, "任务"):
		return "任务"
	case strings.Contains(t, "demand") || strings.Contains(t, "requirement") || strings.Contains(t, "需求"):
		return "需求"
	case t == "":
		return "任务"
	default:
		return raw
	}
}

func extractIssueProjectKey(taskID string) string {
	taskID = strings.TrimSpace(taskID)
	if idx := strings.Index(taskID, "-"); idx > 0 {
		return strings.ToUpper(strings.TrimSpace(taskID[:idx]))
	}
	return ""
}

func issueMatchesGroup(taskID, title, assignee string, group config.DailyJiraProjectGroup) int {
	projKey := extractIssueProjectKey(taskID)
	normAssignee := strings.ToLower(normalizeAssignee(assignee))

	if projKey != "" {
		for _, p := range group.Projects {
			pTrim := strings.TrimSpace(p)
			if pTrim != "" && strings.EqualFold(projKey, pTrim) {
				score := 100
				for _, o := range group.Owners {
					normOwner := strings.ToLower(normalizeAssignee(o))
					if normAssignee != "" && normOwner != "" && normAssignee == normOwner {
						score += 10
						break
					}
				}
				return score
			}
		}
	}

	if len(group.Projects) == 0 {
		for _, o := range group.Owners {
			normOwner := strings.ToLower(normalizeAssignee(o))
			if normAssignee != "" && normOwner != "" && normAssignee == normOwner {
				return 10
			}
		}
	}

	return 0
}

func groupIssues(issues []emailIssue, groups []config.DailyJiraProjectGroup) ([]emailIssueGroup, []emailIssue) {
	if len(groups) == 0 {
		return nil, issues
	}
	result := make([]emailIssueGroup, len(groups))
	for i, g := range groups {
		result[i] = emailIssueGroup{
			Name:        g.Name,
			Owners:      g.Owners,
			OwnersLabel: strings.Join(g.Owners, "、"),
			Projects:    g.Projects,
			Issues:      []emailIssue{},
		}
	}
	var ungrouped []emailIssue

	claimedProjects := make(map[string]int)
	for i, g := range groups {
		for _, p := range g.Projects {
			pTrim := strings.ToUpper(strings.TrimSpace(p))
			if pTrim != "" {
				if _, exists := claimedProjects[pTrim]; !exists {
					claimedProjects[pTrim] = i
				}
			}
		}
	}

	for issueIdx := range issues {
		issue := issues[issueIdx]
		bestIdx := -1
		bestScore := 0

		projKey := extractIssueProjectKey(issue.TaskID)
		if claimantIdx, claimed := claimedProjects[projKey]; claimed {
			bestIdx = claimantIdx
			bestScore = 100
		} else {
			normAssignee := strings.ToLower(normalizeAssignee(issue.Assignee))
			if normAssignee != "" {
				for i, g := range groups {
					if len(g.Projects) > 0 {
						continue
					}
					for _, o := range g.Owners {
						normOwner := strings.ToLower(normalizeAssignee(o))
						if normOwner != "" && normAssignee == normOwner {
							if bestIdx == -1 {
								bestIdx = i
								bestScore = 10
							}
							break
						}
					}
				}
			}
		}

		if bestIdx >= 0 && bestScore > 0 {
			issueCopy := issue
			issueCopy.GroupName = result[bestIdx].Name
			issues[issueIdx].GroupName = result[bestIdx].Name
			result[bestIdx].Issues = append(result[bestIdx].Issues, issueCopy)
			result[bestIdx].Count++
		} else {
			ungrouped = append(ungrouped, issue)
		}
	}

	return result, ungrouped
}

func prepareEmailProjectGroupStats(report *emailReport, groups []config.DailyJiraProjectGroup) {
	if len(groups) == 0 && len(report.YesterdayGroups) == 0 && len(report.UnresolvedGroups) == 0 {
		return
	}

	if len(groups) == 0 {
		seen := map[string]bool{}
		for _, g := range report.YesterdayGroups {
			if !seen[g.Name] && g.Name != "其他项目 / 未分组" {
				seen[g.Name] = true
				groups = append(groups, config.DailyJiraProjectGroup{
					Name:     g.Name,
					Owners:   g.Owners,
					Projects: g.Projects,
				})
			}
		}
		for _, g := range report.UnresolvedGroups {
			if !seen[g.Name] && g.Name != "其他项目 / 未分组" {
				seen[g.Name] = true
				groups = append(groups, config.DailyJiraProjectGroup{
					Name:     g.Name,
					Owners:   g.Owners,
					Projects: g.Projects,
				})
			}
		}
	}

	stats := make([]emailProjectGroupStat, 0, len(groups)+1)
	for _, g := range groups {
		ownersLabel := strings.Join(g.Owners, "、")
		if ownersLabel == "" {
			ownersLabel = "未指定"
		}
		projectsLabel := strings.Join(g.Projects, ", ")

		yCount := 0
		resolvedCount := 0
		for _, yg := range report.YesterdayGroups {
			if yg.Name == g.Name {
				yCount = yg.Count
				for _, issue := range yg.Issues {
					if isResolvedDailyJiraStatus(issue.Status) {
						resolvedCount++
					}
				}
				break
			}
		}

		recentUnresolved := 0
		for _, ug := range report.UnresolvedGroups {
			if ug.Name == g.Name {
				recentUnresolved = ug.Count
				break
			}
		}

		rate := 0
		if yCount > 0 {
			rate = (resolvedCount*100 + yCount/2) / yCount
			if rate > 100 {
				rate = 100
			}
		}

		projSet := map[string]struct{}{}
		for _, p := range g.Projects {
			pTrim := strings.TrimSpace(p)
			if pTrim != "" {
				projSet[strings.ToUpper(pTrim)] = struct{}{}
			}
		}
		if len(projSet) == 0 {
			for _, yg := range report.YesterdayGroups {
				if yg.Name == g.Name {
					for _, issue := range yg.Issues {
						if k := extractIssueProjectKey(issue.TaskID); k != "" {
							projSet[k] = struct{}{}
						}
					}
					break
				}
			}
			for _, ug := range report.UnresolvedGroups {
				if ug.Name == g.Name {
					for _, issue := range ug.Issues {
						if k := extractIssueProjectKey(issue.TaskID); k != "" {
							projSet[k] = struct{}{}
						}
					}
					break
				}
			}
		}
		projectCount := len(projSet)
		if projectCount == 0 && (yCount > 0 || recentUnresolved > 0) {
			projectCount = 1
		}

		commitCount := 0
		if report.Commits != nil && len(report.Commits.Authors) > 0 {
			for _, owner := range g.Owners {
				normOwner := strings.ToLower(normalizeAssignee(owner))
				for author, count := range report.Commits.Authors {
					if strings.ToLower(normalizeAssignee(author)) == normOwner {
						commitCount += count
					}
				}
			}
		}

		stats = append(stats, emailProjectGroupStat{
			Name:                  g.Name,
			Owners:                g.Owners,
			OwnersLabel:           ownersLabel,
			Projects:              g.Projects,
			ProjectsLabel:         projectsLabel,
			ProjectCount:          projectCount,
			YesterdayCount:        yCount,
			ResolvedCount:         resolvedCount,
			UnresolvedCount:       yCount - resolvedCount,
			ResolutionRate:        rate,
			RecentUnresolvedCount: recentUnresolved,
			CommitCount:           commitCount,
		})
	}

	if len(report.YesterdayUngrouped) > 0 || len(report.UnresolvedUngrouped) > 0 {
		ungroupedResolved := 0
		ungroupedProjSet := map[string]struct{}{}
		for _, issue := range report.YesterdayUngrouped {
			if isResolvedDailyJiraStatus(issue.Status) {
				ungroupedResolved++
			}
			if k := extractIssueProjectKey(issue.TaskID); k != "" {
				ungroupedProjSet[k] = struct{}{}
			}
		}
		for _, issue := range report.UnresolvedUngrouped {
			if k := extractIssueProjectKey(issue.TaskID); k != "" {
				ungroupedProjSet[k] = struct{}{}
			}
		}
		ungroupedProjectCount := len(ungroupedProjSet)
		if ungroupedProjectCount == 0 && (len(report.YesterdayUngrouped) > 0 || len(report.UnresolvedUngrouped) > 0) {
			ungroupedProjectCount = 1
		}

		yCount := len(report.YesterdayUngrouped)
		rate := 0
		if yCount > 0 {
			rate = (ungroupedResolved*100 + yCount/2) / yCount
			if rate > 100 {
				rate = 100
			}
		}
		stats = append(stats, emailProjectGroupStat{
			Name:                  "其他项目 / 未分组",
			Owners:                []string{},
			OwnersLabel:           "未指定分组",
			Projects:              []string{},
			ProjectsLabel:         "其他项目",
			ProjectCount:          ungroupedProjectCount,
			YesterdayCount:        yCount,
			ResolvedCount:         ungroupedResolved,
			UnresolvedCount:       yCount - ungroupedResolved,
			ResolutionRate:        rate,
			RecentUnresolvedCount: len(report.UnresolvedUngrouped),
			CommitCount:           0,
		})
	}

	report.ProjectGroupStats = stats
}
