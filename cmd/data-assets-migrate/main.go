package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"well-ambient/internal/db"
	"well-ambient/internal/deliveryplanning"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	var databasePath string
	var backupPath string
	var afterID uint
	var batchSize int
	var apply bool
	flag.StringVar(&databasePath, "database", "well-ambient.db", "path to the SQLite database")
	flag.StringVar(&backupPath, "backup", "", "required new backup file path when --apply is used")
	flag.UintVar(&afterID, "after-id", 0, "resume after this WorkItemEvent id")
	flag.IntVar(&batchSize, "batch-size", 200, "WorkItemEvent rows per keyset batch (max 500)")
	flag.BoolVar(&apply, "apply", false, "create additive schema and append assets; default is read-only dry-run")
	flag.Parse()

	if batchSize < 1 {
		exitOnError(fmt.Errorf("--batch-size must be positive"))
	}
	if !apply {
		conn, err := openReadOnly(databasePath)
		exitOnError(err)
		report, err := deliveryplanning.BackfillWorkItemAssets(context.Background(), conn, deliveryplanning.WorkItemAssetBackfillOptions{
			AfterID: afterID, BatchSize: batchSize,
		})
		exitOnError(err)
		writeReport(report)
		return
	}
	if backupPath == "" {
		exitOnError(fmt.Errorf("--backup is required with --apply"))
	}
	exitOnError(createConsistentBackup(databasePath, backupPath))
	conn, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	exitOnError(err)
	exitOnError(db.MigrateDataAssets(conn))
	report, err := deliveryplanning.BackfillWorkItemAssets(context.Background(), conn, deliveryplanning.WorkItemAssetBackfillOptions{
		AfterID: afterID, BatchSize: batchSize, Apply: true,
	})
	exitOnError(err)
	writeReport(report)
}

func openReadOnly(databasePath string) (*gorm.DB, error) {
	absolutePath, err := filepath.Abs(databasePath)
	if err != nil {
		return nil, err
	}
	dsn := (&url.URL{Scheme: "file", Path: absolutePath, RawQuery: "mode=ro&_query_only=1"}).String()
	return gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
}

func createConsistentBackup(sourcePath, destinationPath string) error {
	sourceAbsolute, err := filepath.Abs(sourcePath)
	if err != nil {
		return err
	}
	destinationAbsolute, err := filepath.Abs(destinationPath)
	if err != nil {
		return err
	}
	if sourceAbsolute == destinationAbsolute {
		return fmt.Errorf("backup path must differ from database path")
	}
	if _, err := os.Stat(destinationAbsolute); err == nil {
		return fmt.Errorf("backup path already exists: %s", destinationAbsolute)
	} else if !os.IsNotExist(err) {
		return err
	}
	sourceInfo, err := os.Stat(sourceAbsolute)
	if err != nil {
		return err
	}
	if !sourceInfo.Mode().IsRegular() {
		return fmt.Errorf("database path is not a regular file")
	}
	dsn := (&url.URL{Scheme: "file", Path: sourceAbsolute, RawQuery: "mode=ro"}).String()
	conn, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return err
	}
	sqlConn, err := conn.DB()
	if err != nil {
		return err
	}
	created := false
	defer func() {
		_ = sqlConn.Close()
		if !created {
			_ = os.Remove(destinationAbsolute)
		}
	}()
	if err := conn.Exec("VACUUM INTO ?", destinationAbsolute).Error; err != nil {
		return err
	}
	if err := os.Chmod(destinationAbsolute, sourceInfo.Mode().Perm()); err != nil {
		return err
	}
	backup, err := openReadOnly(destinationAbsolute)
	if err != nil {
		return err
	}
	backupSQL, err := backup.DB()
	if err != nil {
		return err
	}
	defer backupSQL.Close()
	var integrity string
	if err := backup.Raw("PRAGMA quick_check").Scan(&integrity).Error; err != nil {
		return err
	}
	if integrity != "ok" {
		return fmt.Errorf("backup integrity check failed: %s", integrity)
	}
	file, err := os.Open(destinationAbsolute)
	if err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	created = true
	return nil
}

func writeReport(report deliveryplanning.WorkItemAssetBackfillReport) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	exitOnError(encoder.Encode(report))
}

func exitOnError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
