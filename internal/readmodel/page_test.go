package readmodel

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type pageTestRow struct {
	ID    uint `gorm:"primaryKey"`
	Value string
}

func TestPageWindowRejectsCursorAfterDatasetMutation(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := conn.AutoMigrate(&pageTestRow{}); err != nil {
		t.Fatalf("migrate rows: %v", err)
	}
	if err := EnsureDatasets(conn, []Dataset{{Name: "page_test", Tables: []string{"page_test_rows"}}}); err != nil {
		t.Fatalf("ensure dataset: %v", err)
	}
	request := PageRequest{
		Dataset: "page_test", Contract: "page-test", Scope: map[string]string{"status": "active"},
		Limit: "20", DefaultLimit: 50, MaxLimit: 100,
	}
	var position struct {
		ID uint `json:"id"`
	}
	window, err := OpenPage(context.Background(), conn, request, &position)
	if err != nil {
		t.Fatalf("open first page: %v", err)
	}
	if window.Limit != 20 || window.HasCursor {
		t.Fatalf("first page window = %+v", window)
	}
	cursor, err := window.Next(struct {
		ID uint `json:"id"`
	}{ID: 10})
	if err != nil {
		t.Fatalf("encode next cursor: %v", err)
	}
	request.Cursor = cursor
	if _, err := OpenPage(context.Background(), conn, request, &position); err != nil {
		t.Fatalf("open unchanged continuation: %v", err)
	}
	if err := conn.Create(&pageTestRow{Value: "changed"}).Error; err != nil {
		t.Fatalf("insert row: %v", err)
	}
	if _, err := OpenPage(context.Background(), conn, request, &position); !errors.Is(err, ErrStaleCursor) {
		t.Fatalf("changed continuation error = %v, want ErrStaleCursor", err)
	}
	if generation, err := CurrentGeneration(context.Background(), conn, "page_test"); err != nil || generation != window.Generation+1 {
		t.Fatalf("generation after insert = %d, %v; want %d", generation, err, window.Generation+1)
	}
}

func TestEnsureDatasetsRejectsUnsafeIdentifiers(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := EnsureDatasets(conn, []Dataset{{Name: "bad-name", Tables: []string{"rows; drop table"}}}); err == nil {
		t.Fatal("expected unsafe dataset identifiers to be rejected")
	}
}
