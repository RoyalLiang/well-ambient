package telemetry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"well-ambient/internal/codereview"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/delivery"
	"well-ambient/internal/kanban"
	providerllm "well-ambient/internal/llm"

	"gorm.io/gorm"
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
		ID        string   `json:"id"`
		Message   string   `json:"message"`
		Timestamp string   `json:"timestamp"`
		Added     []string `json:"added"`
		Modified  []string `json:"modified"`
		Removed   []string `json:"removed"`
		Author    struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"commits"`
}

type MergeRequestHookPayload struct {
	ObjectKind string     `json:"object_kind"`
	User       GitLabUser `json:"user"`
	Project    struct {
		ID     int    `json:"id"`
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
	if cfg.GitLab.Secret == "" {
		http.Error(w, "GitLab webhook secret is not configured", http.StatusServiceUnavailable)
		return
	}
	secret := r.Header.Get("X-Gitlab-Token")
	if secret != cfg.GitLab.Secret {
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
		if cfg.GitLab.Enabled && cfg.AI.Enabled {
			if db.DB == nil {
				http.Error(w, "Code review queue unavailable", http.StatusServiceUnavailable)
				return
			}
			reviewer := codereview.Service{DB: db.DB, Config: func() *config.Config { return cfg }}
			if err := reviewer.Hook(r.Context(), event, body); err != nil {
				log.Printf("Code review enqueue failed: %v", err)
				http.Error(w, "Code review enqueue failed", http.StatusInternalServerError)
				return
			}
		}
		if err := ProcessWebhookEvent(cfg, event, body); err != nil {
			log.Printf("Error processing event %s: %v", event, err)
		}
	} else {
		log.Printf("Ignored event type: %s", event)
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(fmt.Sprintf("Event %s received", event)))
}

// ProcessWebhookEvent parses GitLab push/merge request hooks and saves telemetry.
// Code-review enqueue is owned by HandleWebhook so Kanban failures cannot drop it.
func ProcessWebhookEvent(cfg *config.Config, event string, body []byte) (resultErr error) {
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
		if taskID == "" {
			for _, commit := range payload.Commits {
				if taskID = ExtractTaskID(commit.Message); taskID != "" {
					break
				}
			}
		}
		taskID = resolveCanonicalTaskID(taskID)

		// AI Semantic Linker fallback if taskID is not specified
		if taskID == "" && cfg.AI.Enabled && db.DB != nil {
			taskID = trySemanticLink(cfg, branchName, lastCommit, repoName, assigneeName)
			if taskID != "" {
				notif := db.Notification{
					Type:      SemanticLinkerType,
					TaskID:    taskID,
					Title:     "🤖 AI 语义无感关联",
					Message:   fmt.Sprintf("AI 自动将推送分支/Commit关联到未完成任务: %s（已通过证据一致性校验）", taskID),
					Assignee:  assigneeName,
					Link:      "",
					CreatedAt: time.Now(),
				}
				db.DB.Create(&notif)
			}
		}

		// A commit is source evidence even when no task can be associated with it.
		// Task-specific telemetry and notifications remain gated below.
		if db.DB != nil {
			for _, c := range payload.Commits {
				cMsg := strings.TrimSpace(c.Message)
				cTaskID := ExtractTaskID(cMsg)
				if cTaskID == "" {
					cTaskID = taskID
				} else {
					cTaskID = resolveCanonicalTaskID(cTaskID)
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
				paths := append(append(append([]string(nil), c.Added...), c.Modified...), c.Removed...)
				commitLog.DedupeKey = gitCommitDedupeKey(repoName, c.ID, commitLog.Action)
				if *cfg.PerformanceBrain.Normalized().GitDedupeEnabled {
					commitLog.ContentFingerprint = gitCommitContentFingerprint(repoName, cMsg, paths)
				}
				if commitLog.ContentFingerprint == "" {
					commitLog.TelemetryQuality = "insufficient_paths"
				} else {
					commitLog.TelemetryQuality = "path_message_v1"
					var duplicate db.GitCommitLog
					if err := db.DB.Where("repo = ? AND author = ? AND action = ? AND content_fingerprint = ? AND commit_id <> ?", repoName, commitLog.Author, "git_push", commitLog.ContentFingerprint, c.ID).
						Order("created_at ASC, id ASC").First(&duplicate).Error; err == nil {
						commitLog.DuplicateOfCommit = duplicate.CommitID
					}
				}
				if _, err := persistGitPushCommit(db.DB, commitLog); err != nil {
					log.Printf("Failed to save GitCommitLog: %v", err)
				}
			}
		}
		if taskID != "" && db.DB != nil {
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
				commitLink = fmt.Sprintf("%s/commit/%s", strings.TrimSuffix(payload.Project.WebURL, "/"), lastCommitID)
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
			if OnTelemetryBroadcast != nil {
				OnTelemetryBroadcast(taskID)
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
		taskID = resolveCanonicalTaskID(taskID)
		closed, policyErr := enforceMRCommitPolicy(cfg, payload)

		// AI Semantic Linker fallback if taskID is not specified
		if taskID == "" && cfg.AI.Enabled && db.DB != nil && !closed && policyErr == nil {
			taskID = trySemanticLink(cfg, branchName, mrTitle, repoName, assigneeName)
			if taskID != "" {
				notif := db.Notification{
					Type:      SemanticLinkerType,
					TaskID:    taskID,
					Title:     "🤖 AI 语义无感关联",
					Message:   fmt.Sprintf("AI 自动将 MR (!%d) 关联到任务: %s（已通过证据一致性校验）", mrIID, taskID),
					Assignee:  assigneeName,
					Link:      mrURL,
					CreatedAt: time.Now(),
				}
				db.DB.Create(&notif)
			}
		}

		if db.DB != nil {
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

		}
		if policyErr != nil {
			return fmt.Errorf("MR commit policy: %w", policyErr)
		}
		if closed {
			return nil
		}
		if taskID != "" && db.DB != nil {
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
			if OnTelemetryBroadcast != nil {
				OnTelemetryBroadcast(taskID)
			}

			// Deep code review is queued independently of Jira identity after processing.
		}
		if db.DB != nil {
			reconcileAutonomousExecutionMR(branchName, mrURL, action, state)
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
	var estimateDays float64
	var estimateHours float64
	var difficulty string
	var estimateSource string
	var estimateArchiveID uint

	if existing.TaskID != "" {
		taskCreatedAt = existing.TaskCreatedAt
		issueType = existing.IssueType
		creator = existing.Creator
		creatorDept = existing.CreatorDept
		dueDate = existing.DueDate
		decisionLogs = existing.DecisionLogs
		taskGroupID = existing.TaskGroupID
		estimateDays = existing.EstimateDays
		estimateHours = existing.EstimateHours
		difficulty = existing.Difficulty
		estimateSource = existing.EstimateSource
		estimateArchiveID = existing.EstimateArchiveID
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
	source := strings.TrimSpace(existing.Source)
	if source == "" && existing.TaskID == "" {
		source = "git"
	}
	assignee := assigneeName
	if existing.TaskID != "" && strings.EqualFold(source, "jira") && strings.TrimSpace(existing.Assignee) != "" {
		// Git authors are evidence actors, not the owner of the canonical Jira work item.
		// Keep Jira responsibility stable so core-member visibility and owner filters do
		// not hide an otherwise correctly linked commit.
		assignee = existing.Assignee
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
		TaskID:            taskID,
		ProjectKey:        existing.ProjectKey,
		Source:            source,
		ExternalKey:       existing.ExternalKey,
		ParentWorkItemID:  existing.ParentWorkItemID,
		Revision:          existing.Revision,
		PlanningState:     existing.PlanningState,
		Title:             taskTitle,
		Description:       existing.Description,
		Repo:              repoName,
		Assignee:          assignee,
		Creator:           creator,
		CreatorDept:       creatorDept,
		Branch:            branchName,
		LastCommit:        lastCommit,
		Status:            status,
		IssueType:         issueType,
		TaskCreatedAt:     taskCreatedAt,
		LastUpdate:        time.Now(),
		CompletedAt:       completedAt,
		DueDate:           dueDate,
		DecisionLogs:      decisionLogs,
		MrIID:             mrIID,
		MrURL:             mrURL,
		TaskGroupID:       taskGroupID,
		EstimateDays:      estimateDays,
		EstimateHours:     estimateHours,
		Difficulty:        difficulty,
		EstimateSource:    estimateSource,
		EstimateArchiveID: estimateArchiveID,
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

func gitCommitDedupeKey(repo, commitID, action string) string {
	return strings.ToLower(strings.TrimSpace(repo)) + ":" + strings.ToLower(strings.TrimSpace(action)) + ":" + strings.ToLower(strings.TrimSpace(commitID))
}

func gitCommitContentFingerprint(repo, message string, paths []string) string {
	normalizedPaths := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		path = strings.ToLower(strings.TrimSpace(path))
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		normalizedPaths = append(normalizedPaths, path)
	}
	if len(normalizedPaths) == 0 {
		return ""
	}
	sort.Strings(normalizedPaths)
	normalizedMessage := strings.ToLower(strings.Join(strings.Fields(message), " "))
	digest := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(repo)) + "\n" + normalizedMessage + "\n" + strings.Join(normalizedPaths, "\n")))
	return hex.EncodeToString(digest[:])
}

func persistGitPushCommit(conn *gorm.DB, commit db.GitCommitLog) (bool, error) {
	if conn == nil {
		return false, errors.New("database is not initialized")
	}
	if strings.TrimSpace(commit.CommitID) == "" {
		return true, conn.Create(&commit).Error
	}
	var existing db.GitCommitLog
	err := conn.Where("repo = ? AND commit_id = ? AND action = ?", commit.Repo, commit.CommitID, commit.Action).First(&existing).Error
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	if err := conn.Create(&commit).Error; err != nil {
		// The partial unique index is the final idempotency boundary when two
		// webhook deliveries race after the initial read.
		lookupErr := conn.Where("repo = ? AND commit_id = ? AND action = ?", commit.Repo, commit.CommitID, commit.Action).First(&existing).Error
		if lookupErr == nil {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func resolveCanonicalTaskID(taskID string) string {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || db.DB == nil {
		return taskID
	}

	variants := []string{taskID}
	for _, variant := range []string{strings.ToUpper(taskID), strings.ToLower(taskID)} {
		alreadyIncluded := false
		for _, existing := range variants {
			if variant == existing {
				alreadyIncluded = true
				break
			}
		}
		if !alreadyIncluded {
			variants = append(variants, variant)
		}
	}

	var candidates []db.TaskTelemetry
	if err := db.DB.Where("task_id IN ?", variants).Find(&candidates).Error; err != nil {
		return taskID
	}
	for _, candidate := range candidates {
		if strings.EqualFold(candidate.TaskID, taskID) && (candidate.Source == "jira" || candidate.ExternalKey != "") {
			return candidate.TaskID
		}
	}
	for _, candidate := range candidates {
		if strings.EqualFold(candidate.TaskID, taskID) {
			return candidate.TaskID
		}
	}
	return taskID
}

func reconcileAutonomousExecutionMR(branchName, mrURL, action, state string) {
	var run db.ExecutionRun
	query := db.DB.Where("topic_branch = ?", strings.TrimSpace(branchName))
	if strings.TrimSpace(mrURL) != "" {
		query = db.DB.Where("topic_branch = ? OR mr_url = ?", strings.TrimSpace(branchName), strings.TrimSpace(mrURL))
	}
	if err := query.First(&run).Error; err != nil {
		return
	}
	now := time.Now()
	if action == "merge" || state == "merged" {
		run.MRState = "merged"
		if run.AcceptanceState == "accepted" && run.PipelineStatus == "success" {
			run.Status = delivery.RunDelivered
			run.BlockReason = ""
			run.CompletedAt = &now
			_ = db.DB.Model(&db.TaskTelemetry{}).Where("task_id = ?", run.DemandID).Updates(map[string]interface{}{
				"status": "done", "completed_at": &now, "last_update": now,
			}).Error
			_, _ = delivery.EnsureCorpusCandidates(db.DB, run)
		} else {
			run.Status = delivery.RunAcceptancePending
			run.BlockReason = "MR merged; waiting for successful pipeline and human acceptance"
		}
	} else if action == "close" || state == "closed" {
		run.MRState = "closed"
		if !delivery.IsTerminalRun(run.Status) {
			run.Status = delivery.RunRejected
			run.BlockReason = "Draft MR closed before delivery"
			run.CompletedAt = &now
		}
	} else if action == "open" || action == "reopen" || state == "opened" {
		run.MRState = "opened"
	}
	run.UpdatedAt = now
	_ = db.DB.Save(&run).Error
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

// OnTelemetryBroadcast notifies open task evidence views after Git evidence is persisted.
var OnTelemetryBroadcast func(taskID string)

// trySemanticLink matches an untracked commit/branch to an active Jira task via LLM semantic analysis
func trySemanticLink(cfg *config.Config, branchName, lastCommit, repoName, assigneeName string) string {
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

	if ok, reason := semanticLinkTrustReason(matched, branchName, repoName, assigneeName); !ok {
		log.Printf("Semantic Linker: AI matched %s but evidence guard rejected it: %s", matched, reason)
		createSemanticReviewNotification(matched, branchName, lastCommit, repoName, assigneeName, reason)
		return ""
	}

	log.Printf("Semantic Linker: AI successfully linked branch/commit to TaskID: %s", matched)
	return matched
}

// queryLLM contacts the configured Responses or Claude Messages endpoint.
func queryLLM(cfg *config.Config, systemPrompt, userPrompt string) (string, error) {
	if !cfg.AI.Enabled || cfg.AI.APIToken == "" || cfg.AI.BaseURL == "" {
		return "", fmt.Errorf("AI configuration not enabled or missing credentials")
	}

	client := providerllm.Client{Config: cfg.AI}
	return client.Generate(context.Background(), providerllm.Request{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	})
}

// compressContext shortens input context text to prevent Token overflow
func compressContext(text string, limit int) string {
	text = strings.TrimSpace(text)
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "... [已压缩超出长度的内容]"
}
