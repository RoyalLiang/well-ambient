package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"well-ambient/internal/db"
	"well-ambient/internal/delivery"

	"gorm.io/gorm"
)

type demandSpecPayload struct {
	ID                 uint         `json:"id"`
	DemandID           string       `json:"demand_id"`
	SourceArchiveID    uint         `json:"source_archive_id"`
	ContextPackID      uint         `json:"context_pack_id"`
	OriginalText       string       `json:"original_text"`
	Intent             string       `json:"intent"`
	IntentConfidence   float64      `json:"intent_confidence"`
	Summary            string       `json:"summary"`
	UserGoal           string       `json:"user_goal"`
	Facts              []string     `json:"facts"`
	Inferences         []string     `json:"inferences"`
	MissingContext     []string     `json:"missing_context"`
	BusinessRules      []string     `json:"business_rules"`
	MainFlows          []string     `json:"main_flows"`
	ExceptionFlows     []string     `json:"exception_flows"`
	PermissionRules    []string     `json:"permission_rules"`
	DataImpact         []string     `json:"data_impact"`
	APIImpact          []string     `json:"api_impact"`
	UIImpact           []string     `json:"ui_impact"`
	Dependencies       []string     `json:"dependencies"`
	Risks              []string     `json:"risks"`
	AcceptanceCriteria []string     `json:"acceptance_criteria"`
	TestPlan           []string     `json:"test_plan"`
	MappedRepos        []string     `json:"mapped_repos"`
	Tasks              []TaskDetail `json:"tasks"`
	ReadinessScore     int          `json:"readiness_score"`
	ModelVersion       string       `json:"model_version"`
	RuleVersion        string       `json:"rule_version"`
}

type demandSpecDTO struct {
	ID                 uint         `json:"id"`
	DemandID           string       `json:"demand_id"`
	Version            int          `json:"version"`
	Status             string       `json:"status"`
	SourceArchiveID    uint         `json:"source_archive_id"`
	ContextPackID      uint         `json:"context_pack_id"`
	OriginalText       string       `json:"original_text"`
	Intent             string       `json:"intent"`
	IntentConfidence   float64      `json:"intent_confidence"`
	Summary            string       `json:"summary"`
	UserGoal           string       `json:"user_goal"`
	Facts              []string     `json:"facts"`
	Inferences         []string     `json:"inferences"`
	MissingContext     []string     `json:"missing_context"`
	BusinessRules      []string     `json:"business_rules"`
	MainFlows          []string     `json:"main_flows"`
	ExceptionFlows     []string     `json:"exception_flows"`
	PermissionRules    []string     `json:"permission_rules"`
	DataImpact         []string     `json:"data_impact"`
	APIImpact          []string     `json:"api_impact"`
	UIImpact           []string     `json:"ui_impact"`
	Dependencies       []string     `json:"dependencies"`
	Risks              []string     `json:"risks"`
	AcceptanceCriteria []string     `json:"acceptance_criteria"`
	TestPlan           []string     `json:"test_plan"`
	MappedRepos        []string     `json:"mapped_repos"`
	Tasks              []TaskDetail `json:"tasks"`
	ReadinessScore     int          `json:"readiness_score"`
	ModelVersion       string       `json:"model_version"`
	RuleVersion        string       `json:"rule_version"`
	AuthoredBy         string       `json:"authored_by"`
	ReviewedBy         string       `json:"reviewed_by"`
	FrozenBy           string       `json:"frozen_by"`
	FrozenAt           *time.Time   `json:"frozen_at"`
	CreatedAt          time.Time    `json:"created_at"`
	UpdatedAt          time.Time    `json:"updated_at"`
}

type protectedPathRule struct {
	Pattern            string   `json:"pattern"`
	RequiredRole       string   `json:"required_role"`
	ReviewerCandidates []string `json:"reviewer_candidates"`
}

type reviewContractPayload struct {
	ID                  uint                `json:"id"`
	DemandSpecVersionID uint                `json:"demand_spec_version_id"`
	RequiredRoles       []string            `json:"required_roles"`
	ReviewerCandidates  []string            `json:"reviewer_candidates"`
	AcceptanceOwner     string              `json:"acceptance_owner"`
	MinimumApprovals    int                 `json:"minimum_approvals"`
	ProtectedPathRules  []protectedPathRule `json:"protected_path_rules"`
	SegregationRules    []string            `json:"segregation_rules"`
	ReviewSLAHours      int                 `json:"review_sla_hours"`
	EscalationOwner     string              `json:"escalation_owner"`
}

