package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/kanban"
	providerllm "well-ambient/internal/llm"
	"well-ambient/internal/telemetry"

	"gorm.io/gorm"
)

const defaultEstimateHoursPerDay = 8

type DeconstructRequest struct {
	Text        string `json:"text"`
	DemandID    string `json:"demand_id,omitempty"`
	TaskGroupID string `json:"task_group_id,omitempty"`
}

type TaskDetail struct {
	ID             string  `json:"id"`
	Repo           string  `json:"repo"`
	Title          string  `json:"title"`
	Assignee       string  `json:"assignee"`
	Priority       string  `json:"priority"`
	Complexity     string  `json:"complexity"`
	Difficulty     string  `json:"difficulty"`
	EstimatedDays  float64 `json:"estimated_days"`
	EstimatedHours float64 `json:"estimated_hours"`
	EstimateBasis  string  `json:"estimate_basis"`
	PeriodDays     float64 `json:"period_days,omitempty"`
}

type DeconstructAnalysis struct {
	CompletenessScore     int      `json:"completeness_score"`
	MissingInfo           []string `json:"missing_info"`
	Risks                 []string `json:"risks"`
	Dependencies          []string `json:"dependencies"`
	BusinessRules         []string `json:"business_rules"`
	MainFlows             []string `json:"main_flows"`
	ExceptionFlows        []string `json:"exception_flows"`
	PermissionRules       []string `json:"permission_rules"`
	DataImpact            []string `json:"data_impact"`
	APIImpact             []string `json:"api_impact"`
	UIImpact              []string `json:"ui_impact"`
	AcceptanceCriteria    []string `json:"acceptance_criteria"`
	ScheduleNotes         []string `json:"schedule_notes"`
	MeetingQuestions      []string `json:"meeting_questions"`
	Confidence            float64  `json:"confidence"`
	OverallEstimatedDays  float64  `json:"overall_estimated_days"`
	OverallEstimatedHours float64  `json:"overall_estimated_hours"`
	OverallDifficulty     string   `json:"overall_difficulty"`
	EstimateBasis         string   `json:"estimate_basis"`
}

type DeconstructResponse struct {
	MappedRepos    []string                `json:"mappedRepos"`
	Tasks          []TaskDetail            `json:"tasks"`
	Analysis       DeconstructAnalysis     `json:"analysis"`
	ContextPackID  uint                    `json:"context_pack_id,omitempty"`
	ContextPackKey string                  `json:"context_pack_key,omitempty"`
	AttachmentIDs  []uint                  `json:"attachment_ids,omitempty"`
	Trace          *AIOutputTraceReadModel `json:"trace,omitempty"`
	IsMock         bool                    `json:"is_mock,omitempty"`
}

type deconstructStreamEvent struct {
	Type    string               `json:"type"`
	Phase   string               `json:"phase,omitempty"`
	Message string               `json:"message,omitempty"`
	Delta   string               `json:"delta,omitempty"`
	Result  *DeconstructResponse `json:"result,omitempty"`
}

