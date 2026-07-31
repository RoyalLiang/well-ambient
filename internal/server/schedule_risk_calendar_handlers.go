package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

type ScheduleRiskCalendarResponseDTO struct {
	GeneratedAt string                          `json:"generated_at"`
	WindowStart string                          `json:"window_start"`
	WindowEnd   string                          `json:"window_end"`
	Summary     ScheduleRiskCalendarSummaryDTO  `json:"summary"`
	Events      []ScheduleRiskCalendarEventDTO  `json:"events"`
	Weeks       []ScheduleRiskCalendarBucketDTO `json:"weeks"`
	Months      []ScheduleRiskCalendarBucketDTO `json:"months"`
}

type ScheduleRiskCalendarSummaryDTO struct {
	Total           int `json:"total"`
	Overdue         int `json:"overdue"`
	DueSoon         int `json:"due_soon"`
	Stale           int `json:"stale"`
	MissingSchedule int `json:"missing_schedule"`
	HighRisk        int `json:"high_risk"`
	ThisWeek        int `json:"this_week"`
	NextWeek        int `json:"next_week"`
	Later           int `json:"later"`
}

type ScheduleRiskCalendarEventDTO struct {
	ID              string   `json:"id"`
	DemandID        string   `json:"demand_id"`
	Title           string   `json:"title"`
	Assignee        string   `json:"assignee"`
	Department      string   `json:"department"`
	ProjectKey      string   `json:"project_key"`
	ProjectPriority string   `json:"project_priority"`
	IssueType       string   `json:"issue_type"`
	RiskType        string   `json:"risk_type"`
	RiskLevel       string   `json:"risk_level"`
	RiskLabel       string   `json:"risk_label"`
	RiskReason      string   `json:"risk_reason"`
	RiskRank        int      `json:"risk_rank"`
	DueDate         string   `json:"due_date"`
	DaysRemaining   int      `json:"days_remaining"`
	WeekKey         string   `json:"week_key"`
	WeekLabel       string   `json:"week_label"`
	MonthKey        string   `json:"month_key"`
	MonthLabel      string   `json:"month_label"`
	RescheduleCount int      `json:"reschedule_count"`
	SuggestedAction string   `json:"suggested_action"`
	Evidence        []string `json:"evidence"`
}

type ScheduleRiskCalendarBucketDTO struct {
	Key             string   `json:"key"`
	Label           string   `json:"label"`
	StartDate       string   `json:"start_date"`
	EndDate         string   `json:"end_date"`
	Total           int      `json:"total"`
	Overdue         int      `json:"overdue"`
	DueSoon         int      `json:"due_soon"`
	Stale           int      `json:"stale"`
	MissingSchedule int      `json:"missing_schedule"`
	HighRisk        int      `json:"high_risk"`
	EventIDs        []string `json:"event_ids"`
}

