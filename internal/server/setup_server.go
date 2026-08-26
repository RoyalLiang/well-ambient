package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"

	"gorm.io/gorm"
)

const (
	SetupTokenEnvironment = "WELL_AMBIENT_SETUP_TOKEN"
	setupRequestLimit     = 32 << 10
	minimumSetupTokenSize = 32
)

var (
	errSetupDatabaseNotEmpty      = errors.New("database schema is not empty")
	errSetupDatabaseNotMigrated   = errors.New("database schema is not a complete well-ambient database")
	errSetupPostgresUnsupported   = errors.New("PostgreSQL version is not supported")
	errSetupCreateDatabaseDenied  = errors.New("current PostgreSQL role cannot create databases")
	errSetupLegacyMigrationFailed = errors.New("legacy SQLite migration failed")
)

type DatabaseSetupRequest struct {
	Host                string `json:"host"`
	Port                int    `json:"port"`
	Database            string `json:"database"`
	Username            string `json:"username"`
	Password            string `json:"password"`
	SSLMode             string `json:"ssl_mode"`
	SSLRootCert         string `json:"ssl_root_cert,omitempty"`
	MaintenanceDatabase string `json:"maintenance_database,omitempty"`
	Mode                string `json:"mode,omitempty"`
}

type DatabaseSetupResult struct {
	ServerVersion     string `json:"server_version"`
	SchemaState       string `json:"schema_state"`
	DatabaseExists    bool   `json:"database_exists"`
	CanCreateDatabase bool   `json:"can_create_database"`
	RequiredOperation string `json:"required_operation"`
}

type LegacySQLiteDecisionRequest struct {
	Decision string `json:"decision"`
}

type LegacySQLiteStatus struct {
	Available        bool   `json:"available"`
	SizeBytes        int64  `json:"size_bytes,omitempty"`
	TableCount       int    `json:"table_count,omitempty"`
	PromptRequired   bool   `json:"prompt_required"`
	DecisionRecorded bool   `json:"decision_recorded"`
	Decision         string `json:"decision,omitempty"`
}

type DatabaseSetupOperation struct {
	ID              string               `json:"id"`
	State           string               `json:"state"`
	Stage           string               `json:"stage"`
	Table           string               `json:"table,omitempty"`
	TablesCompleted int                  `json:"tables_completed"`
	TablesTotal     int                  `json:"tables_total"`
	RowsCopied      int64                `json:"rows_copied"`
	Database        *DatabaseSetupResult `json:"database,omitempty"`
	Error           *setupOperationError `json:"error,omitempty"`
	RestartRequired bool                 `json:"restart_required"`
}

type setupOperationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type setupPublicError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *setupPublicError) Error() string { return e.Code }

type databaseSetupBackend interface {
	Inspect(context.Context, setupDatabaseConnection) (DatabaseSetupResult, error)
	Apply(context.Context, setupDatabaseConnection, string, string, func(db.LegacyMigrationProgress)) (DatabaseSetupResult, error)
}

type setupDatabaseConnection struct {
	Target              db.Options
	Maintenance         db.Options
	TargetDatabase      string
	MaintenanceDatabase string
}

type postgresSetupBackend struct{}

func (postgresSetupBackend) Inspect(ctx context.Context, connection setupDatabaseConnection) (DatabaseSetupResult, error) {
	maintenance, err := db.Open(connection.Maintenance)
	if err != nil {
		return DatabaseSetupResult{}, err
	}
	defer func() { _ = db.CloseConnection(maintenance) }()

	version, err := inspectPostgresVersion(ctx, maintenance)
	if err != nil {
		return DatabaseSetupResult{}, err
	}
	var databaseExists bool
	if err := maintenance.WithContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = ?)", connection.TargetDatabase,
	).Scan(&databaseExists).Error; err != nil {
		return DatabaseSetupResult{}, err
	}
	var canCreateDatabase bool
	if err := maintenance.WithContext(ctx).Raw(
		"SELECT COALESCE(rolcreatedb OR rolsuper, FALSE) FROM pg_roles WHERE rolname = current_user",
	).Scan(&canCreateDatabase).Error; err != nil {
		return DatabaseSetupResult{}, err
	}
	if !databaseExists {
		return DatabaseSetupResult{
			ServerVersion: version, SchemaState: "database_missing", DatabaseExists: false,
			CanCreateDatabase: canCreateDatabase, RequiredOperation: "create_database",
		}, nil
	}

	target, err := db.Open(connection.Target)
	if err != nil {
		return DatabaseSetupResult{}, err
	}
	defer func() { _ = db.CloseConnection(target) }()
	result, err := inspectPostgresDatabase(ctx, target)
	if err != nil {
		return DatabaseSetupResult{}, err
	}
	result.ServerVersion = version
	result.DatabaseExists = true
	result.CanCreateDatabase = canCreateDatabase
	result.RequiredOperation = operationForSchemaState(result.SchemaState)
	return result, nil
}

