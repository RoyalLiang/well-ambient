package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"

	"gorm.io/gorm"
)

type ConfigVersionDTO struct {
	ID                  uint                   `json:"id"`
	Version             int                    `json:"version"`
	ActorID             string                 `json:"actor_id"`
	ActorName           string                 `json:"actor_name"`
	Source              string                 `json:"source"`
	Config              map[string]interface{} `json:"config"`
	ChangedSections     []string               `json:"changed_sections"`
	Diff                []ConfigDiffEntry      `json:"diff"`
	PreviousVersionID   uint                   `json:"previous_version_id"`
	RollbackFromVersion uint                   `json:"rollback_from_version_id"`
	CreatedAt           time.Time              `json:"created_at"`
}

type ConfigDiffEntry struct {
	Path   string      `json:"path"`
	Before interface{} `json:"before"`
	After  interface{} `json:"after"`
}

func (s *Server) handleListConfigVersions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	var versions []db.ConfigVersion
	if err := db.DB.Order("version desc").Limit(limit).Find(&versions).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to query config versions: %v", err), http.StatusInternalServerError)
		return
	}

	dtos := make([]ConfigVersionDTO, 0, len(versions))
	for _, version := range versions {
		dtos = append(dtos, configVersionDTO(version))
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(dtos); err != nil {
		log.Printf("Error encoding config versions: %v", err)
	}
}

func (s *Server) handleRollbackConfigVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		http.Error(w, "Bad Request: invalid version id", http.StatusBadRequest)
		return
	}

	var target db.ConfigVersion
	if err := db.DB.First(&target, id).Error; err != nil {
		http.Error(w, "Config version not found", http.StatusNotFound)
		return
	}

	var restored config.Config
	if err := json.Unmarshal([]byte(target.ConfigJSON), &restored); err != nil {
		http.Error(w, fmt.Sprintf("Stored config snapshot is invalid: %v", err), http.StatusInternalServerError)
		return
	}

	previous := *s.config
	if err := s.applyConfig(restored); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	version, err := s.recordConfigVersion(previous, restored, r, "rollback", target.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Config restored but version archive failed: %v", err), http.StatusInternalServerError)
		return
	}
	BroadcastConfigUpdated(configVersionDTO(version))

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Configuration rolled back and applied successfully",
		"version": configVersionDTO(version),
	}); err != nil {
		log.Printf("Error encoding config rollback response: %v", err)
	}
}

func BootstrapVersionedConfig(cfg *config.Config) error {
	if db.DB == nil {
		return fmt.Errorf("database not initialized")
	}

	var latest db.ConfigVersion
	err := db.DB.Order("version desc").First(&latest).Error
	if err == nil {
		restored, unmarshalErr := restoreVersionedConfig(*cfg, latest.ConfigJSON)
		if unmarshalErr != nil {
			return fmt.Errorf("latest database config snapshot is invalid: %w", unmarshalErr)
		}
		*cfg = restored
		log.Printf("Loaded configuration from database version %d", latest.Version)
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to load latest config version: %w", err)
	}

	nextJSON, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	redactedJSON, err := json.Marshal(redactConfigForArchive(*cfg))
	if err != nil {
		return err
	}
	emptySections, _ := json.Marshal([]string{"initial"})
	emptyDiff, _ := json.Marshal([]ConfigDiffEntry{})

	version := db.ConfigVersion{
		Version:             1,
		ActorID:             "system",
		ActorName:           "system",
		Source:              "bootstrap-file",
		ConfigJSON:          string(nextJSON),
		RedactedConfigJSON:  string(redactedJSON),
		ChangedSectionsJSON: string(emptySections),
		DiffJSON:            string(emptyDiff),
		CreatedAt:           time.Now(),
	}
	if err := db.DB.Create(&version).Error; err != nil {
		return fmt.Errorf("failed to archive initial config version: %w", err)
	}
	log.Printf("Archived initial file configuration as database version %d", version.Version)
	return nil
}

