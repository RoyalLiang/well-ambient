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
			name:         "Pure domain completions",
			baseURL:      "https://ai-pixel.online",
			endpointType: "completions",
			expected:     "https://ai-pixel.online/v1/chat/completions",
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
			expected:     "https://ai-pixel.online/v1/chat/completions",
		},
		{
			name:         "Already complete completions path",
			baseURL:      "https://ai-pixel.online/v1/chat/completions",
			endpointType: "completions",
			expected:     "https://ai-pixel.online/v1/chat/completions",
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
			expected:     "https://ai-pixel.online/V1/chat/completions",
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
