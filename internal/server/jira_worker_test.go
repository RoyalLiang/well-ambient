package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/deliveryplanning"
	"well-ambient/internal/kanban"
	"well-ambient/internal/performance"
	"well-ambient/internal/solutions"
	"well-ambient/internal/telemetry"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestJiraInboundWorkerContinuesWhilePerformanceHistorySyncIsBlocked(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inboundRuns := make(chan struct{}, 8)
	performanceStarted := make(chan struct{}, 1)
	releasePerformance := make(chan struct{})
	defer close(releasePerformance)

	startIndependentJiraWorkers(
		ctx,
		5*time.Millisecond,
		5*time.Millisecond,
		func() {
			select {
			case inboundRuns <- struct{}{}:
			default:
			}
		},
		func() {
			select {
			case performanceStarted <- struct{}{}:
			default:
			}
			<-releasePerformance
		},
	)

	select {
	case <-performanceStarted:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("performance Jira history worker did not start")
	}

	for run := 1; run <= 3; run++ {
		select {
		case <-inboundRuns:
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("Jira inbound sync stopped at run %d while performance history was blocked", run)
		}
	}
}

func TestMapJiraStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"To Do", "backlog"},
		{"backlog", "backlog"},
		{"Open", "backlog"},
		{"reopened", "backlog"},
		{"NEW", "backlog"},
		{"todo", "backlog"},
		{"In Progress", "progress"},
		{"progress", "progress"},
		{"active", "progress"},
		{"doing", "progress"},
		{"In Review", "review"},
		{"review", "review"},
		{"under review", "review"},
		{"qa", "review"},
		{"testing", "review"},
		{"Done", "done"},
		{"closed", "done"},
		{"resolved", "done"},
		{"completed", "done"},
		{"unknown status", "backlog"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			actual := mapJiraStatus(tc.input)
			if actual != tc.expected {
				t.Errorf("mapJiraStatus(%q) = %q; want %q", tc.input, actual, tc.expected)
			}
		})
	}
}

func TestMapJiraIssueType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Task", "requirement"},
		{"任务", "requirement"},
		{"Story", "requirement"},
		{"需求", "requirement"},
		{"Feature", "requirement"},
		{"Epic", "requirement"},
		{"Bug", "bug"},
		{"缺陷", "bug"},
		{"故障", "bug"},
		{"Defect", "bug"},
		{"unknown", "requirement"},
		{"", "requirement"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			actual := mapJiraIssueType(tc.input)
			if actual != tc.expected {
				t.Errorf("mapJiraIssueType(%q) = %q; want %q", tc.input, actual, tc.expected)
			}
		})
	}
}

func TestBuildJQL(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.JiraConfig
		expected string
	}{
		{
			name:     "empty config",
			cfg:      config.JiraConfig{},
			expected: "",
		},
		{
			name: "only projects",
			cfg: config.JiraConfig{
				SyncProjects: []string{"PROJ", "TEST"},
			},
			expected: `project in ("PROJ", "TEST")`,
		},
		{
			name: "only users",
			cfg: config.JiraConfig{
				SyncUsers: []string{"eddie@company.com", "dev@company.com"},
			},
			expected: `assignee in ("eddie@company.com", "dev@company.com")`,
		},
		{
			name: "only statuses",
			cfg: config.JiraConfig{
				SyncStatuses: []string{"To Do", "In Progress"},
			},
			expected: `status in ("To Do", "In Progress")`,
		},
		{
			name: "all filters combined",
			cfg: config.JiraConfig{
				SyncProjects: []string{"PROJ"},
				SyncUsers:    []string{"eddie@company.com"},
				SyncStatuses: []string{"In Progress"},
			},
			expected: `project in ("PROJ") AND assignee in ("eddie@company.com") AND status in ("In Progress")`,
		},
		{
			name: "custom JQL overrides other filters",
			cfg: config.JiraConfig{
				SyncProjects: []string{"PROJ"},
				SyncUsers:    []string{"eddie@company.com"},
				SyncStatuses: []string{"In Progress"},
				CustomJQL:    "project = MYPROJ AND type = Bug",
			},
			expected: "project = MYPROJ AND type = Bug",
		},
		{
			name: "whitespace trimming",
			cfg: config.JiraConfig{
				SyncProjects: []string{" PROJ  ", ""},
				SyncUsers:    []string{" eddie@company.com "},
			},
			expected: `project in ("PROJ") AND assignee in ("eddie@company.com")`,
		},
		{
			name: "only Jira version source",
			cfg: config.JiraConfig{
				BaseURL: "https://jira.example.com",
				VersionSources: []config.JiraVersionSource{{
					ProjectKey:  "PRJ25024",
					ProjectName: "Release Train",
					VersionURL:  "https://jira.example.com/projects/PRJ25024/versions/13622",
				}},
			},
			expected: `(project = "PRJ25024" AND fixVersion = 13622)`,
		},
		{
			name: "ordinary filters and versions are additive",
			cfg: config.JiraConfig{
				BaseURL:      "https://jira.example.com",
				SyncProjects: []string{"OPS"},
				VersionSources: []config.JiraVersionSource{{
					ProjectKey:  "PRJ25024",
					ProjectName: "Release Train",
					VersionURL:  "https://jira.example.com/projects/PRJ25024/versions/13622",
				}},
			},
			expected: `(project in ("OPS")) OR (project = "PRJ25024" AND fixVersion = 13622)`,
		},
		{
			name: "custom JQL and versions are additive",
			cfg: config.JiraConfig{
				BaseURL:   "https://jira.example.com",
				CustomJQL: "project = OPS AND type = Bug",
				VersionSources: []config.JiraVersionSource{{
					ProjectKey:  "PRJ25024",
					ProjectName: "Release Train",
					VersionURL:  "https://jira.example.com/projects/PRJ25024/versions/13622",
				}},
			},
			expected: `(project = OPS AND type = Bug) OR (project = "PRJ25024" AND fixVersion = 13622)`,
		},
		{
			name: "invalid version source is ignored",
			cfg: config.JiraConfig{
				BaseURL: "https://jira.example.com",
				VersionSources: []config.JiraVersionSource{{
					ProjectKey:  "PRJ25024",
					ProjectName: "Release Train",
					VersionURL:  "https://jira.example.com/browse/PRJ25024-1",
				}},
			},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := buildJQL(&tc.cfg)
			if actual != tc.expected {
				t.Errorf("buildJQL(...) = %q; want %q", actual, tc.expected)
			}
		})
	}
}

func TestBuildPerformanceJiraHistoryJQLUsesAssessmentScope(t *testing.T) {
	cfg := &config.Config{
		Jira: config.JiraConfig{
			Enabled:   true,
			CustomJQL: `project in (WA, HIT) AND issuetype in (Bug, Task) AND status not in (done) AND assignee in (Alice, Bob)`,
		},
		PerformanceBrain: config.PerformanceBrainConfig{Enabled: true, AssessmentWindowDays: 90},
	}
	jql := buildPerformanceJiraHistoryJQL(cfg, []string{"Alice", "Bob"})
	for _, expected := range []string{
		`project in ("WA", "HIT")`,
		`issuetype in ("Bug", "Task")`,
		`assignee in ("Alice", "Bob")`,
		`issuetype in ("Bug")`,
		`created >= -90d`,
		`updated >= -90d`,
		`statusCategory = Done`,
		`resolutiondate >= -90d`,
		`due >= -90d`,
		`due <= now()`,
	} {
		if !strings.Contains(jql, expected) {
			t.Fatalf("history JQL %q missing %q", jql, expected)
		}
	}
	if strings.Contains(strings.ToLower(jql), "status not in") {
		t.Fatalf("history JQL inherited the active-only exclusion: %q", jql)
	}
}

