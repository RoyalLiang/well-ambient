package jiraquery

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"well-ambient/internal/db"
	"well-ambient/internal/openaccess"
)

func testModule(t *testing.T) (*Module, *gorm.DB, openaccess.Policy) {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := conn.AutoMigrate(&db.TaskTelemetry{}, &db.JiraReportChange{}, &db.OpenQuerySnapshot{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	module := New(conn)
	module.now = func() time.Time {
		return time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	}
	fields := append([]string(nil), defaultFields...)
	fields = append(fields, "description", "reporter", "severity", "parent_key")
	policy := openaccess.Policy{
		Version: 7,
		PolicySpec: openaccess.PolicySpec{
			AllowedProjects: []string{"WA", "NS2"},
			Actions:         []string{"jira.read"},
			DataRules: map[string]string{
				"jira_issue_visibility": "verified_cache",
			},
			FieldRules: map[string][]string{
				"jira_describe_schema": append(append([]string(nil), fields...), "history"),
				"jira_search_issues":   fields,
				"jira_get_issue":       append(append([]string(nil), fields...), "history"),
				"jira_history":         {"status", "assignee", "priority", "duedate"},
				"jira_aggregate_issues": {
					"key", "project", "issue_type", "status", "assignee", "priority",
					"bug_category", "due_date", "created_at", "updated_at", "history_complete", "history",
				},
			},
			QueryLimits: openaccess.QueryLimits{
				MaxPageSize: 2, MaxScanRows: 10, MaxGroupBy: 2, MaxTimeBuckets: 10,
			},
		},
	}
	return module, conn, policy
}

func seedIssues(t *testing.T, conn *gorm.DB) {
	t.Helper()
	now := time.Date(2026, time.September, 27, 10, 0, 0, 0, time.UTC)
	duePast := now.Add(-24 * time.Hour)
	dueFuture := now.Add(24 * time.Hour)
	rows := []db.TaskTelemetry{
		{
			TaskID: "WA-3", ProjectKey: "WA", Source: "jira", Title: "Third",
			IssueType: "bug", Status: "progress", Assignee: "",
			Priority: "high", JiraBugCategory: "logic", JiraHistoryComplete: false,
			TaskCreatedAt: now.Add(-72 * time.Hour), SourceUpdatedAt: now.Add(3 * time.Hour),
			LastUpdate: now.Add(3 * time.Hour), DueDate: &duePast,
		},
		{
			TaskID: "WA-2", ProjectKey: "WA", Source: "jira", Title: "Second",
			IssueType: "bug", Status: "done", Assignee: "Alice",
			Priority: "medium", JiraBugCategory: "logic", JiraHistoryComplete: true,
			TaskCreatedAt: now.Add(-48 * time.Hour), SourceUpdatedAt: now.Add(2 * time.Hour),
			LastUpdate: now.Add(2 * time.Hour), DueDate: &duePast,
		},
		{
			TaskID: "WA-1", ProjectKey: "WA", Source: "jira", Title: "First",
			IssueType: "requirement", Status: "progress", Assignee: "Bob",
			Priority: "low", JiraHistoryComplete: true,
			TaskCreatedAt: now.Add(-24 * time.Hour), SourceUpdatedAt: now.Add(time.Hour),
			LastUpdate: now.Add(time.Hour), DueDate: &dueFuture,
		},
		{
			TaskID: "NS2-1", ProjectKey: "NS2", Source: "jira", Title: "Other project",
			IssueType: "bug", Status: "review", Assignee: "Alice",
			Priority: "high", JiraHistoryComplete: true,
			TaskCreatedAt: now, SourceUpdatedAt: now, LastUpdate: now,
		},
		{
			TaskID: "SECRET-1", ProjectKey: "SECRET", Source: "jira", Title: "Restricted",
			IssueType: "bug", Status: "progress", SourceUpdatedAt: now.Add(10 * time.Hour),
		},
		{
			TaskID: "TASK-1", ProjectKey: "WA", Source: "local", Title: "Local",
			IssueType: "task", Status: "progress", SourceUpdatedAt: now.Add(20 * time.Hour),
		},
	}
	if err := conn.Create(&rows).Error; err != nil {
		t.Fatalf("seed issues: %v", err)
	}
	changes := []db.JiraReportChange{
		{TaskID: "WA-2", HistoryID: "sensitive", ItemIndex: 0, Field: "custom_email", FromValue: "", ToValue: "secret@example.com", OccurredAt: now.Add(2 * time.Hour)},
		{TaskID: "WA-2", HistoryID: "h2", ItemIndex: 0, Field: "status", FromValue: "progress", ToValue: "done", OccurredAt: now.Add(90 * time.Minute)},
		{TaskID: "WA-2", HistoryID: "h1", ItemIndex: 0, Field: "assignee", FromValue: "Bob", ToValue: "Alice", OccurredAt: now.Add(30 * time.Minute)},
	}
	if err := conn.Create(&changes).Error; err != nil {
		t.Fatalf("seed history: %v", err)
	}
}

func TestSearchUsesPublishedScopeStableCursorAndFullCompleteness(t *testing.T) {
	module, conn, policy := testModule(t)
	seedIssues(t, conn)
	ctx := context.Background()

	first, err := module.Search(ctx, policy, "req-1", SearchRequest{
		Filters: Filters{Projects: []string{"WA"}},
		Fields:  []string{"key", "summary", "history_complete"},
		Limit:   2,
	})
	if err != nil {
		t.Fatalf("search first page: %v", err)
	}
	if len(first.Items) != 2 || first.Items[0]["key"] != "WA-3" || first.Items[1]["key"] != "WA-2" {
		t.Fatalf("unexpected first page: %+v", first.Items)
	}
	if !first.HasMore || first.NextCursor == "" {
		t.Fatalf("first page did not expose cursor: %+v", first)
	}
	firstSnapshot := first.Meta.QuerySnapshotRef
	if err := conn.Model(&db.TaskTelemetry{}).Where("task_id = ?", "WA-1").
		Updates(map[string]any{
			"title":             "Changed after snapshot",
			"source_updated_at": time.Date(2026, time.September, 28, 10, 0, 0, 0, time.UTC),
		}).Error; err != nil {
		t.Fatal(err)
	}
	if first.Meta.Completeness != 2.0/3.0 {
		t.Fatalf("completeness = %v, want 2/3", first.Meta.Completeness)
	}
	if len(first.Meta.MissingReasons) == 0 || strings.Contains(first.Meta.QuerySnapshotRef, "SECRET") {
		t.Fatalf("unexpected snapshot metadata: %+v", first.Meta)
	}

	second, err := module.Search(ctx, policy, "req-2", SearchRequest{
		Filters: Filters{Projects: []string{"WA"}},
		Fields:  []string{"key", "summary", "history_complete"},
		Limit:   2,
		Cursor:  first.NextCursor,
	})
	if err != nil {
		t.Fatalf("search second page: %v", err)
	}
	if len(second.Items) != 1 || second.Items[0]["key"] != "WA-1" ||
		second.Items[0]["summary"] != "First" || second.HasMore {
		t.Fatalf("unexpected second page: %+v", second)
	}
	if second.Meta.QuerySnapshotRef != firstSnapshot {
		t.Fatalf("snapshot changed across pages: %q -> %q", firstSnapshot, second.Meta.QuerySnapshotRef)
	}

	if _, err := module.Search(ctx, policy, "req-3", SearchRequest{
		Filters: Filters{Projects: []string{"SECRET"}},
		Fields:  []string{"key"},
	}); openaccess.ErrorCode(err) != "forbidden" {
		t.Fatalf("restricted project error = %v", err)
	}
	if _, err := module.Search(ctx, policy, "req-4", SearchRequest{
		Filters: Filters{Projects: []string{"WA"}},
		Fields:  []string{"key"},
		Cursor:  first.NextCursor + "tampered",
	}); openaccess.ErrorCode(err) != "invalid_query" {
		t.Fatalf("tampered cursor error = %v", err)
	}
}

func TestAggregateUsesWholeAuthorizedSetAndDoesNotTreatMissingHistoryAsZero(t *testing.T) {
	module, conn, policy := testModule(t)
	seedIssues(t, conn)
	response, err := module.Aggregate(context.Background(), policy, "req-aggregate", AggregateRequest{
		Filters: Filters{Projects: []string{"WA"}},
		Metrics: []string{"issue_count", "unassigned_count", "overdue_count", "history_incomplete_count"},
		GroupBy: []string{"status"},
	})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if response.Meta.Completeness != 2.0/3.0 || len(response.Meta.MissingReasons) == 0 {
		t.Fatalf("aggregate completeness = %+v", response.Meta)
	}
	var progress AggregateRow
	for _, row := range response.Rows {
		if row.Groups["status"] == "progress" {
			progress = row
		}
	}
	if progress.Metrics["issue_count"] != 2 ||
		progress.Metrics["unassigned_count"] != 1 ||
		progress.Metrics["overdue_count"] != 1 ||
		progress.Metrics["history_incomplete_count"] != 1 {
		t.Fatalf("unexpected progress aggregate: %+v", progress)
	}
	for _, row := range response.Rows {
		if row.Groups["status"] == "done" {
			if value, ok := row.Metrics["unassigned_count"]; !ok || value != 0 {
				t.Fatalf("done unassigned_count = %v, present=%v; want explicit zero", value, ok)
			}
		}
	}

	policy.QueryLimits.MaxScanRows = 2
	if _, err := module.Aggregate(context.Background(), policy, "req-budget", AggregateRequest{
		Filters: Filters{Projects: []string{"WA"}},
		Metrics: []string{"issue_count"},
	}); openaccess.ErrorCode(err) != "invalid_query" {
		t.Fatalf("over-budget aggregate error = %v", err)
	}
}

func TestGetReturnsBoundedHistoryAndSourceReferences(t *testing.T) {
	module, conn, policy := testModule(t)
	seedIssues(t, conn)
	first, err := module.Get(context.Background(), policy, "req-get", GetRequest{
		Key: "wa-2", Fields: []string{"key", "summary", "assignee"},
		IncludeHistory: true, HistoryLimit: 1,
	})
	if err != nil {
		t.Fatalf("get issue: %v", err)
	}
	if first.Issue["key"] != "WA-2" || len(first.History) != 1 ||
		first.History[0].HistoryID != "h2" || !first.HasMore || first.NextCursor == "" {
		t.Fatalf("unexpected first detail page: %+v", first)
	}
	if len(first.Meta.SourceRefs) < 2 || first.Meta.Completeness != 1 {
		t.Fatalf("unexpected detail metadata: %+v", first.Meta)
	}

	second, err := module.Get(context.Background(), policy, "req-get-2", GetRequest{
		Key: "WA-2", Fields: []string{"key"}, IncludeHistory: true,
		HistoryLimit: 1, HistoryCursor: first.NextCursor,
	})
	if err != nil {
		t.Fatalf("get second history page: %v", err)
	}
	if len(second.History) != 1 || second.History[0].HistoryID != "h1" || second.HasMore {
		t.Fatalf("unexpected second history page: %+v", second)
	}
}

func TestJiraQueriesFailClosedWithoutVerifiedIssueVisibility(t *testing.T) {
	module, conn, policy := testModule(t)
	seedIssues(t, conn)
	policy.DataRules = nil
	ctx := context.Background()
	assertForbidden := func(name string, err error) {
		t.Helper()
		if openaccess.ErrorCode(err) != "forbidden" {
			t.Fatalf("%s error = %v, want forbidden", name, err)
		}
	}
	_, err := module.DescribeSchema(ctx, policy, "schema")
	assertForbidden("schema", err)
	_, err = module.Search(ctx, policy, "search", SearchRequest{Fields: []string{"key"}})
	assertForbidden("search", err)
	_, err = module.Get(ctx, policy, "get", GetRequest{Key: "WA-1", Fields: []string{"key"}})
	assertForbidden("get", err)
	_, err = module.Aggregate(ctx, policy, "aggregate", AggregateRequest{Metrics: []string{"issue_count"}})
	assertForbidden("aggregate", err)
}

func TestHistoricalMetricsSeparateEventsDistinctIssuesBucketsAndCompleteness(t *testing.T) {
	module, conn, policy := testModule(t)
	seedIssues(t, conn)
	base := time.Date(2026, time.September, 27, 10, 0, 0, 0, time.UTC)
	extra := []db.JiraReportChange{
		{TaskID: "WA-2", HistoryID: "h3", ItemIndex: 0, Field: "status", FromValue: "done", ToValue: "progress", OccurredAt: base.Add(2 * time.Hour)},
		{TaskID: "WA-2", HistoryID: "h4", ItemIndex: 0, Field: "status", FromValue: "progress", ToValue: "done", OccurredAt: base.Add(3 * time.Hour)},
		{TaskID: "WA-1", HistoryID: "h5", ItemIndex: 0, Field: "status", FromValue: "progress", ToValue: "done", OccurredAt: base.Add(3 * time.Hour)},
		{TaskID: "WA-1", HistoryID: "end-exclusive", ItemIndex: 0, Field: "status", FromValue: "done", ToValue: "progress", OccurredAt: base.Add(4 * time.Hour)},
	}
	if err := conn.Create(&extra).Error; err != nil {
		t.Fatal(err)
	}
	until := base.Add(4 * time.Hour)
	response, err := module.Aggregate(context.Background(), policy, "history", AggregateRequest{
		Filters: Filters{Projects: []string{"WA"}},
		Metrics: []string{
			"resolved_event_count", "resolved_issue_count", "reopened_event_count",
			"assignee_change_event_count", "assignee_change_issue_count",
		},
		GroupBy: []string{"project"}, EventFrom: &base, EventUntil: &until, TimeBucket: "day",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Rows) != 1 || response.Rows[0].Groups["time_bucket"] != "2026-09-27" {
		t.Fatalf("historical rows = %+v", response.Rows)
	}
	metrics := response.Rows[0].Metrics
	if metrics["resolved_event_count"] != 3 || metrics["resolved_issue_count"] != 2 ||
		metrics["reopened_event_count"] != 1 ||
		metrics["assignee_change_event_count"] != 1 ||
		metrics["assignee_change_issue_count"] != 1 {
		t.Fatalf("historical metrics = %+v", metrics)
	}
	if response.MetricCompleteness["resolved_event_count"] != "partial" ||
		response.Meta.Completeness != 2.0/3.0 || response.TimeZone != "UTC" {
		t.Fatalf("historical completeness = %+v meta=%+v", response.MetricCompleteness, response.Meta)
	}
}

func TestCreatedAndStatusDurationHistoricalMetrics(t *testing.T) {
	module, conn, policy := testModule(t)
	seedIssues(t, conn)
	base := time.Date(2026, time.September, 27, 10, 0, 0, 0, time.UTC)
	extra := []db.JiraReportChange{
		{TaskID: "WA-2", HistoryID: "h3", ItemIndex: 0, Field: "status", FromValue: "done", ToValue: "progress", OccurredAt: base.Add(2 * time.Hour)},
		{TaskID: "WA-2", HistoryID: "h4", ItemIndex: 0, Field: "status", FromValue: "progress", ToValue: "done", OccurredAt: base.Add(3 * time.Hour)},
	}
	if err := conn.Create(&extra).Error; err != nil {
		t.Fatal(err)
	}
	createdFrom := base.Add(-80 * time.Hour)
	createdUntil := base
	created, err := module.Aggregate(context.Background(), policy, "created", AggregateRequest{
		Filters: Filters{Projects: []string{"WA"}}, Metrics: []string{"created_issue_count"},
		EventFrom: &createdFrom, EventUntil: &createdUntil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Rows) != 1 || created.Rows[0].Metrics["created_issue_count"] != 3 ||
		created.MetricCompleteness["created_issue_count"] != "complete" ||
		created.Meta.Completeness != 1 {
		t.Fatalf("created metrics = %+v", created)
	}

	durationUntil := base.Add(4 * time.Hour)
	duration, err := module.Aggregate(context.Background(), policy, "duration", AggregateRequest{
		Filters: Filters{Projects: []string{"WA"}}, Metrics: []string{"status_duration_hours"},
		GroupBy: []string{"status"}, EventFrom: &base, EventUntil: &durationUntil,
	})
	if err != nil {
		t.Fatal(err)
	}
	hours := map[string]float64{}
	for _, row := range duration.Rows {
		hours[row.Groups["status"]] = row.Metrics["status_duration_hours"]
	}
	if hours["progress"] != 6.5 || hours["done"] != 1.5 {
		t.Fatalf("status duration hours = %+v", hours)
	}
	if duration.MetricCompleteness["status_duration_hours"] != "partial" {
		t.Fatalf("duration completeness = %+v", duration.MetricCompleteness)
	}
}