type reviewContractDTO struct {
	ID                  uint                `json:"id"`
	DemandSpecVersionID uint                `json:"demand_spec_version_id"`
	DemandID            string              `json:"demand_id"`
	Status              string              `json:"status"`
	RequiredRoles       []string            `json:"required_roles"`
	ReviewerCandidates  []string            `json:"reviewer_candidates"`
	ResolvedReviewers   []string            `json:"resolved_reviewers"`
	AcceptanceOwner     string              `json:"acceptance_owner"`
	MinimumApprovals    int                 `json:"minimum_approvals"`
	ProtectedPathRules  []protectedPathRule `json:"protected_path_rules"`
	SegregationRules    []string            `json:"segregation_rules"`
	ReviewSLAHours      int                 `json:"review_sla_hours"`
	EscalationOwner     string              `json:"escalation_owner"`
	ResolutionStatus    string              `json:"resolution_status"`
	ResolutionReason    string              `json:"resolution_reason"`
	ApprovedBy          string              `json:"approved_by"`
	ApprovedAt          *time.Time          `json:"approved_at"`
	CreatedAt           time.Time           `json:"created_at"`
	UpdatedAt           time.Time           `json:"updated_at"`
}

func (s *Server) handleListDemandSpecs(w http.ResponseWriter, r *http.Request) {
	demandID := strings.TrimSpace(r.URL.Query().Get("demand_id"))
	if demandID == "" {
		http.Error(w, "demand_id is required", http.StatusBadRequest)
		return
	}
	var specs []db.DemandSpecVersion
	if err := db.DB.Where("demand_id = ?", demandID).Order("version desc").Find(&specs).Error; err != nil {
		http.Error(w, "failed to list demand specs", http.StatusInternalServerError)
		return
	}
	items := make([]demandSpecDTO, 0, len(specs))
	for _, spec := range specs {
		items = append(items, demandSpecFromModel(spec))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handleSaveDemandSpec(w http.ResponseWriter, r *http.Request) {
	var payload demandSpecPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	actor := authenticatedActor(r)
	if payload.ID != 0 {
		s.updateDemandSpec(w, payload, actor)
		return
	}
	s.createDemandSpec(w, payload, actor)
}

func (s *Server) createDemandSpec(w http.ResponseWriter, payload demandSpecPayload, actor string) {
	demandID := strings.TrimSpace(payload.DemandID)
	if demandID == "" {
		http.Error(w, "demand_id is required", http.StatusBadRequest)
		return
	}

	tx := db.DB.Begin()
	var demand db.TaskTelemetry
	if err := tx.Where("task_id = ? AND issue_type = ?", demandID, "demand").First(&demand).Error; err != nil {
		tx.Rollback()
		http.Error(w, "demand not found", http.StatusNotFound)
		return
	}
	if err := hydrateSpecPayloadFromArchive(tx, &payload); err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.OriginalText) == "" {
		payload.OriginalText = firstNonBlank(demand.Description, demand.Title)
	}
	if strings.TrimSpace(payload.Intent) == "" {
		intent := analyzeIntentDeterministic(payload.OriginalText)
		payload.Intent = intent.Intent
		payload.IntentConfidence = intent.Confidence
		if payload.Summary == "" {
			payload.Summary = intent.Summary
		}
		payload.Facts = appendUniqueStrings(payload.Facts, intent.Facts...)
		payload.Inferences = appendUniqueStrings(payload.Inferences, intent.Inferences...)
		payload.MissingContext = appendUniqueStrings(payload.MissingContext, intent.MissingContext...)
	}

	var maxVersion int
	if err := tx.Model(&db.DemandSpecVersion{}).Where("demand_id = ?", demandID).Select("COALESCE(MAX(version), 0)").Scan(&maxVersion).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to allocate spec version", http.StatusInternalServerError)
		return
	}
	now := time.Now()
	spec := demandSpecModel(payload, demandID, maxVersion+1, actor, now)
	if err := tx.Create(&spec).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to create demand spec", http.StatusInternalServerError)
		return
	}
	contract := defaultReviewContract(spec, demand, payload, s.config.Jira.SyncUsers, now)
	if err := tx.Create(&contract).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to create review contract", http.StatusInternalServerError)
		return
	}

	completedAt := now
	if err := tx.Model(&db.ExecutionRun{}).
		Where("demand_id = ? AND demand_spec_version_id != ? AND status IN ?", demandID, spec.ID, []string{delivery.RunPending, delivery.RunPreflightBlocked, delivery.RunPreflightReady}).
		Updates(map[string]interface{}{"status": delivery.RunCancelled, "block_reason": fmt.Sprintf("superseded by demand spec v%d", spec.Version), "completed_at": &completedAt}).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to invalidate stale execution runs", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to commit demand spec", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"spec": demandSpecFromModel(spec), "review_contract": reviewContractFromModel(contract)})
}

