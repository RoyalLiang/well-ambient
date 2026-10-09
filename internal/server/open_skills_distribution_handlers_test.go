package server

import (
	"archive/tar"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"well-ambient/internal/config"
)

func runIsolatedOpenInstaller(t *testing.T, failedSkill string) (string, error) {
	t.Helper()
	workDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workDir, ".agents", "skills"), 0o700); err != nil {
		t.Fatal(err)
	}
	archiveDir := filepath.Join(workDir, "fixtures")
	if err := os.MkdirAll(archiveDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, skill := range []string{"jira-analysis", "board-decision", "code-review-reader"} {
		file, err := os.Create(filepath.Join(archiveDir, skill+".tar.gz"))
		if err != nil {
			t.Fatal(err)
		}
		compressed := gzip.NewWriter(file)
		archive := tar.NewWriter(compressed)
		content := "isolated skill fixture\n"
		if err := archive.WriteHeader(&tar.Header{Name: skill + "/SKILL.md", Mode: 0o600, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
		for _, close := range []func() error{archive.Close, compressed.Close, file.Close} {
			if err := close(); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Override every curl invocation; the script writes only to the temporary
	// working directory's .agents/skills, never a real client or user skill path.
	stub := `curl() {
  local target='' output='' skill=''
  while [ "$#" -gt 0 ]; do
    case "$1" in
      -o) output="$2"; shift 2 ;;
      http*) target="$1"; shift ;;
      *) shift ;;
    esac
  done
  case "$target" in */health) return 0 ;; esac
  skill="${target%/archive}"; skill="${skill##*/}"
  if [ "$skill" = "$WELL_AMBIENT_INSTALL_TEST_FAILED_SKILL" ]; then return 22; fi
  cp "$WELL_AMBIENT_INSTALL_TEST_ARCHIVES/$skill.tar.gz" "$output"
}
`
	command := exec.Command("bash", "--noprofile", "--norc", "-c", stub+renderInstallScript("https://source.example"))
	command.Dir = workDir
	command.Env = append(os.Environ(), "WELL_AMBIENT_INSTALL_TEST_ARCHIVES="+archiveDir, "WELL_AMBIENT_INSTALL_TEST_FAILED_SKILL="+failedSkill)
	output, err := command.CombinedOutput()
	return string(output), err
}

func TestOpenInstallScriptReportsPartialFailureAndExitsNonzero(t *testing.T) {
	output, err := runIsolatedOpenInstaller(t, "board-decision")
	if err == nil {
		t.Fatalf("partial installation reported success: %s", output)
	}
	if !strings.Contains(output, "已成功安装: jira-analysis code-review-reader") || !strings.Contains(output, "安装失败: board-decision") {
		t.Fatalf("partial installation did not describe actual results: %s", output)
	}
	if strings.Contains(output, "所有就绪") || strings.Contains(output, "✓ 技能安装完成") {
		t.Fatalf("partial installation claimed completion: %s", output)
	}
}

func TestOpenInstallScriptReportsAllSuccessfulDownloads(t *testing.T) {
	output, err := runIsolatedOpenInstaller(t, "")
	if err != nil {
		t.Fatalf("isolated successful installation: %v %s", err, output)
	}
	if !strings.Contains(output, "已成功安装: jira-analysis board-decision code-review-reader") || !strings.Contains(output, "所有就绪") {
		t.Fatalf("successful installation summary = %s", output)
	}
}

func TestOpenInstallScriptRejectsCredentialURLs(t *testing.T) {
	srv := &Server{config: &config.Config{}}
	request := httptest.NewRequest(http.MethodGet, "/open/v1/install.sh?token=must-not-reach-script", nil)
	recorder := httptest.NewRecorder()
	srv.handleGetOpenInstallScript(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("credential URL status = %d, want 400", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "must-not-reach-script") {
		t.Fatal("credential URL was reflected in the response")
	}
}

func TestOpenInstallScriptIsSecretFreeAndShellSafe(t *testing.T) {
	const baseURL = "https://source.example/$(printf shell-evaluated)/a'b"
	srv := &Server{config: &config.Config{Server: config.ServerConfig{PublicURL: baseURL}}}
	request := httptest.NewRequest(http.MethodGet, "/open/v1/install.sh", nil)
	request.Header.Set("Authorization", "Bearer header-must-not-reach-script")
	recorder := httptest.NewRecorder()
	srv.handleGetOpenInstallScript(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("public installer status = %d, want 200", recorder.Code)
	}
	script := recorder.Body.String()
	if strings.Contains(script, "header-must-not-reach-script") || strings.Contains(script, "&auth=") {
		t.Fatal("installer must not embed credentials or generate credential-bearing URLs")
	}
	if !strings.Contains(script, `"Authorization": "Bearer <YOUR_API_KEY>"`) {
		t.Fatal("installer must provide a local Authorization placeholder")
	}
	syntax := exec.Command("bash", "-n")
	syntax.Stdin = strings.NewReader(script)
	if output, err := syntax.CombinedOutput(); err != nil {
		t.Fatalf("installer shell syntax: %v %s", err, output)
	}
	for _, line := range strings.Split(script, "\n") {
		if !strings.HasPrefix(line, "BASE_URL=") {
			continue
		}
		// Evaluate only the base URL assignment; never run the installer.
		assignment := exec.Command("bash", "-c", line+"\nprintf '%s' \"$BASE_URL\"")
		output, err := assignment.Output()
		if err != nil || string(output) != baseURL {
			t.Fatalf("installer base URL was not preserved literally: %v %q", err, output)
		}
		return
	}
	t.Fatal("installer base URL assignment is missing")
}
