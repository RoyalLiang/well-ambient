package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
	"well-ambient/internal/telemetry"
)

type StrongestBrainDecisionQueueResponse struct {
	GeneratedAt      string                         `json:"generated_at"`
	Summary          StrongestBrainDecisionSummary  `json:"summary"`
	ExceptionSummary StrongestBrainExceptionSummary `json:"exception_summary"`
	WeeklyDecisions  []StrongestBrainWeeklyDecision `json:"weekly_decisions"`
	Items            []StrongestBrainDecisionItem   `json:"items"`
	Evidence         []StrongestBrainEvidenceDigest `json:"evidence"`
}

type StrongestBrainExceptionCenterResponse struct {
	GeneratedAt string                               `json:"generated_at"`
	Mode        string                               `json:"mode"`
	Summary     StrongestBrainExceptionCenterSummary `json:"summary"`
	Items       []StrongestBrainExceptionCenterItem  `json:"items"`
	Evidence    []StrongestBrainEvidenceDigest       `json:"evidence"`
}

type StrongestBrainExceptionCenterSummary struct {
	Total              int            `json:"total"`
	Open               int            `json:"open"`
	P0                 int            `json:"p0"`
	P1                 int            `json:"p1"`
	P2                 int            `json:"p2"`
	ByType             map[string]int `json:"by_type"`
	ByOwner            map[string]int `json:"by_owner"`
	EvidenceIncomplete int            `json:"evidence_incomplete"`
	StatusMismatch     int            `json:"status_mismatch"`
	MissingSchedule    int            `json:"missing_schedule"`
	StaleAfterSchedule int            `json:"stale_after_schedule"`
	DeadlineChainRisks int            `json:"deadline_chain_risks"`
}

type StrongestBrainExceptionCenterItem struct {
	ID                   string   `json:"id"`
	Type                 string   `json:"type"`
	Severity             string   `json:"severity"`
	Status               string   `json:"status"`
	Title                string   `json:"title"`
	Reason               string   `json:"reason"`
	DecisionOwner        string   `json:"decision_owner"`
	Deadline             string   `json:"deadline"`
	RecommendedAction    string   `json:"recommended_action"`
	ImpactScope          string   `json:"impact_scope"`
	Source               string   `json:"source"`
	EvidenceRefs         []string `json:"evidence_refs"`
	MissingLinks         []string `json:"missing_links"`
	CloseRequires        []string `json:"close_requires"`
	ChainStatus          string   `json:"chain_status"`
	EvidenceCompleteness int      `json:"evidence_completeness"`
}

type StrongestBrainWeeklyDecisionCenterResponse struct {
	GeneratedAt string                                    `json:"generated_at"`
	Mode        string                                    `json:"mode"`
	Summary     StrongestBrainWeeklyDecisionCenterSummary `json:"summary"`
	Items       []StrongestBrainWeeklyDecision            `json:"items"`
}

type StrongestBrainWeeklyDecisionCenterSummary struct {
	Total              int `json:"total"`
	MustDecide         int `json:"must_decide"`
	ThisWeek           int `json:"this_week"`
	DecisionDebt       int `json:"decision_debt"`
	Critical           int `json:"critical"`
	Warning            int `json:"warning"`
	EvidenceIncomplete int `json:"evidence_incomplete"`
	StatusMismatch     int `json:"status_mismatch"`
}

type StrongestBrainDecisionSummary struct {
	Total               int `json:"total"`
	Critical            int `json:"critical"`
	Warning             int `json:"warning"`
	Open                int `json:"open"`
	ScheduleRisks       int `json:"schedule_risks"`
	EvidenceRisks       int `json:"evidence_risks"`
	ContextGaps         int `json:"context_gaps"`
	EvidenceIncomplete  int `json:"evidence_incomplete"`
	StatusMismatch      int `json:"status_mismatch"`
	MissingSchedule     int `json:"missing_schedule"`
	StaleAfterSchedule  int `json:"stale_after_schedule"`
	DeadlineChainRisks  int `json:"deadline_chain_risks"`
	WeeklyDecisionCount int `json:"weekly_decision_count"`
}

type StrongestBrainDecisionItem struct {
	ID                   string   `json:"id"`
	TaskID               string   `json:"task_id"`
	Title                string   `json:"title"`
	Problem              string   `json:"problem"`
	Evidence             []string `json:"evidence"`
	EvidenceRefs         []string `json:"evidence_refs"`
	MissingLinks         []string `json:"missing_links"`
	SuggestedAction      string   `json:"suggested_action"`
	RecommendedAction    string   `json:"recommended_action"`
	DecisionOwner        string   `json:"decision_owner"`
	Deadline             string   `json:"deadline"`
	ImpactScope          string   `json:"impact_scope"`
	JumpLabel            string   `json:"jump_label"`
	JumpURL              string   `json:"jump_url"`
	RiskLevel            string   `json:"risk_level"`
	RiskType             string   `json:"risk_type"`
	Status               string   `json:"status"`
	Assignee             string   `json:"assignee"`
	Project              string   `json:"project"`
	IssueType            string   `json:"issue_type"`
	UpdatedAt            string   `json:"updated_at"`
	Source               string   `json:"source"`
	ChainStatus          string   `json:"chain_status"`
	EvidenceCompleteness int      `json:"evidence_completeness"`
	Rank                 int      `json:"rank"`
}

type StrongestBrainEvidenceDigest struct {
	TaskID               string   `json:"task_id"`
	TaskGroupID          string   `json:"task_group_id"`
	CommitCount          int      `json:"commit_count"`
	MRCount              int      `json:"mr_count"`
	LastEvidence         string   `json:"last_evidence"`
	Signals              []string `json:"signals"`
	EvidenceRefs         []string `json:"evidence_refs"`
	MissingLinks         []string `json:"missing_links"`
	ChainStatus          string   `json:"chain_status"`
	EvidenceCompleteness int      `json:"evidence_completeness"`
}

type StrongestBrainEvidenceChainResponse struct {
	GeneratedAt          string                     `json:"generated_at"`
	TaskID               string                     `json:"task_id"`
	TaskGroupID          string                     `json:"task_group_id"`
	Root                 *StrongestBrainChainTask   `json:"root,omitempty"`
	Related              []StrongestBrainChainTask  `json:"related"`
	Evidence             []StrongestBrainChainLog   `json:"evidence"`
	EvidenceRefs         []string                   `json:"evidence_refs"`
	MissingLinks         []string                   `json:"missing_links"`
	ChainStatus          string                     `json:"chain_status"`
	EvidenceCompleteness int                        `json:"evidence_completeness"`
	Summary              StrongestBrainChainSummary `json:"summary"`
}

type StrongestBrainChainTask struct {
	TaskID      string `json:"task_id"`
	Title       string `json:"title"`
	IssueType   string `json:"issue_type"`
	Status      string `json:"status"`
	Assignee    string `json:"assignee"`
	Repo        string `json:"repo"`
	Branch      string `json:"branch"`
	DueDate     string `json:"due_date,omitempty"`
	TaskGroupID string `json:"task_group_id"`
}

type StrongestBrainChainLog struct {
	ID        uint   `json:"id"`
	TaskID    string `json:"task_id"`
	Action    string `json:"action"`
	Repo      string `json:"repo"`
	Branch    string `json:"branch"`
	CommitID  string `json:"commit_id"`
	MRURL     string `json:"mr_url"`
	CreatedAt string `json:"created_at"`
}

type StrongestBrainChainSummary struct {
	RelatedTasks         int      `json:"related_tasks"`
	Commits              int      `json:"commits"`
	MergeRequests        int      `json:"merge_requests"`
	MergedMRs            int      `json:"merged_mrs"`
	Signals              []string `json:"signals"`
	MissingLinks         []string `json:"missing_links"`
	StatusMismatches     []string `json:"status_mismatches"`
	ChainStatus          string   `json:"chain_status"`
	EvidenceCompleteness int      `json:"evidence_completeness"`
}

type StrongestBrainExceptionSummary struct {
	Total              int            `json:"total"`
	Critical           int            `json:"critical"`
	Warning            int            `json:"warning"`
	Open               int            `json:"open"`
	ByType             map[string]int `json:"by_type"`
	ByOwner            map[string]int `json:"by_owner"`
	EvidenceIncomplete int            `json:"evidence_incomplete"`
	StatusMismatch     int            `json:"status_mismatch"`
	MissingSchedule    int            `json:"missing_schedule"`
	StaleAfterSchedule int            `json:"stale_after_schedule"`
	DeadlineChainRisks int            `json:"deadline_chain_risks"`
}

type StrongestBrainWeeklyDecision struct {
	ID                   string   `json:"id"`
	SourceItemID         string   `json:"source_item_id"`
	TaskID               string   `json:"task_id"`
	DecisionType         string   `json:"decision_type"`
	Question             string   `json:"question"`
	RiskLevel            string   `json:"risk_level"`
	DecisionOwner        string   `json:"decision_owner"`
	Deadline             string   `json:"deadline"`
	RecommendedAction    string   `json:"recommended_action"`
	Options              []string `json:"options"`
	EvidenceRefs         []string `json:"evidence_refs"`
	MissingLinks         []string `json:"missing_links"`
	ImpactScope          string   `json:"impact_scope"`
	ChainStatus          string   `json:"chain_status"`
	EvidenceCompleteness int      `json:"evidence_completeness"`
}

type AIIntentRequest struct {
	Text     string            `json:"text"`
	Messages []AIIntentMessage `json:"messages"`
	Mode     string            `json:"mode"`
}

type AIIntentMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIIntentResponse struct {
	Intent          string   `json:"intent"`
	IntentLabel     string   `json:"intent_label"`
	Confidence      float64  `json:"confidence"`
	Summary         string   `json:"summary"`
	SuggestedAction string   `json:"suggested_action"`
	MissingContext  []string `json:"missing_context"`
	NextQuestions   []string `json:"next_questions"`
	Facts           []string `json:"facts"`
	Inferences      []string `json:"inferences"`
	RoutedTo        string   `json:"routed_to"`
	Source          string   `json:"source"`
}