func (backend postgresSetupBackend) Apply(
	ctx context.Context,
	connection setupDatabaseConnection,
	expectedOperation string,
	legacySQLitePath string,
	progress func(db.LegacyMigrationProgress),
) (DatabaseSetupResult, error) {
	inspection, err := backend.Inspect(ctx, connection)
	if err != nil {
		return DatabaseSetupResult{}, err
	}
	if !setupOperationCompatible(expectedOperation, inspection.RequiredOperation) {
		return DatabaseSetupResult{}, errSetupDatabaseNotMigrated
	}
	if inspection.SchemaState == "unknown" {
		return DatabaseSetupResult{}, errSetupDatabaseNotEmpty
	}
	if inspection.SchemaState == "database_missing" {
		if !inspection.CanCreateDatabase {
			return DatabaseSetupResult{}, errSetupCreateDatabaseDenied
		}
		if progress != nil {
			progress(db.LegacyMigrationProgress{Stage: "creating_database"})
		}
		if err := createPostgresDatabase(ctx, connection); err != nil {
			// CREATE DATABASE is non-transactional. If another installer won the
			// race, a fresh inspection establishes whether it is safe to continue.
			rechecked, inspectErr := backend.Inspect(ctx, connection)
			if inspectErr != nil || !rechecked.DatabaseExists {
				return DatabaseSetupResult{}, err
			}
		}
	}

	target, err := db.Open(connection.Target)
	if err != nil {
		return DatabaseSetupResult{}, err
	}
	defer func() { _ = db.CloseConnection(target) }()

	targetInspection, err := inspectPostgresDatabase(ctx, target)
	if err != nil {
		return DatabaseSetupResult{}, err
	}
	switch targetInspection.SchemaState {
	case "well_ambient":
		// Schema initialization may have committed before runtime YAML could be
		// persisted. A complete database is an idempotent recovery point.
		return backend.Inspect(ctx, connection)
	case "unknown":
		return DatabaseSetupResult{}, errSetupDatabaseNotEmpty
	}

	if legacySQLitePath != "" {
		if _, err := db.MigrateLegacySQLite(ctx, legacySQLitePath, target, MigrateReadModels, progress); err != nil {
			return DatabaseSetupResult{}, fmt.Errorf("%w: %v", errSetupLegacyMigrationFailed, err)
		}
	} else {
		if progress != nil {
			progress(db.LegacyMigrationProgress{Stage: "initializing_schema"})
		}
		if err := target.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := db.Migrate(tx); err != nil {
				return err
			}
			return MigrateReadModels(tx)
		}); err != nil {
			return DatabaseSetupResult{}, fmt.Errorf("initialize well-ambient schema: %w", err)
		}
	}
	return backend.Inspect(ctx, connection)
}

func inspectPostgresVersion(ctx context.Context, conn *gorm.DB) (string, error) {
	var versionNumberText string
	if err := conn.WithContext(ctx).Raw("SHOW server_version_num").Scan(&versionNumberText).Error; err != nil {
		return "", err
	}
	versionNumber, err := strconv.Atoi(versionNumberText)
	if err != nil || versionNumber < 140000 {
		return "", errSetupPostgresUnsupported
	}
	var version string
	if err := conn.WithContext(ctx).Raw("SHOW server_version").Scan(&version).Error; err != nil {
		return "", err
	}
	return version, nil
}

func inspectPostgresDatabase(ctx context.Context, conn *gorm.DB) (DatabaseSetupResult, error) {
	if conn == nil {
		return DatabaseSetupResult{}, errors.New("database connection is not initialized")
	}
	version, err := inspectPostgresVersion(ctx, conn)
	if err != nil {
		return DatabaseSetupResult{}, err
	}
	var tables []string
	if err := conn.WithContext(ctx).Raw(`SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'
		ORDER BY table_name`).Scan(&tables).Error; err != nil {
		return DatabaseSetupResult{}, err
	}
	state := "empty"
	if len(tables) > 0 {
		state = "unknown"
		complete, err := hasRequiredSetupSchema(ctx, conn)
		if err != nil {
			return DatabaseSetupResult{}, err
		}
		if complete && verifyReadModels(conn.WithContext(ctx)) == nil {
			state = "well_ambient"
		}
	}
	return DatabaseSetupResult{
		ServerVersion: version, SchemaState: state, DatabaseExists: true,
		RequiredOperation: operationForSchemaState(state),
	}, nil
}

