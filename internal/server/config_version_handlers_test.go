package server

import (
	"net/http/httptest"
	"strings"
	"testing"
	"well-ambient/internal/config"
	"well-ambient/internal/db"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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
	if err := gormDB.AutoMigrate(&db.ConfigVersion{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	previousDB := db.DB
	db.DB = gormDB
	defer func() { db.DB = previousDB }()

	s := &Server{config: &config.Config{}}
	before := config.Config{}
	after := config.Config{
		Jira: config.JiraConfig{
			Enabled: true,
			BaseURL: "https://jira.example.com",
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
}
