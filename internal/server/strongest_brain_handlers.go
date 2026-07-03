package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

type StrongestBrainDecisionQueueResponse struct {
	GeneratedAt string                         `json:"generated_at"`
	Summary     StrongestBrainDecisionSummary  `json:"summary"`
	Items       []StrongestBrainDecisionItem   `json:"items"`
	Evidence    []StrongestBrainEvidenceDigest `json:"evidence"`
}

type StrongestBrainDecisionSummary struct {
	Total         int `json:"total"`
	Critical      int `json:"critical"`
	Warning       int `json:"warning"`
	Open          int `json:"open"`
	ScheduleRisks int `json:"schedule_risks"`
	EvidenceRisks int `json:"evidence_risks"`
	ContextGaps   int `json:"context_gaps"`
}

type StrongestBrainDecisionItem struct {
	ID              string   `json:"id"`
	TaskID          string   `json:"task_id"`
	Title           string   `json:"title"`
	Problem         string   `json:"problem"`
	Evidence        []string `json:"evidence"`
	SuggestedAction string   `json:"suggested_action"`
	ImpactScope     string   `json:"impact_scope"`
	JumpLabel       string   `json:"jump_label"`
	JumpURL         string   `json:"jump_url"`
	RiskLevel       string   `json:"risk_level"`
	RiskType        string   `json:"risk_type"`
	Status          string   `json:"status"`
	Assignee        string   `json:"assignee"`
	Project         string   `json:"project"`
	IssueType       string   `json:"issue_type"`
	UpdatedAt       string   `json:"updated_at"`
	Source          string   `json:"source"`
	Rank            int      `json:"rank"`
}

type StrongestBrainEvidenceDigest struct {
	TaskID       string   `json:"task_id"`
	TaskGroupID  string   `json:"task_group_id"`
	CommitCount  int      `json:"commit_count"`
	MRCount      int      `json:"mr_count"`
	LastEvidence string   `json:"last_evidence"`
	Signals      []string `json:"signals"`
}

type StrongestBrainEvidenceChainResponse struct {
	GeneratedAt string                     `json:"generated_at"`
	TaskID      string                     `json:"task_id"`
	TaskGroupID string                     `json:"task_group_id"`
	Root        *StrongestBrainChainTask   `json:"root,omitempty"`
	Related     []StrongestBrainChainTask  `json:"related"`
	Evidence    []StrongestBrainChainLog   `json:"evidence"`
	Summary     StrongestBrainChainSummary `json:"summary"`
}

type StrongestBrainChainTask struct {
	TaskID      string `json:"task_id"`
	Title       string `json:"title"`
	IssueType   string `json:"issue_type"`
	Status      string `json:"status"`
	Assignee    string `json:"assignee"`
	Repo        string `json:"repo"`
	Branch      string `json:"branch"`
	DueDate     string `json:"due_date,omitempty"`
	TaskGroupID string `json:"task_group_id"`
}

type StrongestBrainChainLog struct {
	ID        uint   `json:"id"`
	TaskID    string `json:"task_id"`
	Action    string `json:"action"`
	Repo      string `json:"repo"`
	Branch    string `json:"branch"`
	CommitID  string `json:"commit_id"`
	MRURL     string `json:"mr_url"`
	CreatedAt string `json:"created_at"`
}

type StrongestBrainChainSummary struct {
	RelatedTasks  int      `json:"related_tasks"`
	Commits       int      `json:"commits"`
	MergeRequests int      `json:"merge_requests"`
	MergedMRs     int      `json:"merged_mrs"`
	Signals       []string `json:"signals"`
}

type AIIntentRequest struct {
	Text     string            `json:"text"`
	Messages []AIIntentMessage `json:"messages"`
	Mode     string            `json:"mode"`
}

type AIIntentMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIIntentResponse struct {
	Intent          string   `json:"intent"`
	IntentLabel     string   `json:"intent_label"`
	Confidence      float64  `json:"confidence"`
	Summary         string   `json:"summary"`
	SuggestedAction string   `json:"suggested_action"`
	MissingContext  []string `json:"missing_context"`
	NextQuestions   []string `json:"next_questions"`
	Facts           []string `json:"facts"`
	Inferences      []string `json:"inferences"`
	RoutedTo        string   `json:"routed_to"`
	Source          string   `json:"source"`
}

