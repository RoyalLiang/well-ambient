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
	"well-ambient/internal/deliveryplanning"
)

type ExecutionTasksResponseDTO struct {
	GeneratedAt string                 `json:"generated_at"`
	Summary     ExecutionSummaryDTO    `json:"summary"`
	Facets      ExecutionFacetsDTO     `json:"facets"`
	Items       []ExecutionTaskItemDTO `json:"items"`
}

type ExecutionFacetsDTO struct {
	Projects  []ExecutionProjectOptionDTO  `json:"projects"`
	Assignees []ExecutionAssigneeOptionDTO `json:"assignees"`
}

type ExecutionProjectOptionDTO struct {
	ProjectKey  string `json:"project_key"`
	ProjectName string `json:"project_name"`
}

type ExecutionAssigneeOptionDTO struct {
	Value      string   `json:"value"`
	Label      string   `json:"label"`
	Department string   `json:"department,omitempty"`
	Aliases    []string `json:"aliases,omitempty"`
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
	Source            string   `json:"source,omitempty"`
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
	ParentWorkItemID  string   `json:"parent_work_item_id,omitempty"`
	ParentWorkItem    string   `json:"parent_work_item,omitempty"`
	ParentIssueType   string   `json:"parent_issue_type,omitempty"`
	ProjectKey        string   `json:"project_key,omitempty"`
	TargetReleaseID   uint     `json:"target_release_id,omitempty"`
	TargetRelease     string   `json:"target_release,omitempty"`
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

var executionWorkItemKinds = []string{
	"requirement", "demand", "story", "需求", "user story", "product requirement",
	"bug", "defect", "缺陷", "故障",
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

	projectFilters := queryFilterValues(r, "project")
	assigneeFilters := queryFilterValues(r, "assignee")
	searchFilter := r.URL.Query().Get("search")
	riskFilter := r.URL.Query().Get("risk")
	visibility, users, err := s.loadCoreMemberVisibility()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query users for execution tasks: %v", err), http.StatusInternalServerError)
		return
	}

	tx := db.DB.Model(&db.TaskTelemetry{}).
		Where("LOWER(TRIM(issue_type)) IN ?", []string{
			"execution_task", "execution-task", "task", "sub-task", "subtask", "执行任务",
		})
	tx, err = applyRequestProjectScope(tx, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to apply project preferences: %v", err), http.StatusInternalServerError)
		return
	}

	var tasks []db.TaskTelemetry
	if err := tx.Find(&tasks).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to query execution tasks: %v", err), http.StatusInternalServerError)
		return
	}
	tasks = filterTaskTelemetriesByQuery(tasks, projectFilters, nil)
	executionTasks := make([]db.TaskTelemetry, 0, len(tasks))
	for _, task := range tasks {
		if !isCommitDerivedTelemetryTask(task) {
			executionTasks = append(executionTasks, task)
		}
	}
	tasks = append([]db.TaskTelemetry(nil), executionTasks...)

	// Git 日志是执行证据，不是任务本身。把已有需求/缺陷中存在 Git 证据的条目
	// 投影到执行追踪，同时拒绝为只有 commit 日志、没有 WorkItem 的 key 造任务。
	var evidenceTaskIDs []string
	if err := db.DB.Model(&db.GitCommitLog{}).
		Distinct("task_id").
		Where("TRIM(task_id) <> ''").
		Pluck("task_id", &evidenceTaskIDs).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to query execution evidence keys: %v", err), http.StatusInternalServerError)
		return
	}
	if len(evidenceTaskIDs) > 0 {
		evidenceQuery := db.DB.Model(&db.TaskTelemetry{}).
			Where("LOWER(TRIM(issue_type)) IN ?", executionWorkItemKinds).
			Where("task_id IN ?", evidenceTaskIDs)
		evidenceQuery, err = applyRequestProjectScope(evidenceQuery, r)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to apply project preferences: %v", err), http.StatusInternalServerError)
			return
		}
		var evidenceWorkItems []db.TaskTelemetry
		if err := evidenceQuery.Find(&evidenceWorkItems).Error; err != nil {
			http.Error(w, fmt.Sprintf("Failed to query evidence-backed work items: %v", err), http.StatusInternalServerError)
			return
		}
		evidenceWorkItems = filterTaskTelemetriesByQuery(evidenceWorkItems, projectFilters, nil)
		tasks = append(tasks, evidenceWorkItems...)
	}

	parentIDs := make([]string, 0)
	groupIDs := make([]string, 0)
	for _, task := range executionTasks {
		if parentID := strings.TrimSpace(task.ParentWorkItemID); parentID != "" {
			parentIDs = append(parentIDs, parentID)
		}
		if groupID := normalizedTaskGroupID(task.TaskGroupID); groupID != "" {
			groupIDs = append(groupIDs, groupID)
		}
	}
	if len(parentIDs) > 0 || len(groupIDs) > 0 {
		parentQuery := db.DB.Model(&db.TaskTelemetry{}).
			Where("LOWER(TRIM(issue_type)) IN ?", executionWorkItemKinds)
		if len(parentIDs) > 0 && len(groupIDs) > 0 {
			parentQuery = parentQuery.Where("task_id IN ? OR task_group_id IN ?", parentIDs, groupIDs)
		} else if len(parentIDs) > 0 {
			parentQuery = parentQuery.Where("task_id IN ?", parentIDs)
		} else {
			parentQuery = parentQuery.Where("task_group_id IN ?", groupIDs)
		}
		var parents []db.TaskTelemetry
		if err := parentQuery.Find(&parents).Error; err != nil {
			http.Error(w, fmt.Sprintf("Failed to query parent work items: %v", err), http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, parents...)
	}

	// 4. 仅查询与这些过滤任务关联的提交日志，规避全表扫描
	taskIDSet := make(map[string]struct{}, len(tasks))
	for _, t := range executionTasks {
		if taskID := strings.TrimSpace(t.TaskID); taskID != "" {
			taskIDSet[taskID] = struct{}{}
		}
	}
	for _, task := range tasks {
		kind, kindErr := deliveryplanning.NormalizeIssueType(task.IssueType)
		if kindErr != nil || kind == deliveryplanning.ExecutionTask {
			continue
		}
		if taskID := strings.TrimSpace(task.TaskID); taskID != "" {
			taskIDSet[taskID] = struct{}{}
		}
	}
	taskIDs := make([]string, 0, len(taskIDSet))
	for taskID := range taskIDSet {
		taskIDs = append(taskIDs, taskID)
	}

	var logs []db.GitCommitLog
	if len(taskIDs) > 0 {
		if err := db.DB.Where("task_id IN ?", taskIDs).Order("created_at desc").Find(&logs).Error; err != nil {
			http.Error(w, fmt.Sprintf("Failed to query execution evidence: %v", err), http.StatusInternalServerError)
			return
		}
	}

	response := buildExecutionTasksResponse(tasks, logs, users, time.Now())
	if err := enrichExecutionPlanningFacts(&response); err != nil {
		http.Error(w, fmt.Sprintf("Failed to load execution planning facts: %v", err), http.StatusInternalServerError)
		return
	}
	response.Items = visibility.filterExecutionItems(response.Items)
	projects, err := s.executionProjectOptions(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load execution project facets: %v", err), http.StatusInternalServerError)
		return
	}
	response.Items = normalizeExecutionProjectFacts(response.Items, projects)
	response.Facets = ExecutionFacetsDTO{
		Projects:  projects,
		Assignees: s.executionAssigneeOptions(response.Items, visibility, users),
	}

	// 2. 负责人过滤必须在 DTO 构建后执行，因为绑定的 Jira Task 需求主线可能已经换了负责人。
	if len(assigneeFilters) > 0 {
		response.Items = filterExecutionItems(response.Items, func(item ExecutionTaskItemDTO) bool {
			return matchesQueryFilter(item.Assignee, assigneeFilters)
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
	workItemsByGroup := make(map[string]db.TaskTelemetry)
	workItemsByID := make(map[string]db.TaskTelemetry)
	evidenceByTask := make(map[string]executionEvidenceStats)
	items := make([]ExecutionTaskItemDTO, 0)
	summary := ExecutionSummaryDTO{}

	for _, task := range tasks {
		if isArchivedTask(task) {
			continue
		}
		kind, err := deliveryplanning.NormalizeIssueType(task.IssueType)
		if err != nil || kind == deliveryplanning.ExecutionTask {
			continue
		}
		workItemsByID[task.TaskID] = task
		groupID := normalizedTaskGroupID(task.TaskGroupID)
		if groupID != "" {
			workItemsByGroup[groupID] = task
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
		kind, err := deliveryplanning.NormalizeIssueType(task.IssueType)
		if isArchivedTask(task) || err != nil || kind != deliveryplanning.ExecutionTask {
			continue
		}
		issueType := deliveryplanning.ExecutionTask

		groupID := normalizedTaskGroupID(task.TaskGroupID)
		parentDemand := workItemsByID[strings.TrimSpace(task.ParentWorkItemID)]
		if parentDemand.TaskID == "" {
			parentDemand = workItemsByGroup[groupID]
		}
		stats := evidenceByTask[task.TaskID]
		displayAssignee, executionAssignee, jiraAssignee := resolveExecutionAssignees(task, parentDemand)
		effectiveStatus, jiraStatus := resolveExecutionStatus(task, parentDemand)
		risk := resolveExecutionRisk(task, parentDemand, stats, now)
		identity := directory.resolve(displayAssignee)
		if identity.IsLinked && identity.Name != "" {
			displayAssignee = identity.Name
		}
		projectOwner := parentDemand
		if projectOwner.TaskID == "" {
			projectOwner = task
		}
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
			Source:            strings.ToLower(strings.TrimSpace(task.Source)),
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
			ParentWorkItemID:  strings.TrimSpace(parentDemand.TaskID),
			ParentWorkItem:    strings.TrimSpace(parentDemand.Title),
			ParentIssueType:   normalizedParentIssueType(parentDemand),
			ProjectKey:        db.ResolveTaskProjectKey(projectOwner),
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

	// WorkItem 自身存在 Git 证据时，也应成为执行追踪中的一行。这里用 WorkItem
	// 作为自己的归属主线，因此不会把原始 commit 消息或无主 task key 暴露为任务。
	projectedWorkItems := make(map[string]struct{})
	for _, task := range tasks {
		kind, err := deliveryplanning.NormalizeIssueType(task.IssueType)
		taskID := strings.TrimSpace(task.TaskID)
		if isArchivedTask(task) || err != nil || kind == deliveryplanning.ExecutionTask || taskID == "" {
			continue
		}
		if _, exists := projectedWorkItems[taskID]; exists {
			continue
		}
		stats := evidenceByTask[taskID]
		if stats.LastLog == nil {
			continue
		}
		projectedWorkItems[taskID] = struct{}{}

		displayAssignee := normalizeAssignee(task.Assignee)
		identity := directory.resolve(displayAssignee)
		if identity.IsLinked && identity.Name != "" {
			displayAssignee = identity.Name
		}
		status := strings.ToLower(strings.TrimSpace(task.Status))
		risk := resolveExecutionRisk(task, task, stats, now)
		lastActivity := scheduleActivityTime(task)
		if stats.LastLog.CreatedAt.After(lastActivity) {
			lastActivity = stats.LastLog.CreatedAt
		}
		evidenceAgeHours := int(now.Sub(stats.LastLog.CreatedAt).Hours())
		if evidenceAgeHours < 0 {
			evidenceAgeHours = 0
		}
		repo := strings.TrimSpace(task.Repo)
		branch := strings.TrimSpace(task.Branch)
		if repo == "" {
			repo = strings.TrimSpace(stats.LastLog.Repo)
		}
		if branch == "" {
			branch = strings.TrimSpace(stats.LastLog.Branch)
		}
		jiraAssignee := ""
		jiraStatus := ""
		if strings.EqualFold(strings.TrimSpace(task.Source), "jira") {
			jiraAssignee = displayAssignee
			jiraStatus = status
		}

		item := ExecutionTaskItemDTO{
			TaskID:           taskID,
			Title:            strings.TrimSpace(task.Title),
			IssueType:        deliveryplanning.ExecutionTask,
			Source:           strings.ToLower(strings.TrimSpace(task.Source)),
			Assignee:         displayAssignee,
			JiraAssignee:     jiraAssignee,
			JiraStatus:       jiraStatus,
			Department:       normalizeDepartment(identity.Department),
			Repo:             repo,
			Branch:           branch,
			Status:           status,
			TaskGroupID:      normalizedTaskGroupID(task.TaskGroupID),
			ParentDemandID:   taskID,
			ParentDemand:     strings.TrimSpace(task.Title),
			ParentWorkItemID: taskID,
			ParentWorkItem:   strings.TrimSpace(task.Title),
			ParentIssueType:  kind,
			ProjectKey:       db.ResolveTaskProjectKey(task),
			CreatedAt:        formatDateTime(task.TaskCreatedAt),
			LastUpdate:       formatDateTime(lastActivity),
			LastEvidenceAt:   formatDateTime(stats.LastLog.CreatedAt),
			LastCommit:       firstNonEmpty(stats.LastCommit, strings.TrimSpace(task.LastCommit)),
			MRURL:            firstNonEmpty(stats.LastMRURL, strings.TrimSpace(task.MrURL)),
			MRIID:            firstPositive(stats.LastMRIID, task.MrIID),
			CommitCount:      stats.CommitCount,
			MRCount:          stats.MRCount,
			MergedMRCount:    stats.MergedMRCount,
			EvidenceScore:    executionEvidenceScore(task, stats),
			RiskLevel:        risk.Level,
			RiskLabel:        risk.Label,
			RiskReason:       risk.Reason,
			RiskRank:         risk.Rank,
			ResultState:      executionResultState(task, task, stats),
			ResultLabel:      executionResultLabel(task, task, stats),
			ActiveDays:       activeTaskDays(task.TaskCreatedAt, now),
			EvidenceAgeHours: evidenceAgeHours,
			RiskTags:         risk.Tags,
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

func normalizedParentIssueType(parent db.TaskTelemetry) string {
	if strings.TrimSpace(parent.TaskID) == "" {
		return ""
	}
	kind, err := deliveryplanning.NormalizeIssueType(parent.IssueType)
	if err != nil || kind == deliveryplanning.ExecutionTask {
		return ""
	}
	return kind
}

func isCommitDerivedTelemetryTask(task db.TaskTelemetry) bool {
	source := strings.ToLower(strings.TrimSpace(task.Source))
	if source == "git" || source == "gitlab" {
		return true
	}
	if source != "" || strings.TrimSpace(task.ExternalKey) != "" ||
		strings.TrimSpace(task.ParentWorkItemID) != "" || strings.TrimSpace(task.TaskGroupID) != "" {
		return false
	}
	title := strings.TrimSpace(task.Title)
	lastCommit := strings.TrimSpace(task.LastCommit)
	if title != "" && lastCommit != "" {
		firstLine := strings.TrimSpace(strings.SplitN(lastCommit, "\n", 2)[0])
		if strings.EqualFold(title, firstLine) {
			return true
		}
	}
	return strings.TrimSpace(task.ProjectKey) == "" &&
		strings.TrimSpace(task.Repo) != "" && strings.TrimSpace(task.Repo) != "-" &&
		strings.TrimSpace(task.Branch) != "" && strings.TrimSpace(task.Branch) != "-" &&
		!looksLikeJiraIssueKey(task.TaskID)
}

func looksLikeJiraIssueKey(value string) bool {
	value = strings.TrimSpace(value)
	separator := strings.LastIndex(value, "-")
	if separator <= 0 || separator == len(value)-1 {
		return false
	}
	prefix := value[:separator]
	number := value[separator+1:]
	if prefix != strings.ToUpper(prefix) {
		return false
	}
	for _, char := range prefix {
		if (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '_' {
			return false
		}
	}
	for _, char := range number {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func (s *Server) executionProjectOptions(r *http.Request) ([]ExecutionProjectOptionDTO, error) {
	available, err := s.availableProjectPreferenceOptions()
	if err != nil {
		return nil, err
	}
	selectedKeys, err := requestProjectPreferenceKeys(r)
	if err != nil {
		return nil, err
	}
	selected := make(map[string]struct{}, len(selectedKeys))
	for _, key := range selectedKeys {
		selected[strings.ToUpper(strings.TrimSpace(key))] = struct{}{}
	}
	options := make([]ExecutionProjectOptionDTO, 0, len(available))
	for _, project := range available {
		if len(selected) > 0 {
			if _, exists := selected[project.ProjectKey]; !exists {
				continue
			}
		}
		options = append(options, ExecutionProjectOptionDTO{
			ProjectKey:  project.ProjectKey,
			ProjectName: project.ProjectName,
		})
	}
	return options, nil
}

func normalizeExecutionProjectFacts(items []ExecutionTaskItemDTO, projects []ExecutionProjectOptionDTO) []ExecutionTaskItemDTO {
	knownProjects := make(map[string]struct{}, len(projects))
	for _, project := range projects {
		knownProjects[strings.ToUpper(strings.TrimSpace(project.ProjectKey))] = struct{}{}
	}
	for index := range items {
		projectKey := strings.ToUpper(strings.TrimSpace(items[index].ProjectKey))
		if _, exists := knownProjects[projectKey]; !exists {
			items[index].ProjectKey = ""
			continue
		}
		items[index].ProjectKey = projectKey
	}
	return items
}

func (s *Server) executionAssigneeOptions(_ []ExecutionTaskItemDTO, visibility coreMemberVisibility, users []userdb.User) []ExecutionAssigneeOptionDTO {
	return s.deliveryAssigneeOptions(visibility, users)
}

func enrichExecutionPlanningFacts(response *ExecutionTasksResponseDTO) error {
	if response == nil || len(response.Items) == 0 || db.DB == nil {
		return nil
	}
	parentIDs := make([]string, 0, len(response.Items))
	for _, item := range response.Items {
		if item.ParentWorkItemID != "" {
			parentIDs = append(parentIDs, item.ParentWorkItemID)
		}
	}
	if len(parentIDs) == 0 {
		return nil
	}
	type releaseFact struct {
		WorkItemID string
		ReleaseID  uint
		Name       string
	}
	var facts []releaseFact
	if err := db.DB.Table("work_item_release_links AS links").
		Select("links.work_item_id, releases.id AS release_id, releases.name").
		Joins("JOIN release_versions AS releases ON releases.id = links.release_version_id").
		Where("links.work_item_id IN ? AND links.relation = ? AND links.is_primary = ? AND links.active = ?",
			parentIDs, deliveryplanning.ReleaseTargetFix, true, true).
		Scan(&facts).Error; err != nil {
		return err
	}
	byParent := make(map[string]releaseFact, len(facts))
	for _, fact := range facts {
		byParent[fact.WorkItemID] = fact
	}
	for index := range response.Items {
		item := &response.Items[index]
		if fact, ok := byParent[item.ParentWorkItemID]; ok {
			item.TargetReleaseID = fact.ReleaseID
			item.TargetRelease = fact.Name
		}
	}
	return nil
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
