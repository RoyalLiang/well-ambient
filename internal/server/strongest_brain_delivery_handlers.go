package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

type StrongestBrainDeliveryCockpitResponse struct {
	GeneratedAt     string                              `json:"generated_at"`
	NorthStar       string                              `json:"north_star"`
	Health          StrongestBrainDeliveryHealth        `json:"health"`
	Evidence        StrongestBrainDeliveryEvidence      `json:"evidence"`
	Exceptions      StrongestBrainDeliveryExceptions    `json:"exceptions"`
	WeeklyDecisions StrongestBrainWeeklyDecisionSummary `json:"weekly_decisions"`
	Schedule        StrongestBrainDeliverySchedule      `json:"schedule"`
	AITrace         StrongestBrainAITraceSummary        `json:"ai_trace"`
	Override        StrongestBrainOverrideSummary       `json:"override"`
	Authorization   StrongestBrainAuthorizationSummary  `json:"authorization"`
	EntryPoints     []StrongestBrainEntryPoint          `json:"entry_points"`
}

type StrongestBrainDeliveryHealth struct {
	Score                int      `json:"score"`
	Label                string   `json:"label"`
	EvidenceCompleteness int      `json:"evidence_completeness"`
	OpenExceptions       int      `json:"open_exceptions"`
	WeeklyDecisionCount  int      `json:"weekly_decision_count"`
	AITraceability       int      `json:"ai_traceability"`
	ScheduleHighRisk     int      `json:"schedule_high_risk"`
	Signals              []string `json:"signals"`
}

type StrongestBrainDeliveryEvidence struct {
	TotalRequirements       int                                 `json:"total_requirements"`
	CompleteChains          int                                 `json:"complete_chains"`
	IncompleteChains        int                                 `json:"incomplete_chains"`
	AverageCompleteness     int                                 `json:"average_completeness"`
	JiraCodeMismatch        int                                 `json:"jira_code_mismatch"`
	MissingCodeEvidence     int                                 `json:"missing_code_evidence"`
	MergedButStatusOpen     int                                 `json:"merged_but_status_open"`
	TopMissingLinks         []string                            `json:"top_missing_links"`
	RequirementCompleteness []StrongestBrainRequirementEvidence `json:"requirement_completeness"`
	RecentEvidence          []StrongestBrainEvidenceDigest      `json:"recent_evidence"`
}

type StrongestBrainRequirementEvidence struct {
	DemandID        string   `json:"demand_id"`
	Title           string   `json:"title"`
	Assignee        string   `json:"assignee"`
	Project         string   `json:"project"`
	ChainStatus     string   `json:"chain_status"`
	Completeness    int      `json:"completeness"`
	MissingLinks    []string `json:"missing_links"`
	EvidenceRefs    []string `json:"evidence_refs"`
	RecommendedNext string   `json:"recommended_next"`
}

type StrongestBrainDeliveryExceptions struct {
	Total int                               `json:"total"`
	P0    int                               `json:"p0"`
	P1    int                               `json:"p1"`
	P2    int                               `json:"p2"`
	Items []StrongestBrainDeliveryException `json:"items"`
}

type StrongestBrainDeliveryException struct {
	ID                string   `json:"id"`
	Type              string   `json:"type"`
	Severity          string   `json:"severity"`
	Title             string   `json:"title"`
	Reason            string   `json:"reason"`
	DecisionOwner     string   `json:"decision_owner"`
	Deadline          string   `json:"deadline"`
	RecommendedAction string   `json:"recommended_action"`
	EvidenceRefs      []string `json:"evidence_refs"`
	Source            string   `json:"source"`
}

type StrongestBrainWeeklyDecisionSummary struct {
	Total        int                                `json:"total"`
	MustDecide   int                                `json:"must_decide"`
	ThisWeek     int                                `json:"this_week"`
	DecisionDebt int                                `json:"decision_debt"`
	Items        []StrongestBrainWeeklyDecisionItem `json:"items"`
}

