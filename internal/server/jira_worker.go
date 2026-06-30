package server

import (
	"fmt"
	"log"
	"strings"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
	"well-ambient/internal/kanban"
	"well-ambient/internal/telemetry"
)

// startJiraSyncWorker starts a background loop to fetch tasks/bugs from Jira
func (s *Server) startJiraSyncWorker() {
	log.Println("Starting background Jira task synchronization worker...")
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds for quick local testing/responsiveness
	defer ticker.Stop()

	// Initial run
	s.syncJiraTasks()

	for range ticker.C {
		s.syncJiraTasks()
	}
}

func (s *Server) syncJiraTasks() {
	if !s.config.Jira.Enabled {
		return
	}

	jql := buildJQL(&s.config.Jira)
	if jql == "" {
		return
	}
	jc := telemetry.NewJiraClient(&s.config.Jira)

	issues, err := jc.SearchIssues(jql)
	if err != nil {
		log.Printf("Jira sync: failed to search issues: %v", err)
		return
	}

	if len(issues) == 0 {
		return
	}

	log.Printf("Jira sync: retrieved %d issues matching JQL: %s", len(issues), jql)

	for _, issue := range issues {
		// Determine assignee name
		assigneeName := "未指派"
		if issue.Fields.Assignee != nil {
			s.syncJiraUserToLocal(issue.Fields.Assignee.Name, issue.Fields.Assignee.DisplayName, issue.Fields.Assignee.EmailAddress)
			if issue.Fields.Assignee.DisplayName != "" {
				assigneeName = issue.Fields.Assignee.DisplayName
			} else if issue.Fields.Assignee.Name != "" {
				assigneeName = issue.Fields.Assignee.Name
			} else {
				assigneeName = issue.Fields.Assignee.EmailAddress
			}
		}

		jiraMappedStatus := mapJiraStatus(issue.Fields.Status.Name)
		issueType := mapJiraIssueType(issue.Fields.IssueType.Name)
		createdTime := parseJiraTime(issue.Fields.Created)
		jiraProject := formatJiraProjectLabel(issue.Fields.Project.Key, issue.Fields.Project.Name)

		// Check if task exists in SQLite DB
		var existing db.TaskTelemetry
		err := db.DB.Where("task_id = ?", issue.Key).First(&existing).Error
		if err != nil {
			// Insert new task
			newTelemetry := db.TaskTelemetry{
				TaskID:        issue.Key,
				Title:         issue.Fields.Summary,
				Repo:          firstNonBlank(jiraProject, "-"),
				Assignee:      assigneeName,
				Branch:        "-",
				LastCommit:    "-",
				Status:        jiraMappedStatus,
				IssueType:     issueType,
				TaskCreatedAt: createdTime,
				LastUpdate:    time.Now(),
			}

			if err := db.DB.Create(&newTelemetry).Error; err != nil {
				log.Printf("Jira sync: failed to save new task %s: %v", issue.Key, err)
			} else {
				log.Printf("Jira sync: created new local task %s with status %s", issue.Key, jiraMappedStatus)
				if err := kanban.SyncTaskToKanban(&newTelemetry); err != nil {
					log.Printf("Jira sync: failed to write task %s to kanban markdown: %v", issue.Key, err)
				}
			}
		} else {
			// Check if we should update existing task
			hasChanges := false

			if existing.Title != issue.Fields.Summary {
				existing.Title = issue.Fields.Summary
				hasChanges = true
			}

			if existing.Assignee != assigneeName {
				if shouldPreserveLocalAssignee(existing, assigneeName, time.Now()) {
					log.Printf("Jira sync: preserving local assignee override for %s (%s), ignoring Jira assignee %s", issue.Key, existing.Assignee, assigneeName)
				} else {
					existing.Assignee = assigneeName
					hasChanges = true
				}
			}

			if existing.IssueType != issueType {
				existing.IssueType = issueType
				hasChanges = true
			}

			if jiraProject != "" && existing.Repo != jiraProject {
				existing.Repo = jiraProject
				hasChanges = true
			}

			if existing.TaskCreatedAt.Unix() != createdTime.Unix() {
				existing.TaskCreatedAt = createdTime
				hasChanges = true
			}

			// Only pull status changes from Jira if the local status wasn't updated in the last 15 seconds
			// to prevent overwriting a status that was just moved by a local GitLab webhook.
			if existing.Status != jiraMappedStatus && time.Since(existing.LastUpdate) > 15*time.Second {
				log.Printf("Jira sync: status of %s changed on Jira from %s -> %s, updating locally", issue.Key, existing.Status, jiraMappedStatus)
				existing.Status = jiraMappedStatus
				hasChanges = true
			}

			if hasChanges {
				existing.LastUpdate = time.Now()
				if err := db.DB.Save(&existing).Error; err != nil {
					log.Printf("Jira sync: failed to update task %s: %v", issue.Key, err)
				} else {
					if err := kanban.SyncTaskToKanban(&existing); err != nil {
						log.Printf("Jira sync: failed to write updated task %s to kanban markdown: %v", issue.Key, err)
					}
				}
			}
		}
		// 每次成功拉取到 Jira 任务时同步其评论
		s.syncJiraComments(jc, issue.Key)
	}

	// =================================================================
	// Phase 2: Active Local Jira Tasks Keep-Alive & Correction Sync
	// =================================================================
	var activeLocalTasks []db.TaskTelemetry
	// Query local active Jira issues (contains '-' in TaskID and status != 'done')
	if err := db.DB.Where("status != 'done' AND task_id LIKE '%-%'").Find(&activeLocalTasks).Error; err == nil && len(activeLocalTasks) > 0 {
		var activeKeys []string
		for _, t := range activeLocalTasks {
			parts := strings.Split(t.TaskID, "-")
			if len(parts) != 2 {
				continue
			}
			projKey := parts[0]
			issueNum := parts[1]

			// 1. Validate projectKey: starts with letter, only letters/digits, length 2-10
			if len(projKey) < 2 || len(projKey) > 10 {
				continue
			}
			first := projKey[0]
			if !((first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z')) {
				continue
			}
			isValid := true
			for i := 0; i < len(projKey); i++ {
				c := projKey[i]
				if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
					isValid = false
					break
				}
			}
			if !isValid {
				continue
			}

			// 2. Validate issueNum: digits only
			if len(issueNum) == 0 {
				continue
			}
			isNum := true
			for i := 0; i < len(issueNum); i++ {
				if issueNum[i] < '0' || issueNum[i] > '9' {
					isNum = false
					break
				}
			}
			if !isNum {
				continue
			}

			// 3. Limit to configured SyncProjects
			if len(s.config.Jira.SyncProjects) > 0 {
				isSync := false
				for _, sp := range s.config.Jira.SyncProjects {
					if strings.EqualFold(strings.TrimSpace(sp), projKey) {
						isSync = true
						break
					}
				}
				if !isSync {
					continue
				}
			}

			activeKeys = append(activeKeys, fmt.Sprintf("%q", t.TaskID))
		}

		// Sync in batches of 50 to prevent JQL length overflow
		batchSize := 50
		for i := 0; i < len(activeKeys); i += batchSize {
			end := i + batchSize
			if end > len(activeKeys) {
				end = len(activeKeys)
			}
			batch := activeKeys[i:end]

			keepAliveJQL := fmt.Sprintf("key in (%s)", strings.Join(batch, ", "))
			aliveIssues, err := jc.SearchIssues(keepAliveJQL)
			if err != nil {
				log.Printf("Jira sync keep-alive: failed to search JQL %s: %v", keepAliveJQL, err)
				continue
			}

			for _, issue := range aliveIssues {
				assigneeName := "未指派"
				if issue.Fields.Assignee != nil {
					s.syncJiraUserToLocal(issue.Fields.Assignee.Name, issue.Fields.Assignee.DisplayName, issue.Fields.Assignee.EmailAddress)
					if issue.Fields.Assignee.DisplayName != "" {
						assigneeName = issue.Fields.Assignee.DisplayName
					} else if issue.Fields.Assignee.Name != "" {
						assigneeName = issue.Fields.Assignee.Name
					} else {
						assigneeName = issue.Fields.Assignee.EmailAddress
					}
				}

				jiraMappedStatus := mapJiraStatus(issue.Fields.Status.Name)
				issueType := mapJiraIssueType(issue.Fields.IssueType.Name)
				jiraProject := formatJiraProjectLabel(issue.Fields.Project.Key, issue.Fields.Project.Name)

				var existing db.TaskTelemetry
				if err := db.DB.Where("task_id = ?", issue.Key).First(&existing).Error; err == nil {
					hasChanges := false
					if existing.Assignee != assigneeName {
						if shouldPreserveLocalAssignee(existing, assigneeName, time.Now()) {
							log.Printf("Jira sync keep-alive: preserving local assignee override for %s (%s), ignoring Jira assignee %s", issue.Key, existing.Assignee, assigneeName)
						} else {
							log.Printf("Jira sync keep-alive: assignee of %s corrected from %s -> %s (even if out of configuration range)", issue.Key, existing.Assignee, assigneeName)
							existing.Assignee = assigneeName
							hasChanges = true
						}
					}
					if existing.Title != issue.Fields.Summary {
						existing.Title = issue.Fields.Summary
						hasChanges = true
					}
					if existing.IssueType != issueType {
						existing.IssueType = issueType
						hasChanges = true
					}
					if jiraProject != "" && existing.Repo != jiraProject {
						existing.Repo = jiraProject
						hasChanges = true
					}
					if existing.Status != jiraMappedStatus && time.Since(existing.LastUpdate) > 15*time.Second {
						log.Printf("Jira sync keep-alive: status of %s aligned from %s -> %s", issue.Key, existing.Status, jiraMappedStatus)
						existing.Status = jiraMappedStatus
						hasChanges = true
					}

					if hasChanges {
						existing.LastUpdate = time.Now()
						if err := db.DB.Save(&existing).Error; err != nil {
							log.Printf("Jira sync keep-alive: failed to save correction for %s: %v", issue.Key, err)
						} else {
							_ = kanban.SyncTaskToKanban(&existing)
						}
					}
					// 自动同步评论
					s.syncJiraComments(jc, issue.Key)
				}
			}
		}
	}
}

