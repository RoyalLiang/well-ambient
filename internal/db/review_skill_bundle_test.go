package db

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
	"well-ambient/internal/reviewskill"
)

func TestExplicitReviewSkillInstallationRetainsHistoryAndIsIdempotent(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "skill.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := conn.DB()
	defer sqlDB.Close()
	if err = conn.AutoMigrate(&SolutionPromptTemplate{}); err != nil {
		t.Fatal(err)
	}
	old := SolutionPromptTemplate{Purpose: "code_review", ScopeType: "global", ScopeID: "", Version: 1, Status: "active", Name: "previous", SystemPrompt: "old", ContentHash: "old", ValidationStatus: "passed", CreatedBy: "test"}
	if err = conn.Create(&old).Error; err != nil {
		t.Fatal(err)
	}
	first, err := InstallBundledCodeReviewSkill(conn)
	if err != nil {
		t.Fatal(err)
	}
	second, err := InstallBundledCodeReviewSkill(conn)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.Version != 2 || first.ContentHash != reviewskill.Hash() {
		t.Fatalf("install identities=%+v %+v", first, second)
	}
	var count int64
	conn.Model(&SolutionPromptTemplate{}).Count(&count)
	if count != 2 {
		t.Fatal(count)
	}
	conn.First(&old, old.ID)
	if old.Status != "inactive" || old.SystemPrompt != "old" {
		t.Fatal("prior version was lost")
	}
	if err := conn.Model(&SolutionPromptTemplate{}).Where("id = ?", first.ID).Update("validation_status", "failed").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := InstallBundledCodeReviewSkill(conn); err == nil {
		t.Fatal("failed active version falsely reported as installed")
	}

}
