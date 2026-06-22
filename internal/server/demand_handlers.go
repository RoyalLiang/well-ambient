package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
	"well-ambient/internal/kanban"
)

type CreateDemandRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Assignee    string `json:"assignee"`
	DueDate     string `json:"due_date"` // YYYY-MM-DD
	Repo        string `json:"repo"`
	CreatorDept string `json:"creator_dept"`
}

type DemandOptionsResponse struct {
	Assignees []string `json:"assignees"`
	Projects  []string `json:"projects"`
}

func isMissingDepartment(dept string) bool {
	dept = strings.TrimSpace(dept)
	return dept == "" || dept == "未分配" || dept == "无部门"
}

// handleGetDemandOptions returns low-risk form metadata for demand creation.
func (s *Server) handleGetDemandOptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	assignees := make(map[string]string)
	projects := make(map[string]string)
	addDemandOption(assignees, r.Header.Get("x-authenticated-user-name"))
	addDemandOption(assignees, r.Header.Get("x-authenticated-user-id"))

	var users []userdb.User
	if err := db.DB.Order("name asc, username asc").Find(&users).Error; err == nil {
		for _, u := range users {
			addDemandOption(assignees, firstNonBlank(u.Name, u.Username, u.Email))
		}
	}

	var tasks []db.TaskTelemetry
	if err := db.DB.Select("assignee", "repo").Find(&tasks).Error; err == nil {
		for _, task := range tasks {
			addDemandOption(assignees, task.Assignee)
			addDemandOption(projects, task.Repo)
		}
	}

	if s.config != nil {
		for _, repo := range s.config.GitLab.Repos {
			addDemandOption(projects, firstNonBlank(repo.Name, repo.Path, repo.ProjectID))
		}
		for _, project := range s.config.Jira.SyncProjects {
			addDemandOption(projects, project)
		}
		for _, user := range s.config.Jira.SyncUsers {
			addDemandOption(assignees, user)
		}
		for _, user := range extractJIRAAssignees(s.config.Jira.CustomJQL) {
			addDemandOption(assignees, user)
		}
		for _, project := range extractJIRAProjects(s.config.Jira.CustomJQL) {
			addDemandOption(projects, project)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DemandOptionsResponse{
		Assignees: sortedDemandOptions(assignees),
		Projects:  sortedDemandOptions(projects),
	})
}

func addDemandOption(options map[string]string, value string) {
	value = strings.TrimSpace(strings.Trim(value, `"'`))
	if value == "" || value == "-" || value == "未指派" || value == "unassigned" {
		return
	}
	key := strings.ToLower(value)
	if _, exists := options[key]; !exists {
		options[key] = value
	}
}

func sortedDemandOptions(options map[string]string) []string {
	values := make([]string, 0, len(options))
	for _, value := range options {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		return strings.ToLower(values[i]) < strings.ToLower(values[j])
	})
	return values
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeScheduleEstimateSource(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ai_deconstruct":
		return "ai_deconstruct"
	case "manual", "manual_adjusted":
		return "manual_adjusted"
	default:
		return ""
	}
}

func extractJIRAAssignees(jql string) []string {
	match := regexp.MustCompile(`(?i)assignee\s+in\s*\(([^)]*)\)`).FindStringSubmatch(jql)
	if len(match) < 2 {
		return nil
	}
	return splitJIRAListValues(match[1])
}

func extractJIRAProjects(jql string) []string {
	inMatch := regexp.MustCompile(`(?i)project\s+in\s*\(([^)]*)\)`).FindStringSubmatch(jql)
	if len(inMatch) >= 2 {
		return splitJIRAListValues(inMatch[1])
	}
	eqMatch := regexp.MustCompile(`(?i)project\s*=\s*("[^"]+"|'[^']+'|[A-Za-z0-9_.-]+)`).FindStringSubmatch(jql)
	if len(eqMatch) >= 2 {
		project := strings.TrimSpace(strings.Trim(eqMatch[1], `"'`))
		if project != "" {
			return []string{project}
		}
	}
	return nil
}

