package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func prepareOpenArchiveFixture(t *testing.T) string {
	t.Helper()
	t.Chdir(t.TempDir())
	dir := filepath.Join(".agents", "skills", "jira-analysis")
	for _, name := range []string{"well-ambient", "jira-analysis", "board-decision", "code-review-reader", "runtime-knowledge"} {
		folder := filepath.Join(".agents", "skills", name)
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, "SKILL.md"), []byte("skill fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, "capability-manifest.yaml"), []byte("id: "+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func requestOpenArchive() *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "/open/v1/skills/jira-analysis/archive", nil)
	r.SetPathValue("name", "jira-analysis")
	w := httptest.NewRecorder()
	(&Server{}).handleGetOpenSkillArchive(w, r)
	return w
}

func TestOpenSkillArchiveDoesNotStreamPackingFailures(t *testing.T) {
	dir := prepareOpenArchiveFixture(t)
	// This forces an os.Open failure after the tar header has been written,
	// independent of the test runner's UID or filesystem permissions.
	if err := os.Symlink("missing-file", filepath.Join(dir, "zz-unreadable")); err != nil {
		t.Fatal(err)
	}
	w := requestOpenArchive()
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("packing failure returned HTTP %d; body_hex=%x", w.Code, w.Body.Bytes())
	}
	if strings.Contains(w.Header().Get("Content-Type"), "gzip") || bytes.HasPrefix(w.Body.Bytes(), []byte{0x1f, 0x8b}) {
		t.Fatal("packing failure must not be presented as a gzip archive")
	}
	if !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
		t.Fatal("packing failure must not be cached")
	}
	if strings.Contains(w.Body.String(), dir) {
		t.Fatal("error response exposes a server filesystem path")
	}
}

func TestOpenSkillArchiveReturnsCompleteUncacheableGzip(t *testing.T) {
	prepareOpenArchiveFixture(t)
	w := requestOpenArchive()
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
		t.Fatal("archive URL must not cache bytes across image versions")
	}
	gz, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(bytes.NewReader(content))
	entries := map[string]string{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		entries[header.Name] = string(data)
	}
	if entries["well-ambient/SKILL.md"] != "skill fixture\n" || entries["well-ambient/modules/jira-analysis/WORKFLOW.md"] != "skill fixture\n" {
		t.Fatalf("missing root or module: %v", entries)
	}
	count := 0
	for name := range entries {
		if filepath.Base(name) == "SKILL.md" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("want one discovered skill, got %d", count)
	}

}

func TestOpenInstallScriptRejectsCorruptArchiveBeforeWriting(t *testing.T) {
	clientRoot := t.TempDir()
	existing := filepath.Join(clientRoot, "skills", "jira-analysis", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(existing), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("existing skill\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Header.Get("Cache-Control") != "no-cache" {
			t.Error("installer must request a fresh archive")
		}
		// Reproduce the old endpoint's gzip header followed by plain error text.
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(append([]byte{0x1f, 0x8b, 8, 0, 0, 0, 0, 0, 0, 0xff}, []byte("Failed to pack skill\n")...))
	}))
	defer server.Close()
	command := exec.Command("/bin/bash", "--noprofile", "--norc", "-c", renderInstallScript(server.URL), "installer", "--root", clientRoot, "--skill", "jira-analysis")
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("corrupt download reported success: %s", output)
	}
	if !strings.Contains(string(output), "技能包校验失败") {
		t.Fatalf("missing archive-specific failure: %s", output)
	}
	content, err := os.ReadFile(existing)
	if err != nil || string(content) != "existing skill\n" {
		t.Fatalf("corrupt download changed an installed skill: %q, %v", content, err)
	}
}
