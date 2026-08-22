package performance

import (
	"math"
	"strings"
	"time"
)

const (
	defaultInterval            = time.Hour
	defaultWindow              = 90 * 24 * time.Hour
	defaultRetention           = 90 * 24 * time.Hour
	defaultFormulaVersion      = "v6.0"
	defaultCoverageGate        = 0.70
	defaultMinimumSamples      = 5
	defaultMinimumExposureDays = 30
	defaultBusyRetries         = 3
	defaultBusyRetryDelay      = 200 * time.Millisecond
)

const (
	metricDemandCompletion = "D01"
	metricDemandOnTime     = "D02"
	metricDefectDensity    = "B01"
)

const (
	publicationModeShadow = "shadow"
	publicationModeFormal = "formal"
)

const (
	SourceEventIssueSnapshot  = "jira_issue_snapshot"
	SourceEventAssigneeChange = "jira_assignee_changed"
	SourceEventStatusChange   = "jira_status_changed"
	SourceEventDueDateChange  = "jira_due_date_changed"
	SourceEventPriorityChange = "jira_priority_changed"
)

// Settings is the small configuration seam for the background module.
type Settings struct {
	Enabled             bool
	CoreMembers         []string
	Interval            time.Duration
	Window              time.Duration
	Retention           time.Duration
	FormulaVersion      string
	CoverageGate        float64
	MinimumSamples      int
	MinimumExposureDays int
	BusyRetries         int
	BusyRetryDelay      time.Duration
	PublicationMode     string
	EnableDemandMetrics bool
	EnableBugMetrics    bool
	EnableCodeMetrics   bool
	JiraHistoryEnabled  bool
	GitDedupeEnabled    bool
	FeatureFlagsSet     bool
}

func (s Settings) normalized() Settings {
	s.CoreMembers = normalizeCoreMemberValues(s.CoreMembers)
	if s.Interval <= 0 {
		s.Interval = defaultInterval
	}
	if s.Window <= 0 {
		s.Window = defaultWindow
	}
	if s.Retention <= 0 {
		s.Retention = defaultRetention
	}
	// These values are the audited v6 scorecard contract rather than tunable
	// deployment knobs. Keep legacy persisted configuration from changing the
	// meaning of a published personnel score after an upgrade.
	s.FormulaVersion = defaultFormulaVersion
	s.CoverageGate = defaultCoverageGate
	s.MinimumSamples = defaultMinimumSamples
	s.MinimumExposureDays = defaultMinimumExposureDays
	s.PublicationMode = strings.ToLower(strings.TrimSpace(s.PublicationMode))
	if s.PublicationMode != publicationModeShadow {
		s.PublicationMode = publicationModeFormal
	}
	if !s.FeatureFlagsSet {
		s.EnableDemandMetrics = true
		s.EnableBugMetrics = true
		s.EnableCodeMetrics = true
		s.JiraHistoryEnabled = true
		s.GitDedupeEnabled = true
	}
	if s.BusyRetries <= 0 {
		s.BusyRetries = defaultBusyRetries
	}
	if s.BusyRetryDelay <= 0 {
		s.BusyRetryDelay = defaultBusyRetryDelay
	}
	return s
}

// RunResult reports durable effects of one run without exposing calculation internals.
type RunResult struct {
	RunID                   string
	SnapshotCount           int
	DeletedRunCount         int64
	DeletedSnapshotCount    int64
	DeletedEvidenceCount    int64
	DeletedAuditCount       int64
	DeletedSourceEventCount int64
}

const (
	evidenceActionObserve = "observe"
	evidenceActionVoid    = "void"

	evidenceDefectExposure        = "defect_exposure"
	evidenceDefectAttribution     = "defect_attribution"
	evidenceBugReopenOutcome      = "bug_reopen_outcome"
	evidenceRollbackOutcome       = "change_rollback_outcome"
	evidenceChangeSafetyOutcome   = "change_safety_outcome"
	evidencePredictabilityOutcome = "predictability_outcome"
	evidenceVerifiedImprovement   = "verified_improvement"
	evidenceTrendAdjustment       = "trend_adjustment"
	evidenceTrustRiskAdjustment   = "trust_risk_adjustment"
	evidenceLeverageAdjustment    = "leverage_adjustment"
)

// EvidenceCommand appends one auditable revision to the formal evidence ledger.
// A void command only needs EvidenceKey, Action, SourceSystem, SourceRecordID,
// Payload, and CreatedBy; immutable fact identity is copied from the prior row.
type EvidenceCommand struct {
	EvidenceKey         string         `json:"evidence_key"`
	Action              string         `json:"action"`
	EventType           string         `json:"event_type"`
	SubjectKey          string         `json:"subject_key"`
	WorkItemID          string         `json:"work_item_id"`
	ProjectKey          string         `json:"project_key"`
	Severity            string         `json:"severity"`
	EscapeStage         string         `json:"escape_stage"`
	ReleaseMethod       string         `json:"release_method"`
	RollbackImpact      string         `json:"rollback_impact"`
	ReasonCode          string         `json:"reason_code"`
	ResponsibilityShare float64        `json:"responsibility_share"`
	Outcome             float64        `json:"outcome"`
	Weight              float64        `json:"weight"`
	OccurredAt          time.Time      `json:"occurred_at"`
	SourceSystem        string         `json:"source_system"`
	SourceRecordID      string         `json:"source_record_id"`
	Payload             map[string]any `json:"payload"`
	CreatedBy           string         `json:"-"`
}

