package performance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"well-ambient/internal/db"

	"gorm.io/gorm"
)

const (
	runStatusCompleted = "completed"
	runStatusFailed    = "failed"
)

// Module owns the complete scoring, audit, scheduling, and retention boundary.
type Module struct {
	db         *gorm.DB
	settingsMu sync.RWMutex
	settings   Settings
	now        func() time.Time

	lifecycleMu     sync.Mutex
	parentContext   context.Context
	runMu           sync.Mutex
	stateMu         sync.Mutex
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	started         bool
	running         bool
	lastStartedAt   *time.Time
	lastCompletedAt *time.Time
	lastError       string
	serial          atomic.Uint64
}

func New(conn *gorm.DB, settings Settings) *Module {
	return &Module{db: conn, settings: settings.normalized(), now: time.Now}
}

// Start begins the silent background loop. The first run happens immediately;
// later runs are serialized on the configured interval.
func (m *Module) Start(parent context.Context) bool {
	if m == nil || m.db == nil {
		return false
	}
	if parent == nil {
		parent = context.Background()
	}
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	m.parentContext = parent
	return m.startLocked(parent)
}

func (m *Module) startLocked(parent context.Context) bool {
	settings := m.currentSettings()
	if !settings.Enabled {
		return false
	}
	m.stateMu.Lock()
	if m.started {
		m.stateMu.Unlock()
		return false
	}
	ctx, cancel := context.WithCancel(parent)
	m.cancel = cancel
	m.started = true
	m.wg.Add(1)
	m.stateMu.Unlock()

	go func() {
		defer m.wg.Done()
		defer func() {
			m.stateMu.Lock()
			m.started = false
			m.cancel = nil
			m.stateMu.Unlock()
		}()
		_, _ = m.RunOnce(ctx, "startup")
		ticker := time.NewTicker(settings.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = m.RunOnce(ctx, "schedule")
			}
		}
	}()
	return true
}

// Stop cancels the background loop and waits until an in-flight run exits.
func (m *Module) Stop() {
	if m == nil {
		return
	}
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	m.parentContext = nil
	m.stopLocked()
}

func (m *Module) stopLocked() {
	m.stateMu.Lock()
	cancel := m.cancel
	m.stateMu.Unlock()
	if cancel != nil {
		cancel()
	}
	m.wg.Wait()
}

// Reconfigure atomically replaces the runner settings. When Start has already
// registered a server lifecycle context, the old loop is stopped and the new
// enabled state takes effect immediately.
func (m *Module) Reconfigure(settings Settings) bool {
	if m == nil {
		return false
	}
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	m.stopLocked()
	m.settingsMu.Lock()
	m.settings = settings.normalized()
	m.settingsMu.Unlock()
	if m.parentContext == nil {
		return false
	}
	return m.startLocked(m.parentContext)
}

func (m *Module) currentSettings() Settings {
	m.settingsMu.RLock()
	defer m.settingsMu.RUnlock()
	settings := m.settings
	settings.CoreMembers = append([]string(nil), m.settings.CoreMembers...)
	return settings.normalized()
}

