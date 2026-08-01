package db

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

var ErrImmutableDataAsset = errors.New("data asset records are append-only")

// DataAssetEvent is the narrow, indexed metadata row for one immutable fact.
// Large source payloads are deliberately stored in DataAssetEventPayload.
type DataAssetEvent struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	DedupeKey         string     `gorm:"uniqueIndex;size:255;not null;column:dedupe_key" json:"-"`
	Fingerprint       string     `gorm:"size:64;not null" json:"-"`
	ProjectKey        string     `gorm:"size:64;not null;default:'';column:project_key" json:"project_key,omitempty"`
	SubjectType       string     `gorm:"size:64;not null;column:subject_type" json:"subject_type"`
	SubjectID         string     `gorm:"size:255;not null;column:subject_id" json:"subject_id"`
	EventType         string     `gorm:"size:96;not null;column:event_type" json:"event_type"`
	SourceSystem      string     `gorm:"size:64;not null;column:source_system" json:"source_system"`
	SourceRecordID    string     `gorm:"size:255;not null;column:source_record_id" json:"source_record_id"`
	SourceEventID     string     `gorm:"size:255;not null;default:'';column:source_event_id" json:"source_event_id,omitempty"`
	ActorID           string     `gorm:"size:160;not null;column:actor_id" json:"actor_id"`
	ActorRole         string     `gorm:"size:64;not null;default:system;column:actor_role" json:"actor_role"`
	CorrelationID     string     `gorm:"size:255;not null;default:'';column:correlation_id" json:"correlation_id,omitempty"`
	CausationID       string     `gorm:"size:255;not null;default:'';column:causation_id" json:"causation_id,omitempty"`
	SupersedesEventID uint       `gorm:"not null;default:0;column:supersedes_event_id" json:"supersedes_event_id,omitempty"`
	Classification    string     `gorm:"size:32;not null;default:internal" json:"classification"`
	RetentionClass    string     `gorm:"size:32;not null;default:standard;column:retention_class" json:"retention_class"`
	SchemaVersion     int        `gorm:"not null;default:1;column:schema_version" json:"schema_version"`
	OccurredAt        time.Time  `gorm:"not null;column:occurred_at" json:"occurred_at"`
	ObservedAt        time.Time  `gorm:"not null;column:observed_at" json:"observed_at"`
	RecordedAt        time.Time  `gorm:"not null;column:recorded_at" json:"recorded_at"`
	ExpiresAt         *time.Time `gorm:"column:expires_at" json:"expires_at,omitempty"`
	PayloadHash       string     `gorm:"size:64;not null;column:payload_hash" json:"payload_hash"`
	PayloadEncoding   string     `gorm:"size:16;not null;column:payload_encoding" json:"payload_encoding"`
	PayloadBytes      int64      `gorm:"not null;column:payload_bytes" json:"payload_bytes"`
	StoredBytes       int64      `gorm:"not null;column:stored_bytes" json:"stored_bytes"`
}

func (*DataAssetEvent) BeforeUpdate(*gorm.DB) error { return ErrImmutableDataAsset }
func (*DataAssetEvent) BeforeDelete(*gorm.DB) error { return ErrImmutableDataAsset }

// DataAssetEventPayload is the cold one-to-one payload for a DataAssetEvent.
type DataAssetEventPayload struct {
	EventID      uint      `gorm:"primaryKey;autoIncrement:false;column:event_id" json:"event_id"`
	ContentType  string    `gorm:"size:96;not null;column:content_type" json:"content_type"`
	Encoding     string    `gorm:"size:16;not null" json:"encoding"`
	Data         []byte    `gorm:"type:blob;not null" json:"-"`
	OriginalSize int64     `gorm:"not null;column:original_size" json:"original_size"`
	StoredSize   int64     `gorm:"not null;column:stored_size" json:"stored_size"`
	CreatedAt    time.Time `gorm:"not null;column:created_at" json:"created_at"`
}

func (*DataAssetEventPayload) BeforeUpdate(*gorm.DB) error { return ErrImmutableDataAsset }
func (*DataAssetEventPayload) BeforeDelete(*gorm.DB) error { return ErrImmutableDataAsset }

