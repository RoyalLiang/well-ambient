package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

type ScheduleResponseDTO struct {
	GeneratedAt string             `json:"generated_at"`
	Summary     ScheduleSummaryDTO `json:"summary"`
	Items       []ScheduleItemDTO  `json:"items"`
}

type ScheduleSummaryDTO struct {
	Total       int `json:"total"`
	Scheduled   int `json:"scheduled"`
	Unscheduled int `json:"unscheduled"`
	InProgress  int `json:"in_progress"`
	Review      int `json:"review"`
	Done        int `json:"done"`
	Overdue     int `json:"overdue"`
	DueSoon     int `json:"due_soon"`
	Stale       int `json:"stale"`
}

type ScheduleItemDTO struct {
	DemandID      string  `json:"demand_id"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Assignee      string  `json:"assignee"`
	Department    string  `json:"department"`
	Repo          string  `json:"repo"`
	Branch        string  `json:"branch"`
	Status        string  `json:"status"`
	TaskGroupID   string  `json:"task_group_id"`
	Scheduled     bool    `json:"scheduled"`
	DueDate       string  `json:"due_date"`
	CreatedAt     string  `json:"created_at"`
	LastUpdate    string  `json:"last_update"`
	CompletedAt   string  `json:"completed_at,omitempty"`
	MRURL         string  `json:"mr_url,omitempty"`
	EstimateDays  float64 `json:"estimate_days"`
	EstimateHours float64 `json:"estimate_hours"`
	Difficulty    string  `json:"difficulty"`
	RiskLevel     string  `json:"risk_level"`
	RiskLabel     string  `json:"risk_label"`
	RiskReason    string  `json:"risk_reason"`
	RiskRank      int     `json:"risk_rank"`
	DaysRemaining int     `json:"days_remaining"`
	SubtaskTotal  int     `json:"subtask_total"`
	SubtaskDone   int     `json:"subtask_done"`
	SubtaskActive int     `json:"subtask_active"`
	SubtaskReview int     `json:"subtask_review"`
	IssueType     string  `json:"issue_type"`
	ProjectKey      string  `json:"project_key"`
	ProjectPriority string  `json:"project_priority"`
}

type scheduleSubtaskStats struct {
	Total  int
	Done   int
	Active int
	Review int
}

type scheduleRisk struct {
	Level         string
	Label         string
	Reason        string
	Rank          int
	DaysRemaining int
}

// handleGetSchedule returns a compact schedule table projection for demand planning.
func (s *Server) handleGetSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var tasks []db.TaskTelemetry
	if err := db.DB.Find(&tasks).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to query schedule tasks: %v", err), http.StatusInternalServerError)
		return
	}

	var users []userdb.User
	if err := db.DB.Find(&users).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to query users for schedule: %v", err), http.StatusInternalServerError)
		return
	}

	response := buildScheduleResponse(tasks, users, time.Now())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func buildScheduleResponse(tasks []db.TaskTelemetry, users []userdb.User, now time.Time) ScheduleResponseDTO {
	directory := newKPIUserDirectory(users)
	subtasksByGroup := make(map[string]scheduleSubtaskStats)
	demands := make([]db.TaskTelemetry, 0)

	for _, task := range tasks {
		if isArchivedTask(task) {
			continue
		}
		issueType := normalizeIssueType(task.IssueType)
		if issueType == "demand" || issueType == "bug" || issueType == "缺陷" || issueType == "故障" || issueType == "defect" {
			demands = append(demands, task)
			continue
		}
		groupID := normalizedTaskGroupID(task.TaskGroupID)
		if groupID == "" {
			continue
		}
		stats := subtasksByGroup[groupID]
		stats.Total++
		switch strings.ToLower(strings.TrimSpace(task.Status)) {
		case "done":
			stats.Done++
		case "progress":
			stats.Active++
		case "review":
			stats.Active++
			stats.Review++
		}
		subtasksByGroup[groupID] = stats
	}

	var projConfigs []db.ProjectConfig
	if err := db.DB.Find(&projConfigs).Error; err != nil {
		// fallback silently
	}
	projPriorityMap := make(map[string]string)
	for _, pc := range projConfigs {
		projPriorityMap[strings.ToUpper(pc.ProjectKey)] = pc.BasePriority
	}

	items := make([]ScheduleItemDTO, 0, len(demands))
	summary := ScheduleSummaryDTO{Total: len(demands)}
	for _, demand := range demands {
		groupID := normalizedTaskGroupID(demand.TaskGroupID)
		stats := subtasksByGroup[groupID]
		risk := resolveScheduleRisk(demand, stats, now)
		identity := directory.resolve(demand.Assignee)
		department := normalizeDepartment(identity.Department)
		if department == "未分配" && !isMissingDepartment(demand.CreatorDept) {
			department = normalizeDepartment(demand.CreatorDept)
		}

		issueTypeNormalized := "demand"
		rawT := strings.ToLower(strings.TrimSpace(demand.IssueType))
		if rawT == "bug" || rawT == "缺陷" || rawT == "故障" || rawT == "defect" {
			issueTypeNormalized = "bug"
		}

		projKey := ""
		idx := strings.Index(demand.TaskID, "-")
		if idx > 0 {
			projKey = strings.ToUpper(demand.TaskID[:idx])
		}
		projPriority := "P1" // default
		if p, ok := projPriorityMap[projKey]; ok {
			projPriority = p
		}

		item := ScheduleItemDTO{
			DemandID:        strings.TrimSpace(demand.TaskID),
			Title:           strings.TrimSpace(demand.Title),
			Description:     strings.TrimSpace(demand.Description),
			Assignee:        normalizeAssignee(demand.Assignee),
			Department:      department,
			Repo:            strings.TrimSpace(demand.Repo),
			Branch:          strings.TrimSpace(demand.Branch),
			Status:          strings.ToLower(strings.TrimSpace(demand.Status)),
			TaskGroupID:     groupID,
			Scheduled:       isDemandScheduled(demand),
			DueDate:         formatOptionalDate(demand.DueDate),
			CreatedAt:       formatDateTime(demand.TaskCreatedAt),
			LastUpdate:      formatDateTime(scheduleActivityTime(demand)),
			CompletedAt:     formatOptionalDate(demand.CompletedAt),
			MRURL:           strings.TrimSpace(demand.MrURL),
			EstimateDays:    roundOneDecimal(demand.EstimateDays),
			EstimateHours:   roundOneDecimal(demand.EstimateHours),
			Difficulty:      strings.TrimSpace(demand.Difficulty),
			RiskLevel:       risk.Level,
			RiskLabel:       risk.Label,
			RiskReason:      risk.Reason,
			RiskRank:        risk.Rank,
			DaysRemaining:   risk.DaysRemaining,
			SubtaskTotal:    stats.Total,
			SubtaskDone:     stats.Done,
			SubtaskActive:   stats.Active,
			SubtaskReview:   stats.Review,
			IssueType:       issueTypeNormalized,
			ProjectKey:      projKey,
			ProjectPriority: projPriority,
		}
		items = append(items, item)
		accumulateScheduleSummary(&summary, demand, risk)
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].RiskRank != items[j].RiskRank {
			return items[i].RiskRank > items[j].RiskRank
		}
		if items[i].DueDate != items[j].DueDate {
			if items[i].DueDate == "" {
				return false
			}
			if items[j].DueDate == "" {
				return true
			}
			return items[i].DueDate < items[j].DueDate
		}
		return items[i].DemandID < items[j].DemandID
	})

	return ScheduleResponseDTO{
		GeneratedAt: formatDateTime(now),
		Summary:     summary,
		Items:       items,
	}
}

func accumulateScheduleSummary(summary *ScheduleSummaryDTO, demand db.TaskTelemetry, risk scheduleRisk) {
	if isDemandScheduled(demand) {
		summary.Scheduled++
	} else {
		summary.Unscheduled++
	}

	switch strings.ToLower(strings.TrimSpace(demand.Status)) {
	case "progress":
		summary.InProgress++
	case "review":
		summary.Review++
	case "done":
		summary.Done++
	}

	switch risk.Level {
	case "overdue":
		summary.Overdue++
	case "due_soon":
		summary.DueSoon++
	case "stale":
		summary.Stale++
	}
}

func resolveScheduleRisk(demand db.TaskTelemetry, stats scheduleSubtaskStats, now time.Time) scheduleRisk {
	status := strings.ToLower(strings.TrimSpace(demand.Status))
	if status == "done" {
		return scheduleRisk{
			Level:  "done",
			Label:  "已交付",
			Reason: "需求已经完成交付",
			Rank:   5,
		}
	}

	if !hasScheduleBranch(demand.Branch) {
		return scheduleRisk{
			Level:  "unscheduled",
			Label:  "待排期",
			Reason: "尚未绑定开发分支，无法进入交付追踪",
			Rank:   70,
		}
	}

	if demand.DueDate == nil || demand.DueDate.IsZero() {
		return scheduleRisk{
			Level:  "unscheduled",
			Label:  "缺截止日",
			Reason: "已有开发分支，但缺少可追踪的截止日期",
			Rank:   62,
		}
	}

	today := startOfDay(now)
	dueDay := startOfDay(*demand.DueDate)
	daysRemaining := int(dueDay.Sub(today).Hours() / 24)

	if daysRemaining < 0 {
		return scheduleRisk{
			Level:         "overdue",
			Label:         "已逾期",
			Reason:        fmt.Sprintf("超过计划截止日 %d 天", -daysRemaining),
			Rank:          100,
			DaysRemaining: daysRemaining,
		}
	}

	if daysRemaining <= 3 {
		return scheduleRisk{
			Level:         "due_soon",
			Label:         "临期",
			Reason:        fmt.Sprintf("%d 天后到期，需要关注交付确定性", daysRemaining),
			Rank:          82,
			DaysRemaining: daysRemaining,
		}
	}

	if isScheduleStale(demand, stats, now) {
		return scheduleRisk{
			Level:         "stale",
			Label:         "推进滞后",
			Reason:        "排期后长时间未出现任务进展或代码活动",
			Rank:          50,
			DaysRemaining: daysRemaining,
		}
	}

	return scheduleRisk{
		Level:         "safe",
		Label:         "正常",
		Reason:        "排期、截止日与任务推进状态完整",
		Rank:          10,
		DaysRemaining: daysRemaining,
	}
}

func isScheduleStale(demand db.TaskTelemetry, stats scheduleSubtaskStats, now time.Time) bool {
	if stats.Total > 0 && stats.Done == stats.Total {
		return false
	}
	activity := scheduleActivityTime(demand)
	if activity.IsZero() {
		return false
	}
	return now.Sub(activity) > 72*time.Hour
}

func scheduleActivityTime(task db.TaskTelemetry) time.Time {
	if !task.LastUpdate.IsZero() {
		return task.LastUpdate
	}
	if !task.TaskCreatedAt.IsZero() {
		return task.TaskCreatedAt
	}
	return time.Time{}
}

func isDemandScheduled(demand db.TaskTelemetry) bool {
	return hasScheduleBranch(demand.Branch) && demand.DueDate != nil && !demand.DueDate.IsZero()
}

func hasScheduleBranch(branch string) bool {
	branch = strings.TrimSpace(branch)
	return branch != "" && branch != "-"
}

func normalizedTaskGroupID(groupID string) string {
	groupID = strings.TrimSpace(groupID)
	if groupID == "-" {
		return ""
	}
	return groupID
}

func isArchivedTask(task db.TaskTelemetry) bool {
	return strings.EqualFold(strings.TrimSpace(task.Status), "archived")
}

func startOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}
