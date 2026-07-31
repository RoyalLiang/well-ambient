package config

import (
	"testing"
)

func TestGetRealAPIURL(t *testing.T) {
	tests := []struct {
		name         string
		baseURL      string
		endpointType string
		expected     string
	}{
		{
			name:         "Legacy completions migrates to responses",
			baseURL:      "https://ai-pixel.online",
			endpointType: "completions",
			expected:     "https://ai-pixel.online/v1/responses",
		},
		{
			name:         "Pure domain responses",
			baseURL:      "https://ai-pixel.online",
			endpointType: "responses",
			expected:     "https://ai-pixel.online/v1/responses",
		},
		{
			name:         "Pure domain with trailing slash",
			baseURL:      "https://ai-pixel.online/",
			endpointType: "completions",
			expected:     "https://ai-pixel.online/v1/responses",
		},
		{
			name:         "Explicit completions path migrates to responses",
			baseURL:      "https://ai-pixel.online/v1/chat/completions",
			endpointType: "completions",
			expected:     "https://ai-pixel.online/v1/responses",
		},
		{
			name:         "Already complete responses path",
			baseURL:      "https://ai-pixel.online/v1/responses",
			endpointType: "responses",
			expected:     "https://ai-pixel.online/v1/responses",
		},
		{
			name:         "Complete completions path with extra case difference",
			baseURL:      "https://ai-pixel.online/V1/chat/completions",
			endpointType: "responses",
			expected:     "https://ai-pixel.online/v1/responses",
		},
		{
			name:         "Version root does not duplicate v1",
			baseURL:      "https://ai-pixel.online/v1",
			endpointType: "responses",
			expected:     "https://ai-pixel.online/v1/responses",
		},
		{
			name:         "Claude messages endpoint",
			baseURL:      "https://api.anthropic.com",
			endpointType: "messages",
			expected:     "https://api.anthropic.com/v1/messages",
		},
		{
			name:         "Empty URL",
			baseURL:      "",
			endpointType: "completions",
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AIConfig{
				BaseURL:      tt.baseURL,
				EndpointType: tt.endpointType,
			}
			result := cfg.GetRealAPIURL()
			if result != tt.expected {
				t.Errorf("GetRealAPIURL() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestAIConfigProtocol(t *testing.T) {
	tests := []struct {
		name     string
		config   AIConfig
		expected string
	}{
		{name: "responses by default", config: AIConfig{}, expected: "responses"},
		{name: "legacy completions", config: AIConfig{EndpointType: "completions"}, expected: "responses"},
		{name: "explicit messages", config: AIConfig{EndpointType: "messages"}, expected: "messages"},
		{name: "native anthropic", config: AIConfig{Provider: "anthropic", BaseURL: "https://api.anthropic.com"}, expected: "messages"},
		{name: "anthropic through sub2api", config: AIConfig{Provider: "anthropic", BaseURL: "https://sub2api.example"}, expected: "responses"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.Protocol(); got != tt.expected {
				t.Fatalf("Protocol() = %q, expected %q", got, tt.expected)
			}
		})
	}
}
