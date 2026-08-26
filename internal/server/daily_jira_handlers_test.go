package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/kanban"
)

func TestBuildDailyJiraAuditResponseUsesNonOverlappingNaturalDayBuckets(t *testing.T) {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, time.July, 18, 9, 30, 0, 0, location)
	created := func(daysAgo int) time.Time {
		return now.AddDate(0, 0, -daysAgo).Add(-2 * time.Hour)
	}

	tasks := []db.TaskTelemetry{
		{TaskID: "WA-100", Title: "today", Repo: "Well Ambient (WA)", Status: "backlog", Assignee: "Alice", TaskCreatedAt: created(0), LastUpdate: created(0)},
		{TaskID: "WA-102", Title: "watch", Repo: "Well Ambient (WA)", Status: "progress", Assignee: "Alice", TaskCreatedAt: created(2), LastUpdate: created(1)},
		{TaskID: "WA-103", Title: "three", Repo: "Well Ambient (WA)", Status: "progress", Assignee: "Bob", TaskCreatedAt: created(3), LastUpdate: created(2)},
		{TaskID: "WA-106", Title: "six", Repo: "Well Ambient (WA)", Status: "review", Assignee: "Bob", TaskCreatedAt: created(6), LastUpdate: created(4)},
		{TaskID: "WA-107", Title: "seven", Repo: "Well Ambient (WA)", Status: "progress", Assignee: "Carol", TaskCreatedAt: created(7), LastUpdate: created(7)},
		{TaskID: "WA-120", Title: "done", Repo: "Well Ambient (WA)", Status: "done", Assignee: "Alice", TaskCreatedAt: created(12), LastUpdate: created(1)},
		{TaskID: "TASK-999", Title: "local task", Repo: "Well Ambient (WA)", Status: "progress", Assignee: "Alice", TaskCreatedAt: created(9), LastUpdate: created(2)},
		{TaskID: "DEMAND-001", Title: "local demand", Repo: "Well Ambient (WA)", Status: "progress", Assignee: "Alice", Creator: "Alice", TaskCreatedAt: created(9), LastUpdate: created(2)},
		{TaskID: "WA-130", Title: "unknown age", Repo: "Well Ambient (WA)", Status: "progress", Assignee: "Alice"},
	}
	events := []db.DecisionEvent{{TaskID: "WA-107", Action: "daily_jira_escalate", Actor: "PM", CreatedAt: now.Add(-time.Hour)}}
	decisions := []db.DailyJiraDecision{{TaskID: "WA-107", Status: "escalate", Actor: "PM", ReminderAt: now.Add(-time.Minute), CreatedAt: now.Add(-5 * time.Hour)}}

	response := buildDailyJiraAuditResponse(tasks, events, decisions, []string{"Alice", "Bob", "Carol"}, now)
	if response.Summary.Total != 4 || response.Summary.Today != 1 || response.Summary.ThreeDay != 2 || response.Summary.SevenDay != 1 {
		t.Fatalf("unexpected summary: %+v", response.Summary)
	}
	if response.RecentWatchCount != 1 {
		t.Fatalf("recent watch count = %d, want 1", response.RecentWatchCount)
	}
	if response.UnclassifiedCount != 1 {
		t.Fatalf("unclassified count = %d, want 1", response.UnclassifiedCount)
	}
	if len(response.Buckets) != 3 || response.Buckets[1].Items[0].TaskID != "WA-106" || response.Buckets[2].Items[0].DecisionEvents[0].Action != "daily_jira_escalate" {
		t.Fatalf("unexpected bucket content: %+v", response.Buckets)
	}
	latest := response.Buckets[2].Items[0].LatestDecision
	if latest == nil || latest.Status != "escalate" || !latest.ReminderDue {
		t.Fatalf("unexpected latest decision projection: %+v", latest)
	}
}

