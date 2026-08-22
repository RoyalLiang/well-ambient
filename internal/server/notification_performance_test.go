package server

import (
	"bytes"
	stdlog "log"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/db"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestDelayAlertReadIsProjectedAliasAwareAndSideEffectFree(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}
	createdAt := time.Now().Add(-10 * 24 * time.Hour)
	for _, task := range []db.TaskTelemetry{
		{TaskID: "ACTIVE-1", Title: "active", Assignee: "Alice", Status: "backlog", TaskCreatedAt: createdAt},
		{TaskID: "DONE-1", Title: "done alias", Assignee: "Bob", Status: "Done", TaskCreatedAt: createdAt},
		{TaskID: "RESOLVED-1", Title: "resolved alias", Assignee: "Carol", Status: "resolved", TaskCreatedAt: createdAt},
	} {
		if err := db.DB.Create(&task).Error; err != nil {
			t.Fatalf("seed %s: %v", task.TaskID, err)
		}
	}

	var output bytes.Buffer
	originalOutput := stdlog.Writer()
	stdlog.SetOutput(&output)
	t.Cleanup(func() { stdlog.SetOutput(originalOutput) })
	originalLogger := db.DB.Config.Logger
	db.DB = db.DB.Session(&gorm.Session{
		Logger: gormlogger.New(stdlog.New(&output, "", 0), gormlogger.Config{LogLevel: gormlogger.Info}),
	})
	t.Cleanup(func() { db.DB = db.DB.Session(&gorm.Session{Logger: originalLogger}) })

	alerts := (&Server{}).computeDelayAlerts()
	if len(alerts) != 1 || alerts[0].TaskID != "ACTIVE-1" {
		t.Fatalf("delay alerts = %+v, want only ACTIVE-1", alerts)
	}
	logs := output.String()
	if strings.Contains(logs, "Email extension hook") {
		t.Fatalf("read path triggered notification side effect:\n%s", logs)
	}
	if strings.Contains(logs, "SELECT *") {
		t.Fatalf("delay alert query loaded full task rows:\n%s", logs)
	}
}