func TestSyncPerformanceJiraHistoryPersistsCompletionEvidence(t *testing.T) {
	setupServerTestDB(t)
	var receivedJQL string
	var receivedFields string
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/rest/api/2/search" {
			http.NotFound(w, r)
			return
		}
		receivedJQL = r.URL.Query().Get("jql")
		receivedFields = r.URL.Query().Get("fields")
		fmt.Fprint(w, `{"total":2,"issues":[{"key":"WA-808","changelog":{"histories":[{"id":"h-808","created":"2026-08-09T10:00:00.000+0800","author":{"displayName":"Jira Bot"},"items":[{"field":"assignee","fieldId":"assignee","fromString":"alice@example.com","toString":"Alice"},{"field":"status","fieldId":"status","fromString":"In Progress","toString":"Done"}]}]},"fields":{"summary":"Historical accepted demand","created":"2026-08-01T08:00:00.000+0800","resolutiondate":"2026-08-10T18:30:00.000+0800","duedate":"2026-08-11","timeoriginalestimate":57600,"issuetype":{"name":"Task"},"priority":{"name":"P1"},"assignee":{"name":"alice","displayName":"Alice","emailAddress":"alice@example.com"},"status":{"name":"Done"},"project":{"key":"WA","name":"Well Ambient"},"fixVersions":[],"versions":[],"updated":"2026-08-10T18:30:00.000+0800"}},{"key":"WA-809","fields":{"summary":"Historical overdue demand","created":"2026-07-01T08:00:00.000+0800","duedate":"2026-08-09","timeoriginalestimate":28800,"issuetype":{"name":"Task"},"priority":{"name":"P2"},"assignee":{"name":"alice","displayName":"Alice","emailAddress":"alice@example.com"},"status":{"name":"In Progress"},"project":{"key":"WA","name":"Well Ambient"},"fixVersions":[],"versions":[],"updated":"2026-08-12T18:30:00.000+0800"}}]}`)
	}))
	defer jira.Close()

	cfg := &config.Config{
		Jira:             config.JiraConfig{Enabled: true, BaseURL: jira.URL, SyncProjects: []string{"WA"}, SyncUsers: []string{"Alice"}},
		PerformanceBrain: config.PerformanceBrainConfig{Enabled: true, AssessmentWindowDays: 90},
	}
	server := NewServer(cfg, "")
	changed, err := server.syncPerformanceJiraHistory(telemetry.NewJiraClient(&cfg.Jira))
	if err != nil {
		t.Fatal(err)
	}
	if !changed || !strings.Contains(receivedJQL, "resolutiondate >= -90d") || !strings.Contains(receivedFields, "parent") || !strings.Contains(receivedFields, "issuelinks") {
		t.Fatalf("history sync did not execute its bounded evidence query: changed=%v jql=%q fields=%q", changed, receivedJQL, receivedFields)
	}
	var task db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", "WA-808").First(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.Source != "jira" || task.Status != "done" || task.IssueType != "requirement" || task.CompletedAt == nil || task.DueDate == nil || task.EstimateDays != 2 || task.Priority != "P1" || task.Severity != "P1" {
		t.Fatalf("historical Jira scoring fields were not persisted: %+v", task)
	}
	if task.CompletedAt.Format("2006-01-02 15:04") != "2026-08-10 18:30" || task.DueDate.Format("2006-01-02") != "2026-08-11" {
		t.Fatalf("historical Jira timestamps were not preserved: completed=%v due=%v", task.CompletedAt, task.DueDate)
	}
	var overdue db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", "WA-809").First(&overdue).Error; err != nil {
		t.Fatal(err)
	}
	if overdue.Status != "progress" || overdue.CompletedAt != nil || overdue.DueDate == nil || overdue.DueDate.Format("2006-01-02") != "2026-08-09" {
		t.Fatalf("due-eligible Jira denominator was not persisted: %+v", overdue)
	}
	var sourceEvents []db.PerformanceWorkItemEvent
	if err := db.DB.Where("work_item_id = ?", "WA-808").Order("occurred_at ASC, id ASC").Find(&sourceEvents).Error; err != nil {
		t.Fatal(err)
	}
	if len(sourceEvents) != 3 || sourceEvents[0].EventType != performance.SourceEventAssigneeChange || sourceEvents[1].EventType != performance.SourceEventStatusChange || sourceEvents[2].EventType != performance.SourceEventIssueSnapshot {
		t.Fatalf("historical Jira changelog/snapshot events = %+v", sourceEvents)
	}
	changedAgain, err := server.syncPerformanceJiraHistory(telemetry.NewJiraClient(&cfg.Jira))
	if err != nil {
		t.Fatal(err)
	}
	if changedAgain {
		t.Fatal("unchanged historical Jira evidence triggered another score refresh")
	}
}

func TestAppendJiraPerformanceEventsReplaysLegacySnapshotSchema(t *testing.T) {
	setupServerTestDB(t)

	var issue telemetry.JiraIssue
	if err := json.Unmarshal([]byte(`{
		"key":"WA-910",
		"fields":{
			"created":"2026-08-01T08:00:00.000+0800",
			"issuetype":{"name":"Bug"},
			"priority":{"name":"Medium"},
			"assignee":{"name":"alice","displayName":"Alice"},
			"status":{"name":"Awaiting fix"},
			"project":{"key":"WA","name":"Well Ambient"},
			"parent":{"key":"WA-900","fields":{"issuetype":{"name":"Task"}}},
			"updated":"2026-08-14T06:51:37.000+0000"
		}
	}`), &issue); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{PerformanceBrain: config.PerformanceBrainConfig{Enabled: true}}
	server := NewServer(cfg, "")
	snapshotID := strings.TrimSpace(issue.Fields.Updated)
	updatedAt := parseOptionalJiraTime(snapshotID)
	legacyPayload := map[string]any{
		"status": issue.Fields.Status.Name, "assignee": jiraIssueAssigneeName(issue),
		"priority": issue.Fields.Priority.Name, "due_date": issue.Fields.DueDate,
		"resolution_date": issue.Fields.ResolutionDate, "issue_type": mapJiraIssueType(issue.Fields.IssueType.Name),
	}
	if _, err := server.performance.AppendSourceEvent(context.Background(), performance.SourceEventCommand{
		DedupeKey:  "jira:" + issue.Key + ":snapshot:" + snapshotID,
		WorkItemID: issue.Key, ProjectKey: issue.Fields.Project.Key, IssueType: "bug",
		EventType: performance.SourceEventIssueSnapshot, OccurredAt: updatedAt,
		SourceSystem: "jira", SourceEventID: issue.Key + ":" + snapshotID,
		Actor: "jira-sync", Payload: legacyPayload,
	}); err != nil {
		t.Fatalf("seed legacy Jira performance snapshot: %v", err)
	}

	changed, err := server.appendJiraPerformanceEvents(issue)
	if err != nil {
		t.Fatalf("legacy immutable Jira snapshot blocked current synchronization: %v", err)
	}
	if changed {
		t.Fatal("legacy Jira snapshot replay was reported as a new source event")
	}
	var eventCount int64
	if err := db.DB.Model(&db.PerformanceWorkItemEvent{}).
		Where("work_item_id = ? AND event_type = ?", issue.Key, performance.SourceEventIssueSnapshot).
		Count(&eventCount).Error; err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 {
		t.Fatalf("legacy snapshot replay created %d immutable rows, want 1", eventCount)
	}
}

func TestAppendJiraPerformanceEventsReplaysHistoryAfterAuthorDisplayNameChanges(t *testing.T) {
	setupServerTestDB(t)

	var issue telemetry.JiraIssue
	if err := json.Unmarshal([]byte(`{
		"key":"FZ-2257",
		"fields":{
			"issuetype":{"name":"Task"},
			"priority":{"name":"Highest"},
			"assignee":{"name":"liang.zhiyuan","displayName":"梁志远"},
			"status":{"name":"To Do"},
			"project":{"key":"FZ","name":"Fuzhou"},
			"updated":"2026-07-29T08:59:27.000+0000"
		},
		"changelog":{"histories":[{
			"id":"2964008",
			"created":"2026-07-07T09:59:14.670+0800",
			"author":{"name":"wuhaiyang","displayName":"武海洋 [X]"},
			"items":[{"field":"Priority","fieldId":"","fromString":"Medium","toString":"Highest"}]
		}]}
	}`), &issue); err != nil {
		t.Fatal(err)
	}

	server := NewServer(&config.Config{PerformanceBrain: config.PerformanceBrainConfig{Enabled: true}}, "")
	changed, err := server.appendJiraPerformanceEvents(issue)
	if err != nil || !changed {
		t.Fatalf("seed Jira performance history: changed=%v err=%v", changed, err)
	}

	issue.Changelog.Histories[0].Author.DisplayName = "武海洋"
	changed, err = server.appendJiraPerformanceEvents(issue)
	if err != nil {
		t.Fatalf("renamed Jira history author blocked automatic synchronization: %v", err)
	}
	if changed {
		t.Fatal("renamed Jira history author was reported as new immutable evidence")
	}

	var eventCount int64
	if err := db.DB.Model(&db.PerformanceWorkItemEvent{}).
		Where("dedupe_key = ?", "jira:FZ-2257:2964008:0").Count(&eventCount).Error; err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 {
		t.Fatalf("renamed Jira history author created %d immutable rows, want 1", eventCount)
	}
}

