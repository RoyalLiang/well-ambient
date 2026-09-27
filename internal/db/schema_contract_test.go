package db

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRequiredSchemaModelsCoverCoreIdentityFactsAndDataAssets(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open schema parser: %v", err)
	}
	tables := make(map[string]struct{})
	for _, model := range RequiredSchemaModels() {
		statement := &gorm.Statement{DB: conn}
		if err := statement.Parse(model); err != nil {
			t.Fatalf("parse required model %T: %v", model, err)
		}
		if _, exists := tables[statement.Schema.Table]; exists {
			t.Fatalf("duplicate required schema table %q", statement.Schema.Table)
		}
		tables[statement.Schema.Table] = struct{}{}
	}
	for _, required := range []string{
		"users", "task_telemetries", "config_versions", "runtime_configs", "solution_assets",
		"performance_score_snapshots", "data_asset_events", "data_asset_snapshot_payloads",
		"integration_sources", "integration_credentials", "integration_quota_windows",
		"integration_quota_leases", "integration_policy_versions",
		"jira_execution_bindings", "decision_plans", "open_operations",
		"open_operation_actions", "open_outboxes", "capability_invocations", "open_query_snapshots",
	} {
		if _, ok := tables[required]; !ok {
			t.Fatalf("required schema table %q is not covered", required)
		}
	}
	if len(tables) < 60 {
		t.Fatalf("required schema coverage unexpectedly small: %d tables", len(tables))
	}
}
