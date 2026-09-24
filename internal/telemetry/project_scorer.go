package telemetry

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

// CalculateAndSaveScores computes the health indices (PHDI) for all active projects
func CalculateAndSaveScores() ([]db.ProjectScore, error) {
	var tasks []db.TaskTelemetry
	if err := db.DB.Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch tasks for scoring: %v", err)
	}

	// Fetch latest Jira sync projects configuration
	var latestConfig db.ConfigVersion
	var syncProjects []string
	if err := db.DB.Order("version desc").First(&latestConfig).Error; err == nil {
		var cfg struct {
			Jira config.JiraConfig `json:"jira"`
		}
		if errDec := json.Unmarshal([]byte(latestConfig.ConfigJSON), &cfg); errDec == nil {
			syncProjects = config.JiraProjectKeys(&cfg.Jira)
		}
	}
	var projectConfigs []db.ProjectConfig
	if err := db.DB.Find(&projectConfigs).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch project configs for scoring: %v", err)
	}
	projectConfigByKey := make(map[string]db.ProjectConfig, len(projectConfigs))
	for _, projectConfig := range projectConfigs {
		key := strings.ToUpper(strings.TrimSpace(projectConfig.ProjectKey))
		if key != "" {
			projectConfigByKey[key] = projectConfig
		}
	}

	// Helper to check if key is a valid Jira project key format
	isValidJiraKey := func(k string) bool {
		if len(k) < 2 || len(k) > 10 {
			return false
		}
		first := k[0]
		if !((first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z')) {
			return false
		}
		for i := 0; i < len(k); i++ {
			c := k[i]
			if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
				return false
			}
		}
		return true
	}

	// Helper to check if key is configured for sync
	isSyncProject := func(k string) bool {
		if !isValidJiraKey(k) {
			return false
		}
		if len(syncProjects) == 0 {
			_, configured := projectConfigByKey[strings.ToUpper(strings.TrimSpace(k))]
			return configured
		}
		for _, p := range syncProjects {
			if strings.EqualFold(strings.TrimSpace(p), k) {
				return true
			}
		}
		return false
	}

	// Global name map based on any task's Repo label (e.g. "PRJ23003-重庆赛力斯三厂-QTruck (CHQ)")
	keyToName := make(map[string]string)
	namePrefixToName := make(map[string]string) // e.g. "PRJ23003" -> "PRJ23003-重庆赛力斯三厂-QTruck"
	for _, t := range tasks {
		if t.Repo != "" && t.Repo != "-" {
			idxOpen := strings.LastIndex(t.Repo, "(")
			idxClose := strings.LastIndex(t.Repo, ")")
			if idxOpen > 0 && idxClose > idxOpen {
				key := strings.TrimSpace(strings.ToUpper(t.Repo[idxOpen+1 : idxClose]))
				name := strings.TrimSpace(t.Repo[:idxOpen])
				if key != "" && name != "" {
					keyToName[key] = name
					// Check if name has a prefix before "-"
					idxDash := strings.Index(name, "-")
					if idxDash > 0 {
						prefix := strings.TrimSpace(strings.ToUpper(name[:idxDash]))
						namePrefixToName[prefix] = name
					}
				}
			}
		}
	}

	// 1. Group tasks by persisted project fact. Task-ID prefixes are only a
	// measured compatibility fallback and must still match an authoritative project.
	projectTasks := make(map[string][]db.TaskTelemetry)
	for _, task := range tasks {
		key := db.ResolveTaskProjectKey(task)
		if key == "" || !isSyncProject(key) {
			continue
		}
		projectTasks[key] = append(projectTasks[key], task)
	}

	now := time.Now()
	snapshotDate := now.Format("2006-01-02")
	var scores []db.ProjectScore

	for projKey, tList := range projectTasks {
		projKeyUpper := strings.ToUpper(projKey)
		extractedName := ""

		// 1. Try key map directly
		if name, ok := keyToName[projKeyUpper]; ok {
			extractedName = name
		}

		// 2. Try prefix map if not found
		if extractedName == "" {
			if name, ok := namePrefixToName[projKeyUpper]; ok {
				extractedName = name
			}
		}

		// 3. Fallback to scanning this project's tasks (local check)
		if extractedName == "" {
			for _, t := range tList {
				if t.Repo != "" && t.Repo != "-" {
					idxOpen := strings.LastIndex(t.Repo, "(")
					if idxOpen > 0 {
						extractedName = strings.TrimSpace(t.Repo[:idxOpen])
						break
					}
				}
			}
		}

		// 2. Project configuration is authoritative, read-only input. Scoring
		// must not infer or overwrite saved project facts from telemetry labels.
		projectConfig, hasProjectConfig := projectConfigByKey[projKeyUpper]
		if !hasProjectConfig {
			defaultName := projKey
			if extractedName != "" {
				defaultName = extractedName
			}
			projectConfig = db.ProjectConfig{
				ProjectKey:      projKey,
				ProjectName:     defaultName,
				BasePriority:    "P1",
				GitReposJSON:    "[]",
				BaseScore:       60.0,
				BaseScoreWeight: 0.10,
			}
		}

		// 3. Perform multidimensional calculations
		sh := computeScheduleHealth(tList, now)
		eq := computeEngineeringQuality(tList)
		ce := computeCollaborationEfficiency(tList)
		si := computeStabilityIndex(tList)

		// Compound Health Score (PHDI) with Base Score Integration
		baseWeight := projectConfig.BaseScoreWeight
		if baseWeight < 0 {
			baseWeight = 0
		} else if baseWeight > 1 {
			baseWeight = 1
		}
		baseScore := projectConfig.BaseScore

		metricsScore := 0.30*sh + 0.25*eq + 0.25*ce + 0.20*si
		phdi := baseWeight*baseScore + (1.0-baseWeight)*metricsScore

		// Generate smart diagnostic text based on details
		diagnostic := generateDiagnostic(projKey, projectConfig.BasePriority, sh, eq, ce, si, tList)

		score := db.ProjectScore{
			ProjectKey:          projKey,
			ProjectName:         projectConfig.ProjectName,
			ScheduleHealthScore: math.Round(sh*100) / 100,
			EngineeringQuality:  math.Round(eq*100) / 100,
			CollaborationEffic:  math.Round(ce*100) / 100,
			StabilityIndex:      math.Round(si*100) / 100,
			CompoundScore:       math.Round(phdi*100) / 100,
			Diagnostic:          diagnostic,
			SnapshotDate:        snapshotDate,
			CreatedAt:           now,
		}

		// Save score snapshot to database
		// Overwrite if snapshot for this date already exists, otherwise create new
		var existing db.ProjectScore
		err := db.DB.Where("project_key = ? AND snapshot_date = ?", projKey, snapshotDate).First(&existing).Error
		if err == nil {
			score.ID = existing.ID
			db.DB.Save(&score)
		} else {
			db.DB.Create(&score)
		}

		scores = append(scores, score)
	}

	return scores, nil
}

