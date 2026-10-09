package openaccess

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/db"
)

func TestIntelligenceAggregatesInvocationsOperationsAndUnknownAge(t *testing.T) {
	service := testService(t)
	if err := service.db.AutoMigrate(&db.OpenOperation{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	for index := 0; index < 5; index++ {
		outcome := "succeeded"
		errorCode := ""
		if index >= 3 {
			outcome = "failed"
			errorCode = "invalid_query"
		}
		if err := service.db.Create(&db.CapabilityInvocation{
			RequestID: "req-" + string(rune('a'+index)), SourceID: "source",
			Transport: "http", ToolName: "jira_aggregate_issues", ToolVersion: "1",
			PolicyVersion: 1, DurationMS: int64(100 + index), Outcome: outcome,
			ErrorCode: errorCode, CreatedAt: now.Add(-time.Duration(index) * time.Hour),
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	operations := []db.OpenOperation{
		{
			ID: "op-success", PlanID: "plan-success", SourceID: "source", Scope: "decision_execute",
			IdempotencyKey: "success", RequestDigest: "a", State: "succeeded",
			CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-time.Hour),
		},
		{
			ID: "op-unknown", PlanID: "plan-unknown", SourceID: "source", Scope: "decision_execute",
			IdempotencyKey: "unknown", RequestDigest: "b", State: "unknown",
			CreatedAt: now.Add(-4 * time.Hour), UpdatedAt: now.Add(-3 * time.Hour),
		},
	}
	if err := service.db.Create(&operations).Error; err != nil {
		t.Fatal(err)
	}
	report, err := service.Intelligence(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if report.InvocationTotal != 5 || len(report.Invocations) != 1 {
		t.Fatalf("invocation report = %+v", report)
	}
	metric := report.Invocations[0]
	if metric.Succeeded != 3 || metric.Failed != 2 || metric.SuccessRate != 0.6 {
		t.Fatalf("tool metric = %+v", metric)
	}
	if report.Operations.UnknownCount != 1 || report.Operations.ConfirmedRate != 0.5 ||
		report.Operations.OldestUnknownSince == nil {
		t.Fatalf("operation metric = %+v", report.Operations)
	}
	if len(report.Recommendations) < 2 {
		t.Fatalf("recommendations = %+v", report.Recommendations)
	}
}

func TestIntelligenceReportsStartedInvocationsSeparately(t *testing.T) {
	service := testService(t)
	if err := service.db.AutoMigrate(&db.OpenOperation{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.October, 3, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	for _, invocation := range []db.CapabilityInvocation{
		{RequestID: "unfinished", SourceID: "source", ToolName: "jira_search_issues", Outcome: "started", CreatedAt: now.Add(-time.Hour)},
		{RequestID: "finished", SourceID: "source", ToolName: "jira_search_issues", Outcome: "succeeded", DurationMS: 100, CreatedAt: now.Add(-time.Minute)},
	} {
		if err := service.db.Create(&invocation).Error; err != nil {
			t.Fatal(err)
		}
	}
	report, err := service.Intelligence(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if report.InvocationTotal != 2 || len(report.Invocations) != 1 || report.Invocations[0].Failed != 0 || report.Invocations[0].SuccessRate != 1 || report.Invocations[0].AverageMS != 100 || !strings.Contains(string(encoded), `"started_count":1`) {
		t.Fatalf("unfinished invocation was misclassified: %s", encoded)
	}
	if !strings.Contains(strings.Join(report.Recommendations, " "), "未结束") {
		t.Fatalf("unfinished invocation is not actionable: %+v", report.Recommendations)
	}
}

func TestIntelligenceKeepsOutstandingUnknownVisibleOutsideWindow(t *testing.T) {
	service := testService(t)
	if err := service.db.AutoMigrate(&db.OpenOperation{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.October, 3, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	oldUnknownSince := now.Add(-45 * 24 * time.Hour)
	operations := []db.OpenOperation{
		{ID: "old-unknown", PlanID: "old-plan", SourceID: "source", Scope: "decision_execute", IdempotencyKey: "old-key", RequestDigest: "old", State: "unknown", CreatedAt: now.Add(-60 * 24 * time.Hour), UpdatedAt: oldUnknownSince},
		{ID: "recent-success", PlanID: "recent-plan", SourceID: "source", Scope: "decision_execute", IdempotencyKey: "recent-key", RequestDigest: "recent", State: "succeeded", CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)},
	}
	if err := service.db.Create(&operations).Error; err != nil {
		t.Fatal(err)
	}
	report, err := service.Intelligence(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if report.Operations.Total != 1 || report.Operations.UnknownCount != 0 || report.Operations.OldestUnknownSince != nil || report.Operations.ConfirmedRate != 1 {
		t.Fatalf("window statistics changed meaning: %+v", report.Operations)
	}
	if report.Operations.OutstandingUnknownCount != 1 || report.Operations.OldestOutstandingUnknownUpdatedAt == nil || !report.Operations.OldestOutstandingUnknownUpdatedAt.Equal(oldUnknownSince) {
		t.Fatalf("old unresolved risk disappeared: %+v", report.Operations)
	}
	if !strings.Contains(strings.Join(report.Recommendations, "\n"), "1 个 unknown Operation") {
		t.Fatalf("unknown risk recommendation missing: %v", report.Recommendations)
	}
}

func TestIntelligenceIncludesLatestFailureAndUnknownBeyondTenThousandRows(t *testing.T) {
	service := testService(t)
	if err := service.db.AutoMigrate(&db.OpenOperation{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.October, 3, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	invocations := make([]db.CapabilityInvocation, 10001)
	operations := make([]db.OpenOperation, 10001)
	for index := range invocations {
		outcome, state, errorCode := "succeeded", "succeeded", ""
		if index == 10000 {
			outcome, state, errorCode = "failed", "unknown", "upstream_unavailable"
		}
		createdAt := now.Add(-time.Duration(10001-index) * time.Second)
		invocations[index] = db.CapabilityInvocation{
			RequestID: fmt.Sprintf("full-window-%d", index), SourceID: "source",
			Transport: "http", ToolName: "jira_search_issues", ToolVersion: "1",
			Outcome: outcome, ErrorCode: errorCode, DurationMS: 100, CreatedAt: createdAt,
		}
		operations[index] = db.OpenOperation{
			ID: fmt.Sprintf("full-window-op-%d", index), PlanID: fmt.Sprintf("full-window-plan-%d", index),
			SourceID: "source", Scope: "decision_execute", IdempotencyKey: fmt.Sprintf("full-window-key-%d", index),
			RequestDigest: "full-window", State: state, CreatedAt: createdAt, UpdatedAt: createdAt,
		}
	}
	if err := service.db.CreateInBatches(&invocations, 100).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.db.CreateInBatches(&operations, 100).Error; err != nil {
		t.Fatal(err)
	}
	report, err := service.Intelligence(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if report.InvocationTotal != 10001 || len(report.Invocations) != 1 {
		t.Fatalf("full invocation total = %d, metrics = %+v", report.InvocationTotal, report.Invocations)
	}
	metric := report.Invocations[0]
	if metric.Total != 10001 || metric.Succeeded != 10000 || metric.Failed != 1 || metric.ErrorCodes["upstream_unavailable"] != 1 || metric.AverageMS != 100 {
		t.Fatalf("latest failure was lost = %+v", metric)
	}
	if metric.SuccessRate >= 1 || report.Operations.ConfirmedRate >= 1 {
		t.Fatalf("partial success presented as complete: invocation=%v operation=%v", metric.SuccessRate, report.Operations.ConfirmedRate)
	}
	if report.Operations.Total != 10001 || report.Operations.UnknownCount != 1 || report.Operations.States["succeeded"] != 10000 || report.Operations.States["unknown"] != 1 {
		t.Fatalf("latest unknown operation was lost = %+v", report.Operations)
	}
	if report.Operations.OldestUnknownSince == nil || !report.Operations.OldestUnknownSince.Equal(now.Add(-time.Second)) {
		t.Fatalf("latest unknown age = %v", report.Operations.OldestUnknownSince)
	}
}
