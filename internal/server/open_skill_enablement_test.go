package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/db"
)

func TestOpenSkillEnablementPersistsAndControlsDistribution(t *testing.T) {
	srv, token := setupAIGovernanceOpenCapabilityServer(t)
	request := func(method, path string, body any, auth string) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(data))
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		rec := httptest.NewRecorder()
		srv.handler.ServeHTTP(rec, req)
		return rec
	}
	version := func() int {
		var current db.RuntimeConfig
		if err := db.DB.First(&current, runtimeConfigSingletonID).Error; err != nil {
			t.Fatal(err)
		}
		return current.Version
	}
	save := func(disabled *[]string, v int) *httptest.ResponseRecorder {
		return request(http.MethodPut, "/api/ai-governance/open-capabilities/config", updateOpenCapabilitiesConfigRequest{DisabledSkills: disabled, ExpectedVersion: v}, token)
	}
	for _, skill := range srv.openSkillsCatalog("") {
		if !skill.Enabled {
			t.Fatal("legacy config must default to enabled")
		}
	}
	initial := version()
	disabled := []string{"well-jira-analysis", "jira-analysis"}
	if rec := save(&disabled, initial); rec.Code != 200 {
		t.Fatalf("disable: %d %s", rec.Code, rec.Body.String())
	}
	var stored db.RuntimeConfig
	if err := db.DB.First(&stored, runtimeConfigSingletonID).Error; err != nil {
		t.Fatal(err)
	}
	restored, err := restoreVersionedConfig(*srv.config, stored.ConfigJSON)
	if err != nil || len(restored.OpenCapabilities.DisabledSkills) != 1 || restored.OpenCapabilities.DisabledSkills[0] != "jira-analysis" {
		t.Fatalf("database persistence: %v %+v", err, restored.OpenCapabilities)
	}
	for _, path := range []string{"/open/v1/skills/jira-analysis/archive", "/open/v1/skills/well-jira-analysis/archive"} {
		if rec := request(http.MethodGet, path, nil, ""); rec.Code != http.StatusOK {
			t.Fatalf("disabled archive %s: %d", path, rec.Code)
		}
	}
	if rec := request(http.MethodGet, "/open/v1/skills/well-jira-analysis/skill.md", nil, ""); rec.Code != 200 {
		t.Fatal("disabled Skill documentation must remain accessible")
	}
	for _, path := range []string{"/open/v1/skills", "/api/ai-governance/open-capabilities/overview"} {
		rec := request(http.MethodGet, path, nil, token)
		var list []openSkillSummary
		if strings.HasPrefix(path, "/api/") {
			var overview openCapabilitiesOverviewResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &overview); err != nil {
				t.Fatal(err)
			}
			list = overview.Skills
		} else if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
			t.Fatal(err)
		}
		if len(list) != 1 || list[0].Children[0].Enabled || !list[0].Children[1].Enabled {
			t.Fatalf("catalogue lost state: %s", rec.Body.String())
		}
	}
	if rec := save(&disabled, initial); rec.Code != 409 {
		t.Fatal("stale update accepted")
	}
	invalid := []string{"../../other"}
	if rec := save(&invalid, version()); rec.Code != 400 {
		t.Fatal("unknown Skill accepted")
	}
	if rec := save(nil, version()); rec.Code != 200 {
		t.Fatal("legacy request rejected")
	}
	if srv.openSkillEnabled("jira-analysis") {
		t.Fatal("legacy config client reset enablement")
	}
	full := *srv.config
	full.OpenCapabilities.DisabledSkills = nil
	if rec := request(http.MethodPost, "/api/config", full, token); rec.Code != 200 {
		t.Fatalf("legacy full config: %d %s", rec.Code, rec.Body.String())
	}
	if srv.openSkillEnabled("jira-analysis") {
		t.Fatal("legacy full config reset enablement")
	}
	// No token can modify configuration, while the catalogue is intentionally public.
	if rec := request(http.MethodPut, "/api/ai-governance/open-capabilities/config", updateOpenCapabilitiesConfigRequest{DisabledSkills: &disabled, ExpectedVersion: version()}, ""); rec.Code == 200 {
		t.Fatal("unauthenticated enablement update accepted")
	}
	prefix := "well-ambient"
	if rec := request(http.MethodPut, "/api/ai-governance/open-capabilities/config", updateOpenCapabilitiesConfigRequest{NamePrefix: &prefix, ExpectedVersion: version()}, token); rec.Code != 200 {
		t.Fatal("prefix update failed")
	}
	if list := srv.openSkillsCatalog(""); list[0].Children[0].Enabled || list[0].Name != "well-ambient" {
		t.Fatal("renaming reset enablement")
	}
	enabled := []string{}
	if rec := save(&enabled, version()); rec.Code != 200 {
		t.Fatalf("enable: %d %s", rec.Code, rec.Body.String())
	}
	if rec := request(http.MethodGet, "/open/v1/skills/team-2-jira-analysis/archive", nil, ""); rec.Code != 200 {
		t.Fatalf("re-enabled archive: %d", rec.Code)
	}
}

