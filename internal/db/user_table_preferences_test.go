package db

import (
	"fmt"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUserTablePreferencePersistsColumnsPerUser(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	conn, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := conn.AutoMigrate(&UserTablePreference{}); err != nil {
		t.Fatalf("migrate preference: %v", err)
	}

	if err := SaveUserTablePreference(conn, "alice@example.com", "decision_agenda", []string{"task_id", "title", "risk", "risk"}); err != nil {
		t.Fatalf("save Alice preference: %v", err)
	}
	columns, found, err := LoadUserTablePreference(conn, "alice@example.com", "decision_agenda")
	if err != nil {
		t.Fatalf("load Alice preference: %v", err)
	}
	if !found || strings.Join(columns, ",") != "task_id,title,risk" {
		t.Fatalf("Alice columns = %#v, found=%v", columns, found)
	}

	_, found, err = LoadUserTablePreference(conn, "bob@example.com", "decision_agenda")
	if err != nil {
		t.Fatalf("load Bob preference: %v", err)
	}
	if found {
		t.Fatal("Bob must not inherit Alice's table preference")
	}
}