func (s *Server) handleGetScheduleRiskCalendar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	projectKeys, err := requestProjectPreferenceKeys(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to apply project preferences: %v", err), http.StatusInternalServerError)
		return
	}
	schedule, err := buildStrongestBrainScheduleSnapshot(time.Now(), projectKeys)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to build schedule risk calendar: %v", err), http.StatusInternalServerError)
		return
	}
	response := buildScheduleRiskCalendarResponse(schedule, time.Now())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func buildScheduleRiskCalendarResponse(schedule ScheduleResponseDTO, now time.Time) ScheduleRiskCalendarResponseDTO {
	windowStart := startOfDay(now).AddDate(0, 0, -7)
	windowEnd := startOfDay(now).AddDate(0, 0, 45)
	events := make([]ScheduleRiskCalendarEventDTO, 0)
	for _, item := range schedule.Items {
		if item.RiskLevel == "safe" || item.RiskLevel == "done" || item.RiskLevel == "" {
			continue
		}
		if item.RiskLevel != "overdue" && item.RiskLevel != "due_soon" && item.RiskLevel != "stale" && item.RiskLevel != "unscheduled" {
			continue
		}
		eventDate := scheduleRiskEventDate(item, now)
		weekStart := weekStartDate(eventDate)
		weekEnd := weekStart.AddDate(0, 0, 6)
		monthStart := time.Date(eventDate.Year(), eventDate.Month(), 1, 0, 0, 0, 0, eventDate.Location())
		riskType := scheduleRiskType(item)
		event := ScheduleRiskCalendarEventDTO{
			ID:              fmt.Sprintf("%s:%s", riskType, item.DemandID),
			DemandID:        item.DemandID,
			Title:           item.Title,
			Assignee:        item.Assignee,
			Department:      item.Department,
			ProjectKey:      item.ProjectKey,
			ProjectPriority: item.ProjectPriority,
			IssueType:       item.IssueType,
			RiskType:        riskType,
			RiskLevel:       scheduleCalendarRiskLevel(item.RiskLevel),
			RiskLabel:       item.RiskLabel,
			RiskReason:      item.RiskReason,
			RiskRank:        item.RiskRank,
			DueDate:         item.DueDate,
			DaysRemaining:   item.DaysRemaining,
			WeekKey:         scheduleCalendarDate(weekStart),
			WeekLabel:       scheduleBucketLabel(weekStart, weekEnd),
			MonthKey:        monthStart.Format("2006-01"),
			MonthLabel:      monthStart.Format("2006-01"),
			RescheduleCount: scheduleRescheduleCount(item),
			SuggestedAction: scheduleCalendarSuggestedAction(item),
			Evidence:        scheduleCalendarEvidence(item),
		}
		events = append(events, event)
	}
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].RiskRank != events[j].RiskRank {
			return events[i].RiskRank > events[j].RiskRank
		}
		if events[i].DueDate != events[j].DueDate {
			if events[i].DueDate == "" {
				return false
			}
			if events[j].DueDate == "" {
				return true
			}
			return events[i].DueDate < events[j].DueDate
		}
		return events[i].DemandID < events[j].DemandID
	})

	response := ScheduleRiskCalendarResponseDTO{
		GeneratedAt: schedule.GeneratedAt,
		WindowStart: scheduleCalendarDate(windowStart),
		WindowEnd:   scheduleCalendarDate(windowEnd),
		Events:      events,
		Weeks:       buildScheduleRiskBuckets(events, "week"),
		Months:      buildScheduleRiskBuckets(events, "month"),
	}
	for _, event := range events {
		accumulateScheduleRiskCalendarSummary(&response.Summary, event, now)
	}
	response.Summary.Total = len(events)
	return response
}

func buildScheduleRiskBuckets(events []ScheduleRiskCalendarEventDTO, mode string) []ScheduleRiskCalendarBucketDTO {
	byKey := map[string]*ScheduleRiskCalendarBucketDTO{}
	for _, event := range events {
		key := event.WeekKey
		label := event.WeekLabel
		if mode == "month" {
			key = event.MonthKey
			label = event.MonthLabel
		}
		if key == "" {
			key = "unscheduled"
			label = "未排期"
		}
		bucket := byKey[key]
		if bucket == nil {
			bucket = &ScheduleRiskCalendarBucketDTO{Key: key, Label: label}
			if mode == "week" && key != "unscheduled" {
				if start, err := time.Parse("2006-01-02", key); err == nil {
					bucket.StartDate = scheduleCalendarDate(start)
					bucket.EndDate = scheduleCalendarDate(start.AddDate(0, 0, 6))
				}
			}
			if mode == "month" && key != "unscheduled" {
				if start, err := time.Parse("2006-01", key); err == nil {
					bucket.StartDate = scheduleCalendarDate(start)
					bucket.EndDate = scheduleCalendarDate(start.AddDate(0, 1, -1))
				}
			}
			byKey[key] = bucket
		}
		accumulateScheduleRiskBucket(bucket, event)
	}
	buckets := make([]ScheduleRiskCalendarBucketDTO, 0, len(byKey))
	for _, bucket := range byKey {
		buckets = append(buckets, *bucket)
	}
	sort.SliceStable(buckets, func(i, j int) bool {
		if buckets[i].Key == "unscheduled" {
			return false
		}
		if buckets[j].Key == "unscheduled" {
			return true
		}
		return buckets[i].Key < buckets[j].Key
	})
	if mode == "week" && len(buckets) > 8 {
		return buckets[:8]
	}
	if mode == "month" && len(buckets) > 4 {
		return buckets[:4]
	}
	return buckets
}

