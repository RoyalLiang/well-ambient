package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"well-ambient/internal/config"
)

func TestAIHealthCheckForwardsReasoning(t *testing.T) {
	for _, protocol := range []string{"responses", "messages"} {
		t.Run(protocol, func(t *testing.T) {
			called := false
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				key := "reasoning"
				if protocol == "messages" {
					key = "output_config"
				}
				effort, ok := body[key].(map[string]any)
				if !ok || effort["effort"] != "xhigh" {
					t.Errorf("effort missing: %v", body)
				}
				if _, limited := body["max_output_tokens"]; limited {
					t.Error("reasoning health check must not cap Responses output")
				}
				w.Header().Set("Content-Type", "application/json")
				if protocol == "messages" {
					if body["max_tokens"].(float64) <= 5 {
						t.Error("Claude token cap too small")
					}
					w.Write([]byte(`{"content":[{"type":"text","text":"pong"}]}`))
				} else {
					w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"pong"}]}]}`))
				}
			}))
			defer upstream.Close()
			cfg := config.AIConfig{BaseURL: upstream.URL, APIToken: "fixture", EndpointType: protocol, ReasoningEffort: "xhigh"}
			ok, _, details := testAIConnection(context.Background(), &cfg)
			if !called || !ok {
				t.Fatalf("health check failed: %s", details)
			}
		})
	}
}
