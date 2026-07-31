package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"well-ambient/internal/deliveryplanning"
)

type output struct {
	GeneratedAt    string                          `json:"generated_at"`
	DatabaseSHA256 string                          `json:"database_sha256"`
	Report         deliveryplanning.BaselineReport `json:"report"`
}

func main() {
	databasePath := flag.String("database", "well-ambient.db", "SQLite database path (opened read-only)")
	flag.Parse()

	report, err := deliveryplanning.ReadSQLiteBaseline(context.Background(), *databasePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "delivery audit failed: %v\n", err)
		os.Exit(1)
	}
	checksum, err := fileSHA256(*databasePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "delivery audit checksum failed: %v\n", err)
		os.Exit(1)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output{
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		DatabaseSHA256: checksum,
		Report:         report,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "delivery audit output failed: %v\n", err)
		os.Exit(1)
	}
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
