package db

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestEmailTemplateCandidateIncludedInRequiredSchema(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := conn.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	found := false
	for _, model := range RequiredSchemaModels() {
		statement := &gorm.Statement{DB: conn}
		if err := statement.Parse(model); err != nil {
			t.Fatal(err)
		}
		if statement.Schema.Table == "email_template_candidates" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("email template gallery omitted from required schema/migration")
	}
	if err := conn.AutoMigrate(&EmailTemplateCandidate{}); err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"id", "name", "template_json", "created_at"} {
		if !conn.Migrator().HasColumn(&EmailTemplateCandidate{}, column) {
			t.Errorf("missing gallery column %s", column)
		}
	}
}