func TestBuildDailyJiraAuditResponseExcludesGitLabMRProjection(t *testing.T) {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, time.August, 13, 10, 0, 0, 0, location)
	tasks := []db.TaskTelemetry{{
		TaskID:        "fz-2247",
		Source:        "git",
		Title:         "feat: FZ-2247 添加吊具检测驶离保护",
		Repo:          "task_executor",
		Branch:        "dev_fuzhou",
		LastCommit:    "feat: FZ-2247 添加吊具检测驶离保护\n\nSee merge request fms/task_executor!23",
		MrIID:         23,
		MrURL:         "https://gitlab.example.com/fms/task_executor/-/merge_requests/23",
		Assignee:      "zhiyuan.liang",
		Status:        "progress",
		TaskCreatedAt: now.AddDate(0, 0, -8),
		LastUpdate:    now.Add(-time.Hour),
	}}

	response := buildDailyJiraAuditResponse(tasks, nil, nil, nil, now)
	if response.Summary.Total != 0 {
		t.Fatalf("Daily Jira included GitLab MR telemetry as a Jira issue: %+v", response.Buckets)
	}
}

func TestDailyJiraAuditReadsOnlyTheBoundedProjectionPage(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "daily-jira-query@westwell-lab.com", "Daily Jira Query", []string{"decision:read"})

	now := time.Now()
	tasks := make([]db.TaskTelemetry, 0, 350)
	for index := 0; index < 200; index++ {
		tasks = append(tasks, db.TaskTelemetry{
			TaskID: fmt.Sprintf("HISTORY-%d", 10000+index), ProjectKey: "HISTORY", Source: "jira",
			Title: "Resolved historical Jira row that must not be materialized", Status: "done",
			IssueType: "bug", TaskCreatedAt: now.Add(-30 * 24 * time.Hour), LastUpdate: now,
		})
	}
	for index := 0; index < 150; index++ {
		tasks = append(tasks, db.TaskTelemetry{
			TaskID: fmt.Sprintf("ACTIVE-%d", 20000+index), ProjectKey: "ACTIVE", Source: "jira",
			Title: "Unresolved Jira row", Status: "progress", IssueType: "bug",
			TaskCreatedAt: now.Add(-8 * 24 * time.Hour), LastUpdate: now,
		})
	}
	if err := db.DB.CreateInBatches(tasks, 100).Error; err != nil {
		t.Fatalf("seed Daily Jira query rows: %v", err)
	}
	for index := 0; index < 12; index++ {
		if err := db.DB.Create(&db.DecisionEvent{
			TaskID: "ACTIVE-20000", Action: "daily_jira_follow_up", Actor: "PM",
			Reason: fmt.Sprintf("history %d", index), CreatedAt: now.Add(time.Duration(index) * time.Minute),
		}).Error; err != nil {
			t.Fatalf("seed bounded Daily Jira event: %v", err)
		}
		if err := db.DB.Create(&db.DailyJiraDecision{
			TaskID: "ACTIVE-20000", Status: "follow_up", Actor: "PM",
			Note: fmt.Sprintf("decision %d", index), CreatedAt: now.Add(time.Duration(index) * time.Minute),
		}).Error; err != nil {
			t.Fatalf("seed bounded Daily Jira decision: %v", err)
		}
	}
	events, decisions, err := loadDailyJiraPageEnrichments([]string{"ACTIVE-20000"})
	if err != nil {
		t.Fatalf("load bounded Daily Jira enrichment: %v", err)
	}
	if len(events) != 8 || len(decisions) != 1 || decisions[0].Note != "decision 11" {
		t.Fatalf("bounded enrichment events/decisions = %d/%+v", len(events), decisions)
	}

	server := NewServer(&config.Config{}, "")
	request := httptest.NewRequest(http.MethodGet, "/api/decision/daily-jira?bucket=seven_day&limit=25", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	server.mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("Daily Jira query status = %d body %s", response.Code, response.Body.String())
	}

	var audit dailyJiraAuditResponse
	if err := json.NewDecoder(response.Body).Decode(&audit); err != nil {
		t.Fatalf("decode bounded Daily Jira response: %v", err)
	}
	if audit.Summary.Total != 150 || audit.Summary.SevenDay != 150 {
		t.Fatalf("Daily Jira summary = %+v, want 150 unresolved rows", audit.Summary)
	}
	sevenDay := audit.Buckets[2]
	if len(sevenDay.Items) != 25 || sevenDay.Count != 150 {
		t.Fatalf("Daily Jira page rows/count = %d/%d, want 25/150", len(sevenDay.Items), sevenDay.Count)
	}
	if !audit.Page.HasMore || audit.Page.NextCursor == "" || audit.Page.Limit != 25 || audit.Page.SearchMode == "" {
		t.Fatalf("Daily Jira page metadata is incomplete: %+v", audit.Page)
	}
	firstPageTaskIDs := make([]string, len(sevenDay.Items))
	for index, item := range sevenDay.Items {
		firstPageTaskIDs[index] = item.TaskID
	}

	nextRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/decision/daily-jira?bucket=seven_day&limit=25&cursor="+audit.Page.NextCursor,
		nil,
	)
	nextRequest.Header.Set("Authorization", "Bearer "+token)
	nextResponse := httptest.NewRecorder()
	server.mux.ServeHTTP(nextResponse, nextRequest)
	if nextResponse.Code != http.StatusOK {
		t.Fatalf("Daily Jira next-page status = %d body %s", nextResponse.Code, nextResponse.Body.String())
	}
	var nextAudit dailyJiraAuditResponse
	if err := json.NewDecoder(nextResponse.Body).Decode(&nextAudit); err != nil {
		t.Fatalf("decode Daily Jira next page: %v", err)
	}
	if !nextAudit.Page.HasPrevious || nextAudit.Page.PreviousCursor == "" {
		t.Fatalf("Daily Jira next page did not expose a previous cursor: %+v", nextAudit.Page)
	}

	previousRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/decision/daily-jira?bucket=seven_day&limit=25&direction=previous&cursor="+nextAudit.Page.PreviousCursor,
		nil,
	)
	previousRequest.Header.Set("Authorization", "Bearer "+token)
	previousResponse := httptest.NewRecorder()
	server.mux.ServeHTTP(previousResponse, previousRequest)
	if previousResponse.Code != http.StatusOK {
		t.Fatalf("Daily Jira previous-page status = %d body %s", previousResponse.Code, previousResponse.Body.String())
	}
	var previousAudit dailyJiraAuditResponse
	if err := json.NewDecoder(previousResponse.Body).Decode(&previousAudit); err != nil {
		t.Fatalf("decode Daily Jira previous page: %v", err)
	}
	previousItems := previousAudit.Buckets[2].Items
	if len(previousItems) != len(firstPageTaskIDs) {
		t.Fatalf("Daily Jira previous-page row count = %d, want %d", len(previousItems), len(firstPageTaskIDs))
	}
	for index, item := range previousItems {
		if item.TaskID != firstPageTaskIDs[index] {
			t.Fatalf("Daily Jira previous-page item %d = %s, want %s", index, item.TaskID, firstPageTaskIDs[index])
		}
	}
}