func (s *Server) handleGetStrongestBrainDecisionQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	now := time.Now()
	projectKeys, err := requestProjectPreferenceKeys(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to apply project preferences: %v", err), http.StatusInternalServerError)
		return
	}
	readModel, err := buildStrongestBrainDecisionSnapshot(now, projectKeys)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to build strongest brain decision snapshot: %v", err), http.StatusInternalServerError)
		return
	}

	response := StrongestBrainDecisionQueueResponse{
		GeneratedAt:      formatDateTime(now),
		Summary:          readModel.Summary,
		ExceptionSummary: readModel.ExceptionSummary,
		WeeklyDecisions:  readModel.WeeklyDecisions,
		Items:            readModel.Items,
		Evidence:         readModel.Evidence,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleGetStrongestBrainExceptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	now := time.Now()
	projectKeys, err := requestProjectPreferenceKeys(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to apply project preferences: %v", err), http.StatusInternalServerError)
		return
	}
	readModel, err := buildStrongestBrainDecisionSnapshot(now, projectKeys)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to build strongest brain exception snapshot: %v", err), http.StatusInternalServerError)
		return
	}
	items := buildStrongestBrainExceptionCenterItems(readModel.Items, boundedQueryLimit(r, 50, 200))

	response := StrongestBrainExceptionCenterResponse{
		GeneratedAt: formatDateTime(now),
		Mode:        "exceptions_only",
		Summary:     buildStrongestBrainExceptionCenterSummary(items),
		Items:       items,
		Evidence:    readModel.Evidence,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleGetStrongestBrainWeeklyDecisions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	now := time.Now()
	projectKeys, err := requestProjectPreferenceKeys(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to apply project preferences: %v", err), http.StatusInternalServerError)
		return
	}
	readModel, err := buildStrongestBrainDecisionSnapshot(now, projectKeys)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to build strongest brain weekly decisions: %v", err), http.StatusInternalServerError)
		return
	}
	limit := boundedQueryLimit(r, 50, 200)
	items := readModel.WeeklyDecisions
	if len(items) > limit {
		items = items[:limit]
	}

	response := StrongestBrainWeeklyDecisionCenterResponse{
		GeneratedAt: formatDateTime(now),
		Mode:        "decision_meeting",
		Summary:     buildStrongestBrainWeeklyDecisionCenterSummary(items),
		Items:       items,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleGetStrongestBrainEvidenceChain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	taskID := strings.TrimSpace(r.URL.Query().Get("task_id"))
	if taskID == "" {
		http.Error(w, "Bad Request: task_id is required", http.StatusBadRequest)
		return
	}
	allowed, err := requestCanAccessTask(r, taskID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to apply project preferences: %v", err), http.StatusInternalServerError)
		return
	}
	if !allowed {
		http.Error(w, fmt.Sprintf("Task %s not found", taskID), http.StatusNotFound)
		return
	}

	var root db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", taskID).First(&root).Error; err != nil {
		http.Error(w, fmt.Sprintf("Task %s not found", taskID), http.StatusNotFound)
		return
	}

	groupID := normalizedTaskGroupID(root.TaskGroupID)
	var related []db.TaskTelemetry
	if groupID != "" {
		if err := db.DB.Where("task_group_id = ? AND status != ?", groupID, "archived").Find(&related).Error; err != nil {
			http.Error(w, fmt.Sprintf("Failed to query related tasks: %v", err), http.StatusInternalServerError)
			return
		}
	} else {
		related = []db.TaskTelemetry{root}
	}

	taskIDs := make([]string, 0, len(related))
	for _, task := range related {
		if strings.TrimSpace(task.TaskID) != "" {
			taskIDs = append(taskIDs, strings.TrimSpace(task.TaskID))
		}
	}

	var logs []db.GitCommitLog
	if len(taskIDs) > 0 {
		if err := db.DB.Where("task_id IN ?", taskIDs).Order("created_at desc").Find(&logs).Error; err != nil {
			http.Error(w, fmt.Sprintf("Failed to query evidence chain: %v", err), http.StatusInternalServerError)
			return
		}
	}

	now := time.Now()
	rootDTO := chainTaskDTO(root)
	response := StrongestBrainEvidenceChainResponse{
		GeneratedAt: formatDateTime(now),
		TaskID:      taskID,
		TaskGroupID: groupID,
		Root:        &rootDTO,
		Related:     make([]StrongestBrainChainTask, 0, len(related)),
		Evidence:    make([]StrongestBrainChainLog, 0, len(logs)),
	}
	for _, task := range related {
		response.Related = append(response.Related, chainTaskDTO(task))
	}
	for _, log := range logs {
		response.Evidence = append(response.Evidence, StrongestBrainChainLog{
			ID:        log.ID,
			TaskID:    strings.TrimSpace(log.TaskID),
			Action:    strings.TrimSpace(log.Action),
			Repo:      strings.TrimSpace(log.Repo),
			Branch:    strings.TrimSpace(log.Branch),
			CommitID:  strings.TrimSpace(log.CommitID),
			MRURL:     strings.TrimSpace(log.MrURL),
			CreatedAt: formatDateTime(log.CreatedAt),
		})
		if log.Action == "git_push" {
			response.Summary.Commits++
		}
		if strings.HasPrefix(log.Action, "mr_") {
			response.Summary.MergeRequests++
		}
		if log.Action == "mr_merge" {
			response.Summary.MergedMRs++
		}
	}
	response.Summary.RelatedTasks = len(response.Related)
	profile := buildStrongestBrainEvidenceChainProfile(root, related, logs, now)
	response.EvidenceRefs = profile.EvidenceRefs
	response.MissingLinks = profile.MissingLinks
	response.ChainStatus = profile.ChainStatus
	response.EvidenceCompleteness = profile.EvidenceCompleteness
	response.Summary.MissingLinks = profile.MissingLinks
	response.Summary.StatusMismatches = profile.StatusMismatches
	response.Summary.ChainStatus = profile.ChainStatus
	response.Summary.EvidenceCompleteness = profile.EvidenceCompleteness
	response.Summary.Signals = evidenceChainSignals(response)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleAIIntentSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var req AIIntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request: invalid JSON", http.StatusBadRequest)
		return
	}
	text := intentRequestText(req)
	if strings.TrimSpace(text) == "" {
		http.Error(w, "Bad Request: text or messages are required", http.StatusBadRequest)
		return
	}

	response := analyzeIntentDeterministic(text)
	if strings.EqualFold(strings.TrimSpace(req.Mode), "summary") || strings.Contains(r.URL.Path, "summary") {
		response.Intent = "summary"
		response.IntentLabel = intentLabel("summary")
		response.SuggestedAction = suggestedActionForIntent("summary")
		response.MissingContext = missingContextForIntent("summary", text)
		response.NextQuestions = nextQuestionsForIntent("summary")
		response.Inferences = []string{intentInference("summary")}
		response.RoutedTo = routeForIntent("summary")
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func buildStrongestBrainScheduleSnapshot(now time.Time, projectScopes ...[]string) (ScheduleResponseDTO, error) {
	var projectKeys []string
	if len(projectScopes) > 0 {
		projectKeys = projectScopes[0]
	}
	var tasks []db.TaskTelemetry
	query := db.ApplyTaskProjectScope(db.DB.Where("status != ?", "archived"), projectKeys)
	if err := query.Find(&tasks).Error; err != nil {
		return ScheduleResponseDTO{}, err
	}
	var users []userdb.User
	if err := db.DB.Find(&users).Error; err != nil {
		return ScheduleResponseDTO{}, err
	}
	return buildScheduleResponse(tasks, users, now), nil
}

func buildStrongestBrainExecutionSnapshot(now time.Time, projectScopes ...[]string) (ExecutionTasksResponseDTO, []db.GitCommitLog, error) {
	var projectKeys []string
	if len(projectScopes) > 0 {
		projectKeys = projectScopes[0]
	}
	var tasks []db.TaskTelemetry
	query := db.ApplyTaskProjectScope(db.DB.Where("status != ?", "archived"), projectKeys)
	if err := query.Find(&tasks).Error; err != nil {
		return ExecutionTasksResponseDTO{}, nil, err
	}
	var taskIDs []string
	for _, task := range tasks {
		if strings.TrimSpace(task.TaskID) != "" {
			taskIDs = append(taskIDs, strings.TrimSpace(task.TaskID))
		}
	}
	var logs []db.GitCommitLog
	if len(taskIDs) > 0 {
		if err := db.DB.Where("task_id IN ?", taskIDs).Order("created_at desc").Find(&logs).Error; err != nil {
			return ExecutionTasksResponseDTO{}, nil, err
		}
	}
	var users []userdb.User
	if err := db.DB.Find(&users).Error; err != nil {
		return ExecutionTasksResponseDTO{}, nil, err
	}
	return buildExecutionTasksResponse(tasks, logs, users, now), logs, nil
}

type strongestBrainDecisionReadModel struct {
	Summary          StrongestBrainDecisionSummary
	ExceptionSummary StrongestBrainExceptionSummary
	WeeklyDecisions  []StrongestBrainWeeklyDecision
	Items            []StrongestBrainDecisionItem
	Evidence         []StrongestBrainEvidenceDigest
}

type strongestBrainEvidenceProfile struct {
	TaskID               string
	TaskGroupID          string
	IsDemand             bool
	HasIssue             bool
	HasParentDemand      bool
	HasBranch            bool
	HasDueDate           bool
	HasExecutionTask     bool
	HasCommit            bool
	HasMR                bool
	HasMergedMR          bool
	HasDoneStatus        bool
	StaleAfterSchedule   bool
	DeadlineChainRisk    bool
	DueDate              string
	LastEvidence         string
	EvidenceRefs         []string
	MissingLinks         []string
	StatusMismatches     []string
	ChainStatus          string
	EvidenceCompleteness int
}

type strongestBrainLogStats struct {
	HasCommit    bool
	HasMR        bool
	HasMergedMR  bool
	LastEvidence string
	EvidenceRefs []string
}

func buildStrongestBrainDecisionSnapshot(now time.Time, projectScopes ...[]string) (strongestBrainDecisionReadModel, error) {
	var projectKeys []string
	if len(projectScopes) > 0 {
		projectKeys = projectScopes[0]
	}
	schedule, err := buildStrongestBrainScheduleSnapshot(now, projectKeys)
	if err != nil {
		return strongestBrainDecisionReadModel{}, fmt.Errorf("build schedule snapshot: %w", err)
	}
	execution, logs, err := buildStrongestBrainExecutionSnapshot(now, projectKeys)
	if err != nil {
		return strongestBrainDecisionReadModel{}, fmt.Errorf("build execution snapshot: %w", err)
	}
	return buildStrongestBrainDecisionReadModel(schedule, execution, logs, now, projectKeys), nil
}

func buildStrongestBrainDecisionReadModel(schedule ScheduleResponseDTO, execution ExecutionTasksResponseDTO, logs []db.GitCommitLog, now time.Time, projectScopes ...[]string) strongestBrainDecisionReadModel {
	var projectKeys []string
	if len(projectScopes) > 0 {
		projectKeys = projectScopes[0]
	}
	profiles := buildStrongestBrainEvidenceProfiles(schedule, execution, logs)
	items := make([]StrongestBrainDecisionItem, 0)

	for _, item := range schedule.Items {
		if item.RiskLevel == "safe" || item.RiskLevel == "done" {
			continue
		}
		items = append(items, decisionFromScheduleItem(item))
	}
	for _, item := range execution.Items {
		if item.RiskLevel == "safe" || item.RiskLevel == "done" {
			continue
		}
		items = append(items, decisionFromExecutionItem(item))
	}
	items = append(items, strictEvidenceChainConsistencyDecisions(execution, now)...)
	items = append(items, semanticEvidenceReviewDecisions(now, projectKeys)...)
	items = append(items, contextGapDecisions(now)...)

	items = enrichStrongestBrainDecisionItems(items, profiles, now)
	items = dedupeStrongestBrainDecisionItems(items)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Rank != items[j].Rank {
			return items[i].Rank > items[j].Rank
		}
		return items[i].UpdatedAt > items[j].UpdatedAt
	})

	weeklyDecisions := buildStrongestBrainWeeklyDecisions(items)
	summary := buildStrongestBrainDecisionSummary(items, len(weeklyDecisions))
	return strongestBrainDecisionReadModel{
		Summary:          summary,
		ExceptionSummary: buildStrongestBrainExceptionSummary(items),
		WeeklyDecisions:  weeklyDecisions,
		Items:            items,
		Evidence:         buildEvidenceDigest(logs, profiles),
	}
}

