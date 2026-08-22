package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	providerllm "well-ambient/internal/llm"
	"well-ambient/internal/readmodel"

	"gorm.io/gorm"
)

const (
	maxContextMarkdownBytes = 256 << 10
	maxContextUploadBytes   = 10 << 20
	maxContextImportFields  = 64 << 10
)

type importContextDocumentRequest struct {
	Title        string `json:"title"`
	OriginalName string `json:"original_name"`
	MimeType     string `json:"mime_type"`
	Type         string `json:"type"`
	Scope        string `json:"scope"`
	ScopeID      string `json:"scope_id"`
	Content      string `json:"content"`
}

type contextDocumentImportSource struct {
	Request     importContextDocumentRequest
	ContentHash string
	Markdown    string
	File        *providerllm.FileInput
}

type contextDocumentDTO struct {
	db.ContextDocument
	CandidateCount int `json:"candidate_count"`
	PendingCount   int `json:"pending_count"`
	PublishedCount int `json:"published_count"`
}

type corpusDocumentExtraction struct {
	DocumentSummary  string                         `json:"document_summary"`
	DocumentMarkdown string                         `json:"document_markdown"`
	Candidates       []corpusDocumentCandidateDraft `json:"candidates"`
}

type corpusDocumentCandidateDraft struct {
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

func (s *Server) handleListContextDocuments(w http.ResponseWriter, r *http.Request) {
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "all" && status != "active" && status != "archived" {
		status = "visible"
	}
	position := struct {
		UpdatedAt time.Time `json:"updated_at"`
		ID        uint      `json:"id"`
	}{}
	var items []contextDocumentDTO
	var page readPageMeta
	err := db.DB.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		window, err := readmodel.OpenPage(r.Context(), tx, readmodel.PageRequest{
			Dataset: "context_corpus", Contract: "context-documents",
			Scope: map[string]string{"status": status}, Cursor: r.URL.Query().Get("cursor"),
			Limit: r.URL.Query().Get("limit"), DefaultLimit: 50, MaxLimit: 100,
		}, &position)
		if err != nil {
			return err
		}
		query := tx.Order("updated_at desc, id desc").Limit(window.Limit + 1)
		switch status {
		case "all":
		case "active", "archived":
			query = query.Where("status = ?", status)
		default:
			query = query.Where("status <> ?", "archived")
		}
		if window.HasCursor {
			query = query.Where("updated_at < ? OR (updated_at = ? AND id < ?)", position.UpdatedAt, position.UpdatedAt, position.ID)
		}
		var documents []db.ContextDocument
		if err := query.Find(&documents).Error; err != nil {
			return err
		}
		hasMore := len(documents) > window.Limit
		if hasMore {
			documents = documents[:window.Limit]
		}
		ids := make([]uint, 0, len(documents))
		for _, document := range documents {
			ids = append(ids, document.ID)
		}
		type candidateCounts struct {
			ContextDocumentID uint
			CandidateCount    int
			PendingCount      int
			PublishedCount    int
		}
		countsByDocument := make(map[uint]candidateCounts, len(ids))
		if len(ids) > 0 {
			var counts []candidateCounts
			if err := tx.Model(&db.CorpusCandidate{}).
				Select(`context_document_id,
					COUNT(*) AS candidate_count,
					SUM(CASE WHEN status IN ('pending', 'impact_review') THEN 1 ELSE 0 END) AS pending_count,
					SUM(CASE WHEN status = 'accepted' THEN 1 ELSE 0 END) AS published_count`).
				Where("context_document_id IN ?", ids).
				Group("context_document_id").Scan(&counts).Error; err != nil {
				return err
			}
			for _, count := range counts {
				countsByDocument[count.ContextDocumentID] = count
			}
		}
		items = make([]contextDocumentDTO, 0, len(documents))
		for _, document := range documents {
			document.Content = ""
			count := countsByDocument[document.ID]
			items = append(items, contextDocumentDTO{
				ContextDocument: document,
				CandidateCount:  count.CandidateCount,
				PendingCount:    count.PendingCount,
				PublishedCount:  count.PublishedCount,
			})
		}
		last := position
		if len(documents) > 0 {
			last.UpdatedAt = documents[len(documents)-1].UpdatedAt
			last.ID = documents[len(documents)-1].ID
		}
		page, err = buildReadPageMeta(window, hasMore, last)
		return err
	})
	if err != nil {
		if errors.Is(err, readmodel.ErrStaleCursor) || errors.Is(err, readmodel.ErrCursorScope) || errors.Is(err, readmodel.ErrInvalidCursor) || strings.Contains(err.Error(), "limit") {
			writeReadPageError(w, err)
			return
		}
		http.Error(w, "failed to list context documents", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items, "page": page})
}