func getProjectKey(taskID string) string {
	idx := strings.Index(taskID, "-")
	if idx <= 0 {
		return ""
	}
	return strings.ToUpper(taskID[:idx])
}

// SH: Schedule Health
func computeScheduleHealth(tasks []db.TaskTelemetry, now time.Time) float64 {
	var totalDemands float64
	var scheduledDemands float64
	var overdueTasks float64
	var totalTasks float64

	for _, task := range tasks {
		issueT := strings.ToLower(task.IssueType)
		isDemand := issueT == "demand" || strings.Contains(issueT, "需求") || strings.HasPrefix(task.TaskID, "DEMAND-")

		if isDemand {
			totalDemands++
			if task.DueDate != nil {
				scheduledDemands++
			}
		}

		totalTasks++
		if task.DueDate != nil && task.Status != "done" && task.DueDate.Before(now) {
			overdueTasks++
		}
	}

	sr := 1.0 // Schedule Coverage Rate
	if totalDemands > 0 {
		sr = scheduledDemands / totalDemands
	}

	or := 0.0 // Overdue Rate
	if totalTasks > 0 {
		or = overdueTasks / totalTasks
	}

	return 100.0 * (0.4*sr + 0.6*(1.0-or))
}

// EQ: Engineering Quality
func computeEngineeringQuality(tasks []db.TaskTelemetry) float64 {
	var activeDevTasks float64
	var branchBoundTasks float64

	for _, task := range tasks {
		issueT := strings.ToLower(task.IssueType)
		isBugOrDemand := issueT == "demand" || issueT == "bug" || strings.Contains(issueT, "需求") || strings.Contains(issueT, "缺陷") || strings.HasPrefix(task.TaskID, "DEMAND-") || strings.HasPrefix(task.TaskID, "BUG-")

		if isBugOrDemand && task.Status != "backlog" {
			activeDevTasks++
			cleanedBranch := strings.TrimSpace(task.Branch)
			if cleanedBranch != "" && cleanedBranch != "-" {
				branchBoundTasks++
			}
		}
	}

	cr := 1.0 // Code evidence binding rate
	if activeDevTasks > 0 {
		cr = branchBoundTasks / activeDevTasks
	}

	// We assume a standard Unit Test success rate of 95% as telemetry is mock-fed
	tr := 0.95

	return 100.0 * (0.6*cr + 0.4*tr)
}