func (s *Server) handleGetStrongestBrainDecisionQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	now := time.Now()
	schedule, err := buildStrongestBrainScheduleSnapshot(now)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to build schedule snapshot: %v", err), http.StatusInternalServerError)
		return
	}
	execution, logs, err := buildStrongestBrainExecutionSnapshot(now)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to build execution snapshot: %v", err), http.StatusInternalServerError)
		return
	}

	items := make([]StrongestBrainDecisionItem, 0)
	for _, item := range schedule.Items {
		if item.RiskLevel == "safe" || item.RiskLevel == "done" {
			continue
		}
		items = append(items, decisionFromScheduleItem(item))
	}
	for _, item := range execution.Items {
		if item.RiskLevel == "safe" || item.RiskLevel == "done" {
			continue
		}
		items = append(items, decisionFromExecutionItem(item))
	}

	items = append(items, contextGapDecisions(now)...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Rank != items[j].Rank {
			return items[i].Rank > items[j].Rank
		}
		return items[i].UpdatedAt > items[j].UpdatedAt
	})

	summary := StrongestBrainDecisionSummary{Total: len(items)}
	for _, item := range items {
		if item.RiskLevel == "critical" {
			summary.Critical++
		} else if item.RiskLevel == "warning" {
			summary.Warning++
		}
		if item.Status == "open" {
			summary.Open++
		}
		switch item.Source {
		case "schedule":
			summary.ScheduleRisks++
		case "execution":
			summary.EvidenceRisks++
		case "context":
			summary.ContextGaps++
		}
	}

	response := StrongestBrainDecisionQueueResponse{
		GeneratedAt: formatDateTime(now),
		Summary:     summary,
		Items:       items,
		Evidence:    buildEvidenceDigest(logs),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleGetStrongestBrainEvidenceChain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	taskID := strings.TrimSpace(r.URL.Query().Get("task_id"))
	if taskID == "" {
		http.Error(w, "Bad Request: task_id is required", http.StatusBadRequest)
		return
	}

	var root db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", taskID).First(&root).Error; err != nil {
		http.Error(w, fmt.Sprintf("Task %s not found", taskID), http.StatusNotFound)
		return
	}

	groupID := normalizedTaskGroupID(root.TaskGroupID)
	var related []db.TaskTelemetry
	if groupID != "" {
		if err := db.DB.Where("task_group_id = ? AND status != ?", groupID, "archived").Find(&related).Error; err != nil {
			http.Error(w, fmt.Sprintf("Failed to query related tasks: %v", err), http.StatusInternalServerError)
			return
		}
	} else {
		related = []db.TaskTelemetry{root}
	}

	taskIDs := make([]string, 0, len(related))
	for _, task := range related {
		if strings.TrimSpace(task.TaskID) != "" {
			taskIDs = append(taskIDs, strings.TrimSpace(task.TaskID))
		}
	}

	var logs []db.GitCommitLog
	if len(taskIDs) > 0 {
		if err := db.DB.Where("task_id IN ?", taskIDs).Order("created_at desc").Find(&logs).Error; err != nil {
			http.Error(w, fmt.Sprintf("Failed to query evidence chain: %v", err), http.StatusInternalServerError)
			return
		}
	}

	rootDTO := chainTaskDTO(root)
	response := StrongestBrainEvidenceChainResponse{
		GeneratedAt: formatDateTime(time.Now()),
		TaskID:      taskID,
		TaskGroupID: groupID,
		Root:        &rootDTO,
		Related:     make([]StrongestBrainChainTask, 0, len(related)),
		Evidence:    make([]StrongestBrainChainLog, 0, len(logs)),
	}
	for _, task := range related {
		response.Related = append(response.Related, chainTaskDTO(task))
	}
	for _, log := range logs {
		response.Evidence = append(response.Evidence, StrongestBrainChainLog{
			ID:        log.ID,
			TaskID:    strings.TrimSpace(log.TaskID),
			Action:    strings.TrimSpace(log.Action),
			Repo:      strings.TrimSpace(log.Repo),
			Branch:    strings.TrimSpace(log.Branch),
			CommitID:  strings.TrimSpace(log.CommitID),
			MRURL:     strings.TrimSpace(log.MrURL),
			CreatedAt: formatDateTime(log.CreatedAt),
		})
		if log.Action == "git_push" {
			response.Summary.Commits++
		}
		if strings.HasPrefix(log.Action, "mr_") {
			response.Summary.MergeRequests++
		}
		if log.Action == "mr_merge" {
			response.Summary.MergedMRs++
		}
	}
	response.Summary.RelatedTasks = len(response.Related)
	response.Summary.Signals = evidenceChainSignals(response)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleAIIntentSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var req AIIntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request: invalid JSON", http.StatusBadRequest)
		return
	}
	text := intentRequestText(req)
	if strings.TrimSpace(text) == "" {
		http.Error(w, "Bad Request: text or messages are required", http.StatusBadRequest)
		return
	}

	response := analyzeIntentDeterministic(text)
	if strings.EqualFold(strings.TrimSpace(req.Mode), "summary") || strings.Contains(r.URL.Path, "summary") {
		response.Intent = "summary"
		response.IntentLabel = intentLabel("summary")
		response.SuggestedAction = suggestedActionForIntent("summary")
		response.MissingContext = missingContextForIntent("summary", text)
		response.NextQuestions = nextQuestionsForIntent("summary")
		response.Inferences = []string{intentInference("summary")}
		response.RoutedTo = routeForIntent("summary")
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func buildStrongestBrainScheduleSnapshot(now time.Time) (ScheduleResponseDTO, error) {
	var tasks []db.TaskTelemetry
	if err := db.DB.Where("status != ?", "archived").Find(&tasks).Error; err != nil {
		return ScheduleResponseDTO{}, err
	}
	var users []userdb.User
	if err := db.DB.Find(&users).Error; err != nil {
		return ScheduleResponseDTO{}, err
	}
	return buildScheduleResponse(tasks, users, now), nil
}

func buildStrongestBrainExecutionSnapshot(now time.Time) (ExecutionTasksResponseDTO, []db.GitCommitLog, error) {
	var tasks []db.TaskTelemetry
	if err := db.DB.Where("status != ?", "archived").Find(&tasks).Error; err != nil {
		return ExecutionTasksResponseDTO{}, nil, err
	}
	var taskIDs []string
	for _, task := range tasks {
		if strings.TrimSpace(task.TaskID) != "" {
			taskIDs = append(taskIDs, strings.TrimSpace(task.TaskID))
		}
	}
	var logs []db.GitCommitLog
	if len(taskIDs) > 0 {
		if err := db.DB.Where("task_id IN ?", taskIDs).Order("created_at desc").Find(&logs).Error; err != nil {
			return ExecutionTasksResponseDTO{}, nil, err
		}
	}
	var users []userdb.User
	if err := db.DB.Find(&users).Error; err != nil {
		return ExecutionTasksResponseDTO{}, nil, err
	}
	return buildExecutionTasksResponse(tasks, logs, users, now), logs, nil
}

func decisionFromScheduleItem(item ScheduleItemDTO) StrongestBrainDecisionItem {
	riskLevel := "warning"
	if item.RiskLevel == "overdue" {
		riskLevel = "critical"
	}
	riskType := item.RiskLevel
	if item.RiskLevel == "unscheduled" {
		riskType = "missing_schedule"
	}
	return StrongestBrainDecisionItem{
		ID:              fmt.Sprintf("schedule:%s:%s", item.DemandID, riskType),
		TaskID:          item.DemandID,
		Title:           item.Title,
		Problem:         item.RiskReason,
		Evidence:        scheduleEvidenceLines(item),
		SuggestedAction: scheduleSuggestedAction(item),
		ImpactScope:     scheduleImpactScope(item),
		JumpLabel:       item.DemandID,
		RiskLevel:       riskLevel,
		RiskType:        riskType,
		Status:          decisionStatusFromLogs(item.RiskReason),
		Assignee:        item.Assignee,
		Project:         firstNonEmpty(item.ProjectKey, item.Repo, "未归属"),
		IssueType:       item.IssueType,
		UpdatedAt:       item.LastUpdate,
		Source:          "schedule",
		Rank:            item.RiskRank,
	}
}

func decisionFromExecutionItem(item ExecutionTaskItemDTO) StrongestBrainDecisionItem {
	riskLevel := "warning"
	if item.RiskLevel == "high" {
		riskLevel = "critical"
	}
	return StrongestBrainDecisionItem{
		ID:              fmt.Sprintf("execution:%s:%s", item.TaskID, item.RiskLevel),
		TaskID:          item.TaskID,
		Title:           item.Title,
		Problem:         item.RiskReason,
		Evidence:        executionEvidenceLines(item),
		SuggestedAction: executionSuggestedAction(item),
		ImpactScope:     executionImpactScope(item),
		JumpLabel:       firstNonEmpty(item.ParentDemandID, item.TaskID),
		JumpURL:         item.MRURL,
		RiskLevel:       riskLevel,
		RiskType:        firstNonEmpty(firstRiskTag(item.RiskTags), item.RiskLevel),
		Status:          "open",
		Assignee:        item.Assignee,
		Project:         firstNonEmpty(item.Repo, "未归属"),
		IssueType:       item.IssueType,
		UpdatedAt:       firstNonEmpty(item.LastEvidenceAt, item.LastUpdate),
		Source:          "execution",
		Rank:            item.RiskRank,
	}
}

func contextGapDecisions(now time.Time) []StrongestBrainDecisionItem {
	var count int64
	db.DB.Model(&db.ContextFact{}).Where("status = ?", "active").Count(&count)
	if count > 0 {
		return nil
	}
	return []StrongestBrainDecisionItem{{
		ID:              "context:missing-active-facts",
		TaskID:          "AI-CONTEXT",
		Title:           "系统设计语料库缺少 active 资料",
		Problem:         "AI 需求解构缺少可审计的系统事实上下文，可能导致追问和估算漂移",
		Evidence:        []string{"context_facts active 数量为 0", "AI 将退回 legacy 配置或默认上下文"},
		SuggestedAction: "补充架构、流程、功能边界、估算口径四类最小事实卡，再预览 context pack",
		ImpactScope:     "影响新需求解构、意图澄清、估算复盘和周会摘要可信度",
		JumpLabel:       "AI Context",
		RiskLevel:       "warning",
		RiskType:        "context_missing",
		Status:          "open",
		Assignee:        "系统管理员",
		Project:         "global",
		IssueType:       "config",
		UpdatedAt:       formatDateTime(now),
		Source:          "context",
		Rank:            58,
	}}
}

func scheduleEvidenceLines(item ScheduleItemDTO) []string {
	lines := []string{
		fmt.Sprintf("状态 %s，风险 %s", item.Status, item.RiskLabel),
	}
	if item.DueDate != "" {
		lines = append(lines, "截止日 "+item.DueDate)
	}
	if item.Branch != "" {
		lines = append(lines, "分支 "+item.Branch)
	}
	if item.SubtaskTotal > 0 {
		lines = append(lines, fmt.Sprintf("影子任务 %d/%d 完成", item.SubtaskDone, item.SubtaskTotal))
	}
	return lines
}

func scheduleSuggestedAction(item ScheduleItemDTO) string {
	switch item.RiskLevel {
	case "overdue":
		return "确认是否拆分范围、转派协助或重新承诺截止日，并记录延期原因"
	case "due_soon":
		return "确认剩余工作、验收口径和合并窗口，避免临期变逾期"
	case "stale":
		return "检查阻塞事实，要求负责人补充下一次代码或任务证据"
	case "unscheduled":
		return "补齐负责人、开发分支和截止日，让需求进入可追踪排期"
	default:
		return "保持监听，只有证据缺口或风险升级时打断人工"
	}
}

func scheduleImpactScope(item ScheduleItemDTO) string {
	return fmt.Sprintf("%s / %s / %s", firstNonEmpty(item.ProjectKey, item.Repo, "未归属"), item.Assignee, firstNonEmpty(item.TaskGroupID, "无任务组"))
}

func executionEvidenceLines(item ExecutionTaskItemDTO) []string {
	lines := []string{
		fmt.Sprintf("证据分 %d，commit %d，MR %d", item.EvidenceScore, item.CommitCount, item.MRCount),
	}
	if item.ParentDemandID != "" {
		lines = append(lines, "归属需求 "+item.ParentDemandID)
	}
	if item.LastEvidenceAt != "" {
		lines = append(lines, "最后证据 "+item.LastEvidenceAt)
	}
	if item.ResultLabel != "" {
		lines = append(lines, item.ResultLabel)
	}
	return lines
}

func executionSuggestedAction(item ExecutionTaskItemDTO) string {
	switch item.RiskLabel {
	case "完成无证据":
		return "要求补齐 commit、MR 或验收证据，否则不要把完成状态写入复盘"
	case "状态不一致":
		return "触发 Jira/GitLab 状态对齐，确认 MR 合并后是否应回写完成"
	case "未启动":
		return "确认任务是否真实启动，必要时转派或退回需求拆解"
	case "推进停滞":
		return "检查阻塞原因，给出下一次提交或评审时间"
	case "未绑定需求":
		return "绑定父级需求或任务组，让执行结果能回流排期"
	default:
		return "保留证据链并继续监听状态变化"
	}
}

func executionImpactScope(item ExecutionTaskItemDTO) string {
	if item.ParentDemandID != "" {
		return fmt.Sprintf("影响父需求 %s，执行任务 %s", item.ParentDemandID, item.TaskID)
	}
	return fmt.Sprintf("影响未归属执行任务 %s，难以进入需求复盘", item.TaskID)
}

func decisionStatusFromLogs(reason string) string {
	if strings.Contains(reason, "尚未") || strings.Contains(reason, "缺少") {
		return "open"
	}
	return "open"
}

func firstRiskTag(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return strings.TrimSpace(tags[0])
}

func buildEvidenceDigest(logs []db.GitCommitLog) []StrongestBrainEvidenceDigest {
	byTask := make(map[string]*StrongestBrainEvidenceDigest)
	for _, log := range logs {
		taskID := strings.TrimSpace(log.TaskID)
		if taskID == "" {
			continue
		}
		item := byTask[taskID]
		if item == nil {
			item = &StrongestBrainEvidenceDigest{TaskID: taskID}
			byTask[taskID] = item
		}
		if log.Action == "git_push" {
			item.CommitCount++
		}
		if strings.HasPrefix(log.Action, "mr_") {
			item.MRCount++
		}
		if item.LastEvidence == "" || log.CreatedAt.Format(time.RFC3339) > item.LastEvidence {
			item.LastEvidence = formatDateTime(log.CreatedAt)
		}
		if len(item.Signals) < 3 {
			item.Signals = append(item.Signals, firstNonEmpty(log.Action, "git_event")+" / "+firstNonEmpty(log.Branch, log.Repo, "unknown"))
		}
	}
	items := make([]StrongestBrainEvidenceDigest, 0, len(byTask))
	for _, item := range byTask {
		items = append(items, *item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].LastEvidence > items[j].LastEvidence
	})
	if len(items) > 12 {
		return items[:12]
	}
	return items
}

