package telemetry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/kanban"
)

type GitLabUser struct {
	Name     string `json:"name"`
	Username string `json:"username"`
}

type PushHookPayload struct {
	ObjectKind string `json:"object_kind"`
	Ref        string `json:"ref"`
	UserName   string `json:"user_name"`
	Project    struct {
		Name   string `json:"name"`
		WebURL string `json:"web_url"`
	} `json:"project"`
	Commits []struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"commits"`
}

type MergeRequestHookPayload struct {
	ObjectKind string     `json:"object_kind"`
	User       GitLabUser `json:"user"`
	Project    struct {
		Name   string `json:"name"`
		WebURL string `json:"web_url"`
	} `json:"project"`
	ObjectAttributes struct {
		Action       string `json:"action"`
		State        string `json:"state"`
		Title        string `json:"title"`
		SourceBranch string `json:"source_branch"`
		LastCommit   struct {
			Message string `json:"message"`
		} `json:"last_commit"`
		IID int    `json:"iid"`
		URL string `json:"url"`
	} `json:"object_attributes"`
	Assignees []GitLabUser `json:"assignees"`
	Assignee  *GitLabUser  `json:"assignee"`
}

// HandleWebhook processes the incoming GitLab webhook POST request
func HandleWebhook(cfg *config.Config, w http.ResponseWriter, r *http.Request) {
	// 1. Verify token
	secret := r.Header.Get("X-Gitlab-Token")
	if cfg.GitLab.Secret != "" && secret != cfg.GitLab.Secret {
		http.Error(w, "Unauthorized: invalid GitLab secret token", http.StatusUnauthorized)
		return
	}

	event := r.Header.Get("X-Gitlab-Event")
	if event == "" {
		http.Error(w, "Bad Request: missing X-Gitlab-Event header", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Internal Server Error: failed to read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	log.Printf("Received GitLab webhook event: %s (size: %d bytes)", event, len(body))

	// 2. Store payload to SQLite database db.WebhookLog
	webhookLog := db.WebhookLog{
		Event:     event,
		Payload:   string(body),
		CreatedAt: time.Now(),
	}
	if db.DB != nil {
		if err := db.DB.Create(&webhookLog).Error; err != nil {
			log.Printf("Failed to save webhook log to DB: %v", err)
		}
	} else {
		log.Printf("Warning: DB is not initialized, skipping WebhookLog save")
	}

	// 3. Process event
	if event == "Push Hook" || event == "Merge Request Hook" {
		if err := ProcessWebhookEvent(cfg, event, body); err != nil {
			log.Printf("Error processing event %s: %v", event, err)
		}
	} else {
		log.Printf("Ignored event type: %s", event)
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(fmt.Sprintf("Event %s received", event)))
}

// ProcessWebhookEvent parses GitLab push/merge request hooks and saves telemetry
func ProcessWebhookEvent(cfg *config.Config, event string, body []byte) error {
	var taskID, branchName, lastCommit, repoName, assigneeName, status, mrTitle string
	var mrIID int
	var mrURL string

	if event == "Push Hook" {
		var payload PushHookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal push hook: %w", err)
		}

		branchName = strings.TrimPrefix(payload.Ref, "refs/heads/")
		if len(payload.Commits) > 0 {
			lastCommit = strings.TrimSpace(payload.Commits[len(payload.Commits)-1].Message)
			assigneeName = payload.Commits[len(payload.Commits)-1].Author.Name
		}
		if assigneeName == "" {
			assigneeName = payload.UserName
		}
		repoName = payload.Project.Name
		status = "progress"

		taskID = ExtractTaskID(branchName)
		if taskID == "" {
			taskID = ExtractTaskID(lastCommit)
		}

		// AI Semantic Linker fallback if taskID is not specified
		if taskID == "" && cfg.AI.Enabled && db.DB != nil {
			taskID = trySemanticLink(cfg, branchName, lastCommit, repoName)
			if taskID != "" {
				notif := db.Notification{
					Type:      "semantic_linker",
					TaskID:    taskID,
					Title:     "🤖 AI 语义无感关联",
					Message:   fmt.Sprintf("AI 自动将推送分支/Commit关联到未完成任务: %s", taskID),
					Assignee:  assigneeName,
					Link:      "",
					CreatedAt: time.Now(),
				}
				db.DB.Create(&notif)
			}
		}

		if taskID != "" && db.DB != nil {
			for _, c := range payload.Commits {
				cMsg := strings.TrimSpace(c.Message)
				cTaskID := ExtractTaskID(cMsg)
				if cTaskID == "" {
					cTaskID = taskID
				}
				commitLog := db.GitCommitLog{
					TaskID:    cTaskID,
					Repo:      repoName,
					Branch:    branchName,
					CommitID:  c.ID,
					Message:   cMsg,
					Author:    c.Author.Name,
					Action:    "git_push",
					CreatedAt: time.Now(),
				}
				if err := db.DB.Create(&commitLog).Error; err != nil {
					log.Printf("Failed to save GitCommitLog: %v", err)
				}
			}
			if len(payload.Commits) == 0 {
				commitLog := db.GitCommitLog{
					TaskID:    taskID,
					Repo:      repoName,
					Branch:    branchName,
					Message:   "Branch telemetry tracked",
					Author:    assigneeName,
					Action:    "git_push",
					CreatedAt: time.Now(),
				}
				if err := db.DB.Create(&commitLog).Error; err != nil {
					log.Printf("Failed to save branch GitCommitLog: %v", err)
				}
			}

			// Save git_push notification
			lastCommitID := ""
			if len(payload.Commits) > 0 {
				lastCommitID = payload.Commits[len(payload.Commits)-1].ID
			}
			commitLink := ""
			if payload.Project.WebURL != "" && lastCommitID != "" {
				commitLink = fmt.Sprintf("%s/-/commit/%s", strings.TrimSuffix(payload.Project.WebURL, "/"), lastCommitID)
			}
			notif := db.Notification{
				Type:      "git_push",
				TaskID:    taskID,
				Title:     "代码推送",
				Message:   fmt.Sprintf("%s 推送了代码至分支 %s, 项目: %s", assigneeName, branchName, repoName),
				Assignee:  assigneeName,
				Link:      commitLink,
				CreatedAt: time.Now(),
			}
			db.DB.Create(&notif)
			if OnNotificationBroadcast != nil {
				OnNotificationBroadcast()
			}
		}
	} else if event == "Merge Request Hook" {
		var payload MergeRequestHookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal merge request hook: %w", err)
		}

		branchName = payload.ObjectAttributes.SourceBranch
		lastCommit = strings.TrimSpace(payload.ObjectAttributes.LastCommit.Message)
		mrTitle = payload.ObjectAttributes.Title
		repoName = payload.Project.Name
		mrIID = payload.ObjectAttributes.IID
		mrURL = payload.ObjectAttributes.URL

		if len(payload.Assignees) > 0 {
			assigneeName = payload.Assignees[0].Name
		} else if payload.Assignee != nil {
			assigneeName = payload.Assignee.Name
		} else {
			assigneeName = payload.User.Name
		}

		action := payload.ObjectAttributes.Action
		state := payload.ObjectAttributes.State

		if action == "merge" || state == "merged" {
			status = "done"
		} else if action == "open" || action == "reopen" || action == "update" || state == "opened" {
			status = "review"
		} else if action == "close" || state == "closed" {
			status = "progress"
		} else {
			status = "progress"
		}

		taskID = ExtractTaskID(branchName)
		if taskID == "" {
			taskID = ExtractTaskID(lastCommit)
		}

		// AI Semantic Linker fallback if taskID is not specified
		if taskID == "" && cfg.AI.Enabled && db.DB != nil {
			taskID = trySemanticLink(cfg, branchName, mrTitle, repoName)
			if taskID != "" {
				notif := db.Notification{
					Type:      "semantic_linker",
					TaskID:    taskID,
					Title:     "🤖 AI 语义无感关联",
					Message:   fmt.Sprintf("AI 自动将 MR (!%d) 关联到任务: %s", mrIID, taskID),
					Assignee:  assigneeName,
					Link:      mrURL,
					CreatedAt: time.Now(),
				}
				db.DB.Create(&notif)
			}
		}

		if taskID != "" && db.DB != nil {
			mrLog := db.GitCommitLog{
				TaskID:    taskID,
				Repo:      repoName,
				Branch:    branchName,
				MrIID:     mrIID,
				MrURL:     mrURL,
				Message:   mrTitle,
				Author:    assigneeName,
				Action:    "mr_" + action,
				CreatedAt: time.Now(),
			}
			if action == "merge" || state == "merged" {
				mrLog.Action = "mr_merge"
			} else if action == "open" || action == "reopen" || state == "opened" {
				mrLog.Action = "mr_open"
			}
			if err := db.DB.Create(&mrLog).Error; err != nil {
				log.Printf("Failed to save GitCommitLog for MR: %v", err)
			}

			// Save mr_event notification
			notif := db.Notification{
				Type:      "mr_event",
				TaskID:    taskID,
				Title:     fmt.Sprintf("MR %s", action),
				Message:   fmt.Sprintf("%s 触发了 MR !%d (%s) 事件: %s", assigneeName, mrIID, action, mrTitle),
				Assignee:  assigneeName,
				Link:      mrURL,
				CreatedAt: time.Now(),
			}
			db.DB.Create(&notif)
			if OnNotificationBroadcast != nil {
				OnNotificationBroadcast()
			}

			// Launch AI Contextual MR Reviewer in background
			if status == "review" && cfg.AI.Enabled {
				go runAIMrReview(cfg, taskID, repoName, branchName, mrTitle, mrURL, lastCommit, mrIID, assigneeName)
			}
		}
	}

	if taskID == "" {
		log.Printf("No task ID found in event payload, skipping telemetry update")
		return nil
	}

	// 4. Resolve Title
	var taskTitle string
	// Attempt to query from DB first
	var existing db.TaskTelemetry
	if db.DB != nil {
		if err := db.DB.Where("task_id = ?", taskID).First(&existing).Error; err == nil {
			taskTitle = existing.Title
		}
	}

	// If not found in DB, search in task_status.md
	if taskTitle == "" {
		if content, err := os.ReadFile(kanban.KanbanFilePath); err == nil {
			if board, err := kanban.ParseKanbanBoard(string(content)); err == nil {
				for _, sec := range board.Sections {
					for _, row := range sec.Rows {
						if strings.ToLower(row.TaskID) == strings.ToLower(taskID) {
							taskTitle = row.Title
							break
						}
					}
					if taskTitle != "" {
						break
					}
				}
			}
		}
	}

	// Fallback title resolving
	if taskTitle == "" {
		if mrTitle != "" {
			taskTitle = mrTitle
		} else if lastCommit != "" {
			// Use first line of last commit message
			lines := strings.Split(lastCommit, "\n")
			if len(lines) > 0 {
				taskTitle = strings.TrimSpace(lines[0])
			}
		} else {
			taskTitle = "Task " + taskID
		}
	}

	// 5. Construct and Save TaskTelemetry
	var taskCreatedAt time.Time
	var issueType string
	var creator string
	var creatorDept string
	var dueDate *time.Time
	var decisionLogs string
	var taskGroupID string

	if existing.TaskID != "" {
		taskCreatedAt = existing.TaskCreatedAt
		issueType = existing.IssueType
		creator = existing.Creator
		creatorDept = existing.CreatorDept
		dueDate = existing.DueDate
		decisionLogs = existing.DecisionLogs
		taskGroupID = existing.TaskGroupID
		if mrIID == 0 {
			mrIID = existing.MrIID
		}
		if mrURL == "" {
			mrURL = existing.MrURL
		}
	} else {
		taskCreatedAt = time.Now()
		issueType = "task" // default for local git telemetry tasks
	}

	var completedAt *time.Time
	if status == "done" {
		if existing.Status == "done" && existing.CompletedAt != nil {
			completedAt = existing.CompletedAt
		} else {
			now := time.Now()
			completedAt = &now
		}
	}

	telemetry := db.TaskTelemetry{
		TaskID:        taskID,
		Title:         taskTitle,
		Description:   existing.Description,
		Repo:          repoName,
		Assignee:      assigneeName,
		Creator:       creator,
		CreatorDept:   creatorDept,
		Branch:        branchName,
		LastCommit:    lastCommit,
		Status:        status,
		IssueType:     issueType,
		TaskCreatedAt: taskCreatedAt,
		LastUpdate:    time.Now(),
		CompletedAt:   completedAt,
		DueDate:       dueDate,
		DecisionLogs:  decisionLogs,
		MrIID:         mrIID,
		MrURL:         mrURL,
		TaskGroupID:   taskGroupID,
	}

	if db.DB != nil {
		if err := db.DB.Save(&telemetry).Error; err != nil {
			log.Printf("Failed to save TaskTelemetry: %v", err)
		}
	}

	// 6. Sync to Markdown Board
	if err := kanban.SyncTaskToKanban(&telemetry); err != nil {
		return fmt.Errorf("failed to sync task to kanban board: %w", err)
	}

	// 7. Write status back to Jira (Reverse Confirmation)
	SyncStatusToJira(cfg, telemetry.TaskID, telemetry.Status)

	return nil
}

// SyncStatusToJira transitions the status of a Jira issue in the background
func SyncStatusToJira(cfg *config.Config, taskID string, newStatus string) {
	if cfg == nil || !cfg.Jira.Enabled {
		return
	}

	// Run in a background goroutine to avoid blocking webhook responses
	go func() {
		log.Printf("Jira sync: Attempting to transition status of %s to %s", taskID, newStatus)
		jc := NewJiraClient(&cfg.Jira)

		// 1. Get available transitions
		transitions, err := jc.GetTransitions(taskID)
		if err != nil {
			log.Printf("Jira sync: failed to fetch transitions for %s (it might be a local task or non-existent in Jira): %v", taskID, err)
			return
		}

		// 2. Map target status to standard Jira statuses
		targetStatusName := ""
		switch newStatus {
		case "backlog":
			targetStatusName = "to do"
		case "progress":
			targetStatusName = "in progress"
		case "review":
			targetStatusName = "in review"
		case "done":
			targetStatusName = "done"
		}

		if targetStatusName == "" {
			return
		}

		// 3. Find matching transition ID
		var matchedTransitionID string
		for _, tr := range transitions {
			toName := strings.ToLower(strings.TrimSpace(tr.To.Name))
			trName := strings.ToLower(strings.TrimSpace(tr.Name))

			// Match target status name, transition name or common synonyms
			if toName == targetStatusName || trName == targetStatusName ||
				(targetStatusName == "in review" && (toName == "review" || toName == "under review" || toName == "qa" || toName == "testing")) ||
				(targetStatusName == "done" && (toName == "closed" || toName == "resolved" || toName == "completed")) ||
				(targetStatusName == "to do" && (toName == "backlog" || toName == "open" || toName == "reopened" || toName == "new")) {
				matchedTransitionID = tr.ID
				break
			}
		}

		if matchedTransitionID == "" {
			log.Printf("Jira sync: could not find matching transition on Jira for status %q on issue %s", newStatus, taskID)
			return
		}

		// 4. Perform the transition
		err = jc.TransitionIssue(taskID, matchedTransitionID)
		if err != nil {
			log.Printf("Jira sync: failed to transition issue %s: %v", taskID, err)
		} else {
			log.Printf("Jira sync: successfully transitioned issue %s on Jira to %s", taskID, targetStatusName)
		}
	}()
}

// OnNotificationBroadcast is a callback function configured by the server package to broadcast events via SSE
var OnNotificationBroadcast func()

// trySemanticLink matches an untracked commit/branch to an active Jira task via LLM semantic analysis
func trySemanticLink(cfg *config.Config, branchName, lastCommit, repoName string) string {
	var activeTasks []db.TaskTelemetry
	if err := db.DB.Where("status != 'done'").Order("last_update desc").Limit(30).Find(&activeTasks).Error; err != nil || len(activeTasks) == 0 {
		return ""
	}

	var taskListStrings []string
	for _, t := range activeTasks {
		taskListStrings = append(taskListStrings, fmt.Sprintf("- ID: %s, Title: %s, Assignee: %s", t.TaskID, t.Title, t.Assignee))
	}
	activeTasksContext := strings.Join(taskListStrings, "\n")

	systemPrompt := `你是一个软件开发任务关联引擎。你负责分析开发者提交的分支名或 Commit 消息，将其映射到一个当前未完成的任务列表中。
请只返回最匹配的任务 ID。如果没有任何任务与之相关，必须只返回 "NONE"。不要返回任何其他文字。`

	userPrompt := fmt.Sprintf(`当前开发者的行为信息如下：
- 代码仓: %s
- 分支名: %s
- 提交消息/MR标题: %s

当前未完成的任务列表如下：
%s

请分析该开发者正在进行的工作与哪个任务最吻合。请务必谨慎匹配，如果不存在明显的语义关联，请返回 "NONE"。如果匹配成功，请仅输出匹配到的任务ID（例如 PROJ-123）。`, repoName, branchName, lastCommit, activeTasksContext)

	matched, err := queryLLM(cfg, systemPrompt, userPrompt)
	if err != nil {
		log.Printf("Semantic Linker: AI query failed: %v", err)
		return ""
	}

	matched = strings.TrimSpace(matched)
	matched = strings.Trim(matched, "`\"' \n\t")
	if matched == "" || strings.ToUpper(matched) == "NONE" {
		return ""
	}

	valid := false
	for _, t := range activeTasks {
		if strings.ToLower(t.TaskID) == strings.ToLower(matched) {
			matched = t.TaskID
			valid = true
			break
		}
	}

	if !valid {
		log.Printf("Semantic Linker: AI matched a hallucinated TaskID: %s", matched)
		return ""
	}

	log.Printf("Semantic Linker: AI successfully linked branch/commit to TaskID: %s", matched)
	return matched
}

