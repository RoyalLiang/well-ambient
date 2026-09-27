package openaccess

import (
	"context"
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