type StrongestBrainWeeklyDecisionItem struct {
	ID                string   `json:"id"`
	Question          string   `json:"question"`
	WhyNow            string   `json:"why_now"`
	Options           []string `json:"options"`
	RecommendedAction string   `json:"recommended_action"`
	DecisionOwner     string   `json:"decision_owner"`
	Deadline          string   `json:"deadline"`
	EvidenceRefs      []string `json:"evidence_refs"`
}

type StrongestBrainDeliverySchedule struct {
	Total           int                            `json:"total"`
	Scheduled       int                            `json:"scheduled"`
	Unscheduled     int                            `json:"unscheduled"`
	Overdue         int                            `json:"overdue"`
	DueSoon         int                            `json:"due_soon"`
	Stale           int                            `json:"stale"`
	HighRisk        int                            `json:"high_risk"`
	RiskCalendarURL string                         `json:"risk_calendar_url"`
	Upcoming        []ScheduleRiskCalendarEventDTO `json:"upcoming"`
}

type StrongestBrainAITraceSummary struct {
	TotalOutputs        int                         `json:"total_outputs"`
	TraceableOutputs    int                         `json:"traceable_outputs"`
	TraceabilityPercent int                         `json:"traceability_percent"`
	AverageReadiness    int                         `json:"average_readiness"`
	Latest              []StrongestBrainAITraceItem `json:"latest"`
}

type StrongestBrainAITraceItem struct {
	ID                    uint     `json:"id"`
	DemandID              string   `json:"demand_id"`
	TaskGroupID           string   `json:"task_group_id"`
	ContextPackID         uint     `json:"context_pack_id"`
	ContextPackSummary    string   `json:"context_pack_summary"`
	InputSummary          string   `json:"input_summary"`
	Model                 string   `json:"model"`
	PromptTemplateVersion string   `json:"prompt_template_version"`
	RuleVersion           string   `json:"rule_version"`
	Confidence            float64  `json:"confidence"`
	ReadinessScore        int      `json:"readiness_score"`
	MissingQuestions      []string `json:"missing_questions"`
	AcceptanceCriteria    []string `json:"acceptance_criteria"`
	RiskFlags             []string `json:"risk_flags"`
	HumanFeedback         string   `json:"human_feedback"`
	CreatedAt             string   `json:"created_at"`
}

type StrongestBrainOverrideSummary struct {
	Recent        int                         `json:"recent"`
	Protected     int                         `json:"protected"`
	NeedsReview   int                         `json:"needs_review"`
	AuditTrailURL string                      `json:"audit_trail_url"`
	Latest        []StrongestBrainOverrideLog `json:"latest"`
}

type StrongestBrainOverrideLog struct {
	ID               uint   `json:"id"`
	TaskID           string `json:"task_id"`
	Actor            string `json:"actor"`
	Action           string `json:"action"`
	OriginalValue    string `json:"original_value"`
	OverrideValue    string `json:"override_value"`
	Reason           string `json:"reason"`
	ProtectionWindow string `json:"protection_window"`
	RollbackHint     string `json:"rollback_hint"`
	CreatedAt        string `json:"created_at"`
}

type StrongestBrainAuthorizationSummary struct {
	ExplainPanelAvailable bool   `json:"explain_panel_available"`
	Endpoint              string `json:"endpoint"`
	RecentDecisions       int    `json:"recent_decisions"`
	RecentDenials         int    `json:"recent_denials"`
	HighRiskGrants        int    `json:"high_risk_grants"`
}

type StrongestBrainEntryPoint struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Status      string `json:"status"`
}

type StrongestBrainDemandReadinessResponse struct {
	GeneratedAt string                               `json:"generated_at"`
	Summary     StrongestBrainDemandReadinessSummary `json:"summary"`
	Items       []StrongestBrainAITraceItem          `json:"items"`
}

type StrongestBrainDemandReadinessSummary struct {
	Total        int `json:"total"`
	Ready        int `json:"ready"`
	NeedsClarify int `json:"needs_clarify"`
	Blocked      int `json:"blocked"`
	AverageScore int `json:"average_score"`
}