func TestJiraAutomaticSyncSurvivesHistoricalAuthorDisplayNameChange(t *testing.T) {
	setupServerTestDB(t)

	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })

	authorDisplayName := "武海洋 [X]"
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/search":
			fmt.Fprintf(w, `{"total":1,"issues":[{
				"key":"FZ-2257",
				"fields":{
					"summary":"Historical Jira author rename",
					"created":"2026-07-01T08:00:00.000+0800",
					"issuetype":{"name":"Task"},
					"priority":{"name":"Highest"},
					"assignee":{"name":"liang.zhiyuan","displayName":"梁志远"},
					"status":{"name":"To Do"},
					"project":{"key":"FZ","name":"Fuzhou"},
					"updated":"2026-07-29T08:59:27.000+0000"
				},
				"changelog":{"histories":[{
					"id":"2964008",
					"created":"2026-07-07T09:59:14.670+0800",
					"author":{"name":"wuhaiyang","displayName":%q},
					"items":[{"field":"priority","fieldId":"","fromString":"Medium","toString":"Highest"}]
				}]}
			}]}`, authorDisplayName)
		case strings.HasSuffix(r.URL.Path, "/comment"):
			fmt.Fprint(w, `{"comments":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer jira.Close()

	server := NewServer(&config.Config{
		Jira:             config.JiraConfig{Enabled: true, BaseURL: jira.URL, CustomJQL: `key = "FZ-2257"`},
		PerformanceBrain: config.PerformanceBrainConfig{Enabled: true},
	}, "")
	server.syncJiraTasks()

	authorDisplayName = "武海洋"
	server.syncJiraTasks()

	state, err := loadJiraInboundSyncState()
	if err != nil {
		t.Fatal(err)
	}
	if state.LastError != "" || state.LastSucceededAt.IsZero() {
		t.Fatalf("automatic Jira cycle remained failed after author rename: %+v", state)
	}
	var eventCount int64
	if err := db.DB.Model(&db.PerformanceWorkItemEvent{}).
		Where("dedupe_key = ?", "jira:FZ-2257:2964008:0").Count(&eventCount).Error; err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 {
		t.Fatalf("automatic Jira replay created %d source events, want 1", eventCount)
	}
}

func TestApplyJiraPerformanceFieldsPersistsOnlyUnambiguousBugOrigin(t *testing.T) {
	var issue telemetry.JiraIssue
	if err := json.Unmarshal([]byte(`{
		"key":"WA-900",
		"fields":{
			"issuetype":{"name":"Bug"},
			"priority":{"name":"P1"},
			"parent":{"key":"WA-800","fields":{"issuetype":{"name":"Task"}}}
		}
	}`), &issue); err != nil {
		t.Fatal(err)
	}
	task := db.TaskTelemetry{TaskID: issue.Key, IssueType: "bug"}
	if !applyJiraPerformanceFields(&task, issue) || task.ParentWorkItemID != "WA-800" {
		t.Fatalf("Jira parent was not persisted as defect origin: %+v", task)
	}

	issue = telemetry.JiraIssue{}
	if err := json.Unmarshal([]byte(`{
		"key":"WA-901",
		"fields":{
			"issuetype":{"name":"Bug"},
			"issuelinks":[
				{"inwardIssue":{"key":"WA-801","fields":{"issuetype":{"name":"Task"}}}},
				{"outwardIssue":{"key":"WA-802","fields":{"issuetype":{"name":"Story"}}}}
			]
		}
	}`), &issue); err != nil {
		t.Fatal(err)
	}
	if got := jiraOriginRequirementID(issue); got != "" {
		t.Fatalf("ambiguous Jira links produced individual attribution %q", got)
	}
}

func TestParseJiraTime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string // YYYY-MM-DD HH:MM:SS format of UTC time
	}{
		{
			name:     "RFC3339 Zulu",
			input:    "2026-06-12T04:27:03Z",
			expected: "2026-06-12 04:27:03",
		},
		{
			name:     "Jira standard with offset",
			input:    "2026-06-12T12:27:03.000+0800",
			expected: "2026-06-12 04:27:03", // UTC equivalent
		},
		{
			name:     "empty string defaults to now",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parsed := parseJiraTime(tc.input)
			if tc.input == "" {
				if parsed.IsZero() {
					t.Errorf("expected now for empty time, got zero time")
				}
				return
			}
			utcStr := parsed.UTC().Format("2006-01-02 15:04:05")
			if utcStr != tc.expected {
				t.Errorf("parseJiraTime(%q) = %q (UTC); want %q (UTC)", tc.input, utcStr, tc.expected)
			}
		})
	}
}

func TestShouldPreserveLocalAssignee(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name              string
		task              db.TaskTelemetry
		incoming          string
		incomingStatus    string
		incomingUpdatedAt time.Time
		want              bool
	}{
		{
			name: "recent decision reassignment",
			task: db.TaskTelemetry{
				Assignee:     "Bob",
				LastUpdate:   now.Add(-2 * time.Hour),
				DecisionLogs: "[2026-06-23 10:00:00] PM 调停干预：将指派人从 [Alice] 转派给 [Bob]。",
			},
			incoming:       "Alice",
			incomingStatus: "progress",
			want:           true,
		},
		{
			name: "newer Jira update ends local protection",
			task: db.TaskTelemetry{
				Assignee:     "Bob",
				LastUpdate:   now.Add(-2 * time.Hour),
				DecisionLogs: "[2026-06-23 10:00:00] PM 调停干预：将指派人从 [Alice] 转派给 [Bob]。",
			},
			incoming:          "Alice",
			incomingStatus:    "progress",
			incomingUpdatedAt: now.Add(-time.Hour),
			want:              false,
		},
		{
			name: "completed Jira status ends local protection",
			task: db.TaskTelemetry{
				Assignee:     "Bob",
				LastUpdate:   now.Add(-2 * time.Hour),
				DecisionLogs: "[2026-06-23 10:00:00] PM 调停干预：将指派人从 [Alice] 转派给 [Bob]。",
			},
			incoming:       "Alice",
			incomingStatus: "done",
			want:           false,
		},
		{
			name: "stale decision can be refreshed from Jira",
			task: db.TaskTelemetry{
				Assignee:     "Bob",
				LastUpdate:   now.Add(-25 * time.Hour),
				DecisionLogs: "[2026-06-22 10:00:00] PM 调停干预：将指派人从 [Alice] 转派给 [Bob]。",
			},
			incoming:       "Alice",
			incomingStatus: "progress",
			want:           false,
		},
		{
			name: "no decision log follows Jira",
			task: db.TaskTelemetry{
				Assignee:   "Bob",
				LastUpdate: now.Add(-2 * time.Hour),
			},
			incoming:       "Alice",
			incomingStatus: "progress",
			want:           false,
		},
		{
			name: "same assignee is not protected",
			task: db.TaskTelemetry{
				Assignee:     "Bob",
				LastUpdate:   now.Add(-2 * time.Hour),
				DecisionLogs: "调整需求负责人：从 [Alice] 转派给 [Bob]。",
			},
			incoming:       "Bob",
			incomingStatus: "progress",
			want:           false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldPreserveLocalAssignee(tc.task, tc.incoming, tc.incomingStatus, tc.incomingUpdatedAt, now)
			if got != tc.want {
				t.Fatalf("shouldPreserveLocalAssignee() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestShouldIncludeInJiraKeepAlive(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		task db.TaskTelemetry
		want bool
	}{
		{
			name: "active task is always kept alive",
			task: db.TaskTelemetry{
				TaskID:     "FZ-2220",
				Status:     "progress",
				LastUpdate: now.Add(-45 * 24 * time.Hour),
			},
			want: true,
		},
		{
			name: "recent completed issue is kept alive for owner corrections",
			task: db.TaskTelemetry{
				TaskID:     "FZ-2220",
				Status:     "done",
				LastUpdate: now.Add(-6 * 24 * time.Hour),
			},
			want: true,
		},
		{
			name: "old completed issue exits keep alive",
			task: db.TaskTelemetry{
				TaskID:     "FZ-2220",
				Status:     "done",
				LastUpdate: now.Add(-30 * 24 * time.Hour),
			},
			want: false,
		},
		{
			name: "completed issue without update time exits keep alive",
			task: db.TaskTelemetry{
				TaskID: "FZ-2220",
				Status: "done",
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldIncludeInJiraKeepAlive(tc.task, now)
			if got != tc.want {
				t.Fatalf("shouldIncludeInJiraKeepAlive() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestJiraSyncBroadcastsTelemetryUpdateWhenTaskChanges(t *testing.T) {
	setupServerTestDB(t)

	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })

	const issueKey = "WA-909"
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/search":
			fmt.Fprintf(w, `{"total":1,"issues":[{"key":%q,"fields":{"summary":"Synced Jira item","created":"2026-07-01T08:00:00.000+0800","issuetype":{"name":"Bug"},"assignee":null,"status":{"name":"In Progress"},"project":{"key":"WA","name":"Well Ambient"},"fixVersions":[],"versions":[],"updated":"2026-08-02T08:00:00.000+0800"}}]}`, issueKey)
		case strings.HasSuffix(r.URL.Path, "/comment"):
			fmt.Fprint(w, `{"comments":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer jira.Close()

	updates := make(chan string, 4)
	clientsMu.Lock()
	telemetryClients[updates] = true
	clientsMu.Unlock()
	t.Cleanup(func() {
		clientsMu.Lock()
		delete(telemetryClients, updates)
		clientsMu.Unlock()
	})

	server := NewServer(&config.Config{Jira: config.JiraConfig{
		Enabled:   true,
		BaseURL:   jira.URL,
		CustomJQL: `project = "WA"`,
	}}, "")
	server.syncJiraTasks()

	var synced db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", issueKey).First(&synced).Error; err != nil {
		t.Fatalf("query synced task: %v", err)
	}
	if synced.Status != "progress" || synced.Title != "Synced Jira item" {
		t.Fatalf("unexpected synced task: %+v", synced)
	}

	select {
	case got := <-updates:
		if got != issueKey {
			t.Fatalf("telemetry update = %q, want %q", got, issueKey)
		}
	default:
		t.Fatal("Jira sync changed local Daily Jira facts without broadcasting a telemetry update")
	}

	server.syncJiraTasks()
	select {
	case got := <-updates:
		t.Fatalf("unchanged Jira sync broadcast duplicate update for %q", got)
	default:
	}
}

