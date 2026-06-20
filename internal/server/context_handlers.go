package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"well-ambient/internal/config"
	"well-ambient/internal/db"

	"gorm.io/gorm"
)

const (
	defaultContextTokenBudget = 1800
	contextPackTemplateV1     = "context-pack-v1"
)

type contextFactRequest struct {
	ID                uint    `json:"id"`
	ContextDocumentID uint    `json:"context_document_id"`
	Type              string  `json:"type"`
	Scope             string  `json:"scope"`
	ScopeID           string  `json:"scope_id"`
	Source            string  `json:"source"`
	Owner             string  `json:"owner"`
	Status            string  `json:"status"`
	Version           int     `json:"version"`
	Summary           string  `json:"summary"`
	Content           string  `json:"content"`
	Freshness         float64 `json:"freshness"`
	Confidence        float64 `json:"confidence"`
}

type contextPackPreviewRequest struct {
	Text            string  `json:"text"`
	DemandText      string  `json:"demand_text"`
	TokenBudget     int     `json:"token_budget"`
	Model           string  `json:"model"`
	Provider        string  `json:"provider"`
	WorkHoursPerDay float64 `json:"work_hours_per_day"`
	Persist         bool    `json:"persist"`
	Purpose         string  `json:"purpose"`
}

type contextPackItemDTO struct {
	ID         uint    `json:"id"`
	FactID     uint    `json:"fact_id"`
	Type       string  `json:"type"`
	Scope      string  `json:"scope"`
	ScopeID    string  `json:"scope_id"`
	Source     string  `json:"source"`
	Summary    string  `json:"summary"`
	Content    string  `json:"content"`
	TokenCount int     `json:"token_count"`
	Score      float64 `json:"score"`
	Reason     string  `json:"reason"`
}

type contextPackDTO struct {
	ID            uint                 `json:"id,omitempty"`
	Summary       string               `json:"summary"`
	TokenCount    int                  `json:"token_count"`
	BudgetTokens  int                  `json:"budget_tokens"`
	CacheKey      string               `json:"cache_key"`
	ContextHash   string               `json:"context_hash"`
	Scope         string               `json:"scope_signature"`
	Items         []contextPackItemDTO `json:"items"`
	LegacyContext bool                 `json:"legacy_context"`
}

type scoredContextFact struct {
	fact   db.ContextFact
	score  float64
	reason string
}

func (s *Server) handleListContextFacts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	status := strings.TrimSpace(r.URL.Query().Get("status"))
	query := db.DB.Order("updated_at desc, id desc")
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var facts []db.ContextFact
	if err := query.Find(&facts).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to query context facts: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"facts": facts,
	})
}

func (s *Server) handleSaveContextFact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	var req contextFactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		return
	}

	fact, err := normalizeContextFactRequest(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.ID > 0 {
		var existing db.ContextFact
		if err := db.DB.First(&existing, req.ID).Error; err != nil {
			http.Error(w, "Context fact not found", http.StatusNotFound)
			return
		}
		fact.ID = existing.ID
		fact.CreatedAt = existing.CreatedAt
		if err := db.DB.Save(&fact).Error; err != nil {
			http.Error(w, fmt.Sprintf("Failed to update context fact: %v", err), http.StatusInternalServerError)
			return
		}
	} else if err := db.DB.Create(&fact).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to create context fact: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"fact":    fact,
	})
}

func (s *Server) handlePreviewContextPack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	var req contextPackPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		return
	}

	text := strings.TrimSpace(req.DemandText)
	if text == "" {
		text = strings.TrimSpace(req.Text)
	}
	if text == "" {
		http.Error(w, "Demand text is required", http.StatusBadRequest)
		return
	}

	pack, err := s.buildContextPack(db.DB, text, req.TokenBudget, req.Purpose, req.Persist)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to assemble context pack: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"pack":    pack,
	})
}