// handleDeconstruct processes deconstruction requests
func (s *Server) handleDeconstruct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// If AI is not enabled or not configured, return an error and block request
	if !s.config.AI.Enabled || s.config.AI.APIToken == "" || s.config.AI.BaseURL == "" {
		http.Error(w, "AI 需求解构引擎未启用，请在集成面板中配置并开启大模型服务。", http.StatusBadRequest)
		return
	}

	req, attachments, err := s.readDeconstructRequest(w, r)
	if err != nil {
		http.Error(w, err.Error(), deconstructRequestErrorStatus(err))
		return
	}
	if strings.TrimSpace(req.Text) == "" && len(attachments) == 0 {
		http.Error(w, "Request text or attachment cannot be empty", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		req.Text = "请结合上传的需求附件完成需求解构。"
	}

	streamToBrowser := strings.Contains(strings.ToLower(r.Header.Get("Accept")), "application/x-ndjson")

	// 1. Get current maximum TaskID number in database to prevent collision
	maxIDNum := getNextTaskIDNum()

	// 2. Fetch configured GitLab repositories as candidates
	var repoNames []string
	for _, repo := range s.config.GitLab.Repos {
		if repo.Name != "" {
			repoNames = append(repoNames, repo.Name)
		}
	}
	if len(repoNames) == 0 {
		// Use default ones if none configured in gitlab
		repoNames = []string{"frontend-dashboard", "backend-core"}
	}

	// 2b. Fetch configured JIRA sync users as candidate assignees
	teamMembers := s.config.Jira.SyncUsers
	if len(teamMembers) == 0 {
		teamMembers = []string{"Eddie", "Antigravity"}
	}

	var memberQuotes []string
	for _, m := range teamMembers {
		memberQuotes = append(memberQuotes, fmt.Sprintf(`"%s"`, m))
	}
	membersStr := strings.Join(memberQuotes, "、")
	exampleAssignee := fmt.Sprintf(`"%s"`, teamMembers[0])
	workHoursPerDay := normalizeWorkHoursPerDay(s.config.AI.DefaultWorkHoursPerDay)
	projectContext := formatAIProjectContext(s.config.AI)
	contextPackID := uint(0)
	contextPackKey := ""
	if db.DB != nil {
		contextPack, err := s.buildContextPack(db.DB, req.Text, defaultContextTokenBudget, "deconstruct", true)
		if err != nil {
			log.Printf("[AI Deconstruct] Context pack assembly failed, using legacy fallback: %v", err)
		} else {
			projectContext = contextPack.Summary
			contextPackID = contextPack.ID
			contextPackKey = contextPack.CacheKey
		}
	}
	attachmentIDs := make([]uint, 0, len(attachments))
	for index := range attachments {
		attachmentIDs = append(attachmentIDs, attachments[index].ID)
		attachments[index].DemandID = strings.TrimSpace(req.DemandID)
		attachments[index].TaskGroupID = strings.TrimSpace(req.TaskGroupID)
		attachments[index].ContextPackID = contextPackID
	}
	if len(attachmentIDs) > 0 && db.DB != nil {
		if err := db.DB.Model(&db.DemandAttachment{}).
			Where("id IN ?", attachmentIDs).
			Updates(map[string]interface{}{
				"demand_id":       strings.TrimSpace(req.DemandID),
				"task_group_id":   strings.TrimSpace(req.TaskGroupID),
				"context_pack_id": contextPackID,
				"status":          "ready_for_llm",
				"updated_at":      time.Now(),
			}).Error; err != nil {
			http.Error(w, fmt.Sprintf("Failed to associate attachments: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// 3. Assemble Prompt
	systemPrompt := fmt.Sprintf(`你是一个专业的软件需求自解构引擎（Deconstructor）。你负责将用户的产品需求/开发任务（一段自然语言描述）解构成多个独立的、可执行的具体开发任务（Task），并将其映射到系统的多仓库中。

请将用户输入的开发需求解构为具体的任务列表。

【当前系统上下文】
%s

【可选仓库候选列表】
%s

【工时估算口径】
1. 以小时为主要估算单位，"estimated_hours" 与 "analysis.overall_estimated_hours" 必须优先可靠；"estimated_days" = 小时数 / %.1f，仅用于排期折算。
2. 估算必须结合当前系统上下文。若需求涉及已实现能力，请按增量改造、配置、复用、联调和验证成本估算，不要按从零开发估算。
3. 子任务工时需按真实工作构成分配，包括实现、接口/数据联调、自测、回归、配置和必要的验收支持。
4. 难度不是工时的同义词。难度需要结合技术不确定性、跨系统协作、风险暴露、验收复杂度与回滚成本。
5. 如关键信息缺失，请在 analysis.missing_info、risks、meeting_questions 中明确列出，并在 estimate_basis 说明估算置信度。

【约束条件】
1. 生成的任务中的 "repo" 字段必须匹配可选仓库候选列表中的某一个仓库名称。如果需求不属于这些仓库，请选择最接近或最合理的仓库。
2. 为生成的任务分配 ID。当前系统中已有的最大任务编号为 %d。请从 %d 开始递增生成你的任务 ID（格式如 "task-106"、"task-107" 等）。
3. 推荐合适的负责人（"assignee"）。根据任务偏向前端还是后端，从当前开发团队中做出合理推荐（当前开发团队成员包括 %s）。推荐格式为 "人员名字"（如 %s）。
4. 评估任务优先级 "priority"（可选 "High", "Medium", "Low"）、复杂度 "complexity"（可选 "High", "Medium", "Low"）与估算难度 "difficulty"（可选 "High", "Medium", "Low"）。"difficulty" 表示交付综合难度，必须结合技术不确定性、跨系统协作、验收复杂度和风险暴露，不要只复制 complexity。
5. 必须给出整体估算："analysis.overall_estimated_hours"、"analysis.overall_estimated_days" 和 "analysis.overall_difficulty"。整体估算必须能够覆盖所有子任务的关键路径，不等同于简单相加。
6. 必须为每个子任务输出 "estimated_hours"、"estimated_days"、"difficulty" 和 "estimate_basis"。子任务估算总量应与整体估算一致或可解释，允许并行任务让子任务合计大于整体关键路径。
	7. 必须额外输出 "analysis" 字段，形成可以直接指导实现的规格。除了完整性、风险、依赖、排期和验收口径，还必须明确业务规则、主流程、异常流程、权限规则、数据影响、API 影响和 UI 影响。字段缺失时用空数组或中性分数，不要省略字段。
8. 必须只返回一个紧凑的 JSON 对象，不能包含任何 Markdown 包裹标记（如三个反引号开头的 json 代码块），也不要包含 any 额外的说明文本，因为输出将直接由程序进行 JSON 反序列化。

【输出 JSON 格式要求】
{
  "mappedRepos": ["仓库名称1", "仓库名称2"],
  "tasks": [
    {
      "id": "task-xxx",
      "repo": "仓库名称",
      "title": "任务标题（清晰明确，描述此仓库需要开发的具体子功能）",
      "assignee": "推荐人名字（例如 %s）",
      "priority": "High/Medium/Low",
      "complexity": "High/Medium/Low",
      "difficulty": "High/Medium/Low",
      "estimated_days": 2.5,
      "estimated_hours": 20,
      "estimate_basis": "估算依据，例如接口数量、联调范围、测试覆盖或依赖不确定性"
    }
  ],
  "analysis": {
    "completeness_score": 82,
    "overall_estimated_days": 6,
    "overall_estimated_hours": 48,
    "overall_difficulty": "Medium",
    "estimate_basis": "整体估算依据，说明关键路径、并行空间和主要不确定性",
    "missing_info": ["仍需补充的业务背景、边界条件、数据口径或权限规则"],
	    "risks": ["可能导致返工、延期、质量问题或跨系统影响的风险"],
	    "dependencies": ["依赖的接口、数据、账号权限、上游决策或外部系统"],
	    "business_rules": ["实现必须遵守且可以被测试的业务规则"],
	    "main_flows": ["按用户动作和系统响应描述的主成功路径"],
	    "exception_flows": ["失败、回退、重试、并发冲突和边界输入的处理路径"],
	    "permission_rules": ["角色、资源范围、读写边界和高风险操作约束"],
	    "data_impact": ["新增或变更的数据实体、字段、迁移、兼容和审计要求"],
	    "api_impact": ["新增或变更的接口、请求响应、错误码、幂等和兼容要求"],
	    "ui_impact": ["涉及的页面、组件、状态、交互、响应式和可访问性要求"],
	    "acceptance_criteria": ["可验证的验收标准，必须可测试、可观察"],
    "schedule_notes": ["排期建议、并行/串行关系、关键路径或建议里程碑"],
    "meeting_questions": ["下次需求评审会议必须确认的问题"],
    "confidence": 0.78
  }
}`, projectContext, strings.Join(repoNames, ", "), workHoursPerDay, maxIDNum, maxIDNum+1, membersStr, exampleAssignee, exampleAssignee)

	// 4. Stream attachments into the provider, then use the unified provider client.
	providerFileIDs, providerFilesEndpoint, err := s.uploadAttachmentsToProvider(r.Context(), attachments)
	if err != nil {
		if len(attachmentIDs) > 0 && db.DB != nil {
			_ = db.DB.Model(&db.DemandAttachment{}).Where("id IN ?", attachmentIDs).Updates(map[string]interface{}{"status": "provider_failed", "updated_at": time.Now()}).Error
		}
		http.Error(w, fmt.Sprintf("Failed to stream attachments to LLM provider: %v", err), deconstructRequestErrorStatus(err))
		return
	}
	if len(providerFileIDs) > 0 {
		cleanupClient := &http.Client{Timeout: 30 * time.Second}
		defer s.deleteProviderFiles(context.Background(), cleanupClient, providerFilesEndpoint, providerFileIDs)
	}

	var writeStreamEvent func(deconstructStreamEvent) bool
	if streamToBrowser {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming is not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-transform")
		w.Header().Set("X-Accel-Buffering", "no")
		writeStreamEvent = func(event deconstructStreamEvent) bool {
			data, marshalErr := json.Marshal(event)
			if marshalErr != nil {
				return false
			}
			if _, writeErr := w.Write(append(data, '\n')); writeErr != nil {
				return false
			}
			flusher.Flush()
			return true
		}
		if !writeStreamEvent(deconstructStreamEvent{Type: "status", Phase: "generating", Message: "AI 已建立流式连接，正在解构需求"}) {
			return
		}
	} else {
		w.Header().Set("Content-Type", "application/json")
	}

	providerClient := providerllm.Client{Config: s.config.AI}
	providerRequest := providerllm.Request{
		SystemPrompt: systemPrompt,
		UserPrompt:   req.Text,
		FileIDs:      providerFileIDs,
	}
	var rawContent string
	if streamToBrowser {
		rawContent, err = providerClient.Stream(r.Context(), providerRequest, func(delta string) error {
			if !writeStreamEvent(deconstructStreamEvent{Type: "provider_delta", Phase: "generating", Delta: delta}) {
				return fmt.Errorf("client disconnected while receiving LLM stream")
			}
			return nil
		})
	} else {
		rawContent, err = providerClient.Generate(r.Context(), providerRequest)
	}
	if err != nil {
		log.Printf("[AI Deconstruct] Provider request failed: %v", err)
		if streamToBrowser {
			writeStreamEvent(deconstructStreamEvent{Type: "error", Phase: "generating", Message: err.Error()})
		} else {
			http.Error(w, fmt.Sprintf("Failed to connect to LLM provider: %v", err), http.StatusBadGateway)
		}
		return
	}
	if streamToBrowser && !writeStreamEvent(deconstructStreamEvent{Type: "status", Phase: "parsing", Message: "生成完成，正在校验解构结果"}) {
		return
	}

	result, cleanedContent, err := parseDeconstructResponseContentWithWorkHours(rawContent, workHoursPerDay)
	if err != nil {
		log.Printf("Raw LLM output: %s", rawContent)
		log.Printf("Cleaned LLM output: %s", cleanedContent)
		if streamToBrowser {
			writeStreamEvent(deconstructStreamEvent{Type: "error", Phase: "parsing", Message: fmt.Sprintf("Failed to parse deconstruction JSON: %v", err)})
		} else {
			http.Error(w, fmt.Sprintf("Failed to parse deconstruction JSON: %v. Raw response: %s", err, rawContent), http.StatusInternalServerError)
		}
		return
	}
	result.ContextPackID = contextPackID
	result.ContextPackKey = contextPackKey
	result.AttachmentIDs = attachmentIDs
	if len(attachmentIDs) > 0 && db.DB != nil {
		_ = db.DB.Model(&db.DemandAttachment{}).Where("id IN ?", attachmentIDs).Updates(map[string]interface{}{"status": "processed", "updated_at": time.Now()}).Error
	}
	trace, err := s.buildAIOutputTraceFromOutput(db.DB, nil, req.Text, "", "", contextPackID, aiTraceOutputFromDeconstructResponse(result), time.Now())
	if err != nil {
		log.Printf("[AI Deconstruct] Trace response assembly failed: %v", err)
	} else {
		result.Trace = &trace
	}

	if streamToBrowser {
		writeStreamEvent(deconstructStreamEvent{Type: "complete", Phase: "complete", Message: "AI 需求解构已完成", Result: &result})
		return
	}
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("Error writing deconstruction response: %v", err)
	}
}

// Helper to sanitize markdown block wraps out of the LLM JSON response
func cleanJSONContent(input string) string {
	content := strings.TrimSpace(input)
	// Remove ```json ... ``` wrapper
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
	}
	return strings.TrimSpace(content)
}

func normalizeWorkHoursPerDay(value float64) float64 {
	if value <= 0 {
		return defaultEstimateHoursPerDay
	}
	return value
}

func formatAIProjectContext(ai config.AIConfig) string {
	sections := []struct {
		label string
		value string
	}{
		{"系统架构与模块边界", ai.ProjectArchitecture},
		{"研发流程与状态流转", ai.DeliveryWorkflow},
		{"已实现能力与约束", ai.ImplementedFeatures},
		{"团队估算口径补充", ai.EstimationGuidelines},
	}

	lines := make([]string, 0, len(sections))
	for _, section := range sections {
		value := strings.TrimSpace(section.value)
		if value != "" {
			lines = append(lines, fmt.Sprintf("- %s：%s", section.label, value))
		}
	}
	if len(lines) > 0 {
		return strings.Join(lines, "\n")
	}

	return formatAIDefaultProjectContext()
}

func formatAIDefaultProjectContext() string {
	return strings.Join([]string{
		"- 系统架构与模块边界：well-ambient 是面向研发协同的内部系统，Go 后端负责配置、任务、遥测与归档 API，Svelte 前端负责需求解构、任务看板、决策面板、日报/周报预览和集成配置。",
		"- 研发流程与状态流转：需求进入系统后可由 AI 解构成影子任务，再同步到看板，并结合 GitLab、Jira、飞书和任务遥测形成事实流。",
		"- 已实现能力与约束：系统已具备 AI 需求解构、影子任务导入、任务估算归档、GitLab/Jira 配置、决策干预面板和交付状态呈现；新需求应优先按现有能力增量改造评估。",
		"- 团队估算口径补充：估算需覆盖开发、联调、自测、回归、配置和验收支持，并显式标注缺失信息、风险与置信度。",
	}, "\n")
}

func parseDeconstructResponseContent(rawContent string) (DeconstructResponse, string, error) {
	return parseDeconstructResponseContentWithWorkHours(rawContent, defaultEstimateHoursPerDay)
}

func parseDeconstructResponseContentWithWorkHours(rawContent string, workHoursPerDay float64) (DeconstructResponse, string, error) {
	cleanedContent := cleanJSONContent(rawContent)

	var parsed struct {
		MappedRepos []string             `json:"mappedRepos"`
		Tasks       []TaskDetail         `json:"tasks"`
		Analysis    *DeconstructAnalysis `json:"analysis"`
		IsMock      bool                 `json:"is_mock,omitempty"`
	}
	if err := json.Unmarshal([]byte(cleanedContent), &parsed); err != nil {
		return DeconstructResponse{}, cleanedContent, err
	}

	result := DeconstructResponse{
		MappedRepos: parsed.MappedRepos,
		Tasks:       parsed.Tasks,
		IsMock:      parsed.IsMock,
	}
	analysisMissing := parsed.Analysis == nil
	if parsed.Analysis != nil {
		result.Analysis = *parsed.Analysis
	}
	normalizeDeconstructResponseWithWorkHours(&result, analysisMissing, workHoursPerDay)
	return result, cleanedContent, nil
}

func normalizeDeconstructResponse(result *DeconstructResponse, analysisMissing bool) {
	normalizeDeconstructResponseWithWorkHours(result, analysisMissing, defaultEstimateHoursPerDay)
}

func normalizeDeconstructResponseWithWorkHours(result *DeconstructResponse, analysisMissing bool, workHoursPerDay float64) {
	workHoursPerDay = normalizeWorkHoursPerDay(workHoursPerDay)
	if result.MappedRepos == nil {
		result.MappedRepos = []string{}
	}
	if result.Tasks == nil {
		result.Tasks = []TaskDetail{}
	}

	analysis := &result.Analysis
	for i := range result.Tasks {
		normalizeTaskEstimateWithWorkHours(&result.Tasks[i], workHoursPerDay)
	}
	if analysis.CompletenessScore < 0 {
		analysis.CompletenessScore = 0
	}
	if analysis.CompletenessScore > 100 {
		analysis.CompletenessScore = 100
	}
	if analysisMissing && analysis.CompletenessScore == 0 {
		if len(result.Tasks) == 0 {
			analysis.CompletenessScore = 30
		} else if len(analysis.MissingInfo) > 0 || len(analysis.Risks) > 0 {
			analysis.CompletenessScore = 70
		} else {
			analysis.CompletenessScore = 60
		}
	}

	if analysis.Confidence > 1 && analysis.Confidence <= 100 {
		analysis.Confidence = analysis.Confidence / 100
	}
	if analysis.Confidence < 0 {
		analysis.Confidence = 0
	}
	if analysis.Confidence > 1 {
		analysis.Confidence = 1
	}
	if analysisMissing && analysis.Confidence == 0 {
		analysis.Confidence = 0.5
	}
	analysis.OverallDifficulty = normalizeDifficulty(analysis.OverallDifficulty)
	if analysis.OverallDifficulty == "" {
		analysis.OverallDifficulty = inferOverallDifficulty(result.Tasks)
	}
	if analysis.OverallDifficulty == "" {
		analysis.OverallDifficulty = "Medium"
	}
	if analysis.EstimateBasis != "" {
		analysis.EstimateBasis = strings.TrimSpace(analysis.EstimateBasis)
	}
	normalizeOverallEstimateWithWorkHours(analysis, result.Tasks, workHoursPerDay)
	distributeMissingTaskEstimatesWithWorkHours(analysis, result.Tasks, workHoursPerDay)
	for i := range result.Tasks {
		normalizeTaskEstimateWithWorkHours(&result.Tasks[i], workHoursPerDay)
	}

	analysis.MissingInfo = normalizeStringList(analysis.MissingInfo)
	analysis.Risks = normalizeStringList(analysis.Risks)
	analysis.Dependencies = normalizeStringList(analysis.Dependencies)
	analysis.AcceptanceCriteria = normalizeStringList(analysis.AcceptanceCriteria)
	analysis.ScheduleNotes = normalizeStringList(analysis.ScheduleNotes)
	analysis.MeetingQuestions = normalizeStringList(analysis.MeetingQuestions)
}

func normalizeStringList(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}

	normalized := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	if normalized == nil {
		return []string{}
	}
	return normalized
}

func normalizeTaskEstimate(task *TaskDetail) {
	normalizeTaskEstimateWithWorkHours(task, defaultEstimateHoursPerDay)
}

func normalizeTaskEstimateWithWorkHours(task *TaskDetail, workHoursPerDay float64) {
	workHoursPerDay = normalizeWorkHoursPerDay(workHoursPerDay)
	task.ID = strings.TrimSpace(task.ID)
	task.Repo = strings.TrimSpace(task.Repo)
	task.Title = strings.TrimSpace(task.Title)
	task.Assignee = strings.TrimSpace(task.Assignee)
	task.Priority = normalizeLevel(task.Priority)
	if task.Priority == "" {
		task.Priority = "Medium"
	}
	task.Complexity = normalizeLevel(task.Complexity)
	if task.Complexity == "" {
		task.Complexity = "Medium"
	}
	task.Difficulty = normalizeDifficulty(task.Difficulty)
	if task.Difficulty == "" {
		task.Difficulty = normalizeDifficulty(task.Complexity)
	}
	if task.Difficulty == "" {
		task.Difficulty = "Medium"
	}
	if task.EstimatedDays == 0 && task.PeriodDays > 0 {
		task.EstimatedDays = task.PeriodDays
	}
	if task.EstimatedDays < 0 {
		task.EstimatedDays = 0
	}
	if task.EstimatedHours < 0 {
		task.EstimatedHours = 0
	}
	if task.EstimatedDays == 0 && task.EstimatedHours > 0 {
		task.EstimatedDays = task.EstimatedHours / workHoursPerDay
	}
	if task.EstimatedHours == 0 && task.EstimatedDays > 0 {
		task.EstimatedHours = task.EstimatedDays * workHoursPerDay
	}
	task.EstimatedDays = roundEstimate(task.EstimatedDays)
	task.EstimatedHours = roundEstimate(task.EstimatedHours)
	task.PeriodDays = task.EstimatedDays
	task.EstimateBasis = strings.TrimSpace(task.EstimateBasis)
}

func normalizeLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "high", "高", "困难", "复杂":
		return "High"
	case "medium", "mid", "中", "常规":
		return "Medium"
	case "low", "低", "简单":
		return "Low"
	default:
		return ""
	}
}