func TestJiraSyncBroadcastsWhenOnlyJiraCommentChanges(t *testing.T) {
	setupServerTestDB(t)

	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })

	const issueKey = "DL-4309"
	issueUpdatedAt := parseJiraTime("2026-08-15T22:41:25.000+0800")
	resolvedAt := parseJiraTime("2026-08-15T22:41:08.000+0800")
	createdAt := parseJiraTime("2026-08-03T11:27:00.000+0800")
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID: issueKey, ProjectKey: "DL", Source: "jira", ExternalKey: issueKey,
		PlanningState: deliveryplanning.PlanningReady, Title: "移动锁站FMS相关适配",
		Repo: "-", Assignee: "梁志远", Branch: "-", LastCommit: "-",
		Status: "done", IssueType: "requirement", TaskCreatedAt: createdAt,
		LastUpdate: time.Now().Add(-time.Hour), SourceUpdatedAt: issueUpdatedAt,
		CompletedAt: &resolvedAt,
	}).Error; err != nil {
		t.Fatalf("seed synced Jira task: %v", err)
	}

	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/search":
			fmt.Fprintf(w, `{"total":1,"issues":[{"key":%q,"fields":{"summary":"移动锁站FMS相关适配","created":"2026-08-03T11:27:00.000+0800","resolutiondate":"2026-08-15T22:41:08.000+0800","issuetype":{"name":"Task"},"assignee":{"name":"zhiyuan_liang","displayName":"梁志远"},"status":{"name":"Done"},"project":{"key":"DL","name":"Dalian"},"fixVersions":[],"versions":[],"updated":"2026-08-15T22:41:25.000+0800"}}]}`, issueKey)
		case strings.HasSuffix(r.URL.Path, "/comment"):
			fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":1,"comments":[{"id":"533697","author":{"displayName":"梁志远"},"body":"仿真环境已部署","created":"2026-08-15T22:41:25.000+0800","updated":"2026-08-15T22:41:25.000+0800"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer jira.Close()

	updates := make(chan string, 4)
	clientsMu.Lock()
	telemetryClients[updates] = true
	clientsMu.Unlock()
	t.Cleanup(func() {
		clientsMu.Lock()
		delete(telemetryClients, updates)
		clientsMu.Unlock()
	})

	server := NewServer(&config.Config{Jira: config.JiraConfig{
		Enabled: true, BaseURL: jira.URL, CustomJQL: `project = "DL"`,
	}}, "")
	server.syncJiraTasks()

	var comment db.JiraCommentLog
	if err := db.DB.Where("comment_id = ?", "533697").First(&comment).Error; err != nil {
		t.Fatalf("Jira comment was not persisted: %v", err)
	}
	if comment.TaskID != issueKey || comment.Body != "仿真环境已部署" {
		t.Fatalf("unexpected Jira comment projection: %+v", comment)
	}
	select {
	case got := <-updates:
		if got != issueKey {
			t.Fatalf("telemetry update = %q, want %q", got, issueKey)
		}
	default:
		t.Fatal("Jira comment changed local activity without broadcasting a telemetry update")
	}
}

func TestJiraSyncDoesNotRefetchUnchangedCommentsAcrossCyclesOrDuplicatePaths(t *testing.T) {
	setupServerTestDB(t)

	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })

	const issueKey = "WA-912"
	issueUpdatedAt := parseJiraTime("2026-08-15T20:00:00.000+0800")
	createdAt := parseJiraTime("2026-08-01T08:00:00.000+0800")
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID: issueKey, ProjectKey: "WA", Source: "jira", ExternalKey: issueKey,
		PlanningState: deliveryplanning.PlanningReady, Title: "Stable Jira item",
		Repo: "-", Assignee: "未指派", Branch: "-", LastCommit: "-",
		Status: "progress", IssueType: "requirement", TaskCreatedAt: createdAt,
		LastUpdate: time.Now().Add(-time.Hour), SourceUpdatedAt: issueUpdatedAt,
	}).Error; err != nil {
		t.Fatalf("seed stable Jira task: %v", err)
	}

	commentRequests := 0
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/search":
			fmt.Fprintf(w, `{"total":1,"issues":[{"key":%q,"fields":{"summary":"Stable Jira item","created":"2026-08-01T08:00:00.000+0800","issuetype":{"name":"Task"},"assignee":null,"status":{"name":"In Progress"},"project":{"key":"WA","name":"Well Ambient"},"fixVersions":[],"versions":[],"updated":"2026-08-15T20:00:00.000+0800"}}]}`, issueKey)
		case strings.HasSuffix(r.URL.Path, "/comment"):
			commentRequests++
			fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":0,"comments":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer jira.Close()

	server := NewServer(&config.Config{Jira: config.JiraConfig{
		Enabled: true, BaseURL: jira.URL, CustomJQL: `project = "WA"`,
	}}, "")
	server.syncJiraTasks()
	server.syncJiraTasks()

	if commentRequests != 1 {
		t.Fatalf("unchanged Jira item fetched comments %d times across two cycles, want one initial reconcile", commentRequests)
	}
}

func TestJiraReconciliationJQLUsesOverlapScopeDeduplicationAndBoundedBatches(t *testing.T) {
	checkpoint := time.Date(2026, 8, 15, 15, 0, 0, 0, time.UTC)
	now := checkpoint.Add(2 * time.Minute)
	localTasks := make([]db.TaskTelemetry, 0, 53)
	for index := 1; index <= 52; index++ {
		localTasks = append(localTasks, db.TaskTelemetry{TaskID: fmt.Sprintf("DL-%04d", index), Source: "jira"})
	}
	localTasks = append(localTasks, db.TaskTelemetry{TaskID: "WA-9999", Source: "jira"})

	queries := buildJiraReconciliationJQLs(
		&config.JiraConfig{SyncProjects: []string{"DL"}},
		localTasks,
		map[string]struct{}{"DL-0001": {}},
		checkpoint,
		now,
	)
	if len(queries) != 1 {
		t.Fatalf("reconciliation queries = %d, want one project-window query", len(queries))
	}
	joined := strings.Join(queries, "\n")
	if !strings.Contains(joined, `project in ("DL")`) {
		t.Fatalf("reconciliation query lost configured project boundary: %s", joined)
	}
	if strings.Contains(joined, `"DL-0001"`) || strings.Contains(joined, `"WA-9999"`) {
		t.Fatalf("reconciliation query still enumerates local issue keys: %s", joined)
	}
	wantSince := checkpoint.Add(-jiraInboundSyncOverlap).In(time.Local).Format("2006-01-02 15:04")
	for _, query := range queries {
		if !strings.Contains(query, `updated >= "`+wantSince+`"`) {
			t.Fatalf("reconciliation JQL %q missing inclusive overlap boundary %q", query, wantSince)
		}
	}

	futureCheckpoint := now.Add(time.Hour)
	futureQueries := buildJiraReconciliationJQLs(
		&config.JiraConfig{SyncProjects: []string{"DL"}},
		[]db.TaskTelemetry{{TaskID: "DL-4309", Source: "jira"}},
		map[string]struct{}{},
		futureCheckpoint,
		now,
	)
	wantClampedSince := now.Add(-jiraInboundSyncOverlap).In(time.Local).Format("2006-01-02 15:04")
	if len(futureQueries) != 1 || !strings.Contains(futureQueries[0], `updated >= "`+wantClampedSince+`"`) {
		t.Fatalf("future checkpoint was not clamped to now-overlap: %v", futureQueries)
	}
}

func TestJiraReconciliationDoesNotCreateHundredsOfKeyBatchesForHistoricalDoneRows(t *testing.T) {
	now := time.Date(2026, 8, 19, 20, 0, 0, 0, time.Local)
	localTasks := make([]db.TaskTelemetry, 0, 35500)
	for index := 0; index < 35500; index++ {
		localTasks = append(localTasks, db.TaskTelemetry{
			TaskID: fmt.Sprintf("WA-%05d", index+1), Source: "jira", Status: "done",
			LastUpdate: now.Add(-30 * 24 * time.Hour),
		})
	}

	queries := buildJiraReconciliationJQLs(
		&config.JiraConfig{SyncProjects: []string{"WA"}},
		localTasks,
		map[string]struct{}{},
		now.Add(-30*time.Second),
		now,
	)
	if len(queries) != 1 {
		t.Fatalf("large historical reconciliation produced %d queries, want 1", len(queries))
	}
	if strings.Contains(queries[0], "key in") || !strings.Contains(queries[0], `project in ("WA")`) {
		t.Fatalf("large reconciliation was not reduced to a project update window: %q", queries[0])
	}
}