func (s *Server) handleGetContextDocument(w http.ResponseWriter, r *http.Request) {
	id, err := pathUint(r, "id")
	if err != nil {
		http.Error(w, "invalid context document id", http.StatusBadRequest)
		return
	}
	var document db.ContextDocument
	if err := db.DB.First(&document, id).Error; err != nil {
		http.Error(w, "context document not found", http.StatusNotFound)
		return
	}
	var candidates []db.CorpusCandidate
	if err := db.DB.WithContext(r.Context()).Where("context_document_id = ?", document.ID).
		Order("created_at desc, id desc").Limit(101).Find(&candidates).Error; err != nil {
		http.Error(w, "failed to load bounded document candidates", http.StatusInternalServerError)
		return
	}
	hasMore := len(candidates) > 100
	if hasMore {
		candidates = candidates[:100]
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"document":   document,
		"candidates": candidates,
		"candidate_page": map[string]interface{}{
			"limit": 100, "has_more": hasMore,
			"continuation_endpoint": fmt.Sprintf("/api/corpus-candidates?document_id=%d", document.ID),
		},
	})
}

func (s *Server) handleImportContextDocument(w http.ResponseWriter, r *http.Request) {
	source, status, err := decodeContextDocumentImport(w, r)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}
	req := source.Request
	originalName, mimeType, err := normalizeContextDocumentFile(req.OriginalName, req.MimeType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = strings.TrimSuffix(originalName, filepath.Ext(originalName))
	}
	if title == "" {
		title = "未命名系统设计资料"
	}
	scope := normalizeContextScope(req.Scope)
	scopeID := strings.TrimSpace(req.ScopeID)
	if scope == "global" {
		scopeID = ""
	} else if scopeID == "" {
		http.Error(w, "scope_id is required for non-global documents", http.StatusBadRequest)
		return
	}
	contentHash := source.ContentHash
	var reused db.ContextDocument
	if err := db.DB.Where("content_hash = ? AND scope = ? AND scope_id = ? AND ingestion_status = ?", contentHash, scope, scopeID, "completed").Order("id desc").First(&reused).Error; err == nil {
		var existing []db.CorpusCandidate
		_ = db.DB.Where("context_document_id = ?", reused.ID).Order("id asc").Find(&existing).Error
		writeJSON(w, http.StatusOK, map[string]interface{}{"document": reused, "candidates": existing, "reused": true})
		return
	}

	var previous db.ContextDocument
	version := 1
	parentDocumentID := uint(0)
	if err := db.DB.Where("title = ? AND scope = ? AND scope_id = ?", title, scope, scopeID).Order("version desc, id desc").First(&previous).Error; err == nil {
		version = previous.Version + 1
		parentDocumentID = previous.ID
	} else if err != gorm.ErrRecordNotFound {
		http.Error(w, "failed to resolve document version", http.StatusInternalServerError)
		return
	}
	now := time.Now()
	actor := authenticatedActor(r)
	document := db.ContextDocument{
		ParentDocumentID: parentDocumentID,
		Title:            title,
		OriginalName:     originalName,
		MimeType:         mimeType,
		Type:             firstNonBlank(strings.TrimSpace(req.Type), "system_design"),
		Scope:            scope,
		ScopeID:          scopeID,
		Source:           "doc",
		Owner:            actor,
		ImportedBy:       actor,
		Status:           "active",
		IngestionStatus:  "processing",
		Version:          version,
		ContentHash:      contentHash,
		Summary:          title,
		Content:          "",
		TokenCount:       0,
		Freshness:        1,
		Confidence:       1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.DB.Create(&document).Error; err != nil {
		http.Error(w, "failed to persist context document", http.StatusInternalServerError)
		return
	}

	if source.File != nil {
		source.File.Name = originalName
		source.File.MIMEType = mimeType
	}
	extraction, err := extractCorpusCandidatesFromDocument(r.Context(), s.config, document, source)
	if err != nil {
		markContextDocumentIngestionFailed(document.ID, err)
		document.IngestionStatus = "failed"
		document.IngestionError = err.Error()
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{"document": document, "error": err.Error()})
		return
	}
	candidates, err := persistDocumentExtraction(document, extraction, actor, s.config)
	if err != nil {
		markContextDocumentIngestionFailed(document.ID, err)
		http.Error(w, "failed to persist corpus candidates", http.StatusInternalServerError)
		return
	}
	if err := db.DB.First(&document, document.ID).Error; err != nil {
		http.Error(w, "failed to reload context document", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"document": document, "candidates": candidates, "reused": false})
}

