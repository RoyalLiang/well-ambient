package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/headroom"
)

func TestHeadroomDefaultHookPreservesProviderEnvelope(t *testing.T) {
	python := os.Getenv("WELL_AMBIENT_HEADROOM_TEST_PYTHON")
	if python == "" {
		t.Skip("real package integration requires package Python")
	}
	worker, _ := filepath.Abs("../../capabilities/plugins/headroom/worker.py")
	t.Setenv("WELL_AMBIENT_HEADROOM_PYTHON", python)
	t.Setenv("WELL_AMBIENT_HEADROOM_WORKER", worker)
	priorDB := db.DB
	t.Cleanup(func() { headroom.Default().Close(); db.DB = priorDB })
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&db.Capability{CapabilityKey: headroom.CapabilityKey, Kind: "plugin", Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	rows := make([]string, 200)
	for i := range rows {
		rows[i] = fmt.Sprintf(`{"id":%d,"status":"INFO","component":"default-agent-check","limit":2048}`, i)
	}
	prompt := "[" + strings.Join(rows, ",") + "]"
	for _, protocol := range []string{"responses", "completions", "messages"} {
		for _, stream := range []bool{false, true} {
			client := Client{Config: config.AIConfig{Enabled: true, BaseURL: "https://local-validation.invalid", EndpointType: protocol, Model: "gpt-4o", APIToken: "local-placeholder"}, HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				data, _ := json.Marshal(payload)
				body := string(data)
				if !strings.Contains(body, "SYSTEM-PROTECTED") || !strings.Contains(body, "2048") || len(body) >= len(prompt) {
					t.Fatalf("invalid optimized envelope for %s stream=%v bytes=%d", protocol, stream, len(body))
				}
				if protocol != "messages" && !strings.Contains(body, base64.StdEncoding.EncodeToString([]byte("FILE-PROTECTED"))) {
					t.Fatal("attachment changed")
				}
				if payload["stream"] != stream {
					t.Fatal("stream flag changed")
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("{}"))}, nil
			})}}
			var files []FileInput
			if protocol != "messages" {
				files = []FileInput{{Name: "fixture.png", MIMEType: "image/png", Data: []byte("FILE-PROTECTED")}}
			}
			response, _, err := client.do(context.Background(), Request{SystemPrompt: "SYSTEM-PROTECTED", UserPrompt: prompt, Files: files}, stream)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
		}
	}
	count := headroom.RuntimeStatus().Calls
	off := false
	client := Client{Config: config.AIConfig{Enabled: true, HeadroomEnabled: &off, BaseURL: "https://local-validation.invalid", EndpointType: "responses", Model: "gpt-4o", APIToken: "local-placeholder"}, HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		data, _ := json.Marshal(body)
		if len(data) < len(prompt) {
			t.Fatal("disabled path compressed input")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	})}}
	response, _, err := client.do(context.Background(), Request{SystemPrompt: "SYSTEM-PROTECTED", UserPrompt: prompt, FileIDs: []string{"file-protected"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if headroom.RuntimeStatus().Calls != count {
		t.Fatal("explicit disable still invoked Headroom")
	}
	if count < 6 {
		t.Fatalf("default hook missing: calls=%d", count)
	}
}
