package openmcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPInvokerUsesFixedOpenRoutesAndBearerCredential(t *testing.T) {
	var method, path, authorization, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, authorization = r.Method, r.URL.RequestURI(), r.Header.Get("Authorization")
		payload := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(payload)
		body = string(payload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"key":"WA-1"}]}`))
	}))
	defer server.Close()
	invoker, err := NewHTTPInvoker(server.URL, "wa_live_key.secret-value-long-enough", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	input, _ := json.Marshal(map[string]any{
		"filters": map[string]any{"projects": []string{"WA"}},
		"fields":  []string{"key"},
	})
	result, err := invoker.Invoke(context.Background(), "jira_search_issues", input)
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/open/v1/jira/issues/search" ||
		authorization != "Bearer wa_live_key.secret-value-long-enough" ||
		!strings.Contains(body, `"projects":["WA"]`) ||
		result.Value["items"] == nil {
		t.Fatalf("request = %s %s auth=%q body=%s result=%+v", method, path, authorization, body, result)
	}
}

func TestHTTPInvokerDoesNotAcceptArbitraryOrCredentialBearingURL(t *testing.T) {
	for _, raw := range []string{
		"",
		"file:///tmp/socket",
		"https://user:pass@example.com",
		"https://example.com?target=https://evil.example",
		"http://example.com",
	} {
		if _, err := NewHTTPInvoker(raw, "secret", nil); err == nil {
			t.Fatalf("base URL %q was accepted", raw)
		}
	}
}

func TestHTTPInvokerPreservesPublicErrorCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":{"code":"stale_plan","message":"plan expired","request_id":"req-1"}}`))
	}))
	defer server.Close()
	invoker, err := NewHTTPInvoker(server.URL, "secret", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = invoker.Invoke(context.Background(), "decision_get_operation", json.RawMessage(`{"operation_id":"op-1"}`))
	if err == nil || !strings.Contains(err.Error(), "plan expired") {
		t.Fatalf("public error = %v", err)
	}
}