func normalizeDifficulty(value string) string {
	return normalizeLevel(value)
}

func normalizeOverallEstimate(analysis *DeconstructAnalysis, tasks []TaskDetail) {
	normalizeOverallEstimateWithWorkHours(analysis, tasks, defaultEstimateHoursPerDay)
}

func normalizeOverallEstimateWithWorkHours(analysis *DeconstructAnalysis, tasks []TaskDetail, workHoursPerDay float64) {
	workHoursPerDay = normalizeWorkHoursPerDay(workHoursPerDay)
	if analysis.OverallEstimatedDays < 0 {
		analysis.OverallEstimatedDays = 0
	}
	if analysis.OverallEstimatedHours < 0 {
		analysis.OverallEstimatedHours = 0
	}
	if analysis.OverallEstimatedDays == 0 && analysis.OverallEstimatedHours > 0 {
		analysis.OverallEstimatedDays = analysis.OverallEstimatedHours / workHoursPerDay
	}
	if analysis.OverallEstimatedHours == 0 && analysis.OverallEstimatedDays > 0 {
		analysis.OverallEstimatedHours = analysis.OverallEstimatedDays * workHoursPerDay
	}
	if analysis.OverallEstimatedDays == 0 && len(tasks) > 0 {
		var sum float64
		var max float64
		for _, task := range tasks {
			days := task.EstimatedDays
			if days == 0 {
				days = defaultDaysForDifficulty(task.Difficulty)
			}
			sum += days
			if days > max {
				max = days
			}
		}
		if sum > 0 {
			parallelAllowance := sum * 0.65
			if parallelAllowance < max {
				parallelAllowance = max
			}
			analysis.OverallEstimatedDays = parallelAllowance
		}
	}
	if analysis.OverallEstimatedHours == 0 && analysis.OverallEstimatedDays > 0 {
		analysis.OverallEstimatedHours = analysis.OverallEstimatedDays * workHoursPerDay
	}
	analysis.OverallEstimatedDays = roundEstimate(analysis.OverallEstimatedDays)
	analysis.OverallEstimatedHours = roundEstimate(analysis.OverallEstimatedHours)
}