func decodeContextDocumentImport(w http.ResponseWriter, r *http.Request) (contextDocumentImportSource, int, error) {
	contentType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if contentType != "multipart/form-data" {
		r.Body = http.MaxBytesReader(w, r.Body, maxContextMarkdownBytes+(32<<10))
		var req importContextDocumentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return contextDocumentImportSource{}, http.StatusBadRequest, fmt.Errorf("invalid JSON body")
		}
		markdown := strings.TrimSpace(req.Content)
		if markdown == "" {
			return contextDocumentImportSource{}, http.StatusBadRequest, fmt.Errorf("Markdown content is required")
		}
		if len([]byte(markdown)) > maxContextMarkdownBytes {
			return contextDocumentImportSource{}, http.StatusRequestEntityTooLarge, fmt.Errorf("Markdown content exceeds the 256 KiB ingestion limit")
		}
		return contextDocumentImportSource{
			Request: req, ContentHash: stableHash(markdown), Markdown: markdown,
		}, http.StatusOK, nil
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxContextUploadBytes+(1<<20))
	reader, err := r.MultipartReader()
	if err != nil {
		return contextDocumentImportSource{}, http.StatusBadRequest, fmt.Errorf("invalid multipart body")
	}
	var req importContextDocumentRequest
	var fileInput *providerllm.FileInput
	for {
		part, nextErr := reader.NextPart()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return contextDocumentImportSource{}, http.StatusBadRequest, fmt.Errorf("read multipart body: %w", nextErr)
		}
		name := part.FormName()
		if name == "file" {
			if fileInput != nil {
				_ = part.Close()
				return contextDocumentImportSource{}, http.StatusBadRequest, fmt.Errorf("only one source file is allowed")
			}
			data, readErr := io.ReadAll(io.LimitReader(part, maxContextUploadBytes+1))
			_ = part.Close()
			if readErr != nil {
				return contextDocumentImportSource{}, http.StatusBadRequest, fmt.Errorf("read source file: %w", readErr)
			}
			if len(data) == 0 {
				return contextDocumentImportSource{}, http.StatusBadRequest, fmt.Errorf("source file is empty")
			}
			if len(data) > maxContextUploadBytes {
				return contextDocumentImportSource{}, http.StatusRequestEntityTooLarge, fmt.Errorf("source file exceeds the 10 MiB ingestion limit")
			}
			fileInput = &providerllm.FileInput{Name: part.FileName(), MIMEType: part.Header.Get("Content-Type"), Data: data}
			continue
		}
		value, readErr := io.ReadAll(io.LimitReader(part, maxContextImportFields+1))
		_ = part.Close()
		if readErr != nil {
			return contextDocumentImportSource{}, http.StatusBadRequest, fmt.Errorf("read import field: %w", readErr)
		}
		if len(value) > maxContextImportFields {
			return contextDocumentImportSource{}, http.StatusRequestEntityTooLarge, fmt.Errorf("import metadata is too large")
		}
		switch name {
		case "title":
			req.Title = string(value)
		case "original_name":
			req.OriginalName = string(value)
		case "mime_type":
			req.MimeType = string(value)
		case "type":
			req.Type = string(value)
		case "scope":
			req.Scope = string(value)
		case "scope_id":
			req.ScopeID = string(value)
		}
	}
	if fileInput == nil {
		return contextDocumentImportSource{}, http.StatusBadRequest, fmt.Errorf("source file is required")
	}
	// The multipart file part is authoritative. Metadata fields must not be able
	// to disguise a different extension or MIME type.
	req.OriginalName = fileInput.Name
	req.MimeType = fileInput.MIMEType
	return contextDocumentImportSource{
		Request: req, ContentHash: hashContextDocumentBytes(fileInput.Data), File: fileInput,
	}, http.StatusOK, nil
}

func hashContextDocumentBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *Server) handleArchiveContextDocument(w http.ResponseWriter, r *http.Request) {
	id, err := pathUint(r, "id")
	if err != nil {
		http.Error(w, "invalid context document id", http.StatusBadRequest)
		return
	}
	tx := db.DB.Begin()
	var document db.ContextDocument
	if err := tx.First(&document, id).Error; err != nil {
		tx.Rollback()
		http.Error(w, "context document not found", http.StatusNotFound)
		return
	}
	now := time.Now()
	actor := authenticatedActor(r)
	if document.Status != "archived" {
		document.Status = "archived"
		document.UpdatedAt = now
		if err := tx.Save(&document).Error; err != nil {
			tx.Rollback()
			http.Error(w, "failed to archive context document", http.StatusInternalServerError)
			return
		}
	}
	var unpublished []db.CorpusCandidate
	if err := tx.Where("context_document_id = ? AND status IN ?", document.ID, []string{"pending", "impact_review"}).Find(&unpublished).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to load unpublished document candidates", http.StatusInternalServerError)
		return
	}
	for index := range unpublished {
		unpublished[index].Status = "rejected"
		unpublished[index].ReviewedBy = actor
		unpublished[index].ReviewedAt = &now
		unpublished[index].ImpactPreviewedBy = ""
		unpublished[index].ImpactPreviewedAt = nil
		unpublished[index].UpdatedAt = now
		if unpublished[index].ReviewNote != "" {
			unpublished[index].ReviewNote += "\n"
		}
		unpublished[index].ReviewNote += "Source document archived before publication."
		if err := tx.Save(&unpublished[index]).Error; err != nil {
			tx.Rollback()
			http.Error(w, "failed to withdraw unpublished document candidates", http.StatusInternalServerError)
			return
		}
	}
	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to commit context document archive", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"document": document, "withdrawn_candidate_count": len(unpublished)})
}

func normalizeContextDocumentFile(originalName, mimeType string) (string, string, error) {
	originalName = strings.TrimSpace(filepath.Base(originalName))
	if originalName == "" || originalName == "." {
		originalName = "pasted-markdown.md"
	}
	ext := strings.ToLower(filepath.Ext(originalName))
	defaultMIME, supported := map[string]string{
		".md": "text/markdown", ".markdown": "text/markdown", ".txt": "text/plain",
		".json": "application/json", ".html": "text/html", ".htm": "text/html", ".xml": "application/xml",
		".pdf": "application/pdf", ".doc": "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document", ".rtf": "application/rtf",
		".odt": "application/vnd.oasis.opendocument.text", ".ppt": "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".csv":  "text/csv", ".tsv": "text/tab-separated-values", ".xls": "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}[ext]
	if !supported {
		return "", "", fmt.Errorf("file type %s is not supported for document ingestion", firstNonBlank(ext, "(none)"))
	}
	mimeType = strings.TrimSpace(mimeType)
	if parsed, _, parseErr := mime.ParseMediaType(mimeType); parseErr == nil && parsed != "" {
		mimeType = parsed
	} else {
		mimeType = defaultMIME
	}
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = defaultMIME
	}
	return originalName, mimeType, nil
}