func chainTaskDTO(task db.TaskTelemetry) StrongestBrainChainTask {
	return StrongestBrainChainTask{
		TaskID:      strings.TrimSpace(task.TaskID),
		Title:       strings.TrimSpace(task.Title),
		IssueType:   normalizeIssueType(task.IssueType),
		Status:      strings.TrimSpace(task.Status),
		Assignee:    normalizeAssignee(task.Assignee),
		Repo:        strings.TrimSpace(task.Repo),
		Branch:      strings.TrimSpace(task.Branch),
		DueDate:     formatOptionalDate(task.DueDate),
		TaskGroupID: normalizedTaskGroupID(task.TaskGroupID),
	}
}

func evidenceChainSignals(response StrongestBrainEvidenceChainResponse) []string {
	signals := []string{}
	if response.Summary.RelatedTasks == 0 {
		signals = append(signals, "未找到关联任务")
	}
	if response.Summary.Commits == 0 && response.Summary.MergeRequests == 0 {
		signals = append(signals, "缺少代码证据")
	}
	if response.Summary.MergedMRs > 0 {
		signals = append(signals, "已有 MR 合并证据")
	}
	if len(signals) == 0 {
		signals = append(signals, "证据链正常回流")
	}
	return signals
}

func intentRequestText(req AIIntentRequest) string {
	parts := []string{strings.TrimSpace(req.Text)}
	for _, msg := range req.Messages {
		content := strings.TrimSpace(msg.Content)
		if content != "" {
			role := strings.TrimSpace(msg.Role)
			if role != "" {
				parts = append(parts, role+": "+content)
			} else {
				parts = append(parts, content)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func analyzeIntentDeterministic(text string) AIIntentResponse {
	normalized := strings.ToLower(strings.TrimSpace(text))
	scores := map[string]int{
		"demand_deconstruction": 0,
		"schedule_adjustment":   0,
		"risk_query":            0,
		"override_intervention": 0,
		"summary":               0,
		"authorization":         0,
		"configuration":         0,
	}
	intentKeywords := map[string][]string{
		"demand_deconstruction": {"需求", "拆解", "解构", "影子任务", "验收", "依赖", "acceptance", "deconstruct"},
		"schedule_adjustment":   {"排期", "延期", "截止", "due", "schedule", "工时", "估算", "临期"},
		"risk_query":            {"风险", "卡点", "异常", "红区", "停滞", "逾期", "阻塞", "risk"},
		"override_intervention": {"调停", "干预", "转派", "负责人", "挂起", "override", "reassign"},
		"summary":               {"总结", "汇总", "周报", "日报", "复盘", "summary", "report"},
		"authorization":         {"权限", "访问", "拒绝", "授权", "policy", "rbac"},
		"configuration":         {"配置", "jira", "gitlab", "webhook", "api key", "token", "ai 配置"},
	}
	for intent, keywords := range intentKeywords {
		for _, keyword := range keywords {
			if strings.Contains(normalized, strings.ToLower(keyword)) {
				scores[intent]++
			}
		}
	}

	intent := "demand_deconstruction"
	maxScore := 0
	for candidate, score := range scores {
		if score > maxScore {
			intent = candidate
			maxScore = score
		}
	}
	if maxScore == 0 {
		intent = "summary"
	}

	confidence := 0.48 + float64(maxScore)*0.12
	if confidence > 0.92 {
		confidence = 0.92
	}
	response := AIIntentResponse{
		Intent:          intent,
		IntentLabel:     intentLabel(intent),
		Confidence:      roundOneDecimal(confidence*100) / 100,
		Summary:         conciseSummary(text),
		SuggestedAction: suggestedActionForIntent(intent),
		MissingContext:  missingContextForIntent(intent, text),
		NextQuestions:   nextQuestionsForIntent(intent),
		Facts:           extractFactLines(text),
		Inferences:      []string{intentInference(intent)},
		RoutedTo:        routeForIntent(intent),
		Source:          "deterministic",
	}
	return response
}

func intentLabel(intent string) string {
	switch intent {
	case "schedule_adjustment":
		return "排期与估算"
	case "risk_query":
		return "风险查询"
	case "override_intervention":
		return "人工调停"
	case "summary":
		return "总结复盘"
	case "authorization":
		return "权限诊断"
	case "configuration":
		return "系统配置"
	default:
		return "需求解构"
	}
}

func suggestedActionForIntent(intent string) string {
	switch intent {
	case "schedule_adjustment":
		return "补齐负责人、截止日、分支和估算口径后进入排期治理"
	case "risk_query":
		return "拉取决策队列和证据链，先确认事实再决定是否干预"
	case "override_intervention":
		return "进入调停预检，确认字段变化、通知对象、同步结果和回滚条件"
	case "summary":
		return "生成日内或周会摘要，并明确区分事实、推断和建议"
	case "authorization":
		return "调用权限解释，定位命中策略、缺失权限和资源范围"
	case "configuration":
		return "进入系统配置检查连接、Webhook、AI 引擎和语料状态"
	default:
		return "先做需求澄清和 AI 解构，输出缺失信息、验收标准、风险和影子任务"
	}
}

func missingContextForIntent(intent string, text string) []string {
	missing := []string{}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "-") && !strings.Contains(lower, "jira") && !strings.Contains(lower, "需求") {
		missing = append(missing, "关联需求或 Jira 编号")
	}
	switch intent {
	case "schedule_adjustment":
		missing = append(missing, "目标截止日", "负责人或协作人", "估算工时")
	case "override_intervention":
		missing = append(missing, "干预原因", "预期影响范围", "回滚条件")
	case "risk_query":
		missing = append(missing, "项目范围", "风险时间窗口")
	case "summary":
		missing = append(missing, "总结周期", "受众角色")
	case "authorization":
		missing = append(missing, "操作动作", "资源范围")
	case "configuration":
		missing = append(missing, "配置模块", "错误现象或测试结果")
	default:
		missing = append(missing, "业务目标", "验收标准", "依赖系统")
	}
	return compactStrings(missing, 4)
}

func nextQuestionsForIntent(intent string) []string {
	switch intent {
	case "schedule_adjustment":
		return []string{"这次调整是延期、提前、还是重新估算？", "是否已有开发分支和目标负责人？"}
	case "risk_query":
		return []string{"要看日内异常、周会议题，还是某个项目的风险？", "是否只看红区和逾期项？"}
	case "override_intervention":
		return []string{"本次干预要改变负责人、截止日、状态，还是升级会议？", "是否需要同步 Jira 并通知相关人？"}
	case "summary":
		return []string{"总结对象是个人、部门、项目，还是全局？", "输出用于日报、周会，还是复盘？"}
	case "authorization":
		return []string{"哪个用户在执行哪个动作时被拒绝？", "资源范围是全局、项目、部门，还是个人？"}
	case "configuration":
		return []string{"要检查 Jira、GitLab、飞书，还是 AI 引擎？", "当前失败信息或返回状态是什么？"}
	default:
		return []string{"这个需求的业务目标和验收标准是什么？", "是否已有目标项目、负责人或截止日？"}
	}
}

func intentInference(intent string) string {
	return fmt.Sprintf("系统将该输入路由为%s，原因是文本命中了相关关键词和协同上下文。", intentLabel(intent))
}

func routeForIntent(intent string) string {
	switch intent {
	case "schedule_adjustment":
		return "schedule"
	case "risk_query":
		return "decision_queue"
	case "override_intervention":
		return "override"
	case "summary":
		return "kpi_report"
	case "authorization":
		return "authz_explain"
	case "configuration":
		return "settings"
	default:
		return "deconstructor"
	}
}

func conciseSummary(text string) string {
	cleaned := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if cleaned == "" {
		return ""
	}
	runes := []rune(cleaned)
	if len(runes) > 96 {
		return string(runes[:96]) + "..."
	}
	return cleaned
}

func extractFactLines(text string) []string {
	lines := strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == '。' || r == ';' || r == '；'
	})
	facts := make([]string, 0, 3)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if utf8.RuneCountInString(line) > 80 {
			runes := []rune(line)
			line = string(runes[:80]) + "..."
		}
		facts = append(facts, line)
		if len(facts) >= 3 {
			break
		}
	}
	if len(facts) == 0 {
		facts = append(facts, "用户提供了一段待识别文本")
	}
	return facts
}