func distributeMissingTaskEstimates(analysis *DeconstructAnalysis, tasks []TaskDetail) {
	distributeMissingTaskEstimatesWithWorkHours(analysis, tasks, defaultEstimateHoursPerDay)
}

func distributeMissingTaskEstimatesWithWorkHours(analysis *DeconstructAnalysis, tasks []TaskDetail, workHoursPerDay float64) {
	workHoursPerDay = normalizeWorkHoursPerDay(workHoursPerDay)
	if len(tasks) == 0 {
		return
	}

	var knownDays float64
	var missingWeight float64
	for _, task := range tasks {
		if task.EstimatedDays > 0 {
			knownDays += task.EstimatedDays
			continue
		}
		missingWeight += difficultyWeight(task.Difficulty)
	}
	if missingWeight == 0 {
		return
	}

	availableDays := analysis.OverallEstimatedDays
	if availableDays <= 0 {
		availableDays = knownDays
	}
	if availableDays <= 0 {
		availableDays = float64(len(tasks)) * defaultDaysForDifficulty(analysis.OverallDifficulty)
	}

	remainingDays := availableDays - knownDays
	if remainingDays <= 0 {
		remainingDays = availableDays
	}
	if remainingDays <= 0 {
		remainingDays = float64(len(tasks))
	}

	for i := range tasks {
		if tasks[i].EstimatedDays > 0 {
			continue
		}
		weight := difficultyWeight(tasks[i].Difficulty)
		days := remainingDays * weight / missingWeight
		floor := defaultDaysForDifficulty(tasks[i].Difficulty) * 0.5
		if days < floor {
			days = floor
		}
		tasks[i].EstimatedDays = roundEstimate(days)
		tasks[i].EstimatedHours = roundEstimate(tasks[i].EstimatedDays * workHoursPerDay)
		if tasks[i].EstimateBasis == "" {
			tasks[i].EstimateBasis = "由整体估算、任务难度权重和并行空间自动分配"
		}
	}
}

