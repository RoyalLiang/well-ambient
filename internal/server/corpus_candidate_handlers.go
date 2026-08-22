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
	"well-ambient/internal/readmodel"

	"gorm.io/gorm"
)

type corpusCandidatePatch struct {
	CandidateType string  `json:"candidate_type"`
	Scope         string  `json:"scope"`
	ScopeID       string  `json:"scope_id"`
	Title         string  `json:"title"`
	Summary       string  `json:"summary"`
	Content       string  `json:"content"`
	SourceAnchor  string  `json:"source_anchor"`
	EvidenceKind  string  `json:"evidence_kind"`
	Confidence    float64 `json:"confidence"`
	Sensitivity   string  `json:"sensitivity"`
}

type reviewCorpusCandidateRequest struct {
	Decision  string                `json:"decision"`
	Note      string                `json:"note"`
	Candidate *corpusCandidatePatch `json:"candidate,omitempty"`
}

type publishCorpusCandidateRequest struct {
	Note      string                `json:"note"`
	Candidate *corpusCandidatePatch `json:"candidate,omitempty"`
}

type corpusCandidateDocumentRef struct {
	ID              uint   `json:"id"`
	Title           string `json:"title"`
	OriginalName    string `json:"original_name"`
	Version         int    `json:"version"`
	ContentHash     string `json:"content_hash"`
	IngestionStatus string `json:"ingestion_status"`
}

type corpusCandidateListItem struct {
	db.CorpusCandidate
	SourceDocument *corpusCandidateDocumentRef `json:"source_document,omitempty"`
}

type corpusCandidateImpactDTO struct {
	Candidate            db.CorpusCandidate  `json:"candidate"`
	SourceDocument       *db.ContextDocument `json:"source_document,omitempty"`
	CurrentFacts         []db.ContextFact    `json:"current_facts"`
	BeforeMarkdown       string              `json:"before_markdown"`
	AfterMarkdown        string              `json:"after_markdown"`
	SourceMarkdown       string              `json:"source_markdown"`
	RequiresImpactReview bool                `json:"requires_impact_review"`
	Reason               string              `json:"reason"`
}