func buildStrongestBrainExceptionCenterItems(items []StrongestBrainDecisionItem, limit int) []StrongestBrainExceptionCenterItem {
	if limit <= 0 {
		limit = 50
	}
	result := make([]StrongestBrainExceptionCenterItem, 0, len(items))
	for _, item := range items {
		status := strings.TrimSpace(item.Status)
		if status == "" {
			status = "open"
		}
		result = append(result, StrongestBrainExceptionCenterItem{
			ID:                   item.ID,
			Type:                 item.RiskType,
			Severity:             decisionSeverity(item),
			Status:               status,
			Title:                item.Title,
			Reason:               item.Problem,
			DecisionOwner:        firstNonEmpty(item.DecisionOwner, item.Assignee, "未指定"),
			Deadline:             firstNonEmpty(item.Deadline, decisionDeadline(item)),
			RecommendedAction:    firstNonEmpty(item.RecommendedAction, item.SuggestedAction),
			ImpactScope:          item.ImpactScope,
			Source:               item.Source,
			EvidenceRefs:         firstNStrings(firstNonEmptyStringSlice(item.EvidenceRefs, item.Evidence), 8),
			MissingLinks:         firstNStrings(item.MissingLinks, 8),
			CloseRequires:        strongestBrainExceptionCloseRequirements(item),
			ChainStatus:          item.ChainStatus,
			EvidenceCompleteness: item.EvidenceCompleteness,
		})
		if len(result) >= limit {
			break
		}
	}
	return result
}

func buildStrongestBrainExceptionCenterSummary(items []StrongestBrainExceptionCenterItem) StrongestBrainExceptionCenterSummary {
	summary := StrongestBrainExceptionCenterSummary{
		Total:   len(items),
		ByType:  map[string]int{},
		ByOwner: map[string]int{},
	}
	for _, item := range items {
		if item.Status == "" || item.Status == "open" {
			summary.Open++
		}
		switch item.Severity {
		case "P0":
			summary.P0++
		case "P1":
			summary.P1++
		default:
			summary.P2++
		}
		summary.ByType[firstNonEmpty(item.Type, "unknown")]++
		summary.ByOwner[firstNonEmpty(item.DecisionOwner, "未指定")]++
		if len(item.MissingLinks) > 0 || item.EvidenceCompleteness < 80 {
			summary.EvidenceIncomplete++
		}
		if strings.Contains(item.Type, "mismatch") {
			summary.StatusMismatch++
		}
		if item.Type == "missing_schedule" {
			summary.MissingSchedule++
		}
		if item.Type == "stale_after_schedule" {
			summary.StaleAfterSchedule++
		}
		if item.Type == "due_soon" || item.Type == "overdue" {
			summary.DeadlineChainRisks++
		}
	}
	return summary
}

func buildStrongestBrainWeeklyDecisionCenterSummary(items []StrongestBrainWeeklyDecision) StrongestBrainWeeklyDecisionCenterSummary {
	summary := StrongestBrainWeeklyDecisionCenterSummary{Total: len(items), ThisWeek: len(items)}
	for _, item := range items {
		if item.RiskLevel == "critical" {
			summary.Critical++
			summary.MustDecide++
		}
		if item.RiskLevel == "warning" {
			summary.Warning++
		}
		if len(item.MissingLinks) > 0 || item.EvidenceCompleteness < 80 {
			summary.EvidenceIncomplete++
			summary.DecisionDebt++
		}
		if strings.Contains(item.DecisionType, "status") || strings.Contains(item.DecisionType, "mismatch") {
			summary.StatusMismatch++
			summary.MustDecide++
		}
		if item.DecisionType == "context_clarification" || strings.Contains(item.Question, "补充") {
			summary.DecisionDebt++
		}
	}
	if summary.MustDecide > summary.Total {
		summary.MustDecide = summary.Total
	}
	return summary
}

func strongestBrainExceptionCloseRequirements(item StrongestBrainDecisionItem) []string {
	requirements := []string{"记录处理结果"}
	if len(item.MissingLinks) > 0 {
		requirements = append(requirements, "补齐缺失证据："+strings.Join(item.MissingLinks, "、"))
	}
	switch item.RiskType {
	case "missing_schedule":
		requirements = append(requirements, "绑定排期负责人和截止日")
	case "due_soon", "overdue":
		requirements = append(requirements, "确认延期、拆分、转派或范围调整结论")
	case "stale_after_schedule":
		requirements = append(requirements, "绑定最新 Git、MR、Jira 或人工调停证据")
	case "evidence_missing":
		requirements = append(requirements, "补充 MR、commit、部署或验收记录")
	case "status_mismatch":
		requirements = append(requirements, "完成 Jira 与代码状态对账")
	case "context_missing":
		requirements = append(requirements, "补齐需求澄清问题")
	}
	return requirements
}

func firstNonEmptyStringSlice(candidates ...[]string) []string {
	for _, candidate := range candidates {
		if len(candidate) > 0 {
			return candidate
		}
	}
	return []string{}
}

func buildStrongestBrainEvidenceProfiles(schedule ScheduleResponseDTO, execution ExecutionTasksResponseDTO, logs []db.GitCommitLog) map[string]strongestBrainEvidenceProfile {
	logStats := strongestBrainLogStatsByTask(logs)
	profiles := make(map[string]*strongestBrainEvidenceProfile)

	for _, item := range schedule.Items {
		taskID := strings.TrimSpace(item.DemandID)
		if taskID == "" {
			continue
		}
		profile := ensureStrongestBrainProfile(profiles, taskID)
		profile.IsDemand = true
		profile.HasIssue = true
		profile.TaskGroupID = firstNonEmpty(profile.TaskGroupID, item.TaskGroupID)
		profile.HasBranch = profile.HasBranch || hasScheduleBranch(item.Branch)
		profile.HasDueDate = profile.HasDueDate || strings.TrimSpace(item.DueDate) != ""
		profile.HasExecutionTask = profile.HasExecutionTask || item.SubtaskTotal > 0
		profile.HasDoneStatus = profile.HasDoneStatus || strings.EqualFold(strings.TrimSpace(item.Status), "done")
		profile.StaleAfterSchedule = profile.StaleAfterSchedule || item.RiskLevel == "stale"
		profile.DeadlineChainRisk = profile.DeadlineChainRisk || item.RiskLevel == "due_soon" || item.RiskLevel == "overdue"
		profile.DueDate = firstNonEmpty(profile.DueDate, item.DueDate)
		profile.EvidenceRefs = append(profile.EvidenceRefs, strongestBrainScheduleEvidenceRefs(item)...)
		applyStrongestBrainLogStats(profile, logStats[taskID])
	}

	for _, item := range execution.Items {
		taskID := strings.TrimSpace(item.TaskID)
		if taskID == "" {
			continue
		}
		profile := ensureStrongestBrainProfile(profiles, taskID)
		profile.HasIssue = true
		profile.HasParentDemand = strings.TrimSpace(item.ParentDemandID) != ""
		profile.TaskGroupID = firstNonEmpty(profile.TaskGroupID, item.TaskGroupID)
		profile.HasBranch = profile.HasBranch || hasScheduleBranch(item.Branch)
		profile.HasExecutionTask = true
		profile.HasCommit = profile.HasCommit || item.CommitCount > 0 || firstNonEmpty(item.LastCommit) != ""
		profile.HasMR = profile.HasMR || item.MRCount > 0 || strings.TrimSpace(item.MRURL) != "" || item.MRIID > 0
		profile.HasMergedMR = profile.HasMergedMR || item.MergedMRCount > 0
		profile.HasDoneStatus = profile.HasDoneStatus || strings.EqualFold(strings.TrimSpace(item.Status), "done")
		profile.StaleAfterSchedule = profile.StaleAfterSchedule || executionItemIndicatesStaleAfterSchedule(item)
		profile.EvidenceRefs = append(profile.EvidenceRefs, strongestBrainExecutionEvidenceRefs(item)...)
		applyStrongestBrainLogStats(profile, logStats[taskID])

		parentID := strings.TrimSpace(item.ParentDemandID)
		if parentID == "" {
			continue
		}
		parent := ensureStrongestBrainProfile(profiles, parentID)
		parent.HasExecutionTask = true
		parent.HasCommit = parent.HasCommit || profile.HasCommit
		parent.HasMR = parent.HasMR || profile.HasMR
		parent.HasMergedMR = parent.HasMergedMR || profile.HasMergedMR
		parent.StaleAfterSchedule = parent.StaleAfterSchedule || profile.StaleAfterSchedule
		parent.LastEvidence = latestFormattedTime(parent.LastEvidence, profile.LastEvidence)
		parent.EvidenceRefs = append(parent.EvidenceRefs, "task:"+taskID)
		parent.EvidenceRefs = append(parent.EvidenceRefs, strongestBrainExecutionEvidenceRefs(item)...)
		if strings.EqualFold(strings.TrimSpace(item.Status), "done") && !executionItemHasCodeResultEvidence(item) {
			parent.StatusMismatches = append(parent.StatusMismatches, "child_completed_without_commit_or_mr:"+taskID)
		}
		if item.MergedMRCount > 0 && !strings.EqualFold(strings.TrimSpace(item.Status), "done") {
			parent.StatusMismatches = append(parent.StatusMismatches, "child_merged_mr_status_not_done:"+taskID)
		}
		if executionItemIndicatesStaleAfterSchedule(item) {
			parent.StatusMismatches = append(parent.StatusMismatches, "child_scheduled_without_progress:"+taskID)
		}
	}

	finalized := make(map[string]strongestBrainEvidenceProfile, len(profiles))
	for key, profile := range profiles {
		finalized[key] = finalizeStrongestBrainEvidenceProfile(*profile)
	}
	return finalized
}

