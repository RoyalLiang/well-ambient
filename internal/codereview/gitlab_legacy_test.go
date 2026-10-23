package codereview

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGitLabLegacyMRUsesChangesWhenDiffsEndpointIsUnavailable(t *testing.T) {
	f := newFixture(t)
	original := f.s.HTTP.Transport
	legacyCalls := 0
	f.s.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/diffs") {
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
		}
		if strings.HasSuffix(r.URL.Path, "/changes") {
			legacyCalls++
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"diff_refs":{"base_sha":"` + base + `","head_sha":"` + head + `"},"changes":[{"old_path":"main.go","new_path":"main.go","diff":"@@ -1 +1 @@\n-old\n+new"}]}`))}, nil
		}
		return original.RoundTrip(r)
	})
	snapshot, err := f.s.git().Snapshot(context.Background(), "10", "mr", "42")
	if err != nil || len(snapshot.Files) != 1 || legacyCalls != 1 {
		t.Fatalf("legacy GitLab MR cannot be reviewed: files=%d changes_calls=%d err=%v", len(snapshot.Files), legacyCalls, err)
	}
}

func TestGitLabLegacyMRNeverBypassesDiffAuthorizationFailure(t *testing.T) {
	f := newFixture(t)
	original := f.s.HTTP.Transport
	legacyCalled := false
	f.s.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/diffs") {
			return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
		}
		if strings.HasSuffix(r.URL.Path, "/changes") {
			legacyCalled = true
		}
		return original.RoundTrip(r)
	})
	if _, err := f.s.git().Snapshot(context.Background(), "10", "mr", "42"); err == nil || legacyCalled {
		t.Fatalf("forbidden diff should fail without fallback: fallback=%v err=%v", legacyCalled, err)
	}
}

func TestGitLabLegacyMROverflowRemainsPartial(t *testing.T) {
	f := newFixture(t)
	original := f.s.HTTP.Transport
	f.s.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/diffs") {
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
		}
		if strings.HasSuffix(r.URL.Path, "/changes") {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"overflow":true,"changes":[{"old_path":"main.go","new_path":"main.go","diff":"@@ -1 +1 @@\n-old\n+new"}]}`)), Header: make(http.Header)}, nil
		}
		return original.RoundTrip(r)
	})
	snapshot, err := f.s.git().Snapshot(context.Background(), "10", "mr", "42")
	if err != nil || snapshot.Complete || len(snapshot.Files) != 1 || len(snapshot.Gaps) == 0 {
		t.Fatalf("overflow misrepresented as complete: snapshot=%+v err=%v", snapshot, err)
	}
}
