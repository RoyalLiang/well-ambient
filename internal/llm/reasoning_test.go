package llm

import (
	"testing"
	"well-ambient/internal/config"
)

func TestReasoningEffortPayload(t *testing.T) {
	for _, protocol := range []string{"responses", "messages"} {
		for _, stream := range []bool{false, true} {
			for _, effort := range []string{"", "low", "middle", "medium", "high", "xhigh", " HIGH "} {
				cfg := config.AIConfig{EndpointType: protocol, ReasoningEffort: effort}
				payload, err := buildPayload(cfg, Request{UserPrompt: "hello"}, stream)
				if err != nil {
					t.Fatal(err)
				}
				key, other := "reasoning", "output_config"
				if protocol == "messages" {
					key, other = other, key
				}
				if _, ok := payload[other]; ok {
					t.Fatalf("%s included %s", protocol, other)
				}
				if effort == "" {
					if _, ok := payload[key]; ok {
						t.Fatal("default must omit effort")
					}
					continue
				}
				want := effort
				if effort == "middle" {
					want = "medium"
				}
				if effort == " HIGH " {
					want = "high"
				}
				if got := payload[key].(map[string]any)["effort"]; got != want {
					t.Fatalf("%s stream=%v effort=%q got %v", protocol, stream, effort, got)
				}
			}
		}
	}
	for _, protocol := range []string{"responses", "messages"} {
		if _, err := buildPayload(config.AIConfig{EndpointType: protocol, ReasoningEffort: "invalid"}, Request{}, false); err == nil {
			t.Fatal("invalid effort accepted")
		}
	}
}