func (s *Server) handleGetStrongestBrainDeliveryCockpit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	schedule, err := buildStrongestBrainScheduleSnapshot(now)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to build schedule snapshot: %v", err), http.StatusInternalServerError)
		return
	}
	execution, logs, err := buildStrongestBrainExecutionSnapshot(now)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to build execution snapshot: %v", err), http.StatusInternalServerError)
		return
	}
	riskCalendar := buildScheduleRiskCalendarResponse(schedule, now)
	decisionItems := buildStrongestBrainDeliveryDecisionItems(schedule, execution, now)
	aiTrace := buildStrongestBrainAITraceSummary(8)
	evidence := buildStrongestBrainDeliveryEvidence(schedule, execution, logs)
	exceptions := buildStrongestBrainDeliveryExceptions(decisionItems)
	weekly := buildStrongestBrainDeliveryWeeklyDecisions(decisionItems, now)
	override := buildStrongestBrainOverrideSummary(8, now)
	authorization := buildStrongestBrainAuthorizationSummary(now)

	response := StrongestBrainDeliveryCockpitResponse{
		GeneratedAt:     formatDateTime(now),
		NorthStar:       "系统维护事实，人处理判断",
		Health:          buildStrongestBrainDeliveryHealth(evidence, exceptions, weekly, aiTrace, riskCalendar),
		Evidence:        evidence,
		Exceptions:      exceptions,
		WeeklyDecisions: weekly,
		Schedule:        buildStrongestBrainDeliverySchedule(schedule, riskCalendar),
		AITrace:         aiTrace,
		Override:        override,
		Authorization:   authorization,
		EntryPoints:     strongestBrainEntryPoints(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleGetStrongestBrainAITraces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}
	limit := boundedQueryLimit(r, 50, 200)
	response := map[string]interface{}{
		"generated_at": formatDateTime(time.Now()),
		"summary":      buildStrongestBrainAITraceSummary(limit),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleGetStrongestBrainDemandReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}
	limit := boundedQueryLimit(r, 80, 200)
	items := buildStrongestBrainAITraceItems(limit)
	summary := StrongestBrainDemandReadinessSummary{Total: len(items)}
	var scoreSum int
	for _, item := range items {
		scoreSum += item.ReadinessScore
		if item.ReadinessScore >= 80 {
			summary.Ready++
		} else if item.ReadinessScore >= 50 {
			summary.NeedsClarify++
		} else {
			summary.Blocked++
		}
	}
	if summary.Total > 0 {
		summary.AverageScore = scoreSum / summary.Total
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(StrongestBrainDemandReadinessResponse{
		GeneratedAt: formatDateTime(time.Now()),
		Summary:     summary,
		Items:       items,
	})
}

func (s *Server) handleGetStrongestBrainOverrideAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}
	limit := boundedQueryLimit(r, 60, 200)
	summary := buildStrongestBrainOverrideSummary(limit, time.Now())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"generated_at": formatDateTime(time.Now()),
		"summary":      summary,
		"items":        summary.Latest,
	})
}

func buildStrongestBrainDeliveryDecisionItems(schedule ScheduleResponseDTO, execution ExecutionTasksResponseDTO, now time.Time) []StrongestBrainDecisionItem {
	items := make([]StrongestBrainDecisionItem, 0)
	for _, item := range schedule.Items {
		if item.RiskLevel == "safe" || item.RiskLevel == "done" || item.RiskLevel == "" {
			continue
		}
		items = append(items, decisionFromScheduleItem(item))
	}
	for _, item := range execution.Items {
		if item.RiskLevel == "safe" || item.RiskLevel == "done" || item.RiskLevel == "" {
			continue
		}
		items = append(items, decisionFromExecutionItem(item))
	}
	items = append(items, semanticEvidenceReviewDecisions(now)...)
	items = append(items, contextGapDecisions(now)...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Rank != items[j].Rank {
			return items[i].Rank > items[j].Rank
		}
		return items[i].UpdatedAt > items[j].UpdatedAt
	})
	return items
}

