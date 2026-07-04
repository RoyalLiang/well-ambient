package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/kanban"
	"well-ambient/internal/telemetry"

	"gorm.io/gorm"
)

const defaultEstimateHoursPerDay = 8

type DeconstructRequest struct {
	Text string `json:"text"`
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
	Trace          *AIOutputTraceReadModel `json:"trace,omitempty"`
	IsMock         bool                    `json:"is_mock,omitempty"`
}

// handleDeconstruct processes deconstruction requests
func (s *Server) handleDeconstruct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DeconstructRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Text) == "" {
		http.Error(w, "Request text cannot be empty", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// If AI is not enabled or not configured, return an error and block request
	if !s.config.AI.Enabled || s.config.AI.APIToken == "" || s.config.AI.BaseURL == "" {
		http.Error(w, "AI 需求解构引擎未启用，请在集成面板中配置并开启大模型服务。", http.StatusBadRequest)
		return
	}

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
7. 必须额外输出 "analysis" 字段，帮助产品/研发评估需求完整性、风险、依赖、排期和验收口径。字段缺失时用空数组或中性分数，不要省略字段。
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
    "acceptance_criteria": ["可验证的验收标准，必须可测试、可观察"],
    "schedule_notes": ["排期建议、并行/串行关系、关键路径或建议里程碑"],
    "meeting_questions": ["下次需求评审会议必须确认的问题"],
    "confidence": 0.78
  }
}`, projectContext, strings.Join(repoNames, ", "), workHoursPerDay, maxIDNum, maxIDNum+1, membersStr, exampleAssignee, exampleAssignee)

	// 4. Construct request to AI API
	apiURL := s.config.AI.GetRealAPIURL()
	endpointType := strings.ToLower(s.config.AI.EndpointType)
	if endpointType == "" {
		endpointType = "completions"
	}

	modelName := s.config.AI.Model
	if modelName == "" {
		modelName = "gpt-4o"
	}

	requestPayload := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": req.Text},
		},
		"temperature": 0.2,
	}

	reqBytes, err := json.Marshal(requestPayload)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to marshal LLM request: %v", err), http.StatusInternalServerError)
		return
	}

	aiReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBytes))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create HTTP request to LLM: %v", err), http.StatusInternalServerError)
		return
	}
	aiReq.Header.Set("Content-Type", "application/json")
	aiReq.Header.Set("Authorization", "Bearer "+s.config.AI.APIToken)

	log.Printf("[AI Deconstruct] Outgoing HTTP POST to: %s", apiURL)
	log.Printf("[AI Deconstruct] Target Model: %s, Protocol Mode: %s", modelName, endpointType)
	log.Printf("[AI Deconstruct] Request payload size: %d bytes", len(reqBytes))

	client := http.Client{Timeout: 30 * time.Second}
	aiResp, err := client.Do(aiReq)
	if err != nil {
		log.Printf("[AI Deconstruct] Direct request failed: %v", err)
		http.Error(w, fmt.Sprintf("Failed to connect to LLM provider: %v", err), http.StatusBadGateway)
		return
	}
	defer aiResp.Body.Close()

	log.Printf("[AI Deconstruct] LLM HTTP Response received. Status: %s, Content-Type: %s", aiResp.Status, aiResp.Header.Get("Content-Type"))

	bodyBytes, err := io.ReadAll(aiResp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read LLM response body: %v", err), http.StatusInternalServerError)
		return
	}

	if aiResp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("LLM provider returned status code %d: %s", aiResp.StatusCode, string(bodyBytes)), http.StatusBadGateway)
		return
	}

	// 5. Parse LLM response
	contentType := aiResp.Header.Get("Content-Type")
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		snippetLen := 300
		if len(bodyBytes) < snippetLen {
			snippetLen = len(bodyBytes)
		}
		htmlSnippet := string(bodyBytes[:snippetLen])
		errMsg := fmt.Sprintf("大模型提供商返回了非 JSON 的 HTML 网页。这通常是因为您的 API 请求地址填写有误，或者是您的 API 密钥失效被网关拦截。网页前 %d 字符为：%s", snippetLen, htmlSnippet)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	var rawContent string
	if endpointType == "responses" {
		var responsesResp struct {
			Output []struct {
				Type    string `json:"type"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"output"`
		}
		if err := json.Unmarshal(bodyBytes, &responsesResp); err != nil {
			// Fallback to chat completion format
			var chatCompletion struct {
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}
			if err2 := json.Unmarshal(bodyBytes, &chatCompletion); err2 == nil && len(chatCompletion.Choices) > 0 {
				rawContent = chatCompletion.Choices[0].Message.Content
			} else {
				bodyStr := string(bodyBytes)
				snippetLen := 300
				if len(bodyStr) < snippetLen {
					snippetLen = len(bodyStr)
				}
				snippet := bodyStr[:snippetLen]
				diagnosticMsg := fmt.Sprintf("无法解析大模型返回的 Responses JSON。解析错误：%v。响应前 %d 字符为：%s", err, snippetLen, snippet)
				http.Error(w, diagnosticMsg, http.StatusInternalServerError)
				return
			}
		} else {
			for _, item := range responsesResp.Output {
				if item.Type == "message" {
					for _, c := range item.Content {
						if c.Type == "text" && c.Text != "" {
							rawContent += c.Text
						}
					}
				}
			}
		}
	} else {
		var chatCompletion struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(bodyBytes, &chatCompletion); err != nil {
			bodyStr := string(bodyBytes)
			snippetLen := 300
			if len(bodyStr) < snippetLen {
				snippetLen = len(bodyStr)
			}
			snippet := bodyStr[:snippetLen]

			var diagnosticMsg string
			if strings.Contains(strings.ToLower(snippet), "<html") || strings.Contains(strings.ToLower(snippet), "<!doctype") {
				diagnosticMsg = fmt.Sprintf("大模型端点返回了 HTML 页面而非 JSON 格式数据。这通常是由于 API 请求地址填写错误或 API 请求被中间网关拦截。响应前 %d 字符为：%s", snippetLen, snippet)
			} else {
				diagnosticMsg = fmt.Sprintf("无法解析大模型返回的 Completions JSON。解析错误：%v。响应前 %d 字符为：%s", err, snippetLen, snippet)
			}
			http.Error(w, diagnosticMsg, http.StatusInternalServerError)
			return
		}
		if len(chatCompletion.Choices) > 0 {
			rawContent = chatCompletion.Choices[0].Message.Content
		}
	}

	if rawContent == "" {
		http.Error(w, "LLM returned empty output", http.StatusInternalServerError)
		return
	}

	result, cleanedContent, err := parseDeconstructResponseContentWithWorkHours(rawContent, workHoursPerDay)
	if err != nil {
		log.Printf("Raw LLM output: %s", rawContent)
		log.Printf("Cleaned LLM output: %s", cleanedContent)
		http.Error(w, fmt.Sprintf("Failed to parse deconstruction JSON: %v. Raw response: %s", err, rawContent), http.StatusInternalServerError)
		return
	}
	result.ContextPackID = contextPackID
	result.ContextPackKey = contextPackKey
	trace, err := s.buildAIOutputTraceFromOutput(db.DB, nil, req.Text, "", "", contextPackID, aiTraceOutputFromDeconstructResponse(result), time.Now())
	if err != nil {
		log.Printf("[AI Deconstruct] Trace response assembly failed: %v", err)
	} else {
		result.Trace = &trace
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

	apiURL := cfg.GetRealAPIURL()
	endpointType := strings.ToLower(cfg.EndpointType)
	if endpointType == "" {
		endpointType = "completions"
	}

	modelName := cfg.Model
	if modelName == "" {
		modelName = "gpt-4o"
	}

	requestBody, err := json.Marshal(map[string]interface{}{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
		"max_tokens": 5,
	})

	if err != nil {
		return false, "Failed to serialize AI test request", err.Error()
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return false, "Failed to construct AI verification request", err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIToken)

	log.Printf("[AI Connection Test] Outgoing HTTP POST to: %s", apiURL)
	log.Printf("[AI Connection Test] Target Model: %s, Protocol Mode: %s", modelName, endpointType)

	client := http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[AI Connection Test] Connection error: %v", err)
		return false, "Failed to contact AI API endpoint", err.Error()
	}
	defer resp.Body.Close()

	log.Printf("[AI Connection Test] Response received. Status: %s, Content-Type: %s", resp.Status, resp.Header.Get("Content-Type"))

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "Failed to read AI verification response body", err.Error()
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		snippetLen := 300
		if len(bodyBytes) < snippetLen {
			snippetLen = len(bodyBytes)
		}
		return false, "AI host returned HTML response instead of JSON. Check your API URL config.", string(bodyBytes[:snippetLen])
	}

	if resp.StatusCode != http.StatusOK {
		errDetails := string(bodyBytes)
		if len(errDetails) > 300 {
			errDetails = errDetails[:300]
		}
		if errDetails == "" {
			errDetails = fmt.Sprintf("HTTP Status: %s", resp.Status)
		}
		return false, fmt.Sprintf("AI host responded with status code %d", resp.StatusCode), errDetails
	}

	if endpointType == "responses" {
		var responsesResp struct {
			Output []struct {
				Type string `json:"type"`
			} `json:"output"`
		}
		if err := json.Unmarshal(bodyBytes, &responsesResp); err != nil {
			var chatCompletion struct {
				Choices []struct {
					Index int `json:"index"`
				} `json:"choices"`
			}
			if err2 := json.Unmarshal(bodyBytes, &chatCompletion); err2 != nil {
				snippetLen := 200
				if len(bodyBytes) < snippetLen {
					snippetLen = len(bodyBytes)
				}
				return false, "AI API returned invalid JSON for Responses protocol", string(bodyBytes[:snippetLen])
			}
		}
	} else {
		var chatCompletion struct {
			Choices []struct {
				Index int `json:"index"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(bodyBytes, &chatCompletion); err != nil {
			snippetLen := 200
			if len(bodyBytes) < snippetLen {
				snippetLen = len(bodyBytes)
			}
			return false, "AI API returned invalid JSON for Completions protocol", string(bodyBytes[:snippetLen])
		}
	}

	return true, "AI connection successful.", fmt.Sprintf("Model '%s' is reachable and responded successfully.", modelName)
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
