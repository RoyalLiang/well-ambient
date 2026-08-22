package performance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"well-ambient/internal/db"

	"gorm.io/gorm"
)

var supportedEvidenceTypes = map[string]struct{}{
	evidenceDefectExposure:        {},
	evidenceDefectAttribution:     {},
	evidenceBugReopenOutcome:      {},
	evidenceRollbackOutcome:       {},
	evidenceChangeSafetyOutcome:   {},
	evidencePredictabilityOutcome: {},
	evidenceVerifiedImprovement:   {},
	evidenceTrendAdjustment:       {},
	evidenceTrustRiskAdjustment:   {},
	evidenceLeverageAdjustment:    {},
}

// AppendEvidence records a new immutable evidence revision. It deliberately
// does not trigger a score run; the silent scheduler consumes it at the next
// input watermark.
func (m *Module) AppendEvidence(ctx context.Context, command EvidenceCommand) (EvidenceResult, error) {
	if m == nil || m.db == nil {
		return EvidenceResult{}, errors.New("performance module database is not configured")
	}
	command = normalizeEvidenceCommand(command)
	if err := validateEvidenceCommand(command); err != nil {
		return EvidenceResult{}, err
	}

	var result EvidenceResult
	err := m.withBusyRetry(ctx, func() error {
		return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var previous db.PerformanceEvidenceFact
			queryErr := tx.Where("evidence_key = ?", command.EvidenceKey).
				Order("revision DESC").First(&previous).Error
			if queryErr != nil && !errors.Is(queryErr, gorm.ErrRecordNotFound) {
				return queryErr
			}

			if command.Action == evidenceActionVoid {
				if errors.Is(queryErr, gorm.ErrRecordNotFound) {
					return fmt.Errorf("cannot void unknown evidence key %q", command.EvidenceKey)
				}
				if previous.Action == evidenceActionVoid {
					return fmt.Errorf("evidence key %q is already void", command.EvidenceKey)
				}
				command = copyEvidenceIdentityForVoid(command, previous)
			}

			revision := uint(1)
			if queryErr == nil {
				revision = previous.Revision + 1
			}
			payloadJSON, payloadHash, err := encodeEvidencePayload(command, revision)
			if err != nil {
				return err
			}
			now := m.now().UTC()
			fact := db.PerformanceEvidenceFact{
				EvidenceKey: command.EvidenceKey, Revision: revision, Action: command.Action,
				EventType: command.EventType, SubjectKey: command.SubjectKey,
				WorkItemID: command.WorkItemID, ProjectKey: command.ProjectKey,
				Severity: command.Severity, EscapeStage: command.EscapeStage,
				ReleaseMethod: command.ReleaseMethod, RollbackImpact: command.RollbackImpact,
				ReasonCode:          command.ReasonCode,
				ResponsibilityShare: command.ResponsibilityShare,
				Outcome:             command.Outcome, Weight: command.Weight, OccurredAt: command.OccurredAt.UTC(),
				ObservedAt: now, SourceSystem: command.SourceSystem, SourceRecordID: command.SourceRecordID,
				EvidenceRef: command.SourceSystem + ":" + command.SourceRecordID,
				PayloadJSON: payloadJSON, PayloadHash: payloadHash,
				CreatedBy: command.CreatedBy, CreatedAt: now,
			}
			if err := tx.Create(&fact).Error; err != nil {
				return err
			}
			if err := createAudit(tx, db.PerformanceAuditEvent{
				RunID:     fmt.Sprintf("evidence:%s:%d", fact.EvidenceKey, fact.Revision),
				EventType: "evidence_recorded", SubjectKey: fact.SubjectKey,
				RecordType: "performance_evidence_fact", RecordID: fact.ID,
				FormulaVersion: m.currentSettings().FormulaVersion, CreatedAt: now,
			}, map[string]any{
				"evidence_key": fact.EvidenceKey,
				"revision":     fact.Revision,
				"action":       fact.Action,
				"event_type":   fact.EventType,
				"evidence_ref": fact.EvidenceRef,
				"payload_hash": fact.PayloadHash,
				"created_by":   fact.CreatedBy,
			}); err != nil {
				return err
			}
			result = EvidenceResult{ID: fact.ID, EvidenceKey: fact.EvidenceKey, Revision: fact.Revision, Action: fact.Action}
			return nil
		})
	})
	return result, err
}

