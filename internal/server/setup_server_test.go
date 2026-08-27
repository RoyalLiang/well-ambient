package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	stdlog "log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

const testSetupToken = "0123456789abcdef0123456789abcdef"

type fakeSetupBackend struct {
	inspectResult  DatabaseSetupResult
	inspectError   error
	applyResult    DatabaseSetupResult
	applyError     error
	inspectCalls   int
	applyCalls     int
	lastConnection setupDatabaseConnection
	lastMode       string
	lastLegacyPath string
}

type setupSQLStateError struct {
	code string
}

func (e setupSQLStateError) Error() string    { return "unsafe database detail" }
func (e setupSQLStateError) SQLState() string { return e.code }

func (f *fakeSetupBackend) Inspect(_ context.Context, connection setupDatabaseConnection) (DatabaseSetupResult, error) {
	f.inspectCalls++
	f.lastConnection = connection
	return f.inspectResult, f.inspectError
}

func (f *fakeSetupBackend) Apply(
	_ context.Context,
	connection setupDatabaseConnection,
	mode string,
	legacyPath string,
	progress func(db.LegacyMigrationProgress),
) (DatabaseSetupResult, error) {
	f.applyCalls++
	f.lastConnection = connection
	f.lastMode = mode
	f.lastLegacyPath = legacyPath
	if progress != nil {
		progress(db.LegacyMigrationProgress{Stage: "copying_data", Table: "task_telemetries", TablesCompleted: 1, TablesTotal: 2, RowsCopied: 25})
	}
	return f.applyResult, f.applyError
}

func newTestSetupServer(t *testing.T, backend databaseSetupBackend) *SetupServer {
	t.Helper()
	cfg := &config.Config{
		Database: config.DatabaseConfig{Driver: "setup"},
		Server:   config.ServerConfig{Host: "127.0.0.1", Port: 8080},
	}
	server, err := NewSetupServer(cfg, "/runtime/config.yaml", testSetupToken)
	if err != nil {
		t.Fatalf("NewSetupServer() error = %v", err)
	}
	server.service.backend = backend
	return server
}

func setupRequestBody(mode string) []byte {
	payload := DatabaseSetupRequest{
		Host: "postgres", Port: 5432, Database: "well_ambient", MaintenanceDatabase: "postgres",
		Username: "ambient", Password: "s3cret:/?#@", SSLMode: "disable", Mode: mode,
	}
	data, _ := json.Marshal(payload)
	return data
}

