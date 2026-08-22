package db

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

var ErrImmutablePerformanceSourceEvent = errors.New("performance source events are append-only")

// PerformanceWorkItemEvent is an immutable Jira/Git source fact used to
// reconstruct responsibility and status segments. Retention may delete expired
// rows, but source facts are never updated in place while retained.
type PerformanceWorkItemEvent struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	DedupeKey     string    `gorm:"uniqueIndex;size:255;not null;column:dedupe_key" json:"dedupe_key"`
	WorkItemID    string    `gorm:"index;size:160;not null;column:work_item_id" json:"work_item_id"`
	ProjectKey    string    `gorm:"index;size:160;column:project_key" json:"project_key"`
	IssueType     string    `gorm:"index;size:32;column:issue_type" json:"issue_type"`
	EventType     string    `gorm:"index;size:64;not null;column:event_type" json:"event_type"`
	FieldName     string    `gorm:"index;size:64;column:field_name" json:"field_name"`
	FromValue     string    `gorm:"type:text;column:from_value" json:"from_value"`
	ToValue       string    `gorm:"type:text;column:to_value" json:"to_value"`
	Actor         string    `gorm:"size:160" json:"actor"`
	OccurredAt    time.Time `gorm:"index;not null;column:occurred_at" json:"occurred_at"`
	ObservedAt    time.Time `gorm:"index;not null;column:observed_at" json:"observed_at"`
	SourceSystem  string    `gorm:"index;size:64;not null;column:source_system" json:"source_system"`
	SourceEventID string    `gorm:"index;size:255;not null;column:source_event_id" json:"source_event_id"`
	PayloadJSON   string    `gorm:"type:text;not null;column:payload_json" json:"payload_json"`
	PayloadHash   string    `gorm:"index;size:64;not null;column:payload_hash" json:"payload_hash"`
	CreatedAt     time.Time `gorm:"index;not null" json:"created_at"`
}

func (*PerformanceWorkItemEvent) BeforeUpdate(*gorm.DB) error {
	return ErrImmutablePerformanceSourceEvent
}

// PerformanceScoreRun is the immutable summary of one completed or failed
// personnel-performance calculation. A new row is created for every run.
type PerformanceScoreRun struct {
	ID                      uint       `gorm:"primaryKey" json:"id"`
	RunID                   string     `gorm:"uniqueIndex;size:80;not null;column:run_id" json:"run_id"`
	Trigger                 string     `gorm:"index;size:32;not null" json:"trigger"`
	Status                  string     `gorm:"index;size:32;not null" json:"status"`
	FormulaVersion          string     `gorm:"index;size:80;not null;column:formula_version" json:"formula_version"`
	AssessmentWindowStart   time.Time  `gorm:"index;not null;column:assessment_window_start" json:"assessment_window_start"`
	AssessmentWindowEnd     time.Time  `gorm:"index;not null;column:assessment_window_end" json:"assessment_window_end"`
	InputWatermark          time.Time  `gorm:"index;not null;column:input_watermark" json:"input_watermark"`
	SnapshotCount           int        `gorm:"not null;default:0;column:snapshot_count" json:"snapshot_count"`
	DeletedRunCount         int64      `gorm:"not null;default:0;column:deleted_run_count" json:"deleted_run_count"`
	DeletedSnapshotCount    int64      `gorm:"not null;default:0;column:deleted_snapshot_count" json:"deleted_snapshot_count"`
	DeletedEvidenceCount    int64      `gorm:"not null;default:0;column:deleted_evidence_count" json:"deleted_evidence_count"`
	DeletedAuditCount       int64      `gorm:"not null;default:0;column:deleted_audit_count" json:"deleted_audit_count"`
	DeletedSourceEventCount int64      `gorm:"not null;default:0;column:deleted_source_event_count" json:"deleted_source_event_count"`
	RetentionCutoff         time.Time  `gorm:"index;not null;column:retention_cutoff" json:"retention_cutoff"`
	ErrorMessage            string     `gorm:"type:text;column:error_message" json:"error_message"`
	CompletedAt             *time.Time `gorm:"index;column:completed_at" json:"completed_at"`
	CreatedAt               time.Time  `gorm:"index;not null" json:"created_at"`
}

// PerformanceEvidenceFact is an append-only, revisioned fact ledger. Corrections
// and voids create a new revision for the same evidence key; old rows are never
// updated in place while they remain inside the configured retention window.
type PerformanceEvidenceFact struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	EvidenceKey         string    `gorm:"uniqueIndex:idx_performance_evidence_revision,priority:1;index;size:180;not null;column:evidence_key" json:"evidence_key"`
	Revision            uint      `gorm:"uniqueIndex:idx_performance_evidence_revision,priority:2;not null;column:revision" json:"revision"`
	Action              string    `gorm:"index;size:24;not null" json:"action"`
	EventType           string    `gorm:"index;size:64;not null;column:event_type" json:"event_type"`
	SubjectKey          string    `gorm:"index;size:160;not null;column:subject_key" json:"subject_key"`
	WorkItemID          string    `gorm:"index;size:160;column:work_item_id" json:"work_item_id"`
	ProjectKey          string    `gorm:"index;size:160;column:project_key" json:"project_key"`
	Severity            string    `gorm:"size:32" json:"severity"`
	EscapeStage         string    `gorm:"size:32;column:escape_stage" json:"escape_stage"`
	ReleaseMethod       string    `gorm:"size:32;column:release_method" json:"release_method"`
	RollbackImpact      string    `gorm:"size:32;column:rollback_impact" json:"rollback_impact"`
	ReasonCode          string    `gorm:"index;size:64;column:reason_code" json:"reason_code"`
	ResponsibilityShare float64   `gorm:"not null;default:0;column:responsibility_share" json:"responsibility_share"`
	Outcome             float64   `gorm:"not null;default:0;column:outcome" json:"outcome"`
	Weight              float64   `gorm:"not null;default:1" json:"weight"`
	OccurredAt          time.Time `gorm:"index;not null;column:occurred_at" json:"occurred_at"`
	ObservedAt          time.Time `gorm:"index;not null;column:observed_at" json:"observed_at"`
	SourceSystem        string    `gorm:"index;size:80;not null;column:source_system" json:"source_system"`
	SourceRecordID      string    `gorm:"index;size:180;not null;column:source_record_id" json:"source_record_id"`
	EvidenceRef         string    `gorm:"size:320;not null;column:evidence_ref" json:"evidence_ref"`
	PayloadJSON         string    `gorm:"type:text;not null;column:payload_json" json:"payload_json"`
	PayloadHash         string    `gorm:"index;size:64;not null;column:payload_hash" json:"payload_hash"`
	CreatedBy           string    `gorm:"index;size:160;not null;column:created_by" json:"created_by"`
	CreatedAt           time.Time `gorm:"index;not null" json:"created_at"`
}

