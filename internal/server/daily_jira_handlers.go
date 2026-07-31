package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
	"well-ambient/internal/kanban"
)

type dailyJiraBucketKey string

const (
	dailyJiraBucketToday    dailyJiraBucketKey = "today"
	dailyJiraBucketThreeDay dailyJiraBucketKey = "three_day"
	dailyJiraBucketSevenDay dailyJiraBucketKey = "seven_day"
)

var stableJiraIssueKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*-[0-9]+$`)

type dailyJiraAuditItem struct {
	TaskID         string                    `json:"task_id"`
	Title          string                    `json:"title"`
	Project        string                    `json:"project"`
	Assignee       string                    `json:"assignee"`
	Status         string                    `json:"status"`
	IssueType      string                    `json:"issue_type"`
	TaskCreatedAt  time.Time                 `json:"task_created_at"`
	LastUpdate     time.Time                 `json:"last_update"`
	DueDate        *time.Time                `json:"due_date"`
	AgeDays        int                       `json:"age_days"`
	Bucket         dailyJiraBucketKey        `json:"bucket"`
	Overdue        bool                      `json:"overdue"`
	DecisionLogs   string                    `json:"decision_logs"`
	DecisionEvents []db.DecisionEvent        `json:"decision_events"`
	LatestDecision *dailyJiraDecisionSummary `json:"latest_decision"`
}

type dailyJiraDecisionSummary struct {
	ID          uint      `json:"id"`
	Status      string    `json:"status"`
	Assignee    string    `json:"assignee"`
	Actor       string    `json:"actor"`
	Note        string    `json:"note"`
	DecidedAt   time.Time `json:"decided_at"`
	ReminderAt  time.Time `json:"reminder_at"`
	ReminderDue bool      `json:"reminder_due"`
}

type dailyJiraAuditBucket struct {
	Key         dailyJiraBucketKey   `json:"key"`
	Label       string               `json:"label"`
	Description string               `json:"description"`
	Count       int                  `json:"count"`
	Items       []dailyJiraAuditItem `json:"items"`
}

type dailyJiraAuditSummary struct {
	Total    int `json:"total"`
	Today    int `json:"today"`
	ThreeDay int `json:"three_day"`
	SevenDay int `json:"seven_day"`
}

type dailyJiraAuditResponse struct {
	GeneratedAt       time.Time              `json:"generated_at"`
	Summary           dailyJiraAuditSummary  `json:"summary"`
	Buckets           []dailyJiraAuditBucket `json:"buckets"`
	Assignees         []string               `json:"assignees"`
	RecentWatchCount  int                    `json:"recent_watch_count"`
	UnclassifiedCount int                    `json:"unclassified_count"`
}

type dailyJiraReviewRequest struct {
	TaskID   string `json:"task_id"`
	Decision string `json:"decision"`
	Assignee string `json:"assignee"`
	Note     string `json:"note"`
}

func (s *Server) handleGetDailyJiraAudit(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	visibility, users, err := s.loadCoreMemberVisibility()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query users for daily Jira visibility: %v", err), http.StatusInternalServerError)
		return
	}

	taskQuery, err := applyRequestProjectScope(db.DB.Model(&db.TaskTelemetry{}), r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to apply project preferences: %v", err), http.StatusInternalServerError)
		return
	}
	var tasks []db.TaskTelemetry
	if err := taskQuery.Find(&tasks).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to query daily Jira tasks: %v", err), http.StatusInternalServerError)
		return
	}
	visibleTasks := make([]db.TaskTelemetry, 0, len(tasks))
	for _, task := range tasks {
		if isDailyJiraAssigneeVisible(visibility, task.Assignee) {
			visibleTasks = append(visibleTasks, task)
		}
	}
	tasks = visibleTasks

	taskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if isDailyJiraCandidate(task) {
			taskIDs = append(taskIDs, task.TaskID)
		}
	}

	var events []db.DecisionEvent
	var decisions []db.DailyJiraDecision
	if len(taskIDs) > 0 {
		if err := db.DB.
			Where("task_id IN ? AND action LIKE ?", taskIDs, "daily_jira_%").
			Order("created_at desc").
			Find(&events).Error; err != nil {
			http.Error(w, fmt.Sprintf("Failed to query daily Jira decisions: %v", err), http.StatusInternalServerError)
			return
		}
		if err := db.DB.
			Where("task_id IN ?", taskIDs).
			Order("created_at desc").
			Find(&decisions).Error; err != nil {
			http.Error(w, fmt.Sprintf("Failed to query daily Jira decision reminders: %v", err), http.StatusInternalServerError)
			return
		}
	}

	response := buildDailyJiraAuditResponse(tasks, events, decisions, collectDailyJiraAssignees(s, visibility, users, tasks), time.Now())
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding daily Jira audit: %v", err)
	}
}

func (s *Server) handlePostDailyJiraReview(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	var req dailyJiraReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.TaskID = strings.TrimSpace(req.TaskID)
	req.Decision = strings.ToLower(strings.TrimSpace(req.Decision))
	req.Assignee = strings.TrimSpace(req.Assignee)
	req.Note = strings.TrimSpace(req.Note)
	if req.TaskID == "" {
		http.Error(w, "task_id is required", http.StatusBadRequest)
		return
	}
	if req.Decision != "follow_up" && req.Decision != "escalate" && req.Decision != "reassign" {
		http.Error(w, "decision must be follow_up, escalate, or reassign", http.StatusBadRequest)
		return
	}
	if req.Decision == "reassign" && req.Assignee == "" {
		http.Error(w, "assignee is required for reassign", http.StatusBadRequest)
		return
	}
	if utf8.RuneCountInString(req.Note) > 500 {
		http.Error(w, "note must be 500 characters or fewer", http.StatusBadRequest)
		return
	}
	if utf8.RuneCountInString(req.Assignee) > 128 {
		http.Error(w, "assignee must be 128 characters or fewer", http.StatusBadRequest)
		return
	}

	var task db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", req.TaskID).First(&task).Error; err != nil {
		http.Error(w, fmt.Sprintf("Task %s not found", req.TaskID), http.StatusNotFound)
		return
	}
	allowed, err := requestCanAccessTask(r, task.TaskID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to apply project preferences: %v", err), http.StatusInternalServerError)
		return
	}
	if !allowed {
		http.Error(w, fmt.Sprintf("Task %s not found", req.TaskID), http.StatusNotFound)
		return
	}
	if !isDailyJiraCandidate(task) || isResolvedDailyJiraStatus(task.Status) {
		http.Error(w, "Task is not an unresolved Jira item", http.StatusConflict)
		return
	}
	visibility, _, err := s.loadCoreMemberVisibility()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query daily Jira visibility: %v", err), http.StatusInternalServerError)
		return
	}
	if !isDailyJiraAssigneeVisible(visibility, task.Assignee) {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	if req.Decision == "reassign" && !visibility.includesAssignee(req.Assignee) {
		http.Error(w, "assignee is outside the configured Jira audit scope", http.StatusBadRequest)
		return
	}

	actorID := strings.TrimSpace(r.Header.Get("x-authenticated-user-id"))
	actorName := strings.TrimSpace(r.Header.Get("x-authenticated-user-name"))
	if actorName == "" {
		actorName = actorID
	}
	if actorName == "" {
		actorName = "Unknown"
	}

	now := time.Now()
	oldValue := task.Assignee
	newValue := req.Decision
	decisionLabel := "继续跟进"
	if req.Note == "" {
		req.Note = "早会 Jira 审计"
	}

	switch req.Decision {
	case "reassign":
		if strings.EqualFold(task.Assignee, req.Assignee) {
			http.Error(w, "assignee must differ from the current assignee", http.StatusConflict)
			return
		}
		task.Assignee = req.Assignee
		task.LastUpdate = now
		newValue = req.Assignee
		decisionLabel = fmt.Sprintf("转派给 %s", req.Assignee)
	case "escalate":
		decisionLabel = "升级协同"
	case "follow_up":
		decisionLabel = "继续跟进"
	}

	decisionLog := fmt.Sprintf("[%s] %s 每日 Jira 审计：%s。备注：%s",
		now.Format("2006-01-02 15:04:05"), actorName, decisionLabel, req.Note)
	if task.DecisionLogs == "" {
		task.DecisionLogs = decisionLog
	} else {
		task.DecisionLogs += "\n" + decisionLog
	}

	event := db.DecisionEvent{
		TaskID:    task.TaskID,
		Actor:     actorName,
		Action:    "daily_jira_" + req.Decision,
		OldValue:  oldValue,
		NewValue:  newValue,
		Reason:    req.Note,
		CreatedAt: now,
	}
	decisionRecord := db.DailyJiraDecision{
		TaskID:     task.TaskID,
		Status:     req.Decision,
		Assignee:   task.Assignee,
		Actor:      actorName,
		Note:       req.Note,
		ReminderAt: now.Add(dailyJiraReminderDelay(req.Decision)),
		CreatedAt:  now,
	}

	tx := db.DB.Begin()
	if tx.Error != nil {
		http.Error(w, fmt.Sprintf("Failed to begin daily Jira review: %v", tx.Error), http.StatusInternalServerError)
		return
	}
	if err := tx.Save(&task).Error; err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to save daily Jira task: %v", err), http.StatusInternalServerError)
		return
	}
	if err := tx.Create(&event).Error; err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to record daily Jira decision: %v", err), http.StatusInternalServerError)
		return
	}
	if err := tx.Create(&decisionRecord).Error; err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to schedule daily Jira reminder: %v", err), http.StatusInternalServerError)
		return
	}
	if err := tx.Commit().Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to commit daily Jira review: %v", err), http.StatusInternalServerError)
		return
	}

	if err := kanban.SyncTaskToKanban(&task); err != nil {
		log.Printf("Daily Jira review sync failed for task %s: %v", task.TaskID, err)
	}
	if req.Decision == "reassign" && s.config != nil && s.config.Jira.Enabled {
		go s.syncAssigneeToJira(task.TaskID, task.Assignee)
	}
	userdb.RecordAuditLog(db.DB, actorID, "daily_jira_"+req.Decision, "jira", task.TaskID,
		fmt.Sprintf("%s: %s", decisionLabel, req.Note), r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "success",
		"updated_task": task,
		"event":        event,
		"decision":     decisionRecord,
	}); err != nil {
		log.Printf("Error encoding daily Jira review: %v", err)
	}
}

func buildDailyJiraAuditResponse(tasks []db.TaskTelemetry, events []db.DecisionEvent, decisions []db.DailyJiraDecision, assignees []string, now time.Time) dailyJiraAuditResponse {
	eventsByTask := make(map[string][]db.DecisionEvent)
	for _, event := range events {
		if len(eventsByTask[event.TaskID]) >= 8 {
			continue
		}
		eventsByTask[event.TaskID] = append(eventsByTask[event.TaskID], event)
	}
	latestDecisionByTask := make(map[string]db.DailyJiraDecision)
	for _, decision := range decisions {
		if _, exists := latestDecisionByTask[decision.TaskID]; !exists {
			latestDecisionByTask[decision.TaskID] = decision
		}
	}

	bucketItems := map[dailyJiraBucketKey][]dailyJiraAuditItem{
		dailyJiraBucketToday:    {},
		dailyJiraBucketThreeDay: {},
		dailyJiraBucketSevenDay: {},
	}
	recentWatchCount := 0
	unclassifiedCount := 0
	today := beginningOfLocalDay(now)

	for _, task := range tasks {
		if !isDailyJiraCandidate(task) || isResolvedDailyJiraStatus(task.Status) {
			continue
		}
		if task.TaskCreatedAt.IsZero() {
			unclassifiedCount++
			continue
		}

		createdDay := beginningOfLocalDay(task.TaskCreatedAt.In(now.Location()))
		ageDays := int(today.Sub(createdDay).Hours() / 24)
		bucket, ok := dailyJiraBucketForAge(ageDays)
		if !ok {
			if ageDays == 1 || ageDays == 2 {
				recentWatchCount++
			} else {
				unclassifiedCount++
			}
			continue
		}

		item := dailyJiraAuditItem{
			TaskID:         task.TaskID,
			Title:          task.Title,
			Project:        task.Repo,
			Assignee:       task.Assignee,
			Status:         task.Status,
			IssueType:      task.IssueType,
			TaskCreatedAt:  task.TaskCreatedAt,
			LastUpdate:     task.LastUpdate,
			DueDate:        task.DueDate,
			AgeDays:        ageDays,
			Bucket:         bucket,
			DecisionLogs:   task.DecisionLogs,
			DecisionEvents: append([]db.DecisionEvent{}, eventsByTask[task.TaskID]...),
		}
		if decision, exists := latestDecisionByTask[task.TaskID]; exists {
			item.LatestDecision = &dailyJiraDecisionSummary{
				ID:          decision.ID,
				Status:      decision.Status,
				Assignee:    decision.Assignee,
				Actor:       decision.Actor,
				Note:        decision.Note,
				DecidedAt:   decision.CreatedAt,
				ReminderAt:  decision.ReminderAt,
				ReminderDue: !decision.ReminderAt.After(now),
			}
		}
		item.Overdue = task.DueDate != nil && beginningOfLocalDay(task.DueDate.In(now.Location())).Before(today)
		bucketItems[bucket] = append(bucketItems[bucket], item)
	}

	for key := range bucketItems {
		sort.SliceStable(bucketItems[key], func(i, j int) bool {
			left := bucketItems[key][i]
			right := bucketItems[key][j]
			if left.Overdue != right.Overdue {
				return left.Overdue
			}
			if left.AgeDays != right.AgeDays {
				return left.AgeDays > right.AgeDays
			}
			if !left.LastUpdate.Equal(right.LastUpdate) {
				return left.LastUpdate.Before(right.LastUpdate)
			}
			return left.TaskID < right.TaskID
		})
	}

	buckets := []dailyJiraAuditBucket{
		{
			Key:         dailyJiraBucketToday,
			Label:       "今日新增",
			Description: "今天创建且尚未解决",
			Count:       len(bucketItems[dailyJiraBucketToday]),
			Items:       bucketItems[dailyJiraBucketToday],
		},
		{
			Key:         dailyJiraBucketThreeDay,
			Label:       "3 日",
			Description: "创建 3-6 天仍未解决",
			Count:       len(bucketItems[dailyJiraBucketThreeDay]),
			Items:       bucketItems[dailyJiraBucketThreeDay],
		},
		{
			Key:         dailyJiraBucketSevenDay,
			Label:       "7 日及以上",
			Description: "创建至少 7 天仍未解决",
			Count:       len(bucketItems[dailyJiraBucketSevenDay]),
			Items:       bucketItems[dailyJiraBucketSevenDay],
		},
	}

	return dailyJiraAuditResponse{
		GeneratedAt: now,
		Summary: dailyJiraAuditSummary{
			Total:    len(bucketItems[dailyJiraBucketToday]) + len(bucketItems[dailyJiraBucketThreeDay]) + len(bucketItems[dailyJiraBucketSevenDay]),
			Today:    len(bucketItems[dailyJiraBucketToday]),
			ThreeDay: len(bucketItems[dailyJiraBucketThreeDay]),
			SevenDay: len(bucketItems[dailyJiraBucketSevenDay]),
		},
		Buckets:           buckets,
		Assignees:         assignees,
		RecentWatchCount:  recentWatchCount,
		UnclassifiedCount: unclassifiedCount,
	}
}

func dailyJiraReminderDelay(status string) time.Duration {
	if strings.EqualFold(strings.TrimSpace(status), "escalate") {
		return 4 * time.Hour
	}
	return 24 * time.Hour
}

func collectDailyJiraAssignees(s *Server, visibility coreMemberVisibility, users []userdb.User, _ []db.TaskTelemetry) []string {
	options := s.deliveryAssigneeOptions(visibility, users)
	assignees := make([]string, 0, len(options))
	for _, option := range options {
		assignees = append(assignees, option.Value)
	}
	return assignees
}

func dailyJiraBucketForAge(ageDays int) (dailyJiraBucketKey, bool) {
	switch {
	case ageDays == 0:
		return dailyJiraBucketToday, true
	case ageDays >= 3 && ageDays < 7:
		return dailyJiraBucketThreeDay, true
	case ageDays >= 7:
		return dailyJiraBucketSevenDay, true
	default:
		return "", false
	}
}

func beginningOfLocalDay(value time.Time) time.Time {
	location := value.Location()
	year, month, day := value.In(location).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, location)
}

func isResolvedDailyJiraStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "done", "archived", "resolved", "closed":
		return true
	default:
		return false
	}
}

func isDailyJiraAssigneeVisible(visibility coreMemberVisibility, assignee string) bool {
	normalized := strings.ToLower(strings.TrimSpace(assignee))
	if normalized == "" || normalized == "-" || normalized == "unassigned" || normalized == "未指派" {
		return true
	}
	return visibility.includesAssignee(assignee)
}

func isDailyJiraCandidate(task db.TaskTelemetry) bool {
	key := strings.ToUpper(strings.TrimSpace(task.TaskID))
	if !stableJiraIssueKeyPattern.MatchString(key) || strings.HasPrefix(key, "TASK-") || strings.HasPrefix(key, "DEMAND-") {
		return false
	}
	separator := strings.LastIndex(key, "-")
	if separator <= 0 {
		return false
	}
	projectKey := key[:separator]
	repo := strings.ToUpper(strings.TrimSpace(task.Repo))
	if strings.Contains(repo, "("+projectKey+")") {
		return true
	}
	return strings.TrimSpace(task.Creator) == "" && repo != "" && repo != "-"
}