func TestBuildPostgresSetupDSNEncodesCredentialsAndValidatesTLS(t *testing.T) {
	dsn, err := buildPostgresSetupDSN(DatabaseSetupRequest{
		Host: "2001:db8::1", Port: 5432, Database: "well ambient", Username: "ambient@example",
		Password: "s3cret:/?#@", SSLMode: "verify-full", SSLRootCert: "/etc/well-ambient/ca.pem",
	})
	if err != nil {
		t.Fatalf("buildPostgresSetupDSN() error = %v", err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}
	password, ok := parsed.User.Password()
	if !ok || parsed.User.Username() != "ambient@example" || password != "s3cret:/?#@" {
		t.Fatal("credentials did not round-trip through encoded DSN")
	}
	if parsed.Hostname() != "2001:db8::1" || parsed.Port() != "5432" || parsed.Query().Get("sslrootcert") != "/etc/well-ambient/ca.pem" {
		t.Fatalf("unexpected DSN fields: %s", dsn)
	}
	if !strings.Contains(parsed.EscapedPath(), "well%20ambient") {
		t.Fatalf("database path is not escaped: %s", parsed.EscapedPath())
	}

	_, err = buildPostgresSetupDSN(DatabaseSetupRequest{
		Host: "postgres", Port: 5432, Database: "ambient", Username: "ambient", Password: "secret",
		SSLMode: "disable", SSLRootCert: "relative-ca.pem",
	})
	var publicError *setupPublicError
	if !errors.As(err, &publicError) || publicError.Code != "invalid_ssl_root_cert" {
		t.Fatalf("relative root cert error = %#v", err)
	}
	if err := validatePostgresDatabaseName(strings.Repeat("数", 22), "database"); err == nil {
		t.Fatal("database name longer than PostgreSQL's 63-byte identifier limit was accepted")
	}
}

func TestCreatePostgresDatabaseStatementQuotesIdentifierAndUsesPristineTemplate(t *testing.T) {
	statement := createPostgresDatabaseStatement(`ambient"prod`)
	if statement != `CREATE DATABASE "ambient""prod" WITH TEMPLATE template0 ENCODING 'UTF8'` {
		t.Fatalf("unexpected CREATE DATABASE statement: %s", statement)
	}
}

func TestSetupStatusIsPublicButWritesRequireOneTimeToken(t *testing.T) {
	backend := &fakeSetupBackend{inspectResult: missingDatabaseResult()}
	server := newTestSetupServer(t, backend)

	status := httptest.NewRecorder()
	server.Handler().ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/api/setup/status", nil))
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"setup_required":true`) {
		t.Fatalf("status = %d %s", status.Code, status.Body.String())
	}
	if strings.Contains(status.Body.String(), testSetupToken) {
		t.Fatal("public setup status exposed the setup token")
	}

	unauthorized := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/setup/database/test", bytes.NewReader(setupRequestBody("create_database")))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Setup-Token", "wrong-token")
	server.Handler().ServeHTTP(unauthorized, request)
	if unauthorized.Code != http.StatusUnauthorized || backend.inspectCalls != 0 {
		t.Fatalf("unauthorized test = %d calls=%d body=%s", unauthorized.Code, backend.inspectCalls, unauthorized.Body.String())
	}
}

func TestSetupTestExplicitlyReportsMissingTargetDatabaseAndCreatePermission(t *testing.T) {
	backend := &fakeSetupBackend{inspectResult: missingDatabaseResult()}
	server := newTestSetupServer(t, backend)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/setup/database/test", bytes.NewReader(setupRequestBody("create_database")))
	request.Header.Set("X-Setup-Token", testSetupToken)
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || backend.inspectCalls != 1 {
		t.Fatalf("test = %d calls=%d body=%s", recorder.Code, backend.inspectCalls, recorder.Body.String())
	}
	if !backend.lastConnection.Target.Silent || !backend.lastConnection.Maintenance.Silent {
		t.Fatal("setup probes must suppress logs that could contain connection secrets")
	}
	target, _ := url.Parse(backend.lastConnection.Target.DSN)
	maintenance, _ := url.Parse(backend.lastConnection.Maintenance.DSN)
	if target.Path != "/well_ambient" || maintenance.Path != "/postgres" {
		t.Fatalf("unexpected target/maintenance DSNs: %s %s", target.Path, maintenance.Path)
	}
	body := recorder.Body.String()
	for _, fact := range []string{`"schema_state":"database_missing"`, `"database_exists":false`, `"can_create_database":true`, `"required_operation":"create_database"`} {
		if !strings.Contains(body, fact) {
			t.Fatalf("missing inspection fact %s: %s", fact, body)
		}
	}
	if strings.Contains(body, "s3cret") || strings.Contains(body, testSetupToken) {
		t.Fatalf("test response exposed a secret: %s", body)
	}
}

func TestSetupApplyPersistsPostgresAndClearsLegacySetupState(t *testing.T) {
	backend := &fakeSetupBackend{inspectResult: missingDatabaseResult(), applyResult: configuredDatabaseResult()}
	server := newTestSetupServer(t, backend)
	server.config.Database.LegacyMigrationDecision = "skip"
	server.config.Database.LegacySQLitePath = "/var/lib/well-ambient/legacy/well-ambient.db"
	persistCalls := 0
	server.service.persist = func(path string, cfg *config.Config) error {
		persistCalls++
		if path != "/runtime/config.yaml" || cfg.Database.Driver != "postgres" || cfg.Database.DSN == "" || cfg.Database.DSNEnv != "" {
			t.Fatalf("unsafe persisted config: path=%q cfg=%#v", path, cfg.Database)
		}
		if cfg.Database.AutoMigrate == nil || *cfg.Database.AutoMigrate {
			t.Fatalf("persisted auto_migrate = %#v, want false", cfg.Database.AutoMigrate)
		}
		if cfg.Database.LegacySQLitePath != "" || cfg.Database.LegacyMigrationDecision != "" {
			t.Fatalf("setup-only legacy state survived final config: %#v", cfg.Database)
		}
		return nil
	}

	request := DatabaseSetupRequest{
		Host: "postgres", Port: 5432, Database: "well_ambient", MaintenanceDatabase: "postgres",
		Username: "ambient", Password: "secret", SSLMode: "disable", Mode: "create_database",
	}
	result, err := server.service.Apply(context.Background(), request)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if result.SchemaState != "well_ambient" || backend.applyCalls != 1 || persistCalls != 1 {
		t.Fatalf("result=%#v apply=%d persist=%d", result, backend.applyCalls, persistCalls)
	}
	if server.config.Database.Driver != "postgres" {
		t.Fatalf("runtime config driver = %q", server.config.Database.Driver)
	}
}

func TestSetupApplyRunsAsPollableOperation(t *testing.T) {
	backend := &fakeSetupBackend{inspectResult: missingDatabaseResult(), applyResult: configuredDatabaseResult()}
	server := newTestSetupServer(t, backend)
	server.service.persist = func(string, *config.Config) error { return nil }

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/setup/database/apply", bytes.NewReader(setupRequestBody("create_database")))
	request.Header.Set("X-Setup-Token", testSetupToken)
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted || !strings.Contains(recorder.Body.String(), `"state":"queued"`) {
		t.Fatalf("async apply = %d %s", recorder.Code, recorder.Body.String())
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		operation, ok := server.service.Operation()
		if ok && operation.State == "completed" {
			if operation.RowsCopied != 25 || operation.Table != "task_telemetries" || !operation.RestartRequired {
				t.Fatalf("unexpected completed operation: %#v", operation)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("setup operation did not complete")
}

func TestSetupApplyFailureKeepsLastStageAndLogsOnlySafeDiagnostic(t *testing.T) {
	backend := &fakeSetupBackend{
		inspectResult: missingDatabaseResult(),
		applyError:    wrapLegacyMigrationFailure(setupSQLStateError{code: "22021"}),
	}
	server := newTestSetupServer(t, backend)
	server.service.persist = func(string, *config.Config) error { return nil }

	var output bytes.Buffer
	originalOutput := stdlog.Writer()
	originalFlags := stdlog.Flags()
	originalPrefix := stdlog.Prefix()
	stdlog.SetOutput(&output)
	stdlog.SetFlags(0)
	stdlog.SetPrefix("")
	t.Cleanup(func() {
		stdlog.SetOutput(originalOutput)
		stdlog.SetFlags(originalFlags)
		stdlog.SetPrefix(originalPrefix)
	})

	request := DatabaseSetupRequest{
		Host: "postgres", Port: 5432, Database: "well_ambient", MaintenanceDatabase: "postgres",
		Username: "ambient", Password: "s3cret", SSLMode: "disable", Mode: "create_database",
	}
	if _, err := server.service.StartApply(request, nil); err != nil {
		t.Fatalf("StartApply() error = %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		operation, ok := server.service.Operation()
		if ok && operation.State == "failed" {
			if operation.Stage != "copying_data" || operation.Table != "task_telemetries" {
				t.Fatalf("failure lost last progress: %#v", operation)
			}
			if operation.Error == nil || !strings.Contains(operation.Error.Message, "PostgreSQL 错误码：22021") {
				t.Fatalf("failure did not expose the safe SQLSTATE reference: %#v", operation.Error)
			}
			logged := output.String()
			for _, fact := range []string{"code=legacy_migration_failed", "stage=copying_data", "table=task_telemetries", "sqlstate=22021"} {
				if !strings.Contains(logged, fact) {
					t.Fatalf("safe diagnostic missing %q: %s", fact, logged)
				}
			}
			if strings.Contains(logged, "s3cret") || strings.Contains(logged, "unsafe database detail") {
				t.Fatalf("setup failure log exposed sensitive detail: %s", logged)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("setup failure operation did not finish")
}

func TestLegacySQLiteDecisionIsPubliclyDiscoverableButRecordedOnlyOnce(t *testing.T) {
	backend := &fakeSetupBackend{inspectResult: missingDatabaseResult()}
	server := newTestSetupServer(t, backend)
	legacyPath := createLegacySQLiteFixture(t)
	server.config.Database.LegacySQLitePath = legacyPath
	server.service.persist = func(_ string, cfg *config.Config) error {
		if cfg.Database.LegacyMigrationDecision != "migrate" {
			t.Fatalf("unexpected persisted decision: %#v", cfg.Database)
		}
		return nil
	}

	status := httptest.NewRecorder()
	server.Handler().ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/api/setup/status", nil))
	if !strings.Contains(status.Body.String(), `"available":true`) || !strings.Contains(status.Body.String(), `"prompt_required":true`) {
		t.Fatalf("legacy status = %s", status.Body.String())
	}
	if strings.Contains(status.Body.String(), legacyPath) {
		t.Fatal("public setup status exposed server filesystem path")
	}

	first := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodPost, "/api/setup/legacy-sqlite/decision", strings.NewReader(`{"decision":"migrate"}`))
	firstRequest.Header.Set("X-Setup-Token", testSetupToken)
	server.Handler().ServeHTTP(first, firstRequest)
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), `"decision_recorded":true`) {
		t.Fatalf("record decision = %d %s", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodPost, "/api/setup/legacy-sqlite/decision", strings.NewReader(`{"decision":"skip"}`))
	secondRequest.Header.Set("X-Setup-Token", testSetupToken)
	server.Handler().ServeHTTP(second, secondRequest)
	if second.Code != http.StatusConflict || !strings.Contains(second.Body.String(), "legacy_decision_already_recorded") {
		t.Fatalf("second decision = %d %s", second.Code, second.Body.String())
	}
}

func TestSetupConnectExistingRejectsEmptySchemaWithoutPersisting(t *testing.T) {
	backend := &fakeSetupBackend{inspectResult: DatabaseSetupResult{
		ServerVersion: "17.11", SchemaState: "empty", DatabaseExists: true, RequiredOperation: "initialize_empty",
	}}
	server := newTestSetupServer(t, backend)
	persistCalls := 0
	server.service.persist = func(string, *config.Config) error { persistCalls++; return nil }
	request := DatabaseSetupRequest{
		Host: "postgres", Port: 5432, Database: "well_ambient", Username: "ambient", Password: "secret",
		SSLMode: "disable", Mode: "connect_existing",
	}
	if _, err := server.service.Apply(context.Background(), request); err == nil || persistCalls != 0 || backend.applyCalls != 0 {
		t.Fatalf("empty schema connect was not blocked: err=%v persist=%d apply=%d", err, persistCalls, backend.applyCalls)
	}
}

func TestSetupBackendErrorsDoNotEchoDatabaseSecrets(t *testing.T) {
	backend := &fakeSetupBackend{inspectError: errors.New("dial postgres://ambient:s3cret@postgres/well_ambient failed")}
	server := newTestSetupServer(t, backend)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/setup/database/test", bytes.NewReader(setupRequestBody("create_database")))
	request.Header.Set("X-Setup-Token", testSetupToken)
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadGateway || strings.Contains(recorder.Body.String(), "s3cret") || strings.Contains(recorder.Body.String(), "postgres://") {
		t.Fatalf("unsafe backend error response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func missingDatabaseResult() DatabaseSetupResult {
	return DatabaseSetupResult{
		ServerVersion: "17.11", SchemaState: "database_missing", DatabaseExists: false,
		CanCreateDatabase: true, RequiredOperation: "create_database",
	}
}

func configuredDatabaseResult() DatabaseSetupResult {
	return DatabaseSetupResult{
		ServerVersion: "17.11", SchemaState: "well_ambient", DatabaseExists: true,
		CanCreateDatabase: true, RequiredOperation: "connect_existing",
	}
}

func createLegacySQLiteFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "well-ambient.db")
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`CREATE TABLE task_telemetries (task_id TEXT PRIMARY KEY, title TEXT)`); err != nil {
		conn.Close()
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