func operationForSchemaState(state string) string {
	switch state {
	case "database_missing":
		return "create_database"
	case "empty":
		return "initialize_empty"
	case "well_ambient":
		return "connect_existing"
	default:
		return "blocked"
	}
}

func setupOperationCompatible(expected, actual string) bool {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == actual {
		return true
	}
	// Retrying a failed database-creation operation is safe after CREATE
	// DATABASE succeeded but the transactional schema/data stage rolled back.
	return expected == "create_database" && (actual == "initialize_empty" || actual == "connect_existing")
}

func createPostgresDatabase(ctx context.Context, connection setupDatabaseConnection) error {
	maintenance, err := db.Open(connection.Maintenance)
	if err != nil {
		return err
	}
	defer func() { _ = db.CloseConnection(maintenance) }()
	statement := createPostgresDatabaseStatement(connection.TargetDatabase)
	return maintenance.WithContext(ctx).Exec(statement).Error
}

func createPostgresDatabaseStatement(databaseName string) string {
	return fmt.Sprintf(
		"CREATE DATABASE %s WITH TEMPLATE template0 ENCODING 'UTF8'",
		quotePostgresSetupIdentifier(databaseName),
	)
}

func quotePostgresSetupIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func hasRequiredSetupSchema(ctx context.Context, conn *gorm.DB) (bool, error) {
	type schemaColumn struct {
		TableName  string `gorm:"column:table_name"`
		ColumnName string `gorm:"column:column_name"`
	}
	var rows []schemaColumn
	if err := conn.WithContext(ctx).Raw(`SELECT table_name, column_name
		FROM information_schema.columns
		WHERE table_schema = current_schema()`).Scan(&rows).Error; err != nil {
		return false, err
	}
	columns := make(map[string]map[string]struct{}, len(rows))
	for _, row := range rows {
		if columns[row.TableName] == nil {
			columns[row.TableName] = make(map[string]struct{})
		}
		columns[row.TableName][row.ColumnName] = struct{}{}
	}
	for _, model := range db.RequiredSchemaModels() {
		statement := &gorm.Statement{DB: conn}
		if err := statement.Parse(model); err != nil {
			return false, fmt.Errorf("inspect required database model: %w", err)
		}
		tableColumns, ok := columns[statement.Schema.Table]
		if !ok {
			return false, nil
		}
		for _, field := range statement.Schema.Fields {
			if field.DBName == "" || field.IgnoreMigration {
				continue
			}
			if _, ok := tableColumns[field.DBName]; !ok {
				return false, nil
			}
		}
	}
	return true, nil
}

type databaseSetupService struct {
	mu         sync.Mutex
	config     *config.Config
	configPath string
	backend    databaseSetupBackend
	persist    func(string, *config.Config) error
	operation  *DatabaseSetupOperation
}

func newDatabaseSetupService(cfg *config.Config, configPath string) *databaseSetupService {
	return &databaseSetupService{
		config:     cfg,
		configPath: configPath,
		backend:    postgresSetupBackend{},
		persist:    config.SaveConfig,
	}
}

func (s *databaseSetupService) Test(ctx context.Context, request DatabaseSetupRequest) (DatabaseSetupResult, error) {
	connection, err := s.options(request)
	if err != nil {
		return DatabaseSetupResult{}, err
	}
	result, err := s.backend.Inspect(ctx, connection)
	if err != nil {
		return DatabaseSetupResult{}, publicSetupBackendError(err)
	}
	return result, nil
}

