package server

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

// Opt-in local rendering probe. Opens the supplied database read-only and never
// invokes SMTP or the Confluence client. No report content or credentials logged.
func TestConfluenceLocalReportDiagnostic(t *testing.T) {
	path := os.Getenv("WELL_CONFLUENCE_DIAGNOSTIC_DB")
	if path == "" {
		t.Skip("requires explicit read-only database path")
	}
	conn, err := gorm.Open(sqlite.Open("file:"+path+"?mode=ro"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := conn.DB()
	defer sqlDB.Close()
	original := db.DB
	db.DB = conn
	defer func() { db.DB = original }()
	var raw string
	if err = conn.Raw("SELECT config_json FROM runtime_configs LIMIT 1").Scan(&raw).Error; err != nil {
		t.Fatal("cannot load runtime configuration")
	}
	var cfg config.Config
	if err = json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal("cannot decode runtime configuration")
	}
	s := &Server{config: &cfg}
	dates := os.Getenv("WELL_CONFLUENCE_DIAGNOSTIC_DATES")
	if dates == "" {
		t.Fatal("explicit comma-separated report dates required")
	}
	for _, date := range strings.Split(dates, ",") {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		report, err := s.buildEmailReport(ctx, cfg, cfg.DailyJiraEmail, date, time.Now())
		cancel()
		if err != nil {
			t.Fatalf("%s render: %v", date, err)
		}
		body, attachments, err := confluenceReportStorage(&report)
		if err != nil {
			t.Fatalf("%s storage: %v", date, err)
		}
		decoder := xml.NewDecoder(strings.NewReader("<root>" + body + "</root>"))
		for {
			_, err = decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("%s XML: %v", date, err)
			}
		}
		t.Logf("date=%s storage_bytes=%d attachments=%d XML=valid", date, len(body), len(attachments))
	}
}