func TestOpenSkillInstallerSkipsDisabledPackages(t *testing.T) {
	srv, _ := setupAIGovernanceOpenCapabilityServer(t)
	srv.config.OpenCapabilities.DisabledSkills = []string{"jira-analysis"}
	server := httptest.NewServer(srv.mux)
	defer server.Close()
	srv.config.Server.PublicURL = server.URL
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/open/v1/install.sh", nil))
	script := rec.Body.String()
	run := func(args ...string) (string, error) {
		command := exec.Command("bash", append([]string{"-s", "--"}, args...)...)
		command.Stdin = strings.NewReader(script)
		output, err := command.CombinedOutput()
		return string(output), err
	}
	root := t.TempDir()
	if output, err := run("--root", root, "--skip-mcp"); err != nil {
		t.Fatalf("installer: %v %s", err, output)
	}
	for _, name := range []string{"board-decision", "code-review-reader"} {
		if _, err := os.Stat(filepath.Join(root, "skills", "well-ambient", "modules", name, "WORKFLOW.md")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "skills/well-ambient/modules/jira-analysis")); !os.IsNotExist(err) {
		t.Fatal("disabled Skill installed")
	}
	if output, err := run("--root", root, "--skill", "jira-analysis", "--skip-mcp"); err != nil || !strings.Contains(output, "已归入 well-ambient") {
		t.Fatalf("disabled explicit install: %v %s", err, output)
	}
	srv.config.OpenCapabilities.DisabledSkills = []string{"jira-analysis", "board-decision", "code-review-reader", "runtime-knowledge"}
	rec = httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/open/v1/install.sh", nil))
	script = rec.Body.String()
	if output, err := run("--yes", "--skip-mcp"); err == nil || !strings.Contains(output, "暂无已启用") {
		t.Fatalf("empty catalogue installer: %v %s", err, output)
	}
}

func TestOpenSkillUpgradeReplacesWithdrawnModuleAndBacksUpLegacy(t *testing.T) {
	srv, _ := setupAIGovernanceOpenCapabilityServer(t)
	srv.config.OpenCapabilities.DisabledSkills = []string{"runtime-knowledge"}
	host := httptest.NewServer(srv.mux)
	defer host.Close()
	root := t.TempDir()
	for _, file := range []string{"skills/well-ambient/modules/runtime-knowledge/WORKFLOW.md", "skills/jira-analysis/SKILL.md"} {
		full := filepath.Join(root, file)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("---\nname: jira-analysis\n---\nuser modification\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("bash", "-s", "--", "--root", root, "--skip-mcp")
	cmd.Stdin = strings.NewReader(renderInstallScript(host.URL))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("upgrade: %v %s", err, out)
	}
	for _, file := range []string{"skills/well-ambient/modules/runtime-knowledge", "skills/jira-analysis"} {
		if _, err := os.Stat(filepath.Join(root, file)); !os.IsNotExist(err) {
			t.Fatalf("stale entry remained %s: %v", file, err)
		}
	}
	backups, err := filepath.Glob(filepath.Join(root, "skill-backups", "well-ambient.*", "jira-analysis", "SKILL.md"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("missing reversible migration: %v %v", backups, err)
	}
	body, _ := os.ReadFile(backups[0])
	if !strings.Contains(string(body), "user modification") {
		t.Fatal("legacy edits lost")
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "well-ambient", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func TestOpenSkillInstallersSerializeBeforeDownload(t *testing.T) {
	srv, _ := setupAIGovernanceOpenCapabilityServer(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/archive") {
			close(entered)
			<-release
		}
		srv.mux.ServeHTTP(w, r)
	}))

	root := t.TempDir()
	script := renderInstallScript(host.URL)
	command := func() *exec.Cmd {
		cmd := exec.Command("bash", "-s", "--", "--root", root, "--skip-mcp")
		cmd.Stdin = strings.NewReader(script)
		return cmd
	}
	first := command()
	var output bytes.Buffer
	first.Stdout = &output
	first.Stderr = &output
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		host.Close()
	})
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("first installer did not reach download")
	}
	secondOutput, err := command().CombinedOutput()
	if err == nil || !strings.Contains(string(secondOutput), "已有安装正在运行") {
		t.Fatalf("second installer entered mutation: %v %s", err, secondOutput)
	}
	close(release)
	if err := first.Wait(); err != nil {
		t.Fatalf("first failed: %v %s", err, output.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".well-ambient-install.lock")); !os.IsNotExist(err) {
		t.Fatalf("lock not released: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "well-ambient", "well-ambient")); !os.IsNotExist(err) {
		t.Fatal("nested duplicate created")
	}
}
