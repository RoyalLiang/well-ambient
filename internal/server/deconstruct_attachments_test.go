package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestProviderFilesHTTPClientHasNoOverallTimeout(t *testing.T) {
	if got := providerFilesHTTPClient().Timeout; got != 0 {
		t.Fatalf("provider files timeout = %s, want no overall timeout", got)
	}
}

func TestDeconstructAttachmentStreamsStoresAndLinks(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init database: %v", err)
	}
	if err := db.DB.Create(&db.TaskTelemetry{TaskID: "DEMAND-901", Title: "附件关联需求", IssueType: "demand", Status: "backlog"}).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}

	const documentBody = "需求附件正文：支持 PDF 与 Word 原件直传，并保留归档证据。"
	var fileUploadCount atomic.Int32
	var fileDeleteCount atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/files":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			file, _, err := r.FormFile("file")
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			defer file.Close()
			body, _ := io.ReadAll(file)
			if string(body) != documentBody {
				t.Errorf("provider received %q, want %q", string(body), documentBody)
			}
			fileUploadCount.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"file-demand-1"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/responses":
			body, _ := io.ReadAll(r.Body)
			if !bytes.Contains(body, []byte(`"type":"input_file"`)) || !bytes.Contains(body, []byte(`"file_id":"file-demand-1"`)) {
				t.Errorf("responses payload does not reference uploaded file: %s", body)
			}
			modelOutput := `{"mappedRepos":["frontend-dashboard"],"tasks":[{"id":"task-901","repo":"frontend-dashboard","title":"实现附件流转","assignee":"Eddie","priority":"High","complexity":"Medium","difficulty":"Medium","estimated_days":1,"estimated_hours":8,"estimate_basis":"附件流与归档联调"}],"analysis":{"completeness_score":90,"overall_estimated_days":1,"overall_estimated_hours":8,"overall_difficulty":"Medium","estimate_basis":"单链路改造","missing_info":[],"risks":[],"dependencies":[],"acceptance_criteria":["附件可追溯"],"schedule_notes":[],"meeting_questions":[],"confidence":0.9}}`
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"output": []map[string]interface{}{{
					"type":    "message",
					"content": []map[string]string{{"type": "output_text", "text": modelOutput}},
				}},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/files/file-demand-1":
			fileDeleteCount.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"deleted":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()

	server := NewServer(&config.Config{
		Server: config.ServerConfig{AttachmentDir: t.TempDir()},
		AI: config.AIConfig{
			Enabled:      true,
			BaseURL:      provider.URL,
			EndpointType: "completions",
			APIToken:     "test-token",
			Model:        "gpt-4o",
		},
		GitLab: config.GitLabConfig{Repos: []config.RepoMapping{{Name: "frontend-dashboard"}}},
		Jira:   config.JiraConfig{SyncUsers: []string{"Eddie"}},
	}, "")

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	_ = writer.WriteField("text", "请解构附件中的需求")
	_ = writer.WriteField("demand_id", "DEMAND-901")
	_ = writer.WriteField("task_group_id", "brain-demand-901")
	filePart, err := writer.CreateFormFile("files", "requirement.docx")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = io.WriteString(filePart, documentBody)
	_ = writer.Close()

	request := httptest.NewRequest(http.MethodPost, "/api/deconstruct", &requestBody)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("x-authenticated-user-id", "Eddie")
	response := httptest.NewRecorder()
	server.handleDeconstruct(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("deconstruct status = %d, body = %s", response.Code, response.Body.String())
	}

	var result DeconstructResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode deconstruct response: %v", err)
	}
	if len(result.AttachmentIDs) != 1 || result.ContextPackID == 0 {
		t.Fatalf("attachment IDs/context pack missing: %+v", result)
	}
	if fileUploadCount.Load() != 1 || fileDeleteCount.Load() != 1 {
		t.Fatalf("provider upload/delete = %d/%d, want 1/1", fileUploadCount.Load(), fileDeleteCount.Load())
	}

	var stored db.DemandAttachment
	if err := db.DB.First(&stored, result.AttachmentIDs[0]).Error; err != nil {
		t.Fatalf("load stored attachment: %v", err)
	}
	if stored.Status != "processed" || stored.DemandID != "DEMAND-901" || stored.OriginalSize != int64(len(documentBody)) || stored.CompressedSize <= 0 {
		t.Fatalf("unexpected stored attachment: %+v", stored)
	}

	importPayload := map[string]interface{}{
		"task_group_id":   "brain-demand-901",
		"demand_id":       "DEMAND-901",
		"input_text":      "请解构附件中的需求",
		"mappedRepos":     result.MappedRepos,
		"analysis":        result.Analysis,
		"context_pack_id": result.ContextPackID,
		"attachment_ids":  result.AttachmentIDs,
		"tasks":           result.Tasks,
	}
	importBody, _ := json.Marshal(importPayload)
	importRequest := httptest.NewRequest(http.MethodPost, "/api/tasks/import", bytes.NewReader(importBody))
	importRequest.Header.Set("Content-Type", "application/json")
	importRequest.Header.Set("x-authenticated-user-id", "Eddie")
	importResponse := httptest.NewRecorder()
	server.handleImportTasks(importResponse, importRequest)
	if importResponse.Code != http.StatusOK {
		t.Fatalf("import status = %d, body = %s", importResponse.Code, importResponse.Body.String())
	}
	if err := db.DB.First(&stored, stored.ID).Error; err != nil {
		t.Fatalf("reload linked attachment: %v", err)
	}
	if stored.Status != "linked" || stored.DeconstructArchiveID == 0 || stored.DemandID != "DEMAND-901" || !strings.EqualFold(stored.TaskGroupID, "brain-demand-901") {
		t.Fatalf("attachment was not linked to archive: %+v", stored)
	}
}

