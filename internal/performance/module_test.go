package performance

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var performanceTestDBSerial atomic.Uint64

func TestRunOnceAppendsAuditedSnapshotsWithoutPublishingIncompleteEvidence(t *testing.T) {
	conn := newPerformanceTestDB(t)
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	createdAt := now.Add(-48 * time.Hour)
	completedAt := now.Add(-2 * time.Hour)
	dueAt := now.Add(-time.Hour)

	if err := conn.Create(&db.ProjectConfig{
		ProjectKey: "WA", ProjectName: "Well Ambient", BasePriority: "P1", ProjectPhase: "交付",
	}).Error; err != nil {
		t.Fatal(err)
	}
	fixtures := []db.TaskTelemetry{
		{
			TaskID: "WA-101", ProjectKey: "WA", Assignee: "Alice", IssueType: "requirement", Status: "done",
			TaskCreatedAt: createdAt, LastUpdate: now.Add(-time.Hour), CompletedAt: &completedAt, DueDate: &dueAt,
			EstimateDays: 2, Difficulty: "High",
		},
		{
			TaskID: "WA-102", ProjectKey: "WA", Assignee: "Alice", IssueType: "bug", Status: "done",
			TaskCreatedAt: createdAt, LastUpdate: now.Add(-time.Hour), CompletedAt: &completedAt, DueDate: &dueAt,
		},
	}
	if err := conn.Create(&fixtures).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Create(&db.ExecutionRun{
		RunKey: "run-wa-101", DemandID: "WA-101", Status: "delivered",
		PipelineStatus: "success", AcceptanceState: "accepted", CreatedAt: now.Add(-24 * time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}

	module := New(conn, Settings{
		Enabled: true, FormulaVersion: "personnel-v1", CoverageGate: 0.80,
		CoreMembers: []string{"Alice"}, MinimumSamples: 2,
		Window: 90 * 24 * time.Hour, Retention: 90 * 24 * time.Hour,
	})
	module.now = func() time.Time { return now }

	first, err := module.RunOnce(context.Background(), "run_once")
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	second, err := module.RunOnce(context.Background(), "run_once")
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if first.RunID == second.RunID || first.SnapshotCount != 1 || second.SnapshotCount != 1 {
		t.Fatalf("unexpected run results: first=%+v second=%+v", first, second)
	}

	var runs []db.PerformanceScoreRun
	if err := conn.Order("id ASC").Find(&runs).Error; err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 || runs[0].Status != runStatusCompleted || runs[1].Status != runStatusCompleted {
		t.Fatalf("runs were not append-only completed records: %+v", runs)
	}
	var snapshots []db.PerformanceScoreSnapshot
	if err := conn.Order("id ASC").Find(&snapshots).Error; err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("snapshot count = %d, want 2", len(snapshots))
	}
	latest := snapshots[1]
	if latest.EvidenceCoverage != 0 {
		t.Fatalf("coverage = %.2f, want 0 because every available-looking metric is below its v6 sample minimum", latest.EvidenceCoverage)
	}
	if latest.ObservedScore != nil {
		t.Fatalf("reference score = %v, want nil until at least one metric reaches its sample minimum", latest.ObservedScore)
	}
	if latest.FinalScore != nil || latest.Level != "" || latest.RatingStatus != "insufficient_evidence" {
		t.Fatalf("incomplete evidence was published as a rating: %+v", latest)
	}
	duplicate := latest
	duplicate.ID = 0
	if err := conn.Create(&duplicate).Error; err == nil {
		t.Fatal("same run and subject must not create a duplicate snapshot")
	}

	var factors []itemFactor
	if err := json.Unmarshal([]byte(latest.ItemFactorsJSON), &factors); err != nil {
		t.Fatal(err)
	}
	if len(factors) != 2 || factors[0].DeliveryWeight != 2.3 || factors[0].DemandLevelFactor != 1 || factors[0].ProjectFactor != 1.15 || factors[0].ComplexityFactor != 0 || factors[0].StageFactor != 0 || factors[0].RoleFactor != 0 {
		t.Fatalf("unexpected delivery factors: %+v", factors)
	}
	bug := factors[1]
	if bug.FixContributionShare != 1 || bug.DefectResponsibilityShare != nil || bug.BugLoss != nil || !containsString(bug.Warnings, "fix_contribution_is_not_defect_responsibility") {
		t.Fatalf("bug fix contribution leaked into defect responsibility: %+v", bug)
	}

	var auditCount int64
	if err := conn.Model(&db.PerformanceAuditEvent{}).Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 8 {
		t.Fatalf("audit event count = %d, want 8", auditCount)
	}
}

func TestLoadPerformanceWorkItemEventsBatchesLargeTaskScopes(t *testing.T) {
	conn := newPerformanceTestDB(t)
	now := time.Date(2026, 8, 23, 8, 0, 0, 0, time.UTC)
	taskIDs := make([]string, 35_660)
	for index := range taskIDs {
		taskIDs[index] = "WA-" + strconv.Itoa(index+1)
	}
	want := []db.PerformanceWorkItemEvent{
		performanceSourceFixture("large-scope-first", taskIDs[0], SourceEventIssueSnapshot, "", "", now.Add(-time.Hour)),
		performanceSourceFixture("large-scope-last", taskIDs[len(taskIDs)-1], SourceEventStatusChange, "open", "done", now),
	}
	if err := conn.Create(&want).Error; err != nil {
		t.Fatal(err)
	}

	events, err := loadPerformanceWorkItemEvents(conn, taskIDs, now)
	if err != nil {
		t.Fatalf("load events across SQLite variable limit: %v", err)
	}
	if len(events) != len(want) || events[0].DedupeKey != want[0].DedupeKey || events[1].DedupeKey != want[1].DedupeKey {
		t.Fatalf("loaded events = %+v, want first and last task events", events)
	}
}

func TestRunOnceAppliesConfigurableRetentionAndAuditsDeletion(t *testing.T) {
	conn := newPerformanceTestDB(t)
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	cutoff := now.Add(-30 * 24 * time.Hour)
	old := cutoff.Add(-time.Nanosecond)

	createStoredPerformanceSet(t, conn, "old", old)
	createStoredPerformanceSet(t, conn, "boundary", cutoff)
	module := New(conn, Settings{
		Enabled: true, FormulaVersion: "personnel-v1", Window: 90 * 24 * time.Hour,
		Retention: 30 * 24 * time.Hour,
	})
	module.now = func() time.Time { return now }

	result, err := module.RunOnce(context.Background(), "schedule")
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletedRunCount != 1 || result.DeletedSnapshotCount != 1 || result.DeletedEvidenceCount != 1 || result.DeletedAuditCount != 1 || result.DeletedSourceEventCount != 1 {
		t.Fatalf("unexpected retention counts: %+v", result)
	}
	for _, model := range []any{&db.PerformanceScoreRun{}, &db.PerformanceScoreSnapshot{}, &db.PerformanceEvidenceFact{}, &db.PerformanceAuditEvent{}, &db.PerformanceWorkItemEvent{}} {
		var count int64
		if err := conn.Model(model).Where("created_at < ?", cutoff).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("expired %T rows remain: %d", model, count)
		}
	}
	var event db.PerformanceAuditEvent
	if err := conn.Where("run_id = ? AND event_type = ?", result.RunID, "retention_applied").First(&event).Error; err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["retention_days"] != float64(30) || payload["deleted_snapshots"] != float64(1) || payload["deleted_source_events"] != float64(1) {
		t.Fatalf("retention audit payload = %#v", payload)
	}
}