func ensureStrongestBrainProfile(profiles map[string]*strongestBrainEvidenceProfile, taskID string) *strongestBrainEvidenceProfile {
	taskID = strings.TrimSpace(taskID)
	profile := profiles[taskID]
	if profile == nil {
		profile = &strongestBrainEvidenceProfile{
			TaskID:       taskID,
			HasIssue:     taskID != "",
			EvidenceRefs: []string{"task:" + taskID},
		}
		profiles[taskID] = profile
	}
	return profile
}

func strongestBrainLogStatsByTask(logs []db.GitCommitLog) map[string]strongestBrainLogStats {
	statsByTask := make(map[string]strongestBrainLogStats)
	for _, log := range logs {
		taskID := strings.TrimSpace(log.TaskID)
		if taskID == "" {
			continue
		}
		stats := statsByTask[taskID]
		if log.Action == "git_push" {
			stats.HasCommit = true
		}
		if strings.HasPrefix(log.Action, "mr_") {
			stats.HasMR = true
		}
		if log.Action == "mr_merge" {
			stats.HasMergedMR = true
		}
		createdAt := formatDateTime(log.CreatedAt)
		stats.LastEvidence = latestFormattedTime(stats.LastEvidence, createdAt)
		stats.EvidenceRefs = append(stats.EvidenceRefs, strongestBrainLogEvidenceRef(log))
		statsByTask[taskID] = stats
	}
	return statsByTask
}

func applyStrongestBrainLogStats(profile *strongestBrainEvidenceProfile, stats strongestBrainLogStats) {
	profile.HasCommit = profile.HasCommit || stats.HasCommit
	profile.HasMR = profile.HasMR || stats.HasMR
	profile.HasMergedMR = profile.HasMergedMR || stats.HasMergedMR
	profile.LastEvidence = latestFormattedTime(profile.LastEvidence, stats.LastEvidence)
	profile.EvidenceRefs = append(profile.EvidenceRefs, stats.EvidenceRefs...)
}

func finalizeStrongestBrainEvidenceProfile(profile strongestBrainEvidenceProfile) strongestBrainEvidenceProfile {
	missing := append([]string{}, profile.MissingLinks...)
	if profile.IsDemand {
		if !profile.HasBranch {
			missing = append(missing, "branch")
		}
		if !profile.HasDueDate {
			missing = append(missing, "deadline")
		}
		if !profile.HasExecutionTask {
			missing = append(missing, "execution_task")
		}
	} else {
		if !profile.HasParentDemand {
			missing = append(missing, "parent_demand")
		}
		if !profile.HasBranch && !profile.HasCommit && !profile.HasMR {
			missing = append(missing, "branch")
		}
	}
	if !profile.HasCommit {
		missing = append(missing, "commit")
	}
	if !profile.HasMR {
		missing = append(missing, "merge_request")
	}
	if profile.HasDoneStatus && (!profile.HasCommit || !profile.HasMR) {
		missing = append(missing, "completion_evidence")
		profile.StatusMismatches = append(profile.StatusMismatches, "completed_without_commit_or_mr")
	}
	if profile.HasMergedMR && !profile.HasDoneStatus {
		missing = append(missing, "status_update")
		profile.StatusMismatches = append(profile.StatusMismatches, "merged_mr_status_not_done")
	}
	if profile.StaleAfterSchedule {
		profile.StatusMismatches = append(profile.StatusMismatches, "scheduled_without_progress")
	}

	profile.MissingLinks = compactStrings(missing, 10)
	profile.StatusMismatches = compactStrings(profile.StatusMismatches, 10)
	profile.EvidenceRefs = compactStrings(profile.EvidenceRefs, 12)
	profile.EvidenceCompleteness = strongestBrainEvidenceCompleteness(profile)
	if profile.DeadlineChainRisk && profile.EvidenceCompleteness < 80 {
		profile.StatusMismatches = compactStrings(append(profile.StatusMismatches, "deadline_at_risk_incomplete_chain"), 10)
	}
	profile.ChainStatus = strongestBrainChainStatus(profile)
	return profile
}

func strongestBrainEvidenceCompleteness(profile strongestBrainEvidenceProfile) int {
	score := 0
	if profile.HasIssue {
		score += 20
	}
	if profile.IsDemand {
		if profile.HasBranch {
			score += 10
		}
		if profile.HasDueDate {
			score += 10
		}
		if profile.HasExecutionTask {
			score += 15
		}
	} else {
		if profile.HasParentDemand {
			score += 15
		}
		if profile.HasBranch {
			score += 15
		}
	}
	if profile.HasCommit {
		score += 25
	}
	if profile.HasMR {
		score += 15
	}
	if profile.HasMergedMR || (profile.HasDoneStatus && profile.HasCommit && profile.HasMR) {
		score += 10
	}
	if score > 100 {
		return 100
	}
	return score
}

func strongestBrainChainStatus(profile strongestBrainEvidenceProfile) string {
	if len(profile.StatusMismatches) > 0 {
		return "mismatch"
	}
	if profile.DeadlineChainRisk && profile.EvidenceCompleteness < 80 {
		return "deadline_at_risk"
	}
	if profile.StaleAfterSchedule {
		return "stale"
	}
	if profile.EvidenceCompleteness >= 80 {
		return "complete"
	}
	if profile.EvidenceCompleteness >= 50 {
		return "partial"
	}
	return "incomplete"
}

func strictEvidenceChainConsistencyDecisions(execution ExecutionTasksResponseDTO, now time.Time) []StrongestBrainDecisionItem {
	items := make([]StrongestBrainDecisionItem, 0)
	for _, item := range execution.Items {
		if !strings.EqualFold(strings.TrimSpace(item.Status), "done") && !strings.EqualFold(strings.TrimSpace(item.ExecutionStatus), "done") {
			continue
		}
		if executionItemHasCodeResultEvidence(item) {
			continue
		}
		items = append(items, StrongestBrainDecisionItem{
			ID:              fmt.Sprintf("execution:%s:completed-without-code-result", item.TaskID),
			TaskID:          item.TaskID,
			Title:           item.Title,
			Problem:         "任务已完成，但缺少 commit 或 MR 结果证据",
			Evidence:        []string{fmt.Sprintf("执行状态 %s，主线状态 %s，commit %d，MR %d", item.ExecutionStatus, item.Status, item.CommitCount, item.MRCount), "分支只能证明进入排期，不能证明交付结果"},
			SuggestedAction: "要求负责人补齐 commit、MR 或验收证据；无法补齐时回退完成状态并记录原因",
			ImpactScope:     executionImpactScope(item),
			JumpLabel:       firstNonEmpty(item.ParentDemandID, item.TaskID),
			JumpURL:         item.MRURL,
			RiskLevel:       "critical",
			RiskType:        "evidence_missing",
			Status:          "open",
			Assignee:        item.Assignee,
			Project:         firstNonEmpty(item.Repo, "未归属"),
			IssueType:       item.IssueType,
			UpdatedAt:       firstNonEmpty(item.LastEvidenceAt, item.LastUpdate, formatDateTime(now)),
			Source:          "execution",
			Rank:            94,
		})
	}
	return items
}

func decisionFromScheduleItem(item ScheduleItemDTO) StrongestBrainDecisionItem {
	riskLevel := "warning"
	if item.RiskLevel == "overdue" {
		riskLevel = "critical"
	}
	riskType := item.RiskLevel
	if item.RiskLevel == "unscheduled" {
		riskType = "missing_schedule"
	} else if item.RiskLevel == "stale" {
		riskType = "stale_after_schedule"
	}
	return StrongestBrainDecisionItem{
		ID:              fmt.Sprintf("schedule:%s:%s", item.DemandID, riskType),
		TaskID:          item.DemandID,
		Title:           item.Title,
		Problem:         item.RiskReason,
		Evidence:        scheduleEvidenceLines(item),
		SuggestedAction: scheduleSuggestedAction(item),
		ImpactScope:     scheduleImpactScope(item),
		JumpLabel:       item.DemandID,
		RiskLevel:       riskLevel,
		RiskType:        riskType,
		Status:          decisionStatusFromLogs(item.RiskReason),
		Assignee:        item.Assignee,
		Project:         firstNonEmpty(item.ProjectKey, item.Repo, "未归属"),
		IssueType:       item.IssueType,
		UpdatedAt:       item.LastUpdate,
		Source:          "schedule",
		Rank:            item.RiskRank,
	}
}

