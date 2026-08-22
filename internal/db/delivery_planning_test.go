package db

import (
	"fmt"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDeliveryPlanningSchemaAutoMigrateAndReleaseIdentity(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:delivery-schema-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := conn.AutoMigrate(
		&TaskTelemetry{},
		&ReleaseVersion{},
		&WorkItemReleaseLink{},
		&WorkItemEvent{},
		&WorkItemSyncOperation{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	for _, column := range []string{"project_key", "source", "external_key", "parent_work_item_id", "revision", "planning_state", "source_updated_at"} {
		if !conn.Migrator().HasColumn(&TaskTelemetry{}, column) {
			t.Fatalf("expected task telemetry column %q", column)
		}
	}
	for _, model := range []any{&ReleaseVersion{}, &WorkItemReleaseLink{}, &WorkItemEvent{}, &WorkItemSyncOperation{}} {
		if !conn.Migrator().HasTable(model) {
			t.Fatalf("expected migrated table for %T", model)
		}
	}
	for _, index := range []struct {
		model any
		name  string
	}{
		{&TaskTelemetry{}, "idx_task_jira_project_type"},
		{&ReleaseVersion{}, "idx_release_catalog"},
		{&WorkItemReleaseLink{}, "idx_work_item_active_relation_primary"},
		{&WorkItemReleaseLink{}, "idx_release_active_relation_primary"},
	} {
		if !conn.Migrator().HasIndex(index.model, index.name) {
			t.Fatalf("expected performance index %q", index.name)
		}
	}

	release := ReleaseVersion{
		ProjectKey: "HIT",
		Source:     "jira",
		ExternalID: "13622",
		Name:       "1.1",
	}
	if err := conn.Create(&release).Error; err != nil {
		t.Fatalf("create release: %v", err)
	}
	duplicate := release
	duplicate.ID = 0
	if err := conn.Create(&duplicate).Error; err == nil {
		t.Fatal("expected duplicate project/source/external release identity to fail")
	}

	otherProject := release
	otherProject.ID = 0
	otherProject.ProjectKey = "FEL2WD"
	if err := conn.Create(&otherProject).Error; err != nil {
		t.Fatalf("same external id in another project must remain valid: %v", err)
	}
}