func normalizeEvidenceCommand(command EvidenceCommand) EvidenceCommand {
	command.EvidenceKey = strings.TrimSpace(command.EvidenceKey)
	command.Action = strings.ToLower(strings.TrimSpace(command.Action))
	if command.Action == "" {
		command.Action = evidenceActionObserve
	}
	command.EventType = strings.ToLower(strings.TrimSpace(command.EventType))
	command.SubjectKey = strings.TrimSpace(command.SubjectKey)
	command.WorkItemID = strings.TrimSpace(command.WorkItemID)
	command.ProjectKey = strings.TrimSpace(command.ProjectKey)
	command.Severity = strings.ToUpper(strings.TrimSpace(command.Severity))
	command.EscapeStage = strings.ToLower(strings.TrimSpace(command.EscapeStage))
	command.ReleaseMethod = strings.TrimSpace(command.ReleaseMethod)
	command.RollbackImpact = strings.TrimSpace(command.RollbackImpact)
	command.ReasonCode = strings.ToUpper(strings.TrimSpace(command.ReasonCode))
	command.SourceSystem = strings.TrimSpace(command.SourceSystem)
	command.SourceRecordID = strings.TrimSpace(command.SourceRecordID)
	command.CreatedBy = strings.TrimSpace(command.CreatedBy)
	if command.Weight == 0 {
		command.Weight = 1
	}
	if command.EventType == evidenceRollbackOutcome || command.EventType == evidenceChangeSafetyOutcome {
		if factor, ok := releaseMethodFactor(command.ReleaseMethod); ok {
			command.Weight = factor
		}
	}
	if command.Payload == nil {
		command.Payload = map[string]any{}
	}
	return command
}

func validateEvidenceCommand(command EvidenceCommand) error {
	if command.EvidenceKey == "" {
		return errors.New("evidence_key is required")
	}
	if command.Action != evidenceActionObserve && command.Action != evidenceActionVoid {
		return errors.New("action must be observe or void")
	}
	if command.SourceSystem == "" || command.SourceRecordID == "" {
		return errors.New("source_system and source_record_id are required")
	}
	if command.CreatedBy == "" {
		return errors.New("created_by is required")
	}
	if command.Action == evidenceActionVoid {
		return nil
	}
	if _, ok := supportedEvidenceTypes[command.EventType]; !ok {
		return fmt.Errorf("unsupported event_type %q", command.EventType)
	}
	if command.SubjectKey == "" {
		return errors.New("subject_key is required")
	}
	if command.OccurredAt.IsZero() {
		return errors.New("occurred_at is required")
	}
	if command.Weight <= 0 {
		return errors.New("weight must be greater than zero")
	}
	if command.EventType == evidenceDefectAttribution {
		if _, ok := responsibilityFactor(command.ResponsibilityShare); !ok || command.ResponsibilityShare == 0 {
			return errors.New("defect responsibility_share must match v4 confirmed (1) or shared (0.5) attribution")
		}
		if _, ok := bugLoss(command.Severity, command.EscapeStage, command.ResponsibilityShare); !ok {
			return errors.New("defect attribution requires a supported severity and escape_stage")
		}
	}
	if command.EventType == evidenceBugReopenOutcome || command.EventType == evidenceRollbackOutcome || command.EventType == evidenceChangeSafetyOutcome || command.EventType == evidencePredictabilityOutcome || command.EventType == evidenceVerifiedImprovement {
		if command.Outcome < 0 || command.Outcome > 1 {
			return errors.New("outcome must be between zero and one")
		}
	}
	if command.EventType == evidenceRollbackOutcome || command.EventType == evidenceChangeSafetyOutcome {
		if _, ok := releaseMethodFactor(command.ReleaseMethod); !ok {
			return errors.New("release_method must match v4 full, canary, or hotfix")
		}
	}
	if command.EventType == evidenceRollbackOutcome && command.Outcome < 1 {
		if _, ok := rollbackImpactFactor(command.RollbackImpact); !ok {
			return errors.New("rollback_impact must match v4 full, partial, or config/single-node")
		}
		if _, ok := responsibilityFactor(command.ResponsibilityShare); !ok {
			return errors.New("rollback responsibility_share must match v4 confirmed (1), shared (0.5), or unattributable/disputed (0)")
		}
	}
	if command.EventType == evidenceTrendAdjustment || command.EventType == evidenceTrustRiskAdjustment || command.EventType == evidenceLeverageAdjustment {
		value, ok := adjustmentValue(command.ReasonCode)
		if !ok {
			return errors.New("reason_code is not defined by the v4 assessment table")
		}
		if command.EventType == evidenceTrendAdjustment && (value < -5 || value > 5 || !strings.HasPrefix(command.ReasonCode, "TREND_")) {
			return errors.New("trend adjustment requires a TREND_* reason_code")
		}
		if command.EventType == evidenceTrustRiskAdjustment && (value >= 0 || !strings.HasPrefix(command.ReasonCode, "TRUST_")) {
			return errors.New("trust risk adjustment requires a TRUST_* reason_code")
		}
		if command.EventType == evidenceLeverageAdjustment && (value <= 0 || !strings.HasPrefix(command.ReasonCode, "LEV_")) {
			return errors.New("leverage adjustment requires a LEV_* reason_code")
		}
	}
	return nil
}

