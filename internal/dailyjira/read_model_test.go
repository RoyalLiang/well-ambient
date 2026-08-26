package dailyjira_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/dailyjira"
	"well-ambient/internal/db"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openReadModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	conn, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := conn.AutoMigrate(&db.TaskTelemetry{}); err != nil {
		t.Fatalf("migrate task telemetry: %v", err)
	}
	if err := dailyjira.Migrate(conn); err != nil {
		t.Fatalf("migrate Daily Jira read model: %v", err)
	}
	return conn
}

func TestReadPageUsesBoundedGenerationCursorAndStableSort(t *testing.T) {
	conn := openReadModelTestDB(t)
	now := time.Now()
	dueYesterday := now.AddDate(0, 0, -1)
	tasks := []db.TaskTelemetry{
		{TaskID: "HIT-100", ProjectKey: "HIT", Source: "jira", Title: "old healthy", Repo: "HIT", Assignee: "Alice", Status: "progress", IssueType: "bug", TaskCreatedAt: now.AddDate(0, 0, -12), LastUpdate: now.Add(-time.Hour)},
		{TaskID: "HIT-101", ProjectKey: "HIT", Source: "jira", Title: "old overdue", Repo: "HIT", Assignee: "Alice", Status: "progress", IssueType: "bug", TaskCreatedAt: now.AddDate(0, 0, -10), LastUpdate: now.Add(-2 * time.Hour), DueDate: &dueYesterday},
		{TaskID: "HIT-102", ProjectKey: "HIT", Source: "jira", Title: "newer overdue", Repo: "HIT", Assignee: "Bob", Status: "review", IssueType: "task", TaskCreatedAt: now.AddDate(0, 0, -8), LastUpdate: now.Add(-3 * time.Hour), DueDate: &dueYesterday},
		{TaskID: "HIT-103", ProjectKey: "HIT", Source: "jira", Title: "resolved", Repo: "HIT", Assignee: "Bob", Status: "done", IssueType: "bug", TaskCreatedAt: now.AddDate(0, 0, -20), LastUpdate: now},
		{TaskID: "TASK-104", ProjectKey: "HIT", Source: "git", Title: "not Jira", Repo: "HIT", Assignee: "Bob", Status: "progress", IssueType: "task", TaskCreatedAt: now.AddDate(0, 0, -20), LastUpdate: now},
	}
	if err := conn.Create(&tasks).Error; err != nil {
		t.Fatalf("seed tasks: %v", err)
	}

	reader := dailyjira.NewReader(conn)
	first, err := reader.ReadPage(context.Background(), dailyjira.Query{Bucket: dailyjira.BucketSevenDay, Limit: 2, Now: now})
	if err != nil {
		t.Fatalf("read first page: %v", err)
	}
	if first.Summary.Total != 3 || first.Summary.SevenDay != 3 {
		t.Fatalf("summary = %+v, want three seven-day Jira items", first.Summary)
	}
	if len(first.Items) != 2 || first.Items[0].TaskID != "HIT-101" || first.Items[1].TaskID != "HIT-102" {
		t.Fatalf("first page order = %+v, want overdue oldest first", first.Items)
	}
	if !first.HasMore || first.NextCursor == "" {
		t.Fatalf("first page must expose a continuation cursor: %+v", first)
	}

	second, err := reader.ReadPage(context.Background(), dailyjira.Query{Bucket: dailyjira.BucketSevenDay, Limit: 2, Cursor: first.NextCursor, Now: now})
	if err != nil {
		t.Fatalf("read second page: %v", err)
	}
	if len(second.Items) != 1 || second.Items[0].TaskID != "HIT-100" || second.HasMore || second.NextCursor != "" {
		t.Fatalf("second page = %+v, want only HIT-100 and no continuation", second)
	}
}

