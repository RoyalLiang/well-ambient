package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"well-ambient/internal/db"
)

func TestOpenNamespacePersistsWithVersionConflictAndLegacyClient(t *testing.T) {
	srv, token := setupAIGovernanceOpenCapabilityServer(t)
	var initial db.RuntimeConfig
	if err := db.DB.First(&initial, runtimeConfigSingletonID).Error; err != nil {
		t.Fatal(err)
	}
	update := func(prefix string, version int) *httptest.ResponseRecorder {
		body, _ := json.Marshal(updateOpenCapabilitiesConfigRequest{NamePrefix: &prefix, ExpectedVersion: version})
		req := httptest.NewRequest(http.MethodPut, "/api/ai-governance/open-capabilities/config", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		srv.handler.ServeHTTP(rec, req)
		return rec
	}
	if rec := update("team-2", initial.Version); rec.Code != 400 {
		t.Fatalf("custom namespace accepted: %d", rec.Code)
	}
	if rec := update("well", initial.Version); rec.Code != 200 {
		t.Fatalf("legacy migration: %d %s", rec.Code, rec.Body.String())
	}
	var current db.RuntimeConfig
	db.DB.First(&current, runtimeConfigSingletonID)
	if !strings.Contains(current.ConfigJSON, `"name_prefix":"well-ambient"`) || srv.openNamePrefix() != "well-ambient" {
		t.Fatal("namespace not canonicalized")
	}
	if rec := update("well-ambient", initial.Version); rec.Code != 409 {
		t.Fatal("stale version accepted")
	}
	for _, alias := range []string{"well-ambient", "jira-analysis", "well-jira-analysis", "well-ambient-runtime-knowledge"} {
		req := httptest.NewRequest("GET", "/open/v1/skills/"+alias, nil)
		rec := httptest.NewRecorder()
		srv.handler.ServeHTTP(rec, req)
		var parent openSkillSummary
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &parent) != nil || parent.Name != "well-ambient" || len(parent.Children) != 4 {
			t.Fatalf("alias %s: %d %s", alias, rec.Code, rec.Body.String())
		}
	}
}
func TestOpenMCPUsesFixedNamespaceAndCanonicalAudit(t *testing.T) {
	srv, credential, _ := setupOpenCapabilityServer(t)
	srv.config.OpenCapabilities.NamePrefix = "team-2"
	request := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+credential.Secret)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2026-07-28")
		var envelope struct {
			Method string
			Params struct{ Name string }
		}
		json.Unmarshal([]byte(body), &envelope)
		req.Header.Set("Mcp-Method", envelope.Method)
		if envelope.Params.Name != "" {
			req.Header.Set("Mcp-Name", envelope.Params.Name)
		}
		rec := httptest.NewRecorder()
		srv.handler.ServeHTTP(rec, req)
		return rec
	}
	meta := `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"prefix-test","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}`
	listed := request(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{` + meta + `}}`)
	var response struct {
		Result struct{ Tools []struct{ Name string } }
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Result.Tools) != 13 {
		t.Fatalf("tool count: %s", listed.Body.String())
	}
	for _, tool := range response.Result.Tools {
		if !strings.HasPrefix(tool.Name, "well-ambient-") {
			t.Fatalf("unprefixed tool: %s", tool.Name)
		}
	}
	called := request(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{` + meta + `,"name":"well-ambient-jira_describe_schema","arguments":{}}}`)
	if called.Code != http.StatusOK || strings.Contains(called.Body.String(), `"error"`) {
		t.Fatalf("prefixed call failed: %s", called.Body.String())
	}
	var count int64
	if err := db.DB.Model(&db.CapabilityInvocation{}).Where("tool_name = ?", "jira_describe_schema").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("canonical audit count=%d err=%v", count, err)
	}
}