func extractCorpusCandidatesFromDocument(ctx context.Context, cfg *config.Config, document db.ContextDocument, source contextDocumentImportSource) (corpusDocumentExtraction, error) {
	if cfg == nil || !cfg.AI.Enabled || strings.TrimSpace(cfg.AI.APIToken) == "" || strings.TrimSpace(cfg.AI.BaseURL) == "" {
		return corpusDocumentExtraction{}, fmt.Errorf("AI configuration is not enabled or is missing credentials")
	}
	systemPrompt := `你是系统设计语料治理助手。读取用户消息中附带的文件或 Markdown，将资料解析、整理为可审核的规范 Markdown，并拆成可独立审核、可追溯的原子语料候选。只返回 JSON，不要代码围栏或说明文字。

返回结构：
{"document_summary":"一句话概括","document_markdown":"完整、忠实、结构清晰的 Markdown 解析稿","candidates":[{"candidate_type":"architecture|workflow|feature_boundary|estimation_rule|glossary|risk_rule|delivery_history","scope":"global|repo|module|demand_type","scope_id":"非 global 时必填","title":"候选标题","summary":"候选摘要","content":"保留语义的 Markdown 内容","source_anchor":"对应解析稿标题或段落定位","evidence_kind":"source_fact|source_rewrite|ai_suggestion","confidence":0.0,"sensitivity":"normal|internal|high|restricted"}]}

规则：document_markdown 必须覆盖资料中的有效正文，不得只返回摘要；每条 source_fact/source_rewrite 必须能定位到解析稿；仅改善表达标记 source_rewrite；资料没有依据的补充必须标记 ai_suggestion；不得把推断伪装成事实；涉及凭证、个人数据、安全边界或全局架构约束时提高 sensitivity；最多返回 24 条候选。`
	userPrompt := fmt.Sprintf("默认范围：%s:%s\n资料标题：%s\n原始文件：%s\nMIME：%s", document.Scope, document.ScopeID, document.Title, document.OriginalName, document.MimeType)
	request := providerllm.Request{SystemPrompt: systemPrompt, UserPrompt: userPrompt, MaxOutputTokens: 7000}
	if source.File != nil {
		request.Files = []providerllm.FileInput{*source.File}
	} else {
		request.UserPrompt += "\n\nMarkdown：\n" + source.Markdown
	}
	client := providerllm.Client{Config: cfg.AI}
	raw, err := client.Generate(ctx, request)
	if err != nil {
		return corpusDocumentExtraction{}, err
	}
	jsonText := strings.TrimSpace(raw)
	if start := strings.Index(jsonText, "{"); start >= 0 {
		if end := strings.LastIndex(jsonText, "}"); end >= start {
			jsonText = jsonText[start : end+1]
		}
	}
	var extraction corpusDocumentExtraction
	if err := json.Unmarshal([]byte(jsonText), &extraction); err != nil {
		return corpusDocumentExtraction{}, fmt.Errorf("decode LLM corpus extraction: %w", err)
	}
	if len(extraction.Candidates) == 0 {
		return corpusDocumentExtraction{}, fmt.Errorf("LLM returned no corpus candidates")
	}
	if strings.TrimSpace(extraction.DocumentMarkdown) == "" {
		return corpusDocumentExtraction{}, fmt.Errorf("LLM returned no parsed document Markdown")
	}
	if len(extraction.Candidates) > 24 {
		extraction.Candidates = extraction.Candidates[:24]
	}
	return extraction, nil
}