func inferOverallDifficulty(tasks []TaskDetail) string {
	var high int
	var medium int
	for _, task := range tasks {
		switch normalizeDifficulty(task.Difficulty) {
		case "High":
			high++
		case "Medium":
			medium++
		}
	}
	if high > 0 {
		return "High"
	}
	if medium > 0 || len(tasks) > 0 {
		return "Medium"
	}
	return ""
}

func difficultyWeight(difficulty string) float64 {
	switch normalizeDifficulty(difficulty) {
	case "High":
		return 3
	case "Low":
		return 1
	default:
		return 2
	}
}

func defaultDaysForDifficulty(difficulty string) float64 {
	switch normalizeDifficulty(difficulty) {
	case "High":
		return 5
	case "Low":
		return 1.5
	default:
		return 3
	}
}

func roundEstimate(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return float64(int(value*10+0.5)) / 10
}

func getNextTaskIDNum() int {
	var maxID string
	err := db.DB.Model(&db.TaskTelemetry{}).Select("MAX(task_id)").Scan(&maxID).Error
	if err != nil || maxID == "" {
		return 104 // Default starting index helper (so next starts at 105 as in mock)
	}

	re := regexp.MustCompile(`task-(\d+)`)
	matches := re.FindStringSubmatch(maxID)
	if len(matches) > 1 {
		num, err := strconv.Atoi(matches[1])
		if err == nil {
			return num
		}
	}
	return 104
}