func formatJiraProjectLabel(key, name string) string {
	key = strings.TrimSpace(key)
	name = strings.TrimSpace(name)
	if key == "" {
		return name
	}
	if name == "" || strings.EqualFold(name, key) {
		return key
	}
	return fmt.Sprintf("%s (%s)", name, key)
}

// mapJiraStatus maps general Jira issue statuses to our 4 Kanban stages
func mapJiraStatus(jiraStatus string) string {
	s := strings.ToLower(strings.TrimSpace(jiraStatus))
	switch s {
	case "to do", "backlog", "open", "reopened", "new", "todo":
		return "backlog"
	case "in progress", "progress", "active", "doing":
		return "progress"
	case "in review", "review", "under review", "qa", "testing":
		return "review"
	case "done", "closed", "resolved", "completed":
		return "done"
	default:
		return "backlog"
	}
}

// mapJiraIssueType converts Jira issue types into the app's planning model.
// In this deployment Jira Task/Story-like items are real demands; AI-generated
// shadow work remains `task` when it enters through the deconstruction import.
func mapJiraIssueType(jiraIssueType string) string {
	s := strings.ToLower(strings.TrimSpace(jiraIssueType))
	switch s {
	case "bug", "缺陷", "故障", "defect":
		return "bug"
	case "task", "任务", "story", "故事", "requirement", "需求", "feature", "epic":
		return "demand"
	default:
		return "demand"
	}
}

