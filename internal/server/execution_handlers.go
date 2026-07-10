package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

type ExecutionTasksResponseDTO struct {
	GeneratedAt string                 `json:"generated_at"`
	Summary     ExecutionSummaryDTO    `json:"summary"`
	Items       []ExecutionTaskItemDTO `json:"items"`
}

type ExecutionSummaryDTO struct {
	Total           int `json:"total"`
	Active          int `json:"active"`
	Done            int `json:"done"`
	Bound           int `json:"bound"`
	Orphan          int `json:"orphan"`
	WithEvidence    int `json:"with_evidence"`
	MissingEvidence int `json:"missing_evidence"`
	Stale           int `json:"stale"`
	Mismatch        int `json:"mismatch"`
	HighRisk        int `json:"high_risk"`
}

type ExecutionTaskItemDTO struct {
	TaskID            string   `json:"task_id"`
	Title             string   `json:"title"`
	IssueType         string   `json:"issue_type"`
	Assignee          string   `json:"assignee"`
	ExecutionAssignee string   `json:"execution_assignee,omitempty"`
	JiraAssignee      string   `json:"jira_assignee,omitempty"`
	JiraStatus        string   `json:"jira_status,omitempty"`
	Department        string   `json:"department"`
	Repo              string   `json:"repo"`
	Branch            string   `json:"branch"`
	Status            string   `json:"status"`
	ExecutionStatus   string   `json:"execution_status,omitempty"`
	TaskGroupID       string   `json:"task_group_id"`
	ParentDemandID    string   `json:"parent_demand_id,omitempty"`
	ParentDemand      string   `json:"parent_demand,omitempty"`
	CreatedAt         string   `json:"created_at"`
	LastUpdate        string   `json:"last_update"`
	LastEvidenceAt    string   `json:"last_evidence_at,omitempty"`
	LastCommit        string   `json:"last_commit"`
	MRURL             string   `json:"mr_url,omitempty"`
	MRIID             int      `json:"mr_iid,omitempty"`
	CommitCount       int      `json:"commit_count"`
	MRCount           int      `json:"mr_count"`
	MergedMRCount     int      `json:"merged_mr_count"`
	EvidenceScore     int      `json:"evidence_score"`
	RiskLevel         string   `json:"risk_level"`
	RiskLabel         string   `json:"risk_label"`
	RiskReason        string   `json:"risk_reason"`
	RiskRank          int      `json:"risk_rank"`
	ResultState       string   `json:"result_state"`
	ResultLabel       string   `json:"result_label"`
	ActiveDays        int      `json:"active_days"`
	EvidenceAgeHours  int      `json:"evidence_age_hours"`
	RiskTags          []string `json:"risk_tags"`
}

type executionEvidenceStats struct {
	CommitCount   int
	MRCount       int
	MergedMRCount int
	LastLog       *db.GitCommitLog
	LastMRURL     string
	LastMRIID     int
	LastCommit    string
}

type executionRisk struct {
	Level  string
	Label  string
	Reason string
	Rank   int
	Tags   []string
}