func buildStrongestBrainDeliveryEvidence(schedule ScheduleResponseDTO, execution ExecutionTasksResponseDTO, logs []db.GitCommitLog) StrongestBrainDeliveryEvidence {
	byDemand := make(map[string][]ExecutionTaskItemDTO)
	for _, item := range execution.Items {
		if item.ParentDemandID == "" {
			continue
		}
		byDemand[item.ParentDemandID] = append(byDemand[item.ParentDemandID], item)
	}

	result := StrongestBrainDeliveryEvidence{
		TotalRequirements: schedule.Summary.Total,
		RecentEvidence:    buildEvidenceDigest(logs),
	}
	missingCounts := map[string]int{}
	totalCompleteness := 0
	for _, item := range schedule.Items {
		requirement := buildRequirementEvidence(item, byDemand[item.DemandID])
		result.RequirementCompleteness = append(result.RequirementCompleteness, requirement)
		totalCompleteness += requirement.Completeness
		if requirement.Completeness >= 80 && requirement.ChainStatus == "complete" {
			result.CompleteChains++
		} else {
			result.IncompleteChains++
		}
		for _, link := range requirement.MissingLinks {
			missingCounts[link]++
		}
		for _, child := range byDemand[item.DemandID] {
			if child.RiskLabel == "完成无证据" {
				result.MissingCodeEvidence++
			}
			if child.RiskLabel == "状态不一致" {
				result.MergedButStatusOpen++
			}
		}
	}
	if result.TotalRequirements > 0 {
		result.AverageCompleteness = totalCompleteness / result.TotalRequirements
	}
	result.JiraCodeMismatch = result.MergedButStatusOpen + result.MissingCodeEvidence
	result.TopMissingLinks = topMissingLinks(missingCounts, 5)
	sort.SliceStable(result.RequirementCompleteness, func(i, j int) bool {
		if result.RequirementCompleteness[i].Completeness != result.RequirementCompleteness[j].Completeness {
			return result.RequirementCompleteness[i].Completeness < result.RequirementCompleteness[j].Completeness
		}
		return result.RequirementCompleteness[i].DemandID < result.RequirementCompleteness[j].DemandID
	})
	if len(result.RequirementCompleteness) > 8 {
		result.RequirementCompleteness = result.RequirementCompleteness[:8]
	}
	return result
}

func buildRequirementEvidence(item ScheduleItemDTO, children []ExecutionTaskItemDTO) StrongestBrainRequirementEvidence {
	score := 0
	missing := make([]string, 0)
	refs := make([]string, 0)
	if item.Scheduled {
		score += 20
		refs = append(refs, "排期完整")
	} else {
		missing = append(missing, "排期")
	}
	if item.Branch != "" && item.Branch != "-" {
		score += 15
		refs = append(refs, "分支 "+item.Branch)
	} else {
		missing = append(missing, "开发分支")
	}
	if item.DueDate != "" {
		score += 10
		refs = append(refs, "截止日 "+item.DueDate)
	}
	if item.SubtaskTotal > 0 {
		score += 15
		refs = append(refs, fmt.Sprintf("影子任务 %d 个", item.SubtaskTotal))
	} else {
		missing = append(missing, "拆解任务")
	}

	commitCount := 0
	mrCount := 0
	mergedCount := 0
	for _, child := range children {
		commitCount += child.CommitCount
		mrCount += child.MRCount
		mergedCount += child.MergedMRCount
	}
	if commitCount > 0 {
		score += 15
		refs = append(refs, fmt.Sprintf("commit %d 个", commitCount))
	} else {
		missing = append(missing, "commit")
	}
	if mrCount > 0 || item.MRURL != "" {
		score += 15
		refs = append(refs, fmt.Sprintf("MR %d 个", mrCount))
	} else {
		missing = append(missing, "MR")
	}
	if mergedCount > 0 || item.Status == "done" {
		score += 10
	}
	if score > 100 {
		score = 100
	}
	status := "incomplete"
	if score >= 80 && len(missing) == 0 {
		status = "complete"
	} else if score >= 50 {
		status = "partial"
	}
	next := "补齐证据链缺口后再进入完成复盘"
	if len(missing) > 0 {
		next = "优先补齐：" + strings.Join(missing, "、")
	}
	if status == "complete" {
		next = "证据链完整，可进入验收或复盘"
	}
	return StrongestBrainRequirementEvidence{
		DemandID:        item.DemandID,
		Title:           item.Title,
		Assignee:        item.Assignee,
		Project:         firstNonEmpty(item.ProjectKey, item.Repo, "未归属"),
		ChainStatus:     status,
		Completeness:    score,
		MissingLinks:    missing,
		EvidenceRefs:    refs,
		RecommendedNext: next,
	}
}

