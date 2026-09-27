package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"well-ambient/internal/config"
)

func setupAIGovernanceOpenCapabilityServer(t *testing.T) (*Server, string) {
	t.Helper()
	setupServerTestDB(t)
	cfg := config.Config{
		Database: config.DatabaseConfig{Driver: "sqlite", DSN: ":memory:"},
		Server: config.ServerConfig{
			Port:      8080,
			PublicURL: "https://well-ambient.example.com",
		},
	}
	if err := BootstrapVersionedConfig(&cfg); err != nil {
		t.Fatalf("bootstrap versioned config: %v", err)
	}
	token := superAdminToken(
		t,
		"ai-governance-admin@westwell-lab.com",
		"AI Governance Admin",
		[]string{"solution_prompt:manage", "config:read", "config:write", "ai_context:read"},
	)
	return NewServer(&cfg, ""), token
}

func TestAIGovernanceOpenCapabilitiesOverviewDefaultDisabled(t *testing.T) {
	srv, token := setupAIGovernanceOpenCapabilityServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai-governance/open-capabilities/overview", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("overview status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp openCapabilitiesOverviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// Verify default disabled
	if resp.Config.Enabled || resp.Config.ReadEnabled || resp.Config.PrepareEnabled || resp.Config.ExecuteEnabled {
		t.Fatalf("expected open capabilities disabled by default, got: %+v", resp.Config)
	}
	if resp.ActiveFeatures.Read || resp.ActiveFeatures.Prepare || resp.ActiveFeatures.Execute {
		t.Fatalf("expected active features disabled by default, got: %+v", resp.ActiveFeatures)
	}

	// Verify MCP metadata
	if !strings.HasSuffix(resp.MCP.EndpointURL, "/mcp") {
		t.Fatalf("unexpected mcp endpoint: %s", resp.MCP.EndpointURL)
	}
	if len(resp.MCP.Tools) != 10 {
		t.Fatalf("expected 10 MCP tools, got %d", len(resp.MCP.Tools))
	}

	// Verify skills catalog
	if len(resp.Skills) != 3 {
		t.Fatalf("expected 3 official skills, got %d", len(resp.Skills))
	}
}

func TestAIGovernanceOpenCapabilitiesConfigUpdateAndDynamicReload(t *testing.T) {
	srv, token := setupAIGovernanceOpenCapabilityServer(t)

	// Step 1: Read current version
	req := httptest.NewRequest(http.MethodGet, "/api/ai-governance/open-capabilities/overview", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	var initialResp openCapabilitiesOverviewResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &initialResp)

	// Step 2: Update config to enable Read & Prepare
	updateBody := updateOpenCapabilitiesConfigRequest{
		Enabled:         true,
		ReadEnabled:     true,
		PrepareEnabled:  true,
		ExecuteEnabled:  false,
		ExpectedVersion: initialResp.Version,
	}
	bodyBytes, _ := json.Marshal(updateBody)
	putReq := httptest.NewRequest(http.MethodPut, "/api/ai-governance/open-capabilities/config", bytes.NewReader(bodyBytes))
	putReq.Header.Set("Authorization", "Bearer "+token)
	putReq.Header.Set("Content-Type", "application/json")
	putRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(putRec, putReq)

	if putRec.Code != http.StatusOK {
		t.Fatalf("update config status = %d, body = %s", putRec.Code, putRec.Body.String())
	}

	// Step 3: Verify dynamic in-memory features updated immediately
	if !srv.openFeatures.Read || !srv.openFeatures.Prepare || srv.openFeatures.Execute {
		t.Fatalf("expected Read=true, Prepare=true, Execute=false, got: %+v", srv.openFeatures)
	}

	// Step 4: Verify optimistic lock conflict with stale version
	staleReq := httptest.NewRequest(http.MethodPut, "/api/ai-governance/open-capabilities/config", bytes.NewReader(bodyBytes))
	staleReq.Header.Set("Authorization", "Bearer "+token)
	staleReq.Header.Set("Content-Type", "application/json")
	staleRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(staleRec, staleReq)

	if staleRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict on stale version, got %d", staleRec.Code)
	}
}