func (s *databaseSetupService) Apply(ctx context.Context, request DatabaseSetupRequest) (DatabaseSetupResult, error) {
	connection, err := s.options(request)
	if err != nil {
		return DatabaseSetupResult{}, err
	}
	mode := strings.ToLower(strings.TrimSpace(request.Mode))
	switch mode {
	case "create_database", "initialize_empty", "connect_existing":
	default:
		return DatabaseSetupResult{}, &setupPublicError{
			Code: "invalid_mode", Message: "请先重新测试连接并使用检查结果对应的数据库操作。", Status: http.StatusBadRequest,
		}
	}

	s.mu.Lock()
	databaseConfig := s.config.Database
	s.mu.Unlock()

	inspection, err := s.backend.Inspect(ctx, connection)
	if err != nil {
		return DatabaseSetupResult{}, publicSetupBackendError(err)
	}
	if !setupOperationCompatible(mode, inspection.RequiredOperation) {
		return DatabaseSetupResult{}, publicSetupBackendError(errSetupDatabaseNotMigrated)
	}
	legacyPath := ""
	if inspection.SchemaState == "database_missing" || inspection.SchemaState == "empty" {
		legacyStatus := inspectLegacySQLiteStatus(databaseConfig)
		switch strings.ToLower(strings.TrimSpace(databaseConfig.LegacyMigrationDecision)) {
		case "migrate":
			if !legacyStatus.Available {
				return DatabaseSetupResult{}, &setupPublicError{
					Code: "legacy_sqlite_unavailable", Message: "已记录迁移本地数据，但服务器不再能读取并校验 SQLite 快照。请恢复原快照后重试。", Status: http.StatusConflict,
				}
			}
			legacyPath = strings.TrimSpace(databaseConfig.LegacySQLitePath)
		case "skip":
		default:
			if legacyStatus.Available {
				return DatabaseSetupResult{}, &setupPublicError{
					Code: "legacy_decision_required", Message: "检测到本地 SQLite 数据，请先在下一步确认迁移或跳过。", Status: http.StatusConflict,
				}
			}
		}
	}

	result, err := s.backend.Apply(ctx, connection, mode, legacyPath, s.updateMigrationProgress)
	if err != nil {
		return DatabaseSetupResult{}, publicSetupBackendError(err)
	}
	if result.SchemaState != "well_ambient" {
		return DatabaseSetupResult{}, &setupPublicError{
			Code: "schema_incomplete", Message: "数据库结构未完成，配置未保存。", Status: http.StatusUnprocessableEntity,
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	next := *s.config
	autoMigrate := false
	next.Database = config.DatabaseConfig{
		Driver:                       "postgres",
		DSN:                          connection.Target.DSN,
		AutoMigrate:                  &autoMigrate,
		MaxOpenConnections:           connection.Target.MaxOpenConnections,
		MaxIdleConnections:           connection.Target.MaxIdleConnections,
		ConnectionMaxLifetimeMinutes: connection.Target.ConnectionMaxLifetimeMinutes,
		ConnectionMaxIdleTimeMinutes: connection.Target.ConnectionMaxIdleTimeMinutes,
	}
	if strings.TrimSpace(s.configPath) == "" {
		return DatabaseSetupResult{}, &setupPublicError{
			Code: "config_not_writable", Message: "运行配置路径不可用，数据库配置未保存。", Status: http.StatusInternalServerError,
		}
	}
	if err := s.persist(s.configPath, &next); err != nil {
		return DatabaseSetupResult{}, &setupPublicError{
			Code: "config_not_writable", Message: "数据库已就绪，但运行配置无法安全写入。请检查配置目录权限后重试。", Status: http.StatusInternalServerError,
		}
	}
	*s.config = next
	return result, nil
}

func (s *databaseSetupService) StartApply(request DatabaseSetupRequest, onComplete func()) (DatabaseSetupOperation, error) {
	if _, err := s.options(request); err != nil {
		return DatabaseSetupOperation{}, err
	}
	s.mu.Lock()
	if s.operation != nil && (s.operation.State == "queued" || s.operation.State == "running") {
		s.mu.Unlock()
		return DatabaseSetupOperation{}, &setupPublicError{
			Code: "operation_in_progress", Message: "已有数据库安装任务正在执行。", Status: http.StatusConflict,
		}
	}
	id, err := newSetupOperationID()
	if err != nil {
		s.mu.Unlock()
		return DatabaseSetupOperation{}, &setupPublicError{
			Code: "operation_unavailable", Message: "无法创建数据库安装任务。", Status: http.StatusInternalServerError,
		}
	}
	s.operation = &DatabaseSetupOperation{ID: id, State: "queued", Stage: "queued"}
	operation := *s.operation
	s.mu.Unlock()

	go func() {
		s.mu.Lock()
		s.operation.State = "running"
		s.operation.Stage = "checking_database"
		s.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()
		result, applyErr := s.Apply(ctx, request)
		request.Password = ""
		s.mu.Lock()
		if applyErr != nil {
			publicErr := publicSetupBackendError(applyErr)
			var safe *setupPublicError
			if !errors.As(publicErr, &safe) {
				safe = &setupPublicError{Code: "setup_failed", Message: "数据库配置失败。", Status: http.StatusInternalServerError}
			}
			s.operation.State = "failed"
			s.operation.Stage = "failed"
			s.operation.Error = &setupOperationError{Code: safe.Code, Message: safe.Message}
			s.mu.Unlock()
			return
		}
		s.operation.State = "completed"
		s.operation.Stage = "completed"
		s.operation.Database = &result
		s.operation.RestartRequired = true
		s.mu.Unlock()
		if onComplete != nil {
			time.AfterFunc(2*time.Second, onComplete)
		}
	}()
	return operation, nil
}

func (s *databaseSetupService) Operation() (DatabaseSetupOperation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.operation == nil {
		return DatabaseSetupOperation{}, false
	}
	copy := *s.operation
	if s.operation.Database != nil {
		database := *s.operation.Database
		copy.Database = &database
	}
	if s.operation.Error != nil {
		opErr := *s.operation.Error
		copy.Error = &opErr
	}
	return copy, true
}

func (s *databaseSetupService) updateMigrationProgress(progress db.LegacyMigrationProgress) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.operation == nil || s.operation.State != "running" {
		return
	}
	s.operation.Stage = progress.Stage
	s.operation.Table = progress.Table
	s.operation.TablesCompleted = progress.TablesCompleted
	s.operation.TablesTotal = progress.TablesTotal
	s.operation.RowsCopied = progress.RowsCopied
}

func (s *databaseSetupService) LegacySQLiteStatus() LegacySQLiteStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return inspectLegacySQLiteStatus(s.config.Database)
}