func TestReadPageSupportsBoundedPreviousNavigationWithoutSkippingRows(t *testing.T) {
	conn := openReadModelTestDB(t)
	now := time.Now()
	for index := 0; index < 7; index++ {
		task := db.TaskTelemetry{
			TaskID: fmt.Sprintf("BACK-%03d", index), ProjectKey: "BACK", Source: "jira",
			Title: fmt.Sprintf("bounded page %d", index), Repo: "Backfill", Status: "progress",
			TaskCreatedAt: now.AddDate(0, 0, -20+index), LastUpdate: now.Add(time.Duration(index) * time.Minute),
		}
		if err := conn.Create(&task).Error; err != nil {
			t.Fatalf("seed previous-page task: %v", err)
		}
	}

	reader := dailyjira.NewReader(conn)
	first, err := reader.ReadPage(context.Background(), dailyjira.Query{
		Bucket: dailyjira.BucketSevenDay, Limit: 3, Now: now,
	})
	if err != nil {
		t.Fatalf("read first bounded page: %v", err)
	}
	second, err := reader.ReadPage(context.Background(), dailyjira.Query{
		Bucket: dailyjira.BucketSevenDay, Limit: 3, Cursor: first.NextCursor, Now: now,
	})
	if err != nil {
		t.Fatalf("read second bounded page: %v", err)
	}
	if !second.HasPrevious || second.PreviousCursor == "" {
		t.Fatalf("second page must expose a previous cursor: %+v", second)
	}

	back, err := reader.ReadPage(context.Background(), dailyjira.Query{
		Bucket: dailyjira.BucketSevenDay, Limit: 3, Cursor: second.PreviousCursor,
		Direction: dailyjira.DirectionPrevious, Now: now,
	})
	if err != nil {
		t.Fatalf("read previous bounded page: %v", err)
	}
	if len(back.Items) != len(first.Items) {
		t.Fatalf("previous page length = %d, want %d", len(back.Items), len(first.Items))
	}
	for index := range first.Items {
		if back.Items[index].TaskID != first.Items[index].TaskID {
			t.Fatalf("previous page item %d = %s, want %s", index, back.Items[index].TaskID, first.Items[index].TaskID)
		}
	}
	if back.HasPrevious || back.PreviousCursor != "" || !back.HasMore || back.NextCursor == "" {
		t.Fatalf("previous page boundaries are incorrect: %+v", back)
	}
}

func TestReadPageRejectsCursorAfterProjectionGenerationChanges(t *testing.T) {
	conn := openReadModelTestDB(t)
	now := time.Now()
	for index := 0; index < 3; index++ {
		task := db.TaskTelemetry{
			TaskID: fmt.Sprintf("NS2-%d", 200+index), ProjectKey: "NS2", Source: "jira",
			Title: "cursor", Repo: "NS2", Status: "progress", TaskCreatedAt: now.AddDate(0, 0, -8-index), LastUpdate: now,
		}
		if err := conn.Create(&task).Error; err != nil {
			t.Fatalf("seed cursor task: %v", err)
		}
	}

	reader := dailyjira.NewReader(conn)
	first, err := reader.ReadPage(context.Background(), dailyjira.Query{Bucket: dailyjira.BucketSevenDay, Limit: 1, Now: now})
	if err != nil {
		t.Fatalf("read first page: %v", err)
	}
	if err := conn.Model(&db.TaskTelemetry{}).Where("task_id = ?", "NS2-201").Update("title", "changed").Error; err != nil {
		t.Fatalf("change projection generation: %v", err)
	}
	_, err = reader.ReadPage(context.Background(), dailyjira.Query{Bucket: dailyjira.BucketSevenDay, Limit: 1, Cursor: first.NextCursor, Now: now})
	if !errors.Is(err, dailyjira.ErrStaleCursor) {
		t.Fatalf("cursor error = %v, want ErrStaleCursor", err)
	}
}

func TestProjectionGenerationIgnoresUnrelatedTaskUpdates(t *testing.T) {
	conn := openReadModelTestDB(t)
	now := time.Now()
	task := db.TaskTelemetry{
		TaskID: "WA-250", ProjectKey: "WA", Source: "jira", Title: "stable projection",
		Repo: "Well Ambient", Status: "progress", TaskCreatedAt: now.AddDate(0, 0, -8), LastUpdate: now,
	}
	if err := conn.Create(&task).Error; err != nil {
		t.Fatalf("seed stable projection task: %v", err)
	}
	reader := dailyjira.NewReader(conn)
	before, err := reader.ReadPage(context.Background(), dailyjira.Query{Bucket: dailyjira.BucketSevenDay, Now: now})
	if err != nil {
		t.Fatalf("read initial projection generation: %v", err)
	}
	if err := conn.Model(&db.TaskTelemetry{}).Where("task_id = ?", task.TaskID).Update("branch", "feature/read-model").Error; err != nil {
		t.Fatalf("update unrelated task field: %v", err)
	}
	afterUnrelated, err := reader.ReadPage(context.Background(), dailyjira.Query{Bucket: dailyjira.BucketSevenDay, Now: now})
	if err != nil {
		t.Fatalf("read generation after unrelated update: %v", err)
	}
	if afterUnrelated.Generation != before.Generation {
		t.Fatalf("unrelated update advanced generation from %d to %d", before.Generation, afterUnrelated.Generation)
	}
	if err := conn.Model(&db.TaskTelemetry{}).Where("task_id = ?", task.TaskID).Update("title", "changed projection").Error; err != nil {
		t.Fatalf("update projected task field: %v", err)
	}
	afterProjected, err := reader.ReadPage(context.Background(), dailyjira.Query{Bucket: dailyjira.BucketSevenDay, Now: now})
	if err != nil {
		t.Fatalf("read generation after projected update: %v", err)
	}
	if afterProjected.Generation <= afterUnrelated.Generation || afterProjected.Items[0].Title != "changed projection" {
		t.Fatalf("projected update did not advance generation/data: before=%d after=%+v", afterUnrelated.Generation, afterProjected)
	}
}