// restoreVersionedConfig keeps archived sections authoritative while allowing
// configuration sections introduced after that archive was written to inherit
// their values from the current file. Without this schema-evolution merge, an
// old snapshot silently resets every newly added top-level section to Go zero
// values during startup.
func restoreVersionedConfig(fileConfig config.Config, archivedJSON string) (config.Config, error) {
	var archivedSections map[string]json.RawMessage
	if err := json.Unmarshal([]byte(archivedJSON), &archivedSections); err != nil {
		return config.Config{}, err
	}
	if archivedSections == nil {
		archivedSections = make(map[string]json.RawMessage)
	}

	fileJSON, err := json.Marshal(fileConfig)
	if err != nil {
		return config.Config{}, err
	}
	var fileSections map[string]json.RawMessage
	if err := json.Unmarshal(fileJSON, &fileSections); err != nil {
		return config.Config{}, err
	}
	for section, value := range fileSections {
		if _, exists := archivedSections[section]; !exists {
			archivedSections[section] = value
		}
	}

	mergedJSON, err := json.Marshal(archivedSections)
	if err != nil {
		return config.Config{}, err
	}
	var restored config.Config
	if err := json.Unmarshal(mergedJSON, &restored); err != nil {
		return config.Config{}, err
	}
	return restored, nil
}

func (s *Server) applyConfig(next config.Config) error {
	if s.configPath != "" {
		if err := config.SaveConfig(s.configPath, &next); err != nil {
			return fmt.Errorf("Failed to save config: %v", err)
		}
		log.Printf("Configuration saved to %s", s.configPath)
	} else {
		log.Printf("Warning: configPath is empty, configuration not saved to disk")
	}
	*s.config = next
	if s.performance != nil {
		s.performance.Reconfigure(s.performanceSettings())
	}
	return nil
}

func (s *Server) recordConfigVersion(previous config.Config, next config.Config, r *http.Request, source string, rollbackFromID uint) (db.ConfigVersion, error) {
	if db.DB == nil {
		return db.ConfigVersion{}, fmt.Errorf("database not initialized")
	}

	nextJSON, err := json.Marshal(next)
	if err != nil {
		return db.ConfigVersion{}, err
	}
	redacted := redactConfigForArchive(next)
	redactedJSON, err := json.Marshal(redacted)
	if err != nil {
		return db.ConfigVersion{}, err
	}
	diffEntries := diffConfigMaps(configToMap(redactConfigForArchive(previous)), configToMap(redacted))
	diffJSON, err := json.Marshal(diffEntries)
	if err != nil {
		return db.ConfigVersion{}, err
	}
	changedSections := changedTopLevelSections(diffEntries)
	changedJSON, err := json.Marshal(changedSections)
	if err != nil {
		return db.ConfigVersion{}, err
	}

	actorID := r.Header.Get("x-authenticated-user-id")
	actorName := r.Header.Get("x-authenticated-user-name")

	var created db.ConfigVersion
	err = db.DB.Transaction(func(tx *gorm.DB) error {
		var previousVersion db.ConfigVersion
		previousID := uint(0)
		nextVersion := 1
		err := tx.Order("version desc").First(&previousVersion).Error
		if err == nil {
			previousID = previousVersion.ID
			nextVersion = previousVersion.Version + 1
		} else if err != gorm.ErrRecordNotFound {
			return err
		}

		created = db.ConfigVersion{
			Version:               nextVersion,
			ActorID:               actorID,
			ActorName:             actorName,
			Source:                source,
			ConfigJSON:            string(nextJSON),
			RedactedConfigJSON:    string(redactedJSON),
			ChangedSectionsJSON:   string(changedJSON),
			DiffJSON:              string(diffJSON),
			PreviousVersionID:     previousID,
			RollbackFromVersionID: rollbackFromID,
			CreatedAt:             time.Now(),
		}
		return tx.Create(&created).Error
	})
	return created, err
}

func redactConfigForArchive(cfg config.Config) map[string]interface{} {
	raw := configToMap(cfg)
	return redactMap(raw, "")
}