func (s *Server) handleListCorpusCandidates(w http.ResponseWriter, r *http.Request) {
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	statuses := make([]string, 0, 3)
	switch status {
	case "", "review_queue":
		status = "review_queue"
		statuses = []string{"pending", "impact_review"}
	case "all":
	default:
		for _, value := range strings.Split(status, ",") {
			value = strings.TrimSpace(value)
			if value != "" {
				statuses = append(statuses, value)
			}
		}
	}
	documentID := uint(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("document_id")); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || parsed == 0 {
			http.Error(w, "document_id must be a positive integer", http.StatusBadRequest)
			return
		}
		documentID = uint(parsed)
	}
	position := struct {
		CreatedAt time.Time `json:"created_at"`
		ID        uint      `json:"id"`
	}{}
	var items []corpusCandidateListItem
	var page readPageMeta
	err := db.DB.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		window, err := readmodel.OpenPage(r.Context(), tx, readmodel.PageRequest{
			Dataset: "context_corpus", Contract: "corpus-candidates",
			Scope: map[string]any{"status": status, "statuses": statuses, "document_id": documentID}, Cursor: r.URL.Query().Get("cursor"),
			Limit: r.URL.Query().Get("limit"), DefaultLimit: 50, MaxLimit: 100,
		}, &position)
		if err != nil {
			return err
		}
		query := tx.Order("created_at desc, id desc").Limit(window.Limit + 1)
		if len(statuses) > 0 {
			query = query.Where("status IN ?", statuses)
		}
		if documentID > 0 {
			query = query.Where("context_document_id = ?", documentID)
		}
		if window.HasCursor {
			query = query.Where("created_at < ? OR (created_at = ? AND id < ?)", position.CreatedAt, position.CreatedAt, position.ID)
		}
		var candidates []db.CorpusCandidate
		if err := query.Find(&candidates).Error; err != nil {
			return err
		}
		hasMore := len(candidates) > window.Limit
		if hasMore {
			candidates = candidates[:window.Limit]
		}
		documentIDs := make([]uint, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate.ContextDocumentID > 0 {
				documentIDs = append(documentIDs, candidate.ContextDocumentID)
			}
		}
		var documents []db.ContextDocument
		if len(documentIDs) > 0 {
			if err := tx.Select("id", "title", "original_name", "version", "content_hash", "ingestion_status").
				Where("id IN ?", documentIDs).Find(&documents).Error; err != nil {
				return err
			}
		}
		documentByID := make(map[uint]db.ContextDocument, len(documents))
		for _, document := range documents {
			documentByID[document.ID] = document
		}
		items = make([]corpusCandidateListItem, 0, len(candidates))
		for _, candidate := range candidates {
			item := corpusCandidateListItem{CorpusCandidate: candidate}
			if document, ok := documentByID[candidate.ContextDocumentID]; ok {
				item.SourceDocument = &corpusCandidateDocumentRef{
					ID: document.ID, Title: document.Title, OriginalName: document.OriginalName,
					Version: document.Version, ContentHash: document.ContentHash, IngestionStatus: document.IngestionStatus,
				}
			}
			items = append(items, item)
		}
		last := position
		if len(candidates) > 0 {
			last.CreatedAt = candidates[len(candidates)-1].CreatedAt
			last.ID = candidates[len(candidates)-1].ID
		}
		page, err = buildReadPageMeta(window, hasMore, last)
		return err
	})
	if err != nil {
		if errors.Is(err, readmodel.ErrStaleCursor) || errors.Is(err, readmodel.ErrCursorScope) || errors.Is(err, readmodel.ErrInvalidCursor) || strings.Contains(err.Error(), "limit") {
			writeReadPageError(w, err)
			return
		}
		http.Error(w, "failed to list corpus candidates", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items, "page": page})
}

func (s *Server) handleReviewCorpusCandidate(w http.ResponseWriter, r *http.Request) {
	id, err := pathUint(r, "id")
	if err != nil {
		http.Error(w, "invalid corpus candidate id", http.StatusBadRequest)
		return
	}
	var req reviewCorpusCandidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	decision := strings.ToLower(strings.TrimSpace(req.Decision))
	if decision != "accepted" && decision != "rejected" {
		http.Error(w, "decision must be accepted or rejected", http.StatusBadRequest)
		return
	}

	tx := db.DB.Begin()
	var candidate db.CorpusCandidate
	if err := tx.First(&candidate, id).Error; err != nil {
		tx.Rollback()
		http.Error(w, "corpus candidate not found", http.StatusNotFound)
		return
	}
	if candidate.Status == "accepted" || candidate.Status == "rejected" {
		tx.Rollback()
		writeJSON(w, http.StatusOK, map[string]interface{}{"candidate": candidate, "reused": true})
		return
	}
	if err := applyCorpusCandidatePatch(&candidate, req.Candidate); err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	now := time.Now()
	actor := authenticatedActor(r)
	candidate.ReviewMode = corpusCandidateReviewMode(candidate)
	candidate.ReviewedBy = actor
	candidate.ReviewNote = strings.TrimSpace(req.Note)
	candidate.ReviewedAt = &now
	candidate.UpdatedAt = now

	var fact *db.ContextFact
	requiresImpactReview := false
	if decision == "rejected" {
		candidate.Status = "rejected"
	} else if candidate.ReviewMode == "impact_required" {
		candidate.Status = "impact_review"
		candidate.ImpactPreviewedBy = ""
		candidate.ImpactPreviewedAt = nil
		requiresImpactReview = true
	} else {
		created, err := contextFactFromCandidate(tx, candidate, actor, now)
		if err != nil {
			tx.Rollback()
			http.Error(w, "failed to promote corpus candidate", http.StatusInternalServerError)
			return
		}
		fact = &created
		candidate.Status = "accepted"
		candidate.AcceptedContextFactID = created.ID
		candidate.PublishedBy = actor
		candidate.PublishedAt = &now
	}
	if err := tx.Save(&candidate).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to save corpus review", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to commit corpus review", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"candidate":              candidate,
		"context_fact":           fact,
		"requires_impact_review": requiresImpactReview,
		"reused":                 false,
	})
}

