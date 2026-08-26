package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestHealthSeparatesLivenessFromDatabaseReadiness(t *testing.T) {
	previous := db.DB
	db.DB = nil
	defer func() { db.DB = previous }()

	server := &Server{}
	live := httptest.NewRecorder()
	server.handleLiveness(live, httptest.NewRequest(http.MethodGet, "/live", nil))
	if live.Code != http.StatusOK || live.Body.String() != "OK" {
		t.Fatalf("liveness = %d %q", live.Code, live.Body.String())
	}

	ready := httptest.NewRecorder()
	server.handleHealth(ready, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if ready.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness without database = %d %s", ready.Code, ready.Body.String())
	}
}

func TestHealthIsReadyAfterDatabaseInitialization(t *testing.T) {
	previous := db.DB
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("initialize database: %v", err)
	}
	defer func() {
		_ = db.Close()
		db.DB = previous
	}()

	server := &Server{}
	ready := httptest.NewRecorder()
	server.handleHealth(ready, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if ready.Code != http.StatusOK || ready.Body.String() != "OK" {
		t.Fatalf("readiness = %d %q", ready.Code, ready.Body.String())
	}
}

func TestStatusReportsInjectedBuildIdentity(t *testing.T) {
	previousBuildInfo := buildInfo
	defer func() { buildInfo = previousBuildInfo }()
	SetBuildInfo("1.2.3", "abc123", "2026-08-26T10:00:00Z")

	server := &Server{config: &config.Config{}}
	recorder := httptest.NewRecorder()
	server.handleStatus(recorder, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if payload["version"] != "1.2.3" || payload["commit"] != "abc123" || payload["build_time"] != "2026-08-26T10:00:00Z" {
		t.Fatalf("unexpected build identity: %#v", payload)
	}
}
