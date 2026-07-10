package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"well-ambient/internal/db"

	"gorm.io/gorm"
)

type reviewCorpusCandidateRequest struct {
	Decision string `json:"decision"`
	Note     string `json:"note"`
}

func (s *Server) handleListCorpusCandidates(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	query := db.DB.Order("created_at desc").Limit(200)
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	var candidates []db.CorpusCandidate
	if err := query.Find(&candidates).Error; err != nil {
		http.Error(w, "failed to list corpus candidates", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": candidates})
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
	now := time.Now()
	candidate.Status = decision
	candidate.ReviewedBy = authenticatedActor(r)
	candidate.ReviewNote = strings.TrimSpace(req.Note)
	candidate.ReviewedAt = &now
	candidate.UpdatedAt = now

	var fact *db.ContextFact
	if decision == "accepted" {
		created, err := contextFactFromCandidate(tx, candidate, authenticatedActor(r), now)
		if err != nil {
			tx.Rollback()
			http.Error(w, "failed to promote corpus candidate", http.StatusInternalServerError)
			return
		}
		fact = &created
		candidate.AcceptedContextFactID = created.ID
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
	writeJSON(w, http.StatusOK, map[string]interface{}{"candidate": candidate, "context_fact": fact, "reused": false})
}

func contextFactFromCandidate(tx *gorm.DB, candidate db.CorpusCandidate, owner string, now time.Time) (db.ContextFact, error) {
	factType := map[string]string{
		"delivery_case":       "delivery_history",
		"requirement_pattern": "workflow",
		"risk_rule":           "risk_rule",
	}[candidate.CandidateType]
	if factType == "" {
		factType = "delivery_history"
	}
	var version int
	if err := tx.Model(&db.ContextFact{}).Where("type = ? AND scope = ? AND scope_id = ?", factType, candidate.Scope, candidate.ScopeID).Select("COALESCE(MAX(version), 0)").Scan(&version).Error; err != nil {
		return db.ContextFact{}, err
	}
	digest := sha256.Sum256([]byte(candidate.Content))
	fact := db.ContextFact{
		Type: factType, Scope: candidate.Scope, ScopeID: candidate.ScopeID, Source: "archive",
		Owner: owner, Status: "active", Version: version + 1, ContentHash: hex.EncodeToString(digest[:]),
		Summary: candidate.Title + " · " + candidate.Summary, Content: candidate.Content,
		TokenCount: maxInt(1, len([]rune(candidate.Content))/4), Freshness: 1, Confidence: candidate.Confidence,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.Create(&fact).Error; err != nil {
		return db.ContextFact{}, err
	}
	return fact, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