func TestSourceEventLedgerIsIdempotentImmutableAndConflictDetecting(t *testing.T) {
	conn := newPerformanceTestDB(t)
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	module := New(conn, Settings{Enabled: true})
	module.now = func() time.Time { return now }
	command := SourceEventCommand{
		DedupeKey: "jira:WA-220:history-1:0", WorkItemID: "WA-220", ProjectKey: "WA",
		IssueType: "bug", EventType: SourceEventAssigneeChange, FieldName: "assignee",
		FromValue: "alice.smith@example.com", ToValue: "bob", OccurredAt: now.Add(-time.Hour),
		SourceSystem: "jira", SourceEventID: "WA-220:history-1:0", Payload: map[string]any{"history_id": "history-1"},
	}
	first, err := module.AppendSourceEvent(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := module.AppendSourceEvent(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == 0 || replay.ID != first.ID || !replay.Replayed {
		t.Fatalf("source event replay = first %+v replay %+v", first, replay)
	}
	command.ToValue = "carol"
	if _, err := module.AppendSourceEvent(context.Background(), command); err == nil || !strings.Contains(err.Error(), "conflicts with retained payload") {
		t.Fatalf("conflicting source event error = %v", err)
	}
	var stored db.PerformanceWorkItemEvent
	if err := conn.First(&stored, first.ID).Error; err != nil {
		t.Fatal(err)
	}
	stored.ToValue = "mallory"
	if err := conn.Save(&stored).Error; !errors.Is(err, db.ErrImmutablePerformanceSourceEvent) {
		t.Fatalf("immutable update error = %v", err)
	}
	var sourceAuditCount int64
	if err := conn.Model(&db.PerformanceAuditEvent{}).Where("event_type = ?", "source_event_recorded").Count(&sourceAuditCount).Error; err != nil {
		t.Fatal(err)
	}
	if sourceAuditCount != 1 {
		t.Fatalf("source event audit count = %d, want 1", sourceAuditCount)
	}
}

func TestSourceEventLedgerAllowsExplicitActorDriftWithoutWeakeningPayloadConflict(t *testing.T) {
	conn := newPerformanceTestDB(t)
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	module := New(conn, Settings{Enabled: true})
	module.now = func() time.Time { return now }
	command := SourceEventCommand{
		DedupeKey: "jira:FZ-2257:2964008:0", WorkItemID: "FZ-2257", ProjectKey: "FZ",
		IssueType: "requirement", EventType: SourceEventPriorityChange, FieldName: "priority",
		FromValue: "Medium", ToValue: "Highest", Actor: "武海洋 [X]", OccurredAt: now.Add(-time.Hour),
		SourceSystem: "jira", SourceEventID: "FZ-2257:2964008:0",
		Payload: map[string]any{"field_id": "", "jira_history_id": "2964008"},
	}
	first, err := module.AppendSourceEvent(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}

	command.Actor = "武海洋"
	replay, err := module.AppendSourceEventAllowingActorDrift(context.Background(), command)
	if err != nil {
		t.Fatalf("mutable Jira author label blocked retained event replay: %v", err)
	}
	if replay.ID != first.ID || !replay.Replayed {
		t.Fatalf("actor-drift replay = first %+v replay %+v", first, replay)
	}

	command.ToValue = "Low"
	if _, err := module.AppendSourceEventAllowingActorDrift(context.Background(), command); !errors.Is(err, ErrSourceEventConflict) {
		t.Fatalf("business payload conflict was weakened: %v", err)
	}
}

func TestJiraLinkedBugIsAttributedToOriginDemandDoneOwnerNotFixer(t *testing.T) {
	conn := newPerformanceTestDB(t)
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	createdAt := now.Add(-50 * 24 * time.Hour)
	completedAt := now.Add(-40 * 24 * time.Hour)
	dueAt := completedAt.Add(24 * time.Hour)
	if err := conn.Create(&userdb.User{Username: "alice_smith", Email: "alice.smith@example.com", Name: "Alice Smith"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Create(&userdb.User{Username: "bob", Email: "bob@example.com", Name: "Bob"}).Error; err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 5; index++ {
		if err := conn.Create(&db.TaskTelemetry{
			TaskID: "WA-21" + strconv.Itoa(index), ProjectKey: "WA", Source: "jira",
			Assignee: "alice.smith@example.com", IssueType: "requirement", Status: "done",
			Priority: "P2", TaskCreatedAt: createdAt, LastUpdate: now, CompletedAt: &completedAt,
			DueDate: &dueAt, EstimateDays: 2,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	bugCompletedAt := now.Add(-5 * 24 * time.Hour)
	if err := conn.Create(&db.TaskTelemetry{
		TaskID: "WA-221", ParentWorkItemID: "WA-211", ProjectKey: "WA", Source: "jira",
		Assignee: "Bob", IssueType: "bug", Status: "done", Priority: "P1", Severity: "S2",
		TaskCreatedAt: now.Add(-10 * 24 * time.Hour), LastUpdate: now, CompletedAt: &bugCompletedAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
	events := []db.PerformanceWorkItemEvent{
		performanceSourceFixture("bug-reopened", "WA-221", SourceEventStatusChange, "Done", "In Progress", now.Add(-7*24*time.Hour)),
	}
	if err := conn.Create(&events).Error; err != nil {
		t.Fatal(err)
	}
	evidence, err := loadSubjectEvidence(conn, now.Add(-90*24*time.Hour), now, []string{"alice.smith", "bob"})
	if err != nil {
		t.Fatal(err)
	}
	bySubject := make(map[string]subjectCalculation)
	for _, subject := range evidence {
		calculation, calculationErr := calculateSubject(subject, Settings{}.normalized(), now.Add(-90*24*time.Hour), now)
		if calculationErr != nil {
			t.Fatal(calculationErr)
		}
		bySubject[subject.SubjectKey] = calculation
	}
	aliceQuality := metricResultFromSnapshot(t, bySubject["Alice Smith"].MetricsJSON, metricDefectDensity)
	bobQuality := metricResultFromSnapshot(t, bySubject["Bob"].MetricsJSON, metricDefectDensity)
	if aliceQuality.Numerator == nil || *aliceQuality.Numerator != 6 || aliceQuality.Denominator == nil || *aliceQuality.Denominator != 10 {
		t.Fatalf("origin-demand quality attribution = %+v", aliceQuality)
	}
	if bobQuality.Available {
		t.Fatalf("fixer was incorrectly treated as defect owner: %+v", bobQuality)
	}
	if bySubject["Alice Smith"].BugReopenCount != 1 || bySubject["Bob"].BugReopenCount != 1 {
		t.Fatalf("fix activity evidence counts drifted: alice=%+v bob=%+v", bySubject["Alice Smith"], bySubject["Bob"])
	}
}

func TestEvidenceLedgerAppendsRevisionsAndVoidsWithoutMutation(t *testing.T) {
	conn := newPerformanceTestDB(t)
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	module := New(conn, Settings{Enabled: true})
	module.now = func() time.Time { return now }
	base := EvidenceCommand{
		EvidenceKey: "jira:WA-201:defect", EventType: evidenceDefectAttribution,
		SubjectKey: "Alice", WorkItemID: "WA-201", ProjectKey: "WA",
		Severity: "S2", EscapeStage: "production", ResponsibilityShare: 0.5,
		OccurredAt: now.Add(-time.Hour), SourceSystem: "jira", SourceRecordID: "WA-201:1",
		Payload: map[string]any{"review": "quality-board"}, CreatedBy: "super-admin",
	}
	first, err := module.AppendEvidence(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	base.Severity = "S1"
	base.SourceRecordID = "WA-201:2"
	second, err := module.AppendEvidence(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	voided, err := module.AppendEvidence(context.Background(), EvidenceCommand{
		EvidenceKey: base.EvidenceKey, Action: evidenceActionVoid,
		SourceSystem: "manual-review", SourceRecordID: "WA-201:void",
		Payload: map[string]any{"reason": "attribution withdrawn"}, CreatedBy: "super-admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != 1 || second.Revision != 2 || voided.Revision != 3 || voided.Action != evidenceActionVoid {
		t.Fatalf("unexpected revisions: first=%+v second=%+v void=%+v", first, second, voided)
	}
	var facts []db.PerformanceEvidenceFact
	if err := conn.Order("revision ASC").Find(&facts).Error; err != nil {
		t.Fatal(err)
	}
	if len(facts) != 3 || facts[0].Severity != "S2" || facts[1].Severity != "S1" || facts[2].Action != evidenceActionVoid {
		t.Fatalf("ledger was not append-only: %+v", facts)
	}
	active, err := loadActiveEvidenceFacts(conn, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("voided evidence remains active: %+v", active)
	}
	var auditCount int64
	if err := conn.Model(&db.PerformanceAuditEvent{}).Where("event_type = ?", "evidence_recorded").Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 3 {
		t.Fatalf("evidence audit count = %d, want 3", auditCount)
	}
}

func TestV6PublishesThreeFormalCalculationsAndAppliesGitOnlyAsRisk(t *testing.T) {
	conn := newPerformanceTestDB(t)
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	createdAt := now.Add(-45 * 24 * time.Hour)
	completedAt := now.Add(-40 * 24 * time.Hour)
	dueAt := completedAt.Add(24 * time.Hour)
	for index := 1; index <= 5; index++ {
		demandID := "WA-30" + strconv.Itoa(index)
		if err := conn.Create(&db.TaskTelemetry{
			TaskID: demandID, ProjectKey: "WA", Assignee: "Alice", IssueType: "requirement", Status: "done",
			TaskCreatedAt: createdAt, LastUpdate: now.Add(-time.Hour), CompletedAt: &completedAt, DueDate: &dueAt,
			Difficulty: "High",
		}).Error; err != nil {
			t.Fatal(err)
		}
		if err := conn.Create(&db.ExecutionRun{
			RunKey: "run-" + strings.ToLower(demandID), DemandID: demandID, Status: "delivered",
			PipelineStatus: "success", AcceptanceState: "accepted", CreatedAt: now.Add(-24 * time.Hour),
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for index := 1; index <= 15; index++ {
		fingerprint := "fingerprint-" + strconv.Itoa(index)
		duplicateOf := ""
		if index > 9 {
			fingerprint = "fingerprint-" + strconv.Itoa(index-9)
			duplicateOf = "commit-1"
		}
		if err := conn.Create(&db.GitCommitLog{
			TaskID: "WA-301", Repo: "ambient", CommitID: "commit-" + strconv.Itoa(index),
			Author: "Alice", Action: "git_push", ContentFingerprint: fingerprint,
			DuplicateOfCommit: duplicateOf, TelemetryQuality: "path_message_v1", CreatedAt: now.Add(-time.Duration(16-index) * time.Hour),
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	module := New(conn, Settings{Enabled: true, CoreMembers: []string{"Alice"}, MinimumSamples: 5, CoverageGate: 0.80})
	module.now = func() time.Time { return now }
	commands := []EvidenceCommand{{EvidenceKey: "exp-301", EventType: evidenceDefectExposure, Outcome: 1}}
	for index := range commands {
		commands[index].SubjectKey = "Alice"
		if commands[index].WorkItemID == "" {
			commands[index].WorkItemID = "WA-301"
		}
		commands[index].ProjectKey = "WA"
		commands[index].OccurredAt = now.Add(-time.Hour)
		commands[index].SourceSystem = "test"
		commands[index].SourceRecordID = commands[index].EvidenceKey
		commands[index].CreatedBy = "quality-board"
		if _, err := module.AppendEvidence(context.Background(), commands[index]); err != nil {
			t.Fatalf("append %s: %v", commands[index].EvidenceKey, err)
		}
	}
	result, err := module.RunOnce(context.Background(), "run_once")
	if err != nil {
		t.Fatal(err)
	}
	var snapshot db.PerformanceScoreSnapshot
	if err := conn.Where("run_id = ? AND subject_key = ?", result.RunID, "Alice").First(&snapshot).Error; err != nil {
		t.Fatal(err)
	}
	if snapshot.EvidenceCoverage != 1 || snapshot.RatingStatus != "formal" || snapshot.FinalScore == nil || snapshot.Level == "" {
		t.Fatalf("formal evidence did not publish a rating: %+v", snapshot)
	}
	if snapshot.ObservedScore == nil || *snapshot.FinalScore != *snapshot.ObservedScore || snapshot.TrendAdjustment != 0 || snapshot.RiskPenalty != 6 || snapshot.LeverageBonus != 0 {
		t.Fatalf("v6 score or code-risk contract drifted: %+v", snapshot)
	}
	var metrics []metricResult
	if err := json.Unmarshal([]byte(snapshot.MetricsJSON), &metrics); err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 3 {
		t.Fatalf("metric count = %d, want 3", len(metrics))
	}
	for _, metric := range metrics {
		if !metric.Available {
			t.Fatalf("metric %s remains unavailable: %+v", metric.Code, metric)
		}
	}
	detail, err := module.ExplainSnapshot(context.Background(), snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !detail.ReadOnly || detail.Snapshot.ID != snapshot.ID || detail.Snapshot.SubjectKey != "Alice" {
		t.Fatalf("unexpected snapshot detail: %+v", detail)
	}
	var detailFactors []itemFactor
	if err := json.Unmarshal(detail.ItemFactors, &detailFactors); err != nil {
		t.Fatal(err)
	}
	if len(detailFactors) != 5 {
		t.Fatalf("detail factor count = %d, want 5", len(detailFactors))
	}
	var adjustments []adjustmentResult
	if err := json.Unmarshal(detail.Snapshot.Adjustments, &adjustments); err != nil || len(adjustments) != 2 || adjustments[0].Value != -6 || adjustments[1].Value != 0 {
		t.Fatalf("detail adjustments = %+v, err=%v", adjustments, err)
	}
	if strings.Contains(string(detail.Snapshot.Adjustments), `"value":-0`) {
		t.Fatalf("zero risk adjustment must not persist as negative zero: %s", detail.Snapshot.Adjustments)
	}
}

func TestRunOnceIncludesOnlyConfiguredCoreMembersAndCanonicalizesAliases(t *testing.T) {
	conn := newPerformanceTestDB(t)
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	completedAt := now.Add(-time.Hour)
	dueAt := now
	if err := conn.Create(&userdb.User{
		Username: "alice_smith", Email: "alice.smith@example.com", Name: "Alice Smith",
	}).Error; err != nil {
		t.Fatal(err)
	}
	tasks := []db.TaskTelemetry{
		{TaskID: "WA-CORE", Assignee: "Alice Smith", IssueType: "requirement", Status: "done", TaskCreatedAt: now.Add(-24 * time.Hour), LastUpdate: now, CompletedAt: &completedAt, DueDate: &dueAt, EstimateDays: 1},
		{TaskID: "WA-OUT", Assignee: "Bob", IssueType: "requirement", Status: "done", TaskCreatedAt: now.Add(-24 * time.Hour), LastUpdate: now, CompletedAt: &completedAt, DueDate: &dueAt, EstimateDays: 1},
	}
	if err := conn.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	module := New(conn, Settings{Enabled: true, CoreMembers: []string{"alice.smith"}})
	module.now = func() time.Time { return now }
	for _, command := range []EvidenceCommand{
		{EvidenceKey: "core-improvement", EventType: evidenceVerifiedImprovement, SubjectKey: "alice_smith", WorkItemID: "WA-CORE", Outcome: 1},
		{EvidenceKey: "outsider-improvement", EventType: evidenceVerifiedImprovement, SubjectKey: "Bob", WorkItemID: "WA-OUT", Outcome: 1},
	} {
		command.OccurredAt = now.Add(-time.Hour)
		command.SourceSystem = "test"
		command.SourceRecordID = command.EvidenceKey
		command.CreatedBy = "test"
		if _, err := module.AppendEvidence(context.Background(), command); err != nil {
			t.Fatal(err)
		}
	}

	result, err := module.RunOnce(context.Background(), "run_once")
	if err != nil {
		t.Fatal(err)
	}
	if result.SnapshotCount != 1 {
		t.Fatalf("snapshot count = %d, want one core member", result.SnapshotCount)
	}
	var snapshots []db.PerformanceScoreSnapshot
	if err := conn.Where("run_id = ?", result.RunID).Find(&snapshots).Error; err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 || snapshots[0].SubjectKey != "Alice Smith" || snapshots[0].DeliveryUnitCount != 1 {
		t.Fatalf("unexpected core snapshots: %+v", snapshots)
	}
	if !strings.Contains(snapshots[0].ItemFactorsJSON, "WA-CORE") || strings.Contains(snapshots[0].ItemFactorsJSON, "WA-OUT") {
		t.Fatalf("core work-item boundary leaked: %s", snapshots[0].ItemFactorsJSON)
	}
}

func TestHistoricalJiraDoneRequirementsWithoutDueProduceWeightedAcceptanceContribution(t *testing.T) {
	conn := newPerformanceTestDB(t)
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	completedAt := now.Add(-24 * time.Hour)
	tasks := make([]db.TaskTelemetry, 0, 5)
	for index := 1; index <= 5; index++ {
		key := "WA-HISTORY-" + strconv.Itoa(index)
		tasks = append(tasks, db.TaskTelemetry{
			TaskID: key, ProjectKey: "WA", Source: "jira", ExternalKey: key,
			Assignee: "Alice", IssueType: "requirement", Status: "done",
			TaskCreatedAt: now.Add(-10 * 24 * time.Hour), LastUpdate: now,
			SourceUpdatedAt: completedAt, CompletedAt: &completedAt,
		})
	}
	if err := conn.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	module := New(conn, Settings{Enabled: true, CoreMembers: []string{"Alice"}, CoverageGate: 0.8, MinimumSamples: 5})
	module.now = func() time.Time { return now }
	result, err := module.RunOnce(context.Background(), "run_once")
	if err != nil {
		t.Fatal(err)
	}
	var snapshot db.PerformanceScoreSnapshot
	if err := conn.Where("run_id = ? AND subject_key = ?", result.RunID, "Alice").First(&snapshot).Error; err != nil {
		t.Fatal(err)
	}
	if snapshot.ObservedScore == nil || *snapshot.ObservedScore != 35 || snapshot.FinalScore != nil || snapshot.EvidenceCoverage != 0.35 || snapshot.EffectiveSampleCount != 5 {
		t.Fatalf("historical Jira completions without due dates did not produce the weighted D01 contribution: %+v", snapshot)
	}
	var metrics []metricResult
	if err := json.Unmarshal([]byte(snapshot.MetricsJSON), &metrics); err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 3 || !metrics[0].Available || !metrics[0].SampleQualified || metrics[0].EvidenceCount != 5 || metrics[0].PointLevel == nil || *metrics[0].PointLevel != 5 || metrics[0].Score == nil || *metrics[0].Score != 5 || metrics[0].WeightedPoints == nil || *metrics[0].WeightedPoints != 35 || len(metrics[0].EvidenceRefs) != 5 || metrics[0].EvidenceRefs[0] != "jira:WA-HISTORY-1:completion" {
		t.Fatalf("Jira completions without due dates did not remain auditable as a qualified D01 metric: %+v", metrics)
	}
	if metrics[1].Available || metrics[1].EvidenceCount != 0 {
		t.Fatalf("requirement without a due date leaked into D02 on-time scoring: %+v", metrics[1])
	}
}

func TestV6CompletionUsesAllDueEligibleRequirementsAsDenominator(t *testing.T) {
	conn := newPerformanceTestDB(t)
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	dueAt := now.Add(-24 * time.Hour)
	completedAt := dueAt.Add(-time.Hour)
	tasks := make([]db.TaskTelemetry, 0, 5)
	for index := 1; index <= 5; index++ {
		status := "done"
		var completed *time.Time
		if index <= 4 {
			completed = &completedAt
		} else {
			status = "progress"
		}
		tasks = append(tasks, db.TaskTelemetry{
			TaskID: "WA-DUE-" + strconv.Itoa(index), ProjectKey: "WA", Source: "jira",
			Assignee: "Alice", IssueType: "requirement", Status: status,
			TaskCreatedAt: now.Add(-60 * 24 * time.Hour), LastUpdate: now,
			CompletedAt: completed, DueDate: &dueAt, EstimateDays: 2, Difficulty: "中",
		})
	}
	if err := conn.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	module := New(conn, Settings{Enabled: true, CoreMembers: []string{"Alice"}})
	module.now = func() time.Time { return now }
	result, err := module.RunOnce(context.Background(), "run_once")
	if err != nil {
		t.Fatal(err)
	}
	var snapshot db.PerformanceScoreSnapshot
	if err := conn.Where("run_id = ? AND subject_key = ?", result.RunID, "Alice").First(&snapshot).Error; err != nil {
		t.Fatal(err)
	}
	var metrics []metricResult
	if err := json.Unmarshal([]byte(snapshot.MetricsJSON), &metrics); err != nil {
		t.Fatal(err)
	}
	acceptance := metrics[0]
	if !acceptance.Available || !acceptance.SampleQualified || acceptance.EvidenceCount != 5 || acceptance.RawRatio == nil || *acceptance.RawRatio != 0.8 || acceptance.Score == nil || *acceptance.Score != 3 || acceptance.WeightedPoints == nil || *acceptance.WeightedPoints != 21 {
		t.Fatalf("D01 did not score 4 of 5 due requirements through the v6 boundary: %+v", acceptance)
	}
}

func TestReopenedJiraRequirementDoesNotReusePriorCompletionEvidence(t *testing.T) {
	conn := newPerformanceTestDB(t)
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	completedAt := now.Add(-48 * time.Hour)
	dueAt := now.Add(-24 * time.Hour)
	if err := conn.Create(&db.TaskTelemetry{
		TaskID: "WA-REOPENED", ProjectKey: "WA", Source: "jira", ExternalKey: "WA-REOPENED",
		Assignee: "Alice", IssueType: "requirement", Status: "backlog",
		TaskCreatedAt: now.Add(-10 * 24 * time.Hour), LastUpdate: now,
		SourceUpdatedAt: now, CompletedAt: &completedAt, DueDate: &dueAt, EstimateDays: 2,
	}).Error; err != nil {
		t.Fatal(err)
	}
	module := New(conn, Settings{Enabled: true, CoreMembers: []string{"Alice"}, CoverageGate: 0.8, MinimumSamples: 5})
	module.now = func() time.Time { return now }
	result, err := module.RunOnce(context.Background(), "run_once")
	if err != nil {
		t.Fatal(err)
	}
	var snapshot db.PerformanceScoreSnapshot
	if err := conn.Where("run_id = ? AND subject_key = ?", result.RunID, "Alice").First(&snapshot).Error; err != nil {
		t.Fatal(err)
	}
	if snapshot.ObservedScore != nil || snapshot.EvidenceCoverage != 0 || snapshot.EffectiveSampleCount != 1 || snapshot.FinalScore != nil {
		t.Fatalf("reopened Jira requirement bypassed the metric sample minimum: %+v", snapshot)
	}
	var metrics []metricResult
	if err := json.Unmarshal([]byte(snapshot.MetricsJSON), &metrics); err != nil {
		t.Fatal(err)
	}
	if !metrics[0].Available || metrics[0].SampleQualified || len(metrics[0].EvidenceRefs) != 1 || metrics[0].EvidenceRefs[0] != "jira:WA-REOPENED:completion" {
		t.Fatalf("reopened due item did not remain in the denominator: %+v", metrics[0])
	}
}

func TestRunOnceFailsClosedWhenCoreMemberConfigurationIsEmpty(t *testing.T) {
	conn := newPerformanceTestDB(t)
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	if err := conn.Create(&db.TaskTelemetry{
		TaskID: "WA-UNSCOPED", Assignee: "Alice", IssueType: "requirement",
		TaskCreatedAt: now.Add(-time.Hour), LastUpdate: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	module := New(conn, Settings{Enabled: true})
	module.now = func() time.Time { return now }
	result, err := module.RunOnce(context.Background(), "run_once")
	if err != nil {
		t.Fatal(err)
	}
	if result.SnapshotCount != 0 {
		t.Fatalf("unconfigured core-member boundary created %d snapshots", result.SnapshotCount)
	}
}

func TestExplainShowsOnlyCurrentCoreMembersFromPersistedSnapshots(t *testing.T) {
	conn := newPerformanceTestDB(t)
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	if err := conn.Create(&userdb.User{
		Username: "alice_smith", Email: "alice.smith@example.com", Name: "Alice Smith",
	}).Error; err != nil {
		t.Fatal(err)
	}
	createStoredPerformanceSet(t, conn, "alice_smith", now.Add(-time.Hour))
	var olderAliceSnapshot db.PerformanceScoreSnapshot
	if err := conn.Where("subject_key = ?", "alice_smith").First(&olderAliceSnapshot).Error; err != nil {
		t.Fatal(err)
	}
	latestAliceSnapshot := olderAliceSnapshot
	latestAliceSnapshot.ID = 0
	latestAliceSnapshot.RunID = "alice_smith-latest"
	latestAliceSnapshot.InputWatermark = now
	latestAliceSnapshot.InputDigest = strings.Repeat("c", 64)
	latestAliceSnapshot.CreatedAt = now
	if err := conn.Create(&latestAliceSnapshot).Error; err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 105; index++ {
		createdAt := now.Add(time.Duration(index) * time.Minute)
		createStoredPerformanceSet(t, conn, "outsider-"+strconv.Itoa(index), createdAt)
	}

	module := New(conn, Settings{Enabled: true, CoreMembers: []string{"alice.smith"}})
	module.now = func() time.Time { return now }
	explanation, err := module.Explain(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(explanation.RecentSnapshots) != 1 || explanation.RecentSnapshots[0].SubjectKey != "Alice Smith" || explanation.RecentSnapshots[0].ID != latestAliceSnapshot.ID {
		t.Fatalf("recent snapshots must contain only the latest row for each current core member: %+v", explanation.RecentSnapshots)
	}
	detail, err := module.ExplainSnapshot(context.Background(), explanation.RecentSnapshots[0].ID)
	if err != nil || detail.Snapshot.SubjectKey != "Alice Smith" {
		t.Fatalf("core snapshot detail unavailable: detail=%+v err=%v", detail, err)
	}
	historicalDetail, err := module.ExplainSnapshot(context.Background(), olderAliceSnapshot.ID)
	if err != nil || historicalDetail.Snapshot.ID != olderAliceSnapshot.ID {
		t.Fatalf("historical core snapshot was not preserved for audit: detail=%+v err=%v", historicalDetail, err)
	}
	var aliceSnapshotCount int64
	if err := conn.Model(&db.PerformanceScoreSnapshot{}).Where("subject_key = ?", "alice_smith").Count(&aliceSnapshotCount).Error; err != nil {
		t.Fatal(err)
	}
	if aliceSnapshotCount != 2 {
		t.Fatalf("core snapshot history count = %d, want 2", aliceSnapshotCount)
	}
	var outsider db.PerformanceScoreSnapshot
	if err := conn.Where("subject_key = ?", "outsider-104").First(&outsider).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := module.ExplainSnapshot(context.Background(), outsider.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("non-core snapshot detail error = %v, want record not found", err)
	}
}

func TestExplainIsReadOnlyAndReturnsPersistedContract(t *testing.T) {
	conn := newPerformanceTestDB(t)
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	module := New(conn, Settings{Enabled: true, Interval: 30 * time.Minute, Retention: 120 * 24 * time.Hour})
	module.now = func() time.Time { return now }
	if _, err := module.RunOnce(context.Background(), "run_once"); err != nil {
		t.Fatal(err)
	}
	var before int64
	if err := conn.Model(&db.PerformanceScoreRun{}).Count(&before).Error; err != nil {
		t.Fatal(err)
	}
	explanation, err := module.Explain(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	var after int64
	if err := conn.Model(&db.PerformanceScoreRun{}).Count(&after).Error; err != nil {
		t.Fatal(err)
	}
	if before != after || !explanation.ReadOnly || explanation.LatestRun == nil {
		t.Fatalf("explanation mutated calculation state: before=%d after=%d result=%+v", before, after, explanation)
	}
	if len(explanation.Metrics) != 3 || len(explanation.FactorGroups) != 7 {
		t.Fatalf("incomplete explanation contract: metrics=%d factors=%d", len(explanation.Metrics), len(explanation.FactorGroups))
	}
	if explanation.Runtime.IntervalMinutes != 30 || explanation.Runtime.RetentionDays != 120 || explanation.Runtime.MinimumExposureDays != 30 {
		t.Fatalf("runtime config missing from explanation: %+v", explanation.Runtime)
	}
}

func TestBusyRetryOnlyRetriesSQLiteContention(t *testing.T) {
	module := New(nil, Settings{BusyRetries: 3, BusyRetryDelay: time.Microsecond})
	attempts := 0
	err := module.withBusyRetry(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return errors.New("database table is locked")
		}
		return nil
	})
	if err != nil || attempts != 3 {
		t.Fatalf("busy retry failed: attempts=%d err=%v", attempts, err)
	}
	attempts = 0
	err = module.withBusyRetry(context.Background(), func() error {
		attempts++
		return errors.New("validation failed")
	})
	if err == nil || attempts != 1 {
		t.Fatalf("non-busy error was retried: attempts=%d err=%v", attempts, err)
	}
}

func TestRunOncePersistsFailureAudit(t *testing.T) {
	conn := newPerformanceTestDB(t)
	if err := conn.Migrator().DropTable(&db.TaskTelemetry{}); err != nil {
		t.Fatal(err)
	}
	module := New(conn, Settings{Enabled: true})
	module.now = func() time.Time { return time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC) }

	result, err := module.RunOnce(context.Background(), "schedule")
	if err == nil {
		t.Fatal("expected missing source table to fail the run")
	}
	var run db.PerformanceScoreRun
	if err := conn.Where("run_id = ?", result.RunID).First(&run).Error; err != nil {
		t.Fatal(err)
	}
	if run.Status != runStatusFailed || !strings.Contains(strings.ToLower(run.ErrorMessage), "table") {
		t.Fatalf("failure run not persisted: %+v", run)
	}
	var failedCount int64
	if err := conn.Model(&db.PerformanceAuditEvent{}).
		Where("run_id = ? AND event_type = ?", result.RunID, "run_failed").Count(&failedCount).Error; err != nil {
		t.Fatal(err)
	}
	if failedCount != 1 {
		t.Fatalf("run_failed audit count = %d, want 1", failedCount)
	}
}

func TestBackgroundRunnerStopsWithoutFurtherRefresh(t *testing.T) {
	conn := newPerformanceTestDB(t)
	module := New(conn, Settings{Enabled: true, Interval: 15 * time.Millisecond})
	if !module.Start(context.Background()) {
		t.Fatal("runner did not start")
	}
	deadline := time.Now().Add(time.Second)
	for {
		var count int64
		if err := conn.Model(&db.PerformanceScoreRun{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("runner did not produce two runs before deadline; count=%d", count)
		}
		time.Sleep(5 * time.Millisecond)
	}
	module.Stop()
	var stoppedCount int64
	if err := conn.Model(&db.PerformanceScoreRun{}).Count(&stoppedCount).Error; err != nil {
		t.Fatal(err)
	}
	time.Sleep(40 * time.Millisecond)
	var laterCount int64
	if err := conn.Model(&db.PerformanceScoreRun{}).Count(&laterCount).Error; err != nil {
		t.Fatal(err)
	}
	if laterCount != stoppedCount {
		t.Fatalf("runner continued after stop: before=%d after=%d", stoppedCount, laterCount)
	}
}

func TestPerformanceInitialDelayStaggersLongRunningBackgroundWork(t *testing.T) {
	if got := performanceInitialDelay(time.Hour); got != 5*time.Second {
		t.Fatalf("hourly startup delay = %s, want 5s", got)
	}
	if got := performanceInitialDelay(15 * time.Millisecond); got != 15*time.Millisecond {
		t.Fatalf("short-interval startup delay = %s, want interval", got)
	}
}

func TestReconfigureStartsAndStopsRegisteredBackgroundRunner(t *testing.T) {
	conn := newPerformanceTestDB(t)
	module := New(conn, Settings{Enabled: false, Interval: 15 * time.Millisecond})
	if module.Start(context.Background()) {
		t.Fatal("disabled runner should register lifecycle without starting")
	}
	if !module.Reconfigure(Settings{Enabled: true, Interval: 15 * time.Millisecond}) {
		t.Fatal("enabled reconfiguration did not start runner")
	}
	deadline := time.Now().Add(time.Second)
	for {
		var count int64
		if err := conn.Model(&db.PerformanceScoreRun{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("reconfigured runner did not produce a run")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if module.Reconfigure(Settings{Enabled: false}) {
		t.Fatal("disabled reconfiguration reported a started runner")
	}
	var stoppedCount int64
	if err := conn.Model(&db.PerformanceScoreRun{}).Count(&stoppedCount).Error; err != nil {
		t.Fatal(err)
	}
	time.Sleep(40 * time.Millisecond)
	var laterCount int64
	if err := conn.Model(&db.PerformanceScoreRun{}).Count(&laterCount).Error; err != nil {
		t.Fatal(err)
	}
	if laterCount != stoppedCount {
		t.Fatalf("disabled reconfiguration kept running: before=%d after=%d", stoppedCount, laterCount)
	}
	module.Stop()
}

func TestBugLossUsesSeverityEscapeAndDefectResponsibilityOnly(t *testing.T) {
	value, ok := bugLoss("S1", "production", 0.5)
	if !ok || value != 6 {
		t.Fatalf("bug loss = %.2f, %v; want 6, true", value, ok)
	}
	if _, ok := bugLoss("", "production", 1); ok {
		t.Fatal("missing severity must not silently use a default")
	}
	if _, ok := bugLoss("S1", "production", 1.1); ok {
		t.Fatal("responsibility share above one must be rejected")
	}
}

func newPerformanceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "-") + "-" +
		strconv.FormatUint(performanceTestDBSerial.Add(1), 10) + "?mode=memory&cache=shared"
	conn, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := conn.DB()
	if err != nil {
		t.Fatal(err)
	}
	// The runner and this test poll concurrently. A single in-memory SQLite
	// connection keeps this regression focused on lifecycle behavior instead of
	// testing the shared-cache connection pool's SQLITE_LOCKED semantics.
	sqlDB.SetMaxOpenConns(1)
	if err := conn.AutoMigrate(
		&db.TaskTelemetry{}, &db.ProjectConfig{}, &db.ExecutionRun{}, &db.GitCommitLog{}, &userdb.User{},
		&db.PerformanceScoreRun{}, &db.PerformanceScoreSnapshot{}, &db.PerformanceEvidenceFact{}, &db.PerformanceAuditEvent{},
		&db.PerformanceWorkItemEvent{},
	); err != nil {
		t.Fatal(err)
	}
	return conn
}

func createStoredPerformanceSet(t *testing.T, conn *gorm.DB, key string, createdAt time.Time) {
	t.Helper()
	run := db.PerformanceScoreRun{
		RunID: key, Trigger: "schedule", Status: runStatusCompleted, FormulaVersion: "personnel-v1",
		AssessmentWindowStart: createdAt.Add(-24 * time.Hour), AssessmentWindowEnd: createdAt,
		InputWatermark: createdAt, RetentionCutoff: createdAt, CreatedAt: createdAt,
	}
	if err := conn.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	snapshot := db.PerformanceScoreSnapshot{
		RunID: key, SubjectKey: key, FormulaVersion: "personnel-v1",
		AssessmentWindowStart: createdAt.Add(-24 * time.Hour), AssessmentWindowEnd: createdAt,
		InputWatermark: createdAt, InputDigest: strings.Repeat("a", 64),
		RatingStatus: "insufficient_evidence", MetricsJSON: "[]", ItemFactorsJSON: "[]", ExclusionsJSON: "[]",
		CreatedAt: createdAt,
	}
	if err := conn.Create(&snapshot).Error; err != nil {
		t.Fatal(err)
	}
	evidence := db.PerformanceEvidenceFact{
		EvidenceKey: key, Revision: 1, Action: evidenceActionObserve, EventType: evidenceDefectExposure,
		SubjectKey: key, Outcome: 1, Weight: 1, OccurredAt: createdAt, ObservedAt: createdAt,
		SourceSystem: "test", SourceRecordID: key, EvidenceRef: "test:" + key,
		PayloadJSON: "{}", PayloadHash: strings.Repeat("b", 64), CreatedBy: "test", CreatedAt: createdAt,
	}
	if err := conn.Create(&evidence).Error; err != nil {
		t.Fatal(err)
	}
	audit := db.PerformanceAuditEvent{
		RunID: key, EventType: "run_completed", RecordType: "performance_score_run",
		FormulaVersion: "personnel-v1", PayloadJSON: "{}", CreatedAt: createdAt,
	}
	if err := conn.Create(&audit).Error; err != nil {
		t.Fatal(err)
	}
	sourceEvent := performanceSourceFixture(key+"-source", key, SourceEventIssueSnapshot, "", "", createdAt)
	if err := conn.Create(&sourceEvent).Error; err != nil {
		t.Fatal(err)
	}
}

func performanceSourceFixture(key, workItemID, eventType, fromValue, toValue string, occurredAt time.Time) db.PerformanceWorkItemEvent {
	return db.PerformanceWorkItemEvent{
		DedupeKey: key, WorkItemID: workItemID, ProjectKey: "WA", IssueType: "bug",
		EventType: eventType, FromValue: fromValue, ToValue: toValue,
		OccurredAt: occurredAt, ObservedAt: occurredAt, SourceSystem: "test", SourceEventID: key,
		PayloadJSON: "{}", PayloadHash: strings.Repeat("c", 63) + strconv.Itoa(len(key)%10), CreatedAt: occurredAt,
	}
}

func metricResultFromSnapshot(t *testing.T, metricsJSON, code string) metricResult {
	t.Helper()
	var metrics []metricResult
	if err := json.Unmarshal([]byte(metricsJSON), &metrics); err != nil {
		t.Fatal(err)
	}
	for _, metric := range metrics {
		if metric.Code == code {
			return metric
		}
	}
	t.Fatalf("metric %s not found in %s", code, metricsJSON)
	return metricResult{}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
