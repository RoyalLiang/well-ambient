package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestStreamAIDemandSpecEmitsMarkdownAndCompletesAfterPersistence(t *testing.T) {
	setupServerTestDB(t)
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID: "DEMAND-STREAM", Title: "流式规格", Description: "实时生成可审核规格", Repo: "well-ambient",
		IssueType: "demand", Status: "backlog", Assignee: "Owner", Creator: "Creator", LastUpdate: time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}

	markdown := "# 流式规格\n\n> 实时生成可审核规格\n\n## 验收标准\n\n- 首字节及时返回\n- 草案事务落库"
	structured := `{"intent":"feature","intent_confidence":0.91,"summary":"流式规格","user_goal":"实时生成可审核规格","facts":["LLM 响应较慢"],"inferences":[],"missing_context":[],"business_rules":["完成事件必须晚于持久化"],"main_flows":["建立流连接","增量输出","保存草案"],"exception_flows":["断开后取消上游请求"],"permission_rules":["需要需求规格写权限"],"data_impact":["保存 Markdown 原文"],"api_impact":["新增 NDJSON 流式端点"],"ui_impact":["Markdown 实时预览"],"dependencies":[],"risks":["代理缓冲"],"acceptance_criteria":["可以观察到增量 Markdown","完成后可读取草案"],"test_plan":["模拟 SSE provider","校验数据库"],"mapped_repos":["well-ambient"],"tasks":[],"readiness_score":88}`
	providerRunes := []rune(markdown + demandSpecStreamDelimiter + structured)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Errorf("provider path = %q", r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode provider request: %v", err)
		}
		if payload["stream"] != true {
			t.Errorf("stream flag = %#v", payload["stream"])
		}
		if payload["instructions"] == nil || payload["input"] == nil || payload["messages"] != nil {
			t.Errorf("provider payload is not Responses API: %#v", payload)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for len(providerRunes) > 0 {
			width := 13
			if len(providerRunes) < width {
				width = len(providerRunes)
			}
			chunk := string(providerRunes[:width])
			providerRunes = providerRunes[width:]
			data, _ := json.Marshal(map[string]interface{}{"type": "response.output_text.delta", "delta": chunk})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer provider.Close()

	srv := NewServer(&config.Config{AI: config.AIConfig{
		Enabled: true, BaseURL: provider.URL + "/v1/chat/completions", EndpointType: "completions", APIToken: "test-token", Model: "test-model",
	}}, "")
	body := bytes.NewBufferString(`{"demand_id":"DEMAND-STREAM","original_text":"流式规格\\n实时生成可审核规格"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/demand-specs/ai-stream", body)
	recorder := httptest.NewRecorder()
	srv.handleStreamAIDemandSpec(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var events []demandSpecStreamEvent
	for _, line := range strings.Split(strings.TrimSpace(recorder.Body.String()), "\n") {
		var event demandSpecStreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode stream event %q: %v", line, err)
		}
		events = append(events, event)
	}
	var streamed strings.Builder
	completeIndex := -1
	for index, event := range events {
		if event.Type == "markdown_delta" {
			streamed.WriteString(event.Delta)
		}
		if event.Type == "complete" {
			completeIndex = index
			if event.Spec == nil || event.Spec.ID == 0 || event.ReviewContract == nil || event.ReviewContract.ID == 0 {
				t.Fatalf("complete event missing persisted records: %+v", event)
			}
		}
	}
	if completeIndex < 0 || completeIndex != len(events)-1 {
		t.Fatalf("complete event index = %d of %d", completeIndex, len(events))
	}
	if strings.TrimSpace(streamed.String()) != markdown {
		t.Fatalf("streamed markdown = %q", streamed.String())
	}
	if strings.Contains(streamed.String(), "WELL_AMBIENT_SPEC_JSON") || strings.Contains(streamed.String(), `"readiness_score"`) {
		t.Fatalf("structured payload leaked into markdown: %s", streamed.String())
	}

	var saved db.DemandSpecVersion
	if err := db.DB.Where("demand_id = ?", "DEMAND-STREAM").First(&saved).Error; err != nil {
		t.Fatalf("saved spec not found: %v", err)
	}
	if saved.MarkdownContent != markdown || saved.ReadinessScore != 88 {
		t.Fatalf("saved spec mismatch: markdown=%q readiness=%d", saved.MarkdownContent, saved.ReadinessScore)
	}
	var contractCount int64
	if err := db.DB.Model(&db.ReviewContract{}).Where("demand_spec_version_id = ?", saved.ID).Count(&contractCount).Error; err != nil || contractCount != 1 {
		t.Fatalf("review contract count = %d, err = %v", contractCount, err)
	}
}