// handleGetExecutionTasks returns Jira/task-level execution observability with query filters
func (s *Server) handleGetExecutionTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	projectFilter := r.URL.Query().Get("project")
	assigneeFilter := r.URL.Query().Get("assignee")
	searchFilter := r.URL.Query().Get("search")
	riskFilter := r.URL.Query().Get("risk")
	visibility, users, err := s.loadCoreMemberVisibility()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query users for execution tasks: %v", err), http.StatusInternalServerError)
		return
	}

	tx := db.DB.Model(&db.TaskTelemetry{})

	// 1. 项目前缀过滤
	if projectFilter != "" && projectFilter != "all" {
		tx = tx.Where("task_id LIKE ?", projectFilter+"-%")
	}

	var tasks []db.TaskTelemetry
	if err := tx.Find(&tasks).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to query execution tasks: %v", err), http.StatusInternalServerError)
		return
	}

	// 4. 仅查询与这些过滤任务关联的提交日志，规避全表扫描
	var taskIDs []string
	for _, t := range tasks {
		if t.TaskID != "" {
			taskIDs = append(taskIDs, t.TaskID)
		}
	}

	var logs []db.GitCommitLog
	if len(taskIDs) > 0 {
		if err := db.DB.Where("task_id IN ?", taskIDs).Order("created_at desc").Find(&logs).Error; err != nil {
			http.Error(w, fmt.Sprintf("Failed to query execution evidence: %v", err), http.StatusInternalServerError)
			return
		}
	}

	response := buildExecutionTasksResponse(tasks, logs, users, time.Now())
	response.Items = visibility.filterExecutionItems(response.Items)

	// 2. 负责人过滤必须在 DTO 构建后执行，因为绑定的 Jira Task 需求主线可能已经换了负责人。
	if assigneeFilter != "" && assigneeFilter != "all" {
		response.Items = filterExecutionItems(response.Items, func(item ExecutionTaskItemDTO) bool {
			if assigneeFilter == "外部协同" {
				return false
			}
			return strings.EqualFold(item.Assignee, assigneeFilter)
		})
	}

	// 3. 关键字匹配同样使用 DTO 字段，避免漏掉 Jira 当前负责人和主线需求信息。
	if searchFilter != "" {
		query := strings.ToLower(strings.TrimSpace(searchFilter))
		response.Items = filterExecutionItems(response.Items, func(item ExecutionTaskItemDTO) bool {
			return strings.Contains(strings.ToLower(item.TaskID), query) ||
				strings.Contains(strings.ToLower(item.Title), query) ||
				strings.Contains(strings.ToLower(item.IssueType), query) ||
				strings.Contains(strings.ToLower(item.Assignee), query) ||
				strings.Contains(strings.ToLower(item.ExecutionAssignee), query) ||
				strings.Contains(strings.ToLower(item.JiraAssignee), query) ||
				strings.Contains(strings.ToLower(item.Department), query) ||
				strings.Contains(strings.ToLower(item.Repo), query) ||
				strings.Contains(strings.ToLower(item.Branch), query) ||
				strings.Contains(strings.ToLower(item.ParentDemandID), query) ||
				strings.Contains(strings.ToLower(item.ParentDemand), query) ||
				strings.Contains(strings.ToLower(item.RiskLabel), query) ||
				strings.Contains(strings.ToLower(item.ResultLabel), query)
		})
	}

	// 4. 风险层级过滤
	if riskFilter == "attention" {
		var filteredItems []ExecutionTaskItemDTO
		for _, item := range response.Items {
			if item.RiskLevel != "safe" && item.RiskLevel != "done" {
				filteredItems = append(filteredItems, item)
			}
		}
		response.Items = filteredItems
	} else if riskFilter != "" && riskFilter != "all" {
		var filteredItems []ExecutionTaskItemDTO
		for _, item := range response.Items {
			if item.RiskLevel == riskFilter {
				filteredItems = append(filteredItems, item)
			}
		}
		response.Items = filteredItems
	}

	response.Summary = summarizeExecutionItems(response.Items)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func buildExecutionTasksResponse(tasks []db.TaskTelemetry, logs []db.GitCommitLog, users []userdb.User, now time.Time) ExecutionTasksResponseDTO {
	directory := newKPIUserDirectory(users)
	demandsByGroup := make(map[string]db.TaskTelemetry)
	evidenceByTask := make(map[string]executionEvidenceStats)
	items := make([]ExecutionTaskItemDTO, 0)
	summary := ExecutionSummaryDTO{}

	for _, task := range tasks {
		if isArchivedTask(task) || normalizeIssueType(task.IssueType) != "demand" {
			continue
		}
		groupID := normalizedTaskGroupID(task.TaskGroupID)
		if groupID != "" {
			demandsByGroup[groupID] = task
		}
	}

	for _, log := range logs {
		taskID := strings.TrimSpace(log.TaskID)
		if taskID == "" {
			continue
		}
		stats := evidenceByTask[taskID]
		if log.Action == "git_push" {
			stats.CommitCount++
			if strings.TrimSpace(log.CommitID) != "" {
				stats.LastCommit = strings.TrimSpace(log.CommitID)
			}
		}
		if strings.HasPrefix(log.Action, "mr_") {
			stats.MRCount++
			if strings.TrimSpace(log.MrURL) != "" {
				stats.LastMRURL = strings.TrimSpace(log.MrURL)
			}
			if log.MrIID > 0 {
				stats.LastMRIID = log.MrIID
			}
			if log.Action == "mr_merge" {
				stats.MergedMRCount++
			}
		}
		if stats.LastLog == nil || log.CreatedAt.After(stats.LastLog.CreatedAt) {
			logCopy := log
			stats.LastLog = &logCopy
		}
		evidenceByTask[taskID] = stats
	}

	for _, task := range tasks {
		issueType := normalizeIssueType(task.IssueType)
		if isArchivedTask(task) || issueType == "demand" {
			continue
		}

		groupID := normalizedTaskGroupID(task.TaskGroupID)
		parentDemand := demandsByGroup[groupID]
		stats := evidenceByTask[task.TaskID]
		displayAssignee, executionAssignee, jiraAssignee := resolveExecutionAssignees(task, parentDemand)
		effectiveStatus, jiraStatus := resolveExecutionStatus(task, parentDemand)
		risk := resolveExecutionRisk(task, parentDemand, stats, now)
		identity := directory.resolve(displayAssignee)
		evidenceScore := executionEvidenceScore(task, stats)
		lastEvidenceAt := ""
		evidenceAgeHours := 0
		if stats.LastLog != nil {
			lastEvidenceAt = formatDateTime(stats.LastLog.CreatedAt)
			evidenceAgeHours = int(now.Sub(stats.LastLog.CreatedAt).Hours())
			if evidenceAgeHours < 0 {
				evidenceAgeHours = 0
			}
		}

		item := ExecutionTaskItemDTO{
			TaskID:            strings.TrimSpace(task.TaskID),
			Title:             strings.TrimSpace(task.Title),
			IssueType:         issueType,
			Assignee:          displayAssignee,
			ExecutionAssignee: executionAssignee,
			JiraAssignee:      jiraAssignee,
			JiraStatus:        jiraStatus,
			Department:        normalizeDepartment(identity.Department),
			Repo:              strings.TrimSpace(task.Repo),
			Branch:            strings.TrimSpace(task.Branch),
			Status:            effectiveStatus,
			ExecutionStatus:   strings.TrimSpace(task.Status),
			TaskGroupID:       groupID,
			ParentDemandID:    strings.TrimSpace(parentDemand.TaskID),
			ParentDemand:      strings.TrimSpace(parentDemand.Title),
			CreatedAt:         formatDateTime(task.TaskCreatedAt),
			LastUpdate:        formatDateTime(executionDisplayActivityTime(task, parentDemand)),
			LastEvidenceAt:    lastEvidenceAt,
			LastCommit:        firstNonEmpty(stats.LastCommit, strings.TrimSpace(task.LastCommit)),
			MRURL:             firstNonEmpty(stats.LastMRURL, strings.TrimSpace(task.MrURL)),
			MRIID:             firstPositive(stats.LastMRIID, task.MrIID),
			CommitCount:       stats.CommitCount,
			MRCount:           stats.MRCount,
			MergedMRCount:     stats.MergedMRCount,
			EvidenceScore:     evidenceScore,
			RiskLevel:         risk.Level,
			RiskLabel:         risk.Label,
			RiskReason:        risk.Reason,
			RiskRank:          risk.Rank,
			ResultState:       executionResultState(task, parentDemand, stats),
			ResultLabel:       executionResultLabel(task, parentDemand, stats),
			ActiveDays:        activeTaskDays(task.TaskCreatedAt, now),
			EvidenceAgeHours:  evidenceAgeHours,
			RiskTags:          risk.Tags,
		}
		items = append(items, item)
		accumulateExecutionSummary(&summary, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].RiskRank != items[j].RiskRank {
			return items[i].RiskRank > items[j].RiskRank
		}
		if items[i].LastEvidenceAt != items[j].LastEvidenceAt {
			if items[i].LastEvidenceAt == "" {
				return false
			}
			if items[j].LastEvidenceAt == "" {
				return true
			}
			return items[i].LastEvidenceAt > items[j].LastEvidenceAt
		}
		return items[i].TaskID < items[j].TaskID
	})

	return ExecutionTasksResponseDTO{
		GeneratedAt: formatDateTime(now),
		Summary:     summary,
		Items:       items,
	}
}