func (s *Server) updateDemandSpec(w http.ResponseWriter, payload demandSpecPayload, actor string) {
	var spec db.DemandSpecVersion
	if err := db.DB.First(&spec, payload.ID).Error; err != nil {
		http.Error(w, "demand spec not found", http.StatusNotFound)
		return
	}
	if spec.Status != delivery.SpecDraft {
		http.Error(w, "frozen demand specs are immutable; create a new version", http.StatusConflict)
		return
	}
	payload.DemandID = spec.DemandID
	payload.SourceArchiveID = firstNonZero(payload.SourceArchiveID, spec.SourceArchiveID)
	payload.ContextPackID = firstNonZero(payload.ContextPackID, spec.ContextPackID)
	updated := demandSpecModel(payload, spec.DemandID, spec.Version, spec.AuthoredBy, spec.CreatedAt)
	updated.ID = spec.ID
	updated.ReviewedBy = actor
	updated.UpdatedAt = time.Now()
	if err := db.DB.Save(&updated).Error; err != nil {
		http.Error(w, "failed to update demand spec", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"spec": demandSpecFromModel(updated)})
}

func (s *Server) handleFreezeDemandSpec(w http.ResponseWriter, r *http.Request) {
	id, err := pathUint(r, "id")
	if err != nil {
		http.Error(w, "invalid demand spec id", http.StatusBadRequest)
		return
	}
	tx := db.DB.Begin()
	var spec db.DemandSpecVersion
	if err := tx.First(&spec, id).Error; err != nil {
		tx.Rollback()
		http.Error(w, "demand spec not found", http.StatusNotFound)
		return
	}
	if spec.Status == delivery.SpecFrozen {
		tx.Rollback()
		writeJSON(w, http.StatusOK, map[string]interface{}{"spec": demandSpecFromModel(spec)})
		return
	}
	var contract db.ReviewContract
	if err := tx.Where("demand_spec_version_id = ?", spec.ID).First(&contract).Error; err != nil || contract.Status != delivery.ReviewApproved {
		tx.Rollback()
		http.Error(w, "an approved review contract is required before freezing", http.StatusConflict)
		return
	}
	if blockers := specFreezeBlockers(spec); len(blockers) > 0 {
		tx.Rollback()
		writeJSON(w, http.StatusUnprocessableEntity, map[string]interface{}{"error": "spec_not_ready", "blockers": blockers})
		return
	}
	now := time.Now()
	spec.Status = delivery.SpecFrozen
	spec.FrozenBy = authenticatedActor(r)
	spec.FrozenAt = &now
	spec.UpdatedAt = now
	if err := tx.Save(&spec).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to freeze demand spec", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to commit frozen demand spec", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"spec": demandSpecFromModel(spec), "review_contract": reviewContractFromModel(contract)})
}