// PerformanceScoreSnapshot is an append-only derived result for one person,
// formula version, input watermark, and assessment cycle.
type PerformanceScoreSnapshot struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	RunID                 string     `gorm:"uniqueIndex:idx_performance_run_subject,priority:1;index;size:80;not null;column:run_id" json:"run_id"`
	SubjectKey            string     `gorm:"uniqueIndex:idx_performance_run_subject,priority:2;index;size:160;not null;column:subject_key" json:"subject_key"`
	FormulaVersion        string     `gorm:"index;size:80;not null;column:formula_version" json:"formula_version"`
	AssessmentWindowStart time.Time  `gorm:"index;not null;column:assessment_window_start" json:"assessment_window_start"`
	AssessmentWindowEnd   time.Time  `gorm:"index;not null;column:assessment_window_end" json:"assessment_window_end"`
	InputWatermark        time.Time  `gorm:"index;not null;column:input_watermark" json:"input_watermark"`
	InputDigest           string     `gorm:"index;size:64;not null;column:input_digest" json:"input_digest"`
	DeliveryUnitCount     int        `gorm:"not null;default:0;column:delivery_unit_count" json:"delivery_unit_count"`
	BugCount              int        `gorm:"not null;default:0;column:bug_count" json:"bug_count"`
	DemandDelayCount      int        `gorm:"not null;default:0;column:demand_delay_count" json:"demand_delay_count"`
	BugReopenCount        int        `gorm:"not null;default:0;column:bug_reopen_count" json:"bug_reopen_count"`
	CommitCount           int        `gorm:"not null;default:0;column:commit_count" json:"commit_count"`
	DuplicateCommitCount  int        `gorm:"not null;default:0;column:duplicate_commit_count" json:"duplicate_commit_count"`
	EffectiveSampleCount  int        `gorm:"not null;default:0;column:effective_sample_count" json:"effective_sample_count"`
	ExposureDays          int        `gorm:"not null;default:0;column:exposure_days" json:"exposure_days"`
	EvidenceCoverage      float64    `gorm:"not null;default:0;column:evidence_coverage" json:"evidence_coverage"`
	ObservedScore         *float64   `gorm:"column:observed_score" json:"observed_score"`
	FinalScore            *float64   `gorm:"column:final_score" json:"final_score"`
	TrendAdjustment       float64    `gorm:"not null;default:0;column:trend_adjustment" json:"trend_adjustment"`
	RiskPenalty           float64    `gorm:"not null;default:0;column:risk_penalty" json:"risk_penalty"`
	LeverageBonus         float64    `gorm:"not null;default:0;column:leverage_bonus" json:"leverage_bonus"`
	RatingStatus          string     `gorm:"index;size:40;not null;column:rating_status" json:"rating_status"`
	Level                 string     `gorm:"index;size:16" json:"level"`
	MetricsJSON           string     `gorm:"type:text;not null;column:metrics_json" json:"metrics_json"`
	ItemFactorsJSON       string     `gorm:"type:text;not null;column:item_factors_json" json:"item_factors_json"`
	ExclusionsJSON        string     `gorm:"type:text;not null;column:exclusions_json" json:"exclusions_json"`
	AdjustmentsJSON       string     `gorm:"type:text;not null;default:'[]';column:adjustments_json" json:"adjustments_json"`
	CreatedAt             time.Time  `gorm:"index;not null" json:"created_at"`
	RetainedUntil         *time.Time `gorm:"index;column:retained_until" json:"retained_until"`
}

// PerformanceAuditEvent is the append-only audit ledger for score runs,
// snapshots, failures, and retention actions.
type PerformanceAuditEvent struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	RunID          string    `gorm:"index;size:80;not null;column:run_id" json:"run_id"`
	EventType      string    `gorm:"index;size:64;not null;column:event_type" json:"event_type"`
	SubjectKey     string    `gorm:"index;size:160;column:subject_key" json:"subject_key"`
	RecordType     string    `gorm:"index;size:64;not null;column:record_type" json:"record_type"`
	RecordID       uint      `gorm:"index;column:record_id" json:"record_id"`
	FormulaVersion string    `gorm:"index;size:80;not null;column:formula_version" json:"formula_version"`
	PayloadJSON    string    `gorm:"type:text;not null;column:payload_json" json:"payload_json"`
	CreatedAt      time.Time `gorm:"index;not null" json:"created_at"`
}