func splitJIRAListValues(raw string) []string {
	rawValues := strings.Split(raw, ",")
	values := make([]string, 0, len(rawValues))
	for _, rawValue := range rawValues {
		value := strings.TrimSpace(strings.Trim(rawValue, `"'`))
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

// handleCreateDemand creates a new demand item, generates notification and syncs to markdown
func (s *Server) handleCreateDemand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	actorUsername := r.Header.Get("x-authenticated-user-id")

	var req CreateDemandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Title == "" || req.Assignee == "" {
		http.Error(w, "Title and assignee are required", http.StatusBadRequest)
		return
	}

	// Resolve Creator name & department
	creatorName := actorUsername
	creatorDept := "未分配"
	headerDept := strings.TrimSpace(r.Header.Get("x-authenticated-user-department"))
	requestDept := strings.TrimSpace(req.CreatorDept)
	if actorUsername != "" {
		var u userdb.User
		if err := db.DB.Where("username = ?", actorUsername).First(&u).Error; err == nil {
			creatorName = u.Name
			dbDept := strings.TrimSpace(u.Department)
			if !isMissingDepartment(dbDept) {
				creatorDept = dbDept
			} else if !isMissingDepartment(headerDept) {
				creatorDept = headerDept
				u.Department = headerDept
				db.DB.Save(&u)
			} else if !isMissingDepartment(requestDept) {
				creatorDept = requestDept
				u.Department = requestDept
				db.DB.Save(&u)
			}
		}
	}
	if isMissingDepartment(creatorDept) && !isMissingDepartment(headerDept) {
		creatorDept = headerDept
	}
	if isMissingDepartment(creatorDept) && !isMissingDepartment(requestDept) {
		creatorDept = requestDept
	}

	// Resolve DueDate if provided
	var dueDate *time.Time
	if req.DueDate != "" {
		parsed, err := time.Parse("2006-01-02", req.DueDate)
		if err == nil {
			dueDate = &parsed
		} else {
			http.Error(w, fmt.Sprintf("Invalid due_date format (must be YYYY-MM-DD): %v", err), http.StatusBadRequest)
			return
		}
	}

	repo := req.Repo
	if repo == "" {
		repo = "-"
	}

	// Auto-generate TaskID for demand (format: DEMAND-001)
	var count int64
	db.DB.Model(&db.TaskTelemetry{}).Where("task_id LIKE ?", "DEMAND-%").Count(&count)
	idNum := count + 1
	taskID := fmt.Sprintf("DEMAND-%03d", idNum)
	for {
		var exist int64
		db.DB.Model(&db.TaskTelemetry{}).Where("task_id = ?", taskID).Count(&exist)
		if exist == 0 {
			break
		}
		idNum++
		taskID = fmt.Sprintf("DEMAND-%03d", idNum)
	}

	telemetry := db.TaskTelemetry{
		TaskID:        taskID,
		Title:         req.Title,
		Description:   req.Description,
		Repo:          repo,
		Assignee:      req.Assignee,
		Creator:       creatorName,
		CreatorDept:   creatorDept,
		Branch:        "-",
		LastCommit:    "-",
		Status:        "backlog", // Initial status for demand is backlog
		IssueType:     "demand",
		TaskCreatedAt: time.Now(),
		LastUpdate:    time.Now(),
		DueDate:       dueDate,
	}

	if err := db.DB.Create(&telemetry).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to save demand telemetry: %v", err), http.StatusInternalServerError)
		return
	}

	// Create persistent Notification for assignee
	notif := db.Notification{
		Type:      "demand_assigned",
		TaskID:    taskID,
		Title:     "📋 收到新需求指派",
		Message:   fmt.Sprintf("您收到一个新指派的需求: %s。请尽快进行排期。", req.Title),
		Assignee:  req.Assignee,
		CreatedAt: time.Now(),
	}
	db.DB.Create(&notif)

	// Broadcast SSE to trigger update on client
	BroadcastNotifications()

	// Sync to markdown task_status.md
	if err := kanban.SyncTaskToKanban(&telemetry); err != nil {
		log.Printf("Failed to sync demand to kanban file: %v", err)
	}

	// Record audit log
	userdb.RecordAuditLog(db.DB, actorUsername, "demand_create", "demand", taskID,
		fmt.Sprintf("创建需求 %s '%s' 并指派给 %s", taskID, req.Title, req.Assignee), r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Demand created successfully",
		"task_id": taskID,
	})
}