func decisionFromExecutionItem(item ExecutionTaskItemDTO) StrongestBrainDecisionItem {
	riskLevel := "warning"
	if item.RiskLevel == "high" {
		riskLevel = "critical"
	}
	return StrongestBrainDecisionItem{
		ID:              fmt.Sprintf("execution:%s:%s", item.TaskID, item.RiskLevel),
		TaskID:          item.TaskID,
		Title:           item.Title,
		Problem:         item.RiskReason,
		Evidence:        executionEvidenceLines(item),
		SuggestedAction: executionSuggestedAction(item),
		ImpactScope:     executionImpactScope(item),
		JumpLabel:       firstNonEmpty(item.ParentDemandID, item.TaskID),
		JumpURL:         item.MRURL,
		RiskLevel:       riskLevel,
		RiskType:        executionDecisionRiskType(item),
		Status:          "open",
		Assignee:        item.Assignee,
		Project:         firstNonEmpty(item.Repo, "未归属"),
		IssueType:       item.IssueType,
		UpdatedAt:       firstNonEmpty(item.LastEvidenceAt, item.LastUpdate),
		Source:          "execution",
		Rank:            item.RiskRank,
	}
}

func contextGapDecisions(now time.Time) []StrongestBrainDecisionItem {
	var count int64
	db.DB.Model(&db.ContextFact{}).Where("status = ?", "active").Count(&count)
	if count > 0 {
		return nil
	}
	return []StrongestBrainDecisionItem{{
		ID:              "context:missing-active-facts",
		TaskID:          "AI-CONTEXT",
		Title:           "系统设计语料库缺少 active 资料",
		Problem:         "AI 需求解构缺少可审计的系统事实上下文，可能导致追问和估算漂移",
		Evidence:        []string{"context_facts active 数量为 0", "AI 将退回 legacy 配置或默认上下文"},
		SuggestedAction: "补充架构、流程、功能边界、估算口径四类最小事实卡，再预览 context pack",
		ImpactScope:     "影响新需求解构、意图澄清、估算复盘和周会摘要可信度",
		JumpLabel:       "AI Context",
		RiskLevel:       "warning",
		RiskType:        "context_missing",
		Status:          "open",
		Assignee:        "系统管理员",
		Project:         "global",
		IssueType:       "config",
		UpdatedAt:       formatDateTime(now),
		Source:          "context",
		Rank:            58,
	}}
}

func semanticEvidenceReviewDecisions(now time.Time, projectScopes ...[]string) []StrongestBrainDecisionItem {
	var projectKeys []string
	if len(projectScopes) > 0 {
		projectKeys = projectScopes[0]
	}
	var notifications []db.Notification
	if err := db.DB.
		Where("type = ?", telemetry.SemanticLinkerType).
		Order("created_at desc").
		Limit(80).
		Find(&notifications).Error; err != nil || len(notifications) == 0 {
		return nil
	}

	items := make([]StrongestBrainDecisionItem, 0)
	seenLogs := make(map[uint]bool)
	for _, notification := range notifications {
		taskID := strings.TrimSpace(notification.TaskID)
		if taskID == "" {
			continue
		}
		var task db.TaskTelemetry
		_ = db.DB.Where("task_id = ?", taskID).First(&task).Error
		if !db.TaskMatchesExplicitProjectScope(task.ProjectKey, taskID, projectKeys) {
			continue
		}

		var log db.GitCommitLog
		if err := db.DB.
			Where("task_id = ? AND action = ? AND created_at BETWEEN ? AND ?", taskID, "git_push", notification.CreatedAt.Add(-10*time.Minute), notification.CreatedAt.Add(10*time.Minute)).
			Order("created_at desc").
			First(&log).Error; err != nil || log.ID == 0 || seenLogs[log.ID] {
			continue
		}
		if !telemetry.IsWeakSemanticCommit(taskID, log) {
			continue
		}
		seenLogs[log.ID] = true

		shortCommit := strings.TrimSpace(log.CommitID)
		if len(shortCommit) > 8 {
			shortCommit = shortCommit[:8]
		}

		items = append(items, StrongestBrainDecisionItem{
			ID:              fmt.Sprintf("execution:%s:semantic-evidence:%d", taskID, log.ID),
			TaskID:          taskID,
			Title:           firstNonEmpty(task.Title, "弱语义证据待核验"),
			Problem:         fmt.Sprintf("commit %s 通过 AI 语义猜测挂到 %s，但分支和提交信息均未显式携带 Jira 号", firstNonEmpty(shortCommit, "-"), taskID),
			Evidence:        []string{fmt.Sprintf("repo=%s / branch=%s", firstNonEmpty(log.Repo, "-"), firstNonEmpty(log.Branch, "-")), fmt.Sprintf("author=%s / assignee=%s", firstNonEmpty(log.Author, "-"), firstNonEmpty(task.Assignee, "-")), "semantic_linker 仅作为候选证据，需人工确认归属"},
			SuggestedAction: "人工复核 commit 所属项目、分支和负责人；确认误绑后移除错误证据或恢复任务状态",
			ImpactScope:     fmt.Sprintf("影响 %s 的完成/评审自动流转可信度", taskID),
			JumpLabel:       taskID,
			RiskLevel:       "critical",
			RiskType:        "semantic_evidence_review",
			Status:          "open",
			Assignee:        normalizeAssignee(firstNonEmpty(task.Assignee, notification.Assignee)),
			Project:         firstNonEmpty(task.Repo, log.Repo, "未归属"),
			IssueType:       firstNonEmpty(normalizeIssueType(task.IssueType), "task"),
			UpdatedAt:       firstNonEmpty(formatDateTime(log.CreatedAt), formatDateTime(now)),
			Source:          "execution",
			Rank:            97,
		})
	}
	return items
}

func scheduleEvidenceLines(item ScheduleItemDTO) []string {
	lines := []string{
		fmt.Sprintf("状态 %s，风险 %s", item.Status, item.RiskLabel),
	}
	if item.DueDate != "" {
		lines = append(lines, "截止日 "+item.DueDate)
	}
	if item.Branch != "" {
		lines = append(lines, "分支 "+item.Branch)
	}
	if item.SubtaskTotal > 0 {
		lines = append(lines, fmt.Sprintf("影子任务 %d/%d 完成", item.SubtaskDone, item.SubtaskTotal))
	}
	return lines
}

func scheduleSuggestedAction(item ScheduleItemDTO) string {
	switch item.RiskLevel {
	case "overdue":
		return "确认是否拆分范围、转派协助或重新承诺截止日，并记录延期原因"
	case "due_soon":
		return "确认剩余工作、验收口径和合并窗口，避免临期变逾期"
	case "stale":
		return "检查阻塞事实，要求负责人补充下一次代码或任务证据"
	case "unscheduled":
		return "补齐负责人、开发分支和截止日，让需求进入可追踪排期"
	default:
		return "保持监听，只有证据缺口或风险升级时打断人工"
	}
}

func scheduleImpactScope(item ScheduleItemDTO) string {
	return fmt.Sprintf("%s / %s / %s", firstNonEmpty(item.ProjectKey, item.Repo, "未归属"), item.Assignee, firstNonEmpty(item.TaskGroupID, "无任务组"))
}

func executionEvidenceLines(item ExecutionTaskItemDTO) []string {
	lines := []string{
		fmt.Sprintf("证据分 %d，commit %d，MR %d", item.EvidenceScore, item.CommitCount, item.MRCount),
	}
	if item.ParentDemandID != "" {
		lines = append(lines, "归属需求 "+item.ParentDemandID)
	}
	if item.LastEvidenceAt != "" {
		lines = append(lines, "最后证据 "+item.LastEvidenceAt)
	}
	if item.ResultLabel != "" {
		lines = append(lines, item.ResultLabel)
	}
	return lines
}

func executionSuggestedAction(item ExecutionTaskItemDTO) string {
	switch item.RiskLabel {
	case "完成无证据":
		return "要求补齐 commit、MR 或验收证据，否则不要把完成状态写入复盘"
	case "状态不一致":
		return "触发 Jira/GitLab 状态对齐，确认 MR 合并后是否应回写完成"
	case "未启动":
		return "确认任务是否真实启动，必要时转派或退回需求拆解"
	case "推进停滞":
		return "检查阻塞原因，给出下一次提交或评审时间"
	case "未绑定需求":
		return "绑定父级需求或任务组，让执行结果能回流排期"
	default:
		return "保留证据链并继续监听状态变化"
	}
}

func executionImpactScope(item ExecutionTaskItemDTO) string {
	if item.ParentDemandID != "" {
		return fmt.Sprintf("影响父需求 %s，执行任务 %s", item.ParentDemandID, item.TaskID)
	}
	return fmt.Sprintf("影响未归属执行任务 %s，难以进入需求复盘", item.TaskID)
}

func executionDecisionRiskType(item ExecutionTaskItemDTO) string {
	switch item.RiskLabel {
	case "完成无证据":
		return "evidence_missing"
	case "状态不一致":
		return "status_mismatch"
	case "未启动", "推进停滞":
		return "stale_after_schedule"
	case "未绑定需求":
		return "orphan_execution"
	}
	tag := firstRiskTag(item.RiskTags)
	switch tag {
	case "state_mismatch":
		return "status_mismatch"
	case "missing_evidence":
		return "evidence_missing"
	case "stale":
		return "stale_after_schedule"
	case "orphan":
		return "orphan_execution"
	default:
		return firstNonEmpty(tag, item.RiskLevel)
	}
}

func executionItemHasCodeResultEvidence(item ExecutionTaskItemDTO) bool {
	return item.CommitCount > 0 ||
		item.MRCount > 0 ||
		item.MergedMRCount > 0 ||
		firstNonEmpty(item.LastCommit) != "" ||
		strings.TrimSpace(item.MRURL) != "" ||
		item.MRIID > 0
}

func executionItemIndicatesStaleAfterSchedule(item ExecutionTaskItemDTO) bool {
	if item.RiskLabel == "未启动" || item.RiskLabel == "推进停滞" {
		return true
	}
	for _, tag := range item.RiskTags {
		if tag == "stale" || tag == "missing_evidence" {
			return true
		}
	}
	return false
}

func strongestBrainScheduleEvidenceRefs(item ScheduleItemDTO) []string {
	refs := []string{"task:" + strings.TrimSpace(item.DemandID)}
	if item.TaskGroupID != "" {
		refs = append(refs, "task_group:"+item.TaskGroupID)
	}
	if item.Branch != "" {
		refs = append(refs, "branch:"+item.Branch)
	}
	if item.DueDate != "" {
		refs = append(refs, "deadline:"+item.DueDate)
	}
	if item.MRURL != "" {
		refs = append(refs, "mr:"+item.MRURL)
	}
	return compactStrings(refs, 8)
}

