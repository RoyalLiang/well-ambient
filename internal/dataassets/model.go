package dataassets

import (
	"encoding/json"
	"errors"
	"time"
)

const (
	ClassificationPublic       = "public"
	ClassificationInternal     = "internal"
	ClassificationConfidential = "confidential"
	ClassificationRestricted   = "restricted"

	RetentionEphemeral = "ephemeral"
	RetentionStandard  = "standard"
	RetentionLongTerm  = "long_term"
	RetentionLegalHold = "legal_hold"
)

var (
	ErrNotInitialized       = errors.New("data asset module is not initialized")
	ErrInvalidCommand       = errors.New("invalid data asset command")
	ErrIdempotencyConflict  = errors.New("data asset idempotency conflict")
	ErrPayloadTooLarge      = errors.New("data asset payload exceeds configured limit")
	ErrPayloadIntegrity     = errors.New("data asset payload failed integrity validation")
	ErrQueryScopeRequired   = errors.New("data asset timeline requires a bounded scope or time range")
	ErrCursorInvalid        = errors.New("invalid data asset cursor")
	ErrCursorFilterMismatch = errors.New("data asset cursor does not match query filters")
	ErrEvidenceMissing      = errors.New("snapshot references missing data asset evidence")
	ErrEvidenceAfterAsOf    = errors.New("snapshot references evidence after its as-of time")
	ErrInputWatermark       = errors.New("snapshot input high watermark is not present in the ledger")
	ErrEvidenceLimit        = errors.New("snapshot evidence exceeds configured limit")
)

type AppendCommand struct {
	DedupeKey         string
	ProjectKey        string
	SubjectType       string
	SubjectID         string
	EventType         string
	SourceSystem      string
	SourceRecordID    string
	SourceEventID     string
	ActorID           string
	ActorRole         string
	CorrelationID     string
	CausationID       string
	SupersedesEventID uint
	Classification    string
	RetentionClass    string
	SchemaVersion     int
	OccurredAt        time.Time
	ObservedAt        time.Time
	ExpiresAt         *time.Time
	Payload           any
}

type Event struct {
	ID                uint       `json:"id"`
	ProjectKey        string     `json:"project_key,omitempty"`
	SubjectType       string     `json:"subject_type"`
	SubjectID         string     `json:"subject_id"`
	EventType         string     `json:"event_type"`
	SourceSystem      string     `json:"source_system"`
	SourceRecordID    string     `json:"source_record_id"`
	SourceEventID     string     `json:"source_event_id,omitempty"`
	ActorID           string     `json:"actor_id"`
	ActorRole         string     `json:"actor_role"`
	CorrelationID     string     `json:"correlation_id,omitempty"`
	CausationID       string     `json:"causation_id,omitempty"`
	SupersedesEventID uint       `json:"supersedes_event_id,omitempty"`
	Classification    string     `json:"classification"`
	RetentionClass    string     `json:"retention_class"`
	SchemaVersion     int        `json:"schema_version"`
	OccurredAt        time.Time  `json:"occurred_at"`
	ObservedAt        time.Time  `json:"observed_at"`
	RecordedAt        time.Time  `json:"recorded_at"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	PayloadHash       string     `json:"payload_hash"`
	PayloadEncoding   string     `json:"payload_encoding"`
	PayloadBytes      int64      `json:"payload_bytes"`
	StoredBytes       int64      `json:"stored_bytes"`
}

type AppendResult struct {
	Event    Event `json:"event"`
	Replayed bool  `json:"replayed"`
}

type AppendInspection struct {
	Exists   bool  `json:"exists"`
	Replayed bool  `json:"replayed"`
	Conflict bool  `json:"conflict"`
	Event    Event `json:"event,omitempty"`
}

type EventRecord struct {
	Event
	Payload json.RawMessage `json:"payload"`
}

type TimelineQuery struct {
	ProjectKey     string
	SubjectType    string
	SubjectID      string
	EventTypes     []string
	SourceSystem   string
	SourceRecordID string
	CorrelationID  string
	Since          *time.Time
	Until          *time.Time
	Limit          int
	Cursor         string
}

type TimelinePage struct {
	Items         []Event `json:"items"`
	NextCursor    string  `json:"next_cursor,omitempty"`
	HighWatermark uint    `json:"high_watermark"`
}

type SealSnapshotCommand struct {
	DedupeKey          string
	Kind               string
	ScopeType          string
	ScopeID            string
	AsOf               time.Time
	InputHighWatermark uint
	Producer           string
	ProducerVersion    string
	CreatedBy          string
	Classification     string
	RetentionClass     string
	ExpiresAt          *time.Time
	EvidenceEventIDs   []uint
	Payload            any
}

type Snapshot struct {
	ID                 uint       `json:"id"`
	Kind               string     `json:"kind"`
	ScopeType          string     `json:"scope_type"`
	ScopeID            string     `json:"scope_id"`
	AsOf               time.Time  `json:"as_of"`
	InputHighWatermark uint       `json:"input_high_watermark"`
	Producer           string     `json:"producer"`
	ProducerVersion    string     `json:"producer_version"`
	CreatedBy          string     `json:"created_by"`
	Classification     string     `json:"classification"`
	RetentionClass     string     `json:"retention_class"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
	EvidenceCount      int        `json:"evidence_count"`
	EvidenceHash       string     `json:"evidence_hash"`
	PayloadHash        string     `json:"payload_hash"`
	PayloadEncoding    string     `json:"payload_encoding"`
	PayloadBytes       int64      `json:"payload_bytes"`
	StoredBytes        int64      `json:"stored_bytes"`
	CreatedAt          time.Time  `json:"created_at"`
}

type SealSnapshotResult struct {
	Snapshot Snapshot `json:"snapshot"`
	Replayed bool     `json:"replayed"`
}

type SnapshotRecord struct {
	Snapshot
	EvidenceEventIDs []uint          `json:"evidence_event_ids"`
	Payload          json.RawMessage `json:"payload"`
}

type LatestSnapshotQuery struct {
	Kind      string
	ScopeType string
	ScopeID   string
	AsOf      *time.Time
}
