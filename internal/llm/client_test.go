package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestDefaultHTTPClientTimeouts(t *testing.T) {
	client := Client{}
	if got := client.httpClient(false).Timeout; got != defaultRequestTimeout {
		t.Fatalf("non-stream timeout = %s, want %s", got, defaultRequestTimeout)
	}
	if got := client.httpClient(true).Timeout; got != 0 {
		t.Fatalf("stream timeout = %s, want no overall timeout", got)
	}

	custom := &http.Client{Timeout: 2 * time.Second}
	if got := (Client{HTTPClient: custom}).httpClient(true); got != custom {
		t.Fatal("custom HTTP client was not preserved")
	}
}

func TestStreamStillHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := Client{
		Config: config.AIConfig{
			BaseURL:      "http://local-validation.invalid",
			EndpointType: "responses",
			APIToken:     "token",
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		})},
	}
	_, err := client.Stream(ctx, Request{UserPrompt: "cancel"}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("stream error = %v, want context cancellation", err)
	}
}

func TestResponsesGenerateNormalizesLegacyCompletionsConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if _, exists := payload["messages"]; exists {
			t.Fatalf("legacy messages payload leaked: %#v", payload)
		}
		if payload["instructions"] != "system" || payload["stream"] != false {
			t.Fatalf("payload = %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"output":[{"type":"message","content":[{"type":"output_text","text":"ready"}]}]}`)
	}))
	defer server.Close()

	client := Client{Config: config.AIConfig{
		BaseURL: server.URL + "/v1/chat/completions", EndpointType: "completions", APIToken: "token", Model: "test-model",
	}}
	got, err := client.Generate(context.Background(), Request{SystemPrompt: "system", UserPrompt: "user"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "ready" {
		t.Fatalf("text = %q", got)
	}
}

func TestResponsesStreamAccumulatesOutputTextDeltasAndFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["stream"] != true {
			t.Fatalf("stream = %#v", payload["stream"])
		}
		encoded, _ := json.Marshal(payload["input"])
		if !strings.Contains(string(encoded), `"type":"input_file"`) || !strings.Contains(string(encoded), `"file_id":"file-1"`) {
			t.Fatalf("input = %s", encoded)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: response.output_text.delta\n")
		fmt.Fprint(w, `data: {"type":"response.output_text.delta","delta":"hello "}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"response.output_text.delta","delta":"world"}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"response.completed","response":{"output":[{"type":"message","content":[{"type":"output_text","text":"hello world"}]}]}}`+"\n\n")
	}))
	defer server.Close()

	client := Client{Config: config.AIConfig{BaseURL: server.URL, EndpointType: "responses", APIToken: "token"}}
	var deltas strings.Builder
	got, err := client.Stream(context.Background(), Request{UserPrompt: "user", FileIDs: []string{"file-1"}}, func(delta string) error {
		deltas.WriteString(delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello world" || deltas.String() != got {
		t.Fatalf("got = %q deltas = %q", got, deltas.String())
	}
}

func TestResponsesGeneratePassesOpaqueFileBytesAsInputFile(t *testing.T) {
	wantBytes := []byte{0x00, 0x7f, 0xff, 'o', 'p', 'a', 'q', 'u', 'e'}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Input []struct {
				Content []struct {
					Type     string `json:"type"`
					Text     string `json:"text"`
					Filename string `json:"filename"`
					FileData string `json:"file_data"`
				} `json:"content"`
			} `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Input) != 1 || len(payload.Input[0].Content) != 2 {
			t.Fatalf("input payload = %#v", payload)
		}
		filePart := payload.Input[0].Content[0]
		if filePart.Type != "input_file" || filePart.Filename != "rules.md" {
			t.Fatalf("file part = %#v", filePart)
		}
		const prefix = "data:text/markdown;base64,"
		if !strings.HasPrefix(filePart.FileData, prefix) {
			t.Fatalf("file_data = %q", filePart.FileData)
		}
		gotBytes, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(filePart.FileData, prefix))
		if err != nil {
			t.Fatal(err)
		}
		if string(gotBytes) != string(wantBytes) {
			t.Fatalf("opaque bytes changed: got=%v want=%v", gotBytes, wantBytes)
		}
		textPart := payload.Input[0].Content[1]
		if textPart.Type != "input_text" || textPart.Text != "extract structure only" {
			t.Fatalf("text part = %#v", textPart)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"output_text":"ready"}`)
	}))
	defer server.Close()

	client := Client{Config: config.AIConfig{BaseURL: server.URL, EndpointType: "responses", APIToken: "token"}}
	got, err := client.Generate(context.Background(), Request{
		UserPrompt: "extract structure only",
		Files:      []FileInput{{Name: "rules.md", MIMEType: "text/markdown; charset=utf-8", Data: wantBytes}},
	})
	if err != nil || got != "ready" {
		t.Fatalf("generate = %q err=%v", got, err)
	}
}

func TestClaudeMessagesRejectsFilePassthroughExplicitly(t *testing.T) {
	_, err := buildPayload(config.AIConfig{EndpointType: "messages"}, Request{
		UserPrompt: "extract",
		Files:      []FileInput{{Name: "rules.md", MIMEType: "text/markdown", Data: []byte("opaque")}},
	}, false)
	if err == nil || !strings.Contains(err.Error(), "Responses protocol") {
		t.Fatalf("expected explicit protocol error, got %v", err)
	}
}

func TestClaudeMessagesGenerateAndStream(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "claude-token" || r.Header.Get("anthropic-version") == "" {
			t.Fatalf("request path=%q headers=%v", r.URL.Path, r.Header)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if _, exists := payload["instructions"]; exists {
			t.Fatalf("responses instructions leaked: %#v", payload)
		}
		if payload["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, `event: content_block_delta`+"\n")
			fmt.Fprint(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"streamed"}}`+"\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"type":"message","content":[{"type":"text","text":"generated"}]}`)
	}))
	defer server.Close()

	client := Client{Config: config.AIConfig{Provider: "claude", BaseURL: server.URL, EndpointType: "messages", APIToken: "claude-token"}}
	generated, err := client.Generate(context.Background(), Request{SystemPrompt: "system", UserPrompt: "user"})
	if err != nil || generated != "generated" {
		t.Fatalf("generate = %q err=%v", generated, err)
	}
	streamed, err := client.Stream(context.Background(), Request{SystemPrompt: "system", UserPrompt: "user"}, nil)
	if err != nil || streamed != "streamed" {
		t.Fatalf("stream = %q err=%v", streamed, err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
}
