package main

import (
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type backupFixture struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

func TestCreateConsistentBackupCapturesCommittedWALAndIsReadOnlyVerifiable(t *testing.T) {
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "source.db")
	backupPath := filepath.Join(directory, "backup.db")
	source, err := gorm.Open(sqlite.Open(sourcePath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	if err := source.Exec("PRAGMA journal_mode=WAL").Error; err != nil {
		t.Fatalf("enable WAL: %v", err)
	}
	if err := source.Exec("PRAGMA wal_autocheckpoint=0").Error; err != nil {
		t.Fatalf("disable automatic checkpoint: %v", err)
	}
	if err := source.AutoMigrate(&backupFixture{}); err != nil {
		t.Fatalf("migrate fixture: %v", err)
	}
	if err := source.Create(&[]backupFixture{{Name: "first"}, {Name: "second"}}).Error; err != nil {
		t.Fatalf("write WAL fixture: %v", err)
	}
	if info, err := os.Stat(sourcePath + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("fixture did not retain committed WAL content: info=%v err=%v", info, err)
	}

	if err := createConsistentBackup(sourcePath, backupPath); err != nil {
		t.Fatalf("create consistent backup: %v", err)
	}
	backup, err := openReadOnly(backupPath)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	var count int64
	if err := backup.Model(&backupFixture{}).Count(&count).Error; err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if count != 2 {
		t.Fatalf("backup rows = %d, want 2", count)
	}
	if err := backup.Exec("CREATE TABLE forbidden_write (id INTEGER)").Error; err == nil {
		t.Fatal("read-only backup connection accepted a write")
	}
	if err := createConsistentBackup(sourcePath, backupPath); err == nil {
		t.Fatal("existing backup path was overwritten")
	}
}