func TestJiraSyncEmptyScopeRecordsVisibleFailure(t *testing.T) {
	setupServerTestDB(t)

	server := NewServer(&config.Config{Jira: config.JiraConfig{Enabled: true}}, "")
	server.syncJiraTasks()

	var checkpoint db.JiraInboundSyncState
	if err := db.DB.Where("scope = ?", jiraInboundSyncScope).First(&checkpoint).Error; err != nil {
		t.Fatalf("load Jira inbound checkpoint: %v", err)
	}
	if strings.TrimSpace(checkpoint.LastError) == "" {
		t.Fatal("enabled Jira sync with an empty scope left no visible failure")
	}
	if !checkpoint.LastSucceededAt.IsZero() || !checkpoint.SuccessfulThrough.IsZero() {
		t.Fatalf("empty Jira scope advanced success watermark: %+v", checkpoint)
	}
}

func TestJiraSyncRefreshesOldCompletedIssueAfterPrimaryJQLExcludesIt(t *testing.T) {
	setupServerTestDB(t)

	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })

	const issueKey = "DL-4200"
	completedAt := time.Now().Add(-30 * 24 * time.Hour)
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID: issueKey, ProjectKey: "DL", Source: "jira", ExternalKey: issueKey,
		PlanningState: deliveryplanning.PlanningReady, Title: "Old completed Jira item",
		Repo: "-", Assignee: "梁志远", Branch: "-", LastCommit: "-",
		Status: "done", IssueType: "requirement", TaskCreatedAt: completedAt.Add(-24 * time.Hour),
		LastUpdate: completedAt, SourceUpdatedAt: completedAt, CompletedAt: &completedAt,
	}).Error; err != nil {
		t.Fatalf("seed old completed Jira task: %v", err)
	}

	queriedKnownIssue := false
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/search":
			jql := r.URL.Query().Get("jql")
			if strings.Contains(jql, `project in ("DL")`) && strings.Contains(jql, "updated >=") {
				queriedKnownIssue = true
				fmt.Fprint(w, `{"total":2,"issues":[{"key":"DL-4200","fields":{"summary":"Old completed Jira item","created":"2026-06-01T08:00:00.000+0800","issuetype":{"name":"Task"},"assignee":{"name":"zhiyuan_liang","displayName":"梁志远"},"status":{"name":"In Progress"},"project":{"key":"DL","name":"Dalian"},"fixVersions":[],"versions":[],"updated":"2026-08-15T22:50:00.000+0800"}},{"key":"DL-9999","fields":{"summary":"Not in local reconciliation set","created":"2026-08-15T08:00:00.000+0800","issuetype":{"name":"Epic"},"assignee":null,"status":{"name":"Done"},"project":{"key":"DL","name":"Dalian"},"fixVersions":[],"versions":[],"updated":"2026-08-15T22:50:00.000+0800"}}]}`)
				return
			}
			fmt.Fprint(w, `{"total":0,"issues":[]}`)
		case strings.HasSuffix(r.URL.Path, "/comment"):
			fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":1,"comments":[{"id":"old-done-new-comment","author":{"displayName":"梁志远"},"body":"完成后补充证据","created":"2026-08-15T22:50:00.000+0800","updated":"2026-08-15T22:50:00.000+0800"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer jira.Close()

	server := NewServer(&config.Config{Jira: config.JiraConfig{
		Enabled: true, BaseURL: jira.URL, SyncProjects: []string{"DL"},
		CustomJQL: `project = "DL" AND status != Done`,
	}}, "")
	server.syncJiraTasks()

	if !queriedKnownIssue {
		t.Fatal("old completed Jira issue was excluded from both primary and reconciliation queries")
	}
	var refreshed db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", issueKey).First(&refreshed).Error; err != nil {
		t.Fatalf("query refreshed old Jira task: %v", err)
	}
	if refreshed.Status != "progress" {
		t.Fatalf("old completed Jira issue status = %q, want progress after reopen", refreshed.Status)
	}
	var unexpected db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", "DL-9999").First(&unexpected).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("project-window reconciliation imported an unknown local key: %+v, err=%v", unexpected, err)
	}
	var comment db.JiraCommentLog
	if err := db.DB.Where("comment_id = ?", "old-done-new-comment").First(&comment).Error; err != nil {
		t.Fatalf("post-completion Jira comment was not synchronized: %v", err)
	}
}

func TestJiraSyncCompletionOverridesRecentLocalProjectionAndRemovesDailyJiraCandidate(t *testing.T) {
	setupServerTestDB(t)

	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })

	const issueKey = "DL-4309"
	now := time.Now()
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID: issueKey, ProjectKey: "DL", Source: "jira", ExternalKey: issueKey,
		PlanningState: deliveryplanning.PlanningReady, Title: "Recently touched local projection",
		Repo: "Dalian (DL)", Assignee: "梁志远", Branch: "-", LastCommit: "-",
		Status: "progress", IssueType: "requirement", TaskCreatedAt: now.Add(-8 * 24 * time.Hour),
		LastUpdate: now, SourceUpdatedAt: now.Add(-time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed recently touched Jira task: %v", err)
	}

	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/search":
			fmt.Fprint(w, `{"total":1,"issues":[{"key":"DL-4309","fields":{"summary":"Recently touched local projection","created":"2026-08-08T08:00:00.000+0800","issuetype":{"name":"Task"},"assignee":{"name":"zhiyuan_liang","displayName":"梁志远"},"status":{"name":"Done"},"project":{"key":"DL","name":"Dalian"},"fixVersions":[],"versions":[],"updated":"2026-08-16T21:50:00.000+0800"}}]}`)
		case strings.HasSuffix(r.URL.Path, "/comment"):
			fmt.Fprint(w, `{"comments":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer jira.Close()

	server := NewServer(&config.Config{Jira: config.JiraConfig{
		Enabled: true, BaseURL: jira.URL, CustomJQL: `project = "DL"`,
	}}, "")
	server.syncJiraTasks()

	var refreshed db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", issueKey).First(&refreshed).Error; err != nil {
		t.Fatalf("query completed Jira task: %v", err)
	}
	if refreshed.Status != "done" {
		t.Fatalf("Jira completion was blocked by unrelated recent local activity: status = %q, want done", refreshed.Status)
	}
	audit := buildDailyJiraAuditResponse([]db.TaskTelemetry{refreshed}, nil, nil, []string{"梁志远"}, now)
	if audit.Summary.Total != 0 {
		t.Fatalf("completed Jira item remained in Daily Jira: %+v", audit.Summary)
	}
}

func TestJiraCommentSyncInitialWatermarkDoesNotLogRecordNotFound(t *testing.T) {
	setupServerTestDB(t)

	var databaseLog bytes.Buffer
	previousLogger := db.DB.Logger
	db.DB.Logger = gormlogger.New(log.New(&databaseLog, "", 0), gormlogger.Config{LogLevel: gormlogger.Warn})
	t.Cleanup(func() { db.DB.Logger = previousLogger })

	const issueKey = "DL-4310"
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/comment") {
			fmt.Fprint(w, `{"comments":[]}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer jira.Close()

	server := NewServer(&config.Config{}, "")
	issueUpdatedAt := time.Now().Add(-time.Minute).UTC().Truncate(time.Second)
	changed, err := server.syncJiraCommentsForIssue(
		telemetry.NewJiraClient(&config.JiraConfig{BaseURL: jira.URL}),
		issueKey,
		issueUpdatedAt,
	)
	if err != nil {
		t.Fatalf("initial Jira comment sync: %v", err)
	}
	if changed {
		t.Fatal("empty initial Jira comment projection reported a content change")
	}
	if strings.Contains(strings.ToLower(databaseLog.String()), "record not found") {
		t.Fatalf("optional Jira issue watermark lookup emitted an error log: %s", databaseLog.String())
	}
	var state db.JiraIssueSyncState
	if err := db.DB.Where("task_id = ?", issueKey).First(&state).Error; err != nil {
		t.Fatalf("initial Jira issue watermark was not persisted: %v", err)
	}
	if !state.SourceUpdatedAt.Equal(issueUpdatedAt) || state.LastSucceededAt.IsZero() {
		t.Fatalf("unexpected initial Jira issue watermark: %+v", state)
	}
}

