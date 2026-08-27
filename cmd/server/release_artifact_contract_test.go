package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseMetadataGeneratorHonorsExplicitOverrides(t *testing.T) {
	projectRoot := releaseContractProjectRoot(t)
	outputDir := t.TempDir()
	command := exec.Command("bash", filepath.Join(projectRoot, "scripts", "release-metadata.sh"), "--output-dir", outputDir)
	command.Dir = projectRoot
	command.Env = append(os.Environ(),
		"WELL_AMBIENT_VERSION=2026.08.27-test",
		"WELL_AMBIENT_COMMIT=0123456789abcdef0123456789abcdef01234567",
		"WELL_AMBIENT_BUILD_TIME=2026-08-27T01:02:03Z",
		"WELL_AMBIENT_RELEASE_BATCH=automated release batch",
		"WELL_AMBIENT_WORKTREE_DIRTY=false",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("release metadata generator failed: %v\n%s", err, output)
	}

	releaseEnv := readReleaseContractFile(t, filepath.Join(outputDir, "release.env"))
	for _, expected := range []string{
		"WELL_AMBIENT_VERSION=2026.08.27-test",
		"WELL_AMBIENT_COMMIT=0123456789abcdef0123456789abcdef01234567",
		"WELL_AMBIENT_BUILD_TIME=2026-08-27T01:02:03Z",
		`WELL_AMBIENT_RELEASE_BATCH=automated\ release\ batch`,
		"WELL_AMBIENT_WORKTREE_DIRTY=false",
	} {
		if !strings.Contains(releaseEnv, expected) {
			t.Errorf("release.env missing %q\n%s", expected, releaseEnv)
		}
	}

	releaseNotes := readReleaseContractFile(t, filepath.Join(outputDir, "release-notes.txt"))
	for _, expected := range []string{"2026.08.27-test", "2026-08-27T01:02:03Z", "automated release batch"} {
		if !strings.Contains(releaseNotes, expected) {
			t.Errorf("release notes missing %q\n%s", expected, releaseNotes)
		}
	}
}

func TestReleaseWorkflowUsesOneGeneratedMetadataSet(t *testing.T) {
	projectRoot := releaseContractProjectRoot(t)
	makefile := readReleaseContractFile(t, filepath.Join(projectRoot, "Makefile"))
	deployment := readReleaseContractFile(t, filepath.Join(projectRoot, "deploy", "deploy.sh"))
	rollback := readReleaseContractFile(t, filepath.Join(projectRoot, "deploy", "rollback.sh"))
	productionEnv := readReleaseContractFile(t, filepath.Join(projectRoot, "deploy", ".env.production.example"))
	dockerfile := readReleaseContractFile(t, filepath.Join(projectRoot, "Dockerfile"))

	for name, expected := range map[string]string{
		"metadata target":           "release-metadata:",
		"release target":            "release: verify image-bundle compose-bundle",
		"shared release env":        `. "$(RELEASE_ENV)"`,
		"compressed image bundle":   "well-ambient-images-$$WELL_AMBIENT_VERSION.tar.gz",
		"default deploy invocation": "./deploy/deploy.sh",
	} {
		if !strings.Contains(makefile, expected) {
			t.Errorf("Makefile missing %s contract %q", name, expected)
		}
	}
	if strings.Contains(makefile, "./deploy/deploy.sh $(VERSION)") {
		t.Error("Makefile still requires a manually supplied deploy version")
	}
	if !strings.Contains(deployment, "deploy/generated/release.env") || !strings.Contains(deployment, "release-notes.txt") {
		t.Error("deployment does not consume and retain generated release metadata")
	}
	if !strings.Contains(rollback, "previous-release.env") || !strings.Contains(rollback, "previous-release-notes.txt") {
		t.Error("rollback does not restore the release metadata that belongs to the previous image version")
	}
	if strings.Contains(productionEnv, "WELL_AMBIENT_VERSION=") {
		t.Error("production environment example still asks operators to enter a version")
	}
	if strings.Count(dockerfile, "org.opencontainers.image.version") < 2 ||
		strings.Count(dockerfile, "org.opencontainers.image.created") < 2 {
		t.Error("both runtime images must carry the same generated OCI release metadata")
	}
}

func TestComposeHealthchecksUseImageIndependentRuntimePrimitives(t *testing.T) {
	projectRoot := releaseContractProjectRoot(t)
	compose := readReleaseContractFile(t, filepath.Join(projectRoot, "compose.yaml"))
	deployScript := readReleaseContractFile(t, filepath.Join(projectRoot, "deploy", "deploy.sh"))

	for _, forbidden := range []string{
		`test: ["CMD", "curl"`,
		`test: ["CMD", "wget"`,
	} {
		if strings.Contains(compose, forbidden) {
			t.Errorf("compose healthcheck still depends on an optional runtime tool: %s", forbidden)
		}
	}

	for _, expected := range []string{
		`test: ["CMD", "/usr/local/bin/well-ambient", "--healthcheck-url", "http://127.0.0.1:8080/ready"]`,
		`test: ["CMD-SHELL", "test -s /usr/share/nginx/html/index.html"]`,
	} {
		if !strings.Contains(compose, expected) {
			t.Errorf("compose missing image-independent healthcheck %q", expected)
		}
	}

	for _, expected := range []string{
		"print_compose_diagnostics",
		`docker inspect --format`,
		`logs --no-color --tail 120 server web`,
	} {
		if !strings.Contains(deployScript, expected) {
			t.Errorf("deploy failure path missing diagnostic %q", expected)
		}
	}
}

func releaseContractProjectRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current release contract test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}

func readReleaseContractFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
