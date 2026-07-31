package server

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

type UserKPIDTO struct {
	Username             string   `json:"username"`
	Name                 string   `json:"name"`
	Avatar               string   `json:"avatar"`
	Department           string   `json:"department"`
	TasksCompleted       int      `json:"tasks_completed"`
	BugsCompleted        int      `json:"bugs_completed"`
	DemandsCompleted     int      `json:"demands_completed"`
	TotalCompleted       int      `json:"total_completed"`
	OverdueCompleted     int      `json:"overdue_completed"`
	ActiveOverdue        int      `json:"active_overdue"`
	AvgCycleDays         float64  `json:"avg_cycle_days"`
	ReviewCount          int      `json:"review_count"`
	MRCount              int      `json:"mr_count"`
	RiskNotes            []string `json:"risk_notes"`
	TaskCount            int      `json:"task_count"`
	BugCount             int      `json:"bug_count"`
	DelayRatio           float64  `json:"delay_ratio"`
	RequirementBaseScore float64  `json:"requirement_base_score"`
	ScoredItemCount      int      `json:"scored_item_count"`
	ManualScoreCount     int      `json:"manual_score_count"`
}

type DeptKPIDTO struct {
	Department       string `json:"department"`
	TasksCompleted   int    `json:"tasks_completed"`
	BugsCompleted    int    `json:"bugs_completed"`
	DemandsCompleted int    `json:"demands_completed"`
	TotalCompleted   int    `json:"total_completed"`
}

type KPISummaryDTO struct {
	TotalCompleted   int `json:"total_completed"`
	TasksCompleted   int `json:"tasks_completed"`
	BugsCompleted    int `json:"bugs_completed"`
	DemandsCompleted int `json:"demands_completed"`
}

type KPIPerformanceResponse struct {
	Period        string        `json:"period"`
	Summary       KPISummaryDTO `json:"summary"`
	UserKPI       []UserKPIDTO  `json:"user_kpi"`
	DepartmentKPI []DeptKPIDTO  `json:"department_kpi"`
}

type KPIReportPreviewResponse struct {
	Period             string                          `json:"period"`
	Type               string                          `json:"type"`
	User               string                          `json:"user,omitempty"`
	GeneratedAt        string                          `json:"generated_at"`
	Overview           KPIReportOverviewDTO            `json:"overview"`
	PersonalSections   []KPIReportPersonalSectionDTO   `json:"personal_sections"`
	DepartmentSections []KPIReportDepartmentSectionDTO `json:"department_sections"`
	Risks              []KPIReportRiskDTO              `json:"risks"`
	MeetingFocus       []KPIReportMeetingFocusDTO      `json:"meeting_focus"`
	Evidence           []KPIReportEvidenceDTO          `json:"evidence"`
}

type KPIReportOverviewDTO struct {
	Title            string `json:"title"`
	PeriodLabel      string `json:"period_label"`
	Summary          string `json:"summary"`
	TotalCompleted   int    `json:"total_completed"`
	ActiveOverdue    int    `json:"active_overdue"`
	OverdueCompleted int    `json:"overdue_completed"`
	MRCount          int    `json:"mr_count"`
	PersonalCount    int    `json:"personal_count"`
	DepartmentCount  int    `json:"department_count"`
}

type KPIReportPersonalSectionDTO struct {
	User        UserKPIDTO `json:"user"`
	Highlights  []string   `json:"highlights"`
	RiskNotes   []string   `json:"risk_notes"`
	EvidenceIDs []string   `json:"evidence_ids"`
}

type KPIReportDepartmentSectionDTO struct {
	Department       string   `json:"department"`
	TasksCompleted   int      `json:"tasks_completed"`
	BugsCompleted    int      `json:"bugs_completed"`
	DemandsCompleted int      `json:"demands_completed"`
	TotalCompleted   int      `json:"total_completed"`
	OverdueCompleted int      `json:"overdue_completed"`
	ActiveOverdue    int      `json:"active_overdue"`
	ReviewCount      int      `json:"review_count"`
	MRCount          int      `json:"mr_count"`
	Highlights       []string `json:"highlights"`
	EvidenceIDs      []string `json:"evidence_ids"`
}

type KPIReportRiskDTO struct {
	Level       string   `json:"level"`
	Owner       string   `json:"owner"`
	Department  string   `json:"department"`
	Title       string   `json:"title"`
	Detail      string   `json:"detail"`
	EvidenceIDs []string `json:"evidence_ids"`
}

type KPIReportMeetingFocusDTO struct {
	Topic       string   `json:"topic"`
	Owner       string   `json:"owner"`
	Department  string   `json:"department"`
	Reason      string   `json:"reason"`
	EvidenceIDs []string `json:"evidence_ids"`
}

