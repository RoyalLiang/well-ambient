package config

import (
	"os"
	"path/filepath"
	"testing"
)

func boolPointer(value bool) *bool { return &value }

func TestDatabaseConfigResolveDefaultsSQLiteForExistingConfigs(t *testing.T) {
	resolved, err := (DatabaseConfig{}).Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Driver != "sqlite" || resolved.DSN != "well-ambient.db" || !resolved.AutoMigrate {
		t.Fatalf("unexpected SQLite defaults: %#v", resolved)
	}
	if resolved.MaxOpenConnections != 4 || resolved.MaxIdleConnections != 2 {
		t.Fatalf("unexpected SQLite pool defaults: %#v", resolved)
	}
}

func TestDatabaseConfigResolvePostgresFromEnvironment(t *testing.T) {
	const envName = "WELL_AMBIENT_TEST_DATABASE_DSN"
	t.Setenv(envName, "postgres://ambient:secret@postgres/ambient?sslmode=disable")
	resolved, err := (DatabaseConfig{Driver: "postgresql", DSNEnv: envName}).Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Driver != "postgres" || resolved.DSN != os.Getenv(envName) || resolved.AutoMigrate {
		t.Fatalf("unexpected PostgreSQL defaults: %#v", resolved)
	}
	if resolved.MaxOpenConnections != 20 || resolved.MaxIdleConnections != 10 {
		t.Fatalf("unexpected PostgreSQL pool defaults: %#v", resolved)
	}
}

func TestDatabaseConfigResolveRejectsMissingPostgresDSNAndInvalidPool(t *testing.T) {
	if _, err := (DatabaseConfig{Driver: "postgres"}).Resolve(); err == nil {
		t.Fatal("missing PostgreSQL DSN was accepted")
	}
	if _, err := (DatabaseConfig{Driver: "postgres", DSN: "postgres://example", MaxOpenConnections: 2, MaxIdleConnections: 3, AutoMigrate: boolPointer(false)}).Resolve(); err == nil {
		t.Fatal("invalid PostgreSQL pool was accepted")
	}
}

func TestDatabaseConfigRequiresExplicitSetupDriver(t *testing.T) {
	if !(DatabaseConfig{Driver: " setup "}).RequiresSetup() {
		t.Fatal("explicit setup driver was not detected")
	}
	if (DatabaseConfig{Driver: "postgres"}).RequiresSetup() {
		t.Fatal("configured PostgreSQL was treated as setup")
	}
}

func TestSaveConfigIsOwnerOnlyAndReplacesCompleteYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("stale: true\n"), 0644); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	cfg := &Config{
		Database: DatabaseConfig{Driver: "postgres", DSN: "postgres://ambient:secret@postgres/well_ambient"},
		Server:   ServerConfig{Host: "127.0.0.1", Port: 8080},
	}
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("config mode = %04o, want 0600", info.Mode().Perm())
	}
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if loaded.Database.Driver != "postgres" || loaded.Database.DSN != cfg.Database.DSN || loaded.Server.Port != 8080 {
		t.Fatalf("unexpected saved config: %#v", loaded)
	}
	if matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".config.yaml.tmp-*")); err != nil || len(matches) != 0 {
		t.Fatalf("temporary files remain: %v, %v", matches, err)
	}
}

func TestSaveConfigPersistsSetupOnlyLegacyMigrationDecision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := &Config{
		Database: DatabaseConfig{
			Driver:                  "setup",
			LegacySQLitePath:        "/var/lib/well-ambient/legacy/well-ambient.db",
			LegacyMigrationDecision: "migrate",
		},
		Server: ServerConfig{Host: "127.0.0.1", Port: 8080},
	}
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if loaded.Database.LegacySQLitePath != cfg.Database.LegacySQLitePath || loaded.Database.LegacyMigrationDecision != "migrate" {
		t.Fatalf("legacy migration setup fields did not round-trip: %#v", loaded.Database)
	}
}

func TestGetRealAPIURL(t *testing.T) {
	tests := []struct {
		name         string
		baseURL      string
		endpointType string
		expected     string
	}{
		{
			name:         "Legacy completions migrates to responses",
			baseURL:      "https://ai-pixel.online",
			endpointType: "completions",
			expected:     "https://ai-pixel.online/v1/responses",
		},
		{
			name:         "Pure domain responses",
			baseURL:      "https://ai-pixel.online",
			endpointType: "responses",
			expected:     "https://ai-pixel.online/v1/responses",
		},
		{
			name:         "Pure domain with trailing slash",
			baseURL:      "https://ai-pixel.online/",
			endpointType: "completions",
			expected:     "https://ai-pixel.online/v1/responses",
		},
		{
			name:         "Explicit completions path migrates to responses",
			baseURL:      "https://ai-pixel.online/v1/chat/completions",
			endpointType: "completions",
			expected:     "https://ai-pixel.online/v1/responses",
		},
		{
			name:         "Already complete responses path",
			baseURL:      "https://ai-pixel.online/v1/responses",
			endpointType: "responses",
			expected:     "https://ai-pixel.online/v1/responses",
		},
		{
			name:         "Complete completions path with extra case difference",
			baseURL:      "https://ai-pixel.online/V1/chat/completions",
			endpointType: "responses",
			expected:     "https://ai-pixel.online/v1/responses",
		},
		{
			name:         "Version root does not duplicate v1",
			baseURL:      "https://ai-pixel.online/v1",
			endpointType: "responses",
			expected:     "https://ai-pixel.online/v1/responses",
		},
		{
			name:         "Claude messages endpoint",
			baseURL:      "https://api.anthropic.com",
			endpointType: "messages",
			expected:     "https://api.anthropic.com/v1/messages",
		},
		{
			name:         "Empty URL",
			baseURL:      "",
			endpointType: "completions",
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AIConfig{
				BaseURL:      tt.baseURL,
				EndpointType: tt.endpointType,
			}
			result := cfg.GetRealAPIURL()
			if result != tt.expected {
				t.Errorf("GetRealAPIURL() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestAIConfigProtocol(t *testing.T) {
	tests := []struct {
		name     string
		config   AIConfig
		expected string
	}{
		{name: "responses by default", config: AIConfig{}, expected: "responses"},
		{name: "legacy completions", config: AIConfig{EndpointType: "completions"}, expected: "responses"},
		{name: "explicit messages", config: AIConfig{EndpointType: "messages"}, expected: "messages"},
		{name: "native anthropic", config: AIConfig{Provider: "anthropic", BaseURL: "https://api.anthropic.com"}, expected: "messages"},
		{name: "anthropic through sub2api", config: AIConfig{Provider: "anthropic", BaseURL: "https://sub2api.example"}, expected: "responses"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.Protocol(); got != tt.expected {
				t.Fatalf("Protocol() = %q, expected %q", got, tt.expected)
			}
		})
	}
}
