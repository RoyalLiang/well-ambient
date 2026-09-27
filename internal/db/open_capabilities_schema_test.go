package db

import (
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestOpenCapabilityMigrationCreatesConcurrencyAndReadIndexes(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "open_capability_schema.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(conn); err != nil {
		t.Fatal(err)
	}
	var names []string
	if err := conn.Raw(
		"SELECT name FROM sqlite_master WHERE type = 'index' AND name IN ?",
		[]string{
			"idx_integration_policy_single_active",
			"idx_open_outbox_claim",
			"idx_open_operation_state_updated",
			"idx_capability_invocation_tool_created",
			"idx_open_query_snapshot_expiry",
			"idx_integration_quota_window_cleanup",
			"idx_integration_quota_lease_source_expiry",
		},
	).Scan(&names).Error; err != nil {
		t.Fatal(err)
	}
	if len(names) != 7 {
		t.Fatalf("open capability indexes = %v, want 7 required indexes", names)
	}
	now := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	first := IntegrationPolicyVersion{
		Version: 1, Status: "active", AllowedProjectsJSON: `["WA"]`,
		AllowedRepositoriesJSON: `[]`, ActionsJSON: `[]`, FieldRulesJSON: `{}`,
		ApprovalRulesJSON: `{}`, QueryLimitsJSON: `{}`, Digest: "one", CreatedAt: now,
	}
	if err := conn.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	second := first
	second.ID = 0
	second.Version = 2
	second.Digest = "two"
	if err := conn.Create(&second).Error; err == nil {
		t.Fatal("database accepted two active integration policies")
	}
}
