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
		"users", "task_telemetries", "config_versions", "solution_assets",
		"performance_score_snapshots", "data_asset_events", "data_asset_snapshot_payloads",
	} {
		if _, ok := tables[required]; !ok {
			t.Fatalf("required schema table %q is not covered", required)
		}
	}
	if len(tables) < 60 {
		t.Fatalf("required schema coverage unexpectedly small: %d tables", len(tables))
	}
}