func TestReadPageAppliesProjectAssigneeAndIndexedSearchInsideTheModule(t *testing.T) {
	conn := openReadModelTestDB(t)
	now := time.Now()
	tasks := []db.TaskTelemetry{
		{TaskID: "FZ-301", ProjectKey: "FZ", Source: "jira", Title: "吊具传感器异常", Repo: "Fuzhou", Assignee: "梁志远", Status: "progress", TaskCreatedAt: now.AddDate(0, 0, -9), LastUpdate: now},
		{TaskID: "FZ-302", ProjectKey: "FZ", Source: "jira", Title: "箱号识别异常", Repo: "Fuzhou", Assignee: "外部成员", Status: "progress", TaskCreatedAt: now.AddDate(0, 0, -9), LastUpdate: now},
		{TaskID: "HIT-303", ProjectKey: "HIT", Source: "jira", Title: "吊具传感器异常", Repo: "HIT", Assignee: "梁志远", Status: "progress", TaskCreatedAt: now.AddDate(0, 0, -9), LastUpdate: now},
	}
	if err := conn.Create(&tasks).Error; err != nil {
		t.Fatalf("seed scoped tasks: %v", err)
	}

	page, err := dailyjira.NewReader(conn).ReadPage(context.Background(), dailyjira.Query{
		Bucket: dailyjira.BucketSevenDay,
		Search: "吊具传",
		Scope:  dailyjira.Scope{ProjectKeys: []string{"fz"}, Assignees: []string{"梁志远"}, FilterAssignees: true},
		Now:    now,
	})
	if err != nil {
		t.Fatalf("read scoped search: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].TaskID != "FZ-301" {
		t.Fatalf("scoped search items = %+v, want only FZ-301", page.Items)
	}
	if page.Summary.SevenDay != 1 {
		t.Fatalf("scoped summary = %+v, want one visible seven-day item", page.Summary)
	}
}

func TestReadPageRollsOnlyDueNaturalDayTransitions(t *testing.T) {
	conn := openReadModelTestDB(t)
	now := time.Now()
	task := db.TaskTelemetry{
		TaskID: "WA-401", ProjectKey: "WA", Source: "jira", Title: "rollover", Repo: "Well Ambient",
		Status: "progress", TaskCreatedAt: now.AddDate(0, 0, -2), LastUpdate: now,
	}
	if err := conn.Create(&task).Error; err != nil {
		t.Fatalf("seed rollover task: %v", err)
	}

	reader := dailyjira.NewReader(conn)
	watch, err := reader.ReadPage(context.Background(), dailyjira.Query{Bucket: dailyjira.BucketThreeDay, Now: now})
	if err != nil {
		t.Fatalf("read watch state: %v", err)
	}
	if watch.Summary.RecentWatch != 1 || watch.Summary.ThreeDay != 0 {
		t.Fatalf("watch summary = %+v", watch.Summary)
	}

	if _, err := dailyjira.RollForwardBatch(context.Background(), conn, now.AddDate(0, 0, 1), 100); err != nil {
		t.Fatalf("roll projection forward: %v", err)
	}
	threeDay, err := reader.ReadPage(context.Background(), dailyjira.Query{Bucket: dailyjira.BucketThreeDay, Now: now.AddDate(0, 0, 1)})
	if err != nil {
		t.Fatalf("read rollover state: %v", err)
	}
	if threeDay.Summary.RecentWatch != 0 || threeDay.Summary.ThreeDay != 1 || len(threeDay.Items) != 1 {
		t.Fatalf("rolled summary/page = %+v / %+v", threeDay.Summary, threeDay.Items)
	}
}

func TestReadPageQueryPlansStayOnCompositeIndexes(t *testing.T) {
	conn := openReadModelTestDB(t)

	pagePlan := explainPlan(t, conn, `SELECT task_id
		FROM daily_jira_audit_entries
		WHERE bucket = ? AND (sort_overdue, created_unix, activity_unix, task_id) > (?, ?, ?, ?)
		ORDER BY sort_overdue ASC, created_unix ASC, activity_unix ASC, task_id ASC
		LIMIT ?`, "seven_day", 0, 1, 1, "HIT-1", 101)
	assertPlanUses(t, pagePlan, "idx_daily_jira_page")
	assertPlanAvoidsTempSort(t, pagePlan)

	scopedPlan := explainPlan(t, conn, `SELECT task_id
		FROM daily_jira_audit_entries
		WHERE project_key = ? AND assignee_key = ? AND bucket = ?
		ORDER BY sort_overdue ASC, created_unix ASC, activity_unix ASC, task_id ASC
		LIMIT ?`, "HIT", "梁志远", "seven_day", 101)
	assertPlanUses(t, scopedPlan, "idx_daily_jira_project_assignee_page")
	assertPlanAvoidsTempSort(t, scopedPlan)

	countPlan := explainPlan(t, conn, `SELECT bucket, SUM(item_count)
		FROM daily_jira_audit_counts
		WHERE item_count > 0
		GROUP BY bucket`)
	assertPlanUses(t, countPlan, "idx_daily_jira_count_bucket")
	assigneeCountPlan := explainPlan(t, conn, `SELECT bucket, SUM(item_count)
		FROM daily_jira_audit_counts
		WHERE item_count > 0 AND assignee_key IN (?, ?)
		GROUP BY bucket`, "梁志远", "")
	assertPlanUses(t, assigneeCountPlan, "idx_daily_jira_count_assignee")

	searchPlan := explainPlan(t, conn, `SELECT e.task_id
		FROM daily_jira_audit_entries AS e
		JOIN (
			SELECT task_id FROM daily_jira_audit_entries INDEXED BY idx_daily_jira_short_task_search
				WHERE bucket = ? AND task_id GLOB ?
			UNION
			SELECT task_id FROM daily_jira_audit_entries INDEXED BY idx_daily_jira_short_title_search
				WHERE bucket = ? AND title_key GLOB ?
			UNION
			SELECT task_id FROM daily_jira_audit_entries INDEXED BY idx_daily_jira_short_project_search
				WHERE bucket = ? AND project_key GLOB ?
			UNION
			SELECT task_id FROM daily_jira_audit_entries INDEXED BY idx_daily_jira_short_assignee_search
				WHERE bucket = ? AND assignee_key GLOB ?
			UNION
			SELECT task_id FROM daily_jira_audit_entries INDEXED BY idx_daily_jira_short_status_search
				WHERE bucket = ? AND status_key GLOB ?
		) AS matching ON matching.task_id = e.task_id
		WHERE e.bucket = ?
		ORDER BY e.sort_overdue, e.created_unix, e.activity_unix, e.task_id
		LIMIT ?`,
		"seven_day", "HIT-*",
		"seven_day", "吊具*",
		"seven_day", "HIT*",
		"seven_day", "梁*",
		"seven_day", "progress*",
		"seven_day", 101,
	)
	if strings.Contains(searchPlan, "SCAN daily_jira_audit_entries") {
		t.Fatalf("prefix search fell back to a full projection scan:\n%s", searchPlan)
	}
	assertPlanUses(t, searchPlan, "idx_daily_jira_short_task_search")
	assertPlanUses(t, searchPlan, "idx_daily_jira_short_title_search")
}

func explainPlan(t *testing.T, conn *gorm.DB, query string, args ...any) string {
	t.Helper()
	var rows []struct {
		Detail string `gorm:"column:detail"`
	}
	if err := conn.Raw("EXPLAIN QUERY PLAN "+query, args...).Scan(&rows).Error; err != nil {
		t.Fatalf("explain query plan: %v", err)
	}
	details := make([]string, 0, len(rows))
	for _, row := range rows {
		details = append(details, row.Detail)
	}
	return strings.Join(details, "\n")
}

func assertPlanUses(t *testing.T, plan, indexName string) {
	t.Helper()
	if !strings.Contains(plan, indexName) {
		t.Fatalf("query plan did not use %s:\n%s", indexName, plan)
	}
}

func assertPlanAvoidsTempSort(t *testing.T, plan string) {
	t.Helper()
	if strings.Contains(strings.ToUpper(plan), "TEMP B-TREE") {
		t.Fatalf("query plan requires a temporary sort:\n%s", plan)
	}
}
