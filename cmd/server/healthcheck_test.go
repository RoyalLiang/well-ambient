package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type healthcheckRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn healthcheckRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestCheckHealthEndpoint(t *testing.T) {
	t.Run("accepts a successful readiness response", func(t *testing.T) {
		client := &http.Client{Transport: healthcheckRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodGet || request.URL.String() != "http://127.0.0.1:8080/ready" {
				t.Fatalf("unexpected health request: %s %s", request.Method, request.URL)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("SETUP")),
				Header:     make(http.Header),
			}, nil
		})}

		if err := checkHealthEndpoint(context.Background(), client, "http://127.0.0.1:8080/ready"); err != nil {
			t.Fatalf("checkHealthEndpoint() error = %v", err)
		}
	})

	t.Run("rejects a non-success readiness response", func(t *testing.T) {
		client := &http.Client{Transport: healthcheckRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       io.NopCloser(strings.NewReader("NOT READY")),
				Header:     make(http.Header),
			}, nil
		})}

		err := checkHealthEndpoint(context.Background(), client, "http://127.0.0.1:8080/ready")
		if err == nil || !strings.Contains(err.Error(), "503") {
			t.Fatalf("checkHealthEndpoint() error = %v, want HTTP 503", err)
		}
	})

	t.Run("reports transport failures", func(t *testing.T) {
		client := &http.Client{Transport: healthcheckRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("connection refused")
		})}

		err := checkHealthEndpoint(context.Background(), client, "http://127.0.0.1:8080/ready")
		if err == nil || !strings.Contains(err.Error(), "connection refused") {
			t.Fatalf("checkHealthEndpoint() error = %v, want transport failure", err)
		}
	})
}