func TestResolvedDailyJiraStatusMatchesDatabaseReadPredicate(t *testing.T) {
	for _, status := range []string{"done", "archived", "resolved", "closed", "completed", "已完成", "已关闭"} {
		if !isResolvedDailyJiraStatus(status) {
			t.Fatalf("status %q should be treated as resolved", status)
		}
	}
	for _, status := range []string{"", "todo", "progress", "review", "待处理"} {
		if isResolvedDailyJiraStatus(status) {
			t.Fatalf("status %q should remain in Daily Jira", status)
		}
	}
}

func TestDailyJiraManualSyncRefreshesAssigneeCommentsAndListMembership(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "daily-jira-sync@westwell-lab.com", "Daily Jira Sync", []string{"decision:read"})

	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })

	now := time.Now()
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID: "NS2-2262", ProjectKey: "NS2", Source: "jira", ExternalKey: "NS2-2262",
		Title: "Stale Daily Jira item", Repo: "Nansha (NS2)", Assignee: "梁志远",
		Status: "progress", IssueType: "bug", TaskCreatedAt: now.Add(-8 * 24 * time.Hour),
		LastUpdate: now.Add(-2 * time.Hour), SourceUpdatedAt: now.Add(-2 * time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed stale Daily Jira item: %v", err)
	}

	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/search":
			jql := r.URL.Query().Get("jql")
			if strings.Contains(jql, "updated >=") && strings.Contains(jql, `"NS2"`) {
				fmt.Fprint(w, `{"total":1,"issues":[{"key":"NS2-2262","fields":{"summary":"Stale Daily Jira item","created":"2026-08-11T08:00:00.000+0800","issuetype":{"name":"Bug"},"assignee":{"name":"external.user","displayName":"外部协作方"},"reporter":{"name":"reporter.user","displayName":"问题报告人"},"status":{"name":"Open"},"project":{"key":"NS2","name":"Nansha"},"fixVersions":[],"versions":[],"updated":"2026-08-19T22:30:00.000+0800"}}]}`)
				return
			}
			fmt.Fprint(w, `{"total":0,"issues":[]}`)
		case strings.HasSuffix(r.URL.Path, "/comment"):
			fmt.Fprint(w, `{"comments":[{"id":"ns2-2262-comment","author":{"displayName":"测试人员"},"body":"负责人已调整，请按评论决策处理","created":"2026-08-19T22:29:00.000+0800","updated":"2026-08-19T22:29:00.000+0800"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer jira.Close()

	server := NewServer(&config.Config{Jira: config.JiraConfig{
		Enabled: true, BaseURL: jira.URL, SyncProjects: []string{"NS2"}, SyncUsers: []string{"梁志远"},
		CustomJQL: `project = NS2 AND assignee in ("梁志远")`,
	}}, "")
	syncRequest := httptest.NewRequest(http.MethodPost, "/api/decision/daily-jira/sync", nil)
	syncRequest.Header.Set("Authorization", "Bearer "+token)
	syncResponse := httptest.NewRecorder()
	server.mux.ServeHTTP(syncResponse, syncRequest)
	if syncResponse.Code != http.StatusOK {
		t.Fatalf("manual Daily Jira sync status = %d body %s", syncResponse.Code, syncResponse.Body.String())
	}

	var refreshed db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", "NS2-2262").First(&refreshed).Error; err != nil {
		t.Fatalf("query refreshed NS2-2262: %v", err)
	}
	if refreshed.Assignee != "外部协作方" {
		t.Fatalf("NS2-2262 assignee = %q, want 外部协作方", refreshed.Assignee)
	}
	if refreshed.JiraReporter != "问题报告人" || refreshed.JiraReporterUser != "reporter.user" {
		t.Fatalf("NS2-2262 reporter projection = %q/%q", refreshed.JiraReporter, refreshed.JiraReporterUser)
	}
	var comment db.JiraCommentLog
	if err := db.DB.Where("task_id = ? AND comment_id = ?", "NS2-2262", "ns2-2262-comment").First(&comment).Error; err != nil {
		t.Fatalf("query refreshed NS2-2262 comment: %v", err)
	}
	if comment.Body != "负责人已调整，请按评论决策处理" || !comment.Current {
		t.Fatalf("unexpected refreshed Jira comment: %+v", comment)
	}

	auditRequest := httptest.NewRequest(http.MethodGet, "/api/decision/daily-jira", nil)
	auditRequest.Header.Set("Authorization", "Bearer "+token)
	auditResponse := httptest.NewRecorder()
	server.mux.ServeHTTP(auditResponse, auditRequest)
	if auditResponse.Code != http.StatusOK {
		t.Fatalf("Daily Jira audit status = %d body %s", auditResponse.Code, auditResponse.Body.String())
	}
	var audit dailyJiraAuditResponse
	if err := json.NewDecoder(auditResponse.Body).Decode(&audit); err != nil {
		t.Fatalf("decode Daily Jira audit: %v", err)
	}
	if audit.Summary.Total != 0 {
		t.Fatalf("NS2-2262 remained in Daily Jira after manual sync: %+v", audit.Summary)
	}
}

func TestDailyJiraVisibilityKeepsUnassignedAndExcludesExternalAssignees(t *testing.T) {
	server := NewServer(&config.Config{Jira: config.JiraConfig{SyncUsers: []string{"Alice"}}}, "")
	visibility := coreMemberVisibility{
		filter:    server.buildKPICoreMemberFilter(nil),
		directory: newKPIUserDirectory(nil),
	}
	if !isDailyJiraAssigneeVisible(visibility, "") || !isDailyJiraAssigneeVisible(visibility, "未指派") {
		t.Fatal("unassigned Jira must remain visible for morning assignment")
	}
	if !isDailyJiraAssigneeVisible(visibility, "Alice") {
		t.Fatal("configured core assignee should be visible")
	}
	if isDailyJiraAssigneeVisible(visibility, "External Vendor") {
		t.Fatal("external assignee should remain outside the daily Jira audit scope")
	}
}

func TestPostDailyJiraReviewPersistsDecisionAndReassignment(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}

	tmpDir := t.TempDir()
	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(tmpDir, "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })

	originalUpdate := time.Date(2026, time.July, 17, 8, 0, 0, 0, time.Local)
	task := db.TaskTelemetry{
		TaskID:        "WA-207",
		Title:         "Seven day unresolved Jira",
		Repo:          "Well Ambient (WA)",
		Assignee:      "Alice",
		Status:        "progress",
		IssueType:     "bug",
		TaskCreatedAt: time.Date(2026, time.July, 11, 8, 0, 0, 0, time.Local),
		LastUpdate:    originalUpdate,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if err := kanban.SyncTaskToKanban(&task); err != nil {
		t.Fatalf("seed kanban: %v", err)
	}

	server := NewServer(&config.Config{}, "")
	body := bytes.NewBufferString(`{"task_id":"WA-207","decision":"reassign","assignee":"Bob","note":"早会确认由 Bob 接手"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/decision/daily-jira/review", body)
	request.Header.Set("x-authenticated-user-id", "pm@westwell-lab.com")
	request.Header.Set("x-authenticated-user-name", "PM")
	response := httptest.NewRecorder()
	server.handlePostDailyJiraReview(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("review status = %d body %s", response.Code, response.Body.String())
	}

	var updated db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", task.TaskID).First(&updated).Error; err != nil {
		t.Fatalf("query updated task: %v", err)
	}
	if updated.Assignee != "Bob" || !strings.Contains(updated.DecisionLogs, "每日 Jira 审计：转派给 Bob") {
		t.Fatalf("unexpected updated task: %+v", updated)
	}

	var event db.DecisionEvent
	if err := db.DB.Where("task_id = ?", task.TaskID).First(&event).Error; err != nil {
		t.Fatalf("query decision event: %v", err)
	}
	if event.Action != "daily_jira_reassign" || event.Actor != "PM" || event.OldValue != "Alice" || event.NewValue != "Bob" || event.Reason != "早会确认由 Bob 接手" {
		t.Fatalf("unexpected event: %+v", event)
	}
	var decision db.DailyJiraDecision
	if err := db.DB.Where("task_id = ?", task.TaskID).First(&decision).Error; err != nil {
		t.Fatalf("query decision record: %v", err)
	}
	if decision.Status != "reassign" || decision.Assignee != "Bob" || decision.Actor != "PM" || decision.Note != "早会确认由 Bob 接手" {
		t.Fatalf("unexpected decision record: %+v", decision)
	}
	if reminderDelay := decision.ReminderAt.Sub(decision.CreatedAt); reminderDelay != 24*time.Hour {
		t.Fatalf("reassign reminder delay = %s, want 24h", reminderDelay)
	}

	kanbanContent, err := os.ReadFile(kanban.KanbanFilePath)
	if err != nil {
		t.Fatalf("read kanban: %v", err)
	}
	if !strings.Contains(string(kanbanContent), "| WA-207 | Seven day unresolved Jira | Well Ambient (WA) | Bob |") {
		t.Fatalf("kanban did not contain reassignment:\n%s", string(kanbanContent))
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload["status"] != "success" {
		t.Fatalf("unexpected response payload: %v err=%v", payload, err)
	}
}

func TestPostDailyJiraReviewRequiresDecisionComment(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}
	task := db.TaskTelemetry{
		TaskID: "WA-208", Title: "Decision comment required", Repo: "Well Ambient (WA)",
		Assignee: "Alice", Status: "progress", IssueType: "bug",
		TaskCreatedAt: time.Now().Add(-8 * 24 * time.Hour), LastUpdate: time.Now().Add(-time.Hour),
	}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	server := NewServer(&config.Config{}, "")
	request := httptest.NewRequest(http.MethodPost, "/api/decision/daily-jira/review", bytes.NewBufferString(`{"task_id":"WA-208","decision":"reassign","assignee":"Bob","note":"   "}`))
	request.Header.Set("x-authenticated-user-name", "PM")
	response := httptest.NewRecorder()
	server.handlePostDailyJiraReview(response, request)

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "comment") {
		t.Fatalf("blank decision comment status = %d body %q, want 400 comment error", response.Code, response.Body.String())
	}
	var decisionCount int64
	if err := db.DB.Model(&db.DailyJiraDecision{}).Where("task_id = ?", task.TaskID).Count(&decisionCount).Error; err != nil {
		t.Fatalf("count decisions: %v", err)
	}
	if decisionCount != 0 {
		t.Fatalf("blank comment persisted %d decisions, want 0", decisionCount)
	}
}