func (s *Server) handlePreviewCorpusCandidateImpact(w http.ResponseWriter, r *http.Request) {
	id, err := pathUint(r, "id")
	if err != nil {
		http.Error(w, "invalid corpus candidate id", http.StatusBadRequest)
		return
	}
	var candidate db.CorpusCandidate
	if err := db.DB.First(&candidate, id).Error; err != nil {
		http.Error(w, "corpus candidate not found", http.StatusNotFound)
		return
	}
	if candidate.Status == "impact_review" {
		now := time.Now()
		candidate.ImpactPreviewedBy = authenticatedActor(r)
		candidate.ImpactPreviewedAt = &now
		candidate.UpdatedAt = now
		if err := db.DB.Save(&candidate).Error; err != nil {
			http.Error(w, "failed to record corpus impact preview", http.StatusInternalServerError)
			return
		}
	}
	var sourceDocument *db.ContextDocument
	if candidate.ContextDocumentID > 0 {
		var document db.ContextDocument
		if err := db.DB.First(&document, candidate.ContextDocumentID).Error; err == nil {
			sourceDocument = &document
		}
	}
	factType := contextFactTypeFromCandidate(candidate.CandidateType)
	var currentFacts []db.ContextFact
	_ = db.DB.Where("status = ? AND type = ? AND scope = ? AND scope_id = ?", "active", factType, normalizeContextScope(candidate.Scope), strings.TrimSpace(candidate.ScopeID)).Order("updated_at desc, id desc").Limit(20).Find(&currentFacts).Error
	impact := corpusCandidateImpactDTO{
		Candidate:            candidate,
		SourceDocument:       sourceDocument,
		CurrentFacts:         currentFacts,
		BeforeMarkdown:       renderCurrentFactsMarkdown(currentFacts),
		AfterMarkdown:        renderCandidateMarkdown(candidate),
		RequiresImpactReview: corpusCandidateReviewMode(candidate) == "impact_required",
		Reason:               corpusCandidateImpactReason(candidate),
	}
	if sourceDocument != nil {
		impact.SourceMarkdown = sourceDocument.Content
	} else {
		impact.SourceMarkdown = renderCandidateProvenanceMarkdown(candidate)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"impact": impact})
}

func (s *Server) handlePublishCorpusCandidate(w http.ResponseWriter, r *http.Request) {
	id, err := pathUint(r, "id")
	if err != nil {
		http.Error(w, "invalid corpus candidate id", http.StatusBadRequest)
		return
	}
	var req publishCorpusCandidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	tx := db.DB.Begin()
	var candidate db.CorpusCandidate
	if err := tx.First(&candidate, id).Error; err != nil {
		tx.Rollback()
		http.Error(w, "corpus candidate not found", http.StatusNotFound)
		return
	}
	if candidate.Status == "accepted" {
		tx.Rollback()
		writeJSON(w, http.StatusOK, map[string]interface{}{"candidate": candidate, "reused": true})
		return
	}
	if candidate.Status != "impact_review" {
		tx.Rollback()
		http.Error(w, "candidate must complete impact review before publication", http.StatusConflict)
		return
	}
	if candidate.ImpactPreviewedAt == nil || strings.TrimSpace(candidate.ImpactPreviewedBy) == "" {
		tx.Rollback()
		http.Error(w, "candidate impact must be previewed before publication", http.StatusConflict)
		return
	}
	now := time.Now()
	actor := authenticatedActor(r)
	fact, err := contextFactFromCandidate(tx, candidate, actor, now)
	if err != nil {
		tx.Rollback()
		http.Error(w, "failed to publish corpus candidate", http.StatusInternalServerError)
		return
	}
	candidate.Status = "accepted"
	candidate.ReviewMode = "impact_required"
	candidate.AcceptedContextFactID = fact.ID
	candidate.PublishedBy = actor
	candidate.PublishedAt = &now
	candidate.UpdatedAt = now
	if note := strings.TrimSpace(req.Note); note != "" {
		if candidate.ReviewNote != "" {
			candidate.ReviewNote += "\n"
		}
		candidate.ReviewNote += note
	}
	if err := tx.Save(&candidate).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to save corpus publication", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to commit corpus publication", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"candidate": candidate, "context_fact": fact, "reused": false})
}

