package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"well-ambient/internal/dailyjira"
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
	Reporter       string                    `json:"reporter"`
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
	Page              dailyJiraPageMeta      `json:"page"`
}

type dailyJiraPageMeta struct {
	Bucket     dailyJiraBucketKey `json:"bucket"`
	Limit      int                `json:"limit"`
	Generation int64              `json:"generation"`
	SearchMode string             `json:"search_mode"`
	NextCursor string             `json:"next_cursor,omitempty"`
	HasMore    bool               `json:"has_more"`
}

type dailyJiraReviewRequest struct {
	TaskID       string `json:"task_id"`
	Decision     string `json:"decision"`
	Assignee     string `json:"assignee"`
	AssigneeMode string `json:"assignee_mode"`
	Note         string `json:"note"`
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

	projectKeys, err := requestProjectPreferenceKeys(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to apply project preferences: %v", err), http.StatusInternalServerError)
		return
	}
	bucket := parseDailyJiraBucket(r.URL.Query().Get("bucket"))
	limit := 100
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil || parsed <= 0 {
			http.Error(w, "limit must be a positive integer", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	page, err := dailyjira.NewReader(db.DB).ReadPage(r.Context(), dailyjira.Query{
		Bucket: dailyJiraReadBucket(bucket),
		Search: r.URL.Query().Get("search"),
		Cursor: strings.TrimSpace(r.URL.Query().Get("cursor")),
		Limit:  limit,
		Scope: dailyjira.Scope{
			ProjectKeys:     projectKeys,
			Assignees:       dailyJiraVisibilityAssignees(visibility),
			FilterAssignees: visibility.filter.enabled,
		},
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, dailyjira.ErrInvalidCursor) {
			status = http.StatusBadRequest
		} else if errors.Is(err, dailyjira.ErrStaleCursor) {
			status = http.StatusConflict
		}
		http.Error(w, fmt.Sprintf("Failed to query daily Jira page: %v", err), status)
		return
	}

	tasks := dailyJiraPageTasks(page.Items)

	taskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if isDailyJiraCandidate(task) {
			taskIDs = append(taskIDs, task.TaskID)
		}
	}

	events, decisions, err := loadDailyJiraPageEnrichments(taskIDs)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query daily Jira page enrichment: %v", err), http.StatusInternalServerError)
		return
	}

	response := buildDailyJiraAuditResponse(tasks, events, decisions, collectDailyJiraAssignees(s, visibility, users, tasks), page.GeneratedAt)
	response.Summary = dailyJiraAuditSummary{
		Total:    int(page.Summary.Total),
		Today:    int(page.Summary.Today),
		ThreeDay: int(page.Summary.ThreeDay),
		SevenDay: int(page.Summary.SevenDay),
	}
	response.RecentWatchCount = int(page.Summary.RecentWatch)
	response.UnclassifiedCount = int(page.Summary.Unclassified)
	for index := range response.Buckets {
		response.Buckets[index].Count = dailyJiraBucketCount(response.Buckets[index].Key, page.Summary)
		if response.Buckets[index].Key != bucket {
			response.Buckets[index].Items = []dailyJiraAuditItem{}
		}
	}
	response.Page = dailyJiraPageMeta{
		Bucket:     bucket,
		Limit:      min(limit, 100),
		Generation: page.Generation,
		SearchMode: page.SearchMode,
		NextCursor: page.NextCursor,
		HasMore:    page.HasMore,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding daily Jira audit: %v", err)
	}
}

func loadDailyJiraPageEnrichments(taskIDs []string) ([]db.DecisionEvent, []db.DailyJiraDecision, error) {
	if len(taskIDs) == 0 {
		return []db.DecisionEvent{}, []db.DailyJiraDecision{}, nil
	}
	var events []db.DecisionEvent
	if err := db.DB.Raw(`WITH ranked_events AS (
			SELECT decision_events.*,
				ROW_NUMBER() OVER (
					PARTITION BY task_id ORDER BY created_at DESC, id DESC
				) AS daily_jira_rank
			FROM decision_events INDEXED BY idx_daily_jira_event_task_action_created
			WHERE task_id IN ? AND action GLOB ?
		)
		SELECT id, task_id, actor, action, old_value, new_value, reason, confidence, created_at
		FROM ranked_events
		WHERE daily_jira_rank <= 8
		ORDER BY created_at DESC, id DESC`, taskIDs, "daily_jira_*").Scan(&events).Error; err != nil {
		return nil, nil, fmt.Errorf("read Daily Jira audit events: %w", err)
	}
	var decisions []db.DailyJiraDecision
	if err := db.DB.Raw(`WITH ranked_decisions AS (
			SELECT daily_jira_decisions.*,
				ROW_NUMBER() OVER (
					PARTITION BY task_id ORDER BY created_at DESC, id DESC
				) AS daily_jira_rank
			FROM daily_jira_decisions INDEXED BY idx_daily_jira_decision_task_created
			WHERE task_id IN ?
		)
		SELECT id, task_id, status, assignee, actor, note, reminder_at, created_at
		FROM ranked_decisions
		WHERE daily_jira_rank = 1
		ORDER BY created_at DESC, id DESC`, taskIDs).Scan(&decisions).Error; err != nil {
		return nil, nil, fmt.Errorf("read Daily Jira latest decisions: %w", err)
	}
	return events, decisions, nil
}

