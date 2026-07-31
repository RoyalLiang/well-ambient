package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"well-ambient/internal/config"
	providerllm "well-ambient/internal/llm"
)

const demandSpecStreamDelimiter = "\n<!-- WELL_AMBIENT_SPEC_JSON -->\n"

type demandSpecStreamEvent struct {
	Type           string             `json:"type"`
	Phase          string             `json:"phase,omitempty"`
	Message        string             `json:"message,omitempty"`
	Delta          string             `json:"delta,omitempty"`
	Spec           *demandSpecDTO     `json:"spec,omitempty"`
	ReviewContract *reviewContractDTO `json:"review_contract,omitempty"`
}

type llmStreamResult struct {
	content string
	err     error
}

func (s *Server) handleStreamAIDemandSpec(w http.ResponseWriter, r *http.Request) {
	var request struct {
		DemandID     string `json:"demand_id"`
		OriginalText string `json:"original_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	request.DemandID = strings.TrimSpace(request.DemandID)
	request.OriginalText = strings.TrimSpace(request.OriginalText)
	if request.DemandID == "" || request.OriginalText == "" {
		http.Error(w, "demand_id and original_text are required", http.StatusBadRequest)
		return
	}
	if !s.config.AI.Enabled || strings.TrimSpace(s.config.AI.APIToken) == "" || strings.TrimSpace(s.config.AI.BaseURL) == "" {
		http.Error(w, "AI configuration is not enabled or is missing credentials", http.StatusServiceUnavailable)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")

	writeEvent := func(event demandSpecStreamEvent) bool {
		data, err := json.Marshal(event)
		if err != nil {
			return false
		}
		if _, err := w.Write(append(data, '\n')); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	if !writeEvent(demandSpecStreamEvent{Type: "status", Phase: "preparing", Message: "已建立流式连接，正在准备需求上下文"}) {
		return
	}

	systemPrompt := demandSpecAIPrompt()
	userPrompt := fmt.Sprintf("需求编号：%s\n\n需求原文：\n%s", request.DemandID, request.OriginalText)
	deltas := make(chan string, 32)
	result := make(chan llmStreamResult, 1)
	go func() {
		content, err := queryServerLLMStream(r.Context(), s.config, systemPrompt, userPrompt, deltas)
		result <- llmStreamResult{content: content, err: err}
		close(deltas)
	}()

	if !writeEvent(demandSpecStreamEvent{Type: "status", Phase: "generating", Message: "AI 正在编写实现规格"}) {
		return
	}
	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()
	var raw strings.Builder
	emittedMarkdown := 0
	delimiterFound := false
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if !writeEvent(demandSpecStreamEvent{Type: "heartbeat", Phase: "generating", Message: "AI 仍在生成，请保持页面开启"}) {
				return
			}
		case delta, open := <-deltas:
			if !open {
				deltas = nil
				continue
			}
			raw.WriteString(delta)
			if delimiterFound {
				continue
			}
			current := raw.String()
			if delimiterIndex := strings.Index(current, demandSpecStreamDelimiter); delimiterIndex >= 0 {
				if delimiterIndex > emittedMarkdown && !writeEvent(demandSpecStreamEvent{Type: "markdown_delta", Delta: current[emittedMarkdown:delimiterIndex]}) {
					return
				}
				emittedMarkdown = delimiterIndex
				delimiterFound = true
				continue
			}
			safeEnd := validUTF8PrefixEnd(current, len(current)-len(demandSpecStreamDelimiter))
			if safeEnd > emittedMarkdown {
				if !writeEvent(demandSpecStreamEvent{Type: "markdown_delta", Delta: current[emittedMarkdown:safeEnd]}) {
					return
				}
				emittedMarkdown = safeEnd
			}
		case final := <-result:
			if final.err != nil {
				writeEvent(demandSpecStreamEvent{Type: "error", Phase: "generating", Message: final.err.Error()})
				return
			}
			content := final.content
			if raw.Len() == 0 {
				raw.WriteString(content)
			}
			separator := strings.Index(content, demandSpecStreamDelimiter)
			if separator < 0 {
				writeEvent(demandSpecStreamEvent{Type: "error", Phase: "parsing", Message: "AI 返回缺少规格数据分隔符，请重试"})
				return
			}
			markdown := strings.TrimSpace(content[:separator])
			if !delimiterFound && len(content[:separator]) > emittedMarkdown {
				if !writeEvent(demandSpecStreamEvent{Type: "markdown_delta", Delta: content[emittedMarkdown:separator]}) {
					return
				}
			}
			if !writeEvent(demandSpecStreamEvent{Type: "status", Phase: "persisting", Message: "内容生成完成，正在建立草案与审核契约"}) {
				return
			}
			var payload demandSpecPayload
			jsonPart := cleanDemandSpecJSON(content[separator+len(demandSpecStreamDelimiter):])
			if err := json.Unmarshal([]byte(jsonPart), &payload); err != nil {
				writeEvent(demandSpecStreamEvent{Type: "error", Phase: "parsing", Message: fmt.Sprintf("AI 规格结构解析失败：%v", err)})
				return
			}
			payload.ID = 0
			payload.DemandID = request.DemandID
			payload.OriginalText = request.OriginalText
			payload.MarkdownContent = markdown
			payload.ModelVersion = firstNonBlank(payload.ModelVersion, s.config.AI.Model)
			payload.RuleVersion = firstNonBlank(payload.RuleVersion, "demand-spec-stream-v1")
			if payload.ReadinessScore < 0 {
				payload.ReadinessScore = 0
			} else if payload.ReadinessScore > 100 {
				payload.ReadinessScore = 100
			}
			spec, contract, _, err := s.createDemandSpecRecord(payload, authenticatedActor(r))
			if err != nil {
				writeEvent(demandSpecStreamEvent{Type: "error", Phase: "persisting", Message: err.Error()})
				return
			}
			specDTO := demandSpecFromModel(spec)
			contractDTO := reviewContractFromModel(contract)
			writeEvent(demandSpecStreamEvent{Type: "complete", Phase: "complete", Message: "AI 草案已生成并保存", Spec: &specDTO, ReviewContract: &contractDTO})
			return
		}
	}
}

func validUTF8PrefixEnd(value string, end int) int {
	if end > len(value) {
		end = len(value)
	}
	for end > 0 && !utf8.ValidString(value[:end]) {
		end--
	}
	return end
}

func demandSpecAIPrompt() string {
	return `你是资深软件需求分析师和技术规格作者。请把需求写成可审核、可测试、可追溯的实现规格。

输出必须严格分为两部分：
1. 先输出完整 Markdown 文档，使用清晰的一级/二级标题、编号步骤、项目列表、表格或代码块（仅在确有必要时）。不要使用 JSON 代码块，不要解释输出格式。
2. 紧接着原样输出分隔符：
<!-- WELL_AMBIENT_SPEC_JSON -->
3. 分隔符后只输出一个紧凑 JSON 对象，不要加 Markdown 围栏或额外文字。

Markdown 必须覆盖：需求摘要、用户目标、业务规则、主流程、异常与回退、权限与安全、数据影响、API 影响、界面与交互、依赖、风险、验收标准、测试计划、目标仓库与待确认事项。信息不足时明确标注“待确认”，不要虚构。

JSON 必须使用以下结构，所有数组都必须存在：
{"intent":"feature|bugfix|refactor|research|operations","intent_confidence":0.8,"summary":"...","user_goal":"...","facts":["..."],"inferences":["..."],"missing_context":["..."],"business_rules":["..."],"main_flows":["..."],"exception_flows":["..."],"permission_rules":["..."],"data_impact":["..."],"api_impact":["..."],"ui_impact":["..."],"dependencies":["..."],"risks":["..."],"acceptance_criteria":["..."],"test_plan":["..."],"mapped_repos":["..."],"tasks":[],"readiness_score":80,"model_version":"","rule_version":"demand-spec-stream-v1"}

验收标准和测试计划必须可操作；API 需说明请求、响应、错误、幂等或兼容性；界面需说明加载、空、错误、完成状态和响应式行为。`
}

func cleanDemandSpecJSON(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "```json")
	value = strings.TrimPrefix(value, "```")
	value = strings.TrimSuffix(value, "```")
	return strings.TrimSpace(value)
}

func queryServerLLMStream(ctx context.Context, cfg *config.Config, systemPrompt, userPrompt string, deltas chan<- string) (string, error) {
	client := providerllm.Client{Config: cfg.AI}
	return client.Stream(ctx, providerllm.Request{SystemPrompt: systemPrompt, UserPrompt: userPrompt}, func(delta string) error {
		select {
		case deltas <- delta:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
}