// DataAssetSnapshot is immutable metadata for a report, metric projection, or
// governed analysis output. Its document is stored in DataAssetSnapshotPayload.
type DataAssetSnapshot struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	DedupeKey          string     `gorm:"uniqueIndex;size:255;not null;column:dedupe_key" json:"-"`
	Fingerprint        string     `gorm:"size:64;not null" json:"-"`
	Kind               string     `gorm:"size:96;not null" json:"kind"`
	ScopeType          string     `gorm:"size:64;not null;column:scope_type" json:"scope_type"`
	ScopeID            string     `gorm:"size:255;not null;column:scope_id" json:"scope_id"`
	AsOf               time.Time  `gorm:"not null;column:as_of" json:"as_of"`
	InputHighWatermark uint       `gorm:"not null;column:input_high_watermark" json:"input_high_watermark"`
	Producer           string     `gorm:"size:96;not null" json:"producer"`
	ProducerVersion    string     `gorm:"size:96;not null;column:producer_version" json:"producer_version"`
	CreatedBy          string     `gorm:"size:160;not null;column:created_by" json:"created_by"`
	Classification     string     `gorm:"size:32;not null;default:internal" json:"classification"`
	RetentionClass     string     `gorm:"size:32;not null;default:standard;column:retention_class" json:"retention_class"`
	ExpiresAt          *time.Time `gorm:"column:expires_at" json:"expires_at,omitempty"`
	EvidenceCount      int        `gorm:"not null;column:evidence_count" json:"evidence_count"`
	EvidenceHash       string     `gorm:"size:64;not null;column:evidence_hash" json:"evidence_hash"`
	PayloadHash        string     `gorm:"size:64;not null;column:payload_hash" json:"payload_hash"`
	PayloadEncoding    string     `gorm:"size:16;not null;column:payload_encoding" json:"payload_encoding"`
	PayloadBytes       int64      `gorm:"not null;column:payload_bytes" json:"payload_bytes"`
	StoredBytes        int64      `gorm:"not null;column:stored_bytes" json:"stored_bytes"`
	CreatedAt          time.Time  `gorm:"not null;column:created_at" json:"created_at"`
}

func (*DataAssetSnapshot) BeforeUpdate(*gorm.DB) error { return ErrImmutableDataAsset }
func (*DataAssetSnapshot) BeforeDelete(*gorm.DB) error { return ErrImmutableDataAsset }

type DataAssetSnapshotPayload struct {
	SnapshotID   uint      `gorm:"primaryKey;autoIncrement:false;column:snapshot_id" json:"snapshot_id"`
	ContentType  string    `gorm:"size:96;not null;column:content_type" json:"content_type"`
	Encoding     string    `gorm:"size:16;not null" json:"encoding"`
	Data         []byte    `gorm:"type:blob;not null" json:"-"`
	OriginalSize int64     `gorm:"not null;column:original_size" json:"original_size"`
	StoredSize   int64     `gorm:"not null;column:stored_size" json:"stored_size"`
	CreatedAt    time.Time `gorm:"not null;column:created_at" json:"created_at"`
}

func (*DataAssetSnapshotPayload) BeforeUpdate(*gorm.DB) error { return ErrImmutableDataAsset }
func (*DataAssetSnapshotPayload) BeforeDelete(*gorm.DB) error { return ErrImmutableDataAsset }

type DataAssetSnapshotEvidence struct {
	ID         uint `gorm:"primaryKey" json:"id"`
	SnapshotID uint `gorm:"uniqueIndex:idx_data_asset_snapshot_event,priority:1;not null;column:snapshot_id" json:"snapshot_id"`
	EventID    uint `gorm:"uniqueIndex:idx_data_asset_snapshot_event,priority:2;not null;column:event_id" json:"event_id"`
	Position   int  `gorm:"not null" json:"position"`
}

func (*DataAssetSnapshotEvidence) BeforeUpdate(*gorm.DB) error { return ErrImmutableDataAsset }
func (*DataAssetSnapshotEvidence) BeforeDelete(*gorm.DB) error { return ErrImmutableDataAsset }

var dataAssetModels = []any{
	&DataAssetEvent{},
	&DataAssetEventPayload{},
	&DataAssetSnapshot{},
	&DataAssetSnapshotPayload{},
	&DataAssetSnapshotEvidence{},
}

