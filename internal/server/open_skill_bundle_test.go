package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/db"
)

func readSkillBundle(t *testing.T, srv *Server) map[string][]byte {
	t.Helper()
	rec := httptest.NewRecorder()
	srv.handler.ServeHTTP(rec, httptest.NewRequest("GET", "/open/v1/skills/well-ambient/archive", nil))
	if rec.Code != 200 {
		t.Fatalf("archive: %d %s", rec.Code, rec.Body.String())
	}
	gz, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	entries := map[string][]byte{}
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		body, e := io.ReadAll(tr)
		if e != nil {
			t.Fatal(e)
		}
		entries[h.Name] = body
	}
	return entries
}
func TestOpenSkillUnifiedBundleAndDisabledChild(t *testing.T) {
	srv, _ := setupAIGovernanceOpenCapabilityServer(t)
	entries := readSkillBundle(t, srv)
	discovered := 0
	for file, body := range entries {
		if path.Base(file) == "SKILL.md" {
			discovered++
		}
		if strings.Contains(string(body), "well-jira_") {
			t.Fatalf("legacy tool in %s", file)
		}
	}
	if discovered != 1 {
		t.Fatalf("discovered %d entrypoints", discovered)
	}
	for _, child := range officialSkills {
		if len(entries["well-ambient/modules/"+child.name+"/WORKFLOW.md"]) == 0 {
			t.Fatalf("missing %s", child.name)
		}
	}
	root := string(entries["well-ambient/SKILL.md"])
	if !strings.Contains(root, "modules/runtime-knowledge/WORKFLOW.md") {
		t.Fatal("root router missing child")
	}
	srv.config.OpenCapabilities.DisabledSkills = []string{"runtime-knowledge"}
	entries = readSkillBundle(t, srv)
	for file, body := range entries {
		if strings.Contains(file, "modules/runtime-knowledge/") {
			t.Fatal("unpublished module remains")
		}
		if file == "well-ambient/SKILL.md" && strings.Contains(string(body), "modules/runtime-knowledge/") {
			t.Fatal("unpublished link remains")
		}
	}
	if strings.Contains(string(entries["well-ambient/capability-manifest.yaml"]), "knowledge_get_context") {
		t.Fatal("unpublished tools remain")
	}
	catalog := srv.openSkillsCatalog("")
	if len(catalog) != 1 || len(catalog[0].Children) != 4 || len(catalog[0].Tools) != 10 {
		t.Fatalf("catalog mismatch: %+v", catalog)
	}
}
func TestOpenSkillObservationBoundedRedactedAndAuthorized(t *testing.T) {
	srv, token := setupAIGovernanceOpenCapabilityServer(t)
	now := time.Now().UTC()
	for i := 0; i < 25; i++ {
		outcome := "succeeded"
		code := ""
		if i == 0 {
			outcome = "failed"
			code = "forbidden"
		}
		if i == 1 {
			outcome = "started"
		}
		row := db.CapabilityInvocation{RequestID: fmt.Sprintf("observed-%d", i), SourceID: "integration-a", KeyID: "private-key-id", TargetRefsJSON: `["private-target"]`, Transport: "mcp", ToolName: "knowledge_get_context", Outcome: outcome, ErrorCode: code, DurationMS: 40, CreatedAt: now.Add(-time.Duration(i+1) * time.Minute)}
		if err := db.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i, when := range []time.Time{now.Add(-25 * time.Hour), now.Add(time.Hour)} {
		db.DB.Create(&db.CapabilityInvocation{RequestID: fmt.Sprintf("outside-%d", i), SourceID: "integration-a", ToolName: "knowledge_get_context", Outcome: "succeeded", CreatedAt: when})
	}
	request := func(auth string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/ai-governance/open-skills/overview", nil)
		if auth != "" {
			r.Header.Set("Authorization", "Bearer "+auth)
		}
		w := httptest.NewRecorder()
		srv.handler.ServeHTTP(w, r)
		return w
	}
	if rec := request(""); rec.Code == 200 {
		t.Fatal("anonymous observation exposed")
	}
	reader := performanceMemberToken(t, "observation-reader@example.test", []string{"ai_context:read"})
	if rec := request(reader); rec.Code != 403 {
		t.Fatalf("non-admin observation: %d", rec.Code)
	}
	rec := request(token)
	if rec.Code != 200 {
		t.Fatalf("observation %d %s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private-")) {
		t.Fatal("audit metadata leaked")
	}
	var body struct {
		Subskills []openSubskillObservation `json:"subskills"`
		Recent    []openSkillCall           `json:"recent_calls"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Recent) != 20 {
		t.Fatalf("recent unbounded: %d", len(body.Recent))
	}
	found := false
	for _, metric := range body.Subskills {
		if metric.ID == "runtime-knowledge" {
			found = true
			if metric.Calls != 25 || metric.Succeeded != 23 || metric.Failed != 1 || metric.InProgress != 1 || metric.SuccessRate == nil || metric.AverageMS == nil || *metric.AverageMS != 40 {
				t.Fatalf("metrics: %+v", metric)
			}
		} else if metric.SuccessRate != nil || metric.AverageMS != nil {
			t.Fatal("empty metrics fabricated")
		}
	}
	if !found {
		t.Fatal("missing knowledge metric")
	}
}
func TestOpenSkillRegistrationSeedsParentAndChildren(t *testing.T) {
	srv, _ := setupAIGovernanceOpenCapabilityServer(t)
	srv.ensureAgentRuntime()
	keys := []string{"well-ambient", "well-ambient.open-capabilities-mcp", "well-ambient.jira-analysis", "well-ambient.board-decision", "well-ambient.code-review-reader", "well-ambient.runtime-knowledge"}
	for _, key := range keys {
		var count int64
		if err := db.DB.Model(&db.Capability{}).Where("capability_key = ?", key).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("missing registry entry %s: %d %v", key, count, err)
		}
	}
}
