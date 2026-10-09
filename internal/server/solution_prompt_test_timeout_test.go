package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/config"
)

type promptPreviewTransport func(*http.Request) (*http.Response, error)

func (fn promptPreviewTransport) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

func promptPreviewServer() *Server {
	return &Server{config: &config.Config{AI: config.AIConfig{
		Enabled: true, BaseURL: "https://model.test", EndpointType: "responses", APIToken: "test-token",
	}}}
}

func promptPreviewRequest() *http.Request {
	return httptest.NewRequest(http.MethodPost, "/api/solution-prompts/test",
		strings.NewReader(`{"system_prompt":"preview rules","markdown":"example"}`))
}

func setPromptPreviewTransport(t *testing.T, transport http.RoundTripper) {
	t.Helper()
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous })
}

func TestSolutionPromptPreviewBoundsModelRequest(t *testing.T) {
	called := false
	setPromptPreviewTransport(t, promptPreviewTransport(func(r *http.Request) (*http.Response, error) {
		called = true
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 3*time.Minute {
			t.Errorf("interactive preview model request has no usable bounded deadline: %v, %v", deadline, ok)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{"output_text":"preview"}`))}, nil
	}))
	w := httptest.NewRecorder()
	promptPreviewServer().handleTestSolutionPrompt(w, promptPreviewRequest())
	if !called || w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "preview") {
		t.Fatalf("preview called=%v, response=%d %s", called, w.Code, w.Body.String())
	}
}

func TestSolutionPromptPreviewReportsTimeout(t *testing.T) {
	setPromptPreviewTransport(t, promptPreviewTransport(func(*http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	}))
	w := httptest.NewRecorder()
	promptPreviewServer().handleTestSolutionPrompt(w, promptPreviewRequest())
	if w.Code != http.StatusGatewayTimeout || !strings.Contains(w.Body.String(), "超时") {
		t.Fatalf("timeout response=%d %s", w.Code, w.Body.String())
	}
}

func TestSolutionPromptPreviewPreservesCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	setPromptPreviewTransport(t, promptPreviewTransport(func(r *http.Request) (*http.Response, error) {
		if r.Context().Err() != context.Canceled {
			t.Error("preview detached from caller cancellation")
		}
		return nil, r.Context().Err()
	}))
	w := httptest.NewRecorder()
	promptPreviewServer().handleTestSolutionPrompt(w, promptPreviewRequest().WithContext(ctx))
	if w.Code != http.StatusRequestTimeout || !strings.Contains(w.Body.String(), "取消") {
		t.Fatalf("cancellation response=%d %s", w.Code, w.Body.String())
	}
}