func strongestBrainExecutionEvidenceRefs(item ExecutionTaskItemDTO) []string {
	refs := []string{"task:" + strings.TrimSpace(item.TaskID)}
	if item.ParentDemandID != "" {
		refs = append(refs, "parent_demand:"+item.ParentDemandID)
	}
	if item.TaskGroupID != "" {
		refs = append(refs, "task_group:"+item.TaskGroupID)
	}
	if firstNonEmpty(item.LastCommit) != "" {
		refs = append(refs, "commit:"+shortEvidenceToken(item.LastCommit))
	}
	if item.MRURL != "" {
		refs = append(refs, "mr:"+item.MRURL)
	} else if item.MRIID > 0 {
		refs = append(refs, fmt.Sprintf("mr_iid:%d", item.MRIID))
	}
	return compactStrings(refs, 8)
}

func strongestBrainLogEvidenceRef(log db.GitCommitLog) string {
	switch {
	case strings.HasPrefix(log.Action, "mr_") && strings.TrimSpace(log.MrURL) != "":
		return "mr:" + strings.TrimSpace(log.MrURL)
	case strings.HasPrefix(log.Action, "mr_") && log.MrIID > 0:
		return fmt.Sprintf("mr_iid:%d", log.MrIID)
	case strings.TrimSpace(log.CommitID) != "":
		return "commit:" + shortEvidenceToken(log.CommitID)
	case strings.TrimSpace(log.Action) != "":
		return "git_event:" + strings.TrimSpace(log.Action)
	default:
		return "git_event"
	}
}

func shortEvidenceToken(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

func latestFormattedTime(current string, candidate string) string {
	current = strings.TrimSpace(current)
	candidate = strings.TrimSpace(candidate)
	if current == "" {
		return candidate
	}
	if candidate == "" {
		return current
	}
	if candidate > current {
		return candidate
	}
	return current
}

func decisionStatusFromLogs(reason string) string {
	if strings.Contains(reason, "尚未") || strings.Contains(reason, "缺少") {
		return "open"
	}
	return "open"
}

func firstRiskTag(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return strings.TrimSpace(tags[0])
}

func enrichStrongestBrainDecisionItems(items []StrongestBrainDecisionItem, profiles map[string]strongestBrainEvidenceProfile, now time.Time) []StrongestBrainDecisionItem {
	enriched := make([]StrongestBrainDecisionItem, 0, len(items))
	for _, item := range items {
		profile, hasProfile := profiles[item.TaskID]
		if item.RecommendedAction == "" {
			item.RecommendedAction = item.SuggestedAction
		}
		if item.DecisionOwner == "" {
			item.DecisionOwner = decisionOwnerForStrongestBrainItem(item)
		}
		if item.Deadline == "" {
			item.Deadline = decisionDeadlineForStrongestBrainItem(item, profile, hasProfile, now)
		}
		if len(item.EvidenceRefs) == 0 {
			if hasProfile {
				item.EvidenceRefs = profile.EvidenceRefs
			} else {
				item.EvidenceRefs = fallbackEvidenceRefsForStrongestBrainItem(item)
			}
		}
		if len(item.MissingLinks) == 0 && hasProfile {
			item.MissingLinks = profile.MissingLinks
		}
		if item.ChainStatus == "" && hasProfile {
			item.ChainStatus = profile.ChainStatus
		}
		if item.EvidenceCompleteness == 0 && hasProfile {
			item.EvidenceCompleteness = profile.EvidenceCompleteness
		}
		if item.RiskType == "evidence_missing" {
			item.ChainStatus = "mismatch"
			item.MissingLinks = append(item.MissingLinks, "completion_evidence")
		}
		if item.RecommendedAction == "" {
			item.RecommendedAction = "确认事实证据后决定处理动作"
		}
		if item.SuggestedAction == "" {
			item.SuggestedAction = item.RecommendedAction
		}
		item.EvidenceRefs = compactStrings(item.EvidenceRefs, 12)
		item.MissingLinks = compactStrings(item.MissingLinks, 10)
		enriched = append(enriched, item)
	}
	return enriched
}

func dedupeStrongestBrainDecisionItems(items []StrongestBrainDecisionItem) []StrongestBrainDecisionItem {
	seen := make(map[string]bool, len(items))
	deduped := make([]StrongestBrainDecisionItem, 0, len(items))
	for _, item := range items {
		key := firstNonEmpty(item.ID, item.Source+":"+item.TaskID+":"+item.RiskType)
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, item)
	}
	return deduped
}

func buildStrongestBrainDecisionSummary(items []StrongestBrainDecisionItem, weeklyDecisionCount int) StrongestBrainDecisionSummary {
	summary := StrongestBrainDecisionSummary{Total: len(items), WeeklyDecisionCount: weeklyDecisionCount}
	for _, item := range items {
		if item.RiskLevel == "critical" {
			summary.Critical++
		} else if item.RiskLevel == "warning" {
			summary.Warning++
		}
		if item.Status == "open" {
			summary.Open++
		}
		switch item.Source {
		case "schedule":
			summary.ScheduleRisks++
		case "execution":
			summary.EvidenceRisks++
		case "context":
			summary.ContextGaps++
		}
		accumulateStrongestBrainExceptionCounters(&summary.EvidenceIncomplete, &summary.StatusMismatch, &summary.MissingSchedule, &summary.StaleAfterSchedule, &summary.DeadlineChainRisks, item)
	}
	return summary
}

func buildStrongestBrainExceptionSummary(items []StrongestBrainDecisionItem) StrongestBrainExceptionSummary {
	summary := StrongestBrainExceptionSummary{
		Total:   len(items),
		ByType:  make(map[string]int),
		ByOwner: make(map[string]int),
	}
	for _, item := range items {
		if item.RiskLevel == "critical" {
			summary.Critical++
		} else if item.RiskLevel == "warning" {
			summary.Warning++
		}
		if item.Status == "open" {
			summary.Open++
		}
		riskType := firstNonEmpty(item.RiskType, "unknown")
		summary.ByType[riskType]++
		owner := firstNonEmpty(item.DecisionOwner, item.Assignee, "未分配")
		summary.ByOwner[owner]++
		accumulateStrongestBrainExceptionCounters(&summary.EvidenceIncomplete, &summary.StatusMismatch, &summary.MissingSchedule, &summary.StaleAfterSchedule, &summary.DeadlineChainRisks, item)
	}
	return summary
}

func accumulateStrongestBrainExceptionCounters(evidenceIncomplete *int, statusMismatch *int, missingSchedule *int, staleAfterSchedule *int, deadlineChainRisks *int, item StrongestBrainDecisionItem) {
	countedEvidence := false
	if item.EvidenceCompleteness > 0 && item.EvidenceCompleteness < 80 {
		*evidenceIncomplete = *evidenceIncomplete + 1
		countedEvidence = true
	}
	switch item.RiskType {
	case "evidence_missing":
		if !countedEvidence {
			*evidenceIncomplete = *evidenceIncomplete + 1
		}
	case "status_mismatch":
		*statusMismatch = *statusMismatch + 1
	case "missing_schedule":
		*missingSchedule = *missingSchedule + 1
	case "stale_after_schedule":
		*staleAfterSchedule = *staleAfterSchedule + 1
	case "due_soon", "overdue":
		if item.ChainStatus == "deadline_at_risk" || len(item.MissingLinks) > 0 {
			*deadlineChainRisks = *deadlineChainRisks + 1
		}
	}
	if item.ChainStatus == "mismatch" && item.RiskType != "status_mismatch" {
		*statusMismatch = *statusMismatch + 1
	}
}

func buildStrongestBrainWeeklyDecisions(items []StrongestBrainDecisionItem) []StrongestBrainWeeklyDecision {
	cards := make([]StrongestBrainWeeklyDecision, 0, 8)
	for _, item := range items {
		if item.Status != "open" {
			continue
		}
		if item.Rank < 60 && item.RiskLevel != "critical" {
			continue
		}
		cards = append(cards, StrongestBrainWeeklyDecision{
			ID:                   "weekly:" + item.ID,
			SourceItemID:         item.ID,
			TaskID:               item.TaskID,
			DecisionType:         weeklyDecisionTypeForRisk(item.RiskType),
			Question:             weeklyDecisionQuestion(item),
			RiskLevel:            item.RiskLevel,
			DecisionOwner:        item.DecisionOwner,
			Deadline:             item.Deadline,
			RecommendedAction:    item.RecommendedAction,
			Options:              weeklyDecisionOptions(item.RiskType),
			EvidenceRefs:         item.EvidenceRefs,
			MissingLinks:         item.MissingLinks,
			ImpactScope:          item.ImpactScope,
			ChainStatus:          item.ChainStatus,
			EvidenceCompleteness: item.EvidenceCompleteness,
		})
		if len(cards) >= 8 {
			break
		}
	}
	return cards
}

func decisionOwnerForStrongestBrainItem(item StrongestBrainDecisionItem) string {
	if assignee := normalizeAssignee(item.Assignee); assignee != "" && assignee != "未分配" {
		return assignee
	}
	switch item.RiskType {
	case "missing_schedule", "context_missing":
		return "PM / 需求负责人"
	case "status_mismatch", "overdue":
		return "研发负责人"
	default:
		return "需求负责人"
	}
}

func decisionDeadlineForStrongestBrainItem(item StrongestBrainDecisionItem, profile strongestBrainEvidenceProfile, hasProfile bool, now time.Time) string {
	if hasProfile && profile.DueDate != "" && (item.RiskType == "due_soon" || item.RiskType == "overdue") {
		return profile.DueDate
	}
	if item.RiskLevel == "critical" {
		return formatOptionalDatePtr(now.AddDate(0, 0, 1))
	}
	return formatOptionalDatePtr(now.AddDate(0, 0, 3))
}

func fallbackEvidenceRefsForStrongestBrainItem(item StrongestBrainDecisionItem) []string {
	refs := []string{}
	if item.TaskID != "" {
		refs = append(refs, "task:"+item.TaskID)
	}
	if item.JumpURL != "" {
		refs = append(refs, "url:"+item.JumpURL)
	}
	if item.Source != "" {
		refs = append(refs, "source:"+item.Source)
	}
	return compactStrings(refs, 6)
}