func normalizeContextFactRequest(req contextFactRequest) (db.ContextFact, error) {
	summary := strings.TrimSpace(req.Summary)
	content := strings.TrimSpace(req.Content)
	if summary == "" || content == "" {
		return db.ContextFact{}, fmt.Errorf("summary and content are required")
	}

	scope := normalizeContextScope(req.Scope)
	scopeID := strings.TrimSpace(req.ScopeID)
	if scope != "global" && scopeID == "" {
		return db.ContextFact{}, fmt.Errorf("scope_id is required for non-global context facts")
	}
	if scope == "global" {
		scopeID = ""
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" {
		status = "active"
	}
	version := req.Version
	if version <= 0 {
		version = 1
	}
	confidence := req.Confidence
	if confidence <= 0 {
		confidence = 0.85
	}
	if confidence > 1 {
		confidence = confidence / 100
	}
	if confidence > 1 {
		confidence = 1
	}

	fact := db.ContextFact{
		ContextDocumentID: req.ContextDocumentID,
		Type:              normalizeContextFactType(req.Type),
		Scope:             scope,
		ScopeID:           scopeID,
		Source:            normalizeContextSource(req.Source),
		Owner:             strings.TrimSpace(req.Owner),
		Status:            status,
		Version:           version,
		Summary:           summary,
		Content:           content,
		TokenCount:        estimateTokenCount(summary + "\n" + content),
		Freshness:         normalizeContextScore(req.Freshness, 0.85),
		Confidence:        confidence,
	}
	fact.ContentHash = stableHash(strings.Join([]string{fact.Type, fact.Scope, fact.ScopeID, fact.Source, fact.Summary, fact.Content}, "\n"))
	return fact, nil
}

func (s *Server) buildContextPack(tx *gorm.DB, demandText string, tokenBudget int, purpose string, persist bool) (contextPackDTO, error) {
	if tx == nil {
		tx = db.DB
	}
	ai := config.AIConfig{}
	if s != nil && s.config != nil {
		ai = s.config.AI
	}
	if tokenBudget <= 0 {
		tokenBudget = defaultContextTokenBudget
	}
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		purpose = "deconstruct"
	}

	facts, legacyContext := s.loadContextFacts(tx)
	scored := scoreContextFacts(facts, demandText)
	items, tokenCount := selectContextPackItems(scored, tokenBudget)
	summary := renderContextPackSummary(items, tokenCount, tokenBudget, legacyContext)
	contextHash := stableHash(summary)
	scopeSignature := contextPackScopeSignature(items)
	cacheKey := stableHash(strings.Join([]string{
		contextPackTemplateV1,
		stableHash(demandText),
		scopeSignature,
		contextHash,
		fmt.Sprintf("%d", tokenBudget),
	}, "|"))

	dto := contextPackDTO{
		Summary:       summary,
		TokenCount:    tokenCount,
		BudgetTokens:  tokenBudget,
		CacheKey:      cacheKey,
		ContextHash:   contextHash,
		Scope:         scopeSignature,
		Items:         items,
		LegacyContext: legacyContext,
	}

	if persist {
		pack := db.ContextPack{
			Purpose:               purpose,
			DemandTextHash:        stableHash(demandText),
			ScopeSignature:        scopeSignature,
			Model:                 ai.Model,
			PromptTemplateVersion: contextPackTemplateV1,
			WorkHoursPerDay:       normalizeWorkHoursPerDay(ai.DefaultWorkHoursPerDay),
			TokenBudget:           tokenBudget,
			TokenCount:            tokenCount,
			Summary:               summary,
			ContextHash:           contextHash,
			ItemCount:             len(items),
			CreatedAt:             time.Now(),
		}
		if err := tx.Create(&pack).Error; err != nil {
			return dto, err
		}
		for i, item := range items {
			packItem := db.ContextPackItem{
				ContextPackID: pack.ID,
				ContextFactID: item.FactID,
				Position:      i + 1,
				Score:         item.Score,
				TokenCount:    item.TokenCount,
				Summary:       item.Summary,
				CreatedAt:     time.Now(),
			}
			if err := tx.Create(&packItem).Error; err != nil {
				return dto, err
			}
		}
		dto.ID = pack.ID
	}

	return dto, nil
}