func shouldPreserveLocalAssignee(task db.TaskTelemetry, incomingAssignee string, now time.Time) bool {
	current := strings.TrimSpace(task.Assignee)
	incoming := strings.TrimSpace(incomingAssignee)
	if current == "" || strings.EqualFold(current, incoming) {
		return false
	}
	if task.LastUpdate.IsZero() || now.Sub(task.LastUpdate) > 24*time.Hour {
		return false
	}

	logs := strings.TrimSpace(task.DecisionLogs)
	if logs == "" {
		return false
	}

	return strings.Contains(logs, "转派") ||
		strings.Contains(logs, "调整需求负责人") ||
		strings.Contains(logs, "调停干预")
}

// buildJQL constructs a JQL query string from the Jira sync configuration
func buildJQL(cfg *config.JiraConfig) string {
	if strings.TrimSpace(cfg.CustomJQL) != "" {
		return strings.TrimSpace(cfg.CustomJQL)
	}

	var parts []string

	// Filter by projects
	if len(cfg.SyncProjects) > 0 {
		var quotedProjects []string
		for _, p := range cfg.SyncProjects {
			if p = strings.TrimSpace(p); p != "" {
				quotedProjects = append(quotedProjects, fmt.Sprintf("%q", p))
			}
		}
		if len(quotedProjects) > 0 {
			parts = append(parts, fmt.Sprintf("project in (%s)", strings.Join(quotedProjects, ", ")))
		}
	}

	// Filter by assignees
	if len(cfg.SyncUsers) > 0 {
		var quotedUsers []string
		for _, u := range cfg.SyncUsers {
			if u = strings.TrimSpace(u); u != "" {
				quotedUsers = append(quotedUsers, fmt.Sprintf("%q", u))
			}
		}
		if len(quotedUsers) > 0 {
			parts = append(parts, fmt.Sprintf("assignee in (%s)", strings.Join(quotedUsers, ", ")))
		}
	}

	// Filter by status/progress
	if len(cfg.SyncStatuses) > 0 {
		var quotedStatuses []string
		for _, st := range cfg.SyncStatuses {
			if st = strings.TrimSpace(st); st != "" {
				quotedStatuses = append(quotedStatuses, fmt.Sprintf("%q", st))
			}
		}
		if len(quotedStatuses) > 0 {
			parts = append(parts, fmt.Sprintf("status in (%s)", strings.Join(quotedStatuses, ", ")))
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " AND ")
}

// parseJiraTime parses standard Jira timestamps into time.Time
func parseJiraTime(timeStr string) time.Time {
	if timeStr == "" {
		return time.Now()
	}
	// Try RFC3339 format
	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return t
	}
	// Try Jira standard format: "2006-01-02T15:04:05.000-0700"
	if t, err := time.Parse("2006-01-02T15:04:05.000-0700", timeStr); err == nil {
		return t
	}
	return time.Now()
}