func TestDeconstructStreamsResponsesDeltasAndCompletedResult(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init database: %v", err)
	}
	modelOutput := `{"mappedRepos":["frontend-dashboard"],"tasks":[{"id":"task-902","repo":"frontend-dashboard","title":"验证 Responses 流","assignee":"Eddie","priority":"High","complexity":"Medium","difficulty":"Medium","estimated_days":1,"estimated_hours":8,"estimate_basis":"协议与浏览器流验证"}],"analysis":{"completeness_score":95,"overall_estimated_days":1,"overall_estimated_hours":8,"overall_difficulty":"Medium","estimate_basis":"单链路验证","missing_info":[],"risks":[],"dependencies":[],"acceptance_criteria":["浏览器收到完整结果"],"schedule_notes":[],"meeting_questions":[],"confidence":0.95}}`
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Errorf("provider path = %q", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if bytes.Contains(body, []byte(`"messages"`)) || !bytes.Contains(body, []byte(`"stream":true`)) {
			t.Errorf("unexpected Responses payload: %s", body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		midpoint := len(modelOutput) / 2
		for _, delta := range []string{modelOutput[:midpoint], modelOutput[midpoint:]} {
			data, _ := json.Marshal(map[string]interface{}{"type": "response.output_text.delta", "delta": delta})
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer provider.Close()

	server := NewServer(&config.Config{
		AI: config.AIConfig{
			Enabled: true, BaseURL: provider.URL + "/v1/chat/completions", EndpointType: "completions", APIToken: "test-token", Model: "test-model",
		},
		GitLab: config.GitLabConfig{Repos: []config.RepoMapping{{Name: "frontend-dashboard"}}},
		Jira:   config.JiraConfig{SyncUsers: []string{"Eddie"}},
	}, "")
	body := bytes.NewBufferString(`{"text":"验证需求解构流"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/deconstruct", body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/x-ndjson")
	response := httptest.NewRecorder()
	server.handleDeconstruct(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("deconstruct status = %d, body = %s", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); !strings.Contains(contentType, "application/x-ndjson") {
		t.Fatalf("content type = %q", contentType)
	}
	var deltas int
	var completed *DeconstructResponse
	for _, line := range strings.Split(strings.TrimSpace(response.Body.String()), "\n") {
		var event deconstructStreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode stream event: %v", err)
		}
		if event.Type == "provider_delta" {
			deltas++
		}
		if event.Type == "complete" {
			completed = event.Result
		}
	}
	if deltas != 2 || completed == nil || len(completed.Tasks) != 1 || completed.Tasks[0].ID != "task-902" {
		t.Fatalf("unexpected stream result: deltas=%d completed=%+v", deltas, completed)
	}
}

func TestProviderFilesURL(t *testing.T) {
	got, err := providerFilesURL("https://example.test/v1/responses")
	if err != nil {
		t.Fatalf("providerFilesURL: %v", err)
	}
	if got != "https://example.test/v1/files" {
		t.Fatalf("providerFilesURL = %q", got)
	}
	responsesURL, err := providerResponsesURL("https://example.test/v1/chat/completions")
	if err != nil {
		t.Fatalf("providerResponsesURL: %v", err)
	}
	if responsesURL != "https://example.test/v1/responses" {
		t.Fatalf("providerResponsesURL = %q", responsesURL)
	}
}