func (s *Server) loadContextFacts(tx *gorm.DB) ([]db.ContextFact, bool) {
	var facts []db.ContextFact
	if tx != nil {
		_ = tx.Where("status = ?", "active").Order("updated_at desc, id desc").Find(&facts).Error
	}
	for i := range facts {
		if facts[i].TokenCount <= 0 {
			facts[i].TokenCount = estimateTokenCount(facts[i].Summary + "\n" + facts[i].Content)
		}
	}
	if len(facts) > 0 {
		return facts, false
	}
	ai := config.AIConfig{}
	if s != nil && s.config != nil {
		ai = s.config.AI
	}
	return legacyAIConfigFacts(ai), true
}

func legacyAIConfigFacts(ai config.AIConfig) []db.ContextFact {
	sections := []struct {
		factType string
		summary  string
		content  string
	}{
		{"architecture", "系统架构与模块边界", ai.ProjectArchitecture},
		{"workflow", "研发流程与状态流转", ai.DeliveryWorkflow},
		{"feature_boundary", "已实现能力与约束", ai.ImplementedFeatures},
		{"estimation_rule", "团队估算口径补充", ai.EstimationGuidelines},
	}

	facts := make([]db.ContextFact, 0, len(sections))
	for _, section := range sections {
		content := strings.TrimSpace(section.content)
		if content == "" {
			continue
		}
		facts = append(facts, db.ContextFact{
			Type:        section.factType,
			Scope:       "global",
			Source:      "legacy_config",
			Status:      "active",
			Version:     1,
			Summary:     section.summary,
			Content:     content,
			TokenCount:  estimateTokenCount(section.summary + "\n" + content),
			Freshness:   0.7,
			Confidence:  0.75,
			ContentHash: stableHash(section.summary + "\n" + content),
		})
	}
	if len(facts) > 0 {
		return facts
	}

	defaults := strings.Split(formatAIDefaultProjectContext(), "\n")
	facts = make([]db.ContextFact, 0, len(defaults))
	for _, line := range defaults {
		line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "：", 2)
		summary := parts[0]
		content := line
		if len(parts) == 2 {
			content = parts[1]
		}
		facts = append(facts, db.ContextFact{
			Type:        "architecture",
			Scope:       "global",
			Source:      "system_default",
			Status:      "active",
			Version:     1,
			Summary:     summary,
			Content:     strings.TrimSpace(content),
			TokenCount:  estimateTokenCount(line),
			Freshness:   0.6,
			Confidence:  0.68,
			ContentHash: stableHash(line),
		})
	}
	return facts
}

func scoreContextFacts(facts []db.ContextFact, demandText string) []scoredContextFact {
	demandTokens := keywordSet(demandText)
	scored := make([]scoredContextFact, 0, len(facts))
	for _, fact := range facts {
		text := strings.Join([]string{fact.Type, fact.Scope, fact.ScopeID, fact.Summary, fact.Content}, " ")
		factTokens := keywordSet(text)
		overlap := 0
		for token := range demandTokens {
			if _, ok := factTokens[token]; ok {
				overlap++
			}
		}
		scopeScore := 0.05
		if fact.Scope == "global" {
			scopeScore = 0.1
		}
		confidence := fact.Confidence
		if confidence <= 0 {
			confidence = 0.75
		}
		score := 0.42 + scopeScore + float64(overlap)*0.08 + confidence*0.18 + normalizeContextScore(fact.Freshness, 0.7)*0.1
		if strings.Contains(strings.ToLower(fact.Type), "estimation") || fact.Type == "estimation_rule" {
			score += 0.08
		}
		reason := "全局基础事实"
		if overlap > 0 {
			reason = fmt.Sprintf("命中 %d 个需求关键词", overlap)
		}
		scored = append(scored, scoredContextFact{fact: fact, score: score, reason: reason})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].fact.ID < scored[j].fact.ID
		}
		return scored[i].score > scored[j].score
	})
	return scored
}

