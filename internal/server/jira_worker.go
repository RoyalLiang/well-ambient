package server

import (
	"fmt"
	"log"
	"strings"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
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
			if issue.Fields.Assignee.DisplayName != "" {
				assigneeName = issue.Fields.Assignee.DisplayName
			} else if issue.Fields.Assignee.Name != "" {
				assigneeName = issue.Fields.Assignee.Name
			} else {
				assigneeName = issue.Fields.Assignee.EmailAddress
			}
		}

		jiraMappedStatus := mapJiraStatus(issue.Fields.Status.Name)
		rawType := strings.ToLower(strings.TrimSpace(issue.Fields.IssueType.Name))
		issueType := "task"
		if rawType == "bug" || rawType == "缺陷" || rawType == "故障" || rawType == "defect" {
			issueType = "bug"
		}
		createdTime := parseJiraTime(issue.Fields.Created)

		// Check if task exists in SQLite DB
		var existing db.TaskTelemetry
		err := db.DB.Where("task_id = ?", issue.Key).First(&existing).Error
		if err != nil {
			// Insert new task
			newTelemetry := db.TaskTelemetry{
				TaskID:        issue.Key,
				Title:         issue.Fields.Summary,
				Repo:          "-",
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
				existing.Assignee = assigneeName
				hasChanges = true
			}

			if existing.IssueType != issueType {
				existing.IssueType = issueType
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
	}

	// =================================================================
	// Phase 2: Active Local Jira Tasks Keep-Alive & Correction Sync
	// =================================================================
	var activeLocalTasks []db.TaskTelemetry
	// Query local active Jira issues (contains '-' in TaskID and status != 'done')
	if err := db.DB.Where("status != 'done' AND task_id LIKE '%-%'").Find(&activeLocalTasks).Error; err == nil && len(activeLocalTasks) > 0 {
		var activeKeys []string
		for _, t := range activeLocalTasks {
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
					if issue.Fields.Assignee.DisplayName != "" {
						assigneeName = issue.Fields.Assignee.DisplayName
					} else if issue.Fields.Assignee.Name != "" {
						assigneeName = issue.Fields.Assignee.Name
					} else {
						assigneeName = issue.Fields.Assignee.EmailAddress
					}
				}

				jiraMappedStatus := mapJiraStatus(issue.Fields.Status.Name)
				rawType := strings.ToLower(strings.TrimSpace(issue.Fields.IssueType.Name))
				issueType := "task"
				if rawType == "bug" || rawType == "缺陷" || rawType == "故障" || rawType == "defect" {
					issueType = "bug"
				}

				var existing db.TaskTelemetry
				if err := db.DB.Where("task_id = ?", issue.Key).First(&existing).Error; err == nil {
					hasChanges := false
					if existing.Assignee != assigneeName {
						log.Printf("Jira sync keep-alive: assignee of %s corrected from %s -> %s (even if out of configuration range)", issue.Key, existing.Assignee, assigneeName)
						existing.Assignee = assigneeName
						hasChanges = true
					}
					if existing.Title != issue.Fields.Summary {
						existing.Title = issue.Fields.Summary
						hasChanges = true
					}
					if existing.IssueType != issueType {
						existing.IssueType = issueType
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
				}
			}
		}
	}
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
