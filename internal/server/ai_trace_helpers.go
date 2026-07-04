package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"well-ambient/internal/db"

	"gorm.io/gorm"
)

const (
	aiOutputTraceRuleVersion             = "ai-output-trace-v1"
	requirementClarificationRuleVersion  = "requirement-clarification-v1"
	defaultRequirementReadinessThreshold = 70
)

type aiTraceQuery struct {
	ArchiveID     uint
	ContextPackID uint
	DemandID      string
	TaskGroupID   string
}

type AIOutputTraceReadModel struct {
	ID                    string                    `json:"id"`
	ArchiveID             uint                      `json:"archive_id,omitempty"`
	ContextPackID         uint                      `json:"context_pack_id"`
	InputSnapshot         AITraceInputSnapshot      `json:"input_snapshot"`
	RequirementSummary    string                    `json:"requirement_summary"`
	Output                AITraceOutputSnapshot     `json:"output"`
	ModelVersion          string                    `json:"model_version"`
	RuleVersion           string                    `json:"rule_version"`
	PromptTemplateVersion string                    `json:"prompt_template_version"`
	SourceVersions        []AITraceSourceVersion    `json:"source_versions"`
	ReferencedFacts       []AITraceReferencedFact   `json:"referenced_facts"`
	Confidence            float64                   `json:"confidence"`
	ContextPack           *AITraceContextPackReplay `json:"context_pack,omitempty"`
	HumanFeedback         AITraceHumanFeedback      `json:"human_feedback"`
	CreatedAt             time.Time                 `json:"created_at"`
}

type AITraceInputSnapshot struct {
	DemandID    string `json:"demand_id,omitempty"`
	TaskGroupID string `json:"task_group_id,omitempty"`
	Text        string `json:"text"`
	TextHash    string `json:"text_hash"`
}

type AITraceOutputSnapshot struct {
	MappedRepos []string            `json:"mappedRepos"`
	Tasks       []TaskDetail        `json:"tasks"`
	Analysis    DeconstructAnalysis `json:"analysis"`
	IsMock      bool                `json:"is_mock,omitempty"`
}

type AITraceSourceVersion struct {
	FactID      uint   `json:"fact_id,omitempty"`
	Type        string `json:"type"`
	Scope       string `json:"scope"`
	ScopeID     string `json:"scope_id,omitempty"`
	Source      string `json:"source"`
	Version     int    `json:"version"`
	ContentHash string `json:"content_hash,omitempty"`
}

type AITraceReferencedFact struct {
	FactID      uint    `json:"fact_id,omitempty"`
	Type        string  `json:"type"`
	Scope       string  `json:"scope"`
	ScopeID     string  `json:"scope_id,omitempty"`
	Source      string  `json:"source"`
	Version     int     `json:"version"`
	Summary     string  `json:"summary"`
	Content     string  `json:"content,omitempty"`
	ContentHash string  `json:"content_hash,omitempty"`
	TokenCount  int     `json:"token_count"`
	Score       float64 `json:"score"`
}

type AITraceContextPackReplay struct {
	ID                    uint                    `json:"id"`
	Purpose               string                  `json:"purpose"`
	Model                 string                  `json:"model"`
	PromptTemplateVersion string                  `json:"prompt_template_version"`
	WorkHoursPerDay       float64                 `json:"work_hours_per_day"`
	TokenBudget           int                     `json:"token_budget"`
	TokenCount            int                     `json:"token_count"`
	Summary               string                  `json:"summary"`
	ContextHash           string                  `json:"context_hash"`
	ScopeSignature        string                  `json:"scope_signature"`
	ItemCount             int                     `json:"item_count"`
	Items                 []AITraceReferencedFact `json:"items"`
	CreatedAt             time.Time               `json:"created_at"`
}