func getMockDeconstructResponse() DeconstructResponse {
	result := DeconstructResponse{
		IsMock:      true,
		MappedRepos: []string{"frontend-dashboard", "backend-core"},
		Tasks: []TaskDetail{
			{
				ID:             "task-105",
				Repo:           "frontend-dashboard",
				Title:          "个人中心新增“手机绑定”状态展示与更换手机交互弹窗",
				Assignee:       "Antigravity (推荐)",
				Priority:       "High",
				Complexity:     "Medium",
				Difficulty:     "Medium",
				EstimatedDays:  2,
				EstimatedHours: 16,
				EstimateBasis:  "前端状态展示和更换手机弹窗可并行开发，主要工作在交互状态与接口联调",
			},
			{
				ID:             "task-106",
				Repo:           "backend-core",
				Title:          "开发手机验证码发送 API (`POST /api/v1/auth/sms`) 与安全防刷限流",
				Assignee:       "Eddie (推荐)",
				Priority:       "High",
				Complexity:     "High",
				Difficulty:     "High",
				EstimatedDays:  4,
				EstimatedHours: 32,
				EstimateBasis:  "验证码发送、防刷、审计和异常处理涉及安全边界，需完整接口测试",
			},
			{
				ID:             "task-107",
				Repo:           "backend-core",
				Title:          "开发手机绑定与验证接口 (`POST /api/v1/user/phone/bind`)",
				Assignee:       "Eddie (推荐)",
				Priority:       "Medium",
				Complexity:     "Medium",
				Difficulty:     "Medium",
				EstimatedDays:  2.5,
				EstimatedHours: 20,
				EstimateBasis:  "绑定状态、验证码校验和用户资料写入需要串联验证",
			},
		},
		Analysis: DeconstructAnalysis{
			CompletenessScore:     72,
			OverallEstimatedDays:  5.5,
			OverallEstimatedHours: 44,
			OverallDifficulty:     "Medium",
			EstimateBasis:         "后端验证码能力是关键路径，前端弹窗和绑定接口可部分并行",
			MissingInfo: []string{
				"短信服务商、验证码有效期和发送频率限制尚未明确",
				"换绑手机号时是否需要校验旧手机号仍需确认",
			},
			Risks: []string{
				"验证码接口需要防刷和审计，否则可能产生资损或短信成本异常",
				"前后端状态口径不一致会影响用户中心展示可信度",
			},
			Dependencies: []string{
				"短信服务商配置与测试账号",
				"用户中心当前手机号字段与权限校验规则",
			},
			AcceptanceCriteria: []string{
				"用户可在个人中心看到手机绑定状态并完成手机号更换",
				"验证码发送、校验、限流和错误提示均可通过接口测试验证",
			},
			ScheduleNotes: []string{
				"后端验证码与绑定接口应先行，前端弹窗可并行开发并使用 mock 数据联调",
			},
			MeetingQuestions: []string{
				"换绑时是否必须验证旧手机号或仅验证登录态",
				"短信验证码失败、过期和频控提示是否有统一文案",
			},
			Confidence: 0.74,
		},
	}
	normalizeDeconstructResponse(&result, false)
	return result
}

