package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"well-ambient/internal/config"
	"well-ambient/internal/db"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBootstrapVersionedConfigInheritsNewTopLevelSectionFromFile(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := gormDB.AutoMigrate(&db.ConfigVersion{}, &db.RuntimeConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	previousDB := db.DB
	db.DB = gormDB
	defer func() { db.DB = previousDB }()

	legacyJSON, err := json.Marshal(map[string]any{
		"server": map[string]any{"host": "127.0.0.1", "port": 9100},
		"jira":   map[string]any{"enabled": true, "base_url": "https://jira.example.com"},
	})
	if err != nil {
		t.Fatalf("marshal legacy config: %v", err)
	}
	if err := gormDB.Create(&db.ConfigVersion{
		Version: 1, Source: "legacy", ConfigJSON: string(legacyJSON),
	}).Error; err != nil {
		t.Fatalf("seed legacy config: %v", err)
	}

	fileConfig := config.Config{
		Database: config.DatabaseConfig{Driver: "postgres", DSNEnv: "WELL_AMBIENT_DATABASE_DSN"},
		Server:   config.ServerConfig{Host: "0.0.0.0", Port: 8080},
		PerformanceBrain: config.PerformanceBrainConfig{
			Enabled: true, IntervalMinutes: 37, RetentionDays: 91,
		},
	}
	if err := BootstrapVersionedConfig(&fileConfig); err != nil {
		t.Fatalf("bootstrap config: %v", err)
	}

	if fileConfig.Server.Port != 9100 || fileConfig.Jira.BaseURL != "https://jira.example.com" {
		t.Fatalf("stored configuration did not remain authoritative: %#v", fileConfig)
	}
	if !fileConfig.PerformanceBrain.Enabled || fileConfig.PerformanceBrain.IntervalMinutes != 37 || fileConfig.PerformanceBrain.RetentionDays != 91 {
		t.Fatalf("new file-only performance section was discarded: %#v", fileConfig.PerformanceBrain)
	}
	if fileConfig.Database.Driver != "postgres" || fileConfig.Database.DSNEnv != "WELL_AMBIENT_DATABASE_DSN" {
		t.Fatalf("bootstrap-only database config was replaced by runtime archive: %#v", fileConfig.Database)
	}
	var synchronized db.RuntimeConfig
	if err := gormDB.First(&synchronized, runtimeConfigSingletonID).Error; err != nil {
		t.Fatalf("legacy archive was not synchronized into current config: %v", err)
	}
	if synchronized.Version != 1 || strings.Contains(synchronized.ConfigJSON, `"database"`) {
		t.Fatalf("unexpected synchronized runtime config: %#v", synchronized)
	}
}

func TestBootstrapVersionedConfigLoadsCurrentDatabaseConfig(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := gormDB.AutoMigrate(&db.ConfigVersion{}, &db.RuntimeConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	previousDB := db.DB
	db.DB = gormDB
	defer func() { db.DB = previousDB }()

	stored := config.Config{
		Server: config.ServerConfig{Host: "127.0.0.1", Port: 9200},
		Jira: config.JiraConfig{
			Enabled: true, BaseURL: "https://jira.database.example.com", APIToken: "database-secret",
		},
	}
	storedJSON, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf("marshal runtime config: %v", err)
	}
	if err := gormDB.Create(&db.RuntimeConfig{ID: 1, Version: 7, ConfigJSON: string(storedJSON)}).Error; err != nil {
		t.Fatalf("seed runtime config: %v", err)
	}

	fileConfig := config.Config{
		Database: config.DatabaseConfig{Driver: "postgres", DSNEnv: "WELL_AMBIENT_DATABASE_DSN"},
		Server:   config.ServerConfig{Host: "0.0.0.0", Port: 8080},
		Jira:     config.JiraConfig{APIToken: "stale-file-secret"},
	}
	if err := BootstrapVersionedConfig(&fileConfig); err != nil {
		t.Fatalf("bootstrap config: %v", err)
	}

	if fileConfig.Server.Port != 9200 || fileConfig.Jira.BaseURL != stored.Jira.BaseURL || fileConfig.Jira.APIToken != "database-secret" {
		t.Fatalf("runtime database config was not authoritative: %#v", fileConfig)
	}
	if fileConfig.Database.Driver != "postgres" || fileConfig.Database.DSNEnv != "WELL_AMBIENT_DATABASE_DSN" {
		t.Fatalf("bootstrap database config was not retained: %#v", fileConfig.Database)
	}
}

func TestBootstrapVersionedConfigKeepsLegacyMigrationTargetSafe(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "target.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Migrate(gormDB); err != nil {
		t.Fatalf("initialize target: %v", err)
	}
	previousDB := db.DB
	db.DB = gormDB
	defer func() { db.DB = previousDB }()

	fileConfig := config.Config{
		Database: config.DatabaseConfig{Driver: "postgres", DSNEnv: "WELL_AMBIENT_DATABASE_DSN"},
		Server:   config.ServerConfig{Host: "0.0.0.0", Port: 8080},
	}
	if err := BootstrapVersionedConfig(&fileConfig); err != nil {
		t.Fatalf("bootstrap config: %v", err)
	}

	safe, err := db.LegacyMigrationTargetIsSafe(gormDB)
	if err != nil {
		t.Fatalf("inspect migration target: %v", err)
	}
	if !safe {
		t.Fatal("fresh target became unsafe after normal service configuration bootstrap")
	}
}

