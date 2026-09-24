package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"well-ambient/internal/config"
)

const (
	defaultModel           = "gpt-4o"
	defaultAnthropicTokens = 4096
	maxResponseBody        = 4 << 20
)

type Request struct {
	SystemPrompt    string
	UserPrompt      string
	FileIDs         []string
	Files           []FileInput
	MaxOutputTokens int
}

// FileInput is an opaque provider input. Callers supply bytes and transport
// metadata; provider adapters decide how to encode them without interpreting
// or extracting the file body.
type FileInput struct {
	Name     string
	MIMEType string
	Data     []byte
}

type Client struct {
	Config     config.AIConfig
	HTTPClient *http.Client
}

func (c Client) Generate(ctx context.Context, input Request) (string, error) {
	response, protocol, err := c.do(ctx, input, false)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBody))
	if err != nil {
		return "", fmt.Errorf("read LLM response: %w", err)
	}
	if err := validateResponse(response, body); err != nil {
		return "", err
	}
	text, err := extractText(body, protocol)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("LLM provider returned empty output")
	}
	return strings.TrimSpace(text), nil
}

func (c Client) Stream(ctx context.Context, input Request, onDelta func(string) error) (string, error) {
	response, protocol, err := c.do(ctx, input, true)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 8<<10))
		return "", providerStatusError(response.StatusCode, body)
	}
	contentType := strings.ToLower(response.Header.Get("Content-Type"))
	if !strings.Contains(contentType, "text/event-stream") && !strings.Contains(contentType, "ndjson") {
		body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBody))
		if err != nil {
			return "", fmt.Errorf("read LLM response: %w", err)
		}
		if err := validateResponse(response, body); err != nil {
			return "", err
		}
		text, err := extractText(body, protocol)
		if err != nil {
			return "", err
		}
		if text != "" && onDelta != nil {
			if err := onDelta(text); err != nil {
				return "", err
			}
		}
		return text, nil
	}

	var accumulated strings.Builder
	var completedFallback string
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 64*1024), maxResponseBody)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "event:") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
		if line == "" || line == "[DONE]" {
			continue
		}
		delta, fallback, terminalErr := parseStreamEvent([]byte(line), protocol)
		if terminalErr != nil {
			return "", terminalErr
		}
		if fallback != "" {
			completedFallback = fallback
		}
		if delta == "" {
			continue
		}
		accumulated.WriteString(delta)
		if onDelta != nil {
			if err := onDelta(delta); err != nil {
				return "", err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read LLM stream: %w", err)
	}
	text := accumulated.String()
	if text == "" {
		text = completedFallback
		if text != "" && onDelta != nil {
			if err := onDelta(text); err != nil {
				return "", err
			}
		}
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("LLM provider returned an empty stream")
	}
	return text, nil
}

func (c Client) do(ctx context.Context, input Request, stream bool) (*http.Response, string, error) {
	if strings.TrimSpace(c.Config.APIToken) == "" || strings.TrimSpace(c.Config.BaseURL) == "" {
		return nil, "", fmt.Errorf("AI configuration is missing credentials")
	}
	protocol := c.Config.Protocol()
	payload, err := buildPayload(c.Config, input, stream)
	if err != nil {
		return nil, "", err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", fmt.Errorf("encode LLM request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Config.GetRealAPIURL(), bytes.NewReader(body))
	if err != nil {
		return nil, "", fmt.Errorf("create LLM request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if protocol == "messages" {
		req.Header.Set("x-api-key", strings.TrimSpace(c.Config.APIToken))
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Config.APIToken))
	}
	client := c.httpClient()
	response, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("contact LLM provider: %w", err)
	}
	return response, protocol, nil
}

func (c Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		client := *c.HTTPClient
		client.Timeout = 0
		return &client
	}
	// Model generation can legitimately stay open for an arbitrary duration.
	// Timeout must remain zero; caller context cancellation still stops the
	// request when a browser disconnects or the owning service shuts down.
	return &http.Client{}
}