// runAIMrReview performs background code review by comparing MR diffs/commits with Jira stories
func runAIMrReview(cfg *config.Config, taskID, repoName, branchName, mrTitle, mrURL, lastCommit string, mrIID int, assigneeName string) {
	log.Printf("AI MR Review: starting reviewer daemon for task %s (MR !%d)", taskID, mrIID)

	jc := NewJiraClient(&cfg.Jira)
	issues, err := jc.SearchIssues(fmt.Sprintf("key = %s", taskID))
	jiraTitle := ""
	jiraDesc := ""
	if err == nil && len(issues) > 0 {
		jiraTitle = issues[0].Fields.Summary
		jiraDesc = issues[0].Fields.Description
	} else {
		log.Printf("AI MR Review: failed to fetch Jira details for %s: %v", taskID, err)
		jiraTitle = "Unknown Jira Title"
		jiraDesc = "No description available."
	}

	systemPrompt := `你是一个资深的软件架构师和代码评审专家。你负责评估一次代码变更（Merge Request）与对应 Jira 需求任务描述及验收标准（AC）的一致性，识别潜在的设计风险并推荐回归测试场景。`
	userPrompt := fmt.Sprintf(`[Jira 任务信息]
ID: %s
标题: %s
描述与验收标准:
%s

[当前 Merge Request 变动]
项目: %s
MR 标题: %s
最后提交消息/修改概述:
%s

请提供代码评审报告：
1. 【业务一致性审计】：本次修改是否覆盖了 Jira 的核心诉求？是否存在严重遗漏或超出需求范围的偏离？
2. 【架构与隐性风险】：对核心系统有什么潜在的副作用或安全/并发风险？
3. 【推荐回归场景】：推荐 QA 重点测试的 2-3 个业务回归场景。
请仅以清晰的 Markdown 格式输出（字数限制在 300 字以内），排版要紧凑美观。不要有废话。`, taskID, jiraTitle, compressContext(jiraDesc, 1200), repoName, mrTitle, compressContext(lastCommit, 800))

	aiReport, err := queryLLM(cfg, systemPrompt, userPrompt)
	if err != nil {
		log.Printf("AI MR Review: LLM query failed for %s: %v", taskID, err)
		return
	}

	aiReport = strings.TrimSpace(aiReport)
	if aiReport == "" {
		return
	}

	reviewLog := db.GitCommitLog{
		TaskID:    taskID,
		Repo:      repoName,
		Branch:    branchName,
		MrIID:     mrIID,
		MrURL:     mrURL,
		Message:   aiReport,
		Author:    "🤖 AI Reviewer",
		Action:    "ai_review",
		CreatedAt: time.Now(),
	}

	if db.DB != nil {
		if err := db.DB.Create(&reviewLog).Error; err != nil {
			log.Printf("AI MR Review: failed to save review log: %v", err)
		}

		notif := db.Notification{
			Type:      "ai_review",
			TaskID:    taskID,
			Title:     "🤖 AI 自动代码评审完成",
			Message:   fmt.Sprintf("已完成对 %s (MR !%d) 的业务一致性审计。报告已追加至活动日志。", taskID, mrIID),
			Assignee:  assigneeName,
			Link:      mrURL,
			CreatedAt: time.Now(),
		}
		db.DB.Create(&notif)

		if OnNotificationBroadcast != nil {
			OnNotificationBroadcast()
		}
	}
	log.Printf("AI MR Review: successfully generated and saved code review report for %s", taskID)
}