// CE: Collaboration Efficiency
func computeCollaborationEfficiency(tasks []db.TaskTelemetry) float64 {
	// 1. Calculate Task Assignment Gini Coefficient
	assigneeCounts := make(map[string]float64)
	for _, task := range tasks {
		if task.Status != "done" {
			cleanedName := strings.TrimSpace(task.Assignee)
			if cleanedName == "" || cleanedName == "未指派" || cleanedName == "unassigned" {
				cleanedName = "Unassigned"
			}
			assigneeCounts[cleanedName]++
		}
	}

	var gini float64
	if len(assigneeCounts) > 1 {
		var values []float64
		for _, val := range assigneeCounts {
			values = append(values, val)
		}
		sort.Float64s(values)
		n := float64(len(values))

		var sumOfX float64
		for _, x := range values {
			sumOfX += x
		}

		if sumOfX > 0 {
			var weightedSum float64
			for i, x := range values {
				weightedSum += float64(2*(i+1)-int(n)-1) * x
			}
			gini = weightedSum / (n * sumOfX)
		}
	}

	// 2. Subtask deconstruction depth (ideal deconstruction rate: 3~8 tasks per demand)
	// We check task group sizes
	groupTaskCount := make(map[string]int)
	for _, task := range tasks {
		gID := strings.TrimSpace(task.TaskGroupID)
		if gID != "" && gID != "-" {
			groupTaskCount[gID]++
		}
	}

	var totalDeconstructRatio float64
	var demandCount float64
	for _, count := range groupTaskCount {
		demandCount++
		// Normalise around 5 tasks (ideal)
		ratio := float64(count) / 5.0
		if ratio > 1.0 {
			ratio = 1.0 / ratio // Penalize over-fragmentation
		}
		totalDeconstructRatio += ratio
	}

	sd := 1.0
	if demandCount > 0 {
		sd = totalDeconstructRatio / demandCount
	}

	return 100.0 * (0.5*(1.0-gini) + 0.5*sd)
}

// SI: Stability Index
func computeStabilityIndex(tasks []db.TaskTelemetry) float64 {
	var bugCount float64
	var demandCount float64
	var resolvedBugs float64
	var totalBugResolveDays float64

	for _, task := range tasks {
		issueT := strings.ToLower(task.IssueType)
		isBug := issueT == "bug" || strings.Contains(issueT, "缺陷") || strings.Contains(issueT, "故障")
		isDemand := issueT == "demand" || strings.Contains(issueT, "需求") || strings.HasPrefix(task.TaskID, "DEMAND-")

		if isBug {
			bugCount++
			if task.Status == "done" && task.CompletedAt != nil {
				resolvedBugs++
				resolveDuration := task.CompletedAt.Sub(task.TaskCreatedAt)
				days := resolveDuration.Hours() / 24.0
				if days < 0 {
					days = 0
				}
				totalBugResolveDays += days
			}
		}
		if isDemand {
			demandCount++
		}
	}

	bd := 0.0
	if demandCount > 0 {
		bd = bugCount / demandCount
	}

	rt := 1.0
	if resolvedBugs > 0 {
		rt = totalBugResolveDays / resolvedBugs
	}

	// BD threshold: 0.5 bugs per demand is penalty floor
	bdScore := 1.0 - bd/0.5
	if bdScore < 0 {
		bdScore = 0
	}

	// RT threshold: 7 days resolve time is floor
	rtScore := 1.0 - rt/7.0
	if rtScore < 0 {
		rtScore = 0
	}

	return 100.0 * (0.5*bdScore + 0.5*rtScore)
}

