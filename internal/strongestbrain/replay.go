package strongestbrain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
	"well-ambient/internal/agentruntime"
	"well-ambient/internal/db"
)

// ReplayCommand requests an offline evaluation of a candidate version vs baseline.
type ReplayCommand struct {
	CandidateVersionID uint   `json:"candidate_version_id"`
	BaselineVersionID  uint   `json:"baseline_version_id"`
	DatasetRef         string `json:"dataset_ref"` // e.g. "historical-reviews-q3"
	SampleRunIDs       []uint `json:"sample_run_ids,omitempty"`
}

// ReplayEngine checks manifests only; it does not execute historical samples.
type ReplayEngine struct {
	db       *gorm.DB
	registry *agentruntime.Registry
}

// NewReplayEngine initializes a replay evaluation engine.
func NewReplayEngine(database *gorm.DB, registry *agentruntime.Registry) *ReplayEngine {
	return &ReplayEngine{
		db:       database,
		registry: registry,
	}
}

// Evaluate validates declared permissions without claiming an executed replay.
func (re *ReplayEngine) Evaluate(ctx context.Context, cmd ReplayCommand) (*db.CapabilityEvaluation, error) {
	if cmd.CandidateVersionID == 0 || cmd.BaselineVersionID == 0 {
		return nil, fmt.Errorf("candidate and baseline version IDs are required for manifest checks")
	}
	if cmd.DatasetRef == "" {
		cmd.DatasetRef = "standard-regression-suite"
	}

	var candidateVer db.CapabilityVersion
	if err := re.db.WithContext(ctx).First(&candidateVer, cmd.CandidateVersionID).Error; err != nil {
		return nil, fmt.Errorf("candidate version %d not found: %w", cmd.CandidateVersionID, err)
	}
	var baselineVer db.CapabilityVersion
	if err := re.db.WithContext(ctx).First(&baselineVer, cmd.BaselineVersionID).Error; err != nil {
		return nil, fmt.Errorf("baseline version %d not found: %w", cmd.BaselineVersionID, err)
	}

	var candidateManifest agentruntime.CapabilityManifest
	if err := json.Unmarshal([]byte(candidateVer.ManifestJSON), &candidateManifest); err != nil {
		return nil, fmt.Errorf("failed to parse candidate manifest: %w", err)
	}
	if err := candidateManifest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid candidate manifest: %w", err)
	}
	var baselineManifest agentruntime.CapabilityManifest
	if err := json.Unmarshal([]byte(baselineVer.ManifestJSON), &baselineManifest); err != nil {
		return nil, fmt.Errorf("failed to parse baseline manifest: %w", err)
	}
	if err := baselineManifest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid baseline manifest: %w", err)
	}
	if candidateVer.CapabilityID != baselineVer.CapabilityID || candidateManifest.ID != baselineManifest.ID {
		return nil, fmt.Errorf("baseline and candidate must belong to the same capability")
	}

	// An empty baseline permission set still prohibits every new permission.
	baselinePermissions := make(map[string]bool, len(baselineManifest.Permissions))
	for _, permission := range baselineManifest.Permissions {
		baselinePermissions[permission] = true
	}
	permissionExpanded := false
	for _, permission := range candidateManifest.Permissions {
		if !baselinePermissions[permission] {
			permissionExpanded = true
			break
		}
	}

	verdict := "manual_review"
	regressions := []string{}
	if permissionExpanded {
		verdict = "reject"
		regressions = append(regressions, "硬门禁违规：候选版本要求了超出基线权限集合的权限（禁止权限静默扩大）")
	}
	regressionsBytes, _ := json.Marshal(regressions)
	metricsBytes, _ := json.Marshal(map[string]any{
		"candidate_digest":    candidateVer.ContentDigest,
		"baseline_digest":     baselineVer.ContentDigest,
		"dataset_ref":         cmd.DatasetRef,
		"permission_expanded": permissionExpanded,
	})
	evaluation := db.CapabilityEvaluation{
		CandidateVersionID: cmd.CandidateVersionID,
		BaselineVersionID:  cmd.BaselineVersionID,
		DatasetRef:         cmd.DatasetRef,
		Verdict:            verdict,
		RegressionsJSON:    string(regressionsBytes),
		MetricsJSON:        string(metricsBytes),
		CreatedAt:          time.Now(),
	}
	markManifestChecksOnly(&evaluation, false)
	if err := re.db.WithContext(ctx).Create(&evaluation).Error; err != nil {
		return nil, fmt.Errorf("failed to save capability evaluation: %w", err)
	}
	return &evaluation, nil
}

// markManifestChecksOnly annotates the actual evaluation boundary. Zero-valued
// score fields preserve the existing DTO shape; metrics_available is authoritative.
// Historical results are annotated in memory without rewriting stored records.
func markManifestChecksOnly(evaluation *db.CapabilityEvaluation, readingStored bool) {
	var metrics map[string]any
	_ = json.Unmarshal([]byte(evaluation.MetricsJSON), &metrics)
	if metrics == nil {
		metrics = make(map[string]any)
	}
	if readingStored && metrics["evaluation_kind"] != "manifest_checks_only" {
		metrics["legacy_static_result"] = true
		metrics["stored_verdict"] = evaluation.Verdict
	}
	delete(metrics, "weights")
	metrics["evaluation_kind"] = "manifest_checks_only"
	metrics["metrics_available"] = false
	metrics["evaluated_sample_count"] = 0
	metrics["evaluation_note"] = "Historical samples were not executed; manifest checks do not establish replay quality or performance."
	metricsBytes, _ := json.Marshal(metrics)
	evaluation.MetricsJSON = string(metricsBytes)
	if evaluation.Verdict == "pass" {
		evaluation.Verdict = "manual_review"
	}
	evaluation.QualityScore = 0
	evaluation.EvidenceScore = 0
	evaluation.SafetyScore = 0
	evaluation.StabilityScore = 0
	evaluation.TokenEfficiencyScore = 0
	evaluation.LatencyScore = 0
	evaluation.OverallScore = 0
}
