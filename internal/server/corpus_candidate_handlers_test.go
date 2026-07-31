package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/delivery"
)

func TestCorpusCandidatesRequireReviewBeforeContextPromotion(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "curator@example.com", "Corpus Curator", nil)
	srv := NewServer(&config.Config{}, "")
	spec := db.DemandSpecVersion{
		DemandID: "DEMAND-CORPUS", Version: 1, Status: delivery.SpecFrozen,
		Summary: "Reviewed delivery", UserGoal: "Promote only curated facts", ContextPackID: 11,
		AcceptanceCriteriaJSON: `["candidate is reviewed"]`, TestPlanJSON: `["go test ./..."]`, TasksJSON: `[]`, RisksJSON: `[]`,
	}
	if err := db.DB.Create(&spec).Error; err != nil {
		t.Fatalf("seed spec: %v", err)
	}
	run := db.ExecutionRun{
		RunKey: "corpus-run", DemandID: spec.DemandID, DemandSpecVersionID: spec.ID, Repo: "backend-core",
		Status: delivery.RunDelivered, PipelineStatus: "success", AcceptanceState: "accepted", MRURL: "https://gitlab/mr/3",
	}
	if err := db.DB.Create(&run).Error; err != nil {
		t.Fatalf("seed run: %v", err)
	}
	candidates, err := delivery.EnsureCorpusCandidates(db.DB, run)
	if err != nil {
		t.Fatalf("ensure candidates: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidate count=%d, want 2", len(candidates))
	}
	if _, err := delivery.EnsureCorpusCandidates(db.DB, run); err != nil {
		t.Fatalf("idempotent ensure: %v", err)
	}
	var candidateCount int64
	_ = db.DB.Model(&db.CorpusCandidate{}).Count(&candidateCount).Error
	if candidateCount != 2 {
		t.Fatalf("idempotent candidate count=%d", candidateCount)
	}
	var factCount int64
	_ = db.DB.Model(&db.ContextFact{}).Count(&factCount).Error
	if factCount != 0 {
		t.Fatalf("pending candidates must not create context facts, got %d", factCount)
	}

	listReq := authenticatedJSONRequest(t, srv, token, http.MethodGet, "/api/corpus-candidates?status=pending", nil)
	if listReq.Code != http.StatusOK {
		t.Fatalf("list candidates status=%d body=%s", listReq.Code, listReq.Body.String())
	}
	var listed struct {
		Items []db.CorpusCandidate `json:"items"`
	}
	if err := json.NewDecoder(listReq.Body).Decode(&listed); err != nil || len(listed.Items) != 2 {
		t.Fatalf("listed candidates=%+v err=%v", listed.Items, err)
	}

	accepted := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/corpus-candidates/"+itoa(listed.Items[0].ID)+"/review", reviewCorpusCandidateRequest{Decision: "accepted", Note: "verified"})
	if accepted.Code != http.StatusOK {
		t.Fatalf("accept candidate status=%d body=%s", accepted.Code, accepted.Body.String())
	}
	var promoted db.CorpusCandidate
	if err := db.DB.First(&promoted, listed.Items[0].ID).Error; err != nil {
		t.Fatalf("reload candidate: %v", err)
	}
	if promoted.Status != "accepted" || promoted.AcceptedContextFactID == 0 {
		t.Fatalf("candidate not promoted: %+v", promoted)
	}
	var fact db.ContextFact
	if err := db.DB.First(&fact, promoted.AcceptedContextFactID).Error; err != nil {
		t.Fatalf("load promoted fact: %v", err)
	}
	if fact.Status != "active" || fact.Source != "archive" || fact.ScopeID != "backend-core" {
		t.Fatalf("unexpected promoted fact: %+v", fact)
	}

	rejected := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/corpus-candidates/"+itoa(listed.Items[1].ID)+"/review", reviewCorpusCandidateRequest{Decision: "rejected", Note: "not reusable"})
	if rejected.Code != http.StatusOK {
		t.Fatalf("reject candidate status=%d body=%s", rejected.Code, rejected.Body.String())
	}
	var rejectedCandidate db.CorpusCandidate
	if err := db.DB.First(&rejectedCandidate, listed.Items[1].ID).Error; err != nil {
		t.Fatalf("reload rejected candidate: %v", err)
	}
	if rejectedCandidate.Status != "rejected" || rejectedCandidate.AcceptedContextFactID != 0 {
		t.Fatalf("rejected candidate must stay outside active context: %+v", rejectedCandidate)
	}
	_ = db.DB.Model(&db.ContextFact{}).Count(&factCount).Error
	if factCount != 1 {
		t.Fatalf("rejected candidate created an unexpected fact, got %d facts", factCount)
	}

	reused := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/corpus-candidates/"+itoa(promoted.ID)+"/review", reviewCorpusCandidateRequest{Decision: "rejected"})
	if reused.Code != http.StatusOK || !strings.Contains(reused.Body.String(), `"reused":true`) {
		t.Fatalf("review should be idempotent: %d %s", reused.Code, reused.Body.String())
	}
}

