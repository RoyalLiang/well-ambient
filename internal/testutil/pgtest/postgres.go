// Package pgtest runs opt-in tests against a disposable local PostgreSQL server.
// It never accepts a DSN or connects to an existing database.
package pgtest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open requires WELL_AMBIENT_TEST_PG_BIN to point at PostgreSQL's bin directory.
// Each invocation owns its cluster and private Unix socket, with TCP disabled.
func Open(t testing.TB) *gorm.DB {
	t.Helper()
	bin := os.Getenv("WELL_AMBIENT_TEST_PG_BIN")
	if bin == "" {
		t.Skip("set WELL_AMBIENT_TEST_PG_BIN to run isolated PostgreSQL tests")
	}
	// macOS limits Unix socket paths; testing.TempDir may exceed that limit.
	root, err := os.MkdirTemp("/tmp", "wa-pg-")
	if err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(root, "data")
	started := false
	t.Cleanup(func() {
		if started {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if output, err := exec.CommandContext(ctx, filepath.Join(bin, "pg_ctl"), "-D", data, "-m", "immediate", "-w", "stop").CombinedOutput(); err != nil {
				t.Errorf("stop test PostgreSQL (preserved %s): %v\n%s", root, err, output)
				return
			}
		}
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove own PostgreSQL fixture: %v", err)
		}
	})
	run := func(name string, args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if output, err := exec.CommandContext(ctx, filepath.Join(bin, name), args...).CombinedOutput(); err != nil {
			log, _ := os.ReadFile(filepath.Join(root, "postgres.log"))
			t.Fatalf("%s: %v\n%s\n%s", name, err, output, log)
		}
	}
	run("initdb", "-D", data, "-U", "postgres", "--auth-local=trust", "--auth-host=reject", "--encoding=UTF8", "--locale=C", "--no-sync")
	// The socket lives in a new mode-0700 directory, with no network listener.
	// A failed/timed-out pg_ctl can still have started a child; attempt to stop
	// it during cleanup before considering removal of its data directory.
	started = true
	run("pg_ctl", "-D", data, "-l", filepath.Join(root, "postgres.log"), "-o",
		fmt.Sprintf("-F -c listen_addresses='' -c max_connections=20 -c timezone=UTC -k %s -p 55439", root), "-w", "start")
	conn, err := gorm.Open(postgres.Open(fmt.Sprintf("host=%s port=55439 user=postgres dbname=postgres sslmode=disable", root)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := conn.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(12)
	pool.SetMaxIdleConns(12)
	t.Cleanup(func() { _ = pool.Close() })
	return conn
}