type EvidenceResult struct {
	ID          uint   `json:"id"`
	EvidenceKey string `json:"evidence_key"`
	Revision    uint   `json:"revision"`
	Action      string `json:"action"`
}

type SourceEventCommand struct {
	DedupeKey     string
	WorkItemID    string
	ProjectKey    string
	IssueType     string
	EventType     string
	FieldName     string
	FromValue     string
	ToValue       string
	Actor         string
	OccurredAt    time.Time
	SourceSystem  string
	SourceEventID string
	Payload       map[string]any
}

type SourceEventResult struct {
	ID       uint
	Replayed bool
}

type metricResult struct {
	Code            string   `json:"code"`
	Name            string   `json:"name"`
	Weight          float64  `json:"weight"`
	Available       bool     `json:"available"`
	SampleQualified bool     `json:"sample_qualified"`
	Direction       string   `json:"direction"`
	MinimumSamples  int      `json:"minimum_samples"`
	RawRatio        *float64 `json:"raw_ratio,omitempty"`
	PointLevel      *int     `json:"point_level,omitempty"`
	Score           *float64 `json:"score,omitempty"`
	WeightedPoints  *float64 `json:"weighted_points,omitempty"`
	EvidenceCount   int      `json:"evidence_count"`
	EvidenceRefs    []string `json:"evidence_refs,omitempty"`
	Reason          string   `json:"reason,omitempty"`
	Numerator       *float64 `json:"numerator,omitempty"`
	Denominator     *float64 `json:"denominator,omitempty"`
	RawUnit         string   `json:"raw_unit,omitempty"`
}

type itemFactor struct {
	WorkItemID                string   `json:"work_item_id"`
	OriginWorkItemID          string   `json:"origin_work_item_id,omitempty"`
	Kind                      string   `json:"kind"`
	ProjectKey                string   `json:"project_key,omitempty"`
	SizePoints                float64  `json:"size_points,omitempty"`
	PriorityFactor            float64  `json:"priority_factor,omitempty"`
	DemandLevelFactor         float64  `json:"demand_level_factor,omitempty"`
	ProjectFactor             float64  `json:"project_factor,omitempty"`
	ComplexityFactor          float64  `json:"complexity_factor,omitempty"`
	StageFactor               float64  `json:"stage_factor,omitempty"`
	RoleFactor                float64  `json:"role_factor,omitempty"`
	ResponsibilityShare       float64  `json:"responsibility_share,omitempty"`
	DeliveryWeight            float64  `json:"delivery_weight,omitempty"`
	ContributionWeight        float64  `json:"contribution_weight,omitempty"`
	SeverityFactor            *float64 `json:"severity_factor,omitempty"`
	EscapeFactor              *float64 `json:"escape_factor,omitempty"`
	ReleaseMethodFactor       *float64 `json:"release_method_factor,omitempty"`
	RollbackImpactFactor      *float64 `json:"rollback_impact_factor,omitempty"`
	DefectResponsibilityShare *float64 `json:"defect_responsibility_share,omitempty"`
	BugLoss                   *float64 `json:"bug_loss,omitempty"`
	ClosureMultiplier         *float64 `json:"closure_multiplier,omitempty"`
	FixContributionShare      float64  `json:"fix_contribution_share,omitempty"`
	OwnershipSegmentHours     float64  `json:"ownership_segment_hours,omitempty"`
	Warnings                  []string `json:"warnings,omitempty"`
}

type weightedEvidence struct {
	ref    string
	weight float64
	value  float64
}

type adjustmentResult struct {
	Type         string   `json:"type"`
	Name         string   `json:"name,omitempty"`
	ReasonCode   string   `json:"reason_code"`
	Value        float64  `json:"value"`
	RawRatio     *float64 `json:"raw_ratio,omitempty"`
	Numerator    *float64 `json:"numerator,omitempty"`
	Denominator  *float64 `json:"denominator,omitempty"`
	RawUnit      string   `json:"raw_unit,omitempty"`
	EvidenceRef  string   `json:"evidence_ref,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	Reason       string   `json:"reason,omitempty"`
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func clamp(value, minimum, maximum float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func levelFor(score float64) string {
	switch {
	case score >= 85:
		return "显著超出预期"
	case score >= 75:
		return "超出预期"
	case score >= 60:
		return "达到预期"
	case score >= 45:
		return "部分达到预期"
	default:
		return "未达到预期"
	}
}