func TestPostDailyJiraReviewUpdatesAssigneeAndCommentInOneJiraRequest(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}
	task := db.TaskTelemetry{
		TaskID: "WA-209", Title: "Atomic Jira decision update", Repo: "Well Ambient (WA)",
		Assignee: "Alice", Status: "progress", IssueType: "bug",
		TaskCreatedAt: time.Now().Add(-8 * 24 * time.Hour), LastUpdate: time.Now().Add(-time.Hour),
	}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	type jiraUpdate struct {
		Path string
		Body map[string]interface{}
	}
	updates := make(chan jiraUpdate, 2)
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		updates <- jiraUpdate{Path: r.URL.Path, Body: payload}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer jira.Close()

	server := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: true, BaseURL: jira.URL}}, "")
	request := httptest.NewRequest(http.MethodPost, "/api/decision/daily-jira/review", bytes.NewBufferString(`{"task_id":"WA-209","decision":"reassign","assignee":"Bob","note":"请 Bob 接手并在今天反馈处理结论"}`))
	request.Header.Set("x-authenticated-user-name", "PM")
	response := httptest.NewRecorder()
	server.handlePostDailyJiraReview(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("review status = %d body %s", response.Code, response.Body.String())
	}

	select {
	case update := <-updates:
		if update.Path != "/rest/api/2/issue/WA-209" {
			t.Fatalf("Jira update path = %q, want atomic issue update path", update.Path)
		}
		fields, _ := update.Body["fields"].(map[string]interface{})
		assignee, _ := fields["assignee"].(map[string]interface{})
		if assignee["name"] != "Bob" {
			t.Fatalf("Jira assignee payload = %#v, want Bob", assignee)
		}
		updatePayload, _ := update.Body["update"].(map[string]interface{})
		if _, ok := updatePayload["comment"]; !ok {
			t.Fatalf("Jira update payload omitted decision comment: %#v", update.Body)
		}
	case <-time.After(time.Second):
		t.Fatal("daily Jira review did not write Jira")
	}
}