func buildStrongestBrainDeliveryExceptions(items []StrongestBrainDecisionItem) StrongestBrainDeliveryExceptions {
	result := StrongestBrainDeliveryExceptions{Total: len(items)}
	for _, item := range items {
		severity := decisionSeverity(item)
		switch severity {
		case "P0":
			result.P0++
		case "P1":
			result.P1++
		default:
			result.P2++
		}
		if len(result.Items) < 8 {
			result.Items = append(result.Items, StrongestBrainDeliveryException{
				ID:                item.ID,
				Type:              item.RiskType,
				Severity:          severity,
				Title:             item.Title,
				Reason:            item.Problem,
				DecisionOwner:     firstNonEmpty(item.Assignee, "未指定"),
				Deadline:          decisionDeadline(item),
				RecommendedAction: item.SuggestedAction,
				EvidenceRefs:      item.Evidence,
				Source:            item.Source,
			})
		}
	}
	return result
}

func buildStrongestBrainDeliveryWeeklyDecisions(items []StrongestBrainDecisionItem, now time.Time) StrongestBrainWeeklyDecisionSummary {
	result := StrongestBrainWeeklyDecisionSummary{}
	for _, item := range items {
		if item.RiskLevel == "critical" || item.RiskType == "context_missing" || strings.Contains(item.RiskType, "mismatch") {
			result.MustDecide++
		}
		if item.RiskType == "context_missing" || item.RiskType == "missing_schedule" || strings.Contains(item.Problem, "缺少") {
			result.DecisionDebt++
		}
		deadline := decisionDeadline(item)
		if deadline == "" {
			deadline = formatOptionalDatePtr(startOfDay(now).AddDate(0, 0, 5))
		}
		if len(result.Items) < 6 {
			result.Items = append(result.Items, StrongestBrainWeeklyDecisionItem{
				ID:                item.ID,
				Question:          deliveryWeeklyDecisionQuestion(item),
				WhyNow:            item.Problem,
				Options:           deliveryWeeklyDecisionOptions(item),
				RecommendedAction: item.SuggestedAction,
				DecisionOwner:     firstNonEmpty(item.Assignee, "未指定"),
				Deadline:          deadline,
				EvidenceRefs:      item.Evidence,
			})
		}
	}
	result.Total = len(result.Items)
	result.ThisWeek = result.Total
	return result
}

func buildStrongestBrainDeliverySchedule(schedule ScheduleResponseDTO, riskCalendar ScheduleRiskCalendarResponseDTO) StrongestBrainDeliverySchedule {
	upcoming := riskCalendar.Events
	if len(upcoming) > 6 {
		upcoming = upcoming[:6]
	}
	return StrongestBrainDeliverySchedule{
		Total:           schedule.Summary.Total,
		Scheduled:       schedule.Summary.Scheduled,
		Unscheduled:     schedule.Summary.Unscheduled,
		Overdue:         schedule.Summary.Overdue,
		DueSoon:         schedule.Summary.DueSoon,
		Stale:           schedule.Summary.Stale,
		HighRisk:        riskCalendar.Summary.HighRisk,
		RiskCalendarURL: "/api/schedule/risk-calendar",
		Upcoming:        upcoming,
	}
}

func buildStrongestBrainAITraceSummary(limit int) StrongestBrainAITraceSummary {
	total, traceable := aiTraceCounts()
	items := buildStrongestBrainAITraceItems(limit)
	summary := StrongestBrainAITraceSummary{
		TotalOutputs:        total,
		TraceableOutputs:    traceable,
		TraceabilityPercent: percent(traceable, total),
		Latest:              items,
	}
	var scoreSum int
	for _, item := range items {
		scoreSum += item.ReadinessScore
	}
	if len(items) > 0 {
		summary.AverageReadiness = scoreSum / len(items)
	}
	return summary
}

