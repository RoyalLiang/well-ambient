package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"well-ambient/internal/config"
)

func queryServerLLM(cfg *config.Config, systemPrompt, userPrompt string) (string, error) {
	if cfg == nil || !cfg.AI.Enabled || strings.TrimSpace(cfg.AI.APIToken) == "" || strings.TrimSpace(cfg.AI.BaseURL) == "" {
		return "", fmt.Errorf("AI configuration is not enabled or is missing credentials")
	}
	endpointType := strings.ToLower(strings.TrimSpace(cfg.AI.EndpointType))
	if endpointType == "" {
		endpointType = "completions"
	}
	model := firstNonBlank(cfg.AI.Model, "gpt-4o")
	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.1,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, cfg.AI.GetRealAPIURL(), bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.AI.APIToken))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to contact AI provider: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI provider returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
		return "", fmt.Errorf("AI provider returned HTML instead of JSON")
	}
	content, err := extractLLMText(body, endpointType)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("AI provider returned empty output")
	}
	return strings.TrimSpace(content), nil
}

func extractLLMText(body []byte, endpointType string) (string, error) {
	if endpointType == "responses" {
		var response struct {
			Output []struct {
				Type    string `json:"type"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"output"`
		}
		if err := json.Unmarshal(body, &response); err == nil {
			var text strings.Builder
			for _, output := range response.Output {
				for _, content := range output.Content {
					if content.Text != "" {
						text.WriteString(content.Text)
					}
				}
			}
			if text.Len() > 0 {
				return text.String(), nil
			}
		}
	}
	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &completion); err != nil {
		return "", fmt.Errorf("failed to decode AI response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return "", fmt.Errorf("AI response contains no choices")
	}
	return completion.Choices[0].Message.Content, nil
}
