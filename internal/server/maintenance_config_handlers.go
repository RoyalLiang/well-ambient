package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

type wellOSMaintenanceStatus struct {
	Configured     bool      `json:"configured"`
	Effective      bool      `json:"effective"`
	Source         string    `json:"source"`
	OverrideLocked bool      `json:"override_locked"`
	OverrideName   string    `json:"override_name,omitempty"`
	Version        int       `json:"version"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

type wellOSMaintenanceUpdateRequest struct {
	Enabled         bool `json:"enabled"`
	ExpectedVersion int  `json:"expected_version"`
}

func maintenanceModeOverride() (enabled bool, source, name string) {
	raw := strings.TrimSpace(os.Getenv("WELL_AMBIENT_MAINTENANCE_MODE"))
	if raw != "1" && !strings.EqualFold(raw, "true") && !strings.EqualFold(raw, "yes") && !strings.EqualFold(raw, "on") {
		return false, "database", ""
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("WELL_AMBIENT_MAINTENANCE_MODE_SOURCE")), "cli") {
		return true, "cli", "--maintenance-mode"
	}
	return true, "environment", "WELL_AMBIENT_MAINTENANCE_MODE"
}

func isMaintenanceModeEnabled(cfg *config.Config) bool {
	if enabled, _, _ := maintenanceModeOverride(); enabled {
		return true
	}
	return cfg != nil && cfg.Server.MaintenanceMode
}

func (s *Server) isMaintenanceMode() bool {
	return isMaintenanceModeEnabled(s.config)
}

func (s *Server) currentWellOSMaintenanceStatus() (wellOSMaintenanceStatus, error) {
	status := wellOSMaintenanceStatus{
		Configured: s.config != nil && s.config.Server.MaintenanceMode,
		Source:     "database",
	}
	if enabled, source, name := maintenanceModeOverride(); enabled {
		status.Effective = true
		status.Source = source
		status.OverrideLocked = true
		status.OverrideName = name
	} else {
		status.Effective = status.Configured
	}
	if db.DB == nil {
		return status, nil
	}
	var current db.RuntimeConfig
	if err := db.DB.First(&current, runtimeConfigSingletonID).Error; err != nil {
		return status, err
	}
	status.Version = current.Version
	status.UpdatedAt = current.UpdatedAt
	return status, nil
}

func (s *Server) handleGetWellOSMaintenance(w http.ResponseWriter, r *http.Request) {
	status, err := s.currentWellOSMaintenanceStatus()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read maintenance configuration: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func (s *Server) handleUpdateWellOSMaintenance(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}
	var request wellOSMaintenanceUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		return
	}

	s.configMutationMu.Lock()
	defer s.configMutationMu.Unlock()

	var current db.RuntimeConfig
	if err := db.DB.First(&current, runtimeConfigSingletonID).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to read current configuration: %v", err), http.StatusInternalServerError)
		return
	}
	if request.ExpectedVersion != current.Version {
		writeLoginError(w, http.StatusConflict, "config_version_conflict", "配置已被其他操作更新，请刷新后重试。")
		return
	}

	previous, err := restoreVersionedConfig(*s.config, current.ConfigJSON)
	if err != nil {
		http.Error(w, fmt.Sprintf("Current database configuration is invalid: %v", err), http.StatusInternalServerError)
		return
	}
	previous.Database = s.config.Database
	previous.Server.Host = s.config.Server.Host
	previous.Server.Port = s.config.Server.Port
	previous.Server.AttachmentDir = s.config.Server.AttachmentDir
	next := previous
	next.Server.MaintenanceMode = request.Enabled

	version, err := s.recordConfigVersion(previous, next, r, "wellos-maintenance", 0)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to persist maintenance configuration: %v", err), http.StatusInternalServerError)
		return
	}
	if err := s.applyConfig(next); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	action := "wellos_maintenance_disabled"
	if request.Enabled {
		action = "wellos_maintenance_enabled"
	}
	userdb.RecordAuditLog(
		db.DB,
		r.Header.Get("x-authenticated-user-id"),
		action,
		"config",
		"wellos-maintenance",
		fmt.Sprintf("WellOS maintenance login configured=%t version=%d", request.Enabled, version.Version),
		r.RemoteAddr,
	)
	BroadcastConfigUpdated(configVersionDTO(version))

	status, err := s.currentWellOSMaintenanceStatus()
	if err != nil {
		http.Error(w, fmt.Sprintf("Configuration saved but readback failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}