func weeklyDecisionTypeForRisk(riskType string) string {
	switch riskType {
	case "missing_schedule":
		return "补排期"
	case "due_soon", "overdue":
		return "延期或拆分"
	case "stale_after_schedule":
		return "升级阻塞"
	case "evidence_missing", "status_mismatch", "semantic_evidence_review":
		return "状态证据对齐"
	case "context_missing":
		return "补充需求信息"
	default:
		return "人工决策"
	}
}

func weeklyDecisionQuestion(item StrongestBrainDecisionItem) string {
	switch item.RiskType {
	case "missing_schedule":
		return fmt.Sprintf("%s 是否进入本周排期并承诺截止日？", item.TaskID)
	case "due_soon":
		return fmt.Sprintf("%s 临近截止，是否需要拆分范围或调整合并窗口？", item.TaskID)
	case "overdue":
		return fmt.Sprintf("%s 已逾期，是否延期、转派或缩减范围？", item.TaskID)
	case "stale_after_schedule":
		return fmt.Sprintf("%s 已排期但无推进，是否升级阻塞处理？", item.TaskID)
	case "status_mismatch":
		return fmt.Sprintf("%s MR/Jira 状态不一致，本周由谁拍板回写？", item.TaskID)
	case "evidence_missing":
		return fmt.Sprintf("%s 完成状态缺证据，是否接受或回退完成结论？", item.TaskID)
	default:
		return firstNonEmpty(item.Problem, item.Title)
	}
}

func weeklyDecisionOptions(riskType string) []string {
	switch riskType {
	case "missing_schedule":
		return []string{"补齐负责人/分支/截止日", "退回需求澄清", "挂起并记录原因"}
	case "due_soon", "overdue":
		return []string{"延期并记录原因", "拆分范围先交付核心", "转派/加人协助"}
	case "stale_after_schedule":
		return []string{"升级阻塞", "重新承诺下一次证据时间", "调整负责人"}
	case "evidence_missing":
		return []string{"补齐 commit/MR", "回退完成状态", "记录人工验收证据"}
	case "status_mismatch":
		return []string{"回写 Jira 完成", "回退 MR/任务状态", "补充验收后再关闭"}
	default:
		return []string{"接受推荐动作", "人工改派", "挂起观察"}
	}
}

func buildEvidenceDigest(logs []db.GitCommitLog, profileSets ...map[string]strongestBrainEvidenceProfile) []StrongestBrainEvidenceDigest {
	profiles := map[string]strongestBrainEvidenceProfile{}
	if len(profileSets) > 0 && profileSets[0] != nil {
		profiles = profileSets[0]
	}
	byTask := make(map[string]*StrongestBrainEvidenceDigest)
	for _, log := range logs {
		taskID := strings.TrimSpace(log.TaskID)
		if taskID == "" {
			continue
		}
		item := byTask[taskID]
		if item == nil {
			item = &StrongestBrainEvidenceDigest{TaskID: taskID}
			byTask[taskID] = item
		}
		if log.Action == "git_push" {
			item.CommitCount++
		}
		if strings.HasPrefix(log.Action, "mr_") {
			item.MRCount++
		}
		if item.LastEvidence == "" || log.CreatedAt.Format(time.RFC3339) > item.LastEvidence {
			item.LastEvidence = formatDateTime(log.CreatedAt)
		}
		if len(item.Signals) < 3 {
			item.Signals = append(item.Signals, firstNonEmpty(log.Action, "git_event")+" / "+firstNonEmpty(log.Branch, log.Repo, "unknown"))
		}
	}
	items := make([]StrongestBrainEvidenceDigest, 0, len(byTask))
	for _, item := range byTask {
		if profile, ok := profiles[item.TaskID]; ok {
			item.TaskGroupID = profile.TaskGroupID
			item.EvidenceRefs = profile.EvidenceRefs
			item.MissingLinks = profile.MissingLinks
			item.ChainStatus = profile.ChainStatus
			item.EvidenceCompleteness = profile.EvidenceCompleteness
		}
		items = append(items, *item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].LastEvidence > items[j].LastEvidence
	})
	if len(items) > 12 {
		return items[:12]
	}
	return items
}

func chainTaskDTO(task db.TaskTelemetry) StrongestBrainChainTask {
	return StrongestBrainChainTask{
		TaskID:      strings.TrimSpace(task.TaskID),
		Title:       strings.TrimSpace(task.Title),
		IssueType:   normalizeIssueType(task.IssueType),
		Status:      strings.TrimSpace(task.Status),
		Assignee:    normalizeAssignee(task.Assignee),
		Repo:        strings.TrimSpace(task.Repo),
		Branch:      strings.TrimSpace(task.Branch),
		DueDate:     formatOptionalDate(task.DueDate),
		TaskGroupID: normalizedTaskGroupID(task.TaskGroupID),
	}
}

func buildStrongestBrainEvidenceChainProfile(root db.TaskTelemetry, related []db.TaskTelemetry, logs []db.GitCommitLog, now time.Time) strongestBrainEvidenceProfile {
	profile := strongestBrainEvidenceProfile{
		TaskID:        strings.TrimSpace(root.TaskID),
		TaskGroupID:   normalizedTaskGroupID(root.TaskGroupID),
		IsDemand:      normalizeIssueType(root.IssueType) == "demand" || normalizeIssueType(root.IssueType) == "bug",
		HasIssue:      strings.TrimSpace(root.TaskID) != "",
		HasBranch:     hasScheduleBranch(root.Branch),
		HasDueDate:    root.DueDate != nil && !root.DueDate.IsZero(),
		HasDoneStatus: strings.EqualFold(strings.TrimSpace(root.Status), "done"),
		DueDate:       formatOptionalDate(root.DueDate),
		EvidenceRefs:  []string{"task:" + strings.TrimSpace(root.TaskID)},
	}
	if profile.TaskGroupID != "" {
		profile.EvidenceRefs = append(profile.EvidenceRefs, "task_group:"+profile.TaskGroupID)
	}
	if profile.HasBranch {
		profile.EvidenceRefs = append(profile.EvidenceRefs, "branch:"+strings.TrimSpace(root.Branch))
	}
	if profile.HasDueDate {
		profile.EvidenceRefs = append(profile.EvidenceRefs, "deadline:"+profile.DueDate)
		profile.DeadlineChainRisk = strongestBrainDueDateAtRisk(*root.DueDate, now)
	}

	for _, task := range related {
		issueType := normalizeIssueType(task.IssueType)
		if strings.TrimSpace(task.TaskID) != strings.TrimSpace(root.TaskID) && issueType != "demand" && issueType != "bug" {
			profile.HasExecutionTask = true
			profile.EvidenceRefs = append(profile.EvidenceRefs, "task:"+strings.TrimSpace(task.TaskID))
		}
		profile.HasBranch = profile.HasBranch || hasScheduleBranch(task.Branch)
		profile.HasDoneStatus = profile.HasDoneStatus || strings.EqualFold(strings.TrimSpace(task.Status), "done")
		if strings.TrimSpace(task.LastCommit) != "" && strings.TrimSpace(task.LastCommit) != "-" {
			profile.HasCommit = true
			profile.EvidenceRefs = append(profile.EvidenceRefs, "commit:"+shortEvidenceToken(task.LastCommit))
		}
		if strings.TrimSpace(task.MrURL) != "" || task.MrIID > 0 {
			profile.HasMR = true
			if strings.TrimSpace(task.MrURL) != "" {
				profile.EvidenceRefs = append(profile.EvidenceRefs, "mr:"+strings.TrimSpace(task.MrURL))
			} else {
				profile.EvidenceRefs = append(profile.EvidenceRefs, fmt.Sprintf("mr_iid:%d", task.MrIID))
			}
		}
	}

	stats := strongestBrainLogStatsByTask(logs)
	for _, stat := range stats {
		applyStrongestBrainLogStats(&profile, stat)
	}
	for _, log := range logs {
		var task db.TaskTelemetry
		for _, candidate := range related {
			if strings.TrimSpace(candidate.TaskID) == strings.TrimSpace(log.TaskID) {
				task = candidate
				break
			}
		}
		if log.Action == "mr_merge" && !strings.EqualFold(strings.TrimSpace(task.Status), "done") {
			profile.StatusMismatches = append(profile.StatusMismatches, "merged_mr_status_not_done:"+strings.TrimSpace(log.TaskID))
		}
	}
	if profile.IsDemand && !profile.HasExecutionTask && profile.HasBranch && !profile.HasCommit && !profile.HasMR {
		profile.StaleAfterSchedule = now.Sub(scheduleActivityTime(root)) > 72*time.Hour
	}
	return finalizeStrongestBrainEvidenceProfile(profile)
}

func strongestBrainDueDateAtRisk(due time.Time, now time.Time) bool {
	daysRemaining := int(startOfDay(due).Sub(startOfDay(now)).Hours() / 24)
	return daysRemaining <= 3
}

func evidenceChainSignals(response StrongestBrainEvidenceChainResponse) []string {
	signals := []string{}
	if response.Summary.RelatedTasks == 0 {
		signals = append(signals, "未找到关联任务")
	}
	if response.Summary.Commits == 0 && response.Summary.MergeRequests == 0 {
		signals = append(signals, "缺少代码证据")
	}
	if response.Summary.MergedMRs > 0 {
		signals = append(signals, "已有 MR 合并证据")
	}
	if len(response.Summary.MissingLinks) > 0 {
		signals = append(signals, "证据链缺口："+strings.Join(response.Summary.MissingLinks, " / "))
	}
	if len(response.Summary.StatusMismatches) > 0 {
		signals = append(signals, "状态不一致："+strings.Join(response.Summary.StatusMismatches, " / "))
	}
	if len(signals) == 0 {
		signals = append(signals, "证据链正常回流")
	}
	return signals
}