func TestPostDailyJiraReviewCanReturnIssueToReporter(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}
	task := db.TaskTelemetry{
		TaskID: "NS2-2262", Title: "Return to reporter", Repo: "Nansha (NS2)", Source: "jira",
		Assignee: "梁志远", JiraReporter: "问题报告人", JiraReporterUser: "reporter.user",
		Status: "progress", IssueType: "bug", TaskCreatedAt: time.Now().Add(-8 * 24 * time.Hour),
		LastUpdate: time.Now().Add(-time.Hour),
	}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	server := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: true}}, "")
	var syncedAssignee *string
	var syncedComment string
	server.jiraDailyReviewSync = func(issueKey string, assignee *string, comment string) error {
		if issueKey != task.TaskID {
			t.Fatalf("Jira issue key = %q, want %q", issueKey, task.TaskID)
		}
		syncedAssignee = assignee
		syncedComment = comment
		return nil
	}
	request := httptest.NewRequest(http.MethodPost, "/api/decision/daily-jira/review", bytes.NewBufferString(`{"task_id":"NS2-2262","decision":"reassign","assignee_mode":"reporter","note":"现有信息不足，请报告人补充复现步骤"}`))
	request.Header.Set("x-authenticated-user-name", "PM")
	response := httptest.NewRecorder()
	server.handlePostDailyJiraReview(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("return-to-reporter status = %d body %s", response.Code, response.Body.String())
	}
	if syncedAssignee == nil || *syncedAssignee != "reporter.user" {
		t.Fatalf("synced reporter username = %#v, want reporter.user", syncedAssignee)
	}
	if !strings.Contains(syncedComment, "现有信息不足，请报告人补充复现步骤") {
		t.Fatalf("Jira comment omitted decision content: %q", syncedComment)
	}
	var updated db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", task.TaskID).First(&updated).Error; err != nil {
		t.Fatalf("query returned task: %v", err)
	}
	if updated.Assignee != "问题报告人" {
		t.Fatalf("local assignee = %q, want 问题报告人", updated.Assignee)
	}
}