func TestJiraSyncCommentFailureDoesNotAdvanceWatermarksAndRetries(t *testing.T) {
	setupServerTestDB(t)

	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })

	const issueKey = "DL-4308"
	createdAt := parseJiraTime("2026-08-03T11:00:00.000+0800")
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID: issueKey, ProjectKey: "DL", Source: "jira", ExternalKey: issueKey,
		PlanningState: deliveryplanning.PlanningReady, Title: "Retry Jira comments",
		Repo: "-", Assignee: "梁志远", Branch: "-", LastCommit: "-",
		Status: "progress", IssueType: "requirement", TaskCreatedAt: createdAt,
		LastUpdate: time.Now().Add(-time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed Jira task: %v", err)
	}

	commentRequests := 0
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/search":
			fmt.Fprintf(w, `{"total":1,"issues":[{"key":%q,"fields":{"summary":"Retry Jira comments","created":"2026-08-03T11:00:00.000+0800","issuetype":{"name":"Task"},"assignee":{"name":"zhiyuan_liang","displayName":"梁志远"},"status":{"name":"In Progress"},"project":{"key":"DL","name":"Dalian"},"fixVersions":[],"versions":[],"updated":"2026-08-15T22:55:00.000+0800"}}]}`, issueKey)
		case strings.HasSuffix(r.URL.Path, "/comment"):
			commentRequests++
			if commentRequests == 1 {
				http.Error(w, "temporary Jira failure", http.StatusServiceUnavailable)
				return
			}
			fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":1,"comments":[{"id":"retry-comment","author":{"displayName":"梁志远"},"body":"重试后到达","created":"2026-08-15T22:55:00.000+0800","updated":"2026-08-15T22:55:00.000+0800"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer jira.Close()

	server := NewServer(&config.Config{Jira: config.JiraConfig{
		Enabled: true, BaseURL: jira.URL, CustomJQL: `project = "DL"`,
	}}, "")
	server.syncJiraTasks()

	var failedIssueState db.JiraIssueSyncState
	if err := db.DB.Where("task_id = ?", issueKey).First(&failedIssueState).Error; err != nil {
		t.Fatalf("load failed issue sync state: %v", err)
	}
	if failedIssueState.LastError == "" || !failedIssueState.SourceUpdatedAt.IsZero() {
		t.Fatalf("failed comment sync advanced issue watermark: %+v", failedIssueState)
	}
	var failedCycle db.JiraInboundSyncState
	if err := db.DB.Where("scope = ?", jiraInboundSyncScope).First(&failedCycle).Error; err != nil {
		t.Fatalf("load failed cycle state: %v", err)
	}
	if failedCycle.LastError == "" || !failedCycle.SuccessfulThrough.IsZero() {
		t.Fatalf("failed comment sync advanced cycle checkpoint: %+v", failedCycle)
	}

	server.syncJiraTasks()
	if commentRequests != 2 {
		t.Fatalf("comment requests = %d, want one failure and one retry", commentRequests)
	}
	var comment db.JiraCommentLog
	if err := db.DB.Where("comment_id = ? AND current = ?", "retry-comment", true).First(&comment).Error; err != nil {
		t.Fatalf("retried Jira comment was not persisted: %v", err)
	}
	var recoveredIssueState db.JiraIssueSyncState
	if err := db.DB.Where("task_id = ?", issueKey).First(&recoveredIssueState).Error; err != nil {
		t.Fatalf("load recovered issue sync state: %v", err)
	}
	if recoveredIssueState.LastError != "" || recoveredIssueState.SourceUpdatedAt.IsZero() {
		t.Fatalf("successful retry did not advance issue watermark: %+v", recoveredIssueState)
	}
	var recoveredCycle db.JiraInboundSyncState
	if err := db.DB.Where("scope = ?", jiraInboundSyncScope).First(&recoveredCycle).Error; err != nil {
		t.Fatalf("load recovered cycle state: %v", err)
	}
	if recoveredCycle.LastError != "" || recoveredCycle.SuccessfulThrough.IsZero() {
		t.Fatalf("successful retry did not advance cycle checkpoint: %+v", recoveredCycle)
	}
}

func TestJiraSyncRetiresDeletedCommentFromCurrentProjection(t *testing.T) {
	setupServerTestDB(t)

	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })

	const issueKey = "DL-4307"
	issueUpdatedAt := "2026-08-15T22:56:00.000+0800"
	commentExists := true
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/search":
			fmt.Fprintf(w, `{"total":1,"issues":[{"key":%q,"fields":{"summary":"Deleted Jira comment","created":"2026-08-03T11:00:00.000+0800","issuetype":{"name":"Task"},"assignee":null,"status":{"name":"In Progress"},"project":{"key":"DL","name":"Dalian"},"fixVersions":[],"versions":[],"updated":%q}}]}`, issueKey, issueUpdatedAt)
		case strings.HasSuffix(r.URL.Path, "/comment"):
			if commentExists {
				fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":1,"comments":[{"id":"deleted-comment","author":{"displayName":"梁志远"},"body":"将被删除","created":"2026-08-15T22:56:00.000+0800","updated":"2026-08-15T22:56:00.000+0800"}]}`)
				return
			}
			fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":0,"comments":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer jira.Close()

	server := NewServer(&config.Config{Jira: config.JiraConfig{
		Enabled: true, BaseURL: jira.URL, CustomJQL: `project = "DL"`,
	}}, "")
	server.syncJiraTasks()
	commentExists = false
	issueUpdatedAt = "2026-08-15T22:57:00.000+0800"
	server.syncJiraTasks()

	var comment db.JiraCommentLog
	if err := db.DB.Where("comment_id = ?", "deleted-comment").First(&comment).Error; err != nil {
		t.Fatalf("load retired Jira comment: %v", err)
	}
	if comment.Current {
		t.Fatalf("deleted Jira comment remained in current activity projection: %+v", comment)
	}
}

func TestJiraSyncBroadcastsFreshIssueBeforeUnrelatedSlowIssueFinishes(t *testing.T) {
	setupServerTestDB(t)

	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })

	seeded := []db.TaskTelemetry{
		{TaskID: "DL-4309", ProjectKey: "DL", Source: "jira", ExternalKey: "DL-4309", PlanningState: deliveryplanning.PlanningReady, Title: "Fresh issue", Repo: "-", Assignee: "未指派", Branch: "-", LastCommit: "-", Status: "progress", IssueType: "requirement", TaskCreatedAt: parseJiraTime("2026-08-03T11:27:00.000+0800"), LastUpdate: time.Now().Add(-time.Hour)},
		{TaskID: "WA-100", ProjectKey: "WA", Source: "jira", ExternalKey: "WA-100", PlanningState: deliveryplanning.PlanningReady, Title: "Slow unrelated issue", Repo: "-", Assignee: "未指派", Branch: "-", LastCommit: "-", Status: "progress", IssueType: "requirement", TaskCreatedAt: parseJiraTime("2026-08-01T08:00:00.000+0800"), LastUpdate: time.Now().Add(-time.Hour)},
	}
	if err := db.DB.Create(&seeded).Error; err != nil {
		t.Fatalf("seed Jira tasks: %v", err)
	}

	slowStarted := make(chan struct{})
	releaseSlow := make(chan struct{})
	defer func() {
		select {
		case <-releaseSlow:
		default:
			close(releaseSlow)
		}
	}()
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/search":
			fmt.Fprint(w, `{"total":2,"issues":[{"key":"WA-100","fields":{"summary":"Slow unrelated issue","created":"2026-08-01T08:00:00.000+0800","issuetype":{"name":"Task"},"assignee":null,"status":{"name":"In Progress"},"project":{"key":"WA","name":"Well Ambient"},"fixVersions":[],"versions":[],"updated":"2026-08-15T22:00:00.000+0800"}},{"key":"DL-4309","fields":{"summary":"Fresh issue","created":"2026-08-03T11:27:00.000+0800","issuetype":{"name":"Task"},"assignee":null,"status":{"name":"Done"},"project":{"key":"DL","name":"Dalian"},"fixVersions":[],"versions":[],"updated":"2026-08-15T22:58:00.000+0800"}}]}`)
		case strings.Contains(r.URL.Path, "/DL-4309/comment"):
			fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":0,"comments":[]}`)
		case strings.Contains(r.URL.Path, "/WA-100/comment"):
			close(slowStarted)
			<-releaseSlow
			fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":0,"comments":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer jira.Close()

	updates := make(chan string, 4)
	clientsMu.Lock()
	telemetryClients[updates] = true
	clientsMu.Unlock()
	t.Cleanup(func() {
		clientsMu.Lock()
		delete(telemetryClients, updates)
		clientsMu.Unlock()
	})

	server := NewServer(&config.Config{Jira: config.JiraConfig{
		Enabled: true, BaseURL: jira.URL, CustomJQL: `project in (DL, WA)`,
	}}, "")
	done := make(chan struct{})
	go func() {
		server.syncJiraTasks()
		close(done)
	}()

	select {
	case <-slowStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("slow Jira issue was not reached")
	}
	select {
	case got := <-updates:
		if got != "DL-4309" {
			t.Fatalf("first telemetry update = %q, want fresh issue DL-4309", got)
		}
	default:
		t.Fatal("fresh Jira issue notification waited for an unrelated slow issue")
	}
	close(releaseSlow)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Jira sync did not finish after slow issue was released")
	}
}