func intentRequestText(req AIIntentRequest) string {
	parts := []string{strings.TrimSpace(req.Text)}
	for _, msg := range req.Messages {
		content := strings.TrimSpace(msg.Content)
		if content != "" {
			role := strings.TrimSpace(msg.Role)
			if role != "" {
				parts = append(parts, role+": "+content)
			} else {
				parts = append(parts, content)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func analyzeIntentDeterministic(text string) AIIntentResponse {
	normalized := strings.ToLower(strings.TrimSpace(text))
	scores := map[string]int{
		"demand_deconstruction": 0,
		"schedule_adjustment":   0,
		"risk_query":            0,
		"override_intervention": 0,
		"summary":               0,
		"authorization":         0,
		"configuration":         0,
	}
	intentKeywords := map[string][]string{
		"demand_deconstruction": {"需求", "拆解", "解构", "影子任务", "验收", "依赖", "acceptance", "deconstruct"},
		"schedule_adjustment":   {"排期", "延期", "截止", "due", "schedule", "工时", "估算", "临期"},
		"risk_query":            {"风险", "卡点", "异常", "红区", "停滞", "逾期", "阻塞", "risk"},
		"override_intervention": {"调停", "干预", "转派", "负责人", "挂起", "override", "reassign"},
		"summary":               {"总结", "汇总", "周报", "日报", "复盘", "summary", "report"},
		"authorization":         {"权限", "访问", "拒绝", "授权", "policy", "rbac"},
		"configuration":         {"配置", "jira", "gitlab", "webhook", "api key", "token", "ai 配置"},
	}
	for intent, keywords := range intentKeywords {
		for _, keyword := range keywords {
			if strings.Contains(normalized, strings.ToLower(keyword)) {
				scores[intent]++
			}
		}
	}

	intent := "demand_deconstruction"
	maxScore := 0
	for candidate, score := range scores {
		if score > maxScore {
			intent = candidate
			maxScore = score
		}
	}
	if maxScore == 0 {
		intent = "summary"
	}

	confidence := 0.48 + float64(maxScore)*0.12
	if confidence > 0.92 {
		confidence = 0.92
	}
	response := AIIntentResponse{
		Intent:          intent,
		IntentLabel:     intentLabel(intent),
		Confidence:      roundOneDecimal(confidence*100) / 100,
		Summary:         conciseSummary(text),
		SuggestedAction: suggestedActionForIntent(intent),
		MissingContext:  missingContextForIntent(intent, text),
		NextQuestions:   nextQuestionsForIntent(intent),
		Facts:           extractFactLines(text),
		Inferences:      []string{intentInference(intent)},
		RoutedTo:        routeForIntent(intent),
		Source:          "deterministic",
	}
	return response
}

func intentLabel(intent string) string {
	switch intent {
	case "schedule_adjustment":
		return "排期与估算"
	case "risk_query":
		return "风险查询"
	case "override_intervention":
		return "人工调停"
	case "summary":
		return "总结复盘"
	case "authorization":
		return "权限诊断"
	case "configuration":
		return "系统配置"
	default:
		return "需求解构"
	}
}

func suggestedActionForIntent(intent string) string {
	switch intent {
	case "schedule_adjustment":
		return "补齐负责人、截止日、分支和估算口径后进入排期治理"
	case "risk_query":
		return "拉取决策队列和证据链，先确认事实再决定是否干预"
	case "override_intervention":
		return "进入调停预检，确认字段变化、通知对象、同步结果和回滚条件"
	case "summary":
		return "生成日内或周会摘要，并明确区分事实、推断和建议"
	case "authorization":
		return "调用权限解释，定位命中策略、缺失权限和资源范围"
	case "configuration":
		return "进入系统配置检查连接、Webhook、AI 引擎和语料状态"
	default:
		return "先做需求澄清和 AI 解构，输出缺失信息、验收标准、风险和影子任务"
	}
}

func missingContextForIntent(intent string, text string) []string {
	missing := []string{}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "-") && !strings.Contains(lower, "jira") && !strings.Contains(lower, "需求") {
		missing = append(missing, "关联需求或 Jira 编号")
	}
	switch intent {
	case "schedule_adjustment":
		missing = append(missing, "目标截止日", "负责人或协作人", "估算工时")
	case "override_intervention":
		missing = append(missing, "干预原因", "预期影响范围", "回滚条件")
	case "risk_query":
		missing = append(missing, "项目范围", "风险时间窗口")
	case "summary":
		missing = append(missing, "总结周期", "受众角色")
	case "authorization":
		missing = append(missing, "操作动作", "资源范围")
	case "configuration":
		missing = append(missing, "配置模块", "错误现象或测试结果")
	default:
		missing = append(missing, "业务目标", "验收标准", "依赖系统")
	}
	return compactStrings(missing, 4)
}

func nextQuestionsForIntent(intent string) []string {
	switch intent {
	case "schedule_adjustment":
		return []string{"这次调整是延期、提前、还是重新估算？", "是否已有开发分支和目标负责人？"}
	case "risk_query":
		return []string{"要看日内异常、周会议题，还是某个项目的风险？", "是否只看红区和逾期项？"}
	case "override_intervention":
		return []string{"本次干预要改变负责人、截止日、状态，还是升级会议？", "是否需要同步 Jira 并通知相关人？"}
	case "summary":
		return []string{"总结对象是个人、部门、项目，还是全局？", "输出用于日报、周会，还是复盘？"}
	case "authorization":
		return []string{"哪个用户在执行哪个动作时被拒绝？", "资源范围是全局、项目、部门，还是个人？"}
	case "configuration":
		return []string{"要检查 Jira、GitLab、飞书，还是 AI 引擎？", "当前失败信息或返回状态是什么？"}
	default:
		return []string{"这个需求的业务目标和验收标准是什么？", "是否已有目标项目、负责人或截止日？"}
	}
}

func intentInference(intent string) string {
	return fmt.Sprintf("系统将该输入路由为%s，原因是文本命中了相关关键词和协同上下文。", intentLabel(intent))
}

func routeForIntent(intent string) string {
	switch intent {
	case "schedule_adjustment":
		return "schedule"
	case "risk_query":
		return "decision_queue"
	case "override_intervention":
		return "override"
	case "summary":
		return "kpi_report"
	case "authorization":
		return "authz_explain"
	case "configuration":
		return "settings"
	default:
		return "deconstructor"
	}
}

func conciseSummary(text string) string {
	cleaned := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if cleaned == "" {
		return ""
	}
	runes := []rune(cleaned)
	if len(runes) > 96 {
		return string(runes[:96]) + "..."
	}
	return cleaned
}

func extractFactLines(text string) []string {
	lines := strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == '。' || r == ';' || r == '；'
	})
	facts := make([]string, 0, 3)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if utf8.RuneCountInString(line) > 80 {
			runes := []rune(line)
			line = string(runes[:80]) + "..."
		}
		facts = append(facts, line)
		if len(facts) >= 3 {
			break
		}
	}
	if len(facts) == 0 {
		facts = append(facts, "用户提供了一段待识别文本")
	}
	return facts
}

func compactStrings(values []string, limit int) []string {
	seen := map[string]bool{}
	result := make([]string, 0, limit)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func (s *Server) handleStrongestBrainIntervention(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TaskID      string `json:"task_id"`
		Action      string `json:"action"` // "reassign", "reschedule", "link_repo"
		Value       string `json:"value"`  // 新指派人, 新截止日期, 新仓库名等
		Reason      string `json:"reason"` // 兼容旧客户端的本地审计理由
		MeetingNote string `json:"meeting_note"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request: invalid JSON", http.StatusBadRequest)
		return
	}

	if req.TaskID == "" || req.Action == "" || req.Value == "" {
		http.Error(w, "Bad Request: task_id, action, and value are required", http.StatusBadRequest)
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

	actor := r.Header.Get("x-authenticated-user-name")
	if actor == "" {
		actor = r.Header.Get("x-authenticated-user-id")
	}
	if actor == "" {
		actor = "Unknown"
	}

	var oldVal string
	switch req.Action {
	case "reassign":
		oldVal = task.Assignee
		task.Assignee = req.Value
		task.LastUpdate = time.Now()
	case "reschedule":
		if task.DueDate != nil {
			oldVal = task.DueDate.Format("2006-01-02")
		} else {
			oldVal = ""
		}

		t, err := time.Parse("2006-01-02", req.Value)
		if err != nil {
			t2, err2 := time.Parse(time.RFC3339, req.Value)
			if err2 != nil {
				http.Error(w, "Bad Request: invalid date format (expected YYYY-MM-DD)", http.StatusBadRequest)
				return
			}
			t = t2
		}
		newDueDate := t.Format("2006-01-02")
		if oldVal == newDueDate {
			http.Error(w, "Bad Request: new due date must differ from the current due date", http.StatusConflict)
			return
		}

		task.DueDate = &t
		task.LastUpdate = time.Now()
	case "link_repo":
		oldVal = task.Repo
		task.Repo = req.Value
		task.LastUpdate = time.Now()
	default:
		http.Error(w, "Bad Request: invalid action", http.StatusBadRequest)
		return
	}

	tx := db.DB.Begin()
	if err := tx.Save(&task).Error; err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to update task: %v", err), http.StatusInternalServerError)
		return
	}

	meetingNote := strings.TrimSpace(req.MeetingNote)
	auditReason := meetingNote
	if auditReason == "" {
		auditReason = strings.TrimSpace(req.Reason)
	}
	event := db.DecisionEvent{
		TaskID:    req.TaskID,
		Actor:     actor,
		Action:    "override_" + req.Action,
		OldValue:  oldVal,
		NewValue:  req.Value,
		Reason:    auditReason,
		CreatedAt: time.Now(),
	}
	if err := tx.Create(&event).Error; err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to record decision event: %v", err), http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to commit intervention: %v", err), http.StatusInternalServerError)
		return
	}

	jiraCommentRequested := false
	if s.config.Jira.Enabled && !strings.HasPrefix(task.TaskID, "TASK-") {
		switch req.Action {
		case "reassign":
			go s.jiraAssigneeSync(task.TaskID, task.Assignee)
		case "reschedule":
			go s.jiraDueDateSync(task.TaskID, task.DueDate.Format("2006-01-02"))
		}
		if meetingNote != "" {
			jiraCommentRequested = true
			go s.jiraCommentSync(task.TaskID, meetingNote)
		}
	}

	dueDate := ""
	if task.DueDate != nil {
		dueDate = task.DueDate.Format("2006-01-02")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":              "success",
		"event":               event,
		"due_date":            dueDate,
		"meeting_note_synced": jiraCommentRequested,
	})
}