func TestRestoreVersionedConfigKeepsArchivedTopLevelSectionAuthoritative(t *testing.T) {
	archivedJSON, err := json.Marshal(config.Config{
		PerformanceBrain: config.PerformanceBrainConfig{
			Enabled: false, IntervalMinutes: 15, RetentionDays: 30,
		},
	})
	if err != nil {
		t.Fatalf("marshal archived config: %v", err)
	}

	restored, err := restoreVersionedConfig(config.Config{
		PerformanceBrain: config.PerformanceBrainConfig{
			Enabled: true, IntervalMinutes: 60, RetentionDays: 90,
		},
	}, string(archivedJSON))
	if err != nil {
		t.Fatalf("restore config: %v", err)
	}

	if restored.PerformanceBrain.Enabled || restored.PerformanceBrain.IntervalMinutes != 15 || restored.PerformanceBrain.RetentionDays != 30 {
		t.Fatalf("archived performance section lost precedence: %#v", restored.PerformanceBrain)
	}
}

func TestHandleSaveConfigRejectsInvalidJiraVersionSourceBeforeApply(t *testing.T) {
	current := &config.Config{}
	s := &Server{config: current}
	body := []byte(`{"jira":{"base_url":"https://jira.example.com","version_sources":[{"project_key":"OTHER","project_name":"Other","version_url":"https://jira.example.com/projects/PROJ/versions/13622"}]}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	s.handleSaveConfig(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if len(current.Jira.VersionSources) != 0 {
		t.Fatalf("invalid Jira version source was applied: %#v", current.Jira.VersionSources)
	}
}

func TestHandleSaveConfigRejectsInvalidJiraQueryBeforeApply(t *testing.T) {
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/search" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errorMessages":["project has no value FMS-20660"]}`))
	}))
	defer jira.Close()

	current := &config.Config{Jira: config.JiraConfig{Enabled: false, BaseURL: jira.URL}}
	s := &Server{config: current}
	newConfig := config.Config{Jira: config.JiraConfig{
		Enabled: true, BaseURL: jira.URL, CustomJQL: `project = "FMS-20660"`,
	}}
	body, err := json.Marshal(newConfig)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	s.handleSaveConfig(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if current.Jira.Enabled || current.Jira.CustomJQL != "" {
		t.Fatalf("invalid Jira query was applied: %#v", current.Jira)
	}
	if !strings.Contains(rr.Body.String(), "FMS-20660") || !strings.Contains(rr.Body.String(), "配置未保存") {
		t.Fatalf("response was not actionable: %s", rr.Body.String())
	}
}

