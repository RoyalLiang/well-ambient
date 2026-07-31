package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"well-ambient/internal/db"
	"well-ambient/internal/deliveryplanning"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	var databasePath string
	var backupPath string
	var apply bool
	flag.StringVar(&databasePath, "database", "well-ambient.db", "path to the SQLite database")
	flag.StringVar(&backupPath, "backup", "", "required new backup file path when --apply is used")
	flag.BoolVar(&apply, "apply", false, "apply the migration; default behavior is read-only dry-run")
	flag.Parse()

	ctx := context.Background()
	if !apply {
		report, err := deliveryplanning.ReadSQLiteMigrationReport(ctx, databasePath)
		exitOnError(err)
		writeReport(report)
		return
	}
	if backupPath == "" {
		exitOnError(fmt.Errorf("--backup is required with --apply"))
	}
	exitOnError(copyBackup(databasePath, backupPath))

	conn, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	exitOnError(err)
	exitOnError(conn.AutoMigrate(
		&db.TaskTelemetry{},
		&db.ReleaseVersion{},
		&db.WorkItemReleaseLink{},
		&db.WorkItemEvent{},
		&db.WorkItemSyncOperation{},
	))
	report, err := deliveryplanning.LoadMigrationReport(ctx, conn)
	exitOnError(err)
	applied, err := deliveryplanning.ApplyMigration(ctx, conn, report)
	exitOnError(err)
	writeReport(applied)
}

func copyBackup(sourcePath, destinationPath string) error {
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
	source, err := os.Open(sourceAbsolute)
	if err != nil {
		return err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("database path is not a regular file")
	}
	destination, err := os.OpenFile(destinationAbsolute, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	copied := false
	defer func() {
		_ = destination.Close()
		if !copied {
			_ = os.Remove(destinationAbsolute)
		}
	}()
	if _, err := io.Copy(destination, source); err != nil {
		return err
	}
	if err := destination.Sync(); err != nil {
		return err
	}
	if err := destination.Close(); err != nil {
		return err
	}
	copied = true
	return nil
}

func writeReport(report deliveryplanning.MigrationReport) {
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