func createDeconstructArchive(tx *gorm.DB, inputText string, demandID string, taskGroupID string, contextPackID uint, result DeconstructResponse, now time.Time) (uint, error) {
	if contextPackID == 0 {
		return 0, fmt.Errorf("context_pack_id is required for deconstruct archive")
	}
	mappedReposJSON, err := json.Marshal(result.MappedRepos)
	if err != nil {
		return 0, err
	}
	tasksJSON, err := json.Marshal(result.Tasks)
	if err != nil {
		return 0, err
	}
	analysisJSON, err := json.Marshal(result.Analysis)
	if err != nil {
		return 0, err
	}

	archive := db.DeconstructArchive{
		DemandID:             demandID,
		TaskGroupID:          taskGroupID,
		ContextPackID:        contextPackID,
		InputText:            inputText,
		MappedReposJSON:      string(mappedReposJSON),
		TasksJSON:            string(tasksJSON),
		AnalysisJSON:         string(analysisJSON),
		OverallEstimateDays:  result.Analysis.OverallEstimatedDays,
		OverallEstimateHours: result.Analysis.OverallEstimatedHours,
		OverallDifficulty:    result.Analysis.OverallDifficulty,
		CompletenessScore:    result.Analysis.CompletenessScore,
		Confidence:           result.Analysis.Confidence,
		IsMock:               result.IsMock,
		CreatedAt:            now,
	}
	if err := tx.Create(&archive).Error; err != nil {
		return 0, err
	}
	return archive.ID, nil
}