func TestPostDailyJiraReviewDoesNotCommitLocalDecisionWhenJiraUpdateFails(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}
	task := db.TaskTelemetry{
		TaskID: "WA-210", Title: "Failed Jira update", Repo: "Well Ambient (WA)", Source: "jira",
		Assignee: "Alice", Status: "progress", IssueType: "bug",
		TaskCreatedAt: time.Now().Add(-8 * 24 * time.Hour), LastUpdate: time.Now().Add(-time.Hour),
	}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	server := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: true}}, "")
	server.jiraDailyReviewSync = func(string, *string, string) error {
		return fmt.Errorf("Jira is unavailable")
	}
	request := httptest.NewRequest(http.MethodPost, "/api/decision/daily-jira/review", bytes.NewBufferString(`{"task_id":"WA-210","decision":"reassign","assignee":"Bob","note":"请 Bob 接手"}`))
	request.Header.Set("x-authenticated-user-name", "PM")
	response := httptest.NewRecorder()
	server.handlePostDailyJiraReview(response, request)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("failed Jira update status = %d body %s, want 502", response.Code, response.Body.String())
	}
	var unchanged db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", task.TaskID).First(&unchanged).Error; err != nil {
		t.Fatalf("query unchanged task: %v", err)
	}
	if unchanged.Assignee != "Alice" {
		t.Fatalf("failed Jira update changed local assignee to %q", unchanged.Assignee)
	}
	var decisionCount int64
	if err := db.DB.Model(&db.DailyJiraDecision{}).Where("task_id = ?", task.TaskID).Count(&decisionCount).Error; err != nil {
		t.Fatalf("count decisions: %v", err)
	}
	if decisionCount != 0 {
		t.Fatalf("failed Jira update persisted %d local decisions", decisionCount)
	}
}