type AITraceHumanFeedback struct {
	Status    string     `json:"status"`
	Decision  string     `json:"decision,omitempty"`
	Actor     string     `json:"actor,omitempty"`
	Note      string     `json:"note,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type RequirementClarificationReadModel struct {
	ArchiveID               uint                        `json:"archive_id,omitempty"`
	DemandID                string                      `json:"demand_id,omitempty"`
	TaskGroupID             string                      `json:"task_group_id,omitempty"`
	ContextPackID           uint                        `json:"context_pack_id"`
	ReadinessScore          int                         `json:"readiness_score"`
	ReadinessLevel          string                      `json:"readiness_level"`
	MissingQuestions        RequirementMissingQuestions `json:"missing_questions"`
	AcceptanceCriteriaDraft []string                    `json:"acceptance_criteria_draft"`
	RiskFlags               []RequirementRiskFlag       `json:"risk_flags"`
	Confidence              float64                     `json:"confidence"`
	RuleVersion             string                      `json:"rule_version"`
	CreatedAt               time.Time                   `json:"created_at"`
}

type RequirementMissingQuestions struct {
	MustAnswer []RequirementQuestion `json:"must_answer"`
	CanDefer   []RequirementQuestion `json:"can_defer"`
	Suggested  []RequirementQuestion `json:"suggested"`
}

type RequirementQuestion struct {
	Question string `json:"question"`
	Source   string `json:"source"`
	Reason   string `json:"reason"`
}

type RequirementRiskFlag struct {
	Flag     string `json:"flag"`
	Severity string `json:"severity"`
	Source   string `json:"source"`
	Reason   string `json:"reason"`
}

func (s *Server) handleGetAIOutputTrace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	trace, err := s.buildAIOutputTraceReadModel(db.DB, parseAITraceQuery(r))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"trace": trace,
	})
}

func (s *Server) handleGetRequirementClarification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	clarification, err := s.buildRequirementClarificationReadModel(db.DB, parseAITraceQuery(r))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"clarification": clarification,
	})
}

func (s *Server) handleGetContextPackReplay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	id := parseUintParam(r, "id", "context_pack_id")
	if id == 0 {
		http.Error(w, "context_pack_id is required", http.StatusBadRequest)
		return
	}
	replay, err := buildContextPackReplay(db.DB, id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"context_pack": replay,
	})
}

func parseAITraceQuery(r *http.Request) aiTraceQuery {
	return aiTraceQuery{
		ArchiveID:     parseUintParam(r, "archive_id", "id"),
		ContextPackID: parseUintParam(r, "context_pack_id"),
		DemandID:      strings.TrimSpace(r.URL.Query().Get("demand_id")),
		TaskGroupID:   strings.TrimSpace(r.URL.Query().Get("task_group_id")),
	}
}

func parseUintParam(r *http.Request, names ...string) uint {
	for _, name := range names {
		value := strings.TrimSpace(r.URL.Query().Get(name))
		if value == "" {
			value = strings.TrimSpace(r.PathValue(name))
		}
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err == nil {
			return uint(parsed)
		}
	}
	return 0
}

func (s *Server) buildAIOutputTraceReadModel(tx *gorm.DB, query aiTraceQuery) (AIOutputTraceReadModel, error) {
	archive, err := findDeconstructArchive(tx, query)
	if err != nil {
		return AIOutputTraceReadModel{}, err
	}

	output, err := decodeArchiveOutput(archive)
	if err != nil {
		return AIOutputTraceReadModel{}, err
	}
	return s.buildAIOutputTraceFromOutput(tx, &archive, archive.InputText, archive.DemandID, archive.TaskGroupID, archive.ContextPackID, output, archive.CreatedAt)
}

func (s *Server) buildAIOutputTraceFromOutput(tx *gorm.DB, archive *db.DeconstructArchive, inputText string, demandID string, taskGroupID string, contextPackID uint, output AITraceOutputSnapshot, createdAt time.Time) (AIOutputTraceReadModel, error) {
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	var contextPack *AITraceContextPackReplay
	if contextPackID > 0 && tx != nil {
		replay, err := buildContextPackReplay(tx, contextPackID)
		if err != nil {
			return AIOutputTraceReadModel{}, err
		}
		contextPack = &replay
	}

	archiveID := uint(0)
	confidence := output.Analysis.Confidence
	if archive != nil {
		archiveID = archive.ID
		if archive.Confidence > 0 {
			confidence = archive.Confidence
		}
	}
	confidence = normalizeContextScore(confidence, 0.5)

	modelVersion := ""
	promptTemplateVersion := contextPackTemplateV1
	var referencedFacts []AITraceReferencedFact
	var sourceVersions []AITraceSourceVersion
	if contextPack != nil {
		modelVersion = strings.TrimSpace(contextPack.Model)
		if contextPack.PromptTemplateVersion != "" {
			promptTemplateVersion = contextPack.PromptTemplateVersion
		}
		referencedFacts = contextPack.Items
		sourceVersions = sourceVersionsFromFacts(contextPack.Items)
	}
	if modelVersion == "" && s != nil && s.config != nil {
		modelVersion = strings.TrimSpace(s.config.AI.Model)
	}
	if modelVersion == "" {
		modelVersion = "unconfigured"
	}

	traceID := fmt.Sprintf("deconstruct-archive-%d", archiveID)
	if archiveID == 0 {
		traceID = fmt.Sprintf("deconstruct-context-%d-%s", contextPackID, compactHashSuffix(stableHash(inputText)))
	}

	inputText = strings.TrimSpace(inputText)
	trace := AIOutputTraceReadModel{
		ID:            traceID,
		ArchiveID:     archiveID,
		ContextPackID: contextPackID,
		InputSnapshot: AITraceInputSnapshot{
			DemandID:    strings.TrimSpace(demandID),
			TaskGroupID: strings.TrimSpace(taskGroupID),
			Text:        inputText,
			TextHash:    stableHash(inputText),
		},
		RequirementSummary:    summarizeRequirementInput(inputText, output),
		Output:                output,
		ModelVersion:          modelVersion,
		RuleVersion:           aiOutputTraceRuleVersion,
		PromptTemplateVersion: promptTemplateVersion,
		SourceVersions:        sourceVersions,
		ReferencedFacts:       referencedFacts,
		Confidence:            confidence,
		ContextPack:           contextPack,
		HumanFeedback: AITraceHumanFeedback{
			Status: "pending",
		},
		CreatedAt: createdAt,
	}
	return trace, nil
}

func (s *Server) buildRequirementClarificationReadModel(tx *gorm.DB, query aiTraceQuery) (RequirementClarificationReadModel, error) {
	archive, err := findDeconstructArchive(tx, query)
	if err != nil {
		return RequirementClarificationReadModel{}, err
	}
	output, err := decodeArchiveOutput(archive)
	if err != nil {
		return RequirementClarificationReadModel{}, err
	}
	return buildRequirementClarificationFromOutput(&archive, output), nil
}

func buildRequirementClarificationFromOutput(archive *db.DeconstructArchive, output AITraceOutputSnapshot) RequirementClarificationReadModel {
	analysis := output.Analysis
	readiness := deriveReadinessScore(analysis, output.Tasks)
	missingQuestions := buildMissingQuestions(analysis, readiness)
	acceptanceCriteria := buildAcceptanceCriteriaDraft(analysis, output.Tasks)
	riskFlags := buildRequirementRiskFlags(analysis, readiness, missingQuestions, acceptanceCriteria)

	model := RequirementClarificationReadModel{
		ContextPackID:           0,
		ReadinessScore:          readiness,
		ReadinessLevel:          readinessLevel(readiness),
		MissingQuestions:        missingQuestions,
		AcceptanceCriteriaDraft: acceptanceCriteria,
		RiskFlags:               riskFlags,
		Confidence:              normalizeContextScore(analysis.Confidence, 0.5),
		RuleVersion:             requirementClarificationRuleVersion,
	}
	if archive != nil {
		model.ArchiveID = archive.ID
		model.DemandID = archive.DemandID
		model.TaskGroupID = archive.TaskGroupID
		model.ContextPackID = archive.ContextPackID
		model.CreatedAt = archive.CreatedAt
		if archive.Confidence > 0 {
			model.Confidence = normalizeContextScore(archive.Confidence, model.Confidence)
		}
	}
	if model.CreatedAt.IsZero() {
		model.CreatedAt = time.Now()
	}
	return model
}

func findDeconstructArchive(tx *gorm.DB, query aiTraceQuery) (db.DeconstructArchive, error) {
	if tx == nil {
		tx = db.DB
	}
	if tx == nil {
		return db.DeconstructArchive{}, fmt.Errorf("database not initialized")
	}

	dbQuery := tx.Model(&db.DeconstructArchive{})
	if query.ArchiveID > 0 {
		dbQuery = dbQuery.Where("id = ?", query.ArchiveID)
	} else if query.ContextPackID > 0 {
		dbQuery = dbQuery.Where("context_pack_id = ?", query.ContextPackID)
	} else if query.DemandID != "" {
		dbQuery = dbQuery.Where("demand_id = ?", query.DemandID)
	} else if query.TaskGroupID != "" {
		dbQuery = dbQuery.Where("task_group_id = ?", query.TaskGroupID)
	} else {
		return db.DeconstructArchive{}, fmt.Errorf("archive_id, context_pack_id, demand_id, or task_group_id is required")
	}

	var archive db.DeconstructArchive
	if err := dbQuery.Order("created_at desc, id desc").First(&archive).Error; err != nil {
		return db.DeconstructArchive{}, err
	}
	return archive, nil
}

func decodeArchiveOutput(archive db.DeconstructArchive) (AITraceOutputSnapshot, error) {
	output := AITraceOutputSnapshot{
		MappedRepos: []string{},
		Tasks:       []TaskDetail{},
	}
	if strings.TrimSpace(archive.MappedReposJSON) != "" {
		if err := json.Unmarshal([]byte(archive.MappedReposJSON), &output.MappedRepos); err != nil {
			return output, fmt.Errorf("decode mapped repos: %w", err)
		}
	}
	if strings.TrimSpace(archive.TasksJSON) != "" {
		if err := json.Unmarshal([]byte(archive.TasksJSON), &output.Tasks); err != nil {
			return output, fmt.Errorf("decode tasks: %w", err)
		}
	}
	if strings.TrimSpace(archive.AnalysisJSON) != "" {
		if err := json.Unmarshal([]byte(archive.AnalysisJSON), &output.Analysis); err != nil {
			return output, fmt.Errorf("decode analysis: %w", err)
		}
	}
	output.IsMock = archive.IsMock
	normalized := DeconstructResponse{
		MappedRepos: output.MappedRepos,
		Tasks:       output.Tasks,
		Analysis:    output.Analysis,
		IsMock:      output.IsMock,
	}
	normalizeDeconstructResponse(&normalized, false)
	output.MappedRepos = normalized.MappedRepos
	output.Tasks = normalized.Tasks
	output.Analysis = normalized.Analysis
	return output, nil
}

func buildContextPackReplay(tx *gorm.DB, contextPackID uint) (AITraceContextPackReplay, error) {
	if tx == nil {
		tx = db.DB
	}
	if tx == nil {
		return AITraceContextPackReplay{}, fmt.Errorf("database not initialized")
	}

	var pack db.ContextPack
	if err := tx.First(&pack, contextPackID).Error; err != nil {
		return AITraceContextPackReplay{}, err
	}

	var items []db.ContextPackItem
	if err := tx.Where("context_pack_id = ?", contextPackID).Order("position asc, id asc").Find(&items).Error; err != nil {
		return AITraceContextPackReplay{}, err
	}

	factIDs := make([]uint, 0, len(items))
	for _, item := range items {
		if item.ContextFactID > 0 {
			factIDs = append(factIDs, item.ContextFactID)
		}
	}
	factsByID := map[uint]db.ContextFact{}
	if len(factIDs) > 0 {
		var facts []db.ContextFact
		if err := tx.Where("id IN ?", factIDs).Find(&facts).Error; err != nil {
			return AITraceContextPackReplay{}, err
		}
		for _, fact := range facts {
			factsByID[fact.ID] = fact
		}
	}

	referenced := make([]AITraceReferencedFact, 0, len(items))
	for _, item := range items {
		fact := factsByID[item.ContextFactID]
		ref := AITraceReferencedFact{
			FactID:      item.ContextFactID,
			Type:        fact.Type,
			Scope:       fact.Scope,
			ScopeID:     fact.ScopeID,
			Source:      fact.Source,
			Version:     fact.Version,
			Summary:     item.Summary,
			Content:     fact.Content,
			ContentHash: fact.ContentHash,
			TokenCount:  item.TokenCount,
			Score:       roundScore(item.Score),
		}
		if ref.Summary == "" {
			ref.Summary = fact.Summary
		}
		if ref.TokenCount == 0 {
			ref.TokenCount = fact.TokenCount
		}
		if ref.Type == "" {
			ref.Type = "context_pack_item"
		}
		if ref.Scope == "" {
			ref.Scope = "global"
		}
		if ref.Source == "" {
			ref.Source = "context_pack"
		}
		if ref.Version <= 0 {
			ref.Version = 1
		}
		referenced = append(referenced, ref)
	}

	replay := AITraceContextPackReplay{
		ID:                    pack.ID,
		Purpose:               pack.Purpose,
		Model:                 pack.Model,
		PromptTemplateVersion: pack.PromptTemplateVersion,
		WorkHoursPerDay:       pack.WorkHoursPerDay,
		TokenBudget:           pack.TokenBudget,
		TokenCount:            pack.TokenCount,
		Summary:               pack.Summary,
		ContextHash:           pack.ContextHash,
		ScopeSignature:        pack.ScopeSignature,
		ItemCount:             pack.ItemCount,
		Items:                 referenced,
		CreatedAt:             pack.CreatedAt,
	}
	if replay.PromptTemplateVersion == "" {
		replay.PromptTemplateVersion = contextPackTemplateV1
	}
	if replay.ItemCount == 0 {
		replay.ItemCount = len(referenced)
	}
	return replay, nil
}

func sourceVersionsFromFacts(facts []AITraceReferencedFact) []AITraceSourceVersion {
	versions := make([]AITraceSourceVersion, 0, len(facts))
	seen := map[string]struct{}{}
	for _, fact := range facts {
		key := fmt.Sprintf("%d|%s|%s|%s|%s", fact.FactID, fact.Type, fact.Scope, fact.ScopeID, fact.Source)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		version := fact.Version
		if version <= 0 {
			version = 1
		}
		versions = append(versions, AITraceSourceVersion{
			FactID:      fact.FactID,
			Type:        fact.Type,
			Scope:       fact.Scope,
			ScopeID:     fact.ScopeID,
			Source:      fact.Source,
			Version:     version,
			ContentHash: fact.ContentHash,
		})
	}
	return versions
}

func aiTraceOutputFromDeconstructResponse(result DeconstructResponse) AITraceOutputSnapshot {
	return AITraceOutputSnapshot{
		MappedRepos: append([]string{}, result.MappedRepos...),
		Tasks:       append([]TaskDetail{}, result.Tasks...),
		Analysis:    result.Analysis,
		IsMock:      result.IsMock,
	}
}

func summarizeRequirementInput(inputText string, output AITraceOutputSnapshot) string {
	inputText = strings.TrimSpace(inputText)
	if inputText != "" {
		return compactText(inputText, 180)
	}
	if len(output.Tasks) > 0 {
		titles := make([]string, 0, len(output.Tasks))
		for _, task := range output.Tasks {
			if strings.TrimSpace(task.Title) != "" {
				titles = append(titles, strings.TrimSpace(task.Title))
			}
			if len(titles) >= 3 {
				break
			}
		}
		if len(titles) > 0 {
			return compactText(strings.Join(titles, "；"), 180)
		}
	}
	return "未提供需求输入快照"
}

func buildImportInputSnapshot(demandID string, result DeconstructResponse) string {
	var b strings.Builder
	if strings.TrimSpace(demandID) != "" {
		b.WriteString("需求ID：")
		b.WriteString(strings.TrimSpace(demandID))
		b.WriteString("\n")
	}
	if len(result.MappedRepos) > 0 {
		b.WriteString("映射仓库：")
		b.WriteString(strings.Join(result.MappedRepos, "、"))
		b.WriteString("\n")
	}
	for _, task := range result.Tasks {
		if strings.TrimSpace(task.Title) == "" {
			continue
		}
		b.WriteString("- ")
		if strings.TrimSpace(task.ID) != "" {
			b.WriteString(strings.TrimSpace(task.ID))
			b.WriteString("：")
		}
		b.WriteString(strings.TrimSpace(task.Title))
		if strings.TrimSpace(task.Repo) != "" {
			b.WriteString("（")
			b.WriteString(strings.TrimSpace(task.Repo))
			b.WriteString("）")
		}
		b.WriteString("\n")
	}
	if len(result.Analysis.MissingInfo) > 0 {
		b.WriteString("缺失信息：")
		b.WriteString(strings.Join(result.Analysis.MissingInfo, "；"))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func compactText(text string, limit int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if limit <= 0 || len([]rune(text)) <= limit {
		return text
	}
	runes := []rune(text)
	return string(runes[:limit]) + "..."
}

func compactHashSuffix(hash string) string {
	if len(hash) <= 10 {
		return hash
	}
	return hash[:10]
}

func deriveReadinessScore(analysis DeconstructAnalysis, tasks []TaskDetail) int {
	score := analysis.CompletenessScore
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	if score == 0 && len(analysis.MissingInfo) == 0 && len(analysis.Risks) == 0 && len(tasks) > 0 {
		score = 60
	}
	if analysis.Confidence > 0 && normalizeContextScore(analysis.Confidence, 0.5) < 0.45 && score > 10 {
		score -= 10
	}
	if len(analysis.AcceptanceCriteria) == 0 && score > 5 {
		score -= 5
	}
	if score < 0 {
		return 0
	}
	return score
}

func readinessLevel(score int) string {
	switch {
	case score >= 80:
		return "ready"
	case score >= defaultRequirementReadinessThreshold:
		return "review_needed"
	case score >= 50:
		return "clarification_required"
	default:
		return "blocked"
	}
}

func buildMissingQuestions(analysis DeconstructAnalysis, readiness int) RequirementMissingQuestions {
	questions := RequirementMissingQuestions{
		MustAnswer: []RequirementQuestion{},
		CanDefer:   []RequirementQuestion{},
		Suggested:  []RequirementQuestion{},
	}

	for _, item := range normalizeStringList(analysis.MissingInfo) {
		questions.MustAnswer = append(questions.MustAnswer, RequirementQuestion{
			Question: ensureQuestion(item),
			Source:   "analysis.missing_info",
			Reason:   "缺失信息会影响开发边界、接口口径或验收判断。",
		})
	}

	for _, item := range normalizeStringList(analysis.MeetingQuestions) {
		question := RequirementQuestion{
			Question: ensureQuestion(item),
			Source:   "analysis.meeting_questions",
			Reason:   "需求评审需要确认该问题以降低返工风险。",
		}
		switch classifyQuestionPriority(item, readiness) {
		case "suggested":
			question.Reason = "补充后可提升交付共识，但不阻塞开发启动。"
			questions.Suggested = append(questions.Suggested, question)
		case "can_defer":
			question.Reason = "可以先启动开发，但需要在联调或验收前确认。"
			questions.CanDefer = append(questions.CanDefer, question)
		default:
			questions.MustAnswer = append(questions.MustAnswer, question)
		}
	}

	for _, item := range normalizeStringList(analysis.Dependencies) {
		questions.CanDefer = append(questions.CanDefer, RequirementQuestion{
			Question: ensureQuestion("依赖是否已准备：" + item),
			Source:   "analysis.dependencies",
			Reason:   "依赖未就绪通常不阻塞需求拆解，但会影响联调、排期和验收窗口。",
		})
	}

	if len(analysis.AcceptanceCriteria) == 0 {
		questions.MustAnswer = append(questions.MustAnswer, RequirementQuestion{
			Question: "请补充可测试、可观察的验收标准？",
			Source:   "analysis.acceptance_criteria",
			Reason:   "没有验收标准时，开发完成和需求满足无法被客观判定。",
		})
	}

	return questions
}

func classifyQuestionPriority(text string, readiness int) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	if containsAny(lower, []string{"可后置", "后置", "上线后", "后续", "联调前", "验收前"}) {
		return "can_defer"
	}
	if containsAny(lower, []string{"建议", "补充", "优化", "文案", "埋点", "运营"}) {
		return "suggested"
	}
	if readiness < defaultRequirementReadinessThreshold {
		return "must_answer"
	}
	if containsAny(lower, []string{"必须", "权限", "数据", "接口", "验收", "安全", "合规", "回滚", "灰度", "核心", "边界", "口径"}) {
		return "must_answer"
	}
	return "suggested"
}

func buildAcceptanceCriteriaDraft(analysis DeconstructAnalysis, tasks []TaskDetail) []string {
	criteria := normalizeStringList(analysis.AcceptanceCriteria)
	if len(criteria) > 0 {
		return criteria
	}

	criteria = []string{}
	for _, task := range tasks {
		title := strings.TrimSpace(task.Title)
		if title == "" {
			continue
		}
		criteria = append(criteria, fmt.Sprintf("%s 已完成，并有自测、联调或回归证据。", title))
		if len(criteria) >= 5 {
			break
		}
	}
	if len(criteria) == 0 {
		criteria = append(criteria, "需求目标、主流程、异常流程和完成证据均被确认。")
	}
	return criteria
}

func buildRequirementRiskFlags(analysis DeconstructAnalysis, readiness int, questions RequirementMissingQuestions, acceptanceCriteria []string) []RequirementRiskFlag {
	flags := []RequirementRiskFlag{}
	for _, item := range normalizeStringList(analysis.Risks) {
		flags = append(flags, RequirementRiskFlag{
			Flag:     item,
			Severity: riskSeverity(item),
			Source:   "analysis.risks",
			Reason:   "AI 解构识别出的交付、质量或协作风险。",
		})
	}
	if readiness < defaultRequirementReadinessThreshold {
		flags = append(flags, RequirementRiskFlag{
			Flag:     "需求就绪度低于开发准入阈值",
			Severity: "high",
			Source:   "readiness_score",
			Reason:   fmt.Sprintf("当前 readiness_score=%d，低于阈值 %d。", readiness, defaultRequirementReadinessThreshold),
		})
	}
	if len(questions.MustAnswer) > 0 {
		flags = append(flags, RequirementRiskFlag{
			Flag:     "仍有必须回答的问题",
			Severity: "high",
			Source:   "missing_questions.must_answer",
			Reason:   fmt.Sprintf("共有 %d 个问题不回答会影响开发边界或验收。", len(questions.MustAnswer)),
		})
	}
	if normalizeContextScore(analysis.Confidence, 0.5) < 0.55 {
		flags = append(flags, RequirementRiskFlag{
			Flag:     "AI 置信度偏低",
			Severity: "medium",
			Source:   "analysis.confidence",
			Reason:   "需求信息或上下文证据不足，AI 建议需要人工复核。",
		})
	}
	if len(acceptanceCriteria) == 0 {
		flags = append(flags, RequirementRiskFlag{
			Flag:     "缺少验收标准草案",
			Severity: "medium",
			Source:   "acceptance_criteria_draft",
			Reason:   "没有验收口径会导致完成定义不清。",
		})
	}
	return flags
}

func riskSeverity(text string) string {
	lower := strings.ToLower(text)
	if containsAny(lower, []string{"安全", "权限", "合规", "越权", "支付", "资损", "阻塞", "延期", "返工", "数据", "上线", "回滚"}) {
		return "high"
	}
	return "medium"
}

func ensureQuestion(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if strings.HasSuffix(text, "?") || strings.HasSuffix(text, "？") {
		return text
	}
	return text + "？"
}

func containsAny(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}