// RunOnce creates a new immutable set of snapshots and applies retention in one
// transaction. It never updates a previous score in place.
func (m *Module) RunOnce(ctx context.Context, trigger string) (result RunResult, runErr error) {
	if m == nil || m.db == nil {
		return RunResult{}, errors.New("performance module database is not configured")
	}
	m.runMu.Lock()
	defer m.runMu.Unlock()

	settings := m.currentSettings()
	now := m.now().UTC()
	windowStart := now.Add(-settings.Window)
	retentionCutoff := now.Add(-settings.Retention)
	runID := m.newRunID(now)
	trigger = normalizeTrigger(trigger)
	result = RunResult{RunID: runID}
	m.markRunStarted(now)
	defer func() {
		m.markRunFinished(m.now().UTC(), runErr)
	}()

	if err := m.withBusyRetry(ctx, func() error {
		return m.appendAudit(ctx, db.PerformanceAuditEvent{
			RunID: runID, EventType: "run_started", RecordType: "performance_score_run",
			FormulaVersion: settings.FormulaVersion, CreatedAt: now,
		}, map[string]any{
			"trigger": trigger, "assessment_window_start": windowStart,
			"assessment_window_end": now, "input_watermark": now,
		})
	}); err != nil {
		return result, err
	}

	err := m.withBusyRetry(ctx, func() error {
		return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			evidence, err := loadSubjectEvidence(tx, windowStart, now, settings.CoreMembers)
			if err != nil {
				return err
			}
			calculations := make([]subjectCalculation, 0, len(evidence))
			for _, subject := range evidence {
				calculation, err := calculateSubject(subject, settings, windowStart, now)
				if err != nil {
					return err
				}
				calculations = append(calculations, calculation)
			}

			deletedSnapshots := tx.Where("created_at < ?", retentionCutoff).Delete(&db.PerformanceScoreSnapshot{})
			if deletedSnapshots.Error != nil {
				return deletedSnapshots.Error
			}
			deletedRuns := tx.Where("created_at < ?", retentionCutoff).Delete(&db.PerformanceScoreRun{})
			if deletedRuns.Error != nil {
				return deletedRuns.Error
			}
			deletedEvidence := tx.Where("created_at < ?", retentionCutoff).Delete(&db.PerformanceEvidenceFact{})
			if deletedEvidence.Error != nil {
				return deletedEvidence.Error
			}
			deletedAudits := tx.Where("created_at < ?", retentionCutoff).Delete(&db.PerformanceAuditEvent{})
			if deletedAudits.Error != nil {
				return deletedAudits.Error
			}
			deletedSourceEvents := tx.Where("created_at < ?", retentionCutoff).Delete(&db.PerformanceWorkItemEvent{})
			if deletedSourceEvents.Error != nil {
				return deletedSourceEvents.Error
			}

			completedAt := now
			run := db.PerformanceScoreRun{
				RunID: runID, Trigger: trigger, Status: runStatusCompleted,
				FormulaVersion:        settings.FormulaVersion,
				AssessmentWindowStart: windowStart, AssessmentWindowEnd: now, InputWatermark: now,
				SnapshotCount: len(calculations), DeletedRunCount: deletedRuns.RowsAffected,
				DeletedSnapshotCount: deletedSnapshots.RowsAffected, DeletedEvidenceCount: deletedEvidence.RowsAffected,
				DeletedAuditCount:       deletedAudits.RowsAffected,
				DeletedSourceEventCount: deletedSourceEvents.RowsAffected,
				RetentionCutoff:         retentionCutoff, CompletedAt: &completedAt, CreatedAt: now,
			}
			if err := tx.Create(&run).Error; err != nil {
				return err
			}

			retainedUntil := now.Add(settings.Retention)
			for _, calculation := range calculations {
				snapshot := db.PerformanceScoreSnapshot{
					RunID: runID, SubjectKey: calculation.SubjectKey, FormulaVersion: settings.FormulaVersion,
					AssessmentWindowStart: windowStart, AssessmentWindowEnd: now, InputWatermark: now,
					InputDigest:       calculation.InputDigest,
					DeliveryUnitCount: calculation.DeliveryUnitCount, BugCount: calculation.BugCount,
					DemandDelayCount: calculation.DemandDelayCount, BugReopenCount: calculation.BugReopenCount,
					CommitCount: calculation.CommitCount, DuplicateCommitCount: calculation.DuplicateCommitCount,
					EffectiveSampleCount: calculation.EffectiveSampleCount,
					ExposureDays:         calculation.ExposureDays,
					EvidenceCoverage:     calculation.EvidenceCoverage,
					ObservedScore:        calculation.ObservedScore, FinalScore: calculation.FinalScore,
					TrendAdjustment: calculation.TrendAdjustment, RiskPenalty: calculation.RiskPenalty,
					LeverageBonus: calculation.LeverageBonus,
					RatingStatus:  calculation.RatingStatus, Level: calculation.Level,
					MetricsJSON: calculation.MetricsJSON, ItemFactorsJSON: calculation.ItemFactorsJSON,
					ExclusionsJSON:  calculation.ExclusionsJSON,
					AdjustmentsJSON: calculation.AdjustmentsJSON,
					CreatedAt:       now, RetainedUntil: &retainedUntil,
				}
				if err := tx.Create(&snapshot).Error; err != nil {
					return err
				}
				if err := createAudit(tx, db.PerformanceAuditEvent{
					RunID: runID, EventType: "snapshot_created", SubjectKey: calculation.SubjectKey,
					RecordType: "performance_score_snapshot", RecordID: snapshot.ID,
					FormulaVersion: settings.FormulaVersion, CreatedAt: now,
				}, map[string]any{
					"input_digest":           calculation.InputDigest,
					"evidence_coverage":      calculation.EvidenceCoverage,
					"effective_sample_count": calculation.EffectiveSampleCount,
					"exposure_days":          calculation.ExposureDays,
					"rating_status":          calculation.RatingStatus,
					"trend_adjustment":       calculation.TrendAdjustment,
					"risk_penalty":           calculation.RiskPenalty,
					"leverage_bonus":         calculation.LeverageBonus,
				}); err != nil {
					return err
				}
			}

			if err := createAudit(tx, db.PerformanceAuditEvent{
				RunID: runID, EventType: "retention_applied", RecordType: "retention_policy",
				FormulaVersion: settings.FormulaVersion, CreatedAt: now,
			}, map[string]any{
				"cutoff": retentionCutoff, "retention_days": settings.Retention.Hours() / 24,
				"deleted_runs":          deletedRuns.RowsAffected,
				"deleted_snapshots":     deletedSnapshots.RowsAffected,
				"deleted_evidence":      deletedEvidence.RowsAffected,
				"deleted_audits":        deletedAudits.RowsAffected,
				"deleted_source_events": deletedSourceEvents.RowsAffected,
			}); err != nil {
				return err
			}
			if err := createAudit(tx, db.PerformanceAuditEvent{
				RunID: runID, EventType: "run_completed", RecordType: "performance_score_run",
				RecordID: run.ID, FormulaVersion: settings.FormulaVersion, CreatedAt: now,
			}, map[string]any{"snapshot_count": len(calculations)}); err != nil {
				return err
			}

			result.SnapshotCount = len(calculations)
			result.DeletedRunCount = deletedRuns.RowsAffected
			result.DeletedSnapshotCount = deletedSnapshots.RowsAffected
			result.DeletedEvidenceCount = deletedEvidence.RowsAffected
			result.DeletedAuditCount = deletedAudits.RowsAffected
			result.DeletedSourceEventCount = deletedSourceEvents.RowsAffected
			return nil
		})
	})
	if err == nil {
		return result, nil
	}

	failureContext, cancelFailureAudit := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancelFailureAudit()
	failureErr := m.withBusyRetry(failureContext, func() error {
		return m.persistFailure(failureContext, db.PerformanceScoreRun{
			RunID: runID, Trigger: trigger, Status: runStatusFailed,
			FormulaVersion:        settings.FormulaVersion,
			AssessmentWindowStart: windowStart, AssessmentWindowEnd: now, InputWatermark: now,
			RetentionCutoff: retentionCutoff, ErrorMessage: err.Error(), CompletedAt: &now, CreatedAt: now,
		}, err)
	})
	if failureErr != nil {
		return result, errors.Join(err, failureErr)
	}
	return result, err
}