func TestJiraSyncRefreshesCustomJQLProjectAfterAssigneeLeavesCoreScope(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "task-reader@westwell-lab.com", "Task Reader", []string{"dashboard:read"})

	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })

	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID:        "YBET-138",
		Title:         "Stored Jira item",
		Assignee:      "梁志远",
		Status:        "backlog",
		IssueType:     "bug",
		LastUpdate:    time.Now().Add(-48 * time.Hour),
		TaskCreatedAt: time.Date(2026, 7, 29, 8, 0, 0, 0, time.Local),
	}).Error; err != nil {
		t.Fatalf("seed stale Jira task: %v", err)
	}

	var searchJQLs []string
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/search":
			jql := r.URL.Query().Get("jql")
			searchJQLs = append(searchJQLs, jql)
			if strings.Contains(jql, "updated >=") && strings.Contains(jql, `"YBET"`) {
				fmt.Fprint(w, `{"total":1,"issues":[{"key":"YBET-138","fields":{"summary":"Stored Jira item","created":"2026-07-29T08:00:00.000+0800","issuetype":{"name":"Bug"},"assignee":{"name":"vendor.user","displayName":"外部协作方","emailAddress":"vendor@example.com"},"status":{"name":"Open"},"project":{"key":"YBET","name":"YBET"},"fixVersions":[],"versions":[],"updated":"2026-08-13T08:00:00.000+0800"}}]}`)
				return
			}
			fmt.Fprint(w, `{"total":1,"issues":[{"key":"WA-1","fields":{"summary":"Current core Jira item","created":"2026-08-01T08:00:00.000+0800","issuetype":{"name":"Task"},"assignee":{"name":"liang.zhiyuan","displayName":"梁志远","emailAddress":"liang@example.com"},"status":{"name":"Open"},"project":{"key":"WA","name":"Well Ambient"},"fixVersions":[],"versions":[],"updated":"2026-08-13T08:00:00.000+0800"}}]}`)
		case strings.HasSuffix(r.URL.Path, "/comment"):
			fmt.Fprint(w, `{"comments":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer jira.Close()

	server := NewServer(&config.Config{
		Server: config.ServerConfig{Port: 9090, Host: "127.0.0.1"},
		Jira: config.JiraConfig{
			Enabled:      true,
			BaseURL:      jira.URL,
			SyncProjects: []string{"WA"},
			CustomJQL:    `project in (WA, YBET) AND assignee in ("梁志远")`,
		},
	}, "")
	server.syncJiraTasks()

	var refreshed db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", "YBET-138").First(&refreshed).Error; err != nil {
		t.Fatalf("query refreshed Jira task: %v", err)
	}
	if refreshed.Assignee != "外部协作方" {
		t.Fatalf("YBET-138 assignee = %q, want Jira assignee 外部协作方; search JQLs: %v", refreshed.Assignee, searchJQLs)
	}
	if refreshed.ProjectKey != "YBET" || refreshed.Source != "jira" {
		t.Fatalf("YBET-138 Jira identity was not repaired: %+v", refreshed)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	server.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/tasks status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var visible []db.TaskTelemetry
	if err := json.NewDecoder(rr.Body).Decode(&visible); err != nil {
		t.Fatalf("decode tasks response: %v", err)
	}
	for _, task := range visible {
		if task.TaskID == "YBET-138" {
			t.Fatalf("YBET-138 remained visible after Jira moved it to an external assignee: %+v", task)
		}
	}
}

func TestJiraSyncRunsKeepAliveWhenPrimarySearchIsEmpty(t *testing.T) {
	setupServerTestDB(t)

	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })

	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID:     "WA-138",
		Title:      "Stored Jira item",
		Assignee:   "梁志远",
		Status:     "backlog",
		IssueType:  "bug",
		LastUpdate: time.Now().Add(-48 * time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed stale Jira task: %v", err)
	}

	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/search":
			jql := r.URL.Query().Get("jql")
			if strings.Contains(jql, `project in ("WA")`) && strings.Contains(jql, "updated >=") {
				fmt.Fprint(w, `{"total":1,"issues":[{"key":"WA-138","fields":{"summary":"Stored Jira item","created":"2026-07-29T08:00:00.000+0800","issuetype":{"name":"Bug"},"assignee":{"name":"vendor.user","displayName":"外部协作方","emailAddress":"vendor@example.com"},"status":{"name":"Open"},"project":{"key":"WA","name":"Well Ambient"},"fixVersions":[],"versions":[],"updated":"2026-08-13T08:00:00.000+0800"}}]}`)
				return
			}
			fmt.Fprint(w, `{"total":0,"issues":[]}`)
		case strings.HasSuffix(r.URL.Path, "/comment"):
			fmt.Fprint(w, `{"comments":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer jira.Close()

	server := NewServer(&config.Config{Jira: config.JiraConfig{
		Enabled:      true,
		BaseURL:      jira.URL,
		SyncProjects: []string{"WA"},
		SyncUsers:    []string{"梁志远"},
	}}, "")
	server.syncJiraTasks()

	var refreshed db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", "WA-138").First(&refreshed).Error; err != nil {
		t.Fatalf("query refreshed Jira task: %v", err)
	}
	if refreshed.Assignee != "外部协作方" {
		t.Fatalf("WA-138 assignee = %q, want keep-alive Jira assignee 外部协作方", refreshed.Assignee)
	}
}

func TestJiraSyncKeepAliveExcludesGitLabTelemetry(t *testing.T) {
	setupServerTestDB(t)

	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })

	seeded := []db.TaskTelemetry{
		{
			TaskID:     "FZ-2247",
			Source:     "git",
			Title:      "feat: FZ-2247 添加吊具检测驶离保护",
			Repo:       "task_executor",
			Branch:     "dev_fuzhou",
			LastCommit: "See merge request fms/task_executor!23",
			Status:     "progress",
			LastUpdate: time.Now().Add(-time.Hour),
		},
		{
			TaskID:      "FZ-2299",
			Source:      "jira",
			ExternalKey: "FZ-2299",
			Title:       "Jira issue",
			Status:      "progress",
			LastUpdate:  time.Now().Add(-time.Hour),
		},
	}
	for _, task := range seeded {
		if err := db.DB.Create(&task).Error; err != nil {
			t.Fatalf("seed %s: %v", task.TaskID, err)
		}
	}

	var keepAliveJQL string
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/rest/api/2/search" {
			http.NotFound(w, r)
			return
		}
		jql := r.URL.Query().Get("jql")
		if strings.HasPrefix(jql, "key in") {
			keepAliveJQL = jql
		}
		fmt.Fprint(w, `{"total":0,"issues":[]}`)
	}))
	defer jira.Close()

	server := NewServer(&config.Config{Jira: config.JiraConfig{
		Enabled:   true,
		BaseURL:   jira.URL,
		SyncUsers: []string{"梁志远"},
	}}, "")
	server.syncJiraTasks()

	if !strings.Contains(keepAliveJQL, `"FZ-2299"`) {
		t.Fatalf("keep-alive skipped Jira source row: %q", keepAliveJQL)
	}
	if strings.Contains(keepAliveJQL, `"FZ-2247"`) {
		t.Fatalf("keep-alive mixed GitLab MR telemetry into Jira search: %q", keepAliveJQL)
	}
}