func TestContextDocumentImportAndGovernedPublication(t *testing.T) {
	setupServerTestDB(t)
	sourceBody := []byte("# OPAQUE_UPLOAD_SENTINEL\n\nThese exact bytes must only appear in the input_file block.")
	providerCalls := 0
	extraction := map[string]interface{}{
		"document_summary":  "订单服务的架构与发布流程",
		"document_markdown": "# 订单系统设计\n\n## 架构约束\n\n订单事件必须携带版本号。\n\n## 发布流程\n\n发布前完成回归与回滚检查。",
		"candidates": []map[string]interface{}{
			{
				"candidate_type": "workflow",
				"scope":          "repo",
				"scope_id":       "orders-service",
				"title":          "订单服务发布检查",
				"summary":        "发布前必须完成回归和回滚检查",
				"content":        "## 发布检查\n\n- 完成回归测试\n- 验证回滚脚本",
				"source_anchor":  "发布流程",
				"evidence_kind":  "source_fact",
				"confidence":     0.94,
				"sensitivity":    "normal",
			},
			{
				"candidate_type": "architecture",
				"scope":          "global",
				"title":          "全局事件契约",
				"summary":        "所有订单事件必须携带版本号",
				"content":        "## 全局事件契约\n\n订单事件必须携带 `schema_version`。",
				"source_anchor":  "架构约束",
				"evidence_kind":  "source_rewrite",
				"confidence":     0.88,
				"sensitivity":    "internal",
			},
		},
	}
	extractionJSON, _ := json.Marshal(extraction)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerCalls++
		var payload struct {
			Input []struct {
				Content []struct {
					Type     string `json:"type"`
					Text     string `json:"text"`
					Filename string `json:"filename"`
					FileData string `json:"file_data"`
				} `json:"content"`
			} `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Input) != 1 || len(payload.Input[0].Content) != 2 {
			t.Fatalf("provider input = %#v", payload)
		}
		filePart := payload.Input[0].Content[0]
		if filePart.Type != "input_file" || filePart.Filename != "orders.md" {
			t.Fatalf("provider file part = %#v", filePart)
		}
		const prefix = "data:text/markdown;base64,"
		if !strings.HasPrefix(filePart.FileData, prefix) {
			t.Fatalf("provider file data prefix = %q", filePart.FileData)
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(filePart.FileData, prefix))
		if err != nil || !bytes.Equal(decoded, sourceBody) {
			t.Fatalf("provider bytes changed: got=%q err=%v", decoded, err)
		}
		textPart := payload.Input[0].Content[1]
		if textPart.Type != "input_text" || strings.Contains(textPart.Text, "OPAQUE_UPLOAD_SENTINEL") {
			t.Fatalf("uploaded body leaked into prompt: %#v", textPart)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"output": []map[string]interface{}{{
				"type":    "message",
				"content": []map[string]interface{}{{"type": "output_text", "text": string(extractionJSON)}},
			}},
		})
	}))
	defer provider.Close()

	token := superAdminToken(t, "knowledge-owner@example.com", "Knowledge Owner", []string{
		"ai_context:read", "ai_context:write", "ai_context:preview", "corpus_candidate:read", "corpus_candidate:review",
	})
	srv := NewServer(&config.Config{AI: config.AIConfig{
		Enabled: true, BaseURL: provider.URL, EndpointType: "responses", APIToken: "token", Model: "governance-model",
	}}, "")
	imported := authenticatedContextDocumentMultipartRequest(t, srv, token, map[string]string{
		"title": "订单系统设计", "scope": "repo", "scope_id": "orders-service", "type": "system_design",
	}, "orders.md", sourceBody)
	if imported.Code != http.StatusCreated {
		t.Fatalf("import context document status=%d body=%s", imported.Code, imported.Body.String())
	}
	var importResponse struct {
		Document   db.ContextDocument   `json:"document"`
		Candidates []db.CorpusCandidate `json:"candidates"`
	}
	if err := json.NewDecoder(imported.Body).Decode(&importResponse); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	if importResponse.Document.IngestionStatus != "completed" || importResponse.Document.OriginalName != "orders.md" || len(importResponse.Candidates) != 2 {
		t.Fatalf("unexpected document import response: %+v", importResponse)
	}
	if strings.Contains(importResponse.Document.Content, "OPAQUE_UPLOAD_SENTINEL") || !strings.Contains(importResponse.Document.Content, "订单事件必须携带版本号") {
		t.Fatalf("document must persist only LLM parsed Markdown: %q", importResponse.Document.Content)
	}
	var workflow, architecture db.CorpusCandidate
	for _, candidate := range importResponse.Candidates {
		switch candidate.CandidateType {
		case "workflow":
			workflow = candidate
		case "architecture":
			architecture = candidate
		}
	}
	if workflow.ID == 0 || workflow.ReviewMode != "standard" {
		t.Fatalf("workflow candidate should use standard review: %+v", workflow)
	}
	if architecture.ID == 0 || architecture.ReviewMode != "impact_required" {
		t.Fatalf("global architecture candidate should require impact review: %+v", architecture)
	}
	var factCount int64
	_ = db.DB.Model(&db.ContextFact{}).Count(&factCount).Error
	if factCount != 0 {
		t.Fatalf("LLM import must not create active facts, got %d", factCount)
	}

	acceptedWorkflow := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/corpus-candidates/"+itoa(workflow.ID)+"/review", reviewCorpusCandidateRequest{Decision: "accepted", Note: "source checked"})
	if acceptedWorkflow.Code != http.StatusOK || !strings.Contains(acceptedWorkflow.Body.String(), `"requires_impact_review":false`) {
		t.Fatalf("standard review status=%d body=%s", acceptedWorkflow.Code, acceptedWorkflow.Body.String())
	}
	var promotedWorkflow db.CorpusCandidate
	_ = db.DB.First(&promotedWorkflow, workflow.ID).Error
	if promotedWorkflow.Status != "accepted" || promotedWorkflow.AcceptedContextFactID == 0 {
		t.Fatalf("standard candidate was not published: %+v", promotedWorkflow)
	}

	approvedArchitecture := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/corpus-candidates/"+itoa(architecture.ID)+"/review", reviewCorpusCandidateRequest{Decision: "accepted", Note: "needs impact preview"})
	if approvedArchitecture.Code != http.StatusOK || !strings.Contains(approvedArchitecture.Body.String(), `"requires_impact_review":true`) {
		t.Fatalf("impact review transition status=%d body=%s", approvedArchitecture.Code, approvedArchitecture.Body.String())
	}
	var gatedArchitecture db.CorpusCandidate
	_ = db.DB.First(&gatedArchitecture, architecture.ID).Error
	if gatedArchitecture.Status != "impact_review" || gatedArchitecture.AcceptedContextFactID != 0 {
		t.Fatalf("global architecture must remain unpublished before impact approval: %+v", gatedArchitecture)
	}
	directPublish := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/corpus-candidates/"+itoa(architecture.ID)+"/publish", publishCorpusCandidateRequest{Note: "must not bypass preview"})
	if directPublish.Code != http.StatusConflict {
		t.Fatalf("publish without impact preview status=%d body=%s", directPublish.Code, directPublish.Body.String())
	}

	impact := authenticatedJSONRequest(t, srv, token, http.MethodGet, "/api/corpus-candidates/"+itoa(architecture.ID)+"/impact", nil)
	if impact.Code != http.StatusOK || !strings.Contains(impact.Body.String(), "订单事件必须携带版本号") || !strings.Contains(impact.Body.String(), `"requires_impact_review":true`) {
		t.Fatalf("impact preview status=%d body=%s", impact.Code, impact.Body.String())
	}

	publishedArchitecture := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/corpus-candidates/"+itoa(architecture.ID)+"/publish", publishCorpusCandidateRequest{Note: "impact confirmed"})
	if publishedArchitecture.Code != http.StatusOK {
		t.Fatalf("publish gated candidate status=%d body=%s", publishedArchitecture.Code, publishedArchitecture.Body.String())
	}
	_ = db.DB.First(&gatedArchitecture, architecture.ID).Error
	if gatedArchitecture.Status != "accepted" || gatedArchitecture.AcceptedContextFactID == 0 || gatedArchitecture.ImpactPreviewedAt == nil || gatedArchitecture.PublishedAt == nil {
		t.Fatalf("gated candidate was not published: %+v", gatedArchitecture)
	}
	var architectureFact db.ContextFact
	if err := db.DB.First(&architectureFact, gatedArchitecture.AcceptedContextFactID).Error; err != nil {
		t.Fatalf("load published architecture fact: %v", err)
	}
	if architectureFact.Status != "active" || architectureFact.ContextDocumentID != importResponse.Document.ID || architectureFact.Source != "doc" {
		t.Fatalf("unexpected published architecture fact: %+v", architectureFact)
	}

	documents := authenticatedJSONRequest(t, srv, token, http.MethodGet, "/api/context/documents", nil)
	if documents.Code != http.StatusOK || !strings.Contains(documents.Body.String(), `"candidate_count":2`) || !strings.Contains(documents.Body.String(), `"published_count":2`) {
		t.Fatalf("document management list status=%d body=%s", documents.Code, documents.Body.String())
	}

	reused := authenticatedContextDocumentMultipartRequest(t, srv, token, map[string]string{
		"title": "订单系统设计", "scope": "repo", "scope_id": "orders-service", "type": "system_design",
	}, "orders.md", sourceBody)
	if reused.Code != http.StatusOK || !strings.Contains(reused.Body.String(), `"reused":true`) {
		t.Fatalf("duplicate document should reuse completed extraction: status=%d body=%s", reused.Code, reused.Body.String())
	}
	if providerCalls != 1 {
		t.Fatalf("duplicate document called provider %d times, want 1", providerCalls)
	}
}

func TestContextDocumentFileImportFailurePersistsNoUploadedBody(t *testing.T) {
	setupServerTestDB(t)
	sourceBody := []byte("# FAILED_UPLOAD_SENTINEL\n\nThis body must never be persisted.")
	token := superAdminToken(t, "failed-import@example.com", "Failed Import", []string{"ai_context:write"})
	srv := NewServer(&config.Config{}, "")

	failed := authenticatedContextDocumentMultipartRequest(t, srv, token, map[string]string{
		"title": "失败导入", "scope": "global", "type": "system_design",
	}, "failed.md", sourceBody)
	if failed.Code != http.StatusBadGateway {
		t.Fatalf("failed import status=%d body=%s", failed.Code, failed.Body.String())
	}
	var response struct {
		Document db.ContextDocument `json:"document"`
	}
	if err := json.NewDecoder(failed.Body).Decode(&response); err != nil {
		t.Fatalf("decode failed import: %v", err)
	}
	if response.Document.ID == 0 || response.Document.IngestionStatus != "failed" || response.Document.Content != "" {
		t.Fatalf("failed document retained unexpected state: %+v", response.Document)
	}
	var stored db.ContextDocument
	if err := db.DB.First(&stored, response.Document.ID).Error; err != nil {
		t.Fatalf("reload failed document: %v", err)
	}
	if stored.Content != "" || strings.Contains(stored.Summary, "FAILED_UPLOAD_SENTINEL") || strings.Contains(stored.IngestionError, "FAILED_UPLOAD_SENTINEL") {
		t.Fatalf("uploaded body leaked into failed document: %+v", stored)
	}
}

func TestDecodeContextDocumentImportPreservesManualMarkdownPath(t *testing.T) {
	body := `{"title":"手工修正","original_name":"manual.md","scope":"global","content":"# 手工语料\n\n仅用于小范围修正。"}`
	req := httptest.NewRequest(http.MethodPost, "/api/context/documents/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	source, status, err := decodeContextDocumentImport(httptest.NewRecorder(), req)
	if err != nil || status != http.StatusOK {
		t.Fatalf("decode manual Markdown status=%d err=%v", status, err)
	}
	if source.File != nil || source.Markdown != "# 手工语料\n\n仅用于小范围修正。" || source.ContentHash == "" {
		t.Fatalf("manual Markdown source = %+v", source)
	}
}

func TestDecodeContextDocumentImportRejectsMissingAndOversizeFiles(t *testing.T) {
	var missingBody bytes.Buffer
	missingWriter := multipart.NewWriter(&missingBody)
	if err := missingWriter.WriteField("title", "missing file"); err != nil {
		t.Fatal(err)
	}
	if err := missingWriter.Close(); err != nil {
		t.Fatal(err)
	}
	missingRequest := httptest.NewRequest(http.MethodPost, "/api/context/documents/import", bytes.NewReader(missingBody.Bytes()))
	missingRequest.Header.Set("Content-Type", missingWriter.FormDataContentType())
	_, missingStatus, missingErr := decodeContextDocumentImport(httptest.NewRecorder(), missingRequest)
	if missingErr == nil || missingStatus != http.StatusBadRequest || !strings.Contains(missingErr.Error(), "source file is required") {
		t.Fatalf("missing file status=%d err=%v", missingStatus, missingErr)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "too-large.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(bytes.Repeat([]byte{'x'}, maxContextUploadBytes+1)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	oversizeRequest := httptest.NewRequest(http.MethodPost, "/api/context/documents/import", bytes.NewReader(body.Bytes()))
	oversizeRequest.Header.Set("Content-Type", writer.FormDataContentType())
	_, oversizeStatus, oversizeErr := decodeContextDocumentImport(httptest.NewRecorder(), oversizeRequest)
	if oversizeErr == nil || oversizeStatus != http.StatusRequestEntityTooLarge || !strings.Contains(oversizeErr.Error(), "10 MiB") {
		t.Fatalf("oversize file status=%d err=%v", oversizeStatus, oversizeErr)
	}
}

func authenticatedContextDocumentMultipartRequest(t *testing.T, srv *Server, token string, fields map[string]string, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			t.Fatalf("write multipart field %s: %v", name, err)
		}
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/context/documents/import", io.NopCloser(bytes.NewReader(body.Bytes())))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	return rr
}

func TestArchiveContextDocumentWithdrawsUnpublishedCandidates(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "archive-owner@example.com", "Archive Owner", []string{"ai_context:write"})
	srv := NewServer(&config.Config{}, "")
	document := db.ContextDocument{
		Title: "Deprecated architecture source", Scope: "global", Source: "doc", Status: "active", IngestionStatus: "completed", Version: 1,
	}
	if err := db.DB.Create(&document).Error; err != nil {
		t.Fatalf("seed context document: %v", err)
	}
	candidates := []db.CorpusCandidate{
		{ContextDocumentID: document.ID, CandidateType: "workflow", Scope: "global", Title: "Pending", Summary: "Pending", Content: "Pending", Status: "pending"},
		{ContextDocumentID: document.ID, CandidateType: "architecture", Scope: "global", Title: "Impact", Summary: "Impact", Content: "Impact", Status: "impact_review"},
		{ContextDocumentID: document.ID, CandidateType: "workflow", Scope: "global", Title: "Published", Summary: "Published", Content: "Published", Status: "accepted", AcceptedContextFactID: 77},
	}
	if err := db.DB.Create(&candidates).Error; err != nil {
		t.Fatalf("seed document candidates: %v", err)
	}

	archived := authenticatedJSONRequest(t, srv, token, http.MethodPost, "/api/context/documents/"+itoa(document.ID)+"/archive", nil)
	if archived.Code != http.StatusOK || !strings.Contains(archived.Body.String(), `"withdrawn_candidate_count":2`) {
		t.Fatalf("archive document status=%d body=%s", archived.Code, archived.Body.String())
	}
	var reloaded []db.CorpusCandidate
	if err := db.DB.Where("context_document_id = ?", document.ID).Order("id asc").Find(&reloaded).Error; err != nil {
		t.Fatalf("reload document candidates: %v", err)
	}
	if reloaded[0].Status != "rejected" || reloaded[1].Status != "rejected" || reloaded[2].Status != "accepted" {
		t.Fatalf("unexpected archive candidate states: %+v", reloaded)
	}
	if reloaded[0].ReviewedAt == nil || reloaded[1].ImpactPreviewedAt != nil || reloaded[2].AcceptedContextFactID != 77 {
		t.Fatalf("archive audit/publication state mismatch: %+v", reloaded)
	}
}
