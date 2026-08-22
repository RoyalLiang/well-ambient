package server

import (
	"context"
	"fmt"
	"strings"

	"well-ambient/internal/config"
	providerllm "well-ambient/internal/llm"
)

func queryServerLLM(cfg *config.Config, systemPrompt, userPrompt string) (string, error) {
	return queryServerLLMContext(context.Background(), cfg, systemPrompt, userPrompt)
}

func queryServerLLMContext(ctx context.Context, cfg *config.Config, systemPrompt, userPrompt string) (string, error) {
	if cfg == nil || !cfg.AI.Enabled || strings.TrimSpace(cfg.AI.APIToken) == "" || strings.TrimSpace(cfg.AI.BaseURL) == "" {
		return "", fmt.Errorf("AI configuration is not enabled or is missing credentials")
	}
	client := providerllm.Client{Config: cfg.AI}
	return client.Generate(ctx, providerllm.Request{SystemPrompt: systemPrompt, UserPrompt: userPrompt})
}