// handleDeleteDemand deletes a demand completely and updates markdown
func (s *Server) handleDeleteDemand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	actorUsername := r.Header.Get("x-authenticated-user-id")
	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	var telemetry db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", taskID).First(&telemetry).Error; err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Verify permissions: Creator or Admin/Super Admin
	isAdmin := s.checkIsAdmin(actorUsername)
	isCreator := strings.ToLower(telemetry.Creator) == strings.ToLower(actorUsername) ||
		strings.Contains(strings.ToLower(actorUsername), strings.ToLower(telemetry.Creator))

	if !isAdmin && !isCreator {
		http.Error(w, "Access Denied: Only creator or administrators can delete this demand", http.StatusForbidden)
		return
	}

	// 1. Delete from database
	if err := db.DB.Delete(&telemetry).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete demand: %v", err), http.StatusInternalServerError)
		return
	}

	// 2. Sync to markdown (Remove task from all sections)
	content, err := os.ReadFile(kanban.KanbanFilePath)
	if err == nil {
		if board, err := kanban.ParseKanbanBoard(string(content)); err == nil {
			board.RemoveTask(taskID)
			if err := os.WriteFile(kanban.KanbanFilePath, []byte(board.String()), 0644); err != nil {
				log.Printf("Failed to update markdown file on delete: %v", err)
			}
		}
	}

	// Broadcast SSE update
	BroadcastNotifications()

	// Record audit log
	userdb.RecordAuditLog(db.DB, actorUsername, "demand_delete", "demand", taskID,
		fmt.Sprintf("物理删除了需求 %s '%s'", taskID, telemetry.Title), r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Demand deleted successfully",
	})
}

// handleArchiveDemand archives a demand by moving status to archived and removing from markdown
func (s *Server) handleArchiveDemand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	actorUsername := r.Header.Get("x-authenticated-user-id")

	var req struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TaskID == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	var telemetry db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", req.TaskID).First(&telemetry).Error; err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Verify permissions: Creator, Assignee, or Admin/Super Admin
	isAdmin := s.checkIsAdmin(actorUsername)
	isCreator := strings.ToLower(telemetry.Creator) == strings.ToLower(actorUsername) ||
		strings.Contains(strings.ToLower(actorUsername), strings.ToLower(telemetry.Creator))
	isAssignee := strings.ToLower(telemetry.Assignee) == strings.ToLower(actorUsername) ||
		strings.Contains(strings.ToLower(actorUsername), strings.ToLower(telemetry.Assignee))

	if !isAdmin && !isCreator && !isAssignee {
		http.Error(w, "Access Denied: Only creator, assignee, or administrators can archive this demand", http.StatusForbidden)
		return
	}

	// 1. Move status to archived in DB
	telemetry.Status = "archived"
	telemetry.LastUpdate = time.Now()
	if err := db.DB.Save(&telemetry).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to archive demand: %v", err), http.StatusInternalServerError)
		return
	}

	// 2. Remove from markdown kanban file (since archived should not be shown on active lanes)
	content, err := os.ReadFile(kanban.KanbanFilePath)
	if err == nil {
		if board, err := kanban.ParseKanbanBoard(string(content)); err == nil {
			board.RemoveTask(req.TaskID)
			if err := os.WriteFile(kanban.KanbanFilePath, []byte(board.String()), 0644); err != nil {
				log.Printf("Failed to update markdown file on archive: %v", err)
			}
		}
	}

	// Broadcast SSE update
	BroadcastNotifications()

	// Record audit log
	userdb.RecordAuditLog(db.DB, actorUsername, "demand_archive", "demand", req.TaskID,
		fmt.Sprintf("归档了需求 %s '%s'", req.TaskID, telemetry.Title), r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Demand archived successfully",
	})
}

type ScheduleTaskRequest struct {
	TaskID         string  `json:"task_id"`
	Branch         string  `json:"branch"`
	DueDate        string  `json:"due_date"` // YYYY-MM-DD
	Status         string  `json:"status"`   // backlog, progress, review, done
	TaskGroupID    string  `json:"task_group_id"`
	EstimateDays   float64 `json:"estimate_days"`
	EstimateHours  float64 `json:"estimate_hours"`
	Difficulty     string  `json:"difficulty"`
	EstimateSource string  `json:"estimate_source"`
}