func (m *Module) markRunStarted(startedAt time.Time) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	m.running = true
	started := startedAt.UTC()
	m.lastStartedAt = &started
}

func (m *Module) markRunFinished(completedAt time.Time, err error) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	m.running = false
	completed := completedAt.UTC()
	m.lastCompletedAt = &completed
	if err == nil {
		m.lastError = ""
		return
	}
	m.lastError = err.Error()
}

func (m *Module) persistFailure(ctx context.Context, run db.PerformanceScoreRun, runErr error) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&run).Error; err != nil {
			return err
		}
		return createAudit(tx, db.PerformanceAuditEvent{
			RunID: run.RunID, EventType: "run_failed", RecordType: "performance_score_run",
			RecordID: run.ID, FormulaVersion: run.FormulaVersion, CreatedAt: run.CreatedAt,
		}, map[string]any{"error": runErr.Error()})
	})
}

func (m *Module) appendAudit(ctx context.Context, event db.PerformanceAuditEvent, payload any) error {
	return createAudit(m.db.WithContext(ctx), event, payload)
}

func createAudit(tx *gorm.DB, event db.PerformanceAuditEvent, payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	event.PayloadJSON = string(encoded)
	return tx.Create(&event).Error
}

func (m *Module) newRunID(now time.Time) string {
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err == nil {
		return fmt.Sprintf("perf-%d-%s", now.UnixNano(), hex.EncodeToString(bytes))
	}
	return fmt.Sprintf("perf-%d-%d", now.UnixNano(), m.serial.Add(1))
}

func normalizeTrigger(trigger string) string {
	switch trigger {
	case "startup", "schedule", "run_once":
		return trigger
	default:
		return "run_once"
	}
}
