package server

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"
	"well-ambient/internal/db"
)

func TestEmailSevenDayDistributionUsesScopedSourceTimestamps(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	cfg.DailyJiraEmail.IncludeCommits = false
	today, _, _, err := emailDateBounds("2026-03-09", cfg.DailyJiraEmail.Timezone, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	start := today.AddDate(0, 0, -7)
	old := today.AddDate(0, 0, -30)
	var tasks []db.TaskTelemetry
	for i := 0; i < 7; i++ {
		tasks = append(tasks, db.TaskTelemetry{TaskID: fmt.Sprintf("TREND-%d", i+1), Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Assignee: "alice", Status: "progress", TaskCreatedAt: old, SourceUpdatedAt: start.AddDate(0, 0, i).Add(time.Hour).In(time.FixedZone("CST", 8*3600))})
	}
	tasks = append(tasks,
		db.TaskTelemetry{TaskID: "OUTSIDE-1", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Assignee: "bob", Status: "progress", TaskCreatedAt: old, SourceUpdatedAt: start.Add(time.Hour)},
		db.TaskTelemetry{TaskID: "LOCAL-1", Source: "local", Assignee: "alice", Status: "progress", TaskCreatedAt: old, SourceUpdatedAt: start.Add(time.Hour)},
		db.TaskTelemetry{TaskID: "BEFORE-1", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Assignee: "alice", Status: "progress", TaskCreatedAt: old, SourceUpdatedAt: start.Add(-time.Nanosecond)},
		db.TaskTelemetry{TaskID: "TODAY-1", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Assignee: "alice", Status: "progress", TaskCreatedAt: old, SourceUpdatedAt: today},
	)
	if err := conn.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	report, err := (&Server{config: &cfg}).buildEmailReport(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", today)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report.TrendPoints, []int{1, 1, 1, 1, 1, 1, 1}) {
		t.Fatalf("incorrect day distribution: %v", report.TrendPoints)
	}
	if len(report.YesterdayUpdated) != 1 || len(report.RecentUnresolved) != 0 {
		t.Fatal("trend query changed existing report windows")
	}
	if report.TrendPoints[6] != len(report.YesterdayUpdated) {
		t.Fatal("yesterday total disagrees with trend endpoint")
	}
	cfg.Jira.SyncUsers = nil
	cfg.Jira.CustomJQL = ""
	report, err = (&Server{config: &cfg}).buildEmailReport(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", today)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.TrendPoints) != 0 {
		t.Fatal("unconfigured scope exposed global distribution")
	}
}

func TestEmailLargeTrendDoesNotBlockMainReport(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	cfg.DailyJiraEmail.IncludeCommits = false
	today, yesterday, _, err := emailDateBounds("2026-03-09", cfg.DailyJiraEmail.Timezone, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	older := today.AddDate(0, 0, -5)
	created := today.AddDate(0, 0, -30)
	rows := make([]db.TaskTelemetry, 10001)
	for i := range rows {
		rows[i] = db.TaskTelemetry{TaskID: fmt.Sprintf("HISTORY-%d", i+1), Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Assignee: "bob", TaskCreatedAt: created, SourceUpdatedAt: older}
	}
	if err := conn.Select("task_id", "source", "assignee", "task_created_at", "source_updated_at").CreateInBatches(&rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	current := db.TaskTelemetry{TaskID: "TODAYREPORT-1", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Assignee: "alice", Status: "progress", TaskCreatedAt: created, SourceUpdatedAt: yesterday.Add(time.Hour)}
	if err := conn.Create(&current).Error; err != nil {
		t.Fatal(err)
	}
	report, err := (&Server{config: &cfg}).buildEmailReport(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", today)
	if err != nil {
		t.Fatal("optional history blocked the main report:", err)
	}
	if len(report.YesterdayUpdated) != 1 || report.YesterdayUpdated[0].TaskID != current.TaskID {
		t.Fatal("main report facts were lost")
	}
	if len(report.TrendPoints) != 0 || report.TrendUnavailable != "近7日数据过多，未展示" || report.CurveChartDesc != report.TrendUnavailable {
		t.Fatalf("partial/unexplained history rendered: %v %q", report.TrendPoints, report.CurveChartDesc)
	}
}