func buildStrongestBrainAITraceItems(limit int) []StrongestBrainAITraceItem {
	if limit <= 0 {
		limit = 20
	}
	var archives []db.DeconstructArchive
	if err := db.DB.Order("created_at desc, id desc").Limit(limit).Find(&archives).Error; err != nil {
		return []StrongestBrainAITraceItem{}
	}
	packIDs := make([]uint, 0, len(archives))
	for _, archive := range archives {
		if archive.ContextPackID > 0 {
			packIDs = append(packIDs, archive.ContextPackID)
		}
	}
	packs := map[uint]db.ContextPack{}
	if len(packIDs) > 0 {
		var rows []db.ContextPack
		if err := db.DB.Where("id IN ?", packIDs).Find(&rows).Error; err == nil {
			for _, row := range rows {
				packs[row.ID] = row
			}
		}
	}

	items := make([]StrongestBrainAITraceItem, 0, len(archives))
	for _, archive := range archives {
		item := aiTraceItemFromArchive(archive, packs[archive.ContextPackID])
		items = append(items, item)
	}
	return items
}

func aiTraceItemFromArchive(archive db.DeconstructArchive, pack db.ContextPack) StrongestBrainAITraceItem {
	analysis := DeconstructAnalysis{}
	_ = json.Unmarshal([]byte(archive.AnalysisJSON), &analysis)
	normalizeDeconstructAnalysisLists(&analysis)
	model := strings.TrimSpace(pack.Model)
	if model == "" {
		model = "unknown"
	}
	promptVersion := strings.TrimSpace(pack.PromptTemplateVersion)
	if promptVersion == "" {
		promptVersion = contextPackTemplateV1
	}
	return StrongestBrainAITraceItem{
		ID:                    archive.ID,
		DemandID:              strings.TrimSpace(archive.DemandID),
		TaskGroupID:           strings.TrimSpace(archive.TaskGroupID),
		ContextPackID:         archive.ContextPackID,
		ContextPackSummary:    strings.TrimSpace(pack.Summary),
		InputSummary:          compactText(archive.InputText, 120),
		Model:                 model,
		PromptTemplateVersion: promptVersion,
		RuleVersion:           "readiness-v1",
		Confidence:            archive.Confidence,
		ReadinessScore:        archive.CompletenessScore,
		MissingQuestions:      readinessQuestions(analysis),
		AcceptanceCriteria:    firstNStrings(analysis.AcceptanceCriteria, 5),
		RiskFlags:             firstNStrings(analysis.Risks, 5),
		HumanFeedback:         "pending",
		CreatedAt:             formatDateTime(archive.CreatedAt),
	}
}

func buildStrongestBrainOverrideSummary(limit int, now time.Time) StrongestBrainOverrideSummary {
	var events []db.DecisionEvent
	query := db.DB.Where("action LIKE ?", "override_%").Order("created_at desc, id desc").Limit(limit)
	if err := query.Find(&events).Error; err != nil {
		return StrongestBrainOverrideSummary{AuditTrailURL: "/api/strongest-brain/override-audit"}
	}
	summary := StrongestBrainOverrideSummary{
		Recent:        len(events),
		AuditTrailURL: "/api/strongest-brain/override-audit",
		Latest:        make([]StrongestBrainOverrideLog, 0, len(events)),
	}
	for _, event := range events {
		if now.Sub(event.CreatedAt) <= 24*time.Hour {
			summary.Protected++
		}
		if strings.TrimSpace(event.Reason) == "" {
			summary.NeedsReview++
		}
		summary.Latest = append(summary.Latest, StrongestBrainOverrideLog{
			ID:               event.ID,
			TaskID:           event.TaskID,
			Actor:            event.Actor,
			Action:           event.Action,
			OriginalValue:    event.OldValue,
			OverrideValue:    event.NewValue,
			Reason:           event.Reason,
			ProtectionWindow: "24h local override guard",
			RollbackHint:     "使用审计记录中的 original_value 手动恢复，或在调停工作台接受外部同步",
			CreatedAt:        formatDateTime(event.CreatedAt),
		})
	}
	return summary
}