func (s *Server) handleGetReviewContract(w http.ResponseWriter, r *http.Request) {
	specID, err := strconv.ParseUint(strings.TrimSpace(r.URL.Query().Get("demand_spec_version_id")), 10, 64)
	if err != nil || specID == 0 {
		http.Error(w, "demand_spec_version_id is required", http.StatusBadRequest)
		return
	}
	var contract db.ReviewContract
	if err := db.DB.Where("demand_spec_version_id = ?", uint(specID)).First(&contract).Error; err != nil {
		http.Error(w, "review contract not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"review_contract": reviewContractFromModel(contract)})
}

func (s *Server) handleSaveReviewContract(w http.ResponseWriter, r *http.Request) {
	var payload reviewContractPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	var contract db.ReviewContract
	query := db.DB
	if payload.ID != 0 {
		query = query.First(&contract, payload.ID)
	} else {
		query = query.Where("demand_spec_version_id = ?", payload.DemandSpecVersionID).First(&contract)
	}
	if query.Error != nil {
		http.Error(w, "review contract not found", http.StatusNotFound)
		return
	}
	if contract.Status == delivery.ReviewApproved {
		http.Error(w, "approved review contracts are immutable", http.StatusConflict)
		return
	}
	var spec db.DemandSpecVersion
	if err := db.DB.First(&spec, contract.DemandSpecVersionID).Error; err != nil || spec.Status == delivery.SpecFrozen {
		http.Error(w, "review contract cannot be changed after spec freeze", http.StatusConflict)
		return
	}
	applyReviewContractPayload(&contract, payload)
	if err := db.DB.Save(&contract).Error; err != nil {
		http.Error(w, "failed to update review contract", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"review_contract": reviewContractFromModel(contract)})
}

func (s *Server) handleApproveReviewContract(w http.ResponseWriter, r *http.Request) {
	id, err := pathUint(r, "id")
	if err != nil {
		http.Error(w, "invalid review contract id", http.StatusBadRequest)
		return
	}
	var contract db.ReviewContract
	if err := db.DB.First(&contract, id).Error; err != nil {
		http.Error(w, "review contract not found", http.StatusNotFound)
		return
	}
	if contract.Status == delivery.ReviewApproved {
		writeJSON(w, http.StatusOK, map[string]interface{}{"review_contract": reviewContractFromModel(contract)})
		return
	}
	candidates := decodeStringList(contract.ReviewerCandidatesJSON)
	if contract.MinimumApprovals < 1 || len(candidates) < contract.MinimumApprovals || strings.TrimSpace(contract.AcceptanceOwner) == "" {
		http.Error(w, "review candidates, minimum approvals, and acceptance owner are required", http.StatusUnprocessableEntity)
		return
	}
	now := time.Now()
	contract.Status = delivery.ReviewApproved
	contract.ApprovedBy = authenticatedActor(r)
	contract.ApprovedAt = &now
	contract.UpdatedAt = now
	if err := db.DB.Save(&contract).Error; err != nil {
		http.Error(w, "failed to approve review contract", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"review_contract": reviewContractFromModel(contract)})
}

func hydrateSpecPayloadFromArchive(tx *gorm.DB, payload *demandSpecPayload) error {
	if payload.SourceArchiveID == 0 {
		return nil
	}
	var archive db.DeconstructArchive
	if err := tx.First(&archive, payload.SourceArchiveID).Error; err != nil {
		return fmt.Errorf("source deconstruction archive not found")
	}
	if archive.DemandID != "" && payload.DemandID != "" && archive.DemandID != payload.DemandID {
		return fmt.Errorf("source archive belongs to a different demand")
	}
	payload.ContextPackID = firstNonZero(payload.ContextPackID, archive.ContextPackID)
	if payload.OriginalText == "" {
		payload.OriginalText = archive.InputText
	}
	if len(payload.MappedRepos) == 0 {
		_ = json.Unmarshal([]byte(archive.MappedReposJSON), &payload.MappedRepos)
	}
	if len(payload.Tasks) == 0 {
		_ = json.Unmarshal([]byte(archive.TasksJSON), &payload.Tasks)
	}
	analysis := DeconstructAnalysis{}
	if json.Unmarshal([]byte(archive.AnalysisJSON), &analysis) == nil {
		payload.MissingContext = appendUniqueStrings(payload.MissingContext, analysis.MissingInfo...)
		payload.Risks = appendUniqueStrings(payload.Risks, analysis.Risks...)
		payload.Dependencies = appendUniqueStrings(payload.Dependencies, analysis.Dependencies...)
		payload.AcceptanceCriteria = appendUniqueStrings(payload.AcceptanceCriteria, analysis.AcceptanceCriteria...)
		payload.TestPlan = appendUniqueStrings(payload.TestPlan, analysis.AcceptanceCriteria...)
		if payload.ReadinessScore == 0 {
			payload.ReadinessScore = analysis.CompletenessScore
		}
	}
	return nil
}

func demandSpecModel(payload demandSpecPayload, demandID string, version int, authoredBy string, createdAt time.Time) db.DemandSpecVersion {
	return db.DemandSpecVersion{
		DemandID:               demandID,
		Version:                version,
		Status:                 delivery.SpecDraft,
		SourceArchiveID:        payload.SourceArchiveID,
		ContextPackID:          payload.ContextPackID,
		OriginalText:           strings.TrimSpace(payload.OriginalText),
		Intent:                 strings.TrimSpace(payload.Intent),
		IntentConfidence:       payload.IntentConfidence,
		Summary:                strings.TrimSpace(payload.Summary),
		UserGoal:               strings.TrimSpace(payload.UserGoal),
		FactsJSON:              encodeJSON(payload.Facts),
		InferencesJSON:         encodeJSON(payload.Inferences),
		MissingContextJSON:     encodeJSON(payload.MissingContext),
		BusinessRulesJSON:      encodeJSON(payload.BusinessRules),
		MainFlowsJSON:          encodeJSON(payload.MainFlows),
		ExceptionFlowsJSON:     encodeJSON(payload.ExceptionFlows),
		PermissionRulesJSON:    encodeJSON(payload.PermissionRules),
		DataImpactJSON:         encodeJSON(payload.DataImpact),
		APIImpactJSON:          encodeJSON(payload.APIImpact),
		UIImpactJSON:           encodeJSON(payload.UIImpact),
		DependenciesJSON:       encodeJSON(payload.Dependencies),
		RisksJSON:              encodeJSON(payload.Risks),
		AcceptanceCriteriaJSON: encodeJSON(payload.AcceptanceCriteria),
		TestPlanJSON:           encodeJSON(payload.TestPlan),
		MappedReposJSON:        encodeJSON(payload.MappedRepos),
		TasksJSON:              encodeJSON(payload.Tasks),
		ReadinessScore:         payload.ReadinessScore,
		ModelVersion:           strings.TrimSpace(payload.ModelVersion),
		RuleVersion:            strings.TrimSpace(payload.RuleVersion),
		AuthoredBy:             authoredBy,
		CreatedAt:              createdAt,
		UpdatedAt:              time.Now(),
	}
}

func defaultReviewContract(spec db.DemandSpecVersion, demand db.TaskTelemetry, payload demandSpecPayload, configuredUsers []string, now time.Time) db.ReviewContract {
	candidates := append([]string{}, configuredUsers...)
	for _, task := range payload.Tasks {
		candidates = appendUniqueStrings(candidates, strings.TrimSpace(task.Assignee))
	}
	candidates = normalizeDeliveryStrings(candidates)
	acceptanceOwner := firstNonBlank(demand.Assignee, demand.Creator)
	escalationOwner := firstNonBlank(demand.Creator, acceptanceOwner)
	return db.ReviewContract{
		DemandSpecVersionID:    spec.ID,
		DemandID:               spec.DemandID,
		Status:                 delivery.ReviewDraft,
		RequiredRolesJSON:      encodeJSON([]string{"code_owner"}),
		ReviewerCandidatesJSON: encodeJSON(candidates),
		ResolvedReviewersJSON:  "[]",
		AcceptanceOwner:        acceptanceOwner,
		MinimumApprovals:       1,
		ProtectedPathRulesJSON: "[]",
		SegregationRulesJSON:   encodeJSON([]string{"author_cannot_self_approve"}),
		ReviewSLAHours:         24,
		EscalationOwner:        escalationOwner,
		ResolutionStatus:       "pending",
		CreatedAt:              now,
		UpdatedAt:              now,
	}
}

func applyReviewContractPayload(contract *db.ReviewContract, payload reviewContractPayload) {
	contract.RequiredRolesJSON = encodeJSON(normalizeDeliveryStrings(payload.RequiredRoles))
	contract.ReviewerCandidatesJSON = encodeJSON(normalizeDeliveryStrings(payload.ReviewerCandidates))
	contract.AcceptanceOwner = strings.TrimSpace(payload.AcceptanceOwner)
	contract.MinimumApprovals = payload.MinimumApprovals
	if contract.MinimumApprovals < 1 {
		contract.MinimumApprovals = 1
	}
	contract.ProtectedPathRulesJSON = encodeJSON(payload.ProtectedPathRules)
	contract.SegregationRulesJSON = encodeJSON(normalizeDeliveryStrings(payload.SegregationRules))
	contract.ReviewSLAHours = payload.ReviewSLAHours
	if contract.ReviewSLAHours < 1 {
		contract.ReviewSLAHours = 24
	}
	contract.EscalationOwner = strings.TrimSpace(payload.EscalationOwner)
	contract.UpdatedAt = time.Now()
}

func specFreezeBlockers(spec db.DemandSpecVersion) []string {
	blockers := make([]string, 0)
	if spec.ReadinessScore < 70 {
		blockers = append(blockers, "readiness_score must be at least 70")
	}
	if strings.TrimSpace(spec.Summary) == "" && strings.TrimSpace(spec.UserGoal) == "" {
		blockers = append(blockers, "summary or user_goal is required")
	}
	if len(decodeStringList(spec.AcceptanceCriteriaJSON)) == 0 {
		blockers = append(blockers, "acceptance criteria are required")
	}
	if len(decodeStringList(spec.TestPlanJSON)) == 0 {
		blockers = append(blockers, "test plan is required")
	}
	if len(decodeStringList(spec.MappedReposJSON)) == 0 {
		blockers = append(blockers, "at least one mapped repository is required")
	}
	return blockers
}

func demandSpecFromModel(spec db.DemandSpecVersion) demandSpecDTO {
	tasks := []TaskDetail{}
	_ = json.Unmarshal([]byte(spec.TasksJSON), &tasks)
	return demandSpecDTO{
		ID: spec.ID, DemandID: spec.DemandID, Version: spec.Version, Status: spec.Status,
		SourceArchiveID: spec.SourceArchiveID, ContextPackID: spec.ContextPackID,
		OriginalText: spec.OriginalText, Intent: spec.Intent, IntentConfidence: spec.IntentConfidence,
		Summary: spec.Summary, UserGoal: spec.UserGoal, Facts: decodeStringList(spec.FactsJSON),
		Inferences: decodeStringList(spec.InferencesJSON), MissingContext: decodeStringList(spec.MissingContextJSON),
		BusinessRules: decodeStringList(spec.BusinessRulesJSON), MainFlows: decodeStringList(spec.MainFlowsJSON),
		ExceptionFlows: decodeStringList(spec.ExceptionFlowsJSON), PermissionRules: decodeStringList(spec.PermissionRulesJSON),
		DataImpact: decodeStringList(spec.DataImpactJSON), APIImpact: decodeStringList(spec.APIImpactJSON), UIImpact: decodeStringList(spec.UIImpactJSON),
		Dependencies: decodeStringList(spec.DependenciesJSON), Risks: decodeStringList(spec.RisksJSON),
		AcceptanceCriteria: decodeStringList(spec.AcceptanceCriteriaJSON), TestPlan: decodeStringList(spec.TestPlanJSON),
		MappedRepos: decodeStringList(spec.MappedReposJSON), Tasks: tasks, ReadinessScore: spec.ReadinessScore,
		ModelVersion: spec.ModelVersion, RuleVersion: spec.RuleVersion, AuthoredBy: spec.AuthoredBy,
		ReviewedBy: spec.ReviewedBy, FrozenBy: spec.FrozenBy, FrozenAt: spec.FrozenAt,
		CreatedAt: spec.CreatedAt, UpdatedAt: spec.UpdatedAt,
	}
}

func reviewContractFromModel(contract db.ReviewContract) reviewContractDTO {
	rules := []protectedPathRule{}
	_ = json.Unmarshal([]byte(contract.ProtectedPathRulesJSON), &rules)
	return reviewContractDTO{
		ID: contract.ID, DemandSpecVersionID: contract.DemandSpecVersionID, DemandID: contract.DemandID,
		Status: contract.Status, RequiredRoles: decodeStringList(contract.RequiredRolesJSON),
		ReviewerCandidates: decodeStringList(contract.ReviewerCandidatesJSON), ResolvedReviewers: decodeStringList(contract.ResolvedReviewersJSON),
		AcceptanceOwner: contract.AcceptanceOwner, MinimumApprovals: contract.MinimumApprovals,
		ProtectedPathRules: rules, SegregationRules: decodeStringList(contract.SegregationRulesJSON),
		ReviewSLAHours: contract.ReviewSLAHours, EscalationOwner: contract.EscalationOwner,
		ResolutionStatus: contract.ResolutionStatus, ResolutionReason: contract.ResolutionReason,
		ApprovedBy: contract.ApprovedBy, ApprovedAt: contract.ApprovedAt, CreatedAt: contract.CreatedAt, UpdatedAt: contract.UpdatedAt,
	}
}

func encodeJSON(value interface{}) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func decodeStringList(raw string) []string {
	items := []string{}
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &items)
	}
	return normalizeDeliveryStrings(items)
}

func normalizeDeliveryStrings(items []string) []string {
	result := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		key := strings.ToLower(item)
		if item == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, item)
	}
	return result
}

func appendUniqueStrings(items []string, extra ...string) []string {
	return normalizeDeliveryStrings(append(items, extra...))
}

func authenticatedActor(r *http.Request) string {
	return firstNonBlank(r.Header.Get("x-authenticated-user-name"), r.Header.Get("x-authenticated-user-id"), "unknown")
}

func pathUint(r *http.Request, name string) (uint, error) {
	value, err := strconv.ParseUint(strings.TrimSpace(r.PathValue(name)), 10, 64)
	return uint(value), err
}

func firstNonZero(values ...uint) uint {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
