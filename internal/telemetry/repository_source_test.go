package telemetry

import (
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"testing"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestRepositoryIdentitySeparatesSameSHAAndSurvivesRename(t *testing.T) {
	previous := db.DB
	t.Cleanup(func() { db.DB = previous })
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	db.DB = conn
	if err := conn.AutoMigrate(&db.GitCommitLog{}, &db.TaskTelemetry{}, &db.Notification{}); err != nil {
		t.Fatal(err)
	}
	global := config.GitLabConfig{BaseURL: "https://devgit.example.test", Repos: []config.RepoMapping{{Name: "service", Path: "core/service", ProjectID: "10"}, {Name: "service", Path: "project-code/a/service", ProjectID: "fde:10", InstanceID: "fde", RemoteProjectID: "10"}, {Name: "service", Path: "project-code/b/service", ProjectID: "fde:11", InstanceID: "fde", RemoteProjectID: "11"}}, Connections: []config.GitLabConnection{{ID: "fde", Enabled: true, BaseURL: "https://fde.example.test", RootGroup: "project-code"}}}
	fde, _ := global.Connection("fde")
	push := func(cfg config.GitLabConfig, id int, path string) {
		t.Helper()
		body := []byte(fmt.Sprintf(`{"ref":"refs/heads/main","project":{"id":%d,"name":"service","web_url":"%s/%s"},"commits":[{"id":"same-sha","message":"plain update","modified":["main.go"],"author":{"name":"dev"}}]}`, id, cfg.BaseURL, path))
		if err := ProcessWebhookEvent(&config.Config{GitLab: cfg}, "Push Hook", body); err != nil {
			t.Fatal(err)
		}
	}
	push(fde, 10, "project-code/a/service")
	push(fde, 11, "project-code/b/service")
	push(fde, 10, "project-code/renamed/service")
	legacy, _ := global.Connection("")
	push(legacy, 10, "core/service")
	var logs []db.GitCommitLog
	if err := conn.Find(&logs).Error; err != nil {
		t.Fatal(err)
	}
	if len(logs) != 3 {
		t.Fatalf("same SHA across repositories / renamed replay: %d records", len(logs))
	}
	keys := map[string]bool{}
	for _, entry := range logs {
		keys[entry.RepositoryKey] = true
		if entry.RepositorySourceJSON == "" {
			t.Fatal("source snapshot missing")
		}
	}
	if len(keys) != 3 {
		t.Fatal("cross-instance source identities collided")
	}
}