func inspectLegacySQLiteStatus(databaseConfig config.DatabaseConfig) LegacySQLiteStatus {
	decision := strings.ToLower(strings.TrimSpace(databaseConfig.LegacyMigrationDecision))
	status := LegacySQLiteStatus{
		DecisionRecorded: decision == "migrate" || decision == "skip",
		Decision:         decision,
	}
	snapshot, err := db.InspectLegacySQLite(databaseConfig.LegacySQLitePath)
	if err == nil {
		status.Available = true
		status.SizeBytes = snapshot.SizeBytes
		status.TableCount = snapshot.TableCount
	}
	status.PromptRequired = status.Available && !status.DecisionRecorded
	return status
}

func (s *databaseSetupService) RecordLegacyDecision(decision string) (LegacySQLiteStatus, error) {
	decision = strings.ToLower(strings.TrimSpace(decision))
	if decision != "migrate" && decision != "skip" {
		return LegacySQLiteStatus{}, &setupPublicError{
			Code: "invalid_legacy_decision", Message: "请选择迁移本地数据或跳过迁移。", Status: http.StatusBadRequest,
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.operation != nil && (s.operation.State == "queued" || s.operation.State == "running") {
		return LegacySQLiteStatus{}, &setupPublicError{
			Code: "operation_in_progress", Message: "数据库安装已开始，不能再修改迁移选择。", Status: http.StatusConflict,
		}
	}
	existing := strings.ToLower(strings.TrimSpace(s.config.Database.LegacyMigrationDecision))
	if existing != "" {
		if existing == decision {
			return inspectLegacySQLiteStatus(s.config.Database), nil
		}
		return LegacySQLiteStatus{}, &setupPublicError{
			Code: "legacy_decision_already_recorded", Message: "本次安装的本地数据迁移选择已经记录，只能通过服务器运行配置人工重置。", Status: http.StatusConflict,
		}
	}
	status := inspectLegacySQLiteStatus(s.config.Database)
	if decision == "migrate" && !status.Available {
		return LegacySQLiteStatus{}, &setupPublicError{
			Code: "legacy_sqlite_unavailable", Message: "服务器未发现可读取且校验通过的 SQLite 快照。", Status: http.StatusConflict,
		}
	}
	if strings.TrimSpace(s.configPath) == "" {
		return LegacySQLiteStatus{}, &setupPublicError{
			Code: "config_not_writable", Message: "运行配置路径不可用，迁移选择未保存。", Status: http.StatusInternalServerError,
		}
	}
	next := *s.config
	next.Database.LegacyMigrationDecision = decision
	if err := s.persist(s.configPath, &next); err != nil {
		return LegacySQLiteStatus{}, &setupPublicError{
			Code: "config_not_writable", Message: "迁移选择无法安全写入运行配置。请检查配置目录权限后重试。", Status: http.StatusInternalServerError,
		}
	}
	*s.config = next
	return inspectLegacySQLiteStatus(s.config.Database), nil
}

func newSetupOperationID() (string, error) {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func (s *databaseSetupService) options(request DatabaseSetupRequest) (setupDatabaseConnection, error) {
	targetDSN, err := buildPostgresSetupDSN(request)
	if err != nil {
		return setupDatabaseConnection{}, err
	}
	maintenanceDatabase := strings.TrimSpace(request.MaintenanceDatabase)
	if maintenanceDatabase == "" {
		maintenanceDatabase = "postgres"
	}
	if err := validatePostgresDatabaseName(maintenanceDatabase, "maintenance_database"); err != nil {
		return setupDatabaseConnection{}, err
	}
	maintenanceRequest := request
	maintenanceRequest.Database = maintenanceDatabase
	maintenanceDSN, err := buildPostgresSetupDSN(maintenanceRequest)
	if err != nil {
		return setupDatabaseConnection{}, err
	}

	s.mu.Lock()
	databaseConfig := s.config.Database
	s.mu.Unlock()
	maxOpen := databaseConfig.MaxOpenConnections
	if maxOpen <= 0 {
		maxOpen = 20
	}
	maxIdle := databaseConfig.MaxIdleConnections
	if maxIdle <= 0 {
		maxIdle = 10
	}
	maxLifetime := databaseConfig.ConnectionMaxLifetimeMinutes
	if maxLifetime <= 0 {
		maxLifetime = 30
	}
	maxIdleTime := databaseConfig.ConnectionMaxIdleTimeMinutes
	if maxIdleTime <= 0 {
		maxIdleTime = 5
	}
	target := db.Options{
		Driver: "postgres", DSN: targetDSN, AutoMigrate: false,
		MaxOpenConnections: maxOpen, MaxIdleConnections: maxIdle,
		ConnectionMaxLifetimeMinutes: maxLifetime,
		ConnectionMaxIdleTimeMinutes: maxIdleTime,
		Silent:                       true,
	}
	maintenance := target
	maintenance.DSN = maintenanceDSN
	maintenance.MaxOpenConnections = 2
	maintenance.MaxIdleConnections = 1
	return setupDatabaseConnection{
		Target: target, Maintenance: maintenance,
		TargetDatabase: strings.TrimSpace(request.Database), MaintenanceDatabase: maintenanceDatabase,
	}, nil
}

func buildPostgresSetupDSN(request DatabaseSetupRequest) (string, error) {
	host := strings.TrimSpace(request.Host)
	database := strings.TrimSpace(request.Database)
	username := strings.TrimSpace(request.Username)
	sslMode := strings.ToLower(strings.TrimSpace(request.SSLMode))
	rootCert := strings.TrimSpace(request.SSLRootCert)
	if host == "" || len(host) > 253 || strings.ContainsAny(host, "/?#@ \t\r\n") {
		return "", setupFieldError("host", "请输入有效的 PostgreSQL 主机名或 IP 地址。")
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	if request.Port < 1 || request.Port > 65535 {
		return "", setupFieldError("port", "端口必须在 1 到 65535 之间。")
	}
	if err := validatePostgresDatabaseName(database, "database"); err != nil {
		return "", err
	}
	if username == "" || len(username) > 128 || strings.ContainsRune(username, '\x00') {
		return "", setupFieldError("username", "请输入有效的数据库用户名。")
	}
	if request.Password == "" || len(request.Password) > 4096 || strings.ContainsRune(request.Password, '\x00') {
		return "", setupFieldError("password", "数据库密码不能为空。")
	}
	if sslMode == "" {
		sslMode = "disable"
	}
	switch sslMode {
	case "disable", "prefer", "require", "verify-ca", "verify-full":
	default:
		return "", setupFieldError("ssl_mode", "请选择受支持的 SSL 模式。")
	}
	if rootCert != "" && !filepath.IsAbs(rootCert) {
		return "", setupFieldError("ssl_root_cert", "根证书路径必须是服务器容器内的绝对路径。")
	}
	if rootCert != "" && sslMode != "verify-ca" && sslMode != "verify-full" {
		return "", setupFieldError("ssl_root_cert", "仅 verify-ca 或 verify-full 模式可填写根证书路径。")
	}

	query := url.Values{}
	query.Set("sslmode", sslMode)
	query.Set("connect_timeout", "10")
	query.Set("application_name", "well-ambient-setup")
	if rootCert != "" {
		query.Set("sslrootcert", rootCert)
	}
	connectionURL := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(username, request.Password),
		Host:     net.JoinHostPort(host, strconv.Itoa(request.Port)),
		Path:     "/" + database,
		RawQuery: query.Encode(),
	}
	return connectionURL.String(), nil
}

func validatePostgresDatabaseName(value, field string) error {
	value = strings.TrimSpace(value)
	if value == "" || len([]byte(value)) > 63 || strings.ContainsAny(value, "/\x00") {
		message := "请输入有效的数据库名（UTF-8 编码不超过 63 字节）。"
		if field == "maintenance_database" {
			message = "请输入有效的维护数据库名（通常为 postgres，UTF-8 编码不超过 63 字节）。"
		}
		return setupFieldError(field, message)
	}
	return nil
}

func setupFieldError(field, message string) error {
	return &setupPublicError{Code: "invalid_" + field, Message: message, Status: http.StatusBadRequest}
}

func publicSetupBackendError(err error) error {
	var publicError *setupPublicError
	if errors.As(err, &publicError) {
		return publicError
	}
	switch {
	case errors.Is(err, errSetupDatabaseNotEmpty):
		return &setupPublicError{Code: "database_not_empty", Message: "目标 schema 已有表且不是完整的 Well Ambient 数据库。请使用空库，或先按迁移指南完成迁移。", Status: http.StatusConflict}
	case errors.Is(err, errSetupDatabaseNotMigrated):
		return &setupPublicError{Code: "database_not_migrated", Message: "目标数据库尚未完成 Well Ambient 结构与读模型迁移，不能直接接入。", Status: http.StatusConflict}
	case errors.Is(err, errSetupPostgresUnsupported):
		return &setupPublicError{Code: "postgres_unsupported", Message: "需要 PostgreSQL 14 或更高版本。", Status: http.StatusUnprocessableEntity}
	case errors.Is(err, errSetupCreateDatabaseDenied):
		return &setupPublicError{Code: "create_database_denied", Message: "目标数据库不存在，当前 PostgreSQL 角色没有 CREATEDB 权限。请授权建库或由管理员先创建目标数据库。", Status: http.StatusForbidden}
	case errors.Is(err, errSetupLegacyMigrationFailed):
		return &setupPublicError{Code: "legacy_migration_failed", Message: "本地 SQLite 数据迁移失败，PostgreSQL 数据事务已回滚。SQLite 快照保持只读，请检查服务端日志后重试。", Status: http.StatusUnprocessableEntity}
	default:
		return &setupPublicError{Code: "database_connection_failed", Message: "数据库连接或检查失败。请核对主机、端口、数据库名、用户名、密码和 SSL 配置。", Status: http.StatusBadGateway}
	}
}

type SetupServer struct {
	config      *config.Config
	mux         *http.ServeMux
	service     *databaseSetupService
	tokenDigest [sha256.Size]byte
	completed   chan struct{}
	complete    sync.Once
}

func NewSetupServer(cfg *config.Config, configPath, token string) (*SetupServer, error) {
	if cfg == nil || !cfg.Database.RequiresSetup() {
		return nil, errors.New("database setup mode is not enabled")
	}
	if len(token) < minimumSetupTokenSize {
		return nil, fmt.Errorf("%s must contain at least %d characters", SetupTokenEnvironment, minimumSetupTokenSize)
	}
	server := &SetupServer{
		config:      cfg,
		mux:         http.NewServeMux(),
		service:     newDatabaseSetupService(cfg, configPath),
		tokenDigest: sha256.Sum256([]byte(token)),
		completed:   make(chan struct{}),
	}
	server.routes()
	return server, nil
}

func (s *SetupServer) routes() {
	s.mux.HandleFunc("GET /live", s.handleLive)
	s.mux.HandleFunc("GET /ready", s.handleReady)
	s.mux.HandleFunc("GET /health", s.handleReady)
	s.mux.HandleFunc("GET /api/setup/status", s.handleStatus)
	s.mux.HandleFunc("POST /api/setup/database/test", s.withSetupToken(s.handleTest))
	s.mux.HandleFunc("POST /api/setup/database/apply", s.withSetupToken(s.handleApply))
	s.mux.HandleFunc("GET /api/setup/database/operation", s.withSetupToken(s.handleOperation))
	s.mux.HandleFunc("POST /api/setup/legacy-sqlite/decision", s.withSetupToken(s.handleLegacySQLiteDecision))
}

func (s *SetupServer) Handler() http.Handler { return s.mux }

func (s *SetupServer) Start() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return s.Serve(ctx)
}

func (s *SetupServer) Serve(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	httpServer := &http.Server{
		Addr: addr, Handler: s.mux, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute,
	}
	serverError := make(chan error, 1)
	go func() { serverError <- httpServer.ListenAndServe() }()

	select {
	case err := <-serverError:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	case <-s.completed:
	}
	shutdownContext, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownContext); err != nil {
		_ = httpServer.Close()
		return fmt.Errorf("graceful setup HTTP shutdown: %w", err)
	}
	err := <-serverError
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *SetupServer) handleLive(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func (s *SetupServer) handleReady(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("SETUP"))
}

func (s *SetupServer) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeSetupJSON(w, http.StatusOK, map[string]any{
		"setup_required":      true,
		"database_driver":     "postgres",
		"supported_modes":     []string{"create_database", "initialize_empty", "connect_existing"},
		"supported_ssl_modes": []string{"disable", "prefer", "require", "verify-ca", "verify-full"},
		"legacy_sqlite":       s.service.LegacySQLiteStatus(),
	})
}