func TestPostDailyJiraReviewRecordsFollowUpWithoutResettingActivity(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}

	tmpDir := t.TempDir()
	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(tmpDir, "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })

	originalUpdate := time.Date(2026, time.July, 14, 8, 0, 0, 0, time.Local)
	task := db.TaskTelemetry{
		TaskID:        "WA-303",
		Title:         "Three day follow up",
		Repo:          "Well Ambient (WA)",
		Assignee:      "Alice",
		Status:        "progress",
		IssueType:     "demand",
		TaskCreatedAt: originalUpdate,
		LastUpdate:    originalUpdate,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	server := NewServer(&config.Config{}, "")
	request := httptest.NewRequest(http.MethodPost, "/api/decision/daily-jira/review", bytes.NewBufferString(`{"task_id":"WA-303","decision":"follow_up","note":"下午同步联调结果"}`))
	request.Header.Set("x-authenticated-user-name", "PM")
	response := httptest.NewRecorder()
	server.handlePostDailyJiraReview(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("review status = %d body %s", response.Code, response.Body.String())
	}

	var updated db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", task.TaskID).First(&updated).Error; err != nil {
		t.Fatalf("query updated task: %v", err)
	}
	if !updated.LastUpdate.Equal(originalUpdate) {
		t.Fatalf("follow-up reset Jira activity: got %s want %s", updated.LastUpdate, originalUpdate)
	}
	if !strings.Contains(updated.DecisionLogs, "结论 [继续跟进]") && !strings.Contains(updated.DecisionLogs, "每日 Jira 审计：继续跟进") {
		t.Fatalf("follow-up decision log missing: %s", updated.DecisionLogs)
	}
	var decision db.DailyJiraDecision
	if err := db.DB.Where("task_id = ?", task.TaskID).First(&decision).Error; err != nil {
		t.Fatalf("query follow-up decision: %v", err)
	}
	if decision.Status != "follow_up" || decision.ReminderAt.Sub(decision.CreatedAt) != 24*time.Hour {
		t.Fatalf("unexpected follow-up decision reminder: %+v", decision)
	}
}

