package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func setupMaintenanceConfigServer(t *testing.T) (*Server, string) {
	t.Helper()
	setupServerTestDB(t)
	t.Setenv("WELL_AMBIENT_MAINTENANCE_MODE", "")
	t.Setenv("WELL_AMBIENT_MAINTENANCE_MODE_SOURCE", "")

	cfg := &config.Config{
		Database: config.DatabaseConfig{Driver: "sqlite", DSN: ":memory:"},
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
	}
	if err := BootstrapVersionedConfig(cfg); err != nil {
		t.Fatalf("bootstrap versioned config: %v", err)
	}
	token := superAdminToken(
		t,
		"maintenance-admin@westwell-lab.com",
		"Maintenance Admin",
		[]string{"config:read", "config:write"},
	)
	return NewServer(cfg, ""), token
}

func TestWellOSMaintenanceConfigPersistsAsVersionedRuntimeConfig(t *testing.T) {
	srv, token := setupMaintenanceConfigServer(t)

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/config/wellos-maintenance",
		bytes.NewBufferString(`{"enabled":true,"expected_version":1}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	srv.mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var status wellOSMaintenanceStatus
	if err := json.NewDecoder(recorder.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if !status.Configured || !status.Effective || status.Source != "database" || status.OverrideLocked || status.Version != 2 {
		t.Fatalf("unexpected maintenance status: %#v", status)
	}
	if !srv.config.Server.MaintenanceMode {
		t.Fatal("maintenance mode was not applied in memory")
	}

	var runtime db.RuntimeConfig
	if err := db.DB.First(&runtime, runtimeConfigSingletonID).Error; err != nil {
		t.Fatalf("read runtime config: %v", err)
	}
	runtimeServer := runtimeServerConfig(t, runtime.ConfigJSON)
	if runtimeServer["maintenance_mode"] != true {
		t.Fatalf("unexpected runtime config: %s", runtime.ConfigJSON)
	}
	assertNoBootstrapServerFields(t, runtimeServer)

	var version db.ConfigVersion
	if err := db.DB.Order("version desc").First(&version).Error; err != nil {
		t.Fatalf("read config version: %v", err)
	}
	if version.Version != 2 || version.Source != "wellos-maintenance" ||
		!strings.Contains(version.ChangedSectionsJSON, `"server"`) {
		t.Fatalf("unexpected config version: %#v", version)
	}
}

func TestWellOSMaintenanceConfigRejectsStaleVersion(t *testing.T) {
	srv, token := setupMaintenanceConfigServer(t)

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/config/wellos-maintenance",
		bytes.NewBufferString(`{"enabled":true,"expected_version":0}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	srv.mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "config_version_conflict") {
		t.Fatalf("stale update status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestWellOSMaintenanceConfigReportsEnvironmentOverrideAndPersistsDatabaseValue(t *testing.T) {
	srv, token := setupMaintenanceConfigServer(t)
	t.Setenv("WELL_AMBIENT_MAINTENANCE_MODE", "1")

	getRequest := httptest.NewRequest(http.MethodGet, "/api/config/wellos-maintenance", nil)
	getRequest.Header.Set("Authorization", "Bearer "+token)
	getRecorder := httptest.NewRecorder()
	srv.mux.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getRecorder.Code, getRecorder.Body.String())
	}
	var status wellOSMaintenanceStatus
	if err := json.NewDecoder(getRecorder.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.Configured || !status.Effective || !status.OverrideLocked ||
		status.Source != "environment" || status.OverrideName != "WELL_AMBIENT_MAINTENANCE_MODE" {
		t.Fatalf("unexpected override status: %#v", status)
	}

	putRequest := httptest.NewRequest(
		http.MethodPut,
		"/api/config/wellos-maintenance",
		bytes.NewBufferString(`{"enabled":true,"expected_version":1}`),
	)
	putRequest.Header.Set("Authorization", "Bearer "+token)
	putRecorder := httptest.NewRecorder()
	srv.mux.ServeHTTP(putRecorder, putRequest)
	if putRecorder.Code != http.StatusOK {
		t.Fatalf("override update status = %d, body = %s", putRecorder.Code, putRecorder.Body.String())
	}
	if err := json.NewDecoder(putRecorder.Body).Decode(&status); err != nil {
		t.Fatalf("decode updated status: %v", err)
	}
	if !status.Configured || !status.Effective || !status.OverrideLocked || status.Version != 2 {
		t.Fatalf("unexpected updated override status: %#v", status)
	}
}

func TestGenericConfigRollbackCannotChangeWellOSMaintenance(t *testing.T) {
	srv, token := setupMaintenanceConfigServer(t)

	update := httptest.NewRequest(
		http.MethodPut,
		"/api/config/wellos-maintenance",
		bytes.NewBufferString(`{"enabled":true,"expected_version":1}`),
	)
	update.Header.Set("Authorization", "Bearer "+token)
	updateRecorder := httptest.NewRecorder()
	srv.mux.ServeHTTP(updateRecorder, update)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("enable maintenance status = %d, body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	var target db.ConfigVersion
	if err := db.DB.Where("version = ?", 1).First(&target).Error; err != nil {
		t.Fatalf("read rollback target: %v", err)
	}
	rollback := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/config/versions/%d/rollback", target.ID),
		nil,
	)
	rollback.Header.Set("Authorization", "Bearer "+token)
	rollbackRecorder := httptest.NewRecorder()
	srv.mux.ServeHTTP(rollbackRecorder, rollback)
	if rollbackRecorder.Code != http.StatusOK {
		t.Fatalf("rollback status = %d, body = %s", rollbackRecorder.Code, rollbackRecorder.Body.String())
	}
	if !srv.config.Server.MaintenanceMode {
		t.Fatal("generic config rollback changed the current maintenance mode")
	}

	var runtime db.RuntimeConfig
	if err := db.DB.First(&runtime, runtimeConfigSingletonID).Error; err != nil {
		t.Fatalf("read current runtime config: %v", err)
	}
	if runtimeServerConfig(t, runtime.ConfigJSON)["maintenance_mode"] != true {
		t.Fatalf("generic rollback persisted a maintenance change: %s", runtime.ConfigJSON)
	}
}

func TestWellOSMaintenanceConfigRequiresGlobalSuperAdminToUpdate(t *testing.T) {
	srv, _ := setupMaintenanceConfigServer(t)
	user := seedLocalUserWithGroup(t, "maintenance-operator@westwell-lab.com", "Maintenance Operator", "admin", "secret")
	token, err := GenerateJWT(
		user.Username,
		user.Name,
		"mock_wellos_token",
		"",
		[]string{"admin"},
		[]string{"config:read", "config:write"},
	)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/config/wellos-maintenance",
		bytes.NewBufferString(`{"enabled":true,"expected_version":1}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	srv.mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("non-super-admin update status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