func TestJiraSyncReclassifiesDedicatedSolutionAuthorAndQueuesPolish(t *testing.T) {
	setupServerTestDB(t)

	const issueKey = "DG-394"
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID: issueKey, ProjectKey: "DG", Title: "充电状态同步", Description: "需求初始内容",
	}).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}

	author := "普通账号"
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"startAt":0,"maxResults":100,"total":1,"comments":[{"id":"457134","author":{"displayName":%q},"body":"FMS持续更新车辆充电状态，GUI同步显示。","created":"2026-08-12T10:00:00.000+0800","updated":"2026-08-12T10:00:00.000+0800"}]}`, author)
	}))
	defer jira.Close()

	cfg := &config.Config{Jira: config.JiraConfig{Enabled: true, BaseURL: jira.URL}}
	server := NewServer(cfg, "")
	client := telemetry.NewJiraClient(&cfg.Jira)

	server.syncJiraComments(client, issueKey)
	var initial db.SolutionSourceRef
	if err := db.DB.Where("external_id = ?", "457134").First(&initial).Error; err != nil {
		t.Fatalf("query initial source: %v", err)
	}
	if initial.Eligible {
		t.Fatalf("ordinary author source unexpectedly eligible: %+v", initial)
	}

	author = "jira公用-解决方案"
	server.syncJiraComments(client, issueKey)

	var sources []db.SolutionSourceRef
	if err := db.DB.Where("external_id = ?", "457134").Find(&sources).Error; err != nil {
		t.Fatalf("query reclassified source: %v", err)
	}
	if len(sources) != 1 || !sources[0].Current || !sources[0].Eligible || sources[0].Marker != "author:jira公用-解决方案" {
		t.Fatalf("same immutable source was not reclassified: %+v", sources)
	}

	var jobs []db.SolutionPolishJob
	if err := db.DB.Find(&jobs).Error; err != nil {
		t.Fatalf("query polish jobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Status != "queued" {
		t.Fatalf("dedicated solution comment did not queue exactly one polish job: %+v", jobs)
	}
	workspace, err := server.solutions.GetWorkspace(context.Background(), issueKey)
	if err != nil || workspace.Working != nil || len(workspace.History) != 0 || len(workspace.Candidates) != 0 {
		t.Fatalf("Jira sync exposed an empty draft before Agent completion: workspace=%+v err=%v", workspace, err)
	}
	var seed db.SolutionRevision
	if err := db.DB.First(&seed, jobs[0].InputRevisionID).Error; err != nil {
		t.Fatalf("load hidden generation seed: %v", err)
	}
	if seed.Kind != solutions.KindSystemSeed || seed.Version != 0 {
		t.Fatalf("Jira initial generation seed = %+v, want hidden v0 system seed", seed)
	}

	server.syncJiraComments(client, issueKey)
	var jobCount int64
	if err := db.DB.Model(&db.SolutionPolishJob{}).Count(&jobCount).Error; err != nil || jobCount != 1 {
		t.Fatalf("replayed eligible comment duplicated polish job: count=%d err=%v", jobCount, err)
	}
}

func TestJiraSyncDoesNotQueueAgentDraftWhenHumanDraftExists(t *testing.T) {
	setupServerTestDB(t)

	const issueKey = "DG-396"
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID: issueKey, ProjectKey: "DG", Title: "人工方案优先", Description: "需求初始内容",
	}).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}
	server := NewServer(&config.Config{}, "")
	if _, err := server.solutions.EnsureDraft(context.Background(), solutions.EnsureDraftCommand{
		DemandID: issueKey, Title: "人工方案优先", Markdown: "# 已确认人工方案\n\n保持人工内容", Actor: "Alice",
	}); err != nil {
		t.Fatalf("seed human draft: %v", err)
	}

	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":1,"comments":[{"id":"solution-human-boundary","author":{"displayName":"jira公用-解决方案"},"body":"这是后续同步的方案评论。","created":"2026-08-12T10:00:00.000+0800","updated":"2026-08-12T10:00:00.000+0800"}]}`)
	}))
	defer jira.Close()
	server.config.Jira = config.JiraConfig{Enabled: true, BaseURL: jira.URL}
	server.syncJiraComments(telemetry.NewJiraClient(&server.config.Jira), issueKey)

	var jobCount int64
	if err := db.DB.Model(&db.SolutionPolishJob{}).Count(&jobCount).Error; err != nil || jobCount != 0 {
		t.Fatalf("existing human draft queued background Agent candidate: count=%d err=%v", jobCount, err)
	}
	workspace, err := server.solutions.GetWorkspace(context.Background(), issueKey)
	if err != nil || workspace.Working == nil || workspace.Working.AuthoredBy != "Alice" || len(workspace.Candidates) != 0 {
		t.Fatalf("human draft boundary changed after Jira sync: workspace=%+v err=%v", workspace, err)
	}
}

func TestJiraSyncQueuesOnePolishJobAfterReconcilingAllSolutionComments(t *testing.T) {
	setupServerTestDB(t)

	const issueKey = "DG-395"
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID: issueKey, ProjectKey: "DG", Title: "多条方案评论", Description: "需求初始内容",
	}).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}

	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":2,"comments":[{"id":"solution-1","author":{"displayName":"jira公用-解决方案"},"body":"第一部分方案。","created":"2026-08-12T10:00:00.000+0800","updated":"2026-08-12T10:00:00.000+0800"},{"id":"solution-2","author":{"displayName":"jira公用-解决方案"},"body":"第二部分方案。","created":"2026-08-12T10:01:00.000+0800","updated":"2026-08-12T10:01:00.000+0800"}]}`)
	}))
	defer jira.Close()

	cfg := &config.Config{Jira: config.JiraConfig{Enabled: true, BaseURL: jira.URL}}
	server := NewServer(cfg, "")
	server.syncJiraComments(telemetry.NewJiraClient(&cfg.Jira), issueKey)

	var jobs []db.SolutionPolishJob
	if err := db.DB.Order("id ASC").Find(&jobs).Error; err != nil {
		t.Fatalf("query polish jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("one Jira response queued %d polish jobs, want one final-source job: %+v", len(jobs), jobs)
	}
	var sourceIDs []uint
	if err := json.Unmarshal([]byte(jobs[0].SourceRefsJSON), &sourceIDs); err != nil {
		t.Fatalf("decode source refs: %v", err)
	}
	if len(sourceIDs) != 2 {
		t.Fatalf("queued source refs = %v, want both Jira solution comments", sourceIDs)
	}
}

func TestJiraSyncUsesJiraUpdatedAsRecentActivity(t *testing.T) {
	setupServerTestDB(t)

	oldKanbanPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = filepath.Join(t.TempDir(), "task_status.md")
	t.Cleanup(func() { kanban.KanbanFilePath = oldKanbanPath })

	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/search":
			fmt.Fprint(w, `{"total":2,"issues":[{"key":"WA-910","fields":{"summary":"Older Jira activity","created":"2026-07-01T08:00:00.000+0800","issuetype":{"name":"Task"},"assignee":null,"status":{"name":"To Do"},"project":{"key":"WA","name":"Well Ambient"},"fixVersions":[],"versions":[],"updated":"2026-07-10T09:15:00.000+0800"}},{"key":"WA-911","fields":{"summary":"Newer Jira activity","created":"2026-07-02T08:00:00.000+0800","issuetype":{"name":"Bug"},"assignee":null,"status":{"name":"In Progress"},"project":{"key":"WA","name":"Well Ambient"},"fixVersions":[],"versions":[],"updated":"2026-07-28T16:45:30.000+0800"}}]}`)
		case strings.HasSuffix(r.URL.Path, "/comment"):
			fmt.Fprint(w, `{"comments":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer jira.Close()

	sharedLocalSyncTime := time.Date(2026, time.August, 1, 18, 30, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	seeded := []db.TaskTelemetry{
		{TaskID: "WA-910", ProjectKey: "WA", Source: "jira", ExternalKey: "WA-910", PlanningState: "ready", Title: "Older Jira activity", Repo: "Well Ambient (WA)", Assignee: "未指派", Status: "backlog", IssueType: "requirement", TaskCreatedAt: parseJiraTime("2026-07-01T08:00:00.000+0800"), LastUpdate: sharedLocalSyncTime},
		{TaskID: "WA-911", ProjectKey: "WA", Source: "jira", ExternalKey: "WA-911", PlanningState: "ready", Title: "Newer Jira activity", Repo: "Well Ambient (WA)", Assignee: "未指派", Status: "progress", IssueType: "bug", TaskCreatedAt: parseJiraTime("2026-07-02T08:00:00.000+0800"), LastUpdate: sharedLocalSyncTime},
	}
	if err := db.DB.Create(&seeded).Error; err != nil {
		t.Fatalf("seed existing Jira tasks: %v", err)
	}

	server := NewServer(&config.Config{Jira: config.JiraConfig{
		Enabled:   true,
		BaseURL:   jira.URL,
		CustomJQL: `project = "WA"`,
	}}, "")
	server.syncJiraTasks()

	expected := map[string]time.Time{
		"WA-910": parseJiraTime("2026-07-10T09:15:00.000+0800"),
		"WA-911": parseJiraTime("2026-07-28T16:45:30.000+0800"),
	}
	var synced []db.TaskTelemetry
	if err := db.DB.Where("task_id IN ?", []string{"WA-910", "WA-911"}).Find(&synced).Error; err != nil {
		t.Fatalf("query synced tasks: %v", err)
	}
	audit := buildDailyJiraAuditResponse(synced, nil, nil, nil, time.Date(2026, time.August, 2, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60)))
	actual := make(map[string]time.Time)
	for _, bucket := range audit.Buckets {
		for _, item := range bucket.Items {
			actual[item.TaskID] = item.LastUpdate
		}
	}
	for issueKey, want := range expected {
		got, ok := actual[issueKey]
		if !ok {
			t.Fatalf("Daily Jira audit omitted %s", issueKey)
		}
		if !got.Equal(want) {
			t.Errorf("%s recent activity = %s, want Jira updated %s", issueKey, got.Format(time.RFC3339), want.Format(time.RFC3339))
		}
	}
}