var dataAssetIndexes = []string{
	`CREATE INDEX IF NOT EXISTS idx_data_asset_occurred_time ON data_asset_events (occurred_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_data_asset_subject_time ON data_asset_events (subject_type, subject_id, occurred_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_data_asset_project_time ON data_asset_events (project_key, occurred_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_data_asset_source_time ON data_asset_events (source_system, occurred_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_data_asset_source_record_time ON data_asset_events (source_system, source_record_id, occurred_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_data_asset_event_type_time ON data_asset_events (event_type, occurred_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_data_asset_correlation_time ON data_asset_events (correlation_id, occurred_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_data_asset_recorded_watermark ON data_asset_events (recorded_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_data_asset_retention_expiry ON data_asset_events (retention_class, expires_at, id)`,
	`CREATE INDEX IF NOT EXISTS idx_data_asset_snapshot_scope_time ON data_asset_snapshots (kind, scope_type, scope_id, as_of DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_data_asset_snapshot_retention_expiry ON data_asset_snapshots (retention_class, expires_at, id)`,
	`CREATE INDEX IF NOT EXISTS idx_data_asset_snapshot_evidence_event ON data_asset_snapshot_evidences (event_id, snapshot_id)`,
}

var dataAssetImmutableTables = []string{
	"data_asset_events",
	"data_asset_event_payloads",
	"data_asset_snapshots",
	"data_asset_snapshot_payloads",
	"data_asset_snapshot_evidences",
}

var dataAssetImmutableInsertGuards = []struct {
	table     string
	condition string
}{
	{table: "data_asset_events", condition: "EXISTS (SELECT 1 FROM data_asset_events WHERE dedupe_key = NEW.dedupe_key OR (NEW.id > 0 AND id = NEW.id))"},
	{table: "data_asset_event_payloads", condition: "EXISTS (SELECT 1 FROM data_asset_event_payloads WHERE event_id = NEW.event_id)"},
	{table: "data_asset_snapshots", condition: "EXISTS (SELECT 1 FROM data_asset_snapshots WHERE dedupe_key = NEW.dedupe_key OR (NEW.id > 0 AND id = NEW.id))"},
	{table: "data_asset_snapshot_payloads", condition: "EXISTS (SELECT 1 FROM data_asset_snapshot_payloads WHERE snapshot_id = NEW.snapshot_id)"},
	{table: "data_asset_snapshot_evidences", condition: "EXISTS (SELECT 1 FROM data_asset_snapshot_evidences WHERE (NEW.id > 0 AND id = NEW.id) OR (snapshot_id = NEW.snapshot_id AND event_id = NEW.event_id))"},
}

// MigrateDataAssets applies only additive schema/index changes and installs
// database-level immutability guards for the active SQLite adapter.
func MigrateDataAssets(conn *gorm.DB) error {
	if conn == nil {
		return errors.New("database is not initialized")
	}
	if conn.Dialector.Name() == "sqlite" {
		return conn.Transaction(migrateDataAssetSchema)
	}
	return migrateDataAssetSchema(conn)
}

func migrateDataAssetSchema(conn *gorm.DB) error {
	if conn.Dialector.Name() == "sqlite" {
		for _, table := range dataAssetImmutableTables {
			for _, action := range []string{"update", "delete"} {
				if err := conn.Exec("DROP TRIGGER IF EXISTS trg_" + table + "_no_" + action).Error; err != nil {
					return err
				}
			}
		}
		for _, guard := range dataAssetImmutableInsertGuards {
			if err := conn.Exec("DROP TRIGGER IF EXISTS trg_" + guard.table + "_no_replace").Error; err != nil {
				return err
			}
		}
	}
	if err := conn.AutoMigrate(dataAssetModels...); err != nil {
		return err
	}
	for _, statement := range dataAssetIndexes {
		if err := conn.Exec(statement).Error; err != nil {
			return err
		}
	}
	if conn.Dialector.Name() != "sqlite" {
		return nil
	}
	for _, table := range dataAssetImmutableTables {
		for _, action := range []string{"update", "delete"} {
			statement := "CREATE TRIGGER IF NOT EXISTS trg_" + table + "_no_" + action +
				" BEFORE " + action + " ON " + table +
				" BEGIN SELECT RAISE(ABORT, 'data asset records are append-only'); END"
			if err := conn.Exec(statement).Error; err != nil {
				return err
			}
		}
	}
	for _, guard := range dataAssetImmutableInsertGuards {
		statement := "CREATE TRIGGER IF NOT EXISTS trg_" + guard.table + "_no_replace" +
			" BEFORE INSERT ON " + guard.table + " WHEN " + guard.condition +
			" BEGIN SELECT RAISE(ABORT, 'data asset records are append-only'); END"
		if err := conn.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}