func (s *SetupServer) handleTest(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeSetupRequest(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	result, err := s.service.Test(ctx, request)
	if err != nil {
		writeSetupError(w, err)
		return
	}
	writeSetupJSON(w, http.StatusOK, map[string]any{"ok": true, "database": result})
}

func (s *SetupServer) handleApply(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeSetupRequest(w, r)
	if !ok {
		return
	}
	operation, err := s.service.StartApply(request, func() {
		s.complete.Do(func() { close(s.completed) })
	})
	if err != nil {
		writeSetupError(w, err)
		return
	}
	writeSetupJSON(w, http.StatusAccepted, map[string]any{
		"ok": true, "operation": operation,
	})
}

func (s *SetupServer) handleOperation(w http.ResponseWriter, _ *http.Request) {
	operation, ok := s.service.Operation()
	if !ok {
		writeSetupJSON(w, http.StatusNotFound, map[string]any{
			"error": map[string]string{"code": "operation_not_found", "message": "尚未启动数据库安装任务。"},
		})
		return
	}
	writeSetupJSON(w, http.StatusOK, map[string]any{"operation": operation})
}

func (s *SetupServer) handleLegacySQLiteDecision(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, setupRequestLimit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request LegacySQLiteDecisionRequest
	if err := decoder.Decode(&request); err != nil {
		writeSetupJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]string{"code": "invalid_request", "message": "请求格式无效或超过大小限制。"},
		})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeSetupJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]string{"code": "invalid_request", "message": "请求只能包含一个 JSON 对象。"},
		})
		return
	}
	status, err := s.service.RecordLegacyDecision(request.Decision)
	if err != nil {
		writeSetupError(w, err)
		return
	}
	writeSetupJSON(w, http.StatusOK, map[string]any{"ok": true, "legacy_sqlite": status})
}