func parseDailyJiraBucket(value string) dailyJiraBucketKey {
	switch dailyJiraBucketKey(strings.TrimSpace(value)) {
	case dailyJiraBucketToday:
		return dailyJiraBucketToday
	case dailyJiraBucketThreeDay:
		return dailyJiraBucketThreeDay
	case dailyJiraBucketSevenDay:
		return dailyJiraBucketSevenDay
	default:
		return dailyJiraBucketSevenDay
	}
}

func dailyJiraReadBucket(bucket dailyJiraBucketKey) dailyjira.Bucket {
	switch bucket {
	case dailyJiraBucketToday:
		return dailyjira.BucketToday
	case dailyJiraBucketThreeDay:
		return dailyjira.BucketThreeDay
	default:
		return dailyjira.BucketSevenDay
	}
}

func dailyJiraVisibilityAssignees(visibility coreMemberVisibility) []string {
	if !visibility.filter.enabled {
		return nil
	}
	assignees := make([]string, 0, len(visibility.filter.keys))
	for key := range visibility.filter.keys {
		assignees = append(assignees, key)
	}
	sort.Strings(assignees)
	return assignees
}

func dailyJiraPageTasks(items []dailyjira.Item) []db.TaskTelemetry {
	tasks := make([]db.TaskTelemetry, 0, len(items))
	for _, item := range items {
		tasks = append(tasks, db.TaskTelemetry{
			TaskID:          item.TaskID,
			ProjectKey:      item.ProjectKey,
			Source:          "jira",
			Title:           item.Title,
			Repo:            item.Project,
			Assignee:        item.Assignee,
			JiraReporter:    item.Reporter,
			Status:          item.Status,
			IssueType:       item.IssueType,
			TaskCreatedAt:   item.TaskCreatedAt,
			LastUpdate:      item.LastActivityAt,
			SourceUpdatedAt: item.LastActivityAt,
			DueDate:         item.DueDate,
			DecisionLogs:    item.DecisionLogs,
		})
	}
	return tasks
}

func dailyJiraBucketCount(bucket dailyJiraBucketKey, summary dailyjira.Summary) int {
	switch bucket {
	case dailyJiraBucketToday:
		return int(summary.Today)
	case dailyJiraBucketThreeDay:
		return int(summary.ThreeDay)
	case dailyJiraBucketSevenDay:
		return int(summary.SevenDay)
	default:
		return 0
	}
}