func configToMap(value interface{}) map[string]interface{} {
	data, _ := json.Marshal(value)
	var result map[string]interface{}
	_ = json.Unmarshal(data, &result)
	if result == nil {
		return map[string]interface{}{}
	}
	return result
}

func redactMap(input map[string]interface{}, parent string) map[string]interface{} {
	output := make(map[string]interface{}, len(input))
	for key, value := range input {
		fullPath := key
		if parent != "" {
			fullPath = parent + "." + key
		}
		if isSensitiveConfigKey(key) {
			output[key] = redactSecretValue(value)
			continue
		}
		switch typed := value.(type) {
		case map[string]interface{}:
			output[key] = redactMap(typed, fullPath)
		case []interface{}:
			output[key] = redactSlice(typed, fullPath)
		default:
			output[key] = typed
		}
	}
	return output
}

func redactSlice(input []interface{}, parent string) []interface{} {
	output := make([]interface{}, 0, len(input))
	for _, value := range input {
		switch typed := value.(type) {
		case map[string]interface{}:
			output = append(output, redactMap(typed, parent))
		case []interface{}:
			output = append(output, redactSlice(typed, parent))
		default:
			output = append(output, typed)
		}
	}
	return output
}

func isSensitiveConfigKey(key string) bool {
	normalized := strings.ToLower(key)
	return strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "api_key") ||
		strings.Contains(normalized, "apikey")
}

func redactSecretValue(value interface{}) string {
	raw := fmt.Sprintf("%v", value)
	if raw == "" || raw == "<nil>" {
		return ""
	}
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("configured:%s", hex.EncodeToString(hash[:])[:12])
}

func diffConfigMaps(before map[string]interface{}, after map[string]interface{}) []ConfigDiffEntry {
	var entries []ConfigDiffEntry
	diffValue("", before, after, &entries)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries
}

func diffValue(path string, before interface{}, after interface{}, entries *[]ConfigDiffEntry) {
	beforeMap, beforeMapOK := before.(map[string]interface{})
	afterMap, afterMapOK := after.(map[string]interface{})
	if beforeMapOK && afterMapOK {
		keys := map[string]bool{}
		for key := range beforeMap {
			keys[key] = true
		}
		for key := range afterMap {
			keys[key] = true
		}
		sortedKeys := make([]string, 0, len(keys))
		for key := range keys {
			sortedKeys = append(sortedKeys, key)
		}
		sort.Strings(sortedKeys)
		for _, key := range sortedKeys {
			nextPath := key
			if path != "" {
				nextPath = path + "." + key
			}
			diffValue(nextPath, beforeMap[key], afterMap[key], entries)
		}
		return
	}

	if !reflect.DeepEqual(before, after) {
		*entries = append(*entries, ConfigDiffEntry{
			Path:   path,
			Before: before,
			After:  after,
		})
	}
}

func changedTopLevelSections(entries []ConfigDiffEntry) []string {
	seen := map[string]bool{}
	for _, entry := range entries {
		section := entry.Path
		if dot := strings.Index(section, "."); dot >= 0 {
			section = section[:dot]
		}
		if section != "" {
			seen[section] = true
		}
	}
	result := make([]string, 0, len(seen))
	for section := range seen {
		result = append(result, section)
	}
	sort.Strings(result)
	return result
}

func configVersionDTO(version db.ConfigVersion) ConfigVersionDTO {
	dto := ConfigVersionDTO{
		ID:                  version.ID,
		Version:             version.Version,
		ActorID:             version.ActorID,
		ActorName:           version.ActorName,
		Source:              version.Source,
		PreviousVersionID:   version.PreviousVersionID,
		RollbackFromVersion: version.RollbackFromVersionID,
		CreatedAt:           version.CreatedAt,
		Config:              map[string]interface{}{},
		ChangedSections:     []string{},
		Diff:                []ConfigDiffEntry{},
	}
	_ = json.Unmarshal([]byte(version.RedactedConfigJSON), &dto.Config)
	_ = json.Unmarshal([]byte(version.ChangedSectionsJSON), &dto.ChangedSections)
	_ = json.Unmarshal([]byte(version.DiffJSON), &dto.Diff)
	return dto
}