func applyCorpusCandidatePatch(candidate *db.CorpusCandidate, patch *corpusCandidatePatch) error {
	if candidate == nil || patch == nil {
		return nil
	}
	candidate.CandidateType = normalizeImportedCandidateType(firstNonBlank(patch.CandidateType, candidate.CandidateType))
	candidate.Scope = normalizeContextScope(firstNonBlank(patch.Scope, candidate.Scope))
	candidate.ScopeID = strings.TrimSpace(firstNonBlank(patch.ScopeID, candidate.ScopeID))
	if candidate.Scope == "global" {
		candidate.ScopeID = ""
	} else if candidate.ScopeID == "" {
		return fmt.Errorf("scope_id is required for non-global candidates")
	}
	candidate.Title = strings.TrimSpace(firstNonBlank(patch.Title, candidate.Title))
	candidate.Summary = strings.TrimSpace(firstNonBlank(patch.Summary, candidate.Summary))
	candidate.Content = strings.TrimSpace(firstNonBlank(patch.Content, candidate.Content))
	if candidate.Title == "" || candidate.Summary == "" || candidate.Content == "" {
		return fmt.Errorf("candidate title, summary, and content are required")
	}
	if strings.TrimSpace(patch.SourceAnchor) != "" {
		candidate.SourceAnchor = strings.TrimSpace(patch.SourceAnchor)
	}
	candidate.EvidenceKind = normalizeEvidenceKind(firstNonBlank(patch.EvidenceKind, candidate.EvidenceKind))
	candidate.Sensitivity = normalizeCorpusSensitivity(firstNonBlank(patch.Sensitivity, candidate.Sensitivity))
	if patch.Confidence > 0 {
		candidate.Confidence = normalizeContextScore(patch.Confidence, candidate.Confidence)
	}
	candidate.ReviewMode = corpusCandidateReviewMode(*candidate)
	return nil
}

func corpusCandidateReviewMode(candidate db.CorpusCandidate) string {
	sensitivity := strings.ToLower(strings.TrimSpace(candidate.Sensitivity))
	if sensitivity == "high" || sensitivity == "restricted" || sensitivity == "critical" {
		return "impact_required"
	}
	if normalizeEvidenceKind(candidate.EvidenceKind) == "ai_suggestion" {
		return "impact_required"
	}
	if normalizeContextScope(candidate.Scope) == "global" && contextFactTypeFromCandidate(candidate.CandidateType) == "architecture" {
		return "impact_required"
	}
	return "standard"
}

func corpusCandidateImpactReason(candidate db.CorpusCandidate) string {
	if normalizeEvidenceKind(candidate.EvidenceKind) == "ai_suggestion" {
		return "该内容包含没有原文直接依据的 AI 补充，需要对照来源与当前上下文后再发布。"
	}
	sensitivity := strings.ToLower(strings.TrimSpace(candidate.Sensitivity))
	if sensitivity == "high" || sensitivity == "restricted" || sensitivity == "critical" {
		return "该内容被标记为高敏感语料，需要确认对现有上下文和权限边界的影响。"
	}
	if normalizeContextScope(candidate.Scope) == "global" && contextFactTypeFromCandidate(candidate.CandidateType) == "architecture" {
		return "该内容会作为全局架构规则参与上下文选择，需要完成发布前影响对照。"
	}
	return "该候选属于普通语料，人工审核通过后可直接发布。"
}