func generateDiagnostic(projKey, priority string, sh, eq, ce, si float64, tasks []db.TaskTelemetry) string {
	var warnings []string

	// 1. Check Overdue
	var overdueCount int
	for _, t := range tasks {
		if t.DueDate != nil && t.Status != "done" && t.DueDate.Before(time.Now()) {
			overdueCount++
		}
	}
	if overdueCount > 0 {
		warnings = append(warnings, fmt.Sprintf("存在 %d 个任务发生逾期，严重拉低进度评分", overdueCount))
	}

	// 2. Check Unscheduled
	var totalDemands, unscheduledDemands int
	for _, t := range tasks {
		issueT := strings.ToLower(t.IssueType)
		isDemand := issueT == "demand" || strings.Contains(issueT, "需求") || strings.HasPrefix(t.TaskID, "DEMAND-")
		if isDemand {
			totalDemands++
			if t.DueDate == nil {
				unscheduledDemands++
			}
		}
	}
	if unscheduledDemands > 0 && totalDemands > 0 {
		pct := (float64(unscheduledDemands) / float64(totalDemands)) * 100.0
		warnings = append(warnings, fmt.Sprintf("有 %.0f%% 的需求尚未录入排期，请及时确认交付点", pct))
	}

	// 3. Check code evidence
	var activeDev, missingEvidence int
	for _, t := range tasks {
		if t.Status != "backlog" {
			issueT := strings.ToLower(t.IssueType)
			if issueT == "demand" || strings.Contains(issueT, "需求") {
				activeDev++
				cleanedB := strings.TrimSpace(t.Branch)
				if cleanedB == "" || cleanedB == "-" {
					missingEvidence++
				}
			}
		}
	}
	if missingEvidence > 0 {
		warnings = append(warnings, fmt.Sprintf("有 %d 个进入开发阶段的需求缺失 Git 分支绑定", missingEvidence))
	}

	// 4. Check Gini workload imbalance
	assigneeCounts := make(map[string]int)
	for _, t := range tasks {
		if t.Status != "done" {
			assigneeCounts[t.Assignee]++
		}
	}
	maxTasks := 0
	busyMan := ""
	totalActiveTasks := 0
	for name, count := range assigneeCounts {
		totalActiveTasks += count
		if count > maxTasks {
			maxTasks = count
			busyMan = name
		}
	}
	if totalActiveTasks > 3 && maxTasks > 0 {
		ratio := float64(maxTasks) / float64(totalActiveTasks)
		if ratio > 0.6 {
			warnings = append(warnings, fmt.Sprintf("任务分配极度失衡，负责人【%s】承担了 %.0f%% 的负载", busyMan, ratio*100))
		}
	}

	// 5. Check bug density
	var bugCount int
	for _, t := range tasks {
		issueT := strings.ToLower(t.IssueType)
		if issueT == "bug" || strings.Contains(issueT, "缺陷") {
			bugCount++
		}
	}
	if bugCount > 5 {
		warnings = append(warnings, fmt.Sprintf("积压缺陷数达 %d 个，产品质量稳定度处于预警状态", bugCount))
	}

	if len(warnings) == 0 {
		return fmt.Sprintf("大脑诊断：当前【%s】项目交付极为健康，进度、代码和协作各项指标优异，请继续保持。", projKey)
	}

	diag := fmt.Sprintf("大脑诊断：当前【%s】项目综合健康指数处于警戒阶段。核心问题包括：\n", projKey)
	for i, w := range warnings {
		diag += fmt.Sprintf("  %d) %s；\n", i+1, w)
	}
	diag += "建议优先分配过载人员的任务，并补全开发代码证据以纠偏项目状态。"
	return diag
}