func buildStrongestBrainAuthorizationSummary(now time.Time) StrongestBrainAuthorizationSummary {
	since := now.AddDate(0, 0, -7)
	var total int64
	var denied int64
	var highRisk int64
	db.DB.Model(&userdb.AuthorizationAuditLog{}).Where("created_at >= ?", since).Count(&total)
	db.DB.Model(&userdb.AuthorizationAuditLog{}).Where("created_at >= ? AND allowed = ?", since, false).Count(&denied)
	db.DB.Model(&userdb.AuthorizationAuditLog{}).Where("created_at >= ? AND allowed = ? AND risk_level = ?", since, true, "high").Count(&highRisk)
	return StrongestBrainAuthorizationSummary{
		ExplainPanelAvailable: true,
		Endpoint:              "/api/authz/explain",
		RecentDecisions:       int(total),
		RecentDenials:         int(denied),
		HighRiskGrants:        int(highRisk),
	}
}

func buildStrongestBrainDeliveryHealth(evidence StrongestBrainDeliveryEvidence, exceptions StrongestBrainDeliveryExceptions, weekly StrongestBrainWeeklyDecisionSummary, ai StrongestBrainAITraceSummary, calendar ScheduleRiskCalendarResponseDTO) StrongestBrainDeliveryHealth {
	score := 100
	score -= exceptions.P0 * 16
	score -= exceptions.P1 * 7
	score -= calendar.Summary.HighRisk * 4
	score -= (100 - evidence.AverageCompleteness) / 3
	score -= (100 - ai.TraceabilityPercent) / 5
	if score < 0 {
		score = 0
	}
	label := "SAFE"
	if score < 60 || exceptions.P0 > 0 {
		label = "CRITICAL"
	} else if score < 80 || exceptions.P1 > 0 {
		label = "ATTENTION"
	}
	signals := []string{
		fmt.Sprintf("证据链完整度 %d%%", evidence.AverageCompleteness),
		fmt.Sprintf("开放异常 %d 个", exceptions.Total),
		fmt.Sprintf("本周决策 %d 个", weekly.Total),
		fmt.Sprintf("AI 可追溯率 %d%%", ai.TraceabilityPercent),
	}
	return StrongestBrainDeliveryHealth{
		Score:                score,
		Label:                label,
		EvidenceCompleteness: evidence.AverageCompleteness,
		OpenExceptions:       exceptions.Total,
		WeeklyDecisionCount:  weekly.Total,
		AITraceability:       ai.TraceabilityPercent,
		ScheduleHighRisk:     calendar.Summary.HighRisk,
		Signals:              signals,
	}
}

func strongestBrainEntryPoints() []StrongestBrainEntryPoint {
	return []StrongestBrainEntryPoint{
		{Key: "evidence_chain", Label: "需求证据链", Description: "查看需求到 MR、commit、CI、部署和验收的缺口", URL: "/api/strongest-brain/evidence-chain?task_id={id}", Status: "available"},
		{Key: "risk_calendar", Label: "风险日历", Description: "查看截止日、停滞、临期和二次延期风险", URL: "/api/schedule/risk-calendar", Status: "available"},
		{Key: "ai_replay", Label: "AI 回放", Description: "查看 context_pack_id、语料版本、置信度和需求澄清结果", URL: "/api/strongest-brain/ai-traces", Status: "available"},
		{Key: "override_audit", Label: "Override 审计", Description: "查看人工调停、保护窗口和回滚线索", URL: "/api/strongest-brain/override-audit", Status: "available"},
		{Key: "permission_explain", Label: "权限解释", Description: "解释为什么用户能或不能执行某个操作", URL: "/api/authz/explain", Status: "available"},
	}
}