func TestDailyJiraEscalationUsesShorterReminderWindow(t *testing.T) {
	if got := dailyJiraReminderDelay("escalate"); got != 4*time.Hour {
		t.Fatalf("escalation reminder = %s, want 4h", got)
	}
	if got := dailyJiraReminderDelay("follow_up"); got != 24*time.Hour {
		t.Fatalf("follow-up reminder = %s, want 24h", got)
	}
	if got := dailyJiraReminderDelay("reassign"); got != 24*time.Hour {
		t.Fatalf("reassign reminder = %s, want 24h", got)
	}
}

func TestDailyJiraReminderAlertsUseLatestUnresolvedDecision(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}
	now := time.Now()
	tasks := []db.TaskTelemetry{
		{TaskID: "WA-401", Title: "new decision wins", Assignee: "Alice", Status: "progress"},
		{TaskID: "WA-402", Title: "resolved Jira", Assignee: "Bob", Status: "done"},
		{TaskID: "WA-403", Title: "escalation due", Assignee: "Carol", Status: "review"},
	}
	for _, task := range tasks {
		if err := db.DB.Create(&task).Error; err != nil {
			t.Fatalf("seed task %s: %v", task.TaskID, err)
		}
	}
	decisions := []db.DailyJiraDecision{
		{TaskID: "WA-401", Status: "follow_up", ReminderAt: now.Add(-time.Hour), CreatedAt: now.Add(-25 * time.Hour)},
		{TaskID: "WA-401", Status: "reassign", ReminderAt: now.Add(time.Hour), CreatedAt: now.Add(-time.Hour)},
		{TaskID: "WA-402", Status: "follow_up", ReminderAt: now.Add(-time.Hour), CreatedAt: now.Add(-25 * time.Hour)},
		{TaskID: "WA-403", Status: "escalate", Note: "等待平台团队响应", ReminderAt: now.Add(-time.Minute), CreatedAt: now.Add(-4*time.Hour - time.Minute)},
	}
	if err := db.DB.Create(&decisions).Error; err != nil {
		t.Fatalf("seed decisions: %v", err)
	}

	server := NewServer(&config.Config{}, "")
	alerts := server.computeDailyJiraDecisionAlerts()
	if len(alerts) != 1 {
		t.Fatalf("alerts = %+v, want one due latest unresolved decision", alerts)
	}
	if alerts[0].TaskID != "WA-403" || alerts[0].Type != "daily_jira_reminder" || alerts[0].Severity != "critical" || alerts[0].Status != "escalate" {
		t.Fatalf("unexpected reminder alert: %+v", alerts[0])
	}
	if !strings.Contains(alerts[0].Message, "等待平台团队响应") {
		t.Fatalf("reminder does not contain decision context: %s", alerts[0].Message)
	}
}

func TestDailyJiraReminderCanBeDismissedPerUser(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}
	task := db.TaskTelemetry{TaskID: "WA-404", Title: "dismiss reminder", Assignee: "Alice", Status: "progress"}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	decision := db.DailyJiraDecision{
		TaskID:     task.TaskID,
		Status:     "follow_up",
		Assignee:   task.Assignee,
		ReminderAt: time.Now().Add(-time.Minute),
		CreatedAt:  time.Now().Add(-24*time.Hour - time.Minute),
	}
	if err := db.DB.Create(&decision).Error; err != nil {
		t.Fatalf("seed decision: %v", err)
	}

	server := NewServer(&config.Config{}, "")
	if alerts := server.getMergedNotifications("pm@westwell-lab.com"); len(alerts) != 1 || alerts[0].Type != "daily_jira_reminder" {
		t.Fatalf("expected visible Jira reminder before dismissal, got %+v", alerts)
	}
	state := db.UserNotificationState{
		NotificationKey: fmt.Sprintf("daily_jira_reminder_%d", decision.ID),
		UserID:          "pm@westwell-lab.com",
		Status:          "dismissed",
		UpdatedAt:       time.Now(),
	}
	if err := db.DB.Create(&state).Error; err != nil {
		t.Fatalf("seed dismissed state: %v", err)
	}
	if alerts := server.getMergedNotifications("pm@westwell-lab.com"); len(alerts) != 0 {
		t.Fatalf("dismissed Jira reminder remained visible: %+v", alerts)
	}
}