func compactStrings(values []string, limit int) []string {
	seen := map[string]bool{}
	result := make([]string, 0, limit)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func (s *Server) handleStrongestBrainIntervention(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TaskID string `json:"task_id"`
		Action string `json:"action"` // "reassign", "reschedule", "link_repo"
		Value  string `json:"value"`  // 新指派人, 新截止日期, 新仓库名等
		Reason string `json:"reason"` // 理由
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request: invalid JSON", http.StatusBadRequest)
		return
	}

	if req.TaskID == "" || req.Action == "" || req.Value == "" {
		http.Error(w, "Bad Request: task_id, action, and value are required", http.StatusBadRequest)
		return
	}

	var task db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", req.TaskID).First(&task).Error; err != nil {
		http.Error(w, fmt.Sprintf("Task %s not found", req.TaskID), http.StatusNotFound)
		return
	}

	actor := r.Header.Get("x-authenticated-user-name")
	if actor == "" {
		actor = r.Header.Get("x-authenticated-user-id")
	}
	if actor == "" {
		actor = "Unknown"
	}

	var oldVal string
	switch req.Action {
	case "reassign":
		oldVal = task.Assignee
		task.Assignee = req.Value
		task.LastUpdate = time.Now()
	case "reschedule":
		if task.DueDate != nil {
			oldVal = task.DueDate.Format("2006-01-02")
		} else {
			oldVal = ""
		}
		
		t, err := time.Parse("2006-01-02", req.Value)
		if err != nil {
			t2, err2 := time.Parse(time.RFC3339, req.Value)
			if err2 != nil {
				http.Error(w, "Bad Request: invalid date format (expected YYYY-MM-DD)", http.StatusBadRequest)
				return
			}
			t = t2
		}
		
		task.DueDate = &t
		task.LastUpdate = time.Now()
	case "link_repo":
		oldVal = task.Repo
		task.Repo = req.Value
		task.LastUpdate = time.Now()
	default:
		http.Error(w, "Bad Request: invalid action", http.StatusBadRequest)
		return
	}

	tx := db.DB.Begin()
	if err := tx.Save(&task).Error; err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to update task: %v", err), http.StatusInternalServerError)
		return
	}

	event := db.DecisionEvent{
		TaskID:    req.TaskID,
		Actor:     actor,
		Action:    "override_" + req.Action,
		OldValue:  oldVal,
		NewValue:  req.Value,
		Reason:    req.Reason,
		CreatedAt: time.Now(),
	}
	if err := tx.Create(&event).Error; err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to record decision event: %v", err), http.StatusInternalServerError)
		return
	}

	tx.Commit()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"event":  event,
	})
}
