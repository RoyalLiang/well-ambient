package telemetry

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestPushWithoutTaskIDStillCollectsCommits(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = conn.AutoMigrate(&db.GitCommitLog{}, &db.TaskTelemetry{}, &db.Notification{}); err != nil {
		t.Fatal(err)
	}
	previous := db.DB
	db.DB = conn
	t.Cleanup(func() { db.DB = previous })
	cfg := &config.Config{}
	payload := []byte(`{"ref":"refs/heads/production","user_name":"Developer","project":{"name":"task_info_service"},"commits":[{"id":"abc123","message":"fix: normalize crane point ordering","author":{"name":"Developer"},"modified":["points.go"]},{"id":"def456","message":"Merge branch prod_develop into production","author":{"name":"Developer"}}]}`)
	for i := 0; i < 2; i++ {
		if err := ProcessWebhookEvent(cfg, "Push Hook", payload); err != nil {
			t.Fatal(err)
		}
	}
	var commits []db.GitCommitLog
	if err := conn.Order("id").Find(&commits).Error; err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 unlinked commits after repeated webhook, got %d", len(commits))
	}
	for _, c := range commits {
		if c.TaskID != "" || c.Repo != "task_info_service" || c.Action != "git_push" {
			t.Fatalf("incorrect unlinked commit: %+v", c)
		}
	}
	var tasks, notifications int64
	conn.Model(&db.TaskTelemetry{}).Count(&tasks)
	conn.Model(&db.Notification{}).Count(&notifications)
	if tasks != 0 || notifications != 0 {
		t.Fatalf("unlinked commits must not fabricate task activity: tasks=%d notifications=%d", tasks, notifications)
	}
}