func contextFactTypeFromCandidate(candidateType string) string {
	switch strings.ToLower(strings.TrimSpace(candidateType)) {
	case "delivery_case", "delivery_history":
		return "delivery_history"
	case "requirement_pattern", "workflow":
		return "workflow"
	case "risk_rule":
		return "risk_rule"
	case "feature_boundary":
		return "feature_boundary"
	case "estimation_rule":
		return "estimation_rule"
	case "glossary":
		return "glossary"
	default:
		return "architecture"
	}
}

func contextFactFromCandidate(tx *gorm.DB, candidate db.CorpusCandidate, owner string, now time.Time) (db.ContextFact, error) {
	factType := contextFactTypeFromCandidate(candidate.CandidateType)
	scope := normalizeContextScope(candidate.Scope)
	scopeID := strings.TrimSpace(candidate.ScopeID)
	if scope == "global" {
		scopeID = ""
	}
	var version int
	if err := tx.Model(&db.ContextFact{}).Where("type = ? AND scope = ? AND scope_id = ?", factType, scope, scopeID).Select("COALESCE(MAX(version), 0)").Scan(&version).Error; err != nil {
		return db.ContextFact{}, err
	}
	source := "archive"
	if candidate.ContextDocumentID > 0 {
		source = "doc"
	}
	fact := db.ContextFact{
		ContextDocumentID: candidate.ContextDocumentID,
		Type:              factType,
		Scope:             scope,
		ScopeID:           scopeID,
		Source:            source,
		Owner:             owner,
		Status:            "active",
		Version:           version + 1,
		ContentHash:       stableHash(strings.Join([]string{factType, scope, scopeID, candidate.Summary, candidate.Content}, "\n")),
		Summary:           firstNonBlank(strings.TrimSpace(candidate.Summary), strings.TrimSpace(candidate.Title)),
		Content:           strings.TrimSpace(candidate.Content),
		TokenCount:        estimateTokenCount(candidate.Summary + "\n" + candidate.Content),
		Freshness:         1,
		Confidence:        normalizeContextScore(candidate.Confidence, 0.8),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := tx.Create(&fact).Error; err != nil {
		return db.ContextFact{}, err
	}
	return fact, nil
}

func renderCurrentFactsMarkdown(facts []db.ContextFact) string {
	if len(facts) == 0 {
		return "# 当前生效上下文\n\n暂无相同类型和范围的生效语料。"
	}
	var output strings.Builder
	output.WriteString("# 当前生效上下文\n")
	for _, fact := range facts {
		output.WriteString("\n## ")
		output.WriteString(firstNonBlank(strings.TrimSpace(fact.Summary), fmt.Sprintf("Context Fact %d", fact.ID)))
		output.WriteString("\n\n")
		output.WriteString(strings.TrimSpace(fact.Content))
		output.WriteString("\n")
	}
	return output.String()
}

func renderCandidateMarkdown(candidate db.CorpusCandidate) string {
	return fmt.Sprintf("# %s\n\n%s\n\n%s", firstNonBlank(strings.TrimSpace(candidate.Title), "拟发布语料"), strings.TrimSpace(candidate.Summary), strings.TrimSpace(candidate.Content))
}

func renderCandidateProvenanceMarkdown(candidate db.CorpusCandidate) string {
	return fmt.Sprintf("# 来源证据\n\n该候选来自交付归档或系统事件。\n\n```json\n%s\n```", strings.TrimSpace(candidate.ProvenanceJSON))
}