func persistDocumentExtraction(document db.ContextDocument, extraction corpusDocumentExtraction, actor string, cfg *config.Config) ([]db.CorpusCandidate, error) {
	now := time.Now()
	model := ""
	if cfg != nil {
		model = strings.TrimSpace(cfg.AI.Model)
	}
	candidates := make([]db.CorpusCandidate, 0, len(extraction.Candidates))
	for _, draft := range extraction.Candidates {
		content := strings.TrimSpace(draft.Content)
		if content == "" {
			continue
		}
		candidateType := normalizeImportedCandidateType(draft.CandidateType)
		scope := normalizeContextScope(firstNonBlank(draft.Scope, document.Scope))
		scopeID := strings.TrimSpace(firstNonBlank(draft.ScopeID, document.ScopeID))
		if scope == "global" {
			scopeID = ""
		} else if scopeID == "" {
			scope = document.Scope
			scopeID = document.ScopeID
			if scope != "global" && scopeID == "" {
				scope = "global"
			}
		}
		evidenceKind := normalizeEvidenceKind(draft.EvidenceKind)
		sensitivity := normalizeCorpusSensitivity(draft.Sensitivity)
		provenance, _ := json.Marshal(map[string]interface{}{
			"context_document_id": document.ID,
			"content_hash":        document.ContentHash,
			"original_name":       document.OriginalName,
			"source_anchor":       strings.TrimSpace(draft.SourceAnchor),
			"evidence_kind":       evidenceKind,
			"ai_model":            model,
			"imported_by":         actor,
		})
		candidate := db.CorpusCandidate{
			ContextDocumentID: document.ID,
			CandidateType:     candidateType,
			Scope:             scope,
			ScopeID:           scopeID,
			Title:             firstNonBlank(strings.TrimSpace(draft.Title), strings.TrimSpace(draft.Summary), document.Title),
			Summary:           firstNonBlank(strings.TrimSpace(draft.Summary), strings.TrimSpace(draft.Title), document.Summary),
			Content:           content,
			SourceAnchor:      strings.TrimSpace(draft.SourceAnchor),
			EvidenceKind:      evidenceKind,
			AIModel:           model,
			ProvenanceJSON:    string(provenance),
			Confidence:        normalizeContextScore(draft.Confidence, 0.8),
			Sensitivity:       sensitivity,
			Status:            "pending",
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		candidate.ReviewMode = corpusCandidateReviewMode(candidate)
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("LLM candidates did not contain publishable content")
	}
	tx := db.DB.Begin()
	for index := range candidates {
		if err := tx.Create(&candidates[index]).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	document.Summary = firstNonBlank(strings.TrimSpace(extraction.DocumentSummary), document.Title)
	document.Content = strings.TrimSpace(extraction.DocumentMarkdown)
	document.TokenCount = estimateTokenCount(document.Content)
	document.IngestionStatus = "completed"
	document.IngestionError = ""
	document.UpdatedAt = now
	if err := tx.Save(&document).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return candidates, nil
}

func markContextDocumentIngestionFailed(documentID uint, cause error) {
	message := strings.TrimSpace(cause.Error())
	if len(message) > 2000 {
		message = message[:2000]
	}
	_ = db.DB.Model(&db.ContextDocument{}).Where("id = ?", documentID).Updates(map[string]interface{}{
		"ingestion_status": "failed",
		"ingestion_error":  message,
		"updated_at":       time.Now(),
	}).Error
}

func normalizeImportedCandidateType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "workflow", "requirement_pattern":
		return "workflow"
	case "feature_boundary":
		return "feature_boundary"
	case "estimation_rule":
		return "estimation_rule"
	case "glossary":
		return "glossary"
	case "risk_rule":
		return "risk_rule"
	case "delivery_history", "delivery_case":
		return "delivery_history"
	default:
		return "architecture"
	}
}

func normalizeEvidenceKind(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "source_rewrite":
		return "source_rewrite"
	case "ai_suggestion":
		return "ai_suggestion"
	default:
		return "source_fact"
	}
}

func normalizeCorpusSensitivity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "high", "restricted", "critical":
		return strings.ToLower(strings.TrimSpace(value))
	case "internal":
		return "internal"
	default:
		return "normal"
	}
}