func (s *Server) handlePostDailyJiraSync(w http.ResponseWriter, _ *http.Request) {
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}
	if s == nil || s.config == nil || !s.config.Jira.Enabled {
		http.Error(w, "Jira synchronization is not enabled", http.StatusConflict)
		return
	}

	s.syncJiraTasks()
	state, err := loadJiraInboundSyncState()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read Jira synchronization result: %v", err), http.StatusInternalServerError)
		return
	}
	if strings.TrimSpace(state.LastError) != "" {
		http.Error(w, fmt.Sprintf("Jira synchronization failed: %s", state.LastError), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":            "success",
		"last_started_at":   state.LastStartedAt,
		"last_succeeded_at": state.LastSucceededAt,
		"issue_count":       state.LastIssueCount,
		"changed_count":     state.LastChangedCount,
	}); err != nil {
		log.Printf("Error encoding Daily Jira sync result: %v", err)
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
	req.AssigneeMode = strings.ToLower(strings.TrimSpace(req.AssigneeMode))
	req.Note = strings.TrimSpace(req.Note)
	if req.TaskID == "" {
		http.Error(w, "task_id is required", http.StatusBadRequest)
		return
	}
	if req.Decision != "follow_up" && req.Decision != "escalate" && req.Decision != "reassign" {
		http.Error(w, "decision must be follow_up, escalate, or reassign", http.StatusBadRequest)
		return
	}
	if req.Note == "" {
		http.Error(w, "decision comment is required", http.StatusBadRequest)
		return
	}
	if req.Decision == "reassign" && req.AssigneeMode == "" {
		req.AssigneeMode = "specified"
	}
	if req.Decision == "reassign" && req.AssigneeMode != "reporter" && req.AssigneeMode != "specified" {
		http.Error(w, "assignee_mode must be reporter or specified", http.StatusBadRequest)
		return
	}
	if req.Decision == "reassign" && req.AssigneeMode == "specified" && req.Assignee == "" {
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
	if req.Decision == "reassign" && req.AssigneeMode == "specified" && !visibility.includesAssignee(req.Assignee) {
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
	var jiraAssignee *string

	switch req.Decision {
	case "reassign":
		targetAssignee := req.Assignee
		jiraUsername := ""
		if req.AssigneeMode == "reporter" {
			targetAssignee = strings.TrimSpace(task.JiraReporter)
			jiraUsername = strings.TrimSpace(task.JiraReporterUser)
			if targetAssignee == "" || jiraUsername == "" {
				http.Error(w, "Jira reporter is unavailable for this item", http.StatusConflict)
				return
			}
		} else {
			resolvedUsername, resolveErr := resolveJiraAssigneeUsername(targetAssignee)
			if resolveErr != nil {
				http.Error(w, resolveErr.Error(), http.StatusBadRequest)
				return
			}
			jiraUsername = resolvedUsername
		}
		if strings.EqualFold(task.Assignee, targetAssignee) {
			http.Error(w, "assignee must differ from the current assignee", http.StatusConflict)
			return
		}
		task.Assignee = targetAssignee
		task.LastUpdate = now
		newValue = targetAssignee
		decisionLabel = fmt.Sprintf("转派给 %s", targetAssignee)
		jiraAssignee = &jiraUsername
	case "escalate":
		decisionLabel = "升级协同"
	case "follow_up":
		decisionLabel = "继续跟进"
	}

	jiraSyncStatus := "disabled"
	if s.config != nil && s.config.Jira.Enabled {
		if s.jiraDailyReviewSync == nil {
			http.Error(w, "Jira decision synchronization is unavailable", http.StatusServiceUnavailable)
			return
		}
		jiraComment := fmt.Sprintf("每日 Jira 决策：%s\n%s\n操作人：%s", decisionLabel, req.Note, actorName)
		if err := s.jiraDailyReviewSync(task.TaskID, jiraAssignee, jiraComment); err != nil {
			http.Error(w, fmt.Sprintf("Failed to update Jira decision: %v", err), http.StatusBadGateway)
			return
		}
		jiraSyncStatus = "completed"
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
	BroadcastTelemetryUpdated(task.TaskID)
	userdb.RecordAuditLog(db.DB, actorID, "daily_jira_"+req.Decision, "jira", task.TaskID,
		fmt.Sprintf("%s: %s", decisionLabel, req.Note), r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "success",
		"updated_task": task,
		"event":        event,
		"decision":     decisionRecord,
		"jira_sync":    jiraSyncStatus,
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
			Reporter:       task.JiraReporter,
			Status:         task.Status,
			IssueType:      task.IssueType,
			TaskCreatedAt:  task.TaskCreatedAt,
			LastUpdate:     dailyJiraRecentActivity(task),
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

func dailyJiraRecentActivity(task db.TaskTelemetry) time.Time {
	if !task.SourceUpdatedAt.IsZero() {
		return task.SourceUpdatedAt
	}
	return task.LastUpdate
}

func dailyJiraReminderDelay(status string) time.Duration {
	if strings.EqualFold(strings.TrimSpace(status), "escalate") {
		return 4 * time.Hour
	}
	return 24 * time.Hour
}

func collectDailyJiraAssignees(s *Server, visibility coreMemberVisibility, users []userdb.User, tasks []db.TaskTelemetry) []string {
	if visibility.filter.enabled {
		options := s.deliveryAssigneeOptions(visibility, users)
		assignees := make([]string, 0, len(options))
		for _, option := range options {
			assignees = append(assignees, option.Value)
		}
		return assignees
	}

	seen := make(map[string]string, len(users)+len(tasks))
	add := func(raw string) {
		value := strings.TrimSpace(raw)
		if value == "" || value == "-" || value == "未指派" || strings.EqualFold(value, "unassigned") {
			return
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; !exists {
			seen[key] = value
		}
	}
	for _, user := range users {
		identity := kpiIdentityFromUser(user)
		add(identity.Name)
	}
	for _, task := range tasks {
		add(task.Assignee)
	}

	assignees := make([]string, 0, len(seen))
	for _, value := range seen {
		assignees = append(assignees, value)
	}
	sort.Slice(assignees, func(i, j int) bool {
		return strings.ToLower(assignees[i]) < strings.ToLower(assignees[j])
	})
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
	case "done", "archived", "resolved", "closed", "completed", "已完成", "已关闭":
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
	source := strings.ToLower(strings.TrimSpace(task.Source))
	if source != "" {
		return source == "jira"
	}
	separator := strings.LastIndex(key, "-")
	if separator <= 0 {
		return false
	}
	projectKey := key[:separator]
	repo := strings.ToUpper(strings.TrimSpace(task.Repo))
	return strings.Contains(repo, "("+projectKey+")")
}
