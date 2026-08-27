package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"well-ambient/internal/config"
)

func TestApplyHTTPAddressOverrideWinsAfterRuntimeRestore(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Host: "0.0.0.0", Port: 8080}}
	if err := applyHTTPAddressOverride(cfg, "127.0.0.1", 18197); err != nil {
		t.Fatalf("applyHTTPAddressOverride() error = %v", err)
	}
	if cfg.Server.Host != "127.0.0.1" || cfg.Server.Port != 18197 {
		t.Fatalf("HTTP override not applied after runtime restore: %#v", cfg.Server)
	}
}

func TestApplyHTTPAddressOverrideDefaultsToStoredRuntimeAddress(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Host: "0.0.0.0", Port: 8080}}
	if err := applyHTTPAddressOverride(cfg, "", 0); err != nil {
		t.Fatalf("applyHTTPAddressOverride() error = %v", err)
	}
	if cfg.Server.Host != "0.0.0.0" || cfg.Server.Port != 8080 {
		t.Fatalf("empty HTTP override changed runtime address: %#v", cfg.Server)
	}
}

func TestLocalSetupLauncherWiresViteToIsolatedSetupServer(t *testing.T) {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))

	launcher := readLocalSetupContractFile(t, filepath.Join(projectRoot, "scripts", "dev-setup.sh"))
	makefile := readLocalSetupContractFile(t, filepath.Join(projectRoot, "Makefile"))

	for name, expected := range map[string]string{
		"isolated API port":      `api_port="${DEV_SETUP_API_PORT:-18197}"`,
		"stable web port":        `web_port="${DEV_SETUP_WEB_PORT:-5175}"`,
		"setup database driver":  `driver: setup`,
		"legacy SQLite snapshot": `legacy_sqlite_path:`,
		"generated setup token":  `env -u WELL_AMBIENT_SETUP_TOKEN`,
		"owner-only config":      `chmod 600 "$config_path"`,
		"temporary runtime":      `mktemp -d`,
		"cleanup trap":           `trap cleanup EXIT INT TERM`,
		"setup readiness":        `"setup_required"[[:space:]]*:[[:space:]]*true`,
		"stable backend host":    `--http-host "$api_host"`,
		"stable backend port":    `--http-port "$api_port"`,
		"Vite API proxy":         `VITE_API_PROXY_TARGET="http://$api_host:$api_port"`,
	} {
		if !strings.Contains(launcher, expected) {
			t.Errorf("local setup launcher missing %s contract %q", name, expected)
		}
	}
	if strings.Contains(launcher, "$project_root/config.yaml") {
		t.Fatal("local setup launcher must not overwrite the developer's config.yaml")
	}
	if !strings.Contains(makefile, "dev-setup:") || !strings.Contains(makefile, "./scripts/dev-setup.sh") {
		t.Fatal("Makefile does not expose the local setup launcher")
	}
}

func readLocalSetupContractFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
