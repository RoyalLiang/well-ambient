package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"well-ambient/internal/db"
	"well-ambient/internal/openaccess"
	"well-ambient/internal/runtimeknowledge"
)

func TestRuntimeKnowledgeHTTPAndMCPSharePolicyEvidenceAndAudit(t *testing.T) {
	srv, credential, policy := setupOpenCapabilityServer(t)
	ctx := context.Background()
	fact := db.ContextFact{Scope: "global", Status: "active", Summary: "并发调度能力边界", Content: "车辆场位需要原子提交；现场故障切换尚未验证", Version: 1}
	if err := db.DB.Create(&fact).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := srv.runtimeKnowledge().Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body, secret string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if secret != "" {
			r.Header.Set("Authorization", "Bearer "+secret)
		}
		w := httptest.NewRecorder()
		srv.handler.ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "/open/v1/knowledge", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", w.Code)
	}
	if w := call("GET", "/open/v1/knowledge", "", credential.Secret); w.Code != http.StatusForbidden {
		t.Fatalf("legacy policy implicitly publishes knowledge: %d", w.Code)
	}
	policy.Actions = append(policy.Actions, "knowledge.read")
	policy.DataRules["knowledge_global_context"] = "allow"
	policy.FieldRules["knowledge_get_context"] = []string{"context_fact"}
	var err error
	policy, err = srv.openAccess.ActivatePolicy(ctx, policy.PolicySpec, "test-admin")
	if err != nil {
		t.Fatal(err)
	}
	request := `{"text":"并发调度","token_budget":1800}`
	w := call("POST", "/open/v1/knowledge/context", request, credential.Secret)
	if w.Code != http.StatusOK {
		t.Fatalf("context=%d %s", w.Code, w.Body.String())
	}
	var pack runtimeknowledge.ContextResponse
	if err := json.Unmarshal(w.Body.Bytes(), &pack); err != nil {
		t.Fatal(err)
	}
	if len(pack.Evidence) != 1 || !strings.Contains(pack.Compact, "现场故障切换") {
		t.Fatalf("missing evidence: %+v", pack)
	}
	invoker := openMCPLocalInvoker{server: srv, call: openaccess.CallContext{Identity: openaccess.Identity{SourceID: "trusted-agent", KeyID: credential.KeyID}, Policy: policy}}
	mcp, err := invoker.Invoke(ctx, "knowledge_get_context", json.RawMessage(request))
	if err != nil {
		t.Fatal(err)
	}
	if mcp.Value["context_hash"] != pack.ContextHash || len(mcp.TargetRefs) != 1 {
		t.Fatalf("HTTP and MCP diverged: %+v", mcp)
	}
	body, _ := json.Marshal(runtimeknowledge.EvidenceRequest{SnapshotID: pack.SnapshotID, SourceRef: pack.Evidence[0].SourceRef})
	w = call("POST", "/open/v1/knowledge/evidence", string(body), credential.Secret)
	if w.Code != http.StatusOK {
		t.Fatalf("evidence=%d %s", w.Code, w.Body.String())
	}
	if err := db.DB.Model(&fact).Update("status", "archived").Error; err != nil {
		t.Fatal(err)
	}
	w = call("POST", "/open/v1/knowledge/evidence", string(body), credential.Secret)
	if w.Code != http.StatusNotFound {
		t.Fatalf("withdrawn evidence status=%d", w.Code)
	}
	var audits int64
	if err := db.DB.Model(&db.CapabilityInvocation{}).Where("tool_name = ? AND outcome = ?", "knowledge_get_context", "succeeded").Count(&audits).Error; err != nil || audits != 2 {
		t.Fatalf("HTTP/MCP audit count=%d err=%v", audits, err)
	}
	srv.openFeatures.Read = false
	w = call("POST", "/open/v1/knowledge/context", request, credential.Secret)
	if w.Code != http.StatusForbidden {
		t.Fatalf("disabled read accepted: %d", w.Code)
	}
}

func TestRuntimeKnowledgeAnonymousAdministrationIsDenied(t *testing.T) {
	srv, _, _ := setupOpenCapabilityServer(t)
	for _, route := range []struct{ method, path string }{{"GET", "/api/agent-runtime/knowledge/settings"}, {"PUT", "/api/agent-runtime/knowledge/settings"}, {"POST", "/api/agent-runtime/knowledge/refresh"}} {
		r := httptest.NewRequest(route.method, route.path, nil)
		w := httptest.NewRecorder()
		srv.handler.ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
			t.Fatalf("anonymous knowledge administration status=%d: %s", w.Code, route.path)
		}
	}
}

func TestRuntimeKnowledgeAdminCanConfigureAndManuallyRefresh(t *testing.T) {
	srv, _, _ := setupOpenCapabilityServer(t)
	token := superAdminToken(t, "knowledge-admin@example.com", "Knowledge Admin", []string{"ai_context:read", "ai_context:write"})
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		srv.handler.ServeHTTP(w, r)
		return w
	}
	w := call("GET", "/api/agent-runtime/knowledge/settings", "")
	if w.Code != http.StatusOK {
		t.Fatalf("admin settings=%d %s", w.Code, w.Body.String())
	}
	var state db.RuntimeKnowledgeState
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(runtimeknowledge.SettingsRequest{Enabled: false, IntervalMinutes: 60, ExpectedVersion: state.Version})
	w = call("PUT", "/api/agent-runtime/knowledge/settings", string(body))
	if w.Code != http.StatusOK {
		t.Fatalf("configure=%d %s", w.Code, w.Body.String())
	}
	w = call("PUT", "/api/agent-runtime/knowledge/settings", string(body))
	if w.Code != http.StatusConflict {
		t.Fatalf("stale save=%d", w.Code)
	}
	w = call("POST", "/api/agent-runtime/knowledge/refresh", "")
	if w.Code != http.StatusOK {
		t.Fatalf("manual refresh=%d %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if state.Enabled || state.IntervalMinutes != 60 || state.GenerationID == 0 {
		t.Fatalf("wrong settings %+v", state)
	}
}