func TestAIGovernanceCredentialsIssueAndRevoke(t *testing.T) {
	srv, token := setupAIGovernanceOpenCapabilityServer(t)

	// Issue credential
	issueBody := issueCredentialRequest{
		SourceID: "test-agent",
		TTLHours: 24,
	}
	bodyBytes, _ := json.Marshal(issueBody)
	req := httptest.NewRequest(http.MethodPost, "/api/ai-governance/open-capabilities/credentials", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("issue credential status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var issued struct {
		KeyID     string `json:"key_id"`
		Key       string `json:"key"`
		KeyPrefix string `json:"key_prefix"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &issued); err != nil {
		t.Fatalf("unmarshal issued key: %v", err)
	}
	if issued.KeyID == "" || !strings.HasPrefix(issued.Key, "wa_live_") {
		t.Fatalf("unexpected issued credential: %+v", issued)
	}

	// Revoke credential
	revokeReq := httptest.NewRequest(http.MethodPost, "/api/ai-governance/open-capabilities/credentials/"+issued.KeyID+"/revoke", nil)
	revokeReq.Header.Set("Authorization", "Bearer "+token)
	revokeRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(revokeRec, revokeReq)

	if revokeRec.Code != http.StatusOK {
		t.Fatalf("revoke credential status = %d, body = %s", revokeRec.Code, revokeRec.Body.String())
	}
}

func TestOpenSkillsPublicDistributionEndpoints(t *testing.T) {
	srv, _ := setupAIGovernanceOpenCapabilityServer(t)

	// 1. GET /open/v1/skills
	req := httptest.NewRequest(http.MethodGet, "/open/v1/skills", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("skills list status = %d", rec.Code)
	}
	var skills []openSkillSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &skills); err != nil {
		t.Fatalf("unmarshal skills: %v", err)
	}
	if len(skills) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(skills))
	}

	// 2. GET /open/v1/skills/jira-analysis/manifest
	manifestReq := httptest.NewRequest(http.MethodGet, "/open/v1/skills/jira-analysis/manifest", nil)
	manifestRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(manifestRec, manifestReq)
	if manifestRec.Code != http.StatusOK {
		t.Fatalf("skill manifest status = %d", manifestRec.Code)
	}
	if !strings.Contains(manifestRec.Body.String(), "well-ambient.jira-analysis") {
		t.Fatalf("unexpected manifest content: %s", manifestRec.Body.String())
	}

	// 3. GET /open/v1/skills/jira-analysis/skill.md
	docReq := httptest.NewRequest(http.MethodGet, "/open/v1/skills/jira-analysis/skill.md", nil)
	docRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(docRec, docReq)
	if docRec.Code != http.StatusOK {
		t.Fatalf("skill doc status = %d", docRec.Code)
	}
	if !strings.Contains(docRec.Body.String(), "jira-analysis") {
		t.Fatalf("unexpected doc content: %s", docRec.Body.String())
	}

	// 4. GET /open/v1/skills/jira-analysis/archive
	archiveReq := httptest.NewRequest(http.MethodGet, "/open/v1/skills/jira-analysis/archive", nil)
	archiveRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(archiveRec, archiveReq)
	if archiveRec.Code != http.StatusOK {
		t.Fatalf("skill archive status = %d", archiveRec.Code)
	}
	if archiveRec.Header().Get("Content-Type") != "application/gzip" {
		t.Fatalf("unexpected Content-Type = %s", archiveRec.Header().Get("Content-Type"))
	}

	// Unpack and verify tar.gz
	gr, err := gzip.NewReader(archiveRec.Body)
	if err != nil {
		t.Fatalf("create gzip reader: %v", err)
	}
	tr := tar.NewReader(gr)
	foundSkillMD := false
	foundManifest := false
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read error: %v", err)
		}
		if strings.HasSuffix(header.Name, "SKILL.md") {
			foundSkillMD = true
		}
		if strings.HasSuffix(header.Name, "capability-manifest.yaml") {
			foundManifest = true
		}
	}
	if !foundSkillMD || !foundManifest {
		t.Fatalf("archive missing expected files: foundSkillMD=%v, foundManifest=%v", foundSkillMD, foundManifest)
	}

	// 5. GET /open/v1/install.sh
	scriptReq := httptest.NewRequest(http.MethodGet, "/open/v1/install.sh?token=wa_sec_test_secret_123", nil)
	scriptRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(scriptRec, scriptReq)
	if scriptRec.Code != http.StatusOK {
		t.Fatalf("install script status = %d", scriptRec.Code)
	}
	scriptContent := scriptRec.Body.String()
	if !strings.HasPrefix(scriptContent, "#!/usr/bin/env bash") {
		t.Fatalf("expected bash script, got prefix: %s", scriptContent[:30])
	}
	if !strings.Contains(scriptContent, "wa_sec_test_secret_123") {
		t.Fatal("expected token to be injected into install script")
	}
	if !strings.Contains(scriptContent, "mcpServers") || !strings.Contains(scriptContent, "cursor://") {
		t.Fatal("expected mcpServers and cursor scheme in install script")
	}
}