func filterExecutionItems(items []ExecutionTaskItemDTO, keep func(ExecutionTaskItemDTO) bool) []ExecutionTaskItemDTO {
	filtered := make([]ExecutionTaskItemDTO, 0, len(items))
	for _, item := range items {
		if keep(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func summarizeExecutionItems(items []ExecutionTaskItemDTO) ExecutionSummaryDTO {
	summary := ExecutionSummaryDTO{}
	for _, item := range items {
		accumulateExecutionSummary(&summary, item)
	}
	return summary
}

func resolveExecutionAssignees(task db.TaskTelemetry, parentDemand db.TaskTelemetry) (displayAssignee string, executionAssignee string, jiraAssignee string) {
	executionAssignee = normalizeAssignee(task.Assignee)
	displayAssignee = executionAssignee
	if parentDemand.TaskID == "" {
		return displayAssignee, executionAssignee, ""
	}

	jiraAssignee = normalizeAssignee(parentDemand.Assignee)
	if jiraAssignee != "" && jiraAssignee != "未指派" {
		displayAssignee = jiraAssignee
	}
	return displayAssignee, executionAssignee, jiraAssignee
}

func resolveExecutionStatus(task db.TaskTelemetry, parentDemand db.TaskTelemetry) (effectiveStatus string, jiraStatus string) {
	taskStatus := strings.ToLower(strings.TrimSpace(task.Status))
	if parentDemand.TaskID == "" {
		return taskStatus, ""
	}
	jiraStatus = strings.ToLower(strings.TrimSpace(parentDemand.Status))
	if jiraStatus != "" {
		return jiraStatus, jiraStatus
	}
	return taskStatus, ""
}

func executionDisplayActivityTime(task db.TaskTelemetry, parentDemand db.TaskTelemetry) time.Time {
	last := scheduleActivityTime(task)
	if parentDemand.TaskID == "" {
		return last
	}
	parentLast := scheduleActivityTime(parentDemand)
	if parentLast.After(last) {
		return parentLast
	}
	return last
}

func resolveExecutionRisk(task db.TaskTelemetry, parentDemand db.TaskTelemetry, stats executionEvidenceStats, now time.Time) executionRisk {
	status, _ := resolveExecutionStatus(task, parentDemand)
	hasEvidence := hasExecutionEvidence(task, stats)
	tags := make([]string, 0, 3)

	if parentDemand.TaskID == "" {
		tags = append(tags, "orphan")
	}
	if !hasEvidence {
		tags = append(tags, "missing_evidence")
	}

	if status == "done" && !hasEvidence {
		return executionRisk{
			Level:  "high",
			Label:  "完成无证据",
			Reason: "Jira 已完成，但缺少 commit、分支或 MR 证据",
			Rank:   96,
			Tags:   tags,
		}
	}

	if stats.MergedMRCount > 0 && status != "done" {
		tags = append(tags, "state_mismatch")
		return executionRisk{
			Level:  "high",
			Label:  "状态不一致",
			Reason: "MR 已合并，但 Jira/任务状态尚未完成",
			Rank:   92,
			Tags:   tags,
		}
	}

	if status != "done" && !hasEvidence && activeTaskDays(task.TaskCreatedAt, now) >= 1 {
		return executionRisk{
			Level:  "medium",
			Label:  "未启动",
			Reason: "任务已创建超过 1 天，但没有分支、commit 或 MR 证据",
			Rank:   76,
			Tags:   tags,
		}
	}

	if status != "done" && isExecutionStale(task, stats, now) {
		tags = append(tags, "stale")
		return executionRisk{
			Level:  "medium",
			Label:  "推进停滞",
			Reason: "超过 72 小时没有新的代码或任务活动",
			Rank:   68,
			Tags:   tags,
		}
	}

	if parentDemand.TaskID == "" {
		return executionRisk{
			Level:  "medium",
			Label:  "未绑定需求",
			Reason: "执行任务没有归属需求，结果难以回流排期",
			Rank:   56,
			Tags:   tags,
		}
	}

	if status == "done" {
		return executionRisk{
			Level:  "done",
			Label:  "已闭环",
			Reason: "任务完成且具备执行证据或明确绑定关系",
			Rank:   8,
			Tags:   tags,
		}
	}

	return executionRisk{
		Level:  "safe",
		Label:  "推进中",
		Reason: "任务有执行证据，当前未命中异常规则",
		Rank:   16,
		Tags:   tags,
	}
}

func accumulateExecutionSummary(summary *ExecutionSummaryDTO, item ExecutionTaskItemDTO) {
	summary.Total++
	if item.Status == "done" {
		summary.Done++
	} else {
		summary.Active++
	}
	if item.ParentDemandID != "" {
		summary.Bound++
	} else {
		summary.Orphan++
	}
	if item.EvidenceScore > 0 {
		summary.WithEvidence++
	} else {
		summary.MissingEvidence++
	}
	for _, tag := range item.RiskTags {
		switch tag {
		case "stale":
			summary.Stale++
		case "state_mismatch":
			summary.Mismatch++
		}
	}
	if item.RiskLevel == "high" {
		summary.HighRisk++
	}
}

func hasExecutionEvidence(task db.TaskTelemetry, stats executionEvidenceStats) bool {
	if stats.CommitCount > 0 || stats.MRCount > 0 || stats.MergedMRCount > 0 {
		return true
	}
	return hasScheduleBranch(task.Branch) ||
		strings.TrimSpace(task.LastCommit) != "" && strings.TrimSpace(task.LastCommit) != "-" ||
		strings.TrimSpace(task.MrURL) != "" ||
		task.MrIID > 0
}

func executionEvidenceScore(task db.TaskTelemetry, stats executionEvidenceStats) int {
	score := 0
	if hasScheduleBranch(task.Branch) {
		score += 25
	}
	if strings.TrimSpace(task.LastCommit) != "" && strings.TrimSpace(task.LastCommit) != "-" {
		score += 20
	}
	if stats.CommitCount > 0 {
		score += 25
	}
	if stats.MRCount > 0 || strings.TrimSpace(task.MrURL) != "" || task.MrIID > 0 {
		score += 20
	}
	if stats.MergedMRCount > 0 {
		score += 10
	}
	if score > 100 {
		return 100
	}
	return score
}

func executionResultState(task db.TaskTelemetry, parentDemand db.TaskTelemetry, stats executionEvidenceStats) string {
	status, _ := resolveExecutionStatus(task, parentDemand)
	if status == "done" && stats.MergedMRCount > 0 {
		return "merged_done"
	}
	if status == "done" {
		return "jira_done"
	}
	if stats.MergedMRCount > 0 {
		return "merged_waiting_jira"
	}
	if stats.MRCount > 0 {
		return "mr_active"
	}
	if stats.CommitCount > 0 || hasScheduleBranch(task.Branch) {
		return "coding"
	}
	return "not_started"
}

func executionResultLabel(task db.TaskTelemetry, parentDemand db.TaskTelemetry, stats executionEvidenceStats) string {
	switch executionResultState(task, parentDemand, stats) {
	case "merged_done":
		return "MR 合并且已完成"
	case "jira_done":
		return "Jira 已完成"
	case "merged_waiting_jira":
		return "MR 已合并待回写"
	case "mr_active":
		return "MR 处理中"
	case "coding":
		return "开发推进中"
	default:
		return "暂无开发证据"
	}
}

func isExecutionStale(task db.TaskTelemetry, stats executionEvidenceStats, now time.Time) bool {
	last := scheduleActivityTime(task)
	if stats.LastLog != nil && stats.LastLog.CreatedAt.After(last) {
		last = stats.LastLog.CreatedAt
	}
	if last.IsZero() {
		return false
	}
	return now.Sub(last) > 72*time.Hour
}

func activeTaskDays(createdAt time.Time, now time.Time) int {
	if createdAt.IsZero() || now.Before(createdAt) {
		return 0
	}
	return int(now.Sub(createdAt).Hours() / 24)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "-" {
			return value
		}
	}
	return ""
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