func (s *SetupServer) withSetupToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provided := r.Header.Get("X-Setup-Token")
		providedDigest := sha256.Sum256([]byte(provided))
		if subtle.ConstantTimeCompare(providedDigest[:], s.tokenDigest[:]) != 1 {
			writeSetupJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{"code": "setup_token_invalid", "message": "安装令牌无效。"},
			})
			return
		}
		next(w, r)
	}
}

func decodeSetupRequest(w http.ResponseWriter, r *http.Request) (DatabaseSetupRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, setupRequestLimit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request DatabaseSetupRequest
	if err := decoder.Decode(&request); err != nil {
		writeSetupJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]string{"code": "invalid_request", "message": "请求格式无效或超过大小限制。"},
		})
		return DatabaseSetupRequest{}, false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeSetupJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]string{"code": "invalid_request", "message": "请求只能包含一个 JSON 对象。"},
		})
		return DatabaseSetupRequest{}, false
	}
	return request, true
}

func writeSetupError(w http.ResponseWriter, err error) {
	var publicError *setupPublicError
	if !errors.As(err, &publicError) {
		publicError = &setupPublicError{Code: "setup_failed", Message: "数据库配置失败。", Status: http.StatusInternalServerError}
	}
	writeSetupJSON(w, publicError.Status, map[string]any{"error": publicError})
}

func writeSetupJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