func TestHandleSaveConfigPersistsDatabaseWithoutRewritingBootstrapFile(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := gormDB.AutoMigrate(&db.ConfigVersion{}, &db.RuntimeConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	previousDB := db.DB
	db.DB = gormDB
	defer func() { db.DB = previousDB }()

	bootstrapPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(bootstrapPath, []byte("bootstrap-only\n"), 0o600); err != nil {
		t.Fatalf("write bootstrap fixture: %v", err)
	}
	current := &config.Config{
		Database: config.DatabaseConfig{Driver: "postgres", DSNEnv: "WELL_AMBIENT_DATABASE_DSN"},
	}
	s := &Server{config: current, configPath: bootstrapPath}
	body, err := json.Marshal(config.Config{Server: config.ServerConfig{Host: "127.0.0.1", Port: 8123}})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	s.handleSaveConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	contents, err := os.ReadFile(bootstrapPath)
	if err != nil {
		t.Fatalf("read bootstrap fixture: %v", err)
	}
	if string(contents) != "bootstrap-only\n" {
		t.Fatalf("settings save rewrote the bootstrap file: %q", contents)
	}
	var runtime db.RuntimeConfig
	if err := gormDB.First(&runtime, runtimeConfigSingletonID).Error; err != nil {
		t.Fatalf("load runtime config: %v", err)
	}
	if runtime.Version != 1 || !strings.Contains(runtime.ConfigJSON, `"port":8123`) {
		t.Fatalf("settings were not persisted as current database config: %#v", runtime)
	}
	if current.Server.Port != 8123 {
		t.Fatalf("persisted settings were not applied in memory: %#v", current.Server)
	}
}

func TestRecordConfigVersionRollsBackArchiveWhenCurrentConfigCannotPersist(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := gormDB.AutoMigrate(&db.ConfigVersion{}); err != nil {
		t.Fatalf("migrate archive only: %v", err)
	}
	previousDB := db.DB
	db.DB = gormDB
	defer func() { db.DB = previousDB }()

	s := &Server{config: &config.Config{}}
	_, err = s.recordConfigVersion(config.Config{}, config.Config{Server: config.ServerConfig{Port: 8123}}, httptest.NewRequest(http.MethodPost, "/api/config", nil), "manual-save", 0)
	if err == nil {
		t.Fatal("expected current config persistence to fail without its migrated table")
	}
	var count int64
	if err := gormDB.Model(&db.ConfigVersion{}).Count(&count).Error; err != nil {
		t.Fatalf("count versions: %v", err)
	}
	if count != 0 {
		t.Fatalf("version archive escaped the failed transaction: %d", count)
	}
}

func TestRedactConfigForArchiveHashesSecrets(t *testing.T) {
	cfg := config.Config{
		GitLab: config.GitLabConfig{
			BaseURL:  "https://gitlab.example.com",
			Secret:   "secret-token",
			APIToken: "api-token",
		},
		Feishu: config.FeishuConfig{
			AppID:     "app-id",
			AppSecret: "app-secret",
		},
	}

	redacted := redactConfigForArchive(cfg)
	gitlab := redacted["gitlab"].(map[string]interface{})
	feishu := redacted["feishu"].(map[string]interface{})

	if gitlab["secret_token"] == "secret-token" || gitlab["api_token"] == "api-token" || feishu["app_secret"] == "app-secret" {
		t.Fatalf("expected secrets to be redacted: %#v", redacted)
	}
	if !strings.HasPrefix(gitlab["secret_token"].(string), "configured:") {
		t.Fatalf("expected redacted hash marker, got %q", gitlab["secret_token"])
	}
	if gitlab["base_url"] != "https://gitlab.example.com" {
		t.Fatalf("expected non-secret values to remain readable")
	}
}

func TestRecordConfigVersionStoresDiffAndSections(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := gormDB.AutoMigrate(&db.ConfigVersion{}, &db.RuntimeConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	previousDB := db.DB
	db.DB = gormDB
	defer func() { db.DB = previousDB }()

	s := &Server{config: &config.Config{}}
	before := config.Config{}
	after := config.Config{
		Jira: config.JiraConfig{
			Enabled:  true,
			BaseURL:  "https://jira.example.com",
			APIToken: "plain-test-secret",
		},
	}
	req := httptest.NewRequest("POST", "/api/config", nil)
	req.Header.Set("x-authenticated-user-id", "admin@example.com")
	req.Header.Set("x-authenticated-user-name", "Admin")

	version, err := s.recordConfigVersion(before, after, req, "manual-save", 0)
	if err != nil {
		t.Fatalf("record version: %v", err)
	}

	dto := configVersionDTO(version)
	if dto.Version != 1 {
		t.Fatalf("expected version 1, got %d", dto.Version)
	}
	if len(dto.ChangedSections) != 1 || dto.ChangedSections[0] != "jira" {
		t.Fatalf("unexpected changed sections: %#v", dto.ChangedSections)
	}
	if len(dto.Diff) == 0 {
		t.Fatalf("expected diff entries")
	}
	if strings.Contains(version.ConfigJSON, "plain-test-secret") || !strings.Contains(version.ConfigJSON, configuredSecretPlaceholder) {
		t.Fatalf("restorable archive must contain only a configured marker: %s", version.ConfigJSON)
	}
	var runtime db.RuntimeConfig
	if err := gormDB.First(&runtime, 1).Error; err != nil {
		t.Fatalf("load runtime config: %v", err)
	}
	if runtime.Version != version.Version || !strings.Contains(runtime.ConfigJSON, "plain-test-secret") {
		t.Fatalf("current database config did not preserve the applied secret: %#v", runtime)
	}
	if strings.Contains(runtime.ConfigJSON, `"database"`) {
		t.Fatalf("bootstrap-only database settings leaked into runtime config: %s", runtime.ConfigJSON)
	}
}

func TestSanitizeConfigVersionSecretsPreservesUnknownFields(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := gormDB.AutoMigrate(&db.ConfigVersion{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	legacy := db.ConfigVersion{
		Version:    1,
		ConfigJSON: `{"jira":{"base_url":"https://jira.example.com","api_token":"legacy-test-secret"},"future":{"unknown":true}}`,
	}
	if err := gormDB.Create(&legacy).Error; err != nil {
		t.Fatalf("seed legacy archive: %v", err)
	}

	if err := sanitizeConfigVersionSecrets(gormDB); err != nil {
		t.Fatalf("sanitize archives: %v", err)
	}
	var stored db.ConfigVersion
	if err := gormDB.First(&stored, legacy.ID).Error; err != nil {
		t.Fatalf("reload archive: %v", err)
	}
	if strings.Contains(stored.ConfigJSON, "legacy-test-secret") || !strings.Contains(stored.ConfigJSON, configuredSecretPlaceholder) {
		t.Fatalf("legacy secret was not sanitized: %s", stored.ConfigJSON)
	}
	if !strings.Contains(stored.ConfigJSON, `"future":{"unknown":true}`) {
		t.Fatalf("unknown archive fields were lost: %s", stored.ConfigJSON)
	}
}

func TestRestoreVersionedConfigInheritsCurrentSecrets(t *testing.T) {
	current := config.Config{Jira: config.JiraConfig{APIToken: "current-test-secret"}}
	archived := config.Config{Jira: config.JiraConfig{
		Enabled:  true,
		BaseURL:  "https://jira.example.com",
		APIToken: configuredSecretPlaceholder,
	}}
	encoded, err := json.Marshal(archived)
	if err != nil {
		t.Fatalf("marshal archive: %v", err)
	}
	restored, err := restoreVersionedConfig(current, string(encoded))
	if err != nil {
		t.Fatalf("restore archive: %v", err)
	}
	mergeConfiguredSecrets(&restored, current)
	if restored.Jira.APIToken != current.Jira.APIToken || restored.Jira.BaseURL != archived.Jira.BaseURL {
		t.Fatalf("restore did not combine historical non-secrets with current secret: %#v", restored.Jira)
	}
}

func TestSaveConfigRejectsInvalidReasoningEffort(t *testing.T) {
	s := &Server{config: &config.Config{AI: config.AIConfig{ReasoningEffort: "low"}}}
	rr := httptest.NewRecorder()
	s.handleSaveConfig(rr, httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(`{"ai":{"reasoning_effort":"invalid"}}`)))
	if rr.Code != http.StatusBadRequest || s.config.AI.ReasoningEffort != "low" {
		t.Fatalf("invalid config applied: status %d", rr.Code)
	}
}