type KPIReportEvidenceDTO struct {
	ID          string `json:"id"`
	TaskID      string `json:"task_id"`
	Title       string `json:"title"`
	Repo        string `json:"repo"`
	Assignee    string `json:"assignee"`
	Department  string `json:"department"`
	Status      string `json:"status"`
	IssueType   string `json:"issue_type"`
	DueDate     string `json:"due_date,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	LastUpdate  string `json:"last_update"`
	MRURL       string `json:"mr_url,omitempty"`
	TaskGroupID string `json:"task_group_id,omitempty"`
}

type kpiPeriodWindow struct {
	Period string
	Since  time.Time
	Label  string
}

type kpiAssigneeIdentity struct {
	Name       string
	Username   string
	Avatar     string
	Department string
	IsLinked   bool
}

type kpiUserDirectory struct {
	byKey map[string]userdb.User
}

type kpiAssigneeStats struct {
	name             string
	username         string
	avatar           string
	dept             string
	tasks            int
	bugs             int
	demands          int
	total            int
	overdueCompleted int
	activeOverdue    int
	reviewCount      int
	mrCount          int
	cycleDaysTotal   float64
	cycleCount       int
	isLinked         bool
	taskCount        int
	bugCount         int
	baseScoreTotal   float64
	scoredItemCount  int
	manualScoreCount int
	delayedWorkItems int
}

type kpiEvidenceBundle struct {
	items     []KPIReportEvidenceDTO
	byUserKey map[string][]string
	byDept    map[string][]string
	active    []db.TaskTelemetry
	completed []db.TaskTelemetry
}

type kpiCoreMemberFilter struct {
	enabled bool
	keys    map[string]struct{}
}

// handleGetKPIPerformance gathers finished work metrics grouped by member and department.
func (s *Server) handleGetKPIPerformance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	now := time.Now()
	window := resolveKPIPeriod(r.URL.Query().Get("period"), now)
	completedTasks, activeTasks, users, err := loadKPIInputs(window.Since)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	completedTasks, activeTasks, users = s.filterKPIInputsByCoreMembers(completedTasks, activeTasks, users)

	response := buildKPIPerformanceResponse(window.Period, completedTasks, activeTasks, users, now)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetKPIReportPreview returns a deterministic, evidence-backed report draft.
func (s *Server) handleGetKPIReportPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	reportType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if reportType == "" {
		reportType = "weekly"
	}
	if reportType != "daily" && reportType != "weekly" {
		http.Error(w, "Bad Request: type must be daily or weekly", http.StatusBadRequest)
		return
	}

	now := time.Now()
	period := r.URL.Query().Get("period")
	if period == "" {
		if reportType == "daily" {
			period = "day"
		} else {
			period = "week"
		}
	}
	window := resolveKPIPeriod(period, now)
	userFilter := strings.TrimSpace(r.URL.Query().Get("user"))
	completedTasks, activeTasks, users, err := loadKPIInputs(window.Since)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	completedTasks, activeTasks, users = s.filterKPIInputsByCoreMembers(completedTasks, activeTasks, users)

	performance := buildKPIPerformanceResponse(window.Period, completedTasks, activeTasks, users, now)
	response := buildKPIReportPreview(window, reportType, userFilter, performance, completedTasks, activeTasks, users, now)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func resolveKPIPeriod(period string, now time.Time) kpiPeriodWindow {
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "day", "daily":
		return kpiPeriodWindow{Period: "day", Since: now.AddDate(0, 0, -1), Label: "近 24 小时"}
	case "month":
		return kpiPeriodWindow{Period: "month", Since: now.AddDate(0, 0, -30), Label: "近 30 天"}
	case "year":
		return kpiPeriodWindow{Period: "year", Since: now.AddDate(0, 0, -365), Label: "近一年"}
	case "week":
		fallthrough
	default:
		return kpiPeriodWindow{Period: "week", Since: now.AddDate(0, 0, -7), Label: "近 7 天"}
	}
}

func loadKPIInputs(since time.Time) ([]db.TaskTelemetry, []db.TaskTelemetry, []userdb.User, error) {
	var completedTasks []db.TaskTelemetry
	err := db.DB.Where("status = ? AND ((completed_at IS NOT NULL AND completed_at >= ?) OR (completed_at IS NULL AND last_update >= ?))",
		"done", since, since).Find(&completedTasks).Error
	if err != nil {
		return nil, nil, nil, err
	}

	var activeTasks []db.TaskTelemetry
	if err := db.DB.Where("status IN ?", []string{"backlog", "progress", "review"}).Find(&activeTasks).Error; err != nil {
		return nil, nil, nil, err
	}

	var users []userdb.User
	if err := db.DB.Find(&users).Error; err != nil {
		return nil, nil, nil, err
	}

	return completedTasks, activeTasks, users, nil
}

func (s *Server) filterKPIInputsByCoreMembers(completedTasks, activeTasks []db.TaskTelemetry, users []userdb.User) ([]db.TaskTelemetry, []db.TaskTelemetry, []userdb.User) {
	filter := s.buildKPICoreMemberFilter(users)
	if !filter.enabled {
		return completedTasks, activeTasks, users
	}

	directory := newKPIUserDirectory(users)
	return filterKPITasksByCoreMembers(completedTasks, directory, filter),
		filterKPITasksByCoreMembers(activeTasks, directory, filter),
		filterKPIUsersByCoreMembers(users, filter)
}

func (s *Server) buildKPICoreMemberFilter(users []userdb.User) kpiCoreMemberFilter {
	members := s.configuredKPICoreMembers()
	if len(members) == 0 {
		return kpiCoreMemberFilter{}
	}

	directory := newKPIUserDirectory(users)
	filter := kpiCoreMemberFilter{
		enabled: true,
		keys:    make(map[string]struct{}, len(members)*3),
	}
	for _, member := range members {
		addKPICoreMemberKeys(filter.keys, member)
		identity := directory.resolve(member)
		addKPICoreMemberKeys(filter.keys, identity.Name)
		addKPICoreMemberKeys(filter.keys, identity.Username)
	}
	return filter
}

func (s *Server) configuredKPICoreMembers() []string {
	if s == nil || s.config == nil {
		return nil
	}

	members := normalizeConfiguredKPICoreMembers(s.config.Jira.SyncUsers)
	if len(members) > 0 {
		return members
	}
	return normalizeConfiguredKPICoreMembers(extractJIRAAssignees(s.config.Jira.CustomJQL))
}

func normalizeConfiguredKPICoreMembers(values []string) []string {
	members := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "-" || value == "未指派" || strings.EqualFold(value, "unassigned") {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		members = append(members, value)
	}
	return members
}

func filterKPITasksByCoreMembers(tasks []db.TaskTelemetry, directory kpiUserDirectory, filter kpiCoreMemberFilter) []db.TaskTelemetry {
	filtered := make([]db.TaskTelemetry, 0, len(tasks))
	for _, task := range tasks {
		if filter.includesAssignee(task.Assignee, directory) {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func filterKPIUsersByCoreMembers(users []userdb.User, filter kpiCoreMemberFilter) []userdb.User {
	filtered := make([]userdb.User, 0, len(users))
	for _, user := range users {
		if filter.includesIdentity(kpiIdentityFromUser(user)) {
			filtered = append(filtered, user)
		}
	}
	return filtered
}

func (f kpiCoreMemberFilter) includesAssignee(assignee string, directory kpiUserDirectory) bool {
	if !f.enabled {
		return true
	}
	if hasKPICoreMemberKey(f.keys, assignee) {
		return true
	}
	return f.includesIdentity(directory.resolve(assignee))
}

func (f kpiCoreMemberFilter) includesIdentity(identity kpiAssigneeIdentity) bool {
	if !f.enabled {
		return true
	}
	return hasKPICoreMemberKey(f.keys, identity.Name) ||
		hasKPICoreMemberKey(f.keys, identity.Username)
}

func addKPICoreMemberKeys(keys map[string]struct{}, value string) {
	for _, key := range kpiCoreMemberKeyVariants(value) {
		keys[key] = struct{}{}
	}
}

func hasKPICoreMemberKey(keys map[string]struct{}, value string) bool {
	for _, key := range kpiCoreMemberKeyVariants(value) {
		if _, exists := keys[key]; exists {
			return true
		}
	}
	return false
}

func kpiCoreMemberKeyVariants(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" || value == "未指派" || strings.EqualFold(value, "unassigned") {
		return nil
	}

	variants := []string{value, normalizeAssignee(value)}
	if idx := strings.Index(value, "@"); idx != -1 {
		variants = append(variants, value[:idx])
	}
	if fields := strings.Fields(value); len(fields) > 0 {
		variants = append(variants, fields[0])
	}

	keys := make([]string, 0, len(variants))
	seen := make(map[string]struct{}, len(variants))
	for _, variant := range variants {
		key := strings.ToLower(strings.TrimSpace(variant))
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys
}

func buildKPIPerformanceResponse(period string, completedTasks, activeTasks []db.TaskTelemetry, users []userdb.User, now time.Time) KPIPerformanceResponse {
	directory := newKPIUserDirectory(users)
	statsMap := make(map[string]*kpiAssigneeStats)

	for _, u := range users {
		identity := kpiIdentityFromUser(u)
		statsMap[kpiStatKey(identity.Name)] = &kpiAssigneeStats{
			name:     identity.Name,
			username: identity.Username,
			avatar:   identity.Avatar,
			dept:     identity.Department,
			isLinked: true,
		}
	}

	var totalTasks, totalBugs, totalDemands, totalAll int
	for _, task := range completedTasks {
		stat := ensureKPIStats(statsMap, directory.resolve(task.Assignee))
		accumulateKPIWorkProfile(stat, task, now)

		switch strings.ToLower(strings.TrimSpace(task.IssueType)) {
		case "bug":
			stat.bugs++
			totalBugs++
		case "demand":
			stat.demands++
			totalDemands++
		default:
			stat.tasks++
			totalTasks++
		}
		stat.total++
		totalAll++

		if isCompletedAfterDue(task) {
			stat.overdueCompleted++
		}
		if hasMergeRequest(task) {
			stat.mrCount++
		}
		if days, ok := taskCycleDays(task); ok {
			stat.cycleDaysTotal += days
			stat.cycleCount++
		}
	}

	for _, task := range activeTasks {
		stat := ensureKPIStats(statsMap, directory.resolve(task.Assignee))
		accumulateKPIWorkProfile(stat, task, now)
		if isActiveOverdue(task, now) {
			stat.activeOverdue++
		}
		if strings.EqualFold(strings.TrimSpace(task.Status), "review") {
			stat.reviewCount++
		}
		if hasMergeRequest(task) {
			stat.mrCount++
		}
	}

	var userKPIList []UserKPIDTO
	deptMap := make(map[string]*DeptKPIDTO)
	for _, stat := range statsMap {
		dept := normalizeDepartment(stat.dept)
		avgCycleDays := 0.0
		if stat.cycleCount > 0 {
			avgCycleDays = roundOneDecimal(stat.cycleDaysTotal / float64(stat.cycleCount))
		}

		if stat.total > 0 || stat.activeOverdue > 0 || stat.reviewCount > 0 || stat.mrCount > 0 {
			delayRatio := 0.0
			workItemCount := stat.taskCount + stat.bugCount
			if workItemCount > 0 {
				delayRatio = roundOneDecimal(float64(stat.delayedWorkItems) * 100 / float64(workItemCount))
			}
			baseScore := 60.0
			if stat.scoredItemCount > 0 {
				baseScore = roundOneDecimal(stat.baseScoreTotal / float64(stat.scoredItemCount))
			}
			userKPIList = append(userKPIList, UserKPIDTO{
				Username:             stat.username,
				Name:                 stat.name,
				Avatar:               stat.avatar,
				Department:           dept,
				TasksCompleted:       stat.tasks,
				BugsCompleted:        stat.bugs,
				DemandsCompleted:     stat.demands,
				TotalCompleted:       stat.total,
				OverdueCompleted:     stat.overdueCompleted,
				ActiveOverdue:        stat.activeOverdue,
				AvgCycleDays:         avgCycleDays,
				ReviewCount:          stat.reviewCount,
				MRCount:              stat.mrCount,
				RiskNotes:            buildUserRiskNotes(stat, avgCycleDays),
				TaskCount:            stat.taskCount,
				BugCount:             stat.bugCount,
				DelayRatio:           delayRatio,
				RequirementBaseScore: baseScore,
				ScoredItemCount:      stat.scoredItemCount,
				ManualScoreCount:     stat.manualScoreCount,
			})
		}

		if stat.total > 0 {
			dStat, exists := deptMap[dept]
			if !exists {
				dStat = &DeptKPIDTO{Department: dept}
				deptMap[dept] = dStat
			}
			dStat.TasksCompleted += stat.tasks
			dStat.BugsCompleted += stat.bugs
			dStat.DemandsCompleted += stat.demands
			dStat.TotalCompleted += stat.total
		}
	}

	sort.Slice(userKPIList, func(i, j int) bool {
		if userKPIList[i].TotalCompleted != userKPIList[j].TotalCompleted {
			return userKPIList[i].TotalCompleted > userKPIList[j].TotalCompleted
		}
		return userKPIList[i].Name < userKPIList[j].Name
	})

	var deptKPIList []DeptKPIDTO
	for _, d := range deptMap {
		deptKPIList = append(deptKPIList, *d)
	}
	sort.Slice(deptKPIList, func(i, j int) bool {
		if deptKPIList[i].TotalCompleted != deptKPIList[j].TotalCompleted {
			return deptKPIList[i].TotalCompleted > deptKPIList[j].TotalCompleted
		}
		return deptKPIList[i].Department < deptKPIList[j].Department
	})

	return KPIPerformanceResponse{
		Period: period,
		Summary: KPISummaryDTO{
			TotalCompleted:   totalAll,
			TasksCompleted:   totalTasks,
			BugsCompleted:    totalBugs,
			DemandsCompleted: totalDemands,
		},
		UserKPI:       userKPIList,
		DepartmentKPI: deptKPIList,
	}
}

func accumulateKPIWorkProfile(stat *kpiAssigneeStats, task db.TaskTelemetry, now time.Time) {
	if strings.EqualFold(strings.TrimSpace(task.IssueType), "demand") {
		return
	}
	if strings.EqualFold(strings.TrimSpace(task.IssueType), "bug") {
		stat.bugCount++
	} else {
		stat.taskCount++
	}
	if isCompletedAfterDue(task) || isActiveOverdue(task, now) {
		stat.delayedWorkItems++
	}
	if score, ok := requirementBaseScore(task); ok {
		stat.baseScoreTotal += score
		stat.scoredItemCount++
		if strings.EqualFold(strings.TrimSpace(task.EstimateSource), "manual_adjusted") {
			stat.manualScoreCount++
		}
	}
}

func requirementBaseScore(task db.TaskTelemetry) (float64, bool) {
	difficulty := strings.ToLower(strings.TrimSpace(task.Difficulty))
	if difficulty == "" && task.EstimateDays <= 0 && task.EstimateHours <= 0 {
		return 0, false
	}
	score := 60.0
	switch difficulty {
	case "high":
		score = 90
	case "medium":
		score = 75
	case "low":
		score = 60
	default:
		days := task.EstimateDays
		if days <= 0 && task.EstimateHours > 0 {
			days = task.EstimateHours / 8
		}
		if days >= 5 {
			score = 88
		} else if days >= 3 {
			score = 76
		} else if days >= 1 {
			score = 64
		}
	}
	return score, true
}

func buildKPIReportPreview(window kpiPeriodWindow, reportType, userFilter string, performance KPIPerformanceResponse, completedTasks, activeTasks []db.TaskTelemetry, users []userdb.User, now time.Time) KPIReportPreviewResponse {
	directory := newKPIUserDirectory(users)
	includedUsers := make([]UserKPIDTO, 0, len(performance.UserKPI))
	includedUserKeys := make(map[string]bool)
	includedDepartments := make(map[string]bool)

	for _, userKPI := range performance.UserKPI {
		if !includeUserInReport(userKPI, userFilter) {
			continue
		}
		includedUsers = append(includedUsers, userKPI)
		includedUserKeys[kpiStatKey(userKPI.Name)] = true
		includedDepartments[normalizeDepartment(userKPI.Department)] = true
	}

	evidence := buildKPIEvidenceBundle(completedTasks, activeTasks, directory, includedUserKeys, userFilter, now)
	personalSections := make([]KPIReportPersonalSectionDTO, 0, len(includedUsers))
	for _, userKPI := range includedUsers {
		key := kpiStatKey(userKPI.Name)
		personalSections = append(personalSections, KPIReportPersonalSectionDTO{
			User:        userKPI,
			Highlights:  buildPersonalHighlights(userKPI),
			RiskNotes:   userKPI.RiskNotes,
			EvidenceIDs: evidence.byUserKey[key],
		})
	}

	departmentSections := buildDepartmentReportSections(includedUsers, includedDepartments, userFilter, evidence.byDept)
	risks := buildKPIReportRisks(evidence, directory, now)
	meetingFocus := buildKPIReportMeetingFocus(evidence, directory, now)
	overview := buildKPIReportOverview(window, reportType, userFilter, personalSections, departmentSections)

	return KPIReportPreviewResponse{
		Period:             window.Period,
		Type:               reportType,
		User:               userFilter,
		GeneratedAt:        now.Format(time.RFC3339),
		Overview:           overview,
		PersonalSections:   personalSections,
		DepartmentSections: departmentSections,
		Risks:              risks,
		MeetingFocus:       meetingFocus,
		Evidence:           evidence.items,
	}
}

func newKPIUserDirectory(users []userdb.User) kpiUserDirectory {
	directory := kpiUserDirectory{byKey: make(map[string]userdb.User)}
	for _, u := range users {
		addUserDirectoryKey(directory.byKey, u.Name, u)
		addUserDirectoryKey(directory.byKey, u.Username, u)
		if idx := strings.Index(u.Username, "@"); idx != -1 {
			addUserDirectoryKey(directory.byKey, u.Username[:idx], u)
		}
	}
	return directory
}

func addUserDirectoryKey(userMap map[string]userdb.User, key string, user userdb.User) {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return
	}
	userMap[key] = user
	if alias := kpiUserDirectoryAliasKey(key); alias != "" && alias != key {
		if _, exists := userMap[alias]; !exists {
			userMap[alias] = user
		}
	}
}

func kpiUserDirectoryAliasKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if idx := strings.Index(value, "@"); idx != -1 {
		value = value[:idx]
	}
	replacer := strings.NewReplacer(".", "", "_", "", "-", "", " ", "")
	return replacer.Replace(value)
}

func (d kpiUserDirectory) resolve(assignee string) kpiAssigneeIdentity {
	assignee = normalizeAssignee(assignee)
	if u, ok := d.byKey[strings.ToLower(assignee)]; ok {
		return kpiIdentityFromUser(u)
	}
	if u, ok := d.byKey[kpiUserDirectoryAliasKey(assignee)]; ok {
		return kpiIdentityFromUser(u)
	}
	return kpiAssigneeIdentity{
		Name:       assignee,
		Department: "未分配",
	}
}

func kpiIdentityFromUser(user userdb.User) kpiAssigneeIdentity {
	name := strings.TrimSpace(user.Name)
	if name == "" {
		name = strings.TrimSpace(user.Username)
	}
	return kpiAssigneeIdentity{
		Name:       normalizeAssignee(name),
		Username:   strings.TrimSpace(user.Username),
		Avatar:     strings.TrimSpace(user.Avatar),
		Department: normalizeDepartment(user.Department),
		IsLinked:   true,
	}
}

func ensureKPIStats(statsMap map[string]*kpiAssigneeStats, identity kpiAssigneeIdentity) *kpiAssigneeStats {
	key := kpiStatKey(identity.Name)
	if stat, exists := statsMap[key]; exists {
		if identity.IsLinked {
			stat.isLinked = true
			stat.username = identity.Username
			stat.avatar = identity.Avatar
			stat.dept = identity.Department
		}
		return stat
	}

	stat := &kpiAssigneeStats{
		name:     identity.Name,
		username: identity.Username,
		avatar:   identity.Avatar,
		dept:     normalizeDepartment(identity.Department),
		isLinked: identity.IsLinked,
	}
	statsMap[key] = stat
	return stat
}

func buildUserRiskNotes(stat *kpiAssigneeStats, avgCycleDays float64) []string {
	notes := make([]string, 0, 4)
	if stat.activeOverdue > 0 {
		notes = append(notes, fmt.Sprintf("当前有 %d 项任务超期未完成", stat.activeOverdue))
	}
	if stat.overdueCompleted > 0 {
		notes = append(notes, fmt.Sprintf("本周期有 %d 项任务延期完成", stat.overdueCompleted))
	}
	if stat.reviewCount > 0 {
		notes = append(notes, fmt.Sprintf("当前有 %d 项任务停留在评审", stat.reviewCount))
	}
	if avgCycleDays >= 7 {
		notes = append(notes, fmt.Sprintf("平均交付周期 %.1f 天，建议拆小需求或提前评审", avgCycleDays))
	}
	return notes
}

func buildPersonalHighlights(userKPI UserKPIDTO) []string {
	highlights := []string{
		fmt.Sprintf("本周期交付 %d 项，需求 %d 项，大需求 %d 项，故障 %d 项。", userKPI.TotalCompleted, userKPI.TasksCompleted, userKPI.DemandsCompleted, userKPI.BugsCompleted),
		fmt.Sprintf("平均交付周期 %.1f 天，关联 MR %d 次，当前评审中 %d 项。", userKPI.AvgCycleDays, userKPI.MRCount, userKPI.ReviewCount),
	}
	if userKPI.ActiveOverdue == 0 && userKPI.OverdueCompleted == 0 {
		highlights = append(highlights, "暂未发现超期交付风险。")
	}
	return highlights
}

func includeUserInReport(userKPI UserKPIDTO, userFilter string) bool {
	filter := strings.ToLower(strings.TrimSpace(userFilter))
	if filter != "" {
		return filter == strings.ToLower(userKPI.Name) ||
			filter == strings.ToLower(userKPI.Username) ||
			filter == emailPrefix(userKPI.Username)
	}
	return userKPI.TotalCompleted > 0 ||
		userKPI.OverdueCompleted > 0 ||
		userKPI.ActiveOverdue > 0 ||
		userKPI.ReviewCount > 0 ||
		userKPI.MRCount > 0
}

func buildKPIEvidenceBundle(completedTasks, activeTasks []db.TaskTelemetry, directory kpiUserDirectory, includedUserKeys map[string]bool, userFilter string, now time.Time) kpiEvidenceBundle {
	bundle := kpiEvidenceBundle{
		byUserKey: make(map[string][]string),
		byDept:    make(map[string][]string),
	}
	includeAll := strings.TrimSpace(userFilter) == ""
	seen := make(map[string]bool)

	completedSorted := append([]db.TaskTelemetry(nil), completedTasks...)
	sort.SliceStable(completedSorted, func(i, j int) bool {
		return taskCompletionTime(completedSorted[i]).After(taskCompletionTime(completedSorted[j]))
	})
	for _, task := range completedSorted {
		if !includeTaskForReport(task, directory, includedUserKeys, includeAll) {
			continue
		}
		bundle.completed = append(bundle.completed, task)
		appendKPIEvidence(&bundle, task, directory, seen)
	}

	activeCandidates := make([]db.TaskTelemetry, 0, len(activeTasks))
	for _, task := range activeTasks {
		if !isActiveOverdue(task, now) && !strings.EqualFold(strings.TrimSpace(task.Status), "review") {
			continue
		}
		if !includeTaskForReport(task, directory, includedUserKeys, includeAll) {
			continue
		}
		activeCandidates = append(activeCandidates, task)
	}
	sort.SliceStable(activeCandidates, func(i, j int) bool {
		if activeCandidates[i].DueDate != nil && activeCandidates[j].DueDate != nil {
			return activeCandidates[i].DueDate.Before(*activeCandidates[j].DueDate)
		}
		if activeCandidates[i].DueDate != nil {
			return true
		}
		if activeCandidates[j].DueDate != nil {
			return false
		}
		return activeCandidates[i].LastUpdate.After(activeCandidates[j].LastUpdate)
	})
	for _, task := range activeCandidates {
		bundle.active = append(bundle.active, task)
		appendKPIEvidence(&bundle, task, directory, seen)
	}

	return bundle
}

func includeTaskForReport(task db.TaskTelemetry, directory kpiUserDirectory, includedUserKeys map[string]bool, includeAll bool) bool {
	identity := directory.resolve(task.Assignee)
	return includedUserKeys[kpiStatKey(identity.Name)]
}

func appendKPIEvidence(bundle *kpiEvidenceBundle, task db.TaskTelemetry, directory kpiUserDirectory, seen map[string]bool) {
	id := strings.TrimSpace(task.TaskID)
	if id == "" {
		return
	}
	identity := directory.resolve(task.Assignee)
	userKey := kpiStatKey(identity.Name)
	dept := normalizeDepartment(identity.Department)
	bundle.byUserKey[userKey] = appendUniqueString(bundle.byUserKey[userKey], id)
	bundle.byDept[dept] = appendUniqueString(bundle.byDept[dept], id)

	if seen[id] {
		return
	}
	seen[id] = true
	bundle.items = append(bundle.items, KPIReportEvidenceDTO{
		ID:          id,
		TaskID:      task.TaskID,
		Title:       task.Title,
		Repo:        task.Repo,
		Assignee:    identity.Name,
		Department:  dept,
		Status:      task.Status,
		IssueType:   normalizeIssueType(task.IssueType),
		DueDate:     formatOptionalDate(task.DueDate),
		CompletedAt: formatOptionalDate(task.CompletedAt),
		LastUpdate:  formatDateTime(task.LastUpdate),
		MRURL:       task.MrURL,
		TaskGroupID: task.TaskGroupID,
	})
}

func buildDepartmentReportSections(userKPIList []UserKPIDTO, includedDepartments map[string]bool, userFilter string, evidenceByDept map[string][]string) []KPIReportDepartmentSectionDTO {
	includeAll := strings.TrimSpace(userFilter) == ""
	deptMap := make(map[string]*KPIReportDepartmentSectionDTO)
	for _, userKPI := range userKPIList {
		dept := normalizeDepartment(userKPI.Department)
		if !includeAll && !includedDepartments[dept] {
			continue
		}
		section, exists := deptMap[dept]
		if !exists {
			section = &KPIReportDepartmentSectionDTO{Department: dept}
			deptMap[dept] = section
		}
		section.TasksCompleted += userKPI.TasksCompleted
		section.BugsCompleted += userKPI.BugsCompleted
		section.DemandsCompleted += userKPI.DemandsCompleted
		section.TotalCompleted += userKPI.TotalCompleted
		section.OverdueCompleted += userKPI.OverdueCompleted
		section.ActiveOverdue += userKPI.ActiveOverdue
		section.ReviewCount += userKPI.ReviewCount
		section.MRCount += userKPI.MRCount
	}

	sections := make([]KPIReportDepartmentSectionDTO, 0, len(deptMap))
	for dept, section := range deptMap {
		if includeAll && section.TotalCompleted == 0 && section.ActiveOverdue == 0 && section.ReviewCount == 0 && section.MRCount == 0 {
			continue
		}
		section.EvidenceIDs = evidenceByDept[dept]
		section.Highlights = buildDepartmentHighlights(*section)
		sections = append(sections, *section)
	}
	sort.Slice(sections, func(i, j int) bool {
		if sections[i].TotalCompleted != sections[j].TotalCompleted {
			return sections[i].TotalCompleted > sections[j].TotalCompleted
		}
		if sections[i].ActiveOverdue != sections[j].ActiveOverdue {
			return sections[i].ActiveOverdue > sections[j].ActiveOverdue
		}
		return sections[i].Department < sections[j].Department
	})
	return sections
}

func buildDepartmentHighlights(section KPIReportDepartmentSectionDTO) []string {
	highlights := []string{
		fmt.Sprintf("部门交付 %d 项，需求 %d 项，大需求 %d 项，故障 %d 项。", section.TotalCompleted, section.TasksCompleted, section.DemandsCompleted, section.BugsCompleted),
		fmt.Sprintf("当前超期 %d 项，评审中 %d 项，关联 MR %d 次。", section.ActiveOverdue, section.ReviewCount, section.MRCount),
	}
	if section.OverdueCompleted > 0 {
		highlights = append(highlights, fmt.Sprintf("本周期有 %d 项延期完成，建议复盘排期假设。", section.OverdueCompleted))
	}
	return highlights
}

func buildKPIReportRisks(evidence kpiEvidenceBundle, directory kpiUserDirectory, now time.Time) []KPIReportRiskDTO {
	risks := make([]KPIReportRiskDTO, 0)
	for _, task := range evidence.active {
		if !isActiveOverdue(task, now) {
			continue
		}
		identity := directory.resolve(task.Assignee)
		level := "medium"
		if task.DueDate != nil && now.Sub(*task.DueDate) >= 48*time.Hour {
			level = "high"
		}
		risks = append(risks, KPIReportRiskDTO{
			Level:       level,
			Owner:       identity.Name,
			Department:  normalizeDepartment(identity.Department),
			Title:       "进行中任务已超期",
			Detail:      fmt.Sprintf("%s 已超过截止日期 %s，当前状态为 %s。", task.Title, formatOptionalDate(task.DueDate), task.Status),
			EvidenceIDs: []string{task.TaskID},
		})
		if len(risks) >= 8 {
			return risks
		}
	}

	for _, task := range evidence.completed {
		if !isCompletedAfterDue(task) {
			continue
		}
		identity := directory.resolve(task.Assignee)
		risks = append(risks, KPIReportRiskDTO{
			Level:       "medium",
			Owner:       identity.Name,
			Department:  normalizeDepartment(identity.Department),
			Title:       "本周期存在延期完成",
			Detail:      fmt.Sprintf("%s 在截止日期 %s 后完成。", task.Title, formatOptionalDate(task.DueDate)),
			EvidenceIDs: []string{task.TaskID},
		})
		if len(risks) >= 8 {
			return risks
		}
	}
	return risks
}

func buildKPIReportMeetingFocus(evidence kpiEvidenceBundle, directory kpiUserDirectory, now time.Time) []KPIReportMeetingFocusDTO {
	focus := make([]KPIReportMeetingFocusDTO, 0)
	seen := make(map[string]bool)
	for _, task := range evidence.active {
		if !isActiveOverdue(task, now) {
			continue
		}
		identity := directory.resolve(task.Assignee)
		focus = append(focus, KPIReportMeetingFocusDTO{
			Topic:       "明确超期任务处置",
			Owner:       identity.Name,
			Department:  normalizeDepartment(identity.Department),
			Reason:      fmt.Sprintf("%s 已超期，需要确认缩减范围、调整截止日期或补充协作人。", task.Title),
			EvidenceIDs: []string{task.TaskID},
		})
		seen["overdue:"+task.TaskID] = true
		if len(focus) >= 8 {
			return focus
		}
	}

	for _, task := range evidence.active {
		if !strings.EqualFold(strings.TrimSpace(task.Status), "review") {
			continue
		}
		key := "review:" + task.TaskID
		if seen[key] {
			continue
		}
		identity := directory.resolve(task.Assignee)
		focus = append(focus, KPIReportMeetingFocusDTO{
			Topic:       "推动评审合并",
			Owner:       identity.Name,
			Department:  normalizeDepartment(identity.Department),
			Reason:      fmt.Sprintf("%s 当前处于评审状态，需要确认 reviewer、阻塞点和合并条件。", task.Title),
			EvidenceIDs: []string{task.TaskID},
		})
		seen[key] = true
		if len(focus) >= 8 {
			return focus
		}
	}
	return focus
}

func buildKPIReportOverview(window kpiPeriodWindow, reportType, userFilter string, personalSections []KPIReportPersonalSectionDTO, departmentSections []KPIReportDepartmentSectionDTO) KPIReportOverviewDTO {
	totalCompleted := 0
	activeOverdue := 0
	overdueCompleted := 0
	mrCount := 0
	for _, section := range personalSections {
		totalCompleted += section.User.TotalCompleted
		activeOverdue += section.User.ActiveOverdue
		overdueCompleted += section.User.OverdueCompleted
		mrCount += section.User.MRCount
	}

	reportLabel := "周报"
	if reportType == "daily" {
		reportLabel = "日报"
	}
	title := fmt.Sprintf("%s预览", reportLabel)
	if strings.TrimSpace(userFilter) != "" {
		title = fmt.Sprintf("%s个人预览", reportLabel)
	}
	summary := fmt.Sprintf("%s内已交付 %d 项，当前超期 %d 项，延期完成 %d 项，关联 MR %d 次。", window.Label, totalCompleted, activeOverdue, overdueCompleted, mrCount)

	return KPIReportOverviewDTO{
		Title:            title,
		PeriodLabel:      window.Label,
		Summary:          summary,
		TotalCompleted:   totalCompleted,
		ActiveOverdue:    activeOverdue,
		OverdueCompleted: overdueCompleted,
		MRCount:          mrCount,
		PersonalCount:    len(personalSections),
		DepartmentCount:  len(departmentSections),
	}
}

func normalizeAssignee(assignee string) string {
	assignee = strings.TrimSpace(assignee)
	if assignee == "" || assignee == "-" || assignee == "未指派" {
		return "未分配"
	}
	return assignee
}

func normalizeDepartment(department string) string {
	department = strings.TrimSpace(department)
	if department == "" {
		return "未分配"
	}
	return department
}

func normalizeIssueType(issueType string) string {
	issueType = strings.TrimSpace(issueType)
	if issueType == "" {
		return "task"
	}
	return strings.ToLower(issueType)
}

func emailPrefix(username string) string {
	username = strings.ToLower(strings.TrimSpace(username))
	if idx := strings.Index(username, "@"); idx != -1 {
		return username[:idx]
	}
	return username
}

func kpiStatKey(value string) string {
	return strings.ToLower(normalizeAssignee(value))
}

func isCompletedAfterDue(task db.TaskTelemetry) bool {
	if task.DueDate == nil {
		return false
	}
	completedAt := taskCompletionTime(task)
	return !completedAt.IsZero() && completedAt.After(*task.DueDate)
}

func isActiveOverdue(task db.TaskTelemetry, now time.Time) bool {
	if task.DueDate == nil || strings.EqualFold(strings.TrimSpace(task.Status), "done") {
		return false
	}
	return now.After(*task.DueDate)
}

func taskCompletionTime(task db.TaskTelemetry) time.Time {
	if task.CompletedAt != nil {
		return *task.CompletedAt
	}
	if !task.LastUpdate.IsZero() {
		return task.LastUpdate
	}
	return task.TaskCreatedAt
}

func taskCycleDays(task db.TaskTelemetry) (float64, bool) {
	start := task.TaskCreatedAt
	if start.IsZero() {
		return 0, false
	}
	end := taskCompletionTime(task)
	if end.IsZero() || end.Before(start) {
		return 0, false
	}
	return end.Sub(start).Hours() / 24, true
}

func hasMergeRequest(task db.TaskTelemetry) bool {
	return strings.TrimSpace(task.MrURL) != "" || task.MrIID > 0
}

func roundOneDecimal(value float64) float64 {
	return math.Round(value*10) / 10
}

func appendUniqueString(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func formatOptionalDate(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func formatDateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}