// syncAssigneeToJira updates the assignee in Jira. It runs asynchronously to avoid blocking the API response.
func (s *Server) syncAssigneeToJira(issueKey string, localAssignee string) {
	if !s.config.Jira.Enabled {
		return
	}
	log.Printf("Jira sync: attempting to sync assignee override for %s -> %s", issueKey, localAssignee)
	jc := telemetry.NewJiraClient(&s.config.Jira)

	// 1. 根据中文显示名称/邮箱名反查 Jira 所需的 username
	jiraUser := localAssignee
	if localAssignee == "未指派" || localAssignee == "-" || localAssignee == "Unassigned" {
		jiraUser = "" // 取消指派
	} else {
		var user userdb.User
		err := db.DB.Where("name = ? OR username = ? OR email = ?", localAssignee, localAssignee, localAssignee).First(&user).Error
		if err == nil {
			jiraUser = user.Username
			if idx := strings.Index(jiraUser, "@"); idx > 0 {
				jiraUser = jiraUser[:idx] // 剥离邮箱前缀，有些 Jira 的登录用户名就是邮箱前缀
			}
		} else {
			// 如果没有查到，且 localAssignee 包含中文，说明是无法直接使用的中文名，为防 Jira API 报错应终止同步
			hasChinese := false
			for _, r := range localAssignee {
				if r >= 0x4e00 && r <= 0x9fa5 {
					hasChinese = true
					break
				}
			}
			if hasChinese {
				log.Printf("Jira sync WARNING: cannot map local assignee '%s' to a valid Jira username, skipping Jira sync.", localAssignee)
				return
			}
		}
	}

	// 2. 调用 JiraClient 接口
	err := jc.UpdateAssignee(issueKey, jiraUser)
	if err != nil {
		log.Printf("Jira sync: failed to sync assignee for %s to Jira: %v", issueKey, err)
	} else {
		log.Printf("Jira sync: successfully synced assignee for %s to Jira (%s)", issueKey, jiraUser)
	}
}

// syncJiraUserToLocal automatically upserts the user info retrieved from Jira into the local users table.
func (s *Server) syncJiraUserToLocal(username, displayName, email string) {
	if username == "" && email == "" {
		return
	}

	var user userdb.User
	// 尝试通过 Username 或 Email 查询
	err := db.DB.Where("username = ? OR email = ?", username, email).First(&user).Error
	if err != nil {
		// 未找到，新建用户映射
		dbEmail := email
		if dbEmail == "" {
			dbEmail = username + "@westwell-lab.com"
		}
		dbUsername := username
		if dbUsername == "" {
			dbUsername = email
		}
		dbName := displayName
		if dbName == "" {
			dbName = username
		}

		newUser := userdb.User{
			Username:   dbUsername,
			Email:      dbEmail,
			Name:       dbName,
			Department: "未分配",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		if err := db.DB.Create(&newUser).Error; err != nil {
			log.Printf("Jira sync user: failed to create user mapping for %s: %v", username, err)
		} else {
			log.Printf("Jira sync user: automatically created local user mapping: %s (%s)", dbName, dbUsername)
		}
	} else {
		// 已存在，如果显示名有变化则同步更新
		if displayName != "" && user.Name != displayName {
			user.Name = displayName
			user.UpdatedAt = time.Now()
			if err := db.DB.Save(&user).Error; err == nil {
				log.Printf("Jira sync user: updated local user name mapping for %s: %s -> %s", username, user.Name, displayName)
			}
		}
	}
}

// syncJiraComments pulls comments for a Jira issue and saves them locally.
func (s *Server) syncJiraComments(jc *telemetry.JiraClient, issueKey string) {
	if jc == nil || issueKey == "" {
		return
	}

	comments, err := jc.GetComments(issueKey)
	if err != nil {
		log.Printf("Jira sync comments: failed to fetch comments for %s: %v", issueKey, err)
		return
	}

	for _, c := range comments {
		var existing db.JiraCommentLog
		err := db.DB.Where("comment_id = ?", c.ID).First(&existing).Error
		if err != nil {
			createdTime := parseJiraTime(c.Created)
			authorName := c.Author.DisplayName
			if authorName == "" {
				authorName = "Unknown"
			}
			newComment := db.JiraCommentLog{
				TaskID:    issueKey,
				CommentID: c.ID,
				Author:    authorName,
				Body:      c.Body,
				CreatedAt: createdTime,
			}
			if err := db.DB.Create(&newComment).Error; err != nil {
				log.Printf("Jira sync comments: failed to save comment %s for %s: %v", c.ID, issueKey, err)
			}
		}
	}
}