func selectContextPackItems(scored []scoredContextFact, tokenBudget int) ([]contextPackItemDTO, int) {
	items := make([]contextPackItemDTO, 0, len(scored))
	tokenCount := 0
	for _, candidate := range scored {
		fact := candidate.fact
		factTokens := fact.TokenCount
		if factTokens <= 0 {
			factTokens = estimateTokenCount(fact.Summary + "\n" + fact.Content)
		}
		if len(items) > 0 && tokenCount+factTokens > tokenBudget {
			continue
		}
		items = append(items, contextPackItemDTO{
			ID:         fact.ID,
			FactID:     fact.ID,
			Type:       fact.Type,
			Scope:      fact.Scope,
			ScopeID:    fact.ScopeID,
			Source:     fact.Source,
			Summary:    fact.Summary,
			Content:    fact.Content,
			TokenCount: factTokens,
			Score:      roundScore(candidate.score),
			Reason:     candidate.reason,
		})
		tokenCount += factTokens
		if tokenCount >= tokenBudget {
			break
		}
	}
	return items, tokenCount
}

func renderContextPackSummary(items []contextPackItemDTO, tokenCount int, tokenBudget int, legacyContext bool) string {
	var b strings.Builder
	b.WriteString("AI 需求解构上下文包\n")
	b.WriteString(fmt.Sprintf("Token 预算：%d / %d\n", tokenCount, tokenBudget))
	if legacyContext {
		b.WriteString("来源：legacy_config_fallback\n")
	} else {
		b.WriteString("来源：context_registry\n")
	}
	for i, item := range items {
		scope := item.Scope
		if item.ScopeID != "" {
			scope += ":" + item.ScopeID
		}
		b.WriteString(fmt.Sprintf("\n%d. [%s][%s][score %.2f] %s\n%s\n", i+1, item.Type, scope, item.Score, item.Summary, item.Content))
	}
	if len(items) == 0 {
		b.WriteString("\n暂无可用上下文事实。请在上下文事实注册表中维护系统架构、流程、能力边界与估算规则。\n")
	}
	return strings.TrimSpace(b.String())
}

func contextPackScopeSignature(items []contextPackItemDTO) string {
	scopes := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		scope := item.Scope
		if item.ScopeID != "" {
			scope += ":" + item.ScopeID
		}
		if scope == "" {
			scope = "global"
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	if len(scopes) == 0 {
		return "global"
	}
	return strings.Join(scopes, ",")
}

func normalizeContextFactType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "architecture"
	}
	return value
}

func normalizeContextScope(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "repo", "module", "demand_type":
		return value
	default:
		return "global"
	}
}

func normalizeContextSource(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "manual"
	}
	return value
}

func normalizeContextScore(value float64, fallback float64) float64 {
	if value <= 0 {
		return fallback
	}
	if value > 1 {
		value = value / 100
	}
	if value > 1 {
		return 1
	}
	return value
}

func estimateTokenCount(text string) int {
	runes := utf8.RuneCountInString(strings.TrimSpace(text))
	if runes == 0 {
		return 0
	}
	tokens := runes / 2
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}

var contextKeywordRegexp = regexp.MustCompile(`[A-Za-z0-9_\-]+|[\p{Han}]{2,}`)

func keywordSet(text string) map[string]struct{} {
	matches := contextKeywordRegexp.FindAllString(strings.ToLower(text), -1)
	set := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		match = strings.TrimSpace(match)
		if len([]rune(match)) < 2 {
			continue
		}
		set[match] = struct{}{}
	}
	return set
}

func stableHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func roundScore(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}