func buildPayload(cfg config.AIConfig, input Request, stream bool) (map[string]any, error) {
	if err := cfg.NormalizeReasoningEffort(); err != nil {
		return nil, err
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultModel
	}
	if cfg.Protocol() == "messages" {
		if len(input.FileIDs) > 0 || len(input.Files) > 0 {
			return nil, fmt.Errorf("native Claude Messages file passthrough is not supported; use the Responses protocol for document imports")
		}
		maxTokens := input.MaxOutputTokens
		if maxTokens <= 0 {
			maxTokens = defaultAnthropicTokens
		}
		payload := map[string]any{
			"model":      model,
			"system":     input.SystemPrompt,
			"messages":   []map[string]any{{"role": "user", "content": input.UserPrompt}},
			"max_tokens": maxTokens,
			"stream":     stream,
		}
		if cfg.ReasoningEffort != "" {
			payload["output_config"] = map[string]any{"effort": cfg.ReasoningEffort}
		}
		return payload, nil
	}
	content := make([]map[string]any, 0, len(input.FileIDs)+len(input.Files)+1)
	for _, fileID := range input.FileIDs {
		fileID = strings.TrimSpace(fileID)
		if fileID != "" {
			content = append(content, map[string]any{"type": "input_file", "file_id": fileID})
		}
	}
	for _, file := range input.Files {
		if len(file.Data) == 0 {
			return nil, fmt.Errorf("LLM file input is empty")
		}
		name := strings.TrimSpace(filepath.Base(file.Name))
		if name == "" || name == "." {
			return nil, fmt.Errorf("LLM file input name is required")
		}
		mimeType := strings.TrimSpace(file.MIMEType)
		if mimeType == "" {
			mimeType = mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
		}
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		if parsed, _, err := mime.ParseMediaType(mimeType); err == nil && parsed != "" {
			mimeType = parsed
		}
		content = append(content, map[string]any{
			"type":      "input_file",
			"filename":  name,
			"file_data": "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(file.Data),
		})
	}
	content = append(content, map[string]any{"type": "input_text", "text": input.UserPrompt})
	payload := map[string]any{
		"model":        model,
		"instructions": input.SystemPrompt,
		"input":        []map[string]any{{"role": "user", "content": content}},
		"stream":       stream,
	}
	if input.MaxOutputTokens > 0 {
		payload["max_output_tokens"] = input.MaxOutputTokens
	}
	if cfg.ReasoningEffort != "" {
		payload["reasoning"] = map[string]any{"effort": cfg.ReasoningEffort}
	}
	return payload, nil
}

func validateResponse(response *http.Response, body []byte) error {
	if response.StatusCode != http.StatusOK {
		return providerStatusError(response.StatusCode, body)
	}
	if strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "text/html") {
		return fmt.Errorf("LLM provider returned HTML instead of JSON")
	}
	return nil
}

func providerStatusError(status int, body []byte) error {
	detail := strings.TrimSpace(string(body))
	if len(detail) > 1000 {
		detail = detail[:1000]
	}
	return fmt.Errorf("LLM provider returned status %d: %s", status, detail)
}

func extractText(body []byte, protocol string) (string, error) {
	if protocol == "messages" {
		var response struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			return "", fmt.Errorf("decode Claude Messages response: %w", err)
		}
		var text strings.Builder
		for _, block := range response.Content {
			if block.Type == "text" && block.Text != "" {
				text.WriteString(block.Text)
			}
		}
		return text.String(), nil
	}
	var response struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("decode Responses API response: %w", err)
	}
	if response.OutputText != "" {
		return response.OutputText, nil
	}
	var text strings.Builder
	for _, item := range response.Output {
		if item.Type != "" && item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if (part.Type == "output_text" || part.Type == "text" || part.Type == "") && part.Text != "" {
				text.WriteString(part.Text)
			}
		}
	}
	return text.String(), nil
}

func parseStreamEvent(data []byte, protocol string) (delta string, completedFallback string, err error) {
	var event struct {
		Type  string `json:"type"`
		Delta any    `json:"delta"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Response json.RawMessage `json:"response"`
	}
	if unmarshalErr := json.Unmarshal(data, &event); unmarshalErr != nil {
		return "", "", nil
	}
	if event.Error != nil && event.Error.Message != "" {
		return "", "", fmt.Errorf("LLM provider stream error: %s", event.Error.Message)
	}
	if event.Type == "error" || event.Type == "response.failed" || event.Type == "response.incomplete" {
		return "", "", fmt.Errorf("LLM provider stream ended with %s", event.Type)
	}
	if protocol == "messages" {
		if event.Type != "content_block_delta" {
			return "", "", nil
		}
		var anthropic struct {
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if json.Unmarshal(data, &anthropic) == nil && anthropic.Delta.Type == "text_delta" {
			return anthropic.Delta.Text, "", nil
		}
		return "", "", nil
	}
	if event.Type == "response.output_text.delta" {
		if value, ok := event.Delta.(string); ok {
			return value, "", nil
		}
	}
	if event.Type == "response.completed" && len(event.Response) > 0 {
		text, extractErr := extractText(event.Response, "responses")
		return "", text, extractErr
	}
	return "", "", nil
}