func decisionSeverity(item StrongestBrainDecisionItem) string {
	if item.RiskLevel == "critical" || item.RiskType == "semantic_evidence_review" {
		return "P0"
	}
	if item.RiskLevel == "warning" || item.RiskType == "context_missing" {
		return "P1"
	}
	return "P2"
}

func decisionDeadline(item StrongestBrainDecisionItem) string {
	for _, evidence := range item.Evidence {
		if strings.HasPrefix(evidence, "截止日 ") {
			return strings.TrimSpace(strings.TrimPrefix(evidence, "截止日 "))
		}
	}
	return ""
}

func deliveryWeeklyDecisionQuestion(item StrongestBrainDecisionItem) string {
	switch item.RiskType {
	case "overdue":
		return fmt.Sprintf("%s 是否延期、拆分或转派？", item.Title)
	case "due_soon":
		return fmt.Sprintf("%s 是否能按当前截止日交付？", item.Title)
	case "missing_schedule":
		return fmt.Sprintf("%s 是否进入本周排期？", item.Title)
	case "context_missing":
		return "系统设计语料库是否先补齐再继续需求解构？"
	default:
		return fmt.Sprintf("%s 需要谁拍板下一步？", item.Title)
	}
}

func deliveryWeeklyDecisionOptions(item StrongestBrainDecisionItem) []string {
	switch item.RiskType {
	case "overdue":
		return []string{"延期并记录原因", "拆分范围", "转派协助"}
	case "missing_schedule":
		return []string{"补齐分支和截止日", "退回需求澄清", "挂起"}
	case "context_missing":
		return []string{"补齐最小事实卡", "继续使用 legacy 配置", "暂停 AI 解构"}
	default:
		return []string{"继续推进", "升级会议", "人工调停"}
	}
}

func aiTraceCounts() (int, int) {
	var total int64
	var traceable int64
	db.DB.Model(&db.DeconstructArchive{}).Count(&total)
	db.DB.Model(&db.DeconstructArchive{}).Where("context_pack_id > 0").Count(&traceable)
	return int(total), int(traceable)
}

func normalizeDeconstructAnalysisLists(analysis *DeconstructAnalysis) {
	if analysis == nil {
		return
	}
	analysis.MissingInfo = normalizeStringList(analysis.MissingInfo)
	analysis.Risks = normalizeStringList(analysis.Risks)
	analysis.AcceptanceCriteria = normalizeStringList(analysis.AcceptanceCriteria)
}

func readinessQuestions(analysis DeconstructAnalysis) []string {
	questions := make([]string, 0)
	for _, missing := range analysis.MissingInfo {
		questions = append(questions, "必须回答："+missing)
	}
	for _, question := range analysis.MeetingQuestions {
		questions = append(questions, "周会确认："+question)
	}
	if len(questions) == 0 && analysis.CompletenessScore < 80 {
		questions = append(questions, "必须补充验收标准、边界条件或依赖信息")
	}
	return firstNStrings(questions, 8)
}

func topMissingLinks(counts map[string]int, limit int) []string {
	type pair struct {
		key   string
		count int
	}
	pairs := make([]pair, 0, len(counts))
	for key, count := range counts {
		pairs = append(pairs, pair{key: key, count: count})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].count != pairs[j].count {
			return pairs[i].count > pairs[j].count
		}
		return pairs[i].key < pairs[j].key
	})
	result := make([]string, 0, limit)
	for _, p := range pairs {
		if len(result) >= limit {
			break
		}
		result = append(result, fmt.Sprintf("%s x%d", p.key, p.count))
	}
	return result
}

func percent(part int, total int) int {
	if total <= 0 {
		return 100
	}
	value := int(float64(part) * 100 / float64(total))
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func firstNStrings(values []string, limit int) []string {
	if values == nil {
		return []string{}
	}
	cleaned := normalizeStringList(values)
	if limit > 0 && len(cleaned) > limit {
		return cleaned[:limit]
	}
	return cleaned
}

func boundedQueryLimit(r *http.Request, defaultLimit int, maxLimit int) int {
	limit := defaultLimit
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	return limit
}

func formatOptionalDatePtr(t time.Time) string {
	return t.Format("2006-01-02")
}