// handleScheduleTask updates the task's due date, development branch and handles notification read status
func (s *Server) handleScheduleTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	actorUsername := r.Header.Get("x-authenticated-user-id")
	actorName := r.Header.Get("x-authenticated-user-name")
	if actorName == "" {
		actorName = actorUsername
	}

	var req ScheduleTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TaskID == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	var telemetry db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", req.TaskID).First(&telemetry).Error; err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Check permissions: either the assignee or an admin/super_admin can schedule
	isAdmin := s.checkIsAdmin(actorUsername)
	isAssignee := strings.ToLower(telemetry.Assignee) == strings.ToLower(actorName) ||
		strings.ToLower(telemetry.Assignee) == strings.ToLower(actorUsername) ||
		strings.Contains(strings.ToLower(actorUsername), strings.ToLower(telemetry.Assignee))

	if !isAdmin && !isAssignee {
		http.Error(w, "Access Denied: Only assignee or managers can schedule this task", http.StatusForbidden)
		return
	}

	// Resolve DueDate if provided
	if req.DueDate != "" {
		parsed, err := time.Parse("2006-01-02", req.DueDate)
		if err == nil {
			telemetry.DueDate = &parsed
		} else {
			http.Error(w, fmt.Sprintf("Invalid due_date format: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Update Branch if provided
	if req.Branch != "" {
		telemetry.Branch = req.Branch
	}

	// Update Status if provided
	if req.Status != "" {
		req.Status = strings.ToLower(req.Status)
		if req.Status == "backlog" || req.Status == "progress" || req.Status == "review" || req.Status == "done" {
			// Manage CompletedAt
			if req.Status == "done" {
				if telemetry.Status != "done" || telemetry.CompletedAt == nil {
					now := time.Now()
					telemetry.CompletedAt = &now
				}
			} else {
				telemetry.CompletedAt = nil
			}
			telemetry.Status = req.Status
		}
	}

	// Update TaskGroupID if provided (use "-" to unbind)
	if req.TaskGroupID == "-" {
		telemetry.TaskGroupID = ""
	} else if req.TaskGroupID != "" {
		telemetry.TaskGroupID = req.TaskGroupID
	}

	estimateSource := normalizeScheduleEstimateSource(req.EstimateSource)
	estimateRequested := estimateSource != "" || req.EstimateHours > 0 || req.EstimateDays > 0 || strings.TrimSpace(req.Difficulty) != ""
	if estimateRequested {
		telemetry.EstimateHours = 0
		telemetry.EstimateDays = 0
		telemetry.Difficulty = ""
		if req.EstimateHours > 0 {
			telemetry.EstimateHours = roundOneDecimal(req.EstimateHours)
		}
		if req.EstimateDays > 0 {
			telemetry.EstimateDays = roundOneDecimal(req.EstimateDays)
		}
		if difficulty := normalizeDifficulty(req.Difficulty); difficulty != "" {
			telemetry.Difficulty = difficulty
		}
		if estimateSource == "" {
			estimateSource = "manual_adjusted"
		}
		telemetry.EstimateSource = estimateSource
	}

	telemetry.LastUpdate = time.Now()

	if err := db.DB.Save(&telemetry).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to update schedule: %v", err), http.StatusInternalServerError)
		return
	}

	// Dismiss the assignment notifications for this task for this user
	var notifs []db.Notification
	err := db.DB.Where("task_id = ? AND type = ?", telemetry.TaskID, "demand_assigned").Find(&notifs).Error
	if err == nil && len(notifs) > 0 {
		for _, n := range notifs {
			key := fmt.Sprintf("%s_%d", n.Type, n.ID)
			// Upsert user notification state as dismissed
			var state db.UserNotificationState
			errState := db.DB.Where("notification_key = ? AND user_id = ?", key, actorUsername).First(&state).Error
			if errState != nil {
				state = db.UserNotificationState{
					NotificationKey: key,
					UserID:          actorUsername,
					Status:          "dismissed",
					UpdatedAt:       time.Now(),
				}
				db.DB.Create(&state)
			} else {
				state.Status = "dismissed"
				state.UpdatedAt = time.Now()
				db.DB.Save(&state)
			}
		}
	}

	// Broadcast SSE update
	BroadcastNotifications()

	// Sync to markdown
	if err := kanban.SyncTaskToKanban(&telemetry); err != nil {
		log.Printf("Failed to sync scheduled task to kanban file: %v", err)
	}

	// Record audit log
	userdb.RecordAuditLog(db.DB, actorUsername, "demand_schedule", "demand", telemetry.TaskID,
		fmt.Sprintf("对任务 %s 进行了排期: 分支=%s, 截止时间=%s, 状态=%s", telemetry.TaskID, telemetry.Branch, req.DueDate, telemetry.Status), r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Task scheduled successfully",
	})
}

// checkIsAdmin checks if user has demands:write permission
func (s *Server) checkIsAdmin(username string) bool {
	if username == "" {
		return false
	}
	var u userdb.User
	if err := db.DB.Where("username = ?", username).First(&u).Error; err != nil {
		return false
	}
	var permCount int64
	db.DB.Table("user_group_memberships").
		Joins("join group_permissions on group_permissions.user_group_id = user_group_memberships.user_group_id").
		Joins("join permissions on permissions.id = group_permissions.permission_id").
		Where("user_group_memberships.user_id = ? AND permissions.code = ?", u.ID, "demands:write").
		Count(&permCount)
	return permCount > 0
}