func copyEvidenceIdentityForVoid(command EvidenceCommand, previous db.PerformanceEvidenceFact) EvidenceCommand {
	command.EventType = previous.EventType
	command.SubjectKey = previous.SubjectKey
	command.WorkItemID = previous.WorkItemID
	command.ProjectKey = previous.ProjectKey
	command.Severity = previous.Severity
	command.EscapeStage = previous.EscapeStage
	command.ReleaseMethod = previous.ReleaseMethod
	command.RollbackImpact = previous.RollbackImpact
	command.ReasonCode = previous.ReasonCode
	command.ResponsibilityShare = previous.ResponsibilityShare
	command.Outcome = previous.Outcome
	command.Weight = previous.Weight
	command.OccurredAt = previous.OccurredAt
	return command
}

func encodeEvidencePayload(command EvidenceCommand, revision uint) (string, string, error) {
	encoded, err := json.Marshal(struct {
		Revision            uint           `json:"revision"`
		Action              string         `json:"action"`
		EventType           string         `json:"event_type"`
		SubjectKey          string         `json:"subject_key"`
		WorkItemID          string         `json:"work_item_id,omitempty"`
		ProjectKey          string         `json:"project_key,omitempty"`
		Severity            string         `json:"severity,omitempty"`
		EscapeStage         string         `json:"escape_stage,omitempty"`
		ReleaseMethod       string         `json:"release_method,omitempty"`
		RollbackImpact      string         `json:"rollback_impact,omitempty"`
		ReasonCode          string         `json:"reason_code,omitempty"`
		ResponsibilityShare float64        `json:"responsibility_share,omitempty"`
		Outcome             float64        `json:"outcome"`
		Weight              float64        `json:"weight"`
		OccurredAt          time.Time      `json:"occurred_at"`
		SourceSystem        string         `json:"source_system"`
		SourceRecordID      string         `json:"source_record_id"`
		Payload             map[string]any `json:"payload"`
	}{
		Revision: revision, Action: command.Action, EventType: command.EventType,
		SubjectKey: command.SubjectKey, WorkItemID: command.WorkItemID, ProjectKey: command.ProjectKey,
		Severity: command.Severity, EscapeStage: command.EscapeStage,
		ReleaseMethod: command.ReleaseMethod, RollbackImpact: command.RollbackImpact, ReasonCode: command.ReasonCode,
		ResponsibilityShare: command.ResponsibilityShare, Outcome: command.Outcome,
		Weight: command.Weight, OccurredAt: command.OccurredAt.UTC(),
		SourceSystem: command.SourceSystem, SourceRecordID: command.SourceRecordID, Payload: command.Payload,
	})
	if err != nil {
		return "", "", err
	}
	digest := sha256.Sum256(encoded)
	return string(encoded), hex.EncodeToString(digest[:]), nil
}

func loadActiveEvidenceFacts(tx *gorm.DB, watermark time.Time) ([]db.PerformanceEvidenceFact, error) {
	var revisions []db.PerformanceEvidenceFact
	if err := tx.Where("created_at <= ?", watermark).
		Order("evidence_key ASC, revision ASC").Find(&revisions).Error; err != nil {
		return nil, err
	}
	latest := make(map[string]db.PerformanceEvidenceFact, len(revisions))
	for _, revision := range revisions {
		latest[revision.EvidenceKey] = revision
	}
	keys := make([]string, 0, len(latest))
	for key, fact := range latest {
		if fact.Action != evidenceActionVoid {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	result := make([]db.PerformanceEvidenceFact, 0, len(keys))
	for _, key := range keys {
		result = append(result, latest[key])
	}
	return result, nil
}