// testAIConnection dry-runs connection to LLM API
func testAIConnection(cfg *config.AIConfig) (bool, string, string) {
	if cfg.BaseURL == "" || cfg.APIToken == "" {
		return false, "AI API Base URL and API Token are required", ""
	}

	modelName := strings.TrimSpace(cfg.Model)
	if modelName == "" {
		modelName = "gpt-4o"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	client := providerllm.Client{
		Config:     *cfg,
		HTTPClient: &http.Client{Timeout: 8 * time.Second},
	}
	log.Printf("[AI Connection Test] Outgoing HTTP POST to: %s", cfg.GetRealAPIURL())
	log.Printf("[AI Connection Test] Target Model: %s, Protocol Mode: %s", modelName, cfg.Protocol())
	output, err := client.Generate(ctx, providerllm.Request{
		UserPrompt:      "ping",
		MaxOutputTokens: 5,
	})
	if err != nil {
		log.Printf("[AI Connection Test] Connection error: %v", err)
		return false, "Failed to contact AI API endpoint", err.Error()
	}
	if strings.TrimSpace(output) == "" {
		return false, "AI API returned an empty response", ""
	}
	return true, "AI connection successful.", fmt.Sprintf("Model '%s' is reachable through %s and responded successfully.", modelName, cfg.Protocol())
}

// handleImportTasks imports generated shadow tasks into database
func (s *Server) handleImportTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	var req struct {
		TaskGroupID   string               `json:"task_group_id"`
		DemandID      string               `json:"demand_id"`
		InputText     string               `json:"input_text"`
		MappedRepos   []string             `json:"mappedRepos"`
		Analysis      *DeconstructAnalysis `json:"analysis"`
		ContextPackID uint                 `json:"context_pack_id"`
		AttachmentIDs []uint               `json:"attachment_ids"`
		IsMock        bool                 `json:"is_mock"`
		Tasks         []TaskDetail         `json:"tasks"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		return
	}

	if len(req.Tasks) == 0 {
		http.Error(w, "Task list cannot be empty", http.StatusBadRequest)
		return
	}

	normalizedImport := DeconstructResponse{
		MappedRepos: req.MappedRepos,
		Tasks:       req.Tasks,
		IsMock:      req.IsMock,
	}
	analysisMissing := req.Analysis == nil
	if req.Analysis != nil {
		normalizedImport.Analysis = *req.Analysis
	}
	normalizeDeconstructResponse(&normalizedImport, analysisMissing)

	tx := db.DB.Begin()
	now := time.Now()
	var importedCount int

	taskGroupID := strings.TrimSpace(req.TaskGroupID)
	demandID := strings.TrimSpace(req.DemandID)
	var linkDemand *db.TaskTelemetry
	if demandID != "" {
		var demand db.TaskTelemetry
		if err := tx.Where("task_id = ? AND issue_type = 'demand'", demandID).First(&demand).Error; err != nil {
			tx.Rollback()
			http.Error(w, fmt.Sprintf("Demand %s not found", demandID), http.StatusNotFound)
			return
		}
		if taskGroupID == "" && demand.TaskGroupID != "" {
			taskGroupID = demand.TaskGroupID
		}
		linkDemand = &demand
	}
	if taskGroupID == "" && demandID != "" {
		cleanDemandID := strings.ToLower(strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
				return r
			}
			return -1
		}, demandID))
		if cleanDemandID != "" {
			taskGroupID = "brain-" + cleanDemandID
		}
	}
	if taskGroupID == "" {
		taskGroupID = fmt.Sprintf("group-%d", now.Unix())
	}

	archiveInputText := strings.TrimSpace(req.InputText)
	if archiveInputText == "" {
		archiveInputText = buildImportInputSnapshot(demandID, normalizedImport)
	}
	contextPackID := req.ContextPackID
	if contextPackID == 0 && archiveInputText != "" {
		contextPack, err := s.buildContextPack(tx, archiveInputText, defaultContextTokenBudget, "import_archive", true)
		if err != nil {
			tx.Rollback()
			http.Error(w, fmt.Sprintf("Failed to archive context pack: %v", err), http.StatusInternalServerError)
			return
		}
		contextPackID = contextPack.ID
	}

	archiveID, err := createDeconstructArchive(tx, archiveInputText, demandID, taskGroupID, contextPackID, normalizedImport, now)
	if err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to archive deconstruction estimate: %v", err), http.StatusInternalServerError)
		return
	}
	if len(req.AttachmentIDs) > 0 {
		uploadedBy := strings.TrimSpace(r.Header.Get("x-authenticated-user-id"))
		attachmentUpdate := tx.Model(&db.DemandAttachment{}).
			Where("id IN ? AND uploaded_by = ?", req.AttachmentIDs, uploadedBy).
			Updates(map[string]interface{}{
				"demand_id":              demandID,
				"task_group_id":          taskGroupID,
				"context_pack_id":        contextPackID,
				"deconstruct_archive_id": archiveID,
				"status":                 "linked",
				"updated_at":             now,
			})
		if attachmentUpdate.Error != nil {
			tx.Rollback()
			http.Error(w, fmt.Sprintf("Failed to link deconstruction attachments: %v", attachmentUpdate.Error), http.StatusInternalServerError)
			return
		}
		if attachmentUpdate.RowsAffected != int64(len(req.AttachmentIDs)) {
			tx.Rollback()
			http.Error(w, "One or more deconstruction attachments are missing or not owned by the current user", http.StatusForbidden)
			return
		}
	}
	trace, err := s.buildAIOutputTraceReadModel(tx, aiTraceQuery{ArchiveID: archiveID})
	if err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to build AI output trace: %v", err), http.StatusInternalServerError)
		return
	}

	for _, t := range normalizedImport.Tasks {
		if t.ID == "" || t.Title == "" {
			continue
		}

		var existing db.TaskTelemetry
		_ = tx.Where("task_id = ?", t.ID).First(&existing).Error

		assignee := t.Assignee
		assignee = strings.ReplaceAll(assignee, " (推荐)", "")
		assignee = strings.ReplaceAll(assignee, "（推荐）", "")

		issueType := "task"
		lowerTitle := strings.ToLower(t.Title)
		if strings.Contains(lowerTitle, "bug") || strings.Contains(lowerTitle, "故障") || strings.Contains(lowerTitle, "修复") || strings.Contains(lowerTitle, "调试") || strings.Contains(lowerTitle, "crash") || strings.Contains(lowerTitle, "异常") {
			issueType = "bug"
		}

		var dueDate *time.Time
		periodDays := int(t.EstimatedDays + 0.999)
		if periodDays > 0 {
			dt := now.AddDate(0, 0, periodDays)
			dueDate = &dt
		} else if existing.TaskID != "" {
			dueDate = existing.DueDate
		}

		telemetry := db.TaskTelemetry{
			TaskID:            t.ID,
			Title:             t.Title,
			Description:       existing.Description,
			Repo:              t.Repo,
			Assignee:          assignee,
			Creator:           existing.Creator,
			CreatorDept:       existing.CreatorDept,
			Branch:            existing.Branch,
			LastCommit:        existing.LastCommit,
			Status:            "backlog",
			IssueType:         issueType,
			TaskCreatedAt:     now,
			LastUpdate:        now,
			DueDate:           dueDate,
			DecisionLogs:      existing.DecisionLogs,
			MrIID:             existing.MrIID,
			MrURL:             existing.MrURL,
			TaskGroupID:       taskGroupID,
			EstimateDays:      t.EstimatedDays,
			EstimateHours:     t.EstimatedHours,
			Difficulty:        t.Difficulty,
			EstimateSource:    "ai_deconstruct",
			EstimateArchiveID: archiveID,
		}
		if existing.TaskID != "" {
			telemetry.TaskCreatedAt = existing.TaskCreatedAt
			if existing.Branch == "" {
				telemetry.Branch = "-"
			}
			if existing.LastCommit == "" {
				telemetry.LastCommit = "-"
			}
		}

		if err := tx.Save(&telemetry).Error; err != nil {
			tx.Rollback()
			http.Error(w, fmt.Sprintf("Failed to save task %s: %v", t.ID, err), http.StatusInternalServerError)
			return
		}
		importedCount++
	}

	if linkDemand != nil {
		linkDemand.TaskGroupID = taskGroupID
		linkDemand.LastUpdate = now
		if err := tx.Save(linkDemand).Error; err != nil {
			tx.Rollback()
			http.Error(w, fmt.Sprintf("Failed to link demand %s: %v", demandID, err), http.StatusInternalServerError)
			return
		}
	}

	// Insert a notification about this AI import
	notificationMsg := fmt.Sprintf(
		"AI 需求解构引擎已成功同步 %d 个影子任务至看板。整体预估 %.1f 天，难度 %s。",
		importedCount,
		normalizedImport.Analysis.OverallEstimatedDays,
		normalizedImport.Analysis.OverallDifficulty,
	)
	notif := db.Notification{
		Type:      "semantic_linker",
		Title:     "🤖 AI 影子卡片导入成功",
		Message:   notificationMsg,
		CreatedAt: now,
	}
	if err := tx.Create(&notif).Error; err == nil {
		// Attempt broadcast to SSE if global hook is defined
		if telemetry.OnNotificationBroadcast != nil {
			// Trigger async broadcast after transaction commits
			defer func() {
				telemetry.OnNotificationBroadcast()
			}()
		}
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to commit transaction: %v", err), http.StatusInternalServerError)
		return
	}

	// Sync to markdown after successful commit
	if linkDemand != nil {
		if err := kanban.SyncTaskToKanban(linkDemand); err != nil {
			log.Printf("Failed to sync linked demand to kanban file: %v", err)
		}
	}
	for _, t := range normalizedImport.Tasks {
		var importedTask db.TaskTelemetry
		if err := db.DB.Where("task_id = ?", t.ID).First(&importedTask).Error; err != nil {
			continue
		}
		if err := kanban.SyncTaskToKanban(&importedTask); err != nil {
			log.Printf("Failed to sync imported task %s to kanban file: %v", importedTask.TaskID, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"message":         fmt.Sprintf("Successfully imported %d tasks", importedCount),
		"imported_count":  importedCount,
		"archive_id":      archiveID,
		"context_pack_id": contextPackID,
		"trace":           trace,
	})
}