// queryLLM contacts OpenAI compatible endpoint configured in Config
func queryLLM(cfg *config.Config, systemPrompt, userPrompt string) (string, error) {
	if !cfg.AI.Enabled || cfg.AI.APIToken == "" || cfg.AI.BaseURL == "" {
		return "", fmt.Errorf("AI configuration not enabled or missing credentials")
	}

	apiURL := cfg.AI.GetRealAPIURL()
	endpointType := strings.ToLower(cfg.AI.EndpointType)
	if endpointType == "" {
		endpointType = "completions"
	}

	modelName := cfg.AI.Model
	if modelName == "" {
		modelName = "gpt-4o"
	}

	requestPayload := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.2,
	}

	reqBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.AI.APIToken)

	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM provider returned status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		snippetLen := 300
		if len(bodyBytes) < snippetLen {
			snippetLen = len(bodyBytes)
		}
		return "", fmt.Errorf("LLM provider returned HTML response instead of JSON. Check your API URL: %s", string(bodyBytes[:snippetLen]))
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
			// 尝试用 chatCompletion 格式做二次反序列化（防止网关自作聪明在协议之间做隐式映射）
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
				snippetLen := 300
				if len(bodyBytes) < snippetLen {
					snippetLen = len(bodyBytes)
				}
				snippet := string(bodyBytes[:snippetLen])
				return "", fmt.Errorf("failed to decode JSON response from Responses API: %v. Raw snippet: %s", err, snippet)
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
			snippetLen := 300
			if len(bodyBytes) < snippetLen {
				snippetLen = len(bodyBytes)
			}
			snippet := string(bodyBytes[:snippetLen])
			if strings.Contains(strings.ToLower(snippet), "<html") || strings.Contains(strings.ToLower(snippet), "<!doctype") {
				return "", fmt.Errorf("LLM provider returned HTML page. Check API URL. Content: %s", snippet)
			}
			return "", fmt.Errorf("failed to decode JSON response: %v. Raw snippet: %s", err, snippet)
		}
		if len(chatCompletion.Choices) > 0 {
			rawContent = chatCompletion.Choices[0].Message.Content
		}
	}

	if rawContent == "" {
		return "", fmt.Errorf("LLM returned empty output")
	}

	return rawContent, nil
}

// compressContext shortens input context text to prevent Token overflow
func compressContext(text string, limit int) string {
	text = strings.TrimSpace(text)
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "... [已压缩超出长度的内容]"
}