func accumulateScheduleRiskCalendarSummary(summary *ScheduleRiskCalendarSummaryDTO, event ScheduleRiskCalendarEventDTO, now time.Time) {
	switch event.RiskType {
	case "overdue":
		summary.Overdue++
	case "due_soon":
		summary.DueSoon++
	case "stale_after_schedule":
		summary.Stale++
	case "missing_schedule":
		summary.MissingSchedule++
	}
	if event.RiskLevel == "critical" {
		summary.HighRisk++
	}
	eventDay := scheduleEventDateFromDTO(event, now)
	thisWeekStart := weekStartDate(now)
	nextWeekStart := thisWeekStart.AddDate(0, 0, 7)
	laterStart := nextWeekStart.AddDate(0, 0, 7)
	if !eventDay.Before(thisWeekStart) && eventDay.Before(nextWeekStart) {
		summary.ThisWeek++
	} else if !eventDay.Before(nextWeekStart) && eventDay.Before(laterStart) {
		summary.NextWeek++
	} else {
		summary.Later++
	}
}

func accumulateScheduleRiskBucket(bucket *ScheduleRiskCalendarBucketDTO, event ScheduleRiskCalendarEventDTO) {
	bucket.Total++
	bucket.EventIDs = append(bucket.EventIDs, event.ID)
	switch event.RiskType {
	case "overdue":
		bucket.Overdue++
	case "due_soon":
		bucket.DueSoon++
	case "stale_after_schedule":
		bucket.Stale++
	case "missing_schedule":
		bucket.MissingSchedule++
	}
	if event.RiskLevel == "critical" {
		bucket.HighRisk++
	}
}

func scheduleRiskType(item ScheduleItemDTO) string {
	switch item.RiskLevel {
	case "overdue":
		return "overdue"
	case "due_soon":
		return "due_soon"
	case "stale":
		return "stale_after_schedule"
	case "unscheduled":
		return "missing_schedule"
	default:
		return firstNonEmpty(item.RiskLevel, "schedule_risk")
	}
}

func scheduleCalendarRiskLevel(level string) string {
	switch level {
	case "overdue":
		return "critical"
	case "due_soon", "stale", "unscheduled":
		return "warning"
	default:
		return level
	}
}

func scheduleRiskEventDate(item ScheduleItemDTO, now time.Time) time.Time {
	if item.DueDate != "" {
		if due, err := time.Parse("2006-01-02", item.DueDate); err == nil {
			return due
		}
	}
	if item.LastUpdate != "" {
		if last, err := time.ParseInLocation("2006-01-02 15:04", item.LastUpdate, time.Local); err == nil {
			return last
		}
	}
	return now
}

func scheduleEventDateFromDTO(event ScheduleRiskCalendarEventDTO, now time.Time) time.Time {
	if event.DueDate != "" {
		if due, err := time.Parse("2006-01-02", event.DueDate); err == nil {
			return due
		}
	}
	if event.WeekKey != "" {
		if week, err := time.Parse("2006-01-02", event.WeekKey); err == nil {
			return week
		}
	}
	return now
}

func weekStartDate(t time.Time) time.Time {
	day := startOfDay(t)
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}

func scheduleCalendarDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func scheduleBucketLabel(start time.Time, end time.Time) string {
	return fmt.Sprintf("%s - %s", start.Format("01-02"), end.Format("01-02"))
}

func scheduleCalendarEvidence(item ScheduleItemDTO) []string {
	evidence := []string{fmt.Sprintf("%s：%s", item.RiskLabel, item.RiskReason)}
	if item.DueDate != "" {
		evidence = append(evidence, "截止日 "+item.DueDate)
	}
	if item.Branch != "" && item.Branch != "-" {
		evidence = append(evidence, "分支 "+item.Branch)
	}
	if item.SubtaskTotal > 0 {
		evidence = append(evidence, fmt.Sprintf("影子任务 %d/%d 完成", item.SubtaskDone, item.SubtaskTotal))
	}
	return compactStrings(evidence, 4)
}

func scheduleCalendarSuggestedAction(item ScheduleItemDTO) string {
	switch item.RiskLevel {
	case "overdue":
		return "确认是否延期、拆分范围或转派协助，并记录影响范围"
	case "due_soon":
		return "确认剩余工作、验收窗口和合并计划"
	case "stale":
		return "要求负责人补充下一次提交、评审或阻塞原因"
	case "unscheduled":
		return "补齐负责人、开发分支和截止日后再纳入交付节奏"
	default:
		return "保持监听并在风险升级时进入决策队列"
	}
}

func scheduleRescheduleCount(item ScheduleItemDTO) int {
	text := strings.ToLower(item.RiskReason + " " + item.Description)
	count := strings.Count(text, "延期") + strings.Count(text, "reschedule")
	if count > 0 {
		return count
	}
	return 0
}
